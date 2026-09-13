# Журнал разработки EPIC-002 «Состояние и механика»

<!-- dev-log T-051 -->
## developer#1 · T-051 · RNG и формулы: добор статистических тестов · 2026-09-13

Ветка `task/T-051-rng-statistics` (от `1c2ee7e`), TEAM-1, Opus. Подробности — карточка `tasks/T-051.md`, раздел «Выполнение».
- **Что сделано**: новый `internal/mechanics/rng_stats_test.go`, код пакета не менялся.
  - `TestRollReproducibleThousandCausesTwoOrders` — 1000 причин × 4 броска хода (`d20`, `2d6+1`, `d20`, `d4`), два прохода в разном порядке на разных наборах правил, 4000 из 4000 `Roll` совпадают.
  - `TestRollD20ChiSquare` — χ² d20 по каждому индексу (n = 2500) и общий (n = 10000), порог 43,82 (df = 19, p = 0,001). Факт: 22,45 / 15,14 / 29,28 / 20,10, общий 13,69.
  - `TestRollIndicesUncorrelated` — Пирсон на 1000 причинах, |r| < 0,1 для пары 0–1 (+0,0349) и остальных пяти пар.
  - `TestStatisticsHelpers` — формулы χ² и r на ручных примерах.
- **Решения по ходу.** Броски идут через `Rules.Roll`, а не через `NewRNG` напрямую: проверяется тот же путь, по которому бросает бой и фон. Порог χ² применён к каждому индексу отдельно — смещение на одном индексе в общей сумме размывается.
- **Отклонения**: векторы `Seed` — inline (T-015), не в `testdata`; разрешено оркестратором.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути): M0 контрольный — красный (сборка); D1 (крайние грани вдвое чаще, среднее 10,5), D2 (сдвиг вверх), D3 (грань 1 → 2) — χ² красный, D1 и D2 прежний `TestRollDistribution` не ловит; C1 (индекс 1 на seed индекса 0) — r = 1, C2 (повтор у каждой пятой причины) — r = 0,23, красный, C2 прежний `TestRollIndicesDiffer` не ловит; S1 (состояние в `NewRNG`) — воспроизводимость красная (2172 из 4000).
- **Прогоны**: `go build ./... && go vet ./...` — 0; `gofmt -l internal/mechanics` — пусто; `go test -short -count=1 ./...` — 27 пакетов ok; `golangci-lint run ./internal/mechanics/...` — 0 issues; `make test` — exit 0, покрытие `internal/mechanics` 94,9 %. `-race` недоступен (нет cgo).
- Docker, интеграционные тесты и стенд `:8888` не трогались, `.env` не открывался. Не коммитил.

<!-- dev-log T-052 -->
## developer#2 · T-052 · Схемы EPIC-002: ревизия, шаблон хеша, фикстуры valid/invalid · 2026-09-13

Ветка `task/T-052-schemas-fixtures` (от `8768980`), TEAM-1, Opus. Подробности — карточка `tasks/T-052.md`, раздел «Выполнение».
- **Ревизия восьми схем** (`OwnerState` в `registry.go`; `entity.delete.proposed` в реестре нет): enum `op`, опциональный `expected_version`, семь причин отказа C-02 v1.4 без `invariant`, `changed[]` для `append`, `cause` с `forget|resolve|init|author`, `component` C-14 v1.2, `mode ∈ recovery|test`, `events_hash_match` — на месте, правок не потребовали. Примеры `api-contracts.md` §2.3.4, §2.3.5, §2.3.12 (и §2.3.14) сверены — совпадают, пример §2.3.4 перенесён в валидную фикстуру.
- **Шаблон хеша** (решение system-architect#1, ревизия 4 п. 3): `snapshot.state_hash`, `replay.state_hash_before`, `replay.state_hash_after` получили `pattern ^sha256:[0-9a-f]{64}$` — форма `entity.StateHash`. Следствие — четыре тестовых литерала хеша в `shared/contracts/validate_test.go` (EPIC-001) заменены на форму `sha256:` (по указанию оркестратора; код и реестр не менялись).
- **Фикстуры**: 16 файлов `testdata/fixtures/events/<тип>.v1.{valid,invalid}.json`; у каждой невалидной одно нарушение (сверено выводом валидатора), описано в README. `events_test.go` не менялся.
- **Тест словарей и форм**: `test/fixtures/state_schemas_test.go` — семь причин, `mode`/`component`, поля совладения с EPIC-005, шаблон у трёх хешей и его совпадение с `entity.StateHash`.
- **Записано явно** (README и карточка): C-02 v1.3 (одна сущность дважды) и v1.4 (путь ↔ причина, статус в то же значение) проверяет State, а не схема.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути): M0 контрольный — красный; M1–M14 (снятие `required`, ослабление `const`, расширение/сужение enum, снятие `type` у `seed`, снятие и ослабление шаблона хеша, удаление `events_hash_match`) — все красные.
- **Прогоны**: `mvctl contracts check` — 65 типов, 58 файлов; `go test -short -count=1 ./shared/contracts/... ./test/fixtures/...` — ok; `golangci-lint run ./test/... ./shared/contracts/...` — 0 issues; `go build ./...`, `go vet`, `gofmt -l` — чисто; `mvctl privacy scan testdata/` — чисто; `make test` — exit 0 (27 пакетов ok, `internal/mechanics` 94,9 %, без `-race`).
- **Вопросы и риски**: `proposal_id` у `entity.create.proposed` оставлен опциональным (ОВ-21) — подтвердить; T-445 правит тот же `testdata/fixtures/events/README.md` — возможен текстовый конфликт при слиянии. Docker, интеграционные тесты и стенд `:8888` не трогались, `.env` не открывался. Не коммитил.

<!-- dev-log T-052 итерация 2 -->
## developer#2 · T-052 · итерация 2 по ревью #1 (2 Minor, 3 Nit) · 2026-09-13

Подробности и ответ по каждому пункту — карточка `tasks/T-052.md`, «Итерация 2». Не коммитил.
- **Mi-1**: пара `analytics.replay.completed` приведена к согласованному сочетанию §4.4 — штатная остановка: `events_replayed: 0`, `state_hash_before == state_hash_after` (хеш снапшота seq 0), `identical: true`. Невалидная отличается от валидной одной строкой `mode: "replay"`.
- **Mi-2**: README и комментарий `TestReplayKeepsTheFieldsOfModeTest` больше не приписывают EPIC-005 `identical` и `state_hash_before`. За EPIC-005 — `events_hash_match` и значение `mode=test` (`ownership.md` §1, C-14). Строка карточки «правок полей EPIC-005 нет» заменена.
- **Уведомление tech-lead#1 и EPIC-005 (совладение `schemas/events/analytics.replay.completed.v1.json`)**: в T-052 полю `replay.state_hash_before` добавлен `pattern ^sha256:[0-9a-f]{64}$` и описание (прежде — любая строка, включая пустую); то же — `state_hash_after`. Харнесс `mode=test` (EPIC-005) обязан писать хеши в форме `entity.StateHash`, пустая строка вместо отсутствующего поля теперь невалидна — поле без снапшота опускается. `events_hash_match` и `mode` не менялись.
- **Nit 3**: ссылки сведены к одной форме — нормы C-02 v1.3/v1.4 проверяет State в T-056; `StatusTransitionAllowed(x, x)` правит T-050.
- **Nit 5**: сценарий невалидной `entity.updated` — `set status alive` у живого персонажа в бою (`cause: combat`) вместо «повторного `/forget`»; в README добавлено, что у терминальной сущности тот же `set` — `dead_entity`.
- **Nit 4** не трогался (решение оркестратора) — в бэклог: `schemaEnum` через `schemaNode`.
- **Проверки**: ровно одна ошибка валидатора у каждой из 8 невалидных фикстур (диагностический тест в копии в scratch, копия удалена по точному пути); `mvctl contracts check` — 65 типов, 58 файлов; `go test -short -count=1 ./shared/contracts/... ./test/fixtures/...` — ok; `golangci-lint run ./test/... ./shared/contracts/...` — 0 issues; `mvctl privacy scan testdata/` — чисто; `gofmt -l test`, `git diff --check` — чисто.
- Вопросы `proposal_id` и форма `changed[]` переданы оркестратором system-architect и tech-lead#1, в этой итерации не трогались. Docker, стенд `:8888`, `.env` не трогались.
# Журнал разработки EPIC-002 «Состояние и механика»

