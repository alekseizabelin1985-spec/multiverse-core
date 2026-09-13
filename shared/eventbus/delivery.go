package eventbus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"time"

	"multiverse-core.io/shared/clock"
)

// DefaultBackoff is the pause before each retry of a failing handler
// (ADR-007 p. 6): three retries, hence four calls of the handler in all,
// before the event is parked in dead_letters.
var DefaultBackoff = []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 2 * time.Second}

// MaxDeadLetterRaw caps the undecodable body carried into dead_letters. A
// reader accepts a record of up to kafkaMaxBytes, but a dead letter larger
// than the 1 MiB default of max.message.bytes cannot be written back: the
// write fails, the offset is never committed, and the topic stalls forever on
// the very message dead_letters exists to get it past. Half a mebibyte leaves
// room for base64 and for the envelope around it.
const MaxDeadLetterRaw = 512 << 10

// DeadLetter is the envelope of dead_letters: the event that could not be
// handled, why, and by whom.
type DeadLetter struct {
	Original Event  `json:"original"`
	Error    string `json:"error"`
	Consumer string `json:"consumer"`
	// Attempts counts the calls of the handler; zero means the event was
	// rejected by validation and the handler was never called.
	Attempts int       `json:"attempts"`
	FailedAt time.Time `json:"failed_at"`
	// Raw carries the message body when it could not be parsed as an event.
	// It is bytes, not json.RawMessage: an undecodable body is by definition
	// not valid JSON, and a truncated one never is, so embedding it verbatim
	// would make the dead letter itself unmarshalable.
	Raw []byte `json:"raw,omitempty"`
	// RawTruncated says that Raw holds only the first MaxDeadLetterRaw bytes.
	RawTruncated bool `json:"raw_truncated,omitempty"`
}

// DeadLetterSink parks an event that could not be handled. Every bus
// implementation provides one over its own transport.
type DeadLetterSink interface {
	WriteDeadLetter(ctx context.Context, dl DeadLetter) error
}

// DeadLetterFunc adapts a function to DeadLetterSink.
type DeadLetterFunc func(ctx context.Context, dl DeadLetter) error

// WriteDeadLetter calls f.
func (f DeadLetterFunc) WriteDeadLetter(ctx context.Context, dl DeadLetter) error { return f(ctx, dl) }

// Delivery is the read side of the C-01 contract, shared by every bus
// implementation: validation on read, the topic policy, the retries, the catch
// of a handler panic and the dead-letter fallback. Keeping it in one place is
// what makes the kafka adapter and membus behave identically, which the
// contract test of F-5t checks against both.
type Delivery struct {
	// Consumer names who was reading, recorded in the dead letter. For a
	// subscription it is the consumer group, for the journal the topic.
	Consumer string
	Registry Registry
	DLQ      DeadLetterSink
	// SkipValidateOnRead turns off validation on read. The zero value
	// validates: SEC-16 wants the check on unless it is switched off on
	// purpose, and a Delivery is built by every bus implementation and every
	// harness, none of which should be able to disable a defence in depth by
	// forgetting a field. The process reads MV_BUS_VALIDATE_ON_READ through
	// shared/env and sets SkipValidateOnRead = !MV_BUS_VALIDATE_ON_READ; this
	// package must not read the environment itself.
	SkipValidateOnRead bool
	// Backoff overrides DefaultBackoff; its length is the number of retries.
	Backoff []time.Duration
	Timers  clock.Timers
	Log     *slog.Logger
}

// Deliver hands one event to the handler under the read-side contract. It
// returns nil when the event is done with — handled, or parked in
// dead_letters — so that the caller may commit its offset. An error means the
// event was not accounted for and must not be committed.
//
// A panic of the handler is caught here, at the delivery boundary, and nowhere
// else (C-01 v1.5): the event is parked at once with ErrHandlerPanic, without a
// retry, and the panic is logged at Error level with its stack. Without the
// catch the event that brought the process down would be redelivered after the
// restart and bring it down again, together with every context it hosts.
func (d Delivery) Deliver(ctx context.Context, pos Position, ev Event, h Handler) error {
	ctx = ContextWithPosition(ctx, pos)

	if !d.SkipValidateOnRead {
		if err := d.validate(ev); err != nil {
			d.log(ctx, "event rejected on read", ev, pos, err)
			return d.deadLetter(ctx, pos, ev, err, 0, nil, "")
		}
	}

	backoff := d.backoff()
	var lastErr error
	for attempt := 0; attempt <= len(backoff); attempt++ {
		if attempt > 0 {
			if err := d.wait(ctx, backoff[attempt-1]); err != nil {
				return err
			}
		}
		panicked, err := d.call(ctx, pos, ev, h, attempt+1)
		if panicked {
			// A panic is a defect, not a transient failure (C-01 v1.5): a retry
			// would run the handler again over state the panic may have left half
			// changed, and a deterministic panic would simply happen four times.
			return d.deadLetter(ctx, pos, ev, err, attempt+1, nil, "")
		}
		lastErr = err
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			// The subscription is shutting down: leave the event
			// uncommitted so that it is redelivered, rather than burning
			// its retries against a cancelled context.
			return ctx.Err()
		}
	}
	return d.deadLetter(ctx, pos, ev, lastErr, len(backoff)+1, nil, "event parked in dead letters")
}

