package v1

import (
	"context"
	"testing"
	"time"

	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
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
}
