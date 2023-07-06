package sync

import (
	"context"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Item struct {
	Key  string
	Item kmetav1.Object
}

type Sink struct {
	error      chan error
	delete     chan interface{}
	deleteFunc func(interface{}) interface{}
	upsert     chan interface{}
	upsertFunc func(*Item) interface{}
}

func NewSink(upsertFunc func(*Item) interface{}, deleteFunc func(interface{}) interface{}) *Sink {
	return &Sink{
		error:      make(chan error),
		delete:     make(chan interface{}),
		deleteFunc: deleteFunc,
		upsert:     make(chan interface{}),
		upsertFunc: upsertFunc,
	}
}

func (s *Sink) Delete(ctx context.Context, key string) error {
	select {
	case s.delete <- s.deleteFunc(key):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Sink) DeleteCh() <-chan interface{} {
	return s.delete
}

func (s *Sink) Error(ctx context.Context, err error) error {
	select {
	case s.error <- err:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Sink) ErrorCh() <-chan error {
	return s.error
}

func (s *Sink) Upsert(ctx context.Context, item *Item) error {
	if !item.Item.GetDeletionTimestamp().IsZero() {
		// K8s might dispatch an update event for an object that is already marked for
		// deletion due to its sub-resources being deleted/modified. However, neither event
		// matters to us as their parent object is going to be deleted soon.
		return nil
	}

	select {
	case s.upsert <- s.upsertFunc(item):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Sink) UpsertCh() <-chan interface{} {
	return s.upsert
}
