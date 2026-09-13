// packages/shared/agent/lod.go
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
)

// LODConfig конфигурация адаптивного LOD
type LODConfig struct {
	// Thresholds пороги переключения
	PlayerDensityHigh int // Игроков на км² для перехода на LODBasic
	PlayerDensityMed  int // Игроков на км² для перехода на LODRuleOnly
	PlayerDensityLow  int // Игроков на км² для перехода на LODFull

	// Queue thresholds пороги очереди
	QueueDepthHigh int // Размер очереди для понижения LOD
	QueueDepthMed  int // Средний размер очереди
	QueueDepthLow  int // Низкий размер очереди

	// LLM latency пороги задержки
	LLMLatencyHigh time.Duration // > 500ms — понижаем LOD
	LLMLatencyMed  time.Duration // > 200ms — мониторинг
	LLMLatencyLow  time.Duration // < 100ms — полный LOD

	// Check interval интервал проверки
	CheckInterval time.Duration // Как часто проверять (по умолчанию 10s)

	// Decay time время возврата на высокий LOD
	DecayTime time.Duration // Через какое время вернуть LODFull (по умолчанию 5m)
}

// DefaultLODConfig возвращает конфигурацию по умолчанию
func DefaultLODConfig() LODConfig {
	return LODConfig{
		PlayerDensityHigh: 10,
		PlayerDensityMed:  5,
		PlayerDensityLow:  0,

		QueueDepthHigh: 1000,
		QueueDepthMed:  500,
		QueueDepthLow:  100,

		LLMLatencyHigh: 500 * time.Millisecond,
		LLMLatencyMed:  200 * time.Millisecond,
		LLMLatencyLow:  100 * time.Millisecond,

		CheckInterval: 10 * time.Second,
		DecayTime:     5 * time.Minute,
	}
}

// LODManager управляет динамическим переключением LOD для агентов
type LODManager struct {
	config LODConfig

	// agentLOD хранит текущий LOD для каждого агента
	agentLOD map[string]LODLevel

	// lastHighLOD время последнего высокого LOD для каждого агента
	lastHighLOD map[string]time.Time

	// metrics метрики нагрузки
	metrics *LODMetrics

	// mu защищает concurrent доступ
	mu sync.RWMutex

	// callbacks обратные вызовы при изменении LOD
	callbacks []LODCallback

	// clock and timers come from the caller so that replay and tests drive
	// the time of LOD changes (shared/clock, ADR-001 addendum p. 2).
	clock  clock.Clock
	timers clock.Timers
}

// LODCallback вызывается при изменении LOD
type LODCallback func(agentID string, oldLOD, newLOD LODLevel)

// LODMetrics метрики для LOD
type LODMetrics struct {
	mu               sync.RWMutex
	avgPlayerDensity float64
	avgQueueDepth    float64
	avgLLMLatency    time.Duration
	totalDowngrades  int64
	totalUpgrades    int64
	lastChangeTime   time.Time
}

// NewLODManager создает новый менеджер LOD
func NewLODManager(config LODConfig, clk clock.Clock, timers clock.Timers) *LODManager {
	return &LODManager{
		config:      config,
		agentLOD:    make(map[string]LODLevel),
		lastHighLOD: make(map[string]time.Time),
		metrics:     &LODMetrics{},
		clock:       clk,
		timers:      timers,
	}
}

// SetAgentLOD устанавливает LOD для агента
func (lm *LODManager) SetAgentLOD(agentID string, lod LODLevel) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	oldLOD, exists := lm.agentLOD[agentID]
	if !exists {
		lm.agentLOD[agentID] = lod
		return
	}

	if oldLOD == lod {
		return
	}

	lm.agentLOD[agentID] = lod
	lm.lastHighLOD[agentID] = lm.clock.Now()

	// Обновляем метрики
	lm.metrics.mu.Lock()
	if lod < oldLOD {
		lm.metrics.totalDowngrades++
	} else {
		lm.metrics.totalUpgrades++
	}
	lm.metrics.lastChangeTime = lm.clock.Now()
	lm.metrics.mu.Unlock()

	// Вызываем callbacks
	for _, cb := range lm.callbacks {
		cb(agentID, oldLOD, lod)
	}
}

// GetAgentLOD возвращает текущий LOD агента
func (lm *LODManager) GetAgentLOD(agentID string) LODLevel {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	lod, exists := lm.agentLOD[agentID]
	if !exists {
		return LODFull // По умолчанию полный LOD
	}

	return lod
}

