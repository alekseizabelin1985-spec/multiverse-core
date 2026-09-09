# Задачи EPIC-005 «Память и операции» (005-ops — Must, волна 1→2 · 005-memory — Should, волна 2, **отрезаемо**)

Версия 0.1 · 2026-09-09 · tech-lead#1 (TEAM-1) · статус: к G3.
Команда TEAM-1 · ветка `epic/EPIC-005-memory-ops` (от `integration/mvp-1`; создаётся при старте 005-ops, подволна 1.8) · G2 утверждён 2026-09-09 (допущение «005-memory отрезаема» принято пользователем).
Основание: `epics/EPIC-005-memory-ops/design.md` v0.1 (§1 состав частей, §4.1/§4.2 структура и порядок, §9 тестируемость, §12 запросы к архитектору); `plan/epics.md` v0.2 §2 «EPIC-005»; `architecture/contracts.md` v0.2 (C-01, C-06, C-07, C-09, C-10, C-14, C-15); ADR-004, ADR-005 п. 4, ADR-009, ADR-010, ADR-011, ADR-021; `project/metrics.md` §6; `plan/teams.md` §4; `plan/ownership.md` v0.2; `epics/EPIC-001-foundation/tasks.md` §8 (волны проекта).

Диапазон номеров: **T-130…T-199** (TEAM-1). Размер: **S** ≤ полдня · **M** ≤ одной сессии · L не допускается.

**Две части, раздельная приёмка** (ID эпика один):

