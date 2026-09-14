# `testkit/gateway` — харнесс игроков: в шину (`Harness`) и по HTTP (`HTTPHarness`)

Пакет играет персонажами фикстур (`testdata/fixtures`, `player-A/B/C`) двумя
способами. Оба остаются: у них разные потребители.

| Тип | Файл | Как действует | Кому нужен |
|---|---|---|---|
| `Harness` (v0) | `harness.go`, `scenario.go` | публикует `player.*` и предложения прямо в `membus`, без HTTP, сессий и `action_key` (C-04 «Заглушка») | тесты EPIC-002/003 и стенд T-400 до готовности шлюза |
| `HTTPHarness` | `http.go` | ходит в настоящий шлюз по HTTP API C-08 клиентом `internal/gateway/client` — тем же, что бот (NFR-092) | e2e шлюза (T-313 и далее), тесты поверх `gatewaytest.FakeGateway` |

API `Harness` v0 (`NewHarness`, `Start`/`Wait`/`Observe`, `CreatePlayer`,
`Enter`, `Leave`, `Look`, `Say`, `Rest`, `Attack`, `Flee`, `Fight`, `Status`,
`Position`, `Version`, `WithLog`, `WithTimeout`) T-308 не меняла. Потребителям
v0 переходить никуда не нужно.

In-process сервер шлюза `FakeGateway` живёт не здесь, а в
`internal/gateway/gatewaytest` (ADR-001, доп. 2026-09-13, п. 5; C-08 v1.4):
общий слой `shared/*` не импортирует `internal/*`. Этот пакет видит из шлюза
ровно `internal/gateway/client` и `internal/gateway/api` (правило depguard
`shared-testkit-gateway`), поэтому его тесты `FakeGateway` не поднимают. Тест,
которому нужны оба, живёт вне `internal/<контекст>` (или в самом
`internal/gateway/gatewaytest`, как проверки T-308).

## `HTTPHarness`

```go
g, _ := gatewaytest.Start(gatewaytest.Config{Bus: bus}) // шлюз на membus, clock.Manual, id из последовательности
defer g.Close()
h, _ := gateway.NewHTTPHarness(gateway.NewCIClient(g.URL), fixtures) // ci-harness, X-Actor-Kind: ci
h.WithClock(g.Clock())

entered, _ := h.RegisterAndEnter(ctx, "player-A", "dark-forest-01")
mech, _ := h.AwaitDeliveryOf(ctx, entered.CorrelationID, "mechanics", 5*time.Second)
text, _ := h.AwaitDeliveryOf(ctx, entered.CorrelationID, "narrative", 5*time.Second)
corr, _ := h.Act(ctx, entered.PlayerID, api.ActionRequest{Type: api.ActionLook})
```

- **`RegisterAndEnter(ctx, fixture, region)`** — `links/resolve` →
  `links/consent` (уведомление, согласие, возраст; `shown_at` — из
  `WithClock`) → `POST /v1/characters` в мире фикстур с именем фикстуры →
  действие `enter` → ожидание, пока `GET /v1/players/{id}` не покажет персонажа
  `alive` в регионе. Внешний ID аккаунта — id фикстуры (`player-A`), это не
  ПДн. Без State на шине (`testkit/state.FakeState` или настоящий) шлюз не
  создаст и не переместит персонажа, и вызов закончится по таймауту
  (`WithTimeout`, по умолчанию 5 с настенного времени). Возвращает `player_id`
  и `correlation_id` входа.
- **`Act(ctx, player_id, req)`** — `POST …/actions`; пустой `action_key`
  заполняется ключом харнесса (`ci-1`, `ci-2`…). Возвращает `correlation_id`
  ответа `202 accepted` или `202 pending`; `200` группы без корреляции — ошибка.
- **`AwaitDelivery(ctx, kind, timeout)`**, **`AwaitDeliveryOf(ctx, corr, kind, timeout)`**
  — long-poll `GET /v1/clients/ci-harness/deliveries` от последнего `cursor`.
  Каждую выданную доставку харнесс сразу подтверждает (`POST …/ack`): шлюз
  держит в лизинге одну доставку на игрока, и следующая приходит только после
  ack. Доставки другого вида остаются во «входящих» и достаются следующим
  вызовом. Таймаут — настенное время (ожидание long-poll идёт по настенным
  часам во всех режимах, C-01 v1.8); по его истечении — `ErrNoDelivery` с
  перечнем того, что пришло. Запрос long-poll не просит ждать дольше
  оставшегося времени и не обрывается посреди ответа.
