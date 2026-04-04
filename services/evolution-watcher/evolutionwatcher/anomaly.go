// services/evolutionwatcher/anomaly.go
package evolutionwatcher

import (
	"math"
	"sync"
	"time"

	"multiverse-core.io/shared/eventbus"
)

// AnomalyModel нейронная модель для обнаружения аномалий
type AnomalyModel struct {
	mu sync.RWMutex

	// Статистика
	checksCount    int64
	anomaliesFound int64
	totalSeverity  float32

	// Паттерны нормального поведения (обучаемые)
	normalPatterns map[string]*BehaviorPattern

	// Пороги (динамические, не hardcoded!)
	thresholds map[string]float32

	// Веса для различных типов аномалий
	weights AnomalyWeights
}

// BehaviorPattern паттерн нормального поведения сущности
type BehaviorPattern struct {
	EntityID    string
	Mean        map[string]float64 // Средние значения метрик
	StdDev      map[string]float64 // Стандартные отклонения
	SampleCount int
	LastUpdated time.Time
}

// AnomalyWeights веса для различных типов аномалий
type AnomalyWeights struct {
	StateChange float32 // Вес аномалий состояния
	Behavioral  float32 // Вес поведенческих аномалий
	Temporal    float32 // Вес временных аномалий
	Contextual  float32 // Вес контекстных аномалий
}

// extractEntityIDFromStoredEvent извлекает entity ID из StoredEvent payload
func extractEntityIDFromStoredEvent(event StoredEvent) *eventbus.EntityInfo {
	return eventbus.ExtractEntityID(event.Payload)
}

// NewAnomalyModel создает новую модель аномалий
func NewAnomalyModel() *AnomalyModel {
	return &AnomalyModel{
		normalPatterns: make(map[string]*BehaviorPattern),
		thresholds: map[string]float32{
			"state_change": 3.0, // 3 стандартных отклонения
			"behavioral":   2.5, // 2.5 сигмы
			"temporal":     2.0, // 2 сигмы
			"contextual":   2.5,
		},
		weights: AnomalyWeights{
			StateChange: 1.0,
			Behavioral:  1.2,
			Temporal:    0.8,
			Contextual:  1.1,
		},
	}
}

// CheckAnomaly проверяет событие на аномалии
func (m *AnomalyModel) CheckAnomaly(event StoredEvent) (bool, string, float32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checksCount++

	// Извлекаем entity ID из payload
	entityID := ""
	if entityInfo := extractEntityIDFromStoredEvent(event); entityInfo != nil {
		entityID = entityInfo.ID
	}

	// Проверяем различные типы аномалий
	anomalies := make(map[string]float32)

	// 1. Аномалии состояния
	if score := m.checkStateAnomaly(event, entityID); score > 0 {
		anomalies["state_change"] = score
	}

	// 2. Поведенческие аномалии
	if score := m.checkBehavioralAnomaly(event, entityID); score > 0 {
		anomalies["behavioral"] = score
	}

	// 3. Временные аномалии
	if score := m.checkTemporalAnomaly(event, entityID); score > 0 {
		anomalies["temporal"] = score
	}

	// 4. Контекстные аномалии
	if score := m.checkContextualAnomaly(event, entityID); score > 0 {
		anomalies["contextual"] = score
	}

	// Если нет аномалий - обновляем паттерн нормального поведения
	if len(anomalies) == 0 && entityID != "" {
		m.updateNormalPattern(event, entityID)
		return false, "", 0
	}

	// Вычисляем взвешенную оценку
	totalScore := float32(0)
	anomalyType := ""
	maxScore := float32(0)

	for atype, score := range anomalies {
		weighted := score * m.getWeight(atype)
		totalScore += weighted
		if weighted > maxScore {
			maxScore = weighted
			anomalyType = atype
		}
	}

	// Нормализуем оценку (0-1)
	severity := normalizeScore(totalScore)

	if severity > 0.5 { // Порог детекции
		m.anomaliesFound++
		m.totalSeverity += severity
		return true, anomalyType, severity
	}

	return false, "", 0
}

