// Package semanticmemory handles event processing and indexing.
package semanticmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/jsonpath"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// toStringSlice converts an interface{} to a []string
func toStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		var result []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return []string{}
	}
}

// BuildTextContext creates a text context from entity data
func BuildTextContext(entityID, entityType string, payload map[string]interface{}) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Entity ID: %s", entityID))
	parts = append(parts, fmt.Sprintf("Entity Type: %s", entityType))

	// Add payload information
	if len(payload) > 0 {
		parts = append(parts, "Payload:")
		for key, value := range payload {
			// Convert value to string representation
			valueStr := ""
			switch v := value.(type) {
			case string:
				valueStr = v
			case []string:
				valueStr = strings.Join(v, ", ")
			case []interface{}:
				var items []string
				for _, item := range v {
					items = append(items, fmt.Sprintf("%v", item))
				}
				valueStr = strings.Join(items, ", ")
			default:
				valueStr = fmt.Sprintf("%v", v)
			}
			parts = append(parts, fmt.Sprintf("  %s: %s", key, valueStr))
		}
	}

	return strings.Join(parts, "\n")
}

// RelationsMetrics tracks explicit relations processing statistics.
type RelationsMetrics struct {
	ExplicitCount  int64 // events with explicit Relations[]
	FallbackCount  int64 // events using legacy LinkEventToEntities
	EntityCreated  int64 // stub entities auto-created from relations
	ValidationErrs int64 // relations validation failures
}

// Indexer processes entity events and indexes them.
type Indexer struct {
	chroma  SemanticStorage
	neo4j   *Neo4jClient
	minio   *minio.Client
	Metrics RelationsMetrics
}

// NewIndexer creates a new Indexer.
func NewIndexer() (*Indexer, error) {
	var storage SemanticStorage
	var err error

	// Check environment variable to determine which implementation to use
	useChromaV2 := os.Getenv("CHROMA_USE_V2") == "true"
	log.Printf("Using ChromaDB v2: %t", useChromaV2)
	if useChromaV2 {
		// Only try to create ChromaV2Client if the build tag is enabled
		storage, err = createChromaV2Client()
		if err != nil {
			log.Printf("Warning: failed to create ChromaDB v2 client: %v. Falling back to ChromaDB v1 client.", err)
			storage = NewChromaClient() // ← Возвращаемся к старому клиенту в случае ошибки
		}
	} else {
		storage = NewChromaClient() // ← Использует старый клиент по умолчанию
	}

	neo4j, err := NewNeo4jClient()
	if err != nil {
		return nil, err
	}

	// Initialize MinIO client
	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = "minio:9000"
	}

	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		log.Printf("Warning: failed to create MinIO client: %v", err)
		// Continue without MinIO client
		minioClient = nil
	}

	return &Indexer{
		chroma: storage,
		neo4j:  neo4j,
		minio:  minioClient,
	}, nil
}

// HandleEvent processes all events and indexes them.
func (i *Indexer) HandleEvent(ev eventbus.Event) {
	// Validate input event
	if ev.EventID == "" {
		log.Printf("Invalid event: missing EventID")
		return
	}

	// Process all events, not just entity-related events
	ctx := context.Background()

	// Save to both ChromaDB and Neo4j independently
	i.saveEventToChroma(ctx, ev)
	i.saveEventToNeo4j(ctx, ev)

	// Process entity-related events for both ChromaDB and Neo4j
	if ev.EventType == "entity.created" || ev.EventType == "entity.updated" {
		i.processEntityEvent(ctx, ev)
	}
}

