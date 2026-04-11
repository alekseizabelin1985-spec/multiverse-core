package agent

import (
	"context"
	"fmt"
	"time"
)

// LifecycleManager implements agent.Lifecycle
// Управляет жизненным циклом агентов
type LifecycleManager struct {
	// blueprintParser для парсинга блупринтов
	blueprintParser BlueprintParser
	
	// agentFactory создает новых агентов
	agentFactory AgentFactory
	
	// mu protects concurrent access
	mu sync.RWMutex
	
	// ttlManager для управления временем жизни
	ttlManager *TTLManager
	
	// registeredFactories хранит фабрики для разных типов агентов
	registeredFactories map[string]AgentFactory
	
	// defaultTTL по умолчанию для всех агентов
	defaultTTL time.Duration
	
	// checkInterval интервал проверки TTL
	checkInterval time.Duration
}

// AgentFactory создает агенты
type AgentFactory interface {
	CreateAgent(ctx context.Context, blueprint *AgentBlueprint, context *AgentContext) (Agent, error)
}

// NewLifecycleManager создает новый менеджер жизненного цикла
func NewLifecycleManager(parser BlueprintFactory, defaultTTL time.Duration) *LifecycleManager {
	return &LifecycleManager{
		blueprintParser:     parser,
		agentFactory:        nil, // Должна быть установлена
		registeredFactories: make(map[string]AgentFactory),
		defaultTTL:          defaultTTL,
		checkInterval:       1 * time.Minute, // Проверка каждую минуту
		ttlManager: &TTLManager{
			CheckInterval: 1 * time.Minute,
			DefaultTTL:    defaultTTL,
		},
	}
}

// SetAgentFactory устанавливает фабрику по умолчанию
func (lm *LifecycleManager) SetAgentFactory(factory AgentFactory) {
	lm.agentFactory = factory
}

// RegisterFactory регистрирует фабрику для конкретного типа
func (lm *LifecycleManager) RegisterFactory(agentType string, factory AgentFactory) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.registeredFactories[agentType] = factory
}

// Create создает нового агента
func (lm *LifecycleManager) Create(ctx context.Context, blueprint *AgentBlueprint, context *AgentContext) (Agent, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	
	// Определяем фабрику
	factory := lm.agentFactory
	if f, exists := lm.registeredFactories[blueprint.Type()]; exists {
		factory = f
	}
	
	if factory == nil {
		return nil, fmt.Errorf("no factory registered for agent type %s", blueprint.Type())
	}
	
	// Создаем агента
	agent, err := factory.CreateAgent(ctx, blueprint, context)
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}
	
	return agent, nil
}

// Start запускает агента
func (lm *LifecycleManager) Start(agent Agent) error {
	agentCtx := agent.Context()
	
	// Устанавливаем TTL, если указан в блупринте
	if agentCtx.ExpiresAt == "" && agentCtx.Level != LevelGlobal && agentCtx.Level != LevelMonitor {
		expiresAt := time.Now().Add(lm.defaultTTL)
		agentCtx.ExpiresAt = expiresAt.Format(time.RFC3339)
	}
	
	// Начальная инициализация
	return nil
}

// Stop останавливает агента
func (lm *LifecycleManager) Stop(ctx context.Context, agentID string) error {
	// TODO: реализовать остановку
	// Для агентов: вызвать Shutdown()
	return nil
}

// Pause ставит на паузу
func (lm *LifecycleManager) Pause(ctx context.Context, agentID string) error {
	// TODO: реализовать паузу
	return nil
}

// Resume продолжает работу после паузы
func (lm *LifecycleManager) Resume(ctx context.Context, agentID string) error {
	// TODO: реализовать возобновление
	return nil
}

// TTLManager возвращает менеджер TTL
func (lm *LifecycleManager) TTLManager() *TTLManager {
	return lm.ttlManager
}

// Cleanup запускает очистку просроченных агентов
func (lm *LifecycleManager) Cleanup(ctx context.Context) error {
	now := time.Now()
	
	lm.mu.Lock()
	defer lm.mu.Unlock()
	
	// TODO: перебрать всех агентов и удалить просроченных
	// agentCtx.ExpiresAt < now
	
	return nil
}

// SetDefaultTTL устанавливает TTL по умолчанию
func (lm *LifecycleManager) SetDefaultTTL(ttl time.Duration) {
	lm.defaultTTL = ttl
}

// SetCheckInterval устанавливает интервал проверки TTL
func (lm *LifecycleManager) SetCheckInterval(interval time.Duration) {
	lm.checkInterval = interval
}

// RegisterBlueprintFactory регистрирует фабрику для конкретного блупринта
func (lm *LifecycleManager) RegisterBlueprintFactory(bpName string, factory AgentFactory) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.registeredFactories[bpName] = factory
}

// GetRegisteredFactoriesCount возвращает количество зарегистрированных фабрик
func (lm *LifecycleManager) GetRegisteredFactoriesCount() int {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return len(lm.registeredFactories)
}
