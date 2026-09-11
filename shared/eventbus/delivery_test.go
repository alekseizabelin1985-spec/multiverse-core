package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
)

func testDelivery(sink DeadLetterSink) Delivery {
	// Validation on read is not switched on here: the zero value of
	// SkipValidateOnRead validates (SEC-16), and that is exactly what the
	// tests below rely on.
	return Delivery{
		Consumer: "core.state",
		Registry: testRegistry(),
		DLQ:      sink,
		Backoff:  noPause,
	}
}

func validEvent(t *testing.T) Event {
	t.Helper()
	return NewRoot("player.attacked", "gateway", "dark-forest-world",
		&ScopeRef{ID: "solo:player-A", Type: "solo"}, ActorHuman, map[string]any{"target": "wolf"})
}

func TestDeliverCallsTheHandlerOnce(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	ev := validEvent(t)

	calls := 0
	err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents, Offset: 7}, ev,
		func(context.Context, Event) error {
			calls++
			return nil
		})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if calls != 1 {
		t.Errorf("handler called %d times, want 1", calls)
	}
	if got := sink.all(); len(got) != 0 {
		t.Errorf("a handled event was parked: %+v", got)
	}
}

func TestDeliverPutsThePositionInTheContext(t *testing.T) {
	deterministicSources(t)
	want := Position{Topic: TopicPlayerEvents, Offset: 42}

	var got Position
	var ok bool
	err := testDelivery(&recordingSink{}).Deliver(t.Context(), want, validEvent(t),
		func(ctx context.Context, _ Event) error {
			got, ok = PositionFromContext(ctx)
			return nil
		})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if !ok || got != want {
		t.Errorf("position = %+v (ok=%v), want %+v", got, ok, want)
	}
}

func TestDeliverRetriesThreeTimesThenParksTheEvent(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	ev := validEvent(t)

	calls := 0
	err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents, Offset: 3}, ev,
		func(context.Context, Event) error {
			calls++
			return errHandler
		})

	if err != nil {
		t.Fatalf("deliver reported an error instead of parking the event: %v", err)
	}
	// One call plus the three retries of ADR-007 p. 6.
	if calls != 4 {
		t.Errorf("handler called %d times, want 4 (one call and three retries)", calls)
	}
	letters := sink.all()
	if len(letters) != 1 {
		t.Fatalf("%d dead letters, want 1", len(letters))
	}
	dl := letters[0]
	if dl.Original.ID != ev.ID {
		t.Errorf("dead letter carries %q, want the original %q", dl.Original.ID, ev.ID)
	}
	if dl.Consumer != "core.state" {
		t.Errorf("consumer = %q, want core.state", dl.Consumer)
	}
	if dl.Attempts != 4 {
		t.Errorf("attempts = %d, want 4", dl.Attempts)
	}
	if !strings.Contains(dl.Error, errHandler.Error()) {
		t.Errorf("error = %q, want the handler error", dl.Error)
	}
	if dl.FailedAt.IsZero() {
		t.Error("failed_at is zero: the dead letter cannot be aged")
	}
}

func TestDeliverStopsRetryingWhenTheHandlerSucceeds(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}

	calls := 0
	err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, validEvent(t),
		func(context.Context, Event) error {
			calls++
			if calls < 3 {
				return errHandler
			}
			return nil
		})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if calls != 3 {
		t.Errorf("handler called %d times, want 3", calls)
	}
	if got := sink.all(); len(got) != 0 {
		t.Errorf("an eventually handled event was parked: %+v", got)
	}
}

