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

<!-- dev-log T-458 -->
## developer#3 · T-458 · Запись сессии: `shared/recording`, `ReadJournal`, `Deps.Recording`, маршрут часов replay · 2026-09-13

Ветка `task/T-458-session-recording` (от `a037efb`), TEAM-1, Opus. Подробности — карточка `tasks/T-458.md`, раздел «Выполнение (developer)». Сделаны части А и Б. Кроме двух `git mv` в индекс ничего не добавлялось, коммитов нет.
- **Что сделано.**
  - `git mv internal/replay/recording{,_test}.go shared/recording/` отдельным шагом. Против `a037efb` сходство 74 % и 80 %.
  - API пакета — `Open`/`Read`, ошибки с префиксом `recording:`.
  - `LLMOutputKeyOf` не усекает `attempt`: целое ≥ 1, одинаково из JSON и из Go.
  - Новый `recording.ReadJournal` с контракт-тестами на membus.
  - Поле `runtime.Deps.Recording`.
  - `serve.go` читает запись один раз (`timeOf`, шпион `process.openRecording`), монтирует маршрут часов в replay и выводит новую справку `--recording`.
  - Тест `Derive` в новом `shared/eventbus/derive_replay_test.go`.
  - `**/shared/recording/**` добавлен в `no-testkit-in-production`.
  - `EventClock.Advance` и `ErrClockBehind`.
  - `internal/replay/clockroute.go`: `POST /v1/admin/replay/clock` отвечает 204/400/405/409, 403 даёт `runtime.AdminOnly`, в live — 404.
- **Решения по ходу.**
  - В `process` добавлено поле `openRecording`, а не пакетная переменная: так уже внедрён `openBus`, и `serve_test.go` (EPIC-004) менять не нужно.
  - Равный `at` часы не трогает: `Now()` сохраняет исходную зону.
  - Тела 400/405/409 — `{"error": <код>, "message": <текст>}`, форма записана в карточку для EPIC-004.
- **Отклонения.**
  - Тест «`llm.output` без ключа» идёт на membus с `SkipValidateOnRead`. По схеме и политике роя запись без ключа не проходит валидацию, и на валидирующей шине её паркует `Delivery` ещё до обработчика.
  - Зонд depguard использует `internal/mechanics`: импорт `internal/replay` из `shared/recording` — цикл импорта.
- **Факт зонда А5.** В replay `ReadJournal` над `Deps.Journal` проходит через middleware T-060. Чтение сдвигает `EventClock` на самое позднее время прочитанного, у событий записи `Meta.Replay == true`. Закреплено тестом, поведение не менялось, вопрос — system-architect.
- **Мутанты** (копии `scratchpad/t458-<имя>`, без `-overlay`, удалены по точным путям). Контрольный мутант первым, красный. M6, M7, M12, M13, M16, M20–M22 на новом месте красные. Из 21 нового мутанта 19 красные.
  - Выжил А5 «убрана ветка `End ≤ from`»: на membus эквивалентен.
  - Выжил Б1 «`Now()` + `Observe` без общей блокировки»: DoD это предполагал, атомарность держат блокировка и `-race` в CI.
  - Зонды depguard: импорт `internal/*` и `shared/testkit` в не-тестовом файле красные, `testkit` в `_test.go` — зелёный.
- **Прогоны** (go1.26.8, windows/amd64):
  - `gofmt -l` — пусто;
  - `go build ./... && go vet ./...` — 0;
  - `go test -short -count=1 ./...` — 31 пакет ok;
  - `go test -tags e2e -count=1 ./test/e2e/...` — ok;
  - `golangci-lint run ./...` — 0 issues;
  - `mvctl env check` — 68 переменных, 0; `contracts check` — 65 типов, 0;
  - `make test` — exit 0, `internal/replay` 100 % (было 96,8 % вместе с записью), `shared/recording` 96,6 %; без `-race` (нет cgo).
  - Интеграционные тесты не нужны по DoD, Docker, стенд `:8888` и `.env` не трогались.
- **Слияние.** С кончиком эпика `281348a` по коду расхождений нет. `.golangci.yml` сливается с EPIC-003 через `git merge-file` без конфликтов. `serve_test.go` задача не меняет.

<!-- dev-log T-458 iteration 2 -->
## developer#3 · T-458 · итерация 2: чтение истории без часов replay, шаблон и тела маршрута часов · 2026-09-13

Ветка `task/T-458-session-recording`, TEAM-1, Opus. Основание — ревью system-architect#1 (У-1…У-4, А5 — вариант (б)) и ревью #1 code-reviewer#3 (Mi-1, Mi-2, N-1…N-5). Подробности — карточка `tasks/T-458.md`, раздел «Итерация 2 (developer)». Индекс не менялся (два `git mv`), коммитов нет.
- **Что сделано.**
  - У-1: `ReadJournal` читает журнал под меткой контекста, `recording.InReadJournal(ctx)` её проверяет. Middleware replay пропускает помеченный обработчик без `Observe` и без `Meta.Replay`. В зону задачи добавлены `internal/replay/middleware.go` и `middleware_test.go`. Зонд А5 перевёрнут: часы после `ReadJournal` стоят на старте записи, `Meta.Replay` как в журнале, `POST …/clock` раньше прочитанных событий — `204`.
  - У-2: маршрут монтируется шаблоном `POST /v1/admin/replay/clock`. Процесс с контекстом, монтирующим `POST /v1/admin/{path...}`, стартует; `GET` получает `405` с `Allow: POST` от mux.
  - У-3 и Mi-2: тела `400`/`409` (и защитного `405`) — `{"error":{"code","message"}}`. `403` у `AdminOnly` не тронут.
  - У-4: `Read` — `recording: line N: …`, `Open` — `recording: <path>: line N: …` и `recording: open <path>: …`.
  - Mi-1: процессный тест маршрута часов — с записью и без.
  - N-1…N-5: объяснение мутанта `Advance` (логическая гонка, не гонка данных, `-race` не видит, атомарность держит блокировка; поправка к записи итерации 1 выше); `rec == nil` при ошибке; `times.recording != nil` и `recordingPath`; `shared/recording/*` в `desc` depguard; `wholeAttempt` — все целые типы и `float32`.
