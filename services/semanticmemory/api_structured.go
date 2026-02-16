// Package semanticmemory handles structured context API endpoints.
package semanticmemory

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"multiverse-core/internal/eventbus"
)

// StructuredContextRequest запрос структурированного контекста
type StructuredContextRequest struct {
	EntityIDs   []string `json:"entity_ids"`
	WorldID     string   `json:"world_id,omitempty"`
	RegionID    string   `json:"region_id,omitempty"`
	TimeRange   string   `json:"time_range"`   // "last_1h", "last_24h", "last_7d"
	MaxEvents   int      `json:"max_events,omitempty"`
	IncludeDesc bool     `json:"include_description,omitempty"`
	EventTypes  []string `json:"event_types,omitempty"`
}

// HandleStructuredContext обрабатывает запрос структурированного контекста
func (s *Service) HandleStructuredContext(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

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

	// 1. Загружаем кэш сущностей из Neo4j
	entityCache, err := s.indexer.neo4j.GetEntityCache(ctx, req.EntityIDs)
	if err != nil {
		log.Printf("Failed to load entity cache: %v", err)
		// Не блокируем запрос, используем fallback
		entityCache = buildFallbackEntityCache(req.EntityIDs)
	}

	// 2. Получаем события для сущностей
	events, err := s.indexer.GetEventsForEntities(ctx, req.EntityIDs, req.WorldID, parseTimeRange(req.TimeRange), req.MaxEvents)
	if err != nil {
		log.Printf("Failed to load events: %v", err)
		writeError(w, "failed_to_load_events", http.StatusInternalServerError)
		return
	}

	// 3. Фильтруем по типам событий если указано
	if len(req.EventTypes) > 0 {
		events = filterEventsByTypes(events, req.EventTypes)
	}

	// 4. Строим структурированный контекст
	structured := s.indexer.BuildStructuredContext(events, entityCache)
	structured.Metadata.ProcessingMs = time.Since(start).Milliseconds()
	structured.Metadata.TimeRange = req.TimeRange

	// 5. Возвращаем ответ
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(structured); err != nil {
		log.Printf("Failed to encode structured context response: %v", err)
		http.Error(w, "internal_error", http.StatusInternalServerError)
	}
}

// parseTimeRange конвертирует строку времени в time.Duration
func parseTimeRange(rangeStr string) time.Duration {
	switch rangeStr {
	case "last_1h", "last_1_hour":
		return 1 * time.Hour
	case "last_2h", "last_2_hours":
		return 2 * time.Hour
	case "last_6h", "last_6_hours":
		return 6 * time.Hour
	case "last_12h", "last_12_hours":
		return 12 * time.Hour
	case "last_24h", "last_1d", "last_1_day":
		return 24 * time.Hour
	case "last_7d", "last_7_days":
		return 7 * 24 * time.Hour
	case "last_30d", "last_30_days":
		return 30 * 24 * time.Hour
	default:
		return 2 * time.Hour // default to last 2 hours
	}
}

// buildFallbackEntityCache создаёт минимальный кэш сущностей при ошибке загрузки
func buildFallbackEntityCache(entityIDs []string) map[string]EntityInfo {
	cache := make(map[string]EntityInfo, len(entityIDs))
	for _, id := range entityIDs {
		cache[id] = EntityInfo{
			ID:   id,
			Name: id, // fallback: используем ID как имя
			Type: "unknown",
		}
	}
	return cache
}

// filterEventsByTypes фильтрует события по указанным типам
func filterEventsByTypes(events []eventbus.Event, types []string) []eventbus.Event {
	if len(types) == 0 {
		return events
	}

	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	var filtered []eventbus.Event
	for _, ev := range events {
		if typeSet[ev.EventType] {
			filtered = append(filtered, ev)
		}
	}
	return filtered
}

