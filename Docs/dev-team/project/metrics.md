# Метрики продукта: multiverse-core

**Инициатива:** PROJECT · **Версия:** 0.2 · 2026-09-09 · продуктовый аналитик (dev-team)
**Статус:** согласовано с `vision.md` v1.1 (видение подтверждено пользователем, G1)

Источники: `project/vision.md` v1.1 (S1–S14, «Рой GM-агентов», D7/D8),
`requirements/prd.md`, `requirements/nfr.md` (в т. ч. раздел «Что измерить
первым»), `requirements/user-stories.md`, `architecture/audit-facts.md` (§4–5:
топики, reality-monitor, структура события), `journal.md` (решения пользователя,
раунд 5 и «ВИДЕНИЕ ПОДТВЕРЖДЕНО ЯВНО»). Идентификаторы метрик и событий —
английские, текст — русский.

**Changelog**

- **0.2 (2026-09-09)** — точечный пересмотр под vision v1.1: (1) гипотеза H5
  «мир тратит LLM только там, где есть игроки» и метрика `idle_scope_llm_calls`
  **удалены** (D8: фоновая жизнь — в MVP); вместо них — метрики фоновой жизни
  §3.5 (`background_events_per_region_per_day`,
  `background_llm_calls_per_region_per_hour`, `background_event_surfaced_rate`,
  `world_break_from_background`) и гипотеза H5'; (2) метрики роя GM (D7):
  `llm_calls_per_turn_by_level`, `narrative_latency_group_p95_ms`,
  `cross_level_event_rate`; (3) критерий S14 добавлен в §2.1 (пороги — после
  замера, решение архитектора); (4) определение сессии и числа S7 (8/3 за
  4 недели) переведены из допущений в **решённые** (G1); (5) S4 — целевое
  железо RTX 4090 24 ГБ, две модели Qwen3 одновременно, фон включён, отдельный
  порог для группы; (6) события: поля `actor_kind=system` и
  `agent{level,blueprint}` на `llm.output` и фоновых доменных событиях,
  `narrative.agent_level` и `absence{…}` в `analytics.turn.completed`; нового
  события для фоновых тиков **не вводится** (§4.1 п. 8); (7) §6.1, §7 (B1, B8),
  §8 (п. 14–17), §10 — обновлены соответственно. Остальное — без изменений.
- **0.1 (2026-09-09)** — первая версия к G1.

## 0. Как читать

- **Ход (turn)** — одно действие игрока, принятое game-service, с механическим
  результатом и нарративом или шаблоном (глоссарий). Единица большинства метрик.
- **Сессия (session)** — последовательность ходов в одном scope (`solo` или
  `group`) от первого принятого действия до выхода всех участников / смерти /
  ошибки / простоя ≥ 30 мин (**решено на G1**, vision v1.1; см.
  `analytics.session.*`). Сессии делятся по `actor_kind`: `human` (живые игроки
  через бота), `ci` (автосценарий), `sim` (нагрузочные боты). В S7 считаются
  только `human` (решено на G1).
- **Уровень роя (agent_level)** — уровень GM-агента Agent GM Core, породившего
  событие: `global` (глобальный GM мира), `domain` (региональный GM; в целевом —
  и городской), `task` (персональный GM игрока, агенты встреч/квестов),
  `object`, `monitor` (целевое). Значения — `AgentLevel` из
  `shared/agent/agent_types.go`; продуктовую роль внутри уровня различает
  `agent.blueprint` (например, `domain-dark-forest`, `personal-gm`, `encounter-wolf`).
- **Фоновый тик (background tick)** — запуск глобального или регионального GM
  по расписанию (trigger `gm.tick`) без действия игрока. **Фоновое событие** —
  доменное событие (`world.*`, `region.*`, `entity.updated`, `dice.rolled`),
  порождённое фоновым тиком; помечается `actor_kind = system` (§4.1 п. 8).
  Фоновые тики **не образуют ходов и сессий**; их стоимость и результат
  считаются отдельно (§3.5) и входят в S8 (ноль внешних затрат) и S14.
- **Поломка мира** — по определению из `vision.md` (противоречие числового
  состояния, потерянная/задвоенная сущность, нарратив против канона, ручная
  правка состояния). Машинно ловится событием `analytics.consistency.violated`
  (severity `break`), вручную — записью в `incidents.csv` (§6).
- **Текущее значение** для всех метрик — **н/д**: ни один игровой сценарий не
  проходит сквозь систему, reality-monitor неработоспособен, у TimescaleDB нет
  клиента (audit-facts §4, §7). Колонка «Текущее» в таблицах опущена; первый
  замер — по протоколу §7.
- **«после замера»** — порог фиксируется после первого замера на целевом железе
  (**RTX 4090 24 ГБ VRAM, 128 ГБ RAM, i9-13900**; обе модели Qwen3 — 8b для
  решений/фона и 30b-a3b/32b для нарратива — загружены одновременно; фоновые
  тики включены) по протоколу §7; до этого — ориентир, не критерий приёмки.
  Пороги S14 (интервал, потолок фоновых вызовов) фиксирует **архитектор** после
  замера B8 (решение G1).
- Все метрики выводятся из **событий шины** (доменных `player.*`, `dice.rolled`,
  `combat.decided`, `entity.updated`, `world.*`/`region.*` фоновых, `llm.output`,
  `llm.output.rejected`) плюс пяти событий `analytics.*` (§4) и двух CSV
  оператора (§6). Внешние ID игроков, свободный текст игрока и текст нарратива
  в метрики **не попадают** (BR-07).

---

## 1. Северная звезда

**`clean_human_turns_weekly`** — число ходов, сыгранных живыми игроками
(`actor_kind = human`) в сессиях **без поломки мира**, за неделю.

- Формула: `Σ turns_count` по `analytics.session.ended` за 7 дней, где
  `actor_kind = human` и `world_break_count(session) = 0`.
- Почему эта: объединяет «в мир играют» (ходы, а не сессии — длина сессии
  тоже сигнал) и «мир держится» (ходы из сломанных сессий не засчитываются).
  Прямо ведёт к S7 и цели 1 видения; не требует внешней аудитории.
- Ограничители (guardrails), без которых рост NSM ничего не значит:
  `world_break_count` (§2, = 0), `llm_cost_per_turn` (§3.4, = 0 в базовой
  конфигурации), `unanswered_action_ratio` (= 0),
  `background_llm_calls_per_region_per_hour` (§3.5, ≤ потолка конфигурации —
  фон не должен «съесть» бюджет на ходы).
- Цель: после готовности MVP-1 — ≥ 8 сессий за 4 недели (S7, решено G1) при
  `world_break_count = 0`; численная цель по ходам — после первых двух недель
  сессий (ориентир: ≥ 60 ходов/нед ≈ 2 сессии × 30 ходов).
- Периодичность: неделя; считается из `ops/metrics/sessions.csv` (§6).

---

## 2. Метрики успеха по критериям S1–S14

Формат: `id` · определение · формула · источник · цель · периодичность.
Ссылки на события — §4. «CI-прогон» = автосценарий на записанных `llm.output`
(NFR-061/062); «стенд» = живой прогон с LLM на целевой машине (§0, «после
замера»).

### 2.1. Приёмка MVP-1 (S1–S10, S14)

