// packages/shared/agent/tools/narrative_tool.go
package tools

import (
	"context"
	"fmt"
	"sync"
	"multiverse-core.io/shared/eventbus"
	"time"
)

// NarrativeTool позволяет агентам генерировать нарративы и управлять событиями
type NarrativeTool struct {
	worldID string
	scopeID string
	bus     *eventbus.EventBus
	history []NarrativeEntry
	historyMu sync.RWMutex
}

// NarrativeEntry запись нарратива
type NarrativeEntry struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Entities  []string  `json:"entities"`
}

// NewNarrativeTool создает инструмент для нарративов
func NewNarrativeTool(worldID, scopeID string, bus *eventbus.EventBus) *NarrativeTool {
	return &NarrativeTool{
		worldID: worldID,
		scopeID: scopeID,
		bus:     bus,
		history: make([]NarrativeEntry, 0),
	}
}

// Name возвращает имя инструмента
func (t *NarrativeTool) Name() string {
	return "narrative"
}

// Description возвращает описание для LLM
func (t *NarrativeTool) Description() string {
	return `Генерация нарративов и управление событиями: создание описаний, публикация событий, управление историей.
Используйте для: generate_description, publish_event, get_history, add_history`
}

// Schema возвращает JSON schema параметров
func (t *NarrativeTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Действие: generate_description, publish_event, get_history, add_history",
				"enum":        []string{"generate_description", "publish_event", "get_history", "add_history"},
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
func (t *NarrativeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, ok := params["action"].(string)
	if !ok {
		return nil, fmt.Errorf("action is required")
	}

	switch action {
	case "generate_description":
		return t.generateDescription(params["params"])
	case "publish_event":
		return t.publishEvent(params["params"])
	case "get_history":
		return t.getHistory(params["params"])
	case "add_history":
		return t.addHistory(params["params"])
	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}

// generateDescription генерирует описание сцены
func (t *NarrativeTool) generateDescription(params interface{}) (map[string]interface{}, error) {
	// TODO: реализовать генерацию описания
	return map[string]interface{}{
		"action":     "generate_description",
		"world_id":   t.worldID,
		"scope_id":   t.scopeID,
		"description": "Сцена сгенерирована",
		"timestamp":  time.Now().Format(time.RFC3339),
	}, nil
}

// publishEvent публикует событие в EventBus
func (t *NarrativeTool) publishEvent(params interface{}) (map[string]interface{}, error) {
	// TODO: реализовать публикацию события
	return map[string]interface{}{
		"action":     "publish_event",
		"world_id":   t.worldID,
		"scope_id":   t.scopeID,
		"status":     "event_published",
		"timestamp":  time.Now().Format(time.RFC3339),
	}, nil
}

// getHistory возвращает историю нарративов
func (t *NarrativeTool) getHistory(params interface{}) (map[string]interface{}, error) {
	t.historyMu.RLock()
	defer t.historyMu.RUnlock()

	entries := make([]map[string]interface{}, 0, len(t.history))
	for _, entry := range t.history {
		entries = append(entries, map[string]interface{}{
			"id":        entry.ID,
			"text":      entry.Text,
			"type":      entry.Type,
			"timestamp": entry.Timestamp.Format(time.RFC3339),
			"entities":  entry.Entities,
		})
	}

	return map[string]interface{}{
		"action":    "get_history",
		"world_id":  t.worldID,
		"scope_id":  t.scopeID,
		"history":   entries,
		"count":     len(entries),
	}, nil
}

// addHistory добавляет запись в историю
func (t *NarrativeTool) addHistory(params interface{}) (map[string]interface{}, error) {
	t.historyMu.Lock()
	defer t.historyMu.Unlock()

	entry := NarrativeEntry{
		ID:        fmt.Sprintf("narr-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
	}

	// TODO: реализовать логику добавления записи
	t.history = append(t.history, entry)

	return map[string]interface{}{
		"action":     "add_history",
		"world_id":   t.worldID,
		"scope_id":   t.scopeID,
		"status":     "history_added",
		"entry_id":   entry.ID,
	}, nil
}

var _ Tool = (*NarrativeTool)(nil)
