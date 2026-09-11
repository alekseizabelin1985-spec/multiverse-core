// internal/intent/prompt_builder.go
package intent

import (
	"fmt"
	"strings"
	"time"
)

// PromptBuilder строитель промптов для Oracle
type PromptBuilder struct {
	systemPrompt string
	context      strings.Builder
	history      strings.Builder
	constraints  []string
}

// NewPromptBuilder создает новый строитель промптов
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		systemPrompt: defaultSystemPrompt,
		constraints:  make([]string, 0),
	}
}

const defaultSystemPrompt = `Ты - система распознавания намерений в RPG игре с живым миром.
Твоя задача - преобразовать текст игрока в структурированное действие для системы правил.

ВАЖНО:
1. Отвечай ТОЛЬКО валидным JSON
2. Не выдумывай правила - используй только существующие
3. Учитывай контекст мира и состояние сущности
4. Фильтруй запрещенный контент
5. Уважай предпочтения игрока (возрастные ограничения и т.д.)`

// WithSystemPrompt устанавливает системный промпт
func (b *PromptBuilder) WithSystemPrompt(prompt string) *PromptBuilder {
	b.systemPrompt = prompt
	return b
}

// WithWorldContext добавляет контекст мира
func (b *PromptBuilder) WithWorldContext(worldID, worldName, location string) *PromptBuilder {
	b.context.WriteString(fmt.Sprintf("## Контекст Мира\n"))
	b.context.WriteString(fmt.Sprintf("- Мир: %s (%s)\n", worldName, worldID))
	b.context.WriteString(fmt.Sprintf("- Локация: %s\n", location))
	b.context.WriteString("\n")
	return b
}

// WithEntityState добавляет состояние сущности
func (b *PromptBuilder) WithEntityState(entityID, entityType string, state map[string]float32) *PromptBuilder {
	b.context.WriteString(fmt.Sprintf("## Сущность\n"))
	b.context.WriteString(fmt.Sprintf("- ID: %s\n", entityID))
	b.context.WriteString(fmt.Sprintf("- Тип: %s\n", entityType))
	b.context.WriteString(fmt.Sprintf("- Состояние: %v\n", state))
	b.context.WriteString("\n")
	return b
}

// WithRecentEvents добавляет последние события
func (b *PromptBuilder) WithRecentEvents(events []string) *PromptBuilder {
	if len(events) == 0 {
		return b
	}

	b.history.WriteString("## Последние События\n")
	for i, event := range events {
		if i >= 10 { // Ограничиваем 10 событиями
			break
		}
		b.history.WriteString(fmt.Sprintf("%d. %s\n", i+1, event))
	}
	b.history.WriteString("\n")
	return b
}

// WithAvailableRules добавляет доступные правила
func (b *PromptBuilder) WithAvailableRules(rules []RuleInfo) *PromptBuilder {
	if len(rules) == 0 {
		return b
	}

	b.context.WriteString("## Доступные Правила\n")
	for _, rule := range rules {
		b.context.WriteString(fmt.Sprintf("- %s: %s\n", rule.ID, rule.Name))
	}
	b.context.WriteString("\n")
	return b
}

// WithConstraints добавляет ограничения
func (b *PromptBuilder) WithConstraints(constraints ...string) *PromptBuilder {
	b.constraints = append(b.constraints, constraints...)
	return b
}

// WithPlayerPreferences добавляет предпочтения игрока
func (b *PromptBuilder) WithPlayerPreferences(ageRating string, blockedTopics []string) *PromptBuilder {
	b.constraints = append(b.constraints, fmt.Sprintf("Возрастной рейтинг: %s", ageRating))
	if len(blockedTopics) > 0 {
		b.constraints = append(b.constraints, fmt.Sprintf("Запрещенные темы: %v", blockedTopics))
	}
	return b
}

