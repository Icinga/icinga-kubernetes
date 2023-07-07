package v1

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/icinga/icinga-kubernetes/pkg/database"
	"github.com/icinga/icinga-kubernetes/pkg/sync"
	"github.com/icinga/icinga-kubernetes/pkg/types"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"reflect"
)

type FactoryFunc func() contracts.Entity

type Sync struct {
	db       *database.Database
	informer cache.SharedIndexInformer
	log      logr.Logger
	factory  FactoryFunc
	store    cache.Store
}

func NewSync(
	db *database.Database,
	informer cache.SharedIndexInformer,
	log logr.Logger,
	factory FactoryFunc,
) *Sync {
	return &Sync{
		db:       db,
		informer: informer,
		log:      log,
		factory:  factory,
		store:    cache.NewStore(ObjectMetaKeyFunc),
	}
}

func (s *Sync) Run(ctx context.Context, features ...sync.Feature) error {
	controller := sync.NewController(s.informer, s.log.WithName("controller"))

	with := sync.NewFeatures(features...)

	if !with.NoWarmup() {
		if err := s.warmup(ctx, controller); err != nil {
			s.log.Error(err, "warmup failed")
			return err
		}

		s.log.Info("sync warmup finished")
	}

	s.log.Info("start syncing configs")

	return s.sync(ctx, controller, features...)
}

// GetState returns the cached entity of the given object.
// It returns an error if it fails to internally generate a key for the specified object,
// and nil if the provided object doesn't have a cached state.
func (s *Sync) GetState(obj interface{}) (contracts.Entity, error) {
	item, exist, err := s.store.Get(obj)
	if err != nil {
		return nil, err
	}

	if !exist {
		return nil, nil
	}

	return item.(contracts.Entity), nil
}

// Delete removes the given entity and all its references from the cache store.
func (s *Sync) Delete(entity contracts.Entity, cascade bool) {
	if _, ok := entity.(database.HasRelations); ok && cascade {
		items := s.store.List()
		for _, it := range items {
			item := it.(contracts.Entity)
			if entity.ID().Equal(item.ParentID()) {
				// Erase all references of this entity recursively from the cache store as well.
				// Example: Remove v1.Pod -> v1.Container -> v1.ContainerMount etc...
				s.Delete(item, cascade)
			}
		}
	}

	// We don't know whether there is a cached item by the given hash, so ignore any errors.
	_ = s.store.Delete(entity)
}

