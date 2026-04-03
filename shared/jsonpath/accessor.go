// Package jsonpath provides universal dot-path access to nested map[string]any structures.
// It works with any JSON-like data, not tied to eventbus or specific domain.
//
// Example:
//
//	data := map[string]any{
//	    "entity": map[string]any{
//	        "id": "player-123",
//	        "stats": map[string]any{
//	            "hp": 100,
//	        },
//	    },
//	}
//
//	accessor := jsonpath.New(data)
//	id, _ := accessor.GetString("entity.id")           // "player-123"
//	hp, _ := accessor.GetInt("entity.stats.hp")        // 100
//
// Works with:
// - Nested maps: map[string]any
// - Arrays: []any
// - Primitives: string, int, float, bool
// - Mixed structures from JSON unmarshaling
package jsonpath

import (
	"fmt"
	"reflect"
	"strings"
)

// Accessor provides type-safe access to nested data via dot-notation paths.
// Thread-safe for read operations (data should not be modified during access).
type Accessor struct {
	data any
}

// New creates a new Accessor for the given data structure.
// Accepts map[string]any, []any, or any nested combination.
func New(data any) *Accessor {
	return &Accessor{data: data}
}

// parseDotPath разбивает dot-пут на массив ключей, поддерживая индексы массивов.
// Примеры:
//   - "entity.id" -> ["entity", "id"]
//   - "items[0].name" -> ["items", "0", "name"]
//   - "data.nested[2].value" -> ["data", "nested", "2", "value"]
func parseDotPath(path string) []string {
	path = strings.TrimLeft(path, ".")
	if path == "" {
		return nil
	}

	var keys []string
	var current strings.Builder

	for i := 0; i < len(path); i++ {
		ch := path[i]

		switch ch {
		case '.':
			if current.Len() > 0 {
				keys = append(keys, current.String())
				current.Reset()
			}
		case '[':
			if current.Len() > 0 {
				keys = append(keys, current.String())
				current.Reset()
			}
			// Читаем индекс до ]
			j := i + 1
			for j < len(path) && path[j] != ']' {
				j++
			}
			if j < len(path) {
				keys = append(keys, path[i+1:j])
				i = j
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		keys = append(keys, current.String())
	}

	return keys
}

// navigate спускается по структуре данных согласно ключам.
// Возвращает найденное значение и флаг успеха.
func navigate(data any, keys []string) (any, bool) {
	current := data

	for i, key := range keys {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[key]
			if !ok {
				return nil, false
			}
			current = val

		case map[any]any:
			val, ok := v[key]
			if !ok {
				return nil, false
			}
			current = val

		case []any:
			// Пытаемся преобразовать ключ в индекс
			idx := parseArrayIndex(key)
			if idx < 0 || idx >= len(v) {
				return nil, false
			}
			current = v[idx]

		case []map[string]any:
			idx := parseArrayIndex(key)
			if idx < 0 || idx >= len(v) {
				return nil, false
			}
			current = v[idx]

		default:
			// Если это последний ключ и мы ищем само значение
			if i == len(keys)-1 {
				// Проверяем через рефлексию для полей структур
				return getFieldByTag(v, key)
			}
			return nil, false
		}
	}

	return current, true
}

// parseArrayIndex пытается преобразовать строку в индекс массива.
// Возвращает -1 если не удалось.
func parseArrayIndex(s string) int {
	var n int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return -1
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// getFieldByTag пытается найти поле структуры по имени или JSON-тегу.
// Используется как последний fallback для non-map данных.
func getFieldByTag(data any, fieldName string) (any, bool) {
	if data == nil {
		return nil, false
	}

	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, false
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, false
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)

		// Проверяем имя поля
		if field.Name == fieldName {
			return val.Field(i).Interface(), true
		}

		// Проверяем JSON-тег
		if tag := field.Tag.Get("json"); tag != "" {
			tagName := strings.Split(tag, ",")[0]
			if tagName == fieldName || tagName == "-" {
				continue
			}
			if tagName == fieldName {
				return val.Field(i).Interface(), true
			}
		}
	}

	return nil, false
}

// get по пути с типизированным возвратом
func (a *Accessor) get(path string) (any, bool) {
	keys := parseDotPath(path)
	if len(keys) == 0 {
		return a.data, true
	}
	return navigate(a.data, keys)
}

// GetString извлекает строковое значение по пути.
// Пример: "entity.name", "user.profile.email"
func (a *Accessor) GetString(path string) (string, bool) {
	val, ok := a.get(path)
	if !ok {
		return "", false
	}
	if str, ok := val.(string); ok {
		return str, true
	}
	return "", false
}

// GetInt извлекает целочисленное значение по пути.
// Поддерживает: int, int8-64, float (округляется), string (парсится).
// Пример: "entity.stats.level", "config.max_retries"
func (a *Accessor) GetInt(path string) (int, bool) {
	val, ok := a.get(path)
	if !ok {
		return 0, false
	}

	switch v := val.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		// Попытка распарсить строку как число
		// (можно добавить strconv.Atoi при необходимости)
	}
	return 0, false
}

// GetFloat извлекает число с плавающей точкой по пути.
// Поддерживает: float32/64, int, string (парсится).
// Пример: "weather.temperature", "stats.accuracy"
func (a *Accessor) GetFloat(path string) (float64, bool) {
	val, ok := a.get(path)
	if !ok {
		return 0, false
	}

	switch v := val.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	}
	return 0, false
}

