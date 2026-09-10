# Контракты API и событий

**Инициатива:** PROJECT · **Этап:** A4 (планирование) · **Версия:** 0.2.1 · 2026-09-09 · системный аналитик (dev-team); точечные правки v0.2.1 — system-architect (сведение 3, `architecture/consolidation.md` §14; system-analyst не запущен)
**Основание:** `prd.md` v0.2 (FR-002…FR-005, FR-009, FR-013, FR-021, FR-022, FR-030…FR-036, FR-045, FR-050, FR-060, FR-084…FR-088, FR-120…FR-128, BR-02…BR-08, BR-13, BR-16); `metrics.md` §4; `domain-review.md` §3; аудит as-is (`architecture/audit-facts.md` §4–5); код `shared/eventbus` (`Event{id,type,timestamp,source,world,scope,payload,relations}`, `EventPayload` builder: `WithEntity/WithTarget/WithSource/WithWorld/WithScope`, `SetNested`, `event.Path()`), `shared/agent/agent_types.go` (`AgentLevel`, `AgentBlueprint`), `services/game-service` (HTTP как есть); решения G1; **v0.2** — `architecture/contracts.md` v0.2 (C-01…C-15, утверждены на G2), `architecture/consolidation.md` §2–§7, §9, ADR-007 (конверт `meta`), ADR-009 (`link_id`, allowlist, лимиты), ADR-020 (раунды), решения G2 (`journal.md`).

Состав: **§1** HTTP API gateway для эталонного клиента; **§2** контракты событий MVP-1 с картой «событие → топик → издатель → потребители»; **§3** контракт блупринта агента; **§4** сверка с `contracts.md` v0.2. Пометки «решение архитектора» v0.1 сняты там, где решение принято (ссылка на C-NN/ADR).

**Имена компонентов.** В v0.1 использовались имена as-is сервисов; после ADR-001 (модульный монолит) они читаются так: `game-service` → **gateway** (контекст `internal/gateway`, EPIC-004); `entity-manager` → **State** (`internal/state`, EPIC-002); `рантайм роя` / `конвейер агента` → **Swarm / LLM-шлюз** (`internal/swarm`, `internal/llm`, EPIC-003); `механика` → `internal/mechanics` (EPIC-002, вызывается агентом встречи); `semantic-memory` → **memory** (`internal/memory`, EPIC-005 005-memory — отрезаемая часть, решение G2: при её отсутствии Swarm использует `journalContext`, C-09). В тексте ниже старые имена сохранены там, где правка не менялась по существу.

### Изменения v0.2 (changelog)

| Раздел | Изменение | Основание |
|---|---|---|
| **v0.2.1 (сведение 3, system-architect)** §1.2, §2.3.2, §2.3.4 | `/forget`: `entity.update.proposed {cause: forget}` от gateway (`status=abandoned`, только из `alive`), `group.left`/`group.leader_changed {cause: forget}`, `end_reason=forget` | C-02 v1.2, C-04 v1.1, C-08 v1.2 (З-1, З-2) |
| v0.2.1 §1.3, §1.6, §1.7, §2.3.2 | `GroupView.leader_id: string \| null`, `group.leader_changed {leader: null}`, `409 no_leader` | C-04 v1.1, C-08 v1.2 (З-4) |
| v0.2.1 §1.3, §2.3.16 | `GET /v1/worlds → worlds[].llm{cloud_enabled}`; `config.cloud_enabled` публикуется при каждом старте `core` | C-06 v1.1, C-08 v1.2 (З-3) |
| v0.2.1 §2.3.10 | `validation_status` — единый enum из 6 значений; опц. `reasons[]` | C-07 v1.2, ADR-017 доп. 1 (TL2-1) |
| v0.2.1 §2.3.5, §2.3.14 | издатели `dice.rolled` по `Spec.Publishers`; `end_reason` + `forget` | contracts §0 v0.4, C-10 v1.1 (TL2-2, З-1) |
| §0, §2.1 | Сквозные поля `schema_version`, `correlation_id`, `causation_id`/`causation_type`, `actor_kind`, `agent{…}`, `replay`, `locale`, `gm_path` перенесены из `payload` в конверт `meta`; `payload.trigger{event}` не используется; примеры обновлены | ADR-007 п. 1; C-01; consolidation §9 |
| §2.0 (новый) | Интерфейс шины и журнала: `Bus`, `Journal{ReadRange, Tail, End}`, `Position`/`PositionFromContext`, `Dedup`, валидация при чтении, политики топиков | C-01 v1.1 (S-1, G-9, T-10) |
| §1.1, §1.6 | Новые коды `403 client_unknown`, `403 client_mismatch`, `409 no_open_round`, `409 poll_in_progress`, `413 payload_too_large`, `501 not_implemented`; `{client_id}` пути == `X-Client-Id`; лимиты: 30 действий/мин на `player_id` (burst 5), 20 команд/мин в боте, тело ≤ 64 КиБ, один long-poll на `client_id` | C-08 v1.1 (G-1, T-6, T-12) |
| §1.2, §1.3 | `character_status ∈ none\|creating\|alive\|dead`; идемпотентность `POST /v1/characters` — по `(link_id, action_key)`, не по внешнему ID; allowlist Telegram user id и только личные чаты — предусловие регистрации (сторона бота) | C-08 v1.1 (G-3, T-2), ADR-009 дополнение п. 1–2 |
| §1.4 | `202 {status: "pending", group_id, correlation_id}` для `group.*` при ожидании факта > 2 с; `say` в scope `group` не открывает раунд и не входит в `acted[]`; открывающие действия раунда — `attack\|flee\|defend\|group.leave` | C-08 v1.1 (G-2), C-04 (G-6), ADR-020 п. 2–3 |
| §1.5 | `/stream` в MVP-1 → `501 not_implemented`; параметры long-poll закреплены; `route.external_id` — только клиенту платформы связки | C-08 v1.1 |
| §2.2 | `dice.rolled` → `game_events`; новые топики `llm_records`, `analytics_events`, `dead_letters`; retention 30/90/180; `scope_management` удалён; `config.cloud_enabled`; `group.entered_region` — единственный формат перемещения группы | ADR-007 п. 4 + дополнение п. 1, 6; C-04 |
| §2.3.4 | Семантика C-02 v1.1: `rest` предлагает gateway, `atomic=false` — отказ по сущности, `applied_at` = `timestamp` предложения, причина `duplicate_entity`, `changed[]` для `append` | C-02 v1.1 (S-4) |
| §2.3.5 | Функция seed зафиксирована: `SHA-256(eventID+":"+idx)[:8]` BigEndian, PCG | C-03 |
| §2.3.7, §2.3.9–§2.3.11 | `encounter.started.round{timeout, idle_after_missed}` (опц.); `tick.fired.tick.lod_allowed` обязателен; `agent.spawned.content_hash` (опц.); `llm.output.parse{strategy, recovered}` (опц.), `validation_status=error` + `error.code=yielded`; `narrative.output.narrative_event_id` (опц.) | C-05/C-06/C-07 v1.1 (G-4, W-2, W-4) |
| §2.3.12 | `component ∈ state\|swarm\|gateway`; `snapshot{seq, key}`; `latest.json` — указатель, не снапшот | C-14 v1.1 (S-5, G-5) |
| §3 | Роль `group-narrator` (блупринт и скелет); glob в `trigger.event_name`; валидация модели против `Provider.Models()`, блупринт не выбирает провайдера/облако; `round` блупринта `encounter-*` — источник `encounter.started.round`; `player-gm.level = task` (решено); формат парсера — ADR-015 | C-11 v1.1 (W-1, W-5, T-7), C-05 |
| §4 (новый) | Таблица сверки C-01…C-15 → разделы документа | задача A4 |

---

## 0. Общие соглашения

- Идентификаторы, имена полей, типы событий, коды ошибок — английские, `snake_case`; типы событий — `object.action` в прошедшем времени для фактов (`entity.updated`), `*.proposed` для предложений, `*.requested` для команд.
- Время — ISO-8601 UTC. Деньги — `cost_usd` (число).
- **Сквозные поля живут в конверте `meta`, не в `payload`** (решение ADR-007 п. 1, OQ-A-09 закрыт; C-01). Издатель их не копирует вручную: `eventbus.NewRoot(...)` создаёт корневое событие, `eventbus.Derive(parent, ...)` наследует `meta` от причины. Состав `meta` — §2.1.
- `meta.correlation_id` — `id` корневого события цепочки (действие игрока, `tick.fired`, `round.closed`); у корневого `correlation_id = id`.
- `meta.causation_id`, `meta.causation_type` — `id` и тип непосредственной причины; у корневых отсутствуют. Заменяют `payload.trigger{event}` из v0.1 (**не используется**).
- `meta.actor_kind` ∈ `human | ci | sim | system`; для ходов наследуется от сессии, для тиков и фоновых событий — `system`. Политика топика `player_events`: только `human | ci | sim` и без `meta.agent` (проверяется библиотекой при публикации и чтении).
- `meta.agent{id, level, blueprint, blueprint_version}` — обязателен во всех событиях, изданных рантаймом роя (`tick.*`, `agent.*`, `llm.*`, `narrative.output`, `combat.decided`, `encounter.*`, `world.*`/`region.*`/`npc.*` от агентов, `entity.*.proposed` от агентов); `level` ∈ `global | domain | task | object | monitor` (строки `AgentLevel.String()`).
- `meta.schema_version` (int, с 1) — версия схемы типа; совместимые изменения (новое опциональное поле) версию не меняют, несовместимые — `+1`, потребители принимают `n` и `n-1`.
- `meta.replay` (bool) — `true` только у событий, прочитанных из журнала в режиме replay и проброшенных потребителям в тестовом режиме (не переиздаются); в живом режиме `false`.
- `meta.locale` (`ru`), `meta.gm_path` (`agent | legacy`) — наследуются по цепочке.
- Доменные поля остаются в `payload`: `laws_version` (в `llm.output`, `world.laws.*`, `snapshot.created`, `narrative.output`), `phase`, `round`, `encounter`, `entity`, `target`, `scope`-специфика и т. д.

---

## 1. HTTP API gateway (эталонный клиент — бот) — контракт C-08

Спецификация — `api/gateway.openapi.yaml` (OpenAPI 3.1) = этот раздел + дополнения C-08 v1.1; раздел `admin` в том же файле — прокси к HTTP-серверу процесса `core` (C-06). Владелец — EPIC-004.

### 1.1. Общие правила

