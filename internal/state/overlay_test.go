package state

import (
	"context"
	"testing"
	"time"

	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Between ApplyOps and Commit the copy is what the invariants of T-056 will
// read (§4.5 p. 8), and entity.ApplyOps puts the map or slice of an operation
// value into it as it is (backlog of T-448 p. 3). State copies the value, so a
// change to the operation after planning — whoever still holds it — reaches
// neither the entity committed nor its hash.
func TestAnOperationValueChangedAfterPlanningChangesNothing(t *testing.T) {
	const world = "w"
	store := memstore.New()
	e := entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, world, "", map[string]any{"hp": 10}, time.Time{})
	if err := store.Put(world, e); err != nil {
		t.Fatal(err)
	}
	a, err := NewApplier(ApplierConfig{WorldID: world, Store: store, Publisher: discard{}})
	if err != nil {
		t.Fatal(err)
	}
	gear := map[string]any{"slot": "hand", "items": []any{"sword"}}
	set := entity.ChangeSet{
		Entity: eventbus.Entity{Entity: eventbus.EntityRef{ID: "player-A", Type: entity.TypePlayer}},
		Ops:    []entity.Op{{Op: entity.OpSet, Path: "gear", Value: gear}},
	}

	plan, refusal := a.plan(set)
	if refusal != nil {
		t.Fatalf("plan refused: %+v", refusal)
	}
	want := entity.StateHash([]*entity.Entity{commit(clonePlan(plan), entity.LastChange{})})

	gear["slot"] = "stolen"
	gear["items"].([]any)[0] = "nothing"

	committed := commit(plan, entity.LastChange{})
	if got := entity.StateHash([]*entity.Entity{committed}); got != want {
		t.Errorf("state hash %s after the operation value changed, want %s: the plan shares the value", got, want)
	}
	if slot := committed.Attributes["gear"].(map[string]any)["slot"]; slot != "hand" {
		t.Errorf("gear.slot = %v, want hand", slot)
	}
}

// clonePlan is the plan as it stood, copied before the value is touched.
func clonePlan(p planned) planned {
	return planned{entity: entity.Clone(p.entity), attrs: entity.Clone(&entity.Entity{Attributes: p.attrs}).Attributes, changed: p.changed}
}

type discard struct{}

func (discard) Publish(context.Context, eventbus.Event) error { return nil }
