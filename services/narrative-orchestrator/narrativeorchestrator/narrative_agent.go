// services/narrativeorchestrator/narrative_agent.go

package narrativeorchestrator

import (
	"context"
	"fmt"
	"log"
	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/agent/tools"
	"sync"
	"time"
)

// NarrativeAgent — агент который использует TwoPhasePipeline для обработки событий.
// Это мост между текущей GMInstance архитектурой и новой Agent GM Core.
type NarrativeAgent struct {
	// ID уникальный идентификатор агента
	id string

	// type тип агента
	typeName string

	// level иерархический уровень
	level agent.AgentLevel

	// state состояние жизненного цикла
	state agent.AgentLifecycleState

	// context контекст агента
	context *agent.AgentContext

	// pipeline двухфазный конвейер
	pipeline *agent.TwoPhasePipeline

	// bus шина событий для публикации
	bus *eventbus.EventBus

	// worldID ID мира
	worldID string

	// scopeID ID области видимости
	scopeID string

	// scopeType Type области видимости
	scopeType string

	// focusEntities сущности на которые фокусируется агент
	focusEntities []string

	// history история событий
	history []HistoryEntry

	// state данные состояния
	stateData map[string]interface{}

	// config конфигурация
	config map[string]interface{}

	// logger логгер
	logger *log.Logger

	// mu mutex для потокобезопасности
	mu sync.RWMutex

	// lastProcessTime время последней обработки
	lastProcessTime int64

	// emittedEvents отслеживание опубликованных событий
	emittedEvents map[string]bool

	// toolsRegistry реестр инструментов агента
	toolsRegistry *tools.ToolRegistry

	// worldTool инструмент для взаимодействия с миром
	worldTool *tools.WorldTool

	// entityTool инструмент для работы с сущностями
	entityTool *tools.EntityTool

	// narrativeTool инструмент для нарративов
	narrativeTool *tools.NarrativeTool

	// stateManager менеджер состояния с версионированием
	stateManager *agent.StateManager

	// stateKey ключ для хранения состояния
	stateKey string
}

// NewNarrativeAgent создает нового NarrativeAgent
func NewNarrativeAgent(
	id string,
	typeName string,
	level agent.AgentLevel,
	scopeID string,
	scopeType string,
	worldID string,
	focusEntities []string,
	config map[string]interface{},
	pipeline *agent.TwoPhasePipeline,
	bus *eventbus.EventBus,
	logger *log.Logger,
	stateManager *agent.StateManager,
) *NarrativeAgent {
	a := &NarrativeAgent{
		id:            id,
		typeName:      typeName,
		level:         level,
		state:         agent.LifecycleRunning,
		context: &agent.AgentContext{
			AgentID:   id,
			AgentType: typeName,
			Level:     level,
			ScopeID:   scopeID,
			Entities:  toEntityRefs(focusEntities),
			CreatedAt: time.Now().Format(time.RFC3339),
		},
		pipeline:        pipeline,
		bus:             bus,
		worldID:         worldID,
		scopeID:         scopeID,
		scopeType:       scopeType,
		focusEntities:   focusEntities,
		history:         []HistoryEntry{},
		stateData:       make(map[string]interface{}),
		config:          config,
		logger:          logger,
		mu:              sync.RWMutex{},
		lastProcessTime: 0,
		emittedEvents:   make(map[string]bool),
		toolsRegistry:   tools.NewToolRegistry(),
		worldTool:       tools.NewWorldTool(worldID, scopeID, bus),
		entityTool:      tools.NewEntityTool(worldID, scopeID),
		narrativeTool:   tools.NewNarrativeTool(worldID, scopeID, bus),
		stateManager:    stateManager,
		stateKey:        fmt.Sprintf("narrative-agent:%s:%s", worldID, id),
	}

	// Регистрируем инструменты в реестре
	a.toolsRegistry.Register(a.worldTool)
	a.toolsRegistry.Register(a.entityTool)
	a.toolsRegistry.Register(a.narrativeTool)

	return a
}

// toEntityRefs конвертирует []string в []agent.EntityRef
func toEntityRefs(ids []string) []agent.EntityRef {
	entities := make([]agent.EntityRef, 0, len(ids))
	for _, id := range ids {
		entities = append(entities, agent.EntityRef{
			ID:    id,
			Type:  extractEntityType(id),
			Name:  extractEntityName(id),
			Level: agent.LODBasic,
		})
	}
	return entities
}

