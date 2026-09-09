# ADR-019: SQLite в gateway — `modernc.org/sqlite`, два файла с разными PRAGMA, миграции `goose` из embed, единственный писатель, физическое удаление при `/forget`

Статус: предложено (к утверждению на G2 с планом EPIC-004) · Дата: 2026-09-09 · Автор: architect#3 (TEAM-3, EPIC-004) · Уровень: реализация блока
Связи: ADR-004 п. 2 (SQLite, два файла — не пересматривается; здесь — схема, PRAGMA, миграции), ADR-006 п. 3 (outbox), ADR-009 п. 2 (`links.db` — единственная копия, физическое удаление); FR-060, FR-061, FR-085, BR-07; NFR-010, NFR-013, NFR-041, NFR-042; `data-model.md` §5; `components/gateway-and-bot.md` §4, §8.

## Контекст

ADR-004 выбрал SQLite через `modernc.org/sqlite` (CGO-free) с двумя файлами: `links.db` (ПДн-связки) и `gateway.db` (сессии, ходы, раунды, ключи идемпотентности, outbox) и оставил команде «схемы SQLite и миграции (`golang-migrate` или встроенные SQL-файлы)». Требования: физическое удаление строки при `/forget` (после которого поиск по внешнему ID пуст и файл не содержит ID даже в свободных страницах); порядок доставок на игрока и голова очереди; идемпотентность `action_key` 24 ч; восстановление сессий/раундов/outbox после рестарта; ни одного внешнего ID вне `links.db`; один разработчик — минимум инструментов. Замечание системного архитектора: схемы и миграции — предмет этого ADR; идемпотентность `POST /v1/characters` содержит внешний ID в ключе.

## Рассмотренные варианты

| Вопрос | Вариант | Плюсы | Минусы |
|---|---|---|---|
| Драйвер | **`modernc.org/sqlite` v1.58.0 (SQLite 3.53.4, BSD-3, 2026-09-01)** | без CGO — один путь сборки (ADR-004), кросс-компиляция; DSN-параметры `_pragma`, `_busy_timeout`; 3,5 тыс. импортёров | медленнее `mattn` на ~20–30 % — незначимо при десятках запросов/с |
| | `mattn/go-sqlite3` | эталон | CGO |
| Миграции | **`github.com/pressly/goose/v3` v3.28.0 (MIT) как библиотека, `goose.NewProvider(DialectSQLite3, db, embedFS)`** | версии в таблице `goose_db_version`, `Up/Status`, embed FS, `Provider` без глобального состояния; работает с любым `*sql.DB` (драйвер `sqlite`) | +1 зависимость (небольшая) |
| | `golang-migrate/migrate` | распространён | драйвер `sqlite` завязан на `modernc` через отдельный модуль; CLI-ориентирован; тяжелее |
| | свой мигратор (таблица + embed, ~60 строк) | ноль зависимостей | «ещё один велосипед» без `Status`/`Down`; тесты писать самим |
| Идемпотентность `POST /v1/characters` | **таблица `character_requests` в `links.db`** (ключ содержит внешний ID) | внешний ID остаётся в единственном файле; каскадное удаление при `/forget` | `links.db` содержит не только связки |
| | ключ = `SHA-256(platform:external_id)` в `gateway.db` | один файл для идемпотентности | хэш числового Telegram ID обратим перебором — нарушает NFR-042/ADR-009 |
| Физическое удаление | **`DELETE` + `PRAGMA secure_delete=ON` + `auto_vacuum=INCREMENTAL` и `incremental_vacuum` в sweeper'е** | освобождённые страницы обнуляются немедленно; grep по файлу после `/forget` = 0 | `secure_delete` замедляет запись (файл ≤ 10 строк — незначимо) |
| | `DELETE` + `VACUUM` по расписанию (ADR-004) | проще | между DELETE и VACUUM байты ID остаются в свободных страницах и WAL |
| Долговечность | **`links.db`: `synchronous=FULL`; `gateway.db`: WAL + `synchronous=NORMAL`** | ПДн — максимальная надёжность; служебные — компромисс скорость/долговечность (при сбое ОС возможна потеря последних мс транзакций — покрывается повтором событий шиной: outbox идемпотентен по `(event_id, player_id)`) | два набора PRAGMA |
| Конкурентность | **`db.SetMaxOpenConns(1)` на каждую БД; long-poll ждёт вне транзакции** | нет `SQLITE_BUSY` внутри процесса; единственный писатель — по ADR-004 | нет параллельных чтений (не нужны при нагрузке MVP-1) |
| Порядок outbox | **`seq INTEGER PRIMARY KEY AUTOINCREMENT` + оконная функция `ROW_NUMBER() OVER (PARTITION BY player_id ORDER BY seq)`** | строгий порядок создания; выбор головы очереди одним запросом | AUTOINCREMENT чуть дороже rowid — незначимо |

