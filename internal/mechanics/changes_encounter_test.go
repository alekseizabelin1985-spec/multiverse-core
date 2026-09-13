package mechanics_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	mech "multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

// This file is the check tasks.md T-053 asks for: the package the encounter
// double of EPIC-003 builds by hand (FakeEncounter.wound, pack) and the one
// ChangesFor builds from the same outcome are the same package.
//
// The double runs here on the real rule set — *mechanics.Rules is its
// Mechanics — so the dice, the outcomes and the package on the bus are those of
// the real Resolve. The package is compared with ChangesFor over the very
// outcomes and actors the double resolved, which a spy on its Mechanics
// records; Action.At is set to the time of the cause, as C-03 v1.3 has every
// caller do.
//
// Everything of a fighter matches exactly — the operations, expected_version,
// cause, died_at and acquired_at included — except the differences named below.
// Each required one is proven to occur, the tolerated one may or may not, and
// the encounter entity, which is the
// package of the encounter agent and not of ChangesFor, is held to the paths
// the double is known to write. shared/testkit/swarm is not changed by this
// task.

const (
	worldID  = "dark-forest-world"
	regionID = "dark-forest-01"
	playerA  = "player-A"
	wolfID   = "wolf-alpha"
)

// The differences between the double and ChangesFor, by what they are.
const (
	// gapPlayerDeathRecord: the double writes died_at and killed_by on a
	// fallen character too. data-model.md §3.3 has neither on a character and
	// §4.6 gives the task level no such path on a player. A defect of the
	// double, a task of EPIC-003 (T-452); ChangesFor is right. Tolerated, not
	// required: the test passes with the defect and without it, so that it
	// stays green once T-452 is merged next to this task.
	gapPlayerDeathRecord = "the double writes an NPC death record on a character (defect of the double, EPIC-003 task)"
	// gapFleePosition: the position of a character who got away is the
	// caller's, written with Rules.FleePosition (C-03 v1.3); ChangesFor
	// does not produce it.
	gapFleePosition = "the position after an escape is the caller's (Rules.FleePosition)"
	// gapLastDamager: npcs[].last_damager of the encounter is written by the
	// package of the encounter agent (EPIC-003, C-03 v1.3), and the package of
	// ChangesFor has none — nor anything else of the encounter.
	gapLastDamager = "npcs[].last_damager is not in the package of ChangesFor"
)

// encounterPaths are the paths of the encounter entity the double writes in a
// package of a fight (FakeEncounter.turnOps). A path outside them — npcs[]
// above all — is a change of the encounter nobody has compared yet.
var encounterPaths = []string{
	entity.AttrRoundSeq, entity.AttrParticipants, entity.AttrState,
	entity.AttrResolution, entity.AttrClosedByEventID,
}

// spy records what the double asked the real rule set to decide.
type spy struct {
	rules *mech.Rules
	mu    sync.Mutex
	calls []decision
}

type decision struct {
	cause  string
	start  int
	action mech.Action
	actors map[string]mech.Actor
	out    mech.Outcome
	rolls  []mech.Roll
}

func (s *spy) Resolve(cause string, start int, a mech.Action,
	actors map[string]*mech.Actor) (mech.Outcome, []mech.Roll, error) {
	seen := make(map[string]mech.Actor, len(actors))
	for id, who := range actors {
		seen[id] = *who
	}
	out, rolls, err := s.rules.Resolve(cause, start, a, actors)
	if err == nil {
		s.mu.Lock()
		s.calls = append(s.calls, decision{cause: cause, start: start, action: a, actors: seen, out: out, rolls: rolls})
		s.mu.Unlock()
	}
	return out, rolls, err
}

func (s *spy) of(cause string) []decision {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []decision
	for _, d := range s.calls {
		if d.cause == cause {
			out = append(out, d)
		}
	}
	return out
}