// CalculateNewLOD рассчитывает новый LOD на основе метрик
func (lm *LODManager) CalculateNewLOD(agentID string, playerDensity int, queueDepth int, llmLatency time.Duration) LODLevel {
	// Начинаем с текущего LOD
	currentLOD := lm.GetAgentLOD(agentID)
	newLOD := currentLOD

	// Проверяем player density
	if playerDensity >= lm.config.PlayerDensityHigh {
		newLOD = LODRuleOnly // Максимальная оптимизация
	} else if playerDensity >= lm.config.PlayerDensityMed {
		newLOD = LODBasic // Средняя оптимизация
	} else if playerDensity <= lm.config.PlayerDensityLow && queueDepth < lm.config.QueueDepthLow && llmLatency < lm.config.LLMLatencyLow {
		newLOD = LODFull // Можно полный LOD
	}

	// Проверяем queue depth
	if queueDepth >= lm.config.QueueDepthHigh && newLOD > LODRuleOnly {
		newLOD = LODRuleOnly
	} else if queueDepth >= lm.config.QueueDepthMed && newLOD > LODBasic {
		newLOD = LODBasic
	}

	// Проверяем LLM latency
	if llmLatency >= lm.config.LLMLatencyHigh && newLOD > LODRuleOnly {
		newLOD = LODRuleOnly
	} else if llmLatency >= lm.config.LLMLatencyMed && newLOD > LODBasic {
		newLOD = LODBasic
	}

	// Не понижаем ниже LODRuleOnly и не повышаем выше LODFull
	if newLOD < LODRuleOnly {
		newLOD = LODRuleOnly
	}
	if newLOD > LODFull {
		newLOD = LODFull
	}

	// Если LOD изменился, обновляем
	if newLOD != currentLOD {
		lm.SetAgentLOD(agentID, newLOD)
	}

	return newLOD
}

// AddCallback добавляет обратный вызов при изменении LOD
func (lm *LODManager) AddCallback(cb LODCallback) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.callbacks = append(lm.callbacks, cb)
}

// GetAgentCount возвращает количество отслеживаемых агентов
func (lm *LODManager) GetAgentCount() int {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	return len(lm.agentLOD)
}

// GetStats возвращает статистику LOD
func (lm *LODManager) GetStats() map[string]interface{} {
	lm.mu.RLock()
	agentCount := len(lm.agentLOD)
	lm.mu.RUnlock()

	lm.metrics.mu.RLock()
	stats := map[string]interface{}{
		"total_downgrades": lm.metrics.totalDowngrades,
		"total_upgrades":   lm.metrics.totalUpgrades,
		"last_change":      lm.metrics.lastChangeTime.Format(time.RFC3339),
	}
	lm.metrics.mu.RUnlock()

	// Считаем распределение LOD
	lodDistribution := map[string]int{
		"LODFull":     0,
		"LODBasic":    0,
		"LODRuleOnly": 0,
		"LODDisabled": 0,
	}

	lm.mu.RLock()
	for _, lod := range lm.agentLOD {
		lodDistribution[lod.String()]++
	}
	lm.mu.RUnlock()

	stats["agent_count"] = agentCount
	stats["lod_distribution"] = lodDistribution

	return stats
}

// Start запускает периодическую проверку LOD
func (lm *LODManager) Start(ctx context.Context, getMetrics func() (playerDensity int, queueDepth int, llmLatency time.Duration)) {
	ticker := lm.timers.Every(lm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			playerDensity, queueDepth, llmLatency := getMetrics()

			lm.metrics.mu.Lock()
			lm.metrics.avgPlayerDensity = float64(playerDensity)
			lm.metrics.avgQueueDepth = float64(queueDepth)
			lm.metrics.avgLLMLatency = llmLatency
			lm.metrics.mu.Unlock()

			// Проверяем всех агентов
			lm.mu.RLock()
			agentIDs := make([]string, 0, len(lm.agentLOD))
			for id := range lm.agentLOD {
				agentIDs = append(agentIDs, id)
			}
			lm.mu.RUnlock()

			for _, agentID := range agentIDs {
				lm.CalculateNewLOD(agentID, playerDensity, queueDepth, llmLatency)
			}
		}
	}
}

// DecayLOD возвращает LOD на высокий уровень если нагрузка снизилась
func (lm *LODManager) DecayLOD(agentID string) {
	lm.mu.RLock()
	lastLOD, exists := lm.lastHighLOD[agentID]
	lm.mu.RUnlock()

	if !exists {
		return
	}

	if lm.clock.Now().Sub(lastLOD) > lm.config.DecayTime {
		currentLOD := lm.GetAgentLOD(agentID)
		if currentLOD < LODFull {
			lm.SetAgentLOD(agentID, LODFull)
		}
	}
}

// ResetLOD сбрасывает LOD агента на полный
func (lm *LODManager) ResetLOD(agentID string) {
	lm.SetAgentLOD(agentID, LODFull)
}

// ValidateLOD валидирует LOD
func ValidateLOD(lod LODLevel) error {
	switch lod {
	case LODDisabled, LODRuleOnly, LODBasic, LODFull:
		return nil
	default:
		return fmt.Errorf("invalid LOD level: %d", lod)
	}
}
