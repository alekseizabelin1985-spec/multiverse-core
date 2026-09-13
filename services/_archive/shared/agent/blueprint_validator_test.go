// packages/shared/agent/blueprint_validator_test.go
package agent_test

import (
	"testing"

	"multiverse-core.io/shared/agent"
)

// TestBlueprintValidator_ValidBlueprint — валидный блупринт
func TestBlueprintValidator_ValidBlueprint(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:         "test-gm",
		Type:         "game-master",
		Version:      "1.0",
		Description:  "Test blueprint",
		Phase1Prompt: "Process event: {{.EventType}}",
		Phase2Prompt: "Generate narrative: {{.Narrative}}",
		LLM: agent.LLMConfig{
			Model:       "qwen:7b",
			Temperature: 0.7,
			MaxTokens:   2048,
		},
		Tools: []agent.ToolReference{
			{Name: "world"},
			{Name: "entity"},
		},
	}

	result := validator.Validate(bp)

	if !result.IsValid {
		t.Fatalf("Expected valid blueprint, got errors: %+v", result.Errors)
	}

	t.Logf("✅ Valid blueprint: %+v", result.Info)
}

// TestBlueprintValidator_MissingRequiredFields — отсутствующие обязательные поля
func TestBlueprintValidator_MissingRequiredFields(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:    "test-gm",
		Version: "1.0",
		// Type отсутствует
	}

	result := validator.Validate(bp)

	if result.IsValid {
		t.Fatal("Expected invalid blueprint, got valid")
	}

	if len(result.Errors) == 0 {
		t.Fatal("Expected at least one error")
	}

	t.Logf("✅ Missing fields detected: %+v", result.Errors)
}

// TestBlueprintValidator_InvalidType — недопустимый тип
func TestBlueprintValidator_InvalidType(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:    "test-gm",
		Type:    "invalid-type",
		Version: "1.0",
	}

	result := validator.Validate(bp)

	if result.IsValid {
		t.Fatal("Expected invalid blueprint, got valid")
	}

	found := false
	for _, err := range result.Errors {
		if contains(err, "invalid type") {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected 'invalid type' error, got: %+v", result.Errors)
	}

	t.Logf("✅ Invalid type detected: %+v", result.Errors)
}

// TestBlueprintValidator_InvalidVersion — невалидная версия
func TestBlueprintValidator_InvalidVersion(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:    "test-gm",
		Type:    "game-master",
		Version: "1", // Не semver
	}

	result := validator.Validate(bp)

	if result.IsValid {
		t.Fatal("Expected invalid blueprint, got valid")
	}

	found := false
	for _, err := range result.Errors {
		if contains(err, "version format") {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected 'version format' error, got: %+v", result.Errors)
	}

	t.Logf("✅ Invalid version detected: %+v", result.Errors)
}

// TestBlueprintValidator_EmptyTools — пустые инструменты
func TestBlueprintValidator_EmptyTools(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:    "test-gm",
		Type:    "game-master",
		Version: "1.0",
		Tools:   []agent.ToolReference{}, // Пусто
		LLM: agent.LLMConfig{
			Model: "qwen:7b",
		},
	}

	result := validator.Validate(bp)

	if result.IsValid {
		t.Fatal("Expected invalid blueprint, got valid")
	}

	found := false
	for _, err := range result.Errors {
		if contains(err, "tool") {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected 'tool' error, got: %+v", result.Errors)
	}

	t.Logf("✅ Empty tools detected: %+v", result.Errors)
}

// TestBlueprintValidator_InvalidLLMConfig — невалидная LLM конфигурация
func TestBlueprintValidator_InvalidLLMConfig(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:    "test-gm",
		Type:    "game-master",
		Version: "1.0",
		LLM: agent.LLMConfig{
			Model:       "",  // Пустая модель
			Temperature: 3.0, // За пределами 0-2
			MaxTokens:   0,   // Неположительный
		},
		Tools: []agent.ToolReference{
			{Name: "world"},
		},
	}

	t.Logf("DEBUG: bp.LLM.Model = '%s'", bp.LLM.Model)

	result := validator.Validate(bp)

	t.Logf("DEBUG: result.IsValid = %v, result.Errors = %+v", result.IsValid, result.Errors)

	if result.IsValid {
		t.Fatal("Expected invalid blueprint, got valid")
	}

	if len(result.Errors) == 0 {
		t.Fatal("Expected at least one error")
	}

	t.Logf("✅ Invalid LLM config detected: %+v", result.Errors)
}