func TestDeliverParksAnInvalidEventWithoutCallingTheHandler(t *testing.T) {
	deterministicSources(t)

	cases := map[string]Event{
		"unknown type": NewRoot("player.teleported", "gateway", "w1", nil, ActorHuman, nil),
		"policy violation": NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil,
			WithAgent(AgentRef{ID: "personal-gm", Level: "task", Blueprint: "personal-gm"})),
		"broken envelope": func() Event {
			ev := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
			ev.Meta.CorrelationID = ""
			return ev
		}(),
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			sink := &recordingSink{}
			called := false
			err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents, Offset: 1}, ev,
				func(context.Context, Event) error {
					called = true
					return nil
				})
			if err != nil {
				t.Fatalf("deliver: %v", err)
			}
			if called {
				t.Error("the handler saw an event that failed validation on read")
			}
			letters := sink.all()
			if len(letters) != 1 {
				t.Fatalf("%d dead letters, want 1", len(letters))
			}
			if letters[0].Attempts != 0 {
				t.Errorf("attempts = %d, want 0: the handler was never called", letters[0].Attempts)
			}
		})
	}
}

func TestDeliverSkipsValidationWhenItIsOff(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	d := testDelivery(sink)
	d.SkipValidateOnRead = true

	// MV_BUS_VALIDATE_ON_READ=false is defence in depth turned off, not a
	// second gate: an unregistered type still reaches the handler.
	called := false
	err := d.Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, NewRoot("player.teleported", "gateway", "w1", nil, ActorHuman, nil),
		func(context.Context, Event) error {
			called = true
			return nil
		})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if !called {
		t.Error("the handler was skipped although validation on read is off")
	}
	if got := sink.all(); len(got) != 0 {
		t.Errorf("event parked although validation on read is off: %+v", got)
	}
}

func TestZeroDeliveryValidatesOnRead(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}

	// MV_BUS_VALIDATE_ON_READ defaults to true (C-01, SEC-16), so a caller
	// that builds a Delivery without touching the flag must get the check,
	// not lose it. This is the whole point of the field being a "skip".
	d := Delivery{Consumer: "core.state", Registry: testRegistry(), DLQ: sink, Backoff: noPause}

	called := false
	err := d.Deliver(t.Context(), Position{Topic: TopicPlayerEvents},
		NewRoot("player.teleported", "gateway", "w1", nil, ActorHuman, nil),
		func(context.Context, Event) error {
			called = true
			return nil
		})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if called {
		t.Error("an unregistered type reached the handler: the zero value disabled validation")
	}
	if got := sink.all(); len(got) != 1 {
		t.Errorf("%d dead letters, want the unregistered event parked", len(got))
	}
}

func TestDeliverRawTruncatesAnOversizedBody(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	raw := make([]byte, MaxDeadLetterRaw+4096)
	for i := range raw {
		raw[i] = 'x'
	}

	err := testDelivery(sink).DeliverRaw(t.Context(), Position{Topic: TopicPlayerEvents, Offset: 5},
		raw, errors.New("unexpected end of JSON input"))
	if err != nil {
		t.Fatalf("deliver raw: %v", err)
	}

	dl := sink.all()[0]
	if len(dl.Raw) != MaxDeadLetterRaw {
		t.Errorf("raw is %d bytes, want it cut to %d: a dead letter over 1 MiB never leaves the topic",
			len(dl.Raw), MaxDeadLetterRaw)
	}
	if !dl.RawTruncated {
		t.Error("raw_truncated is not set: the reader of dead_letters cannot tell the body is partial")
	}
	// The dead letter is written as JSON; a truncated body must not stop it.
	if _, err := json.Marshal(dl); err != nil {
		t.Errorf("the truncated dead letter does not marshal: %v", err)
	}
}

func TestDeliverRawKeepsASmallBodyWhole(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}

	if err := testDelivery(sink).DeliverRaw(t.Context(), Position{Topic: TopicPlayerEvents},
		[]byte(`{"id": "ev-1", "type":`), errors.New("unexpected end of JSON input")); err != nil {
		t.Fatalf("deliver raw: %v", err)
	}
	if dl := sink.all()[0]; dl.RawTruncated {
		t.Error("raw_truncated is set for a body that fits")
	}
}

func TestDeliverLetsADeprecatedTypeThrough(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}

	// A legacy envelope carries no meta at all; only the envelope is checked.
	called := false
	err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents},
		NewEvent("player.moved", "legacy-orchestrator", "w1", nil),
		func(context.Context, Event) error {
			called = true
			return nil
		})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if !called {
		t.Error("a deprecated type was parked instead of handled")
	}
}

