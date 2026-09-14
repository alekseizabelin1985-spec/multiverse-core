package entity_test

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"multiverse-core.io/shared/entity"
)

// The spellings the reviews of T-056 and system-architect named (review #1 Ma-2,
// review #2 N-3, review #3 N-4, condition U-1), moved here from internal/state
// with the rule (C-02 v1.8 p. 2).
func TestTheCanonicalSpellingOfAPath(t *testing.T) {
	spellings := map[string]bool{
		"hp": true, "inventory[0]": true, "inventory[10]": true, "inventory[999999999]": true,
		"npcs[0].last_damager": true, "position.region": true, "метка.лис": true, "a[1][2]": true,
		"": false, "status.": false, ".description": false, "mark..fox": false, "a[]": false, "a[x]": false,
		"inventory[01]": false, "inventory[00]": false, "inventory[1000000000]": false,
		"a[99999999999999999999]": false, "inventory.0": false, "inventory.0.kind": false,
		"a.+1": false, "a.-1": false, "[0]": false, "inventory[-1]": false,
		// C-02 v1.8b p. 2: a key written as a decimal integer is refused at any
		// length, including where strconv.Atoi overflows and where the size of
		// int decides (2147483648 on a 32-bit platform); an exponent is not a
		// decimal integer (condition И2-1 of system-architect).
		"a.99999999999999999999": false, "a.-99999999999999999999": false, "a.+99999999999999999999": false,
		"a.2147483648": false, "a.1e3": true,
	}
	if len(spellings) != 29 {
		t.Fatalf("%d spellings, want the 23 of T-056, the empty path and the 5 of И2-1", len(spellings))
	}
	for path, want := range spellings {
		if got := entity.CanonicalPath(path); got != want {
			t.Errorf("CanonicalPath(%q) = %v, want %v", path, got, want)
		}
		// An entity of no type has no scalars, so what refuses a path here is
		// the grammar and nothing else.
		e := entity.New(entity.Ref{ID: "x"}, "w", "", map[string]any{"a": map[string]any{}}, proposedAt)
		_, _, err := entity.ApplyOps(e, []entity.Op{{Op: entity.OpSet, Path: path, Value: "v"}})
		var invalid entity.ErrInvalidOp
		refusedByGrammar := errors.As(err, &invalid) &&
			(invalid.Reason == entity.ReasonBadPath || invalid.Reason == entity.ReasonEmptyPath)
		if refusedByGrammar == want {
			t.Errorf("ApplyOps(set %q) = %v, want refused by the grammar = %v", path, err, !want)
		}
	}
}

// probe U-1 of T-056. remove inventory.0 on a list of three used to report the
// element — {path: inventory.0, old: a, new: b} — and a consumer catching up
// wrote b into the slot instead of shifting the list, so its state hash went
// elsewhere. The spelling is refused now; the bracketed one reports the list,
// and catching up lands where State did.
func TestTheRemovalOfAnElementIsSpelledWithBrackets(t *testing.T) {
	three := func() map[string]any {
		return map[string]any{entity.AttrInventory: []any{
			map[string]any{"item_id": "a"}, map[string]any{"item_id": "b"}, map[string]any{"item_id": "c"},
		}}
	}
	e := entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, "w", "", three(), proposedAt)

	if invalid := applyErr(t, e, entity.Op{Op: entity.OpRemove, Path: "inventory.0"}); invalid.Reason != entity.ReasonBadPath {
		t.Fatalf("remove inventory.0: reason = %q, want %q", invalid.Reason, entity.ReasonBadPath)
	}

	attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpRemove, Path: "inventory[0]"})
	if len(changed) != 1 || changed[0].Path != entity.AttrInventory || !changed[0].HasOld || !changed[0].HasNew {
		t.Fatalf("changed = %+v, want one entry on the path of the list with the list before and after", changed)
	}
	wantCaughtUp(t, e, attrs, changed)
}

