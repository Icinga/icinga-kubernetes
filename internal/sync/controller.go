package sync

import (
	"context"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
	if err := c.informers.AddEventHandler(NewEventHandler(c.queue, c.log)); err != nil {
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
