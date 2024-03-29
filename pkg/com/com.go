package com

import (
	"context"
	"golang.org/x/sync/errgroup"
)

// WaitAsync calls Wait() on the passed Waiter in a new goroutine and
// sends the first non-nil error (if any) to the returned channel.
// The returned channel is always closed when the Waiter is done.
func WaitAsync(ctx context.Context, w Waiter) <-chan error {
	errs := make(chan error, 1)

	go func() {
		defer close(errs)

		if e := w.Wait(); e != nil {
			select {
			case errs <- e:
			case <-ctx.Done():
			}

		}
	}()

	return errs
}

// ErrgroupReceive adds a goroutine to the specified group that
// returns the first non-nil error (if any) from the specified channel.
// If the channel is closed, it will return nil.
func ErrgroupReceive(ctx context.Context, g *errgroup.Group, err <-chan error) {
	g.Go(func() error {
		select {
		case e := <-err:
			return e
		case <-ctx.Done():
			return ctx.Err()
		}
	})
}

// CopyFirst asynchronously forwards all items from input to forward and synchronously returns the first item.
func CopyFirst[T any](
	ctx context.Context, input <-chan T,
) (first T, forward <-chan T, err error) {
	var ok bool
	select {
	case <-ctx.Done():
		err = ctx.Err()

		return
	case first, ok = <-input:
	}

	if !ok {
		return
	}

	// Buffer of one because we receive an item and send it back immediately.
	fwd := make(chan T, 1)
	fwd <- first

	forward = fwd

	go func() {
		defer close(fwd)

		for {
			select {
			case i, more := <-input:
				if !more {
					return
				}

				select {
				case fwd <- i:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return
}
