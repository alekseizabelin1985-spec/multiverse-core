package eventbus

import (
	"context"
	"maps"
	"time"

	"github.com/google/uuid"
	"multiverse-core.io/shared/jsonpath"
)

type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Source    string         `json:"source"`
	World     *WorldRef      `json:"world,omitempty"`
	Scope     *ScopeRef      `json:"scope,omitempty"`
	Payload   map[string]any `json:"payload"`
	// Relations declares explicit semantic edges for the knowledge graph (optional).
	// Produced by Oracle/GM/WorldGenerator, consumed by semantic-memory → Neo4j.
	Relations []Relation `json:"relations,omitempty"`
}

func NewEvent(eventType, source, worldID string, payload map[string]any) Event {
	if payload == nil {
		payload = make(map[string]any)
	}

	var worldRef *WorldRef
	if worldID != "" {
		worldRef = &WorldRef{Entity: EntityRef{ID: worldID, Type: "world"}}
	}

	return Event{
		ID:        uuid.NewString(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Source:    source,
		World:     worldRef,
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

// GetWorldIDFromEvent извлекает world.entity.id из события
func GetWorldIDFromEvent(event Event) string {
	// Топ-уровень World
	if event.World != nil {
		return event.World.Entity.ID
	}
	// Fallback на payload.world.entity.id
	if worldID := ExtractWorldID(event.Payload); worldID != "" {
		return worldID
	}
	return ""
}

// GetScopeFromEvent извлекает scope из события
func GetScopeFromEvent(event Event) *ScopeRef {
	// Сначала пробуем топ-уровень Scope
	if event.Scope != nil {
		return event.Scope
	}
	// Fallback на payload.scope
	if scope := ExtractScope(event.Payload); scope != nil && (scope.ID != "" || scope.Type != "") {
		return scope
	}
	return nil
}

// Path возвращает универсальный jsonpath.Accessor для доступа к данным события по dot-путям.
// Пример: event.Path().GetString("entity.id")
func (e *Event) Path() *jsonpath.Accessor {
	return jsonpath.New(e.Payload)
}

// GetEntityIDWithFallback извлекает entity ID из события с цепочкой fallback.
// Проверяет: entity.entity.id → entity.id → entity_id/player_id/actor_id/npc_id
// Возвращает (*EntityInfo, true) если найден или (nil, false) если нет.
func (e *Event) GetEntityIDWithFallback() (*EntityInfo, bool) {
	info := ExtractEntityID(e.Payload)
	return info, info != nil
}

// GetTargetEntityID извлекает target entity ID из события с цепочкой fallback.
// Проверяет: target.entity.id → target.id → target_id/npc_id
// Возвращает (*EntityInfo, true) если найден или (nil, false) если нет.
func (e *Event) GetTargetEntityID() (*EntityInfo, bool) {
	info := ExtractTargetEntityID(e.Payload)
	return info, info != nil
}
