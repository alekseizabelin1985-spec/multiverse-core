package mechanics

import (
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/entity"
)

func fixedTime() time.Time { return time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC) }

func playerEntity(attrs map[string]any) *entity.Entity {
	base := map[string]any{
		entity.AttrHP:     7,
		entity.AttrHPMax:  10,
		entity.AttrAtk:    2,
		entity.AttrDef:    12,
		entity.AttrDmg:    "d6",
		entity.AttrFlee:   "2",
		entity.AttrStatus: entity.StatusAlive,
	}
	for k, v := range attrs {
		if v == nil {
			delete(base, k)
			continue
		}
		base[k] = v
	}
	return entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, "dark-forest-world", "Вася", base, fixedTime())
}

func wolfEntity() *entity.Entity {
	return entity.New(entity.Ref{ID: "wolf-alpha", Type: entity.TypeNPC}, "dark-forest-world", "Вожак",
		map[string]any{
			entity.AttrHP:     10,
			entity.AttrHPMax:  10,
			entity.AttrAtk:    3,
			entity.AttrDef:    11,
			entity.AttrDmg:    "d4",
			entity.AttrStatus: entity.StatusAlive,
			entity.AttrKind:   "wolf",
		}, fixedTime())
}

func encounterEntity(participation, lastDamager string) *entity.Entity {
	return entity.New(entity.Ref{ID: "encounter-1", Type: entity.TypeEncounter}, "dark-forest-world", "Схватка",
		map[string]any{
			entity.AttrParticipants: []any{
				map[string]any{"player_id": "player-A", "state": participation, "damage_dealt": 3},
			},
			entity.AttrNPCs: []any{
				map[string]any{"npc_id": "wolf-alpha", "last_damager": lastDamager},
			},
		}, fixedTime())
}

// TestActorFromEntity reads a fighter out of the world, without an encounter
// and with one. What the encounter adds is exactly the two facts that belong to
// it: how a character is taking part, and who hit the wolf last.
func TestActorFromEntity(t *testing.T) {
	a, err := ActorFromEntity(playerEntity(nil), nil)
	if err != nil {
		t.Fatalf("player: %v", err)
	}
	want := Actor{
		ID: "player-A", Type: entity.TypePlayer, Version: 1,
		HP: 7, HPMax: 10, Atk: 2, Def: 12,
		Dmg: "d6", Flee: "2", Status: entity.StatusAlive,
	}
	if *a != want {
		t.Errorf("actor %+v, want %+v", *a, want)
	}

	a, err = ActorFromEntity(playerEntity(nil), encounterEntity(entity.ParticipationIdle, "player-A"))
	if err != nil {
		t.Fatalf("player in an encounter: %v", err)
	}
	if a.Participation != entity.ParticipationIdle {
		t.Errorf("participation %q, want idle", a.Participation)
	}
	if a.LastDamager != "" {
		t.Errorf("a character carries a last damager: %q", a.LastDamager)
	}

	npc, err := ActorFromEntity(wolfEntity(), encounterEntity(entity.ParticipationActive, "player-A"))
	if err != nil {
		t.Fatalf("wolf: %v", err)
	}
	if npc.LastDamager != "player-A" {
		t.Errorf("last damager %q, want player-A", npc.LastDamager)
	}
	if npc.Kind != "wolf" {
		t.Errorf("kind %q, want wolf: the loot table is looked up by it", npc.Kind)
	}
	if npc.Version != 1 {
		t.Errorf("version %d, the entity is at 1: it is what ChangesFor pins", npc.Version)
	}
	moved := wolfEntity()
	moved.Version = 12
	if a, err := ActorFromEntity(moved, nil); err != nil || a.Version != 12 {
		t.Errorf("a wolf at version 12 reads back as (%+v, %v): the version is the entity's", a, err)
	}
	if npc.Participation != "" {
		t.Errorf("an NPC carries a participation: %q", npc.Participation)
	}
	if npc.Flee != "" {
		t.Errorf("the wolf has a flee bonus %q: it does not run", npc.Flee)
	}

	// An actor of an encounter it is not in keeps the world's answer: the
	// encounter adds facts, it does not invent them.
	other := encounterEntity(entity.ParticipationIdle, "player-B")
	other.Attributes[entity.AttrParticipants] = []any{
		map[string]any{"player_id": "player-Z", "state": entity.ParticipationIdle},
	}
	a, err = ActorFromEntity(playerEntity(nil), other)
	if err != nil {
		t.Fatalf("player outside the encounter: %v", err)
	}
	if a.Participation != "" {
		t.Errorf("participation %q for a character not in the encounter", a.Participation)
	}
}

