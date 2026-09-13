package readmodel_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
)

const encounterID = "encounter-1"

func started(t *testing.T) eventbus.Event {
	t.Helper()
	ev := event(t, "started-1", readmodel.TypeEncounterStarted, "enter-A", map[string]any{
		"encounter":    ref(encounterID, entity.TypeEncounter),
		"region":       ref("dark-forest-01", entity.TypeRegion),
		"participants": []any{ref("player-A", entity.TypePlayer)},
		"npcs":         []any{ref("wolf-alpha", entity.TypeNPC)},
		"round":        map[string]any{"timeout": "60s", "idle_after_missed": 2},
	})
	ev.Scope = &eventbus.ScopeRef{ID: "player-A", Type: "solo"}
	return ev
}

func encounterCreated(t *testing.T) eventbus.Event {
	t.Helper()
	return created(t, encounterID, entity.TypeEncounter, "enter-A", map[string]any{
		"region_id": "dark-forest-01", "scope": map[string]any{"id": "player-A", "type": "solo"},
		"state": "active", "round_seq": 0, "opened_by_event_id": "started-1",
		"participants": []any{map[string]any{"player_id": "player-A", "state": "active", "damage_dealt": 0}},
		"npcs":         []any{map[string]any{"npc_id": "wolf-alpha"}},
	})
}

func resolvedFact(t *testing.T) eventbus.Event {
	t.Helper()
	return updated(t, "resolved-1", encounterID, entity.TypeEncounter, 2, "attack-A",
		set("state", "active", "resolved"), set("resolution", nil, "npc_dead"), set("closed_by_event_id", nil, "ended-1"))
}

func ended(t *testing.T) eventbus.Event {
	t.Helper()
	return event(t, "ended-1", readmodel.TypeEncounterEnded, "attack-A", map[string]any{
		"encounter": ref(encounterID, entity.TypeEncounter), "reason": "npc_dead", "rounds": 1,
		"killer": ref("player-A", entity.TypePlayer),
	})
}

// The end of an encounter is the first of the fact with state=resolved and
// encounter.ended; the two topics may deliver them in either order (C-04 v1.3,
// C-05 v1.4 p. 5). Both orders close the same encounter, the second event of
// the pair changes nothing and claims no end of its own, and an action after
// the end finds no encounter to act in (not_in_encounter, T-305).
func TestBothOrdersOfTheEndCloseTheSameEncounter(t *testing.T) {
	orders := map[string][]eventbus.Event{
		"fact first":  {resolvedFact(t), ended(t)},
		"event first": {ended(t), resolvedFact(t)},
	}
	var closed []readmodel.Encounter
	for name, order := range orders {
		t.Run(name, func(t *testing.T) {
			m, _ := newModel(t)
			mustApply(t, m, started(t))
			mustApply(t, m, encounterCreated(t))
			if _, ok := m.EncounterOf("player-A"); !ok {
				t.Fatal("the open encounter is not found for its participant")
			}

			first := mustApply(t, m, order[0])
			if first.EncounterEnded != encounterID {
				t.Errorf("the first end (%s) claimed %q, want the encounter", order[0].Type, first.EncounterEnded)
			}
			between, _ := m.Encounter(encounterID)
			second := mustApply(t, m, order[1])
			if second.EncounterEnded != "" {
				t.Errorf("the second end (%s) claimed %q again", order[1].Type, second.EncounterEnded)
			}
			after, _ := m.Encounter(encounterID)
			if after.State != entity.EncounterStateResolved || after.Resolution != "npc_dead" {
				t.Errorf("after both: %+v", after)
			}
			if between.State != entity.EncounterStateResolved {
				t.Errorf("after the first end the encounter is %q", between.State)
			}
			if _, ok := m.EncounterOf("player-A"); ok {
				t.Error("an action after the end still finds the encounter")
			}
			closed = append(closed, after)
		})
	}
	if len(closed) == 2 && !reflect.DeepEqual(closed[0], closed[1]) {
		t.Errorf("the two orders closed different encounters:\n%+v\n%+v", closed[0], closed[1])
	}
}

