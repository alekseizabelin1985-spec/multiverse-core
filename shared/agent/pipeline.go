package agent

import (
	"context"
	"fmt"
	"time"
)

// TwoPhasePipeline implements двухфазный LLM-конвейер
// Фаза 1: Decision (механика) - быстрая, строгий JSON
// Фаза 2: Narrative (описание) - асинхронная, креативная
type TwoPhasePipeline struct {
	// llmClient клиент для вызова LLM
	llmClient LLMClient
	
	// cache кэш для оптимизации
	cache Cache
	
	// fallbackRouter роутер fallback правил
	fallbackRouter *RuleEngineRouter
	
	// config конфигурация конвейера
	config PipelineConfig
}

// PipelineConfig конфигурация конвейера
type PipelineConfig struct {
	// Phase1Model модель для фазы 1 (быстрая)
	Phase1Model string
	
	// Phase2Model модель для фазы 2 (качественная)
	Phase2Model string
	
	// Phase1MaxTokens максимальные токены для фазы 1
	Phase1MaxTokens int
	
	// Phase2MaxTokens максимальные токены для фазы 2
	Phase2MaxTokens int
	
	// EnableCache включать кэш
	EnableCache bool
	
	// CacheTTL время жизни кэша
	CacheTTL time.Duration
	
	// Phase1Timeout таймаут для фазы 1
	Phase1Timeout time.Duration
	
	// Phase2Timeout таймаут для фазы 2
	Phase2Timeout time.Duration
	
	// LogLevel уровень логирования
	LogLevel int
}

// DefaultPipelineConfig возвращает конфиг по умолчанию
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Phase1Model:     "qwen:7b",
		Phase2Model:     "qwen:72b",
		Phase1MaxTokens: 1024,
		Phase2MaxTokens: 2048,
		EnableCache:     true,
		CacheTTL:        1 * time.Hour,
		Phase1Timeout:   2 * time.Second,
		Phase2Timeout:   10 * time.Second,
		LogLevel:        2, // Info
	}
}

// NewTwoPhasePipeline создает двухфазный конвейер
func NewTwoPhasePipeline(client LLMClient, config PipelineConfig) *TwoPhasePipeline {
	return &TwoPhasePipeline{
		llmClient:      client,
		cache:          NewMemoryCache(),
		fallbackRouter: NewRuleEngineRouter(),
		config:         config,
	}
}

// Process выполняет двухфазную обработку события
func (p *TwoPhasePipeline) Process(ctx context.Context, event Event, agent Agent) (*TwoPhaseResult, error) {
	// Фазa 1: Decision
	decision, err := p.processPhase1(ctx, event, agent)
	if err != nil {
		// Fallback на rule-engine
		decision = p.fallbackRouter.Decide(ctx, event)
	}
	
	// Если фаза 2 не нужна, возвращаем результат
	if decision.NextPhase != "narrative" {
		return &TwoPhaseResult{
			Decision: decision,
			Status:   "decision_only",
		}, nil
	}
	
	// Фаза 2: Narrative (асинхронно)
	narrativeCh := make(chan *NarrativeResult, 1)
	go p.processPhase2Async(ctx, event, agent, decision, narrativeCh)
	
	// Возвращаем результат фазы 1
	return &TwoPhaseResult{
		Decision:    decision,
		Narrative:   <-narrativeCh,
		Status:      "completed",
		ProcessedAt: time.Now(),
	}, nil
}

// processPhase1 выполняет фазу 1 (механика)
func (p *TwoPhasePipeline) processPhase1(ctx context.Context, event Event, agent Agent) (*DecisionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.Phase1Timeout)
	defer cancel()
	
	// Проверяем кэш
	if p.config.EnableCache {
		cacheKey := p.getCacheKey(event, "phase1")
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached.(*DecisionResult), nil
		}
	}
	
	// Получаем prompt для фазы 1
	agentBP := agent.Context().Blueprint
	if agentBP == nil {
		return nil, fmt.Errorf("blueprint not available")
	}
	
	prompt := p.buildPhase1Prompt(event, agent, agentBP)
	
	// Вызываем LLM
	result, err := p.llmClient.Generate(ctx, p.config.Phase1Model, prompt, GenerateConfig{
		MaxTokens:   p.config.Phase1MaxTokens,
		StrictJSON:  true,
		Temperature: 0.1, // Низкая температура для строгости
	})
	
	if err != nil {
		return nil, fmt.Errorf("phase1 generate: %w", err)
	}
	
	// Парсим результат
	decision := &DecisionResult{}
	if err := decision.ParseJSON(result.Content); err != nil {
		return nil, fmt.Errorf("parse decision: %w", err)
	}
	
	// Кэшируем результат
	if p.config.EnableCache {
		p.cache.Set(p.getCacheKey(event, "phase1"), decision, p.config.CacheTTL)
	}
	
	return decision, nil
}

