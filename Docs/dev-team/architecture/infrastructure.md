# Инфраструктура и доставка

Версия 0.1 · 2026-09-09 · devops-engineer · статус: **предложение к G2** (реализация — EPIC-001, задачи F-1, F-6, F-7, F-8; см. `plan/epics.md` §6).
Основание: `architecture/overview.md` (§3, §5, §9, §13, §15–§19), ADR-001, ADR-004, ADR-005, ADR-007, ADR-009, ADR-010, `contracts.md` (§0, C-01, C-14), `plan/ownership.md`, `audit-facts.md` (§1, §3, §7, §8), `requirements/nfr.md` (§2, §4, §6, §8, «Что измерить первым»), `project/metrics.md` (§6, §7), `analysis/integrations.md` (§3, §4, §7).
Правило документа: **значения секретов нигде не приводятся** — только имена переменных. Код и конфиги проекта этим документом не меняются; всё ниже — задания для разработчика и devops в EPIC-001.

Версии образов и инструментов проверены 2026-09-09 по Docker Hub, `proxy.golang.org` и GitHub Releases (раздел 2.4). Пины — в одном месте (`docker-compose.yml`, `go.mod`, `.github/workflows/go.yml`, `.pre-commit-config.yaml`); обновление пинов — отдельные задачи, не «по дороге».

---

## 0. Сводка решений

| Область | Решение | Где закреплено |
|---|---|---|
| Окружения | три: `dev` (машина владельца, Docker Desktop + Git Bash), `ci` (GitHub-hosted `ubuntu-latest` с Docker, без GPU), `prod` (та же машина владельца, набор compose-профилей `memory,gpu-или-native,bot`) | §1 |
| Топология | один образ `multiverse-core` с тремя бинарниками; процессы `gateway`, `core`, `memory`, `telegram-bot`; профили compose `memory`, `gpu`, `bot`, `dev`, `legacy` | §1.2, §1.3 |
| Сборка | единый модуль `multiverse-core.io`, `go 1.25` + `toolchain go1.25.14`; Makefile — одна цель на действие (`build`, `lint`, `test`, `ci`, `up`, `down`, `health`, `warm`, `bench`, `backup`); Dockerfile multi-stage с кэшем модулей/сборки, runtime `distroless/static:nonroot` | §2 |
| CI | `.github/workflows/go.yml`: `unit`, `integration` (testcontainers-go), `e2e` (один процесс, без Docker), `contracts`, `security` (gitleaks + govulncheck), `compose-lint`; ≤ 10 мин; `validate-blueprints.yml` удаляется; `qwen-*` не трогаем | §3 |
| Конфигурация | только env с префиксом `MV_` через единственный пакет `shared/env` (реестр переменных); `.env.example` полный и сверяется тестом; `os.Getenv` вне `shared/env` запрещён линтером | §4.1–§4.3 |
| Секреты | `.env`, `.mcp.env` вне индекса; `gitleaks` в pre-commit и CI; compose без паролей по умолчанию (`MINIO_ROOT_PASSWORD`, `NEO4J_PASSWORD` обязательны); `git filter-repo` — только по команде владельца | §4.4, §4.5 |
| Данные | Redpanda 8 топиков `-p 1 -r 1` с `retention.ms` по классам; MinIO bucket versioning + ILM; Qdrant и Neo4j перестраиваемые (бэкап не нужен); SQLite `links.db`/`gateway.db` в томе gateway, бэкап `links.db` шифрованный `age`, ≤ 30 дней | §5 |
| Ollama | рекомендация — **нативно на Windows** (GPU напрямую, без лимитов WSL2), контейнер в профиле `gpu` как альтернатива; `OLLAMA_KEEP_ALIVE=-1`, `OLLAMA_MAX_LOADED_MODELS=2`, `OLLAMA_NUM_PARALLEL=1`; прогрев в `make up`; матрица замера — `scripts/bench-llm.sh` | §6 |
| Наблюдаемость | `slog` JSON в stdout, ротация docker-логов; `/health` у каждого процесса; метрики MVP-1 — `mvctl session-report` → CSV; Prometheus/Grafana — EPIC-012 | §7 |
| Деплой/откат | образ `multiverse-core:<git-sha>`; `MV_IMAGE_TAG` в `.env`; откат = предыдущий тег + при необходимости `.down.sql`; бэкап перед каждым релизом | §8 |

---

## 1. Окружения

### 1.1. Три окружения и их различия

| | `dev` (машина владельца) | `ci` (GitHub Actions) | `prod` (та же машина, «боевой» запуск) |
|---|---|---|---|
| Хост | Windows 11, i9-13900, 128 ГБ RAM, RTX 4090 24 ГБ; Docker Desktop 29.x (WSL2), Compose v5.x, Go 1.25.3 локально, Git Bash, Python | `ubuntu-latest` (4 vCPU, 16 ГБ, Docker есть, GPU нет) | как `dev` |
| Как запускается платформа | `make up` (compose) **или** `go run ./cmd/multiverse --contexts=all --bus=memory` для быстрой отладки без инфраструктуры | `go test` (unit/e2e без Docker; integration — testcontainers) | `make up` с `COMPOSE_PROFILES=memory,bot` (+ `gpu`, если Ollama в контейнере) |
| LLM | Ollama нативно (рекомендация) или профиль `gpu` | нет; `providers/recorded` + `providers/fake` | Ollama, прогретые модели |
| Данные | тома Docker; допускается `make reset` (полная очистка) | эфемерные контейнеры testcontainers | тома Docker + бэкапы (§5.6) |
| Секреты | `.env` (локально, вне git) | GitHub Secrets: для MVP-1 **не требуются** (см. §4.4) | `.env`, права файла только владельцу |
| Порты | все публикуются только на `127.0.0.1` | не публикуются | как `dev` |
| Профили compose | `dev` (+ Redpanda Console), по желанию `memory`, `gpu`, `legacy` | — | `memory`, `bot`, (`gpu`) |
| Логи | `docker compose logs`, json-file с ротацией | артефакты job | json-file с ротацией; логи бота — ≤ 7 файлов × 10 МБ |

`staging` отсутствует намеренно: одна машина, один оператор; роль «предпрод» выполняет прогон e2e/интеграции в CI и `make ci` локально перед `make deploy`.

### 1.2. Топология процессов (ADR-001, overview §13)

```
telegram-bot ──HTTP──▶ gateway (:8088) ──┐
                                          ├── Redpanda (8 топиков) ◀── core (state,mechanics,swarm,llm,laws; :8081 health/admin)
                        memory (:8082) ◀──┘        │
                        │  └── Qdrant, Neo4j        └── MinIO ── Ollama (native :11434 или контейнер)
mvctl (CLI, не демон) ── читает шину/MinIO
```

Один бинарник `cmd/multiverse`, флаг `--contexts` (или `MV_CONTEXTS`) задаёт набор контекстов процесса. Порт `core` (`MV_CORE_ADDR`, `127.0.0.1:8081`) нужен для `/health` и `/v1/admin/*` (тик по требованию, `agents`), к которым gateway проксирует (ADR-010 п. 1) — в `overview.md` §13 этого порта нет, см. §11 замечание 7.

### 1.3. Compose-профили

| Профиль | Сервисы | Когда включён |
|---|---|---|
| (без профиля — всегда) | `redpanda`, `redpanda-init`, `minio`, `minio-init`, `gateway`, `core` | всегда: минимальный стек «соло без памяти и без LLM» (деградация FR-080/NFR-072) |
| `memory` | `qdrant`, `neo4j`, `memory` | память Should (EPIC-005); в `prod` включён |
| `gpu` | `ollama` (контейнер с NVIDIA) | только если Ollama в контейнере (§6.1); при нативном Ollama профиль не нужен |
| `bot` | `telegram-bot` | когда задан `MV_TELEGRAM_BOT_TOKEN`; в CI/e2e выключен |
| `dev` | `redpanda-console` | удобство разработчика |
| `legacy` | `narrative-orchestrator` (as-is, отдельный `build/legacy.Dockerfile`) | только на время миграции GM (`MV_GM_PATH=legacy`), удаляется в EPIC-003 I2 |

Набор профилей задаётся `COMPOSE_PROFILES` в `.env` (`COMPOSE_PROFILES=memory,bot`); `make up PROFILES=...` переопределяет. Замечание: профиль `bot` не был в списке архитектора (`gpu/memory/dev/legacy`) — добавлен, чтобы `make up` в CI и на чистой машине не падал без токена (§11, замечание 11).

### 1.4. Порты (все — `127.0.0.1`)

| Сервис | Внутри сети compose | На хосте |
|---|---|---|
| gateway | `gateway:8088` | `127.0.0.1:8088` |
| core (health/admin) | `core:8081` | `127.0.0.1:8081` |
| memory | `memory:8082` | `127.0.0.1:8082` |
| Redpanda Kafka | `redpanda:9092` (internal listener) | `127.0.0.1:19092` (external listener) |
| Redpanda Admin | `redpanda:9644` | `127.0.0.1:9644` |
| Redpanda Console (`dev`) | `redpanda-console:8080` | `127.0.0.1:8092` |
| MinIO API / Console | `minio:9000` / `9001` | `127.0.0.1:9000` / `9001` |
| Qdrant REST / gRPC | `qdrant:6333` / `6334` | `127.0.0.1:6333` / `6334` |
| Neo4j HTTP / Bolt | `neo4j:7474` / `7687` | `127.0.0.1:7474` / `7687` |
| Ollama | `ollama:11434` (профиль `gpu`) или `host.docker.internal:11434` (нативно) | `127.0.0.1:11434` |

Два listener'а Redpanda (`internal://redpanda:9092`, `external://localhost:19092`) нужны, чтобы один и тот же кластер обслуживал контейнеры и процессы, запущенные `go run` на хосте. Порт `8081` Schema Registry больше не публикуется (ADR-007: реестр схем — в репозитории).

---

## 2. Сборка и зависимости

### 2.1. Единый модуль и toolchain (ADR-001, F-2)

