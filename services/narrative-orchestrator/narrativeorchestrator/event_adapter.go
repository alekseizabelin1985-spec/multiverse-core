// services/narrativeorchestrator/event_adapter.go

package narrativeorchestrator

import (
	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/eventbus"
	"time"
)

// EventBusToAgentEvent конвертирует eventbus.Event в agent.Event
func EventBusToAgentEvent(ev eventbus.Event) agent.Event {
	return agent.Event{
		Type:      ev.Type,
		ID:        ev.ID,
		Timestamp: ev.Timestamp,
		Payload:   ev.Payload,
		ScopeID:   eventbus.GetScopeFromEvent(ev).ID,
	}
}

// AgentEventToBusEvent конвертирует agent.Event в eventbus.Event
func AgentEventToBusEvent(ev agent.Event) eventbus.Event {
	return eventbus.Event{
		Type:      ev.Type,
		ID:        ev.ID,
		Timestamp: ev.Timestamp,
		Payload:   ev.Payload,
		Source:    "narrative-orchestrator",
	}
}

// AgentActionToBusEvent конвертирует agent.Action в eventbus.Event
func AgentActionToBusAction(action agent.Action) map[string]interface{} {
	payload := map[string]interface{}{
		"action_type": action.Type,
		"target":      action.Target,
		"priority":    action.Priority,
	}
	for k, v := range action.Payload {
		payload[k] = v
	}
	return payload
}

// BuildAgentEvent создает agent.Event из параметров
func BuildAgentEvent(eventType, eventID, scopeID string, payload map[string]interface{}) agent.Event {
	return agent.Event{
		Type:      eventType,
		ID:        eventID,
		Timestamp: time.Now(),
		Payload:   payload,
		ScopeID:   scopeID,
	}
}

// BuildBusEvent создает eventbus.Event из параметров
func BuildBusEvent(eventType, eventID, worldID, scopeID string, payload map[string]interface{}) eventbus.Event {
	ev := eventbus.NewEvent(eventType, "narrative-orchestrator", worldID, payload)
	eventbus.SetNested(ev.Payload, "scope.id", scopeID)
	return ev
}
