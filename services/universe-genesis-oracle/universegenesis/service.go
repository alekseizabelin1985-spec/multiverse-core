// services/universegenesis/service.go
package universegenesis

import (
	"context"
	"log"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/oracle" // <-- Импорт общего клиента

	"github.com/google/uuid"
)

type Service struct {
	bus       *eventbus.EventBus
	archivist *ArchivistClient
	oracle    *oracle.Client // <-- Изменён тип
	generator *Generator
}

func NewService(bus *eventbus.EventBus, archivist *ArchivistClient) *Service { // <-- Принимаем URL, а не готовый клиент
	oracleClient := oracle.NewClient() // <-- Создаём общий клиент
	return &Service{
		bus:       bus,
		archivist: archivist,
		oracle:    oracleClient,                               // <-- Передаём общий клиент
		generator: NewGenerator(bus, archivist, oracleClient), // <-- Передаём общий клиент
	}
}

func (s *Service) Run(ctx context.Context) error {
	log.Println("UniverseGenesisOracle starting and waiting for genesis requests...")

	// Подписка на системный топик для получения запросов на генерацию вселенной
	s.bus.Subscribe(ctx, eventbus.TopicSystemEvents, "universe-genesis-oracle", func(event eventbus.Event) {
		s.handleSystemEvent(ctx, event)
	})

	// Сервис работает постоянно, обрабатывая события
	<-ctx.Done()
	log.Println("UniverseGenesisOracle shutting down...")
	return nil
}

// handleSystemEvent обрабатывает события из системного топика
func (s *Service) handleSystemEvent(ctx context.Context, event eventbus.Event) {
	log.Printf("Received system event: %s from source: %s", event.Type, event.Source)

	// Обработка запроса на генерацию вселенной
	if event.Type == "universe.genesis.request" {
		seed := eventbus.GetWorldIDFromEvent(event) // Используем WorldID как seed для простоты
		if seed == "" {
			seed = uuid.New().String()
		}

		constraints, ok := event.Payload["constraints"].([]string)
		if !ok {
			constraints = []string{}
		}

		log.Printf("Processing universe genesis request with seed: %s", seed)
		if err := s.generator.StartGenesis(ctx, seed, constraints); err != nil {
			log.Printf("Universe Genesis failed: %v", err)
			// Можно опубликовать событие об ошибке
		} else {
			log.Printf("Universe Genesis completed for seed: %s", seed)
		}
	}
}
