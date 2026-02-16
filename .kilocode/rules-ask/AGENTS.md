# Project Documentation Rules (Non-Obvious Only)

- All services must be built with CGO_ENABLED=0 for cross-platform compatibility
- Services communicate through event bus, not direct API calls
- Entity manager supports both state changes and full entity snapshots
- All services must be able to recover from snapshots and replay events
- Services must handle entity travel between worlds through snapshot management
- All services must be able to handle concurrent access to shared resources
- Services use specific event types for different operations (entity.created, entity.updated, world.generated, etc.)
- Semantic Memory service stores all events for context and replay
- Custom Oracle client with retry logic and structured response handling
- Custom MinIO client with common interfaces for storage operations
- Event bus with configurable polling frequency via `KAFKA_POLL_FREQUENCY_MS` environment variable
- World entities are stored in MinIO buckets named `entities-{world_id}`
- Global entities are stored in `entities-global` bucket
- Schema versions are stored in MinIO with path pattern: `schemas/{type}/{name}/v{version}.json`
- Services use specific naming conventions for topics: `player_events`, `world_events`, `game_events`, `system_events`, `scope_management`, `narrative_output`
- Services use specific environment variables for configuration (MINIO_ENDPOINT, ORACLE_URL, SEMANTIC_MEMORY_URL, etc.)
- World generation requires specific seed-based prompts for Qwen3
- All services must be run with Docker Compose for proper environment setup