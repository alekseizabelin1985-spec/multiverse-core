package eventbus

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
)

func TestPermanentKeepsTheCauseAndItsText(t *testing.T) {
	if err := Permanent(nil); err != nil {
		t.Fatalf("Permanent(nil) = %v, want nil", err)
	}

	cause := &fs.PathError{Op: "open", Path: "event.json", Err: fs.ErrNotExist}
	err := Permanent(cause)
	if !errors.Is(err, ErrPermanent) {
		t.Error("errors.Is(Permanent(cause), ErrPermanent) = false")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Error("errors.Is does not reach through Permanent to the cause")
	}
	var asPath *fs.PathError
	if !errors.As(err, &asPath) || asPath != cause {
		t.Error("errors.As does not reach the cause through Permanent")
	}
	if err.Error() != cause.Error() {
		t.Errorf("text = %q, want the text of the cause %q: it is what the dead letter carries", err.Error(), cause.Error())
	}

	wrapped := fmt.Errorf("decode proposal: %w", err)
	if !errors.Is(wrapped, ErrPermanent) {
		t.Error("a permanent error wrapped once more is no longer permanent")
	}
	if errors.Is(errHandler, ErrPermanent) {
		t.Error("an ordinary error reads as permanent")
	}
}

// stalledDelivery retries on manual timers that nobody advances: a retry
// hangs there, so a delivery that returns parked without a pause.
func stalledDelivery(sink DeadLetterSink) Delivery {
	d := testDelivery(sink)
	d.Backoff = DefaultBackoff
	d.Timers = clock.NewManual(fixtureTime).Timers()
	return d
}

// deliverWithin runs Deliver and fails the test instead of hanging when it
// does not return: on a stalled delivery that means it waited for a retry.
func deliverWithin(t *testing.T, deliver func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- deliver() }()
	select {
	case err := <-done:
		return err
	case <-clock.RealTimers{}.After(5 * time.Second).C():
		t.Fatal("Deliver did not return: the permanent error waited for a retry")
		return nil
	}
}

func TestDeliverParksAPermanentErrorAtOnceWithoutAPause(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	ev := validEvent(t)
	cause := errors.New("proposal names no world")

	calls := 0
	err := deliverWithin(t, func() error {
		return stalledDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents, Offset: 3}, ev,
			func(context.Context, Event) error {
				calls++
				return Permanent(cause)
			})
	})

	if err != nil {
		t.Fatalf("deliver = %v, want nil: the parked event is done with", err)
	}
	if calls != 1 {
		t.Errorf("handler called %d times, want 1: a permanent error is not retried", calls)
	}
	letters := sink.all()
	if len(letters) != 1 {
		t.Fatalf("dead letters = %+v, want one", letters)
	}
	dl := letters[0]
	if dl.Attempts != 1 || dl.Error != cause.Error() || dl.Original.ID != ev.ID || dl.Consumer != "core.state" {
		t.Errorf("dead letter = {attempts %d, error %q, original %s, consumer %s}, want {1, %q, %s, core.state}",
			dl.Attempts, dl.Error, dl.Original.ID, dl.Consumer, cause.Error(), ev.ID)
	}
}

// attempts is the number of the call that returned the permanent error, not
// one: the calls before it failed in an ordinary way and were retried.
func TestDeliverParksAPermanentErrorWithTheNumberOfItsCall(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}

	calls := 0
	err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, validEvent(t),
		func(context.Context, Event) error {
			calls++
			if calls == 1 {
				return errHandler
			}
			return fmt.Errorf("decode: %w", Permanent(errors.New("bad payload")))
		})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if calls != 2 {
		t.Errorf("handler called %d times, want 2", calls)
	}
	letters := sink.all()
	if len(letters) != 1 || letters[0].Attempts != 2 || letters[0].Error != "decode: bad payload" {
		t.Errorf("dead letters = %+v, want one with attempts 2 and error %q", letters, "decode: bad payload")
	}
}

