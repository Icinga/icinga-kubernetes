package sync

import (
	"context"
	"sync"
	"time"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

// forbiddenInformers maps the informers that are denied to list or watch to
// their resource version at their latest denial. It is shared, as the same
// informer can be part of several Informers, e.g., pods for their sync and
// their metrics.
var forbiddenInformers sync.Map

const (
	// permittedInterval is how often a denied informer is checked for being permitted again.
	permittedInterval = 1 * time.Second

	// permittedFor is how long an informer must keep looking permitted, as a
	// denied watch may follow a permitted list.
	permittedFor = 5 * time.Second
)

// Informers combines informers of the same resource, e.g., one per watched
// namespace, so that they can be used like a single one.
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

// SkipForbidden makes informers that are denied to list or watch count as synced,
// so that they do not hold up the others. Each logs a warning when denied and
// keeps retrying quietly, so it catches up once permitted, which is logged as
// well. It must be called before Run.
func (i Informers) SkipForbidden(log *logging.Logger) error {
	for _, informer := range i {
		if err := informer.SetWatchErrorHandlerWithContext(func(ctx context.Context, r *cache.Reflector, err error) {
			if !kerrors.IsForbidden(err) {
				cache.DefaultWatchErrorHandler(ctx, r, err)

				return
			}

			if _, denied := forbiddenInformers.Swap(informer, informer.LastSyncResourceVersion()); denied {
				return
			}

			log.Warnw("Cannot list and watch. Skipping until permitted", logging.Error(err))

			go awaitPermitted(ctx, log, informer, err)
		}); err != nil {
			return err
		}
	}

	return nil
}

// awaitPermitted logs once denied informer is permitted to list and watch
// again, i.e., its resource version has changed since its latest denial.
// It then forgets the denial, so that the next one is logged again.
func awaitPermitted(ctx context.Context, log *logging.Logger, informer cache.SharedIndexInformer, denial error) {
	// The resource version of the denial the informer looks permitted after.
	var permittedAfter string

	// When the informer started to look permitted, or zero if it does not.
	var passedAt time.Time

	if err := wait.PollUntilContextCancel(ctx, permittedInterval, false, func(context.Context) (bool, error) {
		v, ok := forbiddenInformers.Load(informer)
		if !ok {
			// The denial was forgotten elsewhere, so there is nothing to await.
			// This is defensive and currently can't happen.
			return false, errors.New("denial forgotten")
		}
		deniedRV := v.(string)

		// Forgetting the denial before the informer has synced would make it
		// hold up the others again.
		if informer.LastSyncResourceVersion() == deniedRV || !informer.HasSynced() {
			passedAt = time.Time{}

			return false, nil
		}

		// A permitted list changes the resource version, but a denied watch may
		// follow it within moments. Only if no denial follows for permittedFor
		// is the informer permitted.
		if passedAt.IsZero() || permittedAfter != deniedRV {
			permittedAfter = deniedRV
			passedAt = time.Now()

			return false, nil
		}

		if time.Since(passedAt) < permittedFor {
			return false, nil
		}

		return forbiddenInformers.CompareAndDelete(informer, deniedRV), nil
	}); err != nil {
		return
	}

	log.Infow("Permitted to list and watch now. No longer skipping", zap.String("denied", denial.Error()))
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
		if _, forbidden := forbiddenInformers.Load(informer); !forbidden && !informer.HasSynced() {
			return false
		}
	}

	return true
}

// GetByKey returns the object stored under key in the first informer that has it.
// The informers' stores are disjoint, so the first one is the only one.
func (i Informers) GetByKey(key string) (kmetav1.Object, bool, error) {
	for _, informer := range i {
		item, exists, err := informer.GetStore().GetByKey(key)
		if err != nil || exists {
			obj, _ := item.(kmetav1.Object)

			return obj, exists, err
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

// ListByNamespace returns the objects of all informers in namespace that match selector.
func (i Informers) ListByNamespace(namespace string, selector labels.Selector) ([]any, error) {
	var items []any
	for _, informer := range i {
		if err := cache.ListAllByNamespace(informer.GetIndexer(), namespace, selector, func(item any) {
			items = append(items, item)
		}); err != nil {
			return nil, err
		}
	}

	return items, nil
}
