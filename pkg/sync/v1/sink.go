package v1

import (
	"context"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type Item struct {
	Key  string
	Item *kmetav1.Object
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

func (s *Sink) Delete(ctx context.Context, key interface{}) error {
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
	if item.Item != nil {
		deletionTimestamp := (*item.Item).GetDeletionTimestamp()
		if !deletionTimestamp.IsZero() && deletionTimestamp.Time.Compare(time.Now().Add(30*time.Second)) <= 0 {
			// Don't process UPSERTs if the resource is about to be deleted in the next 30 seconds to
			// prevent races between simultaneous UPSERT and DELETE statements for the same resource,
			// where an UPSERT statement can occur after a DELETE statement has already been executed.
			return ctx.Err()
		}
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
