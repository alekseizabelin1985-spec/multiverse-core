# `shared/eventbus`

Конверт события платформы и интерфейсы шины и журнала, через которые общаются все контексты.

Контракт: `Docs/dev-team/architecture/contracts.md` **C-01 v1.4** · ADR-007, ADR-027 ·
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
| `cause_id.go` | `WithCauseID`, `CauseIDNamespace` — id производного события из причины (ADR-027) |
| `dedup.go` | `Dedup` — LRU по `event.id` у потребителя: `Seen` одним шагом, `Has`/`Add` в два |
| `kafka.go` | реализация `Bus` + `Journal` на `segmentio/kafka-go` (Redpanda) |
| `topics.go` | имена восьми топиков MVP-1 |
| `payload_types.go`, `nested_payload.go`, `relations.go` | билдер payload, доступ по dot-путям, связи для графа |

In-memory реализация (`membus`, подпакет `shared/eventbus/membus`) — вторая реализация C-01 и
транспорт режима `--bus=memory` (C-01 v1.4; до T-418 лежала в `shared/testkit`). Она использует те
же `Route` и `Delivery`, поэтому ведёт себя одинаково с kafka-адаптером — это проверяет
contract-тест F-5t (`shared/testkit/contract`).

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

### Id из причины (`WithCauseID`, C-01 v1.4, ADR-027)

```go
text := eventbus.Derive(decided, "narrative.output", "core/swarm", payload,
    eventbus.WithCauseID("turn"),           // parts: вид нарратива
    eventbus.WithAgent(narrator))
```

Id — UUIDv5 в пространстве имён `eventbus.CauseIDNamespace` от `causation_id`, типа события и
`parts`; каждая часть кодируется с длиной впереди, поэтому `("a|b")` и `("a", "b")` не совпадают.
Одна причина, тип и части дают один id при повторе публикации, после рестарта и в replay — при любом
`SetIDSource`: генератор с этой опцией не вызывается вовсе и последовательность не сдвигается.
Повтор гасит та дедупликация по id, которая уже есть у каждого потребителя.

| Где | `parts` |
|---|---|
| `narrative.output` — обязательна (T-229) | вид нарратива, если на одну причину их несколько |
| `encounter.started`, `encounter.ended` — обязательна (ADR-026) | id встречи |
| факты State `entity.created`/`entity.updated` — рекомендована (решает EPIC-002) | id сущности |
| `dice.rolled` | только с индексом броска: без него два броска получат один id |

Опция — только для события, которое издатель публикует не больше одного раза на
`(причина, тип, части)`. Неверные `parts` склеивают два разных события в одно, и второе потребитель
молча гасит как дубль.

Опция паникует, когда у события нет причины: в `NewRoot` (у корня нет причины — ошибка программиста,
видна в первом тесте) и в `Derive` от конверта без `id`. Второй случай достижим по данным. Конверт без
`id` не проходит проверку конверта, поэтому при валидации при чтении (по умолчанию) он уходит в
`dead_letters` и до обработчика не доходит. При `MV_BUS_VALIDATE_ON_READ=false` он доходит, и паника
опции становится паникой обработчика. Её перехватывает `Deliver` (C-01 v1.5, «Гарантии и правила»):
событие уходит в `dead_letters`, процесс живёт, но сессия всё равно провалена — это дефект. Поэтому
обработчик, который может работать без валидации при чтении, проверяет `ev.ID != ""` до
`Derive(…, WithCauseID(…))`: пишет `Error` и отказывается отвечать.

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

`Seen` запоминает **до** обработки — так нужно, когда повтор вреден (агент встречи задвоил бы кости).
Если побочный эффект — одна публикация, запоминать надо **после** её успеха, иначе упавшая
публикация пропадает бесследно. Для этого есть двухшаговая форма (C-01 v1.4):

```go
if answered.Has(ev.ID) { return nil }   // спросить: окно не меняется, даже порядок вытеснения
if err := bus.Publish(ctx, text); err != nil {
    return err                           // не запомнено: шина повторит обработчик
}
answered.Add(ev.ID)                      // запомнить: вытеснение и ёмкость — как у Seen
```

Два шага не атомарны: они рассчитывают, что обработчик не вызывается конкурентно для одного
события, а обе реализации шины отдают топик по одному событию. Окно, заполненное любым путём,
снимается в снапшот C-14 теми же `IDs`/`Restore`.

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
- **Граница `Close` — транспорт, а не обработчик** (C-01 v1.7, ADR-023 п. 4). `Close` вправе
  прервать только ожидание транспорта: выборку следующего сообщения и коммит. Контекст, который
  получает `Handler`, — это контекст вызывающего `Subscribe`/`Tail`, и `Close` его не отменяет:
  обработчик может быть посреди PUT в объектное хранилище, а C-02 публикует факт только после
  успешной записи. Поэтому в kafka-адаптере два контекста: контекст цикла (от вызывающего плюс
  отмена по `Close`) — только на `FetchMessage` и `CommitMessages`; по нему же решается, что
  подписка остановлена. Обработчику уходит контекст вызывающего. Останавливает обработчик тот, кто
  вправе прервать работу: процесс отменяет контекст подписки в `Stop` её контекста до `Close` шины,
  на весь путь остановки есть `runtime.StopTimeout`. Событие, чей обработчик закончил работу после
  **возврата** `Close`, не коммитится и будет выдано снова: горутин читателя уже нет, коммит никто не
  обслужит. Если обработчик закончил, пока `Close` ещё идёт, событие может и закоммититься — на отмене
  kafka-go сбрасывает уже поставленные в очередь коммиты (`reader.go:199-219`); оба исхода законны,
  C-01 п. 2 коммит не гарантирует (at-least-once). Якорь — кейс
  `CloseUnderAFailingHandlerIsAnOrderlyStop` contract-набора.
- **Паника обработчика перехватывается** (C-01 v1.5): `recover` стоит вокруг вызова обработчика в
  `Delivery.Deliver`. Через него читают `Subscribe`, `ReadRange` и `Tail` обеих реализаций.
  Паника — дефект (NFR-012), а не временный сбой, поэтому она не повторяется. Событие сразу уходит в
  `dead_letters` с ошибкой `eventbus.ErrHandlerPanic` (`eventbus: handler panic: <значение>`), а
  `attempts` равен номеру вызова, в котором была паника. Стек в `dead_letters` не пишется: он идёт в
  лог уровня `Error` с полями `panic`, `stack` и `handled=false`. Офсет коммитится после записи в
  `dead_letters`. Если запись не удалась, `Deliver` возвращает ошибку и офсет не коммитится.
  Письма при остановке могут повториться. Kafka-подписка коммитит с уже отменённым контекстом, и
  коммит может не пройти. Тогда после рестарта событие приходит снова, и в `dead_letters` появляется
  второе письмо с тем же `original.id`. Это обычный at-least-once, письма разбираются по
  `original.id`. В `membus` курсор сдвигается по `nil`, повтора нет.
  Без перехвата событие, уронившее процесс, пришло бы снова после рестарта и уронило бы его снова
  вместе со всеми контекстами (ADR-001).
  Перехват стоит только на границе доставки: паника вне обработчика по-прежнему роняет процесс.
  **Stateful-контекст**, который после паники в своём обработчике не может доказать целостность своего
  состояния, ставит собственный `recover` на своей границе и останавливает себя (`/health fail`) —
  правило C-01 v1.5. Шина не знает, чьё состояние испорчено, и решает только судьбу события.
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
