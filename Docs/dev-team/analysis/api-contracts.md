# Контракты API и событий

**Инициатива:** PROJECT · **Этап:** A2 · **Версия:** 0.1 · 2026-09-09 · системный аналитик (dev-team)
**Основание:** `prd.md` v0.2 (FR-002…FR-005, FR-009, FR-013, FR-021, FR-022, FR-030…FR-036, FR-045, FR-050, FR-060, FR-084…FR-088, FR-120…FR-128, BR-02…BR-08, BR-13, BR-16); `metrics.md` §4; `domain-review.md` §3; аудит as-is (`architecture/audit-facts.md` §4–5); код `shared/eventbus` (`Event{id,type,timestamp,source,world,scope,payload,relations}`, `EventPayload` builder: `WithEntity/WithTarget/WithSource/WithWorld/WithScope`, `SetNested`, `event.Path()`), `shared/agent/agent_types.go` (`AgentLevel`, `AgentBlueprint`), `services/game-service` (HTTP как есть); решения G1.

Состав: **§1** HTTP API game-service для эталонного клиента; **§2** контракты событий MVP-1 с картой «событие → топик → издатель → потребители»; **§3** контракт блупринта агента. Технологические решения (какой топик создавать, формат схем — OpenAPI/JSON Schema, транспорт push) помечены «решение архитектора».

---

## 0. Общие соглашения

- Идентификаторы, имена полей, типы событий, коды ошибок — английские, `snake_case`; типы событий — `object.action` в прошедшем времени для фактов (`entity.updated`), `*.proposed` для предложений, `*.requested` для команд.
- Время — ISO-8601 UTC. Деньги — `cost_usd` (число).
- `correlation_id` — `event_id` события-причины (действие игрока или `tick.fired`); путь в событии — `payload.correlation_id` **(предложение PA; решение архитектора по OQ-A-09)**; кроме него — `payload.trigger.event{id,type}` (непосредственная причина).
- `actor_kind` ∈ `human | ci | sim | system` — в `payload.actor_kind`; для ходов наследуется от сессии, для тиков — `system`.
- `agent{id, level, blueprint, blueprint_version}` — обязателен во всех событиях, изданных агентами роя; `level` ∈ `global | domain | task | object | monitor` (строки `AgentLevel.String()`).
- `schema_version` (int, с 1) — `payload.schema_version` во всех новых типах **(решение архитектора: возможно вынести на уровень конверта)**.
- `laws_version` — в `llm.output`, `world.laws.*`, `snapshot.created`, контексте агента.
- `replay` (bool) — в `llm.output`, `dice.rolled`, `tick.fired`, `round.closed`: `true` только для событий, прочитанных в replay (не переиздаются; флаг нужен потребителям в тестовом режиме).

---

## 1. HTTP API game-service (эталонный клиент — бот)

### 1.1. Общие правила

| Аспект | Правило |
|---|---|
| Базовый путь | `/v1` (as-is эндпоинты без версии — см. §1.9) |
| Формат | JSON UTF-8; `Content-Type: application/json` |
| Авторизация | MVP-1: нет (доверенный контур, бот в той же сети). Заголовок `X-Client-Id` (идентификатор экземпляра клиента: `telegram-bot`, `ci-harness`) — обязателен, используется для маршрутизации доставок и отчётов. Целевое: токен клиента (E-H) |
| `actor_kind` | заголовок `X-Actor-Kind: human\|ci\|sim` (по умолчанию `human`); `ci`/`sim` разрешены только клиентам из списка конфигурации; служебные эндпоинты §1.8 — только `ci` |
| Идемпотентность | все изменяющие запросы принимают `action_key` (строка ≤ 64, уникальна в пределах `player_id`/внешнего ID, TTL 24 ч); повтор с тем же ключом → тот же ответ и тот же `correlation_id`, второго хода нет; ключ отличается, тело совпадает — новый ход |
| Ошибки | `4xx/5xx` с телом `{ "error": { "code": "<snake_case>", "message": "<ru текст для игрока>", "details": {…} } }`; коды — §1.6 |
| Синхронность | действия принимаются `202 Accepted` после валидации (≤ 300 мс NFR-003); результаты приходят доставками (§1.5). Регистрация/статус — синхронно |
| Приватность | внешний ID допускается только в §1.2 и §1.3 (создание персонажа); в остальных запросах — `player_id`. Сервер не логирует тела запросов §1.2/§1.3 |
| Версионирование | путь `/v1`; несовместимые изменения — `/v2`; поля добавляются совместимо |

### 1.2. Связки и согласие

