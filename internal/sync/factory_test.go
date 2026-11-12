package sync

import (
	"context"
	"testing"
	"time"

	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestFactory(t *testing.T) {
	objects := []runtime.Object{
		&kcorev1.Namespace{Name: "a", Labels: map[string]string{kcorev1.LabelMetadataName: "a"}},
		&kcorev1.Namespace{Name: "b", Labels: map[string]string{kcorev1.LabelMetadataName: "b"}},
		&kcorev1.Namespace{Name: "c", Labels: map[string]string{kcorev1.LabelMetadataName: "c"}},
		&kcorev1.Pod{Namespace: "a", Name: "pod"},
		&kcorev1.Pod{Namespace: "b", Name: "pod"},
		&kcorev1.Pod{Namespace: "c", Name: "pod"},
		&kcorev1.Node{Name: "node"},
	}

	tests := map[string]struct {
		include, exclude []string
		podInformers     int
		pods, namespaces int
	}{
		"All":     {podInformers: 1, pods: 3, namespaces: 3},
		"Include": {include: []string{"a", "b"}, podInformers: 2, pods: 2, namespaces: 2},
		"Exclude": {exclude: []string{"c"}, podInformers: 1, pods: 2, namespaces: 2},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f, err := NewFactory(newClientset(objects...), tt.include, tt.exclude)
			if err != nil {
				t.Fatal(err)
			}

			pods, namespaces, nodes := f.Pods(), f.Namespaces(), f.Nodes()

			if len(pods) != tt.podInformers {
				t.Errorf("Pods() has %d informers, want %d", len(pods), tt.podInformers)
			}

			if len(nodes) != 1 {
				t.Errorf("Nodes() has %d informers, want 1", len(nodes))
			}

			run(t, pods, namespaces, nodes)

			if n := len(pods.List()); n != tt.pods {
				t.Errorf("Pods() lists %d pods, want %d", n, tt.pods)
			}

			if n := len(namespaces.List()); n != tt.namespaces {
				t.Errorf("Namespaces() lists %d namespaces, want %d", n, tt.namespaces)
			}

			if n := len(nodes.List()); n != 1 {
				t.Errorf("Nodes() lists %d nodes, want 1", n)
			}
		})
	}
}

// newClientset returns a fake clientset that, unlike the plain one, applies the
// field selectors of list requests.
func newClientset(objects ...runtime.Object) *fake.Clientset {
	clientset := fake.NewClientset(objects...)
	clientset.PrependReactor("list", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
		selector := action.(ktesting.ListAction).GetListRestrictions().Fields
		if selector.Empty() {
			return false, nil, nil
		}

		_, list, err := ktesting.ObjectReaction(clientset.Tracker())(action)
		if err != nil {
			return true, nil, err
		}

		items, err := meta.ExtractList(list)
		if err != nil {
			return true, nil, err
		}

		var matching []runtime.Object
		for _, item := range items {
			o, err := meta.Accessor(item)
			if err != nil {
				return true, nil, err
			}

			if selector.Matches(fields.Set{"metadata.name": o.GetName(), "metadata.namespace": o.GetNamespace()}) {
				matching = append(matching, item)
			}
		}

		return true, list, meta.SetList(list, matching)
	})

	return clientset
}

// run runs informers until the test ends and waits for them to sync.
func run(t *testing.T, informers ...Informers) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	for _, i := range informers {
		go i.Run(ctx.Done())

		if !cache.WaitForCacheSync(ctx.Done(), i.HasSynced) {
			t.Fatal("timed out waiting for cache sync")
		}
	}
}
