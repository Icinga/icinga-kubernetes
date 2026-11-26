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
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// NumRequeues is the number of requeues before an item is dropped from queue.
const NumRequeues = 5

// ErrShutdown is returned by processNextChange once the queue is shut down.
var ErrShutdown = errors.New("worker was shut down")

// Controller syncs one Kubernetes resource into the database. It turns
// informer events into KChanges, resolves each against the informer store,
// and streams the results to the database as upserts and deletes.
type Controller struct {
	db        *kdatabase.DB
	informers Informers
	log       *logging.Logger
	queue     workqueue.TypedRateLimitingInterface[KChange]
	factory   func() schema.Resource
	upserts   chan database.Entity
	deletes   chan any
}

// NewController returns a new Controller for the resource that
// factory creates.
func NewController(db *kdatabase.DB, informers Informers, log *logging.Logger, factory func() schema.Resource) *Controller {
	return &Controller{
		db:        db,
		informers: informers,
		log:       log,
		factory:   factory,
		upserts:   make(chan database.Entity),
		deletes:   make(chan any),
		queue: workqueue.NewTypedRateLimitingQueue[KChange](
			workqueue.DefaultTypedControllerRateLimiter[KChange](),
		),
	}
}

// Run syncs the resource into the database until ctx is done or any of its
// goroutines fails. It first reads the entities already synced, unless
// WithNoWarmup() is given, then registers the event handlers, starts the
// informers and waits for their caches to sync. From there it streams
// changes from the queue into the database, and deletes the entities that
// vanished while the daemon was not running.
func (c *Controller) Run(ctx context.Context, features ...Feature) error {
	with := NewFeatures(features...)

	var synced map[string]types.UUID
	if !with.NoWarmup() {
		var err error
		if synced, err = c.warmup(ctx); err != nil {
			return err
		}
	}

	g, ctx := errgroup.WithContext(ctx)

	// errgroup cancels ctx only once Wait() returns, which the error
	// returns below never reach. Cancel on every return instead, so
	// the informers and the shutdown watcher always stop with Run.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Unblock queue.Get() when any group member fails.
	g.Go(func() error {
		<-ctx.Done()
		c.queue.ShutDown()

		return nil
	})

	// Register informers event handlers before starting the informer to ensure
	// that we don't miss any changes that happen during the initial sync.
	if err := c.informers.AddEventHandler(c.eventHandler()); err != nil {
		return err
	}

	c.log.Debug("Start syncing")

	// Start the informer to begin streaming changes.
	go c.informers.Run(ctx.Done())

	// Wait for the caches to sync before starting the worker so that we
	// don't miss any changes that happen during the initial sync.
	if !cache.WaitForCacheSync(ctx.Done(), c.informers.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}

	g.Go(func() error {
		// Run the worker to process events from the queue.
		return c.runWorker(ctx)
	})

	g.Go(func() error {
		return c.deleteVanished(ctx, synced)
	})

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

			ids, err := c.vanished(objects)
			if err != nil {
				return err
			}

			if len(ids) > 0 {
				// Not sent from here, as this holds a slot of the table's
				// semaphore, which the delete needs as well.
				g.Go(func() error {
					for _, id := range ids {
						select {
						case c.deletes <- id:
						case <-ctx.Done():
							return ctx.Err()
						}
					}

					return nil
				})
			}

			return nil
		}
	}

	g.Go(func() error {
		return c.db.UpsertStreamedCascading(ctx, c.upserts, onUpsert)
	})

	g.Go(func() error {
		if with.NoDelete() {
			for {
				select {
				case _, more := <-c.deletes:
					if !more {
						return nil
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		} else {
			return c.db.SoftDeleteStreamed(ctx, c.factory(), c.deletes, with.OnDelete())
		}
	})

	return g.Wait()
}

// warmup returns the UUIDs of the entities already synced to the database,
// keyed the way the informer keys its store, so that deleteVanished() can
// tell which of them are gone from the cluster.
func (c *Controller) warmup(ctx context.Context) (map[string]types.UUID, error) {
	g, ctx := errgroup.WithContext(ctx)

	meta := &schema.Meta{ClusterUuid: cluster.ClusterUuidFromContext(ctx)}
	query := c.db.BuildSelectStmt(c.factory(), meta) + ` WHERE "cluster_uuid"=:cluster_uuid AND "deleted" IS NULL`

	entities, errs := c.db.YieldAll(ctx, func() database.Entity { return c.factory() }, query, meta)

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
// cluster no longer has. Call it only once the informer caches have synced:
// whatever the initial list did not deliver was deleted while this daemon was
// not running, and no event will ever report it. A key the list does deliver
// under a different UID counts as gone as well, its object having been
// replaced by another one of the same name.
func (c *Controller) deleteVanished(ctx context.Context, synced map[string]types.UUID) error {
	if len(synced) == 0 {
		return nil
	}

	for key, uuid := range synced {
		obj, exists, err := c.informers.GetByKey(key)
		if err != nil {
			return err
		}

		if exists && schema.EnsureUUID(obj.GetUID()) == uuid {
			continue
		}

		select {
		case c.deletes <- uuid:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// vanished returns the UUIDs of the given entities that the informers no
// longer have, or have under a different UID.
func (c *Controller) vanished(entities []kmetav1.Object) ([]any, error) {
	var ids []any

	for _, entity := range entities {
		key, err := cache.MetaNamespaceKeyFunc(entity)
		if err != nil {
			return nil, err
		}

		obj, exists, err := c.informers.GetByKey(key)
		if err != nil {
			return nil, err
		}

		if exists && obj.GetUID() == entity.GetUID() {
			continue
		}

		ids = append(ids, schema.EnsureUUID(entity.GetUID()))
	}

	return ids, nil
}

// enqueue takes an object and creates a KChange out of it which is then added
// to the queue. It generates a key for the object with the given keyFunc.
func (c *Controller) enqueue(changeType KChangeType, obj any, keyFunc cache.KeyFunc) error {
	var kobj kmetav1.Object

	switch t := obj.(type) {
	case kmetav1.Object:
		kobj = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		kobj, ok = t.Obj.(kmetav1.Object)
		if !ok {
			return errors.Errorf("object %#v is not kmetav1.Object", obj)
		}
	default:
		return errors.Errorf("unknown object type %#v", obj)
	}

	kkey, err := keyFunc(obj)
	if err != nil {
		return errors.Wrapf(err, "cannot make key for object: Object=%#v", obj)
	}

	change := NewKChange(changeType, schema.EnsureUUID(kobj.GetUID()), kkey)

	c.queue.Add(change)

	c.log.Debugw("Added change to queue", zap.Inline(change))

	return nil
}

// eventHandler returns a cache.ResourceEventHandler that will be called when
// objects are added, updated, or deleted from the informer's store.
func (c *Controller) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if err := c.enqueue(KChangeUpsert, obj, cache.MetaNamespaceKeyFunc); err != nil {
				c.log.Errorw("Cannot enqueue change", zap.String("type", string(KChangeUpsert)), logging.Error(err))
			}
		},
		UpdateFunc: func(_, obj any) {
			if err := c.enqueue(KChangeUpsert, obj, cache.MetaNamespaceKeyFunc); err != nil {
				c.log.Errorw("Cannot enqueue change", zap.String("type", string(KChangeUpsert)), logging.Error(err))
			}
		},
		DeleteFunc: func(obj any) {
			if err := c.enqueue(KChangeDelete, obj, cache.DeletionHandlingMetaNamespaceKeyFunc); err != nil {
				c.log.Errorw("Cannot enqueue change", zap.String("type", string(KChangeDelete)), logging.Error(err))
			}
		},
	}
}

// runWorker calls processNextChange to process the next change from the queue.
func (c *Controller) runWorker(ctx context.Context) error {
	for {
		if err := c.processNextChange(ctx); err != nil {
			if errors.Is(err, ErrShutdown) {
				return nil
			}

			return err
		}
	}
}

// processNextChange takes the next change from the queue and hands it to
// processChange. It returns ErrShutdown once the queue is shut down, and
// nil after processing a change, so that runWorker can call it in a loop.
//
// Note that queue.Get() does not observe ctx: cancellation only unblocks it
// through the error group in Run that shuts the queue down. ctx does abort
// a change waiting to be sent to an output channel, which returns ctx.Err().
func (c *Controller) processNextChange(ctx context.Context) error {
	// Set item to processing state.
	change, shutdown := c.queue.Get()
	if shutdown {
		return ErrShutdown
	}

	// Must be called after queue.Get() to remove item from processing state.
	defer c.queue.Done(change)

	obj, exists, err := c.informers.GetByKey(change.KKey())
	if err != nil {
		if c.queue.NumRequeues(change) < NumRequeues {
			// Requeue the change if an error occurred while fetching the
			// key from the store and the number of requeues did not
			// exceed the limit. Then process the next change.
			c.queue.AddRateLimited(change)
			c.log.Errorw("Cannot fetch key. Retrying", zap.String("key", change.KKey()), logging.Error(err))
		} else {
			// Stop retrying the change if an error occurred while fetching
			// the key from the store and the number of requeues did exceed
			// the limit. Then process the next change.
			c.queue.Forget(change)
			c.log.Errorw("Cannot fetch key. Stopped retrying", zap.String("key", change.KKey()), logging.Error(err))
		}

		return nil
	}

	// Stop retrying the change now that the store lookup succeeded.
	// Forget() only resets the rate limiter; the deferred Done() is what
	// ends the processing state.
	c.queue.Forget(change)

	return c.processChange(ctx, change, obj, exists)
}

// processChange processes a single change from the queue and sends the change
// to the appropriate channel.
func (c *Controller) processChange(ctx context.Context, change KChange, obj kmetav1.Object, exists bool) error {
	// The store has priority over the queue. So if the object
	// is not in the store it gets sent to the deletes channel.
	// Every other change is an upsert and goes to the upserts channel.
	if !exists || change.ChangeType() == KChangeDelete {
		select {
		case c.deletes <- change.Uuid():
		case <-ctx.Done():
			return ctx.Err()
		}
	} else {
		entity := c.factory()
		entity.Obtain(obj, cluster.ClusterUuidFromContext(ctx))

		select {
		case c.upserts <- entity:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	c.log.Debugw("Forwarded change from queue to channel", zap.Inline(change))

	return nil
}
