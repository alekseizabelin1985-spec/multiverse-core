# ⚖️ BanOfWorld

> **BanOfWorld отвечает за мониторинг целостности миров и применение мер при нарушении.**

## 🎯 Назначение

- Мониторинг "резонанса" миров для обеспечения целостности
- Обнаружение и реагирование на нарушения целостности
- Применение мер по восстановлению целостности
- Поддержка онтологической целостности

## 📝 Новая структура событий

### Поддержка структурированных ID

Сервис полностью поддерживает **новый формат событий** с вложенной структурой `entity.id`, а также сохраняет **backward compatibility** со старым форматом.

#### Новый формат (рекомендуемый)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "skill": "fire_breath",
  "violation_type": "elemental_conflict"
}
```

#### Старый формат (поддерживается)
```json
{
  "entity_id": "player-123",
  "player_id": "player-123",
  "skill": "fire_breath",
  "violation_type": "elemental_conflict"
}
```

## 🔄 Жизненный цикл

1. Получает события о действиях игроков через `world_events` и `player_events`
2. Проверяет действия на соответствие онтологическим правилам мира
3. При нарушении — публикует событие `violation.detected`
4. Применяет меры (transform, punish, teleport)

## 🧠 Состояние BanOfWorld

- Не хранит долгосрочное состояние
- Реагирует на события в режиме реального времени
- Поддерживает world-specific правила

## 📡 Обработка событий

### Входящие:
- `player.used_skill` — использование навыка
- `player.used_item` — использование предмета
- `player.moved` — перемещение
- `entity.travelled` — путешествие сущности

### Публикация событий:
- `violation.detected` — нарушение целостности
- `skill.transformed` — трансформация навыка
- `player.punished` — наказание игрока

## 🌐 Интеграция

- **WorldGenerator**: получение информации о мире
- **EntityManager**: состояние сущностей
- **EntityActor**: игровые действия
- **SemanticMemory**: семантический контекст
- **OntologicalArchivist**: онтологические схемы
- **CultivationModule**: проверки для культивации

## 📊 Примеры событий

### Skill Usage (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "skill": "fire_breath"
}
```

### Violation Detected (исходящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "skill": "fire_breath",
  "violation_type": "elemental_conflict",
  "original_event": "evt-123abc"
}
```

### Skill Transformed (исходящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "original": "fire_breath",
  "transformed": "scream_of_pain",
  "reason": "resonance_with_core"
}
```

### Player Punished (исходящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "punishment": "memory_corruption",
  "duration": "1h",
  "reason": "violation_of_memory_laws"
}
```

### Movement Violation (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player"
  },
  "destination": "outside-prison"
}
```

## ✅ Преимущества

- Мониторинг целостности миров
- Автоматическое реагирование на нарушения
- Поддержка онтологической целостности
- Реальное время реакции

## 🛠️ Техническая реализация

- Сервис реализован в пакете `services/banofworld`
- Использует `eventbus.EventBus` для подписки на события
- Подписывается на `eventbus.TopicSystemEvents`
- Использует метрики "резонанса" для оценки

## 🔧 Конфигурация

- Переменные окружения: `KAFKA_BROKERS`, `RESONANCE_THRESHOLD`
- По умолчанию: `localhost:9092`, `0.8`

## 📊 Мониторинг

- Количество проверок целостности
- Количество обнаруженных нарушений
- Частота применения мер
- Среднее значение "резонанса"