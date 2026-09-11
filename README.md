# Multiverse-Core

Событийная платформа «живых миров» на Go: единый модуль (`multiverse-core.io`), один
бинарник `cmd/multiverse` (набор поднятых контекстов задаётся флагом `--contexts`,
а не сборкой) и оператор-CLI `cmd/mvctl`. Шина — Redpanda (Kafka API); объектное
хранилище состояния — MinIO, собранный из исходников; опционально — Qdrant + Neo4j
(векторная и графовая память) и локальный LLM (`llama-server`/Ollama) для нарратива.

> Статус: репозиторий проходит переход на новую раскладку (`EPIC-001 «Фундамент»`,
> `Docs/dev-team/epics/EPIC-001-foundation/`). На этом этапе платформа поднимается
> целиком (единый модуль, инфраструктура, `/health` у каждого процесса), но игровая
> логика (`internal/state`, `internal/swarm`, `internal/gateway` и т. д.) ещё не
> реализована — контексты `state`, `mechanics`, `swarm`, `llm`, `laws`, `gateway`,
> `memory` отвечают заглушками. Список сервисов до раскладки, их статус (архив /
> заморожен / источник переписывания) — в [`services/_archive/README.md`](services/_archive/README.md)
> и в разделе «Статус кода вне единого модуля» ниже.

## Требования

- Windows 11 (основная среда разработки) или Linux/WSL.
- Docker Desktop (`docker compose` — плагин Docker CLI, а не отдельный бинарник
  `docker-compose` v1; версия плагина на стенде владельца — v5.2.0).
- Go **1.26** (см. точный патч в [`build/versions.env`](build/versions.env), `GO_VERSION`).
- Git, GNU make, PowerShell 7 (для LLM-скриптов на Windows).
- Инструменты разработчика для `make ci` (цель — `lint test contracts secrets-scan
  privacy-scan vuln compose-lint test-e2e`, см. «Участие в разработке»): `golangci-lint`
  (пин `GOLANGCI_LINT_VERSION=v2.13.2` в [`build/versions.env`](build/versions.env), тем же
  пином пользуется `golangci-lint-action` в CI), `govulncheck`, `gitleaks`
  (пин `GITLEAKS_VERSION=v8.30.1`) и `python3` — без него `compose-lint` завершается ошибкой
  «python3 is required to read the compose model» (`scripts/compose-lint.sh`). Без них
  `make ci` локально не пройдёт целиком; в CI автоматически ставятся только `golangci-lint`,
  `gitleaks` и `govulncheck` (`.github/workflows/go.yml`), `python3` есть на раннере
  `ubuntu-latest` по умолчанию. `pre-commit` (хук из T-001, `.pre-commit-config.yaml`) в
  `make ci` не участвует — он нужен только для локального хука перед коммитом.
- Пакеты через `winget` (Windows):
  ```powershell
  winget install ezwinports.make      # GNU make для Git Bash / PowerShell
  winget install jqlang.jq            # используется scripts/llm-bench.sh
  winget install FiloSottile.age      # шифрование бэкапа links.db; сегодня не вызывается ни
                                       # одной целью Makefile, понадобится с EPIC-004 (links.db)
  winget install Gitleaks.Gitleaks    # локальный pre-commit-хук и make secrets-scan без go install
  ```
  Без `make` тот же результат даёт `wsl make <цель>`.

## Запуск за 5 команд

> **Эти пять команд ещё никто не прогонял.** GNU make не установлен ни на одной
> машине, где шла разработка T-001…T-398 — ни один участник ни разу не выполнил
> ни одной цели `make` по-настоящему (правки `Makefile` проверялись построчной
> эмуляцией оболочки), и CI цели `make` тоже не вызывает. Подробности и что
> сделать первым, кто прогоняет это на чистой машине — [`Docs/ops/runbook.md`](Docs/ops/runbook.md),
> врезка «Порядок ниже не прогнан целиком».

