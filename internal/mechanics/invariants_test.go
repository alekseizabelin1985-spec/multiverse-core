package mechanics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"multiverse-core.io/shared/entity"
)

// --- the view the checks are handed ---

const (
	invWorld  = "dark-forest-world"
	invRegion = "dark-forest-01"
)

// mapView is a world in a map. ByType answers in map order on purpose: a check
// whose answer depends on the order the world is listed in is a check a replay
// cannot trust, and the determinism test runs it often enough to see.
type mapView struct {
	world string
	byID  map[string]*entity.Entity
}

func (v mapView) Get(id string) (*entity.Entity, bool) {
	e, ok := v.byID[id]
	return e, ok
}

func (v mapView) ByType(t string) []*entity.Entity {
	var out []*entity.Entity
	for _, e := range v.byID {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func (v mapView) WorldID() string { return v.world }

// guardianView is the method set the guardian of the swarm declares as its
// InvariantView (ADR-001, addendum 2026-09-13 p. 1): the adapter in
// internal/swarm rests on it being the method set of StateView. The two
// assignments below fail to compile the moment either side grows or loses a
// method.
type guardianView interface {
	Get(id string) (*entity.Entity, bool)
	ByType(t string) []*entity.Entity
	WorldID() string
}

var (
	_ StateView    = guardianView(nil)
	_ guardianView = StateView(nil)
	_ StateView    = mapView{}
)

// worldOf is the smallest world the laws of a solo fight can be read in: the
// world, its one region, and whatever the case adds. A later entity with the
// same id replaces the earlier one, which is how a case breaks the base world.
func worldOf(entities ...*entity.Entity) mapView {
	v := mapView{world: invWorld, byID: map[string]*entity.Entity{}}
	base := []*entity.Entity{
		ent(invWorld, entity.TypeWorld, map[string]any{entity.AttrLawsVersion: "v1"}),
		ent(invRegion, entity.TypeRegion, map[string]any{entity.AttrRespawnTTL: "24h"}),
	}
	for _, e := range append(base, entities...) {
		v.byID[e.ID] = e
	}
	return v
}

func ent(id, typ string, attrs map[string]any) *entity.Entity {
	return &entity.Entity{SchemaVersion: 1, ID: id, Type: typ, WorldID: invWorld, Version: 1, Attributes: attrs}
}

// with is a copy of attrs with the given keys replaced; a nil value removes
// the key.
func with(attrs map[string]any, kv ...any) map[string]any {
	out := make(map[string]any, len(attrs))
	for k, val := range attrs {
		out[k] = val
	}
	for i := 0; i+1 < len(kv); i += 2 {
		key := kv[i].(string)
		if kv[i+1] == nil {
			delete(out, key)
			continue
		}
		out[key] = kv[i+1]
	}
	return out
}

func playerAttrs() map[string]any {
	return map[string]any{
		entity.AttrHP: 10, entity.AttrHPMax: 10, entity.AttrAtk: 2, entity.AttrDef: 12,
		entity.AttrDmg: "d6", entity.AttrFlee: "2", entity.AttrStatus: entity.StatusAlive,
		entity.AttrPosition: invRegion, entity.AttrInventory: []any{},
	}
}

func wolfAttrs() map[string]any {
	return map[string]any{
		entity.AttrKind: "wolf", entity.AttrRegionID: invRegion, entity.AttrPosition: invRegion,
		entity.AttrHP: 10, entity.AttrHPMax: 10, entity.AttrAtk: 3, entity.AttrDef: 11,
		entity.AttrDmg: "d4", entity.AttrStatus: entity.StatusAlive,
	}
}

func player(id string, kv ...any) *entity.Entity {
	return ent(id, entity.TypePlayer, with(playerAttrs(), kv...))
}

func wolf(id string, kv ...any) *entity.Entity {
	return ent(id, entity.TypeNPC, with(wolfAttrs(), kv...))
}

// encounterOf is a fight in the region: participants as player id → state, in
// the order given, and the NPCs it holds.
func encounterOf(id, state string, participants []any, npcs ...string) *entity.Entity {
	list := make([]any, 0, len(npcs))
	for _, n := range npcs {
		list = append(list, map[string]any{"npc_id": n})
	}
	return ent(id, entity.TypeEncounter, map[string]any{
		entity.AttrRegionID: invRegion, entity.AttrState: state, entity.AttrRoundSeq: 1,
		entity.AttrParticipants: participants, entity.AttrNPCs: list,
	})
}

func participantIn(playerID, state string) map[string]any {
	return map[string]any{"player_id": playerID, "state": state, "damage_dealt": 0}
}

// trophyItem is an item taken from an NPC on a decision, the way ChangesFor
// appends it and the wire carries it.
func trophyItem(kind, npcID, eventID string) map[string]any {
	return map[string]any{
		"item_id": eventID + ":" + kind, "kind": kind, "name": "волчья шкура",
		"source":      map[string]any{"entity": map[string]any{"id": npcID, "type": entity.TypeNPC}, "event_id": eventID},
		"acquired_at": "2026-09-13T18:30:05Z",
	}
}

// nullRecord writes both attributes of a death as JSON null, the way a create
// that names them without a value arrives from the wire.
func nullRecord(e *entity.Entity) *entity.Entity {
	e.Attributes[entity.AttrDiedAt] = nil
	e.Attributes[entity.AttrKilledBy] = nil
	return e
}

// --- running a check ---

// checkOf is the check the rules in force carry for a law — not the function
// by name, so that a law whose Check is dropped from the register fails here
// rather than passing through a direct call.
func checkOf(t *testing.T, id string) func(StateView, []string) []Violation {
	t.Helper()
	for _, inv := range load(t).Invariants() {
		if inv.ID == id {
			if inv.Check == nil {
				t.Fatalf("%s has no check in the rules in force", id)
			}
			return inv.Check
		}
	}
	t.Fatalf("%s is not in force", id)
	return nil
}

// broken is one expected violation: on which entity, and a piece of the
// message that says why.
type broken struct {
	entity string
	says   string
}

type invCase struct {
	name    string
	world   mapView
	touched []string
	want    []broken
}

func runInvariant(t *testing.T, id string, cases []invCase) {
	t.Helper()
	check := checkOf(t, id)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := check(c.world, c.touched)
			if len(got) != len(c.want) {
				t.Fatalf("%d violations %+v, want %d %+v", len(got), got, len(c.want), c.want)
			}
			for i, w := range c.want {
				g := got[i]
				if g.InvariantID != id {
					t.Errorf("violation %d is of %s, want %s", i, g.InvariantID, id)
				}
				if g.EntityID != w.entity {
					t.Errorf("violation %d is on %q, want %q (%s)", i, g.EntityID, w.entity, g.Message)
				}
				if !strings.Contains(g.Message, w.says) {
					t.Errorf("violation %d says %q, want it to say %q", i, g.Message, w.says)
				}
			}
		})
	}
}

