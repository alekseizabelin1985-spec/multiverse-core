# События генерации мира

JSON события для отправки в шину событий Redpanda для генерации виртуальных миров.

## Отправка событий

События отправляются в топик `system_events` с типом `world.generation.requested`.

### Пример отправки через eventbus:

```go
event := eventbus.NewStructuredEvent(
    "world.generation.requested",
    "player-service",
    worldID,
    payload,
)
bus.Publish(ctx, eventbus.TopicSystemEvents, event)
```

## Доступные сценарии

### 1. Культивационный мир [cultivation-world-event.json](cultivation-world-event.json)
- **Тема**: Cultivation (культивация)
- **Масштаб**: Large
- **Особенности**: Магические реки, летающие острова, древние храмы, духи-зверы, система сект
- **Ограничения**: Нет современных технологий, нет оружия

### 2. Steampunk мир [steampunk-world-event.json](steampunk-world-event.json)
- **Тема**: Steampunk
- **Масштаб**: Medium
- **Особенности**: Паровые машины, механические големы, воздушные корабли, индустриальные города
- **Ограничения**: Нет магии, нет электричества

### 3. Dark Fantasy мир [dark-fantasy-world-event.json](dark-fantasy-world-event.json)
- **Тема**: Dark Fantasy
- **Масштаб**: Large
- **Особенности**: Демонические порталы, проклятые леса, тёмные культы
- **Ограничения**: Нет великого добра, нет мирных решений

### 4. Sci-Fi мир [scifi-world-event.json](scifi-world-event.json)
- **Тема**: Sci-Fi
- **Масштаб**: Large
- **Особенности**: Киборги, ИИ, космические флоты, квантовые технологии
- **Ограничения**: Нет магии, нет средневековых технологий

### 5. Случайный мир [random-world-event.json](random-world-event.json)
- **Режим**: Random
- **Особенности**: Полная процедурная генерация без пользовательского контекста

### 6. Мир с ограничениями [constrained-world-event.json](constrained-world-event.json)
- **Режим**: Contextual
- **Особенности**: Пользовательские ограничения на количество объектов генерации
- **Параметры**: max_regions, max_water_bodies, max_cities, дополнительные опции

## Структура события

```json
{
  "id": "уникальный-идентификатор",
  "event_type": "world.generation.requested",
  "timestamp": "ISO-8601",
  "source": "player | system",
  "payload": {
    "seed": "строка-семена",
    "mode": "contextual | random",
    "user_context": {
      "description": "свободное описание",
      "theme": "тема мира",
      "key_elements": ["элемент1", "элемент2"],
      "scale": "small | medium | large",
      "restrictions": ["запрет1", "запрет2"]
    },
    "constraints": {
      "max_regions": 10,
      "max_water_bodies": 8,
      "max_cities": 12,
      "include_dungeons": true
    }
  }
}
```

## Параметры

### `mode`
- **contextual**: Генерация на основе пользовательского описания
- **random**: Полная процедурная генерация

### `scale` (только для contextual)
- **small**: 2-3 региона, 1-2 водных объекта, 1-2 города
- **medium**: 3-5 регионов, 2-4 водных объекта, 2-4 города
- **large**: 5-8 регионов, 4-7 водных объектов, 4-8 городов

### `theme`
Поддерживаемые темы: `cultivation`, `steampunk`, `dark_fantasy`, `sci-fi`, `mythology`, `cyberpunk`, `nature_fantasy`, `post_apocalyptic`

## Примеры тематик

| Тема | Описание |
|------|----------|
| cultivation | Мир культивации с ци, духами, сектами |
| steampunk | Victorian эпоха, паровые машины, механика |
| dark_fantasy | Тёмное фэнтези, демоны, проклятия |
| sci-fi | Космос, ИИ, киборги, технологии |
| mythology | Древние боги, мифические существа |
| cyberpunk | Неоновые города, корпорации, хакеры |
| nature_fantasy | Гармония с природой, друиды |
| post_apocalyptic | Выживание после катастрофы |

## Полный набор событий

Все сценарии также доступны в [world-generation-events.json](world-generation-events.json) в формате одного файла со всеми вариантами.
