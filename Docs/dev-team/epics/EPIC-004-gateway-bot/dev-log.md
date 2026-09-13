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
# Журнал разработки EPIC-004 «Вход игрока: gateway и Telegram-бот»

Записи разработчиков TEAM-3: что сделано, решения по ходу, отклонения от дизайна, как проверено. Подробности каждой задачи — в карточке `tasks/T-NNN.md`.

<!-- dev-log T-301 -->
## developer#1 · T-301 · OpenAPI-скелет, DTO, таблица кодов ошибок и Go-клиент gateway · 2026-09-13

Ветка `task/T-301-openapi-dto-client` (родитель — `epic/EPIC-004-gateway-bot`), TEAM-3, Opus. Подробности — карточка `tasks/T-301.md`, раздел «Выполнение». Задача сокращена по сверке tech-lead#3: схемы C-04/C-10 и их регистрация уже сделаны в EPIC-001. Контракты — C-04 v1.3, C-08 v1.3, C-01 v1.7.
- **Что сделано:**
  - `api/gateway.openapi.yaml` — OpenAPI 3.1: 18 операций, 38 схем, 10 общих ответов ошибок с `x-error-codes`. Раздел `admin` (3 операции C-06) помечен `x-reserved: EPIC-003 T-239`. В спецификации `worlds[].llm.cloud_enabled`, nullable `GroupView.leader_id`, `409 no_leader`;
  - `internal/gateway/api`: `dto.go` (типы C-08 с правилом тегов «без `omitempty` — обязательное, указатель без `omitempty` — nullable»), `errors.go` (36 кодов, `NewError`, `WriteError`), `router.go` (таблица маршрутов без сервера, `Mount` на mux процесса);
  - `internal/gateway/client`: методы C-08, `APIError`, повторы на сетевые ошибки и `503` (3 повтора, 200/400/800 мс через `clock.Timers`, то же тело и тот же `action_key`), `4xx` без повтора; `Deliveries`/`Ack`.
- **Решения по ходу:**
  - `Action` возвращает `ActionResult{Accepted|Pending|Group}`: `202 pending` C-08 v1.1 не помещается в сигнатуру component §6;
  - `Deliveries`/`Ack` берут `client_id` из `Client.ClientID`, чтобы путь и `X-Client-Id` не разошлись (SEC-12);
  - long-poll внутри клиента не повторяется, иначе повтор мог бы получить `409 poll_in_progress`;
  - `/health` есть в спецификации, но не в роутере шлюза: его регистрирует `shared/runtime`;
  - сверка роутера со спецификацией идёт через список `notYetMounted`, который вычёркивают T-303 и следующие задачи.
- **Тесты:**
  - `openapi_test.go`: заголовок; операции §5.6; зарезервированный `admin`; роутер ↔ спецификация; коды `errors.go` ↔ `x-error-codes` в обе стороны со статусами; `$ref` разрешаются; DTO ↔ `components.schemas` (свойства, `required`, nullable, типы); значения, общие со схемами событий (`actor_kind`, `session.kind`, `participation`, `close_reason`, `player.said.text` ≤ 500) и с `shared/runtime`; у каждого действия есть зарегистрированный тип события;
  - `errors_test.go`, `router_test.go`;
  - клиент — 22 теста на `httptest` с фейковыми `clock.Timers`.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути; скрипт тоже удалён): M0 контрольный — красный (сборка); A1–A23 — все красные. Среди них: `no_leader` с другим статусом или без строки в таблице или спецификации; `leader_id` с `omitempty` или не указатель; `maxLength` 501; снятый `x-reserved`; `Mount` без метода; повтор на любой `5xx`/`4xx`; лишний повтор; пауза ×4; long-poll с повторами; пауза без контекста; лишнее поле DTO; `pending` как `accepted`; `limit` без ограничения; изменённые enum и тип поля; смонтированный `health`.
