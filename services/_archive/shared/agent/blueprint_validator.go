// packages/shared/agent/blueprint_validator.go
package agent

import (
	"fmt"
	"strings"
)

// BlueprintValidator валидирует MD-блупринты
type BlueprintValidator struct {
	// requiredSections обязательные секции в MD
	requiredSections []string

	// optionalSections опциональные секции
	optionalSections []string

	// allowedTypes допустимые типы агентов
	allowedTypes []string

	// allowedLevels допустимые уровни
	allowedLevels []string
}

// BlueprintValidationResult результат валидации
type BlueprintValidationResult struct {
	IsValid     bool     `json:"is_valid"`
	Errors      []string `json:"errors"`
	Warnings    []string `json:"warnings"`
	Info        []string `json:"info"`
	Suggestions []string `json:"suggestions"`
}

// NewBlueprintValidator создает новый валидатор
func NewBlueprintValidator() *BlueprintValidator {
	return &BlueprintValidator{
		requiredSections: []string{
			"name",
			"type",
			"version",
		},
		optionalSections: []string{
			"description",
			"phase1_prompt",
			"phase2_prompt",
			"tools",
			"llm",
			"trigger",
			"constraints",
			"parent",
			"ttl",
		},
		allowedTypes: []string{
			"game-master",
			"narrator",
			"entity-actor",
			"world-generator",
			"guardian",
			"oracle",
		},
		allowedLevels: []string{
			"global",
			"domain",
			"task",
			"object",
			"monitor",
		},
	}
}

// Validate валидирует блупринт
func (v *BlueprintValidator) Validate(blueprint *AgentBlueprint) *BlueprintValidationResult {
	result := &BlueprintValidationResult{
		IsValid: true,
	}

	// 1. Проверяем required fields
	if err := v.validateRequiredFields(blueprint); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// 2. Проверяем type
	if err := v.validateType(blueprint.Type); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// 3. Проверяем version
	if err := v.validateVersion(blueprint.Version); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// 4. Проверяем tools
	if err := v.validateTools(blueprint.Tools); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// 5. Проверяем LLM config
	if err := v.validateLLMConfig(blueprint.LLM); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// 6. Проверяем что tools не пустой
	if len(blueprint.Tools) == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "at least one tool is required")
	}

	// 7. Генерируем warnings
	if warnings := v.generateWarnings(blueprint); len(warnings) > 0 {
		result.Warnings = warnings
	}

	// 8. Генерируем suggestions
	if suggestions := v.generateSuggestions(blueprint); len(suggestions) > 0 {
		result.Suggestions = suggestions
	}

	// 9. Генерируем info
	if info := v.generateInfo(blueprint); len(info) > 0 {
		result.Info = info
	}

	return result
}

// validateRequiredFields проверяет обязательные поля
func (v *BlueprintValidator) validateRequiredFields(bp *AgentBlueprint) error {
	var missing []string

	if bp.Name == "" {
		missing = append(missing, "name")
	}
	if bp.Type == "" {
		missing = append(missing, "type")
	}
	if bp.Version == "" {
		missing = append(missing, "version")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	return nil
}

// validateType проверяет тип агента
func (v *BlueprintValidator) validateType(tp string) error {
	for _, allowed := range v.allowedTypes {
		if tp == allowed {
			return nil
		}
	}

	return fmt.Errorf("invalid type '%s', allowed types: %s", tp, strings.Join(v.allowedTypes, ", "))
}

// validateVersion проверяет версию
func (v *BlueprintValidator) validateVersion(version string) error {
	// Проверяем формат semver
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid version format '%s', expected semver (e.g., 1.0, 1.0.0)", version)
	}

	return nil
}

// validateTools проверяет инструменты
func (v *BlueprintValidator) validateTools(tools []ToolReference) error {
	if len(tools) == 0 {
		return fmt.Errorf("at least one tool is required")
	}

	validTools := map[string]bool{
		"world":     true,
		"entity":    true,
		"narrative": true,
		"combat":    true,
		"dialogue":  true,
		"quest":     true,
		"economy":   true,
		"spatial":   true,
	}

	for _, tool := range tools {
		if tool.Name == "" {
			return fmt.Errorf("tool name cannot be empty")
		}
		if !validTools[tool.Name] {
			return fmt.Errorf("unknown tool '%s', valid tools: %s", tool.Name, getValidToolNames())
		}
	}

	return nil
}

