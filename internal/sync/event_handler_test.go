package sync

import (
	"context"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-kubernetes/internal/schema"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	oldUid = ktypes.UID("11111111-1111-1111-1111-111111111111")
	newUid = ktypes.UID("22222222-2222-2222-2222-222222222222")
)

func newDeployment(uid ktypes.UID) *appsv1.Deployment {
	meta := kmetav1.ObjectMeta{Namespace: "ns", Name: "name", UID: uid}
	return &appsv1.Deployment{
		ObjectMeta: meta,
	}
}

func newTestHandler() (*EventHandler, workqueue.TypedInterface[EventHandlerItem]) {
	queue := workqueue.NewTyped[EventHandlerItem]()

	return &EventHandler{queue: queue, log: testLogger()}, queue
}

func testLogger() *logging.Logger {
	return logging.NewLogger(zap.NewNop().Sugar(), time.Second)
}

// drain empties the queue and returns the items in the order they were added.
// It checks Len() first because Get() blocks forever on an empty queue.
func drain(queue workqueue.TypedInterface[EventHandlerItem]) []EventHandlerItem {
	var items []EventHandlerItem
	for queue.Len() > 0 {
		item, _ := queue.Get()
		queue.Done(item)
		items = append(items, item)
	}

	return items
}

func TestEventHandlerOnUpdateSameUid(t *testing.T) {
	handler, queue := newTestHandler()

	handler.OnUpdate(newDeployment(oldUid), newDeployment(oldUid))

	got := drain(queue)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 item, got %d: %+v", len(got), got)
	}
	if got[0].Type != EventUpdate {
		t.Errorf("expected Type %q, got %q", EventUpdate, got[0].Type)
	}
}

func TestEventHandlerOnUpdateChangedUid(t *testing.T) {
	handler, queue := newTestHandler()

	handler.OnUpdate(newDeployment(oldUid), newDeployment(newUid))

	want := []EventHandlerItem{
		{Type: EventDelete, Id: schema.EnsureUUID(oldUid), KKey: "ns/name"},
		{Type: EventUpdate, Id: schema.EnsureUUID(newUid), KKey: "ns/name"},
	}

	got := drain(queue)
	if len(got) != len(want) {
		t.Fatalf("expected %d items, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: expected %+v, got %+v", i, want[i], got[i])
		}
	}
}

func deploymentList(uid ktypes.UID, resourceVersion string) *appsv1.DeploymentList {
	return &appsv1.DeploymentList{
		ListMeta: kmetav1.ListMeta{ResourceVersion: resourceVersion},
		Items: []appsv1.Deployment{
			{ObjectMeta: kmetav1.ObjectMeta{Namespace: "ns", Name: "api", UID: uid}},
		},
	}
}

// expiredWatch returns a watch whose first event is "410 Gone: resource version
// too old", which makes the reflector drop the watch and list again.
func expiredWatch() watch.Interface {
	// Buffered, so Error() doesn't block until the reflector reads it.
	fake := watch.NewFakeWithOptions(watch.FakeOptions{ChannelSize: 1})
	fake.Error(&kmetav1.Status{
		Status: kmetav1.StatusFailure,
		Code:   http.StatusGone,
		Reason: kmetav1.StatusReasonExpired,
	})

	return fake
}

// waitForItems polls the queue until it has collected n items.
func waitForItems(t *testing.T, queue workqueue.TypedInterface[EventHandlerItem], n int) []EventHandlerItem {
	t.Helper()

	var items []EventHandlerItem
	for deadline := time.Now().Add(15 * time.Second); len(items) < n; {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d items, got %d: %+v", n, len(items), items)
		}
		time.Sleep(50 * time.Millisecond)
		items = append(items, drain(queue)...)
	}

	return items
}

// TestInformerRelistDeliversUidChangeAsUpdate checks the premise behind
// EventHandler.OnUpdate with a real informer: a Deployment deleted and recreated
// under the same name while the watch is broken shows up after the relist as ONE
// update whose old and new UIDs differ.
func TestInformerRelistDeliversUidChangeAsUpdate(t *testing.T) {
	// client-go lists via a streaming watch by default, which would bypass the
	// ListWithContextFunc below. A plain ListWatch can't opt out, so switch the
	// feature off for this test (restored automatically).
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	var lists, watches atomic.Int32
	listWatch := &cache.ListWatch{
		ListWithContextFunc: func(context.Context, kmetav1.ListOptions) (runtime.Object, error) {
			if lists.Add(1) == 1 {
				return deploymentList(oldUid, "1"), nil
			}

			return deploymentList(newUid, "2"), nil // the relist
		},
		WatchFuncWithContext: func(context.Context, kmetav1.ListOptions) (watch.Interface, error) {
			if watches.Add(1) == 1 {
				return expiredWatch(), nil
			}

			return watch.NewFake(), nil // silent, so nothing else happens
		},
	}

	queue := workqueue.NewTyped[EventHandlerItem]()
	t.Cleanup(queue.ShutDown)

	informer := cache.NewSharedIndexInformer(listWatch, &appsv1.Deployment{}, 0, cache.Indexers{})
	if _, err := informer.AddEventHandler(NewEventHandler(queue, testLogger())); err != nil {
		t.Fatal(err)
	}
	go informer.RunWithContext(t.Context())

	// The reflector backs off before relisting, so this takes about a second.
	got := waitForItems(t, queue, 3)

	want := []EventHandlerItem{
		{Type: EventAdd, Id: schema.EnsureUUID(oldUid), KKey: "ns/api"},    // 1st list
		{Type: EventDelete, Id: schema.EnsureUUID(oldUid), KKey: "ns/api"}, // relist: old UID is gone
		{Type: EventUpdate, Id: schema.EnsureUUID(newUid), KKey: "ns/api"}, // relist: new UID
	}
	if !slices.Equal(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}
