// Package semanticmemory handles structured context API endpoints.
package semanticmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"multiverse-core/internal/eventbus"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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
	entityCache, err := GetEntityCache(ctx, s.indexer.neo4j, req.EntityIDs)
	if err != nil {
		log.Printf("Failed to load entity cache: %v", err)
		// Не блокируем запрос, используем fallback
		entityCache = buildFallbackEntityCache(req.EntityIDs)
	}

	// 2. Получаем события для сущностей через ChromaDB
	var allEvents []eventbus.Event
	for _, entityID := range req.EntityIDs {
		events, err := SearchEventsByEntity(ctx, s.indexer.chroma, entityID, req.WorldID, parseTimeRange(req.TimeRange), req.MaxEvents)
		if err != nil {
			log.Printf("Warning: failed to search events for entity %s: %v", entityID, err)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	// 3. Сортируем по времени и ограничиваем
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
	})
	if req.MaxEvents > 0 && len(allEvents) > req.MaxEvents {
		allEvents = allEvents[:req.MaxEvents]
	}

	// 4. Фильтруем по типам событий если указано
	if len(req.EventTypes) > 0 {
		allEvents = filterEventsByTypes(allEvents, req.EventTypes)
	}

	// 5. Строим структурированный контекст
	structured := s.indexer.BuildStructuredContext(allEvents, entityCache)
	structured.Metadata.ProcessingMs = time.Since(start).Milliseconds()
	structured.Metadata.TimeRange = req.TimeRange

	// 6. Возвращаем ответ
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

// GetEntityCache получает информацию о сущностях из Neo4j
func GetEntityCache(ctx context.Context, neo4jClient *Neo4jClient, entityIDs []string) (map[string]EntityInfo, error) {
	if neo4jClient == nil || neo4jClient.driver == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	session := neo4jClient.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close() // v5: Close() без аргументов

	query := `
	MATCH (e:Entity)
	WHERE e.id IN $entity_ids
	RETURN e.id AS id, e.name AS name, e.type AS type, e.world_id AS world_id, e.description AS description, e.payload AS payload
	`

	result, err := session.ReadTransaction(ctx, func(tx neo4j.Transaction) (any, error) {
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
				if s, ok := val.(string); ok {
					id = s
				}
			}
			if val, ok := record.Get("name"); ok && val != nil {
				if s, ok := val.(string); ok {
					name = s
				}
			}
			if val, ok := record.Get("type"); ok && val != nil {
				if s, ok := val.(string); ok {
					entityType = s
				}
			}
			if val, ok := record.Get("world_id"); ok && val != nil {
				if s, ok := val.(string); ok {
					worldID = s
				}
			}
			if val, ok := record.Get("description"); ok && val != nil {
				if s, ok := val.(string); ok {
					description = s
				}
			}
			if val, ok := record.Get("payload"); ok && val != nil {
				if p, ok := val.(map[string]interface{}); ok {
					payload = p
				}
			}

			if id != "" {
				cache[id] = EntityInfo{
					ID:          id,
					Name:        name,
					Type:        entityType,
					WorldID:     worldID,
					Description: description,
					Payload:     payload,
				}
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

// SearchEventsByEntity ищет события по entity_id через интерфейс SemanticStorage
// Реализация зависит от конкретного клиента (ChromaClient или ChromaV2Client)
func SearchEventsByEntity(ctx context.Context, storage SemanticStorage, entityID, worldID string, timeRange time.Duration, limit int) ([]eventbus.Event, error) {
	// Для ChromaV2Client можно использовать прямой доступ через type assertion
	if v2Client, ok := storage.(*ChromaV2Client); ok {
		return searchEventsByEntityV2(ctx, v2Client, entityID, worldID, timeRange, limit)
	}

	// Fallback для HTTP-клиента: возвращаем пустой список
	// В реальной реализации нужно добавить метод в интерфейс SemanticStorage
	log.Printf("SearchEventsByEntity: using fallback for non-v2 client, entity=%s", entityID)
	return []eventbus.Event{}, nil
}

// searchEventsByEntityV2 реализует поиск для официального клиента ChromaDB v2
// Примечание: эта функция использует internal API ChromaV2Client
func searchEventsByEntityV2(ctx context.Context, client *ChromaV2Client, entityID, worldID string, timeRange time.Duration, limit int) ([]eventbus.Event, error) {
	// Эта функция должна быть в chroma_v2.go из-за build tag
	// Здесь оставляем заглушку для компиляции
	log.Printf("searchEventsByEntityV2: entity=%s, world=%s, range=%v", entityID, worldID, timeRange)

	// В production реализовать через v2 API:
	// whereFilter := v2.And(
	//     v2.EqString("entity_id", entityID),
	//     v2.GteString("timestamp", startTime.Format(time.RFC3339)),
	// )
	// result, err := client.collection.Query(ctx, v2.WithWhereQuery(whereFilter), ...)

	return []eventbus.Event{}, nil
}
