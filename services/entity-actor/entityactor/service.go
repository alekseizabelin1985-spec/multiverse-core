// services/entityactor/service.go
package entityactor

import (
	"context"
	"fmt"
	"log"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/intent"
	"multiverse-core.io/shared/minio"
	"multiverse-core.io/shared/redis"
	"multiverse-core.io/shared/rules"
	"multiverse-core.io/shared/tinyml"
)

// Config конфигурация сервиса EntityActor
type Config struct {
	MinioEndpoint   string
	MinioAccessKey  string
	MinioSecretKey  string
	RedisHost       string
	RedisPort       int
	KafkaBrokers    []string
	OracleURL       string
	IntentCacheSize int
	IntentCacheTTL  time.Duration
	RuleCacheSize   int
}

// Service сервис EntityActor
type Service struct {
	minioClient  *minio.MinIOOfficialClient
	redisClient  *redis.Client
	eventBus     *eventbus.EventBus
	modelLoader  *tinyml.ModelLoader
	ruleEngine   *rules.Engine
	intentCache  *intent.IntentCache
	oracleClient *intent.OracleClient
	manager      *Manager
	logger       *log.Logger
	config       Config
}

// NewService создает новый сервис EntityActor
func NewService(cfg Config) (*Service, error) {
	logger := log.New(log.Writer(), "EntityActor: ", log.LstdFlags|log.Lshortfile)

	// Валидация конфигурации
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Инициализация MinIO
	minioClient, err := minio.NewMinIOOfficialClient(minio.Config{
		Endpoint:        cfg.MinioEndpoint,
		AccessKeyID:     cfg.MinioAccessKey,
		SecretAccessKey: cfg.MinioSecretKey,
		UseSSL:          false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MinIO: %w", err)
	}

	// Инициализация Redis
	redisClient, err := redis.NewClient(redis.Config{
		Host: cfg.RedisHost,
		Port: cfg.RedisPort,
	})
	if err != nil {
		logger.Printf("Warning: Failed to initialize Redis: %v", err)
		// Продолжаем без Redis (будет использоваться in-memory)
	}

	// Инициализация Event Bus
	eventBus := eventbus.NewEventBus(cfg.KafkaBrokers)

	// Инициализация Model Loader
	modelLoader := tinyml.NewModelLoader(minioClient, "tinyml-models")

	// Инициализация Rule Engine
	ruleEngine := rules.NewEngine(minioClient, "rules", cfg.RuleCacheSize)

	// Инициализация Intent Cache
	intentCache := intent.NewIntentCache(cfg.IntentCacheTTL, cfg.IntentCacheSize)

	// Инициализация Oracle Client
	oracleClient := intent.NewOracleClient(intent.OracleConfig{
		BaseURL: cfg.OracleURL,
		Model:   "qwen3",
		Timeout: 30 * time.Second,
	})

	// Инициализация Manager
	manager := NewManager(
		log.New(log.Writer(), "EntityActorManager: ", log.LstdFlags|log.Lshortfile),
		eventBus,
		minioClient,
		redisClient,
		intentCache,
		ruleEngine,
		modelLoader,
	)

	service := &Service{
		minioClient:  minioClient,
		redisClient:  redisClient,
		eventBus:     eventBus,
		modelLoader:  modelLoader,
		ruleEngine:   ruleEngine,
		intentCache:  intentCache,
		oracleClient: oracleClient,
		manager:      manager,
		logger:       logger,
		config:       cfg,
	}

	return service, nil
}

// Start запускает сервис
func (s *Service) Start(ctx context.Context) error {
	s.logger.Println("EntityActor service started")

	// Запускаем менеджер
	s.manager.Start(ctx)

	// Подписываемся на топики
	topics := []string{
		eventbus.TopicPlayerEvents,
		eventbus.TopicWorldEvents,
		eventbus.TopicGameEvents,
		eventbus.TopicSystemEvents,
	}

	for _, topic := range topics {
		groupID := fmt.Sprintf("entity-actor-%s-group", topic)
		s.logger.Printf("Subscribing to %s as %s", topic, groupID)

		go func(t string, g string) {
			s.eventBus.Subscribe(ctx, t, g, func(ev eventbus.Event) {
				if ev.Type == "" {
					s.logger.Printf("Warning: Empty event type in %s", t)
					return
				}
				s.handleEvent(ctx, ev)
			})
		}(topic, groupID)
	}

	// Подписываемся на lifecycle события
	go func() {
		s.eventBus.Subscribe(ctx, "entity_actor_events", "entity-actor-lifecycle", func(ev eventbus.Event) {
			s.logger.Printf("Lifecycle event: %s", ev.Type)
		})
	}()

	return nil
}

// handleEvent обрабатывает событие
func (s *Service) handleEvent(ctx context.Context, ev eventbus.Event) {
	if ev.Payload == nil {
		s.logger.Printf("Warning: Empty payload in event %s", ev.Type)
		return
	}

	switch ev.Type {
	// Создание и управление сущностями
	case "entity.created":
		s.handleEntityCreated(ctx, ev)
	case "entity.deleted":
		s.handleEntityDeleted(ctx, ev)
	case "entity.travelled":
		s.handleEntityTravelled(ctx, ev)
	case "entity.state_changed":
		s.handleEntityStateChanged(ctx, ev)
	case "entity.snapshot":
		s.handleEntitySnapshot(ctx, ev)

	// Действия игроков
	case "player.action":
		s.handlePlayerAction(ctx, ev)
	case "player.moved":
		s.handlePlayerMoved(ctx, ev)
	case "player.used_skill":
		s.handlePlayerUsedSkill(ctx, ev)
	case "player.used_item":
		s.handlePlayerUsedItem(ctx, ev)
	case "player.interacted":
		s.handlePlayerInteracted(ctx, ev)

	// Боевые события
	case "combat.started":
		s.handleCombatStarted(ctx, ev)
	case "combat.ended":
		s.handleCombatEnded(ctx, ev)
	case "combat.damage_dealt":
		s.handleCombatDamageDealt(ctx, ev)

	// NPC события
	case "npc.action":
		s.handleNPCAction(ctx, ev)
	case "npc.dialogue":
		s.handleNPCDialogue(ctx, ev)

	// Квесты
	case "quest.started":
		s.handleQuestStarted(ctx, ev)
	case "quest.completed":
		s.handleQuestCompleted(ctx, ev)
	case "quest.updated":
		s.handleQuestUpdated(ctx, ev)

	// Экономика
	case "item.traded":
		s.handleItemTraded(ctx, ev)
	case "item.crafted":
		s.handleItemCrafted(ctx, ev)
	case "currency.changed":
		s.handleCurrencyChanged(ctx, ev)

	// Окружение
	case "world.weather_changed":
		s.handleWeatherChanged(ctx, ev)
	case "world.time_tick":
		s.handleWorldTimeTick(ctx, ev)

	// Entity Actor lifecycle
	case "entity.actor.created":
		s.logger.Printf("Entity actor created: %v", ev.Payload)
	case "entity.actor.destroyed":
		s.logger.Printf("Entity actor destroyed: %v", ev.Payload)
	case "entity.actor.state_saved":
		s.logger.Printf("Entity actor state saved: %v", ev.Payload)

	// Необработанные события - передаем в общий обработчик
	default:
		s.handleGenericEvent(ctx, ev)
	}
}

// handleEntityCreated обрабатывает создание сущности
func (s *Service) handleEntityCreated(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling entity created event")

	// Извлекаем entity с поддержкой новой структуры (entity.entity.id) и fallback
	entityInfo, ok := ev.GetEntityIDWithFallback()
	if !ok || entityInfo == nil {
		s.logger.Printf("Warning: entity missing in entity.created event")
		return
	}

	entityID := entityInfo.ID
	entityType := entityInfo.Type
	worldID := entityInfo.World
	if worldID == "" {
		worldID = eventbus.ExtractWorldID(ev.Payload)
	}
	if worldID == "" {
		worldID = "global"
	}

	// Используем имя из события, если доступно; иначе fallback на ID
	actorName := entityInfo.Name
	if actorName == "" {
		actorName = entityID
	}

	_, err := s.manager.CreateActor(ctx, entityID, actorName, entityType, worldID)
	if err != nil {
		s.logger.Printf("Failed to create actor for entity %s: %v", entityID, err)
	}
}

// handleEntityDeleted обрабатывает удаление сущности
func (s *Service) handleEntityDeleted(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling entity deleted event")

	// Извлекаем entity ID с поддержкой новой структуры
	entityInfo, ok := ev.GetEntityIDWithFallback()
	if !ok || entityInfo == nil || entityInfo.ID == "" {
		s.logger.Printf("Warning: entity_id missing in entity.deleted event")
		return
	}

	// Находим актора по entity_id и уничтожаем
	_, err := s.manager.GetActor(entityInfo.ID)
	if err == nil {
		if err := s.manager.DestroyActor(ctx, entityInfo.ID); err != nil {
			s.logger.Printf("Failed to destroy actor %s: %v", entityInfo.ID, err)
		}
	}
}

// handleEntityTravelled обрабатывает путешествие сущности
func (s *Service) handleEntityTravelled(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling entity travelled event")

	// Обрабатываем entity_snapshots
	pa := ev.Path()
	if snapshots, ok := pa.GetSlice("entity_snapshots"); ok {
		for _, snapshot := range snapshots {
			if snapshotMap, ok := snapshot.(map[string]interface{}); ok {
				entityID, _ := snapshotMap["entity_id"].(string)
				newWorldID, _ := snapshotMap["world_id"].(string)

				if entityID == "" {
					continue
				}

				// Находим актора и обновляем его world_id
				actor, err := s.manager.GetActor(entityID)
				if err == nil && actor != nil {
					s.logger.Printf("Entity %s travelled to world %s", entityID, newWorldID)
					// В production: actor.UpdateWorldID(newWorldID)
				}
			}
		}
	}
}

// handleEntityStateChanged обрабатывает изменение состояния
func (s *Service) handleEntityStateChanged(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling entity state changed event")

	// Извлекаем entity ID с поддержкой новой структуры
	entityInfo, ok := ev.GetEntityIDWithFallback()
	if !ok || entityInfo == nil || entityInfo.ID == "" {
		s.logger.Printf("Warning: entity_id missing in entity.state_changed event")
		return
	}

	pa := ev.Path()
	if changes, ok := pa.GetSlice("state_changes"); ok {
		for _, change := range changes {
			if changeMap, ok := change.(map[string]interface{}); ok {
				entityID, _ := changeMap["entity_id"].(string)
				operations, _ := changeMap["operations"].([]interface{})

				if entityID == "" {
					continue
				}

				// Проверяем наличие актора
				_, err := s.manager.GetActor(entityID)
				if err != nil {
					s.logger.Printf("Actor %s not found for state change", entityID)
					continue
				}

				// Применяем операции к состоянию актора
				for _, op := range operations {
					if opMap, ok := op.(map[string]interface{}); ok {
						path, _ := opMap["path"].(string)
						opType, _ := opMap["op"].(string)

						s.logger.Printf("Applying operation %s to %s for entity %s", opType, path, entityID)
						// В production: actor.ApplyOperation(path, value, opType)
					}
				}
			}
		}
	}
}

// handleEntitySnapshot обрабатывает снимок состояния сущности
func (s *Service) handleEntitySnapshot(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling entity snapshot event")

	pa := ev.Path()
	if snapshots, ok := pa.GetSlice("entity_snapshots"); ok {
		for _, snapshot := range snapshots {
			if snapshotMap, ok := snapshot.(map[string]interface{}); ok {
				entityID, _ := snapshotMap["entity_id"].(string)
				if entityID == "" {
					continue
				}

				s.logger.Printf("Snapshot received for entity %s", entityID)
				// В production: сохранить снимок в хранилище
			}
		}
	}
}

// handlePlayerAction обрабатывает действие игрока
func (s *Service) handlePlayerAction(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling player action event")

	// Извлекаем entity ID с поддержкой новой структуры
	entityInfo, ok := ev.GetEntityIDWithFallback()
	if !ok || entityInfo == nil || entityInfo.ID == "" {
		s.logger.Printf("Warning: entity_id missing in player.action event")
		return
	}
	entityID := entityInfo.ID
	
	pa := ev.Path()
	playerText, _ := pa.GetString("player_text")
	if playerText == "" {
		playerText, _ = pa.GetString("action.text")
	}

	// Проверяем наличие актора
	_, err := s.manager.GetActor(entityID)
	if err != nil {
		s.logger.Printf("Actor %s not found for player action", entityID)
		return
	}

	s.logger.Printf("Player %s action: %s", entityID, playerText)
	// В production: actor.ProcessPlayerAction(playerText)
}

// handlePlayerMoved обрабатывает перемещение игрока
func (s *Service) handlePlayerMoved(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling player moved event")

	// Извлекаем entity ID с поддержкой новой структуры
	entityInfo, ok := ev.GetEntityIDWithFallback()
	var entityID string
	if ok && entityInfo != nil {
		entityID = entityInfo.ID
	}

	pa := ev.Path()
	fromX, _ := pa.GetFloat("from_x")
	fromY, _ := pa.GetFloat("from_y")
	toX, _ := pa.GetFloat("to_x")
	toY, _ := pa.GetFloat("to_y")

	s.logger.Printf("Player %s moved from (%f, %f) to (%f, %f)", entityID, fromX, fromY, toX, toY)
	// В production: обновить позицию актора
}

// handlePlayerUsedSkill обрабатывает использование навыка игроком
func (s *Service) handlePlayerUsedSkill(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling player used skill event")

	// Извлекаем entity ID с поддержкой новой структуры
	entityInfo, ok := ev.GetEntityIDWithFallback()
	var entityID string
	if ok && entityInfo != nil {
		entityID = entityInfo.ID
	}

	pa := ev.Path()
	skillID, _ := pa.GetString("skill_id")

	// Извлекаем target ID с поддержкой новой структуры
	targetInfo, ok := ev.GetTargetEntityID()
	var targetID string
	if ok && targetInfo != nil {
		targetID = targetInfo.ID
	}

	s.logger.Printf("Player %s used skill %s on target %s", entityID, skillID, targetID)
	// В production: применить эффект навыка
}

// handlePlayerUsedItem обрабатывает использование предмета игроком
func (s *Service) handlePlayerUsedItem(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling player used item event")

	// Извлекаем entity ID с поддержкой новой структуры
	entityInfo, ok := ev.GetEntityIDWithFallback()
	var entityID string
	if ok && entityInfo != nil {
		entityID = entityInfo.ID
	}

	pa := ev.Path()
	itemID, _ := pa.GetString("item_id")

	s.logger.Printf("Player %s used item %s", entityID, itemID)
	// В production: применить эффект предмета
}

// handlePlayerInteracted обрабатывает взаимодействие игрока
func (s *Service) handlePlayerInteracted(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling player interacted event")

	// Извлекаем entity ID с поддержкой новой структуры
	entityInfo, ok := ev.GetEntityIDWithFallback()
	var entityID string
	if ok && entityInfo != nil {
		entityID = entityInfo.ID
	}

	// Извлекаем target ID с поддержкой новой структуры
	targetInfo, ok := ev.GetTargetEntityID()
	var targetID string
	if ok && targetInfo != nil {
		targetID = targetInfo.ID
	}

	pa := ev.Path()
	interactionType, _ := pa.GetString("interaction_type")

	s.logger.Printf("Player %s interacted with %s via %s", entityID, targetID, interactionType)
	// В production: обработать взаимодействие
}

// handleCombatStarted обрабатывает начало боя
func (s *Service) handleCombatStarted(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling combat started event")

	pa := ev.Path()
	combatID, _ := pa.GetString("combat_id")
	participants, _ := pa.GetSlice("participants")

	s.logger.Printf("Combat %s started with %d participants", combatID, len(participants))
	// В production: инициализировать боевые состояния акторов
}

// handleCombatEnded обрабатывает окончание боя
func (s *Service) handleCombatEnded(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling combat ended event")

	pa := ev.Path()
	combatID, _ := pa.GetString("combat_id")
	winner, _ := pa.GetString("winner")

	s.logger.Printf("Combat %s ended, winner: %s", combatID, winner)
	// В production: применить награды/штрафы
}

// handleCombatDamageDealt обрабатывает нанесение урона
func (s *Service) handleCombatDamageDealt(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling combat damage dealt event")

	// Извлекаем attacker ID с поддержкой новой структуры
	attackerInfo, ok := ev.GetEntityIDWithFallback()
	var attackerID string
	if ok && attackerInfo != nil {
		attackerID = attackerInfo.ID
	}

	// Извлекаем target ID с поддержкой новой структуры
	targetInfo, ok := ev.GetTargetEntityID()
	var targetID string
	if ok && targetInfo != nil {
		targetID = targetInfo.ID
	}

	pa := ev.Path()
	damage, _ := pa.GetFloat("damage")
	damageType, _ := pa.GetString("damage_type")

	s.logger.Printf("%s dealt %.0f %s damage to %s", attackerID, damage, damageType, targetID)
	// В production: обновить HP цели
}

// handleNPCAction обрабатывает действие NPC
func (s *Service) handleNPCAction(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling NPC action event")

	// Извлекаем npc ID с поддержкой новой структуры
	entityInfo, ok := ev.GetEntityIDWithFallback()
	var npcID string
	if ok && entityInfo != nil {
		npcID = entityInfo.ID
	}

	pa := ev.Path()
	action, _ := pa.GetString("action")

	// Извлекаем target ID с поддержкой новой структуры
	targetInfo, ok := ev.GetTargetEntityID()
	var targetID string
	if ok && targetInfo != nil {
		targetID = targetInfo.ID
	}

	s.logger.Printf("NPC %s performed action %s on %s", npcID, action, targetID)
	// В production: обработать действие NPC
}

// handleNPCDialogue обрабатывает диалог с NPC
func (s *Service) handleNPCDialogue(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling NPC dialogue event")

	pa := ev.Path()
	npcID, _ := pa.GetString("npc_id")
	playerID, _ := pa.GetString("player_id")
	dialogueID, _ := pa.GetString("dialogue_id")

	s.logger.Printf("Dialogue %s between NPC %s and player %s", dialogueID, npcID, playerID)
	// В production: обновить состояние диалога
}

// handleQuestStarted обрабатывает начало квеста
func (s *Service) handleQuestStarted(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling quest started event")

	pa := ev.Path()
	questID, _ := pa.GetString("quest_id")
	playerID, _ := pa.GetString("player_id")

	s.logger.Printf("Quest %s started by player %s", questID, playerID)
	// В production: инициализировать квест
}

// handleQuestCompleted обрабатывает завершение квеста
func (s *Service) handleQuestCompleted(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling quest completed event")

	pa := ev.Path()
	questID, _ := pa.GetString("quest_id")
	playerID, _ := pa.GetString("player_id")
	rewards, _ := pa.GetSlice("rewards")

	s.logger.Printf("Quest %s completed by player %s, rewards: %d", questID, playerID, len(rewards))
	// В production: выдать награды
}

// handleQuestUpdated обрабатывает обновление квеста
func (s *Service) handleQuestUpdated(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling quest updated event")

	pa := ev.Path()
	questID, _ := pa.GetString("quest_id")
	playerID, _ := pa.GetString("player_id")
	objective, _ := pa.GetString("objective")

	s.logger.Printf("Quest %s updated for player %s, objective: %s", questID, playerID, objective)
	// В production: обновить прогресс квеста
}

// handleItemTraded обрабатывает торговлю предметом
func (s *Service) handleItemTraded(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling item traded event")

	pa := ev.Path()
	fromID, _ := pa.GetString("from_id")
	toID, _ := pa.GetString("to_id")
	itemID, _ := pa.GetString("item_id")
	amount, _ := pa.GetFloat("amount")

	s.logger.Printf("Item %s (%.0f) traded from %s to %s", itemID, amount, fromID, toID)
	// В production: передать предмет
}

// handleItemCrafted обрабатывает создание предмета
func (s *Service) handleItemCrafted(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling item crafted event")

	pa := ev.Path()
	crafterID, _ := pa.GetString("crafter_id")
	itemID, _ := pa.GetString("item_id")
	recipeID, _ := pa.GetString("recipe_id")
	quality, _ := pa.GetFloat("quality")

	s.logger.Printf("Item %s crafted by %s using recipe %s, quality: %.2f", itemID, crafterID, recipeID, quality)
	// В production: создать предмет
}

// handleCurrencyChanged обрабатывает изменение валюты
func (s *Service) handleCurrencyChanged(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling currency changed event")

	entityInfo, ok := ev.GetEntityIDWithFallback()
	if !ok || entityInfo == nil {
		s.logger.Printf("Entity not found in currency changed event")
		return
	}
	entityID := entityInfo.ID

	pa := ev.Path()
	currencyType, _ := pa.GetString("currency_type")
	amount, _ := pa.GetFloat("amount")
	reason, _ := pa.GetString("reason")

	s.logger.Printf("Currency %s changed by %.2f for %s, reason: %s", currencyType, amount, entityID, reason)
	// В production: обновить баланс
}

// handleWeatherChanged обрабатывает изменение погоды
func (s *Service) handleWeatherChanged(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling weather changed event")

	pa := ev.Path()
	worldID, _ := pa.GetString("world_id")
	weatherType, _ := pa.GetString("weather_type")
	intensity, _ := pa.GetFloat("intensity")

	s.logger.Printf("Weather in world %s changed to %s with intensity %.2f", worldID, weatherType, intensity)
	// В production: применить эффекты погоды к акторам
}

// handleWorldTimeTick обрабатывает тик времени в мире
func (s *Service) handleWorldTimeTick(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling world time tick event")

	pa := ev.Path()
	worldID, _ := pa.GetString("world_id")
	timestamp, _ := pa.GetFloat("timestamp")
	dayPhase, _ := pa.GetString("day_phase")

	s.logger.Printf("Time tick in world %s: %.0f, phase: %s", worldID, timestamp, dayPhase)
	// В production: обновить время для акторов
}

// handleGenericEvent обрабатывает необработанные события
func (s *Service) handleGenericEvent(ctx context.Context, ev eventbus.Event) {
	s.logger.Printf("Handling generic event: %s", ev.Type)

	// Извлекаем entity_id если есть через новый helper
	entityInfo, ok := ev.GetEntityIDWithFallback()
	if ok && entityInfo != nil {
		// Пропускаем не-actor сущности (миры, регионы, предметы и т.д.)
		switch entityInfo.Type {
		case "world", "region", "location", "city", "item", "quest", "scope":
			return
		}

		entityID := entityInfo.ID
		// Проверяем есть ли актор для этой сущности
		actor, err := s.manager.GetActor(entityID)
		if err == nil && actor != nil {
			s.logger.Printf("Found actor %s for event %s", entityID, ev.Type)
			// В production: передать событие актору для обработки
			// actor.ProcessEvent(ev)
		}
	}

	// Логируем payload для отладки
	s.logger.Printf("Event payload: %v", ev.Payload)
}

// Stop останавливает сервис
func (s *Service) Stop() error {
	s.logger.Println("Stopping EntityActor service")

	// Останавливаем менеджер
	s.manager.Stop()

	// Закрываем event bus
	if s.eventBus != nil {
		s.eventBus.Close()
	}

	// Закрываем Redis
	if s.redisClient != nil {
		s.redisClient.Close()
	}

	s.logger.Println("EntityActor service stopped")
	return nil
}

// GetManager возвращает менеджер акторов
func (s *Service) GetManager() *Manager {
	return s.manager
}

// validateConfig валидирует конфигурацию
func validateConfig(cfg Config) error {
	if cfg.MinioEndpoint == "" {
		return fmt.Errorf("MinIO endpoint cannot be empty")
	}
	if cfg.MinioAccessKey == "" {
		return fmt.Errorf("MinIO access key cannot be empty")
	}
	if cfg.MinioSecretKey == "" {
		return fmt.Errorf("MinIO secret key cannot be empty")
	}
	if len(cfg.KafkaBrokers) == 0 {
		return fmt.Errorf("Kafka brokers list cannot be empty")
	}
	return nil
}
