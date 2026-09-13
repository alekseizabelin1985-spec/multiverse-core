package outbox_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

var update = flag.Bool("update", false, "write testdata/golden/*.txt of the outbox from the texts")

// goldenDir holds one text per case. The files are merge=binary
// (testdata/golden/.gitattributes): a conflicting text is taken whole from one
// side and written again by -update, never merged line by line.
var goldenDir = filepath.Join("testdata", "golden")

// world is the projection the texts read: a player, a wolf, a region.
type world struct{}

func (world) Character(id string) (readmodel.CharacterState, bool) {
	if id != playerA {
		return readmodel.CharacterState{}, false
	}
	return readmodel.CharacterState{ID: playerA, Name: "Вася", HP: 10, HPMax: 10, Status: entity.StatusAlive}, true
}

func (world) NPC(id string) (readmodel.NPC, bool) {
	if id != "wolf-alpha" {
		return readmodel.NPC{}, false
	}
	return readmodel.NPC{ID: id, Name: "Волк", HP: 7, HPMax: 10, Status: entity.StatusAlive}, true
}

func (world) Region(worldID, regionID string) (readmodel.Region, bool) {
	if worldID != "dark-forest-world" || regionID != "dark-forest-01" {
		return readmodel.Region{}, false
	}
	return readmodel.Region{ID: regionID, WorldID: worldID, Name: "Тёмный лес"}, true
}

func (world) Encounter(id string) (readmodel.Encounter, bool) {
	if id != "enc-1" {
		return readmodel.Encounter{}, false
	}
	return readmodel.Encounter{ID: id, State: entity.EncounterStateActive, Participants: []string{playerA},
		NPCIDs: []string{"wolf-alpha"}}, true
}

func event(typ string, payload map[string]any) eventbus.Event {
	raw, _ := json.Marshal(payload)
	var p map[string]any
	_ = json.Unmarshal(raw, &p)
	return eventbus.NewRoot(typ, contracts.SourceState, "dark-forest-world",
		&eventbus.ScopeRef{ID: playerA, Type: "solo"}, eventbus.ActorSystem, p)
}

func ref(id, typ, name string) map[string]any {
	out := map[string]any{"entity": map[string]any{"id": id, "type": typ}}
	if name != "" {
		out["name"] = name
	}
	return out
}

func decided(action string, attacker, defender map[string]any, outcome map[string]any, hp map[string]any, extra map[string]any) eventbus.Event {
	p := map[string]any{"encounter": ref("enc-1", entity.TypeEncounter, ""), "round": map[string]any{"seq": 1},
		"action": action, "attacker": attacker, "outcome": outcome, "rolls": []any{}, "rules_version": "1", "phase1_mode": "rules"}
	if defender != nil {
		p["defender"] = defender
	}
	if hp != nil {
		p["hp"] = hp
	}
	for k, v := range extra {
		p[k] = v
	}
	return event(outbox.TypeCombatDecided, p)
}

func fact(id, typ, cause string, changed ...map[string]any) eventbus.Event {
	list := make([]any, 0, len(changed))
	for _, c := range changed {
		list = append(list, c)
	}
	return event(outbox.TypeEntityUpdated, map[string]any{"entity": ref(id, typ, ""), "version": 2, "changed": list,
		"cause": cause, "proposal_id": "p-1", "applied_at": "2026-09-13T12:00:00Z"})
}

