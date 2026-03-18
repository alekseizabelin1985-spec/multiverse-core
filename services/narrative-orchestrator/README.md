# 🎭 NarrativeOrchestrator (Game Master, GM)
## Сводная архитектурная спецификация
*Версия 2.0 — С полными описаниями событий*

> **GM — это единый, параметризуемый, stateful-агент, который создаётся по событию и управляет повествованием для области.
> Он не решает, не оценивает, не интерпретирует. Он *наблюдает, собирает факты, передаёт их LLM — и публикует результат*.
> Вся творческая, драматургическая и стилистическая ответственность лежит на LLM.**

---

## 🎯 Назначение

- Генерирует иммерсивное повествование для любой области (`scope_id`)
- Сохраняет состояние сюжетной дуги между событиями и сессиями
- Использует:
  - **Semantic Memory Builder** — для контекста сущностей и мира,
  - **LLM** (через `/v1/chat/completions`) — для генерации повествования и последствий.

---

## 🔄 Жизненный цикл (управляется событиями)

| Событие | Топик | Действие |
|--------|-------|----------|
| `gm.created` | `eventbus.TopicSystemEvents` | Создание GM для `scope_id` (восстановление из снапшота или `new`) |
| `gm.deleted` | `eventbus.TopicSystemEvents` | Финальный снапшот → отписка от событий |
| `gm.merged` | `eventbus.TopicSystemEvents` | Объединение GM (зоны слияния) — обработка в начале следующего тика |
| `gm.split` | `eventbus.TopicSystemEvents` | Разделение GM — создание новых экземпляров |

→ Все `gm.*` события обрабатываются **в порядке поступления**, с сохранением causal context.

---

## 🧠 Состояние GM

- Хранится в памяти: `map[scope_id]*GMInstance`
- Содержит:
  - Текущее состояние сюжета (`NarrativeArc`),
  - Буфер накопленных событий,
  - Локальный `KnowledgeBase` (факты, канон, `last_mood`),
  - Конфигурация (из YAML).
- Регулярно сохраняется в MinIO (снапшоты).

---

## 📡 Обработка событий

1. Получает события из топиков:
   - `eventbus.TopicWorldEvents` — игровые события (`player.moved`, `weather.changed`),
   - `eventbus.TopicGameEvents` — сюжетные события (`combat.start`, `ritual.completed`),
   - `eventbus.TopicSystemEvents` — управляющие (`gm.*`, `time.syncTime`).
2. Агрегирует события по `scope_id`.
3. Запрашивает у **Semantic Memory Builder** (опционально):
   - Описание локаций (`GET /location/{id}`),
   - **Полные события** (`POST /v1/events-by-entities`) — с payloads и описаниями,
   - Историю сущностей (`GET /entity/{id}/history`).
4. Формирует промт для LLM (system + user messages) **с полными описаниями событий**.
5. Отправляет через **LLM Client** → `/v1/chat/completions`.
6. Публикует `new_events` в `eventbus.TopicWorldEvents`.

---

## 🌐 Интеграция

| Компонент | Интерфейс | Назначение |
|----------|-----------|------------|
| **Semantic Memory Builder** | HTTP (POST `/v1/events-by-entities`) | Получение полных событий с payloads |
| **Semantic Memory Builder** | HTTP (GET) | Дополнение контекста: локации, история |
| **LLM** | HTTP (`/v1/chat/completions`) | Генерация повествования и последствий |
| **Event Bus** | Kafka/NATS | Приём и публикация событий |
| **MinIO** | S3 API | Хранение снапшотов `KnowledgeBase` |

---

## 🛠️ Техническая реализация

- **Пакет**: `services/narrativeorchestrator`
- **Event Bus**: подписка на топики:
  - `eventbus.TopicSystemEvents`,
  - `eventbus.TopicNarrativeOutput`,
  - `eventbus.TopicWorldEvents`,
  - `eventbus.TopicGameEvents`.