| Критерий | Метрика (id) | Определение и формула | Источник | Цель | Когда |
|---|---|---|---|---|---|
| S1 | `e2e_solo_pass` | Автосценарий соло, 30 ходов: `pass = unanswered_actions = 0 ∧ state_divergence_count = 0 ∧ consistency_violations(break) = 0 ∧ service_panics = 0` | отчёт CI + `analytics.session.ended` (`actor_kind=ci`) + `analytics.consistency.violated` | pass в каждом прогоне на `main` | каждый CI-прогон |
| S1, S2, S7 | `world_break_count` | Число поломок мира в сессии = `count(analytics.consistency.violated, severity=break)` + `count(incidents.csv, kind=world_break)` за сессию | события + `incidents.csv` | 0 | сессия |
| S1, S2 | `state_divergence_count` | Расхождения снапшота с агрегатом лога по HP / инвентарю / позиции / статусу после сессии; = `count(analytics.consistency.violated, code=state_divergence)` | сверка в тесте / CLI `session-report --audit` | 0 | сессия (CI: каждый прогон; human: по требованию) |
| S1, NFR-012 | `unanswered_action_ratio` | `count(turn.completed, status∈{timeout,error}) + orphan_actions` / `count(player.* действий)`, где `orphan_actions` — действия без `turn.completed` через 60 с | `player.*` (game-service) + `analytics.turn.completed` | 0 | сессия |
| S1, NFR-012 | `service_panics` | Число паник и необработанных ошибок в логах всех сервисов за сессию (`level=error` без `handled=true`, `panic:`) | структурированные логи (NFR-033) | 0 | сессия |
| S2 | `e2e_group_pass` | Автосценарий группы из 3, 30 ходов: то же, что `e2e_solo_pass`, плюс `group_view_consistency = 1` и `state_divergence_count = 0` при одновременных атаках (US-007) | CI | pass | каждый CI-прогон |
| S2 | `group_view_consistency` | Доля групповых ходов, в которых все участники scope получили результат и нарратив с одним и тем же `event_id`: `count(turn.completed, scope.type=group, delivery.delivered_count = delivery.recipients_count ∧ delivery.result_event_id одинаков)` / `count(turn.completed, scope.type=group)` | `analytics.turn.completed` | 1.0 | сессия |
| S3 | `replay_llm_calls` | Число вызовов LLM за время replay = `replay.llm_calls` | `analytics.replay.completed` (мок-провайдер со счётчиком) | 0 | каждый тест восстановления |
| S3 | `replay_new_dice` | Новых `dice.rolled` за время replay = `replay.dice_rolled_new` | `analytics.replay.completed` | 0 | каждый тест восстановления |
| S3, NFR-010 | `recovery_state_identical` | `replay.state_hash_after == state_hash_before` (хэш состояния scope по HP/инвентарю/позиции/статусу всех сущностей) | `analytics.replay.completed` | true | каждый тест восстановления |
| S3, NFR-010 | `recovery_time_s` | RTO: от старта сервисов до первого успешного `turn.completed` после рестарта = `replay.duration_ms` + время старта | `analytics.replay.completed` + `/health` | ≤ 120 с (после замера) | каждый тест восстановления |
| S3, NFR-061 | `replay_determinism` | Повторный прогон записи даёт побайтово идентичную последовательность `dice.rolled`, `combat.decided`, `entity.updated` **и фоновых `world.*`/`region.*`** (`actor_kind=system` входят в запись наравне с ходами): `replay.events_hash_match` | `analytics.replay.completed` | true | каждый CI-прогон |
| S4, NFR-001 | `mechanics_latency_p95_ms` (и p50) | `timings.mechanics_at − timings.received_at` по ходам со статусом `ok|degraded`, где был Phase 1; перцентили за сессию. **Условия стенда S4:** RTX 4090, обе модели загружены, фоновые тики включены | `analytics.turn.completed` | ≤ 500 мс (после замера B1) | сессия; стенд |
| S4, NFR-002 | `narrative_latency_p95_ms` (и p50) | `timings.narrative_at − timings.mechanics_at` по ходам с `generated_by=llm` (ходы без Phase 1 — от `received_at`); шаблоны исключаются; **только `scope.type=solo`** (группа — отдельная метрика ниже) | `analytics.turn.completed` | ≤ 5 000 мс соло, локальная модель, фон включён (после замера B1) | сессия; стенд |
| S4, NFR-002, D1/D7 | `narrative_latency_group_p95_ms` (и p50) | То же для `scope.type=group`: `timings.narrative_at` — момент доставки **последнему** участнику; в группе из 3 на ход работают до трёх персональных GM + региональный, поэтому порог **отдельный** | `analytics.turn.completed` | порог для группы из 3 — **после замера B1** (ориентир: не хуже соло более чем в 2 раза; число — архитектор) | сессия; стенд |
| NFR-003 | `ack_latency_p95_ms` | `timings.acked_at − timings.received_at` | `analytics.turn.completed` | ≤ 300 мс | сессия |
| S5, BR-12 | `legacy_gm_path_share` | Доля ходов, обработанных legacy `GMInstance`: `count(turn.completed, turn.gm_path=legacy)` / `count(turn.completed)` | `analytics.turn.completed` | 0 к концу MVP-1 (флаг снят — проверка кода) | сессия |
| S5 | `gm_instances_active` | Число живых legacy-GM при `agent_mode=on` (лог/метрика narrative-orchestrator) | логи | 0 | стенд |
| S6 | `region_added_go_diff` | Число изменённых `.go`-файлов при добавлении второго региона по инструкции (`git diff --stat -- '*.go'`) | ручной сценарий US-015 | 0 | при приёмке |
| S7 | `human_sessions_4w` | `count(analytics.session.ended, actor_kind=human)` за 4 недели после готовности MVP-1; сессия — по определению §0 (простой ≥ 30 мин) | `sessions.csv` | ≥ 8 (**решено G1**) | неделя |
| S7 | `human_group_sessions_4w` | то же при `scope.type=group` и `players_count ≥ 2` | `sessions.csv` | ≥ 3 (**решено G1**) | неделя |
| S7 | `manual_fix_sessions` | Число сессий с записью `incidents.csv` `manual_fix=true` | `incidents.csv` | 0 | неделя |
| S8, NFR-050 | `cloud_llm_calls` | `count(llm.output, provider ≠ ollama)` за период в базовой конфигурации, **включая фоновые вызовы** (`actor_kind=system`) — считается за сессию и за сутки (фон идёт и без сессий) | `llm.output` | 0 | сессия; сутки |
| S9, NFR-041 | `external_id_leaks` | Число вхождений известных тестовых внешних ID (Telegram/Discord ID, ник) в топиках, снапшотах, индексах, логах, промптах | тест US-009 (grep по хранилищам) | 0 | каждый CI-прогон с интеграцией |
| S9, NFR-045 | `ai_notice_coverage` | Доля `player_id` с проставленной датой уведомления об ИИ в хранилище связок: `count(links, notice_shown_at ≠ null)` / `count(links)` | хранилище связок game-service (без шины) | 1.0 | при приёмке |
| S10, NFR-040 | `secrets_in_index` | Число находок сканера секретов в `git ls-files` | gitleaks / аналог в CI | 0 | каждый коммит |
| S14 (фон) | `background_events_between_sessions` | Число фоновых событий региона между двумя сессиями одного игрока: `count(world.*, region.*; actor_kind=system; agent.level ∈ {global, domain}; scope.id = регион; timestamp ∈ (prev_session.ended_at, next_session.started_at))` при интервале ≥ `S14.gap_hours` **и отсутствии активной сессии в регионе** в момент события | фоновые доменные события × `analytics.session.*` | ≥ 1 (**пороги после замера**: интервал — предложение PO 6 ч; итог — архитектор) | автосценарий S14 (ускоренные тики); стенд — каждая пара сессий |
| S14 (фон) | `background_event_surfaced_rate` | Доля фоновых событий из предыдущей строки, отражённых в нарративе входа: `count(absence.surfaced_event_ids)` / `count(background_events_between_sessions)` по ходу `action_type=enter` следующей сессии; MVP-1: подтверждается человеком по чек-листу (`detected_by=human`) | `analytics.turn.completed` (`absence{…}`, §4.2) + чек-лист | ≥ 1 событие отражено (`surfaced ≥ 1`); доля — измеряем, ориентир ≥ 0,5 после замера | автосценарий S14; стенд |
| S14 (фон), S8 | `background_llm_calls_per_region_per_hour` | см. §3.5 — фоновый расход не превышает потолок конфигурации | `llm.output` | ≤ `background.llm_calls_cap` (**после замера B8**; предложение PO — 1) | час; сутки |
| S14 (фон), S1/S2 | `world_break_from_background` | см. §3.5 — ни одно фоновое событие не поломало мир | `analytics.consistency.violated` + `incidents.csv` | 0 | сутки; автосценарий S14 |

