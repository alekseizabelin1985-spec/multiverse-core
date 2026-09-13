# Ревью EPIC-004

Формат записи: задача, номер ревью, дата, экземпляр ревьюера; границы ревью; вердикт; что проверено; замечания по серьёзности (Critical / Major / Minor / Nit) в виде «файл:строка — суть — предлагаемая правка». Записи добавляются, старые не удаляются.

## T-301 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-3)

### Границы ревью

Ветка `task/T-301-openapi-dto-client`, папка `.worktrees/T-301`. Изменения не закоммичены, все файлы новые (`git status`):
- `api/gateway.openapi.yaml`;
- `internal/gateway/api/{dto.go,errors.go,router.go,openapi_test.go,errors_test.go,router_test.go}`;
- `internal/gateway/client/{client.go,longpoll.go,client_test.go,longpoll_test.go}`;
- `tasks/T-301.md`, `dev-log.md`.

Задача проверялась в сокращённом составе по сверке 2026-09-13 (раздел T-301 в `tasks.md` ветки эпика 53f1ea2): схемы C-04/C-10 и реестр сделаны в EPIC-001 и не менялись. Чужие файлы, `go.mod`, `.golangci.yml`, схемы и реестр не тронуты.

Основания:
- `contracts.md` v0.10: C-04 v1.3, C-06, C-08 v1.1–v1.3, C-10 v1.1;
- `analysis/api-contracts.md` §1;
- `components/gateway-and-bot.md` §5, §6, §10.4–10.5;
- решения ревизии 4 system-architect (журнал `develop` 2026-09-13; черновик T-444 — `contracts.md` v0.11, ADR-001 доп. п. 5; черновик T-445 — `.golangci.yml`).

### Вердикт

**ПРИНЯТЬ** — Critical 0 · Major 0 · Minor 7 · Nit 6.

Спецификация, DTO, таблица кодов и клиент соответствуют C-08 v1.3 и КД. Отклонения от §5.3, §5.6 и §6 обоснованы в карточке и потребителей плана не ломают. Тест сверки доказателен в заявленных пределах: мутанты автора A1–A23 не перепроверялись поштучно, свои мутанты ниже. Замечания касаются дыр в тестах и ловушек для будущих обработчиков и потребителей.

Mi-1, Mi-2 и Mi-5 дешевле закрыть до поставки в `develop`: DTO и спецификацию берут другие команды. Остальное можно внести в DoD T-303, T-308 и T-310/T-311. Решение за tech-lead#3.

### Прогоны (go1.26.8 windows/amd64, golangci-lint 2.13.2, рабочая папка T-301)

| Команда | Результат |
|---|---|
| `go build ./... && go vet ./...` | 0 |
| `gofmt -l internal/gateway` | пусто |
| `go test -short -count=1 ./internal/gateway/...` | `api` ok, `client` ok |
| `golangci-lint run ./internal/gateway/...` | 0 issues |
| то же с `--config` черновика T-445 (правило `internal-gateway-client`, решение 1e) | 0 issues |
| `go list -deps ./internal/gateway/client` (модуль) | только `internal/gateway/api`, `shared/clock` — клиент листовой |
| `go run ./cmd/mvctl contracts check` | 65 types, 8 topics, 58 schema files, 0 |

### Мутанты ревьюера

Копия дерева (`git ls-files -co --exclude-standard` без `services/`, `Docs/`, `.claude/`) лежала в scratch в каталоге с точным именем, без `-overlay`. Базовый прогон копии был зелёным. Каждая мутация — точная замена (`count == 1`), после неё файл восстанавливался со сверкой sha256. В конце шесть изменяемых файлов копии сверены с рабочей папкой — равны. Копия удалена по сохранённому точному пути.

