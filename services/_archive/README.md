# `services/_archive/` — архив выведенного из сборки кода

Индекс архива EPIC-001 F-3 (задача T-002). **Ничего не удалено** — решение владельца U-1 / OQ-A-17:
код перенесён `git mv`, история читается через `git log --follow`.

Основание: `Docs/dev-team/architecture/infrastructure.md` v0.3 §4.6, `architecture/components/foundation.md`
v0.2 §11, `architecture/overview.md` §16, `plan/ownership.md` §1 (строка `services/_archive/**`),
ADR-001 доп. п. 5.

## Правила

- Каталог **вне сборки**: собственный `go.mod` (`multiverse-core.io/archive`, без `require`), поэтому
  `go build ./...` корня архив не видит. Исключение архива проверено фактом (T-003, T-012): в
  `.golangci.yml` — строки 217 и 274 (`services/_archive`, `services/`); в `.dockerignore` —
  строка 46 (`services/`); в `.github/CODEOWNERS` — строка 40 (`/services/_archive/`, комментарий
  строка 17). Ни один рецепт `Makefile` и ни один сервис ни одного из трёх compose-файлов
  (`docker-compose.yml`, `docker-compose.bot.yml`, `docker-compose.legacy.yml`) архив не
  собирает, поэтому явное исключение им не требовалось. Оговорка «не собирают
  `services/*`» верна только для `docker-compose.yml` и `docker-compose.bot.yml`:
  профиль `legacy` собирает `services/narrative-orchestrator` и
  `services/semantic-memory`, но не из рабочего дерева — цель `legacy-src`
  выкладывает их из `LEGACY_SRC_REF` в отдельный каталог сборки, и архив при этом
  всё равно не участвует. `docker-compose.legacy.yml:6,33` ссылается на
  `services/_archive/build/docker-compose.as-is.yml` и `services/_archive/shared/` только в
  комментариях (документирует решение T-008), а не как на зависимость сборки.
  Из `gitleaks` архив — **не** исключён.
- Возврат отдельного пакета в сборку — `git mv` обратно; подкаталоги без собственного `go.mod`
  (`shared/{schema,redis,rules,intent,tinyml}`) через `replace` не подключаются (ревью T-002, M-2).
- Содержимое **не правится**: автофиксеры `pre-commit` отключены пофайлово на `^services/_archive/`
  (`.pre-commit-config.yaml`), чтобы `git log --follow` оставался чистым.
- Путь внутри архива повторяет исходный путь от корня репозитория.
- Возврат кода в сборку — **только через архитектора**.
- В каждом каталоге — `ARCHIVED.md` с причиной, коммитами и списком того, что этот код использовало.

## Что перенесено

Коммит архивации для всех строк, кроме двух файлов `shared/agent`, один: задача **T-002**, ветка
`epic/EPIC-001-foundation` (база `04a3a15`; хэш проставляется при коммите задачи). `shared/agent/filter.go`
и `shared/agent/e2e_dark_forest_test.go` перенесены задачей **T-003** (база `5a20bb8`) — см. отдельную
строку и `services/_archive/shared/agent/ARCHIVED.md`.

