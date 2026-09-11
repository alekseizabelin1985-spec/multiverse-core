package entity_test

import (
	"errors"
	"testing"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

func TestNewTakesItsTimeFromTheProposal(t *testing.T) {
	e := player(t)
	if e.Version != 1 {
		t.Fatalf("version = %d, want 1", e.Version)
	}
	if !e.CreatedAt.Equal(proposedAt) || !e.UpdatedAt.Equal(proposedAt) {
		t.Fatalf("created_at/updated_at = %v/%v, want %v", e.CreatedAt, e.UpdatedAt, proposedAt)
	}
	if e.SchemaVersion != entity.SchemaVersion {
		t.Fatalf("schema_version = %d, want %d", e.SchemaVersion, entity.SchemaVersion)
	}
}

func TestNewWorldIsItsOwnWorld(t *testing.T) {
	world := entity.New(entity.Ref{ID: "dark-forest-world", Type: entity.TypeWorld}, "", "Тёмный лес", nil, proposedAt)
	if world.WorldID != world.ID {
		t.Fatalf("world_id = %q, want %q", world.WorldID, world.ID)
	}
	if world.Attributes == nil {
		t.Fatal("attributes = nil, want an empty map to write into")
	}
}

func TestNewCopiesTheAttributesItIsGiven(t *testing.T) {
	attrs := map[string]any{"weather": "clear"}
	world := entity.New(entity.Ref{ID: "w", Type: entity.TypeWorld}, "", "", attrs, proposedAt)
	attrs["weather"] = "storm"

	if got, _ := world.Weather(); got != "clear" {
		t.Fatalf("weather = %q, want the entity to hold its own copy", got)
	}
}

func TestCloneSharesNothing(t *testing.T) {
	e := player(t)
	e.Attributes["tags"] = []any{"hunted"}
	e.History = []entity.HistoryEntry{{Version: 1, EventID: "ev-1"}}
	e.LastChange = &entity.LastChange{
		ProposalID: "p-1",
		Changed:    []entity.Change{{Path: "hp", Old: 10.0, New: 6.0}},
	}

	clone := entity.Clone(e)
	clone.Attributes["tags"].([]any)[0] = "calm"
	clone.Attributes[entity.AttrHP] = 1
	clone.History[0].EventID = "ev-2"
	clone.LastChange.ProposalID = "p-2"
	clone.LastChange.Changed[0].New = 0

	if tags, _ := e.AttrStrings("tags"); tags[0] != "hunted" {
		t.Fatalf("tags = %v, want the original untouched", tags)
	}
	if hp, _ := e.HP(); hp != 10 {
		t.Fatalf("hp = %d, want 10", hp)
	}
	if e.History[0].EventID != "ev-1" {
		t.Fatalf("history = %+v, want the original untouched", e.History)
	}
	if e.LastChange.ProposalID != "p-1" || e.LastChange.Changed[0].New != 6.0 {
		t.Fatalf("last_change = %+v, want the original untouched", e.LastChange)
	}
	if entity.Clone(nil) != nil {
		t.Fatal("Clone(nil) is not nil")
	}
}

func TestCheckVersion(t *testing.T) {
	e := player(t)
	e.Version = 7

	if err := e.CheckVersion(nil); err != nil {
		t.Fatalf("CheckVersion(nil) = %v, want no lock and no error", err)
	}
	seven := int64(7)
	if err := e.CheckVersion(&seven); err != nil {
		t.Fatalf("CheckVersion(7) = %v, want nil", err)
	}

	three := int64(3)
	err := e.CheckVersion(&three)
	var conflict entity.ErrVersionConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("CheckVersion(3) = %v, want ErrVersionConflict", err)
	}
	if conflict.Expected != 3 || conflict.Actual != 7 || conflict.Ref != e.Ref() {
		t.Fatalf("conflict = %+v, want expected 3, actual 7 for %v", conflict, e.Ref())
	}
	if conflict.Error() == "" {
		t.Fatal("Error() is empty")
	}
}

func TestCommitMovesTheVersionOnlyWhenSomethingChanged(t *testing.T) {
	e := player(t)
	appliedAt := proposedAt.Add(time.Minute)

	attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -4})
	e.Commit(attrs, changed, entity.LastChange{
		ProposalID:      "p-1",
		ProposalEventID: "ev-proposal-1",
		Cause:           "combat",
		AppliedAt:       appliedAt,
		Atomic:          true,
		BatchSize:       1,
	})

	if e.Version != 2 {
		t.Fatalf("version = %d, want 2", e.Version)
	}
	if hp, _ := e.HP(); hp != 6 {
		t.Fatalf("hp = %d, want 6", hp)
	}
	if !e.UpdatedAt.Equal(appliedAt) {
		t.Fatalf("updated_at = %v, want %v", e.UpdatedAt, appliedAt)
	}
	if e.LastChange == nil || e.LastChange.FactEventID != "" || len(e.LastChange.Changed) != 1 {
		t.Fatalf("last_change = %+v, want the commit record without a fact id yet", e.LastChange)
	}
	if len(e.History) != 1 || e.History[0].Version != 2 || e.History[0].ProposalID != "p-1" {
		t.Fatalf("history = %+v, want one entry for version 2", e.History)
	}

	// A turn that changed nothing: the fact is still published, at the same
	// version (C-02, UC-011 E2).
	attrs, changed = applyOK(t, e, entity.Op{Op: entity.OpSet, Path: entity.AttrHP, Value: 6})
	e.Commit(attrs, changed, entity.LastChange{ProposalID: "p-2", Cause: "rest", AppliedAt: appliedAt})
	if e.Version != 2 {
		t.Fatalf("version = %d, want it to stay at 2 for an empty changed list", e.Version)
	}
	if len(e.History) != 2 {
		t.Fatalf("history = %+v, want the turn recorded all the same", e.History)
	}
}

