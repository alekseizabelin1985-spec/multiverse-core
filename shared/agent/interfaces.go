package agent

import (
	"context"
	"time"
)

// Agent - интерфейс основного агента
type Agent interface {
	// ID возвращает уникальный идентификатор
	ID() string
	
	// Type возвращает тип агента
	Type() string
	
	// Level возвращает иерархический уровень
	Level() AgentLevel
	
	// State возвращает текущее состояние жизненного цикла
	State() AgentLifecycleState
	
	// Context возвращает контекст агента
	Context() *AgentContext
	
	// Tick выполняет один такт агента с входящим событием
	Tick(ctx context.Context, event Event) (Action, error)
	
	// HandleEvent обрабатывает событие
	HandleEvent(ctx context.Context, event Event) error
	
	// Shutdown завершает работу агента
	Shutdown(ctx context.Context) error
	
	// Pause ставит на паузу
	Pause(ctx context.Context) error
	
	// Resume продолжает работу после паузы
	Resume(ctx context.Context) error
	
	// Memory возвращает доступ к памяти агента
	Memory() MemoryStore
	
	// Tools возвращает доступ к инструментам
	Tools() ToolRegistry
}

// Event - базовое событие для агентов
type Event struct {
	// Type тип события
	Type string `json:"type"`
	
	// ID уникальный идентификатор события
	ID string `json:"id"`
	
	// Timestamp время события
	Timestamp time.Time `json:"timestamp"`
	
	// Payload полезная нагрузка
	Payload map[string]interface{} `json:"payload"`
	
	// ScopeID область видимости
	ScopeID string `json:"scope_id"`
}

// Action - действие, возвращаемое агентом
type Action struct {
	// Type тип действия
	Type string `json:"type"`
	
	// Target цель действия
	Target string `json:"target"`
	
	// Payload полезная нагрузка
	Payload map[string]interface{} `json:"payload"`
	
	// Priority приоритет выполнения
	Priority int `json:"priority"`
	
	// Async флаг асинхронности
	Async bool `json:"async"`
}

// MemoryStore - интерфейс хранилища памяти
type MemoryStore interface {
	// Save сохраняет векторное представление
	Save(ctx context.Context, entityID string, vector []float64) error
	
	// Load загружает векторное представление
	Load(ctx context.Context, entityID string) ([]float64, error)
	
	// Search выполняет семантический поиск
	Search(ctx context.Context, query []float64, topK int) ([]MemoryResult, error)
	
	// Delete удаляет запись
	Delete(ctx context.Context, entityID string) error
}

// MemoryResult - результат поиска в памяти
type MemoryResult struct {
	// EntityID идентификатор сущности
	EntityID string `json:"entity_id"`
	
	// Score сходимость
	Score float64 `json:"score"`
	
	// Vector векторное представление
	Vector []float64 `json:"vector"`
	
	// Metadata дополнительные данные
	Metadata map[string]interface{} `json:"metadata"`
}

// ToolRegistry - интерфейс реестра инструментов
type ToolRegistry interface {
	// Register регистрирует инструмент
	Register(name string, fn ToolFunc, schema map[string]interface{}) error
	
	// Get получает инструмент по имени
	Get(name string) (ToolFunc, bool)
	
	// List возвращает список зарегистрированных инструментов
	List() []string
	
	// Execute выполняет инструмент
	Execute(ctx context.Context, name string, args map[string]interface{}) (interface{}, error)
}

// ToolFunc тип функции инструмента
type ToolFunc func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// Router - интерфейс маршрутизатора событий
type Router interface {
	// RegisterBlueprint регистрирует блупринт агента
	RegisterBlueprint(bp *AgentBlueprint) error
	
	// GetBlueprint получает блупринт по имени
	GetBlueprint(name string) (*AgentBlueprint, error)
	
	// MatchEvents находит подходящие блупринты для события
	MatchEvents(event Event) []*AgentBlueprint
	
	// SpawnAgent создает агента по блупринту
	SpawnAgent(ctx context.Context, blueprint *AgentBlueprint, context *AgentContext) (Agent, error)
	
	// UnregisterAgent удаляет агента
	UnregisterAgent(agentID string) error
	
	// GetAgent получает агента по ID
	GetAgent(agentID string) (Agent, bool)
	
	// ListAgents возвращает список всех агентов
	ListAgents() []Agent
	
	// RouteEvent маршрутизирует событие
	RouteEvent(ctx context.Context, event Event) error
}

// Lifecycle - интерфейс жизненного цикла
type Lifecycle interface {
	// Create создает нового агента
	Create(ctx context.Context, blueprint *AgentBlueprint, context *AgentContext) (Agent, error)
	
	// Start запускает агента
	Start(agent Agent) error
	
	// Stop останавливает агента
	Stop(ctx context.Context, agentID string) error
	
	// Pause ставит на паузу
	Pause(ctx context.Context, agentID string) error
	
	// Resume продолжает работу
	Resume(ctx context.Context, agentID string) error
	
	// TTLManager проверяет TTL и удаляет просроченных агентов
	TTLManager() *TTLManager
	
	// Cleanup запускает очистку
	Cleanup(ctx context.Context) error
}

// TTLManager управляет временем жизни агентов
type TTLManager struct {
	// CheckInterval интервал проверки TTL
	CheckInterval time.Duration
	
	// DefaultTTL время жизни по умолчанию
	DefaultTTL time.Duration
}

// Worker - интерфейс воркера для обработки агентов
type Worker interface {
	// Process обрабатывает одного агента
	Process(ctx context.Context, agent Agent, event Event) error
	
	// ProcessBatch обрабатывает batch событий
	ProcessBatch(ctx context.Context, events []Event) error
	
	// Shutdown завершает работу воркера
	Shutdown(ctx context.Context) error
	
	// Status возвращает статус воркера
	Status() WorkerStatus
}

// WorkerStatus статус воркера
type WorkerStatus struct {
	// WorkerID идентификатор воркера
	WorkerID string `json:"worker_id"`
	
	// Status статус "running", "idle", "stopped"
	Status string `json:"status"`
	
	// ProcessedCount количество обработанных событий
	ProcessedCount int64 `json:"processed_count"`
	
	// LastEventTime время последнего события
	LastEventTime time.Time `json:"last_event_time"`
	
	// ErrorCount количество ошибок
	ErrorCount int64 `json:"error_count"`
}

// BlueprintParser - интерфейс парсера блупринтов
type BlueprintParser interface {
	// ParseFile парсит файл блупринта
	ParseFile(path string) (*AgentBlueprint, error)
	
	// ParseYAML парсит YAML содержание
	ParseYAML(data []byte) (*AgentBlueprint, error)
	
	// Validate валидирует блупринт
	Validate(bp *AgentBlueprint) error
	
	// Serialize serializes blueprint to YAML
	Serialize(bp *AgentBlueprint) ([]byte, error)
}
