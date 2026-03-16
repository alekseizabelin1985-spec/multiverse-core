// Package semanticmemory handles structured context building with embedded IDs for AI.
package semanticmemory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"multiverse-core/internal/eventbus"
)

// StructuredContext представляет контекст с встроенными ID для AI
type StructuredContext struct {
	Context  string                 `json:"context"`  // "{player_123:Вася} {event_456:вошел в} {region_forest123:Темный лес}"
	Entities map[string]EntityInfo  `json:"entities"` // Детали сущностей
	Timeline []TimelineEvent        `json:"timeline"` // Хронология событий
	Metadata ContextMetadata        `json:"metadata"` // Метаданные запроса
}

// EntityInfo содержит информацию о сущности
type EntityInfo struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	WorldID     string                 `json:"world_id"`
	Description string                 `json:"description,omitempty"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}

// TimelineEvent представляет событие в хронологии
type TimelineEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventID   string    `json:"event_id"`
	Type      string    `json:"type"`
	Format    string    `json:"format"`   // "{event_id:time} {source_id:name} {event_id:action} {target_id:name}"
	Entities  []string  `json:"entities"` // IDs всех участвующих сущностей
}

// ContextMetadata содержит метаданные контекста
type ContextMetadata struct {
	RequestTime   string   `json:"request_time"`
	EntityCount   int      `json:"entity_count"`
	EventCount    int      `json:"event_count"`
	TimeRange     string   `json:"time_range"`
	ProcessingMs  int64    `json:"processing_ms"`
	SourceIndexes []string `json:"source_indexes"` // ["chroma:world_memory", "neo4j:entity_graph"]
}

// FormatWithIDs форматирует событие в строку с встроенными ID
func FormatWithIDs(sourceID, sourceName, eventID, action, targetID, targetName string, timestamp time.Time) string {
	timeStr := timestamp.Format("15:04")
	return fmt.Sprintf("{%s:%s} {%s:%s} {%s:%s} {%s:%s}",
		eventID, timeStr,
		sourceID, sourceName,
		eventID, action,
		targetID, targetName)
}

// BuildStructuredContext строит контекст из событий с ID
func (i *Indexer) BuildStructuredContext(events []eventbus.Event, entityCache map[string]EntityInfo) StructuredContext {
	// Сортируем события по времени
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	var timeline []TimelineEvent
	var contextParts []string
	entitySet := make(map[string]bool)

	for _, ev := range events {
		// Извлекаем участников события
		sourceID := extractEntityID(ev, "entity_id")
		targetID := extractTargetEntityID(ev)

		if sourceID == "" {
			continue
		}

		// Получаем имена из кэша или fallback на ID
		sourceName := getEntityName(entityCache, sourceID)
		targetName := getEntityName(entityCache, targetID)

		// Определяем действие на основе типа события
		action := getActionFromEventType(ev.EventType)

		// Форматируем строку с ID
		formatted := ""
		if targetID != "" && targetName != "" {
			formatted = FormatWithIDs(sourceID, sourceName, ev.EventID, action, targetID, targetName, ev.Timestamp)
		} else {
			// Формат без цели (например, для world events)
			formatted = fmt.Sprintf("{%s:%s} {%s:%s} {%s:%s}",
				ev.EventID, ev.Timestamp.Format("15:04"),
				sourceID, sourceName,
				ev.EventID, action)
		}

		contextParts = append(contextParts, formatted)

		// Добавляем в timeline
		timeline = append(timeline, TimelineEvent{
			Timestamp: ev.Timestamp,
			EventID:   ev.EventID,
			Type:      ev.EventType,
			Format:    formatted,
			Entities:  []string{sourceID, targetID},
		})

		// Отмечаем сущности
		entitySet[sourceID] = true
		if targetID != "" {
			entitySet[targetID] = true
		}
	}

	// Формируем финальный контекст
	contextText := strings.Join(contextParts, ". ")
	if contextText != "" {
		contextText += "."
	}

	// Собираем метаданные
	metadata := ContextMetadata{
		RequestTime:   time.Now().Format(time.RFC3339),
		EntityCount:   len(entitySet),
		EventCount:    len(events),
		SourceIndexes: []string{"chroma:world_memory", "neo4j:entity_graph"},
	}

	return StructuredContext{
		Context:  contextText,
		Entities: entityCache,
		Timeline: timeline,
		Metadata: metadata,
	}
}

// extractEntityID извлекает ID сущности из payload по ключу
func extractEntityID(ev eventbus.Event, key string) string {
	if val, ok := ev.Payload[key].(string); ok {
		return val
	}
	return ""
}

// extractTargetEntityID извлекает ID целевой сущности из события
func extractTargetEntityID(ev eventbus.Event) string {
	// Пробуем разные возможные ключи для целевой сущности
	keys := []string{"target_entity_id", "npc_id", "item_id", "region_id", "location_id", "quest_id"}
	for _, key := range keys {
		if val, ok := ev.Payload[key].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

// getEntityName получает имя сущности из кэша или возвращает ID
func getEntityName(cache map[string]EntityInfo, entityID string) string {
	if entityID == "" {
		return "unknown"
	}
	if info, ok := cache[entityID]; ok && info.Name != "" {
		return info.Name
	}
	return entityID // fallback на ID
}

// getActionFromEventType определяет действие на основе типа события
func getActionFromEventType(eventType string) string {
	actions := map[string]string{
		"entity.moved":      "entered",
		"entity.interacted": "interacted_with",
		"item.found":        "found",
		"item.used":         "used",
		"item.lost":         "lost",
		"npc.met":           "met",
		"npc.talked":        "talked_to",
		"combat.started":    "fought",
		"combat.ended":      "defeated",
		"quest.accepted":    "accepted_quest",
		"quest.completed":   "completed_quest",
		"dialogue.started":  "started_dialogue",
		"dialogue.ended":    "ended_dialogue",
		"world.storm":       "experienced_storm",
		"world.war":         "declared_war",
	}
	if action, ok := actions[eventType]; ok {
		return action
	}
	// fallback: преобразуем event.type в readable action
	return strings.ReplaceAll(eventType, ".", "_")
}

// FilterEventsByTimeRange фильтрует события по временному диапазону
func FilterEventsByTimeRange(events []eventbus.Event, startTime, endTime time.Time) []eventbus.Event {
	var filtered []eventbus.Event
	for _, ev := range events {
		if (ev.Timestamp.After(startTime) || ev.Timestamp.Equal(startTime)) &&
			(ev.Timestamp.Before(endTime) || ev.Timestamp.Equal(endTime)) {
			filtered = append(filtered, ev)
		}
	}
	return filtered
}

// LimitEvents ограничивает количество событий
func LimitEvents(events []eventbus.Event, limit int) []eventbus.Event {
	if limit <= 0 || limit > len(events) {
		return events
	}
	return events[:limit]
}
