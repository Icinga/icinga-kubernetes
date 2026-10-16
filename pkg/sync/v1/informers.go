package v1

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/icinga/icinga-go-library/logging"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

// forbidden holds the informers that were denied to list or watch.
// It is shared, as the same informer can be part of several Informers, e.g. pods for their sync and their metrics.
var forbidden sync.Map

// TypedInformers combines informers of the same resource, e.g., one per watched
// namespace, so that they can be used like a single one. T is the type of the
// objects they store.
type TypedInformers[T kmetav1.Object] []cache.SharedIndexInformer

// Informers is what Sync and Controller need from informers, independent of the object type.
type Informers interface {
	AddEventHandler(handler cache.ResourceEventHandler) error
	SkipForbidden(log *logging.Logger) error
	Forbidden() bool
	Run(stopCh <-chan struct{})
	HasSynced() bool
	Object(key string) (kmetav1.Object, bool, error)
}

// NewTypedInformers returns the informer for objects of type T,
// e.g., *corev1.Pod, from each factory.
// It panics if T is not a kind known to client-go.
func NewTypedInformers[T kmetav1.Object](factories ...informers.SharedInformerFactory) TypedInformers[T] {
	var gvks []schema.GroupVersionKind
	for gvk, t := range scheme.Scheme.AllKnownTypes() {
		if reflect.PointerTo(t) == reflect.TypeFor[T]() {
			gvks = append(gvks, gvk)
		}
	}

	if len(gvks) != 1 {
		panic(fmt.Sprintf("%v has %d kinds, want 1", reflect.TypeFor[T](), len(gvks)))
	}

	gvr, _ := meta.UnsafeGuessKindToResource(gvks[0])

	var i TypedInformers[T]
	for _, f := range factories {
		generic, err := f.ForResource(gvr)
		if err != nil {
			panic(err)
		}

		i = append(i, generic.Informer())
	}

	return i
}

// AddEventHandler adds handler to all informers.
func (i TypedInformers[T]) AddEventHandler(handler cache.ResourceEventHandler) error {
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
func (i TypedInformers[T]) SkipForbidden(log *logging.Logger) error {
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
func (i TypedInformers[T]) Forbidden() bool {
	for _, informer := range i {
		if _, ok := forbidden.Load(informer); ok && !informer.HasSynced() {
			return true
		}
	}

	return false
}

// Run runs all informers until stopCh is closed.
func (i TypedInformers[T]) Run(stopCh <-chan struct{}) {
	for _, informer := range i {
		go informer.Run(stopCh)
	}

	<-stopCh
}

// HasSynced reports whether all informers have synced or were denied to list or watch.
func (i TypedInformers[T]) HasSynced() bool {
	for _, informer := range i {
		if _, ok := forbidden.Load(informer); !ok && !informer.HasSynced() {
			return false
		}
	}

	return true
}

// GetByKey returns the object stored under key in the first informer that has it.
// The informers' stores are disjoint, so the first one is the only one.
func (i TypedInformers[T]) GetByKey(key string) (T, bool, error) {
	for _, informer := range i {
		item, exists, err := informer.GetStore().GetByKey(key)
		if err != nil {
			return *new(T), false, err
		}

		if exists {
			return item.(T), true, nil
		}
	}

	return *new(T), false, nil
}

// Object is GetByKey for callers that only need the object's metadata.
func (i TypedInformers[T]) Object(key string) (kmetav1.Object, bool, error) {
	obj, exists, err := i.GetByKey(key)
	if err != nil || !exists {
		return nil, exists, err
	}

	return obj, true, nil
}

// List returns the objects of all informers that match selector.
func (i TypedInformers[T]) List(selector labels.Selector) []T {
	var items []T
	for _, informer := range i {
		for _, item := range informer.GetStore().List() {
			obj := item.(T)
			if selector.Matches(labels.Set(obj.GetLabels())) {
				items = append(items, obj)
			}
		}
	}

	return items
}
