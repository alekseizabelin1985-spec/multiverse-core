// services/ruleengine/service.go
package ruleengine

import (
	"context"
	"log"

	"multiverse-core/internal/eventbus"
	"multiverse-core/internal/minio"
)

// Config represents the configuration for the RuleEngine service
type Config struct {
	KafkaBrokers   []string
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
}

// Service represents the RuleEngine service
type Service struct {
	engine *Engine
	bus    *eventbus.EventBus
	logger *log.Logger
}

// NewService creates a new RuleEngine service instance
func NewService(cfg Config) (*Service, error) {
	// Create MinIO client using HTTP client approach (matching engine.go expectations)
	minioCfg := minio.Config{
		Endpoint:        cfg.MinioEndpoint,
		AccessKeyID:     cfg.MinioAccessKey,
		SecretAccessKey: cfg.MinioSecretKey,
		UseSSL:          false,
	}

	// Create MinIO client directly (matching the type expected by engine.go)
	minioClient, err := minio.NewClientHTTP(minioCfg)
	if err != nil {
		return nil, err
	}

	// Create EventBus
	bus := eventbus.NewEventBus(cfg.KafkaBrokers)

	// Create RuleEngine instance
	engine := NewEngine(bus, minioClient)

	return &Service{
		engine: engine,
		bus:    bus,
		logger: log.New(log.Writer(), "RuleEngineService: ", log.LstdFlags|log.Lshortfile),
	}, nil
}

// Start starts the RuleEngine service
func (s *Service) Start(ctx context.Context) {
	s.logger.Println("RuleEngine service started")

	// Subscribe to rule application events
	go s.bus.Subscribe(ctx, "system_events", "rule-engine-group", func(ev eventbus.Event) {
		if ev.EventType == "rule.apply" {
			// Handle rule application event
			// This would typically involve applying rules to entities
			s.logger.Printf("Received rule application event: %s", ev.EventType)
		}
	})
}

// Stop stops the RuleEngine service
func (s *Service) Stop() {
	s.bus.Close()
	s.logger.Println("RuleEngine service stopped")
}