// Under a cancelled context a permanent error is not parked (C-01 v1.10): the
// stopping process cannot tell a bad event from one it did not finish, so the
// event stays uncommitted and comes again after the restart.
func TestDeliverDoesNotParkAPermanentErrorUnderCancellation(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	logs := &recordingLog{}
	d := stalledDelivery(sink)
	d.Log = slog.New(logs)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0
	err := deliverWithin(t, func() error {
		return d.Deliver(ctx, Position{Topic: TopicPlayerEvents}, validEvent(t),
			func(context.Context, Event) error {
				calls++
				cancel()
				return Permanent(errors.New("bad payload"))
			})
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("deliver = %v, want the cancellation: the event must be redelivered, not parked", err)
	}
	if calls != 1 {
		t.Errorf("handler called %d times, want 1", calls)
	}
	if got := sink.all(); len(got) != 0 {
		t.Errorf("a permanent error was parked under a cancelled context: %+v", got)
	}
	if got := logs.all(); len(got) != 0 {
		t.Errorf("log = %v, want nothing: the event was neither parked nor lost", messagesOf(got))
	}
}

// A panic stays a panic, whether the context is alive or cancelled: it is
// parked with ErrHandlerPanic, and its one log line is the line of the panic —
// panic and stack, handled=false (C-01 v1.10: ErrHandlerPanic and handled=false
// stay with the catch of a panic). A panic whose value is a permanent error
// matches ErrPermanent through ErrHandlerPanic, so a Deliver that asked
// errors.Is before it asked whether the handler panicked would add the handled
// line of a permanent error to it; the dead letter would read the same, and
// only the log tells the two apart (review #1 of T-460, Mi-2).
func TestDeliverKeepsAPanicApartFromAPermanentError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cancel bool
	}{
		{name: "live context", cancel: false},
		{name: "cancelled context", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deterministicSources(t)
			sink := &recordingSink{}
			logs := &recordingLog{}
			d := testDelivery(sink)
			d.Log = slog.New(logs)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			err := d.Deliver(ctx, Position{Topic: TopicPlayerEvents}, validEvent(t),
				func(context.Context, Event) error {
					if tc.cancel {
						cancel()
					}
					panic(Permanent(errors.New("bad payload")))
				})

			if err != nil {
				t.Fatalf("deliver = %v, want nil: a panic is parked", err)
			}
			letters := sink.all()
			if len(letters) != 1 {
				t.Fatalf("dead letters = %+v, want one", letters)
			}
			if want := ErrHandlerPanic.Error() + ": bad payload"; letters[0].Error != want {
				t.Errorf("dead letter says %q, want %q", letters[0].Error, want)
			}

			records := logs.all()
			if len(records) != 1 {
				t.Fatalf("log = %v, want the one line of the panic", messagesOf(records))
			}
			rec := records[0]
			if strings.Contains(rec.Message, "failed permanently") {
				t.Errorf("the line of a panic reads as a permanent error: %q", rec.Message)
			}
			attrs := attrsOf(rec)
			if got := attrs["handled"]; got != "false" {
				t.Errorf("handled = %q, want false: a panic is not handled", got)
			}
			for _, key := range []string{"panic", "stack"} {
				if attrs[key] == "" {
					t.Errorf("the line of a panic carries no %s", key)
				}
			}
		})
	}
}

func TestDeliverLogsAPermanentErrorAsHandledWithoutAStack(t *testing.T) {
	deterministicSources(t)
	logs := &recordingLog{}
	d := testDelivery(&recordingSink{})
	d.Log = slog.New(logs)
	ev := validEvent(t)

	if err := d.Deliver(t.Context(), Position{Topic: TopicPlayerEvents, Offset: 17}, ev,
		func(context.Context, Event) error { return Permanent(errors.New("bad payload")) }); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	records := logs.all()
	if len(records) != 1 {
		t.Fatalf("log = %v, want one record", messagesOf(records))
	}
	rec := records[0]
	if rec.Level != slog.LevelError {
		t.Errorf("level = %s, want ERROR", rec.Level)
	}
	attrs := attrsOf(rec)
	for key, want := range map[string]string{
		"handled":    "true",
		"attempts":   "1",
		"error":      "bad payload",
		"event_id":   ev.ID,
		"event_type": ev.Type,
		"topic":      TopicPlayerEvents,
		"offset":     "17",
		"consumer":   "core.state",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	// service_panics counts the lines with a panic; a permanent error is not
	// one.
	for _, key := range []string{"panic", "stack"} {
		if _, ok := attrs[key]; ok {
			t.Errorf("the line of a permanent error carries %s", key)
		}
	}
}

// A dead letter that could not be written leaves the event uncommitted, and
// the log says so instead of claiming it was handled.
func TestDeliverReportsAFailingSinkForAPermanentError(t *testing.T) {
	deterministicSources(t)
	errSink := errors.New("dead letter topic unreachable")
	logs := &recordingLog{}
	d := testDelivery(&recordingSink{err: errSink})
	d.Log = slog.New(logs)

	err := d.Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, validEvent(t),
		func(context.Context, Event) error { return Permanent(errors.New("bad payload")) })

	if !errors.Is(err, errSink) {
		t.Errorf("deliver = %v, want the sink error", err)
	}
	records := logs.all()
	if len(records) != 1 || attrsOf(records[0])["handled"] != "false" {
		t.Errorf("log = %v, want the one line of an event not parked, handled=false", messagesOf(records))
	}
}