| Аспект | Правило |
|---|---|
| Базовый путь | `/v1` (as-is эндпоинты без версии — см. §1.9) |
| Формат | JSON UTF-8; `Content-Type: application/json`; тело запроса ≤ 64 КиБ, иначе `413 payload_too_large` (SEC-11) |
| Авторизация | MVP-1: доверенный контур (порт gateway привязан к `127.0.0.1`, ADR-009 п. 9). Заголовок `X-Client-Id` (идентификатор экземпляра клиента: `telegram-bot`, `ci-harness`) — обязателен; значение должно входить в список `MV_GATEWAY_CLIENTS`, иначе `403 client_unknown`. В путях `/v1/clients/{client_id}/…` значение `{client_id}` обязано совпадать с `X-Client-Id`, иначе `403 client_mismatch` (SEC-12). Целевое: токен клиента (E-H) |
| `actor_kind` | заголовок `X-Actor-Kind: human\|ci\|sim` (по умолчанию `human`); `ci`/`sim` разрешены только клиентам из списка конфигурации, иначе `403 actor_kind_forbidden`; служебные эндпоинты §1.8 — только `ci`/operator |
| Предусловие регистрации (сторона бота) | Бот принимает команды **только из личных чатов** и **только от Telegram user id из allowlist** `MV_TELEGRAM_ALLOWED_USER_IDS`; проверка выполняется до любого вызова gateway (ADR-006 дополнение п. 1–2, ADR-009 дополнение п. 1). Групповые чаты и не-allowlist аккаунты gateway не видит вовсе. Инвайт-коды — E-H |
| Идемпотентность действий | `POST …/actions` принимает `action_key` (строка ≤ 64, уникальна в пределах `player_id`, TTL 24 ч); повтор с тем же ключом → тот же ответ (включая тот же `4xx`, если был) и тот же `correlation_id`, второго хода нет; ключ отличается, тело совпадает — новый ход |
| Идемпотентность создания персонажа | `POST /v1/characters` — ключ `(link_id, action_key)`, где `link_id` — суррогат связки (ULID), присваиваемый при `POST /v1/links/resolve` и хранимый **только** в `links.db`; внешний ID в ключе идемпотентности и в `gateway.db` не используется (SEC-03, ADR-009 дополнение п. 2, C-08 v1.1). Клиенту `link_id` не возвращается — не нужен |
| Rate limit | `POST …/actions`: 30 действий/мин на `player_id` (token bucket `MV_GATEWAY_RATE_ACTIONS_PER_MIN=30`, burst 5) → `429 rate_limited` + `Retry-After`; один активный long-poll на `client_id` (второй → `409 poll_in_progress`). Бот дополнительно ограничивает 20 команд/мин на Telegram user id в своей памяти (до вызова gateway) |
| Ошибки | `4xx/5xx` с телом `{ "error": { "code": "<snake_case>", "message": "<ru текст для игрока>", "details": {…} } }`; коды — §1.6 |
| Синхронность | действия принимаются `202 Accepted` после валидации (≤ 300 мс p95, NFR-003); результаты приходят доставками (§1.5). Регистрация/статус — синхронно; `group.*` — синхронно с ожиданием факта ≤ `MV_GATEWAY_FACT_WAIT` (2 с), иначе `202 pending` (§1.4) |
| Приватность | внешний ID допускается только в §1.2 и §1.3 (создание персонажа); в остальных запросах — `player_id`. Сервер не логирует тела запросов §1.2/§1.3 |
| Версионирование | путь `/v1`; несовместимые изменения — `/v2`; поля добавляются совместимо |

### 1.2. Связки и согласие

#### `POST /v1/links/resolve` — разрешить внешний аккаунт в `player_id`
Запрос: `{ "external_platform": "telegram", "external_id": "<string>" }`
Ответ `200`: `{ "link_status": "none|pending_consent|consented", "player_id": "player-A|null", "world_id": "…|null", "character_status": "none|creating|alive|dead", "notice_due": true|false }` (`notice_due` — показать уведомление повторно: `/help` или ≥ 30 дней с `last_seen_at`; сервер обновляет `last_seen_at`). `character_status=creating` — между `entity.create.proposed` и `entity.created` (C-08 v1.1); `none` — персонажа нет (в v0.1 — `null`).
Побочный эффект: при первом обращении связке присваивается `link_id` (ULID) — внутренний ключ идемпотентности создания персонажа (§1.1); в ответ не входит.
Ошибки: `400 invalid_request`. Идемпотентно. Связь: UC-001, UC-003.

#### `POST /v1/links/consent` — зафиксировать уведомление, согласие, 18+
Запрос: `{ "external_platform", "external_id", "notice_shown": true, "consent": true, "age_confirmed": true, "shown_at": "<ts>" }`
Ответ `200`: `{ "link_status": "consented", "consent_at": "<ts>", "age_confirmed_at": "<ts>", "notice_shown_at": "<ts>" }`
Ошибки: `400 consent_incomplete` (любой из трёх флагов не `true` — запись `pending_consent`, ничего не создаётся). Идемпотентно (повтор обновляет `last_seen_at`). Связь: UC-001.

#### `DELETE /v1/links` — удалить связку (`/forget`)
Запрос: `{ "external_platform", "external_id" }`
Ответ `200`: `{ "deleted": true, "player_id_detached": "player-A|null" }`; если связки нет — `200 { "deleted": false }` («нечего удалять»).
Побочные эффекты (порядок — C-04 v1.1, сведение 3): outbox игрока очищен (`pending → dropped`); если персонаж `alive` — один `entity.update.proposed atomic=true cause=forget` (игрок `set status=abandoned` + при лидерстве группа `set leader_id`), затем `group.left {cause: forget}` и при необходимости `group.leader_changed {cause: forget, leader: …|null}`; активная сессия закрыта `end_reason=forget`; после этого связка удаляется физически. Персонаж `dead` — предложение не публикуется (терминальный статус); `creating` — предложение при получении `entity.created` без связки. Связь: UC-031, FR-061.

#### `DELETE /v1/admin/links/{player_id}` — удаление оператором
Ответ как выше. Только `X-Client-Id` из списка операторских клиентов.

### 1.3. Миры и персонажи

#### `GET /v1/worlds` — список миров
Ответ `200`: `{ "worlds": [ { "world_id": "dark-forest-world", "name": "…", "regions": [ { "region_id": "dark-forest-01", "name": "Тёмный лес" } ], "laws_version": "v1", "llm": { "cloud_enabled": false } } ] }`. **`llm.cloud_enabled`** (C-08 v1.2, сведение 3, З-3) — проекция gateway последнего `config.cloud_enabled.enabled` из `system_events` (по умолчанию `false`); имя провайдера, URL и ключи не передаются (SEC-21). Бот кэширует флаг (`MV_TELEGRAM_CLOUD_FLAG_TTL=60s`; обновление на `/start`, `/help`, по TTL) и при `true` показывает отдельное уведомление один раз за сессию диалога до обработки команды (US-008). Связь: UC-002, FR-006, US-008.

#### `POST /v1/characters` — создать персонажа
Запрос: `{ "external_platform", "external_id", "world_id", "character_name", "action_key" }`
Ответ `201`: `{ "player_id": "player-A", "character": <CharacterState>, "created": true }`
`200`: живой персонаж уже есть → `{ "player_id", "character", "created": false }`.
`202`: создание принято, факт ещё не подтверждён → `{ "player_id", "status": "creating" }` (бот опрашивает `GET /v1/players/{id}` либо `POST /v1/links/resolve` → `character_status`).
Идемпотентность: по `(link_id, action_key)` — gateway разрешает `external_platform + external_id` в `link_id` через `links.db`, затем ищет запись `character_requests` (тоже в `links.db`, каскадно удаляется при `/forget`); повтор → тот же ответ и тот же `player_id`. В `gateway.db` (`pending_characters`, `idempotency_keys`, `turns`…) внешний ID не попадает ни в одно поле (SEC-03).
Ошибки: `404 world_not_found`; `403 consent_required`; `400 name_required` / `name_invalid` (2–32 символа; буквы, цифры, пробел, дефис; без управляющих); `409 character_alive_exists` (не используется — возвращается `200`, оставлен для строгого режима). Поведение при `dead` — новый `player_id` (UC-002 A2). Связь: UC-002.

#### `GET /v1/players/{player_id}` — состояние персонажа (`status`)
Ответ `200`: `CharacterState`:
```json
{
  "player_id": "player-A", "name": "Вася", "world_id": "dark-forest-world",
  "status": "alive", "hp": 7, "hp_max": 10,
  "position": { "kind": "region", "id": "dark-forest-01", "name": "Тёмный лес" },
  "scope": { "id": "group:g-1", "type": "group" },
  "group": { "group_id": "g-1", "leader_id": "player-A", "members": [ { "player_id": "player-A", "name": "Вася", "participation": "active" } ] },
  "encounter": { "encounter_id": "enc-7", "npcs": [ { "npc_id": "wolf-alpha", "name": "Альфа-волк", "hp": 4, "hp_max": 10, "status": "alive" } ], "my_state": "in_combat", "round_seq": 3 },
  "inventory": [ { "item_id": "item-1", "kind": "wolf-pelt", "name": "волчья шкура" } ],
  "world": { "weather": "rain", "time_of_day": "night", "day": 12 },
  "session": { "session_id": "solo:player-A:1757400000", "turns_count": 9 },
  "version": 23
}
```
Не ход; событий не порождает. Пока факт `entity.created` не получен — `200` с `status: "creating"` и только полями `player_id`, `name`, `world_id`, `status` (остальные отсутствуют). `status ∈ creating|alive|dead|abandoned` (`abandoned` — после `/forget`, FR-061; сведение 3); `group.leader_id` — `string | null` (группа без живых участников, З-4). Ошибки: `404 player_not_found`. Связь: UC-003.

### 1.4. Действия

#### `POST /v1/players/{player_id}/actions` — отправить действие
Запрос:
```json
{ "action_key": "<uuid>", "type": "attack", "target": "wolf-alpha", "text": null, "client_ts": "<ts>" }
```
Словарь `type` и правила валидации (все проверки — до Phase 1, без LLM):

| `type` | `target` | `text` | Предусловия | Ход? | Phase 1 |
|---|---|---|---|---|---|
| `enter` | `region_id` обяз. | — | `alive`; вне встречи; `solo` или лидер группы; регион в мире | да | позиция |
| `leave` | — | — | `alive`; в регионе; вне встречи; `solo` или лидер | да | позиция |
| `look` | — | — | `alive` | да | нет |
| `attack` | `npc_id` обяз. | — | `alive`; во встрече; цель жива и в этой встрече; в группе — не действовал в открытом раунде | да | бой |
| `flee` | — | — | `alive`; во встрече | да | бегство |
| `rest` | — | — | `alive`; вне встречи | да | HP (`entity.update.proposed cause=rest, set hp=hp_max` публикует gateway; State отклоняет при активной встрече — C-02 v1.1) |
| `say` | — | 1–500 символов после trim | `alive` | да; **в scope `group` не открывает раунд и не входит в `acted[]`**, доставляется сразу (C-04, UC-016 E2; BA закрепляет в FR-025) | нет |
| `defend` | — | — | `alive`; во встрече (явное «пропустить») | да | нет (цель для NPC) |
| `group.create` | — | — | `alive`; не в группе | нет (служебное) | группа |
| `group.join` | `group_id` | — | `alive`; не в группе; группа ≤ 5; группа вне встречи | нет | группа |
| `group.leave` | — | — | в группе; во встрече → исполняется как `flee` | нет / да (как flee) | группа |

