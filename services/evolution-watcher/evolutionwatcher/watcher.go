// services/evolutionwatcher/watcher.go
package evolutionwatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/intent"
	"multiverse-core.io/shared/minio"
	"multiverse-core.io/shared/redis"
)

// Watcher отслеживает аномалии в поведении сущностей с иерархической памятью
type Watcher struct {
	mu sync.RWMutex

	// Инфраструктура
	eventBus    *eventbus.EventBus
	minioClient *minio.MinIOOfficialClient
	redisClient *redis.Client
	intentCache *intent.IntentCache
	logger      *slog.Logger

	// Иерархическая память
	shortTermMemory  *ShortTermMemory  // Последние 50 событий (RAM)
	mediumTermMemory *MediumTermMemory // Последние 1000 событий (Redis)
	longTermMemory   *LongTermMemory   // Вся история (MinIO)

	// Модель аномалий
	anomalyModel *AnomalyModel

	// Конфигурация
	shortTermLimit  int
	mediumTermLimit int
	bucketName      string
	worldID         string
}

// ShortTermMemory краткосрочная память в RAM
type ShortTermMemory struct {
	mu          sync.RWMutex
	events      []StoredEvent
	maxSize     int
	entityIndex map[string][]int // Индекс по сущностям
}

// StoredEvent сохраненное событие
type StoredEvent struct {
	EventID      string                 `json:"event_id"`
	EventType    string                 `json:"event_type"`
	EntityID     string                 `json:"entity_id,omitempty"`
	WorldID      string                 `json:"world_id"`
	Timestamp    time.Time              `json:"timestamp"`
	Payload      map[string]interface{} `json:"payload"`
	ReceivedAt   time.Time              `json:"received_at"`
	AnomalyScore float32                `json:"anomaly_score,omitempty"`
}

// NewShortTermMemory создает краткосрочную память
func NewShortTermMemory(maxSize int) *ShortTermMemory {
	return &ShortTermMemory{
		events:      make([]StoredEvent, 0, maxSize),
		maxSize:     maxSize,
		entityIndex: make(map[string][]int),
	}
}

// Add добавляет событие в краткосрочную память
func (m *ShortTermMemory) Add(event StoredEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Добавляем событие
	m.events = append(m.events, event)

	// Индексируем по сущности
	if event.EntityID != "" {
		m.entityIndex[event.EntityID] = append(m.entityIndex[event.EntityID], len(m.events)-1)
	}

	// Удаляем старые если превысили лимит
	if len(m.events) > m.maxSize {
		// Удаляем первое (самое старое) событие
		oldEvent := m.events[0]
		m.events = m.events[1:]

		// Обновляем индекс
		if oldEvent.EntityID != "" {
			if indices, ok := m.entityIndex[oldEvent.EntityID]; ok && len(indices) > 0 {
				m.entityIndex[oldEvent.EntityID] = indices[1:]
			}
		}
	}
}

// GetRecent получает последние события
func (m *ShortTermMemory) GetRecent(count int) []StoredEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if count > len(m.events) {
		count = len(m.events)
	}

	result := make([]StoredEvent, count)
	copy(result, m.events[len(m.events)-count:])
	return result
}

// GetByEntity получает события конкретной сущности
func (m *ShortTermMemory) GetByEntity(entityID string) []StoredEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indices, ok := m.entityIndex[entityID]
	if !ok {
		return nil
	}

	result := make([]StoredEvent, len(indices))
	for i, idx := range indices {
		result[i] = m.events[idx]
	}
	return result
}

// GetAll возвращает все события
func (m *ShortTermMemory) GetAll() []StoredEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]StoredEvent, len(m.events))
	copy(result, m.events)
	return result
}

// MediumTermMemory среднесрочная память в Redis
type MediumTermMemory struct {
	redisClient *redis.Client
	maxSize     int
	ttl         time.Duration
	keyPrefix   string
}

// NewMediumTermMemory создает среднесрочную память
func NewMediumTermMemory(redisClient *redis.Client, maxSize int, ttl time.Duration) *MediumTermMemory {
	return &MediumTermMemory{
		redisClient: redisClient,
		maxSize:     maxSize,
		ttl:         ttl,
		keyPrefix:   "evolution:medium:",
	}
}

// Add добавляет событие в среднесрочную память
func (m *MediumTermMemory) Add(ctx context.Context, event StoredEvent) error {
	key := fmt.Sprintf("%s%s", m.keyPrefix, event.WorldID)

	// Получаем текущий список
	data, err := m.redisClient.Get(ctx, key)
	var events []StoredEvent
	if err == nil {
		json.Unmarshal(data, &events)
	}

	// Добавляем новое событие
	events = append(events, event)

	// Удаляем старые если превысили лимит
	if len(events) > m.maxSize {
		events = events[len(events)-m.maxSize:]
	}

	// Сохраняем обратно
	data, err = json.Marshal(events)
	if err != nil {
		return err
	}

	return m.redisClient.Set(ctx, key, data, m.ttl)
}

