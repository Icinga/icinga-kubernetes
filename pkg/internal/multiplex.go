package internal

import (
	"context"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/runtime"
)

type Multiplex interface {
	In() chan interface{}
	Out() chan interface{}
	Do(context.Context) error
}

func NewMultiplex() Multiplex {
	return &multiplex{
		started: false,
		in:      make([]chan interface{}, 0, 1),
		out:     make([]chan interface{}, 0, 2),
	}
}

type multiplex struct {
	started bool
	in      []chan interface{}
	out     []chan interface{}
}

func (m *multiplex) In() chan interface{} {
	if m.started {
		panic("already started")
	}

	ch := make(chan interface{})
	m.in = append(m.in, ch)

	return ch
}

func (m *multiplex) Out() chan interface{} {
	if m.started {
		panic("already started")
	}

	ch := make(chan interface{})
	m.out = append(m.out, ch)

	return ch
}

func (m *multiplex) Do(ctx context.Context) error {
	m.started = true

	g, ctx := errgroup.WithContext(ctx)

	sink := make(chan interface{})
	defer close(sink)

	g.Go(func() error {
		defer runtime.HandleCrash()

		for {
			for _, in := range m.in {
				select {
				case item, more := <-in:
					if !more {
						return nil
					}

					select {
					case sink <- item:
					case <-ctx.Done():
						return ctx.Err()
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	})

	g.Go(func() error {
		defer runtime.HandleCrash()

		for {
			select {
			case item, more := <-sink:
				if !more {
					return nil
				}

				for _, out := range m.out {
					select {
					case out <- item:
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
