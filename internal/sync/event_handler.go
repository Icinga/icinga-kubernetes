package sync

import (
	"fmt"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-kubernetes/internal/schema"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type EventHandler struct {
	queue workqueue.TypedInterface[KChange]
	log   *logging.Logger
}

func NewEventHandler(queue workqueue.TypedInterface[KChange], log *logging.Logger) cache.ResourceEventHandler {
	return &EventHandler{queue: queue, log: log}
}

func (e *EventHandler) OnAdd(obj any, _ bool) {
	e.enqueue(KChangeUpsert, obj, cache.MetaNamespaceKeyFunc)
}

func (e *EventHandler) OnUpdate(_, newObj any) {
	e.enqueue(KChangeUpsert, newObj, cache.MetaNamespaceKeyFunc)
}

func (e *EventHandler) OnDelete(obj any) {
	e.enqueue(KChangeDelete, obj, cache.DeletionHandlingMetaNamespaceKeyFunc)
}

func (e *EventHandler) enqueue(changeType KChangeType, obj any, keyFunc cache.KeyFunc) {
	kkey, err := keyFunc(obj)
	if err != nil {
		e.log.Errorw("Cannot make key", logging.Error(err))

		return
	}

	var uuid types.UUID
	switch v := obj.(type) {
	case kmetav1.Object:
		uuid = schema.EnsureUUID(v.GetUID())
	case cache.DeletedFinalStateUnknown:
		uuid = schema.EnsureUUID(v.Obj.(kmetav1.Object).GetUID())
	default:
		panic(fmt.Sprintf("unknown object type %#v", v))
	}

	e.queue.Add(NewKChange(changeType, uuid, kkey))
}