// DeliverRaw parks a message that could not be decoded into an event. There is
// nothing to hand a handler, so it goes straight to dead_letters.
func (d Delivery) DeliverRaw(ctx context.Context, pos Position, raw []byte, cause error) error {
	ev := Event{}
	err := fmt.Errorf("decode message at %s/%d: %w", pos.Topic, pos.Offset, cause)
	return d.deadLetter(ctx, pos, ev, err, 0, raw, "undecodable message parked in dead letters")
}

// validate applies the read-side checks. A Delivery without a registry checks
// the envelope only: it is the shape used by fixtures, never by the process,
// which always builds the bus with a registry.
func (d Delivery) validate(ev Event) error {
	if err := ev.ValidateEnvelope(); err != nil {
		return err
	}
	if d.Registry == nil {
		return nil
	}
	spec, ok := d.Registry.Lookup(ev.Type)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownType, ev.Type)
	}
	if spec.Deprecated {
		return nil
	}
	if err := spec.Policy.Check(ev); err != nil {
		return err
	}
	if err := d.Registry.Validate(ev); err != nil {
		return fmt.Errorf("eventbus: validate %s: %w", ev.Type, err)
	}
	return nil
}

// deadLetter writes the dead letter and says in the log what became of the
// event. parked is logged only once the write has succeeded: a line claiming
// the event was parked before the write is known to have happened is a lie
// whenever the write fails, and after Close it is the only trace the event
// leaves, because the subscription returns nil (review #1 of T-436, Mi-1).
// An empty parked means the caller has already logged why the event goes to
// dead_letters.
func (d Delivery) deadLetter(ctx context.Context, pos Position, ev Event, cause error, attempts int, raw []byte, parked string) error {
	if cause == nil {
		cause = fmt.Errorf("%w: handler failed without an error", ErrInvalidEnvelope)
	}
	if d.DLQ == nil {
		err := fmt.Errorf("eventbus: no dead letter sink for %s (cause: %v)", ev.Type, cause)
		d.logNotParked(ctx, ev, pos, err, false)
		return err
	}
	body, truncated := truncateRaw(raw)
	dl := DeadLetter{
		Original:     ev,
		Error:        cause.Error(),
		Consumer:     d.Consumer,
		Attempts:     attempts,
		FailedAt:     nowUTC(),
		Raw:          body,
		RawTruncated: truncated,
	}
	// The dead letter must survive the cancellation that may have killed the
	// handler, otherwise a shutdown silently drops the failed event.
	writeCtx := context.WithoutCancel(ctx)
	if err := d.DLQ.WriteDeadLetter(writeCtx, dl); err != nil {
		wrapped := fmt.Errorf("eventbus: write dead letter for %s: %w (cause: %v)", ev.Type, err, cause)
		d.logNotParked(ctx, ev, pos, wrapped, busClosed(err))
		return wrapped
	}
	if parked != "" {
		d.log(ctx, parked, ev, pos, cause)
	}
	return nil
}

// busClosed tells a dead letter refused by a closed bus from a write that
// failed on a bus still running. A closed membus and Kafka.writer on a closed
// bus refuse with ErrClosed; a kafka writer taken from Kafka.writer before
// Close and called after it refuses with io.ErrClosedPipe, which Kafka.write
// wraps — hence errors.Is. A write already inside WriteMessages when Close
// comes is waited for and does not fail this way. Only the error of the sink
// is looked at: the cause is never in the chain (C-01 v1.6).
func busClosed(err error) bool {
	return errors.Is(err, ErrClosed) || errors.Is(err, io.ErrClosedPipe)
}