// C-02 v1.8 p. 2: State publishes canonical paths only, so an entry of changed[]
// in any other spelling is a corrupt fact, the same as an element past the end
// of its list (state_divergence when State meets it catching up).
func TestCatchingUpRefusesAPathNotInItsCanonicalForm(t *testing.T) {
	list := func() map[string]any { return map[string]any{"tags": []any{"wounded", "hunted"}} }
	for _, test := range []struct {
		name    string
		change  entity.Change
		corrupt bool
	}{
		{"the element in brackets", entity.Change{Path: "tags[0]", New: "x", HasNew: true}, false},
		{"the element after a dot", entity.Change{Path: "tags.0", New: "x", HasNew: true}, true},
		{"the element after a dot, without new", entity.Change{Path: "tags.1", Old: "hunted", HasOld: true}, true},
		{"a trailing dot", entity.Change{Path: "tags.", New: []any{}, HasNew: true}, true},
		{"a leading zero", entity.Change{Path: "tags[01]", New: "x", HasNew: true}, true},
		{"a path that is not there, in another spelling", entity.Change{Path: ".fresh", Old: 1, HasOld: true}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "", list(), proposedAt)
			_, err := catchUp(e, []entity.Change{test.change})
			if errors.Is(err, errCorruptFact) != test.corrupt {
				t.Fatalf("catchUp error = %v, want corrupt = %v", err, test.corrupt)
			}
			if !test.corrupt && err != nil {
				t.Fatalf("catchUp: %v", err)
			}
		})
	}
}

// The tables of data-model.md §3.1–§3.4, §3.6, §3.7, row by row, with the common
// name (successor of TestEveryScalarOfTheDataModelIsListed of T-056). Item
// (§3.5) is a value inside inventory[] and no table of its own; scope is not a
// scalar, it can be the object {id, type}.
func TestEveryAttributeOfTheDataModelHasItsKind(t *testing.T) {
	type row struct {
		kind     entity.ValueKind
		required bool
		nullable bool
	}
	var (
		req  = func(k entity.ValueKind) row { return row{kind: k, required: true} }
		opt  = func(k entity.ValueKind) row { return row{kind: k, nullable: true} }
		name = row{kind: entity.KindText}
	)
	model := map[string]map[string]row{
		entity.TypeWorld: { // §3.1
			"name": name, "laws_version": req(entity.KindText), "weather": req(entity.KindEnum),
			"time_of_day": req(entity.KindEnum), "day": req(entity.KindInteger), "season": opt(entity.KindText),
			"epoch": opt(entity.KindText), "blueprint_ref": req(entity.KindText), "locale": req(entity.KindText),
		},
		entity.TypeRegion: { // §3.2
			"name": name, "description": req(entity.KindText), "canon": opt(entity.KindOpen),
			"npc_ids": req(entity.KindOpen), "respawn_ttl": req(entity.KindDuration),
			"perception_radius": opt(entity.KindNumber), "encounter_chance": req(entity.KindNumber),
			"players_present": req(entity.KindOpen), "last_background_event_at": opt(entity.KindTime),
			"blueprint_ref": req(entity.KindText),
		},
		entity.TypePlayer: { // §3.3
			"name": name, "hp": req(entity.KindInteger), "hp_max": req(entity.KindInteger),
			"atk": req(entity.KindModifier), "def": req(entity.KindModifier), "dmg": req(entity.KindModifier),
			"flee":   opt(entity.KindModifier), // null or none is "does not run" (§3.3)
			"status": req(entity.KindEnum), "position": req(entity.KindText), "scope": req(entity.KindOpen),
			"group_id": opt(entity.KindRef), "encounter_id": opt(entity.KindRef), "inventory": req(entity.KindOpen),
			"actor_kind": req(entity.KindEnum), "last_session_ended_at": opt(entity.KindTime),
		},
		entity.TypeNPC: { // §3.4
			"name": name, "kind": req(entity.KindText), "region_id": req(entity.KindRef),
			"hp": req(entity.KindInteger), "hp_max": req(entity.KindInteger), "atk": req(entity.KindModifier),
			"def": req(entity.KindModifier), "dmg": req(entity.KindModifier), "status": req(entity.KindEnum),
			"position": req(entity.KindText), "died_at": opt(entity.KindTime), "killed_by": opt(entity.KindRef),
			"loot": req(entity.KindOpen), "loot_claimed_by": opt(entity.KindRef), "spawned_by": opt(entity.KindOpen),
		},
		entity.TypeGroup: { // §3.6
			"name":      name,
			"leader_id": {kind: entity.KindRef, required: true, nullable: true},
			"members":   req(entity.KindOpen), "position": req(entity.KindText), "scope": req(entity.KindOpen),
			"encounter_id": opt(entity.KindRef), "state": req(entity.KindEnum),
		},
		entity.TypeEncounter: { // §3.7
			"name": name, "region_id": req(entity.KindRef), "scope": req(entity.KindOpen),
			"participants": req(entity.KindOpen), "npcs": req(entity.KindOpen), "state": req(entity.KindEnum),
			"resolution": opt(entity.KindEnum), "round_seq": req(entity.KindInteger),
			"task_agent_id": opt(entity.KindRef), "opened_by_event_id": opt(entity.KindRef),
			"closed_by_event_id": opt(entity.KindRef),
		},
	}
	if types := slices.Sorted(maps.Keys(model)); !slices.Equal(types, slices.Sorted(slices.Values(entity.Types))) ||
		!slices.Equal(types, entity.AttributeTypes()) {
		t.Fatalf("the model covers %v, the platform knows %v, the table has %v", types, entity.Types, entity.AttributeTypes())
	}
	for typ, rows := range model {
		// Both directions at once: a row the table holds without the model —
		// whatever its name — is as much a failure as a row it lacks.
		if names, want := entity.AttributeNames(typ), slices.Sorted(maps.Keys(rows)); !slices.Equal(names, want) {
			t.Errorf("%s: the table holds %v, data-model.md §3 names %v", typ, names, want)
		}
		for attr, want := range rows {
			spec, ok := entity.AttributeSpecOf(typ, attr)
			if got := (row{spec.Kind, spec.Required, spec.Nullable}); !ok || got != want {
				t.Errorf("%s.%s = %+v, %v; want %+v", typ, attr, got, ok, want)
			}
		}
		// Every scalar refuses a path below it; a container and the scope
		// take one.
		for attr, r := range rows {
			e := entity.New(entity.Ref{ID: "x", Type: typ}, "w", "", nil, proposedAt)
			_, _, err := entity.ApplyOps(e, []entity.Op{{Op: entity.OpSet, Path: attr + ".x", Value: "v"}})
			var invalid entity.ErrInvalidOp
			below := errors.As(err, &invalid) && invalid.Reason == entity.ReasonBelowScalar
			if below != (r.kind != entity.KindOpen) {
				t.Errorf("%s: set %s.x = %v, want refused as below a scalar = %v", typ, attr, err, r.kind != entity.KindOpen)
			}
		}
	}
	// Nothing outside the tables is typed: not an attribute of another type
	// (died_at of a character), not a field of Item, not a type the platform
	// does not know.
	for _, outside := range [][2]string{
		{entity.TypePlayer, "died_at"}, {entity.TypePlayer, "item_id"}, {entity.TypePlayer, "acquired_at"},
		{entity.TypeNPC, "flee"}, {"city", "hp"}, {"", "status"},
	} {
		if spec, ok := entity.AttributeSpecOf(outside[0], outside[1]); ok {
			t.Errorf("%s.%s is typed as %+v; the model does not type it", outside[0], outside[1], spec)
		}
	}
}