```bash
cp .env.example .env                # заполнить переменные, отмеченные `[required]` в комментарии над
                                     # ними — обязательный минимум для активного набора профилей
                                     # (COMPOSE_PROFILES=memory в примере); пустую переменную в .env
                                     # комментировать ТОЛЬКО строкой выше, не после `=` (T-397, см. ниже)
make llm-up                         # старт нативного llama-server (127.0.0.1:1234); можно пропустить —
                                     # платформа стартует и без LLM (деградация нарратива, FR-080)
make minio-image                    # сборка своего образа MinIO из исходников (~5 мин первый раз)
make up                             # docker compose up -d --wait + make health (не строго по LLM)
make health                         # таблица статусов gateway/core/memory/LLM + возраст бэкапа
```

`make up` поднимает набор сервисов по умолчанию (`redpanda`, `minio`, `gateway`, `core`,
плюс одноразовые init-контейнеры `redpanda-init`/`minio-init`, которые создают топики и
бакеты и завершаются) плюс профили из `COMPOSE_PROFILES` в `.env` (`memory`, `gpu`, `dev`
— см. `Makefile`, переменная `PROFILES` переопределяет их разово:
`make up PROFILES=memory`).
LLM (`llama-server`) — нативный процесс вне compose: `make up`/`make down` его не
трогают, им управляют `make llm-up` / `make llm-down` / `make llm-health`.

> **Стек — только через `make`.** Версии образов лежат в `build/versions.env`, и
> compose видит этот файл только через `COMPOSE_ENV_FILES` в окружении своего
> процесса. Строку `COMPOSE_ENV_FILES` в `.env` compose не читает: переменная
> называет env-файлы и поэтому не может прийти из одного из них (проверено на
> compose v5.2, T-412). Makefile её экспортирует, а голый `docker compose config`
> или `up` без `make` падает на первой переменной образа (`REDPANDA_IMAGE` или
> другой `*_IMAGE`). Для ручной команды compose
> (`exec`, `logs`, `restart`) задайте переменную в оболочке один раз:
> `export COMPOSE_ENV_FILES=.env,build/versions.env`
> (PowerShell: `$env:COMPOSE_ENV_FILES = ".env,build/versions.env"`).

> **Профили `bot` и `legacy` — только через `make`.** Оба описаны в собственных
> compose-файлах (`docker-compose.bot.yml`, `docker-compose.legacy.yml`) и
> подключаются Makefile'ом, только когда профиль реально запрошен:
> `make up PROFILES=bot`, `make up PROFILES=memory,legacy` или
> `COMPOSE_PROFILES=...` в `.env`. Причина — `docker compose` интерполирует
> файл целиком ДО отбора по профилям, и обязательные переменные этих сервисов
> (токен бота, образ Chroma) раньше валили `make up` всем подряд, в том числе
> тем, кто эти профили не поднимает (T-397). **Голый `docker compose --profile
> bot up` без Makefile официально не поддерживается:** без `COMPOSE_ENV_FILES`
> в оболочке он не поднимет ничего и откажет на первой переменной образа
> (`*_IMAGE`, врезка выше), а с переменной, но без `-f docker-compose.bot.yml`,
> не увидит файл бота и поднимет стек без бота. Это осознанное решение, а не пробел.
> Профиль `bot` сегодня всё равно не стартует: бинарник `cmd/telegram-bot`
> появится в EPIC-004. `make up PROFILES=legacy` сам сначала выполняет
> `legacy-src` (цель `up` объявляет её своей предпосылкой) — тот делает
> `rm -rf build/.legacy-src` и `git archive` из `LEGACY_SRC_REF`, поэтому
> первый запуск профиля дольше обычного; отдельно вызывать `make legacy-src`
> не нужно.

Полный порядок первого запуска, диагностика, профили и типовые инциденты — в
[`Docs/ops/runbook.md`](Docs/ops/runbook.md).

## Архитектура: карта пакетов

