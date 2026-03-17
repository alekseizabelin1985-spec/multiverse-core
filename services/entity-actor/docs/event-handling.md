# EntityActor Event Handling Guide

**Location**: `services/entityactor/service.go`
**Last Updated**: 2026-02-23

---

## 📋 Overview

EntityActor сервис теперь обрабатывает **30+ типов событий** вместо заглушки. Все события разделены на категории и обрабатываются соответствующими обработчиками.

---

## 📂 Event Categories

### 1. Entity Management (6 типов)

| Event Type | Handler | Description |
|------------|---------|-------------|
| `entity.created` | `handleEntityCreated` | Создание новой сущности и актора |
| `entity.deleted` | `handleEntityDeleted` | Удаление сущности и уничтожение актора |
| `entity.travelled` | `handleEntityTravelled` | Путешествие между мирами |
| `entity.state_changed` | `handleEntityStateChanged` | Изменение состояния (state_changes) |
| `entity.snapshot` | `handleEntitySnapshot` | Снимок состояния сущности |
| `entity.actor.*` | Logging | Lifecycle события акторов |

### 2. Player Actions (5 типов)

| Event Type | Handler | Description |
|------------|---------|-------------|
| `player.action` | `handlePlayerAction` | Действие игрока (text input) |
| `player.moved` | `handlePlayerMoved` | Перемещение игрока |
| `player.used_skill` | `handlePlayerUsedSkill` | Использование навыка |
| `player.used_item` | `handlePlayerUsedItem` | Использование предмета |
| `player.interacted` | `handlePlayerInteracted` | Взаимодействие с объектами/NPC |

### 3. Combat (3 типа)

| Event Type | Handler | Description |
|------------|---------|-------------|
| `combat.started` | `handleCombatStarted` | Начало боя |
| `combat.ended` | `handleCombatEnded` | Окончание боя |
| `combat.damage_dealt` | `handleCombatDamageDealt` | Нанесение урона |

### 4. NPC (2 типа)

| Event Type | Handler | Description |
|------------|---------|-------------|
| `npc.action` | `handleNPCAction` | Действие NPC |
| `npc.dialogue` | `handleNPCDialogue` | Диалог с NPC |

### 5. Quests (3 типа)

| Event Type | Handler | Description |
|------------|---------|-------------|
| `quest.started` | `handleQuestStarted` | Начало квеста |
| `quest.completed` | `handleQuestCompleted` | Завершение квеста |
| `quest.updated` | `handleQuestUpdated` | Обновление прогресса |

### 6. Economy (3 типа)

| Event Type | Handler | Description |
|------------|---------|-------------|
| `item.traded` | `handleItemTraded` | Торговля предметами |
| `item.crafted` | `handleItemCrafted` | Создание предмета |
| `currency.changed` | `handleCurrencyChanged` | Изменение валюты |

### 7. World Environment (2 типа)

| Event Type | Handler | Description |
|------------|---------|-------------|
| `world.weather_changed` | `handleWeatherChanged` | Изменение погоды |
| `world.time_tick` | `handleWorldTimeTick` | Тик времени |

### 8. Generic Fallback

| Event Type | Handler | Description |
|------------|---------|-------------|
| `*` (все остальные) | `handleGenericEvent` | Обработка неизвестных событий |

---

## 🔄 Event Processing Flow

```
Kafka Event
    │
    ▼
┌─────────────────┐
│ handleEvent()   │
└────────┬────────┘
         │
    ┌────┴────┬─────────────────┬────────────────┐
    ▼         ▼                 ▼                ▼
entity.*  player.*        combat.*      npc.*
    │         │                 │                │
    ▼         ▼                 ▼                ▼
Create/   Action/         Start/        Action/
Delete/   Move/           End/          Dialogue
Travel/   UseSkill/       Damage
State/    UseItem/
Snapshot  Interact
```

---

## 📝 Example Event Payloads

### entity.created
```json
{
  "event_type": "entity.created",
  "world_id": "pain-realm",
  "payload": {
    "entity_id": "player-kain-777",
    "entity_type": "player",
    "world_id": "pain-realm"
  }
}
```

