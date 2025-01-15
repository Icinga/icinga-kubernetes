package database

import (
	"context"
)

type Feature func(*Features)

type Features struct {
	blocking  bool
	cascading bool
	onSuccess ProcessBulk[any]
}

func NewFeatures(features ...Feature) *Features {
	f := &Features{}
	for _, feature := range features {
		feature(f)
	}

	return f
}

func WithBlocking() Feature {
	return func(f *Features) {
		f.blocking = true
	}
}

func WithCascading() Feature {
	return func(f *Features) {
		f.cascading = true
	}
}

func WithOnSuccess(fn ProcessBulk[any]) Feature {
	return func(f *Features) {
		f.onSuccess = fn
	}
}

type ProcessBulk[T any] func(ctx context.Context, bulk []T) (err error)

func ForwardBulk[T any](ch chan<- T) ProcessBulk[T] {
	return func(ctx context.Context, rows []T) error {
		for _, row := range rows {
			select {
			case ch <- row:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	}
}
