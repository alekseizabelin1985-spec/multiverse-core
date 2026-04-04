// Package eventbus provides the event bus infrastructure for the multiverse-core system.
package eventbus

import (
	"fmt"
	"strings"
	"time"

	"multiverse-core.io/shared/jsonpath"
)

// SetNested устанавливает значение по dot-пути в map
// Пример: SetNested(payload, "weather.change.to", "шторм")
func SetNested(m map[string]any, path string, value any) {
	keys := parseDotPath(path)
	if len(keys) == 0 {
		return
	}

	// Рекурсивно создаём вложенные структуры
	for i := 0; i < len(keys)-1; i++ {
		currentKey := keys[i]

		currentVal, ok := m[currentKey]
		if !ok {
			m[currentKey] = make(map[string]any)
			currentVal = m[currentKey]
		}

		currentMap, ok := currentVal.(map[string]any)
		if !ok {
			// Если на пути есть не-map значение — перезаписываем
			m[currentKey] = make(map[string]any)
			currentMap = m[currentKey].(map[string]any)
		}

		m = currentMap
	}

	// Устанавливаем конечное значение
	finalKey := keys[len(keys)-1]
	m[finalKey] = value
}

// GetNested извлекает значение по dot-пути из map
// Пример: GetNested(payload, "weather.change.to")
func GetNested(m map[string]any, path string) (any, bool) {
	keys := parseDotPath(path)
	if len(keys) == 0 {
		return nil, false
	}

	current := m
	for _, key := range keys {
		val, ok := current[key]
		if !ok {
			return nil, false
		}

		// Если это последний ключ — возвращаем значение
		if key == keys[len(keys)-1] {
			return val, true
		}

		// Иначе продолжаем спускаться
		if nextMap, ok := val.(map[string]any); ok {
			current = nextMap
		} else {
			return nil, false
		}
	}

	return nil, false
}

// parseDotPath разбивает dot-пут на массив ключей
// Пример: "weather.change.in.region.id" -> ["weather", "change", "in", "region", "id"]
func parseDotPath(path string) []string {
	// Убираем ведущие точки
	path = strings.TrimLeft(path, ".")
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}

// EntityInfo содержит извлечённую информацию о сущности
type EntityInfo struct {
	ID    string
	Type  string
	Name  string
	World string
}

// ExtractEntityID извлекает ID сущности из payload
// Поддержка как новой структуры (entity.id), так и старой (entity_id)
func ExtractEntityID(payload map[string]any) *EntityInfo {
	// Проверяем новую структуру: entity.id
	if entity, ok := payload["entity"].(map[string]any); ok {
		if id, ok := entity["id"].(string); ok && id != "" {
			return &EntityInfo{
				ID:    id,
				Type:  getSafeString(entity, "type"),
				Name:  getSafeString(entity, "name"),
				World: getWorldID(entity),
			}
		}
	}

	// Fallback на старую структуру: entity_id, player_id, actor_id, character_id
	oldKeys := []string{"entity_id", "player_id", "actor_id", "character_id", "npc_id"}
	for _, key := range oldKeys {
		if id, ok := payload[key].(string); ok && id != "" {
			return &EntityInfo{
				ID:    id,
				Type:  getSafeString(payload, "entity_type"),
				Name:  getSafeString(payload, "entity_name"),
				World: getSafeString(payload, "world_id"),
			}
		}
	}

	return nil
}

// ExtractTargetEntityID извлекает ID целевой сущности из payload
// Поддержка новой (target.entity.id) и старой (target_id, npc_id, item_id) структур
func ExtractTargetEntityID(payload map[string]any) *EntityInfo {
	// Проверяем новую структуру: target.entity.id
	if target, ok := payload["target"].(map[string]any); ok {
		if entity, ok := target["entity"].(map[string]any); ok {
			if id, ok := entity["id"].(string); ok && id != "" {
				return &EntityInfo{
					ID:    id,
					Type:  getSafeString(entity, "type"),
					Name:  getSafeString(entity, "name"),
					World: getWorldID(entity),
				}
			}
		}
		// Fallback внутри target: target.id
		if id, ok := target["id"].(string); ok && id != "" {
			return &EntityInfo{
				ID:    id,
				Type:  getSafeString(target, "type"),
				Name:  getSafeString(target, "name"),
			}
		}
	}

	// Fallback на старые ключи
	oldKeys := []string{"target_id", "npc_id", "item_id", "region_id", "location_id", "quest_id"}
	for _, key := range oldKeys {
		if id, ok := payload[key].(string); ok && id != "" {
			return &EntityInfo{
				ID: id,
			}
		}
	}

	return nil
}

