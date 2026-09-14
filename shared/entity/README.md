# `shared/entity`

Модель сущности мира: то, что State хранит в объектном хранилище, что несёт снапшот и что
проецируют все read-model. Типизированные операции (`ops`) и канонический хэш состояния.

Контракт: `Docs/dev-team/architecture/contracts.md` **C-02 v1.8** (запись пути и виды атрибутов — п. 2, T-472) ·
`architecture/components/state-and-mechanics.md` §3 · `analysis/data-model.md` §3 ·
ADR-011, ADR-013. Схемы событий — `schemas/events/entity.*.v1.json`.
Изменение публичного API этого пакета — **только через системного архитектора**
(`contracts.md` §16); с волны 1 владелец пакета — EPIC-002.

## Что здесь есть

| Файл | Содержимое |
|---|---|
| `entity.go` | `Entity`, `HistoryEntry`, `LastChange`, `Ref`, `New`, `Clone`, `CheckVersion`/`ErrVersionConflict`, `Commit`, `SetFactEventID` |
| `ops.go` | `OpKind`, `Op`, `Change`, `ChangeSet`, `Propose`, `ApplyOps`, `CheckOp`, `ErrInvalidOp`, `ReservedPaths` |
| `path.go` | каноническая запись пути `CanonicalPath` (C-02 v1.8 п. 2); запись по пути (`splitPath`, `setIn`, `deleteIn`); чтение делегировано `shared/jsonpath` |
| `attrs.go` | типизированные геттеры всех атрибутов `data-model.md` §3 (`Flee` читает и текст `"+2"` фикстур, и число `flee: 2` правил; дробь, `bool` и число вне `int64` возвращает текстом, на котором формула падает, как на `"abc"`); таблица видов атрибутов `data-model.md` §3 по типам (`ValueKind`, `AttributeSpec`, `AttributeSpecOf`) и проверка атрибутов создания `CheckAttributes`/`ErrInvalidAttribute` |
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
| `set` | записать `value` по пути, создавая промежуточные map | зарезервированный путь; значение не JSON-совместимо (в том числе число по модулю не меньше 2^53 внутри; допустимо до 2^53−1); индекса нет в списке; индекс по объекту (`members[0]`, где `members` — не список: список растит только `append`) |
| `inc` | целое прибавить к текущему (отсутствующее = 0); по пути `hp` — clamp в `[0, hp_max]` | текущее не число; `value` не целое; приращение или результат по модулю не меньше 2^53; текущее за пределом — своя причина `ReasonCurrentRange` |
| `append` | добавить в список (создавая его); объект с `item_id`/`player_id`/`npc_id` не дублируется по id, объект без id — если в списке есть канонически равный ему целиком (`{at: 2}` = `{at: 2.0}`); повтор это no-op; скаляры не дедуплицируются | по пути не список |
| `remove` | с `value` — убрать равные элементы (объекты по id); без `value` — удалить ключ или элемент списка по индексу (`inventory[0]`) | по пути нет ключа/списка |

Элементы списка сравниваются в **проводной форме**, а не как значения Go: `entity.Item`, собранный
в Go, — тот же предмет, что и `map` с тем же `item_id`, прочитанный из шины или снапшота, а `3` —
то же число, что `3.0` из JSON. Поэтому повторный `append` трофея-структуры — no-op (inv-03), а
`remove` числа из списка, пришедшего по проводу, действительно убирает его (T-050).

**Запись пути (C-02 v1.8 п. 2, T-472).** Путь операции — только в канонической записи
(`entity.CanonicalPath`): ключи через одну точку; индекс элемента — только `[n]` после ключа, `n` — `0` или
до девяти цифр без ведущего нуля (`a[0][1]`); ключ не пуст, без `.`, `[`, `]` и не записан как десятичное
целое любой длины — `^[+-]?[0-9]+$` (`inventory.0`, `a.+1`, `a.-1`, `a.99999999999999999999` недопустимы;
`a.1e3` допустим). Проверка — шаблоном, не `strconv.Atoi`: граница переполнения `Atoi` зависит от размера
`int` платформы, и форма пакета разошлась бы между машинами (C-02 v1.8b, NFR-061). Иначе — `invalid_op` с `ReasonBadPath` или
`ReasonEmptyPath`. `shared/jsonpath` терпимее (`status.`, `inventory.0` для него — второе написание), поэтому
проверка стоит до чтения и записи. Зарезервированы `ReservedPaths` (`id`, `type`, `world_id`, `version`,
`created_at`, `updated_at`, `last_event_id`, `history`, `last_change`, `schema_version`) и любой путь с
ведущим `_`.

