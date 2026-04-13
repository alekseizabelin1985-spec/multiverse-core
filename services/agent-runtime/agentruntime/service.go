// agentruntime/service.go
package agentruntime

import (
	"context"
	"log"
	"time"

	"multiverse-core.io/services/agent-runtime/agentruntime/action"
	"multiverse-core.io/services/agent-runtime/agentruntime/cache"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/jsonpath"
	"multiverse-core.io/shared/minio"
	"multiverse-core.io/shared/oracle"
	"multiverse-core.io/shared/rules"
)

// Config конфигурация сервиса agent-runtime
type Config struct {
	KafkaBrokers   []string
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	OracleURL      string
	OracleModel    string
}

// Service — главная структура сервиса
type Service struct {
	cfg      Config
	bus      *eventbus.EventBus
	resolver *action.Resolver
}

// NewService инициализирует все зависимости и возвращает готовый Service
func NewService(cfg Config) (*Service, error) {
	// Kafka event bus
	bus := eventbus.NewEventBus(cfg.KafkaBrokers)

	// MinIO client для загрузки правил
	minioClient, err := minio.NewMinIOOfficialClient(minio.Config{
		Endpoint:        cfg.MinioEndpoint,
		AccessKeyID:     cfg.MinioAccessKey,
		SecretAccessKey: cfg.MinioSecretKey,
		UseSSL:          false,
	})
	if err != nil {
		return nil, err
	}

	// Oracle (LLM) client
	oracleClient := oracle.NewClient()

	// Rule engine с LRU кэшем (128 правил)
	ruleEngine := rules.NewEngine(minioClient, "rules", 128)

	// Кэши трёх уровней
	promptCache := cache.NewPromptCache(1 * time.Hour)
	resultCache := cache.NewResultCache(30 * time.Second)
	narrativeCache := cache.NewNarrativeCache(5 * time.Minute)

	// Phase1 + Phase2 callers
	phase1 := action.NewPhase1Caller(oracleClient, promptCache, resultCache)
	phase2 := action.NewPhase2Caller(oracleClient, promptCache, narrativeCache)

	// Resolver связывает все компоненты
	resolver := action.NewResolver(ruleEngine, phase1, phase2, bus)

	return &Service{
		cfg:      cfg,
		bus:      bus,
		resolver: resolver,
	}, nil
}

// Start запускает подписку на события и обработку
func (s *Service) Start(ctx context.Context) {
	log.Println("[agent-runtime] service started")
	go s.consumePlayerEvents(ctx)
}

// consumePlayerEvents слушает player_events и вызывает Resolver для action-запросов
func (s *Service) consumePlayerEvents(ctx context.Context) {
	s.bus.Subscribe(ctx, eventbus.TopicPlayerEvents, "agent-runtime", func(event eventbus.Event) {
		if event.Type != "player.cast_action" {
			return
		}

		pa := event.Path()

		ruleID, _ := pa.GetString("action.rule_id")
		attackerID, _ := pa.GetString("entity.id")
		targetID, _ := pa.GetString("target.id")
		worldID := eventbus.GetWorldIDFromEvent(event)

		scopeID := ""
		if scope := eventbus.GetScopeFromEvent(event); scope != nil {
			scopeID = scope.ID
		}

		if ruleID == "" || attackerID == "" || targetID == "" {
			log.Printf("[agent-runtime] skip event %s: missing required fields (rule=%s, attacker=%s, target=%s)",
				event.ID, ruleID, attackerID, targetID)
			return
		}

		attackerStats := extractStats(pa, "attacker.stats")
		targetStats := extractStats(pa, "target.stats")

		req := action.ActionRequest{
			RuleID:        ruleID,
			AttackerID:    attackerID,
			TargetID:      targetID,
			WorldID:       worldID,
			ScopeID:       scopeID,
			TriggerID:     event.ID,
			AttackerStats: attackerStats,
			TargetStats:   targetStats,
		}

		if err := s.resolver.Resolve(ctx, req); err != nil {
			log.Printf("[agent-runtime] resolve error for event %s: %v", event.ID, err)
		}
	})
}

// Stop корректно завершает сервис
func (s *Service) Stop() {
	log.Println("[agent-runtime] stopping...")
}

// extractStats извлекает map[string]float32 из payload по dot-пути
func extractStats(pa *jsonpath.Accessor, prefix string) map[string]float32 {
	raw, ok := pa.GetMap(prefix)
	if !ok {
		return make(map[string]float32)
	}
	stats := make(map[string]float32, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case float64:
			stats[k] = float32(val)
		case float32:
			stats[k] = val
		case int:
			stats[k] = float32(val)
		}
	}
	return stats
}
