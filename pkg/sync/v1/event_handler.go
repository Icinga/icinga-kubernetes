package v1

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/icinga/icinga-go-library/types"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type EventHandler struct {
	queue workqueue.TypedInterface[EventHandlerItem]
	log   logr.Logger
}

type EventHandlerItem struct {
	Type EventType
	Id   types.UUID
	KKey string
}

type EventType string

const EventAdd EventType = "ADDED"
const EventUpdate EventType = "UPDATED"
const EventDelete EventType = "DELETED"

func NewEventHandler(queue workqueue.TypedInterface[EventHandlerItem], log logr.Logger) cache.ResourceEventHandler {
	return &EventHandler{queue: queue, log: log}
}

func (e *EventHandler) OnAdd(obj interface{}, _ bool) {
	e.enqueue(EventAdd, obj, cache.MetaNamespaceKeyFunc)
}

func (e *EventHandler) OnUpdate(_, newObj interface{}) {
	e.enqueue(EventUpdate, newObj, cache.MetaNamespaceKeyFunc)
}

func (e *EventHandler) OnDelete(obj interface{}) {
	e.enqueue(EventDelete, obj, cache.DeletionHandlingMetaNamespaceKeyFunc)
}

func (e *EventHandler) enqueue(_type EventType, obj interface{}, keyFunc cache.KeyFunc) {
	key, err := keyFunc(obj)
	if err != nil {
		e.log.Error(err, "Can't make key")

		return
	}

	var id types.UUID
	switch v := obj.(type) {
	case kmetav1.Object:
		id = schemav1.EnsureUUID(v.GetUID())
	case cache.DeletedFinalStateUnknown:
		id = schemav1.EnsureUUID(v.Obj.(kmetav1.Object).GetUID())
	default:
		panic(fmt.Sprintf("unknown object type %#v", v))
	}

	e.queue.Add(EventHandlerItem{
		Type: _type,
		Id:   id,
		KKey: key,
	})
}
