package v1

import (
	"fmt"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/types"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	"go.uber.org/zap"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type EventHandler struct {
	queue workqueue.TypedInterface[EventHandlerItem]
	log   *logging.Logger
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

func NewEventHandler(queue workqueue.TypedInterface[EventHandlerItem], log *logging.Logger) cache.ResourceEventHandler {
	return &EventHandler{queue: queue, log: log}
}

func (e *EventHandler) OnAdd(obj any, _ bool) {
	e.enqueue(EventAdd, obj, cache.MetaNamespaceKeyFunc)
}

func (e *EventHandler) OnUpdate(_, newObj any) {
	e.enqueue(EventUpdate, newObj, cache.MetaNamespaceKeyFunc)
}

func (e *EventHandler) OnDelete(obj any) {
	e.enqueue(EventDelete, obj, cache.DeletionHandlingMetaNamespaceKeyFunc)
}

func (e *EventHandler) enqueue(_type EventType, obj any, keyFunc cache.KeyFunc) {
	key, err := keyFunc(obj)
	if err != nil {
		e.log.Errorw("cannot make key", zap.Error(err))

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
