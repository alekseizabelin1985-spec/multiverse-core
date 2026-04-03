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

### 🔀 Hierarchical Event Access (NEW — PREFERRED)

**Always use `event.Path()` for reading event data** — it provides universal, type-safe access with fallback support:

```go
// ✅ PREFERRED: Universal access via jsonpath
func handler(event eventbus.Event) {
    pa := event.Path()  // *jsonpath.Accessor
    
    // Extract with fallback: new structure → old structure → default
    entityID, _ := pa.GetString("entity.id")
    if entityID == "" {
        entityID, _ = pa.GetString("entity_id")  // fallback
    }
    
    // World/Scope: use helper functions for both structures
    worldID := eventbus.GetWorldIDFromEvent(event)  // payload.world.id OR world_id
    scope := eventbus.GetScopeFromEvent(event)      // payload.scope:{id,type} OR scope_id
    
    // Type-safe getters for any depth
    level, _ := pa.GetInt("entity.stats.level")
    active, _ := pa.GetBool("entity.active")
    tags, _ := pa.GetSlice("entity.tags")
    
    // Array access by index
    firstTag, _ := pa.GetString("entity.tags[0]")
    
    // Quick existence check
    if pa.Has("quest.objectives") { /* ... */ }
}
```

### 📝 Creating Events with Hierarchical Structure

**Use the builder pattern for new events** — ensures consistent, LLM-friendly structure:

```go
// ✅ PREFERRED: Builder + hierarchical structure
payload := eventbus.NewEventPayload().
    WithEntity("player-123", "player", "Вася").
    WithScope("city-xyz", "city").        // solo/group/city/region/quest
    WithWorld("world-abc")

// Add custom fields with dot-notation for flexibility
eventbus.SetNested(payload.GetCustom(), "action", "talk")
eventbus.SetNested(payload.GetCustom(), "dialogue.text", "Hello!")

// Optional: add hierarchical paths explicitly for LLM clarity
eventbus.SetNested(payload.GetCustom(), "entity.id", "player-123")
eventbus.SetNested(payload.GetCustom(), "world.id", "world-abc")

event := eventbus.NewStructuredEvent("player.talked", "entity-actor", "world-abc", payload)
bus.Publish(ctx, eventbus.TopicWorldEvents, event)
```

### ⚠️ Deprecated Patterns (Still Supported, But Avoid in New Code)

```go
// ❌ AVOID in new code (still works for backward compatibility):
entityID := event.Payload["entity_id"].(string)  // panics if missing/wrong type!
worldID := event.WorldID                          // top-level field, not in payload

// ✅ USE instead:
pa := event.Path()
entityID, _ := pa.GetString("entity.id")  // safe + fallback support
worldID := eventbus.GetWorldIDFromEvent(event)  // unified access
```

### 🎭 LLM Prompt Generation (Narrative Orchestrator)

**Prompts must use hierarchical JSON schema** — see `prompt_builder.go` for exact format:

```json
{
  "event_type": "player.entered_region",
  "world": {"id": "world-abc"},
  "scope": {"id": "solo-xyz", "type": "solo"},
  "entity": {"id": "player-123", "type": "player", "name": "Вася"},
  "target": {"entity": {"id": "region-456", "type": "region", "name": "Тёмный лес"}},
  "payload": {"description": "...", "weather": "пасмурно"}
}
```

**Rules for AI-generated events**:
- `world.id` is REQUIRED (not `world_id`)
- `scope:{id,type}` is OPTIONAL but preferred over `scope_id`
- `entity:{id,type,name}` is OPTIONAL but improves context
- Always validate JSON structure before publishing

### 📦 Universal `jsonpath` Package

The `shared/jsonpath` package works with ANY `map[string]any` data, not just events:

```go
import "multiverse-core.io/shared/jsonpath"

// Works with configs, API responses, any JSON-like data
acc := jsonpath.New(anyData)

// All the same getters:
val, _ := acc.GetString("config.db.host")
port, _ := acc.GetInt("server.ports[0]")
meta, _ := acc.GetMap("user.profile")

// Debug: list all available paths
for _, path := range acc.GetAllPaths() {
    fmt.Println(path)  // config, config.db, config.db.host, ...
}
```

**Read**: [`shared/jsonpath/README.md`](shared/jsonpath/README.md) for full API reference.

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