// GetRecent получает последние события из Redis
func (m *MediumTermMemory) GetRecent(ctx context.Context, worldID string, count int) ([]StoredEvent, error) {
	key := fmt.Sprintf("%s%s", m.keyPrefix, worldID)

	data, err := m.redisClient.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var events []StoredEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}

	if count > len(events) {
		count = len(events)
	}

	result := make([]StoredEvent, count)
	copy(result, events[len(events)-count:])
	return result, nil
}

// LongTermMemory долгосрочная память в MinIO
type LongTermMemory struct {
	minioClient *minio.MinIOOfficialClient
	bucketName  string
	mu          sync.Mutex
}

// NewLongTermMemory создает долгосрочную память
func NewLongTermMemory(minioClient *minio.MinIOOfficialClient, bucketName string) *LongTermMemory {
	return &LongTermMemory{
		minioClient: minioClient,
		bucketName:  bucketName,
	}
}

// Archive архивирует события в MinIO
func (m *LongTermMemory) Archive(ctx context.Context, worldID string, events []StoredEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Формируем ключ с датой
	date := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("evolution/long/%s/%s.json", worldID, date)

	data, err := json.Marshal(events)
	if err != nil {
		return err
	}

	// В production: m.minioClient.PutObject(...)
	// Для примера просто логируем
	_ = key
	_ = data

	return nil
}

// GetHistory получает историю за период
func (m *LongTermMemory) GetHistory(ctx context.Context, worldID string, from, to time.Time) ([]StoredEvent, error) {
	// В production: загрузка из MinIO за указанный период
	return nil, nil
}

// NewWatcher создает новый Watcher
func NewWatcher(
	eventBus *eventbus.EventBus,
	minioClient *minio.MinIOOfficialClient,
	redisClient *redis.Client,
	intentCache *intent.IntentCache,
	logger *slog.Logger,
	worldID string,
) *Watcher {
	return &Watcher{
		eventBus:         eventBus,
		minioClient:      minioClient,
		redisClient:      redisClient,
		intentCache:      intentCache,
		logger:           logger,
		anomalyModel:     NewAnomalyModel(),
		shortTermMemory:  NewShortTermMemory(50),
		mediumTermMemory: NewMediumTermMemory(redisClient, 1000, 24*time.Hour),
		longTermMemory:   NewLongTermMemory(minioClient, "evolution-archive"),
		shortTermLimit:   50,
		mediumTermLimit:  1000,
		bucketName:       "evolution-archive",
		worldID:          worldID,
	}
}

// Start запускает Watcher
func (w *Watcher) Start(ctx context.Context) error {
	w.logger.Info("Starting Evolution Watcher")

	// Подписываемся на топики
	topics := []string{
		eventbus.TopicPlayerEvents,
		eventbus.TopicWorldEvents,
		eventbus.TopicGameEvents,
		eventbus.TopicSystemEvents,
	}

	for _, topic := range topics {
		go w.eventBus.Subscribe(ctx, topic, "evolution-watcher", w.handleEvent)
		w.logger.Info("Subscribed to topic", "topic", topic)
	}

	// Запускаем фоновые процессы
	go w.periodicArchive()

	w.logger.Info("Evolution Watcher started successfully")
	return nil
}

// Stop останавливает Watcher
func (w *Watcher) Stop(ctx context.Context) error {
	w.logger.Info("Stopping Evolution Watcher")

	// Сохраняем краткосрочную память в среднесрочную
	for _, event := range w.shortTermMemory.GetAll() {
		w.mediumTermMemory.Add(ctx, event)
	}

	return nil
}

// handleEvent обрабатывает событие
func (w *Watcher) handleEvent(event eventbus.Event) {
	eventID := event.ID
	w.logger.Debug("Processing event", "event_id", eventID, "event_type", event.Type)

	// Создаем StoredEvent
	stored := StoredEvent{
		EventID:    eventID,
		EventType:  event.Type,
		WorldID:    eventbus.GetWorldIDFromEvent(event),
		Timestamp:  event.Timestamp,
		Payload:    event.Payload,
		ReceivedAt: time.Now(),
	}

	// Извлекаем entity_id с поддержкой нового формата (entity.id)
	entityInfo := eventbus.ExtractEntityID(event.Payload)
	if entityInfo != nil && entityInfo.ID != "" {
		stored.EntityID = entityInfo.ID
	}

	// Добавляем в иерархическую память
	w.shortTermMemory.Add(stored)

	ctx := context.Background()
	w.mediumTermMemory.Add(ctx, stored)

	// Проверяем на аномалии
	anomalyDetected, anomalyType, severity := w.anomalyModel.CheckAnomaly(stored)
	if anomalyDetected {
		w.logger.Warn("Anomaly detected",
			"event_id", eventID,
			"entity_id", stored.EntityID,
			"anomaly_type", anomalyType,
			"severity", severity)

		// Создаем событие о нарушении
		entityType := ""
		if entityInfo := eventbus.ExtractEntityID(event.Payload); entityInfo != nil {
			entityType = entityInfo.Type
		}
		w.publishViolation(eventID, stored.EntityID, entityType, anomalyType, severity)

		// Отправляем контекст в Oracle для генерации правила
		w.sendToOracle(stored)
	}
}