#### `POST /v1/links/resolve` — разрешить внешний аккаунт в `player_id`
Запрос: `{ "external_platform": "telegram", "external_id": "<string>" }`
Ответ `200`: `{ "link_status": "none|pending_consent|consented", "player_id": "player-A|null", "world_id": "…|null", "character_status": "alive|dead|null", "notice_due": true|false }` (`notice_due` — показать уведомление повторно: `/help` или ≥ 30 дней с `last_seen_at`; сервер обновляет `last_seen_at`).
Ошибки: `400 invalid_request`. Идемпотентно. Связь: UC-001, UC-003.

#### `POST /v1/links/consent` — зафиксировать уведомление, согласие, 18+
Запрос: `{ "external_platform", "external_id", "notice_shown": true, "consent": true, "age_confirmed": true, "shown_at": "<ts>" }`
Ответ `200`: `{ "link_status": "consented", "consent_at": "<ts>", "age_confirmed_at": "<ts>", "notice_shown_at": "<ts>" }`
Ошибки: `400 consent_incomplete` (любой из трёх флагов не `true` — запись `pending_consent`, ничего не создаётся). Идемпотентно (повтор обновляет `last_seen_at`). Связь: UC-001.

#### `DELETE /v1/links` — удалить связку (`/forget`)
Запрос: `{ "external_platform", "external_id" }`
Ответ `200`: `{ "deleted": true, "player_id_detached": "player-A|null" }`; если связки нет — `200 { "deleted": false }` («нечего удалять»).
Побочные эффекты: outbox игрока очищен; активная сессия закрыта `end_reason=leave`; игрок выведен из группы (`group.left`). Связь: UC-031.

#### `DELETE /v1/admin/links/{player_id}` — удаление оператором
Ответ как выше. Только `X-Client-Id` из списка операторских клиентов.

### 1.3. Миры и персонажи

#### `GET /v1/worlds` — список миров
Ответ `200`: `{ "worlds": [ { "world_id": "dark-forest-world", "name": "…", "regions": [ { "region_id": "dark-forest-01", "name": "Тёмный лес" } ], "laws_version": "v1" } ] }`. Связь: UC-002, FR-006.

#### `POST /v1/characters` — создать персонажа
Запрос: `{ "external_platform", "external_id", "world_id", "character_name", "action_key" }`
Ответ `201`: `{ "player_id": "player-A", "character": <CharacterState>, "created": true }`
`200`: живой персонаж уже есть → `{ "player_id", "character", "created": false }`.
`202`: создание принято, факт ещё не подтверждён → `{ "player_id", "status": "creating" }` (бот опрашивает `GET /v1/players/{id}`).
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
Не ход; событий не порождает. Ошибки: `404 player_not_found`. Связь: UC-003.

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
| `rest` | — | — | `alive`; вне встречи | да | HP |
| `say` | — | 1–500 символов после trim | `alive` | да (не закрывает раунд) | нет |
| `defend` | — | — | `alive`; во встрече (явное «пропустить») | да | нет (цель для NPC) |
| `group.create` | — | — | `alive`; не в группе | нет (служебное) | группа |
| `group.join` | `group_id` | — | `alive`; не в группе; группа ≤ 5; группа вне встречи | нет | группа |
| `group.leave` | — | — | в группе; во встрече → исполняется как `flee` | нет / да (как flee) | группа |

Ответ `202`:
```json
{ "correlation_id": "<event_id действия>", "turn": { "seq": 10, "session_id": "…", "round_seq": 3 }, "status": "accepted", "acked_at": "<ts>" }
```
Для `group.*` — `200` с составом группы: `{ "group_id", "leader_id", "members": [...], "position" }`.
Ошибки (`4xx`, `status=rejected`, `analytics.turn.completed status=rejected`): см. §1.6. Повтор `action_key` → `202` с тем же `correlation_id`.
Связь: UC-004…UC-017.

### 1.5. Доставка результатов клиенту

Два эквивалентных механизма (контракт сообщения один; транспорт — **решение архитектора**):

#### `GET /v1/clients/{client_id}/deliveries?after=<cursor>&limit=100&wait_ms=25000` — long-poll outbox
Ответ `200`: `{ "deliveries": [ <Delivery> ], "cursor": "<opaque>" }`. Клиент подтверждает: `POST /v1/clients/{client_id}/deliveries/ack { "ids": [...] }` (без ack — повтор через `redelivery_after`).

#### `GET /v1/clients/{client_id}/stream` — push (WebSocket) тех же `Delivery`; ack тем же эндпоинтом.

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
`kind` ∈ `ack | mechanics | narrative | world_event | group | system`; `generated_by` ∈ `rules | llm | template`; для `template` — `fallback_reason` и пометка «упрощённый режим» в тексте; для `narrative` — `narrative_event_id` одинаков у всех адресатов раунда. Порядок доставки одному `player_id` — строго по `created_at`. `route.external_id` заполняется из хранилища связок в момент выдачи; после `/forget` доставки отбрасываются.

### 1.6. Коды ошибок