### 2.2. Целевое видение (S11–S13; не приёмка MVP-1)

| Критерий | Метрика (id) | Определение и формула | Источник | Цель |
|---|---|---|---|---|
| S11, NFR-025 | `law_breach_events` | `count(world.law_breach.proposed)` за мир за период | событие пробоя (контракт FR-041) | ≥ 1 задокументированный; ≤ N/мир/период (N автора) |
| S11 | `law_breach_reviewed_ratio` | Доля пробоев с итоговым статусом ревью (`approved|rolled_back`) в срок: `count(review_status ≠ pending)` / `count(law_breach_events)` | журнал версий законов | 1.0 |
| S11 | `law_breach_applied_ok` | Для утверждённого пробоя: в следующих ≥ 10 ходах нет `consistency.violated` с `layer=law` по старой версии закона | `analytics.consistency.violated` | true |
| S12 | `autonomous_actor_actions` | Число событий от Entity-Actor без действия игрока-причины (`trigger.event.type ∉ player.*`), отражённых в `entity.updated` и нарративе | события `entity.actor.*` | ≥ 1 в сценарном тесте |
| S12, FR-115 | `rule_evolution_reviewed_ratio` | Доля предложений правил со статусом ревью человека | журнал правил | 1.0 |
| S13 | `second_world_generated` | Мир второго жанра собран из онтологий и проходит цикл US-002 в своём регионе | ручной сценарий | true |

---

## 3. Качественные метрики «живого мира»

Все считаются за сессию (и агрегируются за неделю), если не указано иное.

### 3.1. Консистентность

| Метрика (id) | Определение и формула | Источник | Цель |
|---|---|---|---|
| `consistency_violations_per_session` | `count(analytics.consistency.violated)` с разбивкой по `violation.code`, `layer`, `severity` | `analytics.consistency.violated` | `break` = 0; `warn` — измеряем |
| `invariant_violations` (NFR-020) | Подмножество с `detected_by=invariant_check` (инварианты блупринта: «мёртвый не действует», `HP ∈ [0, max]`, одна сущность — одна позиция и т. п.) | то же | 0 |
| `narrative_mechanics_mismatch_per_100` (NFR-023) | `count(violated, code=narrative_contradiction, layer=state)` / turns × 100; в MVP-1 — чек-лист человека по выборке 100 ходов, `detected_by=human`; далее — LLM-судья | то же + ручная разметка | ≤ 5 (после замера) |
| `canon_contradictions_per_100` (NFR-024) | `count(violated, code=narrative_contradiction, layer=canon)` / turns × 100 | то же | после замера |
| `guardian_rejections_per_100` | Отклонённые выводы LLM на 100 ходов с разбивкой по `reason`: `unknown_entity` (FR-034), `player_agency` (BR-06), `schema_invalid`, `language` (NFR-090), `filter_error` (FR-050), `budget_exceeded` (FR-072) | `llm.output.rejected` | измеряем; `unknown_entity` и `player_agency`, дошедшие до состояния, = 0 |
| `llm_valid_first_try_ratio` (NFR-022) | По фазам: `count(llm.output, attempt=1, validation_status=valid)` / `count(llm.output, attempt=1)` | `llm.output` | Phase 1 ≥ 0.90, Phase 2 ≥ 0.85 (после замера) |
| `narrative_language_reject_ratio` (NFR-090) | `count(llm.output.rejected, reason=language)` / `count(llm.output, phase=narrative)` | `llm.output.rejected`, `llm.output` | ≤ 0.02 |
| `memory_consistency_spot_checks` (US-005) | Доля пройденных ручных проверок памяти после сессии (волк не «ожил», рана не «зажила», трофей на месте): `passed / checked` | чек-лист исследователя (3–5 пунктов) | 1.0 |

### 3.2. Надёжность и деградация

| Метрика (id) | Определение и формула | Источник | Цель |
|---|---|---|---|
| `llm_fallback_rate` | Доля ходов с шаблонным нарративом при включённой LLM: `count(turn.completed, generated_by=template ∧ llm.enabled)` / turns; разбивка по `fallback_reason` (`timeout`, `unavailable`, `invalid_after_retries`, `budget`, `filter_error`) | `analytics.turn.completed` | измеряем; ориентир ≤ 0.05 после замера |
| `degraded_mode_success_ratio` (NFR-015) | При выключенной LLM: `count(turn.completed, status∈{ok,degraded})` / actions | тест US-004 | 1.0 |
| `phase2_queue_depth_max` (NFR-005) | Максимальная глубина очереди Phase 2 за сессию; тренд за 5 мин при 6 игроках | лаг consumer-группы Phase 2 (`rpk group describe`) или `agent.queue` в логах | не растёт монотонно |
| `llm_cold_start_s` (NFR-004) | Латентность первого `llm.output` после простоя ≥ 10 мин | `llm.output.latency_ms` | ≤ 20 с (после замера) |
| `defects_per_session` | Число записей `incidents.csv` любого `kind` за сессию | `incidents.csv` | тренд вниз; `world_break` = 0 |

### 3.3. Игра и группа

| Метрика (id) | Определение и формула | Источник | Цель |
|---|---|---|---|
| `turns_per_session` | `session.turns_count` | `analytics.session.ended` | измеряем; ориентир ≥ 30 (длина приёмочного сценария) |
| `session_length_min` | `ended_at − started_at` | то же | измеряем |
| `session_end_reason_share` | Доля сессий по `end_reason`: `leave`, `idle`, `death`, `error` | то же | `error` = 0 |
| `group_sessions_share` | `count(session, scope.type=group)` / `count(session)` для `human` | `sessions.csv` | ≥ 3 из 8 (S7) |
| `group_size_avg` | Среднее `players_count` для групповых сессий | то же | измеряем |
| `player_actions_by_type` | Распределение `turn.action_type` (`enter, look, attack, flee, say, leave`) | `analytics.turn.completed` | измеряем (сигнал: только `attack` — мир скучен) |
| `return_sessions_share` | Доля `human`-сессий, где хотя бы один участник уже играл ранее (`player_id` встречался) | `sessions.csv` | измеряем (не retention — «тестеры возвращаются») |

### 3.4. Стоимость и бюджет LLM