Раунд в scope `group` (ADR-020, C-04): открывающие действия — `attack | flee | defend | group.leave`; раунд 1 открывается gateway по `encounter.started`, раунд N+1 — первым открывающим действием после `round.closed`; второе действие того же участника в открытом раунде → `409 already_acted`; в `solo` раунд = ход, `round.*` не публикуются.

Ответ `202`:
```json
{ "correlation_id": "<event_id действия>", "turn": { "seq": 10, "session_id": "…", "round_seq": 3 }, "status": "accepted", "acked_at": "<ts>" }
```
Для `group.*` — `200` с составом группы: `{ "group_id", "leader_id", "members": [...], "position" }`, если факт `entity.created`/`entity.updated` получен в пределах `MV_GATEWAY_FACT_WAIT` (2 с); иначе `202 { "status": "pending", "group_id": "g-1", "correlation_id": "…" }` — клиент опрашивает `GET /v1/groups/{group_id}` (C-08 v1.1).
Ошибки (`4xx`, `status=rejected`, `analytics.turn.completed status=rejected`): см. §1.6. Повтор `action_key` → `202` с тем же `correlation_id` (или тот же `4xx`, что был). `503 bus_unavailable` — действие не принято, ключ не записан.
Связь: UC-004…UC-017.

### 1.5. Доставка результатов клиенту

Контракт сообщения один; транспорт MVP-1 — long-poll (ADR-006, C-08); WebSocket зарезервирован. `{client_id}` во всех путях ниже обязан совпадать с `X-Client-Id` (`403 client_mismatch`).

#### `GET /v1/clients/{client_id}/deliveries?after=<cursor>&limit=100&wait_ms=25000` — long-poll outbox
Ответ `200`: `{ "deliveries": [ <Delivery> ], "cursor": "<opaque>" }`. Ограничения: `wait_ms ≤ 25000`, `limit ≤ 100`; один активный long-poll на `client_id` (второй → `409 poll_in_progress`). Клиент подтверждает: `POST /v1/clients/{client_id}/deliveries/ack { "ids": [...] }`; без ack — повтор через 30 с; доставки одному `player_id` строго по порядку (одна в лизинге).

#### `GET /v1/clients/{client_id}/stream` — push (WebSocket) тех же `Delivery`; в MVP-1 отвечает `501 not_implemented` (маршрут зарезервирован, ack тем же эндпоинтом при реализации в E-H).

`Delivery`:
```json
{
  "id": "d-123", "player_id": "player-B",
  "route": { "external_platform": "telegram", "external_id": "<только клиенту той же платформы>" },
  "kind": "mechanics", 
  "correlation_id": "…", "event_id": "…", "round_seq": 3,
  "generated_by": "rules",
  "text": "Попадание! Урон 3. Волк: 4/10.",
  "data": { "combat": { "attacker": "player-A", "defender": "wolf-alpha", "hit": true, "damage": 3, "hp_after": 4 } },
  "created_at": "<ts>"
}
```
`kind` ∈ `ack | mechanics | narrative | world_event | group | system`; `generated_by` ∈ `rules | llm | template`; для `template` — `fallback_reason` и пометка «упрощённый режим» в тексте; для `narrative` — `narrative_event_id` одинаков у всех адресатов раунда (берётся из `narrative.output.narrative_event_id`, при его отсутствии — `id` события). Порядок доставки одному `player_id` — строго по `created_at`. `route.external_id` заполняется из `links.db` в момент выдачи и **только клиенту той же платформы, что связка** (`external_platform` == платформа клиента по `MV_GATEWAY_CLIENTS`); в outbox (`gateway.db`) хранится лишь `player_id` и `platform`; после `/forget` доставки отбрасываются. Текст — plain text без разметки (бот отправляет без `parse_mode`, SEC-10).

### 1.6. Коды ошибок

| HTTP | `code` | Когда |
|---|---|---|
| 400 | `invalid_request` | тело/поля не по схеме |
| 400 | `unknown_action` | `type` вне словаря |
| 400 | `text_invalid` | `say` пустой/длиннее 500 |
| 400 | `name_required`, `name_invalid` | имя персонажа |
| 403 | `consent_required` | связка не `consented` |
| 403 | `actor_kind_forbidden` | `ci`/`sim` от неразрешённого клиента; служебный эндпоинт от не-`ci` |
| 403 | `client_unknown` | `X-Client-Id` отсутствует или не входит в `MV_GATEWAY_CLIENTS` (C-08 v1.1) |
| 403 | `client_mismatch` | `{client_id}` пути ≠ `X-Client-Id` (SEC-12, C-08 v1.1) |
| 404 | `player_not_found`, `world_not_found` | |
| 404 | `unknown_target` | регион/NPC/группа не существует или не в scope |
| 409 | `character_dead` | действия от `dead` или `abandoned` (сведение 3) |
| 409 | `in_encounter` | `enter/leave/rest/join` во встрече |
| 409 | `not_in_encounter` | `attack/flee/defend` без встречи |
| 409 | `target_dead` | атака мёртвой цели (соло) |
| 409 | `no_leader` | `enter/leave` группы при `leader_id = null` (живых участников нет, BR-13; проверяется раньше `not_leader`; C-08 v1.2, сведение 3) |
| 409 | `not_leader` | `enter/leave` от не-лидера |
| 409 | `not_in_region` | `leave` с опушки |
| 409 | `already_acted` | второе действие в открытом раунде |
| 409 | `no_open_round` | `POST /v1/scopes/{scope_id}/rounds/close` без открытого раунда (C-08 v1.1) |
| 409 | `poll_in_progress` | второй одновременный long-poll на тот же `client_id` (C-08 v1.1) |
| 409 | `already_in_group`, `not_in_group`, `group_full`, `group_in_encounter` | группа |
| 409 | `encounter_unavailable` | Task-агент встречи не готов (временно) |
| 413 | `payload_too_large` | тело > 64 КиБ (SEC-11) |
| 422 | `filter_error` | `InputFilter` упал (fail-closed) |
| 429 | `rate_limited` | 30 действий/мин на `player_id` (burst 5); ответ с `Retry-After` |
| 501 | `not_implemented` | `GET /v1/clients/{client_id}/stream` в MVP-1 |
| 503 | `bus_unavailable`, `state_unavailable` | зависимость недоступна — действие **не** принято, `action_key` не записан |

### 1.7. Группы (чтение)
`GET /v1/groups/{group_id}` → состав, лидер, позиция, встреча (`GroupView{group_id, leader_id: string | null, members[]{player_id, name, participation, status}, position, encounter?, state}`; `leader_id = null` — живых участников нет, сведение 3 З-4). Связь: UC-014.

### 1.8. Служебные эндпоинты (тест-харнесс, оператор)

| Метод | Путь | Кто | Назначение |
|---|---|---|---|
| `POST` | `/v1/scopes/{scope_id}/rounds/close` | `ci` | явное закрытие раунда → `round.closed close_reason=explicit`; нет открытого раунда → `409 no_open_round` |
| `POST` | `/v1/admin/agents/{agent_id}/tick` | `ci`, оператор | немедленный тик (ускорение фона) → обычный `tick.fired mode=background`, `meta.actor_kind=ci` |
| `GET` | `/v1/admin/agents` | оператор | активные агенты по уровням (`agents_active_by_level`) |
| `GET` | `/v1/admin/sessions` | оператор | активные сессии (реализует gateway) |
| `GET` | `/v1/admin/llm/usage?since=` | оператор | вызовы/токены по фазам/уровням/провайдерам (Should; MVP-1 — через CLI) |
| `GET` | `/health` | все | `{ "status": "ok|degraded|fail", "deps": { "bus": "ok", "state_store": "ok", "llm": "unavailable", "links_store": "ok" }, "agents_by_level": {...}, "snapshot": "ok|corrupted|missing" }` |

Решение (C-06, D-7): маршруты `/v1/admin/agents*` и `/v1/admin/llm/usage` реализуют контексты `swarm`/`llm` на HTTP-сервере процесса `core` (`shared/runtime`, `MV_CORE_ADDR=127.0.0.1:8090`; там же `/health` процесса); gateway проксирует `/v1/admin/*` → `MV_CORE_URL` для единого входа харнесса; доступ — только клиенты `ci`/operator. Спецификация admin-маршрутов — раздел `admin` в `api/gateway.openapi.yaml`.

### 1.9. Соответствие as-is → to-be

| As-is (`services/game-service`) | To-be |
|---|---|
| `POST /players/register {player_id, player_name, world_id}` — клиент задаёт `player_id`, пишет в MinIO напрямую, публикует `entity.created` | `POST /v1/characters` — `player_id = "player-" + ULID` генерирует gateway; создание через `entity.create.proposed` → State; идемпотентность по `(link_id, action_key)` |
| `POST /players/login` | `POST /v1/links/resolve` + `GET /v1/players/{id}` |
| `GET /entities/{id}?world_id=` | `GET /v1/players/{id}` (игроку — только своё); сущности — не публичный API |
| `GET /entities/{id}/history`, `GET /events/recent` (TODO) | не в клиентском API; трасса — CLI/`/v1/admin/trace/{correlation_id}` (Should) |
| `GET /run_test` (публикация тестовых событий) | удалить; тест-харнесс работает через §1.4/§1.8 |
| `WS /ws/entities`, `/ws/events`, `/ws/actions` — широковещание всем клиентам | `GET /v1/clients/{id}/stream` — адресно; приём действий только через `POST …/actions` |
| `player.moved`, `player.used_skill` с плоским payload | события §2.3.1 с иерархическим payload |

---

## 2. Контракты событий

### 2.0. Шина и журнал как интерфейс (C-01 v1.1, владелец EPIC-001)

Логический контракт библиотеки `shared/eventbus`, на который опираются все издатели и потребители ниже (детали Go-API — `contracts.md` C-01):

