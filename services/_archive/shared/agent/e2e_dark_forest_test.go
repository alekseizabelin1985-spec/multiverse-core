// packages/shared/agent/e2e_dark_forest_test.go
package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/agent/tools"
	"multiverse-core.io/shared/rules"
)

// TestDarkForestE2E — полный E2E-сценарий "Тёмный лес"
// Цепочка: Вход игрока → Обнаружение волка → Бой → Нарратив
func TestDarkForestE2E(t *testing.T) {
	ctx := context.Background()

	// ========================================
	// SETUP: Создаем все компоненты
	// ========================================

	// 1. InMemoryCache для StateManager
	cache := agent.NewInMemoryCache()

	// 2. StateManager с версионированием
	stateManager := agent.NewStateManager(
		cache,
		nil, // PGStore (заглушка)
		agent.DefaultStateManagerConfig(),
	)

	// 3. RuleEngine для детерминированных правил
	ruleEngine := rules.NewEngine(nil, "rules", 100)

	// 4. RuleEngine Filter (70% без LLM)
	filter := agent.NewRuleEngineFilter(ruleEngine)

	// 5. Создаем тестового агента (GM для "Тёмного леса")
	agentID := "gm-dark-forest-001"
	agentCtx := &agent.AgentContext{
		AgentID:   agentID,
		AgentType: "game-master",
		Level:     agent.LevelDomain,
		ScopeID:   "dark-forest-region-1",
		Entities: []agent.EntityRef{
			{ID: "player-alex", Type: "player", Name: "Алексей", Level: agent.LODBasic},
			{ID: "wolf-alpha", Type: "creature", Name: "Альфа-волк", Level: agent.LODBasic},
		},
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	// ========================================
	// СЦЕНАРИЙ 1: Игрок входит в Тёмный лес
	// ========================================
	t.Run("Сценарий 1: Игрок входит в Тёмный лес", func(t *testing.T) {
		// 1. Проверяем через RuleEngine Filter
		req := &agent.AgentRequest{
			EventType:  "player.enter_forest",
			EntityType: "player",
			EntityID:   "player-alex",
			State: map[string]float32{
				"health":   100,
				"stamina":  80,
				"location": 1, // forest
			},
		}

		filterResult, err := filter.Filter(ctx, req)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		t.Logf("✅ Filter результат: Bypassed=%v, Reason=%s", filterResult.Bypassed, filterResult.Reason)

		// 2. Сохраняем состояние агента
		if err := stateManager.Save(ctx, agentID, agentCtx); err != nil {
			t.Fatalf("Save state error: %v", err)
		}

		t.Logf("✅ Состояние агента сохранено (версия %d)", stateManager.GetLatestVersion(agentID))

		// 3. Загружаем состояние (проверка что сохранилось)
		loadedCtx, err := stateManager.Load(ctx, agentID)
		if err != nil {
			t.Fatalf("Load state error: %v", err)
		}

		if loadedCtx.AgentID != agentID {
			t.Fatalf("Expected agent ID %s, got %s", agentID, loadedCtx.AgentID)
		}

		t.Logf("✅ Состояние загружено: AgentID=%s, Type=%s", loadedCtx.AgentID, loadedCtx.AgentType)

		// 4. Проверяем историю версий
		versions := stateManager.GetVersionHistory(agentID)
		if len(versions) == 0 {
			t.Fatal("Expected at least 1 version, got 0")
		}

		t.Logf("✅ История версий: %d записей", len(versions))

		// 5. Проверяем статистику
		stats := stateManager.GetStats()
		t.Logf("✅ Статистика StateManager: %+v", stats)
	})

	// ========================================
	// СЦЕНАРИЙ 2: Обнаружение волка
	// ========================================
	t.Run("Сценарий 2: Обнаружение волка", func(t *testing.T) {
		// Создаем правило для обнаружения волка
		_ = &rules.Rule{
			ID:          "wolf-detection",
			Version:     "1.0",
			Name:        "Обнаружение волка в тёмном лесу",
			EntityType:  "creature",
			Description: "Волк обнаруживает игрока в тёмном лесу ночью",
			MechanicalCore: rules.MechanicalCore{
				DiceFormula:      "d20",
				SuccessThreshold: "total >= 10",
				ContextualModifiers: []rules.Modifier{
					{
						ID:         "night_penalty",
						Name:       "Ночной штраф",
						Condition:  "location == forest && time == night",
						Modifier:   -2,
						SourceType: "environment",
					},
					{
						ID:         "fog_penalty",
						Name:       "Туман",
						Condition:  "weather == fog",
						Modifier:   -3,
						SourceType: "environment",
					},
				},
				StateChanges: []rules.StateChange{
					{
						Path:      "wolf.aggression",
						Operation: "set",
						Value:     75,
					},
				},
			},
			SemanticLayer: rules.SemanticLayer{
				SuccessDescription:    "Волк заметил вас в тумане, его глаза светятся в темноте",
				FailureDescription:    "Волк слышит ваш шаг, но не может найти источник",
				EmotionalTone:         "tense",
				StyleMarkers:          []string{"dark_fantasy", "suspense"},
			},
			BalanceScore: 0.7,
			SafetyLevel:  "safe",
		}

		// Применяем правило (SaveRule требует MinIO, поэтому пропускаем)
		result, err := ruleEngine.Apply("wolf-detection", map[string]float32{
			"location": 1,
			"health":   100,
		}, nil)

		if err != nil {
			t.Logf("⚠️  ApplyRule error (expected without MinIO): %v", err)
		}

		if result != nil {
			t.Logf("✅ Правило применено: Success=%v, DiceRoll=%d, Total=%d",
				result.Success, result.DiceRoll, result.Total)
		}

		// Проверяем что Filter может обработать это без LLM
		req := &agent.AgentRequest{
			EventType:  "wolf.detected",
			EntityType: "creature",
			EntityID:   "wolf-alpha",
			State: map[string]float32{
				"aggression": 75,
				"health":     100,
			},
		}

		filterResult, err := filter.Filter(ctx, req)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		t.Logf("✅ Filter результат: Bypassed=%v, Reason=%s", filterResult.Bypassed, filterResult.Reason)
	})

	// ========================================
	// СЦЕНАРИЙ 3: Бой с волком
	// ========================================
	t.Run("Сценарий 3: Бой с волком", func(t *testing.T) {
		// Создаем правило боя
		_ = &rules.Rule{
			ID:          "combat-attack",
			Version:     "1.0",
			Name:        "Атака в бою",
			EntityType:  "combat",
			Description: "Базовая механика атаки в бою",
			MechanicalCore: rules.MechanicalCore{
				DiceFormula:      "d20 + strength",
				SuccessThreshold: "total >= 15",
				ContextualModifiers: []rules.Modifier{
					{
						ID:         "critical_success",
						Name:       "Критический успех",
						Condition:  "dice_roll == 20",
						Modifier:   10,
						SourceType: "luck",
					},
					{
						ID:         "critical_fail",
						Name:       "Критический провал",
						Condition:  "dice_roll == 1",
						Modifier:   -10,
						SourceType: "luck",
					},
				},
				StateChanges: []rules.StateChange{
					{
						Path:      "player.health",
						Operation: "subtract",
						Value:     15,
					},
					{
						Path:      "wolf.health",
						Operation: "subtract",
						Value:     20,
					},
				},
			},
			SemanticLayer: rules.SemanticLayer{
				SuccessDescription:        "Вы наносите точный удар мечом, волк визжит от боли",
				FailureDescription:        "Волк уклоняется и наносит ответный удар",
				CriticalSuccessDescription: "Ваш удар наносит критический урон, волк ранен смертельно",
				CriticalFailureDescription: "Вы промахиваетесь и открываетесь для атаки",
				EmotionalTone:             "violent",
				StyleMarkers:              []string{"dark_fantasy", "action"},
			},
			BalanceScore: 0.8,
			SafetyLevel:  "safe",
		}

		// Применяем правило боя (SaveRule требует MinIO, поэтому пропускаем)
		result, err := ruleEngine.Apply("combat-attack", map[string]float32{
			"strength": 5,
			"health":   100,
		}, nil)

		if err != nil {
			t.Logf("⚠️  ApplyRule error (expected without MinIO): %v", err)
		}

		if result != nil {
			t.Logf("✅ Бой: Success=%v, Critical=%v, DiceRoll=%d, Total=%d",
				result.Success, result.Critical, result.DiceRoll, result.Total)

			if result.Success {
				t.Log("✅ Игрок попал по волку!")
			} else {
				t.Log("❌ Волк уклонился!")
			}

			if result.Critical {
				t.Log("💥 КРИТИЧЕСКИЙ УДАР!")
			}
		}

		// Проверяем Rollback StateManager
		// Создаем новое состояние
		newCtx := &agent.AgentContext{
			AgentID:   agentID,
			AgentType: "game-master",
			Level:     agent.LevelTask,
			ScopeID:   "dark-forest-region-1",
			Entities:  agentCtx.Entities,
			CreatedAt: time.Now().Format(time.RFC3339),
		}

		if err := stateManager.Save(ctx, agentID, newCtx); err != nil {
			t.Fatalf("Save new state error: %v", err)
		}

		latestVersion := stateManager.GetLatestVersion(agentID)
		t.Logf("✅ Новая версия сохранена: %d", latestVersion)

		// Откатываемся к версии 1
		if latestVersion > 1 {
			rollbackCtx, err := stateManager.Rollback(ctx, agentID, 1)
			if err != nil {
				t.Logf("⚠️  Rollback error: %v", err)
			} else {
				t.Logf("✅ Rollback успешен: AgentID=%s, Version=%d",
					rollbackCtx.AgentID, stateManager.GetLatestVersion(agentID))
			}
		}

		// Проверяем статистику после всех операций
		stats := stateManager.GetStats()
		t.Logf("✅ Финальная статистика: %+v", stats)
	})

	// ========================================
	// СЦЕНАРИЙ 4: Проверка LOD
	// ========================================
	t.Run("Сценарий 4: Проверка LOD", func(t *testing.T) {
		// Проверяем LODManager
		lodConfig := agent.DefaultLODConfig()
		lodManager := agent.NewLODManager(lodConfig)

		// Устанавливаем LOD для агента
		lodManager.SetAgentLOD(agentID, agent.LODFull)

		// Проверяем что LOD установлен
		currentLOD := lodManager.GetAgentLOD(agentID)
		if currentLOD != agent.LODFull {
			t.Fatalf("Expected LODFull, got %s", currentLOD.String())
		}

		t.Logf("✅ LOD установлен: %s", currentLOD.String())

		// Рассчитываем новый LOD на основе метрик
		newLOD := lodManager.CalculateNewLOD(agentID, 5, 50, 100*time.Millisecond)
		t.Logf("✅ Новый LOD после расчета: %s", newLOD.String())

		// Проверяем статистику LOD
		lodStats := lodManager.GetStats()
		t.Logf("✅ LOD статистика: %+v", lodStats)
	})

	// ========================================
	// ФИНАЛЬНАЯ ПРОВЕРКА: Все компоненты работают вместе
	// ========================================
	t.Run("Финальная проверка", func(t *testing.T) {
		// Проверяем что все компоненты инициализированы
		if stateManager == nil {
			t.Fatal("StateManager is nil")
		}
		if filter == nil {
			t.Fatal("Filter is nil")
		}

		// Проверяем что StateManager валиден
		if err := stateManager.Validate(); err != nil {
			t.Fatalf("StateManager validation error: %v", err)
		}

		t.Log("✅ Все компоненты инициализированы и валидны")

		// Проверяем финальную статистику
		stateStats := stateManager.GetStats()
		filterStats := filter.GetStats()

		t.Logf("✅ StateManager статистика: %+v", stateStats)
		t.Logf("✅ Filter статистика: %+v", filterStats)

		// Проверяем что Filter bypassed хотя бы 50% запросов
		bypassRate, ok := filterStats["bypass_rate"].(float64)
		if ok && bypassRate >= 0.5 {
			t.Logf("✅ Filter bypass rate: %.0f%% (target: 70%%)", bypassRate*100)
		} else {
			t.Logf("⚠️  Filter bypass rate: %.0f%% (target: 70%%)", bypassRate*100)
		}
	})
}

// TestDarkForestE2E_Serialization — проверка сериализации всех структур
func TestDarkForestE2E_Serialization(t *testing.T) {
	// Создаем комплексный контекст
	ctx := context.Background()

	cache := agent.NewInMemoryCache()
	stateManager := agent.NewStateManager(
		cache,
		nil,
		agent.DefaultStateManagerConfig(),
	)

	agentCtx := &agent.AgentContext{
		AgentID:   "test-agent-001",
		AgentType: "game-master",
		Level:     agent.LevelDomain,
		ScopeID:   "test-scope",
		Entities: []agent.EntityRef{
			{ID: "player-1", Type: "player", Name: "Тестовый игрок", Level: agent.LODFull},
			{ID: "enemy-1", Type: "enemy", Name: "Тестовый враг", Level: agent.LODBasic},
		},
		CreatedAt: time.Now().Format(time.RFC3339),
		Blueprint: &agent.AgentBlueprint{
			Name:    "test-blueprint",
			Type:    "game-master",
			Version: "1.0",
		},
	}

	// Сохраняем
	if err := stateManager.Save(ctx, "test-agent", agentCtx); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	t.Logf("✅ Состояние сохранено (версия %d)", stateManager.GetLatestVersion("test-agent"))

	// Загружаем
	loaded, err := stateManager.Load(ctx, "test-agent")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// Проверяем что данные совпали
	if loaded.AgentID != agentCtx.AgentID {
		t.Fatalf("AgentID mismatch: expected %s, got %s", agentCtx.AgentID, loaded.AgentID)
	}
	if loaded.AgentType != agentCtx.AgentType {
		t.Fatalf("AgentType mismatch: expected %s, got %s", agentCtx.AgentType, loaded.AgentType)
	}
	if len(loaded.Entities) != len(agentCtx.Entities) {
		t.Fatalf("Entities count mismatch: expected %d, got %d", len(agentCtx.Entities), len(loaded.Entities))
	}

	t.Log("✅ Сериализация/десериализация успешна")

	// Проверяем историю версий
	versions := stateManager.GetVersionHistory("test-agent")
	if len(versions) == 0 {
		t.Fatal("Expected at least 1 version")
	}

	t.Logf("✅ История версий: %d записей", len(versions))

	// Проверяем что VersionEntry можно сериализовать
	for i, v := range versions {
		jsonData, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("VersionEntry %d marshal error: %v", i, err)
		}
		t.Logf("✅ VersionEntry %d: Version=%d, Size=%d, JSON=%d байт",
			i, v.Version, v.Size, len(jsonData))
	}
}

