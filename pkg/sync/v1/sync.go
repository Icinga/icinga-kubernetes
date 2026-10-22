package v1

import (
	"context"

	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/pkg/cluster"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"golang.org/x/sync/errgroup"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type Sync struct {
	db       *kdatabase.DB
	informer cache.SharedIndexInformer
	log      *logging.Logger
	factory  func() schemav1.Resource
}

func NewSync(
	db *kdatabase.DB,
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

	return s.sync(ctx, controller, synced, with)
}

// warmup returns the UUIDs of the entities already synced to the database,
// keyed the way the informer keys its store, so that deleteVanished() can tell
// which of them are gone from the cluster.
func (s *Sync) warmup(ctx context.Context) (map[string]types.UUID, error) {
	g, ctx := errgroup.WithContext(ctx)

	meta := &schemav1.Meta{ClusterUuid: cluster.ClusterUuidFromContext(ctx)}
	query := s.db.BuildSelectStmt(s.factory(), meta) + ` WHERE cluster_uuid=:cluster_uuid AND deleted IS NULL`

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

// vanished returns the UUIDs of the given entities that the informer no longer
// has, or has under a different UID.
func (s *Sync) vanished(entities []kmetav1.Object) ([]any, error) {
	var ids []any

	for _, entity := range entities {
		key, err := cache.MetaNamespaceKeyFunc(entity)
		if err != nil {
			return nil, err
		}

		obj, exists, err := s.informer.GetStore().GetByKey(key)
		if err != nil {
			return nil, err
		}

		if exists && obj.(kmetav1.Object).GetUID() == entity.GetUID() {
			continue
		}

		ids = append(ids, schemav1.EnsureUUID(entity.GetUID()))
	}

	return ids, nil
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

	onUpsert := with.OnUpsert()
	if !with.NoDelete() {
		original := onUpsert

		// Upserts and deletes are batched and flushed independently, so the
		// delete of an entity can run before its first upsert, match no row
		// and leave the upserted row undeleted. The informer drops an entity
		// before it reports the deletion, so whatever it lacks once the upsert
		// has been written is deleted again. Deleting twice is harmless, as the
		// soft delete skips rows already marked as deleted, but WithOnDelete()
		// callbacks see such a deletion twice.
		onUpsert = func(bulkCtx context.Context, entities []database.Entity) error {
			if original != nil {
				if err := original(bulkCtx, entities); err != nil {
					return err
				}
			}

			objects := make([]kmetav1.Object, 0, len(entities))
			for _, e := range entities {
				objects = append(objects, e.(kmetav1.Object))
			}

			ids, err := s.vanished(objects)
			if err != nil {
				return err
			}

			if len(ids) > 0 {
				// Not sent from here, as this holds a slot of the table's
				// semaphore, which the delete needs as well.
				g.Go(func() error {
					for _, id := range ids {
						if err := sink.Delete(ctx, id); err != nil {
							return err
						}
					}

					return nil
				})
			}

			return nil
		}
	}

	g.Go(func() error {
		return c.Stream(ctx, sink)
	})
	g.Go(func() error {
		return s.deleteVanished(ctx, sink, synced)
	})
	g.Go(func() error {
		return s.db.UpsertStreamedCascading(ctx, sink.UpsertCh(), onUpsert)
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
			return s.db.SoftDeleteStreamed(ctx, s.factory(), sink.DeleteCh(), with.OnDelete())
		}
	})

	return g.Wait()
}