// ExtractSourceEntityID извлекает ID сущности-источника из payload
func ExtractSourceEntityID(payload map[string]any) *EntityInfo {
	// Проверяем новую структуру: source.entity.id
	if source, ok := payload["source"].(map[string]any); ok {
		if entity, ok := source["entity"].(map[string]any); ok {
			if id, ok := entity["id"].(string); ok && id != "" {
				return &EntityInfo{
					ID:    id,
					Type:  getSafeString(entity, "type"),
					Name:  getSafeString(entity, "name"),
					World: getWorldID(entity),
				}
			}
		}
	}

	return nil
}

// ExtractWorldID извлекает ID мира из payload
func ExtractWorldID(payload map[string]any) string {
	// Новая структура: world.id
	if world, ok := payload["world"].(map[string]any); ok {
		if id, ok := world["id"].(string); ok && id != "" {
			return id
		}
	}

	// Fallback: world_id
	if id, ok := payload["world_id"].(string); ok && id != "" {
		return id
	}

	return ""
}

// ExtractScope извлекает информацию о скоупе из payload (новая и старая структура)
func ExtractScope(payload map[string]any) *ScopeRef {
	// Используем универсальный jsonpath для извлечения с fallback
	acc := jsonpath.New(payload)
	
	// Новая структура: scope: { id, type }
	if scopeMap, ok := acc.GetMap("scope"); ok {
		ref := &ScopeRef{}
		if id, ok := scopeMap["id"].(string); ok && id != "" {
			ref.ID = id
		}
		if typ, ok := scopeMap["type"].(string); ok && typ != "" {
			ref.Type = typ
		}
		if ref.ID != "" || ref.Type != "" {
			return ref
		}
	}

	// Fallback: плоские ключи scope_id, scope_type
	ref := &ScopeRef{}
	if id, ok := payload["scope_id"].(string); ok && id != "" {
		ref.ID = id
	}
	if typ, ok := payload["scope_type"].(string); ok && typ != "" {
		ref.Type = typ
	}

	if ref.ID != "" || ref.Type != "" {
		return ref
	}

	return nil
}

// Type aliases для удобства — делегируют к универсальному jsonpath.Accessor
// Это сохраняет обратную совместимость кода, использующего eventbus.PathAccessor
type PathAccessor = jsonpath.Accessor

// NewPathAccessor создает аксессор для работы с payload (алиас на jsonpath.New)
func NewPathAccessor(data map[string]any) *PathAccessor {
	return jsonpath.New(data)
}

// Helper функции

func getSafeString(m map[string]any, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getWorldID(entity map[string]any) string {
	if world, ok := entity["world"].(map[string]any); ok {
		if id, ok := world["id"].(string); ok {
			return id
		}
	}
	return ""
}

// ToMapWithEntity создает event payload с entity в новой структуре
func ToMapWithEntity(entityID, entityType, worldID string, customFields map[string]any) map[string]interface{} {
	result := map[string]interface{}{
		"entity": map[string]interface{}{
			"id":     entityID,
			"type":   entityType,
			"world":  map[string]interface{}{"id": worldID},
		},
	}

	for k, v := range customFields {
		result[k] = v
	}

	return result
}

// FormatEventContext форматирует событие для LLM-контекста
// Пример: "{player_123:Вася} {event:вошел в} {region_forest123:Темный лес}"
func FormatEventContext(sourceID, sourceName, eventID, action, targetID, targetName string, timestamp time.Time) string {
	timeStr := timestamp.Format("15:04")
	return fmt.Sprintf("{%s:%s} {%s:%s} {%s:%s} {%s:%s}",
		eventID, timeStr,
		sourceID, sourceName,
		eventID, action,
		targetID, targetName)
}