// TestActorFromEntityStatsMatchRules is the tie between the world and the rule
// book: an entity created from Rules.Stats reads back as the same actor. It is
// what T-016 checks the fixtures against.
func TestActorFromEntityStatsMatchRules(t *testing.T) {
	r := load(t)
	stats, ok := r.Stats("wolf")
	if !ok {
		t.Fatal("no stats for a wolf")
	}
	a, err := ActorFromEntity(wolfEntity(), nil)
	if err != nil {
		t.Fatalf("wolf: %v", err)
	}
	if a.HPMax != stats.HPMax || a.Atk != stats.Atk || a.Def != stats.Def || a.Dmg != stats.Dmg {
		t.Errorf("the wolf of the world is %+v, the rules say %+v", *a, stats)
	}
}

// TestActorFromEntityRejects: every number a formula reads is required. An
// actor missing one is a defect of whoever created it, and defaulting the
// number would hide that defect inside a fight.
func TestActorFromEntityRejects(t *testing.T) {
	cases := map[string]struct {
		e    *entity.Entity
		want string
	}{
		"no hp":     {playerEntity(map[string]any{entity.AttrHP: nil}), entity.AttrHP},
		"no hp_max": {playerEntity(map[string]any{entity.AttrHPMax: nil}), entity.AttrHPMax},
		"no atk":    {playerEntity(map[string]any{entity.AttrAtk: nil}), entity.AttrAtk},
		"no def":    {playerEntity(map[string]any{entity.AttrDef: nil}), entity.AttrDef},
		"no dmg":    {playerEntity(map[string]any{entity.AttrDmg: nil}), entity.AttrDmg},
		"no status": {playerEntity(map[string]any{entity.AttrStatus: nil}), entity.AttrStatus},
		"hp is not a number": {
			playerEntity(map[string]any{entity.AttrHP: "many"}), entity.AttrHP,
		},
		// Damage is dice and only dice (T-053, the backlog item of T-050): a
		// flat number is named as what it is, not reported as missing.
		"dmg is a flat number": {
			playerEntity(map[string]any{entity.AttrDmg: 3}), "not a dice expression",
		},
		"dmg is not dice": {
			playerEntity(map[string]any{entity.AttrDmg: "a lot"}), entity.AttrDmg,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ActorFromEntity(c.e, nil); err == nil {
				t.Fatal("read an actor")
			} else if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %v does not name %s", err, c.want)
			}
		})
	}

	// Every NPC has a kind (data-model.md §3.4, C-03 v1.3): without one it
	// would fall without the trophy of its kind and nothing would notice.
	for name, kind := range map[string]any{"no kind": nil, "an empty kind": ""} {
		wolf := wolfEntity()
		if kind == nil {
			delete(wolf.Attributes, entity.AttrKind)
		} else {
			wolf.Attributes[entity.AttrKind] = kind
		}
		if _, err := ActorFromEntity(wolf, nil); err == nil || !strings.Contains(err.Error(), entity.AttrKind) {
			t.Errorf("an npc with %s answered %v, want a refusal naming %s", name, err, entity.AttrKind)
		}
	}

	if _, err := ActorFromEntity(nil, nil); err == nil {
		t.Error("read an actor out of nothing")
	}

	region := entity.New(entity.Ref{ID: "dark-forest-01", Type: entity.TypeRegion}, "dark-forest-world", "Лес", nil, fixedTime())
	if _, err := ActorFromEntity(region, nil); err == nil {
		t.Error("a region fights")
	}

	if _, err := ActorFromEntity(playerEntity(nil), region); err == nil {
		t.Error("a region served as an encounter")
	}

	broken := encounterEntity(entity.ParticipationActive, "")
	broken.Attributes[entity.AttrParticipants] = "нет"
	if _, err := ActorFromEntity(playerEntity(nil), broken); err == nil {
		t.Error("an encounter with unreadable participants was accepted")
	}
	broken = encounterEntity(entity.ParticipationActive, "")
	broken.Attributes[entity.AttrNPCs] = 7
	if _, err := ActorFromEntity(wolfEntity(), broken); err == nil {
		t.Error("an encounter with unreadable npcs was accepted")
	}
}

// TestActorFromEntityTerminalStatus: a dead or abandoned character reads back as
// what it is, and the rules exclude it either way (inv-01, C-02 v1.2).
func TestActorFromEntityTerminalStatus(t *testing.T) {
	r := load(t)
	for _, status := range []string{entity.StatusDead, entity.StatusAbandoned, entity.StatusAscendedFinal} {
		a, err := ActorFromEntity(playerEntity(map[string]any{entity.AttrStatus: status}), nil)
		if err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		if a.Alive() {
			t.Errorf("%s reads as alive", status)
		}
		if !r.Excluded(*a) {
			t.Errorf("%s is not excluded from a round", status)
		}
	}
}
