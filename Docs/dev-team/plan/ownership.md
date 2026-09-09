# Карта владения

Версия 0.2 · 2026-09-09 · предложение system-architect#1 (утверждается на G2 вместе с `epics.md`; ведёт tech-lead#1). Правило по умолчанию: путь меняет только команда-владелец; общий код — только через запрос на изменение контракта (раздел 2). Изменения v0.2: `shared/clock`, `shared/runtime`, `build/versions.env`, `build/minio.Dockerfile`, `.gitattributes`, `.golangci.yml`; префикс `snapshots-{world}/gateway/`; совладение схемы `analytics.replay.completed`; `services/_archive/` вместо удаления (OQ-A-17); `shared/agent/levels.go` как источник `contracts.OwnershipRules`; таблица запросов заполнена и закрыта (`architecture/consolidation.md`).

## 1. Пути и ресурсы

| Путь / ресурс | Владелец (эпик) | Правило |
|---|---|---|
| `go.mod`, `go.sum`, `cmd/multiverse/main.go`, `build/` (`Dockerfile`, `minio.Dockerfile`, `versions.env`, `redpanda-init.sh`), `docker-compose.yml`, `Makefile`, `.github/workflows/**`, `.github/dependabot.yml`, `CODEOWNERS`, `.gitignore`, `.gitattributes`, `.golangci.yml`, `.gitleaks.toml`, `.env.example`, `.mcp.env.example` | EPIC-001 (после волны 0 — tech-lead#1 + devops-engineer) | изменения после волны 0 — через tech-lead#1; добавление зависимости в `go.mod` — любой командой, но с пометкой в отчёте; версии образов — только в `build/versions.env` |
| `shared/eventbus/**`, `shared/jsonpath/**`, `shared/contracts/**`, `schemas/events/_common.json` | EPIC-001 → system-architect | **общий код**: только через запрос на изменение контракта (C-01); политики топиков в реестре — system-architect |
| `shared/clock/**`, `shared/runtime/**` | EPIC-001 → system-architect | **общий код** (часть C-01 v1.1): интерфейсы `Clock/Timers`, `Deps/Context`, HTTP-сервер процесса; правки — запрос |
| `shared/objstore/**`, `shared/env/**`, `shared/logging/**` | EPIC-001 → tech-lead#1 (`objstore` — интерфейс по ADR-021: правки через system-architect) | правки `env`/`logging` — запрос к tech-lead#1 (не контракт, но общий код); расширение интерфейса `objstore` — контракт |
| `schemas/events/<type>.v<n>.json` | владелец типа по `contracts.md` §0 (EPIC-002/003/004/005) | добавлять схемы своих типов можно; менять чужие — запрос |
| `schemas/events/analytics.replay.completed.v1.json` | **совладение**: файл — EPIC-002 (`mode=recovery`); поля `mode=test`, `events_hash_match` добавляет EPIC-005 через PR в файл с ревью EPIC-002 | несовместимые правки — запрос |
| `shared/testkit/**` | EPIC-001 (`membus` с `Journal`, `Dedup`-псевдоним, `Versions()`, фикстуры); фейки контрактов добавляет **поставщик** контракта (`FakeState`/`FixedMechanics` — EPIC-002, `FakeNarrator`/`RecordingWriter` — EPIC-003, `FakeGateway`/`Harness` — EPIC-004) | каждый фейк — в подпакете владельца: `shared/testkit/state`, `…/swarm`, `…/gateway` |
| `shared/entity/**`, `internal/state/**`, `internal/mechanics/**`, `internal/replay/**`, `rules/**` | EPIC-002 | `rules/dark-forest.yaml` — числа меняет EPIC-002 по DR-04 после прогонов; блупринты ссылаются на файл; `internal/replay` импортирует только `cmd/multiverse` (ADR-001 дополнение п. 2) |
| `shared/agent/**` (включая `levels.go` — источник `contracts.OwnershipRules`), `internal/swarm/**`, `internal/llm/**`, `internal/laws/**`, `blueprints/**`, `laws/**`, `schemas/agent/**`, `config/absolute-limits.yaml` | EPIC-003 | формат блупринта (C-11) и `laws` (C-12) — контракты; изменение таблицы уровней в `levels.go` — запрос (влияет на State через C-02); новые блупринты регионов после MVP-1 — авторы (оператор) без запроса |
| `internal/gateway/**`, `cmd/telegram-bot/**`, `api/gateway.openapi.yaml` (включая раздел `admin` — прокси C-06), `internal/gateway/migrations/{links,gateway}/*.sql` | EPIC-004 | миграции SQLite — нумерация `0001_*.sql`… только EPIC-004; коды ошибок и лимиты C-08 — совместимые дополнения без запроса, несовместимые — запрос |
| `internal/memory/**`, `cmd/mvctl/**`, `api/memory.openapi.yaml`, `ops/metrics/**`, `testdata/recordings/**`, `testdata/analytics/**` | EPIC-005 | `mvctl contracts check`, `mvctl blueprint validate`, `mvctl storage init` — каркас в EPIC-001, наполнение EPIC-005/EPIC-003 (валидатор — код EPIC-003, команда CLI — EPIC-005); записи — только из сессий `actor_kind=ci` (ADR-010 дополнение) |
| `testdata/fixtures/**` (мир, регион, NPC, игроки `player-A/B/C`) | EPIC-001 (создаёт) → общие | добавлять можно всем; менять существующие — уведомить tech-lead#1 |
| `services/<frozen>/**` (world-generator, universe-genesis-oracle, ontological-archivist, cultivation-module, plan-manager, city-governor, entity-actor, evolution-watcher) | заморожены (EPIC-006…010) | не менять до старта соответствующего эпика; `FROZEN.md` — EPIC-001 F-3 |
| `services/narrative-orchestrator/**`, `services/semantic-memory/**` (as-is, профиль `legacy`) | EPIC-003 (только на время профиля `legacy`); Neo4j-часть semantic-memory переносится в `internal/memory` EPIC-005 | не развивать; в EPIC-003 I2 (S5) → `services/_archive/` |
| `services/game-service/**`, `services/entity-manager/**`, `services/rule-engine/**` | источники для переписывания (EPIC-004, EPIC-002, EPIC-002) | переносить код, а не править на месте; после переноса → `services/_archive/` владельцем нового пакета |
| **`services/_archive/**`** (ban-of-world, reality-monitor, `shared/{schema,redis,config,minio,oracle,rules,intent,tinyml,spatial}`, `fake_deps`, `test_minio.go`, `shared/agent/tools/*`, лишние Dockerfile; позже — legacy-сервисы) | EPIC-001 F-1/F-3 создаёт; далее — никто (только чтение) | решение пользователя OQ-A-17: **ничего не удалять**; каждый каталог — `ARCHIVED.md` (причина, коммит, эпик возврата); вне `go.mod`/compose/Makefile/линтера; возврат кода в сборку — через архитектора |
| `Docs/dev-team/architecture/**` | system-architect (overview, ADR-001…010, ADR-021, contracts, consolidation); `architect#N` — `components/*.md`, ADR-011…020; security-engineer — `threat-model.md`; devops-engineer — `infrastructure.md` | ADR системного уровня — только system-architect; правки `infrastructure.md` по решениям сведения (порт 8090, профиль `legacy` + Chroma, `prompts-*` 30 дней, gitleaks allowlist, `versions.env`) — devops-engineer |
| `Docs/dev-team/plan/**` | tech-lead#1 (epics после G2, teams, decomposition-review, ownership), project-manager (roadmap, risks) | — |
| `Docs/archive/**`, `CLAUDE.md`, `AGENTS.md`, `README.md` | tech-writer (EPIC-001 F-9) | по факту кода |
| Топики Redpanda и их конфигурация (`redpanda-init`: retention, `segment.ms`, `max.message.bytes`) | EPIC-001 → devops-engineer | новый топик — только через архитектора (ADR-007) |
| Бакеты MinIO (`entities-*`, `snapshots-*`, `prompts-*`, `ops-artifacts`) | EPIC-002 (`entities`, `snapshots/state`), EPIC-003 (`snapshots/swarm`, `prompts`), **EPIC-004 (`snapshots/gateway`)**, EPIC-005 (`ops-artifacts`) | имена бакетов и префиксов — в `shared/objstore/buckets.go` (EPIC-001); versioning/ILM включает `EnsureBucket` (ADR-004 дополнение п. 2) |
| Коллекции Qdrant, схема Neo4j | EPIC-005 | без плагина APOC (ADR-004 дополнение п. 5) |
| `links.db`, `gateway.db` | EPIC-004 | доступ только из `internal/gateway`; внешний ID — только `links.db` (включая `character_requests`); тест NFR-041 (EPIC-005) читает оба файла и WAL только для проверки отсутствия утечек |
| Переменные окружения `MV_*` | владелец процесса/контекста объявляет через `shared/env.Declare`; `.env.example` — EPIC-001/devops | без префикса `MV_` — только сторонние (`OLLAMA_*`, `MINIO_ROOT_*`, `NEO4J_AUTH`, `COMPOSE_PROFILES`) |

## 2. Запросы на изменение контракта

Формат: контракт C-NN, суть изменения, совместимость, потребители. Процедура — `architecture/contracts.md` §16. Полная таблица с решениями по каждому запросу и замечанию — `architecture/consolidation.md`.

| # | От | Что | Затрагивает | Статус |
|---|---|---|---|---|
| 1 | architect#1 (EPIC-002) | C-01: `Journal{ReadRange, Tail, End}`, `PositionFromContext`, `eventbus.Dedup` | EPIC-001 (реализация), 002/003/004 (потребители) | принято → C-01 v1.1 |
| 2 | architect#1 | ADR-001 п. 3: `state → mechanics`, `cmd/multiverse → internal/replay`; пакеты `shared/clock`, `shared/runtime` | EPIC-001, EPIC-002 | принято → ADR-001 дополнение п. 2–3 |
| 3 | architect#1 | C-03: совместимые дополнения Go-API | EPIC-003 | принято → C-03 v1.1 |
| 4 | architect#1 | C-02: семантика `rest`, `atomic=false`, `applied_at`, `duplicate_entity`, `append` | EPIC-003, EPIC-004 | принято → C-02 v1.1 |
| 5 | architect#1 | C-14: `latest.json` указатель, `component` enum | EPIC-003, EPIC-004, system-analyst | принято → C-14 v1.1 |
| 6 | architect#1 | схема `analytics.replay.completed` — совладение | EPIC-005 | принято → §1 этой карты |
| 7 | architect#3 (EPIC-004) | C-08: коды ошибок, `202 pending`, `creating` | бот, харнесс | принято → C-08 v1.1 |
| 8 | architect#3 | C-05: `encounter.started.round{}` | EPIC-003 (издатель) | принято → C-05 v1.1, C-11 v1.1 |
| 9 | architect#3 | C-14/ownership: `snapshots-{world}/gateway/` | EPIC-001, EPIC-005 | принято → §1 |
| 10 | architect#3 | C-04: `say` не открывает раунд; BR-13 лидерство при смерти | BA (FR-025, BR-13) | принято как уточнение → C-04; требование — BA |
| 11 | architect#3 | ADR-006 п. 5: только личные чаты в MVP-1 | бот | принято → ADR-006 дополнение п. 2 |
| 12 | architect#3 | C-01: признак «конец журнала» (`Bus.Lag()`) | EPIC-001 | принято с изменением → `Journal.End()` вместо `Lag()` |
| 13 | architect#2 (EPIC-003) | C-11: glob в `trigger.event_name`; схемы: `llm.output.parse{}`, `narrative_event_id`, `lod_allowed` обязателен, `content_hash` | EPIC-004, EPIC-005 | принято → C-05/C-06/C-07/C-11 v1.1 |
| 14 | security-engineer | 5 Major + 12 Minor/Info (`threat-model.md` §8) | ADR-004/005/006/009/010, C-01/C-08/C-09/C-11 | принято (16), передано BA (1: текст FR-009) |
| 15 | devops-engineer | 14 замечаний (`infrastructure.md` §11) | ADR-001/004/005/007/009/010, ADR-021, C-01/C-06, overview §13/§19 | принято (10), принято с изменением (4: Chroma — в профиле `legacy`; порт `core` 8090; логи бота — по объёму; golden `merge=binary` без `-diff`) |