// extractEntityType определяет тип сущности по ID
func extractEntityType(id string) string {
	if len(id) == 0 {
		return "unknown"
	}
	// Простая эвристика: если есть :, то часть после : — это тип
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == ':' {
			return id[i+1:]
		}
	}
	return "entity"
}

// extractEntityName извлекает имя сущности
func extractEntityName(id string) string {
	if len(id) == 0 {
		return "unknown"
	}
	// Простая эвристика: если есть :, то часть до : — это имя
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == ':' {
			return id[:i]
		}
	}
	return id
}

// ID возвращает уникальный идентификатор
func (a *NarrativeAgent) ID() string {
	return a.id
}

// Type возвращает тип агента
func (a *NarrativeAgent) Type() string {
	return a.typeName
}

// Level возвращает иерархический уровень
func (a *NarrativeAgent) Level() agent.AgentLevel {
	return a.level
}

// State возвращает текущее состояние жизненного цикла
func (a *NarrativeAgent) State() agent.AgentLifecycleState {
	return a.state
}

// Context возвращает контекст агента
func (a *NarrativeAgent) Context() *agent.AgentContext {
	return a.context
}

// Tick выполняет один такт агента с входящим событием
func (a *NarrativeAgent) Tick(ctx context.Context, event agent.Event) (agent.Action, error) {
	// Обрабатываем событие
	if err := a.HandleEvent(ctx, event); err != nil {
		return agent.Action{}, fmt.Errorf("tick: %w", err)
	}

	return agent.Action{
		Type:     "processed",
		Target:   a.scopeID,
		Priority: 1,
		Async:    true,
	}, nil
}

// HandleEvent обрабатывает событие через TwoPhasePipeline
func (a *NarrativeAgent) HandleEvent(ctx context.Context, event agent.Event) error {
	a.logger.Printf("[NarrativeAgent %s] Handling event %s (%s)", a.id, event.Type, event.ID)

	// Вызываем TwoPhasePipeline
	result, err := a.pipeline.Process(ctx, event, a)
	if err != nil {
		return fmt.Errorf("pipeline process: %w", err)
	}

	// Обрабатываем результат фазы 1 (Decision)
	if result.Decision != nil {
		a.logger.Printf("[NarrativeAgent %s] Decision: %d decisions, next phase: %s",
			a.id, len(result.Decision.Decisions), result.Decision.NextPhase)

		// Публикуем механические решения как события
		for _, decision := range result.Decision.Decisions {
			decisionEvent := a.buildDecisionEvent(decision)
			if err := a.bus.Publish(ctx, eventbus.TopicWorldEvents, decisionEvent); err != nil {
				a.logger.Printf("[NarrativeAgent %s] Failed to publish decision event: %v", a.id, err)
			}
		}
	}

	// Обрабатываем результат фазы 2 (Narrative)
	if result.Narrative != nil && result.Narrative.Text != "" {
		narrativeEvent := a.buildNarrativeEvent(result.Narrative.Text, result.Narrative.Effects)
		if err := a.bus.Publish(ctx, eventbus.TopicNarrativeOutput, narrativeEvent); err != nil {
			a.logger.Printf("[NarrativeAgent %s] Failed to publish narrative event: %v", a.id, err)
		}
	}

	// Сохраняем историю
	a.history = append(a.history, HistoryEntry{
		EventID:   event.ID,
		EventType: event.Type,
		Timestamp: event.Timestamp,
	})

	return nil
}

// buildDecisionEvent создает eventbus.Event из решения
func (a *NarrativeAgent) buildDecisionEvent(decision agent.Decision) eventbus.Event {
	payload := map[string]interface{}{
		"decision_type": decision.Type,
		"target":        decision.Target,
		"pipeline":      "two-phase",
	}
	for k, v := range decision.Payload {
		payload[k] = v
	}

	return eventbus.NewEvent(
		fmt.Sprintf("decision.%s", decision.Type),
		"narrative-orchestrator",
		a.worldID,
		payload,
	)
}

// buildNarrativeEvent создает eventbus.Event из нарратива
func (a *NarrativeAgent) buildNarrativeEvent(text string, effects []agent.NarrativeEffect) eventbus.Event {
	payload := map[string]interface{}{
		"narrative": text,
		"pipeline":  "two-phase",
	}

	for _, effect := range effects {
		payload[effect.Type] = effect.Value
	}

	return eventbus.NewEvent(
		"narrative.generate",
		"narrative-orchestrator",
		a.worldID,
		payload,
	)
}

