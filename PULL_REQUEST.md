# Agent GM Core - Pull Request

## 🎯 Overview

This PR implements the Agent GM Architecture as described in `Docs/agent-gm-research/`.

## ✅ Implemented

### Phase 1: Base Types and Interfaces (✅ Complete)
- `shared/agent/agent_types.go` - AgentLevel, AgentBlueprint, LODLevel, AgentContext
- `shared/agent/interfaces.go` - Agent, Router, Lifecycle, Worker, MemoryStore, ToolRegistry

### Phase 2: Router + Lifecycle + Worker (✅ Complete)
- `shared/agent/router.go` - Event routing to agents based on blueprints
- `shared/agent/lifecycle.go` - Agent creation, lifecycle management, TTL
- `shared/agent/worker_pool.go` - Async worker pool with job processing
- `shared/agent/helpers.go` - BlueprintFactory, TTLManager
- `shared/agent/agent_test.go` - Comprehensive unit tests

### Phase 3: MD Parser (✅ Complete)
- `shared/agent/md_parser.go` - YAML block extraction from MD files
- `shared/agent/examples/domain-dark-forest.md` - Example blueprint

### Phase 4: Two-Phase LLM Pipeline (✅ Complete)
- `shared/agent/pipeline.go` - Decision (Phase 1) + Narrative (Phase 2) pipeline

### Phase 5: Integration & Migration (✅ Complete)
- `shared/agent/MIGRATION.md` - Migration guide from narrative-orchestrator
- `shared/agent/README.md` - Updated documentation

## 📊 Statistics

- **Files added:** 11
- **Lines of code:** ~17,000+
- **Unit tests:** ✅ All passing
- **Branch:** `feature/agent-gm-core`

## 🎯 Key Features

1. **Agent Hierarchy** - Global → Domain → Task → Object → Monitor
2. **Event-driven Spawning** - Agents created on-demand by events
3. **Two-Phase Pipeline** - Phase 1 (Decision <100ms) + Phase 2 (Narrative async)
4. **LOD (Level of Detail)** - Dynamic adaptation (0-3)
5. **Fallback on Rule-Engine** - Graceful degradation on LLM failure
6. **Blueprint in MD** - Single source (config + prompt + constraints)

## 📈 Expected Improvements

| Metric | Before | After |
|--------|--------|-------|
| Latency p95 | ~500ms | **<100ms** |
| LLM cost | 100% | **~20%** (cache) |
| Scalability | ~100/shard | **~1000/shard** |
| Availability | ~99% | **99.9%** |

## 🔗 Related

- User Stories: `Docs/agent-gm-research/01_user_stories.md`
- System Analysis: `Docs/agent-gm-research/02_system_analysis.md`
- Roadmap: `Docs/agent-gm-research/03_conclusions_roadmap.md`

## 🚀 Next Steps

1. Review and approve
2. Merge to main
3. Deploy `services/agent-orchestrator` in staging
4. Run A/B test with 10% traffic
5. Monitor metrics and roll out

---

**Status:** Ready for Review | **Author:** Алексей Забелин | **Date:** 2026-04-11
