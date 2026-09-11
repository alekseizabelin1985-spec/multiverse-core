# Дизайн эпика EPIC-002 «Состояние и механика»

Версия 0.1 · 2026-09-09 · architect#1 (TEAM-1) · статус: к нарезке tech-lead#1 (`tasks.md`), инкремент I1 — волна 1, I2 — после приёмки I1 тимлидом.
Команда TEAM-1 · ветка `epic/EPIC-002-state-mechanics` (от `integration/mvp-1` после тега `mvp-1/wave-0`; не создавать до конца волны 0) · G2 утверждён 2026-09-09.
Основание: `plan/epics.md` v0.2 §2 «EPIC-002» (I1/I2, единый способ инициализации мира); `architecture/components/state-and-mechanics.md` v0.2 (детальная архитектура — **не дублируется**, ссылки по §); ADR-011 (хранение/снапшоты), ADR-012 (правила как данные), ADR-013 (версии/atomic); `contracts.md` v0.2 (C-01 v1.1, C-02 v1.1, C-03 v1.1, C-13, C-14 v1.1); `consolidation.md` §2 (S-1…S-10); `epics/EPIC-001-foundation/design.md` §5 (что уже есть после F-10).

---

## 1. Цель и границы

**Цель**: единственный писатель состояния мира (`internal/state`), детерминированная механика боя как данные (`internal/mechanics`, `rules/dark-forest.yaml`), режим replay (`internal/replay`), инициализация мира из фикстур (`mvctl world init`) — так, чтобы I1-α «соло на шаблонах» работала без роя, а рестарт давал `identical=true, llm_calls=0`.

**Входит**: I1 — State (предложения → факты, версии, инварианты inv-01…10, MinIO write-through, снапшот/`latest.json`, восстановление, `analytics.replay.completed mode=recovery`), `bootstrap.go` + `mvctl world init --fixtures`, Mechanics (`Resolve`, `NPCTarget`, seed/RNG, `ChangesFor`, `DiceRolledPayload`, инварианты), `internal/replay` (`EventClock`, `NullTimers`, `Recording`, middleware), `shared/entity` v2 (принимает от F-10), замена v0 `FakeState`/`FixedMechanics` реализацией, схемы `entity.*`, `dice.rolled`, `snapshot.created`, `analytics.replay.completed`. I2 — atomic-пакеты для группы (позиция, участники, `Participation`), `idle`/`out_of_combat` в `NPCTarget`, снапшот по `analytics.session.ended`, `internal/state/audit` для `mvctl report --audit` (совместно с 005-ops), e2e `group-3x30`.

**Не входит**: публикация `dice.rolled`/`combat.decided`/`entity.update.proposed` в бою — агент встречи (EPIC-003; до него — `testkit/swarm.FakeEncounter`, EPIC-003 T-219; `WithEncounterStub` не делается, C-05 v0.3); координация раундов и `rest`-валидация «не во встрече» на входе (EPIC-004); файл законов `laws/*.yaml` (EPIC-003; EPIC-002 — только `Invariants()` с теми же id); `mvctl report` (EPIC-005; EPIC-002 даёт библиотеку `audit.Recompute`); перенос as-is `entity-manager`/`rule-engine` в архив — последняя задача I1 после зелёного S3.

---

## 2. Входящие требования (трассировка)

