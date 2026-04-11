# Agent Package — Агентная архитектура Game Master

Базовый пакет для реализации агентной архитектуры согласно документации **Docs/agent-gm-research/**.

## 📁 Структура

```
shared/agent/
├── agent_types.go      # Основные типы и структуры
├── interfaces.go       # Интерфейсы для агентов, роутера, воркеров
├── blueprint_loader.go # Парсер YAML/MD блупринтов
└── README.md           # Эта документация
```

## 🎯 Компоненты

### Agent Types

Базовые типы для агентной системы:

- **AgentLevel** - иерархия агентов (Global → Domain → Task → Object)
- **AgentBlueprint** - декларативный блупринт (конфиг + промпт + ограничения)
- **AgentContext** - контекст выполнения агента
- **LODLevel** - уровень детализации (0-3)

### Interfaces

Ключевые интерфейсы:

- **Agent** - основной интерфейс агента
- **Router** - маршрутизация событий к агентам
- **Lifecycle** - управление жизненным циклом
- **Worker** - обработка агентов
- **MemoryStore** - векторная память (ChromaDB)
- **ToolRegistry** - реестр инструментов

## 📖 Пример блупринта

```yaml
# domain-dark-forest.md
name: domain-dark-forest
version: "1.0"
description: Агенты региона "Тёмный лес"

trigger:
  type: event
  event_name: player.entered_region
  conditions:
    - field: player_count
      operator: "=="
      value: 1

constraints:
  max_instances: 1
  priority: 50

ttl: "1h"

llm:
  model: qwen:7b
  temperature: 0.7
  max_tokens: 1024

tools:
  - name: search_entities
  - name: spawn_npc
  - name: modify_terrain
```

## 🚀 Использование

### Парсинг блупринтов

```go
import "multiverse-core/shared/agent"

parser := agent.NewYAMLParser()
bp, err := parser.ParseFile("domain-dark-forest.md")
if err != nil {
    log.Fatal(err)
}
```

### Загрузка всех блупринтов из директории

```go
blueprints, err := agent.LoadBlueprintsFromDir("./blueprints")
if err != nil {
    log.Fatal(err)
}
```

### Контекст агента

```go
ctx := &agent.AgentContext{
    AgentID:   "agent-123",
    AgentType: "domain",
    Level:     agent.LevelDomain,
    ScopeID:   "dark-forest-01",
    MemoryRef: "chroma:region:dark-forest-01",
    LOD:       agent.LODBasic,
}
```

## 🔄 Связи с другими компонентами

- **Events** - агентные события публикуются в Redpanda/Kafka
- **Memory** - векторная память через ChromaDB
- **LLM Gateway** - маршрутизация к Qwen моделям

## 📝 Дорожная карта

Phase 1 (текущая): ✅ Базовые типы и интерфейсы
Phase 2: Router + Lifecycle + Worker
Phase 3: Blueprint парсер + MD форматы
Phase 4: Двухфазный конвейер LLM
Phase 5: Интеграция с narrative-orchestrator

## 🔗 Ссылки

- [01_user_stories.md](../agent-gm-research/01_user_stories.md) - пользовательские истории
- [02_system_analysis.md](../agent-gm-research/02_system_analysis.md) - архитектура
- [03_conclusions_roadmap.md](../agent-gm-research/03_conclusions_roadmap.md) - дорожная карта
