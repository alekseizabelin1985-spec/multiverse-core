# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Project Overview

This is a multiverse game engine built with event-driven architecture. The system consists of multiple specialized services that communicate through an event bus (Redpanda/Kafka).

Each service has its own AGENTS.md file in its respective directory under `services/{service-name}/AGENTS.md` with specific guidance for working with that service.

## Build/lint/test commands

- Build all services: `make build`
- Build specific service: `make build-service SERVICE=entity-manager`
- Run specific service: `make run SERVICE=entity-manager`
- Start all services: `make up`
- Stop all services: `make down`
- Clean build artifacts: `make clean`
- Show logs for all services: `make logs`
- Show logs for specific service: `make logs-service SERVICE=entity-manager`
- Run tests: `make test`

## Code style guidelines

- All services are written in Go 1.25
- Use JSON Schema Draft 7 for entity payload validation
- Follow event-driven architecture with Kafka/Redpanda as message broker
- Services communicate through events in the event bus (topics: player_events, world_events, game_events, system_events, scope_management, narrative_output)
- Entity management uses MinIO for storage with bucket naming pattern: `entities-{world_id}`
- All services are stateful and support recovery via snapshots and event replay
- **Use `event.Path()` (jsonpath.Accessor) for ALL event data access** — provides type-safe, fallback-compatible access
- **Use hierarchical event structure** (`entity:{id,type}`, `world:{id}`, `scope:{id,type}`) for NEW events; flat keys still supported for backward compatibility
- All services must be built with CGO disabled for cross-platform compatibility (`CGO_ENABLED=0`)
- Use UUIDs for event IDs and entity IDs
- All services must be containerized with Docker using multi-stage builds

### 🔀 Event Access Patterns (MANDATORY)

```go
// ✅ ALWAYS use for reading event data:
pa := event.Path()  // *jsonpath.Accessor

// Extract with fallback chain — NEW format: entity.entity.id
entityID, _ := pa.GetString("entity.entity.id")
if entityID == "" {
    entityID, _ = pa.GetString("entity.id")  // fallback previous format
}
if entityID == "" {
    entityID, _ = pa.GetString("entity_id")  // fallback legacy
}

// World/Scope: use unified helpers
worldID := eventbus.GetWorldIDFromEvent(event)  // reads event.World.Entity.ID
scope := eventbus.GetScopeFromEvent(event)      // returns *ScopeRef{ID, Type}

// Type-safe getters for any depth:
level, _ := pa.GetInt("entity.stats.level")
active, _ := pa.GetBool("entity.active")
items, _ := pa.GetSlice("entity.inventory")

// Array access by index:
firstItem, _ := pa.GetString("entity.inventory[0].name")

// Quick existence check:
if pa.Has("quest.objectives") { /* ... */ }
```

### 📝 Creating Events (MANDATORY)

```go
// ✅ ALWAYS use builder for new events:
payload := eventbus.NewEventPayload().
    WithEntity(id, entityType, name).
    WithScope(scopeID, scopeType).  // solo/group/city/region/quest
    WithWorld(worldID)

// Add custom fields with dot-notation — use entity/event reference format:
eventbus.SetNested(payload.GetCustom(), "entity.entity.id", entityID)
eventbus.SetNested(payload.GetCustom(), "entity.entity.type", entityType)
eventbus.SetNested(payload.GetCustom(), "trigger.event.id", triggerEventID)
eventbus.SetNested(payload.GetCustom(), "trigger.event.type", "event")

event := eventbus.NewStructuredEvent(type, source, worldID, payload)
bus.Publish(ctx, topic, event)
```

### 🕸️ EntityRef format (ALL references)

All entity/event references in payload use unified format:

```json
{
  "entity":  {"entity": {"id": "player-123", "type": "player"}},
  "target":  {"entity": {"id": "sword-456", "type": "item"}},
  "world":   {"entity": {"id": "world-789", "type": "world"}},
  "trigger": {"event":  {"id": "evt-abc", "type": "event"}}
}
```

Neo4j automatically creates relationships from payload keys:
- `(ev)-[:ENTITY]->(player-123:Entity)`
- `(ev)-[:TARGET]->(sword-456:Entity)`
- `(ev)-[:WORLD]->(world-789:Entity)`
- `(ev)-[:TRIGGER]->(evt-abc:Event)`

Entity↔Entity relations created via `relations[]` array.

