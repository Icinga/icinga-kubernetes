package sync

import (
	"context"
	"golang.org/x/sync/errgroup"
	"sync/atomic"
)

// ChannelMux is a multiplexer for channels of variable types.
// It fans out all input channels to all output channels.
type ChannelMux[T any] interface {
	// In adds the given input channel reading.
	In(<-chan T)

	// Out returns a new output channel that receives from all input channels.
	Out() <-chan T

	// AddOut registers the given output channel to receive from all input channels.
	AddOut(chan<- T)

	// Run starts multiplexing of all input channels to all output channels.
	Run(context.Context) error
}

type channelMux[T any] struct {
	in       []<-chan T
	out      []chan<- T
	outAdded []chan<- T
	started  atomic.Bool
}

// NewChannelMux returns a new ChannelMux initialized with at least one input channel.
func NewChannelMux[T any](inChannel <-chan T, inChannels ...<-chan T) ChannelMux[T] {
	return &channelMux[T]{
		in: append(inChannels, inChannel),
	}
}

func (mux *channelMux[T]) In(channel <-chan T) {
	if mux.started.Load() {
		panic("channelMux already started")
	}

	mux.in = append(mux.in, channel)
}

func (mux *channelMux[T]) Out() <-chan T {
	if mux.started.Load() {
		panic("channelMux already started")
	}

	channel := make(chan T)
	mux.out = append(mux.out, channel)

	return channel
}

func (mux *channelMux[T]) AddOut(channel chan<- T) {
	if mux.started.Load() {
		panic("channelMux already started")
	}

	mux.outAdded = append(mux.outAdded, channel)
}

func (mux *channelMux[T]) Run(ctx context.Context) error {
	if mux.started.Swap(true) {
		panic("channelMux already started")
	}

	defer func() {
		for _, channelToClose := range mux.out {
			close(channelToClose)
		}
	}()

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
