package sync

import (
	"context"
	"golang.org/x/sync/errgroup"
	"testing"
	"time"
)

func TestAddedChannelOutput(t *testing.T) {
	multiplexChannel := make(chan int)
	outputChannel1 := make(chan int)
	outputChannel2 := make(chan int)
	outputChannel3 := make(chan int)

	multiplexer := NewChannelSpreader[int](multiplexChannel)

	multiplexer.AddChannel(outputChannel1)
	multiplexer.AddChannel(outputChannel2)
	multiplexer.AddChannel(outputChannel3)

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		return multiplexer.Run(ctx)
	})

	want := 10

	multiplexChannel <- want

	if got := <-outputChannel1; got != want {
		t.Errorf("got '%d' for 1st test channel, wanted '%d'", got, want)
	}
	if got := <-outputChannel2; got != want {
		t.Errorf("got '%d' for 2nd test channel, wanted '%d'", got, want)
	}
	if got := <-outputChannel3; got != want {
		t.Errorf("got '%d' for 3rd test channel, wanted '%d'", got, want)
	}
}

func TestCreatedChannelOutput(t *testing.T) {
	multiplexChannel := make(chan int)

	multiplexer := NewChannelSpreader[int](multiplexChannel)

	outputChannel1 := multiplexer.NewChannel()
	outputChannel2 := multiplexer.NewChannel()
	outputChannel3 := multiplexer.NewChannel()

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		return multiplexer.Run(ctx)
	})

	want := 10

	multiplexChannel <- want

	if got := <-outputChannel1; got != want {
		t.Errorf("got '%d' for 1st test channel, wanted '%d'", got, want)
	}
	if got := <-outputChannel2; got != want {
		t.Errorf("got '%d' for 2nd test channel, wanted '%d'", got, want)
	}
	if got := <-outputChannel3; got != want {
		t.Errorf("got '%d' for 3rd test channel, wanted '%d'", got, want)
	}
}

func TestClosedChannels(t *testing.T) {
	multiplexChannel := make(chan int)

	multiplexer := NewChannelSpreader[int](multiplexChannel)

	outputChannel1 := multiplexer.NewChannel()
	outputChannel2 := multiplexer.NewChannel()
	outputChannel3 := multiplexer.NewChannel()

	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return multiplexer.Run(ctx)
	})

	cancel()

	select {
	case <-outputChannel1:
	case <-time.After(time.Second):
		t.Error("1st channel is still open, should be closed")
	}

	select {
	case <-outputChannel2:
	case <-time.After(time.Second):
		t.Error("2nd channel is still open, should be closed")
	}

	select {
	case <-outputChannel3:
	case <-time.After(time.Second):
		t.Error("3rd channel is still open, should be closed")
	}

}