// publishViolation публикует событие о нарушении в новом формате EntityRef
func (w *Watcher) publishViolation(eventID, entityID, entityType, anomalyType string, severity float32) {
	payload := eventbus.NewEventPayload().
		WithWorld(w.worldID)

	// Entity ссылка в новом формате
	if entityID != "" {
		eType := entityType
		if eType == "" {
			eType = "unknown"
		}
		eventbus.SetNested(payload.GetCustom(), "entity.entity.id", entityID)
		eventbus.SetNested(payload.GetCustom(), "entity.entity.type", eType)
	}

	// Trigger event ссылка
	eventbus.SetNested(payload.GetCustom(), "trigger.entity.id", eventID)
	eventbus.SetNested(payload.GetCustom(), "trigger.entity.type", "event")

	// Мета данные аномалии
	eventbus.SetNested(payload.GetCustom(), "anomaly_type", anomalyType)
	eventbus.SetNested(payload.GetCustom(), "severity", severity)
	eventbus.SetNested(payload.GetCustom(), "timestamp", time.Now().Unix())

	violationEvent := eventbus.NewStructuredEvent(
		"violation.integrity",
		"evolution-watcher",
		w.worldID,
		payload,
	)

	w.eventBus.Publish(context.Background(), eventbus.TopicSystemEvents, violationEvent)
}

// sendToOracle отправляет контекст аномалии в Oracle для генерации правила
func (w *Watcher) sendToOracle(event StoredEvent) {
	// Формируем контекст аномалии
	anomalyCtx := AnomalyContext{
		Event:         event,
		RecentEvents:  w.shortTermMemory.GetRecent(10),
		EntityHistory: w.shortTermMemory.GetByEntity(event.EntityID),
		Timestamp:     time.Now(),
	}

	// В production здесь был бы вызов Oracle для генерации rule proposal
	// Для примера просто логируем
	w.logger.Info("Sending anomaly context to Oracle",
		"event_id", event.EventID,
		"entity_id", event.EntityID)

	// Получаем rule proposal от Oracle
	// ruleProposal := w.callOracle(context)

	// Отправляем на валидацию
	// w.validateRule(ruleProposal)
	_ = anomalyCtx // Используем переменную
}

// periodicArchive периодически архивирует старые события
func (w *Watcher) periodicArchive() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		w.archiveOldEvents()
	}
}

// archiveOldEvents архивирует старые события из среднесрочной памяти в долгосрочную
func (w *Watcher) archiveOldEvents() {
	ctx := context.Background()

	// Получаем события часовой давности
	events, err := w.mediumTermMemory.GetRecent(ctx, w.worldID, 100)
	if err != nil {
		w.logger.Error("Failed to get events for archiving", "error", err)
		return
	}

	// Архивируем
	if err := w.longTermMemory.Archive(ctx, w.worldID, events); err != nil {
		w.logger.Error("Failed to archive events", "error", err)
		return
	}

	w.logger.Info("Archived old events", "count", len(events))
}

// GetStats возвращает статистику Watcher
func (w *Watcher) GetStats() WatcherStats {
	shortTerm := w.shortTermMemory.GetAll()
	mediumCount := 0 // В production: получить из Redis

	return WatcherStats{
		ShortTermCount:  len(shortTerm),
		ShortTermLimit:  w.shortTermLimit,
		MediumTermCount: mediumCount,
		MediumTermLimit: w.mediumTermLimit,
		AnomalyModel:    w.anomalyModel.GetStats(),
	}
}

// WatcherStats статистика Watcher
type WatcherStats struct {
	ShortTermCount  int        `json:"short_term_count"`
	ShortTermLimit  int        `json:"short_term_limit"`
	MediumTermCount int        `json:"medium_term_count"`
	MediumTermLimit int        `json:"medium_term_limit"`
	AnomalyModel    ModelStats `json:"anomaly_model"`
}

// AnomalyContext контекст аномалии для Oracle
type AnomalyContext struct {
	Event         StoredEvent   `json:"event"`
	RecentEvents  []StoredEvent `json:"recent_events"`
	EntityHistory []StoredEvent `json:"entity_history"`
	Timestamp     time.Time     `json:"timestamp"`
}

// ModelStats статистика модели аномалий
type ModelStats struct {
	ChecksCount     int64   `json:"checks_count"`
	AnomaliesFound  int64   `json:"anomalies_found"`
	AverageSeverity float32 `json:"average_severity"`
}
