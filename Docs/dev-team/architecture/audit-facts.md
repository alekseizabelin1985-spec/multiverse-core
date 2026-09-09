# Аудит платформы multiverse-core — сырые факты

Дата: 2026-09-09. Ветка: `feature/agent-gm-core` (HEAD `e6103e4`, 2026-05-27). Автор: system-architect#1 (Flow A, шаг A0).
Всё ниже установлено по коду и конфигурации, а не по документации. Команды выполнялись только на чтение.

## 1. Сборка, vet, тесты

Окружение: Windows 11, `go1.25.3 windows/amd64`. Из-за кириллицы в пути `go build` без `-buildvcs=false` падает с `error obtaining VCS status` — это артефакт окружения, не ошибка кода. Ниже результаты с `-buildvcs=false`.

| Модуль | build | vet | test (`-count=1 -short`) |
|---|---|---|---|
| `.` (root, `multiverse-core.io`) | OK | **FAIL**: `test_minio.go:44:3: result of fmt.Errorf call not used` | `shared/jsonpath` ok; остальные пакеты root-модуля без тестов |
| services/ban-of-world | OK | OK | no test files |
| services/city-governor | OK | OK | no test files |
| services/cultivation-module | OK | OK | no test files |
| services/entity-actor | OK | OK | no test files |
| services/entity-manager | OK | OK | no test files |
| services/evolution-watcher | OK | OK | no test files |
| services/game-service | OK | OK | no test files |
| services/narrative-orchestrator | OK | OK | **ok** (0.5s) |
| services/ontological-archivist | OK | OK | no test files |
| services/plan-manager | OK | OK | no test files |
| services/reality-monitor | OK | OK | no test files |
| services/rule-engine | OK | **WARN** ×3: `engine.go:121,127,135 self-assignment` | no test files |
| services/semantic-memory | OK | OK | **FAIL** — 6 тестов требуют живой Neo4j (`lookup neo4j: no such host`), не отмечены как интеграционные, не пропускаются по `-short`; 66 c |
| services/universe-genesis-oracle | OK | OK | no test files |
| services/world-generator | OK | OK | **ok** (2.3s) |
| shared/agent | OK | OK | **ok** |
| shared/agent/tools | OK | OK | no test files |
| shared/config | OK | OK | no test files |
| shared/eventbus | OK | OK | **ok** |
| shared/minio | OK | OK | не запустился: `fork/exec minio.test.exe: Access is denied` (ограничение Windows-окружения; результат неубедителен) |
| shared/oracle | OK | OK | no test files |
| shared/spatial | OK | OK | no test files |

Примечание: `go test ./...` из корня workspace прогоняет только root-модуль (пакеты `shared/entity|intent|jsonpath|redis|rules|schema|tinyml`), а не сервисы. `Makefile test` обходит сервисы циклом — корректно.

Тестовые файлы всего: 18 (`*_test.go`): сервисы 10 (narrative-orchestrator 2, semantic-memory 6, world-generator 2), shared 8 (agent 3, eventbus 3, jsonpath 1, minio 1). 12 из 15 сервисов не имеют тестов.

## 2. Счётчики

- Go LOC: 33 003 всего, 28 602 без тестов.
- Сервисы (LOC без тестов / файлов): narrative-orchestrator 4 808/11 (orchestrator.go — 2 316 строк, 72 KB), semantic-memory 4 897/14, entity-actor 2 322/7, world-generator 1 354/6, game-service 1 188/8, evolution-watcher 1 166/4, rule-engine 783/5, city-governor 582/3, universe-genesis-oracle 529/4, cultivation-module 486/3, entity-manager 451/4, ban-of-world 396/3, ontological-archivist 245/3, reality-monitor 235/2, plan-manager 214/3.
- Shared (LOC/файлов): agent 5 871/18 (+tools 5 файлов), eventbus 2 168/8, jsonpath 1 296/2, minio 777/5, intent 729/3, rules 685/2, tinyml 451/2, spatial 309/5, redis 267/1, entity 264/2, oracle 241/1, config 195/1, schema 94/2.
- Маркеры `TODO|FIXME|stub|заглушка|not implemented` в не-тестовом коде: 82 (из них ~45 в `shared/agent` и `shared/agent/tools`).
- Git: 133 коммита, 364 отслеживаемых файла, 40+ веток (15 `claude/*`, 4 `gm-<timestamp>`).

