package sync

import (
	"context"
	"sync/atomic"
)

type ChannelSpreader[T any] struct {
	channelToBreak  <-chan T
	createdChannels []chan<- T
	channels        []chan<- T
	started         atomic.Bool
}

func NewChannelSpreader[T any](channelToBreak <-chan T) *ChannelSpreader[T] {
	return &ChannelSpreader[T]{
		channelToBreak: channelToBreak,
	}
}

func (cs *ChannelSpreader[T]) NewChannel() <-chan T {
	if cs.started.Load() == true {
		panic("ChannelSpreader already started")
	}

	channel := make(chan T)
	cs.createdChannels = append(cs.createdChannels, channel)

	return channel
}

func (cs *ChannelSpreader[T]) AddChannel(channel chan<- T) {
	if cs.started.Load() == true {
		panic("ChannelSpreader already started")
	}

	cs.channels = append(cs.channels, channel)
}

func (cs *ChannelSpreader[T]) Run(ctx context.Context) error {

	cs.started.Store(true)

	defer func() {
		for _, channelToClose := range cs.createdChannels {
			close(channelToClose)
		}
	}()

	cs.channels = append(cs.channels, cs.createdChannels...)

	for {
		select {
		case spread, more := <-cs.channelToBreak:
			if !more {
				return nil
			}

			for _, channel := range cs.channels {
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
