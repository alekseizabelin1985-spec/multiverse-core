package eventbus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
)

// fixtureTime is the instant every deterministic test starts at.
var fixtureTime = time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

// fakeRegistry stands in for shared/contracts (T-006), which cannot be
// imported here: it validates eventbus.Event and therefore depends on this
// package.
type fakeRegistry struct {
	specs    map[string]TypeSpec
	validate func(Event) error
}

func (r fakeRegistry) Lookup(eventType string) (TypeSpec, bool) {
	spec, ok := r.specs[eventType]
	return spec, ok
}

func (r fakeRegistry) Validate(ev Event) error {
	if r.validate == nil {
		return nil
	}
	return r.validate(ev)
}

// testRegistry mirrors the policies of C-01 on four representative types.
func testRegistry() fakeRegistry {
	return fakeRegistry{specs: map[string]TypeSpec{
		"player.attacked": {Topic: TopicPlayerEvents, SchemaVersion: 1, Policy: PlayerEventsPolicy()},
		"tick.fired":      {Topic: TopicSystemEvents, SchemaVersion: 1, Policy: SwarmPolicy()},
		"entity.updated":  {Topic: TopicSystemEvents, SchemaVersion: 2},
		"player.moved":    {Topic: TopicPlayerEvents, Policy: PlayerEventsPolicy(), Deprecated: true},
	}}
}

// deterministicSources installs a sequence generator and a manual clock, so
// that two runs of the same test produce byte-identical events (NFR-061).
func deterministicSources(t *testing.T) *clock.Manual {
	t.Helper()
	manual := clock.NewManual(fixtureTime)
	SetIDSource(SequenceIDs("ev"))
	SetClock(manual)
	t.Cleanup(func() {
		SetIDSource(nil)
		SetClock(nil)
		SetRegistry(nil)
	})
	return manual
}

// recordingSink collects the dead letters a delivery parks.
type recordingSink struct {
	mu      sync.Mutex
	letters []DeadLetter
	err     error
}

func (s *recordingSink) WriteDeadLetter(_ context.Context, dl DeadLetter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.letters = append(s.letters, dl)
	return nil
}

func (s *recordingSink) all() []DeadLetter {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]DeadLetter(nil), s.letters...)
}

// errHandler is the failure a handler under test reports.
var errHandler = errors.New("handler failed")

// noPause is a backoff of three retries that does not actually wait: the
// number of attempts is what the tests check, not the wall time.
var noPause = []time.Duration{0, 0, 0}
