package sync

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type EventHandler struct {
	queue workqueue.Interface
	log   logr.Logger
}

func NewEventHandler(queue workqueue.Interface, log logr.Logger) cache.ResourceEventHandler {
	return &EventHandler{queue: queue, log: log}
}

func (e *EventHandler) OnAdd(obj interface{}, _ bool) {
	e.enqueue(obj, cache.MetaNamespaceKeyFunc)
}

func (e *EventHandler) OnUpdate(_, newObj interface{}) {
	e.enqueue(newObj, cache.MetaNamespaceKeyFunc)
}

func (e *EventHandler) OnDelete(obj interface{}) {
	e.enqueue(obj, cache.DeletionHandlingMetaNamespaceKeyFunc)
}

func (e *EventHandler) enqueue(obj interface{}, keyFunc cache.KeyFunc) {
	key, err := keyFunc(obj)
	if err != nil {
		e.log.Error(err, "Can't make key")

		return
	}

	e.queue.Add(key)
}