Формат записи: экземпляр разработчика, задача, что сделано (команды), отклонения от дизайна,
результаты проверок DoD, открытые вопросы. Язык — русский; код и конфиги — английские.

---

<!-- dev-log T-050 -->
## developer#2 · T-050 · `shared/entity` — сверка с моделью данных и добор тестов · 2026-09-13

Ветка `task/T-050-entity-reconcile` (база `1c2ee7e`), TEAM-1, Opus. Подробности — карточка `tasks/T-050.md`, раздел «Выполнение».
- **Что сделано.**
  - C-02 v1.4 (`contract-change`): `StatusTransitionAllowed(alive, alive)` = `true`; для терминальных и неизвестных значений «то же значение» — `false`. Тест `alive→alive` сменил ожидание, добавлены терминальные и неизвестные строки и тест связки с `ApplyOps` (пустой `changed`, версия не растёт).
  - `FakeState` не правился: матрица там применяется к `changed[]`, `go test ./shared/testkit/state/...` зелёный и на новом коде, и на мутанте со старым правилом.
  - Новый геттер `LastBackgroundEventAt` (§3.2); тесты на `GroupID`, `EncounterID`, `Scope`/`Position`/`RegionID` группы и встречи.
- **Расхождения кода с моделью — исправлены.**
  - `Flee()` читал только строку, а правила пишут `flee: 2` числом: персонаж из правил «не бежал». Число теперь возвращается текстом.
  - `CanonicalJSON` писал структуру Go выводом `json.Marshal` (поля в порядке объявления), а та же сущность из снапшота — `map` с сортированными ключами: одна сущность, два `StateHash`. Такое значение теперь пишется в проводной форме. Золотой литерал не менялся.
  - Дедуп `append` по `item_id` работал только для `map`: повтор `entity.Item`-структуры (так добавляет трофей `FakeEncounter`) давал второй трофей. Элементы сравниваются в проводной форме; сравнение без id — каноническое, поэтому и `remove 3` из `[3.0]` теперь срабатывает.
- **Решения по ходу.** Id в `sameIdentity` оставлены на `reflect.DeepEqual`: мутант на `sameCanonical` выжил как эквивалентный (id — строки), лишнюю правку откатил. `Dmg()` числом не расширял — в правилах урон всегда кубы; вынесено в бэклог.
- **Тесты**: `TestSameStatusIsATurnWithNothingChanged`, `TestFleeReadsEveryShapeTheWritersUse`, `TestMembershipAttributes`, `TestLastBackgroundEventAt`, `TestApplyAppendOfAnItemBuiltInGoIsIdempotent`, `TestApplyRemoveComparesTheWireForm`, `TestApplyIncAcceptsEveryWholeNumber`, `TestApplyRemoveOfAKeyInsideAListElement`, `TestStateHashOfAGoBuiltWorldMatchesItsWireForm`; расширен `TestStatusTransitionAllowed`. Покрытие `shared/entity` 86,9 % → 93,0 %.
- **Мутанты** (копия в scratch, без `-overlay`, контрольный первым, удалена по точному пути): M0 контрольный — красный (сборка); M1 прежнее правило, M2 «то же значение всегда да», M3 `Flee` только строка, M4 хэш структуры как есть, M5 `asObject` только `map`, M6 значение по `reflect.DeepEqual`, M7 `hasSameIdentity` только `map`, M8 геттер читает другой атрибут, M9 без проверки терминального — все красные.
- **Прогоны**: `go build ./... && go vet ./...` — 0; `gofmt -l` — пусто; `go test -short -count=1 ./shared/entity/... ./shared/testkit/...` — 8 пакетов ok; `golangci-lint run ./shared/...` — 0 issues; `make test` — exit 0. `-race` недоступен (нет cgo). Docker не запускался.
- **Для T-055/T-056** (в карточке подробно): правило v1.3 и «путь ↔ причина» v1.4 — State; матрицу статуса звать на `changed[]`; `dead_entity` — по `IsTerminal()` (включая `abandoned`, которого нет в списке §4.5 п. 5).
- Не коммитил; `tasks.md` не трогал.

<!-- dev-log T-050 iteration 2 -->
## developer#2 · T-050 · итерация 2 по ревью #1 · 2026-09-13

Ветка `task/T-050-entity-reconcile` (база `1c2ee7e`), TEAM-1, Opus. Ревью #1 — «принять» (Minor 3, Nit 3), оркестратор решил закрыть до приёмки. Подробности — карточка `tasks/T-050.md`, раздел «Итерация 2».
- **Mi-1 (вариант А).** `CanonicalJSON` значения, собранного в Go, равен его форме после JSON туда-обратно. Типизированные nil-список и nil-map пишутся `null`; `[]any`, `map[string]any` и `[]map[string]any` остаются `[]`/`{}`, потому что `jsonpath.Clone` пересобирает их пустыми. `float32` пишется как у кодировщика, `[]byte` — base64, типы с `MarshalJSON`/`MarshalText` — своим методом, map с целыми ключами — объектом. Золотой литерал не менялся. Тест-свойство на 32 формы Go и воспроизведение ревьюера (`set` nil-значения → снапшот → повтор) добавлены.
- **Mi-2.** `Flee()` для дроби, `bool`, числа вне `int64` и объекта возвращает текст значения и `true`; механика ведёт себя на нём как на `"abc"` (зонд: одинаковая ошибка `unknown identifier "flee"`). `asInt64` больше не заворачивает `uint64 > MaxInt64` в отрицательное — `inc` на таком значении отвечает `invalid_op`.
- **Mi-3.** Метка `contract-change` в карточке перечисляет все пять изменений поведения; нужно ревью system-architect. Абзац C-02 v1.4 «Код расходится… T-056» поправлен T-444 в EPIC-001.
- **Nit.** Комментарий и README про дедуп объекта без id приведены к коду. Таблица сверки — ниже. Перекодирование значения на каждый элемент списка — в бэклог с цифрами ревьюера, не исправлял.
- **Мутанты** (новая копия в scratch, без `-overlay`, контрольный первым, удалена по точному пути): M0 — красный; A1–A8, F1–F3, S1, D1, D2 — красные. F4 (снята проверка диапазона `float` → `int64`) выжил как эквивалентный на amd64: там преобразование вне диапазона даёт `MinInt64`, и его отвергает следующая проверка; на arm64 оно насыщается, поэтому проверку оставил. A6 в первом прогоне выжил — добавлены формы, у которых вид и метод кодирования расходятся.
- **По ходу.** Python-правки на Windows записали CRLF в куски `ops.go`, `attrs_test.go`, `hash_test.go`; файлы нормализованы в LF.
- **Прогоны**: `go build ./... && go vet ./...` — 0; `gofmt -l` — пусто; `go test -short -count=1 ./...` — exit 0; `golangci-lint run ./shared/...` — 0 issues; `make test` — exit 0, 27 пакетов ok, `shared/entity` 93,7 %. `-race` недоступен (нет cgo). Docker не запускался.
- Не коммитил; `tasks.md` не трогал.