// TestChangesForMatchesTheEncounterDouble runs one exchange of each shape
// through the double on the membus and compares its package with ChangesFor.
func TestChangesForMatchesTheEncounterDouble(t *testing.T) {
	rules, err := mech.Load(filepath.Join("..", "..", "rules", "dark-forest.yaml"))
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	actorsOf := func(t *testing.T, world []*entity.Entity) (player, wolf mech.Actor) {
		t.Helper()
		for _, e := range world {
			switch e.ID {
			case playerA:
				a, err := mech.ActorFromEntity(e, nil)
				if err != nil {
					t.Fatalf("player: %v", err)
				}
				player = *a
			case wolfID:
				a, err := mech.ActorFromEntity(e, nil)
				if err != nil {
					t.Fatalf("wolf: %v", err)
				}
				wolf = *a
			}
		}
		return player, wolf
	}
	// attackLands says how an attack with this cause goes from the fixtures:
	// whether the blow lands and kills, and whether the bite that answers it
	// lands and kills.
	attackLands := func(t *testing.T, world []*entity.Entity, cause string) (hit, kill, bite, bitten bool) {
		t.Helper()
		player, wolf := actorsOf(t, world)
		actors := map[string]*mech.Actor{playerA: &player, wolfID: &wolf}
		out, _, err := rules.Resolve(cause, 0, mech.Action{Kind: mech.ActionAttack, Actor: playerA, Target: wolfID}, actors)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if out.TargetDead {
			return out.Hit, true, false, false
		}
		wolf.HP = out.HPAfter
		answer, _, err := rules.Resolve(cause, 2, mech.Action{Kind: mech.ActionNPCAttack, Actor: wolfID, Target: playerA}, actors)
		if err != nil {
			t.Fatalf("resolve the bite: %v", err)
		}
		return out.Hit, false, answer.Hit, answer.TargetDead
	}
	fleeGoes := func(t *testing.T, world []*entity.Entity, cause string) (escaped, bite bool) {
		t.Helper()
		player, wolf := actorsOf(t, world)
		actors := map[string]*mech.Actor{playerA: &player, wolfID: &wolf}
		out, _, err := rules.Resolve(cause, 0, mech.Action{Kind: mech.ActionFlee, Actor: playerA, LivingEnemies: 1}, actors)
		if err != nil {
			t.Fatalf("resolve the flight: %v", err)
		}
		if *out.Success {
			return true, false
		}
		answer, _, err := rules.Resolve(cause, 1, mech.Action{Kind: mech.ActionFreeAttack, Actor: wolfID, Target: playerA}, actors)
		if err != nil {
			t.Fatalf("resolve the free attack: %v", err)
		}
		return false, answer.Hit
	}

	seen := map[string]bool{}
	scenarios := []struct {
		name   string
		change func(e *entity.Entity)
		action func() eventbus.Event
		want   func(t *testing.T, world []*entity.Entity, cause string) bool
	}{
		{
			name:   "both sides land",
			action: attackEvent,
			want: func(t *testing.T, world []*entity.Entity, cause string) bool {
				hit, kill, bite, bitten := attackLands(t, world, cause)
				return hit && !kill && bite && !bitten
			},
		},
		{
			name:   "a killing blow hands out the trophy",
			change: setAttr(wolfID, entity.AttrHP, 1),
			action: attackEvent,
			want: func(t *testing.T, world []*entity.Entity, cause string) bool {
				_, kill, _, _ := attackLands(t, world, cause)
				return kill
			},
		},
		{
			name:   "a character falls to the bite",
			change: setAttr(playerA, entity.AttrHP, 1),
			action: attackEvent,
			want: func(t *testing.T, world []*entity.Entity, cause string) bool {
				hit, _, _, bitten := attackLands(t, world, cause)
				return !hit && bitten
			},
		},
		{
			name:   "a failed flight is punished",
			action: fleeEvent,
			want: func(t *testing.T, world []*entity.Entity, cause string) bool {
				escaped, bite := fleeGoes(t, world, cause)
				return !escaped && bite
			},
		},
		{
			name:   "an escape",
			action: fleeEvent,
			want: func(t *testing.T, world []*entity.Entity, cause string) bool {
				escaped, _ := fleeGoes(t, world, cause)
				return escaped
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			testkit.Deterministic(t, "t053")
			bus := newBus(t)
			world, err := state.LoadFixtures(filepath.Join("..", "..", "testdata", "fixtures"))
			if err != nil {
				t.Fatalf("fixtures: %v", err)
			}
			if sc.change != nil {
				for _, e := range world {
					sc.change(e)
				}
			}
			decide := &spy{rules: rules}
			enc, err := swarm.NewFakeEncounter(swarm.EncounterConfig{
				Bus: bus, WorldID: worldID, Rules: rules, Mechanics: decide,
			})
			if err != nil {
				t.Fatalf("encounter: %v", err)
			}
			if err := enc.Seed(world); err != nil {
				t.Fatalf("seed: %v", err)
			}
			st, err := state.New(state.Config{Bus: bus, WorldID: worldID})
			if err != nil {
				t.Fatalf("state: %v", err)
			}
			if err := st.Seed(world); err != nil {
				t.Fatalf("seed state: %v", err)
			}
			read := 0
			answer := func() {
				t.Helper()
				for {
					events := eventsOf(t, bus, eventbus.TopicSystemEvents)
					if read >= len(events) {
						return
					}
					ev := events[read]
					read++
					switch ev.Source {
					case contracts.SourceTestkitSwarm:
						if err := st.Apply(t.Context(), ev); err != nil {
							t.Fatalf("state on %s: %v", ev.Type, err)
						}
					case contracts.SourceTestkitState:
						if err := enc.Observe(t.Context(), ev); err != nil {
							t.Fatalf("observe %s: %v", ev.Type, err)
						}
					}
				}
			}

			if err := enc.Act(t.Context(), enteredEvent()); err != nil {
				t.Fatalf("enter: %v", err)
			}
			answer()
			if _, open := enc.ActiveEncounter(playerA); !open {
				t.Fatal("the fight did not open: the comparison would compare nothing")
			}

			var action eventbus.Event
			for range 500 {
				candidate := sc.action()
				if sc.want(t, world, candidate.ID) {
					action = candidate
					break
				}
			}
			if action.ID == "" {
				t.Fatal("no cause out of 500 lands on this scenario")
			}
			if err := enc.Act(t.Context(), action); err != nil {
				t.Fatalf("act: %v", err)
			}

			decisions := decide.of(action.ID)
			compareRolls(t, bus, action.ID, decisions)
			proposal := proposalOf(t, bus, action.ID)
			facts := decidedOf(t, bus, action.ID)
			if len(facts) != len(decisions) {
				t.Fatalf("%d combat.decided for %d decisions", len(facts), len(decisions))
			}

			ours := map[string]*mech.ProposedChange{}
			var order []string
			playerLanded := false
			for i, d := range decisions {
				attacker := d.actors[d.action.Actor]
				var target *mech.Actor
				if d.action.Target != "" {
					who := d.actors[d.action.Target]
					target = &who
				}
				d.action.At = action.Timestamp
				if d.action.Kind == mech.ActionAttack && d.out.Hit {
					playerLanded = true
				}
				changes, err := mech.ChangesFor(d.action, d.out, &attacker, target, facts[i])
				if err != nil {
					t.Fatalf("changes for %s: %v", d.action.Kind, err)
				}
				for j := range changes {
					c := &changes[j]
					if c.Entity.Type == entity.TypeEncounter {
						t.Errorf("ChangesFor changes the encounter %s: that is the package of the encounter agent", c.Entity)
					}
					if have, ok := ours[c.Entity.ID]; ok {
						if have.Cause != c.Cause {
							t.Errorf("one package, two causes for %s: %s and %s", c.Entity, have.Cause, c.Cause)
						}
						if *have.ExpectedVersion != *c.ExpectedVersion {
							t.Errorf("one package, two versions for %s: %d and %d", c.Entity, *have.ExpectedVersion, *c.ExpectedVersion)
						}
						have.Ops = append(have.Ops, c.Ops...)
						continue
					}
					ours[c.Entity.ID] = c
					order = append(order, c.Entity.ID)
				}
			}

			cause, _ := proposal.Path().GetString("cause")
			var theirs []string
			for _, set := range changeSetsOf(t, proposal) {
				ref := entity.RefFrom(set.Entity.Entity)
				if ref.Type == entity.TypeEncounter {
					for _, op := range set.Ops {
						if !slices.Contains(encounterPaths, op.Path) {
							t.Errorf("the double writes %s of the encounter: not among %v, compare it with the package of the encounter agent",
								op.Path, encounterPaths)
						}
					}
					continue
				}
				ops := dropOp(set.Ops, entity.AttrPosition, func(op entity.Op) {
					if want := rules.FleePosition(worldID, regionID); op.Value != want {
						t.Errorf("the double places the escaped character at %v, Rules.FleePosition says %s", op.Value, want)
					}
					seen[gapFleePosition] = true
				})
				if ref.Type == entity.TypePlayer {
					before := len(ops)
					ops = dropOp(dropOp(ops, entity.AttrDiedAt, nil), entity.AttrKilledBy, nil)
					if len(ops) != before {
						seen[gapPlayerDeathRecord] = true
					}
				}
				if len(ops) == 0 {
					continue // an escape: nothing but the position
				}
				theirs = append(theirs, ref.ID)

				mine, ok := ours[ref.ID]
				if !ok {
					t.Errorf("the double changes %s with %v, ChangesFor does not", ref, set.Ops)
					continue
				}
				if mine.Entity != ref {
					t.Errorf("entity %s, ChangesFor names %s", ref, mine.Entity)
				}
				switch {
				case set.ExpectedVersion == nil || mine.ExpectedVersion == nil:
					t.Errorf("expected_version of %s: the double %v, ChangesFor %v", ref, set.ExpectedVersion, mine.ExpectedVersion)
				case *set.ExpectedVersion != *mine.ExpectedVersion:
					t.Errorf("expected_version of %s: the double %d, ChangesFor %d", ref, *set.ExpectedVersion, *mine.ExpectedVersion)
				}
				if mine.Cause != cause {
					t.Errorf("cause of %s: the double %q, ChangesFor %q", ref, cause, mine.Cause)
				}
				if have, want := normalized(t, mine.Ops), normalized(t, ops); have != want {
					t.Errorf("ops of %s\n  ChangesFor: %s\n  the double: %s", ref, have, want)
				}
			}
			if !slices.Equal(order, theirs) {
				t.Errorf("entities in order %v, the double %v", order, theirs)
			}
			if playerLanded {
				// The blow moved the last damager of the NPC, and nothing of it
				// is in the package of ChangesFor: that is the encounter agent's.
				for _, mine := range ours {
					for _, op := range mine.Ops {
						if op.Path == entity.AttrNPCs {
							t.Errorf("ChangesFor writes %s of %s", op.Path, mine.Entity)
						}
					}
				}
				seen[gapLastDamager] = true
			}
		})
	}

	// gapPlayerDeathRecord is not among them: it is tolerated, not required.
	for _, gap := range []string{gapFleePosition, gapLastDamager} {
		if !seen[gap] {
			t.Errorf("the gap %q did not occur: drop it from the list, or the scenarios stopped covering it", gap)
		}
	}
}

