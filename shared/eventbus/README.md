# Event Bus

Event Bus предоставляет инфраструктуру для обмена событиями между микросервисами Multiverse-Core.

## Структура событий

### Новый формат (рекомендуемый)

События используют вложенную структуру для явной семантики ID:

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
  "custom_fields": {
    "weather.change.to": "шторм",
    "weather.change.in.region.id": "region-456"
  }
}
```

### Старый формат (backward compatibility)

Поддерживается старый плоский формат для совместимости:

```json
{
  "entity_id": "player-123",
  "entity_type": "player",
  "target_id": "region-456",
  "world_id": "world-789"
}
```

## API

### Создание событий с типобезопасным builder pattern

```go
// Создание события с сущностью
payload := eventbus.NewEventPayload().
    WithEntity("player-123", "player", "Вася").
    WithTarget("region-456", "region", "Темный лес").
    WithWorld("world-789")

// Добавление кастомных полей через dot notation
eventbus.SetNested(payload.GetCustom(), "weather.change.to", "шторм")
eventbus.SetNested(payload.GetCustom(), "weather.change.in.region.id", "region-456")

// Создание события
event := eventbus.NewStructuredEvent("player.action", "entity-actor", "world-789", payload)
bus.Publish(ctx, eventbus.TopicPlayerEvents, event)
```

### Готовые функции для common событий

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

### Извлечение ID сущностей

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

### Dot notation helpers

```go
// Установка вложенного поля
payload := make(map[string]any)
eventbus.SetNested(payload, "weather.change.to", "шторм")
eventbus.SetNested(payload, "weather.change.in.region.id", "region-456")

// Чтение из вложенного поля
to, ok := eventbus.GetNested(payload, "weather.change.to")
if ok {
    fmt.Println("Weather change to:", to)
}
```

## Форматирование для LLM контекста

```go
// Форматирование события для AI
context := eventbus.FormatEventContext(
    sourceID, sourceName, eventID, action, targetID, targetName, timestamp,
)
// Результат: "{event_123:14:30} {player_456:Вася} {event_123:вошел в} {region_789:Темный лес}"
```

## Примеры событий

### Entity created
```json
{
  "entity_id": "world-123",
  "entity_type": "world",
  "payload": {
    "seed": "my-world",
    "theme": "cultivation",
    "core": "Мир для культивации",
    "era": "древний",
    "unique_traits": ["magical_rivers", "floating_islands"],
    "plan": 0
  },
  "world_id": "world-123"
}
```

### Entity created (новый формат)
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

### Player action
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

## Миграция

1. **Phase 1**: Shared helpers (payload_types.go, nested_payload.go) - готово
2. **Phase 2**: world-generator использует новый формат - готово
3. **Phase 3**: Consumers обновляются с fallback на старый формат
4. **Phase 4**: Удаление fallback после миграции

## Best Practices

1. Используйте `WithEntity()`, `WithTarget()`, `WithWorld()` для типобезопасного создания событий
2. Используйте `SetNested()` для кастомных полей с любой глубиной вложенности
3. Используйте `ExtractEntityID()` и `ExtractTargetEntityID()` для извлечения ID с поддержкой обоих форматов
4. Для LLM контекста используйте `FormatEventContext()` с форматом `{entity.id:name}`