| # | Мутант | Результат |
|---|---|---|
| R0 | контрольный: синтаксическая ошибка в `router.go` | **красный** (сборка) |
| R1 | спецификация: `GroupView.leader_id` без `null` (сторона YAML; у автора — сторона Go) | **красный**: `TestDTOsMatchOpenAPISchemas` |
| R2 | спецификация: `cloud_enabled` → `cloudEnabled` | **красный**: `TestDTOsMatchOpenAPISchemas` |
| R3 | спецификация: `ActionPending.status` enum `[accepted]` | **зелёный** → Mi-5 |
| R4 | `group_full` удалён согласованно из `errors.go` и из `x-error-codes`/описания `Conflict` | **зелёный** → Mi-5 |
| R5 | роутер монтирует `resolveLink` как `GET`, операция вычеркнута из `notYetMounted` | **красный**: `TestOpenAPIRoutesMatchRouter` |
| R5p | то же с верным `POST` (позитивный) | `TestOpenAPIRoutesMatchRouter` зелёный; красные только `router_test`, которые рассчитывают на пустой `NewRouter()`, — ожидаемо |
| R6 | второй конструктор маршрутов (как в T-303) с неверным путём; тест остаётся на `api.NewRouter()` | **зелёный** → Mi-6 |
| R7 | клиент повторяет `429` | **зелёный** → Mi-4 |
| R8 | транспортная ошибка клиента содержит тело запроса (`external_id`) | **зелёный** → Mi-4 |
| R9 | повтор уходит с другим `action_key` | **красный**: `TestUnavailableIsRepeatedThreeTimesWithTheSameActionKey` |
| R10 | `CloseRound` без повторов | зелёный — поведение не закреплено, см. Mi-7 |

### Проверено по пунктам задания

**1. OpenAPI против C-04/C-08.**
- Операции: 17 `operationId` §5.6 плюс `adminLlmUsage` (в §5.6 без имени) и `health`. Пути, методы, `x-idempotency` у `createCharacter`/`postAction`, `x-actor-kind: [ci]` у служебных — верно.
- Коды: 36 кодов таблицы = 33 кода §1.6 и C-08 v1.1 + `consent_incomplete` (§1.2), `internal` (§5.1), `character_alive_exists` (§1.3). Статусы совпадают с §1.6.
- `202 pending`: `postAction` отдаёт `202 oneOf [ActionAccepted, ActionPending]`, `status` в двух ветках не пересекается; `200` — `GroupView` (C-08 v1.1). Верно.
- `409 no_leader`: есть в `Conflict`, в описании — «checked before not_leader»; в `postAction` — то же. Верно.
- `leader_id`: `[string, 'null']` и обязательное; `CharacterState.group` ссылается на тот же `GroupView` (C-08 v1.2). Верно.
- `llm.cloud_enabled`: `WorldSummary.llm: WorldLLM{cloud_enabled}` обязательное; провайдера, URL и ключей нет (SEC-21). Верно.
- `external_platform`: `enum: [telegram]` в `ResolveRequest`/`ConsentRequest`/`ForgetRequest`/`CreateCharacterRequest`. В `DeliveryRoute` (ответ) — строка без enum. Для MVP-1 так правильно. Расширение enum запроса для второго бота совместимо (открытый вопрос автора 2 — согласен, снимать enum не нужно).
- `admin`: три операции C-06 с `x-reserved: EPIC-003 T-239`, тег с пометкой Reserved, ответ `default`. Совладение по C-06 соблюдено, тест не даёт снять пометку незаметно. Неточность описания тега — N-1.
- `character_dead` — «also for abandoned»; `CharacterState.status` содержит `abandoned`; `ResolveResponse.character_status` без `abandoned` (после `/forget` — `none`). Верно.
- Расхождение: `ForgetResponse` — Mi-2.

**2. Доказательность теста сверки.**
- Сверка DTO со схемами (свойства, `required`, nullable, вид типа, `$ref` на свой тип) ловит расхождение с обеих сторон (R1, R2; A4, A5, A17, A22 автора).
- Коды ↔ спецификация — в обе стороны со статусами.
- Роутер ↔ спецификация: неверный метод при монтировании краснеет (R5), смонтированная операция не может остаться в списке.
- Пределы: `notYetMounted` не вынуждает вычеркнуть операцию, пока тест смотрит на пустой `api.NewRouter()` (R6, Mi-6). Согласованное выпадение кода §1.6 не ловится (R4), константы статусов действия со спецификацией не связаны (R3) — Mi-5.

