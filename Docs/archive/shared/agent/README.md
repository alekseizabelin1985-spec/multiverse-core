# Agent GM Architecture - Итоговый README

## 🎯 Обзор

Это базовый пакет для реализации **агентной архитектуры Game Master** согласно документу **Docs/agent-gm-research/**.

## ✅ Фазы реализации

| Фаза | Статус | Компоненты |
|------|--------|------------|
| **Phase 1** | ✅ Завершена | Базовые типы и интерфейсы |
| **Phase 2** | ✅ Завершена | Router, Lifecycle, Worker Pool |
| **Phase 3** | ✅ Завершена | Markdown parser для блупринтов |
| **Phase 4** | ✅ Завершена | Двухфазный LLM-конвейер |
| **Phase 5** | 🚧 В работе | Интеграция с narrative-orchestrator |

## 📁 Структура

```
shared/agent/
├── agent_types.go      # AgentLevel, AgentBlueprint, LODLevel, AgentContext
├── interfaces.go       # Agent, Router, Lifecycle, Worker, MemoryStore, ToolRegistry
├── router.go           # Event routing to agents
├── lifecycle.go        # Agent creation, lifecycle management, TTL
├── worker_pool.go      # Async worker pool
├── helpers.go          # BlueprintFactory, TTLManager
├── md_parser.go        # YAML block extraction from MD files
├── pipeline.go         # Two-phase LLM pipeline
├── blueprint_loader.go # YAML/MD blueprint parsing
├── agent_test.go       # Unit tests
├── examples/
│   └── domain-dark-forest.md
├── MIGRATION.md        # Migration guide
└── README.md           # This file
```

## 🚀 Быстрый старт

### 1. Создание агента

```go
import "multiverse-core/shared/agent"

// Создать роутер
lifecycle := agent.NewLifecycleManager(agent.NewDefaultBlueprintFactory(), 1*time.Hour)
router := agent.NewRouter(lifecycle)

// Создать пул воркеров
pool := agent.NewWorkerPool(10)
pool.Start(context.Background())

// Создать двухфазный конвейер
llmClient := getLLMClient()
pipeline := agent.NewTwoPhasePipeline(llmClient, agent.DefaultPipelineConfig())

// Зарегистрировать блупринт
bp := agent.NewDefaultBlueprintFactory().ParseFile("domain-dark-forest.md")
router.RegisterBlueprint(bp)

// Обработать событие
event := agent.Event{
    Type:      "player.entered_region",
    ScopeID:   "dark-forest-01",
    Timestamp: time.Now(),
}

router.RouteEvent(context.Background(), event)
```

### 2. Создание блупринта

```yaml
# domain-dark-forest.md
name: domain-dark-forest
version: "1.0"
type: domain

trigger:
  type: event
  event_name: player.entered_region

constraints:
  max_instances: 1
  priority: 50

ttl: "1h"

llm:
  model: qwen:7b
  temperature: 0.7
  max_tokens: 2048

tools:
  - name: spawn_npc
  - name: modify_terrain

phase1_prompt: |
  Ты — GM региона {region_name}. Игрок {player_name} вошел.
  Верни JSON с решениями.

phase2_prompt: |
  Опиши событие атмосферно.

parent:
  name: global_supervisor
```

## 📊 Ключевые компоненты

### Agent Level (Иерархия)

| Уровень | TTL | Пример |
|---------|-----|--------|
| `LevelGlobal` | Долгоживущий | надзор за миром |
| `LevelDomain` | ~1 час | регион "Тёмный лес" |
| `LevelTask` | ~минуты | квест, встреча |
| `LevelObject` | ~секунды | дерево, NPC реакция |
| `LevelMonitor` | Долгоживущий | аномалии |

### LOD (Level of Detail)

| LOD | Описание | Использует |
|-----|----------|------------|
| `0` | Disabled | Агент спит |
| `1` | RuleOnly | Только rule-engine |
| `2` | Basic | Простой LLM + кэш |
| `3` | Full | Полный ReAct loop |

### Двухфазный конвейер

```
Phase 1: Decision (механика)
  - qwen:7b
  - <100ms p95
  - Строгий JSON
  - Fallback на rule-engine

Phase 2: Narrative (описание)
  - qwen:72b
  - <500ms async
  - Креативный
```

## 📈 Метрики

```go
// Статистика роутера
stats := router.Stats()
// {
//   "blueprints_count": 15,
//   "agents_count": 42,
//   "uptime_seconds": 3600,
// }

// Статистика воркера
poolStats := pool.Statistics()
// {
//   "workers": 10,
//   "total_processed": 15000,
//   "total_errors": 23,
//   "queue_utilization": 0.45,
// }
```

## 🧪 Тестирование

```bash
cd shared/agent
go test -v ./...
go test -cover ./...
go test -bench=.
```

## 🔗 Интеграция

### С EventBus

```go
import "multiverse-core/shared/eventbus"

bus.Subscribe("player.*", func(e eventbus.Event) {
    router.RouteEvent(ctx, toAgentEvent(e))
})
```

### С Memory Store

```go
import "multiverse-core/shared/agent"

memory := agent.ChromaMemoryStore{}
agent.Memory() = memory
```

### С LLM Gateway

```go
llmClient := NewQwenClient("qwen:default")
pipeline := agent.NewTwoPhasePipeline(llmClient, config)
```

## 📚 Документация

- [User Stories](../agent-gm-research/01_user_stories.md)
- [System Analysis](../agent-gm-research/02_system_analysis.md)
- [Roadmap](../agent-gm-research/03_conclusions_roadmap.md)
- [Migration Guide](MIGRATION.md)

## 🎯 Дорожная карта

| Фаза | Задачи | Срок |
|------|--------|------|
| **Phase 1** | ✅ Базовые типы и интерфейсы | 3 дня |
| **Phase 2** | ✅ Router + Lifecycle + Worker | 2 недели |
| **Phase 3** | ✅ MD парсер + форматы | 1 неделя |
| **Phase 4** | ✅ Двухфазный конвейер | 2 недели |
| **Phase 5** | 🚧 Интеграция | 3-4 недели |

## 👥 Контакты

- Архитектор: Алексей (alexey@multiverse-core)
- AI Engineer: [назначить]
- Backend Lead: [назначить]

## 📝 License

MIT

---

**Status**: Development | **Version**: 0.4.0 | **Last Updated**: 2026-04-11
