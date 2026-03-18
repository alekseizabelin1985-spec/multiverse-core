// Package semanticmemory — entity query types and indexer methods.
package semanticmemory

import (
	"context"
	"fmt"
)

// EntityQuery describes filters for querying entities from the graph store.
// All fields are optional; omitting a field means "no filter on that field".
// Multiple non-empty fields are combined with AND logic.
type EntityQuery struct {
	// IDs filters entities to those whose ID appears in this list.
	IDs []string `json:"ids,omitempty"`

	// Type filters entities by their type field (exact match, e.g. "player", "npc", "item").
	Type string `json:"type,omitempty"`

	// WorldID filters entities that belong to a specific world.
	WorldID string `json:"world_id,omitempty"`

	// Name filters entities whose name contains this substring (case-insensitive).
	Name string `json:"name,omitempty"`

	// Limit caps the number of returned results. Defaults to 20 when <= 0.
	Limit int `json:"limit,omitempty"`
}

// GetEntityByID returns the entity with the given ID, or nil if not found.
func (i *Indexer) GetEntityByID(ctx context.Context, entityID string) (*EntityInfo, error) {
	if entityID == "" {
		return nil, fmt.Errorf("GetEntityByID: entityID cannot be empty")
	}
	cache, err := i.neo4j.GetEntityCache([]string{entityID})
	if err != nil {
		return nil, err
	}
	if info, ok := cache[entityID]; ok {
		return &info, nil
	}
	return nil, nil
}

// QueryEntities returns entities matching the provided filter from the graph store.
// Returns an empty slice (not an error) when no entities match.
func (i *Indexer) QueryEntities(ctx context.Context, q EntityQuery) ([]EntityInfo, error) {
	return i.neo4j.QueryEntities(q)
}