// encounter.started may arrive before entity.created: the encounter is held as
// announced, and both orders open the same encounter once both have arrived.
func TestBothOrdersOfTheStartOpenTheSameEncounter(t *testing.T) {
	var open []readmodel.Encounter
	for name, order := range map[string][]eventbus.Event{
		"start first":  {started(t), encounterCreated(t)},
		"entity first": {encounterCreated(t), started(t)},
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := newModel(t)
			first := mustApply(t, m, order[0])
			if first.EncounterOpened != encounterID {
				t.Errorf("the first opening (%s) claimed %q", order[0].Type, first.EncounterOpened)
			}
			if order[0].Type == readmodel.TypeEncounterStarted {
				enc, ok := m.EncounterOf("player-A")
				if !ok || enc.State != readmodel.EncounterAnnounced || enc.Version != 0 {
					t.Errorf("announced encounter = %+v %v", enc, ok)
				}
			}
			if second := mustApply(t, m, order[1]); second.EncounterOpened != "" {
				t.Errorf("the second opening (%s) claimed %q again", order[1].Type, second.EncounterOpened)
			}
			enc, ok := m.EncounterOf("player-A")
			if !ok || enc.State != entity.EncounterStateActive || enc.RegionID != "dark-forest-01" ||
				enc.Round == nil || enc.Round.Timeout != "60s" || enc.Version != 1 {
				t.Errorf("after both: %+v %v", enc, ok)
			}
			open = append(open, enc)
		})
	}
	if len(open) == 2 && !reflect.DeepEqual(open[0], open[1]) {
		t.Errorf("the two orders opened different encounters:\n%+v\n%+v", open[0], open[1])
	}
}

// A start that the topics deliver behind the end of its own fight does not
// reopen it, and opens nothing.
func TestAStartBehindItsEndOpensNothing(t *testing.T) {
	for name, end := range map[string]eventbus.Event{"by the event": ended(t), "by the fact": resolvedFact(t)} {
		t.Run(name, func(t *testing.T) {
			m, _ := newModel(t)
			if end.Type == readmodel.TypeEntityUpdated {
				mustApply(t, m, encounterCreated(t))
			}
			mustApply(t, m, end)
			if res := mustApply(t, m, started(t)); res.EncounterOpened != "" {
				t.Errorf("a start behind the end opened %q", res.EncounterOpened)
			}
			if enc, ok := m.Encounter(encounterID); !ok || enc.State != entity.EncounterStateResolved {
				t.Errorf("encounter = %+v %v, want resolved", enc, ok)
			}
			if _, ok := m.EncounterOf("player-A"); ok {
				t.Error("the ended encounter is found open")
			}
		})
	}
}

// The end belongs to the event that claimed it, not to the first call: a
// retry of that event after its effect failed reports the end again.
func TestARetryOfTheClaimingEventReportsTheEndAgain(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, encounterCreated(t))
	for i := range 2 {
		if res := mustApply(t, m, resolvedFact(t)); res.EncounterEnded != encounterID {
			t.Errorf("delivery %d of the claiming fact: EncounterEnded = %q", i+1, res.EncounterEnded)
		}
	}
	if res := mustApply(t, m, ended(t)); res.EncounterEnded != "" {
		t.Errorf("encounter.ended after the fact claimed %q", res.EncounterEnded)
	}
}

// The deadline of a wait for the answer to an action is not the end of the
// encounter: an agent that gave up publishes no refusal, the waiting gateway
// sees only its deadline, and the encounter ends by its fact or its event
// alone (C-05 v1.4 p. 1д).
func TestTheDeadlineOfAnAnswerDoesNotEndTheEncounter(t *testing.T) {
	m, manual := newModel(t)
	mustApply(t, m, started(t))
	mustApply(t, m, encounterCreated(t))

	wait := m.Expect("attack-A", 2*time.Second, "attack-A")
	manual.Advance(2 * time.Second)
	if _, err := wait.Wait(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait = %v, want the deadline", err)
	}
	enc, ok := m.EncounterOf("player-A")
	if !ok || !enc.Open() {
		t.Fatalf("after the deadline the encounter is %+v %v, want it open", enc, ok)
	}
	mustApply(t, m, ended(t))
	if _, ok := m.EncounterOf("player-A"); ok {
		t.Error("encounter.ended did not end the encounter")
	}
}