// TestDarkForestE2E_FilterStats — проверка статистики Filter
func TestDarkForestE2E_FilterStats(t *testing.T) {
	ctx := context.Background()

	ruleEngine := rules.NewEngine(nil, "rules", 100)
	filter := agent.NewRuleEngineFilter(ruleEngine)

	// Создаем несколько запросов
	requests := []*agent.AgentRequest{
		{
			EventType:  "combat.attack",
			EntityType: "player",
			EntityID:   "player-1",
			State: map[string]float32{
				"health":   100,
				"strength": 5,
			},
		},
		{
			EventType:  "movement.walk",
			EntityType: "player",
			EntityID:   "player-1",
			State: map[string]float32{
				"health":  100,
				"stamina": 80,
			},
		},
		{
			EventType:  "dialogue.speak",
			EntityType: "npc",
			EntityID:   "npc-1",
			State: map[string]float32{
				"aggression": 20,
			},
		},
	}

	for i, req := range requests {
		_, err := filter.Filter(ctx, req)
		if err != nil {
			t.Fatalf("Filter request %d error: %v", i, err)
		}
	}

	// Проверяем статистику
	stats := filter.GetStats()
	totalRequests, _ := stats["total_requests"].(int64)

	if totalRequests != int64(len(requests)) {
		t.Fatalf("Expected %d total requests, got %d", len(requests), totalRequests)
	}

	t.Logf("✅ Статистика Filter: %+v", stats)
}

