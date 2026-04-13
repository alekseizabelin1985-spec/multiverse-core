// agentruntime/cache/prompt.go
package cache

import (
	"sync"
	"time"
)

// PromptCache — L1 in-memory кэш для system prompt-ов.
// Ключ: rule_id (или стилевой ключ для Phase2).
// Ценность: одинаковый system prompt → Ollama внутренне переиспользует KV cache.
type PromptCache struct {
	mu    sync.RWMutex
	items map[string]*promptEntry
	ttl   time.Duration
}

type promptEntry struct {
	prompt    string
	expiresAt time.Time
}

// NewPromptCache создаёт L1 кэш с заданным TTL (рекомендуется 1 час)
func NewPromptCache(ttl time.Duration) *PromptCache {
	pc := &PromptCache{
		items: make(map[string]*promptEntry),
		ttl:   ttl,
	}
	go pc.sweepLoop()
	return pc
}

// GetOrBuild возвращает кэшированный prompt или строит новый через buildFn.
// buildFn вызывается только при промахе кэша.
func (pc *PromptCache) GetOrBuild(key string, buildFn func() string) string {
	// Быстрый путь — read lock
	pc.mu.RLock()
	if entry, ok := pc.items[key]; ok && time.Now().Before(entry.expiresAt) {
		pc.mu.RUnlock()
		return entry.prompt
	}
	pc.mu.RUnlock()

	// Промах — строим и записываем
	prompt := buildFn()

	pc.mu.Lock()
	pc.items[key] = &promptEntry{
		prompt:    prompt,
		expiresAt: time.Now().Add(pc.ttl),
	}
	pc.mu.Unlock()

	return prompt
}

// sweepLoop периодически удаляет устаревшие записи
func (pc *PromptCache) sweepLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		pc.mu.Lock()
		for k, v := range pc.items {
			if now.After(v.expiresAt) {
				delete(pc.items, k)
			}
		}
		pc.mu.Unlock()
	}
}

// ResultCache — L2 Redis-like кэш для результатов Phase1 LLM.
// При одинаковых входных данных (AoE, массовые атаки) → пропустить LLM.
// NOTE: в текущей реализации in-memory; для продакшна заменить на Redis.
type ResultCache struct {
	mu    sync.RWMutex
	items map[string]*resultEntry
	ttl   time.Duration
}

type resultEntry struct {
	value     interface{}
	expiresAt time.Time
}

// NewResultCache создаёт L2 кэш с заданным TTL (рекомендуется 30 сек)
func NewResultCache(ttl time.Duration) *ResultCache {
	rc := &ResultCache{
		items: make(map[string]*resultEntry),
		ttl:   ttl,
	}
	go rc.sweepLoop()
	return rc
}

// Get возвращает закэшированный результат Phase1 или nil
func (rc *ResultCache) Get(key string) interface{} {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	if entry, ok := rc.items[key]; ok && time.Now().Before(entry.expiresAt) {
		return entry.value
	}
	return nil
}

// Set сохраняет результат Phase1 в кэш
func (rc *ResultCache) Set(key string, value interface{}) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.items[key] = &resultEntry{value: value, expiresAt: time.Now().Add(rc.ttl)}
}

func (rc *ResultCache) sweepLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rc.mu.Lock()
		for k, v := range rc.items {
			if now.After(v.expiresAt) {
				delete(rc.items, k)
			}
		}
		rc.mu.Unlock()
	}
}

// NarrativeCache — L3 кэш для нарративных текстов Phase2.
// Ключ: outcome_tag + стиль. Похожие ситуации → переиспользовать текст (5 мин TTL).
type NarrativeCache struct {
	mu    sync.RWMutex
	items map[string]*narrativeEntry
	ttl   time.Duration
}

type narrativeEntry struct {
	text      string
	expiresAt time.Time
}

// NewNarrativeCache создаёт L3 кэш с заданным TTL (рекомендуется 5 минут)
func NewNarrativeCache(ttl time.Duration) *NarrativeCache {
	nc := &NarrativeCache{
		items: make(map[string]*narrativeEntry),
		ttl:   ttl,
	}
	go nc.sweepLoop()
	return nc
}

// Get возвращает закэшированный нарратив или пустую строку
func (nc *NarrativeCache) Get(key string) string {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	if entry, ok := nc.items[key]; ok && time.Now().Before(entry.expiresAt) {
		return entry.text
	}
	return ""
}

// Set сохраняет нарратив в кэш
func (nc *NarrativeCache) Set(key, text string) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.items[key] = &narrativeEntry{text: text, expiresAt: time.Now().Add(nc.ttl)}
}

func (nc *NarrativeCache) sweepLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		nc.mu.Lock()
		for k, v := range nc.items {
			if now.After(v.expiresAt) {
				delete(nc.items, k)
			}
		}
		nc.mu.Unlock()
	}
}
