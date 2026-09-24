package v1

import (
	"context"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	informer cache.SharedIndexInformer
	log      *logging.Logger
	queue    workqueue.TypedRateLimitingInterface[EventHandlerItem]
}

func NewController(
	informer cache.SharedIndexInformer,
	log *logging.Logger,
) *Controller {

	return &Controller{
		informer: informer,
		log:      log,
		queue: workqueue.NewTypedRateLimitingQueue[EventHandlerItem](
			workqueue.DefaultTypedControllerRateLimiter[EventHandlerItem](),
		),
	}
}

func (c *Controller) Stream(ctx context.Context, sink *Sink) error {
	_, err := c.informer.AddEventHandler(NewEventHandler(c.queue, c.log))
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		c.queue.ShutDown()
	}()

	go c.informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}

	return c.stream(ctx, sink)
}

func (c *Controller) stream(ctx context.Context, sink *Sink) error {
	var eventHandlerItem EventHandlerItem
	var key string
	var shutdown bool
	for {
		c.queue.Done(eventHandlerItem)

		eventHandlerItem, shutdown = c.queue.Get()
		if shutdown {
			return ctx.Err()
		}

		key = eventHandlerItem.KKey

		item, exists, err := c.informer.GetStore().GetByKey(key)
		if err != nil {
			if c.queue.NumRequeues(eventHandlerItem) < 5 {
				c.log.Errorw("Cannot fetch key. Retrying", zap.String("key", key), zap.Error(errors.WithStack(err)))
				c.queue.AddRateLimited(eventHandlerItem)
			} else {
				c.log.Errorw("Cannot fetch key. Stopped retrying", zap.String("key", key), zap.Error(errors.WithStack(err)))
				c.queue.Forget(eventHandlerItem)
			}

			continue
		}

		c.queue.Forget(eventHandlerItem)

		if !exists || eventHandlerItem.Type == EventDelete {
			if err := sink.Delete(ctx, eventHandlerItem.Id); err != nil {
				return err
			}
		} else {
			obj := item.(kmetav1.Object)
			err := sink.Upsert(ctx, &Item{
				Key:  key,
				Item: &obj,
			})
			if err != nil {
				return err
			}
		}
	}
}