// The entity is the fact of State and the start only its announcement: where
// they disagree — a participant who joined, the round the fight is in — the
// encounter says what the entity says.
func TestTheEntityOfAnEncounterPrevailsOverItsStart(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, started(t))
	mustApply(t, m, encounterCreated(t))
	mustApply(t, m, updated(t, "u-2", encounterID, entity.TypeEncounter, 2, "attack-A",
		set("round_seq", 0, 3),
		set("participants", nil, []any{
			map[string]any{"player_id": "player-A", "state": "active", "damage_dealt": 4},
			map[string]any{"player_id": "player-B", "state": "active", "damage_dealt": 0},
		})))
	enc, ok := m.EncounterOf("player-B")
	if !ok || enc.RoundSeq != 3 || enc.Version != 2 || len(enc.Participants) != 2 || enc.Scope.ID != "player-A" {
		t.Errorf("encounter = %+v %v, want the participants and the round of the entity", enc, ok)
	}
}

// EncounterOf follows the encounter the entity describes: a participant who
// left is no longer in it, one who joined is, and after the end nobody is,
// while a later encounter of the same player is found.
func TestEncounterOfFollowsTheParticipantsAndTheEnd(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, started(t))
	mustApply(t, m, encounterCreated(t))
	mustApply(t, m, updated(t, "u-2", encounterID, entity.TypeEncounter, 2, "attack-A",
		set("participants", nil, []any{map[string]any{"player_id": "player-B", "state": "active", "damage_dealt": 0}})))
	if _, ok := m.EncounterOf("player-A"); ok || readmodel.IndexedEncounters(m, "player-A") != 0 {
		t.Error("a participant who left still finds the encounter")
	}
	if enc, ok := m.EncounterOf("player-B"); !ok || enc.ID != encounterID {
		t.Errorf("the participant who joined finds %+v %v", enc, ok)
	}
	mustApply(t, m, ended(t))
	if _, ok := m.EncounterOf("player-B"); ok || readmodel.IndexedEncounters(m, "player-B") != 0 {
		t.Error("the encounter is found, or kept in the index, after its end")
	}
	next := created(t, "encounter-2", entity.TypeEncounter, "enter-B", map[string]any{
		"region_id": "dark-forest-01", "state": "active",
		"participants": []any{map[string]any{"player_id": "player-B", "state": "active", "damage_dealt": 0}},
	})
	mustApply(t, m, next)
	if enc, ok := m.EncounterOf("player-B"); !ok || enc.ID != "encounter-2" {
		t.Errorf("the next encounter of the player = %+v %v", enc, ok)
	}
}

