package state_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// The checks of T-056 over the Applier as State runs it: the ownership table
// and the norms over it in force, and the laws of rules/dark-forest.yaml.

const (
	forestRegion = "dark-forest-01"
	outside      = "outside:" + world
)

var (
	lawsOnce sync.Once
	lawsList []mechanics.Invariant
	lawsErr  error
)

// laws are the invariants the rule book of the world switches on.
func laws(t *testing.T) []mechanics.Invariant {
	t.Helper()
	lawsOnce.Do(func() {
		rules, err := mechanics.Load(filepath.Join("..", "..", "rules", "dark-forest.yaml"))
		if err != nil {
			lawsErr = err
			return
		}
		lawsList = rules.Invariants()
	})
	if lawsErr != nil {
		t.Fatalf("rules: %v", lawsErr)
	}
	return lawsList
}

// forest is the Applier of State over a small world that keeps every law: the
// world and its region, a character outside the forest, a wolf in it, an
// encounter between them that is on, and a group.
func forest(t *testing.T) *fixture {
	t.Helper()
	f := newOwnedFixture(t, laws(t)...)
	f.seed(t, world, entity.TypeWorld, 1, map[string]any{entity.AttrLawsVersion: "v1"})
	f.seed(t, forestRegion, entity.TypeRegion, 1, map[string]any{entity.AttrDescription: "a dark forest"})
	f.seed(t, "player-A", entity.TypePlayer, 1, character(entity.StatusAlive, outside))
	f.seed(t, "wolf-alpha", entity.TypeNPC, 1, map[string]any{
		entity.AttrHP: 8, entity.AttrHPMax: 8, entity.AttrStatus: entity.StatusAlive, entity.AttrPosition: forestRegion,
	})
	f.seed(t, "enc-1", entity.TypeEncounter, 1, encounter(entity.EncounterStateActive, "player-A", "wolf-alpha"))
	f.seed(t, "g-1", entity.TypeGroup, 1, map[string]any{entity.AttrState: "forming", entity.AttrMembers: []any{}})
	return f
}

func character(status, position string) map[string]any {
	return map[string]any{
		entity.AttrHP: 10, entity.AttrHPMax: 10, entity.AttrStatus: status,
		entity.AttrPosition: position, entity.AttrInventory: []any{},
	}
}

func encounter(state, playerID, npcID string) map[string]any {
	participants := []any{}
	if playerID != "" {
		participants = append(participants, map[string]any{"player_id": playerID, "state": entity.ParticipationActive})
	}
	return map[string]any{
		entity.AttrState: state, entity.AttrRoundSeq: 1,
		entity.AttrParticipants: participants,
		entity.AttrNPCs:         []any{map[string]any{"npc_id": npcID}},
	}
}

// answer is the one event the Applier published for a proposal.
func (f *fixture) answer(t *testing.T) eventbus.Event {
	t.Helper()
	all := f.journal.all()
	if len(all) != 1 {
		t.Fatalf("published %v, want one answer", types(all))
	}
	return all[0]
}

var (
	playerA = ref("player-A", entity.TypePlayer)
	wolf    = ref("wolf-alpha", entity.TypeNPC)
	enc1    = ref("enc-1", entity.TypeEncounter)
)

// matrixCase is one row of the matrix "proposer × type × path × cause ×
// reason" (DoD of T-056). want "" is a proposal that is applied.
type matrixCase struct {
	name     string
	arrange  func(t *testing.T, f *fixture)
	proposal func(t *testing.T) eventbus.Event
	want     state.Reason
	entity   string
	law      string
}

func setOp(path string, value any) entity.Op              { return op(entity.OpSet, path, value) }
func incOp(path string, value any) entity.Op              { return op(entity.OpInc, path, value) }
func v(n int64) *int64                                    { return version(n) }
func one(r entity.Ref, ops ...entity.Op) entity.ChangeSet { return set(r, v(1), ops...) }

