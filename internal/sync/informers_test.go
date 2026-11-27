package sync

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	kcorev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestInformers(t *testing.T) {
	tests := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		// wanted
		"a/pod-1": {obj: &kcorev1.Pod{Namespace: "a", Name: "pod-1"}, want: true},
		"a/pod-2": {obj: &kcorev1.Pod{Namespace: "a", Name: "pod-2"}, want: true},
		"b/pod-1": {obj: &kcorev1.Pod{Namespace: "b", Name: "pod-1"}, want: true},
		"b/pod-2": {obj: &kcorev1.Pod{Namespace: "b", Name: "pod-2"}, want: true},
		"b/pod-3": {obj: &kcorev1.Pod{Namespace: "b", Name: "pod-3"}, want: true},

		// unwanted
		"c/pod-1": {obj: &kcorev1.Pod{Namespace: "c", Name: "pod-1"}, want: false},
		"c/pod-2": {obj: &kcorev1.Pod{Namespace: "c", Name: "pod-2"}, want: false},
	}

	wantTotal := 5
	wantPerNamespace := map[string]int{"a": 2, "b": 3, "c": 0}

	client := fake.NewClientset()
	for _, tt := range tests {
		_ = client.Tracker().Add(tt.obj)
	}

	var i Informers
	for _, ns := range []string{"a", "b"} {
		factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace(ns))
		i = append(i, factory.Core().V1().Pods().Informer())
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	go i.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), i.HasSynced) {
		t.Fatal("timed out waiting for cache sync")
	}

	for key, tt := range tests {
		if _, exists, err := i.GetByKey(key); err != nil || exists != tt.want {
			t.Errorf("GetByKey(%q) = %v, %v; want %v, nil", key, exists, err, tt.want)
		}
	}

	if got := len(i.List()); got != wantTotal {
		t.Errorf("List() returned %d items, want %d", got, wantTotal)
	}

	for ns, want := range wantPerNamespace {
		items, err := i.ListByNamespace(ns, labels.Everything())
		if err != nil {
			t.Errorf("ListByNamespace(%q, labels.Everything()) returned an error: %v", ns, err)
		}
		got := len(items)
		if got != want {
			t.Errorf("ListByNamespace(%q, labels.Everything()) returned %d items, want %d", ns, got, want)
		}
	}
}

func TestInformersSkipForbidden(t *testing.T) {
	client := fake.NewClientset(&kcorev1.Pod{Namespace: "b", Name: "pod"})

	var denied atomic.Bool
	denied.Store(true)

	deny := kerrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("denied"))
	client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		return denied.Load(), nil, deny
	})
	client.PrependWatchReactor("pods", func(ktesting.Action) (bool, watch.Interface, error) {
		return denied.Load(), nil, deny
	})

	factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace("b"))
	informer := factory.Core().V1().Pods().Informer()
	i := Informers{informer}

	if err := i.SkipForbidden(logging.NewLogger(zap.NewNop().Sugar(), time.Second)); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	go i.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), i.HasSynced) {
		t.Fatal("timed out waiting for cache sync")
	}

	// A denied informer cannot sync, so only the skip lets it count as synced.
	if _, forbidden := forbiddenInformers.Load(informer.GetIndexer()); !forbidden || informer.HasSynced() {
		t.Fatal("informer was not skipped as denied")
	}

	denied.Store(false)

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		t.Fatal("timed out waiting for the permitted informer to catch up")
	}

	if _, exists, err := i.GetByKey("b/pod"); err != nil || !exists {
		t.Errorf("GetByKey(%q) = %v, %v; want true, nil", "b/pod", exists, err)
	}
}

// stubInformer is denied and synced whenever a test says so, instead of
// whenever the reflector retries. It has an indexer of its own, as denials are
// recorded by indexer. Its other methods panic, as the embedded informer is nil.
type stubInformer struct {
	cache.SharedIndexInformer
	handler cache.WatchErrorHandlerWithContext
	indexer cache.Indexer
	synced  atomic.Bool
}

func (s *stubInformer) SetWatchErrorHandlerWithContext(handler cache.WatchErrorHandlerWithContext) error {
	s.handler = handler

	return nil
}

func (s *stubInformer) HasSynced() bool {
	return s.synced.Load()
}

func (s *stubInformer) GetIndexer() cache.Indexer {
	return s.indexer
}