| Элемент | Назначение | Гарантии |
|---|---|---|
| `Bus{Publish, Subscribe, Close}` | публикация по типу (топик выбирает реестр `shared/contracts`), «живые» подписки с consumer group `{process}.{context}` | at-least-once; порядок внутри топика (одна партиция); неизвестный/невалидный тип → ошибка публикации; ошибка обработчика → повтор ×3 (100/500/2000 мс) → `dead_letters` |
| `Journal{ReadRange(topic, from, to), Tail(topic, from), End(topic)}` | чтение по офсетам без consumer group: догон с курсора снапшота (State — свой курсор; Swarm и Gateway — от `cursor` снапшота, C-14); `End` = офсет следующего сообщения (high watermark) | `ReadRange` — строго по возрастанию офсета; `End` монотонен; «конец журнала» для выхода из `replay` в `live` = `позиция ≥ End − 1` по всем читаемым топикам; отдельного `Bus.Lag()` нет |
| `Position{Topic, Offset}`, `PositionFromContext(ctx)` | адаптер кладёт позицию текущего события в контекст перед вызовом обработчика (и в `Subscribe`, и в `Journal`) — для курсоров снапшотов | одна семантика в kafka-адаптере и `membus` |
| `Dedup` (LRU по `event.id`, по умолчанию 10 000; `Seen(id)`) | дедуп у потребителя; сериализуется в снапшот потребителя | обязанность потребителя, не шины |
| Валидация при чтении | `Subscribe` валидирует конверт и payload по JSON Schema 2020-12 (`MV_BUS_VALIDATE_ON_READ`, по умолчанию `true`); невалидное → `dead_letters` без вызова обработчика | защита в глубину при PLAINTEXT-шине (SEC-16) |
| Политики топиков (`Spec.Policy`) | `player_events`: только `meta.actor_kind ∈ human\|ci\|sim`, `meta.agent == nil`; `llm_records`, `system_events(tick.*, agent.*)`, `narrative_output`, `combat.decided`: `meta.agent` обязателен | проверяется при публикации и при чтении |
| Часы | `shared/clock.Clock{Now}`, `Timers{After, Every}`; `time.Now()` в `internal/*` запрещён; в replay — `EventClock`/`NullTimers` | таймеры в replay не срабатывают (NFR-061) |

Заглушка: `shared/testkit/membus` — in-memory шина **и журнал** с той же семантикой (порядок, офсеты, `End`, дубли по флагу `--chaos=duplicate`).

### 2.1. Конверт события и `meta`

Конверт — `shared/eventbus.Event` (обратно совместим по JSON: новые поля добавляются): `id, type, timestamp, source, world{entity{id,type:world}}, scope{id,type}, meta{…}, payload, relations[]`. Ключ сообщения — `world.entity.id` (`global` для событий без мира). Решение ADR-007 п. 1: **сквозные поля — в `meta`, payload — только доменные поля по схеме типа**.

```json
{ "id": "uuid", "type": "combat.decided", "timestamp": "2026-09-09T12:00:00Z", "source": "core/swarm",
  "world": { "entity": { "id": "dark-forest-world", "type": "world" } },
  "scope": { "id": "solo:player-A", "type": "solo" },
  "meta": { "schema_version": 1,
            "correlation_id": "<id корня: действие игрока | tick.fired | round.closed>",
            "causation_id": "<id непосредственной причины>", "causation_type": "player.attacked",
            "actor_kind": "human",
            "agent": { "id": "encounter-wolf:solo:player-A", "level": "task", "blueprint": "encounter-wolf", "blueprint_version": "1.0" },
            "replay": false, "locale": "ru", "gm_path": "agent" },
  "payload": { "…доменные поля по схеме типа (§2.3)…" }, "relations": [] }
```

| Поле `meta` | Тип | Обяз. | Описание |
|---|---|---|---|
| `schema_version` | int | да | версия схемы типа (с 1); `+1` при несовместимом изменении |
| `correlation_id` | string | да | `id` корневого события цепочки; у корневого = `id` |
| `causation_id`, `causation_type` | string | нет (у корневых отсутствуют) | непосредственная причина; заменяют `payload.trigger{event}` v0.1 |
| `actor_kind` | enum | да | `human, ci, sim, system` |
| `agent{id,level,blueprint,blueprint_version}` | obj | у событий рантайма роя | политика топиков §2.0; в `player_events` запрещён |
| `replay` | bool | да | `true` только у проброшенных из журнала в тестовом режиме |
| `locale` | string | да | `ru` |
| `gm_path` | enum | да | `agent, legacy` |

Библиотека: `eventbus.NewRoot(type, source, worldID, scope, actorKind, payload)` — корневое событие; `eventbus.Derive(parent, type, source, payload, opts…)` копирует `correlation_id`, `actor_kind`, `locale`, `gm_path`, ставит `causation_id = parent.id`, `causation_type = parent.type`; `WithAgent(AgentRef)`. `Event.CorrelationID()`, `Event.Path()`, `eventbus.GetWorldIDFromEvent`, `GetScopeFromEvent` — без изменений.

Доменные поля `payload`, общие для многих типов (по схеме конкретного типа): `entity{entity{id,type},name}` — основная сущность; `target{entity{id,type},name}` — цель; `laws_version`; `encounter{entity}`; `round{seq}`. Дублировать `world`/`scope` в payload не требуется (хелперы читают конверт); fallback на плоские поля (`entity_id`, `player_id`) — только для legacy-типов (`gm_path=legacy`) на время миграции. Общие определения схем — `schemas/events/_common.json` (`EntityRef`, `ScopeRef`, `AgentRef`, `Money`).

### 2.2. Карта «событие → топик → издатель → потребители» (MVP-1)

Топики (ADR-007 п. 4, дополнение п. 1–2; создаёт `redpanda-init`, все с `segment.ms=1d`, 1 партиция): `PE` = `player_events`, `WE` = `world_events`, `GE` = `game_events`, `SE` = `system_events`, `NO` = `narrative_output`, `LR` = `llm_records`, `AE` = `analytics_events`, `DL` = `dead_letters`. `scope_management` и фантомные топики **удалены**.

| Топик | Типы | Retention (решение пользователя U-3) | Читают в replay |
|---|---|---|---|
| `player_events` | `player.*`, `group.entered_region`, `group.left_region` | 30 дн. | да |
| `game_events` | `group.*` (кроме перемещений), `round.*`, `combat.decided`, `dice.rolled` | 30 дн. | да |
| `world_events` | `world.*`, `region.*`, `npc.*`, `encounter.*`, `world.laws.*`, `world.law_breach.*` | 30 дн. | да |
| `system_events` | `entity.*`, `tick.*`, `agent.*`, `snapshot.created`, `content.incident.recorded`, `config.*` | 30 дн. | да |
| `narrative_output` | `narrative.output` | 30 дн. | да (доставка) |
| `llm_records` | `llm.output`, `llm.output.rejected` (`max.message.bytes=4 МиБ`) | 90 дн. | да (record-replay) |
| `analytics_events` | `analytics.*` | 180 дн. | **нет** |
| `dead_letters` | любой, обёрнут `{original, error, consumer}` | 30 дн. | нет |

Издатели в таблице ниже — в именах v0.2 (gateway / State / Swarm / LLM-шлюз / механика; см. шапку). Столбец «As-is» — разрыв из `audit-facts.md` §5.

| Тип | Топик | Издатель | Потребители | v | As-is |
|---|---|---|---|---|---|
| `player.entered_region` | PE | gateway (только `solo`) | GM региона, персональный GM, memory, страж | 1 | есть в блупринте (`domain-dark-forest`), издателя нет |
| `player.left_region` | PE | gateway (только `solo`) | GM региона, персональный GM, memory | 1 | нет |
| `player.looked` | PE | gateway | персональный GM, memory | 1 | `player.looked_around` только в тест-харнессе |
| `player.attacked` | PE | gateway | Task-агент встречи, персональный GM, memory | 1 | `player.used_skill` (legacy) |
| `player.flee_attempted` | PE | gateway | Task-агент, персональный GM | 1 | нет |
| `player.rested` | PE | gateway | персональный GM (сопутствующее `entity.update.proposed cause=rest` — тоже gateway, C-02 v1.1) | 1 | нет |
| `player.said` | PE | gateway | персональный GM / `group-narrator`, memory | 1 | нет |
| `player.defended` | PE | gateway (`cause: player\|round_timeout`; публикуется **до** `round.closed`) | Task-агент, memory, аналитика | 1 | нет |
| `group.created`, `group.joined`, `group.left`, `group.leader_changed`, `group.disbanded` | GE | gateway | GM региона, персональные GM, `group-narrator` (спавн по `group.created`), memory | 1 | `gm.merged/split` (legacy, заменяются) |
| `group.entered_region`, `group.left_region` | PE | gateway — **единственный формат перемещения группы** (`player.entered_region` для участников не издаётся; C-04, ADR-007) | как `player.entered_region` | 1 | нет |
| `round.opened`, `round.closed` | GE | gateway (координатор раундов, ADR-020) | Task-агент, `group-narrator`, рантайм роя (replay) | 1 | нет |
| `entity.create.proposed`, `entity.update.proposed` | SE | gateway (player, group, position/scope/group/participation, `rest`), агент встречи (из `mechanics.ChangesFor`), агенты роя | State | 1 | нет (game-service пишет в MinIO напрямую) |
| `entity.created`, `entity.updated`, `entity.update.rejected` | SE | State | gateway (проекция), агенты роя, memory, страж | 1 | `entity.created` публикуют game-service/world-generator; entity-manager **не публикует** |
| `dice.rolled` | **GE** | агент встречи через `mechanics.DiceRolledPayload` (тип принадлежит EPIC-002); публикуется **до** `combat.decided` | replay, аудит, `session-report` | 1 | нет |
| `combat.decided` | GE | агент встречи (Swarm) на основе `mechanics.Resolve` | State (через `entity.update.proposed`), персональный GM / `group-narrator`, gateway (`Delivery kind=mechanics`), memory | 1 | `combat.started/ended/damage_dealt` — подписчики без издателя |
| `encounter.started`, `encounter.ended` | WE | GM региона (`started`) / агент встречи (`ended`) | персональные GM, gateway (read-model `encounter`, первый раунд группы), глобальный GM (сводка), memory | 1 | нет |
| `world.weather_changed`, `world.time_advanced`, `world.event_occurred` | WE | глобальный GM | GM регионов, персональные GM, memory | 1 | `world.weather_changed`, `world.time_tick` — подписчики без издателя |
| `region.event_occurred`, `npc.moved`, `npc.spawned` | WE | GM региона | глобальный GM, персональные GM, memory | 1 | `npc.action`, `npc.moved` — без издателя/контракта |
| `tick.fired`, `tick.aborted` | SE | планировщик роя | агент-адресат, replay, `session-report` | 1 | `time.syncTime` (legacy narrative-orchestrator) |
| `agent.spawned`, `agent.child_resolved`, `agent.stopped`, `agent.spawn_rejected`, `agent.blueprint_reloaded` | SE | рантайм роя | оператор/health, `session-report`, memory | 1 | `gm.created/deleted/merged/split` (legacy) |
| `llm.output`, `llm.output.rejected` | LR | LLM-шлюз (middleware записи) | replay (`recorded`-провайдер), страж (аудит), `session-report`, memory (только метаданные) | 1 | нет |
| `narrative.output` | NO | персональный GM (`solo`) / `group-narrator` (`group`) | gateway (доставка), memory | 1 | `narrative.generate` (legacy; game-service не обрабатывает) |
| `content.incident.recorded` | SE | LLM-шлюз (фильтр (a)) | оператор, `session-report` | 1 | нет |
| `snapshot.created` | SE | State (`component=state`), Swarm (`swarm`), gateway (`gateway`) | оператор, `session-report --audit`, потребители при старте (через `latest.json`) | 1 | нет |
| `config.cloud_enabled` | SE | LLM-шлюз (при включении облака `MV_LLM_CLOUD_ENABLED=true`) | оператор, `session-report`, аудит | 1 | нет (новый, ADR-005 дополнение п. 3) |
| `world.laws.changed` | WE | автор (CLI `mvctl laws bump`) / механика пробоя (E-B) | все агенты, State, memory | 1 | нет (регистрация без обработчиков) |
| `world.law_breach.proposed / rejected / applied / review_decided / rolled_back` | WE | E-B | E-B | 1 | нет (регистрация без обработчиков; `mvctl contracts check` знает исключение) |
| `analytics.session.started / ended`, `analytics.turn.completed` | AE | gateway | `session-report` | 1 | нет |
| `analytics.consistency.violated` | AE | тест-харнесс, `session-report --audit`, страж (целевое) | `session-report` | 1 | `violation.detected` (ban-of-world, другой смысл) |
| `analytics.replay.completed` | AE | State (`mode=recovery`) / `mvctl replay` (`mode=test`) — одна схема, совладение EPIC-002/005 (C-14) | `session-report`, CI | 1 | нет |
| `dead_letters` (обёртка `{original, error, consumer}`) | DL | библиотека шины (после 3 повторов или невалидного события при чтении) | оператор, `session-report --audit` | 1 | нет |

