package mechanics

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/entity"
)

// rulesPath is the file the whole platform balances on. The tests read it
// rather than a fixture: golden numbers that live next to the assertion drift
// away from the file they claim to describe (ADR-012 p. 1).
const rulesPath = "../../rules/dark-forest.yaml"

func load(t *testing.T) *Rules {
	t.Helper()
	r, err := Load(rulesPath)
	if err != nil {
		t.Fatalf("load %s: %v", rulesPath, err)
	}
	return r
}

// readRules is the shipped file as text, for the tests that break one line of
// it on purpose.
func readRules() (string, error) {
	b, err := os.ReadFile(rulesPath)
	return string(b), err
}

// TestLoadGoldenNumbers is appendix A of the PRD (domain-review §3.3) as an
// assertion. A change to any of these numbers is a change to the balance of the
// world and has to be made here as well as in the file.
func TestLoadGoldenNumbers(t *testing.T) {
	r := load(t)

	if r.Version != "0.1" {
		t.Errorf("rules_version %q, want 0.1", r.Version)
	}
	if r.World != "dark-forest-world" {
		t.Errorf("world %q, want dark-forest-world", r.World)
	}

	want := map[string]Actor{
		"player": {Type: entity.TypePlayer, HP: 10, HPMax: 10, Atk: 2, Def: 12, Dmg: "d6", Flee: "2", Status: entity.StatusAlive},
		"wolf":   {Type: entity.TypeNPC, Kind: "wolf", HP: 10, HPMax: 10, Atk: 3, Def: 11, Dmg: "d4", Flee: "", Status: entity.StatusAlive},
	}
	for kind, expected := range want {
		got, ok := r.Stats(kind)
		if !ok {
			t.Fatalf("no stats for %s", kind)
		}
		if got != expected {
			t.Errorf("stats %s = %+v, want %+v", kind, got, expected)
		}
	}
	if _, ok := r.Stats("dragon"); ok {
		t.Error("stats for a kind the rules do not describe")
	}

	if got := r.Kinds(); len(got) != 2 || got[0] != "player" || got[1] != "wolf" {
		t.Errorf("kinds %v, want [player wolf]", got)
	}

	attack := r.Attack()
	if attack.CritNatural != 20 || attack.FumbleNatural != 1 || attack.CritMultiplier != 2 {
		t.Errorf("attack = %+v, want crit 20 x2, fumble 1", attack)
	}
	if got := strings.Join(attack.Rolls, ","); got != "hit,damage" {
		t.Errorf("attack.rolls = %v, want [hit damage]", attack.Rolls)
	}
	if hit, ok := r.Check(CheckAttackHit); !ok || hit.String() != "d20 + atk >= def" {
		t.Errorf("attack.hit = %q, want d20 + atk >= def", hit)
	}
	if !r.NPCAttack().OncePerRound {
		t.Error("npc_attack.once_per_round is off: an NPC would answer every action of a round")
	}

	flee, ok := r.Check(CheckFlee)
	if !ok || flee.String() != "d20 + flee >= 10 + living_enemies" {
		t.Errorf("flee.check = %q, want d20 + flee >= 10 + living_enemies", flee)
	}
	if r.Flee().OnFail != OnFailFreeAttack {
		t.Errorf("flee.on_fail = %q, want %s", r.Flee().OnFail, OnFailFreeAttack)
	}

	if r.Rest().Restore != RestoreToMax || r.Rest().AllowedInEncounter {
		t.Errorf("rest = %+v, want a full recovery outside an encounter", r.Rest())
	}

	target := r.TargetRules()
	if strings.Join(target.Order, ",") != "last_damager,min_hp,player_id_asc" {
		t.Errorf("npc_target.order = %v", target.Order)
	}
	if strings.Join(target.Exclude, ",") != "idle,out_of_combat,dead" {
		t.Errorf("npc_target.exclude = %v", target.Exclude)
	}

	loot := r.Loot("wolf")
	if len(loot) != 1 || loot[0].Kind != "wolf-pelt" {
		t.Errorf("loot wolf = %+v, want one wolf-pelt", loot)
	}
	if got := r.Loot("player"); got != nil {
		t.Errorf("loot player = %+v, want nothing: a character is not looted", got)
	}

	if r.Round() != (RoundRules{Timeout: 60 * time.Second, IdleAfterMissed: 2}) {
		t.Errorf("round = %+v, want 60s and 2 missed rounds", r.Round())
	}

	if got := len(r.Invariants()); got != 10 {
		t.Errorf("%d invariants in force, want all 10 (NFR-020)", got)
	}
}