| HTTP | `code` | Когда |
|---|---|---|
| 400 | `invalid_request` | тело/поля не по схеме |
| 400 | `unknown_action` | `type` вне словаря |
| 400 | `text_invalid` | `say` пустой/длиннее 500 |
| 400 | `name_required`, `name_invalid` | имя персонажа |
| 403 | `consent_required` | связка не `consented` |
| 403 | `actor_kind_forbidden` | `ci`/`sim` от неразрешённого клиента |
| 404 | `player_not_found`, `world_not_found` | |
| 404 | `unknown_target` | регион/NPC/группа не существует или не в scope |
| 409 | `character_dead` | действия от `dead` |
| 409 | `in_encounter` | `enter/leave/rest/join` во встрече |
| 409 | `not_in_encounter` | `attack/flee/defend` без встречи |
| 409 | `target_dead` | атака мёртвой цели (соло) |
| 409 | `not_leader` | `enter/leave` от не-лидера |
| 409 | `not_in_region` | `leave` с опушки |
| 409 | `already_acted` | второе действие в открытом раунде |
| 409 | `already_in_group`, `not_in_group`, `group_full`, `group_in_encounter` | группа |
| 409 | `encounter_unavailable` | Task-агент встречи не готов (временно) |
| 422 | `filter_error` | `InputFilter` упал (fail-closed) |
| 429 | `rate_limited` | защита от флуда (конфигурация) |
| 503 | `bus_unavailable`, `state_unavailable` | зависимость недоступна — действие **не** принято |

### 1.7. Группы (чтение)
`GET /v1/groups/{group_id}` → состав, лидер, позиция, встреча. Связь: UC-014.

### 1.8. Служебные эндпоинты (тест-харнесс, оператор)

| Метод | Путь | Кто | Назначение |
|---|---|---|---|
| `POST` | `/v1/scopes/{scope_id}/rounds/close` | `ci` | явное закрытие раунда → `round.closed close_reason=explicit` |
| `POST` | `/v1/admin/agents/{agent_id}/tick` | `ci`, оператор | немедленный тик (ускорение фона) → обычный `tick.fired` |
| `GET` | `/v1/admin/agents` | оператор | активные агенты по уровням (`agents_active_by_level`) |
| `GET` | `/v1/admin/sessions` | оператор | активные сессии |
| `GET` | `/v1/admin/llm/usage?since=` | оператор | вызовы/токены по фазам/уровням/провайдерам (Should; MVP-1 — через CLI) |
| `GET` | `/health` | все | `{ "status": "ok|degraded|fail", "deps": { "bus": "ok", "state_store": "ok", "llm": "unavailable", "links_store": "ok" }, "agents_by_level": {...}, "snapshot": "ok|corrupted|missing" }` |

Рантайм роя может отдавать `/v1/admin/agents` сам — архитектор.

### 1.9. Соответствие as-is → to-be

| As-is (`services/game-service`) | To-be |
|---|---|
| `POST /players/register {player_id, player_name, world_id}` — клиент задаёт `player_id`, пишет в MinIO напрямую, публикует `entity.created` | `POST /v1/characters` — `player_id` генерирует сервер; создание через `entity.create.proposed` → entity-manager |
| `POST /players/login` | `POST /v1/links/resolve` + `GET /v1/players/{id}` |
| `GET /entities/{id}?world_id=` | `GET /v1/players/{id}` (игроку — только своё); сущности — не публичный API |
| `GET /entities/{id}/history`, `GET /events/recent` (TODO) | не в клиентском API; трасса — CLI/`/v1/admin/trace/{correlation_id}` (Should) |
| `GET /run_test` (публикация тестовых событий) | удалить; тест-харнесс работает через §1.4/§1.8 |
| `WS /ws/entities`, `/ws/events`, `/ws/actions` — широковещание всем клиентам | `GET /v1/clients/{id}/stream` — адресно; приём действий только через `POST …/actions` |
| `player.moved`, `player.used_skill` с плоским payload | события §2.3.1 с иерархическим payload |

---

## 2. Контракты событий

### 2.1. Конверт и общие поля payload

Конверт — как в `shared/eventbus.Event`: `id, type, timestamp, source, world{entity{id,type:world}}, scope{id,type}, payload, relations[]`. Ключ сообщения — `world.entity.id`.

Обязательные поля `payload` для **всех** типов MVP-1 (создаются билдером `NewEventPayload().WithWorld().WithScope()` + `SetNested`):