// --- inv-01 ---

// TestInvDeadDoesNotAct: a fight that is still on holds no terminal fighter as
// a target or as a character taking part (§5.7, C-02 v1.2).
func TestInvDeadDoesNotAct(t *testing.T) {
	alive := player("player-A")
	active := func(state string) []any { return []any{participantIn("player-A", state)} }

	cases := []invCase{
		{
			name:    "a fight of the living",
			world:   worldOf(alive, wolf("wolf-alpha"), encounterOf("enc-1", entity.EncounterStateActive, active(entity.ParticipationActive), "wolf-alpha")),
			touched: []string{"enc-1", "player-A", "wolf-alpha"},
		},
		{
			name: "the killing blow closes the fight in its own package",
			world: worldOf(alive, wolf("wolf-alpha", entity.AttrHP, 0, entity.AttrStatus, entity.StatusDead),
				encounterOf("enc-1", entity.EncounterStateResolved, active(entity.ParticipationOutOfCombat), "wolf-alpha")),
			touched: []string{"enc-1", "player-A", "wolf-alpha"},
		},
		{
			name: "a resolved fight keeps its dead as history",
			world: worldOf(player("player-A", entity.AttrStatus, entity.StatusDead), wolf("wolf-alpha", entity.AttrStatus, entity.StatusDead),
				encounterOf("enc-1", entity.EncounterStateResolved, active(entity.ParticipationActive), "wolf-alpha")),
			touched: []string{"enc-1"},
		},
		{
			name:    "a dead wolf is still a target",
			world:   worldOf(alive, wolf("wolf-alpha", entity.AttrHP, 0, entity.AttrStatus, entity.StatusDead), encounterOf("enc-1", entity.EncounterStateActive, active(entity.ParticipationActive), "wolf-alpha")),
			touched: []string{"enc-1", "wolf-alpha"},
			want:    []broken{{"wolf-alpha", "is still a target of encounter enc-1"}},
		},
		{
			name:    "a fight is opened against a corpse",
			world:   worldOf(alive, wolf("wolf-alpha", entity.AttrStatus, entity.StatusDead), encounterOf("enc-2", entity.EncounterStateActive, active(entity.ParticipationActive), "wolf-alpha")),
			touched: []string{"enc-2"},
			want:    []broken{{"wolf-alpha", "is still a target of encounter enc-2"}},
		},
		{
			name:    "a fight without a state is taken as on",
			world:   worldOf(alive, wolf("wolf-alpha", entity.AttrStatus, entity.StatusDead), encounterOf("enc-1", "", active(entity.ParticipationActive), "wolf-alpha")),
			touched: []string{"enc-1"},
			want:    []broken{{"wolf-alpha", "no longer lives"}},
		},
		{
			name:    "an empty participation reads as active",
			world:   worldOf(player("player-A", entity.AttrStatus, entity.StatusDead), wolf("wolf-alpha"), encounterOf("enc-1", entity.EncounterStateActive, active(""), "wolf-alpha")),
			touched: []string{"enc-1"},
			want:    []broken{{"player-A", `as "active"`}},
		},
		{
			name: "a character who fell and left the round",
			world: worldOf(player("player-A", entity.AttrStatus, entity.StatusDead), player("player-B"), player("player-C", entity.AttrStatus, entity.StatusAbandoned), wolf("wolf-alpha"),
				encounterOf("enc-1", entity.EncounterStateActive, []any{
					participantIn("player-A", entity.StatusDead),
					participantIn("player-B", entity.ParticipationActive),
					participantIn("player-C", entity.ParticipationOutOfCombat),
				}, "wolf-alpha")),
			touched: []string{"enc-1", "player-A"},
		},
		{
			name:    "an idle corpse does not act",
			world:   worldOf(player("player-A", entity.AttrStatus, entity.StatusDead), wolf("wolf-alpha"), encounterOf("enc-1", entity.EncounterStateActive, active(entity.ParticipationIdle), "wolf-alpha")),
			touched: []string{"enc-1"},
		},
		{
			// /forget in the middle of a fight changes the character, not the
			// encounter; the encounter agent closes the fight after the fact.
			name:    "a /forget of a fighter is not refused",
			world:   worldOf(player("player-A", entity.AttrStatus, entity.StatusAbandoned), wolf("wolf-alpha"), encounterOf("enc-1", entity.EncounterStateActive, active(entity.ParticipationActive), "wolf-alpha")),
			touched: []string{"player-A"},
		},
		{
			name:    "a fighter the world does not hold",
			world:   worldOf(wolf("wolf-alpha"), encounterOf("enc-1", entity.EncounterStateActive, []any{participantIn("player-Z", entity.ParticipationActive)}, "wolf-alpha", "wolf-ghost")),
			touched: []string{"enc-1"},
		},
		{
			name:    "a corpse named twice is one violation",
			world:   worldOf(alive, wolf("wolf-alpha", entity.AttrStatus, entity.StatusDead), encounterOf("enc-1", entity.EncounterStateActive, active(entity.ParticipationActive), "wolf-alpha", "wolf-alpha")),
			touched: []string{"enc-1"},
			want:    []broken{{"wolf-alpha", "still a target"}},
		},
		{
			name: "both sides of a broken fight, in id order",
			world: worldOf(player("player-A", entity.AttrStatus, entity.StatusDead), wolf("wolf-alpha", entity.AttrStatus, entity.StatusDead),
				encounterOf("enc-1", entity.EncounterStateActive, active(entity.ParticipationActive), "wolf-alpha")),
			touched: []string{"enc-1"},
			want:    []broken{{"player-A", "still takes part"}, {"wolf-alpha", "still a target"}},
		},
		{
			name: "npcs that do not decode",
			world: worldOf(alive, ent("enc-1", entity.TypeEncounter, map[string]any{
				entity.AttrState: entity.EncounterStateActive, entity.AttrNPCs: "wolf-alpha", entity.AttrParticipants: []any{},
			})),
			touched: []string{"enc-1"},
			want:    []broken{{"enc-1", "npcs of the encounter do not decode"}},
		},
		{
			name: "participants that do not decode",
			world: worldOf(alive, ent("enc-1", entity.TypeEncounter, map[string]any{
				entity.AttrState: entity.EncounterStateActive, entity.AttrNPCs: []any{}, entity.AttrParticipants: map[string]any{"player_id": "player-A"},
			})),
			touched: []string{"enc-1"},
			want:    []broken{{"enc-1", "participants of the encounter do not decode"}},
		},
	}
	runInvariant(t, InvDeadDoesNotAct, cases)
}