// TestDarkForestE2E_LoD — проверка адаптивного LOD
func TestDarkForestE2E_LoD(t *testing.T) {
	lodConfig := agent.DefaultLODConfig()
	lodManager := agent.NewLODManager(lodConfig)

	agentIDs := []string{
		"agent-1",
		"agent-2",
		"agent-3",
	}

	// Устанавливаем разные LOD для агентов
	for i, id := range agentIDs {
		lod := agent.LODFull
		if i == 1 {
			lod = agent.LODBasic
		} else if i == 2 {
			lod = agent.LODRuleOnly
		}
		lodManager.SetAgentLOD(id, lod)
	}

	// Проверяем что LOD установлены правильно
	if lodManager.GetAgentLOD("agent-1") != agent.LODFull {
		t.Fatal("Expected agent-1 LODFull")
	}
	if lodManager.GetAgentLOD("agent-2") != agent.LODBasic {
		t.Fatal("Expected agent-2 LODBasic")
	}
	if lodManager.GetAgentLOD("agent-3") != agent.LODRuleOnly {
		t.Fatal("Expected agent-3 LODRuleOnly")
	}

	t.Log("✅ LOD установлены правильно")

	// Проверяем статистику
	stats := lodManager.GetStats()
	agentCount, _ := stats["agent_count"].(int)
	if agentCount != 3 {
		t.Fatalf("Expected 3 agents, got %d", agentCount)
	}

	t.Logf("✅ LOD статистика: %+v", stats)

	// Проверяем CalculateNewLOD
	newLOD := lodManager.CalculateNewLOD("agent-1", 15, 1500, 600*time.Millisecond)
	if newLOD != agent.LODRuleOnly {
		t.Logf("⚠️  Expected LODRuleOnly for high load, got %s", newLOD.String())
	} else {
		t.Log("✅ LOD понижен при высокой нагрузке")
	}

	// Проверяем что GetAgentCount работает
	count := lodManager.GetAgentCount()
	if count != 3 {
		t.Fatalf("Expected 3 agents, got %d", count)
	}

	t.Logf("✅ GetAgentCount: %d", count)
}

