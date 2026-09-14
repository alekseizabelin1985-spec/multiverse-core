package entity_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/jsonpath"
)

var proposedAt = time.Date(2026, 9, 9, 10, 15, 0, 0, time.UTC)

// player is the dark-forest fixture of state-and-mechanics.md §4.10, built in
// Go: hit points, combat stats, an empty inventory and a solo scope.
func player(t *testing.T) *entity.Entity {
	t.Helper()
	return entity.New(
		entity.Ref{ID: "player-A", Type: entity.TypePlayer},
		"dark-forest-world", "Аня",
		map[string]any{
			entity.AttrHP:        float64(10),
			entity.AttrHPMax:     float64(10),
			entity.AttrAtk:       float64(2),
			entity.AttrDef:       float64(12),
			entity.AttrDmg:       "d6",
			entity.AttrFlee:      "+2",
			entity.AttrStatus:    entity.StatusAlive,
			entity.AttrPosition:  "outside:dark-forest-world",
			entity.AttrScope:     "solo:player-A",
			entity.AttrInventory: []any{},
			entity.AttrActorKind: entity.ActorKindCI,
		},
		proposedAt,
	)
}

func applyOK(t *testing.T, e *entity.Entity, ops ...entity.Op) (map[string]any, []entity.Change) {
	t.Helper()
	attrs, changed, err := entity.ApplyOps(e, ops)
	if err != nil {
		t.Fatalf("ApplyOps(%+v): %v", ops, err)
	}
	return attrs, changed
}

func applyErr(t *testing.T, e *entity.Entity, op entity.Op) entity.ErrInvalidOp {
	t.Helper()
	_, _, err := entity.ApplyOps(e, []entity.Op{op})
	var invalid entity.ErrInvalidOp
	if !errors.As(err, &invalid) {
		t.Fatalf("ApplyOps(%+v) error = %v, want ErrInvalidOp", op, err)
	}
	return invalid
}

func TestApplyOpsLeavesTheEntityAlone(t *testing.T) {
	e := player(t)
	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusDead})

	if len(changed) != 1 {
		t.Fatalf("changed = %+v, want one entry", changed)
	}
	if status, _ := e.Status(); status != entity.StatusAlive {
		t.Fatalf("entity status = %q, want the entity untouched", status)
	}
	if attrs[entity.AttrStatus] != entity.StatusDead {
		t.Fatalf("returned attributes = %+v, want status dead", attrs)
	}
	if e.Version != 1 {
		t.Fatalf("version = %d, want 1: ApplyOps does not commit", e.Version)
	}
}

func TestApplySet(t *testing.T) {
	e := player(t)
	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: "dark-forest-01"},
		entity.Op{Op: entity.OpSet, Path: "banner.colour", Value: "green"})

	if attrs[entity.AttrPosition] != "dark-forest-01" {
		t.Fatalf("position = %v, want dark-forest-01", attrs[entity.AttrPosition])
	}
	nested, ok := jsonpath.New(attrs).GetString("banner.colour")
	if !ok || nested != "green" {
		t.Fatalf("banner.colour = %q, %v; want green: set creates intermediate maps", nested, ok)
	}
	want := []entity.Change{
		{Path: entity.AttrPosition, Old: "outside:dark-forest-world", New: "dark-forest-01", HasOld: true, HasNew: true},
		{Path: "banner.colour", New: "green", HasNew: true},
	}
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("changed = %+v, want %+v", changed, want)
	}
}

// An attribute data-model.md §3 does not type is open: a text there can become
// an object. A typed scalar cannot (C-02 v1.8 p. 2) — see
// TestApplyOpsRefusesAPathBelowAScalar.
func TestApplySetOverAScalarReplacesItWithAMap(t *testing.T) {
	e := player(t)
	e.Attributes["motto"] = "run"
	attrs, _ := applyOK(t, e,
		entity.Op{Op: entity.OpSet, Path: "motto.text", Value: "hide"})

	if _, ok := attrs["motto"].(map[string]any); !ok {
		t.Fatalf("motto = %#v, want a map", attrs["motto"])
	}
}

func TestApplySetAnIndexedPath(t *testing.T) {
	group := entity.New(entity.Ref{ID: "group-1", Type: entity.TypeGroup}, "dark-forest-world", "", map[string]any{
		entity.AttrMembers: []any{
			map[string]any{"player_id": "player-A", "participation": entity.ParticipationActive},
			map[string]any{"player_id": "player-B", "participation": entity.ParticipationActive},
		},
	}, proposedAt)

	attrs, changed := applyOK(t, group, entity.Op{
		Op:    entity.OpSet,
		Path:  "members[1].participation",
		Value: entity.ParticipationIdle,
	})

	got, ok := jsonpath.New(attrs).GetString("members[1].participation")
	if !ok || got != entity.ParticipationIdle {
		t.Fatalf("members[1].participation = %q, %v; want idle", got, ok)
	}
	if first, _ := jsonpath.New(attrs).GetString("members[0].participation"); first != entity.ParticipationActive {
		t.Fatalf("members[0].participation = %q, want the other member untouched", first)
	}
	if len(changed) != 1 || changed[0].Path != "members[1].participation" {
		t.Fatalf("changed = %+v, want one entry on the element path", changed)
	}
}

func TestApplyInc(t *testing.T) {
	tests := []struct {
		name  string
		start any
		delta any
		want  any
	}{
		{name: "damage", start: float64(10), delta: -4, want: int64(6)},
		{name: "clamped at zero", start: float64(3), delta: -9, want: int64(0)},
		{name: "clamped at hp_max", start: float64(7), delta: 9, want: int64(10)},
		{name: "float delta that is whole", start: float64(10), delta: float64(-2), want: int64(8)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := player(t)
			e.Attributes[entity.AttrHP] = test.start
			attrs, changed := applyOK(t, e,
				entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: test.delta})

			if !reflect.DeepEqual(attrs[entity.AttrHP], test.want) {
				t.Fatalf("hp = %#v, want %#v", attrs[entity.AttrHP], test.want)
			}
			if len(changed) != 1 || !reflect.DeepEqual(changed[0].Old, test.start) {
				t.Fatalf("changed = %+v, want one entry with old %v", changed, test.start)
			}
		})
	}
}