// saveEventToChroma saves an event to ChromaDB
func (i *Indexer) saveEventToChroma(ctx context.Context, ev eventbus.Event) {
	// Validate input event
	if ev.EventID == "" {
		log.Printf("Invalid event: missing EventID for ChromaDB")
		return
	}

	// Create a text representation of the event for ChromaDB
	eventText := i.buildEventTextContext(ev)

	// Metadata for ChromaDB — используем универсальные функции с fallback на старую структуру
	pa := ev.Path()

	metadata := map[string]interface{}{
		"event_id":   ev.EventID,
		"event_type": ev.EventType,
		"world_id":   eventbus.GetWorldIDFromEvent(ev), // новая: payload.world.id / старая: world_id
		"source":     ev.Source,
		"timestamp":  ev.Timestamp,
	}

	// Scope: новая структура scope:{id,type} или старая scope_id (fallback)
	if scope := eventbus.GetScopeFromEvent(ev); scope != nil {
		if scope.ID != "" {
			metadata["scope_id"] = scope.ID
		}
		if scope.Type != "" {
			metadata["scope_type"] = scope.Type
		}
	} else if ev.ScopeID != nil {
		// Fallback на топ-уровень
		metadata["scope_id"] = *ev.ScopeID
	}

	// Сохраняем также иерархические пути для совместимости с LLM
	if entityID, ok := pa.GetString("entity.id"); ok {
		metadata["entity.id"] = entityID
	}
	if entityType, ok := pa.GetString("entity.type"); ok {
		metadata["entity.type"] = entityType
	}

	// Save to ChromaDB
	eventID := fmt.Sprintf("event_%s", ev.EventID)
	if err := i.chroma.UpsertDocument(ctx, eventID, eventText, metadata); err != nil {
		log.Printf("ChromaDB upsert failed for event %s: %v", ev.EventID, err)
	} else {
		log.Printf("Saved event %s to ChromaDB", ev.EventID)
	}
}

// buildEventTextContext creates a human-readable context string from an event
func (i *Indexer) buildEventTextContext(ev eventbus.Event) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Event ID: %s", ev.EventID))
	parts = append(parts, fmt.Sprintf("Event Type: %s", ev.EventType))
	parts = append(parts, fmt.Sprintf("Timestamp: %s", ev.Timestamp.Format("2006-01-02 15:04:05")))
	parts = append(parts, fmt.Sprintf("Source: %s", ev.Source))

	// World: используем универсальное извлечение (новая/старая структура)
	parts = append(parts, fmt.Sprintf("World ID: %s", eventbus.GetWorldIDFromEvent(ev)))

	// Scope: новая структура scope:{id,type} или старая scope_id
	if scope := eventbus.GetScopeFromEvent(ev); scope != nil {
		if scope.ID != "" && scope.Type != "" {
			parts = append(parts, fmt.Sprintf("Scope: %s (%s)", scope.ID, scope.Type))
		} else if scope.ID != "" {
			parts = append(parts, fmt.Sprintf("Scope ID: %s", scope.ID))
		}
	} else if ev.ScopeID != nil {
		parts = append(parts, fmt.Sprintf("Scope ID: %s", *ev.ScopeID))
	}

	// Add payload information — используем jsonpath для упорядоченного вывода ключей (для стабильности LLM-контекста)
	pa := jsonpath.New(ev.Payload)
	if len(ev.Payload) > 0 {
		parts = append(parts, "Payload:")
		// Сортируем пути для детерминированного вывода (важно для эмбеддингов!)
		paths := pa.GetAllPaths()
		for _, path := range paths {
			if val, ok := pa.GetAny(path); ok {
				// Пропускаем вложенные мапы/слайсы — они уже будут в своих под-путях
				switch val.(type) {
				case map[string]any, []any:
					continue
				}
				valueStr := fmt.Sprintf("%v", val)
				parts = append(parts, fmt.Sprintf("  %s: %s", path, valueStr))
			}
		}
	}

	return strings.Join(parts, "\n")
}

