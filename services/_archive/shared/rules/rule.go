// internal/rules/rule.go
package rules

import (
	"time"
)

// Rule представляет правило с разделением на механику и нарратив
type Rule struct {
	// Метаданные
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	EntityType  string    `json:"entity_type"` // Тип сущностей, к которым применяется
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Механическое ядро - чистая математика без нарратива
	MechanicalCore MechanicalCore `json:"mechanical_core"`

	// Семантический слой - нарративные описания
	SemanticLayer SemanticLayer `json:"semantic_layer"`

	// Баланс и ограничения
	BalanceScore float32 `json:"balance_score"` // 0.0-1.0
	SafetyLevel  string  `json:"safety_level"`  // "safe", "review_required", "blocked"
}

// MechanicalCore чистая механика без нарратива
type MechanicalCore struct {
	// Формула броска кубиков: "d20", "2d6+3", "d10+charisma"
	DiceFormula string `json:"dice_formula"`

	// Порог успеха: "total >= 10", "difficulty + 5"
	SuccessThreshold string `json:"success_threshold"`

	// Контекстные модификаторы
	ContextualModifiers []Modifier `json:"contextual_modifiers"`

	// Сенсорные эффекты (числовые значения)
	SensoryEffects map[string]float32 `json:"sensory_effects"`

	// Изменения состояния
	StateChanges []StateChange `json:"state_changes"`

	// Требования для применения
	Requirements []Requirement `json:"requirements"`
}

// Modifier модификатор к броску
type Modifier struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Condition   string `json:"condition"`   // Условие применения: "health > 50", "is_night"
	Modifier    int    `json:"modifier"`    // Значение модификатора
	SourceType  string `json:"source_type"` // "buff", "debuff", "environment", "item"
	Description string `json:"description"` // Краткое описание
}

// StateChange изменение состояния сущности
type StateChange struct {
	Path      string  `json:"path"`      // Путь к полю: "stats.hp", "inventory"
	Operation string  `json:"operation"` // "set", "add", "subtract", "multiply"
	Value     float32 `json:"value"`     // Значение
	Duration  int     `json:"duration"`  // Длительность в секундах (0 = постоянно)
}

// Requirement требование для применения правила
type Requirement struct {
	Type           string  `json:"type"`            // "stat", "item", "location", "time"
	Attribute      string  `json:"attribute"`       // Атрибут для проверки
	MinValue       float32 `json:"min_value"`       // Минимальное значение
	MaxValue       float32 `json:"max_value"`       // Максимальное значение
	Consumable     bool    `json:"consumable"`      // Потребляется ли ресурс
	RequiredAmount int     `json:"required_amount"` // Требуемое количество
}

// SemanticLayer семантический слой для нарратива
type SemanticLayer struct {
	// Поэтическое описание результата
	PoeticDescription string `json:"poetic_description"`

	// Описание успеха
	SuccessDescription string `json:"success_description"`

	// Описание провала
	FailureDescription string `json:"failure_description"`

	// Описание критического успеха
	CriticalSuccessDescription string `json:"critical_success_description"`

	// Описание критического провала
	CriticalFailureDescription string `json:"critical_failure_description"`

	// Контекстные подсказки для GM
	ContextHints []string `json:"context_hints"`

	// Эмоциональная окраска
	EmotionalTone string `json:"emotional_tone"` // "heroic", "dark", "mysterious", "comedic"

	// Стилистические маркеры
	StyleMarkers []string `json:"style_markers"` // ["ancient_prose", "modern_slang", "poetic"]

	// Запрещенные темы (для фильтрации)
	BlockedTopics []string `json:"blocked_topics"`
}

// RuleResult результат применения правила
type RuleResult struct {
	RuleID         string             `json:"rule_id"`
	RuleVersion    string             `json:"rule_version"`
	DiceRoll       int                `json:"dice_roll"`
	DiceFormula    string             `json:"dice_formula"`
	Total          int                `json:"total"`
	Success        bool               `json:"success"`
	Critical       bool               `json:"critical"` // Критический успех/провал
	Context        map[string]float32 `json:"context"`
	Modifiers      []AppliedModifier  `json:"modifiers"`
	SensoryEffects map[string]float32 `json:"sensory_effects"`
	StateChanges   []StateChange      `json:"state_changes"`
	MechanicalOnly bool               `json:"mechanical_only"` // Только механика, без нарратива
	AppliedAt      time.Time          `json:"applied_at"`
}

// AppliedModifier примененный модификатор
type AppliedModifier struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Value      int    `json:"value"`
	Condition  string `json:"condition"`
	WasApplied bool   `json:"was_applied"` // Было ли применено (условие выполнено)
}

// RuleProposal предложение нового правила от Oracle
type RuleProposal struct {
	AnomalyContext string  `json:"anomaly_context"`
	ProposedRule   Rule    `json:"proposed_rule"`
	Reasoning      string  `json:"reasoning"`
	Confidence     float32 `json:"confidence"`   // 0.0-1.0
	ImpactScore    float32 `json:"impact_score"` // Ожидаемое влияние на баланс
}

// RuleValidationResult результат валидации правила
type RuleValidationResult struct {
	RuleID          string   `json:"rule_id"`
	IsValid         bool     `json:"is_valid"`
	SafetyPassed    bool     `json:"safety_passed"`
	BalancePassed   bool     `json:"balance_passed"`
	Issues          []string `json:"issues"`
	Recommendations []string `json:"recommendations"`
	BalanceScore    float32  `json:"balance_score"`
	SafetyLevel     string   `json:"safety_level"`
}
