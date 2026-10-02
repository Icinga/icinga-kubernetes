package v1

import (
	"time"

	"github.com/icinga/icinga-go-library/database"
)

type Feature func(*Features)

type Features struct {
	noDelete         bool
	noWarmup         bool
	deletedRetention time.Duration
	onDelete         database.OnSuccess[any]
	onUpsert         database.OnSuccess[any]
}

func NewFeatures(features ...Feature) *Features {
	f := &Features{}
	for _, feature := range features {
		feature(f)
	}

	return f
}

func (f *Features) NoDelete() bool {
	return f.noDelete
}

func (f *Features) NoWarmup() bool {
	return f.noWarmup
}

func (f *Features) DeletedRetention() time.Duration {
	return f.deletedRetention
}

func (f *Features) OnDelete() database.OnSuccess[any] {
	return f.onDelete
}

func (f *Features) OnUpsert() database.OnSuccess[any] {
	return f.onUpsert
}

func WithNoDelete() Feature {
	return func(f *Features) {
		f.noDelete = true
	}
}

func WithNoWarmup() Feature {
	return func(f *Features) {
		f.noWarmup = true
	}
}

func WithDeletedRetention(retention time.Duration) Feature {
	return func(f *Features) {
		f.deletedRetention = retention
	}
}

func WithOnDelete(fn database.OnSuccess[any]) Feature {
	return func(f *Features) {
		f.onDelete = fn
	}
}

func WithOnUpsert(fn database.OnSuccess[any]) Feature {
	return func(f *Features) {
		f.onUpsert = fn
	}
}
