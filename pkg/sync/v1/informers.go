package v1

import (
	"k8s.io/client-go/tools/cache"
)

// Informers combines informers of the same resource, e.g. one per watched namespace,
// so that they can be used like a single one.
type Informers []cache.SharedIndexInformer

// AddEventHandler adds handler to all informers.
func (i Informers) AddEventHandler(handler cache.ResourceEventHandler) error {
	for _, informer := range i {
		if _, err := informer.AddEventHandler(handler); err != nil {
			return err
		}
	}

	return nil
}

// SetWatchErrorHandler sets handler for the list and watch errors of all informers.
// It must be called before Run.
func (i Informers) SetWatchErrorHandler(handler cache.WatchErrorHandlerWithContext) error {
	for _, informer := range i {
		if err := informer.SetWatchErrorHandlerWithContext(handler); err != nil {
			return err
		}
	}

	return nil
}

// Run runs all informers until stopCh is closed.
func (i Informers) Run(stopCh <-chan struct{}) {
	for _, informer := range i {
		go informer.Run(stopCh)
	}

	<-stopCh
}

// HasSynced reports whether all informers have synced.
func (i Informers) HasSynced() bool {
	for _, informer := range i {
		if !informer.HasSynced() {
			return false
		}
	}

	return true
}

// GetByKey returns the object stored under key in the first informer that has it.
// The informers' stores are disjoint, so the first one is the only one.
func (i Informers) GetByKey(key string) (any, bool, error) {
	for _, informer := range i {
		item, exists, err := informer.GetStore().GetByKey(key)
		if err != nil || exists {
			return item, exists, err
		}
	}

	return nil, false, nil
}

// List returns the objects of all informers.
func (i Informers) List() []any {
	var items []any
	for _, informer := range i {
		items = append(items, informer.GetStore().List()...)
	}

	return items
}