| Поле | Тип | Описание |
|---|---|---|
| `schema_version` | int | версия схемы типа (с 1) |
| `correlation_id` | string | причина-корень |
| `trigger{event{id,type}}` | ref | непосредственная причина (у корневых — отсутствует или равна себе) |
| `actor_kind` | enum | `human, ci, sim, system` |
| `world{entity{id,type}}` | ref | дублирует конверт (для `event.Path()`) |
| `scope{id,type}` | ScopeRef | по типу события |
| `entity{entity{id,type},name}` | ref | основная сущность (где применимо) |
| `target{entity{id,type},name}` | ref | цель (где применимо) |
| `agent{id,level,blueprint,blueprint_version}` | obj | у событий агентов/механики в конвейере агента |

Чтение — `event.Path().GetString("entity.entity.id")`, `eventbus.GetWorldIDFromEvent`, `eventbus.GetScopeFromEvent`; fallback на плоские поля (`entity_id`, `player_id`) — только для legacy-типов на время миграции.

### 2.2. Карта «событие → топик → издатель → потребители» (MVP-1)

Топики: `PE` = `player_events`, `WE` = `world_events`, `GE` = `game_events`, `SE` = `system_events`, `NO` = `narrative_output`; **новые (решение архитектора):** `AE` = `analytics_events`, `LR` = `llm_records`. `scope_management` — не используется (кандидат на удаление). Столбец «As-is» — разрыв из `audit-facts.md` §5.

| Тип | Топик | Издатель | Потребители | v | As-is |
|---|---|---|---|---|---|
| `player.entered_region` | PE | game-service | GM региона, персональный GM, semantic-memory, страж | 1 | есть в блупринте (`domain-dark-forest`), издателя нет |
| `player.left_region` | PE | game-service | GM региона, персональный GM, semantic-memory | 1 | нет |
| `player.looked` | PE | game-service | персональный GM, semantic-memory | 1 | `player.looked_around` только в тест-харнессе |
| `player.attacked` | PE | game-service | Task-агент встречи, персональный GM, semantic-memory | 1 | `player.used_skill` (legacy) |
| `player.flee_attempted` | PE | game-service | Task-агент, персональный GM | 1 | нет |
| `player.rested` | PE | game-service | механика, персональный GM | 1 | нет |
| `player.said` | PE | game-service | нарратор scope, semantic-memory | 1 | нет |
| `player.defended` | PE | game-service (`cause: player\|round_timeout`) | Task-агент | 1 | нет |
| `group.created`, `group.joined`, `group.left`, `group.leader_changed`, `group.disbanded` | GE | game-service | GM региона, персональные GM, semantic-memory | 1 | `gm.merged/split` (legacy, заменяются) |
| `group.entered_region`, `group.left_region` | PE | game-service | как `player.entered_region` | 1 | нет |
| `round.opened`, `round.closed` | GE | game-service | Task-агент, нарратор, рантайм роя (replay) | 1 | нет |
| `entity.create.proposed`, `entity.update.proposed` | SE | game-service, механика, агенты роя | entity-manager | 1 | нет (game-service пишет в MinIO напрямую) |
| `entity.created`, `entity.updated`, `entity.update.rejected` | SE | entity-manager | game-service (проекция), агенты роя, semantic-memory, страж | 1 | `entity.created` публикуют game-service/world-generator; entity-manager **не публикует** |
| `dice.rolled` | LR (или GE) | механика | replay, аудит, `session-report` | 1 | нет |
| `combat.decided` | GE | механика/Task-агент | entity-manager (через `entity.update.proposed`), персональный GM/нарратор, game-service, semantic-memory | 1 | `combat.started/ended/damage_dealt` — подписчики без издателя |
| `encounter.started`, `encounter.ended` | WE | GM региона / Task-агент | персональные GM, game-service, глобальный GM (сводка), semantic-memory | 1 | нет |
| `world.weather_changed`, `world.time_advanced`, `world.event_occurred` | WE | глобальный GM | GM регионов, персональные GM, semantic-memory | 1 | `world.weather_changed`, `world.time_tick` — подписчики без издателя |
| `region.event_occurred`, `npc.moved`, `npc.spawned` | WE | GM региона | глобальный GM, персональные GM, semantic-memory | 1 | `npc.action`, `npc.moved` — без издателя/контракта |
| `tick.fired`, `tick.aborted` | SE | планировщик роя | агент-адресат, replay, `session-report` | 1 | `time.syncTime` (legacy narrative-orchestrator) |
| `agent.spawned`, `agent.child_resolved`, `agent.stopped`, `agent.spawn_rejected`, `agent.blueprint_reloaded` | SE | рантайм роя | оператор/health, `session-report`, semantic-memory | 1 | `gm.created/deleted/merged/split` (legacy) |
| `llm.output`, `llm.output.rejected` | LR | конвейер агента (LLM-шлюз) | replay, страж (аудит), `session-report`, semantic-memory (только метаданные) | 1 | нет |
| `narrative.output` | NO | персональный GM / нарратор | game-service (доставка), semantic-memory | 1 | `narrative.generate` (legacy; game-service не обрабатывает) |
| `content.incident.recorded` | SE | конвейер (фильтр) | оператор, `session-report` | 1 | нет |
| `snapshot.created` | SE | каждый stateful-компонент | оператор, `session-report --audit` | 1 | нет |
| `world.laws.changed` | WE | автор (CLI) / механика пробоя (E-B) | все агенты, entity-manager, semantic-memory | 1 | нет (регистрация без обработчиков) |
| `world.law_breach.proposed / rejected / applied / review_decided / rolled_back` | WE | E-B | E-B | 1 | нет (регистрация без обработчиков) |
| `analytics.session.started / ended`, `analytics.turn.completed` | AE | game-service | `session-report` | 1 | нет |
| `analytics.consistency.violated` | AE | тест-харнесс, `session-report --audit`, страж (целевое) | `session-report` | 1 | `violation.detected` (ban-of-world, другой смысл) |
| `analytics.replay.completed` | AE | компонент восстановления | `session-report`, CI | 1 | нет |