// processPhase2Async выполняет фазу 2 асинхронно
func (p *TwoPhasePipeline) processPhase2Async(ctx context.Context, event Event, agent Agent, decision *DecisionResult, ch chan<- *NarrativeResult) {
	defer close(ch)
	
	ctx, cancel := context.WithTimeout(ctx, p.config.Phase2Timeout)
	defer cancel()
	
	// Проверяем кэш
	if p.config.EnableCache {
		cacheKey := p.getCacheKey(event, "phase2")
		if cached, ok := p.cache.Get(cacheKey); ok {
			ch <- cached.(*NarrativeResult)
			return
		}
	}
	
	// Получаем prompt для фазы 2
	agentBP := agent.Context().Blueprint
	if agentBP == nil {
		ch <- &NarrativeResult{
			Text:  "",
			Error: "blueprint not available",
		}
		return
	}
	
	prompt := p.buildPhase2Prompt(event, agent, decision, agentBP)
	
	// Вызываем LLM
	result, err := p.llmClient.Generate(ctx, p.config.Phase2Model, prompt, GenerateConfig{
		MaxTokens:   p.config.Phase2MaxTokens,
		StrictJSON:  false,
		Temperature: 0.7, // Высокая температура для креатива
	})
	
	if err != nil {
		ch <- &NarrativeResult{
			Text:  "",
			Error: err.Error(),
		}
		return
	}
	
	narrative := &NarrativeResult{
		Text:      result.Content,
		Effects:   result.Effects,
		ProcessedAt: time.Now(),
	}
	
	// Кэшируем результат
	if p.config.EnableCache {
		p.cache.Set(p.getCacheKey(event, "phase2"), narrative, p.config.CacheTTL)
	}
	
	ch <- narrative
}

// buildPhase1Prompt строит prompt для фазы 1
func (p *TwoPhasePipeline) buildPhase1Prompt(event Event, agent Agent, bp *AgentBlueprint) string {
	agentCtx := agent.Context()
	
	template := bp.Phase1Prompt
	if template == "" {
		template = defaultPhase1PromptTemplate
	}
	
	// Заполняем переменные
	prompt := template
	prompt = strings.ReplaceAll(prompt, "{player_name}", getEntityName(event.Payload, "player"))
	prompt = strings.ReplaceAll(prompt, "{time_of_day}", "day") // TODO: получить из контекста
	prompt = strings.ReplaceAll(prompt, "{weather}", "clear")   // TODO: получить из контекста
	prompt = strings.ReplaceAll(prompt, "{nearby_entities}", getEntitiesString(event.Payload))
	prompt = strings.ReplaceAll(prompt, "{region_history}", "no history") // TODO: получить из памяти
	
	return prompt
}

// buildPhase2Prompt строит prompt для фазы 2
func (p *TwoPhasePipeline) buildPhase2Prompt(event Event, agent Agent, decision *DecisionResult, bp *AgentBlueprint) string {
	template := bp.Phase2Prompt
	if template == "" {
		template = defaultPhase2PromptTemplate
	}
	
	// Заполняем переменные
	prompt := template
	prompt = strings.ReplaceAll(prompt, "{player_name}", getEntityName(event.Payload, "player"))
	prompt = strings.ReplaceAll(prompt, "{event_description}", event.Type)
	prompt = strings.ReplaceAll(prompt, "{phase1_result}", decision.String())
	
	return prompt
}

// getCacheKey создает ключ для кэша
func (p *TwoPhasePipeline) getCacheKey(event Event, phase string) string {
	return fmt.Sprintf("%s:%s:%s", event.ScopeID, event.Type, phase)
}

