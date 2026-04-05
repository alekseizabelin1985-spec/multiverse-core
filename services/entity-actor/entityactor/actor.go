// services/entityactor/actor.go
package entityactor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/intent"
	"multiverse-core.io/shared/redis"
	"multiverse-core.io/shared/rules"
	"multiverse-core.io/shared/tinyml"

	"strings"
)

// Actor представляет собой актор сущности с буферизацией и состоянием
type Actor struct {
	mu sync.RWMutex

	// Идентифичность
	ID         string
	EntityID   string
	EntityType string
	WorldID    string

	// Состояние
	State      map[string]float32
	Model      *tinyml.TinyModel
	RuleEngine *rules.Engine

	// Буферизация событий
	EventBuffer     []BufferedEvent
	BufferMutex     sync.Mutex
	LastProcessTime time.Time
	MaxBufferSize   int
	BufferTimeout   time.Duration

	// Инфраструктура
	EventBus    *eventbus.EventBus
	RedisClient *redis.Client
	IntentCache *intent.IntentCache
	Logger      *log.Logger

	// Персистентность
	SnapshotHistory  []StateSnapshot
	LastSnapshot     time.Time
	SnapshotInterval time.Duration

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// BufferedEvent буферизованное событие
type BufferedEvent struct {
	Event      eventbus.Event
	ReceivedAt time.Time
}

// ActorConfig конфигурация актора
type ActorConfig struct {
	MaxBufferSize    int
	BufferTimeout    time.Duration
	SnapshotInterval time.Duration
	TTL              time.Duration
}

// DefaultActorConfig возвращает конфигурацию по умолчанию
func DefaultActorConfig() ActorConfig {
	return ActorConfig{
		MaxBufferSize:    10,
		BufferTimeout:    5 * time.Second,
		SnapshotInterval: 30 * time.Second,
		TTL:              24 * time.Hour,
	}
}

// NewActor создает новый актор сущности
func NewActor(
	ctx context.Context,
	id, entityID, entityType, worldID string,
	model *tinyml.TinyModel,
	ruleEngine *rules.Engine,
	eventBus *eventbus.EventBus,
	redisClient *redis.Client,
	intentCache *intent.IntentCache,
	logger *log.Logger,
	config ActorConfig,
) *Actor {
	actorCtx, cancel := context.WithCancel(ctx)

	actor := &Actor{
		ID:               id,
		EntityID:         entityID,
		EntityType:       entityType,
		WorldID:          worldID,
		State:            make(map[string]float32),
		Model:            model,
		RuleEngine:       ruleEngine,
		EventBuffer:      make([]BufferedEvent, 0, config.MaxBufferSize),
		MaxBufferSize:    config.MaxBufferSize,
		BufferTimeout:    config.BufferTimeout,
		SnapshotInterval: config.SnapshotInterval,
		EventBus:         eventBus,
		RedisClient:      redisClient,
		IntentCache:      intentCache,
		Logger:           logger,
		SnapshotHistory:  make([]StateSnapshot, 0, 100),
		ctx:              actorCtx,
		cancel:           cancel,
	}

	// Запускаем фоновые процессы
	go actor.bufferProcessor()
	go actor.snapshotScheduler()

	return actor
}

// ProcessEvent обрабатывает событие с буферизацией
func (a *Actor) ProcessEvent(event eventbus.Event) error {
	a.BufferMutex.Lock()
	defer a.BufferMutex.Unlock()

	// Добавляем в буфер
	buffered := BufferedEvent{
		Event:      event,
		ReceivedAt: time.Now(),
	}
	a.EventBuffer = append(a.EventBuffer, buffered)

	a.Logger.Printf("Actor %s: Event buffered (size: %d/%d)", a.ID, len(a.EventBuffer), a.MaxBufferSize)

	// Проверяем не пора ли обработать
	if len(a.EventBuffer) >= a.MaxBufferSize {
		return a.processBufferLocked()
	}

	return nil
}

// processBufferLocked обрабатывает буфер (должен вызываться с захваченным BufferMutex)
func (a *Actor) processBufferLocked() error {
	if len(a.EventBuffer) == 0 {
		return nil
	}

	a.Logger.Printf("Actor %s: Processing buffer of %d events", a.ID, len(a.EventBuffer))

	// Извлекаем события из буфера
	events := make([]eventbus.Event, len(a.EventBuffer))
	for i, be := range a.EventBuffer {
		events[i] = be.Event
	}

	// Очищаем буфер
	a.EventBuffer = a.EventBuffer[:0]
	a.LastProcessTime = time.Now()

	// Обрабатываем пакет событий
	return a.processEventBatch(events)
}

// bufferProcessor фоновый процессор буфера
func (a *Actor) bufferProcessor() {
	ticker := time.NewTicker(a.BufferTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.BufferMutex.Lock()
			if len(a.EventBuffer) > 0 && time.Since(a.LastProcessTime) >= a.BufferTimeout {
				a.processBufferLocked()
			}
			a.BufferMutex.Unlock()
		}
	}
}

