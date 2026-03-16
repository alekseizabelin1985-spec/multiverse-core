package evolutionwatcher

import (
	"context"
	"log/slog"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/intent"
	"multiverse-core.io/shared/minio"
	"multiverse-core.io/shared/redis"
)

// Service структура сервиса Evolution Watcher
type Service struct {
	watcher     *Watcher
	logger      *slog.Logger
	config      Config
	minioClient *minio.MinIOOfficialClient
	redisClient *redis.Client
	intentCache *intent.IntentCache
}

// Config конфигурация сервиса
type Config struct {
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	RedisHost      string
	RedisPort      int
	KafkaBrokers   []string
	WorldID        string
}

// NewService создает новый экземпляр сервиса Evolution Watcher
func NewService(config Config, logger *slog.Logger) (*Service, error) {
	// Инициализация MinIO
	minioClient, err := minio.NewMinIOOfficialClient(minio.Config{
		Endpoint:        config.MinioEndpoint,
		AccessKeyID:     config.MinioAccessKey,
		SecretAccessKey: config.MinioSecretKey,
		UseSSL:          false,
	})
	if err != nil {
		return nil, err
	}

	// Инициализация Redis
	redisClient, err := redis.NewClient(redis.Config{
		Host: config.RedisHost,
		Port: config.RedisPort,
	})
	if err != nil {
		logger.Warn("Failed to initialize Redis", "error", err)
	}

	// Инициализация Event Bus
	eventBus := eventbus.NewEventBus(config.KafkaBrokers)

	// Инициализация Intent Cache
	intentCache := intent.NewIntentCache(24*time.Hour, 10000)

	// Создание Watcher
	watcher := NewWatcher(eventBus, minioClient, redisClient, intentCache, logger, config.WorldID)

	return &Service{
		watcher:     watcher,
		logger:      logger,
		config:      config,
		minioClient: minioClient,
		redisClient: redisClient,
		intentCache: intentCache,
	}, nil
}

// Start запускает сервис Evolution Watcher
func (s *Service) Start(ctx context.Context) error {
	s.logger.Info("Starting Evolution Watcher service")

	// Запускаем Watcher
	if err := s.watcher.Start(ctx); err != nil {
		return err
	}

	s.logger.Info("Evolution Watcher service started successfully")

	return nil
}

// Stop останавливает сервис Evolution Watcher
func (s *Service) Stop(ctx context.Context) error {
	s.logger.Info("Stopping Evolution Watcher service")

	// Останавливаем Watcher
	if err := s.watcher.Stop(ctx); err != nil {
		s.logger.Error("Error stopping watcher", "error", err)
		return err
	}

	s.logger.Info("Evolution Watcher service stopped successfully")
	return nil
}

// GetWatcher возвращает экземпляр Watcher
func (s *Service) GetWatcher() *Watcher {
	return s.watcher
}

// HealthCheck проверяет здоровье сервиса
func (s *Service) HealthCheck(ctx context.Context) error {
	// В реальной реализации здесь будет проверка подключения к MinIO
	// Для примера просто возвращаем nil
	return nil
}

// ServiceManager менеджер сервиса Evolution Watcher
type ServiceManager struct {
	service *Service
	logger  *slog.Logger
	config  Config
}

// NewServiceManager создает новый менеджер сервиса
func NewServiceManager(config Config, logger *slog.Logger) *ServiceManager {
	return &ServiceManager{
		logger: logger,
		config: config,
	}
}

// Start запускает сервис через менеджер
func (sm *ServiceManager) Start(ctx context.Context) error {
	service, err := NewService(sm.config, sm.logger)
	if err != nil {
		return err
	}

	sm.service = service

	return service.Start(ctx)
}

// Stop останавливает сервис через менеджер
func (sm *ServiceManager) Stop(ctx context.Context) error {
	if sm.service != nil {
		return sm.service.Stop(ctx)
	}
	return nil
}

// GetService возвращает экземпляр сервиса
func (sm *ServiceManager) GetService() *Service {
	return sm.service
}

// ServiceStatus статус сервиса
type ServiceStatus struct {
	IsRunning bool      `json:"is_running"`
	StartTime time.Time `json:"start_time,omitempty"`
	Error     error     `json:"error,omitempty"`
}

// GetStatus возвращает статус сервиса
func (sm *ServiceManager) GetStatus() ServiceStatus {
	if sm.service != nil {
		return ServiceStatus{
			IsRunning: true,
			StartTime: time.Now(), // В реальном случае нужно сохранять время запуска
		}
	}

	return ServiceStatus{
		IsRunning: false,
		Error:     nil,
	}
}

// Metrics метрики сервиса
type Metrics struct {
	TotalEventsProcessed int64   `json:"total_events_processed"`
	AnomaliesDetected    int64   `json:"anomalies_detected"`
	ProcessingTime       float64 `json:"processing_time_ms"`
	ErrorCount           int64   `json:"error_count"`
}

// GetMetrics возвращает метрики сервиса
func (sm *ServiceManager) GetMetrics() Metrics {
	// В реальной реализации здесь будут подсчеты метрик
	return Metrics{
		TotalEventsProcessed: 0,
		AnomaliesDetected:    0,
		ProcessingTime:       0,
		ErrorCount:           0,
	}
}

// ServiceConfig конфигурация сервиса для использования в других частях системы
type ServiceConfig struct {
	MinioEndpoint string   `json:"minio_endpoint"`
	KafkaBrokers  []string `json:"kafka_brokers"`
}

// NewServiceConfig создает новую конфигурацию сервиса
func NewServiceConfig(minioEndpoint string, kafkaBrokers []string) ServiceConfig {
	return ServiceConfig{
		MinioEndpoint: minioEndpoint,
		KafkaBrokers:  kafkaBrokers,
	}
}
