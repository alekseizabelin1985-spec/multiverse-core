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
			"id":    "player-123",
			"type":  "player",
			"name":  "Вася",
			"world": map[string]any{"id": "world-789"},
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
		"entity_id":   "npc-456",
		"entity_type": "npc",
		"entity_name": "Старейшина",
		"world_id":    "world-789",
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

func TestEventPayloadWithScope(t *testing.T) {
	payload := NewEventPayload().
		WithEntity("player-123", "player", "Вася").
		WithWorld("world-789").
		WithScope("solo-abc", "solo")

	if payload.Scope == nil {
		t.Fatal("Scope should not be nil")
	}

	if payload.Scope.ID != "solo-abc" {
		t.Errorf("Expected scope ID 'solo-abc', got '%s'", payload.Scope.ID)
	}

	if payload.Scope.Type != "solo" {
		t.Errorf("Expected scope type 'solo', got '%s'", payload.Scope.Type)
	}

	// Проверяем конвертацию в map
	result := payload.ToMap()
	scope, ok := result["scope"].(map[string]any)
	if !ok {
		t.Fatal("scope should be map[string]any in ToMap output")
	}

	if scope["id"] != "solo-abc" {
		t.Errorf("Expected scope.id 'solo-abc', got '%v'", scope["id"])
	}

	if scope["type"] != "solo" {
		t.Errorf("Expected scope.type 'solo', got '%v'", scope["type"])
	}
}

func TestExtractScope(t *testing.T) {
	// Тест с новой структурой: scope: {id, type}
	newStructPayload := map[string]any{
		"scope": map[string]any{
			"id":   "group-xyz",
			"type": "group",
		},
	}

	scope := ExtractScope(newStructPayload)
	if scope == nil {
		t.Fatal("Scope should not be nil")
	}

	if scope.ID != "group-xyz" {
		t.Errorf("Expected ID 'group-xyz', got '%s'", scope.ID)
	}

	if scope.Type != "group" {
		t.Errorf("Expected type 'group', got '%s'", scope.Type)
	}

	// Тест со старой структурой (fallback): scope_id, scope_type
	oldStructPayload := map[string]any{
		"scope_id":   "city-123",
		"scope_type": "city",
	}

	scope = ExtractScope(oldStructPayload)
	if scope == nil {
		t.Fatal("Scope should not be nil for old structure")
	}

	if scope.ID != "city-123" {
		t.Errorf("Expected ID 'city-123', got '%s'", scope.ID)
	}

	if scope.Type != "city" {
		t.Errorf("Expected type 'city', got '%s'", scope.Type)
	}

	// Тест с частичными данными (только ID)
	partialPayload := map[string]any{
		"scope": map[string]any{
			"id": "quest-456",
		},
	}

	scope = ExtractScope(partialPayload)
	if scope == nil || scope.ID != "quest-456" {
		t.Error("Should extract scope with only ID")
	}

	// Тест с пустым payload (должен вернуть nil)
	emptyPayload := map[string]any{}
	scope = ExtractScope(emptyPayload)
	if scope != nil {
		t.Error("Should return nil for empty payload")
	}
}

func TestPathAccessor_GetString(t *testing.T) {
	payload := map[string]any{
		"entity": map[string]any{
			"id":   "player-123",
			"type": "player",
			"metadata": map[string]any{
				"level": 15,
			},
		},
		"scope": map[string]any{
			"id":   "solo-abc",
			"type": "solo",
		},
		"world": map[string]any{
			"id": "world-789",
		},
	}

	pa := NewPathAccessor(payload)

	// Тест извлечения строк по разным путям

	tests := []struct {
		path     string
		expected string
		shouldOK bool
	}{
		{"entity.id", "player-123", true},
		{"entity.type", "player", true},
		{"scope.id", "solo-abc", true},
		{"scope.type", "solo", true},
		{"world.id", "world-789", true},
		{"nonexistent.path", "", false},
		{"entity.metadata", "", false}, // не строка, а map
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			val, ok := pa.GetString(tt.path)
			if ok != tt.shouldOK {
				t.Errorf("GetString(%q): expected ok=%v, got ok=%v", tt.path, tt.shouldOK, ok)
			}
			if tt.shouldOK && val != tt.expected {
				t.Errorf("GetString(%q): expected %q, got %q", tt.path, tt.expected, val)
			}
		})
	}
}

func TestPathAccessor_GetInt(t *testing.T) {
	payload := map[string]any{
		"entity": map[string]any{
			"metadata": map[string]any{
				"level": 15,
				"xp":    int64(2500),
			},
		},
		"stats": map[string]any{
			"hp": 100.0, // float64
		},
	}

	pa := NewPathAccessor(payload)

	// Тест извлечения int по разным путям с разными типами исходных данных

	tests := []struct {
		path     string
		expected int
		shouldOK bool
	}{
		{"entity.metadata.level", 15, true},
		{"entity.metadata.xp", 2500, true},
		{"stats.hp", 100, true}, // float64 -> int
		{"nonexistent.path", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			val, ok := pa.GetInt(tt.path)
			if ok != tt.shouldOK {
				t.Errorf("GetInt(%q): expected ok=%v, got ok=%v", tt.path, tt.shouldOK, ok)
			}
			if tt.shouldOK && val != tt.expected {
				t.Errorf("GetInt(%q): expected %d, got %d", tt.path, tt.expected, val)
			}
		})
	}
}

