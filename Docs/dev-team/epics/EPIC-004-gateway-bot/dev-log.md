# Журнал разработки EPIC-004 «Вход игрока: gateway и Telegram-бот»

Формат записи: экземпляр разработчика, задача, что сделано, решения по ходу, отклонения от дизайна,
результаты проверок DoD, открытые вопросы. Язык — русский; код и конфиги — английские.

---

<!-- dev-log T-302 -->
## developer#2 · T-302 · хранилище gateway: SQLite и миграции · 2026-09-13

Ветка `task/T-302-store-migrations` (родитель — `epic/EPIC-004-gateway-bot`), TEAM-3, Opus. Подробности — карточка `tasks/T-302.md`, раздел «Выполнение».

- **Что сделано.**
  - `internal/gateway/migrations`: `embed.go` (`Links()`, `Gateway()` — `fs.Sub` над одним `embed.FS`), `links/0001_init.sql`, `gateway/0001_init.sql`. Схема — component §4.1/§4.2 целиком, вместе с `rounds` и `group_participation`. `sessions.end_reason` допускает `forget` (C-10 v1.1). В комментарии миграции сказано, что статус `abandoned` в gateway не хранится.
  - `internal/gateway/store`:
    - `open.go`: `OpenLinks`/`OpenGateway(ctx, path)`, `DataDir(env.Source)` (читает `MV_GATEWAY_DATA_DIR` из манифеста, пустое значение — ошибка), `LinksPath`/`GatewayPath`. PRAGMA передаются в DSN, `SetMaxOpenConns(1)`, после открытия PRAGMA читаются обратно. Каталог создаётся с `0700`, файл — с `0600`; права шире — ошибка (кроме Windows).
    - `migrate.go`: `MigrateLinks`/`MigrateGateway` — goose `Provider` без глобального реестра, возвращают число применённых миграций.
    - `compact.go`: `CompactLinks` — `incremental_vacuum`, затем `wal_checkpoint(TRUNCATE)`; `busy=1` — ошибка.
    - `retention.go`: сроки хранения §4.2 константами и интерфейс `Sweeper` без запуска.
  - `go.mod`/`go.sum`: `modernc.org/sqlite v1.58.0`, `github.com/pressly/goose/v3 v3.28.0` (ADR-019).
