// agentruntime/action/phase2.go
package action

import (
	"context"
	"fmt"
	"strings"

	"multiverse-core.io/services/agent-runtime/agentruntime/cache"
	"multiverse-core.io/shared/oracle"
)

// Phase2Caller вызывает Narrative LLM ("Сказитель").
// Использует SemanticLayer из правила и MechanicalResult от Phase1.
type Phase2Caller struct {
	oracle       *oracle.Client
	promptCache  *cache.PromptCache
	narrativeCache *cache.NarrativeCache
}

// NewPhase2Caller создаёт Phase2Caller
func NewPhase2Caller(oracleClient *oracle.Client, promptCache *cache.PromptCache, narrativeCache *cache.NarrativeCache) *Phase2Caller {
	return &Phase2Caller{
		oracle:         oracleClient,
		promptCache:    promptCache,
		narrativeCache: narrativeCache,
	}
}

// Generate создаёт нарративное описание действия на основе MechanicalResult.
// Асинхронный путь — вызывается в горутине из Resolver.
func (p *Phase2Caller) Generate(ctx context.Context, m *MechanicalResult) (string, error) {
	// L3 кэш: похожие ситуации → переиспользовать нарратив (5 мин TTL)
	cacheKey := buildNarrativeCacheKey(m)
	if cached := p.narrativeCache.Get(cacheKey); cached != "" {
		return cached, nil
	}

	// System prompt кэшируется по стилю из SemanticLayer
	styleKey := m.SemanticHints.EmotionalTone + ":" + strings.Join(m.SemanticHints.StyleMarkers, ",")
	systemPrompt := p.promptCache.GetOrBuild("narrative:"+styleKey, func() string {
		return buildPhase2SystemPrompt(m)
	})

	userPrompt := buildPhase2UserPrompt(m)

	text, err := p.oracle.CallNarrative(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("phase2 oracle call: %w", err)
	}

	text = strings.TrimSpace(text)
	p.narrativeCache.Set(cacheKey, text)

	return text, nil
}

// buildPhase2SystemPrompt формирует системный промпт на основе SemanticLayer правила.
// Одинаков для всех вызовов с одинаковым стилем → Ollama KV cache reuse.
func buildPhase2SystemPrompt(m *MechanicalResult) string {
	sl := m.SemanticHints

	style := sl.EmotionalTone
	if style == "" {
		style = "neutral"
	}

	markers := strings.Join(sl.StyleMarkers, ", ")
	if markers == "" {
		markers = "realistic"
	}

	blocked := ""
	if len(sl.BlockedTopics) > 0 {
		blocked = fmt.Sprintf("\nЗапрещённые темы: %s.", strings.Join(sl.BlockedTopics, ", "))
	}

	return fmt.Sprintf(`Ты — нарратор боевой сцены в стиле %s (%s).
Пишешь 2-3 предложения описания действия. Конкретно, живо, без пафоса.
Упомяни куда попал удар, как выглядела реакция цели.%s
Отвечай только текстом описания, без вводных слов.`, style, markers, blocked)
}

// buildPhase2UserPrompt формирует пользовательский промпт из MechanicalResult и SemanticLayer
func buildPhase2UserPrompt(m *MechanicalResult) string {
	sl := m.SemanticHints

	// Выбираем шаблон в зависимости от outcome_tag
	var template string
	switch m.OutcomeTag {
	case OutcomeKill:
		template = sl.CriticalSuccessDescription
		if template == "" {
			template = "Цель уничтожена. Опиши финальный удар эпично и кратко."
		}
	case OutcomeWound, OutcomeGraze:
		template = sl.SuccessDescription
		if template == "" {
			template = "Атака попала. Опиши удар и реакцию цели."
		}
	case OutcomeMiss:
		template = sl.FailureDescription
		if template == "" {
			template = "Атака промахнулась. Опиши как цель уклонилась."
		}
	case OutcomeReflected:
		template = "Атака отражена. Опиши как и что произошло с отражённым зарядом."
	default:
		template = sl.PoeticDescription
		if template == "" {
			template = "Опиши что произошло."
		}
	}

	hpStatus := ""
	if m.TargetHPAfter > 0 {
		hpStatus = fmt.Sprintf("Цель ещё жива (осталось HP: %.0f). ", m.TargetHPAfter)
	} else {
		hpStatus = "Цель погибла. "
	}

	statusStr := ""
	if len(m.StatusEffects) > 0 {
		statusStr = fmt.Sprintf("Статус-эффекты: %s. ", strings.Join(m.StatusEffects, ", "))
	}

	critStr := ""
	if m.Critical {
		critStr = "КРИТИЧЕСКИЙ УДАР. "
	}

	return fmt.Sprintf(`%s%sУрон: %d. %s%s
Шаблон описания: %s`,
		critStr, hpStatus, m.Damage, statusStr,
		actionDescription(m),
		template,
	)
}

// actionDescription строит краткое описание произошедшего
func actionDescription(m *MechanicalResult) string {
	switch m.OutcomeTag {
	case OutcomeKill:
		return fmt.Sprintf("Атакующий [%s] убил цель [%s].", m.AttackerID, m.TargetID)
	case OutcomeWound:
		return fmt.Sprintf("Атакующий [%s] ранил цель [%s] на %d HP.", m.AttackerID, m.TargetID, m.Damage)
	case OutcomeGraze:
		return fmt.Sprintf("Атакующий [%s] задел цель [%s] на %d HP (касательный).", m.AttackerID, m.TargetID, m.Damage)
	case OutcomeMiss:
		return fmt.Sprintf("Атакующий [%s] промахнулся мимо [%s].", m.AttackerID, m.TargetID)
	case OutcomeReflected:
		return fmt.Sprintf("Атака [%s] отразилась от [%s].", m.AttackerID, m.TargetID)
	default:
		return fmt.Sprintf("Действие [%s] → [%s]: %s.", m.AttackerID, m.TargetID, m.OutcomeTag)
	}
}

// buildNarrativeCacheKey строит ключ для L3 нарративного кэша
func buildNarrativeCacheKey(m *MechanicalResult) string {
	style := m.SemanticHints.EmotionalTone
	return fmt.Sprintf("narrative:%s:%s:%s", m.RuleID, m.OutcomeTag, style)
}
