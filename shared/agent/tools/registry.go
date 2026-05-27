// packages/shared/agent/tools/registry.go
package tools

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Tool определяет интерфейс для всех инструментов агента
type Tool interface {
	// Name возвращает имя инструмента
	Name() string

	// Description возвращает описание для LLM
	Description() string

	// Schema возвращает JSON schema параметров (OpenAPI format)
	Schema() map[string]interface{}

	// Execute выполняет инструмент с параметрами
	Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)
}

// ToolRegistry управляет регистрацией и выполнением инструментов
type ToolRegistry struct {
	// tools хранит зарегистрированные инструменты
	tools map[string]Tool

	// rateLimits ограничивает частоту вызовов
	rateLimits map[string]*RateLimit

	// mu защищает concurrent доступ
	mu sync.RWMutex

	// stats статистика вызовов
	stats *ToolStats
}

// RateLimit ограничивает частоту вызовов инструмента
type RateLimit struct {
	MaxCalls    int           // Максимум вызовов
	Window      time.Duration // Временное окно
	LastCalled  time.Time     // Время последнего вызова
	CallCount   int           // Счётчик вызовов
}

// ToolStats статистика инструментов
type ToolStats struct {
	TotalCalls     int64            // Всего вызовов
	TotalErrors    int64            // Всего ошибок
	TotalLatency   time.Duration    // Общая задержка
	CallsByTool    map[string]int64 // Вызовы по инструментам
	ErrorsByTool   map[string]int64 // Ошибки по инструментам
}

// NewToolRegistry создает новый реестр инструментов
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:      make(map[string]Tool),
		rateLimits: make(map[string]*RateLimit),
		stats: &ToolStats{
			CallsByTool:  make(map[string]int64),
			ErrorsByTool: make(map[string]int64),
		},
	}
}

// Register регистрирует инструмент
func (r *ToolRegistry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	r.tools[name] = tool

	return nil
}

// RegisterWithRateLimit регистрирует инструмент с ограничением частоты
func (r *ToolRegistry) RegisterWithRateLimit(tool Tool, maxCalls int, window time.Duration) error {
	if err := r.Register(tool); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.rateLimits[tool.Name()] = &RateLimit{
		MaxCalls: maxCalls,
		Window:   window,
	}

	return nil
}

// Get возвращает инструмент по имени
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// List возвращает все зарегистрированные инструменты
func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	return tools
}

// Execute выполняет инструмент с проверкой rate-limit и аудитом
func (r *ToolRegistry) Execute(ctx context.Context, name string, params map[string]interface{}) (interface{}, error) {
	start := time.Now()

	tool, exists := r.Get(name)
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	// Проверяем rate-limit
	if r.checkRateLimit(name) {
		return nil, fmt.Errorf("rate limit exceeded for tool %s", name)
	}

	// Выполняем инструмент
	result, err := tool.Execute(ctx, params)

	// Обновляем статистику
	r.updateStats(name, start, err)

	return result, err
}

// checkRateLimit проверяет ограничение частоты вызовов
func (r *ToolRegistry) checkRateLimit(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limiter, exists := r.rateLimits[name]
	if !exists {
		return false // Нет ограничения
	}

	now := time.Now()

	// Сбрасываем счётчик если окно истекло
	if now.Sub(limiter.LastCalled) > limiter.Window {
		limiter.CallCount = 0
		limiter.LastCalled = now
	}

	limiter.CallCount++

	return limiter.CallCount > limiter.MaxCalls
}

// updateStats обновляет статистику вызовов
func (r *ToolRegistry) updateStats(name string, start time.Time, err error) {
	latency := time.Since(start)

	r.stats.TotalCalls++
	r.stats.CallsByTool[name]++
	r.stats.TotalLatency += latency

	if err != nil {
		r.stats.TotalErrors++
		r.stats.ErrorsByTool[name]++
	}
}

// GetStats возвращает статистику инструментов
func (r *ToolRegistry) GetStats() map[string]interface{} {
	avgLatency := time.Duration(0)
	if r.stats.TotalCalls > 0 {
		avgLatency = r.stats.TotalLatency / time.Duration(r.stats.TotalCalls)
	}

	return map[string]interface{}{
		"total_calls":       r.stats.TotalCalls,
		"total_errors":      r.stats.TotalErrors,
		"avg_latency_ms":    avgLatency.Milliseconds(),
		"calls_by_tool":     r.stats.CallsByTool,
		"errors_by_tool":    r.stats.ErrorsByTool,
	}
}

// Remove удаляет инструмент из реестра
func (r *ToolRegistry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	delete(r.tools, name)
	delete(r.rateLimits, name)

	return nil
}

// BuildToolDefinitions возвращает определения инструментов для LLM (tools array)
func (r *ToolRegistry) BuildToolDefinitions() []map[string]interface{} {
	tools := r.List()
	defs := make([]map[string]interface{}, 0, len(tools))

	for _, tool := range tools {
		def := map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"parameters":  tool.Schema(),
		}
		defs = append(defs, def)
	}

	return defs
}
