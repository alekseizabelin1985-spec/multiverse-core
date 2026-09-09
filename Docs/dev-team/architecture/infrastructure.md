# Инфраструктура и доставка

Версия 0.3 · 2026-09-09 · devops-engineer · статус: **утверждено на G2, приведено к сведению 2** (реализация — EPIC-001, задачи F-1, F-3, F-5, F-6, F-7, F-8; порядок — `plan/epics.md` v0.2 §6).
Основание: `architecture/overview.md` (§3, §5, §9, **§13**, §15–§19, **§18.1**, §21), ADR-001, ADR-004, **ADR-005 (дополнение 2)**, ADR-006, ADR-007, ADR-009, ADR-010 (все — с дополнениями «2026-09-09»), ADR-019, ADR-021, `architecture/consolidation.md` (§1 U-1…U-7, §6 T-3/T-8/T-13/T-15/T-16/T-17, §7 D-1…D-14, §9, **§10 U-8…U-12, §13**), `architecture/threat-model.md` (§4.5, §4.6, §4.9, SEC-04/05/09/13/14/15/22/24/25/30/32/33), `contracts.md` (§0, C-01, C-14, **C-15 v1.1**, §16 п. 5), `plan/epics.md` v0.2 §6, `plan/teams.md` §3, `plan/ownership.md`, `audit-facts.md` (§1, §3, §7, §8), `requirements/nfr.md` (NFR-002, NFR-076, NFR-090, «Что измерить первым»), `project/metrics.md` (§6, §7), `journal.md` (записи 2026-09-09: G2, F-0, решения по безопасности, **«Решения пользователя (DevOps)»**).
Правило документа: **значения секретов нигде не приводятся** — только имена переменных. Код и конфиги проекта этим документом не меняются; всё ниже — задания для разработчика и devops в EPIC-001. Команды в документе перечисляются, но **не выполнялись**.

**Изменения v0.3** (сведение 2, `consolidation.md` §10 U-8…U-12 и §13 строка devops; ADR-005 дополнение 2 п. 1–9; `overview.md` §13/§18.1). Точечная правка: разделы, не связанные с LLM-рантаймом, CODEOWNERS и MinIO-форком, оставлены без изменений.
1. **LLM-рантайм по умолчанию — нативный `llama-server` (llama.cpp) на `127.0.0.1:1234`**, вне compose; Ollama — опциональный второй рантайм (U-8, ADR-005 доп. 2 п. 1/7). `scripts/llm-server.ps1` + `.sh`, `make llm-up / llm-down / llm-health`; `make up` процесс не поднимает, только проверяет health — §6 (переписан), §2.2, §1.1–§1.4.
2. `.env.example`: `MV_LLM_PROVIDER=openai_compat`, `MV_LLM_URL`, `MV_LLM_API_KEY` (пусто локально), `MV_LLM_CLOUD_ENABLED=false` (гейт по host, не по имени провайдера); `MV_OLLAMA_URL` и блок `OLLAMA_*` — только при `MV_LLM_PROVIDER=ollama`, из обязательных убраны — §4.2, §4.4.
3. Матрица замера F-8: варианты **E** (`Qwen3.8-27B-UD-Q3_K_XL` на llama-server) и **E+** (router 27B + 8B) добавлены к C и A; `bench-matrix.json` получает поле `provider`; скрипт ходит в `/v1/chat/completions` (`response_format json_schema`, `chat_template_kwargs.enable_thinking=false`), метрики из `usage`/`timings` + `nvidia-smi` — §6.4.
4. CI: `CODEOWNERS` = `@alekseizabelin1985-spec` (U-9), репозиторий приватный личный → gitleaks-action без `GITLEAKS_LICENSE`, лимит Actions 2 000 мин/мес — §3.2, §12.
5. MinIO: форк `minio/minio` в аккаунт `alekseizabelin1985-spec` делает владелец в F-6, `build/minio.Dockerfile` клонирует **форк** по тегу `RELEASE.2025-10-15T17-29-55Z`; тарбол в `backups/` — вторая страховка (U-10) — §2.5, §12.
6. IDE/AI-каталоги в индексе — в F-1 **не трогаются**, решение в F-9 (U-11) — §4.5 п. 8.
7. Runbook: карточка процесса `llama-server` (старт/стоп/health/смена модели/обновление версии) — §9.3, §9.8; риск «обновление llama.cpp ломает грамматику/преамбулу» → пин билда в `build/versions.env` + золотой набор — §12.
8. CLI, публикующие в шину (`mvctl laws bump`, `world init`, `record`), запускаются на хосте — им нужен адрес брокера `MV_KAFKA_BROKERS=127.0.0.1:19092` (external listener). Явно зафиксировано — §4.2 (секция `mvctl`), §9.1, §9.8.
9. §13: вопросы 1–5 закрыты решениями U-8…U-12; §3.3 GPU-замер — вручную (U-12).

**Изменения v0.2** (по `consolidation.md` §7 D-1…D-14, §6 T-3/T-8/T-13/T-15/T-16/T-17, §9; решения пользователя U-1/U-2/U-3/U-5/U-6; F-0 выполнен):
1. Go: `go 1.26` + `toolchain go1.26.x`, builder `golang:1.26`; `govulncheck` блокирующий (D-1) — §2.1, §2.4, §3.1.
2. Порт `core` — **8090** (`MV_CORE_ADDR`, `MV_CORE_URL`), HTTP-сервер принадлежит процессу (`shared/runtime`) (D-7) — §1.2, §1.4, §4.2, §7.2.
3. Профиль `legacy` = as-is `narrative-orchestrator` + as-is `semantic-memory` (`127.0.0.1:8083`) + `chromadb` (D-3); профиль `bot` подтверждён (D-10) — §1.3, §1.4, §5.5.
4. MinIO — **собственная сборка из исходников** `build/minio.Dockerfile` по ADR-021 (OQ-A-20 решён пользователем на G2); builder-образ Go с пином; тот же образ в testcontainers (D-2) — §2.4, §2.5, §5.2.
5. `prompts-*`: versioning **off**, ILM **30 дней**, вне бэкапа (T-13) — §5.2, §5.6.
6. `.gitleaks.toml`: allowlist **только `*.example`** (T-16) — §4.5 п. 1.
7. Neo4j **без APOC** (T-17) — §5.3.
8. Логи бота — ротация по объёму **`10m × 3`** (D-12) — §7.1.
9. `segment.ms=1d`, retention 30/90/180 закреплены в ADR-007 (D-9, U-3) — §5.1 без изменений по существу.
10. JSON Schema — `santhosh-tekuri/jsonschema/v6` (D-5); `objstore.EnsureBucket` включает versioning/ILM, `Capabilities()` (D-6); `build/versions.env` — единый источник версий (D-13); `*.jsonl merge=binary` **без** `-diff` (D-14) — §2.4, §4.5 п. 10, §5.2, §9.5.
11. Префикс `MV_` для всех платформенных переменных подтверждён (D-4, G-11, W-3); `MV_TELEGRAM_ALLOWED_USER_IDS`, `MV_BUS_VALIDATE_ON_READ`, `MV_BACKUP_AGE_RECIPIENT` добавлены — §4.1, §4.2.
12. §4.5 гигиена F-1 — точный список команд по факту `git ls-files` после F-0 (шаг 0 выполнен, коммит `744fb10`); плейсхолдер в `shared/oracle/README.md` строки 14/29/68 (U-5); код в `services/_archive/` (U-1, F-3), не удаляется — §4.5, §4.6.
13. CI: пины Actions по SHA, `permissions`, `go mod verify`/`tidy -diff`, Dependabot, CODEOWNERS, `privacy-scan`, `compose-lint` по ADR-010 доп. п. 4; ветки `integration/mvp-1`, `epic/EPIC-00N-*` (T-15, `teams.md` §3) — §3.
14. Матрица замера — `scripts/llm-bench.ps1` (основной, Windows) и `scripts/llm-bench.sh`; конфигурации по U-2; критерии NFR-002/NFR-090 (F-8) — §6.4.
15. Бэкапы `links.db`: требование «вне OneDrive» снято (U-6), именованный том `gateway-data` и шифрование `age` остаются (T-8) — §5.4, §5.6.
16. Миграции SQLite — `goose` без `Down` (ADR-019; в v0.1 был `golang-migrate`) — §5.4, §8.
17. Runbook: заготовки по каждому процессу (`gateway`, `core`, `memory`) и боту — §9.8; чек-лист §10 перестроен под F-0…F-10; §11 — статусы замечаний; §12 — риски обновлены; §13 — вопросы владельцу.

Версии образов и инструментов проверены 2026-09-09 по Docker Hub, `proxy.golang.org` и GitHub Releases (раздел 2.4). Пины — в одном месте (`build/versions.env` для образов и toolchain; `go.mod`; `.github/workflows/go.yml`; `.pre-commit-config.yaml`); обновление пинов — отдельные задачи, не «по дороге».

---

## 0. Сводка решений

| Область | Решение | Где закреплено |
|---|---|---|
| Окружения | три: `dev` (машина владельца, Docker Desktop + Git Bash/PowerShell), `ci` (GitHub-hosted `ubuntu-latest` с Docker, без GPU), `prod` (та же машина владельца, `COMPOSE_PROFILES=memory,bot`; LLM — нативный `llama-server` вне compose, профиль `gpu` только при контейнерном Ollama) | §1 |
| Топология | один образ `multiverse-core` с тремя бинарниками; процессы `gateway` (:8088), `core` (:8090), `memory` (:8082), `telegram-bot`; профили compose `memory`, `gpu`, `bot`, `dev`, `legacy` | §1.2, §1.3 |
| Сборка | единый модуль `multiverse-core.io`, **`go 1.26` + `toolchain go1.26.x`** (ADR-001 доп. п. 1); Makefile — одна цель на действие; `build/Dockerfile` multi-stage с кэшем, runtime `distroless/static:nonroot`; **`build/minio.Dockerfile`** — MinIO из исходников (ADR-021); **`build/versions.env`** — единственный источник версий образов/toolchain для compose, Makefile, testcontainers | §2 |
| CI | `.github/workflows/go.yml`: `unit`, `integration` (testcontainers-go, образы из `versions.env`), `e2e`, `contracts`, `security` (gitleaks + govulncheck **блокирующий** + privacy-scan), `compose-lint`; Actions с пином по SHA, `permissions: contents: read`, Dependabot, CODEOWNERS; ≤ 10 мин; `validate-blueprints.yml` удаляется; `qwen-*` не трогаем | §3 |
| Конфигурация | только env с префиксом `MV_` через единственный пакет `shared/env` (реестр `Declare`); сторонние переменные образов — без префикса; `.env.example` полный и сверяется `mvctl env check`; `os.Getenv` вне `shared/env` запрещён линтером | §4.1–§4.3 |
| Секреты | `.env`, `.mcp.env`, `.claude/settings.local.json` вне индекса; `gitleaks` в pre-commit и CI; allowlist **только `*.example`**; compose без паролей по умолчанию; `git filter-repo` — только по команде владельца | §4.4, §4.5 |
| Данные | Redpanda 8 топиков `-p 1 -r 1`, retention 30/90/180 дней, `segment.ms=1d`; MinIO: versioning + ILM включает `objstore.EnsureBucket`, `prompts-*` без versioning и ILM 30 дн.; Qdrant и Neo4j (без APOC) перестраиваемые; SQLite `links.db`/`gateway.db` в именованном томе `gateway-data`, бэкап `links.db` шифрованный `age`, ≤ 30 дней | §5 |
| LLM-рантайм | **нативный `llama-server` (llama.cpp) на `127.0.0.1:1234`, вне compose** (U-8, ADR-005 доп. 2): `scripts/llm-server.ps1`/`.sh`, `make llm-up/llm-down/llm-health`; провайдер `openai_compat` (`/v1/chat/completions` + `response_format json_schema`); прогрев не нужен — модель грузится при старте, `/health` 503 = loading. Ollama — **опциональный** второй рантайм (нативно или профиль `gpu`) для конфигураций C/A и эмбеддингов; матрица замера — `scripts/llm-bench.ps1`/`.sh`, варианты **E** (базовый кандидат), E+, C, A | §6 |
| Наблюдаемость | `slog` JSON в stdout, ротация docker-логов (платформа `50m × 5`, бот **`10m × 3`**); `/health` у каждого процесса; метрики MVP-1 — `mvctl session-report` → CSV; Prometheus/Grafana — EPIC-012 | §7 |
| Деплой/откат | образ `multiverse-core:<git-sha>`; `MV_IMAGE_TAG` в `.env`; откат = предыдущий тег + при необходимости восстановление файла SQLite из бэкапа (ADR-019: `Down` не пишутся); бэкап перед каждым релизом | §8 |
| Архив вместо удаления | весь выводимый из сборки код → `services/_archive/<исходный путь>/` + `ARCHIVED.md` (U-1); из индекса удаляется только не-код (бинарники, логи, секреты); данные as-is — `make archive-legacy` перед пересозданием томов | §4.6, §5.5 |

---

## 1. Окружения

### 1.1. Три окружения и их различия

| | `dev` (машина владельца) | `ci` (GitHub Actions) | `prod` (та же машина, «боевой» запуск) |
|---|---|---|---|
| Хост | Windows 11, i9-13900, 128 ГБ RAM, RTX 4090 24 ГБ; Docker Desktop 29.x (WSL2), Compose v5.x (`COMPOSE_ENV_FILES` поддерживается), Go **1.26.x** локально (ставится при F-2; сейчас 1.25.3), Git Bash, PowerShell 7, Python (`pre-commit`) | `ubuntu-latest` (4 vCPU, 16 ГБ, Docker есть, GPU нет) | как `dev` |
| Как запускается платформа | `make llm-up` (нативный llama-server, вне compose) → `make up` (compose) **или** `go run ./cmd/multiverse --contexts=all --bus=memory` для быстрой отладки без инфраструктуры | `go test` (unit/e2e без Docker; integration — testcontainers) | `make llm-up` → `make up` с `COMPOSE_PROFILES=memory,bot` (+ `gpu`, только если выбран контейнерный Ollama) |
| LLM | **`llama-server` нативно, `127.0.0.1:1234`** (`MV_LLM_PROVIDER=openai_compat`); Ollama нативно или профиль `gpu` — опционально, для конфигураций C/A и эмбеддингов | нет; `providers/recorded` + `providers/fake` | `llama-server` нативно, модель резидентна с момента старта процесса |
| Данные | тома Docker; допускается `make reset` (полная очистка) | эфемерные контейнеры testcontainers | тома Docker + бэкапы (§5.6) |
| Секреты | `.env` (локально, вне git) | GitHub Secrets: для MVP-1 **не требуются** (см. §4.4) | `.env`, права файла только владельцу |
| Порты | **все** публикуются только на `127.0.0.1` (ADR-009 доп. п. 3, SEC-13) | не публикуются | как `dev` |
| Профили compose | `dev` (+ Redpanda Console), по желанию `memory`, `gpu`, `legacy`, `bot` | — | `memory`, `bot`, (`gpu`) |
| Логи | `docker compose logs`, json-file с ротацией | артефакты job | json-file с ротацией; платформа `50m × 5`, бот `10m × 3` (ADR-006 доп. п. 5) |

`staging` отсутствует намеренно: одна машина, один оператор; роль «предпрод» выполняет прогон e2e/интеграции в CI и `make ci` локально перед `make deploy`.

### 1.2. Топология процессов (ADR-001 доп. п. 4, overview §13)

```
telegram-bot ──HTTP──▶ gateway (:8088) ──┐   /v1/admin/* → прокси в core
                                          ├── Redpanda (8 топиков) ◀── core (state,mechanics,swarm,llm,laws; :8090 health/admin)
                        memory (:8082) ◀──┘        │
                        │  └── Qdrant, Neo4j        └── MinIO (собственная сборка)
                                                   llama-server (llama.cpp, нативно, вне compose) :1234
                                                   [опц.] Ollama (native :11434 или профиль gpu)
mvctl (CLI, не демон) ── читает шину/MinIO, публикует в шину (laws bump, world init) через 127.0.0.1:19092
```

Один бинарник `cmd/multiverse`, флаг `--contexts` (или `MV_CONTEXTS`) задаёт набор контекстов процесса. У каждого процесса есть HTTP-сервер `MV_<PROCESS>_ADDR` (`shared/runtime`): `/health` (NFR-030) и служебные маршруты, которые монтируют контексты (`/v1/admin/agents*`, `/v1/admin/llm/usage`). Порт `core` — **8090** (`MV_CORE_ADDR=:8090`; 8081 исторически занимал Schema Registry — D-7); gateway проксирует `/v1/admin/*` на `MV_CORE_URL` (ADR-010 п. 1, ADR-009 п. 9 — только клиентам из allow-list). Раздел `admin` описан в `api/gateway.openapi.yaml` (system-analyst).

### 1.3. Compose-профили (ADR-001 доп. п. 6)

| Профиль | Сервисы | Когда включён |
|---|---|---|
| (без профиля — всегда) | `redpanda`, `redpanda-init`, `minio` (образ `build/minio.Dockerfile`), `minio-init`, `gateway`, `core` | всегда: минимальный стек «соло без памяти и без LLM» (деградация FR-080/NFR-072) |
| `memory` | `qdrant`, `neo4j`, `memory` | память Should (EPIC-005-memory); в `prod` включён |
| `gpu` | `ollama` (контейнер с NVIDIA) | **опционально**: только если выбран Ollama и именно в контейнере (§6.5). Основной рантайм `llama-server` — нативный процесс, в compose его нет и профиля для него не заводим (ADR-005 доп. 2 п. 7) |
| `bot` | `telegram-bot` | когда задан `MV_TELEGRAM_BOT_TOKEN`; в CI/e2e выключен; `env_file`/`environment` с токеном — только у этого сервиса (D-10, `compose-lint` проверяет) |
| `dev` | `redpanda-console` | удобство разработчика; консоли только здесь (SEC-33) |
| `legacy` | as-is `narrative-orchestrator` (`build/legacy.Dockerfile`), **as-is `semantic-memory` (`127.0.0.1:8083`) и `chromadb`** | только на время миграции GM (`MV_GM_PATH=legacy`, S5); оркестратор жёстко ходит в `/v1/context-with-events` без деградации (`orchestrator.go:117,177`), поэтому Chroma живёт здесь до S5 и удаляется вместе с профилем в EPIC-003 I2 (D-3, ADR-004 доп. п. 8). Legacy-сервисы читают **свои as-is переменные** (без `MV_`), собираются из `services/narrative-orchestrator`, `services/semantic-memory` (замороженные, `FROZEN.md`), не из корневого модуля |

Набор профилей задаётся `COMPOSE_PROFILES` в `.env` (`COMPOSE_PROFILES=memory,bot`); `make up PROFILES=...` переопределяет. Жизнеспособность профиля `legacy` подтверждается до старта волны 1 (`epics.md` §6 F-6); запасной вариант S5 — «флаг удалён, `gm_path=agent` в 100 % `llm.output`».

### 1.4. Порты (все — `127.0.0.1`; `compose-lint` проверяет, SEC-13)

| Сервис | Внутри сети compose | На хосте |
|---|---|---|
| gateway | `gateway:8088` | `127.0.0.1:8088` |
| core (health/admin) | `core:8090` | `127.0.0.1:8090` |
| memory | `memory:8082` | `127.0.0.1:8082` |
| telegram-bot (health) | `telegram-bot:8089` | `127.0.0.1:8089` |
| Redpanda Kafka | `redpanda:9092` (internal listener) | `127.0.0.1:19092` (external listener) |
| Redpanda Admin | `redpanda:9644` | `127.0.0.1:9644` |
| Redpanda Console (`dev`) | `redpanda-console:8080` | `127.0.0.1:8092` |
| MinIO API / Console | `minio:9000` / `9001` | `127.0.0.1:9000` / `9001` (консоль — только профиль `dev`; в остальных не публикуется) |
| Qdrant REST / gRPC | `qdrant:6333` / `6334` | `127.0.0.1:6333` / `6334` |
| Neo4j HTTP / Bolt | `neo4j:7474` / `7687` | `127.0.0.1:7474` / `7687` (HTTP/Browser — только профиль `dev`) |
| **`llama-server`** (нативный процесс, не сервис compose) | `host.docker.internal:1234` (для контейнеров) | `127.0.0.1:1234` — слушает **только loopback** (`--host 127.0.0.1`); веб-UI, `--ui-mcp-proxy` и `--tools all` наружу не публикуются (SEC-15) |
| Ollama (опционально) | `ollama:11434` (профиль `gpu`) или `host.docker.internal:11434` (нативно) | `127.0.0.1:11434` |
| legacy `semantic-memory` | `semantic-memory:8082` | `127.0.0.1:8083` (профиль `legacy`) |
| legacy `chromadb`, `narrative-orchestrator` | `chromadb:8000`, `narrative-orchestrator` | не публикуются |
| `redpanda-init`, `minio-init` | — | ничего не публикуют |

