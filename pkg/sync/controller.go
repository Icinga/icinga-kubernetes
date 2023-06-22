package sync

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	informer cache.SharedIndexInformer
	log      logr.Logger
	queue    workqueue.RateLimitingInterface
}

func NewController(
	informer cache.SharedIndexInformer,
	log logr.Logger,
) *Controller {

	return &Controller{
		informer: informer,
		log:      log,
		queue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}
}

func (c *Controller) Announce(obj interface{}) error {
	return c.informer.GetStore().Add(obj)
}

func (c *Controller) Stream(ctx context.Context, sink *Sink) error {
	_, err := c.informer.AddEventHandler(NewEventHandler(c.queue, c.log.WithName("events")))
	if err != nil {
		return err
	}

	go func() {
		defer runtime.HandleCrash()

		select {
		case <-ctx.Done():
			c.queue.ShutDown()
			return
		}
	}()

	go c.informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}

	return c.stream(ctx, sink)
}

func (c *Controller) stream(ctx context.Context, sink *Sink) error {
	var key interface{}
	var shutdown bool
	for {
		c.queue.Done(key)

		key, shutdown = c.queue.Get()
		if shutdown {
			return ctx.Err()
		}

		item, exists, err := c.informer.GetStore().GetByKey(key.(string))
		if err != nil {
			if c.queue.NumRequeues(key) < 5 {
				c.log.Error(errors.WithStack(err), fmt.Sprintf("Fetching key %s failed. Retrying", key))

				c.queue.AddRateLimited(key)
			} else {
				c.queue.Forget(key)

				if err := sink.Error(ctx, errors.Wrapf(err, "fetching key %s failed", key)); err != nil {
					return err
				}
			}

			continue
		}

		c.queue.Forget(key)

		if !exists {
			if err := sink.Delete(ctx, key.(string)); err != nil {
				return err
			}
		} else {
			err := sink.Upsert(ctx, &Item{
				Key:  key.(string),
				Item: item.(kmetav1.Object),
			})
			if err != nil {
				return err
			}
		}
	}
}
