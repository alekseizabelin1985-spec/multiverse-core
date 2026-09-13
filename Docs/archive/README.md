# `Docs/archive/` — архив документации

Каталог создан в EPIC-001 F-3 (задача T-002) под исторические документы as-is: они **не удаляются**
(U-1 / OQ-A-17), а переносятся сюда `git mv`, когда теряют актуальность.

Основание: `architecture/components/foundation.md` v0.2 §11 (последняя строка таблицы),
`architecture/infrastructure.md` v0.3 §10 (F-3), `plan/ownership.md` §1 (`Docs/archive/**` — tech-writer).

## Правила

- Владелец каталога — **tech-writer**; перенос выполняется по факту нового кода (T-019/F-9, затем T-398).
- Перенос — только `git mv` (история `git log --follow` сохраняется), содержимое не правится.
- Каждый перенесённый документ получает строку в таблице ниже: исходный путь, причина, чем заменён.
- Актуальная документация проекта живёт в `Docs/dev-team/**` (артефакты команды) и в `Docs/**`
  вне `archive/` (пользовательская и эксплуатационная документация, например
  `Docs/ops/runbook.md`); строчного `docs/` в корне нет (T-398).

## Перенесённые документы

| Исходный путь | Причина | Чем заменён | Задача |
|---|---|---|---|
| `docs/LIVING_WORLDS_ARCHITECTURE.md` | Дизайн отдельной, впоследствии отклонённой архитектуры «автономных Entity-Actor» (нейросетевые веса как состояние, Go 1.22, Redis, ChromaDB) — не пересекается с ADR-001 (модульный монолит) и текущим кодом | `Docs/dev-team/architecture/overview.md`, `adr/ADR-001-modular-monolith-topology.md` и соседние ADR | T-398 |
| `docs/LIVING_WORLDS_FEATURE_CHECKLIST.md` | Чек-лист приёмки той же отклонённой архитектуры Entity-Actor | тот же ADR-набор; актуальные критерии готовности — `epics/EPIC-001-foundation/tasks.md`, `foundation.md` §12 | T-398 |
| `docs/LIVING_WORLDS_INTEGRATION_GUIDE.md` | Инструкция интеграции отклонённой архитектуры Entity-Actor с сервисами, которых в текущей раскладке нет (`entity-actor` заморожен) | `Docs/dev-team/architecture/**`, `services/entity-actor/FROZEN.md` | T-398 |
| `docs/LIVING_WORLDS_QUICK_START.md` | Быстрый старт для отклонённой архитектуры (Go 1.22, Redis, Kafka вручную) | `README.md` «Запуск за 5 команд» | T-398 |
| `docs/LIVING_WORLDS_SUMMARY.md` | Резюме и бизнес-метрики отклонённой архитектуры Entity-Actor | `Docs/dev-team/architecture/overview.md` | T-398 |
| `docs/FEATURES_VERIFICATION.md` | Матрица верификации требований к той же отклонённой архитектуре | `Docs/dev-team/requirements/**`, `architecture/**` | T-398 |
| `README_LIVING_WORLDS.md` | Корневой README ветки с описанием отклонённой архитектуры Entity-Actor, статус «production ready» не подтверждён кодом (`requirements/inventory.md` INV-23) | корневой `README.md` | T-398 |
| `QWEN.md` | Прежняя картина проекта (Go 1.24, 12+ отдельных сервисов, ChromaDB как основная БД, TimescaleDB/Redis) — не соответствует текущей раскладке единого модуля (ADR-001) | `CLAUDE.md`, `AGENTS.md`, `README.md` | T-398 |
| `AI_AGENT_INSTRUCTIONS.md` | Инструкция режима работы одного ассистента-соавтора («CONSULT/RESEARCH/ARCHITECTURE/…», стек Java/Python) — не соответствует ни стеку (Go), ни процессу команды из ролей (`.dev-team.json`) | `CLAUDE.md`, `.dev-team.json`, роли команды (`Docs/dev-team/plan/teams.md`) | T-398 |
| `AUTOMATION-SETUP.md` | Список subagent'ов/skills/MCP-серверов на момент установки; состав разошёлся с фактическим (`.claude/agents/*`, `.claude/skills/*`, `.mcp.json`) и не поддерживался при их изменении | сами каталоги `.claude/agents/**`, `.claude/skills/**`, файл `.mcp.json` (самоописание, отдельный документ не требуется) | T-398 |
| `Docs/LIVING_WORLDS_IMPLEMENTATION_STATUS.md` | Отчёт о готовности той же отклонённой архитектуры Entity-Actor («Core Implementation Complete ✅ ~85%»), статус не подтверждён кодом — `requirements/inventory.md` INV-23, `project/open-questions.md` | `Docs/dev-team/architecture/overview.md` | T-398 |
| `shared/agent/README.md` → `Docs/archive/shared/agent/README.md` | Описание as-is Agent GM Core (router, lifecycle, pipeline, worker pool, блупринт v1, импорт `multiverse-core/shared/agent`); код перенесён в `services/_archive/shared/agent/`, путь сохранён, чтобы не столкнуться с этим `README.md` | `Docs/dev-team/architecture/components/swarm-llm-laws.md` §2, §13; ADR-015; код `shared/agent/{types,blueprint,parser,placeholders}.go` | T-201 |
| `shared/agent/MIGRATION.md` → `Docs/archive/shared/agent/MIGRATION.md` | Инструкция миграции на тот же as-is Agent GM Core (формат блупринта v1) — не соответствует формату v2 (C-11) | `swarm-llm-laws.md` §13, §17 (миграция от narrative-orchestrator); ADR-015 | T-201 |

## Кандидаты на перенос (решение будущей задачей, здесь только список к рассмотрению)

Список составлен по `foundation.md` §11 и не является решением; в T-398 не входит:

- `Docs/architecture.md`, `Docs/architecture_old.md`, `Docs/architecture-updated.md` — три поколения
  описания архитектуры as-is; истина после G2 — `Docs/dev-team/architecture/**`;
- `Docs/EVENTS-MIGRATION.md` — документ предыдущей итерации продукта;
- `PULL_REQUEST.md` — корневой документ as-is;
- `memory/**`, `plans/**`, `reports/**` — рабочие заметки прошлых сессий (по U-11 из индекса
  не выводятся; решение по ним — отдельная задача).

`CLAUDE.md`, `AGENTS.md`, `README.md` не архивируются — они переписываются в F-9.
