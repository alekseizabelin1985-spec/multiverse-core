# 🌱 CultivationModule

> **CultivationModule отвечает за систему культивации, прогресс игроков и ascension.**

## 🎯 Назначение

- Генерация "портретов Дао" на основе истории игрока
- Подготовка данных для ascension событий
- Отслеживание прогресса игрока в культивации
- Управление Dao paths и системами культивации
- Поддержка развития персонажей

## 📝 Новая структура событий

### Поддержка структурированных ID

```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "skill": "fire_breath",
  "cultivation_progress": 1500
}
```

## 🔄 Жизненный цикл

1. Получает события о действиях игрока через `player_events`
2. Отслеживает использование навыков и предметов
3. Обновляет прогресс культивации
4. Обрабатывает ascension события
5. Интегрирует новые Dao paths

## 🧠 Состояние CultivationModule

- Отслеживает прогресс культивации в реальном времени
- Отслеживает историю действий игрока
- Генерирует портреты Дао для ascension
- Не хранит долгосрочное состояние

## 📡 Обработка событий

### Входящие:
- `player.used_skill` — использование навыка
- `player.used_item` — использование предмета
- `entity.ascended` — ascension игрока
- `player.interacted` — взаимодействие с Dao

### Публикация событий:
- `cultivation.progress.updated` — обновление прогресса
- `cultivation.system.updated` — обновление системы культивации
- `dao.interaction.success` — успешное взаимодействие с Dao
- `dao.interaction.conflict` — конфликт с Dao

## 🌐 Интеграция

- **GameService**: взаимодействие с игроками
- **EntityManager**: данные о сущностях игрока
- **NarrativeOrchestrator**: контекст для повествования
- **BanOfWorld**: мониторинг целостности
- **EntityActor**: обработка действий игрока

## ✅ Преимущества

- Генерация персонализированных портретов Дао
- Поддержка ascension процесса
- Отслеживание прогресса игрока
- Интеграция с игровым процессом
- **Новая структура событий**: полная поддержка `entity.id`

## 📊 Примеры событий

### Skill Usage (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "skill": "meditation",
  "cultivation_progress": 50
}
```

### Cultivation Progress Updated (исходящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "skill_used": "fire_breath",
  "progress_gained": 150
}
```

### Ascension (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "from_plan": 5,
  "to_plan": 6
}
```

### Cultivation System Updated (исходящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "new_plan": 6,
  "system_type": "golden_elixir"
}
```

### Dao Interaction (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player"
  },
  "target_dao": "elemental_dao",
  "interaction_type": "merge"
}
```

## 🛠️ Техническая реализация

- Сервис реализован в пакете `services/cultivationmodule`
- Использует `eventbus.EventBus` для подписки на события
- Подписывается на `eventbus.TopicPlayerEvents`
- Обрабатывает события, связанные с развитием игрока

## 🔧 Конфигурация

- Переменные окружения: `KAFKA_BROKERS`
- По умолчанию: `localhost:9092`

## 📊 Мониторинг

- Количество сгенерированных портретов Дао
- Количество обработанных событий игрока
- Время генерации портретов
- Качество сгенерированных данных
- Прогресс культивации игроков