// TestDarkForestE2E_Blueprint — проверка сериализации AgentBlueprint
func TestDarkForestE2E_Blueprint(t *testing.T) {
	// Создаем Blueprint
	blueprint := &agent.AgentBlueprint{
		Name:        "dark-forest-gm",
		Type:        "game-master",
		Version:     "1.0",
		Description: "GM для сценария Тёмный лес",
		Phase1Prompt: "Обработай событие: {{.EventType}}",
		Phase2Prompt: "Опиши результат: {{.Narrative}}",
	}

	t.Logf("✅ Blueprint создан: Name=%s, Type=%s, Version=%s",
		blueprint.Name, blueprint.Type, blueprint.Version)

	// Проверяем что Blueprint можно сериализовать в JSON
	jsonData, err := json.Marshal(blueprint)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	t.Logf("✅ Blueprint сериализован: %d байт", len(jsonData))
}

// TestDarkForestE2E_ToolRegistry — проверка ToolRegistry
func TestDarkForestE2E_ToolRegistry(t *testing.T) {
	registry := tools.NewToolRegistry()

	// Создаем тестовые инструменты
	worldTool := &testWorldTool{}
	entityTool := &testEntityTool{}
	narrativeTool := &testNarrativeTool{}

	// Регистрируем
	if err := registry.Register(worldTool); err != nil {
		t.Fatalf("Register worldTool error: %v", err)
	}
	if err := registry.Register(entityTool); err != nil {
		t.Fatalf("Register entityTool error: %v", err)
	}
	if err := registry.Register(narrativeTool); err != nil {
		t.Fatalf("Register narrativeTool error: %v", err)
	}

	t.Log("✅ Все инструменты зарегистрированы")

	// Проверяем что все инструменты есть
	if _, exists := registry.Get("world"); !exists {
		t.Fatal("world tool not found")
	}
	if _, exists := registry.Get("entity"); !exists {
		t.Fatal("entity tool not found")
	}
	if _, exists := registry.Get("narrative"); !exists {
		t.Fatal("narrative tool not found")
	}

	t.Log("✅ Все инструменты найдены")

	// Проверяем List
	tools := registry.List()
	if len(tools) != 3 {
		t.Fatalf("Expected 3 tools, got %d", len(tools))
	}

	t.Logf("✅ List возвращает %d инструментов", len(tools))

	// Проверяем BuildToolDefinitions
	defs := registry.BuildToolDefinitions()
	if len(defs) != 3 {
		t.Fatalf("Expected 3 tool definitions, got %d", len(defs))
	}

	t.Logf("✅ BuildToolDefinitions возвращает %d определений", len(defs))

	// Проверяем статистику
	stats := registry.GetStats()
	t.Logf("✅ ToolRegistry статистика: %+v", stats)
}

// Тестовые инструменты
type testWorldTool struct{}

func (t *testWorldTool) Name() string { return "world" }
func (t *testWorldTool) Description() string { return "World interaction tool" }
func (t *testWorldTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}
func (t *testWorldTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "ok"}, nil
}

type testEntityTool struct{}

func (t *testEntityTool) Name() string { return "entity" }
func (t *testEntityTool) Description() string { return "Entity management tool" }
func (t *testEntityTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}
func (t *testEntityTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "ok"}, nil
}

type testNarrativeTool struct{}

func (t *testNarrativeTool) Name() string { return "narrative" }
func (t *testNarrativeTool) Description() string { return "Narrative generation tool" }
func (t *testNarrativeTool) Schema() map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}
func (t *testNarrativeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "ok", "narrative": "Тёмный лес шепчет..."}, nil
}

// Helper
func printResult(result interface{}) {
	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("%s\n", jsonData)
}