Замкнутость: у каждого типа есть издатель и ≥ 1 потребитель; фантомные топики (`entity_actor_events`, `mechanical_results`, `entity.created` как топик, `world.metrics.*`, `reality.anomaly.detected`) в контракте отсутствуют. Legacy-типы (`player.moved`, `player.used_skill`, `gm.*`, `narrative.generate`, `violation.detected`, `time.syncTime`) — в реестре с пометкой `deprecated, gm_path=legacy`, допустимы только в профиле `legacy` (флаг `agent_mode=off`) и удаляются с ним. Правило потребления (C-01 §0): любой блок может **читать** любой топик; **публиковать** тип может только владелец типа.

### 2.3. Спецификации payload

Ниже — доменные поля `payload`; сквозные поля (`correlation_id`, `causation_*`, `actor_kind`, `agent`, `replay`, `schema_version`) — в конверте `meta` (§2.1) и в payload **не дублируются**. Примеры сокращены; `"…"` = прочие доменные поля.

#### 2.3.1. Действия игрока (`player.*`) — издатель gateway; `meta.actor_kind` сессии, `meta.agent` отсутствует (политика `player_events`)
```json
{ "entity": { "entity": { "id": "player-A", "type": "player" }, "name": "Вася" },
  "action": { "type": "attack", "key_hash": "<hash action_key>" },
  "target": { "entity": { "id": "wolf-alpha", "type": "npc" }, "name": "Альфа-волк" },
  "encounter": { "entity": { "id": "enc-7", "type": "encounter" } },
  "round": { "seq": 3 },
  "session": { "id": "…" },
  "text": "…"  // только player.said, ≤ 500, без внешних ID
}
```
Корневые события: `meta.correlation_id = id`, `causation_*` отсутствуют; `scope` — в конверте (`solo:{player_id}` или `group:{group_id}`). `player.defended`: `cause: player | round_timeout` (при `round_timeout` издаёт gateway за молчавшего — корневое событие, `actor_kind` сессии scope; публикуется до `round.closed`). `player.entered_region`/`left_region`: `target{region}`, `position{from, to}` — **только для `solo`**; перемещение группы — исключительно `group.entered_region`/`left_region` (§2.3.2), `cause: group_move` из v0.1 не используется. Инвариант: **ни один агент не публикует `player.*`** (страж: `player_agency`; политика топика — библиотека шины).

#### 2.3.2. Группа (`group.*`) — издатель gateway
`group{entity{id,type:group}}`, `leader{entity{player}} | null`, `members[]{entity{player}, name, participation}`, `cause: create|join|leave|death|forget|disband|group_move` (`forget` — сведение 3, FR-061 п. 3). `group.leader_changed {leader: {entity{player}} | null, cause: leave|death|forget}` — при выходе/смерти/отвязке лидера новый лидер = старейший `alive` по `joined_at` (BR-13 v0.4); **если живых участников нет — `leader: null`** (группа без лидера до `disbanded`; сопутствующее `entity.update.proposed set leader_id = null`; З-4). `group.left {cause: forget}` публикует gateway в каскаде `/forget` после `entity.update.proposed status=abandoned` (C-04 v1.1). `group.entered_region`/`left_region` (топик `player_events`) — **единственный формат перемещения группы**: `target{region}`, `position{from,to}`, `by{entity{player лидер}}`, `members[]` на момент перемещения; сопутствующее `entity.update.proposed atomic=true` (позиция группы и всех `alive` участников).

#### 2.3.3. Раунд (`round.*`) — издатель gateway (координатор раундов, ADR-020); корневые события (`meta.correlation_id = id`, `meta.actor_kind` сессии scope)
`round.opened`: `scope`, `encounter{entity}`, `round{seq}`, `expected[]{entity{player}}` (= `members ∩ alive ∩ participation=active ∩ ¬out_of_combat`), `deadline_at`. Раунд 1 — по `encounter.started` (параметры `round{timeout, idle_after_missed}` из него, §2.3.7), раунд N+1 — первым открывающим действием; публикуется до `202` на открывающее действие.
`round.closed`: `round{seq, close_reason: all_acted|timeout|explicit}`, `acted[]{entity{player}, event{id}}` (по времени приёма), `auto_defended[]{entity{player}}`, `idle[]{entity{player}}`, `closed_at`. Самодостаточно для агента встречи (порядок между топиками не гарантирован — агент не ждёт `player.defended` из `player_events`); seed ответа NPC = `Seed(round.closed.id, idx)`; для `(scope, seq)` публикуется ровно один раз. `replay` — в `meta`. `say` в `acted[]` не входит.

#### 2.3.4. Состояние (`entity.*`) — контракт C-02 (владелец EPIC-002)
`entity.create.proposed`: `entity{entity{id,type},name}`, `attributes{…}` (полный начальный набор по типу), `cause`; `meta.agent` обязателен от агентов; `proposer=system` при загрузке фикстур (`mvctl world init --fixtures`, UC-036).
`entity.update.proposed`:
```json
{ "proposal_id": "<uuid>",
  "changes": [
    { "entity": { "entity": { "id": "wolf-alpha", "type": "npc" } }, "expected_version": 12,
      "ops": [ { "op": "set", "path": "hp", "value": 7 }, { "op": "set", "path": "status", "value": "alive" } ] },
    { "entity": { "entity": { "id": "player-A", "type": "player" } }, "expected_version": 23,
      "ops": [ { "op": "append", "path": "inventory", "value": { "item_id": "item-1", "kind": "wolf-pelt", "name": "волчья шкура", "source": { "entity": { "id": "wolf-alpha", "type": "npc" }, "event_id": "…" } } } ] }
  ],
  "atomic": true,
  "cause": "combat|rest|move|loot|spawn|tick|group|create|bootstrap|forget"
}
```
`meta.agent` (`{ "id": "encounter-wolf:group:g-1", "level": "task", … }`) — в конверте. `ops.op` ∈ `set | inc | append | remove`. Семантика (C-02 v1.1):
- `atomic=true` — все изменения пакета применяются вместе или отклоняются одним `entity.update.rejected` (нужно для инварианта позиции группы); `atomic=false` — отказ по каждой сущности отдельно (по одному `rejected` на сущность), остальные применяются;
- `expected_version` опционален, **но обязателен** для путей `hp`, `status`, `inventory`, `position` игрока/NPC в бою (ADR-013 п. 1); без него — применение к текущей версии с проверкой инвариантов;
- `rest` предлагает **gateway** (`cause=rest`, `set hp = hp_max`); State отклоняет при активной встрече (`law_violation`, `details.invariant_id`);
- повтор предложения с тем же `proposal_id` не применяется второй раз (факты досылаются, если не были опубликованы);
- **`cause=forget` (C-02 v1.2, сведение 3, FR-061)**: только gateway (без `meta.agent`), только `set status=abandoned` для `player` в статусе `alive` (+`expected_version`) и, в том же atomic-пакете, `set leader_id = <id|null>` группы; от агента → `level_violation`; над `dead`/`ascended_final` → `dead_entity`. `abandoned` — терминальный: правило `dead_entity`, inv-01, `NPCTarget` и видимость стража трактуют `status ∈ dead|abandoned|ascended_final` одинаково; факт `entity.updated {changed:[{path: status, old: alive, new: abandoned}], cause: forget}`.

`entity.updated`: `entity{…}`, `version` (новая), `changed[]{path, old, new}`, `cause`, `proposal_id`, `applied_at`. Публикуется по одному на сущность; при пакетном применении — общий `proposal_id`. **`applied_at` и `timestamp` факта = `timestamp` предложения** (время из событий, ADR-003 п. 6). Для `append` путь в `changed[]` — путь элемента (`inventory[n]`), `old` отсутствует. При `changed=[]` версия не меняется, факт публикуется (ход засчитан — UC-011 E2).
`entity.created`: `entity{…}`, `version: 1`, `attributes{…}`.
`entity.update.rejected`: `proposal_id`, `reason: version_conflict | unknown_entity | level_violation | law_violation | invalid_op | dead_entity | duplicate_entity`, `entity?`, `details{expected_version, actual_version, invariant_id}`. `duplicate_entity` — `entity.create.proposed` с уже существующим `id` (v1.1, расширение enum).

Правило владения (`level_violation`) — `data-model.md` §4; State проверяет `meta.agent.level` и `cause` по `contracts.OwnershipRules` (единственный источник — таблица уровней `shared/agent/levels.go`, экспортируемая в реестр при сборке). Факт публикуется после успешной записи в объектное хранилище; латентность применения p95 ≤ 50 мс при ≤ 50 сущностей/scope.

#### 2.3.5. Броски (`dice.rolled`) — топик `game_events`; тип (схема, семантика) принадлежит EPIC-002; издатели по `Spec.Publishers` реестра (`contracts.md` §0 v0.4, сведение 3): агент встречи EPIC-003 через `Derive(cause, "dice.rolled", mechanics.DiceRolledPayload(roll, roller))` **до** `combat.decided`, а также `testkit/swarm.FakeEncounter` (только `membus`/`MV_SWARM_FAKE=true`)
`roll{index, formula, seed, result, natural}`, `purpose: hit|damage|flee|npc_hit|npc_damage|encounter_chance|background`, `roller{entity}`. Причина (`meta.causation_id`) — действие игрока или `round.closed`; `seed = Seed(causeEventID, roll.index) = SHA-256(eventID + ":" + idx)[:8]` BigEndian, RNG — `math/rand/v2` PCG(seed, 0); **на проводе `seed` — десятичная строка** (uint64 не переживает разбор JSON в `map[string]any`: число становится `float64`, и значение выше 2^53 читается другим — измерено на T-015, `state-and-mechanics.md` §5.6) (C-03; зафиксировано архитектором, одна функция на проект). `replay` — в `meta`. Броски вне боя (шанс встречи, таблицы фона) — `Rules.Roll` с тем же контрактом.