- `go.mod` в корне: `module multiverse-core.io`, `go 1.25`, `toolchain go1.25.14` (последний патч 1.25 на 2026-09-09). `go.work`, `go.work.sum` удаляются; замороженные сервисы сохраняют свои `go.mod`, но в сборку не входят (`FROZEN.md`, каталоги перечислены в `.golangci.yml: run.skip-dirs` и не собираются `./...`, потому что это отдельные модули).
- **Важно (см. §11, замечание 1):** на дату документа актуальный Go — **1.27.1**; политика поддержки Go — два последних minor (1.26, 1.27). Пин 1.25.14 выполняет указание архитектора, но стандартная библиотека 1.25 уже **не получает исправлений безопасности**; `govulncheck` в CI начнёт сообщать о непоправимых уязвимостях stdlib. Рекомендация: задача «переход на `go 1.26` + `toolchain go1.26.x`» ставится сразу после `make ci` зелёного в EPIC-001, а не «когда-нибудь».
- Локальный запуск на пути с кириллицей: `go build` без `-buildvcs=false` падает (`audit-facts` §1). Makefile экспортирует `GOFLAGS=-buildvcs=false` для локальных целей; версия бинарника прошивается через `-ldflags "-X main.version=$(GIT_SHA)"`, поэтому VCS-штамп не нужен. В CI флаг не используется.
- `make` на Windows: Git for Windows не содержит `make`; ставится `winget install ezwinports.make` (GNU make 4.4), Makefile объявляет `SHELL := bash` и `.ONESHELL`. Альтернатива без установки — выполнять цели в WSL2 (`wsl make ci`).

### 2.2. Makefile — целевой набор (F-6)

| Цель | Что делает (одна команда — одно действие) |
|---|---|
| `make build` | `go build -o bin/ ./cmd/...` (три бинарника: `multiverse`, `telegram-bot`, `mvctl`) |
| `make lint` | `golangci-lint run` (конфиг `.golangci.yml`: `govet`, `staticcheck`, `errcheck`, `depguard` границ `internal/*`, `forbidigo` для `os.Getenv`, `gofmt`/`goimports`) |
| `make test` | `go test -short -race -count=1 -cover ./...` (unit, без сети) |
| `make test-integration` | `go test -tags integration -count=1 -timeout 15m ./...` (testcontainers; нужен Docker) |
| `make test-e2e` | `go test -tags e2e -count=1 -timeout 10m ./...` (один процесс `--contexts=all --mode=replay --bus=memory`) |
| `make contracts` | `go run ./cmd/mvctl contracts check && go run ./cmd/mvctl blueprint validate blueprints/ && go run ./cmd/mvctl env check` |
| `make secrets-scan` | `gitleaks git --no-banner --redact .` (весь репозиторий) |
| `make vuln` | `govulncheck ./...` |
| `make ci` | `lint test contracts secrets-scan vuln test-e2e` — то же, что CI без Docker; `make ci-full` добавляет `test-integration` |
| `make image` | `docker build -t multiverse-core:$(GIT_SHA) -t multiverse-core:dev -f build/Dockerfile .` |
| `make up` | `docker compose --profile … up -d --wait` → `make health` → `make warm` (если LLM доступен) |
| `make down` | `docker compose down` (тома сохраняются); `make reset` — `down -v` с подтверждением |
| `make health` | обходит `/health` gateway/core/memory и `ollama ps`, печатает таблицу; код возврата ≠ 0, если что-то не `ok` |
| `make models` | `ollama pull` по списку `ops/models.txt` (или из блупринтов) |
| `make warm` | `POST /api/generate {"model":M,"prompt":"","keep_alive":-1}` для каждой модели |
| `make bench` | `scripts/bench-llm.sh` — матрица замера §6.4, результат в `ops/metrics/bench-<date>.csv` |
| `make backup` / `make restore FILE=` | §5.6 |
| `make deploy` | `image` + `up` с `MV_IMAGE_TAG=$(GIT_SHA)`, предварительно `backup` (§8) |
| `make logs SERVICE=` | `docker compose logs -f --tail=200 $(SERVICE)` |
| `make replay RECORDING=` | `go run ./cmd/multiverse --contexts=all --mode=replay --bus=memory --recording=$(RECORDING)` |
| `make clean` | `rm -rf bin/ coverage.out` (без `docker system prune` — он удалял чужие образы) |

Правила: без `docker-compose` (v1) — только `docker compose`; без `latest`; `SERVICES` as-is удаляется; отдельные цели `build-service` для 15 сервисов исчезают вместе с workspace.

### 2.3. Dockerfile (F-6)

Один образ на три бинарника; `command` в compose выбирает точку входа.

```dockerfile
# build/Dockerfile
# syntax=docker/dockerfile:1.7
FROM golang:1.25.14-bookworm AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/ ./cmd/...

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/multiverse /out/telegram-bot /out/mvctl /
COPY --from=builder /src/blueprints /blueprints
COPY --from=builder /src/rules      /rules
COPY --from=builder /src/laws       /laws
COPY --from=builder /src/config     /config
USER nonroot:nonroot
ENTRYPOINT ["/multiverse"]
```

- `distroless/static` подходит, потому что все зависимости CGO-free (`modernc.org/sqlite`, `minio-go`, `kafka-go`, `qdrant/go-client`); ONNX/CGO-ветка as-is удаляется вместе с `fake_deps`.
- В образе нет shell и `curl` — healthcheck compose вызывает подкоманду бинарника: `/multiverse health --url http://127.0.0.1:8088/health` (возвращает 0/1). Подкоманда — часть каркаса `cmd/multiverse` (F-2).
- Кэш модулей и сборки — `--mount=type=cache` (BuildKit включён в Docker Desktop по умолчанию); пересборка при изменении кода ≈ 20–40 с.
- `.dockerignore`: `Docs/`, `services/` (замороженные), `.git`, `bin/`, `backups/`, `*.exe`, `*.log`, `.env*` кроме `.env.example`, `testdata/recordings` (не нужны в образе).
- Корневой `Dockerfile` и три копии в `services/*/Dockerfile` удаляются (F-1); остаётся `build/Dockerfile` и `build/legacy.Dockerfile` (as-is `build/Dockerfile` переименовывается для профиля `legacy`, пока он жив).

### 2.4. Версии инструментов и образов (проверено 2026-09-09)

| Компонент | Пин | Источник / примечание |
|---|---|---|
| Go toolchain | `go1.25.14` (актуальный релиз Go — 1.27.1; см. §2.1) | go.dev/dl, Docker Hub `golang:1.25.14-bookworm` |
| Builder image | `golang:1.25.14-bookworm` | Docker Hub |
| Runtime image | `gcr.io/distroless/static-debian12:nonroot` | по digest в `build/Dockerfile` (обновляется отдельной задачей) |
| Redpanda | `docker.redpanda.com/redpandadata/redpanda:v26.1.17` | Docker Hub; v26.2.2 вышел 2026-08-22 (две недели, 2 патча) — берём зрелую линейку 26.1 (17 патчей), переход на 26.2 — отдельной задачей. Из as-is v24.2.5 том **не обновляется** прямым скачком (Redpanda обновляется по одной feature-версии) — том пересоздаётся (§5.5) |
| Redpanda Console | `docker.redpanda.com/redpandadata/console:v3.11.0` | профиль `dev` |
| MinIO | `minio/minio:RELEASE.2025-09-07T16-13-09Z` | **последний образ на Docker Hub**; upstream выпустил `RELEASE.2025-10-15T17-29-55Z` только исходниками — публикация образов прекращена (см. §11, замечание 2) |
| MinIO Client (`mc`) | `minio/mc:RELEASE.2025-08-13T08-35-41Z` | init-контейнер |
| Qdrant | `qdrant/qdrant:v1.19.1` | Docker Hub; клиент `github.com/qdrant/go-client v1.19.2` (minor клиента = minor сервера) |
| Neo4j | `neo4j:5.26.30-community` | Docker Hub (LTS-линейка 5.26, патч 30); драйвер `neo4j-go-driver/v5 v5.28.4` |
| Ollama | контейнер `ollama/ollama:0.33.3`; нативно — 0.33.3 (0.34.0 в rc) | Docker Hub, GitHub; требование ≥ 0.12 выполнено; structured output/`think` есть |
| testcontainers-go | `v0.44.0` + модули `modules/redpanda`, `modules/minio`, `modules/qdrant`, `modules/neo4j` (все `v0.44.0`) | proxy.golang.org |
| golangci-lint | `v2.13.2`; action `golangci/golangci-lint-action@v9.3.0` | proxy.golang.org, GitHub |
| gitleaks | `v8.30.1`; action `gitleaks/gitleaks-action@v3.0.0`; образ `zricethezav/gitleaks:v8.30.1` | GitHub, Docker Hub |
| govulncheck | `golang.org/x/vuln v1.8.0`; action `golang/govulncheck-action@v1.1.0` | proxy.golang.org, GitHub |
| GitHub Actions базовые | `actions/checkout@v7.0.1`, `actions/setup-go@v7.0.0`, `actions/cache@v6.1.0`, `actions/upload-artifact@v7.0.1`, `docker/setup-buildx-action@v4.3.0`, `docker/build-push-action@v7.3.0` | GitHub Releases (в workflow — пин по major + Dependabot для `github-actions`) |
| pre-commit | `v4.6.2` | GitHub |
| Go-библиотеки (ориентир для F-2/F-4/F-5) | `segmentio/kafka-go v0.4.51` (as-is 0.4.49), `minio-go/v7 v7.3.0` (as-is 7.0.95), `modernc.org/sqlite v1.58.0`, `golang-migrate/migrate/v4 v4.20.0` (драйвер `sqlite` поверх modernc), `santhosh-tekuri/jsonschema/v6 v6.0.3` (JSON Schema 2020-12; as-is `xeipuuv/gojsonschema` — только draft-07, см. §11 замечание 5), `oklog/ulid/v2 v2.1.2`, `go-telegram/bot v1.25.0` | proxy.golang.org |

Dependabot (`.github/dependabot.yml`): экосистемы `gomod` (еженедельно, группировать minor/patch), `github-actions` (еженедельно), `docker` (ежемесячно, только уведомление — пины compose обновляются вручную после проверки).

---

## 3. CI/CD (F-7)

### 3.1. Workflow `.github/workflows/go.yml`

Триггеры: `push` и `pull_request` в `main`, `develop`, `epic/**`; `paths-ignore: ['Docs/**', '**/*.md']` (кроме `blueprints/**/*.md` — они данные); `concurrency: {group: ci-${{ github.ref }}, cancel-in-progress: true}`; `permissions: contents: read` (job `security` — `security-events: write` для SARIF). Все job'ы — `runs-on: ubuntu-latest`, `timeout-minutes` явные.

