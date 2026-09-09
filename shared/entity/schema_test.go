package entity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"multiverse-core.io/schemas"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// The model and schemas/events/entity.*.v1.json describe the same thing from
// two sides. These tests keep them on speaking terms: a payload built from the
// model validates against the schema, and a payload written to the schema
// applies to the model.
//
// They compile the embedded schemas directly rather than through
// shared/contracts: the subject here is the shape of the model against the
// files, and it holds whether or not the registry has an entry for the type.

const schemaBase = "https://multiverse-core.io/schemas/events/"

func schemaOf(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()

	files, err := fs.Glob(schemas.FS, "events/*.json")
	if err != nil {
		t.Fatalf("list schemas: %v", err)
	}
	for _, file := range files {
		body, err := schemas.FS.Open(file)
		if err != nil {
			t.Fatalf("open %s: %v", file, err)
		}
		doc, err := jsonschema.UnmarshalJSON(body)
		_ = body.Close()
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		if err := compiler.AddResource(schemaBase+path.Base(file), doc); err != nil {
			t.Fatalf("add %s: %v", file, err)
		}
	}
	schema, err := compiler.Compile(schemaBase + name)
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return schema
}

func mustValidate(t *testing.T, schemaName string, payload any) {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if err := schemaOf(t, schemaName).Validate(doc); err != nil {
		t.Fatalf("%s does not accept the payload:\n%s\n%v", schemaName, encoded, err)
	}
}

func TestModelBuildsAValidUpdateProposal(t *testing.T) {
	e := player(t)
	ops := []entity.Op{
		{Op: entity.OpInc, Path: entity.AttrHP, Value: -4},
		{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusAlive},
		{Op: entity.OpAppend, Path: entity.AttrInventory, Value: map[string]any{
			"item_id": "item-1", "kind": "wolf-pelt", "name": "волчья шкура",
		}},
		{Op: entity.OpRemove, Path: entity.AttrEncounterID},
	}

	mustValidate(t, "entity.update.proposed.v1.json", map[string]any{
		"proposal_id": "prop-1",
		"changes":     []entity.ChangeSet{e.Propose(ops, true)},
		"atomic":      true,
		"cause":       "combat",
	})
}

func TestModelBuildsAValidForgetProposal(t *testing.T) {
	// The one transition into abandoned, as the gateway publishes it in the
	// /forget cascade (C-02 v1.2).
	e := player(t)
	change := e.Propose([]entity.Op{
		{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusAbandoned},
	}, true)

	mustValidate(t, "entity.update.proposed.v1.json", map[string]any{
		"proposal_id": "forget-player-A",
		"changes":     []entity.ChangeSet{change},
		"atomic":      true,
		"cause":       "forget",
	})
}

func TestModelBuildsAValidUpdatedFact(t *testing.T) {
	e := player(t)
	attrs, changed := applyOK(t, e,
		entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -4},
		entity.Op{Op: entity.OpAppend, Path: entity.AttrInventory, Value: map[string]any{"item_id": "item-1"}},
		entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: "dark-forest-01"})
	e.Commit(attrs, changed, entity.LastChange{ProposalID: "prop-1", Cause: "combat", AppliedAt: proposedAt})

	mustValidate(t, "entity.updated.v1.json", map[string]any{
		"entity":      e.EventEntity(),
		"version":     e.Version,
		"changed":     e.LastChange.Changed,
		"cause":       e.LastChange.Cause,
		"proposal_id": e.LastChange.ProposalID,
		"applied_at":  e.LastChange.AppliedAt.Format(time.RFC3339Nano),
	})
}

func TestModelBuildsAValidUpdatedFactWithNothingChanged(t *testing.T) {
	// UC-011 E2: hit points already at the maximum. The turn counted, the
	// version stayed, and changed has to encode as [] rather than null.
	e := player(t)
	attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpSet, Path: entity.AttrHP, Value: float64(10)})
	e.Commit(attrs, changed, entity.LastChange{ProposalID: "prop-2", Cause: "rest", AppliedAt: proposedAt})

	if e.Version != 1 {
		t.Fatalf("version = %d, want it to stay at 1", e.Version)
	}
	mustValidate(t, "entity.updated.v1.json", map[string]any{
		"entity":      e.EventEntity(),
		"version":     e.Version,
		"changed":     e.LastChange.Changed,
		"cause":       e.LastChange.Cause,
		"proposal_id": e.LastChange.ProposalID,
		"applied_at":  e.LastChange.AppliedAt.Format(time.RFC3339Nano),
	})
}

