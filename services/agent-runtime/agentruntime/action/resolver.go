// agentruntime/action/resolver.go
package action

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/rules"
)

// ActionRequest описывает входные данные для разрешения действия
type ActionRequest struct {
	RuleID     string            // ID правила (например "sword_strike")
	AttackerID string            // ID атакующей сущности
	TargetID   string            // ID цели
	WorldID    string            // ID мира (для маршрутизации событий)
	ScopeID    string            // ID области (для маршрутизации)
	TriggerID  string            // ID события-триггера (для lineage)

	// Статы подаются снаружи — Resolver не делает I/O за сущностями.
	// Это обязанность вызывающего (ZoneAgent собирает контекст до spawn).
	AttackerStats map[string]float32
	TargetStats   map[string]float32
}

// Resolver выполняет детерминированную подготовку к LLM-вызовам.
// Не содержит if-else игровой логики — все решения передаются Phase1 LLM.
type Resolver struct {
	engine      *rules.Engine
	phase1      *Phase1Caller
	phase2      *Phase2Caller
	bus         *eventbus.EventBus
}

// NewResolver создаёт Resolver с готовыми зависимостями
func NewResolver(engine *rules.Engine, phase1 *Phase1Caller, phase2 *Phase2Caller, bus *eventbus.EventBus) *Resolver {
	return &Resolver{
		engine: engine,
		phase1: phase1,
		phase2: phase2,
		bus:    bus,
	}
}

// Resolve обрабатывает ActionRequest:
//  1. Применяет rule engine (броски кубиков, модификаторы) — детерминированно
//  2. Отправляет результат в Phase1 LLM → MechanicalResult
//  3. Эмитит action.resolved (sync)
//  4. Запускает Phase2 LLM в горутине → эмитит narrative.action (async)
func (r *Resolver) Resolve(ctx context.Context, req ActionRequest) error {
	// Шаг 1: детерминированная механика (броски, модификаторы, порог успеха)
	ruleResult, err := r.engine.Apply(req.RuleID, req.AttackerStats, nil)
	if err != nil {
		return fmt.Errorf("rule engine apply: %w", err)
	}

	// Получаем полное правило для SemanticLayer
	rule, err := r.engine.GetRule(req.RuleID)
	if err != nil {
		return fmt.Errorf("get rule for semantic layer: %w", err)
	}

	// Шаг 2: Phase1 LLM — интерпретирует числа и принимает механическое решение
	mechanical, err := r.phase1.Decide(ctx, Phase1Input{
		RuleID:        req.RuleID,
		DiceFormula:   ruleResult.DiceFormula,
		DiceRoll:      ruleResult.DiceRoll,
		Total:         ruleResult.Total,
		AttackerStats: req.AttackerStats,
		TargetStats:   req.TargetStats,
		RuleName:      rule.Name,
		SemanticLayer: rule.SemanticLayer,
	})
	if err != nil {
		return fmt.Errorf("phase1 decide: %w", err)
	}
	mechanical.AttackerID = req.AttackerID
	mechanical.TargetID = req.TargetID
	mechanical.RuleID = req.RuleID
	mechanical.RuleVersion = rule.Version
	mechanical.DiceFormula = ruleResult.DiceFormula
	mechanical.DiceRoll = ruleResult.DiceRoll
	mechanical.Total = ruleResult.Total
	mechanical.SemanticHints = rule.SemanticLayer

	// Шаг 3: эмитить action.resolved (sync — EntityManager применяет state_changes)
	if err := r.emitActionResolved(ctx, req, mechanical); err != nil {
		log.Printf("[resolver] failed to emit action.resolved: %v", err)
	}

	// Шаг 4: Phase2 нарратив — асинхронно, не блокирует игровой цикл
	go func() {
		narrativeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		text, err := r.phase2.Generate(narrativeCtx, mechanical)
		if err != nil {
			log.Printf("[resolver] phase2 narrative failed: %v", err)
			return
		}

		if err := r.emitNarrativeAction(narrativeCtx, req, mechanical, text); err != nil {
			log.Printf("[resolver] failed to emit narrative.action: %v", err)
		}
	}()

	return nil
}

// emitActionResolved публикует механический результат в world_events
func (r *Resolver) emitActionResolved(ctx context.Context, req ActionRequest, m *MechanicalResult) error {
	payload := eventbus.NewEventPayload().
		WithEntity(req.AttackerID, "player", "").
		WithWorld(req.WorldID)

	// Кладём механический результат как custom данные
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	var resultMap map[string]any
	if err := json.Unmarshal(data, &resultMap); err != nil {
		return err
	}
	eventbus.SetNested(payload.GetCustom(), "mechanical_result", resultMap)
	eventbus.SetNested(payload.GetCustom(), "target.id", req.TargetID)
	eventbus.SetNested(payload.GetCustom(), "rule.id", req.RuleID)

	event := eventbus.NewStructuredEvent("action.resolved", "agent-runtime", req.WorldID, payload)
	return r.bus.Publish(ctx, eventbus.TopicWorldEvents, event)
}

// emitNarrativeAction публикует нарративный текст для WebSocket/SSE доставки
func (r *Resolver) emitNarrativeAction(ctx context.Context, req ActionRequest, m *MechanicalResult, text string) error {
	payload := eventbus.NewEventPayload().
		WithEntity(req.AttackerID, "player", "").
		WithWorld(req.WorldID)

	eventbus.SetNested(payload.GetCustom(), "narrative.text", text)
	eventbus.SetNested(payload.GetCustom(), "narrative.outcome_tag", m.OutcomeTag)
	eventbus.SetNested(payload.GetCustom(), "target.id", req.TargetID)
	eventbus.SetNested(payload.GetCustom(), "attacker.id", req.AttackerID)

	event := eventbus.NewStructuredEvent("narrative.action", "agent-runtime", req.WorldID, payload)
	return r.bus.Publish(ctx, eventbus.TopicNarrativeOutput, event)
}
