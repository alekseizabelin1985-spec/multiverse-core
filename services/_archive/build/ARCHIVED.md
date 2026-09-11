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

## `docker-compose.as-is.yml` (добавлено в T-004)

| Поле | Значение |
|---|---|
| Исходный путь | `docker-compose.yml` (корень) |
| Причина | ядро compose переписано под целевую топологию `gateway`/`core` + инфраструктура (`infrastructure.md` v0.3 §1.2–§1.4, задача T-004/F-6a). As-is файл описывал 15 сервисов, ссылался на перенесённые в архив `Dockerfile` и на образы без пинов (`latest`), публиковал порты на все интерфейсы |
| Коммит архивации | ветка `epic/EPIC-001-foundation`, задача T-004 (база `b7df900`; хэш переноса проставляется при коммите задачи) |
| Последний рабочий коммит | `e6103e4` |
| Эпик возврата | нет; **источник для профиля `legacy`** — секции `narrative-orchestrator`, `semantic-memory`, `chromadb` и их as-is переменные переносит T-008 (F-6b, `infrastructure.md` §1.3, D-3) |
| Что использовало | `make up`/`make run SERVICE=` as-is, `docker compose up` из корня |

Перенос сделан `git mv`, но путь `docker-compose.yml` тут же занят новым ядром, поэтому git видит
не переименование, а копию: `--follow` историю не подхватит. As-is содержимое читается двумя
способами — этим файлом и `git show b7df900:docker-compose.yml` (`git log -- docker-compose.yml` до
коммита T-004). U-1 соблюдён: ничего не удалено.
