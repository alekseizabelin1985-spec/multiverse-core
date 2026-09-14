# `internal/gateway` — контекст платформы «шлюз»

Точка входа игрока в мир по HTTP (C-08): связки внешних аккаунтов (`links`),
персонажи, приём действий игрока, сессии и ходы, проекция мира, читаемая с
шины, и исходящие доставки игроку (outbox, long-poll). Реализует
`runtime.Context` (`Name/DependsOn/Start/Stop/Health`) и `runtime.Routes`;
своего HTTP-сервера у пакета нет — маршруты монтируются на общий mux процесса
(`shared/runtime`, слушает `MV_CORE_ADDR`), а `GET /health` отдаёт сам процесс,
агрегируя `Health()` всех контекстов.

Подробное поведение — `Docs/dev-team/architecture/components/gateway-and-bot.md`,
решения — ADR-018 (бот), ADR-019 (SQLite, схемы, миграции). Здесь — только то,
что уже есть в коде этого каталога.

## Статус реализации (2026-09-14)

Из состава компонента в коде есть только часть — остальное числится за
задачами EPIC-004, ещё не принятыми на момент этой страницы:

| Готово (в этом каталоге) | Пакет | Задача |
|---|---|---|
| Связки (`resolve/consent/forget`, `link_id`, `RouteFor`) | `links/` | T-303 |
| Хранилища и миграции обеих баз | `store/`, `migrations/` | T-302 |
| HTTP-слой: роутер, `errors.go`, middleware | `api/` | T-301, T-303 |
| Проекция мира и диспетчер шины | `readmodel/`, `consumer/` | T-304 |
| Приём действий игрока (`POST /v1/players/{id}/actions`) | `actions/`, `handlers/` | T-305 |
| Персонажи, `GET /v1/worlds`, `GET /v1/players/{id}`, сессии, ходы, аналитика C-10 | `characters/`, `session/`, `turns/` | T-306 |
| Исходящие доставки: outbox, long-poll и ack, тексты правил, эффекты consumer | `outbox/`, `consumer/`, `handlers/` | T-307 |
| Снапшот шлюза, догон журнала до `Journal.End()`, восстановление сессий и ходов при старте, полный `/health` | `snapshot/`, `consumer/`, `health.go`, `snapshots.go` | T-309 |

| Ещё не реализовано (эти пакеты в дереве отсутствуют) | Что войдёт | Задача |
|---|---|---|
| `FakeGateway` и HTTP-обвязка рядом с `Harness` v0 (`shared/testkit/gateway` уже есть) | `gatewaytest/` | T-308 |
| Группы, раунды, групповые доставки (`group.*`, `round.opened`, `player.said`), служебные и admin-маршруты | `groups/`, `rounds/` | T-352…T-356 |

Прокси `/v1/admin/*` к `core` (T-356) ещё нет, поэтому в `/health` нет поля
`core_admin`.

## Пакеты

