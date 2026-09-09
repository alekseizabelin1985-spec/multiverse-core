// packages/shared/agent/filter.go
package agent

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"multiverse-core.io/shared/rules"
)

// FilterResult результат работы фильтра
type FilterResult struct {
	// Bypassed означает что запрос обработан без LLM
	Bypassed bool

	// DeterministicResult детерминированный результат (если Bypassed=true)
	DeterministicResult *DeterministicOutput

	// Cached означает что результат взят из кэша
	Cached bool

	// CacheKey ключ кэша (если Cached=true)
	CacheKey string

	// Reason причина обхода LLM
	Reason string
}

// DeterministicOutput детерминированный вывод (без LLM)
type DeterministicOutput struct {
	// Phase1Result результат фазы 1 (механика)
	Phase1Result *Phase1Output

	// Phase2Result результат фазы 2 (нарратив) — может быть nil
	Phase2Result *Phase2Output

	// Confidence уверенность в результате (0.0-1.0)
	Confidence float32

	// Source источник результата: "cache", "rule", "pattern"
	Source string
}

// RuleEngineFilter фильтр который обрабатывает 70% запросов без LLM
type RuleEngineFilter struct {
	// engine движок правил
	engine *rules.Engine

	// patternCache кэш паттернов поведения
	patternCache *PatternCache

	// stats статистика
	stats *FilterStats

	// mu защищает concurrent доступ
	mu sync.RWMutex
}

// PatternCache кэш паттернов поведения
type PatternCache struct {
	mu      sync.RWMutex
	patterns map[string]*PatternEntry
	maxSize int
}

// PatternEntry запись паттерна
type PatternEntry struct {
	Key         string
	InputHash   uint32
	Output      *DeterministicOutput
	MatchRate   float32  // Насколько точно совпадает (0.0-1.0)
	LastUsed    time.Time
	UsageCount  int
}

// FilterStats статистика фильтра
type FilterStats struct {
	mu                sync.RWMutex
	TotalRequests     int64
	BypassedRequests  int64
	LLMRequests       int64
	CacheHits         int64
	RuleHits          int64
	PatternHits       int64
	AvgBypassRate     float32
	LastUpdateTime    time.Time
}

// NewRuleEngineFilter создает новый фильтр
func NewRuleEngineFilter(ruleEngine *rules.Engine) *RuleEngineFilter {
	return &RuleEngineFilter{
		engine:       ruleEngine,
		patternCache: &PatternCache{
			patterns: make(map[string]*PatternEntry),
			maxSize:  10000,
		},
		stats: &FilterStats{},
	}
}

// Filter проверяет можно ли обработать запрос без LLM
func (f *RuleEngineFilter) Filter(ctx context.Context, req *AgentRequest) (*FilterResult, error) {
	f.stats.mu.Lock()
	f.stats.TotalRequests++
	f.stats.mu.Unlock()

	// 1. Проверяем кэш по ключу
	if result, ok := f.checkCache(req); ok {
		f.stats.mu.Lock()
		f.stats.CacheHits++
		f.stats.BypassedRequests++
		f.stats.mu.Unlock()

		return &FilterResult{
			Bypassed:            true,
			DeterministicResult: result,
			Cached:              true,
			CacheKey:            f.generateCacheKey(req),
			Reason:              "cache_hit",
		}, nil
	}

	// 2. Проверяем паттерны поведения
	if result, ok := f.checkPatterns(req); ok {
		f.stats.mu.Lock()
		f.stats.PatternHits++
		f.stats.BypassedRequests++
		f.stats.mu.Unlock()

		return &FilterResult{
			Bypassed:            true,
			DeterministicResult: result,
			Cached:              false,
			Reason:              "pattern_match",
		}, nil
	}

	// 3. Проверяем правила
	if result, ok := f.checkRules(req); ok {
		f.stats.mu.Lock()
		f.stats.RuleHits++
		f.stats.BypassedRequests++
		f.stats.mu.Unlock()

		// Сохраняем в кэш
		cacheKey := f.generateCacheKey(req)
		f.saveToCache(req, result, cacheKey)

		return &FilterResult{
			Bypassed:            true,
			DeterministicResult: result,
			Cached:              false,
			CacheKey:            cacheKey,
			Reason:              "rule_match",
		}, nil
	}

	// 4. Требуется LLM
	f.stats.mu.Lock()
	f.stats.LLMRequests++
	f.stats.mu.Unlock()

	return &FilterResult{
		Bypassed: false,
		Reason:   "requires_llm",
	}, nil
}

