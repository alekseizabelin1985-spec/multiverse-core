// Package replay drives a process from recorded events instead of the wall
// clock: the time of the contexts is the time of the events they have read
// (EventClock), their timers never fire on their own (NullTimers), and what a
// session recorded can be read back and indexed (Recording).
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
package replay

import (
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
