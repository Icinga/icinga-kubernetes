package v1

import (
	"context"
	"github.com/icinga/icinga-kubernetes/internal"
	"golang.org/x/sync/errgroup"
)

type EventsMultiplexer interface {
	UpsertEvents() internal.ChannelMultiplexer[any]
	DeleteEvents() internal.ChannelMultiplexer[any]
	Run(context.Context) error
}

type EventsMultiplexers interface {
	DaemonSets() EventsMultiplexer
	Deployments() EventsMultiplexer
	Nodes() EventsMultiplexer
	Pods() EventsMultiplexer
	ReplicaSets() EventsMultiplexer
	StatefulSets() EventsMultiplexer
	Run(context.Context) error
}

func Multiplexers() EventsMultiplexers {
	return m
}

type events struct {
	upsertEvents internal.ChannelMultiplexer[any]
	deleteEvents internal.ChannelMultiplexer[any]
}

func (e events) UpsertEvents() internal.ChannelMultiplexer[any] {
	return e.upsertEvents
}

func (e events) DeleteEvents() internal.ChannelMultiplexer[any] {
	return e.deleteEvents
}

func (e events) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return e.upsertEvents.Run(ctx)
	})

	g.Go(func() error {
		return e.deleteEvents.Run(ctx)
	})

	return g.Wait()
}

type multiplexers struct {
	daemonSets   events
	deployments  events
	nodes        events
	pods         events
	replicaSets  events
	statefulSets events
}

func (m multiplexers) DaemonSets() EventsMultiplexer {
	return m.daemonSets
}

func (m multiplexers) Deployments() EventsMultiplexer {
	return m.deployments
}

func (m multiplexers) Nodes() EventsMultiplexer {
	return m.nodes
}

func (m multiplexers) Pods() EventsMultiplexer {
	return m.pods
}

func (m multiplexers) ReplicaSets() EventsMultiplexer {
	return m.replicaSets
}

func (m multiplexers) StatefulSets() EventsMultiplexer {
	return m.statefulSets
}

func (m multiplexers) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return m.daemonSets.Run(ctx)
	})

	g.Go(func() error {
		return m.deployments.Run(ctx)
	})

	g.Go(func() error {
		return m.nodes.Run(ctx)
	})

	g.Go(func() error {
		return m.pods.Run(ctx)
	})

	g.Go(func() error {
		return m.replicaSets.Run(ctx)
	})

	g.Go(func() error {
		return m.statefulSets.Run(ctx)
	})

	return g.Wait()
}

var m multiplexers

func init() {
	m = multiplexers{
		daemonSets: events{
			upsertEvents: internal.NewChannelMux[any](),
			deleteEvents: internal.NewChannelMux[any](),
		},
		deployments: events{
			upsertEvents: internal.NewChannelMux[any](),
			deleteEvents: internal.NewChannelMux[any](),
		},
		nodes: events{
			upsertEvents: internal.NewChannelMux[any](),
			deleteEvents: internal.NewChannelMux[any](),
		},
		pods: events{
			upsertEvents: internal.NewChannelMux[any](),
			deleteEvents: internal.NewChannelMux[any](),
		},
		replicaSets: events{
			upsertEvents: internal.NewChannelMux[any](),
			deleteEvents: internal.NewChannelMux[any](),
		},
		statefulSets: events{
			upsertEvents: internal.NewChannelMux[any](),
			deleteEvents: internal.NewChannelMux[any](),
		},
	}
}
