# 🎭 EntityActor

> **EntityActor — это сервис управления "акторами" (жизненными сущностями) в мире.**

## 🎯 Назначение

EntityActor управляет жизненным циклом акторов — сущностей, которые обладают поведением и взаимодействуют с миром:
- Игроки (player actors)
- NPC (npc actors)
- Животные (animal actors)
- Special entities (magical creatures, etc.)

### Основные функции:
- Создание и уничтожение акторов
- Обработка действий акторов
- Управление состоянием акторов
- Поддержка skills, items, combat

## 📝 Новая структура событий

### Поддержка структурированных ID

Сервис полностью поддерживает **новый формат событий** с вложенной структурой `entity.id`, а также сохраняет **backward compatibility** со старым плоским форматом.

#### Новый формат (рекомендуемый)
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
      "id": "npc-456",
      "type": "npc",
      "name": "Старейшина"
    }
  },
  "action": "use_skill",
  "skill_id": "fireball"
}
```

#### Старый формат (поддерживается)
```json
{
  "entity_id": "player-123",
  "entity_type": "player",
  "target_id": "npc-456",
  "target_type": "npc",
  "skill_id": "fireball"
}
```

## 🔄 Жизненный цикл

1. Получает событие `entity.created` из EntityManager или WorldGenerator
2. Создает actor в памяти
3. Обрабатывает действия игрока (`player.action`)
4. Обрабатывает изменения состояния (`entity.state_changed`)
5. Обрабатывает удаление (`entity.deleted`)
6. Публикует события о состояниях актора

## 📡 Обработка событий

### Подписка на события:

#### Входящие:
- `entity.created` — создание сущности
- `entity.updated` — обновление сущности
- `entity.deleted` — удаление сущности
- `entity.travelled` — перемещение сущности
- `entity.state_changed` — изменение состояния
- `entity.snapshot` — снимок состояния

#### Игровые события:
- `player.action` — действие игрока
- `player.moved` — перемещение игрока
- `player.used_skill` — использование навыка
- `player.used_item` — использование предмета
- `player.interacted` — взаимодействие
- `combat.started` — начало боя
- `combat.ended` — конец боя
- `combat.damage_dealt` — нанесение урона
- `npc.action` — действие NPC
- `npc.dialogue` — диалог с NPC

### Публикация событий:
- Публикует события о состояниях акторов
- Обработанные действия игроков

## 📊 Примеры событий

### Entity Created (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася",
    "world": {
      "id": "world-789"
    }
  }
}
```

### Player Action (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "action": "use_skill",
  "target": {
    "entity": {
      "id": "npc-456",
      "type": "npc"
    }
  },
  "skill_id": "fireball",
  "target_entity_id": "npc-456"
}
```

### Player Used Skill (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "skill": {
    "id": "fireball",
    "level": 5
  },
  "target": {
    "entity": {
      "id": "enemy-789",
      "type": "enemy"
    }
  }
}
```

### Entity Moved (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "from": {
    "x": 45.0,
    "y": 67.0
  },
  "to": {
    "x": 48.5,
    "y": 70.2
  }
}
```

### Combat Damage (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player"
  },
  "attacker": {
    "entity": {
      "id": "enemy-456",
      "type": "enemy"
    }
  },
  "damage": 25,
  "damage_type": "physical",
  "target": {
    "entity": {
      "id": "player-123",
      "type": "player"
    }
  }
}
```

## 🌐 Интеграция

### Входящие интеграции:
- **EntityManager**: получение событий создания/удаления сущностей
- **CityGovernor**: информация о гражданах города
- **BanOfWorld**: проверка действий на нарушения
- **CultivationModule**: прогресс культивации
- **SemanticMemory**: индексация событий
- **NarrativeOrchestrator**: повествовательный контекст

### Исходящие интеграции:
- Публикация событий о состояниях акторов

## ✅ Преимущества

- **Типобезопасность**: поддержка новой структуры `entity.id`
- **Гибкость**: fallback на старый формат для совместимости
- **Масштабируемость**: легкое добавление новых типов акторов
- **Производительность**: in-memory storage для быстрого доступа
- **Полная совместимость**: backward compatibility с существующими событиями

## 🔧 Техническая реализация

### Архитектура:
- Сервис реализован в пакете `services/entityactor`
- Использует `eventbus.EventBus` для подписки на события
- Подписывается на все игровые события
- In-memory storage акторов

### Поддерживаемые типы акторов:
- `player` — игровые персонажи
- `npc` — non-player characters
- `animal` — животные
- `enemy` — вражеские сущности
- `custom` — пользовательские типы

## 🛠️ Конфигурация

### Переменные окружения:
- `KAFKA_BROKERS` — адреса брокеров Kafka/Redpanda
- `REDIS_ENDPOINT` — адрес Redis (опционально, для persistence)

### Значения по умолчанию:
- `KAFKA_BROKERS`: `localhost:9092`
- `REDIS_ENDPOINT`: `localhost:6379`

## 📊 Мониторинг

### Метрики:
- Количество активных акторов
- Количество обработанных событий
- Среднее время обработки события
- Количество ошибок

### Методы мониторинга:
- Логирование событий
- Сбор метрик через Prometheus