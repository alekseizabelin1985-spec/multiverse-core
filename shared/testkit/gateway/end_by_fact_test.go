package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
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

// TestAStartHeardAfterItsEndEndsTheWaitOfItsCharacter: the end came from the
// fact, which names the encounter and no character, so the harness did not know
// whose fight was over and let a flight go out into it. The start that arrives
// while the flight waits says whose it was, and the flight ends with
// ErrFightOver instead of the deadline (T-433).
func TestAStartHeardAfterItsEndEndsTheWaitOfItsCharacter(t *testing.T) {
	bus := newBus(t)
	h := knowing(t, bus, nil)
	publish(t, bus, resolvedFact(t, "encounter-1", entity.ResolutionNPCDead))
	waitFor(t, "the harness to fold the fact", func() bool {
		_, known := h.Version("encounter-1")
		return known
	})

	fled := make(chan error, 1)
	go func() { fled <- h.Flee(t.Context(), playerA) }()
	waitFor(t, "the flight to go out", func() bool {
		return len(ofType(read(t, bus, eventbus.TopicPlayerEvents), gateway.TypeFleeAttempted)) == 1
	})
	opens(t, bus, h, "encounter-1", playerA)

	err := <-fled
	if !errors.Is(err, gateway.ErrFightOver) {
		t.Fatalf("the flight ended with %v; a start heard after its end says the fight "+
			"is over, and a wait inside it is ErrFightOver", err)
	}
	if !strings.Contains(err.Error(), entity.ResolutionNPCDead) {
		t.Errorf("the error is %q; it should name how the fight ended", err)
	}
}

// TestAFightHeardOverWhileTheActionIsBuiltEndsIt: between the check that the
// fight is still on and the moment the action starts to wait, the harness lets
// go of its lock to build the action. A start or an end heard in that moment
// ends every wait it finds, and the wait of this action is not one of them yet;
// nobody answers an action outside a fight, so that action used to wait out the
// deadline (Mi-1 of review #1 of T-433).
//
// The moment is hit exactly rather than hoped for: the identifier of the action
// is drawn there (Harness.root), so the source of identifiers is where the test
// hands the harness what it hears in between.
func TestAFightHeardOverWhileTheActionIsBuiltEndsIt(t *testing.T) {
	cases := map[string]struct {
		// before is what the harness heard before the action was checked.
		before func(t *testing.T, bus *membus.Bus, h *gateway.Harness)
		// between is what it hears while the action is being built.
		between func(t *testing.T) eventbus.Event
	}{
		"the start of a fight already heard to end": {
			before: func(t *testing.T, bus *membus.Bus, h *gateway.Harness) {
				publish(t, bus, resolvedFact(t, "encounter-1", entity.ResolutionNPCDead))
				waitFor(t, "the harness to fold the fact", func() bool {
					_, known := h.Version("encounter-1")
					return known
				})
			},
			between: func(t *testing.T) eventbus.Event { return startOf(t, "encounter-1", playerA) },
		},
		"the end of a fight already heard to start": {
			before: func(t *testing.T, bus *membus.Bus, h *gateway.Harness) {
				opens(t, bus, h, "encounter-1", playerA)
			},
			between: func(t *testing.T) eventbus.Event {
				return resolvedFact(t, "encounter-1", entity.ResolutionNPCDead)
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			bus := newBus(t)
			h := knowing(t, bus, nil)
			c.before(t, bus, h)
			heard := asDelivered(t, c.between(t))

			ids := eventbus.SequenceIDs("window")
			var once sync.Once
			eventbus.SetIDSource(func() string {
				once.Do(func() {
					if err := h.Observe(context.Background(), heard); err != nil {
						t.Errorf("observe %s: %v", heard.Type, err)
					}
				})
				return ids()
			})

			err := h.Flee(t.Context(), playerA)
			if !errors.Is(err, gateway.ErrFightOver) {
				t.Fatalf("the flight ended with %v; a fight heard over before the flight "+
					"started to wait is ErrFightOver", err)
			}
			if fled := ofType(read(t, bus, eventbus.TopicPlayerEvents),
				gateway.TypeFleeAttempted); len(fled) != 0 {
				t.Errorf("%d flights went out into a fight the harness knew was over", len(fled))
			}
		})
	}
}

// startOf is encounter.started around one character, as opens publishes it.
func startOf(t *testing.T, encounterID, player string) eventbus.Event {
	t.Helper()
	fixtures := loadFixtures(t)
	return eventbus.NewRoot(gateway.TypeEncounterStarted, contracts.SourceTestkitSwarm,
		worldID, nil, eventbus.ActorSystem, map[string]any{
			"encounter":    ref(encounterID, entity.TypeEncounter),
			"region":       named(pick(t, fixtures, regionID)),
			"participants": []any{named(pick(t, fixtures, player))},
			"npcs":         []any{named(pick(t, fixtures, npcID))},
		}, eventbus.WithAgent(agent()))
}

// asDelivered is the event as a subscriber is handed it — through JSON, the
// way the bus carries it — for a test that calls Observe itself.
func asDelivered(t *testing.T, ev eventbus.Event) eventbus.Event {
	t.Helper()
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal %s: %v", ev.Type, err)
	}
	var out eventbus.Event
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", ev.Type, err)
	}
	return out
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
