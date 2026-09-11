# CLAUDE.md

Инструкции для Claude Code (claude.ai/code) при работе с этим репозиторием.

## Обзор проекта

**Multiverse-Core** — событийная платформа «живых миров» на Go 1.26: единый модуль
(`module multiverse-core.io`, `go.work` не используется), один бинарник платформы
`cmd/multiverse` и оператор-CLI `cmd/mvctl`. Шина — Redpanda (Kafka API); объектное
хранилище — MinIO (собственная сборка из исходников); опционально — Qdrant + Neo4j
(память) и локальный LLM (`llama-server`/Ollama) для нарратива.

**Ключевой принцип** (ADR-001, NFR-075): число процессов — параметр деплоя, а не
архитектуры. Контексты платформы (`state`, `mechanics`, `swarm`, `llm`, `laws`,
`gateway`, `memory`) — пакеты одного модуля с общим интерфейсом
`runtime.Context{Start, Stop, Health}`; какие из них подняты в конкретном процессе,
решает флаг `--contexts` у `cmd/multiverse`, а не отдельная сборка на сервис.

**Статус кода**: репозиторий проходит переход на эту раскладку (`EPIC-001
«Фундамент»`, `Docs/dev-team/epics/EPIC-001-foundation/`). На данный момент
`state/mechanics/swarm/llm/laws/gateway/memory` зарегистрированы как заглушки
(`cmd/multiverse/contexts.go`) и отвечают `/health: ok`, не реализуя домен —
доменные пакеты `internal/state`, `internal/swarm`, `internal/gateway` и т. д.
появляются по мере выполнения своих эпиков (EPIC-002…EPIC-005). Реализован пока
только `internal/mechanics` (типы механики, `Load` правил, RNG — без `Resolve`).

## Архитектура

### Карта каталогов

```
multiverse-core/
├── go.mod                     # module multiverse-core.io, go 1.26 (без go.work)
├── cmd/
│   ├── multiverse/             # бинарник платформы: serve (--contexts/--mode/--bus/--recording;
│   │                            # то же без подкоманды), health --url, db backup|check, version
│   └── mvctl/                  # CLI оператора: contracts, env, storage, privacy, version (реализованы);
│                                # world, blueprint, laws, record, golden, llm, memory, report, trace
│                                # зарезервированы под будущие эпики (cmd/mvctl/main.go — реестр)
├── internal/
│   └── mechanics/              # типы механики C-03, Load(rules/*.yaml), формулы, RNG
├── shared/                     # общий код единого модуля (не отдельные Go-модули)
│   ├── eventbus/                # конверт события (Meta, World, Scope), Bus/Journal, DLQ, kafka
│   ├── jsonpath/                 # универсальный доступ по dot-path к map[string]any
│   ├── contracts/                # реестр типов событий, JSON Schema 2020-12, OwnershipRules
│   ├── entity/                   # модель сущности v2 (Op/ApplyOps/StateHash/History)
│   ├── objstore/                 # интерфейс объектного хранилища (minio + in-memory)
│   ├── env/                      # реестр переменных окружения (префикс MV_)
│   ├── logging/                  # slog с обязательными полями, редакция секретов
│   ├── runtime/                  # HTTP-сервер процесса, Context{Start/Stop/Health}
│   ├── clock/                    # Clock/Timers (реальные и управляемые для тестов)
│   ├── agent/                    # каркас роя агентов GM (типы, парсер MD-блупринтов);
│   │                              # целевой рантайм — internal/swarm (EPIC-003)
│   └── testkit/                  # membus, contract-тест шины, фейки для тестов и e2e
│       ├── membus/, contract/     # in-memory Bus/Journal + contract-тест против kafka
│       ├── state/, mechanics/     # FakeState, FixedMechanics (заглушки до EPIC-002)
│       ├── gateway/, swarm/       # Harness, FakeNarrator (заглушки до EPIC-003/004)
│       └── containers.go, versions.go
├── schemas/events/              # JSON-схемы событий — источник для shared/contracts
├── rules/dark-forest.yaml        # детерминированная механика (Load, формулы, статы)
├── testdata/fixtures/            # фикстуры мира для mvctl world init (EPIC-002) и e2e
├── build/                        # Dockerfile платформы, legacy.Dockerfile, minio.Dockerfile,
│                                  # minio-init.sh, redpanda-init.sh, versions.env
├── docker-compose.yml             # профили: (default) / memory / gpu / dev
├── docker-compose.bot.yml         # профиль bot; подключает Makefile по PROFILES=...
├── docker-compose.legacy.yml      # профиль legacy; подключает Makefile по PROFILES=...
├── Makefile                       # единая точка входа оператора (`make help`)
└── services/                      # код вне единого модуля (свои go.mod) — см. ниже
    └── _archive/                  # архив выведенного из сборки кода (git mv, не удаление)
```

