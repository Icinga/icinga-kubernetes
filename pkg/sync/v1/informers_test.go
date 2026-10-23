package v1

import (
	"context"
	"reflect"
	"testing"
	"time"

	kappsv1 "k8s.io/api/apps/v1"
	kbatchv1 "k8s.io/api/batch/v1"
	kcorev1 "k8s.io/api/core/v1"
	kdiscoveryv1 "k8s.io/api/discovery/v1"
	keventsv1 "k8s.io/api/events/v1"
	knetworkingv1 "k8s.io/api/networking/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestInformers(t *testing.T) {
	client := fake.NewClientset(
		&kcorev1.Pod{Namespace: "a", Name: "pod"},
		&kcorev1.Pod{Namespace: "b", Name: "pod"},
		&kcorev1.Pod{Namespace: "c", Name: "pod"},
	)

	var i TypedInformers[*kcorev1.Pod]
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

	if n := len(i.List(labels.Everything())); n != 2 {
		t.Errorf("List() returned %d items, want 2", n)
	}
}

func TestNewTypedInformers(t *testing.T) {
	f := informers.NewSharedInformerFactory(fake.NewClientset(), 0)

	testNewTypedInformers[*kappsv1.DaemonSet](t, f, f.Apps().V1().DaemonSets().Informer())
	testNewTypedInformers[*kappsv1.Deployment](t, f, f.Apps().V1().Deployments().Informer())
	testNewTypedInformers[*kappsv1.ReplicaSet](t, f, f.Apps().V1().ReplicaSets().Informer())
	testNewTypedInformers[*kappsv1.StatefulSet](t, f, f.Apps().V1().StatefulSets().Informer())
	testNewTypedInformers[*kbatchv1.CronJob](t, f, f.Batch().V1().CronJobs().Informer())
	testNewTypedInformers[*kbatchv1.Job](t, f, f.Batch().V1().Jobs().Informer())
	testNewTypedInformers[*kcorev1.ConfigMap](t, f, f.Core().V1().ConfigMaps().Informer())
	testNewTypedInformers[*kcorev1.Namespace](t, f, f.Core().V1().Namespaces().Informer())
	testNewTypedInformers[*kcorev1.Node](t, f, f.Core().V1().Nodes().Informer())
	testNewTypedInformers[*kcorev1.PersistentVolumeClaim](t, f, f.Core().V1().PersistentVolumeClaims().Informer())
	testNewTypedInformers[*kcorev1.PersistentVolume](t, f, f.Core().V1().PersistentVolumes().Informer())
	testNewTypedInformers[*kcorev1.Pod](t, f, f.Core().V1().Pods().Informer())
	testNewTypedInformers[*kcorev1.Secret](t, f, f.Core().V1().Secrets().Informer())
	testNewTypedInformers[*kcorev1.Service](t, f, f.Core().V1().Services().Informer())
	testNewTypedInformers[*kdiscoveryv1.EndpointSlice](t, f, f.Discovery().V1().EndpointSlices().Informer())
	testNewTypedInformers[*keventsv1.Event](t, f, f.Events().V1().Events().Informer())
	testNewTypedInformers[*knetworkingv1.Ingress](t, f, f.Networking().V1().Ingresses().Informer())

	t.Run("UnknownType", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("NewTypedInformers[*kmetav1.ObjectMeta]() did not panic")
			}
		}()

		NewTypedInformers[*kmetav1.ObjectMeta](f)
	})
}

// testNewTypedInformers checks that NewTypedInformers returns the same informer for T as the typed accessor of f.
// It compares the stores because each call of a typed accessor wraps the informer anew.
func testNewTypedInformers[T kmetav1.Object](t *testing.T, f informers.SharedInformerFactory, want cache.SharedIndexInformer) {
	t.Helper()

	got := NewTypedInformers[T](f)
	if len(got) != 1 || got[0].GetStore() != want.GetStore() {
		t.Errorf("NewTypedInformers[%v]() does not return the informer of the factory", reflect.TypeFor[T]())
	}
}
