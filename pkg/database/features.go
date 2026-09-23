package database

import (
	"github.com/icinga/icinga-go-library/database"
)

type Feature func(*Features)

type Features struct {
	blocking  bool
	cascading bool
	onSuccess database.OnSuccess[any]
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

func WithOnSuccess(fn database.OnSuccess[any]) Feature {
	return func(f *Features) {
		f.onSuccess = fn
	}
}

// withoutOnSuccess returns a Feature that applies f except for its onSuccess callback.
func (f *Features) withoutOnSuccess() Feature {
	return func(feat *Features) {
		*feat = *f
		feat.onSuccess = nil
	}
}