Модуль один (`go.mod` в корне, `module multiverse-core.io`, `go.work` больше нет).
Контексты платформы — пакеты, а не отдельные сборки; какие из них запущены в
конкретном процессе, решает флаг `--contexts` бинарника `cmd/multiverse`.

```
multiverse-core/
├── cmd/
│   ├── multiverse/        # единственный бинарник платформы: serve/health/db/version,
│   │                       # флаги serve --contexts/--mode/--bus/--recording
│   │                       # (serve можно опустить: `multiverse --contexts=...`)
│   └── mvctl/              # CLI оператора: contracts, env, storage, privacy, version;
│                            # world/blueprint/laws/record/golden/llm/memory/report/trace
│                            # зарезервированы под будущие эпики (см. cmd/mvctl/main.go)
├── internal/
│   └── mechanics/          # типы механики, Load(rules/*.yaml), формулы, RNG
│                            # (internal/state, internal/swarm, internal/llm,
│                            #  internal/laws, internal/gateway, internal/memory —
│                            #  появятся по мере реализации своих эпиков)
├── shared/
│   ├── eventbus/           # конверт события, Bus/Journal, DLQ, kafka-реализация
│   ├── jsonpath/           # универсальный доступ по dot-path к любым map[string]any
│   ├── contracts/          # реестр типов событий, JSON Schema 2020-12, OwnershipRules
│   ├── entity/             # модель сущности v2 (Op/ApplyOps/StateHash/History)
│   ├── objstore/           # интерфейс объектного хранилища (MinIO + in-memory)
│   ├── env/                # реестр переменных окружения (префикс MV_)
│   ├── logging/            # slog с обязательными полями и редакцией секретов
│   ├── runtime/            # HTTP-сервер процесса, Context{Start/Stop/Health}
│   ├── clock/              # Clock/Timers (реальные и управляемые для тестов)
│   ├── agent/               # каркас роя агентов GM (типы, парсер блупринтов; целевой
│   │                         # рантайм — internal/swarm, EPIC-003)
│   └── testkit/             # membus, contract-тест шины, фейки (FakeState,
│                             # FixedMechanics, Harness, FakeNarrator) для тестов
│                             # и e2e других команд
├── schemas/events/          # JSON-схемы событий (source of truth для shared/contracts)
├── rules/                   # rules/dark-forest.yaml — детерминированная механика
├── testdata/fixtures/       # фикстуры мира для mvctl world init / e2e
├── build/                   # Dockerfile платформы, legacy.Dockerfile, minio.Dockerfile,
│                             # minio-init.sh, redpanda-init.sh, versions.env
├── docker-compose.yml       # профили: memory / gpu / dev; bot и legacy — в docker-compose.bot.yml /
│                             # docker-compose.legacy.yml (Makefile подключает их по `PROFILES=...`)
├── Makefile                 # единая точка входа оператора (см. `make help`)
└── services/                # код вне единого модуля — см. таблицу ниже
    └── _archive/            # архив выведенного из сборки кода (git mv, не удаление)
```

Подробное обоснование раскладки — `Docs/dev-team/architecture/overview.md`
(разделы «Целевое состояние», карта контекстов); правила именования событий и
пример доступа к ним из кода — `shared/jsonpath/README.md`, `shared/eventbus/README.md`.

## Статус кода вне единого модуля

`services/*` — не часть корневого модуля: каждый каталог ниже имеет собственный
`go.mod`, поэтому `go build ./...` из корня их не видит. Полный список с причиной,
коммитом архивации и путём возврата — [`services/_archive/README.md`](services/_archive/README.md).

