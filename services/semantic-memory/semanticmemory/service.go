// Package semanticmemory implements the SemanticMemory service.
package semanticmemory

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"multiverse-core.io/shared/eventbus"

	"github.com/gorilla/mux"
)

// Service manages the SemanticMemory lifecycle.
type Service struct {
	bus     *eventbus.EventBus
	indexer *Indexer
	server  *http.Server
}

// contextRequest represents a context request.
type contextRequest struct {
	EntityIDs []string `json:"entity_ids"`
	Depth     int      `json:"depth,omitempty"`
}

// eventRequest represents an event request.
type eventRequest struct {
	EventType string `json:"event_type"`
	Limit     int    `json:"limit,omitempty"`
}

// contextWithEventsRequest represents a context request with events.
type contextWithEventsRequest struct {
	EntityIDs  []string `json:"entity_ids"`
	EventTypes []string `json:"event_types"`
	Depth      int      `json:"depth,omitempty"`
}

// NewService creates a new SemanticMemory service.
func NewService(bus *eventbus.EventBus) (*Service, error) {
	indexer, err := NewIndexer()
	if err != nil {
		return nil, err
	}

	// Setup HTTP server
	r := mux.NewRouter()

	// Legacy endpoint for backward compatibility
	r.HandleFunc("/v1/context", func(w http.ResponseWriter, r *http.Request) {
		var req contextRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		contexts, err := indexer.GetContext(r.Context(), req.EntityIDs, req.Depth)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{"contexts": contexts}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(response)
	}).Methods("POST")

	// Endpoint for events by type
	r.HandleFunc("/v1/events", func(w http.ResponseWriter, r *http.Request) {
		var req eventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Set default limit if not provided
		if req.Limit == 0 {
			req.Limit = 10
		}

		// Retrieve events from storage
		events, err := indexer.GetEventsByType(r.Context(), req.EventType, req.Limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{"events": events}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(response)
	}).Methods("POST")

	// Endpoint for context with events (legacy)
	r.HandleFunc("/v1/context-with-events", func(w http.ResponseWriter, r *http.Request) {
		var req contextWithEventsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		log.Println(req)
		// Retrieve context with events from storage
		contexts, err := indexer.GetContextWithEvents(r.Context(), req.EntityIDs, req.EventTypes, req.Depth)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{"contexts": contexts}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(response)
	}).Methods("POST")

	// NEW: Structured context endpoint with embedded IDs for AI
	r.HandleFunc("/v1/context/structured", func(w http.ResponseWriter, r *http.Request) {
		var req StructuredContextRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "invalid_json", http.StatusBadRequest)
			return
		}

		if len(req.EntityIDs) == 0 {
			writeError(w, "entity_ids_required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		start := time.Now()

		// Load entity cache from Neo4j
		entityCache, err := indexer.neo4j.GetEntityCache(req.EntityIDs)
		if err != nil {
			log.Printf("Failed to load entity cache: %v", err)
			entityCache = buildFallbackEntityCache(req.EntityIDs)
		}

		// Get events for entities
		events, err := indexer.GetEventsForEntities(ctx, req.EntityIDs, req.WorldID, parseTimeRange(req.TimeRange), req.MaxEvents)
		if err != nil {
			log.Printf("Failed to load events: %v", err)
			writeError(w, "failed_to_load_events", http.StatusInternalServerError)
			return
		}

		// Filter by event types if specified
		if len(req.EventTypes) > 0 {
			events = filterEventsByTypes(events, req.EventTypes)
		}

		// Build structured context
		structured := indexer.BuildStructuredContext(events, entityCache)
		structured.Metadata.ProcessingMs = time.Since(start).Milliseconds()
		structured.Metadata.TimeRange = req.TimeRange

		// Return response
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(structured); err != nil {
			log.Printf("Failed to encode structured context response: %v", err)
			http.Error(w, "internal_error", http.StatusInternalServerError)
		}
	}).Methods("POST")

	// NEW: Entity context endpoint for Living Worlds integration
	r.HandleFunc("/v1/entity-context/{entity_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		entityID := vars["entity_id"]

		if entityID == "" {
			http.Error(w, "entity_id is required", http.StatusBadRequest)
			return
		}

		// Get time range from query parameters
		timeRange := r.URL.Query().Get("time_range")
		if timeRange == "" {
			timeRange = "last_24h" // default
		}

		// Load entity context from storage (using existing logic)
		context, err := indexer.GetEntityContext(r.Context(), entityID, timeRange)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"entity_id":  entityID,
			"context":    context,
			"time_range": timeRange,
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(response)
	}).Methods("GET")

	// GET /v1/entities/{entity_id} — retrieve a single entity by its ID.
	r.HandleFunc("/v1/entities/{entity_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		entityID := vars["entity_id"]
		if entityID == "" {
			writeError(w, "entity_id_required", http.StatusBadRequest)
			return
		}

		entity, err := indexer.GetEntityByID(r.Context(), entityID)
		if err != nil {
			log.Printf("GetEntityByID(%s): %v", entityID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if entity == nil {
			writeError(w, "entity_not_found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(entity)
	}).Methods("GET")

	// POST /v1/entities/query — flexible entity query.
	// All filter fields are optional; results combine matching filters with AND logic.
	r.HandleFunc("/v1/entities/query", func(w http.ResponseWriter, r *http.Request) {
		var q EntityQuery
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			writeError(w, "invalid_json", http.StatusBadRequest)
			return
		}

		entities, err := indexer.QueryEntities(r.Context(), q)
		if err != nil {
			log.Printf("entities/query: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{"entities": entities})
	}).Methods("POST")

	// GET /v1/events/{event_id} — retrieve a single event by its ID.
	r.HandleFunc("/v1/events/{event_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		eventID := vars["event_id"]
		if eventID == "" {
			writeError(w, "event_id_required", http.StatusBadRequest)
			return
		}

		ev, err := indexer.neo4j.GetEventByID(eventID)
		if err != nil {
			log.Printf("GetEventByID(%s): %v", eventID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if ev == nil {
			writeError(w, "event_not_found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(ev)
	}).Methods("GET")

	// POST /v1/events/query — flexible event query by entity_ids, world_id, event_types, time_range.
	r.HandleFunc("/v1/events/query", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			EntityIDs  []string `json:"entity_ids"`
			WorldID    string   `json:"world_id"`
			EventTypes []string `json:"event_types"`
			TimeRange  string   `json:"time_range"`
			Limit      int      `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "invalid_json", http.StatusBadRequest)
			return
		}

		if req.Limit <= 0 {
			req.Limit = 10
		}

		ctx := r.Context()

		// Branch: query by entity IDs via Neo4j (supports time range + world filter).
		if len(req.EntityIDs) > 0 {
			events, err := indexer.GetEventsForEntities(ctx, req.EntityIDs, req.WorldID, parseTimeRange(req.TimeRange), req.Limit)
			if err != nil {
				log.Printf("events/query GetEventsForEntities: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(req.EventTypes) > 0 {
				events = filterEventsByTypes(events, req.EventTypes)
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]interface{}{"events": events})
			return
		}

		// Branch: query by world_id only via Neo4j.
		if req.WorldID != "" && len(req.EventTypes) == 0 {
			events, err := indexer.neo4j.GetEventsByWorldID(req.WorldID, req.Limit)
			if err != nil {
				log.Printf("events/query GetEventsByWorldID: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]interface{}{"events": events})
			return
		}

		// Branch: query by event_type(s) via Neo4j.
		if len(req.EventTypes) == 1 {
			var events []eventbus.Event
			var err error
			if req.WorldID != "" {
				events, err = indexer.neo4j.GetEventsByWorldAndType(req.WorldID, req.EventTypes[0], req.Limit)
			} else {
				events, err = indexer.neo4j.GetEventsByTypeNeo4j(req.EventTypes[0], req.Limit)
			}
			if err != nil {
				log.Printf("events/query GetEventsByType (Neo4j): %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]interface{}{"events": events})
			return
		}

		// Branch: multiple event types via Neo4j single query.
		if len(req.EventTypes) > 1 || req.WorldID != "" {
			events, err := indexer.neo4j.GetEventsByTypes(req.EventTypes, req.WorldID, req.Limit)
			if err != nil {
				log.Printf("events/query GetEventsByTypes (Neo4j): %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]interface{}{"events": events})
			return
		}

		writeError(w, "no_filter_provided", http.StatusBadRequest)
	}).Methods("POST")

	// Health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(response)
	}).Methods("GET")

	semanticport := os.Getenv("SEMANTIC_PORT")
	if semanticport == "" {
		semanticport = "8080"
	}

	server := &http.Server{
		Addr:         ":" + semanticport,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return &Service{
		bus:     bus,
		indexer: indexer,
		server:  server,
	}, nil
}

// Run starts the service and blocks until context is cancelled.
func (s *Service) Run(ctx context.Context) error {
	// Start HTTP server
	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server failed: %v", err)
		}
	}()

	// Subscribe to all event topics for comprehensive context
	go s.bus.Subscribe(ctx, eventbus.TopicPlayerEvents, "semantic-memory-player-group", s.indexer.HandleEvent)
	go s.bus.Subscribe(ctx, eventbus.TopicWorldEvents, "semantic-memory-world-group", s.indexer.HandleEvent)
	go s.bus.Subscribe(ctx, eventbus.TopicGameEvents, "semantic-memory-game-group", s.indexer.HandleEvent)
	go s.bus.Subscribe(ctx, eventbus.TopicSystemEvents, "semantic-memory-system-group", s.indexer.HandleEvent)
	go s.bus.Subscribe(ctx, eventbus.TopicScopeManagement, "semantic-memory-scope-group", s.indexer.HandleEvent)
	go s.bus.Subscribe(ctx, eventbus.TopicNarrativeOutput, "semantic-memory-narrative-group", s.indexer.HandleEvent)

	<-ctx.Done()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	s.server.Shutdown(shutdownCtx)

	// Close chroma client
	if s.indexer.chroma != nil {
		s.indexer.chroma.Close()
	}

	// Close Neo4j driver
	s.indexer.neo4j.Close()

	return ctx.Err()
}