// TestTheMatrixOfRefusals walks every reason of C-02 at least once, and the
// rows of the ownership table and its norms that decide level_violation.
func TestTheMatrixOfRefusals(t *testing.T) {
	cases := []matrixCase{
		// --- unknown_entity, version_conflict, duplicate_entity ---
		{name: "author: an entity the world does not hold", want: state.ReasonUnknownEntity, entity: "ghost",
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, set(ref("ghost", entity.TypeNPC), nil, incOp("hp", -1)))
			}},
		{name: "task: a stale version of the character", want: state.ReasonVersionConflict, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, set(playerA, v(9), incOp("hp", -1)))
			}},
		{name: "author: a create over an identifier taken", want: state.ReasonDuplicateEntity, entity: "wolf-alpha",
			proposal: func(*testing.T) eventbus.Event {
				return create("p", wolf, "", map[string]any{entity.AttrHP: 1})
			}},

		// --- invalid_op ---
		{name: "task: an operation outside the four", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, one(playerA, op("teleport", "hp", 1)))
			}},
		{name: "gateway: a number past 2^53-1 in the value, before its rights are asked", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "move", true, one(playerA, setOp("seed", float64(1<<53))))
			}},
		{name: "task: a character proposed as an NPC", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, one(ref("player-A", entity.TypeNPC), incOp("hp", -1)))
			}},
		{name: "task: a status the matrix does not know", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, one(playerA, setOp("status", "sleeping")))
			}},
		{name: "author: an inc on a text", want: state.ReasonInvalidOp, entity: "g-1",
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, one(ref("g-1", entity.TypeGroup), incOp("state", 1)))
			}},

		// --- level_violation: the table ---
		{name: "task: hp of a character with cause=resolve", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "resolve", true, one(playerA, incOp("hp", -1)))
			}},
		{name: "task: hp of an NPC with cause=resolve", want: state.ReasonLevelViolation, entity: "wolf-alpha",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "resolve", true, one(wolf, incOp("hp", -1)))
			}},
		{name: "task: the encounter with cause=resolve", entity: "enc-1",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "resolve", true,
					one(enc1, setOp("state", entity.EncounterStateResolved), setOp("resolution", "players_out")))
			}},
		{name: "task: died_at of a character", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, one(playerA, setOp("died_at", "2026-09-13T00:00:00Z")))
			}},
		{name: "task: hp_max of a character, which the prefix hp does not cover", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, one(playerA, setOp("hp_max", 20)))
			}},
		{name: "task: an item of the inventory, which the prefix inventory covers", entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "loot", true, one(playerA,
					op(entity.OpAppend, "inventory", map[string]any{"item_id": "pelt-1", "kind": "wolf-pelt"})))
			}},
		{name: "gateway: an NPC", want: state.ReasonLevelViolation, entity: "wolf-alpha",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "move", true, one(wolf, setOp("position", forestRegion)))
			}},
		{name: "gateway: encounter_id of a group", want: state.ReasonLevelViolation, entity: "g-1",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "group", true, one(ref("g-1", entity.TypeGroup), setOp("encounter_id", "enc-1")))
			}},
		{name: "domain: the description of a region", want: state.ReasonLevelViolation, entity: forestRegion,
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("domain"), "p", "tick", true, one(ref(forestRegion, entity.TypeRegion), setOp("description", "bright")))
			}},
		{name: "global: laws_version of the world", want: state.ReasonLevelViolation, entity: world,
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("global"), "p", "tick", true, one(ref(world, entity.TypeWorld), setOp("laws_version", "v2")))
			}},
		{name: "object: reserved by C-13, refuses everything", want: state.ReasonLevelViolation, entity: "wolf-alpha",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("object"), "p", "tick", true, one(wolf, incOp("hp", -1)))
			}},
		{name: "swarm without meta.agent: nobody the table knows", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, proposer{source: contracts.SourceSwarm}, "p", "combat", true, one(playerA, incOp("hp", -1)))
			}},
		{name: "an agent that names author as its level", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("author"), "p", "author", true, one(playerA, incOp("hp", -1)))
			}},
		{name: "author: the only causes are init and author", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "move", true, one(playerA, setOp("position", forestRegion)))
			}},
		{name: "gateway: a create under the cause of the author", want: state.ReasonLevelViolation, entity: "player-D",
			proposal: func(*testing.T) eventbus.Event {
				return createdBy(gateway, "p", "init", ref("player-D", entity.TypePlayer), "", character(entity.StatusAlive, outside))
			}},
		{name: "gateway: a create of an NPC", want: state.ReasonLevelViolation, entity: "wolf-beta",
			proposal: func(*testing.T) eventbus.Event {
				return createdBy(gateway, "p", "create", ref("wolf-beta", entity.TypeNPC), "", map[string]any{entity.AttrHP: 1})
			}},
		{name: "gateway: a character created", entity: "player-D",
			proposal: func(*testing.T) eventbus.Event {
				return createdBy(gateway, "p", "create", ref("player-D", entity.TypePlayer), "", character(entity.StatusAlive, outside))
			}},
		{name: "task: an element under the prefix inventory, inventory[0]", entity: "player-A",
			arrange: func(t *testing.T, f *fixture) {
				f.seed(t, "player-A", entity.TypePlayer, 1, merge(character(entity.StatusAlive, outside),
					entity.AttrInventory, []any{map[string]any{"item_id": "pelt-1", "kind": "wolf-pelt"}}))
			},
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "loot", true, one(playerA, op(entity.OpRemove, "inventory[0]", nil)))
			}},
		{name: "gateway source with meta.agent of level gateway: no rights of the gateway", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, proposer{source: contracts.SourceGateway, level: "gateway"}, "p", "forget", true,
					one(playerA, setOp("status", entity.StatusAbandoned)))
			}},
		{name: "the double of the gateway proposes as the gateway", entity: "player-D",
			proposal: func(*testing.T) eventbus.Event {
				return createdBy(proposer{source: contracts.SourceTestkitGateway}, "p", "create",
					ref("player-D", entity.TypePlayer), "", character(entity.StatusAlive, outside))
			}},
		{name: "domain: an encounter created with cause=spawn", entity: "enc-2",
			proposal: func(*testing.T) eventbus.Event {
				return createdBy(agentOf("domain"), "p", "spawn", ref("enc-2", entity.TypeEncounter), "",
					encounter(entity.EncounterStateActive, "", "wolf-alpha"))
			}},

		// --- level_violation: the norms over the table (C-02 v1.4, v1.5) ---
		{name: "gateway: set status with cause=move", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "move", true, one(playerA, setOp("status", entity.StatusAlive)))
			}},
		{name: "gateway: hp with cause=move", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "move", true, one(playerA, setOp("hp", 10)))
			}},
		{name: "gateway: a status other than abandoned with cause=forget", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "forget", true, one(playerA, setOp("status", entity.StatusDead)))
			}},
		{name: "task: abandoned with cause=death", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "death", true, one(playerA, setOp("status", entity.StatusAbandoned)))
			}},
		{name: "task: abandoned with cause=forget, the transition of the gateway from an agent", want: state.ReasonLevelViolation, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "forget", true, one(playerA, setOp("status", entity.StatusAbandoned)))
			}},
		{name: "gateway: alive -> abandoned with cause=forget", entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "forget", true, one(playerA, setOp("status", entity.StatusAbandoned)))
			}},
		{name: "gateway: rest outside an encounter up to hp_max", entity: "player-A",
			arrange: func(t *testing.T, f *fixture) {
				f.seed(t, "player-A", entity.TypePlayer, 1, merge(character(entity.StatusAlive, outside), entity.AttrHP, 3))
			},
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "rest", true, one(playerA, setOp("hp", 10)))
			}},

		// --- invalid_op: a path not in its canonical form (review #1, Ma-2) ---
		{name: "task: status. with a dot at the end, into abandoned", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, one(playerA, setOp("status.", entity.StatusAbandoned)))
			}},
		{name: "gateway: status. into dead with cause=forget", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "forget", true, one(playerA, setOp("status.", entity.StatusDead)))
			}},
		{name: "domain: .description with a dot in front", want: state.ReasonInvalidOp, entity: forestRegion,
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("domain"), "p", "tick", true, one(ref(forestRegion, entity.TypeRegion), setOp(".description", "bright")))
			}},
		{name: "gateway: .encounter_id of a group", want: state.ReasonInvalidOp, entity: "g-1",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "group", true, one(ref("g-1", entity.TypeGroup), setOp(".encounter_id", "enc-1")))
			}},
		{name: "author: an empty segment a..b", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, one(playerA, setOp("mark..fox", 1)))
			}},
		{name: "author: an index with a leading zero, inventory[01]", want: state.ReasonInvalidOp, entity: "player-A",
			arrange: func(t *testing.T, f *fixture) {
				f.seed(t, "player-A", entity.TypePlayer, 1, merge(character(entity.StatusAlive, outside),
					entity.AttrInventory, []any{map[string]any{"item_id": "a"}, map[string]any{"item_id": "b"}}))
			},
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, one(playerA, op(entity.OpRemove, "inventory[01]", nil)))
			}},

		// --- invalid_op: a path below a scalar attribute (review #2, Ma-3) ---
		{name: "task: status.x into abandoned", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, one(playerA, setOp("status.x", entity.StatusAbandoned)))
			}},
		{name: "task: status.x into a status the matrix does not know", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, one(playerA, setOp("status.x", "sleeping")))
			}},
		{name: "gateway: status.x into dead with cause=forget", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "forget", true, one(playerA, setOp("status.x", entity.StatusDead)))
			}},
		{name: "gateway: rest writes hp.x", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "rest", true, one(playerA, setOp("hp.x", 999)))
			}},
		{name: "author: hp_max.x, which no norm reads", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, one(playerA, setOp("hp_max.x", 5)))
			}},
		{name: "author: name.x, the attribute of every type", want: state.ReasonInvalidOp, entity: "player-A",
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, one(playerA, setOp("name.x", "Ари")))
			}},

		// --- invalid_op: a key that reads as an index, an index past nine digits (У-1) ---
		{name: "author: remove inventory.0 on a list of two", want: state.ReasonInvalidOp, entity: "player-A",
			arrange: twoItems,
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, one(playerA, op(entity.OpRemove, "inventory.0", nil)))
			}},
		{name: "author: set inventory.0.kind", want: state.ReasonInvalidOp, entity: "player-A",
			arrange: twoItems,
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, one(playerA, setOp("inventory.0.kind", "rope")))
			}},
		{name: "author: an index of ten digits, inventory[1000000000]", want: state.ReasonInvalidOp, entity: "player-A",
			arrange: twoItems,
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, one(playerA, op(entity.OpRemove, "inventory[1000000000]", nil)))
			}},
		{name: "author: an index of two digits, inventory[10]", entity: "player-A",
			arrange: func(t *testing.T, f *fixture) {
				items := make([]any, 11)
				for i := range items {
					items[i] = map[string]any{"item_id": "item-" + itoa(i)}
				}
				f.seed(t, "player-A", entity.TypePlayer, 1, merge(character(entity.StatusAlive, outside), entity.AttrInventory, items))
			},
			proposal: func(t *testing.T) eventbus.Event {
				return update(t, "p", "author", true, one(playerA, op(entity.OpRemove, "inventory[10]", nil)))
			}},

		// --- invalid_op: a path below a scalar of every type of entity (У-2) ---
		{name: "task: state.x resolved on an encounter", want: state.ReasonInvalidOp, entity: "enc-1",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "resolve", true, one(enc1, setOp("state.x", entity.EncounterStateResolved)))
			}},
		{name: "global: weather.x of the world", want: state.ReasonInvalidOp, entity: world,
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("global"), "p", "tick", true, one(ref(world, entity.TypeWorld), setOp("weather.x", "rain")))
			}},
		{name: "domain: respawn_ttl.x of a region", want: state.ReasonInvalidOp, entity: forestRegion,
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("domain"), "p", "tick", true, one(ref(forestRegion, entity.TypeRegion), setOp("respawn_ttl.x", "1h")))
			}},
		{name: "domain: region_id.x of an NPC", want: state.ReasonInvalidOp, entity: "wolf-alpha",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("domain"), "p", "tick", true, one(wolf, setOp("region_id.x", forestRegion)))
			}},
		{name: "gateway: leader_id.x of a group", want: state.ReasonInvalidOp, entity: "g-1",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "group", true, one(ref("g-1", entity.TypeGroup), setOp("leader_id.x", "player-A")))
			}},

		// --- dead_entity: step 5, before ownership (the full table is below) ---
		{name: "gateway: a move of a dead character, not its path to write either", want: state.ReasonDeadEntity, entity: "player-A",
			arrange: func(t *testing.T, f *fixture) {
				f.seed(t, "player-A", entity.TypePlayer, 1, merge(character(entity.StatusDead, outside), entity.AttrHP, 0))
			},
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "combat", true, one(playerA, incOp("hp", -1)))
			}},

		// --- law_violation: the norm of rest reads the world ---
		{name: "gateway: rest inside an encounter", want: state.ReasonLawViolation, entity: "player-A",
			arrange: func(t *testing.T, f *fixture) {
				f.seed(t, "player-A", entity.TypePlayer, 1, merge(character(entity.StatusAlive, forestRegion),
					entity.AttrHP, 3, entity.AttrEncounterID, "enc-1"))
			},
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "rest", true, one(playerA, setOp("hp", 10)))
			}},
		{name: "gateway: rest inside an encounter that erases encounter_id in the same set", want: state.ReasonLawViolation, entity: "player-A",
			arrange: func(t *testing.T, f *fixture) {
				f.seed(t, "player-A", entity.TypePlayer, 1, merge(character(entity.StatusAlive, forestRegion),
					entity.AttrHP, 3, entity.AttrEncounterID, "enc-1"))
			},
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "rest", true, one(playerA, setOp("hp", 10), setOp("encounter_id", "")))
			}},
		{name: "gateway: rest above hp_max", want: state.ReasonLawViolation, entity: "player-A", law: "inv-02",
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, gateway, "p", "rest", true, one(playerA, setOp("hp", 50)))
			}},

		// --- law_violation: the laws of the world ---
		{name: "task: a package of the encounter alone over a dead wolf", want: state.ReasonLawViolation, entity: "wolf-alpha", law: "inv-01",
			arrange: func(t *testing.T, f *fixture) {
				f.seed(t, "wolf-alpha", entity.TypeNPC, 1, map[string]any{
					entity.AttrHP: 0, entity.AttrHPMax: 8, entity.AttrStatus: entity.StatusDead, entity.AttrPosition: forestRegion,
				})
			},
			proposal: func(t *testing.T) eventbus.Event {
				return proposed(t, agentOf("task"), "p", "combat", true, one(enc1, setOp("round_seq", 2)))
			}},
		{name: "author: a create that stands nowhere", want: state.ReasonLawViolation, entity: "wolf-beta", law: "inv-10",
			proposal: func(*testing.T) eventbus.Event {
				return create("p", ref("wolf-beta", entity.TypeNPC), "", map[string]any{
					entity.AttrHP: 5, entity.AttrHPMax: 5, entity.AttrStatus: entity.StatusAlive, entity.AttrPosition: "nowhere",
				})
			}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := forest(t)
			if c.arrange != nil {
				c.arrange(t, f)
			}
			ev := c.proposal(t)
			f.apply(t, ev)
			answer := f.answer(t)
			if c.want == "" {
				if answer.Type == state.TypeRejected {
					reason, _ := answer.Path().GetString("reason")
					t.Fatalf("refused %s, want it applied: %s", reason, f.log)
				}
				if id, _ := answer.Path().GetString("entity.entity.id"); id != c.entity {
					t.Errorf("the fact is about %q, want %q", id, c.entity)
				}
				return
			}
			assertRejected(t, answer, "p", c.want, c.entity)
			if law, _ := answer.Path().GetString("details.invariant_id"); law != c.law {
				t.Errorf("details.invariant_id %q, want %q", law, c.law)
			}
		})
	}

	// Every reason of the enum is in the matrix.
	seen := map[state.Reason]bool{}
	for _, c := range cases {
		seen[c.want] = true
	}
	for _, reason := range []state.Reason{state.ReasonUnknownEntity, state.ReasonVersionConflict, state.ReasonInvalidOp,
		state.ReasonDuplicateEntity, state.ReasonLevelViolation, state.ReasonLawViolation, state.ReasonDeadEntity} {
		if !seen[reason] {
			t.Errorf("no row of the matrix answers %s", reason)
		}
	}
}

