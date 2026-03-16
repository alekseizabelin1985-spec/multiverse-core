// internal/intent/cache.go
package intent

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// IntentCache кэш распознанных намерений с 24h TTL
type IntentCache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	ttl     time.Duration
	maxSize int
	hits    int64
	misses  int64
}

type cacheItem struct {
	intent     *IntentResponse
	hash       string
	createdAt  time.Time
	lastUsedAt time.Time
	expiresAt  time.Time
}

// CacheStats статистика кэша
type CacheStats struct {
	Size      int     `json:"size"`
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	HitRate   float64 `json:"hit_rate"`
	MaxSize   int     `json:"max_size"`
	AvgAgeSec float64 `json:"avg_age_sec"`
}

// NewIntentCache создает новый кэш намерений
func NewIntentCache(ttl time.Duration, maxSize int) *IntentCache {
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	if maxSize == 0 {
		maxSize = 10000
	}

	cache := &IntentCache{
		items:   make(map[string]*cacheItem),
		ttl:     ttl,
		maxSize: maxSize,
	}

	// Запускаем очистку просроченных элементов
	go cache.startCleanup()

	return cache
}

// Get получает намерение из кэша
func (c *IntentCache) Get(playerText string, entityID string, context string) (*IntentResponse, bool) {
	hash := c.computeHash(playerText, entityID, context)

	c.mu.RLock()
	item, exists := c.items[hash]
	if !exists {
		c.mu.RUnlock()
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Проверяем не истекло ли время
	if time.Now().After(item.expiresAt) {
		c.mu.RUnlock()
		c.removeExpired(hash)
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Обновляем lastUsedAt
	c.mu.RUnlock()
	c.mu.Lock()
	item.lastUsedAt = time.Now()
	c.hits++
	c.mu.Unlock()

	return item.intent, true
}

// Put добавляет намерение в кэш
func (c *IntentCache) Put(playerText string, entityID string, context string, intent *IntentResponse) {
	hash := c.computeHash(playerText, entityID, context)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Если уже есть - обновляем
	if item, exists := c.items[hash]; exists {
		item.intent = intent
		item.lastUsedAt = time.Now()
		item.expiresAt = time.Now().Add(c.ttl)
		return
	}

	// Если достигли maxSize - удаляем самые старые
	if len(c.items) >= c.maxSize {
		c.evictOldest()
	}

	// Добавляем новый элемент
	c.items[hash] = &cacheItem{
		intent:     intent,
		hash:       hash,
		createdAt:  time.Now(),
		lastUsedAt: time.Now(),
		expiresAt:  time.Now().Add(c.ttl),
	}
}

// Remove удаляет намерение из кэша
func (c *IntentCache) Remove(playerText string, entityID string, context string) {
	hash := c.computeHash(playerText, entityID, context)
	c.removeExpired(hash)
}

// Clear очищает весь кэш
func (c *IntentCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}

// GetStats возвращает статистику кэша
func (c *IntentCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalRequests := c.hits + c.misses
	hitRate := 0.0
	if totalRequests > 0 {
		hitRate = float64(c.hits) / float64(totalRequests)
	}

	totalAge := 0.0
	count := 0
	for _, item := range c.items {
		totalAge += time.Since(item.createdAt).Seconds()
		count++
	}

	avgAge := 0.0
	if count > 0 {
		avgAge = totalAge / float64(count)
	}

	return CacheStats{
		Size:      len(c.items),
		Hits:      c.hits,
		Misses:    c.misses,
		HitRate:   hitRate,
		MaxSize:   c.maxSize,
		AvgAgeSec: avgAge,
	}
}

// computeHash вычисляет хэш для ключа кэша
func (c *IntentCache) computeHash(playerText, entityID, context string) string {
	data := playerText + "|" + entityID + "|" + context
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// removeExpired удаляет просроченный элемент
func (c *IntentCache) removeExpired(hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, hash)
}

// evictOldest удаляет самый старый элемент
func (c *IntentCache) evictOldest() {
	var oldestHash string
	var oldestTime time.Time

	for hash, item := range c.items {
		if oldestHash == "" || item.lastUsedAt.Before(oldestTime) {
			oldestHash = hash
			oldestTime = item.lastUsedAt
		}
	}

	if oldestHash != "" {
		delete(c.items, oldestHash)
	}
}

// startCleanup запускает фоновую очистку просроченных элементов
func (c *IntentCache) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanupExpired()
	}
}

// cleanupExpired очищает просроченные элементы
func (c *IntentCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for hash, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, hash)
		}
	}
}

// DedupKey создает ключ для дедупликации
func (c *IntentCache) DedupKey(playerText string, entityID string) string {
	return c.computeHash(playerText, entityID, "")
}

// IsDuplicate проверяет является ли запрос дубликатом
func (c *IntentCache) IsDuplicate(playerText string, entityID string, window time.Duration) bool {
	hash := c.computeHash(playerText, entityID, "")

	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[hash]
	if !exists {
		return false
	}

	// Проверяем не слишком ли давно был последний запрос
	return time.Since(item.lastUsedAt) < window
}
