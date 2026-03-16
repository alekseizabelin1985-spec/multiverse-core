package ruleengine

import (
	"time"
)

// Rule represents a mechanical rule that governs entity behavior
type Rule struct {
	ID             string         `json:"rule_id"`
	MechanicalCore MechanicalCore `json:"mechanical_core"`
	SemanticLayer  SemanticLayer  `json:"semantic_layer"`
	Version        int            `json:"version"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// MechanicalCore defines the core mechanical aspects of a rule
type MechanicalCore struct {
	DiceFormula         string             `json:"dice_formula"`    // "d10 + charisma"
	BaseDifficulty      int                `json:"base_difficulty"` // 12
	ContextualModifiers []Modifier         `json:"contextual_modifiers"`
	SuccessThreshold    string             `json:"success_threshold"` // "total >= difficulty"
	SensoryEffects      map[string]float32 `json:"sensory_effects"`   // visibility, sound, etc.
}

// Modifier represents a contextual modifier that can be applied to rule calculations
type Modifier struct {
	Condition string `json:"condition"` // "environment == 'intimate'"
	Modifier  int    `json:"modifier"`  // +3
}

// SemanticLayer provides semantic context for mechanical rules
type SemanticLayer struct {
	Name         string            `json:"name"`
	Descriptions map[string]string `json:"descriptions"` // mechanical, poetic
}

// RuleResult represents the outcome of applying a rule
type RuleResult struct {
	RuleID         string             `json:"rule_id"`
	DiceRoll       int                `json:"dice_roll"`
	Total          int                `json:"total"`
	Success        bool               `json:"success"`
	Context        map[string]float32 `json:"context"`
	Modifiers      []Modifier         `json:"modifiers"`
	SensoryEffects map[string]float32 `json:"sensory_effects"`
	AppliedAt      time.Time          `json:"applied_at"`
}