| Job | Шаги | Время (оценка) | Блокирует мерж |
|---|---|---|---|
| `unit` | checkout → setup-go (`go-version-file: go.mod`, cache on) → `go mod verify` → `go build ./...` → `go vet ./...` → golangci-lint-action (`version: v2.13.2`) → `go test -short -race -coverprofile=coverage.out ./...` → `scripts/coverage-gate.sh 60 internal/state internal/mechanics internal/swarm internal/llm internal/replay` (пакеты, которых ещё нет, пропускаются с предупреждением) → upload `coverage.out` | 3–4 мин | да |
| `integration` | setup-go → `go test -tags integration -count=1 -timeout 15m ./...` (testcontainers-go поднимает Redpanda/MinIO/Qdrant/Neo4j; Docker на раннере есть; `TESTCONTAINERS_RYUK_DISABLED=false`) | 4–6 мин | да |
| `e2e` | setup-go → `go test -tags e2e -count=1 -timeout 10m ./...` (один процесс, `--bus=memory`, записи `testdata/recordings/*.jsonl`; без Docker) → upload `ops/metrics/sessions/*.json` как артефакт | 1–2 мин | да |
| `contracts` | setup-go → `go run ./cmd/mvctl contracts check` → `go run ./cmd/mvctl blueprint validate blueprints/` → `go run ./cmd/mvctl env check` (`.env.example` ↔ реестр `shared/env`) → `go test ./shared/contracts/... -run TestSchemasValid` → проверка `api/*.openapi.yaml` загружается (`kin-openapi` в тесте) | 1 мин | да |
| `security` | `gitleaks/gitleaks-action@v3` (полная история PR; `GITLEAKS_LICENSE` не нужен для личного аккаунта) → `golang/govulncheck-action@v1.1.0` (`go-version-file: go.mod`) | 1–2 мин | да (govulncheck — `warn` до перехода на поддерживаемый Go, затем блокирует; см. §2.1) |
| `compose-lint` | `docker compose -f docker-compose.yml config -q` (с `.env.example` как `--env-file`) → `scripts/compose-pins.sh` (каждый `image:` имеет явный тег ≠ `latest`, ни один `ports:` не без `127.0.0.1:`) → `hadolint/hadolint-action` для `build/Dockerfile` | < 1 мин | да |
| `image` (только `push` в `main`) | buildx → `docker build` с кэшем `type=gha` → **без push** (образы собираются на машине владельца `make image`; публикация в GHCR — по отдельному решению) | 3 мин | нет |

