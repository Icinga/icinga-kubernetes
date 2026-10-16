package v1

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/pkg/kdatabase"
	"go.uber.org/zap"
	kcorev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestControllerStreamForbidden(t *testing.T) {
	newController := func(forbidden string) *Controller {
		client := fake.NewClientset(
			&kcorev1.Pod{ObjectMeta: kmetav1.ObjectMeta{Namespace: "a", Name: "pod"}},
			&kcorev1.Pod{ObjectMeta: kmetav1.ObjectMeta{Namespace: "b", Name: "pod"}},
		)

		deny := kerrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("denied"))
		client.PrependReactor("list", "pods", func(a ktesting.Action) (bool, runtime.Object, error) {
			return a.GetNamespace() == forbidden, nil, deny
		})
		client.PrependWatchReactor("pods", func(a ktesting.Action) (bool, watch.Interface, error) {
			return a.GetNamespace() == forbidden, nil, deny
		})

		var i Informers
		for _, ns := range []string{"a", "b"} {
			factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace(ns))
			i = append(i, factory.Core().V1().Pods().Informer())
		}

		return NewController(i, logging.NewLogger(zap.NewNop().Sugar(), time.Second))
	}

	t.Run("Forbidden", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()

		err := newController("b").Stream(ctx, NewSink(func(i *Item) database.Entity { return keyEntity{key: i.Key} }, nil))
		if !kerrors.IsForbidden(err) {
			t.Fatalf("Stream() = %v, want forbidden error", err)
		}
	})

	t.Run("Permitted", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		sink := NewSink(func(i *Item) database.Entity { return keyEntity{key: i.Key} }, nil)

		errs := make(chan error, 1)
		go func() { errs <- newController("").Stream(ctx, sink) }()

		// Both pods arriving means the initial list succeeded and the stream is running.
		for range 2 {
			select {
			case <-sink.UpsertCh():
			case err := <-errs:
				t.Fatalf("Stream() returned early: %v", err)
			case <-time.After(30 * time.Second):
				t.Fatal("timed out waiting for pods")
			}
		}

		cancel()
		if err := <-errs; !errors.Is(err, context.Canceled) {
			t.Fatalf("Stream() = %v, want context.Canceled", err)
		}
	})
}