## 3. Workspace и версии

- `go.work`: `go 1.24.11`; 22 модуля: `.`, 15 сервисов, `shared/{agent, agent/tools, config, eventbus, minio, oracle, spatial}`.
- Root `go.mod`: `go 1.24.0`, `toolchain go1.24.7`. Содержит пакеты `shared/{entity,intent,jsonpath,redis,rules,schema,tinyml}` (не отдельные модули) и **`test_minio.go` (`package main` в корне)**.
- Сервисы: `go 1.24.0` (14 шт.), `semantic-memory` — `go 1.24.11`. Shared-модули: `go 1.24`.
- `shared/agent/tools/go.mod` использует `replace` на `../` и `../../eventbus` — единственный модуль с replace, остальные полагаются на workspace.
- Модуль вне workspace: `fake_deps/` (`module fake_deps`, `go 1.23`, файлы с `//go:build ignore`) — мёртвый.
- Dockerfile: `golang:1.24`. CLAUDE.md: «Go 1.25». AGENTS.md: «Go 1.24.0». Локально 1.25.3.
- Docker-образы: redpanda `v24.2.5`, `chromadb/chroma:latest` (не запинен), neo4j `5.18`, `timescale/timescaledb:latest-pg16`, `ollama/ollama:latest`, `qdrant/qdrant:latest`, `minio/minio` (без тега).

## 4. Инвентарь сервисов

Все 15 сервисов имеют `cmd/main.go`, `go.mod`, запись в `go.work` и `Makefile SERVICES`.

