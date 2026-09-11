# ADR-003: Владение состоянием и record-replay — единственный писатель, снапшот + журнал, seed бросков, время из событий

Статус: предложено (к утверждению на G2) · Дата: 2026-09-09 · Автор: system-architect#1
Связи: OQ-A-03 (решено р2), OQ-D-01, OQ-D-08; FR-021, FR-022, FR-030…FR-034, FR-100; BR-03, BR-04; NFR-010…NFR-014, NFR-060, NFR-061; `data-model.md` §1, §3, §7, §8; `domain-review.md` DR-05, DR-13.

## Контекст

As-is: сущности пишут entity-manager и game-service (оба в `entities-{world}`), entity-actor хранит копии; entity-manager ничего не публикует; версий нет; replay не реализован; rule-engine seed-ится от `time.Now()`; LLM-ответы не сохраняются. Решения пользователя: entity-manager — единственный писатель; снапшот — истина; выводы LLM и броски записываются как события; replay не вызывает LLM. Аналитик задал контракты `entity.*.proposed/updated/rejected`, `dice.rolled`, `llm.output`, `tick.fired`, `round.closed`, `snapshot.created`; развилка — функция хэша seed.

## Рассмотренные варианты

| Вариант | Плюсы | Минусы |
|---|---|---|
| A. Снапшот — истина, журнал фактов для догона; предложения не переигрываются; record-replay недетерминированных источников | соответствует решению р2; replay дёшев (только факты + записи); аудит промптов | нужна дисциплина «записать до использования» во всех точках недетерминизма |
| B. Полный event-sourcing: состояние = свёртка всех событий, retention «навсегда» | «истина в журнале» | retention бесконечен; каждое изменение схемы события = миграция свёртки; отклонено р2 |
| C. Только снапшоты, без replay | проще | теряется история после снапшота (RPO > 0), нет воспроизводимых тестов; отклонено р2 |
| Seed: `hash(event_id)` для всех бросков события | просто | броски одного события коррелируют (DR-05) |
| **Seed: `hash(event_id, roll_index)`** | независимые броски; воспроизводимость по паре | нужен счётчик `roll_index` в событии-причине |
| Хэш: FNV-64 | быстро | слабое перемешивание близких входов |
| **Хэш: SHA-256 → первые 8 байт** | стандарт, детерминирован, есть в stdlib | микросекунды на бросок (незначимо) |

## Решение

1. **Единственный писатель**: контекст State (`internal/state`) — единственный, кто пишет сущности мира. Вход: `entity.create.proposed`, `entity.update.proposed` (с `proposal_id`, `changes[]{entity, expected_version?, ops[]{set|inc|append|remove}}`, `atomic`, `cause`, `meta.agent`). Выход: `entity.created`, `entity.updated` (по одному на сущность, общий `proposal_id`), `entity.update.rejected {reason: version_conflict|unknown_entity|level_violation|law_violation|invalid_op|dead_entity}`. Проверки: `expected_version` (optimistic lock, `version` строго +1), владение по уровню агента (`data-model.md` §4: таблица «кто что может»), инварианты `laws@v1` (clamp HP → `law_violation` при явном нарушении, единственность трофея по `item.source`, позиция группы = позиции участников при `atomic`, игрок ровно в одном scope, `dead` не меняется кроме `history`), `dead_entity`.
2. **Хранилище**: State держит рабочий набор мира в памяти (`map[entityID]*Entity`, ≤ тысяч записей); каждое применённое изменение — PUT объекта `entities-{world}/{type}/{id}.json` (версия в теле, bucket versioning включён) **до** публикации факта; факт публикуется после успешного PUT. Снапшот мира — объект `snapshots-{world}/state/{ts}-{seq}.json` (все сущности, `laws_version`, `cursor{topic: offset}`, `state_hash` = SHA-256 по каноническому JSON HP/инвентаря/позиции/статуса всех сущностей, `size_bytes`) + событие `snapshot.created`; периодичность: каждые `N=200` применённых фактов, по `analytics.session.ended`, по `SIGTERM`; хранить последние `K=5`.
3. **Восстановление State**: последний снапшот → чтение `system_events` с `cursor` → применение только фактов `entity.created/updated` (идемпотентно по `event.id`; версия строго +1, иначе `analytics.consistency.violated state_divergence`) → `analytics.replay.completed {mode: recovery, identical, state_hash_after}`. Предложения (`*.proposed`) при восстановлении **не** переигрываются — они уже породили факты или были отклонены.
4. **Record-replay недетерминизма** (BR-04): LLM-шлюз пишет `llm.output` **до** возврата ответа конвейеру; Mechanics публикует `dice.rolled` на каждый бросок до публикации `combat.decided`; Scheduler публикует `tick.fired` до запуска тика; Gateway публикует `round.closed` до обработки раунда. В режиме `--mode=replay`: провайдер LLM = `RecordedProvider` (ключ `correlation_id+agent.id+phase+attempt`), RNG = seed из события (см. п. 5), `Clock` = `EventClock` (время последнего прочитанного `tick.fired`/`round.closed`/действия), планировщик и таймеры раунда выключены, `tick.fired`/`round.closed` читаются из журнала; счётчики `llm_calls=0`, `dice_rolled_new=0` попадают в `analytics.replay.completed`.
5. **Seed и RNG** (одна функция в `internal/mechanics/rng.go`):
   - `seed(event_id, roll_index) = binary.BigEndian.Uint64(sha256(event_id + ":" + strconv.Itoa(roll_index))[:8])`;
   - `r := rand.New(rand.NewPCG(seed, 0))` (`math/rand/v2`), бросок `dN = 1 + r.IntN(N)`;
   - `event_id` — id события-причины броска (`player.attacked` для соло-хода; `round.closed` для раунда группы; `tick.fired` для `encounter_chance`), `roll_index` — порядковый номер броска внутри причины (0…); в `dice.rolled` пишутся `seed`, `formula`, `result`, `natural`, `purpose`, `roll.index`; тест NFR-060: 1000 событий × 4 броска — детерминизм и отсутствие корреляции.
6. **Время**: все контексты используют интерфейс `Clock{Now() time.Time}`; производные события наследуют `timestamp` причины при replay; таймауты раунда и TTL агентов — таймеры поверх `Clock`, в replay не срабатывают (читаются записанные `round.closed`, `agent.stopped`).
7. **Снапшот роя** — в Swarm (ADR-002): `AgentInstance[]`, `tick_seq`, `next_tick_at`, бюджет окна, версии блупринтов, `cursor`; восстановление — снапшот + события scope после курсора в режиме replay до «конца журнала», затем переключение в `live`.
8. **Gateway** восстанавливает свои данные из `gateway.db` (ADR-004) и read-model из снапшота State + `entity.updated`; активные сессии — из `analytics.session.started` без парного `ended` (metrics.md §4.2), либо закрывает их `end_reason=error`.

## Последствия

- Позитивные: RPO = 0 для подтверждённых событий; воспроизводимые e2e без GPU (NFR-061); аудит промптов; конфликт двух GM за одну сущность решается версией (BR-16 п. 4).
- Негативные: каждый вызов LLM порождает событие ≤ 4 КБ (retention `llm_records` 90 дней — ADR-007); PUT на каждое изменение (≤ 20 мс; при росте — батч-запись за интерфейсом `Store`).
- Что придётся сделать: EPIC-002 (State, Mechanics, `internal/replay`, `mvctl replay`, тесты NFR-060/061); ADR-007 — конверт `meta.replay`; `shared/entity` получает `Version`, `LastEventID`, типизированные `ops`.
