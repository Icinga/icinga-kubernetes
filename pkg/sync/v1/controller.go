package v1

import (
	"context"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	informers Informers
	log       *logging.Logger
	queue     workqueue.TypedRateLimitingInterface[EventHandlerItem]
}

func NewController(informers Informers, log *logging.Logger) *Controller {
	return &Controller{
		informers: informers,
		log:       log,
		queue: workqueue.NewTypedRateLimitingQueue[EventHandlerItem](
			workqueue.DefaultTypedControllerRateLimiter[EventHandlerItem](),
		),
	}
}

func (c *Controller) Stream(ctx context.Context, sink *Sink) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Informers retry failed lists and watches forever, so a missing permission would keep the sync waiting
	// for its initial list without any hint but the logs. Fail instead.
	err := c.informers.SetWatchErrorHandler(func(ctx context.Context, r *cache.Reflector, err error) {
		if kerrors.IsForbidden(err) {
			cancel(errors.Wrap(err, "missing permission to list and watch"))
		}

		cache.DefaultWatchErrorHandler(ctx, r, err)
	})
	if err != nil {
		return err
	}

	err = c.informers.AddEventHandler(NewEventHandler(c.queue, c.log))
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		c.queue.ShutDown()
	}()

	go c.informers.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.informers.HasSynced) {
		return context.Cause(ctx)
	}

	if err := c.stream(ctx, sink); ctx.Err() == nil {
		return err
	}

	return context.Cause(ctx)
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

		item, exists, err := c.informers.GetByKey(key)
		if err != nil {
			if c.queue.NumRequeues(eventHandlerItem) < 5 {
				c.log.Errorw("Cannot fetch key. Retrying", zap.String("key", key), logging.Error(err))
				c.queue.AddRateLimited(eventHandlerItem)
			} else {
				c.log.Errorw("Cannot fetch key. Stopped retrying", zap.String("key", key), logging.Error(err))
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
