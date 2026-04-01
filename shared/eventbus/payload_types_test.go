package eventbus

import (
	"encoding/json"
	"testing"
)

func TestNewEventPayload(t *testing.T) {
	payload := NewEventPayload()

	if payload.Custom == nil {
		t.Error("Custom map should not be nil")
	}
}

func TestEventPayloadWithEntity(t *testing.T) {
	payload := NewEventPayload().
		WithEntity("player-123", "player", "Вася").
		WithWorld("world-789")

	if payload.Entity == nil {
		t.Fatal("Entity should not be nil")
	}

	if payload.Entity.ID != "player-123" {
		t.Errorf("Expected entity ID 'player-123', got '%s'", payload.Entity.ID)
	}

	if payload.Entity.Type != "player" {
		t.Errorf("Expected entity type 'player', got '%s'", payload.Entity.Type)
	}

	if payload.Entity.Name != "Вася" {
		t.Errorf("Expected entity name 'Вася', got '%s'", payload.Entity.Name)
	}

	if payload.World == nil || payload.World.ID != "world-789" {
		t.Error("World should be set correctly")
	}
}

func TestEventPayloadWithTarget(t *testing.T) {
	payload := NewEventPayload().
		WithEntity("player-123", "player", "Вася").
		WithTarget("region-456", "region", "Темный лес")

	if payload.Target == nil {
		t.Fatal("Target should not be nil")
	}

	if payload.Target.ID != "region-456" {
		t.Errorf("Expected target ID 'region-456', got '%s'", payload.Target.ID)
	}

	if payload.Target.Name != "Темный лес" {
		t.Errorf("Expected target name 'Темный лес', got '%s'", payload.Target.Name)
	}
}

func TestEventPayloadToMap(t *testing.T) {
	payload := NewEventPayload().
		WithEntity("player-123", "player", "Вася").
		WithTarget("region-456", "region", "").
		WithWorld("world-789")

	payload.Custom["custom_field"] = "custom_value"

	result := payload.ToMap()

	// Проверяем entity
	entity, ok := result["entity"].(map[string]any)
	if !ok {
		t.Fatal("entity should be map[string]any")
	}

	if entity["id"] != "player-123" {
		t.Errorf("Expected entity.id 'player-123', got '%v'", entity["id"])
	}

	// Проверяем target
	target, ok := result["target"].(map[string]any)
	if !ok {
		t.Fatal("target should be map[string]any")
	}

	if target["id"] != "region-456" {
		t.Errorf("Expected target.id 'region-456', got '%v'", target["id"])
	}

	// Проверяем world
	world, ok := result["world"].(map[string]any)
	if !ok {
		t.Fatal("world should be map[string]any")
	}

	if world["id"] != "world-789" {
		t.Errorf("Expected world.id 'world-789', got '%v'", world["id"])
	}

	// Проверяем кастомное поле
	if result["custom_field"] != "custom_value" {
		t.Error("custom_field should be preserved")
	}
}

func TestSetNested(t *testing.T) {
	payload := make(map[string]any)

	// Тестируем вложенную структуру
	SetNested(payload, "weather.change.to", "шторм")
	SetNested(payload, "weather.change.in.region.id", "region-456")
	SetNested(payload, "weather.previous.temperature.value", 25.5)

	// Проверяем что value установлено
	val, ok := GetNested(payload, "weather.change.to")
	if !ok {
		t.Fatal("weather.change.to should be set")
	}

	if val != "шторм" {
		t.Errorf("Expected 'шторм', got '%v'", val)
	}

	// Проверяем более глубокую вложенность
	val, ok = GetNested(payload, "weather.change.in.region.id")
	if !ok {
		t.Fatal("weather.change.in.region.id should be set")
	}

	if val != "region-456" {
		t.Errorf("Expected 'region-456', got '%v'", val)
	}

	// Проверяем числовое значение
	val, ok = GetNested(payload, "weather.previous.temperature.value")
	if !ok {
		t.Fatal("weather.previous.temperature.value should be set")
	}

	if val != 25.5 {
		t.Errorf("Expected 25.5, got '%v'", val)
	}
}

func TestSetNestedJSONSerialization(t *testing.T) {
	payload := make(map[string]any)

	SetNested(payload, "entity.id", "player-123")
	SetNested(payload, "entity.type", "player")
	SetNested(payload, "weather.change.to", "шторм")
	SetNested(payload, "weather.change.in.region.id", "region-456")

	// Проверяем JSON сериализацию
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	var result map[string]any
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal from JSON: %v", err)
	}

	// Проверяем структуру
	entity, ok := result["entity"].(map[string]any)
	if !ok {
		t.Fatal("entity should be map[string]any")
	}

	if entity["id"] != "player-123" {
		t.Errorf("Expected entity.id 'player-123', got '%v'", entity["id"])
	}

	weather, ok := result["weather"].(map[string]any)
	if !ok {
		t.Fatal("weather should be map[string]any")
	}

	weatherChange, ok := weather["change"].(map[string]any)
	if !ok {
		t.Fatal("weather.change should be map[string]any")
	}

	toVal, ok := weatherChange["to"].(string)
	if !ok || toVal != "шторм" {
		t.Errorf("Expected weather.change.to 'шторм', got '%v'", toVal)
	}
}