// TestInvDeadDoesNotActTreatsEveryEndAlike is the addendum of consolidation 3
// (C-02 v1.2, ADR-017 add. 1 p. 5): dead, abandoned and ascended_final break
// the law the same way and are answered with the same violation.
func TestInvDeadDoesNotActTreatsEveryEndAlike(t *testing.T) {
	check := checkOf(t, InvDeadDoesNotAct)
	var first []Violation
	for _, status := range entity.TerminalStatuses {
		t.Run(status, func(t *testing.T) {
			world := worldOf(player("player-A", entity.AttrStatus, status), wolf("wolf-alpha"),
				encounterOf("enc-1", entity.EncounterStateActive, []any{participantIn("player-A", entity.ParticipationActive)}, "wolf-alpha"))
			got := check(world, []string{"enc-1", "player-A"})
			if len(got) != 1 || got[0].InvariantID != InvDeadDoesNotAct || got[0].EntityID != "player-A" {
				t.Fatalf("violations %+v, want one inv-01 on player-A", got)
			}
			if first == nil {
				first = got
				return
			}
			if !reflect.DeepEqual(got, first) {
				t.Errorf("%s answers %+v, %s answered %+v: one law, one violation", status, got, entity.TerminalStatuses[0], first)
			}
		})
	}
	if len(first) == 0 {
		t.Fatal("no terminal status broke the law")
	}
}

