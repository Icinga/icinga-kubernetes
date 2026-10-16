package v1

import (
	"context"
	"sync"

	"github.com/icinga/icinga-go-library/logging"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

// forbidden holds the informers that were denied to list or watch.
// It is shared, as the same informer can be part of several Informers, e.g. pods for their sync and their metrics.
var forbidden sync.Map

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

// SkipForbidden makes informers that are denied to list or watch count as synced, so that they do not hold up
// the others. Each logs a warning once and keeps retrying quietly, so it catches up once permitted.
// It must be called before Run.
func (i Informers) SkipForbidden(log *logging.Logger) error {
	for _, informer := range i {
		err := informer.SetWatchErrorHandlerWithContext(func(ctx context.Context, r *cache.Reflector, err error) {
			if !kerrors.IsForbidden(err) {
				cache.DefaultWatchErrorHandler(ctx, r, err)

				return
			}

			if _, warned := forbidden.LoadOrStore(informer, struct{}{}); !warned {
				log.Warnw("Cannot list and watch. Skipping until permitted", logging.Error(err))
			}
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Forbidden reports whether any informer has not synced because it was denied to list or watch.
func (i Informers) Forbidden() bool {
	for _, informer := range i {
		if _, ok := forbidden.Load(informer); ok && !informer.HasSynced() {
			return true
		}
	}

	return false
}

// Run runs all informers until stopCh is closed.
func (i Informers) Run(stopCh <-chan struct{}) {
	for _, informer := range i {
		go informer.Run(stopCh)
	}

	<-stopCh
}

// HasSynced reports whether all informers have synced or were denied to list or watch.
func (i Informers) HasSynced() bool {
	for _, informer := range i {
		if _, ok := forbidden.Load(informer); !ok && !informer.HasSynced() {
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
