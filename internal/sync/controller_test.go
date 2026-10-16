package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/internal/kdatabase"
	"go.uber.org/zap"
	kcorev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// keyEntity is the database.Entity a test sink emits for an upserted object,
// identified by its key.
type keyEntity struct {
	kdatabase.EntityStub
	key string
}

func (e keyEntity) ID() database.ID { return e }

func (e keyEntity) String() string { return e.key }

// TestControllerStreamForbidden checks that a namespace whose pods may not be
// listed does not hold up streaming the pods of the others.
func TestControllerStreamForbidden(t *testing.T) {
	client := fake.NewClientset(
		&kcorev1.Pod{Namespace: "a", Name: "pod"},
		&kcorev1.Pod{Namespace: "b", Name: "pod"},
	)

	deny := kerrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("denied"))
	client.PrependReactor("list", "pods", func(a ktesting.Action) (bool, runtime.Object, error) {
		return a.GetNamespace() == "b", nil, deny
	})
	client.PrependWatchReactor("pods", func(a ktesting.Action) (bool, watch.Interface, error) {
		return a.GetNamespace() == "b", nil, deny
	})

	var i Informers
	for _, ns := range []string{"a", "b"} {
		factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace(ns))
		i = append(i, factory.Core().V1().Pods().Informer())
	}

	c := NewController(i, logging.NewLogger(zap.NewNop().Sugar(), time.Second))

	ctx, cancel := context.WithCancel(t.Context())
	sink := NewSink(func(i *Item) database.Entity { return keyEntity{key: i.Key} }, nil)

	errs := make(chan error, 1)
	go func() { errs <- c.Stream(ctx, sink) }()

	select {
	case key := <-sink.UpsertCh():
		if got := key.(keyEntity).key; got != "a/pod" {
			t.Fatalf("got %q, want a/pod", got)
		}
	case err := <-errs:
		t.Fatalf("Stream() returned early: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for a/pod")
	}

	// Streaming starts only once every informer has synced or was denied,
	// so the informer of b is known as denied by now.
	if _, denied := forbiddenInformers.Load(i[1]); !denied {
		t.Fatal("informer of b was not denied")
	}

	cancel()
	if err := <-errs; !errors.Is(err, context.Canceled) {
		t.Fatalf("Stream() = %v, want context.Canceled", err)
	}
}