func TestApplyIncOnAMissingPathStartsAtZero(t *testing.T) {
	e := player(t)
	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpInc, Path: "kills", Value: 1})

	if !reflect.DeepEqual(attrs["kills"], int64(1)) {
		t.Fatalf("kills = %#v, want 1", attrs["kills"])
	}
	if len(changed) != 1 || changed[0].HasOld || !changed[0].HasNew {
		t.Fatalf("changed = %+v, want one entry with no old and a new", changed)
	}
}

func TestApplyIncWithoutAnHPMaxOnlyClampsAtZero(t *testing.T) {
	e := player(t)
	delete(e.Attributes, entity.AttrHPMax)
	attrs, _ := applyOK(t, e, entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: 90})

	if !reflect.DeepEqual(attrs[entity.AttrHP], int64(100)) {
		t.Fatalf("hp = %#v, want 100 when there is no hp_max to clamp against", attrs[entity.AttrHP])
	}
}

func TestApplyAppend(t *testing.T) {
	e := player(t)
	pelt := map[string]any{"item_id": "item-1", "kind": "wolf-pelt", "name": "волчья шкура"}

	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt})

	list, _ := jsonpath.New(attrs).GetSlice(entity.AttrInventory)
	if len(list) != 1 {
		t.Fatalf("inventory = %+v, want one item", list)
	}
	if len(changed) != 1 || changed[0].Path != "inventory[0]" {
		t.Fatalf("changed = %+v, want the path of the element (C-02 v1.1)", changed)
	}
	if changed[0].HasOld || !changed[0].HasNew {
		t.Fatalf("changed[0] = %+v, want no old: the element did not exist (C-02 v1.6)", changed[0])
	}
}

func TestApplyAppendIsIdempotentPerItemID(t *testing.T) {
	e := player(t)
	pelt := map[string]any{"item_id": "item-1", "kind": "wolf-pelt"}
	same := map[string]any{"item_id": "item-1", "kind": "something else entirely"}

	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt},
		entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: same})

	list, _ := jsonpath.New(attrs).GetSlice(entity.AttrInventory)
	if len(list) != 1 {
		t.Fatalf("inventory = %+v, want the repeat to be a no-op (inv-03)", list)
	}
	if len(changed) != 1 {
		t.Fatalf("changed = %+v, want one entry", changed)
	}
}

func TestApplyAppendCreatesTheList(t *testing.T) {
	e := player(t)
	delete(e.Attributes, entity.AttrInventory)

	attrs, _ := applyOK(t, e,
		entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: "rope"})

	list, ok := jsonpath.New(attrs).GetSlice(entity.AttrInventory)
	if !ok || len(list) != 1 || list[0] != "rope" {
		t.Fatalf("inventory = %+v, %v; want a list with one element", list, ok)
	}
}

func TestApplyAppendOfAScalarDoesNotDeduplicate(t *testing.T) {
	e := player(t)
	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpAppend, Path: "tags", Value: "wounded"},
		entity.Op{Op: entity.OpAppend, Path: "tags", Value: "wounded"})

	list, _ := jsonpath.New(attrs).GetSlice("tags")
	if len(list) != 2 {
		t.Fatalf("tags = %+v, want both appends: only objects with an id are deduplicated", list)
	}
	if len(changed) != 2 {
		t.Fatalf("changed = %+v, want two element paths", changed)
	}
}

func TestApplyRemove(t *testing.T) {
	e := player(t)
	e.Attributes[entity.AttrInventory] = []any{
		map[string]any{"item_id": "item-1", "kind": "wolf-pelt"},
		map[string]any{"item_id": "item-2", "kind": "rope"},
	}

	attrs, changed := applyOK(t, e, entity.Op{
		Op:    entity.OpRemove,
		Path:  entity.AttrInventory,
		Value: map[string]any{"item_id": "item-1"},
	})

	list, _ := jsonpath.New(attrs).GetSlice(entity.AttrInventory)
	if len(list) != 1 {
		t.Fatalf("inventory = %+v, want the item removed by item_id", list)
	}
	if len(changed) != 1 || changed[0].Path != entity.AttrInventory {
		t.Fatalf("changed = %+v, want one entry on the list", changed)
	}
	if old, ok := changed[0].Old.([]any); !ok || len(old) != 2 {
		t.Fatalf("changed[0].Old = %#v, want the list as it was", changed[0].Old)
	}
}

func TestApplyRemoveOfAScalarElement(t *testing.T) {
	e := player(t)
	e.Attributes["tags"] = []any{"wounded", "hunted"}

	attrs, _ := applyOK(t, e,
		entity.Op{Op: entity.OpRemove, Path: "tags", Value: "wounded"})

	list, _ := jsonpath.New(attrs).GetSlice("tags")
	if len(list) != 1 || list[0] != "hunted" {
		t.Fatalf("tags = %+v, want only hunted left", list)
	}
}

func TestApplyRemoveWithoutAValueDropsTheKey(t *testing.T) {
	e := player(t)
	e.Attributes[entity.AttrEncounterID] = "enc-1"
	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpRemove, Path: entity.AttrEncounterID})
	if _, present := attrs[entity.AttrEncounterID]; present {
		t.Fatalf("attributes = %+v, want encounter_id gone", attrs)
	}
	if len(changed) != 1 || changed[0].Old != "enc-1" || !changed[0].HasOld || changed[0].HasNew {
		t.Fatalf("changed = %+v, want old enc-1 and no new (C-02 v1.6)", changed)
	}
}