// Build строит финальный промпт
func (b *PromptBuilder) Build(playerText string) string {
	var prompt strings.Builder

	// Системный промпт
	prompt.WriteString(b.systemPrompt)
	prompt.WriteString("\n\n")

	// Контекст
	if b.context.Len() > 0 {
		prompt.WriteString(b.context.String())
	}

	// История
	if b.history.Len() > 0 {
		prompt.WriteString(b.history.String())
	}

	// Ограничения
	if len(b.constraints) > 0 {
		prompt.WriteString("## Ограничения\n")
		for _, c := range b.constraints {
			prompt.WriteString(fmt.Sprintf("- %s\n", c))
		}
		prompt.WriteString("\n")
	}

	// Текст игрока
	prompt.WriteString("## Текст Игрока\n")
	prompt.WriteString(fmt.Sprintf("\"%s\"\n\n", playerText))

	// Формат ответа
	prompt.WriteString("## Формат Ответа\n")
	prompt.WriteString(`Верни JSON в формате:
{
  "intent": "attack|talk|move|use_item|cast_spell|examine|craft|other",
  "confidence": 0.0-1.0,
  "base_action": "базовое действие",
  "modifiers": [{"type": "...", "value": "..."}],
  "target_entity": "цель (если есть)",
  "parameters": {...},
  "requires_roll": true/false,
  "suggested_rule": "правило (если нужен бросок)",
  "reasoning": "краткое обоснование"
}

Отвечай ТОЛЬКО JSON, без дополнительного текста.`)

	return prompt.String()
}

// RuleInfo информация о правиле для промпта
type RuleInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// BuildExampleRequest создает пример запроса для тестирования
func BuildExampleRequest() IntentRequest {
	return IntentRequest{
		PlayerText:   "Атакую гоблина огненным шаром!",
		EntityID:     "player-kain-777",
		EntityType:   "player",
		WorldContext: "pain-realm",
		State: map[string]float32{
			"hp":    85.0,
			"mp":    45.0,
			"level": 5.0,
		},
		History: []string{
			"Вошли в темную пещеру",
			"Увидели гоблина-стража",
		},
		Metadata: map[string]interface{}{
			"time_of_day": "night",
			"weather":     "clear",
		},
	}
}

// IntentTemplate шаблоны для распространенных намерений
var IntentTemplates = map[string]string{
	"attack":     `{"intent": "attack", "base_action": "%s", "target_entity": "%s", "requires_roll": true, "suggested_rule": "%s"}`,
	"talk":       `{"intent": "talk", "base_action": "dialogue", "target_entity": "%s", "requires_roll": false}`,
	"move":       `{"intent": "move", "base_action": "travel", "parameters": {"destination": "%s"}, "requires_roll": false}`,
	"use_item":   `{"intent": "use_item", "base_action": "activate", "parameters": {"item": "%s"}, "requires_roll": false}`,
	"cast_spell": `{"intent": "cast_spell", "base_action": "cast", "parameters": {"spell": "%s", "target": "%s"}, "requires_roll": true, "suggested_rule": "spell_cast"}`,
	"examine":    `{"intent": "examine", "base_action": "inspect", "target_entity": "%s", "requires_roll": false}`,
	"craft":      `{"intent": "craft", "base_action": "crafting", "parameters": {"recipe": "%s"}, "requires_roll": true, "suggested_rule": "crafting_check"}`,
}

// GetIntentTemplate возвращает шаблон для намерения
func GetIntentTemplate(intent string) string {
	return IntentTemplates[intent]
}

// FormatIntent форматирует намерение из шаблона
func FormatIntent(intent string, args ...interface{}) string {
	template := GetIntentTemplate(intent)
	if template == "" {
		return ""
	}
	return fmt.Sprintf(template, args...)
}

// TimeContext возвращает контекст времени
func TimeContext(t time.Time) string {
	hour := t.Hour()
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "day"
	case hour >= 17 && hour < 22:
		return "evening"
	default:
		return "night"
	}
}