// checkCache проверяет кэш по ключу
func (f *RuleEngineFilter) checkCache(req *AgentRequest) (*DeterministicOutput, bool) {
	_ = f.generateCacheKey(req)

	// TODO: реализовать проверку Redis кэша
	// if cached, return result

	return nil, false
}

// checkPatterns проверяет паттерны поведения
func (f *RuleEngineFilter) checkPatterns(req *AgentRequest) (*DeterministicOutput, bool) {
	f.patternCache.mu.Lock()
	defer f.patternCache.mu.Unlock()

	inputHash := f.hashInput(req)

	// Ищем точное совпадение
	for _, entry := range f.patternCache.patterns {
		if entry.InputHash == inputHash && entry.MatchRate >= 0.95 {
			entry.UsageCount++
			entry.LastUsed = time.Now()
			return entry.Output, true
		}
	}

	// Ищем приблизительное совпадение
	for _, entry := range f.patternCache.patterns {
		if f.calculateSimilarity(req, entry) >= 0.8 {
			entry.UsageCount++
			entry.LastUsed = time.Now()
			return entry.Output, true
		}
	}

	return nil, false
}

// checkRules проверяет правила через RuleEngine
func (f *RuleEngineFilter) checkRules(req *AgentRequest) (*DeterministicOutput, bool) {
	// Получаем соответствующие правила
	matchingRules := f.getMatchingRules(req)

	if len(matchingRules) == 0 {
		return nil, false
	}

	// Применяем правила
	var results []*rules.RuleResult
	for _, rule := range matchingRules {
		result, err := f.engine.Apply(rule.ID, req.State, req.Modifiers)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, false
	}

	// Формируем детерминированный результат
	output := &DeterministicOutput{
		Phase1Result: &Phase1Output{
			Action:    results[0].RuleID,
			Success:   results[0].Success,
			DiceRoll:  results[0].DiceRoll,
			Total:     results[0].Total,
			Critical:  results[0].Critical,
			Confidence: 1.0, // Полная уверенность в детерминированном результате
		},
		Phase2Result: f.generateNarrativeFromRules(results),
		Confidence:   1.0,
		Source:       "rule",
	}

	// Сохраняем паттерн
	f.patternCache.mu.Lock()
	defer f.patternCache.mu.Unlock()

	if len(f.patternCache.patterns) >= f.patternCache.maxSize {
		// Удаляем наименее используемый
		f.evictLeastUsed()
	}

	entry := &PatternEntry{
		Key:         f.generateCacheKey(req),
		InputHash:   f.hashInput(req),
		Output:      output,
		MatchRate:   1.0,
		LastUsed:    time.Now(),
		UsageCount:  1,
	}

	f.patternCache.patterns[entry.Key] = entry

	return output, true
}

// generateNarrativeFromRules генерирует нарратив на основе правил
func (f *RuleEngineFilter) generateNarrativeFromRules(results []*rules.RuleResult) *Phase2Output {
	// Используем SemanticLayer из правил для генерации нарратива
	// Это полностью детерминированный нарратив без LLM

	// TODO: реализовать генерацию на основе SemanticLayer
	return &Phase2Output{
		Narrative: "Результат обработан правилами",
		Source:    "rule_engine",
	}
}

// getMatchingRules возвращает правила подходящие для запроса
func (f *RuleEngineFilter) getMatchingRules(req *AgentRequest) []*rules.Rule {
	// TODO: реализовать поиск правил по entity_type и контексту
	return nil
}

// saveToCache сохраняет результат в кэш
func (f *RuleEngineFilter) saveToCache(req *AgentRequest, result *DeterministicOutput, key string) {
	// TODO: реализовать сохранение в Redis
	_ = key
}

// generateCacheKey генерирует ключ кэша
func (f *RuleEngineFilter) generateCacheKey(req *AgentRequest) string {
	h := fnv.New32a()
	h.Write([]byte(req.EventType))
	h.Write([]byte(req.EntityType))
	h.Write([]byte(fmt.Sprintf("%v", req.State)))
	return fmt.Sprintf("%x", h.Sum32())
}

