package database

import (
	"github.com/icinga/icinga-kubernetes/pkg/com"
)

type Feature func(*Features)

type Features struct {
	blocking  bool
	cascading bool
	onSuccess com.ProcessBulk[any]
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

func WithOnSuccess(fn com.ProcessBulk[any]) Feature {
	return func(f *Features) {
		f.onSuccess = fn
	}
}
