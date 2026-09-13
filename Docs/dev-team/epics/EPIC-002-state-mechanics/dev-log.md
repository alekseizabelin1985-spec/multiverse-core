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
