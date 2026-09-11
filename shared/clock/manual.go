package clock

import (
	"sort"
	"sync"
	"time"
)

// Manual is a Clock that only moves when a test moves it. Timers created by
// its Timers implementation fire during Set and Advance, in deadline order,
// so a test observes the same sequence on every run.
type Manual struct {
	mu     sync.Mutex
	now    time.Time
	timers []*manualTimer
}

// NewManual returns a Manual clock positioned at t.
func NewManual(t time.Time) *Manual { return &Manual{now: t} }

// Now returns the current manual time.
func (m *Manual) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// Set moves the clock to t and fires every timer whose deadline is not after t.
// Moving the clock backwards does not fire anything.
func (m *Manual) Set(t time.Time) {
	m.mu.Lock()
	backwards := !t.After(m.now)
	m.now = t
	if backwards {
		m.mu.Unlock()
		return
	}
	due := m.collectDue()
	m.mu.Unlock()
	fire(due, t)
}

// Advance moves the clock forward by d and fires every timer that becomes due.
func (m *Manual) Advance(d time.Duration) {
	m.mu.Lock()
	m.now = m.now.Add(d)
	at := m.now
	due := m.collectDue()
	m.mu.Unlock()
	fire(due, at)
}

// Timers returns the Timers implementation bound to this clock.
func (m *Manual) Timers() *ManualTimers { return &ManualTimers{clock: m} }

// collectDue must be called with m.mu held. It returns the timers due at the
// current time and re-arms periodic ones.
func (m *Manual) collectDue() []*manualTimer {
	var due []*manualTimer
	kept := m.timers[:0]
	for _, t := range m.timers {
		if t.stopped {
			continue
		}
		if t.deadline.After(m.now) {
			kept = append(kept, t)
			continue
		}
		due = append(due, t)
		if t.period > 0 {
			for !t.deadline.After(m.now) {
				t.deadline = t.deadline.Add(t.period)
			}
			kept = append(kept, t)
			continue
		}
		t.stopped = true
	}
	m.timers = kept
	sort.SliceStable(due, func(i, j int) bool { return due[i].deadline.Before(due[j].deadline) })
	return due
}

func fire(due []*manualTimer, at time.Time) {
	for _, t := range due {
		select {
		case t.ch <- at:
		default:
		}
	}
}

func (m *Manual) add(t *manualTimer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t.deadline = m.now.Add(t.delay)
	m.timers = append(m.timers, t)
}

// ManualTimers creates timers driven by a Manual clock.
type ManualTimers struct{ clock *Manual }

// NewManualTimers returns the Timers implementation bound to m.
func NewManualTimers(m *Manual) *ManualTimers { return &ManualTimers{clock: m} }

// After returns a timer that fires once when the clock reaches now+d.
func (mt *ManualTimers) After(d time.Duration) Timer {
	t := &manualTimer{ch: make(chan time.Time, 1), delay: d, owner: mt.clock}
	mt.clock.add(t)
	return t
}

// Every returns a timer that fires every d of manual time until stopped.
func (mt *ManualTimers) Every(d time.Duration) Timer {
	t := &manualTimer{ch: make(chan time.Time, 1), delay: d, period: d, owner: mt.clock}
	mt.clock.add(t)
	return t
}

type manualTimer struct {
	ch       chan time.Time
	delay    time.Duration
	period   time.Duration
	deadline time.Time
	stopped  bool
	owner    *Manual
}

func (t *manualTimer) C() <-chan time.Time { return t.ch }

// Stop reports whether the timer was still armed when it was stopped.
func (t *manualTimer) Stop() bool {
	t.owner.mu.Lock()
	defer t.owner.mu.Unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}
