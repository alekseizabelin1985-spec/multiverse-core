# Задачи EPIC-001 «Фундамент» (волна 0)

Версия 0.1.8 · 2026-09-14 · tech-lead#1 (TEAM-1, тимлид проекта) · статус: к G3.
**Правки сведения 3** (`architecture/consolidation.md` §14, `contracts.md` v0.4, `infrastructure.md` v0.3; внесено tech-lead#1): T-001 (F-1 — IDE-каталоги), T-004 (F-6a — `MINIO_REPO`/`LLAMACPP_BUILD`/`LLM_MODEL_DEFAULT`, `extra_hosts`, форк MinIO), T-006 (F-4b-1 — `Spec.Publishers`, строка gateway в `OwnershipRules`, `cause=forget`), T-007 (F-5 — `MV_LLM_*`, условные `OLLAMA_*`), T-008 (F-6b — `llm-server.*`, `make llm-*`, правило `compose-lint`), T-012 (F-7 — CODEOWNERS), T-013 (F-8 — `prompts.jsonl`, порядок E→C→A), T-017 (F-10d — `replay.completed`, `abandoned`), §0 (F-6 +0,5), §9 п. 1. Структура подволн и состав задач не менялись.
**Правки ревизии контрактов T-416** (`contracts.md` v0.7; ADR-025 подтверждён, ADR-026, ADR-027; внесено tech-lead#1 2026-09-11): T-006 — пометка «таблица владения — единственная истина»; T-413 — пункты T-416 п. 15–16; §8 — `mvctl report`; §9 п. 4 — пометка. Новые задачи T-417 и T-418 завёл оркестратор. Добавленные пункты помечены «(T-416, 2026-09-11)», отменённые — «заменено (T-416)» и не удалены.
**Правки 2026-09-13 (tech-lead#1, приёмка T-461; версия 0.1.2)**: добавлены разделы T-460 (`in_progress`), T-461 (`done`) и T-463 (`todo`). Статусы T-400, T-403, T-404, T-408–T-412, T-414, T-417, T-418 и T-439 в заголовках разделов приведены к `state.js` (`todo` → `done`; бэклог приёмки T-456, п. 4). Тексты этих разделов не менялись.
**Правки 2026-09-13 (tech-lead#1, приёмка T-463; версия 0.1.3)**: T-463 — `done`; ветка раздела приведена к факту, часть (в) отмечена как вынесенная в T-464, п. 8 DoD сужен по решению оркестратора (вариант (б) по Ma-2 ревью #2). Добавлен раздел T-468 (`todo`).
**Правки 2026-09-13 (tech-lead#1, приёмка T-464; версия 0.1.4)**: добавлены разделы T-464 (`done`, раздела не было — карточку создал исполнитель по поручению оркестратора) и T-469 (`todo`, номер выдан оркестратором). Тексты прочих разделов не менялись.
**Правки 2026-09-14 (tech-lead#1, приёмка T-470; версия 0.1.5 — после 0.1.4 приёмки T-464)**: добавлен раздел T-470 (`done`; раздела не было — задачу system-architect завёл оркестратор). В раздел T-468 внесён объём по C-15 v1.6 (передача T-470); строки для T-469 — в разделе T-470. T-466 (C-01 v1.11, КД State §6.1, §6.2) оркестратор закрыл как покрытую T-470; раздела T-466 в индексе нет. Тексты прочих разделов не менялись.
**Правки 2026-09-14 (tech-lead#1, приёмка T-469; версия 0.1.6)**: T-469 — `done`; в разделе T-469 ветка приведена к факту (`task/T-469-compose-gateway-vars`), «восемь `MV_GATEWAY_*`» заменено на десять (добавлены две переменные T-307), добавлены итог приёмки, передача и бэклог. В разделе T-470 отмечен п. 1 передачи для T-469. Тексты прочих разделов не менялись.
**Правки 2026-09-14 (tech-lead#1, приёмка T-476; версия 0.1.7)**: T-476 — `done`. В разделе T-476 проверка слияния T-059 заменена поиском коммита слияния задачи (`git log --merges … | grep -F 'Merge task/T-059-'`): прежний `grep T-059` по журналу `develop` находил служебные коммиты. Добавлены итог приёмки, передача, бэклог и ссылка на отметку по файлам EPIC-001 из T-059. Тексты прочих разделов не менялись.
**Правки 2026-09-14 (tech-lead#1, приёмка T-482; версия 0.1.8)**: добавлен раздел T-482 (`done`; раздела не было — задачу завёл оркестратор, номер выдан им). В разделе: итог приёмки, передача (порядок слияния, `integration` красный до T-483) и бэклог п. 1–6; п. 3 закрыт при приёмке, п. 4 сведён с п. 2 бэклога T-454. Тексты прочих разделов не менялись.
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

### T-398: Свести раскол `docs/` и `Docs/`, вычистить устаревшие файлы-инструкции · Размер: M · Статус: done · Волна 1
- **Состав**: 6 файлов под строчным `docs/` против 83 под `Docs/`; `QWEN.md` (723 строки прежней картины), `AI_AGENT_INSTRUCTIONS.md`, `README_LIVING_WORLDS.md`, `AUTOMATION-SETUP.md`.
- **Причина**: на файловой системе с различением регистра это два каталога; перечисленные файлы читаются агентами как инструкции и описывают систему, которой нет.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #2 (Nit 2). 11 файлов перенесены `git mv` в `Docs/archive/`, все R100: 6 из `docs/`, 4 корневых и, по Mj-1 ревью #1, `Docs/LIVING_WORLDS_IMPLEMENTATION_STATUS.md`. Хэши совпадают, файлов 968 до и после, путей `docs/` в индексе нет. Таблица README архива совпадает с фактом. Nit N-1 (`dev-log.md:8286`) и N-2 (карточка, «четырёх пунктов») исправлены при приёмке. Исполнитель — tech-writer#1, ветка `task/T-398-docs-case-split`. Подробно — карточка `tasks/T-398.md`, «Приёмка».
- **Бэклог (из ревью T-398, 2026-09-13)** — не блокирует:
  - `architecture/infrastructure.md:1044`, `:1106`: `docs/ops/runbook.md` строчными → `Docs/ops/runbook.md`. Владелец — devops-engineer; удобно закрыть в T-399, которая правит тот же файл.
  - `architecture/overview.md:5` (system-architect) и `project/glossary.md:8` (BA) указывают на `Docs/LIVING_WORLDS_*.md` → `Docs/archive/LIVING_WORLDS_*.md`. Одна строка у владельцев при следующей правке.
  - `Docs/agent-gm-research/` (4 файла) по A3 (`open-questions.md:754`) исторический, но в списке кандидатов `Docs/archive/README.md` отсутствует. Добавить при следующей гигиенической задаче tech-writer вместе с оставшимися кандидатами (`Docs/architecture*.md`, `Docs/EVENTS-MIGRATION.md`, `PULL_REQUEST.md`, `memory/`, `plans/`, `reports/`).
  - Битая ссылка `.claude/plugins/multiverse-core-plugins/README.md:175` на `AUTOMATION-SETUP.md` — вопрос владельцу (файл владельца), передан оркестратором.

### T-399: Привести `infrastructure.md` к состоянию после T-397 · Размер: S · Статус: done · Волна 1 · Исполнитель: architect#1
- **Причина (Mi-2 ревью T-397)**: §1.3, §2.2, §3.1.1 описывают прежнюю топологию, а §4.2 хранит эталон `.env.example` ровно в запрещённом теперь формате — следующий исполнитель воспроизведёт дефект «по дизайну».
- **Состав**: топология трёх compose-файлов и причина разделения; правило 7 в списке правил линтера; запрет инлайн-комментария после пустого значения и пометка обязательных переменных в §4.1/§4.2.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/0/2/3) и итерации 2 без повторного ревью. Все три пункта «Состава» сверены с деревом своими проверками: §1.3 и §2.2 — с тремя compose-файлами и `Makefile`, §3.1.1 — с кодом `compose-lint.sh`, эталон §4.2 — с `.env.example` по ключам, значениям и семи `[required]`. `compose-lint` зелёный (8 правил, 15 сервисов), `--fixtures` — 52 bad, 10 good. §6 побайтно равен базе, история версий не тронута. Mi-1, Mi-2, N-1…N-3 и строчный `docs/ops/runbook.md` закрыты. При приёмке правлена одна строка §3.1.1 п. 3: «подстановка своей переменной» была строже скрипта и противоречила `NEO4J_AUTH: neo4j/${NEO4J_PASSWORD:?…}`. Слияние с эпиком (T-437, T-440, T-398): `infrastructure.md` и `tasks.md` без конфликта, `dev-log.md` и `review.md` — конфликт дописывания в конец, сохранить обе стороны. Ветка — `task/T-399-infrastructure-after-t397`. Подробно — карточка `tasks/T-399.md`, «Приёмка».
- **Бэклог (из ревью #1 T-399, не блокирует)**:
  1. `infrastructure.md` §9 сверен с `Docs/ops/runbook.md` только в местах про профили `bot`/`legacy` и `-f` файлов профиля. Остальной §9 — заготовка, у которой есть более свежая копия в runbook: сверить целиком или сократить до ссылки. Исполнитель — tech-writer вместе с devops-engineer.
  2. `build/versions.env:10`: шапка говорит «CI — `cat build/versions.env >> "$GITHUB_ENV"`», а `go.yml` читает файл через `grep -E '^[A-Z][A-Z0-9_]*='`. Правка в одну строку, исполнитель — devops-engineer.
  3. Из карточки T-399: `infrastructure.md` §2.2 — `make build` («три бинарника», в `cmd/` два), `make contracts` (нет `blueprint validate`), `make secrets-scan` (индекс, не рабочая копия), проверка LLM в `make up` (T-404); §0/§2.3 — «один образ с тремя бинарниками».
  4. Из приёмки T-399: `infrastructure.md` §1.1 (`:72`, `:77`) даёт `prod` с `COMPOSE_PROFILES=memory,bot` без оговорки N-3 про EPIC-004. §8 п. 1 и п. 3 описывают деплой и откат прямым `docker compose up -d --wait`, а `make deploy`/`make rollback` идут через `$(COMPOSE)`: при `bot` в наборе прямая команда bot-файл не подключит. Закрыть вместе с п. 1, тем же исполнителем.

### T-400: `Harness` v0 — методы `Attack` и `Flee` и боевой шаг сценария · Размер: S · Статус: done · Волна 1, бридж-блок, ДО T-219
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

### T-403: Стендовый прогон команд README на чистой машине · Размер: S · Статус: done · Волна 1 · Требует владельца · БЛОКИРУЕТ критерии готовности эпика
- **Причина**: GNU make на машине не установлен, ни одна цель не исполнялась ни разу, CI цели `make` не вызывает. Правки Makefile из T-397 проверены только эмуляцией оболочкой.
- **Состав**: установить make; `make -n up PROFILES=bot`, `make -n up PROFILES=memory,legacy`, `make compose-lint`; затем настоящие `make ci` и `make up`.
- **Что без этого не закрывается**: три критерия готовности эпика — «`make ci` зелёный», «`make up` поднимает инфраструктуру и пустые контексты с ответом на проверке здоровья», «команды README выполняются на чистой машине».

### T-404: Эксплуатационная обвязка LLM не должна предполагать llama.cpp · Размер: M · Статус: done · Волна 1 · architect#1 + devops-engineer
- **Уточнение владельца 2026-09-11**: провайдером LLM может быть что угодно, совместимое с OpenAI, — сейчас локальный unsloth на базе llama.cpp, но равно допустимы официальное облачное API, Qwen, Claude и любое другое. Среда выполнения это УЖЕ учитывает: `MV_LLM_PROVIDER` (`openai_compat|ollama|anthropic|recorded|fake`), `MV_LLM_URL`, `MV_LLM_API_KEY`, шлюз на нелокальный адрес по `MV_LLM_CLOUD_ENABLED` (ADR-005 доп. 2 п. 3). Не учитывает этого **обвязка**.
- **Что предполагает локальный llama.cpp и ломается на любом другом провайдере**:
  1. `make llm-up` и `make llm-down` запускают и останавливают нативный процесс. Для облачного или чужого сервера останавливать нечего, а `llm-up` вводит в заблуждение: цель обязана честно сказать «этот провайдер не запускается отсюда».
  2. Сверка закреплённой сборки (`LLAMACPP_BUILD` против `MV_LLM_BIN`) для не-llama.cpp неприменима в принципе. Нужен другой способ закрепить версию среды выполнения: для облака это модель и её версия из ответа, а не бинарник. Сегодня на стенде это видно как «MV_LLM_BIN не задан, сравнить нельзя».
  3. Отчёт о видеопамяти имеет смысл только для локального процесса.
  4. Точка `/health` принадлежит llama.cpp, а не контракту. Частично исправлено в T-403 (при 404 переход на `/v1/models`), но правка сделана только в PowerShell-варианте — `scripts/llm-server.sh` требует того же.
  5. **Два источника истины для одного адреса**: платформа берёт адрес из `MV_LLM_URL`, а скрипт собирает его из `MV_LLM_PORT`. Ровно из-за этого на стенде проверка стучалась в 1234, когда сервер слушал 8888. Обвязка обязана выводить адрес пробы из `MV_LLM_URL`, подменяя `host.docker.internal` на `127.0.0.1` для проверки с хоста.
- **Итог задачи**: цели `llm-*` и проверка здоровья ведут себя осмысленно для каждого значения `MV_LLM_PROVIDER`, включая облачные; способ закрепления версии среды выполнения описан для каждого случая; адрес имеет один источник истины.

### T-405: Стенд паритета двух реализаций скриптов — в репозиторий и в CI · Размер: S · Статус: done · Волна 1
- **Причина**: паритет варианта для оболочки и варианта для PowerShell — это свойство, которое дважды за одну задачу оказывалось нарушенным (ревью T-404 нашло 10 расхождений из 24 входов, в том числе противоположные коды возврата на одном входе). Стенд, доказавший 0 расхождений на 32 входах, собран во временном каталоге сессии и при следующей правке скриптов будет собран заново — то есть свойство снова не будет держаться ничем.
- **Состав**: положить прогон паритета и подставные сервер и бинарник в репозиторий (`scripts/lib/` либо `testdata/`), звать из CI. На линуксе PowerShell может отсутствовать — тогда прогоняется половина для оболочки плюс сверка, что тексты сообщений в двух файлах совпадают.
- **Ссылки**: раздел «T-404 · ревью #1» в `review.md` (матрица на 24 входа), отчёт итерации 2 (таблица на 32 входа).
- **Приёмка (tech-lead#1, 2026-09-13): принята.** Основание: ревью #1 («вернуть») и ревью #2 («принять»); итерацию 3 проверил сам. Мои прогоны:
  - `make scripts-parity` — rc=0: 97 passed, 2 known-failing, сводка K01 10/16 и 6 UNCOVERED, K02 7/7, порты 34/34;
  - то же с `-pwsh off` — rc=0, 2 SKIP;
  - `make parity-mutants` — rc=0: M00 убит, M01 зелёный, M02–M17 убиты, M17 — тремя строками `FIXED-BUT-MARKED` в сводке;
  - `make ci BASE=develop` — rc=0.
  
  Слушатели TCP до и после прогонов совпали, запросов к `:8888` не было. Решения приёмки:
  - K01: stdout `llm-bench` — только таблица, все строки `bench:` — в stderr, как в `.sh` (строка бэклога ниже);
  - `scripts-parity` в required checks `main`/`develop` — решение пользователя, строка ниже с условием про триггеры `go.yml`.
  
  Подробности — в карточке, раздел «Приёмка».
- **Бэклог**:
  - (из T-434, 2026-09-12) Доля кэша в консольной таблице `llm-bench` у близнецов разная: на входе 348/800 `.sh` печатает `0.43` (`round` питона), `.ps1` — `0.44` (`[math]::Round`); `llm-bench.sh:956-958`, `llm-bench.ps1:704-705`. Считать одинаково (например, в целых промилле) либо через общее правило форматирования и закрепить сценарием стенда паритета (Nit N-1 ревью #1 T-434). *(закрыто в T-405: `Get-BenchRound` = `round` Python для всех округлений `.ps1`, сценарии B01/B02)*
- **Бэклог (из T-405, 2026-09-13)** — отдельные задачи, не блокируют:
  - K01 стенда: **правило потоков `llm-bench` выбрано на приёмке T-405 (tech-lead#1, 2026-09-13): stdout — только таблица результатов (заголовок и строки ячеек), всё с префиксом `bench:` — ход прогона, сборка, пропуск фазы, пути отчёта и CSV, «this is ONE run», предупреждения и завершающие сообщения `Stop-Bench` — stderr; как в `.sh`.** Задача: привести `llm-bench.ps1` (16 строк `Write-Host` → stderr, таблица остаётся в stdout, код выхода 2 у `Stop-Bench` сохраняется) и снять пометку K01 — убрать 16 пунктов из `known.go` (включая шесть `withoutScenario`), `Reports` и сам сценарий K01; стенд сам покажет каждый исправленный пункт строкой `FIXED-BUT-MARKED` и кодом 1, пока пометка не снята. В той же задаче: записать правило в шапках обоих скриптов замера и в `ops/metrics/README.md` (с оговоркой, почему замер отличается от правила `llm-server`/`llm-endpoint` «здоровый вывод — stdout»: продукт замера — таблица и файлы, диагностика не должна смешиваться с таблицей), там же одной строкой — три правила вывода, выровненные T-405 без описания в документах оператора: CSV из одного заголовка не остаётся, отказ `nvidia-smi` даёт пустую `vram_used_mb`, пути в сообщениях — относительно корня репозитория; в runbook §3 — что значат `UNCOVERED` и `FIXED-BUT-MARKED` в выводе стенда. Размер S, исполнитель devops-engineer; вместе с ней — строки ревью #1 п. 2 (`messages` → `streams`) и ревью #2 N-1 (мутант на порядок строк) ниже.
  - K02 стенда: типы значений JSON-отчёта, ровно семь путей: `meta.num_ctx`, `meta.n_per_cell`, `meta.repeats`, `meta.runs_required`, `meta.first_call_ms`, `requests[].http` — строка в `.sh`, число в `.ps1`; `cells[].vram_used_mb` — число в `.sh`, строка в `.ps1`. Решение о переписывании отчётов T-402 (бэклог T-434). После правки убрать пункты из `known.go` и `Reports` сценария K02.
  - **Решение пользователя** (настройка GitHub владельца, как `race`, T-401): внести задание `scripts-parity` в required checks веток `main` и `develop` (и `integration/mvp-1`, пока правило защиты на ней действует). Условие для `develop`: сейчас `.github/workflows/go.yml` не запускается ни на `push`, ни на `pull_request` в `develop` (в триггерах только `main`, `integration/**`, `epic/**`) — required check, который не отчитывается, навсегда заблокирует PR (об этом предупреждает комментарий в самом `go.yml`); добавить `develop` в триггеры — отдельной задачей devops до включения защиты.
  - `sum()` Python 3.12+ компенсирует ошибку округления (Neumaier), `llm-bench.ps1` складывает подряд: суммы `latin`/`predicted_ms` длинной ячейки могут разойтись в последнем бите и сдвинуть округление. Считать одинаково в обеих реализациях и закрепить сценарием с дробными значениями на границе.
  - Сценарий PID-файла с живым чужим процессом — только для Linux (CI): под Git Bash `kill -0` не видит номер нативного процесса Windows. Там же — сценарий `Get-ReportPath` с коротким именем 8.3 внутри репозитория (бэклог карточки T-434 п. 3), только для Windows.
  - Первый прогон `scripts-parity` на `ubuntu-latest` — первая проверка обеих половин на Linux: если он покраснеет на `up`/`down` `.ps1` (`Start-Process` без окна, разбор строки аргументов .NET), это дефект Linux-ветки скрипта, а не стенда.
  - (ревью #1 T-405, предложение 1) Линтер стенда: отдельный запуск `golangci-lint` для `testdata/script-parity` с урезанным набором (без `forbidigo`, без errcheck на `Close`), чтобы код стенда не деградировал вне поля зрения. Сейчас цель `scripts-parity` держит только `go vet`.
  - (ревью #1 T-405, предложение 2) Когда K01 будет решён, в сценариях замера сравнивать порядок сообщений обоих потоков целиком, а не отсортированный набор (`messages` заменить на `streams` без пунктов).
  - (ревью #1 T-405, предложение 3) `json-fields` сверяет объединение путей по всем записям `requests[]`: поле, пропавшее в одной записи из многих, не ловится. Сверять набор путей на каждую запись.
  - (ревью #1 T-405, предложение 4) Относительный `--out-dir`/`-OutDir`: `.sh` берёт путь от текущего каталога, `.ps1` — от корня репозитория (`Resolve-RepoPath`). Появилось до T-405; нужен сценарий с рабочим каталогом, отличным от корня, и одно правило.
  - (T-405, итерация 2) `-OutDir` с `..` внутри репозитория: `.ps1` печатает нормализованный относительный путь (`Get-RepoRelativePath`), `.sh` — путь как задан (`${path#"$repo_root/"}`). Записано в комментарии к функции; привести к одному, если это станет видно оператору.
  - (ревью #2 T-405, предложение 2) Сценарии для шести непокрытых пунктов K01 (`withoutScenario` в `known.go`): `/health` 503 с последующим 200 (`loading`), 503 дольше 180 с через уменьшаемый в тесте таймаут (`still loading`), не-200 на `/v1/models` (`not an endpoint`), ошибки части запросов (`had N failed requests`, `produced no usable answer`), ошибка переменной адреса конфигурации (`endpoint variable`). Когда сценарий появится, стенд потребует снять пометку.
  - (ревью #2 T-405, N-1 и предложение 3) Проверка порядка строк внутри потока сейчас ничем не нагружена: общих для двух половин строк в одном потоке почти нет. После решения K01 добавить мутант, меняющий местами две строки одного потока; до того в §3.1.2 это оговорено словами «порядок общих строк».
  - (ревью #2 T-405, N-2 и предложение 4) `walkJSON` не посещает пустые `[]`/`{}`: `[]` против `{}` или против отсутствующего ключа не видят `json-fields`/`json-types`/`json-values` — типичная ошибка `ConvertTo-Json`. Посещать пустой контейнер и возвращать `array`/`object` в `kind`.
  - (ревью #2 T-405, N-3) Режим мутантов: дочерний стенд запускать с контекстом и не печатать прерванного ребёнка как `BROKEN`; в строке «interrupted after N of M» не считать выполненным сценарий, вернувшийся с «interrupted».
  - (T-405, итерация 2) Прерывание стенда (Ctrl+C, отмена задания) проверено чтением кода, не прогоном: на Windows сигнал консоли из этой среды не посылается. Проверить на первом отменённом прогоне CI или вручную в терминале владельца.
  - (приёмка T-405) `infrastructure.md` §6.4 устарел относительно реализации: эскиз `llm-bench.ps1` считает доли `[math]::Round` (правило теперь — `round` Python, §3.1.2), п. 6 «Правила прогона» описывает `.sh` на `jq`/`awk` и говорит, что расхождение колонок «ловится ревью при F-8», — теперь его держит стенд `make scripts-parity`. Пометить эскиз как исторический и сослаться на §3.1.2; область architect#1, размер XS.

### T-406: Удалить `readySubscriber` из заглушки состояния, закрепить гарантию первого офсета кейсом контракта · Размер: S · Статус: done · Волна 1, бридж-блок
- **Причина**: C-01 v1.2 (ADR-022) признал гарантией контракта то, что новая группа читает журнал с первого офсета — её дают обе реализации (адаптер подписывается с первого офсета, шина в памяти ведёт курсор с нуля). Раз гарантия есть, рукопожатие готовности не нужно никому, и необязательный интерфейс, введённый в T-017 как обходной путь, становится лишним.
- **Состав**: удалить интерфейс, ветку его выбора и комментарий из `shared/testkit/state/state.go`; удалить тест-«часовой» о том, что шины дерева готовность не сообщают, из `state_test.go`; добавить в контрактный набор кейс «группа, созданная ПОСЛЕ публикации, получает событие с первого офсета» и прогнать на обеих реализациях.
- **Почему отдельной задачей, а не довеском к T-053**: другой пакет, другой эпик-владелец и другая метка — `shared/testkit/state` принадлежит EPIC-001, а `internal/mechanics` из T-053 принадлежит EPIC-002. Рекомендация architect#1, принята.
- **Ссылки**: ADR-022, `contracts.md` C-01 v1.2, запись T-017 в dev-log.
- **Бэклог (из T-406)**: ADR-022 запрещает топикам платформы политику `latest`. Если в композиции появится настройка `auto.offset.reset`, её стоит ловить линтером композиции. Сейчас такой настройки нет, поэтому это наблюдение, а не задача. Прогон нового кейса контракта на Redpanda — вместе с T-394.

### T-408: Режим и вид шины — один источник истины · Размер: S · Статус: done · Волна 1
- **Причина (найдено при рисовании диаграмм, проверено оркестратором)**: `MV_MODE` и `MV_BUS` объявлены в манифесте, передаются композицией в контейнеры и НЕ ЧИТАЮТСЯ НИКЕМ — в коде нет ни одного обращения к ним. Настоящий выбор идёт флагами `--mode` и `--bus`, а композиция передаёт только `--contexts`. Оператор, поставивший в настройках режим воспроизведения, получит рабочий режим и не узнает об этом. Словари значений тоже расходятся: манифест объявляет вид шины `redpanda`, флаг принимает `kafka` или `memory`.
- **Как чинить (решение оркестратора)**: тем же способом, что и адрес модели в T-404 — манифест единственный источник, флаг перекрывает его для разработчика. Словарь свести к одному. Тот же разбор применить к `MV_GATEWAY_ADDR` и `MV_MEMORY_ADDR`: объявлены, не читаются, адрес всех трёх процессов берётся из `MV_CORE_ADDR`, а композиция дублирует значение.
- **Класс дефекта**: настройка, которая выглядит применённой и не применяется. Тот же класс, что комментарий, ставший значением переменной (T-397), и адрес, разошедшийся с портом (T-404).

### T-409: Свести документы архитектуры с деревом · Размер: M · Статус: done · Волна 1 · architect#1 + system-architect
- **Причина**: при рисовании диаграмм найдено 24 расхождения документов с кодом. Диаграммы их зафиксировали, но живут они в документах, по которым работают исполнители.
- **Существенное**: из семи контекстов написан один, а `components/*.md` описывают ответственность всех как реализованную; нет каталогов блупринтов, законов, конфигурации и схем агентов, на которые ссылаются пять контрактов; `shared/agent` остался прежним, ни один целевой файл не создан; **`shared/agent/levels.go` не существует, хотя контракт объявляет его единственной истиной таблицы владения и требует блокирующего теста равенства с копией — фактической истиной служит копия, ровно то, что процедура запрещает**; имя переменной допущенных клиентов шлюза расходится между контрактом, манифестом и композицией, и значения по умолчанию у манифеста и композиции разные; две переменные ожидания воспроизведения введены контрактом, но в манифест не внесены.
- **Решения оркестратора, которые надо внести тем же проходом**: правило «агент, получивший отказ по конфликту версий, обновляет представление и предлагает заново» — в C-05; «встреча не отвечает на непринятое действие, потребитель различает состояние по событиям встречи» — в C-05 явно; расхождение состава зависимостей рантайма с контрактом — править контракт под код, потому что контекст создаёт хранилище со своими настройками, а мультиплексор уже используется; проверка перехода статуса в себя же — править код под контракт, потому что «ход засчитан, движения не было» уже принятое правило, и повтор действия не должен превращаться в ошибку игроку; подкоманду отчёта в документах привести к зарезервированному имени.

### T-410: Подключить шину, журнал и реестр к runtime.Deps в serve.go; убрать устаревшие комментарии · Размер: S · Статус: done · Волна 1
- **Причина (T-409)**: C-01 v1.3 приведён к коду, но `cmd/multiverse/serve.go` не передаёт контекстам шину, журнал и реестр через `Deps` — контексты EPIC-002 не смогут их получить. Срок — не позже EPIC-002 T-055.
- **Попутно, размер XS**: комментарии, называющие истиной несуществующий `shared/agent/levels.go` (в `shared/contracts/ownership.go` и его тесте; истина — сам `ownership.go`, ADR-025); комментарий над `Deps` про «Store and Env in F-5»; ссылка в `shared/env/vars.go` на `llm-endpoint.psm1`, тогда как файл называется `LlmEndpoint.psm1`. Ещё: doc-комментарий `shared/runtime/http.go:104` говорит `"operator" when unset`, а манифест — `operator,mvctl,ci-harness` (ревью T-411).
- **Метка**: `contract-change`.

### T-411: Композиция задаёт свои умолчания для списков клиентов — второй источник значения · Размер: S · Статус: done · Волна 1
- **Причина (T-409)**: для `MV_GATEWAY_CLIENT_IDS`, `MV_GATEWAY_ACTOR_KIND_CLIENTS` и `MV_CORE_ADMIN_CLIENTS` композиция подставляет собственные умолчания, отличные от манифеста. Тот же класс, что закрыт в T-404 и T-408: два источника одного значения, и оператор не знает, какой решает.
- **Решение оркестратора**: убрать умолчания из композиции; значение приходит из файла настроек и манифеста. Исполнитель — devops-engineer.

### T-412: Голый docker compose не читает COMPOSE_ENV_FILES из .env, а документы обещают обратное · Размер: S · Статус: done · Волна 1
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

### T-414: Подкоманда serve: бинарник её не знает, а документы и задания пишут · Размер: XS · Статус: done · Волна 1 (бэклог)
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

### T-417: C-01 v1.4 в коде: двухшаговый Dedup и id события из причины · Размер: S · Статус: done · Волна 1
- **Причина (T-416, ADR-027)**: `eventbus.Dedup.Seen` спрашивает и запоминает одним шагом — издатель с единственным побочным эффектом вынужден заводить своё окно (нарратор T-220); дубль при потерянном подтверждении публикации не гасится, потому что `Derive` каждый раз берёт новый id.
- **Что сделать**: `Dedup.Has`/`Add` (двухшаговый API, `Seen` остаётся); опция `WithCauseID` у `Derive` — id из причины (UUIDv5 по таблице применимости ADR-027); окна дедупликации пригодны для снапшота C-14 v1.2. Метка `contract-change`.
- **DoD**: тесты на оба механизма; таблица применимости `WithCauseID` из ADR-027 покрыта тестом (неверные `parts` не склеивают разные события); contract-тест шины зелёный на `membus` и Redpanda (T-394). Срок — до T-229.

### T-418: Перенос membus в shared/eventbus/membus · Размер: S · Статус: done · Волна 1
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

### T-434: llm-bench — прогрев через python-помощник, путь матрицы вне репо (имя + sha256), first_call_ms пустой при не-200, timings.prompt_ms на запрос; паритет .ps1 · Размер: S · Статус: done · Волна 1 (бэклог)
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
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/0/0/2). Все пункты DoD проверены своими прогонами на подставном сервере: прогрев с id-путём даёт 200, при 500 `first_call_ms` пуст у обеих реализаций, в `meta` нет абсолютных путей, CSV — 24 колонки с заголовком эпика, в том числе у `.ps1` под `ru-RU`. Мутанты MA и MB красные; двойной MD (без культуры и без инвариантного форматирования) красный, одиночные m8 и MC выживают, потому что защиты две. Подробно — карточка `tasks/T-434.md`, «Приёмка».
- **Бэклог**:
  - (из T-434, 2026-09-12) Строка stderr `bench: prompts …, matrix …` (`llm-bench.sh:745`, `llm-bench.ps1:318`) печатает абсолютный путь матрицы и промптов. Печатать то же, что уходит в отчёт: имя файла или относительный путь плюс короткий префикс `sha256` (Nit N-2 ревью #1). Решать вместе с типами в JSON: в `.sh` все значения `meta` — строки, в `.ps1` — родные типы; `requests[].http` — `"200"`/`"000"` против `200`/`0`. Правка типов меняет формат существующих полей, поэтому нужно решение, переписывать ли отчёты T-402 (п. 1 бэклога исполнителя).

### T-435: U-2 — базовая конфигурация LLM по трём зачётным прогонам E; включать ли Qwen3.6-35B-A3B в матрицу · Размер: S · Статус: done · Волна 1
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

- **Итог (итерация 2, 2026-09-12)**: базовая на переход — E, целевая конфигурация нарратива — Qwen3.6-35B-A3B (ADR-005 дополнение 3, решение пользователя). Вынесены T-437, T-438, T-439 (ниже). OQ-A-18 закрыт.

### T-437: `--alias` в `llm-server.{sh,ps1}`, пин `LLAMACPP_BUILD`, сокращение `ops/models.txt` · Размер: S · Статус: done · Волна 1 (бэклог)
- **Причина (T-435, бэклог T-402 п. 1)**: `scripts/llm-server.{sh,ps1}` передают `-m "$MV_LLM_MODEL_FILE"` без `--alias`, и `llama-server` отдаёт в `/v1/models` путь к файлу вместо stem. Это ломает не только замер: на пути платформы валидатор блупринтов (C-11 правило 7а) и `Health` провайдера (`degraded(model_not_resident)`) сверяют модель блупринта со списком `/v1/models`, а `llm-bench` пропускает фазы («model … is not in /v1/models — phase … skipped», `llm-bench.sh:743`, `llm-bench.ps1:384`). Плюс два пункта чек-листа `baseline.md` §5, которые нельзя закрыть до стендовой сессии.
- **DoD**:
  - `--alias <stem MV_LLM_MODEL_FILE>` в обоих скриптах, порядок аргументов и текст сообщений — в паритете (T-405); в router-режиме (`--models-dir`) алиас не добавляется;
  - `make llm-up` на стенде даёт `/v1/models` = `Qwen3.8-27B-UD-Q3_K_XL`; проверка — в карточке, стенд владельца не требуется (подставной бинарник);
  - `LLAMACPP_BUILD` в `build/versions.env` поднят до билда контрольного прогона T-438 (зачётные прогоны T-402 — `b10878-4850c7727`, пин — `b10441`); сверка пина — T-404 п. 2, не дублировать;
  - `ops/models.txt` сокращён по итогу T-438: секция `gguf` — E и Qwen3.6-35B-A3B (целевая), теги Ollama остаются только за C/A как за запасными;
  - `make ci` зелёный, dev-log заполнен.
- **Зависимости**: сокращение `ops/models.txt` и пин — после T-438; правка `--alias` нужна **до** T-438.
- **Исполнитель**: devops-engineer. Ветка `task/T-437-llm-alias-and-pin`.
- **(приёмка tech-lead#2, 2026-09-13)** Принята после ревью #1 (0/0/0/3). В single-режиме оба скрипта передают `--alias` с именем файла без `.gguf`, в router-режиме алиаса нет; правило имени у `.sh` и `.ps1` совпало на моих входах. Пин `b10878` и `ops/models.txt` сделаны до T-438 по поручению оркестратора, это отклонение от DoD принято. N-1 и N-2 закрыты правкой текста при приёмке (`versions.env`, `infrastructure.md` §6.5), N-3 — в бэклог. Фактическая ветка — `task/T-437-llm-server-alias`. Подробно — карточка `tasks/T-437.md`, «Приёмка».
- **Бэклог (из T-437, 2026-09-13)** — отдельные задачи, не блокируют:
  - N-3 ревью #1: в `ops/models.txt:24` у Qwen3.6 указан источник «unsloth (Hugging Face)» без точного репозитория, а по файлам проекта он не устанавливается. Выяснить у владельца при сессии T-438.
  - К T-438, как последствие отклонения: в начале сессии сверить `build_info` из `/props` с `LLAMACPP_BUILD`, после исхода привести `ops/models.txt` к `decision` матрицы.
  - `.ps1` рвёт аргумент с пробелом: `Start-Process -ArgumentList` склеивает без кавычек, задеты `-m`, `--models-dir`, `--slot-save-path`. Дефект старше T-437, сценарий паритета — T-405.
  - Тело прогрева в `llm-server.sh` собирается строкой оболочки, а `"` в имени файла даст 500. Собирать python-помощником, как в T-434.
  - `make llm-health` не сверяет `/v1/models` с `LLM_MODEL_DEFAULT`, и сервер без `--alias` выглядит зелёным. Строка алиаса в `up` сообщает намерение скрипта, а не ответ сервера (m1 ревью #1).
  - Отставшие тексты: ADR-005 «Уточнение исполнения» п. 6, отметки T-437 в `baseline.md` §5, `_model_ids` в `bench-matrix.json`. Правит architect#1.
  - Размер весов E «13,1 ГБ» в `infrastructure.md` (`:227`, `:818`, §6.4 п. 4) расходится с 13,4 ГБ по байтам файла стенда (`baseline.md` §1, `ops/models.txt`). §6.4 правит architect#1, остальное — devops-engineer.

### T-438: Стендовая сессия: контрольный прогон E и три прогона Qwen3.6-35B-A3B · Размер: S · Статус: todo · Волна 1 · Требует владельца
- **Причина (T-435, ответы владельца на В-1/В-4)**: зачётные прогоны E шли в ячейке вне матрицы (4 × 200 192) и на KV `q4_0` — два отклонения от `infrastructure.md` §6.4 п. 3. Ячейку платформы (8192 × 1, f16) не измеряли ни разу. Целевая конфигурация нарратива (Qwen3.6) имеет один справочный прогон, а по методике нужны три.
- **Сессия одна, перезапуски делает владелец. Порядок и что решает каждый прогон**:

  | # | Прогон | Ячейка и сервер | Команда (основная — `.ps1`, стенд на Windows) | Что решает |
  |---|---|---|---|---|
  | 1 | E, контрольный | `MV_LLM_NUM_CTX=8192`, `MV_LLM_SLOTS=1`, без `--cache-type-*` (f16), `--alias Qwen3.8-27B-UD-Q3_K_XL`, другой день относительно 2026-09-12 | `pwsh scripts/llm-bench.ps1 -Configs E -NumCtx 8192 -KvCache f16` · `bash scripts/llm-bench.sh --configs E --num-ctx 8192 --kv-cache f16` | доказывает ячейку платформы: переход `q4_0` → `f16` и окно 200 192 → 8192 не ухудшают `phase2`; закрывает оговорки 3, 4 и 6 `baseline.md` §3 |
  | 2–4 | Qwen3.6-35B-A3B, три прогона | та же ячейка, файл `Qwen3.6-35B-A3B-UD-Q3_K_XL.gguf`, `--alias Qwen3.6-35B-A3B-UD-Q3_K_XL`; перезапуск сервера перед каждым прогоном либо разные дни | `pwsh scripts/llm-bench.ps1 -Configs Q36-A3B -NumCtx 8192 -KvCache f16` · `bash scripts/llm-bench.sh --configs Q36-A3B --num-ctx 8192 --kv-cache f16` | принимает или отвергает целевую конфигурацию нарратива: три `pass` по `phase2` **и** хвост «прочего» > 2 с исчез или объяснён по `prompt_ms` |
  | 5 | Калибровка (не прогон) | сервер уже поднят | `POST {MV_LLM_URL}/tokenize` с русскими текстами (например, поле `system` из `testdata/bench/prompts.jsonl`) для обеих моделей | даёт «символов на токен» для `text.maxLength` в T-439 вместо допущения 2,5 |

- **Термины исходов** (ADR-005 «Уточнение исполнения» п. 2, одинаково во всех документах):
  - **«E прошла»** = `verdict=pass` по NFR-002 (p95 ≤ 5000 мс) **и** p95 ≤ 4400 мс **и** медиана ≤ 3000 мс. При n = 10 p95 прогона — его самый медленный запрос, поэтому граница 4400 мс сама по себе сработала бы от одного выброса.
  - **«E прошла по порогу, но не по сигналу»** = `pass` по NFR-002 при p95 > 4400 мс или медиане > 3000 мс. Это сигнал к пересмотру, а не отказ: ячейка платформы (`f16`, 8192) оказалась медленнее зачётных прогонов на `q4_0`.
  - **«Qwen3.6 принята»** = три `pass` по `phase2` **и** хвост «прочего» > 2 с исчез либо объяснён по `prompt_ms`.
- **Исходы и что из них следует**:

  | Исход | Следствие |
  |---|---|
  | E прошла, Qwen3.6 принята | Qwen3.6 становится базовой и целевой: `decision.chosen` → `Q36-A3B`, блупринты и `schemas/agent/narrative.json` переводятся одной правкой данных (T-260), потолок — финальные числа T-439 |
  | E прошла, Qwen3.6 не принята | E остаётся базовой с потолком 160 токенов; вопрос «нарратив длиннее или порог NFR-002 5 с» возвращается владельцу (ADR-005 доп. 3 п. 4) |
  | E прошла по порогу, но не по сигналу (при любом исходе Qwen3.6) | Решение U-2 не отменяется автоматически: сначала разбор «прочего» и `prompt_ms` контрольного прогона против зачётных, строка о разнице `q4_0` против `f16` — в `baseline.md`. Дальше — architect#1: остаётся ли ячейка платформы прежней |
  | E `fail` по NFR-002, Qwen3.6 принята | Qwen3.6 — базовая сразу; E уходит в запасные; расхождение `f16` против `q4_0` разбирается по `prompt_ms` |
  | E `fail` по NFR-002, Qwen3.6 не принята | U-2 открывается заново: следующая по `decision_order` (C) плюс вопрос владельцу о длине и пороге |

- **Оговорка о длине ответов (важна для чтения исходов).** Промпты замера просят короткий нарратив: в справочном прогоне Qwen3.6 ответы заняли 68–111 токенов соло и 132–212 в группе при целевых ≈ 220 и ≈ 410. Значит, три `pass` получены на ответах примерно вдвое короче целевых, и **NFR-002 при новом потолке держит расчёт, а не замер**. Закрывается так: в прогонах 2–4 хотя бы один промпт идёт с `max_tokens` под целевой потолок и заданием на длинный нарратив (правка копии `testdata/bench/prompts.jsonl` вне репозитория, как копия матрицы в T-402), а если это не сделано — в `baseline.md` пишется прямая оговорка «потолок подтверждён расчётом; проверка — `narrative_latency_p95_ms` в S1/S2».

- **DoD**: отчёты в `ops/metrics/`; строки прогонов в `baseline.md` §2 и `results` матрицы (architect#1); `decision` матрицы приведён к исходу; вопросы владельцу, если исход требует, — через оркестратора.
- **Плюс к DoD (tech-lead#1 при приёмке T-435, 2026-09-12; предложение ревью #2)**: после сессии записать в `baseline.md` отдельной строкой разницу `q4_0` против `f16` по `phase2` (контрольный прогон против зачётных 2–4) — это первый замер влияния типа KV на этом стенде, и он понадобится, если окно или тип KV придётся менять.
- **Плюс к DoD (ревью #1 T-440, решение оркестратора)**: `ops/metrics/README.md:89` — пример `--configs` с `Q36-A3B` (подготовка сессии).
- **Зависимости**: **T-434** (прогрев, `prompt_ms`, путь матрицы) и **T-437** (`--alias`). Без T-437 сервер поднимается вручную с `--alias`, иначе скрипт пропустит фазы.
- **Исполнитель**: оператор прогона — devops-engineer или оркестратор; перезапуски сервера и модель — владелец. Ветка для заполнения артефактов — `task/T-438-stand-session-e-q36`.

### T-439: EPIC-003 — потолок длины нарратива под целевую конфигурацию · Размер: S · Статус: done · Волна 1 · Исполнитель: architect#2 (EPIC-003)
- **Причина (T-435, ADR-005 доп. 3)**: КД `components/swarm-llm-laws.md` §13.3 задаёт `player-gm llm.phase2 max_tokens: 700` и §13.4 — `narrative.json text.maxLength: 1500`; `group-narrator` (`:736`) — «как у `player-gm`». Ни одно из этих значений не проходит NFR-002 ни на базовой E (≈ 163 токена в 5 с), ни на целевой Qwen3.6 (≈ 220 токенов при нынешнем хвосте). Плюс в КД осталась стартовая модель C (`:735` и таблица `:977`), хотя `design.md` и DoD T-203 уже говорят E.
- **Что сделать**:
  1. `max_tokens` фазы и `maxLength`/`maxItems` схемы привести к конфигурации: E (на переход) — 160; Qwen3.6 (целевая) — предварительно 220, окончательно после T-438 (≈ 410, если хвост исчезнет).
  2. Правило расчёта записать формулой: `text.maxLength` ≈ (`max_tokens` − каркас JSON − служебные поля) × символов на токен × 0,9, где каркас ≈ 30 токенов (ключи, скобки, `kind`/`tone`), служебные поля ≈ 36 токенов (`mentions` `maxItems` 4 и `background_refs` `maxItems` 2, элемент ≈ 6 токенов) — вместе ≈ 66 токенов. Проверка формулы: E (160 − 66) × 2,5 × 0,9 ≈ 210 символов; Qwen3.6 (220 − 66) × 2,25 ≈ 345 и (410 − 66) × 2,25 ≈ 775. «Символов на токен» — из калибровки T-438 (шаг 5), до неё допущение 2,5. Другие `maxItems` — пересчёт по той же формуле.
  3. Модель в КД `:735` и `:977` — по `ops/metrics/baseline.md`, запасные — по `decision_order` матрицы (E → Qwen3.6 → C → A).
  4. Что делает T-203, если стартует раньше T-438: берёт значения базовой E (160) и комментарий-указатель на `baseline.md` §5; смена конфигурации — правка YAML блупринтов и `schemas/agent/narrative.json` одним PR (T-260), Go не меняется.
- **DoD**: КД §13.3–13.4 и строка `:977` не противоречат `baseline.md` §5; DoD T-203 ссылается на значения; тест на стенде — ответ предельной длины не обрезается (схема закрывает строку сама, `maxLength` грамматики json_schema).
- **Зависимости**: числа окончательны после T-438; до старта T-203 нужен хотя бы предварительный набор.
- **Исполнитель и ветка**: architect#2 (EPIC-003), ветка `task/T-439-narrative-length-cap`.
- **Владение (важно)**: задача стоит в индексе EPIC-001 как **передача** — она родилась из решения U-2. Сами правки идут в документы EPIC-003 (КД `components/swarm-llm-laws.md` §13.3–13.4, строки `:735` и `:977`, DoD T-203): вносит их **architect#2**, подтверждает **tech-lead#2**. Архитектор EPIC-001 в чужие документы не пишет. Дублировать ли строку в индексе EPIC-003 — решение оркестратора.

### T-440: Редакционно: overview.md §18.1 и владение infrastructure.md · Размер: XS · Статус: done · Волна 1 (бэклог)
- **Причина (приёмка T-435, tech-lead#1)**: после ADR-005 доп. 3 `overview.md` §18.1 (`:373`, `:551`) по-прежнему называет старый порядок и правило «проходит замер — базовая». `plan/ownership.md` закрепляет `infrastructure.md` за devops-engineer, хотя §6.4 правил architect#1 (T-399, T-435).
- **DoD**:
  1. `overview.md` §15 (строка «Модели») и §18.1 приведены к ADR-005 доп. 3: правило «проходит пороги **и** даёт нужную длину нарратива», E — базовая на переход, `Qwen3.6-35B-A3B` — целевая конфигурация нарратива, порядок E → Qwen3.6-35B-A3B → C → A. Ссылки на ADR-005 доп. 3 и `baseline.md` §5, расчёты не дублируются.
  2. `plan/ownership.md`: владение `infrastructure.md` приведено к факту — devops-engineer — эксплуатация, architect#1 — §6.4. Чужие решения не переписываются.
  3. Грэп по `Docs/` на старый порядок, старый JSON `decision_order` и старое правило: вне записей прошлого (ADR своего времени, журнал, dev-log, ревью, карточки и закрытые вопросы) пусто. Записи прошлого не переписываются.
- **Исполнитель**: system-architect#1 (Opus). Ветка `task/T-440-overview-order-ownership`. Карточка — `tasks/T-440.md`.
- **Бэклог (из ревью #1 T-440, решение оркестратора)**:
  - `requirements/nfr.md:125`, `:154` — варианты E/C/A привести к доп. 3 ADR-005; владелец — BA.
  - Набор грэп-шаблонов старого правила и порядка, со всеми вариантами написания, записать для проверки после исходов T-438. Рабочий набор T-440 — в карточке, раздел «Итерация 2».
- **Бэклог (из ревью #2 T-440, решение оркестратора; внесено tech-lead#1 при приёмке, 2026-09-13)**:
  - N-1 · EPIC-003 `tasks.md:810` (T-260) и `:877`: «или зафиксирован переход на следующую конфигурацию по `decision_order`» оставляет прочтение правила первого прохода. Заменить на «или зафиксирован переход по `baseline.md` §5 (ADR-005 доп. 3)», без слова «следующую». Владелец — tech-lead#2, при следующей правке индекса EPIC-003.
  - N-2 · строки истории версий «T-440, редакционно» в EPIC-003 `design.md:11`, `tasks.md:15` и `plan/epics.md:3` — на усмотрение владельцев (tech-lead#2, tech-lead#1) при следующей версии документа.
  - N-3 · в набор грэп-шаблонов (строка выше) добавить после T-438 три варианта: «стартово C», «по итогам F-8», «если не проходит, C».

### T-441: Delivery — не писать «parked», если запись в dead_letters не удалась · Размер: XS · Статус: done · Волна 1 (бэклог)
- **Причина (ревью #1 T-436, Mi-1; строка бэклога раздела T-436)**: `Delivery.Deliver` (`shared/eventbus/delivery.go:128`) пишет `Warn` «event parked in dead letters» **до** записи в `dead_letters`. Если запись падает, событие не припарковано и не закоммичено, а лог утверждает обратное.
  - После T-436 это путь kafka-адаптера: последняя попытка падающего обработчика заканчивается после `Close`, запись падает на `ErrClosed`, `stopped(loopCtx, …)` превращает её в `nil`. `Subscribe` возвращает `nil`, событие придёт снова, и ложный `Warn` — единственный след.
  - У membus так было и раньше (T-395). Та же ошибка у `DeliverRaw`: «undecodable message parked in dead letters» тоже пишется до записи.
- **Что сделать** (`shared/eventbus/delivery.go`; обе шины одной правкой, потому что обе читают через `Delivery`):
  - строку «parked» писать только после успешной записи в `dead_letters`;
  - при неудачной записи, включая ветку без приёмника, — строку «event not parked in dead letters; it stays uncommitted and will be delivered again» с причиной и ошибкой записи. Уровень — по исходу:
    - шина закрыта (`ErrClosed`, `io.ErrClosedPipe` писателя kafka-go) — `Warn`: это штатный конец подписки под `Close`;
    - запись упала на живой шине — `Error`: подписка падает, топик стоит;
  - возврат ошибок, цепочку ошибки (C-01 v1.6) и коммит не менять; Go-API `shared/eventbus` не менять.
- **DoD**:
  - тест на лог для `Deliver` и `DeliverRaw`:
    - запись удалась → одна строка «parked»;
    - запись не удалась (закрытая шина, закрытый писатель, сбой на живой шине, нет приёмника) → строки «parked» нет, есть «not parked» нужного уровня с причиной;
    - паника с неудачной записью → `Error` паники плюс «not parked»;
  - мутант «вернуть безусловный `Warn` до записи» краснеет; контрольный мутант идёт первым; мутанты — в копии в scratch, не через `-overlay`;
  - `go build ./... && go vet ./...`, `go test -short -count=1 ./...`, `golangci-lint run ./...` зелёные; dev-log заполнен.
- **Метка**: `contract-change` не нужна — Go-API и контракт не меняются, меняются только текст и уровень строки лога. Правка `shared/eventbus` — с ревью system-architect по карте владения.
- **Исполнитель**: developer#1. Ветка `task/T-441-no-false-parked-warn` (от эпика после T-436, `adadca5`).
- **(приёмка tech-lead#2, 2026-09-13)** Принята после ревью #1 (0/0/2/1). Итерация 2 закрыла Mi-1, Mi-2 и N-1, исправления проверены при приёмке. Мои мутанты в копии в scratch: контрольный M0 красный; R1 (`err == io.ErrClosedPipe`), R2 (`err == ErrClosed`), H1 (без `handled`), «безусловный `Warn` до записи» на обоих путях, перевёрнутый уровень и `busClosed(cause)` — красные. `go build`/`go vet` — 0, `go test -short -count=1 ./...` — 27 ok, `golangci-lint run ./...` — 0 issues. Подробно — карточка `tasks/T-441.md`, «Приёмка».
- **Бэклог (из T-441, 2026-09-13)** — отдельные задачи, не блокируют:
  - Гонка в `Kafka.Close` (владелец `shared/eventbus`, EPIC-001; ревью #1 T-441, п. 1 бэклога). `closed=true` ставится под `mu` (`kafka.go:348`) раньше отмены `loopCtx` — после `stopReaders()` (`:364`) и в горутине `context.AfterFunc` (`:206`). Запись в `dead_letters` в этом промежутке получает `ErrClosed`, `stopped(loopCtx, …)` его не узнаёт: `Subscribe` возвращает «write dead letter … bus is closed» вместо `nil`, в логе — `Warn` с `bus_closed=true`. Правка: `stopped` принимает `errors.Is(err, ErrClosed)` или адаптер проверяет `k.closing.Err()`; unit-тест на `stopped` и мутант. Задачу заводит оркестратор.
  - README `shared/eventbus`: одной строкой назвать исходы `Delivery` в логе — parked / not parked, `bus_closed`, уровни. Не срочно.
  - Строка `logPanic` «event handler panicked; the event goes to dead letters» пишется до записи в `dead_letters` (NFR-012, C-01 v1.5 п. 3; T-441 её не трогала). При неудачной записи за ней теперь идёт «not parked». Переформулировать ли — вопрос к владельцу метрики `service_panics`.

### T-442: Редакционно: тексты об `--alias`, пине и `ops/models.txt` приведены к слиянию T-437 · Размер: XS · Статус: done · Волна 1 (передача T-437)
- **Причина (бэклог T-437: карточка исполнителя п. 4, ревью #1, приёмка tech-lead#2 п. 6)**: T-437 слита в эпик (мерж `c3bfcd5`). В single-режиме `scripts/llm-server.{sh,ps1}` передают `--alias` с именем файла без `.gguf`, пин `LLAMACPP_BUILD` — `b10878`, `ops/models.txt` — E, Q36-A3B, C, A. Три текста architect#1 всё ещё описывали T-437 как будущую задачу: ADR-005 «Уточнение исполнения» п. 6 («скрипты передают `-m` без `--alias`»), три пункта `[ ]` «вынесено в T-437» в `baseline.md` §5, `_model_ids` в `bench-matrix.json`.
- **Состав**:
  1. ADR-005 УИ п. 6: факт до T-437 — в прошедшем времени. Добавлен абзац «Исполнено (T-437, 2026-09-13; редакционно, T-442)»: что делают скрипты, какие id отдаёт `/v1/models`, чем проверено. Решение «поле-алиас не вводится, id — stem» не менялось. Ссылки на строки `llm-bench` (`:743`, `:384`) устарели, их заменил текст сообщения о пропуске фазы.
  2. `baseline.md` §5: три пункта отмечены `[x]` «выполнено в T-437». Отклонения T-437 (пин и список сделаны до T-438) и сверка `build_info` в начале T-438 названы. К «Кто и когда» T-435 дописана строка состояния на 2026-09-13.
  3. `bench-matrix.json`: текст `_model_ids` — факт T-437 и то, что остаётся за T-438; `updated` → `2026-09-13`. Формат и ключи не менялись, скрипты `_model_ids` не читают.
  - Решение T-435 не менялось. T-438 по-прежнему будущая: нигде не утверждается, что алиас проверен на настоящем `llama-server`.
- **Ссылки**: карточка `tasks/T-442.md`; `tasks/T-437.md` («Бэклог», «Приёмка»); `review.md` «T-437 · ревью #1».
- **Исполнитель**: architect#1. Ветка `task/T-442-t437-followup-texts`.
- **(приёмка tech-lead#2, 2026-09-13)** Принята после ревью #1 (0/0/1/1). Все утверждения трёх текстов сходятся с деревом после мержа T-437. В матрице изменены только `updated` и `_model_ids`, решение T-435 и записи прошлого не тронуты, T-438 нигде не названа проведённой. JSON валиден, `gitleaks dir` чист. Mi-1 («ключи матрицы» → «id моделей») и N-1 («было условием» → «это условие сессии T-438, выполнено T-437») закрыты правкой текста при приёмке. Подробно — карточка `tasks/T-442.md`, «Приёмка».
- **Бэклог (из T-442, 2026-09-13)** — отдельные задачи, не блокируют:
  - Раздел T-438, «Зависимости» (`:845`): «Без T-437 сервер поднимается вручную с `--alias`» — записать зависимость от T-437 как выполненную, оставить требование `--alias` для сервера, поднятого не через `make llm-up`. Владелец — tech-lead#1.
  - `ops/metrics/README.md`, к подготовке T-438: фраза о том, что сервер, поднятый не через `make llm-up`, запускается с `--alias`, иначе скрипт пропустит фазы. Владелец — devops-engineer.
  - Вес E «13,1 ГБ» в `infrastructure.md` §6.4 п. 4 против 13,4 ГБ по байтам файла стенда — в T-438, architect#1 (§2 и §6.2 — devops-engineer, бэклог T-437).

### T-443: Kafka — письмо в `dead_letters`, отвергнутое внутри `Close`, — штатная остановка · Размер: S · Статус: done · Волна 1 (из ревью T-441)
- **Причина (ревью #1 T-441, п. 1 бэклога; подтверждено тимлидом)**: `Kafka.Close` ставит `closed=true` под `mu` (`kafka.go:348`) раньше, чем отменяется контекст цикла подписки: `stopReaders()` (`:364`) вызывается после снятия блокировки, а сама отмена `loopCtx` идёт в горутине `context.AfterFunc` (`:206`). Обработчик, чья последняя попытка упала в этом промежутке, получает отказ записи в `dead_letters` с `ErrClosed`, а `stopped(loopCtx, …)` его не узнавал. `Subscribe` возвращал «write dead letter … bus is closed» вместо `nil`, хотя в логе уже стоял `Warn` с `bus_closed=true`. Это расходится с C-01 v1.2 (под `Close` все `Subscribe`/`Tail` возвращают `nil`) и ADR-023 п. 4. Тот же отказ у `Tail`/`ReadRange` kafka-адаптера возвращался ошибкой и после возврата `Close`.
- **Что сделать** (`shared/eventbus/kafka.go`, без смены Go-API и контракта):
  - `stopped` признаёт закрытую шину по ошибке той же проверкой, что и `bus_closed` в логе (`busClosed`: `ErrClosed` или `io.ErrClosedPipe`), плюс прежние `ctx.Err()` и `io.EOF`;
  - не ломать T-014 (коммит под отменой), T-436 (два контекста), T-441 (уровни и строки parked / not parked), сторож `panic_stop_test.go` (причина письма в цепочку не входит);
  - проверить membus на такое же окно.
- **DoD**:
  1. Unit-тест без брокера воспроизводит окно: `loopCtx` построен как в `Subscribe`, у шины выставлен только флаг закрытия, `deliver` падающего обработчика возвращает `ErrClosed`, `loopCtx` жив. `stopped` обязан вернуть `true`, в логе — один `Warn` с `bus_closed=true`. До правки тест красный, после — зелёный.
  2. Таблица `stopped`: голый и обёрнутый письмом `ErrClosed` → остановка. Сторож `panic_stop_test.go` дополнен паникой с `ErrClosed` и ошибкой, оборачивающей `ErrClosed`: на живой шине это не остановка.
  3. Мутанты в копии в scratch, без `-overlay`, контрольный первым: откат `stopped`, `==` вместо `errors.Is`, распознавание по тексту, потеря `io.EOF` — красные.
  4. `go build ./... && go vet ./...` (и `-tags integration` по `eventbus`/`contract`), `gofmt`, `go test -short -count=1 ./...`, `golangci-lint run ./...` (и `--build-tags integration`) — зелёные. Contract-набор на одноразовой Redpanda (`go test -tags integration ./shared/testkit/contract/...`) — зелёный, контейнеров testcontainers после прогона нет.
  5. README `shared/eventbus`: когда подписка считается остановленной; попутно — исходы `Delivery` в логе (бэклог T-441).
- **Метка**: `contract-change` не нужна: Go-API `shared/eventbus` и формулировка C-01 не меняются, код приводится к C-01 v1.2 и ADR-023 п. 4. Правка `shared/eventbus` — с ревью system-architect по карте владения.
- **Исполнитель**: developer#1 (Opus). Ветка `task/T-443-kafka-close-dead-letter-race` (от эпика, после T-436 и T-441). Карточка — `tasks/T-443.md`.
- **(приёмка tech-lead#2, 2026-09-13)** Принята после ревью #1 (0/0/0/2). Nit N-1 (README обещал больше, чем делает `stopped`) и N-2 (перенос комментария сторожа) внесены при приёмке; ту же неточность «идут вместе» поправил в комментариях `stopped` (`kafka.go`) и `busClosed` (`delivery.go`). Мои мутанты в копии в scratch: контрольный M0, прежний `stopped`, `err == ErrClosed`, причина письма через `%w` — красные. `go build`/`go vet` (и `-tags integration`) — 0, `go test -short -count=1 ./...` — 27 ok, `golangci-lint run ./shared/eventbus/...` (и `--build-tags integration` с `contract`) — 0 issues. Подробно — карточка `tasks/T-443.md`, «Приёмка».
- **Бэклог (из ревью #1 T-443, 2026-09-13)** — отдельные задачи, не блокируют:
  - system-architect, C-01 при следующей ревизии (совместимо): `contracts.md:213` («Почему не `%w`», v1.6) перечисляет `io.EOF` и `io.ErrClosedPipe` как то, что `stopped` принимает в любом месте цепочки. После T-443 туда входит и `ErrClosed`.
  - Владелец `shared/eventbus`, редакционно: комментарии `membus.go:88-90` и `:174-178` говорят, что читатель kafka-адаптера под `Close` падает «with the cancellation or with io.ErrClosedPipe». На деле выборка на закрытом читателе отвечает `io.EOF`, а отказ `dead_letters` — `ErrClosed`. Поправить при ближайшей правке файла.
  - Когда у kafka-адаптера появится подменяемый читатель (интерфейс над `FetchMessage`/`CommitMessages`): unit-тест цикла `Subscribe` с отказом `dead_letters` при живом `loopCtx`. Сегодня то, что циклы решают «остановлен» через `stopped`, держит только недетерминированный кейс contract-набора (мутант R3 ревью выживает). Тестовую точку в production-коде ради этого не заводить.
  - Проверить, может ли `io.EOF` прийти из `w.partitions` (kafka-go v0.4.51 `writer.go:651-654`) в ошибке записи `dead_letters` на живой шине: `stopped` принимает `io.EOF` в любом месте цепочки (унаследовано, T-443 путь не расширяет).

### T-444: Ревизия контрактов 4 — документы · Размер: M · Статус: done · Волна 1 · contract-change
- **Причина.** При сверке планов EPIC-002/003/004 с деревом эпика и в ревью T-214/T-215, T-441, T-443 накопились вопросы к контрактам. Без ответов первые волны трёх команд упираются либо в линтер, либо в схему. Вот эти вопросы:
  - границы depguard против плана EPIC-003: страж и законы, писатель записей, подпакеты `llm`, шаблоны, двойники шлюза;
  - имя поля `config.cloud_enabled`;
  - кодировка хешей;
  - условная обязательность записей LLM;
  - издатель `gm.created`;
  - таймауты HTTP процесса для long-poll;
  - кто регистрирует настоящие контексты;
  - мелкие правки: `ErrClosed`, заголовок C-08, `group_move`, `scope` в payload, `EventRef`, ADR-016;
  - правила на период трёх веток.

  Решения system-architect#1 оркестратор принял без изменений (2026-09-13). Дополнение оркестратора того же дня — вопросы tech-lead#1 из плана EPIC-002: источник предложений bootstrap, регистрация `state`, причина отказа по связке «путь ↔ причина», две ссылки на задачи и владение `test/e2e`, `testdata/fixtures/events`.
- **Что сделать** (только документы; код, схемы, линтер и реестр — T-445 и T-446):
  - `architecture/contracts.md` v0.11 — C-01 v1.8, C-02 v1.5, C-04 v1.4, C-06 v1.2, C-07 v1.3, C-08 v1.4, C-11, C-12 (`WorldVersions`), C-15, §0, §16 п. 8, §17, сводка v0.11 с колонкой «Кто прав» по пунктам 1–11;
  - `architecture/adr/ADR-001` — дополнение 2026-09-13: границы импортов 1a–1f, регистрация контекстов;
  - `plan/ownership.md` v0.6 — файлы владельцев в `cmd/*` и `shared/contracts`, разделение `cmd/mvctl/**`, перенос `RecordingWriter` и `FakeGateway`, `test/e2e/**`, `testdata/fixtures/events/**`, §3 «Период трёх веток эпиков», запрос № 25;
  - документы владельцев с пометкой «(изм. T-444)», только приведение текста к решению:
    - `components/swarm-llm-laws.md` §2, §3, §9.1, §9.2, §9.4, §10.4, §12.2, §14, §20 п. 10, дополнение сведения 3;
    - `components/gateway-and-bot.md` §1, §3, §5.1, §15;
    - `components/state-and-mechanics.md` §4.6, §4.10 (источник bootstrap);
    - `threat-model.md` T-22 и SEC-21 (облако);
    - `analysis/api-contracts.md` v0.2.2 — §2.3.2, §2.3.3, §2.3.7, §2.3.10, §2.3.16.
  - строки DoD тимлидам — списком в отчёте и в карточке, `tasks.md` эпиков не правится (их правят tech-lead#1/#2/#3 в ветках эпиков).
- **DoD:**
  1. `grep -rna allow_external_players Docs/dev-team/architecture Docs/dev-team/analysis schemas/` — только записи истории.
  2. По каждому пункту решения 1–9 и дополнения 10–11 — строка в сводке v0.11 `contracts.md` с колонками «Кто прав», «Совместимость», «Источник».
  3. У каждого затронутого контракта — версия в заголовке и запись в истории; заголовок C-08 совпадает с историей (v1.1…v1.4).
  4. Правки в документах владельцев помечены «(изм. T-444)»; правки, которые меняли бы решение владельца, не внесены, а перечислены в отчёте.
  5. Переводы строк сохранены (CRLF, число CR равно числу строк в каждом изменённом файле).
  6. `gitleaks dir` по изменённым файлам — 0 утечек.
- **Ссылки:** `contracts.md` §16, ADR-001, ADR-005 п. 6, ADR-007 п. 1, ADR-017 доп. 1, ADR-023, ADR-025; ревью T-009 (Mi-2, Mi-5, Mi-7), T-215 (открытые вопросы), T-443 (бэклог п. 1); `EPIC-003-swarm-llm-laws/dev-log.md` (T-214, T-215); `.golangci.yml`; `shared/contracts/registry.go:53-58, :189`; `shared/runtime/http.go:15, :78`; `cmd/multiverse/serve.go:339`; `cmd/multiverse/contexts.go`; `cmd/mvctl/main.go`.
- **Исполнитель:** system-architect#1 (Opus). Ветка `task/T-444-contracts-revision-4` от эпика после слияния в develop, папка `.worktrees/T-444`. Карточка — `tasks/T-444.md`. Параллельно — T-445 (developer#3): `.golangci.yml`, `registry.go`, схемы, `_common.json`, фикстуры, `blockv_test.go`.
- **Итерация 2 (по ревью #1: 0/3/5/2, решения оркестратора 2026-09-13).** Ma-1 — «Код расходится» в C-02 v1.5, двойник `FakeState` правит EPIC-002 в T-056. Ma-2 — мягкий режим с названными исключениями: §0, C-01 v1.4, C-14, §16 п. 8, `ownership.md` строки 9/10/12 и §3 п. 4. Ma-3 — сценарии `test/e2e` привязаны к задачам, правило имён с префиксом владельца (`ownership.md` §1, §2 № 26). Mi-1 — версии C-05 v1.7, C-11 v1.2, C-12 v1.1, C-15 v1.2. Mi-2 — §17. Mi-3 — КД роя. Mi-4 — `response_hash` при `error`, `api-contracts.md` §2.3.10. Mi-5 — строки DoD T-352, T-353. N-1, N-2. Бэклог: девять легаси-типов; NUL в КД роя убран, строка таблицы провайдеров получила inline `gitleaks:allow`. Задача раскладки — T-446. Подробности и строки DoD — карточка, «Итерация 2».
- **Итерация 3 (по ревью #2: «принять», 0/0/3/3).** Mi-1 — одна формулировка «кто принимает»: tech-lead эпика принимает, tech-lead#1 обязательно просматривает общие файлы, system-architect ревьюит строки реестра. Mi-2 — T-251 в привязке `group-3x30`. Mi-3 — `TestMain` только у EPIC-001, эпики регистрируют подготовку. N-1…N-3. Подробности — карточка, «Итерация 3».
- **Бэклог EPIC-001 (из ревью #2 T-444):** завести `.git-blame-ignore-revs` и внести в него коммит T-444, который нормализует КД роя в LF; иначе `git blame` по всем 1024 строкам укажет на T-444. Номер коммита известен только после коммита. Владелец — гигиена репозитория EPIC-001.
- **Бэклог EPIC-001 (итерация 3 T-444, `ownership.md` §1, строка `test/e2e/**`):** завести `test/e2e/main_test.go` (`package e2e_test`).
  - Состав: единственный `TestMain` пакета и регистратор `registerPackageSetup(owner string, setup func() (teardown func(), err error))`, который эпики зовут из `init()` своих файлов.
  - Порядок: `TestMain` вызывает подготовки в порядке имени владельца, завершения — в обратном.
  - DoD:
    - тест регистратора на двух фиктивных владельцах: порядок подготовок, обратный порядок завершений, при ошибке второй подготовки завершается первая и пакет падает;
    - повторная регистрация одного владельца — ошибка;
    - `go test ./test/e2e/...` зелёный, `go vet` и `golangci-lint` чисты.
  - Когда: по первому запросу эпика; до этого эпикам хватает ленивых помощников с префиксом на `sync.Once`.
  - Размер S, владелец EPIC-001 (tech-lead#1), ревью system-architect не требуется: файл EPIC-001 без контракта.
- **(приёмка tech-lead#2, 2026-09-13)** Принята после ревью #2 («принять», 0/0/3/3), итерацию 3 проверил при приёмке.
  - DoD 1–6 выполнены:
    - `grep -rna allow_external_players` находит одну запись истории;
    - сводка v0.11 по пунктам 1a–11 с «Кто прав»;
    - версии и история у C-01, C-02, C-04…C-08, C-11, C-12, C-15;
    - пометки «(изм. T-444)» стоят;
    - CRLF у 13 файлов;
    - `gitleaks dir` — 0 с `.gitleaksignore` и без него, контроль из HEAD — 1.
  - Также на месте дополнение ADR-001 и `ownership.md` v0.6.
  - «Кто принимает» определено в `contracts.md` §16 п. 8, остальные места ссылаются на него без противоречий.
  - Правило `TestMain` выполнимо: ни в одной ветке и рабочей папке в `test/e2e` его нет.
  - Документы согласованы с T-445 (схемы `llm.output*`, `EventRef`, `gm.created`), T-301 (`group_move`) и T-050/T-052 (`shared/entity`, `state_hash`).
  - При приёмке склеена распавшаяся строка DoD T-218 в карточке.
  - Не блокируют и переданы в карточку:
    - `group_move` в КД шлюза §8.1;
    - абзац C-02 «Код расходится», который устареет после слияния EPIC-002;
    - C-14 без записи истории;
    - запятая в `ownership.md:26`.
  - Правка плана EPIC-003 (T-204, T-205, T-221, шапка `:8`) передана tech-lead#2 TEAM-2 — карточка, «Приёмка».
  - Скан истории — после коммита.

### T-445: Ревизия контрактов 4 — линтер, реестр, схемы · Размер: M · Статус: done · Волна 1 · contract-change
- **Причина**: решения system-architect#1 от 2026-09-13 по сверкам планов EPIC-003/004 и ревью T-214/T-215 (`journal.md`, 2026-09-13; ревизия контрактов 4). Границы depguard 1c–1e расходились с планами роя, памяти и шлюза. Хеши записей модели и блупринта были строкой любой формы. Условная обязательность полей `llm.output` и `llm.output.rejected` жила только в тексте (Ma-1 ревью T-215, Mi-2 ревью T-009). У `gm.created` не было издателя-шлюза, который нужен плану EPIC-004 (T-305). Тексты контрактов (contracts.md v0.11, C-04 v1.4, C-07 v1.3, дополнение ADR-001) правит T-444, здесь их не трогали.
- **Состав**:
  - `.golangci.yml`: 1c — `internal-swarm` плюс `llm/prompt$`, `llm/guardian$`, тесты роя отдельным правилом `internal-swarm-tests` с `llm/providers/fake$`; `internal-memory` сужено до `providers$`, `fake$` — в `internal-memory-tests`. 1d — `shared-testkit-swarm` плюс `internal/swarm/template$`, листовое `internal-swarm-template`. 1e — `shared-testkit-gateway` (`gateway/client$`, `gateway/api$`), исключение в `shared`, листовое `internal-gateway-client`, запрет `internal/gateway/gatewaytest` в `no-testkit-in-production` и расширение исключения `_test.go`.
  - Схемы: `sha256:<64 hex>` у `prompt_hash`, `response_hash`, `content_hash`. `allOf` из `if/then` в `llm.output` по таблице C-07 v1.3 и в `llm.output.rejected` (`budget_exceeded`/прочие, `unknown_entity`). Новое поле `background_ref`. `$defs.EventRef` в `_common.json`.
  - Реестр: `gm.created` — издатели `legacy` и `core/gateway` (`legacyEvent` с дополнительными издателями).
  - Тесты и фикстуры: `blockv_test.go`, новый `llmrecord_test.go`, `registry_test.go` (`TestLegacyPublishers`), фикстуры `agent.*` и `llm.output*`, README фикстур.
- **DoD**:
  1. Каждое новое или изменённое правило depguard краснеет своим мутантом (8 мутантов решения и дополнительные), законные импорты зелёные. Мутанты — в копии дерева в scratch, контрольный первым, без `-overlay`.
  2. Хеш без префикса, в верхнем регистре, длиной 63 отвергается.
  3. По каждой ветке `if/then` есть валидный и невалидный случай; мутант удаления ветки погибает.
  4. `go build ./... && go vet ./...`, `go run ./cmd/mvctl contracts check`, `go test -short -count=1 ./...`, `golangci-lint run ./...` (и `--build-tags integration`), `make contracts` — зелёные; `gitleaks dir` по изменённым файлам чист.
- **Ссылки**: карточка `tasks/T-445.md`; решения — `journal.md` 2026-09-13 (system-architect#1, ревизия контрактов 4); ревью T-215 (Ma-1) — `journal.md` 2026-09-13; образец мутантов depguard — T-255 (`EPIC-003-swarm-llm-laws/dev-log.md`, §6, L1–L6); тексты — T-444.
- **Исполнитель**: developer#3 (TEAM-1, Opus). Ветка `task/T-445-depguard-registry-schemas`. Закрывает Ma-1 ревью T-215.
- **(итерация 2, 2026-09-13)** Ревью #1 — вернуть (0/1/2/1). По открытому вопросу system-architect выбрал (б). Исправлено:
  - Ma-1 — исключение `_test.go` привязано к правилу `no-testkit-in-production` и к пакету `gatewaytest` целиком;
  - Mi-1 — запрет `gatewaytest` добавлен в `cmd-multiverse-fake-contexts`;
  - Mi-2 — собственный пакет в правилах `internal-*` записан как `<ctx>$` + `<ctx>/`;
  - N-1 — комментарии `blockv_test.go`.

  Подробно — карточка, «Итерация 2».
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/1/2/1) и итерации 2, которую проверил при приёмке: мутанты M0, A1 (L16), A3 (`gatewaytestx`), B1 (`swarmx`), C1 (`fake_contexts.go`) красные своими правилами, законные G1 (`swarm/template`) и A5 зелёные, текст исключения сверен с находкой golangci-lint 2.13.2 (A9, A5'). Правки T-445 поверх деревьев EPIC-002/003/004 — сборка, тесты и depguard зелёные. Слияние: с T-201 (`.golangci.yml`) текстового конфликта нет; с T-052 (README фикстур) — один конфликт, рецепт в карточке. При приёмке исправлено имя файла в README фикстур (`blockv_test.go` → `llmrecord_test.go`). Подробно — карточка, «Приёмка (tech-lead)».
- **Бэклог (из ревью #1 T-445, 2026-09-13)** — отдельные задачи, не блокируют:
  - system-architect#1, T-444, до слияния. Дополнение ADR-001 от 2026-09-13, п. 3, называет шаблон `**/internal/swarm/**/*_test.go`, а он в depguard не покрывает тесты в корне пакета. В `.golangci.yml` записан `**/internal/swarm/**_test.go`. Текст привести к коду — передано в T-444.
  - Свести случаи `llm.output` из `TestBlockVPayloadRejects` (`blockv_test.go`) и таблиц `llmrecord_test.go` в одну таблицу при ближайшей правке `shared/contracts`.
  - При следующем исключении depguard по образцу 1c–1e включать в стандартный набор мутант «тест другого контекста импортирует новый пакет». Дыру Ma-1 поймал именно такой мутант (L16), а в восьми мутантах решения его не было.
  - **Для EPIC-004 (T-301, T-308) — знать заранее.** После варианта (б) правила контекстов действуют в `_test.go` и для `gatewaytest`. Поэтому тесты `internal/gateway/client` (правило `internal-gateway-client`) и тесты любого другого контекста не могут импортировать `FakeGateway`: мутанты A1 и A10 итерации 2 красные. Тест клиента против `FakeGateway` пишется в `internal/gateway/gatewaytest` или в `test/` (e2e), как и сказано в дополнении ADR-001 п. 5.

### T-453: gitleaks — эталон `.env.example` без построчного отпечатка; CI на `develop` · Размер: XS · Статус: done · Волна 1
- **Причина (оркестратор, 2026-09-13)**:
  - Правило gitleaks `generic-api-key` склеивало пустой `MV_LLM_API_KEY` со следующей строкой `MV_LLM_NUM_CTX` в эталоне `.env.example` (`infrastructure.md` §4.2) в одно совпадение. Находку гасил построчный отпечаток индекса в `.gitleaksignore`. Правка документа выше §4.2 сдвигает строку, и отпечаток уже дважды устаревал (520 → 549). Сам `.env.example` исключён путём в `.gitleaks.toml`.
  - `.github/workflows/go.yml` не запускался на `develop`: `push` — `main`, `integration/**`; `pull_request` — они же и `epic/**`. С перехода на gitflow (2026-09-11) эпики сливаются в `develop`, и защита ветки навсегда блокировала бы PR, проверки которого не приходят.
  - `make secrets-scan` по умолчанию брал диапазон от `integration/mvp-1`. Ветка стоит на F-0 (`744fb10`), диапазон рос с каждой задачей и включал коммит T-399 с той же находкой.
- **Состав**:
  1. `.env.example` и эталон §4.2, одинаково: комментарий `MV_LLM_NUM_CTX` перенесён на строку над переменной. Над `MV_MINIO_USE_SSL` и `MV_EMBED_MODEL` поставлен комментарий: они стоят сразу за пустыми `MV_MINIO_SECRET_KEY` и `MV_NEO4J_PASSWORD` и находку не давали только из-за стоп-слов правила (мутант с переименованными строками даёт 2 находки).
  2. `infrastructure.md`: §4.1 п. 5 — правило «за пустой переменной с идентификатором `generic-api-key` в имени следующая непустая строка — комментарий» (полный список идентификаторов v8.30.1: `ACCESS`, `API`, `AUTH`, `CREDENTIAL`, `CREDS`, `KEY`, `PASSWD`/`PASSWORD`, `SECRET`, `TOKEN` — дополнено при приёмке, Mi-2) и почему пустая строка не спасает; §4.2 — строка проверки; §3.1 — триггеры по факту `go.yml`; §2.2 — строка `make secrets-scan`.
  3. `.gitleaksignore`: снят построчный отпечаток индекса (`…:549`) вместе с комментарием. Отпечаток коммита T-399 (`f8759b0…:520`) нужен скану истории и не сдвигается. В эпик его добавил оркестратор (`1a45b03`), а ветка задачи создана раньше. В ветке задачи эта строка стоит с самостоятельным комментарием. **Синхронизация с эпиком (дополнено при приёмке, N-1):** конфликт в `.gitleaksignore` ожидается ровно в блоке T-399 и разрешается фрагментом, а не целым файлом (`checkout --ours`/`--theirs` запрещён). Из ветки задачи берётся только конфликтный фрагмент блока T-399: комментарий T-453 без построчного `…infrastructure.md:generic-api-key:549` и без второго комментария эпика. Строка `f8759b034eca21dbc4ebcabb0ee7d561fab00074:Docs/dev-team/architecture/infrastructure.md:generic-api-key:520` у обеих сторон одинакова и может оказаться за маркерами конфликта (так в моделировании `git merge-file` против `8613294`) — её не дублировать и не терять. Всё выше блока остаётся из эпика: правка T-444 (`Docs/dev-team/architecture/components/swarm-llm-laws.md:generic-api-key:521` и комментарий к ней про 514 → 521 и inline `gitleaks:allow`) сливается сама. После разрешения в файле ровно одна строка `f8759b0…:520`, строка `swarm-llm-laws.md:generic-api-key:521` есть, строки `infrastructure.md:generic-api-key:549` нет.
  4. `go.yml`: `develop` добавлен в `push` и `pull_request`, обновлены шапка и комментарий триггера. Задание `image` (только `push` в `main`/`integration/mvp-1`) не менялось.
  5. `Makefile`: `BASE ?= develop`. Для CI замена безопасна: CI цель не вызывает, у задания `security` свой скан. Цель отказывает, если `BASE` не разрешается в коммит. Причина: на таком диапазоне gitleaks пишет «0 commits scanned» и выходит с 0, так что клон без ветки `BASE` проходил молча, и с прежним умолчанием тоже. На самой `develop` — `BASE=main`. Пустой диапазон `BASE..HEAD` — предупреждение в stderr с подсказкой `BASE=main`, не отказ (дополнено при приёмке, N-2).
- **DoD**:
  - `gitleaks dir` без построчного отпечатка — 0 находок в трёх копиях: изменённые файлы с сохранёнными путями; та же копия без `.gitleaksignore` (`.env.example` — под другим именем, мимо исключения путём); все отслеживаемые файлы с содержимым рабочей копии. Контрольный мутант (комментарий `MV_LLM_NUM_CTX` возвращён в строку) — 1 находка;
  - за каждой пустой переменной с ключевым словом в имени следующая непустая строка начинается с `#` — в файле и в эталоне;
  - эталон и `.env.example`: те же 60 переменных в том же порядке, те же значения, те же 7 пометок `[required]` (блок комментария разбирается так же, как в правиле 7);
  - `scripts/compose-lint.sh` и `--fixtures` — зелёные; `go run ./cmd/mvctl env check` — 67 переменных, зелёный; `go.yml` — валидный YAML;
  - `make ci BASE=develop` — rc=0: lint 0 issues; test ok; test-race SKIPPED (нет cgo); contracts 65 типов; env 67 переменных; secrets-scan no leaks; privacy-scan ok; govulncheck 0 затрагивающих; compose-lint ok; scripts-parity 97 PASS / 0 FAIL / 2 KNOWN-FAILING; test-e2e ok;
  - скан истории после коммита — оркестратор.
- **Метка**: нет. Контракты и Go-код не меняются.
- **Исполнитель**: devops-engineer#1. Ветка `task/T-453-gitleaks-env-example-ci-develop` (от эпика `ab6cb1d`). Карточка — `tasks/T-453.md`.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/0/2/2). Mi-1 (шапка `go.yml` не называет `develop` защищённой), Mi-2 (полный список идентификаторов `generic-api-key` v8.30.1 в §4.1 п. 5 и §4.2 — по исходнику правила), N-1 (рецепт `.gitleaksignore` фрагментом; проверен моделированием слияния с эпиком `8613294`) и N-2 (предупреждение о пустом `BASE..HEAD`) внесены при приёмке без повторного ревью. Прогоны после правок: `compose-lint` и `--fixtures` ok, `mvctl env check` ok (67), YAML и `actionlint` ok, `make secrets-scan BASE=develop` rc=0 с предупреждением, `BASE=no-such-branch` — отказ, `gitleaks dir` по копиям — 0, контрольный мутант — 2 находки. Подробности — карточка, «Приёмка».
- **Бэклог (из T-453, 2026-09-13)** — отдельные задачи, не блокируют:
  - Владелец (решение пользователя): защита `develop` в настройках репозитория, required checks — как у `integration/mvp-1`. Шапка `go.yml` после приёмки защищённой `develop` не называет; когда защита включена, дописать `develop` и туда. После этого devops-engineer дописывает `develop` в §3.2 и в шапку `.github/CODEOWNERS`: сейчас там правила названы только для `main` и `integration/mvp-1`.
  - Судьба `integration/mvp-1` после gitflow (решение пользователя, вопрос оркестратором уже задан): ветка стоит на F-0. Если её выводят, убрать упоминания из `go.yml` (триггеры, `if` задания `image`), `CODEOWNERS`, §3.1–§3.2 и `README.md:208` («ветка от `integration/mvp-1`»). Там же решить, собирать ли образ на `push` в `develop`: около 3 мин бюджета Actions на каждый.
  - compose-lint, правило 7 (разбор `.env.example`): проверять правило §4.1 п. 5 машинно — за пустой переменной с идентификатором `generic-api-key` в имени (полный список v8.30.1 — §4.1 п. 5: `ACCESS`, `API`, `AUTH`, `CREDENTIAL`, `CREDS`, `KEY`, `PASSWD`/`PASSWORD`, `SECRET`, `TOKEN`; `API` с учётом регистра) следующая непустая строка — комментарий; фикстуры bad/good на каждое слово не нужны, но на `ACCESS` и `API` без `KEY` — нужны. Список сверять при смене `GITLEAKS_VERSION`. Сейчас нарушение в эталоне ловит `gitleaks dir` по `Docs/`, а в самом `.env.example`, исключённом путём, его не ловит никто. devops-engineer.
  - `infrastructure.md:734`, §4.5 п. 12 (критерий F-1): проверка приводит `gitleaks git --redact --log-opts="integration/mvp-1..HEAD" .`. Это историческая запись F-1, но после решения о судьбе `integration/mvp-1` строку привести к стволу gitflow (`develop..HEAD`, на `develop` — `main..HEAD`) или пометить «на момент F-1». Вместе с пунктом о судьбе ветки, devops-engineer.
  - `infrastructure.md` §3.1, строка задания `security`: в тексте `security-events: write` (SARIF), а в `go.yml` — `pull-requests: read` без SARIF. Редакционно, devops-engineer.

### T-449: Документы по решениям system-architect (ревизия 4, продолжение) · Размер: M · Статус: done · Волна 1 · contract-change
- **Причина.** Пакет решений system-architect#1 оркестратор принял 2026-09-13. Он отвечает на вопросы четырёх задач:
  - T-050 (EPIC-002, приёмка с условием подтверждения);
  - T-053 (EPIC-002, вопросы Р1–Р5);
  - T-206 (EPIC-003, Mi-4 и Mi-6 ревью, бэклог бюджета);
  - T-448 (EPIC-002).

  Решения нужно внести в контракты и документы до того, как на них начнут опираться задачи эпиков. Туда же — бэклог приёмки T-444: замечания 1, 3, 4 и шаблон файлов тестов роя в ADR-001.
- **Что сделать** (только документы; код — T-053, T-448, T-450, T-451, T-209, T-250):
  - `architecture/contracts.md` v0.12 — сводка; C-02 (абзац «Код расходится» закрыт T-050, каноническая форма значений, заготовка v1.6 «ожидает T-448»); C-03 v1.3; C-07 v1.4 (`limit_money`); C-14 (запись истории v1.2a); C-15 v1.3 (правило «локальный адрес»);
  - `architecture/components/state-and-mechanics.md` §3.2 (равенство элементов), §3.3 (каноническая форма), §5.4 (позиция побега);
  - `analysis/data-model.md` §3.3 (`flee`);
  - ADR-005 доп. 2 п. 3 (правило «локальный адрес»); ADR-001 доп. 2026-09-13 п. 1 (направление зависимостей `internal/llm`), п. 3 (шаблон `**/internal/swarm/**_test.go`); ADR-012 п. 4 (пометка «дополнено C-03 v1.3»);
  - `architecture/components/gateway-and-bot.md` §8.1 (`group_move`); `plan/ownership.md` строка `test/e2e/**` (запятая).
  - Правки помечены «(изм. T-449)». `design.md` EPIC-002 (I1-4) не правится — это строка тимлиду. Строки DoD — в карточке.
- **DoD:**
  1. У каждого затронутого контракта — версия в заголовке и запись в истории (C-03 v1.3, C-07 v1.4, C-14 v1.2a, C-15 v1.3); сводка v0.12 с колонкой «Кто прав».
  2. Если в карточке T-448 нет готового текста, C-02 v1.6 остаётся заготовкой «ожидает T-448».
  3. Текст C-03 v1.3 совпадает с кодом T-053 в его рабочей папке: `Actor.Version`, `Actor.Kind`, `Action.At`, отказ при `Version <= 0` и при пустом `At`.
  4. Перечень случаев правила «локальный адрес» в C-15 и в ADR-005 совпадает дословно.
  5. CRLF сохранён, байтов NUL нет, строка таблицы провайдеров КД роя не сдвинута; `gitleaks dir --redact -c .gitleaks.toml` по изменённым файлам в копии — 0 находок.
- **Ссылки:** пакет решений system-architect (отчёт 2026-09-13); `epics/EPIC-002-state-mechanics/tasks/T-050.md`, `T-053.md`; `epics/EPIC-003-swarm-llm-laws/review.md` (T-206, Mi-4, Mi-6); приёмка T-444 (`tasks/T-444.md`, «Замечания»); ADR-003 п. 6, ADR-013 п. 1, ADR-005 доп. 2 п. 3.
- **Исполнитель:** system-architect#1 (Opus). Ветка `task/T-449-docs-architect-decisions` от эпика после слияния T-444 и T-445, папка `.worktrees/T-449`. Карточка — `tasks/T-449.md`.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/3/6/5, вернуть), итерации 2 и ревью #2 (code-reviewer#3, 0/0/2/6, принять). Итераций ревью — 2. DoD 1–5 сверен при приёмке:
  - версии и записи истории у C-03 v1.3, C-07 v1.4, C-14 v1.2a, C-15 v1.3, у C-02 — v1.5a и v1.6; сводка v0.12 с колонкой «Кто прав»;
  - C-02 v1.6 внесён текстом из карточки T-448 с пометкой «вступает в силу со слиянием T-448» (п. 2 DoD снят поручением оркестратора на итерацию 2);
  - C-03 v1.3 совпадает с кодом T-053 в `epic/EPIC-002-state-mechanics` (`24f1baf`): `types.go:53`, `:60`, `:140`; `changes.go:209` (`Version <= 0`), `:225` (пустой `At`); `actor.go:30`; `rules.go:717`;
  - пять пунктов правила «локальный адрес» в C-15 и ADR-005 совпадают побайтно;
  - CRLF у 12 файлов, NUL нет; `swarm-llm-laws.md` не изменён (строка 521 с `gitleaks:allow`); `make secrets-scan BASE=epic/EPIC-001-foundation` — rc=0; `gitleaks dir` по копии 12 файлов с конфигурацией ветки и кончика эпика (`dcdb530`), с `.gitleaksignore` и без — 0.

  R2-Mi-1, R2-Mi-2 и шесть Nit ревью #2 при приёмке не правились: `contracts.md` правит только system-architect. Решением оркестратора они переданы в T-456. Слияние с эпиком: пересечение только в `tasks.md`, `review.md`, `dev-log.md`, везде дописывание в конец. У `tasks.md` нет драйвера `appendtail`, поэтому ждать конфликта хвоста; разрешение — объединение: раздел T-453 эпика, затем T-449 и T-456. Подробно — карточка, «Приёмка (tech-lead)».
- **Бэклог (из ревью T-449, 2026-09-13)** — отдельные задачи, не блокируют:
  - КД роя, `swarm-llm-laws.md:533`: строка `budget{kind,limit,window}` после C-07 v1.4 неполна для `kind=cloud`. Править вместе с T-250 или со следующей нормализацией КД роя, с перепроверкой отпечатка `.gitleaksignore` (строка 521). system-architect.
  - `infrastructure.md:360`, `:622`: прежнее определение «локального» (loopback, `host.docker.internal`, RFC 1918). Привести ссылкой на C-15 v1.3. devops-engineer (T-450 или отдельно).
  - C-03, «Состояние реализации на 2026-09-11» (`ErrNotImplemented`): датированная строка при слиянии EPIC-002 в develop. Там же перечислить в C-03 полный набор путей `ChangesFor`, включая `loot_claimed_by` (`changes.go:156`). system-architect.

### T-456: C-08 v1.4 и нормализация КД шлюза · system-architect · S–M · Статус: done · после T-449
- **Причина (оркестратор, 2026-09-13).**
  - Решение system-architect#1 по T-303: новый код `503 forget_incomplete` принят с условиями, а в контрактах и КД шлюза его нет. КД шлюза расходится с кодом T-303.
  - Вопрос дедлайна чтения long-poll из ревью T-446.
  - Minor и Nit ревью #2 T-449, которые при приёмке не правятся: `contracts.md` правит только system-architect.
- **Версия.** Номер C-08 v1.4 уже занят T-444 («двойники шлюза и таймауты маршрутов»). Номер новой записи (ожидаемо v1.5) выбирает system-architect; название задачи оставлено как в журнале.
- **Номера и строки после T-457 (приёмка T-457, tech-lead#1, 2026-09-13).** T-457 заняла `contracts.md` v0.13, C-01 v1.9, C-02 v1.5b, C-05 v1.8, C-07 v1.5, C-15 v1.4 и `ownership.md` v0.7; C-08 не трогала. T-456 берёт `contracts.md` v0.14, у C-15 — v1.5. Строки C-02 из п. 6 (`:322`, `:323`, `:356` на момент ревью T-449) после T-457 — `:379`, `:380`, `:416`. Попутно: лишняя закрывающая скобка в §16 п. 8, строка исключения T-458 (`contracts.md:903`, «… T-457, C-01 v1.9):»).
- **Состав** (только документы; код — T-303, T-311, T-314, T-355):
  1. C-08, `503 forget_incomplete` с условиями:
     - любой `/forget` отвечает 503, пока сжатие отложено;
     - заголовок `Retry-After`;
     - безусловное `CompactLinks` в `Start`;
     - отличие от `503 bus_unavailable`;
     - бот не говорит «удалено» до ответа 200 (T-311).

     Заголовок и запись истории.
  2. `api-contracts.md` §1.6 — строка кода `forget_incomplete`.
  3. КД шлюза, всё по коду T-303:
     - §3 — пакет `internal/gateway/handlers`;
     - §5.1 — порядок `request_id` и `recover`, `nolog`;
     - §6 — сигнатуры `links.Store`;
     - §7.5 и §9.

     ADR-019 — дополнение.
  4. Дедлайн чтения для long-poll: не меньше `wait_ms + 5 с` либо 0 (вопрос ревью T-446).
  5. Строки DoD для T-303, T-311, T-314, T-355 — в карточке, тимлиду EPIC-004.
  6. `contracts.md` C-02, замечания ревью #2 T-449:
     - R2-Mi-1 — пометка «вступает в силу со слиянием T-448» у норм v1.6 в «Вход State», «Выход State» и «Гарантиях» (`:322`, `:323`, `:356` на момент ревью);
     - R2-Mi-2 — порядок «элемент предка, которого нет среди затронутых путей, идёт после их элементов» (уже в тексте v1.6 карточки T-448, итерация 2).

     Сверить C-02 v1.6 с итоговой карточкой T-448, пометки «вступает в силу» снять при её слиянии.
  7. Nit ревью #2 T-449:
     - R2-N-1 — каноническая форма дважды названа в строке версии v0.12;
     - R2-N-2 — числовые записи хоста и `127.0.0.1.` не ужесточение относительно T-206;
     - R2-N-3 — КД State §3.3: числа в `attributes` отвергает проверка создания;
     - R2-N-4 — КД State §5.1: комментарий `ActorFromEntity` v1.3;
     - R2-N-5 — класс `invalid` в C-15 и ADR-005, одинаково: IPv6 без скобок, символы после `]`;
     - R2-N-6 — строка DoD T-450 в карточке T-449 (п. 8).
  8. Строка DoD T-450: IPv4-адрес с точкой на конце (`127.0.0.1.`, `10.0.0.1.`) — `cloud`. Нужны строки таблицы; точка у адресов не снимается в `llm-endpoint.sh` и `LlmEndpoint.psm1`; шапка таблицы исправлена.
  9. C-15 и ADR-005, класс `invalid` (приёмка T-450, tech-lead#1, 2026-09-13; бэклог, строка 2): символы хоста, которые отвергает `url.Parse` (пробел, `\`, `^`, `` ` ``, `{`, `|`, `}`), — в список `invalid`, одинаково с R2-N-5 п. 7. Правило «ни одна реализация не печатает userinfo в текстах отказов» — в C-15 (скрипты — T-450, Go — T-451).
  10. **На решение system-architect** (приёмка T-210, tech-lead#2, 2026-09-13; ревью T-210 Mi-1): вводить ли опциональное `llm.output.called_at` — время попытки по `Clock` шлюза, чтобы окно бюджета `1m` считалось от попытки, а не от времени причины. Решение — в C-07 (следующая версия v1.6: v1.5 занята T-457) и строкой DoD T-250 (EPIC-003): с полем окно считает от него, без поля — от причины, погрешность в `.env.example`. Сверить с C-01 v1.9 «время в обработчике replay»: `Clock.Now()` в replay в байты событий не входит, поэтому в replay и догоне поле берётся из записи. Если поле не вводится — отказ с доводом в карточке T-456.
- **DoD:**
  1. У изменённых контрактов — версия в заголовке и запись в истории; строка сводки в `contracts.md`.
  2. Текст C-08 о `forget_incomplete` совпадает с кодом T-303 (коды, `Retry-After`, `CompactLinks` в `Start`). КД шлюза §3, §5.1, §6 совпадают с кодом T-303 в его ветке.
  3. C-02 v1.6 совпадает с итоговой карточкой T-448. R2-Mi-1, R2-Mi-2, R2-N-1…R2-N-6 закрыты или отклонены с обоснованием.
  4. Правки помечены «(изм. T-456)». CRLF сохранён, байтов NUL нет, строка 521 КД роя не сдвинута; `gitleaks dir` по изменённым файлам — 0.
- **Ссылки:** `journal.md` 2026-09-13 (решения system-architect#1, заведение T-456, ревью #2 T-449); `review.md`, «T-449 · ревью #2»; ревью T-303 и T-446; карточка T-448, «Текст C-02 v1.6 для T-449».
- **Исполнитель:** system-architect#1, метка `contract-change`. Ветка `task/T-456-c08-forget-incomplete-gateway-kd` от эпика после слияния T-449. Параллельно с T-449 не вести: тот же `contracts.md`.
- **Выполнение (system-architect#1, 2026-09-13; на ревью).** Ветка `task/T-456-c08-forget-incomplete` (`14b29aa`). `contracts.md` v0.14: C-08 v1.5 (`503 forget_incomplete`, порядок `/forget`, клиент сам не повторяет), C-02 v1.6 (текст по итоговой карточке T-448, пометки «в develop — с контрольного слияния EPIC-002») и v1.7 (мир в конверте предложения), C-01 v1.10 (`eventbus.Permanent`, `Policy.World`), C-07 v1.6 (`llm.output.called_at` — да), C-15 v1.5; КД шлюза §3, §4.1, §5.1 (п. 8 — дедлайны), §5.2, §5.6, §6, §7.5, §10.5, §11.1, §11.4; ADR-019 доп. 2; ADR-005; `api-contracts.md` §1.2, §1.6; КД State §3.3, §5.1, §9; `data-model.md` §7.2. R2-Mi-1, R2-Mi-2, R2-N-1…R2-N-6 закрыты (R2-N-6 и п. 8 — фактом приёмки T-450). Решения, отвергнутые варианты и строки DoD для EPIC-001…EPIC-004 (новая задача EPIC-001 `contract-change` — T-460) — карточка [`tasks/T-456.md`](tasks/T-456.md).
- **Итерация 2 (system-architect#1, 2026-09-13; по ревью #1, 0/2/3/6, и дополнениям оркестратора; на ревью).** Ma-1: C-01 v1.8 «Уровень маршрута» — по КД шлюза §5.1 п. 8. Ma-2: `Permanent` — только дефект события; под отменой не паркуется; остановленный мир State (`publish_failed` и паника) — обычная ошибка; строки T-460, T-056, КД State §9. Mi-1…Mi-3, N-1…N-6, бэклог ревью п. 1 (T-311, `Resolve`). Ревью #2 T-459: номера задач `labels_hash` в C-07, `ErrLabelTable` подтверждён, нулевое `called_at` — ошибка `Recorder`, у читателя `ErrBudgetKey`. T-451: C-15 v1.5 — не-ASCII и `%` в authority — `invalid`, две точки на конце — `cloud`, при `@` хост в отказе не называется; 15 строк таблицы проверены скриптами и переданы строкой DoD T-451 (таблица не правилась). Подробно — карточка, «Итерация 2».
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #2 (0/0/2/5), итераций ревью — 2. По решениям оркестратора при приёмке закрыты: Mi-1 (C-15 и ADR-005 — пометка «скрипты T-450 пока расходятся», маскировка — задача devops EPIC-001 в бэклоге; тексты побайтно одинаковы), Mi-2 (КД шлюза §10.2 п. 2 — в групповом чате бот не отвечает никому, решение по T-310), N-1 (runbook — T-317), N-2, N-3 (паника — по `delivery.go`), N-5 (довод КД State §9, шапка C-01 v1.10, номер T-460). N-4 — замены для `tasks.md` EPIC-003 в карточке, передать tech-lead#2; вопрос про `@` в пути (C-15) — к system-architect. DoD 1–4 подтверждены: код, схемы, `swarm-llm-laws.md`, `.env.example`, `infrastructure.md` не менялись; CRLF, NUL и BOM нет; `gitleaks dir` по копии — 0. Кончик `epic/EPIC-001-foundation` равен базе `14b29aa`, конфликтов слияния нет; препятствий для контрольного слияния EPIC-001 в develop нет. Подробности — карточка, «Приёмка (tech-lead)».

### T-446: Ревизия контрактов 4 — runtime и раскладка cmd · Размер: M · Статус: done · Волна 1 · contract-change
- **Причина.** Решения system-architect#1 от 2026-09-13 (ревизия контрактов 4, пункты 6 и 7; тексты — T-444, `contracts.md` v0.11: C-01 v1.8, §16 п. 8, ADR-001 доп. п. 7).
  - (6) HTTP-сервер процесса один на все контексты, и контракт о его таймаутах молчал. КД шлюза §5.1 требовал таймаутов маршрутов, а процесс останавливает HTTP раньше контекстов (`serve.go:339`). `Shutdown` не отменяет контексты запросов, поэтому long-poll до 25 с переживает `ShutdownTimeout` 5 с (`http.go:15`), и `srv.Stop` падает по дедлайну.
  - (7) Регистрация контекстов (`contexts.go`), подкоманды `mvctl` (`main.go`) и строки реестра (`registry.go`) лежали в общих файлах EPIC-001. Их правили бы три ветки эпиков одновременно.
- **Состав.**
  - `shared/runtime/http.go`: `ReadHeaderTimeout 5s` (было 10 с), `IdleTimeout 120s`; `ReadTimeout`/`WriteTimeout` сервера не ставятся. `runtime.SetDeadlines(w, read, write) error` — через `http.ResponseController`, время `clock.Real` в любом режиме. `runtime.ShuttingDown(ctx) <-chan struct{}` — через `http.Server.BaseContext`, канал закрывается в начале `HTTP.Stop`. `ShutdownTimeout` не меняется.
  - `cmd/multiverse`: порядок и `init` — в `contexts.go`; фабрики — переменные пакета в `contexts_state.go` (`state`, `mechanics`), `contexts_swarm.go` (`llm`, `laws`, `swarm` = хук `newSwarm`), `contexts_gateway.go`, `contexts_memory.go`. В файлах те же заглушки.
  - `cmd/mvctl`: `main.go` собирает `cli.NewRegistry(slices.Concat(foundation(), stateCmds(), swarmCmds(), opsCmds())...)`; `commands_state.go`, `commands_swarm.go`, `commands_ops.go` содержат `cli.Reserved`. `internal/cli` не менялся.
  - `shared/contracts`: `registry.go` собирает `definitions` из `registry_gateway.go`, `registry_state.go`, `registry_swarm.go`, `registry_ops.go`; легаси-типы EPIC-001 остаются в `registry.go`.
  - `test/e2e/main_test.go`: единственный `TestMain` пакета и `registerPackageSetup(owner, setup)` (бэклог итерации 3 T-444).
- **DoD.**
  1. Тест: long-poll, ждущий 25 с, завершается меньше чем за 1 с после `Stop`; «http shutdown» в логе процесса нет.
  2. Тест: `SetDeadlines` обрывает медленную запись при ручных часах в `Deps` — время реальное.
  3. Тест порядка: `runtime.Names()` = state, laws, mechanics, llm, swarm, gateway, memory при любом порядке файлов (мутант: переименовать файл владельца — зелёный; регистрация из `init` файла владельца — красный).
  4. Вывод `mvctl help` побайтно совпадает с прежним.
  5. `All()` реестра до и после разделения совпадает (типы, порядок, поля).
  6. `MV_SWARM_FAKE` работает как раньше: тесты T-255 зелёные.
  7. После раздела `golangci-lint run ./...` (depguard T-445) — 0 issues.
  8. Регистратор `test/e2e`: порядок подготовок по имени владельца, завершения в обратном, при ошибке второй подготовки завершается первая и пакет падает; повторная регистрация — ошибка.
  9. Мутанты — в копии дерева в scratch, без `-overlay`, контрольный первым; `go build ./... && go vet ./...`, `mvctl contracts check`, `go test -short -count=1 ./...`, `go test -tags e2e ./test/e2e/...`, `make test`, `make ci BASE=develop`.
- **Ссылки:** карточка `tasks/T-446.md`; `contracts.md` v0.11 (C-01 v1.8, §16 п. 8) и ADR-001 доп. 2026-09-13 п. 7 — в T-444; КД шлюза §5.1; `ownership.md` v0.6 §1, §3.
- **Исполнитель:** developer#2 (TEAM-1, Opus). Ветка `task/T-446-runtime-cmd-layout` от эпика после слияния T-445, папка `.worktrees/T-446`. `contracts.md` не правится (текст — T-449).
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/2/1/4, вернуть), итерации 2 и ревью #2 (code-reviewer#3, 0/0/0/1, принять). Итераций ревью — 2; Nit ревью #2 (шапка карточки) исправлен при приёмке. DoD 1–9 сверен. На слитом с кончиком эпика `647c5d8` дереве: `go build`/`go vet` — 0, `go test -short` — 27 ok, e2e — ok, `golangci-lint` (и с тегом `e2e`) — 0 issues, `contracts check` — 65 типов, `mvctl help` побайтно как прежде. Go-код эпика после базы не менялся. Ссылки на §16 п. 8, ADR-001 доп. п. 7 и `ownership.md` v0.6 §1, §3 п. 6–7 после T-449 не сдвинулись. Слияние: `Makefile` — без конфликта (сдвиг 4 строки), `dev-log.md`/`review.md` — `appendtail`, `tasks.md` — конфликт хвостов, объединение (разделы эпика, затем T-446). Отметка владельца `cmd/multiverse` и `cmd/mvctl` (мягкий режим) — tech-lead#1, в карточке. Будущие слияния: T-303 — конфликт в `contexts.go`, фабрика `newGateway` переезжает в `contexts_gateway.go` (рецепт в карточке); T-060 — пересечений нет, пробное слияние с `epic/EPIC-002-state-mechanics` зелёное; T-454 — только хвосты документов. Строка DoD T-256 для tech-lead#2 — в карточке.
- **Бэклог (из T-446, 2026-09-13)** — отдельные задачи, не блокируют:
  - tech-writer, на develop (`ownership.md` §3 п. 3). В `CLAUDE.md` и `README.md` дописать файлы владельцев. Карта каталогов называет «`cmd/mvctl/main.go` — реестр» и «заглушки (`cmd/multiverse/contexts.go`)»; нужно добавить `contexts_<владелец>.go`, `commands_<владелец>.go`, `registry_<владелец>.go` и регистратор `registerPackageSetup` в `test/e2e/main_test.go`.
  - При появлении `shared/runtime/README.md` описать `SetDeadlines`/`ShuttingDown` с примером long-poll из `shutdown_test.go`. Сейчас их описывают только doc-комментарии и C-01 v1.8.
  - Дедлайн чтения для long-poll (КД шлюза §5.1 п. 8: `wait_ms + 5 с` или 0) уже передан в T-456, п. 4 состава. Отдельной задачи не нужно.
  - Через оркестратора — tech-lead#2: в DoD T-256 уточнить, что хук снимается строкой `newSwarmContext` в `contexts_swarm.go`, а константа `swarmContext` лежит в `contexts.go` EPIC-001. Через оркестратора — tech-lead#3: в T-303 фабрику `newGateway` положить в `contexts_gateway.go`, `contexts.go` не править.

### T-454: CI на Linux — гонка данных в `testkit/state` и флак fight-05 стенда `cmd/multiverse` · Размер: S · Статус: done · Волна 1
- **Причина (оркестратор, 2026-09-13)**: первый прогон `go.yml` на Linux (run 34754402826, `develop` `447b892`) красный в `unit`, `race`, `integration`. (1) DATA RACE `shared/testkit/state/consumer_test.go` — `TestAConsumerBuildsItsProjectionFromTheStub`: `(*projection).Handle` пишет в горутине `membus.Subscribe`, `waitFor` читает без синхронизации. (2) Флак `cmd/multiverse` `TestTheProcessRunsTheFightsOfIAlpha/fight-05` под `-race`: «nobody resolved the attack of player-A on wolf-alpha within 2s … the fake had not yet learnt wolf-alpha from entity.created when player-A entered». Локально `-race` недоступен (нет cgo).
- **Состав**:
  1. Проекция потребителя под мьютексом, чтение через `seen` (копия отказов); проверки прежние. Аудит «обработчик пишет — тест читает» по `shared/testkit/**`, `cmd/multiverse`, `test/e2e`: других гонок нет; скрытая передача `stand.bus/world` из горутины `process.run` в тест (детектор молчал из-за аннотации `ioSync` на вводе-выводе сокета) сделана явной каналом `opened`.
  2. Стенд `cmd/multiverse` ждёт готовности двойника по событию: обёртка транспорта `learning` отмечает `entity.created` каждой сущности bootstrap после успешного возврата обработчика `system_events` двойника; персонаж входит только после этого (`ready`, бюджет 10 с — предохранитель). Регрессия `TestTheStandWaitsUntilTheFakeHasLearntTheWorld` (факты держатся, пока стенд не начнёт ждать) воспроизводит текст CI на мутанте без ожидания.
  3. Попутно (стресс `-cpu 1`): конец боя читался из журнала раньше, чем двойник публиковал `encounter.ended` (death «ended ""» 5–7 из 200). Стенд ждёт событие с id из `closed_by_event_id` закрытого энкаунтера.
- **DoD**: мутанты M0 (контроль), M2 (без ожидания — красный, текст CI), M3 (конец боя сразу — 7/200 красных на `-cpu 1`); стресс `-count=50 -cpu 1,2,4,8` fight-тестов и `shared/testkit/state` — ok; `go build/vet`, `go test -short ./...` (27 ok), e2e, `golangci-lint` (0), `make test` — зелёные; `-race` не запускался (gcc нет). Должны позеленеть `unit`, `race`, `integration`; не проверены «Coverage floor» `unit` и e2e-часть `make test-race` (в прогоне не выполнялись).
- **Метка**: нет. Правки только в `_test.go`; production-код заглушек и `cmd/multiverse/*.go` не менялись.
- **Исполнитель**: developer#2 (Opus). Ветка `task/T-454-ci-race-testkit-state-fight05` (от эпика `dcdb530`). Карточка — `tasks/T-454.md`.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/0/1/3). Mi-1 закрыт при приёмке: unit-тесты `learning` без процесса (`cmd/multiverse/fake_contexts_learning_test.go`). `ready` не возвращается, пока двойник не обработал все факты; отметка только после возврата обработчика; ошибка двойника отметки не даёт. Мутанты в копии дерева: R1 (`ready` не ждёт `learnt`), R3 (отметка до обработчика), R6' (отметка при ошибке) — красные 10/10, контроль C0 — ошибка компиляции. N-1 (сообщение без `missing()`), N-2 (`unlearnt`: отказ двойника или факт не дошёл), N-3 (перенос комментария) — внесены без повторного ревью. Прогоны: build/vet, `-count=3` testkit и `cmd/multiverse`, `-count=10 -cpu 1,4` IAlpha и проекции, e2e, `golangci-lint` 0, `make ci BASE=epic/EPIC-001-foundation` — зелёные; `-race` — только CI. С T-446 (уже в эпике) — чисто; с T-303 (EPIC-004, `startProcess`) моделирование `git merge-file` — 0 конфликтов, рецепт на случай конфликта — карточка, «Приёмка».
- **Бэклог (из T-454, 2026-09-13)** — отдельные задачи, не блокируют:
  - `/health` у `swarm.FakeContext`/`FakeEncounter` — ok только после догоняния журнала, по образцу stateful-контекстов C-14; тогда обёртка `learning` стенда `cmd/multiverse` не нужна. Решение за tech-lead#2 (EPIC-003, владелец `shared/testkit/swarm`); до T-256 вряд ли оправдано.
  - Runbook (`Docs/ops/runbook.md`) и README: гонки проверяют только задания CI `unit`/`race`/`integration`, пока у разработчиков нет cgo (`make test`/`make ci` пишут `test-race: SKIPPED`); либо установка gcc в инструкцию разработчика — решение владельца. tech-writer, сверка — devops-engineer.

### T-450: Единое правило «локальный адрес» для скриптов и линтера compose · Размер: M · Статус: done · Волна 1
- **Причина (решение system-architect#1, заведена оркестратором)**: правило «локальный адрес» было записано трижды и по-разному:
  - гейт облака платформы `IsLocalEndpoint` в `internal/llm` (EPIC-003, T-206): loopback, `localhost`, `host.docker.internal`, RFC 1918;
  - `scripts/lib/llm-endpoint.sh` / `LlmEndpoint.psm1`: то же плюс `0.0.0.0`, `::`, `fd??:`, `fe80:` по строковому префиксу, без имени сервиса;
  - `scripts/compose-lint.sh`, правило 6: сервис своего файла и всё, что `ipaddress` Python называет private, включая `0.0.0.0/8` и адреса документации.
  Поэтому `MV_OLLAMA_URL=http://ollama:11434` линтер принимал, а платформа отвергала.
- **Правило** (решение architect#1). Ответ — один из трёх:
  - `local`: loopback `127.0.0.0/8` и `::1`, `localhost` и `*.localhost`, `host.docker.internal`, однословное имя без точки (сервис compose), RFC 1918, link-local `169.254.0.0/16` и `fe80::/10`, IPv6 ULA `fc00::/7`, IPv4-mapped формы всех перечисленных;
  - `invalid`: `0.0.0.0` и `::`, локальный хост без явного порта, порт вне 1–65535, отказы `normalizeURL` T-206 (`?`, `#`, userinfo, не http/https);
  - `cloud`: всё остальное — публичные адреса, имена с точкой (включая `.local`), CGNAT `100.64.0.0/10`, адреса документации, числовые формы.
- **Состав**:
  1. `testdata/llm/local-endpoints.tsv` — единая таблица случаев: URL, ответ, причина; не меньше 40 строк, все классы и граничные формы.
  2. `llm_endpoint_classify` в `llm-endpoint.sh` и `Get-LlmEndpointClass` в `LlmEndpoint.psm1` — классификация по правилу. `llm-server`/`llm-bench` берут класс оттуда же; изменения поведения описаны.
  3. Правило 6 `compose-lint.sh` вызывает функцию скрипта вместо своего `is_private`; фикстуры для ollama и `0.0.0.0`.
  4. Тесты паритета sh и pwsh по таблице — в стенде `testdata/script-parity` (T-405).
  5. `infrastructure.md` (новый §6.3.1, строка правила 6 в §3.1.1; §6.4 и §4.2 не тронуты), `Docs/ops/runbook.md` §3.
  - Go-тест `IsLocalEndpoint` по таблице — T-451 (EPIC-003), не здесь. ADR-005 и C-15 — T-449.
- **DoD**:
  1. Таблица ≥ 40 строк, все три класса. Покрыты граничные формы, регистр, завершающая точка, IPv6 в скобках, mapped-формы, числовые формы, порты 0/65536/без порта, `?` и `#`.
  2. Обе реализации отвечают на каждую строку как таблица; `kind` (причина одним словом) у двух половин совпадает. Проверки стенда `T01`/`T02` выполняются при любом `-run`.
  3. `compose-lint` без своей копии правила; `bash scripts/compose-lint.sh` и `--fixtures` зелёные, новые фикстуры отвергаются правилом 6 по своей причине.
  4. Мутанты краснеют: «однословное имя — облако» и «`0.0.0.0` не any-address» — в обеих половинах стенда и в фикстурах `compose-lint`; контрольный мутант первым.
  5. `make scripts-parity`, `make parity-mutants`, `bash -n`, разбор `.ps1`, `make ci BASE=develop` — зелёные.
- **Исполнитель**: devops-engineer#2 (Opus). Ветка `task/T-450-local-endpoint-table` (от эпика с T-405, `ab6cb1d`). Карточка — `tasks/T-450.md`.
- **Решения исполнителя** (уточнения правила, записаны в шапке таблицы; на ревью architect#1):
  - завершающая точка снимается только у зарезервированных имён (`localhost.`, `*.localhost.`, `host.docker.internal.` — `local`); IPv4 с точкой (`127.0.0.1.`, `0.0.0.0.`) и `ollama.` — `cloud` (решение system-architect#1, итерация 2);
  - однословное имя начинается с буквы и состоит из `[a-z0-9_-]`;
  - записанный порт считается явным, включая `:80`; ведущие нули порта незначимы;
  - `host:` с пустым портом, скобки не вокруг IPv6 и `%` в хосте (экранирование, зона IPv6) — `invalid`;
  - из `0.0.0.0/8` any-address — только `0.0.0.0`; `::127.0.0.1` и `64:ff9b::/96` — не mapped-формы.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #2 (0/0/1/2), итераций ревью — 2. При приёмке закрыты:
  - Mi-R2-1: userinfo вырезается из печатаемого значения до первого отказа, одинаково в `llm-endpoint.sh` и `LlmEndpoint.psm1`. Проверки: сценарии `H61`–`H66` с `Absent: FAKEPW123`, мутант M28, фикстура `bad-llm-url-userinfo`; мутант `compose-lint` C8 красный;
  - N-R2-1: строка про точку выше;
  - N-R2-2: `infrastructure.md` §3.1.2 — 29 мутантов, 105 сценариев, `T01`/`T02`; §4.2 п. 4 — ссылка на §6.3.1.

  Прогоны зелёные: `make scripts-parity` (107 PASS, 2 KNOWN-FAILING, `T01`/`T02` по 123), мутанты M00, M01, M18–M28, `compose-lint` и `--fixtures` (58 bad, 11 good), `bash -n`, разбор `.ps1`, `secrets-scan`, `gitleaks dir`. Слияние: T-450, затем T-455. Три файла сливаются без конфликтов; в `tasks.md` конфликт от дописывания в конец с обеих сторон. Подробности — в карточке, раздел «Приёмка (tech-lead)».
- **Бэклог (из приёмки T-450, 2026-09-13)** — отдельные строки, не блокируют:
  1. T-451 (DoD): Go-тест читает таблицу и `endpointStandCases`; `canonicalHost` не снимает точку у IPv4; `needsExplicitPort` — любой `local`; `0.0.0.0`/`::` — ошибка конфигурации; тексты ошибок не печатают userinfo.
  2. T-456 (DoD, C-15): символы хоста, которые отвергает `url.Parse` (пробел, `\`, `^`, `` ` ``, `{`, `|`, `}`), — в список `invalid`; ни одна реализация не печатает userinfo в текстах отказов.
  3. Nit: комментарий `LLM_EP_RAW` в `llm-endpoint.sh:152` («with any userinfo masked») устарел.
  4. Из итерации 2 исполнителя: топологическая проверка `kind=service` в `compose-lint` (N-3 ревью #1); скорость `llm_parse_url` без fork; сценарий на `//` в конце пути (ревью #1, п. 4).

### T-455: CI `compose-lint` на `develop` — окружение процесса скрывало заглушку `CHROMA_IMAGE` · Размер: XS · Статус: done · Волна 1
- **Причина (оркестратор, 2026-09-13)**: первый прогон CI на `develop` (run 34754402826, коммит `447b892`, job 103716226089). Задание `compose-lint` упало на шаге «The house rules»: `error while interpolating services.chromadb.image: required variable CHROMA_IMAGE is missing a value`. Шаг `docker compose --env-file build/versions.env --env-file .github/ci.env config -q` перед ним прошёл, фикстуры и hadolint пропущены.
- **Найдено (devops-engineer#1)**: не `.env` владельца — копия без `.env` проходит, а с `--env-file` compose `.env` не читает. Шаг «Read the pinned versions» экспортирует `build/versions.env` в `$GITHUB_ENV`, в том числе пустой по D-3 `CHROMA_IMAGE`. Переменная процесса у compose выше `--env-file`, и она скрыла заглушку `.github/ci.env`. Шаг `config -q` читает только `docker-compose.yml`, где `CHROMA_IMAGE` нет.
- **Состав**:
  1. `scripts/compose-lint.sh`: до первого `docker compose` снять из окружения все имена, объявленные в `--env-file` линтера, `build/versions.env` и `.env.example` правила 7 (или в парном `.env` фикстуры). Правило 6 и шапку не трогать.
  2. `--fixtures`: каждый прогон — с этими именами, экспортированными пустыми. Зависимость линтера от окружения краснеет на любой машине, не только в CI.
  3. D-3 не меняется: `build/versions.env`, `.github/ci.env`, `docker-compose.legacy.yml` (`${CHROMA_IMAGE:?}`) и правило 7 — без правок.
  4. `testdata/compose-lint/README.md` и `infrastructure.md` §3.1.1 («Что читается», «Фикстуры») — вслед за скриптом.
- **DoD**:
  - scratch-копия `git archive HEAD` без `.env` с `build/versions.env`, экспортированным как в CI: до правки — ошибка CI дословно, после — ok;
  - `scripts/compose-lint.sh` — ok (15 сервисов, 8 правил); `--fixtures` — ok (52 bad, 10 good); `bash -n` — ok; `CHROMA_IMAGE= QDRANT_IMAGE= make compose-lint` — ok;
  - контрольный мутант (снятие имён отключено) — `--fixtures` красный;
  - остальные шаги задания `compose-lint` и `scripts-parity` проверены на зависимость от окружения (карточка);
  - `make secrets-scan BASE=epic/EPIC-001-foundation` — rc=0; `gitleaks dir` по копии изменённых файлов — no leaks;
  - зелёный `compose-lint` в CI на `develop` — после слияния, оркестратор.
- **Метка**: нет. Контракты и Go-код не меняются.
- **Исполнитель**: devops-engineer#1. Ветка `task/T-455-ci-compose-lint-chroma-image` (от эпика `dcdb530`). Карточка — `tasks/T-455.md`.
- **Бэклог (из T-455, 2026-09-13)** — отдельные задачи, не блокируют:
  - `go.yml:425-426`, задание `compose-lint`: убрать шаг «Read the pinned versions» (выбран при приёмке). Шаг `docker compose … config -q` (`:438`) несёт ту же ловушку: окружение шага выше `--env-file`. Сейчас он безопасен, потому что в `docker-compose.yml` нет `${VAR:?}` с пустым значением в `build/versions.env`. В копии с окружением CI `config -q` — rc=0, тот же вызов с `-f docker-compose.legacy.yml --profile legacy` — rc=1. Ни одному шагу задания пины из окружения не нужны: compose получает их через `--env-file`, hadolint их не читает. Запасной вариант — `env -u` по именам `build/versions.env` для `config -q`. devops-engineer.
  - hadolint (`go.yml:450`, `:460`) на `develop` ещё ни разу не выполнялся: в прогоне 34754402826 оба шага пропущены после падения. Проверить первый реальный прогон после слияния эпика в `develop`. devops-engineer.
  - Упавшие в том же прогоне `unit` (`go test -short -race`), `race` (`make test-race`) и `integration` к T-455 не относятся — разбор отдельными задачами, оркестратор.
- **Приёмка (tech-lead#1, 2026-09-13)**: принята после ревью #1 (0/0/1/2); Mi-1, N-1 и N-2 закрыты при приёмке. `declared_names` понимает `KEY: value` (разделитель `[=:]`): проба `T455_PROBE: dummy` в копии без `.env` при `T455_PROBE=` даёт тот же вердикт, что в чистом окружении, а мутант с `=` повторяет дефект T-455. Тексты скрипта, `infrastructure.md` §3.1.1 и README фикстур сведены к тому, что проверяют фикстуры; умолчание env-файлов — одна переменная `default_env_files`. Рабочая папка: `bash -n`, `compose-lint.sh`, `--fixtures` (52/10), `make compose-lint` — ok; `make secrets-scan BASE=epic/EPIC-001-foundation` — rc=0; `gitleaks dir` по изменённым файлам — no leaks. Копия с `set -a; . build/versions.env; set +a`: основной прогон и `--fixtures` — ok. **Порядок слияния: сначала T-450, затем T-455**; модель `merge-file` после правок приёмки — без конфликтов, после синхронизации повторить `--fixtures`. Подробности — карточка `tasks/T-455.md`, «Приёмка (tech-lead)».

### T-457: Очередь решений system-architect по вопросам ревью и приёмок (ADR-029, T-054, T-060) · Размер: S · Статус: done · Волна 1 · contract-change
- **Причина.** При ревью и приёмках накопились вопросы, которые без system-architect не решаются:
  - EPIC-003, T-439: ADR-029 (ярлыки вызова нарратива) предлагает поправки к ADR-016 и ADR-017 и запрашивает новую версию C-07; ревью #2 T-439 добавило Mi-9, Mi-11 и вопрос о `labels_hash`;
  - EPIC-002, приёмка T-054: вопросы В1–В4 (участие погибшего, мёртвый NPC в `npcs[]`, причина отказа на переход терминальный → живой, `players_present` и inv-10);
  - EPIC-002, T-060: доступ `RecordedProvider` к записи при depguard, время в обработчике replay, время корней в e2e-replay; N-1 ревью T-446 (порядок `All()`).
- **Что сделать** (только документы; код и схемы — задачи эпиков):
  - решения — в карточке `tasks/T-457.md` с доводами и строками DoD для задач-исполнителей;
  - `architecture/contracts.md` — новая версия документа, сводка, истории изменённых контрактов (C-01, C-02, C-05, C-07);
  - при подтверждении ADR-029 — пометки «см. ADR-029» в ADR-016 и ADR-017; в ADR-005 УИ п. 4 — только строка «см. ADR-029», числа — перечнем для architect#1;
  - КД State, `analysis/data-model.md` — по В1–В4.
- **DoD:**
  1. У каждого вопроса — решение, довод и отвергнутые варианты в карточке.
  2. У изменённых контрактов — версия в заголовке и запись в истории; строка сводки в `contracts.md`.
  3. Схемы, код, файлы T-439 (ADR-029, КД роя), `swarm-llm-laws.md` ветки EPIC-001 и C-08 (T-456) не менялись.
  4. Строки DoD для задач-исполнителей EPIC-002, EPIC-003, EPIC-004 и T-458 (EPIC-002, «запись сессии») перечислены (итерация 2: новая задача — в EPIC-002, а не в EPIC-001).
  5. CRLF сохранён, байтов NUL нет; `gitleaks dir --redact` по копии изменённых файлов — 0.
- **Ссылки:** `review.md` EPIC-003 «T-439 · ревью #1», «ревью #2»; ADR-029 (`.worktrees/T-439`); карточки T-054 и T-060 (`epic/EPIC-002-state-mechanics`, «Приёмка», «Отметка владельца»); `review.md` EPIC-002 «T-054 · ревью #1»; `review.md` EPIC-001, ревью T-446, N-1.
- **Исполнитель:** system-architect#1, метка `contract-change`. Ветка `task/T-457-architect-decisions-queue` от `epic/EPIC-001-foundation` (647c5d8). T-456 (C-08) — после этой задачи: тот же `contracts.md`.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #2 (0/0/3/3), итераций ревью — 2. R2-Mi-1…R2-Mi-3 и R2-N-1…R2-N-3 закрыты итерацией 3 исполнителя без повторного ревью; закрытие сверено по тексту. DoD 1–5 подтверждены: схемы, код, `swarm-llm-laws.md` и C-08 не менялись; CRLF, NUL нет; `make secrets-scan` и `gitleaks dir` по копии — 0. Слияние с `epic/EPIC-001-foundation` (`2a0b074`): общие документы без пересечений (после базы там менялся только `infrastructure.md`), конфликт — дописывание в конец `tasks.md`, `dev-log.md`, `review.md`. Раздел T-456 дополнен: номера после T-457, п. 9–10. Строки DoD для задач других эпиков — карточка, «Строки DoD для владельцев задач»; передаёт оркестратор. Подробности — карточка, «Приёмка (tech-lead)».

### T-460: `eventbus.Permanent` и `Policy.World` (C-01 v1.10) · Размер: XS–S · Статус: done · Волна 1 · contract-change
- **Причина (T-456, 2026-09-13).** `contracts.md` v0.14 вводит в C-01 v1.10 окончательную ошибку обработчика `Permanent` и правило мира в политике типа `Policy.World`; C-02 v1.7 требует мир в конверте предложения State. Код — эта задача (карточка T-456, «Строки DoD для владельцев задач», блок «EPIC-001 (tech-lead#1; T-460, `contract-change`, XS–S)», итерация 2 с Ma-2).
- **Ветка**: `task/T-460-permanent-policy-world` от `epic/EPIC-001-foundation` (`a0d45d2`), папка `.worktrees/T-460`.
- **Зависит от**: T-456 (принята и слита). Блокирует T-056 (EPIC-002: `Policy.World: WorldRequired` у `entity.create.proposed` и `entity.update.proposed` — после T-460 из develop). Параллельно с T-461 — пересечение только `dev-log.md`.
- **Файлы** (по рабочей копии исполнителя на 2026-09-13; окончательный список — карточка): `shared/eventbus/delivery.go`, `shared/eventbus/registry.go`, `shared/eventbus/README.md`, тесты `shared/eventbus/permanent_test.go`, `shared/eventbus/policy_world_test.go`, `shared/eventbus/membus/policy_world_test.go`; contract-тест `shared/testkit/contract/contract.go`, `membus_test.go`, `redpanda_integration_test.go`.
- **Описание**: Go-API `shared/eventbus` расширяется строго по C-01 v1.10; нулевые значения сохраняют прежнее поведение. Любое отклонение от текста контракта — только через system-architect.
- **DoD** (карточка T-456, блок EPIC-001):
  1. `shared/eventbus`: `ErrPermanent` и `Permanent(err error) error`; `nil` → `nil`; для обёртки `errors.Is(w, ErrPermanent)` и `errors.Is(w, cause)` истинны.
  2. `Delivery.Deliver` при живом контексте: окончательная ошибка — один вызов, парковка сразу, `attempts` = номер вызова, лог `Error handled=true` без `panic`/`stack`. При отменённом контексте подписки `Permanent` не паркуется: `Deliver` возвращает `ctx.Err()`, событие не фиксируется, проверка отмены — раньше парковки (Ma-2).
  3. README пакета — раздел «Окончательная ошибка»: только дефект события, который повтор не исправит ни в каком процессе; состояние получателя (остановленный мир) окончательной ошибкой не считается.
  4. Contract-тест `shared/testkit/contract` на `membus` и kafka. При живом контексте: один вызов, `dead_letters` с `attempts = 1`, без пауз (ручные `Timers` не двигаются). При отменённом: `ctx.Err()`, dead letter нет, событие выдаётся снова новой подписке той же группы. Мутанты «`Permanent` как обычная ошибка» и «парковка под отменой» — красные.
  5. `Policy.World WorldRule` (`WorldOptional` — нулевое значение, `WorldRequired`): `Check` отвергает `World == nil` и пустой `World.Entity.ID` с `ErrPolicyViolation`. Тесты: `Publish` на `membus`; проверка при чтении — dead letter с `attempts = 0`.
  6. `All()` реестра до и после совпадает: строк типов задача не ставит.
  7. Общий DoD §1: `go build ./...`, `go vet ./...`, `go test -short ./...`, `golangci-lint run ./...` — 0 issues, `mvctl contracts check`; kafka-половина contract-теста — один прогон интеграционного набора `shared/testkit/contract`; dev-log, карточка.
- **Поставка**: в develop, оттуда синхронизация EPIC-002.
- **Исполнитель**: developer#2 (TEAM-1, Opus). Карточка — `tasks/T-460.md` (ведёт исполнитель).
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/0/2/1), итераций ревью — 1. DoD 1–7 подтверждены. При приёмке закрыты с тестами: Mi-2 (тест паники со значением `Permanent` проверяет лог при живом и отменённом контексте; мутант R6 ревью — красный), Mi-1 (фикстуры правила (г) `mvctl contracts check` получили мир, тест на `WorldRequired` у типов `player_events`/`llm_records`), N-1 (`contract.StalledBackoff()` — функция). `delivery.go`, `registry.go` и код шины при приёмке не менялись. Тексты `ErrPermanent` и строки лога — на подтверждении system-architect (T-458, подраздел «Попутно: T-460»). Сборка на трёхстороннем слиянии с `3f34195` чистая. Подробности — карточка, «Приёмка (tech-lead)».
- **Бэклог (из T-460, 2026-09-13)** — не блокирует:
  1. tech-lead#1: contract-кейс «предложение без мира» в `shared/testkit/contract` после T-056 (первые строки `WorldRequired`). Выигрыш мал: обе шины проверяют через тот же `Delivery`.
  2. system-architect: голый `ErrPermanent` без причины — C-01 не говорит, разрешён ли он; сейчас паркуется с текстом «eventbus: permanent handler error».
  3. tech-lead#1: выровнять строки лога паники («goes to») и окончательной ошибки («is parked») — вместе с бэклогом T-441 и после подтверждения текстов (п. 2 бэклога исполнителя).

### T-461: Правило depguard `cmd-telegram-bot` в `.golangci.yml` · Размер: XS · Статус: done · Волна 1 · предусловие T-311
- **Причина (оркестратор, 2026-09-13).** Бот Telegram (`cmd/telegram-bot/**`, EPIC-004) — тонкий клиент gateway (ADR-018; КД gateway-and-bot §3; ADR-001 доп., граница 1e). Правила depguard для бота не было, а шаблон `**/internal/**` правила `internal-unlisted` совпадает с `cmd/telegram-bot/internal/**` и запрещает даже разрешённые `internal/gateway/client` и `internal/gateway/api`. Текст правила проверен tech-lead#1 при отметке владельца T-310 (карточка T-310, EPIC-004, «Отметка владельца», «Правило depguard `cmd-telegram-bot`»).
- **Ветка**: `task/T-461-depguard-telegram-bot` от `epic/EPIC-001-foundation` (`a0d45d2`), папка `.worktrees/T-461`.
- **Зависит от**: T-310 (отметка владельца с текстом правила). Блокирует T-311 (EPIC-004). Параллельно с T-460 — пересечение только `dev-log.md`.
- **Файлы**: `.golangci.yml` (две вставки в `linters.settings.depguard.rules`).
- **Описание**: правило `cmd-telegram-bot` перед `cmd-others` — `files` `**/cmd/telegram-bot/**`, `allow` `internal/gateway/client$` и `internal/gateway/api$`, `deny` `multiverse-core.io/internal` и `multiverse-core.io/shared/eventbus` с `desc`. В `internal-unlisted` — исключение `!**/cmd/telegram-bot/**` с комментарием. Текст — побайтно по карточке T-310. В ветке EPIC-001 кода бота нет, поэтому проверка — на копии кончика EPIC-004.
- **DoD**:
  1. Правило и исключение внесены ровно по тексту карточки T-310, правило стоит перед `cmd-others`.
  2. `golangci-lint run ./...` в рабочей папке — 0 issues.
  3. На копии кончика EPIC-004 с новым конфигом: код как есть — 0 issues; мутанты `shared/eventbus` и `internal/gateway/links` — находка `cmd-telegram-bot`; контроль `internal/gateway/client` — 0. Копия удалена по точному пути.
  4. Общий DoD §1: dev-log, карточка.
- **Исполнитель**: developer#1 (TEAM-1, Opus). Ревью — code-reviewer#1. Карточка — `tasks/T-461.md`.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/0/0/2), итераций ревью — 1. DoD 1–3 подтверждены: оба блока карточки T-310 входят в `.golangci.yml` по одному разу побайтно, файл в LF; `golangci-lint config verify` — ok, `run ./...` — 0 issues. На копии `c92eb22` мутанты `shared/eventbus` и `internal/gateway/links` дают по находке `cmd-telegram-bot`, контроли `client` и `api` — 0. N-1 и N-2 текст правила не меняют и ушли в бэклог system-architect. С T-460 файлы не пересекаются, кроме `dev-log.md`. Подробности — карточка, «Приёмка (tech-lead)».
- **Бэклог (из T-461, 2026-09-13)** — system-architect, не блокирует:
  1. Ловушка для новых `cmd/*` по образцу `internal-unlisted`: сейчас `cmd/<новый>` импортирует любой `internal/*` без находки. Вариант — `cmd-unlisted` (`**/cmd/**` без `cmd/multiverse`, `cmd/mvctl`, `cmd/telegram-bot`, запрет `multiverse-core.io/internal`); сначала сверить импорты `cmd/mvctl`. Учесть N-2: исключение бота нельзя привязать к корню модуля.
  2. До T-315: можно ли тестам бота брать двойники `shared/testkit/*`, которые приносят шину. Если нельзя — запрет `shared/testkit` в `cmd-telegram-bot` и отдельное правило для тестов бота; если можно — уточнить комментарий правила.

### T-463: Devops — срок архивов `make backup`, маскировка значения в отказах LLM-скриптов, `MV_STATE_WORLDS` и `MV_TELEGRAM_*` в compose · Размер: M · Статус: done · Волна 1 (бэклог)
- **Причина (оркестратор, 2026-09-13)** — три независимых хвоста:
  - (а) ревью безопасности T-318 (EPIC-004), M-1: `make backup` не удаляет старые архивы томов MinIO и Redpanda, и копия данных игроков живёт дольше 30 дней. Решение оркестратора: архивы бэкапа — не дольше 30 дней. Ревью безопасности #2 T-318, R2-Mi-1: удалять по возрасту отдельным ежедневным заданием, а не только внутри `make backup`; копии `links.db` и `gateway.db` — тоже не старше 30 дней;
  - (б) C-15 v1.5 «Печать значения в отказах» (T-456): если `@` есть где угодно в значении, ни одна фраза отказа не называет хост. Пометка «Код расходится»: `llm_endpoint_judge` печатает значение целиком. `http://fakepw/x@127.0.0.1:8888` разбирается как хост `fakepw` без порта, и отказ печатает `fakepw/x` и `127.0.0.1`;
  - (в) отметка владельца T-055 (EPIC-002), О-1: `core` не передаёт `MV_STATE_WORLDS`; отметка владельца T-310 (EPIC-004), замечание 1: `docker-compose.bot.yml` не передаёт `MV_TELEGRAM_ACTION_KEY_SALT` и `MV_TELEGRAM_COMMANDS_PER_MIN`.
- **Ветка**: `task/T-463-backup-retention-endpoint-mask` от `epic/EPIC-001-foundation` (`3f34195`), папка `.worktrees/T-463`. Часть (в) по решению оркестратора вынесена в T-464 (при приёмке T-461).
- **Зависит от**: (а) и (б) — нет (копий SQLite ещё нет: `multiverse db backup` появится в EPIC-004, тест их имитирует). (в) — объявления переменных в манифесте ветки: `MV_STATE_WORLDS` пока только в EPIC-002 (T-055), `MV_TELEGRAM_*` — только в EPIC-004 (T-310); без них правило 8 `compose-lint` не найдёт умолчание. (в) — после слияния этих задач в develop и синхронизации EPIC-001, не позже T-390. С T-460 и T-461 пересечение только `dev-log.md`.
- **Файлы**: (а) `Makefile` (цели `backup` и `backup-prune`), новый скрипт удаления (например `scripts/backup-prune.sh`) и его тест, `Docs/ops/runbook.md` (срок, ежедневное задание Планировщика задач), `infrastructure.md` §5.6 (эксплуатационная часть), при необходимости `.github/workflows/go.yml`; (б) `scripts/lib/llm-endpoint.sh`, `scripts/lib/LlmEndpoint.psm1`, `testdata/script-parity/scenarios.go`, `testdata/script-parity/mutants.go`, фикстура `testdata/compose-lint/`; (в) `docker-compose.yml`, `docker-compose.bot.yml`.
- **Описание**: подробно — карточка `tasks/T-463.md`. (а) Отдельная очистка, которую раз в сутки запускает Планировщик задач Windows на стенде, удаляет в `$(BACKUP_DIR)` всё, что строго старше 30 суток: архивы `minio-*.tgz` и `redpanda-*.tgz` (вместе с их строками в `SHA256SUMS`) и копии `links.db`/`gateway.db` в `links/`. Удаляет по возрасту, без «оставить последние N», по точным путям. `make backup` может вызывать ту же очистку, но срок от него не зависит. (б) При `@` в значении фразы `llm_endpoint_judge` и `LlmEndpoint.psm1` печатают значение только схемой и не называют хост. (в) Три переменные передаются в свои сервисы с умолчаниями манифеста.
- **DoD** (полностью — карточка):
  1. (а) Удаление строго по возрасту (старше 30 дней), никогда не «оставить последние N» и не только внутри `make backup`: если бэкапы прекратились, старые копии всё равно удаляются.
  2. (а) Удаление — отдельное ежедневное задание (Планировщик задач Windows на стенде); регистрация, проверка и признак пропуска — в runbook. Регистрирует владелец.
  3. (а) Тест удаления по возрасту: поддельные часы (`--now`) или mtime файлов во временном каталоге, удаление только по точным путям внутри него. 31 день — удалено, 29 — на месте; каталог только со старыми копиями очищен полностью; чужие файлы и цель ссылки вне каталога — на месте. Мутанты «порог 0» и «оставить последний» — красные. Тест — в `make ci` и в задании CI на Linux.
  4. (а) `infrastructure.md` §5.6 согласован с решением «архивы бэкапа — не дольше 30 дней». *Пометка:* текст раздела — system-architect и devops; исполнитель правит эксплуатационную часть, расхождения по существу передаёт строкой system-architect.
  5. (а) Копии `links.db` и `gateway.db` на хосте — не старше 30 дней: то же удаление, формат имён — в runbook; строка для T-317 (EPIC-004).
  6. (б) Сценарии стенда паритета на каждую ветку фразы, включая `MV_LLM_URL=http://fakepw/x@127.0.0.1:{PORT}`, с `Absent` для `fakepw` и `127.0.0.1`; мутант «маскировка снята» красный в обеих половинах; фикстура правила 6 `compose-lint`; `make scripts-parity`, `make parity-mutants` — зелёные; стенд LLM 127.0.0.1:8888 не используется.
  7. (в) Строки в `docker-compose.yml` и `docker-compose.bot.yml` в форме, принятой правилами 3 и 8; `make compose-lint` и `--fixtures` — ok; значения из env-файла пробы доходят до `environment`.
  8. `contracts.md` не правится; строка для system-architect — ~~снять пометку «Код расходится» в C-15 и ADR-005 после слияния~~ пометку сузить до трёх отказов `llm-bench` и строк успеха и `health` `llm-server`; остальная маскировка — T-468 (изменено при приёмке, решение оркестратора — вариант (б)).
  9. `make ci BASE=epic/EPIC-001-foundation`, `make secrets-scan`, `gitleaks dir` по изменённым файлам — зелёные; dev-log, карточка.
- **Метка**: нет. Go-код платформы, контракты, `.env.example` и `shared/env/**` не меняются.
- **Исполнитель**: devops-engineer (экземпляр назначает оркестратор). Карточка — `tasks/T-463.md`.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #2 (0/1/0/2), итераций ревью — 2. Части (а) и (б); (в) — T-464. Ma-2 закрыт решением оркестратора (вариант (б)): код `llm-bench` не менялся, строка для system-architect сужена (карточка, dev-log), маскировка всего вывода скриптов — T-468. N-7 и N-8 закрыты при приёмке: `WARNING … its mtime is more than a day ahead of now` с тестом в случае `mixed` и мутантами P17/P18, комментарий `--fixtures` в `compose-lint.sh`. DoD 1–10 и 15 подтверждены; 11 изменён решением оркестратора; 12–13 — T-464; 14 — по составным частям, без полного `make ci` (см. карточку). Мутанты очистки в `make ci` не входят, только в CI. Слияние с кончиком `3e3c95b` — см. карточку, «Приёмка (tech-lead)».
- **Бэклог (из T-463, 2026-09-13)** — не блокирует:
  1. devops: срок журнала `backup-prune.log` и строка о последнем запуске очистки в `make health` (N-5 ревью #1).
  2. devops: `--mutants` для `compose-lint` по образцу `script-parity` — мутанты маски правила 6 дважды делались вручную.
  3. devops: сценарий с подменой `date` для проверки штампа туда-обратно (`stamp_epoch`), если на стенде появится не-GNU `date`.

### T-470: Документы по просмотру T-056 и ревью T-458 — `contracts.md` v0.15 (C-01 v1.11, C-02 v1.8, C-03 v1.4, C-05 v1.8a, C-14 v1.3, C-15 v1.6), КД State, `data-model.md` §3, ADR-001, ADR-005 · system-architect · M · Статус: done · Волна 1 · contract-change
- **Причина (оркестратор, 2026-09-13)**: тексты просмотра system-architect T-056 (C-02 v1.8, C-03 v1.4, C-14 v1.3, КД State) и п. 6 ревью #1 T-458 с поправками ревью #2 (C-01 v1.11, КД State §6.1, §6.2); сужение пометки «Код расходится» в C-15 и ADR-005 (решения оркестратора по T-463 и T-468); вопросы в пределах полномочий system-architect — смерть персонажа (C-05), соль `MV_TELEGRAM_ACTION_KEY_SALT` в `infrastructure.md` §3.1.1, `@` в пути адреса (ревью #2 T-451).
- **Ветка**: `task/T-470-contracts-state-grammar` от `epic/EPIC-001-foundation` (`55ec4c4`), папка `.worktrees/T-470`.
- **Покрывает**: T-466 (C-01 v1.11, КД State §6.1, §6.2) — закрыта оркестратором как покрытая этой задачей.
- **Условие слияния (приёмка)**: C-02 v1.8 п. 2 считает скалярами `name` и `last_session_ended_at` без пометки «Код расходится», потому что их вносит в `scalarAttributes` приёмка T-056. На момент приёмки T-470 в рабочей копии `.worktrees/T-056` их нет. T-470 сливается после того, как они там появятся; иначе перед слиянием нужна пометка «Код расходится» у C-02 v1.8 п. 2 и КД §3.2 (через system-architect).
- **Файлы**: `architecture/contracts.md`, `architecture/components/state-and-mechanics.md`, `analysis/data-model.md` §3.2–§3.4, `architecture/adr/ADR-001-modular-monolith-topology.md`, `architecture/adr/ADR-005-llm-gateway-providers-models.md`, `architecture/infrastructure.md` §3.1.1 п. 3 (одна строка).
- **DoD** (полностью — карточка `tasks/T-470.md`):
  1. У изменённых контрактов — версия в заголовке и запись в истории; строка сводки v0.15.
  2. Тексты совпадают с одобренными в просмотре T-056 и в п. 6 ревью #1 T-458 (с поправками ревью #2); отступления названы с доводом.
  3. Правки помечены «(изм. T-470)»; тексты C-15 и ADR-005 о правиле «локальный адрес» совпадают построчно.
  4. CRLF, без NUL и BOM; `swarm-llm-laws.md`, `.env.example` и эталоны `infrastructure.md` §4.2 не тронуты; `gitleaks dir --redact` по изменённым файлам — 0.
- **Метка**: `contract-change` (документы). Код, схемы и таблица адресов — T-471, T-472, T-057 (EPIC-002), T-212, T-237 и XS-задача `@` в `IsLocalEndpoint` (EPIC-003), T-468 и T-469 (EPIC-001).
- **Исполнитель**: system-architect#1 (Opus). Ревью — code-reviewer#1. Карточка — `tasks/T-470.md`.
- **(приёмка tech-lead#1, 2026-09-14)** Принята после ревью #2 (0/0/1/1), итераций ревью — 2. DoD 1–4 подтверждены. Замечания ревью #2 закрыты при приёмке: Mi-3 — источник скаляров сужен до таблиц сущностей `data-model.md` §3 без §3.5 Item (C-02 v1.8 п. 2, КД §3.2, строка T-472 карточки); N-5 — у примера отдыха в «Причине отказа по связке» пометка «Код расходится до T-471», место добавлено в строку T-471. Условие слияния — выше. Слияние с кончиком `b085228` — см. карточку, «Приёмка (tech-lead)».
- **Передано в индекс EPIC-001 (из T-470)**:
  - T-468 — объём по C-15 v1.6 внесён в раздел T-468.
  - T-469 (раздел T-469 добавлен приёмкой T-464 на кончике эпика, в ветке T-470 его нет — строки здесь):
    1. `MV_ANTHROPIC_API_KEY` передаётся сервису `core` (контекст `llm`) формой `${MV_ANTHROPIC_API_KEY:-}` — законно пустой облачный секрет (`infrastructure.md` §3.1.1 п. 3), без `:?` и без ключа без значения; фикстура правила 3 `compose-lint` — на литерал этой переменной (отказ). **Выполнено в T-469** (приёмка 2026-09-14): строка в `core`, фикстура `bad-secret-anthropic-literal.yml`.
    2. После передачи — строка для system-architect: снять оговорку «`MV_ANTHROPIC_API_KEY` в композицию пока не передаётся — T-469» в `infrastructure.md` §3.1.1 п. 3 (заменой одной строки, эталоны §4.2 не сдвигать).
- **Передано в индексы других эпиков (через оркестратора)**: EPIC-002 — T-471, T-472, T-057, приёмка T-056; EPIC-003 — T-212, T-237, новая XS-задача (`@` в `IsLocalEndpoint`). Тексты — карточка, «Передать».
- **Бэклог (из T-470, 2026-09-14)** — не блокирует:
  1. system-architect, после T-471: в КД §4.1 сократить зачёркнутый хвост T-444 «решение о ней — за EPIC-002» до ссылки на C-02 v1.8 п. 6 (ревью #2, п. 2).
  2. Открытые вопросы 1–8 карточки (чтение истории на валидирующем журнале, e2e бота, единица `response_len`, `MV_STATE_WORLDS`/`MV_WORLD_ID` в §4.2 и др.) — в очередь system-architect; вопрос 1 — до T-212 и T-237.

### T-468: Devops — значение с `@` во всём выводе LLM-скриптов: строки успеха и `health` `llm-server`, `llm-bench` · Размер: M · Статус: todo · Волна 1 (бэклог)
- **Причина (оркестратор, 2026-09-13; приёмка T-463)**:
  - ревью #2 T-463, Ma-2: три отказа `llm-bench` при принятом значении с `@` печатают пробу и значение целиком (`scripts/llm-bench.sh:844-846`, `scripts/llm-bench.ps1:478-482`);
  - ревью #1 T-463, Mi-1: строки успеха и `health` `llm-server` делают то же;
  - решение оркестратора — вариант (б): объём T-463 не расширять, вся маскировка вывода скриптов — эта задача. Пометка «Код расходится» в C-15 v1.5 и ADR-005 до её слияния сужена до этого перечня.
- **Ветка**: `task/T-468-script-output-at-mask` от `epic/EPIC-001-foundation` (после слияния T-463), папка `.worktrees/T-468`.
- **Зависит от**: T-463 (слита); ответ system-architect — распространяется ли правило C-15 «при `@` хост не называется» на весь вывод скриптов (бэклог ревью #2 T-463, п. 1). Если только на отказы — объём сужается до отказов `llm-bench` и строк `health` с кодом 1. Пересечение с T-464 — нет.
- **Файлы**: `scripts/llm-server.sh`, `scripts/llm-server.ps1`, `scripts/llm-bench.sh`, `scripts/llm-bench.ps1`, `testdata/script-parity/scenarios.go`, `testdata/script-parity/mutants.go`, `infrastructure.md` §3/§6.3.1.
- **Описание**: подробно — карточка `tasks/T-468.md`. При `@` в значении не называют ни хост, ни пробу, ни значение: строки успеха `llm-server` (`probing … (from VAR=RAW)`, `is not an address of this machine`, `the platform calls it at …`); строки `health` с кодом 1; три отказа и строка пропуска фазы `llm-bench`; поля `url`/`probe_url` отчёта `llm-bench`. Формы — как `U19` T-463 (переменная и схема, хвост «not printed»). Без `@` строки прежние побайтно, половины совпадают.
- **DoD** (полностью — карточка):
  1. Строки из описания (в объёме по ответу system-architect) при `@` не содержат хост, пробу и значение, в любом регистре; без `@` — прежние.
  2. Сценарии паритета: по одному с `@` на строку, по одному без `@` на группу; P01/P02 зелёные.
  3. Мутант «маскировка снята» в каждой половине на каждую группу краснеет; контрольный первым; `make parity-mutants` — зелёный.
  4. Отчёт `llm-bench` при `@` без хоста и значения в `url`/`probe_url`; форма поля — с доводом.
  5. `make scripts-parity`, `bash -n`, разбор `.ps1`, `make compose-lint`, `golangci-lint run ./...` — зелёные; строка для system-architect — снять пометку после слияния; dev-log, карточка.
- **Объём по C-15 v1.6 (передача T-470; внесено tech-lead#1 при приёмке T-470, 2026-09-14)** — ответ system-architect на зависимость «распространяется ли правило на весь вывод»; строки ниже заменяют описание и DoD в части объёма. П. 1–4 описания карточки и DoD 1, 5 карточки снимаются строкой в «Выполнении» со ссылкой на C-15 v1.6.
  1. Правило печати относится ко всему выводу. Значение с `@` в любом месте — `invalid`, поэтому до строк успеха и `health` `llm-server`, отказов и полей отчёта `llm-bench` оно не доходит, и их маскировка не нужна.
  2. `testdata/llm/local-endpoints.tsv` — три строки сразу после строки `http://@127.0.0.1:8888`; текст побайтно — карточка `tasks/T-470.md`, «Передать», T-468 п. 2 (те же строки ставит XS-задача EPIC-003 для Go).
  3. `scripts/lib/llm-endpoint.sh` и `scripts/lib/LlmEndpoint.psm1`: `@` в сыром значении → класс `invalid` с отдельным видом (например, `at-sign`) до классификации хоста, в том числе у `cloud`-хоста. Фраза отказа называет переменную и схему с хвостом маски, как `U19`; хост и значение не называет; тексты sh и ps1 совпадают (P01/P02). Ответ один для `llm_endpoint_classify` (правило 6 `compose-lint`) и `llm_endpoint_resolve` (`llm-server`, `llm-bench`).
  4. Стенд паритета: `T01`/`T02` принимают три новые строки. Сценарии `llm-server up`, `llm-server health` и `llm-bench` с `@` в пути `MV_LLM_URL` — отказ до пробы, `AbsentFold` — голова пароля и хост; без `@` строки прежние. Мутант «`@` в пути принимается» краснеет в каждой половине; контрольный — первым.
  5. Путь, где значение с `@` обходит `llm_endpoint_resolve` (например, адрес из матрицы `llm-bench` без судьи), закрывается отказом, а не маской.
  6. После слияния T-468 и XS-задачи EPIC-003 — строка для system-architect «снять пометку „Код расходится“ в C-15 v1.6 и ADR-005»; `infrastructure.md` §3.1.2 и §6.3.1 (число строк таблицы, сценариев и мутантов) — через system-architect.
  7. Порядок с EPIC-003: T-468 приходит в EPIC-003 не раньше XS-задачи `@` в `IsLocalEndpoint` (или в той же синхронизации), иначе табличный тест Go краснеет. Файлы задачи дополняются `scripts/lib/llm-endpoint.sh`, `scripts/lib/LlmEndpoint.psm1`, `testdata/llm/local-endpoints.tsv`.
- **Метка**: нет. `contracts.md`, ADR и Go-код не меняются.
- **Исполнитель**: devops-engineer (экземпляр назначает оркестратор). Карточка — `tasks/T-468.md`.

### T-464: Devops — `MV_STATE_WORLDS` и `MV_TELEGRAM_*` в docker-compose · Размер: XS–S · Статус: done · Волна 1 (бэклог)
- **Причина (оркестратор, 2026-09-13; приёмка T-461)**: часть (в) T-463 вынесена в отдельную задачу. Отметка владельца T-055 (EPIC-002), О-1: `core` не передаёт `MV_STATE_WORLDS`. Отметка владельца T-310 (EPIC-004), замечание 1: `docker-compose.bot.yml` не передаёт `MV_TELEGRAM_ACTION_KEY_SALT` и `MV_TELEGRAM_COMMANDS_PER_MIN`.
- **Ветка**: `task/T-464-compose-env-passthrough` от `epic/EPIC-001-foundation` (`55ec4c4`), папка `.worktrees/T-464`.
- **Зависит от**: T-055 (EPIC-002) и T-310 (EPIC-004) — объявления в манифесте ветки через `develop`; не позже T-390. С T-468 не пересекается.
- **Файлы**: `docker-compose.yml`, `docker-compose.bot.yml`, `scripts/compose-lint.sh` (правила 3 и 8), `testdata/compose-lint/` (фикстуры, `README.md`).
- **Описание**: подробно — карточка `tasks/T-464.md`. Три переменные передаются в свои сервисы с умолчаниями манифеста: `${MV_STATE_WORLDS:-dark-forest-world}`, `${MV_TELEGRAM_COMMANDS_PER_MIN:-20}`, соль — `${MV_TELEGRAM_ACTION_KEY_SALT:-}` без `:?`. Правило 3 `compose-lint` знает `MV_.*_SALT`: литерал соли раньше проходил все правила.
- **DoD** (полностью — карточка):
  1. Строки в `core` и `telegram-bot`, умолчания равны манифесту ветки.
  2. `make compose-lint` и `--fixtures` — ok.
  3. В модели `docker compose config` с env-файлом пробы значения доходят до своих сервисов, без строки действует умолчание; печатаются только ключи.
  4. `mvctl env check` — exit 0.
  5. `gitleaks dir --redact` по изменённым файлам — no leaks.
  6. dev-log, карточка; `.env` не открывался.
- **Метка**: нет. `.env.example`, `shared/env/**`, `contracts.md` и эталон §4.2 не меняются.
- **Исполнитель**: devops-engineer#1 (TEAM-1, Opus). Ревью — code-reviewer#1. Карточка — `tasks/T-464.md`.
- **(приёмка tech-lead#1, 2026-09-13)** Принята после ревью #1 (0/0/1/2), итераций ревью — 1. DoD 1–6 подтверждены. Все замечания ревью закрыты при приёмке:
  - Mi-1: правило 3 советует необязательным секретам `${KEY:-}`, а не `:?`;
  - N-1: комментарий `MUST_BE_REQUIRED` перечисляет законно пустые секреты, включая соль;
  - N-2: отказы правил 3 и 8 печатают `<withheld>` вместо значения секрета, в фикстурах есть `expect-absent`; правка небольшая, в T-468 не выносилась.

  Фикстуры: 64 bad, 12 good. Мутанты K0 (первым), M1–M7 — KILLED. Три вопроса system-architect (`MV_STATE_WORLDS` и `MV_WORLD_ID`, эталон §4.2, соль в §3.1.1) и слияние с `55ec4c4` описаны в карточке, раздел «Приёмка (tech-lead)».
- **Бэклог (из T-464, 2026-09-13)**: T-469 (раздел ниже).

### T-469: Devops — переменные шлюза и `MV_ANTHROPIC_API_KEY` в compose; проверка «переменная контекста доходит до своего сервиса» · Размер: S · Статус: done · Волна 1 (бэклог)
- **Причина (приёмка T-464, 2026-09-13)**: ревью #1 T-464, рекомендация 1 и п. 3 бэклога. Класс «забыли передать в compose» повторился трижды (T-055, T-310, T-305). Сейчас в compose не попадают четыре `MV_GATEWAY_*` T-305 и `MV_ANTHROPIC_API_KEY`, после слияния T-306 — ещё четыре `MV_GATEWAY_*`. Номер выдан оркестратором.
- **Ветка**: `task/T-469-compose-gateway-vars` от `epic/EPIC-001-foundation` (`105f116`; итерация 2 — после синхронизации до `e9c9ce9`, где есть EPIC-004), папка `.worktrees/T-469`. Прежнее имя `task/T-469-context-env-reach` заменено фактическим при приёмке.
- **Зависит от**: T-464 (слита); T-306 (EPIC-004) — в `develop` и в EPIC-001; решение system-architect по `contracts.md` §16 п. 5 — для частей (б) и (в). Часть (а) можно начать после T-306, не дожидаясь решения. С T-468 не пересекается.
- **Файлы**: `docker-compose.yml`; `scripts/compose-lint.sh`; `testdata/compose-lint/` (фикстуры, `README.md`); по решению system-architect — `contracts.md` §16 п. 5 и `infrastructure.md` §3.1.1 (правит system-architect).
- **Описание**: подробно — карточка `tasks/T-469.md`.
  - (а) `gateway` получает `MV_GATEWAY_RATE_ACTIONS_PER_MIN`, `MV_GATEWAY_RATE_ACTIONS_BURST`, `MV_GATEWAY_INPUT_FILTER`, `MV_GATEWAY_ENCOUNTER_GRACE` (T-305), `MV_GATEWAY_SESSION_IDLE`, `MV_GATEWAY_TURN_TIMEOUT`, `MV_GATEWAY_CHARACTER_WAIT`, `MV_GATEWAY_CHARACTER_DEADLINE` (T-306) и `MV_GATEWAY_DELIVERY_LEASE`, `MV_GATEWAY_DELIVERY_TTL` (T-307) — десять переменных с умолчаниями манифеста. `MV_ANTHROPIC_API_KEY` передаётся сервису контекста `llm` в форме `${VAR:-}`.
  - (б) Машинная проверка `compose-lint`: каждая переменная контекста доходит до сервиса, где контекст поднят, или помечена как переменная только хоста. Это изменение контракта §16 п. 5, через system-architect.
  - (в) Общее правило формы передачи — рекомендация ревьюера `${VAR:-<умолчание манифеста>}`; совет правила 8 приводится к правилу.
- **DoD** (полностью — карточка):
  1. Десять `MV_GATEWAY_*` и `MV_ANTHROPIC_API_KEY` в compose; модель `config` с пробой — значения доходят, печатаются только ключи.
  2. Решение system-architect по §16 п. 5; правило проверки с bad/good-фикстурами; мутант «проверка выключена» краснеет, контрольный — первым.
  3. Всё, что нашла проверка на настоящих файлах (в том числе `MV_GM_PATH` у `gateway`, найдено при приёмке T-464), передано или помечено.
  4. Правило формы записано, совет правила 8 и фикстуры приведены к нему.
  5. `bash -n`, `make compose-lint` и `--fixtures`, `mvctl env check` — зелёные; `gitleaks dir --redact` — no leaks; отказы не печатают значения секретов.
  6. dev-log, карточка; `.env` не открывался.
- **Метка**: `contract-change` — части (б) и (в).
- **Исполнитель**: devops-engineer#1 (TEAM-1, Opus). Ревью — code-reviewer#2. Карточка — `tasks/T-469.md`.
- **(приёмка tech-lead#1, 2026-09-14)** Принята: ревью #1 (0/0/3/2), итераций ревью — 1; итерация 2 проверена приёмкой без ревью #2 (решение оркестратора). DoD 1, 2, 4, 6–8 подтверждены прогонами, DoD 3 и 5 закрыты решением оркестратора: таблица `READERS`/`NOT_IN_CONTAINERS` в скрипте, форма `${MV_X:-умолчание}`.
  - Сверх объёма переданы `MV_GM_PATH` (`gateway`), `MV_OLLAMA_URL` (`core`), `MV_CORE_ADMIN_CLIENTS` (`x-platform-env`). Сужение списка администраторов у `gateway` и `memory` при prod-`.env` без `ci-harness` признано приемлемым (SEC-12, один список на процесс).
  - Mi-1…Mi-3, N-1, N-2 закрыты. `make compose-lint` — 9 правил, фикстуры 72 bad / 14 good; `mvctl env check` — 80, exit 0. Мутанты приёмки K0 (первым), MC, MF — KILLED.
  - Передать (подробно — карточка): строки `infrastructure.md` §3.1.1 и `contracts.md` §16 п. 5 — system-architect после слияния; рецепт встречи с T-471 (`MV_RULES_PATH` в `READERS`, причина `MV_SWARM_FAKE`, две фикстуры); строка runbook и release notes про `MV_CORE_ADMIN_CLIENTS`; `make secrets-scan` после коммита.
- **Бэклог (из T-469, 2026-09-14)**: мёртвая ветка `env_file` правила 4; сверка строк `READERS` по документам с кодом (Go-тест); пометка читателя в манифесте вместо таблицы — system-architect.

### T-476: Devops — `degraded` как работающий процесс в `make health` и документах; e2e бота в `test-race` · Размер: S · Статус: done · Волна 1 (бэклог)
- **Причина (оркестратор, 2026-09-14)**:
  - решение system-architect#1 по T-059 (EPIC-002), «Риск compose: решение (б)» и «Пункт для devops-engineer»: мир `uninitialized` отдаёт `/health degraded`, проба `multiverse health` отвечает 0 на `ok` и `degraded` (правка пробы — в T-059), а `make health`, `infrastructure.md` §2.2, §2.3, §7.2, `README.md` и runbook — эта задача;
  - решение system-architect#2 по T-315 (EPIC-004): e2e бота `./cmd/telegram-bot/e2e/...` (тег `e2e`) — во второй команде `test-race`, не в `RACE_PKGS`; строки `infrastructure.md` §3.
- **Ветка**: `task/T-476-health-degraded-make` от `epic/EPIC-001-foundation` (`e7adb6d`, содержит `develop` с T-469), папка `.worktrees/T-476`.
- **Зависит от (порядок слияния; решение оркестратора, вариант (а), 2026-09-14)**: T-476 сливается в `epic/EPIC-001-foundation` сразу после приёмки. `epic/EPIC-001-foundation` → `develop` не раньше, чем в `develop` есть T-059 (EPIC-002) и T-315 (EPIC-004).
  1. **T-059 (EPIC-002)** — документы (`infrastructure.md` §2.3, §7.2, `README.md`, runbook) описывают пробу T-059: без неё в `develop` они опишут пробу раньше, чем она так работает. Строка `core` в `make health` читает слово пробы и верна по обе стороны T-059, порядка она не требует.
  2. **T-315 (EPIC-004)** — каталог `cmd/telegram-bot/e2e` есть только в `epic/EPIC-004-gateway-bot`. Без него шаблон в `go test` — не предупреждение, а отказ (`[setup failed]`, код 1), и задание CI `race` на `develop` покраснело бы.
  - **Проверка перед слиянием эпика в `develop`:** `git ls-tree develop cmd/telegram-bot/e2e` не пуст и `git log --merges --oneline develop | grep -F 'Merge task/T-059-'` не пуст. Искать коммит слияния задачи, а не номер: `grep T-059` по всему журналу `develop` уже находит служебные коммиты `chore(dev-team)`, хотя T-059 не слита (поправлено приёмкой tech-lead#1, 2026-09-14).
  - **Почему локальный `make ci` этого не покажет:** без cgo `test-race` пишет `SKIPPED` и до `go test` не доходит. Отказ `[setup failed]` проявится только в задании CI `race` после push в `develop`.
- **Файлы**: `Makefile` (цели `health`, `up`, `deploy`, `rollback`, `test-race`), `Docs/dev-team/architecture/infrastructure.md` (§2.2, §2.3, §3.1, §7.2, §8, §9.1), `README.md`, `Docs/ops/runbook.md`. `docker-compose.yml`, `cmd/multiverse`, `.env.example` не меняются.
- **Описание**: подробно — карточка `tasks/T-476.md`.
- **DoD** (полностью — карточка):
  1. `DEGRADED_STRICT ?= 1` рядом с `LLM_STRICT`; `make health` печатает настоящий статус (`ok`/`degraded`/`FAIL`) у `core` (слово пробы) и у `gateway`/`memory`/`telegram-bot` (поле `status` тела); `degraded` даёт `rc=1` только при `DEGRADED_STRICT=1`; `up`, `deploy`, `rollback` передают `DEGRADED_STRICT=0`.
  2. `test-race`: вторая команда и её `echo` содержат `./test/e2e/... ./cmd/telegram-bot/e2e/...` с `-tags e2e`; `RACE_PKGS` не меняется.
  3. `infrastructure.md` §2.2, §2.3, §7.2 — тексты решения по T-059; §2.2/§3.1 — строки `make test-race` и задания `race`.
  4. `README.md` и runbook: после свежего `make up` `core` — `degraded` до `mvctl world init`.
  5. Прогоны: `make help`, `make -n test-race`, `go list -tags e2e ./cmd/telegram-bot/e2e/...`, `make compose-lint`, `make secrets-scan`; контейнеры не поднимаются, `make health` против стека не запускается.
  6. dev-log, карточка; `.env` не открывался.
- **Метка**: нет. `contracts.md` и Go-код не меняются.
- **Исполнитель**: devops-engineer#1 (TEAM-1, Opus). Ревью — code-reviewer#2. Карточка — `tasks/T-476.md`.
- **(приёмка tech-lead#1, 2026-09-14)** Принята. Ревью #1 (0/0/2/2), итераций ревью — 1; итерацию 2 проверила приёмка, без ревью #2 (решение оркестратора). DoD 1–6 подтверждены: `make help`, `make -n test-race`, `make compose-lint` (72/14), `make secrets-scan`, `go build ./... && go vet ./...` — зелёные; `go list -tags e2e ./cmd/telegram-bot/e2e/...` — rc=1, ожидаемо до T-315.
  - Mi-1, Mi-2, N-1, N-2 закрыты. Порядок (вариант (а)) в карточке и индексе — один текст.
  - Правка приёмки: проверка T-059 `git log --oneline develop | grep T-059` уже находила 6 служебных коммитов `chore(dev-team)`, хотя T-059 не слита. Заменена на `git log --merges --oneline develop | grep -F 'Merge task/T-059-'` (0 для T-059; контроль T-057 — 1).
  - Передать: T-476 → `epic/EPIC-001-foundation` сейчас; эпик → `develop` — когда не пусты обе проверки; `make secrets-scan` после коммита, до push.
  - Отметка по файлам EPIC-001 из T-059 (`health.go`, `main_test.go`, `serve.go`, `contexts_state.go`, `contexts_state_recovery_test.go`, `fake_contexts_test.go`, `shared/testkit/state/guard.go`) — карточка T-476, раздел «Отметка по файлам EPIC-001 из T-059». Согласовано предварительно, место стража `OneStateOverTheWorld` в `shared/testkit/state` подтверждено; финальный `serve.go` — по завершении итерации 3 T-059.
- **Бэклог (из T-476, 2026-09-14)**:
  1. system-architect: пример JSON в `infrastructure.md` §7.2 — `ok|degraded|fail` вместо `down`, «503 для `down`» → «503 для `fail`».
  2. devops: `make health` зовёт пробу `core` без `--url` (адрес из `MV_CORE_ADDR` контейнера, как healthcheck compose, T-408).
  3. devops, после T-315 в `develop`: страж «шаблон `RACE_PKGS`/`RACE_E2E_PKGS` без пакетов — отказ» (`go list` с тегом по каждому шаблону).
  4. devops (ревью #1): `$(COMPOSE) ps --services | grep -qx` под `pipefail` может дать 141 и показать поднятый сервис как `-`; заменить на `grep -x … >/dev/null`.
  5. devops (ревью #1): у `docker compose exec -T core /multiverse health` нет своего тайм-аута; решать вместе с п. 2.
  6. system-architect (приёмка): `infrastructure.md` §2.2, строка `make health` — среди причин кода ≠ 0 нет недоступного LLM при `LLM_STRICT=1`; вместе с п. 1.

### T-482: CI на `develop` зелёный на Linux — тесты, зависящие от брокера, прав файлов, порядка подписок и SIGPIPE · Размер: M · Статус: done · Волна 1 (бэклог, срочно)
- **Причина (оркестратор, 2026-09-14)**: workflow `go` красный на `develop` на каждом push с 2026-09-13 11:25. Прогоны: O — 34823893793, P — 34826902656, Q — 34827615499. Локально на Windows `make ci` зелёный. Номер выдан оркестратором.
- **Ветка**: `task/T-482-ci-green-on-linux` от `epic/EPIC-001-foundation` (`a3defd6`), папка `.worktrees/T-482`.
- **Связи**: T-483 (EPIC-004) — десятое падение, `internal/gateway/consumer` `TestConsumerOnRedpanda/OutboxWhileTheBrokerIsAway` (`integration`, прогон 34827849541). Вынесено решением оркестратора по Ma-1 ревью #1. Остальные связи: T-414, T-313/T-318, T-433/T-419, T-463, T-405, T-315.
- **Файлы** (владельцы):
  - EPIC-001 — `cmd/multiverse/dispatch_test.go`, `scripts/backup-prune-test.sh`, `testdata/script-parity/stand.go`;
  - EPIC-004 — `internal/gateway/store/open_test.go`, `internal/gateway/gatewaytest/gatewaytest_test.go`, `internal/gateway/snapshot_test.go`, `cmd/telegram-bot/internal/deliver/pace_test.go`, `shared/testkit/gateway/stand_test.go`;
  - EPIC-003 — `shared/testkit/swarm/fake_encounter.go` (`fold`), `fake_encounter_test.go`.
- **Описание**: подробно — карточка `tasks/T-482.md`. Девять причин:
  1. `--bus=kafka` в `dispatch_test` требовал брокер;
  2. `links.db` создавался с umask 0644;
  3. стенд сравнивал Harness с `FakeState.Get` раньше `store.Put`;
  4. SIGPIPE `grep | head` под `pipefail` обрывал прогон мутантов;
  5. а — `t.TempDir` 0755; б — гонка на `bytes.Buffer` лога; в — курсор эффектов отставал до `Stop`; г — гонка за порт двойника в процессе.

  Попутно: `FakeEncounter.fold` откатывал view поздним фактом.
- **DoD** (полностью — карточка):
  1. Тест группы 1 не зависит от брокера.
  2. Файл готовится с 0600, код `store` не ослаблен.
  3. Стенд ждёт State по условию; `-count=50 -cpu=1,2` без падений.
  4. `make backup-prune-test` и `make backup-prune-mutants` зелёные.
  5. Падения 5а–5г разобраны, правки перечислены для владельцев.
  6. build, vet (без тега, `e2e`, `integration`), `go test -short`, `-tags e2e`, `golangci-lint`, `make test` зелёные.
  7. Карточка и `dev-log.md`.
- **Метка**: нет. Production-код не менялся. Правка кода — только двойник `shared/testkit/swarm/fake_encounter.go` (EPIC-003), её подтвердил tech-lead#2.
- **Исполнитель**: developer#2 (TEAM-1, Opus). Ревью — code-reviewer#2. Отметки владельцев EPIC-003 и EPIC-004 — tech-lead#2. Карточка — `tasks/T-482.md`.
- **(приёмка tech-lead#1, 2026-09-14)** Принята. Ревью #1 (0/1/2/1), итераций ревью — 1; итерацию 2 проверила приёмка без ревью #2 (решение оркестратора). DoD 1–7, суженный до девяти причин, подтверждён прогонами.
  - Ma-1 закрыт переносом в T-483. Mi-1 (Harness читается до ожидания State), Mi-2 (`awk` вместо конвейера, проверено под `pipefail`), N-1 (`freePort` через `listenFree`) закрыты.
  - Прогоны: build и vet с тремя наборами тегов, `go test -short` и `-tags e2e` (59 и 61 пакет) — зелёные; `golangci-lint` — 0 issues; стенд `-count=50 -cpu=1,2` — ok; `make backup-prune-test` и `make backup-prune-mutants` (17 KILLED, P01 GREEN, P10 SKIPPED) — rc=0; `make scripts-parity "PARITY_ARGS=-jobs 1"` после N-1 — 113/0/2, порты 36/36, testcontainers во время прогона не было; `make test` с порогом покрытия — зелёный.
  - Оговорка: «Access is denied» у `env`, `updates`, `render` — блокировка Windows; эти пакеты прогнаны через `go test -c`.
  - Отметка владельца EPIC-001 по трём файлам — в карточке.
  - **Передать** (подробно — карточка):
    1. T-482 → `epic/EPIC-001-foundation`, затем контрольное слияние `epic/EPIC-001-foundation` → `develop` и push — первый push, который покажет CI на Linux. Условия порядка T-476 выполнены (`cmd/telegram-bot/e2e` в `develop`, слияние T-059 — `77d06ff`).
    2. **`integration` останется красным до T-483.** Зелёными должны стать `unit` (с «Coverage floor»), `e2e`, `race`, `scripts-parity`, `contracts`, `security`, `compose-lint`.
    3. Группы 2, 3, 5а–5г подтвердит только CI на Linux.
    4. `make secrets-scan` после коммита, до push.
- **Бэклог (из T-482, 2026-09-14)**:
  1. devops (EPIC-001): `head` под `pipefail` в `scripts/llm-bench.sh:712,715,741` и `scripts/llm-server.sh:330,408`. Пока не падали: вход короткий. Заменить на `sed -n 1p`, `awk` или here-string без конвейера, как в T-482.
  2. devops (EPIC-001): гонка портов у двойников-программ `script-parity`. `sc.Program` получает номер через `freePort` и слушает его сам, окно гонки остаётся. Варианты: повтор шага `up` при `address already in use` или передача слушателя дочернему процессу.
  3. devops (EPIC-001), ревью #1 п. 3: независимые шаги CI для `backup-prune-mutants` и `scripts-parity`. **Закрыто при приёмке, задача не нужна**: у обоих шагов `backup-prune` задания `scripts-parity` уже стоит `if: ${{ !cancelled() }}` (T-463, Mi-5). Раздельные задания — только если понадобится отдельный статус в защите ветки.
  4. tech-writer, сверка devops-engineer (EPIC-001): запись в runbook разработчика «локальный `make test` без cgo не ловит гонки (ОВ-5)». Падения 5б, 5в и группы 3 под `-race` видны только в CI. Совпадает с п. 2 бэклога T-454 — вести одной задачей.
  5. EPIC-003 → system-architect, при нарезке рантайма встречи `internal/swarm`:
     - правило «факт новее представления»: атрибуты факта применяются, только если его версия новее представления мира агента, — в дизайн `internal/swarm`;
     - молчание после `drop` («world moved past the decision»): действие не получает ответа, и Harness ждёт дедлайн. Как агент сообщает шлюзу о сдавшемся действии (C-05 п. 1) — решает system-architect.
  6. devops (EPIC-001), приёмка: «Access is denied» при запуске тестового exe на Windows бывает не только у `cmd/telegram-bot/internal/updates` (T-462), но и у `cmd/mvctl/internal/env` и `cmd/telegram-bot/internal/render`. Из-за этого `make test` локально останавливается до порога покрытия. Расширить T-462 или сделать общий обход.