func TestPathAccessor_GetFloat(t *testing.T) {
	payload := map[string]any{
		"weather": map[string]any{
			"temperature": map[string]any{
				"value": 25.5,
			},
		},
		"stats": map[string]any{
			"damage": 100, // int -> float64
		},
	}

	pa := NewPathAccessor(payload)

	val, ok := pa.GetFloat("weather.temperature.value")
	if !ok || val != 25.5 {
		t.Errorf("Expected 25.5, got %v (ok=%v)", val, ok)
	}

	val, ok = pa.GetFloat("stats.damage")
	if !ok || val != 100.0 {
		t.Errorf("Expected 100.0, got %v (ok=%v)", val, ok)
	}

	_, ok = pa.GetFloat("nonexistent")
	if ok {
		t.Error("Should return false for nonexistent path")
	}
}

func TestPathAccessor_GetBool(t *testing.T) {
	payload := map[string]any{
		"entity": map[string]any{
			"active": true,
		},
		"quest": map[string]any{
			"completed": false,
		},
	}

	pa := NewPathAccessor(payload)

	val, ok := pa.GetBool("entity.active")
	if !ok || val != true {
		t.Errorf("Expected true, got %v (ok=%v)", val, ok)
	}

	val, ok = pa.GetBool("quest.completed")
	if !ok || val != false {
		t.Errorf("Expected false, got %v (ok=%v)", val, ok)
	}

	_, ok = pa.GetBool("nonexistent")
	if ok {
		t.Error("Should return false for nonexistent path")
	}
}

func TestPathAccessor_GetMap(t *testing.T) {
	payload := map[string]any{
		"entity": map[string]any{
			"id": "player-123",
			"metadata": map[string]any{
				"level": 15,
			},
		},
		"scope": map[string]any{
			"id": "solo-abc",
		},
	}

	pa := NewPathAccessor(payload)

	// Извлечение map по пути
	emptyMetadata, ok := pa.GetMap("entity.metadata")
	if !ok {
		t.Fatal("entity.metadata should be retrievable as map")
	}
	if emptyMetadata["level"] != 15 {
		t.Errorf("Expected metadata.level=15, got %v", emptyMetadata["level"])
	}

	// Извлечение самого entity
	entityMap, ok := pa.GetMap("entity")
	if !ok {
		t.Fatal("entity should be retrievable as map")
	}
	if entityMap["id"] != "player-123" {
		t.Errorf("Expected entity.id='player-123', got %v", entityMap["id"])
	}

	// Несуществующий путь или не-map значение
	_, ok = pa.GetMap("entity.id") // это строка, не map
	if ok {
		t.Error("Should return false for non-map value")
	}

	_, ok = pa.GetMap("nonexistent.path")
	if ok {
		t.Error("Should return false for nonexistent path")
	}
}

func TestPathAccessor_GetSlice(t *testing.T) {
	payload := map[string]any{
		"player": map[string]any{
			"inventory": []any{"sword", "potion", "map"},
		},
		"quest": map[string]any{
			"objectives": []any{
				map[string]any{"id": "obj1", "done": false},
				map[string]any{"id": "obj2", "done": true},
			},
		},
	}

	pa := NewPathAccessor(payload)

	inv, ok := pa.GetSlice("player.inventory")
	if !ok || len(inv) != 3 {
		t.Errorf("Expected 3 items in inventory, got %d (ok=%v)", len(inv), ok)
	}

	obj, ok := pa.GetSlice("quest.objectives")
	if !ok || len(obj) != 2 {
		t.Errorf("Expected 2 objectives, got %d (ok=%v)", len(obj), ok)
	}

	// Несуществующий путь или не-slice значение
	_, ok = pa.GetSlice("player.inventory[0]") // это строка, не slice
	if ok {
		t.Error("Should return false for non-slice value")
	}
}

func TestPathAccessor_Has(t *testing.T) {
	payload := map[string]any{
		"entity": map[string]any{
			"id": "player-123",
		},
		"scope": map[string]any{
			"id": "solo-abc",
		},
	}

	pa := NewPathAccessor(payload)

	if !pa.Has("entity.id") {
		t.Error("Has('entity.id') should return true")
	}
	if !pa.Has("scope.id") {
		t.Error("Has('scope.id') should return true")
	}
	if pa.Has("nonexistent.path") {
		t.Error("Has('nonexistent.path') should return false")
	}
}

func TestPathAccessor_GetAllPaths(t *testing.T) {
	payload := map[string]any{
		"entity": map[string]any{
			"id":   "player-123",
			"type": "player",
		},
		"scope": map[string]any{
			"id": "solo-abc",
		},
	}

	pa := NewPathAccessor(payload)
	paths := pa.GetAllPaths()

	// Проверяем что все ожидаемые пути присутствуют
	expectedPaths := []string{"entity", "entity.id", "entity.type", "scope", "scope.id"}
	for _, expected := range expectedPaths {
		found := false
		for _, p := range paths {
			if p == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected path '%s' not found in GetAllPaths(): %v", expected, paths)
		}
	}
}

