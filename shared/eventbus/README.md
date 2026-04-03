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

### Извлечение данных: сущности, скоупы, мир, универсальный доступор

```go
// Извлечение entity с поддержкой новой структуры и fallback
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

// Извлечение world ID (новая: world.id, старая: world_id)
worldID := eventbus.ExtractWorldID(payload)

// Извлечение scope (новая: scope: {id, type}, старая: scope_id, scope_type)
scope := eventbus.ExtractScope(payload)
if scope != nil {
    fmt.Println("Scope ID:", scope.ID)
    fmt.Println("Scope Type:", scope.Type)  // solo, group, city, region, quest
}
```

### Универсальный доступ по dot-путям (через jsonpath)

> 💡 `eventbus.PathAccessor` — это type alias на `jsonpath.Accessor` из универсального пакета `shared/jsonpath`.
> Все методы делегируются к нему — можно использовать любые фичи jsonpath.

```go
// Создаём аксессор для payload:
accessor := eventbus.NewPathAccessor(payload)  // alias на jsonpath.New()
// Или через метод события (рекомендуется):
accessor := event.Path()  // возвращает *jsonpath.Accessor

// Извлечение примитивных типов по пути:
entityID, ok := accessor.GetString("entity.id")           // "player-123"
scopeType, ok := accessor.GetString("scope.type")         // "group"
worldID, ok := accessor.GetString("world.id")             // "world-789"
level, ok := accessor.GetInt("entity.metadata.level")     // 15
temperature, ok := accessor.GetFloat("weather.temp.value") // 25.5
isActive, ok := accessor.GetBool("entity.active")         // true

// Извлечение сложных типов:
metadata, ok := accessor.GetMap("entity.metadata")        // map[string]any
inventory, ok := accessor.GetSlice("player.inventory")    // []any

// Быстрая проверка существования:
if accessor.Has("quest.objectives") {
    // Обработка квеста...
}
```

### Примеры использования в хендлерах событий с полной иерархией:
```go
func handlePlayerAction(event eventbus.Event) {
    // Универсальный доступ через встроенный PathAccessor:
    pa := event.Path()
    
    // Извлечение данных по иерархическим путям:
    entityID, _ := pa.GetString("entity.id")
    entityType, _ := pa.GetString("entity.type")
    scopeID, _ := pa.GetString("scope.id")
    scopeType, _ := pa.GetString("scope.type")  // solo/group/city/region/quest
    worldID, _ := pa.GetString("world.id")
    
    // Кастомные поля через dot-notation:
    action, _ := pa.GetString("action")
    skillID, _ := pa.GetString("skill.id")
    targetID, _ := pa.GetString("target.entity.id")
    
    // Метрики/статы:
    damage, _ := pa.GetFloat("combat.damage.value")
    cooldown, _ := pa.GetInt("skill.cooldown")
    
    // Логика обработки...
}

// Создание события с полной иерархией:
event := eventbus.NewStructuredEvent(
    "player.entered_region",
    "entity-actor",
    "world-789",
    eventbus.NewEventPayload().
        WithEntity("player-123", "player", "Вася").
        WithTarget("region-456", "region", "Темный лес").
        WithWorld("world-789").
        WithScope("solo-abc", "solo"),  // Новый метод для scope!
).WithCustom(map[string]any{
    "entry.reason": "quest_trigger",
    "weather.change.to": "шторм",  // dot-notation в custom полях!
})
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
