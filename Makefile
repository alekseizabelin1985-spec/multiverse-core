# Project variables
PROJECT_NAME := multiverse-core
DOCKER_IMAGE := $(PROJECT_NAME)

# Pinned versions of every image and toolchain (infrastructure.md §2.4). This is
# the only place they come from: never write a tag into a recipe.
include build/versions.env

GIT_SHA := $(shell git rev-parse --short HEAD)

# -----------------------------------------------------------------------------
# Images (EPIC-001 F-6a, task T-004). The rest of the target set of
# infrastructure.md §2.2 — up/down/health/ci/compose-lint/llm-* — and the
# removal of the as-is workspace targets below land in T-008 (F-6b), which
# rewrites this file as a whole.
# -----------------------------------------------------------------------------

# Platform image: one image, three binaries; `command` in compose picks the role.
.PHONY: image
image:
	docker build -f build/Dockerfile --build-arg GO_VERSION=$(GO_VERSION) --build-arg VERSION=$(GIT_SHA) -t multiverse-core:$(GIT_SHA) -t multiverse-core:dev .

# MinIO built from source (ADR-021 variant B): no upstream image exists for the
# tag that carries the CVE-2025-62506 fix. The first build takes about five
# minutes, later ones come from the BuildKit cache.
.PHONY: minio-image
minio-image:
	docker build -f build/minio.Dockerfile --build-arg MINIO_TAG=$(MINIO_TAG) --build-arg MINIO_REPO=$(MINIO_REPO) --build-arg MINIO_BUILDER_IMAGE=$(MINIO_BUILDER_IMAGE) -t $(MINIO_IMAGE) .

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
	@echo "  make image                      Build the platform image (build/Dockerfile)"
	@echo "  make minio-image                Build MinIO from source (build/minio.Dockerfile)"
	@echo ""
	@echo "Available services:"
	@echo "  $(SERVICES)"