func merge(attrs map[string]any, kv ...any) map[string]any {
	for i := 0; i+1 < len(kv); i += 2 {
		attrs[kv[i].(string)] = kv[i+1]
	}
	return attrs
}

// --- dead_entity (§4.5 p. 5, C-02 v1.2, v1.5b) ---

// TestATerminalEntityTakesOnlyThePathsOfACorpse: over dead, abandoned and
// ascended_final alike, any change but the four paths of a corpse is
// dead_entity — before ownership and before the laws — and so is leaving the
// terminal status, even in a package that erases the record of the death too.
// Each of the four paths is still taken.
func TestATerminalEntityTakesOnlyThePathsOfACorpse(t *testing.T) {
	corpse := func(t *testing.T, status string) *fixture {
		f := forest(t)
		f.seed(t, "player-A", entity.TypePlayer, 3, merge(character(status, outside),
			entity.AttrHP, 0, entity.AttrDiedAt, "2026-09-13T00:00:00Z", entity.AttrKilledBy, "wolf-alpha"))
		return f
	}
	for _, status := range entity.TerminalStatuses {
		t.Run(status, func(t *testing.T) {
			for name, tc := range map[string]struct {
				by    proposer
				cause string
				ops   []entity.Op
			}{
				"a wound":                                {agentOf("task"), "combat", []entity.Op{incOp("hp", -1)}},
				"back to alive":                          {agentOf("task"), "combat", []entity.Op{setOp("status", entity.StatusAlive)}},
				"back to alive, the record erased":       {author, "author", []entity.Op{setOp("status", entity.StatusAlive), op(entity.OpRemove, "died_at", nil), op(entity.OpRemove, "killed_by", nil)}},
				"a path of the table with a corpse path": {author, "author", []entity.Op{setOp("killed_by", "x"), setOp("position", forestRegion)}},
				"abandoned again from the gateway":       {gateway, "forget", []entity.Op{setOp("status", entity.StatusAbandoned)}},
				"a move by an owner without rights":      {gateway, "move", []entity.Op{setOp("scope", "x")}},
			} {
				t.Run(name, func(t *testing.T) {
					f := corpse(t, status)
					f.apply(t, proposed(t, tc.by, "p", tc.cause, true, set(playerA, v(3), tc.ops...)))
					assertRejected(t, f.answer(t), "p", state.ReasonDeadEntity, "player-A")
					if e := f.get(t, "player-A"); e.Version != 3 {
						t.Errorf("the corpse moved to v%d", e.Version)
					}
				})
			}
			for _, path := range []string{entity.AttrDiedAt, entity.AttrKilledBy, entity.AttrLootClaimedBy, entity.AttrEncounterID} {
				t.Run("the corpse path "+path, func(t *testing.T) {
					f := corpse(t, status)
					f.apply(t, update(t, "p", "author", true, set(playerA, v(3), setOp(path, "enc-1"))))
					if answer := f.answer(t); answer.Type != state.TypeUpdated {
						reason, _ := answer.Path().GetString("reason")
						t.Fatalf("%s of a %s entity refused %s, want applied", path, status, reason)
					}
					if e := f.get(t, "player-A"); e.Version != 4 {
						t.Errorf("v%d, want v4", e.Version)
					}
				})
			}
		})
	}
}