// DecisionResult результат фазы 1
type DecisionResult struct {
	// Decisions решения
	Decisions []Decision `json:"decisions"`
	
	// NextPhase следующая фаза
	NextPhase string `json:"narrative_phase"`
	
	// ProcessedAt время обработки
	ProcessedAt time.Time
	
	// Validated флаг валидации
	Validated bool
}

// Decision одно решение
type Decision struct {
	Type    string                 `json:"type"`
	Target  string                 `json:"target"`
	Payload map[string]interface{} `json:"payload"`
}

// String возвращает строковое представление
func (d *DecisionResult) String() string {
	return fmt.Sprintf("%d decisions, next phase: %s", len(d.Decisions), d.NextPhase)
}

// ParseJSON парсит JSON результат
func (d *DecisionResult) ParseJSON(content string) error {
	return json.Unmarshal([]byte(content), d)
}

// NarrativeResult результат фазы 2
type NarrativeResult struct {
	// Text текст нарратива
	Text string `json:"text"`
	
	// Effects эффекты
	Effects []NarrativeEffect `json:"effects,omitempty"`
	
	// Error ошибка если есть
	Error string `json:"error,omitempty"`
	
	// ProcessedAt время обработки
	ProcessedAt time.Time
	
	// Validated флаг валидации
	Validated bool
}

// NarrativeEffect эффект нарратива
type NarrativeEffect struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// TwoPhaseResult общий результат
type TwoPhaseResult struct {
	// Decision результат фазы 1
	Decision *DecisionResult
	
	// Narrative результат фазы 2
	Narrative *NarrativeResult
	
	// Status статус обработки
	Status string
	
	// ProcessedAt время обработки
	ProcessedAt time.Time
}

// LLMClient интерфейс клиента LLM
type LLMClient interface {
	Generate(ctx context.Context, model string, prompt string, config GenerateConfig) (*LLMResult, error)
}

// GenerateConfig конфигурация генерации
type GenerateConfig struct {
	MaxTokens   int
	StrictJSON  bool
	Temperature float64
}

// LLMResult результат генерации
type LLMResult struct {
	Content string
	Effects []NarrativeEffect
}

// defaultPhase1PromptTemplate по умолчанию
const defaultPhase1PromptTemplate = `Ты — Game Master. Игрок {player_name} совершил действие: {event_description}.
Текущая ситуация:
- Окружение: {weather}, {time_of_day}
- Сущности рядом: {nearby_entities}

Определи:
1. Результат действия (hit, damage, status)
2. Следующее событие
3. Нужно ли нарративное описание?

Верни ТОЛЬКО JSON:
{
  "decisions": [
    {"type": "combat", "target": "enemy-1", "payload": {"damage": 5}}
  ],
  "narrative_phase": "skip"
}`

// defaultPhase2PromptTemplate по умолчанию
const defaultPhase2PromptTemplate = `Опиши событие в атмосферной манере.
Игрок: {player_name}
Событие: {event_description}
Результат: {phase1_result}

Будь креативным, описывай звуки, запахи, ощущения.`

// Мок-реализации для тестов

type MockLLMClient struct{}

func (m *MockLLMClient) Generate(ctx context.Context, model string, prompt string, config GenerateConfig) (*LLMResult, error) {
	return &LLMResult{
		Content: `{"decisions": [], "narrative_phase": "skip"}`,
	}, nil
}

type MockCache struct {
	data map[string]interface{}
}

func NewMemoryCache() *MockCache {
	return &MockCache{data: make(map[string]interface{})}
}

func (c *MockCache) Get(key string) (interface{}, bool) {
	val, ok := c.data[key]
	return val, ok
}

func (c *MockCache) Set(key string, value interface{}, ttl time.Duration) {
	c.data[key] = value
}

func (c *MockCache) Delete(key string) {
	delete(c.data, key)
}

func getEntityName(payload map[string]interface{}, entity string) string {
	if e, ok := payload[entity]; ok {
		if m, ok := e.(map[string]interface{}); ok {
			if name, ok := m["name"]; ok {
				return name.(string)
			}
		}
	}
	return "unknown"
}

func getEntitiesString(payload map[string]interface{}) string {
	return "" // TODO: реализовать
}