func TestModelBuildsAValidCreatedFact(t *testing.T) {
	e := player(t)
	mustValidate(t, "entity.created.v1.json", map[string]any{
		"entity":      e.EventEntity(),
		"version":     e.Version,
		"attributes":  e.Attributes,
		"proposal_id": "bootstrap:dark-forest-world:player/player-A",
	})

	mustValidate(t, "entity.create.proposed.v1.json", map[string]any{
		"proposal_id": "bootstrap:dark-forest-world:player/player-A",
		"entity":      e.EventEntity(),
		"attributes":  e.Attributes,
		"cause":       "init",
	})
}

func TestInvalidOpBecomesAValidRejection(t *testing.T) {
	// The model names the reason of a rejection; State names the payload. What
	// this checks is that the two enums line up.
	invalid := applyErr(t, player(t), entity.Op{Op: "replace", Path: entity.AttrHP, Value: 1})
	if invalid.Reason == "" {
		t.Fatal("ErrInvalidOp carries no reason")
	}
	mustValidate(t, "entity.update.rejected.v1.json", map[string]any{
		"proposal_id": "prop-1",
		"reason":      "invalid_op",
		"entity":      player(t).EventEntity(),
	})

	conflict := player(t).CheckVersion(func() *int64 { v := int64(9); return &v }())
	if conflict == nil {
		t.Fatal("CheckVersion(9) on version 1 returned nil")
	}
	var versionConflict entity.ErrVersionConflict
	if !errors.As(conflict, &versionConflict) {
		t.Fatalf("CheckVersion = %v, want ErrVersionConflict", conflict)
	}
	mustValidate(t, "entity.update.rejected.v1.json", map[string]any{
		"proposal_id": "prop-1",
		"reason":      "version_conflict",
		"entity":      player(t).EventEntity(),
		"details": map[string]any{
			"expected_version": versionConflict.Expected,
			"actual_version":   versionConflict.Actual,
		},
	})
}

// A payload written by hand to entity.update.proposed.v1.json, validated
// against the schema and then applied to the model — the direction State takes
// on every turn.
func TestSchemaPayloadAppliesToTheModel(t *testing.T) {
	const payload = `{
	  "proposal_id": "combat-round-3",
	  "atomic": true,
	  "cause": "combat",
	  "changes": [
	    {
	      "entity": { "entity": { "id": "player-A", "type": "player" }, "name": "Аня" },
	      "expected_version": 1,
	      "ops": [
	        { "op": "inc", "path": "hp", "value": -4 },
	        { "op": "append", "path": "inventory",
	          "value": { "item_id": "item-1", "kind": "wolf-pelt", "name": "волчья шкура" } },
	        { "op": "set", "path": "position", "value": "dark-forest-01" },
	        { "op": "remove", "path": "encounter_id" }
	      ]
	    }
	  ]
	}`

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(payload)))
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if err := schemaOf(t, "entity.update.proposed.v1.json").Validate(doc); err != nil {
		t.Fatalf("the payload of this test is not a valid proposal: %v", err)
	}

	var proposal struct {
		ProposalID string             `json:"proposal_id"`
		Atomic     bool               `json:"atomic"`
		Cause      string             `json:"cause"`
		Changes    []entity.ChangeSet `json:"changes"`
	}
	if err := json.Unmarshal([]byte(payload), &proposal); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if len(proposal.Changes) != 1 {
		t.Fatalf("changes = %+v, want one", proposal.Changes)
	}

	e := player(t)
	e.Attributes[entity.AttrEncounterID] = "enc-1"
	change := proposal.Changes[0]
	if change.Ref() != e.Ref() {
		t.Fatalf("the proposal addresses %v, not %v", change.Ref(), e.Ref())
	}
	if err := e.CheckVersion(change.ExpectedVersion); err != nil {
		t.Fatalf("CheckVersion: %v", err)
	}

	attrs, changed := applyOK(t, e, change.Ops...)
	e.Commit(attrs, changed, entity.LastChange{
		ProposalID:      proposal.ProposalID,
		ProposalEventID: "ev-proposal-1",
		Cause:           proposal.Cause,
		AppliedAt:       proposedAt,
		Atomic:          proposal.Atomic,
		BatchSize:       len(proposal.Changes),
	})

	if hp, _ := e.HP(); hp != 6 {
		t.Fatalf("hp = %d, want 6", hp)
	}
	if position, _ := e.Position(); position != "dark-forest-01" {
		t.Fatalf("position = %q, want dark-forest-01", position)
	}
	if e.HasAttr(entity.AttrEncounterID) {
		t.Fatal("encounter_id survived the remove")
	}
	items, err := e.Inventory()
	if err != nil || len(items) != 1 || items[0].ItemID != "item-1" {
		t.Fatalf("Inventory = %+v, %v", items, err)
	}
	if e.Version != 2 {
		t.Fatalf("version = %d, want 2", e.Version)
	}

	// And back out: what was applied encodes into a fact the schema accepts.
	mustValidate(t, "entity.updated.v1.json", map[string]any{
		"entity":      e.EventEntity(),
		"version":     e.Version,
		"changed":     e.LastChange.Changed,
		"cause":       e.LastChange.Cause,
		"proposal_id": e.LastChange.ProposalID,
		"applied_at":  e.LastChange.AppliedAt.Format(time.RFC3339Nano),
	})

	paths := make([]string, 0, len(e.LastChange.Changed))
	for _, c := range e.LastChange.Changed {
		paths = append(paths, c.Path)
	}
	want := []string{"hp", "inventory[0]", "position", "encounter_id"}
	if len(paths) != len(want) {
		t.Fatalf("changed paths = %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("changed paths = %v, want %v", paths, want)
		}
	}
}