Замкнутость: у каждого типа есть издатель и ≥ 1 потребитель; фантомные топики (`entity_actor_events`, `mechanical_results`, `entity.created` как топик, `world.metrics.*`, `reality.anomaly.detected`) в контракте отсутствуют. Legacy-типы (`player.moved`, `player.used_skill`, `gm.*`, `narrative.generate`, `violation.detected`, `time.syncTime`) допустимы только за флагом `agent_mode=off` и удаляются с ним.

### 2.3. Спецификации payload

Ниже — поля сверх общих (§2.1). Примеры сокращены; `…` = общие поля.

#### 2.3.1. Действия игрока (`player.*`) — издатель game-service, `actor_kind` сессии
```json
{ "…": "общие", "entity": { "entity": { "id": "player-A", "type": "player" }, "name": "Вася" },
  "scope": { "id": "solo:player-A", "type": "solo" },
  "action": { "type": "attack", "key_hash": "<hash action_key>" },
  "target": { "entity": { "id": "wolf-alpha", "type": "npc" }, "name": "Альфа-волк" },
  "encounter": { "entity": { "id": "enc-7", "type": "encounter" } },
  "round": { "seq": 3 },
  "session": { "id": "…" },
  "text": "…"  // только player.said, ≤ 500
}
```
`player.defended`: `cause: player | round_timeout`. `player.entered_region`/`left_region`: `target{region}`, `position{from, to}`, `cause: player | group_move` (при `group.*`-варианте не используется). Инвариант: **ни один агент не публикует `player.*`** (страж: `player_agency`).

#### 2.3.2. Группа (`group.*`) — издатель game-service
`group{entity{id,type:group}}`, `leader{entity{player}}`, `members[]{entity{player}, name, participation}`, `cause: create|join|leave|death|disband|group_move`, для `group.entered_region`/`left_region` — `target{region}`, `position{from,to}`, `by{entity{player лидер}}`.

#### 2.3.3. Раунд (`round.*`) — издатель game-service
`round.opened`: `scope`, `encounter`, `round{seq}`, `expected[]{entity{player}}`, `deadline_at`.
`round.closed`: `round{seq, close_reason: all_acted|timeout|explicit}`, `acted[]{entity{player}, event{id}}`, `auto_defended[]{entity{player}}`, `idle[]{entity{player}}`, `closed_at`, `replay`.

#### 2.3.4. Состояние (`entity.*`)
`entity.create.proposed`: `entity{entity{id,type},name}`, `attributes{…}` (полный начальный набор по типу), `cause`, `agent?`.
`entity.update.proposed`:
```json
{ "…": "общие", "proposal_id": "<uuid>",
  "changes": [
    { "entity": { "entity": { "id": "wolf-alpha", "type": "npc" } }, "expected_version": 12,
      "ops": [ { "op": "set", "path": "hp", "value": 7 }, { "op": "set", "path": "status", "value": "alive" } ] },
    { "entity": { "entity": { "id": "player-A", "type": "player" } }, "expected_version": 23,
      "ops": [ { "op": "append", "path": "inventory", "value": { "item_id": "item-1", "kind": "wolf-pelt", "name": "волчья шкура", "source": { "entity": { "id": "wolf-alpha", "type": "npc" }, "event_id": "…" } } } ] }
  ],
  "atomic": true,
  "cause": "combat|rest|move|loot|spawn|tick|group|create",
  "agent": { "id": "encounter-wolf:group:g-1", "level": "task", "blueprint": "encounter-wolf", "blueprint_version": "1.0" }
}
```
`ops.op` ∈ `set | inc | append | remove`; `atomic=true` — все изменения применяются вместе или отклоняются (нужно для инварианта позиции группы). `expected_version` опционален (без него — применение к текущей с проверкой инвариантов).
`entity.updated`: `entity{…}`, `version` (новая), `changed[]{path, old, new}`, `cause`, `proposal_id`, `applied_at`. Публикуется по одному на сущность; при пакетном применении — общий `proposal_id`.
`entity.created`: `entity{…}`, `version: 1`, `attributes{…}`.
`entity.update.rejected`: `proposal_id`, `reason: version_conflict | unknown_entity | level_violation | law_violation | invalid_op | dead_entity`, `entity?`, `details{expected_version, actual_version, invariant_id}`.