// logNotParked reports an event whose dead letter was not written. Either way
// it stays uncommitted and is delivered again. On a closed bus that is the
// orderly end of a subscription caught by Close, so Warn; on a running bus the
// subscription fails on it and the topic does not move, so Error. The line
// carries handled=false at either level, as logPanic does: NFR-033 wants the
// field on error logs, and the event is not accounted for in both cases.
func (d Delivery) logNotParked(ctx context.Context, ev Event, pos Position, err error, closed bool) {
	if d.Log == nil {
		return
	}
	level := slog.LevelError
	if closed {
		level = slog.LevelWarn
	}
	d.Log.Log(ctx, level, "event not parked in dead letters; it stays uncommitted and will be delivered again",
		append(d.eventAttrs(ev, pos),
			slog.Bool("handled", false),
			slog.Bool("bus_closed", closed),
			slog.Any("error", err))...)
}

// truncateRaw cuts the body down to what dead_letters can carry.
func truncateRaw(raw []byte) ([]byte, bool) {
	if len(raw) <= MaxDeadLetterRaw {
		return raw, false
	}
	return raw[:MaxDeadLetterRaw], true
}

func (d Delivery) backoff() []time.Duration {
	if d.Backoff != nil {
		return d.Backoff
	}
	return DefaultBackoff
}

func (d Delivery) wait(ctx context.Context, pause time.Duration) error {
	if pause <= 0 {
		return ctx.Err()
	}
	timers := d.Timers
	if timers == nil {
		timers = clock.RealTimers{}
	}
	t := timers.After(pause)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C():
		return nil
	}
}

// call runs the handler once and turns a panic into ErrHandlerPanic. The panic
// is logged from inside the deferred function because that is the only place
// where debug.Stack still shows the frames of the handler that panicked.
func (d Delivery) call(ctx context.Context, pos Position, ev Event, h Handler, attempt int) (panicked bool, err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		text := panicText(r)
		panicked, err = true, handlerPanic(r, text)
		d.logPanic(ctx, ev, pos, attempt, text, debug.Stack())
	}()
	return false, h(ctx, ev)
}

// panicText prints the panic value once, and is the only place that does. The
// value is whatever the handler panicked with, so its String or Error method
// may panic in turn; fmt recovers from the first such panic but re-panics when
// the value of that panic cannot be printed either, and a panic escaping here
// would leave the deferred function of call and take the process down after
// all (review #1 of T-426, Mi-1). The type name is what is left to say then.
func panicText(value any) (text string) {
	defer func() {
		if recover() != nil {
			text = fmt.Sprintf("%T (printing the value panicked)", value)
		}
	}()
	return fmt.Sprint(value)
}

// panicError is the error a panic is parked with. Its text is the one
// panicText rendered, never the value printed again: %w would call Error on a
// value that may panic.
type panicError struct {
	text  string
	cause error
}

func (e *panicError) Error() string { return ErrHandlerPanic.Error() + ": " + e.text }

// Unwrap keeps both ErrHandlerPanic and a panic value that is an error
// matchable with errors.Is.
func (e *panicError) Unwrap() []error {
	if e.cause == nil {
		return []error{ErrHandlerPanic}
	}
	return []error{ErrHandlerPanic, e.cause}
}

// handlerPanic builds the error a panic is parked with from the value and the
// text panicText made of it.
func handlerPanic(value any, text string) error {
	err := &panicError{text: text}
	if cause, ok := value.(error); ok {
		err.cause = cause
	}
	return err
}

// logPanic leaves the trace NFR-012 counts as service_panics. It is Error, not
// the Warn of an ordinary parked event: catching the panic changes what the
// defect costs, it does not make it anything other than a defect.
func (d Delivery) logPanic(ctx context.Context, ev Event, pos Position, attempt int, text string, stack []byte) {
	if d.Log == nil {
		return
	}
	attrs := append(d.eventAttrs(ev, pos),
		slog.Int("attempts", attempt),
		slog.Bool("handled", false),
		slog.String("panic", text),
		slog.String("stack", string(stack)),
	)
	d.Log.ErrorContext(ctx, "event handler panicked; the event goes to dead letters", attrs...)
}

func (d Delivery) log(ctx context.Context, msg string, ev Event, pos Position, err error) {
	if d.Log == nil {
		return
	}
	d.Log.WarnContext(ctx, msg, append(d.eventAttrs(ev, pos), slog.Any("error", err))...)
}

func (d Delivery) eventAttrs(ev Event, pos Position) []any {
	return []any{
		slog.String("event_id", ev.ID),
		slog.String("event_type", ev.Type),
		slog.String("correlation_id", ev.CorrelationID()),
		slog.String("topic", pos.Topic),
		slog.Int64("offset", pos.Offset),
		slog.String("consumer", d.Consumer),
	}
}
