// Explicit relation types for event-driven graph building: they let a
// producer declare the semantic edges of an event instead of leaving
// semantic-memory to guess them.

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