func TestApplyRemoveOfAnAbsentElementIsANoOp(t *testing.T) {
	e := player(t)
	e.Attributes["tags"] = []any{"hunted"}

	_, changed := applyOK(t, e,
		entity.Op{Op: entity.OpRemove, Path: "tags", Value: "wounded"})

	if len(changed) != 0 {
		t.Fatalf("changed = %+v, want none", changed)
	}
}

func TestApplyOpsDropsNoOps(t *testing.T) {
	e := player(t)

	// A rest at full health, UC-011 E2: the turn counted, nothing changed.
	_, changed := applyOK(t, e,
		entity.Op{Op: entity.OpSet, Path: entity.AttrHP, Value: float64(10)})
	if len(changed) != 0 {
		t.Fatalf("changed = %+v, want none: the value is the one already there", changed)
	}

	// There and back again inside one proposal is also nothing.
	_, changed = applyOK(t, e,
		entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -3},
		entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: 3})
	if len(changed) != 0 {
		t.Fatalf("changed = %+v, want none: hp ended where it started", changed)
	}
}

func TestApplyOpsReportsTheFirstOldAndTheLastNew(t *testing.T) {
	e := player(t)
	_, changed := applyOK(t, e,
		entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -4},
		entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -3})

	if len(changed) != 1 {
		t.Fatalf("changed = %+v, want one entry for the one path", changed)
	}
	if !reflect.DeepEqual(changed[0].Old, float64(10)) || !reflect.DeepEqual(changed[0].New, int64(3)) {
		t.Fatalf("changed[0] = %+v, want 10 -> 3", changed[0])
	}
}

func TestApplyOpsOnAnEmptyList(t *testing.T) {
	e := player(t)
	attrs, changed, err := entity.ApplyOps(e, nil)
	if err != nil {
		t.Fatalf("ApplyOps(nil): %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("changed = %+v, want none", changed)
	}
	if attrs == nil {
		t.Fatal("attributes = nil, want a copy of the entity's own")
	}
	if status, _ := jsonpath.New(attrs).GetString(entity.AttrStatus); status != entity.StatusAlive {
		t.Fatalf("status = %q, want the attributes copied through", status)
	}
}

func TestApplyOpsChangedIsNeverNull(t *testing.T) {
	// entity.updated requires changed to be an array; a nil slice would encode
	// as null and fail the schema.
	_, changed, err := entity.ApplyOps(player(t), nil)
	if err != nil {
		t.Fatalf("ApplyOps: %v", err)
	}
	if changed == nil {
		t.Fatal("changed = nil, want an empty slice")
	}
}

func TestApplyOpsRejectsReservedPaths(t *testing.T) {
	e := player(t)
	reserved := append([]string{}, entity.ReservedPaths...)
	reserved = append(reserved, "_intent", "history[0].version")

	for _, path := range reserved {
		t.Run(path, func(t *testing.T) {
			invalid := applyErr(t, e, entity.Op{Op: entity.OpSet, Path: path, Value: "x"})
			if invalid.Reason != entity.ReasonReservedPath {
				t.Fatalf("reason = %q, want %q", invalid.Reason, entity.ReasonReservedPath)
			}
		})
	}
}

func TestApplyOpsMalformed(t *testing.T) {
	e := player(t)
	tests := []struct {
		name string
		op   entity.Op
		want string
	}{
		{"unknown verb", entity.Op{Op: "replace", Path: entity.AttrHP, Value: 1}, entity.ReasonUnknownOp},
		{"empty path", entity.Op{Op: entity.OpSet, Path: "", Value: 1}, entity.ReasonEmptyPath},
		{"unterminated bracket", entity.Op{Op: entity.OpSet, Path: "members[0", Value: 1}, entity.ReasonBadPath},
		{"inc of a string", entity.Op{Op: entity.OpInc, Path: entity.AttrDmg, Value: 1}, entity.ReasonNotNumber},
		{"inc by a fraction", entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: 1.5}, entity.ReasonNotInteger},
		{"inc by a string", entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: "1"}, entity.ReasonNotInteger},
		{"append to a scalar", entity.Op{Op: entity.OpAppend, Path: entity.AttrHP, Value: 1}, entity.ReasonNotList},
		{"remove from a scalar", entity.Op{Op: entity.OpRemove, Path: entity.AttrHP, Value: 1}, entity.ReasonNotList},
		{"remove of nothing", entity.Op{Op: entity.OpRemove, Path: "quiver"}, entity.ReasonMissingPath},
		{"set of a channel", entity.Op{Op: entity.OpSet, Path: "chan", Value: make(chan int)}, entity.ReasonNotJSON},
		{"set of an infinity", entity.Op{Op: entity.OpSet, Path: "inf", Value: math.Inf(1)}, entity.ReasonNotJSON},
		{"set past the end of a list", entity.Op{Op: entity.OpSet, Path: "inventory[3]", Value: 1}, entity.ReasonBadIndex},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invalid := applyErr(t, e, test.op)
			if invalid.Reason != test.want {
				t.Fatalf("reason = %q, want %q", invalid.Reason, test.want)
			}
			if invalid.Op.Path != test.op.Path {
				t.Fatalf("ErrInvalidOp.Op = %+v, want the operation that failed", invalid.Op)
			}
			if invalid.Error() == "" {
				t.Fatal("Error() is empty")
			}
		})
	}
}

func TestApplyOpsStopsAtTheFirstBadOperation(t *testing.T) {
	e := player(t)
	attrs, changed, err := entity.ApplyOps(e, []entity.Op{
		{Op: entity.OpSet, Path: entity.AttrPosition, Value: "dark-forest-01"},
		{Op: entity.OpInc, Path: entity.AttrStatus, Value: 1},
	})
	if err == nil {
		t.Fatal("ApplyOps: want an error")
	}
	if attrs != nil || changed != nil {
		t.Fatalf("ApplyOps = %+v, %+v; want nothing alongside the error", attrs, changed)
	}
}

