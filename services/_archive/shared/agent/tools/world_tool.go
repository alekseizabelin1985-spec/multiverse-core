// packages/shared/agent/tools/world_tool.go
package tools

import (
	"context"
	"fmt"
	"sync"
	"multiverse-core.io/shared/eventbus"
	"time"
)

// WorldTool позволяет агентам взаимодействовать с миром
type WorldTool struct {
	worldID   string
	scopeID   string
	bus       *eventbus.EventBus
	state     map[string]interface{}
	stateMu   sync.RWMutex
}

// NewWorldTool создает инструмент для взаимодействия с миром
func NewWorldTool(worldID, scopeID string, bus *eventbus.EventBus) *WorldTool {
	return &WorldTool{
		worldID: worldID,
		scopeID: scopeID,
		bus:     bus,
		state:   make(map[string]interface{}),
	}
}

// Name возвращает имя инструмента
func (t *WorldTool) Name() string {
	return "world"
}

// Description возвращает описание для LLM
func (t *WorldTool) Description() string {
	return `Взаимодействие с миром: получение состояния, изменение погоды, времени, событий.
Используйте для: get_state, set_weather, set_time, trigger_event, get_entities_nearby`
}

// Schema возвращает JSON schema параметров
func (t *WorldTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Действие: get_state, set_weather, set_time, trigger_event, get_entities_nearby",
				"enum":        []string{"get_state", "set_weather", "set_time", "trigger_event", "get_entities_nearby"},
			},
			"params": map[string]interface{}{
				"type":        "object",
				"description": "Параметры действия",
			},
		},
		"required": []string{"action"},
	}
}

// Execute выполняет инструмент
func (t *WorldTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, ok := params["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action is required")
	}

	switch action {
	case "get_state":
		return t.getState(params["params"])
	case "set_weather":
		return t.setWeather(params["params"])
	case "set_time":
		return t.setTime(params["params"])
	case "trigger_event":
		return t.triggerEvent(params["params"])
	case "get_entities_nearby":
		return t.getEntitiesNearby(params["params"])
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// getState возвращает состояние мира
func (t *WorldTool) getState(params interface{}) (map[string]interface{}, error) {
	t.stateMu.RLock()
	defer t.stateMu.RUnlock()

	return map[string]interface{}{
		"world_id": t.worldID,
		"scope_id": t.scopeID,
		"state":    t.state,
		"timestamp": time.Now().Format(time.RFC3339),
	}, nil
}

// setWeather устанавливает погоду
func (t *WorldTool) setWeather(params interface{}) (map[string]interface{}, error) {
	// TODO: реализовать логику
	return map[string]interface{}{
		"action":    "set_weather",
		"world_id":  t.worldID,
		"status":    "weather_updated",
	}, nil
}

// setTime устанавливает время в мире
func (t *WorldTool) setTime(params interface{}) (map[string]interface{}, error) {
	// TODO: реализовать логику
	return map[string]interface{}{
		"action":    "set_time",
		"world_id":  t.worldID,
		"status":    "time_updated",
	}, nil
}

// triggerEvent триггерит событие в мире
func (t *WorldTool) triggerEvent(params interface{}) (map[string]interface{}, error) {
	// TODO: реализовать публикацию события через EventBus
	return map[string]interface{}{
		"action":    "trigger_event",
		"world_id":  t.worldID,
		"status":    "event_triggered",
	}, nil
}

// getEntitiesNearby возвращает сущности рядом с координатами
func (t *WorldTool) getEntitiesNearby(params interface{}) (map[string]interface{}, error) {
	// TODO: реализовать поиск сущностей
	return map[string]interface{}{
		"action":     "get_entities_nearby",
		"world_id":   t.worldID,
		"entities":   []string{},
	}, nil
}

var _ Tool = (*WorldTool)(nil)
