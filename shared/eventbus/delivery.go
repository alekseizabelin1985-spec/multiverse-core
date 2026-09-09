package eventbus

import (
	"context"
	"fmt"
	"log/slog"
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
// implementation: validation on read, the topic policy, the retries and the
// dead-letter fallback. Keeping it in one place is what makes the kafka
// adapter and membus behave identically, which the contract test of F-5t
// checks against both.
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
func (d Delivery) Deliver(ctx context.Context, pos Position, ev Event, h Handler) error {
	ctx = ContextWithPosition(ctx, pos)

	if !d.SkipValidateOnRead {
		if err := d.validate(ev); err != nil {
			d.log(ctx, "event rejected on read", ev, pos, err)
			return d.deadLetter(ctx, ev, err, 0, nil)
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
		lastErr = h(ctx, ev)
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
	d.log(ctx, "event parked in dead letters", ev, pos, lastErr)
	return d.deadLetter(ctx, ev, lastErr, len(backoff)+1, nil)
}

// DeliverRaw parks a message that could not be decoded into an event. There is
// nothing to hand a handler, so it goes straight to dead_letters.
func (d Delivery) DeliverRaw(ctx context.Context, pos Position, raw []byte, cause error) error {
	ev := Event{}
	err := fmt.Errorf("decode message at %s/%d: %w", pos.Topic, pos.Offset, cause)
	d.log(ctx, "undecodable message parked in dead letters", ev, pos, err)
	return d.deadLetter(ctx, ev, err, 0, raw)
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

func (d Delivery) deadLetter(ctx context.Context, ev Event, cause error, attempts int, raw []byte) error {
	if cause == nil {
		cause = fmt.Errorf("%w: handler failed without an error", ErrInvalidEnvelope)
	}
	if d.DLQ == nil {
		return fmt.Errorf("eventbus: no dead letter sink for %s: %w", ev.Type, cause)
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
		return fmt.Errorf("eventbus: write dead letter for %s: %w (cause: %v)", ev.Type, err, cause)
	}
	return nil
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

func (d Delivery) log(ctx context.Context, msg string, ev Event, pos Position, err error) {
	if d.Log == nil {
		return
	}
	d.Log.WarnContext(ctx, msg,
		slog.String("event_id", ev.ID),
		slog.String("event_type", ev.Type),
		slog.String("correlation_id", ev.CorrelationID()),
		slog.String("topic", pos.Topic),
		slog.Int64("offset", pos.Offset),
		slog.String("consumer", d.Consumer),
		slog.Any("error", err),
	)
}