// compareRolls holds the dice the double published to the rolls the real
// Resolve returned — index, formula, seed, result and purpose, in order. The
// indices of a flight are those of §5.4: the flight at 0 and the free attack
// from 1 (FakeEncounter rollFlee, rollFreeAttack).
func compareRolls(t *testing.T, bus *membus.Bus, cause string, decisions []decision) {
	t.Helper()
	var want []map[string]any
	for _, d := range decisions {
		for _, r := range d.rolls {
			want = append(want, map[string]any{
				"index": float64(r.Index), "formula": r.Formula, "seed": strconv.FormatUint(r.Seed, 10),
				"result": float64(r.Result), "natural": float64(r.Natural), "purpose": r.Purpose,
			})
		}
	}
	var have []map[string]any
	for _, ev := range ofTypeCausedBy(t, bus, eventbus.TopicGameEvents, swarm.TypeDiceRolled, cause) {
		roll, _ := ev.Payload["roll"].(map[string]any)
		have = append(have, map[string]any{
			"index": roll["index"], "formula": roll["formula"], "seed": roll["seed"],
			"result": roll["result"], "natural": roll["natural"], "purpose": ev.Payload["purpose"],
		})
	}
	if !reflect.DeepEqual(have, want) {
		t.Errorf("dice on the bus\n  %v\nResolve rolled\n  %v", have, want)
	}
	if len(decisions) > 0 && decisions[0].action.Kind == mech.ActionFlee {
		starts := make([]int, 0, len(decisions))
		for _, d := range decisions {
			starts = append(starts, d.start)
		}
		if want := []int{0, 1}[:len(decisions)]; !slices.Equal(starts, want) {
			t.Errorf("a flight resolved from indices %v, want %v", starts, want)
		}
	}
}