// The write half of the path grammar lives in this package and the read half in
// shared/jsonpath. This pins them together: whatever an op writes, jsonpath
// finds at the same path.
//
// Agreement is necessary and not sufficient. The read half has the same blind
// spot as the write half used to have — jsonpath.navigate resolves [0] on an
// object as the plain key "0" — so two halves that are wrong in the same way
// agree with each other and this test stays green. That is why every indexed
// case here also asserts the type of the container the op left behind, which no
// amount of agreement can fake. Fixing the read half is EPIC-002; until then a
// green run of this test says the halves match, not that either is right.
func TestPathGrammarMatchesJSONPath(t *testing.T) {
	cases := []struct {
		path string
		// list is the container that has to still be a list afterwards.
		list string
	}{
		{path: "colour"},
		{path: "banner.colour"},
		{path: "deep.nested.value"},
		{path: "members[0]", list: "members"},
		{path: "members[1].participation", list: "members"},
		{path: "a.b[0].c", list: "a.b"},
	}
	base := map[string]any{
		"members": []any{map[string]any{}, map[string]any{}},
		"a":       map[string]any{"b": []any{map[string]any{}}},
	}
	for _, test := range cases {
		t.Run(test.path, func(t *testing.T) {
			e := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "", base, proposedAt)
			attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpSet, Path: test.path, Value: "written"})

			got, ok := jsonpath.New(attrs).GetString(test.path)
			if !ok || got != "written" {
				t.Fatalf("jsonpath reads %q, %v at %q; want what the op wrote", got, ok, test.path)
			}
			if len(changed) != 1 || changed[0].Path != test.path {
				t.Fatalf("changed = %+v, want one entry at %q", changed, test.path)
			}
			if test.list == "" {
				return
			}
			container, _ := jsonpath.New(attrs).GetAny(test.list)
			if _, isList := container.([]any); !isList {
				t.Fatalf("%s = %#v, want a list: an index must not turn one into an object", test.list, container)
			}
		})
	}
}