Два listener'а Redpanda (`internal://redpanda:9092`, `external://localhost:19092`) нужны, чтобы один и тот же кластер обслуживал контейнеры и процессы, запущенные `go run` на хосте, **и `mvctl`, который публикует в шину** (`laws bump`, `world init`) — на хосте это `MV_KAFKA_BROKERS=127.0.0.1:19092` (§4.2). Schema Registry (as-is 8081) не поднимается (ADR-007: реестр схем — в репозитории).

Порт `1234` `llama-server` **не публикуется compose** — процесс запускается на хосте и виден контейнерам через `host.docker.internal` (`extra_hosts: ["host.docker.internal:host-gateway"]` у `core`); `compose-lint` проверяет, что сервиса `llama-server` в compose нет и что `MV_LLM_URL` контейнера указывает на `host.docker.internal` либо на сервис внутри сети, но не на публичный адрес (§3.1.1 п. 5).

---

## 2. Сборка и зависимости

### 2.1. Единый модуль и toolchain (ADR-001 доп. п. 1, F-2)

- `go.mod` в корне: `module multiverse-core.io`, **`go 1.26`**, **`toolchain go1.26.x`** — последний патч линейки 1.26 на день выполнения F-2 (проверить `go.dev/dl`; на 2026-09-09 поддерживаются 1.26 и 1.27, актуальный релиз 1.27.1). Точный патч фиксируется один раз в `build/versions.env` (`GO_VERSION=1.26.x`) и оттуда попадает в `go.mod` (`toolchain`), `build/Dockerfile` (`golang:${GO_VERSION}-bookworm`) и `setup-go` (`go-version-file: go.mod`). Переход на 1.27 — отдельной задачей после MVP-1 (Dependabot `gomod` уведомит). Замечание v0.1 о Go 1.25 закрыто (D-1).
- `go.work`, `go.work.sum` удаляются. Замороженные сервисы (8 каталогов с `FROZEN.md`) и `services/_archive/**` сохраняют свои `go.mod`, в корневой модуль не входят, `go build ./...` их не видит (отдельные модули), в `.golangci.yml` перечислены в `run.skip-dirs`/`exclusions.paths` на всякий случай.
- Локальный запуск на пути с кириллицей: `go build` без `-buildvcs=false` падает (`audit-facts` §1). Makefile экспортирует `GOFLAGS=-buildvcs=false` для локальных целей; версия бинарника прошивается через `-ldflags "-X main.version=$(GIT_SHA)"`, поэтому VCS-штамп не нужен. В CI флаг не используется.
- `make` на Windows: Git for Windows не содержит `make`; ставится `winget install ezwinports.make` (GNU make 4.4), Makefile объявляет `SHELL := bash` и `.ONESHELL`. Альтернатива без установки — `wsl make ci`. Скрипты в `scripts/` — парами `.sh` (CI/WSL) и `.ps1` (Windows, где это нужно оператору: **`llm-server`**, `llm-bench`, `backup`); Makefile вызывает `.sh`, а для `llm-*` на Windows — `pwsh scripts/<имя>.ps1` (llama-server запускается нативно на Windows, не в WSL: GPU и пути `D:\Models\…`).

### 2.2. Makefile — целевой набор (F-6)

| Цель | Что делает (одна команда — одно действие) |
|---|---|
| `make build` | `go build -o bin/ ./cmd/...` (три бинарника: `multiverse`, `telegram-bot`, `mvctl`) |
| `make lint` | `golangci-lint run` (конфиг `.golangci.yml`: `govet`, `staticcheck`, `errcheck`, `depguard` границ `internal/*` по ADR-001 доп. п. 2, `forbidigo` для `os.Getenv`/`time.Now` вне `shared/*` и `log.Printf`; `gofmt`/`goimports`) |
| `make test` | `go test -short -race -count=1 -cover ./...` (unit, без сети) |
| `make test-integration` | `go test -tags integration -count=1 -timeout 15m ./...` (testcontainers; нужен Docker и собранный образ MinIO — цель зависит от `minio-image`) |
| `make test-e2e` | `go test -tags e2e -count=1 -timeout 10m ./...` (один процесс `--contexts=all --mode=replay --bus=memory`) |
| `make contracts` | `go run ./cmd/mvctl contracts check && go run ./cmd/mvctl blueprint validate blueprints/ && go run ./cmd/mvctl env check && go test ./shared/contracts/... -run TestSchemasValid` |
| `make secrets-scan` | `gitleaks git --no-banner --redact --log-opts="$(BASE)..HEAD" .` (диапазон ветки; `BASE=integration/mvp-1` по умолчанию) + `gitleaks dir --no-banner --redact .` (рабочая копия) |
| `make vuln` | `govulncheck ./...` |
| `make compose-lint` | `scripts/compose-lint.sh` (§3.1.1) |
| `make ci` | `lint test contracts secrets-scan vuln compose-lint test-e2e` — то же, что CI без Docker-тестов; `make ci-full` добавляет `test-integration` |
| `make image` | `docker build -t multiverse-core:$(GIT_SHA) -t multiverse-core:dev -f build/Dockerfile .` |
| `make minio-image` | `docker build -f build/minio.Dockerfile --build-arg MINIO_TAG=$(MINIO_TAG) --build-arg MINIO_REPO=$(MINIO_REPO) --build-arg MINIO_BUILDER_IMAGE=$(MINIO_BUILDER_IMAGE) -t $(MINIO_IMAGE) .` (§2.5; аргументы из `build/versions.env`) |
| `make up` | `docker compose --profile … up -d --wait` (compose получает `COMPOSE_ENV_FILES=.env,build/versions.env`) → `make health`; LLM-процесс **не запускает** — только проверяет `GET $MV_LLM_URL/health` и печатает предупреждение, если не `200` (деградация FR-080/NFR-072) |
| `make down` | `docker compose down` (тома сохраняются); `make reset` — `down -v` с подтверждением и проверкой свежего бэкапа. LLM-процесс не трогает — `make llm-down` отдельно |
| **`make llm-up`** | `pwsh scripts/llm-server.ps1` (Windows) / `scripts/llm-server.sh` (WSL/Linux) — старт нативного `llama-server` и ожидание `/health = 200` (до 180 с); повторный вызов при живом процессе — no-op с сообщением (§6.2) |
| **`make llm-down`** | остановка процесса по PID-файлу `ops/llm-server.pid` (`Stop-Process` / `kill`); тома и модели не трогаются |
| **`make llm-health`** | `GET $MV_LLM_URL/health` (200 / 503 `loading`) + `GET $MV_LLM_URL/v1/models` (печатает имена моделей) + `nvidia-smi --query-gpu=memory.used`; код возврата ≠ 0, если не `200` |
| `make health` | обходит `/health` gateway/core/memory/bot, вызывает `make llm-health`, печатает таблицу и возраст последнего бэкапа; код возврата ≠ 0, если что-то не `ok` |
| `make models` | **опционально, только при `MV_LLM_PROVIDER=ollama`**: `ollama pull` по списку `ops/models.txt`. Для llama-server модели — файлы `.gguf` в `MV_LLM_MODELS_DIR`, скачивание вручную (`huggingface-cli`/браузер), список — `ops/models.txt` секция `gguf` |
| `make warm` | **только для Ollama**: `POST /api/generate {"model":M,"prompt":"","keep_alive":-1}` для каждой модели. Для llama-server прогрев не нужен (модель загружена при старте, `mmap+mlock`), но первый запрос после старта прогревает граф/кэш — `make llm-up` делает один пустой `/v1/chat/completions` (§6.3) |
| `make bench` | `pwsh scripts/llm-bench.ps1` (в WSL — `scripts/llm-bench.sh`) — матрица замера §6.4, результат в `ops/metrics/bench-<date>.csv`; **требует поднятого LLM** (`make llm-up`), сервер сам не запускает |
| `make backup` / `make restore FILE=` | §5.6 |
| `make archive-legacy` | копия томов as-is (MinIO, Redpanda) в `./backups/legacy-<date>/` до пересоздания томов (§5.5; ADR-004 доп. п. 6); однократно, вне git |
| `make deploy` | `ci` + `backup` + `image` + `up` с `MV_IMAGE_TAG=$(GIT_SHA)` (§8) |
| `make rollback` | `MV_IMAGE_TAG` ← `.env.previous` → `up` (§8) |
| `make logs SERVICE=` | `docker compose logs -f --tail=200 $(SERVICE)` |
| `make replay RECORDING=` | `go run ./cmd/multiverse --contexts=all --mode=replay --bus=memory --recording=$(RECORDING)` |
| `make clean` | `rm -rf bin/ coverage.out` (без `docker system prune` — он удалял чужие образы) |

Правила: без `docker-compose` (v1) — только `docker compose`; без `latest`; `SERVICES` as-is и цели `build-service` для 15 сервисов исчезают вместе с workspace; версии — только через `build/versions.env` (`include build/versions.env` в Makefile).

### 2.3. Dockerfile платформы (F-6)

Один образ на три бинарника; `command` в compose выбирает точку входа.

```dockerfile
# build/Dockerfile
# syntax=docker/dockerfile:1.7
ARG GO_VERSION=1.26.x                      # значение подставляет Makefile/CI из build/versions.env
FROM golang:${GO_VERSION}-bookworm AS builder
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

- `distroless/static` подходит: все зависимости CGO-free (`modernc.org/sqlite`, `minio-go`, `kafka-go`, `qdrant/go-client`, `goose`); ONNX/CGO-ветка as-is уходит в `services/_archive/fake_deps/`.
- В образе нет shell и `curl` — healthcheck compose вызывает подкоманду бинарника: `/multiverse health --url http://127.0.0.1:8088/health` (возвращает 0/1). Подкоманда — часть каркаса `cmd/multiverse` (F-2).
- Кэш модулей и сборки — `--mount=type=cache`; пересборка при изменении кода ≈ 20–40 с.
- `.dockerignore`: `Docs/`, `services/` (замороженные и `_archive`), `.git`, `bin/`, `backups/`, `*.exe`, `*.log`, `.env*` кроме `.env.example`, `testdata/recordings`.
- Корневой `Dockerfile` и копии `services/{entity-actor,evolution-watcher,rule-engine}/Dockerfile` — в `services/_archive/build/` (F-3, не удаляются); as-is `build/Dockerfile` переименовывается в `build/legacy.Dockerfile` для профиля `legacy`.

### 2.4. Версии инструментов и образов (проверено 2026-09-09; источник истины после F-5 — `build/versions.env`)

| Компонент | Пин | Источник / примечание |
|---|---|---|
| Go toolchain | **`go1.26.x`** (последний патч 1.26 на день F-2; актуальный релиз Go — 1.27.1) | go.dev/dl; `GO_VERSION` в `versions.env` |
| Builder image платформы | `golang:${GO_VERSION}-bookworm` | Docker Hub |
| Runtime image | `gcr.io/distroless/static-debian12:nonroot` | по digest в `build/Dockerfile` (обновляется отдельной задачей) |
| Redpanda | `docker.redpanda.com/redpandadata/redpanda:v26.1.17` | Docker Hub; v26.2.2 вышел 2026-08-22 — берём зрелую линейку 26.1, переход на 26.2 отдельной задачей. Том as-is v24.2.5 **пересоздаётся** (§5.5) |
| Redpanda Console | `docker.redpanda.com/redpandadata/console:v3.11.0` | профиль `dev` |
| MinIO | **собственный образ `multiverse-core/minio:RELEASE.2025-10-15T17-29-55Z`** из `build/minio.Dockerfile`; исходники тега — из **форка владельца** `github.com/alekseizabelin1985-spec/minio` (upstream `minio/minio` архивирован; фикс CVE-2025-62506 включён) | ADR-021, OQ-A-20 решён на G2; форк — U-10, форк делает владелец в F-6 (§2.5). Опубликованный `minio/minio:RELEASE.2025-09-07T16-13-09Z` **не используется** |
| MinIO builder | `MINIO_BUILDER_IMAGE=golang:1.26.x-bookworm` — первая попытка; если тег не собирается новым toolchain — `golang:1.24.x-bookworm` (по `go.mod` MinIO; ADR-021 вариант B) — фиксируется при F-6 в `versions.env` | Docker Hub |
| MinIO Client (`mc`) | `minio/mc:RELEASE.2025-08-13T08-35-41Z` — последний опубликованный; только init-контейнер | при исчезновении образа — `mvctl storage init` на `minio-go` (ADR-021 п. 1) |
| Qdrant | `qdrant/qdrant:v1.19.1` | клиент `github.com/qdrant/go-client v1.19.2` |
| Neo4j | `neo4j:5.26.30-community`, **без `NEO4J_PLUGINS`** (T-17) | драйвер `neo4j-go-driver/v5 v5.28.4` |
| Chroma (только профиль `legacy`) | `chromadb/chroma:<тег as-is, зафиксировать при F-6>` (as-is `latest` — заменить на конкретный тег, который работает с as-is `semantic-memory`) | D-3 |
| **llama.cpp `llama-server`** | **пин билда обязателен**: `LLAMACPP_BUILD=b<NNNN>` в `build/versions.env` — номер релиза из `ggml-org/llama.cpp` (`llama-server --version` печатает `build: <N> (<sha>)`); фактическое значение вносится при F-6 по установленной у владельца сборке (`D:\Models\llama\llama\llama-server.exe`). Обновление — отдельной задачей с прогоном золотого набора (§12) | GitHub Releases `ggml-org/llama.cpp`; справочная запись — самообновления нет, бинарник ставится вручную |
| Модель по умолчанию (вариант E) | `Qwen3.8-27B-UD-Q3_K_XL.gguf` (unsloth, 13,1 ГБ) в `MV_LLM_MODELS_DIR`; `LLM_MODEL_DEFAULT` в `versions.env` — **имя** модели, не путь | HF `unsloth/Qwen3.8-27B-GGUF`; ADR-005 доп. 2 |
| Ollama (опционально) | контейнер `ollama/ollama:0.33.3`; нативно — 0.33.3 | нужен только при `MV_LLM_PROVIDER=ollama` (конфигурации C/A, эмбеддинги); требование ≥ 0.12 выполнено |
| testcontainers-go | `v0.44.0` + `modules/redpanda`, `modules/minio` (образ — наш, через `testkit.Versions()`), `modules/qdrant`, `modules/neo4j` | proxy.golang.org |
| golangci-lint | `v2.13.2`; action `golangci/golangci-lint-action` v9.3.0 **по SHA** | GitHub |
| gitleaks | `v8.30.1`; action `gitleaks/gitleaks-action` v3.0.0 по SHA; pre-commit hook `rev: v8.30.1` | GitHub |
| govulncheck | `golang.org/x/vuln v1.8.0`; action `golang/govulncheck-action` v1.1.0 по SHA | GitHub |
| GitHub Actions базовые | `actions/checkout` v7.0.1, `actions/setup-go` v7.0.0, `actions/cache` v6.1.0, `actions/upload-artifact` v7.0.1, `docker/setup-buildx-action` v4.3.0, `docker/build-push-action` v7.3.0 — **все по SHA коммита с комментарием версии** (T-15; SHA берётся `gh api repos/<owner>/<repo>/git/ref/tags/<tag>` при F-7, далее обновляет Dependabot) | GitHub Releases |
| hadolint | `hadolint/hadolint-action` v3.x по SHA | для `build/Dockerfile`, `build/minio.Dockerfile` |
| pre-commit | `v4.6.2`; `pre-commit/pre-commit-hooks rev: v5.0.0` (или новее на день F-1) | GitHub |
| Go-библиотеки (F-2/F-4/F-5) | `segmentio/kafka-go v0.4.51`, `minio-go/v7 v7.3.0`, `modernc.org/sqlite v1.58.0`, **`pressly/goose/v3 v3.28.0`** (ADR-019; `golang-migrate` из v0.1 снят), **`santhosh-tekuri/jsonschema/v6 v6.0.3`** (JSON Schema 2020-12; D-5), `oklog/ulid/v2 v2.1.2`, `go-telegram/bot v1.25.0` (ADR-018), `gopkg.in/yaml.v3`, `stretchr/testify` | proxy.golang.org |

`build/versions.env` (читают `docker-compose.yml` через интерполяцию `${...}`, Makefile через `include`, тесты через `shared/testkit.Versions()`; CI — `cat build/versions.env >> $GITHUB_ENV`):

```dotenv
GO_VERSION=1.26.x
REDPANDA_IMAGE=docker.redpanda.com/redpandadata/redpanda:v26.1.17
REDPANDA_CONSOLE_IMAGE=docker.redpanda.com/redpandadata/console:v3.11.0
MINIO_TAG=RELEASE.2025-10-15T17-29-55Z
MINIO_REPO=https://github.com/alekseizabelin1985-spec/minio.git   # форк владельца (U-10)
MINIO_IMAGE=multiverse-core/minio:RELEASE.2025-10-15T17-29-55Z
MINIO_BUILDER_IMAGE=golang:1.26.x-bookworm
MC_IMAGE=minio/mc:RELEASE.2025-08-13T08-35-41Z
QDRANT_IMAGE=qdrant/qdrant:v1.19.1
NEO4J_IMAGE=neo4j:5.26.30-community
OLLAMA_IMAGE=ollama/ollama:0.33.3           # только профиль gpu (опционально)
CHROMA_IMAGE=chromadb/chroma:<tag>          # только профиль legacy
# --- LLM-рантайм вне compose: пины проекта (машинно-независимые) ---
LLAMACPP_BUILD=b<NNNN>                      # llama-server --version; фиксируется при F-6
LLM_MODEL_DEFAULT=Qwen3.8-27B-UD-Q3_K_XL    # имя (stem) модели варианта E; = model в блупринтах
GOLANGCI_LINT_VERSION=v2.13.2
GITLEAKS_VERSION=v8.30.1
```

Машинно-зависимые пути LLM (`D:\Models\…`, путь к `llama-server.exe`) в `build/versions.env` **не попадают** — файл в Git и общий для всех окружений; они живут в `.env` как `MV_LLM_BIN`, `MV_LLM_MODELS_DIR`, `MV_LLM_MODEL_FILE`, `MV_LLM_SLOT_SAVE_PATH` (§4.2) и читаются только `scripts/llm-server.*`, не платформой. Compose эти переменные не интерполирует (обратные слэши Windows), в контейнеры они не передаются.

Dependabot (`.github/dependabot.yml`): экосистемы `gomod` (еженедельно, группировать minor/patch), `github-actions` (еженедельно; обновляет SHA-пины), `docker` (ежемесячно, каталог `build/`; пины compose/`versions.env` обновляются вручную после проверки — Dependabot их не видит, поэтому раз в месяц оператор сверяет `versions.env` с Docker Hub).

### 2.5. `build/minio.Dockerfile` — MinIO из исходников (ADR-021, F-6)

```dockerfile
# build/minio.Dockerfile
# syntax=docker/dockerfile:1.7
ARG MINIO_BUILDER_IMAGE=golang:1.26.x-bookworm   # из build/versions.env
FROM ${MINIO_BUILDER_IMAGE} AS builder
ARG MINIO_TAG=RELEASE.2025-10-15T17-29-55Z       # последний тег upstream; репозиторий архивирован
ARG MINIO_REPO=https://github.com/alekseizabelin1985-spec/minio.git   # форк владельца (U-10); upstream https://github.com/minio/minio.git — запасной
WORKDIR /src
RUN git clone --depth 1 --branch "${MINIO_TAG}" "${MINIO_REPO}" .
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-trimpath
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -tags kqueue -ldflags "$(go run buildscripts/gen-ldflags.go)" -o /out/minio .

FROM alpine:3.22
RUN adduser -D -u 1000 minio && mkdir -p /data && chown minio:minio /data
COPY --from=builder /out/minio /usr/bin/minio
USER minio
VOLUME /data
EXPOSE 9000 9001
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s \
  CMD wget -qO- http://127.0.0.1:9000/minio/health/live >/dev/null || exit 1
ENTRYPOINT ["/usr/bin/minio"]
CMD ["server", "/data", "--console-address", ":9001"]
```

- Runtime — `alpine` (не distroless): нужен `wget` для healthcheck и запуск не от root; образ ≈ 120 МБ. Сборка ≈ 5 мин впервые, далее из кэша (`cache-from: type=gha` в CI, локально — BuildKit).
- Что проверить при F-6 (критерий задачи): (а) `go.mod` тега собирается выбранным builder-образом (сначала 1.26; при ошибке — 1.24 по ADR-021); (б) `minio --version` печатает `RELEASE.2025-10-15T17-29-55Z`; (в) `mc admin info local` работает с `mc` указанной версии; (г) contract-тест `objstore` (F-5) проходит против этого образа в testcontainers; (д) `Capabilities()` возвращает `Versioning=true, Lifecycle=true`.
- **Страховка от исчезновения исходников — две линии (U-10, вопрос 3 закрыт):**
  1. **Форк `minio/minio` в аккаунт `alekseizabelin1985-spec`** — делает **владелец** (кнопка Fork на GitHub; агенты внешние системы не меняют), однократно, до сборки образа в F-6. Форк архивированного репозитория сохраняет теги, включая `RELEASE.2025-10-15T17-29-55Z`. `build/minio.Dockerfile` клонирует **форк** (`MINIO_REPO`), upstream остаётся запасным значением аргумента.
  2. **Тарбол исходников** тега (`git archive --format=tar.gz --prefix=minio-<tag>/ <tag> > backups/minio-src-<tag>.tar.gz`) в `backups/` вне git — на случай потери доступа к GitHub целиком; проверяется распаковкой при F-6.
