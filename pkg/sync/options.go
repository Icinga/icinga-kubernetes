package sync

import "github.com/icinga/icinga-kubernetes/pkg/contracts"

// syncOption is a functional option for NewSync.
type syncOption func(options *syncOptions)

// syncOptions stores options for sync.
type syncOptions struct {
	forwardUpserts chan<- contracts.KUpsert
	forwardDeletes chan<- contracts.KDelete
}

// newSyncOptions returns a new syncOptions initialized with the given options.
func newSyncOptions(options ...syncOption) *syncOptions {
	syncOpts := &syncOptions{}

	for _, option := range options {
		option(syncOpts)
	}

	return syncOpts
}

// WithForwardUpserts forwards added and updated Kubernetes resources to the specific channel.
func WithForwardUpserts(channel chan<- contracts.KUpsert) syncOption {
	return func(options *syncOptions) {
		options.forwardUpserts = channel
	}
}

// WithForwardDeletes forwards deleted Kubernetes resources to the specific channel.
func WithForwardDeletes(channel chan<- contracts.KDelete) syncOption {
	return func(options *syncOptions) {
		options.forwardDeletes = channel
	}
}