// hashInput хэширует входные данные
func (f *RuleEngineFilter) hashInput(req *AgentRequest) uint32 {
	h := fnv.New32a()
	h.Write([]byte(req.EventType))
	h.Write([]byte(req.EntityType))
	return h.Sum32()
}

// calculateSimilarity рассчитывает сходство запроса с паттерном
func (f *RuleEngineFilter) calculateSimilarity(req *AgentRequest, entry *PatternEntry) float32 {
	// TODO: реализовать векторное сходство
	return 0.0
}

// evictLeastUsed удаляет наименее используемый паттерн
func (f *RuleEngineFilter) evictLeastUsed() {
	var leastUsedKey string
	var leastUsedCount int = int(^uint(0) >> 1) // max int

	for key, entry := range f.patternCache.patterns {
		if entry.UsageCount < leastUsedCount {
			leastUsedCount = entry.UsageCount
			leastUsedKey = key
		}
	}

	if leastUsedKey != "" {
		delete(f.patternCache.patterns, leastUsedKey)
	}
}

// GetStats возвращает статистику фильтра
func (f *RuleEngineFilter) GetStats() map[string]interface{} {
	f.stats.mu.RLock()
	total := f.stats.TotalRequests
	bypassed := f.stats.BypassedRequests
	llm := f.stats.LLMRequests
	cacheHits := f.stats.CacheHits
	ruleHits := f.stats.RuleHits
	patternHits := f.stats.PatternHits
	f.stats.mu.RUnlock()

	bypassRate := float32(0)
	if total > 0 {
		bypassRate = float32(bypassed) / float32(total)
	}

	return map[string]interface{}{
		"total_requests":     total,
		"bypassed_requests":  bypassed,
		"llm_requests":       llm,
		"cache_hits":         cacheHits,
		"rule_hits":          ruleHits,
		"pattern_hits":       patternHits,
		"bypass_rate":        bypassRate,
		"target_bypass_rate": 0.7,
	}
}

// ResetStats сбрасывает статистику
func (f *RuleEngineFilter) ResetStats() {
	f.stats.mu.Lock()
	defer f.stats.mu.Unlock()

	f.stats.TotalRequests = 0
	f.stats.BypassedRequests = 0
	f.stats.LLMRequests = 0
	f.stats.CacheHits = 0
	f.stats.RuleHits = 0
	f.stats.PatternHits = 0
	f.stats.AvgBypassRate = 0
	f.stats.LastUpdateTime = time.Now()
}

// AddPattern добавляет паттерн вручную
func (f *RuleEngineFilter) AddPattern(inputHash uint32, output *DeterministicOutput, matchRate float32) {
	f.patternCache.mu.Lock()
	defer f.patternCache.mu.Unlock()

	if len(f.patternCache.patterns) >= f.patternCache.maxSize {
		f.evictLeastUsed()
	}

	cacheKey := fmt.Sprintf("pattern-%x", inputHash)
	entry := &PatternEntry{
		Key:         cacheKey,
		InputHash:   inputHash,
		Output:      output,
		MatchRate:   matchRate,
		LastUsed:    time.Now(),
		UsageCount:  0,
	}

	f.patternCache.patterns[cacheKey] = entry
}

// AgentRequest запрос к агенту
type AgentRequest struct {
	EventType   string
	EntityType  string
	EntityID    string
	State       map[string]float32
	Modifiers   []map[string]interface{}
	Payload     map[string]interface{}
	Timestamp   time.Time
}

// Phase1Output результат фазы 1 (механика)
type Phase1Output struct {
	Action     string  `json:"action"`
	Target     string  `json:"target,omitempty"`
	Success    bool    `json:"success"`
	DiceRoll   int     `json:"dice_roll"`
	Total      int     `json:"total"`
	Critical   bool    `json:"critical"`
	Confidence float32 `json:"confidence"`
}

// Phase2Output результат фазы 2 (нарратив)
type Phase2Output struct {
	Narrative string            `json:"narrative"`
	Emotion   string            `json:"emotion,omitempty"`
	Source    string            `json:"source"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
}

// Validate валидирует фильтр
func (f *RuleEngineFilter) Validate() error {
	if f.engine == nil {
		return fmt.Errorf("rule engine is required")
	}

	if f.patternCache.maxSize <= 0 {
		return fmt.Errorf("pattern cache size must be positive")
	}

	return nil
}