Правило владения (`level_violation`) — `data-model.md` §4; entity-manager проверяет `agent.level` и `cause`.

#### 2.3.5. Броски (`dice.rolled`) — издатель механика
`roll{index, formula, seed, result, natural}`, `purpose: hit|damage|flee|npc_hit|npc_damage|encounter_chance`, `roller{entity}`, `trigger{event}` (действие или `round.closed`), `replay`. `seed = hash(trigger.event.id, roll.index)` — функция хэша фиксируется архитектором (детерминированная, одна на проект).

#### 2.3.6. Бой (`combat.decided`) — издатель механика (в конвейере Task-агента)
```json
{ "…": "общие", "encounter": { "entity": { "id": "enc-7", "type": "encounter" } }, "round": { "seq": 3 },
  "action": "attack|flee|free_attack|npc_attack",
  "attacker": { "entity": { "id": "player-A", "type": "player" }, "name": "Вася" },
  "defender": { "entity": { "id": "wolf-alpha", "type": "npc" }, "name": "Альфа-волк" },
  "outcome": { "hit": true, "natural": 15, "damage": 3, "critical": false, "fumble": false, "target_dead": false, "success": null },
  "hp": { "defender_before": 7, "defender_after": 4, "defender_max": 10 },
  "rolls": [ { "event": { "id": "…" }, "index": 0 }, { "event": { "id": "…" }, "index": 1 } ],
  "rules_version": "0.1", "gm_path": "agent", "phase1_mode": "rules", "lod": "rule-only",
  "free_attack": false, "agent": { "…": "task" } }
```
Для `flee`: `outcome.success`, `outcome.threshold`, `living_enemies`. Изменение состояния — отдельным `entity.update.proposed` (с `trigger = combat.decided`).

#### 2.3.7. Встреча (`encounter.*`) — издатель GM региона (`started`), Task-агент (`ended`)
`encounter{entity}`, `participants[]{entity{player}}`, `npcs[]{entity{npc}, name}`, `region{entity}`, `scope` (игрока/группы); `encounter.ended`: `reason: npc_dead|players_out|abandoned`, `killer{entity}?`, `rounds: int`.

#### 2.3.8. Фоновые и региональные события (`world.*`, `region.*`, `npc.*`) — `actor_kind=system` при тике
`world.weather_changed {weather{from,to}}`, `world.time_advanced {time_of_day{from,to}, day}`, `world.event_occurred {kind, summary (ru, ≤ 300), affects[]{entity}}`; `region.event_occurred {kind, summary, affects[]}`; `npc.moved {entity{npc}, position{from,to}}`; `npc.spawned {entity{npc}, kind, position, spawned_by{agent, tick{event}}}`. Все — с `agent{level global|domain}`, `trigger{event{id, type: tick.fired}}` (фон) или `trigger{…player.*|world.*}` (реакция). Состояние меняется сопутствующим `entity.update.proposed`.

#### 2.3.9. Тик и жизненный цикл агентов (`tick.*`, `agent.*`) — издатель рантайм роя
`tick.fired {agent{…}, scope, tick{seq, mode: background|active, scheduled_at, fired_at, lod_allowed}, budget{window_calls, cap}, replay}`; `tick.aborted {tick{seq}, reason}`.
`agent.spawned {agent{…}, parent{id}, scope, ttl_expires_at?, lod}`, `agent.child_resolved {agent, parent{id}, reason: npc_dead|players_out|ttl|abandoned}`, `agent.stopped {agent, reason: ttl|shutdown|error}`, `agent.spawn_rejected {blueprint, scope, reason: max_instances|validation}`, `agent.blueprint_reloaded {blueprint, version_from, version_to}`.

#### 2.3.10. Record-replay LLM (`llm.output`, `llm.output.rejected`)
`llm.output` — поля `data-model.md` §7.2 (обязательны: `provider, model, params, prompt_hash, response_raw (кроме quarantined), response_hash, validation_status, correlation_id, caused_by, laws_version, agent, actor_kind, phase, attempt, latency_ms, tokens, cost_usd, gm_path, replay`; опционально `lod, tools_used, filter, error`).
`llm.output.rejected {llm_output{event{id}}?, reason: unknown_entity|player_agency|level_violation|schema_invalid|language|filter_blocked|filter_error|budget_exceeded|law_violation|other, element{index, type}?, entity{…}? (для unknown_entity), category? ("a"), budget{kind: background|turn|cloud, limit, window}?, agent, actor_kind, phase, attempt}`. При `budget_exceeded` `llm_output` отсутствует.