// TestLootIsACopy: a caller handing a trophy out must not be able to empty the
// loot table of the world by editing what it was given.
func TestLootAndDocumentAreCopies(t *testing.T) {
	r := load(t)

	loot := r.Loot("wolf")
	loot[0].Name = "потрёпанная шкура"
	if r.Loot("wolf")[0].Name != "волчья шкура" {
		t.Error("editing the returned loot changed the rules")
	}

	doc := r.Document()
	doc.Entities["player"] = StatsDoc{HPMax: 999}
	doc.Invariants[0] = "inv-99"
	if stats, _ := r.Stats("player"); stats.HPMax != 10 {
		t.Error("editing the returned document changed the rules")
	}

	// The maps are cloned, but a StatsDoc carries the flight bonus by pointer
	// and a loot table by slice: a copy that shares either of them is not the
	// copy the doc comment promises.
	before := r.Document()
	flee := before.Entities["player"].Flee
	if flee == nil {
		t.Fatal("the fixture no longer gives the player a flee bonus: fix the test, not the rules")
	}
	*flee = 99
	before.Loot["wolf"][0].Name = "потрёпанная шкура"
	after := r.Document()
	if got := *after.Entities["player"].Flee; got != 2 {
		t.Errorf("flee of the player is %d after editing a returned document, want 2", got)
	}
	if got := after.Loot["wolf"][0].Name; got != "волчья шкура" {
		t.Errorf("trophy of the wolf is %q after editing a returned document", got)
	}
	if r.Invariants()[0].ID != InvDeadDoesNotAct {
		t.Error("editing the returned document changed the invariants")
	}
}

// TestInvariantsRegister pins what the foundation promises about the laws: ten
// identifiers, in order, every one of them nameable from a rules file, and no
// check implemented yet (T-054).
func TestInvariantsRegister(t *testing.T) {
	r := load(t)

	ids := InvariantIDs()
	if len(ids) != 10 {
		t.Fatalf("%d invariants in the register, want 10", len(ids))
	}
	for i, inv := range r.Invariants() {
		if inv.ID != ids[i] {
			t.Errorf("invariant %d is %s, want %s: the order of the register is the order in force", i, inv.ID, ids[i])
		}
		if len(inv.Where) == 0 {
			t.Errorf("%s says nowhere it is enforced", inv.ID)
		}
		if inv.Check != nil {
			t.Errorf("%s has a check: the logic belongs to EPIC-002 T-054", inv.ID)
		}
	}
}