func TestDeliverReturnsTheCancellationInsteadOfParking(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	ctx, cancel := context.WithCancel(t.Context())

	err := testDelivery(sink).Deliver(ctx, Position{Topic: TopicPlayerEvents}, validEvent(t),
		func(context.Context, Event) error {
			cancel()
			return errHandler
		})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want the cancellation: the event must be redelivered, not parked", err)
	}
	if got := sink.all(); len(got) != 0 {
		t.Errorf("event parked during shutdown: %+v", got)
	}
}

func TestDeliverReportsAFailingSink(t *testing.T) {
	deterministicSources(t)
	errSink := errors.New("dead letter topic unreachable")
	sink := &recordingSink{err: errSink}

	err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, validEvent(t),
		func(context.Context, Event) error { return errHandler })

	// The offset must not be committed when the event was neither handled nor
	// parked.
	if !errors.Is(err, errSink) {
		t.Errorf("err = %v, want the sink error", err)
	}
}

func TestDeliverRawParksTheUndecodableMessage(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	raw := []byte(`{"id": "ev-1", "type":`)

	err := testDelivery(sink).DeliverRaw(t.Context(), Position{Topic: TopicPlayerEvents, Offset: 11},
		raw, errors.New("unexpected end of JSON input"))

	if err != nil {
		t.Fatalf("deliver raw: %v", err)
	}
	letters := sink.all()
	if len(letters) != 1 {
		t.Fatalf("%d dead letters, want 1", len(letters))
	}
	if string(letters[0].Raw) != string(raw) {
		t.Errorf("raw = %q, want the original message", letters[0].Raw)
	}
	if !strings.Contains(letters[0].Error, "offset") && !strings.Contains(letters[0].Error, "/11") {
		t.Errorf("error = %q, want the position of the message", letters[0].Error)
	}
}

func TestDefaultBackoffIsThreeRetries(t *testing.T) {
	want := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 2 * time.Second}
	if len(DefaultBackoff) != len(want) {
		t.Fatalf("backoff = %v, want %v", DefaultBackoff, want)
	}
	for i := range want {
		if DefaultBackoff[i] != want[i] {
			t.Errorf("backoff[%d] = %s, want %s", i, DefaultBackoff[i], want[i])
		}
	}
}

func TestDeadLetterSerialisesTheOriginalEnvelope(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	ev := validEvent(t)

	if err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, ev,
		func(context.Context, Event) error { return errHandler }); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	body, err := json.Marshal(sink.all()[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back DeadLetter
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Original.Type != ev.Type || back.Original.Meta.CorrelationID != ev.Meta.CorrelationID {
		t.Errorf("original = %+v, want the trace of %+v", back.Original.Meta, ev.Meta)
	}
}

func TestChainAppliesMiddlewareOutsideIn(t *testing.T) {
	var order []string
	mw := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context, ev Event) error {
				order = append(order, name)
				return next(ctx, ev)
			}
		}
	}

	h := Chain(func(context.Context, Event) error {
		order = append(order, "handler")
		return nil
	}, mw("first"), nil, mw("second"))

	if err := h(t.Context(), Event{}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	want := []string{"first", "second", "handler"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestDeliverParksAPanicAtOnceWithoutRetrying(t *testing.T) {
	cases := map[string]struct {
		value any
		want  string
	}{
		"a string": {value: "boom", want: "eventbus: handler panic: boom"},
		"an error": {value: errHandler, want: "eventbus: handler panic: " + errHandler.Error()},
		"a nil":    {value: nil, want: "eventbus: handler panic: panic called with nil argument"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			deterministicSources(t)
			sink := &recordingSink{}
			ev := validEvent(t)

			calls := 0
			err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents, Offset: 9}, ev,
				func(context.Context, Event) error {
					calls++
					panic(tc.value)
				})

			// nil: the event is accounted for and its offset may be committed.
			if err != nil {
				t.Fatalf("deliver: %v", err)
			}
			if calls != 1 {
				t.Errorf("handler called %d times, want 1: a panic is a defect and is not retried", calls)
			}
			letters := sink.all()
			if len(letters) != 1 {
				t.Fatalf("%d dead letters, want 1", len(letters))
			}
			dl := letters[0]
			if dl.Original.ID != ev.ID {
				t.Errorf("dead letter carries %q, want the original %q", dl.Original.ID, ev.ID)
			}
			if dl.Attempts != 1 {
				t.Errorf("attempts = %d, want 1: the number of the call that panicked", dl.Attempts)
			}
			if !strings.HasPrefix(dl.Error, tc.want) {
				t.Errorf("error = %q, want it to start with %q", dl.Error, tc.want)
			}
			if strings.Contains(dl.Error, "goroutine") {
				t.Errorf("the stack went into the dead letter: %q", dl.Error)
			}
		})
	}
}

