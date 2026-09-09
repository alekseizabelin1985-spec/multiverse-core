# Контракты между блоками (эпиками)

Версия 0.4 · 2026-09-09 · system-architect#1 · статус: утверждено на G2 (v0.2); v0.3 — уточнения сведения A4-2 (совместимые); **v0.4 — сведение 3 (замечания тимлидов TEAM-2/TEAM-3 при нарезке задач; все изменения совместимые, схемы v1 уточнены до начала их реализации)**, к сведению на G3.
Основание: `analysis/api-contracts.md` v0.2 (поля событий и HTTP API — здесь не дублируются, даются ссылки), `analysis/data-model.md`, ADR-001…ADR-010 (с дополнениями 2026-09-09), ADR-017 (дополнение 1), ADR-021, `plan/epics.md`, `plan/ownership.md`; запросы на изменение из `components/*.md` §14, `threat-model.md` §8, `infrastructure.md` §11 — сведены в `architecture/consolidation.md` (§1–§9 — сведение 1, §10–§13 — сведение 2, **§14 — сведение 3**).
Правило: **любое изменение контракта из этого файла — только через системного архитектора** (раздел «Запросы на изменение контракта» в `plan/ownership.md`) с оценкой влияния на все команды-потребители. Внутри эпика команда свободна.

## Изменения v0.4 (сводка; сведение 3, `consolidation.md` §14)

