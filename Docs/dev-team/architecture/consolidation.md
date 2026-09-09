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
| G-11 | Env `GATEWAY_*`, `BOT_*`, `TELEGRAM_BOT_TOKEN`, `MODE` без префикса | §11.3 | **ПИ** — читать с префиксом `MV_` (`MV_GATEWAY_LISTEN`, `MV_TELEGRAM_BOT_TOKEN`, `MV_MODE`…); architect#3 правит при нарезке задач | `contracts.md` (сводка Env, §16 п. 5) |

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
