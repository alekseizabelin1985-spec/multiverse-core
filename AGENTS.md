# AGENTS.md

Инструкции для агентов при работе с этим репозиторием. Платформо-специфичные
детали (карта пакетов, конверт события, команды) — в [`CLAUDE.md`](CLAUDE.md);
этот файл — более короткая выжимка конвенций для быстрой ориентации.

## Обзор проекта

Событийная платформа «живых миров» на Go 1.26: **единый модуль**
(`module multiverse-core.io`, `go.work` не используется), один бинарник платформы
`cmd/multiverse` (набор поднятых контекстов — флаг `--contexts`, не отдельная
сборка) и CLI оператора `cmd/mvctl`. Шина — Redpanda (Kafka API).

Репозиторий проходит переход на эту раскладку (`EPIC-001 «Фундамент»`); часть
контекстов (`state`, `mechanics`, `swarm`, `llm`, `laws`, `gateway`, `memory`) пока
зарегистрированы как заглушки, отвечающие только `/health: ok`. Код вне единого
модуля (`services/*`, у каждого свой `go.mod`) — активен частично; статус каждого
каталога — в [`services/_archive/README.md`](services/_archive/README.md) и в
таблице ниже.

## Команды сборки/линта/теста

- Сборка всего модуля: `make build` (`go build -o bin/ ./cmd/...`)
- Поднять инфраструктуру: `make minio-image && make up`
- Проверить статус: `make health`
- Остановить: `make down`
- Логи одного сервиса compose: `make logs SERVICE=<имя>`
- Тесты (unit, без сети): `make test`
- Тесты с контейнерами (Redpanda/MinIO/Qdrant/Neo4j): `make test-integration`
- e2e в одном процессе без Docker: `make test-e2e`
- Контракты/схемы: `make contracts`
- Всё как в CI (без Docker): `make ci`; с контейнерами — `make ci-full`

Полный список — `make help`. Целей `make build-service SERVICE=<имя>`,
`make run SERVICE=<имя>`, `make logs-service SERVICE=<имя>` больше **нет** — они
были нужны для набора независимых сервисов workspace, которого в текущей
раскладке не существует.

## Соглашения по коду

- Модуль один: `go build ./...` из корня собирает `cmd/*`, `internal/*`, `shared/*`
  (без `services/*` — у них свои `go.mod`, они вне корневого модуля).
- Go 1.26 везде (см. `go.mod`, `build/versions.env`); `CGO_ENABLED=0` для сборки
  образа платформы (`build/Dockerfile`, `distroless/static-debian12` в рантайме).
- Конфигурация процессов — только переменные окружения с префиксом `MV_`,
  зарегистрированные в `shared/env`; `os.Getenv`/`os.LookupEnv`/`os.Environ`/
  `os.ExpandEnv` вне `shared/env` запрещены линтером (`forbidigo` в `.golangci.yml`).
- Время — только через `shared/clock`: `time.Now`, `time.After`/`Tick`/`NewTimer`/
  `NewTicker`/`Since`/`Until` вне `shared/clock` тоже под запретом `forbidigo` —
  иначе повтор партии (replay) перестаёт быть детерминированным.
- Логирование — только `log/slog` (`shared/logging`); `log.Print*`/`Fatal*`/`Panic*`
  под тем же запретом `forbidigo` — платформа отдаёт исключительно структурные
  JSON-логи.
- JSON Schema для событий — 2020-12 (`santhosh-tekuri/jsonschema/v6`), не Draft 7.
- Идентификаторы событий и сущностей — UUID (`github.com/google/uuid`), кроме
  режима `--id-source=sequence` (детерминированные id для replay/тестов).

### Конверт события и доступ к данным (`shared/eventbus`, `shared/jsonpath`)

Сквозные поля события — в `Meta` и в полях `World`/`Scope` верхнего уровня
`eventbus.Event`, а не внутри `payload`. Публиковать события — только через
конструкторы, не собирать `Event{}` вручную:

```go
// Корневое событие цепочки:
root := eventbus.NewRoot("player.attacked", "gateway", worldID, scope, eventbus.ActorHuman, payload)

// Производное — наследует World/Scope, Correlation/Causation и Timestamp причины
// (нужно для побайтового повтора при replay):
fact := eventbus.Derive(root, "combat.decided", "core/mechanics", payload)

bus.Publish(ctx, fact)
```

Чтение полей **payload** — через `event.Path()` (`*jsonpath.Accessor`, dot-notation,
безопасные геттеры):

