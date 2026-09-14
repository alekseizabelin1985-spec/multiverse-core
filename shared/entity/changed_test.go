package entity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"multiverse-core.io/shared/entity"
)

// T-448. The form of changed[] of C-02 v1.6, row by row: old is present exactly
// when the path existed before, new exactly when it exists after. Each row is
// checked three ways — the flags ApplyOps sets, the keys the wire carries, and
// the property the form exists for: a consumer that catches up by the rule of
// state-and-mechanics.md §4.8 lands on the state hash ApplyOps produced.
func TestChangedFormPerOperation(t *testing.T) {
	pelt := func() map[string]any { return map[string]any{"item_id": "item-1", "kind": "wolf-pelt"} }
	rope := func() map[string]any { return map[string]any{"item_id": "item-2", "kind": "rope"} }

	rows := []struct {
		name  string
		attrs map[string]any
		ops   []entity.Op
		// path is the path of the one entry the row produces; old and new say
		// which keys it carries.
		path     string
		old, new bool
	}{
		{
			name:  "set of an existing path",
			attrs: map[string]any{entity.AttrPosition: "outside:w"},
			ops:   []entity.Op{{Op: entity.OpSet, Path: entity.AttrPosition, Value: "dark-forest-01"}},
			path:  entity.AttrPosition, old: true, new: true,
		},
		{
			name:  "set of an existing path to null",
			attrs: map[string]any{entity.AttrKilledBy: "player-A"},
			ops:   []entity.Op{{Op: entity.OpSet, Path: entity.AttrKilledBy}},
			path:  entity.AttrKilledBy, old: true, new: true,
		},
		{
			name:  "set of an absent path",
			attrs: map[string]any{},
			ops:   []entity.Op{{Op: entity.OpSet, Path: "banner.colour", Value: "green"}},
			path:  "banner.colour", old: false, new: true,
		},
		{
			name:  "set of an absent path to null",
			attrs: map[string]any{},
			ops:   []entity.Op{{Op: entity.OpSet, Path: entity.AttrDiedAt}},
			path:  entity.AttrDiedAt, old: false, new: true,
		},
		{
			name:  "inc of an existing path",
			attrs: map[string]any{entity.AttrHP: float64(10), entity.AttrHPMax: float64(10)},
			ops:   []entity.Op{{Op: entity.OpInc, Path: entity.AttrHP, Value: -4}},
			path:  entity.AttrHP, old: true, new: true,
		},
		{
			name:  "inc of an absent path",
			attrs: map[string]any{},
			ops:   []entity.Op{{Op: entity.OpInc, Path: "kills", Value: 1}},
			path:  "kills", old: false, new: true,
		},
		{
			name:  "append to a list",
			attrs: map[string]any{entity.AttrInventory: []any{rope()}},
			ops:   []entity.Op{{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt()}},
			path:  "inventory[1]", old: false, new: true,
		},
		{
			name:  "append that creates the list",
			attrs: map[string]any{},
			ops:   []entity.Op{{Op: entity.OpAppend, Path: "tags", Value: "wounded"}},
			path:  "tags[0]", old: false, new: true,
		},
		{
			name:  "append of a null element",
			attrs: map[string]any{"tags": []any{"wounded"}},
			ops:   []entity.Op{{Op: entity.OpAppend, Path: "tags"}},
			path:  "tags[1]", old: false, new: true,
		},
		{
			name:  "remove of a key",
			attrs: map[string]any{entity.AttrEncounterID: "enc-1"},
			ops:   []entity.Op{{Op: entity.OpRemove, Path: entity.AttrEncounterID}},
			path:  entity.AttrEncounterID, old: true, new: false,
		},
		{
			name:  "remove of a key that holds null",
			attrs: map[string]any{entity.AttrKilledBy: nil},
			ops:   []entity.Op{{Op: entity.OpRemove, Path: entity.AttrKilledBy}},
			path:  entity.AttrKilledBy, old: true, new: false,
		},
		{
			name:  "remove of a key inside a list element",
			attrs: map[string]any{entity.AttrParticipants: []any{map[string]any{"player_id": "p", "last_hit_at": "t"}}},
			ops:   []entity.Op{{Op: entity.OpRemove, Path: "participants[0].last_hit_at"}},
			path:  "participants[0].last_hit_at", old: true, new: false,
		},
		{
			name:  "remove with a value",
			attrs: map[string]any{entity.AttrInventory: []any{pelt(), rope()}},
			ops:   []entity.Op{{Op: entity.OpRemove, Path: entity.AttrInventory, Value: map[string]any{"item_id": "item-1"}}},
			path:  entity.AttrInventory, old: true, new: true,
		},
		{
			name:  "remove of the last element with a value",
			attrs: map[string]any{entity.AttrInventory: []any{pelt()}},
			ops:   []entity.Op{{Op: entity.OpRemove, Path: entity.AttrInventory, Value: map[string]any{"item_id": "item-1"}}},
			path:  entity.AttrInventory, old: true, new: true,
		},
		{
			name:  "remove of an element by index",
			attrs: map[string]any{entity.AttrInventory: []any{pelt(), rope()}},
			ops:   []entity.Op{{Op: entity.OpRemove, Path: "inventory[0]"}},
			path:  entity.AttrInventory, old: true, new: true,
		},
	}
	updated := schemaOf(t, "entity.updated.v1.json")
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			e := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "", row.attrs, proposedAt)
			attrs, changed := applyOK(t, e, row.ops...)

			if len(changed) != 1 || changed[0].Path != row.path {
				t.Fatalf("changed = %+v, want one entry at %q", changed, row.path)
			}
			c := changed[0]
			if c.HasOld != row.old || c.HasNew != row.new {
				t.Fatalf("changed[0] = %+v, want old present = %v, new present = %v", c, row.old, row.new)
			}

			encoded, err := json.Marshal(c)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var keys map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &keys); err != nil {
				t.Fatalf("unmarshal %s: %v", encoded, err)
			}
			if _, has := keys["old"]; has != row.old {
				t.Fatalf("wire form %s: old key present = %v, want %v", encoded, has, row.old)
			}
			if _, has := keys["new"]; has != row.new {
				t.Fatalf("wire form %s: new key present = %v, want %v", encoded, has, row.new)
			}
			validateFact(t, updated, changed)

			applied := entity.Clone(e)
			applied.Attributes = attrs
			if replayChanged(t, e, changed) != entity.StateHash([]*entity.Entity{applied}) {
				t.Fatalf("catching up on %s does not reproduce the state", encoded)
			}
		})
	}
}