// ApplyOps refuses a value of another kind at the root of a typed attribute once
// the operations have run: probes V1–V3 of review #3 of T-056 (set state {…},
// set hp {x: 999}) and the text and the fraction of hp (C-02 v1.8a p. 3).
// null stands where the model allows it, and an attribute that is gone is not
// a value of another kind.
func TestApplyOpsRefusesAValueOfAnotherKind(t *testing.T) {
	playerAttrs := func() map[string]any { return player(t).Attributes }
	typed := func(typ string, attrs map[string]any) *entity.Entity {
		return entity.New(entity.Ref{ID: "x", Type: typ}, "w", "", attrs, proposedAt)
	}
	for _, test := range []struct {
		name   string
		entity *entity.Entity
		ops    []entity.Op
		want   string
	}{
		{"set state {…} of an encounter", typed(entity.TypeEncounter, map[string]any{"state": "active"}),
			[]entity.Op{{Op: entity.OpSet, Path: "state", Value: map[string]any{"x": 1}}}, entity.ReasonWrongKind},
		{"set hp {x: 999}", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "hp", Value: map[string]any{"x": 999}}}, entity.ReasonWrongKind},
		{"set status.x", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "status.x", Value: "dead"}}, entity.ReasonBelowScalar},
		{"set hp \"10\"", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "hp", Value: "10"}}, entity.ReasonWrongKind},
		{"set hp 5.5", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "hp", Value: 5.5}}, entity.ReasonWrongKind},
		{"set hp null, which is required", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "hp", Value: nil}}, entity.ReasonWrongKind},
		{"set atk 2.5", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "atk", Value: 2.5}}, entity.ReasonWrongKind},
		{"set flee true", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "flee", Value: true}}, entity.ReasonWrongKind},
		{"set day 1.5 of the world", typed(entity.TypeWorld, map[string]any{"day": 1}),
			[]entity.Op{{Op: entity.OpSet, Path: "day", Value: 1.5}}, entity.ReasonWrongKind},
		{"set respawn_ttl soon", typed(entity.TypeRegion, map[string]any{}),
			[]entity.Op{{Op: entity.OpSet, Path: "respawn_ttl", Value: "soon"}}, entity.ReasonWrongKind},
		{"set encounter_chance \"0.25\"", typed(entity.TypeRegion, map[string]any{}),
			[]entity.Op{{Op: entity.OpSet, Path: "encounter_chance", Value: "0.25"}}, entity.ReasonWrongKind},
		{"set died_at yesterday", typed(entity.TypeNPC, map[string]any{}),
			[]entity.Op{{Op: entity.OpSet, Path: "died_at", Value: "yesterday"}}, entity.ReasonWrongKind},
		{"set round_seq \"3\"", typed(entity.TypeEncounter, map[string]any{"round_seq": 2}),
			[]entity.Op{{Op: entity.OpSet, Path: "round_seq", Value: "3"}}, entity.ReasonWrongKind},
		{"a value of another kind put back by a later operation", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "hp", Value: "10"}, {Op: entity.OpSet, Path: "hp", Value: 10}}, ""},
		{"set hp 7.0 off the wire", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "hp", Value: float64(7)}}, ""},
		{"inc hp", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpInc, Path: "hp", Value: -3}}, ""},
		{"remove hp", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpRemove, Path: "hp"}}, ""},
		{"set atk \"+3\"", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "atk", Value: "+3"}}, ""},
		{"set flee null, does not run", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "flee", Value: nil}}, ""},
		{"set encounter_id null, which is optional", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "encounter_id", Value: nil}}, ""},
		{"set leader_id null of a group", typed(entity.TypeGroup, map[string]any{"leader_id": "player-A"}),
			[]entity.Op{{Op: entity.OpSet, Path: "leader_id", Value: nil}}, ""},
		{"set died_at as a time built in Go", typed(entity.TypeNPC, map[string]any{}),
			[]entity.Op{{Op: entity.OpSet, Path: "died_at", Value: proposedAt}}, ""},
		{"set respawn_ttl 24h", typed(entity.TypeRegion, map[string]any{}),
			[]entity.Op{{Op: entity.OpSet, Path: "respawn_ttl", Value: "24h"}}, ""},
		{"set state {…} of an entity of no type", typed("", map[string]any{"state": "active"}),
			[]entity.Op{{Op: entity.OpSet, Path: "state", Value: map[string]any{"x": 1}}}, ""},
		{"a scope object and its id", typed(entity.TypePlayer, playerAttrs()),
			[]entity.Op{{Op: entity.OpSet, Path: "scope", Value: map[string]any{"id": "g-1", "type": "group"}},
				{Op: entity.OpSet, Path: "scope.id", Value: "g-2"}}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := entity.ApplyOps(test.entity, test.ops)
			if test.want == "" {
				if err != nil {
					t.Fatalf("ApplyOps: %v", err)
				}
				return
			}
			var invalid entity.ErrInvalidOp
			if !errors.As(err, &invalid) || invalid.Reason != test.want {
				t.Fatalf("ApplyOps error = %v, want reason %q", err, test.want)
			}
		})
	}
}

