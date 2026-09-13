# `internal/gateway` — контекст платформы «шлюз»

Точка входа игрока в мир по HTTP (C-08): связки внешних аккаунтов (`links`),
приём действий игрока, проекция мира, читаемая с шины. Реализует
`runtime.Context` (`Name/DependsOn/Start/Stop/Health`) и `runtime.Routes`;
своего HTTP-сервера у пакета нет — маршруты монтируются на общий mux процесса
(`shared/runtime`, слушает `MV_CORE_ADDR`), а `GET /health` отдаёт сам процесс,
агрегируя `Health()` всех контекстов.

Подробное поведение — `Docs/dev-team/architecture/components/gateway-and-bot.md`,
решения — ADR-018 (бот), ADR-019 (SQLite, схемы, миграции). Здесь — только то,
что уже есть в коде этого каталога.

## Статус реализации (2026-09-13)

Из состава компонента в коде есть только часть — остальное числится за
задачами EPIC-004, ещё не принятыми на момент этой страницы:

| Готово (в этом каталоге) | Пакет | Задача |
|---|---|---|
| Связки (`resolve/consent/forget`, `link_id`, `RouteFor`) | `links/` | T-303 |
| Хранилища и миграции обеих баз | `store/`, `migrations/` | T-302 |
| HTTP-слой: роутер, `errors.go`, middleware | `api/` | T-301, T-303 |
| Проекция мира и диспетчер шины | `readmodel/`, `consumer/` | T-304 |
| Приём действий игрока (`POST /v1/players/{id}/actions`) | `actions/`, `handlers/` | T-305 |

| Ещё не реализовано (эти пакеты в дереве отсутствуют) | Что войдёт | Задача |
|---|---|---|
| Персонажи, `GET /v1/worlds`, `GET /v1/players/{id}`, сессии, ходы, аналитика | `characters`, `session`, `turns` | T-306 |
| Исходящие доставки игроку (outbox, long-poll `deliveries`) | `outbox/` | T-307 |
| Снапшот шлюза, режим `--mode=replay` до конца, полный состав `/health` | `snapshot/` | T-309 |
| `FakeGateway` и HTTP-обвязка рядом с `Harness` v0 (`shared/testkit/gateway` уже есть) | — | T-308 |

Поэтому: маршруты API — только четыре (см. ниже), `GET /v1/worlds` и выдача
доставок ещё не отвечают, а `--mode=replay` уже не запускает лишние таймеры,
но не дочитывает журнал до конца и не переключается в `live` (это делает
T-309). Статус обновляется вместе с T-306…T-309.

## Пакеты

| Пакет | Назначение |
|---|---|
| `context.go` | `Context` — сборка контекста: открывает `links.db`/`gateway.db`, поднимает `links.Store`, читает снапшот State, догоняет проекцию по журналу, подписывается на шину, монтирует маршруты, ведёт фоновую уборку (`sweep`) |
| `links/` | `Store` — интерфейс и SQLite-реализация связок: `Resolve`, `Consent`, `AttachPlayer`, `RouteFor`, `Forget`/`ForgetByPlayer`, заявки на персонажа, `Compact` |
| `store/` | Открытие баз (`OpenLinks`/`OpenGateway`), миграции (`goose` из `embed.FS` в `migrations/`), права каталога/файлов, интервалы уборки (`SweepInterval`, `KeyTTL`, `LinksCompactInterval`) |
| `migrations/links`, `migrations/gateway` | SQL-схемы обеих баз (goose, `0001_init.sql` — полная схема I1+I2) |
| `api/` | `GatewayRouter` (реестр маршрутов по `operationId`), цепочка middleware (`Chain`), таблица кодов ошибок (`errors.go`), DTO |
| `readmodel/` | Типизированная проекция мира (World/Region/NPC/CharacterState/Group/Encounter) поверх `shared/entity` v2; загрузка из снапшота State, `AwaitFact`, `Hash`/`Cursor` |
| `consumer/` | Подписка на топики шины, дедуп по `event_id`, применение фактов к проекции |
| `actions/` | Валидация и публикация действий игрока, идемпотентность по `action_key`, лимит темпа, `InputFilter` |
| `handlers/` | HTTP-обработчики поверх `links`/`actions` |
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
  уборка (`sweep`: удаление просроченных заявок и ключей, повторная попытка
  отложенного сжатия `links.db`, компакция раз в час) запущена.
