# ADR-004: Хранилища — MinIO для состояния и снапшотов, SQLite для приватных данных gateway, Qdrant + Neo4j для памяти; без TimescaleDB, Redis и ChromaDB

Статус: предложено (к утверждению на G2) · Дата: 2026-09-09 · Автор: system-architect#1
Связи: OQ-A-05 (остаток: векторная БД), OQ-A-07 (Redis/TimescaleDB/Qdrant), OQ-D-04; FR-031, FR-035, FR-060, FR-061, FR-088, FR-127; NFR-010, NFR-041, NFR-042, NFR-071, NFR-073; `integrations.md` §3; `data-model.md` §5, §8, §11.

## Контекст

As-is: MinIO — де-факто основная БД (JSON-объект на сущность, без версий); ChromaDB `latest` против v1 API (Chroma ≥ 1.0 удалил v1), v2 через build-tag + CGO + `chroma-go`; Neo4j 5.18 — самая зрелая часть; TimescaleDB и Qdrant в compose без клиентов; Redis — заглушка без драйвера. Требования: ПДн-связки — отдельное удаляемое хранилище (FR-060/061, NFR-041/042); служебные данные gateway (сессии, раунды, идемпотентность, outbox); снапшоты с историей; retention журнала; метрики MVP-1 — CSV (FR-088). Масштаб MVP-1: ≤ 50 сущностей на scope, ≤ 1 МБ снапшот, десятки доставок в очереди.

## Рассмотренные варианты

| Область | Вариант | Плюсы | Минусы |
|---|---|---|---|
| Состояние сущностей | **MinIO объект на сущность + снапшот мира, bucket versioning** | есть SDK и данные; атомарная замена объекта; версии объектов «бесплатно»; объёмы малы | нет запросов (не нужны: State держит мир в памяти) |
| | PostgreSQL/JSONB | запросы, транзакции | +сервис, миграции, драйвер; не нужен при ≤ 50 сущностей/scope |
| Приватные данные gateway | **SQLite (`modernc.org/sqlite`, CGO-free), два файла: `links.db`, `gateway.db`** | SQL для outbox/идемпотентности; физическое удаление строки; файл = единица бэкапа/шифрования; без сервиса | один писатель (gateway — единственный, ок) |
| | MinIO | уже есть | нет запросов; связки ПДн рядом с состоянием мира — риск NFR-041 |
| | PostgreSQL | полноценно | лишний сервис ради ≤ 10 связок |
| Векторный индекс | **Qdrant + официальный `github.com/qdrant/go-client` (gRPC)** | без CGO; стабильный API; уже в compose с healthcheck; один путь сборки | переписать `chroma*.go` (~600 строк) |
| | Chroma 1.x + `chroma-go` v2 | меньше правок | build-tag + CGO, сторонний клиент, образ ломался на `latest` |
| | без векторов | ничего не делать | теряется семантический контекст; но принимаем как **режим деградации** по умолчанию |
| Граф | **Neo4j 5.26 LTS (пин)** | зрелый код as-is, трасса, связи | +1 ГБ RAM; Should — стартует по профилю |
| Кэш | **нет** (in-memory + снапшот) | ноль сервисов | при выносе Swarm в отдельный процесс понадобится Redis — дверь открыта |
| Метрики | **нет** (CSV + `mvctl`) | по metrics.md §6 | Prometheus/Grafana — E-G |

## Решение

1. **MinIO** (пин на `RELEASE.*`-тег): бакеты `entities-{world}` (`{type}/{id}.json`, версия в теле), `snapshots-{world}` (`state/`, `swarm/`), `prompts-{world}` (полные промпты по флагу `LLM_STORE_PROMPTS=true`), `ops-artifacts`; bucket versioning включается `redpanda-init`-подобным init-контейнером (`mc version enable`); один общий клиент `shared/objstore` над `minio-go/v7`. Ключи доступа — из env (не `minioadmin` по умолчанию в compose — задача DevOps).
2. **SQLite в gateway**: `links.db` — таблица `links(external_platform, external_id, player_id, world_id, status, notice_shown_at, consent_at, age_confirmed_at, last_seen_at, created_at)` с PK `(platform, external_id)`, уникальный индекс по `player_id`; `gateway.db` — `sessions`, `turns`, `rounds`, `idempotency_keys`, `deliveries`. Файлы в томе gateway; `/forget` = `DELETE` + `VACUUM` по расписанию; бэкап связок — отдельная политика оператора (шифрование, срок) — задача DevOps/security.
3. **Qdrant** (пин версии, gRPC :6334): коллекции `events-{world}`, `entities-{world}`; эмбеддинги через LLM-шлюз (`Provider.Embed`, Ollama `nomic-embed-text` или `bge-m3` — выбор при замере); индекс перестраивается из журнала командой `mvctl memory rebuild`. ChromaDB удаляется из compose и кода.
4. **Neo4j 5.26 LTS** (профиль `memory`): существующий индексер semantic-memory (Event/Entity/World, `relations[]`) сохраняется; тесты — за `integration`.
5. **Память как обогащение**: контекст агента строится Swarm из состояния + окна журнала; memory даёт семантический поиск, сводку фона по `(since_at, now)` и трассу; при недоступности memory — `/health degraded`, агенты работают без неё (FR-035 Should).
6. **Не берём**: TimescaleDB (удалить из compose и `.env.example`), Redis (удалить `shared/redis`; вернуться при E-A/E-G или выносе Swarm), ChromaDB, Redpanda Schema Registry (ADR-007).
7. **Шина как журнал** — Redpanda с retention по топикам (ADR-007); резервное копирование тома Redpanda и MinIO — задача DevOps (`architecture/infrastructure.md`).

