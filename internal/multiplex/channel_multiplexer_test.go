package multiplex

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
)

// TestChannelMultiplexerFansOutAddedInputs covers the inputs passed to
// NewChannelMux and AddIn, which Run used to ignore, so their senders blocked
// forever.
func TestChannelMultiplexerFansOutAddedInputs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		initial, added := make(chan int), make(chan int)

		mux := NewChannelMux(initial)
		mux.AddIn(added)

		// Run sends each value to the outputs one after another. Buffering this
		// one keeps Run from blocking on it while the test reads out first.
		addedOut := make(chan int, 2)
		mux.AddOut(addedOut)
		out := mux.Out()

		ctx, cancel := context.WithCancel(t.Context())
		errs := make(chan error, 1)
		go func() { errs <- mux.Run(ctx) }()
		go func() { initial <- 1 }()
		go func() { added <- 2 }()

		want := []int{1, 2}

		got := []int{<-out, <-out}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("out received %v, want %v", got, want)
		}

		got = []int{<-addedOut, <-addedOut}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("addedOut received %v, want %v", got, want)
		}

		cancel()
		if err := <-errs; !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v, want %v", err, context.Canceled)
		}
	})
}

// TestChannelMultiplexerReturnsOnceInputsAreClosed covers Run, which used to
// run until its context was done even after all its inputs were closed, and
// to close the channels from In a second time on return. Closing only some
// of the inputs mustn't end it.
func TestChannelMultiplexerReturnsOnceInputsAreClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		first := make(chan int)
		mux := NewChannelMux(first)
		second := mux.In()
		out := mux.Out()

		errs := make(chan error, 1)
		go func() { errs <- mux.Run(t.Context()) }()

		close(first)
		synctest.Wait()
		select {
		case err := <-errs:
			t.Fatalf("Run returned %v before all its inputs were closed", err)
		default:
		}

		second <- 1
		if got := <-out; got != 1 {
			t.Errorf("out received %d, want 1", got)
		}

		close(second)
		synctest.Wait()
		select {
		case err := <-errs:
			if err != nil {
				t.Errorf("Run returned %v, want nil", err)
			}
		default:
			t.Fatal("Run didn't return once its inputs were closed")
		}
	})
}
