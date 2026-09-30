package v1

import (
	"context"
	"errors"
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

func TestSinkUpsertForwardsLiveItem(t *testing.T) {
	sink := NewSink(
		func(item *Item) any {
			return item.Key
		},
		func(key any) any {
			return key
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- sink.Upsert(
			ctx,
			sinkTestItem("live", nil),
		)
	}()

	select {
	case got := <-sink.UpsertCh():
		if got != "live" {
			t.Fatalf(
				"unexpected forwarded value: got %v, want live",
				got,
			)
		}
	case <-ctx.Done():
		t.Fatalf(
			"live upsert was not forwarded: %v",
			ctx.Err(),
		)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf(
				"live upsert returned error: %v",
				err,
			)
		}
	case <-ctx.Done():
		t.Fatalf(
			"live upsert did not return: %v",
			ctx.Err(),
		)
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

func TestSinkUpsertForwardsItemWithFutureDeletionTimestamp(t *testing.T) {
	sink := NewSink(
		func(item *Item) any {
			return item.Key
		},
		func(key any) any {
			return key
		},
	)

	deletionTimestamp := kmetav1.NewTime(time.Now().Add(time.Minute))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- sink.Upsert(ctx, sinkTestItem("future-terminating", &deletionTimestamp))
	}()

	select {
	case got := <-sink.UpsertCh():
		if got != "future-terminating" {
			t.Fatalf("unexpected forwarded value: got %v, want future-terminating", got)
		}
	case <-ctx.Done():
		t.Fatalf("future terminating upsert was not forwarded: %v", ctx.Err())
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("future terminating upsert returned error: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("future terminating upsert did not return: %v", ctx.Err())
	}
}

func TestSinkUpsertReturnsCanceledContext(t *testing.T) {
	sink := NewSink(
		func(item *Item) any {
			return item.Key
		},
		func(key any) any {
			return key
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sink.Upsert(
		ctx,
		sinkTestItem("canceled", nil),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"canceled upsert error: got %v, want %v",
			err,
			context.Canceled,
		)
	}

	select {
	case got := <-sink.UpsertCh():
		t.Fatalf(
			"canceled upsert was forwarded unexpectedly: %v",
			got,
		)
	default:
	}
}
