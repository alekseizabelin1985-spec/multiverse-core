The user wants me to generate a comprehensive project summary in markdown format based on the conversation history. Let me analyze what happened in this session:

1. The user asked me to analyze the current directory and generate a comprehensive QWEN.md file
2. I explored the project structure by reading multiple files:
   - README_LIVING_WORLDS.md
   - Makefile
   - go.mod
   - docker-compose.yml
   - AGENTS.md
   - CLAUDE.md
   - README.md
   - go.work
   - build/Dockerfile
3. I listed directories to understand the structure
4. I created a comprehensive QWEN.md file

This was essentially a project initialization/context generation session. Let me create a concise summary.# Project Summary

## Overall Goal
Analyze the Multiverse-Core repository structure and generate a comprehensive `QWEN.md` instructional context file for future AI agent interactions.

## Key Knowledge

### Project Identity
- **Name**: Multiverse-Core — distributed event-driven system for managing complex virtual worlds and narratives
- **Philosophy**: Worlds evolve organically through player actions, not hardcoded scripts
- **Language**: Go 1.24+ with `CGO_ENABLED=0` (except `semantic-memory` which uses ONNX Runtime)

### Technology Stack
| Component | Technology |
|-----------|------------|
| Event Streaming | Redpanda v24.2.5 (Kafka-compatible) |
| Object Storage | MinIO |
| Vector DB | ChromaDB + Qdrant |
| Graph DB | Neo4j 5.18 + APOC |
| Time-Series | TimescaleDB (PostgreSQL 16) |
| AI Model | Qwen3 via Ollama |
| Containerization | Docker & Docker Compose |

### Architecture
- **16 microservices** communicating via Kafka topics (`player_events`, `world_events`, `game_events`, `system_events`, `scope_management`, `narrative_output`)
- **Recovery pattern**: Snapshot (MinIO) + Event Replay (Redpanda)
- **Event access**: Use `event.Path()` (jsonpath.Accessor) — mandatory pattern
- **EntityRef format**: Unified nested references `{"entity": {"entity": {"id": "x", "type": "y"}}}`

### Build Commands
```bash
make build              # Docker build all
make build-service SERVICE=<name>  # Local binary
make up / make down     # Start/stop all services
make run SERVICE=<name> # Start specific service
make test               # Run all tests
```

### Critical Conventions
- **NEVER** use direct map access for event payloads (panics on missing keys)
- **ALWAYS** use builder pattern for creating events
- MinIO buckets: `entities-{world_id}` for world-specific, `entities-global` for global
- Schema versions: `schemas/{type}/{name}/v{version}.json`

## Recent Actions

1. **[DONE]** Read and analyzed 9 key files: `README.md`, `README_LIVING_WORLDS.md`, `Makefile`, `go.mod`, `go.work`, `docker-compose.yml`, `AGENTS.md`, `CLAUDE.md`, `build/Dockerfile`
2. **[DONE]** Explored directory structure: `services/` (16 services), `shared/` (12 packages), `Docs/`, `build/`, `configs/`
3. **[DONE]** Generated comprehensive `QWEN.md` file with:
   - Full technology stack and architecture overview
   - All build/run/test commands
   - Mandatory event access and creation patterns
   - EntityRef format specification
   - Design patterns (Event Handler, Snapshot+Replay, Oracle Client, MinIO Isolation)
   - Living Worlds architecture documentation
   - Troubleshooting guide with debug commands
   - Configuration environment variables

## Current Plan

1. **[DONE]** Analyze project structure and key files
2. **[DONE]** Generate comprehensive QWEN.md context file
3. **[TODO]** User review and feedback on generated QWEN.md
4. **[TODO]** Iterate on any missing or incorrect information

## Notable Discoveries

- **Living Worlds** feature branch (`feat/living-worlds-entity-actor`) is design-complete but implementation-pending
- **Go workspace** (`go.work`) manages 15 services + root module
- **Dockerfile** uses multi-stage build with conditional ONNX Runtime installation
- **Semantic Memory** is the only service requiring CGO (for ChromaDB v2 + ONNX)
- Project uses **hierarchical event structure** with backward compatibility for flat keys

---

## Summary Metadata
**Update time**: 2026-04-05T18:39:30.746Z 
