package sync

import (
	"context"
	"sync/atomic"
)

// ChannelMux is a multiplexer for channels of variable types.
// It fans all input channels to all output channels.
type ChannelMux[T any] interface {

	// AddInChannel adds given input channel to the list of input channels.
	AddInChannel(<-chan T)

	// NewOutChannel returns and adds new output channel to the pods of created addedOutChannels.
	NewOutChannel() <-chan T

	// AddOutChannel adds given output channel to the list of added addedOutChannels.
	AddOutChannel(chan<- T)

	// Run combines output channel lists and starts multiplexing.
	Run(context.Context) error
}

type channelMux[T any] struct {
	inChannels         []<-chan T
	createdOutChannels []chan<- T
	addedOutChannels   []chan<- T
	started            atomic.Bool
}

// NewChannelMux creates new ChannelMux initialized with at least one input channel
func NewChannelMux[T any](initInChannel <-chan T, inChannels ...<-chan T) ChannelMux[T] {
	return &channelMux[T]{
		inChannels: append(make([]<-chan T, 0), append(inChannels, initInChannel)...),
	}
}

func (mux *channelMux[T]) AddInChannel(channel <-chan T) {
	if mux.started.Load() {
		panic("channelMux already started")
	}

	mux.inChannels = append(mux.inChannels, channel)
}

func (mux *channelMux[T]) NewOutChannel() <-chan T {
	if mux.started.Load() {
		panic("channelMux already started")
	}

	channel := make(chan T)
	mux.createdOutChannels = append(mux.createdOutChannels, channel)

	return channel
}

func (mux *channelMux[T]) AddOutChannel(channel chan<- T) {
	if mux.started.Load() {
		panic("channelMux already started")
	}

	mux.addedOutChannels = append(mux.addedOutChannels, channel)
}

func (mux *channelMux[T]) Run(ctx context.Context) error {
	mux.started.Store(true)

	defer func() {
		for _, channelToClose := range mux.createdOutChannels {
			close(channelToClose)
		}
	}()

	outChannels := append(mux.addedOutChannels, mux.createdOutChannels...)

	for {
		for _, inChannel := range mux.inChannels {
			select {
			case spread, more := <-inChannel:
				if !more {
					return nil
				}

				for _, outChannel := range outChannels {
					select {
					case outChannel <- spread:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