| Пакет | Назначение |
|---|---|
| `context.go` | `Context` — сборка контекста: открывает `links.db`/`gateway.db`, поднимает `links.Store`, читает снапшот State, догоняет проекцию по журналу, подписывается на шину, монтирует маршруты, ведёт фоновую уборку (`sweep`) |
| `links/` | `Store` — интерфейс и SQLite-реализация связок: `Resolve`, `Consent`, `AttachPlayer`, `RouteFor`, `Forget`/`ForgetByPlayer`, заявки на персонажа, `Compact` |
| `store/` | Открытие баз (`OpenLinks`/`OpenGateway`), миграции (`goose` из `embed.FS` в `migrations/`), права каталога/файлов, интервалы уборки (`SweepInterval`, `KeyTTL`, `LinksCompactInterval`) |
| `migrations/links`, `migrations/gateway` | SQL-схемы обеих баз (goose, `0001_init.sql` — полная схема I1+I2) |
| `api/` | `GatewayRouter` (реестр маршрутов по `operationId`), цепочка middleware (`Chain`), таблица кодов ошибок (`errors.go`), DTO |
| `readmodel/` | Типизированная проекция мира (World/Region/NPC/CharacterState/Group/Encounter) поверх `shared/entity` v2; загрузка из снапшота State, `AwaitFact`, `Hash`/`Cursor` |
| `consumer/` | Подписка на топики шины, дедуп по `event_id`, применение фактов к проекции; эффекты в транзакции `gateway.db`: доставки (`Deliveries`), шаги ходов (`turns.OnMechanics`, `OnNarrative`), заявки персонажей |
| `actions/` | Валидация и публикация действий игрока, идемпотентность по `action_key`, лимит темпа, `InputFilter` |
| `characters/` | Создание персонажа предложением State и ожидание факта (`201`/`200`/`202 creating`), статус персонажа, `GET /v1/players/{id}` |
| `session/` | Сессии scope: открытие первым действием, простой `MV_GATEWAY_SESSION_IDLE`, `analytics.session.started/ended` |
| `turns/` | Ходы: `accepted` → механика → нарратив → ack последнего адресата, `timeout`, `analytics.turn.completed` |
| `outbox/` | Очередь доставок: `Enqueue` (идемпотентно), `Lease` (голова очереди на игрока, одна в лизинге), `Ack`, уборка (`Sweep`), long-poll (`Service.Serve`, `Notifier`), тексты правил (`Mechanics`, `EncounterOpened`, `Died`, `Refused`) |
| `handlers/` | HTTP-обработчики поверх `links`/`actions`/`characters`/`outbox` |
| `client/` | Go-клиент HTTP API шлюза (C-08); потребители — бот (с T-311) и `FakeGateway` (T-308) |

## Как поднять локально

Вне compose умолчание `MV_GATEWAY_DATA_DIR` (`/data`) — корень файловой
системы: на Linux/macOS без root старт падает, на Windows создаётся `\data` в
корне диска. Каталог задают явно, вне репозитория (runbook §2):

```bash
export MV_GATEWAY_DATA_DIR="$HOME/.multiverse/gateway"
```

Без Docker, на шине в памяти, вместе со всеми остальными контекстами (шлюз
нельзя поднять в одиночку на `--bus=memory` — процесс требует `--contexts=all`
для этой шины):

```bash
go run ./cmd/multiverse serve --contexts=all --bus=memory
curl http://127.0.0.1:8090/health
```

Только шлюз, на реальной шине (Redpanda, `--bus=kafka` — умолчание), нужен
поднятый брокер и MinIO (или без `MV_MINIO_*` — тогда снапшот считается
отсутствующим, а не ошибкой):

```bash
go run ./cmd/multiverse serve --contexts=gateway
curl http://127.0.0.1:8090/health
```

В compose шлюз — сервис `gateway` (`docker-compose.yml`), адрес наружу —
`127.0.0.1:8088` (см. `Docs/ops/runbook.md`, разделы 1 и 8).

## Режимы `live`/`replay`

Режим задаёт `MV_MODE` (или флаг `--mode`, если передан явно — правило одно на
процесс, `Docs/ops/runbook.md`, «Режим, вид шины и адрес процесса»). Шлюз
сейчас реагирует на режим частично:

- **`live`** — лимит темпа действий (`actions.Limiter`) активен, фоновая
  уборка (`sweep`) запущена. Раз в минуту: просроченные заявки персонажей и
  ключи действий, метки `processed_events`, повтор отложенного сжатия
  `links.db`, снятие со связки персонажей без факта к дедлайну, `timeout`
  ходов, закрытие простаивающих сессий (`idle`); в outbox — снятие истёкших
  лизингов с повторной выдачей, `dropped` для доставок старше
  `MV_GATEWAY_DELIVERY_TTL`, удаление завершённых доставок старше 7 дней.
  Раз в час — сжатие `links.db`.
- **`replay`** — лимит темпа выключен (`actions.Limiter` не создаётся), фоновая
  горутина уборки не запускается, аналитика не публикуется. Long-poll и ack
  доставок работают: ожидание long-poll идёт по настенным часам в любом
  режиме, как дедлайны соединения (C-01 v1.8), а лизинг — по часам процесса.