```go
pa := event.Path()
level, _ := pa.GetInt("stats.level")
active, _ := pa.GetBool("active")
if pa.Has("objectives") { /* ... */ }
```

`World`/`Scope` читаются как поля события, а не через `Path()`:
`eventbus.GetWorldIDFromEvent(event)` возвращает `event.World.Entity.ID` (с
фолбэком на плоские ключи payload — только для совместимости с legacy-продюсерами
профиля `legacy`, в новом коде не нужен).

Любое изменение публичного API `shared/eventbus` (конверт, `Bus`, `Journal`) и
`schemas/events/_common.json` — **только через системного архитектора**
(`Docs/dev-team/plan/ownership.md`, пометка `contract-change`).

### Реестр типов событий (`shared/contracts`)

Каждый публикуемый тип — запись `Spec{Type, Topic, Schema, Publishers, Consumers}`
и файл `schemas/events/*.v1.json`. После добавления/переименования типа — прогнать
`go run ./cmd/mvctl contracts check` (ловит несоответствия «тип без схемы» и
наоборот).

## Карта каталогов

```
cmd/multiverse/     # бинарник платформы (serve/health/db), флаг --contexts
cmd/mvctl/           # CLI оператора: contracts, env, storage, privacy, version (реализованы),
                      # world/blueprint/laws/record/golden/llm/memory/report/trace (зарезервированы)
internal/mechanics/  # единственный реализованный доменный пакет на сейчас
shared/eventbus/     # конверт события, Bus/Journal, DLQ, kafka-реализация
shared/jsonpath/      # универсальный доступ по dot-path к map[string]any
shared/contracts/     # реестр типов событий + JSON-схемы
shared/entity/        # модель сущности v2
shared/objstore/      # объектное хранилище (minio + in-memory)
shared/env/           # реестр переменных окружения
shared/logging/       # slog с обязательными полями
shared/runtime/       # HTTP-сервер процесса, Context{Start/Stop/Health}
shared/clock/         # Clock/Timers (реальные и управляемые)
shared/agent/         # каркас роя агентов GM (целевой рантайм — internal/swarm)
shared/testkit/       # membus, contract-тест шины, фейки для тестов/e2e
schemas/events/       # JSON-схемы событий
rules/dark-forest.yaml # детерминированная механика
testdata/fixtures/    # фикстуры мира
build/                # Dockerfile платформы, legacy.Dockerfile, minio.Dockerfile,
                      # minio-init.sh, redpanda-init.sh, versions.env
services/             # код вне единого модуля (свои go.mod) — см. таблицу ниже
services/_archive/    # архив выведенного из сборки кода
```

## Статус кода в `services/`

| Статус | Каталоги | Смысл |
|---|---|---|
| Источник переписывания | `entity-manager`, `rule-engine`, `game-service` | референс при переносе на `internal/state`/`internal/mechanics`/`internal/gateway` |
| Legacy (профиль compose `legacy`) | `narrative-orchestrator`, `semantic-memory` | as-is, пока `MV_GM_PATH=legacy`; уходят в архив после EPIC-003 I2 |
| Заморожен (`FROZEN.md`) | `world-generator`, `universe-genesis-oracle`, `ontological-archivist`, `cultivation-module`, `plan-manager`, `city-governor`, `entity-actor`, `evolution-watcher` | вне MVP-1, возврат — EPIC-006…EPIC-010 |
| Архив (`services/_archive/**`) | `ban-of-world`, `reality-monitor`, `shared/{schema,redis,config,minio,oracle,rules,intent,tinyml,spatial}` и др. | заменены; ничего не удалено, `git log --follow` читается |

Подробный список с коммитами и причинами — [`services/_archive/README.md`](services/_archive/README.md).
Файлов `services/<имя>/AGENTS.md` в репозитории сейчас нет — вся специфика служебного
кода описывается в `ARCHIVED.md`/`FROZEN.md` соответствующего каталога, а не в
отдельных `AGENTS.md`.

## Нестандартная структура

- Точки входа платформы — `cmd/multiverse`, `cmd/mvctl` (не внутри `services/*`).
- Домен на сегодня — только `internal/mechanics`; остальные `internal/*`
  (`state`, `swarm`, `llm`, `laws`, `gateway`, `memory`) появятся по мере эпиков.
- Документация команды разработки — `Docs/dev-team/**` (архитектура, ADR, контракты,
  эпики и задачи); ведёт оркестратор процесса, вручную не редактировать.
- Эксплуатационная документация — [`Docs/ops/runbook.md`](Docs/ops/runbook.md).