// TestInformersSkipForbiddenPermittedAgain checks that a denied informer is
// logged as permitted once it is, and that its denial is forgotten then, so that
// the next one is warned about again. Being denied while running, it is logged
// as permitted only once it is permitted again.
func TestInformersSkipForbiddenPermittedAgain(t *testing.T) {
	informer := &stubInformer{indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc, nil)}

	deny := kerrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("denied"))

	i := Informers{informer}

	const (
		permittedFor      = 300 * time.Millisecond
		permittedInterval = 10 * time.Millisecond
	)

	core, logs := observer.New(zap.InfoLevel)
	if err := i.skipForbidden(logging.NewLogger(zap.New(core).Sugar(), time.Second), permittedFor, permittedInterval); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	informer.handler(ctx, nil, deny)

	// Forgetting the denial before the informer has synced would make it hold
	// up the others again.
	time.Sleep(permittedFor + 2*permittedInterval)

	if got := logs.FilterMessage("Permitted to list and watch now. No longer skipping").Len(); got != 0 {
		t.Fatalf("informer was logged as permitted %d times before it synced, want 0", got)
	}

	informer.synced.Store(true)

	if !cache.WaitForCacheSync(ctx.Done(), func() bool {
		return logs.FilterMessage("Permitted to list and watch now. No longer skipping").Len() == 1
	}) {
		t.Fatal("timed out waiting for the informer to be logged as permitted")
	}

	// A synced informer can be denied again, e.g., its watch after a permitted list.
	informer.handler(ctx, nil, deny)

	if !cache.WaitForCacheSync(ctx.Done(), func() bool {
		return logs.FilterMessage("Cannot list and watch. Skipping until permitted").Len() == 2
	}) {
		t.Fatal("timed out waiting for the informer to be warned about as denied again")
	}

	// Check that the informer is not logged as permitted while it is still
	// denied. It would be once permittedFor passes without a denial, but the
	// denials during the loop come often enough to prevent that.
	for range 20 {
		time.Sleep(permittedFor / 10)
		informer.handler(ctx, nil, deny)
	}

	if got := logs.FilterMessage("Permitted to list and watch now. No longer skipping").Len(); got != 1 {
		t.Fatalf("informer was logged as permitted %d times while denied again, want 1", got)
	}

	if !cache.WaitForCacheSync(ctx.Done(), func() bool {
		return logs.FilterMessage("Permitted to list and watch now. No longer skipping").Len() == 2
	}) {
		t.Fatal("timed out waiting for the informer to be logged as permitted again")
	}
}

// TestInformersSkipForbiddenWatch checks that an informer whose list is
// permitted but whose watch is denied syncs and still counts as denied, as the
// reflector reports the denied watch after the permitted list.
func TestInformersSkipForbiddenWatch(t *testing.T) {
	client := fake.NewClientset(&kcorev1.Pod{Namespace: "b", Name: "pod"})

	deny := kerrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("denied"))
	client.PrependWatchReactor("pods", func(ktesting.Action) (bool, watch.Interface, error) {
		return true, nil, deny
	})

	factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace("b"))
	informer := factory.Core().V1().Pods().Informer()
	i := Informers{informer}

	if err := i.SkipForbidden(logging.NewLogger(zap.NewNop().Sugar(), time.Second)); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	go i.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), func() bool {
		_, forbidden := forbiddenInformers.Load(informer.GetIndexer())

		return forbidden && informer.HasSynced()
	}) {
		t.Fatal("timed out waiting for the informer to sync and be denied")
	}
}

func TestInformersSkipForbiddenSeparateCalls(t *testing.T) {
	client := fake.NewClientset(&kcorev1.Pod{Namespace: "b", Name: "pod"})

	deny := kerrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("denied"))
	client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, deny
	})
	client.PrependWatchReactor("pods", func(ktesting.Action) (bool, watch.Interface, error) {
		return true, nil, deny
	})

	// Like main.go, which calls factory.Pods() for the sync and again for
	// ServicePodSync and PromMetricSync.
	factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace("b"))
	forSync := Informers{factory.Core().V1().Pods().Informer()}
	forOthers := Informers{factory.Core().V1().Pods().Informer()}

	if err := forSync.SkipForbidden(logging.NewLogger(zap.NewNop().Sugar(), time.Second)); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	go forSync.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), forSync.HasSynced) {
		t.Fatal("timed out waiting for the informers of the sync")
	}

	if !cache.WaitForCacheSync(ctx.Done(), forOthers.HasSynced) {
		t.Fatal("timed out waiting for the informers of the others")
	}
}
