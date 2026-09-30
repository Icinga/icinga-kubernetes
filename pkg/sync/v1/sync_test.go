package v1

import (
	"context"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/types"
	schemav1 "github.com/icinga/icinga-kubernetes/pkg/schema/v1"
	kcorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestShouldPurgeDeleted(t *testing.T) {
	tests := []struct {
		name     string
		features []Feature
		want     bool
	}{
		{name: "DisabledByDefault", want: false},
		{name: "Enabled", features: []Feature{WithDeletedRetention(time.Hour)}, want: true},
		{name: "NoDeleteWins", features: []Feature{WithDeletedRetention(time.Hour), WithNoDelete()}, want: false},
		{name: "InvalidRetentionDelegatesToDatabase", features: []Feature{WithDeletedRetention(-time.Hour)}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldPurgeDeleted(NewFeatures(tt.features...))
			if got != tt.want {
				t.Fatalf(
					"expected purge decision %t, got %t",
					tt.want,
					got,
				)
			}
		})
	}
}

func TestBuildWarmupQueryExcludesDeleted(t *testing.T) {
	const base = "SELECT uuid FROM pod"
	const want = base + ` WHERE cluster_uuid=:cluster_uuid AND deleted IS NULL`

	if got := buildWarmupQuery(base); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// TestSyncDeleteVanished covers the cleanup that used to run on the informer's
// store, where it made listers panic on the entities warmup() had announced,
// see https://github.com/Icinga/icinga-kubernetes/issues/208.
func TestSyncDeleteVanished(t *testing.T) {
	existing := &kcorev1.Pod{
		Namespace: "default",
		Name:      "existing",
		UID:       "9f1c6a4e-0e0a-4e2b-9a5f-000000000001",
	}

	informer := informers.NewSharedInformerFactory(fake.NewClientset(existing), 0).Core().V1().Pods().Informer()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	go informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		t.Fatal("timed out waiting for cache sync")
	}

	sync := &Sync{informer: informer}

	t.Run("NothingSynced", func(t *testing.T) {
		if err := sync.deleteVanished(ctx, NewSink(nil, nil), nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("DeletesOnlyTheVanished", func(t *testing.T) {
		vanished := schemav1.EnsureUUID("9f1c6a4e-0e0a-4e2b-9a5f-000000000002")
		synced := map[string]types.UUID{
			"default/existing": schemav1.EnsureUUID(existing.UID),
			"default/vanished": vanished,
		}

		sink := NewSink(nil, func(k any) any { return k })

		errs := make(chan error, 1)
		go func() { errs <- sync.deleteVanished(ctx, sink, synced) }()

		var deleted []any
		for done := false; !done; {
			select {
			case id := <-sink.DeleteCh():
				deleted = append(deleted, id)
			case err := <-errs:
				if err != nil {
					t.Error(err)
				}

				done = true
			}
		}

		if len(deleted) != 1 || deleted[0].(types.UUID) != vanished {
			t.Fatalf("expected %v to be deleted, got %v", vanished, deleted)
		}
	})
}
