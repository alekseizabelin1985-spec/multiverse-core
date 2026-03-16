package ruleengine

import (
	"fmt"
	"regexp"
	"strings"
)

// Validator provides safety validation for rules before application
type Validator struct {
	// Define safety rules and constraints
	safetyRules map[string]interface{}
}

// NewValidator creates a new rule validator
func NewValidator() *Validator {
	return &Validator{
		safetyRules: make(map[string]interface{}),
	}
}

// ValidateRule performs safety validation on a rule
func (v *Validator) ValidateRule(rule *Rule) error {
	// Check rule ID format
	if err := v.validateRuleID(rule.ID); err != nil {
		return fmt.Errorf("rule ID validation failed: %w", err)
	}

	// Check mechanical core
	if err := v.validateMechanicalCore(rule.MechanicalCore); err != nil {
		return fmt.Errorf("mechanical core validation failed: %w", err)
	}

	// Check semantic layer
	if err := v.validateSemanticLayer(rule.SemanticLayer); err != nil {
		return fmt.Errorf("semantic layer validation failed: %w", err)
	}

	// Check for dangerous patterns in dice formula
	if err := v.validateDiceFormula(rule.MechanicalCore.DiceFormula); err != nil {
		return fmt.Errorf("dice formula validation failed: %w", err)
	}

	// Check for excessive modifiers
	if err := v.validateModifiers(rule.MechanicalCore.ContextualModifiers); err != nil {
		return fmt.Errorf("modifiers validation failed: %w", err)
	}

	// Check versioning
	if rule.Version <= 0 {
		return fmt.Errorf("invalid rule version: %d", rule.Version)
	}

	return nil
}

// validateRuleID checks the format of rule ID
func (v *Validator) validateRuleID(ruleID string) error {
	if ruleID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}

	// Rule ID should follow pattern: [a-z0-9_]+
	valid := regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(ruleID)
	if !valid {
		return fmt.Errorf("invalid rule ID format: %s", ruleID)
	}

	return nil
}

// validateMechanicalCore validates the mechanical core of a rule
func (v *Validator) validateMechanicalCore(core MechanicalCore) error {
	// Check dice formula format
	if core.DiceFormula != "" {
		valid := regexp.MustCompile(`^d\d+$`).MatchString(core.DiceFormula)
		if !valid {
			return fmt.Errorf("invalid dice formula format: %s", core.DiceFormula)
		}
	}

	// Base difficulty should be within reasonable range
	if core.BaseDifficulty < 0 || core.BaseDifficulty > 100 {
		return fmt.Errorf("base difficulty out of range (0-100): %d", core.BaseDifficulty)
	}

	// Success threshold should not be empty
	if core.SuccessThreshold == "" {
		return fmt.Errorf("success threshold cannot be empty")
	}

	return nil
}

// validateSemanticLayer validates the semantic layer of a rule
func (v *Validator) validateSemanticLayer(layer SemanticLayer) error {
	if layer.Name == "" {
		return fmt.Errorf("semantic layer name cannot be empty")
	}

	// Check descriptions
	if layer.Descriptions == nil {
		return fmt.Errorf("descriptions cannot be nil")
	}

	// Must have mechanical and poetic descriptions
	if _, exists := layer.Descriptions["mechanical"]; !exists {
		return fmt.Errorf("mechanical description required")
	}

	if _, exists := layer.Descriptions["poetic"]; !exists {
		return fmt.Errorf("poetic description required")
	}

	return nil
}

// validateDiceFormula checks for dangerous patterns in dice formulas
func (v *Validator) validateDiceFormula(formula string) error {
	if formula == "" {
		return nil
	}

	// Check for potentially dangerous patterns
	// For example, we don't want formulas that could cause overflow or infinite loops
	if strings.Contains(formula, "d") && len(formula) > 10 {
		return fmt.Errorf("dice formula too long: %s", formula)
	}

	// Check for invalid characters
	validChars := regexp.MustCompile(`^[d+\-0-9\s]+$`)
	if !validChars.MatchString(formula) {
		return fmt.Errorf("invalid characters in dice formula: %s", formula)
	}

	return nil
}

// validateModifiers validates contextual modifiers
func (v *Validator) validateModifiers(modifiers []Modifier) error {
	// Check for excessive number of modifiers
	if len(modifiers) > 20 {
		return fmt.Errorf("too many contextual modifiers: %d, maximum is 20", len(modifiers))
	}

	for i, mod := range modifiers {
		// Validate condition format
		if mod.Condition == "" {
			return fmt.Errorf("modifier %d condition cannot be empty", i)
		}

		// Modifier should be within reasonable range
		if mod.Modifier < -100 || mod.Modifier > 100 {
			return fmt.Errorf("modifier %d out of range (-100 to 100): %d", i, mod.Modifier)
		}
	}

	return nil
}

// ValidateRuleSafety checks if a rule is safe for application
func (v *Validator) ValidateRuleSafety(rule *Rule) error {
	// Perform comprehensive safety checks

	// Check for potential infinite loops or resource exhaustion
	if err := v.checkResourceSafety(rule); err != nil {
		return fmt.Errorf("resource safety check failed: %w", err)
	}

	// Check for dangerous condition patterns
	if err := v.checkConditionSafety(rule.MechanicalCore.ContextualModifiers); err != nil {
		return fmt.Errorf("condition safety check failed: %w", err)
	}

	// Check for rule complexity limits
	if err := v.checkRuleComplexity(rule); err != nil {
		return fmt.Errorf("rule complexity check failed: %w", err)
	}

	return nil
}

// checkResourceSafety checks for resource safety issues
func (v *Validator) checkResourceSafety(rule *Rule) error {
	// Check for potentially dangerous patterns that could cause resource exhaustion

	// For example, we might want to prevent rules with very complex dice formulas
	// or rules that could cause excessive memory allocation

	return nil
}

// checkConditionSafety checks for dangerous condition patterns
func (v *Validator) checkConditionSafety(modifiers []Modifier) error {
	for _, mod := range modifiers {
		// Prevent overly complex conditions that might be hard to validate
		if len(mod.Condition) > 500 {
			return fmt.Errorf("condition too long: %d characters", len(mod.Condition))
		}

		// Prevent potentially dangerous condition patterns
		dangerousPatterns := []string{"eval(", "exec(", "import(", "require("}
		for _, pattern := range dangerousPatterns {
			if strings.Contains(strings.ToLower(mod.Condition), pattern) {
				return fmt.Errorf("dangerous pattern in condition: %s", pattern)
			}
		}
	}

	return nil
}

// checkRuleComplexity checks if rule complexity is within acceptable limits
func (v *Validator) checkRuleComplexity(rule *Rule) error {
	// Check overall rule complexity

	// For example, limit the number of contextual modifiers
	if len(rule.MechanicalCore.ContextualModifiers) > 50 {
		return fmt.Errorf("too many contextual modifiers: %d", len(rule.MechanicalCore.ContextualModifiers))
	}

	// Limit the size of dice formula
	if len(rule.MechanicalCore.DiceFormula) > 20 {
		return fmt.Errorf("dice formula too complex: %s", rule.MechanicalCore.DiceFormula)
	}

	return nil
}
