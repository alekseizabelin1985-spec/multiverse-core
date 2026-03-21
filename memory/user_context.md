# User Context

**User:** Алексейabelin1985-spec (based on git author)

**Role:** Full-stack Go developer working on Multiverse-Core event-driven distributed system

**Focus Areas:**
- Neo4j graph-based storage for semantic memory (recent work: graph indexes, event queries)
- Narrative orchestrator with full event descriptions in prompts
- Redpanda event bus architecture
- AI-powered narrative generation with Qwen3/Ollama

**Technical Preferences:**
- Uses detailed documentation (AGENTS.md, README.md per service)
- Prefers comprehensive context for tasks
- Actively maintains project memory in `memory/` directory

**Recent Activity (March 2026):**
- Implemented Neo4j graph mode with indexes and events queries for semantic-memory
- Added full event descriptions to narrative-orchestrator prompts
- Working on RAG system with dual indexing (vector + graph)

## Project Work Context

**Current Phase:** Active development of multiverse-core microservices architecture

**Key Objectives:**
1. Build living narratives system (Game Master) with AI integration
2. Implement dual-indexing semantic memory (ChromaDB + Neo4j)
3. Develop event-driven world generation and management
4. Ensure data integrity with BanOfWorld and recovery patterns

**Development Pattern:**
- Event-driven microservices (15 services total)
- Snapshot + Event Replay for recovery
- MinIO for object storage, Redpanda for streaming
- AI-generated narratives via Qwen3 through Ollama
