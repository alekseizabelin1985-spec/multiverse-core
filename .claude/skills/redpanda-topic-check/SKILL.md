---
name: redpanda-topic-check
description: Проверка и управление темами Redpanda
---

Проверка наличия и конфигурации тем Redpanda для multiverse-core.

## Доступные темы

### Обязательные темы
- `player_events` - действия игроков
- `world_events` - изменения состояния мира
- `game_events` - игровая механика
- `system_events` - системные операции
- `scope_management` - жизненный цикл scope
- `narrative_output` - результаты нарратива

## Команды

### `/check-topics`
Проверить наличие всех обязательных тем

### `/create-topic <name>`
Создать новую тему

### `/list-topics`
Показать все темы и их статистику

### `/describe-topic <name>`
Подробная информация о теме

## Использование

```
/check-topics
/list-topics
/create-topic custom_events
/describe-topic player_events
```

## Конфигурация
- Redpanda: localhost:9092
- Admin API: localhost:9644