#### 2.3.6. Бой (`combat.decided`) — издатель агент встречи (Swarm) по результату `mechanics.Resolve`; `meta.agent.level=task`
```json
{ "encounter": { "entity": { "id": "enc-7", "type": "encounter" } }, "round": { "seq": 3 },
  "action": "attack|flee|free_attack|npc_attack",
  "attacker": { "entity": { "id": "player-A", "type": "player" }, "name": "Вася" },
  "defender": { "entity": { "id": "wolf-alpha", "type": "npc" }, "name": "Альфа-волк" },
  "outcome": { "hit": true, "natural": 15, "damage": 3, "critical": false, "fumble": false, "target_dead": false, "success": null },
  "hp": { "defender_before": 7, "defender_after": 4, "defender_max": 10 },
  "rolls": [ { "event": { "id": "…" }, "index": 0 }, { "event": { "id": "…" }, "index": 1 } ],
  "rules_version": "0.1", "phase1_mode": "rules", "lod": "rule-only",
  "free_attack": false }
```
`gm_path` и `agent` — в `meta`. Для `flee`: `outcome.success`, `outcome.threshold`, `living_enemies`. Изменение состояния — отдельным `entity.update.proposed` (из `mechanics.ChangesFor`; `meta.causation_id = combat.decided.id`); рекомендация C-03 — один `atomic` пакет на раунд, допускается по одному предложению на `combat.decided`.

#### 2.3.7. Встреча (`encounter.*`) — издатель GM региона (`started`), агент встречи (`ended`)
`encounter{entity}`, `participants[]{entity{player}}`, `npcs[]{entity{npc}, name}`, `region{entity}`, `scope` (игрока/группы); **`encounter.started.round{timeout: "60s", idle_after_missed: 2}`** (опц., C-05 v1.1) — параметры раунда группы из блупринта `encounter-*` (`round` в блупринте; числа — `rules/dark-forest.yaml`); gateway использует их для таймера и порога `idle`, при отсутствии — значения по умолчанию `MV_GATEWAY_ROUND_TIMEOUT` / `MV_GATEWAY_ROUND_IDLE_AFTER_MISSED`. `encounter.ended`: `reason: npc_dead|players_out|abandoned`, `killer{entity}?`, `rounds: int`.

#### 2.3.8. Фоновые и региональные события (`world.*`, `region.*`, `npc.*`) — `meta.actor_kind=system` при тике
`world.weather_changed {weather{from,to}}`, `world.time_advanced {time_of_day{from,to}, day}`, `world.event_occurred {kind, summary (ru, ≤ 300), affects[]{entity}}`; `region.event_occurred {kind, summary, affects[]}`; `npc.moved {entity{npc}, position{from,to}}`; `npc.spawned {entity{npc}, kind, position, spawned_by{agent, tick{event}}}` (респаун по `npc_table` блупринта после `respawn_ttl`; первичное наполнение мира — фикстуры, UC-036). Все — с `meta.agent{level global|domain}`, `meta.causation_type = tick.fired` (фон) или `player.*|world.*` (реакция). Состояние меняется сопутствующим `entity.update.proposed`.

#### 2.3.9. Тик и жизненный цикл агентов (`tick.*`, `agent.*`) — издатель рантайм роя; `meta.agent` обязателен
`tick.fired {agent{…}, scope, tick{seq, mode: background|active, scheduled_at, fired_at, lod_allowed}, budget{window_calls, cap}}` — **`tick.lod_allowed` обязателен** (агент не пересчитывает, ADR-014 п. 6; C-06); корневое событие (`meta.correlation_id = id`, `meta.actor_kind = system`; немедленный тик по admin-запросу — `mode=background`, `meta.actor_kind=ci`). `tick.aborted {tick{seq}, reason}`. `replay` — в `meta`.
`agent.spawned {agent{…}, parent{id}, scope, ttl_expires_at?, lod, content_hash?}` (`content_hash` блупринта — опц., C-07 v1.1), `agent.child_resolved {agent, parent{id}, reason: npc_dead|players_out|ttl|abandoned}`, `agent.stopped {agent, reason: ttl|shutdown|error}`, `agent.spawn_rejected {blueprint, scope, reason: max_instances|validation}`, `agent.blueprint_reloaded {blueprint, version_from, version_to, content_hash}`.

#### 2.3.10. Record-replay LLM (`llm.output`, `llm.output.rejected`) — топик `llm_records`, издатель LLM-шлюз (C-07)
`llm.output` — поля `data-model.md` §7.2 (обязательны: `provider, model, params, prompt_hash, response_raw (кроме quarantined/filter_error), response_hash, validation_status, laws_version, phase, attempt, latency_ms, tokens, cost_usd`; опционально `lod, tools_used[], filter, error{code,message}`, **`parse{strategy, recovered}`** (стратегия восстановления JSON парсером и признак, что ответ был починен — ADR-016 п. 1, v1.1)). `correlation_id`, `caused_by`, `agent`, `actor_kind`, `replay`, `gm_path` — в `meta` (из payload убраны). **`validation_status ∈ valid | partially_rejected | invalid | error | quarantined | filter_error`** — единый enum (C-07 v1.2, ADR-017 дополнение 1; сведение 3): статус — исход конвейера, причины — только в `llm.output.rejected.reason`; `invalid` — схема (`schema_invalid`), язык (`language`), устаревшая `laws_version` или все элементы отброшены; `error` — ответа нет (провайдер/таймаут; `error.code=yielded` — фоновый вызов уступил очередь ходу, ADR-014 п. 2); `quarantined`/`filter_error` — без `response_raw`. Значения `rejected_unknown_entity | rejected_player_agency | rejected_level_violation | rejected_language | budget_exceeded` как статусы **не существуют**. Опц. `reasons[]` — множество `reason` связанных `rejected`. Ключ записи для replay: `(meta.correlation_id, meta.agent.id, phase, attempt)`. Записи для `testdata/recordings`/golden — только `meta.actor_kind=ci` (SEC-23).
`llm.output.rejected {llm_output{event{id}}?, reason: unknown_entity|player_agency|level_violation|schema_invalid|language|filter_blocked|filter_error|budget_exceeded|law_violation|other, element{index, type}?, entity{…}? (для unknown_entity), category? ("a"), budget{kind: background|turn|cloud, limit, window}?, phase, attempt}`. При `budget_exceeded` `llm_output` отсутствует.

#### 2.3.11. Нарратив (`narrative.output`) — топик `narrative_output`; издатель персональный GM (`solo`) / `group-narrator` (`group`); `meta.agent.level=task`
```json
{ "recipients": [ { "entity": { "id": "player-A", "type": "player" } }, { "entity": { "id": "player-B", "type": "player" } } ],
  "text": "…", "generated_by": "llm|template", "fallback_reason": null,
  "kind": "turn|round|world_event|entry|death",
  "round": { "seq": 3 }, "llm_output": { "event": { "id": "…" } },
  "based_on": [ { "event": { "id": "…", "type": "combat.decided" } } ],
  "absence": { "since_at": "<ts>", "background_events_count": 3 },
  "background_refs": [ { "event": { "id": "…", "type": "world.weather_changed" } } ],
  "filter": { "applied": true, "status": "pass", "filter_version": "a-2026-09" },
  "locale": "ru", "laws_version": "v1",
  "narrative_event_id": "<= id этого события; опц., v1.1 — дубль для gateway>" }
```
`scope` (`group:g-1`) и `agent` — в конверте. Один `narrative.output` на ход соло и один на раунд группы (`narrative_event_id` общий для всех адресатов); `recipients[]` — только игроки scope (включая `idle`); текст plain (без разметки для `parse_mode`). Текст в шине — единственная копия для доставки; gateway копирует в `analytics.turn.completed` только счётчики и ID.

#### 2.3.12. Снапшот (`snapshot.created`) — контракт C-14 v1.1
`component: state | swarm | gateway`, `snapshot{id, seq, taken_at, cursor{<topic>: <next_offset>}, laws_version, state_hash, size_bytes, key}`.
Объекты: `snapshots-{world}/{component}/{ts}-{seq}.json` — полный снапшот; **`snapshots-{world}/{component}/latest.json` — указатель** (метаданные + `key` объекта), пишется после успешного PUT объекта; потребители читают указатель, проверяют `state_hash`, затем объект. Префиксы: `state/` — EPIC-002, `swarm/` — EPIC-003, `gateway/` — EPIC-004 (курсоры и проекции; `gateway.db` остаётся истиной для сессий/раундов/outbox). Ротация K=5 в коде, не зависит от bucket versioning (ADR-021). Порядок старта: `state` догоняет журнал (`Journal.ReadRange` с `cursor` до `End`) → `analytics.replay.completed mode=recovery` → `swarm` и `gateway` строят проекции от `state/latest.json` и догоняют `entity.updated` через `Journal`.

#### 2.3.13. Законы и пробой (контракт в MVP-1; обработчики — E-B)
`world.laws.changed {laws{version_from, version_to, status: approved|pending_review|rolled_back}, created_by: author|breach, diff{added[], removed[]}, effective_from{round_boundary: true}}`.
`world.law_breach.proposed {breach{id}, initiator{kind: agent|author|player_indirect, agent?}, cause_event_ids[], strain, laws_removed[], laws_added[], canon_invalidated[], narrative_hook, laws_version_base}`.
`world.law_breach.rejected {breach{id}, reason: immutable_touched|contradiction|canon_conflict_missing|formula_change|unknown_entity|breach_pending|llm_unavailable}`.
`world.law_breach.applied {breach{id}, laws{version_from, version_to}, review{deadline_at}}`.
`world.law_breach.review_decided {breach{id}, decision: approved|rolled_back, reviewer_kind: human|timeout, decided_at}`.
`world.law_breach.rolled_back {breach{id}, laws{version_from, version_to}, retconned_fact_ids[]}`.
Решения G1: `immutable`-законы вселенной не пробиваются (→ `rejected reason=immutable_touched`); истечение окна → `approved reviewer_kind=timeout`.

#### 2.3.14. Аналитика (`analytics.*`) — топик `analytics_events`, по `metrics.md` §4.2; ключ `world.entity.id`; не читаются replay (C-10). **Уточнение (C-10 v1.1, сведение 3, З-1): `analytics.session.ended.session.end_reason ∈ leave | idle | death | error | forget`** — `forget` при закрытии сессии каскадом `/forget` (FR-061); `metrics.md` §4.2 и `session_end_reason_share` — правит BA. `analytics.replay.completed` — одна схема `schemas/events/analytics.replay.completed.v1.json` (владелец файла EPIC-002; поля `mode=test`, `events_hash_match` — EPIC-005).