// --- the abandoned character (C-02 v1.2) and the same value (C-02 v1.4) ---

// TestForgetAbandonsALivingCharacter: the gateway proposes alive -> abandoned in
// the cascade of /forget, atomic, with the version pinned and without an agent;
// the fact carries the transition and cause=forget. Afterwards the character is
// terminal: the same /forget under a new proposal_id is dead_entity, and dead
// never becomes abandoned.
func TestForgetAbandonsALivingCharacter(t *testing.T) {
	f := forest(t)
	f.apply(t, proposed(t, gateway, "forget:player-A", "forget", true, one(playerA, setOp("status", entity.StatusAbandoned))))

	fact := f.answer(t)
	if fact.Type != state.TypeUpdated {
		t.Fatalf("answer %s, want %s: %s", fact.Type, state.TypeUpdated, f.log)
	}
	p := fact.Path()
	if cause, _ := p.GetString("cause"); cause != "forget" {
		t.Errorf("cause %q, want forget", cause)
	}
	changed, _ := p.GetSlice("changed")
	want := []any{map[string]any{"path": "status", "old": entity.StatusAlive, "new": entity.StatusAbandoned}}
	if !reflect.DeepEqual(changed, want) {
		t.Errorf("changed %v, want %v", changed, want)
	}
	if e := f.get(t, "player-A"); e.Version != 2 {
		t.Errorf("v%d, want v2", e.Version)
	}

	f.apply(t, proposed(t, gateway, "forget:player-A:again", "forget", true, set(playerA, v(2), setOp("status", entity.StatusAbandoned))))
	all := f.journal.all()
	assertRejected(t, all[len(all)-1], "forget:player-A:again", state.ReasonDeadEntity, "player-A")

	g := forest(t)
	g.seed(t, "player-A", entity.TypePlayer, 1, character(entity.StatusDead, outside))
	g.apply(t, proposed(t, gateway, "forget:player-A", "forget", true, one(playerA, setOp("status", entity.StatusAbandoned))))
	assertRejected(t, g.answer(t), "forget:player-A", state.ReasonDeadEntity, "player-A")
}

