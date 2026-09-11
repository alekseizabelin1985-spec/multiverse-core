# Задачи EPIC-002 «Состояние и механика» (I1 — волна 1, I2 — волна 1.8→2)

Версия 0.1.1 · 2026-09-09 · tech-lead#1 (TEAM-1) · статус: к G3.
**Правки сведения 3** (`architecture/consolidation.md` §14.1 З-2, `contracts.md` v0.4 C-02 v1.2, ADR-017 доп. 1 п. 5; внесено tech-lead#1): T-053 (`NPCTarget` — `abandoned` = `dead`), T-054 (inv-01 для `status ∈ dead|abandoned|ascended_final`), T-056 (`abandoned` — терминальный, переход только `alive → abandoned` от gateway, `cause=forget`). Схемы `entity.*.proposed` с `cause=forget` создаёт EPIC-001 T-006 (F-4b-1). Структура подволн не менялась.
**Правки ревизии контрактов T-416** (`contracts.md` v0.7: C-01 v1.4, C-02 v1.4; ADR-025, ADR-027; внесено tech-lead#1 2026-09-11): T-055 — решение о `WithCauseID` для фактов State; T-060 — таймеры шины сужены до решения архитектора; T-061, критерии I1/I2 и §5 — `FakeEncounter` вместо `WithEncounterStub`, `mvctl report`, таблица владения одна. Добавленные пункты помечены «(T-416, 2026-09-11)», отменённые — «заменено (T-416)» и не удалены.
Команда TEAM-1 · ветка **`epic/EPIC-002-state-mechanics`** (от `integration/mvp-1` после тега `mvp-1/wave-0`; не создавать до конца волны 0) · G2 утверждён 2026-09-09.
Основание: `epics/EPIC-002-state-mechanics/design.md` v0.1 (§4.1 блоки I1-1…I1-12, §4.2 блоки I2-1…I2-6, §4.3 специфика, §9 тестируемость); `architecture/components/state-and-mechanics.md` v0.2 (§3–§7, §4.10, §11, §12, §16); `architecture/contracts.md` v0.2 (C-01 v1.1, C-02 v1.1, C-03 v1.1, C-13, C-14 v1.1); ADR-003, ADR-011, ADR-012, ADR-013, ADR-021; `plan/epics.md` v0.2 §2; `plan/teams.md` §4; `plan/ownership.md` v0.2; `epics/EPIC-001-foundation/design.md` §5 и `tasks.md` §8 (волны проекта).

Диапазон номеров: **T-050…T-129** (TEAM-1). Слоты волны 1: **2 разработчика** — две нитки: `mechanics` (developer#1) ∥ `entity`/`state` (developer#2).
Размер: **S** ≤ полдня · **M** ≤ одной сессии одного разработчика · L не допускается.

**Стартовая точка — не «с нуля»**: `shared/entity` v2 (T-011), `internal/mechanics` типы + `Load` + `rules/dark-forest.yaml` (T-015), `FakeState` v0 / `FixedMechanics` (T-017), `Harness` v0 (T-018), фикстуры (T-016) уже в `integration/mvp-1`; двойники боя и нарратива — `FakeEncounter` и `FakeNarrator` (EPIC-003 T-219, T-220; по C-05 v1.4 их дорабатывает T-419). *(T-416, 2026-09-11: прежде здесь стояло `FakeNarrator` + `WithEncounterStub` (T-018). `WithEncounterStub` снят сведением 2 и не делался — см. EPIC-001 T-018.)*

---

## 1. Общий DoD (применяется к каждой задаче)

1. `make lint` чист; `go build ./... && go vet ./...` зелёные.
2. `go test -short -race -count=1 ./...` зелёные; тесты написаны в той же задаче (уровни — `design.md` §9, `state-and-mechanics.md` §11).
3. **CI зелёный** на PR в ветку эпика и в `integration/mvp-1`: `unit`, `integration`, `e2e`, `contracts`, `security`, `compose-lint`.
4. `coverage-gate.sh 60 internal/state internal/mechanics internal/replay` — покрытие ядра **≥ 60 %** (NFR-064); падение покрытия ниже порога блокирует приёмку.
5. `dev-log.md` эпика заполнен: `developer#K`, `T-NNN`, что сделано, отклонения от дизайна, запросы к владельцам контрактов.
6. Изменений вне карты владения нет: EPIC-002 владеет `shared/entity/**`, `internal/{state,mechanics,replay}/**`, `rules/**`, `schemas/events/{entity.*,dice.rolled,snapshot.created,analytics.replay.completed}`, `cmd/mvctl/internal/world/**`, `shared/testkit/{state,mechanics}/**`. Правки `shared/{eventbus,contracts,clock,runtime,objstore}` — **запрос к system-architect / tech-lead#1**, не прямая правка.
7. Заглушки других команд (`testkit/gateway`, `testkit/swarm`) **не правятся** — запрос владельцу (EPIC-004 / EPIC-003) через tech-lead#1.
8. Поведение соответствует критериям приёмки US задачи (US-003, US-005, US-011, US-017, US-006/US-007 для I2).

**Критерии приёмки инкрементов** (ворота тимлида):
- **I1**: `mvctl world init --fixtures testdata/fixtures/` создаёт «Тёмный лес» (6 сущностей, снапшот seq 0 `reason=bootstrap`); e2e **`solo-30` на `FakeEncounter` + `FakeNarrator` (EPIC-003 T-219/T-220) с реальными `mechanics`** — 30 ходов без расхождений снапшота и журнала *(T-416: заменено `FakeNarrator(WithEncounterStub(Rules))` — такой заглушки нет)*; **recovery: рестарт → `identical=true`, `llm_calls=0`, `dice_rolled_new=0`** (`analytics.replay.completed mode=recovery`); участие в **I1-α** (живая игра через бота); unit ≥ 60 % ядра.
- **I2**: `group-3x30` в CI зелёный; S2 с харнессом из трёх клиентов (после EPIC-004 I2); `mvctl report --audit` на записи S2 даёт `state_divergence = 0` *(T-416: было `session-report --audit`; имя — C-10, T-409)*.

---

## 2. Инкремент I1 «соло + фон» — задачи T-050…T-064

Порядок (`design.md` §3 п. 3): `entity` + схемы → `mechanics` → `state` (memstore, Applier, факты) → `objStore` + снапшоты → `bootstrap` + `world init` → recovery + `replay` → e2e S1/S3 → архив as-is.

### Подволна 1.1

### T-050: I1-1 · `shared/entity` — дозаполнение и полное покрытие · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.1
- **Описание**: сверить `shared/entity` v2 (из T-011) с `state-and-mechanics.md` §3: геттеры `attrs.go` для **всех** атрибутов `data-model.md` §3 (в т. ч. `flee`, `encounter`, `participation`, `died_at`, `killed_by`, `inventory[].source`), обрезка `HistoryEntry` до 50, `Ref` ⇄ `eventbus.EntityRef` в обе стороны, полный набор тестов `ApplyOps` (§11).
- **Файлы**: `shared/entity/{entity,attrs,ops,types,hash,ref}.go` + тесты.
- **Зависимости**: T-011 (волна 0).
- **Ссылки**: `design.md` §4.1 I1-1; `state-and-mechanics.md` §3, §11; C-02 v1.1; ADR-013.
- **DoD**: тесты покрывают каждую op (`set/inc/append/remove`), каждую ошибку (`invalid_op`, тип не совпал, путь не найден) и no-op; `StateHash` стабилен при перестановке ключей и между процессами; покрытие `shared/entity` ≥ 60 %; изменения структуры (если нужны) согласованы с system-architect как `contract-change`; общий DoD §1.

### T-051: I1-3 · `mechanics` — RNG и вычислитель формул · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.1
- **Описание**: довести `rng.go` (`Seed(eventID, rollIndex)`, `NewRNG`, бросок `NdM+K` на одном RNG) и `formula.go` — от парсера (T-015) до **вычислителя** мини-грамматики `check`/`dice` (`state-and-mechanics.md` §5.3) с подстановкой статов актёров.
- **Файлы**: `internal/mechanics/{rng,formula}.go` + тесты.
- **Зависимости**: T-015 (волна 0).
- **Ссылки**: `design.md` §4.1 I1-3; `state-and-mechanics.md` §5.3; ADR-012; US-003; **NFR-060**.
- **DoD**: тест **1000 событий × 4 броска**: один `(event_id, roll_index)` → один результат в 100 % случаев; броски с разными `roll_index` статистически независимы (корреляция не отличима от 0, χ² на равномерность); фиксированные `Seed`-векторы в `testdata`; вычислитель формул — таблица позитив/негатив; общий DoD §1.

### Подволна 1.2

### T-052: I1-2 · Схемы EPIC-002 — ревизия и примеры · Размер: S · Статус: todo · Исполнитель: developer#2 · Подволна 1.2
- **Описание**: ревизия восьми схем, созданных в F-4b (T-006): `entity.create.proposed`, `entity.update.proposed`, `entity.created`, `entity.updated`, `entity.update.rejected`, `dice.rolled`, `snapshot.created`, `analytics.replay.completed` — enum ops, `expected_version` опционален, `reason` enum C-02 v1.1 (включая `duplicate_entity`), `changed[]` для `append`, `component` enum (C-14 v1.1), `mode ∈ {recovery, test}`; примеры `api-contracts.md` §2.3.4/5/12 валидны.
- **Файлы**: `schemas/events/{entity.*,dice.rolled,snapshot.created,analytics.replay.completed}.v1.json`, тесты в `shared/contracts`.
- **Зависимости**: T-050, T-006/T-009 (волна 0).
- **Ссылки**: `design.md` §4.1 I1-2; C-02 v1.1, C-14 v1.1; `api-contracts.md` §2.3; `ownership.md` §1 (совладение `analytics.replay.completed` с EPIC-005).
- **DoD**: `make contracts` зелёный; каждая схема покрывает соответствующий пример из `api-contracts.md`; поля `mode=test`, `events_hash_match` в `analytics.replay.completed` **не удалены и не переименованы** (совладение с EPIC-005 — уведомить tech-lead#1 при любой правке); FR-034 (enum причин отказа) отражён; общий DoD §1.

### T-053: I1-4 · `mechanics` — `Resolve`, `NPCTarget`, `ChangesFor`, `dice.rolled` · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.2
- **Описание**: `resolve.go` (таблица исходов §5.4: попадание/крит/фамбл/промах, урон по формулам, `flee`), `target.go` (`NPCTarget` — без `Participation`, это I2), `changes.go` (`ChangesFor`: ops HP с clamp, `status`, `died_at`/`killed_by`, `inventory append` с `source`, `encounter.npcs[].last_damager`), `dice_event.go` (`DiceRolledPayload` ↔ схема), `actor.go` (`ActorFromEntity`).
- **Файлы**: `internal/mechanics/{resolve,target,changes,dice_event,actor}.go` + тесты.
- **Зависимости**: T-051.
- **Ссылки**: `design.md` §4.1 I1-4; `state-and-mechanics.md` §5.4, §7.1; C-03 v1.1; US-003 (FR-018, FR-020…FR-022), BR-05.
- **Дополнение (сведение 3, C-02 v1.2; З-2)**: `NPCTarget` и `ActorFromEntity` считают «живым» только `status = alive` — `dead`, **`abandoned`** и `ascended_final` исключаются из кандидатов одинаково (C-03: `Actor.Status ≠ alive` не может быть целью).
- **DoD**: табличные тесты `Resolve` по всем исходам (включая границы крита/фамбла и `flee`); `NPCTarget` детерминирован при равных кандидатах и **не выбирает `abandoned`** (наравне с `dead`); `ChangesFor` — HP не уходит ниже 0 и выше `hp_max`, смерть выставляет `status/died_at/killed_by` одним пакетом; `DiceRolledPayload` валиден против схемы (`contracts.Validate`); `mechanics.ErrNotImplemented` больше не возвращается ни одним из трёх методов; общий DoD §1.

### Подволна 1.3

### T-054: I1-5 · `mechanics` — инварианты соло (inv-01, 02, 03, 09, 10) · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.3
- **Описание**: реализации `Check` для inv-01, inv-02, inv-03, inv-09, inv-10 в `invariants.go`; inv-04/05/06 — `Check` по `overlayView` в I2 (T-066); inv-07/08 — `Check=nil` (`Where: audit|guardian`). Один набор id совпадает с `laws@v1` (ADR-012 п. 5).
- **Дополнение (сведение 3, C-02 v1.2, ADR-017 доп. 1 п. 5; З-2)**: **inv-01 `dead_does_not_act` срабатывает для `status ∈ dead | abandoned | ascended_final`** — покинутый персонаж (`abandoned`) трактуется как `dead` во всех правилах, целях и раундах; отдельного инварианта для `abandoned` не заводим.
- **Файлы**: `internal/mechanics/invariants.go` + тесты.
- **Зависимости**: T-050, T-053.
- **Ссылки**: `design.md` §4.1 I1-5; `state-and-mechanics.md` §5.5; **`contracts.md` v0.4 C-02 v1.2**; ADR-012 п. 5, ADR-017 доп. 1 п. 5; **NFR-020**; BR-03.
- **DoD**: на каждый инвариант — позитивный и негативный тест с `Violation{id, reason}`; **(сведение 3)** негативный тест inv-01 прогоняется на трёх значениях `status` (`dead`, `abandoned`, `ascended_final`) с одинаковым `Violation`; `Invariants()` возвращает 10 записей, id совпадают со списком `laws@v1` (сверка с EPIC-003 — тест `mvctl laws check` в I1b, здесь фиксируется список в `testdata`); inv-04/05/06 явно помечены «I2»; общий DoD §1.

### T-055: I1-6a · `state` — конвейер предложение → факт (memstore, Applier, версии) · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.3
- **Описание**: `context.go` (`runtime.Context` "state", `DependsOn`), `worker.go` (по одному worker'у на мир из `MV_STATE_WORLDS`, единственный писатель), `memstore/`, `proposal.go`, `apply.go`, `facts.go`: приём `entity.create.proposed`/`entity.update.proposed` из `system_events`, применение `ApplyOps` на копии, `version+1`, публикация `entity.created`/`entity.updated` через `Derive` (наследование `timestamp` предложения и `correlation_id`), `atomic` all-or-nothing в памяти. Паника в worker'е не глотается — `/health fail`, мир останавливается (NFR-012).
- **Файлы**: `internal/state/{context,worker,proposal,apply,facts}.go`, `internal/state/memstore/**` + тесты.
- **Зависимости**: T-050, T-052, T-054 (интерфейс `Invariants()`), C-01 (T-005/T-014).
- **Ссылки**: `design.md` §4.1 I1-6; `state-and-mechanics.md` §4.5, §4.1; C-02 v1.1; NFR-012, NFR-013.
- **DoD**: unit — цикл «предложение → факт» для create/update/atomic; версии строго +1 (0 пропусков и 0 повторов на 1000 предложений); факт несёт `causation_id` предложения и его `timestamp`; паника в обработчике → `/health fail` и остановка мира (тест); **(T-410, 2026-09-11)** процесс ставит глобальные источники событий `eventbus.SetRegistry`/`SetClock`/`SetIDSource` из тех же объектов, что кладёт в `runtime.Deps` (сегодня не ставит, хотя `shared/eventbus/sources.go` говорит, что их ставит `cmd/multiverse`), — тест: id и время события, построенного конструктором, берутся из источников `Deps`; правка `cmd/multiverse` — через tech-lead#1, согласовать с T-060 (`--id-source=sequence`) *(подтверждено C-01 v1.4 «Источники конструкторов», T-416: T-055 — срок, первый настоящий издатель; контекст источники не ставит и не меняет)*; **(T-416, 2026-09-11; C-01 v1.4, ADR-027 п. 2)** решение EPIC-002 о `eventbus.WithCauseID` для фактов State (`entity.created`/`entity.updated`, `parts` — id сущности) принято и записано в `dev-log.md` с доводами. Архитектор рекомендует «да»: тогда «факты досылаются, если не были опубликованы» (C-02 «Гарантии») не даёт дублей у потребителей, дедуплицирующих по `id`. Учесть: повтор предложения под тем же `proposal_id` приходит новым событием с другим `causation_id` (C-05 п. 1). Если нужен один id факта на все повторы, в `parts` входит `proposal_id` — выбор за исполнителем с тимлидом. При «да» — тест: досылка факта по повтору предложения даёт факт с тем же `id`, разные сущности одного пакета — разные `id`. При «нет» — обоснование в `dev-log.md`. При «да» задача зависит от **T-417**; общий DoD §1.

### Подволна 1.4

### T-056: I1-6b · `state` — владение, инварианты, дедуп, матрица отказов; **замена `FakeState` v0** · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.4
- **Описание**: `ownership.go` (проверка по статичной таблице `contracts.OwnershipRules`, `level_violation`), `invariants.go` (подключение `mechanics.Invariants()` к конвейеру), `dedup.go` (`proposal_id`: LRU + `last_change`), `world.go` (мир без `latest.json` → `/health degraded {world: uninitialized}`, предложения отклоняются `unknown_entity`); полная матрица причин отказа C-02 v1.1 (`unknown_entity`, `version_conflict`, `invalid_op`, `duplicate_entity`, `level_violation`, `invariant`, `dead_entity`); **замена заглушки**: `shared/testkit/state.FakeState` = `state.Applier` над `memstore` без `Store` I/O, `WithInvariants()` — реальный.
- **Дополнение (сведение 3, `contracts.md` v0.4 C-02 v1.2; З-2)**: статус **`abandoned`** — терминальный.
  - Переход `status: alive → abandoned` разрешён **только** по предложению gateway (`entity.update.proposed {atomic: true, cause: forget}`, `set path=status value=abandoned`, с `expected_version`, **без** `meta.agent`); предложение с `meta.agent` → `level_violation`; строка gateway в `contracts.OwnershipRules` (`Character.status → abandoned`, `Group.leader_id` включая `null`) — из T-006 волны 0.
  - Над `status ∈ dead | abandoned | ascended_final` любое предложение (в т. ч. повторный `abandoned`) → **`dead_entity`**: `dead` терминален по FR-023 и в `abandoned` не переходит.
  - `cause` дополнена значением `forget`; факт — `entity.updated {changed: [{path: status, old: alive, new: abandoned}], cause: forget}`; `narrative.output kind=death` по нему **не** генерируется (это не смерть).
- **Файлы**: `internal/state/{ownership,invariants,dedup,world}.go`, `shared/testkit/state/**` + тесты.
- **Зависимости**: T-055.
- **Ссылки**: `design.md` §4.1 I1-6, §5 (C-02); `state-and-mechanics.md` §4.5, §4.6; **`contracts.md` v0.4 C-02 v1.2**, C-13; `consolidation.md` §14.1 (З-2); **FR-034**, FR-061, NFR-013, NFR-020, BR-16.
- **DoD**: матрица отказов «proposer × тип × путь × причина» покрыта таблично, каждая причина встречается ≥ 1 раз; дедуп: повтор `proposal_id` не создаёт второй факт (`--chaos=duplicate`, NFR-013); `system` proposer разрешён только с `source=core/state`, `author` — только из `mvctl`; **(сведение 3)** тесты: `alive → abandoned` от gateway с `cause=forget` принимается; то же от агента → `level_violation`; `dead`/`abandoned`/`ascended_final` → `dead_entity`; **потребители заглушки (EPIC-003/EPIC-004) компилируются без правок** — проверяется прогоном их e2e на новой реализации; общий DoD §1.

### T-060: I1-10 · `internal/replay` — `EventClock`, `NullTimers`, `Recording`, middleware · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.4 · ⚠ только через tech-lead#1 (сборка в `cmd/multiverse/main.go`)
- **Описание**: `cursor.go`, `eventclock.go` (время из `timestamp` события, монотонность), `timers.go` (`NullTimers` — тики только из `tick.fired`), `journal.go`, `recording.go` (`*.jsonl` round-trip + `Index`), `middleware.go`; сборка в `cmd/multiverse`: `--mode=replay` → `EventClock` + `NullTimers` + middleware, `--recording=<file>`, `--id-source=sequence`. Правка `cmd/multiverse/main.go` — **одним PR через tech-lead#1**.
- **Файлы**: `internal/replay/**`, `cmd/multiverse/main.go` (регистрация).
- **Зависимости**: T-003 (волна 0), C-01.
- **Ссылки**: `design.md` §4.1 I1-10; `state-and-mechanics.md` §6; ADR-003, ADR-010; **US-017**, NFR-014, NFR-061.
- **DoD**: unit — `EventClock` монотонен и не читает wall-clock; `NullTimers` не порождает тиков; `Recording` round-trip побайтово и `Index` находит запись по `(correlation_id, agent.id, phase, attempt)`; `--mode=replay` не даёт ни одного обращения к `time.Now` в доменном коде (`forbidigo` + тест); общий DoD §1. ~~**(ревью T-410, 2026-09-11)** шина в `--mode=replay` получает таймеры от тех же `NullTimers`/`EventClock`, что и контексты, или явно освобождена от них: сегодня она получает ручные таймеры, которые никто не двигает, и первая повторная доставка после ошибки обработчика повиснет до отмены — тест на повтор доставки в replay.~~ **Заменено (T-416):** из двух вариантов архитектор выбрал «шина освобождена». **(T-416, 2026-09-11; C-01 v1.4 «Таймеры повторной доставки» и «Источники конструкторов»)** Процесс даёт шине реальные таймеры (`clock.RealTimers`) в любом режиме, включая `--mode=replay`. `EventClock`/`NullTimers` получают только контексты (`Deps.Clock`, `Deps.Timers`): доменное время — у контекстов, у транспорта его нет. В `--mode=replay` глобальный источник времени конструкторов (`eventbus.SetClock`) — `EventClock`, согласовать с T-055. Паузы повтора (100/500/2000 мс) в байты событий не входят; число попыток и порядок — входят и совпадают с live. Тест: в replay ошибка обработчика → повторная доставка ×3 без зависания, затем `dead_letters`; тесту разрешено дать шине ручные таймеры и двигать их самому.

### Подволна 1.5

### T-057: I1-7 · `state` — `objStore`, интенты, снапшоты, ротация, `latest.json` · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.5
- **Описание**: `store.go` (write-through в `shared/objstore`: `entities-{world}/{type}/{id}.json`, один PUT на сущность), `intent.go` (`entities-{world}/_intents/{proposal_id}.json` — только для пакетов > 1 сущности; `PutIntent` → PUT по `id` → `DeleteIntent`), `snapshot.go` (по `MV_STATE_SNAPSHOT_EVERY=200`, по `SIGTERM`, по admin-маршруту; ротация K=5; `latest.json` — **указатель** с атомарной заменой; публикация `snapshot.created`).
- **Файлы**: `internal/state/{store,intent,snapshot}.go` + тесты (unit + `-tags integration`).
- **Зависимости**: T-056, T-007 (волна 0).
- **Ссылки**: `design.md` §4.1 I1-7; `state-and-mechanics.md` §4.3, §4.4; **ADR-011**, ADR-013, ADR-021; C-14 v1.1; US-011.
- **DoD**: unit на `objstore.Memory` — снапшот, ротация K=5 (шестой удаляет первый), `latest.json` указывает на последний; integration (testcontainers MinIO из `versions.env`) — PUT/GET/List, `EnsureBucket` включает versioning/ILM, `latest.json` корректен после эмуляции падения между PUT; интент создаётся только при пакете > 1 сущности; общий DoD §1.

### T-061: e2e-харнесс `solo-30` (каркас прогона) · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.5
- **Описание**: каркас e2e EPIC-002: один процесс `--contexts=all --bus=memory --mode=replay`, сборка `Harness` v0 (EPIC-004) + `FakeEncounter` + `FakeNarrator` (EPIC-003 T-219/T-220) + реальные `mechanics` *(T-416: заменено `FakeNarrator(WithEncounterStub(Rules))` — такой заглушки нет)*; сбор доменных событий, сверка порядка `player.attacked → dice.rolled → combat.decided → entity.update.proposed → entity.updated → narrative.output`; режим `--chaos=duplicate`. До готовности `state` работает на `FakeState`, затем переключается флагом.
- **Файлы**: `internal/state/e2e_test.go` (или `test/e2e/solo30_test.go`, `-tags e2e`), хелперы в `internal/state/testsupport`.
- **Зависимости**: T-053, T-018 (волна 0).
- **Ссылки**: `design.md` §4.1 I1-11, §4.3 (`FakeEncounter`, T-219; *T-416: было `WithEncounterStub`*); `epics.md` §2 (I1-α); `contracts.md` §17, C-05 «Заглушка»; NFR-062.
- **DoD**: `make test-e2e` прогоняет 30 ходов ≤ 1 мин без Docker и без сети; порядок событий хода соответствует §7.1; `dead_letters` = 0; каркас параметризован реализацией State (заглушка/реальная); общий DoD §1.

### Подволна 1.6

### T-058: I1-8 · `bootstrap.go` + `mvctl world init` / `world status` · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.6 · ⚠ только через tech-lead#1 (реестр подкоманд `cmd/mvctl/main.go`)
- **Описание**: `state.Bootstrap(ctx, deps, worldID, fixturesDir)` — публикует `entity.create.proposed` (proposer `system`, `cause=init`, `source=core/state`, `proposal_id="bootstrap:{world}:{type}/{id}"`) в порядке world → region → npc → players, ждёт факты через `Journal.Tail` (таймаут 10 с), идемпотентен; `cmd/mvctl/internal/world/{init,status}.go`: `EnsureBucket` с `BucketOptionsFor`, отказ `exit 2` при существующем `latest.json` без `--force`, вызов `Bootstrap`, снапшот **seq 0 `reason=bootstrap`** через admin-маршрут (`--bus kafka`) или in-process (`--bus memory`), печать `entities_count` и `state_hash`.
- **Файлы**: `internal/state/bootstrap.go`, `cmd/mvctl/internal/world/**`, регистрация в `cmd/mvctl/main.go` (PR через tech-lead#1).
- **Зависимости**: T-057, T-010 и T-016 (волна 0).
- **Ссылки**: `design.md` §4.1 I1-8; **`state-and-mechanics.md` §4.10**; `epics.md` §2 («единый способ инициализации мира»); C-14 v1.1; `decomposition-review.md` §5.1 п. 1.
- **DoD**: `go run ./cmd/mvctl world init --world dark-forest-world --fixtures testdata/fixtures/ --bus memory` создаёт 6 сущностей и снапшот seq 0 (`entities_count: 6`, `cursor.system_events: 0`); повторный запуск без `--force` → `exit 2` «мир инициализирован»; повторный `Bootstrap` идемпотентен (дедуп по `proposal_id`, 0 новых фактов); тест сверяет статы фикстур с `Rules.Stats(kind)`; `world status` печатает `state_hash`, `entities_count`, `rules_version`; общий DoD §1.

### T-059: I1-9 · `state` — recovery, `/health`, admin-маршруты · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.6
- **Описание**: `recovery.go` — протокол `state-and-mechanics.md` §4.8 шаги (a)–(g): чтение `latest.json` → снапшот → `Journal.ReadRange` от курсора → применение фактов тем же `applyFact` → `analytics.replay.completed mode=recovery {identical, llm_calls, dice_rolled_new, log_gap}`; roll-forward висящего интента; битый/отсутствующий снапшот → не подниматься молча; `health.go` (секция `state`: `world`, `seq`, `state_hash`, `rules_version`, `cursor`); `admin.go` (`Routes(mux)` на `shared/runtime`, `POST /v1/admin/state/{world}/snapshot`, `runtime.AdminOnly`).
- **Файлы**: `internal/state/{recovery,health,admin}.go` + тесты.
- **Зависимости**: T-057.
- **Ссылки**: `design.md` §4.1 I1-9, §9 (строка «unit `state` recovery»); `state-and-mechanics.md` §4.8, §4.9; ADR-009 п. 9, ADR-011; **US-011** (все критерии), NFR-010, NFR-011.
- **DoD**: семь сценариев recovery покрыты unit-тестами: (a) штатная остановка → `identical=true`; (b) факты после снапшота догоняются; (c) висящая запись; (d) roll-forward интента; (e) битый снапшот → `/health fail`, процесс не поднимается с пустым состоянием; (f) дивергенция → `identical=false` + запись; (g) `log_gap`; `analytics.replay.completed` валиден против схемы; admin-маршрут отклоняет запрос без `X-Actor-Kind`; общий DoD §1.

### Подволна 1.7 (I1 завершение)

### T-062: I1-11 · e2e S1 / S3 на реальных `state` + `mechanics`, `--chaos=duplicate`, покрытие · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.7
- **Описание**: перевести каркас T-061 на реальные `state`/`mechanics`; **S1** — `solo-30` (30 ходов, снапшот и журнал без расхождений); **S3** — `recovery`: 10 ходов → `Stop/Start` in-process → `identical=true`, `llm_calls=0`, `dice_rolled_new=0`, игрок делает ход 11; `--chaos=duplicate` (дубли доставки не создают вторых фактов); добор покрытия до ≥ 60 % по `internal/{state,mechanics,replay}`.
- **Файлы**: `test/e2e/{solo30,recovery}_test.go`, добор unit-тестов.
- **Зависимости**: T-058, T-059, T-060, T-056.
- **Ссылки**: `design.md` §4.1 I1-11, §9; `scenarios.md` S1, S3; **US-011**, US-017; NFR-012, NFR-013, NFR-061, NFR-062, NFR-064.
- **DoD**: `make test-e2e` зелёный ≤ 10 мин без Docker; S1 — 30 `narrative.output`, `state_hash` снапшота = хэш пересчёта по журналу; S3 — `identical=true`, `llm_calls=0`; `--chaos=duplicate` — число фактов не изменилось; `coverage-gate.sh 60 internal/state internal/mechanics internal/replay` зелёный; общий DoD §1.

### T-063: I1-α · Стендовый прогон «соло на шаблонах через бота» · Размер: S · Статус: todo · Исполнитель: tech-lead#1 + пользователь (**стенд**, слот не занимает) · Подволна 1.9
- **Описание**: сборка EPIC-002 I1 + EPIC-004 I1 + `FakeNarrator` в `integration/mvp-1` (порядок слияния 002 → 004), `make up`, `mvctl world init`, живая игра человека через Telegram: создание персонажа, вход в регион, `look`, бой с волком, `rest`, `/forget`. Замечания → задачи EPIC-002 (T-0NN) / EPIC-004 (T-3NN).
- **Зависимости**: T-062; EPIC-004 подволна 1.9 принята tech-lead#3 (T-313 e2e соло, T-315 e2e бота; стендовая пара — T-391 EPIC-004); `FakeNarrator` T-220 (EPIC-003) слит ранним merge или используется v0 из T-018.
- **Ссылки**: `epics.md` §2 (точка I1-α), §4; `teams.md` §3.3 п. 2, §5.
- **DoD**: сквозной соло-ход через бота работает с `generated_by=template`; тег **`mvp-1/i1-alpha`** на `integration/mvp-1`; список замечаний человека оформлен задачами с указанием эпика-владельца; запись в `journal.md`.

### Подволны 1.9–1.10 (хвост I1; TEAM-1 переходит на I2 и 005-ops)

### T-064: I1-12 · Архив as-is `entity-manager`, `rule-engine`, `shared/rules` · Размер: S · Статус: todo · Исполнитель: developer#2 · Подволна 1.10
- **Описание**: после зелёного S3 — `git mv services/entity-manager`, `services/rule-engine`, `shared/rules` → `services/_archive/<путь>` с `ARCHIVED.md` (причина, коммит, последний рабочий коммит, эпик возврата — «нет», что использовало) и записью в `services/_archive/README.md`; исключить из `Makefile`/compose, если остались упоминания.
- **Файлы**: `services/_archive/**`, `services/_archive/README.md`.
- **Зависимости**: T-062 (зелёный S3).
- **Ссылки**: `design.md` §1 («перенос as-is … — последняя задача I1»), §6; `infrastructure.md` §4.6; U-1; `ownership.md` §1.
- **DoD**: `go build ./...` не видит перенесённые пакеты; `git log --follow` читается; `services/_archive/README.md` дополнен; общий DoD §1.

---

## 3. Инкремент I2 «группа» — задачи T-065…T-071 (подволны 1.8–1.14, слияние — после интеграции I1)

Старт — **по приёмке I1 тимлидом команды** (не дожидаясь интеграции трёх эпиков); **слияние** I2 в `integration/mvp-1` — после зелёного интеграционного прогона I1 (S1, S3, S8, S9, S10, S14), в порядке 002 → 003 → 004 → 005.

### T-065: I2-1a · Atomic-пакеты группы: перемещение, вступление, выход · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.8
- **Описание**: `group.entered_region` → **один atomic-пакет** (сущность группы + все участники: `position`, `scope`), `group.joined`/`group.left` (`members`, `scope`, `group_id`); протокол `PutIntent` → PUT по `id` → `DeleteIntent`; **roll-forward** незавершённого интента при старте worker'а.
- **Файлы**: `internal/state/{apply,intent}.go` (расширение), `internal/state/group.go` + тесты.
- **Зависимости**: T-057, T-059.
- **Ссылки**: `design.md` §4.2 I2-1; `state-and-mechanics.md` §4.7; **ADR-013**; C-02 v1.1, C-04; US-006, BR-13.
- **DoD**: пакет из 6 сущностей применяется целиком или не применяется вовсе (тест на сбое середины); roll-forward после «падения» между PUT восстанавливает пакет однозначно; `version_conflict` внутри пакета откатывает весь пакет; общий DoD §1.

### T-066: I2-1b · Инварианты группы inv-04, inv-05, inv-06 по `overlayView` · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.9
- **Описание**: реализации `Check` для inv-04 (по `alive` участникам — допущение, ждёт BA, S-7), inv-05, inv-06 через `overlayView` (представление «состояние + непринятые ops пакета»); конфликт версий между уровнями владения.
- **Файлы**: `internal/mechanics/invariants.go`, `internal/state/overlay.go` + тесты.
- **Зависимости**: T-065, T-054.
- **Ссылки**: `design.md` §4.2 I2-1, §11 (риск inv-04); `state-and-mechanics.md` §5.5; NFR-020; BR-13, BR-16 п. 4.
- **DoD**: позитив/негатив на каждый из трёх инвариантов; тест «участник умер — группа не разваливается, мёртвый остаётся в `members`»; решение BA по inv-04 зафиксировано в `dev-log.md` (при ином решении — правка одного `Check`); общий DoD §1.

### T-067: I2-2 · `Participation` в механике (`idle`/`out_of_combat`) · Размер: S · Статус: todo · Исполнитель: developer#1 · Подволна 1.10
- **Описание**: `Actor.Participation` из `encounter.participants[]`; `NPCTarget` исключает `idle|out_of_combat|dead`; `Resolve` возвращает `ErrInvalidTarget` для недопустимой цели; `ActorsFromEncounter`.
- **Файлы**: `internal/mechanics/{target,resolve,actor}.go` + тесты.
- **Зависимости**: T-053.
- **Ссылки**: `design.md` §4.2 I2-2; `state-and-mechanics.md` §5.4; C-03 v1.1, C-05 v1.1; US-007.
- **DoD**: таблица «состав участников → выбранная цель» покрыта; `idle` игрок не выбирается целью; `Resolve` по мёртвой цели → `ErrInvalidTarget` без изменения состояния; общий DoD §1.

### T-068: I2-3 · Снапшот по `analytics.session.ended` · Размер: S · Статус: todo · Исполнитель: developer#1 · Подволна 1.11
- **Описание**: подписка группой `core.state.triggers` на `analytics_events` (**не в режиме replay** — там по счётчику), снапшот с `reason=session_ended`.
- **Файлы**: `internal/state/snapshot.go`, `internal/state/triggers.go` + тесты.
- **Зависимости**: T-057.
- **Ссылки**: `design.md` §4.2 I2-3, §11 (риск S-10); ADR-003, ADR-011; C-10; US-038 (вклад).
- **DoD**: снапшот создаётся один раз на `session.ended` (дубли события не создают второй снапшот); в `--mode=replay` подписка отключена и снапшот берётся по счётчику; общий DoD §1.

### T-069: I2-4 · `internal/state/audit` — `Recompute` и `Compare` · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.12
- **Описание**: `audit.Recompute(snapshot *Snapshot, facts iter.Seq[eventbus.Event]) (hash string, entities []*entity.Entity, err error)` — применяет `entity.created/updated` к снапшоту **тем же** `applyFact`, что recovery; `audit.Compare(hashA, hashB) Divergence` (какая сущность, какое поле, какая версия). Библиотека для `mvctl report --audit` (EPIC-005 T-137; *T-416: было `session-report`*) и для теста S3.
- **Файлы**: `internal/state/audit/**` + тесты.
- **Зависимости**: T-059.
- **Ссылки**: `design.md` §4.2 I2-4, §5 (строка `internal/state/audit`); `epics/EPIC-005-memory-ops/design.md` §3 п. 2, §5; `state-and-mechanics.md` §3.3, §4.8; US-038; NFR-032.
- **DoD**: тест «снапшот + факты = объекты `entities-{world}`» на подложенном расхождении даёт `Divergence` с указанием сущности и поля; `Recompute` и recovery используют **один** `applyFact` (проверяется тестом на общем наборе фактов); API согласовано с EPIC-005 (уведомление tech-lead#1); общий DoD §1.

### T-070: I2-5 · e2e `group-3x30` · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 1.13
- **Описание**: сценарий три игрока × 30 раундов: одновременные удары (`version_conflict` + успешный повтор), atomic-перемещение группы, `idle` участник, смерть участника (мёртвый остаётся в `members`), `flee` из группы. Харнесс — `Harness` v1 (EPIC-004) либо `Harness` v0 + генератор `round.opened/closed`.
- **Файлы**: `test/e2e/group3x30_test.go`.
- **Зависимости**: T-065, T-066, T-067; `Harness` v1 (EPIC-004 I2) — при отсутствии запрос владельцу.
- **Ссылки**: `design.md` §4.2 I2-5, §11 (допущение про `Harness`); `scenarios.md` S2; US-006, US-007.
- **DoD**: `make test-e2e` зелёный; конфликт версий разрешается повтором без потери хода; `state_hash` совпадает с пересчётом по журналу; 0 `dead_letters`; общий DoD §1.

### T-071: I2-6 · Замер латентности atomic-пакета группы (стенд) · Размер: S · Статус: todo · Исполнитель: developer#1 + пользователь (**стенд**) · Подволна 1.14
- **Описание**: замер NFR-001 на стенде: латентность применения пакета из 6 сущностей через интент (`applied.duration_ms` из лога), 20 прогонов, p50/p95; при провале порога — задача «вариант B ADR-013 (объект-транзакция)» отдельно, контракты не меняются.
- **Файлы**: `ops/metrics/bench-state-<date>.csv`, запись в `dev-log.md` и `journal.md`.
- **Зависимости**: T-065, стенд (`make up`).
- **Ссылки**: `design.md` §4.2 I2-6, §11 (риск «интент +40 мс», S-9); ADR-013; **NFR-001** (механика ≤ 0,5 с).
- **DoD**: CSV с p50/p95 по 20 прогонам; вердикт «проходит NFR-001 / не проходит»; при провале — заведена задача на вариант B с оценкой; результат передан BA для закрепления порога (EPIC-005 T-140).

---

## 4. Стендовые задачи EPIC-002 (stand)

| Задача | Что на стенде | Когда | Кто |
|---|---|---|---|
| **T-063 (I1-α)** | живая игра через Telegram на шаблонах: персонаж, вход, `look`, бой с волком, `rest`, `/forget`; тег `mvp-1/i1-alpha` | подволна 1.7 | tech-lead#1 + пользователь |
| T-058 (часть) | `mvctl world init --bus kafka` против поднятого `core` и MinIO (`make up`) | подволна 1.6 | developer#1 + пользователь |
| T-057 (часть) | integration-тесты `objStore` над собранным образом MinIO (Docker Desktop) | подволна 1.5 | developer#2 |
| T-062 (часть) | S3 recovery на живом стенде (перезапуск процесса между ходами) | интеграция I1 | tester#1 + пользователь |
| **T-071 (I2-6)** | латентность atomic-пакета группы из 6 сущностей, 20 прогонов | подволна 2.3–2.4 | developer#1 + пользователь |

---

## 5. Зависимости от других эпиков и допущения

| Что нужно | От кого | Заглушка до готовности | Риск |
|---|---|---|---|
| C-01 (шина, журнал, часы, рантайм, `objstore`) | EPIC-001 (волна 0) | — (готово к старту) | contract-тест T-014 — ворота волны 1 |
| `Harness` (генератор `player.*`) | EPIC-004 | `Harness` v0 (T-018) | для `group-3x30` нужен `Harness` v1 или генератор `round.*` — запрос EPIC-004 |
| `FakeEncounter` + `FakeNarrator` *(T-416: было `FakeNarrator` + `WithEncounterStub`)* | EPIC-003 (T-219, T-220 приняты; по C-05 v1.4 — T-419) | `FakeEncounter` (T-219) — единственная заглушка боя | после слияния EPIC-003 I1b двойники заменяет настоящий агент встречи — e2e T-062 переводится на него (задача EPIC-003) |
| таблица владения `contracts.OwnershipRules` *(T-416: было «`shared/agent/levels.go` ↔ `contracts.OwnershipRules`»)* | EPIC-001 (T-006); строки `author`/`system` — EPIC-002 | — (готова, T-006) | таблица одна (ADR-025, подтверждён T-416), сверять нечего; строки меняют владельцы семантики PR с `contract-change`; норма «путь ↔ причина» проверяется поверх таблицы (C-02 v1.4, T-056) |
| `analytics.session.ended` | EPIC-004 | фикстуры `testdata/analytics` | I2-3 |
| `internal/state/audit` | **поставщик** для EPIC-005 (T-137) | режим `partial` у `--audit` до T-069 | API согласовать до подволны 2.2 |

**Допущения**: (1) inv-04 считается по `alive` участникам до ответа BA; (2) горячая перезагрузка `rules/dark-forest.yaml` не в MVP-1 — `rules_version` в `/health` у `state` и `swarm` сверяет тест; (3) `MV_STATE_WORLDS=dark-forest-world` — один мир на MVP-1; (4) имя ветки эпика — `epic/EPIC-002-state-mechanics` (фиксируется в T-020 EPIC-001, `teams.md` §3.2 правится там же).
