# ARCHIVED — `shared/agent/tools/{adapter,entity_tool,narrative_tool,world_tool}.go`

Каталог выведен из сборки в EPIC-001 F-3 (задача T-002) и **не удалён** (решение владельца U-1 /
OQ-A-17). Код читается, но не собирается, не линтуется и не входит ни в один профиль compose.

| Поле | Значение |
|---|---|
| Исходный путь | `shared/agent/tools/{adapter,entity_tool,narrative_tool,world_tool}.go` |
| Причина | инструменты агента as-is; в `shared/agent/tools` остаётся только реестр инструментов (`registry.go`) — `foundation.md` §11 |
| Коммит архивации | ветка `epic/EPIC-001-foundation`, задача T-002 (база `04a3a15`; хэш коммита переноса проставляется при коммите задачи) |
| Последний рабочий коммит | `e6103e4` |
| Эпик возврата | нет (рантайм агента переписывает EPIC-003) |
| Что использовало | `shared/agent` (e2e-тест), `services/narrative-orchestrator/narrativeorchestrator/narrative_agent.go` |

Возврат кода в сборку — только через архитектора (`plan/ownership.md` §1, строка `services/_archive/**`).
Основание: `architecture/infrastructure.md` v0.3 §4.6, `architecture/components/foundation.md` v0.2 §11,
`architecture/overview.md` §16, ADR-001 доп. п. 5.