// --- inv-02 ---

// TestInvHPInRange: 0 <= hp <= hp_max on every touched entity that has hit
// points (§5.7, data-model.md §3.3).
func TestInvHPInRange(t *testing.T) {
	cases := []invCase{
		{name: "full health", world: worldOf(player("player-A")), touched: []string{"player-A"}},
		{name: "at zero", world: worldOf(player("player-A", entity.AttrHP, 0)), touched: []string{"player-A"}},
		{name: "in between", world: worldOf(wolf("wolf-alpha", entity.AttrHP, 4)), touched: []string{"wolf-alpha"}},
		{name: "a signed modifier is a number", world: worldOf(player("player-A", entity.AttrHP, "+7")), touched: []string{"player-A"}},
		{
			name: "below zero", world: worldOf(player("player-A", entity.AttrHP, -1)), touched: []string{"player-A"},
			want: []broken{{"player-A", "is -1, outside [0, 10]"}},
		},
		{
			name: "a rest above the maximum", world: worldOf(player("player-A", entity.AttrHP, 11)), touched: []string{"player-A"},
			want: []broken{{"player-A", "is 11, outside [0, 10]"}},
		},
		{
			name: "a maximum lowered under the hit points", world: worldOf(wolf("wolf-alpha", entity.AttrHPMax, 6)), touched: []string{"wolf-alpha"},
			want: []broken{{"wolf-alpha", "is 10, outside [0, 6]"}},
		},
		{
			name: "hit points that are not a number", world: worldOf(player("player-A", entity.AttrHP, "ten")), touched: []string{"player-A"},
			want: []broken{{"player-A", "is not a whole number"}},
		},
		{
			name: "a fraction of a hit point", world: worldOf(player("player-A", entity.AttrHP, 2.5)), touched: []string{"player-A"},
			want: []broken{{"player-A", "is not a whole number"}},
		},
		{
			name: "hit points without a maximum", world: worldOf(player("player-A", entity.AttrHPMax, nil)), touched: []string{"player-A"},
			want: []broken{{"player-A", "no whole hp_max"}},
		},
		{name: "an untouched entity is not answered for", world: worldOf(player("player-A", entity.AttrHP, 99), player("player-B")), touched: []string{"player-B"}},
		{name: "an entity without hit points", world: worldOf(), touched: []string{invRegion, invWorld}},
		{
			name: "a character whose hit points were removed", world: worldOf(player("player-A", entity.AttrHP, nil)), touched: []string{"player-A"},
			want: []broken{{"player-A", "player player-A has no hp to hold in range"}},
		},
		{
			name: "an NPC without hit points", world: worldOf(wolf("wolf-alpha", entity.AttrHP, nil)), touched: []string{"wolf-alpha"},
			want: []broken{{"wolf-alpha", "npc wolf-alpha has no hp to hold in range"}},
		},
		{
			name:    "each touched entity once, in id order",
			world:   worldOf(player("player-B", entity.AttrHP, 12), player("player-A", entity.AttrHP, -3)),
			touched: []string{"player-B", "player-A", "player-B"},
			want:    []broken{{"player-A", "is -3"}, {"player-B", "is 12"}},
		},
	}
	runInvariant(t, InvHPInRange, cases)
}

// --- inv-03 ---

