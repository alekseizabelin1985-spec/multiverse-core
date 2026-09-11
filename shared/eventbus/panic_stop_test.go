package eventbus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

// The reason the cause of a dead letter that could not be written is printed
// with %v and not wrapped with %w (reviews #1 and #2 of T-426; T-415). stopped
// takes any io.EOF or io.ErrClosedPipe in the chain of an error for the end of
// the reader, and panicError keeps the panic value matchable, so with %w a
// handler that panicked with one of them — or returned an error wrapping one —
// would turn a failed dead letter into a silent nil return of the kafka
// Subscribe while its context is alive. The event would stay uncommitted and
// the consumer would simply stop.
//
// If the chain ever has to carry more, wrap ErrHandlerPanic alone and keep this
// test: it must stay green.
func TestAFailedDeadLetterIsNeverTakenForTheEndOfTheReader(t *testing.T) {
	deterministicSources(t)
	errSink := errors.New("dead letter topic unreachable")
	causes := map[string]func() error{
		"a panic with io.EOF":                  func() error { panic(io.EOF) },
		"a panic with io.ErrClosedPipe":        func() error { panic(io.ErrClosedPipe) },
		"a panic with an error wrapping EOF":   func() error { panic(fmt.Errorf("read: %w", io.EOF)) },
		"an error wrapping EOF, retries spent": func() error { return fmt.Errorf("read: %w", io.EOF) },
	}
	for name, cause := range causes {
		t.Run(name, func(t *testing.T) {
			d := testDelivery(&recordingSink{err: errSink})
			d.Backoff = []time.Duration{}
			ctx := t.Context()

			err := d.Deliver(ctx, Position{Topic: TopicPlayerEvents}, validEvent(t),
				func(context.Context, Event) error { return cause() })

			if !errors.Is(err, errSink) {
				t.Fatalf("err = %v, want the failure of the sink", err)
			}
			if stopped(ctx, err) {
				t.Errorf("stopped(live context, %q) = true: the kafka Subscribe would return nil and drop the consumer", err)
			}
		})
	}
}
