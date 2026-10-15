package v1

import (
	"context"
	"testing"
	"time"

	kcorev1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestInformers(t *testing.T) {
	client := fake.NewClientset(
		&kcorev1.Pod{ObjectMeta: kmetav1.ObjectMeta{Namespace: "a", Name: "pod"}},
		&kcorev1.Pod{ObjectMeta: kmetav1.ObjectMeta{Namespace: "b", Name: "pod"}},
		&kcorev1.Pod{ObjectMeta: kmetav1.ObjectMeta{Namespace: "c", Name: "pod"}},
	)

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

	for key, want := range map[string]bool{"a/pod": true, "b/pod": true, "c/pod": false} {
		if _, exists, err := i.GetByKey(key); err != nil || exists != want {
			t.Errorf("GetByKey(%q) = %v, %v, want %v, nil", key, exists, err, want)
		}
	}

	if n := len(i.List()); n != 2 {
		t.Errorf("List() returned %d items, want 2", n)
	}
}