// TestInvOneTrophyPerNPC: the trophies of one NPC are one handout — one pair of
// hands, one decision, the character loot_claimed_by names (§5.7, data-model.md
// §3.4–3.5).
func TestInvOneTrophyPerNPC(t *testing.T) {
	pelt := trophyItem("wolf-pelt", "wolf-alpha", "ev-1")
	fang := trophyItem("wolf-fang", "wolf-alpha", "ev-1")
	dead := func(kv ...any) *entity.Entity {
		return wolf("wolf-alpha", append([]any{entity.AttrHP, 0, entity.AttrStatus, entity.StatusDead}, kv...)...)
	}

	cases := []invCase{
		{
			name:    "one trophy to the one who killed",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{pelt}), dead(entity.AttrLootClaimedBy, "player-A")),
			touched: []string{"player-A", "wolf-alpha"},
		},
		{
			name:    "a loot table of two items is one handout",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{pelt, fang}), dead(entity.AttrLootClaimedBy, "player-A")),
			touched: []string{"player-A", "wolf-alpha"},
		},
		{
			name: "items that came from no NPC are not trophies, whoever holds them",
			world: worldOf(
				player("player-A", entity.AttrInventory, []any{map[string]any{"item_id": "stick-1", "kind": "stick", "name": "палка"}}),
				player("player-B", entity.AttrInventory, []any{map[string]any{"item_id": "stick-2", "kind": "stick", "name": "палка"}}),
			),
			touched: []string{"player-A", "player-B"},
		},
		{
			name:    "no claim recorded",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{pelt}), dead()),
			touched: []string{"player-A", "wolf-alpha"},
		},
		{
			name:    "a trophy of an NPC the world no longer holds",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{trophyItem("wolf-pelt", "wolf-gone", "ev-0")})),
			touched: []string{"player-A"},
		},
		{
			name:    "the same trophy in two inventories, asked for the second hand",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{pelt}), player("player-B", entity.AttrInventory, []any{pelt}), dead(entity.AttrLootClaimedBy, "player-A")),
			touched: []string{"player-B"},
			want:    []broken{{"player-B", "held by player-A, player-B"}},
		},
		{
			name:    "the same trophy in two inventories, asked for the NPC",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{pelt}), player("player-B", entity.AttrInventory, []any{pelt}), dead()),
			touched: []string{"wolf-alpha"},
			want:    []broken{{"wolf-alpha", "held by player-A, player-B"}},
		},
		{
			name:    "two handouts are not answered for when nobody involved is touched",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{pelt}), player("player-B", entity.AttrInventory, []any{pelt}), player("player-C"), dead()),
			touched: []string{"player-C"},
		},
		{
			name:    "one NPC, two decisions",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{trophyItem("wolf-pelt", "wolf-alpha", "ev-2"), pelt}), dead(entity.AttrLootClaimedBy, "player-A")),
			touched: []string{"player-A"},
			want:    []broken{{"player-A", "come from 2 decisions (ev-1, ev-2)"}},
		},
		{
			name:    "a trophy in the hands of somebody the NPC did not name",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{pelt}), player("player-B"), dead(entity.AttrLootClaimedBy, "player-B")),
			touched: []string{"player-A", "wolf-alpha"},
			want: []broken{
				{"player-A", "player-A holds a trophy of wolf-alpha, which records player-B"},
				{"wolf-alpha", "player-A holds a trophy of wolf-alpha, which records player-B"},
			},
		},
		{
			name:    "an inventory that does not decode",
			world:   worldOf(player("player-A", entity.AttrInventory, "pelt")),
			touched: []string{"player-A"},
			want:    []broken{{"player-A", "does not decode"}},
		},
		{
			name:    "a broken inventory of somebody else is not this change's",
			world:   worldOf(player("player-A", entity.AttrInventory, []any{pelt}), player("player-B", entity.AttrInventory, 7), dead(entity.AttrLootClaimedBy, "player-A")),
			touched: []string{"player-A"},
		},
	}
	runInvariant(t, InvOneTrophyPerNPC, cases)
}

// --- inv-09 ---

