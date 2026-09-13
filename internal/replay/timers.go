package replay

import (
	"sync/atomic"
	"time"

	"multiverse-core.io/shared/clock"
)

// NullTimers is the Timers of replay: every timer it creates stays armed and
// never fires. What a timer would have caused in live mode — a tick, the
// timeout of a round, the TTL of an agent — was recorded as an event
// (tick.fired, round.closed, agent.stopped) and is read from the journal
// instead (ADR-003 p. 4, 6).
//
// It belongs to the contexts only. The bus measures the pauses between
// redeliveries with real timers in every mode (C-01 v1.4): a pause that never
// ends would turn the first failing handler of a replay into a hang.
type NullTimers struct{}

var _ clock.Timers = NullTimers{}

// After returns a timer that never fires, whatever d is.
func (NullTimers) After(time.Duration) clock.Timer { return newNullTimer() }

// Every returns a timer that never fires. Unlike time.NewTicker it accepts a
// non-positive period: there is no period to honour, and a process that
// panics on a duration only in replay would differ from its live run for no
// reason.
func (NullTimers) Every(time.Duration) clock.Timer { return newNullTimer() }

// nullTimer has a channel of its own rather than a nil one: a caller that
// compares or closes over C() gets a real, distinct channel, and nothing is
// ever sent on it.
type nullTimer struct {
	ch      chan time.Time
	stopped atomic.Bool
}

func newNullTimer() *nullTimer { return &nullTimer{ch: make(chan time.Time)} }

func (t *nullTimer) C() <-chan time.Time { return t.ch }

// Stop reports whether the timer was still armed, like the timers of
// shared/clock: true on the first call, false after it.
func (t *nullTimer) Stop() bool { return t.stopped.CompareAndSwap(false, true) }
