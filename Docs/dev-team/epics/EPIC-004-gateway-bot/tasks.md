# Задачи EPIC-004 «Вход игрока: gateway и Telegram-бот» (волны)

Версия 0.1.4 · 2026-09-13 · tech-lead#3 (TEAM-3) · последняя правка — приёмка T-305 (§9); базовая редакция 0.1.2 — редакционная правка под дерево `develop` 1c2ee7e, `contracts.md` v0.10 и gitflow; объём и состав задач не менялись (история версий — §9). Прежняя редакция: 0.1.1 · 2026-09-09 · этап A4 «Планирование», шаг 3 · к объединённому G3.
**Правки сведения 3 внесены tech-lead#1 от имени tech-lead#3** (одна команда по решению пользователя; основание — `architecture/consolidation.md` §14, `contracts.md` v0.4): T-301 (схемы `group.*`/`session.ended`, OpenAPI), T-302 (CHECK `end_reason`), T-314 и T-355 (каскад `/forget`), T-352 (`no_leader`, `leader_id: null`), T-320 (`blocked` → `planned`); §8 З-1…З-6 — «решено», §9 риски 4–5 сняты. Структура подволн и состав задач не менялись.
**Правки ревизии контрактов T-416 внесены tech-lead#1 (2026-09-11; один проход по всем эпикам по решению оркестратора)** — основание `contracts.md` v0.6–v0.7 (C-04 v1.3, C-05 v1.4, C-08 v1.3, Env), ADR-026: T-303 (`MV_CORE_ADDR`, списки `MV_GATEWAY_CLIENT_IDS`/`MV_GATEWAY_ACTOR_KIND_CLIENTS`), T-304 (конец встречи), T-307 (порядок `narrative_output`), T-350/T-351 (конец при открытом раунде), T-356, T-357, T-392 (имена переменных и подкоманды отчёта). Добавленные пункты помечены «(T-416, 2026-09-11)», отменённые — «заменено (T-416)» и не удалены.
**Редакционная правка 2026-09-13 (tech-lead#3; сверка плана с деревом, `journal.md` 2026-09-13).** Основание: дерево `develop` 1c2ee7e (слияние EPIC-001), `contracts.md` v0.10, решения пользователя 2026-09-13 (три команды, контрольные слияния в `develop`, интеграционные прогоны задач). Шапка и DoD-common приведены к gitflow и одному модулю; DoD T-301…T-319 — к дереву (пункты помечены «(сверка 2026-09-13)», заменённое — «заменено (сверка 2026-09-13)»). Реестр статусов из §6 убран: статусы — в карточках `tasks/T-NNN.md` и `state.js`. Строка C-05 п. 1д, по ошибке попавшая в заголовок §0, перенесена в DoD T-304, куда её внёс оркестратор.
Команда: TEAM-3 (tech-lead#3, architect#3, developer#1–#2, code-reviewer#3, qa-engineer#3, tester#3). Первая волна по плану перезапуска трёх команд: developer#1 — T-301, developer#2 — T-302, параллельно.
Ветка эпика: **`epic/EPIC-004-gateway-bot`** от `develop` (1c2ee7e), рабочая папка `.worktrees/EPIC-004`. Каждая задача — своя ветка `task/T-NNN-<slug>` от ветки эпика в своей папке `.worktrees/T-NNN` (gitflow; слаг — в строке «Ветка» задачи). После «принято» tech-lead#3 оркестратор сливает ветку задачи в ветку эпика; при конфликте tech-lead#3 решает, кто из разработчиков его разрешает. Коммиты — `commits=auto`: коммитит оркестратор, исполнители не коммитят. `integration/mvp-1` не используется. *(Сверка 2026-09-13: заменено «`epic/EPIC-004-gateway` от `integration/mvp-1` (одна на весь MVP-1…; коммиты — `commits=ask`, один запрос на подволну)».)*
Диапазон номеров команды: **T-300…T-399** (TEAM-1 — T-001…T-199, TEAM-2 — T-200…T-299). Занято этим документом: T-301…T-320, T-350…T-357, T-390…T-393. Номера новых задач выдаёт оркестратор из общего счётчика `.dev-team.json` (`counters.task`) — тимлид номера не берёт.

Основание: `epics/EPIC-004-gateway-bot/design.md` v0.1 (§3.1 состав I1 — 9 пунктов, §3.2 состав I2 — 7 пунктов, §3.3 заглушки, §3.4 порядок T-a…T-l), `architecture/components/gateway-and-bot.md` v0.2 (§3 структура, §4 данные, §5 API, §6 интерфейсы, §7 потоки, §8 outbox, §9 раунды, §10 бот, §11 безопасность/конфигурация, §15 тесты, §17 журнал изменений), ADR-018/019/020 (с дополнениями), ADR-006/009/010, `architecture/contracts.md` v0.2 (поставляем C-04, C-08, C-10; потребляем C-01, C-02, C-05, C-06, C-14; §17 заглушки; **действующая редакция — v0.10**: C-01 v1.7, C-02 v1.4, C-04 v1.3, C-05 v1.6, C-08 v1.3, C-10 v1.1, C-14 v1.2 — сверка 2026-09-13), `architecture/threat-model.md` (SEC-03, 06, 07, 08, 10, 11, 12, 26 — обязательны в MVP-1), `analysis/api-contracts.md` v0.2 §1, `requirements/{prd,nfr,user-stories}.md` v0.4, `plan/{epics,teams,ownership,decomposition-review}.md`.

---

## 0. Как читать и общий DoD

**Формат задачи:** `T-NNN: название · инкремент · подволна · исполнитель · размер · зависимости · ссылки · описание · DoD`.
Размер: **S** ≤ полдня, **M** ≤ день-полтора (дифф ≤ ~600 строк с тестами). Задач размера L в эпике нет — это правило нарезки (design §3.4).

**DoD-common — входит в каждую задачу разработки, отдельно не повторяется:**

- [ ] Unit-тесты на новый код написаны и зелёные: `go test -short ./internal/gateway/... ./cmd/telegram-bot/...` (затронутые пакеты) — в корне рабочей папки задачи `.worktrees/T-NNN`.
- [ ] `golangci-lint run` чист; `depguard` не нарушен: `internal/gateway/*` не импортирует другие `internal/*` (правило `internal-gateway` в `.golangci.yml`); `cmd/telegram-bot` импортирует из платформенного кода только `internal/gateway/{client,api}` (component §3). **(сверка 2026-09-13)** Отдельного правила depguard для `cmd/telegram-bot` в `.golangci.yml` нет (есть только `cmd-others` — запрет `internal/replay`); правило — по решению system-architect, до него граница бота проверяется тестом импортов (T-310).
- [ ] `go build ./...` и `go vet ./...` зелёные (модуль один, `go.work` нет); если задача вводит теги — и `go vet -tags integration` / `-tags e2e` своих пакетов. *(Сверка 2026-09-13: заменено «`go work sync` не ломается».)*
- [ ] Время и таймеры — только `Deps.Clock`/`Deps.Timers` (`shared/clock`); `time.Now`, `time.After`, `time.NewTimer`, `time.Since` и т. п. запрещены `forbidigo`, в `--mode=replay` таймеры выключены. **(сверка 2026-09-13)**
- [ ] Изменены только пути владения EPIC-004 (`ownership.md` §1). Правка `shared/*`, чужих схем, `go.mod`-версий общих зависимостей — **не делать**, оформить запросом к system-architect через tech-lead#3 и описать в отчёте. Файлы EPIC-001, которые задача прямо называет (`cmd/multiverse/contexts.go` в T-303, объявления в `shared/env/vars.go`, `.env.example`), — только в объёме задачи. Общие документы (`plan/**`, `contracts.md`, `ownership.md`, `components/*`) исполнители не правят: они меняются одним местом в `develop`, нужная правка — в отчёте. **(сверка 2026-09-13)**
- [ ] Новая зависимость в `go.mod`/`go.sum` — с явной пометкой в отчёте (правило `ownership.md`). В дереве `develop` 1c2ee7e нет ни `modernc.org/sqlite`, ни `pressly/goose/v3`, ни `go-telegram/bot`: их вносят T-302 и T-310. Задачи одной подволны не меняют `go.mod` одновременно без предупреждения tech-lead#3. **(сверка 2026-09-13)**
- [ ] Интеграционный прогон, если задача его вводит, — только `go test -tags integration ./<пакеты задачи>/...` на одноразовых контейнерах testcontainers, не больше одного прогона одновременно; после прогона — проверка, что контейнеров testcontainers не осталось. `make test-integration`, `make up/down` и контейнеры владельца не трогать (решение пользователя 2026-09-13). **(сверка 2026-09-13)**
- [ ] Карточка `tasks/T-NNN.md` (секция «Выполнение») и `Docs/dev-team/epics/EPIC-004-gateway-bot/dev-log.md` дополнены в ветке задачи: экземпляр (`developer#K`), решения, отклонения от дизайна, что не сделано.
- [ ] Поведение соответствует критериям приёмки перечисленных в задаче US.
- [ ] Ни одно новое поле/лог/событие не содержит внешнего ID (Telegram user id, username, chat id) — проверка глазами + тест, где указано (SEC-01/02/03).
- [ ] Секретов в коде и тестовых данных нет; `.env.example` синхронизирован с манифестом `shared/env.Declare`, если задача добавляла переменные (NFR-074). **(сверка 2026-09-13)** Переменная объявляется в `shared/env/vars.go` тем изменением, которое вводит её чтение (урок T-408); уже объявленные (`env.GatewayDataDir`, `GatewayClientIDs`, `GatewayActorKindClients`, `CoreAddr`, `CoreURL`, `GMPath`, `TelegramBotToken`, `TelegramAllowedUserIDs`, `TelegramGatewayURL`, `TelegramPollTimeoutS`, `TelegramHealthAddr`) читаются через свои объявления и повторно не объявляются; проверка — `go run ./cmd/mvctl env check`.

**Правило заглушек (design §3.3).** Заглушки `FakeState`, `FixedMechanics` (EPIC-002), `FakeNarrator` (T-220) и `FakeEncounter` (T-219) — EPIC-003, фикстура `testdata/fixtures/snapshots/state/latest.json`, `shared/eventbus/membus` c `Journal.End()` (реализация C-01, EPIC-001) — чужие. TEAM-3 их **не правит**: дефект → запрос владельцу через tech-lead#3, задача уходит в эпик-владелец. `shared/testkit/gateway` (`Harness` v0) — поставка EPIC-004, но его API не меняется (T-308). *(Сверка 2026-09-13: путь фикстуры и `FakeEncounter` — по дереву.)*

**Поставка в `develop` (контрольные слияния).** Задачи с пометкой «**поставка в develop**» (T-301, T-308) нужны TEAM-1/TEAM-2 раньше конца эпика. После приёмки tech-lead#3 и слияния в ветку эпика они попадают в `develop` контрольным слиянием ветки эпика с сохранением ветки — по постоянному разрешению пользователя (2026-09-13): зелёный `make ci`, чистый `make secrets-scan`, все слитые задачи приняты; без тегов, push и `main`; каждое слияние — запись в `journal.md`. Слияние выполняет оркестратор. Так же сливаются точки I1-α, I1 и I2 (§5). *(Сверка 2026-09-13: заменено «Ранний merge… сливаются в `integration/mvp-1` до интеграции эпика… подтверждает tech-lead#1 на G3».)*

---

## 1. Инкремент I1 «соло» (developer#1 — gateway, developer#2 — бот c подволны 1.4)

### T-301: Схемы событий C-04/C-10, скелет OpenAPI, DTO и каркас Go-клиента · I1 · подволна 1.1 · developer#1 · M · **поставка в develop** · **Статус: in-progress**

**Ветка:** `task/T-301-openapi-dto-client` (`.worktrees/T-301`). Карточку `tasks/T-301.md` и `dev-log.md` ведёт исполнитель в ветке задачи.
**Зависит от:** EPIC-001 F-4a/F-4b (реестр `shared/contracts` с `Spec.Publishers`, `schemas/events/_common.json`), F-5t (`membus`) — всё в `develop`. Внешних блокеров нет.
**Ссылки:** **`contracts.md` v0.4 — C-04 v1.1, C-08 v1.2, C-10 v1.1** (действующая v0.10: C-04 v1.3, C-08 v1.3, C-10 v1.1); `api-contracts.md` §1, §1.3, §1.6, §1.7, §2.3.1–2.3.4, §2.3.14, §2.3.16; `consolidation.md` §14.1 (З-1…З-4); design §3.3, §3.4 (T-a); component §5.3, §5.6; US-008, US-038; FR-061, FR-084…FR-086; BR-13 v0.4; ADR-007.

**Дополнение (сведение 3, внесено tech-lead#1 от имени tech-lead#3; замечания З-1, З-2, З-4 решены — §8):**
- `group.left` и `group.leader_changed` получают поле **`cause` ∈ `leave | death | forget`**; `group.leader_changed.leader` — **nullable** (`ref | null`: живых участников не осталось). Схема v1 уточняется **до** её реализации, обратная совместимость не нарушается.
- `analytics.session.ended.session.end_reason` — **`leave | idle | death | error | forget`** (`forget` — закрытие активной сессии каскадом `/forget`, не ошибка).
- OpenAPI: `worlds[].llm{cloud_enabled: boolean}` в ответе `GET /v1/worlds` (без провайдера, URL и ключей — SEC-21); `GroupView.leader_id: string | null`; новый код ответа **`409 no_leader`** в `components.responses` (проверяется раньше `not_leader`) для `enter`/`leave` группы при `leader_id = null`; `409 character_dead` документируется и для `status = abandoned`.

**Сокращение (сверка 2026-09-13).** Пп. 1–2 описания сделаны в EPIC-001 (F-4b-1, F-4b-2): 20 схем — `player.*` (8), `group.*` (7), `round.*` (2), `analytics.session.started|ended` и `analytics.turn.completed` (3) — лежат в `schemas/events/`, типы зарегистрированы в `shared/contracts/registry.go` с издателем `gateway`, проверки З-1/З-2/З-4 закреплены тестами. Задача сокращается до пп. 3–5: скелет OpenAPI, DTO, таблица кодов ошибок и Go-клиент. `internal/gateway/api/errors.go` (таблица `code → HTTP-статус`, `api-contracts.md` §1.6 + коды C-08 v1.1–v1.3) **создаёт T-301**, T-303 расширяет его обвязкой ответов и middleware. Схемы и реестр правятся только при расхождении с C-04/C-10 v0.10 — запросом к system-architect через tech-lead#3.

**Описание.** Контракты, которые видны другим командам раньше кода.
1. *(сделано в EPIC-001 — см. «Сокращение»)* `schemas/events/`: `player.entered_region|left_region|looked|attacked|flee_attempted|rested|said|defended`, `group.created|joined|left|leader_changed|disbanded|entered_region|left_region`, `round.opened|closed`, `analytics.session.started|ended`, `analytics.turn.completed` — по `api-contracts.md` §2.3.1–2.3.3 и §2.3.14, конверт из `_common.json`.
2. *(сделано в EPIC-001)* Регистрация типов — PR в `shared/contracts/registry.go` (ревью system-architect; только добавление своих типов, ownership §1).
3. `api/gateway.openapi.yaml` — скелет по component §5.6: `openapi 3.1.0`, `info.version 1.0.0`, `securitySchemes.ClientId`, параметр `X-Actor-Kind`, все `components.schemas` из §5.6, `components.responses` с кодами §1.6 + C-08 v1.1, 14 маршрутов с `operationId` и пустыми/минимальными телами ответов.
4. `internal/gateway/api/dto.go` — DTO из component §5.3 (ручной `encoding/json`, теги `snake_case`).
5. `internal/gateway/client/client.go` + `longpoll.go` — сигнатуры методов C-08 (component §6), `APIError{Status, Code, Message, Details}`, повторы 3× на сетевые/`503` c тем же `action_key`; реализация против скелета (тесты — на `httptest`). Пауза между повторами — через `clock.Timers`, переданный клиенту, а не настенные таймеры (сверка 2026-09-13).

**DoD** (+ DoD-common):
- [ ] Все схемы валидны (JSON Schema draft из `_common.json`), проходят `mvctl contracts check` / тест реестра EPIC-001; каждый тип **зарегистрирован** в `shared/contracts/registry.go` с топиком из `api-contracts.md` §2.2. **(сверка 2026-09-13)** Сделано в EPIC-001: проверяется только, что `go run ./cmd/mvctl contracts check` зелёный и схемы не менялись (или изменение согласовано с system-architect).
- [ ] **(сверка 2026-09-13)** Пункты З-1 и З-2/З-4 ниже закрыты тестами EPIC-001 — повторно не писать, в отчёте назвать закрывающие тесты; новый тест — только если найден непокрытый случай.
- [ ] **(сведение 3, З-1)** `analytics.session.ended.session.end_reason` — enum ровно `leave|idle|death|error|forget`; негативный тест на значение вне enum.
- [ ] **(сведение 3, З-2/З-4)** `group.left`/`group.leader_changed` содержат `cause` (`leave|death|forget`); `group.leader_changed.leader` допускает `null` — позитивный тест документа с `leader: null`.
- [ ] **(сведение 3, З-3/З-4)** в OpenAPI присутствуют `worlds[].llm.cloud_enabled`, `GroupView.leader_id` nullable и ответ `409 no_leader`; тест `openapi_test.go` проверяет наличие всех `code` включая `no_leader`.
- [ ] `openapi_test.go` существует и зелёный: множество `(method, path)` YAML == зарегистрированным в `router.go` (на этом шаге — пустой роутер-заглушка, тест сверяет только парсинг и наличие всех `code` из `errors.go`). **(сверка 2026-09-13)** `errors.go` создан этой задачей: таблица кодов §1.6 + C-08 v1.1–v1.3, у каждого кода HTTP-статус.
- [ ] Unit клиента на `httptest`: успешный вызов, `4xx` → `APIError` с `code`, `503` → 3 повтора с тем же `action_key`, `4xx` → без повтора. **(сверка 2026-09-13)** Повторы идут по `clock.Timers`: тест на управляемых часах, без реального ожидания.
- [ ] `api/gateway.openapi.yaml` содержит раздел `admin` как «зарезервировано» (совладение с EPIC-003, `journal.md` — запрос architect#2).
- [ ] Отчёт содержит явный список поставленного другим командам (C-04 типы, C-10 типы, C-08 DTO) — tech-lead#3 передаёт оркестратору для контрольного слияния в `develop` *(сверка 2026-09-13: заменено «tech-lead#1 для раннего merge»)*.

### T-302: Хранилище и миграции обеих БД + тесты миграций и физического `/forget` · I1 · подволна 1.2, идёт параллельно с 1.1 · developer#2 · M · **Статус: in-progress**

**Ветка:** `task/T-302-store-migrations` (`.worktrees/T-302`). Карточку `tasks/T-302.md` и `dev-log.md` ведёт исполнитель в ветке задачи.
**Зависит от:** — (сверка 2026-09-13: идёт параллельно с T-301; общий файл двух задач — только `go.mod`/`go.sum`, его меняет T-302). *(Заменено: «T-301 (не жёстко — только общий каркас модуля)»; исполнитель developer#1 → developer#2.)*
**Ссылки:** ADR-019 (+ доп. п. 1, 2), SEC-03, SEC-04/05; component §4.1, §4.2, §3 (`store/`, `migrations/`); design §3.1 п. 1; US-009; NFR-042.

**Описание.** `internal/gateway/store/open.go` (`OpenLinks`, `OpenGateway`; PRAGMA по component §4.1/§4.2; `SetMaxOpenConns(1)`), `store/migrate.go` (goose Provider из `embed.FS`; сверка 2026-09-13: `//go:embed` лежит в самом пакете `internal/gateway/migrations` — например, `embed.go` экспортирует `FS`, потому что `go:embed` не видит родительских каталогов, и `store` импортирует этот пакет), `migrations/links/0001_init.sql`, `migrations/gateway/0001_init.sql` — **полная схема I1+I2 сразу** (включая `rounds`, `group_participation`), чтобы I2 не добавлял миграций. Права файла `0600`, каталога `0700`. Sweeper-заготовка retention (component §4.2) — интерфейс без запуска. **(сверка 2026-09-13)** Зависимости `modernc.org/sqlite` и `github.com/pressly/goose/v3` в `go.mod` вносит эта задача (в дереве их нет; версии — по design §4 либо последние совместимые с Go 1.26, в отчёте). `modernc.org/sqlite` — чистый Go без cgo, поэтому тесты на временном файле — unit, без тега `integration`.

**DoD** (+ DoD-common):
- [ ] Тест миграций (unit, **без** тега `integration`, идёт в `-short`; временный файл в `t.TempDir()`): миграции обеих БД с нуля; повторный `up` — no-op; `down` не требуется (MVP-1). *(Сверка 2026-09-13: заменено «Integration-тест (`//go:build integration`)».)*
- [ ] Тест схемы (unit, SEC-03): список колонок `gateway.db` сверяется с allowlist; колонка с подстрокой `external` допустима **только** в `links.db`; `link_id` в `gateway.db` отсутствует.
- [ ] Тест физического удаления (unit, без тега — сверка 2026-09-13; SEC-04/05, NFR-042): вставка строки с известным тестовым ID → `DELETE` + `wal_checkpoint(TRUNCATE)` + `incremental_vacuum` в одном вызове → `strings`-скан **файла и WAL** = 0 вхождений.
- [ ] Тест PRAGMA: `links.db` — `secure_delete=ON`, `auto_vacuum=INCREMENTAL`, `journal_mode=WAL`, `synchronous=FULL`; `gateway.db` — `WAL`, `synchronous=NORMAL`, `foreign_keys=ON`, `busy_timeout=5000`.
- [ ] **(сведение 3, З-1)** в `migrations/gateway/0001_init.sql` — `CHECK (end_reason IN ('leave','idle','death','error','forget'))` для `sessions.end_reason`; тест (unit, без тега): вставка каждого из пяти значений проходит, шестое (`forget_all`) отклоняется CHECK'ом. `players`-статус `abandoned` в gateway **не хранится** (это факт State) — зафиксировано в комментарии миграции.
- [ ] Права `0600`/`0700` проверены тестом (skip на Windows-раннере с пометкой).
- [ ] `MV_GATEWAY_DATA_DIR` читается через уже объявленную `env.GatewayDataDir` (`shared/env/vars.go`, умолчание `/data`); повторного `Declare` нет, `.env.example` не меняется. *(Сверка 2026-09-13: заменено «объявлен через `shared/env.Declare`; `.env.example` дополнен» — переменная объявлена в EPIC-001.)*
- [ ] **(сверка 2026-09-13)** `go.mod`/`go.sum`: добавлены `modernc.org/sqlite` и `github.com/pressly/goose/v3`, отмечены в отчёте; `go mod tidy` лишнего не тянет; миграции встроены через `embed` в пакете `internal/gateway/migrations`.

### T-303: `links` (resolve/consent/forget/`link_id`/`RouteFor`) и каркас HTTP-слоя · I1 · подволна 1.3 · developer#1 · M · **точка подключения developer#2**

**Ветка:** `task/T-303-links-http` (`.worktrees/T-303`).
**Зависит от:** T-302, T-301 (DTO/OpenAPI, `errors.go`).
**Ссылки:** C-08 v1.1 (v1.3 — имена списков клиентов), C-01 v1.3 (`Deps`), C-01 v1.4 (источники конструкторов); SEC-03, SEC-08 (частично), SEC-11, SEC-12, SEC-26; component §4.1, §5.1, §5.2 (`links`-маршруты), §6, §7.1, §7.5, §11.1; design §3.1 п. 2; US-001, US-009; FR-060, FR-061; NFR-041/042/045; ADR-009 (+ доп. п. 2, 7).

**Описание.**
1. `links/store.go` — интерфейс и SQLite-реализация `Store` (component §6): `Resolve` (нет строки → INSERT `pending_consent` + новый `link_id`, обновляет `last_seen_at`), `Consent`, `AttachPlayer`, `ByPlayer`, `RouteFor`, `Forget`, `ForgetByPlayer`, `CharacterRequest`/`SaveCharacterRequest` (ключ `(link_id, action_key)`, TTL 24 ч), `Compact`.
2. `links/pseudonym.go` — `NewPlayerID`, `NewLinkID`; `Link.LogValue() → "redacted"`. **(сверка 2026-09-13)** Источник `player_id`, `link_id` и id событий — `Deps.IDs` (`--id-source=uuid|sequence`) либо решение architect#3: ULID из design требует новой зависимости `oklog/ulid/v2`, которой в `go.mod` нет. Глобальный источник `eventbus.SetIDSource` процесс пока не ставит (C-01 v1.4, «Источники конструкторов»), и контекст сам его не ставит.
3. `links/forget.go` — `Forget` + `ForgetHooks` (в I1 подключены `outbox.DropForPlayer`, `session.End` — регистрируются позже; интерфейс и вызов уже здесь), checkpoint+vacuum в том же вызове.
4. `api/`: ~~`server.go`~~ **(сверка 2026-09-13)** своего `http.Server` нет: сервер процесса создаёт `cmd/multiverse` через `runtime.NewHTTP` на `MV_CORE_ADDR` и сам отдаёт `GET /health` (агрегат `Health()` контекстов); контекст gateway реализует `runtime.Routes` и монтирует маршруты на mux процесса до `Start`. Сервер процесса задаёт только `ReadHeaderTimeout 10s` и `ShutdownTimeout 5s`; таймауты component §5.1 (`ReadTimeout 10s`, `WriteTimeout 35s`, `IdleTimeout 120s`) и то, как long-poll отпускает соединение при остановке, — **по решению system-architect** (запрос открыт); до решения — `timeout`-middleware на маршрутах, кроме long-poll. `router.go`, `middleware.go` (порядок 1–8: recover, request_id, client/actor_kind/client_mismatch, body_limit 64 КиБ, `nolog`, ratelimit-заглушка, `pollguard`, timeout), `errors.go` (таблица `api-contracts.md` §1.6 + коды C-08 v1.1), `handlers_links.go`, `handlers_health.go` (поля `/health` gateway отдаются через `Health()` контекста, маршрут `/health` — у сервера процесса). *(T-416, 2026-09-11: заменено `MV_GATEWAY_LISTEN` — такой переменной нет. Адрес HTTP любого процесса задаёт только `MV_CORE_ADDR`, решение T-408, `contracts.md` v0.6 «Env». HTTP-сервер процесса даёт `shared/runtime`, gateway монтирует маршруты на `Deps.Mux`, C-01 v1.3.)*
5. `config.go` — переменные gateway (component §11.3) читаются через объявления манифеста `shared/env`. Два списка: **`MV_GATEWAY_CLIENT_IDS`** — допущенные `X-Client-Id`; **`MV_GATEWAY_ACTOR_KIND_CLIENTS`** — клиенты с правом на `X-Actor-Kind: ci|sim` (C-08 v1.3). Платформа клиента для SEC-12 в MVP-1 — **фиксированное соответствие в коде** (`client_id → platform`). Отдельная переменная появится со вторым ботом (решение оркестратора по T-409, `journal.md` 2026-09-11). *(Заменено (T-416): «`MV_GATEWAY_CLIENTS` парсер `client_id:platform:actor_kinds`».)* **(сверка 2026-09-13)** Оба списка уже объявлены (`env.GatewayClientIDs`, `env.GatewayActorKindClients`) — читаются через них.
6. **(сверка 2026-09-13)** `internal/gateway/context.go` — `runtime.Context` (`Name() = "gateway"`, `DependsOn`, `Start`, `Stop`, `Health`) и `Routes`. Регистрация: в `cmd/multiverse/contexts.go` заглушка `gateway` заменяется фабрикой настоящего контекста — ровно одна регистрация имени (вторая `runtime.Register` паникует). Файл принадлежит EPIC-001: правка согласуется с tech-lead#1 и не трогает `swarm`-хук T-255.

**DoD** (+ DoD-common):
- [ ] Unit `links`: `resolve` без строки → `pending_consent` + непустой `link_id`; повторный `resolve` — тот же `link_id`, обновлён `last_seen_at`; неполное согласие → остаётся `pending_consent`; `consented` ⇒ три timestamp не NULL; `AttachPlayer` при `dead` → новый `player_id`, `link_id` не изменился; уникальность `player_id`.
- [ ] Unit `Forget`: каскад `character_requests`, вызов `ForgetHooks`, возврат `{deleted, player_id_detached}`; повтор — идемпотентно `{deleted:false}` без ошибки; `/forget` от аккаунта без персонажа → «нечего удалять» (US-009).
- [ ] Unit `Link.LogValue()`: `slog` с атрибутом `Link` не печатает внешний ID и `link_id`.
- [ ] Unit `api` (SEC-11/12): `403 client_unknown` при `X-Client-Id` вне `MV_GATEWAY_CLIENT_IDS`; `403 actor_kind_forbidden` при `X-Actor-Kind: ci|sim` от клиента вне `MV_GATEWAY_ACTOR_KIND_CLIENTS` *(T-416: заменено `MV_GATEWAY_CLIENTS`, C-08 v1.3)*; `403 client_mismatch` для `/v1/clients/{id}/*`; `413 payload_too_large` на 65 КиБ; `409 poll_in_progress` на второй параллельный long-poll; `X-Request-Id` в ответе.
- [ ] Unit `nolog` (SEC-01/02): при `400/500` на `POST /v1/links/*` и `POST /v1/characters` в логе есть только `request_id` и `code`, тела и внешнего ID нет.
- [ ] `openapi_test.go` зелёный: маршруты `links`, `/health` из `router.go` есть в `api/gateway.openapi.yaml`; **OpenAPI обновлён** телами ответов этих маршрутов.
- [ ] Семантика `/forget` (что удаляется, что остаётся) отражена в `description` соответствующих операций OpenAPI (SEC-26).
- [ ] Переменные `MV_*` из component §11.3, относящиеся к API и данным, объявлены и есть в `.env.example` — по правилу DoD-common: новые вместе с кодом чтения, объявленные читаются через `env.*`.
- [ ] **(сверка 2026-09-13)** Настоящий контекст зарегистрирован: `go run ./cmd/multiverse serve --contexts=gateway --bus=memory` поднимает gateway, `/health` отражает `Health()` gateway, маршруты `links` отвечают на `MV_CORE_ADDR`; тест на то, что имя `gateway` зарегистрировано один раз и заглушки больше нет.
- [ ] **(сверка 2026-09-13)** Детерминизм id: при источнике `sequence` `player_id`/`link_id` воспроизводимы (unit: два прогона дают одинаковые id); источник — `Deps.IDs` или решение architect#3, записанное в карточке.
- [ ] **(сверка 2026-09-13)** Таймауты HTTP — по решению system-architect, решение и его ссылка — в карточке; своего `http.Server` в `internal/gateway` нет.

**Приёмка (tech-lead#3, 2026-09-13): принято, статус `done`** — карточка [`tasks/T-303.md`](tasks/T-303.md), раздел «Приёмка (tech-lead)». Ревью: 2 итерации, итог 0/0/1/3. R2-Mi-1 и R2-N-1 закрыты тестами при приёмке. В-1 (`startProcess` без `MV_GATEWAY_DATA_DIR`) и В-2 (runbook: проверка тома до удаления) из отметки tech-lead#1 закрыты, нужна его повторная отметка. Уточнения DoD по факту: `/health` — маршрут процесса, в `router.go` не монтируется (решение T-301); `serve --contexts=gateway --bus=memory` закрыт тестом процесса (`--bus=memory` требует `--contexts=all`); новый код `503 forget_incomplete` — по решению system-architect#1, текст C-08 — T-456. Слияние — после повторной отметки tech-lead#1; конфликт с T-446 (`contexts*.go`) разрешать по рецепту карточки. Открытое условие приёмки эпика (не задачи): `make image && make up && make health` на стенде владельца с проверенным по runbook томом `gateway-data`.

### T-304: `readmodel` (проекция, bootstrap, waiters) и `consumer` (диспетчер, `entity.*`, `encounter.*`) · I1 · подволна 1.4 · developer#1 · M · **Статус: done**

**Ветка:** `task/T-304-readmodel-consumer` (`.worktrees/T-304`).
**Зависит от:** T-302, T-303. **Внешнее:** `testkit/state.FakeState` v0 и фикстура `testdata/fixtures/snapshots/state/latest.json` (EPIC-001/002, в `develop`), `shared/entity` v2, `shared/eventbus/membus` с `Journal` (реализация C-01). *(Сверка 2026-09-13: заменён путь `testdata/snapshots/state/latest.json`.)*
**Ссылки:** C-02 v1.1, C-14 v1.1 (v1.2 — окна дедупа в снапшоте), C-01 v1.1 (**v1.3** — состав `Deps`; **v1.5** — id из причины, перехват паники обработчика в `Delivery`; **v1.6** — цепочка ошибки при сбое `dead_letters`, посредник доставки; **v1.7** — `Close` не отменяет контекст обработчика), ADR-027; component §3 (`readmodel/`, `consumer/`), §5.2, §11.2, §16 п. 2; design §3.1 п. 3 (часть), §3.3; US-011; NFR-010/011, NFR-013.

**Описание.** `readmodel/model.go|apply.go|bootstrap.go|waiters.go`: типизированные проекции (World, Region, NPC, CharacterState, Group, Encounter) поверх `entity.Ref`/геттеров `shared/entity` v2; `Apply(entity.created|updated|update.rejected)`, `ApplyEncounter(started|ended)`; `LoadFromStateSnapshot` из `snapshots-{world}/state/latest.json` (нет объекта → пустая проекция, `/health.projection=missing`); `AwaitFact(correlationID, ≤ 2 с)`; `Hash()`, `Cursor()`.
`consumer/dispatcher.go` — подписка на `system_events`, `game_events`, `world_events`, `narrative_output`; дедуп `processed_events`; два курсора («проекция» и «эффекты») в одной транзакции с эффектами; `handle_entity.go`, `handle_encounter.go` (в этой задаче — только проекция и `waiters`, без outbox).
**Окно дедупа и границы шины (сверка 2026-09-13; C-01 v1.4–v1.7, ADR-027, C-14 v1.2).** Событие считается обработанным только после фиксации эффекта: строка `processed_events` пишется в той же транзакции, что эффект и курсор, — это двухшаговая форма `Has`/`Add` («спросить → эффект → запомнить»); запоминание до эффекта (`Seen`) не используется. Если рядом держится окно `eventbus.Dedup` в памяти, оно входит в снапшот gateway (C-14 v1.2, T-309). Ошибка обработчика возвращается шине, а не глотается; панику обработчика перехватывает `Delivery` (C-01 v1.5) — своего `recover` вокруг обработчика gateway не ставит. Остановка: `Stop` gateway отменяет контекст подписок; `Close` шины обработчик не отменяет (C-01 v1.7).

**DoD** (+ DoD-common):
- [ ] Unit: `Apply` строит `CharacterState` из `entity.created`+`entity.updated` (HP, позиция, scope, инвентарь, статус); `entity.update.rejected` с известным `proposal_id` снимает ожидание.
- [ ] Unit `AwaitFact`: факт ≤ 2 с → возврат события; таймаут → `context.DeadlineExceeded`; параллельные ожидания разных `correlation_id` не мешают друг другу.
- [ ] Unit `bootstrap`: загрузка из фикстуры `testdata/fixtures/snapshots/state/latest.json` *(сверка 2026-09-13: путь по дереву)*; отсутствие объекта → пустая проекция и `projection=missing` (US-011 «не поднимается с пустым состоянием молча» — отражено в `/health` и логе).
- [ ] Unit `consumer`: дубль события по `event.id` не применяется дважды (NFR-013); курсоры продвигаются в одной транзакции с эффектом; события с `offset ≤ cursor(эффекты)` применяются только к проекции.
- [ ] **(сверка 2026-09-13; C-01 v1.4, ADR-027)** Unit окна дедупа: эффект не зафиксирован (ошибка записи или публикации) → событие не помечено обработанным, повтор шины применяет его; мутант «пометить до эффекта» краснеет. Ошибка обработчика доходит до шины (возвращается из обработчика).
- [ ] `readmodel` использует **только** типизированные геттеры `shared/entity` v2 — прямых обращений к `map[string]any` нет (риск design §8).
- [ ] `/health.projection` возвращает `ok|missing|stale`.
- [ ] **(T-416, 2026-09-11; C-04 v1.3, C-05 v1.4 п. 3–5, ADR-026 п. 4)** Read-model считает встречу законченной по **первому** из двух событий: факт сущности встречи со `state=resolved` (`entity.updated`, `system_events`) или `encounter.ended` (`world_events`). Второе событие пары состояние не меняет и повторного эффекта не даёт (ожидания, доставки). В журнале факт всегда раньше события, но между топиками порядок не гарантирован (C-01). Встреча, объявленная `encounter.started` раньше `entity.created`, держится как «объявлена, сущности ещё нет», и состояние это временное. Unit: оба порядка прихода «факт / `encounter.ended`» дают одно и то же закрытое состояние; оба порядка «`encounter.started` / `entity.created`» дают одну и ту же открытую встречу; действие после конца, пришедшего любым из двух путей, отвергается предусловием (`not_in_encounter`, T-305).
- [ ] **(оркестратор по T-416, 2026-09-11; C-05 п. 1д; перенесено из заголовка §0 при сверке 2026-09-13)** Срок ожидания ответа на действие — не конец встречи: при варианте «не предлагать повтор» потребитель получает только срок, события отказа нет; шлюз сообщает игроку «ответа нет» и не закрывает встречу, конец — только по факту `resolved` или `encounter.ended` (тест).

**Приёмка (tech-lead#3, 2026-09-13): принято, статус `done`** — карточка [`tasks/T-304.md`](tasks/T-304.md), раздел «Приёмка (tech-lead)». Ревью: 1 итерация, итог 0/0/7/4. Minor закрыты итерацией 2 без повторного ревью (решение оркестратора) и проверены при приёмке мутантами R1–R4. Разрыв «курсор проекции за курсором эффектов» закрыт тестом при приёмке. Решения владельца эпика: Mi-2 — уникальность перехода в пределах процесса и строки DoD T-307/T-351, таблица переходов — вопрос architect#3; Mi-6 — `degraded`, а не ошибка старта (healthcheck compose считает `degraded` провалом, `make up --wait` падает громко); Mi-7 — бюджеты 30 с / 2 мин по настенным часам. Уточнение DoD по факту: «действие после конца отвергается» закрыт на уровне проекции (`EncounterOf`), код `not_in_encounter` даёт T-305. С кончиком эпика 2f8471b не пересекается, пробная сборка и тесты зелёные. Слияние — оркестратор.

### T-305: `actions` — валидация, идемпотентность, лимит, `InputFilter`, публикация `player.*` · I1 · подволна 1.5 · developer#1 · M · **Статус: done**

**Ветка:** `task/T-305-actions` (`.worktrees/T-305`).
**Зависит от:** T-303, T-304.
**Ссылки:** C-04 (v1.2 — только `position` при движении), C-02 v1.1, C-08 v1.1; SEC-11, SEC-12; component §5.4, §5.5, §7.2, §11.3; design §3.1 п. 3, §3.4 (T-e); US-016 (FR-056), US-019 (FR-014); FR-002, FR-006, FR-023, FR-025 (соло-часть); NFR-003, NFR-013.

**Описание.** `actions/validate.go` (таблица `map[ActionType]Rule` — прямая кодировка `api-contracts.md` §1.4, фиксированный порядок проверок component §5.4 п. 1–4; правила группы — заглушки, включаются в I2), `actions/publish.go` (`BuildPlayerEvent`, `BuildPositionProposal`; `enter/leave/rest` → `entity.update.proposed`; `action.key_hash = SHA-256(action_key)`), `actions/idempotency.go`, `actions/ratelimit.go` (token bucket 30/мин burst 5 на `player_id`, уборка раз в 10 мин, в `replay` выключен), `actions/inputfilter.go` (`InputFilter` + `NoopFilter` с записью журнала `input_filter{applied, status, filter, text_len}` без текста; выбор по `MV_GATEWAY_INPUT_FILTER`, иное значение → ошибка старта), `api/handlers_players.go` (`POST /v1/players/{id}/actions`), `MV_GM_PATH=legacy` → дополнительная публикация `gm.created` (один `if`).
**Сверка 2026-09-13.** (1) C-04 v1.2: при `enter`/`leave` `BuildPositionProposal` предлагает **только** `position`; `scope` в набор изменений не входит. (2) `gm.created` — **по решению system-architect**: в реестре это легаси-тип без схемы с издателем только `legacy` (`contracts.md` §0 «Легаси-типы реестра»), публикация от `gateway` не пройдёт проверку издателя. Варианты — добавить `gateway` в издатели легаси-типа или снять требование со шлюза. До решения пункт не реализуется, остальная задача не блокируется. (3) `MV_GATEWAY_INPUT_FILTER` и параметры лимита объявляются вместе с кодом; уборка лимитера — по `Deps.Timers`.

**DoD** (+ DoD-common):
- [ ] Табличный unit `Validate` по **всем** строкам `api-contracts.md` §1.4 и кодам §1.6: `player_not_found`, `character_dead`, `unknown_action`, `invalid_request`, `text_invalid`, `unknown_target`, `in_encounter`, `not_in_encounter`, `target_dead`, `not_in_region`, `encounter_unavailable`; порядок проверок соблюдён (тест на первое несоответствие).
- [ ] Unit идемпотентности (C-08): повтор `action_key` → тот же `correlation_id` и тот же ответ, включая сохранённый `4xx`; `503 bus_unavailable` при ошибке `Publish` — ключ **не** записан, `Turn` не сохранён.
- [ ] Unit rate limit (SEC-11, NFR-049): 31-е действие за минуту → `429 rate_limited` с `Retry-After`; на 61-й секунде снова принимается; в `--mode=replay` лимита нет.
- [ ] Unit `InputFilter` (US-016, FR-056): тестовая реализация «заменить слово» → в `player.said.text` уходит `FilterResult.Text`, исходный текст в шину не попадает; ошибка фильтра → `422 filter_error` (fail-closed); имя персонажа проходит тот же фильтр; `MV_GATEWAY_INPUT_FILTER=other` → ошибка старта.
- [ ] Unit `MV_GM_PATH` (US-019, FR-014): `legacy` → публикуется `gm.created`; `agent` → не публикуется. В коде помечено `// удаляется в EPIC-003 I2 (S5)`. **(сверка 2026-09-13)** Пункт выполняется по решению system-architect; если требование снято со шлюза — пункт вычёркивается записью в карточке со ссылкой на решение.
- [ ] **(сверка 2026-09-13; C-04 v1.2)** Unit: `enter`/`leave` публикуют `entity.update.proposed` только с `position`; `scope` в наборе изменений отсутствует.
- [ ] Unit `say`: `TrimSpace`, 1–500 рун, управляющие символы отклоняются (`text_invalid`, NFR-043).
- [ ] Публикуемые события валидируются против схем из T-301 в тесте (contract-тест реестра).
- [ ] `202` возвращается до ожидания чего-либо, кроме подтверждения брокера; unit с таймером ≤ 100 мс на `membus` (вклад в NFR-003).
- [ ] **OpenAPI обновлён**: `postAction` с `ActionRequest`/`ActionAccepted` и всеми кодами; `openapi_test` зелёный.
- [ ] **(отметка tech-lead#1 по T-458, Н-2; внесено при приёмке T-305, 2026-09-13)** Операция `POST /v1/admin/replay/clock` (`replayClock`, C-01 v1.9) помечена как маршрут процесса: тег `process` и список `servedByProcess` в `openapi_test.go`, как у `GET /health`. `api.GatewayRouter` её не монтирует: маршрут монтирует `cmd/multiverse` в `--mode=replay`, повторная регистрация шаблона в `ServeMux` процесса — паника и отказ старта. Тест таблицы маршрутов `TestOpenAPIRoutesMatchRouter` различает маршруты шлюза и процесса.
- [ ] **(C-01 v1.11, У-3 просмотра T-458; внесено при приёмке T-305)** Ответы `replayClock`: `405` (ответ mux процесса, `Allow: POST`) и `404` (live) — без схемы тела; тела `400 invalid_body` и `409 clock_behind` — в форме `Error` (`{"error": {"code", "message"}}`, без `details`). Проверяет `TestOpenAPIReplayClockAnswersOfC01`.

**Приёмка (tech-lead#3, 2026-09-13): принято, статус `done`** — карточка [`tasks/T-305.md`](tasks/T-305.md), раздел «Приёмка (tech-lead)». Ревью: 2 итерации, итог 0/0/1/2. При приёмке закрыты с тестами Mi-6, N-6 и N-7 ревью #2. Mi-6: ход и ключ после полной публикации пишутся на `context.WithoutCancel`, тест по зонду P4. N-6: утечка записи блокировки игрока закреплена тестом, мутант N3 красный. N-7: оценка памяти пакетов исправлена на десятки МБ. Ответы `replayClock` приведены к C-01 v1.11 (строки выше); шлюз маршрут не монтирует. Приняты отступления: лимит GCRA с минутным журналом вместо «чистого» token bucket, `group.*` → `501` до T-352, `Turns` в памяти до T-306, новые поля `readmodel.Encounter`. Вопросы architect#3 — в карточке: КД §5.5, лимит после `Lookup`, `player_not_found` под ключом, поля `Encounter`, формулировки token bucket. Нужна отметка tech-lead#1: `shared/env/vars.go` и `.env.example` (`MV_GATEWAY_*`), текст `replayClock` (владелец текста — EPIC-001).

### T-306: `characters`, `GET /v1/worlds`, `GET /v1/players/{id}`, `session` + `turns` + аналитика C-10 · I1 · подволна 1.6 · developer#1 · M

**Ветка:** `task/T-306-characters-session-turns` (`.worktrees/T-306`).
**Зависит от:** T-303, T-304, T-305.
**Ссылки:** C-02, C-08 v1.1, C-10; SEC-03; component §5.2, §7.1, §7.6, §4.2 (`sessions`, `turns`, `pending_characters`); design §3.1 п. 2–3, п. 9; US-001, US-038; FR-001, FR-006, FR-023, FR-060, FR-085…FR-088, BR-17; NFR-036, NFR-045.

**Описание.** `api/handlers_characters.go` (`POST /v1/characters`: `consented` обязателен → иначе `consent_required`; `character_requests` по `(link_id, action_key)`; `pending_characters`; `entity.create.proposed` → `AwaitFact ≤ MV_GATEWAY_CHARACTER_WAIT` → `201`/`200`/`202 creating`; дедлайн `MV_GATEWAY_CHARACTER_DEADLINE`), `handlers_players.go` (`GET /v1/players/{id}`, `character_status ∈ none|creating|alive|dead`), `handlers_health.go` (`GET /v1/worlds`), `session/manager.go` + `session/analytics.go` (`Touch`, `Open`, `End`, `UpdateParticipants`, `Current`, `Active`, sweeper простоя 30 мин), `turns/tracker.go` + `turns/store.go` (`Register`, `OnMechanics`, `OnNarrative`, `OnDelivered`, `Sweep`), публикация `analytics.session.started|ended`, `analytics.turn.completed`, фикстура `testdata/analytics/solo-30.jsonl` (для EPIC-005 005-ops).
**Сверка 2026-09-13.** Каталог `testdata/analytics/**` — владение EPIC-005 (`ownership.md` §1). Фикстура кладётся туда только с согласия tech-lead#1 (EPIC-005 ведёт TEAM-1); без согласия — `internal/gateway/testdata/analytics/solo-30.jsonl` с уведомлением EPIC-005. Таймеры sweeper и дедлайнов — `Deps.Timers`.

**DoD** (+ DoD-common):
- [ ] Unit US-001: мир не найден → `world_not_found`, сущность не предлагается; без согласия → `consent_required`; повтор при `alive` → тот же `player_id`, второй персонаж не создаётся (A-7); при `dead` — новый `player_id`, связка обновлена; регистрация без имени → `name_required`; имя 2–32, регулярка совпадает с ботовой (FR-060).
- [ ] Unit идемпотентности `characters` (SEC-03): ключ — `link_id + action_key`; повтор возвращает сохранённый ответ; `gateway.db` при этом не получил ни `external_id`, ни `link_id`.
- [ ] Unit `202 creating`: факт не пришёл за 2 с → `202 {player_id, status:"creating"}`; последующий `GET /v1/players/{id}` → `creating` → `alive`; не пришёл за 60 с или `entity.update.rejected` → `pending_characters` очищен, статус согласован.
- [ ] Unit `session` (BR-17, US-038): первое действие в scope без сессии → `analytics.session.started` без внешних ID; простой ≥ 30 мин → `ended end_reason=idle` + новая сессия; sweeper закрывает без следующего действия; парность `started/ended` (NFR-036); `actor_kind` сессии = `X-Actor-Kind` первого действия; в `--mode=replay` sweeper выключен и `analytics.*` не публикуются.
- [ ] Unit `turns` (US-038): `turn.completed` публикуется на **каждый** принятый и **каждый** отклонённый ход (`status=rejected` с `timings{received_at, acked_at}`); `status=degraded` при `generated_by=template`; `timeout` по дедлайну 60 с; `completed` — после ack последнего адресата.
- [ ] `testdata/analytics/solo-30.jsonl` — фикстура сформирована из зелёного прогона и валидна против схем C-10; отчёт содержит уведомление EPIC-005 (005-ops) о готовности фикстуры. **(сверка 2026-09-13)** Место файла — по согласию tech-lead#1 (см. описание); согласие или запасной путь записаны в карточке.
- [ ] События `analytics.*` — в топике `analytics_events`, не читаются в replay (C-10).
- [ ] **OpenAPI обновлён**: `createCharacter`, `getPlayer`, `listWorlds`; `openapi_test` зелёный.
- [ ] **(приёмка T-303, 2026-09-13; риск ревью #2 T-303)** Окно снятия отметки отложенного сжатия `links.db`. Успешный `Compact` sweeper'а не должен снимать отметку, которую поставил параллельный `/forget` с неудачным сжатием. Способ — мьютекс «DELETE + сжатие» в `links.SQLite` или счётчик поколений отметки. Unit с принудительным порядком «checkpoint sweeper'а → `DELETE` и неудачное сжатие `/forget` → снятие отметки sweeper'ом»: отметка остаётся, следующий `/forget` отвечает `503 forget_incomplete`. Если T-306 не добавляет в `links.db` записи из фоновой задачи, пункт переносится в T-314 записью в карточке.
- [ ] **(приёмка T-305, 2026-09-13; ревью #2 T-305, Mi-6)** Трекер ходов заменяет `actions.MemoryTurns` за интерфейсом `actions.Turns`. `Turns.Accepted` пишет ход в `gateway.db` на контексте без отмены и без срока решения (`context.WithoutCancel`), как `Keys.Save` в T-305. Unit: срок решения истекает между последней публикацией и `Turns.Accepted` → ход записан, повтор ключа отдаёт сохранённый `202`, второго `player.*` нет.
- [ ] **(приёмка T-305; предложение итерации 2 T-305)** `seq` хода резервируется при `Turns.Begin`, а не вычисляется при `Accepted`. Два действия одного scope не получают один `seq`, даже если между сбоем публикации пакета и его повтором scope принял другое действие. Unit на этот порядок: сбой пакета → другое действие того же scope → повтор ключа → разные `seq`.
- [ ] **(приёмка T-305; N-1 ревью #1 T-305)** Имя нового персонажа проходит `actions.Service.FilterText(ctx, actions.KindName, name)`. Результат перепроверяется правилами имени (2–32 символа, регулярка FR-060): пустой → `name_required`, иное несоответствие и отказ фильтра (`text_invalid`, в том числе `blocked`) → `name_invalid`; ошибка фильтра → `422 filter_error`. Unit: фильтр заменяет имя строкой из 1 и из 33 символов → `400 name_invalid`, персонаж не предлагается.

### T-307: `outbox` (store/lease/ack/render/sweeper), long-poll `deliveries`, `consumer` для `combat.decided`/`narrative.output` · I1 · подволна 1.7 · developer#1 · M

**Ветка:** `task/T-307-outbox-deliveries` (`.worktrees/T-307`).
**Зависит от:** T-304, T-306. **Внешнее:** `testkit/swarm.FakeNarrator` из T-220 (EPIC-003, в `develop`: `NewFakeNarrator(bus, worldID)`, шесть поводов C-05, `generated_by=template`) — только для тестов. *(Сверка 2026-09-13: заменено «`FakeNarrator` v0 (C-05, F-10)».)*
**Ссылки:** ADR-006 (+ доп.), C-05 v1.1, C-08 v1.1; SEC-11, SEC-12; component §8.1–8.4, §5.2, §3 (`outbox/`, `consumer/`); design §3.1 п. 4, §3.4 (T-f); US-002/US-004 (вклад), US-008; FR-003, FR-013; NFR-001, NFR-005, NFR-013.

**Описание.** `outbox/types.go|store.go|longpoll.go|render.go|sweeper.go`: `Enqueue` идемпотентно по `(event_id, player_id)`; `Lease` «голова очереди на игрока» (SQL component §8.2); `Ack`; `ReleaseExpiredLeases`; `Expire`; `DropForPlayer`; `Notifier`; `render.Mechanics` (ru-тексты component §7.2, `generated_by=rules`). `consumer/handle_combat.go`, `handle_narrative.go` (адресаты по `payload.recipients[]`, `data{narrative_event_id, kind, absence?, filter}`), постановка доставок для `entity.updated cause ∈ {move, rest, loot}`, `player.status=dead`, `encounter.started`. `api/handlers_deliveries.go`: `GET /v1/clients/{id}/deliveries` (long-poll, `wait_ms ≤ 25000`, `limit ≤ 100`), `POST …/ack`, `GET …/stream` → `501`. Подстановка `route.external_id` через `links.RouteFor` **только в ответ**; нет маршрута → `dropped`.

**DoD** (+ DoD-common):
- [ ] Unit `Lease`: строгий порядок `seq` на игрока; одна доставка в лизинге на игрока; повторная выдача после истечения 30 с; доставка без маршрута → `dropped` и в ответ не попадает.
- [ ] Unit `Enqueue` идемпотентности (NFR-013): повторная доставка того же события шиной не создаёт вторую строку.
- [ ] Unit `Ack`: чужие/неизвестные id → `unknown[]` без ошибки; ack чужого клиента не подтверждает доставку; `turns.OnDelivered` вызван для `kind=narrative`.
- [ ] Unit SEC-12: `route.external_id` не выдаётся клиенту платформы, отличной от платформы связки; `403 client_mismatch` для чужого `{client_id}`.
- [ ] Unit SEC-11: второй параллельный long-poll → `409 poll_in_progress`; `wait_ms`/`limit` ограничиваются сверху.
- [ ] Golden-тест `render.Mechanics`: попадание/промах/крит, `npc_attack`, `flee success/fail`, смерть, `rest`, `move` — тексты в `testdata/golden/`, файлы с `merge=binary` (ownership).
- [ ] Unit long-poll: `Serve` не держит соединение БД во время ожидания; `Notify` после `Enqueue` будит ждущего ≤ 50 мс; периодическое пробуждение раз в 1 с работает при потере сигнала.
- [ ] Sweeper: `ReleaseExpiredLeases`, `Expire` (TTL 24 ч → `dropped`), retention `processed_events`/`idempotency_keys` — по component §4.2; в `--mode=replay` выключен.
- [ ] **(T-416, 2026-09-11; C-05 v1.4 п. 8 и «Гарантии»)** Шлюз **не переупорядочивает** `narrative_output`: доставки `narrative.output` встают в очередь игрока в порядке топика. Порядок «смерть после текста хода» обеспечивает издатель нарратива (T-233), по `based_on[]`, `kind` и времени шлюз ничего не переставляет. Unit: `kind=death` пришёл раньше `kind=turn` той же `correlation_id` → доставки выданы в порядке прихода, без задержки и перестановки.
- [ ] **(сверка 2026-09-13)** Long-poll не держит остановку процесса: при `Stop` сервера процесса (`runtime.ShutdownTimeout` 5 с) ждущий запрос возвращается; предельное время ответа — по решению system-architect о таймаутах (T-303). Unit на управляемых часах.
- [ ] **OpenAPI обновлён**: `pollDeliveries`, `ackDeliveries`, `streamDeliveries (501)`; `openapi_test` зелёный.
- [ ] **(ревью #1 T-304, Mi-2, 2026-09-13)** Переходы `readmodel.Result.EncounterOpened`/`EncounterEnded` уникальны только в пределах процесса: захват первого события пары живёт в памяти, и после рестарта второе событие пары сообщает переход снова (тест `TestTheClaimOfATransitionDoesNotSurviveARestart`). Доставки по переходу встречи (`encounter.started`, конец встречи) идемпотентны по `encounter_id` и виду перехода в `gateway.db`, в транзакции consumer. Если для этого нужна таблица или столбец — миграция `0002_*` по решению architect#3 (схема `gateway.db` — component §4.2; `0001_init.sql` объявлен полной схемой I1/I2) с правкой allow-list SEC-03 в `store/migrate_test.go`. Unit: рестарт между двумя событиями пары (проекция из снапшота State с уже открытой или закрытой встречей) → одна доставка.
- [ ] **(приёмка T-305, 2026-09-13; бэклог T-305)** Отказ State `entity.update.rejected` на предложение шлюза доставляется игроку действия. Речь о движении (`enter`/`leave`, `set position`) и отдыхе (`rest`, `set hp`). Consumer находит игрока по цепочке предложения: `proposal_id` — id предложения, предложение — `Derive` от `player.*`. Доставка `kind=system` несёт текст `render` без кода State и без внешних ID. До T-307 такой отказ (например, `version_conflict` при отстающей проекции) виден только в журнале. Unit: `version_conflict` на предложение `enter` и на предложение `rest` → по одной доставке игроку; повтор того же отказа шиной второй доставки не создаёт.

### T-308: `shared/testkit/gateway` — `FakeGateway` и HTTP-обвязка рядом с `Harness` v0 · I1 · подволна 1.8 · developer#1 · M · **поставка в develop**

**Ветка:** `task/T-308-testkit-gateway` (`.worktrees/T-308`).
**Зависит от:** T-303…T-307. **Внешнее:** `Harness` v0 в `shared/testkit/gateway` (T-018, T-400, T-433): `NewHarness(bus, fixtures)`, `Start`/`Wait`/`Observe`, `CreatePlayer`, `Enter`, `Leave`, `Look`, `Say`, `Rest`, `Attack`, `Flee`, `Fight`, `Status`, `Position`, `Version`, `WithLog`, `WithTimeout`. *(Сверка 2026-09-13: заменено «сигнатуры `Harness` v0 из F-10 (EPIC-001)»; заголовок был «`FakeGateway` и `Harness` (замена v0 из F-10)».)*
**Ссылки:** C-04 «Заглушка», C-08 «Заглушка», `contracts.md` §17; component §15; design §3.1 п. 6, §3.3; ADR-010; NFR-092.

**Описание.** `shared/testkit/gateway/`: `FakeGateway` — in-process HTTP-сервер поверх реального `internal/gateway` на `membus` (для бота и e2e); `Harness` — Go-клиент `internal/gateway/client` + фикстуры `player-A/B/C` платформы `ci` + `RegisterAndEnter`, `Act`, `AwaitDelivery(kind, timeout)`, `CloseRound` (в I1 — заглушка `501`/`no_open_round`), режим генератора `player.*` прямо в `membus` без HTTP (для EPIC-002/003). `--id-source=sequence`, `clock.Manual`.
**Сверка 2026-09-13.** (1) API `Harness` v0 **не меняется**: его используют тесты EPIC-002/003 и стенд T-400, а генератор `player.*` в шину без HTTP в нём уже есть. `RegisterAndEnter`, `Act`, `AwaitDelivery`, `CloseRound` — методы новой HTTP-обвязки рядом (отдельный тип в том же пакете или подпакет), а не замена v0. (2) `FakeGateway` поднимает настоящий `internal/gateway`, а правило depguard `shared` запрещает `shared/testkit/**` импорт `internal/*` (исключения — только `testkit/mechanics` и `testkit/swarm`). Место `FakeGateway` и исключение для `internal/gateway` — **по решению system-architect** (запрос открыт). Вариант без исключения — `FakeGateway` в `internal/gateway/gatewaytest`, обвязка в `shared/testkit/gateway` только на `internal/gateway/client` (тоже `internal/*`, то же решение).

**DoD** (+ DoD-common):
- [ ] **API `Harness` v0 не изменён**: экспортируемые имена v0 в `git diff` не тронуты; `go build ./... && go vet ./...` и `go test -short ./shared/testkit/...` зелёные без правок потребителей v0 (EPIC-002/003, стенд T-400). *(Сверка 2026-09-13: заменено «Сигнатуры v0 сохранены… после подтягивания `integration/mvp-1`; отличия… согласованы с tech-lead#1».)*
- [ ] **(сверка 2026-09-13)** Граница импортов — по решению system-architect (место `FakeGateway`, исключение depguard); `golangci-lint run` чист без `//nolint` на импортах.
- [ ] `FakeGateway` поднимается и гасится в тесте за ≤ 1 с, без внешних зависимостей (`-short`), детерминированные ULID и часы.
- [ ] Unit самой обвязки: `RegisterAndEnter` доводит фикстуру до `alive` в регионе; `Act` возвращает `correlation_id`; `AwaitDelivery` не залипает при отсутствии доставки (таймаут → понятная ошибка).
- [ ] `Harness` использует **тот же** `internal/gateway/client`, что и бот (NFR-092) — проверка compile-time.
- [ ] `shared/testkit/gateway` не импортируется из продакшн-кода (правило depguard `no-testkit-in-production` уже есть в `.golangci.yml`).
- [ ] Отчёт содержит уведомление TEAM-1/TEAM-2 о замене v0 и инструкцию перехода (2–3 строки для `dev-log`/`journal`).

### T-309: `snapshot` gateway, режимы `live|replay`, `Journal.End()`, полный `/health` · I1 · подволна 1.10 · developer#1 · M

**Ветка:** `task/T-309-snapshot-replay-health` (`.worktrees/T-309`).
**Зависит от:** T-304, T-306, T-307. **Внешнее:** `Journal.End()` из C-01 v1.1 (F-5t), `objstore` (F-4c).
**Ссылки:** C-14 v1.1, C-01 v1.1, ADR-003 п. 8; component §11.2, §11.4, §16 п. 7; design §3.1 п. 5; US-011; FR-031…FR-033; NFR-010/011, NFR-014, NFR-016.

**Описание.** `snapshot/writer.go` — объект `snapshots-{world}/gateway/{ts}-{seq}.json` = `{cursors, projection_hash, active_sessions[], open_rounds[], laws_version}` + событие `snapshot.created component=gateway`, по `SIGTERM` и по каждому `analytics.session.ended`, хранить последние 5, указатель `latest.json` (C-14 v1.1). Режимы: `--mode=replay` — таймеры/sweeper/лизинг выключены, `round.*`/`analytics.*` читаются, а не публикуются; чтение `Journal.ReadRange` от курсоров до `End()` по каждому из 4 топиков, переключение в `live` при достижении конца. Полный `/health` по component §11.4. Восстановление сессий/ходов при старте (component §7.6).
**Сверка 2026-09-13.** (1) Если gateway при старте ждёт `analytics.replay.completed`, ожидание ограничено `MV_GATEWAY_REPLAY_WAIT`. Переменная объявляется в `shared/env/vars.go` этой задачей вместе с кодом чтения (`contracts.md` v0.6, строка C-14 «объявляются тем изменением, которое вводит их чтение»); без кода ожидания — не объявляется. (2) Порядок остановки процесса: HTTP-сервер → контексты в обратном порядке старта → шина (`cmd/multiverse/serve.go`, ADR-023 п. 4). Финальный снапшот по `SIGTERM` пишется в `Stop` gateway: HTTP уже закрыт, шина ещё открыта, публикация `snapshot.created` допустима. `Stop` сначала отменяет контекст подписок и таймеров (C-01 v1.7), укладывается в общий `runtime.StopTimeout` (15 с); панику в `Stop` ловит `runtime.StopAll`. (3) Окно дедупа в памяти, если оно есть, входит в снапшот (C-14 v1.2).

**DoD** (+ DoD-common):
- [ ] Unit: снапшот пишется по `SIGTERM` и по `session.ended`; хранится 5 последних, старые удаляются; `latest.json` указывает на последний; `snapshot.created` валиден по схеме C-14.
- [ ] Unit `replay`: таймеры не создаются, sweeper не запускается, `analytics.*`/`round.*` не публикуются; `Publish` действий харнесса разрешён.
- [ ] Unit перехода `replay → live` по `Journal.End()`; «хвост» событий после `End` не применяется повторно (дедуп `processed_events`, component §16 п. 7).
- [ ] Unit восстановления: активные сессии с `last_action_at` старше 30 мин → `ended idle`; открытые `turns` с истёкшим дедлайном → `timeout`.
- [ ] `/health` отдаёт все поля component §11.4; `fail` при недоступности любой БД; `degraded` при `projection=missing/stale` или `core_admin unavailable`; проверка не чаще раза в 10 с (NFR-016).
- [ ] Повреждённый/отсутствующий снапшот → сообщение в `/health` и лог, сервис не поднимается «молча с пустым состоянием» (US-011).
- [ ] **(сверка 2026-09-13)** Unit порядка `Stop`: подписки и таймеры остановлены до записи снапшота; снапшот и `snapshot.created` записаны до возврата `Stop`; повторный `Stop` безопасен; `Stop` укладывается в срок переданного контекста.
- [ ] **(сверка 2026-09-13)** `MV_GATEWAY_REPLAY_WAIT` объявлена ровно вместе с кодом ожидания и есть в `.env.example`; по истечении срока — `/health degraded` (C-14), unit на управляемых часах.
- [ ] **(приёмка T-303, 2026-09-13; ревью #1 T-303, N-3)** `links_store` и `gateway_store` в `Health` gateway — настоящая проверка БД вместо констант `ok`. Проверка не занимает единственное соединение `links.db` (`SetMaxOpenConns(1)`) дольше короткого дедлайна: результат кэшируется не чаще раза в 10 с, при занятом соединении отдаётся последнее известное значение. Unit: закрытая БД → `fail`; `Health` при удерживающем читателе отвечает без ожидания `busy_timeout`.
- [ ] **(приёмка T-303, 2026-09-13; ревью #2 T-303, R2-N-3)** `Stop` gateway не держит `c.mu` на ожидании sweeper'а и сжатия: `started` снимается и ссылки копируются под мьютексом, ожидание и `Compact` идут вне его. Unit: пока `Stop` ждёт заблокированное сжатие, `Health` отвечает `fail` не дольше 100 мс.
- [ ] **(ревью #1 T-304, Mi-1, 2026-09-13)** Сверка `projection_hash` снапшота gateway с `state_hash` State — **только на фактах C-02 v1.6** (T-448, EPIC-002; контрольное слияние EPIC-002 → `develop` — до T-309). Под v1.5 удаление ключа и `set null` неразличимы (`new: null`), проекция хранит `null`, и после удаления ключа хеш расходится с State (`readmodel.changedPath`). Unit сверки хеша — на фактах v1.6; если к началу T-309 в `develop` ещё v1.5, сверка не включается, и это записано в карточке.
- [ ] **(ревью #1 T-304, N-1, 2026-09-13)** `projection=stale` не остаётся до рестарта: правило снятия — по решению architect#3 (component §11.4). Варианты: перезагрузка проекции из более свежего снапшота State (`LoadFromStateSnapshot` уже сбрасывает признак) или догон до `End` без разрывов. Unit на выбранное правило: разрыв версий → `stale` → условие снятия → `ok`.

### T-310: Бот — `config`, `access.Gate`, `updates`, `sender`, `commands`, `privacy` · I1 · подволна 1.4 · developer#2 · M · **Статус: done**

**Ветка:** `task/T-310-bot-core` (`.worktrees/T-310`).
**Зависит от:** T-301 (DTO/клиент), T-303 (коды ошибок и контракт `links/*`). Работает на `httptest`-моке клиента, `FakeGateway` не нужен.
**Ссылки:** ADR-018 (+ доп. п. 1–4), ADR-006 доп. п. 1–5; SEC-06, SEC-07, SEC-08, SEC-10, SEC-11; component §10.1–10.4, §11.1, §11.3; design §3.1 п. 7; US-008; FR-002, FR-130, FR-131; NFR-044, NFR-049.

**Описание.** `cmd/telegram-bot/internal/config` (манифест `MV_*`, обязательный `MV_TELEGRAM_BOT_TOKEN`, `MV_TELEGRAM_ALLOWED_USER_IDS`), `internal/access` (`Gate`: игнор не-`message`, отказ на групповой чат, allowlist по `from.id` (пустой = всем отказ), token bucket 20/мин; выполняется **до** любого вызова gateway), `internal/updates` (`UpdateSource` + `go-telegram/bot` v1.25 + `fake.go`), `internal/sender` (`Sender` без параметра `parse_mode` + retry `429`/`5xx` + `fake.go`), `internal/commands` (`Parse` по словарю FR-002, синонимы без «/», длины), `internal/privacy` (`slog.Handler` с запретом атрибутов `external_id`/`chat_id`/`username`/`text`; `Redact` токена `bot<digits>:<token>` → `bot<redacted>`; `ErrorLog`; запрет `WithDebug`).
**Сверка 2026-09-13.** (1) Имена переменных — по манифесту `shared/env/vars.go`: объявлены `MV_TELEGRAM_BOT_TOKEN`, `MV_TELEGRAM_ALLOWED_USER_IDS`, `MV_TELEGRAM_GATEWAY_URL`, `MV_TELEGRAM_POLL_TIMEOUT_S`, `MV_TELEGRAM_HEALTH_ADDR`, и они читаются через `env.Telegram*`. Переменных `MV_BOT_*` из component §11.3 в манифесте нет. Нужные задаче (соль `action_key`, лимит команд, `client_id`, TTL диалога) объявляются с префиксом `MV_TELEGRAM_` вместе с кодом; окончательные имена согласует architect#3 правкой component §11.3. (2) Граница импортов: правила depguard для `cmd/telegram-bot` нет; правило «из `internal/*` только `internal/gateway/{client,api}`» — запрос system-architect, до решения — тест импортов. (3) `github.com/go-telegram/bot` вносится в `go.mod` этой задачей. (4) Лимит команд и TTL диалога — на часах `shared/clock`: бот — отдельный бинарник того же модуля, `forbidigo` действует и на него.

**DoD** (+ DoD-common):
- [ ] Unit SEC-06: `from.id` вне allowlist → один отказ «доступ по приглашению», `links/resolve` **не вызван** (мок клиента фиксирует ноль вызовов), id не в логе, счётчик `bot_denied_total` увеличен.
- [ ] Unit SEC-06 fail-closed: пустой allowlist → отказ всем.
- [ ] Unit SEC-07: `message.chat.type != "private"` → **ответа нет** (ни участнику allowlist, ни постороннему), gateway не вызван, в лог — одна строка `reason=not_private` без id на серию; связка строится по `from.id`, `chat.id` отдельно не хранится. *(Приёмка T-310, 2026-09-13: заменено «одно сообщение „пишите в личные“» — решение оркестратора по ревью #1 T-310, приоритет US-008 (негативный критерий), FR-131, NFR-044 перед ADR-018 п. 3; правка ADR-018 и component §10.2/§11.1 — architect#3.)*
- [ ] Unit SEC-11/NFR-049: 21-я команда за минуту → «слишком часто, подождите N с» без вызова gateway; на 61-й секунде принимается.
- [ ] Unit SEC-08: строка ошибки HTTP-клиента Telegram с `bot123456:ABC…` после `Redact` не содержит токен; `409 Conflict` редактируется; `WithDebug` не используется (тест на конфигурацию); выход с кодом 3 при `409 getUpdates`.
- [ ] Unit SEC-10: `Sender.Send` доставляет `[x](http://…)`, `<a href=…>`, `*текст*` **буквально**; сигнатура `Send` не содержит параметра разметки.
- [ ] Unit `commands.Parse`: весь словарь FR-002 (`enter/leave/look/attack/flee/say/rest/status/defend/group */start//help//forget`), синонимы без «/», неизвестная команда → `unknown`, длина `say` ≤ 500 с подсказкой.
- [ ] Unit `privacy.Handler`: атрибуты `external_id`, `chat_id`, `username`, `text` отбрасываются на всех уровнях, включая `debug`.
- [ ] `action_key = hex(HMAC-SHA256(<соль>, update_id))[:32]` — стабилен при повторной обработке того же `update_id`, необратим; соль по умолчанию — производная от токена, в лог не попадает. *(Сверка 2026-09-13: имя переменной соли — `MV_TELEGRAM_*` по согласованию с architect#3, было `MV_BOT_ACTION_KEY_SALT`.)*
- [ ] Все переменные бота — `MV_TELEGRAM_*` манифеста; новые объявлены вместе с кодом и есть в `.env.example`. *(Сверка 2026-09-13: заменено «Все `MV_BOT_*`/`MV_TELEGRAM_*` объявлены через манифест».)*
- [ ] **(сверка 2026-09-13)** Граница импортов бота: тест (`go list -deps ./cmd/telegram-bot/...`) — из `multiverse-core.io/internal/` только `internal/gateway/client` и `internal/gateway/api`; правило depguard — по решению system-architect (если оно принято до приёмки — добавлено и `golangci-lint run` чист).
- [ ] **(сверка 2026-09-13)** `github.com/go-telegram/bot` добавлен в `go.mod`/`go.sum` и отмечен в отчёте.

**Приёмка (tech-lead#3, 2026-09-13): принято, статус `done`** — карточка [`tasks/T-310.md`](tasks/T-310.md), раздел «Приёмка (tech-lead)». Ревью: 2 итерации, итог 0/0/1/2. Mi-4, N-7 и N-8 ревью #2 закрыты при приёмке (код и тесты в `access`/`sender`, мутанты в копии дерева): отказы шлюза — отдельным отправителем `sender.NewBestEffortTelegram` (одна попытка, таймаут клиента ≤ `BestEffortHTTPTimeout` = 5 с), общий бюджет ответов «Доступ по приглашению.» — `access.MaxInviteRepliesPerWindow` = 10 за окно на весь бот (сверх — молча, счётчик и строка лога `answered=false`; ответы «Слишком часто» игрокам allowlist бюджетом не ограничены); id отклонённых чатов вычищаются первым же обновлением после окна. Уточнения DoD по факту: SEC-07 — молчание (строка выше); SEC-06 «один отказ» — один ответ на чат за 60 с и не больше 10 ответов на весь бот за 60 с, `bot_denied_total` считает каждое обновление; лимит SEC-11 — скользящее окно вместо token bucket. Граница импортов — только тестом `go list -deps`: правило depguard `cmd-telegram-bot` не принято (предусловие старта T-311). С кончиком эпика abfc04b по `shared/env`, `.env.example`, `go.mod`, `go.sum`, `cmd/` не пересекается; пробная сборка и тесты на копии кончика зелёные. Слияние — после отметки tech-lead#1 по `shared/env/vars.go`, `.env.example` и новой зависимости `go-telegram/bot` v1.25.0 в `go.mod`/`go.sum`.

### T-311: Бот — `flow` (онбординг, `/forget`) и `render` (тексты, клавиатуры, ошибки) · I1 · подволна 1.5 · developer#2 · M

**Ветка:** `task/T-311-bot-flow-render` (`.worktrees/T-311`).
**Зависит от:** T-310, T-318 (текст уведомления; до его готовности — заглушка из US-008, design допущение 2).
**Ссылки:** ADR-018, SEC-26; component §10.3, §10.5; design §3.1 п. 7; US-001, US-008, US-009; FR-009, FR-052, FR-060, BR-09; NFR-045.

**Описание.** `internal/flow` — FSM на чат (`idle → awaiting_consent → awaiting_name → awaiting_name_confirm → awaiting_world → ready`, TTL 15 мин; `/forget` → `/forget confirm`; кэш `chat_id → player_id` TTL 1 ч; `Reset` после forget). `internal/render` — `notice.go` (единственный файл с текстом FR-009), `help.go`, ошибки по `code` (component §10.5), рендер `Delivery` (пометки «текст создан ИИ» / «упрощённый режим (шаблон): {fallback_reason}»), клавиатуры (согласие, базовая, боевая, цели, регионы), нарезка сообщений > 4096 по абзацам.

**DoD** (+ DoD-common):
- [ ] Unit онбординга на `FakeUpdateSource`/`FakeSender`: `/start` → уведомление + клавиатура; «Отказаться»/произвольный текст → повтор уведомления, персонаж не создаётся (US-001, UC-001 E1); подтверждение → `links/consent` с `notice_shown/consent/age_confirmed/shown_at`.
- [ ] Unit имени: 2–32, буквы/цифры/пробел/дефис, без управляющих (та же регулярка, что на сервере); совпадение с `username`/`first_name` без учёта регистра → `awaiting_name_confirm` с предупреждением; значение `username` сравнивается и **отбрасывается**, не логируется и не сохраняется (US-009).
- [ ] Unit TTL: состояние диалога протухает через 15 мин; повторный `/start` восстанавливает по `resolve`.
- [ ] Unit `/forget`: без `confirm` — только вопрос, вызова gateway нет; `/forget confirm` → `DELETE /v1/links`, затем `flow.Reset` и текст «Связка удалена. /start — начать заново»; ответ «нечего удалять» при отсутствии связки.
- [ ] Golden-тест `render`: тексты уведомления, `/help`, ошибок по кодам `in_encounter`, `not_in_encounter`, `not_leader`, `already_acted`, `character_dead`, `consent_required`, `rate_limited`, `bus_unavailable`; неизвестный код → `message` сервера.
- [ ] Текст FR-009 живёт **только** в `render/notice.go` (проверка grep-тестом: обязательные пункты присутствуют — ИИ, 18+, обработка текста, `/forget` и что он удаляет, retention/бэкап, облако).
- [ ] **(приёмка T-318, 2026-09-13; Mi-6, R2-Mi-2; уточняет пункт выше)** Константы `NoticeText`, `ConsentButton`, `DeclineButton`, `DeclineReply` — дословно из карточки [`tasks/T-318.md`](tasks/T-318.md) §1 (длина `NoticeText` — 2 341). Grep-тест: `lower := strings.ToLower(NoticeText)`, для каждой из **24** подстрок списка §3 карточки `strings.Count(lower, s) == 1`; `strings.Count(lower, "30 дней") == 3`; `utf8.RuneCountInString(NoticeText) <= 3000`; в `NoticeText`, `DeclineReply` и кнопках нет `` ` ``, `*`, `_`, `[`, `<`. Мутанты «удалён раздел 2», «удалён раздел 6», «сроки 90 и 180 поменялись местами», «18 лет → 16 лет в разделе 2» — тест красный.
- [ ] **(приёмка T-318; R2-N-2)** Перед сравнением подстрок текст и подстроки нормализуются одним `strings.NewReplacer("ё", "е", "\u00a0", " ", "\u202f", " ", "–", "—")`: косметическая правка типографики тест не роняет, смысловая роняет. Unit: `NoticeText` с заменой `ё`→`е` и U+00A0 перед «дней» проходит проверку подстрок. К кнопкам нормализация не применяется.
- [ ] **(приёмка T-318; N-2)** В `ConsentButton` и `DeclineButton` нет U+00A0, U+200B, U+200C, U+200D, U+2060, U+FEFF; `strings.TrimSpace(b) == b`; `utf8.RuneCountInString(ConsentButton) <= 30`. `flow` сравнивает текст от клиента с кнопкой точным совпадением.
- [ ] **(приёмка T-318; Р-1, component §10.3)** Одна константа кнопки — для клавиатуры, для проверки нажатия во `flow` и для текста: `flow` сравнивает с `render.ConsentButton`/`render.DeclineButton`, своей строки у `flow` нет; тест `strings.Contains(NoticeText, "«"+ConsentButton+"»")`. Текст кнопок в component §7.1/§10.3 правит system-architect; расхождение с КД до правки — не повод возврата.
- [ ] **(приёмка T-318; решение оркестратора Р-3 A, US-008, UC-001 E1; уточняет первый пункт DoD — «Отказаться» → повтор уведомления)** «Отказаться» в `awaiting_consent` → только `DeclineReply` и клавиатура согласия, `NoticeText` сразу не повторяется. Произвольный текст или игровая команда в `awaiting_consent` → `NoticeText` и клавиатура согласия. Персонаж не создаётся, `links/consent` не вызывается. Нажатие `ConsentButton` вне `awaiting_consent` (в том числе после истечения TTL 15 мин) → `NoticeText` и клавиатура, `links/consent` не вызывается. Golden `render` для `DeclineReply`; unit `flow` на все три ветки.
- [ ] **(приёмка T-318; решение оркестратора Р-2 A, I-4, У-8)** `/forget` принимается в любом состоянии онбординга, в том числе в `awaiting_consent` (component §16 п. 6, US-009): `/forget` → вопрос подтверждения, `/forget confirm` → `DELETE /v1/links`, затем `flow.Reset`. Unit на мок клиента: `awaiting_consent` → `/forget` → `/forget confirm` → ровно один `DELETE /v1/links`. КД §10.3 правит system-architect; расхождение с КД до правки — не повод возврата.
- [ ] **(приёмка T-318; Mi-5, FR-009, BR-09, NFR-045)** `links/consent` вызывается **только** при точном совпадении текста с `ConsentButton` в `awaiting_consent`. `/help` у несогласившегося (`none`, `pending_consent`) показывает `NoticeText` и `links/consent` не вызывает; у согласившегося `last_seen_at` обновляется через `links/resolve`. Unit: `/help` в `none` и `pending_consent` — ноль вызовов `consent` на моке.
- [ ] **(приёмка T-318; I-2)** Ответ игроку после исчерпания повторов на `503 forget_incomplete` не содержит «удален» (golden `render` кода `forget_incomplete` и проверка `strings.Contains(..., "удален") == false`): иначе игрок прочтёт «связка удалена», пока строка ещё не затёрта. Остальное — пункт приёмки T-303 ниже.
- [ ] Пометка `generated_by` присутствует у 100 % нарративных доставок (NFR-045).
- [ ] Клавиатуры не содержат разметки; все сообщения уходят через `Sender` без `parse_mode` (SEC-10).
- [ ] **(приёмка T-303, 2026-09-13; решение system-architect#1 по `503 forget_incomplete`)** `/forget confirm` при `503 forget_incomplete`: клиент повторяет сам (`DefaultBackoff`); после исчерпания повторов бот **не** говорит «связка удалена», а просит повторить `/forget confirm` позже. `200 {deleted:false}` с `ForgetResult.Repeated = true` (ответ после 503 в том же вызове) — конец того же забвения, не «нечего удалять». Golden-тест `render` для кода `forget_incomplete`; unit `flow` на обе ветки. Чтение `Retry-After` клиентом — по решению T-456.
- [ ] **(ревью #1 T-310, 2026-09-13)** Тест SEC-06 (`access.Gate` до gateway) переведён с сырого HTTP на `client.New` против того же `httptest`-мока со счётчиком — после решения по правилу depguard для `cmd/telegram-bot`.
- [ ] **(ревью #1 T-310, M-1)** `flow` не вызывает gateway с уже отменённым `ctx`: обработчик получает контекст, который отменяется при остановке, `409` и `401` опроса. Unit: отменённый `ctx` → ноль запросов к моку gateway, ответа игроку нет.
- [ ] **(ревью #1 T-310, M-2)** Ответы `flow` уходят через `Sender` с короткой политикой повторов, а не `sender.DefaultPolicy` (до 5 ожиданий `429` по ≤ 60 с): пауза в единственном обработчике держит команды всех игроков. Unit: `429 retry_after=30` на ответ `flow` не задерживает обработку следующего обновления дольше выбранной политики (на `clock.Timers`, без реального ожидания).
- [ ] **(ревью #1 T-310, N-6)** `flow` и `render` пишут в лог только заранее заданные ключи; внешние id, имена и тексты не попадают в лог ни под какими ключами, включая группы `from`/`chat`/`message`/`update` (unit поверх `privacy.Handler`).
- [ ] **(приёмка T-310, 2026-09-13) Предусловие старта.** В `.golangci.yml` принято правило depguard `cmd-telegram-bot` (решение system-architect/tech-lead#1, в очереди): `files: **/cmd/telegram-bot/**`, allow `multiverse-core.io/internal/gateway/client$` и `…/internal/gateway/api$`, deny `multiverse-core.io/internal` и `multiverse-core.io/shared/eventbus`, исключение `!**/cmd/telegram-bot/**` в `internal-unlisted`; правило для `_test.go` — если тест бота импортирует `internal/gateway` напрямую. Без правила `flow` не может импортировать клиента: `internal-unlisted` запрещает боту любые `internal/*`. Задачу не начинать до слияния правила в ветку эпика; тест `go list -deps` остаётся второй линией.
- [ ] **(приёмка T-310, 2026-09-13; ревью T-456, бэклог п. 1)** После `client.ForgetResult{Repeated: true, Deleted: false}` бот **не** сверяет итог через `Resolve`: `links.Store.Resolve` при отсутствии строки вставляет `pending_consent` с внешним ID, и такая проверка после забвения снова пишет его в `links.db` (SEC-04/05, US-009). `200 {deleted:false}` — итог «связки нет», игроку — «Связка удалена», не «нечего удалять»; повторного обращения к gateway нет. Исправить doc `client.ForgetResult`, который советует `Resolve` (EPIC-004, `internal/gateway/client`). Unit `flow` на фейковом клиенте: после `/forget confirm` с потерянным ответом (`Repeated=true`) вызовов `links/resolve` 0; тест стора или e2e T-314: строк в `links` для аккаунта 0.

### T-312: Бот — `deliver` (long-poll → send → ack) и `main` wiring · I1 · подволна 1.6 · developer#2 · M

**Ветка:** `task/T-312-bot-deliver-main` (`.worktrees/T-312`).
**Зависит от:** T-310, T-311.
**Ссылки:** ADR-006, C-08; `docker-compose.bot.yml` (T-397), `contracts.md` §16 п. 5; component §10.4, §11.1 (retention логов); design §3.1 п. 7; US-008; FR-003, FR-004; NFR-003, NFR-092.

**Описание.** `internal/deliver/loop.go` — цикл `Deliveries(wait=25s)` → последовательный `Send(chatID = route.external_id, render.Delivery(d))` → пакетный ack; обработка `429` (`retry_after`), сети/`5xx` (3 попытки, затем **без ack**), `403 bot was blocked`/`400 chat not found` (ack, лог без ID). `cmd/telegram-bot/main.go` — конфигурация, сборка зависимостей, graceful shutdown, код выхода 3 при `409 getUpdates`.

**DoD** (+ DoD-common):
- [ ] Unit: порядок сообщений на `chat_id` сохраняется, бот ничего не переупорядочивает; ack пакетом после обработки всех доставок ответа.
- [ ] Unit `429`: пауза `retry_after`, затем повтор; unit `5xx`: 3 попытки, ack не отправляется, доставка повторно выдаётся gateway через 30 с.
- [ ] Unit `403 blocked`/`400 chat not found`: доставка подтверждается (ack), в логе нет внешнего ID.
- [ ] Unit: механика и нарратив приходят игроку **разными** сообщениями (US-008, FR-003).
- [ ] Курсор доставок между рестартами не хранится (component §16 п. 5) — поведение подтверждено тестом (после рестарта неподтверждённые выдаются снова).
- [ ] `main` стартует с пустым `MV_TELEGRAM_BOT_TOKEN` → понятная ошибка и выход, токена в сообщении нет (SEC-08).
- [ ] **(сверка 2026-09-13)** Бинарник совместим с `docker-compose.bot.yml` без правки файла (владелец — devops, EPIC-001): `entrypoint /telegram-bot`; подкоманда `telegram-bot health --url <url>` для healthcheck (образ без оболочки и curl) — код 0 при `ok`, иначе ненулевой; умолчание `--url` строится из `MV_TELEGRAM_HEALTH_ADDR` (`:8089`: пустой хост → `127.0.0.1`, как `defaultHealthURL` в `cmd/multiverse/health.go`); `/health` бота слушает `MV_TELEGRAM_HEALTH_ADDR`; адрес gateway — `MV_TELEGRAM_GATEWAY_URL` (умолчание манифеста `http://127.0.0.1:8088`, в compose — `http://gateway:8088`). Unit на подкоманду и на умолчание URL. Новые переменные для compose — devops через отчёт. *(Заменено: «`docker-compose` профиль `bot` не правится (владелец — devops); необходимые переменные переданы devops через отчёт».)*
- [ ] **(ревью #1 T-310, Mi-1)** `main` завершается `os.Exit(updates.ExitCode(err))`: `updates.ErrConflict` → 3; `updates.ErrUnauthorized` — и при старте (`getMe`), и во время опроса (`401 getUpdates`, отозванный токен) → понятная ошибка без токена и код 1, опрос не крутится. Unit на оба случая.
- [ ] **(ревью #1 T-310, Mi-1)** `deliver` решает про ack по типу ошибки `Sender`: `ErrBlocked`, `ErrChatNotFound`, `ErrRejected` → ack; `ErrUnavailable` и `ErrUnauthorized` (`401` при `sendMessage`) → **без** ack, доставка остаётся у gateway. Unit на `401`.
- [ ] **(ревью #1 T-310, M-2; уточнено приёмкой T-310, Mi-4 ревью #2)** `access.Gate` собирается с **отдельным** отправителем `sender.NewBestEffortTelegram` (одна попытка, свой HTTP-клиент с таймаутом ≤ `sender.BestEffortHTTPTimeout` = 5 с), а не `WithPolicy(BestEffortPolicy)` отправителя `flow`/`deliver`: тот делит клиент с таймаутом 30 с. Ответы `deliver` — с политикой по умолчанию в своей горутине, не в обработчике обновлений. Unit `main`: сборка зависимостей — шлюзу best effort, `deliver` — `DefaultPolicy`; мок Bot API не отвечает на отказ → следующая команда игрока доходит до мока gateway не позже таймаута клиента шлюза. *(Заменено: «`sender.Telegram.WithPolicy(sender.BestEffortPolicy)`».)*
- [ ] **(ревью #1 T-310, M-1)** `/health` бота выводит `access.Counters` (`bot_denied_total`). В риски и в отчёт devops для runbook: при `SIGTERM` может потеряться одно обновление, взятое в обработку (опрос с `WithUpdatesChannelCap(0)`; остаток пачки Telegram отдаёт повторно, повтор гасится `action_key`).
- [ ] **(ревью #1 T-310, N-3)** Тест границы импортов `go list -deps` (`config/imports_test.go`) перенесён в пакет `cmd/telegram-bot`.

### T-313: e2e соло — `solo-30`, `death`, `flee-fail` · I1 · подволна 1.9 · developer#1 · M

**Ветка:** `task/T-313-e2e-solo` (`.worktrees/T-313`).
**Зависит от:** T-308. **Внешнее:** `FakeState` (C-02), `FixedMechanics` — EPIC-002; `FakeEncounter` (T-219: Phase 1 боя — `dice.rolled`, `combat.decided`, атомарный пакет, `encounter.*`) и `FakeNarrator` (T-220) — EPIC-003. В одном процессе их монтирует хук `MV_SWARM_FAKE=true` под именем `swarm` (T-255). *(Сверка 2026-09-13: добавлен `FakeEncounter` — без него бой в e2e никто не решает.)*
**Ссылки:** ADR-010; component §15; design §3.1 п. 8, §7; US-001, US-007 (соло), US-008; UC-001…005, 007, 009, 012; NFR-013.

**Описание.** `//go:build e2e`, один процесс `--contexts=all --mode=replay --bus=memory --id-source=sequence`, часы `clock.Manual`. Сценарии: `solo-30` (регистрация → согласие → персонаж → `enter` → встреча → 30 ходов боя/`look`/`say`/`rest` → выход), `death` (персонаж гибнет → дальнейшие действия `character_dead`, предложение `/start`), `flee-fail` (неудачное бегство → удар вслед).

**DoD** (+ DoD-common):
- [ ] Три сценария зелёные и детерминированные (два прогона подряд дают одинаковый журнал событий).
- [ ] `solo-30` проходит 30 ходов без ошибок; на каждый ход есть `analytics.turn.completed`; сессия открыта и закрыта парно.
- [ ] Прогон с `--chaos=duplicate` зелёный (NFR-013): дубли событий не порождают вторых доставок и вторых применений.
- [ ] Таймер в тесте подтверждает `202` ≤ 100 мс на `membus` (вклад в NFR-003; абсолютный замер — стенд T-392).
- [ ] Прогон формирует/обновляет фикстуру `solo-30.jsonl` для EPIC-005 — в месте, согласованном в T-306 (`testdata/analytics/**` — владение EPIC-005). *(Сверка 2026-09-13.)*
- [ ] Тест помечен `e2e`, в `-short` не запускается, в CI — в job, где заглушки доступны.

### T-314: e2e приватности и восстановления — `forget`, `privacy-scan`, `recovery` · I1 · подволна 1.11 · developer#1 · M

**Ветка:** `task/T-314-e2e-privacy-recovery` (`.worktrees/T-314`).
**Зависит от:** T-309, T-313.
**Ссылки:** SEC-03, SEC-04/05, SEC-08, SEC-26; **`contracts.md` v0.4 C-02 v1.2, C-04 v1.1 (каскад `/forget`), C-08 v1.2, C-10 v1.1**; `consolidation.md` §14.1 (З-1, З-2); component §15, §11.2; design §3.1 п. 8, §7; US-009, US-011; FR-023, FR-061; NFR-010/011, NFR-041/042.

**Описание.** `forget` (регистрация → игра → `/forget confirm` → доставки не выдаются, повторный `/start` даёт новый `player_id`), `privacy-scan` (регистрация с известным тестовым внешним ID → grep по `gateway.db` + WAL, по событиям `membus` (включая `analytics.*`), по логам обоих процессов, в т. ч. на фрагмент токена → 0), `recovery` (10 ходов → рестарт контекста gateway in-process → сессии, раунды, outbox, курсоры сохранены, `identical=true`).

**DoD** (+ DoD-common):
- [ ] `privacy-scan` зелёный при `MV_LOG_LEVEL=debug`: 0 вхождений тестового внешнего ID и 0 вхождений фрагмента токена во всех перечисленных источниках (NFR-041, SEC-03, SEC-08).
- [ ] `forget`: `strings` по `links.db` и WAL сразу после вызова = 0 (SEC-04/05); недоставленные сообщения → `dropped`; сессия закрыта с **`end_reason = forget`** (З-1); повторный `/start` → новая связка и новый `player_id` (US-009).
- [ ] **(сведение 3, З-2; C-04 v1.1)** каскад `/forget` проверяется **в обязательном порядке публикаций**, `t.Skip` больше не допускается: (1) `outbox.DropForPlayer` (`pending → dropped`); (2) при `status=alive` — один `entity.update.proposed {atomic: true, cause: forget}` (`set status=abandoned` + `expected_version`; при лидерстве — в том же пакете `set leader_id = <старейший alive | null>`); (3) `group.left {cause: forget}` и, при смене лидера, `group.leader_changed {cause: forget, leader: …|null}`; (4) `session.End(forget)` → `analytics.session.ended {end_reason: forget}`; (5) физическое удаление связки. Тест сверяет и состав, и **порядок** событий.
- [ ] **(сведение 3, З-2)** ветка `status=dead`: шаг (2) пропускается (`dead` терминален, FR-023) — предложение не публикуется, остальные шаги выполняются; ветка `status=creating`: предложение публикуется при получении `entity.created` для `player_id` без связки. Во встрече gateway ничего дополнительно не публикует (встречу закрывает агент по `entity.updated status=abandoned`).
- [ ] `recovery`: после рестарта совпадают `cursors`, `projection_hash`, состав активных сессий, содержимое outbox; повторного применения событий нет (US-011).
- [ ] Скан включён в job `security` CI (сканирование `testdata/` — по чек-листу security-review п. 2).
- [ ] **(приёмка T-303, 2026-09-13)** `forget` под удерживающим читателем `links.db` (открытая транзакция чтения, как в `internal/gateway/compaction_test.go`): `/forget` → `503 forget_incomplete` с `Retry-After`, `/health` gateway `degraded` с `links_compaction: pending`; после ухода читателя повтор → `200 {deleted:false}`, скан `links.db` и `-wal` = 0, `/health` → `ok`. Сюда же переносится пункт T-306 об окне снятия отметки, если T-306 его не закрыла.
- [ ] **(решение оркестратора по приёмке T-318, 2026-09-13; У-8, Р-2 A)** Ветка e2e «забвение до согласия»: `/start` → `/forget` → `/forget confirm` без согласия (связка в `pending_consent`). Сразу после вызова в `links.db` и WAL нет внешнего ID (`strings` = 0).

### T-315: e2e бота на `FakeUpdateSource`/`FakeSender`/`FakeGateway` · I1 · подволна 1.9 · developer#2 · M

**Ветка:** `task/T-315-e2e-bot` (`.worktrees/T-315`).
**Зависит от:** T-308, T-312.
**Ссылки:** ADR-010, ADR-018; component §15; design §3.1 п. 7–8, §7; US-008, US-001, US-009; SEC-06, SEC-07, SEC-10.

**Описание.** Детерминированный сценарий: `/start` → согласие → имя (в т. ч. совпадение с username) → выбор мира → `/enter` → `/attack` → доставка механики и нарратива → `/say` → `/forget confirm`. `FakeUpdateSource` подаёт обновления, `FakeSender` записывает исходящие, gateway — `FakeGateway` in-process.

**DoD** (+ DoD-common):
- [ ] Проверены: тексты (golden), клавиатуры, порядок сообщений, ack каждой доставки, стабильность `action_key` при повторной подаче того же `update_id`.
- [ ] Отдельные ветки: сообщение из группового чата → бот не вызвал gateway и не ответил (SEC-07; решение оркестратора по ревью #1 T-310: US-008, FR-131, NFR-044); `from.id` вне allowlist → один отказ на серию сообщений, gateway не вызван (SEC-06).
- [ ] Ветка `parse_mode`: доставка нарратива с `[x](http://…)` и `<a>` приходит буквально (SEC-10).
- [ ] Ветка ошибок: `429 rate_limited` и `503 bus_unavailable` от gateway → корректная подсказка игроку, команда не теряется (повтор с тем же `action_key`).
- [ ] Сценарий стабилен: 3 прогона подряд дают одинаковую запись `FakeSender`.

### T-316: Integration-тесты `consumer` на testcontainers Redpanda · I1 · подволна 1.10 · developer#2 · S

**Ветка:** `task/T-316-integration-consumer` (`.worktrees/T-316`).
**Зависит от:** T-304, T-307. **Внешнее:** `testkit.Versions()` (F-4a), kafka-адаптер `eventbus` (EPIC-001).
**Ссылки:** C-01 v1.1, ADR-010; component §15; design §7 (строка integration); NFR-013.

**Описание.** `//go:build integration` в `internal/gateway/consumer`: дедуп дублей, продвижение курсоров, DLQ (`dead_letters`), `Journal.End()` на реальном брокере; версии образов — только из `testkit.Versions()`.
**Сверка 2026-09-13 (разрешение пользователя 2026-09-13).** Прогон — только `go test -tags integration ./internal/gateway/consumer/...` на одноразовых контейнерах testcontainers, не больше одного прогона одновременно; после прогона — проверка, что контейнеров testcontainers не осталось. `make test-integration`, `make up/down` и контейнеры владельца не трогать.

**DoD** (+ DoD-common):
- [ ] Тест зелёный локально разрешённой командой (см. описание), остатков контейнеров нет; в CI-job `integration` — в составе job EPIC-001; время прогона ≤ 3 мин. *(Сверка 2026-09-13: заменено «зелёный локально и в CI-job `integration`».)*
- [ ] Дубль события не даёт второй доставки; курсор восстанавливается после переподключения; необработанное событие уходит в `dead_letters` с причиной.
- [ ] `Journal.End()` на kafka-адаптере даёт ту же семантику, что на `membus` (сверка с contract-тестом шины `shared/testkit/contract`).
- [ ] Версии образов не захардкожены (`testkit.Versions()`).

### T-317: Документация блока — README gateway и бота, runbook «ротация токена», сверка `.env.example` · I1 · подволна 1.7 · developer#2 (в паре с tech-writer) · S

**Ветка:** `task/T-317-docs-gateway-bot` (`.worktrees/T-317`).
**Зависит от:** T-303, T-310.
**Ссылки:** SEC-08, SEC-13; D-10 (`infrastructure.md`), правило 4 `compose-lint`; `Docs/ops/runbook.md` §6; component §11.1, §11.3; NFR-074; ownership §1 (`README.md`/`AGENTS.md` — tech-writer).

**Описание.** `internal/gateway/README.md` (назначение, пакеты, как поднять локально, режимы `live|replay`, переменные, `/health`), `cmd/telegram-bot/README.md` (как получить токен, allowlist, профиль `bot`, коды выхода, что делать при `409`), раздел «Токен Telegram-бота» в `Docs/ops/runbook.md` (§6, сейчас «не применимо до EPIC-004»; ротация токена бота: признаки компрометации, отзыв у BotFather, замена `MV_TELEGRAM_BOT_TOKEN` в `.env`, перезапуск по runbook — `make up PROFILES=<набор>,bot` или `docker compose -f docker-compose.yml -f docker-compose.bot.yml up -d telegram-bot`, проверка отсутствия токена в логах, чек-лист). Сверка `.env.example` с манифестами обоих процессов.
**Сверка 2026-09-13.** Отдельного каталога `Docs/runbooks/` нет: единый runbook — `Docs/ops/runbook.md`, правка раздела согласуется с tech-writer и devops. Токен передаётся сервису `telegram-bot` только через `environment:` в `docker-compose.bot.yml`, а не через `env_file` (D-10, правило 4 `compose-lint`) — прежняя формулировка «замена в `env_file`» этому противоречила. *(Заменено: «`Docs/runbooks/telegram-token-rotation.md` … замена в `env_file`, перезапуск профиля `bot`».)*

**DoD** (+ DoD-common, кроме unit — здесь применимо частично):
- [ ] Оба README содержат актуальные списки переменных, совпадающие с манифестом `shared/env.Declare` (проверяется CI-сверкой `.env.example`, NFR-074).
- [ ] Раздел runbook (`Docs/ops/runbook.md` §6) проверен «сухим прогоном» (шаги выполнимы без доступа к прод-токену), содержит явный пункт «токен нигде не логируется», ссылку на SEC-08 и правило D-10 (токен — только `environment:` сервиса `telegram-bot`).
- [ ] **(приёмка T-318, 2026-09-13; условия правдивости уведомления FR-009 У-1, У-2, У-4, У-5)** `cmd/telegram-bot/README.md` (раздел «Данные игроков») и раздел бота в `Docs/ops/runbook.md` (правка согласуется с devops) называют правила оператора, при которых текст `/start` правдив: профили `memory` и `legacy` не запускаются, пока в `MV_TELEGRAM_ALLOWED_USER_IDS` есть кто-то кроме владельца; открытых копий тома `gateway-data` нет, копия `links.db` только через `age`, копии `links.db` и `gateway.db` удаляются не позже 30 дней; архивы `make backup` не старше 30 дней (T-463); `mvctl report` по сессиям живых игроков — только после R2-M-1 (T-131/T-132); при изменении сроков retention или бэкапа `render/notice.go` и тест T-311 правятся вместе (FR-009). Ссылка на карточку [`tasks/T-318.md`](tasks/T-318.md) (У-1…У-9).
- [ ] Документы согласованы с tech-writer; правки в `CLAUDE.md`/`AGENTS.md` — **не** в этой задаче (владелец — tech-writer, EPIC-001 F-9), запрос передан в отчёте.
- [ ] Публикация портов только на `127.0.0.1` описана и помечена как проверка `compose-lint` (SEC-13).
- [ ] **(приёмка T-310, 2026-09-13; ревью #1 T-310, п. 6 бэклога)** `cmd/telegram-bot/README.md` и раздел бота в `Docs/ops/runbook.md` (правка согласуется с devops): в @BotFather `/setjoingroups` → **Disable** — бота нельзя добавить в группы (бот в группе молчит, SEC-07, но и сообщений групп не получает); при `SIGTERM` может потеряться одно обновление, взятое в обработку (T-312); отказ посторонним — не больше одного ответа на чат и 10 на весь бот за 60 с, счётчик `bot_denied_total` в `/health`.

### T-318: Текст уведомления FR-009 (`/start`) и его размещение в `render/notice.go` · I1 · подволна 1.5 (не занимает слот разработчика) · business-analyst + tech-writer, приёмка security-engineer и tech-lead#3 · S · **Статус: done**

**Ветка:** нет — текст передаётся разработчику T-311 (карточка `tasks/T-318.md`).
**Зависит от:** —. **Блокирует:** T-311 (до готовности — заглушка из US-008).
**Ссылки:** FR-009, SEC-26, BR-09; US-008, US-009; `prd.md` v0.4 (открытая формулировка, приложение «Формулировка»); NFR-045, NFR-046.

**Описание.** Финальный русский текст первого сообщения `/start` простыми словами, обязательные пункты: (1) контент создаётся ИИ; (2) игра 18+ — подтвердите возраст; (3) введённый текст (имя персонажа, `say`) обрабатывается для генерации и может уходить облачному провайдеру, если оператор его включил; (4) что именно удаляет `/forget` и что остаётся; (5) сроки хранения (30/90/180 дней) и бэкап связки ≤ 30 дней. Плюс текст подтверждающей кнопки и текст отказа. Результат кладётся разработчиком в `render/notice.go` одной константой.

**DoD:**
- [ ] Текст покрывает все 5 пунктов и проверен по чек-листу security-review («Уведомление FR-009: ИИ, 18+, обработка текста, облако, `/forget` и сроки»).
- [ ] Формулировки согласованы BA и tech-writer, отревьюены security-engineer (SEC-26).
- [ ] Текст помещён в `render/notice.go` одной константой (логика правкой текста не затрагивается); grep-тест на обязательные пункты добавлен в T-311.
- [ ] `prd.md` §«Формулировка» обновлён BA: вопрос закрыт (правку делает BA, не TEAM-3).

**Приёмка (tech-lead#3, 2026-09-13): принято, статус `done`** — карточка [`tasks/T-318.md`](tasks/T-318.md), раздел «Приёмка (tech-lead)». Ревью безопасности: 2 итерации (#1 — «вернуть», #2 — «принять» с N-1). При приёмке внесены N-1 (текст R2-N-1) и подстроки 22–24 (R2-Mi-2). Проверено программой: длина `NoticeText` — 2 341, каждая из 24 подстрок встречается один раз, `30 дней` — три раза. `prd.md` v0.5.1: строка §11 «Формулировка» закрыта (Р-6: «приложение «Формулировка»» в ссылках выше — это строка §11). Р-3 A подтверждён. Добавлены строки DoD: T-311 (подстроки, нормализация, кнопки, Р-2 A, Р-3 A, Mi-5, I-2), T-317 и T-390 (условия правдивости У-1, У-2, У-4, У-5). Строки для T-463, T-236, T-131/T-132 и бэклога EPIC-001/EPIC-005 записаны в карточке, подраздел «Передать в другие эпики»; передаёт оркестратор. R2-M-1 A (без идентификаторов игрока в `ops/metrics`) — условие выпуска бота к игрокам кроме владельца.

### T-319: Перенос `services/game-service` → `services/_archive/game-service/` · I1 (хвост, после стенда) · подволна после T-392 · developer#2 · S

**Ветка:** `task/T-319-archive-game-service` (`.worktrees/T-319`).
**Зависит от:** T-392 (стенд S9 пройден). **Ссылки:** U-1/OQ-A-17; component §13; ownership §1; design §1; `services/_archive/README.md`.

**Описание.** Перенести каталог как есть (`git mv`), добавить `ARCHIVED.md` (причина, коммит, эпик возможного возврата) и строку в таблицу `services/_archive/README.md`, убрать упоминания из `docker-compose*.yml`/`Makefile`/линтера/CI, если они есть. Код не переписывать и не удалять. **(сверка 2026-09-13)** `go.work` в проекте нет; `services/game-service` — отдельный модуль со своим `go.mod`, корневой `go build ./...` его не видит, корневой `go.mod` не меняется. *(Заменено: «исключить из `go.work`/`go.mod`/…».)*

**DoD** (+ DoD-common):
- [ ] `go build ./...` и `make ci` зелёные после переноса. *(Сверка 2026-09-13: убран `go work sync`.)*
- [ ] `ARCHIVED.md` заполнен по шаблону других архивов (EPIC-001 F-1/F-3).
- [ ] Ни одна строка кода не удалена; `git mv`-семантика (история файлов сохранена).
- [ ] Изменения в `docker-compose*.yml`/`Makefile` согласованы с tech-lead#1 и devops (владельцы по ownership) — согласование приложено к отчёту; строка `game-service` в таблице статусов `CLAUDE.md` — запрос tech-writer в отчёте. *(Сверка 2026-09-13: убран `go.work`.)*

### T-320: Уведомление об облачном провайдере в боте (US-008) · I1 (хвост) · подволна 1.11 · developer#2 · S · разблокирована сведением 3, З-3

**Ветка:** `task/T-320-bot-cloud-notice` (`.worktrees/T-320`). *(Сверка 2026-09-13: статус `planned` снят — статус ведётся в карточке.)*
**Зависит от:** T-307, T-311; EPIC-003 T-215/T-212 (схема и публикация `config.cloud_enabled`). **Внешних блокеров нет** — З-3 решено (§8).
**Ссылки:** US-008 (критерий про `config.cloud_enabled`), **`contracts.md` v0.4 C-06 v1.1, C-08 v1.2**, C-01; `api-contracts.md` §1.3, §2.3.16; `consolidation.md` §14.1 (З-3); ADR-005 п. 6, ADR-009 п. 12; SEC-21; NFR-046.

**Описание (уточнено сведением 3, внесено tech-lead#1 от имени tech-lead#3).** gateway читает `config.cloud_enabled` из `system_events` (топик уже в подписке, component §2), держит проекцию последнего значения (по умолчанию `false`, попадает в снапшот `gateway/`) и отдаёт флаг **единственным местом** — `GET /v1/worlds → worlds[].llm{cloud_enabled: boolean}` (**не** в `GET /v1/players/{id}` и **не** через `Delivery kind=system`). Провайдер, URL и ключи клиенту не передаются (SEC-21). Бот кэширует флаг с TTL **`MV_TELEGRAM_CLOUD_FLAG_TTL` (по умолчанию `60s`)**, обновляет на `/start`, `/help` и по истечении TTL, показывает уведомление **один раз за сессию диалога**; текст — без имени провайдера. Контекст `llm` публикует `config.cloud_enabled` при каждом старте `core` (C-06 v1.1), поэтому проекция детерминирована после рестарта.

**DoD** (+ DoD-common):
- [ ] Поле `worlds[].llm.cloud_enabled` есть в OpenAPI (схема из T-301) как совместимое дополнение C-08 v1.2; `openapi_test` зелёный.
- [ ] Unit gateway: `config.cloud_enabled {enabled:true}` → `true` в ответе; `{enabled:false}` → `false`; до первого события — `false`; проекция переживает рестарт (значение из снапшота `gateway/`); в ответе нет ключей, URL и имени провайдера (SEC-21).
- [ ] Unit бота: уведомление показывается **один раз за сессию диалога**; кэш обновляется на `/start`, `/help` и по TTL (`MV_TELEGRAM_CLOUD_FLAG_TTL=60s`, тест с укороченным TTL); при выключенном облаке уведомления нет.
- [ ] `MV_TELEGRAM_CLOUD_FLAG_TTL` объявлен через `shared/env.Declare` вместе с кодом чтения; `.env.example` дополнен (запрос devops).
- [ ] **(решение оркестратора по приёмке T-318, 2026-09-13; У-9, I-3)** Флаг облака обновляется перед первой игровой командой сессии диалога, а не только на `/start`, `/help` и по TTL. Пока флаг не получен, игровая команда не уходит в облако без показанного уведомления. Unit: облако включено в пределах TTL кэша → первая игровая команда новой сессии диалога получает уведомление раньше ответа на неё.

### T-467: Стабилизация флака `shared/testkit/gateway` `TestTheSkirmishStopsSwingingWhenTheFightEnds` под нагрузкой полного `./...` · I1 (вне подволн) · исполнитель — по назначению оркестратора · S

**Ветка:** `task/T-467-skirmish-flake` (`.worktrees/T-467`). Можно вести параллельно любой задаче, которая не правит `shared/testkit/gateway/**`.
**Зависит от:** —.
**Ссылки:** `shared/testkit/gateway/combat_test.go`; журнал 2026-09-13 — падения при фиксации T-303 и при приёмке T-208 (`make test`: «1 proposals were refused»); DoD-common §0.

**Описание.** Под нагрузкой полного `go test ./...` (`make test`) тест падает на последней проверке: в `system_events` есть отказ предложения сценария (`state.TypeRejected`). В одиночном прогоне (`-count=3`) тест зелёный. Нужно найти причину — какое предложение и с каким кодом отклонено, почему только под нагрузкой — и устранить её в харнессе или в тесте. Ожидания в настенном времени, повторы и `t.Skip` не допускаются.

**DoD** (+ DoD-common):
- [ ] Причина записана в карточке: какое предложение отклонено, код отказа, порядок событий, из-за которого это происходит только под нагрузкой.
- [ ] Исправление без увеличения таймаутов в настенном времени, без `time.Sleep`, без повторов теста и без `t.Skip`.
- [ ] Если причина — гонка в коде харнесса, есть детерминированный тест (управляемые часы или хуки), красный до исправления.
- [ ] `go test -count=30 -cpu=1,4 -run TestTheSkirmish ./shared/testkit/gateway/` зелёный, в том числе при параллельном `go test -short -count=1 ./...` в другой оболочке; `make test` — три прогона подряд без падения этого теста. Итоги — в карточке.

---

## 2. Инкремент I2 «группа и раунды» (developer#1 + developer#2)

Старт — по приёмке I1 tech-lead#3 (`epics.md` §2). Контрольное слияние ветки эпика в `develop` — после зелёного интеграционного прогона I1 **и** после слияния EPIC-002 I2 и EPIC-003 I2 (порядок 002 → 003 → **004** → 005). *(Сверка 2026-09-13: «слияние» — контрольное слияние в `develop`, `integration/mvp-1` нет.)*

### T-350: Координатор раундов — ядро (`coordinator`, `state`, `store`) · I2 · подволна 2.1 · developer#1 · M

**Ветка:** `task/T-350-rounds-core` (`.worktrees/T-350`).
**Зависит от:** T-305, T-307, T-309. **Внешнее:** `encounter.started.round{}` от EPIC-003 I2 (C-05 v1.1) — до готовности берутся env-значения (`MV_GATEWAY_ROUND_*` объявляются вместе с кодом).
**Ссылки:** ADR-020 (+ доп. п. 1), C-04, C-05 v1.1; component §9, §4.2 (`rounds`); design §3.2 п. 2; US-007; FR-025, BR-13; NFR-005, NFR-012.

**Описание.** `rounds/coordinator.go|state.go|store.go`: открытие раунда 1 по `encounter.started` для scope `group`, раунда N+1 — первым раундовым действием (`attack|flee|defend|group.leave`); `say` **не** открывает раунд и не входит в `acted[]`; `Accept` (`ErrAlreadyActed` → `409 already_acted`, `ErrNotRoundAction`); закрытие `all_acted|timeout|explicit`; порядок публикации `player.defended cause=round_timeout` для `expected − acted` → затем `round.closed`; мьютекс на scope, состояние `closing`, идемпотентность по `(scope, seq)`; параметры из `encounter.started.round{}`, иначе `MV_GATEWAY_ROUND_TIMEOUT`/`MV_GATEWAY_ROUND_IDLE_AFTER_MISSED`.

**DoD** (+ DoD-common):
- [ ] Unit с `FakeClock` — полный набор component §15: `all_acted`, `timeout`, `explicit`, второе действие участника → `already_acted`, `say` не влияет на раунд, порядок `player.defended` → `round.closed`, `closed_at = Clock.Now()`.
- [ ] Unit: `round.opened` публикуется **до** `202` на открывающее действие (гарантия C-04).
- [ ] Unit: `round.closed` содержит `acted[]` в порядке времени приёма, `auto_defended[]`, `idle[]`, `close_reason`; является корневым событием (`meta.cid = id`).
- [ ] Unit гонки «действие vs таймер»: одновременный `Accept` и срабатывание таймера не дают двух `round.closed` и не теряют действие.
- [ ] В `solo` `round.*` не публикуются (C-04) — тест.
- [ ] Публикуемые события валидны по схемам из T-301.
- [ ] **(T-416, 2026-09-11; C-04 v1.3, ADR-026)** Конец встречи при открытом раунде — первое из «факт сущности встречи `state=resolved`» и `encounter.ended` (read-model T-304) — координатор обрабатывает **тем же путём**, что смертельный удар первым действием раунда. Путь один и для конца без действия группы (существо убито вне встречи, C-05 v1.4 п. 6). Новых значений `close_reason` не вводится. Unit: конец по факту и конец по `encounter.ended` при открытом раунде дают одинаковый итог раунда — один раз, без второго `round.closed`; то же для конца без единого действия группы в раунде.

### T-351: Раунды — `Restore`, `replay`, `idle`/`missed_rounds`, `OnParticipantsChanged` · I2 · подволна 2.2 · developer#1 · M

**Ветка:** `task/T-351-rounds-restore` (`.worktrees/T-351`).
**Зависит от:** T-350, T-352 (для `group_participation` и состава).
**Ссылки:** ADR-020, C-02, C-04; component §9, §4.2 (`group_participation`), §11.2; design §3.2 п. 3; US-007, US-011; BR-13; NFR-014.

**Описание.** `Restore` при старте (открытые `open` — таймер на `deadline_at`, просроченный — немедленное закрытие `timeout`; `closing` без `closed_event_id` — довести идемпотентно); ветка `replay` (таймеров нет, `round.opened/closed` читаются из журнала, `Accept` — только учёт); `group_participation` — счётчик `missed_rounds`, переход `active → idle` после `idle_after_missed`, обратно при первом действии; `OnParticipantsChanged` (смерть, бегство, выход, `/forget` → пересчёт `expected`); предложение `entity.update.proposed members[].participation` (`cause=group`).

**DoD** (+ DoD-common):
- [ ] Unit `Restore`: три случая (открытый с будущим дедлайном, открытый с прошедшим, `closing` без события) — поведение по ADR-020.
- [ ] Unit `replay`: таймеры не создаются, `round.*` не публикуются, состояние `rounds` восстанавливается из журнала (NFR-014).
- [ ] Unit `idle`: 2 пропуска → `participation=idle` + предложение по группе; участник исключён из `expected` и попал в `round.closed.idle[]`; первое действие → `active`, `missed_rounds=0`.
- [ ] Unit `OnParticipantsChanged`: смерть/выход единственного `active` участника закрывает раунд корректно (US-009, критерий про `/forget` в открытом раунде).
- [ ] `gateway` — единственный, кто предлагает `participation` (component §16 п. 3); тест на отсутствие расхождения `group_participation` и предложения.
- [ ] **(T-416, 2026-09-11; C-04 v1.3, ADR-026)** `Restore` и `OnParticipantsChanged` ведут конец встречи при открытом раунде тем же путём, что T-350 (как смертельный удар первым действием раунда), без нового `close_reason`. Unit `Restore`: открытый раунд встречи, конец которой (факт `state=resolved` или `encounter.ended`) пришёл, пока шлюз лежал, закрывается этим путём один раз.
- [ ] **(ревью #1 T-304, Mi-2, 2026-09-13)** Эффект координатора по `readmodel.Result.EncounterEnded` идемпотентен по встрече в `rounds`: второго закрытия раунда и второго `round.closed` нет, в том числе когда переход сообщён повторно после рестарта (захват перехода в `readmodel` живёт в памяти). Unit: рестарт между фактом `state=resolved` и `encounter.ended` → раунд закрыт один раз.

### T-352: `groups` — create/join/leave, лидер, `group.entered_region`, `GET /v1/groups/{id}`, предусловия группы · I2 · подволна 2.1 · developer#2 · M

**Ветка:** `task/T-352-groups` (`.worktrees/T-352`).
**Зависит от:** T-305, T-306. **Внешнее:** atomic-группы `entity.*.proposed` из EPIC-002 I2 (C-02 v1.1).
**Сверка 2026-09-13.** В схеме `group.entered_region.v1.json` поля `cause` нет, а значение `group_move` стоит в enum `cause` у `group.created/joined/left/disbanded`. Где живёт `cause: group_move` для перемещения группы — решение system-architect (вопрос открыт); до решения пункт DoD US-006 про `{cause: group_move, by}` проверяется по решению, записанному в карточке.
**Ссылки:** **`contracts.md` v0.4 C-02 v1.2, C-04 v1.1, C-08 v1.2**; `consolidation.md` §14.1 (З-4); component §7.4, §5.2, §5.4 (правила группы); design §3.2 п. 1; US-006; FR-003, FR-005, FR-025, BR-06, **BR-13 v0.4**; NFR-020.

**Дополнение (сведение 3, З-4; внесено tech-lead#1 от имени tech-lead#3).** Группа **без лидера** — легальное состояние: `Group.leader_id: ref | null`. Если после выхода/смерти/отвязки лидера участников со `status=alive` не осталось, gateway предлагает `set leader_id = null` (в том же atomic-пакете) и публикует `group.leader_changed {leader: null, cause}`; группа живёт без лидера до `group.disbanded` (при выходе последнего участника). Перемещение группы (`enter`/`leave`) при `leader_id = null` → **`409 no_leader`**, проверяется **раньше** `not_leader`; личный `group.leave` участника при этом допустим. `GroupView.leader_id` — `string | null`. `dead`/`abandoned` участники остаются в `members[]` (история), но исключаются из `expected[]`/`acted[]` и `participation=active`.

**Описание.** `groups/service.go`: `Create` (`entity.create.proposed group` → `AwaitFact` → `entity.update.proposed player.scope` → `group.created` → `session.Open(group)` + `session.End(solo, leave)`), `Join` (одно atomic-предложение), `Leave` (вне встречи — атомарное предложение; во встрече — как `flee`), передача лидерства старейшему `alive` по `joined_at` (`cause=leave`; `cause=death` — по BR-13), `group.disbanded`, `group.entered_region/left_region` при `enter/leave` лидера (за участников `player.entered_region` **не** публикуется), `202 pending` при таймауте `MV_GATEWAY_FACT_WAIT`; `api/handlers_groups.go` (`GET /v1/groups/{id}`); включение правил группы в `actions.Validate` (`not_leader`, `already_in_group`, `not_in_group`, `group_full`, `group_in_encounter`, `in_encounter`).

**DoD** (+ DoD-common):
- [ ] Unit US-006: три участника → scope `group`, лидер — создатель, состав и лидер в подтверждении; `enter` от не-лидера → `not_leader` с подсказкой, позиция группы не меняется; `enter` лидера → одно `group.entered_region {cause: group_move, by}`.
- [ ] Unit выхода: не-лидер → `solo` в той же позиции, сессия остальных не прерывается; лидер → `group.leader_changed cause=leave` старейшему по `joined_at`; последний → `group.disbanded` + `session.End`.
- [ ] Unit BR-13 при смерти лидера: `group.leader_changed cause=death`; если живых нет — `set leader_id = null` в том же atomic-пакете и `group.leader_changed {leader: null, cause: death}`; группа без лидера до `disbanded` (З-4 решено).
- [ ] **(сведение 3, З-4)** Unit `no_leader`: `enter`/`leave` группы при `leader_id = null` → `409 no_leader` (а не `not_leader`), позиция группы не меняется; личный `group.leave` участника при `leader_id = null` проходит; `GroupView.leader_id` сериализуется как `null`.
- [ ] Unit `202 pending`: факт не пришёл за 2 с → `{status:"pending", group_id, correlation_id}`; последующий `GET /v1/groups/{id}` отдаёт состав (C-08 v1.1).
- [ ] Unit валидации: `group_full` (> 6), `already_in_group`, `group.join` во встрече → `group_in_encounter` (FR-005).
- [ ] Предложения группы — `atomic=true` (NFR-020); проверка, что gateway не пишет сущности напрямую (только `*.proposed`, C-02).
- [ ] **OpenAPI обновлён**: `getGroup`, `202 pending` у `postAction`, **`409 no_leader` и `GroupView.leader_id: nullable`** (схема из T-301); `openapi_test` зелёный.

### T-353: Групповая доставка outbox и команды `/group` в боте · I2 · подволна 2.2 · developer#2 · M

**Ветка:** `task/T-353-group-delivery` (`.worktrees/T-353`).
**Зависит от:** T-307, T-352.
**Ссылки:** ADR-006, C-05 v1.1, C-08; component §8.1, §10.3; design §3.2 п. 5; US-006, US-008; FR-003, FR-013.

**Описание.** Адресаты по scope (component §8.1): `combat.decided` — всем `alive` участникам; `narrative.output` — по `recipients[]`; `group.*`/`round.opened` — участникам; `player.said` в scope `group` — всем, кроме автора; `entity.updated cause=group_move` — всем участникам. Бот: `/group create|join <id>|leave`, показ `group_id` для передачи друзьям, боевая клавиатура для группы, тексты состава и раунда.

**DoD** (+ DoD-common):
- [ ] Unit адресности: одна `narrative.output` с тремя `recipients` → три доставки с одним `narrative_event_id`; автор `say` своей реплики не получает; `dead` участник не получает `combat.decided`.
- [ ] Unit порядка: на каждого игрока порядок `seq` сохраняется независимо от других (одна в лизинге на игрока).
- [ ] Golden-тексты состава группы, `group.joined/left/leader_changed/disbanded`, «Раунд N. Ждём действий …».
- [ ] Unit бота: `/group join <id>` без id → подсказка; `202 pending` → бот опрашивает `GET /v1/groups/{id}` и показывает состав.
- [ ] Клавиатура боя в группе не содержит целей, недоступных участнику.

### T-354: `POST /v1/scopes/{scope_id}/rounds/close` (только `ci`) и `Harness.CloseRound` · I2 · подволна 2.3 · developer#1 · S

**Ветка:** `task/T-354-rounds-close` (`.worktrees/T-354`).
**Зависит от:** T-350, T-308.
**Ссылки:** C-08 v1.1 (`409 no_open_round`), SEC-12; component §5.2, §9; design §3.2 п. 4; US-007; ADR-020.

**Описание.** `api/handlers_admin.go` — служебный маршрут закрытия раунда, доступный только клиентам с `actor_kind=ci`; `CloseRound(scopeID)` HTTP-обвязки T-308 (замена её заглушки; API `Harness` v0 не меняется — сверка 2026-09-13).

**DoD** (+ DoD-common):
- [ ] Unit: `200 {round{seq, close_reason:"explicit"}}`; повтор при закрытом раунде → `409 no_open_round`; клиент без `ci` → `403 actor_kind_forbidden` (SEC-12).
- [ ] `Harness.CloseRound` работает в e2e и в `--mode=replay`.
- [ ] **OpenAPI обновлён**: `closeRound` с `x-actor-kind: [ci]`; `openapi_test` зелёный.

### T-355: `/forget`-каскад для группы (`ForgetHooks`) · I2 · подволна 2.3 · developer#2 · M

**Ветка:** `task/T-355-forget-group` (`.worktrees/T-355`).
**Зависит от:** T-352, T-351, T-303.
**Ссылки:** FR-023, FR-061, **BR-13 v0.4**, SEC-26; **`contracts.md` v0.4 C-02 v1.2, C-04 v1.1 (каскад `/forget`), C-08 v1.2, C-10 v1.1**; `consolidation.md` §14.1 (З-1, З-2, З-4); component §7.5, §6 (`ForgetHooks`); design §3.2 п. 6; US-009; NFR-042.

**Описание.** Расширение `ForgetHooks.OnForget` — **строго в порядке C-04 v1.1** (сведение 3, З-2; внесено tech-lead#1 от имени tech-lead#3):
1. `outbox.DropForPlayer` (`pending → dropped`);
2. если персонаж `alive` — **один** `entity.update.proposed {atomic: true, cause: forget}`: игрок `set status=abandoned` (+`expected_version`), и, если он лидер группы, в том же пакете `set leader_id = <старейший alive | null>`; персонаж `dead` — шаг пропускается (терминальный статус, FR-023), `creating` — предложение публикуется при получении `entity.created`;
3. если игрок в группе — `group.left {cause: forget}` и, при смене лидера, `group.leader_changed {cause: forget, leader: …|null}`; последний участник → `group.disbanded`;
4. `session.End(forget)` → `analytics.session.ended {end_reason: forget}`;
5. физическое удаление связки (`links.db`).
Сопутствующее: `session.UpdateParticipants`, пересчёт `expected` открытого раунда (`OnParticipantsChanged`). Во встрече gateway `flee` **не** исполняет и ничего дополнительно не публикует — агент встречи видит `entity.updated status=abandoned` и завершает встречу `players_out`, если игроков `alive` не осталось.

**DoD** (+ DoD-common):
- [ ] Unit US-009: `/forget` лидера группы из трёх → `group.left`, `group.leader_changed`, состав из двух, сессия остальных продолжается.
- [ ] Unit: `/forget` единственного `active` участника с открытым раундом → раунд закрывается, игрок не считается ни `acted`, ни `auto_defended`.
- [ ] Unit: `/forget` последнего участника → `group.disbanded` + `session.End`.
- [ ] **(сведение 3, З-2)** Unit порядка каскада: публикации идут ровно в последовательности (1)…(5) — предложение `abandoned` **до** `group.left`, `group.left` **до** `session.ended`, удаление связки — последним; `group.left.cause = forget`, `group.leader_changed.cause = forget`, `end_reason = forget`.
- [ ] **(сведение 3, З-2)** Unit ветки статусов: `alive` → предложение публикуется; `dead` → шаг (2) пропускается, остальные выполняются; повторный `/forget` по уже `abandoned` игроку не публикует второго предложения.
- [ ] **(сведение 3, З-4)** Unit: `/forget` лидера, когда других `alive` нет → в том же atomic-пакете `set leader_id = null` и `group.leader_changed {leader: null, cause: forget}`; группа не распускается, пока есть участники.
- [ ] Физическое удаление связки (checkpoint+vacuum) выполняется **после** хуков и не откатывается при их ошибке — ошибка хука логируется, удаление всё равно происходит (приоритет NFR-042); поведение зафиксировано тестом.
- [ ] **(приёмка T-303, 2026-09-13 — расхождение, до решения T-456 п. 5)** Пункт выше расходится с реализованным в T-303 порядком (`internal/gateway/links/forget.go`). В T-303 хуки выполняются до `DELETE`, упавший хук оставляет связку для повтора `/forget`, поэтому хуки обязаны быть идемпотентными. Шаг (2) публикует предложение с `proposal_id = forget:{player_id}`; `entity.update.rejected` с `dead_entity` на своём предложении забвения хук считает успехом («уже забыт»). Итоговую формулировку даёт system-architect#1 в T-456 п. 5. До неё действует порядок T-303, пункт выше не реализуется.

### T-356: Прокси `/v1/admin/*` к `MV_CORE_URL`, `actor_kind` `ci/sim`, раздел `admin` в OpenAPI · I2 · подволна 2.4 · developer#2 · M

**Ветка:** `task/T-356-admin-proxy` (`.worktrees/T-356`).
**Зависит от:** T-303. **Внешнее:** admin-порт `core` (C-06, EPIC-003 I1b) — для unit достаточно `httptest.Server`.
**Ссылки:** C-06, C-08 v1.1, SEC-12, SEC-13; component §5.1, §5.2, §11.3; design §3.2 п. 7; `journal.md` (совладение раздела `admin` с EPIC-003).

**Описание.** `httputil.ReverseProxy` на `POST /v1/admin/agents/{id}/tick`, `GET /v1/admin/agents`, `GET /v1/admin/llm/usage` (таймаут 30 с); `GET /v1/admin/sessions` — из `session.Active()` (без внешних ID); `DELETE /v1/admin/links/{player_id}`; допуск `ci`/`operator`; право клиента на `X-Actor-Kind: ci|sim` — по `MV_GATEWAY_ACTOR_KIND_CLIENTS` *(T-416: заменено `MV_GATEWAY_CLIENTS`, C-08 v1.3)*; раздел `admin` в `api/gateway.openapi.yaml` согласован с architect#2 (совладение).

**DoD** (+ DoD-common):
- [ ] Unit на `httptest.Server`: заголовки клиента пробрасываются, тело и статус возвращаются как есть, таймаут 30 с; `core` недоступен → `/health.core_admin=unavailable`, `degraded`.
- [ ] Unit SEC-12: клиент без `ci`/`operator` → `403`; `GET /v1/admin/sessions` не содержит внешних ID.
- [ ] Ревью prod-`.env`: `ci-harness` отсутствует во всех трёх списках клиентов — `MV_GATEWAY_CLIENT_IDS`, `MV_GATEWAY_ACTOR_KIND_CLIENTS`, `MV_CORE_ADMIN_CLIENTS` (SEC-12; умолчания только в манифесте, T-411) — пункт вынесен в чек-лист T-392 и зафиксирован в README. *(T-416: заменено `MV_GATEWAY_CLIENTS`.)*
- [ ] Раздел `admin` в OpenAPI не конфликтует с описанием EPIC-003 (согласование в отчёте); `openapi_test` зелёный.
- [ ] Может быть выполнена в конце I1, если EPIC-003 I1b выставил admin-порт (design §3.2 п. 7) — решение принимает tech-lead#3 на подволне 1.11.
- [ ] **(приёмка T-303, 2026-09-13)** `DELETE /v1/admin/links/{player_id}` (`adminForgetLink`) монтируется на готовый `links.SQLite.ForgetByPlayer` с тем же разбором ошибок, что `forgetLink`: `ErrCompactionPending` → `503 forget_incomplete` с `Retry-After`. Операция вычеркнута из `notYetMounted` в `openapi_test.go`. Проверка `x-actor-kind: [ci]` служебных маршрутов — в `api.Chain`. Повтор каскада при смене персонажа во время хуков закреплён на уровне стора (`TestForgetByPlayerFollowsTheLinkToACharacterBoundMeanwhile`, приёмка T-303); в HTTP-тестах его не дублировать.

### T-357: e2e группы — `group-3x30` и `forget` в группе · I2 · подволна 2.4 · developer#1 · M

**Ветка:** `task/T-357-e2e-group` (`.worktrees/T-357`).
**Зависит от:** T-350…T-355.
**Ссылки:** ADR-010, ADR-020; component §15; design §3.2, §7; US-006, US-007, US-009; UC-014…017, 020; NFR-005.

**Описание.** e2e тремя клиентами `ci-harness` (три `external_id` платформы `ci`): 30 раундов группы из трёх (≥ 60 действий) с явным `rounds/close`, ветки `timeout` и `idle`, смерть участника, выход участника; отдельный сценарий `forget` в группе.

**DoD** (+ DoD-common):
- [ ] `group-3x30` зелёный и детерминированный; 30 раундов, у каждого — `round.opened` и `round.closed` с корректными `acted/auto_defended/idle`.
- [ ] Нарратив раунда — один на scope, доставлен всем троим (FR-013, US-006).
- [ ] Ветка `timeout`: молчащий участник получает `player.defended cause=round_timeout` до `round.closed`; после 2 пропусков — `idle`.
- [ ] `forget` в группе: критерии US-009 (лидерство, состав, закрытие раунда) выполнены.
- [ ] Прогон с `--chaos=duplicate` зелёный.
- [ ] Фикстура прогона пригодна для `mvctl report` (EPIC-005) — отмечено в отчёте. *(T-416: было `mvctl session-report`.)*

---

## 3. Стендовые задачи (`stand`) — человек + tester#3, слот разработчика не занимают

Стенд — машина пользователя, живой Telegram, вне CI. Результат каждой задачи — заполненный чек-лист и список замечаний; замечания превращаются в задачи EPIC-002/003/004 по карте владения.

### T-390: Подготовка стенда — токен бота, allowlist, том данных · I1 · до подволны 1.9 · пользователь + tester#3 · S

**Ссылки:** SEC-06, SEC-08, SEC-13; U-7; FR-130; component §11.1, §11.3.

- [ ] Токен бота получен у BotFather и записан в `.env` оператора (`MV_TELEGRAM_BOT_TOKEN`); сервису `telegram-bot` он передаётся **только** через `environment:` в `docker-compose.bot.yml` (D-10, правило 4 `compose-lint`); в git не попадает (`make secrets-scan`). *(Сверка 2026-09-13: заменено «помещён только в `env_file` сервиса `telegram-bot`» — противоречило D-10.)*
- [ ] `MV_TELEGRAM_ALLOWED_USER_IDS` заполнен владельцем: собственный Telegram user id + id известных тестеров; посторонних нет.
- [ ] Том `MV_GATEWAY_DATA_DIR` создан именованным, права `0700`; бэкап `links.db` настроен devops (шифрование, срок ≤ 30 дней).
- [ ] **(приёмка T-318, 2026-09-13; У-2)** Пока в `MV_TELEGRAM_ALLOWED_USER_IDS` есть кто-то кроме владельца, профили `memory` и `legacy` не запущены: в `docker compose ps` нет `qdrant`, `neo4j`, `chromadb`, `semantic-memory`.
- [ ] **(приёмка T-318; У-1)** T-463 (EPIC-001, devops) выполнена до добавления в allowlist кого-либо кроме владельца: в каталоге бэкапа нет архивов `minio-*.tgz`/`redpanda-*.tgz` старше 30 дней; удаление идёт по возрасту, в том числе без новых бэкапов.
- [ ] **(приёмка T-318; У-5)** Копии `links.db` и `gateway.db` на стенде не старше 30 дней; копия `links.db` только зашифрованная (`age`), открытых копий `*.db` вне тома нет; том `gateway-data` открытым tar не архивируется.
- [ ] **(приёмка T-318; У-4, R2-M-1 A)** До выпуска бота к игрокам кроме владельца выполнен R2-M-1 (T-131/T-132, EPIC-005): в `ops/metrics/sessions.csv`, `ops/metrics/sessions/*.json` и `incidents.csv` нет `session.id`, `scope.id` и `player_id`. До этого `mvctl report` по сессиям живых игроков не запускается.
- [ ] Все порты публикуются только на `127.0.0.1` (`compose-lint` зелёный, SEC-13).
- [ ] Бот не публикуется в каталогах Telegram (риск design §8).
- [ ] **(приёмка T-310, 2026-09-13)** В @BotFather для бота стенда выполнено `/setjoingroups` → Disable (по runbook T-317).

### T-391: Живой прогон I1-α «соло на шаблонах через Telegram» · I1-α · после подволны 1.9 · пользователь + tester#3 · M

**Ссылки:** `epics.md` §2 (точка I1-α; тега нет — контрольное слияние в `develop`); US-001, US-008; design §3.1.

- [ ] Сквозной путь через живой Telegram: `/start` → уведомление → согласие → имя → `/enter dark-forest-01` → встреча → `/attack` → механика и нарратив **разными** сообщениями (`generated_by=template`) → `/look`, `/say`, `/rest` → `/status`.
- [ ] Ошибочные пути проверены вживую: чужой аккаунт → отказ; сообщение в групповом чате → отказ; неизвестная команда → список команд.
- [ ] Субъективная оценка текстов и клавиатур зафиксирована; замечания оформлены задачами (EPIC-002 — механика, EPIC-004 — тексты/UX бота).
- [ ] Логи обоих процессов просмотрены: внешних ID и фрагментов токена нет (беглая проверка, полная — T-392).
- [ ] Отчёт передан оркестратору для контрольного слияния I1-α в `develop` (тегов нет — решение пользователя 2026-09-13). *(Заменено: «tech-lead#1 для тега `mvp-1/i1-alpha`».)*

### T-392: Чек-лист I1 на стенде — S9, замер `ack_latency_p95_ms`, учения `/forget`, privacy-скан · I1 · после подволны 1.11 · пользователь + tester#3 + security-engineer · M

**Ссылки:** NFR-003, NFR-041/042; SEC-03…SEC-13; threat-model «чек-лист security-review»; US-009, US-011; `epics.md` (готовность I1).

- [ ] S9 «сквозной соло-ход через бота» пройден на интеграции I1 (с роем EPIC-003), результат зафиксирован.
- [ ] Замер `ack_latency_p95_ms` от сообщения боту до ответа «принято» — значение записано; NFR-003 (≤ 300 мс p95) выполнен либо зафиксировано отклонение с причиной.
- [ ] Учения `/forget`: реальная связка удаляется; `strings` по `links.db` и WAL = 0; повторный `/start` даёт новый `player_id`; недоставленные сообщения не приходят.
- [ ] Privacy-скан на стенде при `MV_LOG_LEVEL=debug`: 0 внешних ID в логах обоих процессов, топиках, MinIO, `gateway.db`, `testdata/`; 0 фрагментов токена.
- [ ] Ревью prod-`.env`: `ci-harness` отсутствует во всех трёх списках клиентов — `MV_GATEWAY_CLIENT_IDS`, `MV_GATEWAY_ACTOR_KIND_CLIENTS`, `MV_CORE_ADMIN_CLIENTS` (T-411); порты только на `127.0.0.1`. *(T-416: заменено `MV_GATEWAY_CLIENTS`.)*
- [ ] S3 (восстановление после рестарта) проверен на стенде: снапшот gateway восстанавливается, игрок продолжает с того же места (US-011).
- [ ] Чек-лист security-review по пунктам SEC-03, 06, 07, 08, 10, 11, 12, 26 отмечен целиком; незакрытые пункты — задачами.

### T-393: Живой прогон I2 — группа из трёх Telegram-аккаунтов · I2 · после подволны 2.4 · пользователь + tester#3 · M

**Ссылки:** US-006, US-007; `epics.md` (готовность I2, S2); BR-13, FR-025.

- [ ] Три живых аккаунта из allowlist (владелец + два тестера) создают группу, входят в регион, проходят встречу с раундами.
- [ ] Проверены: подтверждение состава и лидера, `enter` от не-лидера → отказ, один нарратив раунда всем троим, таймаут раунда, `idle` после 2 пропусков, передача лидерства при выходе лидера.
- [ ] `/forget` одного участника в группе отработал по FR-061.
- [ ] Замечания оформлены задачами; отчёт передан оркестратору для контрольного слияния I2 в `develop` (тегов нет). *(Заменено: «tech-lead#1 для тега `mvp-1/i2`».)*

---

## 4. Волны и параллельность

Слоты TEAM-3 по `teams.md` §4: волна 1 — developer#1, с подволны 1.4 добавляется developer#2 (1 → 2); волна 2 — developer#1 + developer#2. **(сверка 2026-09-13)** По плану перезапуска трёх команд developer#2 стартует сразу: T-302 идёт параллельно T-301; в подволне 1.3 его слот свободен до готовности контракта клиента (T-303) — чем его занять, решает tech-lead#3 по ходу волны. Стендовые задачи и T-318 слот разработчика не занимают. Ревью — `code-reviewer#3` отдельным запуском после каждой подволны, приёмка — tech-lead#3.

| Подволна | Задачи | Параллельность | Выход (что появляется) |
|---|---|---|---|
| 1.1 + 1.2 | T-301 ∥ T-302 | 2 dev (#1 — OpenAPI/клиент, #2 — store) | Скелет OpenAPI, DTO, коды, клиент → **поставка в develop** для TEAM-1/TEAM-2 (схемы уже в дереве); обе БД, миграции, физическое удаление проверено (SEC-03/04/05). *(Сверка 2026-09-13: было две последовательные подволны на developer#1.)* |
| 1.3 | T-303 | 1 dev (#1) | `links` + HTTP-каркас + `/health`; **контракт клиента виден → подключается developer#2** |
| 1.4 | T-304 ∥ T-310 | 2 dev (#1 gateway, #2 бот) | Проекция State и consumer; ядро бота с SEC-06/07/08/10/11 |
| 1.5 | T-305 ∥ T-311 (+ T-318 вне слотов) | 2 dev | Соло-ход публикуется; онбординг и тексты бота |
| 1.6 | T-306 ∥ T-312 | 2 dev | Персонаж, сессии, ходы, аналитика C-10; бот доставляет и подтверждает |
| 1.7 | T-307 ∥ T-317 | 2 dev | Outbox и long-poll — сквозной путь механики и нарратива; документация |
| 1.8 | T-308 | 1 dev (#1); слот #2 свободен → резерв TEAM-2 | `FakeGateway` + HTTP-обвязка рядом с `Harness` v0 → **поставка в develop** |
| 1.9 | T-313 ∥ T-315 | 2 dev | e2e соло и e2e бота → **готовность к I1-α** (стенд T-391) |
| 1.10 | T-309 ∥ T-316 | 2 dev | Снапшот, `replay`, полный `/health`; integration на testcontainers |
| 1.11 | T-314 ∥ T-320 (**З-3 решено — идёт в подволне**) / T-356 (если admin-порт готов) | 2 dev | e2e `forget`/`privacy-scan`/`recovery` → **готовность I1** |
| стенд I1 | T-390 (до 1.9), T-391 (после 1.9), T-392 (после 1.11), затем T-319 | человек + tester#3 | Контрольные слияния I1-α и I1 в `develop` (без тегов); архивация `game-service` |
| 2.1 | T-350 ∥ T-352 | 2 dev | Ядро раундов; группа create/join/leave |
| 2.2 | T-351 ∥ T-353 | 2 dev | Restore/replay/idle; групповая доставка и `/group` в боте |
| 2.3 | T-354 ∥ T-355 | 2 dev | `rounds/close` для CI; `/forget`-каскад группы |
| 2.4 | T-357 ∥ T-356 | 2 dev | e2e `group-3x30` и `forget` в группе; admin-прокси → **готовность I2** |
| стенд I2 | T-393 | человек + tester#3 | Контрольное слияние I2 в `develop` (без тега) |

**Независимые пары (можно параллелить безопасно):** (T-301, T-302) — общий файл только `go.mod`/`go.sum`, меняет его T-302; (T-304, T-310), (T-305, T-311), (T-306, T-312), (T-307, T-317), (T-313, T-315), (T-309, T-316), (T-314, T-320), (T-350, T-352), (T-351, T-353), (T-354, T-355), (T-356, T-357). Пересечений по файлам внутри пары нет: developer#1 работает в `internal/gateway/**`, developer#2 — в `cmd/telegram-bot/**` (в I2 — в `internal/gateway/{groups,outbox}` при неактивном developer#1 в этих пакетах; конфликт `outbox` в 2.2 снимается тем, что T-351 не трогает `outbox/`).

---

## 5. Критерии готовности как чек-листы задач

### I1-α «соло на шаблонах через бота» (контрольное слияние в `develop`; тега нет — решение пользователя 2026-09-13)

- [ ] T-301, T-302, T-303, T-304, T-305, T-306, T-307 приняты (пункты 1–5 состава I1, design §3.1).
- [ ] T-310, T-311, T-312 приняты (пункт 7 — бот).
- [ ] T-308 принят и поставлен в `develop` контрольным слиянием (нужен для e2e и для TEAM-1/TEAM-2).
- [ ] T-313 (`solo-30`) зелёный на `FakeNarrator` (`generated_by=template`) и `FakeState`.
- [ ] T-315 (e2e бота) зелёный.
- [ ] T-318 — финальный текст FR-009 в `render/notice.go` (иначе — явная отметка о заглушке).
- [ ] T-390 выполнен, T-391 пройден человеком на живом Telegram; замечания оформлены задачами.
- [ ] Ветка эпика слита в `develop` контрольным слиянием после EPIC-002 I1 (порядок 002 → **004**, `teams.md` §3.3). *(Сверка 2026-09-13: заменено «слита в `integration/mvp-1`».)*

### I1 (контрольное слияние в `develop`, без тега)

- [ ] Все пункты I1-α выполнены.
- [ ] T-309 (снапшот, `replay`, `/health`), T-314 (`forget`, `privacy-scan`, `recovery`), T-316 (integration consumer) приняты.
- [ ] T-317 (документация и runbook) принят.
- [ ] Unit-блок безопасности зелёный целиком: SEC-03 (T-302), SEC-06/07 (T-310), SEC-08 (T-310), SEC-10 (T-310), SEC-11 (T-303, T-305, T-307, T-310), SEC-12 (T-303, T-307), SEC-26 (T-303, T-318).
- [ ] Покрытие ≥ 60 % по `internal/gateway/{actions,rounds,outbox,links,turns,session}` (component §15).
- [ ] `openapi_test` зелёный; `api/gateway.openapi.yaml` покрывает все реализованные маршруты.
- [ ] T-392 (S9, замер NFR-003, учения `/forget`, privacy-скан, ревью prod-`.env`) пройден; S3 подтверждён.
- [ ] T-319 (архивация `game-service`) выполнен.
- [ ] T-320 закрыт (З-3 решено сведением 3; перенос допустим только по решению tech-lead#1 с отметкой недопокрытия критерия US-008).

### I2 (контрольное слияние в `develop`, без тега)

- [ ] T-350…T-355 приняты; T-356, T-357 приняты.
- [ ] Unit `rounds` — полный набор component §15 (all_acted, timeout, explicit, idle, смерть, restore, replay).
- [ ] e2e `group-3x30` и `forget` в группе зелёные (S2 на интеграции — с харнессом из трёх клиентов).
- [ ] Контрольное слияние в `develop` выполнено **после** EPIC-002 I2 и EPIC-003 I2 (порядок 002 → 003 → **004** → 005).
- [ ] T-393 (группа из трёх живых аккаунтов) пройден.
- [ ] Профиль `legacy`/`MV_GM_PATH` в gateway удалён задачей EPIC-003 I2 после зелёного S2 (проверить, что `if` в `actions.publish` снят).

---

## 6. Сводка и оценка

| Показатель | Значение |
|---|---|
| Задач разработки I1 | **20** (T-301…T-320): 12 — gateway (developer#1), 4 — бот (developer#2), 1 — integration, 1 — документация, 1 — текст FR-009 (BA/tech-writer), 1 — облачное уведомление (условная) |
| Задач разработки I2 | **8** (T-350…T-357): 4 — developer#1, 4 — developer#2 |
| Стендовых задач | **4** (T-390…T-393), слот разработчика не занимают |
| Всего | **32** задачи; из них 6 — e2e/integration, 2 — документация |
| Размеры | 25 × M, 7 × S; задач L нет |
| Подволн | I1 — 11, I2 — 4 |

**Статусы** реестром в этом файле не ведутся: статус задачи — в её карточке `tasks/T-NNN.md` и в `state.js` (ведёт оркестратор); допустимые значения — `todo / in-progress / review / done / blocked`. На 2026-09-13: T-301 и T-302 — `in-progress` (карточки ведут исполнители в ветках задач), остальные — `todo`. Размер и исполнитель — в заголовке задачи и в §4. *(Сверка 2026-09-13: заменён «Реестр статусов (ведёт tech-lead#3 …)» — таблица из 32 строк удалена, её содержание есть в заголовках задач.)*

**Оценка в неделях** (один разработчик-человек, M ≈ 1–1,5 дня с ревью и приёмкой; две параллельные дорожки с подволны 1.4):
- I1: критическая дорожка gateway — 12 задач ≈ 14–16 рабочих дней ≈ **3–3,2 недели**; дорожка бота (4 задачи) укладывается внутрь. Точка I1-α достигается к концу подволны 1.9 ≈ **на 3-й неделе** — совпадает с `epics.md` §2.
- Волна 1 по `epics.md` §4 длится 4–5 недель (ограничена EPIC-003). **Запас ≈ 1,5 недели** — совпадает с `decomposition-review.md` §2. Запас идёт на: замечания живого прогона I1-α, дефекты стыков с EPIC-002 (`shared/entity` v2, `FakeState`), досрочный старт T-356.
- I2: 8 задач на двух дорожках ≈ 5–6 дней на дорожку ≈ **1,5–2 недели** при 3–4 неделях волны 2. Запас ≈ 1,5–2 недели — на интеграционные дефекты и S2.

**Критический путь I1:** (T-301 ∥ **T-302**) → **T-303 → T-305 → T-307** → T-308 → T-313 → T-314 (в скобках выделено ядро, названное в design §3.4 как T-b → T-c → T-e → T-f). T-304 лежит на том же пути (T-305 и T-307 без проекции не работают) и в отчёте о ходе волны должен трактоваться как часть ядра.
**Критический путь I2:** T-350 → T-351 → T-357 (дорожка developer#1); дорожка группы (T-352 → T-353 → T-355) короче и в путь не входит.

**Риски нарезки (сверх design §8):**
1. `shared/entity` v2 и `FakeState` v0 меняются в волне 1 → T-304 переделывается. Реакция: `readmodel` только на типизированных геттерах; расхождения — запрос в EPIC-002, не правка заглушки. Буфер — запас 1,5 нед.
2. ~~Один разработчик до подволны 1.4 → три первые подволны строго последовательны~~ — **смягчён (сверка 2026-09-13)**: T-301 и T-302 идут параллельно (developer#1, developer#2); последовательной остаётся только T-303, которая открывает бота.
3. T-308 (`Harness`) — единственная задача, блокирующая сразу e2e соло, e2e бота и чужие команды. При задержке I1-α сдвигается целиком. Реакция: поставка в `develop` контрольным слиянием, приоритет ревью, API `Harness` v0 не меняется (сверка 2026-09-13).
4. ~~T-320 зависит от решения вне команды (З-3)~~ — **снят (сведение 3)**: З-3 решено (`GET /v1/worlds → worlds[].llm.cloud_enabled`), T-320 в статусе `planned`; остаточная зависимость — публикация `config.cloud_enabled` контекстом `llm` (EPIC-003 T-212), которая по C-06 v1.1 идёт при каждом старте `core`.
5. ~~Разошедшиеся требования по `/forget` (З-1, З-2)~~ — **снят (сведение 3)**: `end_reason` + `forget` (CHECK в T-302), `abandoned` публикует gateway, порядок каскада зафиксирован C-04 v1.1 (T-355). Миграция пишется сразу с полным CHECK — правило «I2 не добавляет миграций» соблюдается.

---

## 7. Правила приёмки задач тимлидом (tech-lead#3)

1. Сверка с DoD задачи и с дизайном (design §3.1/§3.2, component §-ссылки задачи) — по пунктам, не «в целом».
2. Прогон — в папке задачи `.worktrees/T-NNN`: `go test -short ./...` затронутых пакетов, `golangci-lint run`, `openapi_test`, где применимо — `-tags e2e`; `-tags integration` — только `go test -tags integration ./<пакеты задачи>/...`, по одному прогону, с проверкой, что контейнеров testcontainers не осталось (сверка 2026-09-13).
3. Проверка `dev-log.md`: подпись экземпляра, отклонения от дизайна, что не сделано.
4. Проверка карты владения: `git diff --name-only` не содержит путей чужих эпиков и `shared/*` вне `shared/testkit/gateway` — кроме файлов EPIC-001, которые задача прямо называет (`cmd/multiverse/contexts.go` в T-303, объявления в `shared/env/vars.go`, `go.mod`/`go.sum`, `.env.example`).
5. Несоответствие → возврат разработчику **конкретным списком пунктов DoD**, без переписывания кода тимлидом.
6. Статусы ведутся в карточках `tasks/T-NNN.md` и в `state.js` (оркестратор), а не в этом файле. Решение tech-lead#3 — секция «Приёмка» карточки (решение, дата, итог одной фразой); `done` не ставится, пока не заполнены «Выполнение», «Ревью» и «Учёт времени». После «принято» оркестратор сливает ветку задачи в ветку эпика. *(Сверка 2026-09-13: заменено «Статусы ведутся в этом файле… дублируются в `state.js`».)*
7. Эскалация: разработчик и `code-reviewer#3` не сошлись за 3 итерации → техническое решение принимает tech-lead#3; если вопрос про контракт или требование — вопрос пользователю через оркестратора.

---

## 8. Замечания к дизайну, контрактам и требованиям (переданы в отчёте tech-lead#3)

| # | Кому | Суть | Влияние на задачи |
|---|---|---|---|
| **З-1** | BA + system-analyst + architect#3 | `end_reason` при `/forget`: FR-061 v0.4 требует `end_reason=forget`, а `data-model.md` §5, `api-contracts.md` §2.3.14 и CHECK в `migrations/gateway/0001_init.sql` (component §4.2) допускают только `leave|idle|death|error` — вставка упадёт на CHECK | **РЕШЕНО (сведение 3, C-10 v1.1)**: `end_reason ∈ leave\|idle\|death\|error\|forget`, `forget` — не ошибка. Внесено: T-301 (схема), T-302 (CHECK), T-314, T-355. Подволна 1.2 разблокирована |
| **З-2** | BA + architect#3 | Статус `abandoned` при `/forget`: FR-061 п. 2 требует, чтобы персонаж переходил в `abandoned`; component §7.5 и §6 (`ForgetHooks`) этого не предусматривают — не указано, кто публикует `entity.update.proposed status=abandoned cause=forget` и как это соотносится с `group.left {cause: forget}` (FR-061 п. 3) против `cause=leave` в component §7.5 | **РЕШЕНО (сведение 3, C-02 v1.2 / C-04 v1.1)**: публикует **gateway** одним atomic `entity.update.proposed cause=forget`; State валидирует **только** `alive → abandoned` (`dead` остаётся `dead`); `abandoned` терминален и равен `dead` для inv-01/`NPCTarget`/стража; `group.left {cause: forget}`. Внесено: T-301, T-314, T-355 (`t.Skip` снят) |
| **З-3** | system-architect + architect#3 | US-008 (Must) содержит критерий «оператор включил облачный провайдер → бот показывает отдельное уведомление», но `config.cloud_enabled` — событие `system_events` (EPIC-003), а бот шину не читает и C-08 не имеет поля для этого флага. Нужно совместимое дополнение C-08 (поле в `GET /v1/worlds` или `GET /v1/players/{id}`) | **РЕШЕНО (сведение 3, C-08 v1.2 / C-06 v1.1)**: `GET /v1/worlds → worlds[].llm{cloud_enabled}` — единственное место; кэш бота `MV_TELEGRAM_CLOUD_FLAG_TTL=60s`, уведомление раз за сессию диалога; контекст `llm` публикует событие при каждом старте `core`. **T-320 разблокирована** |
| **З-4** | system-analyst + architect#1 | Группа без действующего лидера (все `alive` кончились, component §7.4): не определено представление `leader_id: null` в сущности `group` и в `GroupView` C-08; BA отметила это ещё при v0.4 | **РЕШЕНО (сведение 3, C-04 v1.1 / C-08 v1.2)**: `Group.leader_id: ref \| null`, `GroupView.leader_id: string \| null`, `group.leader_changed {leader: null, cause}`, `409 no_leader` (раньше `not_leader`). Внесено: T-301, T-352, T-355 |
| **З-5** | BA + security-engineer | Имя переменной allowlist расходится: `MV_TELEGRAM_ALLOWED_USER_IDS` (prd FR-130, threat-model SEC-06, US-008) против `MV_TELEGRAM_ALLOWED_USER_IDS` (component §11.3, design). Взято имя компонентного документа | **РЕШЕНО (сведение 3, З-5)**: единое имя — **`MV_TELEGRAM_ALLOWED_USER_IDS`** (по компонентному документу и `.env.example`); правки в prd/threat-model — за BA и security. Задачи T-310, T-390 без изменений |
| **З-6** | BA + architect#3 | FR-130 требует «правится оператором без перезапуска — Should»; ни component §10.2, ни design перезагрузку allowlist не описывают. В MVP-1 предлагаю принять как невыполненный Should и не расширять объём | **РЕШЕНО (сведение 3, З-6)**: в MVP-1 — невыполненный Should, объём не расширяем; кандидат в EPIC-013 (E-H: инвайт-коды/аутентификация заменят allowlist); пометку в FR-130 вносит BA. Задач не добавляется |
| **З-7** | architect#3 | `components/gateway-and-bot.md` §13 ссылается на ветку `epic/EPIC-004-gateway-bot`; канон — `epic/EPIC-004-gateway` (`teams.md` §1). В `design.md` исправлено tech-lead#3; компонентный документ — владение architect#3 | косметика, без влияния на задачи. **Пересмотрено (сверка 2026-09-13):** ветка создана как `epic/EPIC-004-gateway-bot` — по каталогу эпика (`journal.md` 2026-09-13), так что компонентный документ прав; `design.md` и этот файл приведены, `plan/teams.md` §1 и `plan/epics.md` §5 правятся в `develop` |
| **З-8** | architect#3 | design §2 «Трассировка» ссылается на SEC-03…12, но не упоминает SEC-26 (семантика `/forget` в OpenAPI и в уведомлении), хотя это обязательная мера MVP-1. Учтено в DoD T-303 и T-318 | учтено |
| **З-9** | tech-lead#1 | `.dev-team.json` → `counters.task` = 0 и обновляется тремя тимлидами параллельно (риск повторить инцидент с `open-questions.md`). TEAM-3 файл **не правила**; занятый диапазон — T-301…T-320, T-350…T-357, T-390…T-393. Предлагаю: счётчик сводит tech-lead#1 (или оркестратор) один раз после G3 | процесс |

**Статус после сведения 3 (2026-09-09, внесено tech-lead#1 от имени tech-lead#3):** З-1…З-6 — **решено**, блокировок DoD не осталось; З-7, З-8 — косметика/учтено; З-9 — счётчик `counters.task` сведён tech-lead#1 после G3 (`.dev-team.json`, `task = 393`). Основание: `architecture/consolidation.md` §14.1, `contracts.md` v0.4, ADR-017 «Дополнение 1».

---

## 9. История версий документа

| Версия | Дата | Автор | Что изменено |
|---|---|---|---|
| 0.1 | 2026-09-09 | tech-lead#3 | нарезка задач A4 шаг 3 по design v0.1 |
| 0.1.1 | 2026-09-09 | tech-lead#1 от имени tech-lead#3 | правки сведения 3 (`consolidation.md` §14, `contracts.md` v0.4) |
| 0.1.1 + T-416 | 2026-09-11 | tech-lead#1 | ревизия контрактов T-416 (`contracts.md` v0.6–v0.7) |
| 0.1.2 | 2026-09-13 | tech-lead#3 | **редакционная правка** по сверке плана с деревом: ветка `epic/EPIC-004-gateway-bot` от `develop`, задачи в `task/T-NNN`, без `go.work` и `integration/mvp-1`, контрольные слияния в `develop`, `commits=auto`, `contracts.md` v0.10, статусы в карточках и `state.js`, интеграционные прогоны по разрешению пользователя; DoD T-301…T-319 приведены к дереву. Объём и состав задач не менялись; новые задачи не заводились |
| 0.1.3 | 2026-09-13 | tech-lead#3 | приёмка T-303: строка приёмки в §T-303; строки DoD T-306, T-309, T-311, T-314, T-356 и пометка о расхождении в T-355 (до T-456 п. 5). Объём задач не менялся, новые задачи не заводились |
| 0.1.3 + T-318 | 2026-09-13 | tech-lead#3 | приёмка T-318: статус и строка приёмки в §T-318; строки DoD T-311 (текст уведомления, N-2, R2-N-2, Р-1, Р-2 A, Р-3 A, Mi-5, I-2), T-317 и T-390 (условия правдивости У-1, У-2, У-4, У-5). Объём задач не менялся, новые задачи не заводились |
| 0.1.4 | 2026-09-13 | tech-lead#3 | приёмка T-305: статус и строка приёмки в §T-305, строки DoD T-305 по `replayClock` (отметка T-458 Н-2, C-01 v1.11). Решения оркестратора по вопросам приёмки T-318: строки DoD T-320 (У-9) и T-314 (У-8). Строки DoD T-306 (Mi-6, резерв `seq`, `FilterText(KindName)`) и T-307 (`entity.update.rejected` движения и отдыха). Новая задача T-467 — стабилизация флака `shared/testkit/gateway`, раздел в §1 после T-320. Объём задач T-301…T-393 не менялся |
