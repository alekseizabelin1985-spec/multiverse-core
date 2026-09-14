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
// Ma-3). Step 1 refuses a path below a scalar attribute on its own, and since
// T-472 so does ApplyOps at step 7 for the type of the entity; these tests call
// plan past step 1, so that the decision on the entity is held even where a
// path slips through step 1. They run without the laws of the world, so
// that every refusal here is the norm's and none is inv-02's: the process runs
// State with them since T-471 (contexts_state.go).
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
		// fighting seeds the character inside the encounter enc-1.
		fighting bool
	}{
		{"status.x into abandoned", task, "combat", []entity.Op{{Op: entity.OpSet, Path: "status.x", Value: entity.StatusAbandoned}}, ReasonInvalidOp, "", "", false},
		{"status.x into sleeping", task, "combat", []entity.Op{{Op: entity.OpSet, Path: "status.x", Value: "sleeping"}}, ReasonInvalidOp, "", "", false},
		{"status.x into dead by the gateway with forget", gw, "forget", []entity.Op{{Op: entity.OpSet, Path: "status.x", Value: entity.StatusDead}}, ReasonInvalidOp, "", "", false},
		{"status removed", task, "combat", []entity.Op{{Op: entity.OpRemove, Path: "status"}}, ReasonInvalidOp, "", "", false},
		{"rest writes hp.x", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp.x", Value: 999}}, ReasonInvalidOp, "", "", false},
		{"rest writes hp as a text", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: "10"}}, ReasonInvalidOp, "", "", false},
		{"rest writes hp as a fraction", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 5.5}}, ReasonInvalidOp, "", "", false},
		{"rest removes hp", gw, "rest", []entity.Op{{Op: entity.OpRemove, Path: "hp"}}, ReasonInvalidOp, "", "", false},
		// C-02 v1.8 p. 3: hp equal to hp_max; below it the rest is not the
		// gateway's to propose (it was inv-02 under "not above" until T-471).
		{"rest below zero", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: -1}}, ReasonLevelViolation, "", "", false},
		{"rest at hp_max-1", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 9}}, ReasonLevelViolation, "", "", false},
		{"rest that leaves hp where it is, below hp_max", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 3}}, ReasonLevelViolation, "", "", false},
		{"rest at hp_max+1", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 11}}, ReasonLawViolation, "inv-02", "", false},
		{"rest of a character without hp_max", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 10}}, ReasonLawViolation, "inv-02", "hp_max", false},
		{"rest to zero of a character without hp_max", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 0}}, ReasonLawViolation, "inv-02", "hp_max", false},
		{"rest that writes position beside hp", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 10}, {Op: entity.OpSet, Path: "position", Value: "r"}}, ReasonLevelViolation, "", "", false},
		{"rest that writes status", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "status", Value: entity.StatusAlive}}, ReasonLevelViolation, "", "", false},
		{"rest inside an encounter", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 10}}, ReasonLawViolation, "", "", true},
		{"rest inside an encounter that writes position", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 10}, {Op: entity.OpSet, Path: "position", Value: "r"}}, ReasonLevelViolation, "", "", true},
		// C-02 v1.8a p. 3: what step 7 refuses is the form of the package and
		// answers before the encounter; what it lets through — hp gone — is the
		// kind of hp, asked after the encounter.
		{"rest inside an encounter writes hp as a text", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: "10"}}, ReasonInvalidOp, "", "", true},
		{"rest inside an encounter appends to hp", gw, "rest", []entity.Op{{Op: entity.OpAppend, Path: "hp", Value: 10}}, ReasonInvalidOp, "", "", true},
		{"rest inside an encounter removes hp", gw, "rest", []entity.Op{{Op: entity.OpRemove, Path: "hp"}}, ReasonLawViolation, "", "", true},
		{"rest inside an encounter below hp_max", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 9}}, ReasonLawViolation, "", "", true},
		{"abandoned by an agent", task, "death", []entity.Op{{Op: entity.OpSet, Path: "status", Value: entity.StatusAbandoned}}, ReasonLevelViolation, "", "", false},
		{"rest up to hp_max", gw, "rest", []entity.Op{{Op: entity.OpSet, Path: "hp", Value: 10}}, "", "", "", false},
		{"status set to the value it holds", task, "combat", []entity.Op{{Op: entity.OpSet, Path: "status", Value: entity.StatusAlive}}, "", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := memstore.New()
			attrs := map[string]any{"hp": 3, "hp_max": 10, "status": entity.StatusAlive}
			delete(attrs, tc.drop)
			if tc.fighting {
				attrs[entity.AttrEncounterID] = "enc-1"
			}
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

// The encounter of a rest is read on the character as the rest found it, not
// after the operations (review #1 of T-056, Mi-1). Since C-02 v1.8 p. 3 a rest
// that also erases encounter_id is refused at step 6 before this is asked, so
// the reading is held here, on restRefusal itself.
func TestRestReadsTheEncounterBeforeTheOperations(t *testing.T) {
	before := entity.New(entity.Ref{ID: "player-A", Type: entity.TypePlayer}, "w", "",
		map[string]any{"hp": 3, "hp_max": 10, entity.AttrEncounterID: "enc-1"}, time.Time{})
	after := entity.Clone(before)
	after.Attributes = map[string]any{"hp": 10, "hp_max": 10, entity.AttrEncounterID: ""}
	set := entity.ChangeSet{Ops: []entity.Op{
		{Op: entity.OpSet, Path: "hp", Value: 10}, {Op: entity.OpSet, Path: entity.AttrEncounterID, Value: ""},
	}}

	reason, law := restRefusal(Proposer{Kind: ProposerGateway}, set, before, after)

	if reason != ReasonLawViolation || law != "" {
		t.Errorf("refused %q %q, want law_violation without invariant_id: the character rested inside enc-1", reason, law)
	}
}