### Статус кода в `services/`

`services/*` — отдельные Go-модули (собственный `go.mod` в каждом), поэтому
`go build ./...` из корня их не видит. Подробности и обоснование по каждой строке —
[`services/_archive/README.md`](services/_archive/README.md).

| Статус | Каталоги | Смысл |
|---|---|---|
| Источник переписывания | `entity-manager`, `rule-engine`, `game-service` | референс при переносе на `internal/state`/`internal/mechanics`/`internal/gateway`; после переноса — в архив |
| Legacy (профиль compose `legacy`) | `narrative-orchestrator`, `semantic-memory` | работают as-is под флагом `MV_GM_PATH=legacy`, пока рой GM не заменит нарратив (EPIC-003 I2) |
| Заморожен (`FROZEN.md` в каталоге) | `world-generator`, `universe-genesis-oracle`, `ontological-archivist`, `cultivation-module`, `plan-manager`, `city-governor`, `entity-actor`, `evolution-watcher` | вне MVP-1; возврат — EPIC-006…EPIC-010 |
| Архив (`services/_archive/**`) | `ban-of-world`, `reality-monitor`, `shared/{schema,redis,config,minio,oracle,rules,intent,tinyml,spatial}` и др. | функциональность заменена (`internal/mechanics`, `shared/objstore`, `shared/env`, законы мира); ничего не удалено, история — `git log --follow` |

### Инфраструктура

| Компонент | Назначение |
|---|---|
| **Redpanda** | шина событий (Kafka API); топики платформы — `shared/eventbus/topics.go` |
| **MinIO** | объектное хранилище состояния/снапшотов; собственный образ (`build/minio.Dockerfile`, ADR-021) |
| **Qdrant** | векторный поиск (профиль `memory`) — заменяет ChromaDB в целевой архитектуре |
| **Neo4j** | графовая память (профиль `memory`) |
| **llama-server (llama.cpp)** | основной LLM-рантайм — нативный процесс вне compose (`make llm-up`), провайдер `openai_compat` |
| **Ollama** | опциональный второй LLM-рантайм (`MV_LLM_PROVIDER=ollama`), профиль compose `gpu` |
| **ChromaDB** | только профиль `legacy`, вместе с as-is `semantic-memory`; в целевой архитектуре не используется |

TimescaleDB и Redis в целевой архитектуре не используются (заменены CSV/CLI-метриками
и снапшотом состояния в памяти соответственно).

## Команды сборки/линта/теста