func TestExtractEntityID(t *testing.T) {
	// Тест с новой структурой
	newStructPayload := map[string]any{
		"entity": map[string]any{
			"id":     "player-123",
			"type":   "player",
			"name":   "Вася",
			"world":  map[string]any{"id": "world-789"},
		},
	}

	entity := ExtractEntityID(newStructPayload)
	if entity == nil {
		t.Fatal("Entity should not be nil")
	}

	if entity.ID != "player-123" {
		t.Errorf("Expected ID 'player-123', got '%s'", entity.ID)
	}

	if entity.Type != "player" {
		t.Errorf("Expected type 'player', got '%s'", entity.Type)
	}

	if entity.Name != "Вася" {
		t.Errorf("Expected name 'Вася', got '%s'", entity.Name)
	}

	if entity.World != "world-789" {
		t.Errorf("Expected world 'world-789', got '%s'", entity.World)
	}

	// Тест со старой структурой (fallback)
	oldStructPayload := map[string]any{
		"entity_id":     "npc-456",
		"entity_type":   "npc",
		"entity_name":   "Старейшина",
		"world_id":      "world-789",
	}

	entity = ExtractEntityID(oldStructPayload)
	if entity == nil {
		t.Fatal("Entity should not be nil for old structure")
	}

	if entity.ID != "npc-456" {
		t.Errorf("Expected ID 'npc-456', got '%s'", entity.ID)
	}

	if entity.Type != "npc" {
		t.Errorf("Expected type 'npc', got '%s'", entity.Type)
	}

	if entity.Name != "Старейшина" {
		t.Errorf("Expected name 'Старейшина', got '%s'", entity.Name)
	}

	if entity.World != "world-789" {
		t.Errorf("Expected world 'world-789', got '%s'", entity.World)
	}
}

func TestExtractTargetEntityID(t *testing.T) {
	// Тест с новой структурой
	newStructPayload := map[string]any{
		"target": map[string]any{
			"entity": map[string]any{
				"id":   "region-456",
				"type": "region",
				"name": "Темный лес",
			},
		},
	}

	entity := ExtractTargetEntityID(newStructPayload)
	if entity == nil {
		t.Fatal("Target entity should not be nil")
	}

	if entity.ID != "region-456" {
		t.Errorf("Expected ID 'region-456', got '%s'", entity.ID)
	}

	if entity.Type != "region" {
		t.Errorf("Expected type 'region', got '%s'", entity.Type)
	}

	// Тест со старой структурой (fallback)
	oldStructPayload := map[string]any{
		"target_id": "item-789",
	}

	entity = ExtractTargetEntityID(oldStructPayload)
	if entity == nil {
		t.Fatal("Target entity should not be nil for old structure")
	}

	if entity.ID != "item-789" {
		t.Errorf("Expected ID 'item-789', got '%s'", entity.ID)
	}
}

func TestExtractWorldID(t *testing.T) {
	// Тест с новой структурой
	newStructPayload := map[string]any{
		"world": map[string]any{
			"id": "world-123",
		},
	}

	worldID := ExtractWorldID(newStructPayload)
	if worldID != "world-123" {
		t.Errorf("Expected 'world-123', got '%s'", worldID)
	}

	// Тест со старой структурой
	oldStructPayload := map[string]any{
		"world_id": "world-456",
	}

	worldID = ExtractWorldID(oldStructPayload)
	if worldID != "world-456" {
		t.Errorf("Expected 'world-456', got '%s'", worldID)
	}
}

func TestToMapWithEntity(t *testing.T) {
	result := ToMapWithEntity("player-123", "player", "world-789", map[string]any{
		"action": "move",
	})

	entity, ok := result["entity"].(map[string]any)
	if !ok {
		t.Fatal("entity should be map[string]any")
	}

	if entity["id"] != "player-123" {
		t.Errorf("Expected entity.id 'player-123', got '%v'", entity["id"])
	}

	if entity["type"] != "player" {
		t.Errorf("Expected entity.type 'player', got '%v'", entity["type"])
	}

	world, ok := entity["world"].(map[string]any)
	if !ok {
		t.Fatal("entity.world should be map[string]any")
	}

	if world["id"] != "world-789" {
		t.Errorf("Expected entity.world.id 'world-789', got '%v'", world["id"])
	}

	if result["action"] != "move" {
		t.Error("action field should be preserved")
	}
}
