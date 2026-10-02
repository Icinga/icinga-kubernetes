package v1

import (
	"context"
	"testing"
	"time"

	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func sinkTestItem(key string, deletionTimestamp *kmetav1.Time) *Item {
	object := kmetav1.Object(&kmetav1.ObjectMeta{DeletionTimestamp: deletionTimestamp})

	return &Item{
		Key:  key,
		Item: &object,
	}
}
func TestSinkUpsertForwardsTerminatingItem(t *testing.T) {
	sink := NewSink(
		func(item *Item) any {
			return item.Key
		},
		func(key any) any {
			return key
		},
	)

	deletionTimestamp := kmetav1.NewTime(time.Now().Add(-time.Minute))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- sink.Upsert(ctx, sinkTestItem("terminating", &deletionTimestamp))
	}()

	select {
	case got := <-sink.UpsertCh():
		if got != "terminating" {
			t.Fatalf("unexpected forwarded value: got %v, want terminating", got)
		}
	case <-ctx.Done():
		t.Fatalf("terminating upsert was not forwarded: %v", ctx.Err())
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("terminating upsert returned error: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("terminating upsert did not return: %v", ctx.Err())
	}
}
