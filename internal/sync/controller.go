package sync

import (
	"context"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/internal/schema"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// NumRequeues is the number of requeues before an item is dropped from queue.
const NumRequeues = 5

// ErrShutdown is returned by processNextChange once the queue is shut down.
var ErrShutdown = errors.New("worker was shut down")

type Controller struct {
	informers Informers
	log       *logging.Logger
	queue     workqueue.TypedRateLimitingInterface[KChange]
}

func NewController(informers Informers, log *logging.Logger) *Controller {
	return &Controller{
		informers: informers,
		log:       log,
		queue: workqueue.NewTypedRateLimitingQueue[KChange](
			workqueue.DefaultTypedControllerRateLimiter[KChange](),
		),
	}
}

func (c *Controller) Stream(ctx context.Context, sink *Sink) error {
	if err := c.informers.AddEventHandler(c.eventHandler()); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		c.queue.ShutDown()
	}()

	go c.informers.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.informers.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}

	return c.runWorker(ctx, sink)
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
func (c *Controller) runWorker(ctx context.Context, sink *Sink) error {
	for {
		if err := c.processNextChange(ctx, sink); err != nil {
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
// through the goroutine in Stream that shuts the queue down. ctx does abort
// a change waiting to be sent to the sink, which returns ctx.Err().
func (c *Controller) processNextChange(ctx context.Context, sink *Sink) error {
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

	return c.processChange(ctx, sink, change, obj, exists)
}

// processChange processes a single change from the queue and sends the change
// to the appropriate sink method.
func (c *Controller) processChange(
	ctx context.Context, sink *Sink, change KChange, obj kmetav1.Object, exists bool,
) error {
	// The store has priority over the queue. So if the object
	// is not in the store it gets deleted. Every other change
	// is an upsert.
	if !exists || change.ChangeType() == KChangeDelete {
		return sink.Delete(ctx, change.Uuid())
	}

	return sink.Upsert(ctx, &Item{Key: change.KKey(), Item: &obj})
}
