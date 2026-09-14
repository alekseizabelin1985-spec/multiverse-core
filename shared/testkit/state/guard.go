package state

import (
	"fmt"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

// Reporter is the part of testing.TB the guards need: a test passes its *T.
type Reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// OneStateOverTheWorld is the guard of "one State per world" over the events of
// system_events (review #1 of T-056, Mi-4; strict since review #2, Mi-6; moved
// here by T-059 so that the tests of the recovery of internal/state, the e2e of
// T-061/T-062 and the stands with the real State share it):
//
//   - every entity.created and entity.updated is published by the State of the
//     process (core/state);
//   - no event is published twice: a State restarted over its store that
//     published the facts of its snapshot or of its catch-up again would do so
//     under the very ids it first published them with, which State derives
//     from the proposal;
//   - one version of one entity is announced by one fact. A fact with an empty
//     changed[] announces no version (C-02) and is held to its id alone.
//
// A second State over the world — a double beside the process, or a second
// internal/state — answers every proposal again, under its own source or under
// the ids of the first.
func OneStateOverTheWorld(t Reporter, events []eventbus.Event) {
	t.Helper()
	announced := make(map[string]string)
	published := make(map[string]bool)
	for _, ev := range events {
		if ev.Type != TypeCreated && ev.Type != TypeUpdated {
			continue
		}
		id, _ := ev.Path().GetString("entity.entity.id")
		version, _ := ev.Path().GetInt("version")
		if ev.Source != contracts.SourceState {
			t.Errorf("%s of %s v%d is published by %q, want only the State of the process (%s)",
				ev.Type, id, version, ev.Source, contracts.SourceState)
		}
		if published[ev.ID] {
			t.Errorf("%s of %s v%d is published twice under %s: two States answer one world", ev.Type, id, version, ev.ID)
		}
		published[ev.ID] = true
		if changed, ok := ev.Path().GetSlice("changed"); ok && len(changed) == 0 {
			continue
		}
		key := fmt.Sprintf("%s v%d", id, version)
		if first, seen := announced[key]; seen {
			t.Errorf("%s is announced twice, by %s and %s: two States answer one world", key, first, ev.ID)
		}
		announced[key] = ev.ID
	}
}