- Критерий F-6 дополняется: `docker build -f build/minio.Dockerfile --build-arg MINIO_REPO=<форк> …` собирается; `git ls-remote --tags <форк> | grep RELEASE.2025-10-15T17-29-55Z` не пуст; `backups/minio-src-<tag>.tar.gz` существует и распаковывается. `MINIO_REPO` — в `build/versions.env` рядом с `MINIO_TAG`.
- Триггеры замены сервера (SeaweedFS) — ADR-021 п. 4; задача-заглушка в EPIC-012.

---

## 3. CI/CD (F-7)

### 3.1. Workflow `.github/workflows/go.yml`

Триггеры: `push` и `pull_request` в `main`, `integration/**`, `epic/**`, `feature/agent-gm-core` (ветки по `teams.md` §3; `develop` не используется); `paths-ignore: ['Docs/**', '**/*.md']` (кроме `blueprints/**/*.md` — они данные; исключение через `paths` в отдельном фильтре); `concurrency: {group: ci-${{ github.ref }}, cancel-in-progress: true}`; **`permissions: {contents: read}`** на уровне workflow, job `security` — `security-events: write` (SARIF) (T-15, ADR-010 доп. п. 3). Все job'ы — `runs-on: ubuntu-latest`, `timeout-minutes` явные; все `uses:` — по SHA с комментарием версии.

Общий шаг: `cat build/versions.env >> "$GITHUB_ENV"` — версии образов доступны и compose-lint, и testcontainers.

| Job | Шаги | Время (оценка) | Блокирует мерж |
|---|---|---|---|
| `unit` | checkout → setup-go (`go-version-file: go.mod`, cache on) → `go mod verify` → `go mod tidy -diff` → `go build ./...` → `go vet ./...` → golangci-lint-action (`version: ${GOLANGCI_LINT_VERSION}`) → `go test -short -race -coverprofile=coverage.out ./...` → `scripts/coverage-gate.sh 60 internal/state internal/mechanics internal/swarm internal/llm internal/replay` (пакеты, которых ещё нет, пропускаются с предупреждением) → upload `coverage.out` | 3–4 мин | да |
| `integration` | setup-go → buildx → `docker build -f build/minio.Dockerfile … --cache-from type=gha --cache-to type=gha,mode=max --load` (образ `${MINIO_IMAGE}` для testcontainers) → `go test -tags integration -count=1 -timeout 15m ./...` (testcontainers-go поднимает Redpanda/MinIO/Qdrant/Neo4j из `testkit.Versions()`; `TESTCONTAINERS_RYUK_DISABLED=false`) | 5–7 мин (первый раз +5 мин сборка MinIO) | да |
| `e2e` | setup-go → `go test -tags e2e -count=1 -timeout 10m ./...` (один процесс, `--bus=memory`, записи `testdata/recordings/*.jsonl`; без Docker) → upload `ops/metrics/sessions/*.json` как артефакт | 1–2 мин | да |
| `contracts` | setup-go → `go run ./cmd/mvctl contracts check` → `go run ./cmd/mvctl blueprint validate blueprints/` → `go run ./cmd/mvctl env check` (`.env.example` ↔ реестр `shared/env`) → `go test ./shared/contracts/... -run TestSchemasValid` (все `schemas/**/*.json` — валидные JSON Schema 2020-12, `jsonschema/v6`) → проверка `api/*.openapi.yaml` загружается (`kin-openapi`) | 1 мин | да |
| `security` | gitleaks-action (до очистки истории — диапазон PR/ветки `--log-opts="${{ github.event.pull_request.base.sha }}..HEAD"` + `gitleaks dir` рабочей копии; после filter-repo — полная история; `GITLEAKS_LICENSE` **не нужен**: репозиторий приватный на личном аккаунте, лицензия требуется только организациям — U-9) → govulncheck-action (`go-version-file: go.mod`) — **блокирующий** (ADR-010 доп. п. 5; пока `go.mod` не переведён на 1.26 в F-2 — `continue-on-error: true`, снимается тем же PR, что F-2) → `privacy-scan`: `go run ./cmd/mvctl privacy scan testdata/` (числовые внешние ID, username; те же правила, что тест NFR-041; ADR-010 доп. п. 1) | 1–2 мин | да |
| `compose-lint` | `docker compose --env-file build/versions.env --env-file .github/ci.env config -q` (`.github/ci.env` — копия `.env.example` с подставленными фиктивными паролями; без секретов) → `scripts/compose-lint.sh` (§3.1.1) → hadolint для `build/Dockerfile`, `build/minio.Dockerfile` | < 1 мин | да |
| `image` (только `push` в `main`/`integration/mvp-1`) | buildx → `docker build -f build/Dockerfile` с кэшем `type=gha` → **без push** (образы собираются на машине владельца `make image`; публикация в GHCR — по отдельному решению) | 3 мин | нет |

