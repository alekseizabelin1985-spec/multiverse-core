// cmd/evolution-watcher/main.go
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"multiverse-core.io/services/evolution-watcher/evolutionwatcher"
)

func main() {
	// Конфигурация из окружения
	cfg := evolutionwatcher.Config{
		KafkaBrokers:   getEnvBrokers("KAFKA_BROKERS", []string{"redpanda:9092"}),
		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "minio:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down EvolutionWatcher...")
		cancel()
	}()

	// Создаем логгер
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Запуск сервиса
	service, err := evolutionwatcher.NewService(cfg, logger)
	if err != nil {
		log.Fatal("Failed to initialize EvolutionWatcher:", err)
	}

	service.Start(ctx)
	<-ctx.Done()
	service.Stop(ctx)
	log.Println("EvolutionWatcher stopped.")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvBrokers(key string, fallback []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		brokers := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				brokers = append(brokers, trimmed)
			}
		}
		if len(brokers) > 0 {
			return brokers
		}
	}
	return fallback
}
