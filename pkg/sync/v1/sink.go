package v1

import (
	"context"

	"github.com/icinga/icinga-go-library/database"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Item struct {
	Key  string
	Item *kmetav1.Object
}

type Sink struct {
	delete     chan any
	deleteFunc func(any) any
	upsert     chan database.Entity
	upsertFunc func(*Item) database.Entity
}

func NewSink(upsertFunc func(*Item) database.Entity, deleteFunc func(any) any) *Sink {
	return &Sink{
		delete:     make(chan any),
		deleteFunc: deleteFunc,
		upsert:     make(chan database.Entity),
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
	select {
	case s.upsert <- s.upsertFunc(item):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Sink) UpsertCh() <-chan database.Entity {
	return s.upsert
}