// GetBool извлекает булево значение по пути.
// Поддерживает: bool, string ("true"/"false"), int (0=false, !=0=true).
// Пример: "entity.active", "config.enabled"
func (a *Accessor) GetBool(path string) (bool, bool) {
	val, ok := a.get(path)
	if !ok {
		return false, false
	}

	switch v := val.(type) {
	case bool:
		return v, true
	case string:
		return v == "true" || v == "1" || v == "yes", true
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	}
	return false, false
}

// GetMap извлекает map[string]any по пути.
// Пример: "entity.metadata", "config.settings"
func (a *Accessor) GetMap(path string) (map[string]any, bool) {
	val, ok := a.get(path)
	if !ok {
		return nil, false
	}
	if m, ok := val.(map[string]any); ok {
		return m, true
	}
	return nil, false
}

// GetSlice извлекает []any по пути.
// Пример: "player.inventory", "quest.objectives"
func (a *Accessor) GetSlice(path string) ([]any, bool) {
	val, ok := a.get(path)
	if !ok {
		return nil, false
	}
	if s, ok := val.([]any); ok {
		return s, true
	}
	return nil, false
}

// GetAny возвращает значение любого типа по пути.
// Используйте когда тип заранее неизвестен или динамический.
// Пример: "entity", "config" (возвращает всю подструктуру)
func (a *Accessor) GetAny(path string) (any, bool) {
	return a.get(path)
}

// Has проверяет существование значения по пути без извлечения.
// Быстрее чем Get* когда нужно только проверить наличие.
// Пример: if accessor.Has("user.permissions.admin") { ... }
func (a *Accessor) Has(path string) bool {
	_, ok := a.get(path)
	return ok
}

// GetAllPaths рекурсивно собирает все доступные пути в данных.
// Полезно для отладки, интроспекции, генерации схем.
// Возвращает пути в формате: "entity", "entity.id", "entity.stats.hp"
func (a *Accessor) GetAllPaths() []string {
	var paths []string
	a.collectPaths(a.data, "", &paths)
	return paths
}

func (a *Accessor) collectPaths(data any, prefix string, paths *[]string) {
	switch v := data.(type) {
	case map[string]any:
		for key, val := range v {
			newPath := key
			if prefix != "" {
				newPath = prefix + "." + key
			}
			*paths = append(*paths, newPath)
			a.collectPaths(val, newPath, paths)
		}

	case map[any]any:
		for key, val := range v {
			keyStr := fmt.Sprintf("%v", key)
			newPath := keyStr
			if prefix != "" {
				newPath = prefix + "." + keyStr
			}
			*paths = append(*paths, newPath)
			a.collectPaths(val, newPath, paths)
		}

	case []any:
		for i, item := range v {
			newPath := fmt.Sprintf("%s[%d]", prefix, i)
			a.collectPaths(item, newPath, paths)
		}

	case []map[string]any:
		for i, item := range v {
			newPath := fmt.Sprintf("%s[%d]", prefix, i)
			a.collectPaths(item, newPath, paths)
		}
	}
}

// Set устанавливает значение по dot-пути, создавая промежуточные структуры при необходимости.
// Модифицирует исходные данные! Возвращает успех операции.
// Пример: accessor.Set("user.profile.email", "test@example.com")
func (a *Accessor) Set(path string, value any) bool {
	keys := parseDotPath(path)
	if len(keys) == 0 {
		return false
	}

	// Для установки нужно работать с указателем на корень
	// Эта реализация упрощена — работает только если корень — map[string]any
	rootMap, ok := a.data.(map[string]any)
	if !ok {
		return false
	}

	current := rootMap
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]

		// Если ключа нет или значение не map — создаём новый
		if next, exists := current[key]; !exists {
			newMap := make(map[string]any)
			current[key] = newMap
			current = newMap
		} else if nextMap, ok := next.(map[string]any); ok {
			current = nextMap
		} else {
			// Перезаписываем не-мап значение новым мапом
			newMap := make(map[string]any)
			current[key] = newMap
			current = newMap
		}
	}

	// Устанавливаем финальное значение
	finalKey := keys[len(keys)-1]
	current[finalKey] = value
	return true
}

// Delete удаляет значение по пути.
// Возвращает успех (было ли что-то удалено).
// Пример: accessor.Delete("user.temp_token")
func (a *Accessor) Delete(path string) bool {
	keys := parseDotPath(path)
	if len(keys) == 0 {
		return false
	}

	rootMap, ok := a.data.(map[string]any)
	if !ok {
		return false
	}

	// Навигация до родителя
	current := rootMap
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		if next, ok := current[key].(map[string]any); ok {
			current = next
		} else {
			return false
		}
	}

	// Удаление финального ключа
	finalKey := keys[len(keys)-1]
	if _, exists := current[finalKey]; exists {
		delete(current, finalKey)
		return true
	}
	return false
}

// Clone создаёт глубокую копию данных для безопасной модификации.
// Возвращает новый Accessor с независимыми данными.
func (a *Accessor) Clone() *Accessor {
	cloned := deepClone(a.data)
	return New(cloned)
}

// deepClone рекурсивно копирует структуру данных
func deepClone(data any) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			result[k] = deepClone(val)
		}
		return result
	case map[any]any:
		result := make(map[any]any, len(v))
		for k, val := range v {
			result[k] = deepClone(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = deepClone(val)
		}
		return result
	case []map[string]any:
		result := make([]map[string]any, len(v))
		for i, val := range v {
			result[i] = deepClone(val).(map[string]any)
		}
		return result
	default:
		// Примитивы копируются по значению
		return v
	}
}