// processEventBatch обрабатывает пакет событий
func (a *Actor) processEventBatch(events []eventbus.Event) error {
	// Для каждого события
	for _, event := range events {
		if err := a.processSingleEvent(event); err != nil {
			a.Logger.Printf("Actor %s: Error processing event %s: %v", a.ID, event.ID, err)
		}
	}

	// Сохраняем состояние в Redis
	if err := a.saveToRedis(); err != nil {
		a.Logger.Printf("Actor %s: Failed to save to Redis: %v", a.ID, err)
	}

	return nil
}

// processSingleEvent обрабатывает одно событие
func (a *Actor) processSingleEvent(event eventbus.Event) error {
	// Проверяем тип события
	switch event.Type {
	case "player.action":
		return a.handlePlayerAction(event)
	case "entity.state_changed":
		return a.handleStateChanged(event)
	case "entity.travelled":
		return a.handleTravelled(event)
	default:
		// Для остальных событий просто обновляем состояние
		return a.updateStateFromEvent(event)
	}
}

// handlePlayerAction обрабатывает действие игрока — использует универсальный jsonpath
func (a *Actor) handlePlayerAction(event eventbus.Event) error {
	// Используем универсальный доступ через PathAccessor
	pa := event.Path()

	// Извлечение player_text с поддержкой вложенной структуры:
	// Новая: payload.player_text или payload.action.text
	// Старая: плоский ключ player_text
	playerText, _ := pa.GetString("player_text")
	if playerText == "" {
		playerText, _ = pa.GetString("action.text")
	}

	// Извлечение world_id (новая: payload.world.id / старая: world_id или event.WorldID)
	worldContext := eventbus.GetWorldIDFromEvent(event)
	if worldContext == "" {
		worldContext = a.WorldID
	}

	intentReq := intent.IntentRequest{
		PlayerText:   playerText,
		EntityID:     a.EntityID,
		EntityType:   a.EntityType,
		WorldContext: worldContext,
		State:        a.State,
	}

	// Проверяем кэш
	if cachedIntent, found := a.IntentCache.Get(intentReq.PlayerText, a.EntityID, a.WorldID); found {
		a.Logger.Printf("Actor %s: Intent cache hit for '%s'", a.ID, intentReq.PlayerText)
		return a.applyIntent(cachedIntent, event)
	}

	// В production здесь был бы вызов Oracle
	// Для примера используем заглушку
	recogIntent := &intent.IntentResponse{
		Intent:        "generic_action",
		Confidence:    0.8,
		BaseAction:    "default_action",
		RequiresRoll:  true,
		SuggestedRule: "generic_check",
	}

	// Кэшируем результат
	a.IntentCache.Put(intentReq.PlayerText, a.EntityID, a.WorldID, recogIntent)

	return a.applyIntent(recogIntent, event)
}

// applyIntent применяет распознанное намерение
func (a *Actor) applyIntent(intent *intent.IntentResponse, event eventbus.Event) error {
	// Если нужен бросок кубика
	if intent.RequiresRoll && intent.SuggestedRule != "" {
		result, err := a.RuleEngine.Apply(intent.SuggestedRule, a.State, nil)
		if err != nil {
			return fmt.Errorf("rule application failed: %w", err)
		}

		// Публикуем результат
		return a.publishResult(result, intent)
	}

	// Иначе просто обновляем состояние
	return a.updateStateFromEvent(event)
}

