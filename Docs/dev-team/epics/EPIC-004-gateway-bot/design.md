# Дизайн решения: EPIC-004 «Вход игрока: gateway и Telegram-бот»

Версия 0.1 · 2026-09-09 · architect#3 (TEAM-3) · этап A4 «Планирование», шаг 2 · для tech-lead#3 (нарезка `tasks.md`), developer#1–#2, tester#3, code-reviewer#3.
Основание: `architecture/components/gateway-and-bot.md` v0.2 (детальный дизайн блока — здесь не дублируется, даются ссылки на разделы), ADR-018/019/020 с дополнениями после G2, ADR-006/009/010 с дополнениями, `contracts.md` v0.2 (C-04, C-08, C-10 — поставляем; C-01, C-02, C-05, C-06, C-14 — потребляем), `threat-model.md` (SEC-03, 06, 07, 08, 10, 11, 12 — обязательны в MVP-1), `plan/epics.md` v0.2 §2 EPIC-004, `plan/teams.md` §4 (слоты TEAM-3: 1 → 2), `plan/decomposition-review.md` §2 (I1 ~14 задач, I2 ~6), `journal.md` (U-1, U-5, U-6, U-7; G2 утверждён 2026-09-09).
Ветка: `epic/EPIC-004-gateway` от `integration/mvp-1` (одна на весь MVP-1; создаётся tech-lead#3 при старте волны 1). Код не меняется на этом шаге.

---

## 1. Цель и границы

**Цель.** Живой человек через Telegram (и CI-харнесс через тот же HTTP API) регистрируется с согласием, создаёт персонажа, играет соло в «Тёмном лесу» (I1) и в группе 2–6 с раундами (I2); внешний ID мессенджера не покидает `links.db`; каждое действие подтверждается ≤ 300 мс, результаты механики и нарратив приходят отдельными сообщениями; `/forget` физически удаляет связку.

**Входит** (по `ownership.md` §1): `internal/gateway/**` (контекст `gateway` процесса `cmd/multiverse`), `cmd/telegram-bot/**`, `api/gateway.openapi.yaml` (включая раздел `admin` — прокси C-06), `internal/gateway/migrations/{links,gateway}/*.sql`, `shared/testkit/gateway` (`FakeGateway`, `Harness` — замена v0 из F-10), схемы событий `schemas/events/{player.*,group.*,round.*,analytics.session.*,analytics.turn.completed}.v1.json`, префикс `snapshots-{world}/gateway/`, файлы `links.db`/`gateway.db`, перенос `services/game-service` → `services/_archive/game-service/` после S9 (U-1).

**Не входит.** Механика и State (EPIC-002), рой/LLM/фильтр (a) на выходе/`NarrativeFilter` (EPIC-003), память и `mvctl report` (EPIC-005), шина/контракты/`membus`/CI-каркас (EPIC-001), Discord-бот, WebSocket-стрим (`501`), аутентификация клиентов, инвайт-коды, общий чат группы, свободный текст команд, право на забвение — E-H (EPIC-013). Изменения `shared/*` и контрактов — только запросом к system-architect.

**Границы, которые нельзя нарушать:** gateway не пишет сущности мира (только `entity.*.proposed`, C-02); `round.closed` публикуется до обработки раунда; таймеры только через `Clock`, в `--mode=replay` выключены; `player_events` — только `actor_kind ∈ human|ci|sim`; внешний ID — только `links.db`; `cmd/telegram-bot` импортирует из платформенного кода только `internal/gateway/{client,api}`.

---

## 2. Трассировка требований

| Требование | Инкремент | Где реализуется (компонентный документ) | Как принимается |
|---|---|---|---|
| US-001 создание персонажа и возврат (FR-001, FR-006, FR-023, FR-060, A-7) | I1 | §7.1 (`characters`, `pending_characters`, `AwaitFact`), §5.2 `GET /players/{id}` | e2e `solo-30` шаги регистрации; unit `links` (alive → тот же `player_id`; dead → новый); стенд I1-α |
| US-008 Telegram-бот как эталонный клиент (FR-002, FR-003, FR-004, FR-009, FR-052, BR-09, NFR-003, NFR-092) | I1 (соло), I2 (группа) | §10 (бот), §8 (outbox), §5 (API) | e2e бота на `FakeUpdateSource`/`FakeSender`; стенд S9 с замером `ack_latency_p95_ms` |
| US-009 псевдонимизация и `/forget` (FR-060, FR-061, BR-07, NFR-041, NFR-042) | I1 | §4.1, §7.5, §11.1 | e2e `privacy-scan`, `forget`; integration `strings`-скан файла и WAL |
| US-011 снапшот gateway, восстановление (FR-031…033, NFR-010/011; C-14) | I1 | §11.2, компонент `snapshot` | e2e `recovery` (рестарт контекста in-process → сессии/раунды/outbox сохранены); S3 на интеграции |
| US-016 точки вставки фильтра — вход (FR-056; Should) | I1 | §5.4 `InputFilter` (noop, `FilterResult.Text` → шина) | unit: тестовая реализация «заменить слово» → в `player.said.text` заменённый текст |
| US-019 фича-флаг миграции — часть gateway (FR-014) | I1 | §11.3 `MV_GM_PATH`, §13 строка `gm.created` | unit: при `legacy` публикуется `gm.created`, при `agent` — нет; удаляется в EPIC-003 I2 (S5) |
| US-038 события аналитики (FR-084…FR-088, BR-17, NFR-036) | I1 | §7.6 (`session`, `turns`) | unit парность `session.started/ended`, `turn.completed` на каждый принятый/отклонённый ход; e2e — фикстура для `mvctl report` (EPIC-005) |
| US-006 групповая сессия из трёх (FR-003, FR-005, FR-025, BR-06, BR-13, NFR-005) | I2 | §7.4 (`groups`), §8.1 (групповая доставка) | e2e `group-3x30` (три клиента `ci-harness`); S2 на интеграции |
| US-007 бой по раундам (FR-025, FR-030, FR-033, BR-03, BR-13, NFR-012) | I2 | §9, ADR-020 (`rounds`) | unit `rounds` с `FakeClock` (all_acted, timeout, explicit, idle, смерть, restore, replay); e2e `group-3x30` с `rounds/close` |
| US-002 / US-004 (вклад: доставка `template`, S1) | I1 | §8.1 (`narrative` с `generated_by`) | интеграция I1 (S1/S9) — приёмка EPIC-003 |
| UC-001…005, 007, 009, 012, 027, 031 | I1 | §7 | e2e `solo-30`, `death`, `flee-fail`, `forget` |
| UC-014…017, 020 (адресность) | I2 | §7.3–7.4, §9 | e2e `group-3x30` |
| SEC-03 (`gateway.db` без внешних ID, `link_id`) | I1 | §4, ADR-019 доп. п. 2 | тест миграций (имена колонок), `privacy-scan` |
| SEC-06 allowlist, SEC-07 только личные чаты | I1 | §10.2 `access.Gate` | unit бота: чужой id / групповой чат → gateway не вызван |
| SEC-08 редакция токена | I1 | §11.1, `privacy.Redact` | unit: строка ошибки без токена; grep логов e2e |
| SEC-10 без `parse_mode` | I1 | §10.4 | unit `Sender` с `[x](http://…)`, `<a>`, `*` |
| SEC-11 лимиты (20/мин бот, 30/мин `player_id`, 64 КиБ, один long-poll) | I1 | §5.1, §10.2 | unit `api`/`access` |
| SEC-12 `{client_id}` == `X-Client-Id`, `actor_kind`, admin по списку | I1 (клиенты), I2 (`ci/sim`, admin-прокси) | §5.1, §5.2 | unit `api` `403`; ревью prod-`.env` без `ci-harness` |
| NFR-003 ack ≤ 300 мс | I1 | §5.5 (2 SQL + `Publish`) | стенд S9 (`ack_latency_p95_ms`); e2e — таймер ≤ 100 мс на membus |
| NFR-013 идемпотентность | I1 | §4.2 `processed_events`, `idempotency_keys`, индекс `(event_id, player_id)` | e2e с `--chaos=duplicate` |
| NFR-041/042/045 | I1 | §11.1 | `privacy-scan`, `forget`, unit `consent` |
| NFR-092 один API для клиентов | I1 | `internal/gateway/client` общий для бота и `Harness` | compile-time: бот и харнесс на одном пакете |

Полные списки US/FR/NFR за эпиком — `plan/epics.md` §2 EPIC-004; матрица «NFR → как» — компонентный документ §12.

---

## 3. Подход и инкременты

### 3.1. I1 «соло» (волна 1; ~14 задач; developer#1, затем developer#2 на боте)

Состав (что должно работать к приёмке I1 тимлидом команды):

1. **Хранилище и миграции** — `store` (две БД, PRAGMA по ADR-019, `goose` из embed), `migrations/links/0001_init.sql` (с `link_id`), `migrations/gateway/0001_init.sql` (все таблицы §4.2, включая `rounds`/`group_participation` — схема целиком в I1, чтобы I2 не менял миграцию); integration-тесты миграций и физического `/forget`.
2. **Регистрация с согласием** — `links` (`resolve` создаёт `pending_consent` + `link_id`; `consent`; `forget` с checkpoint/vacuum и хуками; `RouteFor`; `character_requests` по `link_id`), `characters` (§7.1: `entity.create.proposed` → `AwaitFact ≤ 2 с` → `201/200/202 creating`), `GET /v1/worlds`, `GET /v1/players/{id}`.
3. **Соло-ход** — `actions` (таблица предусловий §5.4 для соло-словаря `enter/leave/look/attack/flee/rest/say/defend`, `InputFilter` noop, идемпотентность `action_key`, rate limit 30/мин, `player.*` + `entity.update.proposed` для `enter/leave/rest`; `MV_GM_PATH=legacy` → `gm.created`), `readmodel` (проекция + bootstrap из `state/latest.json` + `AwaitFact`), `consumer` (4 топика, дедуп, курсоры, обработчики `entity.*`, `combat.decided`, `encounter.started/ended`, `narrative.output`), `session` + `turns` (§7.6, `analytics.*`).
4. **Outbox** — `outbox` (`Enqueue` идемпотентно, `Lease` «голова очереди на игрока», `Ack`, sweeper, `render.Mechanics` golden-тексты), long-poll `GET/POST /v1/clients/{id}/deliveries[/ack]` с `pollguard`.
5. **API-обвязка** — `api` (server, router, middleware §5.1, ошибки §1.6 + коды C-08 v1.1, DTO), `api/gateway.openapi.yaml` + `openapi_test`, `/health`, `client` (Go-клиент C-08 с повторами), `snapshot` (по `SIGTERM` и `session.ended`, C-14), режимы `live|replay` с `Journal.End()`.
6. **Testkit (поставляем как поставщик C-04/C-08)** — `shared/testkit/gateway`: `FakeGateway` (in-process HTTP поверх реального `internal/gateway` на `membus`), `Harness` (Go-клиент + фикстуры `player-A/B/C`, `RegisterAndEnter`, `Act`, `AwaitDelivery`, `CloseRound`, режим генератора `player.*` прямо в `membus` — замена v0 из F-10 с сохранением сигнатур v0).
7. **Бот** — `cmd/telegram-bot`: `config` (`MV_*`), `access.Gate` (allowlist, личные чаты, 20/мин), `updates`/`sender` (интерфейсы + `go-telegram/bot` v1.25 + фейки), `commands`, `flow` (онбординг FSM, `/forget confirm`), `render` (уведомление FR-009 по тексту BA, `/help`, ошибки по `code`, доставки без `parse_mode`), `deliver` (long-poll → send → ack), `privacy` (handler + `Redact` токена).
8. **CI-харнесс как второй клиент** — e2e `solo-30`, `death`, `flee-fail`, `forget`, `recovery`, `privacy-scan` через `Harness` (`X-Client-Id: ci-harness`, `X-Actor-Kind: ci`) на `membus` + `FakeState` + `FakeNarrator`.
9. **Аналитика сессий** — `analytics.session.started/ended`, `analytics.turn.completed` по C-10 со схемами в `schemas/events/`; фикстура `testdata/analytics/solo-30.jsonl` для EPIC-005 005-ops.

**Точка I1-α «соло на шаблонах через бота»** (~3-я неделя волны 1; тег `mvp-1/i1-alpha`): бот + gateway в `integration/mvp-1` вместе с EPIC-002 I1, нарратив — `FakeNarrator` (`generated_by=template`), без роя и LLM. Человек играет через живой Telegram на стенде; замечания → задачи EPIC-002/004. Для I1-α нужны пункты 1–5, 7 и e2e `solo-30`; `snapshot`, `recovery`, `privacy-scan` могут завершаться между I1-α и I1.

**Готовность I1** (`epics.md`): сквозной соло-ход через бота на стенде (I1-α на `FakeNarrator`, затем S1/S9 с роем на интеграции I1); снапшот gateway восстанавливается (S3); e2e зелёные; unit SEC-03…12 зелёные.

### 3.2. I2 «группа и раунды» (волна 2; ~6–7 задач; developer#1 + developer#2)

Старт — по приёмке I1 тимлидом команды (правило `epics.md` §2); слияние — после зелёного интеграционного прогона I1 и **после** слияния EPIC-002 I2 (atomic-группы) и EPIC-003 I2 (раунд в `encounter`, `group-narrator`) — порядок 002 → 003 → 004.

1. **Группа** — `groups` (§7.4: `group.create/join/leave` через atomic `entity.*.proposed`, `202 pending` при таймауте факта, лидер BR-13 при выходе и — по решению BA — при смерти, `group.entered_region/left_region` при `enter/leave` лидера), `GET /v1/groups/{id}`, предусловия группы в `actions.Validate` (`not_leader`, `group_full`, `in_encounter`…).
2. **Координатор раундов** — `rounds` по ADR-020 (+доп. п. 1: параметры из `encounter.started.round{}`): открытие по `encounter.started` и первому действию, `Accept`/`already_acted`, закрытие `all_acted|timeout|explicit`, порядок `player.defended` → `round.closed`, `Restore`, ветка `replay`.
3. **Лидер, idle** — `group_participation`, предложение `participation=idle/active`, `OnParticipantsChanged` (смерть/бегство/выход → пересчёт `expected`), `TransferLeadership`.
4. **`POST /v1/scopes/{scope_id}/rounds/close`** (только `ci`) + `Harness.CloseRound`; e2e `group-3x30` тремя клиентами `ci-harness` (три `external_id` платформы `ci`).
5. **Групповая доставка** — `outbox` адресаты по scope (`combat.decided` всем `alive`, `narrative.output` по `recipients[]`, `group.*`/`round.opened` участникам, `player.said` всем кроме автора); бот — команды `/group create|join|leave`, клавиатура боя для группы, тексты состава/раунда.
6. **`/forget`-каскад для группы** — `ForgetHooks`: выход из группы (`group.left`, atomic-предложение; во встрече — без исполнения `flee`, встреча остаётся Task-агенту), передача лидерства, `session.UpdateParticipants`, `DropForPlayer`; e2e `forget` в группе.
7. **Прокси `/v1/admin/*` к `MV_CORE_URL`** и `actor_kind` `ci/sim` для клиентов из `MV_GATEWAY_ACTOR_KIND_CLIENTS` (список допуска клиентов — `MV_GATEWAY_CLIENT_IDS`; прежняя одна строка `MV_GATEWAY_CLIENTS` в манифест не принята, C-08 v1.3); раздел `admin` в OpenAPI. (Может быть сделан в конце I1, если EPIC-003 I1b успел выставить admin-порт — тогда S14 на интеграции I1 идёт через gateway.)

**Готовность I2:** S2 с харнессом из трёх клиентов на интеграции; unit `rounds` полный набор; стенд — группа из трёх живых Telegram-аккаунтов из allowlist (владелец + тестеры).

### 3.3. Заглушки и порядок относительно других команд

| Направление | Контракт | До готовности реального | Кто и когда заменяет |
|---|---|---|---|
| Потребляем | C-02 факты State | `testkit/state.FakeState` v0 (F-10, в `integration/mvp-1` с конца волны 0) | EPIC-002 I1 — реализация State; TEAM-3 заглушку не правит, дефекты — запрос владельцу |
| Потребляем | C-05 нарратив/механика | `testkit/swarm.FakeNarrator` (T-220: все шесть поводов C-05, `generated_by=template`) и `testkit/swarm.FakeEncounter` (T-219: Phase 1 боя — `dice.rolled`, `combat.decided`, атомарный пакет, `encounter.*`); механика до EPIC-002 T-053 — `FixedMechanics`. Для read-model встречи и координатора — правила C-05 v1.4 п. 4–6 и C-04 v1.3: конец встречи — первое из «факт сущности `state=resolved`» и `encounter.ended` | EPIC-003 I1a (реальный `FakeNarrator` на шаблонах) → I1b (рой) |
| Потребляем | C-14 `state/latest.json` | фикстура `testdata/snapshots/state/latest.json` (F-10); `readmodel.bootstrap` пишется против неё | EPIC-002 I1 (реальный указатель + объект) |
| Потребляем | C-01 шина/журнал | `membus` с `Journal.End()` (F-5t) | kafka-адаптер — тот же интерфейс, integration-тест `consumer` на testcontainers |
| Потребляем | C-06 admin `core` | не нужен до I2 п. 7; для unit прокси — `httptest.Server` | EPIC-003 I1b |
| Поставляем | C-04 `player.*` | `testkit/gateway.Harness` v0 (F-10): генератор `player.*` из фикстур в `membus` без HTTP — им пользуются EPIC-002/003 с волны 1 | TEAM-3 в I1 п. 6 заменяет v0 реализацией, **сохраняя сигнатуры v0** (`Harness.Act(player, action)` → тот же тип события); слияние — с I1 (порядок 002 → 004 → 003) |
| Поставляем | C-08 HTTP API | `testkit/gateway.FakeGateway` — внутри команды: бот пишется против интерфейса `client` (unit на `httptest`) и против `FakeGateway`, когда он готов | TEAM-3, I1 п. 6 (после п. 1–5) |
| Поставляем | C-10 аналитика | схемы `analytics.session.*`, `analytics.turn.completed` в `schemas/events/` — первая задача I1 (нужны EPIC-005 005-ops для `mvctl report`) | TEAM-3, I1 |

Порядок слияния I1 в `integration/mvp-1`: 002 → **004** → 003 (`teams.md` §3.3); I2: 002 → 003 → **004** → 005. Первая задача I1 — схемы событий C-04/C-10 + `openapi.yaml` скелет (контракты видны другим командам раньше кода).

### 3.4. Порядок задач внутри эпика (рекомендация для нарезки)

```
I1 (developer#1):  T-a схемы событий + OpenAPI-скелет + Go-клиент DTO
                → T-b store + миграции (обе БД) + integration-тесты
                → T-c links (resolve/consent/forget/link_id/RouteFor) + api middleware/errors/server
                → T-d readmodel (+bootstrap, waiters) + consumer (dispatcher, entity/encounter)
                → T-e actions (validate/idempotency/ratelimit/inputfilter/publish) + characters + session/turns
                → T-f outbox (store/lease/ack/render/longpoll/sweeper) + consumer narrative/combat + deliveries API
                → T-g FakeGateway + Harness (замена v0) + e2e solo-30            ← с этого момента developer#2 на боте
                → T-h snapshot + replay/Journal.End + /health + e2e recovery/privacy-scan/forget/death/flee-fail
I1 (developer#2, после T-g или на интерфейсе client с T-c):
                   T-i бот: config/access/updates/sender/commands/privacy (unit на фейках)
                → T-j бот: flow (онбординг, /forget) + render (тексты FR-009 от BA)
                → T-k бот: deliver + main wiring + e2e бота на FakeGateway
                → T-l стенд: живой Telegram, S9-чек-лист, замер ack (человек + tester#3; не занимает слот)
I2: developer#1 — rounds (coordinator/state/store + unit FakeClock) → rounds/close + Harness.CloseRound + e2e group-3x30
    developer#2 — groups (+валидация группы) → групповая доставка + бот /group → forget-каскад группы → admin proxy + actor_kind ci/sim
```

Каждая задача ≤ M (дифф ≤ ~600 строк с тестами); задача содержит свои unit-тесты; e2e-задачи — отдельные.

---

## 4. Затрагиваемые компоненты и файлы

Структура каталогов — компонентный документ §3 (полная). Новое в v0.2: `cmd/telegram-bot/internal/access/` (allowlist, личные чаты, лимит), `privacy.Redact`, `MV_*` в `config`. Общий код, который **читаем, но не меняем**: `shared/{eventbus,contracts,jsonpath,objstore,env,logging,clock,runtime,entity}`, `shared/eventbus/membus`, `shared/testkit/{state,swarm}`. Файлы вне блока, которые трогает эпик: `go.mod` (добавление `modernc.org/sqlite` v1.58.0, `github.com/pressly/goose/v3` v3.28.0, `github.com/go-telegram/bot` v1.25.0, `github.com/oklog/ulid/v2` — уже учтены F-4; отметка в отчёте), `schemas/events/*` своих типов + запись в `shared/contracts/registry.go` через PR (ревью system-architect), `.env.example` (переменные §11.3 — через tech-lead#1/devops), `docker-compose.yml` сервис `telegram-bot` профиля `bot` (devops, F-6 — уже предусмотрен), `services/game-service` → `services/_archive/` после S9.

---

## 5. Изменения данных / API / конфигурации

- **Данные**: две новые БД SQLite (`links.db`, `gateway.db`) — схемы §4.1–4.2 компонентного документа v0.2 (`link_id`, `character_requests(link_id, action_key)`, `rounds.timeout_ms/idle_after_missed`); миграции только `0001_init.sql` для каждой (I2 не добавляет миграций — таблицы группы/раундов уже в I1). Объекты `snapshots-{world}/gateway/{ts}-{seq}.json` + `latest.json` (указатель, C-14 v1.1). Данные as-is game-service не мигрируются.
- **API**: `api/gateway.openapi.yaml` v1.0.0 по §5.6 с кодами C-08 v1.1; события C-04/C-10 по `api-contracts.md` §2.3.1–2.3.3, 2.3.14 (system-analyst правит §1.1/§1.6 под v0.2 — параллельная задача A4).
- **Конфигурация**: переменные `MV_*` §11.3 (gateway 21, бот 10); `MV_TELEGRAM_ALLOWED_USER_IDS` заполняет владелец; prod-`.env` держит все три списка клиентов (`MV_GATEWAY_CLIENT_IDS`, `MV_GATEWAY_ACTOR_KIND_CLIENTS`, `MV_CORE_ADMIN_CLIENTS`) и без `ci-harness` в каждом (`.env.example`, T-411); профиль `bot` compose с `env_file` только у `telegram-bot`; том `MV_GATEWAY_DATA_DIR` именованный, `0700`.

---

## 6. Безопасность (что учтено)

Обязательные для MVP-1 меры threat-model и где они закрыты: SEC-03 (`link_id`, `gateway.db` без внешних ID — §4, ADR-019 доп. п. 2), SEC-04/05 (`secure_delete`, checkpoint+vacuum сразу — §4.1, ADR-019 доп. п. 1), SEC-06/07 (allowlist, личные чаты, `from.id` — §10.2, ADR-018 доп. п. 1), SEC-08 (редакция токена — §11.1, ADR-018 доп. п. 3), SEC-10 (без `parse_mode` — §10.4), SEC-11 (лимиты — §5.1, §10.2), SEC-12 (`client_mismatch`, платформа маршрута, `actor_kind`, admin по списку — §5.1–5.2), SEC-13 (публикация только `127.0.0.1` — devops/compose-lint), SEC-01/02 (логи без ПДн — `nolog`, `LogValue`, `privacy.Handler`), NFR-045 (согласие обязательно для `POST /characters`). Остаточные риски, принятые на G1/G2: вход категории (a) не фильтруется (InputFilter noop; смягчается allowlist); шина без SASL в доверенном контуре; `ci-harness` доверенный клиент в `dev`. Текст уведомления FR-009 (облако, retention, бэкап, что удаляет `/forget`) — от BA (T-14), бот рендерит его из одного файла `render/notice.go`, чтобы правка текста не трогала логику.

---

## 7. Тестируемость (как проверим; ADR-010)

| Уровень | Что | Где / когда |
|---|---|---|
| unit (`-short`) | полный список — компонентный документ §15 (таблица предусловий, `rounds` с `FakeClock`, `outbox.Lease`, `links`, `session`, `turns`, `render` golden, `api` middleware/`openapi_test`, `commands`, `flow` на `FakeUpdateSource`/`FakeSender`) + блок SEC-03…12 | каждая задача; порог покрытия ≥ 60 % по `actions,rounds,outbox,links,turns,session` |
| integration (`integration`) | миграции обеих БД на временном файле; `/forget` физически (файл + WAL); `consumer` на testcontainers Redpanda (дедуп, курсоры, DLQ, `Journal.End`); версии образов — `testkit.Versions()` | T-b, T-d, T-f |
| e2e (`e2e`, один процесс `--contexts=all --mode=replay --bus=memory`) | `solo-30`, `death`, `flee-fail`, `forget`, `recovery`, `privacy-scan` (I1); `group-3x30` с `rounds/close`, `forget` в группе (I2); `--chaos=duplicate` для NFR-013 | T-g, T-h, I2 |
| e2e бота (детерминированный) | `FakeUpdateSource` подаёт сценарий (`/start` → согласие → имя → `/enter` → `/attack` → `/forget confirm`), `FakeSender` записывает исходящие; gateway — `FakeGateway`; проверяются тексты, клавиатуры, стабильность `action_key`, порядок, ack, отказ чужому id/групповому чату | T-k |
| privacy-scan | регистрация с известным тестовым ID → grep по `gateway.db` (+WAL), событиям `membus`, логам обоих процессов (в т. ч. на фрагмент токена) = 0; job `security` CI сканирует `testdata/` | T-h; на стенде — перед I1 (чек-лист security-review п. 2) |
| стенд (вне CI, человек + tester#3) | живой Telegram: S9 сквозной соло-ход на I1-α и на I1; `ack_latency_p95_ms` (NFR-003); учения `/forget` (`strings` чист); группа из трёх аккаунтов (I2); allowlist заполнен только владельцем/тестерами | конец I1-α, I1, I2 — задачи с пометкой «стенд», слот разработчика не занимают |

Детерминизм e2e: `--id-source=sequence` для ULID (`player_id`, `link_id`, `d-*`), `clock.Manual`, `membus`; `Harness` использует фикстуры `player-A/B/C` платформы `ci` (внешние ID = имена фикстур — не ПДн, записи допустимы для golden/`mvctl record`).

---

## 8. Риски и реакции

| Риск | Вероятность / влияние | Реакция |
|---|---|---|
| Telegram Bot API: изменение поведения `getUpdates`/лимитов, недоступность `api.telegram.org` из сети оператора, `429` при рассылке группе | С / С | библиотека за интерфейсами `UpdateSource`/`Sender` (ADR-018 п. 2) — вся логика тестируется на фейках; `429` → `retry_after` в `deliver`; недоступность Telegram не теряет данные (outbox с лизингом/TTL 24 ч); версия библиотеки пинится, обновление — отдельной задачей |
| Политика мессенджера по 18+ контенту: Telegram не даёт механизма возрастной проверки для ботов; бот с 18+ нарративом может быть ограничен/заблокирован при жалобе | Н / В | самодекларация 18+ в `/start` (DR-23) + allowlist только известных тестеров (U-7) + бот не публикуется в каталогах; при блокировке — токен отзывается, gateway и данные не затронуты (бот — тонкий клиент); Discord — тот же API (E-H) |
| ПДн: утечка внешнего ID через новый путь (лог библиотеки, ошибка, тестовые данные), неполное удаление при `/forget` (WAL, бэкап) | С / В | приватность по построению (§11.1) + три теста (unit колонок, integration `strings`, e2e `privacy-scan`) + скан на стенде перед I1; бэкап `links.db` шифруется и живёт ≤ 30 дней (devops), в уведомлении FR-009 это сказано (BA) |
| `shared/entity` v2 и `FakeState` v0 меняются под ногами в волне 1 (decomposition-review §3.3) | С / С | `readmodel` использует только `entity.Ref`/типизированные геттеры v2 из F-10; расхождения — запрос владельцу (EPIC-002), не правка заглушки |
| `FakeNarrator` v0 не публикует `narrative.output` на нужные события для I1-α | С / Н | контракт заглушки C-05 зафиксирован (`combat.decided`/`round.closed`/`player.looked`); e2e `solo-30` — первый потребитель; дефект → задача EPIC-003 I1a |
| Один разработчик в первых подволнах (слоты 1 → 2): бот ждёт `FakeGateway` | В / С | бот стартует на интерфейсе `client` с `httptest` после T-c (DTO и коды известны из OpenAPI), `FakeGateway` подключается позже; порядок §3.4 |
| Гонка «действие vs таймер» и восстановление раундов после рестарта (I2) | С / С | мьютекс на scope, `closing`-состояние, идемпотентная публикация по `(scope, seq)`, unit с `FakeClock` + e2e `recovery` в группе (ADR-020) |
| Расхождение схемы `gateway.openapi.yaml` с кодом и с `api-contracts.md` (правится system-analyst параллельно) | С / Н | `openapi_test` (маршруты и коды ↔ YAML) в CI; при расхождении с `api-contracts.md` — источник истины `contracts.md` v0.2 |
| Профиль `legacy` и `MV_GM_PATH` в gateway живут до S5 | Н / Н | один `if` в `actions.publish`, удаляется задачей EPIC-003 I2; тест на оба значения флага |

---

## 9. Допущения

1. Строка `links` создаётся при первом `resolve` (`pending_consent`, с `link_id`) — до согласия; допустимо по UC-001 E1 и после allowlist (компонентный документ §16 п. 6).
2. Текст уведомления FR-009 приходит от BA до задачи T-j; до этого бот использует текст US-008 (четыре пункта) как заглушку в `render/notice.go`.
3. FR-025 (`say` не открывает раунд) и BR-13 (лидер при смерти) закрепляются BA до старта I2; реализация — по ADR-020 п. 8 и C-04 v0.2, изменение потребует только правки правила в `groups`/`rounds`.
4. `encounter.started.round{}` публикует EPIC-003 I2; в I1 и I1-α раундов нет (соло), env-значения по умолчанию достаточны.
5. Стенд с живым Telegram — машина пользователя; allowlist заполняет пользователь; тестеры — известный круг (U-7).
6. `Harness` v0 из F-10 имеет сигнатуры, совместимые с реализацией I1 (согласовано через `ownership.md`: v0 создаёт EPIC-001 по спецификации §15 компонентного документа).
7. Имя ветки — `epic/EPIC-004-gateway` (канон `plan/teams.md` §1 и `plan/epics.md` §5); расхождение с черновиком v0.1 (`epic/EPIC-004-gateway-bot`) устранено tech-lead#3 при нарезке задач. Каталог артефактов эпика остаётся `Docs/dev-team/epics/EPIC-004-gateway-bot/`.

Связь: ADR-018, ADR-019, ADR-020 (с дополнениями после G2), ADR-006, ADR-009, ADR-010; C-01, C-02, C-04, C-05, C-06, C-08, C-10, C-14.
