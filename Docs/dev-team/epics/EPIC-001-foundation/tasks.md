# Задачи EPIC-001 «Фундамент» (волна 0)

Версия 0.1.1 · 2026-09-09 · tech-lead#1 (TEAM-1, тимлид проекта) · статус: к G3.
**Правки сведения 3** (`architecture/consolidation.md` §14, `contracts.md` v0.4, `infrastructure.md` v0.3; внесено tech-lead#1): T-001 (F-1 — IDE-каталоги), T-004 (F-6a — `MINIO_REPO`/`LLAMACPP_BUILD`/`LLM_MODEL_DEFAULT`, `extra_hosts`, форк MinIO), T-006 (F-4b-1 — `Spec.Publishers`, строка gateway в `OwnershipRules`, `cause=forget`), T-007 (F-5 — `MV_LLM_*`, условные `OLLAMA_*`), T-008 (F-6b — `llm-server.*`, `make llm-*`, правило `compose-lint`), T-012 (F-7 — CODEOWNERS), T-013 (F-8 — `prompts.jsonl`, порядок E→C→A), T-017 (F-10d — `replay.completed`, `abandoned`), §0 (F-6 +0,5), §9 п. 1. Структура подволн и состав задач не менялись.
**Правки ревизии контрактов T-416** (`contracts.md` v0.7; ADR-025 подтверждён, ADR-026, ADR-027; внесено tech-lead#1 2026-09-11): T-006 — пометка «таблица владения — единственная истина»; T-413 — пункты T-416 п. 15–16; §8 — `mvctl report`; §9 п. 4 — пометка. Новые задачи T-417 и T-418 завёл оркестратор. Добавленные пункты помечены «(T-416, 2026-09-11)», отменённые — «заменено (T-416)» и не удалены.
Команда TEAM-1 · ветка `epic/EPIC-001-foundation` (от `integration/mvp-1`, создана после F-0, коммит `744fb10`) · волна 0 · G2 утверждён 2026-09-09.
Основание: `epics/EPIC-001-foundation/design.md` v0.1 (§4 подволны, §5 заглушки v0, §10 тестируемость); `architecture/infrastructure.md` v0.2 (§2.1–§2.5, §3.1–§3.4, §4.5, §4.6, §6.4, §10 чек-лист); `architecture/components/foundation.md` v0.2 (§1–§12, §14); `architecture/contracts.md` v0.2 (C-01, C-02, C-03, C-04, C-05, C-06, C-13, C-14, §17); `plan/epics.md` v0.2 §6; `plan/decomposition-review.md` §2, §3.3, §5.3; `plan/teams.md` §4; `plan/ownership.md` v0.2; ADR-001, ADR-004, ADR-007, ADR-009, ADR-010, ADR-011, ADR-012, ADR-013, ADR-021.

Диапазон номеров TEAM-1: **T-001…T-199** (EPIC-001 — T-001…T-049, EPIC-002 — T-050…T-129, EPIC-005 — T-130…T-199).
Размер задачи: **S** ≤ полдня · **M** ≤ одной сессии одного разработчика · L — не допускается (дробится тимлидом).

---

## 0. Статус F-задач и отклонения от `epics.md` §6

| F | Задачи | Статус |
|---|---|---|
| **F-0** | — | **выполнено** 2026-09-09 (коммит `744fb10`): `git worktree prune`, 13 gitlink'ов `.claude/worktrees/*` из индекса, `.gitignore`, `Docs/dev-team/**` зафиксированы, `integration/mvp-1` от `feature/agent-gm-core`. Проверка: `git status` работает, `git branch --list integration/mvp-1` не пуст |
| F-1 | T-001 | **принята** 2026-09-09 (первая задача волны) |
| F-3 | T-002 | **принята** 2026-09-09 |
| F-2 | T-003 | **принята** 2026-09-09 (итерация 2) |
| **F-6 → F-6a / F-6b** | T-004, T-008 | **разбито тимлидом**: F-6 одной задачей = L (образ MinIO + compose + 5 профилей + два init-скрипта + Makefile 25 целей + `compose-lint`). F-6a — версии, образы, compose-ядро; F-6b — профили, init, Makefile, линтер compose. **Сведение 3 (`infrastructure.md` v0.3): объём F-6 вырос на ≈ 0,5 задачи** — `scripts/llm-server.ps1`/`.sh`, `make llm-up/llm-down/llm-health`, `extra_hosts` у `core`, `LLAMACPP_BUILD`/`LLM_MODEL_DEFAULT`/`MINIO_REPO` в `versions.env`, шестое правило `compose-lint`. Прирост распределён: T-004 (+`versions.env`, `extra_hosts`, форк MinIO), T-008 (+скрипты LLM, цели Makefile, правило линтера); дробление на третью задачу не требуется, обе остаются **M** Статус: T-004 и T-008 — **приняты** 2026-09-09 и 2026-09-10. |
| F-4a | T-005 | **принята** 2026-09-09 (итерация 2) |
| **F-4b → F-4b-1 / F-4b-2** | T-006, T-009 | разбито (подтверждает `decomposition-review.md` §2: «F-4b×2»): блоки «а+б» (player/group/round; entity/snapshot/dice) и блок «в» (agent/tick/llm/narrative/world/region/npc/laws/analytics) Статус: T-006 и T-009 — **приняты** 2026-09-10. |
| F-5 | T-007 | **принята** 2026-09-10 (итерация 2). **Уточнение**: `build/versions.env` создаётся в T-004 (F-6a, подволна 0.2, владелец `build/` — devops), а не в F-5 — иначе F-6a нечем пинить образы. F-5/F-5t файл только читают (`testkit.Versions()`). Отклонение от `design.md` §4 (строка 0.3) — согласовать с architect#1 |
| F-4c | T-010 | **принята** 2026-09-10 |
| F-7 | T-012 | **принята** 2026-09-10 (итерация 2) |
| F-8 | T-013 | **принята частично** 2026-09-10 — оснастка готова; замер на стенде и `baseline.md` перенесены в волну 1 (решение T-020) |
| F-5t | T-014 | **принята** 2026-09-11 (ревью #4) — **ворота волны 1 закрыты** |
| **F-10 → 5 задач** | T-011, T-015, T-016, T-017, T-018 | по указанию оркестратора: `entity` v2 · `mechanics` типы+`Load`+`rules` · фикстуры · `FakeState`+`FixedMechanics` · `Harness`+`FakeNarrator`+e2e. Между ними три последовательные группы зависимостей → подволны 0.4–0.7 Статус: T-011, T-015, T-016 — **приняты**; T-017 — **принята** 2026-09-11; T-018 — **принята** 2026-09-11 в сокращённом составе (см. карточку). |
| F-9 | T-019 | **принята** 2026-09-10 (tech-lead#1, приёмка #3; остаток — две Minor одной строкой каждая и стендовый прогон README владельцем, оба предусловия T-020) |
| — | T-020 | приёмка волны 0, слияние в `integration/mvp-1`, тег `mvp-1/wave-0` (tech-lead#1) |

Порядок (`epics.md` §6, `infrastructure.md` §10): **F-0 → F-1 ∥ F-3 → F-2 → (F-4a ∥ F-4b ∥ F-5 ∥ F-6) → (F-4c ∥ F-5t ∥ F-10 ∥ F-7 ∥ F-8) → F-9**.
Критический путь волны 0: **T-001 → T-003 → T-005 → T-014** (F-1 → F-2 → F-4a → F-5t).

**Изменения общих файлов** (`cmd/multiverse/main.go`, `cmd/mvctl/main.go`, `go.mod`, `docker-compose.yml`, `Makefile`, `.github/**`) после волны 0 — **только через tech-lead#1**; `shared/{eventbus,contracts,entity,clock,runtime}` и `schemas/events/_common.json` — через system-architect (пометка `contract-change`). Задачи с такой пометкой ниже помечены «⚠ только через tech-lead#1».

---

## 1. Общий DoD (применяется к каждой задаче; в задаче — только специфика)

1. `make lint` (`golangci-lint run`) чист; `gofmt`/`goimports` без диффа.
2. `go build ./... && go vet ./...` зелёные; `go test -short -race -count=1 ./...` зелёные.
3. Тесты написаны в той же задаче (уровни — по `design.md` §10); новый код без тестов не принимается.
4. `make secrets-scan` (gitleaks по диапазону ветки + рабочей копии) — 0 находок.
5. `Docs/dev-team/epics/EPIC-001-foundation/dev-log.md` заполнен: экземпляр (`developer#K`), задача `T-NNN`, что сделано, отклонения от дизайна, открытые вопросы.
6. Поведение соответствует критериям приёмки историй задачи (US-010, US-011, US-013, US-017 — по ссылкам в задаче) и `design.md` §10.
7. Изменений вне карты владения (`plan/ownership.md` §1) нет; контракты не меняются без запроса system-architect.
8. Начиная с T-012 (F-7) — **CI зелёный** на PR в `integration/mvp-1`: `unit`, `integration`, `e2e`, `contracts`, `security`, `compose-lint` ≤ 10 мин. До T-012 — эквивалент локально (`make ci`).

---

## 2. Подволна 0.1 — гигиена и архив (2 слота)

### T-001: F-1 · Гигиена индекса, gitleaks и pre-commit · Размер: S · Статус: принята 2026-09-09 · Исполнитель: developer#1 (пара — security-engineer) · Подволна 0.1
- **Описание**: выполнить `infrastructure.md` §4.5 п. 1–13 **в указанном порядке**: (1) `.gitleaks.toml` (allowlist только `^\.env\.example$`, `^\.mcp\.env\.example$` + очевидные плейсхолдеры); (2) `.pre-commit-config.yaml` (§3.4) → `pre-commit install` → `pre-commit run --all-files`; (3) **плейсхолдер ключа**: `shared/oracle/README.md` строки 14, 29 → `sk-xxxxxxxxxxxxxxxxxxxxxxxx`, строка 68 → `export ORACLE_API_KEY="<your-key>"` (U-5; делается **до** переноса файла в архив T-002); (4) `git rm --cached .mcp.env .claude/settings.local.json`; (5) `git rm --cached` + удаление с диска `examples.exe`, `semantic-memory.exe`, `services/narrative-orchestrator/cmd.exe`, `mcp_kafka.log`, `mcp_audit.log`; (6) пустые каталоги `-p/`, `Multiverse/`; (8) целевой `.gitignore`; (9) `.mcp.env.example`; (10) `.gitattributes` (`testdata/recordings/*.jsonl merge=binary` **без** `-diff`, `eol=lf` для `sh/yml/yaml/toml/Makefile`); (11) `.env.example` по §4.2 (полный, значения пустые).
- **Не входит**: `git filter-repo` (только по явной команде пользователя); перенос кода (`fake_deps`, `test_minio.go`, дубли `Dockerfile`) — T-002; решение по IDE/AI-каталогам и `event-model.pdf` — вопрос пользователю (см. §7).
- **Уточнение (сведение 3, `infrastructure.md` v0.3 §4.5 п. 13, U-11)**: вопрос закрыт — уже отслеживаемые `.idea/`, `.vscode/`, `.kilo*`, `.roo*`, `.qwen`, `memory/`, `plans/`, `reports/`, `shared/eventbus/docs/event-model.pdf` **из индекса не выводятся** (`git rm --cached` по ним в T-001 не выполняется, критерий их не проверяет); правила `.gitignore` касаются только новых файлов; окончательное решение по этим каталогам принимает tech-writer в T-019 (F-9). Секреты и бинарники (п. 4–5 описания) выводятся независимо.
- **Файлы**: `.gitleaks.toml`, `.pre-commit-config.yaml`, `.gitignore`, `.gitattributes`, `.env.example`, `.mcp.env.example`, `shared/oracle/README.md`.
- **Зависимости**: F-0 (выполнен).
- **Ссылки**: `design.md` §2 (US-013), §9 (T-16/U-5); `infrastructure.md` §4.5, §3.4; `foundation.md` §12; ADR-009 п. 8, доп. п. 10; US-013 (все три критерия); NFR-040; SEC/threat-model T-16.
- **(T-412, 2026-09-11)** Описание COMPOSE_ENV_FILES в манифесте shared/env/infra.go (строка около 21, «env files compose reads») вводит в заблуждение: compose читает эту переменную только из окружения своего процесса, не из .env. Поправить описание (вариант а) — или убрать переменную из манифеста и .env.example (вариант б, contract-change); исполнитель выбирает с обоснованием.
- **(приёмка T-412, 2026-09-11)** У MV_LLM_URL в .env.example нет пометки [required], хотя композиция требует её через :? (T-404), а CLAUDE.md велит заполнять только помеченное — оператор по инструкции её пропустит. Добавить пометку; проверить линтером или тестом, что каждая переменная с :? в композиции помечена [required] в .env.example.
- **DoD**:
  - `gitleaks dir --redact .` → 0 находок; `gitleaks git --redact --log-opts="integration/mvp-1..HEAD" .` → 0;
  - `git ls-files | grep -E '\.(exe|log)$|\.mcp\.env$|settings\.local\.json$'` → пусто;
  - `grep -n "sk-4659" shared/oracle/README.md` → пусто;
  - `pre-commit run --all-files` зелёный; **проверка хука на Windows**: `git commit` файла с `MV_TELEGRAM_BOT_TOKEN=123456789:AA…` (фиктивный) отклонён;
  - `git status` работает; `.env.example` содержит все переменные `infrastructure.md` §4.2 с префиксом `MV_`;
  - общий DoD §1.

### T-002: F-3 · `services/_archive/`, `FROZEN.md`, `Docs/archive/` · Размер: M · Статус: принята 2026-09-09 · Исполнитель: developer#2 (пара — tech-writer) · Подволна 0.1
- **Описание**: перенести выводимый из сборки код по `infrastructure.md` §4.6 **через `git mv`** (ничего не удалять, U-1): `services/{ban-of-world,reality-monitor}`, `shared/{schema,redis,config,minio,oracle,rules,intent,tinyml,spatial}`, `shared/agent/tools/*` (кроме реестра инструментов), `fake_deps/`, `test_minio.go`, корневой `Dockerfile` → `services/_archive/build/Dockerfile.root`, `services/{entity-actor,evolution-watcher,rule-engine}/Dockerfile` → `services/_archive/build/services/…`, `configs/gm_*.yaml`. В каждом каталоге — `ARCHIVED.md` (причина со ссылкой на ADR/`overview` §16, коммит архивации, последний рабочий коммит, эпик возврата, что использовало); `services/_archive/README.md` — индекс; `services/_archive/go.mod` (`module multiverse-core.io/archive`) и собственные `go.mod` подкаталогов, чтобы корневой `./...` их не видел. `FROZEN.md` в 8 замороженных сервисах (остаются на месте): world-generator, universe-genesis-oracle, ontological-archivist, cultivation-module, plan-manager, city-governor, entity-actor, evolution-watcher. Создать `Docs/archive/` с индексом.
- **Не входит**: `services/{narrative-orchestrator,semantic-memory}` — остаются (профиль `legacy` до S5, EPIC-003 I2); `entity-manager`/`rule-engine` — архивируются EPIC-002 (T-064) после переноса кода.
- **Файлы**: `services/_archive/**`, `services/<8 frozen>/FROZEN.md`, `Docs/archive/README.md`.
- **Зависимости**: T-001 (плейсхолдер ключа ставится до `git mv` `shared/oracle`).
- **Ссылки**: `infrastructure.md` §4.6, §10 (F-3); `foundation.md` §11, §12; `ownership.md` §1 (строка `services/_archive/**`); U-1 / OQ-A-17; `epics.md` §6 F-3.
- **DoD**:
  - `services/_archive/README.md` перечисляет **все** перенесённые пути с причиной и коммитом; в каждом каталоге есть `ARCHIVED.md`;
  - `ls services/*/FROZEN.md | wc -l` = 8;
  - `git log --follow` по перенесённому файлу читается (перенос, а не удаление+добавление);
  - `go build ./...` в корне (после T-003) не видит `services/_archive/**` — проверяется в DoD T-003;
  - общий DoD §1.

---

## 3. Подволна 0.2 — единый модуль и образы (2 слота)

### T-003: F-2 · Единый модуль Go 1.26, `cmd/multiverse`, `shared/runtime`, `shared/clock` · Размер: M · Статус: принята 2026-09-09 (итерация 2) · Исполнитель: developer#1 · Подволна 0.2 · ⚠ только через tech-lead#1 (`cmd/multiverse/main.go`, `go.mod`)
- **Описание**: удалить `go.work`/`go.work.sum`; корневой `go.mod` = `module multiverse-core.io`, `go 1.26`, `toolchain go1.26.x` (точный патч — из `build/versions.env`, T-004); слить shared-модули (`shared/{eventbus,jsonpath,entity,agent}`) в пакеты корневого модуля, удалив их `go.mod`; каркас `cmd/multiverse` — флаги `--contexts/--mode/--bus/--recording`, подкоманды `health --url` (для healthcheck distroless) и `db backup|check`; регистрация **пустых** контекстов `state|mechanics|swarm|llm|laws|gateway|memory` (`Health()=ok`, `Start/Stop` no-op) — по `design.md` §4.1; `shared/runtime` (HTTP-сервер процесса, `Deps`, `Context`, `Routes(mux)`, `AdminOnly`, `MV_CORE_ADDR=127.0.0.1:8090`); `shared/clock` (`Clock`, `Timers`, `Manual`, `ManualTimers`); `.golangci.yml` (`depguard` границ ADR-001 доп. п. 2, `forbidigo` на `os.Getenv`/`time.Now` вне `shared/*` и `log.Printf`, исключение `services/_archive/**` и замороженных); `build/Dockerfile` (§2.3 `infrastructure.md`).
- **Файлы**: `go.mod`, `go.sum`, `cmd/multiverse/**`, `shared/runtime/**`, `shared/clock/**`, `.golangci.yml`, `build/Dockerfile`, удаление `go.work*`.
- **Зависимости**: T-001, T-002.
- **Ссылки**: `foundation.md` §1, §2, §3, §4, §12 (F-2), §14 п. 2/3/8; `infrastructure.md` §2.1, §2.3, §10 (F-2); `design.md` §4.1; ADR-001 + дополнения п. 1–3; US-010 (критерий 1 частично), NFR-075.
- **DoD**:
  - `go mod tidy && go build ./... && go vet ./... && make lint` — зелёные; `go mod verify` ок;
  - `go run ./cmd/multiverse --contexts=all --bus=memory` → `curl 127.0.0.1:8090/health` = `{"status":"ok"}` со списком контекстов; `--contexts=state,gateway` поднимает только их;
  - `go build ./...` не собирает `services/_archive/**` и 8 замороженных сервисов (`go list ./... | grep -c _archive` = 0);
  - unit: `runtime.New` соблюдает порядок `DependsOn`; `clock.Manual`/`ManualTimers` детерминированы; `AdminOnly` отклоняет запрос без `X-Actor-Kind`;
  - `docker build -f build/Dockerfile .` собирает три бинарника (`multiverse`, `mvctl`; `telegram-bot` — заглушка `main` до EPIC-004 или исключён из `./cmd/...` с пометкой);
  - локально установлен Go 1.26 (зафиксировать версию в `dev-log.md`); общий DoD §1.

### T-004: F-6a · `build/versions.env`, образ MinIO из исходников, ядро compose · Размер: M · Статус: принята 2026-09-09 · Исполнитель: devops-engineer (выполняет developer#2 по инструкциям devops-engineer) · Подволна 0.2
- **Описание**: создать `build/versions.env` со всеми пинами `infrastructure.md` §2.4 (`GO_VERSION`, `REDPANDA_IMAGE`, `REDPANDA_CONSOLE_IMAGE`, `MINIO_TAG`, `MINIO_IMAGE`, `MINIO_BUILDER_IMAGE`, `MC_IMAGE`, `QDRANT_IMAGE`, `NEO4J_IMAGE`, `OLLAMA_IMAGE`, `CHROMA_IMAGE`, `GOLANGCI_LINT_VERSION`, `GITLEAKS_VERSION`); `build/minio.Dockerfile` (§2.5) и цель `make minio-image`; ядро `docker-compose.yml`: сервисы `redpanda`, `minio`, `ollama` (профиль `gpu`), образы через `${…}` из `versions.env` (`COMPOSE_ENV_FILES=.env,build/versions.env`), healthchecks, `depends_on: condition`, **все порты только `127.0.0.1`**, пароли только `${VAR:?}`, ротация логов `50m×5`; `.dockerignore`; тарбол исходников MinIO в `backups/minio-src-<tag>.tar.gz` (вне git).
- **Не входит**: профили `memory/dev/legacy/bot`, init-скрипты, Makefile целиком, `compose-lint` — T-008.
- **Дополнение (сведение 3, `infrastructure.md` v0.3 п. 1/5, U-8/U-10)**: в `build/versions.env` добавляются `MINIO_REPO=https://github.com/alekseizabelin1985-spec/minio.git` (форк владельца; upstream `https://github.com/minio/minio.git` — запасное значение аргумента), `LLAMACPP_BUILD=b<NNNN>` (значение вносится по установленной у владельца сборке, `llama-server --version`) и `LLM_MODEL_DEFAULT=Qwen3.8-27B-UD-Q3_K_XL` (**имя** модели, не путь); `build/minio.Dockerfile` клонирует форк (`ARG MINIO_REPO`, `MINIO_TAG=RELEASE.2025-10-15T17-29-55Z`); `make minio-image` передаёт `--build-arg MINIO_REPO`. В compose-ядре у сервиса `core` — `extra_hosts: ["host.docker.internal:host-gateway"]` (нативный `llama-server` на хосте `127.0.0.1:1234` вне compose, ADR-005 доп. 2 п. 7); сервиса `llama-server` в compose **нет** и профиля для него не заводим.
- **Предпосылка (вне задачи, блокирует DoD)**: **форк `minio/minio` в аккаунт `alekseizabelin1985-spec` делает владелец** (кнопка Fork на GitHub, однократно, до сборки образа) — агенты внешние системы не меняют. Если форка нет на момент старта — задача идёт на upstream-значении `MINIO_REPO`, факт фиксируется в `dev-log.md` и эскалируется tech-lead#1.
- **Файлы**: `build/versions.env`, `build/minio.Dockerfile`, `docker-compose.yml` (ядро), `.dockerignore`, `.github/ci.env` (нужен уже для `docker compose config -q`), `Makefile` (только цели `minio-image`, `image`).
- **Зависимости**: — (не зависит от кода; идёт параллельно T-003).
- **Ссылки**: `infrastructure.md` **v0.3** §2.4, §2.5, §1.4, §6, §12, §10 (F-6); ADR-021 (вариант B, решён на G2); ADR-005 доп. 2; `design.md` §7; NFR-071.
- **DoD**:
  - `make minio-image` собирается; `docker run --rm $MINIO_IMAGE --version` печатает `RELEASE.2025-10-15T17-29-55Z`; при провале сборки на `golang:1.26.x` — вторая попытка `golang:1.24.x`, выбранное значение зафиксировано в `versions.env` и в `dev-log.md` (ADR-021);
  - `docker compose --env-file build/versions.env --env-file .github/ci.env config -q` без ошибок;
  - `docker compose config --format json` — ни одной публикации порта без префикса `127.0.0.1`, ни одного `image` без явного тега и с `latest`; у сервиса `core` присутствует `extra_hosts: host.docker.internal:host-gateway`, сервиса `llama-server` в конфиге нет;
  - `mc admin info local` работает против собранного образа (`MC_IMAGE`);
  - **(сведение 3)** `docker build -f build/minio.Dockerfile --build-arg MINIO_REPO=<форк> …` собирается; `git ls-remote --tags <форк> | grep RELEASE.2025-10-15T17-29-55Z` не пуст; `backups/minio-src-<tag>.tar.gz` существует и распаковывается; `build/versions.env` содержит `MINIO_REPO`, `LLAMACPP_BUILD`, `LLM_MODEL_DEFAULT`;
  - тарбол исходников создан, путь записан в `dev-log.md`; общий DoD §1 (пункты про Go-тесты — n/a, отметить).

---

## 4. Подволна 0.3 — шина, контракты, объектное хранилище, профили (4 слота)

### T-005: F-4a · `shared/eventbus`: конверт `meta`, `Bus`, `Journal`, DLQ, kafka · Размер: M · Статус: принята 2026-09-09 (итерация 2) · Исполнитель: developer#1 · Подволна 0.3 · ⚠ общий код (изменения после волны 0 — через system-architect)
- **Описание**: конверт события C-01 v1.1 (`meta`: `event_id`, `correlation_id`, `causation_id`, `timestamp`, `source`, `actor_kind`, `agent{}`, `world`, `scope`), конструкторы `NewRoot`/`Derive` (наследование `CorrelationID` и `Timestamp` причины); интерфейсы `Bus{Publish, Subscribe}`, `Journal{ReadRange, Tail, End}`, `PositionFromContext`, `eventbus.Dedup` (LRU) — все в production-пакете; kafka-реализация на `segmentio/kafka-go v0.4.51` с reader-параметрами `MinBytes=1`, `MaxWait=100ms`; DLQ (`dead_letters`) после 3 повторов; политики топиков (`player_events` отклоняет `actor_kind=system` и `meta.agent`); валидация при чтении по `MV_BUS_VALIDATE_ON_READ`.
- **Файлы**: `shared/eventbus/**` (переиспользование по `foundation.md` §5.1), `shared/jsonpath/**` (перенос как есть).
- **Зависимости**: T-003.
- **Ссылки**: `foundation.md` §5.1–§5.3, §14 п. 4; `contracts.md` C-01 v1.1; `design.md` §2 (FR-036), §10; ADR-007; NFR-012, NFR-013.
- **DoD**:
  - unit: `NewRoot`/`Derive` (наследование `CorrelationID`/`Timestamp`, новый `event_id`), `Dedup` (повтор не проходит дважды), политики топиков на фикстурах, DLQ после 3 повторов;
  - `Journal.End` монотонен, `ReadRange` строго по возрастанию офсета (проверяется полностью в T-014 contract-тестом);
  - покрытие `shared/eventbus` ≥ 60 %;
  - общий DoD §1.

### T-006: F-4b-1 · `shared/contracts` + схемы блоков «а» и «б» · Размер: M · Статус: принята 2026-09-10 (итерация 2) · Исполнитель: developer#2 · Подволна 0.3 · ⚠ общий код

> **Решение tech-lead#1 о пересечении с TEAM-2/TEAM-3**: файлы схем **всех** типов §2.2 создаёт F-4b в волне 0 (так в `design.md` §4, `foundation.md` §12). Задачи владельцев — **ревизия и дополнение**, а не создание с нуля: EPIC-002 T-052, EPIC-003 T-214/T-215, EPIC-004 T-301. Если владелец обнаруживает расхождение с `api-contracts.md` — правит свой файл в своей ветке (это его владение по `ownership.md`), уведомляя tech-lead#1; `_common.json`/`_envelope.json` — только через system-architect.
- **Описание**: пакет `shared/contracts`: реестр `Spec{Type, Topic, Schema, Publishers, Consumers, Policy, Deprecated}`, `Lookup`, `Topics`, `Validate` на `santhosh-tekuri/jsonschema/v6` (JSON Schema 2020-12); статичная таблица `contracts.OwnershipRules` MVP-1 (`state-and-mechanics.md` §4.6; источник для сверки — `shared/agent/levels.go`, EPIC-003 I1a); `schemas/events/_common.json`, `_envelope.json`; **блок «а»** — `player.*`, `group.*`, `round.*`; **блок «б»** — `entity.create.proposed`, `entity.update.proposed`, `entity.created`, `entity.updated`, `entity.update.rejected`, `snapshot.created`, `dice.rolled`, `analytics.replay.completed` (формат по `state-and-mechanics.md` §4.4/§4.8, файлы передаются владельцу EPIC-002).
- **Дополнение (сведение 3, `contracts.md` v0.4 §0, §16 п. 6–7, C-02 v1.2; в `consolidation.md` §14.3 адресовано «F-4a» — по этой нарезке реестр и `OwnershipRules` живут в F-4b-1/T-006, F-4a/T-005 — только конверт и шина)**:
  - `Spec.Publishers` — **обязательное** поле реестра: «тип — владелец схемы и семантики; фактические издатели — список `Publishers`». Заполнить для типов с несколькими издателями: `dice.rolled` (владелец EPIC-002; издатели — агент встречи EPIC-003 и `testkit/swarm.FakeEncounter`), `entity.create.proposed`/`entity.update.proposed` (владелец EPIC-002; издатели — `gateway`, `swarm`, `mvctl` при `world init --fixtures`, фейки), `encounter.started` (EPIC-003; `region-gm` и `FakeEncounter`), `analytics.replay.completed` (`state` EPIC-002, харнесс EPIC-005, `testkit/state.FakeState`), `snapshot.created` (`state`, `swarm`, `gateway`). Источники `testkit/*` допустимы только на `membus`/`MV_SWARM_FAKE=true`.
  - `contracts.OwnershipRules` — статичная копия по `data-model.md` §4, **включая строку gateway**: `Character.status: alive → abandoned` (и только этот переход) и `Group.leader_id` (включая значение `null`). Истина — `shared/agent/levels.go` (EPIC-003, T-202), тест равенства копии и истины блокирует merge; правка копии без истины запрещена (§16 п. 6).
    - *Пометка (T-416, 2026-09-11; ADR-025, C-02 v1.4, `contracts.md` §16 п. 6).* Задача сделана, строка выше и слова «статичная таблица… источник для сверки — `levels.go`» в «Описании» оставлены для истории. Действующее правило: `contracts.OwnershipRules` в `shared/contracts/ownership.go` — **единственная истина (ADR-025)**. Копии и теста равенства нет. Строки меняет владелец их семантики: PR с меткой `contract-change`, ревью system-architect и tech-lead#1. `shared/agent/levels.go` (T-202) своей таблицы владения не держит.
  - Схемы блока «б»: в `entity.update.proposed`/`entity.updated` поле `cause` дополнено значением **`forget`** (C-02 v1.2); `analytics.replay.completed` — поля по C-14 v0.4 (`mode`, `replay{run_id, snapshot_id?, events_replayed, llm_calls, dice_rolled_new, duration_ms, state_hash_after, incomplete_record}`).
- **Файлы**: `shared/contracts/**`, `schemas/events/_common.json`, `_envelope.json`, `schemas/events/{player,group,round,entity,snapshot,dice,analytics.replay}*.v1.json`.
- **Зависимости**: T-003 (может стартовать параллельно T-005 от `analysis/api-contracts.md` §2.2).
- **Ссылки**: `foundation.md` §6, §14 п. 5; **`contracts.md` v0.4** §0, §16 п. 6–7, C-01, **C-02 v1.2**, C-13, C-14 (уточнение v0.4); `consolidation.md` §14.1 (TL2-2, TL2-3, З-2); `analysis/api-contracts.md` §2.2, §2.3.4/5/12; `design.md` §6; ADR-007, ADR-013.
- **DoD**:
  - `go test ./shared/contracts/... -run TestSchemasValid` — все схемы компилируются как JSON Schema 2020-12;
  - каждый пример из `api-contracts.md` §2.3 для блоков «а»/«б» валиден против своей схемы (тест на таблице примеров);
  - каждый `Spec` имеет файл схемы и наоборот (тест «нет фантомов»);
  - `OwnershipRules` покрывает уровни `global/domain/encounter/personal/system/author` + пустые `object/monitor` (C-13) и **строку gateway** (`Character.status → abandoned`, `Group.leader_id` с `null`);
  - **(сведение 3)** у каждого `Spec` непустой `Publishers`; тест на таблице типов с несколькими издателями; `cause` в схемах `entity.*.proposed`/`entity.updated` содержит `forget`;
  - покрытие `shared/contracts` ≥ 60 %; общий DoD §1.

### T-007: F-5 · `shared/objstore`, `shared/env`, `shared/logging` · Размер: M · Статус: принята 2026-09-10 (итерация 2) · Исполнитель: developer#3 · Подволна 0.3
- **Описание**: `shared/objstore` по ADR-021 — минимальный интерфейс (`Put/Get/Stat/List/Delete/EnsureBucket/Capabilities`), **две реализации сразу**: `Memory` (нужна EPIC-002 для unit-тестов State) и `minio` на `minio-go/v7 v7.3.0`; `EnsureBucket` сам включает versioning и ILM по `BucketOptionsFor` (`prompts-*` — без versioning, ILM 30 дн.); `buckets.go` с именами `entities-*`, `snapshots-*`, `prompts-*`, `ops-artifacts`; `shared/env` — `Declare`-манифест с обязательным префиксом `MV_`, `DeclareExternal` для сторонних, сверка с `.env.example`; `shared/logging` — slog с обязательными полями, `BusMiddleware`, редакция внешних ID и токенов.
- **Дополнение (сведение 3, `infrastructure.md` v0.3 §4.2 п. 1, ADR-005 доп. 2)** — манифест `shared/env` и `.env.example`:
  - LLM-блок объявляется под провайдер по умолчанию `openai_compat`: `MV_LLM_PROVIDER` (`openai_compat | ollama | anthropic | recorded | fake`, default `openai_compat`), `MV_LLM_URL` (`http://host.docker.internal:1234/v1` для контейнеров), `MV_LLM_API_KEY`, `MV_LLM_MODELS_DIR`, `MV_LLM_CLOUD_ENABLED`;
  - `OLLAMA_*` (сторонние, без префикса `MV_`) — **обязательны только при `MV_LLM_PROVIDER=ollama`**; `Declare` поддерживает условную обязательность (проверка в `mvctl env check`, T-010), при других провайдерах отсутствие `OLLAMA_*` — не ошибка;
  - `MV_OPENAI_API_KEY` и `MV_DEEPSEEK_API_KEY` **удалены** (были в v0.2): `openai`/`deepseek` — не имена провайдера, а другой `MV_LLM_URL` + `MV_LLM_API_KEY` у той же реализации; `mvctl env check` сообщает о них как об устаревших, если остались в чьём-то `.env`.
- **Файлы**: `shared/objstore/**`, `shared/env/**`, `shared/logging/**`.
- **Зависимости**: T-003.
- **Ссылки**: `foundation.md` §7, §8, §14 п. 1/6; `infrastructure.md` **v0.3** §5.2, §4.1, §4.2, §7.1, §10 (F-5); ADR-004 доп. п. 2, ADR-021 п. 1–2, ADR-005 доп. 2; `contracts.md` v0.4 C-15 v1.1, раздел Env; `design.md` §8; NFR-074, NFR-041.
- **DoD**:
  - unit: `objstore.Memory` полностью (все методы + `Capabilities()`); `env.Declare` отклоняет имя без `MV_`; `logging` — тест «внешний ID и токен не попадают в лог»;
  - **(сведение 3)** unit: при `MV_LLM_PROVIDER=openai_compat` отсутствие `OLLAMA_*` не даёт ошибки, при `ollama` — даёт; `.env.example` не содержит `MV_OPENAI_API_KEY`/`MV_DEEPSEEK_API_KEY` и содержит блок `MV_LLM_*`;
  - integration (`-tags integration`, testcontainers, образ `${MINIO_IMAGE}` из T-004): `Put/Get/Stat/List/Delete`, `EnsureBucket` включает versioning + ILM, `prompts-*` без versioning, `Capabilities()` = `{Versioning:true, Lifecycle:true}`;
  - `.env.example` дополнен всеми объявленными переменными (сверку автоматизирует T-010);
  - покрытие ≥ 60 %; общий DoD §1.

### T-008: F-6b · Профили compose, `redpanda-init`/`minio-init`, Makefile, `compose-lint` · Размер: M · Статус: принята 2026-09-10 (итерация 2) · Исполнитель: devops-engineer (выполняет developer по инструкциям devops) · Подволна 0.3
- **Описание**: профили `gpu`, `memory`, `dev`, `legacy` (as-is `narrative-orchestrator` + `semantic-memory` :8083 + `chromadb` с зафиксированным тегом — D-3), `bot`; `build/redpanda-init.sh` — 8 топиков с retention 30/90/180 дн. и `segment.ms=1d` (U-3, D-9); `build/minio-init.sh` — бакеты и versioning/ILM; Ollama env (`OLLAMA_KEEP_ALIVE=-1`, `OLLAMA_MAX_LOADED_MODELS=2`, `OLLAMA_NUM_PARALLEL=1`, `OLLAMA_ORIGINS` не `*`); Neo4j без `NEO4J_PLUGINS` (T-17); токен бота — только у сервиса `telegram-bot`; полный `Makefile` (`infrastructure.md` §2.2, включая `up/down/health/ci/ci-full/replay/archive-legacy/models/warm/bench/backup/restore/logs/clean`), `SHELL := bash`, `include build/versions.env`, `GOFLAGS=-buildvcs=false` для локальных целей; `scripts/compose-lint.sh` (§3.1.1, 5 проверок); **решение по жизнеспособности профиля `legacy`** — до старта волны 1, при провале зафиксировать запасной вариант S5 в журнале и уведомить tech-lead#2.
- **Дополнение (сведение 3, `infrastructure.md` v0.3 п. 1, §6.2, §2.2, §3.1.1; +0,5 к объёму F-6 — см. §0)**: `scripts/llm-server.ps1` (основной, Windows) и `scripts/llm-server.sh` (WSL/Linux) — старт нативного `llama-server` (llama.cpp) на `127.0.0.1:1234` с зафиксированными параметрами и ожиданием `/health = 200` до 180 с, повторный вызов при живом процессе — no-op с сообщением; цели `make llm-up` / `make llm-down` / `make llm-health`; `make up` процесс **не поднимает**, только проверяет health (LLM вне compose); в `scripts/compose-lint.sh` — **дополнительное правило**: сервиса `llama-server` в compose нет, а `MV_LLM_URL` контейнеров указывает на `host.docker.internal` либо на сервис внутри сети, но не на публичный адрес.
- **Файлы**: `docker-compose.yml`, `build/redpanda-init.sh`, `build/minio-init.sh`, `build/legacy.Dockerfile`, `Makefile`, `scripts/compose-lint.sh`, `scripts/llm-server.ps1`, `scripts/llm-server.sh`, `.github/ci.env` (дополнение).
- **Зависимости**: T-004.
- **Ссылки**: `infrastructure.md` **v0.3** §1.3, §1.4, §2.2, §3.1.1, §5.1, §5.2, §6, §6.2, §10 (F-6); `foundation.md` §10; ADR-005 доп. 2 п. 1/7; D-3, D-9, D-10, U-3, U-8; T-3, T-13, T-17 (threat-model); NFR-070, NFR-071.
- **DoD**:
  - `make compose-lint` зелёный (5 проверок §3.1.1 **+ правило про `llama-server`/`MV_LLM_URL`**); `git grep -i minioadmin` пуст вне `Docs/` и `services/_archive/`;
  - **(сведение 3, стенд)** `make llm-up` поднимает `llama-server` и дожидается `/health = 200`; повторный `make llm-up` — no-op; `make llm-health` печатает статус (503 = loading — не ошибка); `make llm-down` останавливает процесс; `make up` не пытается запустить LLM;
  - `docker compose --env-file build/versions.env --env-file .github/ci.env config -q` для каждого профиля;
  - **стенд**: `make up` на машине владельца → `make health` = ok для `core`/`gateway` (пустые контексты из T-003); `rpk topic list` показывает 8 топиков с ожидаемыми `retention.ms`/`segment.ms`; `make up PROFILES=legacy` поднимает orchestrator + semantic-memory :8083 + chroma **или** в журнал внесено решение о запасном варианте S5;
  - `make down` сохраняет тома; общий DoD §1 (Go-тесты — n/a).

---

## 5. Подволна 0.4 — схемы блока «в», `mvctl`, `entity` v2, CI, замер (4 слота + стенд)

### T-009: F-4b-2 · Схемы блока «в» и политики топиков · Размер: M · Статус: принята 2026-09-10 · Исполнитель: developer#2 · Подволна 0.4 · ⚠ общий код
- **Описание**: схемы `agent.*` (lifecycle), `tick.*`, `llm.output`, `llm.output.rejected`, `narrative.output`, `world.*` (включая `world.law_breach.*` как зарезервированные), `region.*`, `npc.*`, `laws.*`, `analytics.session.*`, `analytics.turn.completed`, **`analytics.consistency.violated`** (издатель EPIC-005 — по запросу §12 п. 2 дизайна EPIC-005); заполнение `Publishers/Consumers/Policy` в реестре для всех типов §2.2 `api-contracts.md`; исключения проверки «издатель + ≥1 потребитель» — `world.law_breach.*`, `rules.change.*`.
- **Файлы**: `schemas/events/*.v1.json` (блок «в»), `shared/contracts/registry.go`.
- **Зависимости**: T-006.
- **Ссылки**: `foundation.md` §6; `contracts.md` §0, C-05…C-07, C-10, C-12; `epics/EPIC-005-memory-ops/design.md` §12 п. 2; `design.md` §4.1 (проверки `contracts check`).
- **DoD**: все схемы блока «в» компилируются и покрывают примеры `api-contracts.md`; реестр полон (проверка «нет фантомов» проходит в T-010); `analytics.consistency.violated` содержит `code ∈ {state_divergence, log_gap, invariant}`, `severity ∈ {break, warn}`, `detected_by`; общий DoD §1.

### T-010: F-4c · Каркас `cmd/mvctl` + `contracts check/topics`, `env check`, `storage init` · Размер: S · Статус: принята 2026-09-10 · Исполнитель: developer#1 · Подволна 0.4 · ⚠ только через tech-lead#1 (`cmd/mvctl/main.go` — реестр подкоманд)
- **Описание**: `cmd/mvctl/main.go` — таблица `name → func(args) int` на стандартном `flag` (без cobra); подкоманды EPIC-001 в `cmd/mvctl/internal/{contracts,env,storage}`: `contracts check` (проверки (а)–(д) `design.md` §4.1, наполнение — EPIC-005 T-138), `contracts topics --format=rpk`, `env check` (`.env.example` ↔ манифест `shared/env`), `storage init` (`EnsureBucket(ops-artifacts)` + `Capabilities()`; страховка от исчезновения `minio/mc`, ADR-021 п. 1). Зарезервировать в реестре имена подкоманд других эпиков (`world`, `blueprint`, `laws`, `record`, `report`, `trace`, `llm`, `golden`, `memory`, `privacy`) с заглушкой «не реализовано» и владельцем в комментарии.
- **Файлы**: `cmd/mvctl/main.go`, `cmd/mvctl/internal/{contracts,env,storage}/**`.
- **Зависимости**: T-006, T-007, T-009.
- **Ссылки**: `design.md` §4.1; `foundation.md` §6, §12; `decomposition-review.md` §5.2 п. 4; `ownership.md` §1 (`cmd/mvctl`).
- **DoD**: `go run ./cmd/mvctl contracts check` — 0 фантомных типов/топиков; `go run ./cmd/mvctl env check` зелёный на `.env.example`; `go run ./cmd/mvctl storage init` создаёт `ops-artifacts` на `objstore.Memory` и на MinIO (integration); `make contracts` зелёный; unit на синтетическом реестре с подложенным фантомом — команда возвращает ≠ 0; общий DoD §1.

### T-011: F-10a · `shared/entity` v2 (модель сущности) · Размер: M · Статус: принята 2026-09-10 (итерация 2) · Исполнитель: developer#3 (пара — architect#1) · Подволна 0.4 · ⚠ общий код (с волны 1 владелец — EPIC-002)
- **Описание**: полная модель по `state-and-mechanics.md` §3 (не заглушка): `Entity`, `LastChange`, `HistoryEntry` (обрезка 50), `Op{set,inc,append,remove}`, `ApplyOps` (все ошибки и no-op), `Clone`, `CanonicalJSON`, `StateHash`, типизированные геттеры `attrs.go` для всех атрибутов `data-model.md` §3, константы типов `types.go`, `Ref` ⇄ `eventbus.EntityRef`.
- **Файлы**: `shared/entity/**`.
- **Зависимости**: T-005 (`Ref` ⇄ `eventbus.EntityRef`), T-003.
- **Ссылки**: `design.md` §5 (строка `shared/entity` v2); `state-and-mechanics.md` §3, §11; `contracts.md` C-02 v1.1; ADR-013; `decomposition-review.md` §3.3.
- **DoD**: unit покрывает `ApplyOps` по всем операциям, ошибкам и no-op; `StateHash` стабилен между запусками и не зависит от порядка ключей (тест на перемешанном `map`); `CanonicalJSON` детерминирован; покрытие `shared/entity` ≥ 60 %; сигнатуры совпадают с §3 `state-and-mechanics.md` (сверка architect#1 в ревью); общий DoD §1.

### T-012: F-7 · CI `.github/workflows/go.yml` и hardening · Размер: M · Статус: принята 2026-09-10 (итерация 2) · Исполнитель: devops-engineer (выполняет developer по инструкциям devops) · Подволна 0.4
- **Описание**: workflow с шестью job'ами (`unit`, `integration`, `e2e`, `contracts`, `security`, `compose-lint`) + `image` (без push) по `infrastructure.md` §3.1; общий шаг `cat build/versions.env >> $GITHUB_ENV`; `permissions: {contents: read}` на workflow, `security-events: write` только у `security`; все `uses:` — по SHA с комментарием версии; `go mod verify`, `go mod tidy -diff`; `govulncheck` блокирующий (до перевода `go.mod` на 1.26 — `continue-on-error`, снимается тем же PR); `privacy-scan` (`mvctl privacy scan testdata/` — заглушка команды до EPIC-005 T-139: сейчас проверка числовых внешних ID и username в `testdata/`); `scripts/coverage-gate.sh 60 internal/state internal/mechanics internal/swarm internal/llm internal/replay` (отсутствующие пакеты — предупреждение); `.github/dependabot.yml`, `.github/CODEOWNERS`, `.github/ci.env`; удалить `validate-blueprints.yml`; не трогать `qwen-*.yml`; branch protection на `main` и `integration/mvp-1` (required checks — шесть job'ов).
- **Дополнение (сведение 3, `infrastructure.md` v0.3 п. 4, §3.2, U-9)**: GitHub-логин владельца — **`@alekseizabelin1985-spec`** (вопрос закрыт), базовое правило `CODEOWNERS` — на весь репозиторий (`*`), «контрактные» каталоги перечислены отдельными строками; `services/_archive/**` из CODEOWNERS исключён. Репозиторий приватный личный → `gitleaks-action` **без** `GITLEAKS_LICENSE`, лимит Actions 2 000 мин/мес (`paths-ignore` + `concurrency` обязательны). Branch protection: «Require review from Code Owners» включается **только на `main`**; на `integration/mvp-1` — required checks (шесть job'ов), линейная история, без force-push, **апрув Code Owners не требуется** (единственный человек-владелец не может апрувить собственный PR и заблокировал бы сам себя).
- **Файлы**: `.github/workflows/go.yml`, `.github/dependabot.yml`, `.github/CODEOWNERS`, `scripts/coverage-gate.sh`, удаление `.github/workflows/validate-blueprints.yml`.
- **Зависимости**: T-008, T-010 (для job `contracts`), T-007 (integration), T-003 (e2e «пустой мир»).
- **Ссылки**: `infrastructure.md` **v0.3** §3.1, §3.1.1, §3.2, §12, §10 (F-7); `design.md` §9, §10; ADR-010 + дополнения; T-15 (threat-model); NFR-062, NFR-063.
- **DoD**: PR в `integration/mvp-1` → все шесть job'ов зелёные, суммарно ≤ 10 мин; второй прогон `integration` берёт образ MinIO из кэша `type=gha` ≤ 1 мин; `CODEOWNERS` содержит `* @alekseizabelin1985-spec` и покрывает `shared/`, `schemas/`, `.github/`, `build/`, `blueprints/`, `laws/`, `config/`, `docker-compose.yml`, `Makefile`; branch protection включена (скриншот/лог в `dev-log.md`): на `main` — Code Owners review, на `integration/mvp-1` — **только** required checks; общий DoD §1.

### T-013: F-8 · Матрица замера LLM и `ops/metrics/baseline.md` · Размер: M · Статус: принята частично 2026-09-10 — оснастка; замер на стенде и заполнение `baseline.md` перенесены в волну 1 (T-020, `review.md` «Приёмка волны 0» §1) · Исполнитель: architect#1 + пользователь (**стенд**, слот разработчика не занимает) · Подволна 0.4–0.5
- **Описание**: `scripts/llm-bench.ps1` (основной) и `scripts/llm-bench.sh` (WSL), `ops/metrics/bench-matrix.json`, `ops/models.txt`, `testdata/bench/prompts.jsonl` (10 ситуаций «Тёмного леса» × фазы `tick`/`phase2`/`phase2-group3`, схемы из `schemas/agent/`); три прогона конфигураций C и A; `ops/metrics/baseline.md` с решением по U-2.
- **Дополнение (сведение 3, `infrastructure.md` v0.3 §6.4, U-8/U-12)**: замер идёт против `llama-server` (`/v1/chat/completions`, `response_format json_schema`, `chat_template_kwargs.enable_thinking=false`), метрики — из `usage`/`timings` ответа + `nvidia-smi`; `bench-matrix.json` содержит поле `provider` и конфигурации **E / E+ / C / A**; **порядок прогона: сначала E** (базовый кандидат, `num_ctx` 8192/16384, KV `f16`/`q8_0`), при `verdict=fail` — **C**, затем **A**; в CSV обязательны колонки `llamacpp_build` и имя модели (`LLM_MODEL_DEFAULT`). Каждая запись `testdata/bench/prompts.jsonl` содержит `schema_name`, `schema` (объект схемы из `schemas/agent/`, подставляется в `json_schema.schema`, `strict: true`) и `max_tokens`; файл поставляет architect#1. Замер **ручной** на машине владельца (self-hosted runner не заводим, U-12).
- **Файлы**: `scripts/llm-bench.{ps1,sh}`, `ops/metrics/bench-matrix.json`, `ops/models.txt` (секции `gguf` и `ollama`), `testdata/bench/prompts.jsonl`, `ops/metrics/bench-<date>.csv`, `ops/metrics/baseline.md`.
- **Зависимости**: T-008 (`make llm-up`, `make models`, `make warm`).
- **Ссылки**: `infrastructure.md` **v0.3** §6.3, §6.4, §12, §10 (F-8); `design.md` §3 п. 6; ADR-005 доп. 2; OQ-A-18 / U-2, U-8, U-12; NFR-002, NFR-076, NFR-090.
- **DoD**: `make llm-up && pwsh scripts/llm-bench.ps1 -Configs E` → `ops/metrics/bench-<date>.csv` со всеми колонками §6.4, колонкой `llamacpp_build` и `verdict` для E; при `fail` — прогон C, затем A; три прогона в разные моменты; каждая строка `prompts.jsonl` валидна (`schema_name`/`schema`/`max_tokens`, схема компилируется как JSON Schema 2020-12); `baseline.md` содержит решение по U-2, VRAM/RAM и `stack_up_time_s`; результат передан BA для порогов NFR (EPIC-005 T-140). **Не блокирует волну 1** — при переносе на волну 1 отметить в журнале.

---

## 6. Подволны 0.5–0.7 — testkit, заглушки v0, документация

### T-014: F-5t · `shared/testkit` ядро и **contract-тест шины** · Размер: M · Статус: принята 2026-09-11 (ревью #4; ворота волны 1 закрыты) · Исполнитель: developer#1 · Подволна 0.5 · **ВОРОТА ВОЛНЫ 1**
- **Описание**: `membus` (in-memory `Bus` + `Journal` с той же семантикой, что kafka), `Dedup`-псевдоним, `Versions()` (чтение `build/versions.env`), `containers.go` (testcontainers: redpanda, MinIO — наш образ, qdrant, neo4j из `Versions()`); **contract-тест шины** — один набор тестов против `membus` (`-short`) и redpanda (`-tags integration`).
- **Файлы**: `shared/testkit/{membus,containers,versions,contract}/**`.
- **Зависимости**: T-005, T-006, T-007, T-004.
- **Ссылки**: `foundation.md` §9, §12, §14 п. 7; `design.md` §3 п. 3, §10 (строка contract-тест); `contracts.md` C-01 v1.1, §17; ADR-010; NFR-063.
- **DoD**:
  - contract-тест проходит на **обеих** реализациях: порядок в топике; at-least-once (дубли по `--chaos=duplicate`); retry ×3 → `dead_letters`; `Journal.ReadRange` строго по возрастанию; `End` монотонен; `Tail` живёт до `ctx.Done()`; `PositionFromContext`; невалидное событие при чтении → DLQ без вызова handler;
  - `make test-integration` зелёный локально (Docker Desktop) и в CI job `integration`;
  - `testkit.Versions()` возвращает пины из `build/versions.env` (тест на расхождение с файлом);
  - **ворота**: без зелёного contract-теста волна 1 не стартует — tech-lead#1 фиксирует результат в `journal.md`;
  - общий DoD §1.

### T-015: F-10b · `internal/mechanics` типы, `Load`, формулы, RNG, `rules/dark-forest.yaml` v0.1 · Размер: M · Статус: принята 2026-09-10 (ревью #2) · Исполнитель: developer#2 (пара — architect#1) · Подволна 0.5
- **Описание**: `rules.go` (`RulesDocument`, `Load`, `LoadBytes`, валидация §5.2), `formula.go` (парсер мини-грамматики `check`/`dice` §5.3 — нужен для валидации при `Load`), `rng.go` (`Seed`, `NewRNG`, `Roll`), типы C-03: `Actor/Action/Outcome/Item/Roll/Invariant/StateView/Violation/ProposedChange`, `Stats`, `DiceRolledPayload`, `ActorFromEntity`; `Invariants()` — 10 записей с `Check=nil`; `Resolve`, `NPCTarget`, `ChangesFor` возвращают `mechanics.ErrNotImplemented`; `rules/dark-forest.yaml` v0.1 с числами приложения A PRD.
- **Не входит**: логика `Resolve`/`NPCTarget`/`ChangesFor` и реализации `Check` — EPIC-002 I1 (T-053, T-054).
- **Файлы**: `internal/mechanics/{rules,formula,rng,types,actor,dice_event}.go`, `rules/dark-forest.yaml`.
- **Зависимости**: T-011.
- **Ссылки**: `design.md` §5 (строка `internal/mechanics`); `state-and-mechanics.md` §5.2, §5.3, §5.4; `contracts.md` C-03 v1.1; ADR-012; NFR-060.
- **DoD**: `Load` на `rules/dark-forest.yaml` — золотые числа приложения A; негативные тесты валидации (неизвестная формула, отрицательный dmg, отсутствующий `entities.*`); `Seed` — фиксированные векторы (детерминизм); `NdM+K` на одном RNG; вызов `Resolve` даёт `ErrNotImplemented`; покрытие `internal/mechanics` ≥ 60 %; общий DoD §1.

### T-016: F-10c · Фикстуры мира и `latest.json` seq 0 · Размер: S · Статус: принята 2026-09-10 · Исполнитель: developer#1 · Подволна 0.6
- **Описание**: `testdata/fixtures/{world,region,npc,players}.json` по `state-and-mechanics.md` §4.10 (`dark-forest-world`; `dark-forest-01` с `encounter_chance`, `respawn_ttl: 24h`; `wolf-alpha`; `player-A/B/C` с `actor_kind=ci`, `position=outside:dark-forest-world`, `scope=solo:{id}`) и `testdata/fixtures/snapshots/state/latest.json` — указатель на снапшот seq 0 (формат §4.4, `entities_count: 6`, `cursor.system_events: 0`, `state_hash` пересчитывается тестом).
- **Файлы**: `testdata/fixtures/**`.
- **Зависимости**: T-011, T-015 (сверка статов с `Rules.Stats`).
- **Ссылки**: `design.md` §5 (строка «Фикстуры»), §7; `state-and-mechanics.md` §4.4, §4.10; `data-model.md` §3; `contracts.md` C-14 v1.1; `ownership.md` §1 (`testdata/fixtures/**`).
- **DoD**: тест сверяет `hp_max/atk/def/dmg` фикстур с `mechanics.Rules.Stats(kind)` — расхождений нет; `latest.json` валиден против `snapshot.created.v1.json`/формата §4.4, `state_hash` пересчитывается в тесте (в файле — вычисленное значение); **privacy-scan** по `testdata/` зелёный (нет внешних ID, username, реальных имён); общий DoD §1.

### T-017: F-10d · `testkit/state.FakeState` v0 и `testkit/mechanics.FixedMechanics` · Размер: M · Статус: принята 2026-09-11 (ревью #2) · Исполнитель: developer#2 · Подволна 0.6
- **Описание**: `FakeState` v0 — `memstore` + подписка `membus` на `entity.*.proposed` в `system_events`; применяет `ApplyOps` на копии, `version+1`, публикует `entity.created/updated` через `Derive` с `timestamp` предложения и общим `proposal_id`; отказы только `unknown_entity`, `version_conflict`, `invalid_op`, `duplicate_entity`; `atomic=true` — всё или ничего в памяти; `Seed(entities)` из фикстур; `Snapshot()` → объект + `latest.json` в `objstore.Memory`; `WithInvariants()` — no-op с логом. `FixedMechanics` — тот же набор методов, что у `*mechanics.Rules`; `Resolve` — таблица исходов по `(Seed(causeEventID, rollIndexStart) mod N)`: 60 % попадание d6, 10 % крит, 10 % фамбл, 20 % промах; `NPCTarget` — первый живой кандидат по `id`; `Stats` из `rules/dark-forest.yaml`.
- **Дополнение (сведение 3, `contracts.md` v0.4 C-14 уточнение и C-02 v1.2; TL2-6, З-2)**:
  - **`FakeState` v0 публикует `analytics.replay.completed`** сразу после подписки на `system_events`: `{mode: recovery, replay{run_id, snapshot_id: null, events_replayed: 0, llm_calls: 0, dice_rolled_new: 0, duration_ms, state_hash_after, incomplete_record: false}}`, `source = testkit/state` (значение из реестра `Spec.Publishers`, T-006). Заглушка соблюдает протокол старта — потребители (рой T-237, gateway) ждут сигнал, а не подстраиваются под заглушку.
  - **Приём `status=abandoned`**: `entity.update.proposed {atomic: true, cause: forget}` с `set path=status value=abandoned` от `source=gateway` — принимается **только** если текущий `status = alive` (факт `entity.updated {changed:[{path: status, old: alive, new: abandoned}], cause: forget}`); над `status ∈ dead | abandoned | ascended_final` — отказ `dead_entity`; `abandoned` — терминальный (повторное предложение → `dead_entity`). `narrative.output kind=death` заглушка не издаёт. Причина `dead_entity` добавляется к четырём существующим (`unknown_entity`, `version_conflict`, `invalid_op`, `duplicate_entity`).
- **Файлы**: `shared/testkit/state/**`, `shared/testkit/mechanics/**`.
- **Зависимости**: T-014, T-015, T-011.
- **Ссылки**: `design.md` §5 (строки `FakeState` v0, `FixedMechanics`), §11 (отклонённая альтернатива «интерфейс в C-03»); `state-and-mechanics.md` §4.5, §5.1; `contracts.md` **v0.4** C-02 v1.2, C-03 v1.1, C-14 (уточнение v0.4), §17; `consolidation.md` §14.1 (TL2-6, З-2).
- **DoD**: unit — цикл «предложение → факт» на membus для каждого типа op; матрица причин отказа (`unknown_entity`, `version_conflict`, `invalid_op`, `duplicate_entity`, **`dead_entity`**); `atomic=true` откатывает пакет целиком; `Snapshot()` кладёт объект и `latest.json` в `objstore.Memory` в формате §4.4; **(сведение 3)** тест «при старте `FakeState` в шине появляется `analytics.replay.completed mode=recovery`, валидное против схемы из T-006»; тест «`set status=abandoned cause=forget` от gateway: `alive` → `entity.updated`, `dead`/`abandoned` → `entity.update.rejected reason=dead_entity`»; сигнатуры совпадают с C-02/C-03 (замена на реализацию EPIC-002 не требует правок у потребителей — проверяется компиляцией теста-потребителя); общий DoD §1.

### T-018: F-10e · `testkit/gateway.Harness` v0, `testkit/swarm.FakeNarrator` v0 и e2e «заглушки v0» · Размер: M · Статус: принята 2026-09-11 в сокращённом составе (без `Attack`/`Flee`, боевого сценария и видов `turn`/`death`/`world_event`; расширение `Harness` — задача бридж-блока волны 1, `review.md` «Приёмка волны 0» §2) · Исполнитель: developer#1 · Подволна 0.7
- **Описание**: `Harness` v0 — генератор `player.*` из фикстур в membus **без HTTP**: `NewHarness(bus, fixtures)`, `CreatePlayer`, `Enter`, `Look`, `Attack`, `Flee`, `Rest`, `Say`, `Leave`, `Scenario("solo-30")`; `meta.actor_kind=ci`, `correlation_id = id`, `source=gateway`. `FakeNarrator` v0 — подписки: `combat.decided` → `narrative.output kind=turn`, `round.closed` → `kind=round`, `player.looked`/`player.entered_region` → `kind=entry`; `generated_by=template`, ≤ 10 русских шаблонов с подстановкой `hp`/`damage`/имени, `recipients[]` игроков scope, `filter{applied:false,status:pass,filter_version:"none"}`, `laws_version:"v1"`, `meta.agent{id:"fake-narrator", level:"task", …}`; **`WithEncounterStub(rules)`** (`design.md` §5.1) — на `player.attacked` вызывает `Resolve`, публикует `dice.rolled` → `combat.decided` → `entity.update.proposed` одним atomic-пакетом, отвечает ударом NPC через `NPCTarget`, издаёт `encounter.started`/`encounter.ended`; включается **только флагом** в e2e/I1-α.
- ✅ **РЕШЕНО оркестратором 2026-09-11 перед стартом задачи: решение сведения 2 подтверждается.** `WithEncounterStub` в F-10 **не делается**. Причины: (1) `contracts.md` v0.4 — действующая редакция контракта, и её блок «Заглушка» C-05 называет единственной заглушкой боя Phase 1 `testkit/swarm.FakeEncounter`; (2) `design.md` §12 прямо называет риск «`WithEncounterStub` прижилась» — строить второй боевой двойник, который потом придётся удалять, значит осознанно навлекать этот риск; (3) при одном разработчике ранний merge TEAM-2 недоступен, но `FakeEncounter` (EPIC-003 T-219) стоит первым же блоком волны 1, сразу после ворот волны 0. **Состав T-018 сокращается**: `Harness` v0 и `FakeNarrator` v0 (только нарратив), сквозной тест «заглушки v0» **без боя** — создание игрока, вход, осмотр, реплика, отдых, выход; ожидаемое число `narrative.output` считается из сценария, а не берётся как 30. **Боевой сквозной тест переносится в I1-α волны 1** (после T-219/T-220), там же проверяются 30 `narrative.output`, `entity.updated` на удар и нулевой поток недоставленных. Прежний текст ниже сохранён для истории; пункты про `WithEncounterStub` не исполняются.
- ⚠ (история) Не внесённое решение сведения 2, обнаружено tech-lead#1 при сведении 3: по `contracts.md` v0.3+ (C-05, блок «Заглушка») и `consolidation.md` §11–§12 **`WithEncounterStub` в F-10 не делается** — единственная заглушка боя Phase 1 — `testkit/swarm.FakeEncounter` (EPIC-003 T-219, ранний merge). Если решение подтверждается, из описания и DoD ниже уходят пункты про `WithEncounterStub`, а e2e «заглушки v0» переводится на `FakeEncounter` (появляется зависимость от раннего merge TEAM-2) либо ограничивается сценарием без боя. Правку вносит tech-lead#1 отдельно — здесь текст сохранён как есть, чтобы не менять состав подволны 0.7 параллельно с построением дорожной карты.
- **Файлы**: `shared/testkit/gateway/**`, `shared/testkit/swarm/**`, e2e-тест `test/e2e/stubs_v0_test.go` (`-tags e2e`).
- **Зависимости**: T-016, T-017.
- **Ссылки**: `design.md` §5, §5.1, §10 (строка e2e), §12 (риск «`WithEncounterStub` прижилась»); `contracts.md` C-04, C-05 v1.1, §17; `epics.md` §2 (I1-α).
- **DoD**:
  - e2e «пустой мир»: `cmd/multiverse --contexts=all --bus=memory` → `/health ok`;
  - e2e «заглушки v0»: `Harness.Scenario("solo-30")` + `FakeState` v0 + `FakeNarrator(WithEncounterStub(FixedMechanics))` на membus → **30 `narrative.output`**, `entity.updated` по каждому удару, `dead_letters` = 0, все события валидны по реестру;
  - в коде `WithEncounterStub` — комментарий «тестовая подмена роли `encounter`; удаляется EPIC-003 I1b» и запись задачи-удаления в `tasks.md` EPIC-003 (уведомить tech-lead#2);
  - `testkit` содержит membus + v0 всех заглушек C-02…C-05 (критерий готовности эпика);
  - общий DoD §1.

### T-019: F-9 · Документация под новую раскладку · Размер: S · Статус: принята 2026-09-10 (приёмка #3, tech-lead#1; `review.md` «T-019 · приёмка #3») · Исполнитель: tech-writer · Подволна 0.7
- **Описание**: `README.md` («запуск за 5 команд», список `winget`: `ezwinports.make`, `jqlang.jq`, `FiloSottile.age`, опц. `Gitleaks.Gitleaks`), `CLAUDE.md` и `AGENTS.md` под карту `internal/*`/`shared/*`/`cmd/*` (as-is описание 15 сервисов и `go.work` — убрать), таблица статусов сервисов (активен / заморожен / архив) со ссылкой на `services/_archive/README.md`, `docs/ops/runbook.md` из `infrastructure.md` §9, `.dev-team.json.stack` по факту (Go 1.26, MinIO из исходников, Qdrant вместо Chroma, Chroma только `legacy`, без Timescale/Redis).
- **Файлы**: `README.md`, `CLAUDE.md`, `AGENTS.md`, `docs/ops/runbook.md`, `services/_archive/README.md` (дополнение), `.dev-team.json`.
- **Зависимости**: T-018, T-012.
- **Ссылки**: `infrastructure.md` §9, §10 (F-9); `epics.md` §6 F-9; NFR-095.
- **DoD**: команды из README выполняются на чистой машине владельца (проверка человеком); в документах нет ссылок на `go.work`, `make build-service`, Chroma как основную БД; `.dev-team.json.stack` обновлён; ревью tech-lead#1; общий DoD §1 (Go-тесты — n/a).

### T-020: Приёмка волны 0 и слияние в `integration/mvp-1` · Размер: S · Статус: приёмка выполнена 2026-09-11 — **волна принята с условиями** (`review.md` «Приёмка волны 0 (T-020)»); остаток за владельцем: стендовый прогон команд README (нужен GNU make), коммит, слияние и тег · Исполнитель: tech-lead#1 · Подволна 0.8
- **Описание**: сверка критериев готовности EPIC-001 (`epics.md` §2, `foundation.md` §12), слияние `epic/EPIC-001-foundation` → `integration/mvp-1`, тег `mvp-1/wave-0`, запрос пользователю на коммит (одним запросом на волну, `commits=ask`), запись в `journal.md`, фиксация имени ветки EPIC-002 (`epic/EPIC-002-state-mechanics` — см. §7), передача владения подпакетами `testkit/{state,mechanics,gateway,swarm}` поставщикам по `ownership.md`.
- **Зависимости**: T-001…T-019 (кроме T-013, который может завершиться в волне 1).
- **DoD** (= критерий готовности эпика): `git status` чист; `pre-commit` с `gitleaks` установлен, `gitleaks git --redact` = 0; `make ci` зелёный; `make up` поднимает инфраструктуру и пустые контексты `core|gateway|memory` с `/health ok`; contract-тест шины зелёный на testcontainers; `testkit` содержит membus + v0 заглушек C-02…C-05; `mvctl contracts check` без фантомов; секретов в HEAD нет; `services/_archive/` и замороженные сервисы вне `go build ./...`; `baseline.md` есть **или** зафиксирован перенос T-013 в волну 1; тег `mvp-1/wave-0` проставлен.

---

## 7. Стендовые задачи EPIC-001 (stand)

Стендовые задачи выполняет человек с одним агентом-помощником; **слот разработчика не занимают** (`teams.md` §4).

| Задача | Что проверяется на стенде | Когда | Кто |
|---|---|---|---|
| T-004 (часть) | `make minio-image`, `minio --version`, `mc admin info local` | 0.2 | devops + человек |
| T-008 (часть) | `make up` → `make health` = ok; 8 топиков с retention; `make up PROFILES=legacy` (orchestrator + semantic-memory :8083 + chroma) или решение по запасному S5 | 0.3 | devops + человек |
| **T-013 (F-8)** | матрица замера LLM: 3 прогона конфигураций C и A, `ollama ps`, `stack_up_time_s`, `baseline.md` по U-2 | 0.4–0.5 (может уехать в волну 1) | architect#1 + человек |
| T-019 (часть) | «запуск за 5 команд» из README на чистой машине | 0.7 | tech-writer + человек |
| T-014 (часть) | `make test-integration` на Docker Desktop владельца | 0.5 | developer#1 + человек |

---

## 8. Волны проекта — координация трёх команд (ведёт tech-lead#1)

Лимиты `.dev-team.json`: `maxTeams=3`, `maxAgentsPerRole=3`, **`maxParallelAgents=6`**. В таблице считаются **слоты разработчиков** (включая devops-задачи, которые выполняет разработчик по инструкциям devops). Ревьюеры (`code-reviewer#N` ≤ 3), тестировщики и QA запускаются **отдельным запуском** после волны разработчиков и в счёт строки не входят. Стендовые задачи слот не занимают.

Номера задач: TEAM-1 — T-001…T-199, TEAM-2 (EPIC-003) — T-200…T-299, TEAM-3 (EPIC-004) — T-300…T-399. Задачи TEAM-2/TEAM-3 обозначены метками их тимлидов (`tasks.md` эпиков 003/004); конкретные `T-NNN` подставляют tech-lead#2 / tech-lead#3.

### 8.1. Волна 0 (≈ 1,5–2 нед., только TEAM-1)

| Подволна | TEAM-1 | TEAM-2 | TEAM-3 | Слотов | Ворота / точка |
|---|---|---|---|---|---|
| 0.1 | T-001 (dev#1), T-002 (dev#2) | — | — | 2 (+security-engineer) | — |
| 0.2 | T-003 (dev#1), T-004 (devops) | — | — | 2 | — |
| 0.3 | T-005 (dev#1), T-006 (dev#2), T-007 (dev#3), T-008 (devops) | — | — | 4 | решение по профилю `legacy` |
| 0.4 | T-010 (dev#1), T-009 (dev#2), T-011 (dev#3), T-012 (devops); T-013 — стенд | — | — | 4 | CI зелёный |
| 0.5 | T-014 (dev#1), T-015 (dev#2) | — | — | 2 | **contract-тест шины — ворота волны 1** |
| 0.6 | T-016 (dev#1), T-017 (dev#2) | — | — | 2 | — |
| 0.7 | T-018 (dev#1), T-019 (tech-writer) | — | — | 2 | e2e «заглушки v0» |
| 0.8 | T-020 (tech-lead#1) | — | — | 0 | **тег `mvp-1/wave-0`**, старт волны 1 |

### 8.2. Волна 1 «соло + фон» (≈ 5–6 нед., ограничена EPIC-003 — оценка tech-lead#2)

Слоты: **TEAM-1 2 / TEAM-2 3 / TEAM-3 1 → 2** (`teams.md` §4; приоритет — критическому пути TEAM-2). Подволны сквозные: одна строка = один запуск оркестратора.
Номера задач TEAM-2 — по `epics/EPIC-003-swarm-llm-laws/tasks.md` §6, TEAM-3 — по `epics/EPIC-004-gateway-bot/tasks.md` §4.

| Подволна | TEAM-1 (EPIC-002 I1 → I2 / 005-ops) | TEAM-2 (EPIC-003 I1a → I1b) | TEAM-3 (EPIC-004 I1) | Слотов | Точка / ранняя поставка |
|---|---|---|---|---|---|
| 1.1 | T-050 (dev#2), T-051 (dev#1) | T-201, T-206, T-214 | T-301 (dev#1) | 6 | **ранний merge**: схемы C-04/C-10 + OpenAPI (T-301), схемы EPIC-003 ч.1 (T-214) |
| 1.2 | T-052 (dev#2), T-053 (dev#1) | T-202, T-207, T-215 | T-302 (dev#1) | 6 | **ранний merge**: схемы ч.2 + `providers/fake` (T-215, T-207) |
| 1.3 | T-055 (dev#2), T-054 (dev#1) | T-203, T-208, T-219 | T-303 (dev#1) | 6 | **ранний merge**: `FakeEncounter` (T-219) |
| 1.4 | T-056 (dev#2), T-060 (dev#1) | T-204, T-209, T-220 | T-304 (dev#1), T-310 (dev#2) | 7 → **6** | **ранний merge**: `FakeNarrator` (T-220) — вклад EPIC-003 в I1-α закрыт. Перебор снимается: TEAM-2 отдаёт слот (developer#3 со сдвигом) |
| 1.5 | T-057 (dev#2), T-061 (dev#1) | T-205, T-210, T-216 | T-305, T-311 | 7 → **6** | то же правило сдвига |
| 1.6 | T-059 (dev#2), T-058 (dev#1) | T-221, T-211, T-218 | T-306, T-312 | 7 → **6** | — |
| 1.7 | T-062 (dev#1), долги/покрытие (dev#2) | T-222, T-212, T-217 | T-307, T-317 | 7 → **6** | приёмка **EPIC-002 I1** тимлидом |
| 1.8 | T-065 (dev#1), **старт 005-ops** T-130 (dev#2) | T-223, T-213, T-234 | T-308 (dev#1; слот #2 → резерв TEAM-2) | 6 | **ранний merge T-a**: `FakeGateway` + `Harness` v1 (T-308); готовность I1a TEAM-2 |
| 1.9 | T-066 (dev#1), T-131 (dev#2) | T-224, T-229, T-235 | T-313, T-315 | 7 → **6** | **точка I1-α**: T-063 (стенд, TEAM-1) + T-391 (стенд, TEAM-3) → тег `mvp-1/i1-alpha` |
| 1.10 | T-064 (dev#2), T-067 (dev#1) | T-225, T-230, T-236 | T-309, T-316 | 7 → **6** | — |
| 1.11 | T-068 (dev#1), T-133 (dev#2) | T-226, T-233, T-237 | T-314, T-320 | 7 → **6** | готовность **EPIC-004 I1** (T-392, стенд) |
| 1.12 | T-069 (dev#1), T-134 (dev#2) | T-227, T-231, T-238 | T-319 (архивация `game-service`) | 6 | — |
| 1.13 | T-070 (dev#1), T-135 (dev#2) | T-228, T-232, T-239 | старт EPIC-004 I2 (T-350) | 6 | — |
| 1.14 | T-071 (стенд), T-136 (dev#2) | T-240, T-241, T-242 | T-352 | 5 | — |
| 1.15 | T-137 (dev#2), подготовка к интеграции (dev#1) | T-245, T-243, T-244 | T-351 | 6 | **готовность EPIC-003 I1** → интеграция |
| **интеграция I1** | tech-lead#1 (слияния 002 → 004 → 003), qa-engineer#1, tester#1 | tester#2 (S14) | tester#3 (S9, стенд) | ≤ 6 (отдельный запуск) | **S1, S3, S8, S9, S10, S14** → тег `mvp-1/i1` |

Правило снятия перебора (7 → 6): при конфликте слотов приоритет — критическому пути **TEAM-2**; сдвигается задача TEAM-1 (005-ops — Must, но не на критическом пути) либо TEAM-2 developer#3 на одну подволну (как в `teams.md` §4 для волны 2). Решение фиксирует tech-lead#1 перед запуском подволны.

**Точка I1-α** (подволна 1.9, ≈ 3–4-я неделя волны 1): EPIC-002 I1 (T-050…T-062) + EPIC-004 I1 (T-301…T-315) + `FakeNarrator` (T-018 v0, затем T-220) → бой с волком через Telegram, `generated_by=template`, без роя и LLM. Замечания человека → задачи в эпик-владелец.
**Правило старта I2**: команда начинает I2 по приёмке своей I1 тимлидом команды (TEAM-1 — с подволны 1.8, TEAM-3 — с 1.13); **слияние** I2 в `integration/mvp-1` — только после зелёного интеграционного прогона I1.

### 8.3. Волна 2 «группа + память/операции» (≈ 3–4 нед.)

| Подволна | TEAM-1 (005-ops → 005-memory) | TEAM-2 (EPIC-003 I2) | TEAM-3 (EPIC-004 I2) | Слотов | Точка |
|---|---|---|---|---|---|
| 2.1 | T-141 (dev#1), T-138 (dev#2) | T-246, T-250 (dev#3 — со сдвигом) | T-350/T-352 хвост | 5–6 | старт 005-memory (**отрезаемо**) |
| 2.2 | T-142 (dev#1), T-139 (dev#2) | T-249, T-247, T-248 | T-351, T-353 | 6 | — |
| 2.3 | T-143 (dev#1), резерв/долги ops (dev#2) | T-251, T-253 | T-354, T-355 | 6 | **S2 зелёный** (TEAM-2) |
| 2.4 | T-144 (dev#1), T-147 (dev#2) | T-252 (архив `legacy`) | T-357, T-356 | 5 | готовность EPIC-003 I2 и EPIC-004 I2 |
| 2.5 | T-145 (dev#1), T-146 (dev#2); T-140 — стенд | — | T-393 (стенд) | 2 | пороги NFR закреплены |
| 2.6 | T-148 (dev#1) | — | — | 1 | готовность 005-memory (если не отрезана) |
| **интеграция I2** | tech-lead#1 (слияния 002 → 003 → 004 → 005), qa-engineer#1, tester#1 | tester#2 | tester#3 | ≤ 6 (отдельный запуск) | **S2, S4, S5, S6** → тег `mvp-1/i2` → **G4** |

### 8.4. Порядок интеграции и ранние поставки

**Порядок слияния** (`teams.md` §3.3, `epics.md` §5):
- волна 0: `epic/EPIC-001-foundation` → `integration/mvp-1`, тег `mvp-1/wave-0`;
- **I1: 002 → 004 → 003** (I1-α не нуждается в рое; EPIC-003 — самый длинный, вливается последним и заменяет `FakeNarrator`/`FixedMechanics` реализацией);
- **I2: 002 → 003 → 004 → 005** (поставщики раньше потребителей: atomic-пакеты State → раунд `encounter` → координатор gateway → память последней).

**Ранние merge-поставки** (сливаются в `integration/mvp-1` вне общего порядка, сразу по приёмке тимлидом команды; каждая — отдельным узким PR только в свой подпакет `shared/testkit/*`, ревью tech-lead#1 + system-architect):

| Поставка | Что | Кому нужна | Когда | Пометка |
|---|---|---|---|---|
| **F-10** (T-011, T-015…T-018) | заглушки v0 C-02…C-05, `shared/entity` v2, `internal/mechanics` типы, фикстуры | всем трём командам на старте волны 1 | конец волны 0 | обязательна, в составе `mvp-1/wave-0` |
| **T-301 (EPIC-004 I1)** | схемы `player.*`/`group.*`/`round.*`/`analytics.*` в реестре, скелет `api/gateway.openapi.yaml`, DTO, Go-клиент | TEAM-1 (005-ops фикстуры), TEAM-2 (потребление C-04) | подволна 1.1 | ранний merge; регистрация типов — PR в `shared/contracts/registry.go` с ревью system-architect |
| **T-214 / T-215 (EPIC-003 «A5–A6»)** | схемы событий EPIC-003 ч. 1 и ч. 2 (`tick.*`, `narrative.*`, `llm.*`, `agent.*`, `laws.*`) | все (валидация при чтении), EPIC-005 (`trace`, `llm usage`) | подволны 1.1–1.2 | ранний merge; расхождения с T-009 — `contract-change` через system-architect |
| **T-207 (EPIC-003 «B2»)** | `providers/fake` + `recorded` (C-07/C-15) | EPIC-005 (`Embedder` фейк, golden), EPIC-002 (replay) | подволна 1.2 | ранний merge |
| **T-219 / T-220 (EPIC-003 «C4»)** | `FakeEncounter` и `FakeNarrator` реализация (замена v0) в `shared/testkit/swarm` | TEAM-3 (e2e бота), TEAM-1 (I1-α, golden 005-ops) | подволны 1.3–1.4 | ранний merge — **решение G3** (запрос architect#2/tech-lead#2; рекомендация тимлида проекта: разрешить) |
| **T-308 (EPIC-004 «T-a»)** | `FakeGateway` + `Harness` v1 (замена v0) в `shared/testkit/gateway` | TEAM-1 (e2e `group-3x30`, T-070), TEAM-2 (e2e роя) | подволна 1.8 | ранний merge, вне порядка 002 → 004 → 003 |
| **T-130…T-137 (005-ops)** | `mvctl report` *(T-416: было `session-report`)*, `trace`, `llm usage`, `golden check`, `--audit` | приёмка I1 и I2 всеми командами | подволны 1.8–1.15 | сливается в общем порядке (005 последним), но **доступно тимлидам раньше** через ветку эпика |

**Правила ранних merge**: (1) только в подпакет-владелец, без правок чужих файлов; (2) `make ci` на `integration/mvp-1` зелёный после каждого слияния; (3) после слияния tech-lead#1 прогоняет e2e `solo-30` на оставшихся заглушках; (4) дефект интеграции — задача в эпик-владелец по карте владения, не «быстрая правка» в интеграционной ветке.

---

## 9. Открытые вопросы и допущения по волне 0

1. ~~**Вопросы пользователю**~~ — **закрыты** (`infrastructure.md` v0.3, решения U-8…U-12; внесено tech-lead#1 по сведению 3): LLM-рантайм — нативный `llama-server`, Ollama опционален (U-8, T-004/T-008); `CODEOWNERS` = `@alekseizabelin1985-spec`, репозиторий приватный личный (U-9, T-012); форк `minio/minio` делает владелец до F-6 (U-10, предпосылка T-004); GPU-замер — вручную, self-hosted runner не заводим (U-12, T-013); IDE/AI-каталоги и `event-model.pdf` из индекса **не выводятся**, решение по ним — за tech-writer в T-019 (U-11, T-001).
2. **Имя ветки EPIC-002**: `teams.md` §3.2 — `epic/EPIC-002-state`; задание оркестратора и каталог — `epic/EPIC-002-state-mechanics`. Тимлид фиксирует **`epic/EPIC-002-state-mechanics`** (как каталог `epics/`) и правит `teams.md` §3.2 в T-020.
3. **`build/versions.env` в F-6a, а не в F-5** — уточнение к `design.md` §4 (см. §0).
4. **`WithEncounterStub`** — тестовая подмена роли `encounter`, издаёт типы EPIC-003 в тестовом режиме; допустимость подтверждена дизайном EPIC-001 §5.1 и запросом architect#1 в журнале; удаление — задача EPIC-003 I1b (критерий готовности I1). *(Пометка T-416, 2026-09-11: допущение волны 0 устарело. `WithEncounterStub` не делался (решение оркестратора перед T-018); единственная заглушка боя — `FakeEncounter` (EPIC-003 T-219), по C-05 v1.4 её дорабатывает T-419.)*
5. Допущение: `shared/agent` переносится в модуль как есть (без `tools/*`); если ломает `go build` — временный `//go:build ignore` с задачей EPIC-003 I1a.

## Задачи волны 1, заведённые по ходу волны 0

Карточки заводит оркестратор в момент создания задачи; полное обоснование каждой —
в `journal.md` по дате. Раздел существует потому, что задача, живущая только в
журнале и живом состоянии, невоспроизводима из артефакта эпика (замечание Mi-3
ревью T-397 и N-2 приёмки T-019).

### T-394: Интеграционные тесты адаптера Kafka в `shared/eventbus` на живом брокере · Размер: M · Статус: todo · Волна 1
- **Причина**: в пакете нет ни одного интеграционного теста, и единственным якорем правки `kafka.go` служит контрактный набор из `testkit`.
- **Состав**: жизненный цикл `Close` под подпиской в каждой точке (обработчик, коммит, парковка), перевыдача некоммитнутого, повторный `Close`, гонка с `Publish`; N повторов как второй независимый якорь мутации H.
- **Ссылки**: `review.md` — T-014 ревью #2 (Major-1) и ревью #4 (хвост).

- **(приёмка T-417, 2026-09-11)** DoD: случай contract-теста `TwoStepDedupRemembersOnlyAfterTheSideEffect` зелёный на Redpanda.
- **(T-425, 2026-09-11)** DoD: случай contract-теста «паника обработчика» (T-426) зелёный и на Redpanda.
- **(ревью #2 T-426, 2026-09-11)** До прогона на Redpanda ограничить длину текста ошибки в DeadLetter.Error и поля panic в логе: слишком большое письмо не запишется в dead_letters, и событие не закоммитится (не регрессия T-426 — так было и для обычной ошибки обработчика); тест с огромным текстом ошибки.
### T-395: Кейс контракта «Close под падающим обработчиком» · Размер: S · Статус: done · Волна 1
- **Причина**: расхождение реализаций при записи dead letter под `Close` осталось без якоря сознательно — детерминированный кейс требует третьей подписки с блокировкой и заново ставит под вопрос якорь 8 из 8.
- **Ссылки**: `review.md` — T-014 ревью #2 (Minor-3), запись итерации 3 в `dev-log.md` (форма кейса выписана целиком).
- **DoD (уточнение system-architect, 2026-09-12)**: пункт «мутант M2 (membus теряет dead letter под `Close`) краснеет» **снят** — решение 2 ниже. Обязательный мутант — M1 (откат `stopping` в `Subscribe` membus). Якорь «контекст обработчика не отменён при `Close`» в эту задачу не входит — он в T-436.
- **Решения system-architect (2026-09-12; C-01 v1.7, ADR-023 «Уточнение исполнения»)** по вопросам ревью #1, раздел «Для system-architect».
  1. **Отмена контекста обработчика kafka-адаптером при `Close` — дефект адаптера, контракт верен.**
     - `Close` вправе прервать только выборку и коммит, но не контекст обработчика.
     - Остановку обработчика ограничивает вызывающий (`Stop` контекста до `Close`) и общий срок `runtime.StopTimeout`.
     - Исправление — T-436.
     - Якорь `ctx.Err() == nil` в удержанном обработчике после `Close` ставится в T-436, вместе с исправлением. Здесь его нет: на Redpanda он красный до правки. T-394 он не ждёт — contract-набор уже гоняется на Redpanda через testcontainers.
  2. **M2 на membus — вариант (а), принять.**
     - После `Close` membus не отдаёт ни журнал, ни курсор, и в `--bus=memory` потерю не видит ни один потребитель.
     - Правило «запись в `dead_letters` не удалась → офсет не коммитится» держит `Delivery`, общая для обеих шин (тест сбоя приёмника T-426).
     - Белый ящик в `membus` отклонён: он проверял бы реализацию, а не наблюдаемое свойство. Вторая шина над тем же журналом (`contract-change`) — тоже.
     - **Условие пересмотра:** у membus появится журнал, переживающий `Close`.
- **(приёмка tech-lead#1, 2026-09-12)** Принята. M1 краснеет только новым кейсом, на обеих подписках; контроль зелёный; кейс ×10 зелёный. Nit N-1 ревью (`release` на пути отказа) принят как есть. Прогон на Redpanda — в T-436. Подробно — карточка `tasks/T-395.md`, «Приёмка».

### T-396: U-11 · Решение по каталогам среды разработки и ассистентов в индексе · Размер: S · Статус: todo · Волна 1
- **Состав**: `.idea/`, `.vscode/`, `.kilo*`, `.roo*`, `.qwen/`, `memory/`, `plans/`, `reports/`, `event-model.pdf`.
- **Требует прямого подтверждения владельца**: файлы останутся на диске, но исчезнут из свежего клона. Ничего не удалять (OQ-A-17).

### T-397: Запуск на чистой машине · Размер: M · Статус: принята 2026-09-11 · Волна 1 (выполнена до T-020)
- **Причина**: `docker compose` интерполирует весь файл до фильтрации по профилям, поэтому обязательные переменные сервисов вне активного набора роняли `make up` до создания первого контейнера.
- **Сделано**: профили `bot` и `legacy` вынесены в собственные compose-файлы; найден и закрыт второй дефект — инлайн-комментарий после пустого значения становился значением переменной; правило 7 линтера и две фикстуры.
- **Приёмка**: code-reviewer#2, ревью #1 — принять (Critical 0, Major 0, Minor 3, Nit 4). Minor-1 (расхождение разбора `.env` с compose) закрыт оркестратором.

### T-398: Свести раскол `docs/` и `Docs/`, вычистить устаревшие файлы-инструкции · Размер: M · Статус: todo · Волна 1
- **Состав**: 6 файлов под строчным `docs/` против 83 под `Docs/`; `QWEN.md` (723 строки прежней картины), `AI_AGENT_INSTRUCTIONS.md`, `README_LIVING_WORLDS.md`, `AUTOMATION-SETUP.md`.
- **Причина**: на файловой системе с различением регистра это два каталога; перечисленные файлы читаются агентами как инструкции и описывают систему, которой нет.

### T-399: Привести `infrastructure.md` к состоянию после T-397 · Размер: S · Статус: todo · Волна 1 · Исполнитель: architect#1
- **Причина (Mi-2 ревью T-397)**: §1.3, §2.2, §3.1.1 описывают прежнюю топологию, а §4.2 хранит эталон `.env.example` ровно в запрещённом теперь формате — следующий исполнитель воспроизведёт дефект «по дизайну».
- **Состав**: топология трёх compose-файлов и причина разделения; правило 7 в списке правил линтера; запрет инлайн-комментария после пустого значения и пометка обязательных переменных в §4.1/§4.2.

### T-400: `Harness` v0 — методы `Attack` и `Flee` и боевой шаг сценария · Размер: S · Статус: todo · Волна 1, бридж-блок, ДО T-219
- **Причина (приёмка волны 0)**: `FakeEncounter` из T-219 подписан на `player.attacked` и `player.flee_attempted`, а у харнесса волны 0 таких методов нет — они сняты решением о сокращении T-018. Без них DoD задачи T-219 («сквозной сценарий на 30 ходов») невыполним, и «зелёный сквозной тест» перестанет что-либо значить.
- **Владение**: `shared/testkit/gateway` по карте принадлежит EPIC-004, чьи задачи идут ПОСЛЕ бридж-блока, то есть владельца у правки сегодня нет. Путь открывается разово через tech-lead#1, как это уже сделано для T-255.

### T-401: Задание CI с детектором гонок для ключевых кейсов · Размер: S · Статус: done · Волна 1
- **Причина**: задание `unit` идёт с `-short`, задание `integration` — без `-race`, поэтому кейс `Close` контрактного набора и весь набор на живом брокере не попадают НИ ПОД ОДНО задание с детектором гонок. За всю волну 0 детектор не запускался ни разу: на машине владельца он недоступен.
- **Состав**: отдельное задание либо флаг у существующего; первыми смотреть кейсы с хвостом, ручку хаоса на работающей шине, `Harness.mu` и проекцию рассказчика.
- **Итог (приёмка tech-lead#1, 2026-09-11)**: сделано и то и другое — `-race` у `integration` и новое задание `race` (`make test-race`: `RACE_PKGS` ×`RACE_COUNT=3` плюс e2e с дочерним процессом, собранным с детектором). Детектор локально не запускался (ОВ-5). Что он ловит, докажет только контрольный мутант на раннере: карточка, «Как проверить на раннере», п. 3.
- **Требует владельца**: внести `race` в required checks `main` и `integration/mvp-1`, но только после T-433. Рекомендация исполнителя — все задания, кроме `image`.
- **Бэклог (из T-401, не блокирует)**: (1) `test/e2e`: `launch` при очистке ищет в выводе процесса `WARNING: DATA RACE`, чтобы стала видна гонка в процессе, убитом отменой контекста (`empty_world`); (2) пакеты `cmd/multiverse` и роя добавить в `RACE_PKGS`, когда у них появятся конкурентные тесты; (3) цифры первого прогона `race` и `integration` вписать в §3.1 `infrastructure.md` вместо оценок. Если `integration` выйдет за 10 мин — отдельная задача: сузить набор так, чтобы тест за тегом не выпадал молча.

### T-402: Стендовый замер LLM и заполнение `ops/metrics/baseline.md` · Размер: S · Статус: done · Волна 1 · Требует владельца
- **Причина**: T-013 закрыта наполовину честно — файл базовых показателей это шаблон с прочерками и пометкой «заполняется после прогона на стенде». DoD приёмки волны такой исход допускает при условии, что перенос зафиксирован; карточки не было, теперь есть.
- **Состав**: прогон `scripts/llm-bench.sh` на конфигурации владельца, заполнение таблицы, сверка с порогами NFR.
- **Итог (приёмка tech-lead#1, 2026-09-12)**: у E три зачётных прогона на чистом llama.cpp (2, 3, 4), все `pass` по NFR-002: p95 `phase2` — 2585 / 2916 / 3533 мс. Оговорки — `baseline.md` §3. Прогон 1 шёл через роутер и не засчитан. Qwen3.6-35B-A3B замерена справочно, вне матрицы (§2.5). Решение U-2 не принято — это T-435. Дефекты скрипта — T-434.
- **Бэклог (из T-402, не блокирует)**:
  1. `bench-matrix.json`: поле-алиас id модели либо договорённость о `--matrix` — для стендов, где сервер отдаёт модель под другим id. У роутера это `Qwen3.8`, у `llama-server` без `--alias` — путь к файлу. Исполнитель — architect#1, удобно решить вместе с T-435.
  2. `make bench` (`Makefile:432-433`) не передаёт аргументы (`--configs`, `--matrix`, `--num-ctx`, `--kv-cache`, `--out-dir`). При `pwsh` в `PATH` цель выбирает `.ps1` — на стенде владельца `pwsh` из `WindowsApps`. Исполнитель — devops-engineer.
  3. Пункты про сам скрипт — `build_info` из `/props`, `timings.prompt_ms`, прогрев — ушли в T-434.

### T-403: Стендовый прогон команд README на чистой машине · Размер: S · Статус: todo · Волна 1 · Требует владельца · БЛОКИРУЕТ критерии готовности эпика
- **Причина**: GNU make на машине не установлен, ни одна цель не исполнялась ни разу, CI цели `make` не вызывает. Правки Makefile из T-397 проверены только эмуляцией оболочкой.
- **Состав**: установить make; `make -n up PROFILES=bot`, `make -n up PROFILES=memory,legacy`, `make compose-lint`; затем настоящие `make ci` и `make up`.
- **Что без этого не закрывается**: три критерия готовности эпика — «`make ci` зелёный», «`make up` поднимает инфраструктуру и пустые контексты с ответом на проверке здоровья», «команды README выполняются на чистой машине».

### T-404: Эксплуатационная обвязка LLM не должна предполагать llama.cpp · Размер: M · Статус: todo · Волна 1 · architect#1 + devops-engineer
- **Уточнение владельца 2026-09-11**: провайдером LLM может быть что угодно, совместимое с OpenAI, — сейчас локальный unsloth на базе llama.cpp, но равно допустимы официальное облачное API, Qwen, Claude и любое другое. Среда выполнения это УЖЕ учитывает: `MV_LLM_PROVIDER` (`openai_compat|ollama|anthropic|recorded|fake`), `MV_LLM_URL`, `MV_LLM_API_KEY`, шлюз на нелокальный адрес по `MV_LLM_CLOUD_ENABLED` (ADR-005 доп. 2 п. 3). Не учитывает этого **обвязка**.
- **Что предполагает локальный llama.cpp и ломается на любом другом провайдере**:
  1. `make llm-up` и `make llm-down` запускают и останавливают нативный процесс. Для облачного или чужого сервера останавливать нечего, а `llm-up` вводит в заблуждение: цель обязана честно сказать «этот провайдер не запускается отсюда».
  2. Сверка закреплённой сборки (`LLAMACPP_BUILD` против `MV_LLM_BIN`) для не-llama.cpp неприменима в принципе. Нужен другой способ закрепить версию среды выполнения: для облака это модель и её версия из ответа, а не бинарник. Сегодня на стенде это видно как «MV_LLM_BIN не задан, сравнить нельзя».
  3. Отчёт о видеопамяти имеет смысл только для локального процесса.
  4. Точка `/health` принадлежит llama.cpp, а не контракту. Частично исправлено в T-403 (при 404 переход на `/v1/models`), но правка сделана только в PowerShell-варианте — `scripts/llm-server.sh` требует того же.
  5. **Два источника истины для одного адреса**: платформа берёт адрес из `MV_LLM_URL`, а скрипт собирает его из `MV_LLM_PORT`. Ровно из-за этого на стенде проверка стучалась в 1234, когда сервер слушал 8888. Обвязка обязана выводить адрес пробы из `MV_LLM_URL`, подменяя `host.docker.internal` на `127.0.0.1` для проверки с хоста.
- **Итог задачи**: цели `llm-*` и проверка здоровья ведут себя осмысленно для каждого значения `MV_LLM_PROVIDER`, включая облачные; способ закрепления версии среды выполнения описан для каждого случая; адрес имеет один источник истины.

### T-405: Стенд паритета двух реализаций скриптов — в репозиторий и в CI · Размер: S · Статус: todo · Волна 1
- **Причина**: паритет варианта для оболочки и варианта для PowerShell — это свойство, которое дважды за одну задачу оказывалось нарушенным (ревью T-404 нашло 10 расхождений из 24 входов, в том числе противоположные коды возврата на одном входе). Стенд, доказавший 0 расхождений на 32 входах, собран во временном каталоге сессии и при следующей правке скриптов будет собран заново — то есть свойство снова не будет держаться ничем.
- **Состав**: положить прогон паритета и подставные сервер и бинарник в репозиторий (`scripts/lib/` либо `testdata/`), звать из CI. На линуксе PowerShell может отсутствовать — тогда прогоняется половина для оболочки плюс сверка, что тексты сообщений в двух файлах совпадают.
- **Ссылки**: раздел «T-404 · ревью #1» в `review.md` (матрица на 24 входа), отчёт итерации 2 (таблица на 32 входа).

### T-406: Удалить `readySubscriber` из заглушки состояния, закрепить гарантию первого офсета кейсом контракта · Размер: S · Статус: done · Волна 1, бридж-блок
- **Причина**: C-01 v1.2 (ADR-022) признал гарантией контракта то, что новая группа читает журнал с первого офсета — её дают обе реализации (адаптер подписывается с первого офсета, шина в памяти ведёт курсор с нуля). Раз гарантия есть, рукопожатие готовности не нужно никому, и необязательный интерфейс, введённый в T-017 как обходной путь, становится лишним.
- **Состав**: удалить интерфейс, ветку его выбора и комментарий из `shared/testkit/state/state.go`; удалить тест-«часовой» о том, что шины дерева готовность не сообщают, из `state_test.go`; добавить в контрактный набор кейс «группа, созданная ПОСЛЕ публикации, получает событие с первого офсета» и прогнать на обеих реализациях.
- **Почему отдельной задачей, а не довеском к T-053**: другой пакет, другой эпик-владелец и другая метка — `shared/testkit/state` принадлежит EPIC-001, а `internal/mechanics` из T-053 принадлежит EPIC-002. Рекомендация architect#1, принята.
- **Ссылки**: ADR-022, `contracts.md` C-01 v1.2, запись T-017 в dev-log.
- **Бэклог (из T-406)**: ADR-022 запрещает топикам платформы политику `latest`. Если в композиции появится настройка `auto.offset.reset`, её стоит ловить линтером композиции. Сейчас такой настройки нет, поэтому это наблюдение, а не задача. Прогон нового кейса контракта на Redpanda — вместе с T-394.

### T-408: Режим и вид шины — один источник истины · Размер: S · Статус: todo · Волна 1
- **Причина (найдено при рисовании диаграмм, проверено оркестратором)**: `MV_MODE` и `MV_BUS` объявлены в манифесте, передаются композицией в контейнеры и НЕ ЧИТАЮТСЯ НИКЕМ — в коде нет ни одного обращения к ним. Настоящий выбор идёт флагами `--mode` и `--bus`, а композиция передаёт только `--contexts`. Оператор, поставивший в настройках режим воспроизведения, получит рабочий режим и не узнает об этом. Словари значений тоже расходятся: манифест объявляет вид шины `redpanda`, флаг принимает `kafka` или `memory`.
- **Как чинить (решение оркестратора)**: тем же способом, что и адрес модели в T-404 — манифест единственный источник, флаг перекрывает его для разработчика. Словарь свести к одному. Тот же разбор применить к `MV_GATEWAY_ADDR` и `MV_MEMORY_ADDR`: объявлены, не читаются, адрес всех трёх процессов берётся из `MV_CORE_ADDR`, а композиция дублирует значение.
- **Класс дефекта**: настройка, которая выглядит применённой и не применяется. Тот же класс, что комментарий, ставший значением переменной (T-397), и адрес, разошедшийся с портом (T-404).

### T-409: Свести документы архитектуры с деревом · Размер: M · Статус: todo · Волна 1 · architect#1 + system-architect
- **Причина**: при рисовании диаграмм найдено 24 расхождения документов с кодом. Диаграммы их зафиксировали, но живут они в документах, по которым работают исполнители.
- **Существенное**: из семи контекстов написан один, а `components/*.md` описывают ответственность всех как реализованную; нет каталогов блупринтов, законов, конфигурации и схем агентов, на которые ссылаются пять контрактов; `shared/agent` остался прежним, ни один целевой файл не создан; **`shared/agent/levels.go` не существует, хотя контракт объявляет его единственной истиной таблицы владения и требует блокирующего теста равенства с копией — фактической истиной служит копия, ровно то, что процедура запрещает**; имя переменной допущенных клиентов шлюза расходится между контрактом, манифестом и композицией, и значения по умолчанию у манифеста и композиции разные; две переменные ожидания воспроизведения введены контрактом, но в манифест не внесены.
- **Решения оркестратора, которые надо внести тем же проходом**: правило «агент, получивший отказ по конфликту версий, обновляет представление и предлагает заново» — в C-05; «встреча не отвечает на непринятое действие, потребитель различает состояние по событиям встречи» — в C-05 явно; расхождение состава зависимостей рантайма с контрактом — править контракт под код, потому что контекст создаёт хранилище со своими настройками, а мультиплексор уже используется; проверка перехода статуса в себя же — править код под контракт, потому что «ход засчитан, движения не было» уже принятое правило, и повтор действия не должен превращаться в ошибку игроку; подкоманду отчёта в документах привести к зарезервированному имени.

### T-410: Подключить шину, журнал и реестр к runtime.Deps в serve.go; убрать устаревшие комментарии · Размер: S · Статус: todo · Волна 1
- **Причина (T-409)**: C-01 v1.3 приведён к коду, но `cmd/multiverse/serve.go` не передаёт контекстам шину, журнал и реестр через `Deps` — контексты EPIC-002 не смогут их получить. Срок — не позже EPIC-002 T-055.
- **Попутно, размер XS**: комментарии, называющие истиной несуществующий `shared/agent/levels.go` (в `shared/contracts/ownership.go` и его тесте; истина — сам `ownership.go`, ADR-025); комментарий над `Deps` про «Store and Env in F-5»; ссылка в `shared/env/vars.go` на `llm-endpoint.psm1`, тогда как файл называется `LlmEndpoint.psm1`. Ещё: doc-комментарий `shared/runtime/http.go:104` говорит `"operator" when unset`, а манифест — `operator,mvctl,ci-harness` (ревью T-411).
- **Метка**: `contract-change`.

### T-411: Композиция задаёт свои умолчания для списков клиентов — второй источник значения · Размер: S · Статус: todo · Волна 1
- **Причина (T-409)**: для `MV_GATEWAY_CLIENT_IDS`, `MV_GATEWAY_ACTOR_KIND_CLIENTS` и `MV_CORE_ADMIN_CLIENTS` композиция подставляет собственные умолчания, отличные от манифеста. Тот же класс, что закрыт в T-404 и T-408: два источника одного значения, и оператор не знает, какой решает.
- **Решение оркестратора**: убрать умолчания из композиции; значение приходит из файла настроек и манифеста. Исполнитель — devops-engineer.

### T-412: Голый docker compose не читает COMPOSE_ENV_FILES из .env, а документы обещают обратное · Размер: S · Статус: todo · Волна 1
- **Причина (T-411)**: воспроизведено на compose v5.2 — у владельца `docker compose config` без `make` падает на `REDPANDA_IMAGE`, потому что версии из `build/versions.env` подхватываются только через Makefile. Шапка `docker-compose.yml`, `.env.example` и комментарий в Makefile обещают, что голый compose работает.
- **DoD**: либо голый `docker compose config -q` на чистом клоне с заполненным `.env` зелёный, либо все три документа честно говорят «только через make» и называют причину. Выбор — за исполнителем с обоснованием.
- **Исполнитель**: devops-engineer.

### T-413: Пограничные случаи правил 3 и 8 линтера композиции · Размер: M · Статус: done · Волна 1 (после приёмки T-255)
- **Причина (ревью T-411 #1 и #2)**: пять Nit, ни один не проявляется в нынешних файлах композиции, большинство даёт громкий отказ.
  - N-1: `$${MV_X:-d}` — ложное срабатывание; во вложенной подстановке сообщение искажено, внутреннее умолчание не проверяется.
  - N-3: правило 3 не видит ключ в кавычках — `"MINIO_ROOT_USER":` без значения проходит (класс существовал до T-411).
  - N-4: граница «адресная переменная — по форме умолчания манифеста» ошибается в обе стороны: `MV_CORE_ADDR` (адрес прослушивания) принимается как сетевой; форма элемента не сверяется с формой переменной (`MV_MINIO_ENDPOINT:-http://…` проходит, хотя манифест требует без схемы); `MV_LLM_URL`, `MV_TELEGRAM_HEALTH_ADDR` и др. отвергаются лишне. Вариант: явный набор из шести сетевых переменных или сверка формы с умолчанием. Решать вместе с вопросом архитектору об адресах сети. **Решено (T-416, 2026-09-11):** оба варианта сразу — явный набор и сверка формы, см. п. «T-416 п. 15» ниже.
  - N-5: строчный YAML-комментарий после ключа без значения даёт ложный отказ; в строке и в `command:` совет «ключ без значения» неприменим.
  - N-6: `${MV_X:+x}`, `${MV_X+x}` и `$$$MV_X` проходят молча, хотя дают пустую строку при молчащем `.env`.
  - Приёмка T-411: `make compose-lint` проверяет у «плохой» фикстуры только ненулевой код, а не номер правила — фикстура может краснеть от чужого правила. Цель должна сверять ожидаемое правило (например, по имени файла или по строке в фикстуре).
  - **T-416 п. 15 (T-416, 2026-09-11; `contracts.md` §16 п. 5, `infrastructure.md` §3.1.1 п. 8; закрывает N-4).** Адрес сервиса сети compose — значение другого контекста запуска, второго поля умолчания в манифесте не вводим. Правило 8 знает **явный набор из шести переменных**: `MV_KAFKA_BROKERS`, `MV_MINIO_ENDPOINT`, `MV_CORE_URL`, `MV_QDRANT_ADDR`, `MV_NEO4J_URI`, `MV_TELEGRAM_GATEWAY_URL`. Он заменяет эвристику «адресная переменная — по форме умолчания». Значение compose для них обязано иметь ту же форму, что умолчание манифеста: схема ↔ схема, `host:port` ↔ `host:port`. Сетевая переменная вне набора краснит линтер. `MV_CORE_ADDR` (адрес прослушивания) и `MV_MEMORY_URL` (пустое умолчание передаётся как есть) в набор не входят.
  - **T-416 п. 16 (T-416, 2026-09-11; `contracts.md` §16 п. 5).** Манифест берёт для `OLLAMA_*` умолчания — значения настройки `infrastructure.md` §6.4 и loopback-origins SEC-15 (`DeclareExternal` с умолчанием); `RequiredWhen` снимается. Правило 8 сверяет равенство умолчания compose и манифеста и для `DeclareExternal`. Файлы: `shared/env/infra.go`, `shared/env/env_test.go` (сегодня ждёт ошибку без `OLLAMA_MAX_LOADED_MODELS` — ожидание меняется), комментарии `.env.example`, `scripts/compose-lint.sh`.
- **DoD**: на каждый случай фикстура (плохая или хорошая), мутант на каждое новое условие зеленит ровно свою фикстуру; настоящие файлы композиции чистые.
  - **(T-416 п. 15, 2026-09-11)** фикстуры на явный набор: переменная набора с формой, отличной от умолчания манифеста (`MV_MINIO_ENDPOINT:-http://…`), — отказ правила 8; `MV_CORE_ADDR` с адресом сервиса сети — отказ; сетевая переменная вне набора — отказ с подсказкой внести её в набор; `MV_LLM_URL` и `MV_TELEGRAM_HEALTH_ADDR` лишне не отвергаются; мутант «убрать переменную из набора» краснит свою фикстуру.
  - **(T-416 п. 16, 2026-09-11)** `mvctl env check` и `go test ./shared/env/...` зелёные с умолчаниями `OLLAMA_*` в манифесте; тест в `env_test.go` проверяет умолчания вместо ошибки; фикстура «умолчание compose для `OLLAMA_*` ≠ умолчанию манифеста» — отказ правила 8.
  - `.env.example` и `shared/env/**` правятся **после приёмки T-255** (developer#1 сейчас держит `shared/env/vars.go` и `.env.example`).
- **(оркестратор, 2026-09-11) Размер S → M, приоритет обычный**: после T-416 задача включает правку Go-кода и теста в `shared/env` (п. 16, `OLLAMA_*`) и нормативное правило контракта (п. 15). Часть `shared/env` — developer, линтер композиции — devops-engineer. Ставится после приёмки T-255: общие `.env.example` и `shared/env/`.
- **Исполнитель**: devops-engineer. Приоритет — низкий, бэклог волны 1. *(T-416: п. 16 — правка Go-кода в `shared/env`; объём и приоритет задачи — на решение оркестратора, см. отчёт tech-lead#1 по T-416.)*

### T-414: Подкоманда serve: бинарник её не знает, а документы и задания пишут · Размер: XS · Статус: todo · Волна 1 (бэклог)
- **Причина (T-410)**: `multiverse serve …` падает с `unexpected argument "serve"`, флаги передаются сразу; CLAUDE.md (карта каталогов и «Прямые команды Go»), задания команды и, возможно, runbook пишут `serve`. Было и до T-410.
- **DoD**: выбрать одно из двух с обоснованием — (а) `serve` принимается явной подкомандой (и синонимом запуска без подкоманды, чтобы композиция и Makefile не сломались), или (б) документы исправлены на запуск без подкоманды. В обоих случаях: тест на форму, которая остаётся; грэп по дереву и `Docs/ops` — нигде не осталось неработающей формы.
- **Исполнитель**: developer. Файлы `cmd/multiverse/` — после приёмки T-410 (тот же `serve.go`).

### T-415: Обходные и беззвучные пути: recover в StartAll, ошибка публикации у заглушки встречи, хук go-fmt вне модуля · Размер: S · Статус: done · Волна 1 (бэклог)
- **Причина (ревью T-410, T-220)**: три мелких пути, где нарушение не видно.
  - `runtime.StartAll` не делает `recover`: паника в `Start` контекста закрывает шину раньше, чем остановлены уже запущенные контексты (обход порядка ADR-023). Нужно: паника → ошибка старта, запущенные останавливаются, шина закрывается после них; тест ленты событий.
  - `shared/testkit/swarm/fake_encounter.go`: неудачная публикация не оставляет следа — хотя бы `log.Error` и ошибка в `Err()` (у нарратора это закрыто итерацией 2 T-220). Правка в принятом файле EPIC-003 — через tech-lead#2.
  - Хук проекта `.claude/hooks/go-fmt.ps1` выдаёт ошибку `go vet` («go.mod not found») на `.go`-файлы вне модуля (временные зонды агентов): пропускать файлы вне корня модуля.
  - Дымовой тест сигналов под Windows (ревью #2 T-410): мутант «`serve` в обход обёртки сигналов» ловит только живой процесс с двойным CTRL_BREAK.
  - Правила depguard `shared-testkit-mechanics` и `shared-testkit-swarm` допускают `internal/mechanics` без `$` — та же дыра по префиксу, что закрыта в T-410 для `membus`; мутант с пакетом `internal/mechanicsx` должен краснеть.
  - Мутант M7c ревью T-410 (подмена конфигурации в строке `eventbus.NewKafka(cfg)`) выживает: две строки `reflect` в `TestOpenBusBuildsTheTransportTheValueNames` либо интеграционный тест `openBus(kafka)` вместе с T-394.
  - Проба e2e не замечает, что дочерний процесс уже вышел, и ждёт 60 с до срока (ревью #1 и #2 T-255, п. 2): проба должна проверять, жив ли процесс, и падать сразу с его кодом выхода.
  - ~~**Вопрос системному архитектору (ревью #1 T-417)**: нужен ли `recover` в `eventbus.Delivery`?~~ **решено T-425: C-01 v1.5 вводит перехват, код — T-426**; пункт DoD этой задачи «мутант на `recover` краснеет» относится к `runtime.StartAll`. Прежний текст: Сейчас паника любого обработчика роняет весь процесс ADR-001; путь по данным есть — `Derive(…, WithCauseID(…))` от конверта без id при `MV_BUS_VALIDATE_ON_READ=false`. Решение — изменение C-01; код — только после решения архитектора.
  - **(ревью T-426)** К вопросу о %w: простая замена (cause: %v) на %w опасна — stopped в kafka.go считает остановкой любой io.EOF в цепочке ошибки, и если ошибка обработчика оборачивает io.EOF, сбой записи в dead_letters молча закончил бы Subscribe с nil; безопаснее заворачивать только ErrHandlerPanic.
  - **(ревью T-426)** runtime.Goexit в обработчике под membus.Subscribe (например, t.FailNow в тесте) оставляет блокировку группы захваченной, и группа зависает — старое поведение; освобождать блокировку отложенно.
  - **(ревью #2 T-426)** Предостережение про %w стало шире: тип panicError с многозвенным Unwrap делает значение паники видимым для errors.Is; с %w для причины паника значением io.EOF при сбое записи в dead_letters заставит kafka-Subscribe молча вернуть nil при живом контексте (зонд подтвердил). Варианты: оборачивать через %w только ErrHandlerPanic или сузить stopped; нужен тест с паникой значением io.EOF при сбое записи. Ветка DLQ == nil в Deliver оборачивает причину через %w — сейчас недостижима (обе шины всегда задают DLQ), но при изменении действует то же.
- **DoD**: на каждый пункт тест или воспроизводимая проверка; мутант на `recover` краснеет.
- **Исполнитель**: developer. Низкий приоритет, бэклог волны 1.
- **Приёмка (tech-lead#1, 2026-09-11): принята** после ревью #1 и #2 (оба «принять»). Мои прогоны build/vet, unit, e2e и lint зелёные; мутанты на `recover` в `StartAll` и на `$` в depguard красные. Подробности — в карточке, раздел «Приёмка». Пункт хука `.claude/hooks/go-fmt.ps1` исключён оркестратором (файл владельца), что нужно владельцу — в карточке. Правки в принятых файлах EPIC-003 (`shared/testkit/swarm/fake_encounter.go`, `fake_encounter_test.go`, новый `fake_encounter_failure_test.go`) ждут подтверждения tech-lead#2.
- **Бэклог по итогам T-415.** EPIC-001 → T-430 (`recover` в `StopAll`; `%w` только для `ErrHandlerPanic` — вопрос system-architect). EPIC-003 — **передано tech-lead#2**: нарратор пишет `ERROR` на отменённом контексте (`fake_narrator.go:815-817`); позднее запоминание у `FakeEncounter` вместе с идемпотентностью полуопубликованного хода → T-229; исключение остановки в `refusedPublication` сузить до ошибок отмены → T-229.

### T-416: Ревизия контрактов волны 1: C-01, C-05, окружение, ADR-025 · Размер: M · Статус: done · Волна 1
- **Причина**: вопросы системному архитектору, накопленные в волне 1, до сих пор жили только в `journal.md` (замечание приёмки T-410). Здесь они собраны в одну задачу.
- **Состав** (16 пунктов, поручение — в записи журнала от 2026-09-11 и в отчёте архитектора):
  - ратификация ADR-025;
  - C-05: п. 1в при смертельном ударе, участие при выброшенной попытке, ждущий потребитель, отказ в создании встречи, существо, убитое чужой рукой, признак конца обмена, порядок `death`/`turn`, блок «Заглушка»;
  - C-01: текст про `Deps` (`contracts.md:122`), статус `membus` (`ownership.md:16`), id нарратива из причины и двухшаговый `Dedup`, глобальные источники событий, таймеры шины в replay;
  - окружение: адреса сервисов сети в композиции, `OLLAMA_*`;
  - документы: диаграммы, `infrastructure.md` §3.1.1, устаревшие `levels.go` и имена в плане и дизайнах.
- **DoD**: по каждому пункту решение с доводами, формулировка в контракте, влияние на задачи; `contracts.md` v0.7; список правок для `tasks.md` эпиков — тимлиду.
- **Исполнитель**: system-architect (Opus — ступень ниже плановой).

### T-417: C-01 v1.4 в коде: двухшаговый Dedup и id события из причины · Размер: S · Статус: todo · Волна 1
- **Причина (T-416, ADR-027)**: `eventbus.Dedup.Seen` спрашивает и запоминает одним шагом — издатель с единственным побочным эффектом вынужден заводить своё окно (нарратор T-220); дубль при потерянном подтверждении публикации не гасится, потому что `Derive` каждый раз берёт новый id.
- **Что сделать**: `Dedup.Has`/`Add` (двухшаговый API, `Seen` остаётся); опция `WithCauseID` у `Derive` — id из причины (UUIDv5 по таблице применимости ADR-027); окна дедупликации пригодны для снапшота C-14 v1.2. Метка `contract-change`.
- **DoD**: тесты на оба механизма; таблица применимости `WithCauseID` из ADR-027 покрыта тестом (неверные `parts` не склеивают разные события); contract-тест шины зелёный на `membus` и Redpanda (T-394). Срок — до T-229.

### T-418: Перенос membus в shared/eventbus/membus · Размер: S · Статус: todo · Волна 1
- **Причина (T-416, дополнение ADR-001 от 2026-09-11)**: `membus` — вторая реализация C-01, её гоняет тот же contract-тест, а не двойник; сейчас она в `shared/testkit`, и бинарник импортирует её через узкое исключение depguard (T-410).
- **Что сделать**: перенести пакет (14 импортёров), снять исключение `cmd-multiverse-bus-memory` в `.golangci.yml`, поправить `foundation.md` §9 и CLAUDE.md. Метка `contract-change`.
- **(оркестратор, 2026-09-11) Ставится после приёмки T-255**: общие `.golangci.yml` и `cmd/multiverse/bus_memory.go`.
- **DoD**: `go build`, тесты и линтер зелёные; в бинарнике нет импорта `shared/testkit`, кроме хука T-255; contract-тест шины на новом пути.

### T-420: Проход по устаревшим именам в документах вне индексов задач · Размер: S · Статус: done · Волна 1
- **Причина (проход tech-lead#1 по T-416)**: вне индексов задач остались действующие тексты со старыми именами — `session-report` (`requirements/{prd,nfr,user-stories}.md`, `testing/strategy.md`, `project/metrics.md`, `project/open-questions.md:713`, `epics/EPIC-003-swarm-llm-laws/design.md:21, :303`, ADR-008, ADR-011, ADR-017, `diagrams/c4-component-foundation.md:83`), `MV_GATEWAY_LISTEN`/`MV_GATEWAY_CLIENTS` (`components/gateway-and-bot.md:363, :370, :891`, `components/foundation.md:259`, `diagrams/c4-component-gateway-and-bot.md:102, :110`, `epics/EPIC-001-foundation/design.md:141`, `epics/EPIC-004-gateway-bot/design.md:82`), `WithEncounterStub` (`epics/EPIC-002-state-mechanics/design.md:15, :89, :105, :175`, `testing/strategy.md:90, :373, :381`), `levels.go` (`epics/EPIC-002-state-mechanics/design.md:90`, `epics/EPIC-003-swarm-llm-laws/design.md:19, :64, :254, :299`); `plan/ownership.md` §1 и ADR-001 доп. п. 8 — «единственный разрешённый импорт `shared/testkit`» (с T-410 их два, удаление хука — T-256); README `shared/testkit/swarm` не называет цену позднего запоминания (приёмка T-220). `plan/backlog.md` не знает T-417…T-420.
- **Правило**: действующий текст — заменить; запись прошлого (ADR как решение своего времени, ревью, журнал, сведения) — не переписывать, достаточно пометки «имя заменено: …, см. T-416». Замены: `session-report` → `mvctl report`; `MV_GATEWAY_LISTEN` → `MV_CORE_ADDR`; `MV_GATEWAY_CLIENTS` → `MV_GATEWAY_CLIENT_IDS` и `MV_GATEWAY_ACTOR_KIND_CLIENTS`; `WithEncounterStub` → `FakeEncounter` (T-219); `levels.go` как истина → `shared/contracts/ownership.go` (ADR-025).
- **`plan/backlog.md`**: добавить T-417…T-420; порядок — T-417 до T-229, T-419 до T-230, T-418 и T-413 после приёмки T-255.
- **DoD**: грэп по `Docs/` на пять имён — вне записей прошлого пусто; список «файл:строка — заменено / помечено» в отчёте.
- **Исполнитель**: tech-writer (Sonnet — по маршрутизации моделей).

### T-425: Ревизия контрактов 2: подтверждения исполнения волны 1 · Размер: S · Статус: done · Волна 1
- **Причина**: очередь вопросов к системному архитектору после T-416 (журнал 2026-09-11).
- **Состав**: (1) ADR-027 п. 1 — префикс длины у каждой части вместо разделителя (T-417, принято оркестратором); «с опцией `WithCauseID` генератор id не вызывается» — подтвердить; причина для `WithCauseID` у событий жизненного цикла — событие, от которого строится пакет, а не факт State (T-419) — подтвердить в C-05 п. 5; (2) `recover` в `eventbus.Delivery` — решение (C-01), паника любого обработчика сейчас роняет процесс; (3) C-05 v1.4: причина пакета закрытия по чужому факту — `resolve`; три отступления двойника T-419 от буквы контракта (отказ не по гонке тоже выбрасывает пакет с откатом; отсрочка действий и на время создания встречи; активной встреча считается от `entity.created` до факта закрытия) — подтвердить или отклонить; (4) `exchange{index,last}` в `analysis/api-contracts.md` §2.3.6; (5) `diagrams/c4-component-foundation.md:16` — подкоманды `serve` (и форма без неё), `health`, `db`, `version` (T-414).
- **DoD**: по каждому пункту решение с доводами и формулировка в контракте/ADR; влияние на задачи (особенно T-229, T-230 и её части, T-415); `contracts.md` — v0.8, если меняется текст контрактов.
- **Исполнитель**: system-architect (Opus — ступень ниже плановой).

### T-426: Перехват паники обработчика в eventbus.Delivery (C-01 v1.5) · Размер: S · Статус: done · Волна 1 (после T-418)
- **Причина (T-425, C-01 v1.5)**: паника любого обработчика роняет весь процесс ADR-001 со всеми контекстами; при at-least-once без коммита событие после рестарта приходит снова — цикл рестартов. Путь по данным есть: `Derive(…, WithCauseID(…))` от конверта без id при `MV_BUS_VALIDATE_ON_READ=false`.
- **Код**: `recover` вокруг обработчика в `Deliver` и экспортируемая `ErrHandlerPanic`; без повтора — сразу в `dead_letters` с `attempts` = номер вызова; лог `Error` с `panic`, `stack`, `handled=false`; коммит после записи в `dead_letters`, неудачная запись — ошибка; покрыты и `Subscribe`, и `Journal`, обе реализации шины. Метка `contract-change`.
- **Тесты**: паника при первом вызове; паника после ошибки (`attempts=2`); сбой записи в `dead_letters`; путь `Journal`; случай contract-теста на `membus` (на Redpanda — с T-394).
- **Мутанты**: снять `recover`; повтор после паники; `Warn` вместо `Error`; первым — контрольный мутант с синтаксической ошибкой, ключи overlay относительные.
- **Документы**: комментарий `Handler` в `bus.go`, README `shared/eventbus` (абзацы «Паника не ловится» и «Id из причины»), `components/foundation.md` §5.
- **Срок и порядок**: до первой настоящей подписки в бинарнике (T-055 или T-237), не позже интеграции I1-α; после T-418 — обе трогают `membus` и contract-тест.
- **Исполнитель**: developer.

### T-428: Проход по документам после переноса membus (T-418) · Размер: XS · Статус: done · Волна 1 (после T-418 и T-413)
- **Причина (T-418)**: после переноса membus в shared/eventbus/membus в документах архитектора и тимлида остались строки о прежнем месте: contracts.md:191 («сегодня shared/testkit/membus») и :577 (стрелка переезда), plan/ownership.md:15-16 («до переезда … импортов два»), analysis/api-contracts.md:259, ADR-010:30, diagrams/c4-component-foundation.md:31 (testkit содержит membus) и :63, epics/EPIC-003-swarm-llm-laws/design.md:142, plan/decomposition-review.md:96, architecture/infrastructure.md:1138 (F-5t, запись сдачи — только пометка).
- **Правило**: действующий текст — заменить на факт «с T-418 — shared/eventbus/membus; единственный импорт shared/testkit в бинарнике — хук T-255»; записи прошлого (ADR своего времени, ревью, сведения) — пометка, не переписывать.
- **DoD**: грэп testkit/membus по Docs/ — вне записей прошлого пусто; список «файл:строка — заменено / помечено».
- **Исполнитель**: tech-writer (Sonnet — по маршрутизации моделей). После приёмки T-418 и T-413 (infrastructure.md правит T-413).
- **Передано system-architect (tech-lead#1 при приёмке, 2026-09-11; в очередь следующей ревизии контрактов, как T-416/T-425; ни одну задачу не блокирует)**: (1) `architecture/diagrams/c4-component-foundation.md:23` — «`OwnershipRules` - статичная копия таблицы владения» противоречит ADR-025 (единственная истина — `shared/contracts/ownership.go`, копии нет; T-416); та же формула «статичная таблица F-4b» — в `epics/EPIC-002-state-mechanics/design.md:62`; (2) `architecture/contracts.md:189` — заголовок C-01 «### Заглушка для потребителей» не совпадает с текстом под ним (`membus` — вторая реализация, а не двойник). Источник — ревью #1 T-428, «Предложения в бэклог», пп. 2–3. **Выполнено в T-431** (system-architect#1, 2026-09-11): диаграмма и design EPIC-002 исправлены, заголовок в C-01 заменён на «Реализации для потребителей» (C-01 v1.6).

### T-429: compose-lint: значения из docker compose config --no-interpolate вместо разбора строк YAML · Размер: M · Статус: done · Волна 1 (бэклог)
- **Причина (ревью #1 T-413)**: построчный разбор YAML в compose-lint расходится с compose молча на целом классе записей — пробел перед двоеточием (закрыт точечно в T-413), $ вместо доллара, удвоенная одинарная кавычка, # в блочном скаляре и в многострочной строке в кавычках, поток-маппинг у правила 3; вложенная подстановка в сообщении :? даёт ложный отказ, хотя compose вычисляет его лениво. Относительный путь в -f не от корня даёт ложный отказ правила 7 (было и до T-413).
- **Что сделать**: брать значения для правил 3, 7 и 8 из вывода docker compose config --no-interpolate (значения после разбора YAML, без подстановки) — это снимает весь класс разом; прогон фикстур — параллельно (сейчас около 68 секунд подряд, цель — около 10).
- **Вопрос системному архитектору**: литерал OLLAMA_* в композиции без подстановки (например OLLAMA_NUM_PARALLEL: 4) правило 8 не проверяет — так же, как литералы MV_* (T-408); допустимо ли это для сторонних переменных.
- **Решение system-architect (T-431, 2026-09-11; `contracts.md` v0.9 §16 п. 5): вариант (б) — отвергать любой литерал. Строка DoD вынесена в T-432** (решение оркестратора по ревью #1 T-431); в T-429 её нет.
- **DoD**: каждая форма из списка — фикстура, эталон — поведение compose v5.2; время make compose-lint указано до и после.
- **Исполнитель**: devops-engineer. Низкий приоритет, бэклог волны 1.
- **Приёмка (tech-lead#1, 2026-09-11)**: принята после ревью #1 и #2 (оба — «принять») и итерации 3. `make compose-lint` → 0, «46 bad, 9 good», 27 с (до задачи — 90 с; время выше цели принял оркестратор). Хеши модели владельца не изменились. Мутант j1 ревью #2 и свой мутант приёмки убиты. Вопрос архитектору о литерале `OLLAMA_*` вынесен в T-432 (решено в T-431). Подробности — карточка `tasks/T-429.md`, «Приёмка».
- **Бэклог (из T-429, не блокирует)**: изолировать интерполированную модель и «чистую машину» правила 7 от окружения процесса. Сейчас переменная, экспортированная в оболочке оператора, перекрывает `--env-file`: например, `MV_TELEGRAM_BOT_TOKEN` в оболочке делает первую половину правила 7 зелёной на `bad-required-outside-default`. Пустой `CHROMA_IMAGE` видит только первая половина. Предложение — запускать эти два вызова `docker compose` через `env -u` для имён из `.env.example` и `build/versions.env`, с фикстурой или зондом, где переменная экспортирована. Так было и до T-429. Исполнитель — devops-engineer, размер S.

### T-430: Паника в Stop контекста и цепочка ошибки паники обработчика · Размер: S · Статус: done · Волна 1 (бэклог)
- **Причина (ревью #1 T-415, п. 2; пункты T-426 в T-415; карточка T-415, предложение п. 4)**:
  - Паника в `Stop` уходит из `shared/runtime` наружу — и при откате `StartAll`, и в `StopAll` на обычной остановке. Остальные контексты не останавливаются, а шина закрывается при раскрутке (зонд P2 ревью #1 T-415). Это тот же обход порядка ADR-023 п. 4, который T-415 закрыла для `Start`.
  - При сбое записи в `dead_letters` после паники ошибку `Deliver` не опознать через `errors.Is(err, ErrHandlerPanic)`: причина включена через `%v`. Простая замена на `%w` опасна. `stopped` в `kafka.go` примет `io.EOF` в цепочке за остановку, и kafka-`Subscribe` молча вернёт `nil` при живом контексте. Варианты: `%w` только для `ErrHandlerPanic` или более узкий `stopped`. Сторож `shared/eventbus/panic_stop_test.go` (T-415) уже стоит.
- **Сначала решение system-architect** (владелец `shared/runtime` и `shared/eventbus`, C-01):
  1. Нужен ли `recover` в `StopAll`: ошибка `stop context <имя>: panic: …`, стек в лог, остальные контексты продолжают останавливаться.
  2. Нужно ли вызывающему различать панику при сбое записи в `dead_letters`, и в какой форме. Изменение цепочки ошибок — `contract-change`.
- **Решение system-architect (T-431, 2026-09-11; C-01 v1.6). Задача может стартовать.**
  1. **`recover` в `StopAll` — нужен.**
     - Ошибка — `stop context <имя>: panic: <значение>`. Стек идёт в лог `Error` с полями `context`, `panic`, `stack`.
     - Остальные контексты останавливаются в обратном порядке, шина закрывается последней (ADR-023 п. 4).
     - Лог внутри `StopAll` нужен потому, что стек виден только в отложенной функции, а у `StopAll` логгера нет. Отсюда новая сигнатура `StopAll(ctx, contexts, log *slog.Logger) []error`: при `nil` лога нет, как у `start` без `deps.Log`. Это изменение Go-API `shared/runtime`, метка `contract-change`. Все вызовы в EPIC-001. В production их три: `StartAll` (`lifecycle.go:26`) передаёт `deps.Log`, а `serve.go:340` и `serve.go:349` — свой логгер. Четвёртый — тестовый, `shared/runtime/runtime_test.go:320`.
     - При откате `StartAll` ошибки `Stop` больше не отбрасываются: они дописываются к ошибке старта в той же строке через «; » — `start context <имя>: <причина>; stop context <имя>: <причина>`. Не `errors.Join`: ошибку `StartAll` `serve.go:62` печатает одной строкой stderr, а `Join` разделяет части переводом строки. Цепочка (`%w`) остаётся у ошибки старта, ошибки `Stop` входят текстом. Переводы строк в тексте паники — и в `Stop`, и в `start` из T-415 — экранируются как `\n` и `\r`, поэтому строка ошибки одна при любом значении паники. В поле лога `panic` значение пишется как есть.
     - *Почему:* паника, ушедшая из `Stop` наружу, обрывает остановку остальных контекстов, а шину закрывает раскрутка стека. Это тот же обход порядка, что T-415 закрыла для `Start`. На откате паника была бы вдобавок единственной причиной, которую никто не увидит.
  2. **Различать панику при сбое записи в `dead_letters` — не нужно. Цепочка не меняется: `%v` для причины остаётся.**
     - Паника уже оставила `Error` со стеком и `handled=false`, ещё до попытки записи (метрика `service_panics`).
     - Реакция на несостоявшуюся запись одна при любой причине: событие не закоммичено, подписка возвращает ошибку. Ветвиться по панике некому.
     - Любой `%w` для причины открывает ловушку `stopped` с `io.EOF`.
     - Код по этому пункту — одна строка: ветку `DLQ == nil` в `deadLetter` перевести на ту же форму: `fmt.Errorf("eventbus: no dead letter sink for %s (cause: %v)", ev.Type, cause)` — без `%w` и без нового значения ошибки, Go-API `shared/eventbus` не меняется. Сейчас ветка недостижима, но при изменении ловушка открылась бы и там. Плюс случай сторожа `panic_stop_test.go` для этой ветки.
     - Если различение понадобится, в цепочку добавляется только `ErrHandlerPanic`, без значения паники, и с тестом на панику `io.EOF`.
- **Код (после решения)**:
  - п. 1 — по образцу `runtime.start` (T-415): `recover` вокруг `Stop` и параметр `log *slog.Logger` у `StopAll`.
    - Тест ленты: паника в `Stop` второго контекста → первый остановлен, шина закрыта последней.
    - Тест отката: паника в `Stop` на откате `StartAll` → текст паники входит в ошибку `StartAll` в той же строке (перевода строки в тексте ошибки нет), стек — в лог.
    - Тест многострочной паники на откате: значение паники в `Stop` (и в `Start`) — `"a\nb"` → в ошибке `StartAll` оно экранировано как `a\nb`, строка одна; в поле лога `panic` значение как есть.
    - Тест неприглядного значения паники — как в T-415.
    - Мутант на `recover` краснеет.
  - п. 2 — по решению выше цепочка не меняется. Ветка `DLQ == nil` переходит на `%v` для причины, у сторожа `panic_stop_test.go` появляется случай этой ветки, и сторож зелёный.
- **DoD**:
  - решение архитектора записано с доводами — выполнено: C-01 v1.6, T-431;
  - тесты и мутанты выше;
  - `go test -short ./...`, e2e и линтер зелёные;
  - doc-комментарии `StopAll` и `StartAll` в `shared/runtime/lifecycle.go` называют перехват паники в `Stop` и формат ошибки отката (README у `shared/runtime` нет и ради этого не заводится);
  - dev-log заполнен;
  - метка `contract-change`.
- **Исполнитель**: system-architect (решение), затем developer. Ветка `task/T-430-panic-in-stop`. Можно вести параллельно с остальными задачами волны: трогает `shared/runtime/lifecycle.go`, `shared/runtime/runtime_test.go`, `shared/eventbus/delivery.go` и вызовы `StopAll` в `cmd/multiverse/serve.go` (решение T-431). Низкий приоритет, бэклог волны 1.
- **Приёмка (tech-lead#1, 2026-09-11)**: принята после ревью #1 («принять», Mi-1 закрыт итерацией 2 текстом, N-1 без действий). Все вызовы `StopAll` передают логгер (грэп: `lifecycle.go:43`, `serve.go:342`, `:351`, тестовые в `runtime_test.go`). Build, vet, vet e2e — 0. `go test -short` — 27 пакетов ok, e2e ok, `golangci-lint` — 0 issues. Мои мутанты M1 (`StopAll` мимо `recover`), M2 (`%w` в ветке `DLQ == nil`) и M3 (`\r` не экранируется) красные, контрольный M0 красный. Подробности — карточка `tasks/T-430.md`, «Приёмка».
- **Бэклог (из T-430, не блокирует)**: одна строка stderr при любой причине отказа (не только при панике) — вопрос system-architect; предложение ревьюера: экранировать один раз при выводе в runServe (serve.go:58, :62); решить до первого настоящего контекста EPIC-002/003, который соберёт ошибки Stop через errors.Join.

### T-431: Ревизия контрактов 3: решения EPIC-003, T-430, литерал OLLAMA_*, редакционные · Размер: S · Статус: done · Волна 1
- **Причина**: очередь вопросов к системному архитектору после T-425 — решения architect#2 по EPIC-003 (коммит `23c845d`), T-430, вопрос исполнителя T-429, замечания приёмки T-428.
- **Состав**:
  1. ADR-028 и design EPIC-003 §14 проверить на соответствие C-01 v1.5, C-05 v1.5, ADR-026, ADR-027.
  2. Пример `Seen` в C-01 и ADR-027 п. 3; прочтение C-05 п. 1.
  3. T-430: `%w` для `ErrHandlerPanic` и `recover` в `StopAll`.
  4. Литерал `OLLAMA_*` в композиции.
  5. `OwnershipRules` как «копия» (диаграмма, design EPIC-002); заголовок «Заглушка для потребителей» в C-01.
- **DoD**:
  - по каждому пункту решение с доводами и место записи;
  - `contracts.md` v0.9, если меняется текст контрактов;
  - ADR — «Уточнение исполнения»;
  - отметка в design EPIC-003 §14;
  - решения в разделах T-430 и T-432 (литерал вынесен из T-429).
- **Исполнитель**: system-architect (Opus — ступень ниже плановой). Карточка — `tasks/T-431.md`.

### T-432: compose-lint: литерал OLLAMA_* без подстановки отвергается · Размер: S · Статус: done · Волна 1 (бэклог)
- **Причина (T-431; вопрос исполнителя T-429)**: литерал `OLLAMA_*` в композиции без подстановки (например, `OLLAMA_NUM_PARALLEL: 4`) правило 8 не проверяет — так же, как литералы `MV_*` (T-408). Литерал молча отбрасывает значение из `.env`: настройка применяется не туда.
- **Решение system-architect (T-431, 2026-09-11; `contracts.md` v0.9 §16 п. 5): вариант (б) — отвергать любой литерал.**
  - Для `OLLAMA_*`, объявленной в `shared/env/infra.go` с умолчанием, литерал в композиции — нарушение правила 8 при любом значении, даже равном умолчанию.
  - Это касается и отображения, и элемента списка. Допустима одна форма — `${OLLAMA_X:-<умолчание манифеста>}`, и отказ её называет.
  - Необъявленные `OLLAMA_*` правило 8 не трогает. Из них правило 5 проверяет только значение `OLLAMA_HOST` (SEC-15); остальные не проверяются, и правило 5 эта задача не расширяет.
- **DoD**:
  - фикстура для отображения с литералом, отличным от умолчания;
  - фикстура для элемента списка;
  - фикстура с литералом, равным умолчанию;
  - у каждой `expect-text` про форму подстановки;
  - рабочая композиция проходит `make compose-lint`;
  - одна фраза в `infrastructure.md` §3.1.1 п. 8;
  - `make ci` зелёный;
  - dev-log заполнен.
- **Порядок**: ветка от эпика после слияния T-429. Там уже новая compose-lint (`docker compose config --no-interpolate`), и правило 8 правится поверх неё. Трогает `scripts/compose-lint.sh` (правило 8) и `testdata/compose-lint/`.
- **Исполнитель**: devops-engineer. Ветка `task/T-432-ollama-literal`. Низкий приоритет, бэклог волны 1.
- **Приёмка (tech-lead#1, 2026-09-11)**: принята после двух итераций (ревью #1 и #2 — «принять»). `make compose-lint` → 0, «52 bad, 10 good». Хеши владельца прежние. Мутанты «проверка записи выключена» и «ветка `-` выключена» убиты. `make -o secrets-scan ci` → 0. Nit ревью #2 N-1…N-4 приняты как есть: доводы в карточке, правка — попутно в бэклоге п. 1. Подробности — `tasks/T-432.md`, «Приёмка».
- **Бэклог из T-432**: (1) `${MV_X-d}` для `MV_*` — решение system-architect, затем S-задача devops-engineer с фикстурой; (2) `${OLLAMA_X:?}`/`${OLLAMA_X?}` в профилях `bot`/`legacy` — правило 7 их не читает, S-задача «для `OLLAMA_*` правило 8 принимает ровно `:-` во всех файлах»; (3) system-architect: в `contracts.md` §16 п. 5 прямо назвать чужую переменную и свою подстановку с текстом нарушением.

### T-433: Флак fight-NN в TestTheHarnessAndTheEncounterOfTheSwarmOnOneBus · Размер: S · Статус: done · Волна 1 (бэклог)
- **Причина (исполнитель T-401; ревью #1 T-401, п. 5)**:
  - `fight-09` → «the harness never heard that player-A was in an encounter» (`shared/testkit/gateway/stand_test.go:100`, утверждение `h.Fight(playerA)` на `:99`). Падение — 1 из 4 серий `go test -count=5 ./shared/eventbus/... ./shared/testkit/... ./shared/runtime/... ./shared/clock/...` без детектора. Это пакеты задания `race`, и в серии они конкурируют за CPU. Отдельно тест не падает: 0 из 20 одиночных запусков, `-count=20` теста и `-count=20` пакета зелёные.
  - Ревьюер: у стенда `standTimeout = 2 * time.Second` (`stand_test.go:47`). Задание `race` гоняет пакет трижды под детектором на двух ядрах раннера, так что падать там будет чаще, чем в `unit`.
- **Гипотеза tech-lead#1 (по чтению кода, не проверена)**: дело не в тайм-ауте, а в порядке доставки. Док-комментарий `Harness.Fight` (`harness.go:740-741`) сам говорит: `false` значит «не слышал», а не «боя нет». `encounter.started` идёт по `world_events`, `Run` ждёт ответов State по `system_events`, а C-01 не упорядочивает топики между собой. Стенд проверяет `h.Fight` сразу после `Run`, без ожидания, и тем самым требует того, чего харнесс не обещает. Соседнее утверждение (`:102`) читает `encounter.started` из журнала шины и проходит. Надо исключить альтернативу: подписка харнесса на `world_events` теряет событие или складывает его не туда. Тогда это дефект харнесса, а не стенда.
- **DoD**:
  - причина найдена и записана в карточке с доказательством: воспроизведение, например принудительной задержкой доставки `world_events` харнессу, или лог порядка доставки на упавшем прогоне;
  - исправлена именно причина. Поднять `standTimeout` или добавить ожидание без такого доказательства — не принимается. Если исправляется утверждение, ожидание ограничено тем же `standTimeout` и при истечении падает с тем же текстом;
  - мутант: приём воспроизведения краснеет без исправления и зеленеет с ним;
  - четыре серии подряд `go test -count=5` по пакетам `RACE_PKGS` (условия, в которых флак ловился) и `go test -count=20 ./shared/testkit/gateway/` — зелёные;
  - при доступном cgo — `make test-race`, иначе зелёный `race` в PR задачи на раннере;
  - `go test -short ./...`, e2e, линтер зелёные, dev-log заполнен;
  - закрыть до того, как `race` станет required check (рекомендация T-401 владельцу).
- **Владение**: `shared/testkit/gateway` по карте принадлежит EPIC-004 (TEAM-3), но стенд создан T-400 бридж-блока EPIC-001. Путь открывается через tech-lead#1, как для T-400, и принимает tech-lead#1. Если исправление затронет `shared/testkit/swarm` (`FakeEncounter`, EPIC-003) — **подтверждает tech-lead#2**. Близнец стенда с нарратором `shared/testkit/swarm/fake_narrator_stand_test.go` (T-220) проверить на то же утверждение без ожидания (грэп `.Fight(` по тестам сегодня его там не находит).
- **Исполнитель**: developer. Ветка `task/T-433-stand-fight-flake`. Можно вести параллельно с T-430 и T-432: файлы не пересекаются. Трогает `shared/testkit/gateway/stand_test.go`, возможно `harness.go`. Низкий приоритет, бэклог волны 1, но до required check `race`.
- **Бэклог из карточки T-433 (отдельные задачи, не блокируют)**: 1) исход боя стенда не закреплён за номером `fight-NN`: общий источник id тянут горутины State, двойника и харнесса, поэтому пути боя стенд покрывает статистически. Воспроизводимость по номеру требует источника id на публикатора в `testkit`. 2) ожидание харнесса не помнит, в какой бой ушло действие: поздний старт старого боя даст ложный `ErrFightOver` действию в новом бою, как и `end` сегодня. В MVP-1 это недостижимо: один персонаж — один бой.

### T-434: llm-bench — прогрев через python-помощник, путь матрицы вне репо (имя + sha256), first_call_ms пустой при не-200, timings.prompt_ms на запрос; паритет .ps1 · Размер: S · Статус: todo · Волна 1 (бэклог)
- **Причина (ревью T-402: #1 — «Дефект прогрева» и бэклог пп. 1–4, #2 — бэклог; бэклог исполнителя T-402 пп. 1, 2, 5)**:
  1. Прогрев в `llm-bench.sh` (`:710-715`) собирает тело JSON строкой bash и не экранирует id модели. `llama-server` без `--alias` отдаёт id как путь Windows с `\`, и прогрев получает HTTP 500. `first_call_ms` пишется всё равно, без пометки. Так в T-402 недействителен B2 у прогонов 2–4 E и у Q36. В `.ps1` тело собирается через `ConvertTo-Json` (`:341-347`), дефекта там нет, то есть двойники расходятся. Но `first_call_ms` при не-200 пишут оба (`.sh:715`, `.ps1:366`).
  2. `meta.matrix`: `.sh:834` пишет путь вне репозитория абсолютным, `.ps1:620` пишет аргумент `-Matrix` как есть. В T-402 так в JSON попал путь профиля пользователя (C-1 ревью #1), и значение правили руками.
  3. В `requests[]` нет `timings.prompt_ms`. Без него причину всплесков «прочего» времени (прогон 1 E, Q36) по отчёту не установить (`baseline.md` §2.1, §2.5).
  4. Когда `MV_LLM_BIN` не задан, `llamacpp_build` в CSV — пин `b10441(pin)`, а не билд стенда. Факт есть в `GET /props` → `build_info`.
- **DoD**:
  - тело прогрева в `.sh` собирается python-помощником, как тела замера. Проверка без сервера: id с `\` и с `"` даёт тело, которое разбирается как JSON;
  - при не-200 на прогреве `first_call_ms` пустой в CSV и JSON обеих реализаций, в stderr — строка с кодом ответа;
  - `meta.matrix` в обеих реализациях: путь внутри репозитория — относительный, как сейчас; вне репозитория — только имя файла плюс `sha256` содержимого (новое поле `meta`). Абсолютного пути в отчёте нет ни при каком `--matrix`/`-Matrix`;
  - `requests[]` обеих реализаций несёт `prompt_ms` из `timings.prompt_ms` (пусто, если сервер его не отдал);
  - `llamacpp_build`: при незаданном `MV_LLM_BIN` — `build_info` из `/props`, если сервер его отдаёт, иначе пин, как сейчас. Источник виден в значении (например `(props)` / `(pin)`). Сверку с пином (T-404 п. 2) не дублировать — сослаться;
  - колонки CSV не меняются, иначе старые отчёты перестанут сравниваться с новыми. Новые поля — только в JSON;
  - паритет: на одних и тех же входах (подставной сервер: id-путь, не-200 на прогреве, ответ без `timings.prompt_ms`) `.sh` и `.ps1` дают одинаковые CSV и одинаковый набор полей JSON. Стенд паритета в репозитории — область T-405, здесь достаточно прогона в карточке;
  - `ops/metrics/README.md` описывает новые поля и пустой `first_call_ms`. `bench-matrix.json` не меняется (им владеет architect#1), отчёты T-402 не переписываются;
  - `bash -n scripts/llm-bench.sh` чист, `.ps1` разбирается парсером PowerShell без ошибок, dev-log заполнен.
- **Исполнитель**: devops-engineer. Ветка `task/T-434-llm-bench-warmup-matrix`. Трогает `scripts/llm-bench.sh`, `scripts/llm-bench.ps1`, `ops/metrics/README.md`. Файлы не пересекаются с T-435, можно вести параллельно в отдельных папках. Закрыть до следующего замера на стенде (прогоны по плану T-435).

### T-435: U-2 — базовая конфигурация LLM по трём зачётным прогонам E; включать ли Qwen3.6-35B-A3B в матрицу · Размер: S · Статус: todo · Волна 1
- **Причина (T-402)**: замер E завершён. Три зачётных прогона на чистом llama.cpp (2, 3, 4), все `pass` по NFR-002: p95 `phase2` — 2585 / 2916 / 3533 мс. Решение U-2 в T-402 сознательно не принято: у вердикта шесть оговорок (`baseline.md` §3), а владелец попросил посмотреть Qwen3.6-35B-A3B. Она замерена справочно, один прогон вне матрицы (§2.5): все пороги пройдены, генерация в 2,3–2,7 раза быстрее в нарративных фазах, но половина нарративных запросов с необъяснённым «прочим» > 2 с.
- **Что решить**:
  1. Принимать ли E базовой конфигурацией при оговорках §3:
     - 40–48 ток/с против справочных 70;
     - при ~0,8 с накладных 5 с хватает на ~175 токенов `phase2`, потолок — 320;
     - ячейка вне матрицы (4 × 200 192 вместо 8192 / 16384): считать ли её ячейкой решения или нужны прогоны в ячейках матрицы с перезапуском стенда владельцем;
     - тип KV неизвестен, VRAM — вся карта;
     - три прогона за ~20 минут одной ночи.
  2. Незачёт прогона 1 (через роутер; путь платформы — чистый llama.cpp) закрепить в решении.
  3. Включать ли Qwen3.6-35B-A3B в матрицу (кандидат, запасной, место в `decision_order`). Если да — три прогона по методике.
  4. Нужен ли потолок длины нарратива в блупринтах MVP-1, если базовой станет E (следует из оговорки про ~175 токенов).
- **Входы**: `ops/metrics/baseline.md` §2–§5, `ops/metrics/bench-matrix.json`, ADR-005 доп. 2 п. 5, `infrastructure.md` §6.4, `nfr.md` NFR-002 / NFR-022 / NFR-076 / NFR-090, отчёты `ops/metrics/bench-*` и `diag-T402-20260911.csv`, карточка T-402.
- **DoD**:
  - решение с доводами — в ADR-005 разделом «Уточнение исполнения» (по образцу ADR-026…028) либо новым ADR;
  - строка решения в `baseline.md` §5: базовая конфигурация, обоснование, кто и когда. Пункты чек-листа §5 либо выполнены, либо вынесены задачами с номерами;
  - если решение принято — в `bench-matrix.json` заполнены `decision.*` и `results.E`: прогоны 2–4 и прогон 1 с пометкой «не зачётный»;
  - если Qwen3.6 включается — в матрице её конфигурация (id, провайдер, ячейки, `expected_vram_mb`, место в `decision_order`), правка ADR-005 доп. 2 п. 5 и план трёх прогонов: кто, когда, на каком сервере, после T-434;
  - всё, что требует владельца (перезапуск стенда, прогоны в ячейках матрицы), сформулировано вопросом оркестратору, а не решено молча;
  - dev-log заполнен.
- **Исполнитель**: architect#1. Ветка `task/T-435-u2-llm-baseline`. Кода нет: трогает ADR-005 (или новый ADR), `baseline.md` §5, `bench-matrix.json`. Можно вести параллельно с T-434 — файлы не пересекаются. Прогоны Qwen3.6 по плану — только после T-434.

### T-436: Kafka-адаптер: Close прерывает чтение и коммит, но не контекст обработчика · Размер: S · Статус: done · Волна 1 (бэклог)
- **Причина (ревью #1 T-395, «Для system-architect» п. 1; решение system-architect 2026-09-12, C-01 v1.7, ADR-023 «Уточнение исполнения» п. 1)**:
  - в `Subscribe` kafka-адаптера контекст цикла отменяется при `Close` (`context.AfterFunc(k.closing, cancel)`, `kafka.go:173-176`), и тот же контекст через `k.deliver` → `Delivery.Deliver` уходит в обработчик;
  - это нарушает C-01 «Остановка» п. 3 и ADR-023 п. 4: обработчик посреди PUT в объектное хранилище обрывается;
  - membus так не делает;
  - якоря нет — мутант M4 (membus отменяет, как kafka) зелёный на всём наборе.
- **Что сделать** (`shared/eventbus/kafka.go`, `Subscribe`): два контекста.
  - **Контекст цикла** — от вызывающего плюс отмена по `k.closing`. На нём только `FetchMessage` и `CommitMessages`, как сейчас.
  - **Контекст обработчика** — контекст вызывающего без `k.closing`; в `k.deliver` передаётся он.
  - Решение «остановлено» после доставки и после коммита принимается по контексту цикла (`stopped(loopCtx, err)`). Иначе ошибка записи в закрытую шину на повторе стала бы ошибкой `Subscribe`, а не `nil`.
  - Комментарий `kafka.go:163-172` переписать: `Close` отменяет ожидание транспорта, а не обработчик, и почему.
- **Как не вернуть зависание коммита T-014 итерации 2**: коммит остаётся на контексте цикла, который отменяет `Close`. Кейс `Close` contract-набора (T-014) должен остаться зелёным на Redpanda — не меньше 8 прогонов подряд, как в T-014.
- **Якорь** (`shared/testkit/contract/contract.go`, кейс T-395 `CloseUnderAFailingHandlerIsAnOrderlyStop`): удержанный обработчик после возврата `Close` и до `release` записывает `ctx.Err()`, кейс требует `nil` на обеих целях. Порядок: сначала якорь — красный на Redpanda с нынешним адаптером (показать в dev-log), потом исправление.
- **Мутанты**:
  - контрольный — синтаксическая ошибка;
  - вернуть `k.closing` в контекст обработчика → якорь красный на Redpanda;
  - M4 (membus отменяет контекст обработчика при `Close`) → якорь красный на membus.

  Ключи overlay — относительные, либо копия рабочей папки, как в T-395.
- **DoD**:
  - якорь и исправление; мутанты выше;
  - `go test -short ./...`, `make test-integration` (contract-набор на Redpanda через testcontainers, T-394 не ждёт), e2e, `make ci` зелёные;
  - кейс `Close` T-014 на Redpanda — 8 из 8;
  - комментарий в `kafka.go` и README `shared/eventbus` (раздел об остановке) называют границу «транспорт, а не обработчик»;
  - dev-log заполнен.
- **Метка**: `contract-change` не нужна — Go-API и контракт не меняются, код приводится к C-01 v1.7. Правка `shared/eventbus` — с ревью system-architect по карте владения.
- **Порядок**: после слияния T-395 — якорь ставится в её кейс. Параллельно с остальными задачами волны, трогает `shared/eventbus/kafka.go`, `shared/testkit/contract/contract.go`, README пакета.
- **Исполнитель**: developer. Ветка `task/T-436-kafka-close-handler-ctx`. Низкий приоритет, бэклог волны 1; закрыть до первой настоящей подписки на брокере в бинарнике (T-055 или T-237).
- **Замечание к старту (tech-lead#1, приёмка T-395)**:
  - После правки `Delivery.Deliver` получает контекст обработчика. Поэтому пауза между повторами (`d.wait`) и проверка `ctx.Err()` после неудачной попытки (`delivery.go:106`, `:121`) идут на нём, и `Close` их не прерывает. Так же ведёт себя membus, и это согласуется с C-01 v1.7: пауза повтора — не ожидание транспорта. Записать в dev-log как осознанное следствие, не «чинить» переносом паузы на контекст цикла без решения system-architect.
  - Задаче нужна среда с Docker (testcontainers): красный якорь на Redpanda до правки — обязательный шаг.
- **(приёмка tech-lead#1, 2026-09-13)** Принята. Contract-набор на Redpanda зелёный 2 из 2, оба кейса `Close` PASS. M1 (`k.deliver(loopCtx, …)`) краснит только якорь, на обеих подписках, 2 из 2. Вместо `make test-integration`/`make ci` выполнены их составляющие (ограничение владельца). Подробно — карточка `tasks/T-436.md`, «Приёмка».
- **Бэклог (из T-436, 2026-09-13)** — отдельные задачи, не блокируют:
  - Mi-1 ревью #1 (владелец `shared/eventbus`, EPIC-001). `Delivery.Deliver` (`delivery.go:128`) пишет `Warn` «event parked in dead letters» до записи. Если запись падает (например, `ErrClosed` после `Close`), событие не припарковано и не закоммичено, а лог говорит обратное. Нужно писать «parked» после успешной записи, а при ошибке — «not parked: …» с причиной. Обе шины, одной задачей по образцу T-430.
  - Тест «процесс укладывается в `runtime.StopTimeout`, когда подписка жива в момент `Close`»: обработчик, не слушающий контекст, и пауза повтора больше не обрываются по `Close`. Завести вместе с первой настоящей подпиской в бинарнике (T-055/T-237).
  - Строгость якоря T-395 «обе подписки» при асинхронной отмене контекста — **решено не делать** (tech-lead#1 вместе с ревьюером). Ожидание `ctx.Done` со сроком внесло бы в якорь время. Пересмотреть, если появится реализация шины, которая отменяет контекст обработчика асинхронно и проходит якорь.
  - Косметика: в `kafka.go` съехал перенос в блоке комментария перед `loopCtx`. Выправить при ближайшем касании файла.