// handleStateChanged обрабатывает изменение состояния — с поддержкой новой иерархической структуры
func (a *Actor) handleStateChanged(event eventbus.Event) error {
	pa := event.Path()

	// Извлекаем state_changes: может быть в payload.state_changes или payload.payload.state_changes
	var changes []interface{}
	if c, ok := pa.GetSlice("state_changes"); ok {
		changes = c
	} else if c, ok := pa.GetSlice("payload.state_changes"); ok {
		changes = c
	} else {
		return nil
	}

	for _, change := range changes {
		changeMap, ok := change.(map[string]interface{})
		if !ok {
			continue
		}

		// Используем универсальное извлечение entity с fallback на старую структуру:
		entity := eventbus.ExtractEntityID(changeMap)
		if entity == nil {
			continue
		}
		entityID := entity.ID
		if entityID != a.EntityID {
			continue
		}

		operations, ok := changeMap["operations"].([]interface{})
		if !ok {
			continue
		}

		for _, op := range operations {
			opMap, ok := op.(map[string]interface{})
			if !ok {
				continue
			}

			path, _ := opMap["path"].(string)
			value, _ := opMap["value"].(float64)
			operation, _ := opMap["op"].(string)

			switch operation {
			case "set":
				a.State[path] = float32(value)
			case "add":
				a.State[path] += float32(value)
			case "subtract":
				a.State[path] -= float32(value)
			case "multiply":
				a.State[path] *= float32(value)
			}
		}
	}

	return nil
}

// handleTravelled обрабатывает путешествие сущности — с поддержкой иерархического world.id
func (a *Actor) handleTravelled(event eventbus.Event) error {
	// Извлекаем world_id: новая структура payload.world.id или старая payload.world_id / event.WorldID
	newWorldID := eventbus.GetWorldIDFromEvent(event)
	if newWorldID == "" {
		return nil
	}

	a.WorldID = newWorldID
	a.Logger.Printf("Actor %s: Travelled to world %s", a.ID, newWorldID)

	return nil
}

// updateStateFromEvent обновляет состояние из события — универсальная версия через jsonpath
func (a *Actor) updateStateFromEvent(event eventbus.Event) error {
	pa := event.Path()

	// Получаем все доступные пути в payload
	paths := pa.GetAllPaths()

	// Обновляем состояние только для числовых значений на верхнем уровне или в payload.*
	for _, path := range paths {
		// Пропускаем служебные иерархические ключи (entity, scope, world, target)
		if isHierarchicalKey(path) {
			continue
		}

		if val, ok := pa.GetFloat(path); ok {
			// Используем последний сегмент пути как ключ состояния (stats.hp -> hp)
			key := getLastPathSegment(path)
			a.State[key] = float32(val)
		}
	}
	return nil
}

// isHierarchicalKey проверяет является ли путь иерархическим служебным ключом
func isHierarchicalKey(path string) bool {
	segments := []string{"entity", "scope", "world", "target", "source"}
	for _, seg := range segments {
		if path == seg || path == seg+".id" || path == seg+".type" {
			return true
		}
	}
	return false
}

// getLastPathSegment возвращает последний сегмент пути (entity.stats.hp -> hp)
func getLastPathSegment(path string) string {
	// Обрабатываем индексы массивов: items[0].name -> name
	if idx := strings.LastIndex(path, "."); idx != -1 {
		return path[idx+1:]
	}
	// Убираем индекс массива если это корневой элемент: items[0] -> items
	if idx := strings.Index(path, "["); idx != -1 {
		return path[:idx]
	}
	return path
}

// publishResult публикует результат применения правила — с иерархической структурой событий
func (a *Actor) publishResult(result *rules.RuleResult, intent *intent.IntentResponse) error {
	// Создаём payload с иерархической структурой через builder
	payload := eventbus.NewEventPayload().
		WithEntity(a.EntityID, a.EntityType, "").
		WithWorld(a.WorldID)

	// Добавляем результат правила и намерение через кастомные поля с dot-notation
	eventbus.SetNested(payload.GetCustom(), "rule_result", result)
	eventbus.SetNested(payload.GetCustom(), "intent", intent.Intent)
	eventbus.SetNested(payload.GetCustom(), "base_action", intent.BaseAction)

	// Если есть целевая сущность — добавляем в иерархической структуре:
	if intent.TargetEntity != "" {
		eventbus.SetNested(payload.GetCustom(), "target.entity.id", intent.TargetEntity)
	}

	// Создаём событие с типобезопасным builder
	publishEvent := eventbus.NewStructuredEvent(
		"entity.action.result",
		"entity-actor",
		a.WorldID,
		payload,
	)

	// ✨ Этап 6: Явные связи для knowledge graph
	if intent.TargetEntity != "" {
		relType := eventbus.RelActedOn
		// Определяем более специфичный тип связи по действию
		action := strings.ToLower(intent.BaseAction)
		if strings.Contains(action, "attack") || strings.Contains(action, "hit") {
			relType = eventbus.RelAttacked
		} else if strings.Contains(action, "talk") || strings.Contains(action, "speak") {
			relType = eventbus.RelTalkedTo
		} else if strings.Contains(action, "find") || strings.Contains(action, "pick") {
			relType = eventbus.RelFound
		} else if strings.Contains(action, "move") || strings.Contains(action, "go") {
			relType = eventbus.RelMovedTo
		}

		publishEvent.Relations = []eventbus.Relation{
			{
				From:     a.EntityID,
				To:       intent.TargetEntity,
				Type:     relType,
				Directed: true,
				Metadata: map[string]any{
					"intent":      intent.Intent,
					"base_action": intent.BaseAction,
				},
			},
		}

		if err := eventbus.ValidateEventRelations(publishEvent); err != nil {
			log.Printf("Invalid relations in entity-actor publishResult: %v", err)
			publishEvent.Relations = nil
		}
	}

	return a.EventBus.Publish(a.ctx, eventbus.TopicWorldEvents, publishEvent)
}

