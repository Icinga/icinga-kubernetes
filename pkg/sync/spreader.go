package sync

import (
	"context"
	"sync/atomic"
)

// ChannelSpreader takes a channel of type T and fans it out to an array of other addedChannels of type T
type ChannelSpreader[T any] struct {
	channelToBreak  <-chan T
	createdChannels []chan<- T
	addedChannels   []chan<- T
	started         atomic.Bool
}

// NewChannelSpreader creates new ChannelSpreader initialized with the channel to break
func NewChannelSpreader[T any](channelToBreak <-chan T) *ChannelSpreader[T] {
	return &ChannelSpreader[T]{
		channelToBreak: channelToBreak,
	}
}

// NewChannel returns and adds new output channel to the list of created addedChannels
func (cs *ChannelSpreader[T]) NewChannel() <-chan T {
	if cs.started.Load() {
		panic("ChannelSpreader already started")
	}

	channel := make(chan T)
	cs.createdChannels = append(cs.createdChannels, channel)

	return channel
}

// AddChannel adds given output channel to the list of added addedChannels
func (cs *ChannelSpreader[T]) AddChannel(channel chan<- T) {
	if cs.started.Load() {
		panic("ChannelSpreader already started")
	}

	cs.addedChannels = append(cs.addedChannels, channel)
}

// Run combines the lists and starts fanning out the channel to the addedChannels from the list
func (cs *ChannelSpreader[T]) Run(ctx context.Context) error {
	cs.started.Store(true)

	defer func() {
		for _, channelToClose := range cs.createdChannels {
			close(channelToClose)
		}
	}()

	channels := append(cs.addedChannels, cs.createdChannels...)

	for {
		select {
		case spread, more := <-cs.channelToBreak:
			if !more {
				return nil
			}

			for _, channel := range channels {
				select {
				case channel <- spread:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
