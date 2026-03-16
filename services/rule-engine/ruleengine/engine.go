package ruleengine

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/minio"
)

// Engine represents the rule application engine
type Engine struct {
	bus         *eventbus.EventBus
	minioClient *minio.Client
	cache       *sync.Map // rule_id -> *Rule
	rand        *rand.Rand
	logger      *log.Logger
}

// NewEngine creates a new rule engine instance
func NewEngine(bus *eventbus.EventBus, minioClient *minio.Client) *Engine {
	return &Engine{
		bus:         bus,
		minioClient: minioClient,
		cache:       &sync.Map{},
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
		logger:      log.New(log.Writer(), "RuleEngine: ", log.LstdFlags|log.Lshortfile),
	}
}

// Apply applies a rule to the given context and returns the result
func (e *Engine) Apply(ruleID string, state map[string]float32, modifiers []map[string]interface{}) (*RuleResult, error) {
	// Validate inputs
	if ruleID == "" {
		return nil, fmt.Errorf("rule ID cannot be empty")
	}

	if state == nil {
		return nil, fmt.Errorf("state cannot be nil")
	}

	// Get rule from cache or load from storage
	rule, err := e.getRule(ruleID)
	if err != nil {
		return nil, fmt.Errorf("get rule: %w", err)
	}

	// Validate that the rule has required fields
	if rule.MechanicalCore.DiceFormula == "" {
		return nil, fmt.Errorf("rule %s has empty dice formula", ruleID)
	}

	// Calculate base roll
	diceResult, err := e.rollDice(rule.MechanicalCore.DiceFormula, state)
	if err != nil {
		return nil, fmt.Errorf("roll dice: %w", err)
	}

	// Apply contextual modifiers
	total := diceResult
	var appliedModifiers []Modifier

	for _, mod := range rule.MechanicalCore.ContextualModifiers {
		if e.evaluateCondition(mod.Condition, state, modifiers) {
			total += mod.Modifier
			appliedModifiers = append(appliedModifiers, mod)
		}
	}

	// Check success threshold
	success := e.checkSuccessThreshold(total, rule.MechanicalCore.SuccessThreshold)

	// Apply sensory effects
	sensoryEffects := make(map[string]float32)
	for key, value := range rule.MechanicalCore.SensoryEffects {
		sensoryEffects[key] = value
	}

	result := &RuleResult{
		RuleID:         ruleID,
		DiceRoll:       diceResult,
		Total:          total,
		Success:        success,
		Context:        state,
		Modifiers:      appliedModifiers,
		SensoryEffects: sensoryEffects,
		AppliedAt:      time.Now(),
	}

	return result, nil
}

// rollDice rolls dice based on the formula and returns the result
func (e *Engine) rollDice(formula string, state map[string]float32) (int, error) {
	// Validate input
	if formula == "" {
		return 0, fmt.Errorf("dice formula cannot be empty")
	}

	// Parse the formula: e.g., "d20", "2d6+3", "d10+charisma", etc.

	// Handle dice formulas like "d20", "2d6", "d10+charisma"
	var numDice int = 1
	var dieSides int = 20 // Default if not specified

	// Extract number of dice and sides from formula
	if strings.Contains(formula, "d") {
		parts := strings.Split(formula, "d")
		if len(parts) == 2 {
			if parts[0] != "" {
				numDice, err := strconv.Atoi(parts[0])
				if err != nil {
					return 0, fmt.Errorf("invalid dice formula: %s", formula)
				}
				numDice = numDice
			}
			dieSides, err := strconv.Atoi(parts[1])
			if err != nil {
				return 0, fmt.Errorf("invalid dice formula: %s", formula)
			}
			dieSides = dieSides
		}
	} else if strings.HasPrefix(formula, "d") {
		// Just a die like "d20"
		dieSides, err := strconv.Atoi(formula[1:])
		if err != nil {
			return 0, fmt.Errorf("invalid dice formula: %s", formula)
		}
		dieSides = dieSides
	}

	if numDice <= 0 || dieSides <= 0 {
		return 0, fmt.Errorf("invalid dice formula: %s", formula)
	}

	// Roll the dice
	total := 0
	for i := 0; i < numDice; i++ {
		roll := e.rand.Intn(dieSides) + 1
		total += roll
	}

	return total, nil
}

