package sink

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/pkg/common"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kcache "k8s.io/client-go/tools/cache"
	kworkqueue "k8s.io/client-go/util/workqueue"
)

// Sink provides handlers for events that happen to Kubernetes resources to stream the changes for further processing.
type Sink interface {
	kcache.ResourceEventHandler

	// Run streams changes,
	// i.e. added, updated and deleted Kubernetes resources to Adds, Updates and Deletes respectively.
	Run(context.Context) error

	// Adds returns a channel through which added Kubernetes resources are delivered.
	Adds() <-chan contracts.KUpsert

	// Updates returns a channel through which updated Kubernetes resources are delivered.
	Updates() <-chan contracts.KUpsert

	// Deletes returns a channel through which deleted Kubernetes resources are delivered.
	Deletes() <-chan contracts.KDelete
}

func NewSink(store kcache.Store, logger *logging.Logger) Sink {
	return &sink{
		store:   store,
		queue:   kworkqueue.NewRateLimitingQueue(kworkqueue.DefaultControllerRateLimiter()),
		logger:  logger,
		adds:    make(chan contracts.KUpsert),
		updates: make(chan contracts.KUpsert),
		deletes: make(chan contracts.KDelete),
	}
}

type sink struct {
	store   kcache.Store
	queue   kworkqueue.RateLimitingInterface
	logger  *logging.Logger
	adds    chan contracts.KUpsert
	updates chan contracts.KUpsert
	deletes chan contracts.KDelete
}

func (s *sink) Adds() <-chan contracts.KUpsert {
	return s.adds
}

func (s *sink) Updates() <-chan contracts.KUpsert {
	return s.updates
}

func (s *sink) Deletes() <-chan contracts.KDelete {
	return s.deletes
}

func (s *sink) OnAdd(obj any, _ bool) {
	s.enqueue(kcache.Added, obj, kcache.MetaNamespaceKeyFunc)
}

func (s *sink) OnUpdate(_, newObj any) {
	s.enqueue(kcache.Updated, newObj, kcache.MetaNamespaceKeyFunc)
}

func (s *sink) OnDelete(obj any) {
	s.enqueue(kcache.Deleted, obj, kcache.DeletionHandlingMetaNamespaceKeyFunc)
}

func (s *sink) Run(ctx context.Context) error {
	defer close(s.adds)
	defer close(s.updates)
	defer close(s.deletes)

	go func() {
		select {
		case <-ctx.Done():
			s.queue.ShutDown()
		}
	}()

	for {
		item, shutdown := s.queue.Get()
		if shutdown {
			return ctx.Err()
		}

		d, obj, exists, err := s.processItem(item)
		if err != nil {
			panic(err)
		}

		if !exists && d.Type != kcache.Deleted {
			panic(d)
		}

		if exists && d.Type == kcache.Deleted {
			panic(d)
		}

		switch d.Type {
		case kcache.Added:
			kupsert := d.Object.(contracts.KEnvelope).KUpsert(obj.(kmetav1.Object))

			select {
			case s.adds <- kupsert:
				s.logger.Debugw(
					fmt.Sprintf("Propagate: %s %s", d.Type, kupsert.GetCanonicalName()),
					zap.String("id", kupsert.ID().String()))
			case <-ctx.Done():
				return ctx.Err()
			}
		case kcache.Updated:
			kupsert := d.Object.(contracts.KEnvelope).KUpsert(obj.(kmetav1.Object))

			select {
			case s.updates <- kupsert:
				s.logger.Debugw(
					fmt.Sprintf("Propagate: %s %s", d.Type, kupsert.GetCanonicalName()),
					zap.String("id", kupsert.ID().String()))
			case <-ctx.Done():
				return ctx.Err()
			}
		case kcache.Deleted:
			kdelete := d.Object.(contracts.KEnvelope).KDelete()

			select {
			case s.deletes <- kdelete:
				s.logger.Debugw(
					fmt.Sprintf("Propagate: %s %s", d.Type, kdelete.GetCanonicalName()),
					zap.String("id", kdelete.ID().String()))
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (s *sink) enqueue(_type kcache.DeltaType, obj any, keyFunc kcache.KeyFunc) {
	key, err := keyFunc(obj)
	if err != nil {
		panic(err)
	}

	kenvelope := common.NewKEnvelope(key)

	s.queue.Add(kcache.Delta{
		Type:   _type,
		Object: kenvelope,
	})

	s.logger.Debugw(
		fmt.Sprintf("Queue: %s %s", _type, kenvelope.GetCanonicalName()),
		zap.String("id", kenvelope.ID().String()))
}

func (s *sink) processItem(item any) (d kcache.Delta, obj any, exists bool, err error) {
	defer s.queue.Done(item)

	d = item.(kcache.Delta)

	obj, exists, err = s.store.GetByKey(d.Object.(contracts.KEnvelope).GetCanonicalName())
	if err != nil {
		err = errors.Wrap(err, "can't get key from store")
	}

	return
}
