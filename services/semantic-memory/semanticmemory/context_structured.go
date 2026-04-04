// Package semanticmemory handles structured context building with embedded IDs for AI.
package semanticmemory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"multiverse-core.io/shared/eventbus"
)

// StructuredContext представляет контекст с встроенными ID для AI
// Формат контекста: "{entity.id:name} {action} {target.entity.id:name}"
type StructuredContext struct {
	Context  string                `json:"context"`
	Entities map[string]EntityInfo `json:"entities"`
	Timeline []TimelineEvent       `json:"timeline"`
	Metadata ContextMetadata       `json:"metadata"`
}

// EntityInfo содержит информацию о сущности
type EntityInfo struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	WorldID     string         `json:"world_id"`
	Description string         `json:"description,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	// Coordinates хранит position объект из payload: {x: float, y: float, z: float}
	Coordinates *Coordinates `json:"coordinates,omitempty"`
}

// WorldInfo содержит расширенную информацию о мире
type WorldInfo struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	WorldID     string                 `json:"world_id"`
	Description string                 `json:"description,omitempty"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	// Coordinates хранит position объект из payload: {x: float, y: float, z: float}
	Coordinates *Coordinates `json:"coordinates,omitempty"`
}

// Coordinates представляет координаты сущности в пространстве
type Coordinates struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// TimelineEvent представляет событие в хронологии
// Формат: "{entity.id:name} {event_type:time} {action} {target.entity.id:name}"
type TimelineEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventID   string    `json:"event_id"`
	Type      string    `json:"type"`
	Format    string    `json:"format"`
	Entities  []string  `json:"entities"` // Все entity.id участников события
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

// FormatWithIDs форматирует событие в строку с встроенными ID для LLM
// Формат: "{entity.id:type:name} {timestamp} {action} {target.entity.id:type:name}"
// Пример: "{player-123:player:Вася} {14:30} {вошел в} {region-456:region:Темный лес}"
func FormatWithIDs(sourceID, sourceType, sourceName, eventID, action, targetID, targetType, targetName string, timestamp time.Time) string {
	timeStr := timestamp.Format("15:04")

	// Форматирование с типом сущности для явной семантики
	sourceFormat := fmt.Sprintf("{%s:%s:%s}", sourceID, sourceType, sourceName)
	targetFormat := fmt.Sprintf("{%s:%s:%s}", targetID, targetType, targetName)

	return fmt.Sprintf("%s {%s:%s} {%s} %s",
		sourceFormat, eventID, timeStr, action, targetFormat)
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
		// Извлекаем участников события с поддержкой новой структуры (entity.id) и fallback
		sourceEntityID := extractStructuredEntityID(ev)
		targetEntityID := extractStructuredTargetEntityID(ev)

		if sourceEntityID == "" {
			continue
		}

		// Получаем информацию о сущности из кэша
		sourceInfo, hasSource := entityCache[sourceEntityID]
		if !hasSource {
			sourceInfo = EntityInfo{ID: sourceEntityID, Name: sourceEntityID}
		}

		var targetInfo EntityInfo
		if targetEntityID != "" {
			targetInfo, _ = entityCache[targetEntityID]
		}

		// Определяем действие на основе типа события
		action := getActionFromEventType(ev.Type)

		// Форматируем строку с ID для LLM
		// Формат: "{entity.id:type:name} {timestamp} {action} {target.entity.id:type:name}"
		sourceName := sourceInfo.Name
		sourceType := sourceInfo.Type
		if sourceType == "" {
			sourceType = "unknown"
		}

		formatted := ""
		if targetEntityID != "" && targetInfo.Name != "" {
			targetName := targetInfo.Name
			targetType := targetInfo.Type
			if targetType == "" {
				targetType = "unknown"
			}
			formatted = FormatWithIDs(sourceEntityID, sourceType, sourceName, ev.ID, action, targetEntityID, targetType, targetName, ev.Timestamp)
		} else {
			// Формат без цели (например, для world events)
			formatted = fmt.Sprintf("{%s:%s:%s} {%s:%s} {%s:%s}",
				sourceEntityID, sourceType, sourceName,
				ev.ID, ev.Timestamp.Format("15:04"),
				ev.ID, action)
		}

		contextParts = append(contextParts, formatted)

		// Добавляем в timeline
		timeline = append(timeline, TimelineEvent{
			Timestamp: ev.Timestamp,
			EventID:   ev.ID,
			Type:      ev.Type,
			Format:    formatted,
			Entities:  []string{sourceEntityID, targetEntityID},
		})

		// Отмечаем сущности
		entitySet[sourceEntityID] = true
		if targetEntityID != "" {
			entitySet[targetEntityID] = true
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

// extractStructuredEntityID извлекает ID сущности
// Поддерживает новый формат (entity.id) и старый (entity_id, player_id, actor_id)
func extractStructuredEntityID(ev eventbus.Event) string {
	entity := eventbus.ExtractEntityID(ev.Payload)
	if entity != nil {
		return entity.ID
	}
	return ""
}

// extractStructuredTargetEntityID извлекает ID целевой сущности
// Поддерживает новый формат (target.entity.id) и старый (target_id, npc_id, item_id)
func extractStructuredTargetEntityID(ev eventbus.Event) string {
	entity := eventbus.ExtractTargetEntityID(ev.Payload)
	if entity != nil {
		return entity.ID
	}
	return ""
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