// The other direction State takes, and the one the T-016 fixtures will take:
// a create proposal written by hand to entity.create.proposed.v1.json,
// validated against the schema and turned into an entity. It carries the whole
// attribute set, so this is where the schema and the getters meet.
func TestSchemaCreatePayloadBuildsTheModel(t *testing.T) {
	const payload = `{
	  "proposal_id": "bootstrap:dark-forest-world:npc/wolf-alpha",
	  "entity": { "entity": { "id": "wolf-alpha", "type": "npc" }, "name": "Вожак" },
	  "cause": "bootstrap",
	  "attributes": {
	    "kind": "wolf",
	    "region_id": "dark-forest-01",
	    "hp": 10, "hp_max": 10, "atk": 3, "def": 11, "dmg": "d4",
	    "status": "alive",
	    "position": "dark-forest-01",
	    "scope": { "id": "solo-player-A", "type": "solo" },
	    "loot": [ { "item_kind": "wolf-pelt", "name": "волчья шкура" } ],
	    "spawned_by": { "agent": "region-gm", "tick_event_id": "ev-tick-7" }
	  }
	}`

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(payload)))
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if err := schemaOf(t, "entity.create.proposed.v1.json").Validate(doc); err != nil {
		t.Fatalf("the payload of this test is not a valid create proposal: %v", err)
	}

	var proposal struct {
		ProposalID string          `json:"proposal_id"`
		Entity     eventbus.Entity `json:"entity"`
		Attributes map[string]any  `json:"attributes"`
		Cause      string          `json:"cause"`
	}
	if err := json.Unmarshal([]byte(payload), &proposal); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	e := entity.New(entity.RefFrom(proposal.Entity.Entity), "dark-forest-world",
		proposal.Entity.Name, proposal.Attributes, proposedAt)

	if e.Version != 1 {
		t.Fatalf("version = %d, want 1: a created entity starts at one", e.Version)
	}
	if !e.CreatedAt.Equal(proposedAt) || !e.UpdatedAt.Equal(proposedAt) {
		t.Fatalf("created_at = %v, updated_at = %v; want the timestamp of the proposal", e.CreatedAt, e.UpdatedAt)
	}
	if kind, ok := e.Kind(); !ok || kind != "wolf" {
		t.Fatalf("Kind = %q, %v", kind, ok)
	}
	if hp, ok := e.HP(); !ok || hp != 10 {
		t.Fatalf("HP = %d, %v", hp, ok)
	}
	if atk, ok := e.Atk(); !ok || atk != 3 {
		t.Fatalf("Atk = %d, %v", atk, ok)
	}
	if scope, ok := e.Scope(); !ok || scope.ID != "solo-player-A" || scope.Type != "solo" {
		t.Fatalf("Scope = %+v, %v", scope, ok)
	}
	loot, err := e.Loot()
	if err != nil || len(loot) != 1 || loot[0].ItemKind != "wolf-pelt" {
		t.Fatalf("Loot = %+v, %v", loot, err)
	}
	spawned, err := e.SpawnedBy()
	if err != nil || spawned.Agent != "region-gm" || spawned.TickEventID != "ev-tick-7" {
		t.Fatalf("SpawnedBy = %+v, %v", spawned, err)
	}

	// And back out: what was built announces itself as entity.created.
	mustValidate(t, "entity.created.v1.json", map[string]any{
		"entity":      e.EventEntity(),
		"version":     e.Version,
		"attributes":  e.Attributes,
		"proposal_id": proposal.ProposalID,
	})
}