// value carries omitempty, which for an interface means "omit only nil". A
// false or a zero is a value like any other and has to reach the wire.
func TestOpKeepsAFalseAndAZeroValue(t *testing.T) {
	encoded, err := json.Marshal([]entity.Op{
		{Op: entity.OpSet, Path: "active", Value: false},
		{Op: entity.OpSet, Path: "missed_rounds", Value: 0},
		{Op: entity.OpRemove, Path: "encounter_id"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `[{"op":"set","path":"active","value":false},` +
		`{"op":"set","path":"missed_rounds","value":0},` +
		`{"op":"remove","path":"encounter_id"}]`
	if string(encoded) != want {
		t.Fatalf("ops encode as %s, want %s", encoded, want)
	}
}

func TestProposeBuildsAChangeSet(t *testing.T) {
	e := player(t)
	ops := []entity.Op{{Op: entity.OpInc, Path: entity.AttrHP, Value: -4}}

	pinned := e.Propose(ops, true)
	if pinned.ExpectedVersion == nil || *pinned.ExpectedVersion != e.Version {
		t.Fatalf("expected_version = %v, want %d", pinned.ExpectedVersion, e.Version)
	}
	if pinned.Ref() != e.Ref() {
		t.Fatalf("Ref = %v, want %v", pinned.Ref(), e.Ref())
	}
	if pinned.Entity.Name != "Аня" {
		t.Fatalf("name = %q, want the display name of the entity", pinned.Entity.Name)
	}

	loose := e.Propose(ops, false)
	if loose.ExpectedVersion != nil {
		t.Fatalf("expected_version = %v, want none", *loose.ExpectedVersion)
	}
}

// M-1. Three ways to change the attributes that all used to report changed: [].
// The value at the path reads as null both before and after in each of them,
// and only the existence of the path says that something happened — while the
// state hash says it loudly. C-02 ties the version to a non-empty changed[], so
// a change that hides here is a version that never moves and an --audit that
// recomputes a hash the journal cannot explain.
func TestApplyOpsSeesTheDifferenceBetweenAbsentAndNull(t *testing.T) {
	tests := []struct {
		name  string
		attrs map[string]any
		op    entity.Op
		path  string
	}{
		{
			name:  "append a null element",
			attrs: map[string]any{"tags": []any{"wounded"}},
			op:    entity.Op{Op: entity.OpAppend, Path: "tags"},
			path:  "tags[1]",
		},
		{
			name:  "set an absent key to null",
			attrs: map[string]any{entity.AttrHP: float64(10)},
			op:    entity.Op{Op: entity.OpSet, Path: entity.AttrDiedAt},
			path:  entity.AttrDiedAt,
		},
		{
			name:  "remove a key that holds null",
			attrs: map[string]any{entity.AttrHP: float64(10), entity.AttrKilledBy: nil},
			op:    entity.Op{Op: entity.OpRemove, Path: entity.AttrKilledBy},
			path:  entity.AttrKilledBy,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := entity.New(entity.Ref{ID: "x", Type: entity.TypeNPC}, "w", "", test.attrs, proposedAt)
			before := entity.StateHash([]*entity.Entity{e})

			attrs, changed := applyOK(t, e, test.op)
			if len(changed) != 1 || changed[0].Path != test.path {
				t.Fatalf("changed = %+v, want one entry at %q", changed, test.path)
			}

			after := entity.Clone(e)
			after.Attributes = attrs
			if entity.StateHash([]*entity.Entity{after}) == before {
				t.Fatal("the state hash did not move, so this test proves nothing")
			}
		})
	}
}

// The guard for the whole class M-1 belongs to: a non-empty changed[] and a
// moved state hash are the same event, in both directions. C-02 states it as
// one sentence — version grows by one exactly when changed[] is not empty — and
// this is that sentence made runnable. The version is held fixed on purpose:
// what is under test is the attributes alone.
//
// The attributes here are all wire-shaped ([]any, float64), which is what the
// bus and the fixtures deliver. A list built in Go as []string is out of scope
// until the read half of the grammar can see into one (EPIC-002).
func TestChangedListAndStateHashMoveTogether(t *testing.T) {
	pelt := func() map[string]any { return map[string]any{"item_id": "item-1", "kind": "wolf-pelt"} }
	rope := func() map[string]any { return map[string]any{"item_id": "item-2", "kind": "rope"} }

	tests := []struct {
		name  string
		attrs map[string]any
		ops   []entity.Op
	}{
		{
			name:  "damage",
			attrs: map[string]any{entity.AttrHP: float64(10), entity.AttrHPMax: float64(10)},
			ops:   []entity.Op{{Op: entity.OpInc, Path: entity.AttrHP, Value: -4}},
		},
		{
			name:  "a rest at full health",
			attrs: map[string]any{entity.AttrHP: float64(10), entity.AttrHPMax: float64(10)},
			ops:   []entity.Op{{Op: entity.OpInc, Path: entity.AttrHP, Value: 5}},
		},
		{
			name:  "there and back inside one proposal",
			attrs: map[string]any{entity.AttrHP: float64(10), entity.AttrHPMax: float64(10)},
			ops: []entity.Op{
				{Op: entity.OpInc, Path: entity.AttrHP, Value: -3},
				{Op: entity.OpInc, Path: entity.AttrHP, Value: 3},
			},
		},
		{
			name:  "a value written over itself",
			attrs: map[string]any{entity.AttrStatus: entity.StatusAlive},
			ops:   []entity.Op{{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusAlive}},
		},
		{
			name:  "an intermediate map created on the way",
			attrs: map[string]any{},
			ops:   []entity.Op{{Op: entity.OpSet, Path: "banner.colour", Value: "green"}},
		},
		{
			name:  "an untyped scalar replaced by a map and put back",
			attrs: map[string]any{"motto": "run"},
			ops: []entity.Op{
				{Op: entity.OpSet, Path: "motto.text", Value: "hide"},
				{Op: entity.OpSet, Path: "motto", Value: "run"},
			},
		},
		{
			name:  "append a null element",
			attrs: map[string]any{"tags": []any{"wounded"}},
			ops:   []entity.Op{{Op: entity.OpAppend, Path: "tags"}},
		},
		{
			name:  "set an absent key to null",
			attrs: map[string]any{entity.AttrHP: float64(10)},
			ops:   []entity.Op{{Op: entity.OpSet, Path: entity.AttrDiedAt}},
		},
		{
			name:  "set null over null",
			attrs: map[string]any{entity.AttrDiedAt: nil},
			ops:   []entity.Op{{Op: entity.OpSet, Path: entity.AttrDiedAt}},
		},
		{
			name:  "remove a key that holds null",
			attrs: map[string]any{entity.AttrHP: float64(10), entity.AttrKilledBy: nil},
			ops:   []entity.Op{{Op: entity.OpRemove, Path: entity.AttrKilledBy}},
		},
		{
			name:  "append an item",
			attrs: map[string]any{entity.AttrInventory: []any{}},
			ops:   []entity.Op{{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt()}},
		},
		{
			name:  "append the same item twice",
			attrs: map[string]any{entity.AttrInventory: []any{pelt()}},
			ops:   []entity.Op{{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt()}},
		},
		{
			name:  "append and remove the same item",
			attrs: map[string]any{entity.AttrInventory: []any{}},
			ops: []entity.Op{
				{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt()},
				{Op: entity.OpRemove, Path: entity.AttrInventory, Value: map[string]any{"item_id": "item-1"}},
			},
		},
		{
			name:  "remove an item by id",
			attrs: map[string]any{entity.AttrInventory: []any{pelt(), rope()}},
			ops: []entity.Op{
				{Op: entity.OpRemove, Path: entity.AttrInventory, Value: map[string]any{"item_id": "item-1"}},
			},
		},
		{
			name:  "remove an item that is not there",
			attrs: map[string]any{entity.AttrInventory: []any{pelt()}},
			ops: []entity.Op{
				{Op: entity.OpRemove, Path: entity.AttrInventory, Value: map[string]any{"item_id": "item-9"}},
			},
		},
		{
			name:  "remove an element by index",
			attrs: map[string]any{entity.AttrInventory: []any{pelt(), rope()}},
			ops:   []entity.Op{{Op: entity.OpRemove, Path: "inventory[0]"}},
		},
		{
			name:  "remove a key",
			attrs: map[string]any{entity.AttrEncounterID: "enc-1"},
			ops:   []entity.Op{{Op: entity.OpRemove, Path: entity.AttrEncounterID}},
		},
		{
			name:  "no operations at all",
			attrs: map[string]any{entity.AttrHP: float64(10)},
			ops:   nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "", test.attrs, proposedAt)
			before := entity.StateHash([]*entity.Entity{e})

			attrs, changed := applyOK(t, e, test.ops...)
			after := entity.Clone(e)
			after.Attributes = attrs

			hashAfter := entity.StateHash([]*entity.Entity{after})
			hashMoved := hashAfter != before
			if hashMoved != (len(changed) > 0) {
				t.Fatalf("state hash moved = %v, changed = %+v; want the two to agree", hashMoved, changed)
			}
			if replayChanged(t, e, changed) != hashAfter {
				t.Fatalf("catching up on changed = %+v does not reproduce the state (C-02 v1.6)", changed)
			}
		})
	}
}