### Сверка с моделью данных: пункт → место в дереве → тест (N-3)

| # | Пункт (описание T-050 в `tasks.md`, `data-model.md` §3, C-02) | Место в дереве | Тест |
|---|---|---|---|
| 1 | `StatusTransitionAllowed(x, x)` по C-02 v1.4 | `shared/entity/types.go` — `StatusTransitionAllowed` | `entity_test.go` — `TestStatusTransitionAllowed`, `TestSameStatusIsATurnWithNothingChanged` |
| 2 | Двойник State не ломается | `shared/testkit/state/apply.go` — `statusRefusal` (не менялся) | `shared/testkit/state` — весь пакет; мутант S1 |
| 3 | Геттеры §3.1 World | `attrs.go` — `LawsVersion`, `Weather`, `TimeOfDay`, `Day`, `Season`, `Epoch`, `Locale`, `BlueprintRef` | `attrs_test.go` — `TestWorldAndRegionAttributes`, `TestAttributesThatOnlyHadANameBefore` |
| 4 | Геттеры §3.2 Region, в том числе `last_background_event_at` | `attrs.go` — `Description`, `Canon`, `NPCIDs`, `RespawnTTL`, `PerceptionRadius`, `PlayersPresent`, `EncounterChance`, `LastBackgroundEventAt` (новый) | `TestWorldAndRegionAttributes`, `TestLastBackgroundEventAt` |
| 5 | Геттеры §3.3 Character (`flee`, `encounter_id`, `group_id`, `inventory`) | `attrs.go` — `HP`, `HPMax`, `Atk`, `Def`, `Dmg`, `Flee`, `Status`, `Position`, `Scope`, `GroupID`, `EncounterID`, `ActorKind`, `Inventory`; `last_session_ended_at` без геттера намеренно | `TestCharacterAttributes`, `TestFleeReadsEveryShapeTheWritersUse`, `TestMembershipAttributes`, `TestAttrIntReadsTheModifierForm`, `TestScopeReadsBothShapes` |
| 6 | Геттеры §3.4 NPC (`died_at`, `killed_by`) | `attrs.go` — `Kind`, `RegionID`, `DiedAt`, `KilledBy`, `Loot`, `SpawnedBy` | `TestNPCAttributes`, `TestAttributesThatOnlyHadANameBefore` |
| 7 | §3.5 Item, `inventory[].source` | `attrs.go` — `Item`, `ItemSource` | `TestInventoryDecodes` |
| 8 | Геттеры §3.6 Group (`participation`) | `attrs.go` — `Members`, `LeaderID`, `State`; `Position`, `Scope`, `EncounterID` группы | `TestGroupAttributes`, `TestMembershipAttributes` |
| 9 | Геттеры §3.7 Encounter (`participants`, `npcs`) | `attrs.go` — `Participants`, `NPCs`, `Resolution`, `RoundSeq`, `TaskAgentID`, `OpenedByEventID`, `ClosedByEventID`; `RegionID`, `Scope` встречи | `TestEncounterAttributes`, `TestMembershipAttributes`, `TestAttributesThatOnlyHadANameBefore` |
| 10 | Обрезка `history` до 50 | `entity.go` — `HistoryLimit`, `appendHistory` | `entity_test.go` — `TestCommitKeepsTheLastFiftyHistoryEntries` |
| 11 | `Ref` ⇄ `eventbus.EntityRef` (файла `ref.go` нет) | `entity.go` — `Ref.EventRef`, `RefFrom`, `Entity.EventEntity` | `TestRefCrossesToTheBusAndBack` |
| 12 | Каждая op и каждая ошибка `invalid_op`, no-op | `ops.go` — `ApplyOps`, `applySet`/`applyInc`/`applyAppend`/`applyRemove`; `path.go` | `ops_test.go` — `TestApplySet*`, `TestApplyInc*`, `TestApplyAppend*`, `TestApplyRemove*`, `TestApplyOpsMalformed`, `TestApplyOpsDropsNoOps`, `TestApplyOpsRejectsReservedPaths`, `TestApplyIncAcceptsEveryWholeNumber`, `TestApplyRemoveOfAKeyInsideAListElement` |
| 13 | Дедуп `append`/`remove` по id в проводной форме | `ops.go` — `sameIdentity`, `asObject`, `hasSameIdentity` | `TestApplyAppendOfAnItemBuiltInGoIsIdempotent`, `TestApplyRemoveComparesTheWireForm`, `TestApplyAppendIsIdempotentPerItemID` |
| 14 | `StateHash` стабилен при перестановке ключей и между процессами | `hash.go` — `CanonicalJSON`, `StateHash` | `hash_test.go` — `TestCanonicalJSONDoesNotDependOnMapOrder`, `TestStateHashDoesNotDependOnTheOrderOfTheWorld`, `TestCanonicalJSONAndStateHashGolden` |
| 15 | `StateHash(e) == StateHash(roundtrip(e))` для значений из Go | `hash.go` — `writeCanonicalOther`, `writeCanonicalNilSlice`, `encodesItself`, `writeCanonicalEncoded` | `TestStateHashOfAGoBuiltWorldMatchesItsWireForm`, `TestASetOfANilListHashesTheSameAfterASnapshot` |
| 16 | Покрытие `shared/entity` ≥ 60 % | весь пакет | `make test`: 93,7 % |

<!-- dev-log T-053 -->
## developer#1 · T-053 · `mechanics`: `Resolve`, `NPCTarget`, `ChangesFor`, `dice.rolled` · 2026-09-13

Ветка `task/T-053-resolve-npctarget-changes` (от `8768980`), TEAM-1, Opus. Подробности — карточка `tasks/T-053.md`, раздел «Выполнение».
- **Что сделано**:
  - `Resolve` — по таблице §5.4: удар, укус, свободная атака, бегство, отдых.
  - `NPCTarget(npc, candidates) (*Actor, error)` — порядок из `npc_target.order`, ничья по id. Без кандидатов — `(nil, nil)`, ошибка — только дефект входа.
  - `ChangesFor` — `hp` с clamp, `status`; у NPC ещё `killed_by`, `loot_claimed_by` и трофей победителю.
  - `ErrNotImplemented` и `TestStubs` удалены. Потребители `NPCTarget` в `shared/testkit/mechanics` переведены на новую сигнатуру.
  - Добавлено поле `Actor.Kind` (развилка ADR-024/§17, метка `contract-change`).
- **Решения**:
  - `dmg` — только кубы (бэклог T-050): число отвергается с явным текстом.
  - `rest` не порождает `combat.decided`. `Resolve(rest)` без бросков даёт `HPAfter = HPMax`; отдых кубами отвергается — нет назначения `dice.rolled`.
  - Бросок свободной атаки делает вызывающий, с индекса после бегства (0 → 1, 2), как `FakeEncounter`.
  - HP в `ChangesFor` считается от цели (повтор после конфликта версий), смена «кто пал» — ошибка.
  - Стороны удара закреплены: персонаж бьёт NPC, NPC кусает персонажа.
