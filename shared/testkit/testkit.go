// Package testkit is the shared test scaffolding of the platform: the pins the
// containers of the integration tests run on, the helpers that make a test
// deterministic, and — in its subpackages — the in-process bus (membus) and the
// behavioural contract of the bus (contract) that both implementations obey
// (foundation.md §9, ADR-010).
//
// It is test-only code that lives outside _test.go files on purpose: every
// epic imports it, and Go cannot import the test files of another package.
// Nothing under cmd/ or internal/ may depend on it.
package testkit

import (
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
)

// Dedup is the deduplication window of a consumer, re-exported under the name
// C-01 gives it in the stub column. It is an alias, not a wrapper: a snapshot
// written by a test must be readable by production code and the other way
// round.
type Dedup = eventbus.Dedup

// NewDedup returns a deduplication window of the given capacity; zero or less
// selects eventbus.DefaultDedupCapacity.
func NewDedup(capacity int) *Dedup { return eventbus.NewDedup(capacity) }

// Epoch is the instant every deterministic test starts at. Fixing it in one
// place is what lets two tests compare golden bytes with each other.
var Epoch = time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

// Sources is what Deterministic installed, handed back so that a test can move
// the clock or read the next identifier.
type Sources struct {
	// Clock is the manual clock the event constructors stamp root events with.
	Clock *clock.Manual
	// Timers fire on Clock.Advance, in deadline order.
	Timers *clock.ManualTimers
}

// Deterministic installs a sequence generator and a manual clock into the
// event constructors and restores the process defaults when the test ends, so
// that two runs of the same test produce byte-identical events (NFR-061).
//
// The sources of eventbus are process wide, so a test that calls this must not
// call t.Parallel: two tests sharing the generator would hand out interleaved
// identifiers and neither would be reproducible.
func Deterministic(t *testing.T, prefix string) Sources {
	t.Helper()
	if prefix == "" {
		prefix = "ev"
	}
	manual := clock.NewManual(Epoch)
	eventbus.SetIDSource(eventbus.SequenceIDs(prefix))
	eventbus.SetClock(manual)
	t.Cleanup(func() {
		eventbus.SetIDSource(nil)
		eventbus.SetClock(nil)
		eventbus.SetRegistry(nil)
	})
	return Sources{Clock: manual, Timers: clock.NewManualTimers(manual)}
}

// Wall returns the real clock. Tests take the wall time from it rather than
// from time.Now, which the linter forbids outside shared/clock: a measurement
// of how long something actually took is the one thing a manual clock cannot
// give (the publish latency assertion of the bus contract, T-014).
func Wall() clock.Clock { return clock.Real{} }

// After is the timeout arm of a select in a test: a channel that fires after d
// of wall time. It comes from shared/clock like every other timer of the
// platform — a test waiting for a broker cannot use a manual clock, but it must
// not reach for time.After either, or the ban that keeps replay reproducible
// would have an exception nobody can see from the call site.
func After(d time.Duration) <-chan time.Time { return clock.RealTimers{}.After(d).C() }