- **Прогоны:**
  - `go build ./... && go vet ./...` — 0; `gofmt` пусто;
  - `go test -short -count=1 ./internal/gateway/...` — ok;
  - `golangci-lint run ./internal/gateway/...` — 0 issues (после исправления одного `errcheck` в тесте);
  - `go run ./cmd/mvctl contracts check` — 0;
  - `make test` из Git Bash — exit 0, покрытие `api` 96.9 %, `client` 91.6 %. Из PowerShell цель падает на `bash scripts/coverage-gate.sh`: там `bash` — это WSL. Предложение в бэклог.
  - `-race` недоступен. Docker и стенд не трогались.
- **Отклонения от дизайна:** сигнатуры клиента (`ActionResult`, `Group`, `Deliveries`/`Ack` без `clientID`), имена схем `WorldSummary`/`RegionSummary`/`WorldLLM` (имя `WorldView` занято состоянием мира), `Health` = `runtime.Status`. Перечень с причинами — в карточке.
- **Не сделано по плану задачи:** тела ответов по каждой операции — минимальные, их уточняют T-303…T-306. Общие файлы, схемы и реестр не менялись.
- Не коммитил.
- **Итерация 2 по ревью #1** (вердикт «принять», Minor 7, Nit 6; закрыто до слияния по решению оркестратора, 2026-09-13).
  - Mi-1: `dto_json.go` — `MarshalJSON` у восьми типов с обязательным массивом; nil кодируется как `[]`. Тест `TestRequiredArraysAreNeverNull` кодирует нулевое значение каждого DTO по значению и по указателю.
  - Mi-2: описание `forgetLink` — `{deleted: false, player_id_detached: null}`.
  - Mi-3: нулевой `Backoff` = `DefaultBackoff`, повторы выключает `client.NoRetry`; литерал `Client{}` повторяет как клиент из `New`.
  - Mi-4: тесты на невповтор 400…504, включая `429`; на отсутствие `external_id` в текстах ошибок `Resolve`/`Consent`/`Forget`/`CreateCharacter`; на отмену висящего long-poll контекстом.
  - Mi-5: `ActionAccepted.status`/`ActionPending.status` связаны с константами; эталон — 36 кодов литералами в `codesOfContract`.
  - Mi-6: `api.Handlers` и `api.GatewayRouter` — единственный конструктор маршрутов. Тест рефлексией заполняет все поля и требует монтирования под своим `operationId`. `notYetMounted` — карта `operationId → задача`.
  - Mi-7: `CloseRound` без повторов. `Forget` → `ForgetResult{…, Repeated}`: при `Repeated && !Deleted` исход неизвестен, T-311 сверяет итог через `Resolve`. Для `Ack` — оговорка в doc.
  - N-1…N-6: тег `admin` уточнён; неизвестный `status` в `202` → `ErrUnexpectedStatus`; кривой `BaseURL` не повторяется; `/` у `BaseURL` срезается при каждом запросе; `X-Request-Id` описан один раз в `info`; doc `Error`/`ErrorResponse`.
  - Решение system-architect (ревизия 4, п. 8): `group_move` убран из `cause` четырёх схем `group.*`. Добавлены 8 фикстур — пары для каждой схемы, невалидная — `cause: "group_move"`. README фикстур дополнен.
  - Мутанты (новая копия в scratch, без `-overlay`, удалена по точному пути, скрипт тоже): M0 контрольный — красный; R3, R4, R6a–c, R7, R8 ревьюера — красные; позитивный R6d — зелёный; свои O1–O9 — красные.
  - Прогоны:
    - `go build ./... && go vet ./...` — 0;
    - `go test -short -count=1 ./internal/gateway/... ./test/fixtures/... ./shared/contracts/...` — ok, покрытие `api` 98.5 %, `client` 94.7 %;
    - `golangci-lint run ./...` — 0 issues;
    - `mvctl contracts check` — 0, `privacy scan testdata/` — чисто;
    - `make test` (Git Bash) — exit 0.
  - Бэклог ревьюера (DoD T-303/T-305/T-306/T-307/T-310/T-311/T-352/T-354/T-356) и 8 пунктов для architect#3 перенесены в карточку.
  - Не коммитил.
