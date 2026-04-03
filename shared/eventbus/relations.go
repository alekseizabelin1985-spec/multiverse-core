// Package eventbus provides explicit relation types for event-driven graph building.
// Relations allow event producers to declare semantic edges in Neo4j without heuristics.
package eventbus

import (
	"fmt"
)

// Relation declares a typed edge between two entities in the knowledge graph.
// Produced by Oracle/GM/WorldGenerator, consumed by semantic-memory → Neo4j.
type Relation struct {
	// From is the source entity ID (recommended format: "type:id", e.g. "player:p123")
	From string `json:"from"`

	// To is the target entity ID
	To string `json:"to"`

	// Type is the semantic edge type (e.g. "FOUND", "LOCATED_IN", "CONTAINS")
	Type string `json:"type"`

	// Directed controls whether the edge is one-way (true) or bidirectional (false)
	Directed bool `json:"directed"`

	// Metadata carries optional edge properties (action, timestamp, confidence, etc.)
	Metadata map[string]any `json:"metadata,omitempty"`
}

// EventWithRelations wraps an Event with explicit relations for the knowledge graph.
// Use this builder when an event producer knows which entities are connected.
type EventWithRelations struct {
	Event     Event
	Relations []Relation
}

// WithRelations attaches explicit relations to an event.
//
// Usage:
//
//	ev := eventbus.NewEvent("player.action", "oracle", worldID, payload)
//	wrapper := eventbus.WithRelations(ev, []eventbus.Relation{
//	    {From: "player:p1", To: "item:sword_1", Type: eventbus.RelFound, Directed: true},
//	})
//	bus.Publish(ctx, topic, wrapper.Event)
func WithRelations(ev Event, relations []Relation) EventWithRelations {
	return EventWithRelations{
		Event:     ev,
		Relations: relations,
	}
}

// AddRelation appends a single relation to the wrapper.
func (w EventWithRelations) AddRelation(rel Relation) EventWithRelations {
	w.Relations = append(w.Relations, rel)
	return w
}

// ValidateEventRelations checks that all relations in the event are well-formed.
// Returns the first validation error found, or nil if all relations are valid.
// Events without relations are always valid (relations are optional).
//
// Validation rules:
//   - From must be non-empty
//   - To must be non-empty
//   - Type must be non-empty
func ValidateEventRelations(ev Event) error {
	return ValidateRelations(ev.Relations)
}

// ValidateRelations validates a slice of relations.
func ValidateRelations(relations []Relation) error {
	for i, rel := range relations {
		if rel.From == "" {
			return fmt.Errorf("relation[%d]: 'from' must not be empty", i)
		}
		if rel.To == "" {
			return fmt.Errorf("relation[%d]: 'to' must not be empty", i)
		}
		if rel.Type == "" {
			return fmt.Errorf("relation[%d]: 'type' must not be empty", i)
		}
	}
	return nil
}