// validateLLMConfig проверяет LLM конфигурацию
func (v *BlueprintValidator) validateLLMConfig(llm LLMConfig) error {
	// Если модель не указана — ошибка
	if llm.Model == "" {
		return fmt.Errorf("LLM model is required (got: '%s')", llm.Model)
	}

	// Если температура указана — проверяем диапазон
	if llm.Temperature != 0 && (llm.Temperature < 0 || llm.Temperature > 2) {
		return fmt.Errorf("LLM temperature must be between 0 and 2, got %.2f", llm.Temperature)
	}

	// Если max_tokens указан — проверяем что положительный
	if llm.MaxTokens <= 0 {
		return fmt.Errorf("LLM max_tokens must be positive, got %d", llm.MaxTokens)
	}

	return nil
}

// generateWarnings генерирует предупреждения
func (v *BlueprintValidator) generateWarnings(bp *AgentBlueprint) []string {
	var warnings []string

	if bp.Description == "" {
		warnings = append(warnings, "Consider adding a description for better documentation")
	}

	if bp.Phase1Prompt == "" {
		warnings = append(warnings, "Phase 1 prompt is empty - agent may not process events correctly")
	}

	if bp.Phase2Prompt == "" {
		warnings = append(warnings, "Phase 2 prompt is empty - agent may not generate narratives")
	}

	if bp.LLM.Temperature > 0.8 {
		warnings = append(warnings, "High LLM temperature may cause unpredictable behavior")
	}

	return warnings
}

// generateSuggestions генерирует рекомендации
func (v *BlueprintValidator) generateSuggestions(bp *AgentBlueprint) []string {
	var suggestions []string

	if bp.Constraints.MaxInstances <= 0 {
		suggestions = append(suggestions, "Set constraints.max_instances to limit agent spawning")
	}

	if bp.TTL == "" && bp.Type == "game-master" {
		suggestions = append(suggestions, "Consider setting a TTL for domain-level agents")
	}

	return suggestions
}

// generateInfo генерирует информационные сообщения
func (v *BlueprintValidator) generateInfo(bp *AgentBlueprint) []string {
	var info []string

	info = append(info, fmt.Sprintf("Blueprint: %s v%s", bp.Name, bp.Version))
	info = append(info, fmt.Sprintf("Type: %s", bp.Type))
	info = append(info, fmt.Sprintf("Tools: %d registered", len(bp.Tools)))

	return info
}

// getValidToolNames возвращает список валидных инструментов
func getValidToolNames() string {
	tools := []string{"world", "entity", "narrative", "combat", "dialogue", "quest", "economy", "spatial"}
	var result []string
	for _, t := range tools {
		result = append(result, fmt.Sprintf("'%s'", t))
	}
	return strings.Join(result, ", ")
}

// ValidateMDString валидирует MD-строку
func (v *BlueprintValidator) ValidateMDString(mdContent string) *BlueprintValidationResult {
	// TODO: реализовать парсинг MD в Blueprint
	// Для пока возвращаем ошибку
	return &BlueprintValidationResult{
		IsValid: false,
		Errors:  []string{"MD parsing not implemented yet"},
	}
}

// ValidateMDFile валидирует MD-файл
func (v *BlueprintValidator) ValidateMDFile(filePath string) (*BlueprintValidationResult, error) {
	// TODO: реализовать чтение файла
	// Для пока возвращаем ошибку
	return nil, fmt.Errorf("file validation not implemented yet")
}

// CompareBlueprints сравнивает два блупринта
func (v *BlueprintValidator) CompareBlueprints(old, newBP *AgentBlueprint) *BlueprintComparisonResult {
	result := &BlueprintComparisonResult{
		HasBreakingChanges: false,
		Changes:            []string{},
	}

	// Проверяем breaking changes
	if old.Name != newBP.Name {
		result.HasBreakingChanges = true
		result.Changes = append(result.Changes, "name changed")
	}

	if old.Type != newBP.Type {
		result.HasBreakingChanges = true
		result.Changes = append(result.Changes, "type changed")
	}

	// Проверяем версии
	if !v.isVersionUpgrade(old.Version, newBP.Version) {
		result.Changes = append(result.Changes, "version not bumped")
	}

	return result
}

// isVersionUpgrade проверяет что новая версия больше старой
func (v *BlueprintValidator) isVersionUpgrade(old, newVersion string) bool {
	// Простая проверка: если версии отличаются, считаем что это апгрейд
	return old != newVersion
}

// BlueprintComparisonResult результат сравнения блупринтов
type BlueprintComparisonResult struct {
	HasBreakingChanges bool     `json:"has_breaking_changes"`
	Changes            []string `json:"changes"`
}