**Виды атрибутов.** Таблица `data-model.md` §3 по типам сущности — в `attrs.go`: вид значения
(`KindText`, `KindInteger`, `KindNumber`, `KindTime` — RFC 3339, `KindDuration` — `"24h"`, `KindRef`,
`KindEnum`, `KindModifier` — «int / формула» боевых статов, `KindOpen` — контейнер или `scope`), обязательность
и допустимость `null` (у необязательных, а из обязательных — у `leader_id`; `flee` необязателен: «`null` или отсутствие — не убегает», §3.3). Тип берётся из
`Entity.Type`; атрибут вне таблицы своего типа и тип вне таблицы не типизированы — модель открыта.
`ApplyOps` отвергает путь ниже скаляра (`status.x`, `hp[0]` — `ReasonBelowScalar`) и после всех операций —
значение чужого вида в корне затронутого скаляра (`set hp "10"`, `set hp {x: 999}`, `set state {…}` —
`ReasonWrongKind`). Удалённый скаляр — не чужой вид: его отсутствие решает норма State. Значение
сравнивается в проводной форме: `10` из Go и `10.0` с шины — одно целое. Перечисление проверяется как
текст: принадлежность словарю (`weather` — словарь блупринта) — вопрос мира, не формы.

`CheckOp(type, op)` — та же проверка формы без сущности: глагол, запись пути, зарезервированный корень,
путь ниже скаляра типа. Её зовёт шаг 1 State с типом, который называет набор изменений.
`CheckAttributes(type, attrs)` проверяет атрибуты создания: все обязательные атрибуты типа есть, каждый
типизированный — своего вида (`ErrInvalidAttribute`; у State — `invalid_op`). Имя сущности живёт в
`Entity.Name` и среди атрибутов не требуется.

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

Ключи элемента стоят **по наличию пути** (C-02 v1.6, T-448): `old` есть ⇔ путь был до изменения,
`new` есть ⇔ путь есть после. У `append` нет `old`, у `remove` ключа нет `new`, у `set` в `null`
есть `new: null`. В Go наличие решают **только** `Change.HasOld`/`HasNew` — не значение, потому что
`null` тоже значение; `MarshalJSON`/`UnmarshalJSON` пишут и читают ключи по ним. `Change`, собранный
вручную, обязан поставить флаги: `MarshalJSON` возвращает ошибку с путём, если нет ни одного флага или
значение не `nil` при опущенном флаге, — иначе дефект всплыл бы только в схеме `entity.updated` у
`Publish`. `UnmarshalJSON` на `null` ничего не делает, как `encoding/json` для остальных типов.

Если путь в конце отсутствует, а его предка предложение создало (`inc fresh.deep` + `remove fresh.deep`
оставляет `fresh = {}`), в `changed[]` попадает и предок — **после** элементов путей: догон применяет
элементы по порядку, и порядок — часть формы.

Правило догона (`state-and-mechanics.md` §4.8): элементы по порядку; `new` есть — записать по пути,
заменяя объектом промежуточный узел, которого нет или который не контейнер (как `set`); для `a[n]` при
`n == len(a)` — дописать элемент простым добавлением, без дедупликации (`null` по пути списка — это
отсутствующий список); `a[n]` с `n > len(a)` в факте State не бывает, это повреждённый факт
(`state_divergence`); путь не в канонической записи — тоже повреждённый факт (C-02 v1.8 п. 2:
State публикует только канонические пути; проверка — `entity.CanonicalPath`); `new` нет — удалить путь,
если он есть. Свойство «догон по `changed[]` после JSON
туда-обратно даёт тот же `StateHash`, что `ApplyOps`» закреплено `TestChangedFormPerOperation` и
`TestCatchingUpOnChangedReproducesTheState`; решения правила — именованными векторами
`TestChangedReportsTheAncestorAProposalCreated`, `TestCatchingUpAppendsWithoutDeduplication`,
`TestCatchingUpSkipsAPathAnEarlierEntryAlreadyRemoved`, `TestCatchingUpRefusesAnElementPastTheEndOfItsList`,
`TestCatchingUpRefusesAPathNotInItsCanonicalForm`; вектор зонда У-1 T-056 (`remove inventory.0` на списке из
трёх) — `TestTheRemovalOfAnElementIsSpelledWithBrackets`.

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

