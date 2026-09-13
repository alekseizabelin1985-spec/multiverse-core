# ARCHIVED — `shared/agent/{filter.go,e2e_dark_forest_test.go}`

Файлы выведены из сборки в EPIC-001 F-2 (задача T-003, решение оркестратора по ревью T-002:
M-4 / ОВ-1, вариант «а») и **не удалены** (решение владельца U-1 / OQ-A-17). Код читается,
но не собирается, не линтуется и не входит ни в один профиль compose.

Остальная часть `shared/agent` (типы, парсер и валидатор блупринтов) в архив **не уходит** —
она переносится в единый модуль `multiverse-core.io` в T-003 (`foundation.md` §11).

| Поле | Значение |
|---|---|
| Исходный путь | `shared/agent/filter.go`, `shared/agent/e2e_dark_forest_test.go` |
| Причина | оба файла зависят от заархивированного в T-002 `shared/rules`; в единый модуль по `foundation.md` §11 переносятся только типы, парсер и валидатор блупринтов — фильтр к ним не относится. Логика фильтра реализуется заново в EPIC-003 (`internal/llm/filter`, ADR-001 п. 2 раскладки) |
| Коммит архивации | ветка `epic/EPIC-001-foundation`, задача T-003 (база `5a20bb8`; хэш коммита переноса проставляется при коммите задачи) |
| Последний рабочий коммит | `e6103e4` |
| Эпик возврата | нет (фильтр переписывает EPIC-003 — `internal/llm/filter`) |
| Что использовало | `shared/agent` (внутрипакетный код `filter.go`; e2e-тест — `shared/agent`, `shared/agent/tools`, `shared/rules`); внешних импортёров нет (проверено `grep` по репозиторию) |

Возврат кода в сборку — только через архитектора (`plan/ownership.md` §1, строка `services/_archive/**`).
Основание: `architecture/components/foundation.md` v0.2 §11, `architecture/infrastructure.md` v0.3 §4.6,
`architecture/overview.md` §16, ADR-001 доп. п. 5, ревью T-002 (M-4).

---

# ARCHIVED — as-is Agent GM Core `shared/agent` (T-201, EPIC-003)

Файлы выведены из пакета `shared/agent` задачей **T-201** (EPIC-003, подволна A, ветка
`task/T-201-agent-blueprint-v2`, база `cd21a48`) и **не удалены** (U-1 / OQ-A-17): перенос `git mv`,
история читается через `git log --follow`. В пакете остались `types.go` (типы уровней, LOD и
жизненного цикла перенесены из `agent_types.go` без изменений), `blueprint.go`, `parser.go`,
`placeholders.go` (формат блупринта v2, C-11, ADR-015), `lod.go` и `tools/registry.go`
(время — через `shared/clock`).

| Поле | Значение |
|---|---|
| Исходный путь | `shared/agent/{agent_types.go, md_parser.go, blueprint_loader.go, blueprint_validator.go, blueprint_validator_test.go, helpers.go, worker_pool.go, state_manager.go, router.go, lifecycle.go, pipeline.go, interfaces.go, agent_test.go}`, `shared/agent/examples/domain-dark-forest.md` |
| Причина | неподключённая вторая архитектура GM (router, lifecycle, pipeline, worker pool, state manager) и формат блупринта v1 (`md_parser.go`, `blueprint_loader.go`, `blueprint_validator.go`); заменяются рантаймом роя `internal/swarm` (EPIC-003 I1b) и форматом v2 (`parser.go` T-201, `validator.go` T-202). `interfaces.go` перенесён целиком: все его интерфейсы (`Agent`, `Event`, `Lifecycle`, `Worker`, `BlueprintParser`, …) относятся к as-is рантайму и в целевой раскладке (`swarm-llm-laws.md` §2) не используются. `examples/domain-dark-forest.md` — чистый YAML v1 с `qwen:7b`, заменяется `blueprints/domain-dark-forest.md` (T-203) |
| Коммит архивации | ветка `task/T-201-agent-blueprint-v2` (база `cd21a48`; хэш проставляется при коммите задачи) |
| Последний рабочий коммит | `b7df900` (`agent_types.go`, `md_parser.go`, `blueprint_loader.go`, `blueprint_validator_test.go`, `helpers.go`, `worker_pool.go`, `state_manager.go`, `router.go`, `pipeline.go`, `agent_test.go`); `e6103e4` (`blueprint_validator.go`, `lifecycle.go`, `interfaces.go`); `81aaf98` (`examples/domain-dark-forest.md`) |
| Эпик возврата | нет |
| Что использовало | внутри корневого модуля — только сам пакет и его тесты (проверено `grep` по `cmd/`, `internal/`, `shared/`); вне модуля — as-is `services/narrative-orchestrator` (профиль `legacy`), который собирается не из рабочего дерева, а из `LEGACY_SRC_REF` (цель `legacy-src` Makefile), и заархивированные ранее `tools/adapter.go`, `e2e_dark_forest_test.go` |

`README.md` и `MIGRATION.md` пакета перенесены в `Docs/archive/shared/agent/` (строка в `Docs/archive/README.md`).
