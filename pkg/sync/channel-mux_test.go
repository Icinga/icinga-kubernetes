package sync

import (
	"context"
	"golang.org/x/sync/errgroup"
	"testing"
	"time"
)

type outputTest struct {
	arg1, want int
}

var outputTests = []outputTest{
	{0, 0},
	{5, 5},
	{35253, 35253},
	{999999, 999999},
	{-7, -7},
}

func TestAddedOutputChannels(t *testing.T) {
	for _, test := range outputTests {
		multiplexChannel := make(chan int)
		multiplexer := NewChannelMux(multiplexChannel)

		outputChannel1 := make(chan int)
		outputChannel2 := make(chan int)
		outputChannel3 := make(chan int)
		multiplexer.AddOut(outputChannel1)
		multiplexer.AddOut(outputChannel2)
		multiplexer.AddOut(outputChannel3)

		g, ctx := errgroup.WithContext(context.Background())

		g.Go(func() error {
			return multiplexer.Run(ctx)
		})

		multiplexChannel <- test.arg1

		if got := <-outputChannel1; got != test.want {
			t.Errorf("got '%d' for 1st test channel, wanted '%d'", got, test.want)
		}
		if got := <-outputChannel2; got != test.want {
			t.Errorf("got '%d' for 2nd test channel, wanted '%d'", got, test.want)
		}
		if got := <-outputChannel3; got != test.want {
			t.Errorf("got '%d' for 3rd test channel, wanted '%d'", got, test.want)
		}
	}
}

func TestCreatedOutputChannels(t *testing.T) {
	for _, test := range outputTests {
		multiplexChannel := make(chan int)
		multiplexer := NewChannelMux(multiplexChannel)

		outputChannel1 := multiplexer.Out()
		outputChannel2 := multiplexer.Out()
		outputChannel3 := multiplexer.Out()

		g, ctx := errgroup.WithContext(context.Background())

		g.Go(func() error {
			return multiplexer.Run(ctx)
		})

		multiplexChannel <- test.arg1

		if got := <-outputChannel1; got != test.want {
			t.Errorf("got '%d' for 1st test channel, wanted '%d'", got, test.want)
		}
		if got := <-outputChannel2; got != test.want {
			t.Errorf("got '%d' for 2nd test channel, wanted '%d'", got, test.want)
		}
		if got := <-outputChannel3; got != test.want {
			t.Errorf("got '%d' for 3rd test channel, wanted '%d'", got, test.want)
		}
	}
}

type inputTest struct {
	arg1, arg2, arg3, want int
}

var inputTests = []inputTest{
	{0, 0, 0, 0},
	{1, 2, 3, 6},
	{535, 64, 6432, 7031},
	{353632, 636232, 64674, 1054538},
	{-1, -2, -3, -6},
}

func TestAddedInputChannels(t *testing.T) {
	for _, test := range inputTests {
		multiplexChannel1 := make(chan int)
		multiplexChannel2 := make(chan int)
		multiplexChannel3 := make(chan int)

		multiplexer := NewChannelMux(multiplexChannel1, multiplexChannel2, multiplexChannel3)

		outputChannel := multiplexer.Out()

		ctx, cancel := context.WithCancel(context.Background())
		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			return multiplexer.Run(ctx)
		})

		g.Go(func() error {
			select {
			case multiplexChannel1 <- test.arg1:
			case <-ctx.Done():
				return ctx.Err()
			}

			select {
			case multiplexChannel2 <- test.arg2:
			case <-ctx.Done():
				return ctx.Err()
			}

			select {
			case multiplexChannel3 <- test.arg3:
			case <-ctx.Done():
				return ctx.Err()
			}

			close(multiplexChannel1)
			close(multiplexChannel2)
			close(multiplexChannel3)

			return nil
		})

		stop := false
		got := 0

		g.Go(func() error {
			for !stop {
				select {
				case output, more := <-outputChannel:
					if !more {
						stop = true
						break
					}

					got += output
				case <-time.After(time.Second * 1):
					stop = true
					break
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			if got != test.want {
				t.Errorf("Got %d, wanted %d", got, test.want)
			}

			cancel()

			return nil
		})

		g.Wait()
	}
}

func TestClosedChannels(t *testing.T) {
	multiplexChannel := make(chan int)
	multiplexer := NewChannelMux(multiplexChannel)

	outputChannel1 := multiplexer.Out()
	outputChannel2 := multiplexer.Out()
	outputChannel3 := multiplexer.Out()

	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return multiplexer.Run(ctx)
	})

	cancel()

	select {
	case <-outputChannel1:
	case <-time.After(time.Second):
		t.Error("1st channel is still open, should be closed")
	}

	select {
	case <-outputChannel2:
	case <-time.After(time.Second):
		t.Error("2nd channel is still open, should be closed")
	}

	select {
	case <-outputChannel3:
	case <-time.After(time.Second):
		t.Error("3rd channel is still open, should be closed")
	}

}