// TestAStatusSetToItsValueIsATurnWithoutAChange: set status alive over alive is
// a fact with an empty changed[] and the version where it was (C-02 v1.4);
// the matrix of transitions is asked about what changed, not about the
// operation.
func TestAStatusSetToItsValueIsATurnWithoutAChange(t *testing.T) {
	f := forest(t)
	f.apply(t, proposed(t, agentOf("task"), "p", "combat", true, one(playerA, setOp("status", entity.StatusAlive))))

	fact := f.answer(t)
	if fact.Type != state.TypeUpdated {
		t.Fatalf("answer %s, want a fact", fact.Type)
	}
	if changed, ok := fact.Payload["changed"].([]any); !ok || len(changed) != 0 {
		t.Errorf("changed %v, want an empty list", fact.Payload["changed"])
	}
	if version, _ := fact.Path().GetInt("version"); version != 1 {
		t.Errorf("version %d, want 1", version)
	}
}

// --- the laws of the world (§4.5 p. 8, КД §4.5 p. 8) ---

// TestASetToTheSameValueStillAsksTheLaws: touched holds every entity of the
// applied part, those without a change included, so a package that changes
// nothing over an entity breaking a law is refused.
func TestASetToTheSameValueStillAsksTheLaws(t *testing.T) {
	f := forest(t)
	f.seed(t, "player-C", entity.TypePlayer, 1, character(entity.StatusAlive, "nowhere"))
	f.apply(t, proposed(t, agentOf("task"), "p", "combat", true, one(ref("player-C", entity.TypePlayer), setOp("hp", 10))))

	answer := f.answer(t)
	assertRejected(t, answer, "p", state.ReasonLawViolation, "player-C")
	if law, _ := answer.Path().GetString("details.invariant_id"); law != "inv-10" {
		t.Errorf("details.invariant_id %q, want inv-10", law)
	}
}