- **Сверка с `FakeEncounter`** (внешний тест на membus с настоящими правилами; двойник не менялся). Совпадают ops HP/статуса/трофея и `cause`. Расхождения (Р1–Р4 держатся тестом; Р5 установлен чтением кода, тестом не держался — поправка итерации 2, Minor-1 ревью #1):
  - Р1 `expected_version` — у актёра нет версии;
  - Р2 `died_at` NPC и `acquired_at` трофея — у функции нет времени;
  - Р3 двойник пишет `died_at`/`killed_by` павшему **персонажу** — вне модели и таблицы владения, **запрос EPIC-003**;
  - Р4 позиция после побега;
  - Р5 `encounter.npcs[].last_damager` не пишет никто.

  Р1, Р2, Р4, Р5 — открытые вопросы к system-architect. Рекомендация по Р1/Р2: совместимо добавить `Action.At` и `Actor.Version`.
- **Отклонение от дизайна**: `ChangesFor` не выставляет `died_at` (DoD п. 5) и `last_damager` встречи (design I1-4) — по сигнатуре C-03 v1.2 это невыразимо. Контракт сверх `Actor.Kind` не расширял.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути, перед удалением сверена `diff -r`): контрольный M00 красный. M01–M35 красные. M16 (ничья по id наоборот) сначала выжил — добавлен тест порядка `[min_hp]`, стал красным. M32 переписан после ошибки сборки — красный.
- **Прогоны**: `go build ./... && go vet ./...` — 0; `gofmt -l` — пусто; `go test -short -count=1 ./...` — 27 пакетов ok; `golangci-lint run ./...` — 0 issues; `make test` — exit 0, покрытие `internal/mechanics` 95,7 %. `-race` недоступен (нет cgo).
- Docker, интеграционные тесты и стенд `:8888` не трогались, `.env` не открывался, схемы, фикстуры и `shared/testkit/swarm` не менялись. Не коммитил.

<!-- dev-log T-053 итерация 2 -->
## developer#1 · T-053 · итерация 2: C-03 v1.3 и замечания ревью #1 · 2026-09-13

Ответ на ревью #1 (принять; 2 Minor, 5 Nit) и решения system-architect#1. Текст C-03 v1.3 пишет T-449 (EPIC-001, system-architect), код — здесь. Подробности — карточка, «Итерация 2».
- **C-03 v1.3 в коде**:
  - добавлены `Actor.Version` (из `e.Version`) и `Action.At` (время причины, ставит вызывающий);
  - `ChangesFor` закрепляет `ExpectedVersion` каждой сущности её версией, пишет `died_at` павшего NPC и `acquired_at` трофея из `At`;
  - `Version == 0` у меняемой сущности или пустое `At` там, где нужно время, — ошибка, а не пакет без поля.

  Пункт DoD «смерть — `status`/`died_at`/`killed_by` одним пакетом» закрыт.
- **Kind**: у NPC `kind` обязателен (`missingAttr`, пустая строка тоже). Фикстуры (`npc.json`, снапшот; и в ветке эпика после T-052) и правила дают `kind: wolf`.
- **Сверка с `FakeEncounter`**:
  - Р1/Р2 больше не расхождения: версия, `died_at` и `acquired_at` сравниваются прямо;
  - остались три обязательных расхождения: Р3 — «дефект двойника, задача EPIC-003»; Р4 — позиция побега за вызывающим, двойник обязан ставить ровно `Rules.FleePosition`; Р5 — `npcs[].last_damager` в пакете `ChangesFor` нет;
  - пути сущности встречи у двойника проверяются по набору {`round_seq`, `participants`, `state`, `resolution`, `closed_by_event_id`} (Minor-1).
- **Nit-ы**:
  - `clampHP` одна на `Rules.ClampHP` и `ChangesFor`;
  - удар без урона по стоящей цели ничего не предлагает;
  - в комментарии названо требование к нескольким ударам по одной цели;
  - `FixedMechanics.NPCTarget` отказывает на все шесть дефектов настоящего, и тест гоняет их на обеих реализациях;
  - `rest.restore` кубами отвергает `Load`, `Rules.Restore(a)` без генератора.
- **Вне списка файлов итерации 1**: `test/fixtures/fixtures_test.go` — `want.Version = e.Version` (актор теперь несёт версию сущности); `formula_test.go` — `TestRestore` под новую сигнатуру.
- **Мутанты** (новая копия в scratch, без `-overlay`, сверена `diff -r` и удалена по точному пути):
  - контрольный M00 — красный; применимые мутанты итерации 1 — красные;
  - N01–N21 (версия 0, пустое `At`, чужая версия, `died_at`/`acquired_at` нет, не в формате, не в UTC, `kind` необязателен или не сохранён, удар без урона, отдых кубами при `Load`, проверки дефектов двойника механики, D3 — двойник пишет `npcs`, беглец не там, двойник без записи смерти персонажа) — красные;
  - N09 («версия не читается») сначала выжил — все сущности тестов были версии 1; добавлена сущность версии 12 — красный.
- **Прогоны**: `go build ./... && go vet ./...` — 0; `gofmt -l` — пусто; `go test -short -count=1 ./...` — 27 пакетов ok; `golangci-lint run ./...` — 0 issues; `make test` — exit 0, покрытие `internal/mechanics` 95,9 %.
- Docker, интеграция, стенд `:8888`, `.env`, схемы и `shared/testkit/swarm` не трогались. Не коммитил.

<!-- dev-log T-060 -->
## developer#2 · T-060 · `internal/replay` — EventClock, NullTimers, Recording, сборка replay в `serve.go` · 2026-09-13

Ветка `task/T-060-replay-eventclock` (от `2196b58`), TEAM-1, Opus. Подробности — карточка `tasks/T-060.md`, раздел «Выполнение».
- **Пакет `internal/replay`**:
  - `EventClock` — монотонен по времени событий, настенных часов не читает.
  - `NullTimers` — таймеры не срабатывают.
  - `Cursor` — `Advance`/`Merge`/`Min`/`Clone`, детерминированный JSON.
  - `ReadToEnd`/`CatchUp` — конец журнала берётся один раз.
  - `Recording` + `Writer` — JSONL, побайтовый round-trip; `Index` по `(correlation_id, agent.id, phase, attempt)`, у каждой части ключа — её длина.
  - `Middleware` + `WithMiddleware` — в replay `Observe` до обработчика и `meta.replay=true`.
- **`cmd/multiverse/serve.go`**:
  - В replay контексты получают `EventClock` и `NullTimers`. Часы стоят на первом событии `--recording`, без записи — на нулевом времени.
  - **Шина получает `clock.RealTimers` в любом режиме** (C-01 v1.4). Прежнее нарушение исправлено и закреплено тестом: повтор ×3 → `dead_letters` без зависания.
  - На время replay-прогона стоит `eventbus.SetClock(EventClock)`, транспорт обёрнут middleware.
  - Нечитаемая запись отказывает в старте; лог старта называет запись.
- **Решения по ходу**:
  - Обёртки `ReadRange`/`Tail` из §6.1 заменены на `ReadToEnd`/`CatchUp`: обёртка без поведения была бы вторым именем метода.
  - Middleware ставится обёрткой транспорта; API `shared/eventbus` не менялся.
  - `SetClock` ставится только в replay, чтобы в live не затирать ручные часы тестов процесса. Источники live, `SetIDSource` и `SetRegistry` — за T-055.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути): M0 контрольный — красный, M1–M19 — все красные. M11 и M17 покраснели после усиления тестов: шпион диапазонов и чтение «с ошибкой на полпути».
- **Прогоны**:
  - `go build ./... && go vet ./...` — 0;
  - `go test -short -count=1 ./...` — ok;
  - `go test -tags e2e ./test/e2e/...` — ok;
  - `golangci-lint run ./...` — 0 issues;
  - `make test` — exit 0, `internal/replay` 97,3 %, без `-race` (нет cgo).
- **Для T-055**:
  - `serve.go` правлен в `process.run`: `timeOf` → `SetClock` в replay → `openBus` с `times.bus`. `SetIDSource`/`SetRegistry` ставить рядом.
  - Тесты сборки режима — в новом `cmd/multiverse/replay_test.go`; `serve_test.go` не менялся.
