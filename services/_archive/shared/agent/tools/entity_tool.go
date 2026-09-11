// packages/shared/agent/tools/entity_tool.go
package tools

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EntityTool позволяет агентам управлять сущностями
type EntityTool struct {
	worldID string
	scopeID string
	entities map[string]*EntityState
	entitiesMu sync.RWMutex
}

// EntityState состояние сущности
type EntityState struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Health      int                    `json:"health"`
	Position    map[string]interface{} `json:"position"`
	Inventory   []string               `json:"inventory"`
	Status      string                 `json:"status"`
	LastUpdated time.Time              `json:"last_updated"`
}

// NewEntityTool создает инструмент для работы с сущностями
func NewEntityTool(worldID, scopeID string) *EntityTool {
	return &EntityTool{
		worldID:  worldID,
		scopeID:  scopeID,
		entities: make(map[string]*EntityState),
	}
}

// Name возвращает имя инструмента
func (t *EntityTool) Name() string {
	return "entity"
}

// Description возвращает описание для LLM
func (t *EntityTool) Description() string {
	return `Управление сущностями: получение состояния, изменение характеристик, инвентаря, статуса.
Используйте для: get_state, set_health, set_status, add_item, remove_item, get_inventory`
}

// Schema возвращает JSON schema параметров
func (t *EntityTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Действие: get_state, set_health, set_status, add_item, remove_item, get_inventory",
				"enum":        []string{"get_state", "set_health", "set_status", "add_item", "remove_item", "get_inventory"},
			},
			"entity_id": map[string]interface{}{
				"type":        "string",
				"description": "ID сущности",
			},
			"params": map[string]interface{}{
				"type":        "object",
				"description": "Параметры действия",
			},
		},
		"required": []string{"action", "entity_id"},
	}
}

// Execute выполняет инструмент
func (t *EntityTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, ok := params["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action is required")
	}

	entityID, ok := params["entity_id"].(string)
	if !ok {
		return nil, fmt.Errorf("entity_id is required")
	}

	switch action {
	case "get_state":
		return t.getState(entityID)
	case "set_health":
		return t.setHealth(entityID, params["params"])
	case "set_status":
		return t.setStatus(entityID, params["params"])
	case "add_item":
		return t.addItem(entityID, params["params"])
	case "remove_item":
		return t.removeItem(entityID, params["params"])
	case "get_inventory":
		return t.getInventory(entityID)
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// getState возвращает состояние сущности
func (t *EntityTool) getState(entityID string) (map[string]interface{}, error) {
	t.entitiesMu.RLock()
	defer t.entitiesMu.RUnlock()

	entity, exists := t.entities[entityID]
	if !exists {
		return nil, fmt.Errorf("entity %s not found", entityID)
	}

	return map[string]interface{}{
		"entity_id":   entityID,
		"world_id":    t.worldID,
		"scope_id":    t.scopeID,
		"state":       entity,
	}, nil
}

// setHealth изменяет здоровье сущности
func (t *EntityTool) setHealth(entityID string, params interface{}) (map[string]interface{}, error) {
	t.entitiesMu.Lock()
	defer t.entitiesMu.Unlock()

	entity, exists := t.entities[entityID]
	if !exists {
		t.entities[entityID] = &EntityState{
			ID:          entityID,
			Health:      100,
			LastUpdated: time.Now(),
		}
		entity = t.entities[entityID]
	}

	// TODO: реализовать логику изменения здоровья
	entity.Health = 100
	entity.LastUpdated = time.Now()

	return map[string]interface{}{
		"action":      "set_health",
		"entity_id":   entityID,
		"new_health":  entity.Health,
		"status":      "health_updated",
	}, nil
}

// setStatus изменяет статус сущности
func (t *EntityTool) setStatus(entityID string, params interface{}) (map[string]interface{}, error) {
	t.entitiesMu.Lock()
	defer t.entitiesMu.Unlock()

	entity, exists := t.entities[entityID]
	if !exists {
		t.entities[entityID] = &EntityState{
			ID:          entityID,
			Status:      "active",
			LastUpdated: time.Now(),
		}
		entity = t.entities[entityID]
	}

	// TODO: реализовать логику изменения статуса
	entity.Status = "active"
	entity.LastUpdated = time.Now()

	return map[string]interface{}{
		"action":      "set_status",
		"entity_id":   entityID,
		"new_status":  entity.Status,
		"status":      "status_updated",
	}, nil
}

// addItem добавляет предмет в инвентарь
func (t *EntityTool) addItem(entityID string, params interface{}) (map[string]interface{}, error) {
	t.entitiesMu.Lock()
	defer t.entitiesMu.Unlock()

	entity, exists := t.entities[entityID]
	if !exists {
		t.entities[entityID] = &EntityState{
			ID:          entityID,
			Inventory:   []string{},
			LastUpdated: time.Now(),
		}
		entity = t.entities[entityID]
	}

	// TODO: реализовать логику добавления предмета
	entity.Inventory = append(entity.Inventory, "new_item")
	entity.LastUpdated = time.Now()

	return map[string]interface{}{
		"action":      "add_item",
		"entity_id":   entityID,
		"status":      "item_added",
	}, nil
}

// removeItem удаляет предмет из инвентаря
func (t *EntityTool) removeItem(entityID string, params interface{}) (map[string]interface{}, error) {
	t.entitiesMu.Lock()
	defer t.entitiesMu.Unlock()

	entity, exists := t.entities[entityID]
	if !exists {
		return nil, fmt.Errorf("entity %s not found", entityID)
	}

	// TODO: реализовать логику удаления предмета
	entity.LastUpdated = time.Now()

	return map[string]interface{}{
		"action":      "remove_item",
		"entity_id":   entityID,
		"status":      "item_removed",
	}, nil
}

// getInventory возвращает инвентарь сущности
func (t *EntityTool) getInventory(entityID string) (map[string]interface{}, error) {
	t.entitiesMu.RLock()
	defer t.entitiesMu.RUnlock()

	entity, exists := t.entities[entityID]
	if !exists {
		return nil, fmt.Errorf("entity %s not found", entityID)
	}

	return map[string]interface{}{
		"action":      "get_inventory",
		"entity_id":   entityID,
		"inventory":   entity.Inventory,
	}, nil
}

var _ Tool = (*EntityTool)(nil)
