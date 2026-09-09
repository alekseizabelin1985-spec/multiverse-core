# ADR-017: Страж как чистая функция над структурированным выводом — правила 3–6, частичное отклонение, видимость по уровню, `reason` enum и инварианты по `check`-ключам законов

Статус: **принято с дополнением 1** (ревью system-architect#1, сведение 3, 2026-09-09) · Дата: 2026-09-09 · Автор: architect#2 (TEAM-2, EPIC-003) · Уровень: реализация блока
Связи: ADR-002 п. 5, ADR-005 п. 2, ADR-008 п. 2/3/5, ADR-009 п. 6; FR-034, FR-042, FR-122, FR-123, FR-128, BR-06, BR-16; NFR-020, NFR-021, NFR-026, NFR-043; UC-022; `api-contracts.md` §2.4, §2.3.10; `data-model.md` §4; C-02 (`OwnershipRules`), C-03 (`Invariants()`), C-12; `components/swarm-llm-laws.md` §10.

## Контекст

Страж MVP-1 — автомат-валидатор в конвейере LLM-шлюза (FR-042 Should как валидация): существующие и видимые сущности, отсутствие `player.*`, белые списки уровня, инварианты `laws@v1`, актуальность `laws_version`. Схема Phase 2 не содержит действий (ADR-009 п. 6), поэтому для нарратива проверяются только структурные ссылки; для тиков — типы событий и `ops`. FR-034 перечисляет `reason` без `law_violation`/`filter_blocked`, `api-contracts.md` §2.3.10 — с ними. Список инвариантов реализует `mechanics.Invariants()` (C-03), а набор действующих законов — `laws@vN`; нужно решить, кто что проверяет, чтобы не дублировать код инвариантов в State и страже.

## Рассмотренные варианты

| Вопрос | Вариант | Плюсы | Минусы |
|---|---|---|---|
| Гранулярность | отклонять весь ответ при любом нарушении | просто | один «призрак» в `affects[]` губит тик целиком; лишние повторы |
| | **элементная**: отбрасывается элемент (`events[i]`, `mentions[i]`, `background_refs[i]`), остаток применяется; `partially_rejected` | UC-022 п. 9; меньше вызовов | нужен `element{index,type}` в `llm.output.rejected` (есть в контракте) |
| Где проверять инварианты | страж дублирует код проверок | независимость | два набора кода расходятся |
| | **страж вызывает `mechanics.Invariant` по `id`, набор берёт из `laws.Current().Laws[kind=invariant].check`** | один код (EPIC-002), набор — данные (законы); State проверяет то же при применении (последняя линия) | `internal/laws` не импортирует `mechanics` — сопоставление делает страж; неизвестный `check` → `/health degraded` |
| Видимость | «сущность существует в мире» | просто | региональный GM «видит» игроков другого региона; нарратив упоминает волка из соседнего леса |
| | **существует и видима по уровню/scope** (таблица §10.3) | FR-003, FR-122 | таблица видимости — код на роль (5 строк) |
| `laws_version` устарела | отбросить и шаблон | безопасно | лишняя деградация из-за гонки с `world.laws.changed` |
| | **`invalid` + один повтор с новой версией** | корректный вывод под текущими законами | +1 вызов в редком случае |
| Побочные эффекты | страж публикует `llm.output.rejected` сам | локально | страж зависит от шины; трудно тестировать |
| | **страж — чистая функция `Evaluate(value, Input) Verdict`; публикует шлюз** | unit-тесты без шины; вызов до записи даёт окончательный `validation_status` | — |

## Решение

1. **Интерфейс**: `guardian.Evaluate(value json.RawMessage, in Input) Verdict{Value, Rejected []Rejection{Reason, Element{Index, Type}, Entity, Details{InvariantID}}, Status}`; `Input{Level, Role, Scope, Allowed, Owned, View StateView, Laws LawsVersion, Invariants map[id]mechanics.Invariant, AbsenceEventIDs, Kind narrative|tick}`. Без I/O и без публикаций.
2. **Правила** (номера — §2.4 api-contracts):
   - 3 `unknown_entity`: `mentions[]`/`background_refs[]` (narrative) и `entity`/`affects[]`/цели `ops` (tick) должны существовать в `View` **и** быть видимы агенту по таблице видимости (global: мир и регионы; domain: регион, его NPC/встречи/игроки в регионе; encounter: участники и NPC встречи; personal-gm/group-narrator: игрок(и), группа, регион позиции, его живые NPC, встреча, мир). Невидимая ссылка → элемент удаляется (`mentions`/`background_refs` — только элемент, текст остаётся; `events[i]` — целиком).
   - 4 `player_agency`: `events[i].type` с префиксом `player.` или `ops` над сущностью типа `player` → элемент отброшен (для нарратива правило структурно неприменимо).
   - 5 `level_violation`: `events[i].type ∉ Allowed`; `ops` над типом ∉ `Owned` или над сущностью вне scope агента (`domain` — только свой регион и его NPC; `task` — только участники встречи) → элемент отброшен.
   - 6 `law_violation`: (а) `in.Laws.Version ≠ Laws.Current` → весь ответ `invalid`, шлюз повторяет один раз с обновлённой версией; (б) для каждого `events[i]` с `ops` — гипотетическое состояние `View + ops` проверяется инвариантами, чьи `check`-ключи присутствуют в `in.Laws` (`inv-1 dead_does_not_act`, `inv-2 hp_in_range`, `inv-9 no_respawn_in_ttl`, `inv-10 single_position` и т. д.); провал → элемент отброшен с `details{invariant_id}`, `laws.Strain().Inc(lawID)` (ADR-008 п. 5). `inv-11 laws_version_current` реализован стражем (не механикой).
   - Статус: `valid` (ничего не отброшено) · `partially_rejected` (часть) · `invalid` (ничего не осталось или (а)). Для `tick` с `invalid` роль переходит в `rule-only`; для `narrative` с `invalid` — повтор/шаблон.
3. **`reason` enum** (единый для схемы `llm.output.rejected.v1.json` и `guardian/reasons.go`): `unknown_entity | player_agency | level_violation | schema_invalid | language | filter_blocked | filter_error | budget_exceeded | law_violation | other` — по `api-contracts.md` §2.3.10 (надмножество FR-034; замечание BA — добавить `law_violation`, `filter_blocked` в FR-034).
4. **Три рубежа** (ADR-008 п. 3) не дублируют код: страж проверяет *предложения LLM до публикации* (набор — законы, код — `mechanics.Invariants()`), State — *применение* (те же функции, C-03), `mvctl --audit` — *сверку* (EPIC-005). `Emitter` роя дополнительно проверяет белые списки для кода ролей (§5.4 компонентного документа) — дефект кода, не отклонение LLM.
5. **Замер**: `guardian_rejections_per_100` по `reason` из `llm.output.rejected` (NFR-021); правило 9 (числа нарратива ≠ механики) — не рантайм, а метрика NFR-023.

## Последствия

- Позитивные: страж тестируется таблично без шины и LLM (9 правил × 2 вида схем); NFR-021 «100 % событий из `llm.output` содержат только зарегистрированные id и свой уровень» обеспечивается до публикации; законы управляют набором проверок без правки Go.
- Негативные: видимость — ещё одна таблица на роль (при новой роли — правка); гипотетическое применение `ops` требует небольшой копии сущности (≤ 50 сущностей/scope — дёшево).
- Что придётся сделать: `internal/llm/guardian/{guardian,visibility,reasons}.go`; схема `llm.output.rejected.v1.json`; проверка полноты `check`-ключей при старте `core`; тесты с подложенными ссылками (US-018, US-037).

## Дополнение 1 (сведение 3, 2026-09-09, system-architect#1) — единый `validation_status`, связь статуса и причин, покинутые персонажи

Контекст: `data-model.md` §7.2 v0.2 задавал 11 значений `validation_status` (в т. ч. `rejected_unknown_entity`, `rejected_player_agency`, `rejected_level_violation`, `rejected_language`, `budget_exceeded`), КД §9.2/§10.2 — 7 (`… | rejected_language | error`), п. 2 этого ADR — 3 исхода стража. Замечание tech-lead#2 (EPIC-003 tasks §10 п. 1): нужно одно перечисление до T-215. US-018 повторяет вариант `data-model`.

Решение (C-07 v1.2):
1. **Статус описывает исход конвейера, причина — почему.** `llm.output.validation_status ∈ valid | partially_rejected | invalid | error | quarantined | filter_error` (6 значений):
   - `valid` / `partially_rejected` / `invalid` — исходы стража и парсера (п. 2 этого ADR: `invalid` также при `schema_invalid`, `language` и устаревшей `laws_version`);
   - `error` — ответа нет (провайдер, таймаут, `error{code: yielded}` по ADR-014 п. 2); стражу нечего проверять;
   - `quarantined` — фильтр (a) `block` (без `response_raw`, `content.incident.recorded`); `filter_error` — сбой фильтра, fail-closed (без `response_raw`). Оставлены статусами, а не причинами, потому что от них зависит хранение `response_raw` (FR-050, SEC) и ветка конвейера без повтора.
2. **Причины — только в `llm.output.rejected.reason`** по п. 3 (`unknown_entity | player_agency | level_violation | schema_invalid | language | filter_blocked | filter_error | budget_exceeded | law_violation | other`); по одному событию на отброшенный элемент (`element{index,type}`), без `element` — на весь ответ. `rejected_*`-статусы удалены: они дублировали `reason` и не позволяли выразить «два элемента по разным причинам». `budget_exceeded` — не статус: `llm.output.rejected` без `llm.output` (вызова не было), как в §9.2 КД.
3. **`llm.output.reasons[]` (опц.)** — множество `reason` связанных `rejected`; заполняет шлюз (страж вычислен до записи — п. 5 §9.2 КД), чтобы `mvctl llm-usage`/`session-report` считали `guardian_rejections_per_100` без join по `llm_output.event.id`.
4. **US-018 (запрос BA)**: критерий «`validation_status = rejected_unknown_entity`» → «`validation_status ∈ partially_rejected|invalid` и `llm.output.rejected reason=unknown_entity` с отсутствующей ссылкой»; критерий про повторы — `invalid` (без изменений). `data-model.md` §7.2/§9.7 и `api-contracts.md` §2.3.10 приведены system-architect в сведении 3 (пометка).
5. **Покинутые персонажи (C-02 v1.2, FR-061)**: `status=abandoned` для правил стража эквивалентен `dead` — сущность видима (нужна для «его больше нет»), `ops` над ней → `law_violation dead_does_not_act` (inv-01 расширен на `status ∈ dead|abandoned|ascended_final`); таблица видимости §10.3 КД без изменений.

Последствия: схема `llm.output.v1.json` (T-215) — enum из 6 значений; `guardian.Verdict.Status` — 3 значения (без изменений); шлюз сводит статусы парсера/фильтра/провайдера и вердикт в один `validation_status`; замер NFR-021/NFR-023 — по `llm.output.rejected` (+ `reasons[]`).