- Docker, интеграционные тесты и стенд `:8888` не трогались, `.env` не открывался. Не коммитил.

<!-- dev-log T-060 итерация 2 -->
## developer#2 · T-060 · итерация 2 по ревью #1 (3 Minor, 4 Nit) · 2026-09-13

Подробности и ответ по каждому пункту — карточка `tasks/T-060.md`, «Итерация 2». Не коммитил.
- **Mi-1**: процессный тест replay читает и через `Deps.Bus.Subscribe`, и проверяет, что `Deps.Bus` и `Deps.Journal` — один объект. `WithMiddleware` возвращает указатель, иначе сравнение паниковало бы. Мутант ревьюера M3 — красный.
- **Mi-2**: пакетный тест — событие, построенное `eventbus.Derive` в обработчике replay, лежит в журнале с `meta.replay=true` и временем причины. Мутант ревьюера M4 — красный; `shared/eventbus` не трогался.
- **Mi-3**: комментарий `Middleware` исправлен. Правило «время события — `ev.Timestamp`, следствие — через `Derive`; `Clock.Now()` при нескольких читателях зависит от планировщика» записано в doc пакета.
- **N-1**: `Recording.Start` — самое раннее ненулевое время, а не первая строка. **N-2**: `Writer.Close` делает `Sync`, комментарий честный. **N-3**: ошибка чтения посреди строки не маскируется ошибкой JSON. **N-4** — строка бэклога для T-055, не исправлялось.
- **Принято**: записанные события в шину никто не публикует — запись служит таблицей ответов `RecordedProvider`. Три вопроса ревьюера к system-architect записаны в карточку.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути): M0 контрольный — красный; прежние M1–M19, мутанты ревьюера M3r и M4r, новые M20–M22 — красные. M21 сначала выжил, после перестановки строк в тесте — красный.
- **Прогоны**: `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — ok; `go test -tags e2e ./test/e2e/...` — ok; `golangci-lint run ./...` — 0 issues; `make test` — exit 0, `internal/replay` 96,8 %.
- Docker, интеграционные тесты и стенд `:8888` не трогались, `.env` не открывался.

<!-- dev-log T-054 -->
## developer#1 · T-054 · `mechanics`: инварианты соло (inv-01, 02, 03, 09, 10) · 2026-09-13

Ветка `task/T-054-solo-invariants` (от `4119170`), TEAM-1, Opus. Подробности — карточка `tasks/T-054.md`, раздел «Выполнение».
- **Что сделано**:
  - в `invariants.go` пять проверок: inv-01 (терминальный боец в незакрытой встрече — цель или участник), inv-02 (`0 ≤ hp ≤ hp_max`), inv-03 (трофеи NPC — одни руки, одно решение, `loot_claimed_by`), inv-09 (запись смерти у нетерминальной сущности), inv-10 (одна позиция: регион мира или `outside:{world}`);
  - у inv-04/05/06 `Check == nil` до T-066, у inv-07/08 — навсегда; `TestInvariantsRegister` перевёрнут и перечисляет все десять;
  - список id `laws@v1` — `internal/mechanics/testdata/laws-v1.invariants.yaml`, сверка множеств — `TestInvariantIDsMatchTheLaws`.
- **Решения**:
  - проверка видит только мир «после» и `touched` (id изменённых сущностей, КД §4.5 п. 8), поэтому законы записаны как свойства состояния от затронутых сущностей наружу; переходы (`dead → alive`, изменение трупа) State решает по сущности «до» (§4.5 п. 5);
  - inv-01 проверяется со стороны затронутой встречи, иначе отклонялись бы `/forget` посреди боя (C-04 v1.1) и смерть NPC от чужой руки (C-05 п. 6);
  - сообщение inv-01 не называет статус: `dead`, `abandoned`, `ascended_final` дают одинаковое `Violation`;
  - нечитаемые `hp`/`inventory`/`npcs`/`participants` у затронутой сущности — нарушение, а не пропуск;
  - нарушения упорядочены `sortViolations` (`InvariantID`, `EntityID`, `Message`), повторы сняты; часы не читаются.
- **Совместимость**: `StateView` не менялся. Набор методов закреплён в тесте присваиванием в обе стороны с `guardianView{Get, ByType, WorldID}` (ADR-001 доп. 2026-09-13 п. 1а).
- **Живые данные**: мир `testdata/fixtures` и пакет `ChangesFor` смертельного удара с трофеем (с закрытием встречи) не нарушают ни одного закона; встреча, оставленная открытой над убитым волком, — одно нарушение inv-01.
- **Мутанты** (копия в scratch без `-overlay`, сверена `diff -r`, удалена по точному пути): контрольный M00 красный; M01–M52 (47) красные. В первом прогоне выжили M44/M45/M50 — эквивалентные: лишняя сортировка и дедупликация убраны из кода. M49 выжил — тест усилен. M40 не собрался — переписан.
- **Открытые вопросы**: форма `touched` (id или пути); `players_present` и inv-10 (`attrs.go`/`data-model.md` против КД §5.7); вывод погибшего из участия в групповом бою (C-05, EPIC-003); TTL возрождения в State.
- **Прогоны**: `go build ./... && go vet ./...` — 0; `gofmt -l` — пусто; `go test -short -count=1 ./...` — 27 пакетов ok; `golangci-lint run ./...` — 0 issues; `make test` — exit 0, покрытие `internal/mechanics` 96,5 %.
- Docker, интеграция, стенд `:8888`, `.env`, `shared/entity`, `shared/testkit/state`, `internal/replay`, `serve.go`, схемы и фикстуры не трогались. Не коммитил.

<!-- dev-log T-448 -->
## developer#3 · T-448 · C-02 v1.6: форма `changed[]`, `proposal_id` у create, ±2^53 · 2026-09-13

Ветка `task/T-448-changed-form-proposal-id` (база `03c551d`), TEAM-1, Opus, `contract-change`. Подробности — карточка `tasks/T-448.md`; там же текст C-02 v1.6 для T-449 и строки для EPIC-003/EPIC-004.
- **Сделано.** (а) `entity.Change{Path, Old, New, HasOld, HasNew}` с `OldPresent`/`NewPresent` и `MarshalJSON`/`UnmarshalJSON` по наличию ключей; `changes()` ставит флаги; схема `entity.updated` — `required: [path]` + `anyOf`; `changedPayload` двойника по наличию; КД §3.2, §4.8 (правило догона), §8. (б) `proposal_id` в `required` у `entity.create.proposed`; двойник без подстановки `event.id` (`refuseMalformed`); пример в `shared/contracts/validate_test.go`. (в) `jsonCompatible` отвергает число за ±2^53 в любом месте значения (разбор токенов с `UseNumber`); `inc` проверяет приращение, текущее и результат.
- **Решения по ходу (нашёл тест-свойство).** Догон `a[n]` — простое добавление без дедупликации; удаление отсутствующего пути — no-op; `ApplyOps` добавляет элемент предка, созданного предложением, если затронутый путь в конце отсутствует (`inc fresh.deep` + `remove fresh.deep` давали `changed: []` при сдвинутом хеше — дефект до T-448, нарушал инвариант T-050). Все три — в КД и тексте v1.6, на подтверждение system-architect.
- **Отклонения.** Правило переигрывания фактов — §4.8 КД, а не §4.4 (там формат `latest.json`); правка внесена в §4.8. Update без `proposal_id` двойник теперь тоже пропускает с `Warn`, а не отвергает под `event.id`. Проверка ±2^53 у `inc` — сверх `jsonCompatible`.
- **Тесты.** `TestChangedFormPerOperation` (15 строк таблицы: флаги, ключи на проводе, схема, догон), `TestCatchingUpOnChangedReproducesTheState` (4000 случайных предложений; разово 1,2 млн в копии), `TestUpdatedSchemaHoldsTheChangedForm`, `TestCreateProposalSchemaRequiresTheProposalID`, `TestChangeJSONRoundTrip`, `TestApplyOpsRefusesNumbersPastTwoToTheFiftyThird`; в двойнике — `TestChangedCarriesOldAndNewByPresence`, `TestCreateWithoutProposalIDIsPassedOver`, `TestUpdateWithoutProposalIDIsPassedOver`, `TestNumberPastTwoToTheFiftyThirdIsInvalidOp`; догон добавлен в `TestChangedListAndStateHashMoveTogether`. `Harness`, `FakeNarrator`, `FakeEncounter` не правились, их тесты зелёные. Невалидные фикстуры — по одной ошибке у каждой (разовая проверка всех 35).
- **Мутанты** (копия в scratch, без `-overlay`, контрольный первым, удалена по точному пути): M0 контрольный — красный; M1–M17 и M6s — красные. M12 (`cloneChanges` теряет флаги) в первом варианте теста выжил — подтест переписан на `null` по обе стороны.
- **Прогоны**: `go build ./... && go vet ./...` — 0; `gofmt -l` — пусто; `mvctl contracts check` — 0; `go test -short -count=1 ./...` — 0 (в одном из прогонов один раз упал `TestTheProcessRunsTheFightsOfIAlpha/fight-00` — гонка старта двойников по диагнозу самого теста; `-count=5` и два полных повтора — ok); `mvctl privacy scan testdata/` — чисто; `go test -tags e2e ./test/e2e/...` — первый прогон FAIL `TestTheProbeGivesUpOnAProcessThatExited` (таймаут пробы 15 с, не State), повтор — ok; `golangci-lint run ./...` — 0 issues; `make test` (Git Bash) — 0, `shared/entity` 94,3 %. `-race` недоступен (нет cgo). Docker, стенд, `.env` не трогались.
- Не коммитил; `contracts.md`, `shared/testkit/{swarm,gateway}`, `internal/mechanics`, `internal/replay`, `serve.go` не трогал.

### developer#3 · T-448 · итерация 2 (ревью #1 и решение system-architect#1) · 2026-09-13

Подробности — карточка `tasks/T-448.md`, «Выполнение» → «Итерация 2». Не закоммичено.
- **Ma-1 и решение п. 1.** Граница перенесена на `|x| ≤ 2^53−1`: `maxSafeInteger = 1<<53 - 1`, отказ при `|x| ≥ 2^53`. Причина: предложение приходит в State уже во `float64`, и 2^53+1 неотличимо от 2^53. Тесты:
  - `±(2^53−1)` проходят;
  - `±2^53`, `float64(2^53)`, `json.Number("9007199254740992")`, `inc` с результатом 2^53 и литерал 9007199254740993 после JSON отвергаются;
  - двойник: payload с `2^53+1` проходит JSON туда-обратно, `invalid_op` (`TestNumberPastTwoToTheFiftyThirdIsInvalidOp`); `-(2^53−1)` применяется.
- **Mi-1.** Именованные векторы: `TestChangedReportsTheAncestorAProposalCreated` (с проверкой порядка «предок после путей», R2-Mi-2 ревью T-449), `TestCatchingUpAppendsWithoutDeduplication`, `TestCatchingUpSkipsAPathAnEarlierEntryAlreadyRemoved`. Мутант «догон через `op: append`» убивает только именованный вектор, генератор его не видит.
- **Mi-2, N-1, N-2, N-3.**
  - Наличие решают только `HasOld`/`HasNew`: `OldPresent`/`NewPresent` удалены. `MarshalJSON` возвращает ошибку, если нет ни одного флага или значение не `nil` при опущенном флаге.
  - `UnmarshalJSON` на `null` ничего не делает.
  - Текущее значение `inc` за пределом получает причину `ReasonCurrentRange`.
  - Правило догона: `null` по пути списка — отсутствующий список; `a[n]` за концом — повреждённый факт. Помощник `catchUp` возвращает `errCorruptFact`, `replayChanged` роняет тест, закреплено `TestCatchingUpRefusesAnElementPastTheEndOfItsList`.
- **Решение пп. 2–6.**
  - КД: §4.8 (замена узла объектом, `a[n]` за концом → `state_divergence`), §4.5 п. 1 и §9 (без `proposal_id` — `Warn` и пропуск), §4.5 (создание: `attributes` через `JSONCompatible`, `proposal_id` в факте), §3.2, §8, §14.
  - Схема `entity.created`: `proposal_id` в `required`.
  - `entity.JSONCompatible` экспортирована, `applyCreate` двойника проверяет `attributes` → `invalid_op`.
  - Строка для EPIC-003/004 о числах строкой.
  - README `shared/entity` и README фикстур, текст v1.6 в карточке (N-3; правило порядка с пометкой R2-Mi-2).
- **Вне зоны.** Одна строка теста EPIC-003: `shared/testkit/swarm/fake_narrator_test.go`, помощник `created()` получил `proposal_id`. Без неё `TestTheNarratorWorksOffTheBusToo` падал на новой схеме `entity.created`. Вынесено оркестратору.
- **Мутанты** (копия `t448i2-mut` в scratch, без `-overlay`, удалена по точному пути):
  - K0 (контрольный) — красный;
  - K1, K1s, K1f, K2, K3, K3n, K4, K5, K5b, K5c, K6, K7, K7b, K8b, K9, K10, K11, K12 — красные;
  - K8 не собрался и заменён K8b.
- **Прогоны:**
  - `go build ./... && go vet ./...` — 0;
  - `gofmt -l` — пусто;
  - `mvctl contracts check` — 0;
  - `go test -short -count=1 ./...` — 0;
  - `go test -tags e2e ./test/e2e/...` — ok;
  - `golangci-lint run ./...` — 0 issues;
  - `make test` (Git Bash) — 0; покрытие `shared/entity` 93,9 %, `shared/testkit/state` 89,0 %, gate `internal/mechanics` 94,9 %;
  - `-race` недоступен.
  - Docker, стенд, `.env`, интеграционные тесты не трогались.

<!-- dev-log T-055 -->
## developer#3 · T-055 · `internal/state`: конвейер предложение → факт, источники конструкторов в `serve.go` · 2026-09-13

Ветка `task/T-055-state-pipeline` (база 34bdcca), TEAM-1, Opus. Не закоммичено. Подробности, таблица DoD → тест, мутанты и рецепт синхронизации с T-446 — карточка `tasks/T-055.md`.
- **Сделано.**
  - `internal/state`: `memstore` (рабочий набор в памяти, копии на входе и выходе), `proposal.go`, `Applier` (`apply.go`), `facts.go`, `worker.go` (горутина на мир со своим `recover`), `context.go` (контекст `state`: подписка-посредник, `Stop`, `Health`).
  - `cmd/multiverse`: регистрация `state` (`contexts_state.go`), один блок источников конструкторов в `serve.go`, Н-1 и Н-2 приёмки T-060.
  - `MV_STATE_WORLDS` в манифесте и `.env.example`.
- **WithCauseID — «да».** `parts` — id сущности; у отказа — id сущности или пусто.
  - Доводы. Ответ на предложение (факты и отказы) публикуется целиком, и только потом копии заменяют сущности в `memstore`, а `proposal_id` попадает в окно. Если публикация упала на втором факте пакета, мир не изменился и шина повторяет то же событие. Первый факт уходит снова — с тем же id, и потребитель, гасящий дубли по id (C-02 «Гарантии»: «факты досылаются»), его не увидит. Со случайным id повтор выглядел бы новостью.
  - Уникальность частей. Пакет называет каждую сущность один раз (C-02 v1.3 проверяется до всего). Отказ всего пакета — один, под своей сущностью или без неё.
  - `proposal_id` в `parts` не нужен. Новое событие под тем же `proposal_id` (C-05 п. 1) несёт другой `causation_id`, и id отличается при любых частях. Такой повтор гасит окно `proposal_id`, фактов он не публикует.
  - Тест `TestAFailedPublicationIsRetriedWithTheSameIDs`, мутант M3 — красный.
- **Паника — ошибка шине, а не повторная паника.**
  - Что происходит. Паника в `Applier` ловится `recover` worker'а. Пишется `Error` со стеком и `handled=false`, мир остановлен, `/health fail` с текстом паники по миру. Обработчик подписки получает `ErrWorldStopped` и отдаёт шине ошибку. Шина повторяет ×3 и паркует событие в `dead_letters` с причиной. Каждое следующее предложение этого мира уходит тем же путём, другие миры работают.
  - Почему не повторная паника. Паника в worker'е и так уже записана со стеком. Повторная паника в обработчике дала бы второй стек на то же событие и ложную панику на каждое последующее предложение, где паники не было. Это исказило бы `service_panics` (NFR-012). Ошибка оставляет событие с причиной в `dead_letters`, это след для разбора.
  - Цена. Пауза повторов (100/500/2000 мс) на каждое предложение остановленного мира задерживает подписку и для остальных миров. В MVP-1 мир один, и остановленный мир — уже `/health fail`.
  - `Delivery` панику worker'а поймать не может: `Applier` работает в другой горутине. Поэтому `recover` на границе worker'а обязателен.
  - Тест `TestAPanicStopsItsWorldAndOnlyItsWorld`, мутанты M7, M14, M23 — красные.
- **Посредник (C-01 v1.6).** Окно id событий в `dispatch`: `Has` до worker'а, `Add` после ответа без ошибки. Окно `proposal_id` — внутри `Applier`, заполняется после публикации всего ответа. Тест `TestTheRetryOfAFailedDeliveryReachesTheWorker`, мутант M1 — красный.
- **Остановка (C-01 v1.7).** `Stop` отменяет контекст подписки (он свой: `WithCancel(WithoutCancel(ctx))`) и ждёт возврата обработчика в пределах своего `ctx`. Worker доводит принятое предложение с `context.WithoutCancel`, иначе публикация второй половины пакета оборвалась бы на отменённом контексте (мутант M8). Процесс закрывает шину после `StopAll`, как и раньше.
- **Отклонения и решения** (подробно в карточке):
  - чтение подпиской `core.state`, а не курсором, до T-059;
  - правило C-02 v1.3 включено, потому что без него ломается «версии строго +1»;
  - предложение без мира в конверте пропускается;
  - в боях I1-α `cmd/multiverse` контекст `state` обслуживает другой мир, пока стенд держит `FakeState`;
  - `memoryOptions().idSource` = `uuid`.
- **Источники конструкторов.** `installSources(deps)` до `openBus`, `defer installSources(runtime.Deps{})` до `defer closeBus`. Сбрасываются значения по умолчанию, а не прежние: у `eventbus` нет читателя источников (бэклог EPIC-001). Комментарий про `t.Parallel` стоит рядом с установкой. Тест в live и replay после `testkit.Deterministic`; мутанты M11, M13 — красные.
- **Разбор доступов (без cgo, `-race` недоступен).**
  - Поля `Context` — под `c.mu`, `dispatch` берёт worker под ним же.
  - `Dedup` — со своим мьютексом; `Has`/`Add` посредника вызывает одна горутина подписки, по событию за раз.
  - `Applier` вызывает только горутина worker'а.
  - `memstore` — `RWMutex`, наружу только копии; `Health` читает `Len` под блокировкой.
  - `worker.failure` — под `w.mu`.
  - Сущности плана — копии из `memstore`, локальные для worker'а до `Put`, который снова копирует.
  - `payload.attributes` факта создания — карта копии, сериализуется в `Publish` до `Put`.
  - В тестах хук шины — под мьютексом, счётчик попыток — `atomic`.
  - Ожидания в тестах — опрос с дедлайном 60 с как верхней границей; окон короче секунды на реальном времени нет, кроме бюджета 50 мс в `TestStopIsBoundedByItsContext`: там проверяется только «вернулся с ошибкой дедлайна не позже 30 с».
  - Стресс `-count=40 -cpu 1,2,8 ./internal/state/...` и `-count=5 -cpu 1,4 ./cmd/multiverse/` — зелёные.
- **Мутанты** (копия `t055-mut` в scratch, без `-overlay`, контрольный первым, удалена по точному пути): 24 из 24 красные.
- **Прогоны:**
  - `go build ./... && go vet ./...` — 0;
  - `gofmt -l` — пусто;
  - `go test -short -count=1 ./...` — ok;
  - `go test -tags e2e ./test/e2e/...` — ok;
  - `golangci-lint run ./...` — 0 issues;
  - `mvctl contracts check`, `mvctl env check`, `make compose-lint` — 0;
  - `make test` — exit 0, `internal/state` 93,5 %.
  - Интеграционных тестов у задачи нет. Docker, стенд, `.env` не трогались.

<!-- dev-log T-055 итерация 2 -->
## developer#3 · T-055 · итерация 2 по ревью #1 (1 Major, 4 Minor, 3 Nit) · 2026-09-13

Ветка `task/T-055-state-pipeline` (база 34bdcca), TEAM-1, Opus. Не закоммичено. Правки итерации 1 тоже не закоммичены.

- **Ma-1 — фантомный факт. Сделано по решению оркестратора: комбинация Б+А, КД §9 не менялся.**
  - Б. `Applier.publish` при ошибке `Publish` публикует то же событие ещё раз: тот же объект, те же id и байты. Пауза экспоненциальная, 100 мс → ×2 → потолок 5 с, по `ApplierConfig.Timers`; контекст берёт их из `Deps.Timers`, по умолчанию `clock.RealTimers`. Шине ошибка не возвращается.
  - Уже вышедшие события ответа повторно не публикуются, повторяется только невышедшее.
  - Каждая неудача пишется в `Warn` с `handled=true`, `proposal_id`, `event_id`, номером попытки и паузой. Пока идут попытки, мир в `/health` — `degraded` с `publish_attempts_failed`.
  - Сами публикации идут на `context.WithoutCancel`: шина отказывает отменённому контексту. Паузы ждут контекст мира, то есть контекст обработчика подписки.
  - А. Отмена этого контекста (`Stop`) обрывает попытки. Мир останавливается с `ErrWorldStopped` + `ErrPublishFailed`. Пишется `Error` с `handled=false`, `reason=publish_failed`, id вышедших событий, ожидавшим событием и `remedy`. В `memstore` ничего не пишется, `proposal_id` в окно не попадает.
  - `Delivery` на отменённом контексте возвращает `ctx.Err()`, без `dead_letters`, и событие остаётся незакоммиченным. Дальнейшие предложения мира получают `ErrWorldStopped` без решения. `/health` мира — `fail`, `reason: publish_failed`.
  - Остановка мира хранится в `Applier`, а не в worker'е. Поэтому она переживает `Stop`/`Start` того же экземпляра. Паника тоже переехала туда: `reason: panic`.
  - `Put` в `memstore` после вышедшего ответа тоже останавливает мир (`keepFailed`). Сегодня путь недостижим: `Put` падает только на пустом id или мире.
  - Детерминированный путь ревью. Отказ по набору с `entity.id` без `entity.type` публикуется без `entity` в payload, потому что схема требует пару целиком. Часть `WithCauseID` — по-прежнему id сущности. Без этого отказ не прошёл бы схему и держал бы мир в попытках вечно.
  - Регрессионный тест по зонду P1 — `TestNoTwoFactsOfOneVersionWhileAPublicationKeepsFailing`. Факт волка не выходит 30 раз, это больше, чем шина повторила бы доставку. Проверяется:
    - `dead_letters` пуст;
    - 31-я попытка проходит;
    - `prop-next` решается от v2;
    - у каждой сущности на шине один факт на версию (`assertOneFactPerVersion`);
    - каждая версия мира объявлена фактом (`assertAnnounced`);
    - `/health` в ходе попыток `degraded`, после — `ok`.
  - `TestAStopThatEndsTheAttemptsStopsTheWorld`: `Stop` посреди попыток. `dead_letters` пуст, мир v1, `Error` с `publish_failed`. После повторного `Start` — `/health fail` с причиной, `prop-next` уходит в `dead_letters` с `ErrWorldStopped`, второго факта v2 нет.
  - Уровень `Applier`: `TestAFailedPublicationIsPublishedAgainAsItIs` (те же байты во всех попытках, мир не двигается до выхода ответа) и `TestAnAnswerAbandonedWithTheContextStopsTheWorld` (update и create).
  - Паузы: `TestThePauseBetweenAttemptsDoublesUpToTheCap`.
  - Ожидания — `clock.Manual`, двигаемый в цикле опроса (`applyAdvancing`, `advancing`). Реальное время только ограничивает дефект сверху, 60 с.
  - `TestTheRetryOfAFailedDeliveryReachesTheWorker` удалён: повтора доставки при ошибке публикации больше нет. Норму посредника «запомнить после ответа» держит M1 — красный на `TestAPanicStopsItsWorldAndOnlyItsWorld` и `TestAStopThatEndsTheAttemptsStopsTheWorld`.
- **Mi-1.** Мир читается из `Event.World` напрямую (`worldOf`) и в `Applier`, и в `dispatch`. Предложение без мира — `Warn` «proposal without world in the envelope: no worker is addressed» с `event_id`, `type`, `proposal_id`. Чужой мир остаётся на `Debug`. Норма «world обязателен» — вопрос к system-architect, код пропускает. Тесты: `TestWhatIsNotAProposalOfThisWorldIsPassedOver` (в payload есть `world_id` нашего мира — legacy-фолбэк его бы адресовал), `TestAProposalWithoutAWorldIsReported`.
- **Mi-2.** `Start` отказывает, пока подписка или любой worker прошлого запуска не вернулись (`previousRunEnded`). `Stop` по таймауту всё равно закрывает `quit` всех worker'ов (`signalStop`). `subErr` пишет только горутина текущего запуска. Тест `TestStopIsBoundedByItsContext`: после таймаута `Start` — ошибка; после освобождения хука `Start` проходит, следующее предложение даёт v3, один факт на версию.
- **Mi-3.** `TestACreateKeepsNothingUntilItsFactIsOut`: сущности нет в мире ни при одной попытке, один `entity.created` под id обеих попыток. `TestACreateUnderAnAppliedProposalIDIsSilent`: новое событие под применённым `proposal_id` не публикует ничего. Create с оборванными попытками — подтест `create` в `TestAnAnswerAbandonedWithTheContextStopsTheWorld`.
- **Mi-4.** `TestAnEventDeliveredTwiceIsAnsweredOnce`: отказ `version_conflict`, те же байты через `bus.Append`, затем сторож — один ответ. `TestTheRefusalsOfOnePackageHaveIDsOfTheirOwn`: два отказа пакета с разными id, повтор того же события даёт те же id.
- **N-1.** `TestTheSourcesOutliveTheBusOfTheRun` (`cmd/multiverse/sources_test.go`). Транспорт процесса строит корневое событие в `Close`, его id — `seq-N` генератора прогона.
- **N-2.** Инструкция про T-446 снята из комментария `contexts_state.go`, осталась одна фраза про `MV_STATE_WORLDS`. П. 2 рецепта синхронизации в карточке дополнен.
- **N-3.** `last_change.batch_size` = число применённых наборов (`len(plans)`), как в КД §4.5 п. 9. Проверка — в `TestANonAtomicPackageRefusesOnlyWhatFailed` (2 из 4). Двойник `shared/testkit/state` пишет размер предложенного пакета. Это чужой файл, расхождение уходит в T-056 вместе с заменой двойника.
- **Мутанты.** Копия дерева в scratch `t055i2-tree` (`tar` без `.git`, `Docs`, `services`), без `-overlay`. Контрольный — первым, базовый прогон копии зелёный. После каждого мутанта файл восстановлен, копия сверена с рабочей папкой (`diff -r` — идентично) и удалена по точному пути. Итог: **21 из 21 красные**.
  - C0 (контрольный): `version` факта +1.
  - MA1: ошибка шине после 3 попыток.
  - MA2: невышедшее событие пропускается, ответ сохраняется («применить без публикации»).
  - MA3: оборванный ответ не останавливает мир.
  - MA4: публикация на отменяемом контексте.
  - MA5: попытки не видны в `/health`.
  - MA6: отказ с пустым `entity.type`.
  - MA7: паника не останавливает мир.
  - MI2a: `Stop` по таймауту не сигналит worker'ам.
  - MI2b: `Start` не ждёт прошлого запуска.
  - X9: `Stop` не останавливает worker'ы.
  - X4: create, `Put` до публикации.
  - X8: create без окна `proposal_id`.
  - X1: посредник без окна id событий.
  - X2: одна `WithCauseID`-часть у всех отказов.
  - M1: посредник запоминает событие до ответа.
  - MI1a: `Applier` читает мир через `GetWorldIDFromEvent`.
  - MI1b: предложение без мира — `Debug`.
  - N3: `batch_size` = размер предложенного пакета.
  - X11: сброс источников до `closeBus`.
  - Не проверялся мутант «`dispatch` через `GetWorldIDFromEvent`». Для событий, прошедших схему, он эквивалентен: у предложения нет ключа мира в payload.
- **Прогоны** (go1.26, windows/amd64):
  - `go build ./... && go vet ./...` — 0; `gofmt -l` — пусто;
  - `go test -short -count=1 ./...` — ok;
  - `go test -short -count=20 -cpu 1,4 ./internal/state/...` — ok (12 с); `go test -count=5 -cpu 1,4 ./cmd/multiverse/` — ok;
  - `go test -tags e2e -count=1 ./test/e2e/...` — ok (11 с);
  - `golangci-lint run ./...` — 0 issues;
  - `mvctl env check` — 68 переменных, 0; `mvctl contracts check` — 65 типов, 0;
  - `make test` (Git Bash) — exit 0, `internal/state` 94,1 % (430 из 457), без `-race` (нет cgo).
  - Интеграционные тесты, Docker, стенд `:8888`, `.env` не трогались.
- **Риски.**
  - Событие, которое шина не примет никогда (дефект конструктора, схема), держит подписку `system_events` в попытках для всех миров процесса до `Stop`. Мир `degraded`, `Warn` на каждую попытку. Путь пустого `entity.type` закрыт, других известных нет. Классифицировать ошибки шины как окончательные нельзя: у ошибки схемы нет sentinel в `shared/eventbus`.
  - При недоступном брокере стоит вся подписка процесса — это цена варианта Б, её назвал ревьюер.
  - Остановленный мир восстанавливается только рестартом процесса. Догон по фактам журнала (§4.8) — T-059, до него память после рестарта пуста.
