// services/narrativeorchestrator/service.go

package narrativeorchestrator

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"multiverse-core.io/shared/eventbus"
)

type Config struct {
	KafkaBrokers []string
}

type Service struct {
	orchestrator *NarrativeOrchestrator
	bus          *eventbus.EventBus
	defaultWorldID string
}

func NewService(cfg Config) (*Service, error) {
	bus := eventbus.NewEventBus(cfg.KafkaBrokers)
	orchestrator := NewNarrativeOrchestrator(bus)

	return &Service{
		orchestrator: orchestrator,
		bus:          bus,
		defaultWorldID: "pain-realm", // Default world for timer events
	}, nil
}

func (s *Service) Start(ctx context.Context) {
	log.Println("NarrativeOrchestrator started")

	// Запускаем таймер для periodic time.syncTime событий (default: every 5 seconds)
	go s.startTimerTicker(ctx)

	// Системные события: gm.*, time.syncTime
	go s.bus.Subscribe(ctx, eventbus.TopicSystemEvents, "narrative-scope-group", func(ev eventbus.Event) {
		switch ev.EventType {
		case "gm.created":
			s.orchestrator.CreateGM(ev)
		case "gm.deleted":
			s.orchestrator.DeleteGM(ev)
		case "gm.merged":
			s.orchestrator.MergeGM(ev)
		case "gm.split":
			s.orchestrator.SplitGM(ev)
		case "time.syncTime":
			s.orchestrator.HandleTimerEvent(ev)
		}
	})

	// Игровые события
	go s.bus.Subscribe(ctx, eventbus.TopicWorldEvents, "narrative-world-group", s.orchestrator.HandleGameEvent)
	go s.bus.Subscribe(ctx, eventbus.TopicGameEvents, "narrative-game-group", s.orchestrator.HandleGameEvent)

	// NEW: Mechanical results from Entity-Actors
	go s.bus.Subscribe(ctx, "mechanical_results", "narrative-mechanical-group", func(ev eventbus.Event) {
		s.orchestrator.HandleMechanicalResult(ev)
	})
}

func (s *Service) startTimerTicker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Println("Timer ticker started for time.syncTime events")

	for {
		select {
		case <-ctx.Done():
			log.Println("Timer ticker stopped")
			return
		case <-ticker.C:
			ev := eventbus.Event{
				EventID:   "timer-" + uuid.NewString()[:8],
				EventType: "time.syncTime",
				Source:    "narrative-orchestrator",
				WorldID:   s.defaultWorldID,
				Timestamp: time.Now().UTC(),
				Payload: map[string]interface{}{
					"current_time_unix_ms": time.Now().UnixMilli(),
				},
			}

			// Publish timer event to default world
			if err := s.bus.PublishSystemEvent(ctx, ev); err != nil {
				log.Printf("Failed to publish time.syncTime: %v", err)
			} else {
				log.Printf("Published time.syncTime event: %s for world %s", ev.EventID, s.defaultWorldID)
			}
		}
	}
}

func (s *Service) Stop() {
	s.bus.Close()
}