Суммарно ≤ 10 мин (job'ы параллельны, критический путь — `integration`). Матрица ОС не нужна: единственная целевая платформа Linux-контейнер; локальная сборка на Windows проверяется разработчиком (`make build`), в CI — не дублируется. При появлении второй платформы — добавить `windows-latest` только для `unit`.

Ветки (ADR-010, `epics.md` §5): CI одинаков на `main`, `develop`, `epic/*`; правило защиты `main`/`develop` — required checks `unit, integration, e2e, contracts, security, compose-lint`, линейная история, без force-push. Порядок слияния эпиков (002 → 003 → 004 → 005) — `epics.md` §5; интеграционная ветка собирается тем же workflow — отдельного «интеграционного» CI не нужно.

### 3.2. Что остаётся и что удаляется

- `validate-blueprints.yml` — удалить (заменён job `contracts`; ссылался на `configs/gm_*.yaml` и `go-version: 1.24`).
- `qwen-*.yml` (5 шт.) — не трогать (AI-триаж issues/PR; не мешают, но используют `vars`/`secrets` владельца — их наличие подтверждает владелец).
- Codecov — не подключаем (артефакт `coverage.out` и порог в скрипте достаточны).

### 3.3. GPU-замер вне CI

`nightly-gpu` (ADR-010 п. 1) не выполняется на GitHub-hosted раннерах. Варианты: (а) вручную `make bench` на машине владельца по необходимости (рекомендация: замер нужен несколько раз за MVP-1, не еженощно); (б) self-hosted GitHub runner на машине владельца (Windows service, метка `gpu`), workflow `bench.yml` по `workflow_dispatch`/`schedule` — даёт историю в артефактах, но раннер имеет доступ к машине и токену репозитория. Решение владельца — открытый вопрос 2.

### 3.4. Pre-commit

`.pre-commit-config.yaml` (Python `pre-commit` уже есть на машине): хуки `gitleaks` (`rev: v8.30.1`, `gitleaks protect --staged --redact`), `golangci-lint fmt` (или `gofmt`/`goimports` через `golangci/golangci-lint` hook `rev: v2.13.2`), `check-added-large-files --maxkb=1024` (ловит `.exe`/`.pdf`), `end-of-file-fixer`, `check-yaml`. Установка — шаг чек-листа §10.

---

## 4. Конфигурация и секреты

### 4.1. Принципы

1. Конфигурация процессов — **только переменные окружения** (`.env` для compose, реальные env для `go run`), префикс `MV_`. Один пакет `shared/env`: `env.String("MV_KAFKA_BROKERS", "127.0.0.1:19092", "адреса брокеров")` регистрирует переменную в реестре (имя, дефолт, описание, секрет ли, обязательна ли). `os.Getenv`/`os.LookupEnv` вне `shared/env` запрещены `forbidigo` в `.golangci.yml`.
2. Переменные инфраструктурных образов (Redpanda, MinIO, Neo4j, Ollama) префикса не имеют — их имена диктуют образы; они живут в том же `.env`, помечены секцией «infrastructure».
3. Конфигурация домена — **файлы в Git**: `blueprints/*.md`, `rules/dark-forest.yaml`, `laws/*.yaml`, `config/absolute-limits.yaml`, `schemas/**`. `configs/gm_*.yaml` и `shared/config` (профили из MinIO) удаляются (overview §16).
4. Фича-флаги MVP-1 — тоже env: `MV_GM_PATH=agent|legacy` (миграция GM, S5), `MV_LLM_CLOUD_ENABLED`, `MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS`, `MV_LAWS_BREACH_PHASE=false`, `MV_LLM_STORE_PROMPTS`. Переключение флага = перезапуск процесса (горячая перезагрузка — E-F). ADR-005/ADR-009 называют их без префикса (`LLM_CLOUD_ENABLED`) — унифицировать на `MV_` (§11, замечание 4).

### 4.2. `.env.example` — целевой состав (F-1/F-6)

```dotenv
# ===== compose =====
COMPOSE_PROJECT_NAME=multiverse
COMPOSE_PROFILES=memory,bot          # набор профилей: memory,gpu,bot,dev,legacy
MV_IMAGE_TAG=dev                     # тег образа multiverse-core (make deploy подставляет git sha)

# ===== infrastructure (имена диктуют образы) =====
MINIO_ROOT_USER=                     # обязательна; не minioadmin
MINIO_ROOT_PASSWORD=                 # обязательна, ≥ 16 символов; секрет
NEO4J_PASSWORD=                      # обязательна при профиле memory; секрет (compose собирает NEO4J_AUTH=neo4j/${NEO4J_PASSWORD})
OLLAMA_KEEP_ALIVE=-1                 # только профиль gpu; нативно — переменные окружения Windows
OLLAMA_MAX_LOADED_MODELS=2
OLLAMA_NUM_PARALLEL=1
OLLAMA_FLASH_ATTENTION=1
OLLAMA_KV_CACHE_TYPE=f16             # q8_0 — вариант E матрицы замера (§6.4)

# ===== platform, общие для всех процессов =====
MV_ENV=dev                           # dev|ci|prod
MV_LOG_LEVEL=info                    # debug|info|warn|error
MV_LOG_FORMAT=json                   # json|text
MV_MODE=live                         # live|replay
MV_BUS=redpanda                      # redpanda|memory (memory — только e2e/отладка одного процесса)
MV_KAFKA_BROKERS=redpanda:9092       # go run на хосте: 127.0.0.1:19092
MV_MINIO_ENDPOINT=minio:9000         # без схемы; на хосте 127.0.0.1:9000
MV_MINIO_ACCESS_KEY=                 # секрет; для dev = MINIO_ROOT_USER, для prod — отдельный пользователь (mc admin user add)
MV_MINIO_SECRET_KEY=                 # секрет
MV_MINIO_USE_SSL=false
MV_WORLD_ID=dark-forest-world        # мир по умолчанию для mvctl и bootstrap

# ===== gateway =====
MV_GATEWAY_ADDR=:8088
MV_GATEWAY_DATA_DIR=/data            # links.db, gateway.db (том gateway_data)
MV_GATEWAY_CLIENT_IDS=telegram-bot,ci-harness,mvctl   # allow-list X-Client-Id (ADR-009 п. 9)
MV_GATEWAY_ACTOR_KIND_CLIENTS=ci-harness,mvctl        # кому разрешён X-Actor-Kind ci|sim
MV_CORE_URL=http://core:8081         # прокси /v1/admin/*

# ===== core =====
MV_CORE_ADDR=:8081
MV_MEMORY_URL=http://memory:8082     # пусто = память выключена (деградация FR-035)
MV_SNAPSHOT_EVERY_FACTS=200          # снапшот State каждые N фактов
MV_GM_PATH=agent                     # agent|legacy — фича-флаг миграции GM (S5)
MV_LAWS_BREACH_PHASE=false

# ===== llm =====
MV_LLM_PROVIDER=ollama               # ollama|openai|deepseek|anthropic|recorded|fake
MV_OLLAMA_URL=http://host.docker.internal:11434   # профиль gpu: http://ollama:11434; go run: http://127.0.0.1:11434
MV_LLM_STORE_PROMPTS=false           # полные промпты в prompts-{world}
MV_LLM_CLOUD_ENABLED=false
MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS=false
MV_LLM_CLOUD_BUDGET_USD_PER_DAY=0
MV_OPENAI_API_KEY=                   # секрет; только при MV_LLM_PROVIDER=openai
MV_DEEPSEEK_API_KEY=                 # секрет
MV_ANTHROPIC_API_KEY=                # секрет

# ===== memory (профиль memory) =====
MV_MEMORY_ADDR=:8082
MV_QDRANT_ADDR=qdrant:6334           # gRPC
MV_NEO4J_URI=neo4j://neo4j:7687
MV_NEO4J_USER=neo4j
MV_NEO4J_PASSWORD=                   # секрет; = NEO4J_PASSWORD
MV_EMBED_MODEL=nomic-embed-text      # или bge-m3 — по замеру

# ===== telegram-bot (профиль bot) =====
MV_TELEGRAM_BOT_TOKEN=               # секрет; выдаёт @BotFather
MV_TELEGRAM_GATEWAY_URL=http://gateway:8088
MV_TELEGRAM_POLL_TIMEOUT_S=25
```

Каждая переменная в реестре `shared/env` имеет описание; `mvctl env check` (job `contracts`, `make contracts`) проверяет: (а) каждая зарегистрированная `MV_*` есть в `.env.example`; (б) каждая `MV_*` из `.env.example` зарегистрирована; (в) переменные, помеченные `secret`, в `.env.example` пусты; (г) обязательные переменные без дефолта пусты в примере и документированы. Инфраструктурные переменные (без префикса) сверяются со списком в `shared/env/infra.go`. Так закрывается NFR-074 без парсинга `os.Getenv` по исходникам — но линтер `forbidigo` дополнительно гарантирует, что мимо реестра ничего не читается.

`.mcp.env.example` (для MCP-серверов IDE, не для платформы): `GITHUB_TOKEN=`, `DB_MCP_TOKEN=` — пустые, с комментарием, где получить.

### 4.3. Локальный запуск без compose (`go run`)

Файл `.env.local.example` не вводим; вместо него — раздел комментариев в `.env.example` и цель `make run-local CONTEXTS=all` (`set -a; source .env; set +a; MV_KAFKA_BROKERS=127.0.0.1:19092 MV_MINIO_ENDPOINT=127.0.0.1:9000 MV_OLLAMA_URL=http://127.0.0.1:11434 go run ./cmd/multiverse --contexts=$(CONTEXTS)`).

### 4.4. Секреты: где живут

| Секрет | Где хранится | Кто читает | В CI |
|---|---|---|---|
| `MINIO_ROOT_USER/PASSWORD`, `MV_MINIO_*` | `.env` на машине владельца (права файла — только владелец) | compose, `minio-init`, процессы | не нужны (testcontainers генерирует свои) |
| `NEO4J_PASSWORD` / `MV_NEO4J_PASSWORD` | `.env` | compose, `memory` | не нужны |
| `MV_TELEGRAM_BOT_TOKEN` | `.env` | только `telegram-bot` (в compose передаётся `environment:` именно этому сервису, не `env_file` всем) | нет; ротация — §9.6 |
| `MV_OPENAI_API_KEY` и др. облачные | `.env`; по умолчанию пусты | `core` | нет |
| `GITHUB_TOKEN`, `DB_MCP_TOKEN` (MCP IDE) | `.mcp.env` вне git | IDE | нет |
| GitHub Secrets репозитория | для MVP-1 — **ни одного**; `GITHUB_TOKEN` автоматический хватает; `qwen-*` workflow используют свои (`QWEN_*`/`vars`) — вне области этого документа | — | — |

Правило compose: `env_file: .env` — только для сервисов платформы (`gateway`, `core`, `memory`), и даже им лучше передавать явный список `environment:` с `${VAR}`; секрет бота — только боту; инфраструктурные пароли — только их контейнерам. Значения по умолчанию для паролей в compose **отсутствуют** (`${MINIO_ROOT_PASSWORD:?set in .env}` — compose падает с понятной ошибкой).

### 4.5. Гигиена репозитория (F-1) — план по шагам

Проверено по `git ls-files` 2026-09-09. Порядок важен: шаг 0 чинит `git status`.

0. **`.claude/worktrees/*` (13 файлов-указателей `gitdir: D:/my project/...`) — удалить из индекса и с диска**: из-за них `git status --ignored` падает с `fatal: not a git repository: D:/my project/.../worktrees/laughing-kare`. `git rm -r --cached .claude/worktrees && rm -rf .claude/worktrees`.
1. Секреты: `git rm --cached .mcp.env` (`.env` уже не отслеживается); добавить `.mcp.env.example`; убедиться, что `.claude/settings.local.json` (локальные настройки, могут содержать пути/токены) тоже `git rm --cached`.
2. Бинарники и логи: `git rm --cached examples.exe semantic-memory.exe services/narrative-orchestrator/cmd.exe mcp_kafka.log mcp_audit.log shared/eventbus/docs/event-model.pdf` (PDF 3,9 МБ — в `Docs/archive/` не переносить, история Git хранит; если документ нужен — ссылка на коммит).
3. Мёртвый код и дубли: `git rm -r test_minio.go fake_deps Dockerfile services/entity-actor/Dockerfile services/evolution-watcher/Dockerfile services/rule-engine/Dockerfile` (as-is `build/Dockerfile` → `build/legacy.Dockerfile` до конца профиля `legacy`).
4. IDE/AI-конфиги и заметки: `git rm -r --cached .idea .vscode .kilo .kilocode .kilocodemodes .roo .roomodes .qwen/settings.json.orig memory plans reports` — решение по `.claude/agents`, `.claude/skills`, `.qwen/agents`, `.roo/rules-*` (инструкции для AI-агентов — возможно, ценны владельцу) — оставить, но обновить под новую раскладку (F-9, tech-writer). `events/*.json` (примеры мировых событий) → `Docs/archive/events/`.
5. Пустые каталоги на диске `-p/`, `Multiverse/` — удалить (`rm -rf -- -p Multiverse`; обратите внимание на `--` перед `-p`).
6. `.gitignore` целевой:
   ```gitignore
   # secrets & local config
   .env
   .env.*
   !.env.example
   .mcp.env
   .claude/settings.local.json
   # build & artifacts
   /bin/
   /dist/
   *.exe
   *.test
   *.out
   coverage.*
   /backups/
   *.db
   *.db-wal
   *.db-shm
   # logs
   *.log
   # IDE / AI tooling (локальное)
   .idea/
   .vscode/
   .kilo*/
   .kilocodemodes
   .roo/
   .roomodes
   .qwen/settings.json.orig
   .claude/worktrees/
   gha-creds-*.json
   # OS
   .DS_Store
   Thumbs.db
   ```
7. `.gitattributes`: добавить `*.exe binary`, `*.pdf binary`, `*.jsonl -diff merge=binary` (записи LLM), `*.sh text eol=lf`, `Makefile text eol=lf`, `*.yml text eol=lf` — иначе `autocrlf` на Windows портит скрипты для контейнеров.
8. `.gitleaks.toml`: allowlist путей `\.env\.example$`, `\.mcp\.env\.example$`, `testdata/recordings/`, `Docs/`; правила по умолчанию; в pre-commit — `gitleaks protect --staged`, в CI — полная история PR.
9. Проверка: `gitleaks git --redact .` по HEAD → 0 находок в рабочем дереве (в истории — останутся до filter-repo); `git ls-files | grep -E '\.(exe|log|pdf)$'` → пусто; `git status` работает.
10. **Очистка истории (`git filter-repo --path .mcp.env --invert-paths` + удаление `.exe`) — отдельная задача, выполняется только по явной команде владельца** (ADR-009 п. 8, OQ-A-11); до неё токены считаются скомпрометированными и отозваны владельцем. После filter-repo: все клоны и 40+ веток (`claude/*`, `gm-*`) переклонируются/удаляются — ветки заморозить до этого (ADR-001).

---

## 5. Данные: инфраструктура, миграции, бэкапы

### 5.1. Redpanda (ADR-007 п. 4–6)

Запуск: `redpanda start --mode dev-container --smp 1 --memory 1G --overprovisioned --node-id 0 --kafka-addr internal://0.0.0.0:9092,external://0.0.0.0:19092 --advertise-kafka-addr internal://redpanda:9092,external://localhost:19092 --rpc-addr redpanda:33145 --advertise-rpc-addr redpanda:33145`. Healthcheck контейнера: `rpk cluster health | grep -q 'Healthy:.*true'` (`interval: 5s`, `start_period: 20s`); `redpanda-init` объявляет `depends_on: redpanda: condition: service_healthy` вместо as-is `sleep 10`, а сервисы платформы — `depends_on: redpanda-init: condition: service_completed_successfully`.

`redpanda-init` (образ Redpanda, скрипт `build/redpanda-init.sh`, идемпотентен):

| Топик | `retention.ms` | Замечания |
|---|---|---|
| `player_events` | `2592000000` (30 дн.) | |
| `game_events` | `2592000000` | |
| `world_events` | `2592000000` | |
| `system_events` | `2592000000` | |
| `narrative_output` | `2592000000` | |
| `llm_records` | `7776000000` (90 дн.) | тяжёлые payload; `max.message.bytes=4194304` (4 МБ — полный промпт/ответ) |
| `analytics_events` | `15552000000` (180 дн.) | не читается в replay |
| `dead_letters` | `2592000000` | |

Для всех: `-p 1 -r 1 -c cleanup.policy=delete -c segment.ms=86400000` — сегмент закрывается раз в сутки, иначе при малом трафике (≈ 2 МБ/сессия) retention никогда не удалит активный сегмент, и «30 дней» будут фикцией. Скрипт: `rpk topic create "$t" -p 1 -r 1 -c ... || rpk topic alter-config "$t" --set retention.ms=... --set segment.ms=...` — повторный запуск обновляет конфиг. `scope_management` и фантомные топики не создаются; Schema Registry не используется.

Оценка диска (OQ-A-19): доменные топики ≈ 2 МБ/сессия × 8 сессий/мес ≈ 16 МБ/мес; `llm_records` ≈ 1 МБ/сессия; фон ≈ 24 тика/сут × 4 КБ ≈ 0,1 МБ/сут. Итого < 200 МБ за retention-окно — ограничений по диску нет; retention 30/90/180 дней утверждать можно.

Consumer-группы: `{process}.{context}` (например `core.state`, `memory.indexer`); reader `MinBytes=1, MaxWait=100ms`; при смене раскладки контекстов между процессами группы переименовываются → офсеты теряются → State стартует со снапшота с курсором (C-14), поэтому это безопасно.

### 5.2. MinIO (ADR-004 п. 1)

- Бакеты: `entities-{world}`, `snapshots-{world}`, `prompts-{world}` (versioning **on**), `ops-artifacts` (versioning off).
- **Versioning включается при создании бакета кодом** `shared/objstore.EnsureBucket(ctx, name, Versioned)` — потому что бакеты `*-{world}` создаются `mvctl world init`, а не заранее; `minio-init` не может включить versioning на ещё не существующем бакете. `minio-init` (`minio/mc`) делает: `mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"`, `mc mb --ignore-existing local/ops-artifacts`, создаёт сервисного пользователя `mc admin user add local "$MV_MINIO_ACCESS_KEY" "$MV_MINIO_SECRET_KEY"` + политику `readwrite` (в `prod` платформа не ходит под root-ключом), и для существующих бакетов `entities-*`/`snapshots-*` — `mc version enable` + ILM (идемпотентно; повторяет то же при каждом `make up`).
- ILM (`mc ilm rule add`): `entities-*`, `snapshots-*` — `--noncurrent-expire-days 30` (старые версии живут 30 дней; текущая — бессрочно), `prompts-*` — `--expire-days 90`, `ops-artifacts` — без правил. Правила задаются и кодом `objstore` при создании бакета (тот же набор), чтобы не зависеть от порядка запуска.
- Данные as-is (`entities-{world}` с плоским payload, `schemas`, `gnue-configs`, `rules`, `models`) **не мигрируются** (overview §19): MVP-1 стартует с `mvctl world init`. Перед `make reset` выполняется `make archive-legacy` — `mc mirror local/ ./backups/legacy-minio-<date>/` (однократно, вне git).

### 5.3. Qdrant и Neo4j (профиль `memory`)

- Qdrant `v1.19.1`, том `qdrant_storage`; коллекции `events-{world}`, `entities-{world}` создаёт `internal/memory` при старте; API-ключ не задаём (порт только на loopback; при выходе за одну машину — `QDRANT__SERVICE__API_KEY`). Healthcheck: образ без `curl`/`wget` — использовать `bash -c 'exec 3<>/dev/tcp/127.0.0.1/6333'` (в образе есть bash) или `/qdrant --help`-заглушку; проверить при реализации.
- Neo4j `5.26.30-community`: `NEO4J_AUTH=neo4j/${NEO4J_PASSWORD:?}`, `NEO4J_server_memory_heap_max__size=1G`, `NEO4J_server_memory_pagecache_size=512M`, `NEO4J_PLUGINS='["apoc"]'` только если код as-is semantic-memory использует APOC (проверить при переносе — иначе убрать), том `neo4j_data`. Healthcheck `wget -qO- http://127.0.0.1:7474` (в образе есть). Constraints/indexes создаются кодом при старте (как as-is) — это единственная «миграция» Neo4j, идемпотентная.
- Оба хранилища — **производные** (перестраиваются `mvctl memory rebuild` из журнала за retention-окно + снапшота). Бэкап не делаем; при потере — rebuild (§9.5). Ограничение: события старше retention (30 дн.) в индекс не вернутся — приемлемо для Should.

### 5.4. SQLite в gateway (ADR-004 п. 2, ADR-009)

- Файлы `/data/links.db`, `/data/gateway.db` в томе `gateway_data`; процесс — `nonroot` (uid 65532); при создании файлы `0600`; том не монтируется другим сервисам. `PRAGMA journal_mode=WAL; synchronous=NORMAL; busy_timeout=5000; foreign_keys=ON`.
- Миграции: `internal/gateway/migrations/NNN_<name>.up.sql` + `.down.sql`, встроены `embed.FS`, применяются при старте gateway через `golang-migrate` (`source/iofs` + `database/sqlite` поверх modernc) с таблицей `schema_migrations`; отдельные цепочки для `links.db` (`migrations/links/`) и `gateway.db` (`migrations/gateway/`). Правила: нумерация трёхзначная сквозная внутри цепочки, одна миграция на PR, применённую миграцию не редактировать; владелец — только EPIC-004 (`ownership.md`), поэтому конфликтов нумерации между эпиками нет. Интеграционный тест `up → down → up` на пустой БД обязателен для каждой миграции; `make migrate-down STEPS=1` для отката (см. §8).
- `/forget` = `DELETE` + `VACUUM` по расписанию (раз в сутки, `MV_GATEWAY_VACUUM_INTERVAL`), чтобы удалённые строки физически исчезли из файла (иначе NFR-042 нарушается на уровне байтов файла).
- Бэкап — §5.6; в образе нет `sqlite3`, поэтому онлайн-копия делается подкомандой `multiverse db backup --out /data/backup/<name>.db` (`VACUUM INTO`, безопасно при работающем писателе) — задача EPIC-004 (каркас подкоманды — EPIC-001).

### 5.5. Миграция от as-is (что нужно, что нет)

| Хранилище | Действие при переходе на EPIC-001 | Причина |
|---|---|---|
| Redpanda том (v24.2.5) | **пересоздать** (`docker volume rm multiverse_redpanda_data`) | обновление 24.2 → 26.1 одним шагом не поддерживается; журнал as-is не нужен (мир создаётся заново) |
| MinIO том | оставить (бакеты as-is — архив) или `make archive-legacy` + очистить | данные не мигрируются (§19); versioning включается на новых бакетах |
| Chroma, TimescaleDB тома | удалить | ADR-004 |
| Neo4j 5.18 → 5.26.30 | пересоздать том (граф перестраивается) | проще, чем миграция store-формата; данные производные |
| Qdrant `latest` → 1.19.1 | пересоздать | клиентов не было, данных нет |

Одноразовый скрипт `scripts/migrate-from-as-is.sh`: `docker compose down`, архив MinIO по желанию, удаление перечисленных томов, `make up`, `mvctl world init`.

### 5.6. Бэкапы и восстановление

| Что | Как | Периодичность | Хранение | Проверка restore |
|---|---|---|---|---|
| Том `minio_data` (истина: сущности + снапшоты) | `make backup` → остановить `core` (чтобы не было записи в момент копии) → `docker run --rm -v multiverse_minio_data:/src:ro -v "$BACKUP_DIR":/dst alpine:3.22 tar czf /dst/minio-<date>.tgz -C /src .` → запустить `core`; альтернатива без остановки — `mc mirror --overwrite local/ ./backups/minio-<date>/` | перед каждым `make deploy`; еженедельно по расписанию Windows (Task Scheduler → `make backup`) | `%USERPROFILE%\multiverse-backups\` (вне репозитория и вне Docker-томов), последние 4 еженедельных + все предрелизные за 30 дней | ежемесячно: `make restore FILE=… TARGET=scratch` в отдельный compose-проект (`COMPOSE_PROJECT_NAME=mv-restore`) → `mvctl session-report --audit` на восстановленном → `state_hash` совпадает со снапшотом |
| Том `redpanda_data` (журнал после снапшота) | тот же `tar` при остановленном `redpanda` (файлы сегментов консистентны только при остановке) | вместе с MinIO | там же | восстановление вместе с MinIO; после старта `core` — `analytics.replay.completed identical=true` |
| `links.db` (ПДн, ADR-009 п. 2) | `docker compose exec gateway /multiverse db backup --out /data/backup/links-<date>.db` → `docker cp` на хост → `age -r <MV_BACKUP_AGE_RECIPIENT> -o links-<date>.db.age` → удалить открытую копию (`shred`/`rm`, в томе — тоже) | вместе с остальным | там же; **≤ 30 дней**, затем удаление (скрипт чистит старше 30 дн.) | ежемесячно: `age -d` → `sqlite3 .tables`/`PRAGMA integrity_check` (или `multiverse db check`) |
| `gateway.db` (сессии, outbox) | тем же `db backup` без шифрования | вместе | там же | как выше |
| Qdrant, Neo4j | не бэкапятся — `mvctl memory rebuild` | — | — | rebuild после восстановления — часть runbook §9.4 |
| Модели Ollama | не бэкапятся — `make models` | — | — | — |
| `ops/metrics/*.csv`, блупринты, правила, законы, записи LLM | в Git | каждый коммит | GitHub | — |

Ключ `age` (пара) генерируется владельцем один раз (`age-keygen`), приватный ключ хранится вне машины проекта (менеджер паролей); в `.env` — только `MV_BACKUP_AGE_RECIPIENT` (публичный ключ, не секрет). Полное восстановление на чистой машине: §9.4.

RPO/RTO (NFR-010/011): RPO для подтверждённых событий = 0 при живых томах (снапшот + журнал); при потере диска RPO = возраст последнего бэкапа (≤ 7 дней при еженедельном; MVP-1 — приемлемо, владелец играет сам); RTO ≤ 2 мин при рестарте, ≈ 15 мин при восстановлении из бэкапа (замер — B6).

---

## 6. Ollama

### 6.1. Размещение: нативно на Windows или контейнер

| Критерий | Нативно (Windows-приложение Ollama 0.33.3) | Контейнер `ollama/ollama:0.33.3` (профиль `gpu`, Docker Desktop/WSL2) |
|---|---|---|
| GPU | CUDA напрямую, полный VRAM 24 ГБ | через WSL2 GPU-паравиртуализацию: работает, накладные расходы малы (единицы %), но требует драйвер NVIDIA с поддержкой WSL и `nvidia` runtime в Docker Desktop |
| RAM для частичной выгрузки MoE (вариант B) | вся 128 ГБ | ограничена `.wslconfig` (`memory=`; по умолчанию 50 % = 64 ГБ — достаточно, но нужно знать) |
| Диск моделей (20–40 ГБ) | `%USERPROFILE%\.ollama\models` (или `OLLAMA_MODELS`) на NTFS, доступны IDE и `ollama` CLI | внутри VHDX WSL2 (растёт, не сжимается автоматически) в томе `ollama_data` |
| Пин версии (NFR-071) | приложение автообновляется; фиксация — запуск `ollama serve` как службы (NSSM/Task Scheduler) вместо tray-приложения и установка конкретной версии инсталлятора | тег образа — точный пин; обновление осознанное |
| Воспроизводимость окружения | зависит от состояния Windows | `docker compose` описывает всё |
| Доступ из контейнеров платформы | `http://host.docker.internal:11434`; Ollama должен слушать не только loopback: `OLLAMA_HOST=0.0.0.0:11434` + правило Windows Firewall «только с адресов Docker (172.16.0.0/12) / WSL» — проверить командой из §9.1 (Docker Desktop иногда доставляет `host.docker.internal` и на loopback — если работает без `0.0.0.0`, оставить loopback) | `http://ollama:11434` внутри сети compose |
| Доступ с хоста (`go run`, `make bench`, `ollama ps`) | `127.0.0.1:11434`, CLI «из коробки» | опубликованный порт `127.0.0.1:11434`; CLI — `docker compose exec ollama ollama ps` |
| Старт/стоп | независим от Docker; `make up` не управляет | `make up/down` управляет; при `down` модели выгружаются, `make warm` обязателен |
| CI | не участвует | не участвует (GPU нет) |

**Рекомендация devops:** нативно — на машине владельца это официально поддерживаемый Ollama путь для Windows + NVIDIA, без двойной виртуализации памяти и без роста VHDX на десятки ГБ; версия фиксируется установкой конкретного инсталлятора и запуском `ollama serve` службой с переменными окружения из §6.2. Контейнерный профиль `gpu` остаётся в compose как задокументированная альтернатива (например, для переезда на Linux-хост). Матрица замера (§6.4) выполняется в выбранном варианте; если выбран нативный — один прогон конфигурации C в контейнере для сравнения (строка `placement` в CSV). Решение — открытый вопрос 1.

### 6.2. Переменные окружения Ollama (оба варианта)

`OLLAMA_KEEP_ALIVE=-1` (модели не выгружаются; NFR-004), `OLLAMA_MAX_LOADED_MODELS=2`, `OLLAMA_NUM_PARALLEL=1` (одна очередь — латентность хода важнее пропускной способности; фон уступает по дизайну роя), `OLLAMA_FLASH_ATTENTION=1`, `OLLAMA_KV_CACHE_TYPE=f16` (базово; `q8_0` — вариант E матрицы: KV-кэш ≈ ×0,5, что может «вместить» пару 8b + 30b-a3b), `OLLAMA_HOST` — по §6.1, `OLLAMA_MODELS` — путь на быстром SSD. Нативно: `setx` для пользователя → перезапуск службы; контейнер — `environment:` в compose (значения из `.env`).

Дополнительно шлюз передаёт в каждом запросе `keep_alive: -1`, `options.num_ctx` (8–16k по фазе), `think: false` для Phase 1/тиков, `format` = JSON-схема (ADR-005).

### 6.3. Модели и прогрев

- `ops/models.txt` — список тегов для `make models` (кандидаты матрицы: `qwen3:8b`, `qwen3:14b`, `qwen3:30b-a3b`, `qwen3:32b`, эмбеддинги `nomic-embed-text`, `bge-m3`); после замера список сокращается до базовой конфигурации (OQ-A-18) и синхронизируется с блупринтами (`mvctl blueprint validate` проверяет, что модель из блупринта есть в `ops/models.txt` — предложение для EPIC-003/005).
- `make warm`: для каждой модели базовой конфигурации `curl -s $MV_OLLAMA_URL/api/generate -d '{"model":"<m>","prompt":"","keep_alive":-1}'`; затем `ollama ps` должен показывать модели `100% GPU` (или ожидаемую долю при варианте B). `make up` вызывает `warm` после `health`, если `MV_LLM_PROVIDER=ollama` и Ollama отвечает; иначе печатает предупреждение и продолжает (NFR-072 — стек живёт без LLM).
- Критерий NFR-070: `make up` → все `/health = ok` и модели резидентны ≤ 3 мин — измеряется `stack_up_time_s` в `make bench` (B5).

### 6.4. Матрица замера как скрипт (F-8)

`scripts/bench-llm.sh` — bash + `curl` + `jq` (`winget install jqlang.jq`), запускается на машине владельца; конфигурации задаются `ops/metrics/bench-matrix.yaml` (архитектор правит без изменения скрипта). Скелет:

```bash
#!/usr/bin/env bash
# scripts/bench-llm.sh — матрица замера LLM (overview §18.1, nfr «Что измерить первым», metrics §7 B1–B5)
set -euo pipefail
URL="${MV_OLLAMA_URL:-http://127.0.0.1:11434}"
OUT="ops/metrics/bench-$(date +%Y%m%d-%H%M).csv"
N="${BENCH_N:-20}"                         # запросов на ячейку (≥ 3 прогона × ситуации золотого набора)
PROMPTS="${BENCH_PROMPTS:-testdata/bench/prompts.jsonl}"   # {"phase":"phase2","system":..., "user":..., "schema":{...}}
echo "config,placement,model,phase,kv_cache,n,p50_ms,p95_ms,valid_json_ratio,cjk_ratio,latin_ratio,vram_used_mb,ollama_ps" > "$OUT"

configs=(
  "A:qwen3:8b,qwen3:14b"        # тики на 8b, нарратив на 14b
  "B:qwen3:8b,qwen3:30b-a3b"    # частичная выгрузка MoE в RAM
  "C:qwen3:30b-a3b"             # одна MoE на всё
  "D:qwen3:32b"                 # dense, если C проваливает NFR-090
)
for entry in "${configs[@]}"; do
  cfg="${entry%%:*}"; models="${entry#*:}"
  # 1) выгрузить всё, загрузить набор конфигурации, прогреть (B2 — cold start)
  for m in $(curl -s "$URL/api/ps" | jq -r '.models[].name'); do curl -s "$URL/api/generate" -d "{\"model\":\"$m\",\"keep_alive\":0}" >/dev/null; done
  IFS=',' read -ra ms <<< "$models"
  for m in "${ms[@]}"; do
    t0=$(date +%s%3N); curl -s "$URL/api/generate" -d "{\"model\":\"$m\",\"prompt\":\"\",\"keep_alive\":-1}" >/dev/null; t1=$(date +%s%3N)
    echo "cold_start,$cfg,$m,$((t1-t0))ms" >&2
  done
  vram=$(nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits | head -1)
  ps=$(curl -s "$URL/api/ps" | jq -c '[.models[]|{name,size_vram,size}]')
  # 2) N запросов на фазу через native /api/chat со structured output и think=false
  for phase in tick phase2; do
    m="${ms[-1]}"; [[ "$phase" == "tick" ]] && m="${ms[0]}"
    lat=(); ok=0; cjk=0; lat_ratio_sum=0
    while IFS= read -r p; do
      body=$(jq -cn --arg m "$m" --argjson p "$p" '{model:$m, stream:false, think:false, keep_alive:-1, format:$p.schema, options:{num_ctx:8192,temperature:0.7}, messages:[{role:"system",content:$p.system},{role:"user",content:$p.user}]}')
      t0=$(date +%s%3N); resp=$(curl -s "$URL/api/chat" -d "$body"); t1=$(date +%s%3N); lat+=($((t1-t0)))
      content=$(jq -r '.message.content' <<<"$resp")
      jq -e . >/dev/null 2>&1 <<<"$content" && ok=$((ok+1))
      grep -qP '[\x{4E00}-\x{9FFF}]' <<<"$content" && cjk=$((cjk+1))
    done < <(jq -c "select(.phase==\"$phase\")" "$PROMPTS" | head -n "$N")
    sorted=($(printf '%s\n' "${lat[@]}" | sort -n)); cnt=${#sorted[@]}
    p50=${sorted[$((cnt*50/100))]}; p95=${sorted[$((cnt*95/100 < cnt ? cnt*95/100 : cnt-1))]}
    echo "$cfg,${BENCH_PLACEMENT:-native},$m,$phase,${OLLAMA_KV_CACHE_TYPE:-f16},$cnt,$p50,$p95,$(bc -l <<<"$ok/$cnt"),$(bc -l <<<"$cjk/$cnt"),n/a,$vram,\"$ps\"" >> "$OUT"
  done
done
echo "written $OUT"
```

Что фиксирует: B1 (p50/p95 по фазам), B2 (холодный старт — в stderr, в CSV добавить колонку при реализации), B3 (`valid_json_ratio`), B4 (`cjk_ratio`; доля латиницы — добавить по регулярному выражению), размещение (`vram_used_mb`, `ollama ps` с `size_vram` — видно, какая часть модели в RAM). Вариант E = конфигурация B с `OLLAMA_KV_CACHE_TYPE=q8_0` (перезапуск Ollama между конфигурациями — вручную или через `nssm restart`). Результат и решение (OQ-A-18) архитектор записывает в `ops/metrics/baseline.md`; CSV коммитится. B5–B8 (старт стека, RTO, стоимость фона) — через `make up` с таймером и `mvctl session-report` после появления EPIC-002/003/005.

---

## 7. Наблюдаемость

### 7.1. Логи

- `shared/logging`: `slog` JSON в stdout; обязательные поля через middleware шины и HTTP: `ts, level, service (gateway|core|memory|telegram-bot), context, msg, correlation_id, event_id, event_type, agent.level?, handled (для error), version`. Уровень — `MV_LOG_LEVEL`; `debug` не включается в `prod` (объём и риск текста в логах).
- **ПДн и текст**: логгер не принимает произвольные `map`/структуры событий — только явные поля; тела `POST /v1/links/*`, `/v1/characters` и обновления Telegram не логируются; `player.said`/`narrative.output` в логах — только `event_id`, длина текста. Тест NFR-041 (`e2e`, `privacy-scan`) прогоняет фикстуры с внешним ID и `username` и `grep`-ает логи всех процессов и бота.
- Docker: `logging: {driver: json-file, options: {max-size: "50m", max-file: "5"}}` для платформы; `telegram-bot` — `max-size: "10m", max-file: "7"` и `MV_LOG_LEVEL=info` (ADR-009 п. 4: retention ≤ 7 дней ≈ 7 файлов при суточной ротации по объёму; точнее — отдельным cron `docker compose logs --since` не нужен). Просмотр: `make logs SERVICE=core`, фильтр по корреляции `docker compose logs core | jq -c 'select(.correlation_id=="…")'`.

### 7.2. Health

`GET /health` у `gateway`, `core`, `memory` (и `telegram-bot` на `127.0.0.1:8089` — только `ok/degraded` по доступности gateway): 
```json
{"status":"ok|degraded|down","version":"<git sha>","mode":"live","contexts":["state","mechanics",…],
 "deps":{"bus":"ok","objstore":"ok","llm":"ok|unavailable","memory":"ok|unavailable","sqlite":"ok"},
 "agents_by_level":{"global":1,"domain":1,"task":2},"checked_at":"…"}
```
Зависимости опрашиваются раз в 10 с (NFR-016: отказ виден ≤ 30 с); HTTP-код 200 для `ok|degraded`, 503 для `down`. Compose `healthcheck` каждого сервиса платформы — `/multiverse health --url http://127.0.0.1:<port>/health` (`interval: 15s`, `start_period: 30s`); `make health` печатает сводку по всем и `ollama ps`.

### 7.3. Метрики MVP-1 (metrics.md §6)

Без Prometheus: `mvctl session-report` читает `analytics_events`, `llm_records`, `game_events` за окно сессии → Markdown в консоль + строка в `ops/metrics/sessions.csv`; `--background --since 24h` → `ops/metrics/background.csv`; `--audit` → `analytics.consistency.violated`; `--json` → `ops/metrics/sessions/<session_id>.json`. `ops/metrics/incidents.csv` — ручной журнал оператора. Всё в Git (ПДн нет). Ресурсный след (NFR-073) — `docker stats --no-stream` и `nvidia-smi` в `make bench` (B5).

### 7.4. Что уходит в EPIC-012 (E-G)

`/metrics` (Prometheus text format) у трёх процессов: латентности фаз, глубина очередей `interactive/background`, `agents_active_by_level`, `llm_calls/tokens/cost` по меткам, `guardian_rejections`, `dlq_size`; `docker compose --profile observability` с Prometheus + Grafana + провизионированные дашборды из `ops/grafana/`; алерты (NFR-035): p95, `llm_fallback_rate > 0.05`, бюджет фона, `dead_letters` растёт, `/health != ok` > 1 мин → уведомление в Telegram владельцу через того же бота (служебный чат). До EPIC-012 «алерт» = `make health` с ненулевым кодом в Task Scheduler раз в 15 мин + сообщение в консоль/уведомление Windows.

---

## 8. Деплой и откат

«Прод» — та же машина; деплой = сборка образа и перезапуск compose.

1. `make deploy` = `make ci` (локально, без Docker-тестов) → `make backup` (§5.6) → `make image` (`multiverse-core:<git-sha>`, плюс тег `current`) → запись `MV_IMAGE_TAG=<git-sha>` в `.env` (предыдущее значение — в `.env.previous`) → `docker compose up -d --wait --remove-orphans` (профили из `COMPOSE_PROFILES`) → `make health` → `make warm` → smoke: `mvctl world status` и один ход `ci-harness` в тестовом scope (`actor_kind=ci`, в S7 не считается).
2. Миграции SQLite применяются gateway при старте (§5.4); события — только совместимые изменения схем в пределах `schema_version` (ADR-007 п. 3), несовместимые — `+1` с поддержкой `n-1` потребителями, поэтому «миграции событий» нет; MinIO — версии объектов.
3. **Откат** (`make rollback`): `MV_IMAGE_TAG` ← значение из `.env.previous` → `docker compose up -d --wait` → если релиз содержал миграцию SQLite с несовместимым `down` — `docker compose run --rm gateway /multiverse db migrate down --steps N` **до** старта старого образа (данные, записанные новой версией между релизом и откатом, могут потеряться — это фиксируется в заметке релиза) → `make health`. Образы хранятся последние 3 (`make image-prune`). Данные MinIO/Redpanda обратно совместимы по построению; при необходимости — `make restore FILE=<предрелизный бэкап>` (потеря событий после бэкапа — осознанное решение).
4. Релизная заметка (`releases/vX.Y.Z.md`, release-manager) содержит: sha образа, список миграций и их обратимость, изменения `.env.example` (новые переменные — оператор дописывает `.env` **до** `make deploy`; `mvctl env check --env .env` проверяет реальный файл на отсутствующие обязательные), изменения retention/бакетов.
5. Версионирование: `main` = то, что задеплоено; теги `vX.Y.Z` на релизах; образ помечается и sha, и тегом.

---

## 9. Runbook — заготовки (переносятся в `docs/ops/runbook.md` tech-writer'ом на A7)

### 9.1. Запуск с нуля (после клонирования)
1. `cp .env.example .env`; заполнить `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD` (≥ 16 симв.), `MV_MINIO_ACCESS_KEY/SECRET_KEY`, `NEO4J_PASSWORD`/`MV_NEO4J_PASSWORD`, при боте — `MV_TELEGRAM_BOT_TOKEN`; выставить `COMPOSE_PROFILES`.
2. Ollama нативно: установить 0.33.3, `setx OLLAMA_KEEP_ALIVE -1` и остальные §6.2, перезапустить службу; `ollama ls`. Проверка доступности из контейнера: `docker run --rm curlimages/curl:8.10.1 -s http://host.docker.internal:11434/api/version` — должен вернуть версию; если нет — `OLLAMA_HOST=0.0.0.0:11434` + правило файрвола.
3. `make models` → `make up` (init-контейнеры создают топики/бакеты/пользователя MinIO) → `make health` → `make warm`.
4. `go run ./cmd/mvctl world init --blueprints blueprints/ --world $MV_WORLD_ID` → `mvctl world status`.
5. Бот: `/start` в Telegram → уведомление FR-009 → создание персонажа.

### 9.2. Остановка / перезапуск
- `make down` — останавливает контейнеры, тома сохраняются; после `make up` `core` восстанавливается по C-14 (снапшот + догон журнала), в логе `analytics.replay.completed identical=true`.
- Перезапуск одного процесса: `docker compose restart core` (bot держит long-poll к gateway — gateway перезапускать отдельно, бот переподключится).
- `make reset` — **удаляет тома** (спрашивает подтверждение, требует свежего `make backup`).

### 9.3. Прогрев и проверка LLM
`make warm` → `ollama ps` (`100% GPU` для базовой конфигурации) → `mvctl llm ping` (один запрос phase2 на записанном промпте, печатает латентность и валидность JSON). Если `/health.llm=unavailable`: `curl $MV_OLLAMA_URL/api/version`, `nvidia-smi`, логи Ollama (`%LOCALAPPDATA%\Ollama\server.log` нативно или `docker compose logs ollama`).

### 9.4. Восстановление
- **После падения процесса/машины (тома целы):** `make up` → `make health` → в логах `core` `replay.completed` → `mvctl session-report --audit` → `state_divergence_count = 0`.
- **Из бэкапа на чистой машине:** установить Docker Desktop, Ollama, `git clone`, `.env` из менеджера паролей владельца → `make restore FILE=minio-<date>.tgz` (распаковка в том при остановленных сервисах) → то же для `redpanda-<date>.tgz` → `age -d links-<date>.db.age > links.db` и `docker cp` в том gateway (`/data/links.db`, права 0600, владелец 65532) → `make up` → `make health` → `mvctl memory rebuild` (профиль `memory`) → `mvctl session-report --audit`.
- **Порча индекса памяти:** `docker compose stop memory` → `docker volume rm` томов qdrant/neo4j (или `mvctl memory reset`) → `docker compose up -d memory` → `mvctl memory rebuild --since 30d`.
- **Переполнение диска:** `docker system df`; `make image-prune`; проверить `segment.ms` и retention (`rpk topic describe -c`); ILM MinIO (`mc ilm ls local/entities-<world>`).

### 9.5. Обновление версий инфраструктуры
Одна версия за раз: изменить пин в `docker-compose.yml` → `make backup` → `docker compose pull <svc>` → `docker compose up -d <svc>` → `make health` → интеграционные тесты testcontainers обновляют пин в `shared/testkit/containers.go` тем же PR (один источник версий: константы в `build/versions.env`, читаемые и compose (`env_file`), и тестами — задача F-6). Redpanda — только на соседнюю feature-версию (26.1 → 26.2), Neo4j — внутри 5.26.x свободно, MinIO — см. §11 замечание 2.

### 9.6. Ротация токена бота
1. `@BotFather` → `/revoke` (старый токен перестаёт работать немедленно; long-poll бота упадёт — ожидаемо).
2. Обновить `MV_TELEGRAM_BOT_TOKEN` в `.env` → `docker compose up -d telegram-bot` (только бот; платформа не перезапускается).
3. `docker compose logs --tail=50 telegram-bot` → `getMe ok`; `/help` в чате.
4. Если старый токен попадал в git/логи: `gitleaks git` по истории — фиксировать в `ops/metrics/incidents.csv` (`kind=other`), очистка истории — по решению владельца.

### 9.7. Ежедневный/еженедельный чек оператора
- Ежедневно (автоматически, Task Scheduler): `make health` (ненулевой код → уведомление), `mvctl session-report --background --since 24h` (строка в `background.csv`).
- Еженедельно: `make backup`, `docker system df`, `mvctl session-report --weekly`, ревью `incidents.csv`, `dependabot` PR.

---

## 10. Чек-лист первой задачи разработчика — «инициализация проекта» (EPIC-001, F-1/F-2/F-6/F-7)

Пошагово; каждый шаг заканчивается проверяемой командой. Порядок задач: F-1 → F-2 → F-5/F-4 (параллельно, по `epics.md`) → F-6 → F-7 → F-3 → F-8 → F-9.

1. **Ветка**: от `main` создать `epic/EPIC-001-foundation` (политика `git.branches=true`; коммиты — по подтверждению владельца, `git.commits=ask`).
2. **Гигиена (F-1, §4.5)**: шаги 0–9 § 4.5; `git status` работает; `gitleaks git --redact .` по рабочему дереву — 0; `.gitignore`, `.gitattributes`, `.gitleaks.toml`, `.mcp.env.example`, `.pre-commit-config.yaml` созданы; `pre-commit install`; `pre-commit run --all-files` зелёный.
3. **Единый модуль (F-2)**: удалить `go.work*`; корневой `go.mod` → `go 1.25`, `toolchain go1.25.14`; перенести `shared/{eventbus,jsonpath,entity,agent}` в корневой модуль (удалив их `go.mod`), остальные `shared/*` — по §16 overview (удалить/заморозить); `services/*` — вне сборки (`FROZEN.md` — F-3); `go mod tidy && go build ./... && go vet ./...` зелёные.
4. **Каркас `cmd/multiverse`**: флаги `--contexts`, `--mode`, `--bus`, `--recording`, `--id-source`; подкоманды `health --url`, `db backup|migrate` (заглушки, реализуют EPIC-004); интерфейс `Context{Start, Health}`; регистрация контекстов-заглушек `gateway, state, mechanics, swarm, llm, laws, memory` с `/health` (`deps` пустые); `cmd/telegram-bot`, `cmd/mvctl` (`contracts check`, `blueprint validate`, `env check`, `world init` — каркас). Проверка: `go run ./cmd/multiverse --contexts=all --bus=memory` → `curl 127.0.0.1:8088/health` → `{"status":"ok",…}`.
5. **`shared/env` (F-5)**: реестр, `env check`; `.env.example` по §4.2; `forbidigo` в `.golangci.yml` запрещает `os.Getenv` вне `shared/env`; `make contracts` зелёный.
6. **`shared/logging` (F-5)**: slog JSON, обязательные поля, middleware; тест «внешний ID не попадает в лог».
7. **`.golangci.yml`** (v2): `govet`, `staticcheck`, `errcheck`, `gofmt/goimports` (formatters), `depguard` (правила: `internal/<a>` не импортирует `internal/<b>`, кроме `swarm → mechanics|laws|llm`), `forbidigo`; `make lint` зелёный.
8. **Compose (F-6, §1.3–§1.4, §5.1–§5.3)**: `docker-compose.yml` с пинами §2.4, профилями, `redpanda-init` (`build/redpanda-init.sh`, 8 топиков, retention/segment), `minio-init` (`build/minio-init.sh`), healthchecks, `depends_on: condition`, порты `127.0.0.1`, `${MINIO_ROOT_PASSWORD:?}`-обязательность, ротация логов; `build/Dockerfile` (§2.3), `.dockerignore`; `docker compose --env-file .env.example config -q` проходит (пароли в примере пустые — для `config` подставить `MINIO_ROOT_PASSWORD=x` через `--env-file` временного файла в CI-скрипте); `make up` на машине владельца → `make health` = ok для `gateway/core` (контексты-заглушки).
9. **Makefile (F-6, §2.2)**: цели из таблицы; `make ci` локально зелёный.
10. **`shared/testkit` + testcontainers (F-5/F-7)**: `containers.go` с пинами = compose (общий `build/versions.env`); один интеграционный тест «Redpanda: топик создан, Publish → Subscribe порядок сохранён» и «MinIO: versioning включён после `EnsureBucket`»; `make test-integration` зелёный локально (Docker Desktop).
11. **CI (F-7, §3.1)**: `.github/workflows/go.yml`, `scripts/coverage-gate.sh`, `scripts/compose-pins.sh`, `.github/dependabot.yml`; удалить `validate-blueprints.yml`; PR в `main` → все шесть job зелёные ≤ 10 мин.
12. **Заморозка (F-3)**: `FROZEN.md` в 8 каталогах (причина, эпик возврата, дата), `Docs/archive/` для трёх версий `architecture*.md`, `LIVING_WORLDS_*`, `EVENTS-MIGRATION.md`, `PULL_REQUEST.md`, `events/`; профиль `legacy` с `build/legacy.Dockerfile` (as-is `build/Dockerfile`) — если архитектор подтвердит его жизнеспособность (§11, замечание 3).
13. **Матрица замера (F-8, §6.4)**: `scripts/bench-llm.sh`, `testdata/bench/prompts.jsonl` (10 ситуаций «Тёмного леса» × 2 фазы, схемы из `schemas/agent/`), `ops/models.txt`; `make models && make bench` → `ops/metrics/bench-<date>.csv`; архитектор → `ops/metrics/baseline.md`.
14. **Документы (F-9)**: `README.md` (запуск за 5 команд), `CLAUDE.md`/`AGENTS.md` — по факту раскладки; `docs/ops/runbook.md` из §9.
15. **Критерий готовности EPIC-001** (epics.md): `make ci` зелёный; `make up` поднимает инфраструктуру и пустые контексты с `/health`; `mvctl contracts check` без фантомов; секретов в HEAD нет; `baseline.md` с конфигурацией моделей.

---

## 11. Замечания к архитектуре (для system-architect / architect#N)

1. **Go 1.25 вне окна поддержки.** На 2026-09-09 актуален Go 1.27.1; поддерживаются 1.26 и 1.27. Пин `toolchain go1.25.14` (указание ADR-001) означает stdlib без security-фиксов и шум `govulncheck` с первого дня. Предложение: в EPIC-001 сразу `go 1.26` + `toolchain go1.26.x` (изменение для нового кода нулевое; `go 1.25`-директива в `go.mod` всё равно означает «минимальная версия языка»), либо явно принять риск и поставить задачу перехода первой после G2. Нужно решение архитектора (не владельца).
2. **MinIO: публикация Docker-образов прекращена.** Последний образ на Docker Hub — `RELEASE.2025-09-07T16-13-09Z` (год назад); upstream выпускает только исходники (последний `RELEASE.2025-10-15`). Пин на этот образ работает, но обновлений безопасности не будет. Варианты: (а) собирать образ из исходников в `build/minio.Dockerfile` (Go, ≈ 5 мин, пин по git-тегу) — рекомендация devops; (б) заменить на другой S3-совместимый сервер с versioning (Garage/SeaweedFS/RustFS) — `shared/objstore` использует только Put/Get/List/Versioning, замена дешёвая, но нужна проверка versioning-семантики; (в) оставить как есть до EPIC-012. ADR-004 стоит дополнить.
3. **Профиль `legacy` конфликтует с удалением Chroma.** as-is narrative-orchestrator ходит в semantic-memory (`/v1/context-with-events`), которая as-is требует Chroma; ADR-004 удаляет Chroma, ADR-001 переписывает semantic-memory на Qdrant с другим HTTP-контрактом. Чтобы профиль `legacy` работал, нужно либо (а) держать as-is `semantic-memory` + `chromadb` в том же профиле (тогда Chroma не «удаляется», а переезжает в `legacy`), либо (б) запускать orchestrator с отключённой памятью (в его коде есть деградация?) — требует проверки владельцем EPIC-003. Иначе флаг `GM_PATH=legacy` и критерий S5 (`legacy_gm_path_share`) не проверяемы.
4. **Имена переменных в ADR-005/ADR-009 без префикса** (`LLM_PROVIDER`, `LLM_CLOUD_ENABLED`, `LLM_STORE_PROMPTS`, `LLM_CLOUD_BUDGET_USD_PER_DAY`). Документ вводит `MV_`-префикс для всего платформенного; ADR лучше привести к `MV_*`, чтобы контракты, `.env.example` и `shared/env` совпадали буквально.
5. **JSON Schema 2020-12 и библиотека.** as-is `xeipuuv/gojsonschema` (в `go.mod`) поддерживает только draft-07 и не развивается; ADR-007 требует 2020-12 — кандидат `github.com/santhosh-tekuri/jsonschema/v6` (v6.0.3). Зафиксировать в C-01/F-4.
6. **Bucket versioning в момент создания.** Init-контейнер `mc version enable` (указание архитектора) применим только к уже существующим бакетам; бакеты `*-{world}` создаёт `mvctl world init`. Нужно, чтобы `shared/objstore.EnsureBucket` включал versioning и ILM сам (init-контейнер — страховка для существующих). Отразить в описании `shared/objstore` (F-5).
7. **Порт и HTTP у `core`.** ADR-010 (e2e `POST /v1/admin/agents/{id}/tick`), ADR-005 (`/v1/admin/llm/usage`) и NFR-030 (`/health` у каждого процесса) требуют HTTP-сервера в `core`; в `overview.md` §13 у `core` порта нет. Предлагаю `MV_CORE_ADDR` (`127.0.0.1:8081`), gateway проксирует `/v1/admin/*` (ADR-009 п. 9 — доступ по allow-list клиентов). Добавить в C4 и `api/core.openapi.yaml` (или в раздел admin `gateway.openapi.yaml`).
8. **Redpanda: том as-is не обновляется прямым скачком 24.2 → 26.1** — пересоздание тома. Совместимо с §19 (данные не мигрируют), но стоит явно записать в план миграции.
9. **`segment.ms` для retention.** Без `segment.ms=1d` retention 30 дней на топиках с трафиком 2 МБ/сессия не срабатывает годами (активный сегмент не удаляется). Добавить в ADR-007 п. 4 как часть конфигурации топиков.
10. **Профиль `bot`.** Добавлен сверх списка `gpu/memory/dev/legacy`: без него `make up` в CI/на чистой машине падает без токена, а `env_file` для всех сервисов протекает токеном в `core/memory`. Подтвердить.
11. **`.claude/worktrees/*` в индексе ломают `git status`** (указатели на `D:/my project/...`). Это не только гигиена, но и блокер для любых git-операций команд — первый шаг F-1.
12. **Логи бота «≤ 7 дней»** реализуемы только ротацией по объёму (`json-file` не умеет по времени); с `max-size 10m × 7` при `info` без пейлоадов это ≈ месяцы, не дни. Если 7 дней — жёсткое требование ADR-009 п. 4, нужен внешний cron (`docker compose logs --since` не удаляет) либо запись логов бота в файл с `lumberjack`-ротацией по дням внутри процесса — уточнить, что важнее: простота или буквальные 7 дней.
13. **Один источник версий для compose и testcontainers** (`build/versions.env`) — иначе интеграционные тесты и стек разъедутся (риск ADR-010 «две реализации шины» аналогичен для версий). Предложение к F-5/F-6.
14. **Golden/records в git**: `*.jsonl` пометить `-diff merge=binary` в `.gitattributes` (ADR-010 п. 2 — «ревью диффа нарративов» тогда делается `mvctl record diff`, а не git-диффом); либо оставить текстовый дифф — выбрать.

---

## 12. Риски и допущения

| Риск / допущение | Влияние | Реакция |
|---|---|---|
| Go 1.25 без поддержки (замечание 1) | уязвимости stdlib без фикса; `govulncheck` красный | job `security` — `govulncheck` в режиме предупреждения до перехода на 1.26; задача перехода — первая после G2 |
| Образ MinIO заморожен (замечание 2) | нет security-обновлений объектного хранилища; порт только на loopback снижает риск | сборка из исходников или замена — решение архитектора; `shared/objstore` держит интерфейс минимальным |
| Ollama нативно: автообновление ломает пин версии и structured output (известные регрессии у Qwen3-«преамбулы») | NFR-071, NFR-022 | `ollama serve` службой с фиксированной версией; `mvctl llm ping` в `make health`; золотой набор ловит регрессию |
| `host.docker.internal` из контейнеров не достигает нативного Ollama на loopback | LLM недоступен из `core` | проверка §9.1 п. 2; `OLLAMA_HOST=0.0.0.0` + файрвол на подсеть Docker |
| testcontainers на GitHub-hosted раннере: pull четырёх образов ≈ 1–2 мин, Neo4j стартует ≈ 30 с | `integration` дольше 6 мин | кэш образов недоступен; Neo4j-тесты — только в `integration`, помечены `t.Parallel()`; при превышении — Neo4j-тесты в отдельный job по `paths` `internal/memory/**` |
| `make` и bash-скрипты на Windows (CRLF, отсутствие `jq`/`age`/`make`) | локальный `make ci` не работает у владельца | `.gitattributes eol=lf`; `winget` список в README (`ezwinports.make`, `jqlang.jq`, `FiloSottile.age`); альтернатива — `wsl make ci` |
| Бэкапы вручную/Task Scheduler — человек забудет | RPO до недели | `make health` ежедневно печатает возраст последнего бэкапа и предупреждает > 7 дней |
| Единственный писатель SQLite и `VACUUM INTO` во время нагрузки | кратковременные `SQLITE_BUSY` | `busy_timeout=5000`; бэкап — в паузе между сессиями (оператор играет сам) |
| GitHub Actions minutes на приватном репозитории — лимит 2 000 мин/мес на Free | ≈ 10 мин × 200 прогонов | `paths-ignore`, `concurrency cancel-in-progress`; при исчерпании — `integration` только на `develop`/`main` |
| Допущение: репозиторий остаётся на GitHub, личный аккаунт (gitleaks-action без лицензии) | — | при переходе на организацию — `GITLEAKS_LICENSE` или `gitleaks` через Docker-образ в job |
| Допущение: Docker Desktop с WSL2, `host-gateway` доступен; NVIDIA-драйвер с WSL-поддержкой установлен (нужен только для профиля `gpu`) | — | профиль `gpu` не запускается без подтверждения `nvidia-smi` внутри `docker run --gpus all` |
| Допущение: `qwen-*` workflow используют секреты/vars владельца и не мешают `go.yml` | — | не трогаем; при конфликте `concurrency` группы разные |