| Метрика (id) | Определение и формула | Источник | Цель |
|---|---|---|---|
| `llm_calls_per_turn` (NFR-006) | `count(llm.output, replay=false, actor_kind ≠ system)` / turns, разбивка по `phase`; фоновые вызовы **исключены** (считаются в §3.5) | `llm.output` | ≤ 2 соло на **все уровни роя вместе** (≤ 1 при Phase 1 на правилах); для группы из 3 — после замера B7 (D7: до трёх персональных GM + региональный) |
| `llm_calls_per_turn_by_level` (D7) | То же с разбивкой по `agent.level` (`global` / `domain` / `task`) и внутри `task` — по `agent.blueprint` (персональный GM vs встреча): `count(llm.output, replay=false, actor_kind ≠ system, agent.level = L)` / turns; связь с ходом — по `correlation_id` | `llm.output` | измеряем; ожидание: `global` ≈ 0 на ход (глобальный GM работает в фоне, на ход — правила/контекст), `domain` ≤ 1, `task` ≤ 1 на игрока; сигнал: `global > 0` регулярно — иерархия «течёт» вниз через LLM, а не через контекст |
| `llm_tokens_per_turn` | `Σ(tokens.prompt + tokens.completion)` / turns | `llm.output` | измеряем; ориентир после замера |
| `llm_busy_ms_per_turn` | `Σ llm.output.latency_ms` / turns — прокси стоимости локальной модели (занятость GPU) | `llm.output` | измеряем |
| `llm_cost_per_turn` | `Σ llm.output.cost_usd` / turns; для Ollama `cost_usd = 0` по определению | `llm.output` | 0 в базовой конфигурации; облако — ≤ лимита |
| `llm_token_accounting_error` (NFR-052) | `|Σ tokens(llm.output) − Σ tokens(отчёт провайдера)|` / `Σ tokens(провайдер)` на 100 вызовах | `llm.output` + лог Ollama/облака | ≤ 0.05 |
| `budget_exceeded_events` (FR-072) | `count(llm.output.rejected, reason=budget_exceeded)`, разбивка по `actor_kind` (`system` — сработал потолок фона, см. §3.5) | `llm.output.rejected` | измеряем; для фона `> 0` — норма (ограничитель работает), для ходов — тревога |

Метрика `idle_scope_llm_calls = 0` (v0.1; NFR-053, BR-10) **удалена**: по D8 мир
живёт и без игроков, вызовы LLM без активной сессии — ожидаемое поведение,
ограниченное потолком (§3.5), а не нарушение.

### 3.5. Рой GM и фоновая жизнь мира (D7, D8, S14)

Фоновые метрики считаются **за регион** (`scope{id,type:region}` фонового
события) и за период (час / сутки), а не за сессию — фон идёт и без сессий.
«Активная сессия в регионе» = есть `analytics.session.started` для scope этого
региона без парного `ended` в момент `timestamp` события.

| Метрика (id) | Определение и формула | Источник | Цель |
|---|---|---|---|
| `background_events_per_region_per_day` | Число фоновых событий региона за сутки: `count(world.*, region.*; actor_kind=system; scope.id = регион)` / сутки; разбивка по `agent.level` (`global` / `domain`) и типу события; отдельно — доля событий, произошедших **без активной сессии** в регионе (`unattended_share`) | фоновые доменные события × `analytics.session.*` | измеряем; S14 требует ≥ 1 за интервал между сессиями; сигнал: 0 за сутки при включённом планировщике — фон не работает; десятки — фон «шумит» и раздувает сводку входа |
| `background_llm_calls_per_region_per_hour` | `count(llm.output, actor_kind=system, replay=false, scope.id = регион)` / час, разбивка по `agent.level` и `model`; p95 по часам суток и максимум за сутки | `llm.output` | ≤ `background.llm_calls_cap` из конфигурации (виден оператору, S8/D8); число — **архитектор после замера B8** (предложение PO: 1); ожидание — фон на малой модели или без LLM (`llm_calls = 0` для тика по правилам) |
| `background_llm_busy_ms_per_hour` | `Σ llm.output.latency_ms (actor_kind=system)` / час — занятость GPU фоном; вместе с `narrative_latency_*` показывает, мешает ли фон активной сессии (S4 измеряется при включённом фоне) | `llm.output` | измеряем; сигнал: рост `narrative_latency_p95_ms` в часы с фоновыми вызовами |
| `background_event_surfaced_rate` | Доля фоновых событий, случившихся между сессиями игрока, которые отражены в нарративе входа: `Σ count(absence.surfaced_event_ids)` / `Σ count(absence.background_events_count)` по ходам `action_type=enter` с `absence` ≠ null; в MVP-1 «отражено» = ID события передан в структурированный вывод персонального GM (`background_refs[]`) **и** подтверждён человеком по чек-листу на выборке | `analytics.turn.completed` (`absence{…}`) + чек-лист | ≥ 1 событие на вход при `background_events_count ≥ 1` (S14); доля — измеряем, ориентир ≥ 0,5 после замера (не 1,0: часть фона намеренно остаётся «за кадром») |
| `world_break_from_background` | Поломки мира, причина которых — фоновое событие: `count(analytics.consistency.violated, severity=break, cause.actor_kind=system)` + `count(incidents.csv, kind=world_break, cause=background)`; `cause` определяется по цепочке `correlation_id` нарушенного состояния (последнее `entity.updated` с `actor_kind=system`) | `analytics.consistency.violated` + `incidents.csv` | 0 (S14, S1/S2) |
| `cross_level_event_rate` (D7) | Доля событий роя, породивших реакцию **другого уровня**: `count(events, agent.level = L, trigger.event.type ∈ types(level ≠ L))` / turns; уровень триггера выводится из префикса типа (`world.*` → global, `region.*`/`encounter.*` → domain/task, `player.*` → игрок); отдельно вниз (`world.*` → `domain`/`task`) и вверх (`task` → `domain`, `domain` → `global`) | `world.*`, `region.*`, `encounter.*`, `narrative_output` c `agent{level}` и `trigger{event{id,type}}` | измеряем; в сценарии S1/S14 — ≥ 1 в каждую сторону (иерархия реально работает, а не три независимых агента); сигнал: 0 вниз при `background_events > 0` — фон не доходит до игрока |
| `agents_active_by_level` (FR-082) | Число живых агентов по `agent.level` в момент времени (из `agent.*` lifecycle: `spawned` − `resolved`); максимум за сессию | `agent.*` lifecycle | ожидание в MVP: `global` = 1, `domain` = 1, `task` = игроки в сессии (+ встречи); больше — утечка агентов |

---

## 4. События аналитики

### 4.1. Принципы

1. **Минимально инвазивно.** Основные источники — уже требуемые доменные события
   (`player.*`, `dice.rolled`, `combat.decided`, `entity.updated`, `llm.output`,
   `llm.output.rejected`). Добавляются **пять** событий `analytics.*` только для
   того, чего в домене нет: границы сессии, сводка хода с таймингами, факт
   нарушения консистентности, итог replay. Ничего не дублируется: токены и
   провайдер живут в `llm.output`, броски — в `dice.rolled`.
2. **Именование** — dot-notation проекта `object.action`; аналитические события
   с префиксом `analytics.` (`analytics.turn.completed`).
3. **Топик.** Предпочтительно отдельный `analytics_events` (одна строка в
   `redpanda-init`): replay-потребители его не читают, retention задаётся
   независимо (дольше, чем у доменных топиков). Запасной вариант —
   `system_events`, тогда потребители replay обязаны игнорировать `analytics.*`.
   Решение — за архитектором (OQ-A-07/08).
4. **Структура payload** — иерархическая, как в CLAUDE.md: `world{entity{id,type}}`,
   `scope{id,type}`, `entity{entity{id,type},name}`, `trigger{event{id,type}}`,
   `correlation_id`. Для игрока `entity.name` — имя персонажа, не ник мессенджера.
5. **Приватность.** В `analytics.*` нет свободного текста игрока, текста
   нарратива, внешних ID. Только `player_id`, типы действий, тайминги, счётчики.
6. **Ключ сообщения** — `world.entity.id` (как у остальных событий), чтобы
   порядок внутри мира сохранялся.
7. **Издатель — тот, кто знает факт**: тайминги хода и границы сессии знает
   только game-service (он принимает действия и доставляет ответы); нарушения —
   проверяющий; итог replay — тот, кто его выполняет.