В обоих режимах старт один: открыть БД, загрузить проекцию из снапшота State,
закрыть ходы с истёкшим дедлайном (`timeout`) и сессии, простаивающие
`MV_GATEWAY_SESSION_IDLE` (`idle`, в `replay` без аналитики), дочитать журнал
каждого топика до его `Journal.End()` — от курсора снапшота State и от курсора
эффектов `gateway.db` — и только затем подписаться на шину. «Хвост» после
`End` приходит через подписку, повтор эффектов гасят курсор эффектов и
`processed_events`.

Снапшот шлюза (`snapshot/`, C-14) пишется в `snapshots-{world}/gateway/`
**только в `live`**: при `Stop` (после остановки подписок, уборки, сжатия
`links.db` и писателя, пока шина открыта) и после завершения сессии (концы во
время записи дают один следующий снапшот). Одна запись ограничена
`SnapshotWriteBudget` (30 с): зависшее хранилище даёт `/health snapshot:
write_failed`, а в `Stop` не отнимает срок у сжатия `links.db`. Порядок: объект `{ts}-{seq}.json`, указатель `latest.json`,
событие `snapshot.created component=gateway`; хранятся пять последних. В
`replay` снапшот не пишется и не объявляется; проверка последнего снапшота при
старте идёт в обоих режимах. Без `MV_MINIO_*` снапшотов нет. Мир ещё не известен
проекции — снапшот пропускается с `Warn`; мир известен, но без `laws_version` —
`Error` и `/health snapshot: no_laws_version`.

`projection=stale` снимается событием `snapshot.created component=state` своего
мира (`readmodel.RepairFromStateSnapshot`). В подписке — синхронно в
обработчике, до сдвига курсоров, и только для анонса, которого проекция ещё не
видела. В догоне — один раз, по последнему такому анонсу, после чтения журнала
до `End` и до подписок: ранние снапшоты не читаются, и зависшее хранилище
стоит старту один срок починки, а не срок на каждый анонс. Шлюз читает объект
по `snapshot.key` события
(срок `StaleRepairBudget` 5 с), сверяет `state_hash` и заменяет каждую
`stale`-сущность копией из снапшота, если версия в нём не ниже. Остальные
сущности и курсор не меняются; неудача — `Warn`, `stale` остаётся, событие не
уходит в `dead_letters`. В `live` и `replay` правило одно.

## Переменные окружения

Читаются только через объявления `shared/env` (`shared/env/vars.go`); полные
описания и умолчания — там же и в `.env.example`.

| Переменная | Смысл |
|---|---|
| `MV_CORE_ADDR` | адрес HTTP-сервера процесса (общий для всех контекстов, не только шлюза) |
| `MV_GATEWAY_DATA_DIR` | каталог `links.db`/`gateway.db` (том `gateway-data` в compose; вне compose — задать явно, см. runbook §2) |
| `MV_GATEWAY_CLIENT_IDS` | allow-list `X-Client-Id` |
| `MV_GATEWAY_ACTOR_KIND_CLIENTS` | кому разрешён `X-Actor-Kind: ci\|sim` |
| `MV_GATEWAY_RATE_ACTIONS_PER_MIN`, `MV_GATEWAY_RATE_ACTIONS_BURST` | лимит темпа действий игрока (в `replay` не действует) |
| `MV_GATEWAY_INPUT_FILTER` | фильтр текста игрока; в MVP-1 допустимо только `noop`, другое значение — ошибка старта |
| `MV_GATEWAY_ENCOUNTER_GRACE` | сколько встреча без агента ждёт до `encounter_unavailable` |
| `MV_GATEWAY_SESSION_IDLE` | простой сессии scope до `end_reason=idle` (30m) |
| `MV_GATEWAY_TURN_TIMEOUT` | ожидание доставки нарратива хода до `turn.completed status=timeout` (60s) |
| `MV_GATEWAY_CHARACTER_WAIT`, `MV_GATEWAY_CHARACTER_DEADLINE` | ожидание факта персонажа до `202 creating` (2s) и срок, после которого персонаж без факта снимается со связки (60s); ожидание короче дедлайна, иначе отказ старта |
| `MV_GATEWAY_DELIVERY_LEASE` | сколько выданная доставка ждёт ack до повторной выдачи (30s) |
| `MV_GATEWAY_DELIVERY_TTL` | сколько доставка остаётся `pending` до `dropped` (24h) |
| `MV_WORLD_ID` | мир, снапшот которого загружается при старте |
| `MV_MINIO_ENDPOINT`, `MV_MINIO_ACCESS_KEY`, `MV_MINIO_SECRET_KEY`, `MV_MINIO_USE_SSL` | объектное хранилище снапшотов; без ключей клиент не создаётся, снапшот считается отсутствующим (не ошибка) |
| `MV_GM_PATH` | `agent` или `legacy` — используется при публикации действий (`agent` — целевой путь) |