// M-2. An index has to land on a list. members[0] where there is no members
// used to write the object {"members":{"0":…}} — quietly, and past the very
// check the write half of the grammar exists for.
func TestApplySetRefusesAnIndexIntoAnObject(t *testing.T) {
	e := player(t)
	e.Attributes["banner"] = map[string]any{"colour": "green"}

	for _, path := range []string{"members[0]", "banner[0]", "banner[0].colour", "members[0].participation"} {
		t.Run(path, func(t *testing.T) {
			invalid := applyErr(t, e, entity.Op{Op: entity.OpSet, Path: path, Value: "x"})
			if invalid.Reason != entity.ReasonIndexOnPath {
				t.Fatalf("reason = %q, want %q", invalid.Reason, entity.ReasonIndexOnPath)
			}
		})
	}

	// And the entity is where it was: a refused op writes nothing.
	if _, present := e.Attributes["members"]; present {
		t.Fatalf("attributes = %+v, want members never to have been created", e.Attributes)
	}
	if _, isMap := e.Attributes["banner"].(map[string]any); !isMap {
		t.Fatalf("banner = %#v, want the object untouched", e.Attributes["banner"])
	}
}

// M-2, the other half: a list built in Go rather than read off the wire is
// still a list, and writing into an element of it must not swallow the rest.
// The assertion is on the type of the container, not on reading the same path
// back — reading it back is exactly what hid the defect.
func TestApplySetIntoAListBuiltInGoKeepsTheList(t *testing.T) {
	e := player(t)
	e.Attributes["tags"] = []string{"wounded", "hunted"}

	attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpSet, Path: "tags[0]", Value: "healed"})

	list, isList := attrs["tags"].([]any)
	if !isList {
		t.Fatalf("tags = %#v, want a list, not an object with the key \"0\"", attrs["tags"])
	}
	if len(list) != 2 || list[0] != "healed" || list[1] != "hunted" {
		t.Fatalf("tags = %#v, want [healed hunted]", list)
	}
	// The old value reads as absent because the read half of the grammar cannot
	// see into a []string (EPIC-002); the change itself is reported either way.
	if len(changed) != 1 || changed[0].Path != "tags[0]" || changed[0].New != "healed" {
		t.Fatalf("changed = %+v, want one entry at tags[0]", changed)
	}
}

// M-3. Removing an element by index shifts the list, so the change is reported
// on the list. Anything else and a read-model that applies changed[] literally
// — the way C-02 and §3.3 describe building one — writes a null into the slot
// the element left and drifts away from State.
func TestApplyRemoveByIndexReportsTheList(t *testing.T) {
	e := player(t)
	e.Attributes[entity.AttrInventory] = []any{
		map[string]any{"item_id": "item-1", "kind": "wolf-pelt"},
		map[string]any{"item_id": "item-2", "kind": "rope"},
	}

	attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpRemove, Path: "inventory[0]"})

	if len(changed) != 1 || changed[0].Path != entity.AttrInventory {
		t.Fatalf("changed = %+v, want one entry on the path of the list", changed)
	}
	if old, ok := changed[0].Old.([]any); !ok || len(old) != 2 {
		t.Fatalf("changed[0].Old = %#v, want the list as it was", changed[0].Old)
	}
	if updated, ok := changed[0].New.([]any); !ok || len(updated) != 1 {
		t.Fatalf("changed[0].New = %#v, want the list as it is", changed[0].New)
	}

	// The fact is honest when a consumer that takes it literally lands on the
	// same state hash as State did.
	applied := entity.Clone(e)
	applied.Attributes = attrs
	if replayChanged(t, e, changed) != entity.StateHash([]*entity.Entity{applied}) {
		t.Fatal("replaying changed[] literally does not reproduce the state")
	}
}

// replayChanged is the read-model of C-02 v1.6, the rule State catches up by
// (state-and-mechanics.md §4.8). changed[] first travels over the wire, so that
// what is replayed is what a consumer reads, and a fact the rule calls corrupt
// fails the test instead of being written somehow.
func replayChanged(t *testing.T, base *entity.Entity, changed []entity.Change) string {
	t.Helper()
	hash, err := catchUp(base, overTheWire(t, changed))
	if err != nil {
		t.Fatalf("catching up on %+v: %v", changed, err)
	}
	return hash
}

// errCorruptFact is an entry of changed[] that no fact of State carries: State
// meeting one while catching up stops the world with state_divergence (§4.8).
var errCorruptFact = errors.New("corrupt fact")

// catchUp applies changed[] entry by entry. A path not in its canonical form is
// a corrupt fact (C-02 v1.8 p. 2). new present is written at its path,
// replacing a missing or scalar node on the way with an object, as set does,
// and appended when the path is the element one past the end of its list; new
// absent deletes the path. The version is carried over so that the hash answers for the
// attributes alone.
//
// The append of catching up is a plain append, not the op: the fact already
// carries the outcome of deduplication, and an element that shares an item_id
// with another one — a set after the append made it so — is still an element.
// And the delete of catching up deletes what is there: an earlier entry may
// already have written the container without the path (inventory[0] = {}
// before inventory[0].kind with no new), and that is the state asked for.
func catchUp(base *entity.Entity, changed []entity.Change) (string, error) {
	out := entity.Clone(base)
	for _, change := range changed {
		op, err := catchUpOp(out.Attributes, change)
		if err != nil {
			return "", err
		}
		if op == nil {
			continue
		}
		attrs, _, err := entity.ApplyOps(out, []entity.Op{*op})
		if err != nil {
			return "", fmt.Errorf("replaying %+v: %w", change, err)
		}
		out.Attributes = attrs
	}
	return entity.StateHash([]*entity.Entity{out}), nil
}

