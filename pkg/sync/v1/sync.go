package v1

import (
	"context"

	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/cluster"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"golang.org/x/sync/errgroup"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type Sync struct {
	db       *database.Database
	informer cache.SharedIndexInformer
	log      *logging.Logger
	factory  func() schemav1.Resource
}

func NewSync(
	db *database.Database,
	informer cache.SharedIndexInformer,
	log *logging.Logger,
	factory func() schemav1.Resource,
) *Sync {
	return &Sync{
		db:       db,
		informer: informer,
		log:      log,
		factory:  factory,
	}
}

func (s *Sync) Run(ctx context.Context, features ...Feature) error {
	controller := NewController(s.informer, s.log)

	with := NewFeatures(features...)

	var synced map[string]types.UUID
	if !with.NoWarmup() {
		var err error
		if synced, err = s.warmup(ctx); err != nil {
			return err
		}
	}

	return s.sync(ctx, controller, synced, features...)
}

// warmup returns the UUIDs of the entities already synced to the database,
// keyed the way the informer keys its store, so that deleteVanished() can tell
// which of them are gone from the cluster.
func (s *Sync) warmup(ctx context.Context) (map[string]types.UUID, error) {
	g, ctx := errgroup.WithContext(ctx)

	meta := &schemav1.Meta{ClusterUuid: cluster.ClusterUuidFromContext(ctx)}
	query := s.db.BuildSelectStmt(s.factory(), meta) + ` WHERE cluster_uuid=:cluster_uuid`

	entities, errs := s.db.YieldAll(ctx, func() (any, error) {
		return s.factory(), nil
	}, query, meta)

	// Let errors from YieldAll() cancel the group.
	com.ErrgroupReceive(g, errs)

	synced := make(map[string]types.UUID)

	g.Go(func() error {
		for {
			select {
			case e, more := <-entities:
				if !more {
					return nil
				}

				object := e.(kmetav1.Object)

				key, err := cache.MetaNamespaceKeyFunc(object)
				if err != nil {
					return err
				}

				synced[key] = schemav1.EnsureUUID(object.GetUID())
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return synced, nil
}

// deleteVanished deletes the entities warmup() read from the database that the
// cluster no longer has. Only a synced informer tells them apart: whatever its
// initial list did not deliver was deleted while this daemon was not running,
// and no event will ever report it. A key the list does deliver under a
// different UID counts as gone as well, its object having been replaced by
// another one of the same name.
func (s *Sync) deleteVanished(ctx context.Context, sink *Sink, synced map[string]types.UUID) error {
	if len(synced) == 0 {
		return nil
	}

	if !cache.WaitForCacheSync(ctx.Done(), s.informer.HasSynced) {
		return ctx.Err()
	}

	for key, id := range synced {
		obj, exists, err := s.informer.GetStore().GetByKey(key)
		if err != nil {
			return err
		}

		if exists && schemav1.EnsureUUID(obj.(kmetav1.Object).GetUID()) == id {
			continue
		}

		if err := sink.Delete(ctx, id); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sync) sync(ctx context.Context, c *Controller, synced map[string]types.UUID, features ...Feature) error {
	sink := NewSink(func(i *Item) any {
		entity := s.factory()
		entity.Obtain(*i.Item, cluster.ClusterUuidFromContext(ctx))

		return entity
	}, func(k any) any {
		return k
	})

	with := NewFeatures(features...)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return c.Stream(ctx, sink)
	})
	g.Go(func() error {
		return s.deleteVanished(ctx, sink, synced)
	})
	g.Go(func() error {
		return s.db.UpsertStreamed(
			ctx, sink.UpsertCh(),
			database.WithCascading(), database.WithOnSuccess(with.OnUpsert()))
	})
	g.Go(func() error {
		if with.NoDelete() {
			for {
				select {
				case _, more := <-sink.DeleteCh():
					if !more {
						return nil
					}
				case <-ctx.Done():
					return ctx.Err()
				}

			}
		} else {
			return s.db.DeleteStreamed(
				ctx, s.factory(), sink.DeleteCh(),
				database.WithBlocking(), database.WithCascading(), database.WithOnSuccess(with.OnDelete()))
		}
	})

	return g.Wait()
}