// TestLoadRejects is the other half of Load: everything a rules file can get
// wrong is caught when it is read, not when a fight starts (§5.2).
func TestLoadRejects(t *testing.T) {
	base, err := readRules()
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}

	cases := map[string]struct {
		old, new string
		path     string
	}{
		"unknown top-level key": {
			old: "world: dark-forest-world",
			new: "world: dark-forest-world\ninitiative: true",
		},
		"unknown nested key": {
			old:  "crit_natural: 20",
			new:  "crit_natural: 20\n  crit_range: 19",
			path: "",
		},
		"future schema": {
			old: "schema_version: 1", new: "schema_version: 2", path: "schema_version",
		},
		"no rules version": {
			old: `rules_version: "0.1"`, new: `rules_version: "draft"`, path: "rules_version",
		},
		"no world": {
			old: "world: dark-forest-world", new: `world: ""`, path: "world",
		},
		"no entities at all": {
			old: "player: { hp_max: 10, atk: 2, def: 12, dmg: d6, flee: 2 }\n  wolf: { hp_max: 10, atk: 3, def: 11, dmg: d4, flee: null }",
			new: "{}", path: "entities",
		},
		"more dice than a turn can roll": {
			old: "dmg: d6", new: "dmg: 200000000d6", path: "entities.player.dmg",
		},
		"hp_max below one": {
			old: "player: { hp_max: 10", new: "player: { hp_max: 0", path: "entities.player.hp_max",
		},
		"def below one": {
			old: "def: 12", new: "def: 0", path: "entities.player.def",
		},
		"negative atk": {
			old: "atk: 2", new: "atk: -1", path: "entities.player.atk",
		},
		"negative damage": {
			old: "dmg: d6", new: "dmg: d6-10", path: "entities.player.dmg",
		},
		"damage that is not dice": {
			old: "dmg: d6", new: "dmg: two", path: "entities.player.dmg",
		},
		"unparsable hit formula": {
			old: `hit: "d20 + atk >= def"`, new: `hit: "d20 + atk > def"`, path: "attack.hit",
		},
		"unknown identifier in a formula": {
			old: `hit: "d20 + atk >= def"`, new: `hit: "d20 + luck >= def"`, path: "attack.hit",
		},
		"context identifier on the actor side": {
			old: `hit: "d20 + atk >= def"`, new: `hit: "d20 + living_enemies >= def"`, path: "attack.hit",
		},
		"crit outside the die": {
			old: "crit_natural: 20", new: "crit_natural: 21", path: "attack.crit_natural",
		},
		"crit and fumble on one face": {
			old: "fumble_natural: 1", new: "fumble_natural: 20", path: "attack.fumble_natural",
		},
		"crit multiplier below one": {
			old: "crit_multiplier: 2", new: "crit_multiplier: 0", path: "attack.crit_multiplier",
		},
		"damage formula names no dice": {
			old: "damage_formula: dmg", new: "damage_formula: atk", path: "attack.damage_formula",
		},
		"unknown roll purpose": {
			old: "rolls: [hit, damage]", new: "rolls: [hit, wound]", path: "attack.rolls[1]",
		},
		"npc attack inherits nothing": {
			old: "inherit: attack", new: "inherit: ~", path: "npc_attack.inherit",
		},
		"unknown flee failure": {
			old: "on_fail: free_attack", new: "on_fail: retreat", path: "flee.on_fail",
		},
		"unknown placeholder in a position": {
			old: `success_position: "outside:{world_id}"`, new: `success_position: "outside:{realm}"`, path: "flee.success_position",
		},
		"rest restores nothing readable": {
			old: "restore: hp_max", new: "restore: everything", path: "rest.restore",
		},
		// dice.rolled has no purpose for a rest roll, so a rest by dice would
		// be a chance nobody could audit (T-053, Nit-5 of review #1).
		"rest restores by dice": {
			old: "restore: hp_max", new: "restore: d4", path: "rest.restore",
		},
		"unknown target order": {
			old: "order: [last_damager, min_hp, player_id_asc]", new: "order: [random]", path: "npc_target.order[0]",
		},
		"repeated target order": {
			old: "order: [last_damager, min_hp, player_id_asc]", new: "order: [min_hp, min_hp]", path: "npc_target.order[1]",
		},
		"empty target order": {
			old: "order: [last_damager, min_hp, player_id_asc]", new: "order: []", path: "npc_target.order",
		},
		"unknown exclusion": {
			old: "exclude: [idle, out_of_combat, dead]", new: "exclude: [asleep]", path: "npc_target.exclude[0]",
		},
		"loot for a kind nobody plays": {
			old: "wolf: [{ kind: wolf-pelt", new: "bear: [{ kind: wolf-pelt", path: "loot.bear",
		},
		"nameless trophy": {
			old: `name: "волчья шкура"`, new: `name: ""`, path: "loot.wolf[0].name",
		},
		"unreadable round timeout": {
			old: "timeout: 60s", new: "timeout: soon", path: "round.timeout",
		},
		"round that never times out": {
			old: "timeout: 60s", new: "timeout: 0s", path: "round.timeout",
		},
		"idle after no missed round": {
			old: "idle_after_missed: 2", new: "idle_after_missed: 0", path: "round.idle_after_missed",
		},
		"unknown invariant": {
			old: "inv-01,", new: "inv-42,", path: "invariants[0]",
		},
		"repeated invariant": {
			old: "inv-02,", new: "inv-01,", path: "invariants[1]",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			broken := strings.Replace(base, c.old, c.new, 1)
			if broken == base {
				t.Fatalf("the fixture no longer contains %q: fix the test, not the rules", c.old)
			}
			_, err := LoadBytes([]byte(broken))
			if !errors.Is(err, ErrInvalidRules) {
				t.Fatalf("error %v, want invalid rules", err)
			}
			if c.path == "" {
				return
			}
			var cfg ConfigError
			if !errors.As(err, &cfg) {
				t.Fatalf("error %v is not a ConfigError", err)
			}
			if cfg.Path != c.path {
				t.Errorf("rejected %q, want %q (%v)", cfg.Path, c.path, err)
			}
		})
	}
}

