package state

import (
	"testing"
	"time"

	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// The status and the hit points of a change set are decided on the entity after
// the operations, not on the path an operation named (review #2 of T-056,
// Ma-3). Step 1 refuses a path below a scalar attribute on its own; these tests
// call plan past step 1, so that the decision on the entity is held even where
// a path slips through the grammar. They run without the laws of the world, as
// State runs in the process (contexts_state.go).
func TestTheStatusAndTheHitPointsAreReadOnTheEntity(t *testing.T) {
	const world = "w"
	task := Proposer{Kind: ProposerAgent, Level: "task"}
	gw := Proposer{Kind: ProposerGateway}
	for _, tc := range []struct {
		name  string
		by    Proposer
		cause string
		ops   []entity.Op
		want  Reason
		law   string
		// drop names an attribute the character is seeded without.
		drop string
	}{
		{"status.x into abandoned", task, "combat", []entity.Op{{Op: entity.OpSet, Path: "status.x", Value: entity.StatusAbandoned}}, ReasonInvalidOp, "", ""},
		{"status.x into sleeping", task, "combat", []entity.Op{{Op: entity.OpSet, Path: "status.x", Value: "sleeping"}}, ReasonInvalidOp, "", ""},
		{"status.x into dead by the gateway with forget", gw, "forget", []entity.Op{{Op: entity.OpSet, Path: "status.x", Value: entity.StatusDead}}, ReasonInvalidOp, "", ""},
		{"status removed", task, "combat", []entity.Op{{Op: entity.OpRemove, Path: "status"}}, ReasonInvalidOp, "", ""},
		{"rest writes hp.x", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp.x", Value: 999}}, ReasonInvalidOp, "", ""},
		{"rest writes hp as a text", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: "10"}}, ReasonInvalidOp, "", ""},
		{"rest below zero", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: -1}}, ReasonLawViolation, "inv-02", ""},
		{"rest at hp_max+1", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 11}}, ReasonLawViolation, "inv-02", ""},
		{"rest of a character without hp_max", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 10}}, ReasonLawViolation, "inv-02", "hp_max"},
		{"abandoned by an agent", task, "death", []entity.Op{{Op: entity.OpSet, Path: "status", Value: entity.StatusAbandoned}}, ReasonLevelViolation, "", ""},
		{"rest up to hp_max", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 10}}, "", "", ""},
		{"status set to the value it holds", task, "combat", []entity.Op{{Op: entity.OpSet, Path: "status", Value: entity.StatusAlive}}, "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := memstore.New()
			attrs := map[string]any{"hp": 3, "hp_max": 10, "status": entity.StatusAlive}
			delete(attrs, tc.drop)
			e := entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, world, "", attrs, time.Time{})
			if err := store.Put(world, e); err != nil {
				t.Fatal(err)
			}
			a, err := NewApplier(ApplierConfig{WorldID: world, Store: store, Publisher: discard{}})
			if err != nil {
				t.Fatal(err)
			}
			set := entity.ChangeSet{
				Entity: eventbus.Entity{Entity: eventbus.EntityRef{ID: "player-A", Type: entity.TypePlayer}},
				Ops:    tc.ops,
			}

			_, refusal := a.plan(&Proposal{Proposer: tc.by, Cause: tc.cause}, set)

			switch {
			case tc.want == "" && refusal != nil:
				t.Fatalf("refused %s, want planned", refusal.Reason)
			case tc.want != "" && refusal == nil:
				t.Fatalf("planned, want %s", tc.want)
			case tc.want != "" && refusal.Reason != tc.want:
				t.Errorf("refused %s, want %s", refusal.Reason, tc.want)
			case tc.want != "" && refusal.InvariantID != tc.law:
				t.Errorf("invariant_id %q, want %q", refusal.InvariantID, tc.law)
			}
		})
	}
}