func TestDeliverParksAPanicAfterAFailedAttemptWithItsCallNumber(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}

	calls := 0
	err := testDelivery(sink).Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, validEvent(t),
		func(context.Context, Event) error {
			calls++
			if calls == 1 {
				return errHandler
			}
			panic("boom on the retry")
		})

	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if calls != 2 {
		t.Errorf("handler called %d times, want 2: the retries stop at the panic", calls)
	}
	letters := sink.all()
	if len(letters) != 1 {
		t.Fatalf("%d dead letters, want 1", len(letters))
	}
	if letters[0].Attempts != 2 {
		t.Errorf("attempts = %d, want 2", letters[0].Attempts)
	}
	if !strings.HasPrefix(letters[0].Error, ErrHandlerPanic.Error()) {
		t.Errorf("error = %q, want the panic rather than the earlier handler error", letters[0].Error)
	}
}

func TestDeliverReportsAFailingSinkAfterAPanic(t *testing.T) {
	deterministicSources(t)
	errSink := errors.New("dead letter topic unreachable")

	calls := 0
	err := testDelivery(&recordingSink{err: errSink}).Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, validEvent(t),
		func(context.Context, Event) error {
			calls++
			panic("boom")
		})

	// Neither handled nor parked: the offset must stay uncommitted, as for an
	// ordinary failure whose dead letter could not be written.
	if !errors.Is(err, errSink) {
		t.Errorf("err = %v, want the sink error", err)
	}
	if err != nil && !strings.Contains(err.Error(), ErrHandlerPanic.Error()) {
		t.Errorf("err = %v, want it to name the panic it could not park", err)
	}
	if calls != 1 {
		t.Errorf("handler called %d times, want 1", calls)
	}
}

