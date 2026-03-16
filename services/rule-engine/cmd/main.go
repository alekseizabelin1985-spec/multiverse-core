// cmd/rule-engine/main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"multiverse-core.io/services/rule-engine/ruleengine"
)

func main() {
	// Конфигурация из окружения
	cfg := ruleengine.Config{
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
		log.Println("Shutting down RuleEngine...")
		cancel()
	}()

	// Запуск сервиса
	service, err := ruleengine.NewService(cfg)
	if err != nil {
		log.Fatal("Failed to initialize RuleEngine:", err)
	}

	service.Start(ctx)
	<-ctx.Done()
	service.Stop()
	log.Println("RuleEngine stopped.")
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