// The attributes of a create are checked against the same table, with the
// required attributes of the type (state-and-mechanics.md §4.5; backlog of
// T-056). The fixtures of the dark forest pass as they are.
func TestCheckAttributesOfACreate(t *testing.T) {
	for _, file := range []string{"world.json", "region.json", "npc.json", "players.json"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", file))
		if err != nil {
			t.Fatal(err)
		}
		var entities []entity.Entity
		if err := json.Unmarshal(raw, &entities); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, e := range entities {
			if err := entity.CheckAttributes(e.Type, e.Attributes); err != nil {
				t.Errorf("fixture %s %s: %v", file, e.ID, err)
			}
		}
	}

	character := player(t).Attributes
	without := func(name string) map[string]any {
		out := map[string]any{}
		for k, v := range character {
			if k != name {
				out[k] = v
			}
		}
		return out
	}
	with := func(name string, value any) map[string]any {
		out := without(name)
		out[name] = value
		return out
	}
	for _, test := range []struct {
		name      string
		typ       string
		attrs     map[string]any
		attribute string
		reason    string
	}{
		{"a character as the gateway creates it", entity.TypePlayer, character, "", ""},
		{"without inventory", entity.TypePlayer, without("inventory"), "inventory", entity.ReasonRequiredMissing},
		{"without scope", entity.TypePlayer, without("scope"), "scope", entity.ReasonRequiredMissing},
		{"without flee, a character that does not run", entity.TypePlayer, without("flee"), "", ""},
		{"with a null flee", entity.TypePlayer, with("flee", nil), "", ""},
		{"with hp as a text", entity.TypePlayer, with("hp", "10"), "hp", entity.ReasonWrongKind},
		{"with a null status", entity.TypePlayer, with("status", nil), "status", entity.ReasonWrongKind},
		{"with an object for position", entity.TypePlayer, with("position", map[string]any{"region": "r"}), "position", entity.ReasonWrongKind},
		{"with a name among the attributes", entity.TypePlayer, with("name", "Аня"), "", ""},
		{"with a name that is not a text", entity.TypePlayer, with("name", 7), "name", entity.ReasonWrongKind},
		{"with an attribute the model does not type", entity.TypePlayer, with("banner", map[string]any{}), "", ""},
		{"a group without a leader", entity.TypeGroup, map[string]any{
			"leader_id": nil, "members": []any{}, "position": "r", "scope": "group:g-1", "state": "forming"}, "", ""},
		{"a group without leader_id", entity.TypeGroup, map[string]any{
			"members": []any{}, "position": "r", "scope": "group:g-1", "state": "forming"}, "leader_id", entity.ReasonRequiredMissing},
		{"an NPC without flee", entity.TypeNPC, map[string]any{
			"kind": "wolf", "region_id": "dark-forest-01", "hp": 10, "hp_max": 10, "atk": 3, "def": 11, "dmg": "d4",
			"status": "alive", "position": "dark-forest-01", "loot": []any{}}, "", ""},
		{"an entity of a type the table does not have", "city", map[string]any{}, "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := entity.CheckAttributes(test.typ, test.attrs)
			if test.reason == "" {
				if err != nil {
					t.Fatalf("CheckAttributes: %v", err)
				}
				return
			}
			var invalid entity.ErrInvalidAttribute
			if !errors.As(err, &invalid) || invalid.Attribute != test.attribute || invalid.Reason != test.reason {
				t.Fatalf("CheckAttributes = %v, want %s: %s", err, test.attribute, test.reason)
			}
			if invalid.Error() == "" {
				t.Fatal("Error() is empty")
			}
		})
	}
}

// Of two problems — hp as a text and a null status on an otherwise complete NPC —
// the one first in the order of the names is reported, whatever the order of
// the map.
func TestCheckAttributesReportsTheFirstNameInOrder(t *testing.T) {
	attrs := map[string]any{
		"kind": "wolf", "region_id": "dark-forest-01", "hp": "10", "hp_max": 10, "atk": 3, "def": 11, "dmg": "d4",
		"status": nil, "position": "dark-forest-01", "loot": []any{},
	}
	names := make([]string, 0)
	for range 20 {
		var invalid entity.ErrInvalidAttribute
		if !errors.As(entity.CheckAttributes(entity.TypeNPC, attrs), &invalid) {
			t.Fatal("CheckAttributes: want an error")
		}
		names = append(names, invalid.Attribute)
	}
	sort.Strings(names)
	if names[0] != names[len(names)-1] || names[0] != "hp" {
		t.Fatalf("reported %v, want hp every time", names)
	}
}