func TestEvent_PathAccessor(t *testing.T) {
	event := NewEvent("test.event", "test-source", "world-123", map[string]any{
		"entity": map[string]any{
			"id":   "player-123",
			"type": "player",
		},
		"scope": map[string]any{
			"id":   "group-xyz",
			"type": "group",
		},
	})

	// Используем метод Path() события — возвращает jsonpath.Accessor
	pa := event.Path()

	entityID, ok := pa.GetString("entity.id")
	if !ok || entityID != "player-123" {
		t.Errorf("Expected entity.id='player-123', got %q (ok=%v)", entityID, ok)
	}

	scopeType, ok := pa.GetString("scope.type")
	if !ok || scopeType != "group" {
		t.Errorf("Expected scope.type='group', got %q (ok=%v)", scopeType, ok)
	}
}

// TestEvent_PathAccessor_JsonPathFeatures проверяет что event.Path() поддерживает все фичи jsonpath
func TestEvent_PathAccessor_JsonPathFeatures(t *testing.T) {
	event := NewEvent("test", "src", "w1", map[string]any{
		"entity": map[string]any{
			"id": "p1",
			"stats": map[string]any{
				"hp": 100,
			},
			"tags": []any{"warrior", "elite"},
		},
		"active": true,
	})

	pa := event.Path()

	// Тестируем разные геттеры из jsonpath
	id, ok := pa.GetString("entity.id")
	if !ok || id != "p1" {
		t.Errorf("GetString: got %q (ok=%v)", id, ok)
	}

	hp, ok := pa.GetInt("entity.stats.hp")
	if !ok || hp != 100 {
		t.Errorf("GetInt: got %d (ok=%v)", hp, ok)
	}

	active, ok := pa.GetBool("active")
	if !ok || !active {
		t.Errorf("GetBool: got %v (ok=%v)", active, ok)
	}

	tags, ok := pa.GetSlice("entity.tags")
	if !ok || len(tags) != 2 {
		t.Errorf("GetSlice: got %d items (ok=%v)", len(tags), ok)
	}

	// Проверка через массивы по индексу (фича jsonpath)
	firstTag, ok := pa.GetString("entity.tags[0]")
	if !ok || firstTag != "warrior" {
		t.Errorf("Array index access: got %q (ok=%v)", firstTag, ok)
	}

	// Has для быстрой проверки
	if !pa.Has("entity.stats.hp") {
		t.Error("Has should return true for existing path")
	}
}

func TestGetWorldIDFromEvent(t *testing.T) {
	// Тест с World на топ-уровне (через NewEvent)
	eventWithWorld := NewEvent("test", "src", "world-123", map[string]any{})

	worldID := GetWorldIDFromEvent(eventWithWorld)
	if worldID != "world-123" {
		t.Errorf("Expected 'world-123' from World, got '%s'", worldID)
	}

	// Тест с пустым миром, но world в payload
	eventPayloadWorld := NewEvent("test", "src", "", map[string]any{
		"world": map[string]any{"id": "world-456"},
	})

	worldID = GetWorldIDFromEvent(eventPayloadWorld)
	// NewEvent с пустым worldID не устанавливает World, хелпер читает из payload
	if worldID != "world-456" {
		t.Errorf("Expected 'world-456' from payload.world.id, got '%s'", worldID)
	}

	// Тест с World на топ-уровне (ручная структура)
	eventWithWorldDirect := Event{
		World:   &WorldRef{ID: "world-789"},
		Payload: map[string]any{},
	}

	worldID = GetWorldIDFromEvent(eventWithWorldDirect)
	if worldID != "world-789" {
		t.Errorf("Expected 'world-789' from World, got '%s'", worldID)
	}
}

func TestGetScopeFromEvent(t *testing.T) {
	// Тест с Scope через EventPayload
	payload := NewEventPayload().WithScope("city-abc", "city")
	eventWithScope := NewStructuredEvent("test", "src", "world-123", payload)

	scope := GetScopeFromEvent(eventWithScope)
	if scope == nil || scope.ID != "city-abc" || scope.Type != "city" {
		t.Errorf("Expected scope={city-abc, city}, got %+v", scope)
	}

	// Тест с Scope на топ-уровне (ручная структура)
	eventWithScopeDirect := Event{
		Scope:   &ScopeRef{ID: "quest-xyz", Type: "quest"},
		Payload: map[string]any{},
	}

	scope = GetScopeFromEvent(eventWithScopeDirect)
	if scope == nil || scope.ID != "quest-xyz" || scope.Type != "quest" {
		t.Errorf("Expected scope={quest-xyz, quest}, got %+v", scope)
	}

	// Тест без scope
	eventNoScope := NewEvent("test", "src", "world-123", map[string]any{})
	scope = GetScopeFromEvent(eventNoScope)
	if scope != nil {
		t.Errorf("Expected nil scope, got %+v", scope)
	}
}