// TestALawAnswersOnAnEntityOutsideThePackage: inv-01 answers on the wolf of an
// encounter the package only touches; the refusal names the wolf with its type
// from the world.
func TestALawAnswersOnAnEntityOutsideThePackage(t *testing.T) {
	f := forest(t)
	f.seed(t, "wolf-dead", entity.TypeNPC, 1, map[string]any{
		entity.AttrHP: 0, entity.AttrHPMax: 8, entity.AttrStatus: entity.StatusDead, entity.AttrPosition: forestRegion,
	})
	f.seed(t, "enc-2", entity.TypeEncounter, 1, encounter(entity.EncounterStateActive, "", "wolf-dead"))

	f.apply(t, proposed(t, agentOf("task"), "p", "combat", true, one(ref("enc-2", entity.TypeEncounter), setOp("round_seq", 2))))

	answer := f.answer(t)
	assertRejected(t, answer, "p", state.ReasonLawViolation, "wolf-dead")
	if typ, _ := answer.Path().GetString("entity.entity.type"); typ != entity.TypeNPC {
		t.Errorf("entity.type %q, want npc from the world", typ)
	}
	if law, _ := answer.Path().GetString("details.invariant_id"); law != "inv-01" {
		t.Errorf("details.invariant_id %q, want inv-01", law)
	}
}

