package v1

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/cluster"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

type Sync struct {
	db       *database.Database
	informer cache.SharedIndexInformer
	log      logr.Logger
	factory  func() schemav1.Resource
}

func NewSync(
	db *database.Database,
	informer cache.SharedIndexInformer,
	log logr.Logger,
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
	controller := NewController(s.informer, s.log.WithName("controller"))

	with := NewFeatures(features...)
	warmupKeys := map[string]types.UUID{}

	if !with.NoWarmup() {
		var err error
		warmupKeys, err = s.warmup(ctx)
		if err != nil {
			return err
		}
	}

	return s.sync(ctx, controller, warmupKeys, features...)
}

func (s *Sync) warmup(ctx context.Context) (map[string]types.UUID, error) {
	g, ctx := errgroup.WithContext(ctx)

	meta := &schemav1.Meta{ClusterUuid: cluster.ClusterUuidFromContext(ctx)}
	query := s.db.BuildSelectStmt(s.factory(), meta) + ` WHERE cluster_uuid=:cluster_uuid`
	warmupKeys := make(map[string]types.UUID)

	entities, errs := s.db.YieldAll(ctx, func() (interface{}, error) {
		return s.factory(), nil
	}, query, meta)

	// Let errors from YieldAll() cancel the group.
	com.ErrgroupReceive(g, errs)

	g.Go(func() error {
		defer runtime.HandleCrash()

		for {
			select {
			case e, more := <-entities:
				if !more {
					return nil
				}

				object := e.(schemav1.Resource)
				key, err := cache.MetaNamespaceKeyFunc(object)
				if err != nil {
					return err
				}

				warmupKeys[key] = schemav1.EnsureUUID(object.GetUID())
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return warmupKeys, g.Wait()
}

func (s *Sync) sync(ctx context.Context, c *Controller, warmupKeys map[string]types.UUID, features ...Feature) error {
	sink := NewSink(func(i *Item) interface{} {
		entity := s.factory()
		entity.Obtain(*i.Item, cluster.ClusterUuidFromContext(ctx))

		return entity
	}, func(k interface{}) interface{} {
		return k
	})

	with := NewFeatures(features...)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer runtime.HandleCrash()

		return c.Stream(ctx, sink, warmupKeys)
	})
	g.Go(func() error {
		defer runtime.HandleCrash()

		return s.db.UpsertStreamed(
			ctx, sink.UpsertCh(),
			database.WithCascading(), database.WithOnSuccess(with.OnUpsert()))
	})
	g.Go(func() error {
		defer runtime.HandleCrash()

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
	g.Go(func() error {
		defer runtime.HandleCrash()

		for {
			select {
			case err, more := <-sink.ErrorCh():
				if !more {
					return nil
				}

				s.log.Error(err, "sync error")
			case <-ctx.Done():
				return ctx.Err()
			}

		}
	})

	return g.Wait()
}
