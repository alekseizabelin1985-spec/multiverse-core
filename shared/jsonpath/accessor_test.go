package jsonpath

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	data := map[string]any{"key": "value"}
	a := New(data)
	if !reflect.DeepEqual(a.data, data) {
		t.Error("New should store the provided data")
	}
}

func TestAccessor_GetString(t *testing.T) {
	data := map[string]any{
		"entity": map[string]any{
			"id":   "player-123",
			"name": "Вася",
			"meta": map[string]any{
				"level": "15",
			},
		},
		"flat_key": "direct_value",
	}
	a := New(data)

	tests := []struct {
		path     string
		expected string
		wantOK   bool
	}{
		{"entity.id", "player-123", true},
		{"entity.name", "Вася", true},
		{"entity.meta.level", "15", true},
		{"flat_key", "direct_value", true},
		{"nonexistent", "", false},
		{"entity", "", false}, // не строка
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := a.GetString(tt.path)
			if ok != tt.wantOK {
				t.Errorf("GetString(%q): ok=%v, want %v", tt.path, ok, tt.wantOK)
			}
			if tt.wantOK && got != tt.expected {
				t.Errorf("GetString(%q): got %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestAccessor_GetInt(t *testing.T) {
	data := map[string]any{
		"stats": map[string]any{
			"level":     15,
			"xp":        int64(2500),
			"accuracy":  95.5,
			"level_str": "20",
		},
	}
	a := New(data)

	tests := []struct {
		path     string
		expected int
		wantOK   bool
	}{
		{"stats.level", 15, true},
		{"stats.xp", 2500, true},
		{"stats.accuracy", 95, true}, // float -> int
		{"nonexistent", 0, false},
		{"stats", 0, false}, // не число
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := a.GetInt(tt.path)
			if ok != tt.wantOK {
				t.Errorf("GetInt(%q): ok=%v, want %v", tt.path, ok, tt.wantOK)
			}
			if tt.wantOK && got != tt.expected {
				t.Errorf("GetInt(%q): got %d, want %d", tt.path, got, tt.expected)
			}
		})
	}
}

func TestAccessor_GetFloat(t *testing.T) {
	data := map[string]any{
		"weather": map[string]any{
			"temp": 25.5,
			"hum":  80, // int -> float
		},
	}
	a := New(data)

	temp, ok := a.GetFloat("weather.temp")
	if !ok || temp != 25.5 {
		t.Errorf("GetFloat(weather.temp): got %v (ok=%v), want 25.5", temp, ok)
	}

	hum, ok := a.GetFloat("weather.hum")
	if !ok || hum != 80.0 {
		t.Errorf("GetFloat(weather.hum): got %v (ok=%v), want 80.0", hum, ok)
	}

	_, ok = a.GetFloat("nonexistent")
	if ok {
		t.Error("GetFloat(nonexistent) should return false")
	}
}

func TestAccessor_GetBool(t *testing.T) {
	data := map[string]any{
		"flags": map[string]any{
			"active":      true,
			"deleted":     false,
			"enabled_str": "true",
			"count":       5, // non-zero -> true
		},
	}
	a := New(data)

	tests := []struct {
		path     string
		expected bool
		wantOK   bool
	}{
		{"flags.active", true, true},
		{"flags.deleted", false, true},
		{"flags.enabled_str", true, true},
		{"flags.count", true, true},
		{"nonexistent", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := a.GetBool(tt.path)
			if ok != tt.wantOK {
				t.Errorf("GetBool(%q): ok=%v, want %v", tt.path, ok, tt.wantOK)
			}
			if tt.wantOK && got != tt.expected {
				t.Errorf("GetBool(%q): got %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestAccessor_GetMap(t *testing.T) {
	data := map[string]any{
		"entity": map[string]any{
			"id": "player-123",
			"meta": map[string]any{
				"level": 15,
			},
		},
	}
	a := New(data)

	meta, ok := a.GetMap("entity.meta")
	if !ok {
		t.Fatal("GetMap(entity.meta) should succeed")
	}
	if meta["level"] != 15 {
		t.Errorf("meta[level]: got %v, want 15", meta["level"])
	}

	_, ok = a.GetMap("entity.id") // это строка, не map
	if ok {
		t.Error("GetMap(entity.id) should fail for non-map value")
	}
}

func TestAccessor_GetSlice(t *testing.T) {
	data := map[string]any{
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
	a := New(data)

	inv, ok := a.GetSlice("player.inventory")
	if !ok || len(inv) != 3 {
		t.Errorf("GetSlice(player.inventory): got %d items (ok=%v), want 3", len(inv), ok)
	}

	obj, ok := a.GetSlice("quest.objectives")
	if !ok || len(obj) != 2 {
		t.Errorf("GetSlice(quest.objectives): got %d items (ok=%v), want 2", len(obj), ok)
	}
}

func TestAccessor_GetAny(t *testing.T) {
	data := map[string]any{
		"entity": map[string]any{
			"id": "player-123",
		},
		"count": 42,
	}
	a := New(data)

	// Извлечение всей подструктуры
	entity, ok := a.GetAny("entity")
	if !ok {
		t.Fatal("GetAny(entity) should succeed")
	}
	entityMap, ok := entity.(map[string]any)
	if !ok || entityMap["id"] != "player-123" {
		t.Errorf("entity[id]: got %v, want player-123", entityMap["id"])
	}

	// Извлечение примитива
	count, ok := a.GetAny("count")
	if !ok || count != 42 {
		t.Errorf("GetAny(count): got %v (ok=%v), want 42", count, ok)
	}
}

func TestAccessor_Has(t *testing.T) {
	data := map[string]any{
		"entity": map[string]any{
			"id": "player-123",
		},
	}
	a := New(data)

	if !a.Has("entity.id") {
		t.Error("Has(entity.id) should return true")
	}
	if a.Has("nonexistent.path") {
		t.Error("Has(nonexistent.path) should return false")
	}
	if !a.Has("entity") {
		t.Error("Has(entity) should return true even for non-primitive")
	}
}

func TestAccessor_GetAllPaths(t *testing.T) {
	data := map[string]any{
		"entity": map[string]any{
			"id":   "player-123",
			"type": "player",
			"stats": map[string]any{
				"hp": 100,
			},
		},
		"scope": map[string]any{
			"id": "solo-abc",
		},
	}
	a := New(data)
	paths := a.GetAllPaths()

	expected := []string{
		"entity", "entity.id", "entity.type", "entity.stats", "entity.stats.hp",
		"scope", "scope.id",
	}

	for _, exp := range expected {
		found := false
		for _, p := range paths {
			if p == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected path %q not found in GetAllPaths(): %v", exp, paths)
		}
	}
}

func TestAccessor_Set(t *testing.T) {
	data := map[string]any{
		"entity": map[string]any{
			"id": "player-123",
		},
	}
	a := New(data)

	// Установка нового вложенного значения
	success := a.Set("entity.stats.hp", 100)
	if !success {
		t.Error("Set should return true on success")
	}

	// Проверка что значение установлено
	hp, ok := a.GetInt("entity.stats.hp")
	if !ok || hp != 100 {
		t.Errorf("After Set: GetInt(entity.stats.hp) = %d (ok=%v), want 100", hp, ok)
	}

	// Установка в корень
	success = a.Set("new_root_key", "value")
	if !success {
		t.Error("Set at root should succeed")
	}
	val, ok := a.GetString("new_root_key")
	if !ok || val != "value" {
		t.Errorf("Get after Set at root: got %q (ok=%v), want value", val, ok)
	}
}

func TestAccessor_Delete(t *testing.T) {
	data := map[string]any{
		"entity": map[string]any{
			"id":   "player-123",
			"temp": "to_delete",
		},
	}
	a := New(data)

	// Удаление существующего ключа
	deleted := a.Delete("entity.temp")
	if !deleted {
		t.Error("Delete should return true for existing key")
	}

	// Проверка что ключ удалён
	_, exists := a.GetString("entity.temp")
	if exists {
		t.Error("Key should be deleted")
	}

	// Удаление несуществующего
	deleted = a.Delete("nonexistent")
	if deleted {
		t.Error("Delete should return false for nonexistent key")
	}
}

func TestAccessor_Clone(t *testing.T) {
	original := map[string]any{
		"entity": map[string]any{
			"id": "player-123",
			"meta": map[string]any{
				"level": 15,
			},
		},
	}
	a := New(original)
	cloned := a.Clone()

	// Модификация клона не должна влиять на оригинал
	cloned.Set("entity.meta.level", 99)

	origLevel, _ := a.GetInt("entity.meta.level")
	cloneLevel, _ := cloned.GetInt("entity.meta.level")

	if origLevel != 15 {
		t.Errorf("Original modified: got %d, want 15", origLevel)
	}
	if cloneLevel != 99 {
		t.Errorf("Clone not modified: got %d, want 99", cloneLevel)
	}
}

func TestParseDotPath(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"entity.id", []string{"entity", "id"}},
		{"a.b.c", []string{"a", "b", "c"}},
		{"items[0].name", []string{"items", "0", "name"}},
		{"data.nested[2].value", []string{"data", "nested", "2", "value"}},
		{".leading.dot", []string{"leading", "dot"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDotPath(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("parseDotPath(%q): len=%d, want %d", tt.input, len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseDotPath(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestArrayIndexAccess(t *testing.T) {
	data := map[string]any{
		"players": []any{
			map[string]any{"id": "p1", "name": "Alice"},
			map[string]any{"id": "p2", "name": "Bob"},
			map[string]any{"id": "p3", "name": "Charlie"},
		},
	}
	a := New(data)

	name, ok := a.GetString("players[1].name")
	if !ok || name != "Bob" {
		t.Errorf("GetString(players[1].name): got %q (ok=%v), want Bob", name, ok)
	}

	id, ok := a.GetString("players[0].id")
	if !ok || id != "p1" {
		t.Errorf("GetString(players[0].id): got %q (ok=%v), want p1", id, ok)
	}

	// Выход за границы
	_, ok = a.GetString("players[10].name")
	if ok {
		t.Error("Should return false for out-of-bounds array index")
	}

	// Неверный индекс
	_, ok = a.GetString("players[abc].name")
	if ok {
		t.Error("Should return false for non-numeric array index")
	}
}

func TestJSONUnmarshalIntegration(t *testing.T) {
	// Типичный кейс: данные из json.Unmarshal
	jsonStr := `{
		"entity": {
			"id": "player-123",
			"type": "player",
			"stats": {
				"hp": 100,
				"mp": 50.5
			},
			"inventory": ["sword", "potion"]
		},
		"active": true
	}`

	var data map[string]any
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	a := New(data)

	// Все геттеры должны работать с распарсенным JSON
	id, ok := a.GetString("entity.id")
	if !ok || id != "player-123" {
		t.Errorf("GetString(entity.id): got %q (ok=%v)", id, ok)
	}

	hp, ok := a.GetInt("entity.stats.hp")
	if !ok || hp != 100 {
		t.Errorf("GetInt(entity.stats.hp): got %d (ok=%v)", hp, ok)
	}

	mp, ok := a.GetFloat("entity.stats.mp")
	if !ok || mp != 50.5 {
		t.Errorf("GetFloat(entity.stats.mp): got %f (ok=%v)", mp, ok)
	}

	active, ok := a.GetBool("active")
	if !ok || !active {
		t.Errorf("GetBool(active): got %v (ok=%v)", active, ok)
	}

	inv, ok := a.GetSlice("entity.inventory")
	if !ok || len(inv) != 2 {
		t.Errorf("GetSlice(entity.inventory): got %d items (ok=%v)", len(inv), ok)
	}
}

func TestDeeplyNestedAccess(t *testing.T) {
	// Очень глубокая вложенность
	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": map[string]any{
						"e": map[string]any{
							"value": "deep",
						},
					},
				},
			},
		},
	}
	a := New(data)

	val, ok := a.GetString("a.b.c.d.e.value")
	if !ok || val != "deep" {
		t.Errorf("Deep access: got %q (ok=%v), want deep", val, ok)
	}
}

func TestNonMapRoot(t *testing.T) {
	// Accessor должен работать не только с map[string]any в корне
	// Например, с []any или примитивом

	// Примитив в корне
	prim := New("just_string")
	val, ok := prim.GetAny("")
	if !ok || val != "just_string" {
		t.Errorf("Root primitive: got %v (ok=%v)", val, ok)
	}

	// Слайс в корне
	slice := New([]any{"a", "b", "c"})
	item, ok := slice.GetString("[1]")
	if !ok || item != "b" {
		t.Errorf("Root slice access: got %q (ok=%v)", item, ok)
	}
}
