package v1

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/icinga/icinga-go-library/types"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"github.com/pkg/errors"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	informer cache.SharedIndexInformer
	log      logr.Logger
	queue    workqueue.TypedRateLimitingInterface[EventHandlerItem]
}

func NewController(
	informer cache.SharedIndexInformer,
	log logr.Logger,
) *Controller {

	return &Controller{
		informer: informer,
		log:      log,
		queue: workqueue.NewTypedRateLimitingQueue[EventHandlerItem](
			workqueue.DefaultTypedControllerRateLimiter[EventHandlerItem](),
		),
	}
}

func (c *Controller) Stream(ctx context.Context, sink *Sink, warmupKeys map[string]types.UUID) error {
	_, err := c.informer.AddEventHandler(NewEventHandler(c.queue, c.log.WithName("events")))
	if err != nil {
		return err
	}

	go func() {
		defer runtime.HandleCrash()

		<-ctx.Done()
		c.queue.ShutDown()
	}()

	go c.informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}

	if err := c.enqueueMissingWarmupDeletes(warmupKeys); err != nil {
		return err
	}

	return c.stream(ctx, sink)
}

func (c *Controller) enqueueMissingWarmupDeletes(warmupKeys map[string]types.UUID) error {
	for key, warmupId := range warmupKeys {
		item, exists, err := c.informer.GetStore().GetByKey(key)
		if err != nil {
			return errors.Wrapf(err, "fetching warmed-up key %s failed", key)
		}

		if exists {
			currentId := schemav1.EnsureUUID(item.(kmetav1.Object).GetUID())
			if currentId == warmupId {
				continue
			}
		}

		c.queue.Add(EventHandlerItem{
			Type: EventDelete,
			Id:   warmupId,
			KKey: key,
		})
	}

	return nil
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
				c.log.Error(errors.WithStack(err), fmt.Sprintf("Fetching key %s failed. Retrying", key))

				c.queue.AddRateLimited(eventHandlerItem)
			} else {
				c.queue.Forget(eventHandlerItem)

				if err := sink.Error(ctx, errors.Wrapf(err, "fetching key %s failed", key)); err != nil {
					return err
				}
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
