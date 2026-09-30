package multiplex

import (
	"context"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// ChannelMultiplexer is a multiplexer for channels of variable types.
// It fans out all input channels to all output channels.
type ChannelMultiplexer[T any] interface {
	// In returns a new input channel. The sender is responsible for closing it.
	In() chan<- T

	// AddIn registers the given input channel. The sender is responsible for closing it.
	AddIn(<-chan T)

	// Out returns a new output channel that receives from all input channels.
	// Run closes it on return.
	Out() <-chan T

	// AddOut registers the given output channel to receive from all input
	// channels. Run doesn't close it, as other senders may share it, so close
	// it only once Run has returned.
	AddOut(chan<- T)

	// Run starts multiplexing of all input channels to all output channels.
	// It returns once the context is done or all input channels are closed.
	// Once run is called, cannot be modified and will panic.
	Run(context.Context) error
}

// NewChannelMux returns a new ChannelMultiplexer initialized with at least one input channel.
func NewChannelMux[T any](inChannels ...<-chan T) ChannelMultiplexer[T] {
	return &channelMultiplexer[T]{
		in: inChannels,
	}
}

type channelMultiplexer[T any] struct {
	in       []<-chan T
	out      []chan<- T
	outAdded []chan<- T
	started  atomic.Bool
}

func (mux *channelMultiplexer[T]) In() chan<- T {
	channel := make(chan T)
	mux.AddIn(channel)

	return channel
}

func (mux *channelMultiplexer[T]) AddIn(channel <-chan T) {
	if mux.started.Load() {
		panic("channelMultiplexer already started")
	}

	mux.in = append(mux.in, channel)
}

func (mux *channelMultiplexer[T]) Out() <-chan T {
	if mux.started.Load() {
		panic("channelMultiplexer already started")
	}

	channel := make(chan T)
	mux.out = append(mux.out, channel)

	return channel
}

func (mux *channelMultiplexer[T]) AddOut(channel chan<- T) {
	if mux.started.Load() {
		panic("channelMultiplexer already started")
	}

	mux.outAdded = append(mux.outAdded, channel)
}

func (mux *channelMultiplexer[T]) Run(ctx context.Context) error {
	if mux.started.Swap(true) {
		panic("channelMultiplexer already started")
	}

	defer func() {
		for _, channelToClose := range mux.out {
			close(channelToClose)
		}
	}()

	if len(mux.in) == 0 {
		if len(mux.out)+len(mux.outAdded) > 0 {
			panic("foobar")
		}

		return nil
	}

	g, ctx := errgroup.WithContext(ctx)

	sink := make(chan T)

	var readers sync.WaitGroup
	for _, ch := range mux.in {
		readers.Add(1)
		g.Go(func() error {
			defer readers.Done()
			for {
				select {
				case spread, more := <-ch:
					if !more {
						return nil
					}
					select {
					case sink <- spread:
					case <-ctx.Done():
						return ctx.Err()
					}

				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

	// The readers are the only senders on sink. Closing it once they're done
	// lets the fan-out below return, so Run ends when all inputs are closed.
	g.Go(func() error {
		readers.Wait()
		close(sink)

		return nil
	})

	outs := append(mux.outAdded, mux.out...)
	g.Go(func() error {
		for {
			select {
			case spread, more := <-sink:
				if !more {
					return nil
				}

				for _, ch := range outs {
					select {
					case ch <- spread:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return g.Wait()
}
