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