// catchUpOp is the operation one entry of changed[] comes down to, or nil when
// there is nothing to do.
func catchUpOp(attrs map[string]any, change entity.Change) (*entity.Op, error) {
	if !entity.CanonicalPath(change.Path) {
		return nil, fmt.Errorf("%w: %q is not a canonical path", errCorruptFact, change.Path)
	}
	if !change.HasNew {
		if _, exists := jsonpath.New(attrs).GetAny(change.Path); !exists {
			return nil, nil
		}
		return &entity.Op{Op: entity.OpRemove, Path: change.Path}, nil
	}
	set := &entity.Op{Op: entity.OpSet, Path: change.Path, Value: change.New}
	parent, n, isElement := elementPath(change.Path)
	if !isElement {
		return set, nil
	}
	current, _ := jsonpath.New(attrs).GetAny(parent)
	var list []any
	switch held := current.(type) {
	case nil:
		// A list that is not there and a null where it would be are the same
		// absent list: the append that creates a list takes either.
	case []any:
		list = held
	default:
		// An index into something that is not a list: set refuses it.
		return set, nil
	}
	switch {
	case n < len(list):
		return set, nil
	case n == len(list):
		return &entity.Op{Op: entity.OpSet, Path: parent, Value: append(slices.Clone(list), change.New)}, nil
	default:
		return nil, fmt.Errorf("%w: %s past the end of a list of %d", errCorruptFact, change.Path, len(list))
	}
}

// elementPath answers whether path is an element a[n] and names a and n.
func elementPath(path string) (string, int, bool) {
	open := strings.LastIndexByte(path, '[')
	if open <= 0 || !strings.HasSuffix(path, "]") {
		return "", 0, false
	}
	n, err := strconv.Atoi(path[open+1 : len(path)-1])
	if err != nil {
		return "", 0, false
	}
	return path[:open], n, true
}

// overTheWire is changed[] as a consumer of entity.updated holds it: encoded by
// the publisher and decoded on the other side.
func overTheWire(t *testing.T, changed []entity.Change) []entity.Change {
	t.Helper()
	encoded, err := json.Marshal(changed)
	if err != nil {
		t.Fatalf("marshal changed: %v", err)
	}
	var decoded []entity.Change
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal changed %s: %v", encoded, err)
	}
	return decoded
}

// Mi-1. An append undone by a remove in the same proposal is not a change: the
// state came back to where it started, so there is nothing to report and
// nothing to move the version for (§3.2, "no-op is excluded").
func TestApplyAppendThenRemoveOfTheSameItemIsANoOp(t *testing.T) {
	e := player(t)
	pelt := map[string]any{"item_id": "item-1", "kind": "wolf-pelt"}

	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt},
		entity.Op{Op: entity.OpRemove, Path: entity.AttrInventory, Value: map[string]any{"item_id": "item-1"}})

	if len(changed) != 0 {
		t.Fatalf("changed = %+v, want none: the item was added and taken back", changed)
	}
	list, _ := jsonpath.New(attrs).GetSlice(entity.AttrInventory)
	if len(list) != 0 {
		t.Fatalf("inventory = %+v, want it empty", list)
	}

	e.Commit(attrs, changed, entity.LastChange{ProposalID: "p-1", AppliedAt: proposedAt})
	if e.Version != 1 {
		t.Fatalf("version = %d, want it to stay at 1", e.Version)
	}
}

// pelt is the trophy of state-and-mechanics.md §3.2 as a proposer builds it in
// Go: a struct, not the map the same item becomes on the wire.
func pelt() entity.Item {
	return entity.Item{
		ItemID: "dec-7:wolf-pelt",
		Kind:   "wolf-pelt",
		Name:   "волчья шкура",
		Source: entity.ItemSource{
			Entity:  eventbus.EntityRef{ID: "wolf-alpha", Type: entity.TypeNPC},
			EventID: "dec-7",
		},
		AcquiredAt: proposedAt,
	}
}

// T-050. append deduplicates by item_id (§3.2, inv-03), and it did so only for
// a map: the entity.Item a proposer appends in Go was compared with
// reflect.DeepEqual, and a redelivered decision handed the trophy out twice.
// The rule is about the item, not about the Go type that happens to carry it,
// so the repeat is a no-op whether the item already in the list came from the
// same Go code or off the wire.
func TestApplyAppendOfAnItemBuiltInGoIsIdempotent(t *testing.T) {
	t.Run("the same struct twice", func(t *testing.T) {
		e := player(t)
		attrs, changed := applyOK(t, e,
			entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt()},
			entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt()})

		if list, _ := jsonpath.New(attrs).GetSlice(entity.AttrInventory); len(list) != 1 {
			t.Fatalf("inventory = %+v, want one trophy (inv-03)", list)
		}
		if len(changed) != 1 {
			t.Fatalf("changed = %+v, want one entry", changed)
		}
	})

	t.Run("a struct over the same item read off the wire", func(t *testing.T) {
		e := player(t)
		e.Attributes[entity.AttrInventory] = []any{wireForm(t, pelt())}

		_, changed := applyOK(t, e,
			entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt()})
		if len(changed) != 0 {
			t.Fatalf("changed = %+v, want the repeat to be a no-op", changed)
		}
	})

	t.Run("a different item is still appended", func(t *testing.T) {
		e := player(t)
		other := pelt()
		other.ItemID = "dec-8:wolf-pelt"
		attrs, changed := applyOK(t, e,
			entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt()},
			entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: other})
		if list, _ := jsonpath.New(attrs).GetSlice(entity.AttrInventory); len(list) != 2 || len(changed) != 2 {
			t.Fatalf("inventory = %+v, changed = %+v; want two trophies of two decisions", list, changed)
		}
	})
}

// wireForm is what a value becomes after a trip through encoding/json: the
// shape every consumer of the bus and every reader of a snapshot holds.
func wireForm(t *testing.T, v any) any {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal %T: %v", v, err)
	}
	return decoded
}

