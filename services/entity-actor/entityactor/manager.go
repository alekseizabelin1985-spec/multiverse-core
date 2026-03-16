// services/entityactor/manager.go
package entityactor

import (
	"context"
	"fmt"
	"log"
	"sync"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/intent"
	"multiverse-core.io/shared/redis"
	"multiverse-core.io/shared/rules"
	"multiverse-core.io/shared/tinyml"
)

// Manager управляет жизненным циклом акторов сущностей
type Manager struct {
	mu          sync.RWMutex
	actors      map[string]*Actor
	logger      *log.Logger
	eventBus    *eventbus.EventBus
	minioClient interface{} // TODO: Add proper MinIO client type
	redisClient *redis.Client
	intentCache *intent.IntentCache
	ruleEngine  *rules.Engine
	modelLoader *tinyml.ModelLoader
	config      ActorConfig
}

// NewManager создает новый менеджер акторов сущностей
func NewManager(
	logger *log.Logger,
	bus *eventbus.EventBus,
	minioClient interface{},
	redisClient *redis.Client,
	intentCache *intent.IntentCache,
	ruleEngine *rules.Engine,
	modelLoader *tinyml.ModelLoader,
) *Manager {
	return &Manager{
		actors:      make(map[string]*Actor),
		logger:      logger,
		eventBus:    bus,
		minioClient: minioClient,
		redisClient: redisClient,
		intentCache: intentCache,
		ruleEngine:  ruleEngine,
		modelLoader: modelLoader,
		config:      DefaultActorConfig(),
	}
}

// CreateActor создает новый актор сущности
func (m *Manager) CreateActor(ctx context.Context, id, entityID, entityType, worldID string) (*Actor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Проверяем, что актор с таким ID не существует
	if _, exists := m.actors[id]; exists {
		return nil, fmt.Errorf("actor with id %s already exists", id)
	}

	// Создаем или загружаем модель
	model, err := m.modelLoader.LoadFromStorage(entityID)
	if err != nil {
		// Создаем новую модель
		model, err = tinyml.NewTinyModel(tinyml.ModelConfig{
			Version:      "v1.0",
			InputSize:    10,
			OutputSize:   5,
			HiddenLayers: []int{8},
			ActivationFn: "relu",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create model: %w", err)
		}
	}

	// Создаем актор
	actor := NewActor(
		ctx,
		id,
		entityID,
		entityType,
		worldID,
		model,
		m.ruleEngine,
		m.eventBus,
		m.redisClient,
		m.intentCache,
		m.logger,
		m.config,
	)

	m.actors[id] = actor

	m.logger.Printf("Created actor %s for entity %s (%s)", id, entityID, entityType)
	return actor, nil
}

// DestroyActor уничтожает актор сущности
func (m *Manager) DestroyActor(ctx context.Context, id string) error {
	m.mu.Lock()
	actor, exists := m.actors[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("actor with id %s does not exist", id)
	}

	// Удаляем из менеджера
	delete(m.actors, id)
	m.mu.Unlock()

	// Останавливаем актор
	actor.Stop()

	m.logger.Printf("Destroyed actor %s", id)
	return nil
}

// GetActor возвращает актор по ID
func (m *Manager) GetActor(id string) (*Actor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	actor, exists := m.actors[id]
	if !exists {
		return nil, fmt.Errorf("actor with id %s does not exist", id)
	}

	return actor, nil
}

// GetAllActors возвращает все акторы
func (m *Manager) GetAllActors() []*Actor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	actors := make([]*Actor, 0, len(m.actors))
	for _, actor := range m.actors {
		actors = append(actors, actor)
	}
	return actors
}

// GetActorCount возвращает количество акторов
func (m *Manager) GetActorCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.actors)
}

// GetActorStats возвращает статистику всех акторов
func (m *Manager) GetActorStats() []ActorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make([]ActorStats, 0, len(m.actors))
	for _, actor := range m.actors {
		stats = append(stats, actor.GetStats())
	}
	return stats
}

// Start запускает менеджер акторов
func (m *Manager) Start(ctx context.Context) {
	m.logger.Println("EntityActor Manager started")
}

// Stop останавливает менеджер акторов
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Останавливаем все акторы
	for _, actor := range m.actors {
		actor.Stop()
	}

	m.logger.Println("EntityActor Manager stopped")
}
