package eventbus

import (
	"context"
	"maps"
	"time"

	"github.com/google/uuid"
	"multiverse-core.io/shared/jsonpath"
)

type Event struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	// Deprecated: используйте payload.world.id вместо world_id в топ-уровне
	WorldID string `json:"world_id,omitempty"`
	// Deprecated: используйте payload.scope: {id, type} вместо scope_id в топ-уровне
	ScopeID *string        `json:"scope_id,omitempty"`
	Payload map[string]any `json:"payload"`
	// Relations declares explicit semantic edges for the knowledge graph (optional).
	// Produced by Oracle/GM/WorldGenerator, consumed by semantic-memory → Neo4j.
	Relations []Relation `json:"relations,omitempty"`
}

func NewEvent(eventType, source, worldID string, payload map[string]any) Event {
	if payload == nil {
		payload = make(map[string]any)
	}
	// Если world_id не задан в payload, добавляем его в иерархической структуре
	if _, hasWorld := payload["world"]; !hasWorld && worldID != "" {
		if _, hasFlat := payload["world_id"]; !hasFlat {
			payload["world"] = map[string]any{"id": worldID}
		}
	}
	return Event{
		EventID:   uuid.NewString(),
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Source:    source,
		WorldID:   worldID, // Оставляем для backward compatibility при чтении из топ-уровня
		Payload:   payload,
	}
}

// NewEventWithDescription creates an event with a pre-filled description in payload
func NewEventWithDescription(eventType, source, worldID, description string) Event {
	payload := map[string]any{
		"description": description,
	}
	return NewEvent(eventType, source, worldID, payload)
}

// NewStructuredEvent creates an event with typed payload using EventPayload builder
func NewStructuredEvent(eventType, source, worldID string, payload *EventPayload) Event {
	return NewEvent(eventType, source, worldID, payload.ToMap())
}

// PublishEntityCreated creates and publishes an entity.created event with new structured payload
func PublishEntityCreated(bus *EventBus, worldID, entityID, entityType, entityName string) {
	payload := NewEventPayload().
		WithEntity(entityID, entityType, entityName).
		WithWorld(worldID)

	event := NewStructuredEvent("entity.created", "entity-manager", worldID, payload)
	bus.Publish(context.TODO(), TopicSystemEvents, event)
}

// PublishEntityUpdated creates and publishes an entity.updated event with new structured payload
func PublishEntityUpdated(bus *EventBus, worldID, entityID, entityType, entityName string, customFields map[string]any) {
	payload := NewEventPayload().
		WithEntity(entityID, entityType, entityName).
		WithWorld(worldID)

	if customFields != nil {
		maps.Copy(payload.Custom, customFields)
	}

	event := NewStructuredEvent("entity.updated", "entity-manager", worldID, payload)
	bus.Publish(context.TODO(), TopicSystemEvents, event)
}

// PublishActionEvent creates and publishes a player.action event with new structured payload
func PublishActionEvent(bus *EventBus, worldID, entityID, action string, targetID, targetType, targetName string, customFields map[string]any) {
	payload := NewEventPayload().
		WithEntity(entityID, "player", "").
		WithTarget(targetID, targetType, targetName).
		WithWorld(worldID)

	payload.Custom["action"] = action

	if customFields != nil {
		maps.Copy(payload.Custom, customFields)
	}

	event := NewStructuredEvent("player.action", "entity-actor", worldID, payload)
	bus.Publish(context.TODO(), TopicPlayerEvents, event)
}

// GetWorldIDFromEvent извлекает world_id из события (поддержка новой и старой структуры)
func GetWorldIDFromEvent(event Event) string {
	// Сначала пробуем новую структуру в payload: world.id
	if worldID := ExtractWorldID(event.Payload); worldID != "" {
		return worldID
	}
	// Fallback на топ-уровень
	return event.WorldID
}

// GetScopeFromEvent извлекает scope из события (поддержка новой и старой структуры)
func GetScopeFromEvent(event Event) *ScopeRef {
	// Пробуем новую структуру в payload: scope: {id, type}
	if scope := ExtractScope(event.Payload); scope != nil && (scope.ID != "" || scope.Type != "") {
		return scope
	}
	// Fallback на топ-уровень: scope_id / scope_type
	if event.ScopeID != nil {
		return &ScopeRef{ID: *event.ScopeID}
	}
	return nil
}

// Path возвращает универсальный jsonpath.Accessor для доступа к данным события по dot-путям.
// Пример: event.Path().GetString("entity.id")
func (e *Event) Path() *jsonpath.Accessor {
	return jsonpath.New(e.Payload)
}