// normalized renders operations the way they travel — through JSON — so that
// an int of Go and a float64 read off the bus compare equal.
func normalized(t *testing.T, ops []entity.Op) string {
	t.Helper()
	raw, err := json.Marshal(ops)
	if err != nil {
		t.Fatalf("marshal ops: %v", err)
	}
	var generic []map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal ops: %v", err)
	}
	out, err := json.Marshal(generic)
	if err != nil {
		t.Fatalf("marshal normalized ops: %v", err)
	}
	return string(out)
}

// dropOp takes every operation on one path out, telling found about each.
func dropOp(ops []entity.Op, path string, found func(entity.Op)) []entity.Op {
	out := make([]entity.Op, 0, len(ops))
	for _, op := range ops {
		if op.Path == path {
			if found != nil {
				found(op)
			}
			continue
		}
		out = append(out, op)
	}
	return out
}

// --- the bus ---

func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{
		Registry: contracts.Default(),
		Topics:   topics,
		Backoff:  []time.Duration{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func eventsOf(t *testing.T, bus *membus.Bus, topic string) []eventbus.Event {
	t.Helper()
	records, err := bus.Records(topic)
	if err != nil {
		t.Fatalf("read %s: %v", topic, err)
	}
	out := make([]eventbus.Event, 0, len(records))
	for i, body := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatalf("decode %s[%d]: %v", topic, i, err)
		}
		out = append(out, ev)
	}
	return out
}

