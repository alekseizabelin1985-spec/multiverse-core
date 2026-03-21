// services/narrativeorchestrator/gm.go

package narrativeorchestrator

import (
	"context"
	"sync"
	"time"

	"multiverse-core.io/shared/spatial"
)

type HistoryEntry struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
}

type GMInstance struct {
	ScopeID         string                 `json:"scope_id"`
	ScopeType       string                 `json:"scope_type"`
	WorldID         string                 `json:"world_id"`
	FocusEntities   []string               `json:"focus_entities"`
	VisibilityScope spatial.VisibilityScope `json:"visibility_scope"`
	State           map[string]interface{} `json:"state"`
	History         []HistoryEntry         `json:"history"`
	Config          map[string]interface{} `json:"config"`
	LastProcessTime int64                  `json:"last_process_time"`
	CreatedAt       time.Time              `json:"created_at"`

	// EmittedEventIDs — event_id которые этот ГМ сам опубликовал, с временем публикации.
	// Используется для предотвращения каскадной реакции на свои же события.
	// Записи старше LastProcessTime - 1min автоматически вычищаются.
	EmittedEventIDs map[string]time.Time `json:"-"`

	// Per-GM mutex protects State, History, Config, EmittedEventIDs from concurrent access
	// without holding the global NarrativeOrchestrator.mu lock.
	mu sync.Mutex `json:"-"`

	// processing guards against concurrent Oracle calls for the same GM.
	// Only one goroutine may process events for a GM at a time.
	processing chan struct{} `json:"-"`

	// ttlTimer is the inactivity cleanup timer. Reset on each received event.
	ttlTimer *time.Timer `json:"-"`
	ttlDur   time.Duration `json:"-"`
}

// trackEmitted запоминает event_id который этот ГМ опубликовал.
// Вызывать под gm.mu.Lock().
func (gm *GMInstance) trackEmitted(eventID string) {
	if gm.EmittedEventIDs == nil {
		gm.EmittedEventIDs = make(map[string]time.Time)
	}
	gm.EmittedEventIDs[eventID] = time.Now()
}

// isOwnEvent проверяет, был ли event_id сгенерирован этим ГМ.
// Вызывать под gm.mu.Lock().
func (gm *GMInstance) isOwnEvent(eventID string) bool {
	if gm.EmittedEventIDs == nil {
		return false
	}
	_, ok := gm.EmittedEventIDs[eventID]
	return ok
}

// evictExpiredEmitted удаляет записи старше LastProcessTime - 1 минута.
// Предотвращает рост map до OOM при долгоживущих ГМ.
// Вызывать под gm.mu.Lock().
func (gm *GMInstance) evictExpiredEmitted() {
	if len(gm.EmittedEventIDs) == 0 {
		return
	}
	var cutoff time.Time
	if gm.LastProcessTime > 0 {
		cutoff = time.UnixMilli(gm.LastProcessTime).Add(-1 * time.Minute)
	} else {
		cutoff = time.Now().Add(-2 * time.Minute)
	}
	for id, ts := range gm.EmittedEventIDs {
		if ts.Before(cutoff) {
			delete(gm.EmittedEventIDs, id)
		}
	}
}

// initConcurrency sets up per-GM concurrency primitives.
// Must be called once after constructing or deserializing a GMInstance.
func (gm *GMInstance) initConcurrency(ttl time.Duration, onExpire func()) {
	gm.processing = make(chan struct{}, 1)
	gm.ttlDur = ttl
	gm.ttlTimer = time.AfterFunc(ttl, onExpire)
}

// tryStartProcessing attempts to acquire the processing lock.
// Returns true if acquired (caller must call doneProcessing when finished).
func (gm *GMInstance) tryStartProcessing() bool {
	select {
	case gm.processing <- struct{}{}:
		return true
	default:
		return false
	}
}

// doneProcessing releases the processing lock.
func (gm *GMInstance) doneProcessing() {
	<-gm.processing
}

// resetTTL resets the inactivity timer. Safe to call from any goroutine.
func (gm *GMInstance) resetTTL() {
	if gm.ttlTimer != nil {
		gm.ttlTimer.Reset(gm.ttlDur)
	}
}

// stopTTL stops the inactivity timer (e.g., on explicit deletion).
func (gm *GMInstance) stopTTL() {
	if gm.ttlTimer != nil {
		gm.ttlTimer.Stop()
	}
}

// EventDetail содержит полную информацию о событии для промта.
type EventDetail struct {
	EventID     string                 `json:"event_id"`
	EventType   string                 `json:"event_type"`
	Timestamp   time.Time              `json:"timestamp"`
	Source      string                 `json:"source"`
	WorldID     string                 `json:"world_id"`
	ScopeID     string                 `json:"scope_id,omitempty"`
	Payload     map[string]interface{} `json:"payload"`
	Description string                 `json:"description"` // Человеко-читаемое описание
}

// EventCluster для временной группировки событий.
type EventCluster struct {
	RelativeTime string        `json:"relative_time"`
	Events       []EventDetail `json:"events"` // Полный список событий в кластере
}

func (gm *GMInstance) UpdateVisibilityScope(provider spatial.GeometryProvider) {
	geometry, _ := provider.GetGeometry(context.Background(), gm.WorldID, gm.ScopeID)
	if geometry == nil {
		return
	}

	gm.VisibilityScope = spatial.DefaultScope(gm.ScopeType, geometry, gm.Config)

	if bufRaw, exists := gm.Config["geometry_buffer_m"]; exists {
		if buf, ok := bufRaw.(float64); ok {
			gm.VisibilityScope = gm.VisibilityScope.Buffer(buf)
		}
	} else if gm.ScopeType == "location" {
		gm.VisibilityScope = gm.VisibilityScope.Buffer(200)
	}
}