```bash
# Сборка
make build                      # go build -o bin/ ./cmd/...  (multiverse, mvctl)
make image                      # docker build -f build/Dockerfile . — образ платформы
make minio-image                # docker build -f build/minio.Dockerfile . — MinIO из исходников

# Запуск
make llm-up / llm-down / llm-health   # нативный llama-server вне compose
make up                         # docker compose up -d --wait + health (LLM не строго)
make down                       # остановить контейнеры, тома сохраняются
make health                     # /health каждого процесса + LLM + возраст бэкапа
make logs SERVICE=<имя>         # docker compose logs -f --tail=200

# Тесты
make test                       # go test -short -race ./... + порог покрытия internal/*
make test-integration           # testcontainers: Redpanda, MinIO (наш образ), Qdrant, Neo4j
make test-e2e                   # e2e в одном процессе, --bus=memory
make contracts                  # mvctl contracts check / env check + валидность JSON-схем
make ci / make ci-full          # всё из CI (без Docker) / с test-integration
make lint                       # golangci-lint run
make secrets-scan               # gitleaks по диапазону ветки + содержимому индекса (не рабочей копии)
```

Полный список — `make help`. Версии образов и инструментов — только в
[`build/versions.env`](build/versions.env) (единственный источник; читают Makefile,
`docker-compose.yml`, `shared/testkit.Versions()`).

### Прямые команды Go

```bash
go build ./...                  # весь корневой модуль (без services/*, у них свои go.mod)
go vet ./...
go test -short -race ./...
go run ./cmd/multiverse serve --contexts=all --bus=memory   # один процесс, шина в памяти, без Docker
                                                      # (serve можно опустить: так запускают compose и образ)
go run ./cmd/mvctl contracts check                    # реестр типов событий без «фантомов»
```

## Паттерны и соглашения кода

### Запреты линтера (`forbidigo` в `.golangci.yml`)

Четыре запрещённых семейства вызовов — каждое ловится `golangci-lint run` и валит
`make lint`/`make ci`:

| Запрещено | Вместо этого | Почему |
|---|---|---|
| `os.Getenv`, `os.LookupEnv`, `os.Environ`, `os.ExpandEnv` | `shared/env` (реестр `MV_*`) | переменная должна попасть в манифест и в `.env.example` (NFR-074) |
| `time.Now` | `shared/clock.Clock` | иначе повтор партии (replay) не воспроизводим |
| `time.After`, `time.Tick`, `time.NewTimer`, `time.NewTicker`, `time.Since`, `time.Until` | `shared/clock.Timers` | иначе таймеры идут по настенному времени и replay не воспроизводим |
| `log.Print*`, `log.Fatal*`, `log.Panic*` | `log/slog` (`shared/logging`) | платформа отдаёт только структурные JSON-логи |

### Конверт события (`shared/eventbus`)

Сквозные поля события — в конверте (`Meta`), а не в payload: `World`, `Scope` —
поля верхнего уровня `Event`, `Meta` несёт `correlation_id`/`causation_id`/
`actor_kind`/`agent`/`replay`/`locale`/`gm_path`. Публикатор строит событие
конструкторами, а не вручную:

```go
// Корневое событие цепочки (нет причины) — этот вызов ставит новый
// correlation_id, ActorKind и GMPath по умолчанию.
root := eventbus.NewRoot("player.attacked", "gateway", worldID, scope, eventbus.ActorHuman, payload)

// Производное событие наследует World/Scope и трассу причины (Correlation/Causation),
// а также Timestamp — это нужно для побайтового повтора при replay (NFR-061).
fact := eventbus.Derive(root, "combat.decided", "core/mechanics", payload,
    eventbus.WithAgent(eventbus.AgentRef{ID: "encounter-wolf:solo:p1", Level: "task"}))

bus.Publish(ctx, fact)
```

Чтение payload — через `event.Path()` (`*jsonpath.Accessor`, универсальный доступ
по dot-path к `map[string]any`); это НЕ относится к `World`/`Scope`, которые —
поля события, а не payload:

```go
pa := event.Path()
level, _ := pa.GetInt("stats.level")
if pa.Has("objectives") { /* ... */ }

worldID := eventbus.GetWorldIDFromEvent(event) // event.World.Entity.ID, либо legacy-фолбэк из payload
```