## `/health`

Отдаёт процесс (`GET /health` на `MV_CORE_ADDR`), а не сам пакет — это
маршрут `shared/runtime`, не роутера шлюза. Поле шлюза внутри общего ответа —
`runtime.Status` с деталями:

- `links_store`, `gateway_store` — `ok`/`fail` по чтению файла БД (`health.go`):
  не чаще раза в 10 с (в `live` — по часам процесса, в `replay` — по настенным)
  и не дольше 100 мс ожидания единственного соединения; если соединение занято,
  остаётся последнее известное значение;
- `sessions_active`, `rounds_open`, `outbox_pending`, `outbox_oldest_age_s` —
  активные сессии, открытые раунды, доставки `pending` и возраст старейшей из
  них в секундах (читаются тем же чтением БД; возраст — по часам процесса, теми
  же, что пишут `created_at`, в `replay` тоже);
- `projection` — `ok` / `missing` (у нового мира ещё нет снапшота — это не
  авария) / `stale`;
- `mode` — `live`/`replay`;
- `links_compaction: pending` — отложенное сжатие `links.db` после `/forget`
  ещё не завершилось (общий статус в это время `degraded`);
- `projection_error` — код причины, если снапшот State не загрузился
  (хранилище настроено неверно, недоступно, битый снапшот);
- `bus` — `ok`, или `fail`, если подписка на шину упала (статус `degraded`);
- `snapshot` — `corrupted` (указатель или объект снапшота шлюза не сходятся),
  `unreadable` (хранилище не прочиталось при старте), `write_failed`
  (последний снапшот не записан) или `no_laws_version` (мир проекции без
  `laws_version`); снимается успешной записью, без проблем ключа нет.

Статус `fail` — до `Start`, после `Stop` и при `fail` любой из БД; иначе `ok`
или `degraded` (см. условия выше). Поля лежат плоско в `details` шлюза, а не
во вложенном `deps{}` component §11.4; `core_admin` — с T-356.

## API (маршруты, смонтированные `GatewayRouter`)

Полный список из `api/gateway.openapi.yaml` в корне репозитория шире:
остальные операции появляются вместе с T-352/T-354/T-356 (I2).

| Метод | Путь | operationId |
|---|---|---|
| `POST` | `/v1/links/resolve` | `resolveLink` |
| `POST` | `/v1/links/consent` | `consentLink` |
| `DELETE` | `/v1/links` | `forgetLink` |
| `GET` | `/v1/worlds` | `listWorlds` |
| `POST` | `/v1/characters` | `createCharacter` |
| `GET` | `/v1/players/{player_id}` | `getPlayer` |
| `POST` | `/v1/players/{player_id}/actions` | `postAction` |
| `GET` | `/v1/clients/{client_id}/deliveries` | `pollDeliveries` |
| `POST` | `/v1/clients/{client_id}/deliveries/ack` | `ackDeliveries` |
| `GET` | `/v1/clients/{client_id}/stream` | `streamDeliveries` (`501`, резерв E-H) |

Порядок middleware (`api.Chain`): `request_id` + журнал запроса → `recover` →
допуск клиента (`X-Client-Id`/`X-Actor-Kind`) → лимит тела (64 КиБ,
`Content-Type: application/json`) → лимит темпа действий → защита от
параллельного long-poll на клиента → таймаут (5 с и дедлайны соединения по
10 с на чтение и запись, кроме long-poll-операций). Для операций
`resolveLink`, `consentLink`, `forgetLink` и `createCharacter` в журнал
запроса пишутся только `request_id` и `code`, без тела и без прочих полей
ошибки (SEC-01/02, внешний ID никогда не попадает в лог).