// TestLoadRejectsGarbage covers the file that is not a rules document at all —
// the case where the decoder, not the validation, has to say no.
func TestLoadRejectsGarbage(t *testing.T) {
	for name, body := range map[string]string{
		"not yaml":     "schema_version: 1\n\tworld: x",
		"a list":       "- schema_version: 1",
		"empty":        "",
		"a plain word": "rules",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadBytes([]byte(body)); !errors.Is(err, ErrInvalidRules) {
				t.Fatalf("error %v, want invalid rules", err)
			}
		})
	}
}

// TestLoadMissingFile: Load names the file it could not read, because the
// caller that sees the error is usually holding the wrong path.
func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-rules.yaml")
	_, err := Load(path)
	if err == nil {
		t.Fatal("loaded a file that does not exist")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error %v, want a not-exist error", err)
	}
}

// TestLoadBytesNamesTheFile: an invalid file loaded through Load carries its
// path, so a stack of three worlds is still diagnosable.
func TestLoadPrefixesThePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.yaml")
	if err := os.WriteFile(path, []byte("schema_version: 7\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := Load(path)
	if !errors.Is(err, ErrInvalidRules) {
		t.Fatalf("error %v, want invalid rules", err)
	}
	if !strings.Contains(err.Error(), "broken.yaml") {
		t.Errorf("error %v does not name the file", err)
	}
}

// TestLoadNamesTheSameKeyEveryTime: with two kinds broken the same way, the
// caller has to hear about the same one on every run. Map order in Go is
// randomised, so a message picked by iteration turns a failure in CI into
// something the author of the rules cannot reproduce at home.
func TestLoadNamesTheSameKeyEveryTime(t *testing.T) {
	base, err := readRules()
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	broken := strings.NewReplacer(
		"player: { hp_max: 10", "player: { hp_max: 0",
		"wolf: { hp_max: 10", "wolf: { hp_max: 0",
	).Replace(base)
	if broken == base {
		t.Fatal("the fixture no longer contains both kinds at hp_max 10: fix the test, not the rules")
	}

	seen := map[string]int{}
	for range 100 {
		_, err := LoadBytes([]byte(broken))
		var cfg ConfigError
		if !errors.As(err, &cfg) {
			t.Fatalf("error %v is not a ConfigError", err)
		}
		seen[cfg.Path]++
	}
	if len(seen) != 1 {
		t.Errorf("100 identical loads named %d different keys: %v", len(seen), seen)
	}
	if _, ok := seen["entities.player.hp_max"]; !ok {
		t.Errorf("the key named is %v, want the first one in order", seen)
	}
}

// TestLoadRejectsASecondDocument: a rules file is one document. A "---" left
// behind by an editor silently drops everything after it, in a package where an
// unknown key is already an error because it is probably a typo.
func TestLoadRejectsASecondDocument(t *testing.T) {
	base, err := readRules()
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	if _, err := LoadBytes([]byte(base)); err != nil {
		t.Fatalf("the fixture itself no longer loads: %v", err)
	}

	for name, tail := range map[string]string{
		"a second rules document": "\n---\nschema_version: 99\n",
		"a second empty document": "\n---\n",
		"rubbish after the break": "\n---\n\tnot yaml\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadBytes([]byte(base + tail)); !errors.Is(err, ErrInvalidRules) {
				t.Fatalf("error %v, want invalid rules", err)
			}
		})
	}
}
