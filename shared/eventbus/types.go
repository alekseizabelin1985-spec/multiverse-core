package eventbus

import (
	"context"
	"maps"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	EventID   string             `json:"event_id"`
	EventType string             `json:"event_type"`
	Timestamp time.Time          `json:"timestamp"`
	Source    string             `json:"source"`
	WorldID   string             `json:"world_id"`
	ScopeID   *string            `json:"scope_id,omitempty"`
	Payload   map[string]any     `json:"payload"`
}

func NewEvent(eventType, source, worldID string, payload map[string]any) Event {
	if payload == nil {
		payload = make(map[string]any)
	}
	return Event{
		EventID:   uuid.NewString(),
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Source:    source,
		WorldID:   worldID,
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
