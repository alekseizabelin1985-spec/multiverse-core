# ADR-007: Формат контрактов событий — конверт `meta` для трассы и происхождения, реестр типов с JSON-схемами, версии, топики и retention

Статус: предложено (к утверждению на G2) · Дата: 2026-09-09 · Автор: system-architect#1
Связи: OQ-A-08 (реестр/узаконивание LLM-событий), OQ-A-09 (формат контрактов), OQ-D-05 (scope-типы), OQ-A-07 (топик `analytics_events`), OQ-A-19 (новый: retention); FR-033, FR-036, FR-084, FR-085, FR-088, FR-126; NFR-011, NFR-013, NFR-031, NFR-036, NFR-091; `api-contracts.md` §0, §2; `metrics.md` §4.1; аудит §5 (фантомные топики, произвольные типы от LLM).

## Контекст

As-is `eventbus.Event{id, type, timestamp, source, world, scope, payload, relations}`; нет `correlation_id`, версий схем, реестра типов; LLM публикует произвольные типы в `world_events`; 6 фантомных топиков; `scope_management` мёртв. Аналитик предложил обязательные поля payload (`schema_version`, `correlation_id`, `trigger{event}`, `actor_kind`, `agent{…}`, `replay`) и оставил архитектору: конверт vs payload, формат схем, новые топики (`analytics_events`, `llm_records`), Schema Registry.

## Рассмотренные варианты

| Вопрос | Вариант | Плюсы | Минусы |
|---|---|---|---|
| Где сквозные поля | в `payload` (предложение PA/SA) | всё в одном месте для `event.Path()` | каждый издатель обязан копировать 6 полей; payload для LLM засоряется служебным; проверка «не забыли» — только тестами |
| | **в конверте `meta`** | копирование одной функцией `eventbus.Derive(parent, …)` в рантайме; payload — доменный; прецедент — `world`/`scope` уже в конверте | старые потребители, читающие `payload.correlation_id`, — их нет |
| Схемы | **JSON Schema 2020-12 в репозитории, реестр в `shared/contracts`** | версионируется с кодом; валидация при публикации библиотекой; без сервиса | нет внешнего реестра для чужих клиентов (их нет) |
| | Redpanda Schema Registry | стандарт | +зависимость, +клиент; один издатель-библиотека |
| | protobuf | строгие типы | смена сериализации всего as-is; LLM-промпты оперируют JSON |
| HTTP API | **OpenAPI 3.1 файл** | документ = контракт; генерация типов по желанию | ручная синхронизация с кодом (тест сверяет) |
| Топики | как as-is (6) | ничего не менять | аналитика в replay-потоке; тяжёлые `llm.output` в `system_events` |
| | **+ `llm_records`, + `analytics_events`, + `dead_letters`, − `scope_management`** | retention и потребители разделены; replay не читает аналитику | +3 строки в `redpanda-init` |

## Решение

1. **Конверт** (`shared/eventbus.Event`, обратная совместимость по JSON — новые поля добавляются):
   ```json
   { "id": "uuid", "type": "combat.decided", "timestamp": "…Z", "source": "core/swarm",
     "world": {"entity": {"id": "dark-forest-world", "type": "world"}},
     "scope": {"id": "solo:player-A", "type": "solo"},
     "meta": { "schema_version": 1, "correlation_id": "<id корня: действие или tick.fired>",
               "causation_id": "<id непосредственной причины>", "causation_type": "round.closed",
               "actor_kind": "human|ci|sim|system", "agent": {"id": "encounter-wolf:group:g-1", "level": "task", "blueprint": "encounter-wolf", "blueprint_version": "1.0"},
               "replay": false, "locale": "ru", "gm_path": "agent" },
     "payload": { "…доменные поля по схеме типа…" }, "relations": [] }
   ```
   `meta.agent` обязателен у событий, изданных рантаймом роя; у корневых событий `correlation_id = id`, `causation_id` отсутствует. Доменные поля (`laws_version`, `phase`, `round`, `encounter`, `entity`, `target`) остаются в `payload`. Библиотека: `eventbus.NewRoot(type, source, world, scope, actorKind, payload)`, `eventbus.Derive(parent Event, type, source, payload, opts…)` копирует `meta`; `Event.CorrelationID()`, `Event.Path()` без изменений. `payload.trigger{event}` из предложения аналитика **не используется** (дубль `meta.causation_*`).
