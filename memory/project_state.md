# Multiverse-Core Project State (2026-03-19)

**Архитектура:** Event-driven distributed system для управления виртуальными мирами и нарративами на Go 1.24/1.25

## Infrastructure Stack

| Компонент | Версия/Описание | Порт |
|-----------|----------------|------|
| **Redpanda** | Kafka-compatible event streaming (v24.2.5) | 9092, 8081, 9644 |
| **MinIO** | Object storage (S3-compatible) | 9000, 9001 |
| **ChromaDB** | Vector database for semantic memory | 8000 |
| **Neo4j** | Graph database with APOC plugin (v5.18) | 7474, 7687 |
| **TimescaleDB** | Time-series metrics (PG16) | 5433 |
| **Qwen3 + Ollama** | AI model for narrative generation | 11434 |
| **Qdrant** | Additional vector store | 6333, 6334 |
| **Redpanda Console** | Kafka UI | 8092 |

## Event Bus Topics (Redpanda)

- `player_events` - Player actions
- `world_events` - World state changes
- `game_events` - Game mechanics
- `system_events` - System operations
- `scope_management` - Scope lifecycle
- `narrative_output` - Narrative results

## Core Services (15 total)

### Stateful Services

| Service | Purpose | State Storage | Topics |
|---------|---------|---------------|--------|
| **entity-manager** | Hierarchical entities with history | MinIO (buckets: `entities-{world_id}`) | entity_events |
| **narrative-orchestrator** | Living narratives (Game Master) | MinIO snapshots + Memory | system_events, world_events, game_events, narrative_output |
| **semantic-memory** | Dual indexing (vector + graph) for RAG | ChromaDB + Neo4j | All event topics |
| **ban-of-world** | Reality integrity guardian | In-memory (real-time) | system_events |
| **city-governor** | City economy, quests, NPCs | Stateful | city_events |
| **cultivation-module** | Player cultivation, ascension | Stateful | cultivation_events |
| **reality-monitor** | Metrics aggregation | Stateful | metrics |
| **plan-manager** | Plane transitions (DAG) | Stateful | plan_events |
| **ontological-archivist** | Schema storage | MinIO + ChromaDB + Neo4j | schema_events |
| **game-service** | HTTP player API | Stateless | HTTP port 8088 |

### Stateless Services

| Service | Purpose | Dependencies |
|---------|---------|--------------|
| **entity-actor** | Entity processing | Redpanda, MinIO |
| **evolution-watcher** | Evolution monitoring | Redpanda, MinIO |
| **rule-engine** | Rule execution | Redpanda, MinIO |
| **world-generator** | World/region/ontology generation | Redpanda, OntologicalArchivist, UniverseGenesisOracle |
| **universe-genesis-oracle** | Ascension/Universe generation | Redpanda, OntologicalArchivist |

## Key Integrations

- **Semantic Memory → Narrative Orchestrator:**
  - `POST /v1/events-by-entities` - Full events with payloads
  - `GET /entity/{id}/history` - Entity history
  - `GET /location/{id}` - Location descriptions

- **Narrative Orchestrator → LLM:**
  - HTTP `/v1/chat/completions` (Ollama/Qwen3)
  - Prompts include full event descriptions with human-readable text

- **Recovery Pattern:** Snapshot (MinIO) + Event Replay (Redpanda)

## Neo4j Graph Model (semantic-memory)

### Nodes
```
:Event {
  id: string,
  type: string,
  timestamp: datetime,
  source: string,
  world_id: string,
  scope_id: string,
  payload_json: string
}

:Entity {
  id: string,
  type: string,
  world_id: string,
  payload: map
}
```

### Relationships
```
(:Event)-[:RELATED_TO]->(:Entity)
```

### Indexes
- `event_id`, `entity_id`, `event_type`, `world_id`, `timestamp`

## Recent Changes (Git)

- `062f1dc` feat(semantic-memory): implement graph-based Neo4j storage for events
- `87e975a` feat(narrative-orchestrator): добавить полные описания событий в промт
- `60a86ec` refactor(semantic-memory): Neo4j graph mode with indexes and events queries

## Configuration

### Environment Variables (Common)
- `KAFKA_BROKERS` - Redpanda address (default: `localhost:9092`)
- `MINIO_ENDPOINT` - MinIO address (default: `localhost:9000`)
- `CHROMA_URL` - ChromaDB address (default: `http://chromadb:8000`)
- `NEO4J_URI` - Neo4j address (default: `neo4j://neo4j:7687`)
- `LLM_ENDPOINT` - LLM API address (default: `http://ollama:11434/v1`)

### Service Ports
- `game-service` - HTTP API (8088)
- `semantic-memory` - HTTP API (8082)
- `ontological-archivist` - HTTP API (8083)
- `qwen3-service` - Ollama (11434)
- `redpanda-console` - UI (8092)

## Memory System

Located at: `D:\my project\Go project\multiverse-core\memory\`
- Stores project context, user preferences, and feedback
- Indexed for semantic search