#### 2.3.11. Нарратив (`narrative.output`) — издатель персональный GM / нарратор
```json
{ "…": "общие", "scope": { "id": "group:g-1", "type": "group" },
  "recipients": [ { "entity": { "id": "player-A", "type": "player" } }, { "entity": { "id": "player-B", "type": "player" } } ],
  "text": "…", "generated_by": "llm|template", "fallback_reason": null,
  "kind": "turn|round|world_event|entry|death",
  "round": { "seq": 3 }, "llm_output": { "event": { "id": "…" } },
  "based_on": [ { "event": { "id": "…", "type": "combat.decided" } } ],
  "absence": { "since_at": "<ts>", "background_events_count": 3 },
  "background_refs": [ { "event": { "id": "…", "type": "world.weather_changed" } } ],
  "filter": { "applied": true, "status": "pass", "filter_version": "a-2026-09" },
  "locale": "ru", "agent": { "level": "task", "blueprint": "player-gm" }, "laws_version": "v1" }
```
Текст в шине — единственная копия для доставки; game-service копирует в `analytics.turn.completed` только счётчики и ID.

#### 2.3.12. Снапшот (`snapshot.created`)
`component: entity-manager|agent-runtime|game-service`, `snapshot{id, taken_at, cursor{<topic>: <offset|event_id>}, laws_version, state_hash, size_bytes}`.

#### 2.3.13. Законы и пробой (контракт в MVP-1; обработчики — E-B)
`world.laws.changed {laws{version_from, version_to, status: approved|pending_review|rolled_back}, created_by: author|breach, diff{added[], removed[]}, effective_from{round_boundary: true}}`.
`world.law_breach.proposed {breach{id}, initiator{kind: agent|author|player_indirect, agent?}, cause_event_ids[], strain, laws_removed[], laws_added[], canon_invalidated[], narrative_hook, laws_version_base}`.
`world.law_breach.rejected {breach{id}, reason: immutable_touched|contradiction|canon_conflict_missing|formula_change|unknown_entity|breach_pending|llm_unavailable}`.
`world.law_breach.applied {breach{id}, laws{version_from, version_to}, review{deadline_at}}`.
`world.law_breach.review_decided {breach{id}, decision: approved|rolled_back, reviewer_kind: human|timeout, decided_at}`.
`world.law_breach.rolled_back {breach{id}, laws{version_from, version_to}, retconned_fact_ids[]}`.
Решения G1: `immutable`-законы вселенной не пробиваются (→ `rejected reason=immutable_touched`); истечение окна → `approved reviewer_kind=timeout`.

#### 2.3.14. Аналитика (`analytics.*`) — по `metrics.md` §4.2 без изменений; ключ `world.entity.id`; не читаются replay.

#### 2.3.15. Инцидент контента (`content.incident.recorded`)
`incident{category: "a", filter_version, llm_output{event{id}}}`, `agent`, `scope` — без текста.

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

Белые списки уровней (MVP-1): `global` → `world.weather_changed, world.time_advanced, world.event_occurred` + `entity.update.proposed(world)`; `domain` → `region.event_occurred, npc.moved, npc.spawned, encounter.started` + `entity.*.proposed(region, npc, encounter)`; `task` (встреча) → `combat.decided` (через механику), `encounter.ended` + `entity.update.proposed(encounter, npc/player во встрече)`; `task/monitor` (персональный GM) → только `narrative.output`.

---

## 3. Контракт блупринта агента (три уровня роя + встреча)

**Формат:** один файл Markdown с YAML-frontmatter (метаданные и конфигурация) и секциями `## system`, `## phase1`, `## phase2`, `## tick`, `## canon` (текст). Чистый YAML допустим как альтернатива при том же наборе полей **(формат парсера — архитектор; поля — контракт)**. Совместимость с `shared/agent.AgentBlueprint`: существующие поля сохранены (`name, version, description, trigger, constraints, llm, tools, parent, ttl, phase1_prompt, phase2_prompt, type`); новые добавлены.

### 3.1. Поля