8. **Фоновые тики — без нового события.** Вместо `analytics.background.tick`
   планировщик кладёт в контекст тика `actor_kind = system` и `agent{level,
   blueprint}`, а конвейер Agent GM Core **копирует их во все события цепочки**
   (`world.*`, `region.*`, `entity.updated`, `dice.rolled`, `llm.output`,
   `llm.output.rejected`) так же, как `correlation_id`; `trigger.event.type =
   "gm.tick"`. Так фон считается из тех же событий, что и ходы, replay
   воспроизводит его без особых веток, а реализация — один параметр контекста.
   Для ходов `actor_kind` наследуется от сессии (`human|ci|sim`), `agent{…}`
   проставляет агент-издатель. Если позже понадобятся тики «впустую» (без
   событий) или длительность тика — добавим `analytics.background.tick`, но пока
   ни одна метрика §3.5 этого не требует.

### 4.2. Новые события `analytics.*`

| Событие | Когда | Издатель | Топик | Обязательные поля payload |
|---|---|---|---|---|
| `analytics.session.started` | Первое принятое действие в scope, у которого нет активной сессии (после `session.ended` или простоя ≥ 30 мин); для `group` — при создании группы | game-service | `analytics_events` | `world{entity{id,type}}`, `scope{id,type}`, `session{id, kind: solo\|group, actor_kind: human\|ci\|sim, started_at, players_count}`, `participants[]{entity{id,type:player}}` |
| `analytics.session.ended` | Все участники вышли (`leave`), все мертвы, простой ≥ 30 мин (TTL) или ошибка | game-service | `analytics_events` | `world`, `scope`, `session{id, kind, actor_kind, started_at, ended_at, end_reason: leave\|idle\|death\|error, players_count, turns_count, turns_degraded, turns_failed}`, `participants[]` |
| `analytics.turn.completed` | Ход завершён: нарратив или шаблон доставлен всем адресатам scope; либо истёк таймаут ожидания (60 с) → `status=timeout`; либо действие отклонено до Phase 1 → `status=rejected` | game-service | `analytics_events` | `world`, `scope`, `entity{entity{id,type:player},name}`, `correlation_id`, `trigger{event{id,type}}` (событие действия), `session{id}`, `turn{seq, action_type, target{entity{id,type}}?, status: ok\|degraded\|rejected\|timeout\|error, gm_path: agent\|legacy, phase1_mode: rules\|llm, lod?}`, `timings{received_at, acked_at, mechanics_at?, narrative_at?, mechanics_ms?, narrative_ms?, total_ms}`, `narrative{generated_by: llm\|template\|none, agent_level: global\|domain\|task, agent_blueprint?, fallback_reason?, filter_applied: bool}`, `delivery{recipients_count, delivered_count, result_event_id?, narrative_event_id?}`, `absence{since_at, background_events_count, surfaced_event_ids[]}?` (только для `action_type=enter` при наличии предыдущей сессии игрока в мире; копируется из `narrative_output` персонального GM, §4.3) |
| `analytics.consistency.violated` | Проверка инвариантов после хода (тест/страж), сверка снапшота с логом, ручная/LLM-оценка нарратива нашли нарушение | MVP-1: тест-харнесс и CLI `session-report --audit`; целевое: страж (Запрет Мира) | `analytics_events` | `world`, `scope`, `correlation_id?`, `session{id}?`, `violation{code: hp_out_of_range\|dead_entity_acts\|duplicate_entity\|missing_entity\|position_mismatch\|inventory_mismatch\|state_divergence\|narrative_contradiction\|player_agency, layer: law\|canon\|state\|momentary, severity: break\|warn, detected_by: invariant_check\|snapshot_diff\|llm_judge\|human, entity{entity{id,type}}?, expected?, actual?}` (значения — числа/идентификаторы, не текст нарратива) |
| `analytics.replay.completed` | Завершено восстановление из снапшота + replay (в тесте или при реальном старте) | компонент восстановления (entity-manager / тест-харнесс) | `analytics_events` | `world`, `replay{run_id, mode: recovery\|test, snapshot_id, snapshot_at, events_replayed, llm_calls, dice_rolled_new, duration_ms, state_hash_before?, state_hash_after, identical: bool?, events_hash_match: bool?, incomplete_record: bool}` |

Примечания к реализации (чтобы разработчик мог сделать без уточнений):

- `session.id = "{scope.id}:{started_at_unix}"`; game-service держит таблицу
  активных сессий в памяти и восстанавливает её из последнего
  `analytics.session.started` без парного `ended` (или закрывает с
  `end_reason=error` при старте).
- `actor_kind` задаётся клиентом при регистрации/действии (заголовок или поле
  запроса game-service): бот — `human`, тест-харнесс — `ci`, нагрузочный скрипт
  — `sim`. Значение по умолчанию — `human`.
- Тайминги — миллисекунды UTC по часам game-service (одна машина, единые
  часы); `mechanics_at` — момент получения game-service события Phase 1
  (`combat.decided` / `entity.updated`) по `correlation_id`; `narrative_at` —
  момент отправки текста последнему адресату.
- `turn.seq` — порядковый номер хода в сессии, начиная с 1.
- Для `status=rejected` (неизвестная цель, мёртвый персонаж, чужой scope)
  `timings` содержит только `received_at`, `acked_at`; такие ходы не входят в
  латентности, но входят в `player_actions_by_type`.
- `absence.since_at` = `ended_at` последней `analytics.session.ended` этого
  `player_id` в мире; `background_events_count` — число фоновых событий региона
  входа за `(since_at, received_at)` (считает персональный GM при сборке сводки
  «пока тебя не было» и отдаёт в `narrative_output`); `surfaced_event_ids[]` —
  ID событий из `background_refs[]` структурированного вывода. Если предыдущей
  сессии нет — поле отсутствует.
- `narrative.agent_level` — уровень агента, чей `narrative_output` доставлен
  игроку (в MVP — `task`, персональный GM; `domain`, если архитектор решит
  рендерить нарратив региональным GM). Для `generated_by=template` — уровень,
  на котором сработал фолбэк.

### 4.3. Существующие/требуемые доменные события — поля, нужные для метрик

Эти события уже требуются PRD; ниже — минимальные добавления, без которых
метрики §2–3 не считаются. Формально оформить их — задача BA (§8).

| Событие | Кто публикует | Поля, нужные метрикам |
|---|---|---|
| `player.*` (действие: `player.entered_region`, `player.attacked`, `player.looked`, …) | game-service | `id` (= `correlation_id` цепочки), `timestamp`, `world`, `scope`, `entity{player}`, `action.type` из словаря FR-002; **без** внешних ID и с текстом `say` только в доменном событии |
| `dice.rolled` (FR-022) | Rule Engine | `correlation_id`, `trigger{event{id}}`, `seed`, `formula`, `result`, `replay: bool` |
| `combat.decided` (Phase 1) | Agent GM Core / Rule Engine | `correlation_id`, `trigger{event{id}}`, `gm_path: agent\|legacy`, `phase1_mode: rules\|llm`, `lod?`, `timestamp` |
| `entity.updated` (FR-030) | entity-manager | `correlation_id`, `entity{entity{id,type}}`, `version`, `changed[]` (имена полей), `timestamp` |
| `llm.output` (FR-032) | конвейер агента | к полям US-018 (`provider, model, params, prompt_hash, response_raw, validation_status, correlation_id, caused_by`) добавить: `phase: decision\|narrative\|other`, `attempt`, `latency_ms`, `tokens{prompt, completion}`, `cost_usd` (0 для Ollama; для облака — по прайс-листу конфигурации), `gm_path`, `lod?`, `replay: bool` (при replay событие не создаётся заново — читается записанное; флаг нужен потребителям, чтобы отличить живой вызов), **`actor_kind: human\|ci\|sim\|system`**, **`agent{level: global\|domain\|task\|object\|monitor, blueprint}`**, `scope{id,type}` (для фона — регион) |
| `llm.output.rejected` (FR-034) | конвейер агента | `correlation_id`, `phase`, `reason: unknown_entity\|player_agency\|schema_invalid\|language\|filter_error\|budget_exceeded\|other`, `entity{…}?` (для `unknown_entity` — ссылка, которой нет), **`actor_kind`, `agent{level, blueprint}`** |
| `world.*`, `region.*`, `encounter.*` (события роя; фоновые и по ходу) | глобальный / региональный GM / агент встречи | `world`, `scope{id,type:region}` (для `world.*` — `scope` отсутствует или мир), `agent{level, blueprint}`, `actor_kind` (`system` для фоновых тиков), `trigger{event{id,type}}` (`gm.tick` для фона; `world.*`/`player.*` — для реакций на другой уровень), `correlation_id`, `timestamp` — источник §3.5 (`background_events_*`, `cross_level_event_rate`) |
| `narrative_output` (нарратив персонального GM игроку) | персональный GM (`task`) | `entity{player}`, `scope`, `correlation_id`, `agent{level, blueprint}`, `generated_by`, для входа — `absence{since_at, background_events_count}` и `background_refs[]{event{id,type}}` (ID фоновых событий, использованных в сводке «пока тебя не было»); текст в метрики не попадает — game-service копирует только счётчики и ID в `analytics.turn.completed` |
| `agent.child_resolved`, `agent.*` lifecycle | Agent GM Core | `scope`, `agent{id, level, blueprint}`, `reason` — для `agents_active_by_level` (FR-082) |
| `world.law_breach.proposed` (FR-041, целевое) | конвейер агента | `law{version_before, version_after}`, `review{status, reviewer_kind, decided_at}` |

