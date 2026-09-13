# ADR-012: Механика как данные — формат `rules/dark-forest.yaml`, мини-грамматика формул вместо движка выражений, один RNG-модуль, единая реализация инвариантов с идентификаторами `laws@v1`

Статус: предложено (к ревью system-architect, утверждение на G3) · Дата: 2026-09-09 · Автор: architect#1 (TEAM-1, EPIC-002)
Уровень: реализация внутри блока EPIC-002 (в границах ADR-003 п. 5, C-03, BR-05, FR-020…FR-022).
Связи: NFR-020, NFR-060, NFR-061; `state-and-mechanics.md` §5; `data-model.md` §6.3, §10; `domain-review.md` §3.3; PRD приложение A; аудит `shared/rules`, `services/rule-engine`, `shared/agent/filter.go` (три движка правил, T10).

## Контекст

FR-020 требует правила боя v0.1 как данные с `damage_formula`; FR-021/022 — детерминированный RNG и `dice.rolled` на бросок; NFR-020 — десять инвариантов как первые законы `laws@v1`; C-03 фиксирует Go-API `Rules/Resolve/NPCTarget/Seed/NewRNG/Invariants`. As-is: `shared/rules.Engine` и `ruleengine.Engine` — JSON-правила из MinIO с произвольными строковыми условиями (`evaluateCondition` парсит `"environment == 'intimate'"`), seed от `time.Now()`, LRU-кэш; `shared/agent/filter.go` — третий фильтр. Системный архитектор просил: `Invariants()` — общий список с `laws@v1`, без дублирования.

## Рассмотренные варианты

| Вопрос | Вариант | Плюсы | Минусы |
|---|---|---|---|
| Формат правил | A. переиспользовать `shared/rules.Rule` (`mechanical_core.dice_formula`, `success_threshold`, `state_changes`) с добавлением `damage_formula` | код есть | правила в MinIO (не в Git — нет ревью/версий), строковые условия общего вида, `state_changes` с константами, кэш и I/O внутри движка, seed от часов |
| | **B. новый `RulesDocument` YAML в Git** с явными секциями `entities/attack/npc_attack/flee/rest/npc_target/loot/round/invariants` | читается автором без Go; `rules_version` в событиях; валидация при загрузке; тесты на золотые числа | нужно написать ≈ 400 строк вместо переиспользования |
| Язык формул | A. общий движок выражений (`expr`, `govaluate`, CEL) | гибко | новая зависимость, недетерминизм типов float, широкая поверхность для ошибок/инъекций из блупринтов; v0.1 нужны ровно две формы (`dice + attr >= attr [+ var]`, `dice`) |
| | **B. мини-грамматика** `check := dice ('+' term)* '>=' term ('+' term)*`, `dice := [N]dM[±K]` | 100 строк парсера, целочисленная арифметика, ошибки при `Load`, полное покрытие тестами | расширение грамматики при новых правилах (E-A/E-C) — осознанная задача с ревью (FR-115) |
| RNG | A. `math/rand` (v1) с `Seed` | привычно | глобальное состояние, устаревший API |
| | **B. `math/rand/v2` PCG(seed, 0)**, один RNG на `roll_index` | без глобального состояния; PCG стабилен между версиями Go; независимые броски по seed из SHA-256 | — |
| Инварианты | A. реализовать в State как проверки при применении, а `laws@v1` описать текстом отдельно | просто | два списка (State и законы) расходятся; `mechanics.Invariants()` из C-03 пуст |
| | B. описать инварианты декларативно в YAML законов и интерпретировать | «всё данные» | интерпретатор инвариантов = ещё один движок выражений над состоянием |
| | **C. одна реализация в `internal/mechanics/invariants.go` с `ID = law id`; законы содержат текст и `kind: invariant`; `rules.invariants[]` включает id; тест сверяет множества** | код один, текст один, ID общие; State и тесты вызывают один список | инварианты живут в `mechanics`, поэтому `state → mechanics` — допустимая библиотечная зависимость (запрос к ADR-001 п. 3) |
| Где публикуется `dice.rolled` | A. State/мechanics публикует сам | «владелец типа = издатель» | mechanics — чистая библиотека без шины; в конвейере агента порядок «dice.rolled до combat.decided» проще гарантировать вызывающему |
| | **B. вызывающий (агент встречи, GM региона) публикует, payload строит `mechanics.DiceRolledPayload`** | библиотека без I/O; схема и структура — у EPIC-002 | издатель события — не владелец типа (уже так по C-03) |

