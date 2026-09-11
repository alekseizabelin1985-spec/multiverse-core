# Дизайн эпика EPIC-005 «Память и операции» (005-ops Must · 005-memory Should)

Версия 0.1 · 2026-09-09 · architect#1 (TEAM-1) · статус: к нарезке tech-lead#1 (`tasks.md`); 005-ops — на освободившихся слотах TEAM-1 после I1-α (волна 1), 005-memory — волна 2, отрезаема на G3/G4.
Команда TEAM-1 · ветка `epic/EPIC-005-memory-ops` (от `integration/mvp-1`; создаётся при старте 005-ops) · G2 утверждён 2026-09-09 (допущение «005-memory отрезаема» принято пользователем).
Основание: `plan/epics.md` v0.2 §2 «EPIC-005»; `contracts.md` v0.2 (C-06, C-07, C-09, C-10, C-14, C-15); ADR-004 (Qdrant/Neo4j), ADR-005 п. 4, ADR-010 (golden, записи, `--audit`); `project/metrics.md` §6 (таблица отчёта, CSV); `state-and-mechanics.md` §3.3 (`StateHash`), §4.8; `swarm-llm-laws.md` §2 (`context/journal.go`, `memory.go`); `consolidation.md` (T-9, T-17, S-6, D-3); as-is `services/semantic-memory` (Neo4j-индексер 1968 строк, Chroma-адаптер — заменяется).

---

## 1. Цель и границы

**Цель**: оператор после каждой сессии получает одну таблицу «мир держится / сколько стоил LLM / как быстро отвечали» и ведёт журнал инцидентов (US-038); CI проверяет golden-набор и полноту реестра контрактов; **отдельно и отрезаемо** — семантическая память (Qdrant + Neo4j) обогащает контекст роя фактами и сводкой «пока тебя не было» через C-09.

Две части с раздельной приёмкой (ID эпика один):

| Часть | Приоритет | Что входит | Чего нет без неё |
|---|---|---|---|
| **005-ops** | Must | `mvctl report` (`--audit`, `--json`, `--weekly`, `--background`), `mvctl trace`, `mvctl llm usage`, `mvctl golden check/update`, наполнение `mvctl contracts check`, `ops/metrics/{sessions,background,incidents}.csv`, `testdata/analytics/`, закрепление порогов «после замера» в `nfr.md` (с BA), `analytics.replay.completed mode=test` (поля в общей схеме) | приёмка S1/S2 без отчёта; golden NFR-065; S7 не измерим |
| **005-memory** | Should, отрезаема | `internal/memory` (Qdrant вместо Chroma, Neo4j 5.26 без APOC), `/v1/context/scope`, `/v1/context/absence`, `/v1/trace/{cid}`, `mvctl memory rebuild`, `api/memory.openapi.yaml`, экранирование `generated`-фактов (T-9) | **ничего из Must**: рой работает на `journalContext` (EPIC-003, штатная деградация C-09); сводка «пока тебя не было» — из окна журнала; трасса — `mvctl trace` |

