# `shared/entity`

Модель сущности мира: то, что State хранит в объектном хранилище, что несёт снапшот и что
проецируют все read-model. Типизированные операции (`ops`) и канонический хэш состояния.

Контракт: `Docs/dev-team/architecture/contracts.md` **C-02 v1.2** ·
`architecture/components/state-and-mechanics.md` §3 · `analysis/data-model.md` §3 ·
ADR-011, ADR-013. Схемы событий — `schemas/events/entity.*.v1.json`.
Изменение публичного API этого пакета — **только через системного архитектора**
(`contracts.md` §16); с волны 1 владелец пакета — EPIC-002.

## Что здесь есть

| Файл | Содержимое |
|---|---|
| `entity.go` | `Entity`, `HistoryEntry`, `LastChange`, `Ref`, `New`, `Clone`, `CheckVersion`/`ErrVersionConflict`, `Commit`, `SetFactEventID` |
| `ops.go` | `OpKind`, `Op`, `Change`, `ChangeSet`, `Propose`, `ApplyOps`, `ErrInvalidOp`, `ReservedPaths` |
| `path.go` | грамматика путей на запись (`splitPath`, `setIn`, `deleteIn`); чтение делегировано `shared/jsonpath` |
| `attrs.go` | типизированные геттеры всех атрибутов `data-model.md` §3 |
| `types.go` | константы типов, статусов, `actor_kind`, состояний группы и встречи, имена атрибутов |
| `hash.go` | `CanonicalJSON`, `StateHash` |

## Три правила пакета

**Нет часов.** Ни одна функция не вызывает `time.Now`: время сущности — это время события,
которое её изменило (`created_at`/`updated_at` = `timestamp` предложения, ADR-003 п. 6).
Поэтому два прогона одной записи дают побайтово одинаковые объекты.

**Нет записи «за спиной» вызывающего.** `ApplyOps` считает эффект предложения на **копии**
атрибутов и возвращает её; сущность двигает только `Commit`. State успевает проверить владение
и инварианты между этими двумя шагами (`state-and-mechanics.md` §4.5 п. 7–10).

**Нет политики.** Кто что может предлагать, какой инвариант держится и в какую причину отказа
превращается ошибка — решает `internal/state` (§4.5, §4.6). Здесь только смысл операции и
признак того, что она сформирована неверно.

## Структура

```go
type Entity struct {
    SchemaVersion int            // 1
    ID, Type      string         // world|region|player|npc|group|encounter
    WorldID, Name string
    Version       int64          // строго +1 на факт с непустым changed
    CreatedAt     time.Time      // timestamp create.proposed
    UpdatedAt     time.Time      // timestamp последнего *.proposed
    LastEventID   string         // id последнего entity.created/updated
    Attributes    map[string]any // доменные атрибуты, data-model.md §3
    LastChange    *LastChange    // commit-record последнего применения
    History       []HistoryEntry // последние 50; полная история — журнал
}
```

## Операции

```go
attrs, changed, err := entity.ApplyOps(e, []entity.Op{
    {Op: entity.OpInc, Path: entity.AttrHP, Value: -4},
    {Op: entity.OpAppend, Path: entity.AttrInventory, Value: pelt},
})
if err != nil { /* ErrInvalidOp → entity.update.rejected reason=invalid_op */ }

e.Commit(attrs, changed, entity.LastChange{
    ProposalID: proposalID, ProposalEventID: cause.ID,
    Cause: "combat", AppliedAt: cause.Timestamp, Atomic: true, BatchSize: 1,
})
// … PUT объекта, затем публикация факта:
e.SetFactEventID(fact.ID)
```

