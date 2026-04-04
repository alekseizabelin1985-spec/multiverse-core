package narrativeorchestrator

import (
	"testing"

	"multiverse-core.io/shared/eventbus"
)

func TestExtractRelations(t *testing.T) {
	// Тест с валидными relations
	raw := []interface{}{
		map[string]interface{}{
			"from":     "player:p1",
			"to":       "item:sword_1",
			"type":     "FOUND",
			"directed": true,
			"metadata": map[string]interface{}{"action": "pick_up"},
		},
		map[string]interface{}{
			"from":     "player:p1",
			"to":       "region:forest",
			"type":     "LOCATED_IN",
			"directed": true,
		},
	}

	relations := extractRelations(raw)
	if len(relations) != 2 {
		t.Fatalf("Expected 2 relations, got %d", len(relations))
	}

	if relations[0].From != "player:p1" || relations[0].To != "item:sword_1" {
		t.Errorf("Unexpected relation: %+v", relations[0])
	}
	if relations[0].Type != "FOUND" {
		t.Errorf("Expected type FOUND, got %s", relations[0].Type)
	}
	if relations[0].Metadata["action"] != "pick_up" {
		t.Errorf("Expected metadata action 'pick_up', got %v", relations[0].Metadata["action"])
	}

	// Тест с невалидными данными (должны быть отфильтрованы)
	invalidRaw := []interface{}{
		map[string]interface{}{
			"from": "", // empty from — should be skipped
			"to":   "item:sword",
			"type": "FOUND",
		},
		"not_a_map", // should be skipped
		123,         // should be skipped
	}

	invalidRelations := extractRelations(invalidRaw)
	if len(invalidRelations) != 0 {
		t.Errorf("Expected 0 relations for invalid data, got %d", len(invalidRelations))
	}

	// Тест с nil
	nilResult := extractRelations(nil)
	if nilResult != nil {
		t.Error("Expected nil for nil input")
	}

	// Тест валидации через eventbus
	ev := eventbus.Event{
		EventType: "player.action",
		Relations: relations,
	}
	if err := eventbus.ValidateEventRelations(ev); err != nil {
		t.Errorf("Expected valid relations, got error: %v", err)
	}
}

func TestGetSafeString(t *testing.T) {
	m := map[string]interface{}{
		"name":   "test",
		"number": 42,
	}

	if getSafeString(m, "name") != "test" {
		t.Error("Expected 'test'")
	}
	if getSafeString(m, "missing") != "" {
		t.Error("Expected empty string for missing key")
	}
	if getSafeString(m, "number") != "" {
		t.Error("Expected empty string for non-string value")
	}
}

func TestGetSafeBool(t *testing.T) {
	m := map[string]interface{}{
		"active": true,
		"false":  false,
	}

	if !getSafeBool(m, "active") {
		t.Error("Expected true")
	}
	if getSafeBool(m, "false") {
		t.Error("Expected false")
	}
	if !getSafeBool(m, "missing") {
		t.Error("Expected default true for missing key")
	}
}
