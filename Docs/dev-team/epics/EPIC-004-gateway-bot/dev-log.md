# Журнал разработки EPIC-004 «Вход игрока: gateway и Telegram-бот»

Формат записи: экземпляр разработчика, задача, что сделано, решения по ходу, отклонения от дизайна,
результаты проверок DoD, открытые вопросы. Язык — русский; код и конфиги — английские.

---

<!-- dev-log T-303 -->
## developer#1 · T-303 · связки (links), каркас HTTP-слоя и контекст gateway · 2026-09-13

Ветка `task/T-303-links-http-layer` (родитель — `epic/EPIC-004-gateway-bot`), TEAM-3, Opus. Подробности, таблица DoD и мутантов — карточка `tasks/T-303.md`, раздел «Выполнение».

- **Что сделано.**
  - `internal/gateway/links` — `Store` и `SQLite` над `links.db`: `Resolve`, `Consent`, `AttachPlayer`, `ByExternal`, `ByPlayer`, `RouteFor`, `CharacterRequest`/`SaveCharacterRequest`, `Forget`/`ForgetByPlayer` с `ForgetHooks`, `Compact`, `Sweep`; `Link` печатается как `redacted` (slog, fmt); `NewPlayerID`/`NewLinkID` из `Deps.IDs`.
  - `internal/gateway/api` — `Handlers` и `GatewayRouter` для `resolveLink`/`consentLink`/`forgetLink` (вычеркнуты из `notYetMounted`); `middleware.go` — цепочка component §5.1 (request_id и access-лог, recover, client/actor_kind/client_mismatch, 64 КиБ, nolog-политика, точка rate limit, pollguard, дедлайн запроса); `errors.go` — `503 forget_incomplete`, `WriteJSON`.
  - `internal/gateway/handlers` — обработчики связок; `internal/gateway/context.go` — контекст `gateway` (`Routes`, `Start`, `Stop`, `Health`, sweeper `links.db` в `live`).
  - `api/gateway.openapi.yaml` — семантика `/forget` (SEC-26), `notice_due`, согласие, `503`.
  - EPIC-001 в мягком режиме ревизии 4: `cmd/multiverse/contexts.go` — настоящая фабрика `gateway` вместо заглушки; тестовые помощники `onLoopback` и `emptyWorldEnv` дают временный `MV_GATEWAY_DATA_DIR`; `TestStubIsHealthyAndDoesNothing` пропускает `gateway`.
- **Решения по ходу.**
  - Обработчики вынесены из `api` в `handlers`: `api` импортирует бот, а обработчик связок тянет драйвер SQLite.
  - Request id и access-лог — внешний слой, recover — второй: `500` паники тоже с `X-Request-Id` и в логе. Nolog — политика операции в access-логе: только `request_id` и `code` (у строки паники ещё `handled=false`).
  - Заблокированный checkpoint после `/forget`: связка удалена, `ErrCompactionPending`, ответ `503 forget_incomplete`, `/health` — `degraded`. Повтор (клиент повторяет `503` сам) доделывает сжатие, страховка — sweeper (1 мин — отложенное, 1 ч — полное) и `Stop`.
  - Хуки каскада — до `DELETE`: упавший хук оставляет связку для повтора. В T-303 хуков и предложений нет; `proposal_id = forget:{player_id}` и «`dead_entity` → 200» — строка для T-314/T-355.
  - `link_id`/`player_id` — из `Deps.IDs` (воспроизводимы при `sequence`), ULID не нужен; `X-Request-Id` — `uuid.NewString`, чтобы запросы не сдвигали последовательность.
  - Серверные таймауты — T-446; в T-303 дедлайн 5 с на контексте запроса всем, кроме `api.LongPollOperations` — точка подключения T-307.
  - Открытие БД в `Start` — под `context.WithoutCancel`; `objstore.New` не нужен до T-304.
- **Отклонения.** Пакет `handlers` вместо `api/handlers_links.go`; порядок recover/request_id; сигнатуры стора (`Resolution`, `ConsentForm`, `ForgetResult`, `now` в запросах персонажа); новый код `503 forget_incomplete` (C-08 — запрос system-architect); DoD `serve --contexts=gateway --bus=memory` дословно невыполним (`serve.go` требует `all` для `memory`) — проверено тестом процесса с одним контекстом `gateway` и e2e; `.env.example` и `vars.go` не менялись — новых переменных нет.
- **Проверки.** `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — ok; `go test -tags e2e ./test/e2e/...` — ok; `golangci-lint run ./...` — 0 issues; `mvctl env check` — 0; `mvctl privacy scan testdata/` — чисто; `make test` — exit 0. Покрытие: `gateway` 81 %, `api` 97 %, `handlers` 100 %, `links` 84 %. Мутанты — 37 засчитаны, все красные (в копии дерева в scratch, без `-overlay`; копия удалена по точному пути).
- Интеграционные тесты, Docker, `make up`, стенд `:8888` не трогал, `.env` не открывал, `tasks.md` эпика не правил. Не коммитил.

## developer#1 · T-303 · итерация 2 (ревью #1) · 2026-09-13

Ветка `task/T-303-links-http-layer`, TEAM-3, Opus. Итерация по ревью #1 (0/1/6/7), решению system-architect#1 по `503 forget_incomplete` и решению оркестратора по M-1. Запуск был прерван лимитом API и возобновлён после сверки `git status`, `diff` и времени файлов. Подробности, ответ по каждому пункту и сигнатуры для T-456 — карточка `tasks/T-303.md`, «Итерация 2».

- **Что сделано.**
  - M-1: `build/Dockerfile` создаёт `/data` владельцем `nonroot` (65532) с правами `0700` (в builder `mkdir`, в runtime `COPY --chown --chmod`). В `Docs/ops/runbook.md` раздел 2 — абзац о пересоздании тома `gateway-data`, созданного до T-303; команда описана, не выполнялась.
  - 503 `forget_incomplete` по решению architect: `Retry-After: 5`, пока сжатие отложено, 503 получает любой `/forget`, OpenAPI описывает смысл «связка удалена, стирание не подтверждено». В `Start` одно безусловное `Compact` до обслуживания (Mi-2).
  - Mi-1: `DELETE … AND player_id IS ?` с проверкой `RowsAffected`; если персонаж сменился, каскад идёт заново (≤ 3 попыток); параллельный `/forget` получает `deleted:false`.
  - Mi-4: неполное согласие без связки ничего не создаёт.
  - Mi-5: ошибка старта называет `MV_GATEWAY_DATA_DIR`; README и комментарий `.env.example` описывают запуск на хосте.
  - Mi-6: `Link.MarshalJSON` → `"redacted"`.
  - Mi-3 и Nit: тесты `degraded`, сжатия в `Stop`, границы 64 КиБ, порядка client → body_limit; `Stop` закрывает БД при истёкшем дедлайне; проверка `nil` отметок согласия; общий `shared/testkit/gateway/sqlitedir.Temp`; контроль в тесте `Sweep`.
- **Решения по ходу.**
  - `Retry-After` = `busy_timeout` (5 с): раньше повтор, скорее всего, встретит того же читателя.
  - Сжатие при старте идёт и в replay (это не таймер) и старт не валит: неудача ставит отметку.
  - `Stop` при неостановленном sweeper'е не сжимает: соединение может быть занято. БД закрываются всегда.
  - `sqlitedir` — отдельный пакет только на stdlib, чтобы тесты `store` не тянули харнесс `shared/testkit/gateway`.
- **Отклонения.**
  - `contexts.go`, `shared/env/vars.go`, `Makefile`, `CLAUDE.md` и общие документы C-08 не менялись. Правка `build/Dockerfile` (EPIC-001) — по решению оркестратора, отметку даёт tech-lead#1.
  - N-3 не менялся: нет в поручении, вынесен в бэклог.
  - Окно снятия отметки параллельным сжатием (риск «г» ревью) оставлено риском.
- **Проверки.** `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — ok; `go test -tags e2e ./test/e2e/...` — ok; `golangci-lint run ./...` — 0 issues; `mvctl env check` — 0; `mvctl privacy scan testdata/` — чисто; `make test` — exit 0. Покрытие: `gateway` 88 %, `api` 97 %, `handlers` 100 %, `links` 85 %. Мутанты — 16 засчитаны, все красные, включая K1–K4 ревью (копия дерева в scratch, без `-overlay`; копия удалена по точному пути).
- Интеграционные тесты, Docker и стенд `:8888` не трогал, `.env` не открывал. Не коммитил.

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