#### 2.3.15. Инцидент контента (`content.incident.recorded`) — топик `system_events`, издатель LLM-шлюз
`incident{category: "a", filter_version, llm_output{event{id}}}` — без текста; `agent`, `scope` — в конверте.

#### 2.3.16. Конфигурация облака (`config.cloud_enabled`) — топик `system_events`, издатель LLM-шлюз (новый тип, ADR-005 дополнение п. 3)
`{ "enabled": true|false, "provider": "<имя провайдера из MV_LLM_PROVIDER>", "external_players_ack": true }` — без ключей, URL и секретов. **Публикуется при каждом старте `core` (в том числе `enabled=false`) и при изменении флага** (C-06 v1.1, сведение 3, З-3) — чтобы проекция gateway (`GET /v1/worlds → worlds[].llm.cloud_enabled`, §1.3) была детерминирована после рестарта; gateway хранит последнее значение по офсету (по умолчанию `false`) и наружу отдаёт только `cloud_enabled`. Включение — флаг `MV_LLM_CLOUD_ENABLED` (+ подтверждение при внешних игроках, UC-030). Блупринт включить облако не может.

### 2.4. Правила валидации вывода LLM (страж в конвейере)

| # | Проверка | Фаза | Провал → `reason` | Действие |
|---|---|---|---|---|
| 1 | JSON по схеме фазы (structured output) | все | `schema_invalid` | повтор ≤ N, затем шаблон/правила |
| 2 | Язык: 0 CJK; латиница ≤ порога (NFR-090) | narrative, tick с текстом | `language` | повтор, затем шаблон |
| 3 | Все ссылки `entity{id}` существуют и видимы агенту | все | `unknown_entity` | элемент отброшен |
| 4 | Нет `player.*` и действий персонажей, которых те не совершали | все | `player_agency` | элемент отброшен |
| 5 | Типы событий ∈ `allowed_event_types` блупринта; изменяемые сущности ∈ `owned_entity_types` уровня и scope агента | все | `level_violation` | элемент отброшен |
| 6 | Инварианты `laws@v1` и `laws_version` актуальна | все | `law_violation` | элемент отброшен |
| 7 | `NarrativeFilter` категории (a) | текст | `filter_blocked` / `filter_error` | `quarantined`/fail-closed, шаблон |
| 8 | Бюджет до вызова | все | `budget_exceeded` | без вызова |
| 9 | Числа нарратива ≠ механики | narrative | — (метрика) | доставляется |

Белые списки уровней (MVP-1): `global` → `world.weather_changed, world.time_advanced, world.event_occurred` + `entity.update.proposed(world)`; `domain` → `region.event_occurred, npc.moved, npc.spawned, encounter.started` + `entity.*.proposed(region, npc, encounter)`; `task` (встреча) → `dice.rolled`, `combat.decided` (через механику), `encounter.ended` + `entity.update.proposed(encounter, npc/player во встрече)`; `task` (персональный GM `player-gm`, нарратор группы `group-narrator`) → только `narrative.output`. Таблица уровней — `shared/agent/levels.go`, экспортируется как `contracts.OwnershipRules` (C-02).

---

## 3. Контракт блупринта агента (три уровня роя + встреча)

**Формат (контракт C-11, парсер — ADR-015):** один файл Markdown с YAML-frontmatter (метаданные и конфигурация) и секциями `## system`, `## phase1`, `## phase2`, `## tick`, `## canon`, `## description` (текст). Чистый YAML допустим при том же наборе полей. Плейсхолдеры промптов — фиксированный словарь (§3.1). `blueprint_version` = семвер из файла; `content_hash` файла попадает в `agent.spawned` и `agent.blueprint_reloaded`. Совместимость с `shared/agent.AgentBlueprint`: существующие поля сохранены (`name, version, description, trigger, constraints, llm, tools, parent, ttl, phase1_prompt, phase2_prompt, type`); новые добавлены. Валидатор один и тот же в `mvctl blueprint validate <dir>` и в рантайме (`agent.Validate(bp, env)`).

**Инициализация мира (решение G2):** блупринты **не** являются источником первичного состояния мира на MVP-1 — мир, регион, NPC и фикстурные персонажи создаются из `testdata/fixtures/*.json` командой `mvctl world init --fixtures` (UC-036); `npc_table` блупринта `domain-*` используется региональным GM только для респауна после `respawn_ttl`; `mvctl world init --blueprints` — вне MVP-1 (кандидат EPIC-011). Тест согласованности (EPIC-003 I1b): каждая запись `npc_table` имеет `stats_ref` в `rules/dark-forest.yaml` и тип сущности из фикстур.

### 3.1. Поля

| Поле | Тип | Обяз. | Уровни | Описание / ограничения |
|---|---|---|---|---|
| `name`, `version`, `description` | string | да | все | `name` уникален; `version` semver |
| `level` | enum `global\|domain\|task\|monitor` | да | все | = `AgentLevel` |
| `role` | enum `global-gm\|region-gm\|city-gm\|personal-gm\|group-narrator\|encounter` | да | все | продуктовая роль; `group-narrator` — нарратор раунда группы (C-11 v0.2; один на scope `group`, `level: task`) |
| `scope_binding` | `{type: world\|region\|solo\|group, id?: string, pattern?: string}` | да | все | `global` → `world`; `domain` → `region` c `id`; `personal-gm` → `solo` (pattern `solo:*`); `group-narrator` → `group` (pattern `group:*`); `encounter` → scope игрока/группы |
| `parent` | `{name, instance?}` | да, кроме `global` | domain, task, monitor | родитель существует; `personal-gm.parent`/`group-narrator.parent` = `region-gm` региона текущей позиции (динамически) |
| `trigger` | `{type: timer\|event, event_name?, conditions[]?, intervals?: {idle: duration, active: duration}}` | да | все | `timer` требует `intervals.idle`; `active` — для `domain` при игроках; `event` — `event_name` из реестра, **допускает glob по сегментам** (`player.*`, `group.*`; `*` не пересекает точку; C-11 v1.1) — валидатор разворачивает по реестру `contracts.Types()` и требует ≥ 1 совпадения |
| `ttl` | duration | task/monitor | | не применяется к `global`/`domain` (игнорируется с предупреждением) |
| `constraints.max_instances` | int | да | все | 1 для `global`/`domain`/`personal-gm` на scope |
| `constraints.priority` | int | нет | | |
| `llm.phase1` | `{model, temperature, max_tokens, thinking: bool, schema_ref}` | да для агентов с Phase 1 | domain, task | MVP-1: `phase1.mode: rules` допустим (без LLM) |
| `llm.phase2` | `{model, temperature, max_tokens, thinking, schema_ref}` | personal-gm, group-narrator | | модель нарратива |
| `llm.tick` | `{model, lod_default: rule-only\|basic, max_tokens, schema_ref}` | global, domain | | модель малая |
| `llm.*.model` (правило) | string | | все | имя модели проверяется против списка провайдера `Provider.Models()` для `MV_LLM_PROVIDER`; **блупринт не может указать провайдера или включить облако** (только `MV_LLM_CLOUD_ENABLED` оператора; SEC-21, C-11 v1.1) |
| `llm.retries` | int | нет | | по умолчанию 2 |
| `llm.fallback` | `template\|rules` | да | | |
| `allowed_event_types[]` | list | да | все | белый список публикуемых типов (§2.4) |
| `owned_entity_types[]` | list | да | все | `world` / `region, npc, encounter` / `encounter, npc, player(in_encounter)` / `[]` |
| `tools[]` | `{name, owner}` | нет | | зарегистрированы в реестре инструментов |
| `laws_ref` | string | да | global, domain | `laws/dark-forest-world@v1` |
| `rules_ref` | string | да | domain, encounter | `rules/dark-forest.yaml` |
| `description_text` (секция `## description`) | text | да | domain | авторское описание региона (слой `authored`) |
| `canon[]` (секция `## canon`) | list текстов | нет | global, domain | факты `authored` |
| `npc_table[]` | `{npc_id, kind, name, stats_ref, count, spawn: {on: region_init\|tick, chance?: formula}}` | да | domain | **только респаун** (`respawn_ttl`); первичное наполнение — фикстуры (`mvctl world init --fixtures`, решение G2); `spawn.on: region_init` на MVP-1 не исполняется, запись должна совпадать с фикстурой (`stats_ref` в `rules`) |
| `respawn_ttl` | duration | да | domain | 24h по умолчанию |
| `encounter` | `{detect_on: tick, perception: all\|radius, chance?: formula, child_blueprint: encounter-wolf\|inline}` | да | domain | |
| `background_events[]` | `{kind, weight, summary_template, ops[]?}` | да | global, domain | таблица LOD 1 без LLM |
| `round` | `{timeout: 60s, idle_after_missed: 2}` | да | encounter | параметры раунда группы; **источник `encounter.started.round{}`** (C-05 v1.1), gateway читает их из события; env gateway — только значения по умолчанию |
| `absolute_limits_ref` (у `group-narrator`) | string | да | group-narrator | как у `personal-gm` |
| `budget` | `{background_calls_per_hour_world: 4}` | да | global (мир) | общий потолок фона |
| `absolute_limits_ref` | string | да | personal-gm, encounter | ссылка на конфиг запретов (короткая позитивная формулировка попадает в `## system`) |
| `invariants[]` | list `{id, check}` | да | global (мир) | инварианты NFR-020 как `laws@v1` |
| `locale` | string | да | все | `ru` |
| `prompts` (секции `## system`, `## phase1`, `## phase2`, `## tick`) | text с плейсхолдерами `{world.weather}`, `{region.description}`, `{events}`, `{state}`, `{absence}`, `{laws_version}`, `{player.name}` | по фазам | | плейсхолдеры — из фиксированного словаря; неизвестный — ошибка валидации |

### 3.2. Валидация (ошибки с файлом, полем, причиной)
Обязательные поля по уровню; `trigger.timer` без `intervals.idle`; `trigger.event_name` (в т. ч. glob) не совпадает ни с одним типом реестра; `parent` не найден; `tools[].name` не зарегистрирован; `allowed_event_types` содержит тип вне реестра или вне разрешённых уровню; `owned_entity_types` шире уровня; `rules_ref`/`laws_ref` не найдены; модели не заданы для используемых фаз; модель отсутствует в `Provider.Models()` текущего провайдера; поле провайдера/облака в блупринте — ошибка; плейсхолдер вне словаря; `ttl` у `global`/`domain` — предупреждение; `max_instances ≠ 1` для `global`/`domain`/`personal-gm`/`group-narrator` — ошибка; `npc_table[].stats_ref` отсутствует в `rules` — ошибка. Тот же валидатор — в рантайме при загрузке и горячей перезагрузке (`agent.blueprint_reloaded`).

### 3.3. Скелеты MVP-1

