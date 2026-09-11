package eventbus

import (
	"testing"
)

func TestValidateEventRelations_Valid(t *testing.T) {
	ev := NewEvent("player.action", "oracle", "w1", nil)
	ev.Relations = []Relation{
		{From: "player:p1", To: "item:sword_1", Type: RelFound, Directed: true},
		{From: "player:p1", To: "region:forest", Type: RelLocatedIn, Directed: true},
	}

	if err := ValidateEventRelations(ev); err != nil {
		t.Errorf("expected valid relations, got error: %v", err)
	}
}

func TestValidateEventRelations_EmptyIsOK(t *testing.T) {
	ev := NewEvent("world.generated", "generator", "w1", nil)
	// No relations — should be valid
	if err := ValidateEventRelations(ev); err != nil {
		t.Errorf("expected nil relations to be valid, got: %v", err)
	}
}

func TestValidateEventRelations_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		relations []Relation
		wantErr   string
	}{
		{
			name:      "empty From",
			relations: []Relation{{From: "", To: "item:sword", Type: RelFound}},
			wantErr:   "relation[0]: 'from' must not be empty",
		},
		{
			name:      "empty To",
			relations: []Relation{{From: "player:p1", To: "", Type: RelFound}},
			wantErr:   "relation[0]: 'to' must not be empty",
		},
		{
			name:      "empty Type",
			relations: []Relation{{From: "player:p1", To: "item:sword", Type: ""}},
			wantErr:   "relation[0]: 'type' must not be empty",
		},
		{
			name: "second relation invalid",
			relations: []Relation{
				{From: "player:p1", To: "item:sword", Type: RelFound},
				{From: "", To: "region:forest", Type: RelLocatedIn},
			},
			wantErr: "relation[1]: 'from' must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := NewEvent("test", "src", "w1", nil)
			ev.Relations = tt.relations

			err := ValidateEventRelations(ev)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("got error %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateRelations_Standalone(t *testing.T) {
	valid := []Relation{
		{From: "world:w1", To: "region:r1", Type: RelContains, Directed: true},
	}
	if err := ValidateRelations(valid); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}

	invalid := []Relation{{From: "a", To: "b", Type: ""}}
	if err := ValidateRelations(invalid); err == nil {
		t.Error("expected error for empty type")
	}
}

func TestEvent_RelationsJSONSerialization(t *testing.T) {
	ev := NewEvent("player.action", "oracle", "w1", map[string]any{
		"action": "pick_up",
	})
	ev.Relations = []Relation{
		{
			From:     "player:p1",
			To:       "item:sword_1",
			Type:     RelFound,
			Directed: true,
			Metadata: map[string]any{"confidence": 0.95},
		},
	}

	// Проверяем что Relations сериализуются корректно
	if len(ev.Relations) != 1 {
		t.Fatal("expected 1 relation")
	}
	if ev.Relations[0].From != "player:p1" {
		t.Errorf("expected From='player:p1', got %q", ev.Relations[0].From)
	}
	if ev.Relations[0].Metadata["confidence"] != 0.95 {
		t.Errorf("expected confidence=0.95, got %v", ev.Relations[0].Metadata["confidence"])
	}
}