| Сервис | compose | Dockerfile | README | Подписки (топики) | Реагирует на типы | Публикует (топик: типы) | Хранилища/интеграции |
|---|---|---|---|---|---|---|---|
| ban-of-world | **нет** | build/Dockerfile | да | player_events | player.used_skill, player.used_item, player.moved | world_events: violation.detected, player.punished, player.teleported, skill.transformed | нет (in-memory) |
| city-governor | **нет** | build/Dockerfile | да | game_events, world_events | player.entered, violation.detected, quest.completed, city.reputation.changed, npc.interaction | game_events: citizen.event, city.population.changed, city.reputation.changed, city.reputation.effect, city.violation.consequence, npc.response.generated, quest.assigned, quest.reward.granted | нет (in-memory) |
| cultivation-module | да | build/Dockerfile | да | player_events, world_events, system_events | player.used_skill, ascension.completed, dao.interaction.attempt, cultivation.form.created | world_events: cultivation.form.validated/rejected, cultivation.progress.updated, cultivation.system.updated, dao.hybrid_formed, dao.interaction.success/conflict | нет (in-memory) |
| entity-actor | да | **свой** (копия build/Dockerfile) | да | player/world/game/system_events + фантомный топик `entity_actor_events` | ~30 типов (player.*, entity.*, combat.*, quest.*, item.*, npc.*, world.*, entity.actor.*) | world_events: результат актора (publishResult) | shared/minio (official), shared/redis (**заглушка, без драйвера**), shared/tinyml, shared/rules, shared/intent (Oracle HTTP). Пакет `entityactor/api` (HTTP handlers, /health, /v1/*) **не подключён ни к одному серверу** |
| entity-manager | да | build/Dockerfile | да | player/world/game/system_events + **топики с именами `entity.created`, `entity.deleted`** (типы событий использованы как имена топиков) | entity.created, entity_snapshots в payload, изменения состояния | **ничего не публикует** (0 вызовов Publish) | MinIO напрямую через `minio-go` (бакеты `entities-{world_id}`), shared/entity |
| evolution-watcher | да | **свой** | **нет** | player/world/game/system_events | все (аномалии) | system_events: violation-событие | shared/minio (official), shared/redis (заглушка), shared/intent (Oracle) |
| game-service | да | build/Dockerfile | да | world/game/player/system_events, narrative_output | обработчики-заглушки (TODO) | player_events: player.moved…; system_events: entity.created, gm.created/merged/split; world/game_events: entity.updated, npc.*, player.* | HTTP :8088 (gorilla/mux: /entities/{id}, /entities/{id}/history, /players/register, /players/login, /events/recent) + WebSocket /ws/{entities,events,actions}; **собственный MinIO-клиент** (`gameservice/minio_client.go`, пишет в `entities-{world_id}`), in-memory кэш |
| narrative-orchestrator | да | build/Dockerfile | да | system_events (gm.*, time.syncTime), world_events, game_events, фантомный топик `mechanical_results` | gm.created/deleted/merged/split, batch.process, time.syncTime, любые события scope | narrative_output: narrative.generate; world_events: decision-события и `new_events` из ответа LLM (произвольные типы); system_events: time.syncTime | HTTP → semantic-memory (`/v1/context-with-events`, `/v1/events-by-entities`), shared/oracle (LLM), shared/minio (снапшоты GM), shared/config (профили GM в бакете `gnue-configs`), shared/spatial, shared/agent (**мост, в runtime не активен** — `main.go` вызывает `NewService`, а не `NewServiceWithPipeline`) |
| ontological-archivist | да | build/Dockerfile | да | **не подключён к шине** (KafkaBrokers в Config не используется) | — | — | HTTP :8081 (`ONTOLOGICAL_PORT`; compose публикует 8083): `POST /v1/schemas`, `GET /v1/schemas/{type}/{name}/{version}`; MinIO напрямую (`minio-go`, бакет `schemas`). compose `depends_on: chromadb, neo4j` — не используются |
| plan-manager | да | build/Dockerfile | да | world_events, system_events | ascension.attempt, plan.convergence.requested, world.generated | system_events: plan.initialized, plan.convergence.activated, route-событие | нет (in-memory) |
| reality-monitor | **нет** | build/Dockerfile | да | **фантомные топики `world.metrics.*` и `reality.anomaly.detected`** (kafka-go не поддерживает wildcard; топиков не существует) | — | system_events: reality.anomaly.detected | нет. `main.go` игнорирует `KAFKA_BROKERS` (пустой `if`), брокер захардкожен `localhost:9092` |
| rule-engine | да | **свой** | **нет** | system_events | rule.apply → **только лог** | ничего | shared/minio (rules bucket). Движок кубиков/модификаторов есть (`engine.go`), к шине не подключён |
| semantic-memory | да | build/Dockerfile (CGO=1, tag `chroma_v2_enabled`) | да | все 6 топиков (6 consumer-групп) | все; entity.created/updated → upsert сущности | (тестовые типы только в тестах) | ChromaDB (v1 HTTP API `/api/v1/*` по умолчанию; v2 через build-tag + `chroma-go` + эмбеддинги Ollama `EMBEDING_URL`), Neo4j (`neo4j-go-driver/v5`), MinIO. HTTP :8080 (`SEMANTIC_PORT`; compose 8082): `/v1/context`, `/v1/events`, `/v1/context-with-events`, `/v1/context/structured`, `/v1/entity-context/{id}`, `/v1/entities/{id}`, `/v1/entities/query`, `/v1/events/{id}`, `/v1/events/query`, `/v1/relations/metrics`, `/health` |
| universe-genesis-oracle | да | build/Dockerfile | да | system_events | universe.genesis.request | system_events: universe.genesis.completed | shared/oracle (LLM), HTTP → ontological-archivist `/v1/schemas` |
| world-generator | да | build/Dockerfile | да | system_events | world.generation.requested | system_events: entity.created (world/region/water/city), world.generated, world.geography.generated | shared/oracle (LLM), HTTP → ontological-archivist `/v1/schemas` |

Статус по коду (шкала: реализован / частично / прототип / заглушка):
- реализован: semantic-memory, narrative-orchestrator (legacy-ветка GMInstance), world-generator;
- частично: game-service (HTTP/WS есть, обработчики-заглушки, нет auth), entity-manager (запись снапшотов есть; replay, публикация событий — нет), entity-actor, evolution-watcher (зависят от Redis-заглушки), ontological-archivist (2 endpoint'а, нет версионирования/списков), universe-genesis-oracle;
- прототип (in-memory, без тестов, без персистентности): ban-of-world, city-governor, cultivation-module, plan-manager;
- заглушка / неработоспособен: rule-engine (только лог), reality-monitor (подписки на несуществующие топики, брокер захардкожен).

## 5. Событийная модель

Структура `eventbus.Event`: `id, type, timestamp, source, world{entity{id,type}}, scope{id,type}, payload map[string]any, relations[]`. Ключ Kafka-сообщения = `world.entity.id` или `global`. `Subscribe` — блокирующий цикл `kafka.Reader` (MinBytes 10 KB, MaxWait из `KAFKA_POLL_FREQUENCY_MS`, default 1000 мс); один writer на топик, `LeastBytes`. Нет DLQ, нет ретраев обработчика, нет идемпотентности, нет схем/валидации на входе.

Топики, создаваемые `redpanda-init`: `player_events, world_events, game_events, system_events, scope_management, narrative_output` (6; CLAUDE.md заявляет 9). `scope_management` — никто не публикует (читает только semantic-memory).

Фантомные топики (есть в коде, не создаются, нет издателя): `entity_actor_events` (entity-actor), `mechanical_results` (narrative-orchestrator), `entity.created` и `entity.deleted` (entity-manager, ошибка: тип вместо топика), `world.metrics.*` и `reality.anomaly.detected` (reality-monitor).

Степень миграции на иерархический доступ (не-тестовый код; `Path()` / прямой `Payload["..."]` / `.WorldID` legacy / helpers / builder):
- ban-of-world 5/0/0/11/12; city-governor 7/0/0/15/18; cultivation-module 5/0/0/5/14; entity-actor 24/0/11/2/2; world-generator 0/0/0/0/12;
- entity-manager 0/**3**/0/2/0; evolution-watcher 0/**3**/2/1/2; plan-manager 0/**8**/0/2/0; semantic-memory 2/**9**/10/9/0; universe-genesis-oracle 0/**1**/0/1/0; narrative-orchestrator 0/**15**/49/20/0; game-service 0/0/4/0/0 (NewEvent с ручными map).
- Итог: 4 сервиса полностью на новом API, 6 — смешанно, `Docs/EVENTS-MIGRATION.md` («Complete — all services migrated») не соответствует коду.

Разрывы publisher → consumer (по литералам типов в не-тестовом коде):
- Потребляются, но **никто не публикует**: `rule.apply`, `ascension.completed`, `ascension.attempt`, `dao.interaction.attempt`, `cultivation.form.created`, `plan.convergence.requested`, `universe.genesis.request`, `player.entered`, `npc.interaction`, `entity.travelled`, `entity.state_changed`, `entity.snapshot`, `combat.started/ended/damage_dealt`, `item.traded/crafted`, `currency.changed`, `world.time_tick`, `world.weather_changed`, `npc.action`, `npc.dialogue`, `quest.started/updated`, `player.interacted`, `entity.actor.*`. Часть из них может приходить из `new_events` ответа LLM (narrative-orchestrator публикует произвольные типы в world_events) или из ручной публикации через `rpk`/`kafkacat` — контракта нет.
- Публикуются, но **нет бизнес-потребителя** (только индексация semantic-memory): `player.punished`, `player.teleported`, `skill.transformed`, все `city.*` (кроме `city.reputation.changed`, который city-governor слушает сам), `citizen.event`, `quest.assigned`, `quest.reward.granted`, все `cultivation.*` и `dao.*`, `plan.*`, `universe.genesis.completed`, `world.geography.generated`, `reality.anomaly.detected`, violation evolution-watcher, результаты entity-actor. `narrative.generate` читает game-service, но обработчик — TODO.
- Реально замкнутые цепочки: (1) game-service `player.moved/used_skill` → ban-of-world → `violation.detected` → city-governor → `city.violation.consequence` → narrative-orchestrator; (2) game-service `gm.created` → narrative-orchestrator → `narrative.generate` → semantic-memory; (3) `world.generation.requested` (внешний) → world-generator → `entity.created` → entity-manager (MinIO) + semantic-memory; `world.generated` → plan-manager; (4) narrative-orchestrator ⇄ semantic-memory по HTTP; (5) world-generator / universe-genesis-oracle → ontological-archivist по HTTP.

## 6. Shared-пакеты

| Пакет | Модуль | Потребители | Замечания |
|---|---|---|---|
| eventbus | отдельный | все, кроме ontological-archivist | ядро; 3 теста; README+MIGRATION+docs/event-model.pdf (3.9 MB в git) |
| jsonpath | root | eventbus, semantic-memory | 1 тест; хорошо документирован |
| minio | отдельный | entity-actor, evolution-watcher, narrative-orchestrator, rule-engine, config, rules, tinyml | **две реализации**: рукописный AWS SigV4 HTTP (`http_client.go`, 440 строк) и обёртка над `minio-go` (`minio_official_client.go`) + `legacy.go`, `factory.go` |
| oracle | отдельный | narrative-orchestrator, universe-genesis-oracle, world-generator | LLM HTTP-клиент с ретраями (`ORACLE_URL/MODEL/API_KEY/TIMEOUT_MS`) |
| intent | root | entity-actor, evolution-watcher | **свой Oracle HTTP-клиент** (дубль shared/oracle), кэш интентов, фильтры |
| entity | root | entity-manager, game-service | структура сущности с историей |
| redis | root | entity-actor, evolution-watcher | **нет драйвера Redis** — интерфейс `Conn` + заглушка; сервиса redis в compose нет |
| rules | root | entity-actor, shared/agent (filter, e2e-тест) | движок правил; дублирует `services/rule-engine/ruleengine/engine.go` и `entityactor/model.go` |
| tinyml | root | entity-actor | forward-pass модель; ONNX — заглушка; дубль `entityactor/model.go` |
| schema | root | **никто** | мёртвый код (JSON Schema validator) |
| spatial | отдельный | narrative-orchestrator | геометрия scope, провайдер через semantic-memory |
| config | отдельный | narrative-orchestrator | профили GM из MinIO бакета `gnue-configs`; **`configs/gm_*.yaml` с диска никто не читает** в активном пути (только `YAMLBlueprintConverter("configs")` в неактивном `NewServiceWithPipeline`) |
| agent (+tools) | отдельные | narrative-orchestrator (6 файлов-мостов) | см. ниже |

`shared/agent` (Agent GM Core), реализовано: типы/интерфейсы, `Router` (оценка условий — TODO), `LifecycleManager` (stop/pause/resume/cleanup — TODO), `WorkerPool` (без backpressure), `md_parser`, `blueprint_loader`, `blueprint_validator` (парсинг MD — «not implemented yet»), `TwoPhasePipeline` (переменные промпта `{weather}`, `{time_of_day}`, `{region_history}` захардкожены), `StateManager` (in-memory кэш, PostgreSQL — TODO), `LODManager`, `filter.go` (Redis/векторный поиск — TODO), `tools/` (реестр с rate-limit; тела entity/narrative/world tools — TODO). Тесты (3 файла, включая e2e dark-forest) проходят. Единственный потребитель — narrative-orchestrator через `NewServiceWithPipeline`/`NewNarrativeOrchestratorWithPipeline`, которые **не вызываются из `cmd/main.go`**; в активном конструкторе создаётся `agent.NewRouter(nil)` без lifecycle. Модели в pipeline захардкожены `qwen:7b` / `qwen:72b`. `PULL_REQUEST.md` упоминает `services/agent-orchestrator` — такого сервиса нет; заявленные «~17 000 LOC» — фактически 5 871. `shared/agent/README.md` использует неверный import-path `multiverse-core/shared/agent` (реальный `multiverse-core.io/shared/agent`). CI `validate-blueprints.yml` фильтрует по `blueprints/**` — каталога нет.

## 7. Инфраструктура и конфигурация

- `docker-compose.yml`: redpanda + console + init (sleep 10, создание 6 топиков), minio, chromadb, neo4j (APOC), timescaledb, qwen3-service (ollama, **обязательная резервация GPU nvidia** — compose не поднимется без nvidia runtime), qdrant, kafkacat, 12 сервисов. Healthcheck только у qdrant; `depends_on` без условий; `redpanda-console` использует `env_file: .env`.
- Не в compose: ban-of-world, city-governor, reality-monitor; сервиса redis нет (нужен entity-actor/evolution-watcher по дизайну).
- Клиентов нет ни у кого: TimescaleDB (0 упоминаний pgx/database/sql), Qdrant (0), ONNX runtime используется только в Dockerfile (скачивание 1.18.0 при CGO=1; `fake_deps` для обхода).
- Chroma: default-путь ходит в `/api/v1/*`; образ `chroma:latest` (Chroma ≥1.0 удалил v1 API) — риск несовместимости; v2-путь требует build-tag и Ollama-эмбеддинги.
- Порты: game-service 8088; semantic-memory 8080 (compose 8082, `.env` `SEMANTIC_PORT`); ontological-archivist 8081 (compose 8083, `ONTOLOGICAL_PORT`); redpanda 9092/8081(schema registry, не используется)/9644; minio 9000/9001; chroma 8000; neo4j 7474/7687; timescale 5433; ollama 11434; qdrant 6333/6334.
- Конфигурация: только env-переменные, дефолты захардкожены в каждом `main.go` (`redpanda:9092`, `minio:9000`, `minioadmin/minioadmin`, `neo4j/password`); единого config-пакета нет; `.env.example` содержит переменные, которые код не читает (`TIMESCALE_*`, `QDRANT_*`, `LOG_LEVEL`, `MINIO_USE_SSL`, `QWEN_MODEL`, `CHROMA_API_KEY`) и не содержит читаемые (`ORACLE_MODEL`, `ORACLE_API_KEY`, `ORACLE_TIMEOUT_MS`, `EMBEDING_URL/MODEL`, `CHROMA_USE_V2`, `CHROMA_COLLECTION_NAME`, `SEMANTIC_PORT`, `ONTOLOGICAL_PORT`).
- Наблюдаемость: стандартный `log` (slog в 3 файлах), нет метрик, трейсинга, корреляционных id; `/health` есть только у semantic-memory (и в неподключённом api entity-actor).
- Миграции: отсутствуют (все хранилища schemaless; Neo4j constraints/indexes создаются кодом при старте — см. `neo4j.go`).
- Восстановление: «snapshot + replay» из документации — replay не реализован ни в одном сервисе; снапшоты пишут entity-manager (сущности), narrative-orchestrator (GM/агент), entity-actor (модель/состояние); загрузка снапшота есть только в narrative-orchestrator (`loadSnapshot`).
- Dockerfile: `build/Dockerfile` (корректный: переписывает `go.work` до одного сервиса; CGO-ветка), корневой `Dockerfile` (устаревший: копирует полный `go.work`, но только один сервис → сборка сломается; используется в README/CLAUDE.md), 3 копии в `services/{entity-actor,evolution-watcher,rule-engine}/Dockerfile`.
- CI (`.github/workflows`): 5 workflow `qwen-*` (AI-триаж issues/PR), `validate-blueprints.yml` (только `go test ./shared/agent/...`, по path-фильтру). **Нет workflow на `go build`/`go vet`/`go test` для сервисов и нет сборки Docker-образов.**

## 8. Секреты, бинарники, мусор в репозитории

- `.gitignore`: только `.env`, `.qwen/`, `gha-creds-*.json`.
- **`.mcp.env` отслеживается git** (коммит `6e328eb`) и содержит непустые `GITHUB_TOKEN` (38 симв.) и `DB_MCP_TOKEN` (28 симв.) — утечка секретов в историю.
- Отслеживаемые бинарники: `services/narrative-orchestrator/cmd.exe` (15.3 MB), `semantic-memory.exe` (12.9 MB), `examples.exe` (3.0 MB); `shared/eventbus/docs/event-model.pdf` (3.9 MB).
- Отслеживаемые логи: `mcp_kafka.log` (211 KB), `mcp_audit.log`.
- Прочее: `test_minio.go` в корне (ломает `go vet` root-модуля); `fake_deps/` (4 файла); пустые каталоги `-p/` и `Multiverse/` (не отслеживаются); `.claude/` (28 файлов), `.qwen/`, `.roo/`, `.kilo*`, `.idea/`, `.vscode/`, `memory/`, `plans/`, `reports/`, `events/` (12 примеров) — в git.
- `Docs/user-stories/` (README, US-001, US-002; датированы 2026-06-26) — **не отслеживаются git** (`??`), при этом это самые свежие требования в репозитории и они описывают иную концептуальную модель (Universe → Ontologies → Overseer → Worlds; «API для фронтендов»), чем `Docs/architecture.md`.

## 9. Расхождения документации и кода

| Документ | Утверждение | Факт |
|---|---|---|
| AGENTS.md | у каждого сервиса есть `services/<svc>/<pkg>/AGENTS.md` | ни одного; есть `README.md` у 13 из 15 (нет у evolution-watcher, rule-engine) |
| AGENTS.md / README.md | точки входа `services/<svc>/cmd/<svc>/main.go` / `cmd/<svc>/main.go` | `services/<svc>/cmd/main.go` |
| AGENTS.md | «internal/ directories» | нет ни одного `internal/` |
| CLAUDE.md | Go 1.25; 9 топиков; TimescaleDB для метрик; recovery = snapshot + replay | go 1.24; 6 топиков; клиента TimescaleDB нет; replay не реализован |
| CLAUDE.md / README | entity-manager публикует `entity.created/updated/history.appended` | entity-manager ничего не публикует |
| Docs/architecture.md | сервис «Ascension Oracle»; GM публикует `narrative.description`, `npc.action.*`, `weather.change.*` | сервиса нет (только HTTP-клиент shared/oracle); GM публикует `narrative.generate` + произвольные типы из LLM |
| Docs/architecture.md | entity-manager подписан на `entity.create/update/link` | обрабатывает `entity.created`, `entity_snapshots`, `state_changes` |
| Docs/EVENTS-MIGRATION.md | «Complete — all services migrated» | 6 сервисов используют прямой `Payload[...]`/legacy `.WorldID` |
| LIVING_WORLDS_IMPLEMENTATION_STATUS.md | API endpoints entity-actor «Complete» | пакет `api` не подключён к HTTP-серверу |
| PULL_REQUEST.md | 17 000 LOC, «Deploy services/agent-orchestrator» | 5 871 LOC; сервиса нет; пайплайн не активирован |
| README (Docker Compose) | стек без Qdrant | в compose есть qdrant и GPU-ollama |
| README | `go build ./cmd/<svc>` | таких путей нет |
| Docs/ каталоги `architecture/`, `services/`, `common-library/`, `development/` | — | пустые |
| Три версии архитектуры: `architecture.md`, `architecture-updated.md`, `architecture_old.md` | — | какой канонический — не определено |

## 10. Дублирование кода (кандидаты на унификацию)

1. LLM-клиенты: `shared/oracle/client.go`, `shared/intent/oracle_client.go`, `services/narrative-orchestrator/narrativeorchestrator/oracle.go`, `oracle_llm_adapter.go`, `shared/agent` `LLMClient` интерфейс.
2. MinIO-клиенты: `shared/minio` (2 реализации), `services/game-service/gameservice/minio_client.go`, прямой `minio-go` в entity-manager, ontological-archivist, semantic-memory.
3. Движок правил: `shared/rules`, `services/rule-engine/ruleengine`, `services/entity-actor/entityactor/model.go` (заглушки `TinyModel`, `RuleEngine`).
4. Dockerfile ×5. `getEnv/getEnvBrokers` скопированы в 8 `main.go`.
5. Модель GM: `GMInstance` (legacy) и `NarrativeAgent`/`agent.Agent` (новая) сосуществуют в одном пакете с мостами `CreateAgentForScope`, `buildBlueprintFromGM`.