**3. Клиент.**
- Повторы — только сетевые ошибки и `503` (`client.go:279-284`), тело сериализуется один раз (`client.go:243-249`), поэтому `action_key` тот же (R9). `4xx` и прочие `5xx` не повторяются (A10, A15 автора). Дыра в тестах по `429` — Mi-4.
- Паузы через `clock.Timers.After`, отмена контекста прерывает паузу. В тестах фейковые таймеры, реального ожидания нет: тесты пакета идут около 10 мс каждый.
- Long-poll не повторяется (`longpoll.go:41`), `409 poll_in_progress` отдаётся как `APIError`. Ограничения `limit ≤ 100`, `wait_ms ≤ 25000` зажимаются. `DefaultHTTPTimeout` 35 с больше серверного предела long-poll `wait_ms + 5 с ≤ 30 с` (§5.1) и согласован с решением 6 ревизии 4 (`runtime.SetDeadlines`/`ShuttingDown`, T-446). Отмена контекстом во время висящего опроса тестом не закреплена — Mi-4.
- Горутин клиент не заводит. Тело ответа закрывается `defer` и читается с пределом 4 МиБ. Таймер паузы при отмене останавливается. Утечек нет.
- `X-Client-Id` ставится на каждый запрос и совпадает с `{client_id}` пути по построению (`longpoll.go:57-59`).
- Логов в клиенте нет. `APIError` несёт только `code`/`message`/`details`/`X-Request-Id` из ответа сервера. Тексты ошибок содержат метод и путь, где есть `player_id`/`group_id`/`scope_id`/`client_id`, но нет внешних ID и тела. Сейчас верно, но не закреплено тестом — Mi-4.

**4. Отклонения от КД §5.3, §5.6, §6.** Все обоснованы.
- `ActionResult`: `202 pending` не помещается в `(ActionAccepted, *GroupView)`.
- `Client.Group`: нужен для опроса после `pending`.
- `Deliveries`/`Ack` без `clientID`: SEC-12, один источник. Вызов бота §10.4 `Deliveries(ctx, "telegram-bot", …)` превращается в `Client.ClientID = "telegram-bot"`.
- `Ack` → `AckResponse`: нужен `unknown[]`.
- `WorldSummary`: имя `WorldView` занято.
- `Health = runtime.Status`: `/health` принадлежит процессу.

Потребители плана ещё не написаны (T-308, T-310, T-311). Сигнатуры им подходят: Harness (T-308) обернёт `Action` в `Act`, бот (T-311) разберёт `ActionResult` по одному полю. Ловушка нулевого `Client{}` для тех, кто пишет по §6, — Mi-3.

**5. forbidigo и depguard.** `time.Now`, `time.After` и `os.Getenv` в новом коде нет. `internal-gateway` чист. Правило `internal-gateway-client` из черновика T-445 на этом дереве чистое: `client` импортирует из модуля только `api` и `shared/clock`. `api` в production-коде зависит только от stdlib, в тестах — от `schemas`, `shared/contracts`, `shared/runtime` и `yaml.v3`, который уже прямая зависимость `go.mod`.

**6. Объём.** 3543 строки:
- спецификация — 949;
- тесты — 1576 (из них `openapi_test.go` — 783);
- код — 1018.

Это больше нормы M (≤ ~600 строк), но вес дают сама спецификация и табличные сверки. Генерация DTO (`oapi-codegen`) отвергнута КД §5.3, и вместо неё стоит рефлексивная сверка. Дублирование «YAML ↔ Go» здесь намеренное и охраняется тестом. Лишнего кода, который стоило бы убрать, не нашёл. Для планирования: пункты «OpenAPI обновлён» в T-303…T-306 и T-352 станут малыми правками. Не замечание.

### Замечания

#### Critical
Нет.

#### Major
Нет.

#### Minor

**Mi-1. `internal/gateway/api/dto.go:111, 118, 192, 210, 312, 325, 342` — nil-срез уходит в JSON как `null` в обязательных массивах.**
- Суть. `worlds`, `regions`, `members`, `npcs`, `deliveries`, `unknown` и `sessions` в спецификации объявлены как `type: array` без `null` и входят в `required`. `encoding/json` кодирует nil-срез как `null`. Комментарий `dto.go:309-310` обещает «empty array, not null», но структура этого не обеспечивает.
- Где сработает. `AckResponse{Acked: n}` без неизвестных id — самый частый ответ `ack` — уйдёт с `"unknown": null`. Сверка `TestDTOsMatchOpenAPISchemas` смотрит форму, а не кодирование, поэтому не поймает. Клиенту Go это безразлично, но харнесс и внешний валидатор увидят нарушение контракта.
- Правка. Добавить рефлексивный тест: кодировать нулевое значение каждого DTO из `dtoOfSchema` и проверять, что обязательные массивы не `null`. Чинить одним из двух способов: `MarshalJSON` у ответов нормализует nil в `[]`, или конструкторы ответов в T-303. Тест лучше завести сейчас: он охранит и будущие поля.

