// packages/shared/agent/state_manager.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// RedisClient интерфейс для кэша (заменяет go-redis)
type RedisClient interface {
	Get(ctx context.Context, key string) *StringResult
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Cleanup()
}

// StringResult результат Get операции
type StringResult struct {
	Value []byte
	Err   error
}

// InMemoryCache простой in-memory кэш (заменяет Redis)
type InMemoryCache struct {
	mu   sync.RWMutex
	data map[string][]byte
	ttls map[string]time.Time
}

// NewInMemoryCache создает in-memory кэш
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Time),
	}
}

// Get получает значение из кэша
func (c *InMemoryCache) Get(ctx context.Context, key string) *StringResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, exists := c.data[key]
	if !exists {
		return &StringResult{Err: fmt.Errorf("not found")}
	}

	return &StringResult{Value: value}
}

// Set сохраняет значение в кэш
func (c *InMemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = value
	if ttl > 0 {
		c.ttls[key] = time.Now().Add(ttl)
	}

	return nil
}

// Delete удаляет значение из кэша
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
	delete(c.ttls, key)

	return nil
}

// Cleanup удаляет истекшие TTL
func (c *InMemoryCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, ttl := range c.ttls {
		if now.After(ttl) {
			delete(c.data, key)
			delete(c.ttls, key)
		}
	}
}

// StateManager управляет контекстом агентов с версионированием
type StateManager struct {
	// cache для hot-данных (быстрый доступ)
	cache RedisClient

	// pgStore для cold-данных (архив)
	pgStore *PGStore

	// versionHistory история версий
	versionHistory map[string][]VersionEntry

	// mu защищает versionHistory
	mu sync.RWMutex

	// config конфигурация
	config StateManagerConfig

	// stats статистика
	stats *StateManagerStats
}

// StateManagerConfig конфигурация StateManager
type StateManagerConfig struct {
	// Redis TTL для hot-данных (по умолчанию 30m)
	HotTTL time.Duration

	// PG архивация через (по умолчанию 1h)
	ColdArchiveAfter time.Duration

	// Max versions to keep (по умолчанию 100)
	MaxVersions int

	// Sync interval (по умолчанию 5m)
	SyncInterval time.Duration
}

// DefaultStateManagerConfig возвращает конфигурацию по умолчанию
func DefaultStateManagerConfig() StateManagerConfig {
	return StateManagerConfig{
		HotTTL:           30 * time.Minute,
		ColdArchiveAfter: 1 * time.Hour,
		MaxVersions:      100,
		SyncInterval:     5 * time.Minute,
	}
}

// VersionEntry запись версии контекста
type VersionEntry struct {
	Version    int       `json:"version"`
	Timestamp  time.Time `json:"timestamp"`
	Hash       string    `json:"hash"`
	Size       int       `json:"size"`
	IsSnapshot bool      `json:"is_snapshot"`
}

// StateManagerStats статистика StateManager
type StateManagerStats struct {
	mu              sync.RWMutex
	TotalLoads      int64
	TotalSaves      int64
	TotalRollbacks  int64
	CacheHits       int64
	CacheMisses     int64
	PGHits          int64
	PGMisses        int64
	AvgLoadTime     time.Duration
	AvgSaveTime     time.Duration
}

// PGStore интерфейс для PostgreSQL хранилища
type PGStore struct {
	// TODO: реализовать PostgreSQL клиент
	// Это заглушка для будущей интеграции
}

// NewStateManager создает новый StateManager
func NewStateManager(cache RedisClient, pgStore *PGStore, config StateManagerConfig) *StateManager {
	return &StateManager{
		cache:          cache,
		pgStore:        pgStore,
		versionHistory: make(map[string][]VersionEntry),
		config:         config,
		stats:          &StateManagerStats{},
	}
}