**`global-dark-forest-world.md`**
```yaml
name: global-dark-forest-world
version: "1.0"
level: global
role: global-gm
scope_binding: { type: world, id: dark-forest-world }
trigger: { type: timer, intervals: { idle: 60m } }
constraints: { max_instances: 1 }
llm:
  tick: { model: qwen3:8b, lod_default: basic, max_tokens: 512, schema_ref: schemas/tick-global.json }
  retries: 1
  fallback: rules
allowed_event_types: [world.weather_changed, world.time_advanced, world.event_occurred]
owned_entity_types: [world]
laws_ref: laws/dark-forest-world@v1
budget: { background_calls_per_hour_world: 4 }
background_events:
  - { kind: weather_shift, weight: 5, summary_template: "Погода меняется: {from} → {to}", ops: [{ path: weather, value: "{next_weather}" }] }
  - { kind: time_advance, weight: 10, summary_template: "Наступает {time_of_day}" }
invariants: [ { id: inv-1, check: "dead entities do not act" }, { id: inv-2, check: "0 <= hp <= hp_max" } ]
locale: ru
```
Секции: `## system` (роль, законы, «мир вымышлен»), `## tick`.

**`domain-dark-forest.md`**
```yaml
name: domain-dark-forest
version: "1.1"
level: domain
role: region-gm
scope_binding: { type: region, id: dark-forest-01 }
parent: { name: global-dark-forest-world }
trigger: { type: timer, intervals: { idle: 30m, active: 60s } }
constraints: { max_instances: 1 }
llm:
  phase1: { mode: rules }
  tick: { model: qwen3:8b, lod_default: basic, max_tokens: 512, schema_ref: schemas/tick-region.json }
  fallback: rules
allowed_event_types: [region.event_occurred, npc.moved, npc.spawned, encounter.started]
owned_entity_types: [region, npc, encounter]
laws_ref: laws/dark-forest-world@v1
rules_ref: rules/dark-forest.yaml
npc_table:   # только респаун после respawn_ttl; wolf-alpha создаётся из фикстур (mvctl world init --fixtures)
  - { npc_id: wolf-alpha, kind: wolf, name: "Альфа-волк", stats_ref: wolf, count: 1, spawn: { on: tick } }
respawn_ttl: 24h
encounter: { detect_on: tick, perception: all, child_blueprint: encounter-wolf }
background_events:
  - { kind: howl, weight: 5, summary_template: "На севере слышен вой" }
  - { kind: npc_wander, weight: 3, summary_template: "{npc.name} переходит в {to}" }
locale: ru
```
Секции: `## description` (регион, слой `authored`), `## canon`, `## system`, `## tick`.

**`encounter-wolf.md`**
```yaml
name: encounter-wolf
version: "1.0"
level: task
role: encounter
scope_binding: { type: solo|group, pattern: "*" }
parent: { name: domain-dark-forest }
trigger: { type: event, event_name: encounter.started }
ttl: 30m
constraints: { max_instances: 1 }
llm: { phase1: { mode: rules }, fallback: rules }
allowed_event_types: [combat.decided, encounter.ended]
owned_entity_types: [encounter, npc, player]   # только участники встречи, только через механику
rules_ref: rules/dark-forest.yaml
round: { timeout: 60s, idle_after_missed: 2 }
locale: ru
```

**`player-gm.md`**
```yaml
name: player-gm
version: "1.0"
level: task            # решено: task (C-05 — meta.agent.level ∈ task для нарратива)
role: personal-gm
scope_binding: { type: solo, pattern: "solo:*" }
parent: { name: "<region-gm of current position>", instance: dynamic }
trigger: { type: event, event_name: player.entered_region }
ttl: 45m               # ≥ границы сессии 30 мин
constraints: { max_instances: 1 }
llm:
  phase2: { model: qwen3:30b-a3b, temperature: 0.8, max_tokens: 700, thinking: false, schema_ref: schemas/narrative.json }
  retries: 2
  fallback: template
allowed_event_types: [narrative.output]
owned_entity_types: []
absolute_limits_ref: config/absolute-limits.yaml
locale: ru
```
Секции: `## system` (роль рассказчика; «числа механики — истина»; короткая позитивная формулировка запретов; `locale`), `## phase2` (плейсхолдеры `{events}`, `{state}`, `{world.weather}`, `{absence}`, `{region.description}`, `{canon}`).

**`group-narrator.md`** (C-11 v0.2; EPIC-003 I2 — нарратор раунда группы; заменяет вариант v0.1 «персональный GM лидера», UC-020 шаг 1)
```yaml
name: group-narrator
version: "1.0"
level: task
role: group-narrator
scope_binding: { type: group, pattern: "group:*" }
parent: { name: "<region-gm of current position>", instance: dynamic }
trigger: { type: event, event_name: group.created }
ttl: 45m
constraints: { max_instances: 1 }
llm:
  phase2: { model: qwen3:30b-a3b, temperature: 0.8, max_tokens: 900, thinking: false, schema_ref: schemas/narrative.json }
  retries: 2
  fallback: template
allowed_event_types: [narrative.output]
owned_entity_types: []
absolute_limits_ref: config/absolute-limits.yaml
locale: ru
```
Секции: `## system`, `## phase2` (плейсхолдеры `{events}` — все `combat.decided`, `player.said`, `round.closed{acted, auto_defended, idle}` раунда; `{state}`, `{world.weather}`, `{region.description}`, `{canon}`). Один `narrative.output kind=round` на `round.closed`, `recipients[]` = все участники группы (включая `idle`; `dead` — только нарратив своей гибели); в группе персональные GM участников вызовов LLM не делают (`rule-only`).

### 3.4. Схема структурированного ответа Phase 2 (`schemas/agent/narrative.json`, логически)
`{ text: string (ru), background_refs: [event_id], mentions: [entity_id], tone?: enum }` — `background_refs` обязателен при наличии `absence` в контексте (может быть пустым). Схема Phase 1 (`decision`) в MVP-1 не используется (`mode: rules`); схема тика (`schemas/agent/tick-global.json`, `tick-region.json`): `{ events: [ { type ∈ allowed_event_types, summary, ops[] } ] }`. Вывод LLM проходит парсер (ADR-016: восстановление JSON, результат — `llm.output.parse{strategy, recovered}`), затем проверки §2.4.

---

## 4. Сверка с `contracts.md` v0.2

| Контракт | Суть | Разделы этого документа | Расхождения |
|---|---|---|---|
| C-01 конверт, реестр, шина, журнал | `meta`, `NewRoot`/`Derive`, `Journal{ReadRange, Tail, End}`, `Position`, `Dedup`, валидация при чтении, политики топиков, JSON Schema 2020-12, `clock`/`runtime` | §0, §2.0, §2.1, §2.2 (топики/retention) | нет |
| C-02 предложения и факты State | `entity.*.proposed/created/updated/rejected`; `rest` от gateway; `atomic=false`; `applied_at`; `duplicate_entity`; `append`; `OwnershipRules` | §2.3.4, §2.4 (белые списки) | нет |
| C-03 механика и RNG (Go) | `Resolve`, `NPCTarget`, `Roll`, `Seed`, `DiceRolledPayload`, `ChangesFor` | §2.3.5 (seed, порядок `dice.rolled` → `combat.decided`), §2.3.6 | нет (Go-сигнатуры — только в C-03) |
| C-04 действия игрока, группа, раунд | `player.*`, `group.*`, `round.*`; `group.entered_region` — единственный формат; `say` вне раунда; лидерство при смерти; `player.defended` до `round.closed` | §1.4, §2.3.1–§2.3.3 | нет; BR-13/FR-025 — у BA |
| C-05 нарратив и механика для доставки | `narrative.output` + `narrative_event_id`; `combat.decided`; `encounter.started.round{}` | §1.5 (`Delivery`), §2.3.6, §2.3.7, §2.3.11 | нет |
| C-06 тики, admin-HTTP `core` | `tick.fired.tick.lod_allowed` обязателен; `/v1/admin/*` на `MV_CORE_ADDR`, прокси через gateway | §1.8, §2.3.9 | нет |
| C-07 записи LLM | `llm.output` + `parse{}`, `yielded`, ключ replay, `content_hash`, записи только `actor_kind=ci` | §2.3.9, §2.3.10, §2.3.15 | нет |
| C-08 HTTP API v1 | коды v1.1, `202 pending`, `creating`, `link_id`, `{client_id}`==`X-Client-Id`, лимиты, `501` для `/stream` | §1.1–§1.8 | нет; `character_status=none` вместо `null` v0.1 (совместимо, до реализации) |
| C-09 память | `POST /v1/context/scope`, `/absence`, `GET /v1/trace/{correlation_id}`; деградация `journalContext`; экранирование `generated` | вне этого документа (`api/memory.openapi.yaml`, EPIC-005 005-memory — отрезаема); использование — UC-004, UC-020 | нет |
| C-10 аналитика | `analytics.*` по `metrics.md` §4.2; `analytics_events`, не в replay | §2.2, §2.3.14 | нет |
| C-11 формат блупринта | §3 + `group-narrator`, glob, валидация модели/провайдера, `round{}` | §3.1–§3.3 | нет |
| C-12 законы и пробой | `world.laws.changed`, `world.law_breach.*`, `laws_version` везде | §2.2, §2.3.13 | нет |
| C-13 резерв E-A | уровень `object`, `rules.change.*` без издателя | §3.1 (`level` enum), §2.2 (исключения замкнутости) | нет (типы `rules.change.*` регистрирует EPIC-001 по C-13; в карте §2.2 не перечислены как MVP-1) |
| C-14 снапшоты | `component ∈ state\|swarm\|gateway`, `latest.json` — указатель, `cursor{next_offset}`, `Journal.End` | §2.3.12, §2.0 | нет |
| C-15 провайдер LLM | `Provider{Generate, Embed, Health, Models}`; облако только флагом оператора; `config.cloud_enabled` | §2.3.16, §3.1 (`llm.*.model`) | нет |

Открытые расхождения с `contracts.md` v0.2: **нет**. **Сведение 3 (v0.2.1, system-architect):** документ приведён к `contracts.md` v0.4 — C-02 v1.2 (`cause=forget`, `abandoned`), C-04 v1.1 (`leader: null`, `cause=forget`, `409 no_leader`, каскад `/forget`), C-06 v1.1 (`config.cloud_enabled` при каждом старте), C-07 v1.2 (`validation_status` 6 значений), C-08 v1.2 (`worlds[].llm.cloud_enabled`, `GroupView.leader_id` nullable), C-10 v1.1 (`end_reason=forget`), §0 (`Spec.Publishers`); при следующей ревизии system-analyst обновляет таблицу §4 и примеры §1.4/§2.3.1 по этим версиям. Пункты, переданные BA (не расхождения): FR-025 (`say` вне раунда), BR-13 (лидерство при смерти), NFR-020 inv-04 (`alive`), FR-009 (текст уведомления), FR-034 (+`law_violation`, `filter_blocked`).