- **Consumer Groups** (для гарантии одного GM на `scope_id`):
  - `narrative-system`  — для `time.syncTime`,
  - `narrative-orchestrator-group` — для `gm.*` и `time.syncTime`,
  - `narrative-world-group` — для `world_events`,
  - `narrative-game-group` — для `game_events`.

> 🔹 Все группы используют **partition key = `scope_id`** → один GM обрабатывает все события для области.

---

## 📝 Формирование промта

### Структура промта

**System prompt**:
- `<role>` — роль повествователя
- `<rules>` — правила генерации и ограничения
- `<canon>` — канонические факты мира (если есть)
- `<schema>` — JSON schema для валидации ответа

**User prompt**:
- `<facts>` — факты о мире и сущностях
- `<situation>` — текущая ситуация в области
- `<events>` — **полные описания событий** с формата:
  ```
  [ относительно_время ]:
  • [event_id] event_type: Человек-читаемое описание
  ```
- `<task>` — задача для LLM

### Пример событий в промте

```
<events>
[ через секунду ]:
• [evt-skil...] Skill Check: Бросок кубика d20 = 17
• [evt-dura...] Duration Expire: Исчезновение заклинания на Кейне

[ одновременно ]:
• [evt-move...] Entity.Moved: перемещение к врагу
• [evt-hlth...] Health.Update: урон 15 единиц
</events>
```

### Формирование описаний событий

Функция `formatEventDescription` извлекает информацию из payload события:

1. **Проверка явных полей**: `description`, `detail`
2. **Извлечение сущности**: `entity_id`, `player_id`, `actor_id`, `character_id`, `npc_id`
3. **Формирование действия**: `action`, `type`, `event_type` → капитализация
4. **Цель**: `target_id`
5. **Fallback**: `Событие {event_id[:8]}`

Пример:
```go
formatEventDescription(Event{
    EventType: "skill.check",
    Payload: map[string]interface{}{
        "entity_id": "player:kain-777",
        "action": "бросок кубика",
        "result": 17,
    },
})
// Возвращает: "player:kain-777: Бросок кубика"
```

---

## 🔧 Конфигурация

| Параметр | Описание | Значение по умолчанию |
|---------|----------|------------------------|
| `KAFKA_BROKERS` | Адрес Kafka | `localhost:9092` |
| `MINIO_ENDPOINT` | Адрес MinIO | `http://minio:9000` |
| `LLM_ENDPOINT` | Адрес LLM API | `http://ollama:11434/v1` |
| `SEMANTIC_MEMORY_ENDPOINT` | Адрес semantic-memory | `http://semantic-memory:8080` |

→ Все параметры — через переменные окружения.

---

## 📊 Мониторинг

| Метрика | Описание |
|---------|----------|
| `gm.active_total` | Количество активных GM |
| `gm.events_processed_total` | Всего обработано событий |
| `llm.calls_total` | Вызовы LLM |
| `llm.latency_ms` | Время ответа LLM (гистограмма) |
| `snapshot.save_success_rate` | Успешность сохранения снапшотов |

→ Экспорт: Prometheus.

---

## 📜 Гарантии системы

1. **Единая кодовая база** — все GM (world, region, player) — один бинарь.
2. **Контекстная целостность** — `KnowledgeBase` восстанавлируется дословно.
3. **Идемпотентность** — каждое событие имеет `event_id`, обрабатывается один раз.
4. **Масштабируемость** — GM живёт только пока нужен; 10⁴+ экземпляров.
5. **Слабая связанность** — взаимодействие только через события.
6. **Полные описания событий** — события передаются в промт с full payloads и читаемыми описаниями.

---

## 📁 Связанные документы

- [`GM_Configuration_Spec.md`](GM_Configuration_Spec.md)
- [`Prompt_Construction_Guide.md`](Prompt_Construction_Guide.md)
- [`KnowledgeBase_Schema.md`](KnowledgeBase_Schema.md)
- [`LLM_Client_Spec.md`](LLM_Client_Spec.md)