// Shutdown завершает работу агента
func (a *NarrativeAgent) Shutdown(ctx context.Context) error {
	a.state = agent.LifecycleFinishing
	a.logger.Printf("[NarrativeAgent %s] Shutting down", a.id)
	a.state = agent.LifecycleFinished
	return nil
}

// Pause ставит на паузу
func (a *NarrativeAgent) Pause(ctx context.Context) error {
	a.state = agent.LifecyclePaused
	a.logger.Printf("[NarrativeAgent %s] Paused", a.id)
	return nil
}

// Resume продолжает работу после паузы
func (a *NarrativeAgent) Resume(ctx context.Context) error {
	a.state = agent.LifecycleRunning
	a.logger.Printf("[NarrativeAgent %s] Resumed", a.id)
	return nil
}

// Memory возвращает доступ к памяти агента
func (a *NarrativeAgent) Memory() agent.MemoryStore {
	// TODO: реализовать интеграцию с Semantic Memory
	return nil
}

// Tools возвращает доступ к инструментам
func (a *NarrativeAgent) Tools() agent.ToolRegistry {
	return tools.GetAgentToolRegistry(a.toolsRegistry)
}

// GetHistory возвращает историю событий
func (a *NarrativeAgent) GetHistory() []HistoryEntry {
	return a.history
}

// GetState возвращает состояние агента
func (a *NarrativeAgent) GetState() map[string]interface{} {
	return a.stateData
}

// SetState устанавливает состояние агента
func (a *NarrativeAgent) SetState(key string, value interface{}) {
	a.stateData[key] = value
}

// GetConfig возвращает конфигурацию
func (a *NarrativeAgent) GetConfig() map[string]interface{} {
	return a.config
}

// GetScopeID возвращает ID области видимости
func (a *NarrativeAgent) GetScopeID() string {
	return a.scopeID
}

// GetWorldID возвращает ID мира
func (a *NarrativeAgent) GetWorldID() string {
	return a.worldID
}

// trackEmitted добавляет event ID в список опубликованных
func (a *NarrativeAgent) trackEmitted(eventID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.emittedEvents[eventID] = true
}

// isOwnEvent проверяет было ли событие уже опубликовано этим агентом
func (a *NarrativeAgent) isOwnEvent(eventID string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.emittedEvents[eventID]
}

// SaveState сохраняет состояние агента в StateManager
func (a *NarrativeAgent) SaveState(ctx context.Context) error {
	if a.stateManager == nil {
		return fmt.Errorf("state manager not initialized")
	}

	// Собираем контекст
	context := &agent.AgentContext{
		AgentID:   a.id,
		AgentType: a.typeName,
		Level:     a.level,
		ScopeID:   a.scopeID,
		Entities:  a.context.Entities,
		CreatedAt: a.context.CreatedAt,
	}

	return a.stateManager.Save(ctx, a.stateKey, context)
}

// LoadState загружает состояние агента из StateManager
func (a *NarrativeAgent) LoadState(ctx context.Context) error {
	if a.stateManager == nil {
		return fmt.Errorf("state manager not initialized")
	}

	context, err := a.stateManager.Load(ctx, a.stateKey)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	// Обновляем контекст агента
	a.context = context

	return nil
}

// RollbackState откатывает состояние агента к предыдущей версии
func (a *NarrativeAgent) RollbackState(ctx context.Context, version int) error {
	if a.stateManager == nil {
		return fmt.Errorf("state manager not initialized")
	}

	context, err := a.stateManager.Rollback(ctx, a.stateKey, version)
	if err != nil {
		return fmt.Errorf("rollback state: %w", err)
	}

	// Обновляем контекст агента
	a.context = context

	return nil
}

// GetVersionHistory возвращает историю версий состояния
func (a *NarrativeAgent) GetVersionHistory() []agent.VersionEntry {
	if a.stateManager == nil {
		return nil
	}

	return a.stateManager.GetVersionHistory(a.stateKey)
}

// GetLatestVersion возвращает номер последней версии
func (a *NarrativeAgent) GetLatestVersion() int {
	if a.stateManager == nil {
		return 0
	}

	return a.stateManager.GetLatestVersion(a.stateKey)
}

// GetStateStats возвращает статистику StateManager
func (a *NarrativeAgent) GetStateStats() map[string]interface{} {
	if a.stateManager == nil {
		return nil
	}

	return a.stateManager.GetStats()
}
