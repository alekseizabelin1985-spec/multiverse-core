// services/narrativeorchestrator/gm.go

package narrativeorchestrator

import (
	"context"
	"time"

	"multiverse-core.io/shared/spatial"
)

type HistoryEntry struct {
	EventID   string    `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
}

type GMInstance struct {
	ScopeID         string                  `json:"scope_id"`
	ScopeType       string                  `json:"scope_type"`
	WorldID         string                  `json:"world_id"`
	FocusEntities   []string                `json:"focus_entities"`
	VisibilityScope spatial.VisibilityScope `json:"visibility_scope"`
	State           map[string]interface{}  `json:"state"`
	History         []HistoryEntry          `json:"history"`
	Config          map[string]interface{}  `json:"config"`
	LastProcessTime int64                   `json:"last_process_time"`
	CreatedAt       time.Time               `json:"created_at"`
}

// EventDetail содержит полную информацию о событии для промта.
type EventDetail struct {
	EventID     string    `json:"event_id"`
	EventType   string    `json:"event_type"`
	Timestamp   time.Time `json:"timestamp"`
	Source      string    `json:"source"`
	WorldID     string    `json:"world_id"`
	ScopeID     string    `json:"scope_id,omitempty"`
	Payload     map[string]interface{} `json:"payload"`
	Description string    `json:"description"` // Человеко-читаемое описание
}

// EventCluster для временной группировки событий.
type EventCluster struct {
	RelativeTime string       `json:"relative_time"`
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