// renderCases are the texts of the rules component §7.2 lists: a hit, a miss,
// a critical hit, the attack of an NPC and its blow at a fleeing player, a
// flight that worked and two that did not, a death, a rest, both moves, the
// move of a group, loot, the opening of an encounter and the refusals of State.
func renderCases() map[string]string {
	player := ref(playerA, entity.TypePlayer, "Вася")
	wolf := ref("wolf-alpha", entity.TypeNPC, "Волк")
	mechanics := func(ev eventbus.Event) string {
		text, _, ok := outbox.Mechanics(ev, world{})
		if !ok {
			return "<no text>"
		}
		return text
	}
	success, failure := true, false
	opened, _ := outbox.EncounterOpened(readmodel.Encounter{ID: "enc-1", NPCIDs: []string{"wolf-alpha"}}, world{})
	return map[string]string{
		"attack_hit": mechanics(decided("attack", player, wolf, map[string]any{"hit": true, "natural": 14, "damage": 3},
			map[string]any{"defender_before": 10, "defender_after": 7, "defender_max": 10}, nil)),
		"attack_miss": mechanics(decided("attack", player, wolf, map[string]any{"hit": false, "natural": 3, "damage": 0},
			map[string]any{"defender_before": 7, "defender_after": 7, "defender_max": 10}, nil)),
		"attack_critical": mechanics(decided("attack", player, ref("wolf-alpha", entity.TypeNPC, ""),
			map[string]any{"hit": true, "natural": 20, "damage": 6, "critical": true},
			map[string]any{"defender_before": 7, "defender_after": 1, "defender_max": 10}, nil)),
		"npc_attack": mechanics(decided("npc_attack", wolf, player, map[string]any{"hit": true, "natural": 12, "damage": 2},
			map[string]any{"defender_before": 10, "defender_after": 8, "defender_max": 10}, nil)),
		"free_attack": mechanics(decided("free_attack", wolf, player, map[string]any{"hit": false, "natural": 5, "damage": 0},
			map[string]any{"defender_before": 8, "defender_after": 8, "defender_max": 10}, nil)),
		"flee_success": mechanics(decided("flee", player, nil, map[string]any{"hit": false, "natural": 17, "success": success}, nil, nil)),
		"flee_fail": mechanics(decided("flee", player, nil, map[string]any{"hit": false, "natural": 2, "success": failure}, nil,
			map[string]any{"free_attack": false})),
		"flee_fail_free_attack": mechanics(decided("flee", player, nil, map[string]any{"hit": false, "natural": 2, "success": failure}, nil,
			map[string]any{"free_attack": true})),
		"death":            outbox.Died(),
		"rest":             mechanics(fact(playerA, entity.TypePlayer, outbox.CauseRest, map[string]any{"path": "hp", "old": 4, "new": 10})),
		"move_enter":       mechanics(fact(playerA, entity.TypePlayer, outbox.CauseMove, map[string]any{"path": "position", "old": "outside:dark-forest-world", "new": "dark-forest-01"})),
		"move_leave":       mechanics(fact(playerA, entity.TypePlayer, outbox.CauseMove, map[string]any{"path": "position", "old": "dark-forest-01", "new": "outside:dark-forest-world"})),
		"group_move":       mechanics(fact("group-1", entity.TypeGroup, outbox.CauseMove, map[string]any{"path": "position", "new": "dark-forest-01"})),
		"loot":             mechanics(fact(playerA, entity.TypePlayer, outbox.CauseLoot, map[string]any{"path": "inventory[0]", "new": map[string]any{"item_id": "wolf-pelt-1", "kind": "loot", "name": "Волчья шкура"}})),
		"encounter_opened": opened,
		"refused_move":     outbox.Refused("enter"),
		"refused_rest":     outbox.Refused("rest"),
	}
}

// Golden texts of the rules (component §7.2): Russian plain text, one file per
// case in testdata/golden. go test -run TestRenderGolden -update writes them.
func TestRenderGolden(t *testing.T) {
	cases := renderCases()
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range names {
		text := cases[name]
		path := filepath.Join(goldenDir, name+".txt")
		if *update {
			if err := os.WriteFile(path, []byte(text+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		golden, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v (go test -run TestRenderGolden -update writes it)", name, err)
			continue
		}
		if want := strings.TrimRight(strings.ReplaceAll(string(golden), "\r\n", "\n"), "\n"); text != want {
			t.Errorf("%s:\n got %q\nwant %q", name, text, want)
		}
	}
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".txt"); ok {
			if _, known := cases[name]; !known {
				t.Errorf("%s has no case", e.Name())
			}
		}
	}
}

// The texts carry no markup a parse_mode would read (SEC-10) and no code of
// State.
func TestRenderedTextsHaveNoMarkup(t *testing.T) {
	for name, text := range renderCases() {
		if strings.ContainsAny(text, "*_`[]<>") || text == "<no text>" {
			t.Errorf("%s: %q", name, text)
		}
		if strings.Contains(text, "version_conflict") {
			t.Errorf("%s names the reason of State: %q", name, text)
		}
	}
}

// An event without a text of the rules has none: a fight of combat, a fact of
// an NPC, a move without a position.
func TestMechanicsHasNoTextForOtherFacts(t *testing.T) {
	for name, ev := range map[string]eventbus.Event{
		"combat of a player": fact(playerA, entity.TypePlayer, "combat", map[string]any{"path": "hp", "new": 3}),
		"npc":                fact("wolf-alpha", entity.TypeNPC, outbox.CauseMove, map[string]any{"path": "position", "new": "dark-forest-01"}),
		"move without a position": fact(playerA, entity.TypePlayer, outbox.CauseMove,
			map[string]any{"path": "status", "new": "alive"}),
		"another type": event("encounter.ended", map[string]any{}),
	} {
		if text, _, ok := outbox.Mechanics(ev, world{}); ok {
			t.Errorf("%s: text %q", name, text)
		}
	}
}

// The data of a decision names the fighters by id and carries the numbers
// behind the text (api-contracts.md §1.5).
func TestMechanicsDataOfADecision(t *testing.T) {
	ev := decided("attack", ref(playerA, entity.TypePlayer, "Вася"), ref("wolf-alpha", entity.TypeNPC, "Волк"),
		map[string]any{"hit": true, "natural": 14, "damage": 3, "target_dead": true},
		map[string]any{"defender_before": 3, "defender_after": 0, "defender_max": 10}, nil)
	_, data, ok := outbox.Mechanics(ev, world{})
	combat, _ := data["combat"].(map[string]any)
	if !ok || combat["attacker"] != playerA || combat["defender"] != "wolf-alpha" || combat["damage"] != 3 ||
		combat["hp_after"] != 0 || combat["target_dead"] != true {
		t.Errorf("data = %v", data)
	}
}