func ofTypeCausedBy(t *testing.T, bus *membus.Bus, topic, typ, cause string) []eventbus.Event {
	t.Helper()
	var out []eventbus.Event
	for _, ev := range eventsOf(t, bus, topic) {
		if ev.Type == typ && ev.Meta.CausationID == cause {
			out = append(out, ev)
		}
	}
	return out
}

func proposalOf(t *testing.T, bus *membus.Bus, cause string) eventbus.Event {
	t.Helper()
	got := ofTypeCausedBy(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed, cause)
	if len(got) != 1 {
		t.Fatalf("%d packages answer %s, want one", len(got), cause)
	}
	return got[0]
}

func decidedOf(t *testing.T, bus *membus.Bus, cause string) []string {
	t.Helper()
	var ids []string
	for _, ev := range ofTypeCausedBy(t, bus, eventbus.TopicGameEvents, swarm.TypeCombatDecided, cause) {
		ids = append(ids, ev.ID)
	}
	return ids
}

func changeSetsOf(t *testing.T, proposal eventbus.Event) []entity.ChangeSet {
	t.Helper()
	var payload struct {
		Changes []entity.ChangeSet `json:"changes"`
	}
	raw, err := json.Marshal(proposal.Payload)
	if err != nil {
		t.Fatalf("marshal the proposal: %v", err)
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode the proposal: %v", err)
	}
	return payload.Changes
}

// --- the actions ---

func playerAction(typ string, payload map[string]any) eventbus.Event {
	return eventbus.NewRoot(typ, contracts.SourceTestkitGateway, worldID,
		&eventbus.ScopeRef{ID: playerA, Type: "solo"}, entity.ActorKindCI, payload)
}

func actorPayload() map[string]any {
	return map[string]any{"entity": map[string]any{"id": playerA, "type": entity.TypePlayer}, "name": "Вася"}
}

func enteredEvent() eventbus.Event {
	return playerAction(swarm.TypeEnteredRegion, map[string]any{
		"entity": actorPayload(),
		"target": map[string]any{
			"entity": map[string]any{"id": regionID, "type": entity.TypeRegion},
			"name":   "Тёмный лес",
		},
		"position": map[string]any{"from": nil, "to": regionID},
	})
}

func attackEvent() eventbus.Event {
	return playerAction(swarm.TypeAttacked, map[string]any{
		"entity": actorPayload(),
		"action": map[string]any{"type": "attack"},
		"target": map[string]any{"entity": map[string]any{"id": wolfID, "type": entity.TypeNPC}},
	})
}

func fleeEvent() eventbus.Event {
	return playerAction(swarm.TypeFleeAttempted, map[string]any{
		"entity": actorPayload(),
		"action": map[string]any{"type": "flee"},
	})
}

func setAttr(id, path string, value any) func(*entity.Entity) {
	return func(e *entity.Entity) {
		if e.ID == id {
			e.Attributes[path] = value
		}
	}
}