// T-050. remove compares the way append does: an item built in Go takes out
// the same item read off the wire, and a number built in Go takes out the same
// number decoded as float64. Before, both were silently nothing — a remove that
// changed nothing and said so with an empty changed[].
func TestApplyRemoveComparesTheWireForm(t *testing.T) {
	t.Run("an item built in Go", func(t *testing.T) {
		e := player(t)
		e.Attributes[entity.AttrInventory] = []any{
			map[string]any{"item_id": "dec-7:wolf-pelt", "kind": "wolf-pelt"},
			map[string]any{"item_id": "item-2", "kind": "rope"},
		}
		attrs, changed := applyOK(t, e,
			entity.Op{Op: entity.OpRemove, Path: entity.AttrInventory, Value: pelt()})
		if list, _ := jsonpath.New(attrs).GetSlice(entity.AttrInventory); len(list) != 1 || len(changed) != 1 {
			t.Fatalf("inventory = %+v, changed = %+v; want the pelt taken out by item_id", list, changed)
		}
	})

	t.Run("a number built in Go", func(t *testing.T) {
		e := player(t)
		e.Attributes["marked_rounds"] = []any{float64(3), float64(5)}
		attrs, changed := applyOK(t, e,
			entity.Op{Op: entity.OpRemove, Path: "marked_rounds", Value: 3})
		list, _ := jsonpath.New(attrs).GetSlice("marked_rounds")
		if len(list) != 1 || len(changed) != 1 {
			t.Fatalf("list = %+v, changed = %+v; want 3 taken out of [3 5]", list, changed)
		}
	})

	t.Run("an object without an id is compared whole", func(t *testing.T) {
		e := player(t)
		e.Attributes["marks"] = []any{
			map[string]any{"by": "player-A", "at": float64(2)},
			map[string]any{"by": "player-B", "at": float64(2)},
		}
		attrs, _ := applyOK(t, e,
			entity.Op{Op: entity.OpRemove, Path: "marks", Value: map[string]any{"by": "player-A", "at": 2}})
		if list, _ := jsonpath.New(attrs).GetSlice("marks"); len(list) != 1 {
			t.Fatalf("marks = %+v, want only the mark of player-B left", list)
		}
		_, changed := applyOK(t, e,
			entity.Op{Op: entity.OpRemove, Path: "marks", Value: map[string]any{"by": "player-A"}})
		if len(changed) != 0 {
			t.Fatalf("changed = %+v, want nothing: a part of an object is not the object", changed)
		}
	})
}

// T-050. inc takes a whole number in every shape one reaches ApplyOps in: the
// integer widths of Go code, the float64 of encoding/json and the json.Number
// of a decoder with UseNumber. A fraction in any of them is not a whole number.
func TestApplyIncAcceptsEveryWholeNumber(t *testing.T) {
	tests := []struct {
		delta any
		want  int64
	}{
		{int8(-1), 4}, {int16(-1), 4}, {int32(-1), 4}, {int64(-1), 4}, {int(-1), 4},
		{uint(1), 6}, {uint8(1), 6}, {uint16(1), 6}, {uint32(1), 6}, {uint64(1), 6},
		{float32(-1), 4}, {float64(-1), 4}, {json.Number("-1"), 4},
	}
	for _, test := range tests {
		t.Run(reflect.TypeOf(test.delta).String(), func(t *testing.T) {
			e := player(t)
			e.Attributes["kills"] = float64(5)
			attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpInc, Path: "kills", Value: test.delta})
			if !reflect.DeepEqual(attrs["kills"], test.want) {
				t.Fatalf("kills = %#v, want %d", attrs["kills"], test.want)
			}
			if len(changed) != 1 {
				t.Fatalf("changed = %+v, want one entry", changed)
			}
		})
	}

	// Iteration 2: a number past the range of int64 is not a whole number of
	// int64 either. Converted, it wrapped into another number — the largest
	// uint64 into -1 — and an inc healed or hurt by an amount nobody proposed.
	notWhole := []any{
		float32(0.5), json.Number("1.5"), json.Number("many"),
		uint64(math.MaxUint64), uint64(math.MaxInt64) + 1, uint(math.MaxUint64),
		float64(1 << 63), -float64(1<<63) * 2, json.Number("9223372036854775808"),
	}
	for _, fraction := range notWhole {
		t.Run("not whole "+fmt.Sprint(fraction), func(t *testing.T) {
			invalid := applyErr(t, player(t), entity.Op{Op: entity.OpInc, Path: "kills", Value: fraction})
			if invalid.Reason != entity.ReasonNotInteger {
				t.Fatalf("reason = %q, want %q", invalid.Reason, entity.ReasonNotInteger)
			}
		})
	}
}

// T-050. remove without a value inside an element of a list: the key goes, the
// list keeps its length, and the change is reported where the key was — no
// element shifted, so there is no list to report.
func TestApplyRemoveOfAKeyInsideAListElement(t *testing.T) {
	e := player(t)
	e.Attributes[entity.AttrParticipants] = []any{
		map[string]any{"player_id": "player-A", "last_hit_at": "2026-09-09T10:15:00Z"},
	}
	attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpRemove, Path: "participants[0].last_hit_at"})

	list, _ := jsonpath.New(attrs).GetSlice(entity.AttrParticipants)
	if len(list) != 1 {
		t.Fatalf("participants = %+v, want the element kept", list)
	}
	if element, ok := list[0].(map[string]any); !ok || len(element) != 1 {
		t.Fatalf("participants[0] = %#v, want only player_id left", list[0])
	}
	if len(changed) != 1 || changed[0].Path != "participants[0].last_hit_at" || changed[0].HasNew {
		t.Fatalf("changed = %+v, want the removed key with no new", changed)
	}

	applied := entity.Clone(e)
	applied.Attributes = attrs
	if replayChanged(t, e, changed) != entity.StateHash([]*entity.Entity{applied}) {
		t.Fatal("replaying changed[] literally does not reproduce the state")
	}

	invalid := applyErr(t, e, entity.Op{Op: entity.OpRemove, Path: "participants[3].last_hit_at"})
	if invalid.Reason != entity.ReasonMissingPath {
		t.Fatalf("reason = %q, want %q for an element that is not there", invalid.Reason, entity.ReasonMissingPath)
	}
}