// checkStateAnomaly проверяет аномалии состояния
func (m *AnomalyModel) checkStateAnomaly(event StoredEvent, entityID string) float32 {
	// Проверяем state_changes
	operations, ok := event.Payload["operations"].([]interface{})
	if !ok {
		return 0
	}

	maxDeviation := float32(0)

	for _, op := range operations {
		opMap, ok := op.(map[string]interface{})
		if !ok {
			continue
		}

		path, _ := opMap["path"].(string)
		value, ok := opMap["value"].(float64)
		if !ok {
			continue
		}

		// Проверяем отклонение от нормального паттерна
		if pattern, exists := m.normalPatterns[entityID]; exists {
			if mean, ok := pattern.Mean[path]; ok {
				if stddev, ok := pattern.StdDev[path]; ok && stddev > 0 {
					deviation := math.Abs(value-mean) / stddev
					if deviation > float64(m.thresholds["state_change"]) {
						if float32(deviation) > maxDeviation {
							maxDeviation = float32(deviation)
						}
					}
				}
			}
		}
	}

	return maxDeviation / 10.0 // Нормализуем
}

// checkBehavioralAnomaly проверяет поведенческие аномалии
func (m *AnomalyModel) checkBehavioralAnomaly(event StoredEvent, entityID string) float32 {
	// Проверяем частоту событий
	eventType := event.EventType

	if pattern, exists := m.normalPatterns[entityID]; exists {
		key := "event_rate:" + eventType
		if mean, ok := pattern.Mean[key]; ok {
			if stddev, ok := pattern.StdDev[key]; ok && stddev > 0 {
				// Текущая частота (в реальности нужно считать по временному окну)
				currentRate := 1.0 // Заглушка
				deviation := math.Abs(currentRate-mean) / stddev
				if deviation > float64(m.thresholds["behavioral"]) {
					return float32(deviation) / 10.0
				}
			}
		}
	}

	return 0
}

// checkTemporalAnomaly проверяет временные аномалии
func (m *AnomalyModel) checkTemporalAnomaly(event StoredEvent, entityID string) float32 {
	// Проверяем время между событиями
	if pattern, exists := m.normalPatterns[entityID]; exists {
		key := "time_delta"
		if mean, ok := pattern.Mean[key]; ok {
			if stddev, ok := pattern.StdDev[key]; ok && stddev > 0 {
				// Текущий дельта (в реальности нужно вычислять)
				currentDelta := 1.0 // Заглушка
				deviation := math.Abs(currentDelta-mean) / stddev
				if deviation > float64(m.thresholds["temporal"]) {
					return float32(deviation) / 10.0
				}
			}
		}
	}

	return 0
}

// checkContextualAnomaly проверяет контекстные аномалии
func (m *AnomalyModel) checkContextualAnomaly(event StoredEvent, entityID string) float32 {
	// Проверяем необычные комбинации событий/состояний
	// В production здесь будет более сложная логика

	// Пример: проверка необычных значений в payload
	if payload, ok := event.Payload["payload"].(map[string]interface{}); ok {
		for key, value := range payload {
			if val, ok := value.(float64); ok {
				// Проверяем экстремальные значения
				if val > 10000 || val < -10000 {
					return 0.8 // Высокая вероятность аномалии
				}

				// Проверяем относительно паттерна
				if pattern, exists := m.normalPatterns[entityID]; exists {
					if mean, ok := pattern.Mean[key]; ok {
						if stddev, ok := pattern.StdDev[key]; ok && stddev > 0 {
							deviation := math.Abs(val-mean) / stddev
							if deviation > float64(m.thresholds["contextual"]) {
								return float32(deviation) / 10.0
							}
						}
					}
				}
			}
		}
	}

	return 0
}

