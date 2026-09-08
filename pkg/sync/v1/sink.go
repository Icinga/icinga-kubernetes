package v1

import (
	"context"
	"time"

	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Item struct {
	Key  string
	Item *kmetav1.Object
}

type Sink struct {
	delete     chan any
	deleteFunc func(any) any
	upsert     chan any
	upsertFunc func(*Item) any
}

func NewSink(upsertFunc func(*Item) any, deleteFunc func(any) any) *Sink {
	return &Sink{
		delete:     make(chan any),
		deleteFunc: deleteFunc,
		upsert:     make(chan any),
		upsertFunc: upsertFunc,
	}
}

func (s *Sink) Delete(ctx context.Context, key any) error {
	select {
	case s.delete <- s.deleteFunc(key):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Sink) DeleteCh() <-chan any {
	return s.delete
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

func (s *Sink) UpsertCh() <-chan any {
	return s.upsert
}
