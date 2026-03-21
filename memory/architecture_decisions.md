# Architecture Decisions - Multiverse-Core

## Event-Driven Architecture

**Decision:** Use Redpanda (Kafka-compatible) as central event bus

**Why:**
- Kafka API compatibility with simpler deployment
- High throughput for event streaming
- Supports consumer groups for scalable processing
- 9 topics for different event categories

**How to apply:** All services communicate via events through Redpanda. Use partition keys (e.g., `scope_id`) for ordering guarantees within partitions.

## Dual Indexing Semantic Memory

**Decision:** Combine ChromaDB (vector) + Neo4j (graph) for semantic memory

**Why:**
- Vector search for semantic similarity
- Graph queries for relationship traversal
- Full event history stored for context and replay
- Better RAG performance than vector-only

**How to apply:**
- Store embeddings in ChromaDB
- Store events as `:Event` nodes and entities as `:Entity` nodes in Neo4j
- Link via `[:RELATED_TO]` relationships
- Use indexes on `event_id`, `entity_id`, `event_type`, `world_id`, `timestamp`

## Narrative Orchestrator (Game Master)

**Decision:** One GM instance per `scope_id` with state stored in MinIO snapshots

**Why:**
- Isolates narrative context by scope
- Enables parallel GM instances for different worlds/regions
- Snapshots enable recovery and scaling

**How to apply:**
- Subscribe to `gm.created`/`gm.deleted` events for lifecycle
- Store `KnowledgeBase` snapshots in MinIO (`narrative-orchestrator/{scope_id}/snapshot.json`)
- Use consumer groups with `scope_id` as partition key

## Full Event Descriptions in Prompts

**Decision:** Format events with human-readable descriptions in LLM prompts

**Why:**
- Improves LLM understanding of events
- Provides context for narrative generation
- Enables time-relative event sequencing

**How to apply:**
- Extract entity IDs from payload (`entity_id`, `player_id`, `actor_id`, etc.)
- Extract action from `action`, `type`, or `event_type` fields
- Format as: `[relative_time]: • [event_id[:8]] event_type: Human-readable description`

## Recovery Pattern: Snapshot + Event Replay

**Decision:** Store state snapshots in MinIO, replay events from Redpanda for recovery

**Why:**
- Snapshots provide point-in-time state
- Event log enables reconstruction of state changes
- Decouples storage from processing

**How to apply:**
- Regularly snapshot stateful services to MinIO
- On startup, restore from latest snapshot
- Replay events from Redpanda since snapshot timestamp

## Stateless vs Stateful Services

**Decision:** Categorize services into stateless (processing only) and stateful (with persistent state)

**Why:**
- Stateless services scale horizontally easily
- Stateful services manage their own persistence
- Clear separation of concerns

**How to apply:**
- Stateless: entity-actor, evolution-watcher, rule-engine, world-generator
- Stateful: entity-manager, narrative-orchestrator, semantic-memory, ban-of-world, city-governor, cultivation-module

## AI Integration via Ollama

**Decision:** Use local Ollama with Qwen3 model for AI generation

**Why:**
- Local deployment for privacy and control
- Qwen3 for narrative generation
- HTTP `/v1/chat/completions` API compatibility

**How to apply:**
- Set `LLM_ENDPOINT=http://ollama:11434/v1`
- Use structured prompts with system rules and JSON schema responses
- Handle GPU resources via Docker deploy config

## Graph-Based Entity Relationships

**Decision:** Model entities and their relationships in Neo4j, not just events

**Why:**
- Enables complex relationship queries
- Supports traversal of entity networks
- Better than flat storage for relationship-intensive queries

**How to apply:**
- Create `:Entity` nodes for each entity with `id`, `type`, `world_id`, `payload`
- Create `[:RELATED_TO]` relationships between related entities
- Query patterns: `MATCH (a:Entity)-[r]->(b:Entity) WHERE a.id = 'player-123' RETURN b`
