// Package eventbus provides the event bus infrastructure for the multiverse-core system.
package eventbus

import (
	"maps"
	"time"
)

// WorldRef представляет ссылку на мир
type WorldRef struct {
	ID string `json:"id"`
}

// Entity представляет сущность с полной информацией
type Entity struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Name      string     `json:"name,omitempty"`
	Version   string     `json:"version,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	World     *WorldRef  `json:"world,omitempty"`
	// Metadata для дополнительных данных
	Metadata map[string]any `json:"metadata,omitempty"`
}

// EventPayload представляет типобезопасный payload события
type EventPayload struct {
	Entity *Entity          `json:"entity,omitempty"`
	Target *Entity          `json:"target,omitempty"`
	Source *Entity          `json:"source,omitempty"`
	World  *WorldRef        `json:"world,omitempty"`
	Custom map[string]any   `json:"-"` // Для произвольных полей с dot-notation
}

// NewEventPayload создает новый empty payload
func NewEventPayload() *EventPayload {
	return &EventPayload{
		Custom: make(map[string]any),
	}
}

// WithEntity устанавливает основную сущность события
func (p *EventPayload) WithEntity(id, entityType, name string) *EventPayload {
	p.Entity = &Entity{
		ID:     id,
		Type:   entityType,
		Name:   name,
	}
	return p
}

// WithTarget устанавливает целевую сущность
func (p *EventPayload) WithTarget(id, entityType, name string) *EventPayload {
	p.Target = &Entity{
		ID:     id,
		Type:   entityType,
		Name:   name,
	}
	return p
}

// WithSource устанавливает источник события
func (p *EventPayload) WithSource(id, entityType, name string) *EventPayload {
	p.Source = &Entity{
		ID:     id,
		Type:   entityType,
		Name:   name,
	}
	return p
}

// WithWorld устанавливает мир
func (p *EventPayload) WithWorld(worldID string) *EventPayload {
	p.World = &WorldRef{
		ID: worldID,
	}
	return p
}

// GetCustom возвращает map для произвольных полей
func (p *EventPayload) GetCustom() map[string]any {
	if p.Custom == nil {
		p.Custom = make(map[string]any)
	}
	return p.Custom
}

// ToMap конвертирует payload в map[string]any для eventbus.Publish
func (p *EventPayload) ToMap() map[string]any {
	result := make(map[string]any)

	if p.Entity != nil {
		result["entity"] = entityToMap(p.Entity)
	}
	if p.Target != nil {
		result["target"] = entityToMap(p.Target)
	}
	if p.Source != nil {
		result["source"] = entityToMap(p.Source)
	}
	if p.World != nil {
		result["world"] = worldRefToMap(p.World)
	}

	if p.Custom != nil {
		maps.Copy(result, p.Custom)
	}

	return result
}

// entityToMap конвертирует Entity в map[string]any
func entityToMap(e *Entity) map[string]any {
	result := map[string]any{
		"id":     e.ID,
		"type":   e.Type,
	}
	if e.Name != "" {
		result["name"] = e.Name
	}
	if e.Version != "" {
		result["version"] = e.Version
	}
	if e.CreatedAt != nil {
		result["created_at"] = e.CreatedAt
	}
	if e.UpdatedAt != nil {
		result["updated_at"] = e.UpdatedAt
	}
	if e.World != nil {
		result["world"] = worldRefToMap(e.World)
	}
	if e.Metadata != nil {
		result["metadata"] = e.Metadata
	}
	return result
}

// worldRefToMap конвертирует WorldRef в map[string]any
func worldRefToMap(w *WorldRef) map[string]any {
	return map[string]any{"id": w.ID}
}