- **Решения по ходу.** Метка ставится только на `ReadRange`, `End` читается с контекстом вызывающего. `ClockPath` остался путём, метод добавляется в `serve.go`. Двойник журнала в тесте ошибок теперь отдаёт одно событие до ошибки, чтобы R2 было что вернуть.
- **Мутанты** (копии `scratchpad/t458i2-<имя>`, без `-overlay`, удалены по точным путям): контрольный первым — красный; R1, R2 и 16 новых на У-1…У-4, N-2, N-5 — все красные. Мутант «шаблон без метода» в первой редакции не собрался, переписан и перепрогнан — красный по тестам.
- **Прогоны** (go1.26.8, windows/amd64): `gofmt -l` пусто; `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — все ok; e2e — ok; `golangci-lint run ./...` — 0 issues (повтор с `--allow-parallel-runners` после отказа из-за линтера другой папки); `make test` — exit 0, `internal/replay` 100 %, `shared/recording` 97,8 %. Docker, стенд `:8888` и `.env` не трогались.

## developer#2 · T-056 · `state`: владение, законы мира, дедуп, матрица отказов; двойник `FakeState` = `Applier` · 2026-09-13

Ветка `task/T-056-state-invariants` (от 862c8c0), TEAM-1, Opus. Подробности, таблица DoD → тесты и мутанты — карточка `tasks/T-056.md`, «Выполнение (developer)». Коммитов нет. В DoD включены строки T-457 для T-056 (по сообщению оркестратора).
- **Что сделано.** `internal/state`: `ownership.go` (предлагающий из конверта; таблица `contracts.OwnershipRules` + нормы C-02 v1.4/v1.5 поверх неё; переход в `abandoned`; rest во встрече и выше `hp_max`), `invariants.go` (`overlayView`, первый закон по реестру, `touched` с пустым `changed[]`, non-atomic — пересчёт), `dedup.go` (окно + `last_change`), шаг 5 `dead_entity` по сущности «до», тип набора против мира, форма операций до дедупа, `Config.Invariants`. `internal/mechanics`: inv-01 по участникам — только затронутые пакетом (C-05 v1.8 п. 9 (б)). `registry_state.go`: `WorldRequired` у двух типов предложений. `shared/testkit/state`: двойник — `Applier` над `memstore` без таблицы владения, `WithInvariants()` настоящий. `.golangci.yml`: правило `shared-testkit-state`. Стенд боёв I1-α — на `state.Context` процесса.
- **Решения по ходу.** Двойник без таблицы владения — иначе красные тесты потребителей и восемь тестов T-448 из DoD (КД §8 «без опции — только версии и ops»). Дедуп по `last_change` — «любая» целевая сущность, не «все»: частично применённый non-atomic пакет иначе применился бы второй раз. Остановленный мир — обычная ошибка, не `Permanent` (C-01 v1.10, КД §9; строка `tasks.md` про `Permanent(ErrWorldStopped)` устарела). Rest во встрече — `law_violation` без `invariant_id`, выше `hp_max` — `inv-02` (с тимлидом не согласовано, вопрос в карточке). `world.go` не создан: признак «нет `latest.json`» появится с хранилищем T-057 (вопрос).
- **Блокер (EPIC-003).** (1) `FakeEncounter.wound` пишет игроку `died_at`/`killed_by` — строка `task`/`player` §4.6 этого не даёт; настоящий State отвечает `level_violation`, бои стенда со смертью персонажа падают. Зонд: с условием «только NPC» `cmd/multiverse` зелёный. (2) Тесты `FakeEncounter` выбирали кубики по id, которые тратил двойник v0; `Applier` выводит id фактов из причины (T-055) и последовательность не тратит — `TestTheWolfDies`, `TestTheEncounterEntityRecordsTheFightItHeld`, `TestTheTrophyIsHandedOutOnce`, `TestEveryProposalPassesTheOwnershipTable` краснеют. Зонд: без `WithCauseID` у фактов — зелёные. Файлы EPIC-003 не трогались.
- **Мутанты** (`scratchpad/t056-mut`, без `-overlay`, контрольный первым, удалена по точному пути): 28 из 28 красные (M3 в первой редакции не собрался — переписан). Зонды `t056-base`, `t056-probe-ids`, `t056-probe-stand` удалены по точным путям.
- **Прогоны** (go1.26, windows/amd64): `gofmt -l` пусто; `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — ok, кроме `shared/testkit/swarm` (4) и `cmd/multiverse` (бои со смертью персонажа) — блокер; e2e — ok; `golangci-lint run ./...` — 0 issues; `mvctl contracts check`, `env check` — 0; `make test` — exit 2 по тем же двум пакетам, `internal/state` 94,0 %, `internal/mechanics` 96,5 %. Интеграционные наборы, Docker, `.env` и стенд `:8888` не трогались.

### developer#2 · T-056 · итерация 1b: блокер EPIC-003 снят · 2026-09-13

По решениям оркестратора на отчёт итерации 1. Подробности — карточка `tasks/T-056.md`, «Итерация 1b / блокер снят».
- **Что сделано.** `FakeEncounter.wound` пишет `died_at`/`killed_by` только NPC (§4.6). Кто записывает смерть персонажа — вопрос к system-architect. Тесты роя, которым нужна смерть волка, выбирают атаки по исходу (`swingUntilTheWolfFalls`: удар персонажа попадает, укус волка мимо) и больше не зависят от числа id, которые тратил двойник v0. `TestTheCharacterDies` проверяет, что персонажу не предлагаются `died_at`/`killed_by`. `WithCauseID` фактов State не тронут.
- **Решения оркестратора.** `ErrWorldStopped` — обычная ошибка; исключение `shared-testkit-state` пока остаётся; rest — «не выше `hp_max`», во встрече — `law_violation` без `invariant_id`; закон вне пакета при `atomic=false` — как есть, вопрос к system-architect; `world.go` — T-057/T-059.
- **Мутанты** (`scratchpad/t056-mut1b`, без `-overlay`, контрольный первым, удалена по точному пути): B1 «персонажу снова `died_at`» — красный бой стенда `TestTheProcessTellsTheDeathOfACharacter` и `TestTheCharacterDies`; B2 «помощник пускает укус» — красный; B3 «факт без `WithCauseID`» — красный; регресс M1, M5, M15, M19, M20, M27 — красные.
- **Прогоны** (go1.26, windows/amd64): `gofmt -l` пусто; `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — 0; `shared/testkit/{swarm,state,gateway}` и `cmd/multiverse` по три прогона — ok; e2e — ok; `golangci-lint run ./...` — 0 issues; `mvctl contracts check`, `env check` — 0; `make test` — exit 0 (`internal/state` 94,3 %, `internal/mechanics` 96,5 %). Docker, `.env`, стенд `:8888` не трогались.

### developer#2 · T-056 · итерация 2 по ревью #1 (2 Major, 5 Minor, 2 Nit) · 2026-09-13

Подробности — карточка `tasks/T-056.md`, «Итерация 2».
- **Что сделано.** Ma-1: в неатомарном пакете каждый набор получает ответ; закон вне оставшихся наборов отвергает каждый набор под своей сущностью с `invariant_id`, фильтр отказов по `Ref.ID` снят. Ma-2: путь не в каноническом виде (`status.`, `.description`, `a..b`, `[]`) — `invalid_op` на шаге 1. Mi-1: «во встрече» у rest — по сущности до операций. Mi-2/Mi-3: тесты окна `proposal_id`, дочернего пути под префиксом и агента с неизвестным уровнем. Mi-4: страж стенда `oneStateOverTheWorld` (только `core/state`, одна версия — один факт; факты с пустым `changed[]` не считаются). Mi-5: `fake_encounter.go` дословно с кончика EPIC-003 (T-452), тесты EPIC-003 плюс `swingUntilTheWolfFalls`; рецепт слияния — в карточке. N-1: тесты остановленного мира — в `stopped_world_test.go`; N-2 — в бэклог.
- **Решения по ходу.** Отказ за закон вне пакета называет сущность своего набора: так издатель сопоставляет его с набором, закон виден в `invariant_id` (вопрос к system-architect). Страж стенда не считает факты с пустым `changed[]`: такой факт выходит под найденной версией (C-02), и в 1 из 16 боёв страж без этого исключения ложно краснел.
- **Мутанты** (`scratchpad/t056-mut2`, без `-overlay`, контрольный первым, удалена по точному пути): R1, R2, R3, R3b, R5, R7, P1, P2, B1, B2 — все красные; R7 краснеет на страже («published by testkit/state»).
- **Прогоны** (go1.26, windows/amd64): `gofmt -l` пусто; `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — 0; `cmd/multiverse` ×10, `shared/testkit/{swarm,state,gateway}` и `internal/state` ×3 — ok; e2e — ok; `golangci-lint run ./...` — 0 issues; `mvctl contracts check`, `env check` — 0; `make test` — exit 0 (`internal/state` 94,7 %, `internal/mechanics` 96,5 %). Docker, `.env`, стенд `:8888` не трогались.