Структура `eventbus.Event` сегодня не содержит `correlation_id` (audit-facts §5);
для метрик достаточно единого пути — предложение: `payload.correlation_id` +
`trigger.event.id`, конкретика — за архитектором (FR-084, NFR-031).

---

## 5. Гипотезы и критерии проверки

Общие правила: одна машина и один GPU — сравнения **последовательные**, не
параллельные; порядок вариантов чередуется; ≥ 3 прогона на вариант; сценарий и
seed фиксированы (FR-100); сравниваются артефакты прогонов (FR-101/102).
Сравнение по латентности и стоимости — только на **живой** LLM (в replay
вызовов нет по определению).

| # | Гипотеза | Как проверяем | Критерий подтверждения | Если не подтвердилась |
|---|---|---|---|---|
| H1 | **Agent GM Core не дороже и не медленнее legacy-пути** при том же качестве | A/B по фича-флагу `agent_mode` (US-019): сценарий US-002 (30 ходов, фиксированный seed), 3 прогона на вариант на одной модели; метрики `llm_calls_per_turn`, `llm_tokens_per_turn`, `mechanics_latency_p95_ms`, `narrative_latency_p95_ms`, `llm_valid_first_try_ratio`, `narrative_mechanics_mismatch_per_100`, `consistency_violations_per_session` | Agent: `llm_calls_per_turn` и `llm_tokens_per_turn` ≤ legacy (медиана 3 прогонов); `narrative_latency_p95_ms` не хуже legacy более чем на 10 %; `mechanics_latency_p95_ms` ≤ 500 мс; `mismatch_per_100` и `consistency_violations` не хуже legacy. Если legacy не проходит 30 ходов (сегодня — так), гипотеза сводится к абсолютным порогам NFR-001/002/006 | Приоритет решает владелец: стоимость vs качество (см. §9); флаг остаётся до устранения причины, но не дольше миграции (BR-12) |
| H2 | **Группа 2–6 не ломает консистентность** | Сценарий US-006/007: 3 игрока (затем 6 ботов-имитаторов, NFR-005), 30 ходов, включая одновременные атаки в пределах 1 с и дубли доставки; метрики `state_divergence_count`, `group_view_consistency`, `consistency_violations_per_session`, `phase2_queue_depth_max`, `mechanics_latency_p95_ms` | `state_divergence_count = 0`, `group_view_consistency = 1.0`, `consistency_violations(break) = 0`, очередь Phase 2 не растёт монотонно 5 мин, NFR-001 сохраняется при 3 игроках (при 6 — после замера) | Ограничить размер группы в MVP-1 (например, 3) до устранения; зафиксировать в open-questions как влияющее на D1 |
| H3 | **Qwen3 8B достаточно для решений (Phase 1); крупная модель нужна только для нарратива (Phase 2)** | «Золотой набор» 20 ситуаций (NFR-065) с ожидаемым решением, размеченным человеком. Phase 1: 8B vs кандидат покрупнее (14B/32B или облако через абстракцию FR-070), по 3 прогона; метрики `llm_valid_first_try_ratio(phase=decision)`, `decision_agreement` = доля ситуаций, где решение совпало с ожидаемым, `guardian_rejections_per_100`, `latency`. Phase 2: те же модели; `llm_valid_first_try_ratio(phase=narrative)`, `narrative_language_reject_ratio`, `mismatch_per_100`, оценка человеком по чек-листу (язык, соответствие механике, атмосфера — 1–5) | Phase 1 на 8B: `valid_first_try ≥ 0.90`, `decision_agreement ≥ 0.80` и не хуже крупной модели более чем на 5 п. п.; Phase 2: если крупная модель даёт оценку человека выше ≥ 1 балла или `mismatch` ниже ≥ 3 на 100 при латентности в пределах NFR-002 — она оправдана для нарратива | Phase 1 — правила без LLM там, где 8B ошибается (FR-013 допускает `phase1_mode=rules`); либо модель побольше для решений с пересмотром NFR-001/006 |
| H4 | **Record-replay восстанавливает мир без LLM** | Тесты US-011 и US-017: 10 ходов → снапшот на 7-м → рестарт всех сервисов → догон 8–10; отдельно — рестарт между Phase 1 и Phase 2; повторный прогон записи в CI без GPU | `replay_llm_calls = 0`, `replay_new_dice = 0`, `recovery_state_identical = true`, `replay_determinism = true`, `recovery_time_s ≤ 120` (после замера); при неполной записи — остановка с `incomplete_record=true`, а не живой вызов | Пересмотр D5: какие вызовы не записаны (аудит `llm.output` по `correlation_id` цепочки) — обычно недостающее событие, а не архитектурный дефект |
| H5' | **Фон по правилам / на малой модели даёт живой мир без роста стоимости и поломок** (D8, S14; заменяет H5 v0.1 «мир тратит LLM только там, где есть игроки» — отклонено пользователем, OQ-R-15) | Автосценарий S14 с ускоренными тиками: сессия A (10 ходов) → пауза, эквивалентная ≥ `S14.gap_hours` фонового времени, без игроков → сессия B того же игрока (`enter` + 10 ходов); 3 прогона; варианты фона: (а) тики по правилам без LLM, (б) тики на малой модели (8b); метрики `background_events_per_region_per_day`, `background_llm_calls_per_region_per_hour`, `background_llm_busy_ms_per_hour`, `background_event_surfaced_rate`, `world_break_from_background`, `cross_level_event_rate` (вниз), `narrative_latency_p95_ms` в сессии B vs A, `llm_calls_per_turn` B vs A | Для принятого варианта: `background_events_between_sessions ≥ 1`, `surfaced ≥ 1` на входе, `world_break_from_background = 0`, `background_llm_calls_per_region_per_hour ≤ cap` (после замера B8), `narrative_latency_p95_ms` и `llm_calls_per_turn` в сессии B не хуже A более чем на 10 % (фон не мешает ходам и не раздувает контекст) | Если ломает мир — фон только по правилам (без LLM) до устранения; если растит стоимость/латентность — реже тики / LOD `rule-only` для `global`, потолок ниже; если события не всплывают в нарративе — сводка «пока тебя не было» как отдельный шаблонный блок, не через LLM |
| H6 | **Деградация без LLM не ломает сессию** (BR-14) | Тест US-004: остановить контейнер LLM на ходах 10–15, затем вернуть | `degraded_mode_success_ratio = 1.0`, `unanswered_action_ratio = 0`, после возврата LLM `generated_by=llm` без рестарта | Ошибка в конвейере — блокер MVP-1 (NFR-015 Must) |