2. **Реестр типов** `shared/contracts`: таблица `type → {topic, schema_version, schema file, publishers[], consumers[], since}`; `schemas/events/<type>.v<n>.json` (JSON Schema 2020-12; общие определения в `schemas/events/_common.json`: `EntityRef`, `ScopeRef`, `AgentRef`, `Money`); `Publish` валидирует конверт + payload по схеме — неизвестный тип или невалидный payload → ошибка публикации (FR-036 Must с EPIC-001, а не Should); типы `world.law_breach.*`, `world.laws.changed` зарегистрированы без обработчиков; `mvctl contracts check` сверяет издателей/потребителей по коду (ловит фантомы). Legacy-типы (`player.moved`, `player.used_skill`, `gm.*`, `narrative.generate`, `violation.detected`, `time.syncTime`) — в реестре с пометкой `deprecated, gm_path=legacy`, удаляются вместе с профилем `legacy`.
3. **Версионирование**: `meta.schema_version` — целое с 1 на тип; совместимые изменения (добавление опциональных полей) — та же версия; несовместимые — `+1`, потребители обязаны принимать `n` и `n-1` один инкремент; изменение контракта — только через архитектора (`contracts.md`, «Запросы на изменение»).
4. **Топики и retention** (задаются в `redpanda-init`: `rpk topic create <t> -p 1 -r 1 -c retention.ms=…`):

   | Топик | Типы | Retention | Читают в replay |
   |---|---|---|---|
   | `player_events` | `player.*`, `group.entered_region`, `group.left_region` | 30 дн. | да |
   | `game_events` | `group.*` (кроме перемещений), `round.*`, `combat.decided`, `dice.rolled` | 30 дн. | да |
   | `world_events` | `world.*`, `region.*`, `npc.*`, `encounter.*`, `world.laws.*`, `world.law_breach.*` | 30 дн. | да |
   | `system_events` | `entity.*`, `tick.*`, `agent.*`, `snapshot.created`, `content.incident.recorded`, `config.*` | 30 дн. | да |
   | `narrative_output` | `narrative.output` | 30 дн. | да (доставка) |
   | `llm_records` | `llm.output`, `llm.output.rejected` | 90 дн. | да (record-replay) |
   | `analytics_events` | `analytics.*` | 180 дн. | **нет** |
   | `dead_letters` | любой (обёрнут `{original, error, consumer}`) | 30 дн. | нет |

   `scope_management` и фантомные топики удаляются. Retention 30 дней ≥ окна FR-127 и ≥ 3 интервалов снапшота; оценка объёма: ≈ 1 000 событий/сессия × 2 КБ ≈ 2 МБ/сессия — незначимо; `llm_records` ≈ 200 × 4 КБ ≈ 1 МБ/сессия (OQ-A-19 — подтвердить сроки/диск).
5. **Ключ и порядок**: ключ сообщения = `world.entity.id` (`global` для событий без мира); одна партиция на топик в MVP-1 → строгий порядок внутри топика; между топиками порядок не гарантируется — потребители, которым нужна причинность (`combat.decided` → `entity.updated`), ждут по `meta.correlation_id`/версии, а не по времени.
6. **Reader**: `MinBytes=1`, `MaxWait=100 ms` (as-is 10 КБ/1 с недопустимо для NFR-001); consumer-группы именуются `{process}.{context}`; at-least-once; дедуп по `event.id` в потребителе (LRU 10k + курсор снапшота); ошибка обработчика → повтор ×3 с backoff → `dead_letters`.
7. **Scope-типы** (OQ-D-05): `solo | group | region | world` в MVP-1; `city | quest` зарезервированы в реестре; `location`, `player`, `zone` — не допускаются.
8. **HTTP**: `api/gateway.openapi.yaml` (3.1), `api/memory.openapi.yaml`; тест сверяет маршруты с кодом; коды ошибок — §1.6 api-contracts.

## Последствия

- Позитивные: трасса без усилий издателя; фантомные типы невозможны (публикация падает); аналитика вне replay; retention по классам данных.
- Негативные: правка `shared/eventbus` затрагивает все активные контексты (все они переписываются — момент удачный); схемы нужно писать (≈ 45 типов, шаблонизируемо).
- Что придётся сделать: EPIC-001 — конверт, реестр, схемы всех типов §2.2 api-contracts, `redpanda-init`, `mvctl contracts check`, миграция `api-contracts.md` §0/§2.1 (пометка «поля перенесены в `meta`» — задача system-analyst на планировании).

## Дополнение 2026-09-09 (сведение A3 шаг 4)

1. **Retention — решение пользователя (OQ-A-19)**: 30 дней доменные топики, 90 дней `llm_records`, 180 дней `analytics_events`; полные промпты — в MinIO по флагу `MV_LLM_STORE_PROMPTS` (по умолчанию выкл., ILM 30 дней). Оценка диска `infrastructure.md` §5.1 (< 200 МБ за окно) подтверждает.
2. **`segment.ms` (п. 4 уточнение)**: все топики создаются с `-c segment.ms=86400000` (1 сутки) — без этого при малом трафике активный сегмент не закрывается и retention не срабатывает годами; `llm_records` дополнительно `max.message.bytes=4194304`. Повторный запуск `redpanda-init` — `rpk topic alter-config` (идемпотентно).
3. **Библиотека схем (п. 2 уточнение)**: JSON Schema 2020-12 валидируется `github.com/santhosh-tekuri/jsonschema/v6` (v6.0.3); as-is `xeipuuv/gojsonschema` (draft-07) в модуль не переносится.
4. **Валидация при чтении и политики топиков (SEC-16)**: `Subscribe` валидирует конверт/payload по схеме (`MV_BUS_VALIDATE_ON_READ`, по умолчанию `true`; невалидное → `dead_letters`); реестр несёт политику топика: `player_events` принимает только `meta.actor_kind ∈ human|ci|sim` без `meta.agent`; события роя (`tick.*`, `agent.*`, `llm.*`, `narrative.output`, `combat.decided`) — `meta.agent` обязателен. Проверяется и при публикации, и при чтении (защита в глубину при PLAINTEXT-шине).
5. **Журнал как интерфейс (C-01 v1.1)**: `eventbus.Journal{ReadRange, Tail, End}` и `PositionFromContext` — часть библиотеки шины; consumer group используют только «живые» подписки, догон с курсора снапшота — через `Journal` (ADR-011 п. 4, C-14).
6. **Новый тип** `config.cloud_enabled` (топик `system_events`, издатель `llm`, ADR-005 дополнение п. 3); `snapshot.created` получает `component=gateway` (C-14).
