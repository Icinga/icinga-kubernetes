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
