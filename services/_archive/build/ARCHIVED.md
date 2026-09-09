# ARCHIVED — `Dockerfile (корневой) и services/{entity-actor,evolution-watcher,rule-engine}/Dockerfile`

Каталог выведен из сборки в EPIC-001 F-3 (задача T-002) и **не удалён** (решение владельца U-1 /
OQ-A-17). Код читается, но не собирается, не линтуется и не входит ни в один профиль compose.

| Поле | Значение |
|---|---|
| Исходный путь | `Dockerfile (корневой) и services/{entity-actor,evolution-watcher,rule-engine}/Dockerfile` |
| Причина | дубли сборочных файлов: единый образ платформы описывает `build/Dockerfile` (`infrastructure.md` §2.3, F-2/T-003) |
| Коммит архивации | ветка `epic/EPIC-001-foundation`, задача T-002 (база `04a3a15`; хэш коммита переноса проставляется при коммите задачи) |
| Последний рабочий коммит | `a40d9f4 (корневой), e13e20c (три сервисных)` |
| Эпик возврата | нет |
| Что использовало | `docker build` из корня и `make build-service` as-is |

Возврат кода в сборку — только через архитектора (`plan/ownership.md` §1, строка `services/_archive/**`).
Основание: `architecture/infrastructure.md` v0.3 §4.6, `architecture/components/foundation.md` v0.2 §11,
`architecture/overview.md` §16, ADR-001 доп. п. 5.
