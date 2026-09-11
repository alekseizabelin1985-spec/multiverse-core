# Сведение запросов на изменение контрактов и замечаний (A3 шаг 4) — материал для G2

Версия 0.1 · 2026-09-09 · system-architect#1.
Входы: `components/state-and-mechanics.md` §14–15 (architect#1), `components/gateway-and-bot.md` §14, §16 (architect#3), `components/swarm-llm-laws.md` §5, §14, §19 (architect#2), `components/foundation.md` §11–13 (architect#1), `threat-model.md` §8–9 (security-engineer), `infrastructure.md` §11 (devops-engineer), `journal.md` (решения пользователя 2026-09-09).
Выходы: `contracts.md` v0.2, дополнения «2026-09-09» в ADR-001/004/005/006/007/009/010, ADR-021, `overview.md` §13/§15/§16/§18/§19/§21, `plan/ownership.md` v0.2, `project/open-questions.md` (OQ-A-17/18/19 решены, OQ-A-20 новый).

Статусы: **П** — принято как предложено; **ПИ** — принято с изменением; **О** — отклонено; **BA** — требование, передано бизнес-аналитику (архитектурная часть принята); **К** — принято к сведению (решения не требует).

## 1. Решения пользователя (журнал 2026-09-09)

| # | Решение | Отклонение от рекомендации | Куда внесено |
|---|---|---|---|
| U-1 | OQ-A-17: весь заменяемый код → `services/_archive/`, ничего не удалять | да (рекомендация была «удалить нерабочее») | ADR-001 дополнение п. 5 (принцип «архив вместо удаления»); `overview.md` §16, §19; `ownership.md` §1; `open-questions.md` OQ-A-17 |
| U-2 | OQ-A-18: одна `qwen3:30b-a3b`, если проходит замер; иначе 8b+14b | нет | ADR-005 дополнение п. 2; `overview.md` §15, §18.1; OQ-A-18 |
| U-3 | OQ-A-19: retention 30/90/180, промпты по флагу | нет | ADR-007 дополнение п. 1; ADR-004 дополнение п. 3; OQ-A-19 |
| U-4 | 3 команды (TEAM-1/2/3) | нет | `epics.md` §4 без изменений; tech-lead — `plan/teams.md` |
| U-5 | Ключ в `shared/oracle/README.md` реальный — пользователь отзывает; плейсхолдер в F-1 | — | ADR-009 дополнение п. 10; `overview.md` §16, §19 |
| U-6 | OneDrive `Documents` не синхронизирует | — | ADR-004 дополнение п. 4 (SEC-30 снят) |
| U-7 | Допуск тестеров — allowlist Telegram user id | — | ADR-006 дополнение п. 1; ADR-009 дополнение п. 1; `overview.md` §13 |

## 2. Запросы architect#1 — `state-and-mechanics.md` §14–15 (EPIC-002)

| # | Запрос | Источник | Решение | Куда внесено |
|---|---|---|---|---|
| S-1 | C-01: `Journal{ReadRange, Tail, End}`, `PositionFromContext`, `eventbus.Dedup` в production-пакете | §14 п. 1 | **П** | C-01 v1.1; ADR-007 дополнение п. 5; `foundation.md` §5.3 уже совпадает |
| S-2 | ADR-001 п. 3: разрешить `state → mechanics`, `cmd/multiverse → internal/replay`; пакеты `shared/clock`, `shared/runtime` в ownership | §14 п. 2 | **П** | ADR-001 дополнение п. 2–3; `ownership.md` §1; C-01 (часы/рантайм) |
| S-3 | C-03: совместимые дополнения Go-API | §14 п. 3 | **П** | C-03 v1.1 |
| S-4 | C-02: семантика `rest`, `atomic=false`, `applied_at`, `duplicate_entity`, `append` | §14 п. 4 | **П** | C-02 v1.1 |
| S-5 | C-14: `latest.json` — указатель; `component ∈ state|swarm|gateway` (правка `api-contracts.md` §2.3.12) | §14 п. 5 | **П** | C-14 v1.1; правка `api-contracts.md` — задача system-analyst на планировании |
| S-6 | Схема `analytics.replay.completed` — совладение EPIC-002/005 | §14 п. 6 | **П** | `ownership.md` §1; C-10/C-14 |
| S-7 | inv-04 применяется к `alive` участникам группы | §15 | **BA** (архитектурно — да: `members[]` сохраняет мёртвых формально, UC-009 A1) | BA — уточнить NFR-020 inv-04 |
| S-8 | Один atomic-пакет на раунд соло vs по одному на `combat.decided` — оба поддерживаются | §15 | **К** | C-03 v1.1 (рекомендация — один пакет) |
| S-9 | Интент как WAL ~40 мс на пакет; вариант B ADR-013 при провале NFR-001 | §15 | **К** | без изменений контрактов; замер на стенде I2 |
| S-10 | Снапшот по `analytics.session.ended` требует чтения `analytics_events` | §15 | **К** (оставлено по ADR-003; группа `core.state.triggers`, ADR-011 п. 4) | — |

## 3. Запросы architect#3 — `gateway-and-bot.md` §14, §16 (EPIC-004)

| # | Запрос | Источник | Решение | Куда внесено |
|---|---|---|---|---|
| G-1 | C-08: коды `403 client_unknown`, `403 client_mismatch`, `409 no_open_round`, `501 not_implemented` | §14 п. 1 | **П** (+ `403 actor_kind_forbidden`, `409 already_acted`, `429 rate_limited`, `413 payload_too_large`, `409 poll_in_progress` из §5.1/SEC-11) | C-08 v1.1 |
| G-2 | C-08: `202 {status: pending, group_id}` для `group.*` | §14 п. 2 | **П** | C-08 v1.1 |
| G-3 | C-08: `character_status=creating` | §14 п. 3 | **П** | C-08 v1.1 |
| G-4 | C-05: `encounter.started.round{timeout, idle_after_missed}` | §14 п. 4 | **П** (источник — `round` блупринта `encounter-*`/`rules`; env gateway — значения по умолчанию) | C-05 v1.1; C-11 v1.1 |
| G-5 | C-14/ownership: префикс `snapshots-{world}/gateway/` | §14 п. 5 | **П** | C-14 v1.1; `ownership.md` §1; `contracts.md` §0 (тип `snapshot.created component=gateway` — EPIC-004) |
| G-6 | C-04: `say` в группе не открывает раунд и не входит в `acted[]` (вариант E2) | §14 п. 6 | **П** как уточнение семантики + **BA** (FR-025) | C-04 |
| G-7 | BR-13: лидерство при смерти лидера | §14 п. 7 | **BA** (архитектурно совместимо: `group.leader_changed {cause: death}` уже в C-04) | C-04 (упомянуто как предложение ADR-020 п. 8) |
| G-8 | ADR-006 п. 5: только личные чаты в MVP-1, общий чат группы — E-H | §14 п. 8 | **П** | ADR-006 дополнение п. 2 |
| G-9 | C-01: признак «конец журнала» (`Bus.Lag()` или событие membus) | §14 п. 9 | **ПИ** — отдельного `Lag()` нет; «конец журнала» = `Journal.End()` по всем читаемым топикам; `membus` реализует `End` | C-01 v1.1; C-14 v1.1 |
| G-10 | Допущения §16 (одна доставка в лизинге; проекция только в памяти; `idle` в двух местах; `turns_count` из `sessions`; бот без курсора) | §16 | **К** — в границах блока, контрактам не противоречит | — |
| G-11 | Env `GATEWAY_*`, `BOT_*`, `TELEGRAM_BOT_TOKEN`, `MODE` без префикса | §11.3 | **ПИ** — читать с префиксом `MV_` (`MV_GATEWAY_LISTEN`, `MV_TELEGRAM_BOT_TOKEN`, `MV_MODE`…); architect#3 правит при нарезке задач *(имя заменено: `MV_GATEWAY_LISTEN` → `MV_CORE_ADDR`, см. T-408/T-416)* | `contracts.md` (сводка Env, §16 п. 5) |

## 4. Запросы architect#2 — `swarm-llm-laws.md` §5, §14, §19 (EPIC-003)

| # | Запрос | Источник | Решение | Куда внесено |
|---|---|---|---|---|
| W-1 | C-11: glob в `trigger.event_name` (`player.*`) | §5.1, ADR-015 п. 3 | **П** (glob по сегментам, `*` не пересекает точку; валидатор требует ≥ 1 совпадения по реестру) | C-11 v1.1 |
| W-2 | Схемы: `llm.output.parse{strategy, recovered}`, `narrative.output.narrative_event_id`, `tick.fired.tick.lod_allowed` обязателен, `agent.spawned.content_hash` | §14 | **П** (совместимые) | C-05/C-06/C-07 v1.1 |
| W-3 | Env `SWARM_*`, `LLM_*`, `LAWS_*`, `MEMORY_URL`, `GM_PATH` без префикса | §14 | **ПИ** — с префиксом `MV_` (`MV_SWARM_ADMIN_ADDR` → заменяется на `MV_CORE_ADDR` процесса: HTTP-сервер принадлежит `shared/runtime`, контекст лишь монтирует маршруты) | C-06; ADR-005 дополнение п. 1; ADR-001 дополнение п. 4 |
| W-4 | `validation_status=error error.code=yielded` (уступка фона, ADR-014 п. 2) | ADR-014 | **П** (совместимое расширение C-07) | C-07 v1.1 |
| W-5 | `Provider.Models()` для валидатора моделей; блупринт не выбирает провайдера | SEC-21 → C-11/C-15 | **П** | C-11 v1.1; C-15; ADR-005 дополнение п. 3 |
| W-6 | Допущения §19 (нет чтения по диапазону времени; число внешних игроков — флаг оператора; стартовые модели до baseline; `strain` в памяти) | §19 | **К** — `Journal` даёт диапазон по офсетам, не по времени; сводка фона из индекса роя остаётся | — |

## 5. Запросы architect#1 — `foundation.md` §11–13 (EPIC-001)

| # | Запрос | Источник | Решение | Куда внесено |
|---|---|---|---|---|
| F-1 | Судьба кода: удалить `shared/config`, `shared/schema`, `shared/redis`, `test_minio.go`, `fake_deps`, `tools/*`; заморозить 8 сервисов; narrative-orchestrator отдельным `go.mod` для `legacy` | §11 | **ПИ** — по решению пользователя U-1: **ничего не удалять**, всё выводимое из сборки → `services/_archive/<путь>` с `ARCHIVED.md`; из индекса удаляются только не-код (бинарники, логи, `.claude/worktrees/*`, секреты); заморозка 8 сервисов и профиль `legacy` — как предложено | ADR-001 дополнение п. 5–6; `overview.md` §16, §19 |
| F-2 | Порядок F-1…F-9, разбиение F-4 на a/b/c, contract-тест шины до волны 1 | §12 | **П**; F-2 — `go 1.26` (не 1.25); F-1 первым шагом — `.claude/worktrees/*`; F-6 — `build/minio.Dockerfile`, `versions.env`, профиль `bot`; F-5 — `objstore` по ADR-021 | ADR-001/004/010 дополнения; tech-lead#1 — `epics.md` §6 |
| F-3 | Новые пакеты `shared/clock`, `shared/runtime` | §3–4 | **П** | `ownership.md` §1; C-01 v1.1 |
| F-4 | `go.mod` зависимости (kafka-go, minio-go, jsonschema/v6, yaml.v3, testify, testcontainers, modernc sqlite, qdrant, neo4j) | §11 | **П** (+ `goose` v3 по ADR-019, `go-telegram/bot` по ADR-018, `oklog/ulid`) | — (по факту в F-2/F-4) |
| F-5 | Риски каркаса §13 | §13 | **К** | — |

## 6. Замечания security-engineer — `threat-model.md` §8

| # | Серьёзность | Замечание | Решение | Куда внесено |
|---|---|---|---|---|
| T-1 | Major | Allowlist Telegram user id; команды только из личных чатов; связка по `from.id` | **П** (решение пользователя U-7) | ADR-006 дополнение п. 1–2; ADR-009 дополнение п. 1; `overview.md` §13, §18 |
| T-2 | Major | Идемпотентность `POST /v1/characters` без `external_id` в `gateway.db` (`link_id` или HMAC) | **П** — `link_id` (ULID); `character_requests` в `links.db`; HMAC не нужен | ADR-009 дополнение п. 2; C-08 v1.1; `overview.md` §13 |
| T-3 | Major | Непубликация **всех** портов + `compose-lint` | **П** | ADR-009 дополнение п. 3; ADR-010 дополнение п. 4; `overview.md` §13 |
| T-4 | Major | Редакция токена бота в логах/ошибках | **П** | ADR-006 дополнение п. 3; ADR-009 дополнение п. 4 |
| T-5 | Major | Записи/golden только `actor_kind=ci`; CI privacy-scan `testdata/` | **П** | ADR-010 дополнение п. 1; C-07 v1.1 |
| T-6 | Minor | `{client_id}` == `X-Client-Id`; `route.external_id` только своей платформе | **П** | C-08 v1.1; ADR-009 дополнение п. 6 |
| T-7 | Minor | Модель блупринта против списка провайдера; блупринт не включает облако; `config.cloud_enabled` без ключей | **П** | ADR-005 дополнение п. 3; C-11/C-15 |
| T-8 | Minor | `links.db`: `secure_delete`, checkpoint+vacuum сразу после `/forget`, именованный том, бэкап-политика | **П** (уточняет ADR-019 п. 5 — sweeper остаётся страховкой) | ADR-004 дополнение п. 4; ADR-009 дополнение п. 8 |
| T-9 | Minor | Экранировать `generated`-факты памяти; инъекции второго порядка в e2e | **П** | ADR-005 дополнение п. 4; C-09 |
| T-10 | Minor | Валидация типа/схемы при чтении; отклонять `actor_kind=system`/`meta.agent` в `player_events` | **П** (флаг `MV_BUS_VALIDATE_ON_READ`, политики топиков в реестре) | C-01 v1.1; ADR-007 дополнение п. 4 |
| T-11 | Minor | Без `parse_mode` / экранирование | **П** — без `parse_mode` | ADR-006 дополнение п. 4; C-05 |
| T-12 | Minor | Rate limit (20/мин бот, 30/мин `player_id`, один long-poll) и тело 64 КБ | **П** (gateway §5.1 «2/с burst 5» заменяется на 30/мин burst 5) | C-08 v1.1; ADR-009 дополнение п. 7 |
| T-13 | Minor | `prompts-*`: versioning off, ≤ 30 дн., вне бэкапа, флаг выкл. | **П** (было 90 дн. в `infrastructure.md` §5.2 — devops правит) | ADR-004 дополнение п. 3 |
| T-14 | Minor | Уведомление FR-009: облако, retention, бэкап, `/forget` | **BA** | ADR-009 дополнение п. 11 |
| T-15 | Minor | CI: пин по SHA, `permissions`, `go mod verify`, Dependabot, CODEOWNERS | **П** | ADR-010 дополнение п. 3 |
| T-16 | Info | F-1: `.claude/worktrees/*`, плейсхолдер в `shared/oracle/README.md`, gitleaks без allowlist README/`testdata/` | **П** (allowlist только `*.example`; `infrastructure.md` §4.5 п. 8 — devops правит) | ADR-009 дополнение п. 10 |
| T-17 | Info | Neo4j APOC: ограничить или убрать | **П** — не включать (код не использует, проверено) | ADR-004 дополнение п. 5; `ownership.md` §1 |
| T-Q | — | Вопросы владельцу (OneDrive; ключ в README; allowlist vs инвайт) | закрыты пользователем (U-5, U-6, U-7) | — |

## 7. Замечания devops-engineer — `infrastructure.md` §11

| # | Замечание | Решение | Куда внесено |
|---|---|---|---|
| D-1 | Go 1.25 вне поддержки → 1.26 сразу в EPIC-001 | **П** — `go 1.26` + `toolchain go1.26.x`; `govulncheck` блокирующий | ADR-001 дополнение п. 1; `overview.md` §15, §19; ADR-010 дополнение п. 5 |
| D-2 | MinIO: образы не публикуются — (а) сборка из исходников / (б) замена / (в) как есть | **П** — (а) на MVP-1 с минимальным `objstore` и триггерами замены; подтверждение владельца — **OQ-A-20** (замена сервера влияет на стек) | **ADR-021**; ADR-004 дополнение п. 1; `overview.md` §13, §15, §21 |
| D-3 | Профиль `legacy` конфликтует с удалением Chroma | **ПИ** — вариант (а): as-is `semantic-memory` + `chromadb` живут в профиле `legacy` (порт 8083) до S5; деградации в коде оркестратора нет (проверено `orchestrator.go:117,177`) | ADR-001 дополнение п. 6; ADR-004 дополнение п. 8; `overview.md` §13, §16, §19 |
| D-4 | Префикс `MV_` для env в ADR-005/009 | **П** — правило для всех платформенных переменных; сторонние без префикса | ADR-005 дополнение п. 1; `contracts.md` (Env, §16 п. 5); `overview.md` §13, §18 |
| D-5 | JSON Schema 2020-12 — `santhosh-tekuri/jsonschema/v6` | **П** | C-01 v1.1; ADR-007 дополнение п. 3; `overview.md` §15 |
| D-6 | Versioning/ILM в `objstore.EnsureBucket` | **П** | ADR-004 дополнение п. 2; `contracts.md` §0 |
| D-7 | HTTP-порт у `core` (`MV_CORE_ADDR`, предложено 8081) | **ПИ** — `MV_CORE_ADDR=127.0.0.1:8090` (порт 8090 уже в C-06, swarm §14, gateway §11.3; 8081 исторически Schema Registry); сервер — у процесса (`shared/runtime`), контексты монтируют маршруты; admin — раздел `admin` в `gateway.openapi.yaml`; devops правит `infrastructure.md` §1.4, §4.2 | C-06; ADR-001 дополнение п. 4; `overview.md` §13 |
| D-8 | Том Redpanda не обновляется скачком — пересоздание в плане миграции | **П** (+ `make archive-legacy` до пересоздания) | ADR-004 дополнение п. 6; `overview.md` §19 |
| D-9 | `segment.ms=1d` в ADR-007 п. 4 | **П** | ADR-007 дополнение п. 2; `overview.md` §13, §18 |
| D-10 | Профиль `bot` | **П** | ADR-001 дополнение п. 6; `overview.md` §13, §18 |
| D-11 | `.claude/worktrees/*` — первый шаг F-1 | **П** | ADR-009 дополнение п. 10; `overview.md` §16, §19 |
| D-12 | Логи бота «≤ 7 дней» — cron/lumberjack или простота | **ПИ** — простота: ротация по объёму (`10m × 3`), «7 дней» — ориентир; логи без ПДн по построению | ADR-006 дополнение п. 5 |
| D-13 | `build/versions.env` — один источник версий | **П** | ADR-004 дополнение п. 7; ADR-010 дополнение п. 6; `ownership.md` §1 |
| D-14 | Golden `*.jsonl`: `-diff merge=binary` или текстовый дифф | **ПИ** — `merge=binary`, текстовый diff сохраняется (ревью нарративов в PR) | ADR-010 дополнение п. 2 |

## 8. Итог

| Категория | Всего | П | ПИ | О | BA | К |
|---|---|---|---|---|---|---|
| architect#1 (state) | 10 | 6 | 0 | 0 | 1 | 3 |
| architect#3 (gateway) | 11 | 7 | 2 | 0 | 1 | 1 |
| architect#2 (swarm) | 6 | 4 | 1 | 0 | 0 | 1 |
| architect#1 (foundation) | 5 | 3 | 1 | 0 | 0 | 1 |
| security-engineer | 17 | 16 | 0 | 0 | 1 | 0 |
| devops-engineer | 14 | 10 | 4 | 0 | 0 | 0 |
| **Итого** | **63** | **46** | **8** | **0** | **3** | **6** |

Отклонённых запросов нет. «Принято с изменением»: G-9 (`Journal.End()` вместо `Bus.Lag()`), G-11/W-3 (префикс `MV_`, `MV_CORE_ADDR` вместо `SWARM_ADMIN_ADDR`), F-1 (архив вместо удаления — решение пользователя), D-3 (Chroma в профиле `legacy`), D-7 (порт 8090, не 8081), D-12 (логи бота по объёму), D-14 (`merge=binary` без `-diff`).

## 9. Что должны внести другие роли (после G2, до нарезки задач)

| Кто | Что | Основание |
|---|---|---|
| tech-lead#1 | `epics.md`: EPIC-001 — `go 1.26`, профиль `bot`, `build/minio.Dockerfile`/`versions.env`, `shared/clock`/`shared/runtime`; F-1/F-3 — «в `services/_archive/`» вместо «удалить»; «удаление ban-of-world/reality-monitor — по OQ-A-17» → решено (архив); F-8 — критерий по U-2 | ADR-001/004 дополнения, U-1 |
| devops-engineer | `infrastructure.md`: §1.4/§4.2 порт `core` 8090 (`MV_CORE_ADDR`, `MV_CORE_URL`); §1.3 профиль `legacy` = orchestrator + semantic-memory (:8083) + chromadb; §5.2 `prompts-*` ILM 30 дн.; §4.5 п. 8 gitleaks allowlist только `*.example`; §2.4 MinIO — собственная сборка (`build/minio.Dockerfile`), Go 1.26 builder; §5.3 Neo4j без APOC; §7.1 логи бота `10m × 3` | D-3, D-7, T-13, T-16, T-17, D-12, ADR-021 |
| architect#1/#2/#3 | В `components/*.md` имена env читать с префиксом `MV_`; `SWARM_ADMIN_ADDR` → `MV_CORE_ADDR`; gateway §5.1 rate limit → 30/мин burst 5; gateway §4.1 — `link_id` в `links`/`character_requests`; ADR-019 п. 5 — checkpoint/vacuum сразу после `/forget`; ADR-020 п. 1 — `round` из `encounter.started` | G-11, W-3, T-2, T-8, T-12, G-4 |
| system-analyst | `api-contracts.md`: §0/§2.1 — поля перенесены в `meta` (ADR-007); §2.3.12 `component ∈ state|swarm|gateway`; §1.6 новые коды C-08 v1.1; §1.1 идемпотентность по `link_id` | S-5, G-1..3, T-2 |
| business-analyst | FR-025 (`say` не открывает раунд), BR-13 (лидерство при смерти), NFR-020 inv-04 (`alive`), FR-009 текст уведомления (облако/retention/бэкап/`/forget`), FR-034 (+`law_violation`, `filter_blocked`) | G-6, G-7, S-7, T-14, ADR-017 п. 3 |
| security-engineer | Обновить `threat-model.md` §8: статусы замечаний по этой таблице; SEC-30 снят; SEC-06/07 — allowlist принят | §6 |
| tech-writer (F-9) | `.dev-team.json.stack` по факту после EPIC-001 (Go 1.26, MinIO из исходников, Qdrant вместо Chroma, без Timescale/Redis) | ADR-001/004/021 |

---

# Сведение 2 (A4, этап планирования) — 2026-09-09, system-architect#1

Входы: `journal.md` «Решения пользователя (DevOps)» 2026-09-09; `epics/EPIC-001-foundation/design.md` §5/§5.1, `epics/EPIC-002-state-mechanics/design.md` §5, `epics/EPIC-005-memory-ops/design.md` §12 (architect#1); `epics/EPIC-003-swarm-llm-laws/design.md` §4, §13 (architect#2); `infrastructure.md` v0.2 §6/§13 (devops); проверка llama.cpp server API и `unsloth/Qwen3.8-27B-GGUF` (2026-09-09).
Выходы: ADR-005 дополнение 2, ADR-001 дополнение п. 7–8, ADR-004 (строка про миграции), `contracts.md` v0.3 (C-05, C-06, C-10, C-15 v1.1, §0, §17), `plan/ownership.md` v0.3, `overview.md` §13/§15/§18/§18.1/§19/§21, `plan/epics.md` §1/§2/§6/§7, `open-questions.md` OQ-A-18 (уточнён), OQ-A-21 (новый, решён архитектором). Новых ADR нет (`counters.adr` = 21).

## 10. Решения пользователя (журнал 2026-09-09, DevOps)

| # | Решение | Отклонение от рекомендации | Куда внесено |
|---|---|---|---|
| U-8 | LLM на целевой машине — **нативно llama.cpp `llama-server`** (`Qwen3.8-27B-UD-Q3_K_XL.gguf`, `127.0.0.1:1234`, `--reasoning on`, `--models-dir`, `mmap+mlock`); Ollama тоже есть → провайдер по умолчанию `openai_compat`, Ollama — альтернатива; кандидат замера — Qwen3.8-27B | да: рекомендация была «Ollama нативно» (`infrastructure.md` §13 п. 1) и «Ollama native API как основной путь» (ADR-005 п. 1, `overview.md` §15). Проверено: llama-server покрывает structured output (`response_format json_schema`), учёт токенов (`usage`), здоровье (`/health`, `/v1/models`); единственное ограничение — thinking надо выключать на запрос (llama.cpp #20345) — совместимо с `Params.Think=false` MVP-1 | ADR-005 доп. 2 п. 1–9; C-15 v1.1; `overview.md` §13/§15/§18/§18.1/§19/§21; `epics.md` F-6/F-8; OQ-A-18 (вариант **E**), OQ-A-21 |
| U-9 | CODEOWNERS = `alekseizabelin1985-spec`; репозиторий приватный, личный аккаунт | нет | запрос devops (§13): F-7 `CODEOWNERS`, gitleaks-action без лицензии, лимит Actions |
| U-10 | Форк `minio/minio` (тег `RELEASE.2025-10-15T17-29-55Z`) в аккаунт пользователя — в F-6 | нет | запрос devops (§13): F-6 + `build/minio.Dockerfile` собирает из форка, тарбол в `backups/` остаётся |
| U-11 | IDE/AI-каталоги в индексе — оставить, решить в F-9 | нет | tech-writer (F-9); `epics.md` §6 без изменений |
| U-12 | GPU-замер — вручную (`make bench`), без self-hosted runner | нет (допущение devops подтверждено) | `infrastructure.md` §3.3 без изменений |

## 11. Запросы архитекторов решения (design.md EPIC-001/002/003/005)

| # | Запрос | От | Решение | Обоснование / куда внесено |
|---|---|---|---|---|
| a | ADR-001 п. 3 — разрешить импорт `internal/memory → internal/llm` (только `Provider`/`Registry` для `Embed`) | architect#1 (EPIC-005 §12 п. 1) | **П** | C-15 называет EPIC-005 потребителем; альтернатива (контекст `llm` в процессе `memory`) тянет весь шлюз. Ограничение: только `llm.Provider`, `llm.Request/Response`, `providers.Registry`; `guardian/prompt/filter/parser`, `Gateway` — запрещены (`depguard`). → ADR-001 доп. п. 7; C-15 v1.1 |
| b | Схема `analytics.consistency.violated.v1.json` (владелец EPIC-005) — в блоке «в» F-4b | architect#1 (EPIC-005 §12 п. 2) | **П** | как `replay.completed` для EPIC-002: файл создаётся в F-4b по спецификации EPIC-005 §4.1 и передаётся владельцу. → C-10 v0.3; `epics.md` §6 F-4b; `ownership.md` §1 |
| c | `mvctl trace <cid>` по журналу — 005-ops (Must); `/v1/trace` — 005-memory (Should) | architect#1 (EPIC-005 §12 п. 3) | **П** | критерий трассы US-003/NFR-034 — Must и не должен зависеть от отрезаемой части. → `epics.md` §1/§2 EPIC-005; C-10; `ownership.md` §1 |
| d *(«`WithEncounterStub`» — заглушка не делалась, см. T-416; ниже по строке d и в §12/§13 читать как историю решения)* | `FakeNarrator.WithEncounterStub` (EPIC-001 F-10) vs `testkit/swarm.FakeEncounter` (EPIC-003 C4) — два предложения одной заглушки Phase 1 | architect#1 (EPIC-001 §5.1, EPIC-002 §5) / architect#2 (EPIC-003 §4.1, §11 п. 1) | **ПИ** — выбран **`FakeEncounter` в EPIC-003 (C4) как первая единица I1a с ранним merge `shared/testkit/swarm` в `integration/mvp-1`**; в F-10 — только `FakeNarrator` v0 без боя (`WithEncounterStub` не делается) | (1) Семантика Phase 1 (порядок `dice.rolled → combat.decided → proposed atomic`, `encounter.started/ended`, `expected_version`) — контракт EPIC-003; заглушка у владельца не расходится с ролью `encounter` (риск, названный самим architect#1 в EPIC-002 §11); (2) `template/ru.go` переиспользуется ролями роя — один текст шаблонов; (3) TEAM-2 имеет 3 слота, C4 ≈ 1 единица и стоит первой в потоке C; (4) F-10 у TEAM-1 и так на критическом пути волны 0. Риск «C4 опоздает к I1-α» → запасной вариант: временный `encounterStub` в тестах EPIC-002 (не в `shared/testkit`), эскалация через tech-lead#1. → C-05 v0.3 (заглушка), §0 исключение, §17; `ownership.md` §1; `epics.md` §2/§6 F-10 |
| e | Флаг включения фейков — `MV_SWARM_FAKE=true` контекста `swarm`, не отдельный контекст | architect#2 (EPIC-003 §13 п. 1) | **ПИ** — имя и семантика флага приняты; **читает флаг `cmd/multiverse`**, а не контекст `swarm` | на момент I1-α `internal/swarm` в интеграционной ветке отсутствует (EPIC-003 сливается последним) — контексту негде читать флаг; хук `cmd/multiverse/fake_contexts.go` регистрирует `testkit/swarm.FakeContext` под именем `swarm`; единственный разрешённый импорт `shared/testkit` в production-бинарнике, удаляется в I1. → ADR-001 доп. п. 8; C-05/§0; `epics.md` F-10 |
| f | Раздел `admin` в `api/gateway.openapi.yaml` — совладение EPIC-003 | architect#2 (EPIC-003 §7, §10 п. 11) | **П** | файл — EPIC-004, текст маршрутов `/v1/admin/agents*`, `/v1/admin/llm/usage` — EPIC-003 через PR с ревью EPIC-004 (как `replay.completed` EPIC-002/005). → C-06 v0.3; `ownership.md` §1 |
| g | Миграции SQLite — `goose` без `Down` (ADR-019) vs `golang-migrate` (infrastructure v0.1, ADR-004) | devops (infrastructure v0.2), проверка единообразия | **П** (подтверждено) | ADR-019 п. 2 — единственное решение: `pressly/goose/v3` как библиотека из embed FS, `Down` не пишутся, CLI не используется; `infrastructure.md` v0.2 уже приведён (§2.3, §5.4, §8); `gateway-and-bot.md` §3 — `goose`; `epics.md` §2 EPIC-004 — `goose`. Устаревшая фраза «`golang-migrate` или встроенные SQL» в ADR-004 помечена решённой. Запрос architect#3 — подтвердить, что `design.md` EPIC-004 §6/§7 ссылается на ADR-019 п. 2 (grep расхождений не нашёл) |
| h | US-014 критерий «две модели загружены» → «модели по `baseline.md`» | architect#2 → BA | **BA** | архитектурно верно: конфигурация моделей — результат F-8 (E/C/A), не константа. Передано business-analyst (§13) |
| i | UC-028/029 (регион из блупринта) устарели; SA добавил UC-036 | architect#2 → SA | **К** (проверено) | `use-cases.md` v0.2 таблица изменений: UC-025/028/029 переписаны под фикстуры (UC-028 шаг 2 «привязывает к существующим сущностям», UC-029 шаг 3 → UC-036), UC-036 добавлен, трассировка US-010/US-015 → UC-036. Действий у SA нет; architect#2 при нарезке ссылается на UC-028/029 v0.2 как актуальные |

## 12. Итог сведения 2

| Категория | Всего | П | ПИ | О | BA | К |
|---|---|---|---|---|---|---|
| решения пользователя (U-8…U-12) | 5 | 5 | — | — | — | — |
| запросы architect#1 (a, b, c, d) | 4 | 3 | 1 | 0 | 0 | 0 |
| запросы architect#2 (d, e, f, h, i) | 5 | 1 | 2 | 0 | 1 | 1 |
| devops / единообразие (g) | 1 | 1 | 0 | 0 | 0 | 0 |

Отклонённых нет. «Принято с изменением»: d (одна заглушка Phase 1 — `FakeEncounter` у EPIC-003; `WithEncounterStub` снят с F-10), e (флаг читает `cmd/multiverse`). U-8 меняет рекомендацию архитектуры (Ollama native API → llama-server `openai_compat`) — совместимо с интерфейсом `Provider` и порядком middleware; объём EPIC-003 не растёт (B3a вместо B3, B3b — вторая реализация того же интерфейса, ≈ 300 строк, может быть отложена до F-8).

## 13. Что должны внести другие роли (после сведения 2)

| Кто | Что | Основание |
|---|---|---|
| architect#2 / tech-lead#2 | `design.md` EPIC-003 §3.1: **B3 → B3a `openai_compat` (llama-server: `/v1/chat/completions` + `response_format json_schema`, `chat_template_kwargs.enable_thinking=false`, `/v1/embeddings`, `/v1/models`, `/health` 503 = loading, `usage`/`timings`; семплинг фазы из блупринта, non-thinking профиль `0.7/0.8/20/0/1.5` по умолчанию) — первым; B3b `ollama` native — вторым** (может уйти за F-8, если E проходит); B1 — `Params{TopP, TopK, MinP, PresencePenalty}`, `Response.Tokens.Cached`, `ReasoningLen`; `MV_LLM_URL`/`MV_LLM_API_KEY` в `env.Declare`, гейт облака по host; §13 п. 1 — флаг читает `cmd/multiverse`; §4.1 — `FakeContext` как `runtime.Context`; A3 — стартовая модель блупринтов **E** `Qwen3.8-27B-UD-Q3_K_XL`, `thinking: false`; `components/swarm-llm-laws.md` §9.1 таблица провайдеров и §9.5 (`loading`); §11 п. 3 «стартово C» → «стартово E»; §10 риск 2 — добавить #20345 | U-8, ADR-005 доп. 2, C-15 v1.1, e |
| architect#1 / tech-lead#1 | `design.md` EPIC-001 §5: строка `FakeNarrator` v0 — **без `WithEncounterStub`** (§5.1 переписать: Phase 1 для I1-α — `FakeEncounter` EPIC-003 C4, ранний merge; хук `cmd/multiverse/fake_contexts.go` + `MV_SWARM_FAKE` — задача F-2/F-10); §6 C-05 строка; `design.md` EPIC-002 §3 п. 4, §4.1 I1-11, §5 C-05, §9, §11 — `FakeNarrator(WithEncounterStub(Rules))` → `FakeEncounter` (EPIC-003) на `membus`; §12 EPIC-005 — запросы 1–3 закрыты (ADR-001 доп. п. 7, C-10); `tasks.md` EPIC-001: F-4b — схема `analytics.consistency.violated.v1.json`; F-8 — вариант E, скрипт против `/v1/chat/completions`; `.golangci.yml` — правила `memory → llm` и исключение `cmd/multiverse/fake_contexts.go`; `tasks.md` EPIC-005: `mvctl trace` в 005-ops (Must) | a, b, c, d, e |
| architect#3 / tech-lead#3 | подтвердить, что `design.md` EPIC-004 §6/§7 и `tasks.md` ссылаются на ADR-019 п. 2 (`goose`, embed FS, без `Down`), без упоминаний `golang-migrate`; раздел `admin` `gateway.openapi.yaml` — принять PR с текстом маршрутов от EPIC-003 (совладение) | g, f |
| devops-engineer | `infrastructure.md` §6 переписать под llama-server: нативный процесс вне compose; `scripts/llm-server.ps1` с зафиксированными параметрами (`-m D:\Models\unsloth\Qwen3.8\Qwen3.8-27B-UD-Q3_K_XL.gguf --host 127.0.0.1 --port 1234 --n-gpu-layers 99 --flash-attn on --jinja --kv-offload --kv-unified --reasoning on --ctx-size <MV_LLM_NUM_CTX×slots> --load-mode mmap+mlock`; семплинг по умолчанию не фиксировать в скрипте — задаёт шлюз на запрос; `--models-dir` только в router-режиме без `-m` — отдельный вариант скрипта `-Router` для конфигураций A/E+), `make llm-up/llm-down/llm-health`; health `GET /health` (200/503) и `GET /v1/models`; `.env.example` §4.2: `MV_LLM_PROVIDER=openai_compat`, `MV_LLM_URL=http://host.docker.internal:1234` (go run — `http://127.0.0.1:1234`), `MV_LLM_API_KEY=` (только облако), `MV_OLLAMA_URL` — только при `MV_LLM_PROVIDER=ollama`; Ollama — опционально (профиль `gpu`/нативно), `make warm` только для Ollama; `make up` — проверка `MV_LLM_URL/health` с предупреждением; §6.4: `bench-matrix.json` + вариант **E** (`provider: openai_compat`, `model: Qwen3.8-27B-UD-Q3_K_XL`, `num_ctx` 8192/16384, KV f16/q8_0) и E+ (27B + `qwen3-8b` router), `llm-bench.ps1` ходит в `/v1/chat/completions` с `response_format json_schema` и `chat_template_kwargs.enable_thinking=false`, `usage`/`timings` в CSV, `nvidia-smi` VRAM; §9.3 runbook «прогрев» → «старт llama-server и ожидание `/health=200`»; §12 риск автообновления Ollama → риск «обновление llama.cpp меняет API/грамматику» (пин билда llama.cpp в `build/versions.env` как справочная запись, `mvctl llm ping` в `make health`); §13: вопросы 1–5 закрыты (U-8…U-12); F-7 `CODEOWNERS` = `@alekseizabelin1985-spec`, приватный личный репозиторий; F-6 — форк `minio/minio` в аккаунт пользователя, сборка из форка | U-8…U-12, ADR-005 доп. 2 п. 5–7 |
| business-analyst | US-014: критерий «две модели загружены» → «модели по `ops/metrics/baseline.md` (конфигурация E/C/A), переключение провайдера `openai_compat`/`ollama` без перезапуска ядра»; NFR-076 «размещение моделей» — формулировка «в VRAM целиком» применима к E; `nfr.md` «Что измерить первым» — llama-server как основной рантайм | h, U-8 |
| system-analyst | действий нет (UC-028/029 v0.2 актуальны, UC-036 есть); при следующей правке `api-contracts.md` §1.8/§4 — упомянуть `mvctl trace` (005-ops) рядом с `/v1/trace` | i, c |
| security-engineer | threat-model: SEC-15 (`OLLAMA_ORIGINS/HOST`) → добавить llama-server: слушает только `127.0.0.1:1234`, `--api-key` не обязателен локально, `--ui-mcp-proxy`/`--tools all` в команде пользователя — веб-UI и MCP-прокси на loopback (не публиковать наружу; шлюз их не использует); облако через `openai_compat` — тот же гейт `MV_LLM_CLOUD_ENABLED` по host | U-8, ADR-005 доп. 2 п. 3 |
| qa-engineer | `testing/strategy.md` §5.4 CT-06: пара «`FakeEncounter`/`FakeNarrator` ↔ роли роя» (без `WithEncounterStub`); критерий I1 — удаление хука `MV_SWARM_FAKE` из `cmd/multiverse` | d, e |

## 14. Сведение 3 — замечания тимлидов TEAM-2/TEAM-3 при нарезке задач (планирование, до волны 0)

Версия документа 0.3 · 2026-09-09 · system-architect#1. Входы: `epics/EPIC-004-gateway-bot/tasks.md` §8 (З-1…З-9, tech-lead#3), `epics/EPIC-003-swarm-llm-laws/tasks.md` §10 (1–9, tech-lead#2), `journal.md` (записи «EPIC-004 · tech-lead#3», «EPIC-003 · tech-lead#2»). Выходы: `contracts.md` v0.4, ADR-017 дополнение 1, `plan/ownership.md` v0.4, точечные правки `analysis/data-model.md` v0.2.1 и `analysis/api-contracts.md` v0.2.1 (system-analyst не запущен; правки помечены «(сведение 3, system-architect)»). Код не менялся. Новых ADR нет (`counters.adr` = 21 без изменений).

### 14.1. Решения

| # | От | Суть | Статус | Решение | Куда внесено |
|---|---|---|---|---|---|
| З-1 | tech-lead#3 | `end_reason` при `/forget`: FR-061 требует `forget`, схема/CHECK допускают `leave\|idle\|death\|error` | **П** | `end_reason ∈ leave\|idle\|death\|error\|forget`; `forget` не ошибка (`error` = 0 в S-метриках) | C-10 v1.1; data-model §5.2, §9.5; api-contracts §2.3.14; T-301 схема, **T-302 CHECK** (tech-lead#3); `metrics.md` §4.2 — BA |
| З-2 | tech-lead#3 | Кто публикует `abandoned` при `/forget`; `group.left cause` | **ПИ** | Публикует **gateway** в каскаде `ForgetHooks` одним atomic `entity.update.proposed cause=forget` (игрок `set status=abandoned` + при лидерстве `set leader_id`), затем `group.left {cause: forget}` / `group.leader_changed {cause: forget}`, затем `session.End(forget)`, затем удаление связки. State валидирует **только `alive → abandoned`** (изменение против рекомендации «`alive\|dead → abandoned`»: `dead` терминален по FR-023, правило `dead_entity`/inv-01 не трогаем; `dead` уже исключён из scope и целей). `abandoned` — терминальный, для inv-01/`NPCTarget`/стража = `dead`; `narrative.output kind=death` не генерируется; `creating` → предложение при `entity.created` без связки; строка gateway в `OwnershipRules` дополнена | C-02 v1.2, C-04 v1.1, C-08 v1.2, §16 п. 6; ADR-017 доп. 1 п. 5; data-model §3.3, §4, §9.1; api-contracts §1.2, §2.3.2, §2.3.4; T-314/T-355 (tech-lead#3), T-056 State (tech-lead#1), T-202 `levels.go` (tech-lead#2) |
| З-3 | tech-lead#3 | US-008: уведомление об облаке; бот шину не читает, в C-08 нет поля | **ПИ** | `GET /v1/worlds → worlds[].llm{cloud_enabled}` — единственное место (не `/v1/players/{id}`, не `Delivery kind=system`); источник — проекция gateway последнего `config.cloud_enabled.enabled`, по умолчанию `false`, в снапшоте `gateway/`; без провайдера/URL/ключей (SEC-21); бот кэширует флаг (`MV_TELEGRAM_CLOUD_FLAG_TTL=60s`, обновление на `/start`, `/help`, TTL), уведомление один раз за сессию диалога. Дополнительно (изменение): контекст `llm` публикует `config.cloud_enabled` **при каждом старте `core`** и при изменении, иначе проекция после рестарта недетерминирована | C-08 v1.2, C-06 v1.1; api-contracts §1.3, §2.3.16; T-320 разблокирована (tech-lead#3); T-215/контекст `llm` — публикация при старте (tech-lead#2); US-008 текст без имени провайдера — BA |
| З-4 | tech-lead#3 | `leader_id: null` при отсутствии живых; `GroupView.leader` | **П** | `Group.leader_id: ref \| null`; gateway предлагает `set leader_id = null` в том же atomic-пакете и публикует `group.leader_changed {leader: null, cause}`; `GroupView.leader_id: string \| null`; `enter`/`leave` группы при `null` → `409 no_leader` (раньше `not_leader`); личный `group.leave` допустим | C-04 v1.1, C-08 v1.2; data-model §3.6, §9.4; api-contracts §1.3, §1.6, §1.7, §2.3.2; T-352 (tech-lead#3) |
| З-5 | tech-lead#3 | Имя env allowlist: `MV_TELEGRAM_ALLOWED_USER_IDS` (prd, threat-model, US-008) vs `MV_TELEGRAM_ALLOWED_USER_IDS` (КД, design, infrastructure) | **П** (BA/security) | Единое имя **`MV_TELEGRAM_ALLOWED_USER_IDS`** (по компонентному документу, `infrastructure.md` §1 п. 11 и `.env.example`, `overview.md` §16) | contracts.md Env; запрос BA (FR-130, US-008) и security (SEC-06) |
| З-6 | tech-lead#3 | Hot reload allowlist «без перезапуска — Should» не спроектирован | **BA** | В MVP-1 — невыполненный Should, объём не расширяем; кандидат в EPIC-013 (E-H: инвайт-коды/аутентификация заменят allowlist) | здесь; запрос BA (FR-130 пометка «Should → E-H/EPIC-013») |
| З-7 | tech-lead#3 | Имя ветки в КД §13 | **К** | Канон — `epic/EPIC-004-gateway` (`teams.md`); architect#3 правит КД при следующей ревизии | — |
| З-8 | tech-lead#3 | SEC-26 не упомянут в design §2 | **К** | Учтено в DoD T-303/T-318 | — |
| З-9 | tech-lead#3 / п. 9 tech-lead#2 | `counters.task` при параллельной нарезке | **К** | Счётчик сводит tech-lead#1 один раз после G3 (`task = max`); `.dev-team.json` в сведении 3 не менялся | запрос tech-lead#1 |
| TL2-1 | tech-lead#2 | `validation_status`: 11 (data-model) vs 7 (КД/ADR-017) | **ПИ** | **6 значений** `valid\|partially_rejected\|invalid\|error\|quarantined\|filter_error`; причины — только `llm.output.rejected.reason` (ADR-017 п. 3); `budget_exceeded` — `rejected` без `llm.output`; опц. `llm.output.reasons[]`. Отклонение от рекомендации «4 значения»: `error` (ответа нет) и `filter_error` (fail-closed, без `response_raw`) — самостоятельные ветки конвейера с разными правилами хранения, их нельзя свести к причинам | C-07 v1.2; ADR-017 доп. 1; data-model §7.2; api-contracts §2.3.10; T-215 (tech-lead#2); US-018 — BA |
| TL2-2 | tech-lead#2 | Издатель `dice.rolled` ≠ владелец | **П** | Правило §0: «тип — владелец схемы; издатели — `Spec.Publishers`»; `dice.rolled`: владелец EPIC-002, издатели — агент встречи и `FakeEncounter`; job `contracts` проверяет `source ∈ Publishers` | contracts §0, §16 п. 7, C-03; api-contracts §2.3.5; F-4a реестр (tech-lead#1) |
| TL2-3 *(решение своего времени; истина по ADR-025 — `shared/contracts/ownership.go`, `levels.go` копии не держит, см. T-416)* | tech-lead#2 | Расхождение `levels.go` ↔ `OwnershipRules` | **П** | Истина — `levels.go` (TEAM-2); копия в `shared/contracts` правится **в том же PR** владельцем истины (метка `contract-change`, ревью tech-lead#1 + system-architect, блокер); тест равенства блокирует merge; State — потребитель | contracts §16 п. 6; ownership §1 |
| TL2-4 | tech-lead#2 / architect#2 | US-014 «две модели» vs конфигурация E/C | **BA** | Уже передано BA в сведении 2 (§13); T-203/T-260 без этого критерия до правки | §13 |
| TL2-5 | tech-lead#2 | `encounter.started` от `FakeEncounter` до появления роя | **П** | Покрыто §0 (исключение для заглушек v0.3) и уточнено: `FakeEncounter` в `Spec.Publishers` как `testkit/swarm`, допустим только на `membus`/`MV_SWARM_FAKE=true` | contracts §0 |
| TL2-6 | tech-lead#2 | Догон роя без `replay.completed` от `FakeState` v0 | **ПИ** | Заглушка **публикует** `analytics.replay.completed {mode: recovery}` при старте (F-10/T-017, tech-lead#1) — дешевле и соблюдает протокол; плюс таймаут ожидания `MV_SWARM_REPLAY_WAIT=120s` → `/health degraded {state_replay: missing}` (нужен и в production). Вариант «рой не блокируется» в T-237 заменяется на «ждёт с таймаутом» | C-14 v0.4, §17; T-017 (tech-lead#1), T-237 (tech-lead#2) |
| TL2-7 | tech-lead#2 | `enum` в `schemas/agent/tick-*.json` | **П** (подтверждено) | Файл схемы — полный набор типов MVP-1 для уровня; рантайм при компиляции подставляет пересечение с `allowed_event_types` блупринта (КД §13.4 «схема генерируется из шаблона»); валидатор проверяет `allowed ⊆ enum файла` | T-203 без изменений |
| TL2-8 | tech-lead#2 | `mvctl laws bump` публикует в шину — доступ с хоста | **К** (devops) | Уже предусмотрено `infrastructure.md` §1 п. 8: CLI на хосте (`laws bump`, `world init`, `record`) → `MV_KAFKA_BROKERS=127.0.0.1:19092` (external listener). Имени `MV_KAFKA_BROKERS` нет — в задачах использовать `MV_KAFKA_BROKERS`; runbook раздел «CLI на хосте» — devops | contracts Env; ownership §1; запрос devops |

### 14.2. Итог сведения 3

| Категория | Всего | П | ПИ | О | BA | К |
|---|---|---|---|---|---|---|
| замечания tech-lead#3 (З-1…З-9) | 9 | 3 | 2 | 0 | 1 | 3 |
| замечания tech-lead#2 (1–9) | 9 | 4 | 2 | 0 | 1 | 2 |

Отклонённых нет. Все изменения контрактов — совместимые (расширения enum, новые опциональные поля/коды, nullable-поле в схеме, которая ещё не реализована). Три отклонения от рекомендаций оркестратора обоснованы в таблице: З-2 (`alive → abandoned` только), TL2-1 (6 значений вместо 4), З-3 (публикация `config.cloud_enabled` при каждом старте). Подволна 1.2 EPIC-004 (T-302) разблокирована: З-1, З-2, З-4 решены; T-320 разблокирована З-3.

### 14.3. Что должны внести другие роли (после сведения 3)

| Кто | Что | Основание |
|---|---|---|
| business-analyst | FR-130/US-008: имя `MV_TELEGRAM_ALLOWED_USER_IDS`; FR-130 «без перезапуска — Should» → пометка «не в MVP-1, EPIC-013»; FR-061: издатель `abandoned` — gateway, переход только из `alive`, `dead` остаётся `dead`; US-008: текст уведомления об облаке без имени провайдера (или согласовать `provider_label` как отдельное совместимое дополнение); US-018: критерий `validation_status = rejected_unknown_entity` → `partially_rejected\|invalid` + `rejected reason=unknown_entity`; `metrics.md` §4.2 `session.ended.end_reason` + `forget`, `session_end_reason_share` | З-1, З-2, З-3, З-5, З-6, TL2-1 |
| security-engineer | threat-model SEC-06: `BOT_ALLOWED_USER_IDS` → `MV_TELEGRAM_ALLOWED_USER_IDS`; SEC-21: подтвердить, что `worlds[].llm.cloud_enabled` (bool) не раскрывает провайдера/ключей | З-3, З-5 |
| tech-lead#1 (TEAM-1) | T-017 (`FakeState` v0): публикация `analytics.replay.completed {mode: recovery}` при старте, приём `set status=abandoned cause=forget` (`alive` → ok, `dead` → `dead_entity`); F-4a: `Spec.Publishers` в реестре и копия `OwnershipRules` со строкой gateway (`Character.status → abandoned`, `Group.leader_id` включая `null`); T-056 (State): `abandoned` терминальный, inv-01/`dead_entity` для `status ∈ dead\|abandoned\|ascended_final`, `cause=forget`; схемы EPIC-002: `entity.update.proposed.cause` + `forget`; `counters.task` — свести после G3 | TL2-6, TL2-2, TL2-3, З-2, З-9 |
| tech-lead#2 (TEAM-2) | T-215: `validation_status` — 6 значений, `reasons[]` опц.; контекст `llm`: `config.cloud_enabled` при каждом старте; T-202: строка gateway в `levels.go` + копия в `shared/contracts` тем же PR, тест равенства; T-237: ожидание `replay.completed` с таймаутом `MV_SWARM_REPLAY_WAIT` → `degraded` (вместо «не блокировать»); роли роя/`NPCTarget`/страж: `abandoned` = `dead`; в задачах `MV_KAFKA_BROKERS` → `MV_KAFKA_BROKERS` | TL2-1, З-3, TL2-3, TL2-6, З-2, TL2-8 |
| tech-lead#3 (TEAM-3) | T-301: схемы `group.*` (`cause` + `forget`, `leader` nullable), `analytics.session.ended` (`forget`), OpenAPI (`worlds[].llm`, `GroupView.leader_id` nullable, `409 no_leader`); T-302: CHECK `end_reason IN ('leave','idle','death','error','forget')`; T-314/T-355: каскад `/forget` по C-04 v1.1 (порядок публикаций, `abandoned` только для `alive`); T-352: `no_leader`; T-320: разблокирована (`GET /v1/worlds`, TTL-кэш в боте) | З-1…З-4 |
| architect#3 | КД `gateway-and-bot.md` §7.5 (каскад: `abandoned`, `cause=forget`, `end_reason=forget`), §4.2 CHECK, §7.4 (`leader_id = null`, `no_leader`), §13 имя ветки; §11.3 — `MV_TELEGRAM_CLOUD_FLAG_TTL` | З-1…З-4, З-7 |
| architect#2 | КД `swarm-llm-laws.md` §9.2 (`rejected_language` → `invalid` + `reason=language`), §14 п. 10 (`config.cloud_enabled` при каждом старте), §13.4 (подтверждение TL2-7), `abandoned` в §10.3/§10.4 и `NPCTarget` | TL2-1, З-3, TL2-7, З-2 |
| architect#1 | КД `state-and-mechanics.md` п. «Мёртвые»: `status ∈ {dead, abandoned, ascended_final}`; `cause=forget`; `OwnershipRules` строка gateway | З-2 |
| devops-engineer | runbook «CLI на хосте» (`mvctl laws bump\|world init\|record` → `MV_KAFKA_BROKERS=127.0.0.1:19092`); `.env.example`: `MV_SWARM_REPLAY_WAIT`, `MV_TELEGRAM_CLOUD_FLAG_TTL` | TL2-8, TL2-6, З-3 |
| qa-engineer | `testing/strategy.md`: contract-тест «`source ∈ Spec.Publishers`»; e2e `forget`: `abandoned`, `group.left cause=forget`, `end_reason=forget` | TL2-2, З-1, З-2 |