- **`replay`** — лимит темпа выключен (`actions.Limiter` не создаётся), фоновая
  горутина уборки не запускается. Полный сценарий переигровки — дочитывание
  журнала до `Journal.End()` и переключение в `live` — ещё не реализован
  (T-309); на этом этапе `--mode=replay` для шлюза означает только
  «таймеры и лимит выключены», а не «шлюз восстановлен из записи».

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
| `MV_WORLD_ID` | мир, снапшот которого загружается при старте |
| `MV_MINIO_ENDPOINT`, `MV_MINIO_ACCESS_KEY`, `MV_MINIO_SECRET_KEY`, `MV_MINIO_USE_SSL` | объектное хранилище снапшотов; без ключей клиент не создаётся, снапшот считается отсутствующим (не ошибка) |
| `MV_GM_PATH` | `agent` или `legacy` — используется при публикации действий (`agent` — целевой путь) |

## `/health`

Отдаёт процесс (`GET /health` на `MV_CORE_ADDR`), а не сам пакет — это
маршрут `shared/runtime`, не роутера шлюза. Поле шлюза внутри общего ответа —
`runtime.Status` с деталями:

- `links_store`, `gateway_store` — сейчас всегда `ok`, пока контекст запущен
  (`context.go:460-461`); настоящая проверка обеих БД — ещё T-309 (component
  §11.4);
- `projection` — `ok` / `missing` (у нового мира ещё нет снапшота — это не
  авария) / `stale`;
- `mode` — `live`/`replay`;
- `links_compaction: pending` — отложенное сжатие `links.db` после `/forget`
  ещё не завершилось (общий статус в это время `degraded`);
- `projection_error` — код причины, если снапшот State не загрузился
  (хранилище настроено неверно, недоступно, битый снапшот);
- `bus` — ключ появляется в ответе, только если подписка на шину упала, и
  тогда его значение — `fail`; в остальных случаях ключа `bus` в ответе нет
  (`bus: ok` не бывает).

Статус `fail` — до `Start` и после `Stop`; иначе `ok` или `degraded` (см.
условия выше). Не все поля component §11.4 присутствуют; полный состав
появится вместе с T-309.

## API (маршруты, смонтированные `GatewayRouter`)

Сейчас — только четыре операции; полный список из `api/gateway.openapi.yaml`
в корне репозитория шире, остальные операции появляются вместе с T-306/T-307
(I1) и T-352/T-354/T-356 (I2):

| Метод | Путь | operationId |
|---|---|---|
| `POST` | `/v1/links/resolve` | `resolveLink` |
| `POST` | `/v1/links/consent` | `consentLink` |
| `DELETE` | `/v1/links` | `forgetLink` |
| `POST` | `/v1/players/{player_id}/actions` | `postAction` |

Порядок middleware (`api.Chain`): `request_id` + журнал запроса → `recover` →
допуск клиента (`X-Client-Id`/`X-Actor-Kind`) → лимит тела (64 КиБ,
`Content-Type: application/json`) → лимит темпа действий → защита от
параллельного long-poll на клиента → таймаут (5 с, кроме long-poll-операций).
Для операций `resolveLink`, `consentLink`, `forgetLink` и (в будущем)
`createCharacter` в журнал запроса пишутся только `request_id` и `code`, без
тела и без прочих полей ошибки (SEC-01/02, внешний ID никогда не попадает в
лог).

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