// validateFact checks changed[] inside a whole entity.updated payload.
func validateFact(t *testing.T, schema *jsonschema.Schema, changed []entity.Change) {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"entity":      map[string]any{"entity": map[string]any{"id": "x", "type": entity.TypePlayer}},
		"version":     2,
		"changed":     changed,
		"cause":       "combat",
		"proposal_id": "p-1",
		"applied_at":  "2026-09-09T10:15:00Z",
	})
	if err != nil {
		t.Fatalf("marshal fact: %v", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode fact: %v", err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("entity.updated does not accept\n%s\n%v", encoded, err)
	}
}

// The schema side of the form: an entry needs a path and at least one of old
// and new, and nothing else. A remove of a key is {path, old}; an entry with
// neither says nothing a consumer could apply.
func TestUpdatedSchemaHoldsTheChangedForm(t *testing.T) {
	schema := schemaOf(t, "entity.updated.v1.json")
	fact := func(entry string) string {
		return `{"entity": {"entity": {"id": "x", "type": "player"}}, "version": 2,
			"changed": [` + entry + `], "cause": "combat", "proposal_id": "p-1",
			"applied_at": "2026-09-09T10:15:00Z"}`
	}
	cases := []struct {
		entry string
		valid bool
	}{
		{`{"path": "hp", "old": 10, "new": 6}`, true},
		{`{"path": "inventory[0]", "new": {"item_id": "i-1"}}`, true},
		{`{"path": "encounter_id", "old": "enc-1"}`, true},
		{`{"path": "died_at", "new": null}`, true},
		{`{"path": "killed_by", "old": null}`, true},
		{`{"path": "hp"}`, false},
		{`{"old": 10, "new": 6}`, false},
		{`{"path": "hp", "old": 10, "new": 6, "op": "set"}`, false},
	}
	for _, test := range cases {
		t.Run(test.entry, func(t *testing.T) {
			doc, err := jsonschema.UnmarshalJSON(strings.NewReader(fact(test.entry)))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			err = schema.Validate(doc)
			if (err == nil) != test.valid {
				t.Fatalf("valid = %v, want %v (%v)", err == nil, test.valid, err)
			}
		})
	}
}

// proposal_id is required in entity.create.proposed as in an update
// (C-02 v1.6): it is what State drops a repeat of the proposal by.
func TestCreateProposalSchemaRequiresTheProposalID(t *testing.T) {
	schema := schemaOf(t, "entity.create.proposed.v1.json")
	const body = `"entity": {"entity": {"id": "wolf-alpha", "type": "npc"}}, "attributes": {"hp": 10}, "cause": "init"`
	for _, test := range []struct {
		payload string
		valid   bool
	}{
		{`{"proposal_id": "bootstrap:w:npc/wolf-alpha", ` + body + `}`, true},
		{`{` + body + `}`, false},
		{`{"proposal_id": "", ` + body + `}`, false},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(test.payload))
		if err != nil {
			t.Fatalf("decode %s: %v", test.payload, err)
		}
		if err := schema.Validate(doc); (err == nil) != test.valid {
			t.Errorf("%s: valid = %v, want %v (%v)", test.payload, err == nil, test.valid, err)
		}
	}
}