<!-- dev-log T-304 -->
## developer#1 · T-304 · проекция State (readmodel) и consumer · 2026-09-13

Ветка `task/T-304-readmodel-consumer` (родитель — `epic/EPIC-004-gateway-bot`, 83b9351), TEAM-3, Opus. Подробности, таблица DoD и мутантов — карточка `tasks/T-304.md`, раздел «Выполнение».

- **Что сделано.**
  - `internal/gateway/readmodel` — проекции `World`, `Region`, `NPC`, `CharacterState`, `Group`, `Encounter` на типизированных геттерах `shared/entity`; `Apply` для `entity.created|updated|update.rejected` и `encounter.started|ended` с порядком по версии и признаком `stale`; `LoadFromStateSnapshot` (указатель → объект → сверка `state_hash`); ожидания `Expect`/`Wait`/`AwaitFact` на `clock.Timers` без своих горутин; `Hash`, `Cursor`, `Status`.
  - `internal/gateway/consumer` — `Dispatcher`: догон `Journal` от курсора снапшота, подписки на четыре топика группой `gateway.consumer`, проверка `processed_events` + эффекты + отметка + курсор эффектов одной транзакцией (эффект раньше отметки), `Sweep` окна `processed_events`.
  - `internal/gateway/context.go` — проекция мира `MV_WORLD_ID` из MinIO (`MV_MINIO_*`) при старте, догон и подписки; `Stop` сначала отменяет подписки; `/health` — `projection: ok|missing|stale`, `projection_error`, `bus: fail`.
- **Решения по ходу.**
  - Обе формы `changed[]` (C-02 v1.5 в develop, v1.6 после T-448) читаются одним правилом «ключ `new` есть — записать, нет — удалить»; тест сверяет хеш проекции с `entity.ApplyOps` + `Commit` на каждом шаге для обеих форм. *(Итерация 2, Mi-1: для v1.5 неверно — после удаления ключа хеш расходится с State, см. запись итерации 2.)*
  - Встреча вычисляется при чтении из сущности и `encounter.started`; закрытие необратимо, переход «захватывает» первое событие пары по своему id, повтор того же события сообщает его снова.
  - `projection: missing` без снапшота не деградирует gateway (пустой мир e2e EPIC-001, compose нового мира); деградирует несостоявшаяся загрузка существующего снапшота, `stale` и упавшая подписка.
  - Эффектов в production нет — точка `consumer.Config.Effects` для T-307/T-351.