| Поле | Тип | Обяз. | Уровни | Описание / ограничения |
|---|---|---|---|---|
| `name`, `version`, `description` | string | да | все | `name` уникален; `version` semver |
| `level` | enum `global\|domain\|task\|monitor` | да | все | = `AgentLevel` |
| `role` | enum `global-gm\|region-gm\|city-gm\|personal-gm\|encounter` | да | все | продуктовая роль |
| `scope_binding` | `{type: world\|region\|solo\|group, id?: string, pattern?: string}` | да | все | `global` → `world`; `domain` → `region` c `id`; `personal-gm` → `solo` (pattern `solo:*`); `encounter` → scope игрока/группы |
| `parent` | `{name, instance?}` | да, кроме `global` | domain, task, monitor | родитель существует; `personal-gm.parent` = `region-gm` региона текущей позиции (динамически) |
| `trigger` | `{type: timer\|event, event_name?, conditions[]?, intervals?: {idle: duration, active: duration}}` | да | все | `timer` требует `intervals.idle`; `active` — для `domain` при игроках; `event` — `event_name` из реестра |
| `ttl` | duration | task/monitor | | не применяется к `global`/`domain` (игнорируется с предупреждением) |
| `constraints.max_instances` | int | да | все | 1 для `global`/`domain`/`personal-gm` на scope |
| `constraints.priority` | int | нет | | |
| `llm.phase1` | `{model, temperature, max_tokens, thinking: bool, schema_ref}` | да для агентов с Phase 1 | domain, task | MVP-1: `phase1.mode: rules` допустим (без LLM) |
| `llm.phase2` | `{model, temperature, max_tokens, thinking, schema_ref}` | personal-gm, encounter (нарратор) | | модель нарратива |
| `llm.tick` | `{model, lod_default: rule-only\|basic, max_tokens, schema_ref}` | global, domain | | модель малая |
| `llm.retries` | int | нет | | по умолчанию 2 |
| `llm.fallback` | `template\|rules` | да | | |
| `allowed_event_types[]` | list | да | все | белый список публикуемых типов (§2.4) |
| `owned_entity_types[]` | list | да | все | `world` / `region, npc, encounter` / `encounter, npc, player(in_encounter)` / `[]` |
| `tools[]` | `{name, owner}` | нет | | зарегистрированы в реестре инструментов |
| `laws_ref` | string | да | global, domain | `laws/dark-forest-world@v1` |
| `rules_ref` | string | да | domain, encounter | `rules/dark-forest.yaml` |
| `description_text` (секция `## description`) | text | да | domain | авторское описание региона (слой `authored`) |
| `canon[]` (секция `## canon`) | list текстов | нет | global, domain | факты `authored` |
| `npc_table[]` | `{npc_id, kind, name, stats_ref, count, spawn: {on: region_init\|tick, chance?: formula}}` | да | domain | |
| `respawn_ttl` | duration | да | domain | 24h по умолчанию |
| `encounter` | `{detect_on: tick, perception: all\|radius, chance?: formula, child_blueprint: encounter-wolf\|inline}` | да | domain | |
| `background_events[]` | `{kind, weight, summary_template, ops[]?}` | да | global, domain | таблица LOD 1 без LLM |
| `round` | `{timeout: 60s, idle_after_missed: 2}` | да | encounter (или domain) | параметры раунда группы |
| `budget` | `{background_calls_per_hour_world: 4}` | да | global (мир) | общий потолок фона |
| `absolute_limits_ref` | string | да | personal-gm, encounter | ссылка на конфиг запретов (короткая позитивная формулировка попадает в `## system`) |
| `invariants[]` | list `{id, check}` | да | global (мир) | инварианты NFR-020 как `laws@v1` |
| `locale` | string | да | все | `ru` |
| `prompts` (секции `## system`, `## phase1`, `## phase2`, `## tick`) | text с плейсхолдерами `{world.weather}`, `{region.description}`, `{events}`, `{state}`, `{absence}`, `{laws_version}`, `{player.name}` | по фазам | | плейсхолдеры — из фиксированного словаря; неизвестный — ошибка валидации |

### 3.2. Валидация (ошибки с файлом, полем, причиной)
Обязательные поля по уровню; `trigger.timer` без `intervals.idle`; `parent` не найден; `tools[].name` не зарегистрирован; `allowed_event_types` содержит тип вне реестра или вне разрешённых уровню; `owned_entity_types` шире уровня; `rules_ref`/`laws_ref` не найдены; модели не заданы для используемых фаз; плейсхолдер вне словаря; `ttl` у `global`/`domain` — предупреждение; `max_instances ≠ 1` для `global`/`domain`/`personal-gm` — ошибка.

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
npc_table:
  - { npc_id: wolf-alpha, kind: wolf, name: "Альфа-волк", stats_ref: wolf, count: 1, spawn: { on: region_init } }
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
level: task            # или monitor — решение архитектора
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

### 3.4. Схема структурированного ответа Phase 2 (`schemas/narrative.json`, логически)
`{ text: string (ru), background_refs: [event_id], mentions: [entity_id], tone?: enum }` — `background_refs` обязателен при наличии `absence` в контексте (может быть пустым). Схема Phase 1 (`decision`) в MVP-1 не используется (`mode: rules`); схема тика: `{ events: [ { type ∈ allowed_event_types, summary, ops[] } ] }`.