- **`CloseRound(ctx, scope)`** — `POST /v1/scopes/{scope}/rounds/close`. В I1
  шлюз этот маршрут не монтирует (T-354), и mux отвечает голым `404`; харнесс
  возвращает вместо него ошибку, которая совпадает с `ErrRoundsNotMounted`
  (`errors.Is`) и читается как `*client.APIError` `501 not_implemented`
  (`errors.As`, каждому вызывающему — своя копия). Голый `404` неверного
  базового URL от `404` немонтированного маршрута не отличить, поэтому в
  тексте ошибки остаётся ответ сервера; перевод снимает T-354. Любой другой
  ответ, в том числе `409 no_open_round` будущего маршрута, возвращается как
  есть.

**Один `HTTPHarness` на шлюз.** Харнесс играет всеми фикстурами через один
клиент `ci-harness`, а у всех клиентов с этим id одна очередь доставок: второй
харнесс на том же шлюзе получит `409 poll_in_progress` или заберёт и подтвердит
доставки первого. Вызовы `AwaitDelivery` одного харнесса выполняются по очереди;
срок каждого считается от его вызова. Если ack не прошёл, доставки опроса
остаются в лизинге шлюза до его истечения и ожидание заканчивается ошибкой;
доставки, которые шлюз подтвердил, при `unknown` в ответе остаются во
«входящих».

## Платформа `ci-harness` и общая очередь с ботом

HTTP API MVP-1 создаёт связки только платформы `telegram`, поэтому харнесс
регистрирует фикстуры как аккаунты `telegram`. Доставки `telegram` шлюз выдаёт
клиентам `telegram-bot` и — **условно, до решения system-architect** —
`ci-harness` (`internal/gateway/handlers.ClientPlatforms`, решение 1
оркестратора по T-307). Харнесс видит `route.external_id` своих фикстур.

> **Запрет.** До решения system-architect не запускать харнесс против шлюза, у
> которого работает бот (профиль compose `bot`). Бот и харнесс делят одну
> очередь `telegram`: харнесс заберёт в лизинг и подтвердит сообщения живых
> игроков и увидит их внешние ID, а бот попытается отправить в Telegram
> сообщения тестовых игроков. `FakeGateway` этой проблемы не имеет: у него своя
> шина и свои базы, бот к нему не подключён. Тот же запрет — в README шлюза и в
> `Docs/ops/runbook.md` §6.

Преграда запрещённому сценарию — только allow-list клиентов, и сама она не
стоит: манифест (`shared/env/vars.go`) по умолчанию допускает
`telegram-bot,ci-harness,mvctl`, compose передаёт переменную без своего
значения. **На стенде с ботом и в prod оператор задаёт в `.env`
`MV_GATEWAY_CLIENT_IDS` без `ci-harness`** (например,
`MV_GATEWAY_CLIENT_IDS=telegram-bot,mvctl`) — тогда харнесс получает
`403 client_unknown` (`Docs/ops/runbook.md` §6).

## `gatewaytest.FakeGateway` (кратко)

`internal/gateway/gatewaytest.Start(Config)` поднимает настоящий
`internal/gateway` за процессным HTTP-сервером (`shared/runtime.HTTP`) на
`127.0.0.1:0`: маршруты, затем `Start` контекста, затем сервер — как `serve`.
Умолчания: своя `membus` по всем топикам реестра, `clock.Manual` от
`gatewaytest.Epoch`, идентификаторы `eventbus.SequenceIDs("gw")`
(`player-gw-N`), режим `live`, временный каталог данных, переменные — умолчания
манифеста плюс `Config.Vars` (окружение процесса не читается). `Close`
останавливает сервер (висящий long-poll сразу отвечает пустым списком), контекст,
а шину и каталог — только если создал их сам. Старт и остановка — меньше
секунды, без Docker и сети.

Мир в проекции шлюза появляется из журнала: объектного хранилища у
`FakeGateway` нет, снапшот State считается отсутствующим. Тест публикует
`entity.created` мира и регионов (как State при старте с пустого журнала) и
сеет те же сущности в `FakeState` — пример в
`internal/gateway/gatewaytest/gatewaytest_test.go` (`newWorld`).

В режиме `live` действует лимит темпа шлюза (30 действий в минуту, серия 5) по
часам `clock.Manual`: сценарий дольше пяти действий без `Clock().Advance`
получит `429 rate_limited`. Сценарию можно поднять лимит через
`Config.Vars` (`MV_GATEWAY_RATE_ACTIONS_BURST`).
