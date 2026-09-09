# ARCHIVED — `shared/minio`

Каталог выведен из сборки в EPIC-001 F-3 (задача T-002) и **не удалён** (решение владельца U-1 /
OQ-A-17). Код читается, но не собирается, не линтуется и не входит ни в один профиль compose.

| Поле | Значение |
|---|---|
| Исходный путь | `shared/minio` |
| Причина | две параллельные реализации клиента заменяются единым `shared/objstore` (`foundation.md` §7, ADR-021) |
| Коммит архивации | ветка `epic/EPIC-001-foundation`, задача T-002 (база `04a3a15`; хэш коммита переноса проставляется при коммите задачи) |
| Последний рабочий коммит | `e6103e4` |
| Эпик возврата | нет |
| Что использовало | `services/entity-actor`, `services/evolution-watcher`, `services/rule-engine`, `services/narrative-orchestrator` |

Возврат кода в сборку — только через архитектора (`plan/ownership.md` §1, строка `services/_archive/**`).
Основание: `architecture/infrastructure.md` v0.3 §4.6, `architecture/components/foundation.md` v0.2 §11,
`architecture/overview.md` §16, ADR-001 доп. п. 5.