| Часть | Приоритет | Задачи | Старт | Отрезаемость |
|---|---|---|---|---|
| **005-ops** | **Must** | T-130…T-140 (11) | подволны 1.8–2.2 — **на освободившихся слотах TEAM-1 после приёмки EPIC-002 I1** (developer#2) | не отрезается: нужна для приёмки I1 и I2 (`session-report`, golden), закрывает US-038 CSV, US-012, NFR-065 |
| **005-memory** | **Should** | T-141…T-148 (8) | подволны 2.1–2.6 (developer#1 с 2.1; developer#2 подключается с 2.4, освободившись от 005-ops) | **отрезаемо на G3/G4 без потери Must-историй**: рой работает на `journalContext` (C-09, штатная деградация); ни один Must-критерий не читает память (`MV_MEMORY_ENABLED=false` в CI по построению) |

---

## 1. Общий DoD (применяется к каждой задаче)

1. `make lint` чист; `go build ./... && go vet ./...` зелёные; `go test -short -race -count=1 ./...` зелёные.
2. **CI зелёный** (`unit`, `integration`, `e2e`, `contracts`, `security`, `compose-lint`) на PR в ветку эпика и в `integration/mvp-1`.
3. Тесты в той же задаче по уровням `design.md` §9; целевое покрытие `cmd/mvctl/internal/{report,golden}` ≥ 60 % (ориентир NFR-064; в общий `coverage-gate` не входит — пакеты не в списке ядра).
4. **Только чтение чужих данных**: все команды `mvctl` читают `eventbus.Journal` (kafka) **или** `replay.Recording` (`--from-recording <jsonl>`) и объекты `objstore`; единственные публикации — `analytics.consistency.violated` и `analytics.replay.completed mode=test` с `meta.actor_kind=ci`, `source=mvctl`.
5. **Приватность**: в `ops/metrics/*.csv`, `testdata/analytics/**`, `testdata/golden/**` нет внешних ID, username и текста игроков; job `security` (`privacy-scan`) зелёный (T-5/SEC-23, NFR-041).
6. `dev-log.md` эпика заполнен: `developer#K`, `T-NNN`, что сделано, отклонения, запросы к владельцам контрактов.
7. Владение: EPIC-005 владеет `cmd/mvctl/internal/{report,trace,llmusage,golden,memory,privacy}`, `internal/memory/**`, `api/memory.openapi.yaml`, `ops/metrics/**`, `testdata/analytics/**`, `testdata/golden/**`, схема `analytics.consistency.violated`. Реестр `cmd/mvctl/main.go` — **⚠ только через tech-lead#1**. `schemas/events/analytics.replay.completed.v1.json` — **совладение**: файл EPIC-002, поля `mode=test` добавляются PR с ревью EPIC-002. `testdata/recordings/**` — EPIC-003 (только чтение).
8. Поведение соответствует критериям приёмки US-038, US-012, US-003 (трасса), US-017, US-015 (Should), US-005/US-036 (Should-часть).

**Критерии приёмки частей** (ворота тимлида):
- **005-ops**: `mvctl session-report <session_id>` даёт таблицу `metrics.md` §6.1 + строку `ops/metrics/sessions.csv` + `--json` артефакт по прогону **S1 и S2**; `mvctl golden check solo-30` проходит в CI (job `e2e`); `mvctl trace <cid>` печатает полную цепочку из 7 типов; `mvctl llm usage` даёт вызовы/токены/латентности по фазам и уровням; `mvctl contracts check` без фантомов на полном реестре; `--audit` даёт `state_divergence = 0` на записи S2; пороги «после замера» закреплены в `nfr.md`.
- **005-memory**: сводка «пока тебя не было» из памяти совпадает с журналом (`/v1/context/absence` ⊇ фоновые события `journalContext`); при остановленной памяти рой отвечает без ошибки (`memory_fallback` в логе).

---

## 2. Часть 005-ops (Must) — задачи T-130…T-140

Порядок (`design.md` §4.1): (1) `report` источник + окно + агрегат + CSV/Markdown на фикстурах → (2) `trace` + `llmusage` → (3) `golden check` + CI job → (4) `--audit` (после EPIC-002 T-069) → (5) наполнение `contracts check` + `privacy-scan` → (6) пороги NFR в `nfr.md` с BA.

### T-130: `report` — источник, окно сессии, фикстуры `testdata/analytics/` · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.8 · ⚠ регистрация подкоманды — через tech-lead#1
- **Описание**: `cmd/mvctl/internal/report/source.go` — единый источник событий: `Journal` (kafka, `ReadRange` по всем `ReplayRead`-топикам + `analytics_events` + `llm_records`) **или** `replay.Recording` (`--from-recording <jsonl>`); `window.go` — окно сессии по `session.id` либо `--since/--until`; фикстуры `testdata/analytics/*.jsonl` (синтетика по схемам: `analytics.session.started/ended`, `analytics.turn.completed`, `llm.output`, `combat.decided`, `narrative.output`; `actor_kind=ci`, без внешних ID).
- **Файлы**: `cmd/mvctl/internal/report/{source,window}.go`, `testdata/analytics/**`, регистрация `report` в `cmd/mvctl/main.go` (PR через tech-lead#1).
- **Зависимости**: T-010, T-014 (волна 0); схемы `analytics.*` (T-009).
- **Ссылки**: `design.md` §3 п. 1, §4.1; C-10, C-07, C-01; `metrics.md` §6; US-038.
- **DoD**: одна и та же агрегация работает на `Journal` и на `Recording` (тест на паре «запись ↔ membus-журнал» даёт идентичный набор событий); окно сессии корректно при отсутствии `session.ended` (обрыв → `end_reason=abandoned`); фикстуры проходят `privacy-scan`; общий DoD §1.

### T-131: `report` — агрегат, Markdown, `sessions.csv`, `--json` · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.9
- **Описание**: `aggregate.go` — таблица `metrics.md` §6.1 (ходы, поломки, латентности p50/p95 по фазам, вызовы/токены LLM по фазам и уровням агентов, fallback, `memory_fallback_rate`, миграция `gm_path`, восстановление); `render.go` (Markdown в stdout); `csv.go` (`ops/metrics/sessions.csv` — append, заголовок фиксирован и идемпотентен); `json.go` (`--json` → `ops/metrics/sessions/<session_id>.json`, FR-101).
- **Файлы**: `cmd/mvctl/internal/report/{aggregate,render,csv,json}.go`, `ops/metrics/sessions.csv` (заголовок).
- **Зависимости**: T-130.
- **Ссылки**: `design.md` §4.1; `metrics.md` §6.1, §6.2; **US-038** (критерий «`session-report <session_id>` → таблица + строка в CSV»); FR-085…FR-088, FR-101; NFR-032.
- **DoD**: unit на `testdata/analytics/` — известные p50/p95 и счётчики совпадают с эталоном; повторный запуск не дублирует заголовок CSV и добавляет ровно одну строку; `status=rejected` ход не входит в латентности (US-038 критерий 7); `--json` артефакт валиден и содержит те же числа, что Markdown; общий DoD §1.

### T-132: `report --weekly`, `incidents.csv`, `background.csv` · Размер: S · Статус: todo · Исполнитель: developer#2 · Подволна 1.10
- **Описание**: `weekly.go` — NSM/S7 из `sessions.csv`, **только `actor_kind=human`** (сессии `ci`/`sim` не засчитываются); `ops/metrics/incidents.csv` — ручной журнал (`date, session_id, correlation_id, kind, manual_fix, description`; в git только заголовок), `manual_fix=true` снимает сессию со счёта «без поломки»; `--background` → `ops/metrics/background.csv` (`date, region, events, llm_calls_max_per_hour, cap, busy_ms, breaks`).
- **Файлы**: `cmd/mvctl/internal/report/weekly.go`, `ops/metrics/{incidents,background}.csv`.
- **Зависимости**: T-131.
- **Ссылки**: `design.md` §4.1, §6; `metrics.md` §6.1–§6.3; US-038 (критерии 4 и 6); BR-17.
- **DoD**: `--weekly` считает только `human`-сессии (тест с подмешанными `ci`); `manual_fix_sessions ≥ 1` при строке в `incidents.csv`; заголовки CSV фиксированы (тест на регрессию формата); общий DoD §1.

### T-133: `mvctl trace <correlation_id>` · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.11 · ⚠ регистрация подкоманды — через tech-lead#1
- **Описание**: цепочка по `correlation_id` из журнала (или записи): `ReadRange` по всем `ReplayRead`-топикам + `analytics_events` + `llm_records`, фильтр `meta.correlation_id`, сортировка по `(topic, offset)`, печать цепочки с `agent{level, blueprint}`, `phase`, `prompt_hash`, `laws_version`.
- **Файлы**: `cmd/mvctl/internal/trace/**` (переиспользует `report/source.go`).
- **Зависимости**: T-130.
- **Ссылки**: `design.md` §2 (US-003 последний критерий), §4.1, **§12 п. 3** (уточнение реестра: `mvctl trace` — Must в 005-ops, `/v1/trace` — Should в memory); US-003, US-015, US-018; FR-084, FR-092; NFR-034.
- **DoD**: на записи хода печатаются все 7 звеньев: действие → `dice.rolled` → `combat.decided` → `llm.output` → `entity.updated` → `narrative.output` → `turn.completed`; события без `correlation_id` не попадают в вывод; порядок устойчив при равных офсетах; общий DoD §1.

### T-134: `mvctl llm usage` · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.12 · ⚠ регистрация подкоманды — через tech-lead#1
- **Описание**: `mvctl llm usage [--since 24h | --session <id> | --live]` — агрегат `llm_records`: `calls`, `tokens`, `cost_usd`, латентность p50/p95, разрезы по `level`/`phase`/`provider`/`model`, `cloud_calls`, `llm.output.rejected` по причинам; `--live` → `GET $MV_CORE_URL/v1/admin/llm/usage` (C-06).
- **Файлы**: `cmd/mvctl/internal/llmusage/**`.
- **Зависимости**: T-130; C-07 (записи EPIC-003 I1a), C-06 (admin-порт `core`, EPIC-003 I1b) — до их готовности работает на фикстурах.
- **Ссылки**: `design.md` §2 (US-012), §4.1; **US-012** (критерий 1 и «внешних вызовов = 0»); FR-070…FR-072; NFR-032.
- **DoD**: агрегаты совпадают с эталоном на фикстурах; `cloud_calls = 0` для базовой конфигурации (проверка по метрике провайдера); `--live` корректно обрабатывает недоступный `core` (сообщение, exit ≠ 0, без паники); общий DoD §1.

### T-135: `mvctl golden check|update` · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.13 · ⚠ регистрация подкоманды — через tech-lead#1
- **Описание**: `golden check <scenario>` — прогон записи `testdata/recordings/<scenario>.jsonl` через `testkit/harness` in-process (`--contexts=all --bus=memory --mode=replay`), сбор доменных событий, нормализация (нестабильные поля: `event_id`, wall-clock), сравнение с `testdata/golden/<scenario>.expected.jsonl`; отчёт **«ход/раунд и первое отличающееся событие»**; проверки: схема (`contracts.Validate`), язык (`llm.filter` Language, EPIC-003), числа (`dice`/`hp`), статус фильтра (a); публикация `analytics.replay.completed mode=test {events_hash_match, incomplete_record}`. `golden update` — перегенерация эталона (осознанная задача с ревью диффа).
- **Файлы**: `cmd/mvctl/internal/golden/**`; PR в `schemas/events/analytics.replay.completed.v1.json` (поля `mode=test`) — **с ревью EPIC-002**.
- **Зависимости**: T-130; первая запись `solo-30.jsonl` (EPIC-003 I1a); T-060 (`internal/replay`, EPIC-002).
- **Ссылки**: `design.md` §3 п. 3, §4.1; ADR-010 п. 1, доп. п. 2; **NFR-065**, NFR-061; US-017 (критерий «отчёт указывает ход/раунд и событие расхождения»).
- **DoD**: подложенное отличие в эталоне → команда возвращает ≠ 0 и печатает ход/раунд и первое отличие; неполная запись → `incomplete_record=true` и понятная ошибка, живая модель **не вызывается**; `golden update` меняет только целевой файл; общий DoD §1.

### T-136: Golden-набор `solo-30` и CI job · Размер: S · Статус: todo · Исполнитель: developer#2 · Подволна 1.14
- **Описание**: эталон `testdata/golden/solo-30.expected.jsonl` (20 ходов + 3 фоновых тика из записи EPIC-003, только `actor_kind=ci`); включение `mvctl golden check solo-30` в CI job `e2e`; проверка, что `.gitattributes` содержит `testdata/recordings/*.jsonl merge=binary` и правило распространено на `testdata/golden/*.jsonl`.
- **Файлы**: `testdata/golden/**`, `.github/workflows/go.yml` (шаг в job `e2e` — ⚠ через tech-lead#1), `.gitattributes` (⚠ через tech-lead#1).
- **Зависимости**: T-135; запись `solo-30.jsonl` (EPIC-003).
- **Ссылки**: `design.md` §4.1, §6, §11 (риск конфликтов golden); ADR-010; **NFR-065**, NFR-048 (0 ложных срабатываний фильтра (a) на золотом наборе).
- **DoD**: `mvctl golden check solo-30` зелёный в CI и локально; набор содержит 20 ходов + 3 тика; `git check-attr merge testdata/golden/solo-30.expected.jsonl` = `binary`; общий DoD §1.

### T-137: `session-report --audit` и `analytics.consistency.violated` · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 1.15
- **Описание**: `report/audit.go` — сверка через **`internal/state/audit.Recompute`** (EPIC-002 T-069) и `entity.StateHash`: `hash(снапшот + факты)` против `hash(объектов entities-{world})` из `objstore`; при расхождении — публикация `analytics.consistency.violated code=state_divergence severity=break detected_by=mvctl`; **режим `partial`** (только хэш снапшота против объектов, без фактов) с явной пометкой в отчёте — используется, если EPIC-002 T-069 ещё не слит.
- **Файлы**: `cmd/mvctl/internal/report/audit.go`; схема `schemas/events/analytics.consistency.violated.v1.json` (создана в T-009, владелец — EPIC-005).
- **Зависимости**: T-131; **EPIC-002 T-069** (`internal/state/audit`).
- **Ссылки**: `design.md` §3 п. 2, §4.1, §5, §11 (риск «`--audit` до I2-4»); ADR-011; **US-038** (критерий `--audit`); FR-086; NFR-032.
- **DoD**: integration (`-tags integration`, MinIO testcontainers + `Journal` redpanda) — совпадение даёт `state_divergence = 0`, подложенное расхождение публикует `analytics.consistency.violated` с указанием сущности/поля; событие валидно против схемы; режим `partial` явно помечен в Markdown и JSON; общий DoD §1.

### T-138: Наполнение `mvctl contracts check` · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 2.1
- **Описание**: довести каркас (T-010) до проверок `foundation design` §4.1: (а) каждая схема компилируется (2020-12); (б) каждый `Spec` имеет файл схемы и наоборот; (в) каждый тип имеет издателя и ≥ 1 потребителя по `Publishers/Consumers` (исключения `world.law_breach.*`, `rules.change.*`); (г) политики топиков на фикстурах (`player_events` отклоняет `actor_kind=system`/`meta.agent`); (д) `--format=rpk` печатает команды `redpanda-init`; отчёт «фантомные типы/топики».
- **Файлы**: `cmd/mvctl/internal/contracts/**`.
- **Зависимости**: T-009, T-010 (волна 0); полный реестр после слияния EPIC-003/EPIC-004 I1.
- **Ссылки**: `design.md` §4.1 (строка `contracts/`); `epics/EPIC-001-foundation/design.md` §4.1; C-01; критерий готовности EPIC-001 «`contracts check` без фантомов».
- **DoD**: unit на синтетическом реестре с подложенным фантомом (тип без схемы, схема без типа, тип без потребителя) — команда возвращает ≠ 0 с перечнем; на реальном реестре — 0 находок; `--format=rpk` даёт команды, совпадающие с `build/redpanda-init.sh` (тест сравнения); общий DoD §1.

### T-139: `mvctl privacy scan` и тест NFR-041 · Размер: S · Статус: todo · Исполнитель: developer#2 · Подволна 2.2 · ⚠ регистрация подкоманды — через tech-lead#1
- **Описание**: `mvctl privacy scan <path>` — поиск числовых внешних ID (Telegram user id), username и текста игроков в `testdata/`, `ops/metrics/*.csv`, логах бота; тест NFR-041 читает `gateway.db` (и структуру `links.db`, WAL) **только на предмет отсутствия утечек**; замена заглушки privacy-scan из T-012 в job `security`.
- **Файлы**: `cmd/mvctl/internal/privacy/**`, тест `internal/gateway` не трогать (чтение по `ownership.md`).
- **Зависимости**: T-010; `gateway.db`/`links.db` (EPIC-004 I1).
- **Ссылки**: `design.md` §2 (NFR-041), §8; ADR-009; T-5 (threat-model); **NFR-041**, NFR-040.
- **DoD**: подложенный внешний ID в `testdata/` → команда ≠ 0 с указанием файла и строки; на реальных данных — 0 находок; job `security` использует настоящую команду вместо заглушки; общий DoD §1.

### T-140: Закрепление порогов NFR «после замера» (стенд, совместно с BA) · Размер: S · Статус: todo · Исполнитель: developer#2 + business-analyst + пользователь (**стенд**) · Подволна 2.5–2.6
- **Описание**: по `ops/metrics/baseline.md` (T-013 EPIC-001), первым прогонам S1/S2 на стенде и замеру T-071 (EPIC-002) закрепить численные значения порогов **NFR-030, NFR-032, NFR-034** и уточнить NFR-001/NFR-002/NFR-070 в `requirements/nfr.md`; отметить, какие пороги стали «advisory».
- **Файлы**: `requirements/nfr.md` (правит BA), `ops/metrics/baseline.md` (ссылка), запись в `journal.md`.
- **Зависимости**: T-131, T-134; T-013 (EPIC-001), T-071 (EPIC-002); первые прогоны S1/S2.
- **Ссылки**: `design.md` §2 (строка NFR-030/032/034), §9 (строка «стенд»); `epics.md` §2 (005-ops, «закрепление порогов … совместно с BA»).
- **DoD**: в `nfr.md` нет формулировок «после замера» для NFR-030/032/034; каждое значение имеет ссылку на источник (`baseline.md` / прогон S1 / замер T-071); изменения согласованы с BA и записаны в `journal.md`.

---

## 3. Часть 005-memory (Should, **отрезаемо**) — задачи T-141…T-148

Подволны 2.1–2.6 (`design.md` §4.2). Порядок: каркас + `document`/`indexer` → Qdrant → Neo4j → HTTP → `absence`/`trace`/`rebuild` → e2e «сводка из памяти = сводка из журнала». **developer#2 занят 005-ops до подволны 2.3** (T-138, T-139 + резерв на долги ops), поэтому нитки не строго параллельны: developer#1 ведёт основную линию, developer#2 подключается на `rebuild` (2.4) и `/v1/trace` (2.5).

> **Пометка «отрезаемо»**: любая из задач T-141…T-148 может быть снята на G3/G4 без потери Must-историй. Условие отрезаемости проверяется по построению: e2e S1/S14 EPIC-003 гоняются **без** контекста `memory` (`MV_MEMORY_ENABLED=false` в CI), рой использует `NopMemory` → `journalContext`. При отрезании `api/memory.openapi.yaml` остаётся как спецификация будущего, `MV_MEMORY_URL` пуст.

### T-141: `internal/memory` — каркас контекста, `document`, `indexer`, интерфейсы, `Embedder` · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 2.1
- **Описание**: `context.go` — `runtime.Context "memory"` (`DependsOn []`, `Start`: `EnsureCollections`/`EnsureConstraints`, `Subscribe` группой `core.memory` на `world_events`, `game_events`, `system_events`; `Routes(mux)`); `indexer.go` — перенос as-is `services/semantic-memory/indexer.go` с заменой типов на конверт `meta` и `event.Path()`; `document.go` — текст документа по типу события (`narrative.output.text` → `generated`; `entity.updated` → `validated`; описание блупринта → `authored`); `vector.go` / `graph.go` — интерфейсы `VectorStore` / `GraphStore`; `embed.go` — `Embedder` над `llm.Provider.Embed` (C-15) с кэшем по `sha256(text)`.
- **Файлы**: `internal/memory/{context,indexer,document,vector,graph,embed}.go` + тесты с in-memory фейками.
- **Зависимости**: 005-ops не блокирует; C-15 (`llm.Provider`, EPIC-003 I1a); **разрешение импорта `internal/memory → internal/llm`** (`design.md` §12 п. 1) — до решения `Embedder` за интерфейсом, реализация подставляется в `cmd/multiverse`.
- **Ссылки**: `design.md` §3 п. 4, §4.2, §11 (риск импорта); ADR-001 п. 3; C-09, C-15; FR-035, FR-093.
- **DoD**: unit — шаблоны `document` по каждому индексируемому типу; ранжирование `authored > validated > generated`; `Embedder` кэширует повтор (1 вызов провайдера на 2 одинаковых текста); контекст `memory` включается только при `MV_MEMORY_ENABLED=true` (тест: при `false` `--contexts=all` его не поднимает); общий DoD §1.

### T-142: Qdrant-адаптер (`qdrant.go`) · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 2.2
- **Описание**: реализация `VectorStore` на `qdrant/go-client v1.19.x`: коллекции `events-{world}`, `entities-{world}` с суффиксом модели эмбеддинга; payload `world`, `scope`, `source`, `layer`, `at`, `event_id` с payload-индексами; `Upsert`, `Search(scope, query, k)`, `Delete`; асинхронная очередь эмбеддингов (индексация не блокирует подписку), `index_lag` в `/health`.
- **Файлы**: `internal/memory/qdrant.go` + тесты (`-tags integration`, testcontainers `${QDRANT_IMAGE}`).
- **Зависимости**: T-141.
- **Ссылки**: `design.md` §4.2, §6, §11 (риск «размерность привязана к модели»); ADR-004; `infrastructure.md` §2.4, §5.3.
- **DoD**: integration — upsert/search с фильтром по `scope` и `source`; **индексация ≤ 2 с** от публикации события (US-036); при превышении — `/health degraded` без потери события; смена `MV_MEMORY_EMBED_MODEL` меняет имя коллекции (тест); общий DoD §1.

### T-143: Перенос Neo4j-индексера (`neo4j.go`, без APOC) · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 2.3
- **Описание**: перенос as-is `services/semantic-memory/neo4j.go` в `internal/memory` как реализация `GraphStore` (`UpsertEntity`, `UpsertRelation`, `Neighbors`, `Trace`): замена типов событий на конверт `meta`, constraints/indexes создаются кодом при старте, **без плагина APOC** (T-17); тесты as-is переносятся за тег `integration`.
- **Файлы**: `internal/memory/neo4j.go` + тесты (`-tags integration`, testcontainers `${NEO4J_IMAGE}`).
- **Зависимости**: T-141.
- **Ссылки**: `design.md` §3 п. 4, §4.2, §10 (отклонённая альтернатива «переписать с нуля»); ADR-004 доп. п. 5; T-17 (threat-model); `infrastructure.md` §5.3.
- **DoD**: integration на Neo4j 5.26 без APOC — constraints создаются идемпотентно, relations и `Trace` работают; `compose-lint` подтверждает отсутствие `NEO4J_PLUGINS`; `MV_NEO4J_PASSWORD` обязателен (`${VAR:?}`); общий DoD §1.

### T-144: HTTP `/v1/context/scope`, `/health`, `api/memory.openapi.yaml` · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 2.4
- **Описание**: `http.go` — `POST /v1/context/scope` (факты `authored/validated/generated` с ранжированием и полем `source`; `generated` помечены и обрезаны ≤ 500 симв. — T-9), `GET /health` (`index_lag`, состояние Qdrant/Neo4j); спецификация `api/memory.openapi.yaml`; ответы **без внешних ID**; `player.said.text` индексируется как `layer=player`, `source=validated` и отдаётся только в том же scope (флаг `MV_MEMORY_INDEX_PLAYER_TEXT`).
- **Файлы**: `internal/memory/http.go`, `api/memory.openapi.yaml`.
- **Зависимости**: T-142, T-143.
- **Ссылки**: `design.md` §4.2, §8; **C-09**; T-9, SEC-17; FR-035, FR-093; US-005 (критерий 3).
- **DoD**: `contracts` job проверяет соответствие `api/memory.openapi.yaml` маршрутам (`kin-openapi`); ответ содержит `source` у каждого факта, `generated` обрезаны; ни один ответ не содержит внешних ID (тест); `/health` даёт `degraded` при `index_lag > MV_MEMORY_INDEX_LAG_MAX`; общий DoD §1.

### T-145: `/v1/context/absence` — сводка «пока тебя не было» · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 2.5
- **Описание**: `absence.go` — события `world.*`/`region.*` с `actor_kind=system` за `(region, since_at, until_at)` → сводка по шаблону **без LLM**; `POST /v1/context/absence`.
- **Файлы**: `internal/memory/absence.go`, дополнение `api/memory.openapi.yaml`.
- **Зависимости**: T-144.
- **Ссылки**: `design.md` §2 (FR-127), §4.2, §4.3; C-09; **FR-127**; US-036, US-005 (Should-часть).
- **DoD**: unit на фиктивных фоновых событиях — сводка содержит все события окна и не содержит игровых действий игроков; пустое окно → пустая сводка без ошибки; общий DoD §1.

### T-146: `/v1/trace/{cid}` (Should) · Размер: S · Статус: todo · Исполнитель: developer#2 · Подволна 2.5
- **Описание**: `trace.go` — цепочка по `correlation_id` из Neo4j (`CAUSED_BY`) с fallback на `Journal`; `GET /v1/trace/{cid}`.
- **Файлы**: `internal/memory/trace.go`, дополнение `api/memory.openapi.yaml`.
- **Зависимости**: T-143, T-144.
- **Ссылки**: `design.md` §2 (US-015 Should), §10 (отклонённая альтернатива «`/v1/trace` только в memory»); US-015, US-018; NFR-034.
- **DoD**: результат совпадает с выводом `mvctl trace` (T-133) на том же `correlation_id` (тест сравнения); при недоступном Neo4j — fallback на журнал без ошибки; общий DoD §1.

### T-147: `mvctl memory rebuild` · Размер: M · Статус: todo · Исполнитель: developer#2 · Подволна 2.4 · ⚠ регистрация подкоманды — через tech-lead#1
- **Описание**: `rebuild.go` — очистка коллекций → `ReadRange` всех топиков с earliest → индексация; `--world`; `--blueprints` добавляет `authored`-факты из `blueprints/*.md` (`## description`, `## canon`) через парсер `shared/agent` (EPIC-003); тонкая обёртка `cmd/mvctl/internal/memory/rebuild.go`.
- **Файлы**: `internal/memory/rebuild.go`, `cmd/mvctl/internal/memory/**`.
- **Зависимости**: T-142, T-143; парсер блупринтов (EPIC-003 I1a, C-11).
- **Ссылки**: `design.md` §4.2, §6; C-11; ADR-004; NFR-064.
- **DoD**: `rebuild` на записи восстанавливает индекс до состояния, при котором `/v1/context/scope` даёт тот же набор фактов, что после онлайн-индексации (тест сравнения); обязателен при смене `MV_MEMORY_EMBED_MODEL` (проверка версии коллекции); `--blueprints` добавляет `authored`-факты; общий DoD §1.

### T-148: e2e памяти, деградация и профиль compose `memory` · Размер: M · Статус: todo · Исполнитель: developer#1 · Подволна 2.6
- **Описание**: e2e «сводка из памяти = сводка из журнала»: запись S14 → `memory rebuild` → `/v1/context/absence` ⊇ фоновые события `journalContext`; e2e деградации: контекст `memory` остановлен → `narrative.output` доставляется без ошибки, в логе `memory_fallback`, `session-report` показывает `memory_fallback_rate`; проверка профиля compose `memory` (Qdrant + Neo4j + процесс `memory`).
- **Файлы**: `test/e2e/memory_test.go`, дополнение `docker-compose.yml` (профиль `memory` — ⚠ через tech-lead#1).
- **Зависимости**: T-145, T-147, T-131.
- **Ссылки**: `design.md` §4.3 (таблица деградации), §9 (строка e2e); C-09; US-005, US-036; критерий готовности 005-memory.
- **DoD**: сводка из памяти совпадает со сводкой из журнала по составу событий; при остановленной памяти игрок получает нарратив (0 ошибок), лог содержит `memory_fallback`; `MV_MEMORY_ENABLED=false` в CI — все Must-e2e проходят **без** контекста `memory` (проверка отрезаемости); `make compose-lint` зелёный; общий DoD §1.

---

## 4. Стендовые задачи EPIC-005 (stand)

| Задача | Что на стенде | Когда | Кто |
|---|---|---|---|
| T-131 / T-134 (часть) | `session-report` и `llm usage` на **живой сессии S1**, затем S2 (Redpanda + MinIO, не запись) | интеграция I1 / I2 | developer#2 + пользователь |
| T-137 (часть) | `--audit` на живом мире после сессии: снапшот + журнал ↔ объекты `entities-{world}` | интеграция I2 | developer#2 + пользователь |
| **T-140** | закрепление порогов NFR-030/032/034 по `baseline.md` (T-013) и прогонам S1/S2 | подволна 2.4–2.6 | developer#2 + BA + пользователь |
| T-142 (часть) | индексация на живом Ollama-эмбеддинге (`nomic-embed-text` / `bge-m3`), измерение `index_lag` | подволна 2.5 | developer#1 + пользователь |
| T-148 (часть) | `make up PROFILES=memory` и деградация при остановленной памяти | подволна 2.6 | developer#1 + пользователь |

---

## 5. Зависимости, заглушки и допущения

| Что нужно | От кого | Заглушка до готовности | Влияние на сроки |
|---|---|---|---|
| C-10 (`analytics.session.*`, `turn.completed`) | EPIC-004 I1 | фикстуры `testdata/analytics/*.jsonl` (T-130) | нет: 005-ops стартует на фикстурах |
| C-07 (`llm.output`, записи `testdata/recordings`) | EPIC-003 I1a | фикстуры; golden — после первой записи `solo-30.jsonl` | T-135/T-136 сдвигаются за записью |
| C-06 (`GET /v1/admin/llm/usage`, `POST /v1/admin/agents/{id}/tick`) | EPIC-003 I1b | только журнал (`--live` недоступен) | нет |
| `internal/state/audit` (`Recompute`, `Compare`) | EPIC-002 T-069 | `--audit` в режиме `partial` | T-137 в подволне 2.3 |
| C-15 (`llm.Provider.Embed`) | EPIC-003 I1a | `providers/fake.Embed` (детерминированный хэш-вектор) | 005-memory |
| C-11 (парсер блупринтов) | EPIC-003 I1a | — | `rebuild --blueprints` |
| Разрешение импорта `internal/memory → internal/llm` | **system-architect** (`design.md` §12 п. 1) | `Embedder` за интерфейсом, реализация в `cmd/multiverse` | не блокирует, но решение нужно до подволны 2.4 |

**Допущения**: (1) `mvctl trace` — Must в 005-ops (уточнение реестра `epics.md` §1, запрос `design.md` §12 п. 3); (2) схема `analytics.consistency.violated` создана в F-4b (T-009) с владельцем EPIC-005 (запрос §12 п. 2); (3) `player.said.text` индексируется как `validated` факт scope с флагом отключения `MV_MEMORY_INDEX_PLAYER_TEXT=false`; (4) право на забвение: `/forget` удаляет связку в `links.db`, перестройка памяти не требуется (ADR-009), полное забвение — EPIC-013; (5) CSV в git вместо БД метрик (`metrics.md` §6.2) — на 8 сессий MVP-1 достаточно.