| Требование | Что в EPIC-002 | Инкремент | UC / проверка |
|---|---|---|---|
| US-003 (FR-018, FR-020…FR-022, BR-05) | `rules/dark-forest.yaml`, `Resolve`, `Seed`/RNG, `dice.rolled` payload; трасса по `correlation_id` — через `Derive` (все факты несут `causation_id`) | I1 | UC-007, UC-008, UC-010; тест 1000×4 (NFR-060) |
| US-005 (FR-030, FR-031, BR-03) | состояние — только через факты; HP не меняется фоном (владение `global/domain` не даёт `hp` игрока) | I1 | UC-004 A2, UC-011 |
| US-011 (FR-031…FR-033, FR-087, NFR-010, NFR-011) | снапшоты, `latest.json`, recovery, `replay.completed mode=recovery`, `/health` при битом снапшоте | I1 | UC-025, S3 |
| US-017 (FR-100, NFR-060, NFR-061) | `internal/replay`: `EventClock`, `NullTimers`, `Recording`; факты с `timestamp` причины; `--id-source=sequence` | I1 | UC-026 |
| FR-023, FR-024, FR-025 (позиция, scope, `say`) | ops `position/scope` с инвариантами inv-06, inv-10 | I1 (соло) / I2 (группа) | UC-013, UC-015 |
| FR-034 (причины отказа) | enum `reason` C-02 v1.1 включая `duplicate_entity` | I1 | unit-матрица отказов |
| NFR-012, NFR-013 (0 расхождений; идемпотентность) | версии строго +1; дедуп `proposal_id` (LRU + `last_change`); курсор вместо consumer group | I1 | `--chaos=duplicate` e2e |
| NFR-014 (replay без источников) | mechanics без часов; `NullTimers`; `EventClock` | I1 | UC-026 |
| NFR-020 (inv-01…10) | одна реализация в `mechanics/invariants.go`, id = `laws@v1` | I1 (inv-01,02,03,09,10), I2 (inv-04,05,06 — группы) | unit позитив/негатив |
| NFR-001 (механика ≤ 0,5 с) | один PUT на сущность; интент только для > 1 сущности | I1 / замер I2 | стенд, лог `applied.duration_ms` |
| NFR-064 (покрытие ≥ 60 %) | `internal/{state,mechanics,replay}` | I1 | job `unit` |
| BR-13, BR-16 п. 4, inv-04/05/06 | atomic-пакеты группы, конфликт версий между уровнями | I2 | UC-015, UC-016, UC-017 |
| `epics.md` §2 (инициализация из фикстур) | `bootstrap.go`, `mvctl world init --fixtures` | I1 | «`mvctl world init` создаёт „Тёмный лес“» |

---

## 3. Подход