// The fact answers with the proposal_id of the proposal it came from, and
// since C-02 v1.6 it has to: every create carries one, and a consumer matches
// its proposal to the fact by it.
func TestCreatedSchemaRequiresTheProposalID(t *testing.T) {
	schema := schemaOf(t, "entity.created.v1.json")
	const body = `"entity": {"entity": {"id": "wolf-alpha", "type": "npc"}}, "version": 1, "attributes": {"hp": 10}`
	for _, test := range []struct {
		payload string
		valid   bool
	}{
		{`{"proposal_id": "bootstrap:w:npc/wolf-alpha", ` + body + `}`, true},
		{`{` + body + `}`, false},
		{`{"proposal_id": "", ` + body + `}`, false},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(test.payload))
		if err != nil {
			t.Fatalf("decode %s: %v", test.payload, err)
		}
		if err := schema.Validate(doc); (err == nil) != test.valid {
			t.Errorf("%s: valid = %v, want %v (%v)", test.payload, err == nil, test.valid, err)
		}
	}
}

// The Go side of the wire: presence survives a round trip, a null stays a
// present null, and a Change whose flags contradict its values is refused at
// the publisher rather than written in a form nobody meant.
func TestChangeJSONRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		change entity.Change
		wire   string
	}{
		{"both", entity.Change{Path: "hp", Old: 10.0, New: 6.0, HasOld: true, HasNew: true}, `{"path":"hp","old":10,"new":6}`},
		{"append", entity.Change{Path: "tags[0]", New: "w", HasNew: true}, `{"path":"tags[0]","new":"w"}`},
		{"remove of a key", entity.Change{Path: "encounter_id", Old: "enc-1", HasOld: true}, `{"path":"encounter_id","old":"enc-1"}`},
		{"set to null", entity.Change{Path: "died_at", HasNew: true}, `{"path":"died_at","new":null}`},
		{"remove of a null", entity.Change{Path: "killed_by", HasOld: true}, `{"path":"killed_by","old":null}`},
		{"both null", entity.Change{Path: "died_at", HasOld: true, HasNew: true}, `{"path":"died_at","old":null,"new":null}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.change)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(encoded) != test.wire {
				t.Fatalf("wire = %s, want %s", encoded, test.wire)
			}
			var back entity.Change
			if err := json.Unmarshal(encoded, &back); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(back, test.change) {
				t.Fatalf("round trip = %+v, want %+v", back, test.change)
			}
		})
	}

	t.Run("flags that contradict the values", func(t *testing.T) {
		for _, test := range []struct {
			name   string
			change entity.Change
			want   string
		}{
			{"neither flag, a null removed", entity.Change{Path: "killed_by"}, "neither old nor new"},
			{"neither flag, values set", entity.Change{Path: "hp", Old: 10, New: 6}, "neither old nor new"},
			{"an old without its flag", entity.Change{Path: "hp", Old: 10, New: 6, HasNew: true}, "HasOld is false"},
			{"a new without its flag", entity.Change{Path: "hp", Old: 10, New: 6, HasOld: true}, "HasNew is false"},
		} {
			t.Run(test.name, func(t *testing.T) {
				encoded, err := json.Marshal(test.change)
				if err == nil {
					t.Fatalf("marshal = %s, want an error", encoded)
				}
				if !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), test.change.Path) {
					t.Fatalf("error %q, want it to name %q and the path %q", err, test.want, test.change.Path)
				}
			})
		}
	})

	t.Run("a null in place of the object", func(t *testing.T) {
		back := entity.Change{Path: "hp", Old: 10.0, HasOld: true}
		if err := json.Unmarshal([]byte(` null `), &back); err != nil {
			t.Fatalf("unmarshal null: %v", err)
		}
		if want := (entity.Change{Path: "hp", Old: 10.0, HasOld: true}); !reflect.DeepEqual(back, want) {
			t.Fatalf("after null = %+v, want the change left as it was", back)
		}
	})

	t.Run("a value that cannot be encoded", func(t *testing.T) {
		if _, err := json.Marshal(entity.Change{Path: "inf", New: math.Inf(1)}); err == nil {
			t.Fatal("marshal of an infinity: want an error")
		}
	})

	t.Run("a malformed object", func(t *testing.T) {
		var back entity.Change
		if err := json.Unmarshal([]byte(`{"path": 3}`), &back); err == nil {
			t.Fatal("unmarshal of a numeric path: want an error")
		}
		if err := json.Unmarshal([]byte(`[]`), &back); err == nil {
			t.Fatal("unmarshal of a list: want an error")
		}
	})

	t.Run("the commit record keeps presence", func(t *testing.T) {
		// Nulls on both sides, so that only the flags can say what is present.
		e := player(t)
		e.Attributes[entity.AttrKilledBy] = nil
		attrs, changed := applyOK(t, e,
			entity.Op{Op: entity.OpRemove, Path: entity.AttrKilledBy},
			entity.Op{Op: entity.OpSet, Path: entity.AttrDiedAt})
		e.Commit(attrs, changed, entity.LastChange{ProposalID: "p-1", AppliedAt: proposedAt})
		clone := entity.Clone(e)
		if !reflect.DeepEqual(clone.LastChange.Changed, changed) {
			t.Fatalf("cloned changes = %+v, want %+v", clone.LastChange.Changed, changed)
		}
		if got := clone.LastChange.Changed; len(got) != 2 || !got[0].HasOld || got[0].HasNew ||
			got[1].HasOld || !got[1].HasNew {
			t.Fatalf("cloned changes = %+v, want a present null old, then a present null new", got)
		}
	})
}

// The property behind the form, over sequences nobody wrote down: whatever a
// proposal of up to four operations does to an entity, the changed[] it
// reports — encoded, decoded and applied by the rule of catching up — lands on
// the state hash ApplyOps produced, and is empty exactly when that hash did not
// move. The generator is seeded, so a failure is the same failure on every run.
func TestCatchingUpOnChangedReproducesTheState(t *testing.T) {
	// The paths mix what the grammar and the table of kinds allow with what
	// they refuse: inventory.0 and tags.1 are second spellings of elements
	// (probe U-1 of T-056), dmg.formula and status.x go below a scalar of a
	// character, and hp and status take values of every kind.
	//
	// refused is the oracle for the paths, written out by hand rather than
	// asked of CanonicalPath or of the table: a check by the function under
	// test is blind to that function breaking. The lists hold two elements,
	// because on a list of one remove inventory.0 lands on the same hash under
	// either rule of catching up and probe U-1 cannot show.
	refused := map[string]bool{"inventory.0": true, "tags.1": true, "status.x": true, "dmg.formula": true}
	paths := []string{
		entity.AttrHP, entity.AttrStatus, entity.AttrDiedAt, "kills", "banner", "banner.colour",
		"tags", "tags[0]", "tags[1]", entity.AttrInventory, "inventory[0]", "inventory[0].kind",
		"inventory[1]", "fresh", "fresh.deep", "fresh.list", entity.AttrDmg, "dmg.formula", "banner.colour.shade",
		"inventory.0", "tags.1", "status.x",
	}
	// Values are decoded afresh for every operation: ApplyOps stores the value
	// it was given, and a map shared between runs would be edited by the
	// operations of a later one.
	values := []string{
		`null`, `"x"`, `3`, `[]`, `["a", null]`, `{}`, `{"colour": "red"}`,
		`{"item_id": "item-1"}`, `{"item_id": "item-2", "kind": "rope"}`, `{"item_id": "item-3"}`,
	}
	kinds := []entity.OpKind{entity.OpSet, entity.OpInc, entity.OpAppend, entity.OpRemove}
	rng := rand.New(rand.NewPCG(448, 53))
	pick := func(n int) int { return rng.IntN(n) }
	value := func() any {
		i := pick(len(values) + 1)
		if i == len(values) {
			return 7 // an int built in Go, which the wire turns into a float64
		}
		var decoded any
		if err := json.Unmarshal([]byte(values[i]), &decoded); err != nil {
			t.Fatalf("decode %s: %v", values[i], err)
		}
		return decoded
	}

	applied, empty, elementsRemoved := 0, 0, 0
	for run := range 8000 {
		base := map[string]any{
			entity.AttrHP:        float64(6),
			entity.AttrHPMax:     float64(10),
			entity.AttrStatus:    entity.StatusAlive,
			entity.AttrDiedAt:    nil,
			entity.AttrDmg:       "d6",
			"tags":               []any{"wounded", "hunted"},
			entity.AttrInventory: []any{map[string]any{"item_id": "item-1", "kind": "wolf-pelt"}, map[string]any{"item_id": "item-4", "kind": "rope"}},
			"banner":             map[string]any{"colour": "green"},
		}
		ops := make([]entity.Op, 1+pick(4))
		for i := range ops {
			op := entity.Op{Op: kinds[pick(len(kinds))], Path: paths[pick(len(paths))]}
			switch {
			case op.Op == entity.OpInc:
				op.Value = pick(7) - 3
			case op.Op == entity.OpRemove && pick(2) == 0:
			default:
				op.Value = value()
			}
			ops[i] = op
		}

		e := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "", base, proposedAt)
		before := entity.StateHash([]*entity.Entity{e})
		attrs, changed, err := entity.ApplyOps(e, ops)
		if err != nil {
			continue
		}
		for _, op := range ops {
			if refused[op.Path] {
				t.Fatalf("run %d: ops %+v applied with %q, which C-02 v1.8 p. 2 refuses", run, ops, op.Path)
			}
			if op.Op == entity.OpRemove && op.Value == nil && strings.HasSuffix(op.Path, "]") {
				elementsRemoved++
			}
		}
		applied++
		after := entity.Clone(e)
		after.Attributes = attrs
		want := entity.StateHash([]*entity.Entity{after})

		label := fmt.Sprintf("run %d, ops %+v, changed %+v", run, ops, changed)
		if (want != before) != (len(changed) > 0) {
			t.Fatalf("%s: state hash moved = %v", label, want != before)
		}
		if len(changed) == 0 {
			empty++
		}
		for _, c := range changed {
			if !c.HasOld && !c.HasNew {
				t.Fatalf("%s: an entry with neither old nor new", label)
			}
		}
		if got := replayChanged(t, e, changed); got != want {
			t.Fatalf("%s: catching up gives %s, want %s", label, got, want)
		}
	}
	// A generator that only produces refused proposals, or only no-ops, would
	// make the property vacuous.
	if applied < 1000 || empty == applied || elementsRemoved < 50 {
		t.Fatalf("applied %d proposals, %d of them empty, %d removals of an element: the generator does not exercise the property",
			applied, empty, elementsRemoved)
	}
}

// T-448, C-02 v1.6. A number of magnitude 2^53 or more anywhere in a value is
// not JSON-compatible: every reader of the bus decodes it into a float64, which
// may hold a different number, and State answers invalid_op instead of storing a
// value its own fact cannot carry. The largest magnitude allowed is 2^53-1: 2^53
// itself is refused because 2^53+1 decodes into it.
func TestApplyOpsRefusesNumbersPastTwoToTheFiftyThird(t *testing.T) {
	const (
		limit = int64(1) << 53 // the first magnitude refused
		safe  = limit - 1      // the last magnitude allowed
	)
	type big struct {
		Seed int64 `json:"seed"`
	}
	refused := []struct {
		name string
		op   entity.Op
	}{
		{"set of 2^53", entity.Op{Op: entity.OpSet, Path: "seed", Value: limit}},
		{"set of -2^53", entity.Op{Op: entity.OpSet, Path: "seed", Value: -limit}},
		{"set of a float64 2^53", entity.Op{Op: entity.OpSet, Path: "seed", Value: float64(limit)}},
		{"set of a json.Number 2^53", entity.Op{Op: entity.OpSet, Path: "seed", Value: json.Number("9007199254740992")}},
		{"set of 2^53+1", entity.Op{Op: entity.OpSet, Path: "seed", Value: limit + 1}},
		{"set of -(2^53+1)", entity.Op{Op: entity.OpSet, Path: "seed", Value: -limit - 1}},
		{"set of a uint64", entity.Op{Op: entity.OpSet, Path: "seed", Value: uint64(math.MaxUint64)}},
		{"set of a float64 past the range", entity.Op{Op: entity.OpSet, Path: "seed", Value: float64(1 << 60)}},
		{"set of a huge float64", entity.Op{Op: entity.OpSet, Path: "seed", Value: -1e300}},
		{"set of a json.Number", entity.Op{Op: entity.OpSet, Path: "seed", Value: json.Number("9007199254740993")}},
		{"set of a json.Number that rounds to 2^53", entity.Op{Op: entity.OpSet, Path: "seed", Value: json.Number("9007199254740993.0")}},
		{"set of a json.Number past int64", entity.Op{Op: entity.OpSet, Path: "seed", Value: json.Number("18446744073709551617")}},
		{"set of a json.Number in exponent form", entity.Op{Op: entity.OpSet, Path: "seed", Value: json.Number("1e16")}},
		{"set of a nested number", entity.Op{Op: entity.OpSet, Path: "rolls", Value: map[string]any{"list": []any{1, limit}}}},
		{"set of a struct field", entity.Op{Op: entity.OpSet, Path: "roll", Value: big{Seed: limit}}},
		{"append of a number", entity.Op{Op: entity.OpAppend, Path: "tags", Value: limit}},
		{"append of an object", entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: map[string]any{"item_id": "i", "weight": limit}}},
		{"inc by 2^53", entity.Op{Op: entity.OpInc, Path: "kills", Value: limit}},
		{"inc by a json.Number past the range", entity.Op{Op: entity.OpInc, Path: "kills", Value: json.Number("-9007199254740993")}},
	}
	for _, test := range refused {
		t.Run(test.name, func(t *testing.T) {
			invalid := applyErr(t, player(t), test.op)
			if invalid.Reason != entity.ReasonNotJSON {
				t.Fatalf("reason = %q, want %q", invalid.Reason, entity.ReasonNotJSON)
			}
		})
	}

	t.Run("a literal past the range that went through JSON", func(t *testing.T) {
		// What State meets on the bus: the proposal is decoded before it is
		// applied, and 2^53+1 is a float64 2^53 by then.
		var op entity.Op
		if err := json.Unmarshal([]byte(`{"op": "set", "path": "seed", "value": 9007199254740993}`), &op); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if op.Value != float64(limit) {
			t.Fatalf("decoded value = %v, want the float64 2^53 the wire makes of it", op.Value)
		}
		if invalid := applyErr(t, player(t), op); invalid.Reason != entity.ReasonNotJSON {
			t.Fatalf("reason = %q, want %q", invalid.Reason, entity.ReasonNotJSON)
		}
	})

	for _, test := range []struct {
		name  string
		kills float64
		delta int64
	}{
		{"inc that lands on 2^53", float64(safe), 1},
		{"inc that lands on -2^53", -float64(safe), -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := player(t)
			e.Attributes["kills"] = test.kills
			invalid := applyErr(t, e, entity.Op{Op: entity.OpInc, Path: "kills", Value: test.delta})
			if invalid.Reason != entity.ReasonNotJSON {
				t.Fatalf("reason = %q, want %q", invalid.Reason, entity.ReasonNotJSON)
			}
		})
	}

	t.Run("inc by a delta past the range that would land inside it", func(t *testing.T) {
		// -(2^53-1) + 2^53 = 1: the result is a fine number, the delta is not
		// one a proposal could have carried over the wire.
		e := player(t)
		e.Attributes["kills"] = -float64(safe)
		invalid := applyErr(t, e, entity.Op{Op: entity.OpInc, Path: "kills", Value: limit})
		if invalid.Reason != entity.ReasonNotJSON {
			t.Fatalf("reason = %q, want %q", invalid.Reason, entity.ReasonNotJSON)
		}
	})

	for _, test := range []struct {
		name    string
		current any
	}{
		// Without the check of the current value, MaxInt64 + 1 would wrap to a
		// negative number and the clamp of hp would pull it back to a valid 0.
		{"an int64 at MaxInt64", int64(math.MaxInt64)},
		{"an int64 at 2^53", limit},
		{"a float64 at 2^53", float64(limit)},
		{"a float64 past int64", 1e300},
		{"a uint64 past int64", uint64(math.MaxUint64)},
	} {
		t.Run("inc of hit points already past the range: "+test.name, func(t *testing.T) {
			// The attribute is what is wrong, not the operation, and the reason
			// says so instead of blaming the value.
			e := player(t)
			e.Attributes[entity.AttrHP] = test.current
			delete(e.Attributes, entity.AttrHPMax)
			invalid := applyErr(t, e, entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: 1})
			if invalid.Reason != entity.ReasonCurrentRange {
				t.Fatalf("reason = %q, want %q", invalid.Reason, entity.ReasonCurrentRange)
			}
		})
	}

	t.Run("inc of a current value that is not a whole number", func(t *testing.T) {
		e := player(t)
		e.Attributes["kills"] = 2.5
		invalid := applyErr(t, e, entity.Op{Op: entity.OpInc, Path: "kills", Value: 1})
		if invalid.Reason != entity.ReasonNotNumber {
			t.Fatalf("reason = %q, want %q", invalid.Reason, entity.ReasonNotNumber)
		}
	})

	accepted := []struct {
		name string
		op   entity.Op
	}{
		{"set of 2^53-1", entity.Op{Op: entity.OpSet, Path: "seed", Value: safe}},
		{"set of -(2^53-1)", entity.Op{Op: entity.OpSet, Path: "seed", Value: -safe}},
		{"set of a float64 2^53-1", entity.Op{Op: entity.OpSet, Path: "seed", Value: float64(safe)}},
		{"set of a json.Number 2^53-1", entity.Op{Op: entity.OpSet, Path: "seed", Value: json.Number("9007199254740991")}},
		{"set of a nested 2^53-1", entity.Op{Op: entity.OpSet, Path: "rolls", Value: map[string]any{"list": []any{1, -safe}}}},
		{"set of a fraction", entity.Op{Op: entity.OpSet, Path: "chance", Value: 0.25}},
		{"set of a json.Number in exponent form", entity.Op{Op: entity.OpSet, Path: "seed", Value: json.Number("1e15")}},
		{"set of a seed as a string", entity.Op{Op: entity.OpSet, Path: "seed", Value: "17918835045097094771"}},
		{"append of 2^53-1", entity.Op{Op: entity.OpAppend, Path: "tags", Value: safe}},
		{"inc by 2^53-1 from zero", entity.Op{Op: entity.OpInc, Path: "kills", Value: safe}},
		{"inc down to -(2^53-1)", entity.Op{Op: entity.OpInc, Path: "kills", Value: -safe}},
	}
	for _, test := range accepted {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := entity.ApplyOps(player(t), []entity.Op{test.op}); err != nil {
				t.Fatalf("ApplyOps(%+v): %v, want it accepted", test.op, err)
			}
		})
	}

	t.Run("JSONCompatible on its own", func(t *testing.T) {
		for _, test := range []struct {
			value any
			want  bool
		}{
			{nil, true},
			{map[string]any{"seed": float64(safe), "list": []any{-safe}}, true},
			{map[string]any{"seed": float64(limit)}, false},
			{[]any{"a", map[string]any{"deep": -limit}}, false},
			{math.NaN(), false},
			{make(chan int), false},
		} {
			if got := entity.JSONCompatible(test.value); got != test.want {
				t.Errorf("JSONCompatible(%#v) = %v, want %v", test.value, got, test.want)
			}
		}
	})
}

// --- the rules of catching up, as named vectors (Mi-1 of review #1 of T-448) ---
//
// The property test found these three; here each is written down, so that a
// generator with another seed or other paths cannot stop covering them without
// a word. They are the vectors a consumer that catches up on facts — State
// (§4.8), a read-model of Swarm or Gateway — has to agree with.

// wantCaughtUp checks that catching up on changed reproduces the attributes
// ApplyOps produced, and that the state hash moved at all.
func wantCaughtUp(t *testing.T, e *entity.Entity, attrs map[string]any, changed []entity.Change) string {
	t.Helper()
	applied := entity.Clone(e)
	applied.Attributes = attrs
	want := entity.StateHash([]*entity.Entity{applied})
	if want == entity.StateHash([]*entity.Entity{e}) {
		t.Fatal("the state hash did not move, so this vector proves nothing")
	}
	if got := replayChanged(t, e, changed); got != want {
		t.Fatalf("catching up on %+v gives %s, want %s", changed, got, want)
	}
	return want
}

// An operation whose path is gone at the end can still leave behind the
// container it made on the way: inc fresh.deep creates fresh, remove fresh.deep
// takes deep out again, and fresh = {} stays. Neither path reports it, so
// changed[] carries the ancestor, after the entries of the paths.
func TestChangedReportsTheAncestorAProposalCreated(t *testing.T) {
	inc := entity.Op{Op: entity.OpInc, Path: "fresh.deep", Value: -3}
	remove := entity.Op{Op: entity.OpRemove, Path: "fresh.deep"}
	for _, test := range []struct {
		name  string
		attrs map[string]any
		ops   []entity.Op
		want  []entity.Change
	}{
		{
			name:  "fresh was not there",
			attrs: map[string]any{entity.AttrHP: float64(10)},
			ops:   []entity.Op{inc, remove},
			want:  []entity.Change{{Path: "fresh", New: map[string]any{}, HasNew: true}},
		},
		{
			name:  "fresh was a scalar",
			attrs: map[string]any{entity.AttrHP: float64(10), "fresh": "s"},
			ops:   []entity.Op{inc, remove},
			want:  []entity.Change{{Path: "fresh", Old: "s", HasOld: true, New: map[string]any{}, HasNew: true}},
		},
		{
			name:  "the ancestor comes after the paths",
			attrs: map[string]any{entity.AttrHP: float64(10)},
			ops:   []entity.Op{{Op: entity.OpSet, Path: "fresh.mark", Value: "fox"}, inc, remove},
			want: []entity.Change{
				{Path: "fresh.mark", New: "fox", HasNew: true},
				{Path: "fresh", New: map[string]any{"mark": "fox"}, HasNew: true},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "", test.attrs, proposedAt)
			attrs, changed := applyOK(t, e, test.ops...)
			if !reflect.DeepEqual(changed, test.want) {
				t.Fatalf("changed = %+v, want %+v", changed, test.want)
			}
			wantCaughtUp(t, e, attrs, changed)
		})
	}
}

// The element a[n] one past the end of its list is appended as it is. An
// append made the element and a set gave it the item_id of another one: the
// list holds two elements with one item_id, and a catch-up that deduplicated
// again, the way the op does, would drop the second and land elsewhere.
func TestCatchingUpAppendsWithoutDeduplication(t *testing.T) {
	pelt := func() map[string]any { return map[string]any{"item_id": "item-1"} }
	e := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "",
		map[string]any{entity.AttrInventory: []any{pelt()}}, proposedAt)

	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: map[string]any{"colour": "red"}},
		entity.Op{Op: entity.OpSet, Path: "inventory[1]", Value: pelt()})

	want := []entity.Change{{Path: "inventory[1]", New: pelt(), HasNew: true}}
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("changed = %+v, want %+v", changed, want)
	}
	hash := wantCaughtUp(t, e, attrs, changed)

	deduplicated := entity.Clone(e)
	deduplicated.Attributes, _ = applyOK(t, deduplicated,
		entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt()})
	if entity.StateHash([]*entity.Entity{deduplicated}) == hash {
		t.Fatal("an append that deduplicates lands on the same state, so this vector proves nothing")
	}
}

// An entry without new deletes its path only when the path is there. An
// earlier entry of the same fact may have written the container already
// without it — here inventory[0] = {} before inventory[0].kind with no new —
// and a catch-up that insisted on deleting would fail on a good fact.
func TestCatchingUpSkipsAPathAnEarlierEntryAlreadyRemoved(t *testing.T) {
	e := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "",
		map[string]any{entity.AttrInventory: []any{map[string]any{"item_id": "item-1", "kind": "wolf-pelt"}}}, proposedAt)

	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpSet, Path: "inventory[0]", Value: "x"},
		entity.Op{Op: entity.OpSet, Path: "inventory[0].kind", Value: 3},
		entity.Op{Op: entity.OpRemove, Path: "inventory[0].kind"})

	want := []entity.Change{
		{Path: "inventory[0]", Old: map[string]any{"item_id": "item-1", "kind": "wolf-pelt"}, HasOld: true, New: map[string]any{}, HasNew: true},
		{Path: "inventory[0].kind", Old: "wolf-pelt", HasOld: true},
	}
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("changed = %+v, want %+v", changed, want)
	}
	wantCaughtUp(t, e, attrs, changed)

	written := entity.Clone(e)
	written.Attributes, _ = applyOK(t, written, entity.Op{Op: entity.OpSet, Path: "inventory[0]", Value: map[string]any{}})
	if invalid := applyErr(t, written, entity.Op{Op: entity.OpRemove, Path: "inventory[0].kind"}); invalid.Reason != entity.ReasonMissingPath {
		t.Fatalf("reason = %q, want %q: the path should be gone after the first entry", invalid.Reason, entity.ReasonMissingPath)
	}
}

// An element a[n] with n past the length of its list is in no fact of State:
// appends are numbered one after another, and anything that shortens a list is
// reported on the path of the list. Catching up calls such a fact corrupt
// instead of writing it somehow; an absent list and a null in its place are the
// same empty list.
func TestCatchingUpRefusesAnElementPastTheEndOfItsList(t *testing.T) {
	for _, test := range []struct {
		name    string
		attrs   map[string]any
		path    string
		corrupt bool
	}{
		{"one past the end", map[string]any{"tags": []any{"wounded"}}, "tags[1]", false},
		{"inside the list", map[string]any{"tags": []any{"wounded"}}, "tags[0]", false},
		{"no list, the first element", map[string]any{}, "tags[0]", false},
		{"null in place of the list, the first element", map[string]any{"tags": nil}, "tags[0]", false},
		{"two past the end", map[string]any{"tags": []any{"wounded"}}, "tags[2]", true},
		{"no list, the second element", map[string]any{}, "tags[1]", true},
		{"null in place of the list, the second element", map[string]any{"tags": nil}, "tags[1]", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := entity.New(entity.Ref{ID: "x", Type: entity.TypePlayer}, "w", "", test.attrs, proposedAt)
			_, err := catchUp(e, []entity.Change{{Path: test.path, New: "hunted", HasNew: true}})
			if errors.Is(err, errCorruptFact) != test.corrupt {
				t.Fatalf("catchUp error = %v, want corrupt = %v", err, test.corrupt)
			}
			if !test.corrupt && err != nil {
				t.Fatalf("catchUp: %v", err)
			}
		})
	}
}