// TestTheFirstBrokenLawIsTheSameOnEveryRun: a proposal breaking two laws is
// refused by the first in the order of the register, the same refusal on fifty
// runs.
func TestTheFirstBrokenLawIsTheSameOnEveryRun(t *testing.T) {
	var first []byte
	for run := range 50 {
		f := forest(t)
		f.apply(t, update(t, "p", "author", true, one(playerA, setOp("hp", 50), setOp("position", "nowhere"))))
		answer := f.answer(t)
		assertRejected(t, answer, "p", state.ReasonLawViolation, "player-A")
		if law, _ := answer.Path().GetString("details.invariant_id"); law != "inv-02" {
			t.Fatalf("run %d: details.invariant_id %q, want inv-02, the first of the register", run, law)
		}
		raw := f.journal.raw[0]
		if first == nil {
			first = raw
			continue
		}
		if string(raw) != string(first) {
			t.Fatalf("run %d refused\n%s\nthe first run\n%s", run, raw, first)
		}
	}
}

// TestAForgetOvertakingAPackageOfTheEncounterIsNotRefused is the race of /forget
// in a solo fight (review of T-054, Minor-1, probe P3): the character is
// abandoned before the package "both missed" that touches only the encounter
// arrives. C-05 v1.8 p. 9 (b) — the decision of system-architect on question
// В1 — lets it through: the participation is the agent's to rewrite.
func TestAForgetOvertakingAPackageOfTheEncounterIsNotRefused(t *testing.T) {
	f := forest(t)
	f.apply(t, proposed(t, gateway, "forget:player-A", "forget", true, one(playerA, setOp("status", entity.StatusAbandoned))))
	f.apply(t, proposed(t, agentOf("task"), "enc-round-2", "combat", true, one(enc1, setOp("round_seq", 2))))

	all := f.journal.all()
	if len(all) != 2 || all[1].Type != state.TypeUpdated {
		t.Fatalf("published %v, want two facts: %s", types(all), f.log)
	}
}

// TestALooseLawBreakRefusesOnlyItsChangeSet: atomic=false, the change set of the
// entity a law names is refused and the laws are asked again about the rest,
// which is applied.
func TestALooseLawBreakRefusesOnlyItsChangeSet(t *testing.T) {
	f := forest(t)
	f.apply(t, update(t, "p", "author", false,
		one(playerA, setOp("position", "nowhere")),
		one(wolf, incOp("hp", -1))))

	all := f.journal.all()
	if got := strings.Join(types(all), ","); got != "entity.updated,entity.update.rejected" {
		t.Fatalf("published %s, want the fact of the wolf and the refusal of the character", got)
	}
	assertRejected(t, all[1], "p", state.ReasonLawViolation, "player-A")
	if f.get(t, "player-A").Version != 1 || f.get(t, "wolf-alpha").Version != 2 {
		t.Error("the versions do not match what was applied")
	}
	if batch := f.get(t, "wolf-alpha").LastChange.BatchSize; batch != 1 {
		t.Errorf("batch_size %d, want 1 applied", batch)
	}
}

// TestADeathThatLeavesTheCharacterInTheFightIsRefused is probe P2 of the review
// of T-054 on State: the package kills the character and touches the encounter
// without rewriting the participation (C-05 v1.8 p. 9: the same package writes
// participants[].state = dead). The character is touched by the package, so
// inv-01 answers on it.
func TestADeathThatLeavesTheCharacterInTheFightIsRefused(t *testing.T) {
	f := forest(t)
	f.apply(t, proposed(t, agentOf("task"), "p", "combat", true,
		one(playerA, setOp("hp", 0), setOp("status", entity.StatusDead)),
		one(enc1, setOp("round_seq", 2))))

	answer := f.answer(t)
	assertRejected(t, answer, "p", state.ReasonLawViolation, "player-A")
	if law, _ := answer.Path().GetString("details.invariant_id"); law != "inv-01" {
		t.Errorf("details.invariant_id %q, want inv-01", law)
	}
}