// TestInvRespawnTTL: an entity that carries a death record is dead; a respawn
// is a new entity, never the old one revived (§5.7).
func TestInvRespawnTTL(t *testing.T) {
	record := []any{entity.AttrDiedAt, "2026-09-13T18:30:05Z", entity.AttrKilledBy, "player-A"}
	withRecord := func(kv ...any) []any { return append(slices.Clone(record), kv...) }

	cases := []invCase{
		{name: "a wolf that died stays dead", world: worldOf(wolf("wolf-alpha", withRecord(entity.AttrHP, 0, entity.AttrStatus, entity.StatusDead)...)), touched: []string{"wolf-alpha"}},
		{name: "a living wolf with no record", world: worldOf(wolf("wolf-alpha")), touched: []string{"wolf-alpha"}},
		{name: "a record written as nothing", world: worldOf(wolf("wolf-alpha", entity.AttrDiedAt, "")), touched: []string{"wolf-alpha"}},
		{name: "a record written as null", world: worldOf(nullRecord(wolf("wolf-alpha"))), touched: []string{"wolf-alpha"}},
		{
			name:    "a respawned wolf is a new entity",
			world:   worldOf(wolf("wolf-alpha", withRecord(entity.AttrStatus, entity.StatusDead)...), wolf("wolf-beta")),
			touched: []string{"wolf-beta"},
		},
		{
			name: "the wolf got up again", world: worldOf(wolf("wolf-alpha", withRecord(entity.AttrHP, 10)...)), touched: []string{"wolf-alpha"},
			want: []broken{{"wolf-alpha", `carries died_at and killed_by and its status is "alive"`}},
		},
		{
			name: "died_at alone", world: worldOf(wolf("wolf-alpha", entity.AttrDiedAt, "2026-09-13T18:30:05Z")), touched: []string{"wolf-alpha"},
			want: []broken{{"wolf-alpha", "carries died_at and its status"}},
		},
		{
			name: "killed_by alone", world: worldOf(wolf("wolf-alpha", entity.AttrKilledBy, "player-A")), touched: []string{"wolf-alpha"},
			want: []broken{{"wolf-alpha", "carries killed_by and its status"}},
		},
		{
			name: "a death record without a status", world: worldOf(wolf("wolf-alpha", withRecord(entity.AttrStatus, nil)...)), touched: []string{"wolf-alpha"},
			want: []broken{{"wolf-alpha", `its status is ""`}},
		},
		{name: "an untouched revenant is the audit's", world: worldOf(wolf("wolf-alpha", withRecord()...), wolf("wolf-beta")), touched: []string{"wolf-beta"}},
	}
	// Every terminal status keeps its death record without a break, the way
	// inv-01 answers the three alike: a law that forgot ascended_final would
	// refuse a record on a character who left the world for good.
	for _, status := range entity.TerminalStatuses {
		cases = append(cases, invCase{
			name:    "a death record on a character who is " + status,
			world:   worldOf(player("player-A", withRecord(entity.AttrStatus, status)...)),
			touched: []string{"player-A"},
		})
	}
	runInvariant(t, InvRespawnTTL, cases)
}

// --- inv-10 ---

// TestInvOnePosition: everything that stands somewhere stands in one place of
// this world — a region of it, or outside of it (§5.7, data-model.md §3.3).
func TestInvOnePosition(t *testing.T) {
	group := func(kv ...any) *entity.Entity {
		return ent("group-1", entity.TypeGroup, with(map[string]any{entity.AttrState: entity.GroupStateActive}, kv...))
	}
	cases := []invCase{
		{name: "a character in a region", world: worldOf(player("player-A")), touched: []string{"player-A"}},
		{name: "a character outside the world", world: worldOf(player("player-A", entity.AttrPosition, "outside:"+invWorld)), touched: []string{"player-A"}},
		{name: "a wolf in its region", world: worldOf(wolf("wolf-alpha")), touched: []string{"wolf-alpha"}},
		{name: "a group in a region", world: worldOf(group(entity.AttrPosition, invRegion)), touched: []string{"group-1"}},
		{name: "a group without a position is inv-04's", world: worldOf(group()), touched: []string{"group-1"}},
		{name: "a region stands nowhere", world: worldOf(), touched: []string{invRegion, invWorld}},
		{
			name: "a region the world does not hold", world: worldOf(player("player-A", entity.AttrPosition, "nowhere-at-all")), touched: []string{"player-A"},
			want: []broken{{"player-A", "no such region in the world"}},
		},
		{
			name: "standing on another character", world: worldOf(player("player-A", entity.AttrPosition, "player-B"), player("player-B")), touched: []string{"player-A"},
			want: []broken{{"player-A", "player-B is a player, not a region"}},
		},
		{
			name: "outside another world", world: worldOf(player("player-A", entity.AttrPosition, "outside:other-world")), touched: []string{"player-A"},
			want: []broken{{"player-A", "want outside:" + invWorld}},
		},
		{
			name: "two positions at once", world: worldOf(player("player-A", entity.AttrPosition, []any{invRegion, "outside:" + invWorld})), touched: []string{"player-A"},
			want: []broken{{"player-A", "not one position"}},
		},
		{
			name: "a character with no position", world: worldOf(player("player-A", entity.AttrPosition, nil)), touched: []string{"player-A"},
			want: []broken{{"player-A", "stands nowhere"}},
		},
		{
			name: "a wolf with no position", world: worldOf(wolf("wolf-alpha", entity.AttrPosition, nil)), touched: []string{"wolf-alpha"},
			want: []broken{{"wolf-alpha", "stands nowhere"}},
		},
		{
			name: "a group somewhere unknown", world: worldOf(group(entity.AttrPosition, "dark-forest-99")), touched: []string{"group-1"},
			want: []broken{{"group-1", "no such region"}},
		},
		{name: "an untouched wanderer is the audit's", world: worldOf(player("player-A", entity.AttrPosition, "nowhere"), player("player-B")), touched: []string{"player-B"}},
		{name: "a touched id the world does not hold", world: worldOf(player("player-A")), touched: []string{"player-Z"}},
	}
	runInvariant(t, InvOnePosition, cases)
}

