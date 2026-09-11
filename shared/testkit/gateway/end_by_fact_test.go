package gateway_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway"
)

// For a consumer of state the end of a fight is the fact of the encounter
// entity resolved; encounter.ended follows that fact and never precedes it
// (C-05 v1.4 p. 5, C-04 v1.3). The two cases below are the harness side of it
// (T-419): a harness that knew the end only from the event would strike its
// next blow into a fight State has already closed.

// TestTheFactOfTheEncounterEndsTheFight: the fact alone is enough, and a blow
// after it is refused on the spot rather than waited out.
func TestTheFactOfTheEncounterEndsTheFight(t *testing.T) {
	bus := newBus(t)
	h := knowing(t, bus, nil)
	opens(t, bus, h, "encounter-1", playerA)

	publish(t, bus, resolvedFact(t, "encounter-1", entity.ResolutionNPCDead))
	waitFor(t, "the harness to hear that the fight is over", func() bool {
		_, ended, _ := h.Fight(playerA)
		return ended != ""
	})

	err := h.Attack(t.Context(), playerA, npcID)
	if !errors.Is(err, gateway.ErrFightOver) {
		t.Fatalf("the error is %v; a blow into a fight State has closed is ErrFightOver", err)
	}
	if !strings.Contains(err.Error(), entity.ResolutionNPCDead) {
		t.Errorf("the error is %q; it should name how the fight ended", err)
	}
}

// TestAnEndHeardBeforeTheStartKeepsTheFightOver: the fact travels on
// system_events and the start on world_events, and C-01 orders no two topics.
// A start that arrives behind the end of its own fight does not reopen it.
func TestAnEndHeardBeforeTheStartKeepsTheFightOver(t *testing.T) {
	bus := newBus(t)
	h := knowing(t, bus, nil)
	publish(t, bus, resolvedFact(t, "encounter-1", entity.ResolutionPlayersOut))
	waitFor(t, "the harness to fold the fact", func() bool {
		_, known := h.Version("encounter-1")
		return known
	})

	opens(t, bus, h, "encounter-1", playerA)

	if _, ended, _ := h.Fight(playerA); ended != entity.ResolutionPlayersOut {
		t.Errorf("the fight is %q after its start arrived behind its end, want %q",
			ended, entity.ResolutionPlayersOut)
	}
}

// resolvedFact is the entity.updated State publishes when it applied the package
// that closes an encounter.
func resolvedFact(t *testing.T, encounterID, reason string) eventbus.Event {
	t.Helper()
	return eventbus.NewRoot(gateway.TypeUpdated, contracts.SourceTestkitState, worldID, nil,
		entity.ActorKindCI, map[string]any{
			"entity":  ref(encounterID, entity.TypeEncounter),
			"version": 2,
			"changed": []any{
				map[string]any{"path": entity.AttrState,
					"old": entity.EncounterStateActive, "new": entity.EncounterStateResolved},
				map[string]any{"path": entity.AttrResolution, "new": reason},
			},
			"cause":       "combat",
			"proposal_id": "closing-" + encounterID,
			"applied_at":  testkit.Epoch.Format(time.RFC3339),
		})
}