## Доставки (outbox)

- **Постановка.** Consumer ставит доставки в транзакции события
  (component §8.1). `combat.decided` — текст правил живым игрокам scope.
  `entity.updated` с `cause ∈ {move, rest, loot}` — игроку; перемещение
  игрока в группе не доставляется, его доставляет факт группы всем живым
  участникам. Смерть персонажа — `kind=system`. Открытие встречи
  (`encounter.started` или `entity.created` встречи) — `world_event`
  участникам, идемпотентно по встрече и переходу. Отказ State
  (`entity.update.rejected`) на перемещение или отдых игрока — `kind=system`
  без кода State. `narrative.output` — адресатам `recipients[]` в порядке
  топика, без перестановок. Доставка без связки пишется сразу `dropped`.
- **Выдача.** `GET …/deliveries?after&limit≤100&wait_ms≤25000`: один long-poll
  на клиента (`409 poll_in_progress`), на игрока — одна доставка в лизинге,
  по порядку `seq`. `route.external_id` подставляется из `links.db` в момент
  ответа и только клиенту платформы связки: в MVP-1 платформа `telegram` есть у
  `telegram-bot` и, условно до решения system-architect, у `ci-harness`
  (`handlers.ClientPlatforms`); в prod `ci-harness` не входит в
  `MV_GATEWAY_CLIENT_IDS` и получает `403 client_unknown`. При остановке процесса
  long-poll сразу отвечает пустым списком.
- **Харнесс и бот делят одну очередь `telegram`.** До решения system-architect
  о платформе `ci-harness` не запускать харнесс (T-308, e2e) против шлюза, у
  которого работает бот (профиль `bot`): харнесс заберёт и подтвердит сообщения
  живых игроков и увидит их внешние ID, а бот попытается отправить сообщения
  тестовых игроков. В dev-стеке `ci-harness` допущен в `MV_GATEWAY_CLIENT_IDS`
  по умолчанию.
- **Подтверждение.** `POST …/deliveries/ack`: подтверждаются доставки, которые
  клиент взял в лизинг, в том числе после истечения лизинга, пока их не взял
  другой клиент; остальные id — в `unknown`. Ack последнего адресата нарратива
  завершает ход; если брокер не принял `analytics.turn.completed` за срок
  публикации, весь ack откатывается, ответ — `503 bus_unavailable`.
- **Пустой ответ раньше `wait_ms`** бывает, когда процесс останавливается.
  Клиенту стоит выдержать паузу перед следующим long-poll (порядка секунды),
  иначе он будет повторять запросы к останавливающемуся процессу.

## Ограничения

- Порт шлюза публикуется наружу только на `127.0.0.1:8088` (`docker-compose.yml`);
  правило 2 `scripts/compose-lint.sh` («каждый опубликованный порт привязан к
  `127.0.0.1`», SEC-13) проверяет это для всех сервисов, не только для шлюза.
- `internal/gateway` не импортирует другие пакеты `internal/*` (правило
  depguard `internal-gateway`, `.golangci.yml`).
- Данные каталога `MV_GATEWAY_DATA_DIR` — единственная копия связок игроков
  (`links.db`) и рабочих данных шлюза (`gateway.db`); каталог и файлы
  создаются с правами `0700`/`0600`, более широкие права — отказ старта
  (кроме Windows, где POSIX-прав нет и проверка пропускается,
  `store/open.go:213-215`) (ADR-019 п. 1). Проверка и починка тома, созданного
  до T-303, — runbook §2, «Том `gateway-data`, созданный до T-303».
- `/forget` удаляет строку связки и сразу сжимает `links.db` в том же вызове
  (`incremental_vacuum` + `wal_checkpoint(TRUNCATE)`). Если сжатие не прошло
  (например, базу держит читатель вне процесса), ответ — `503
  forget_incomplete` с `Retry-After: 5`, а `/health` в это время —
  `degraded` с `links_compaction: pending`. Отметку снимает первый успешный
  повтор `/forget`, уборка раз в минуту (`sweep`), часовое сжатие
  (`LinksCompactInterval`) или следующий рестарт шлюза.