// restarted is the projection of a process that starts again from a snapshot
// of State holding entities, and has seen no event since.
func restarted(t *testing.T, entities ...*entity.Entity) *readmodel.Model {
	t.Helper()
	ctx := context.Background()
	raw, err := json.Marshal(entities)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []*entity.Entity
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	const key = "state/20260913T120000Z-000001.json"
	pointer, err := json.Marshal(map[string]any{"component": "state", "snapshot": map[string]any{
		"key": key, "cursor": map[string]int64{"system_events": 2}, "state_hash": entity.StateHash(decoded)}})
	if err != nil {
		t.Fatal(err)
	}
	object, err := json.Marshal(map[string]any{"component": "state", "world_id": world, "entities": decoded})
	if err != nil {
		t.Fatal(err)
	}
	store := objstore.NewMemory()
	bucket := objstore.SnapshotsBucket(world)
	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatal(err)
	}
	for k, body := range map[string][]byte{readmodel.StatePointerKey: pointer, key: object} {
		if _, err := store.Put(ctx, bucket, k, body, objstore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	m, _ := newModel(t)
	if _, err := m.LoadFromStateSnapshot(ctx, store, world); err != nil {
		t.Fatal(err)
	}
	return m
}

func encounterEntity(state string, version int64) *entity.Entity {
	e := entity.New(entity.Ref{ID: encounterID, Type: entity.TypeEncounter}, world, encounterID, map[string]any{
		"region_id": "dark-forest-01", "state": state, "resolution": "npc_dead",
		"participants": []any{map[string]any{"player_id": "player-A", "state": "active", "damage_dealt": 0}},
	}, t0)
	e.Version = version
	return e
}

// The claim of a transition lives in memory and does not survive a restart.
// A process that stopped after the first event of a pair took its effects,
// and starts again from a snapshot of State that already holds the encounter,
// reports the transition once more for the second event of the pair. This
// pins the contract of Result for its consumers: an effect of a transition is
// idempotent by the encounter in gateway.db (T-307, T-351; review #1, Mi-2).
// A claim kept durably would turn this test red, and Result, T-307 and T-351
// would change with it.
func TestTheClaimOfATransitionDoesNotSurviveARestart(t *testing.T) {
	t.Run("end", func(t *testing.T) {
		before, _ := newModel(t)
		mustApply(t, before, encounterCreated(t))
		if res := mustApply(t, before, resolvedFact(t)); res.EncounterEnded != encounterID {
			t.Fatalf("the fact before the restart claimed %q", res.EncounterEnded)
		}
		after := restarted(t, encounterEntity(entity.EncounterStateResolved, 2))
		if _, ok := after.EncounterOf("player-A"); ok || readmodel.IndexedEncounters(after, "player-A") != 0 {
			t.Error("the encounter resolved in the snapshot is found open")
		}
		if res := mustApply(t, after, ended(t)); res.EncounterEnded != encounterID {
			t.Errorf("encounter.ended after the restart claimed %q; if the claim is durable now, Result, T-307 and T-351 change with it",
				res.EncounterEnded)
		}
	})
	t.Run("opening", func(t *testing.T) {
		after := restarted(t, encounterEntity(entity.EncounterStateActive, 1))
		if enc, ok := after.EncounterOf("player-A"); !ok || enc.ID != encounterID {
			t.Errorf("the open encounter of the snapshot = %+v %v", enc, ok)
		}
		if res := mustApply(t, after, started(t)); res.EncounterOpened != encounterID {
			t.Errorf("encounter.started after the restart claimed %q; if the claim is durable now, Result and T-307 change with it",
				res.EncounterOpened)
		}
	})
}

// The gateway answers encounter_unavailable while an active encounter has no
// task agent for longer than its grace (component §5.4 p. 3), so the projection
// carries the agent and the time the encounter was created, both from the
// entity: the announcement alone knows neither.
func TestTheEncounterCarriesItsTaskAgentAndItsCreation(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, started(t))
	enc, ok := m.Encounter(encounterID)
	if !ok || enc.TaskAgentID != "" || !enc.CreatedAt.IsZero() {
		t.Fatalf("announced encounter = %+v %v, want no agent and no creation time", enc, ok)
	}
	mustApply(t, m, encounterCreated(t))
	if enc, _ = m.Encounter(encounterID); enc.TaskAgentID != "" || !enc.CreatedAt.Equal(t0) {
		t.Errorf("created encounter without an agent = %q at %v, want none at %v", enc.TaskAgentID, enc.CreatedAt, t0)
	}
	mustApply(t, m, updated(t, "u-agent", encounterID, entity.TypeEncounter, 2, "spawn-1",
		set("task_agent_id", nil, "encounter-wolf:solo:player-A")))
	if enc, _ = m.Encounter(encounterID); enc.TaskAgentID != "encounter-wolf:solo:player-A" || !enc.CreatedAt.Equal(t0) {
		t.Errorf("encounter after the spawn = %q at %v", enc.TaskAgentID, enc.CreatedAt)
	}
}
