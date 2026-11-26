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

	return c.stream(ctx, sink)
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

func (c *Controller) stream(ctx context.Context, sink *Sink) error {
	var change KChange
	var key string
	var shutdown bool
	for {
		c.queue.Done(change)

		change, shutdown = c.queue.Get()
		if shutdown {
			return ctx.Err()
		}

		key = change.KKey()

		obj, exists, err := c.informers.GetByKey(key)
		if err != nil {
			if c.queue.NumRequeues(change) < 5 {
				c.log.Errorw("Cannot fetch key. Retrying", zap.String("key", key), logging.Error(err))
				c.queue.AddRateLimited(change)
			} else {
				c.log.Errorw("Cannot fetch key. Stopped retrying", zap.String("key", key), logging.Error(err))
				c.queue.Forget(change)
			}

			continue
		}

		c.queue.Forget(change)

		if !exists || change.ChangeType() == KChangeDelete {
			if err := sink.Delete(ctx, change.Uuid()); err != nil {
				return err
			}
		} else {
			if err := sink.Upsert(ctx, &Item{Key: key, Item: &obj}); err != nil {
				return err
			}
		}
	}
}
