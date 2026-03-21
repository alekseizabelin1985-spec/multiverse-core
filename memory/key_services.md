# Key Services Overview

## entity-manager
**Purpose:** Hierarchical entities with history, MinIO storage
**State:** Stateful (MinIO buckets: `entities-{world_id}`)
**Topics:** `entity_events` (entity.created, entity.updated, entity.deleted)
**Key Features:**
- Snapshot/replay recovery
- World isolation via buckets
- Entity history tracking
- Config: `MINIO_ENDPOINT`, `KAFKA_BROKERS`

## narrative-orchestrator (Game Master)
**Purpose:** Living narratives from scope events
**State:** Stateful (MinIO snapshots + Memory)
**Topics:** system_events, world_events, game_events, narrative_output
**Key Features:**
- One GM per `scope_id`
- Consumer groups with partition key `scope_id`
- Full event descriptions in LLM prompts
- KnowledgeBase persistence
- Config: `LLM_ENDPOINT`, `SEMANTIC_MEMORY_ENDPOINT`

## semantic-memory
**Purpose:** Dual indexing (vector + graph) for RAG
**State:** Stateless (ChromaDB + Neo4j storage)
**Topics:** All event topics (player_events, world_events, game_events, system_events, scope_management, narrative_output)
**Key Features:**
- ChromaDB for vector search
- Neo4j for graph relationships
- Event storage as `:Event` nodes
- Entity storage as `:Entity` nodes
- API: `POST /v1/events-by-entities`, `GET /v1/events/{id}`
- Config: `CHROMA_URL`, `NEO4J_URI`, `MINIO_ENDPOINT`

## ban-of-world
**Purpose:** Reality integrity guardian
**State:** Stateless (in-memory real-time)
**Topics:** system_events (world.integrity.check)
**Key Features:**
- Resonance monitoring
- Real-time integrity checking
- Config: `RESONANCE_THRESHOLD` (default: 0.8)

## world-generator
**Purpose:** World/region/ontology generation via AI
**State:** Stateless
**Topics:** world_events (world.generate → world.generated)
**Key Features:**
- Async world generation
- Schema fetching from UniverseGenesisOracle
- Entity creation via EntityManager
- Geographic generation (regions, water, cities)
- Config: `ORACLE_URL`, `KAFKA_BROKERS`

## universe-genesis-oracle
**Purpose:** Ascension and Universe generation
**State:** Stateless
**Topics:** system_events
**Key Features:**
- Schema generation
- Integration with OntologicalArchivist
- Ascension logic

## cultivation-module
**Purpose:** Player cultivation, ascension
**State:** Stateful
**Topics:** cultivation_events
**Key Features:**
- Cultivation tracking
- Ascension management

## city-governor
**Purpose:** City economy, quests, NPCs
**State:** Stateful
**Topics:** city_events
**Key Features:**
- Economic simulation
- Quest generation
- NPC management

## plan-manager
**Purpose:** Plane transitions (DAG)
**State:** Stateful
**Topics:** plan_events
**Key Features:**
- Plane management
- DAG-based transition logic

## ontological-archivist
**Purpose:** Schema storage (MinIO + ChromaDB + Neo4j)
**State:** Stateless
**Topics:** schema_events
**Key Features:**
- Schema persistence
- Integration with WorldGenerator and UniverseGenesisOracle

## game-service
**Purpose:** HTTP player API
**State:** Stateless
**Ports:** HTTP 8088
**Key Features:**
- Player interaction endpoints
- Integration with EntityManager

## entity-actor, evolution-watcher, rule-engine
**Purpose:** Entity processing, monitoring, rule execution
**State:** Stateless
**Dependencies:** Redpanda, MinIO
