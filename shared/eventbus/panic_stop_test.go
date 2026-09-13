package eventbus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// The reason the cause of a dead letter that could not be written is printed
// with %v and not wrapped with %w (reviews #1 and #2 of T-426; T-415). stopped
// takes any io.EOF, io.ErrClosedPipe or ErrClosed (T-443) in the chain of an
// error for the end of the reader or of the bus, and panicError keeps the panic
// value matchable, so with %w a handler that panicked with one of them — or
// returned an error wrapping one — would turn a failed dead letter into a
// silent nil return of the kafka Subscribe while its context is alive. The
// event would stay uncommitted and the consumer would simply stop.
//
// The same holds for a Delivery without a dead letter sink (T-430, C-01 v1.6).
// Both buses always set one, so the branch is unreachable today, but a change
// that reached it with %w would open the same trap there.
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
		// T-443: stopped takes ErrClosed for the end as well. A handler that
		// publishes to a closed bus of its own must not hide a failed dead
		// letter on a bus that is still running.
		"a panic with ErrClosed":                     func() error { panic(ErrClosed) },
		"an error wrapping ErrClosed, retries spent": func() error { return fmt.Errorf("publish: %w", ErrClosed) },
	}
	sinks := map[string]struct {
		sink DeadLetterSink
		want func(error) bool
	}{
		"the sink fails": {
			sink: &recordingSink{err: errSink},
			want: func(err error) bool { return errors.Is(err, errSink) },
		},
		"no sink": {
			want: func(err error) bool { return err != nil && strings.Contains(err.Error(), "no dead letter sink") },
		},
	}
	for name, cause := range causes {
		for sinkName, s := range sinks {
			t.Run(name+", "+sinkName, func(t *testing.T) {
				d := testDelivery(s.sink)
				d.Backoff = []time.Duration{}
				ctx := t.Context()

				err := d.Deliver(ctx, Position{Topic: TopicPlayerEvents}, validEvent(t),
					func(context.Context, Event) error { return cause() })

				if !s.want(err) {
					t.Fatalf("err = %v, want the failure to park the event", err)
				}
				if stopped(ctx, err) {
					t.Errorf("stopped(live context, %q) = true: the kafka Subscribe would return nil and drop the consumer", err)
				}
			})
		}
	}
}