func (s *Sync) warmup(ctx context.Context, c *sync.Controller) error {
	g, ctx := errgroup.WithContext(ctx)

	s.log.Info("starting sync warmup")

	entity := s.factory()
	entities, errs := s.db.YieldAll(ctx, func() (interface{}, bool, error) {
		return s.factory(), true, nil
	}, s.db.BuildSelectStmt(entity, entity.Fingerprint()))
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

				if err := s.store.Add(e.(contracts.Entity).Fingerprint()); err != nil {
					return err
				}

				// The controller doesn't need to know about the entities of a k8s sub resource.
				if resource, ok := e.(contracts.Resource); ok {
					if err := c.Announce(resource); err != nil {
						return err
					}
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}

func (s *Sync) sync(ctx context.Context, c *sync.Controller, features ...sync.Feature) error {
	sink := sync.NewSink(func(i *sync.Item) interface{} {
		entity := s.factory()
		entity.(contracts.Resource).Obtain(i.Item)

		return entity
	}, func(k interface{}) interface{} {
		return types.Checksum(k)
	})

	with := sync.NewFeatures(features...)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer runtime.HandleCrash()

		return c.Stream(ctx, sink)
	})
	g.Go(func() error {
		filterFunc := func(entity contracts.Entity) (bool, error) {
			lastState, err := s.GetState(entity)
			if err != nil {
				return false, err
			}

			// Don't upsert the entities if their checksum hasn't been changed.
			if lastState == nil || !entity.Checksum().Equal(lastState.Checksum()) {
				_ = s.store.Add(entity.Fingerprint())

				return true, nil
			}

			return false, nil
		}

		return s.db.UpsertStreamed(
			ctx, sink.UpsertCh(),
			database.WithCascading(), database.WithPreExecution(filterFunc), database.WithOnSuccess(with.OnUpsert()),
		)
	})
	g.Go(func() error {
		return s.deleteEntities(ctx, s.factory(), sink.DeleteCh(), features...)
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

	err := g.Wait()
	if err != nil {
		s.log.Error(err, "sync error")
	}
	return err
}

// deleteEntities consumes the entities from the provided delete stream and syncs them to the database.
// It also removes the streamed K8s entity and all its references from the cache store automatically.
// To prevent the sender goroutines of this stream from being blocked, the entities are still consumed
// from the stream even if the sync.WithNoDelete feature is specified.
func (s *Sync) deleteEntities(ctx context.Context, subject contracts.Entity, delete <-chan interface{}, features ...sync.Feature) error {
	with := sync.NewFeatures(features...)

	if relations, ok := subject.(database.HasRelations); ok && !with.NoDelete() {
		g, ctx := errgroup.WithContext(ctx)
		streams := make(map[string]chan interface{})
		for _, relation := range relations.Relations() {
			relation := relation
			if !relation.CascadeDelete() {
				continue
			}

			if _, ok := relation.TypePointer().(contracts.Entity); !ok {
				// This shouldn't crush the daemon, when some of the k8s types specify a relation
				// that doesn't satisfy the contracts.Entity interface.
				continue
			}

			relationCh := make(chan interface{})
			g.Go(func() error {
				defer runtime.HandleCrash()
				defer close(relationCh)

				return s.deleteEntities(ctx, relation.TypePointer().(contracts.Entity), relationCh)
			})
			streams[database.TableName(relation)] = relationCh
		}

		deleteIds := make(chan interface{})
		g.Go(func() error {
			defer runtime.HandleCrash()
			defer close(deleteIds)

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case id, ok := <-delete:
					if !ok {
						return nil
					}

					if _, ok := id.(types.Binary); !ok {
						id = types.Binary(id.([]byte))
					}

					subject.SetID(id.(types.Binary))
					items := s.store.List()
					// First delete all references before deleting the parent entity.
					for _, item := range items {
						entity := item.(contracts.Entity)
						if subject.ID().Equal(entity.ParentID()) {
							for _, relation := range subject.(database.HasRelations).Relations() {
								relation := relation
								if reflect.TypeOf(relation.TypePointer().(contracts.Entity).Fingerprint()) == reflect.TypeOf(entity) {
									select {
									case streams[database.TableName(relation)] <- entity.ID():
									case <-ctx.Done():
										return ctx.Err()
									}
								}
							}
						}
					}

					select {
					case deleteIds <- id:
					case <-ctx.Done():
						return ctx.Err()
					}

					s.Delete(subject, false)
				}
			}
		})

		g.Go(func() error {
			defer runtime.HandleCrash()

			return s.db.DeleteStreamed(ctx, subject, deleteIds, database.WithBlocking(), database.WithOnSuccess(with.OnDelete()))
		})

		return g.Wait()
	}

	g, ctx := errgroup.WithContext(ctx)
	deleteIds := make(chan interface{})
	g.Go(func() error {
		defer runtime.HandleCrash()
		defer close(deleteIds)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case id, ok := <-delete:
				if !ok {
					return nil
				}

				if !with.NoDelete() {
					select {
					case deleteIds <- id:
					case <-ctx.Done():
						return ctx.Err()
					}

					if _, ok := id.(types.Binary); !ok {
						id = types.Binary(id.([]byte))
					}

					subject.SetID(id.(types.Binary))
					s.Delete(subject, false)
				}
			}
		}
	})

	if !with.NoDelete() {
		g.Go(func() error {
			defer runtime.HandleCrash()

			return s.db.DeleteStreamed(ctx, subject, deleteIds, database.WithBlocking(), database.WithOnSuccess(with.OnDelete()))
		})
	}

	return g.Wait()
}

// ObjectMetaKeyFunc provides a custom implementation of object key extraction for caching.
// The given object has to satisfy the contracts.IDer interface if it's not an explicit key.
func ObjectMetaKeyFunc(obj interface{}) (string, error) {
	if _, ok := obj.(cache.ExplicitKey); ok {
		return cache.MetaNamespaceKeyFunc(obj)
	}

	return obj.(contracts.IDer).ID().String(), nil
}
