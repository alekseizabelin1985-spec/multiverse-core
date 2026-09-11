# ARCHIVED — `configs/gm_*.yaml`

Каталог выведен из сборки в EPIC-001 F-3 (задача T-002) и **не удалён** (решение владельца U-1 /
OQ-A-17). Код читается, но не собирается, не линтуется и не входит ни в один профиль compose.

| Поле | Значение |
|---|---|
| Исходный путь | `configs/gm_*.yaml` |
| Причина | профили GM в YAML заменяются блупринтами и законами мира (`foundation.md` §11, `overview.md` §16) |
| Коммит архивации | ветка `epic/EPIC-001-foundation`, задача T-002 (база `04a3a15`; хэш коммита переноса проставляется при коммите задачи) |
| Последний рабочий коммит | `9ffca2d` |
| Эпик возврата | нет |
| Что использовало | `.github/workflows/validate-blueprints.yml` (удаляется в T-012), `shared/agent/MIGRATION.md` (документация) |

Возврат кода в сборку — только через архитектора (`plan/ownership.md` §1, строка `services/_archive/**`).
Основание: `architecture/infrastructure.md` v0.3 §4.6, `architecture/components/foundation.md` v0.2 §11,
`architecture/overview.md` §16, ADR-001 доп. п. 5.