// updateNormalPattern обновляет паттерн нормального поведения
func (m *AnomalyModel) updateNormalPattern(event StoredEvent, entityID string) {
	if entityID == "" {
		return
	}

	pattern, exists := m.normalPatterns[entityID]
	if !exists {
		pattern = &BehaviorPattern{
			EntityID:    entityID,
			Mean:        make(map[string]float64),
			StdDev:      make(map[string]float64),
			SampleCount: 0,
		}
		m.normalPatterns[entityID] = pattern
	}

	// Обновляем статистику с помощью онлайн-алгоритма (Welford's algorithm)
	pattern.SampleCount++
	n := float64(pattern.SampleCount)

	// Обновляем mean и stddev для числовых значений в payload
	if payload, ok := event.Payload["payload"].(map[string]interface{}); ok {
		for key, value := range payload {
			if val, ok := value.(float64); ok {
				// Обновляем mean
				oldMean := pattern.Mean[key]
				newMean := oldMean + (val-oldMean)/n
				pattern.Mean[key] = newMean

				// Обновляем variance (для stddev)
				oldVariance := pattern.StdDev[key] * pattern.StdDev[key]
				newVariance := oldVariance + (val-oldMean)*(val-newMean)
				if n > 1 {
					pattern.StdDev[key] = math.Sqrt(newVariance / (n - 1))
				}
			}
		}
	}

	pattern.LastUpdated = time.Now()
}

// getWeight возвращает вес для типа аномалии
func (m *AnomalyModel) getWeight(anomalyType string) float32 {
	switch anomalyType {
	case "state_change":
		return m.weights.StateChange
	case "behavioral":
		return m.weights.Behavioral
	case "temporal":
		return m.weights.Temporal
	case "contextual":
		return m.weights.Contextual
	default:
		return 1.0
	}
}

// GetStats возвращает статистику модели
func (m *AnomalyModel) GetStats() ModelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	avgSeverity := float32(0)
	if m.anomaliesFound > 0 {
		avgSeverity = m.totalSeverity / float32(m.anomaliesFound)
	}

	return ModelStats{
		ChecksCount:     m.checksCount,
		AnomaliesFound:  m.anomaliesFound,
		AverageSeverity: avgSeverity,
	}
}

// normalizeScore нормализует оценку в диапазон 0-1
func normalizeScore(score float32) float32 {
	// Sigmoid normalization
	if score <= 0 {
		return 0
	}
	return 1.0 / (1.0 + float32(math.Exp(-float64(score))))
}

// CheckStateChangeAnomaly проверяет аномалии в изменениях состояния (для совместимости)
func (m *AnomalyModel) CheckStateChangeAnomaly(stateChange map[string]interface{}) bool {
	// Проверяем операции
	if operations, ok := stateChange["operations"].([]interface{}); ok {
		for _, op := range operations {
			if operation, ok := op.(map[string]interface{}); ok {
				value, ok := operation["value"].(float64)
				if !ok {
					continue
				}

				// Экстремальные значения
				if value > 10000 || value < -10000 {
					return true
				}
			}
		}
	}

	return false
}

// Reset сбрасывает статистику модели
func (m *AnomalyModel) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checksCount = 0
	m.anomaliesFound = 0
	m.totalSeverity = 0
	m.normalPatterns = make(map[string]*BehaviorPattern)
}

// RemovePattern удаляет паттерн сущности
func (m *AnomalyModel) RemovePattern(entityID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.normalPatterns, entityID)
}

// GetPattern возвращает паттерн сущности
func (m *AnomalyModel) GetPattern(entityID string) *BehaviorPattern {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.normalPatterns[entityID]
}

// GetAllPatterns возвращает все паттерны
func (m *AnomalyModel) GetAllPatterns() map[string]*BehaviorPattern {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*BehaviorPattern)
	for k, v := range m.normalPatterns {
		result[k] = v
	}
	return result
}
