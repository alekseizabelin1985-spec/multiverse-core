// Package semanticmemory handles event processing and indexing.
package semanticmemory

import (
	"context"
	"fmt"
	"os"
	"testing"

	"multiverse-core.io/shared/eventbus"
)

func TestIndexer_HandleEvent(t *testing.T) {
	// Create a new indexer
	indexer, err := NewIndexer()
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}
	defer indexer.neo4j.Close()

	// Create a test event
	event := eventbus.NewEvent("player.action.move", "player-1", "test-world", map[string]interface{}{
		"action": "move",
		"from":   "location-a",
		"to":     "location-b",
		"scope":  map[string]interface{}{"id": "test-scope"},
	})
	event.ID = "test-event-1"
	event.Scope = &eventbus.ScopeRef{ID: "test-scope"}

	// Handle the event
	indexer.HandleEvent(event)

	// Verify that the event was saved
	ctx := context.Background()
	events, err := indexer.GetEventsByType(ctx, "player.action.move", 10)
	if err != nil {
		t.Fatalf("Failed to retrieve events: %v", err)
	}

	if len(events) == 0 {
		t.Error("No events were retrieved")
	}
}

func TestNeo4jGraphMode(t *testing.T) {
	// Create a new indexer
	indexer, err := NewIndexer()
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}
	defer indexer.neo4j.Close()

	// Create a test event with entity references
	event := eventbus.NewEvent("player.action.attack", "player-1", "test-world", map[string]interface{}{
		"action":    "attack",
		"player_id": "player-1",
		"target_id": "enemy-1",
		"source_id": "hero-2",
		"damage":    10,
		"inventory": []string{"sword-1", "shield-2"},
		"scope":     map[string]interface{}{"id": "test-scope"},
	})
	event.ID = "neo4j-test-event-1"
	event.Scope = &eventbus.ScopeRef{ID: "test-scope"}

	// Handle the event
	indexer.HandleEvent(event)

	// Test: Get events by entity via graph relationships
	events, err := indexer.neo4j.GetEventsByEntity("player-1", 10)
	if err != nil {
		t.Errorf("GetEventsByEntity failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("No events found for player-1 via graph relationships")
	}

	// Test: Get events by world and type
	events, err = indexer.neo4j.GetEventsByWorldAndType("test-world", "player.action.attack", 10)
	if err != nil {
		t.Errorf("GetEventsByWorldAndType failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("No events found for test-world with type player.action.attack")
	}

	// Test: Verify event was saved with correct properties
	eventData, err := indexer.neo4j.GetEventByID("neo4j-test-event-1")
	if err != nil {
		t.Errorf("GetEventByID failed: %v", err)
	}
	if eventData == nil {
		t.Error("Event not found in Neo4j")
	} else if eventData.Type != "player.action.attack" {
		t.Errorf("Expected event type 'player.action.attack', got '%s'", eventData.Type)
	}
	if eventData.World == nil || eventData.World.Entity.ID != "test-world" {
		t.Errorf("Expected world_id 'test-world', got '%v'", eventData.World)
	}

	// Test: Get entities by type
	entities, err := indexer.neo4j.GetEntitiesByType("", "test-world", 10)
	if err != nil {
		t.Errorf("GetEntitiesByType failed: %v", err)
	}
	t.Logf("Found %d entities in test-world", len(entities))
}

func TestCromaV2(t *testing.T) {
	//CHROMA_URL=http://chromadb:8000
	//CHROMA_USE_V2=true
	os.Setenv("CHROMA_URL", "http://127.0.0.1:8082")
	os.Setenv("EMBEDING_URL", "http://127.0.0.1:11434")
	os.Setenv("EMBEDING_MODEL", "nomic-embed-text:latest")
	_, err := createChromaV2Client()
	if err != nil {
		fmt.Printf("Warning: failed to create ChromaDB v2 client: %v. Falling back to ChromaDB v1 client.", err)

	}

}

func TestIndexer_GetContextWithEvents(t *testing.T) {
	// Create a new indexer
	indexer, err := NewIndexer()
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}
	defer indexer.neo4j.Close()

	// Create a test event
	event := eventbus.NewEvent("player.action.attack", "player-1", "test-world", map[string]interface{}{
		"action": "attack",
		"target": "enemy-1",
		"damage": 10,
	})
	event.ID = "test-event-2"

	// Handle the event
	indexer.HandleEvent(event)

	// Get context with events
	ctx := context.Background()
	contexts, err := indexer.GetContextWithEvents(ctx, []string{"player-1"}, []string{"player.action.attack"}, 1)
	if err != nil {
		t.Fatalf("Failed to get context with events: %v", err)
	}

	if len(contexts) == 0 {
		t.Error("No contexts were retrieved")
	}
}

func stringPtr(s string) *string {
	return &s
}
