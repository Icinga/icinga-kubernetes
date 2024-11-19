package internal

import (
	"context"
	"golang.org/x/sync/errgroup"
	"sync/atomic"
)

// ChannelMultiplexer is a multiplexer for channels of variable types.
// It fans out all input channels to all output channels.
type ChannelMultiplexer[T any] interface {
	// In adds the given input channel reading.
	In() chan<- T

	AddIn(chan T)

	// Out returns a new output channel that receives from all input channels.
	Out() <-chan T

	// AddOut registers the given output channel to receive from all input channels.
	AddOut(chan T)

	// Run starts multiplexing of all input channels to all output channels.
	// Once run is called, can't be modified and will panic.
	Run(context.Context) error
}

// NewChannelMux returns a new ChannelMultiplexer initialized with at least one input channel.
func NewChannelMux[T any](inChannels ...chan T) ChannelMultiplexer[T] {
	return &channelMultiplexer[T]{
		inAdded: inChannels,
	}
}

type channelMultiplexer[T any] struct {
	in       []chan T
	inAdded  []chan T
	out      []chan T
	outAdded []chan T
	started  atomic.Bool
}

func (mux *channelMultiplexer[T]) In() chan<- T {
	if mux.started.Load() {
		panic("channelMultiplexer already started")
	}

	channel := make(chan T)

	mux.in = append(mux.in, channel)

	return channel
}

func (mux *channelMultiplexer[T]) AddIn(channel chan T) {
	if mux.started.Load() {
		panic("channelMultiplexer already started")
	}

	mux.inAdded = append(mux.inAdded, channel)
}

func (mux *channelMultiplexer[T]) Out() <-chan T {
	if mux.started.Load() {
		panic("channelMultiplexer already started")
	}

	channel := make(chan T)
	mux.out = append(mux.out, channel)

	return channel
}

func (mux *channelMultiplexer[T]) AddOut(channel chan T) {
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
		for _, channelToClose := range mux.in {
			close(channelToClose)
		}

		for _, channelToClose := range mux.out {
			close(channelToClose)
		}
	}()

	if len(mux.in)+len(mux.inAdded) == 0 {
		if len(mux.out)+len(mux.outAdded) > 0 {
			panic("foobar")
		}

		return nil
	}

	g, ctx := errgroup.WithContext(ctx)

	sink := make(chan T)
	defer close(sink)

	for _, ch := range mux.in {
		ch := ch

		g.Go(func() error {
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