// Save сохраняет контекст агента
func (sm *StateManager) Save(ctx context.Context, agentID string, context *AgentContext) error {
	start := time.Now()

	// Генерируем новую версию
	sm.mu.Lock()
	versions := sm.versionHistory[agentID]
	newVersion := 1
	if len(versions) > 0 {
		newVersion = versions[len(versions)-1].Version + 1
	}
	sm.mu.Unlock()

	// Серилизуем контекст
	data, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("marshal context: %w", err)
	}

	// Вычисляем хэш
	hash := fmt.Sprintf("%x", data)

	// Сохраняем в кэш (hot)
	key := sm.makeKey(agentID, newVersion)
	ttl := sm.config.HotTTL

	if err := sm.cache.Set(ctx, key, data, ttl); err != nil {
		return fmt.Errorf("save to cache: %w", err)
	}

	// Сохраняем метаданные
	metaKey := sm.makeMetaKey(agentID)
	meta := AgentMeta{
		Version:   newVersion,
		Hash:      hash,
		Size:      len(data),
		UpdatedAt: time.Now(),
	}
	metaData, _ := json.Marshal(meta)
	_ = sm.cache.Set(ctx, metaKey, metaData, ttl)

	// Обновляем историю версий
	sm.mu.Lock()
	sm.versionHistory[agentID] = append(versions, VersionEntry{
		Version:    newVersion,
		Timestamp:  time.Now(),
		Hash:       hash,
		Size:       len(data),
		IsSnapshot: false,
	})

	// Ограничиваем количество версий
	if len(sm.versionHistory[agentID]) > sm.config.MaxVersions {
		sm.versionHistory[agentID] = sm.versionHistory[agentID][len(sm.versionHistory[agentID])-sm.config.MaxVersions:]
	}
	sm.mu.Unlock()

	// Архивируем в PG если прошло достаточно времени
	if time.Since(meta.UpdatedAt) > sm.config.ColdArchiveAfter {
		sm.archiveToPG(ctx, agentID, newVersion, data)
	}

	// Обновляем статистику
	sm.updateStats(start, true, false)

	return nil
}

// Load загружает контекст агента
func (sm *StateManager) Load(ctx context.Context, agentID string) (*AgentContext, error) {
	start := time.Now()

	// 1. Проверяем кэш
	metaKey := sm.makeMetaKey(agentID)
	result := sm.cache.Get(ctx, metaKey)
	if result.Err == nil {
		var meta AgentMeta
		if err := json.Unmarshal(result.Value, &meta); err == nil {
			// Загружаем последнюю версию из кэша
			key := sm.makeKey(agentID, meta.Version)
			data := sm.cache.Get(ctx, key)
			if data.Err == nil {
				var context AgentContext
				if err := json.Unmarshal(data.Value, &context); err == nil {
					sm.updateStats(start, false, true)
					return &context, nil
				}
			}
		}
	}

	// 2. Проверяем PG
	data, err := sm.loadFromPG(ctx, agentID)
	if err == nil {
		var context AgentContext
		if err := json.Unmarshal(data, &context); err == nil {
			// Восстанавливаем в кэш
			sm.cache.Set(ctx, metaKey, result.Value, sm.config.HotTTL)
			sm.updateStats(start, false, false)
			return &context, nil
		}
	}

	sm.updateStats(start, false, false)
	return nil, fmt.Errorf("agent %s context not found", agentID)
}

// Rollback откатывает контекст к предыдущей версии
func (sm *StateManager) Rollback(ctx context.Context, agentID string, targetVersion int) (*AgentContext, error) {
	start := time.Now()

	// Проверяем что версия существует
	sm.mu.RLock()
	versions := sm.versionHistory[agentID]
	sm.mu.RUnlock()

	found := false
	for _, v := range versions {
		if v.Version == targetVersion {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("version %d not found for agent %s", targetVersion, agentID)
	}

	// Загружаем версию
	key := sm.makeKey(agentID, targetVersion)
	data := sm.cache.Get(ctx, key)
	if data.Err != nil {
		// Пробуем из PG
		var err error
		data.Value, err = sm.loadFromPG(ctx, agentID, targetVersion)
		if err != nil {
			data.Err = fmt.Errorf("load version %d: %w", targetVersion, err)
			return nil, data.Err
		}
	}

	var context AgentContext
	if err := json.Unmarshal(data.Value, &context); err != nil {
		return nil, fmt.Errorf("unmarshal context: %w", err)
	}

	// Сохраняем как новую версию (для аудита)
	if err := sm.Save(ctx, agentID, &context); err != nil {
		return nil, fmt.Errorf("save rollback: %w", err)
	}

	sm.updateStats(start, true, false)

	return &context, nil
}

// GetVersionHistory возвращает историю версий
func (sm *StateManager) GetVersionHistory(agentID string) []VersionEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	versions := sm.versionHistory[agentID]
	result := make([]VersionEntry, len(versions))
	copy(result, versions)

	return result
}

