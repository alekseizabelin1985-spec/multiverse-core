# `shared/eventbus`

Конверт события платформы и интерфейсы шины и журнала, через которые общаются все контексты.

Контракт: `Docs/dev-team/architecture/contracts.md` **C-01 v1.1** · ADR-007 ·
`architecture/components/foundation.md` §5. Любое изменение публичного API этого пакета —
**только через системного архитектора** (`contracts.md` §16).

## Что здесь есть

| Файл | Содержимое |
|---|---|
| `types.go` | `Event`, `Meta`, `AgentRef`, конструкторы `NewRoot`/`Derive`, опции `DeriveOption` (`With*`) |
| `sources.go` | `SetIDSource`, `SetClock`, `SetRegistry`, `SequenceIDs` — источники, которые ставит процесс |
| `registry.go` | `Registry`, `TypeSpec`, `Policy`, `Route`, `Event.ValidateEnvelope` |
| `bus.go` | `Bus`, `Journal`, `Handler`, `Middleware`, `Position`, `PositionFromContext` |
| `delivery.go` | `Delivery` — общая для всех реализаций логика чтения: валидация, ретраи, `dead_letters` |
| `dedup.go` | `Dedup` — LRU по `event.id` у потребителя |
| `kafka.go` | реализация `Bus` + `Journal` на `segmentio/kafka-go` (Redpanda) |
| `topics.go` | имена восьми топиков MVP-1 |
| `payload_types.go`, `nested_payload.go`, `relations.go` | билдер payload, доступ по dot-путям, связи для графа |

In-memory реализация (`membus`) живёт в `shared/testkit` (T-014) и использует те же `Route` и
`Delivery`, поэтому ведёт себя одинаково с kafka-адаптером — это проверяет contract-тест F-5t.

## Конверт

Сквозные поля — в конверте (`meta`), payload — только доменные поля по схеме типа (ADR-007 п. 1):

```json
{ "id": "uuid", "type": "combat.decided", "timestamp": "2026-09-09T12:00:00Z", "source": "core/swarm",
  "world": { "entity": { "id": "dark-forest-world", "type": "world" } },
  "scope": { "id": "solo:player-A", "type": "solo" },
  "meta": { "schema_version": 1, "correlation_id": "…", "causation_id": "…",
            "causation_type": "player.attacked", "actor_kind": "human",
            "agent": { "id": "encounter-wolf:solo:player-A", "level": "task",
                       "blueprint": "encounter-wolf", "blueprint_version": "1.0" },
            "replay": false, "locale": "ru", "gm_path": "agent" },
  "payload": { "…": "…" }, "relations": [] }
```

## Создание событий

```go
// Корень цепочки: correlation_id = собственный id, причины нет.
root := eventbus.NewRoot("player.attacked", "gateway", worldID,
    &eventbus.ScopeRef{ID: "solo:player-A", Type: "solo"},
    eventbus.ActorHuman,
    map[string]any{"target": map[string]any{"entity": map[string]any{"id": "wolf-1", "type": "npc"}}})

// Следствие: наследует world, scope, correlation_id, actor_kind, locale, gm_path
// и timestamp причины; causation_id/causation_type указывают на неё.
decided := eventbus.Derive(root, "combat.decided", "core/swarm", payload,
    eventbus.WithAgent(eventbus.AgentRef{ID: "encounter-wolf:solo:player-A",
        Level: "task", Blueprint: "encounter-wolf"}))

if err := bus.Publish(ctx, decided); err != nil { /* неизвестный тип, политика, схема */ }
```

`meta.gm_path` обязателен (`api-contracts.md` §2.1). `NewRoot` ставит `agent` — единственный путь
нового кода; издатель профиля `legacy` переопределяет опцией `eventbus.WithGMPath(eventbus.GMPathLegacy)`,
`Derive` наследует значение причины. Конверт с пустым `gm_path` не публикуется и не принимается при
чтении (`ValidateEnvelope`).

`Derive` наследует `timestamp` причины **всегда**, не только в replay: иначе две прогонки одной
записи дают разные байты (NFR-061).

Идентификаторы, часы и реестр — пакетные источники, которые ставит процесс:

```go
eventbus.SetIDSource(uuid.NewString)       // в тестах — eventbus.SequenceIDs("ev")
eventbus.SetClock(clock.Real{})            // в тестах — clock.NewManual(t0)
eventbus.SetRegistry(contracts.Registry()) // shared/contracts, T-006
```

## Чтение payload

```go
pa := ev.Path()                            // shared/jsonpath
entityID, _ := pa.GetString("entity.entity.id")
worldID := eventbus.GetWorldIDFromEvent(ev)
scope := eventbus.GetScopeFromEvent(ev)
```

## Шина и журнал

