package sync

import (
	"context"

	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/cluster"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
	"github.com/icinga/icinga-kubernetes/internal/schema"
	"golang.org/x/sync/errgroup"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type Sync struct {
	db        *kdatabase.DB
	informers Informers
	log       *logging.Logger
	factory   func() schema.Resource
}

func NewSync(
	db *kdatabase.DB,
	informers Informers,
	log *logging.Logger,
	factory func() schema.Resource,
) *Sync {
	return &Sync{
		db:        db,
		informers: informers,
		log:       log,
		factory:   factory,
	}
}

func (s *Sync) Run(ctx context.Context, features ...Feature) error {
	controller := NewController(s.informers, s.log)

	with := NewFeatures(features...)

	var synced map[string]types.UUID
	if !with.NoWarmup() {
		var err error
		if synced, err = s.warmup(ctx); err != nil {
			return err
		}
	}

	return s.sync(ctx, controller, synced, with)
}

// warmup returns the UUIDs of the entities already synced to the database,
// keyed the way the informer keys its store, so that deleteVanished() can tell
// which of them are gone from the cluster.
func (s *Sync) warmup(ctx context.Context) (map[string]types.UUID, error) {
	g, ctx := errgroup.WithContext(ctx)

	meta := &schema.Meta{ClusterUuid: cluster.ClusterUuidFromContext(ctx)}
	query := s.db.BuildSelectStmt(s.factory(), meta) + ` WHERE "cluster_uuid"=:cluster_uuid`

	entities, errs := s.db.YieldAll(ctx, func() database.Entity { return s.factory() }, query, meta)

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

				synced[key] = schema.EnsureUUID(object.GetUID())
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

	if !cache.WaitForCacheSync(ctx.Done(), s.informers.HasSynced) {
		return ctx.Err()
	}

	for key, id := range synced {
		obj, exists, err := s.informers.GetByKey(key)
		if err != nil {
			return err
		}

		if exists && schema.EnsureUUID(obj.(kmetav1.Object).GetUID()) == id {
			continue
		}

		if err := sink.Delete(ctx, id); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sync) sync(ctx context.Context, c *Controller, synced map[string]types.UUID, with *Features) error {
	sink := NewSink(func(i *Item) database.Entity {
		entity := s.factory()
		entity.Obtain(*i.Item, cluster.ClusterUuidFromContext(ctx))

		return entity
	}, func(k any) any {
		return k
	})

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return c.Stream(ctx, sink)
	})
	g.Go(func() error {
		return s.deleteVanished(ctx, sink, synced)
	})
	g.Go(func() error {
		return s.db.UpsertStreamedCascading(ctx, sink.UpsertCh(), with.OnUpsert())
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
			return s.db.DeleteStreamedCascading(ctx, s.factory(), sink.DeleteCh(), with.OnDelete())
		}
	})

	return g.Wait()
}