`GetWorldIDFromEvent`, `GetEntityIDWithFallback`, `GetTargetEntityID` — совместимость
со старыми (as-is/legacy) продюсерами, у которых `world`/`entity`/`target` лежали
плоскими ключами внутри payload; в новом коде эти поля читаются из `Event.World`/
`Event.Scope` напрямую, а не через fallback-цепочку.

Любое изменение публичного API `shared/eventbus` (конверт, `Bus`, `Journal`) —
**только через системного архитектора** (пометка `contract-change`,
`Docs/dev-team/plan/ownership.md`).

### Реестр типов событий (`shared/contracts`)

Каждый публикуемый тип события должен быть зарегистрирован (`Spec{Type, Topic,
Schema, Publishers, Consumers, Policy}`) и иметь файл JSON-схемы в
`schemas/events/*.v1.json` (JSON Schema 2020-12, `santhosh-tekuri/jsonschema/v6`).
`go run ./cmd/mvctl contracts check` ловит рассинхронизацию («тип без схемы» и
наоборот) — запускать после добавления или переименования типа события.

### Универсальный доступ `shared/jsonpath`

Пакет работает с любым `map[string]any`, не только с событиями:

```go
import "multiverse-core.io/shared/jsonpath"

acc := jsonpath.New(anyData)
val, _ := acc.GetString("config.db.host")
port, _ := acc.GetInt("server.ports[0]")
for _, path := range acc.GetAllPaths() { fmt.Println(path) }
```

Подробный API — [`shared/jsonpath/README.md`](shared/jsonpath/README.md); формат
конверта и правила эволюции схем — [`shared/eventbus/README.md`](shared/eventbus/README.md).

### Тестовые заглушки (`shared/testkit`)

До реализации своих эпиков команды используют общие заглушки на in-memory шине
(`shared/testkit/membus`): `testkit/state.FakeState`, `testkit/mechanics.FixedMechanics`,
`testkit/gateway.Harness`, `testkit/swarm.FakeNarrator`. Сигнатуры заглушек совпадают
с целевыми реализациями — замена не требует правок у потребителей (проверяется
компиляцией тестов-потребителей). Contract-тест шины (`shared/testkit/contract`)
гоняет один и тот же набор проверок против `membus` и настоящей Redpanda.

## Рабочий процесс разработки

1. Клонировать репозиторий, установить Go 1.26, Docker Desktop, GNU make
   (`winget install ezwinports.make` на Windows).
2. `cp .env.example .env`, заполнить всё, что помечено `[required]` в комментарии
   над переменной в `.env.example` — правило, а не фиксированный список: состав
   меняется вместе с файлом (сейчас шесть переменных).
3. `make minio-image && make up && make health` — поднять инфраструктуру и пустые
   контексты платформы.
4. Сборка — одна цель на весь модуль: `make build` (`go build -o bin/ ./cmd/...`).
   Целей `make build-service SERVICE=<имя>` и `make build-all` (для набора отдельных
   сервисов из `go.work`) в текущей раскладке нет — модуль один.

Подробности — [`README.md`](README.md) («Запуск за 5 команд») и
[`Docs/ops/runbook.md`](Docs/ops/runbook.md) (эксплуатация, LLM, восстановление).

### Ключевые файлы

- [`docker-compose.yml`](docker-compose.yml) — стек по умолчанию и профили `memory`/`gpu`/`dev`;
  профили `bot` и `legacy` — в `docker-compose.bot.yml`/`docker-compose.legacy.yml`, Makefile
  подключает их по `PROFILES=...` (врезка «Профили `bot` и `legacy`» в README.md).
- [`build/versions.env`](build/versions.env) — единственный источник версий образов и toolchain.
- [`.env.example`](.env.example) — полный список переменных окружения (сверяется `mvctl env check`).
- [`Docs/dev-team/`](Docs/dev-team/) — архитектура, ADR, контракты, эпики и задачи
  (артефакты команды разработки; ведёт оркестратор процесса, не редактировать вручную).