| Контракт | Изменение | Совместимость | Источник |
|---|---|---|---|
| C-02 | v1.2: `cause` + `forget`; переход `alive → abandoned` — **только** по предложению gateway (`set status=abandoned`, `cause=forget`, `expected_version`); `abandoned` — терминальный, правило `dead_entity` и inv-01 действуют для `status ∈ dead\|abandoned\|ascended_final`; `OwnershipRules`: строка gateway дополнена `Character.status: alive→abandoned`, `Group.leader_id` (включая `null`) | совместимо (расширение enum; новая строка таблицы владения) | З-2 (tech-lead#3), FR-061 v0.4 |
| C-04 | v1.1: `group.left`/`group.leader_changed` `{cause: forget}`; `group.leader_changed.leader: null` (живых нет); `enter`/`leave` группы без лидера → `409 no_leader`; каскад `/forget` (порядок публикаций) | совместимо (расширение enum; `leader` nullable — схема v1 уточнена до реализации T-301) | З-2, З-4, BR-13 v0.4 |
| C-06 | v1.1: `config.cloud_enabled {enabled}` публикуется контекстом `llm` **при каждом старте `core`** (и `true`, и `false`) и при изменении — чтобы проекции потребителей были детерминированы после рестарта | уточнение семантики (схема без изменений) | З-3 |
| C-07 | v1.2: `validation_status` — **единый enum из 6 значений** `valid \| partially_rejected \| invalid \| error \| quarantined \| filter_error`; причины — только в `llm.output.rejected.reason` (ADR-017 п. 3); опц. `llm.output.reasons[]` (сводка причин); `budget_exceeded` — не статус | совместимо (сужение enum до реализации T-215; удалённые значения нигде не издавались) | TL2-1 (tech-lead#2), ADR-017 доп. 1 |
| C-08 | v1.2: `GET /v1/worlds` → `worlds[].llm{cloud_enabled}` (без провайдера/URL/ключей, SEC-21); `GroupView.leader_id: string \| null`; `409 no_leader`; побочные эффекты `DELETE /v1/links` (`end_reason=forget`, `abandoned`, `group.left cause=forget`); `409 character_dead` — и для `abandoned` | совместимо (новые поля/код) | З-2, З-3, З-4 |
| C-10 | v1.1: `analytics.session.ended.end_reason` + `forget` | совместимо (расширение enum) | З-1 |
| C-14 | уточнение: `testkit/state.FakeState` v0 публикует `analytics.replay.completed {mode: recovery}` при старте; ожидание сигнала потребителями ограничено таймаутом (`MV_SWARM_REPLAY_WAIT`, по умолчанию 120 с) → `/health degraded {state_replay: missing}` | заглушка + уточнение | TL2-6 |
| §0 | правило «**тип — владелец схемы; издатели — список `Spec.Publishers` реестра**»: `dice.rolled` (EPIC-002) — издают агент встречи (EPIC-003) и `FakeEncounter`; `entity.*.proposed` (EPIC-002) — gateway, агенты, `mvctl` (bootstrap), фейки; `encounter.started` (EPIC-003) — `region-gm` и `FakeEncounter` | правило уточнено | TL2-2, TL2-5 |
| §16 | п. 6: процедура при расхождении `shared/agent/levels.go` ↔ `shared/contracts.OwnershipRules`; п. 7: издатели в реестре | процедура | TL2-3 |
| Env | единое имя allowlist — **`MV_TELEGRAM_ALLOWED_USER_IDS`** (`MV_TELEGRAM_ALLOWED_USER_IDS` в prd/threat-model — исправить, запрос BA/security); CLI на хосте (`mvctl laws bump`, `world init`, `record`) — `MV_KAFKA_BROKERS=127.0.0.1:19092` (уже в `infrastructure.md` §1 п. 8; имени `MV_KAFKA_BROKERS` нет); + `MV_SWARM_REPLAY_WAIT`, `MV_TELEGRAM_CLOUD_FLAG_TTL` | — | З-5, TL2-8, TL2-6, З-3 |

## Изменения v0.3 (сводка; сведение A4-2, `consolidation.md` §10–§12)

| Контракт | Изменение | Совместимость | Источник |
|---|---|---|---|
| C-15 | v1.1: провайдер по умолчанию `openai_compat` (llama-server, `/v1/chat/completions` + `response_format json_schema`, thinking выключается на запрос), `ollama` — второй, облако — тот же `openai_compat` с другим URL/ключом (гейт `MV_LLM_CLOUD_ENABLED` по адресу); значения `MV_LLM_PROVIDER`; поля `Request.Params`/`Response`; `Embed` через `/v1/embeddings`; потребитель `internal/memory` импортирует `llm.Provider`/`providers.Registry` (ADR-001 доп. п. 7) | совместимо (Go API расширен полями, интерфейс `Provider` не изменён) | U-8 (пользователь), ADR-005 доп. 2, EPIC-005 §12 п. 1 |
| C-05 | заглушка: `FakeNarrator` v0 (F-10) — только нарратив; Phase 1 для I1-α — **`testkit/swarm.FakeEncounter`** (EPIC-003 C4, первая единица I1a, ранний merge подпакета); `WithEncounterStub` в F-10 **не делается** | заглушки, схемы без изменений | запрос d (architect#1/#2) |
| §0 | исключение из правила «публикует только владелец типа»: фейки `shared/testkit/*` в режиме e2e/I1-α (`MV_SWARM_FAKE=true`) публикуют типы подменяемого контракта; хук в `cmd/multiverse` удаляется в I1 | правило уточнено | запрос d/e |
| C-06 | раздел `admin` `api/gateway.openapi.yaml` — совладение: файл EPIC-004, текст маршрутов `/v1/admin/agents*`, `/v1/admin/llm/usage` — EPIC-003 через PR с ревью EPIC-004 | уточнение владения | запрос f |
| C-10 | схема `analytics.consistency.violated.v1.json` (владелец EPIC-005) создаётся в F-4b блок «в» и передаётся владельцу; `mvctl trace <correlation_id>` по журналу — 005-ops (Must), `/v1/trace` — 005-memory (Should) | уточнение | запросы b, c |
| Env | + `MV_LLM_URL`, `MV_LLM_API_KEY`, `MV_SWARM_FAKE`; `MV_OLLAMA_URL` — только для провайдера `ollama` | — | U-8 |

## Изменения v0.2 (сводка)

| Контракт | Изменение | Совместимость | Источник |
|---|---|---|---|
| C-01 | + интерфейс `Journal{ReadRange, Tail, End}`, `PositionFromContext`, `eventbus.Dedup` в production-пакете; валидация схемы и при чтении (флаг); политика `player_events` (только `actor_kind ∈ human|ci|sim`, без `meta.agent`); библиотека JSON Schema 2020-12 — `santhosh-tekuri/jsonschema/v6`; «конец журнала» — через `Journal.End()`, отдельного `Bus.Lag()` нет | совместимо (новые методы/пакеты) | state §14 п. 1, gateway §14 п. 9, threat-model п. 10, infra п. 5 |
| C-02 | уточнения семантики: `rest` предлагает gateway (`cause=rest`); `atomic=false` — отказ по сущности; `applied_at`/`timestamp` факта = `timestamp` предложения; новая причина `duplicate_entity`; `changed[]` для `append` — путь элемента; `contracts.OwnershipRules` — экспорт из `shared/agent/levels.go` | совместимо (расширение enum) | state §14 п. 4, ADR-015 п. 4 |
| C-03 | совместимые дополнения Go-API: `Actor.Participation`, `Actor.LastDamager`, `Rules.Roll/Stats`, `LoadBytes`, `ActorFromEntity`, `ChangesFor`, `DiceRolledPayload`, `StateView`, `Violation`, `Invariant.Where` | совместимо | state §14 п. 3 |
| C-04 | уточнение: `say` в scope `group` не открывает раунд и не входит в `acted[]` (передано BA — FR-025) | семантика, схема без изменений | gateway §14 п. 6 |
| C-05 | + `encounter.started.round{timeout, idle_after_missed}` (опц.); + `narrative.output.narrative_event_id` (опц.) | совместимо | gateway §14 п. 4, swarm §14 |
| C-06 | HTTP-сервер процесса `core` — `MV_CORE_ADDR` (`127.0.0.1:8090`): `/health`, `/v1/admin/agents*`, `/v1/admin/llm/usage`; `tick.fired.tick.lod_allowed` обязателен | уточнение | infra п. 7, swarm §14 |
| C-07 | + `llm.output.parse{strategy, recovered}` (опц.); + `agent.spawned.content_hash` (опц.); записи для golden — только `meta.actor_kind=ci` | совместимо | swarm §14, threat-model п. 5 |
| C-08 | новые коды: `403 client_unknown`, `403 client_mismatch`, `403 actor_kind_forbidden`, `409 no_open_round`, `501 not_implemented`; `202 {status: pending, group_id}` для `group.*`; `character_status=creating`; `{client_id}` пути == `X-Client-Id`; идемпотентность `POST /v1/characters` — в пределах `link_id`; rate limit и лимит тела зафиксированы | совместимо | gateway §14 п. 1–3, threat-model п. 2, 6, 12 |
| C-11 | `trigger.event_name` допускает glob (`player.*`); валидатор проверяет модель против списка провайдера, блупринт не может включить облако | совместимо | swarm §5, threat-model п. 7 |
| C-14 | `latest.json` — указатель, не снапшот; `component ∈ state|swarm|gateway`; префикс `snapshots-{world}/gateway/` — EPIC-004; схема `analytics.replay.completed` — совладение EPIC-002/005 | уточнение | state §14 п. 5–6, gateway §14 п. 5 |
| §0 | + `shared/clock`, `shared/runtime` (EPIC-001); `shared/objstore.EnsureBucket` включает versioning/ILM сам; `objstore` не зависит от bucket versioning (ADR-021) | — | foundation §3–4, infra п. 6, ADR-021 |
| Env | все платформенные переменные — с префиксом `MV_` (`MV_LLM_PROVIDER`, `MV_GATEWAY_LISTEN`, `MV_TELEGRAM_BOT_TOKEN` …); сторонние (`OLLAMA_*`, `MINIO_ROOT_*`, `NEO4J_AUTH`, `COMPOSE_PROFILES`) — без префикса. Имена в `components/*.md` читать с префиксом | — | infra п. 4 |

## 0. Границы: кто чем владеет

| Блок (эпик) | Пакеты / процессы | Топики (издатель) | Типы событий (издатель) | Файлы-данные |
|---|---|---|---|---|
| **EPIC-001 Фундамент** | `shared/eventbus`, `shared/jsonpath`, `shared/contracts`, `shared/objstore`, `shared/env`, `shared/logging`, `shared/clock`, `shared/runtime`, `shared/testkit`, `cmd/multiverse` (каркас, HTTP-сервер процесса `/health` + монтирование `/v1/admin/*` контекстами), `build/` (включая `build/minio.Dockerfile`, `build/versions.env`), `docker-compose.yml`, `.github/workflows`, `schemas/events/_common.json` | создаёт все топики (`redpanda-init`) | — (реестр и схемы всех типов; сами типы принадлежат издателям ниже) | `.env.example`, `.mcp.env.example`, `Makefile`, `.gitattributes`, `.golangci.yml` |
| **EPIC-002 Состояние и механика** | `internal/state`, `internal/mechanics`, `internal/replay`, `shared/entity` | `system_events` (entity.*, snapshot.created), `game_events` (dice.rolled) | `entity.created`, `entity.updated`, `entity.update.rejected`, `snapshot.created` (компонент state), `dice.rolled`, `analytics.replay.completed` (mode=recovery; схема — совладение с EPIC-005, см. C-14) | `rules/dark-forest.yaml`, `schemas/events/entity.*.json`, `dice.rolled.json`, `analytics.replay.completed.v1.json` |
| **EPIC-003 Рой GM и LLM** | `internal/swarm`, `internal/llm` (+`guardian`, `prompt`, `providers`, `filter`, `parser`), `internal/laws`, `shared/agent` (типы, парсер, валидатор, `levels.go`) | `world_events`, `system_events` (tick.*, agent.*, content.incident.recorded, snapshot.created swarm, config.cloud_enabled), `game_events` (combat.decided), `narrative_output`, `llm_records` | `combat.decided`, `encounter.started/ended`, `world.weather_changed`, `world.time_advanced`, `world.event_occurred`, `region.event_occurred`, `npc.moved`, `npc.spawned`, `tick.fired`, `tick.aborted`, `agent.*`, `llm.output`, `llm.output.rejected`, `narrative.output`, `content.incident.recorded`, `config.cloud_enabled`, `world.laws.changed` (author), `world.law_breach.*` (зарегистрированы, издатель E-B), `entity.create.proposed`/`entity.update.proposed` (как издатель) | `blueprints/*.md`, `laws/*.yaml`, `schemas/agent/*.json` (tick-global, tick-region, narrative), `config/absolute-limits.yaml` |
| **EPIC-004 Вход игрока** | `internal/gateway`, `cmd/telegram-bot`, `api/gateway.openapi.yaml` | `player_events`, `game_events` (group.*, round.*), `system_events` (entity.create.proposed player/group, entity.update.proposed position/scope/group/participation, snapshot.created gateway), `analytics_events` | `player.*`, `group.*`, `round.opened/closed`, `analytics.session.started/ended`, `analytics.turn.completed`, `snapshot.created` (component=gateway), `entity.*.proposed` (как издатель) | миграции SQLite `internal/gateway/migrations/{links,gateway}/*.sql` |
| **EPIC-005 Память и операции** | `internal/memory`, `cmd/mvctl`, `api/memory.openapi.yaml`, `ops/metrics/` | `analytics_events` (consistency.violated, replay.completed mode=test) | `analytics.consistency.violated`, `analytics.replay.completed` (mode=test) | `ops/metrics/*.csv`, `ops/metrics/baseline.md`, `testdata/recordings/*.jsonl` |
| Целевые EPIC-006…013 | `internal/ext/<domain>`, новые блупринты | по эпику | по эпику; используют только контракты ниже | — |

Правило потребления: любой блок может **читать** любой топик; **публиковать** тип может только владелец типа. Политика `player_events`: публикуются только с `meta.actor_kind ∈ human|ci|sim` и без `meta.agent` — проверяется библиотекой при публикации и при чтении (C-01).
**Уточнение v0.4 (TL2-2, TL2-5): «владелец типа» = владелец схемы и семантики; фактические издатели — список `Spec.Publishers` в реестре `shared/contracts` (значения `source` конверта по реестру EPIC-001; фейки — префикс `testkit/`).** Владелец типа обязан перечислить всех издателей; job `contracts` проверяет `source ∈ Spec.Publishers`, а не «издатель == владелец». Типы с несколькими издателями в MVP-1: `dice.rolled` (владелец EPIC-002; издатели — агент встречи `swarm` EPIC-003 по C-03 и `testkit/swarm.FakeEncounter`), `entity.create.proposed`/`entity.update.proposed` (владелец EPIC-002; издатели — `gateway`, `swarm`, `mvctl` при `world init --fixtures`, фейки), `encounter.started` (владелец EPIC-003; издатели — `region-gm` и `FakeEncounter`), `analytics.replay.completed` (EPIC-002 `state`, EPIC-005 харнесс, `testkit/state.FakeState` — см. C-14), `snapshot.created` (`state`, `swarm`, `gateway`). Источники `testkit/*` допустимы только на `membus` и в `core` при `MV_SWARM_FAKE=true` — это и есть «исключение для заглушек» ниже.
**Исключение для заглушек (v0.3)**: фейк контракта в `shared/testkit/<владелец>` публикует типы **подменяемого** контракта (например, `testkit/swarm.FakeEncounter` — `encounter.*`, `combat.decided`, `entity.*.proposed`, а также `dice.rolled` EPIC-002, который по C-03 и так издаёт агент встречи) — только в e2e на `membus` и в процессе `core` при `MV_SWARM_FAKE=true` (хук `cmd/multiverse`, ADR-001 доп. п. 8). Фейк пишет `meta.agent.blueprint` с префиксом `fake-`; хук в `core` удаляется при слиянии EPIC-003 I1 (тег `mvp-1/i1`); production-код контекстов фейки не импортирует.

## 1. C-01 · EPIC-001 → все: конверт события, реестр типов, библиотека шины и журнала

Тип: общая модель + библиотека · Версия: 1 (v1.1 — дополнения 2026-09-09) · Владелец: EPIC-001 (после — system-architect)

### Интерфейс
- `eventbus.Event{ID, Type, Timestamp, Source, World *WorldRef, Scope *ScopeRef, Meta Meta, Payload map[string]any, Relations []Relation}`; `Meta{SchemaVersion int, CorrelationID, CausationID, CausationType string, ActorKind string, Agent *AgentRef, Replay bool, Locale string, GMPath string}` (ADR-007 п. 1).
- Конструкторы: `NewRoot(type, source, worldID string, scope *ScopeRef, actorKind string, payload map[string]any) Event`; `Derive(parent Event, type, source string, payload map[string]any, opts ...DeriveOption) Event` (копирует `Meta.CorrelationID`, `ActorKind`, `Locale`, `GMPath`; ставит `CausationID = parent.ID`; `WithAgent(AgentRef)`).
- Шина: `Bus{Publish(ctx, Event) error; Subscribe(ctx, topic, group string, h Handler) error; Close() error}` — топик выбирается реестром по типу; `Handler func(ctx, Event) error` — ошибка → повтор ×3 (100/500/2000 мс) → `dead_letters`.
- **Журнал (новое, v1.1)**: `Journal{ReadRange(ctx, topic string, from, to int64, h Handler) (next int64, err error); Tail(ctx, topic string, from int64, h Handler) error; End(ctx, topic string) (int64, error)}` — чтение по офсетам без consumer group (State — свой курсор, ADR-011 п. 4; Swarm и Gateway — догон с курсора снапшота, C-14). `End` = офсет следующего сообщения (high watermark): признак «конца журнала» для выхода из `replay` в `live` = `позиция ≥ End − 1` по всем читаемым топикам; отдельного `Bus.Lag()` **нет** (запрос gateway §14 п. 9 закрыт этим методом). `Position{Topic, Offset}`; `PositionFromContext(ctx) (Position, bool)` — адаптер кладёт позицию перед вызовом handler (и в `Subscribe`, и в `Journal`). Реализации: kafka-go (`SetOffset`/`ReadLastOffset`), `membus` (индексы очередей) — одна семантика, общий contract-тест.
- **Дедуп (новое)**: `eventbus.Dedup` (LRU по `event.id`, по умолчанию 10 000; `Seen(id) bool`; сериализуется в снапшот потребителя) — production-пакет; `testkit.Dedup` — псевдоним.
- Реестр: `contracts.Lookup(type) (Spec{Topic, SchemaVersion, Schema, Policy}, bool)`; `contracts.Validate(Event) error`; `contracts.OwnershipRules` (см. C-02); список типов и топиков — ADR-007 п. 4; схемы — `schemas/events/<type>.v<n>.json`, JSON Schema 2020-12, библиотека **`github.com/santhosh-tekuri/jsonschema/v6`** (as-is `xeipuuv/gojsonschema` — только draft-07, не переносится).
- **Валидация при чтении (новое, SEC-16)**: `Subscribe` валидирует конверт и payload по схеме с флагом `MV_BUS_VALIDATE_ON_READ=true` (по умолчанию включено; невалидное → `dead_letters` без вызова handler). Политики топика (`Spec.Policy`): `player_events` — `actor_kind ∈ human|ci|sim`, `meta.agent == nil`; `llm_records`, `system_events(tick.*, agent.*)` — `meta.agent` обязателен для типов роя.
- Чтение payload: `event.Path()` (`shared/jsonpath`) без изменений; `eventbus.GetWorldIDFromEvent`, `GetScopeFromEvent` без изменений.
- Часы и рантайм (foundation §3–4): `shared/clock.Clock{Now()}`, `Timers{After, Every}`; `shared/runtime.Deps{Bus, Journal, Store, Clock, Timers, Mode, IDs, Log, Env, Contracts}`, `runtime.Context{Name, DependsOn, Start, Stop, Health}`. `time.Now()` в `internal/*` запрещён линтером; `EventClock`/`NullTimers` для replay — `internal/replay` (EPIC-002), внедряет `cmd/multiverse`.

### Гарантии
At-least-once; порядок внутри топика (одна партиция); публикация неизвестного/невалидного типа → ошибка (не публикуется); reader `MinBytes=1, MaxWait≤100ms`; дедуп по `Event.ID` — обязанность потребителя (`eventbus.Dedup`); `Journal.ReadRange` возвращает события строго по возрастанию офсета; `End` монотонен.

### Заглушка для потребителей
`shared/testkit/membus` — in-memory шина **и журнал** с той же семантикой (порядок, офсеты, дубли по флагу `--chaos=duplicate`, `End`); `contracts` со схемами доступен с первой волны EPIC-001 — команды пишут свои схемы в `schemas/events/` (владелец типа) и регистрируют через PR в `shared/contracts/registry.go` (ревью system-architect). Contract-тест шины (F-5t) гоняется против `membus` и kafka-адаптера (testcontainers) до старта волны 1.

### История изменений
v1 — 2026-09-09: создан (ADR-007). v1.1 — 2026-09-09: `Journal`, `Position`, `Dedup`, валидация при чтении и политики топиков, библиотека схем, `clock`/`runtime` (сведение A3 шаг 4).

## 2. C-02 · EPIC-002 → EPIC-003, EPIC-004: предложения и факты состояния

Тип: события · Версия: 1 (v1.1 — уточнения семантики) · Владелец: EPIC-002

### Интерфейс
- Вход State: `entity.create.proposed`, `entity.update.proposed` — payload по `api-contracts.md` §2.3.4 (`proposal_id`, `changes[]{entity, expected_version?, ops[]{op: set|inc|append|remove, path, value}}`, `atomic`, `cause`); `meta.agent` обязателен от агентов; `meta.actor_kind` наследуется.
- Выход State: `entity.created {entity, version:1, attributes}`, `entity.updated {entity, version, changed[]{path, old, new}, cause, proposal_id, applied_at}` (по одному на сущность), `entity.update.rejected {proposal_id, reason, entity?, details{expected_version, actual_version, invariant_id}}`; `reason ∈ version_conflict | unknown_entity | level_violation | law_violation | invalid_op | dead_entity | duplicate_entity` (`duplicate_entity` — для `entity.create.proposed` с уже существующим id; расширение enum, совместимо).
- **Семантика (v1.1)**: `rest` предлагает gateway (`cause=rest`, `set hp = hp_max`), State отклоняет при активной встрече (`law_violation inv-…`); `atomic=false` — отказ по каждой сущности отдельно (по одному `rejected`), `atomic=true` — один `rejected` на пакет; `applied_at` и `timestamp` фактов = `timestamp` предложения (время из событий, ADR-003 п. 6); для `append` путь в `changed[]` — путь элемента (`inventory[n]`); `expected_version` обязателен для путей `hp`, `status`, `inventory`, `position` игрока/NPC в бою (ADR-013 п. 1).
- Правило владения (кто что может предлагать) — `data-model.md` §4; проверка `level_violation` по `meta.agent.level` и `owned_entity_types`; **`contracts.OwnershipRules`** — единственный источник: таблица уровней экспортируется из `shared/agent/levels.go` (EPIC-003) в `shared/contracts` (EPIC-001) при сборке реестра; State читает только `contracts.OwnershipRules`. Процедура при расхождении копии и истины — §16 п. 6.
- **Покинутый персонаж (v1.2, З-2, FR-061)**: `cause` дополнен значением `forget`. Переход `status: alive → abandoned` предлагает **только gateway** (`entity.update.proposed {changes: [{entity: player, expected_version, ops: [{op: set, path: status, value: abandoned}]}], atomic: true, cause: forget}`, без `meta.agent`, `meta.actor_kind` сессии) в каскаде `/forget` (C-04); строка gateway в `OwnershipRules` дополнена `Character.status → abandoned` и `Group.leader_id`. State: предложение от агента (`meta.agent` есть) → `level_violation`; над `dead`/`ascended_final` → `dead_entity` (терминальные не переходят в `abandoned`; `dead` уже исключён из scope и целей — FR-023); `abandoned` — терминальный статус: правило `dead_entity`, inv-01 (`dead_does_not_act`), `NPCTarget` и таблица видимости стража трактуют `status ∈ dead|abandoned|ascended_final` одинаково (C-03: `Actor.Status ≠ alive` исключается из целей). Факт — `entity.updated {changed: [{path: status, old: alive, new: abandoned}], cause: forget}`; потребители (swarm, gateway read-model, memory) обрабатывают его как `status=dead` в части scope/целей/раундов, но `narrative.output kind=death` **не** генерируется. Если персонаж в момент `/forget` в статусе `creating`, gateway публикует то же предложение при получении `entity.created` для `player_id` без связки.
- Read-model: потребители строят проекции из `entity.created/updated`; прямого HTTP к State нет. Начальная загрузка проекции — чтение указателя `snapshots-{world}/state/latest.json`, затем объекта снапшота через `shared/objstore` (только чтение; C-14).

### Гарантии
Факт публикуется после успешной записи в MinIO; `version` строго +1 на сущность при непустом `changed[]` (при `changed=[]` версия не меняется, факт публикуется — ход засчитан); `atomic=true` — все или ничего; повтор предложения с тем же `proposal_id` не применяется второй раз (факты досылаются, если не были опубликованы); латентность применения p95 ≤ 50 мс при ≤ 50 сущностей/scope (без учёта шины).

### Заглушка для потребителей
`testkit/state.FakeState` — применяет предложения в память и публикует факты в `membus` без инвариантов (флаг `WithInvariants()` включает набор inv-01…inv-10).

### История изменений
v1 — 2026-09-09: по `api-contracts.md` §2.3.4 (ADR-003). v1.1 — 2026-09-09: семантика `rest`/`atomic=false`/`applied_at`/`append`, `duplicate_entity`, источник `OwnershipRules`. v1.2 — 2026-09-09 (сведение 3): `cause=forget`, переход `alive → abandoned` от gateway, `abandoned` терминальный, строка gateway в `OwnershipRules`.

## 3. C-03 · EPIC-002 → EPIC-003: библиотека механики и RNG (Go-контракт)

Тип: общая модель (Go API) · Версия: 1 (v1.1 — совместимые дополнения) · Владелец: EPIC-002

### Интерфейс
```go
package mechanics
type Rules struct{ … }                                   // загружено из rules/dark-forest.yaml (RulesDocument, data-model §6.3)
func Load(path string) (*Rules, error)
func LoadBytes(b []byte) (*Rules, error)                                 // v1.1
type Actor struct{ ID, Type string; HP, HPMax, Atk, Def int; Dmg, Flee string; Status string
                   Participation string /* active|idle|out_of_combat */; LastDamager string }   // v1.1: Participation, LastDamager
type Action struct{ Kind string /* attack|flee|npc_attack|rest|free_attack */; Actor, Target string }
type Outcome struct{ Hit, Critical, Fumble, TargetDead bool; Natural, Damage int; Success *bool; Threshold int; HPBefore, HPAfter int; Loot []Item }
type Roll struct{ Index int; Formula string; Seed uint64; Result, Natural int; Purpose string }
func (r *Rules) Resolve(causeEventID string, rollIndexStart int, a Action, actors map[string]*Actor) (Outcome, []Roll, error)
func (r *Rules) NPCTarget(npc *Actor, candidates []*Actor) *Actor      // last_damager → min_hp → player_id asc; исключая idle/out_of_combat/dead
func (r *Rules) Roll(causeEventID string, idx int, formula, purpose string) Roll   // v1.1: броски вне боя (шанс встречи, таблицы фона)
func (r *Rules) Stats(kind string) (Actor, bool)                        // v1.1: базовые статы NPC из rules
func Seed(eventID string, rollIndex int) uint64                          // SHA-256(eventID+":"+idx)[:8] BigEndian
func NewRNG(seed uint64) *rand.Rand                                      // math/rand/v2 PCG(seed, 0)
func (r *Rules) Invariants() []Invariant                                 // NFR-020 inv-01…inv-10; Invariant{ID, Where []string, Check func(StateView, touched) []Violation}  // v1.1: Where, StateView, Violation
func ActorFromEntity(e entity.Entity) (Actor, error)                     // v1.1
func ChangesFor(o Outcome, a Action, actors map[string]*Actor, causeEventID string) []entity.Change   // v1.1: ops для entity.update.proposed
func DiceRolledPayload(roll Roll, roller string) map[string]any          // v1.1: payload dice.rolled
```
Вызывающий (агент встречи EPIC-003) публикует `dice.rolled` на каждый `Roll` (тип принадлежит EPIC-002, схема общая) через `Derive(cause, "dice.rolled", DiceRolledPayload(...))` **до** `combat.decided`, затем `combat.decided` (тип EPIC-003, §2.3.6) и `entity.update.proposed` из `ChangesFor` (рекомендация: один atomic-пакет на раунд; допускается по одному предложению на `combat.decided`).

### Гарантии
Чистые функции, без I/O и часов; детерминизм по `(causeEventID, rollIndex)`; `rules_version` в `Rules`; изменение чисел правил — правка YAML, не кода; сигнатуры v1 не меняются.

### Заглушка
`rules/dark-forest.yaml` с числами приложения A PRD доступен с первой волны EPIC-002; до готовности `Resolve` EPIC-003 использует `testkit/state.FixedMechanics` (табличные исходы по seed).

### История изменений
v1 — 2026-09-09 (ADR-003). v1.1 — 2026-09-09: дополнения по ADR-012 (state §14 п. 3).

## 4. C-04 · EPIC-004 → EPIC-003, EPIC-002: действия игрока, группа, раунд

Тип: события · Версия: 1 · Владелец: EPIC-004

### Интерфейс
`player.entered_region`, `player.left_region`, `player.looked`, `player.attacked`, `player.flee_attempted`, `player.rested`, `player.said`, `player.defended {cause: player|round_timeout}` — payload §2.3.1; `group.created/joined/left/leader_changed/disbanded`, `group.entered_region/left_region` — §2.3.2 (перемещение группы — **только** `group.entered_region`, ADR-007/overview §20 п. 7); `round.opened {scope, encounter, round{seq}, expected[], deadline_at}`, `round.closed {round{seq, close_reason}, acted[], auto_defended[], idle[], closed_at}` — §2.3.3. Все — `meta.correlation_id = id` (корень), `meta.actor_kind` сессии, scope игрока/группы.
Сопутствующие предложения от gateway: `entity.create.proposed` (player, group), `entity.update.proposed` (position, scope, group_id, members, `members[].participation`) — по C-02; `rest` — `entity.update.proposed cause=rest` (C-02 v1.1).
**Семантика раунда (уточнение)**: открывающие действия — `attack | flee | defend | group.leave`; `say` в scope `group` **не** открывает раунд и не входит в `acted[]` (UC-016 E2; BA закрепляет в FR-025); `group.leader_changed {cause: death}` при смерти лидера — старейший `alive` по `joined_at` (предложение ADR-020 п. 8, BA — BR-13).
**Лидерство и группа без лидера (v1.1, З-4, BR-13 v0.4)**: `cause` в `group.left`/`group.leader_changed` ∈ `leave | death | forget`; если после выхода/смерти/отвязки лидера живых (`alive`) участников нет — gateway предлагает `set leader_id = null` (`entity.update.proposed`, тот же atomic-пакет) и публикует `group.leader_changed {leader: null, cause}`; группа остаётся без лидера до `group.disbanded`; `enter`/`leave` группы (перемещение) при `leader_id = null` → `409 no_leader` (проверяется раньше `not_leader`); личный `group.leave` участника допустим. `dead`/`abandoned` участники остаются в `members[]` (история), но исключаются из `expected[]`/`acted[]`, целей NPC и `participation=active` (координатор — `OnParticipantsChanged` по `entity.updated status`).
**Каскад `/forget` (v1.1, З-2, FR-061 п. 1–3; реализует `ForgetHooks`, порядок публикаций обязателен)**: (1) `outbox.DropForPlayer` (`pending → dropped`); (2) если персонаж `alive`: один `entity.update.proposed atomic=true cause=forget` — игрок `set status=abandoned` (+`expected_version`), а если он лидер группы — группа `set leader_id = <старейший alive | null>`; затем, если в группе, — `group.left {cause: forget}` и при смене лидера `group.leader_changed {cause: forget, leader: …|null}`; если участников-`alive` не осталось — по BR-13 (без лидера; `group.disbanded` — при выходе последнего участника); во встрече gateway ничего дополнительно не делает — агент встречи видит `entity.updated status=abandoned` и завершает встречу `players_out`, если игроков `alive` нет; (3) `session.End(forget)` → `analytics.session.ended {end_reason: forget}` (C-10); (4) физическое удаление связки (`links.db`). Персонаж `dead` — шаги (2) пропускаются (терминальный статус, FR-023); `creating` — см. C-02 v1.2.

### Гарантии
Действие опубликовано только после валидации (§1.4) и идемпотентности `action_key`; `round.opened` публикуется до `202` на открывающее действие; `round.closed` публикуется до любых действий по раунду и содержит всё, что нужно механике (порядок `acted[]` — по времени приёма); `player.defended cause=round_timeout` публикуются до `round.closed`; в `solo` раунд = ход и `round.*` **не** публикуются; `player.said.text ≤ 500`, без внешних ID; `player_events` — только `actor_kind ∈ human|ci|sim`, без `meta.agent` (политика C-01).

### Заглушка
`testkit/gateway.Harness` — Go-клиент HTTP API и генератор `player.*` в `membus` для тестов EPIC-003/002 до готовности gateway; фикстуры `player-A/B/C`.

### История изменений
v1 — 2026-09-09. Уточнения 2026-09-09: `say` и раунд, лидерство при смерти (без изменения схем). v1.1 — 2026-09-09 (сведение 3): `cause=forget`, `leader: null`, `409 no_leader`, каскад `/forget` (расширение enum и nullable-поле до реализации схем в T-301).

## 5. C-05 · EPIC-003 → EPIC-004: нарратив и результаты механики для доставки

Тип: события · Версия: 1 (v1.1 — опциональные поля) · Владелец: EPIC-003

### Интерфейс
`narrative.output` — §2.3.11 (`recipients[]`, `text`, `generated_by: llm|template`, `fallback_reason?`, `kind: turn|round|world_event|entry|death`, `round?`, `llm_output{event}`, `based_on[]`, `absence?`, `background_refs[]`, `filter{applied,status,filter_version}`, `locale`, `laws_version`, **`narrative_event_id?`** (= `id` события; дубль для удобства gateway, v1.1)); `meta.agent.level ∈ task` (personal-gm или group-narrator). `combat.decided` — §2.3.6 (gateway строит `Delivery kind=mechanics` из `outcome`/`hp`). `encounter.started/ended` — §2.3.7; **`encounter.started.round{timeout: "60s", idle_after_missed: 2}`** (опц., v1.1) — параметры раунда из блупринта `encounter-*` (`round` в `rules/dark-forest.yaml`); gateway использует их для таймера раунда, при отсутствии — `MV_GATEWAY_ROUND_TIMEOUT`/`MV_GATEWAY_ROUND_IDLE_AFTER_MISSED` как значения по умолчанию. Gateway обновляет read-model `encounter` и открывает первый раунд группы по `encounter.started`.

### Гарантии
Один `narrative.output` на ход соло и один на раунд группы (`narrative_event_id` общий); `recipients[]` — только игроки scope (включая `idle`); текст прошёл фильтр (a) (`filter.status=pass`) или заменён шаблоном; p95 от `combat.decided` до `narrative.output` — порог после замера (NFR-002); при недоступности LLM — `generated_by=template` в пределах таймаута фазы; текст не содержит разметки, требующей `parse_mode` (бот отправляет как plain text, SEC-10).

### Заглушка
Две заглушки в `shared/testkit/swarm` (владелец — EPIC-003; v0.3, сведение A4-2 запрос d):
- **`FakeNarrator`** — `narrative.output generated_by=template` на `player.entered_region` (`entry`), `player.looked` (`turn`), `encounter.started` (`world_event`), последний `combat.decided` цикла (`turn`), `entity.updated(player) status=dead` (`death`), `round.closed` (`round`); шаблоны — `shared/testkit/swarm/template/ru.go` (переиспользуются ролями роя как шаблоны деградации). **v0 создаёт EPIC-001 F-10 — только нарратив, без боя** (`WithEncounterStub` не делается); реализацию заменяет EPIC-003 C4.
- **`FakeEncounter`** — заглушка Phase 1 для I1-α: на `player.entered_region` — `entity.create.proposed encounter` + `encounter.started{round{60s,2}}`; на `player.attacked/flee_attempted` — `mechanics.Rules.Resolve` (`FixedMechanics` до EPIC-002 I1) → `dice.rolled` ×k → `combat.decided` ×2 → один `entity.update.proposed atomic` с `expected_version` → `encounter.ended`. **Делает EPIC-003 как первую единицу I1a (C4)**; подпакет `shared/testkit/swarm` вливается в `integration/mvp-1` сразу после приёмки tech-lead#2 (ранний merge, до I1-α) — не дожидаясь остального I1a. В `core` монтируется хуком `MV_SWARM_FAKE=true` (ADR-001 доп. п. 8). Запасной вариант, если C4 не готов к 2-й неделе волны 1: tech-lead#1 эскалирует system-architect; допускается временный `encounterStub` **внутри тестов EPIC-002** (`internal/state/e2e_test.go`, не в `shared/testkit`), удаляемый при появлении `FakeEncounter`.

### История изменений
v1 — 2026-09-09. v1.1 — 2026-09-09: `encounter.started.round{}`, `narrative_event_id`. v0.3 контрактов — 2026-09-09: заглушки `FakeNarrator`/`FakeEncounter` (схемы без изменений).

## 6. C-06 · EPIC-003 → EPIC-004, EPIC-005: жизненный цикл роя, тики, admin-HTTP процесса `core`

Тип: события + HTTP · Версия: 1 (уточнение порта/адреса) · Владелец: EPIC-003 (события), EPIC-001 (HTTP-сервер процесса)

### Интерфейс
`tick.fired {agent, scope, tick{seq, mode, scheduled_at, fired_at, lod_allowed}, budget{window_calls, cap}}` — **`tick.lod_allowed` обязателен** (агент не пересчитывает, ADR-014 п. 6); `tick.aborted`, `agent.spawned/child_resolved/stopped/spawn_rejected/blueprint_reloaded` — §2.3.9. Служебные HTTP: `POST /v1/admin/agents/{agent_id}/tick`, `GET /v1/admin/agents` (api-contracts §1.8), `GET /v1/admin/llm/usage` (ADR-005 п. 3, Should) — реализуют контексты `swarm`/`llm`, монтируя маршруты на **HTTP-сервере процесса** (`shared/runtime`, EPIC-001): адрес `MV_CORE_ADDR` (по умолчанию `127.0.0.1:8090`; в compose — `core:8090`, наружу `127.0.0.1:8090`); там же `/health` процесса (`agents_by_level`, `llm`, `store`). Gateway проксирует `/v1/admin/*` → `MV_CORE_URL` (`http://core:8090`) для единого входа харнесса; доступ — только клиенты `ci`/operator (ADR-009 п. 9). Спецификация admin-маршрутов — раздел `admin` в `api/gateway.openapi.yaml` (отдельный `core.openapi.yaml` не заводится); **совладение (v0.3, запрос f)**: файл — EPIC-004, текст маршрутов `/v1/admin/agents*` и `/v1/admin/llm/usage` — EPIC-003 через PR в файл с ревью EPIC-004 (тест OpenAPI ↔ маршруты у EPIC-004 включает admin-раздел через прокси).

### Гарантии
`tick.fired` публикуется до выполнения тика; `tick_seq` монотонен на агента; немедленный тик по admin-запросу записывается тем же `tick.fired` с `mode=background` и `meta.actor_kind=ci`; в `--mode=replay` планировщик не издаёт тиков, `FireNow` доступен харнессу.
**`config.cloud_enabled` (v1.1, З-3)**: контекст `llm` публикует `config.cloud_enabled {enabled: bool, provider?, allow_external_players?}` (§2.3.16) **при каждом старте `core`** — и при `enabled=false` (раньше — только при включении), и при изменении флага в рантайме; `provider` — только имя из `MV_LLM_PROVIDER`, без URL и ключей (SEC-21). Потребитель (gateway) хранит последнее значение по офсету в проекции и снапшоте `gateway/`, по умолчанию `false`; наружу отдаёт только `cloud_enabled` (C-08 v1.2).

### Заглушка
До готовности EPIC-003 харнесс EPIC-005 генерирует `tick.fired` напрямую в `membus` (только для тестов отчёта).

### История изменений
v1 — 2026-09-09. Уточнение 2026-09-09: адрес `MV_CORE_ADDR=127.0.0.1:8090` (замечание infra п. 7; порт 8090 сохранён по трём дизайнам, devops правит `infrastructure.md` §1.4), `lod_allowed` обязателен. v1.1 — 2026-09-09 (сведение 3): `config.cloud_enabled` при каждом старте и при изменении.

## 7. C-07 · EPIC-003 → EPIC-005, EPIC-002 (replay): записи LLM

Тип: события · Версия: 1 (v1.1 — опциональные поля) · Владелец: EPIC-003

### Интерфейс
`llm.output` — поля `data-model.md` §7.2 (`provider, model, params, prompt_hash, response_raw?, response_hash, validation_status, laws_version, phase, attempt, latency_ms, tokens, cost_usd, lod?, tools_used[]?, error?`; **`parse{strategy, recovered}`** опц. (ADR-016 п. 1, v1.1); `correlation_id`, `actor_kind`, `agent`, `replay`, `gm_path` — в `meta`); `llm.output.rejected` — §2.3.10, `reason` enum по ADR-017 п. 3; `content.incident.recorded` — §2.3.15; `agent.spawned` + `content_hash` (опц.). Ключ записи для replay: `(meta.correlation_id, meta.agent.id, phase, attempt)`.
**`validation_status` — единый enum (v1.2, TL2-1, ADR-017 доп. 1)**: `valid` (применён целиком) · `partially_rejected` (≥ 1 элемент отброшен стражем, остаток применён) · `invalid` (ответ не использован: `schema_invalid`, `language`, устаревшая `laws_version`, все элементы отброшены → повтор или шаблон) · `error` (ответа нет: провайдер, таймаут, `error.code=yielded` — ADR-014 п. 2) · `quarantined` (фильтр (a) `block`, без `response_raw`, + `content.incident.recorded`) · `filter_error` (фильтр упал, fail-closed, без `response_raw`). **Причины** — только в `llm.output.rejected.reason` (одно событие на отброшенный элемент; без `element` — на весь ответ); `budget_exceeded` — `llm.output.rejected` **без** `llm.output` (вызова не было) и не является статусом. Значения `rejected_unknown_entity | rejected_player_agency | rejected_level_violation | rejected_language | budget_exceeded` из `data-model.md` §7.2 v0.2 **удалены** (никогда не издавались). Опц. `reasons[]` в `llm.output` — множество `reason` связанных `rejected` (денормализация для `mvctl llm-usage`/отчётов без join; заполняется шлюзом, т. к. страж вычисляется до записи).

### Гарантии
`llm.output` записан **до** использования ответа; при `quarantined`/`filter_error` — без `response_raw`; `tokens` — из ответа провайдера (погрешность ≤ 5 %); в `--mode=replay` новых `llm.output` не издаётся (`meta.replay=true` у прочитанных при пробросе потребителям в тестовом режиме). **Записи для `testdata/recordings` и golden (SEC-23)**: `mvctl record` принимает только сессии с `meta.actor_kind=ci` и фикстурных игроков; запись с `actor_kind=human` отклоняется; CI `privacy-scan` сканирует `testdata/`.

### Заглушка
`testkit/swarm.RecordingWriter` — пишет `llm.output` из `providers/fake` для генерации тестовых записей.

### История изменений
v1 — 2026-09-09. v1.1 — 2026-09-09: `parse{}`, `yielded`, `content_hash`, ограничение записей `actor_kind=ci`. v1.2 — 2026-09-09 (сведение 3): единый `validation_status` (6 значений), `reasons[]` опц.

## 8. C-08 · EPIC-004 (gateway) → telegram-bot, CI-харнесс, будущий Discord-бот: HTTP API v1

Тип: HTTP API · Версия: `/v1` (v1.1 — совместимые дополнения) · Владелец: EPIC-004 · Спецификация: `api/gateway.openapi.yaml` = `api-contracts.md` §1 + дополнения ниже (заголовки `X-Client-Id`, `X-Actor-Kind`; `action_key`; коды §1.6; `Delivery` §1.5; служебные §1.8; раздел `admin` — прокси C-06).

### Дополнения v1.1
- **Коды ошибок**: `403 client_unknown` (нет `X-Client-Id` в `MV_GATEWAY_CLIENTS`), `403 client_mismatch` (`{client_id}` пути ≠ `X-Client-Id`, SEC-12), `403 actor_kind_forbidden`, `409 no_open_round` (`rounds/close` без открытого раунда), `409 already_acted`, `429 rate_limited` (+`Retry-After`), `501 not_implemented` (`/stream`).
- **`POST …/actions` для `group.*`**: при ожидании факта > `MV_GATEWAY_FACT_WAIT` (2 с) — `202 {status: "pending", group_id, correlation_id}`; клиент опрашивает `GET /v1/groups/{id}`.
- **`POST /v1/links/resolve`** → `character_status ∈ none | creating | alive | dead` (`creating` — между `entity.create.proposed` и `entity.created`).
- **Идемпотентность `POST /v1/characters`** — в пределах **`link_id`** (ULID, присваивается связке при `links/resolve`), а не внешнего ID; таблица `character_requests` — в `links.db` (ключ `link_id + action_key`), в `gateway.db` внешних ID нет ни в одном поле (SEC-03; ADR-019 п. 4). `route.external_id` в `Delivery` выдаётся только клиенту той же платформы, что связка.
- **Лимиты (SEC-11)**: тело ≤ 64 КиБ (`413 payload_too_large`); rate limit `POST …/actions` — 30 действий/мин на `player_id` (token bucket `MV_GATEWAY_RATE_ACTIONS_PER_MIN=30`, burst 5); один активный long-poll на `client_id` (второй → `409 poll_in_progress`); `wait_ms ≤ 25000`, `limit ≤ 100`. Бот дополнительно: 20 команд/мин на Telegram user id (в памяти бота).

### Дополнения v1.2 (сведение 3)
- **Флаг облака (З-3, US-008, SEC-21)**: `GET /v1/worlds` → `worlds[].llm: {cloud_enabled: bool}` — единственное место; источник — проекция gateway последнего `config.cloud_enabled.enabled` из `system_events` (C-06 v1.1), по умолчанию `false`, хранится в снапшоте `gateway/`. Имя провайдера, URL, ключи клиенту **не** передаются; текст уведомления бота — общий («тексты передаются облачной модели»), без имени провайдера (BA — уточнить US-008). Бот: кэш флага с TTL `MV_TELEGRAM_CLOUD_FLAG_TTL=60s`, обновление при `/start`, `/help` и по истечении TTL; при `true` — отдельное уведомление один раз за сессию диалога до обработки команды. Отдельного эндпоинта, поля в `GET /v1/players/{id}` и `Delivery kind=system` не вводим (один источник, без дублей).
- **Группа без лидера (З-4)**: `GroupView.leader_id: string | null` (в `GET /v1/groups/{id}` и в `CharacterState.group`); `409 no_leader` — `enter`/`leave` группы при `leader_id = null` (BR-13); проверяется раньше `409 not_leader`.
- **`/forget` (З-1, З-2)**: побочные эффекты `DELETE /v1/links` — по каскаду C-04 v1.1: outbox `dropped`, `entity.update.proposed status=abandoned cause=forget` (если `alive`), `group.left {cause: forget}`, `analytics.session.ended {end_reason: forget}`, затем физическое удаление связки. `409 character_dead` возвращается и для `status=abandoned` (действия по `player_id` от `ci`/`sim` или по устаревшей сессии); `POST /v1/links/resolve` после `/forget` → `character_status=none` (связки нет).

### Гарантии
`202` ≤ 300 мс p95; повтор `action_key` → тот же `correlation_id` (и тот же `4xx`, если был); `503 bus_unavailable` — действие не принято, ключ не записан; доставки строго по порядку на `player_id` (одна в лизинге), повтор без ack через 30 с; long-poll `wait_ms ≤ 25000`; WS-стрим зарезервирован, в MVP-1 `501`.

### Заглушка
`testkit/gateway.FakeGateway` (in-process HTTP) для разработки бота до готовности gateway; бот и харнесс используют один Go-клиент `internal/gateway/client`.

### История изменений
v1 — 2026-09-09 (ADR-006). v1.1 — 2026-09-09: коды, `pending`, `creating`, `link_id`, лимиты (gateway §14 п. 1–3, threat-model п. 2/6/12). v1.2 — 2026-09-09 (сведение 3): `worlds[].llm.cloud_enabled`, `leader_id` nullable, `409 no_leader`, побочные эффекты `/forget`.

## 9. C-09 · EPIC-005 → EPIC-003: контекст памяти (HTTP) и деградация

Тип: HTTP API · Версия: `/v1` · Владелец: EPIC-005 · Спецификация: `api/memory.openapi.yaml`.

### Интерфейс
- `POST /v1/context/scope {world_id, scope{id,type}, entity_ids[], since?, limit}` → `{facts[]{text, source: authored|validated|generated, layer, event{id,type}, at}, entities[]{…}}`;
- `POST /v1/context/absence {world_id, region_id, player_id, since_at, until_at}` → `{events[]{event{id,type}, summary, at, actor_kind}, count}` (сводка фона FR-127);
- `GET /v1/trace/{correlation_id}` → цепочка событий;
- `GET /health`.
Go-интерфейс в Swarm: `MemoryClient{ScopeContext, AbsenceSummary, Trace}`.
**Инъекция второго порядка (SEC-17)**: факты `source=generated` Swarm вставляет в промпт в секции `<memory>` как данные с тем же экранированием и лимитом длины, что `<player_text>`; e2e `injections-10` включает сценарии через память.

### Гарантии
Проекция eventually consistent (задержка индексации ≤ 2 с); при ошибке/таймауте (500 мс) Swarm переходит к `journalContext` (состояние + окно журнала) без ошибки игроку; ответы без внешних ID.

### Заглушка
`internal/swarm/context/journal.go` — **штатная** деградация, не тестовая: работает без memory; EPIC-003 не блокируется EPIC-005.

### История изменений
v1 — 2026-09-09 (ADR-004 п. 5). Уточнение 2026-09-09: экранирование `generated`-фактов.

## 10. C-10 · EPIC-004 → EPIC-005: события аналитики

Тип: события · Версия: 1 · Владелец: EPIC-004 (издатель), схема — `metrics.md` §4.2 без изменений (`analytics.session.started/ended`, `analytics.turn.completed`); `analytics.consistency.violated` — издатели EPIC-005 (`mvctl --audit`, харнесс) и целевой страж; `analytics.replay.completed` — EPIC-002 (recovery) / EPIC-005 (test), схема одна (C-14).
**Уточнения v0.3 (запросы b, c)**: файл `schemas/events/analytics.consistency.violated.v1.json` (`code ∈ state_divergence|log_gap|invariant|…`, `severity ∈ break|warn`, `detected_by`) создаётся в EPIC-001 **F-4b, блок «в»** по спецификации EPIC-005 §4.1 и передаётся владельцу EPIC-005 (как `replay.completed` — EPIC-002). Трасса по `correlation_id`: **`mvctl trace <correlation_id>` по журналу (`Journal.ReadRange` по всем топикам) — 005-ops, Must** (закрывает критерий US-003/NFR-034 без памяти); `GET /v1/trace/{cid}` в C-09 — 005-memory, Should (обогащение связями графа).

### Гарантии
Топик `analytics_events`, не читается в replay; без текста и внешних ID; парные `session.started/ended`; `turn.completed` на каждый принятый/отклонённый ход.
**v1.1 (З-1, FR-061 v0.4)**: `analytics.session.ended.session.end_reason ∈ leave | idle | death | error | forget` — `forget` при закрытии активной сессии каскадом `/forget` (C-04 v1.1); CHECK в `migrations/gateway/0001_init.sql` (`sessions.end_reason`) — тот же список (T-302); `metrics.md` §4.2 и `session_end_reason_share` — запрос BA. `forget` не является ошибкой (`error` по-прежнему = 0 в S-метриках).

### Заглушка
`mvctl session-report` работает с фикстурой `testdata/analytics/*.jsonl`.

### История изменений
v1 — 2026-09-09. v1.1 — 2026-09-09 (сведение 3): `end_reason` + `forget`.

## 11. C-11 · EPIC-003 → авторы блупринтов, EPIC-005 (валидатор в CLI): формат блупринта

Тип: файл · Версия: 1 (v1.1 — glob, правила валидатора) · Владелец: EPIC-003 · Спецификация: `api-contracts.md` §3 (поля, валидация, скелеты) + расширение `role: group-narrator` (scope_binding `{type: group, pattern: "group:*"}`, `trigger: {type: event, event_name: group.created}`, `llm.phase2`, `allowed_event_types: [narrative.output]`, `owned_entity_types: []`, `ttl: 45m`).
Парсер (ADR-015): Markdown с YAML-frontmatter и секциями `## system`, `## phase1`, `## phase2`, `## tick`, `## canon`, `## description`; чистый YAML допустим. Плейсхолдеры промптов — фиксированный словарь (§3.1). `blueprint_version` = семвер из файла; `content_hash` — в `agent.blueprint_reloaded`, `agent.spawned`.
**v1.1**: `trigger.event_name` допускает glob по сегментам (`player.*`, `group.*`; `*` не пересекает точку); валидатор разворачивает glob по реестру `contracts.Types()` и требует ≥ 1 совпадения; **модель** (`llm.{phase1,phase2,tick}.model`) валидируется против списка моделей выбранного провайдера (`MV_LLM_PROVIDER`), блупринт **не может** указать провайдера или включить облако — только `MV_LLM_CLOUD_ENABLED` оператора (SEC-21); `round{timeout, idle_after_missed}` в блупринте `encounter-*` — источник для `encounter.started.round` (C-05).

### Гарантии
`mvctl blueprint validate <dir>` = тот же валидатор, что в рантайме (`agent.Validate(bp, env)`); ошибка — файл, поле, причина; новый регион = новый `domain-*.md` + запись в `laws`/`rules` без Go (S6).

### История изменений
v1 — 2026-09-09 (ADR-002). v1.1 — 2026-09-09: glob, валидация модели/провайдера, `round{}`.

## 12. C-12 · EPIC-003 → EPIC-007 (E-B): законы и пробой

Тип: события + файл · Версия: 1 · Владелец: EPIC-003 (MVP-1), затем EPIC-007.
`laws/{world}.v{N}.yaml`, `Laws.Current/Get`, `world.laws.changed`, `world.law_breach.*` — ADR-008 и `api-contracts.md` §2.3.13; флаг `MV_LAWS_BREACH_PHASE_ENABLED`; счётчик `strain`; уровень `monitor` в реестре.

### Гарантии
`laws_version` присутствует в 100 % `llm.output`, `narrative.output`, `snapshot.created`; страж отклоняет устаревшую версию; типы пробоя валидны по схемам, но не имеют издателя до E-B (`mvctl contracts check` знает исключение).

## 13. C-13 · EPIC-002/003 → EPIC-006 (E-A Living Worlds): резерв для Entity-Actor и эволюции правил

Тип: резерв контракта · Версия: 1 · Владелец: system-architect.
- Уровень `object` в реестре уровней; блупринт с `level: object`, `role: entity-actor`, `owned_entity_types: [<свой тип>]`, `allowed_event_types: [entity.update.proposed, object.*]` валидируется, спавн выключен флагом `MV_SWARM_OBJECT_AGENTS_ENABLED`;
- Entity-Actor меняет состояние **только** через `entity.update.proposed` (C-02) — никаких прямых записей;
- эволюция правил: `rules.change.proposed {rules_version_base, diff, evidence[], proposed_by{agent monitor}}` → ревью человека (`mvctl rules review`) → новая версия `rules/*.yaml` + `rules.changed`; типы зарегистрированы в реестре без издателя;
- `Monitor`-агент: `level: monitor`, `trigger: {type: timer}`, читает `analytics_events` и `llm_records` (единственное исключение из «доменные агенты не читают аналитику»).

## 14. C-14 · EPIC-002, EPIC-003, EPIC-004 → все: снапшоты и протокол восстановления

Тип: события + объекты · Версия: 1 (v1.1 — уточнения) · Владелец: EPIC-002 (формат), реализуют все stateful-контексты.
`snapshot.created {component: state|swarm|gateway, snapshot{id, seq, taken_at, cursor{topic: next_offset}, laws_version, state_hash, size_bytes, key}}` — **значения `component` по этому контракту** (не `api-contracts.md` §2.3.12; system-analyst правит на планировании). Объекты: `snapshots-{world}/{component}/{ts}-{seq}.json` (полный снапшот) + **`snapshots-{world}/{component}/latest.json` — указатель** (метаданные + `key` объекта; ADR-011 п. 2), пишется после успешного PUT объекта; потребители читают указатель, проверяют `state_hash`, затем объект. Префиксы: `state/` — EPIC-002, `swarm/` — EPIC-003, `gateway/` — EPIC-004 (курсоры и проекции; `gateway.db` остаётся истиной для сессий/раундов/outbox). Режим `--mode=replay`; `analytics.replay.completed` — одна схема `schemas/events/analytics.replay.completed.v1.json` (владелец файла EPIC-002; поля `mode=test`, `events_hash_match` добавляет EPIC-005 — совладение, `plan/ownership.md`). Порядок старта: `state` догоняет журнал (`Journal.ReadRange` с `cursor` до `End`) → публикует `replay.completed mode=recovery` → `swarm` и `gateway` строят проекции от `state/latest.json` и догоняют `entity.updated` с `cursor.system_events` через `Journal`; «конец журнала» = `End` по всем читаемым топикам (C-01).
**Уточнение v0.4 (TL2-6)**: (а) **`testkit/state.FakeState` v0 (F-10/T-017) публикует `analytics.replay.completed {mode: recovery, replay{run_id, snapshot_id: null, events_replayed: 0, llm_calls: 0, dice_rolled_new: 0, duration_ms, state_hash_after, incomplete_record: false}}`** сразу после подписки на `system_events` — заглушка соблюдает протокол старта, а не потребители подстраиваются под заглушку; (б) ожидание сигнала потребителем ограничено таймаутом — `swarm`: `MV_SWARM_REPLAY_WAIT` (по умолчанию `120s`, = RTO), по истечении контекст стартует и отмечает `/health degraded {state_replay: missing}` (та же ветка нужна в production при падении `state`); `gateway` — аналогично, если его дизайн ждёт сигнал (`MV_GATEWAY_REPLAY_WAIT`). Издатели типа в реестре: `state` (EPIC-002), харнесс EPIC-005 (`mode=test`), `testkit/state` (§0).

### Гарантии
Снапшот содержит `cursor` по всем читаемым топикам; после восстановления `state_hash_after == hash(снапшот + факты)`; разрыв журнала (retention) → восстановление из объектов сущностей + `/health degraded {log_gap}`; RTO ≤ 2 мин на одной машине (после замера). Хранение снапшотов не зависит от bucket versioning (ротация K=5 в коде; ADR-021).

### История изменений
v1 — 2026-09-09. v1.1 — 2026-09-09: `latest.json` указатель, `component` enum, префикс `gateway/`, совладение схемы `replay.completed`, независимость от versioning. Уточнение v0.4 — 2026-09-09 (сведение 3): `FakeState` v0 публикует `replay.completed`, таймаут ожидания.

## 15. C-15 · EPIC-003 → EPIC-013 (E-H облако), EPIC-005 (эмбеддинги): интерфейс провайдера LLM

Тип: Go API · Версия: 1 (v1.1 — сведение A4-2, U-8) · Владелец: EPIC-003 · Потребители: EPIC-003 (шлюз), EPIC-005 (`Embed`/`Models` из `internal/memory` — импорт `llm.Provider`/`providers.Registry` разрешён ADR-001 доп. п. 7), EPIC-013 (облако).

### Интерфейс
```go
package llm
type Provider interface {
    Generate(ctx context.Context, req Request) (Response, error)
    Embed(ctx context.Context, model string, texts []string) ([][]float32, error)
    Health(ctx context.Context) Status            // ok | loading | degraded(model_not_resident) | unavailable
    Models(ctx context.Context) ([]string, error) // имена моделей провайдера (валидатор C-11 п. 7а, /health.llm)
}
type Request struct {
    Phase, Model, System string; Messages []Message
    Schema json.RawMessage                        // JSON Schema 2020-12 ответа (structured output); nil — свободный текст
    Params Params                                 // v1.1: Temperature, TopP, TopK, MinP, PresencePenalty, MaxTokens float/int; Think bool (по умолчанию false)
    Timeout time.Duration; CorrelationID string
}
type Response struct {
    Content string                                // текст (JSON при Schema)
    ReasoningLen int                              // v1.1: длина отброшенного reasoning_content (в llm.output.parse{}); сам текст не сохраняется
    Tokens struct{ Prompt, Completion, Cached int } // v1.1: Cached — из prompt_tokens_details.cached_tokens (0, если нет)
    LatencyMs int64; Provider, Model string
}
```
Реализации (`providers.Registry`, имя = значение `MV_LLM_PROVIDER`): **`openai_compat`** — по умолчанию (llama-server `MV_LLM_URL=http://127.0.0.1:1234`; `/v1/chat/completions` с `response_format: {type: json_schema, json_schema: {name, schema}}`, `chat_template_kwargs: {enable_thinking: Params.Think}` — thinking выключен для всех фаз MVP-1 (llama.cpp #20345: грамматика не применяется при thinking), семплинг из `Params` на запрос; `/v1/embeddings`; `/v1/models`; `/health` 503 → `loading`); **`ollama`** — native `/api/chat`/`/api/embed`/`/api/tags`/`/api/ps` (`MV_OLLAMA_URL`); **`anthropic`** — целевое (E-H); **`recorded`**, **`fake`** — тесты. Облако — `openai_compat` с внешним `MV_LLM_URL` + `MV_LLM_API_KEY`: провайдер отказывает в старте при не-локальном host без `MV_LLM_CLOUD_ENABLED=true` (ADR-005 доп. 2 п. 3). Middleware (бюджет, запись, парсер, язык, фильтр, страж) — вне провайдера и общие для всех. Блупринт провайдера не выбирает; модель блупринта ∈ `Models()`.

### Гарантии
`Generate` не имеет побочных эффектов в шине (запись `llm.output` — обязанность шлюза); отмена `ctx` закрывает соединение (ADR-014 п. 2); `Tokens` из ответа провайдера (погрешность ≤ 5 %, NFR-052), при отсутствии — оценка по длине с пометкой; сигнатуры v1 не меняются (новые поля `Params`/`Response` — с нулевыми значениями по умолчанию).

### Заглушка
`providers/fake` (таблица `(phase, matcher) → Response`, `Embed` — детерминированный хэш-вектор для EPIC-005), `providers/recorded` (replay). Живой llama-server/Ollama — только стенд.

### История изменений
v1 — 2026-09-09 (ADR-005). v1.1 — 2026-09-09: `openai_compat` по умолчанию, `Params`/`Response` поля, потребитель `internal/memory` (сведение A4-2, U-8).

## 16. Правила изменения контрактов

1. Совместимые изменения (новое опциональное поле, новый тип события, новый эндпоинт, новый код ошибки): владелец добавляет схему/OpenAPI + запись в реестр в своём эпике, уведомляет архитектора в отчёте; `schema_version` не меняется.
2. Несовместимые (удаление/переименование поля, смена семантики, новый обязательный аргумент Go-API): запрос в `plan/ownership.md` → оценка влияния system-architect (список потребителей из реестра) → `schema_version+1` (события) или `/v2` (HTTP) → задачи командам-потребителям; потребители принимают `n` и `n-1` один инкремент.
3. Изменения `shared/*` (C-01, C-03, C-15, `clock`, `runtime`, `objstore`) — только через system-architect; PR помечается `contract-change`.
4. Тест `contracts` в CI: схемы валидны; каждый тип имеет владельца-издателя и ≥ 1 потребителя (исключения: `world.law_breach.*`, `rules.change.*` до E-B/E-A); OpenAPI ↔ маршруты; политики топиков (§0) проверяются на фикстурах.
5. Переменные окружения — часть контракта процесса: объявляются через `shared/env.Declare` с префиксом `MV_`; `.env.example` сверяется с манифестом в CI (NFR-074).
6. **Копия таблицы владения (v0.4, TL2-3).** Истина — `shared/agent/levels.go` (EPIC-003, TEAM-2); `shared/contracts/ownership.go` (`OwnershipRules`) — **статичная копия**, создаётся в EPIC-001 F-4a по `data-model.md` §4 (включая строку gateway `Character.status → abandoned`, `Group.leader_id`). Тест равенства `levels.go ↔ OwnershipRules` (T-202, запускается в job `contracts`) **блокирует merge** при расхождении. Процедура изменения: владелец истины (TEAM-2) меняет `levels.go` **и в том же PR** — копию в `shared/contracts` (метка `contract-change`, ревью tech-lead#1 + system-architect, приоритет блокера); State (EPIC-002) — только потребитель копии, узнаёт из отчёта PR; расхождение без PR — дефект TEAM-2. Обратный порядок (правка копии без истины) запрещён.
7. **Издатели типа (v0.4, TL2-2/TL2-5).** Владелец типа поддерживает `Spec.Publishers` в реестре; добавление издателя — совместимое изменение (п. 1) с уведомлением system-architect; job `contracts` проверяет `source ∈ Spec.Publishers` на фикстурах и в e2e; источники `testkit/*` — только на `membus`/`MV_SWARM_FAKE=true`.

## 17. Сводка заглушек для параллельной работы

| Потребитель | Ждёт | Заглушка | Где |
|---|---|---|---|
| EPIC-002, 003, 004 | C-01 шина/журнал/реестр | `testkit/membus` (шина + `Journal`), реестр с первой волны EPIC-001 | `shared/testkit` |
| EPIC-003 | C-02 факты State | `testkit/state.FakeState` — v0 (F-10/T-017) **публикует `analytics.replay.completed {mode: recovery}` при старте** (C-14 v0.4) и принимает `set status=abandoned cause=forget` от gateway (C-02 v1.2) | `shared/testkit/state` |
| EPIC-003 | C-03 механика | `testkit/state.FixedMechanics` + `rules/dark-forest.yaml` | `shared/testkit/state`, `rules/` |
| EPIC-003, 002 | C-04 действия игрока | `testkit/gateway.Harness` (генератор `player.*`) | `shared/testkit/gateway` |
| EPIC-004, EPIC-002 | C-05 нарратив | `testkit/swarm.FakeNarrator` — v0 (только нарратив) из F-10, реализация — EPIC-003 C4 | `shared/testkit/swarm` |
| EPIC-004, EPIC-002 (I1-α) | Phase 1 боя (`encounter.*`, `combat.decided`, `dice.rolled`, `entity.update.proposed`) до готовности роя | **`testkit/swarm.FakeEncounter`** — EPIC-003 C4, первая единица I1a, ранний merge подпакета в `integration/mvp-1`; монтируется в `core` хуком `MV_SWARM_FAKE=true` (`cmd/multiverse`, удаляется в I1) | `shared/testkit/swarm` |
| EPIC-003 | C-09 память | `journalContext` (штатная деградация) | `internal/swarm/context` |
| EPIC-003, EPIC-005 | C-15 провайдер LLM | `providers/fake` (в т. ч. `Embed`), `providers/recorded`; живой llama-server/Ollama — стенд | `internal/llm/providers` |
| telegram-bot | C-08 gateway | `testkit/gateway.FakeGateway` | `shared/testkit/gateway` |
| EPIC-005 | C-06/C-07/C-10 события | фикстуры `testdata/analytics`, `testdata/recordings` | `testdata/` |
| все | часы/таймеры | `clock.Manual`, `ManualTimers` | `shared/clock` |
