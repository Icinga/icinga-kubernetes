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
	if _, forbidden := forbiddenInformers.Load(informer); !forbidden || informer.HasSynced() {
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

// TestInformersSkipForbiddenPermittedAgain checks that a denied informer is
// logged as permitted once it is, and that its denial is forgotten then, so that
// the next one is warned about again. Being denied while running, it is logged
// as permitted only once it is permitted again.
func TestInformersSkipForbiddenPermittedAgain(t *testing.T) {
	if testing.Short() {
		t.Skip("waits about 45 seconds for the reflector")
	}

	client := fake.NewClientset()

	var denied atomic.Bool
	denied.Store(true)

	// The watch of the permitted informer, to end it once denied again.
	var watcher atomic.Pointer[watch.FakeWatcher]

	deny := kerrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("denied"))
	client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		return denied.Load(), nil, deny
	})
	client.PrependWatchReactor("pods", func(ktesting.Action) (bool, watch.Interface, error) {
		if denied.Load() {
			return true, nil, deny
		}

		w := watch.NewFake()
		watcher.Store(w)

		return true, w, nil
	})

	i := Informers{informers.NewSharedInformerFactory(client, 0).Core().V1().Pods().Informer()}

	// Shorter than the reflector's longest backoff, so the test must check that
	// the informer is still denied before its backoff grows that long.
	const permittedFor = 15 * time.Second

	core, logs := observer.New(zap.InfoLevel)
	if err := i.skipForbidden(logging.NewLogger(zap.New(core).Sugar(), time.Second), permittedFor); err != nil {
		t.Fatal(err)
	}

	// Each permitted log takes up to permittedFor plus a poll after the latest
	// denial, and the sleep in between as long again, so about 45 seconds.
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()

	go i.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), i.HasSynced) {
		t.Fatal("timed out waiting for the informer to be denied")
	}

	denied.Store(false)

	if !cache.WaitForCacheSync(ctx.Done(), func() bool {
		return logs.FilterMessage("Permitted to list and watch now. No longer skipping").Len() == 1
	}) {
		t.Fatal("timed out waiting for the informer to be logged as permitted")
	}

	// Ending the watch makes the informer retry, and be denied again.
	denied.Store(true)
	watcher.Load().Stop()

	if !cache.WaitForCacheSync(ctx.Done(), func() bool {
		return logs.FilterMessage("Cannot list and watch. Skipping until permitted").Len() == 2
	}) {
		t.Fatal("timed out waiting for the informer to be warned about as denied again")
	}

	// Check that the informer is not logged as permitted while it is still
	// denied. It would be once permittedFor passes without a denial, but the
	// failed retries during the sleep come often enough to prevent that.
	time.Sleep(permittedFor + 2*permittedInterval)

	if got := logs.FilterMessage("Permitted to list and watch now. No longer skipping").Len(); got != 1 {
		t.Fatalf("informer was logged as permitted %d times while denied again, want 1", got)
	}

	denied.Store(false)

	if !cache.WaitForCacheSync(ctx.Done(), func() bool {
		return logs.FilterMessage("Permitted to list and watch now. No longer skipping").Len() == 2
	}) {
		t.Fatal("timed out waiting for the informer to be logged as permitted again")
	}
}
