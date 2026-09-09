// packages/shared/agent/tools/adapter.go
package tools

import (
	"context"
	"multiverse-core.io/shared/agent"
)

// ToolRegistryAdapter адаптирует tools.ToolRegistry к интерфейсу agent.ToolRegistry
type ToolRegistryAdapter struct {
	registry *ToolRegistry
}

// NewToolRegistryAdapter создает адаптер
func NewToolRegistryAdapter(registry *ToolRegistry) *ToolRegistryAdapter {
	return &ToolRegistryAdapter{
		registry: registry,
	}
}

// Register регистрирует инструмент
func (a *ToolRegistryAdapter) Register(name string, fn agent.ToolFunc, schema map[string]interface{}) error {
	// Конвертируем ToolFunc в Tool
	tool := &FuncTool{
		NameFunc:   name,
		ExecuteFunc: fn,
	}

	return a.registry.Register(tool)
}

// Get получает инструмент по имени
func (a *ToolRegistryAdapter) Get(name string) (agent.ToolFunc, bool) {
	tool, exists := a.registry.Get(name)
	if !exists {
		return nil, false
	}

	// Конвертируем Tool в ToolFunc
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return tool.Execute(ctx, args)
	}, true
}

// List возвращает список зарегистрированных инструментов
func (a *ToolRegistryAdapter) List() []string {
	tools := a.registry.List()
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name())
	}
	return names
}

// Execute выполняет инструмент
func (a *ToolRegistryAdapter) Execute(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	return a.registry.Execute(ctx, name, args)
}

// FuncTool адаптер для ToolFunc
type FuncTool struct {
	NameFunc    string
	ExecuteFunc agent.ToolFunc
}

// Name возвращает имя инструмента
func (f *FuncTool) Name() string {
	return f.NameFunc
}

// Schema возвращает схему инструмента
func (f *FuncTool) Schema() map[string]interface{} {
	return nil
}

// Description возвращает описание инструмента
func (f *FuncTool) Description() string {
	return ""
}

// Execute выполняет инструмент
func (f *FuncTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return f.ExecuteFunc(ctx, args)
}

// GetAgentToolRegistry возвращает адаптер для NarrativeAgent
func GetAgentToolRegistry(registry *ToolRegistry) agent.ToolRegistry {
	return NewToolRegistryAdapter(registry)
}
