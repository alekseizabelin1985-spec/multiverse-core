// Package semanticmemory defines interfaces for semantic storage implementations.
package semanticmemory

import "context"

// SemanticStorage defines the interface for semantic storage implementations.
// This allows switching between different storage backends (e.g., ChromaDB HTTP API, ChromaDB Go client)
// while maintaining the same interface for the rest of the application.
type SemanticStorage interface {
	// UpsertDocument adds or updates a document in the semantic storage.
	UpsertDocument(ctx context.Context, entityID string, text string, metadata map[string]interface{}) error

	// GetDocuments retrieves documents by their IDs from the semantic storage.
	GetDocuments(ctx context.Context, entityIDs []string) (map[string]string, error)

	// SearchEventsByType searches for events by type in the semantic storage.
	SearchEventsByType(ctx context.Context, eventType string, limit int) ([]string, error)

	// QueryByMetadata retrieves documents matching the given metadata filter.
	// The where map supports equality filters on indexed metadata fields
	// (e.g., {"event_type": "player.move", "world_id": "world-1"}).
	// Returns a slice of maps, each containing "id", "document", and "metadata" keys.
	QueryByMetadata(ctx context.Context, where map[string]interface{}, limit int) ([]map[string]interface{}, error)

	// Close closes the connection to the semantic storage.
	Close() error
}