### developer#2 · T-056 · итерация 3 по ревью #2 (1 Major, 1 Minor, 1 Nit) · 2026-09-13

Подробности — карточка `tasks/T-056.md`, «Итерация 3».
- **Что сделано.** Ma-3: переход статуса и rest решаются по сущности до и после `ApplyOps`. Статус после операций не строка или не из матрицы — `invalid_op`. `hp` после rest не целое число — `invalid_op`, вне `[0, hp_max]` — `law_violation {inv-02}`. Путь ниже скалярного атрибута (`status.x`, `hp.x`, `hp_max.x` …) отвергается на шаге 1. Mi-6: страж стенда строгий — без исключения по id, пара (сущность, версия) одним фактом, событие не дважды, пустой `changed[]` — только по id. N-3: индекс без ведущих нулей.
- **Решения по ходу.** Защита двумя слоями: шаг 1 ловит путь ниже скаляра у любого предлагающего, проверка по сущности держит статус и rest, даже если путь пройдёт грамматику. Второй слой проверен тестом `plan` мимо шага 1 и без законов. Перенос в `shared/entity` — вопрос к system-architect (C-03).
- **Мутанты** (`scratchpad/t056-mut3`, без `-overlay`, контрольный первым, удалена по точному пути): S1–S5, N3, K5, K6, K7 — все красные.
- **Прогоны** (go1.26, windows/amd64): `gofmt -l` пусто; `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — 0; три теста стенда ×20 — ok; e2e — ok; `golangci-lint run ./...` — 0 issues; `mvctl contracts check`, `env check` — 0; `make test` — exit 0 (`internal/state` 94,7 %). Docker, `.env`, стенд `:8888` не трогались.

### developer#2 · T-056 · итерация 4: условия system-architect У-1, У-2; ревью #3 Mi-8, N-4 · 2026-09-13

Подробности — карточка `tasks/T-056.md`, «Итерация 4».
- **Что сделано.** У-1: `canonicalPath` отвергает ключ, который читается как целое (`inventory.0`), и индекс длиннее девяти цифр. Зонд подтвердил: в `shared/entity` `remove inventory.0` даёт путь элемента в `changed[]`, и хэш после догона расходится. На входе State закрыто шагом 1, остальное — в бэклог задачи грамматики. У-2: `scalarAttributes` — все скаляры `data-model.md` §3, тест перечня по типам и строки матрицы по каждому новому типу сущности. Mi-8: rest при `hp_max+1` и без `hp_max` → `law_violation {inv-02}`. N-4: положительная строка `inventory[10]`.
- **Не делалось.** Норма rest по ответу 5 (`hp == hp_max`, только `hp`) и строка `system` по ответу 7 — отдельной задачей после слияния C-02 v1.8, по решению оркестратора.
- **Мутанты** (`scratchpad/t056-mut4`, без `-overlay`, контрольный первым, удалена по точному пути): U1a, U1b, M5, U2a, U2b, M1, M2 — все красные.
- **Прогоны** (go1.26, windows/amd64): `gofmt -l` пусто; `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — 0; три теста стенда ×20 — ok; e2e — ok; `golangci-lint run ./...` — 0 issues; `mvctl contracts check`, `env check` — 0; `make test` — exit 0 (`internal/state` 94,9 %). Docker, `.env`, стенд `:8888` не трогались.

### developer#3 · T-471 · законы мира в процессе, норма rest `hp == hp_max`, пустая строка `system` (C-02 v1.8) · 2026-09-14

Подробности — карточка `tasks/T-471.md`, «Выполнение (developer)».
- **Что сделано.** `cmd/multiverse` строит State с законами из `MV_RULES_PATH`: имя и умолчание `rules/dark-forest.yaml` взяты из `design.md` §4.3 и §7. Если книга не загрузилась, контекст `lawless` отказывает в `Start`, и отказ называет переменную, путь, рабочую папку и причину. Образ несёт `rules/` (`WORKDIR /home/nonroot`). Норма rest: путь только `hp` на шаге 6, затем встреча, вид `hp` и значение; `hp < hp_max` → `level_violation`, `hp > hp_max` или без `hp_max` → inv-02. Строка `system` в `ownership.go` пуста. Тесты процесса и e2e получают абсолютный путь к книге дерева (`onLoopback`, `emptyWorldEnv`).
- **Решения.** Книга читается в фабрике контекста: `internal/state/context.go` рядом с T-057 не трогал. Строка T-056 «rest, стирающий `encounter_id`» ожидает `level_violation` (путь раньше встречи); чтение встречи до операций держит прямой тест `restRefusal`. «rest below zero» ожидает `level_violation`.
- **Мутанты** (`scratchpad/t471-mutants`, без `-overlay`, базовый прогон и контрольный первыми, удалена по точному пути): C0, M1–M14 — все красные. M8 сначала выжил, после строки «rest до 0 без `hp_max`» тоже красный.
- **Прогоны** (go1.26, windows/amd64): `gofmt -l` пусто; `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — 0; три теста стенда ×20 — ok, три прогона подряд — ok; e2e — ok; `golangci-lint run ./...` — 0 issues; `mvctl env check`, `contracts check` — 0; `make test` — exit 0 (`internal/state` 95,1 %). Кончик EPIC-004 с патчем T-471 в scratch-клоне: `internal/gateway/actions`, `shared/testkit/gateway`, `cmd/multiverse` — ok, клон удалён по точному пути. Docker-сборка не запускалась, `.env` и стенд `:8888` не трогались.
- **Не снималось.** Пометки «Код расходится до T-471» в `contracts.md` и КД §4.1, §4.6 снимает system-architect после слияния.

### developer#3 · T-471 · итерация 2 по ревью #1 (0 Major, 2 Minor) · 2026-09-14

Подробности — карточка `tasks/T-471.md`, «Итерация 2 (developer)».
- **Mi-1.** Сервису `core` в `docker-compose.yml` передаётся `MV_RULES_PATH: ${MV_RULES_PATH:-rules/dark-forest.yaml}`. Умолчание совпадает с манифестом, правило 8 `compose-lint` зелёное.
- **Mi-2.** Тексты «в образе нет `rules/`» исправлены в трёх местах: `.env.example`, `fake_contexts.go` (комментарий и текст отказа — теперь «not the path MV_RULES_PATH names»), комментарий `fake_contexts_test.go`. Ожидание теста дополнено именем `MV_RULES_PATH`. Хук T-255 правился по разрешению оркестратора, нужна отметка tech-lead EPIC-003.
- **Передать.** Рецепт для второй из T-469/T-471 в `develop`: строка `READERS` `"MV_RULES_PATH": ("state", "swarm")`, передача в `core` (уже есть), новая причина у `MV_SWARM_FAKE` в `NOT_IN_CONTAINERS`.
- **Прогоны** (go1.26, windows/amd64): `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./cmd/multiverse/... ./internal/state/... ./shared/env/...` — ok; e2e — ok; `golangci-lint run ./...` — 0 issues; `mvctl env check` — 0; `make compose-lint` — exit 0 (8 правил, фикстуры 64/12; только `docker compose config`, без контейнеров; `.env` в папке нет). Docker-образ, `.env` и `:8888` не трогались.

### tech-lead#2 · T-471 · приёмка и отметка владельца EPIC-003 · 2026-09-14

Подробности — карточка `tasks/T-471.md`, разделы «Отметка владельца EPIC-003 (tech-lead#2)» и «Приёмка».
- **Решение.** Принято. DoD закрыт построчно. Смена ожиданий двух строк T-056 («rest, стирающий `encounter_id`», «rest ниже нуля» → `level_violation`) следует из C-02 v1.8 п. 3. Итерация 2 (Mi-1, Mi-2) принята по коду без ревью #2. Статус `done` — после отметки tech-lead#1, просмотра system-architect и «Учёт времени».
- **Отметка владельца EPIC-003** по `cmd/multiverse/fake_contexts{,_test}.go`: есть. Текст отказа и комментарии верны по коду заглушки (`DefaultRulesPath`, пустой `ContextConfig`), ожидание теста только усилено. Кончик EPIC-003 эти файлы не меняет.
- **Прогоны** (go1.26.8, windows/amd64): `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — 0 (48 пакетов, `updates` без обхода); e2e — ok; `golangci-lint run ./...` — 0 issues; `mvctl env check`, `contracts check` — 0; `make test` — exit 0 (`internal/state` 95,1 %); стенд I1-α три раза подряд — PASS; мутант M1 (`-overlay`) — красный. Пробное `git merge-file` с кончиками EPIC-002 `0e1aa7e` и EPIC-003 `f4c2fcd` — чисто. Scratch `t471tl-*` удалён по точным путям. Docker, `.env` и стенд `:8888` не трогались.
- **До слияния в эпик.** Отметка tech-lead#1 (`cmd/multiverse/**`, `build/Dockerfile`, `shared/env/vars.go`, `.env.example`, `docker-compose.yml`, `test/e2e/empty_world_test.go`, выбор `MV_RULES_PATH`; по индексу — ещё `shared/contracts/ownership{,_test}.go` и хук `fake_contexts{,_test}.go`). Просмотр system-architect по `shared/contracts/ownership.go`. «Учёт времени». `make ci BASE=develop` целиком.
- **Передать.**
  - Для второй из T-469/T-471 в `develop`: строка `READERS` `"MV_RULES_PATH": ("state", "swarm")` и новая причина `MV_SWARM_FAKE`.
  - system-architect: снять пометки «Код расходится до T-471», поправить эталон `.env.example` в `infrastructure.md` §4.2.
  - Бэклог EPIC-003: рой и фейк читают `MV_RULES_PATH`.
  - T-472: ожидание inv-02 → `invalid_op` в тесте процесса.