- **Отклонения.** `Apply` → `(Result, error)`; `Expect`/`Wait` рядом с `AwaitFact`; правило `degraded` для `missing` и определение `stale` отличаются от component §11.4 (вопрос architect#3); `handle_entity.go`/`handle_encounter.go` не выделены; `Sweep` окна — сверх DoD.
- **Проверки.** См. карточку: build/vet, `go test -short ./...`, e2e, `golangci-lint`, `mvctl contracts check`, `mvctl env check`, `make test` — зелёные; мутанты в копии дерева — 29 засчитано, зелёных нет, контрольный первым, копия удалена по точному пути.
- Файлы EPIC-001, `go.mod`, `shared/*`, схемы и файлы T-310 не менялись. Интеграционные тесты, Docker, стенд `:8888` не трогал, `.env` не открывал. Не коммитил.

## developer#1 · T-304 · итерация 2 по ревью #1 · 2026-09-13

Ветка `task/T-304-readmodel-consumer`, TEAM-3, Opus. Ревью #1 — «принять», 0/0/7/4; по решению оркестратора Minor закрыты итерацией до приёмки. Подробности, таблица ответов и мутантов — карточка `tasks/T-304.md`, раздел «Итерация 2 (ревью #1)».

- **Что сделано.**
  - Mi-1: исправлено утверждение о форме v1.5. Удаление ключа и `set null` там неразличимы, проекция хранит `null`, и `Hash()` после удаления ключа расходится с State. Сверка хеша гарантирована только на фактах v1.6. Тест v1.5 утверждает само расхождение. Строка DoD T-309.
  - Mi-2: выбран вариант 2 — документ `Result`/`Effect` («переход уникален в пределах процесса»), тест контракта после рестарта, строки DoD T-307 и T-351. Таблица переходов не сделана: она меняет схему `gateway.db` (component §4.2, `0001_init.sql` — полная схема I1/I2) и тесты T-302, а это решение architect#3 и чужие файлы.
  - Mi-3…Mi-5: тесты `bus: fail` на уровне контекста, no-op удаления отсутствующего пути (v1.6), `Sweep` по возрасту и вызов из sweeper'а.
  - Mi-6: неверная настройка MinIO (один ключ, пустой endpoint, не булев `MV_MINIO_USE_SSL`) → `degraded`, `projection_error: store_misconfigured`; без обоих ключей — по-прежнему `missing` без деградации.
  - Mi-7: бюджеты старта — загрузка 30 с (превышение → `snapshot_timeout`, старт идёт дальше), догон 2 мин (превышение → ошибка старта).
  - N-2: `Waiter` одноразовый, `ErrWaiterUsed`; `Cancel` при ошибке публикации обязателен. N-3: индекс «игрок → открытые встречи». N-4: `projection_error` — код, текст ошибки только в логе. N-1: строка DoD T-309.
- **Решения по ходу.** Бюджет — `context.WithTimeout`, а не `clock.Timers`: kafka-адаптер ставит дедлайн соединения только из `ctx.Deadline()`. Неверная настройка хранилища — `degraded`, а не ошибка старта, как у нечитаемого снапшота.
- **Отклонения.** Mi-2 — вариант 2 вместо предпочтительного первого (причины выше). Новые экспортируемые имена `readmodel`: `MarkLoadFailed`, `Reason*`, `ErrWaiterUsed`; `gateway`: `SnapshotLoadBudget`, `CatchUpBudget` — вопрос architect#3 по §6.
- **Проверки.** См. карточку: build/vet, `go test -short ./...`, e2e, `golangci-lint`, `mvctl contracts check`, `mvctl env check`, `make test`; мутанты R1–R4 ревьюера и новые — в копии дерева, контрольный первым, копия удалена по точному пути.
- Файлы EPIC-001, `shared/*`, миграции и тесты T-302 не менялись. Интеграционные тесты, Docker, стенд `:8888` не трогал, `.env` не открывал. Не коммитил.

<!-- dev-log T-310 -->
## developer#2 · T-310 · ядро бота: config, access.Gate, updates, sender, commands, privacy · 2026-09-13

Ветка `task/T-310-bot-core` (родитель — `epic/EPIC-004-gateway-bot`, 83b9351), TEAM-3, Opus. Подробности, таблица DoD и мутантов — карточка `tasks/T-310.md`, раздел «Выполнение».

- **Что сделано.**
  - `cmd/telegram-bot/internal/privacy` — `Redact` (`bot<digits>:<token>` → `bot<redacted>` + шаблоны `shared/logging`), `Redactor` точных секретов, `Handler` (отбрасывает `external_id`/`chat_id`/`username`/`text` и соседей на любом уровне, редактирует сообщение и строки, прячет типы `go-telegram/bot/models`), `ErrorLog`.
  - `config` — `Load(env.Source)` по манифесту: токен обязателен, allowlist (пусто = никого), URL gateway, таймаут опроса, адрес `/health`, лимит команд, соль `action_key` (по умолчанию `SHA-256(token)`); ошибки и печать без токена, соли и id.
  - `updates` — `Update` только с полями ADR-018 п. 3, `Source`, `Telegram` (long polling, `allowed_updates=[message]`, `timeout=25`, последовательные обработчики, без `WithDebug`, `409` → `ErrConflict`, `ExitCode` → 3), `Fake`.
  - `sender` — `Sender.Send(ctx, chatID, text, *Keyboard)` без разметки; `Telegram` без `parse_mode`, `429` → `retry_after`, сеть/`5xx` → 3 попытки на `clock.Timers`; ошибки `ErrBlocked`/`ErrChatNotFound`/`ErrRejected`/`ErrUnavailable` без id и токена; `Fake`.
  - `commands` — `Parse` по словарю FR-002, синонимы без «/» в точной форме, `say` ≤ 500 с подсказкой; `ActionKey`.
  - `access` — `Gate`: не личный чат → «Пишите боту в личные сообщения.»; вне allowlist → «Доступ по приглашению.» + `bot_denied_total`; 21-я команда за 60 с → «Слишком часто, подождите N с.» — всё до любого вызова gateway.
  - EPIC-001 в мягком режиме: `shared/env/vars.go` (`MV_TELEGRAM_ACTION_KEY_SALT`, `MV_TELEGRAM_COMMANDS_PER_MIN`) и `.env.example`. **Новая зависимость** `github.com/go-telegram/bot v1.25.0` в `go.mod`/`go.sum`.
- **Решения по ходу.**
  - Лимит — скользящее окно, а не token bucket: ведро пропускает 39 команд в первую минуту и 21-ю через 3 с, а NFR-049 и US-008 требуют отказа до 61-й секунды. На серию отказов — один ответ.
  - Тест SEC-06 проверяет «ноль вызовов» `httptest`-моком gateway без `internal/gateway/client`: правило depguard `internal-unlisted` (`**/internal/**`) запрещает боту любые `internal/*`, включая тесты (проверено пробой, проба удалена).
  - Граница импортов — тест на `go list -deps ./cmd/telegram-bot/...`; `WithDebug` — поведенческий тест (debug-обработчик не срабатывает) и разбор AST.
  - Служебные команды — только со «/»; ответ на групповой чат уходит в сам чат, его id нигде не хранится.
- **Отклонения.** Скользящее окно вместо ведра. Имена `MV_TELEGRAM_*` вместо `MV_BOT_*` (component §11.3; `CLIENT_ID`, TTL диалога, ожидание доставок — в T-311/T-312). Минимум 16 символов у явной соли. `Handler` с `ctx`. Типизированные ошибки `Sender`, потолок `retry_after` 60 с. Выход с кодом 3 — `updates.ExitCode`, сам `os.Exit` — T-312. Объём больше ориентира M: ~1600 строк кода и ~1700 строк тестов.
- **Мутанты.** Копия в scratch, без `-overlay`, удалена по точному пути. M0 контрольный сначала выжил: тест сравнивал текст с константой. Добавлен тест с литералами, после чего M0 красный. Первый проход: 4 выживших в `privacy` (их прятала повторная редакция `shared/logging`); тесты пошли и поверх «голого» JSON-обработчика. Итог — 52 мутанта, все красные.
- **Проверки.** `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — ok; бот ×20 — ok; `golangci-lint run ./...` — 0 issues; `mvctl env check` — 0; `mvctl privacy scan testdata/` — чисто; `make test` (Git Bash) — exit 0, покрытие пакетов бота 93–100 %. Первый прогон `make test` упал на моём тесте: `update_id` 42 совпал с цифрами времени. Id исправлен. `gitleaks dir` по новым файлам — чисто (ложное `generic-api-key` в тесте устранено).
- **Для tech-lead#3 и system-architect.** До T-311 нужно правило depguard для `cmd/telegram-bot` (allow `internal/gateway/client$`, `internal/gateway/api$`; исключение из `internal-unlisted`). Без него `flow` не импортирует клиента. Открытый вопрос: US-008/FR-131 («не отвечает в группе») против DoD/ADR-018 («одно сообщение»); сделано по DoD.
- Telegram API, Docker, `make up`, стенд `:8888`, `.env`, интеграционные тесты не трогал; модуль `go-telegram/bot` один раз загружен через proxy. Не коммитил.

### developer#2 · T-310 · итерация 2 по ревью #1 · 2026-09-13

Ревью #1 (code-reviewer#2): вернуть, 0/2/3/6. Решение оркестратора — в групповом чате бот молчит. Подробности, ответы по каждому пункту и таблица мутантов — карточка `tasks/T-310.md`, «Итерация 2».

- **M-1 (потеря обновлений).**
  - Опрос идёт с `bot.WithUpdatesChannelCap(0)`: следующий `getUpdates`, подтверждающий пачку, уходит только когда последнее обновление пачки уже в работе.
  - `adapt` не отдаёт обработчику обновление с отменённым `ctx`.
  - Строка библиотеки «some updates lost» после остановки больше не ошибка: эти обновления не подтверждены, и Telegram отдаст их снова.
  - Решение 6 карточки исправлено.
  - Тест на `httptest`: пока обрабатывается обновление 1 из пачки 1–3, `getUpdates` с `offset > 2` не уходит; после отмены обработано только `[1]`.
- **M-2 (флуд отказами).**
  - Новый `access/refusals.go`: один ответ и одна строка лога на чат за 60 с. Таблица — не больше 1024 чатов; новый чат в полной таблице отклоняется молча. `TooOften` пишет в лог одну строку на серию.
  - `sender.BestEffortPolicy` (одна попытка, без ожидания `429`) и `Telegram.WithPolicy` — отправитель для шлюза.
  - Групповой чат — ни ответа, ни gateway, одна строка лога без id. Константа «пишите в личные» удалена.
  - Тесты:
    - флуд из 30 сообщений постороннего → 1 ответ, команда игрока доходит до gateway;
    - настоящий `sender.Telegram` против `429 retry_after=30` → ни одного взведённого таймера;
    - граница окна и предел таблицы.
- **Mi-1.** `sender.ErrUnauthorized` на `401` при отправке. `401` на `getUpdates` останавливает опрос и возвращает `updates.ErrUnauthorized` (код 1). **Mi-2** закрыт молчанием в группе, тест покрывает и постороннего. **Mi-3:** `privacy.Redact` вырезает тело ответа из ошибки декодирования библиотеки; тесты — в `privacy` и через настоящий `sendMessage` с битым JSON.
- **Nit.**
  - N-1: границы R1–R4 закреплены тестами, длина соли считается в символах.
  - N-2: URL gateway печатается без `userinfo`, query и fragment.
  - N-3: комментарий о переносе теста импортов в T-312.
  - N-4: без изменений (КД §15).
  - N-5: текст `409` упоминает webhook.
  - N-6: группы и ключи `from`, `chat`, `message`, `update` глушатся.
- **Отклонения.** Молчание в группе вместо DoD SEC-07 и ADR-018 п. 3. Один отказ на чат за окно. Новый API `BestEffortPolicy`/`WithPolicy`. Остановка на `401`. Правки ADR-018 и КД §10.2 — строкой для architect#3, строка DoD T-310 SEC-07 — для tech-lead#3. Правило depguard `cmd-telegram-bot` не добавлял (решение system-architect/tech-lead#1).
- **Мутанты.** Копия в scratch, без `-overlay`, контрольный первым, копия удалена по точному пути.
  - Первый проход — 31 мутант, все красные. Но два из них были красными только по таймауту 90 с (висящий long poll в фейке), а два не собрались.
  - Фейк получил `releaseHeld`, тест `401` — предел 5 с.
  - Второй проход (8 мутантов с контрольным) — все красные на утверждениях.
- **Проверки.**
  - `go build ./... && go vet ./...` — 0.
  - `go test -short -count=1 ./...` — ok; бот ×20 — ok.
  - `golangci-lint run ./...` — 0 issues.
  - `mvctl env check` — 0; `mvctl privacy scan testdata/` — чисто.
  - `make test` — exit 0, покрытие пакетов бота 93–100 %.
  - `gitleaks dir` по `cmd/telegram-bot` — чисто.
- **Документы.** Внесены строки DoD: T-311 (SEC-06 на `client.New`, без вызовов с отменённым `ctx`, короткая политика ответов, ключи лога), T-312 (`os.Exit(ExitCode)` и `ErrUnauthorized`, ack по типам ошибок, шлюз на `BestEffortPolicy`, счётчики и риск одного обновления, перенос теста импортов), T-315 (молчание в группе). «T-313» из бэклога ревью про доставку — это `deliver` в T-312; T-313 (e2e соло) не менялся.
- Telegram API, Docker, `make up`, стенд `:8888`, `.env`, интеграционные тесты не трогал. Не коммитил.

<!-- dev-log T-305 -->
## developer#1 · T-305 · `actions`: валидация, идемпотентность, лимит, `InputFilter`, публикация `player.*` · 2026-09-13

Ветка `task/T-305-actions`, папка `.worktrees/T-305`. Статус — `review`. Подробности, пункты DoD и таблица мутантов — карточка `tasks/T-305.md`, раздел «Выполнение».

- **Что сделано.** Пакет `internal/gateway/actions` (`Validate` по §1.4 в порядке КД §5.4, `BuildPlayerEvent`/`BuildPositionProposal`/`BuildRestProposal`/`BuildGMCreated`, `Keys` над `idempotency_keys`, `Limiter`, `InputFilter`/`NoopFilter`, `Turns` + `MemoryTurns`, `Service.Submit`/`FilterText`); обработчик `POST /v1/players/{player_id}/actions`; подключение в контексте (настройки до открытия БД, лимитер только в live, уборка ключей и лимитера sweeper'ом); `readmodel.Encounter.TaskAgentID`/`CreatedAt`; переменные `MV_GATEWAY_RATE_ACTIONS_PER_MIN`, `…_BURST`, `MV_GATEWAY_INPUT_FILTER`, `MV_GATEWAY_ENCOUNTER_GRACE`. OpenAPI: `postAction`, `replayClock` (тег `process`, T-457), `ForgetIncomplete` (дополнение оркестратора по T-456, C-08 v1.5).
- **Решения по ходу.** Лимит — GCRA + журнал минуты (иначе 35 действий в первую минуту). `gm.created` — по решению system-architect (C-04 v1.4), первым в очереди публикаций. `group.*` — `501` до T-352, не сохраняется. Фильтр: `blocked` → `text_invalid`, ошибка → `422` без текста ошибки в журнале. Сессия и ход — точка вставки T-306.
- **Отклонения.** Лимит не «чистый» token bucket; метка удаления в EPIC-003 I2 — английским комментарием; `Turn` в БД не пишется (T-306); правка `shared/env/vars.go` и `.env.example` (EPIC-001, мягкий режим).
- **Проверки.** `go build`/`vet` (и `-tags e2e`), `go test -short ./...`, e2e, `golangci-lint` (0), `mvctl contracts check`, `env check`, `privacy scan testdata/`, `make test` — зелёные; мутанты M0–M22 в копии дерева, контрольный зелёный, остальные красные; копия удалена по точному пути.
- Интеграционные тесты, Docker, стенд `:8888` не трогал, `.env` не открывал, файлы T-310 не менял. Не коммитил.

<!-- dev-log T-305 i2 -->
## developer#1 · T-305 · итерация 2: замечания ревью #1 (Ma-1, Mi-1…Mi-5, N-1…N-3, N-5) · 2026-09-13

Ветка `task/T-305-actions`, папка `.worktrees/T-305`. Статус — `review`. Подробности, таблица мутантов и оценка остаточного риска — карточка `tasks/T-305.md`, раздел «Итерация 2».

- **Ma-1.** Пакет событий действия (`gm.created` при `legacy`, `player.*`, предложение) строится один раз. Если публикация падает на любом событии, пакет остаётся в памяти сервиса по `(player_id, action_key)`, начиная с упавшего события. Повтор ключа публикует остаток с теми же id и байтами, затем записывает ход и ключ; второго `player.*` нет. После взятия блокировки игрока решение идёт на `context.WithoutCancel` со сроком `api.RequestTimeout` через `clock.Timers`. Уход клиента не рвёт пакет и не оставляет `202` без ключа. Пакеты в памяти ограничены (`DefaultPendingLimit = 4096`, у самого близкого к истечению — вытеснение с предупреждением в журнале). Срок жизни — `store.KeyTTL`; уборка — sweeper контекста (`SweepPending`). Контракт не менялся: `503` по-прежнему значит «ключ не записан».
- **N-5.** Ожидание блокировки игрока прерывается контекстом запроса → `503 bus_unavailable` вместо `500`.
- **Mi-1…Mi-5.** Тесты: `X-Actor-Kind: ci` через HTTP до записи шины и `sim` у всех событий пакета; `character_dead` для `abandoned`; тело повтора `4xx` целиком, вместе с `details`; округление `Retry-After` вверх при дробном ожидании; замена фильтра на недопустимый текст → `text_invalid`; уборка лимитера и пакетов sweeper'ом.
- **Nit.** N-1 — doc `FilterText`. N-2 — встреча без `created_at` не считается недоступной; отрицательный `MV_GATEWAY_ENCOUNTER_GRACE` — ошибка старта. N-3 — нечисловой лимит даёт одну ошибку. N-4 и отступление 5 не менялись: они в бэклоге к architect#3.
- **Остаточный риск.** Перезапуск процесса между публикациями пакета теряет пакет в памяти, и повтор публикует действие заново с новым id. Закреплено тестом `TestARestartBetweenPublicationsBuildsTheActionAgain`. Оценка риска — в карточке. Правило для КД §5.5 вносит architect#3.
- **Проверки.** `go build`/`vet`, `go test -short ./...`, e2e, `golangci-lint` (0), `mvctl env check`, `contracts check`, `make test` — зелёные. Мутанты R2, R3, R6–R9 ревьюера и 9 новых прогнаны в копиях дерева: контрольный зелёный, остальные красные. Копии удалены по точным путям.
- Интеграционные тесты, Docker и стенд `:8888` не трогал, `.env` не открывал. Не коммитил.

### developer#2 · T-311 · `flow` и `render` · 2026-09-13

Ветка `task/T-311-bot-flow-render` от ce7b35a (кончик эпика с T-310, T-318, правилом depguard `cmd-telegram-bot`, contracts v0.14). Подробности — раздел «Выполнение (developer)» в `tasks/T-311.md`. Статус — `review`.

- **Сделано.**
  - `cmd/telegram-bot/internal/render`: `notice.go` — четыре константы T-318 §1, перенесены программой, 2 341 символ; `help.go`; тексты онбординга и `/forget`; `ErrorText` по кодам КД §10.5; клавиатуры; `Delivery` с пометками нарратива и нарезкой больше 4 096 единиц UTF-16.
  - `cmd/telegram-bot/internal/flow`: FSM онбординга, TTL 15 мин на `shared/clock`; кэш `chat_id → player_id` на 1 ч; игровые команды с `action_key`; `/forget` на любом шаге; `Reset`, `StepOf`.
  - `internal/gateway/client`: `forget_incomplete` не повторяется, `APIError.RetryAfter`, doc `ForgetResult` без совета про `Resolve` (C-08 v1.5, T-456).
  - `sender.ReplyPolicy` (M-2 ревью T-310).
  - `access/gate_test.go` на `client.New` (ревью T-310).
- **Решения.**
  - Согласие ставится только кнопкой в `awaiting_consent`, а этот шаг — только после успешной отправки уведомления.
  - После `consent` — повторный `resolve`.
  - Отметка `forget_incomplete` по чату различает «конец того же забвения» и «нечего удалять».
  - Golden-тексты — литералы в тестах; полный текст уведомления — длина и SHA-256.
  - Мутанты текста из DoD — постоянный тест.
- **Отклонения.**
  - Р-2 A, Р-3 A, Mi-5 против КД §10.3 — по решениям оркестратора.
  - Свободный текст в `ready` → список команд по US-008, а не `say` по КД.
  - Клиент не повторяет `forget_incomplete` — по C-08 v1.5, а не по строке DoD из приёмки T-303.
  - TTL — константы, без переменных окружения.
  - Облако (T-320) и `/group` (T-353) не делались.
- **Мутанты.** Копия в scratch, без `-overlay`, контрольный первым. 15 мутантов `flow`, `render`, клиента и `sender`: 14 красных. M4a выжил в первом проходе — тест усилен шлюзом и отправителем, не смотрящими на `ctx`, теперь красный. Выжил M4b: проверка `ctx` в `reply` дублирует `Handle` и `fail`. Копия и скрипты удалены по точным путям.
- **Проверки.**
  - `go build ./... && go vet ./...` — 0.
  - `go test -short -count=1 ./cmd/telegram-bot/... ./internal/gateway/...` — ok; `updates` запустился напрямую, без `Access is denied`.
  - `golangci-lint run ./...` — 0 issues.
  - `mvctl env check` — 0.
  - `make test` — exit 0, без `-race` (нет cgo); покрытие `flow` 87,1 %, `render` 92,1 %.
- **Открытые вопросы.** КД §10.3 «текст → say» против US-008. Одна функция правила имени для бота и T-306. Строка DoD о повторе `forget_incomplete` клиентом.
- Telegram API, Docker, стенд `:8888`, `.env` не трогал. `shared/*`, `.golangci.yml`, `go.mod`, контракты и КД не менял. Не коммитил.

## tech-writer#1 · T-317 · README gateway и бота, runbook «ротация токена», сверка `.env.example` · 2026-09-13

*В паре с developer#2 (занят T-311); вопросы к нему — в отчёте.*

- `internal/gateway/README.md`: назначение, состав пакетов, таблица статуса реализации по коду дерева (T-301…T-305 сделаны, T-306…T-309 и `shared/testkit/gateway` — ещё нет), как поднять локально (`--contexts=all --bus=memory` или `--contexts=gateway` на kafka; `--bus=memory` требует `--contexts=all` — проверено по `cmd/multiverse/main_test.go`), режимы `live`/`replay` по факту кода (`context.go`: в `replay` выключены лимитер и фоновая уборка; полное дочитывание журнала и переключение в `live` — ещё T-309), переменные `MV_GATEWAY_*`/`MV_WORLD_ID`/`MV_MINIO_*`, состав `/health`, таблица из 4 маршрутов, реально смонтированных `GatewayRouter` (`resolveLink`, `consentLink`, `forgetLink`, `postAction` — по `api/router.go`), порядок middleware по `api/middleware.go`.
- `cmd/telegram-bot/README.md`: явно зафиксировано, что `main.go` в дереве нет (T-312 не выполнена) — собрать и запустить бинарник сейчас нельзя; таблица готовых пакетов (T-310) и отсутствующих (`flow`/`render` — T-311, `deliver`/`main` — T-312). Получение токена у `@BotFather`, `/setjoingroups` → Disable, allow-list (fail-closed), профиль compose `bot` (обе формы запуска из `Docs/ops/runbook.md`), переменные `MV_TELEGRAM_*` (включая несвязанные с compose `MV_TELEGRAM_ACTION_KEY_SALT`/`MV_TELEGRAM_COMMANDS_PER_MIN` — открытый пункт T-463 части «в»), таблица кодов выхода `updates.ExitCode` (0/1/3) с разбором `409 Conflict`, раздел «Данные игроков» — правила оператора У-1, У-2, У-4, У-5 из карточки T-318 (профили `memory`/`legacy`, копии `gateway-data`, срок хранения копий и архивов ≤ 30 дней — T-463 ещё `todo`, `mvctl report` — после T-131/T-132). Оба README получили пункт о публикации портов только на `127.0.0.1` (правило 2 `compose-lint`, SEC-13).
- `Docs/ops/runbook.md` §6 переписан: явная пометка текущего статуса (T-312 не выполнена), процедура ротации токена по шагам (`@BotFather` → `/revoke`/`/token` → `.env` → `make up PROFILES=...,bot` или прямой `docker compose -f … -f docker-compose.bot.yml up -d telegram-bot` → проверка отсутствия токена в `logs --tail=50` → проверка webhook при подозрении на `409`), чек-лист, правила оператора У-1/У-2/У-4/У-5 (ссылка на README и карточку T-318), правила из приёмки T-310 (`/setjoingroups` Disable, потеря одного обновления при `SIGTERM`, лимиты отказов и `bot_denied_total`). Только раздел §6 — секции 1–5, 7, 8 не трогал.
- Сверка `.env.example` ↔ `shared/env/vars.go` для `MV_GATEWAY_*` и `MV_TELEGRAM_*`: расхождений нет, все объявленные переменные обеих групп присутствуют в примере с тем же именем; `go run ./cmd/mvctl env check` — `74 variables declared, compared with .env.example`, код возврата 0. Разница дефолтов `MV_TELEGRAM_GATEWAY_URL` (`http://127.0.0.1:8088` в манифесте и `http://gateway:8088` в `.env.example`) — не расхождение: пример держит адрес имени сервиса compose, манифест — адрес процесса на хосте.
- Не трогал: код, `.env.example`, `contracts.md`, `tasks.md` эпика, `.claude/*`, `.mcp.json`, `.qwen/*`, `Docs/user-stories/`. Docker и стенд LLM `:8888` не трогал, `.env` и «— копия.env» не открывал. Не коммитил.
- Открытые вопросы — оркестратору (см. отчёт): (1) правка `CLAUDE.md` — строка статуса `cmd/telegram-bot` без `main.go`, предложением, не сделано; (2) T-463 часть «в» (`MV_TELEGRAM_ACTION_KEY_SALT`/`MV_TELEGRAM_COMMANDS_PER_MIN` не переданы в `docker-compose.bot.yml`) и часть «а» (ротация архивов/копий ≤ 30 дней) — обе ещё `todo`, runbook и README ссылаются на них как на условие, а не как на факт.

### tech-writer#1 · T-317 · итерация 2 по ревью #1 (Ma-1, Ma-2, Mi-1…Mi-11, N-1…N-8) · 2026-09-13

Ветка `task/T-317-gateway-bot-docs`, папка `.worktrees/T-317`. Статус — `review`. Подробности — карточка `tasks/T-317.md`, раздел «Итерация 2».

- **Ma-1.** `internal/gateway/README.md`: `/health.links_store`/`gateway_store` описаны как «всегда `ok`, пока контекст запущен» (реальная проверка БД — T-309, component §11.4), по `context.go:460-461`; уточнено, что `bus` появляется в ответе только со значением `fail`.
- **Ma-2.** `cmd/telegram-bot/README.md` и `Docs/ops/runbook.md` §6: проверка webhook переписана на новый токен, ввод через `read -rs`/`Read-Host -AsSecureString`, `curl -K -` (stdin, без токена в URL/argv/истории); шаг «проверить старым токеном» убран; добавлен запрет выкладывать вывод `docker compose config`/`docker inspect telegram-bot`.
- **Mi-1…Mi-11.** По тексту ревью, каждый факт сверен с кодом: описание `/forget` (сжатие в том же вызове, `Retry-After: 5`, порядок снятия отметки — `links/forget.go`, `handlers/links.go`, `store/retention.go`); `shared/testkit/gateway` убран из таблицы «отсутствует»; `export MV_GATEWAY_DATA_DIR=...` перед командами запуска; правило depguard `cmd-telegram-bot` вместо «правило не принято»; вводная бота приведена к КД §2; пример `PROFILES=memory,bot` дополнен условием У-2; `COMPOSE_ENV_FILES` перед прямыми командами `docker compose`; проверка присутствия сервисов через `docker ps --filter label=...` (список дополнен `memory`, `narrative-orchestrator`); чек-лист ротации — на `make health` вместо несуществующей команды бинарника; дублирующий хвост §6 убран, предупреждение `PROFILES=` перенесено в шаг 3; шаг 2 и чек-лист дополнены обновлением копий `.env` вне репозитория. Mi-11 — сухой прогон runbook §6 и открытый пункт согласования с devops зафиксированы в карточке.
- **N-1…N-8.** Проверка токена в логе бота — по всему логу (`grep -cE`), не по хвосту; формулировка `/revoke` нейтрализована; путь `api/gateway.openapi.yaml` уточнён («в корне репозитория») и дополнен задачами I2; добавлено исключение Windows для прав каталога и исправлен порядок PRAGMA (`incremental_vacuum` + `wal_checkpoint(TRUNCATE)`); формулировки без обращения к автору и без «трекера задачи»/несуществующего «отчёта задачи»; добавлены кликабельные ссылки на карточку T-318 (оба README, runbook) и на КД `gateway-and-bot.md` (runbook); поле «Ветка» карточки приведено к фактическому имени; фраза У-1 про хранение записей в бэкапах игры добавлена в оба места.
- **Передано, не вносил сам:** строки для `CLAUDE.md` (карта каталогов, «Статус кода») и предложения по DoD T-311/T-312 из рекомендаций ревьюера — записаны в карточке, раздел «Передать»; код, `.env.example`, `CLAUDE.md`, `contracts.md`, `tasks.md` эпика не трогал.
- **Проверки.** `go run ./cmd/mvctl env check` — `74 variables declared, compared with .env.example`, код 0. Относительные ссылки в правленых файлах проверены отдельным скриптом в scratch (файл, указанный оркестратором, отсутствовал; написал свой, прогнал, удалил по точному пути) — все ссылки резолвятся.
- Docker и стенд LLM `:8888` не трогал, `.env` и «— копия.env» не открывал. Не коммитил, не делал `git add`.

## developer#1 · T-306 · `characters`, `GET /v1/worlds`, `GET /v1/players/{id}`, `session` + `turns`, аналитика C-10 · 2026-09-13

Ветка `task/T-306-characters-sessions`, папка `.worktrees/T-306`. Статус — `review`. Подробности, итог по DoD, мутанты и вопросы — карточка `tasks/T-306.md`, раздел «Выполнение (developer)». Не коммитил.

- **Новые пакеты.** `session` — сессии scope в `gateway.db`: открытие первым действием, `idle` через 30 мин действием или sweeper'ом, `forget`/`leave`/`death` через `End`, парность `started/ended` (событие, не принятое шиной, откатывает строку). `turns` — трекер ходов вместо `actions.MemoryTurns`: резерв `seq` при `Begin`, запись при `Accepted`, `rejected` сразу, `completed`/`degraded` после ack последнего адресата, `timeout` по дедлайну 60 с. `characters` — `POST /v1/characters` с ожиданием факта 2 с, `202 creating`, дедлайном 60 с, идемпотентностью `(link_id, action_key)`, фильтром имени через `FilterText(KindName)`; `GET /v1/players/{id}`, статус персонажа для `resolve`.
- **API.** Смонтированы `listWorlds`, `createCharacter`, `getPlayer`; OpenAPI описывает поведение трёх операций. `api.ValidCharacterName` — одно правило имени для шлюза и бота: выражение бота, 2–32, строго без двух пробелов подряд и без комбинирующих знаков (поручение оркестратора).
- **Окно отметки сжатия `links.db`** (строка приёмки T-303) закрыто счётчиком поколений удалений; тест с принудительным порядком.
- **Переменные** (КД §11.3): `MV_GATEWAY_SESSION_IDLE`, `MV_GATEWAY_TURN_TIMEOUT`, `MV_GATEWAY_CHARACTER_WAIT`, `MV_GATEWAY_CHARACTER_DEADLINE` — нужна отметка tech-lead#1.
- **Фикстура** `internal/gateway/testdata/analytics/solo-30.jsonl` — запасной путь (согласия tech-lead#1 на `testdata/analytics/` нет); уведомление EPIC-005 — в карточке.
- **Отступление от поручения.** Схемы C-10 требуют `session.id`, `participants[].entity.id`, `entity.entity.id`, а `Publish` проверяет схему: без этих полей аналитика не публикуется вовсе. Пишутся только обязательные поля, `scope` и имена — нет. Решение по R2-M-1 A для событий шины — вопрос оркестратору и system-architect.
- **Проверки.** `go build`/`vet`, `go test -short ./...`, пакет `updates` через `go test -c`, e2e, `golangci-lint` (0), `mvctl env check` (78), `contracts check`, `privacy scan`, `make test` — см. карточку. Мутанты — в копиях дерева, контрольный первым, копии удалены по точным путям.
- Интеграционные тесты, Docker и стенд `:8888` не трогал, `.env` не открывал.

### developer#2 · T-312 · deliver и main бота · 2026-09-13

Ветка `task/T-312-bot-deliver-main`, папка `.worktrees/T-312`, от кончика `epic/EPIC-004-gateway-bot` (aa83086). Статус — `review`. Подробности — карточка `tasks/T-312.md`, раздел «Выполнение (developer)».

- **`internal/deliver`.** Цикл long-poll (`limit=100`, `wait=25s`) → отправка по одной в порядке шлюза → один ack на ответ. Ack по типу ошибки `Sender`: отправлено/`ErrBlocked`/`ErrChatNotFound`/`ErrRejected` — да; `ErrUnavailable`, неизвестная, отмена — нет; `ErrUnauthorized` — нет и остаток ответа не отправляется. Доставка без маршрута Telegram — ack без отправки (открытый вопрос). Курсор только в памяти; неудачный ack повторяется перед следующим опросом; при остановке — последний ack до 5 с; пауза после ошибки опроса 1…30 с.
- **`main`.** `run` с подкомандой `health [--url]` (умолчание из `MV_TELEGRAM_HEALTH_ADDR`, пустой хост → `127.0.0.1`), `/health` со счётчиками `access.Counters`, логгер поверх `privacy.NewHandler` для всех пакетов, отдельные клиенты: отказ — `NewBestEffortTelegram` 5 с; `flow` → шлюз 6 с и один повтор; цикл доставки → шлюз 35 с; `flow` — `ReplyPolicy`, цикл — `DefaultPolicy`. Выход `updates.ExitCode`: `409` → 3, `401` при `getMe` и `getUpdates` → 1, сигнал → 0; ошибки конфигурации → 1, командная строка → 2. `docker-compose.bot.yml` не менялся.
- **Перенос** теста импортов в `cmd/telegram-bot/imports_test.go`. **Документы:** README бота (статус, «Как устроен процесс», `/health`, коды выхода) и runbook §6 (врезка статуса, чек-лист `make health`, таймауты, состояние в памяти); шаги 4–5 ротации — за T-390.
- **Не сделано:** замена `flow.ValidName` на `api.ValidCharacterName` — T-306 не слита в ветку эпика, пункт переходит в первую задачу бота после её слияния.
- **Проверки.** Unit: `deliver` 15, `main` 13. Мутанты в копии дерева в scratch, без `-overlay`, контрольный зелёный первым: 20 красных, 1 эквивалентный. `go build ./... && go vet ./...`, сборка exe в scratch (удалён), `go test -short -count=1 ./cmd/telegram-bot/... ./internal/gateway/client/...` — 11 ok, `golangci-lint run ./...` — 0 issues, `mvctl env check` — 74, код 0, `make test` — код 0, 52 пакета ok, без FAIL; без детектора гонок (нет cgo, как предупреждает цель — гонки проверяет CI на Linux); пороги покрытия internal/state 93,9 %, internal/mechanics 96,5 %, internal/replay 100 %.
- Docker, стенд LLM `:8888`, `.env` не трогал; запросов к Telegram и шлюзу нет (только httptest); не коммитил, `git add` не делал.

## developer#1 · T-307 · `outbox`, long-poll `deliveries`, `consumer` для `combat.decided`/`narrative.output` · 2026-09-14

Ветка `task/T-307-outbox-deliveries`, папка `.worktrees/T-307`. Статус — `review`. Подробности, итог по DoD, мутанты и вопросы — карточка `tasks/T-307.md`, раздел «Выполнение (developer)». Не коммитил; фикстура перенесена `git mv`.

- **`outbox`** (новый пакет): `Enqueue` в транзакции consumer, идемпотентно по выведенному id доставки; `Lease` — голова очереди на игрока, одна в лизинге; `Ack` принимает поздний ack своего лизинга, пока доставку не взял другой клиент (дополнение оркестратора по ревью #1 T-312); `ReleaseExpiredLeases` не стирает `leased_by`; `Expire`, `DropForPlayer` (первый хук `/forget`), `Purge`; long-poll без соединения БД на звонке платформы и пробуждении раз в секунду; тексты правил с golden-файлами (`merge=binary` правилом в каталоге `testdata/golden`).
- **Consumer:** доставки на `combat.decided`, `entity.updated` (`move|rest|loot`, смерть, перемещение группы), открытие встречи (идемпотентно по встрече и переходу — Mi-2 T-304), отказ State на перемещение и отдых (игрок по цепочке предложения, `actions.ProposalID`), `narrative.output` в порядке топика; шаги ходов T-306 подключены.
- **HTTP:** `pollDeliveries`, `ackDeliveries`, `streamDeliveries (501)`; дедлайны соединения обычных операций в `timeout`, long-poll — в обработчике и завершается при остановке сервера процесса; платформа клиента — `telegram-bot → telegram` (КД §5.1).
- **Строки приёмки T-306:** ход без адресатов завершается при нарративе; публикация `turn.completed` со сроком (вариант «со сроком», риск — в карточке); срок у чтения `pending_characters` в `characters.Status`; README шлюза обновлён.
- **Фикстура:** `internal/gateway/testdata/analytics/solo-30.jsonl` → `testdata/analytics/solo-30.jsonl` (`git mv`, содержимое без правки, 33 строки). Перегенерация — `go test -short -count=1 -run TestSolo30Fixture ./internal/gateway/turns/ -update`; прогон с `-update` в этой задаче дал тот же файл. **Уведомление EPIC-005 (T-130):** фикстура на согласованном месте.
- **Переменные:** `MV_GATEWAY_DELIVERY_LEASE=30s`, `MV_GATEWAY_DELIVERY_TTL=24h` — нужна отметка tech-lead#1.
- **Отступления:** id доставки `d-<UUID v5>` вместо `d-<ULID>`; «одна в лизинге» строже SQL КД §8.2; ожидание long-poll по настенным часам в любом режиме; лизинг работает и в replay (КД §11.2 — вопрос).
- **Проверки:** `go build`/`vet`, `go test -short ./...`, e2e, `golangci-lint` (0), `mvctl env check` (80), `contracts check`, `privacy scan`, `make test` (exit 0, флака не было). Мутанты M1–M24 — все красные, контрольный первым, копия удалена по точному пути.
- Интеграционные тесты, Docker и стенд `:8888` не трогал, `.env` не открывал.

## developer#1 · T-307 · итерация 2 · ревью #1 (0/1/2/3) · 2026-09-14

Ветка `task/T-307-outbox-deliveries`, папка `.worktrees/T-307`. Статус — `review`. Подробности — карточка `tasks/T-307.md`, «Итерация 2». Не коммитил.

- **Ma-1:** адресаты хода — только уникальные игроки `recipients[]` со связкой. Consumer передаёт это число в `turns.OnNarrative`, при `0` ход завершается сразу. Способ проще отметки `dropped` через `OnDelivered`: одно число закрывает адресата без связки, повторённого адресата и не-игрока. Тесты (а) без связки, (б) повтор, (в) никто не достижим.
- **Mi-1:** `ci-harness` получает доставки `telegram` — условно до решения system-architect (T-308). В prod без `ci-harness` в `MV_GATEWAY_CLIENT_IDS` — `403`. Тест.
- **Mi-2:** тест `absence`, `round_seq`, `fallback_reason` доставки нарратива в строке и в ответе long-poll; R6 красный.
- **N-2:** тест `Expire` отличает `expires_at` от `created_at`. **N-3:** лизинг не начинается после истёкшего срока ожидания (тест на 20 раундов); ограничение лизинга дедлайном записи — в бэклог. **N-1** — T-352/T-353.
- **Проверки:** `build`/`vet`, `go test -short ./...`, e2e, `golangci-lint` (0), `env check` (80), `contracts check`, `privacy scan`, `make test` (exit 0, флака не было). Мутанты: K и 9 — все красные, копия удалена по точному пути.
- Интеграционные тесты, Docker и стенд `:8888` не трогал, `.env` не открывал.