---

## 6. Минимальный дашборд оператора

### 6.1. Что смотреть после каждой сессии (одна таблица)

| Блок | Показатели | Тревога, если |
|---|---|---|
| Сессия | `session.id`, `kind`, `actor_kind`, `players_count`, `turns_count`, `session_length_min`, `end_reason` | `end_reason=error` |
| Мир держится | `world_break_count`, `consistency_violations` (break/warn по кодам), `state_divergence_count` (если запущен `--audit`), `unanswered_action_ratio`, `service_panics` | любое ≠ 0 |
| Скорость | `mechanics_latency` p50/p95, `narrative_latency` p50/p95 (соло) / `narrative_latency_group` p50/p95 (группа), `ack_latency` p95 | p95 выше порога NFR-001/002/003 (для группы — свой порог) |
| LLM | `llm_calls_per_turn` (+ `by_level`), `llm_tokens_per_turn`, `llm_busy_ms_per_turn`, `llm_fallback_rate` (+причины), `llm_valid_first_try_ratio` по фазам, `guardian_rejections` по причинам, `narrative_language_reject_ratio`, `cloud_llm_calls` (включая фон) | `cloud_llm_calls > 0` в базовой конфигурации; `fallback_rate > 0.05`; `global > 0` на ход регулярно |
| Рой и фон (за сутки, не за сессию) | `agents_active_by_level`, `background_events_per_region_per_day` (+ `unattended_share`), `background_llm_calls_per_region_per_hour` (max/час и cap), `background_llm_busy_ms_per_hour`, `background_event_surfaced_rate` (по последним входам), `world_break_from_background`, `cross_level_event_rate` (вниз/вверх) | `background_llm_calls > cap`; `world_break_from_background > 0`; `background_events = 0` за сутки при включённом планировщике; `agents_active` больше ожидаемого (утечка) |
| Миграция | `legacy_gm_path_share`, `gm_instances_active` | > 0 при `agent_mode=on` |
| Восстановление | результат последнего `analytics.replay.completed` (`llm_calls`, `identical`, `duration_ms`) | что-либо ≠ ожидаемого |
| Игра | `player_actions_by_type`, `group_view_consistency` | только `attack`; consistency < 1 |

### 6.2. Куда класть — самый дешёвый вариант для одного разработчика

**MVP-1 (рекомендация): CLI `session-report` + два CSV в git, без БД.**

- `tools/session-report` (Go, в workspace): читает `analytics_events`,
  `llm.output`/`llm.output.rejected`, `dice.rolled`, фоновые `world.*`/`region.*`
  за окно сессии (по `session.id` или времени) через существующий
  `shared/eventbus` reader, считает таблицу §6.1, печатает Markdown в консоль и
  дописывает строку в `ops/metrics/sessions.csv`; флаг `--background --since 24h`
  считает блок «Рой и фон» за сутки (без привязки к сессии) и дописывает строку
  в `ops/metrics/background.csv` (`date, region, events, llm_calls_max_per_hour,
  cap, busy_ms, breaks`); флаг `--audit` запускает сверку снапшота с
  агрегатом лога и публикует `analytics.consistency.violated`
  (`code=state_divergence`); флаг `--json` сохраняет сырую сводку в
  `ops/metrics/sessions/<session_id>.json` (артефакт прогона по FR-101, пригоден
  для сравнения вариантов в H1/H3).
- `ops/metrics/incidents.csv` — ручной журнал: `date, session_id,
  correlation_id, kind (world_break|defect|content|other), manual_fix (bool),
  description`. Заполняет оператор после сессии; это источник для S7.
- Оба CSV коммитятся: ПДн в них нет (только `player_id`), объём — строки в
  неделю, история сравнима в PR. Недельные агрегаты (NSM, S7) — тем же CLI
  `session-report --weekly`.
- Обоснование: Redpanda retention может быть короче, чем нужно для истории
  сессий; CSV даёт постоянный след без нового сервиса. TimescaleDB сегодня без
  клиента и потребителей (audit-facts §7), reality-monitor неработоспособен —
  поднимать их ради 8 сессий нецелесообразно.

**Целевое (эпик E-G, US-034):** эндпоинт метрик Prometheus-совместимого
формата у сервисов (NFR-032) + Grafana, либо коллектор `analytics_events` →
TimescaleDB. Судьба reality-monitor — за архитектором: либо он становится этим
коллектором, либо удаляется; в MVP-1 не нужен.

---

## 7. Протокол первого замера (для порогов «после замера»)

Совпадает с NFR «Что измерить первым»; каждый пункт — фиксированный сценарий,
≥ 3 прогона, результат — в `ops/metrics/baseline.md` с датой, GPU/VRAM, моделями.
**Стенд:** RTX 4090 24 ГБ / 128 ГБ RAM / i9-13900, обе модели Qwen3 загружены
одновременно (8b решения/фон, 30b-a3b/32b нарратив), фоновые тики **включены**
с настройками MVP (S4). Пороги, помеченные «после замера», по итогам B1–B8
фиксирует архитектор.

| # | Что | Метрики | Сценарий | Фиксируем порог для |
|---|---|---|---|---|
| B1 | Латентность Phase 1 (rules и llm) и Phase 2; соло и группа отдельно; при включённом и выключенном фоне (разница — вклад фона) | `mechanics_latency_p95_ms`, `narrative_latency_p95_ms`, `narrative_latency_group_p95_ms`, `background_llm_busy_ms_per_hour` | US-002 соло; US-006 группа из 3 | NFR-001, NFR-002, S4 (порог группы) |
| B2 | Холодный старт LLM, `keep_alive` | `llm_cold_start_s` | пауза 10 мин → ход | NFR-004 |
| B3 | Валидность JSON с первой попытки | `llm_valid_first_try_ratio` по фазам для `qwen3:8b` и кандидата покрупнее | золотой набор 20 ситуаций | NFR-022, H3 |
| B4 | Язык нарратива | `narrative_language_reject_ratio` | 100 ходов Phase 2 | NFR-090 |
| B5 | Старт стека и ресурсы | `stack_up_time_s` (до всех `/health=ok`), RAM/VRAM | `make up` | NFR-070, NFR-073 |
| B6 | Восстановление | `recovery_time_s`, `replay_*` | US-011 | NFR-010, S3 |
| B7 | Стоимость хода, с разбивкой по уровням роя; соло и группа из 3 | `llm_calls_per_turn`, `llm_calls_per_turn_by_level`, `llm_tokens_per_turn`, `llm_busy_ms_per_turn` | US-002; US-006 | NFR-006, ориентиры §3.4 (в т. ч. порог для группы) |
| B8 | Стоимость и результат фона: 24 ч фонового времени (ускоренные тики) без игроков для вариантов «правила без LLM» и «малая модель», затем вход игрока | `background_events_per_region_per_day`, `background_llm_calls_per_region_per_hour`, `background_llm_busy_ms_per_hour`, `background_event_surfaced_rate`, `world_break_from_background`, `cross_level_event_rate` | автосценарий S14 / H5' | S14: `S14.gap_hours` (предложение PO 6 ч), `background.llm_calls_cap` (предложение PO 1/регион/час), интервал тиков по уровням — **решение архитектора** |

---

## 8. Что нужно добавить в требования, чтобы метрики были собираемы

Предложения бизнес-аналитику (нумерация ориентировочная):

1. **FR-084 / NFR-031 — единый путь `correlation_id`.** Сейчас `eventbus.Event`
   его не содержит; метрики требуют одного детерминированного места
   (предложение: `payload.correlation_id` + `trigger.event.id`), проставляемого
   game-service при приёме действия и копируемого всеми звеньями цепочки.
