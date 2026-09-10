package testkit_test

import (
	"testing"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit"
)

// Deterministic is what makes two runs of one test produce the same bytes
// (NFR-061): the identifiers come from a sequence and the timestamp from a
// clock that only a test moves.
func TestDeterministicGivesRepeatableEvents(t *testing.T) {
	build := func() (eventbus.Event, eventbus.Event) {
		sources := testkit.Deterministic(t, "unit")
		root := eventbus.NewRoot("player.looked", "testkit/gateway", "world-1", nil,
			eventbus.ActorCI, nil)
		sources.Clock.Advance(time.Hour)
		child := eventbus.Derive(root, "player.said", "testkit/gateway", nil)
		return root, child
	}

	firstRoot, firstChild := build()
	secondRoot, secondChild := build()

	if firstRoot.ID != "unit-1" || firstChild.ID != "unit-2" {
		t.Fatalf("ids are %q and %q, want unit-1 and unit-2", firstRoot.ID, firstChild.ID)
	}
	if firstRoot.ID != secondRoot.ID || firstChild.ID != secondChild.ID {
		t.Error("two runs produced different identifiers")
	}
	if !firstRoot.Timestamp.Equal(testkit.Epoch) {
		t.Errorf("root timestamp = %s, want the epoch %s", firstRoot.Timestamp, testkit.Epoch)
	}
	// Derive inherits the timestamp of its cause even though the clock moved:
	// two runs of the same recording must produce the same bytes.
	if !firstChild.Timestamp.Equal(firstRoot.Timestamp) {
		t.Errorf("child timestamp = %s, want the timestamp of its cause %s", firstChild.Timestamp, firstRoot.Timestamp)
	}
	if !secondRoot.Timestamp.Equal(firstRoot.Timestamp) {
		t.Error("two runs produced different timestamps")
	}
}

// The cleanup must put the process sources back, otherwise the next test in
// the package inherits a clock that stopped in 2026.
func TestDeterministicRestoresTheProcessSources(t *testing.T) {
	t.Run("inner", func(t *testing.T) {
		testkit.Deterministic(t, "inner")
	})
	ev := eventbus.NewRoot("player.looked", "testkit/gateway", "world-1", nil, eventbus.ActorCI, nil)
	if ev.ID == "inner-1" {
		t.Error("the sequence generator survived the test that installed it")
	}
	if ev.Timestamp.Equal(testkit.Epoch) {
		t.Error("the manual clock survived the test that installed it")
	}
}

func TestDedupIsTheProductionWindow(t *testing.T) {
	window := testkit.NewDedup(2)
	if window.Capacity() != 2 {
		t.Fatalf("Capacity = %d, want 2", window.Capacity())
	}
	if window.Seen("a") {
		t.Error("the first sighting of an id reported a repeat")
	}
	if !window.Seen("a") {
		t.Error("the second sighting of an id was not reported as a repeat")
	}
	// The alias is the production type, not a copy of it: a window a test
	// writes into a snapshot must be readable by the process, which is what
	// snapshotIDs stands in for here — it compiles only while the two are the
	// same type.
	if ids := snapshotIDs(window); len(ids) != 1 || ids[0] != "a" {
		t.Errorf("the window holds %v, want [a]", ids)
	}
	if testkit.NewDedup(0).Capacity() != eventbus.DefaultDedupCapacity {
		t.Error("a capacity of zero does not select the default")
	}
}

func TestWallIsTheRealClock(t *testing.T) {
	before := testkit.Wall().Now()
	time.Sleep(2 * time.Millisecond)
	if !testkit.Wall().Now().After(before) {
		t.Error("Wall() does not move")
	}
}

// snapshotIDs is the production side of the deduplication window: what a
// consumer serialises into its snapshot (C-14).
func snapshotIDs(window *eventbus.Dedup) []string { return window.IDs() }
