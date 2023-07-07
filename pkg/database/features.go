package database

import (
	"github.com/icinga/icinga-kubernetes/pkg/com"
	"github.com/icinga/icinga-kubernetes/pkg/contracts"
)

type Feature func(*Features)

type PreExecFunc func(contracts.Entity) (bool, error)

type Features struct {
	blocking     bool
	cascading    bool
	onSuccess    com.ProcessBulk[any]
	preExecution PreExecFunc
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

func WithPreExecution(preExec PreExecFunc) Feature {
	return func(f *Features) {
		f.preExecution = preExec
	}
}

func WithOnSuccess(fn com.ProcessBulk[any]) Feature {
	return func(f *Features) {
		f.onSuccess = fn
	}
}