// --- all of them ---

// TestInvariantsAreDeterministic: one world, one answer, whatever order the
// touched ids come in, however often they repeat and whatever order the view
// lists the world in — a replay has to refuse what the first run refused, in
// the same words (ADR-003 p. 5).
func TestInvariantsAreDeterministic(t *testing.T) {
	pelt := trophyItem("wolf-pelt", "wolf-alpha", "ev-1")
	world := worldOf(
		player("player-A", entity.AttrHP, 12, entity.AttrPosition, "nowhere", entity.AttrStatus, entity.StatusDead, entity.AttrInventory, []any{pelt}),
		player("player-B", entity.AttrHP, -1, entity.AttrPosition, "outside:elsewhere", entity.AttrInventory, []any{pelt}),
		player("player-C", entity.AttrInventory, []any{trophyItem("wolf-pelt", "wolf-alpha", "ev-9")}),
		wolf("wolf-alpha", entity.AttrDiedAt, "2026-09-13T18:30:05Z", entity.AttrLootClaimedBy, "player-C"),
		wolf("wolf-beta", entity.AttrStatus, entity.StatusDead),
		encounterOf("enc-1", entity.EncounterStateActive, []any{
			participantIn("player-A", entity.ParticipationActive),
			participantIn("player-B", entity.ParticipationActive),
		}, "wolf-beta", "wolf-alpha"),
	)
	orders := [][]string{
		{"enc-1", "player-A", "player-B", "player-C", "wolf-alpha", "wolf-beta"},
		{"wolf-beta", "wolf-alpha", "player-C", "player-B", "player-A", "enc-1"},
		{"player-B", "enc-1", "wolf-alpha", "player-A", "wolf-beta", "player-C", "player-B", "enc-1"},
	}

	for _, inv := range load(t).Invariants() {
		if inv.Check == nil {
			continue
		}
		t.Run(inv.ID, func(t *testing.T) {
			want := inv.Check(world, orders[0])
			if len(want) == 0 {
				t.Fatalf("%s finds nothing wrong in a world built to break it: the test proves nothing", inv.ID)
			}
			if !slices.IsSortedFunc(want, func(a, b Violation) int {
				return strings.Compare(a.EntityID+"\x00"+a.Message, b.EntityID+"\x00"+b.Message)
			}) {
				t.Errorf("violations are not in entity order: %+v", want)
			}
			for run := range 50 {
				got := inv.Check(world, orders[run%len(orders)])
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("run %d answered %+v, the first run %+v", run, got, want)
				}
			}
		})
	}
}

// TestInvariantsHoldOnTheShippedWorld: the world testdata/fixtures bootstraps
// breaks none of the laws, with every one of its entities touched. A law that
// refuses the fixtures would refuse the first world of every e2e run.
func TestInvariantsHoldOnTheShippedWorld(t *testing.T) {
	files, err := filepath.Glob("../../testdata/fixtures/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("fixtures: %v (%d files)", err, len(files))
	}
	world := mapView{world: invWorld, byID: map[string]*entity.Entity{}}
	for _, name := range files {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var entities []*entity.Entity
		if err := json.Unmarshal(body, &entities); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		for _, e := range entities {
			world.byID[e.ID] = e
		}
	}
	touched := make([]string, 0, len(world.byID))
	for id := range world.byID {
		touched = append(touched, id)
	}
	if len(touched) < 6 {
		t.Fatalf("%d entities in the fixtures, want the six of the dark forest", len(touched))
	}

	// A character who walks into the forest breaks nothing either.
	walked := entity.Clone(world.byID["player-A"])
	walked.Attributes[entity.AttrPosition] = invRegion
	world.byID[walked.ID] = walked

	for _, inv := range load(t).Invariants() {
		if inv.Check == nil {
			continue
		}
		if got := inv.Check(world, touched); len(got) != 0 {
			t.Errorf("%s refuses the shipped world: %+v", inv.ID, got)
		}
	}
}