// TestBlueprintValidator_Warnings — предупреждения
func TestBlueprintValidator_Warnings(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:         "test-gm",
		Type:         "game-master",
		Version:      "1.0",
		Phase1Prompt: "", // Пустой
		Phase2Prompt: "", // Пустой
		LLM: agent.LLMConfig{
			Model:       "qwen:7b",
			Temperature: 0.9, // Высокий
			MaxTokens:   2048,
		},
		Tools: []agent.ToolReference{
			{Name: "world"},
		},
	}

	result := validator.Validate(bp)

	if !result.IsValid {
		t.Fatalf("Expected valid blueprint, got errors: %+v", result.Errors)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("Expected warnings, got none")
	}

	t.Logf("✅ Warnings generated: %+v", result.Warnings)
}

// TestBlueprintValidator_Suggestions — рекомендации
func TestBlueprintValidator_Suggestions(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:    "test-gm",
		Type:    "game-master",
		Version: "1.0",
		LLM: agent.LLMConfig{
			Model:       "qwen:7b",
			Temperature: 0.7,
			MaxTokens:   2048,
		},
		Tools: []agent.ToolReference{
			{Name: "world"},
		},
		// Constraints.MaxInstances не установлен
	}

	result := validator.Validate(bp)

	if !result.IsValid {
		t.Fatalf("Expected valid blueprint, got errors: %+v", result.Errors)
	}

	if len(result.Suggestions) == 0 {
		t.Fatal("Expected suggestions, got none")
	}

	t.Logf("✅ Suggestions generated: %+v", result.Suggestions)
}

// TestBlueprintValidator_CompareBlueprints — сравнение блупринтов
func TestBlueprintValidator_CompareBlueprints(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	oldBP := &agent.AgentBlueprint{
		Name:    "test-gm",
		Type:    "game-master",
		Version: "1.0",
	}

	newBP := &agent.AgentBlueprint{
		Name:    "test-gm-v2", // Изменено имя
		Type:    "game-master",
		Version: "1.1",
	}

	result := validator.CompareBlueprints(oldBP, newBP)

	if !result.HasBreakingChanges {
		t.Fatal("Expected breaking changes, got none")
	}

	found := false
	for _, change := range result.Changes {
		if contains(change, "name changed") {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected 'name changed' in changes, got: %+v", result.Changes)
	}

	t.Logf("✅ Breaking changes detected: %+v", result.Changes)
}

// TestBlueprintValidator_AllowedTypes — все допустимые типы
func TestBlueprintValidator_AllowedTypes(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	allowedTypes := []string{
		"game-master",
		"narrator",
		"entity-actor",
		"world-generator",
		"guardian",
		"oracle",
	}

	for _, tp := range allowedTypes {
		bp := &agent.AgentBlueprint{
			Name:    "test-gm",
			Type:    tp,
			Version: "1.0",
			LLM: agent.LLMConfig{
				Model:       "qwen:7b",
				Temperature: 0.7,
				MaxTokens:   2048,
			},
			Tools: []agent.ToolReference{
				{Name: "world"},
			},
		}

		result := validator.Validate(bp)

		if !result.IsValid {
			t.Fatalf("Expected type '%s' to be valid, got errors: %+v", tp, result.Errors)
		}
	}

	t.Logf("✅ All %d allowed types validated", len(allowedTypes))
}

// TestBlueprintValidator_InvalidTools — недопустимые инструменты
func TestBlueprintValidator_InvalidTools(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:    "test-gm",
		Type:    "game-master",
		Version: "1.0",
		LLM: agent.LLMConfig{
			Model: "qwen:7b",
		},
		Tools: []agent.ToolReference{
			{Name: "invalid-tool"},
		},
	}

	result := validator.Validate(bp)

	if result.IsValid {
		t.Fatal("Expected invalid blueprint, got valid")
	}

	found := false
	for _, err := range result.Errors {
		if contains(err, "unknown tool") {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected 'unknown tool' error, got: %+v", result.Errors)
	}

	t.Logf("✅ Invalid tool detected: %+v", result.Errors)
}

// TestBlueprintValidator_FullValid — полный валидный блупринт
func TestBlueprintValidator_FullValid(t *testing.T) {
	validator := agent.NewBlueprintValidator()

	bp := &agent.AgentBlueprint{
		Name:         "dark-forest-gm",
		Version:      "2.1.0",
		Description:  "GM for dark forest scenario",
		Type:         "game-master",
		Phase1Prompt: "Process event: {{.EventType}}",
		Phase2Prompt: "Generate narrative: {{.Narrative}}",
		LLM: agent.LLMConfig{
			Model:       "qwen:72b",
			Temperature: 0.7,
			MaxTokens:   4096,
		},
		Tools: []agent.ToolReference{
			{Name: "world"},
			{Name: "entity"},
			{Name: "narrative"},
		},
		Constraints: agent.BlueprintConstraints{
			MaxInstances: 5,
			Priority:     10,
		},
		TTL: "1h",
	}

	result := validator.Validate(bp)

	if !result.IsValid {
		t.Fatalf("Expected valid blueprint, got errors: %+v", result.Errors)
	}

	t.Logf("✅ Full valid blueprint: %+v", result.Info)
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