// GetLatestVersion возвращает последнюю версию
func (sm *StateManager) GetLatestVersion(agentID string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	versions := sm.versionHistory[agentID]
	if len(versions) == 0 {
		return 0
	}

	return versions[len(versions)-1].Version
}

// GetStats возвращает статистику StateManager
func (sm *StateManager) GetStats() map[string]interface{} {
	sm.stats.mu.RLock()
	defer sm.stats.mu.RUnlock()

	return map[string]interface{}{
		"total_loads":     sm.stats.TotalLoads,
		"total_saves":     sm.stats.TotalSaves,
		"total_rollbacks": sm.stats.TotalRollbacks,
		"cache_hits":      sm.stats.CacheHits,
		"cache_misses":    sm.stats.CacheMisses,
		"pg_hits":         sm.stats.PGHits,
		"pg_misses":       sm.stats.PGMisses,
		"avg_load_ms":     sm.stats.AvgLoadTime.Milliseconds(),
		"avg_save_ms":     sm.stats.AvgSaveTime.Milliseconds(),
	}
}

// Start запускает периодическую синхронизацию
func (sm *StateManager) Start(ctx context.Context) {
	ticker := time.NewTicker(sm.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.syncToPG(ctx)
			sm.cache.Cleanup()
		}
	}
}

// Helper functions

func (sm *StateManager) makeKey(agentID string, version int) string {
	return fmt.Sprintf("agent:%s:v%d", agentID, version)
}

func (sm *StateManager) makeMetaKey(agentID string) string {
	return fmt.Sprintf("agent:%s:meta", agentID)
}

func (sm *StateManager) updateStats(start time.Time, isSave bool, isCacheHit bool) {
	duration := time.Since(start)

	sm.stats.mu.Lock()
	defer sm.stats.mu.Unlock()

	if isSave {
		sm.stats.TotalSaves++
		sm.stats.AvgSaveTime = (sm.stats.AvgSaveTime + duration) / 2
	} else {
		sm.stats.TotalLoads++
		sm.stats.AvgLoadTime = (sm.stats.AvgLoadTime + duration) / 2

		if isCacheHit {
			sm.stats.CacheHits++
		} else {
			sm.stats.CacheMisses++
		}
	}
}

func (sm *StateManager) archiveToPG(ctx context.Context, agentID string, version int, data []byte) {
	// TODO: реализовать архивирование в PostgreSQL
	_ = ctx
	_ = agentID
	_ = version
	_ = data
}

func (sm *StateManager) loadFromPG(ctx context.Context, agentID string, versions ...int) ([]byte, error) {
	// TODO: реализовать загрузку из PostgreSQL
	_ = ctx
	_ = agentID
	_ = versions
	return nil, fmt.Errorf("PG not implemented")
}

func (sm *StateManager) syncToPG(ctx context.Context) {
	// TODO: реализовать синхронизацию с PostgreSQL
	_ = ctx
}

// AgentMeta метаданные агента
type AgentMeta struct {
	Version   int       `json:"version"`
	Hash      string    `json:"hash"`
	Size      int       `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate валидирует StateManager
func (sm *StateManager) Validate() error {
	if sm.cache == nil {
		return fmt.Errorf("cache is required")
	}

	if sm.config.MaxVersions <= 0 {
		return fmt.Errorf("max versions must be positive")
	}

	return nil
}