2. **FR-032 (`llm.output`) — расширить обязательные поля:** `phase`, `attempt`,
   `latency_ms`, `tokens{prompt,completion}`, `cost_usd`, `gm_path`, `replay`.
   Иначе NFR-006/022/052 и H1/H3 не измеримы.
3. **FR-034 (`llm.output.rejected`) — перечисление `reason`** (§4.3), включая
   `player_agency` (BR-06) и `budget_exceeded` (FR-072).
4. **Новое FR (учёт сессий и ходов):** game-service публикует
   `analytics.session.started/ended` и `analytics.turn.completed` (§4.2);
   определение сессии (граница простоя 30 мин) и поле `actor_kind` в API
   game-service.
5. **Новое FR (нарушения консистентности как событие):** проверка инвариантов
   и сверка снапшота с логом публикуют `analytics.consistency.violated`;
   список инвариантов «Тёмного леса» — часть блупринта/правил (NFR-020).
6. **FR-031/FR-100 (replay) — публиковать `analytics.replay.completed`** со
   счётчиками `llm_calls`, `dice_rolled_new`, хэшами состояния (S3, NFR-014/061).
7. **FR-013 (`combat.decided`) — поля `gm_path`, `phase1_mode`** (S5, H1).
8. **Топик `analytics_events`** в `redpanda-init` с retention ≥ 90 дней (или
   явное правило игнорирования `analytics.*` в replay) — архитектор.
9. **FR-101 (артефакт прогона)** — уточнить формат: JSON-сводка
   `session-report --json` (§6.2) как минимальный артефакт.
10. **FR-060 (хранилище связок)** — добавить поле `notice_shown_at` (дата
    показа уведомления об ИИ): S9 измеряется без событий в шине.
11. **Новое FR (журнал инцидентов оператора):** `ops/metrics/incidents.csv` с
    полями §6.2 — единственный источник для «ручной правки состояния» в S7.
12. **NFR-033 (логи)** — поле `handled: bool` для ошибок, чтобы `service_panics`
    отделял ожидаемые ошибки от необработанных.
13. **Опционально (S7, качество):** три вопроса тестерам после сессии
    (мир помнил? были противоречия? продолжили бы?) по шкале 1–5, привязка к
    `session.id` и `player_id`, без свободного текста в git — дешёвый сигнал
    «живости» для NSM; требует согласия тестеров как участников (не ПДн).

Добавлено в v0.2 (D7/D8; требования BR-10, NFR-053, FR-016, FR-037, US-002
критерий TTL и US-022 сегодня построены на «мир живёт только в активных scope»
и **подлежат пересмотру BA** — vision v1.1 D8):

14. **BR-10 / FR-037 / NFR-053 — заменить** «вызовов LLM без игроков = 0» на
    «фоновые тики глобального и регионального GM идут по расписанию без
    игроков; расход ограничен потолком `background.llm_calls_cap` (вызовов на
    регион в час; значение — конфигурация, по умолчанию — после замера B8) и
    виден оператору; при превышении тик выполняется по правилам без LLM и
    публикуется `llm.output.rejected` с `reason=budget_exceeded,
    actor_kind=system`». FR-016 (TTL агента) — оставить для `task`-агентов
    (персональный GM = сессия, встречи — минуты); для `global`/`domain` — TTL
    не применяется (долгоживущие), уточнить DR-24.
15. **Новое FR (контекст фонового тика):** планировщик тиков задаёт в контексте
    `actor_kind=system`, `agent{level, blueprint}`, `trigger{event{type:
    "gm.tick"}}`, `correlation_id`; конвейер Agent GM Core копирует эти поля во
    все события цепочки (§4.1 п. 8). Поля `actor_kind` и `agent{level,
    blueprint}` — обязательные в `llm.output`, `llm.output.rejected`,
    `world.*`, `region.*`, `encounter.*` (дополнение к п. 2–3).
16. **Новое FR (сводка «пока тебя не было»):** при `enter` игрока с предыдущей
    сессией персональный GM получает список фоновых событий региона за
    `(since_at, now)` и обязан отдать в `narrative_output` структурированные
    `absence{since_at, background_events_count}` и `background_refs[]{event{id,
    type}}` (ID использованных событий); game-service копирует счётчики и ID в
    `analytics.turn.completed.absence`. Без этого S14 («отражено в нарративе»)
    проверяется только вручную.
17. **S4 / NFR-002 — отдельный порог для группы** (`narrative_latency_group_p95_ms`,
    замер B1) и явные условия стенда: RTX 4090, две модели загружены, фон включён;
    **NFR-006** — норма вызовов на ход относится ко **всем уровням роя вместе**,
    фоновые вызовы считаются отдельно (`background_llm_calls_per_region_per_hour`).
    В **S3 / FR-031** — replay включает фоновые события (`actor_kind=system`)
    наравне с ходами.

---

## 9. Приватность и ограничения сбора

- В `analytics.*`, CSV и отчётах — только `player_id`, идентификаторы сущностей,
  типы действий, тайминги, счётчики. Ни внешних ID, ни ников мессенджера, ни
  текста реплик игрока, ни текста нарратива (BR-07, NFR-041).
- `analytics.consistency.violated` содержит `expected/actual` как числа или
  идентификаторы; фрагменты нарратива при ручной разметке хранятся в локальном
  артефакте исследователя, не в шине.
- `response_raw` в `llm.output` — доменное требование (BR-04), а не
  аналитическое; метрики читают из него только статусы и счётчики.
- Опрос тестеров (§8 п. 13) — добровольный, без свободного текста в git.
- При удалении связки (FR-061) аналитика по `player_id` остаётся
  псевдонимной и не требует чистки; полное право на забвение — целевое (FR-062).

## 10. Открытые вопросы

Полный реестр — `project/open-questions.md` (ведёт BA; здесь не правится).
Влияют на метрики:

1. ~~Граница сессии (простой 30 мин), учёт только `human`-сессий и числа 8/3 в
   S7~~ — **решено на G1** (vision v1.1), закрыто.
2. Приоритет при конфликте в H1 «дешевле vs качественнее» — решение владельца
   в момент сравнения; до этого критерий H1 требует «не хуже» по обоим.
3. ~~Характеристики GPU/VRAM~~ — **решено** (RTX 4090 24 ГБ / 128 ГБ / i9-13900,
   две модели одновременно), закрыто; открытыми остаются квантование и режим
   thinking (архитектор) — влияют на сравнимость B1–B8 между прогонами:
   фиксировать в `baseline.md`.
4. `analytics_events` как отдельный топик vs `system_events` — архитектор (OQ-A-07/08).
5. **Пороги S14** — `S14.gap_hours` (предложение PO 6 ч), `background.llm_calls_cap`
   (предложение PO 1/регион/час), интервалы тиков `global`/`domain` — архитектор
   после замера B8 (решение G1). До замера метрики §3.5 — «измеряем».
6. **Порог нарратива для группы из 3** (`narrative_latency_group_p95_ms`) —
   архитектор после B1; ориентир «не хуже соло более чем в 2 раза» — PA.
7. **Кто рендерит нарратив** (персональный GM `task` или региональный `domain`;
   narrative-orchestrator как тонкий рендер или выводится) — архитектор; от
   этого зависит значение `narrative.agent_level` и место сборки `absence{…}`
   (§4.2), но не формулы метрик.
8. **Как считать «отражено в нарративе»** без человека: MVP-1 — `background_refs[]`
   из структурированного вывода + выборочная проверка; целевое — LLM-судья
   (как для `narrative_mechanics_mismatch_per_100`). Требует, чтобы схема вывода
   персонального GM содержала ссылки на события (п. 16 §8) — подтвердить у BA/архитектора.