// saveEventToNeo4j saves an event to Neo4j with explicit relations priority.
// If event has Relations[] — applies them directly (Этап 3: explicit relations).
// Falls back to legacy LinkEventToEntities for backward compatibility.
func (i *Indexer) saveEventToNeo4j(_ context.Context, ev eventbus.Event) error {
	// Serialize payload to JSON string
	payloadJSON, err := json.Marshal(ev.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Save event node itself
	if err := i.neo4j.SaveEventAsGraph(ev, string(payloadJSON)); err != nil {
		log.Printf("Neo4j SaveEventAsGraph failed for event %s: %v", ev.EventID, err)
		return fmt.Errorf("saveEventAsGraph failed for event %s: %w", ev.EventID, err)
	}

	// ✨ Этап 3: Если есть явные связи — применяем их
	if len(ev.Relations) > 0 {
		i.Metrics.ExplicitCount++
		if err := i.applyExplicitRelations(ev); err != nil {
			log.Printf("Explicit relations apply failed for event %s: %v", ev.EventID, err)
			// Fallback — не блокируем сохранение события
		} else {
			log.Printf("Applied %d explicit relations for event %s (total: %d)", len(ev.Relations), ev.EventID, i.Metrics.ExplicitCount)
			return nil
		}
	}

	// Fallback: старая логика для обратной совместимости (события без relations)
	i.Metrics.FallbackCount++
	if err := i.neo4j.LinkEventToEntities(ev.EventID, ev.Payload); err != nil {
		log.Printf("Neo4j LinkEventToEntities fallback failed for event %s: %v", ev.EventID, err)
	}

	return nil
}

// applyExplicitRelations создаёт семантические связи из ev.Relations.
// Автоматически создаёт stub-Entity для сущностей которых ещё нет в графе.
// Это гарантирует что relation будет создана даже если Entity ещё не индексирована.
func (i *Indexer) applyExplicitRelations(ev eventbus.Event) error {
	// Валидация
	if err := eventbus.ValidateEventRelations(ev); err != nil {
		i.Metrics.ValidationErrs++
		return fmt.Errorf("validate relations: %w", err)
	}

	// Создаём stub-Entity для всех участников связей
	seen := make(map[string]bool)
	for _, rel := range ev.Relations {
		for _, entityID := range []string{rel.From, rel.To} {
			if !seen[entityID] {
				i.ensureEntityFromRelation(entityID, ev.WorldID)
				seen[entityID] = true
			}
		}

		// Создаём семантическую связь (MERGE — идемпотентно)
		if err := i.neo4j.CreateRelation(
			rel.From, rel.To, rel.Type, rel.Directed, rel.Metadata,
		); err != nil {
			log.Printf("Failed to create relation %s-[%s]->%s: %v", rel.From, rel.Type, rel.To, err)
		}
	}
	return nil
}

// ensureEntityFromRelation создаёт stub-Entity если его ещё нет.
// Извлекает тип из ID формата "type:id" (например "player:p1" → type="player").
func (i *Indexer) ensureEntityFromRelation(entityID, worldID string) {
	exists, err := i.neo4j.EntityExists(entityID)
	if err != nil || exists {
		return
	}

	// Извлекаем тип из ID формата "type:id"
	entityType := "unknown"
	if idx := strings.Index(entityID, ":"); idx > 0 {
		entityType = entityID[:idx]
	}

	if err := i.neo4j.EnsureEntity(entityID, entityType, worldID, nil); err != nil {
		log.Printf("Failed to ensure stub entity %s: %v", entityID, err)
		return
	}
	i.Metrics.EntityCreated++
}

// GetRelationsMetrics returns a copy of the current relations processing metrics.
func (i *Indexer) GetRelationsMetrics() RelationsMetrics {
	return i.Metrics
}

// ResetRelationsMetrics resets all metrics counters to zero.
func (i *Indexer) ResetRelationsMetrics() {
	i.Metrics = RelationsMetrics{}
}

// processEntityEvent handles entity-related events for backward compatibility
func (i *Indexer) processEntityEvent(ctx context.Context, ev eventbus.Event) {
	// Используем универсальное извлечение с поддержкой новой и старой структуры:
	pa := ev.Path()

	entityID, ok := pa.GetString("entity.id")
	if !ok {
		// Fallback на старую структуру
		entityID, ok = pa.GetString("entity_id")
	}
	if !ok || entityID == "" {
		log.Printf("Invalid entity event: missing entity.id or entity_id")
		return
	}

	entityType, _ := pa.GetString("entity.type")
	if entityType == "" {
		entityType, _ = pa.GetString("entity_type")
	}

	// Извлекаем payload: может быть в payload.payload (вложенный) или в корне
	var payload map[string]interface{}
	if p, ok := pa.GetMap("payload"); ok {
		payload = p
	} else if p, ok := ev.Payload["payload"].(map[string]interface{}); ok {
		payload = p
	} else {
		// Если payload нет — используем весь payload события кроме служебных полей
		payload = make(map[string]interface{})
		for k, v := range ev.Payload {
			if k != "entity" && k != "entity_id" && k != "entity_type" && k != "world" && k != "world_id" {
				payload[k] = v
			}
		}
	}

	// Build text context
	textContext := BuildTextContext(entityID, entityType, payload)

	// Metadata for ChromaDB
	metadata := map[string]interface{}{
		"entity_id":   entityID,
		"entity_type": entityType,
		"world_id":    ev.WorldID,
	}

	// Index in ChromaDB
	if err := i.chroma.UpsertDocument(ctx, entityID, textContext, metadata); err != nil {
		log.Printf("ChromaDB upsert failed for %s: %v", entityID, err)
	}

	// Add world_id to payload for Neo4j indexing
	neo4jPayload := make(map[string]any)
	for k, v := range payload {
		neo4jPayload[k] = v
	}
	neo4jPayload["world_id"] = ev.WorldID

	// Index in Neo4j
	if err := i.neo4j.UpsertEntity(entityID, entityType, neo4jPayload); err != nil {
		log.Printf("Neo4j upsert failed for %s: %v", entityID, err)
	}

	// Create relationships for inventory items
	if inv, ok := payload["inventory"]; ok {
		inventory := toStringSlice(inv)
		for _, itemID := range inventory {
			if err := i.neo4j.CreateRelationship(entityID, itemID, "CONTAINS"); err != nil {
				log.Printf("Neo4j relationship failed: %s -> %s: %v", entityID, itemID, err)
			}
		}
	}

	log.Printf("Indexed entity %s", entityID)
}

// GetContext retrieves full context for entity IDs. Uses Neo4j as primary source, falls back to ChromaDB.
func (i *Indexer) GetContext(ctx context.Context, entityIDs []string, depth int) (map[string]string, error) {
	entityCache, err := i.neo4j.GetEntityCache(entityIDs)
	if err != nil {
		log.Printf("Neo4j GetEntityCache failed, falling back to ChromaDB: %v", err)
		return i.chroma.GetDocuments(ctx, entityIDs)
	}

	result := make(map[string]string, len(entityCache))
	for id, info := range entityCache {
		text := BuildTextContext(id, info.Type, info.Payload)

		// Enrich with related events if depth > 0
		if depth > 0 {
			events, err := i.neo4j.GetEventsByEntity(id, depth)
			if err == nil && len(events) > 0 {
				text += "\n\nRelated Events:"
				for _, ev := range events {
					text += "\n" + i.buildEventTextContext(ev)
				}
			}
		}

		result[id] = text
	}

	return result, nil
}

// GetEventsByType retrieves events by type. Uses Neo4j as primary source, falls back to ChromaDB.
func (i *Indexer) GetEventsByType(ctx context.Context, eventType string, limit int) ([]string, error) {
	events, err := i.neo4j.GetEventsByTypeNeo4j(eventType, limit)
	if err != nil {
		log.Printf("Neo4j GetEventsByTypeNeo4j failed, falling back to ChromaDB: %v", err)
		return i.chroma.SearchEventsByType(ctx, eventType, limit)
	}

	var results []string
	for _, ev := range events {
		results = append(results, i.buildEventTextContext(ev))
	}
	return results, nil
}

// GetEventsForEntities retrieves events for given entity IDs within a time range from Neo4j.
// worldID is optional (pass "" to skip world filter). maxEvents=0 defaults to 50.
func (i *Indexer) GetEventsForEntities(ctx context.Context, entityIDs []string, worldID string, timeRange time.Duration, maxEvents int) ([]eventbus.Event, error) {
	if len(entityIDs) == 0 {
		return nil, nil
	}
	if maxEvents <= 0 {
		maxEvents = 50
	}
	since := time.Now().Add(-timeRange)
	return i.neo4j.GetEventsForEntities(entityIDs, worldID, since, maxEvents)
}

// GetEntityContext retrieves context for a specific entity ID from MinIO storage.
func (i *Indexer) GetEntityContext(ctx context.Context, entityID string, timeRange string) (map[string]interface{}, error) {
	// If no MinIO client is available, return an error
	if i.minio == nil {
		return nil, fmt.Errorf("MinIO client not initialized")
	}

	// Try to load from global bucket first (entities-global)
	bucket := "entities-global"
	obj, err := i.minio.GetObject(ctx, bucket, entityID+".json", minio.GetObjectOptions{})
	if err == nil {
		defer obj.Close()
		var result map[string]interface{}
		if err := json.NewDecoder(obj).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode entity from global bucket: %w", err)
		}
		return result, nil
	}

	// If not found in global bucket, try to find it in world buckets
	// This is a simplified approach - in production, we'd need better logic for world identification

	// Return error if no entity found
	return nil, fmt.Errorf("entity %s not found in storage", entityID)
}

// Close the chroma client when indexer is done
func (i *Indexer) Close() error {
	if i.chroma != nil {
		return i.chroma.Close()
	}
	return nil
}
