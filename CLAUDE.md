# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Multiverse-Core** is a sophisticated event-driven distributed system for managing complex virtual worlds and narratives. Built with Go 1.25, it combines Redpanda (Kafka-compatible) for event streaming, ChromaDB for vector search, Neo4j for graph knowledge, MinIO for object storage, and Qwen3 AI for generative narrative.

**Key Philosophy**: Worlds are not programmed but born, evolving organically through player actions while maintaining internal consistency.

## Architecture

### High-Level Structure

```
multiverse-core/
├── go.work                    # Go workspace (15 services + shared)
├── services/                  # Individual microservices
│   ├── entity-manager/        # Hierarchical entities with history
│   ├── narrative-orchestrator/ # Living narratives (GM)
│   ├── world-generator/       # World/region/ontology generation
│   └── ... (12 more services)
├── shared/                    # Shared Go packages
│   ├── config/                # Configuration management
│   ├── entity/                # Entity structure
│   ├── eventbus/              # Kafka/Redpanda client
│   ├── intent/                # Intent recognition
│   ├── minio/                 # MinIO client
│   ├── oracle/                # Oracle HTTP client
│   └── ... (5 more packages)
├── Docs/                      # Documentation
├── configs/                   # Service YAML configs
├── docker-compose.yml         # Full stack orchestration
└── Makefile                   # Build/test/run commands
```

### Infrastructure Stack

| Component | Purpose |
|-----------|---------|
| **Redpanda** | Event streaming (9 topics including player_events, world_events, narrative_output) |
| **MinIO** | Object storage (entities, snapshots, schemas) |
| **ChromaDB** | Vector database for semantic memory |
| **Neo4j** | Graph database for relationships |
| **TimescaleDB** | Time-series metrics |
| **Ollama + Qwen3** | AI model for narrative generation |

### Event Bus Topics

- `player_events` - Player actions
- `world_events` - World state changes
- `game_events` - Game mechanics
- `system_events` - System operations
- `scope_management` - Scope lifecycle
- `narrative_output` - Narrative results

### Core Services (15 total)

| Service | Stateful | Purpose |
|---------|----------|---------|
| `entity-manager` | ✅ | Hierarchical entities with history, MinIO storage |
| `narrative-orchestrator` | ✅ | Living narratives from scope events |
| `world-generator` | ❌ | Generate worlds/regions via Qwen3 |
| `ban-of-world` | ✅ | Reality integrity guardian |
| `city-governor` | ✅ | City economy, quests, NPCs |
| `cultivation-module` | ✅ | Player cultivation, ascension |
| `reality-monitor` | ✅ | Metrics aggregation |
| `plan-manager` | ✅ | Plane transitions (DAG) |
| `semantic-memory` | ✅ | Event indexing (ChromaDB + Neo4j) |
| `ontological-archivist` | ✅ | Schema storage (MinIO) |
| `game-service` | ✅ | HTTP player API (port 8088) |
| `entity-actor`, `evolution-watcher`, `rule-engine`, `universe-genesis-oracle` | 4 more |

**Recovery Pattern**: Snapshot (MinIO) + Event Replay (Redpanda)

## Build/Lint/Test Commands

### Makefile Commands

```bash
# Build
make build                      # Build all services (Docker)
make build-service SERVICE=<n>  # Build specific service locally
make build-all                  # Build all services locally

# Run
make up                         # Start all services (Docker Compose)
make run SERVICE=<name>         # Start specific service
make down                       # Stop all services

# Logs
make logs                       # All service logs
make logs-service SERVICE=<n>   # Specific service logs

# Test
make test                       # Run all tests
make test-service SERVICE=<n>   # Test specific service
make test-shared                # Test shared module

# Maintenance
make clean                      # Clean build artifacts
make sync                       # Sync Go workspace
```

### Direct Go Commands

```bash
# Sync workspace
go work sync

# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Test specific module
cd services/entity-manager && go test ./...
```

### Docker Commands

```bash
# Build single service locally
cd services/narrative-orchestrator && CGO_ENABLED=0 GOOS=linux go build -o ../../bin/narrative-orchestrator ./cmd/

# Docker build
docker build --build-arg SERVICE=narrative-orchestrator -t multiverse-core:narrative-orchestrator .

# Start infrastructure only
docker-compose up redpanda minio chromadb neo4j timescaledb

# Run service locally with infrastructure in Docker
KAFKA_BROKERS=localhost:9092 MINIO_ENDPOINT=localhost:9000 go run services/narrative-orchestrator/cmd/main.go
```

## Code Patterns & Conventions

### Entity Management

- **Path-based payload access**: Use dot notation (`payload.health.current`)
- **Storage**: MinIO buckets `entities-{world_id}`
- **Events**: `entity.created`, `entity.updated`, `entity.history.appended`

### Event Structure

```go
type Event struct {
    ID        uuid.UUID
    Type      string  // e.g., "entity.create"
    Timestamp time.Time
    Payload   map[string]interface{}
}
```

### Oracle Integration

All services use custom HTTP client with retry logic:
```go
shared/oracle/Client - Ascension Oracle, Universe Genesis Oracle
```

### Testing Patterns

Tests use minimal mocks and follow naming convention:
```bash
services/narrative-orchestrator/narrativeorchestrator/prompt_builder_test.go
services/world-generator/worldgenerator/generator_test.go
```

## Development Workflow

### Setting Up Environment

1. Clone repository
2. Copy `.env.example` to `.env` and configure
3. Start infrastructure: `docker-compose up -d`
4. Build and run: `make build-all` then `make run SERVICE=narrative-orchestrator`

### Service-Specific Guidance

Each service has its own `AGENTS.md` file:
- `services/entity-manager/AGENTS.md`
- `services/narrative-orchestrator/AGENTS.md`
- `services/world-generator/AGENTS.md`
- And 12 more service-specific guides

See [AGENTS.md](AGENTS.md) for cross-service patterns.

### Key Files to Reference

- [docker-compose.yml](docker-compose.yml) - Full infrastructure stack
- [go.work](go.work) - Workspace configuration
- [Configs](configs/) - Service YAML configurations
- [Docs/](Docs/) - Architecture documentation, feature guides