**Не входит**: `mvctl record`/`RecordingWriter` и записи `testdata/recordings` (EPIC-003 I1a); `mvctl blueprint validate`/`laws bump|show` (код EPIC-003, реестр CLI — tech-lead#1); `mvctl world init` (EPIC-002); события `analytics.session.*`/`turn.completed` (EPIC-004); Prometheus/Grafana (EPIC-012); полное право на забвение в памяти (EPIC-013): на MVP-1 `/forget` удаляет только связку в `links.db`, события с псевдонимом `player_id` остаются в журнале и в индексе — перестройка памяти после `/forget` не требуется (ADR-009).

---

## 2. Входящие требования (трассировка)

| Требование | Часть | Что в EPIC-005 | UC / проверка |
|---|---|---|---|
| US-038 (FR-085…FR-088, FR-101, BR-17, NFR-032) — CLI/CSV | ops | `mvctl report` таблица `metrics.md` §6.1, `sessions.csv`, `--audit` → `analytics.consistency.violated code=state_divergence`, `--json`, `--weekly` только `human`; `incidents.csv` ручной с `manual_fix` | UC-033 |
| US-012 (FR-070…FR-072) — учёт | ops | `mvctl llm usage`: вызовы/токены/латентность по фазам и уровням из `llm_records` (журнал) или `GET /v1/admin/llm/usage` (`--live`) | UC-024, UC-030 |
| US-003 (последний критерий), US-015 Should, US-018 (FR-084, FR-092, NFR-034) — трасса | ops (`mvctl trace`) / memory (`/v1/trace`) | цепочка по `correlation_id` из журнала: действие → `dice.rolled` → `combat.decided` → `llm.output` → `entity.updated` → `narrative.output` → `turn.completed` | UC-032 |
| US-017 (FR-101, NFR-061/062), UC-026 | ops | `analytics.replay.completed mode=test` (`events_hash_match`, `incomplete_record`) — поля в общей схеме (совладение S-6); отчёт расхождения «ход/раунд и первое отличающееся событие» — `mvctl golden check` | UC-026 E2 |
| NFR-065 (golden) | ops | golden-набор: 20 ходов + 3 тика из записи EPIC-003 (`actor_kind=ci`), сравнение схемы/языка/чисел/фильтра (a); CI job | ADR-010 п. 1 |
| FR-035, FR-093 (память: слои, приоритет `authored`) | memory | индекс с `source ∈ authored|validated|generated`, `layer`; ранжирование `authored > validated > generated` в `/v1/context/scope` | US-005 крит. 3 |
| FR-127 (сводка «пока тебя не было») | memory (обогащение) | `/v1/context/absence` по `(region, since_at, until_at)`; Must-часть — `journalContext` EPIC-003 | UC-004 шаг 7 |
| US-036 (Should-часть: «события индексируются семантической памятью») | memory | индексация `world.*`, `region.*`, `npc.*`, `entity.updated`, `narrative.output`, `combat.decided` ≤ 2 с | S14 |
| NFR-030/032/034 (пороги «после замера») | ops | значения из `baseline.md` (F-8) и первых прогонов S1 → правка `nfr.md` (BA) | — |
| NFR-041 (нет внешних ID в `gateway.db`/логах) | ops | тест `privacy-scan` над `gateway.db`, `links.db`(только структура), логами бота — читает файлы только для проверки отсутствия утечек (`ownership.md`) | — |
| NFR-064 | ops | покрытие `cmd/mvctl/internal/{report,golden}` unit ≥ 60 % (в общий gate не входит — пакеты не в списке ядра; целевой ориентир) | — |
| T-17 (без APOC), T-9 (экранирование `generated`) | memory | compose без `apoc`; факты `generated` в ответе помечены и обрезаны (≤ 500 симв.) — экранирует Swarm, memory гарантирует поле `source` | SEC-17, SEC-32 |

---

## 3. Подход

1. **005-ops — только чтение журнала и объектов**: все команды `mvctl` работают через `eventbus.Journal` (kafka) **или** через `replay.Recording` (`--from-recording <jsonl>`) — одна и та же агрегация в CI (на записи) и на стенде (на Redpanda). Никакой БД: CSV в git (`metrics.md` §6.2).
2. **`--audit` не изобретает сверку**: использует `internal/state/audit.Recompute` (EPIC-002 I2) и `entity.StateHash` — тот же код, что recovery; сравнивает `hash(снапшот + факты)` с `hash(объекты entities-{world})`.
3. **Golden — отдельная команда, а не отдельный харнесс**: `mvctl golden check` прогоняет запись через `--contexts=all --bus=memory --mode=replay` (через `testkit/harness`) и сравнивает поток доменных событий с эталоном; `mvctl golden update` перегенерирует эталон (осознанная задача с ревью диффа, ADR-010).
4. **005-memory — адаптер, не переписывание**: Neo4j-индексер as-is (`indexer.go`, `neo4j.go`) копируется в `internal/memory` с заменой `eventbus`-типов на конверт `meta` и `event.Path()`; Chroma-адаптер (`chroma*.go`) заменяется Qdrant-адаптером с тем же внутренним интерфейсом `VectorStore`; HTTP — `/v1/context/*`, `/v1/trace` по `api/memory.openapi.yaml`; эмбеддинги — `llm.Provider.Embed` (C-15).
5. **Отрезаемость встроена**: `memory` — отдельный процесс/профиль compose; Swarm без `MV_MEMORY_URL` использует `NopMemory` → `journalContext`; ни один Must-критерий не читает память.

---

## 4. Компоненты

### 4.1. 005-ops — структура

```
cmd/mvctl/internal/
  report/           mvctl report: source.go (Journal | Recording), window.go (окно сессии по session.id или --since/--until),
                    aggregate.go (таблица metrics.md §6.1: ходы, поломки, латентности p50/p95, LLM по фазам/уровням, fallback,
                    миграция, восстановление), audit.go (state/audit.Recompute + objstore ListEntities → consistency.violated),
                    csv.go (sessions.csv, background.csv — append, заголовок фиксирован), json.go (--json артефакт),
                    weekly.go (NSM/S7 из sessions.csv, только actor_kind=human), render.go (Markdown в stdout)
  trace/            mvctl trace <correlation_id> [--from-recording]: ReadRange по всем ReplayRead-топикам + analytics/llm_records,
                    фильтр meta.correlation_id, сортировка по (topic, offset), печать цепочки с agent{level, blueprint}, phase, prompt_hash
  llmusage/         mvctl llm usage [--since 24h | --session id | --live]: агрегат llm_records (calls, tokens, cost_usd, latency p50/p95,
                    by level/phase/provider/model, cloud_calls); --live → GET MV_CORE_URL/v1/admin/llm/usage (C-06)
  golden/           mvctl golden check|update <scenario>: запуск testkit/harness in-process на testdata/recordings/<scenario>.jsonl,
                    сбор доменных событий, нормализация, сравнение с testdata/golden/<scenario>.expected.jsonl; отчёт «ход/раунд,
                    первое отличие»; проверки: схема (contracts.Validate), язык (llm.filter Language — из EPIC-003), числа (dice/hp),
                    фильтр (a) статус; публикует analytics.replay.completed mode=test {events_hash_match, incomplete_record}
  contracts/        (каркас F-4c, EPIC-001) наполнение: проверки (в)–(г) foundation design §4.1; отчёт «фантомные типы/топики»
ops/metrics/
  sessions.csv      заголовок по metrics.md §6.1 (одна строка на сессию)
  background.csv    date, region, events, llm_calls_max_per_hour, cap, busy_ms, breaks
  incidents.csv     date, session_id, correlation_id, kind, manual_fix, description  — ручной, только заголовок в git
  baseline.md       (создаёт F-8) — источник порогов
testdata/analytics/ *.jsonl фикстуры analytics.* + llm.output для unit report (actor_kind=ci, без внешних ID)
testdata/golden/    <scenario>.expected.jsonl — эталоны (записи — testdata/recordings, владелец EPIC-003)
schemas/events/analytics.consistency.violated.v1.json     (издатель — mvctl --audit / golden; EPIC-005)
schemas/events/analytics.replay.completed.v1.json         (файл EPIC-002; EPIC-005 добавляет mode=test поля — PR с ревью EPIC-002)
```

Порядок 005-ops (по слотам `teams.md` §4, developer#2 после I1-α): (1) `report` источник + окно + агрегат + CSV/Markdown на фикстурах `testdata/analytics/` → (2) `trace` + `llmusage` (переиспользуют `report/source.go`) → (3) `golden check` на первой записи EPIC-003 I1a (`solo-30.jsonl`) + CI job → (4) `--audit` после EPIC-002 I2-4 → (5) `contracts check` наполнение + `privacy-scan` NFR-041 → (6) пороги в `nfr.md` с BA после первых прогонов S1 на стенде.

### 4.2. 005-memory — структура

```
internal/memory/
  context.go        runtime.Context "memory": DependsOn [] ; Start: EnsureCollections (Qdrant), EnsureConstraints (Neo4j), Subscribe
                    (группа core.memory) на world_events, game_events, system_events (entity.updated, narrative.output ...), Routes(mux)
  indexer.go        из as-is semanticmemory/indexer.go: событие → Document{text, source, layer, event ref, scope, at} + Relations (Neo4j)
  document.go       текст документа из события (шаблоны по типу; narrative.output.text → generated; blueprint description → authored
                    загружается mvctl memory rebuild --blueprints; entity.updated → validated)
  vector.go         interface VectorStore{Upsert, Search(scope, query, k), Delete}; qdrant.go (qdrant/go-client v1.19.x, коллекции
                    events-{world}, entities-{world}; payload: world, scope, source, layer, at, event_id)
  graph.go          interface GraphStore{UpsertEntity, UpsertRelation, Neighbors, Trace}; neo4j.go (из as-is neo4j.go, без APOC)
  embed.go          Embedder над llm.Provider.Embed (C-15; MV_EMBED_MODEL, кэш по sha256(text))
  http.go           POST /v1/context/scope, POST /v1/context/absence, GET /v1/trace/{cid}, GET /health — по api/memory.openapi.yaml
  absence.go        сводка фона: события world/region за (since_at, until_at) с actor_kind=system → summary (шаблон, без LLM)
  trace.go          цепочка по correlation_id: Neo4j (CAUSED_BY) + fallback Journal
  rebuild.go        mvctl memory rebuild --world: очистка коллекций → ReadRange всех топиков с earliest → индексация; --blueprints
                    добавляет authored-факты из blueprints/*.md (## description, ## canon) — parser EPIC-003 (shared/agent)
api/memory.openapi.yaml
cmd/mvctl/internal/memory/  rebuild.go (тонкая обёртка над internal/memory.Rebuild)
```

Порядок 005-memory (волна 2, developer#1 ∥ developer#2): Qdrant-адаптер + `document`/`indexer` ∥ Neo4j перенос за `integration` + HTTP; затем `absence`/`trace`/`rebuild`; последним — e2e «сводка из памяти = сводка из журнала».

### 4.3. Деградация роя при отсутствии памяти (закреплённая схема)

| Условие | Поведение Swarm (EPIC-003, C-09) | Что делает EPIC-005 |
|---|---|---|
| 005-memory не поставлена (отрезана) / `MV_MEMORY_URL` пуст | `NopMemory` → `journalContext` всегда: `EventWindow` (кольцо N на scope), `BackgroundIndex` (world/region, 30 дн.), `Presence` | ничего; Must-критерии US-005/US-036/FR-127 выполняются на журнале |
| `MV_MEMORY_URL` задан, memory недоступна / таймаут 500 мс / 5xx | fallback на `journalContext` без ошибки игроку; лог `memory_fallback`; `narrative.output` без изменений (поле `based_on[]` из журнала) | `/health` memory `degraded|fail`; `mvctl report` показывает `memory_fallback_rate` (новая строка блока «LLM», не тревога) |
| memory доступна | `<memory>` секция промпта: факты `authored/validated/generated` (генерированные — экранированы и ограничены, T-9) + `absence` из `/v1/context/absence` **объединяется** с `journalContext` (журнал приоритетнее по свежести ≤ 2 с) | индексация ≤ 2 с; ответы без внешних ID |
| `--mode=replay` | `NopMemory` всегда (детерминизм; память не участвует в replay) | `memory rebuild` из записи не требуется |

Проверка отрезаемости: e2e S1/S14 EPIC-003 гоняются **без** контекста `memory` (по построению — `--contexts=all` включает `memory` только если профиль/флаг `MV_MEMORY_ENABLED=true`; в CI `false`).

---

## 5. Интерфейсы с другими эпиками

| Контракт | Роль EPIC-005 | Часть | Что потребляет / поставляет | Заглушка |
|---|---|---|---|---|
| C-10 (аналитика) | потребитель | ops | `analytics.session.started/ended`, `analytics.turn.completed` (EPIC-004); **поставщик** `analytics.consistency.violated` (`--audit`, golden), `analytics.replay.completed mode=test` | фикстуры `testdata/analytics/*.jsonl` |
| C-07 (записи LLM) | потребитель | ops, memory | `llm.output(.rejected)` для `llm usage`, `trace`, golden; записи `testdata/recordings` (только `actor_kind=ci`) | фикстуры; `RecordingWriter` (EPIC-003) |
| C-06 (admin HTTP) | потребитель | ops | `GET /v1/admin/llm/usage` (`--live`), `POST /v1/admin/agents/{id}/tick` (golden `background` сценарий) через `MV_CORE_URL` | до EPIC-003 — только журнал |
| C-14 (снапшоты) | потребитель | ops | `latest.json` + объект State для `--audit`; `analytics.replay.completed` — совладение схемы | фикстура `latest.json` |
| `internal/state/audit` (Go) | потребитель | ops | `Recompute`, `Compare` (EPIC-002 I2) | до I2 — `--audit` сравнивает только `state_hash` снапшота с хэшем объектов (без фактов) с пометкой `partial` |
| C-09 (контекст памяти) | **поставщик** | memory | `/v1/context/scope`, `/v1/context/absence`, `/v1/trace`, `/health`; `MemoryClient` в Swarm | `journalContext` (EPIC-003) — штатная деградация |
| C-15 (провайдер LLM) | потребитель | memory | `llm.Provider.Embed`, `Models` | `providers/fake` Embed (детерминированный хэш-вектор) |
| C-11 (блупринты) | потребитель | memory | `## description`, `## canon` → `authored` факты (`rebuild --blueprints`) | парсер `shared/agent` (EPIC-003 I1a) |
| C-01 | потребитель | обе | `Journal.ReadRange/End`, `contracts.Validate/Lookup/Topics`, `objstore` (чтение) | `membus`, `Recording` |

---

## 6. Данные и миграции

- **005-ops**: три CSV в `ops/metrics/` (заголовки фиксированы; `incidents.csv` — ручной); `ops/metrics/sessions/<session_id>.json` (артефакты `--json`, в `.gitignore` кроме golden); `testdata/golden/*.expected.jsonl` (`merge=binary`); `testdata/analytics/*.jsonl`. Схема `analytics.consistency.violated.v1.json` (издатель EPIC-005). Поля `mode=test`, `events_hash_match`, `incomplete_record` — в `analytics.replay.completed.v1.json` (PR в файл EPIC-002).
- **005-memory**: Qdrant коллекции `events-{world}` (размерность по `MV_EMBED_MODEL`; payload-индексы `world`, `scope`, `source`, `at`), `entities-{world}`; Neo4j constraints/indexes (как as-is, кодом при старте; без APOC). Оба — **производные**: бэкапа нет, `mvctl memory rebuild` из журнала за retention (30 дн. доменных топиков) + снапшот State. Данные as-is Chroma/Neo4j не мигрируют (профиль `legacy` живёт отдельно на `:8083`, D-3).
- **Миграций схем нет** (MVP-1).

---

## 7. Конфигурация

005-ops: `MV_KAFKA_BROKERS`, `MV_MINIO_*`, `MV_CORE_URL=http://127.0.0.1:8090` (или через gateway прокси `MV_GATEWAY_URL`), флаги `--from-recording`, `--since/--until`, `--json`, `--audit`, `--weekly`, `--background`. 005-memory: адрес HTTP — `MV_CORE_ADDR`, как у любого процесса (в compose у `memory` — литерал `:8082`; `MV_MEMORY_ADDR` выведен, T-408), `MV_MEMORY_ENABLED=false` (default; включает контекст в `--contexts=all`), `MV_QDRANT_ADDR`, `MV_NEO4J_URI`, `MV_NEO4J_USER`, `MV_NEO4J_PASSWORD`, `MV_EMBED_MODEL` (имена — по манифесту `shared/env/vars.go`; переменные, которых в манифесте ещё нет, объявляет изменение, вводящее их чтение, — `contracts.md` C-14 v0.4), `MV_LLM_PROVIDER` (для `Embed`), `MV_MEMORY_INDEX_LAG_MAX=2s` (health). У Swarm — `MV_MEMORY_URL` (пусто = `NopMemory`), `MV_MEMORY_TIMEOUT=500ms` (EPIC-003). Профиль compose `memory` (Qdrant, Neo4j, процесс `memory`).

---

## 8. Безопасность (что учтено)

- Записи и golden — только `actor_kind=ci` и фикстурные игроки (T-5/SEC-23); `privacy-scan` над `testdata/`, `ops/metrics/*.csv` (нет внешних ID/текста — только `player_id`, счётчики).
- `--audit`/`trace`/`llm usage` — чтение; единственная запись в шину — `analytics.consistency.violated` и `replay.completed mode=test` с `meta.actor_kind=ci`, `source=mvctl`.
- Память: ответы без внешних ID; `generated`-факты помечены `source` и ограничены по длине — Swarm экранирует как `<player_text>` (T-9, SEC-17); e2e `injections-10` (EPIC-003) включает сценарий через память (когда 005-memory поставлена).
- Neo4j без APOC (T-17); Qdrant без API-ключа только на loopback; `MV_NEO4J_PASSWORD` обязателен (compose `:?`).
- `memory` индексирует `player.said.text` (≤ 500 симв., без внешних ID по C-04) как `layer=player`, `source=validated` (факт события, не вывод LLM); выдаётся только в `/v1/context/scope` того же scope. Swarm экранирует такие факты как `<player_text>` (SEC-17). Отключается флагом `MV_MEMORY_INDEX_PLAYER_TEXT=false`.

---

## 9. Тестируемость (ADR-010)

| Уровень | 005-ops | 005-memory |
|---|---|---|
| unit | `report/aggregate` на `testdata/analytics/` (известные p50/p95, счётчики, `end_reason`); `csv` идемпотентность заголовка; `weekly` только `human`; `trace` сортировка/фильтр; `llmusage` агрегаты; `golden` нормализация и «первое отличие»; `contracts` проверки на синтетическом реестре с фантомом | `document` шаблоны по типам; `absence` сводка на фиктивных событиях; ранжирование `authored > validated > generated`; `Embedder` кэш; `VectorStore`/`GraphStore` in-memory фейки |
| integration (`-tags integration`) | `--audit` над MinIO testcontainers + `Journal` redpanda: снапшот + факты ↔ объекты (совпадение и подложенное расхождение → `state_divergence`) | Qdrant testcontainers: upsert/search по scope; Neo4j 5.26: constraints, relations, `Trace`; индексация ≤ 2 с |
| e2e | `mvctl golden check solo-30` в CI (job `e2e`); `mvctl report --from-recording` на записи S1 даёт CSV/JSON; `trace` по `correlation_id` хода из записи содержит все 7 типов цепочки | «сводка из памяти совпадает с журналом»: S14 запись → `memory rebuild` → `/v1/context/absence` ⊇ фоновые события `journalContext`; деградация: memory остановлена → `narrative.output` без ошибки, `memory_fallback` в логе |
| contracts | `api/memory.openapi.yaml` ↔ маршруты; схемы `analytics.consistency.violated`, `replay.completed` покрывают примеры | то же |
| стенд | `mvctl report` на живой сессии S1/S2; пороги NFR-030/032/034 → `nfr.md` | индексация на живом Ollama-эмбеддинге |
| критерии готовности | `mvctl report` даёт CSV/JSON по прогону S1/S2; golden проходит в CI | сводка «пока тебя не было» из памяти совпадает с журналом |

---

## 10. Отклонённые альтернативы

| Альтернатива | Почему отклонена |
|---|---|
| TimescaleDB/reality-monitor как коллектор метрик | `metrics.md` §6.2: CSV + CLI для 8 сессий; Timescale без клиента (as-is) → `_archive` |
| Golden как отдельный Go-тест в `internal/…` | одна команда `mvctl golden` работает и в CI, и на стенде; отчёт «первое отличие» нужен оператору |
| Своя сверка снапшота в `mvctl --audit` | дублирует recovery; `state/audit.Recompute` — один код (EPIC-002 I2) |
| Оставить Chroma | ADR-004: Qdrant (клиент, фильтры по payload, один бинарник); Chroma живёт только в `legacy` |
| Переписать Neo4j-индексер с нуля | 1968 строк работают, тесты есть (за `integration`); меняются только типы событий |
| Память как обязательная зависимость роя | C-09: штатная деградация; отрезаемость 005-memory — решение G2 |
| `/v1/trace` только в memory | US-003 критерий трассы — Must; `mvctl trace` по журналу в 005-ops закрывает его без памяти |
| Эмбеддинги отдельным HTTP-клиентом к Ollama внутри memory | дублирует провайдера; C-15 предназначен для EPIC-005 — нужен импорт `memory → llm` (запрос в §12) |

---

## 11. Риски и допущения

| Риск / допущение | Реакция |
|---|---|
| 005-ops стартует до готовности записей EPIC-003 I1a | `report`/`trace`/`llmusage` пишутся на фикстурах `testdata/analytics/` (синтетика по схемам); golden — после первой записи `solo-30.jsonl` |
| `--audit` до EPIC-002 I2-4 | режим `partial` (только хэш снапшота vs объекты); полный — после I2 |
| Порог «индексация ≤ 2 с» на Ollama-эмбеддингах при фоновых тиках | эмбеддинг асинхронно от подписки (очередь), `index_lag` в `/health`; при превышении — `degraded`, не потеря |
| Размерность вектора привязана к модели эмбеддинга; смена модели = rebuild | коллекция именуется с суффиксом модели; `rebuild` обязателен при смене `MV_EMBED_MODEL` |
| Записи `merge=binary`: конфликт golden при параллельных правках промптов | `mvctl golden update` — отдельная задача с ревью; одна запись = одна строка, текстовый diff сохраняется (D-14) |
| Импорт `memory → llm` не разрешён ADR-001 п. 3 | запрос system-architect (§12); до решения — `Embedder` за интерфейсом, реализация в `cmd/multiverse` (сборка) |
| 005-memory отрезана на G3/G4 | все Must-критерии проверены без `memory` в CI по построению (`MV_MEMORY_ENABLED=false`); `api/memory.openapi.yaml` остаётся как спецификация будущего |
| Допущение: `player.said.text` индексируется как `validated` факт scope | если security/BA возразят — флаг `MV_MEMORY_INDEX_PLAYER_TEXT=false` |

---

## 12. Запросы к системному архитектору (из этого дизайна)

1. **ADR-001 п. 3 (границы импортов)** — разрешить `internal/memory → internal/llm` (только `llm.Provider`/`providers.Registry` для `Embed`, C-15 явно называет EPIC-005 потребителем). Совместимо; альтернатива — поднимать контекст `llm` в процессе `memory` (`--contexts=memory,llm`), что тянет весь шлюз ради эмбеддингов.
2. **`analytics.consistency.violated`** — подтвердить, что схема (`code ∈ state_divergence|log_gap|invariant|…`, `severity ∈ break|warn`, `detected_by`) создаётся в F-4b (блок «в») с владельцем EPIC-005 — так в `contracts.md` §0, но в списке F-4b явно не назван.
3. **`mvctl trace` в 005-ops** (Must, по журналу) в дополнение к `/v1/trace` (Should, memory) — уточнение реестра `epics.md` §1 (US-003 последний критерий → 005-ops).

Связь: ADR-004, ADR-005 п. 4, ADR-009, ADR-010, ADR-011 (`StateHash`), ADR-021; C-06, C-07, C-09, C-10, C-14, C-15; `metrics.md` §6.
