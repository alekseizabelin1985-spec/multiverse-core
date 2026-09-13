package state

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/eventbus"
)

// Apply handles one event of system_events. It is the handler Start subscribes
// with, and it is exported so that a test can drive the double without a
// goroutine: publishing a proposal and waiting for a fact tests the wiring,
// calling Apply tests the decision.
//
// The decision is internal/state.Applier's (§4.5): everything that is not a
// proposal of this world is passed over — the facts of the double itself
// included, a proposal of another world at Debug, a proposal without a world in
// its envelope at Warn (C-02 v1.7) — and a refusal is an entity.update.rejected
// on the bus and a nil error, because the proposal was handled.
//
// An event of the answer that does not go out is published again until it
// does or ctx ends; then the world stops and every later call returns
// internal/state.ErrWorldStopped (Applier.Apply). ctx is the lifetime of the
// world for that reason: the subscription passes its own.
func (s *FakeState) Apply(ctx context.Context, ev eventbus.Event) error {
	if pos, ok := eventbus.PositionFromContext(ctx); ok && pos.Topic == eventbus.TopicSystemEvents {
		s.mu.Lock()
		s.cursor = pos.Offset + 1
		s.mu.Unlock()
	}
	s.mu.Lock()
	broken := s.broken
	s.mu.Unlock()
	if broken != nil {
		return broken
	}
	s.deciding.Lock()
	defer s.deciding.Unlock()
	return s.applier.Apply(ctx, ev)
}

// rulesFile is the rule book of the world of MVP-1, relative to the root of
// the tree.
const rulesFile = "rules/dark-forest.yaml"

// invariantsOfTheTree loads the laws rules/dark-forest.yaml switches on. The
// file is found from the source of this package, not from the working
// directory: a test runs in the directory of its own package, and the binary
// of cmd/multiverse in whichever it was started from.
func invariantsOfTheTree() ([]mechanics.Invariant, error) {
	_, self, _, ok := goruntime.Caller(0)
	if !ok {
		return nil, errors.New("cannot locate the source of the package")
	}
	for dir := filepath.Dir(self); ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			rules, err := mechanics.Load(filepath.Join(dir, filepath.FromSlash(rulesFile)))
			if err != nil {
				return nil, err
			}
			return rules.Invariants(), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, errors.New("no go.mod above the source of testkit/state")
		}
		dir = parent
	}
}
