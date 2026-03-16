// Package semanticmemory handles structured context API endpoints.
package semanticmemory

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"time"

	"multiverse-core/internal/eventbus"
)

// StructuredContextRequest запрос структурированного контекста
type StructuredContextRequest struct {
	EntityIDs   []string `json:"entity_ids"`
	WorldID     string   `json:"world_id,omitempty"`
	RegionID    string   `json:"region_id,omitempty"`
	TimeRange   string   `json:"time_range"` // "last_1h", "last_24h", "last_7d"
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
	entityCache, err := s.indexer.neo4j.GetEntityCache(req.EntityIDs)
	if err != nil {
		log.Printf("Failed to load entity cache: %v", err)
		// Не блокируем запрос, используем fallback
		entityCache = buildFallbackEntityCache(req.EntityIDs)
	}

	// 2. Получаем события для сущностей через ChromaDB
	var allEvents []eventbus.Event
	for _, entityID := range req.EntityIDs {
		events, err := s.indexer.GetEventsForEntities(ctx, []string{entityID}, req.WorldID, parseTimeRange(req.TimeRange), req.MaxEvents)
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