| Статус | Каталоги | Что это значит |
|---|---|---|
| **Источник переписывания** (в дереве, вне сборки, не архив) | `services/entity-manager`, `services/rule-engine`, `services/game-service` | код читают как референс при переписывании на `internal/state`, `internal/mechanics`, `internal/gateway`; после переноса — в `services/_archive/` |
| **Legacy** (профиль compose `legacy`, свои as-is переменные) | `services/narrative-orchestrator`, `services/semantic-memory` | работают как есть, пока `MV_GM_PATH=legacy`; нужны для сравнения нарратива до замены роем GM; после миграции (EPIC-003 I2) — в архив |
| **Заморожен** (`FROZEN.md` в каталоге, вне сборки и compose) | `services/world-generator`, `services/universe-genesis-oracle`, `services/ontological-archivist`, `services/cultivation-module`, `services/plan-manager`, `services/city-governor`, `services/entity-actor`, `services/evolution-watcher` | код игровых расширений вне MVP-1 (генерация миров, культивация, города, эволюция NPC); возврат — эпики EPIC-006…EPIC-010 |
| **Архив** (`services/_archive/**`, перенесено `git mv`, есть `ARCHIVED.md`) | `services/ban-of-world`, `services/reality-monitor`, `shared/{schema,redis,config,minio,oracle,rules,intent,tinyml,spatial}`, `configs/gm_*.yaml` (6 файлов), `shared/agent/tools/*` (кроме `registry.go`) и `filter.go`, `fake_deps/`, `test_minio.go`, старые `Dockerfile` и др. | функциональность заменена (законы мира, `internal/mechanics`, `shared/objstore`, `shared/env` и т. д.) или не входит в стек MVP-1; **ничего не удалено**, история читается `git log --follow` |

## Сборка и тесты

```bash
make build              # go build -o bin/ ./cmd/...  (сейчас: multiverse, mvctl; cmd/telegram-bot появится в EPIC-004)
make lint               # golangci-lint run
make test               # go test -short -race ./... + порог покрытия ключевых internal/*
make test-integration   # testcontainers: Redpanda, MinIO (наш образ), Qdrant, Neo4j
make test-e2e           # e2e в одном процессе, шина в памяти (--bus=memory)
make contracts          # mvctl contracts check / env check + валидность JSON-схем
make ci                 # всё из CI, что не требует Docker; make ci-full добавляет test-integration
```

Полный список целей — `make help`. Каждая версия/образ фиксируется один раз в
[`build/versions.env`](build/versions.env) — Makefile, `docker-compose.yml` и тесты
(`shared/testkit.Versions()`) читают его, а не хранят версии по отдельности.

## Конфигурация

Все переменные платформы — с префиксом `MV_` и объявлены в реестре `shared/env`;
переменные сторонних образов (`MINIO_*`, `NEO4J_*`, `OLLAMA_*`) префикса не имеют.
Полный и единственный источник значений — [`.env.example`](.env.example)
(копируется в `.env`, секреты там всегда пусты). Проверка соответствия кода и
примера: `go run ./cmd/mvctl env check`.

## Документация

- [`CLAUDE.md`](CLAUDE.md), [`AGENTS.md`](AGENTS.md) — соглашения для агентов и
  разработчиков (карта пакетов, паттерны событий, доступ к данным).
- [`Docs/ops/runbook.md`](Docs/ops/runbook.md) — эксплуатация: запуск с нуля,
  остановка/перезапуск, LLM (`llama-server`), восстановление, ротация токена бота,
  карточки процессов.
- [`services/_archive/README.md`](services/_archive/README.md) — индекс архива.
- [`Docs/dev-team/`](Docs/dev-team/) — артефакты команды: требования, архитектура,
  ADR, эпики и задачи (ведёт оркестратор процесса разработки, не редактировать вручную).

## Участие в разработке

1. Форк/ветка от `integration/mvp-1` (`epic/EPIC-00N-<slug>` для командной работы).
2. `make ci` зелёный локально перед PR.
3. Секреты — `gitleaks git --redact` = 0 (`.gitleaks.toml`, allowlist только `*.example`).
4. Изменения в `shared/{eventbus,contracts,entity}` и `schemas/events/_common.json` —
   только с пометкой `contract-change` (см. `Docs/dev-team/plan/ownership.md`).

## Лицензия

Файл `LICENSE` в репозитории отсутствует — условия использования не определены;
уточнить у владельца перед публикацией или переиспользованием кода.
