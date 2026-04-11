package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Router implements agent.Router
// Маршрутизация событий к агентам на основе блупринтов
type Router struct {
	// blueprintsByType maps blueprint names to their definitions
	blueprints map[string]*AgentBlueprint
	
	// agents stores running agent instances
	agents map[string]Agent
	
	// lifecycle manages agent lifecycle
	lifecycle Lifecycle
	
	// mu protects concurrent access
	mu sync.RWMutex
	
	// startedAt tracks when router was initialized
	startedAt time.Time
}

// NewRouter creates a new event router
func NewRouter(lifecycle Lifecycle) *Router {
	return &Router{
		blueprints: make(map[string]*AgentBlueprint),
		agents:     make(map[string]Agent),
		lifecycle:  lifecycle,
		startedAt:  time.Now(),
	}
}

// RegisterBlueprint регистрирует блупринт агента
func (r *Router) RegisterBlueprint(bp *AgentBlueprint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Валидация
	if bp.Name == "" {
		return fmt.Errorf("blueprint name is required")
	}
	
	// Проверка дубликатов
	if _, exists := r.blueprints[bp.Name]; exists {
		return fmt.Errorf("blueprint %s already exists", bp.Name)
	}
	
	r.blueprints[bp.Name] = bp
	
	return nil
}

// GetBlueprint получает блупринт по имени
func (r *Router) GetBlueprint(name string) (*AgentBlueprint, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	bp, exists := r.blueprints[name]
	return bp, exists
}

// MatchEvents находит подходящие блупринты для события
func (r *Router) MatchEvents(event Event) []*AgentBlueprint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var matches []*AgentBlueprint
	
	// Проверяем все блупринты
	for _, bp := range r.blueprints {
		if r.eventMatchesTrigger(event, bp) {
			matches = append(matches, bp)
		}
	}
	
	return matches
}

// eventMatchesTrigger проверяет, соответствует ли событие триггеру блупринта
func (r *Router) eventMatchesTrigger(event Event, bp *AgentBlueprint) bool {
	// Простая проверка: тип события и имя события
	if event.Type != bp.Trigger.Type && bp.Trigger.EventName != event.Type {
		return false
	}
	
	// Если есть условия, проверяем их
	if len(bp.Trigger.Conditions) > 0 {
		// TODO: реализовать оценку условий
		// Пока возвращаем true, если событие совпадает по типу
		return true
	}
	
	return true
}

// SpawnAgent создает агента по блупринту
func (r *Router) SpawnAgent(ctx context.Context, blueprint *AgentBlueprint, context *AgentContext) (Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Генерируем уникальный ID
	agentID := uuid.New().String()
	
	// Обновляем контекст
	context.AgentID = agentID
	
	// Создаем агента через lifecycle
	agent, err := r.lifecycle.Create(ctx, blueprint, context)
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}
	
	// Сохраняем в карте
	r.agents[agentID] = agent
	
	// Запускаем агента
	if err := r.lifecycle.Start(agent); err != nil {
		return nil, fmt.Errorf("start agent: %w", err)
	}
	
	return agent, nil
}

// UnregisterAgent удаляет агента
func (r *Router) UnregisterAgent(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}
	
	// Останавливаем через lifecycle
	if err := r.lifecycle.Stop(context.Background(), agentID); err != nil {
		return fmt.Errorf("stop agent: %w", err)
	}
	
	delete(r.agents, agentID)
	
	return nil
}

// GetAgent получает агента по ID
func (r *Router) GetAgent(agentID string) (Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	agent, exists := r.agents[agentID]
	return agent, exists
}

// ListAgents возвращает список всех агентов
func (r *Router) ListAgents() []Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	agents := make([]Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	
	return agents
}

// RouteEvent маршрутизирует событие
func (r *Router) RouteEvent(ctx context.Context, event Event) error {
	// Находим подходящие блупринты
	bps := r.MatchEvents(event)
	
	if len(bps) == 0 {
		// Нет подходящих агентов, но это не ошибка
		return nil
	}
	
	// Для каждого блупринта создаем агента
	for _, bp := range bps {
		// Проверяем, есть ли уже агент для этого события
		existing := r.findExistingAgent(event.ScopeID, bp.Name)
		if existing != nil {
			// Уведомляем существующего агента
			if err := existing.HandleEvent(ctx, event); err != nil {
				return fmt.Errorf("handle event for existing agent: %w", err)
			}
			continue
		}
		
		// Создаем нового агента
		agentCtx := &AgentContext{
			ScopeID:   event.ScopeID,
			Level:     r.determineLevel(bp),
			LOD:       LODBasic, // Начальный уровень детализации
		}
		
		agent, err := r.SpawnAgent(ctx, bp, agentCtx)
		if err != nil {
			return fmt.Errorf("spawn agent: %w", err)
		}
		
		// Обработаем событие новым агентом
		if err := agent.HandleEvent(ctx, event); err != nil {
			return fmt.Errorf("handle event for new agent: %w", err)
		}
	}
	
	return nil
}

// findExistingAgent ищет существующего агента для scope
func (r *Router) findExistingAgent(scopeID, agentType string) Agent {
	for _, agent := range r.agents {
		if agent.Context().ScopeID == scopeID && agent.Type() == agentType {
			return agent
		}
	}
	return nil
}

// determineLevel определяет уровень агента из блупринта
func (r *Router) determineLevel(bp *AgentBlueprint) AgentLevel {
	switch bp.Trigger.Type {
	case "event":
		if bp.TTL != "" {
			return LevelDomain // Долгоживущий агент
		}
		return LevelTask // Краткосрочный
	case "condition":
		return LevelMonitor
	default:
		return LevelUnknown
	}
}

// GetBlueprintsCount возвращает количество зарегистрированных блупринтов
func (r *Router) GetBlueprintsCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.blueprints)
}

// GetAgentsCount возвращает количество активных агентов
func (r *Router) GetAgentsCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// StartTime возвращает время инициализации роутера
func (r *Router) StartTime() time.Time {
	return r.startedAt
}

// Stats возвращает статистику роутера
func (r *Router) Stats() map[string]interface{} {
	return map[string]interface{}{
		"blueprints_count":    r.GetBlueprintsCount(),
		"agents_count":        r.GetAgentsCount(),
		"started_at":          r.StartTime().Format(time.RFC3339),
		"uptime_seconds":      time.Since(r.StartTime()).Seconds(),
	}
}