Суммарно ≤ 10 мин (job'ы параллельны, критический путь — `integration`). Матрица ОС не нужна: единственная целевая платформа Linux-контейнер; локальная сборка на Windows проверяется разработчиком (`make build`). При появлении второй платформы — добавить `windows-latest` только для `unit`.

#### 3.1.1. `scripts/compose-lint.sh` (ADR-010 доп. п. 4, SEC-13/14/33)

Проверяет `docker compose config --format json` (после интерполяции):
1. каждый `image:` имеет явный тег ≠ `latest` и совпадает со значением из `build/versions.env` (NFR-071);
2. каждая публикация порта имеет вид `127.0.0.1:<host>:<container>`; у `redpanda-init`, `minio-init`, `chromadb`, `narrative-orchestrator` публикаций нет; консоли (`redpanda-console`, MinIO `9001`, Neo4j `7474`) — только в профиле `dev`;
3. нет паролей по умолчанию: в `environment` нет литералов для `MINIO_ROOT_PASSWORD`, `NEO4J_AUTH`, `MV_*_KEY`, `MV_*_PASSWORD`, `MV_TELEGRAM_BOT_TOKEN` (только `${VAR:?}`); `git grep -i minioadmin` пуст вне `Docs/` и `services/_archive/`;
4. `MV_TELEGRAM_BOT_TOKEN` присутствует только у сервиса `telegram-bot` (профиль `bot`); `env_file` у других сервисов не содержит токен;
5. `NEO4J_PLUGINS` отсутствует (T-17); `OLLAMA_ORIGINS` не `*`, `OLLAMA_HOST` не `0.0.0.0` в контейнерном профиле (SEC-15); **сервиса `llama-server` в compose нет** (нативный процесс, ADR-005 доп. 2 п. 7), а `MV_LLM_URL` у сервисов платформы указывает на `host.docker.internal`, `127.0.0.1`, имя сервиса внутри сети или RFC1918 — публичный host требует `MV_LLM_CLOUD_ENABLED=true` и в compose по умолчанию запрещён (SEC-15, ADR-005 доп. 2 п. 3).
Выход ≠ 0 при любом нарушении; тот же скрипт — `make compose-lint`.

Ветки (ADR-010, `teams.md` §3, `epics.md` v0.2 п. 4): интеграционная ветка **`integration/mvp-1`** (создана в F-0 от `feature/agent-gm-core`), ветки эпиков `epic/EPIC-00N-<slug>` на весь MVP-1, `main` — то, что задеплоено. CI одинаков на всех; правило защиты `main` и `integration/mvp-1` — required checks `unit, integration, e2e, contracts, security, compose-lint`, линейная история, без force-push, CODEOWNERS-ревью. Порядок слияния инкрементов — `epics.md` §5/§6; отдельного «интеграционного» CI не нужно — тот же workflow на `integration/mvp-1`. Сборка интеграционной ветки перед этапом интеграции = зелёный `go.yml` на `integration/mvp-1` после слияния каждого инкремента + `make ci-full` на машине владельца перед I1-α.

### 3.2. Что остаётся, что удаляется, что добавляется

- `validate-blueprints.yml` — удалить (заменён job `contracts`; ссылался на `configs/gm_*.yaml` и `go-version: 1.24`).
- `qwen-*.yml` (5 шт.) — не трогать (AI-триаж issues/PR; используют `vars`/`secrets` владельца).
- Codecov — не подключаем.
- **`.github/CODEOWNERS`** (T-15; логин владельца — **`@alekseizabelin1985-spec`**, U-9, вопрос 2 закрыт). Владелец — единственный ревьюер-человек при трёх виртуальных командах, поэтому базовое правило — на весь репозиторий, а «контрактные» каталоги перечислены отдельно, чтобы правило не терялось при добавлении подкаталогов:
  ```
  # .github/CODEOWNERS
  *                    @alekseizabelin1985-spec
  /blueprints/         @alekseizabelin1985-spec
  /laws/               @alekseizabelin1985-spec
  /config/             @alekseizabelin1985-spec
  /schemas/            @alekseizabelin1985-spec
  /shared/             @alekseizabelin1985-spec
  /build/              @alekseizabelin1985-spec
  /.github/            @alekseizabelin1985-spec
  /docker-compose.yml  @alekseizabelin1985-spec
  /Makefile            @alekseizabelin1985-spec
  ```
  В branch protection `main` и `integration/mvp-1` включается «Require review from Code Owners». Ограничение GitHub: автор PR не может апрувить собственный PR — при работе одного человека правило CODEOWNERS даёт напоминание и историю, но требование апрува ставится **только на `main`**; на `integration/mvp-1` обязательны required checks, апрув — по желанию (иначе владелец заблокирует сам себя). Репозиторий **приватный, личный аккаунт** (U-9): `gitleaks-action` работает без `GITLEAKS_LICENSE` (лицензия нужна организациям), лимит GitHub Actions — 2 000 мин/мес на Free (§12, `paths-ignore` + `concurrency`).
- **`.github/dependabot.yml`** — §2.4.
- **`.github/ci.env`** — не секрет: `.env.example` + фиктивные значения для `${VAR:?}` (`MINIO_ROOT_PASSWORD=ci-only-not-a-secret` и т. п.), нужен только `compose-lint`.

### 3.3. GPU-замер вне CI

`nightly-gpu` (ADR-010 п. 1) не выполняется на GitHub-hosted раннерах. **Решение владельца (U-12, вопрос 4 закрыт): замер — вручную**, `make bench` на машине владельца по необходимости (несколько раз за MVP-1: F-8 и после смены билда llama.cpp/модели). Self-hosted runner **не заводим** — он имел бы доступ к машине и токену репозитория, а выигрыш (история в артефактах) закрывается тем, что `ops/metrics/bench-<date>.csv` коммитится в Git. Требование к ручному прогону: `llama-server` поднят (`make llm-up`), в CSV попадают `LLAMACPP_BUILD`, имя модели и `nvidia-smi` VRAM — прогон воспроизводим по строке CSV.

### 3.4. Pre-commit (Windows-совместимо)

`.pre-commit-config.yaml` (Python `pre-commit` v4.6.2 уже есть на машине; хуки выбраны так, чтобы **не требовать bash**: `language: golang` собирает инструменты локальным Go в кэш pre-commit, `language: python` — встроенные):

```yaml
repos:
  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.30.1
    hooks:
      - id: gitleaks              # entry: gitleaks git --pre-commit --redact --staged --verbose
  - repo: https://github.com/golangci/golangci-lint
    rev: v2.13.2
    hooks:
      - id: golangci-lint-fmt     # gofmt/goimports через golangci-lint fmt
      - id: golangci-lint         # только изменённые пакеты; полный прогон — make lint
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
      - id: check-added-large-files
        args: ['--maxkb=1024']    # ловит .exe/.pdf
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-merge-conflict
      - id: mixed-line-ending
        args: ['--fix=lf']
        files: '\.(sh|yml|yaml|toml)$|^Makefile$'
```

Установка: `pre-commit install` (пишет `.git/hooks/pre-commit`; Git for Windows выполняет его своим `sh` — работает из PowerShell, Git Bash и IDE). Первый прогон: `pre-commit run --all-files`. Требования на машине: Go 1.26 в `PATH` (для `language: golang`), Python. Если `go install` gitleaks за прокси не проходит — альтернатива `winget install Gitleaks.Gitleaks` и хук `repo: local, language: system, entry: gitleaks git --pre-commit --redact --staged`. Проверка на Windows входит в критерий F-1: `git commit` с файлом, содержащим `MV_TELEGRAM_BOT_TOKEN=123456789:AA…` (фиктивный, формата токена), отклоняется хуком.

---

## 4. Конфигурация и секреты

### 4.1. Принципы

1. Конфигурация процессов — **только переменные окружения** (`.env` для compose, реальные env для `go run`), **префикс `MV_` для всех платформенных переменных** (D-4, G-11, W-3; `contracts.md` §16 п. 5). Один пакет `shared/env`: `env.Declare(...)`/`env.String("MV_KAFKA_BROKERS", "127.0.0.1:19092", "адреса брокеров")` регистрирует переменную в реестре (имя, дефолт, описание, секрет ли, обязательна ли). `os.Getenv`/`os.LookupEnv` вне `shared/env` запрещены `forbidigo` в `.golangci.yml`.
2. Переменные сторонних образов (Redpanda, MinIO, Neo4j, Ollama, Chroma) префикса не имеют — их имена диктуют образы; они живут в том же `.env`, секция «infrastructure». Исключение — `llama-server`: он не образ и переменных окружения не требует, всё задаётся флагами командной строки, поэтому его параметры хранятся как **платформенные `MV_LLM_*`** (их читает `scripts/llm-server.*`, а `MV_LLM_URL`/`MV_LLM_PROVIDER` — ещё и шлюз). Legacy-сервисы профиля `legacy` читают свои as-is имена (секция «legacy»), в реестр `shared/env` не входят.
3. Конфигурация домена — **файлы в Git**: `blueprints/*.md`, `rules/dark-forest.yaml`, `laws/*.yaml`, `config/absolute-limits.yaml`, `schemas/**`. `configs/gm_*.yaml` и `shared/config` (профили из MinIO) — в `services/_archive/` (overview §16).
4. Фича-флаги MVP-1 — тоже env: `MV_GM_PATH=agent|legacy` (миграция GM, S5), `MV_LLM_CLOUD_ENABLED`, `MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS`, `MV_LAWS_BREACH_PHASE=false`, `MV_LLM_STORE_PROMPTS=false`, `MV_BUS_VALIDATE_ON_READ=true`. Переключение флага = перезапуск процесса (горячая перезагрузка — E-F).

### 4.2. `.env.example` — целевой состав (F-1/F-5/F-6)

```dotenv
# ===== compose =====
COMPOSE_PROJECT_NAME=multiverse
COMPOSE_PROFILES=memory,bot          # набор профилей: memory,gpu,bot,dev,legacy
COMPOSE_ENV_FILES=.env,build/versions.env   # версии образов — только из build/versions.env
MV_IMAGE_TAG=dev                     # тег образа multiverse-core (make deploy подставляет git sha)

# ===== infrastructure (имена диктуют образы) =====
MINIO_ROOT_USER=                     # обязательна; не minioadmin
MINIO_ROOT_PASSWORD=                 # обязательна, ≥ 16 символов; секрет
NEO4J_PASSWORD=                      # обязательна при профиле memory; секрет (compose собирает NEO4J_AUTH=neo4j/${NEO4J_PASSWORD})
# --- Ollama: НЕОБЯЗАТЕЛЕН, заполняется только при MV_LLM_PROVIDER=ollama (конфигурации C/A, эмбеддинги) ---
#OLLAMA_KEEP_ALIVE=-1                # только профиль gpu; нативно — переменные окружения Windows
#OLLAMA_MAX_LOADED_MODELS=2
#OLLAMA_NUM_PARALLEL=1
#OLLAMA_FLASH_ATTENTION=1
#OLLAMA_KV_CACHE_TYPE=f16            # q8_0 — вариант замера с квантованным KV (§6.4)

# ===== platform, общие для всех процессов =====
MV_ENV=dev                           # dev|ci|prod
MV_LOG_LEVEL=info                    # debug|info|warn|error
MV_LOG_FORMAT=json                   # json|text
MV_MODE=live                         # live|replay
MV_BUS=redpanda                      # redpanda|memory (memory — только e2e/отладка одного процесса)
MV_BUS_VALIDATE_ON_READ=true         # валидация схемы при чтении (ADR-007 доп. п. 4)
MV_KAFKA_BROKERS=redpanda:9092       # go run на хосте: 127.0.0.1:19092
MV_MINIO_ENDPOINT=minio:9000         # без схемы; на хосте 127.0.0.1:9000
MV_MINIO_ACCESS_KEY=                 # секрет; для dev = MINIO_ROOT_USER, для prod — отдельный пользователь (mc admin user add)
MV_MINIO_SECRET_KEY=                 # секрет
MV_MINIO_USE_SSL=false
MV_WORLD_ID=dark-forest-world        # мир по умолчанию для mvctl и bootstrap
MV_BACKUP_AGE_RECIPIENT=             # публичный ключ age для бэкапа links.db (не секрет; §5.6)

# ===== gateway =====
MV_GATEWAY_ADDR=:8088
MV_GATEWAY_DATA_DIR=/data            # links.db, gateway.db (том gateway-data)
MV_GATEWAY_CLIENT_IDS=telegram-bot,ci-harness,mvctl   # allow-list X-Client-Id (ADR-009 п. 9); в prod без ci-harness
MV_GATEWAY_ACTOR_KIND_CLIENTS=ci-harness,mvctl        # кому разрешён X-Actor-Kind ci|sim
MV_CORE_URL=http://core:8090         # прокси /v1/admin/* (D-7)

# ===== core =====
MV_CORE_ADDR=:8090                   # /health и /v1/admin/* (D-7; сервер — shared/runtime)
MV_MEMORY_URL=http://memory:8082     # пусто = память выключена (деградация FR-035)
MV_SNAPSHOT_EVERY_FACTS=200          # снапшот State каждые N фактов
MV_GM_PATH=agent                     # agent|legacy — фича-флаг миграции GM (S5)
MV_LAWS_BREACH_PHASE=false

# ===== llm (рантайм по умолчанию — нативный llama-server, ADR-005 доп. 2) =====
MV_LLM_PROVIDER=openai_compat        # openai_compat|ollama|anthropic|recorded|fake
MV_LLM_URL=http://host.docker.internal:1234       # из контейнеров; go run / mvctl на хосте: http://127.0.0.1:1234
MV_LLM_API_KEY=                      # пусто для локального llama-server; секрет — только для облачного эндпоинта
MV_LLM_NUM_CTX=8192                  # контекст на слот; --ctx-size llama-server = MV_LLM_NUM_CTX × число слотов
MV_LLM_STORE_PROMPTS=false           # полные промпты в prompts-{world} (ILM 30 дн.)
MV_LLM_CLOUD_ENABLED=false           # гейт по HOST в MV_LLM_URL, не по имени провайдера (ADR-005 доп. 2 п. 3)
MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS=false
MV_LLM_CLOUD_BUDGET_USD_PER_DAY=0
MV_ANTHROPIC_API_KEY=                # секрет; только для провайдера anthropic (E-H)

# --- llama-server: локальные пути и параметры запуска (читает только scripts/llm-server.*; в контейнеры не передаются) ---
MV_LLM_BIN=D:\Models\llama\llama\llama-server.exe
MV_LLM_MODEL_FILE=D:\Models\unsloth\Qwen3.8\Qwen3.8-27B-UD-Q3_K_XL.gguf   # single-режим (-m)
MV_LLM_MODELS_DIR=D:\Models\         # router-режим (--models-dir, БЕЗ -m); варианты A/E+
MV_LLM_SLOT_SAVE_PATH=D:\Models\llama\llama-slots
MV_LLM_HOST=127.0.0.1                # наружу не публикуется (SEC-15)
MV_LLM_PORT=1234
MV_LLM_SLOTS=1                       # --parallel; 1 = как OLLAMA_NUM_PARALLEL=1
MV_LLM_NGL=99                        # --n-gpu-layers
MV_LLM_THREADS=32                    # --threads (i9-13900)
MV_LLM_BATCH_SIZE=16000              # --batch-size (значение из командной строки владельца)
MV_LLM_REASONING=off                 # серверный --reasoning on|off|auto; на запрос шлюз шлёт enable_thinking=false

# --- Ollama: только при MV_LLM_PROVIDER=ollama (иначе не заполнять) ---
#MV_OLLAMA_URL=http://host.docker.internal:11434  # профиль gpu: http://ollama:11434; go run: http://127.0.0.1:11434

# ===== mvctl (CLI на хосте; публикует в шину: laws bump, world init, record) =====
# те же MV_* что и у процессов, но с адресами хоста:
#   MV_KAFKA_BROKERS=127.0.0.1:19092   (external listener Redpanda, §1.4)
#   MV_MINIO_ENDPOINT=127.0.0.1:9000
#   MV_LLM_URL=http://127.0.0.1:1234
#   MV_CORE_URL=http://127.0.0.1:8090

# ===== memory (профиль memory) =====
MV_MEMORY_ADDR=:8082
MV_QDRANT_ADDR=qdrant:6334           # gRPC
MV_NEO4J_URI=neo4j://neo4j:7687
MV_NEO4J_USER=neo4j
MV_NEO4J_PASSWORD=                   # секрет; = NEO4J_PASSWORD
MV_EMBED_MODEL=nomic-embed-text      # или bge-m3 — по замеру

# ===== telegram-bot (профиль bot) =====
MV_TELEGRAM_BOT_TOKEN=               # секрет; выдаёт @BotFather; передаётся только сервису telegram-bot
MV_TELEGRAM_ALLOWED_USER_IDS=        # allowlist Telegram user id через запятую (SEC-06); пусто = бот никого не пускает
MV_TELEGRAM_GATEWAY_URL=http://gateway:8088
MV_TELEGRAM_POLL_TIMEOUT_S=25
MV_TELEGRAM_HEALTH_ADDR=:8089

# ===== legacy (профиль legacy; as-is имена, вне реестра shared/env) =====
# переменные as-is narrative-orchestrator / semantic-memory / chromadb — переносятся из as-is docker-compose.yml без изменений при F-6
```

Каждая переменная в реестре `shared/env` имеет описание; `mvctl env check` (job `contracts`, `make contracts`) проверяет: (а) каждая зарегистрированная `MV_*` есть в `.env.example`; (б) каждая `MV_*` из `.env.example` зарегистрирована; (в) переменные, помеченные `secret`, в `.env.example` пусты; (г) обязательные переменные без дефолта пусты в примере и документированы. Инфраструктурные переменные (без префикса) сверяются со списком в `shared/env/infra.go`; секция `legacy` исключена из проверки. Так закрывается NFR-074 без парсинга `os.Getenv` по исходникам — линтер `forbidigo` дополнительно гарантирует, что мимо реестра ничего не читается.

`.mcp.env.example` (для MCP-серверов IDE, не для платформы): `GITHUB_TOKEN=`, `DB_MCP_TOKEN=` — пустые, с комментарием, где получить.

**Что изменилось в v0.3 (U-8, ADR-005 доп. 2 п. 1–3):**
1. `MV_LLM_PROVIDER` по умолчанию — **`openai_compat`**; допустимые значения `openai_compat | ollama | anthropic | recorded | fake`. `openai` и `deepseek` **перестали быть именами провайдера** — это только другой `MV_LLM_URL` + `MV_LLM_API_KEY` у той же реализации, поэтому `MV_OPENAI_API_KEY` и `MV_DEEPSEEK_API_KEY` из `.env.example` **удалены** (были в v0.2). `mvctl env check` поймает их, если они останутся в чьём-то `.env`.
2. `MV_OLLAMA_URL` и блок `OLLAMA_*` — **не обязательны**: они нужны только при `MV_LLM_PROVIDER=ollama` (или при `MV_MEMORY_EMBED_PROVIDER=ollama`, ADR-005 доп. 2 п. 8). В реестре `shared/env` они объявлены с пустым дефолтом и **условно обязательны**: `shared/env` требует их наличия только когда выбран провайдер `ollama`; `mvctl env check` проверяет это же правило (в `.env.example` они закомментированы, и это не считается ошибкой — правило «обязательная переменная пуста в примере» сохраняется).
3. `MV_LLM_API_KEY` — пустой для локального llama-server (сервер запускается без `--api-key`); значение появляется только при облачном `MV_LLM_URL`. Отмечен как `secret` в реестре → в `.env.example` всегда пуст.
4. `MV_LLM_CLOUD_ENABLED=false` — гейт **по host** в `MV_LLM_URL`: loopback / `host.docker.internal` / RFC1918 считаются локальными, любой другой host требует `true`, иначе провайдер не стартует (ADR-005 доп. 2 п. 3). Смена `MV_LLM_URL` на облачный без флага — не «тихая деградация», а fatal при старте.
5. `MV_LLM_BIN`, `MV_LLM_MODEL_FILE`, `MV_LLM_MODELS_DIR`, `MV_LLM_SLOT_SAVE_PATH`, `MV_LLM_HOST/PORT/SLOTS/NGL/BATCH_SIZE/REASONING` читает **только `scripts/llm-server.*`**, платформа их не видит. Они всё равно объявляются в реестре `shared/env` (иначе `mvctl env check` посчитает их «незарегистрированными в `.env.example`») с пометкой `scope=tooling` и не участвуют в проверке обязательности процессов.
6. **CLI на хосте.** `mvctl laws bump`, `mvctl world init`, `mvctl record` публикуют/читают шину напрямую, минуя `gateway`, поэтому на стенде им нужен адрес брокера: `MV_KAFKA_BROKERS=127.0.0.1:19092` (external listener; внутри compose — `redpanda:9092`). Отдельного имени переменной для CLI **не вводим**: `mvctl` — тот же реестр `shared/env`, что и процессы. *Примечание:* в задании сведения 2 эта переменная названа `MV_KAFKA_BROKERS`; в реестре и во всех `design.md` (EPIC-001 §, EPIC-002 §, EPIC-005 §) закреплено имя **`MV_KAFKA_BROKERS`**, поэтому здесь оставлено оно — переименование потребовало бы правки `foundation.md` §8 и трёх `design.md` и должно идти отдельным решением (см. отчёт, «Для tech-lead#1»).

### 4.3. Локальный запуск без compose (`go run`)

Файл `.env.local.example` не вводим; вместо него — раздел комментариев в `.env.example` (секция `mvctl`) и цель `make run-local CONTEXTS=all` (`set -a; source .env; set +a; MV_KAFKA_BROKERS=127.0.0.1:19092 MV_MINIO_ENDPOINT=127.0.0.1:9000 MV_LLM_URL=http://127.0.0.1:1234 MV_CORE_URL=http://127.0.0.1:8090 go run ./cmd/multiverse --contexts=$(CONTEXTS)`). Те же переопределения нужны `mvctl` (`make mvctl ARGS="laws bump …"` экспортирует их же) — иначе CLI пытается достучаться до `redpanda:9092`, имени, которого на хосте нет.

### 4.4. Секреты: где живут

| Секрет | Где хранится | Кто читает | В CI |
|---|---|---|---|
| `MINIO_ROOT_USER/PASSWORD`, `MV_MINIO_*` | `.env` на машине владельца (права файла — только владелец) | compose, `minio-init`, процессы | не нужны (testcontainers генерирует свои) |
| `NEO4J_PASSWORD` / `MV_NEO4J_PASSWORD` | `.env` | compose, `memory` | не нужны |
| `MV_TELEGRAM_BOT_TOKEN` | `.env` | только `telegram-bot` (`environment:` именно этому сервису, не `env_file` всем; `compose-lint` п. 4) | нет; ротация — §9.6 |
| `MV_LLM_API_KEY` (облачный эндпоинт), `MV_ANTHROPIC_API_KEY` | `.env`; по умолчанию **пусты** — локальный `llama-server` запускается без `--api-key` и ключа не требует | `core` | нет |
| `GITHUB_TOKEN`, `DB_MCP_TOKEN` (MCP IDE) | `.mcp.env` вне git | IDE | нет |
| GitHub Secrets репозитория | для MVP-1 — **ни одного**; автоматический `GITHUB_TOKEN` с `permissions: contents: read`; `qwen-*` workflow используют свои — вне области этого документа | — | — |

Правило compose: `env_file: .env` — только для сервисов платформы (`gateway`, `core`, `memory`), и даже им лучше передавать явный список `environment:` с `${VAR}`; секрет бота — только боту; инфраструктурные пароли — только их контейнерам. Значения по умолчанию для паролей в compose **отсутствуют** (`${MINIO_ROOT_PASSWORD:?set in .env}` — compose падает с понятной ошибкой; `shared/env` — обязательные ключи → fatal при старте, SEC-14).

### 4.5. Гигиена репозитория (F-1) — точный список команд

Проверено по `git ls-files` 2026-09-09 **после F-0** (коммит `744fb10`: `git worktree prune`, 13 gitlink-записей `.claude/worktrees/*` удалены из индекса, `.gitignore += .claude/worktrees/`, `Docs/dev-team/**` зафиксированы, ветка `integration/mvp-1` создана). Шаг 0 из v0.1 закрыт; `git status` работает. Команды ниже **перечислены, не выполнены**; выполняются разработчиком в ветке `epic/EPIC-001-foundation` из Git Bash (для PowerShell даны эквиваленты там, где синтаксис отличается). Порядок важен: сначала защита (п. 1–2), потом правки индекса.

1. **`.gitleaks.toml`** (T-16, ADR-009 доп. п. 10) — создать первым, до любых других изменений индекса:
   ```toml
   # .gitleaks.toml — allowlist ТОЛЬКО шаблоны; README, testdata/, Docs/ не исключаются
   [extend]
   useDefault = true

   [[allowlists]]
   description = "example files contain only empty values and placeholders"
   paths = ['''^\.env\.example$''', '''^\.mcp\.env\.example$''']

   [[allowlists]]
   description = "obvious placeholders"
   regexes = ['''sk-x{8,}''', '''<your-key>''', '''123456789:AA[x]{33}''']
   ```
   Точечные ложные срабатывания (например, усечённое `sk-4659b9…` в `Docs/dev-team/architecture/threat-model.md`) — inline `# gitleaks:allow` или fingerprint в `.gitleaksignore`, не расширение allowlist.
2. **Pre-commit** (§3.4): `.pre-commit-config.yaml` → `pre-commit install` → `pre-commit run --all-files` (первый прогон покажет текущие находки gitleaks — ожидаемо ключ из п. 3).
3. **Плейсхолдер в `shared/oracle/README.md`** (U-5; ключ реальный, отзывает пользователь; до замены — скомпрометирован): строки **14, 29, 68** содержат `sk-4659b9…`. Заменить значение на `sk-xxxxxxxxxxxxxxxxxxxxxxxx` (строки 14, 29) и `export ORACLE_API_KEY="<your-key>"` (строка 68). Проверка: `grep -n "sk-4659" shared/oracle/README.md` → пусто. (Файл затем уходит в `services/_archive/shared/oracle/` по F-3 — плейсхолдер ставится до перемещения, чтобы история `git mv` не тянула ключ дальше.)
4. **Секреты и локальные настройки из индекса** (файлы остаются на диске):
   ```bash
   git rm --cached .mcp.env .claude/settings.local.json
   ```
   `.env` в индексе нет (проверено). `.claude/settings.local.json` — локальные разрешения IDE (сейчас изменён в рабочем дереве), в индексе ему не место.
5. **Бинарники и логи из индекса** (файлы удаляются и с диска — они воспроизводимы):
   ```bash
   git rm --cached examples.exe semantic-memory.exe services/narrative-orchestrator/cmd.exe mcp_kafka.log mcp_audit.log
   rm -f examples.exe semantic-memory.exe services/narrative-orchestrator/cmd.exe mcp_kafka.log mcp_audit.log
   ```
   `shared/eventbus/docs/event-model.pdf` (3,9 МБ) — не секрет и не код; решение tech-writer (F-9): оставить или `git rm --cached` со ссылкой на коммит. По умолчанию — оставить (U-1: ничего не удаляем без решения).
6. **Пустые каталоги на диске** (в индексе их нет): `-p/` и `Multiverse/`. Git Bash: `ls -A -- -p Multiverse` (убедиться, что пусто) → `rm -rf -- -p Multiverse` (обязательно `--` перед `-p`). PowerShell: `Remove-Item -LiteralPath '-p','Multiverse' -Recurse -Force`.
7. **Код — не удалять, а перемещать (F-3, §4.6)**: `test_minio.go`, `fake_deps/`, корневой `Dockerfile`, `services/{entity-actor,evolution-watcher,rule-engine}/Dockerfile` — **`git mv` в `services/_archive/`** по §4.6, не `git rm`. В F-1 они не трогаются.
8. **`.gitignore`** — целевой (дополняет текущий: `.env`, `.qwen/`, `gha-creds-*.json`, `.claude/worktrees/`, `Docs/dev-team/.dashboard.port`):
   ```gitignore
   # secrets & local config
   .env
   .env.*
   !.env.example
   .mcp.env
   !.mcp.env.example
   .claude/settings.local.json
   .claude/worktrees/
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
   .qwen/
   gha-creds-*.json
   Docs/dev-team/.dashboard.port
   # OS
   .DS_Store
   Thumbs.db
   ```
   **Решение владельца (U-11, вопрос 5 закрыт): уже отслеживаемые `.idea`, `.vscode`, `.kilo*`, `.roo*`, `.qwen`, `memory/`, `plans/`, `reports/` и `shared/eventbus/docs/event-model.pdf` из индекса в F-1 НЕ выводятся** — они могут быть ценны как инструкции AI-агентам; решение принимает tech-writer в **F-9**. Практическое следствие для F-1: правила `.gitignore` выше касаются только **новых** файлов (Git не перестаёт отслеживать уже добавленные), `git rm --cached` по этим путям в F-1 **не выполняется**, и критерий F-1 их не проверяет. Исключение остаётся за секретами и бинарниками (п. 3–5) — они выводятся из индекса независимо.
9. **`.mcp.env.example`** — создать (§4.2).
10. **`.gitattributes`** — заменить текущий `* text=auto` на:
    ```gitattributes
    * text=auto
    *.sh text eol=lf
    *.yml text eol=lf
    *.yaml text eol=lf
    *.toml text eol=lf
    Makefile text eol=lf
    *.exe binary
    *.pdf binary
    testdata/recordings/*.jsonl merge=binary
    ```
    `merge=binary` **без `-diff`** (D-14, ADR-010 доп. п. 2): текстовый diff записей сохраняется для ревью нарративов, конфликты решаются перегенерацией. `eol=lf` — иначе `autocrlf` на Windows портит скрипты для контейнеров.
11. **`.env.example`** — по §4.2 (полный; `mvctl env check` сверит после F-5).
12. **Проверка** (критерий F-1): `gitleaks dir --redact .` → 0 находок; `gitleaks git --redact --log-opts="integration/mvp-1..HEAD" .` → 0; `git ls-files | grep -E '\.(exe|log)$|\.mcp\.env$|settings\.local\.json$'` → пусто; `git status` работает; `pre-commit run --all-files` зелёный; тест хука на Windows (§3.4).
13. **Очистка истории** (`git filter-repo --path .mcp.env --path shared/oracle/README.md --invert-paths` + удаление `.exe`) — отдельная задача, **только по явной команде владельца** (ADR-009 п. 8, OQ-A-11); до неё токены считаются скомпрометированными и отозванными. После filter-repo все клоны и 40+ веток (`claude/*`, `gm-*`) переклонируются/удаляются — ветки заморозить до этого (ADR-001).

### 4.6. Архив кода вместо удаления (F-3; U-1, ADR-001 доп. п. 5)

Структура: `services/_archive/<исходный путь относительно корня>/` — путь сохраняется целиком, чтобы `git log --follow` и ссылки из документов оставались читаемыми.

```
services/_archive/
├── README.md                          # индекс: путь → причина → коммит → эпик возврата
├── go.mod                             # module multiverse-core.io/archive, без require — чтобы ./... корня не видел файлы архива
├── services/ban-of-world/             # + ARCHIVED.md, свой go.mod (не собирается)
├── services/reality-monitor/
├── shared/{schema,redis,config,minio,oracle,rules,intent,tinyml,spatial}/
├── shared/agent/tools/*               # кроме реестра инструментов
├── fake_deps/
├── test_minio.go
├── build/Dockerfile.root              # корневой Dockerfile as-is
├── build/services/{entity-actor,evolution-watcher,rule-engine}/Dockerfile
└── configs/gm_*.yaml                  # профили GM as-is (overview §16)
```

- Команда перемещения: `git mv <путь> services/_archive/<путь>` (для файлов в корне — `git mv test_minio.go services/_archive/test_minio.go`); после перемещения `go build ./...` в корне не должен видеть архив (нет `go.work`; каталоги с собственным `go.mod` для корневого модуля невидимы).
- `ARCHIVED.md` в каждом каталоге (шаблон): `Причина` (ссылка на ADR/overview §16), `Коммит архивации`, `Последний рабочий коммит`, `Эпик возврата` (если есть; например ban-of-world → «нет», `shared/oracle` → «нет»), `Что использовало`.
- Восемь замороженных сервисов остаются на месте с `FROZEN.md` (заморозка, не архив); narrative-orchestrator и semantic-memory среди них — они нужны профилю `legacy` до S5, затем уходят в `_archive` (EPIC-003 I2).
- `_archive` исключён из `.golangci.yml`, `.dockerignore`, Makefile, compose, CODEOWNERS (не требует ревью); из `gitleaks` — **не** исключён.
- `make archive-legacy` (данные, не код) — §5.5.

---

## 5. Данные: инфраструктура, миграции, бэкапы

### 5.1. Redpanda (ADR-007 п. 4–6, доп. п. 1–2)

Запуск: `redpanda start --mode dev-container --smp 1 --memory 1G --overprovisioned --node-id 0 --kafka-addr internal://0.0.0.0:9092,external://0.0.0.0:19092 --advertise-kafka-addr internal://redpanda:9092,external://localhost:19092 --rpc-addr redpanda:33145 --advertise-rpc-addr redpanda:33145`. Healthcheck контейнера: `rpk cluster health | grep -q 'Healthy:.*true'` (`interval: 5s`, `start_period: 20s`); `redpanda-init` объявляет `depends_on: redpanda: condition: service_healthy` вместо as-is `sleep 10`, а сервисы платформы — `depends_on: redpanda-init: condition: service_completed_successfully`.

`redpanda-init` (образ Redpanda из `versions.env`, скрипт `build/redpanda-init.sh`, идемпотентен, порты не публикует):

| Топик | `retention.ms` | Замечания |
|---|---|---|
| `player_events` | `2592000000` (30 дн.) | |
| `game_events` | `2592000000` | |
| `world_events` | `2592000000` | |
| `system_events` | `2592000000` | |
| `narrative_output` | `2592000000` | |
| `llm_records` | `7776000000` (90 дн.) | тяжёлые payload; `max.message.bytes=4194304` (4 МБ) |
| `analytics_events` | `15552000000` (180 дн.) | не читается в replay |
| `dead_letters` | `2592000000` | |

Для всех: `-p 1 -r 1 -c cleanup.policy=delete -c segment.ms=86400000` (D-9: без суточного сегмента retention при малом трафике не срабатывает). Скрипт: `rpk topic create "$t" -p 1 -r 1 -c ... || rpk topic alter-config "$t" --set retention.ms=... --set segment.ms=...` — повторный запуск обновляет конфиг. `scope_management` и фантомные топики не создаются; Schema Registry не используется. Retention 30/90/180 — решение пользователя U-3 (OQ-A-19).

Оценка диска: доменные топики ≈ 2 МБ/сессия × 8 сессий/мес ≈ 16 МБ/мес; `llm_records` ≈ 1 МБ/сессия; фон ≈ 24 тика/сут × 4 КБ ≈ 0,1 МБ/сут. Итого < 200 МБ за retention-окно.

Consumer-группы: `{process}.{context}` (например `core.state`, `memory.indexer`); reader `MinBytes=1, MaxWait=100ms`; при смене раскладки контекстов группы переименовываются → офсеты теряются → State стартует со снапшота с курсором через `Journal` (C-01 v1.1, C-14), поэтому это безопасно.

### 5.2. MinIO (ADR-004 п. 1, доп. п. 1–3; ADR-021)

- Образ — собственная сборка (§2.5). `shared/objstore.Client`: `Put/Get/Stat/List/Delete/EnsureBucket/Capabilities` — и ничего сверх (ADR-021 п. 2); реализации `minio` (над `minio-go/v7`) и `memory` (тесты). Ни один путь кода не зависит от bucket versioning (откат — ротация снапшотов K=5, `history` сущности).
- Бакеты: `entities-{world}`, `snapshots-{world}` (versioning **on**), **`prompts-{world}` (versioning off)**, `ops-artifacts` (versioning off).
- **Versioning и ILM включает код** `objstore.EnsureBucket(ctx, name, opts{Versioned, NoncurrentExpireDays, ExpireDays})` при создании бакета (`mvctl world init`), если `Capabilities().Versioning/Lifecycle` — иначе предупреждение в `/health.store`, не ошибка (D-6, ADR-004 доп. п. 2). Значения: `entities-*`, `snapshots-*` — `Versioned=true, NoncurrentExpireDays=30`; **`prompts-*` — `Versioned=false, ExpireDays=30`** (T-13, SEC-22; было 90); `ops-artifacts` — без правил.
- `minio-init` (`mc` из `versions.env`) — только страховка и сервисный пользователь: `mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"`; `mc mb --ignore-existing local/ops-artifacts`; `mc admin user add local "$MV_MINIO_ACCESS_KEY" "$MV_MINIO_SECRET_KEY"` + политика `readwrite` (в `prod` платформа не ходит под root-ключом); для уже существующих `entities-*`/`snapshots-*` — `mc version enable` + `mc ilm rule add --noncurrent-expire-days 30`; для `prompts-*` — `mc ilm rule add --expire-days 30` (версионирование не включать). Идемпотентно при каждом `make up`. Проверка SEC-22 — тест в `integration`: после `EnsureBucket("prompts-x")` `mc version info` = `Un-versioned`, ILM = 30 дн.
- Данные as-is (`entities-{world}` с плоским payload, `schemas`, `gnue-configs`, `rules`, `models`) **не мигрируются** (overview §19): MVP-1 стартует с `mvctl world init`. Перед `make reset` — `make archive-legacy` (§5.5).

### 5.3. Qdrant и Neo4j (профиль `memory`)

- Qdrant `v1.19.1`, том `qdrant_storage`; коллекции `events-{world}`, `entities-{world}` создаёт `internal/memory` при старте; API-ключ не задаём (порт только на loopback; при выходе за одну машину — `QDRANT__SERVICE__API_KEY`, SEC-31). Healthcheck: образ без `curl`/`wget` — `bash -c 'exec 3<>/dev/tcp/127.0.0.1/6333'` (в образе есть bash); проверить при реализации.
- Neo4j `5.26.30-community`: `NEO4J_AUTH=neo4j/${NEO4J_PASSWORD:?}`, `NEO4J_server_memory_heap_max__size=1G`, `NEO4J_server_memory_pagecache_size=512M`, **без `NEO4J_PLUGINS`** — APOC не включается: as-is код `semantic-memory` его не использует (T-17, ADR-004 доп. п. 5, SEC-32); если EPIC-005 понадобятся APOC-процедуры — только с `apoc.import.file.enabled=false`/`apoc.export.file.enabled=false`. Том `neo4j_data`. Healthcheck `wget -qO- http://127.0.0.1:7474` (в образе есть). Constraints/indexes создаются кодом при старте — единственная «миграция» Neo4j, идемпотентная. Порт 7474 (Browser) публикуется только в профиле `dev` (SEC-33).
- Оба хранилища — **производные** (перестраиваются `mvctl memory rebuild` из журнала за retention-окно + снапшота). Бэкап не делаем; при потере — rebuild (§9.4). Ограничение: события старше retention (30 дн.) в индекс не вернутся — приемлемо для Should.

### 5.4. SQLite в gateway (ADR-004 п. 2, доп. п. 4; ADR-009; ADR-019)

- Файлы `/data/links.db`, `/data/gateway.db` в **именованном томе `gateway-data`** (не bind-mount; T-8); процесс — `nonroot` (uid 65532); при создании файлы `0600`; том не монтируется другим сервисам. `PRAGMA journal_mode=WAL; synchronous=NORMAL; busy_timeout=5000; foreign_keys=ON`; для `links.db` дополнительно **`secure_delete=ON`, `auto_vacuum=INCREMENTAL`**.
- Миграции — **`pressly/goose/v3` как библиотека** (ADR-019; `golang-migrate` из v0.1 снят): `internal/gateway/migrations/links/NNNN_*.sql`, `…/migrations/gateway/NNNN_*.sql` (embed), `goose.NewProvider(DialectSQLite3, db, fsys).Up(ctx)` при старте контекста, таблица версий в каждом файле. Правила: нумерация сквозная внутри цепочки, одна миграция на PR, применённую не редактировать; владелец — только EPIC-004 (`ownership.md`), конфликтов нумерации между эпиками нет. **`Down` не пишутся** — откат = восстановление файла из предрелизного бэкапа (§8). Интеграционный тест «`Up` на пустой БД + `Up` повторно = no-op» обязателен для каждой миграции.
- `/forget` = `DELETE` → **сразу** `PRAGMA wal_checkpoint(TRUNCATE)` + `PRAGMA incremental_vacuum` в той же операции (ADR-004 доп. п. 4); sweeper раз в час — страховка. Тест: после `/forget` `strings links.db links.db-wal` не содержит id (SEC-04).
- Бэкап — §5.6; в образе нет `sqlite3`, онлайн-копия — подкоманда `multiverse db backup --out /data/backup/<name>.db` (`VACUUM INTO`) — задача EPIC-004 (каркас подкоманды — EPIC-001).

### 5.5. Миграция от as-is (что нужно, что нет)

| Хранилище | Действие при переходе на EPIC-001 | Причина |
|---|---|---|
| Redpanda том (v24.2.5) | **пересоздать** (`docker volume rm multiverse_redpanda_data`) после `make archive-legacy` | обновление 24.2 → 26.1 одним шагом не поддерживается (D-8, ADR-004 доп. п. 6); журнал as-is не нужен |
| MinIO том | `make archive-legacy` → пересоздать (новый образ; данные не мигрируются, §19) | ADR-021; versioning включается на новых бакетах кодом |
| Chroma том | **оставить** до конца профиля `legacy` (S5); удалить вместе с профилем в EPIC-003 I2 | D-3 |
| TimescaleDB, Redis | тома удалить; сервисы вне compose | ADR-004 |
| Neo4j 5.18 → 5.26.30 | пересоздать том (граф перестраивается) | данные производные |
| Qdrant `latest` → 1.19.1 | пересоздать | клиентов не было, данных нет |

`make archive-legacy` (ADR-004 доп. п. 6; однократно, вне git): `docker compose down` → `docker run --rm -v multiverse_minio_data:/src:ro -v "$PWD/backups/legacy-<date>":/dst alpine:3.22 tar czf /dst/minio-as-is.tgz -C /src .` → то же для `multiverse_redpanda_data` → `sha256sum` в `backups/legacy-<date>/SHA256SUMS`. Одноразовый скрипт `scripts/migrate-from-as-is.sh`: `make archive-legacy` → удаление перечисленных томов → `make up` → `mvctl world init`.

### 5.6. Бэкапы и восстановление

| Что | Как | Периодичность | Хранение | Проверка restore |
|---|---|---|---|---|
| Том `minio_data` (истина: сущности + снапшоты; `prompts-*` **исключён**, SEC-22) | `make backup` → остановить `core` → `docker run --rm -v multiverse_minio_data:/src:ro -v "$BACKUP_DIR":/dst alpine:3.22 tar czf /dst/minio-<date>.tgz --exclude='prompts-*' -C /src .` → запустить `core`; альтернатива без остановки — `mc mirror --overwrite --exclude 'prompts-*/**' local/ ./backups/minio-<date>/` | перед каждым `make deploy`; еженедельно (Task Scheduler → `pwsh scripts/backup.ps1`) | `%USERPROFILE%\multiverse-backups\` (вне репозитория и Docker-томов; OneDrive `Documents` не синхронизируется — U-6, отдельного требования нет), последние 4 еженедельных + все предрелизные за 30 дней | ежемесячно: `make restore FILE=… TARGET=scratch` в отдельный compose-проект (`COMPOSE_PROJECT_NAME=mv-restore`) → `mvctl session-report --audit` → `state_hash` совпадает со снапшотом |
| Том `redpanda_data` (журнал после снапшота) | тот же `tar` при остановленном `redpanda` | вместе с MinIO | там же | восстановление вместе с MinIO; после старта `core` — `analytics.replay.completed identical=true` |
| `links.db` (ПДн; SEC-05) | `docker compose exec gateway /multiverse db backup --out /data/backup/links-<date>.db` → `docker cp` на хост → `age -r "$MV_BACKUP_AGE_RECIPIENT" -o links-<date>.db.age` → удалить открытую копию (на хосте и в томе) | вместе с остальным (объём ≤ 10 записей — можно после каждого изменения) | **отдельный каталог** `%USERPROFILE%\multiverse-backups\links\`; **≤ 30 дней**, скрипт чистит старше; предрелизная копия — основа отката миграции (ADR-019: `Down` нет) | ежеквартально (T-16 п. 5): `age -d` → `multiverse db check` (`PRAGMA integrity_check`) |
| `gateway.db` (сессии, outbox) | тем же `db backup` без шифрования | вместе | там же | как выше |
| Qdrant, Neo4j | не бэкапятся — `mvctl memory rebuild` | — | — | rebuild — runbook §9.4 |
| Модели LLM (`.gguf` в `MV_LLM_MODELS_DIR`; модели Ollama) | не бэкапятся — скачиваются заново (`ops/models.txt`; для Ollama — `make models`). Единственное, что фиксируется в Git, — **имя** модели (`LLM_MODEL_DEFAULT`) и билд (`LLAMACPP_BUILD`) | — | — | — |
| `ops/metrics/*.csv`, блупринты, правила, законы, записи LLM | в Git | каждый коммит | GitHub | — |

Ключ `age` (пара) генерируется владельцем один раз (`age-keygen`; `winget install FiloSottile.age`), приватный ключ хранится вне машины проекта (менеджер паролей); в `.env` — только `MV_BACKUP_AGE_RECIPIENT` (публичный ключ, не секрет). Полное восстановление на чистой машине: §9.4.

RPO/RTO (NFR-010/011): RPO для подтверждённых событий = 0 при живых томах; при потере диска RPO = возраст последнего бэкапа (≤ 7 дней при еженедельном — приемлемо для MVP-1); RTO ≤ 2 мин при рестарте, ≈ 15 мин при восстановлении из бэкапа (замер — B6). Чек-лист оператора по SEC-30: BitLocker включён; `.env` и `backups/` — права только владельцу; отдельная учётная запись ОС для служб — по возможности.

---

## 6. LLM-рантайм: нативный `llama-server` (llama.cpp)

Решение пользователя **U-8** (`journal.md` 2026-09-09, `consolidation.md` §10) и **ADR-005 дополнение 2**: основной рантайм — llama.cpp `llama-server`, запущенный **нативно на машине владельца**, провайдер `openai_compat`. Ollama остаётся установленной и используется как **опциональный второй рантайм** (§6.5). Рекомендация v0.2 «Ollama нативно» отменена; §13 вопрос 1 закрыт.

### 6.1. Размещение: нативный процесс вне compose

| Критерий | **`llama-server` нативно (принято)** | Ollama нативно / контейнер (опция) |
|---|---|---|
| GPU | CUDA напрямую, весь VRAM 24 ГБ; `--n-gpu-layers 99` кладёт все слои на GPU | нативно — так же; в контейнере — через WSL2-паравиртуализацию |
| Загрузка модели | при старте процесса, `--load-mode mmap+mlock` — веса резидентны, выгрузки нет; `keep_alive` не существует и не нужен | `OLLAMA_KEEP_ALIVE=-1` + прогрев после каждого рестарта Docker |
| API | OpenAI-совместимый: `POST /v1/chat/completions` (`response_format: json_schema` → grammar-sampling), `POST /v1/embeddings`, `GET /v1/models`, `GET /health`, `GET /props` | native `/api/chat`, `/api/generate`, `/api/ps` (+ свой OpenAI-совместимый слой) |
| Управление версией | бинарник ставится вручную, автообновления нет → **пин билда честный** (`LLAMACPP_BUILD` в `build/versions.env`, `llama-server --version`) | приложение автообновляется — пин требует отдельных усилий (риск v0.2) |
| Модели | файлы `.gguf` в `MV_LLM_MODELS_DIR` на NTFS; смена модели = смена аргумента запуска | реестр Ollama, `ollama pull` |
| Управление процессом | **вне compose**: `make llm-up` / `make llm-down` / `make llm-health` (ADR-005 доп. 2 п. 7). `make up` процесс **не поднимает** | `make up` управляет только контейнерным вариантом |
| Доступ из контейнеров | `http://host.docker.internal:1234` | `http://host.docker.internal:11434` или `http://ollama:11434` |
| Доступ с хоста | `http://127.0.0.1:1234` (`go run`, `mvctl`, `scripts/llm-bench.ps1`) | `http://127.0.0.1:11434` |
| CI | не участвует (GPU нет; `providers/recorded` + `providers/fake`) | не участвует |

Почему процесс **не** заводится сервисом compose: (а) он нативный Windows-процесс с GPU и путями `D:\Models\…` — контейнеризация означала бы WSL2 и повторную настройку CUDA без выигрыша; (б) его жизненный цикл длиннее цикла стека — модель грузится 20–60 с, а `make down`/`make reset` не должны её выгружать; (в) compose-инвариант «все порты на `127.0.0.1`, паролей по умолчанию нет» проще держать, когда LLM вне файла. Служба Windows / Task Scheduler — **не в MVP-1** (один оператор, ручной старт по runbook §9.3); переход к службе — при появлении внешних тестеров (E-H).

Деградация: если `llama-server` не запущен, `core` стартует и работает — `/health.llm=unavailable`, нарратив по шаблонам (FR-080/NFR-072). `make up` печатает предупреждение, но не падает.

### 6.2. `scripts/llm-server.ps1` — запуск с зафиксированными параметрами (F-6)

Основной скрипт — PowerShell (llama-server запускается нативно на Windows); `scripts/llm-server.sh` — тот же набор аргументов для WSL/Linux-сборки, используется только при переносе стенда. Значения берутся из `.env` (`MV_LLM_*`, §4.2), в скрипте не зашиты.

```powershell
#Requires -Version 7
# scripts/llm-server.ps1 — старт нативного llama-server (llama.cpp), ADR-005 доп. 2 п. 7
param(
  [switch]$Router,                                        # router-режим: --models-dir БЕЗ -m (§6.2.1)
  [string]$Model   = $env:MV_LLM_MODEL_FILE,
  [int]   $Ctx     = [int]($env:MV_LLM_NUM_CTX     ?? 8192),
  [int]   $Slots   = [int]($env:MV_LLM_SLOTS       ?? 1),
  [string]$Reason  =       ($env:MV_LLM_REASONING  ?? 'off'),   # серверный дефолт; на запрос решает шлюз
  [switch]$WithUi                                          # веб-UI/MCP-прокси — только по явному требованию оператора
)
$ErrorActionPreference = 'Stop'
$bin  = $env:MV_LLM_BIN
$host_ = ($env:MV_LLM_HOST ?? '127.0.0.1')                 # ТОЛЬКО loopback (SEC-15)
$port  = [int]($env:MV_LLM_PORT ?? 1234)

$args = @(
  '--host', $host_, '--port', $port,
  '--ctx-size', ($Ctx * $Slots),        # llama.cpp делит --ctx-size между слотами
  '--parallel', $Slots,
  '--n-gpu-layers', ($env:MV_LLM_NGL ?? 99),
  '--threads',      ($env:MV_LLM_THREADS ?? 32),
  '--batch-size',   ($env:MV_LLM_BATCH_SIZE ?? 16000),
  '--flash-attn', 'on',
  '--kv-offload', '--kv-unified',
  '--jinja',                             # обязателен: без него не работает шаблон tools/thinking Qwen3
  '--reasoning', $Reason,
  '--slot-save-path', $env:MV_LLM_SLOT_SAVE_PATH,
  '--load-mode', 'mmap+mlock',
  '--no-webui'                           # снимается флагом -WithUi
)
if ($Router) { $args += @('--models-dir', $env:MV_LLM_MODELS_DIR, '--models-max', 2) }
else         { $args += @('-m', $Model) }
if ($WithUi) { $args = $args | Where-Object { $_ -ne '--no-webui' }; $args += @('--tools', 'all', '--ui-mcp-proxy') }

Start-Process -FilePath $bin -ArgumentList $args -PassThru |
  ForEach-Object { $_.Id | Set-Content ops/llm-server.pid }
# ожидание готовности: /health 503 (loading) → 200 (ok), до 180 с
```

**Что зафиксировано и почему — по сравнению с командной строкой владельца** (`journal.md`, «Решения пользователя (DevOps)»):

| Аргумент владельца | В скрипте | Обоснование |
|---|---|---|
| `--host 127.0.0.1 --port 1234` | **да**, из `MV_LLM_HOST`/`MV_LLM_PORT` | только loopback; наружу порт не публикуется (SEC-15, §1.4) |
| `--n-gpu-layers 99` | да (`MV_LLM_NGL`) | вся модель на GPU; вариант E укладывается в 24 ГБ (13,1 ГБ весов + KV) |
| `--batch-size 16000`, `--threads 32` | да (`MV_LLM_BATCH_SIZE`, `MV_LLM_THREADS`) | значения владельца сохранены как дефолты; влияют на prompt-фазу, участвуют в замере |
| `--flash-attn on`, `--kv-offload`, `--kv-unified` | да | `--kv-unified` + `--parallel 1` = один общий KV-кэш, аналог `OLLAMA_NUM_PARALLEL=1`; приоритет интерактива перед фоном обеспечивает планировщик роя (ADR-014), а не сервер |
| `--jinja` | да | нужен для chat-template Qwen3 (thinking/tools); без него `chat_template_kwargs` игнорируется |
| `--slot-save-path …` | да | кэш слотов на диске; шлюзом не используется, но ускоряет ручные эксперименты (§6.3) |
| `--load-mode mmap+mlock` | да | веса резидентны, нет подкачки на первом запросе |
| `-m <model>.gguf` | да — **но только в single-режиме** | см. §6.2.1: `-m` и `--models-dir` взаимоисключающи |
| `--models-dir D:\Models\` | **только с `-Router`, без `-m`** | при заданном `-m` сервер в single-model режиме, `--models-dir` игнорируется и поле `model` в запросе не действует (ADR-005 доп. 2 п. 4). В командной строке владельца заданы оба — фактически работает `-m` |
| `--temp 1 --min-p 0.01 --top-p 0.95 --top-k 20` | **НЕТ** | семплинг задаёт **шлюз на каждый запрос** из блупринта фазы (ADR-005 доп. 2 п. 1). Серверные значения — thinking-профиль Qwen3; для MVP-1 (thinking off) нужен non-thinking-профиль `temp 0.7 / top_p 0.8 / top_k 20 / min_p 0 / presence_penalty 1.5`. Держать два набора дефолтов в двух местах — источник расхождений: серверные значения молча применились бы к запросам, где шлюз не передал параметр |
| `--reasoning on` | **дефолт `off`** (`MV_LLM_REASONING`), управление — **на запрос** | `chat_template_kwargs: {"enable_thinking": false}` в каждом запросе со схемой. Причина: llama.cpp **#20345** — при включённом thinking грамматика `json_schema` не применяется, ответ может прийти в markdown-обёртке. Серверный `off` — вторая линия защиты; для ручных экспериментов с рассуждениями — `-Reason on` |
| `--tools all`, `--ui-mcp-proxy` | **НЕТ по умолчанию**, только `-WithUi` | шлюз не использует ни tool-calling сервера, ни MCP-прокси (инструменты — на стороне роя). Это дополнительная поверхность атаки: веб-UI и MCP-прокси доступны любому процессу на машине. При включении — **только** `--host 127.0.0.1`, порт наружу не публиковать, в файрвол не открывать (SEC-15) |
| — | **`--no-webui`** добавлен | по умолчанию сервер отдаёт веб-интерфейс на том же порту; для служебного рантайма он не нужен |
| — | **`--ctx-size` = `MV_LLM_NUM_CTX` × `MV_LLM_SLOTS`** | llama.cpp делит `--ctx-size` между слотами; при `--parallel 1` это просто `MV_LLM_NUM_CTX` (8192 базово, 16384 — вариант замера) |
| — | `--api-key` **не задаём** | локальный loopback-сервер; ключ появляется только у облачного эндпоинта (`MV_LLM_API_KEY`, §4.2) |

Пин версии: `llama-server --version` печатает `build: <N> (<sha>)`; номер вносится в `build/versions.env` как `LLAMACPP_BUILD` при F-6 и печатается `make llm-health` — расхождение пина и факта = предупреждение (не ошибка) с ссылкой на §9.5 и риск §12.

#### 6.2.1. Два режима: single (`-m`) и router (`--models-dir`)

| | **single** (`make llm-up`) | **router** (`make llm-up ROUTER=1` → `-Router`) |
|---|---|---|
| Аргументы | `-m <файл>.gguf`, **без** `--models-dir` | `--models-dir <каталог>`, `--models-max 2`, **без** `-m` |
| `GET /v1/models` | одно имя — загруженная модель | имена (stem) всех `.gguf` в каталоге; загруженные — по `POST /models/load` / `POST /models/unload` и по первому запросу |
| Поле `model` в запросе | **игнорируется** | выбирает модель; блупринт задаёт имя (C-11 правило 7а сверяет со списком провайдера) |
| Когда нужен | вариант **E** — одна `Qwen3.8-27B-UD-Q3_K_XL` на тики и нарратив (базовый кандидат) | варианты **E+** и **A** — разные модели на тики и нарратив; эмбеддинг-модель рядом с генеративной (ADR-005 доп. 2 п. 8) |
| Риск | нет | обе модели должны помещаться в VRAM одновременно (`--models-max 2`), иначе переключение = выгрузка/загрузка с задержкой в секунды |

Итоговый режим фиксируется по результатам F-8 в `ops/metrics/baseline.md` (architect#1) и в дефолте `make llm-up`.

### 6.3. Health, прогрев, слоты, доступ из контейнеров

**Health (NFR-030, NFR-016).** Два эндпоинта, оба без авторизации на loopback:
- `GET /health` → `200 {"status":"ok"}`; **`503` во время загрузки модели** (`{"error":{"code":503,"message":"Loading model"}}`) — это `loading`, **не** `unavailable`: провайдер `openai_compat` отражает его как `llm=loading` в `/health.deps` `core` (ADR-005 доп. 2 п. 1), `make llm-up` ждёт `200` до 180 с;
- `GET /v1/models` → список имён; `make llm-health` и `/health` провайдера сверяют список с моделью блупринта: нет → `degraded(model_not_resident)`.

`make health` вызывает `make llm-health` и печатает: код `/health`, имена из `/v1/models`, `LLAMACPP_BUILD` (факт vs пин), `nvidia-smi --query-gpu=memory.used,memory.total`. `make up` **только проверяет** `GET $MV_LLM_URL/health` и при не-`200` печатает `warning: LLM недоступен, нарратив деградирует (FR-080); подними процесс: make llm-up` — код возврата `make up` при этом нулевой.

**Прогрев.** Классический прогрев (как `OLLAMA_KEEP_ALIVE=-1` + пустой `generate`) не нужен: `--load-mode mmap+mlock` держит веса в памяти с момента старта, выгрузки по таймауту нет. Но **первый запрос после старта** дороже остальных (аллокация compute-буферов, построение графа, прогрев CUDA-ядер), поэтому `make llm-up` после `/health=200` делает **один короткий `POST /v1/chat/completions`** (`max_tokens: 1`, `enable_thinking: false`) и печатает его латентность — она же служит грубым индикатором «сервер здоров». В `make bench` эта же величина измеряется отдельно как `load_ms`/`first_call_ms` (B2, холодный старт) и в p50/p95 не входит.

**Слоты и кэш промпта.** `--slot-save-path` включает сохранение состояния слотов на диск; шлюзом **не используется** (ADR-005 доп. 2 п. 6). Кэш общего префикса работает автоматически — в ответе видно `usage.prompt_tokens_details.cached_tokens`; поскольку системная часть промпта агента стабильна, доля cached-токенов — полезная метрика замера (§6.4 CSV). Каталог `MV_LLM_SLOT_SAVE_PATH` не бэкапится и может быть удалён в любой момент.

**Доступ из контейнеров.** `core` ходит на `http://host.docker.internal:1234`; в compose у сервисов платформы прописан `extra_hosts: ["host.docker.internal:host-gateway"]`. Проверка при первом запуске (§9.1): `docker run --rm --add-host host.docker.internal:host-gateway curlimages/curl:8.10.1 -s -o /dev/null -w '%{http_code}' http://host.docker.internal:1234/health` → `200`. Если не проходит — **не** переводить сервер на `0.0.0.0`, а сначала проверить правило Windows Firewall для `llama-server.exe` (входящие с подсети Docker `172.16.0.0/12`); вариант `--host 0.0.0.0` допустим **только** вместе с правилом файрвола, ограничивающим источник подсетью Docker, и остаётся нежелательным (SEC-15).

**Безопасность (SEC-15).** Сервер слушает `127.0.0.1`, порт compose не публикует, `--api-key` локально не нужен. Веб-UI выключен (`--no-webui`), `--tools all` и `--ui-mcp-proxy` по умолчанию не передаются; если оператор включает их для ручной работы (`-WithUi`), это допустимо **только** на loopback и без публикации порта — MCP-прокси даёт доступ к инструментам от имени пользователя. Промпты и ответы сервер пишет в свой лог при `--verbose` — в служебном режиме `--verbose` не включаем (ПДн в промптах нет, но объём и SEC-02).

### 6.4. Матрица замера как скрипт (F-8; OQ-A-18 уточнён U-8)

**Цель замера:** решить, проходит ли **вариант E** — одна `Qwen3.8-27B UD-Q3_K_XL` на llama-server на нарратив **и** тики — пороги **NFR-002** (нарратив Phase 2: p95 ≤ 5 с от `combat.decided` до доставки при одном активном scope; отдельная строка — группа из 3, один нарратив на раунд) и **NFR-090** (русский язык: 0 CJK-иероглифов, латиница ≤ 10 % символов, ≥ 98 % ответов). **Критерий выбора базовой конфигурации: E → если не проходит, C → если не проходит, A** (ADR-005 доп. 2 п. 5, `overview.md` §18.1). E+ (27B + 8B в router-режиме), B и D — только материал. Дополнительно фиксируются B2 (холодный старт), B3 (`valid_json_ratio` с грамматикой llama.cpp — NFR-022), VRAM/RAM-размещение (NFR-076), доля `cached_tokens`.

**Файлы:** `scripts/llm-bench.ps1` (основной — машина владельца, Windows; без `jq`/`bc`), `scripts/llm-bench.sh` (WSL/Linux, тот же CSV), `ops/metrics/bench-matrix.json` (конфигурации и пороги — правит архитектор, скрипты не меняются), `testdata/bench/prompts.jsonl` (10 ситуаций «Тёмного леса» × фазы `tick`/`phase2`/`phase2-group3`, схемы из `schemas/agent/`; поставляет architect#1), результат `ops/metrics/bench-<date>.csv`, решение — `ops/metrics/baseline.md` (architect#1).

`ops/metrics/bench-matrix.json` — **у каждой конфигурации появилось поле `provider`** (ADR-005 доп. 2 п. 5): конфигурации E/E+ идут в llama-server, C/A могут идти в Ollama или в llama-server с GGUF — от этого зависит только базовый URL и путь запроса, метрики одинаковые.

```json
{
  "n_per_cell": 20,
  "num_ctx": [8192, 16384],
  "configs": {
    "E":  {"provider": "openai_compat", "url_env": "MV_LLM_URL",
           "tick": "Qwen3.8-27B-UD-Q3_K_XL", "narrative": "Qwen3.8-27B-UD-Q3_K_XL",
           "server_mode": "single", "kv_cache": ["f16", "q8_0"], "primary": true},
    "E+": {"provider": "openai_compat", "url_env": "MV_LLM_URL",
           "tick": "qwen3-8b", "narrative": "Qwen3.8-27B-UD-Q3_K_XL",
           "server_mode": "router", "kv_cache": ["f16"], "material": true},
    "C":  {"provider": "ollama", "url_env": "MV_OLLAMA_URL",
           "tick": "qwen3:30b-a3b", "narrative": "qwen3:30b-a3b", "fallback": 1},
    "A":  {"provider": "ollama", "url_env": "MV_OLLAMA_URL",
           "tick": "qwen3:8b", "narrative": "qwen3:14b", "fallback": 2}
  },
  "request": {
    "stream": false,
    "response_format": "json_schema",
    "chat_template_kwargs": {"enable_thinking": false},
    "sampling_non_thinking": {"temperature": 0.7, "top_p": 0.8, "top_k": 20, "min_p": 0, "presence_penalty": 1.5}
  },
  "thresholds": {
    "NFR-002": {"phase": "phase2", "p95_ms_max": 5000},
    "NFR-002-group3": {"phase": "phase2-group3", "p95_ms_max": 5000, "advisory": true},
    "NFR-090": {"cjk_ratio_max": 0.0, "latin_ratio_max": 0.10, "pass_ratio_min": 0.98},
    "B3": {"valid_json_ratio_min": 0.98}
  },
  "decision_order": ["E", "C", "A"]
}
```

`scripts/llm-bench.ps1` (скелет; PowerShell 7). По сравнению с v0.2: путь `/v1/chat/completions` вместо `/api/chat`, `response_format: json_schema` вместо `format`, `chat_template_kwargs.enable_thinking=false` вместо `think:false`, метрики из `usage`/`timings`, колонки `provider`/`llamacpp_build`/`cached_tokens`.

```powershell
#Requires -Version 7
# scripts/llm-bench.ps1 — матрица замера LLM (overview §18.1, nfr «Что измерить первым», metrics §7 B1–B5)
param(
  [string]  $Matrix   = 'ops/metrics/bench-matrix.json',
  [string]  $Prompts  = 'testdata/bench/prompts.jsonl',
  [string[]]$Configs  = @('E'),                 # E — базовый кандидат; C/A прогоняются, только если E не прошла
  [int]     $NumCtx   = 8192,
  [string]  $KvCache  = 'f16',
  [string]  $Placement= 'native'
)
$ErrorActionPreference = 'Stop'
$m   = Get-Content $Matrix -Raw | ConvertFrom-Json
$out = "ops/metrics/bench-$(Get-Date -Format yyyyMMdd-HHmm).csv"
'config,provider,placement,model,phase,num_ctx,kv_cache,n,first_call_ms,p50_ms,p95_ms,prompt_tok,completion_tok,cached_tok,tps,valid_json_ratio,cjk_ratio,latin_ratio,lang_pass_ratio,vram_used_mb,llamacpp_build,verdict' | Set-Content $out
$prompts = Get-Content $Prompts | ForEach-Object { $_ | ConvertFrom-Json }
$build = (& $env:MV_LLM_BIN --version 2>&1 | Select-String 'build:\s*(\S+)').Matches.Groups[1].Value

foreach ($cfg in $Configs) {
  $c    = $m.configs.$cfg
  $url  = (Get-Item "env:$($c.url_env)").Value       # MV_LLM_URL или MV_OLLAMA_URL
  $head = @{}; if ($env:MV_LLM_API_KEY) { $head['Authorization'] = "Bearer $($env:MV_LLM_API_KEY)" }

  # 0) сервер должен быть поднят нужным режимом: make llm-up [ROUTER=1] с --ctx-size = $NumCtx × slots
  do { Start-Sleep 2; $h = (Invoke-WebRequest "$url/health" -SkipHttpErrorCheck).StatusCode } while ($h -eq 503)
  $models = (Invoke-RestMethod "$url/v1/models" -Headers $head).data.id
  # 1) холодный первый вызов (B2)
  $sw = [Diagnostics.Stopwatch]::StartNew()
  Invoke-RestMethod "$url/v1/chat/completions" -Method Post -Headers $head -ContentType 'application/json; charset=utf-8' -Body (@{
      model = $c.narrative; max_tokens = 1; stream = $false
      chat_template_kwargs = @{ enable_thinking = $false }
      messages = @(@{role='user'; content='ping'})
  } | ConvertTo-Json -Depth 10) | Out-Null
  $first = $sw.ElapsedMilliseconds
  $vram  = (& nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits | Select-Object -First 1)

  # 2) N запросов на фазу
  foreach ($phase in 'tick','phase2','phase2-group3') {
    $model = if ($phase -eq 'tick') { $c.tick } else { $c.narrative }
    if ($models -and ($models -notcontains $model)) { Write-Warning "model $model отсутствует в /v1/models — пропуск"; continue }
    $lat=@(); $ok=0; $cjk=0; $latinSum=0.0; $langPass=0; $pt=0; $ct=0; $cch=0; $predMs=0.0
    foreach ($p in ($prompts | Where-Object phase -eq $phase | Select-Object -First $m.n_per_cell)) {
      $s = $m.request.sampling_non_thinking
      $body = @{
        model = $model; stream = $false; max_tokens = $p.max_tokens
        temperature = $s.temperature; top_p = $s.top_p; top_k = $s.top_k; min_p = $s.min_p; presence_penalty = $s.presence_penalty
        response_format = @{ type = 'json_schema'; json_schema = @{ name = $p.schema_name; schema = $p.schema; strict = $true } }
        chat_template_kwargs = @{ enable_thinking = $false }        # #20345: со включённым thinking грамматика не применяется
        messages = @(@{role='system'; content=$p.system}, @{role='user'; content=$p.user})
      } | ConvertTo-Json -Depth 30
      $sw = [Diagnostics.Stopwatch]::StartNew()
      $r  = Invoke-RestMethod "$url/v1/chat/completions" -Method Post -Headers $head -ContentType 'application/json; charset=utf-8' -Body $body
      $lat += $sw.ElapsedMilliseconds
      $pt  += [int]$r.usage.prompt_tokens; $ct += [int]$r.usage.completion_tokens
      $cch += [int]($r.usage.prompt_tokens_details.cached_tokens ?? 0)
      $predMs += [double]($r.timings.predicted_ms ?? 0)
      $content = [string]$r.choices[0].message.content
      try { $null = $content | ConvertFrom-Json; $ok++ } catch {}
      $hasCjk = $content -match '[\u4E00-\u9FFF]'; if ($hasCjk) { $cjk++ }
      $letters = ($content -replace '[^\p{L}]','').Length
      $latin = if ($letters -gt 0) { (($content -replace '[^A-Za-z]','').Length / $letters) } else { 0 }
      $latinSum += $latin
      if (-not $hasCjk -and $latin -le $m.thresholds.'NFR-090'.latin_ratio_max) { $langPass++ }
    }
    if ($lat.Count -eq 0) { continue }
    $s = $lat | Sort-Object; $n = $s.Count
    $p50 = $s[[math]::Floor($n*0.5)]; $p95 = $s[[math]::Min([math]::Floor($n*0.95), $n-1)]
    $tps = if ($predMs -gt 0) { [math]::Round($ct / ($predMs/1000), 1) } else { 0 }
    $verdict = if ($phase -like 'phase2*') {
        ($p95 -le $m.thresholds.'NFR-002'.p95_ms_max -and ($langPass/$n) -ge $m.thresholds.'NFR-090'.pass_ratio_min -and ($ok/$n) -ge $m.thresholds.B3.valid_json_ratio_min) ? 'pass' : 'fail'
      } else { 'n/a' }
    "$cfg,$($c.provider),$Placement,$model,$phase,$NumCtx,$KvCache,$n,$first,$p50,$p95,$pt,$ct,$cch,$tps,$([math]::Round($ok/$n,3)),$([math]::Round($cjk/$n,3)),$([math]::Round($latinSum/$n,3)),$([math]::Round($langPass/$n,3)),$vram,$build,$verdict" | Add-Content $out
  }
}
Write-Host "written $out"
```

**Правила прогона (F-8):**
1. Сервер поднимается заранее нужным режимом: `make llm-up` (E, C/A на GGUF) или `make llm-up ROUTER=1` (E+, A на двух моделях); `--ctx-size` = `$NumCtx` × слоты. Скрипт сервер **не** запускает и не перезапускает — иначе замер зависел бы от прав и путей.
2. 3 прогона на конфигурацию (разные дни / после перезапуска сервера); `verdict=pass` по всем трём для `phase2` — критерий приёмки конфигурации. `phase2-group3` — справочно (порог группы фиксируется после замера, NFR-002).
3. Порядок решения — `decision_order`: сначала **E** (`num_ctx` 8192 и 16384, KV `f16` и `q8_0` — четыре ячейки); прошла → базовая. Не прошла → **C**, затем **A**. E+ прогоняется как материал, если E не проходит по стоимости тика, но проходит по нарративу.
4. VRAM — `nvidia-smi` в момент прогона (NFR-076); для варианта E ожидание ≈ 16–19 ГБ (13,1 ГБ весов + KV 256 КиБ/токен: ≈ 2 ГБ @8k, ≈ 4 ГБ @16k) — если факт заметно выше, значит часть слоёв или KV ушла в RAM, строку помечать вручную в `baseline.md`.
5. `llamacpp_build` в каждой строке — результаты замера привязаны к билду; после обновления llama.cpp замер варианта-победителя повторяется (§12).
6. `scripts/llm-bench.sh` реализует ту же схему (`curl` + `jq`, вычисления в `awk`); **колонки CSV идентичны** — расхождение колонок ловится ревью при F-8.
7. B5–B8 (старт стека, RTO, стоимость фона) — через `make up` с таймером и `mvctl session-report` после EPIC-002/003/005; `make up` больше не включает прогрев LLM, поэтому `stack_up_time_s` измеряет только контейнеры, а время готовности LLM — отдельная величина из `make llm-up`.

### 6.5. Ollama как опциональный второй рантайм

Ollama остаётся установленной и поддерживается на уровне кода (`MV_LLM_PROVIDER=ollama`, реализация B3b — EPIC-003), но **не является частью пути по умолчанию**:
- нужна для конфигураций **C** и **A** матрицы замера, если они дойдут до прогона, и как запасной путь эмбеддингов (`MV_MEMORY_EMBED_PROVIDER=ollama`, ADR-005 доп. 2 п. 8, решение architect#1 в 005-memory);
- переменные `OLLAMA_*` и `MV_OLLAMA_URL` в `.env.example` закомментированы, из обязательных исключены (§4.2); при `MV_LLM_PROVIDER=ollama` заполняются: `OLLAMA_KEEP_ALIVE=-1`, `OLLAMA_MAX_LOADED_MODELS=2`, `OLLAMA_NUM_PARALLEL=1`, `OLLAMA_FLASH_ATTENTION=1`, `OLLAMA_KV_CACHE_TYPE=f16|q8_0`, `OLLAMA_HOST` (не `0.0.0.0` без файрвола), `OLLAMA_ORIGINS` (никогда `*`) — SEC-15, `compose-lint` п. 5;
- `make models` (`ollama pull` по `ops/models.txt`) и `make warm` (`/api/generate` с `keep_alive:-1`) выполняются **только** при выбранном Ollama;
- контейнерный вариант — профиль `gpu` (§1.3), нативный — предпочтителен по тем же причинам, что и для llama-server (VRAM, диск моделей, отсутствие WSL-прослойки).

`ops/models.txt` получает две секции: `gguf` — файлы для llama-server (`Qwen3.8-27B-UD-Q3_K_XL`; для E+ — `qwen3-8b`; эмбеддинги `nomic-embed-text`/`bge-m3` в GGUF, если пойдут через llama-server), скачиваются вручную в `MV_LLM_MODELS_DIR`; `ollama` — теги для `ollama pull` (`qwen3:30b-a3b`, `qwen3:8b`, `qwen3:14b`, `nomic-embed-text`, `bge-m3`). После F-8 список сокращается до победившей конфигурации и синхронизируется с блупринтами (`mvctl blueprint validate` через `Provider.Models()` — ADR-005 доп. п. 3, C-11 правило 7а).

---

## 7. Наблюдаемость

### 7.1. Логи

- `shared/logging`: `slog` JSON в stdout; обязательные поля через middleware шины и HTTP: `ts, level, service (gateway|core|memory|telegram-bot), context, msg, correlation_id, event_id, event_type, agent.level?, handled (для error), version`. Уровень — `MV_LOG_LEVEL`; `debug` не включается в `prod`. `ReplaceAttr` редактирует ключи `external_id`, `token`, `authorization`, `api_key`, `text` (SEC-02/28); `forbidigo` запрещает `log.Printf`.
- **ПДн и текст**: логгер не принимает произвольные `map`/структуры событий — только явные поля; тела `POST /v1/links/*`, `/v1/characters` и обновления Telegram не логируются; `player.said`/`narrative.output` в логах — только `event_id`, длина текста. Бот редактирует `bot<digits>:<token>` в любом сообщении (ADR-006 доп. п. 3). Тест NFR-041 (`e2e`, `privacy-scan`) прогоняет фикстуры с внешним ID и `username` и `grep`-ает логи всех процессов и бота.
- Docker: `logging: {driver: json-file, options: {max-size: "50m", max-file: "5"}}` для платформы; **`telegram-bot` — `max-size: "10m", max-file: "3"`** (D-12, ADR-006 доп. п. 5: ротация по объёму, «≤ 7 дней» — ориентир; логи бота без ПДн по построению; внешний cron/`lumberjack` не вводятся) и `MV_LOG_LEVEL=info`. Просмотр: `make logs SERVICE=core`, фильтр по корреляции `docker compose logs core | jq -c 'select(.correlation_id=="…")'`.

### 7.2. Health

`GET /health` у `gateway` (:8088), `core` (:8090), `memory` (:8082) и `telegram-bot` (:8089 — только `ok/degraded` по доступности gateway):
```json
{"status":"ok|degraded|down","version":"<git sha>","mode":"live","contexts":["state","mechanics",…],
 "deps":{"bus":"ok","objstore":"ok","llm":"ok|loading|degraded|unavailable","memory":"ok|unavailable","sqlite":"ok"},
 "store":{"versioning":true,"lifecycle":true},
 "agents_by_level":{"global":1,"domain":1,"task":2},"checked_at":"…"}
```
Зависимости опрашиваются раз в 10 с (NFR-016: отказ виден ≤ 30 с); HTTP-код 200 для `ok|degraded`, 503 для `down`; ответ не раскрывает секреты, пути и версии зависимостей (SEC-29). Compose `healthcheck` каждого сервиса платформы — `/multiverse health --url http://127.0.0.1:<port>/health` (`interval: 15s`, `start_period: 30s`); `make health` печатает сводку по всем и вызывает `make llm-health` (§6.3). Значение `llm`: `ok` — `GET $MV_LLM_URL/health` = 200 и модель блупринта есть в `/v1/models`; **`loading`** — 503 от llama-server (модель ещё грузится, это не отказ; провайдер повторяет запрос); `degraded` — сервер отвечает, но нужной модели в списке нет (`model_not_resident`); `unavailable` — соединение не устанавливается. `degraded`/`loading` не переводят процесс в `down`: нарратив деградирует до шаблонов (FR-080/NFR-072).

### 7.3. Метрики MVP-1 (metrics.md §6)

Без Prometheus: `mvctl session-report` читает `analytics_events`, `llm_records`, `game_events` за окно сессии → Markdown в консоль + строка в `ops/metrics/sessions.csv`; `--background --since 24h` → `ops/metrics/background.csv`; `--audit` → `analytics.consistency.violated`; `--json` → `ops/metrics/sessions/<session_id>.json`. `ops/metrics/incidents.csv` — ручной журнал оператора. Всё в Git (ПДн нет). Ресурсный след (NFR-073) — `docker stats --no-stream` и `nvidia-smi` в `make bench` (B5).

### 7.4. Что уходит в EPIC-012 (E-G)

`/metrics` (Prometheus text format) у трёх процессов: латентности фаз, глубина очередей `interactive/background`, `agents_active_by_level`, `llm_calls/tokens/cost` по меткам, `guardian_rejections`, `dlq_size`; `docker compose --profile observability` с Prometheus + Grafana + дашборды из `ops/grafana/`; алерты (NFR-035): p95, `llm_fallback_rate > 0.05`, бюджет фона, `dead_letters` растёт, `/health != ok` > 1 мин → уведомление владельцу через того же бота (служебный чат). До EPIC-012 «алерт» = `make health` с ненулевым кодом в Task Scheduler раз в 15 мин + уведомление Windows. Там же — задача-заглушка «замена объектного хранилища по триггерам ADR-021».

---

## 8. Деплой и откат

«Прод» — та же машина; деплой = сборка образа и перезапуск compose.

1. `make deploy` = `make ci` (локально, без Docker-тестов) → `make backup` (§5.6, включая шифрованный `links.db`) → `make image` (`multiverse-core:<git-sha>`, плюс тег `current`) → запись `MV_IMAGE_TAG=<git-sha>` в `.env` (предыдущее значение — в `.env.previous`) → `docker compose up -d --wait --remove-orphans` (профили из `COMPOSE_PROFILES`) → `make health` (в т. ч. `make llm-health`; если `llama-server` не поднят — `make llm-up` **до** `make deploy`, деплой его не запускает) → smoke: `mvctl world status` и один ход `ci-harness` в тестовом scope (`actor_kind=ci`, в S7 не считается).
2. Миграции SQLite применяются gateway при старте (`goose Up`, §5.4); события — только совместимые изменения схем в пределах `schema_version` (ADR-007 п. 3), несовместимые — `+1` с поддержкой `n-1` потребителями, поэтому «миграции событий» нет; MinIO — версии объектов.
3. **Откат** (`make rollback`): `MV_IMAGE_TAG` ← значение из `.env.previous` → если релиз содержал миграцию SQLite — `docker compose stop gateway` → восстановить `links.db`/`gateway.db` из предрелизного бэкапа (`age -d` + `docker cp` в том `gateway-data`, права `0600`, владелец 65532) **до** старта старого образа (ADR-019: `Down` не пишутся; данные, записанные новой версией между релизом и откатом, теряются — фиксируется в заметке релиза) → `docker compose up -d --wait` → `make health`. LLM-процесс при откате не трогается: он вне образа и вне compose; откат билда llama.cpp — отдельная процедура §9.5. Образы хранятся последние 3 (`make image-prune`). Данные MinIO/Redpanda обратно совместимы по построению; при необходимости — `make restore FILE=<предрелизный бэкап>`.
4. Релизная заметка (`releases/vX.Y.Z.md`, release-manager) содержит: sha образа, список миграций и способ отката (бэкап), изменения `.env.example` (новые переменные — оператор дописывает `.env` **до** `make deploy`; `mvctl env check --env .env` проверяет реальный файл на отсутствующие обязательные), изменения retention/бакетов, изменения `build/versions.env`.
5. Версионирование: `main` = то, что задеплоено; теги `vX.Y.Z` на релизах; образ помечается и sha, и тегом.

---

## 9. Runbook — заготовки (переносятся в `docs/ops/runbook.md` tech-writer'ом на A7)

### 9.1. Запуск с нуля (после клонирования)
1. `cp .env.example .env`; заполнить `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD` (≥ 16 симв.), `MV_MINIO_ACCESS_KEY/SECRET_KEY`, `NEO4J_PASSWORD`/`MV_NEO4J_PASSWORD`, при боте — `MV_TELEGRAM_BOT_TOKEN` и `MV_TELEGRAM_ALLOWED_USER_IDS`; выставить `COMPOSE_PROFILES`.
2. **LLM.** Проверить в `.env`: `MV_LLM_PROVIDER=openai_compat`, `MV_LLM_URL`, `MV_LLM_BIN`, `MV_LLM_MODEL_FILE`, `MV_LLM_MODELS_DIR`, `MV_LLM_NUM_CTX`; модель `.gguf` лежит на месте. `make llm-up` → ждать `/health = 200` → `make llm-health` (печатает `/v1/models` и билд). Проверка доступности из контейнера: `docker run --rm --add-host host.docker.internal:host-gateway curlimages/curl:8.10.1 -s -o /dev/null -w '%{http_code}' http://host.docker.internal:1234/health` → `200`; если нет — правило Windows Firewall для `llama-server.exe` (входящие с `172.16.0.0/12`), **не** `--host 0.0.0.0` (§6.3). Ollama ставится **только** если выбран провайдер `ollama` (§6.5).
3. `make minio-image` (первый раз ≈ 5 мин; клонирует форк `MINIO_REPO`, §2.5) → `make up` (init-контейнеры создают топики/бакеты/пользователя MinIO) → `make health` → `make warm`.
4. `MV_KAFKA_BROKERS=127.0.0.1:19092 MV_MINIO_ENDPOINT=127.0.0.1:9000 go run ./cmd/mvctl world init --blueprints blueprints/ --world $MV_WORLD_ID` (EPIC-002 I1; бакеты с versioning/ILM создаёт `objstore`) → `mvctl world status`. **Любая команда `mvctl`, которая пишет в шину** (`world init`, `laws bump`, `record`), запускается на хосте и должна получить external listener `127.0.0.1:19092` — имени `redpanda` на хосте нет (§1.4, §4.2).
5. Бот: `/start` в Telegram (с user id из allowlist) → уведомление FR-009 → создание персонажа.

### 9.2. Остановка / перезапуск
- `make down` — останавливает контейнеры, тома сохраняются; после `make up` `core` восстанавливается по C-14 (снапшот + догон журнала), в логе `analytics.replay.completed identical=true`.
- Перезапуск одного процесса: `docker compose restart core` (bot держит long-poll к gateway — gateway перезапускать отдельно, бот переподключится).
- `make reset` — **удаляет тома** (спрашивает подтверждение, требует свежего `make backup`).

### 9.3. LLM: старт, проверка, смена модели (карточка `llama-server`)

**Старт.** `make llm-up` (single-режим, вариант E) или `make llm-up ROUTER=1` (router, варианты A/E+). Скрипт стартует процесс, пишет PID в `ops/llm-server.pid` и ждёт `GET $MV_LLM_URL/health = 200` (до 180 с; пока грузится — `503 Loading model`, это норма), затем делает один короткий запрос `/v1/chat/completions` и печатает его латентность. Отдельный «прогрев» (`make warm`) для llama-server **не нужен** — веса резидентны (`mmap+mlock`).

**Проверка.** `make llm-health` → код `/health`, список `GET /v1/models`, `LLAMACPP_BUILD` (факт vs пин в `build/versions.env`), `nvidia-smi` (used/total VRAM). Затем `mvctl llm ping` — один запрос phase2 на записанном промпте: печатает латентность, `valid_json`, `usage.prompt_tokens_details.cached_tokens`.

**Стоп / рестарт.** `make llm-down` (по PID-файлу; при его потере — `Get-Process llama-server | Stop-Process`). Рестарт нужен при: смене модели, `--ctx-size`, числа слотов, обновлении билда. Платформу останавливать не требуется — `core` переживает недоступность LLM как деградацию (FR-080).

**Смена модели.**
1. Положить `.gguf` в `MV_LLM_MODELS_DIR`; 2. `make llm-down`; 3. поправить `MV_LLM_MODEL_FILE` в `.env` (single) **или** запускать `ROUTER=1` и указывать модель полем `model` в блупринте (router, §6.2.1); 4. `make llm-up` → `make llm-health` (имя должно появиться в `/v1/models`); 5. обновить `model` в блупринтах и `LLM_MODEL_DEFAULT` в `build/versions.env`; `mvctl blueprint validate blueprints/` — модель блупринта обязана быть в списке провайдера (C-11 правило 7а), иначе `core` отдаст `degraded(model_not_resident)`; 6. один прогон `make bench` по победившей конфигурации и запись в `ops/metrics/baseline.md`.

**Обновление версии llama.cpp.** Отдельной задачей, не «по дороге» (§9.5, риск §12): скачать новый релиз → `make llm-down` → заменить бинарник (старый каталог сохранить) → `llama-server --version` → обновить `LLAMACPP_BUILD` в `build/versions.env` → `make llm-up` → `make llm-health` → **прогон золотого набора** (`make test-e2e` на записях `testdata/recordings/*.jsonl` + `mvctl llm ping` по каждой фазе со схемой) → если `valid_json_ratio` или язык просели — вернуть прежний бинарник и завести задачу. Изменения в `--reasoning`/грамматике проверяются именно здесь.

**Диагностика `/health.llm`:**
| Симптом | Что смотреть |
|---|---|
| `loading` дольше 3 мин | размер модели vs VRAM; `nvidia-smi` (не занята ли карта другим процессом); при `mlock` — хватает ли RAM |
| `unavailable` с хоста | процесс жив? (`Get-Process llama-server`), `curl http://127.0.0.1:1234/health`, `ops/llm-server.pid` |
| `unavailable` только из контейнера | `host.docker.internal` (см. §9.1 п. 2), правило файрвола, `extra_hosts` у `core` |
| `degraded(model_not_resident)` | `GET /v1/models` vs `model` в блупринте; в single-режиме поле `model` игнорируется — совпадение имени обязательно |
| валидный JSON приходит редко | thinking не выключен: проверить, что шлюз шлёт `chat_template_kwargs.enable_thinking=false`, а сервер запущен с `--jinja` и `--reasoning off` (#20345) |
| ответ в markdown-обёртке / чужие поля | то же (#20345); страховка — стратегия парсера `strip_think` (ADR-016) |

Логи сервера — stdout процесса (`make llm-up` перенаправляет в `ops/llm-server.log`, ротация не настраивается: файл удаляется при рестарте). `--verbose` в служебном режиме **не** включаем (объём, SEC-02).

### 9.4. Восстановление
- **После падения процесса/машины (тома целы):** `make up` → `make health` → в логах `core` `replay.completed` → `mvctl session-report --audit` → `state_divergence_count = 0`.
- **Из бэкапа на чистой машине:** установить Docker Desktop, llama.cpp (билд `LLAMACPP_BUILD`) и скачать модель `.gguf`, `git clone`, `.env` из менеджера паролей владельца → `make minio-image` → `make restore FILE=minio-<date>.tgz` (распаковка в том при остановленных сервисах) → то же для `redpanda-<date>.tgz` → `age -d links-<date>.db.age > links.db` и `docker cp` в том `gateway-data` (`/data/links.db`, права 0600, владелец 65532) → `make llm-up` → `make up` → `make health` → `mvctl memory rebuild` (профиль `memory`) → `mvctl session-report --audit`.
- **Порча индекса памяти:** `docker compose stop memory` → `docker volume rm` томов qdrant/neo4j (или `mvctl memory reset`) → `docker compose up -d memory` → `mvctl memory rebuild --since 30d`.
- **Переполнение диска:** `docker system df`; `make image-prune`; проверить `segment.ms` и retention (`rpk topic describe -c`); ILM MinIO (`mc ilm ls local/entities-<world>`).

### 9.5. Обновление версий инфраструктуры
Одна версия за раз: изменить пин **в `build/versions.env`** (единственное место; compose, Makefile и `testkit.Versions()` читают его) → `make backup` → `docker compose pull <svc>` (или `make minio-image` для MinIO) → `docker compose up -d <svc>` → `make health` → `make test-integration` тем же PR. Redpanda — только на соседнюю feature-версию (26.1 → 26.2), Neo4j — внутри 5.26.x свободно, **llama.cpp — по процедуре §9.3 «Обновление версии» с золотым набором** (пин `LLAMACPP_BUILD`; сервер вне compose, `docker compose pull` к нему не относится), MinIO — новых тегов upstream не будет (ADR-021); обновление = смена builder-образа Go при необходимости; при срабатывании триггера ADR-021 п. 4 — задача EPIC-012.

### 9.6. Ротация токена бота
1. `@BotFather` → `/revoke` (старый токен перестаёт работать немедленно; long-poll бота упадёт — ожидаемо).
2. Обновить `MV_TELEGRAM_BOT_TOKEN` в `.env` → `docker compose up -d telegram-bot` (только бот; платформа не перезапускается).
3. `docker compose logs --tail=50 telegram-bot` → `getMe ok` (токен в логе отредактирован); `/help` в чате.
4. Если старый токен попадал в git/логи: `gitleaks git` по истории — фиксировать в `ops/metrics/incidents.csv` (`kind=other`), очистка истории — по решению владельца.

### 9.7. Ежедневный/еженедельный чек оператора
- Перед сессией (вручную): `make llm-up` — LLM-процесс службой не сделан, после перезагрузки Windows он не поднимется сам (§6.1).
- Ежедневно (автоматически, Task Scheduler): `make health` (ненулевой код → уведомление; включает `llm-health`), `mvctl session-report --background --since 24h` (строка в `background.csv`).
- Еженедельно: `make backup`, `docker system df`, `mvctl session-report --weekly`, ревью `incidents.csv`, PR Dependabot.
- Ежемесячно: сверка `build/versions.env` с Docker Hub (Dependabot compose не видит) **и `LLAMACPP_BUILD` с релизами `ggml-org/llama.cpp`** — обновление только отдельной задачей (§9.3); ежеквартально — учения восстановления `links.db` (§5.6).

### 9.8. Карточки процессов (заготовки для `docs/ops/runbook.md`)

| | `gateway` | `core` | `memory` (профиль `memory`) | `telegram-bot` (профиль `bot`) |
|---|---|---|---|---|
| Команда | `/multiverse --contexts=gateway` | `/multiverse --contexts=state,mechanics,swarm,llm,laws` | `/multiverse --contexts=memory` | `/telegram-bot` |
| Порт / health | `127.0.0.1:8088/health` | `127.0.0.1:8090/health` | `127.0.0.1:8082/health` | `127.0.0.1:8089/health` |
| Зависимости в `/health.deps` | `bus`, `sqlite`, `core` (для admin-прокси) | `bus`, `objstore`, `llm`, `memory` (опц.) | `bus`, `qdrant`, `neo4j` | `gateway` |
| Данные | том `gateway-data` (`links.db`, `gateway.db`) | MinIO (`entities-*`, `snapshots-*`, `prompts-*`), журнал Redpanda | тома `qdrant_storage`, `neo4j_data` (производные) | нет (без курсора; состояние — в gateway) |
| Старт / стоп | `docker compose up -d gateway` / `stop gateway`; при рестарте бот переподключается сам | `docker compose restart core` → в логе `replay.completed identical=true` | `docker compose restart memory`; при `unavailable` `core` деградирует (FR-035) | `docker compose up -d telegram-bot`; long-poll один на токен (409 в логе = второй экземпляр) |
| Логи | `make logs SERVICE=gateway`; access-log без тел (SEC-02) | `make logs SERVICE=core`; фильтр по `correlation_id` | `make logs SERVICE=memory` | `make logs SERVICE=telegram-bot` (`10m × 3`, токен отредактирован) |
| Типичный сбой → действие | `sqlite` ≠ ok → проверить том, `db check`; `403 client_unknown` → `MV_GATEWAY_CLIENT_IDS` | `llm=unavailable` → §9.3; `objstore` ≠ ok → `make health`, `mc admin info`; `store.versioning=false` → предупреждение, не ошибка | индекс повреждён → §9.4 «порча индекса» | бот молчит → `MV_TELEGRAM_ALLOWED_USER_IDS`, `getMe`, `gateway /health`; 409 → убить второй экземпляр |
| Секреты | `MV_MINIO_*` не нужны; только том | `MV_MINIO_*`, облачные ключи (если включены) | `MV_NEO4J_PASSWORD` | `MV_TELEGRAM_BOT_TOKEN` (только здесь) |
| Порядок при полном рестарте | 3 | 2 (после `redpanda-init`, `minio-init`) | 2 | 4 (последним) |

Пятая карточка — **`llama-server`** (нативный процесс, не сервис compose):

| | `llama-server` (llama.cpp, нативно) |
|---|---|
| Команда | `make llm-up` → `pwsh scripts/llm-server.ps1` (`-Router` для двух моделей); аргументы — §6.2 |
| Порт / health | `127.0.0.1:1234`: `GET /health` (200 `ok` / **503 `loading`**), `GET /v1/models` |
| Зависимости | GPU (NVIDIA-драйвер), файл модели в `MV_LLM_MODELS_DIR`; шины и MinIO не касается |
| Данные | только файлы моделей `.gguf` и `MV_LLM_SLOT_SAVE_PATH` (кэш слотов, не бэкапится, удаляем свободно) |
| Старт / стоп | `make llm-up` / `make llm-down` (PID — `ops/llm-server.pid`). **`make up`/`make down` его не трогают**; после перезагрузки машины поднимается вручную (службы в MVP-1 нет) |
| Логи | `ops/llm-server.log` (stdout процесса); `--verbose` не включаем (SEC-02) |
| Типичный сбой → действие | `503` дольше 3 мин → VRAM/RAM, §9.3; `unavailable` из контейнера → `host.docker.internal` + файрвол, §9.1 п. 2; невалидный JSON → thinking не выключен (#20345), §9.3; после обновления билда — золотой набор, §9.3 |
| Секреты | нет: `--api-key` локально не задаётся; `MV_LLM_API_KEY` пуст (§4.2) |
| Порядок при полном рестарте | **1** (до `make up`, чтобы `core` увидел `llm=ok` с первого health-цикла; не обязателен — при отсутствии платформа стартует с деградацией) |

---

## 10. Чек-лист «инициализация проекта» (EPIC-001, под F-0…F-10 по `epics.md` v0.2 §6)

Порядок: **F-0 (выполнен) → F-1 ∥ F-3 → F-2 → (F-4a ∥ F-4b ∥ F-5 ∥ F-6) → (F-4c ∥ F-5t ∥ F-10 ∥ F-7 ∥ F-8) → F-9**. Критический путь: F-1 → F-2 → F-4a → F-5t. Каждый шаг заканчивается проверяемой командой; devops-задачи — F-6, F-7 (+ участие в F-1, F-5, F-8).

| # | Задача | Что сделать (инфраструктурная часть) | Критерий (команда) |
|---|---|---|---|
| F-0 | **выполнено** (коммит `744fb10`) | `git worktree prune`; gitlinks `.claude/worktrees/*` удалены; `.gitignore += .claude/worktrees/`; `Docs/dev-team/**` зафиксированы; ветка `integration/mvp-1` от `feature/agent-gm-core` | `git status` работает; `git branch --list integration/mvp-1` |
| — | Ветка эпика | `git switch -c epic/EPIC-001-foundation integration/mvp-1` (политика `git.branches=true`; коммиты — по подтверждению, `commits=ask`) | ветка есть |
| F-1 | Гигиена индекса (§4.5, п. 1–13) | `.gitleaks.toml` → pre-commit → плейсхолдер `shared/oracle/README.md` (14/29/68) → `git rm --cached` секретов/бинарников/логов → пустые каталоги → `.gitignore`, `.gitattributes`, `.mcp.env.example`, `.env.example` (§4.2 — LLM-блок под `openai_compat`). **IDE/AI-каталоги (`.idea`, `.vscode`, `.kilo*`, `.roo*`, `.qwen`, `memory/`, `plans/`, `reports/`, `event-model.pdf`) НЕ выводятся из индекса — U-11, решение в F-9** | `gitleaks dir --redact .` = 0; `git ls-files \| grep -E '\.(exe\|log)$\|\.mcp\.env$\|settings\.local'` пусто; `pre-commit run --all-files` зелёный; хук отклоняет фиктивный токен на Windows |
| F-3 | Архив кода (§4.6) | `git mv` в `services/_archive/<путь>` + `ARCHIVED.md` + `services/_archive/README.md`; `FROZEN.md` в 8 сервисах; `Docs/archive/`; корневой `Dockerfile` и 3 дубля → `_archive/build/`; as-is `build/Dockerfile` → `build/legacy.Dockerfile` | `go build ./...` в корне не видит `_archive` (после F-2); индекс `_archive/README.md` полон |
| F-2 | Единый модуль | удалить `go.work*`; `go.mod`: `go 1.26`, `toolchain go1.26.x` (патч из `build/versions.env`); слить `shared/{eventbus,jsonpath,entity,agent}`; каркас `cmd/multiverse` (`--contexts/--mode/--bus/--recording`, подкоманды `health --url`, `db backup\|check`), `shared/runtime` (HTTP-сервер процесса, `MV_CORE_ADDR=:8090`), `shared/clock`; `.golangci.yml` (`depguard` по ADR-001 доп. п. 2, `forbidigo`); `build/Dockerfile` (§2.3) | `go mod tidy && go build ./... && go vet ./... && make lint`; `go run ./cmd/multiverse --contexts=all --bus=memory` → `curl 127.0.0.1:8088/health` и `127.0.0.1:8090/health` → `{"status":"ok"}`; локально Go 1.26 установлен |
| F-4a/b/c | Шина, контракты, `mvctl` | (разработчики) — инфраструктурно: `schemas/**` валидируются `jsonschema/v6`; `mvctl env check`, `storage init` | `make contracts` зелёный |
| F-5 | `shared/objstore`, `shared/env`, `shared/logging`, **`build/versions.env`** | `objstore` по ADR-021 п. 2 (`EnsureBucket` с versioning/ILM, `Capabilities()`); реестр `shared/env` (`MV_`); `.env.example` по §4.2; `versions.env` по §2.4 | `mvctl env check` зелёный; unit-тест логгера «внешний ID не попадает в лог»; `testkit.Versions()` читает `versions.env` |
| F-5t | `shared/testkit` | membus с `Journal`, `Dedup`, `Versions()`, `containers.go` (образы из `versions.env`, MinIO — наш), contract-тест шины; интеграционные тесты «Redpanda: Publish → Subscribe порядок», «MinIO: `EnsureBucket` → versioning on / `prompts-*` off + ILM 30» | `make test-integration` зелёный локально (Docker Desktop) |
| **F-6** | Compose + Makefile + образы (devops) | `docker-compose.yml`: образы `${…}` из `versions.env` (`COMPOSE_ENV_FILES`), профили `memory/gpu/bot/dev/legacy` (§1.3), `redpanda-init` (`build/redpanda-init.sh`, 8 топиков, retention 30/90/180, `segment.ms=1d`), `minio-init` (`build/minio-init.sh`), healthchecks, `depends_on: condition`, **все порты `127.0.0.1`**, `${VAR:?}` для паролей, ротация логов (`50m×5`, бот `10m×3`), Neo4j без APOC, токен только у бота; **`build/minio.Dockerfile`** (§2.5) — клонирует **форк владельца** `MINIO_REPO` (U-10; форк создаёт владелец до задачи), тарбол исходников в `backups/`; `.dockerignore`; Makefile (§2.2, включая `minio-image`, `compose-lint`, `archive-legacy`, **`llm-up`/`llm-down`/`llm-health`**); **`scripts/llm-server.ps1` и `.sh`** (§6.2), пин `LLAMACPP_BUILD` и `LLM_MODEL_DEFAULT` в `build/versions.env`; `extra_hosts: host.docker.internal:host-gateway` у `core`; `scripts/compose-lint.sh` (+ правило «нет сервиса llama-server», §3.1.1 п. 5); решение по жизнеспособности `legacy` (Chroma тег, as-is env) | `make minio-image` (из форка) → `minio --version` = тег; `git ls-remote --tags $MINIO_REPO` содержит тег; `make compose-lint` зелёный; `docker compose --env-file build/versions.env --env-file .github/ci.env config -q`; **`make llm-up` → `make llm-health` = 200 + модель в `/v1/models`; из контейнера `curl http://host.docker.internal:1234/health` = 200; `make llm-down` останавливает процесс**; `make up` на машине владельца → `make health` = ok для `gateway/core` (контексты-заглушки) и предупреждение (не ошибка) при остановленном LLM; `make up PROFILES=legacy` поднимает orchestrator + semantic-memory :8083 + chroma |
| **F-7** | CI (devops) | `.github/workflows/go.yml` (§3.1: 6 job'ов + `image`; SHA-пины; `permissions`; `go mod verify`/`tidy -diff`; govulncheck блокирующий; privacy-scan; compose-lint); `scripts/coverage-gate.sh`; `.github/dependabot.yml`; **`.github/CODEOWNERS` = `* @alekseizabelin1985-spec` + контрактные каталоги (§3.2, U-9)**; `.github/ci.env`; удалить `validate-blueprints.yml`; branch protection на `main`, `integration/mvp-1` | PR в `integration/mvp-1` → все шесть job'ов зелёные ≤ 10 мин; `integration` собирает MinIO из кэша ≤ 1 мин на втором прогоне |
| F-8 | Матрица замера (architect#1 + владелец; devops — скрипты) | `scripts/llm-bench.ps1`, `scripts/llm-bench.sh` — против `/v1/chat/completions` (`response_format json_schema`, `chat_template_kwargs.enable_thinking=false`), метрики из `usage`/`timings` + `nvidia-smi`; `ops/metrics/bench-matrix.json` с полем `provider` и конфигурациями **E/E+/C/A**; `ops/models.txt` (секции `gguf` и `ollama`); `testdata/bench/prompts.jsonl` — architect#1 (§6.4) | `make llm-up && pwsh scripts/llm-bench.ps1 -Configs E` → `ops/metrics/bench-<date>.csv` с `verdict` для E (num_ctx 8192/16384, KV f16/q8_0) и колонкой `llamacpp_build`; при `fail` — прогон C, затем A; `baseline.md` (architect#1) |
| F-10 | Заглушки контрактов v0 | (developer + architect#1) — инфраструктурно: фикстуры `testdata/fixtures/*` не содержат внешних ID (privacy-scan) | `security` job зелёный |
| F-9 | Документы (tech-writer) | `README.md` (запуск за 5 команд, список `winget`: `ezwinports.make`, `jqlang.jq`, `FiloSottile.age`, `Gitleaks.Gitleaks` опц.), `CLAUDE.md`/`AGENTS.md`, `services/_archive/README.md`, `docs/ops/runbook.md` из §9, `.dev-team.json.stack` по факту (Go 1.26, MinIO из исходников, Qdrant; Chroma только `legacy`; без Timescale/Redis) | ревью |
| Готовность EPIC-001 | `epics.md` | `make ci` зелёный; `make up` поднимает инфраструктуру и пустые контексты с `/health`; `mvctl contracts check` без фантомов; секретов в HEAD нет; `baseline.md` с конфигурацией моделей; `services/_archive/README.md` полон | — |

---

## 11. Замечания к архитектуре — статус после сведения (`consolidation.md` §7)

| # | Замечание v0.1 | Решение | Отражено в этом документе |
|---|---|---|---|
| D-1 | Go 1.25 вне поддержки | **П** — `go 1.26` + `toolchain go1.26.x`; `govulncheck` блокирующий | §2.1, §2.4, §3.1 |
| D-2 | MinIO: образы не публикуются | **П** — сборка из исходников (ADR-021), OQ-A-20 подтверждён на G2 | §2.4, §2.5, §5.2 |
| D-3 | Профиль `legacy` vs удаление Chroma | **ПИ** — Chroma + as-is `semantic-memory` (:8083) в профиле `legacy` до S5 | §1.3, §1.4, §5.5 |
| D-4 | Префикс `MV_` в ADR-005/009 | **П** | §4.1, §4.2 |
| D-5 | JSON Schema 2020-12 — `jsonschema/v6` | **П** | §2.4, §3.1 |
| D-6 | Versioning/ILM в `EnsureBucket` | **П** | §5.2 |
| D-7 | HTTP-порт `core` | **ПИ** — **8090**, сервер у процесса (`shared/runtime`) | §1.2, §1.4, §4.2, §7.2, §9.8 |
| D-8 | Том Redpanda — пересоздание | **П** (+ `make archive-legacy`) | §5.5 |
| D-9 | `segment.ms=1d` | **П** (ADR-007 доп. п. 2) | §5.1 |
| D-10 | Профиль `bot` | **П** | §1.3 |
| D-11 | `.claude/worktrees/*` — первый шаг F-1 | **П** — выполнено в F-0 | §4.5 |
| D-12 | Логи бота «≤ 7 дней» | **ПИ** — `10m × 3`, ориентир | §7.1 |
| D-13 | `build/versions.env` | **П** | §2.4, §9.5 |
| D-14 | `*.jsonl` `-diff merge=binary` | **ПИ** — `merge=binary` без `-diff` | §4.5 п. 10 |
| T-13 | `prompts-*` ILM 30 дн., versioning off, вне бэкапа | **П** | §5.2, §5.6 |
| T-16 | gitleaks allowlist только `*.example` | **П** | §4.5 п. 1 |
| T-17 | Neo4j без APOC | **П** | §5.3 |
| T-15 | CI hardening | **П** | §3.1, §3.2 |
| T-8 / U-6 | `links.db`: именованный том, бэкап; OneDrive снят | **П** | §5.4, §5.6 |
| U-1 | Архив вместо удаления | **П** | §4.5 п. 7, §4.6 |
| **U-8** | LLM-рантайм — нативный `llama-server`, провайдер `openai_compat`; Ollama опционально | **ПИ** — рекомендация devops v0.2 «Ollama нативно» отменена решением пользователя; принято полностью | §0, §1.1–§1.4, §2.2, §2.4, §4.2, §6 (переписан), §7.2, §8, §9.1, §9.3, §9.8, §12 |
| **U-9** | CODEOWNERS `@alekseizabelin1985-spec`; репозиторий приватный, личный | **П** | §3.1, §3.2, §12 |
| **U-10** | Форк `minio/minio` в аккаунт владельца, сборка из форка (F-6) | **П** | §2.4, §2.5, §10 F-6, §12 |
| **U-11** | IDE/AI-каталоги в индексе — оставить до F-9 | **П** | §4.5 п. 8, §10 F-1 |
| **U-12** | GPU-замер вручную, без self-hosted runner | **П** (допущение devops подтверждено) | §3.3, §6.4 |

Открытых замечаний к архитектуре нет. Новые вопросы — §13.

---

## 12. Риски и допущения

| Риск / допущение | Влияние | Реакция |
|---|---|---|
| Исходники MinIO тега `RELEASE.2025-10-15T17-29-55Z` не собираются builder-образом Go 1.26 (репозиторий архивирован, обновлений нет) | `build/minio.Dockerfile` не собирается | вторая попытка с `golang:1.24.x` (ADR-021); тарбол исходников в `backups/` (§2.5); триггер (б) ADR-021 → SeaweedFS-spike в EPIC-012 |
| Репозиторий `minio/minio` на GitHub становится недоступным | образ нельзя пересобрать | **две страховки (U-10)**: форк `alekseizabelin1985-spec/minio` (делает владелец в F-6, `MINIO_REPO`) + тарбол исходников тега в `backups/` (§2.5) |
| Образ `minio/mc` исчезнет с Docker Hub | `minio-init` не стартует | `mvctl storage init` на `minio-go` (ADR-021 п. 1) — заложить в F-4c как заглушку |
| **Обновление llama.cpp меняет поведение грамматики `json_schema`, шаблона `--jinja` или преамбулы reasoning** — ответы перестают быть валидным JSON или обрастают markdown-обёрткой | NFR-022 (`valid_first_try`), NFR-002, весь путь агентов | **пин `LLAMACPP_BUILD` в `build/versions.env`** + бинарник ставится вручную (автообновления нет); обновление — отдельной задачей по §9.3 с **прогоном золотого набора** (`make test-e2e` на записях + `mvctl llm ping` по каждой фазе со схемой) и повторным `make bench` победившей конфигурации; при регрессе — возврат прежнего бинарника (старый каталог сохраняется). Вторая линия — парсер `strip_think` (ADR-016) |
| **Дефект llama.cpp #20345**: при включённом thinking грамматика `json_schema` не применяется | невалидные ответы во всех фазах со схемой | шлюз шлёт `chat_template_kwargs.enable_thinking=false` на каждый запрос (ADR-005 доп. 2 п. 1), сервер стартует с `--reasoning off` (§6.2) — две независимые точки; регресс ловится golden-набором и `valid_json_ratio` в `make bench` |
| `llama-server` не поднят службой: после перезагрузки Windows LLM молчит, пока оператор не вспомнит `make llm-up` | первая сессия деградирует до шаблонов (FR-080) | `make up` и `make health` печатают явное предупреждение с командой; строка «перед сессией» в §9.7; служба — E-H |
| Установленный у владельца билд llama.cpp не поддерживает флаг из скрипта (`--load-mode`, `--models-dir`, `--kv-unified`, `--reasoning`, `--no-webui` появлялись в разных релизах) | `make llm-up` падает на старте | набор флагов сверяется с фактическим `llama-server --help` **при F-6** и фиксируется вместе с `LLAMACPP_BUILD`; скрипт печатает stderr сервера и не «глотает» ошибку |
| `host.docker.internal` из контейнеров не достигает нативного `llama-server` на loopback | LLM недоступен из `core` (с хоста работает) | проверка §9.1 п. 2 (`--add-host host.docker.internal:host-gateway`); правило Windows Firewall для `llama-server.exe` на подсеть Docker; `--host 0.0.0.0` — только вместе с файрволом и как крайняя мера (SEC-15) |
| Модель варианта E (dense 27B) не проходит NFR-002 по p95 | базовая конфигурация меняется | порядок решения `E → C → A` заложен в `bench-matrix.json` и в §6.4; смена конфигурации — правка блупринтов и аргументов запуска, не кода (`Provider` один) |
| Ollama (если понадобится для C/A): автообновление ломает пин версии и structured output | NFR-071, NFR-022 | `ollama serve` службой с фиксированной версией; `mvctl llm ping` в `make health`; золотой набор ловит регрессию |
| testcontainers на GitHub-hosted раннере: pull четырёх образов ≈ 1–2 мин + сборка MinIO | `integration` дольше 7 мин | кэш `type=gha` для MinIO; Neo4j-тесты только в `integration`, `t.Parallel()`; при превышении — Neo4j-тесты в отдельный job по `paths` `internal/memory/**` |
| Pre-commit на Windows: `language: golang` требует Go в `PATH` у всех Git-клиентов (IDE) | хук не срабатывает из IDE | `pre-commit run --all-files` в `make ci`; CI-gitleaks — вторая линия; альтернатива `winget` + `language: system` |
| Четыре скрипта в двух вариантах (`llm-server`/`llm-bench` × `.ps1`/`.sh`) расходятся | разные аргументы запуска и разные CSV | одна схема колонок и один список аргументов — в этом документе (§6.2, §6.4); `.ps1` — основной (llama-server нативен на Windows), `.sh` проверяется при F-8 или помечается «не поддерживается в MVP-1», если стенд один |
| `make` и bash-скрипты на Windows (CRLF, отсутствие `jq`/`age`/`make`) | локальный `make ci` не работает у владельца | `.gitattributes eol=lf`; `winget`-список в README; альтернатива — `wsl make ci` |
| Бэкапы вручную/Task Scheduler — человек забудет | RPO до недели | `make health` печатает возраст последнего бэкапа и предупреждает > 7 дней |
| Единственный писатель SQLite и `VACUUM INTO` во время нагрузки | кратковременные `SQLITE_BUSY` | `busy_timeout=5000`; бэкап — в паузе между сессиями |
| Откат релиза с миграцией SQLite = восстановление файла (ADR-019, без `Down`) | потеря ходов между релизом и откатом | бэкап `links.db`/`gateway.db` перед каждым `make deploy`; релиз — между сессиями |
| GitHub Actions minutes на приватном репозитории — 2 000 мин/мес на Free (U-9: репозиторий приватный) | ≈ 10 мин × 200 прогонов | `paths-ignore`, `concurrency cancel-in-progress`; при исчерпании — `integration` только на `integration/mvp-1`/`main`; job `image` уже не пушит образы |
| Репозиторий приватный на личном аккаунте (U-9): `gitleaks-action` без лицензии, лимит Actions 2 000 мин/мес | подтверждено владельцем, не допущение | при переходе на организацию — `GITLEAKS_LICENSE` или `gitleaks` через Docker-образ в job; при исчерпании минут — `integration` только на `integration/mvp-1`/`main` |
| Допущение: Docker Desktop с WSL2, `host-gateway` доступен; NVIDIA-драйвер с WSL-поддержкой нужен только для **опционального** профиля `gpu` (основной LLM — нативный, WSL ему не нужен) | — | профиль `gpu` не запускается без подтверждения `nvidia-smi` внутри `docker run --gpus all` |
| Допущение: `qwen-*` workflow используют секреты/vars владельца и не мешают `go.yml` | — | не трогаем; группы `concurrency` разные |
| Допущение: точный патч Go 1.26 и SHA Actions не проверялись на дату документа | пин может устареть к F-2/F-7 | фиксируются при выполнении F-2/F-7 по `go.dev/dl` и `gh api`; далее Dependabot |

---

## 13. Вопросы владельцу — статус

Все пять вопросов v0.2 **закрыты решениями пользователя** от 2026-09-09 (`journal.md` «Решения пользователя (DevOps)», `consolidation.md` §10 U-8…U-12). Новых вопросов у devops нет.

| # | Вопрос v0.2 | Решение | Где отражено |
|---|---|---|---|
| 1 | Ollama нативно (рекомендация devops) или контейнер профиля `gpu`? | **U-8: ни то, ни другое как основной путь** — LLM-рантайм по умолчанию — нативный `llama-server` (llama.cpp) на `127.0.0.1:1234`, провайдер `openai_compat`; Ollama остаётся **опциональным** вторым рантаймом (нативно или профиль `gpu`) для конфигураций C/A и эмбеддингов. Рекомендация devops отклонена пользователем | §6 (переписан), §0, §1.1–§1.4, §2.2, §2.4, §4.2, §7.2, §9.3, §9.8, §12 |
| 2 | GitHub-логин владельца для `CODEOWNERS`; приватность репозитория | **U-9: `@alekseizabelin1985-spec`; репозиторий приватный на личном аккаунте** → `gitleaks-action` без `GITLEAKS_LICENSE`, лимит Actions 2 000 мин/мес учтён | §3.1, §3.2, §10 F-7, §12 |
| 3 | Форк `minio/minio` или только тарбол в `backups/`? | **U-10: форк в аккаунт владельца, в F-6** (создаёт владелец, агенты внешние системы не меняют); `build/minio.Dockerfile` собирает из форка (`MINIO_REPO`); тарбол остаётся **второй** страховкой | §2.4, §2.5, §10 F-6, §12 |
| 4 | GPU-замер вручную или self-hosted runner? | **U-12: вручную `make bench`**; self-hosted runner не заводим (доступ к машине и токену репозитория) | §3.3, §6.4 |
| 5 | IDE/AI-каталоги в индексе — вывести в F-1 или оставить? | **U-11: оставить, решение в F-9** (tech-writer); в F-1 `git rm --cached` по ним не выполняется | §4.5 п. 8, §10 F-1 |

**Требует действия владельца (не вопрос, а шаг вне полномочий агентов):**
1. Создать форк `minio/minio` в аккаунт `alekseizabelin1985-spec` **до** выполнения F-6 (иначе `make minio-image` не найдёт `MINIO_REPO`; временный обход — `MINIO_REPO=https://github.com/minio/minio.git`).
2. Сообщить фактический билд `llama-server` (`llama-server --version` → `build: <N> (<sha>)`) для пина `LLAMACPP_BUILD` в `build/versions.env`; при F-6 сверить набор флагов скрипта с `llama-server --help` этого билда.
3. Настроить branch protection на `main` и `integration/mvp-1` (required checks, CODEOWNERS-ревью на `main`) — операция в UI GitHub, выполняется владельцем.

История изменений: v0.1 — 2026-09-09 — создано (этап A3, к G2); v0.2 — 2026-09-09 — приведено к решениям сведения G2 (перечень в шапке), чек-лист под F-0…F-10, runbook по процессам; **v0.3 — 2026-09-09 — сведение 2 (U-8…U-12, ADR-005 доп. 2): §6 переписан под нативный `llama-server`, §4.2 под провайдер `openai_compat`, §6.4 под варианты E/E+, CODEOWNERS и форк MinIO закреплены, §13 закрыт**.