- **Наблюдение.** Один прогон `TestTheProcessRunsTheFightsOfIAlpha` занял 22,3 с при обычных ~1 с; в повторах не воспроизвелось.

### tech-lead#1 · T-471 · отметка владельца EPIC-001 · 2026-09-14

Подробности — карточка `tasks/T-471.md`, раздел «Отметка владельца EPIC-001 (tech-lead#1)».
- **Решение.** Отметка поставлена, блокирующих замечаний нет. Файлы: `cmd/multiverse/contexts_state{,_test}.go`, `serve_test.go`, `fake_contexts{,_test}.go`, `build/Dockerfile`, `shared/env/vars.go`, `.env.example`, `docker-compose.yml`, `test/e2e/empty_world_test.go`; `shared/contracts/ownership{,_test}.go` — только стиль и тесты.
- **`MV_RULES_PATH`** с умолчанием `rules/dark-forest.yaml` подтверждено: `_PATH` для одного файла, читателей по §4.3 два (`state`, `swarm`). `withTheRuleBook` остаётся в `contexts_state_test.go`.
- **Compose и `make up`** не ломаются. `state` запускает только `core`, переменная ему передаётся, образ несёт `rules/`, а бинарь и книга приходят одним образом.
- **Dockerfile.** Конфиг закреплённого digest distroless `:nonroot`, прочитанный из реестра без Docker: `WorkingDir: /home/nonroot`, `User: 65532`. `WORKDIR` ничего не сдвигает; `ENTRYPOINT`, `/data` и пробы абсолютны.
- **Слияние с T-469.** Рецепт полный, проверен в scratch-копии `311689c` + патч T-471. Без рецепта — одно нарушение правила 9; с рецептом — `9 rules` ok, фикстуры 72/14. Без строки в `good-reach-host-only.yml` фикстура красная; строка в `bad-reach-all.yml` нужна, чтобы заголовок фикстуры оставался верным. Поправка к рецепту T-469: хук `MV_SWARM_FAKE` не читает `MV_RULES_PATH`. `swarm` стоит в строке `READERS` по `design.md` §4.3 и с T-256 не уходит, комментарий над строкой — по карточке T-471.
- **Н-1.** `-count=5`: 1,17 · 1,28 · 1,01 · 0,79 · 1,09 с, PASS ×5, 22,3 с не повторилось.
- **Прогоны** (go1.26.8, windows/amd64): build+vet — 0; `go test -short` по `cmd/multiverse`, `shared/env`, `shared/contracts` — ok; e2e — ok; `golangci-lint` — 0 issues; `mvctl env check` — 0; `make compose-lint` — exit 0 (8 правил, 64/12). Scratch `t471tl1-*` удалён по точным путям. Docker, `.env` и `:8888` не трогались.

### system-architect#1 · T-471 · просмотр: строка `system`, порядок отдыха, пометки «Код расходится до T-471», вопросы T-309 · 2026-09-14

Подробности — карточка `tasks/T-471.md`, «Просмотр system-architect».
- **Вердикт.** Одобрено. Строка `system` пуста, константа осталась. Тесты (1)–(5) соответствуют C-02 v1.8 п. 6 и ответу 7 просмотра T-056.
- **Порядок отдыха.** `append hp` во встрече получает `invalid_op` раньше встречи, и это форма пакета (шаг 7, `ApplyOps`). В C-02 v1.8a п. 3 порядок записан так: шаг 6 → шаг 7 → встреча → вид `hp` → значение. Там же следствие T-472: значение чужого вида в корне `hp` отвергает шаг 7.
- **Пометки.** Сверены с кодом и сняты: в `contracts.md` (C-02: «Связка», «Причина отказа по связке», «Издатель bootstrap», п. 3, п. 6) и в КД State (§4.1, §4.6 — строка `system` и «Отдых»). `contracts.md` → v0.16. В КД — отдельная строка «Правка T-471» без номера версии (v0.4 занят T-057). `git merge-file` с КД `.worktrees/T-057` в обоих порядках слияния — 0 конфликтов.
- **`infrastructure.md` §4.2.** Добавлены `MV_STATE_WORLDS` и `MV_RULES_PATH`, обновлён комментарий над `MV_SWARM_FAKE`. `gitleaks dir` по копии рабочей копии — чисто до и после правки, `.gitleaksignore` не менялся.
- **T-309.** C-14 v1.4: сигнала State ждёт только рой. `snapshot.created` в replay публикует только State, тип в сравнение NFR-061 не входит. Открытый вопрос EPIC-002: предложения, опубликованные, пока State не поднят, при восстановлении остаются без ответа (варианты A/B в карточке). Правка КД State §7.3 — отдельно, текст в карточке.
- Код не менялся. Scratch `t471sa-*` удалён по точным путям. Docker, `.env` и `:8888` не трогались.

## developer#2 · T-057 · `state`: хранилище за рабочим набором, интенты, снапшоты, ротация, `latest.json` · 2026-09-14

Ветка `task/T-057-state-objstore-snapshots` (от `3452816`), TEAM-1, Opus. Подробности, таблица DoD → тесты и мутанты — карточка `tasks/T-057.md`, «Выполнение (developer)». Коммитов нет.
- **Что сделано.** `internal/state`:
  - `store.go` — `Store` §4.3 над `shared/objstore`, `EnsureWorldBuckets`;
  - `intent.go` — запись до фактов, интент атомарного пакета из нескольких сущностей, повтор ×3, `persist_failed`;
  - `snapshot.go` — снапшот с окном ≤ 1000 и курсором, указатель, ротация K=5, `snapshot.created` с миром;
  - `dedup.go` — досылка факта при пустом `fact_event_id` под первым id;
  - `apply.go`, `context.go`, `worker.go` — порядок «запись → публикация → память», курсор, счёт фактов, снапшот при `Stop`, `Context.Snapshot`, `/health` (`persist_failed`, `snapshot`).
  
  `shared/contracts/registry_state.go`: `WorldRequired` у `snapshot.created` (C-14 v1.3). Общий код — нужны просмотр system-architect и отметка tech-lead#1, уведомление tech-lead EPIC-003/004.
- **Решения по ходу.**
  - Решение Ma-1 T-055 (повтор того же события до `Stop`) сохранено и для фактов после записи. Объекты при этом могут опережать память, досылка идёт по `last_change`.
  - Снапшот пишется на worker'е синхронно, ради порядка `snapshot.created` в replay (отступление от ADR-011 п. 6).
  - Блок объекта снапшота — как фикстура seq 0 (с `rules_version`, `entities_count`, `reason`).
  - Интент — только `atomic=true` (ADR-013 п. 4).
  - Переменная — существующая `MV_SNAPSHOT_EVERY_FACTS`.
  - HTTP-маршрут снапшота оставлен T-059.
- **Не сделано по DoD.** Правка КД §9 «ошибка `Publish` до PUT» через system-architect до кода и выбор имени поля `/health` — вопросы в карточке. Оставлено `publish_attempts_failed`.
- **Мутанты** (`scratchpad/t057-mut`, без `-overlay`, контрольный первым, копия удалена по точному пути): M1–M8, S1–S14, R1–R5 — все красные.
  - M6 в первой редакции теста краснел только по таймауту — тест дополнен.
  - S6 в первой редакции не собрался — переписан.
  - M8 краснеет после того, как тестовое хранилище, как клиент сервера, стало отказывать отменённому контексту.
- **Прогоны** (go1.26, windows/amd64):
  - `gofmt -l` — пусто; `go build ./... && go vet ./...` — 0;
  - `go test -short -count=1 ./...` — ok; e2e — ok;
  - `golangci-lint run ./...` — 0 issues;
  - `mvctl contracts check`, `env check` — 0;
  - `make test` — exit 0: `internal/state` 91,1 %, флейк не проявился;
  - интеграция — один прогон `TestTheStoreOfStateOnMinIO` на `MINIO_IMAGE` (образ был локально) — PASS; контейнеров testcontainers после прогона нет.
  
  `make test-integration`, Docker-стек, `.env` и стенд `:8888` не трогались.

## system-architect#1 · T-057 · решения по вопросам исполнителя и просмотр общего кода · 2026-09-14

Подробности — карточка `tasks/T-057.md`, «Решения system-architect».
- **Вопросы.**
  1. КД §9 приведён к коду: ошибка `Publish` ответа — повтор того же события до `Stop`, затем `publish_failed`; после PUT — досылка повтором предложения.
  2. Поле `/health` — `publish_attempts_failed`.
  3. §4.4 приведён к фикстуре: только `size_bytes` лишь в указателе.
  4. Синхронный снапшот принят, ADR-011 п. 6 заменён дополнением 2026-09-14.
  5. Мир без `latest.json` — T-059, правило «отказ до записи» в КД §18.
- **Отступления.** `MV_SNAPSHOT_EVERY_FACTS`, маршрут в T-059, интент только при `atomic=true` — подтверждены. `snapshot_stale` текстом ошибки — временно, возраст доводит T-059.
- **Общий код.** `registry_state.go` (`WorldRequired` у `snapshot.created`) одобрен. Шлюз T-309 ставит мир в конверте, у EPIC-003 издателя пока нет — условие передано через оркестратора.
- **Итерация 2.** `WithScope(nil)` у выведенного `snapshot.created`; `size_bytes` и `duration_ms` в логе снапшота; две ссылки в комментариях.
- **Правило для T-059.** Одна публикация факта через рестарт: recovery принимает висящий объект без публикации. Догон атомарного пакета — по объектам, а не по интенту и не через `state_divergence`.
- **Документы.** КД State v0.4 (§4.4, §4.8–§4.10, §9, §18), ADR-011 — дополнение 2026-09-14. `contracts.md` не менялся. Слияние КД с кончиком эпика проверено `git merge-file` — без конфликтов. Кода, коммитов и `git add` нет.

### developer#2 · T-057 · итерация 2: пункты system-architect и ревью #1 (0/0/4/4) · 2026-09-14

Подробности — карточка `tasks/T-057.md`, «Итерация 2 (developer)». Коммитов нет.
- **Что сделано.**
  - `snapshot.created` по счёту: без `scope` (`WithScope(nil)`), с тем же конвертом, что у `admin`/`shutdown` (`actor_kind system`) — SA 1, N-1.
  - Лог снапшота несёт `size_bytes` и `duration_ms` — SA 2; две ссылки в комментариях — SA 3.
  - Снапшот ограничен `SnapshotTimeout` = 10 с на таймерах `Config.Timers` и больше не снимает отмену: зависшее хранилище не держит `Stop` — Mi-1.
  - Досылка берёт `cause` из записи коммита — Mi-3; конверт не хранится, вопрос к T-059.
  - Досылка при живом интенте этого `proposal_id` ничего не публикует и останавливает мир `persist_failed` — вопрос 2, решение оркестратора.
  - Тест ротации на восемь снапшотов — Mi-4; лишняя строка теста удалена — N-2.
  - Mi-2 — код не менялся, `batch_size` записанной сущности = число применённых наборов, §18 молчит — вопрос.
- **Мутанты** (`scratchpad/t057i2-mut`, без `-overlay`, контрольный первым, копия удалена по точному пути): I1–I10 — все красные.
  - I5 (без срока) краснеет по таймауту `go test`.
  - I3 и I9 в первой редакции не собрались — переписаны.
- **Прогоны** (go1.26, windows/amd64):
  - `gofmt -l` пусто; build/vet (в том числе `-tags integration`) — 0;
  - `go test -short -count=1 ./...` — ok, кроме `cmd/telegram-bot/internal/updates`: Windows «Access is denied» на `test.test.exe`, обход `go test -c -o` — PASS;
  - e2e — ok; `golangci-lint` — 0 issues; `mvctl contracts check` — 0;
  - `make test` — exit 0, `internal/state` 91,4 %.
  
  Интеграция MinIO не повторялась: код записи не менялся. Docker-стек, `.env`, `:8888` не трогались.

## tech-lead#2 · T-057 · приёмка · 2026-09-14

Подробности — карточка `tasks/T-057.md`, разделы «Ревью (code-reviewer)» (ссылка на ревью #1) и «Приёмка». Коммитов, `git add` и слияний нет.
- **Решение: принято.** Условие до слияния — отметка tech-lead#1 по `shared/contracts/registry_state.go` (§16 п. 8), её запрашивает оркестратор. Итерация 2 принята без ревью #2, закрытие каждого пункта проверено по коду.
- **Mi-1.** `SnapshotTimeout` = 10 с. Таймер — `a.timers.After`, это `clock.Timers` из `state.Config.Timers`: по умолчанию `RealTimers`, а не `Deps.Timers` (`NullTimers` в replay), поэтому срок держится и в replay. Вызовов `time.*` из запрета forbidigo в `internal/state` нет, lint — 0.
- **Вопрос 2 ревьюера.** `persist_failed` при живом интенте того же `proposal_id` принят. Перечень причин `fail` — §18 п. 2; по сути это незавершённая запись, класс строки §9 «Ошибка PUT», реакция — рестарт и roll-forward. `state_divergence` и `publish_failed` по смыслу не подходят.
  Оговорки ушли в DoD T-059: интент после неудачного `DeleteIntent` без roll-forward остановит мир тем же путём; `ListIntents` на пути досылки — без повторов.
- **Индекс v0.1.6.** T-057 — статус, «Файлы», интент «`atomic=true` и > 1», `MV_SNAPSHOT_EVERY_FACTS`, строка приёмки. T-058 — три строки: курсор seq 0, `--force`, `--bus kafka`. T-059 — восемь строк:
  - правила §18;
  - `/health`: `snapshot_stale: age_s`, `pending_intents`;
  - N-3, N-4;
  - тесты досылки на восстановлении и roll-forward до приёма предложений;
  - конверт досылки;
  - `batch_size`;
  - подключение хранилища и восстановления в `cmd/multiverse` одним изменением.
- **Прогоны** (go1.26.8, windows/amd64):
  - `gofmt -l` пусто; build/vet (в том числе `-tags integration ./internal/state/...`) — 0;
  - `go test -short -count=1 ./...` — ok, кроме `updates` («Access is denied»); обход `go test -c -o scratchpad/t057tl-upd.exe` — PASS, exe удалён по точному пути;
  - e2e — ok; `golangci-lint` — 0 issues; `contracts check` — 65 типов, exit 0; `env check` — exit 0;
  - `make test` — exit 0, `internal/state` 91,4 %.

  Интеграция не запускалась.
- **Пробное слияние с T-471** (только чтение `.worktrees/T-471`):
  - код — общих файлов нет. Слитое дерево в копии `scratchpad/t057tl-merged`: build, vet, тесты затронутых пакетов, e2e, `contracts`/`env check`, lint — зелёные. Один сбой по времени `shared/testkit/gateway` (окно 200 мс под нагрузкой) при повторе ×3 не проявился;
  - КД `state-and-mechanics.md` (`git merge-file` против текущей копии T-471 со снятыми пометками) — 0 конфликтов;
  - `tasks.md` — 2 конфликта (строка версии и строка changelog v0.1.6), рецепт — в карточке;
  - `dev-log.md` и `review.md` — дописи в конец, драйвер `appendtail`.

  Копии удалены по точному пути. Docker, `.env`, `:8888` не трогались.

## tech-lead#1 · T-057 · отметка владельца `shared/contracts` по `registry_state.go` · 2026-09-14

Подробности — карточка `tasks/T-057.md`, раздел «Отметка владельца EPIC-001 (tech-lead#1)». Коммитов, `git add` и правок кода нет.
- **Решение: отметка поставлена** (§16 п. 8). `WorldRequired` у `snapshot.created` — ровно C-14 v1.3 (помощник `worldRequired`, издатели и потребители без изменений), с v1.4 не расходится: State публикует с миром и в live, и в replay.
- **Издатели.** На кончиках `develop`, `epic/EPIC-001…004` издателя `snapshot.created` в коде нет; `registry_state.go` везде совпадает с базой `3452816`. Из рабочих папок издатель есть только у T-309 (шлюз, `NewRoot` с миром, без `WorldID` писатель не строится). В `internal/{swarm,laws,llm,memory}` издателей нет.
- **Фикстуры и тесты.** `testdata/fixtures/events/snapshot.created.v1.*` — только payload, мир ставит `contracts check`; `validate_test.go`, `test/fixtures`, `shared/testkit/state` строят событие с миром. Правок не нужно.
- **Прогоны:** `go test -short -count=1 ./shared/contracts/... ./internal/state/... ./cmd/mvctl/...` — ok; `./test/fixtures/... ./shared/testkit/state/... ./shared/eventbus/...` — ok; `contracts check` — 65 типов, exit 0.
- **Nit:** метки `contract-change` в разделе T-057 индекса нет — проставить tech-lead#2.
- **`design.md` §4.1 и §7** не правил (зона architect и tech-lead#2): точный текст правки v0.1.2 — в карточке, п. 5. Устаревшие имена стоят только в строках 64 и 123.

## developer#3 · T-472 · грамматика пути и виды атрибутов в `shared/entity` · 2026-09-14 03:08–03:35

Подробности — карточка `tasks/T-472.md`, раздел «Выполнение (developer)». Метка `contract-change`; коммитов и `git add` нет.
- **Оценка влияния — до кода.** Кончики EPIC-003 и EPIC-004 и рабочая копия T-309 — только чтение.
  - Из тестов шлюза задет только шаг `set position.region` в `readmodel/apply_test.go`.
  - Строка «under a number» (`hp.deep`) не задета: `ApplyOps` там не вызывается.
  - Издатели пишут корневые константы.
- **Решение о типе** (исполнитель, на просмотр system-architect): `ApplyOps` берёт тип из `Entity.Type`, шаг 1 State — из `changes[].entity.type`. Таблица — по типам; атрибут вне таблицы своего типа не типизирован.
- **Сделано:**
  - `entity.CanonicalPath`, `entity.CheckOp`, таблица видов `AttributeSpecOf`/`Holds`, `entity.CheckAttributes`;
  - `ApplyOps` отвергает неканонический путь, путь ниже скаляра и значение чужого вида в корне затронутого скаляра;
  - State: шаг 1 зовёт `CheckOp`, создание — `CheckAttributes`; `canonicalPath` и `scalarAttributes` удалены; Н-2 T-471.
- **Отклонения:**
  - виды `KindModifier` («int / формула», §3.3) и `KindOpen` (контейнер или `scope`, только обязательность);
  - `CheckOp` не проверяет значение: причины `inc` сохранены;
  - одна строка матрицы сменила ожидание по C-02 v1.8a п. 3; порядок держит новая строка `remove hp`, M5 на ней красный.
- **Мутанты** (копия `scratchpad/t472-mut`, без `-overlay`, контрольный первым, удалена по точному пути): C0, U1a, U1b, M5g, U2a, U2b, M5, K1–K8 — красные. K4 (шаг 1 без типа) сначала выжил; добавлена строка «gateway: weather.x of the world, the form before the rights».
- **Прогоны** (go1.26, windows/amd64):
  - `gofmt -l` пусто; build/vet — 0;
  - `go test -short -count=1 ./...` — ok, кроме `cmd/mvctl/internal/env` («Access is denied»); обход `go test -c -o scratchpad/t472_env.exe` — PASS, exe удалён по точному пути;
  - стенд `-count=20` — ok; e2e — ok; lint — 0 issues; `contracts check` — exit 0;
  - `make test` — exit 0, `internal/state` 91,4 %.

  Интеграция не запускалась; Docker, `.env`, `:8888` не трогались.
- **Риски слияния:**
  - `internal/gateway/readmodel/apply_test.go` — зона EPIC-004;
  - `cmd/multiverse/contexts_state_test.go` — через tech-lead#1; рядом T-059 правит `contexts_state.go`;
  - `internal/state/{apply,proposal}.go` и `helpers_test.go` — общие файлы с T-058 и T-059.

## system-architect#2 · T-472 · просмотр `contract-change` C-02 · 2026-09-14

Подробности — карточка `tasks/T-472.md`, раздел «Просмотр system-architect». Код не менял и не запускал. Коммитов и `git add` нет, `review.md` не трогал.
- **Вердикт:** одобрено при условии итерации 2 (И2-1).
- **Решения:**
  - таблица по типам принята: тип — `Entity.Type`, на шаге 1 — `changes[].entity.type`. Неизвестный атрибут типа форма не отвергает, его ограничивают таблица владения и inv-09;
  - `KindModifier` и `KindOpen` подтверждены;
  - ключ из цифр любой длины запрещён: граница `strconv.Atoi` зависит от платформы (И2-1);
  - перечисления, ссылки и диапазоны — нормы State и законы, а не форма; записано в C-02;
  - в C-02 добавлена строка об атрибутах создания;
  - функция догона в `shared/entity` — отдельная задача EPIC-002 после T-059. В самой T-059 — проверка `CanonicalPath` в `pathTokens`.
- **Документы:** `contracts.md` v0.17 (C-02 v1.8b); КД State v0.4.1 (§3.2, §4.5 п. 1, абзац создания).
- **Итерация 2:**
  - И2-1 — `CanonicalPath` и тест грамматики;
  - И2-2 (minor) — строка матрицы: `died_at.x` у `player` → `level_violation`.
- **Проверка слияния** КД и `contracts.md` с `.worktrees/T-058` и `.worktrees/T-059` (база `0345177`, CRLF): 0 конфликтов. Строка changelog КД перенесена под «v0.4», чтобы не встать на место вставки T-058.

## developer#3 · T-472 · итерация 2 (ревью #1, просмотр system-architect#2) · 2026-09-14

Подробности — карточка `tasks/T-472.md`, раздел «Итерация 2 (developer)». Коммитов и `git add` нет; разделы system-architect, `contracts.md` и КД не трогались.
- **Условия system-architect:**
  - И2-1 — `CanonicalPath` отвергает ключ шаблоном `^[+-]?[0-9]+$` любой длины, без `strconv.Atoi`; пять новых написаний в тесте грамматики;
  - И2-2 — строка матрицы `task` / `died_at.x` у живого `player` → `level_violation`.
- **Ревью #1:**
  - Mi-1 — полнота таблицы сверяется множествами ключей через `export_test.go`;
  - Mi-2 — свойство-тест со списками из двух элементов и независимым оракулом отвергаемых путей;
  - Mi-3 — `TestAPathBelowAScalarRefusesALoosePackageWhole` (`atomic=false`);
  - N-1…N-3 исправлены.
- **`flee`** (решение оркестратора): при создании необязателен (§3.3); вопрос на подтверждение system-architect.
- **Мутанты** (копия `scratchpad/t472i2-mut`, без `-overlay`, контрольный первым, удалена по точному пути): C0 (роняет и свойство-тест), A1, D1, K4, F1, T1 — красные.
- **Прогоны** (go1.26, windows/amd64):
  - `gofmt -l` пусто; build/vet — 0;
  - тесты `shared/entity`, `internal/state`, `internal/gateway`, `shared/testkit`, `cmd/multiverse` — 26 пакетов ok;
  - lint — 0 issues;
  - `make test` — exit 0, `internal/state` 91,4 %.
- **Передать T-059:** в `CatchUpChanged` нет проверки `CanonicalPath`; добавляет задача, которая сливается второй.

## tech-lead#2 · T-472 · приёмка · 2026-09-14 09:23

Подробности — карточка `tasks/T-472.md`, раздел «Приёмка (tech-lead)». Коммитов, `git add` и слияний нет; `contracts.md`, КД и разделы исполнителя и system-architect не трогал.
- **Решение:** принято; итерация 2 принята без ревью #2 (в ревью #1 только Minor и Nit).
- **До слияния:** отметка tech-lead#1 (`cmd/multiverse/contexts_state_test.go`), отметка tech-lead#3 (`internal/gateway/readmodel/apply_test.go`), «Учёт времени». Подтверждение system-architect по `flee` слияние не держит; до правки `data-model.md` §3.3, C-02 v1.8b п. 2 и КД §4.5 код мягче текста.
- **Итерация 2 по коду:** И2-1, И2-2, Mi-1…Mi-3, N-1…N-3 и `flee` закрыты. Мутанты при приёмке (копия `scratchpad/t472tl-mut`, контрольный первым, копия удалена по точному пути):
  - «любой непустой путь канонический» — красный, в том числе свойство-тест;
  - «ключ по `strconv.Atoi`» — красный;
  - «`died_at` в таблице `player`» — красный;
  - «`flee` обязателен» — красный.
- **Прогоны** (go1.26.8, windows/amd64):
  - `gofmt` пусто; build/vet — 0;
  - `go test -short ./...` — 54 пакета ok, `env` и `updates` — «Access is denied», оба прошли в `make test`;
  - e2e — ok; стенд I1-α `-count=5` — ok; lint — 0 issues; `contracts check` — exit 0;
  - `make test` — exit 0, `internal/state` 91,4 %, `shared/entity` 93,5 %.
- **Риски слияния:**
  - T-059: `apply.go` сливается без конфликтов; пробное дерево зелёное. `CatchUpChanged` не проверяет `CanonicalPath` — рекомендовано сливать T-472 первой, строка DoD T-059 внесена;
  - T-058: общих Go-файлов нет, пробное дерево зелёное. КД сливается в обе стороны без конфликтов (база `0345177` в CRLF). `CauseInit` объявлена и в T-058, и в T-059 — назвать при слиянии;
  - `dev-log.md`/`review.md` — хвост снимает драйвер `appendtail`.
- **Индекс v0.1.8:** статус и приёмка T-472, строка DoD T-059, четыре строки бэклога без номеров (функция догона в `shared/entity`, `opFor` → EPIC-004, виды контейнеров, путь `name`).

## tech-lead#3 · T-472 · отметки владельцев EPIC-004 и EPIC-001 (EPIC-001 — по назначению оркестратора) · 2026-09-14 09:45

Подробности — карточка `tasks/T-472.md`, разделы «Отметка владельца EPIC-004 (tech-lead#3)» и «Отметка владельца EPIC-001 (по назначению оркестратора, tech-lead#3)». Код, индексы, `contracts.md` и разделы других ролей не трогал. Коммитов, `git add` и слияний нет.
- **EPIC-004, `internal/gateway/readmodel/apply_test.go` — одобрено.**
  - Шаг `set banner "green"` + `set banner.region` сохраняет смысл «текст заменяется объектом».
  - `ErrCorruptFact` на неканонический путь или значение чужого вида State не вызывает: шаг 1 и `ApplyOps` отвергают такое предложение, пути `changed[]` — канонические.
  - Пробное дерево T-472 + `internal/gateway`, `shared/testkit/gateway` с кончика `6b67fec` собирается. С тестом кончика падает ровно `TestChangedOfBothFormsReproducesTheStateOfState` (`position.region`). С тестом T-472 — 18 пакетов шлюза и `cmd/multiverse` ok.
  - Риск: журнал dev-стенда с фактами, которые новая форма отвергает, остановит догон шлюза. `RepairFromStateSnapshot` такой факт не спасает.
  - Строка бэклога для §8 индекса EPIC-004 (З-25, `opFor` → `CanonicalPath` у каждого элемента) — в карточке.
- **EPIC-001, `cmd/multiverse/contexts_state_test.go` — одобрено** (за tech-lead#1, по назначению оркестратора).
  - Создание несёт полный набор атрибутов `player`, строка «object at the root of hp» ждёт `invalid_op`.
  - Проверку законов тест не теряет: inv-10, inv-02 и законный ход остаются. Мутанты L1 (законы сняты), L2 (снят inv-02), F1 (снята проверка вида) — красные.
- **Прогоны** (go1.26.8, windows/amd64, `.worktrees/T-472`): `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./internal/gateway/readmodel/... ./cmd/multiverse/...` — ok. Пробная копия `scratchpad/t472tl3-probe` удалена по точному пути. Docker, `.env` и `:8888` не трогал.

## developer#2 · T-059 · recovery, `/health`, admin-маршрут · 2026-09-14

Подробности — карточка `tasks/T-059.md`, раздел «Выполнение (developer)». Коммитов и `git add` нет.
- **Способ чтения — курсор** (ADR-011 п. 4, КД §4.8): `Journal.Tail` с `End` после восстановления каждого мира; группа `core.state` снята. **T-474 получает полный объём (M).** Без хранилища State читает с офсета 0 при первом старте и с события после последнего обработанного — при следующем.
- **`recovery.go`**: указатель → снапшот с проверкой (откат до K=5, все битые — `snapshot_corrupted`, мир не обслуживается) → окно `Restore` → догон фактов `[cursor, End)` по правилу C-02 v1.6 (`catchup.go`) с дополнением окна → сверка с объектами (висящая запись принимается без публикации; та же версия — id факта из журнала сохраняется, §18) → roll-forward интентов до приёма предложений → `analytics.replay.completed mode=recovery`. Разрыв журнала (первый офсет ≠ курсор или курсор > `End`) — мир из объектов, `degraded {log_gap}`.
- **`/health` мира** (`health.go`): `snapshot {seq, taken_at, age_s}`, `snapshot_stale: age_s`, `pending_intents`, `publish_attempts_failed`, `snapshot_event_failed` (N-4), `world: uninitialized`, `reason` с `snapshot_corrupted`/`state_divergence`.
- **`admin.go`**: `POST /v1/admin/state/{world}/snapshot` за `runtime.AdminOnly`. **`world.go`**: мир `uninitialized` (§18). **N-3**: `ErrNoBucket`/`ErrNotFound` не повторяются, `ListIntents` на досылке — с повторами, число повторов — в комментарии `persistPauses`. Интент записанного пакета не останавливает мир на повторе (`finishedIn`).
- **Страж** `OneStateOverTheWorld` перенесён в `shared/testkit/state` — место на подтверждение tech-lead#1.
- **`cmd/multiverse`** (через tech-lead#1): `contexts_state.go` — `Objects` (при заданных ключах MinIO) и `RulesVersion` одним изменением с восстановлением; `fake_contexts_test.go` — вызов общего стража; новый `contexts_state_recovery_test.go` — рестарт процесса над хранилищем, тот же `state_hash`.
- **Отклонения**: битый снапшот не роняет `Start` — мир остановлен; admin-маршрут пускает без `X-Actor-Kind` (контракт `AdminOnly`); `analytics.consistency.violated` не публикуется — в реестре только издатель `mvctl`.
- **Открыто для system-architect**: `log_gap` и повторная публикация факта, конверт досылки (Mi-3), `batch_size` (Mi-2), N-4, число повторов в КД §4.5 п. 11, строки КД §4.8/§9.
- **Риск**: в compose `core` не проходит healthcheck до `mvctl world init` (мир `uninitialized` → `degraded`).
- **Прогоны**: gofmt пусто; build/vet (+`-tags integration`) — 0; `go test -short ./...` — ok (кроме `cmd/mvctl/internal/env`: «Access is denied», обход `go test -c` — PASS); e2e — ok; lint — 0; `contracts check`/`env check` — exit 0; `make test` — exit 0, `internal/state` 89,1 %; стенд I1-α ×3 — 9/9. Мутанты M1–M18, K5–K7 — красные (M1 и M13 после уточнения тестов), C0 зелёный; копия удалена по точному пути. Интеграция MinIO не запускалась (не требуется DoD).

## system-architect#1 · T-059 · решения по вопросам исполнителя и риску compose · 2026-09-14

Подробности — карточка `tasks/T-059.md`, раздел «Решения system-architect»; КД `state-and-mechanics.md` — строка «Правка T-059» и §19. Коммитов, `git add` и правок кода нет.
- **Вопросы:**
  - `log_gap` — подтверждено с уточнением: факты, доставленные чтением с разрывом, отмечают `fact_event_id` у объектов с тем же `id`/`version`/`proposal_id`, остальным повтор досылает факт под первым id;
  - конверт досылки — как в коде, без `contract-change` `shared/entity`;
  - `batch_size` — применённые наборы;
  - N-4 — `snapshot_event_failed` подтверждён;
  - до 12 запросов и ≈ 3,7 с пауз внесены в КД §4.5 п. 11;
  - страж в `shared/testkit/state` — подходит, подтверждает tech-lead#1.
- **Отступления приняты:** битый снапшот останавливает мир, а не процесс; admin-маршрут по `X-Client-Id`; `analytics.consistency.violated` — после строки издателя `core/state` (бэклог EPIC-005, `contract-change`).
- **Compose — вариант (б):** мир `uninitialized` остаётся `degraded`, проба `multiverse health` принимает `ok|degraded` (код 0), `fail` и прочее — код 1. Правка `health.go` — итерация 2 T-059 (просмотр tech-lead#1). `infrastructure.md` §2.2, §2.3, §7.2 и `Makefile` (`DEGRADED_STRICT`) — devops-engineer, точный текст в карточке. `docker-compose.yml` не меняется.
- **Итерация 2:**
  - отметка вышедших фактов при `log_gap`;
  - проба `health.go`;
  - отказ State без хранилища над шиной, которая переживает процесс;
  - тесты решений 2 и 3.
- **Проверки:** `git merge-file` КД против `.worktrees/T-058` и `.worktrees/T-472` (база `0345177` в CRLF) — 0 конфликтов, в том числе последовательно. `contracts.md` не менялся. Docker, `.env`, `:8888` не трогались.