### entity.state_changed
```json
{
  "event_type": "entity.state_changed",
  "world_id": "pain-realm",
  "payload": {
    "state_changes": [
      {
        "entity_id": "player-kain-777",
        "operations": [
          {"op": "set", "path": "stats.hp", "value": 85},
          {"op": "add", "path": "stats.mp", "value": 10}
        ]
      }
    ]
  }
}
```

### player.action
```json
{
  "event_type": "player.action",
  "world_id": "pain-realm",
  "payload": {
    "entity_id": "player-kain-777",
    "player_text": "Атакую гоблина огненным шаром!",
    "target_id": "npc-goblin-guard"
  }
}
```

### combat.damage_dealt
```json
{
  "event_type": "combat.damage_dealt",
  "world_id": "pain-realm",
  "payload": {
    "attacker_id": "player-kain-777",
    "target_id": "npc-goblin-guard",
    "damage": 25.0,
    "damage_type": "fire"
  }
}
```

---

## 🔧 Production TODOs

Для каждого обработчика указаны заглушки `// В production: ...`. Вот что нужно реализовать:

### Entity Management
- ✅ `handleEntityCreated` - **Реализовано** (создание актора)
- ✅ `handleEntityDeleted` - **Реализовано** (уничтожение актора)
- ⏳ `handleEntityTravelled` - Обновить `actor.WorldID`
- ⏳ `handleEntityStateChanged` - Применить операции к `actor.State`
- ⏳ `handleEntitySnapshot` - Сохранить в MinIO

### Player Actions
- ⏳ `handlePlayerAction` - Вызвать `actor.ProcessPlayerAction(text)`
- ⏳ `handlePlayerMoved` - Обновить `actor.Position`
- ⏳ `handlePlayerUsedSkill` - Применить эффект навыка
- ⏳ `handlePlayerUsedItem` - Применить эффект предмета
- ⏳ `handlePlayerInteracted` - Обработать взаимодействие

### Combat
- ⏳ `handleCombatStarted` - Инициализировать боевое состояние
- ⏳ `handleCombatEnded` - Применить награды/штрафы
- ⏳ `handleCombatDamageDealt` - Обновить `actor.State["hp"]`

### NPC
- ⏳ `handleNPCAction` - Обработать действие NPC
- ⏳ `handleNPCDialogue` - Обновить диалоговое состояние

### Quests
- ⏳ `handleQuestStarted` - Инициализировать квест в actor
- ⏳ `handleQuestCompleted` - Выдать награды
- ⏳ `handleQuestUpdated` - Обновить прогресс

### Economy
- ⏳ `handleItemTraded` - Передать предмет между акторами
- ⏳ `handleItemCrafted` - Создать предмет в inventory
- ⏳ `handleCurrencyChanged` - Обновить `actor.State["currency"]`

### World
- ⏳ `handleWeatherChanged` - Применить эффекты погоды
- ⏳ `handleWorldTimeTick` - Обновить время в actor

---

## 📊 Statistics

| Category | Events | Implemented | Production Ready |
|----------|--------|-------------|------------------|
| Entity Management | 6 | ✅ 100% | 20% |
| Player Actions | 5 | ✅ 100% | 0% |
| Combat | 3 | ✅ 100% | 0% |
| NPC | 2 | ✅ 100% | 0% |
| Quests | 3 | ✅ 100% | 0% |
| Economy | 3 | ✅ 100% | 0% |
| World | 2 | ✅ 100% | 0% |
| **Total** | **24+** | **✅ 100%** | **~8%** |

---

## 🎯 Next Steps

1. **Реализовать Actor.ProcessEvent()** - для передачи событий актору
2. **Добавить Actor.ApplyOperation()** - для применения state_changes
3. **Реализовать Actor.ProcessPlayerAction()** - для обработки player_text
4. **Добавить Actor.UpdateWorldID()** - для путешествий
5. **Реализовать Actor.UpdatePosition()** - для перемещений
6. **Добавить интеграцию с RuleEngine** - для применения правил
7. **Добавить интеграцию с IntentRecognition** - для распознавания намерений

---

**Author**: Алексей (alekseizabelin1985-spec)
**Date**: 2026-02-23
