// Package semanticmemory handles ChromaDB integration and compatibility testing.
package semanticmemory

import (
	"context"
	"os"
	"testing"
	"time"

	"multiverse-core/internal/eventbus"
)

// TestChromaCompatibility tests both ChromaClient and ChromaV2Client implementations
// to ensure they maintain the same interface and behavior.
func TestChromaCompatibility(t *testing.T) {
	// Test with ChromaClient (default)
	t.Run("ChromaClient", func(t *testing.T) {
		// Ensure CHROMA_USE_V2 is not set to use the default client
		os.Unsetenv("CHROMA_USE_V2")
		
		testChromaImplementation(t)
	})

	// Test with ChromaV2Client
	t.Run("ChromaV2Client", func(t *testing.T) {
		// Set environment variable to use V2 client
		os.Setenv("CHROMA_USE_V2", "true")
		
		// Skip this test if chroma-go v2 is not properly configured
		// This might happen in environments where the dependency isn't available
		indexer, err := NewIndexer()
		if err != nil {
			t.Skipf("Skipping ChromaV2Client test due to setup error: %v", err)
		}
		
		// Clean up after test
		defer func() {
			indexer.chroma.Close()
			indexer.neo4j.Close()
		}()
		
		testChromaImplementationWithIndexer(t, indexer)
	})
}

// testChromaImplementation tests the Chroma implementation using the default method
func testChromaImplementation(t *testing.T) {
	indexer, err := NewIndexer()
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}
	
	// Clean up after test
	defer func() {
		indexer.chroma.Close()
		indexer.neo4j.Close()
	}()

	testChromaImplementationWithIndexer(t, indexer)
}

// testChromaImplementationWithIndexer contains the actual tests for Chroma implementation
func testChromaImplementationWithIndexer(t *testing.T, indexer *Indexer) {
	ctx := context.Background()
	
	// Test UpsertDocument and GetDocuments
	entityID := "test-entity-compat"
	text := "This is a test document for compatibility testing"
	metadata := map[string]interface{}{
		"test_key": "test_value",
		"version":  "compatibility",
	}
	
	// Insert document
	err := indexer.chroma.UpsertDocument(ctx, entityID, text, metadata)
	if err != nil {
		t.Errorf("Failed to upsert document: %v", err)
	}
	
	// Retrieve document
	documents, err := indexer.chroma.GetDocuments(ctx, []string{entityID})
	if err != nil {
		t.Errorf("Failed to get documents: %v", err)
	}
	
	if len(documents) == 0 {
		t.Error("No documents were retrieved")
	} else if doc, exists := documents[entityID]; !exists || doc != text {
		t.Errorf("Retrieved document doesn't match inserted document. Got: %s, Want: %s", doc, text)
	}
	
	// Test SearchEventsByType with a simulated event
	eventType := "test.event.type"
	limit := 5
	
	// First, insert an event-like document with the event type in metadata
	eventEntityID := "test-event-compat"
	eventText := "This is a test event for compatibility testing"
	eventMetadata := map[string]interface{}{
		"event_type": eventType,
		"test_key":   "test_event_value",
	}
	
	err = indexer.chroma.UpsertDocument(ctx, eventEntityID, eventText, eventMetadata)
	if err != nil {
		t.Errorf("Failed to upsert event document: %v", err)
	}
	
	// Search for events by type
	events, err := indexer.chroma.SearchEventsByType(ctx, eventType, limit)
	if err != nil {
		t.Errorf("Failed to search events by type: %v", err)
	}
	
	if len(events) == 0 {
		t.Error("No events were retrieved for search by type")
	} else {
		found := false
		for _, event := range events {
			if event == eventText {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event text not found in search results. Expected: %s, Got: %v", eventText, events)
		}
	}
}

// BenchmarkChromaImplementations benchmarks both implementations
func BenchmarkChromaImplementations(b *testing.B) {
	implementationNames := []string{"ChromaClient", "ChromaV2Client"}
	
	for i, implName := range implementationNames {
		b.Run(implName, func(b *testing.B) {
			// Set the appropriate environment variable for each implementation
			if i == 1 { // ChromaV2Client
				os.Setenv("CHROMA_USE_V2", "true")
			} else { // ChromaClient
				os.Unsetenv("CHROMA_USE_V2")
			}
			
			b.ResetTimer()
			
			for n := 0; n < b.N; n++ {
				indexer, err := NewIndexer()
				if err != nil {
					b.Fatalf("Failed to create indexer: %v", err)
				}
				
				ctx := context.Background()
				entityID := "benchmark-entity-" + string(rune(n))
				text := "Benchmark document for testing performance"
				metadata := map[string]interface{}{
					"benchmark": true,
					"iteration": n,
				}
				
				// Upsert document
				err = indexer.chroma.UpsertDocument(ctx, entityID, text, metadata)
				if err != nil {
					b.Errorf("Failed to upsert document: %v", err)
				}
				
				// Get document
				_, err = indexer.chroma.GetDocuments(ctx, []string{entityID})
				if err != nil {
					b.Errorf("Failed to get documents: %v", err)
				}
				
				// Clean up
				indexer.chroma.Close()
				indexer.neo4j.Close()
			}
		})
	}
}

// Integration test for end-to-end functionality
func TestIndexerIntegration(t *testing.T) {
	// Test with default implementation
	os.Unsetenv("CHROMA_USE_V2")
	
	indexer, err := NewIndexer()
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}
	defer func() {
		indexer.chroma.Close()
		indexer.neo4j.Close()
	}()

	// Create a test event
	event := eventbus.Event{
		EventID:   "integration-test-event",
		EventType: "integration.test.event",
		Timestamp: time.Now(),
		Source:    "integration-tester",
		WorldID:   "integration-test-world",
		ScopeID:   stringPtr("integration-test-scope"),
		Payload: map[string]interface{}{
			"test_field": "test_value",
			"number":     42,
		},
	}

	// Handle the event
	indexer.HandleEvent(event)

	// Verify that the event was saved
	ctx := context.Background()
	events, err := indexer.GetEventsByType(ctx, "integration.test.event", 10)
	if err != nil {
		t.Fatalf("Failed to retrieve events: %v", err)
	}

	if len(events) == 0 {
		t.Error("No events were retrieved in integration test")
	}
}

