// agentruntime/action/phase1.go
package action

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"multiverse-core.io/services/agent-runtime/agentruntime/cache"
	"multiverse-core.io/shared/oracle"
	"multiverse-core.io/shared/rules"
)

// Phase1Input — всё что нужно для формирования запроса к "Судье"
type Phase1Input struct {
	RuleID        string
	RuleName      string
	DiceFormula   string
	DiceRoll      int
	Total         int
	AttackerStats map[string]float32
	TargetStats   map[string]float32
	SemanticLayer rules.SemanticLayer
}

// Phase1Caller вызывает Mechanical LLM ("Судья").
// Использует L1 кэш system prompt и L2 кэш результатов.
type Phase1Caller struct {
	oracle       *oracle.Client
	promptCache  *cache.PromptCache
	resultCache  *cache.ResultCache
}

// NewPhase1Caller создаёт Phase1Caller
func NewPhase1Caller(oracleClient *oracle.Client, promptCache *cache.PromptCache, resultCache *cache.ResultCache) *Phase1Caller {
	return &Phase1Caller{
		oracle:      oracleClient,
		promptCache: promptCache,
		resultCache: resultCache,
	}
}

// Decide принимает механическое решение через LLM.
// Возвращает MechanicalResult с hit/damage/outcome_tag и т.д.
func (p *Phase1Caller) Decide(ctx context.Context, input Phase1Input) (*MechanicalResult, error) {
	// L2 кэш: одинаковые входные данные → тот же результат (AoE, множественные цели)
	cacheKey := buildResultCacheKey(input)
	if raw := p.resultCache.Get(cacheKey); raw != nil {
		if cached, ok := raw.(*MechanicalResult); ok {
			return cached, nil
		}
	}

	systemPrompt := p.promptCache.GetOrBuild(input.RuleID, func() string {
		return buildPhase1SystemPrompt(input.RuleID, input.RuleName)
	})

	userPrompt := buildPhase1UserPrompt(input)

	raw, err := p.oracle.CallMechanical(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("phase1 oracle call: %w", err)
	}

	result, err := parsePhase1Response(raw)
	if err != nil {
		return nil, fmt.Errorf("phase1 parse response: %w", err)
	}

	// Сохранить в L2 кэш (30 сек — актуально для AoE/raid)
	p.resultCache.Set(cacheKey, result)

	return result, nil
}

// buildPhase1SystemPrompt строит системный промпт для "Судьи".
// Важно: он одинаков для всех вызовов одного правила → Ollama KV reuse.
func buildPhase1SystemPrompt(ruleID, ruleName string) string {
	return fmt.Sprintf(`Ты — механический судья боевой системы (правило: %s, id: %s).
Получаешь: бросок кубиков, статы атакующего, статы цели.
Твоя задача: применить правило и вернуть ТОЛЬКО валидный JSON без пояснений.

Формат ответа:
{"hit":bool,"damage":int,"critical":bool,"status_effects":[string],"target_hp_after":float,"outcome_tag":string}

Значения outcome_tag: "kill" (цель умерла), "wound" (ранена), "graze" (царапина), "miss" (промах), "reflected" (отражено), "blocked" (заблокировано).
Если атака отражается (зеркало, отражающий щит и т.д.) — outcome_tag="reflected", damage=0, укажи reflect_target в status_effects.
При critical:true — удваивай damage.
ТОЛЬКО JSON, никаких пояснений.`, ruleName, ruleID)
}

// buildPhase1UserPrompt строит пользовательский промпт с переменными данными боя
func buildPhase1UserPrompt(input Phase1Input) string {
	attackerJSON, _ := json.Marshal(input.AttackerStats)
	targetJSON, _ := json.Marshal(input.TargetStats)

	return fmt.Sprintf(`Правило: %s (%s)
Бросок: %s → результат %d (итог с модификаторами: %d)
Атакующий: %s
Цель: %s`,
		input.RuleName, input.DiceFormula,
		input.DiceFormula, input.DiceRoll, input.Total,
		string(attackerJSON),
		string(targetJSON),
	)
}

// parsePhase1Response десериализует ответ Phase1 LLM в MechanicalResult.
// Очищает возможный markdown ```json``` если модель добавила обёртку.
func parsePhase1Response(raw string) (*MechanicalResult, error) {
	raw = strings.TrimSpace(raw)
	// Убираем markdown code block если есть
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result MechanicalResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON from Phase1 LLM (%q): %w", raw, err)
	}

	// Базовая валидация: outcome_tag обязателен
	if result.OutcomeTag == "" {
		// Вывести из hit/TargetHPAfter как запасной вариант
		if !result.Hit {
			result.OutcomeTag = OutcomeMiss
		} else if result.TargetHPAfter <= 0 {
			result.OutcomeTag = OutcomeKill
		} else {
			result.OutcomeTag = OutcomeWound
		}
	}

	return &result, nil
}

// buildResultCacheKey строит ключ для L2 кэша (SHA-like из входных данных)
func buildResultCacheKey(input Phase1Input) string {
	attackerJSON, _ := json.Marshal(input.AttackerStats)
	targetJSON, _ := json.Marshal(input.TargetStats)
	return fmt.Sprintf("%s:%d:%d:%s:%s",
		input.RuleID, input.DiceRoll, input.Total,
		string(attackerJSON), string(targetJSON))
}
