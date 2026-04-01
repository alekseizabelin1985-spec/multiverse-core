# Миграция на структурированные события

## Обзор изменений

**Дата**: 2026-04-01
**Причина**: Улучшение семантики ID, поддержка вложенной структуры, LLM-ready формат

---

## Что изменилось

### Старый формат (плоский)

```json
{
  "entity_id": "player-123",
  "entity_type": "player",
  "target_id": "region-456",
  "world_id": "world-789",
  "weather_change": "шторм"
}
```

### Новый формат (структурированный)

```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася",
    "world": {
      "id": "world-789"
    }
  },
  "target": {
    "entity": {
      "id": "region-456",
      "type": "region",
      "name": "Темный лес"
    }
  },
  "weather.change.to": "шторм",
  "weather.change.in.region.id": "region-456"
}
```

---

## API для работы с новым форматом

### 1. Типобезопасные структуры

```go
import "multiverse-core.io/shared/eventbus"

// Создание события с сущностью
payload := eventbus.NewEventPayload().
    WithEntity("player-123", "player", "Вася").
    WithTarget("region-456", "region", "Темный лес").
    WithWorld("world-789")

// Добавление кастомных полей через dot notation
eventbus.SetNested(payload.GetCustom(), "weather.change.to", "шторм")
eventbus.SetNested(payload.GetCustom(), "weather.change.in.region.id", "region-456")

// Публикация
event := eventbus.NewStructuredEvent("player.action", "entity-actor", "world-789", payload)
bus.Publish(ctx, eventbus.TopicPlayerEvents, event)
```

### 2. Готовые функции для common событий

```go
// Создание entity.created события
eventbus.PublishEntityCreated(bus, "world-789", "player-123", "player", "Вася")

// Создание entity.updated события
eventbus.PublishEntityUpdated(
    bus,
    "world-789",
    "player-123",
    "player",
    "Вася",
    map[string]any{
        "level": 15,
        "xp": 2500,
    },
)

// Создание player.action события
eventbus.PublishActionEvent(
    bus,
    "world-789",
    "player-123",
    "use_skill",
    "npc-456",
    "npc",
    "Старейшина",
    map[string]any{
        "skill_id": "fireball",
        "cooldown": 5,
    },
)
```

### 3. Извлечение ID сущностей

```go
// Новая структура с fallback на старую
entity := eventbus.ExtractEntityID(payload)
if entity != nil {
    fmt.Println("Entity ID:", entity.ID)
    fmt.Println("Entity Type:", entity.Type)
    fmt.Println("Entity Name:", entity.Name)
    fmt.Println("World ID:", entity.World)
}

// Извлечение target entity
target := eventbus.ExtractTargetEntityID(payload)
if target != nil {
    fmt.Println("Target ID:", target.ID)
}

// Извлечение world ID
worldID := eventbus.ExtractWorldID(payload)
```

---

## Backward Compatibility

**ВНИМАНИЕ**: Система сохраняет полную совместимость со старыми событиями!

Все функции извлечения ID поддерживают:
- ✅ Новый формат: `entity.id`, `target.entity.id`
- ✅ Старый формат: `entity_id`, `target_id`, `player_id`, `npc_id`

```go
// Эти вызовы работают ОДНАКОВО для старого и нового формата
entity := eventbus.ExtractEntityID(payload)
target := eventbus.ExtractTargetEntityID(payload)
worldID := eventbus.ExtractWorldID(payload)
```

---

## Пример миграции

### До (старый код)

```go
func handleEntityCreated(ev eventbus.Event) {
    entityID, _ := ev.Payload["entity_id"].(string)
    entityType, _ := ev.Payload["entity_type"].(string)
    worldID, _ := ev.Payload["world_id"].(string)

    if entityID == "" {
        return
    }

    // Создание актора...
}
```

### После (новый код)

```go
func handleEntityCreated(ev eventbus.Event) {
    // Извлекаем entity с поддержкой новой структуры и fallback
    entity := eventbus.ExtractEntityID(ev.Payload)
    if entity == nil {
        log.Printf("entity missing in event")
        return
    }

    entityID := entity.ID
    entityType := entity.Type
    worldID := entity.World
    if worldID == "" {
        worldID = eventbus.ExtractWorldID(ev.Payload)
    }

    if entityID == "" {
        return
    }

    // Создание актора...
}
```

---

## Обновление LLM промтов

### Semantic Memory

Контекст теперь в формате:
```
{entity.id:type:name} {timestamp} {action} {target.entity.id:type:name}

Пример: "{player-123:player:Вася} {14:30} {вошел в} {region-456:region:Темный лес}"
```

### Narrative Orchestrator

События передаются в промты в формате:
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "action": "встретил NPC",
  "target": {
    "entity": {
      "id": "npc-456",
      "type": "npc",
      "name": "Старейшина"
    }
  }
}
```

---

## Чеклист миграции

### Phase 1: Shared Layer
- [x] `shared/eventbus/payload_types.go` — типы и builder
- [x] `shared/eventbus/nested_payload.go` — dot notation helpers
- [x] `shared/eventbus/types.go` — готовые функции
- [x] `shared/eventbus/README.md` — документация
- [x] `shared/eventbus/payload_types_test.go` — тесты (12 PASS)

### Phase 2: Producer (world-generator)
- [x] `publishWorldCreated` — новый формат
- [x] `publishWorldGenerated` — новый формат
- [x] `createRegionEntity` — новый формат
- [x] `createWaterEntity` — новый формат
- [x] `createCityEntity` — новый формат
- [x] `publishGeographyGeneratedEvent` — новый формат

### Phase 3: Consumers
- [x] Semantic Memory — `extractStructuredEntityID`
- [x] Entity Actor — все handlers
- [x] Narrative Orchestrator — entity extraction
- [x] Ban-of-World — violation events
- [x] Cultivation Module — progression events
- [x] City Governor — player ID extraction

### Phase 4: LLM Prompts
- [x] Semantic Memory context formatting — обновлено
- [x] Narrative Orchestrator events — JSON формат сохранён

---

## Примеры событий

### Entity Created (новый формат)

```json
{
  "entity": {
    "id": "world-123",
    "type": "world",
    "world": {
      "id": "world-123"
    }
  },
  "payload": {
    "seed": "my-world",
    "theme": "cultivation"
  }
}
```

### Player Action (новый формат)

```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "target": {
    "entity": {
      "id": "npc-456",
      "type": "npc",
      "name": "Старейшина"
    }
  },
  "action": "use_skill",
  "skill_id": "fireball"
}
```

### Weather Event (с вложенной структурой)

```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася",
    "world": {
      "id": "world-789"
    }
  },
  "weather.change.to": "шторм",
  "weather.change.in.region.id": "region-456",
  "weather.previous.condition": "ясно",
  "weather.previous.temperature.value": 25.5
}
```

---

## Тестирование

```bash
# Проверка компиляции
go build ./shared/... ./services/...

# Запуск тестов
go test ./shared/eventbus/... -v
go test ./services/world-generator/... -v

# Запуск инфраструктуры
docker-compose up redpanda minio chromadb neo4j

# Проверка событий в Redpanda
rpk topic consume world_events
```

---

## Поддержка и вопросы

- Документация: [`shared/eventbus/README.md`](README.md)
- Тесты: [`shared/eventbus/payload_types_test.go`](payload_types_test.go)
- Issue tracker: GitHub issues