// TestTheNormsOfRestHoldWithoutTheLaws: State holds rest to hp_max even where no
// rule set gives it the laws of the world — the process of I1 runs State
// without them — and names inv-02, the law of the condition (C-02 v1.5).
func TestTheNormsOfRestHoldWithoutTheLaws(t *testing.T) {
	f := newOwnedFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, character(entity.StatusAlive, outside))

	f.apply(t, proposed(t, gateway, "p", "rest", true, one(playerA, setOp("hp", 50))))

	answer := f.answer(t)
	assertRejected(t, answer, "p", state.ReasonLawViolation, "player-A")
	if law, _ := answer.Path().GetString("details.invariant_id"); law != "inv-02" {
		t.Errorf("details.invariant_id %q, want inv-02", law)
	}
}

// TestEverySetOfALoosePackageIsAnswered is Ma-1 of review #1: in a package with
// atomic=false every change set gets a fact or a refusal. The wolf is dead: its
// wound is refused dead_entity, and the change of the encounter breaks inv-01 on
// that wolf, whose own change set is gone already. The encounter is still
// answered — a refusal under its own entity, with the law that answered.
func TestEverySetOfALoosePackageIsAnswered(t *testing.T) {
	f := forest(t)
	f.seed(t, "wolf-alpha", entity.TypeNPC, 1, map[string]any{
		entity.AttrHP: 0, entity.AttrHPMax: 8, entity.AttrStatus: entity.StatusDead, entity.AttrPosition: forestRegion,
	})

	f.apply(t, proposed(t, agentOf("task"), "p", "combat", false,
		one(wolf, incOp("hp", -1)),
		one(enc1, setOp("round_seq", 2))))

	all := f.journal.all()
	if got := strings.Join(types(all), ","); got != "entity.update.rejected,entity.update.rejected" {
		t.Fatalf("published %s, want a refusal for each of the two change sets", got)
	}
	assertRejected(t, all[0], "p", state.ReasonDeadEntity, "wolf-alpha")
	assertRejected(t, all[1], "p", state.ReasonLawViolation, "enc-1")
	if law, _ := all[1].Path().GetString("details.invariant_id"); law != "inv-01" {
		t.Errorf("details.invariant_id %q, want inv-01", law)
	}
	if all[0].ID == all[1].ID {
		t.Error("the two refusals share an id: a consumer that drops repeats would lose one")
	}
	if f.get(t, "enc-1").Version != 1 {
		t.Error("the encounter moved under a refused change set")
	}
}

// TestALawOutsideALoosePackageRefusesEachRemainingSet: no change set of the
// package names the entity the law answers on, and none was refused before. Each
// change set is refused under its own entity with the law, nothing is applied.
func TestALawOutsideALoosePackageRefusesEachRemainingSet(t *testing.T) {
	f := forest(t)
	f.seed(t, "wolf-dead", entity.TypeNPC, 1, map[string]any{
		entity.AttrHP: 0, entity.AttrHPMax: 8, entity.AttrStatus: entity.StatusDead, entity.AttrPosition: forestRegion,
	})
	f.seed(t, "enc-2", entity.TypeEncounter, 1, encounter(entity.EncounterStateActive, "", "wolf-dead"))

	f.apply(t, update(t, "p", "author", false,
		one(ref("enc-2", entity.TypeEncounter), setOp("round_seq", 2)),
		one(playerA, incOp("hp", -1))))

	all := f.journal.all()
	if got := strings.Join(types(all), ","); got != "entity.update.rejected,entity.update.rejected" {
		t.Fatalf("published %s, want a refusal for each change set", got)
	}
	named := map[string]bool{}
	for _, refusal := range all {
		id, _ := refusal.Path().GetString("entity.entity.id")
		named[id] = true
		if reason, _ := refusal.Path().GetString("reason"); reason != string(state.ReasonLawViolation) {
			t.Errorf("%s refused %s, want law_violation", id, reason)
		}
		if law, _ := refusal.Path().GetString("details.invariant_id"); law != "inv-01" {
			t.Errorf("%s: details.invariant_id %q, want inv-01", id, law)
		}
	}
	if !named["enc-2"] || !named["player-A"] {
		t.Errorf("the refusals name %v, want enc-2 and player-A, each set under its own entity", named)
	}
	if f.get(t, "enc-2").Version != 1 || f.get(t, "player-A").Version != 1 {
		t.Error("a change set was applied under a broken law")
	}
}

// twoItems puts two items into the inventory of player-A.
func twoItems(t *testing.T, f *fixture) {
	t.Helper()
	f.seed(t, "player-A", entity.TypePlayer, 1, merge(character(entity.StatusAlive, outside),
		entity.AttrInventory, []any{map[string]any{"item_id": "a"}, map[string]any{"item_id": "b"}}))
}