// writeError записывает JSON ошибку в ответ
func writeError(w http.ResponseWriter, code string, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// GetEventsForEntities получает события для указанных сущностей
func (i *Indexer) GetEventsForEntities(ctx context.Context, entityIDs []string, worldID string, timeRange time.Duration, maxEvents int) ([]eventbus.Event, error) {
	// Определяем временной диапазон
	endTime := time.Now()
	startTime := endTime.Add(-timeRange)

	// Получаем события из ChromaDB по entity_id
	var allEvents []eventbus.Event

	for _, entityID := range entityIDs {
		// Ищем события, связанные с сущностью через метаданные
		events, err := i.chroma.SearchEventsByEntity(ctx, entityID, worldID, startTime, endTime, maxEvents)
		if err != nil {
			log.Printf("Warning: failed to search events for entity %s: %v", entityID, err)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	// Сортируем по времени и ограничиваем
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
	})

	if maxEvents > 0 && len(allEvents) > maxEvents {
		allEvents = allEvents[:maxEvents]
	}

	return allEvents, nil
}

// GetEntityCache получает информацию о сущностях из Neo4j
func (n *Neo4jClient) GetEntityCache(ctx context.Context, entityIDs []string) (map[string]EntityInfo, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	query := `
	MATCH (e:Entity)
	WHERE e.id IN $entity_ids
	RETURN e.id AS id, e.name AS name, e.type AS type, e.world_id AS world_id, e.description AS description, e.payload AS payload
	`

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, map[string]any{
			"entity_ids": entityIDs,
		})
		if err != nil {
			return nil, err
		}

		cache := make(map[string]EntityInfo)
		for res.Next(ctx) {
			record := res.Record()
			
			var id, name, entityType, worldID, description string
			var payload map[string]interface{}

			if val, ok := record.Get("id"); ok && val != nil {
				id = val.(string)
			}
			if val, ok := record.Get("name"); ok && val != nil {
				name = val.(string)
			}
			if val, ok := record.Get("type"); ok && val != nil {
				entityType = val.(string)
			}
			if val, ok := record.Get("world_id"); ok && val != nil {
				worldID = val.(string)
			}
			if val, ok := record.Get("description"); ok && val != nil {
				description = val.(string)
			}
			if val, ok := record.Get("payload"); ok && val != nil {
				payload = val.(map[string]interface{})
			}

			cache[id] = EntityInfo{
				ID:          id,
				Name:        name,
				Type:        entityType,
				WorldID:     worldID,
				Description: description,
				Payload:     payload,
			}
		}

		return cache, res.Err()
	})

	if err != nil {
		return nil, err
	}

	if cache, ok := result.(map[string]EntityInfo); ok {
		return cache, nil
	}

	return nil, fmt.Errorf("unexpected result type from GetEntityCache")
}

// SearchEventsByEntity ищет события по entity_id в ChromaDB через официальный клиент
func (c *ChromaV2Client) SearchEventsByEntity(ctx context.Context, entityID, worldID string, startTime, endTime time.Time, limit int) ([]eventbus.Event, error) {
	// Создаём where фильтр для поиска событий по entity_id
	whereFilter := v2.And(
		v2.EqString("entity_id", entityID),
		v2.GteString("timestamp", startTime.Format(time.RFC3339)),
		v2.LteString("timestamp", endTime.Format(time.RFC3339)),
	)

	if worldID != "" {
		whereFilter = v2.And(whereFilter, v2.EqString("world_id", worldID))
	}

	// Query the collection with the filter
	result, err := c.collection.Query(ctx,
		v2.WithWhereQuery(whereFilter),
		v2.WithNResults(limit),
		v2.WithIncludeQuery(v2.IncludeDocuments, v2.IncludeMetadatas, v2.IncludeEmbeddings))
	if err != nil {
		return nil, fmt.Errorf("failed to search events by entity: %w", err)
	}

	// Extract documents and convert to events
	var events []eventbus.Event
	docList := result.GetDocumentsGroups()
	if docList == nil || len(docList) == 0 {
		return events, nil
	}

	for _, doc := range docList[0] {
		// Пытаемся распарсить событие из метаданных
		metadata := doc.GetMetadata()
		if metadata != nil {
			event := eventbus.Event{
				EventID:   metadata.GetString("event_id"),
				EventType: metadata.GetString("event_type"),
				WorldID:   metadata.GetString("world_id"),
				Source:    metadata.GetString("source"),
			}
			if ts := metadata.GetString("timestamp"); ts != "" {
				event.Timestamp, _ = time.Parse(time.RFC3339, ts)
			}
			events = append(events, event)
		}
	}

	return events, nil
}

// SearchEventsByEntity для HTTP-клиента (fallback)
func (c *ChromaClient) SearchEventsByEntity(ctx context.Context, entityID, worldID string, startTime, endTime time.Time, limit int) ([]eventbus.Event, error) {
	// TODO: Реализовать поиск по entity_id через HTTP API ChromaDB
	// Пока возвращаем пустой список - основной путь через ChromaV2Client
	log.Printf("SearchEventsByEntity not implemented for HTTP client, using fallback")
	return []eventbus.Event{}, nil
}