// TestInvariantsAcceptWhatChangesForProposes: the package the mechanics build
// for a killing blow — hit points, the death record, the trophy — applied to a
// world, breaks none of the laws. A law that refused it would refuse every
// won fight (C-03, C-05 v1.3 p. 2).
func TestInvariantsAcceptWhatChangesForProposes(t *testing.T) {
	r := load(t)
	world := worldOf(player("player-A"), wolf("wolf-alpha", entity.AttrHP, 2),
		encounterOf("enc-1", entity.EncounterStateActive, []any{participantIn("player-A", entity.ParticipationActive)}, "wolf-alpha"))

	attacker, err := ActorFromEntity(world.byID["player-A"], world.byID["enc-1"])
	if err != nil {
		t.Fatalf("attacker: %v", err)
	}
	target, err := ActorFromEntity(world.byID["wolf-alpha"], world.byID["enc-1"])
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	at := time.Date(2026, 9, 13, 18, 30, 5, 0, time.UTC)
	action := Action{Kind: ActionAttack, Actor: attacker.ID, Target: target.ID, At: at}
	outcome := Outcome{Hit: true, Damage: 4, HPBefore: 2, HPAfter: 0, TargetDead: true, Loot: r.Loot("wolf")}
	proposed, err := ChangesFor(action, outcome, attacker, target, "ev-kill")
	if err != nil {
		t.Fatalf("changes for a killing blow: %v", err)
	}

	var touched []string
	for _, change := range proposed {
		e := entity.Clone(world.byID[change.Entity.ID])
		attrs, _, err := entity.ApplyOps(e, change.Ops)
		if err != nil {
			t.Fatalf("apply to %s: %v", change.Entity.ID, err)
		}
		e.Attributes = attrs
		world.byID[e.ID] = e
		touched = append(touched, e.ID)
	}
	// The encounter agent closes the fight in the same package (C-05 v1.4 p. 5).
	closed := entity.Clone(world.byID["enc-1"])
	closed.Attributes[entity.AttrState] = entity.EncounterStateResolved
	closed.Attributes[entity.AttrParticipants] = []any{participantIn("player-A", entity.ParticipationOutOfCombat)}
	world.byID[closed.ID] = closed
	touched = append(touched, closed.ID)

	if status, _ := world.byID["wolf-alpha"].Status(); status != entity.StatusDead {
		t.Fatalf("the package did not kill the wolf: status %q", status)
	}
	for _, inv := range r.Invariants() {
		if inv.Check == nil {
			continue
		}
		if got := inv.Check(world, touched); len(got) != 0 {
			t.Errorf("%s refuses the package of a won fight: %+v", inv.ID, got)
		}
	}

	// The same fight left open over the corpse is what inv-01 is for.
	world.byID["enc-1"] = encounterOf("enc-1", entity.EncounterStateActive, []any{participantIn("player-A", entity.ParticipationActive)}, "wolf-alpha")
	if got := checkOf(t, InvDeadDoesNotAct)(world, touched); len(got) != 1 || got[0].EntityID != "wolf-alpha" {
		t.Errorf("a fight left open over the wolf it killed: %+v, want one inv-01 on wolf-alpha", got)
	}
}

// TestInvariantIDsMatchTheLaws pins the register to the invariant laws of
// laws@v1 (ADR-012 p. 5). The laws file is EPIC-003's and does not exist yet;
// until mvctl laws check compares the two, the list in testdata is the laws
// side (tasks.md T-054).
func TestInvariantIDsMatchTheLaws(t *testing.T) {
	body, err := os.ReadFile("testdata/laws-v1.invariants.yaml")
	if err != nil {
		t.Fatalf("read the list of laws: %v", err)
	}
	var doc struct {
		Laws []struct {
			ID   string `yaml:"id"`
			Kind string `yaml:"kind"`
		} `yaml:"laws"`
	}
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode the list of laws: %v", err)
	}
	var laws []string
	for _, law := range doc.Laws {
		if law.Kind != "invariant" {
			t.Errorf("law %s is of kind %q: the list holds invariants only", law.ID, law.Kind)
		}
		laws = append(laws, law.ID)
	}
	slices.Sort(laws)

	ids := InvariantIDs()
	slices.Sort(ids)
	if !slices.Equal(ids, laws) {
		t.Errorf("register %v, laws@v1 %v: a law without a check or a check without a law", ids, laws)
	}
}
