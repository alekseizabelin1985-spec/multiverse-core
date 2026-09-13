// Package replay drives a process from recorded events instead of the wall
// clock: the time of the contexts is the time of the events they have read
// (EventClock), their timers never fire on their own (NullTimers), and a reader
// catches up on the journal by a cursor (Cursor, CatchUp). The format of a
// recorded session and its readers live in shared/recording (C-01 v1.9).
//
// Only cmd/multiverse imports the package (ADR-001 addendum p. 2): it builds
// these objects for --mode=replay and hands the contexts nothing but the
// interfaces of shared/clock through runtime.Deps (state-and-mechanics.md §6).
//
// # Time in a handler
//
// In replay the clock of the process is one EventClock. It stands at the
// latest event any reader of the process has observed and never goes back.
// Inside a handler Clock.Now() is therefore the time of the event being handled
// or a later one, and with several readers — subscriptions to different topics,
// the per-world workers of State — which later time depends on the scheduler.
// The rule for a handler: the time of the event it handles is ev.Timestamp. An
// event it publishes as a consequence is built with eventbus.Derive, which
// inherits that timestamp. Clock.Now() never goes into the bytes of an event
// derived from another; it is for what has no cause to inherit from.
//
// A read of history does not move the clock (C-01 v1.11): the handler of
// recording.ReadJournal runs under a marked context, and the middleware hands
// it the events as the journal holds them, without meta.replay. Only the
// deliveries — Subscribe, Tail, ReadRange with a handler of the caller,
// CatchUp — are the time of the contexts.
package replay

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
)

// EventClock is the clock of replay: it stands at the latest event timestamp
// it has observed and never moves backwards (NFR-061). It reads no wall clock,
// so two runs over the same journal see the same times.
type EventClock struct {
	mu  sync.Mutex
	now time.Time
}

var _ clock.Clock = (*EventClock)(nil)

// NewEventClock returns a clock standing at start. The process passes the
// earliest timestamp of the recording (Recording.Start), or the zero time
// without a recording.
func NewEventClock(start time.Time) *EventClock {
	return &EventClock{now: start}
}

// Observe moves the clock to t when t is later than the current time. An
// earlier t is ignored: a journal read at least once may deliver an older
// event again, and time that went backwards would reorder what the contexts
// derive from it.
func (c *EventClock) Observe(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t.After(c.now) {
		c.now = t
	}
}

// Now returns the latest observed event time.
func (c *EventClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// ErrClockBehind is returned by Advance for a time earlier than the clock.
var ErrClockBehind = errors.New("replay: clock behind")

// Advance moves the clock to at, the time the harness gives the next root
// event of a scenario (C-01 v1.9, "Время корневых событий в replay"). A time
// equal to the clock changes nothing. An earlier one is refused with
// ErrClockBehind and leaves the clock where it is: the clock never goes back,
// and a step back swallowed the way Observe swallows a redelivered event would
// stamp the next root with a time that is not the one the harness asked for.
//
// The comparison and the move happen under one lock, as C-01 asks. Now
// followed by Observe would leave a logical race, not a data race: the
// middleware may move the clock between the check and the move, every field is
// still read under the lock, and -race does not see it. Since both calls only
// raise the clock, such an interleaving ends linearizable and no test tells it
// from this one; the one lock is what holds the atomicity.
func (c *EventClock) Advance(at time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if at.Before(c.now) {
		return fmt.Errorf("%w: %s is before the clock at %s", ErrClockBehind,
			at.Format(time.RFC3339Nano), c.now.Format(time.RFC3339Nano))
	}
	if at.After(c.now) {
		c.now = at
	}
	return nil
}
