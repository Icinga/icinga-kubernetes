package sync

import (
	"context"
	"github.com/pkg/errors"
	"sync/atomic"
)

type ChannelSpreader[T any] struct {
	channelToBreak <-chan T
	channels       []chan<- T
	started        atomic.Bool
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
	cs.channels = append(cs.channels, channel)

	return channel
}

func (cs *ChannelSpreader[T]) Run(ctx context.Context) error {

	cs.started.Store(true)

	defer func() {
		for _, channelToClose := range cs.channels {
			close(channelToClose)
		}
	}()

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
					return errors.Wrap(ctx.Err(), "context canceled spread to channels")
				}
			}
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context canceled read spread channel")
		}
	}
}