// saveToRedis сохраняет состояние в Redis — с иерархическим world.id
func (a *Actor) saveToRedis() error {
	if a.RedisClient == nil {
		return nil
	}

	state := &redis.EntityActorState{
		EntityID:     a.EntityID,
		EntityType:   a.EntityType,
		State:        a.State,
		ModelVersion: a.Model.GetVersion(),
		LastUpdated:  time.Now(),
		Metadata: map[string]interface{}{
			"world":    map[string]string{"id": a.WorldID}, // иерархическая структура для совместимости с future
			"world_id": a.WorldID,                          // плоский ключ для обратной совместимости
			"actor_id": a.ID,
		},
	}

	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	return a.RedisClient.SetActorState(ctx, state, 24*time.Hour)
}

// loadFromRedis загружает состояние из Redis
func (a *Actor) loadFromRedis() error {
	if a.RedisClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	state, err := a.RedisClient.GetActorState(ctx, a.EntityID)
	if err != nil {
		return err
	}

	a.State = state.State
	a.WorldID = state.Metadata["world_id"].(string)

	return nil
}

// snapshotScheduler фоновый планировщик снапшотов
func (a *Actor) snapshotScheduler() {
	ticker := time.NewTicker(a.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.saveSnapshot()
		}
	}
}

// saveSnapshot сохраняет снапшот состояния
func (a *Actor) saveSnapshot() {
	a.mu.RLock()
	defer a.mu.RUnlock()

	snapshot := StateSnapshot{
		EntityID:     a.EntityID,
		State:        a.State,
		ModelVersion: a.Model.GetVersion(),
		Timestamp:    time.Now(),
	}

	a.SnapshotHistory = append(a.SnapshotHistory, snapshot)

	// Ограничиваем историю
	if len(a.SnapshotHistory) > 100 {
		a.SnapshotHistory = a.SnapshotHistory[len(a.SnapshotHistory)-100:]
	}

	a.LastSnapshot = time.Now()

	a.Logger.Printf("Actor %s: Snapshot saved", a.ID)
}

// GetState возвращает текущее состояние
func (a *Actor) GetState() map[string]float32 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.State
}

// GetStats возвращает статистику актора
func (a *Actor) GetStats() ActorStats {
	a.BufferMutex.Lock()
	bufferSize := len(a.EventBuffer)
	a.BufferMutex.Unlock()

	return ActorStats{
		ActorID:       a.ID,
		EntityID:      a.EntityID,
		StateSize:     len(a.State),
		BufferSize:    bufferSize,
		MaxBufferSize: a.MaxBufferSize,
		LastProcess:   a.LastProcessTime,
		LastSnapshot:  a.LastSnapshot,
		Uptime:        time.Since(a.LastProcessTime),
	}
}

// ActorStats статистика актора
type ActorStats struct {
	ActorID       string        `json:"actor_id"`
	EntityID      string        `json:"entity_id"`
	StateSize     int           `json:"state_size"`
	BufferSize    int           `json:"buffer_size"`
	MaxBufferSize int           `json:"max_buffer_size"`
	LastProcess   time.Time     `json:"last_process"`
	LastSnapshot  time.Time     `json:"last_snapshot"`
	Uptime        time.Duration `json:"uptime"`
}

// Stop останавливает актор
func (a *Actor) Stop() {
	a.cancel()
	a.saveSnapshot()
	a.saveToRedis()
}