### ⚠️ Deprecated Patterns (AVOID in new code)

```go
// ❌ DON'T use direct map access (panics on missing/wrong type):
entityID := event.Payload["entity_id"].(string)
worldID := event.WorldID

// ❌ DON'T create events with manual maps:
event := eventbus.Event{Payload: map[string]interface{}{...}}

// ❌ DON'T use flat reference fields:
payload["entity_id"] = id
payload["world_id"] = worldID

// ✅ USE the patterns above instead
```

## Custom utilities and patterns

- Entity manager uses a custom entity structure with history tracking and path-based payload access
- World generator uses Ascension Oracle (Qwen3) for generating world details and schemas
- Narrative orchestrator uses Semantic Memory Builder (ChromaDB + Neo4j) for RAG context
- Ontological Archivist stores schemas in MinIO with versioned JSON files
- BanOfWorld service uses "resonance" metrics for world integrity monitoring
- CultivationModule generates "Dao portraits" from player history for ascension events
- All services use structured logging with consistent format
- Services use context for cancellation and timeouts
- Event bus uses Kafka with LeastBytes balancer for load distribution
- Services implement graceful shutdown with HTTP server shutdown and resource cleanup
- Custom Oracle client with retry logic and structured response handling
- Custom MinIO client with common interfaces for storage operations
- Event bus with configurable polling frequency via `KAFKA_POLL_FREQUENCY_MS` environment variable

## Non-standard directory structures

- Services are organized in `services/` directory with each service in its own subdirectory
- Commands are in `cmd/` directory with each command in its own subdirectory
- Internal packages are in `internal/` directory
- Documentation is in `Docs/` directory
- Docker configuration in `build/` directory
- Fake dependencies for CGO issues in `fake_deps/` directory

## Project-specific conventions

- All services must be built with CGO_ENABLED=0 for cross-platform compatibility
- Services use specific naming conventions for topics: `player_events`, `world_events`, `game_events`, `system_events`, `scope_management`, `narrative_output`
- World entities are stored in MinIO buckets named `entities-{world_id}`
- Global entities are stored in `entities-global` bucket
- Schema versions are stored in MinIO with path pattern: `schemas/{type}/{name}/v{version}.json`
- All services must be run with Docker Compose for proper environment setup
- Services communicate through event bus, not direct API calls
- Entity manager supports both state changes and full entity snapshots
- All services must be able to recover from snapshots and replay events
- Services use specific environment variables for configuration (MINIO_ENDPOINT, ORACLE_URL, SEMANTIC_MEMORY_URL, etc.)
- World generation requires specific seed-based prompts for Qwen3
- Services must handle entity travel between worlds through snapshot management
- All services must be able to handle concurrent access to shared resources
- Services use specific event types for different operations (entity.created, entity.updated, world.generated, etc.)
- Semantic Memory service stores all events for context and replay
- Services must handle entity travel between worlds through snapshot management
- All services must be able to handle concurrent access to shared resources
- Services use specific event types for different operations (entity.created, entity.updated, world.generated, etc)
- Semantic Memory service stores all events for context and replay

## Service-Specific Guidance

Each service has detailed guidance in its own AGENTS.md file:

- [EntityManager](services/entitymanager/AGENTS.md) - Manages game entities and their history
- [NarrativeOrchestrator](services/narrativeorchestrator/AGENTS.md) - Generates dynamic narratives
- [WorldGenerator](services/worldgenerator/AGENTS.md) - Creates game worlds
- [BanOfWorld](services/banofworld/AGENTS.md) - Monitors world integrity
- [CityGovernor](services/citygovernor/AGENTS.md) - Manages city structures
- [CultivationModule](services/cultivationmodule/AGENTS.md) - Handles player cultivation
- [RealityMonitor](services/realitymonitor/AGENTS.md) - Monitors game reality state
- [PlanManager](services/planmanager/AGENTS.md) - Manages player plans
- [SemanticMemory](services/semanticmemory/AGENTS.md) - Provides semantic context
- [OntologicalArchivist](services/ontologicalarchivist/AGENTS.md) - Stores schemas
- [UniverseGenesisOracle](services/universegenesis/AGENTS.md) - Generates universes
- [GameService](services/gameservice/AGENTS.md) - Handles player interactions
- [AscensionOracle](services/ascensionoracle/AGENTS.md) - Provides AI generation