## Решение

1. **Драйвер** `modernc.org/sqlite` v1.58.0 (пин); открытие через `store.OpenLinks(path)` / `store.OpenGateway(path)` с DSN `file:<path>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)&_pragma=synchronous(FULL|NORMAL)`; для `links.db` дополнительно `secure_delete(ON)`, `auto_vacuum(INCREMENTAL)`; `SetMaxOpenConns(1)`, `SetConnMaxLifetime(0)`. Права: `links.db` создаётся с `0600` в каталоге `0700` (`GATEWAY_DATA_DIR`); ошибка старта, если права шире.
2. **Миграции** — `goose` v3.28.0 как библиотека: `internal/gateway/migrations/links/NNNN_*.sql` и `…/migrations/gateway/NNNN_*.sql` (embed, `//go:embed`), `goose.NewProvider(goose.DialectSQLite3, db, fsys)` → `Up(ctx)` при старте контекста; отдельная таблица версий в каждом файле. Нумерация — только EPIC-004 (ownership.md); `Down` не пишутся (откат — восстановление файла из бэкапа); CLI `goose` не используется.
3. **Схема** — `components/gateway-and-bot.md` §4 (`links`, `character_requests`; `sessions`, `turns`, `rounds`, `group_participation`, `idempotency_keys`, `pending_characters`, `deliveries`, `processed_events`, `cursors`). Инварианты: уникальный частичный индекс `player_id` в `links`; `ux_sessions_active` — одна активная сессия на scope; `ux_rounds_open` — один открытый раунд на scope; `ix_deliveries_event (event_id, player_id)` — идемпотентная постановка; `json_valid` CHECK на JSON-полях; всё время — ISO-8601 UTC `TEXT`.
4. **Внешние ID** — только `links.db` (`links.external_id`, `character_requests.external_id`); в `gateway.db` ни одного поля с внешним ID; `Delivery.route` формируется из `links.RouteFor(player_id)` в момент выдачи и не сохраняется. Тест NFR-041 включает `strings`-скан обоих файлов и WAL.
5. **`/forget`** — одна транзакция в `links.db`: `DELETE FROM links WHERE …` (каскад `character_requests`); затем хуки в `gateway.db` (outbox → `dropped`, сессия → `ended leave`, выход из группы через события). Sweeper раз в час: `PRAGMA incremental_vacuum` и `PRAGMA wal_checkpoint(TRUNCATE)` на `links.db` — чтобы удалённые байты не оставались в WAL.
6. **Транзакционная модель consumer'а**: обработка события шины = одна транзакция `gateway.db`: `INSERT processed_events` (дубль → выход без эффектов) → эффекты (outbox, turns, rounds, sessions) → `UPDATE cursors`; коммит → ответ `Bus` «обработано». Ошибка → откат и ошибка обработчику (повтор ×3 → DLQ, C-01).
7. **Retention** (sweeper 60 с): истёкшие `idempotency_keys`/`character_requests` (24 ч); `deliveries` `pending` старше `expires_at` → `dropped`; `delivered/dropped` старше 7 дней — удаление; `processed_events` старше 24 ч; `sessions/turns/rounds` не чистятся в MVP-1 (≤ 1 000 строк/сессия; при необходимости — задача E-G).
8. **Бэкап** — файлы целиком (`sqlite3 .backup` или копия при остановленном процессе); `links.db` — по политике DevOps/security (шифрование, ≤ 30 дней), `gateway.db` — вместе с томами Redpanda/MinIO (`infrastructure.md`).

## Последствия

- Позитивные: один путь сборки без CGO; ПДн физически в одном файле с отдельной политикой; миграции версионируются и тестируются как обычный Go-код; outbox и идемпотентность — SQL, а не самописные структуры.
- Негативные: две зависимости (`modernc.org/sqlite`, `goose`) — обе чистый Go; `MaxOpenConns(1)` исключает параллельные чтения — при росте нагрузки достаточно второго read-only пула (WAL это допускает) без изменения схемы; `synchronous=NORMAL` на `gateway.db` допускает потерю последних миллисекунд при сбое ОС — компенсируется идемпотентностью и повтором событий.
- Что придётся сделать: EPIC-004 I1 — `store`, миграции `0001_init.sql` для обеих БД, интеграционные тесты миграций и `forget`; DevOps — том `GATEWAY_DATA_DIR`, права, бэкап; security-engineer — проверка `secure_delete`/WAL-checkpoint в threat-model.