## Решение

1. `rules/dark-forest.yaml` — `RulesDocument` (`state-and-mechanics.md` §5.2): `schema_version`, `rules_version`, `world`, `entities{kind → stats}`, `attack{hit, crit_natural, crit_multiplier, fumble_natural, damage_formula, rolls[]}`, `npc_attack{inherit, once_per_round}`, `flee{check, rolls, on_fail, success_position}`, `rest{restore, allowed_in_encounter}`, `npc_target{order, exclude}`, `loot{kind → items[]}`, `round{timeout, idle_after_missed}`, `invariants[]` (id). Числа — приложение A PRD; изменение чисел — правка YAML и обновление золотых тестов. Неизвестные ключи и неразбираемые формулы — ошибка `Load` (не предупреждение).
2. Формулы — мини-грамматика §5.3, целочисленная; идентификаторы слева резолвятся из атакующего, справа — из цели и контекста (`living_enemies`); `damage_formula` — имя атрибута с dice-выражением или dice-выражение. Общий движок выражений не вводится; `shared/rules`, `services/rule-engine`, `shared/agent/filter.go` не переносятся (удаление/заморозка — OQ-A-17).
3. RNG — `internal/mechanics/rng.go`: `Seed(eventID, idx) = BigEndian(sha256(eventID ":" idx)[:8])`, `NewRNG = rand.New(rand.NewPCG(seed, 0))`, бросок `NdM+K` = сумма `1 + IntN(M)` на одном RNG данного индекса; `Natural` — первый d20. Единственная функция детерминизма в проекте; другие пакеты (фоновые таблицы GM региона, шанс встречи) используют `Rules.Roll` с собственными `purpose`.
4. `Resolve` — чистая функция таблицы исходов §5.4; `NPCTarget` — `last_damager → min_hp → id asc` с исключением `dead|idle|out_of_combat`; `ChangesFor` строит ops для `entity.update.proposed` (HP `set` clamped, `status`, `died_at/killed_by`, `inventory append` с `source{entity, event_id}`, `encounter.npcs[].last_damager`). *Дополнено C-03 v1.3 (2026-09-13, T-449):* `encounter.npcs[].last_damager` и всё остальное в сущности встречи пишет пакет агента встречи EPIC-003, а не `ChangesFor`; `killed_by` и `died_at` — только у NPC; `expected_version`, `died_at`, `acquired_at` `ChangesFor` берёт из `Actor.Version` и `Action.At`; позицию побега предлагает вызывающий через `Rules.FleePosition`.
5. Инварианты — реестр `inv-01…inv-10` в `internal/mechanics/invariants.go` (`Invariant{ID, Where[], Check}`); `Check == nil` для проверяемых вне State (inv-07, inv-08). `laws/dark-forest-world.v1.yaml` (EPIC-003) содержит `{id: inv-NN, kind: invariant, text}` без логики; тест `mvctl laws check` сверяет множества id. State вызывает `rules.Invariants()` над `WorldView` с `touched` для каждого предложения; отказ — `law_violation {invariant_id}`.
6. `dice.rolled` публикует вызывающий (EPIC-003) через `Derive(cause, "dice.rolled", mechanics.DiceRolledPayload(roll, roller))` до `combat.decided`; схема `schemas/events/dice.rolled.v1.json` — EPIC-002.

## Последствия

- Позитивные: правила читаемы и ревьюируемы в Git; детерминизм проверяется одним тестом 1000 × 4; инварианты и законы не расходятся по построению; нет I/O и часов в механике — `Resolve` тестируется таблицей.
- Негативные: грамматика намеренно узкая — новые виды проверок (навыки, сопротивления E-C) потребуют её расширения через ревью; три as-is движка правил выбрасываются.
- Что придётся сделать: `internal/mechanics/{rules,formula,rng,resolve,target,invariants,actor,changes,dice_event}.go`, `rules/dark-forest.yaml`, `testkit/mechanics.FixedMechanics`; согласовать с EPIC-003 формат `laws/*.v1.yaml` (`kind: invariant`, id) и `stats_ref` блупринта → `entities.<kind>`; запрос в ADR-001 п. 3 на `state → mechanics`.
