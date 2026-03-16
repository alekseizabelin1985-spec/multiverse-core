# Project variables
PROJECT_NAME := multiverse-core
DOCKER_IMAGE := $(PROJECT_NAME)

# Services
SERVICES := \
	entity-manager \
	narrative-orchestrator \
	world-generator \
	ban-of-world \
	city-governor \
	cultivation-module \
	reality-monitor \
	plan-manager \
	semantic-memory \
	ontological-archivist \
	entity-actor \
	evolution-watcher \
	rule-engine \
	universe-genesis-oracle \
	game-service

# Default target
.PHONY: all
all: build

# Build all services via Docker
.PHONY: build
build:
	@echo "Building all services..."
	@docker build -t $(DOCKER_IMAGE) -f build/Dockerfile .

# Build specific service locally (make build-service SERVICE=entity-manager)
.PHONY: build-service
build-service:
	@echo "Building service: $(SERVICE)..."
	@cd services/$(SERVICE) && CGO_ENABLED=0 GOOS=linux go build -o ../../bin/$(SERVICE) ./cmd/

# Build all services locally
.PHONY: build-all
build-all:
	@echo "Building all services locally..."
	@mkdir -p bin
	@for svc in $(SERVICES); do \
		echo "  Building $$svc..."; \
		cd services/$$svc && CGO_ENABLED=0 GOOS=linux go build -o ../../bin/$$svc ./cmd/ && cd ../..; \
	done

# Run services
.PHONY: up
up:
	@echo "Starting all services..."
	@docker-compose up -d

# Run specific service (make run SERVICE=entity-manager)
.PHONY: run
run:
	@echo "Starting service: $(SERVICE)..."
	@docker-compose up -d $(SERVICE)

# Stop services
.PHONY: down
down:
	@echo "Stopping all services..."
	@docker-compose down

# Clean
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@docker system prune -f

# Logs
.PHONY: logs
logs:
	@docker-compose logs -f

# Logs for specific service
.PHONY: logs-service
logs-service:
	@docker-compose logs -f $(SERVICE)

# Test all modules
.PHONY: test
test:
	@echo "Running tests..."
	@go test ./shared/...
	@for svc in $(SERVICES); do \
		echo "  Testing $$svc..."; \
		cd services/$$svc && go test ./... && cd ../..; \
	done

# Test specific service
.PHONY: test-service
test-service:
	@echo "Testing service: $(SERVICE)..."
	@cd services/$(SERVICE) && go test ./...

# Test shared module
.PHONY: test-shared
test-shared:
	@echo "Testing shared module..."
	@cd shared && go test ./...

# Sync workspace
.PHONY: sync
sync:
	@echo "Syncing workspace..."
	@go work sync

# Help
.PHONY: help
help:
	@echo "Makefile for $(PROJECT_NAME)"
	@echo ""
	@echo "Usage:"
	@echo "  make build                      Build all services (Docker)"
	@echo "  make build-service SERVICE=<n>  Build specific service locally"
	@echo "  make build-all                  Build all services locally"
	@echo "  make up                         Start all services"
	@echo "  make run SERVICE=<name>         Start specific service"
	@echo "  make down                       Stop all services"
	@echo "  make logs                       Show logs for all services"
	@echo "  make logs-service SERVICE=<n>   Show logs for specific service"
	@echo "  make clean                      Clean build artifacts"
	@echo "  make test                       Run all tests"
	@echo "  make test-service SERVICE=<n>   Run tests for specific service"
	@echo "  make test-shared                Run tests for shared module"
	@echo "  make sync                       Sync go workspace"
	@echo ""
	@echo "Available services:"
	@echo "  $(SERVICES)"