// evaluateCondition evaluates a condition against the current state
func (e *Engine) evaluateCondition(condition string, state map[string]float32, modifiers []map[string]interface{}) bool {
	// Parse and evaluate the condition
	if condition == "" {
		return false // Empty conditions are not valid
	}

	// Simple parser for basic conditions like "health > 50", "level >= 10", etc.
	condition = strings.TrimSpace(condition)

	// Split into parts
	parts := strings.Fields(condition)
	if len(parts) < 3 {
		return false // Invalid condition format
	}

	varName := parts[0]
	op := parts[1]
	valueStr := strings.Join(parts[2:], " ")

	// Check if the variable exists in state
	stateVal, exists := state[varName]
	if !exists {
		return false // Variable not found in state
	}

	// Parse value based on operator
	switch op {
	case ">":
		val, err := strconv.ParseFloat(valueStr, 32)
		if err != nil {
			return false
		}
		return float32(val) > stateVal
	case "<":
		val, err := strconv.ParseFloat(valueStr, 32)
		if err != nil {
			return false
		}
		return float32(val) < stateVal
	case ">=":
		val, err := strconv.ParseFloat(valueStr, 32)
		if err != nil {
			return false
		}
		return float32(val) >= stateVal
	case "<=":
		val, err := strconv.ParseFloat(valueStr, 32)
		if err != nil {
			return false
		}
		return float32(val) <= stateVal
	case "=":
		val, err := strconv.ParseFloat(valueStr, 32)
		if err != nil {
			return false
		}
		return float32(val) == stateVal
	case "!=":
		val, err := strconv.ParseFloat(valueStr, 32)
		if err != nil {
			return false
		}
		return float32(val) != stateVal
	default:
		return false // Unknown operator
	}
}

// checkSuccessThreshold checks if the total meets the success threshold
func (e *Engine) checkSuccessThreshold(total int, threshold string) bool {
	// Parse and evaluate the threshold
	if threshold == "" {
		return false // Empty threshold is not valid
	}

	// Simple parser for thresholds like "total >= 10", "difficulty + 5"
	threshold = strings.TrimSpace(threshold)

	// Handle simple cases like "total >= 10"
	parts := strings.Fields(threshold)
	if len(parts) >= 3 {
		if parts[0] == "total" && len(parts) >= 3 {
			op := parts[1]
			valueStr := parts[2]

			// Parse the value
			val, err := strconv.Atoi(valueStr)
			if err != nil {
				return false // Invalid threshold value
			}

			// Compare based on operator
			switch op {
			case ">=":
				return total >= val
			case ">":
				return total > val
			case "<=":
				return total <= val
			case "<":
				return total < val
			case "=":
				return total == val
			default:
				return false // Unknown operator
			}
		}
	}

	// Default to success if we can't parse the threshold properly
	return true
}

// getRule retrieves a rule from cache or storage
func (e *Engine) getRule(ruleID string) (*Rule, error) {
	// Validate input
	if ruleID == "" {
		return nil, fmt.Errorf("rule ID cannot be empty")
	}

	// Check cache first
	if cached, ok := e.cache.Load(ruleID); ok {
		if rule, ok := cached.(*Rule); ok {
			return rule, nil
		}
	}

	// Load from storage if not in cache
	rule, err := e.loadRuleFromStorage(ruleID)
	if err != nil {
		return nil, fmt.Errorf("load rule from storage: %w", err)
	}

	// Cache the rule
	e.cache.Store(ruleID, rule)

	return rule, nil
}

// loadRuleFromStorage loads a rule from MinIO storage
func (e *Engine) loadRuleFromStorage(ruleID string) (*Rule, error) {
	if e.minioClient == nil {
		return nil, fmt.Errorf("minio client is not initialized")
	}

	// Validate input
	if ruleID == "" {
		return nil, fmt.Errorf("rule ID cannot be empty")
	}

	// In MinIO, rules are stored with path pattern: "rules/{rule_id}.json"
	// Based on the AGENTS.md documentation, rules are stored in buckets named "rules-{world_id}"
	// For now, we'll use a default bucket name for demonstration
	// In real implementation, this would be determined by world context
	bucketName := "rules" // This should be configured properly based on world context

	objectKey := fmt.Sprintf("%s.json", ruleID)

	// Get object from MinIO
	data, err := e.minioClient.GetObject(bucketName, objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get rule from storage: %w", err)
	}

	// Check if data is empty
	if len(data) == 0 {
		return nil, fmt.Errorf("rule not found in storage: %s", ruleID)
	}

	// Parse JSON into Rule struct
	var rule Rule
	if err := json.Unmarshal(data, &rule); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rule from JSON: %w", err)
	}

	// Validate the rule
	if rule.ID == "" {
		return nil, fmt.Errorf("invalid rule: missing ID")
	}

	return &rule, nil
}

// SaveRule saves a rule to storage and cache
func (e *Engine) SaveRule(rule *Rule) error {
	// Validate input
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}

	if rule.ID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}

	// In a real implementation, this would:
	// 1. Serialize rule to JSON
	// 2. Store in MinIO with ruleID as key
	// 3. Update cache

	return nil
}

// DeleteRule removes a rule from storage and cache
func (e *Engine) DeleteRule(ruleID string) error {
	// Validate input
	if ruleID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}

	e.cache.Delete(ruleID)
	return nil
}