Значение, собранное в Go, пишется так, как оно вернётся из JSON, — иначе сущность в памяти и она же,
прочитанная из снапшота, дают разный хэш (свойство `StateHash(e) == StateHash(roundtrip(e))`,
`TestStateHashOfAGoBuiltWorldMatchesItsWireForm`, T-050):

- структура — объект с отсортированными ключами;
- типизированный nil-список или nil-map — `null`; исключение — `[]any`, `map[string]any` и
  `[]map[string]any`: `jsonpath.Clone` пересобирает их пустыми, в сущности nil этих типов не живёт, и
  они пишутся `[]`/`{}`;
- `float32` — десятичная запись кодировщика (`0.1`, а не `0.10000000149011612`);
- `[]byte` — строка base64; `json.RawMessage`, `time.Time` и типы с `MarshalJSON`/`MarshalText` — то,
  что они сами пишут; map с целыми ключами — объект со строковыми ключами.

Значения с провода ни в одну из этих веток не попадают — формат для них не изменился, золотой
литерал тот же. Числа по модулю от 2^53 JSON теряет, поэтому `ApplyOps` их не принимает (C-02 v1.6,
T-448): допустимо `|x| ≤ 2^53−1`. `set`/`append` со значением, внутри которого такое число, и `inc`, у
которого приращение или результат за пределом, — `invalid_op` с `ReasonNotJSON`; текущее значение за
пределом — `ReasonCurrentRange` (виноват атрибут, а не операция). Сама 2^53 отвергается потому, что
предложение приходит в State уже декодированным: `2^53` и `2^53+1` на проводе дают один `float64`, и
включительная граница пропустила бы `2^53+1`, пришедшее с шины. Проверка экспортирована —
`entity.JSONCompatible(v)`: тем же правилом State проверяет `attributes` у `entity.create.proposed`.
Большие идентификаторы и зёрна едут строкой (как `dice.rolled.roll.seed`).

Каноническая форма — межреализационный контракт, поэтому её формат зафиксирован в тесте
литералом (`TestCanonicalJSONAndStateHashGolden`): изменение кодировщика обесценивает все уже
сохранённые `snapshot.state_hash` и проходит только через системного архитектора.

## Статусы

`alive`, `dead`, `abandoned`, `ascended_final`. Последние три — терминальные:
`IsTerminalStatus`, `Entity.IsTerminal`, `StatusTransitionAllowed(from, to)`.
`abandoned` — след `/forget` (FR-061): переходит **только** из `alive`, предлагает **только**
gateway (`cause=forget`), везде читается как `dead`, кроме того, что `narrative.output kind=death`
для него не пишется (C-02 v1.2).

`StatusTransitionAllowed(x, x)` — `true` для `alive` и `false` для терминальных (C-02 v1.4): `set
status` в то же значение у живой сущности — ход без изменений (`changed: []`, версия не растёт), а
не отказ; труп не действует, даже чтобы остаться трупом. Матрица применяется к **изменениям**, то
есть к тому, что осталось в `changed[]` после `ApplyOps`, — `set` без изменения туда не попадает
вовсе. Значение вне списка статусов — не статус: `false` и как цель, и как «то же значение».

## Что пакет намеренно не делает

- не решает, применять ли создание: `CheckAttributes` говорит, что атрибуты не годятся, а отказ публикует State (§4.5);
- не проверяет, что ссылка указывает на существующую сущность и что перечисление взято из своего словаря;
- не даёт геттера `last_session_ended_at` (§3.3): атрибут — проекция `analytics.session.ended` и
  по модели может жить в game-service;
- не знает `OwnershipRules` и инвариантов (`shared/contracts`, `internal/mechanics`);
- не решает, какие пути допустимы над терминальной сущностью (§4.5 п. 5 — правило State);
- не пишет и не читает объекты (`internal/state.Store` над `shared/objstore`).
