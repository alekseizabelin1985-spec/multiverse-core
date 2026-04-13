// agentruntime/action/mechanical_result.go
package action

import "multiverse-core.io/shared/rules"

// MechanicalResult — выход Phase1 LLM ("Судья") и вход для Phase2 LLM ("Сказитель").
// Расширяет rules.RuleResult новыми полями от LLM-интерпретатора.
type MechanicalResult struct {
	// Из rules.RuleResult (механика движка)
	RuleID      string `json:"rule_id"`
	RuleVersion string `json:"rule_version"`
	DiceFormula string `json:"dice_formula"`
	DiceRoll    int    `json:"dice_roll"`
	Total       int    `json:"total"`

	// Решение Phase1 LLM
	Hit      bool    `json:"hit"`
	Damage   int     `json:"damage"`
	Critical bool    `json:"critical"`

	// Состояние цели после удара
	TargetHPAfter float64 `json:"target_hp_after"`

	// Классификатор исхода — ключ для Phase2 и L3 кэша
	// Значения: "kill" | "wound" | "graze" | "miss" | "reflected" | "blocked"
	OutcomeTag string `json:"outcome_tag"`

	// Статус-эффекты: "stunned", "bleeding", "reflected", "poisoned", etc.
	StatusEffects []string `json:"status_effects"`

	// Специальные эффекты (отражение, цепная молния, etc.)
	SpecialEffects map[string]any `json:"special_effects,omitempty"`

	// Контекст для Phase2
	AttackerID string `json:"attacker_id"`
	TargetID   string `json:"target_id"`

	// SemanticLayer из правила — стиль и тон для Phase2 нарратива
	SemanticHints rules.SemanticLayer `json:"semantic_hints"`
}

// OutcomeTag константы
const (
	OutcomeKill      = "kill"
	OutcomeWound     = "wound"
	OutcomeGraze     = "graze"
	OutcomeMiss      = "miss"
	OutcomeReflected = "reflected"
	OutcomeBlocked   = "blocked"
)

// IsFatal возвращает true если цель уничтожена (kill или TargetHPAfter <= 0)
func (r *MechanicalResult) IsFatal() bool {
	return r.OutcomeTag == OutcomeKill || r.TargetHPAfter <= 0
}