| Исходный путь | Путь в архиве | Причина | Последний рабочий коммит | Эпик возврата |
|---|---|---|---|---|
| `services/ban-of-world` | `services/_archive/services/ban-of-world` | роль закрывают законы мира и `internal/laws` | `706d585` | нет |
| `services/reality-monitor` | `services/_archive/services/reality-monitor` | метрики MVP-1 переопределены (`infrastructure.md` §7.3) | `d0fa707` | нет |
| `shared/schema` | `services/_archive/shared/schema` | заменяется `shared/contracts` (JSON Schema 2020-12) | `f89a13b` | нет |
| `shared/redis` | `services/_archive/shared/redis` | Redis вне стека MVP-1 | `f89a13b` | нет |
| `shared/config` | `services/_archive/shared/config` | конфигурация → env (`shared/env`) и блупринты | `e6103e4` | нет |
| `shared/minio` | `services/_archive/shared/minio` | две реализации → единый `shared/objstore` (ADR-021) | `e6103e4` | нет |
| `shared/oracle` | `services/_archive/shared/oracle` | → `internal/llm` (провайдеры LLM, ADR-005 доп. 2) | `04a3a15` | нет |
| `shared/rules` | `services/_archive/shared/rules` | → `internal/mechanics` + `rules/dark-forest.yaml` (ADR-012) | `a40d9f4` | нет |
| `shared/intent` | `services/_archive/shared/intent` | намерение приходит из gateway/роя | `002eece` | нет |
| `shared/tinyml` | `services/_archive/shared/tinyml` | локальные ONNX-модели вне стека MVP-1 | `a40d9f4` | нет |
| `shared/spatial` | `services/_archive/shared/spatial` | → позиция и scope в `shared/entity` v2 | `e6103e4` | нет |
| `shared/agent/filter.go` | `services/_archive/shared/agent/filter.go` | зависит от заархивированного `shared/rules`; в модуль по §11 идут только типы/парсер/валидатор блупринтов, фильтр переписывает EPIC-003 (`internal/llm/filter`) — **перенесён в T-003** (M-4/ОВ-1) | `e6103e4` | нет |
| `shared/agent/e2e_dark_forest_test.go` | `services/_archive/shared/agent/e2e_dark_forest_test.go` | e2e-тест as-is поверх `shared/rules` и заархивированных `agent/tools/*`; сценарий «Тёмный лес» пересобирается в F-10/EPIC-003 — **перенесён в T-003** (M-4/ОВ-1) | `e6103e4` | нет |
| `shared/agent/tools/adapter.go` | `services/_archive/shared/agent/tools/adapter.go` | инструмент агента as-is (в исходном каталоге остаётся только `registry.go`) | `e6103e4` | нет |
| `shared/agent/tools/entity_tool.go` | `services/_archive/shared/agent/tools/entity_tool.go` | инструмент агента as-is | `e6103e4` | нет |
| `shared/agent/tools/narrative_tool.go` | `services/_archive/shared/agent/tools/narrative_tool.go` | инструмент агента as-is | `e6103e4` | нет |
| `shared/agent/tools/world_tool.go` | `services/_archive/shared/agent/tools/world_tool.go` | инструмент агента as-is | `e6103e4` | нет |
| `fake_deps/` | `services/_archive/fake_deps/` | заглушки onnxruntime/tokenizers для `shared/tinyml` | `9ffca2d` | нет |
| `test_minio.go` | `services/_archive/test_minio.go` | черновой скрипт проверки MinIO в корне модуля; проверку заменяет `mvctl storage init` (T-010) | `3f32340` | нет |
| `Dockerfile` | `services/_archive/build/Dockerfile.root` | дубль сборки; единый образ — `build/Dockerfile` (`infrastructure.md` §2.3) | `a40d9f4` | нет |
| `services/entity-actor/Dockerfile` | `services/_archive/build/services/entity-actor/Dockerfile` | дубль сборки сервиса | `e13e20c` | нет |
| `services/evolution-watcher/Dockerfile` | `services/_archive/build/services/evolution-watcher/Dockerfile` | дубль сборки сервиса | `e13e20c` | нет |
| `services/rule-engine/Dockerfile` | `services/_archive/build/services/rule-engine/Dockerfile` | дубль сборки сервиса | `e13e20c` | нет |
| `configs/gm_defaults.yaml` | `services/_archive/configs/gm_defaults.yaml` | профиль GM as-is → блупринты и законы мира (`overview.md` §16) | `9ffca2d` | нет |
| `configs/gm_group.yaml` | `services/_archive/configs/gm_group.yaml` | профиль GM as-is → блупринты и законы мира (`overview.md` §16) | `9ffca2d` | нет |
| `configs/gm_location.yaml` | `services/_archive/configs/gm_location.yaml` | профиль GM as-is → блупринты и законы мира (`overview.md` §16) | `9ffca2d` | нет |
| `configs/gm_player.yaml` | `services/_archive/configs/gm_player.yaml` | профиль GM as-is → блупринты и законы мира (`overview.md` §16) | `9ffca2d` | нет |
| `configs/gm_region.yaml` | `services/_archive/configs/gm_region.yaml` | профиль GM as-is → блупринты и законы мира (`overview.md` §16) | `9ffca2d` | нет |
| `configs/gm_world.yaml` | `services/_archive/configs/gm_world.yaml` | профиль GM as-is → блупринты и законы мира (`overview.md` §16) | `9ffca2d` | нет |

## Что в архив **не** уходит в волне 0

| Путь | Почему остаётся | Далее |
|---|---|---|
| `services/narrative-orchestrator`, `services/semantic-memory` | нужны профилю `legacy` (`:8083` + ChromaDB, D-3) | → архив после S5 (EPIC-003 I2) |
| `services/entity-manager`, `services/rule-engine` | источники для переписывания | → архив в EPIC-002 (T-064) |
| `services/game-service` | источник для переписывания gateway | → архив после EPIC-004 |
| 8 замороженных сервисов (`FROZEN.md`) | заморозка, не архив: `world-generator`, `universe-genesis-oracle`, `ontological-archivist`, `cultivation-module`, `plan-manager`, `city-governor`, `entity-actor`, `evolution-watcher` | EPIC-006…010 |
| `shared/eventbus`, `shared/jsonpath`, `shared/entity`, `shared/agent` (кроме `tools/*`, `filter.go`, `e2e_dark_forest_test.go`) | переносятся в единый модуль в T-003 (F-2) | — |
| `shared/agent/tools/registry.go` | реестр инструментов остаётся | EPIC-003 |

## Смотрите также

Таблица выше — источник истины по каждому перенесённому пути. Сводная таблица
статусов (активен / источник переписывания / legacy / заморожен / архив) для всего
`services/` — в корневых [`README.md`](../../README.md#статус-кода-вне-единого-модуля),
[`CLAUDE.md`](../../CLAUDE.md#статус-кода-в-services) и [`AGENTS.md`](../../AGENTS.md#статус-кода-в-services)
(T-019, F-9); при добавлении новой строки в архив в волне 1+ обновляются обе стороны
одним PR, чтобы таблицы не расходились.