1. **Стартуем от F-10, а не с нуля**: `shared/entity` v2, типы `mechanics` + `Load` + YAML, `FakeState` v0, `FixedMechanics`, фикстуры уже в `integration/mvp-1`. Первая задача I1 — не «написать entity», а «дополнить и покрыть тестами».
2. **Две независимые нитки в волне 1** (`teams.md` §4: developer#1 — `mechanics`, developer#2 — `entity`/`state`): `mechanics` — чистая библиотека, тестируется таблицами; `state` — над `memstore` + `membus`, без MinIO до задачи `objStore`.
3. **Порядок I1** (`state-and-mechanics.md` §12): `entity` + схемы → `mechanics` (RNG, `Resolve`, `NPCTarget`, `Invariants`, `ChangesFor`) → `state` (memstore, Applier, факты; замена `FakeState` v0) → `objStore` + снапшоты → `bootstrap` + `mvctl world init` → recovery + `replay` → e2e S1/S3 → архив as-is.
4. **I1-α как точка приёмки I1** внутри TEAM-1: `solo-30` на `FakeEncounter` + `FakeNarrator` (EPIC-003 T-219/T-220) + `Harness` v0 в одном процессе `--bus=memory --mode=replay`; затем живая игра через бота (EPIC-004 I1) на стенде.
5. **I2 стартует по приёмке I1 тимлидом** (правило `epics.md` §2), слияние I2 — после интеграционного прогона I1; освободившийся слот уходит в 005-ops.

---

## 4. Компоненты и инкременты

Компоненты, структура пакетов, модель данных, потоки — `state-and-mechanics.md` §1–§7 (v0.2). Ниже — разбиение по инкрементам и что специфично для эпика.

### 4.1. Инкремент I1 «соло + фон»

| # | Блок | Пакеты / файлы | Что входит | Зависит от |
|---|---|---|---|---|
| I1-1 | `entity` дозаполнение | `shared/entity/*` | проверка §3 против F-10: `attrs.go` геттеры для всех атрибутов `data-model.md` §3, `HistoryEntry` обрезка 50, `Ref` ⇄ `eventbus.EntityRef`, тесты `ApplyOps` полные (§11) | F-10 |
| I1-2 | схемы EPIC-002 | `schemas/events/{entity.create.proposed,entity.update.proposed,entity.created,entity.updated,entity.update.rejected,dice.rolled,snapshot.created,analytics.replay.completed}.v1.json` | ревизия созданных в F-4b: ops enum, `expected_version` опц., `reason` enum v1.1, `changed[]` для `append`, `component` enum, `mode ∈ recovery|test`; примеры из `api-contracts.md` §2.3.4/5/12 | F-4b |
| I1-3 | `mechanics` RNG + формулы | `rng.go`, `formula.go` | `Seed`/`NewRNG`/бросок `NdM+K` на одном RNG; парсер `check`/`dice` (уже в F-10 для `Load`) — довести вычислитель; тест 1000×4 | F-10 |
| I1-4 | `mechanics` исходы | `resolve.go`, `target.go`, `changes.go`, `dice_event.go`, `actor.go` | таблица §5.4; `NPCTarget` (без `Participation` — I2); `ChangesFor` (ops HP clamp, `status`, `died_at/killed_by`, `inventory append` с `source`, `encounter.npcs[].last_damager`); `DiceRolledPayload` ↔ схема | I1-3 |
| I1-5 | `mechanics` инварианты | `invariants.go` | реализации `Check` для inv-01, 02, 03, 09, 10 (соло); inv-04/05/06 — `Check` по `overlayView` в I2; inv-07/08 — `Check=nil` (`Where: audit|guardian`) | I1-1 |
| I1-6 | `state` ядро | `context.go`, `worker.go`, `proposal.go`, `apply.go`, `ownership.go`, `invariants.go`, `world.go`, `dedup.go`, `facts.go`, `memstore/` | конвейер §4.5 на `memstore` + `membus`; `contracts.OwnershipRules` (статичная таблица F-4b); все причины отказа; `atomic` all-or-nothing в памяти; **замена `FakeState` v0**: `testkit/state.FakeState` = `state.Applier` над `memstore` без `Store` I/O, `WithInvariants()` реальный | I1-1, I1-5, C-01 |
| I1-7 | `state` персист + снапшоты | `store.go` (objStore), `intent.go`, `snapshot.go` | `Store` над `shared/objstore`; интент для пакетов > 1 сущности (в соло — пакет «встреча + игрок + NPC» из агента, §7.1); снапшот по `MV_STATE_SNAPSHOT_EVERY`, `SIGTERM`, admin; ротация K=5; `latest.json` указатель; `snapshot.created` | I1-6, F-5 |
| I1-8 | `bootstrap` + `mvctl world init` | `bootstrap.go`, `cmd/mvctl/internal/world/{init,status}.go` | §4.10 `state-and-mechanics.md`; `EnsureBucket` с `BucketOptionsFor`; снапшот seq 0 `reason=bootstrap`; идемпотентность повторного запуска; `world status` | I1-7, F-4c |
| I1-9 | `state` recovery + `health` + `admin` | `recovery.go`, `health.go`, `admin.go` | протокол §4.8 (a)–(g); `Routes(mux)` на `shared/runtime`; `/health` секция; `analytics.replay.completed mode=recovery`; `log_gap` | I1-7 |
| I1-10 | `internal/replay` | `cursor.go`, `eventclock.go`, `timers.go`, `journal.go`, `recording.go`, `middleware.go` | §6; сборка в `cmd/multiverse` (`--mode=replay` → `EventClock`, `NullTimers`, middleware) — PR в `cmd/multiverse/main.go` через tech-lead#1 | F-2 |
| I1-11 | e2e S1 / S3 | `internal/state/e2e_test.go` | `solo-30` (Harness v0 + `FakeEncounter` + `FakeNarrator`), `recovery` (10 ходов → `Stop/Start` in-process → `identical=true, llm_calls=0`), `--chaos=duplicate` | I1-8…I1-10, F-10 |
| I1-12 | архив as-is | `services/_archive/{services/entity-manager,services/rule-engine,shared/rules}` | `git mv` + `ARCHIVED.md` (U-1); только после зелёного S3 | I1-11 |

Критерий готовности I1 (из `epics.md`): `mvctl world init` создаёт «Тёмный лес» из фикстур; 30 ходов соло на `FakeNarrator` без расхождений снапшота и журнала; рестарт → `identical=true`, `llm_calls=0`; участие в I1-α (живая игра через бота).

### 4.2. Инкремент I2 «группа»

| # | Блок | Что входит | Зависит от |
|---|---|---|---|
| I2-1 | atomic-пакеты группы | §4.7: `group.entered_region` → один пакет (группа + участники), `PutIntent` → PUT по `id` → `DeleteIntent`; `group.joined/left` (members + scope + group_id); roll-forward при старте; inv-04 (по `alive` — до ответа BA), inv-05, inv-06 с `Check` по `overlayView` | I1-7, I1-9 |
| I2-2 | `Participation` в механике | `Actor.Participation` из `encounter.participants[]`; `NPCTarget` исключает `idle|out_of_combat|dead`; `Resolve` → `ErrInvalidTarget`; `ActorsFromEncounter` | I1-4 |
| I2-3 | снапшот по `analytics.session.ended` | подписка группой `core.state.triggers` на `analytics_events` (не в replay); `reason=session_ended` | I1-7 |
| I2-4 | `internal/state/audit` | `audit.Recompute(snapshot *Snapshot, facts iter.Seq[eventbus.Event]) (hash string, entities []*entity.Entity, err error)` — применяет `entity.created/updated` к снапшоту тем же `applyFact`, что recovery; `audit.Compare(hashA, hashB) Divergence`; библиотека для `mvctl report --audit` (EPIC-005) и для теста S3 | I1-9 |
| I2-5 | e2e `group-3x30` | одновременные удары (`version_conflict` + повтор), atomic-перемещение группы, `idle`, смерть участника (мёртвый в `members`), `flee` из группы | I2-1, I2-2; `Harness` v1 (EPIC-004) или v0 + `round.closed` генератор |
| I2-6 | замер NFR-001 на стенде | латентность пакета из 6 сущностей (интент); при провале — вариант B ADR-013 (объект-транзакция) отдельной задачей | I2-1, стенд |

Критерий готовности I2: S2 проходит с харнессом из трёх клиентов (после EPIC-004 I2); `group-3x30` в CI; `--audit` на записи S2 даёт `state_divergence = 0`.

### 4.3. Специфика, которой нет в `state-and-mechanics.md`

- **Список миров**: `MV_STATE_WORLDS=dark-forest-world` (через запятую) — по одному worker'у на мир; мир без `latest.json` и без объектов — «не инициализирован» (`/health degraded {world: uninitialized}`), предложения отклоняются `unknown_entity` до `mvctl world init`.
- **Путь правил**: `MV_RULES_PATH=rules/dark-forest.yaml`. `mechanics` не является `runtime.Context` с состоянием: `Deps` общий и не знает доменных типов, поэтому `state` и `swarm` каждый вызывают `mechanics.Load(MV_RULES_PATH)` сами; `rules_version` в секциях `/health` обоих должен совпадать — проверяет тест интеграции. Имя `mechanics` в `--contexts` остаётся для совместимости с ADR-001 (пустой контекст, `Health()` отдаёт `rules_version`).
- **`testkit/swarm.FakeEncounter`** (EPIC-003 T-219; `contracts.md` C-05 «Заглушка») — заглушка Phase 1 боя для I1-α и потребитель механики (до T-053 строит операции пакета сам). После слияния EPIC-003 I1b её заменяет агент встречи. EPIC-002 ею не владеет, но e2e S1 EPIC-002 строится на ней. `FakeNarrator.WithEncounterStub` не делается (C-05 v0.3, решение d); прежние упоминания в этом документе читать как `FakeEncounter`.
- **Таблица владения**: State читает `shared/contracts/ownership.go` — единственную истину (ADR-025, C-02 v1.4); `shared/agent/levels.go` (EPIC-003) своей таблицы не держит, сверять нечего. Связку «путь ↔ причина» State проверяет поверх таблицы (C-02 v1.4, T-056).

---

## 5. Интерфейсы с другими эпиками

| Контракт | Роль EPIC-002 | Что поставляет / потребляет | Заглушка до готовности |
|---|---|---|---|
| C-02 (предложения/факты) | **поставщик** | вход `entity.*.proposed`; выход `entity.created/updated/update.rejected`; read-model через `latest.json` + `Journal` | `FakeState` v0 (F-10) → реализация I1-6 |
| C-03 (механика, Go API) | **поставщик** | `*mechanics.Rules` полный набор; `rules/dark-forest.yaml` | `FixedMechanics` (F-10) → `Resolve` I1-4; интерфейс объявляет потребитель |
| C-14 (снапшоты) | **поставщик формата** | `snapshot.created`, `latest.json`, порядок старта (`state` первым, `replay.completed` — сигнал остальным) | фикстура `latest.json` seq 0 |
| `analytics.replay.completed` | совладение с EPIC-005 | файл схемы — EPIC-002; `mode=test`, `events_hash_match` — EPIC-005 через PR с ревью EPIC-002 | — |
| C-13 (резерв `object`) | поставщик | пустые строки `object/monitor` в `OwnershipRules`; `level_violation` до `MV_SWARM_OBJECT_AGENTS_ENABLED` | — |
| C-01 (шина/журнал/часы/рантайм) | потребитель | `Bus.Publish`, `Journal.ReadRange/Tail/End`, `PositionFromContext`, `Dedup`, `objstore`, `clock`, `runtime.Routes` | `membus` (EPIC-001) |
| C-04 (действия игрока) | потребитель косвенно (через предложения gateway) | `entity.create.proposed player/group`, `entity.update.proposed position/scope/rest` | `Harness` v0 (F-10) |
| C-05 (`combat.decided` и др.) | потребитель косвенно (агент встречи публикует предложения) | `entity.update.proposed cause=combat` из `ChangesFor` | `testkit/swarm.FakeEncounter` (T-219) |
| `internal/state/audit` (Go, внутри TEAM-1) | поставщик для 005-ops | `Recompute`, `Compare` | — (I2) |

---

## 6. Данные и миграции

- **Объекты**: `entities-{world}/{type}/{id}.json`, `entities-{world}/_intents/{proposal_id}.json`, `snapshots-{world}/state/{ts}-{seq}.json`, `snapshots-{world}/state/latest.json` (ADR-011, §4.3–§4.4). Бакеты создаёт `mvctl world init` (`EnsureBucket` с versioning/ILM; versioning — не опора, ADR-021).
- **Схемы**: восемь файлов §4.1 I1-2; версия 1; несовместимых изменений в MVP-1 нет.
- **Миграция as-is**: бакеты с плоским `payload` не мигрируют; MVP-1 стартует с `mvctl world init` (снапшот seq 0). `services/entity-manager`, `rule-engine`, `shared/rules` → `services/_archive/` в I1-12 (U-1).
- **Фикстуры**: `testdata/fixtures/` — общие (созданы F-10); EPIC-002 при изменении атрибутов уведомляет tech-lead#1.
- **Версионирование правил**: `rules_version` семвер в YAML; изменение чисел — правка YAML + золотые тесты; в `combat.decided.rules_version` и снапшоте.

---

## 7. Конфигурация

`MV_STATE_WORLDS` (default `dark-forest-world`), `MV_STATE_SNAPSHOT_EVERY=200`, `MV_RULES_PATH=rules/dark-forest.yaml`, `MV_STATE_PERSIST_RETRIES=3`; общие — `MV_MODE`, `MV_BUS`, `MV_CORE_ADDR`, `MV_MINIO_*`, `MV_KAFKA_BROKERS`. Флаги `mvctl world init`: `--world`, `--fixtures`, `--bus`, `--force`. Все через `env.Declare`.

---

## 8. Безопасность (что учтено)

- Владение (`level_violation`) по `contracts.OwnershipRules` — агент/gateway не может менять чужие атрибуты (BR-16); `system` proposer только из `bootstrap` (`source=core/state`), `author` — только `mvctl`.
- Admin-маршрут снапшота — `runtime.AdminOnly` (`X-Actor-Kind: ci`/оператор), loopback (ADR-009 п. 9).
- В сущностях и фактах нет внешних ID и текста игроков (`player.said.text` не попадает в состояние); `privacy-scan` покрывает фикстуры.
- Валидация схем при чтении (`MV_BUS_VALIDATE_ON_READ`) — невалидные предложения не доходят до Applier.
- Панику в worker'е не глотаем (NFR-012): `/health fail`, мир останавливается.

---

## 9. Тестируемость (ADR-010) — что покрывает критерии I1/I2

| Уровень | Тесты (полный перечень — `state-and-mechanics.md` §11) | Закрывает |
|---|---|---|
| unit `mechanics` | RNG 1000×4 (детерминизм, независимость, χ²); `Seed` векторы; формулы; `Resolve` таблица; `NPCTarget`; `Load` золотые числа; `ChangesFor`; `DiceRolledPayload` ↔ схема | US-003, NFR-060, C-03 |
| unit `entity` | `ApplyOps` все op/ошибки/no-op; `StateHash` стабильность | C-02 семантика |
| unit `state` | матрица отказов (proposer × тип × путь × причина); atomic/non-atomic; дедуп; инварианты; `create` дубликат; `rest`; `bootstrap` идемпотентность и сверка статов с `Rules.Stats` | FR-034, NFR-013, NFR-020, I1 «world init» |
| unit `state` recovery | (a) штатная остановка `identical=true`; (b) факты после снапшота; (c) висящая запись; (d) roll-forward интента; (e) битый снапшот; (f) дивергенция; (g) `log_gap` | US-011, NFR-010/011, I1 «рестарт → identical» |
| unit `replay` | `EventClock` монотонность; `NullTimers`; `Recording` round-trip/`Index` | US-017, NFR-014 |
| integration | `objStore` над MinIO (testcontainers, образ из `versions.env`): PUT/GET/List, `latest` после падения между PUT; `Journal` над redpanda — общий с EPIC-001 | ADR-011, ADR-021 |
| e2e I1 | `solo-30` (S1) на `FakeEncounter` + `FakeNarrator` (EPIC-003 T-219/T-220); `recovery` (S3) in-process; `--chaos=duplicate` | I1 «30 ходов без расхождений», «рестарт identical, llm_calls=0» |
| e2e I2 | `group-3x30`: конфликт версий + повтор, atomic-перемещение, `idle`, смерть участника | I2, US-006/007 вклад |
| contracts | схемы EPIC-002 валидны и покрывают примеры; `Invariants()` ↔ `laws@v1` (совместно с EPIC-003 через `mvctl laws check`) | C-02/C-14 |
| стенд | I1-α живая игра (человек); латентность пакета группы (I2-6) | NFR-001, US-002 вклад |
| покрытие | `internal/{state,mechanics,replay}` ≥ 60 % | NFR-064 |

---

## 10. Отклонённые альтернативы

| Альтернатива | Почему отклонена |
|---|---|
| Consumer group для `system_events` у State | ADR-011: курсор снапшота — единственная истина; старые предложения не перечитываются |
| `latest.json` — полный снапшот | ADR-011 / C-14 v1.1: указатель, атомарная замена, ротация без дублей |
| Общий движок выражений (`expr`, CEL) для формул | ADR-012: мини-грамматика — детерминизм и узкая поверхность |
| Инварианты в State отдельно от законов | ADR-012 п. 5: одна реализация с id `laws@v1` |
| Объект-транзакция для atomic-пакетов | ADR-013 вариант B — резерв при провале замера NFR-001 |
| Инициализация мира из `npc_table` блупринта (`swarm.InitWorld`) | решение G2: фикстуры — источник; I1-α без роя |
| `mvctl world init` в EPIC-005 | нужен для критерия I1 (`decomposition-review.md` §5.1 п. 1) |
| `internal/mechanics` как `runtime.Context`, отдающий `*Rules` через `Deps` | `Deps` не знает доменных типов; каждый потребитель грузит YAML сам, `rules_version` сверяется тестом |

---

## 11. Риски и допущения

| Риск / допущение | Реакция |
|---|---|
| inv-04 по `alive` участникам — ждёт BA (S-7) | реализуем по `alive`; если BA решит иначе — правка одного `Check` |
| Интент +40 мс на пакет группы (S-9) | замер I2-6; резерв — вариант B ADR-013 без изменения контрактов |
| `testkit/swarm.FakeEncounter` расходится с будущим агентом встречи в порядке `dice.rolled → combat.decided → proposed` | заглушка пишется по C-03 (порядок зафиксирован) и §7.1; e2e S1 EPIC-003 на реальном агенте — критерий I1 интеграции |
| Снапшот по `session.ended` требует чтения `analytics_events` (S-10) | оставлено по ADR-003; в replay — по счётчику |
| `Journal.End` на kafka под нагрузкой (high watermark) отстаёт от реального конца | `End` берётся до `ReadRange`, догон `Tail`'ом; contract-тест EPIC-001 |
| `state` и `swarm` грузят один YAML независимо — рассинхрон версий при горячем изменении файла | горячая перезагрузка правил не в MVP-1; `rules_version` в `/health` обоих сверяет тест |
| Ветка: `epic/EPIC-002-state` (`teams.md`) vs `epic/EPIC-002-state-mechanics` (задание) | tech-lead#1 фиксирует одно имя до создания |
| Допущение: `Harness` v0 достаточно для `group-3x30` до `Harness` v1 EPIC-004 | если нет — генератор `round.opened/closed` добавляется в `testkit/gateway` запросом к EPIC-004 (владелец) |

Связь: ADR-003, ADR-011, ADR-012, ADR-013, ADR-021; C-02, C-03, C-14; `state-and-mechanics.md` v0.2 §16.
