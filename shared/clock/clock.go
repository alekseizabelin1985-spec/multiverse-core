// Package clock provides the time interfaces every context depends on.
//
// Production code never calls time.Now or time.After directly: the linter
// forbids them outside this package, so that replay and unit tests can drive
// time deterministically (foundation.md §4, ADR-001 addendum p. 2).
package clock

import (
	"sync/atomic"
	"time"
)

// Clock reports the current time of the process.
type Clock interface {
	Now() time.Time
}

// Timer is a single delivery channel that fires once (After) or repeatedly
// (Every). Stop reports whether the timer was still armed.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// Timers creates timers bound to a Clock.
type Timers interface {
	After(d time.Duration) Timer
	Every(d time.Duration) Timer
}

// Real is the wall-clock implementation used in mode live.
type Real struct{}

// Now returns the wall-clock time.
func (Real) Now() time.Time { return time.Now() }

// RealTimers creates timers backed by the Go runtime timer wheel.
type RealTimers struct{}

// After returns a timer that fires once after d.
func (RealTimers) After(d time.Duration) Timer { return &realTimer{t: time.NewTimer(d)} }

// Every returns a timer that fires every d until stopped.
func (RealTimers) Every(d time.Duration) Timer { return &realTicker{t: time.NewTicker(d)} }

type realTimer struct{ t *time.Timer }

func (r *realTimer) C() <-chan time.Time { return r.t.C }
func (r *realTimer) Stop() bool          { return r.t.Stop() }

// realTicker adds "was it still armed" to time.Ticker, whose Stop returns
// nothing. The flag is atomic because a context typically owns the ticker in a
// worker goroutine and stops it from Stop(ctx).
type realTicker struct {
	t       *time.Ticker
	stopped atomic.Bool
}

func (r *realTicker) C() <-chan time.Time { return r.t.C }

func (r *realTicker) Stop() bool {
	if !r.stopped.CompareAndSwap(false, true) {
		return false
	}
	r.t.Stop()
	return true
}