- **Решения по ходу.**
  - PRAGMA — в DSN, а не отдельными `Exec`: драйвер применяет параметры к каждому новому соединению, и замена сломанного соединения их не теряет. Взяты короткие ключи драйвера (`_journal_mode`, `_auto_vacuum`, …), не `_pragma=…`. Список `_pragma` драйвер сортирует по алфавиту, а для коротких ключей порядок документирован: `auto_vacuum` применяется до `journal_mode`.
  - Проверка PRAGMA при открытии. SQLite молча оставляет прежнее значение, если PRAGMA применить нельзя. Пример — `auto_vacuum` в файле, где уже есть таблицы. Такой `links.db` не может стереть забытый ID, поэтому старт падает (тест `TestOpenLinksRefusesAFileWithoutIncrementalVacuum`).
  - Порядок в `CompactLinks`: сначала vacuum, потом checkpoint. ADR-019 доп. п. 1 называет их в обратном порядке, но vacuum сам пишет в WAL. Если checkpoint идёт последним, WAL остаётся пустым, а файл — усечённым. Мутант O1 (обратный порядок) краснеет: WAL 4152 байта.
  - `CompactLinks` вынесена в `store`. Её вызовут `links.Store.Forget` (T-303) и страховочный sweeper, а тест физического удаления проверяет продуктовый код, а не SQL из теста.
  - Права `0600` проверяются у обоих файлов, не только у `links.db`: каталог общий, а в `gateway.db` лежат тексты доставок.
  - `embed.FS` объявлен в пакете `migrations` (`store/` не видит `../migrations`).
  - Тесты SQLite — unit, без тега `integration`. SQLite работает в процессе на временном файле, Docker не нужен (уточнение tech-lead#3). Скан «`strings`» — поиск байтов на Go.
  - Помощник `tempDir` в тестах вместо `t.TempDir`. На Windows `-wal`/`-shm` после `Close` может ненадолго остаться в состоянии «delete pending», и `t.TempDir` падает с «The directory is not empty». В прогонах мутантов так упали 3 посторонних теста; удаление повторяется до 5 с.
- **Отклонения от дизайна и задачи.**
  - Тесты миграций и физического удаления — unit, а не `//go:build integration` (так решил tech-lead#3).
  - `OpenLinks`/`OpenGateway` принимают `ctx` (в component §3 — `OpenLinks(path)`): он нужен для проверки PRAGMA.
  - DoD T-302 «`MV_GATEWAY_DATA_DIR` объявлен…; `.env.example` дополнен» уже выполнен: переменная объявлена в `shared/env/vars.go:98` и есть в `.env.example:126`. Заново не объявлял, файлы не трогал.
  - В `store/` сверх списка файлов задачи добавлены `compact.go` и `retention.go`.
  - `design.md:127` утверждает, что зависимости «уже учтены F-4». Это неверно: в `go.mod` их не было.
- **go.mod — общий файл.** goose v3.28.0 требует `go 1.26.0`, поэтому директива `go 1.26` стала `go 1.26.0`. По MVS подняты общие косвенные зависимости:
  - `pierrec/lz4/v4` 4.1.15→4.1.29 (его использует kafka-go);
  - `docker/go-connections` 0.7.0→0.8.1, `moby/moby/client` 0.5.0→0.5.1;
  - otel 1.44→1.46, `otelhttp` 0.69→0.71;
  - `testify` 1.11.1→1.12.1, `go-logr/logr` 1.4.3→1.4.4.

  `multierr` 1.11 — новая косвенная, а не поднятая (исправлено в итерации 2).

  Их требует `go.mod` goose, иначе нельзя. `go mod tidy` убрал `davecgh/go-spew` и `pmezard/go-difflib` и добавил `kr/text`. `make test` и `golangci-lint run ./...` после подъёма зелёные.
- **Тесты:** 11 тестов в `store`, 1 в `migrations`; покрытие `store` — 81 %. Два теста прав пропускаются на Windows, их проверяет Linux-джоб CI.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути). Прогнано 23, все красные:
  - M0 — контрольный, сборка;
  - `CompactLinks`: V1 без vacuum, C1 PASSIVE вместо TRUNCATE, B1 без проверки `busy`, O1 обратный порядок;
  - PRAGMA: S1 без `secure_delete` (ID 15 раз в файле), A1/A2 без `auto_vacuum`, Y1, J1, F1, P1 `MaxOpenConns(2)`, R1 без `verify`;
  - D1, Q1, G1;
  - схема: E1/E2 CHECK `end_reason`, X1 `external_ref` и X2 `link_id` в `gateway.db`, K1 без каскада, U1/U2 неуникальные индексы.

  Первый проход нашёл два дефекта тестов. Q1 был зелёным: на Windows `?` отвергает ОС, тест теперь требует причину. Три посторонних теста упали на удалении TempDir, отсюда `tempDir`. Проход повторён целиком. Таблица — в карточке.
- **Прогоны:** `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./internal/gateway/...` — ok; `golangci-lint run ./internal/gateway/...` и `./...` — 0 issues; `make test` — exit 0; `make vuln` — exit 0 (0 вызываемых уязвимостей; 3 в `golang.org/x/crypto v0.55.0` не вызываются, версия не менялась).
- Карточка и журнал записаны в рабочую папку задачи `.worktrees/T-302`. `tasks.md` эпика не правил. Docker, `make up`, интеграционные тесты и стенд `:8888` не трогал, `.env` не открывал. Не коммитил.

### developer#2 · T-302 · итерация 2 по ревью #1 · 2026-09-13

Вердикт ревью #1 — «принять» (Minor 1, Nit 6). Все замечания закрыты до приёмки по решению оркестратора. Подробности и таблица мутантов — в карточке `tasks/T-302.md`, раздел «Итерация 2».
- **Mi-1.** Сравнение прав — чистая функция `modeTooWide(got, limit fs.FileMode) bool` без `runtime.GOOS`; `checkMode` пропускает Windows только в месте вызова. Табличный `TestModeTooWide` (14 строк) идёт на всех ОС. Тесты на реальных файлах по-прежнему со skip на Windows; `GOOS=linux go vet` их компилирует.
- **N-1.** Комментарий `links/0001_init.sql` описывает порядок как в коде: vacuum, затем checkpoint.
- **N-2.** Фильтр служебных таблиц — `substr(name, 1, 7) <> 'sqlite_'`. Мутант N2b подтвердил дыру прежнего фильтра: `sqlitex_extra` с `external_id` проходила тест.
- **N-3.** Второй контроль теперь проверяет, что после DELETE ID ещё в самом `links.db` и WAL не пуст. Он ловит самостоятельный checkpoint SQLite; мутант T3 красный.
- **N-4.** В «Отклонения» карточки — DSN на коротких ключах драйвера против `_pragma=…` из ADR-019 п. 1.
- **N-5.** `multierr` — новая косвенная, а не поднятая. Перечислены записи только в `go.sum`: `pprof`, `golang-lru/v2`, `x/mod`, `x/tools`, 10 модулей `modernc.org/*`, `otel/sdk{,/metric}` 1.46.0, хэш `creack/pty v1.1.9/go.mod`.
- **N-6.** Ссылка у `LinksCompactInterval` — ADR-019 п. 5 и доп. п. 1.
- Бэклог ревьюера — в карточке, пп. 2–5.
- **Мутанты** (новая копия в scratch, без `-overlay`, удалена по точному пути). Красные: M0 контрольный, W1–W5 на `modeTooWide`, N2, T3; повторены C1, S1, O1, V1. N2b зелёный ожидаемо: это демонстрация дыры, а не мутант кода.
- **Прогоны:** `go build ./... && go vet ./...` — 0; `GOOS=linux go vet ./internal/gateway/...` — 0; `go test -short -count=1 ./internal/gateway/...` — ok (12 PASS, 2 SKIP); `golangci-lint run ./internal/gateway/...` и `./...` — 0 issues; `make test` — exit 0, 29 пакетов.
- `go.mod`/`go.sum` не менялись. Docker, стенд `:8888` и `.env` не трогал. Не коммитил.