```go
bus, err := eventbus.NewKafka(eventbus.KafkaConfig{
    Brokers:  brokers,
    Registry: reg, // shared/contracts
    // Валидация при чтении включена по умолчанию (нулевое значение = проверять).
    // Переменную MV_BUS_VALIDATE_ON_READ читает процесс (shared/env, T-007) и
    // передаёт инверсию: SkipValidateOnRead = !MV_BUS_VALIDATE_ON_READ.
    SkipValidateOnRead: !validateOnRead,
    Log:                logger,
})

// Живая подписка: consumer group {процесс}.{контекст}, at-least-once.
err = bus.Subscribe(ctx, eventbus.TopicSystemEvents, "core.state", handler)

// Догон с курсора снапшота: чтение по офсетам без группы.
end, _ := bus.End(ctx, eventbus.TopicSystemEvents)
next, _ := bus.ReadRange(ctx, eventbus.TopicSystemEvents, cursor, end, handler)
err = bus.Tail(ctx, eventbus.TopicSystemEvents, next, handler)
```

Внутри обработчика доступна позиция текущего события — её записывают в снапшот:

```go
if pos, ok := eventbus.PositionFromContext(ctx); ok {
    snapshot.Cursor[pos.Topic] = pos.Offset
}
```

Дедуп — обязанность потребителя, а не шины:

```go
seen := eventbus.NewDedup(eventbus.DefaultDedupCapacity)
if seen.Seen(ev.ID) { return nil }   // повтор at-least-once
// seen.IDs() сериализуется в снапшот, seen.Restore(ids) — при старте
```

## Гарантии и правила

- **Публикация**: топик выбирает реестр по типу; неизвестный тип, нарушение политики топика или
  несоответствие схеме — ошибка, событие не уходит.
- **Политики топиков**: `player_events` — `actor_kind ∈ human|ci|sim` и `meta.agent == nil`;
  топики роя (`llm_records`, `tick.*`, `agent.*`, `narrative.output`, `combat.decided`) —
  `meta.agent` обязателен. Проверяются при публикации всегда и при чтении, если валидация при
  чтении не выключена (см. ниже).
- **Валидация при чтении**: по умолчанию **включена** — нулевое значение `SkipValidateOnRead`
  означает «проверять» (SEC-16: защитная мера не должна выключаться тем, кто про неё не знает).
  Значение `MV_BUS_VALIDATE_ON_READ` (по умолчанию `true`) читает процесс и передаёт в конфигурацию
  инверсию: `SkipValidateOnRead = !MV_BUS_VALIDATE_ON_READ`.
- **Публикация без ожидания батча**: writer собирается с `BatchSize: 1` и `BatchTimeout: 10 мс`
  вместо умолчаний kafka-go (100 сообщений и 1 с). MVP-1 публикует по одному событию в топик из
  одной партиции — батч не заполняется, и каждая публикация ждала бы секунду, что рушит бюджет
  подтверждения NFR-001 (≤ 300 мс). Фактическую задержку проверяет contract-тест T-014 на брокере.
- **Доставка**: at-least-once; порядок внутри топика (одна партиция); ошибка обработчика — повтор
  ×3 (100/500/2000 мс), затем `dead_letters` и коммит офсета: одно плохое событие не блокирует топик.
  Тело нераспознанного сообщения в `dead_letters` усекается до `MaxDeadLetterRaw` (512 КиБ) с
  пометкой `raw_truncated`: запись больше 1 МиБ не прошла бы в топик, офсет не закоммитился бы и
  топик встал бы навсегда — ровно то, ради чего заводился DLQ.
- **Остановка**: штатное завершение по отмене контекста — это `nil` из `Subscribe` и `Tail`, а не
  `context.Canceled`; membus (T-014) даёт ту же семантику.
- **Паника не ловится**: по NFR-012 паника — дефект, процесс должен упасть со стеком.
- **Журнал**: `ReadRange` отдаёт события строго по возрастанию офсета; `End` — офсет следующего
  сообщения (high watermark) и монотонен; признак конца журнала — позиция ≥ `End − 1` по всем
  читаемым топикам. Отдельного `Lag()` нет.
- **Wire-формат** — JSON конверта; новые поля добавляются без слома потребителей. Событие без
  `meta` принимается только для типов с пометкой `deprecated` (профиль `legacy`).

## Устаревшее

`NewEvent`, `NewEventWithDescription`, `NewStructuredEvent` и `WithTimestamp` помечены
`Deprecated`: они остаются для профиля `legacy` (`gm_path=legacy`) и удаляются вместе с ним (S5).
В новом коде — только `NewRoot`/`Derive`.

`MIGRATION.md` и `docs/` описывают предыдущую итерацию модели событий; решение об их переносе в
`Docs/archive/` принимает tech-writer в T-019 (F-9).
