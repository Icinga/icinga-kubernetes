package v1

import "github.com/icinga/icinga-kubernetes/pkg/database"

type Feature func(*Features)

type Features struct {
	noDelete bool
	noWarmup bool
	onDelete database.ProcessBulk[any]
	onUpsert database.ProcessBulk[any]
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

func (f *Features) OnDelete() database.ProcessBulk[any] {
	return f.onDelete
}

func (f *Features) OnUpsert() database.ProcessBulk[any] {
	return f.onUpsert
}

func WithNoDelete() Feature {
	return func(f *Features) {
		f.noDelete = true
	}
}

func WithNoWarumup() Feature {
	return func(f *Features) {
		f.noWarmup = true
	}
}

func WithOnDelete(fn database.ProcessBulk[any]) Feature {
	return func(f *Features) {
		f.onDelete = fn
	}
}

func WithOnUpsert(fn database.ProcessBulk[any]) Feature {
	return func(f *Features) {
		f.onUpsert = fn
	}
}