| Op | Семантика | `invalid_op` |
|---|---|---|
| `set` | записать `value` по пути, создавая промежуточные map | зарезервированный путь; значение не JSON-совместимо; индекса нет в списке; индекс по объекту (`members[0]`, где `members` — не список: список растит только `append`) |
| `inc` | целое прибавить к текущему (отсутствующее = 0); по пути `hp` — clamp в `[0, hp_max]` | текущее не число; `value` не целое |
| `append` | добавить в список (создавая его); объект с `item_id`/`player_id`/`npc_id` не дублируется — повтор это no-op | по пути не список |
| `remove` | с `value` — убрать равные элементы (объекты по id); без `value` — удалить ключ или элемент списка по индексу (`inventory[0]`) | по пути нет ключа/списка |

Пути — грамматика `shared/jsonpath` (`a.b[0].c`). Зарезервированы `ReservedPaths`
(`id`, `type`, `world_id`, `version`, `created_at`, `updated_at`, `last_event_id`, `history`,
`last_change`, `schema_version`) и любой путь с ведущим `_`.

`changed[]` строится **после** всех ops: для каждого затронутого пути сущность «как была»
сравнивается с копией «как стала» — отсюда и первый `old`, и последний `new`, и то, что
операция, отменённая следующей (`append` + `remove` того же элемента), не оставляет записи.
Изменением считается и другое значение (сравнение каноническое: `10` и `10.0` — одно и то же),
и другой **факт существования** пути: «ключа не было» и «по ключу `null`» читаются одинаково, но
это разные состояния, и `state_hash` их различает. Поэтому `set` отсутствующего ключа в `null`,
`append` элемента `null` и `remove` ключа со значением `null` дают непустой `changed[]`.

Пути в `changed[]` подобраны так, чтобы буквальное применение списка давало то же состояние:
для `append` это путь элемента (`inventory[0]`, C-02 v1.1), а для `remove` элемента списка — путь
**самого списка** со списками до и после, потому что удаление сдвигает хвост.

Пустой `changed` — не ошибка: версия не растёт, факт публикуется с `changed: []`
(ход засчитан, UC-011 E2). Инвариант «`changed[]` непуст ⇔ `state_hash` сдвинулся» закреплён
тестом `TestChangedListAndStateHashMoveTogether`.

## Хэш

```go
entity.CanonicalJSON(e)  // {"attributes":…,"id":…,"type":…,"version":…} — все ключи отсортированы
entity.StateHash(world)  // "sha256:<hex>" по сущностям в порядке (type, id)
```

Хэш **исключает** `updated_at`, `history`, `last_change`, `last_event_id`, `name` и `world_id` —
это то, как сущность сюда попала, а не где она стоит. Именно этот хэш сравнивает
`recovery_state_identical` и хранит `snapshot.state_hash` (§3.3, §4.4).

Каноническая форма — межреализационный контракт, поэтому её формат зафиксирован в тесте
литералом (`TestCanonicalJSONAndStateHashGolden`): изменение кодировщика обесценивает все уже
сохранённые `snapshot.state_hash` и проходит только через системного архитектора.

## Статусы

`alive`, `dead`, `abandoned`, `ascended_final`. Последние три — терминальные:
`IsTerminalStatus`, `Entity.IsTerminal`, `StatusTransitionAllowed(from, to)`.
`abandoned` — след `/forget` (FR-061): переходит **только** из `alive`, предлагает **только**
gateway (`cause=forget`), везде читается как `dead`, кроме того, что `narrative.output kind=death`
для него не пишется (C-02 v1.2).

## Что пакет намеренно не делает

- не проверяет обязательные атрибуты при создании (`state-and-mechanics.md` §4.5, создание — State);
- не даёт геттера `last_session_ended_at` (§3.3): атрибут — проекция `analytics.session.ended` и
  по модели может жить в game-service;
- не знает `OwnershipRules` и инвариантов (`shared/contracts`, `internal/mechanics`);
- не решает, какие пути допустимы над терминальной сущностью (§4.5 п. 5 — правило State);
- не пишет и не читает объекты (`internal/state.Store` над `shared/objstore`).