## Последствия

- Позитивные: минус три сервиса (Chroma, Timescale, Redis-план), один путь сборки без CGO, ПДн изолированы физически, снапшоты с историей версий.
- Негативные: переписывание `chroma*.go` под Qdrant (EPIC-005); SQLite — ещё одна зависимость (чистый Go, без CGO); MinIO остаётся «БД» — при росте объёмов State нужно вынести за `Store` в PostgreSQL (интерфейс предусмотрен).
- Что придётся сделать: EPIC-001 — compose (пины, профили, init versioning), удаление Chroma/Timescale; EPIC-004 — схемы SQLite и миграции (выбор делегирован команде; **решено ADR-019: `pressly/goose/v3` как библиотека из embed FS, без `Down` — `golang-migrate` не используется**; сведение A4-2, запрос g); EPIC-005 — Qdrant-адаптер, `mvctl memory rebuild`; тест NFR-041 обходит все четыре хранилища и логи.

## Дополнение 2026-09-09 (сведение A3 шаг 4)

1. **MinIO — образы больше не публикуются** (Docker Hub заморожен на `RELEASE.2025-09-07T16-13-09Z`; фикс CVE-2025-62506 в `RELEASE.2025-10-15T17-29-55Z` вышел только исходниками; в феврале 2026 репозиторий `minio/minio` архивирован). Решение вынесено в **ADR-021**: MVP-1 — сборка из исходников последнего тега (`build/minio.Dockerfile`, пин по git-тегу), `shared/objstore` ограничен `Put/Get/Stat/List/Delete/EnsureBucket` + `Capabilities()`; ни один путь кода не зависит от bucket versioning (откат — ротация снапшотов K=5 и `history` сущности); замена сервера (SeaweedFS) — по триггерам ADR-021; подтверждение владельца — OQ-A-20.
2. **Versioning и ILM при создании бакета** (п. 1 уточнение): `shared/objstore.EnsureBucket(ctx, name, opts{Versioned, NoncurrentExpireDays, ExpireDays})` включает versioning и правила ILM сам (бакеты `*-{world}` создаёт `mvctl world init`, init-контейнер их не видит); `minio-init` — только страховка для существующих бакетов и создание сервисного пользователя (платформа не ходит под root-ключом). Если сервер не поддерживает versioning (`Capabilities().Versioning=false`) — предупреждение в `/health.store`, не ошибка.
3. **`prompts-{world}`** (SEC-22): versioning **off**, ILM `expire-days 30` (было 90), вне политики бэкапа, флаг `MV_LLM_STORE_PROMPTS=false` по умолчанию (OQ-A-19: промпты по флагу).
4. **`links.db`** (SEC-04/05, п. 2 уточнение): `secure_delete=ON`, `auto_vacuum=INCREMENTAL`; сразу после `/forget` — `PRAGMA wal_checkpoint(TRUNCATE)` + `PRAGMA incremental_vacuum` в той же операции (не «по расписанию»; sweeper раз в час остаётся страховкой) — уточняет ADR-019 п. 5; именованный том Docker `gateway-data`; бэкап — `infrastructure.md` §5.6 (шифрование `age`, ≤ 30 дней). OneDrive: пользователь подтвердил, что `Documents` не синхронизируется — тома данных и `.env` могут лежать рядом с проектом (SEC-30 снят).
5. **Neo4j APOC** (SEC-32): плагин `apoc` в compose **не включается** — as-is код `semantic-memory` его не использует (проверено grep по `*.go`; `NEO4J_PLUGINS: ["apoc"]` встречается только в устаревших worktree-копиях compose). Если EPIC-005 понадобятся APOC-процедуры — только с `apoc.import.file.enabled=false`.
6. **Redpanda: том as-is не мигрирует** прямым скачком v24.2 → v26.1 (обновление по одной feature-версии): том пересоздаётся при первом `make up` целевого стека (данные as-is не нужны — `overview.md` §19); перед этим `make archive-legacy` снимает копию томов MinIO/Redpanda в `./backups/legacy-<date>/` вне git.
7. **Один источник версий**: `build/versions.env` (образы compose, testcontainers, toolchain) — читается `docker-compose.yml` (`${...}`), `Makefile` и интеграционными тестами (`testkit.Versions()`), чтобы стек и testcontainers не расходились.
8. **Chroma в профиле `legacy`** (п. 6 уточнение): удаляется из целевого стека, но остаётся в compose-профиле `legacy` вместе с as-is `semantic-memory` до S5 (ADR-001 дополнение п. 6).