func TestCommitKeepsTheLastFiftyHistoryEntries(t *testing.T) {
	e := player(t)
	for i := 0; i < entity.HistoryLimit+12; i++ {
		attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpInc, Path: "kills", Value: 1})
		e.Commit(attrs, changed, entity.LastChange{ProposalID: "p", AppliedAt: proposedAt})
	}
	if len(e.History) != entity.HistoryLimit {
		t.Fatalf("history = %d entries, want %d", len(e.History), entity.HistoryLimit)
	}
	if last := e.History[len(e.History)-1]; last.Version != e.Version {
		t.Fatalf("last history entry = %+v, want version %d", last, e.Version)
	}
	if first := e.History[0]; first.Version != e.Version-int64(entity.HistoryLimit)+1 {
		t.Fatalf("first history entry = %+v, want the oldest of the tail", first)
	}
}

func TestSetFactEventIDClosesTheCommitRecord(t *testing.T) {
	e := player(t)
	attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1})
	e.Commit(attrs, changed, entity.LastChange{ProposalID: "p-1", AppliedAt: proposedAt})

	e.SetFactEventID("ev-fact-1")

	if e.LastEventID != "ev-fact-1" {
		t.Fatalf("last_event_id = %q, want ev-fact-1", e.LastEventID)
	}
	if e.LastChange.FactEventID != "ev-fact-1" {
		t.Fatalf("last_change.fact_event_id = %q, want ev-fact-1", e.LastChange.FactEventID)
	}
	if e.History[len(e.History)-1].EventID != "ev-fact-1" {
		t.Fatalf("history = %+v, want the fact id on the last entry", e.History)
	}
}

func TestRefCrossesToTheBusAndBack(t *testing.T) {
	e := player(t)
	ref := e.Ref()

	onTheWire := ref.EventRef()
	if onTheWire != (eventbus.EntityRef{ID: "player-A", Type: entity.TypePlayer}) {
		t.Fatalf("EventRef = %+v", onTheWire)
	}
	if back := entity.RefFrom(onTheWire); back != ref {
		t.Fatalf("RefFrom(EventRef) = %v, want %v", back, ref)
	}
	if ref.String() != "player:player-A" {
		t.Fatalf("String = %q, want player:player-A", ref.String())
	}

	payload := e.EventEntity()
	if payload.Entity != onTheWire || payload.Name != "Аня" {
		t.Fatalf("EventEntity = %+v, want the reference and the display name", payload)
	}
}

func TestStatusTransitionAllowed(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		// The one transition /forget makes, and the only way into abandoned
		// (C-02 v1.2, consolidation.md §14 З-2).
		{entity.StatusAlive, entity.StatusAbandoned, true},
		{entity.StatusAlive, entity.StatusDead, true},
		{entity.StatusAlive, entity.StatusAscendedFinal, true},
		// Nothing leaves a terminal status: inv-09 for dead, and the same
		// answer for the two that read as dead.
		{entity.StatusDead, entity.StatusAlive, false},
		{entity.StatusDead, entity.StatusAbandoned, false},
		{entity.StatusAbandoned, entity.StatusAlive, false},
		{entity.StatusAbandoned, entity.StatusAbandoned, false},
		{entity.StatusAscendedFinal, entity.StatusAlive, false},
		{entity.StatusAlive, entity.StatusAlive, false},
	}
	for _, test := range tests {
		t.Run(test.from+"->"+test.to, func(t *testing.T) {
			if got := entity.StatusTransitionAllowed(test.from, test.to); got != test.want {
				t.Fatalf("StatusTransitionAllowed(%q, %q) = %v, want %v", test.from, test.to, got, test.want)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	e := player(t)
	if e.IsTerminal() {
		t.Fatal("a live character is terminal")
	}
	for _, status := range entity.TerminalStatuses {
		e.Attributes[entity.AttrStatus] = status
		if !e.IsTerminal() {
			t.Fatalf("status %q is not terminal", status)
		}
	}
	// An encounter has no status at all and is never terminal by this rule
	// (state-and-mechanics.md §4.5 p. 5).
	encounter := entity.New(entity.Ref{ID: "enc-1", Type: entity.TypeEncounter}, "w", "", nil, proposedAt)
	if encounter.IsTerminal() {
		t.Fatal("an entity without a status is terminal")
	}
}

func TestValidType(t *testing.T) {
	for _, typ := range entity.Types {
		if !entity.ValidType(typ) {
			t.Fatalf("ValidType(%q) = false", typ)
		}
	}
	if entity.ValidType("city") {
		t.Fatal("ValidType(city) = true, want false: it is not a type of MVP-1")
	}
}

// Mi-7. Commit takes the attributes, not the map: the caller keeps its own, and
// what it does with it afterwards is its own business. In the flow of §4.5 the
// same overlay is carried from the ownership check to the write, and a package
// that promises no writes behind the caller's back cannot leave that door open
// in the other direction either.
func TestCommitCopiesTheAttributesItIsGiven(t *testing.T) {
	e := player(t)
	attrs, changed := applyOK(t, e, entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -4})
	e.Commit(attrs, changed, entity.LastChange{ProposalID: "p-1", AppliedAt: proposedAt})

	attrs[entity.AttrHP] = 999
	attrs["banner"] = map[string]any{"colour": "green"}

	if hp, _ := e.HP(); hp != 6 {
		t.Fatalf("hp = %d, want 6: the caller's map reached the entity", hp)
	}
	if e.HasAttr("banner") {
		t.Fatal("a key added to the caller's map after Commit turned up on the entity")
	}
}
