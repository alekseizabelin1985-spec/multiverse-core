# ARCHIVED — `shared/oracle`

Каталог выведен из сборки в EPIC-001 F-3 (задача T-002) и **не удалён** (решение владельца U-1 /
OQ-A-17). Код читается, но не собирается, не линтуется и не входит ни в один профиль compose.

| Поле | Значение |
|---|---|
| Исходный путь | `shared/oracle` |
| Причина | HTTP-клиент внешних «оракулов» заменяется `internal/llm` (провайдеры LLM, ADR-005 доп. 2). В `README.md` ключ заменён плейсхолдером до переноса (T-001, U-5) |
| Коммит архивации | ветка `epic/EPIC-001-foundation`, задача T-002 (база `04a3a15`; хэш коммита переноса проставляется при коммите задачи) |
| Последний рабочий коммит | `04a3a15` |
| Эпик возврата | нет |
| Что использовало | `services/narrative-orchestrator`, `services/universe-genesis-oracle`, `services/world-generator` |

Возврат кода в сборку — только через архитектора (`plan/ownership.md` §1, строка `services/_archive/**`).
Основание: `architecture/infrastructure.md` v0.3 §4.6, `architecture/components/foundation.md` v0.2 §11,
`architecture/overview.md` §16, ADR-001 доп. п. 5.
