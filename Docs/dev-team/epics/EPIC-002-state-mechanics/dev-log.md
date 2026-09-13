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
