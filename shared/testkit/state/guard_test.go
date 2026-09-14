package state_test

import (
	"fmt"
	"strings"
	"testing"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit/state"
)

// caught is a Reporter that keeps what a guard reported instead of failing the
// test that checks the guard.
type caught struct{ errors []string }

func (c *caught) Helper() {}

func (c *caught) Errorf(format string, args ...any) {
	c.errors = append(c.errors, fmt.Sprintf(format, args...))
}

// fact is an entity.updated (or entity.created) as the guard reads it off the
// bus.
func fact(id, typ, source, entityID string, version int, changed []any) eventbus.Event {
	payload := map[string]any{
		"entity":  map[string]any{"entity": map[string]any{"id": entityID, "type": "player"}},
		"version": float64(version),
	}
	if typ == state.TypeUpdated {
		payload["changed"] = changed
	}
	return eventbus.Event{ID: id, Type: typ, Source: source, Payload: payload}
}

var aChange = []any{map[string]any{"path": "hp", "old": float64(10), "new": float64(9)}}

// The guard of one State per world (T-059, moved from cmd/multiverse): the
// mutants K5–K7 of T-056 are the rows that must be caught, and the empty
// changed[] and a world answered once are the rows that must not.
func TestOneStateOverTheWorld(t *testing.T) {
	for name, tc := range map[string]struct {
		events []eventbus.Event
		want   string
	}{
		"one State, every version once": {events: []eventbus.Event{
			fact("f1", state.TypeCreated, contracts.SourceState, "player-A", 1, nil),
			fact("f2", state.TypeUpdated, contracts.SourceState, "player-A", 2, aChange),
			{ID: "p1", Type: "entity.update.proposed", Source: contracts.SourceGateway},
		}},
		// K6: a turn that changed nothing is a second fact under the same
		// version, and a legal one (C-02).
		"a fact with an empty changed[] under the version before": {events: []eventbus.Event{
			fact("f1", state.TypeUpdated, contracts.SourceState, "player-A", 2, aChange),
			fact("f2", state.TypeUpdated, contracts.SourceState, "player-A", 2, []any{}),
		}},
		// K5: every answer of State published twice, as a second core/state
		// with the same ids — or a restart that publishes its catch-up again.
		"a fact published twice": {events: []eventbus.Event{
			fact("f1", state.TypeUpdated, contracts.SourceState, "player-A", 2, aChange),
			fact("f1", state.TypeUpdated, contracts.SourceState, "player-A", 2, aChange),
		}, want: "published twice under f1"},
		"a fact with an empty changed[] published twice": {events: []eventbus.Event{
			fact("f1", state.TypeUpdated, contracts.SourceState, "player-A", 2, []any{}),
			fact("f1", state.TypeUpdated, contracts.SourceState, "player-A", 2, []any{}),
		}, want: "published twice under f1"},
		// K7: a second FakeState over the world of the stand.
		"a fact of another publisher": {events: []eventbus.Event{
			fact("f1", state.TypeCreated, contracts.SourceTestkitState, "player-A", 1, nil),
		}, want: `published by "testkit/state"`},
		"one version announced by two facts": {events: []eventbus.Event{
			fact("f1", state.TypeUpdated, contracts.SourceState, "player-A", 2, aChange),
			fact("f2", state.TypeUpdated, contracts.SourceState, "player-A", 2, aChange),
		}, want: "player-A v2 is announced twice"},
	} {
		t.Run(name, func(t *testing.T) {
			var got caught
			state.OneStateOverTheWorld(&got, tc.events)
			switch {
			case tc.want == "" && len(got.errors) != 0:
				t.Errorf("the guard reported %q over one State", got.errors)
			case tc.want != "" && (len(got.errors) == 0 || !strings.Contains(strings.Join(got.errors, "\n"), tc.want)):
				t.Errorf("the guard reported %q, want %q", got.errors, tc.want)
			}
		})
	}
}