**Mi-2. `api/gateway.openapi.yaml:117-118` против `:673-678` — `ForgetResponse` противоречит сам себе и `api-contracts.md` §1.2.**
- Суть. Описание операции и §1.2 говорят «без связки — `200 {deleted: false}`», а схема требует `player_id_detached`. DTO всегда кодирует поле как `null` — это совместимо со схемой, но не с текстом.
- Правка. В описании `forgetLink` написать `{deleted: false, player_id_detached: null}`. Правку примера §1.2 передать system-analyst/architect#3 (см. «Для architect#3», п. 6).

**Mi-3. `internal/gateway/client/client.go:65-79` (поля `Timers`, `Backoff`) — нулевой `Client` молча отключает повторы.**
- Суть. КД §6 описывает клиента литералом `Client{BaseURL, ClientID, ActorKind, HTTP}`. Разработчик бота, который пишет по §6, получит клиента без повторов: нулевой `Backoff` = 0 повторов. Обещание §10.5 «клиент повторил с тем же `action_key` до 3 раз» не выполнится, и ни один тест бота этого не заметит.
- Правка (одно из двух).
  - (а) Нулевой `Backoff` трактовать как `DefaultBackoff`, а «без повторов» задавать явно (`Retries: -1` или переменная `NoRetry`). `TestZeroBackoffRepeatsNothing` переписать на явную форму.
  - (б) Оставить как есть, но в doc пакета и в §6 (через architect#3) записать: создавать только через `New`. В DoD T-310 и T-308 добавить тест «клиент создан через `New`».

**Mi-4. `internal/gateway/client/client_test.go:223-245, 387-398`, `longpoll_test.go` — три поведения клиента не закреплены тестами (R7, R8 зелёные).**
- (а) Невповтор `4xx` проверен только на `409` и `502`. Повтор `429` (R7) проходит, хотя бьёт по rate limit SEC-11. Правка: табличный тест по статусам 400, 403, 404, 409, 413, 422, 429, 500, 501, 502 — ровно один запрос и ни одной паузы.
- (б) Нет теста, что ошибка `Resolve`/`Consent`/`Forget`/`CreateCharacter` не содержит `external_id`. Нужны оба случая: исчерпанные сетевые повторы и `APIError`. Бот будет логировать эти ошибки (SEC-01/02, DoD-common «тест, где указано»). Утечка тела в текст ошибки (R8) проходит. Правка: тест с `external_id = "tg-4242"` проверяет `err.Error()` и `%+v` у `APIError`.
- (в) Отмена контекстом во время висящего long-poll не проверена. Правка: сервер держит ответ, `cancel()`, проверить `errors.Is(err, context.Canceled)`, один запрос, возврат без ожидания.

**Mi-5. `internal/gateway/api/openapi_test.go:369-390, 685-731` — два расхождения, которые сверка не ловит (R3, R4 зелёные).**
- (а) `api.ActionStatusAccepted`/`ActionStatusPending` не сверены с enum `ActionAccepted.status`/`ActionPending.status`. Клиент различает `202` именно по этой константе (`client.go:205`). Правка: две строки `equalSets` в `TestOpenAPIEnumsMatchGoAndEventSchemas`.
- (б) Код §1.6 можно согласованно убрать из `errors.go` и спецификации: `TestErrorTableHasTheCodesOfC08` закрепляет поимённо только дополнения C-08, а DoD требует наличия всех кодов. Правка: закрепить в тесте полный перечень §1.6 с C-08 v1.1–v1.3 (36 кодов со статусами), чтобы потеря кода была видна в диффе теста.

**Mi-6. `internal/gateway/api/openapi_test.go:237-242, 255` — `notYetMounted` не мешает «забыть» операцию.**
- Суть. Тест сверяет спецификацию с `api.NewRouter()`, который по построению пуст. Настоящий конструктор маршрутов появится в T-303 и может монтировать что угодно по неверному пути — тест останется зелёным (R6), пока кто-то не поменяет строку 255. Список ни к чему не привязан: операция может остаться в нём до конца эпика.
- Правка.
  - Сделать `notYetMounted` картой `operationId → задача` (`"resolveLink": "T-303"`, `"postAction": "T-305"`, `"pollDeliveries": "T-307"`, … — раскладка в «Предложениях в бэклог», п. 1), чтобы пропуск был виден по номеру.
  - Предложить tech-lead#3 строку DoD в T-303: «тест сверяет спецификацию с конструктором маршрутов шлюза, а не с `api.NewRouter()`».
  - Во все задачи с пунктом «OpenAPI обновлён»: «свои операции вычеркнуты из `notYetMounted`». Автор предложил то же в бэклог (п. 1) — поддерживаю, это закрывает замечание.

**Mi-7. `internal/gateway/client/client.go:135` (`Forget`), `:222-229` (`CloseRound`) — повтор после потерянного ответа меняет исход.**
- `Forget` при сетевой ошибке повторяется. Если первая попытка удалила связку, повтор вернёт `{deleted: false}`, а `player_id_detached` потеряется. Бот скажет «нечего удалять» после настоящего удаления (UC-031, SEC-26).
- `CloseRound` после успешного закрытия вернёт `409 no_open_round`, и харнесс увидит провал. В комментарии это признано, но вызывающему не видно.
- Правка.
  - `CloseRound` — `retry: false`: служебный вызов `ci`, харнесс сам решает, повторять ли.
  - `Forget` — в doc метода записать, что `deleted: false` после транспортной ошибки означает «исход неизвестен». Либо вернуть признак повтора (например, `ForgetResult{…, Repeated bool}`), чтобы T-311 проверил итог через `Resolve`.
  - Строку о трактовке добавить в DoD T-311.

#### Nit

- **N-1. `api/gateway.openapi.yaml:45-49`.** Тег `admin` описан как «Proxy of /v1/admin/*», но `/v1/admin/sessions` (`:377`) и `/v1/admin/links/{player_id}` (`:138`) — собственные маршруты шлюза и в прокси не уходят. Правка: «Proxy of /v1/admin/agents* and /v1/admin/llm/usage». Та же неточность есть в C-06 — см. «Для architect#3», п. 7.
- **N-2. `internal/gateway/client/client.go:205-210`.** Любой `status`, кроме `pending`, декодируется как `Accepted`: неизвестное значение пройдёт молча. Правка: `accepted` → `Accepted`, иначе `ErrUnexpectedStatus`.
- **N-3. `internal/gateway/client/client.go:295-298`.** Ошибка `http.NewRequestWithContext` (кривой `BaseURL`) повторяется трижды с паузами как сетевая. Правка: вернуть её сразу, без повтора.
- **N-4. `internal/gateway/client/client.go:85, 287`.** Хвостовой `/` у `BaseURL` срезается только в `New`; литерал с `http://host/` даст `//v1/...`. Правка: срезать в `once`.
- **N-5. `api/gateway.openapi.yaml:73-74`.** Заголовок `X-Request-Id` объявлен только у `200` в `resolveLink`, хотя middleware §5.1 ставит его на каждый ответ. Правка: убрать отовсюду до T-303 или описать единообразно.
- **N-6. `internal/gateway/api/errors.go:128`.** Схема `Error` соответствует Go-типу `ErrorResponse`, а `api.Error` — значение ошибки обработчика. Имена легко спутать. Правка: одна строка в doc `Error`/`ErrorResponse` о соответствии.

### Для architect#3 (записать в `components/gateway-and-bot.md` через develop)

1. §6, клиент:
   - `Action` → `ActionResult{Status, Accepted|Pending|Group}`;
   - добавлен `Group(ctx, groupID)`;
   - `Deliveries(ctx, after, limit, wait)` и `Ack(ctx, ids) (AckResponse, error)` без `clientID` — берётся из `Client.ClientID` (SEC-12);
   - поля `Timers clock.Timers`, `Backoff`, конструктор `New`, `APIError.RequestID`, `ErrUnexpectedStatus`;
   - трактовка повторов «3 повтора, паузы 200/400/800 мс, потолок 1,6 с»;
   - long-poll внутри клиента не повторяется;
   - по итогу Mi-3 — правило создания клиента;
   - §10.4 — вызов `Deliveries` без `"telegram-bot"`.
2. §5.3, DTO:
   - `ResolveResponse.CharacterStatus string` (enum с `none`, а не `*string`);
   - `CreateCharacterResponse.Created *bool`, `Status` с `omitempty`;
   - `GroupView.LeaderID *string`, `Position *PositionRef`, `State`;
   - `Delivery.Route *DeliveryRoute`, `NarrativeEventID`;
   - новые типы `WorldsResponse/WorldSummary/RegionSummary/WorldLLM`, `ActionPending`, `AckResponse`, `RoundCloseResponse/ClosedRound`, `AdminSessions/AdminSession`;
   - правило тегов «без `omitempty` — обязательное, указатель без `omitempty` — nullable».
3. §5.6:
   - 18 операций (17 `operationId` + `health`), имя `adminLlmUsage`;
   - схемы `ErrorBody`, `ActionPending`, `TurnRef`, `ClosedRound`, `AdminSession`, `WorldSummary`/`RegionSummary`/`WorldLLM`;
   - ответы `PayloadTooLarge`, `UnprocessableEntity`, `InternalError`, `NotImplemented` с расширением `x-error-codes`;
   - тест сверяет роутер через `notYetMounted`, а не равенством множеств на пустом роутере.
4. §11.4 и `api-contracts.md` §1.8: схема `/health` в спецификации = `runtime.Status{status, details}`. Расширенная форма (`deps`, `agents_by_level`, `snapshot`) ложится в `details` (T-309).
5. `/v1/admin/sessions` и `DELETE /v1/admin/links/{player_id}` — тег `service`, не раздел `admin`.
6. `api-contracts.md` §1.2: пример «без связки» — `{deleted: false, player_id_detached: null}` (Mi-2; владелец — system-analyst).
7. C-06: «gateway проксирует `/v1/admin/*`» — уточнить до `/v1/admin/agents*` и `/v1/admin/llm/usage`. Собственные admin-маршруты шлюза проксироваться не должны. В процессе `--contexts=all` прокси и маршруты `swarm`/`llm` делят один mux, повторная регистрация шаблона уронит процесс паникой — это решать в задаче прокси (N-1, риск).
8. Открытый вопрос автора 1 (заголовок C-08) закрыт решением 8(б) ревизии 4 (T-444, C-08 v1.4). Ссылки T-301 на C-08 v1.3 после слияния T-444 не устаревают: API `/v1` в v1.4 не менялся.

### Связь с ревизией 4 system-architect

- **1e.** Клиент листовой — подтверждено линтером с черновиком T-445 и `go list -deps`.
- **6.** Таймауты long-poll (`SetDeadlines`, `ShuttingDown`, T-446) — на клиента не влияют, `DefaultHTTPTimeout` согласован.
- **5.** `gm.created` — T-305, T-301 не касается.
- **8(в).** В черновике T-444 (`contracts.md` v0.11, C-04 v1.4, строка 390 и таблица решений п. 8) записано: «`group_move` … Схемы сужает владелец в T-301». В дереве `group_move` ещё есть в `schemas/events/group.{created,joined,left,disbanded}.v1.json`. Карточка T-301 честно говорит «схемы не менялись», и по действующей v0.10 это верно, поэтому замечанием к этой итерации не считаю. Вопрос оркестратору и tech-lead#3 — в отчёте.

### Предложения в бэклог

1. DoD T-303 и задач с пунктом «OpenAPI обновлён»: тест сверяется с конструктором маршрутов шлюза, свои операции вычеркнуты из `notYetMounted` (Mi-6; совпадает с п. 1 автора). Раскладка операций по задачам `tasks.md`:
   - T-303 — `resolveLink`, `consentLink`, `forgetLink`;
   - T-305 — `postAction`;
   - T-306 — `listWorlds`, `createCharacter`, `getPlayer`;
   - T-307 — `pollDeliveries`, `ackDeliveries`, `streamDeliveries`;
   - T-352 — `getGroup`;
   - T-354 — `closeRound`;
   - T-356 — `adminForgetLink`, `adminSessions` и прокси `adminTick`/`adminAgents`/`adminLlmUsage` (текст — EPIC-003 T-239).
1а. **DoD T-303, строка «`openapi_test.go` зелёный: маршруты `links`, `/health` из `router.go` есть в `api/gateway.openapi.yaml`»** противоречит решению T-301 (отклонение 7 карточки). `/health` регистрирует `shared/runtime.NewHTTP`, и тест теперь краснеет, если роутер шлюза его смонтирует (`servedByProcess`). Строку нужно поправить: «`/health` — маршрут процесса, в `router.go` не монтируется». Правка tech-lead#3.
2. DoD T-310 и T-311: клиент создаётся через `New` (Mi-3); `deleted: false` после транспортной ошибки у `/forget` проверяется через `Resolve` (Mi-7).
3. DoD T-303: ответы шлюза не отдают `null` в обязательных массивах (Mi-1), если тест не заведён в T-301.
4. Makefile `test` из PowerShell (п. 2 автора) — поддерживаю, владелец EPIC-001.
