package agent

// AgentLevel определяет иерархический уровень агента
type AgentLevel int

const (
	LevelUnknown AgentLevel = iota
	LevelGlobal  // Global supervisor - надзор за всем миром
	LevelDomain  // Domain agent - регион/город/зона (TTL ~1 час)
	LevelTask    // Task agent - квест/встреча (TTL ~минуты)
	LevelObject  // Object agent - мгновенная реакция (TTL ~секунды)
	LevelMonitor // Monitor agent - долгосрочный мониторинг аномалий
)

// String возвращает строковое представление уровня
func (l AgentLevel) String() string {
	switch l {
	case LevelGlobal:
		return "global"
	case LevelDomain:
		return "domain"
	case LevelTask:
		return "task"
	case LevelObject:
		return "object"
	case LevelMonitor:
		return "monitor"
	default:
		return "unknown"
	}
}

// AgentBlueprint defines the declarative blueprint for spawning an agent
// Структура блупринта для MD-файла (единый источник: конфиг + промпт + ограничения)
type AgentBlueprint struct {
	// Meta информация
	Name        string              `yaml:"name" json:"name"`
	Version     string              `yaml:"version" json:"version"`
	Description string              `yaml:"description" json:"description"`

	// Trigger conditions для event-driven спавна
	Trigger BlueprintTrigger `yaml:"trigger" json:"trigger"`

	// Constraints ограничения и правила
	Constraints BlueprintConstraints `yaml:"constraints" json:"constraints"`

	// LLM настройки
	LLM LLMConfig `yaml:"llm" json:"llm"`

	// Tools доступные инструменты и действия
	Tools []ToolReference `yaml:"tools" json:"tools"`

	// Parent ссылки на родительского агента
	Parent *ParentReference `yaml:"parent" json:"parent"`

	// TTL для Domain/Task агентов
	TTL string `yaml:"ttl,omitempty" json:"ttl,omitempty"`

	// Prompt templates
	Phase1Prompt string `yaml:"phase1_prompt,omitempty" json:"phase1_prompt,omitempty"`
	Phase2Prompt string `yaml:"phase2_prompt,omitempty" json:"phase2_prompt,omitempty"`

	// Type тип блупринта
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
}

// BlueprintTrigger определяет когда агент должен быть спавнен
type BlueprintTrigger struct {
	// Type тип триггера
	Type string `yaml:"type" json:"type"` // "event", "condition", "timer"
	
	// EventName имя события для спавна
	EventName string `yaml:"event_name,omitempty" json:"event_name,omitempty"`
	
	// Conditions условия спавна (пример: player_count >= 1)
	Conditions []Condition `yaml:"conditions,omitempty" json:"conditions,omitempty"`
}

// BlueprintConstraints определяет ограничения на агент
type BlueprintConstraints struct {
	// MaxInstances максимальное количество экземпляров
	MaxInstances int `yaml:"max_instances,omitempty" json:"max_instances,omitempty"`
	
	// SharedResources разделяемые ресурсы
	SharedResources []ResourceReference `yaml:"shared_resources,omitempty" json:"shared_resources,omitempty"`
	
	// Priority приоритет выполнения
	Priority int `yaml:"priority,omitempty" json:"priority,omitempty"`
}

// LLMConfig конфигурация LLM для агента
type LLMConfig struct {
	// Model название модели (qwen:7b, qwen:72b)
	Model string `yaml:"model" json:"model"`
	
	// Temperature температура генерации
	Temperature float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	
	// MaxTokens максимальное количество токенов
	MaxTokens int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
	
	// Schema JSON schema для валидации ответа
	Schema map[string]interface{} `yaml:"schema,omitempty" json:"schema,omitempty"`
	
	// Fallback модель fallback при сбое
	Fallback string `yaml:"fallback,omitempty" json:"fallback,omitempty"`
}

// ToolReference ссылка на инструмент
type ToolReference struct {
	Name  string `yaml:"name" json:"name"`
	Owner string `yaml:"owner,omitempty" json:"owner,omitempty"`
}

// ParentReference ссылка на родителя
type ParentReference struct {
	Name     string `yaml:"name" json:"name"`
	Instance string `yaml:"instance,omitempty" json:"instance,omitempty"`
}

// Condition простая условная операция
type Condition struct {
	Field   string `yaml:"field" json:"field"`
	Operator string `yaml:"operator" json:"operator"` // >=, <=, ==, !=, >, <
	Value   int    `yaml:"value" json:"value"`
}

// ResourceReference ссылка на ресурс
type ResourceReference struct {
	Name string `yaml:"name" json:"name"`
}

// AgentContext контекст выполнения агента
type AgentContext struct {
	// AgentID уникальный идентификатор агента
	AgentID string `json:"agent_id"`
	
	// AgentType тип агента
	AgentType string `json:"agent_type"`
	
	// Level иерархический уровень
	Level AgentLevel `json:"level"`
	
	// ParentID ID родителя (если есть)
	ParentID string `json:"parent_id,omitempty"`
	
	// ScopeID ID области видимости (region_id, city_id, world_id)
	ScopeID string `json:"scope_id"`
	
	// Entities контекстные сущности
	Entities []EntityRef `json:"entities,omitempty"`
	
	// MemoryRef ссылка на память (ChromaDB/Neo4j)
	MemoryRef string `json:"memory_ref,omitempty"`
	
	// CreatedAt время создания
	CreatedAt string `json:"created_at"`
	
	// ExpiresAt время истечения TTL
	ExpiresAt string `json:"expires_at,omitempty"`
	
	// LOD уровень детализации (0-3)
	LOD LODLevel `json:"lod"`
}

// EntityRef ссылка на сущность
type EntityRef struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	Level LODLevel `json:"lod"`
}

// LODLevel определяет уровень детализации
type LODLevel int

const (
	LODDisabled LODLevel = iota // Агент спит
	LODRuleOnly                  // Только rule-engine (без LLM)
	LODBasic                     // Простой LLM + кэш
	LODFull                      // Полный ReAct loop с инструментами
)

// String возвращает строковое представление LOD
func (l LODLevel) String() string {
	switch l {
	case LODDisabled:
		return "disabled"
	case LODRuleOnly:
		return "rule-only"
	case LODBasic:
		return "basic"
	case LODFull:
		return "full"
	default:
		return "unknown"
	}
}

// AgentLifecycleState состояние жизненного цикла агента
type AgentLifecycleState int

const (
	LifecycleInitializing AgentLifecycleState = iota
	LifecycleRunning
	LifecyclePausing
	LifecyclePaused
	LifecycleResuming
	LifecycleFinishing
	LifecycleFinished
)

// String возвращает строковое представление состояния
func (s AgentLifecycleState) String() string {
	switch s {
	case LifecycleInitializing:
		return "initializing"
	case LifecycleRunning:
		return "running"
	case LifecyclePausing:
		return "pausing"
	case LifecyclePaused:
		return "paused"
	case LifecycleResuming:
		return "resuming"
	case LifecycleFinishing:
		return "finishing"
	case LifecycleFinished:
		return "finished"
	default:
		return "unknown"
	}
}