func TestDeliverLogsAPanicAsAnErrorWithItsStack(t *testing.T) {
	deterministicSources(t)
	logs := &recordingLog{}
	d := testDelivery(&recordingSink{})
	d.Log = slog.New(logs)
	ev := validEvent(t)

	if err := d.Deliver(t.Context(), Position{Topic: TopicPlayerEvents, Offset: 13}, ev,
		func(context.Context, Event) error { panicInAHandler(); return nil }); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	records := logs.all()
	if len(records) != 1 {
		t.Fatalf("%d log records, want 1", len(records))
	}
	rec := records[0]
	if rec.Level != slog.LevelError {
		t.Errorf("level = %s, want ERROR: the trace service_panics counts (NFR-012)", rec.Level)
	}
	attrs := attrsOf(rec)
	for key, want := range map[string]string{
		"panic":      "a handler went wrong",
		"handled":    "false",
		"event_id":   ev.ID,
		"event_type": ev.Type,
		"topic":      TopicPlayerEvents,
		"offset":     "13",
		"consumer":   "core.state",
		"attempts":   "1",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	// The stack is the stack of the handler, not of the recover: it names the
	// function that panicked.
	if !strings.Contains(attrs["stack"], "panicInAHandler") {
		t.Errorf("stack does not show the frame that panicked:\n%s", attrs["stack"])
	}
}

func TestHandlerPanicKeepsAnErrorValueMatchable(t *testing.T) {
	err := handlerPanic(errHandler, panicText(errHandler))
	if !errors.Is(err, ErrHandlerPanic) || !errors.Is(err, errHandler) {
		t.Errorf("err = %v, want it to match both ErrHandlerPanic and the panic value", err)
	}
	if want := "eventbus: handler panic: " + errHandler.Error(); err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
}

// nestedStringer and nestedError panic, when printed, with a value that panics
// when printed too: fmt recovers from the first panic and re-panics on the
// second (review #1 of T-426, Mi-1).
type nestedStringer struct{}

func (nestedStringer) String() string { panic(nestedStringer{}) }

type nestedError struct{}

func (nestedError) Error() string { panic(nestedError{}) }

func TestDeliverParksAPanicWhoseValueCannotBePrinted(t *testing.T) {
	cases := map[string]struct {
		value    any
		typeName string
	}{
		"String panics": {value: nestedStringer{}, typeName: "eventbus.nestedStringer"},
		"Error panics":  {value: nestedError{}, typeName: "eventbus.nestedError"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			deterministicSources(t)
			sink := &recordingSink{}
			logs := &recordingLog{}
			d := testDelivery(sink)
			d.Log = slog.New(logs)

			err := d.Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, validEvent(t),
				func(context.Context, Event) error { panic(tc.value) })

			if err != nil {
				t.Fatalf("deliver: %v", err)
			}
			letters := sink.all()
			if len(letters) != 1 {
				t.Fatalf("%d dead letters, want 1: the panic of printing the value escaped the catch", len(letters))
			}
			if dl := letters[0]; !strings.HasPrefix(dl.Error, ErrHandlerPanic.Error()) || !strings.Contains(dl.Error, tc.typeName) {
				t.Errorf("error = %q, want ErrHandlerPanic naming %s", dl.Error, tc.typeName)
			}
			records := logs.all()
			if len(records) != 1 || records[0].Level != slog.LevelError {
				t.Fatalf("log = %v, want one ERROR record", records)
			}
			if got := attrsOf(records[0])["panic"]; !strings.Contains(got, tc.typeName) {
				t.Errorf("panic = %q, want the type name %s", got, tc.typeName)
			}
		})
	}
}

// A panic while the subscription is stopping is parked all the same, unlike an
// ordinary error, which returns the cancellation and is redelivered: C-01 v1.5
// makes no exception for a shutdown, and a panic redelivered after the
// restart would only panic again (review #1 of T-426, Mi-2).
func TestDeliverParksAPanicDuringShutdown(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0
	err := testDelivery(sink).Deliver(ctx, Position{Topic: TopicPlayerEvents}, validEvent(t),
		func(context.Context, Event) error {
			calls++
			cancel()
			panic("boom during shutdown")
		})

	if err != nil {
		t.Fatalf("deliver = %v, want nil: the panic is parked, not left for redelivery", err)
	}
	if calls != 1 {
		t.Errorf("handler called %d times, want 1", calls)
	}
	letters := sink.all()
	if len(letters) != 1 || letters[0].Attempts != 1 {
		t.Errorf("dead letters = %+v, want one with attempts 1", letters)
	}
}

// panicInAHandler gives the stack a frame with a name the test can look for.
func panicInAHandler() {
	panic("a handler went wrong")
}

// recordingLog keeps what a delivery logs.
type recordingLog struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingLog) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingLog) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recordingLog) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingLog) WithGroup(string) slog.Handler      { return h }

func (h *recordingLog) all() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]slog.Record(nil), h.records...)
}

func attrsOf(r slog.Record) map[string]string {
	attrs := make(map[string]string)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	return attrs
}

func TestWaitHonoursTheBackoffDuration(t *testing.T) {
	deterministicSources(t)
	sink := &recordingSink{}
	d := testDelivery(sink)
	d.Backoff = []time.Duration{5 * time.Millisecond}

	wall := clock.Real{}
	start := wall.Now()
	if err := d.Deliver(t.Context(), Position{Topic: TopicPlayerEvents}, validEvent(t),
		func(context.Context, Event) error { return errHandler }); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if elapsed := wall.Now().Sub(start); elapsed < 5*time.Millisecond {
		t.Errorf("the retry took %s, want at least the backoff of 5ms", elapsed)
	}
}
