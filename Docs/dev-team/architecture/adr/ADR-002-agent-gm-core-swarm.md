# ADR-002: Agent GM Core как единственный runtime GM — рой уровней, персональный GM = `task`, нарратор scope, встреча как Task-агент

Статус: предложено (к утверждению на G2) · Дата: 2026-09-09 · Автор: system-architect#1
Связи: OQ-A-02 (решено р1/р5, остаток — здесь), OQ-P-10, OQ-R-13, OQ-R-19 (G1), OQ-D-11 (LOD — целевое); FR-010…FR-017, FR-120…FR-128, FR-012, FR-013, FR-123; BR-10, BR-12, BR-16; NFR-006, NFR-007, NFR-083; vision.md D7/D8; `api-contracts.md` §3; `data-model.md` §6.

## Контекст

Пользователь решил: Agent GM Core (`shared/agent`) — единственная архитектура GM; GM — иерархический рой (глобальный → региональный/городской → персональный); фоновая жизнь мира в MVP-1; один вызов Phase 2 на раунд группы (A-13). Открытыми для архитектора остались: уровень кода персонального GM (`task` vs `monitor`), кто нарратор раунда в группе, встреча «волк» — отдельный Task-агент или часть регионального GM, минимальный объём `shared/agent`, TTL регионального GM (в коде ~1 ч, а он должен жить вечно).

По коду `shared/agent`: типы уровней/LOD/жизненного цикла, парсер MD, каркас Router/Lifecycle/WorkerPool/Pipeline есть, но: Router матчит по имени события без scope, Lifecycle не реализует Stop/Pause/Cleanup, Pipeline хардкодит `qwen:7b/72b` и переменные промпта, StateManager — заглушки Redis/PG, `filter.go` — недоделанный обход LLM правилами, `monitor` в `Lifecycle.Start` — без TTL.

## Рассмотренные варианты

| Вопрос | Вариант | Плюсы | Минусы |
|---|---|---|---|
| Уровень персонального GM | `task` (TTL = сессия + запас) | совпадает с FR-016 (TTL, сон без игроков), с кодом (`task` = TTL-агент), `monitor` остаётся стражу | «монитор видимости игрока» семантически ближе к monitor |
| | `monitor` | название по смыслу | в коде `monitor` без TTL; смешивает роль стража и роль рассказчика |
| Нарратор группы | Task-агент `group-narrator` на scope `group` | один владелец `narrative_event_id`; персональные GM не конкурируют; ≤ 2 вызова на раунд | ещё один блупринт |
| | персональный GM лидера | нет нового агента | смена лидера меняет нарратора; логика «кто главный» в каждом персональном GM |
| | региональный GM | уже есть | смешивает фон региона и нарратив группы; при двух группах в регионе — конфликт бюджета |
| Встреча | отдельный Task-агент `encounter-wolf` (parent = region-gm) | чистое владение `Encounter`/HP участников; `agent.child_resolved`; масштабируется на N встреч | +1 агент на встречу (дёшево: `phase1.mode: rules`) |
| | внутри регионального GM | меньше агентов | региональный GM владеет и фоном, и боем всех групп региона; сложнее TTL и владение |
| Объём `shared/agent` | переписать Router/Lifecycle/Pipeline, удалить StateManager/filter/WorkerPool, оставить типы/парсер/LOD | берём проверенное, не тащим заглушки | часть «Phase 1–4» README обесценивается |
| | довести всё как есть | «уже написано» | 45 TODO, Redis/PG заглушки, не совпадает с контрактами A2 |

## Решение

1. **Уровни и роли MVP-1** (`AgentLevel` из кода): `global` — `global-gm` (1 на мир, тик 60 мин, без TTL); `domain` — `region-gm` (1 на регион, тик 30 мин idle / 60 с active, без TTL; `ttl` в блупринте игнорируется с предупреждением); `task` — `encounter` (1 на встречу scope, TTL 30 мин, `phase1.mode: rules`), `personal-gm` (1 на игрока, scope `solo:{player_id}`, TTL 45 мин от последнего действия), `group-narrator` (1 на группу, scope `group:{id}`, TTL 45 мин от последнего действия любого участника). `monitor` и `object` — зарезервированы (страж-аномалий E-B/E-A, Entity-Actor E-A): реестр уровней их знает, блупринты валидируются, спавн выключен флагом.
2. **Персональный GM = `task`**, роль `personal-gm`. Спавн — роутером по первому событию `player.*` scope `solo:{player_id}` без живого агента (в группе — тоже, но в LOD `rule-only`). Родитель — `region-gm` региона текущей позиции; при `outside` — `global-gm`. Публикует только `narrative.output` (`kind: turn|entry|death|world_event`), никогда `player.*` и `*.proposed` (BR-06, FR-123).
3. **Нарратор scope**: `solo` → персональный GM (Phase 2 на ход); `group` → `group-narrator` (Phase 2 по `round.closed`, один `narrative.output` с `recipients[]` всех `active`/`idle`-участников и одним `narrative_event_id`). Персональные GM участников группы: LOD `rule-only`, генерируют только `kind: entry` при `enter`/возврате после паузы и `kind: death`. Итого в группе ≤ 2 вызова LLM на раунд (NFR-006).
4. **Встреча** — отдельный Task-агент `encounter-wolf` (child `region-gm`); спавн по `encounter.started`, завершение `encounter.ended` + `agent.child_resolved`. Механика вызывается агентом как библиотека (`mechanics.Resolve`), результаты публикуются агентом (`dice.rolled`, `combat.decided`, `entity.update.proposed atomic`).
5. **Рантайм роя** (`internal/swarm`): Router (матчинг по `scope_binding`, `parent`, `trigger`, `max_instances=1`, детерминированный `agent.id = "{blueprint}:{scope.id}"`), Lifecycle (спавн/TTL/стоп с событиями `agent.*`), Scheduler (тики, две очереди `interactive`/`background`, бюджет `B`/час/мир, `lod_allowed`), Pipeline (Phase 1 rules → Phase 2 через LLM-шлюз с record-replay и стражем; тик по схеме `tick-*.json`), Snapshot (AgentInstance[] + tick_seq + бюджет → MinIO). Контекст агента = состояние (read-only копия из `entity.*`) + окно последних N событий scope и родителя + законы + канон блупринта; memory — обогащение (ADR-004).
6. **Из `shared/agent` переиспользуем**: `AgentLevel`, `LODLevel`, `AgentLifecycleState`, `AgentBlueprint` (расширить полями §3 api-contracts: `level`, `role`, `scope_binding`, `trigger.intervals`, `llm.{phase1,phase2,tick}`, `allowed_event_types`, `owned_entity_types`, `laws_ref`, `rules_ref`, `npc_table`, `background_events`, `budget`, `invariants`, `round`, `locale`), `md_parser` (frontmatter + секции `## system/phase1/phase2/tick/canon/description`), `blueprint_validator` (переписать правила под §3.2), `lod.go` (E-G). **Удаляем**: `state_manager.go`, `filter.go`, `worker_pool.go` (заменяется очередями планировщика), `tools/*` кроме реестра (MVP-1 без tools), мосты в narrative-orchestrator.
7. **Фича-флаг миграции** `GM_PATH=agent|legacy` (US-019): Gateway публикует `gm.created` только при `legacy`; legacy narrative-orchestrator — compose-профиль `legacy`; флаг и профиль удаляются в инкременте «группа» (S5).
8. **LOD в MVP-1** фиксированный: бой `rule-only`; тики `basic` → `rule-only` при исчерпании бюджета; персональный GM `basic` (Phase 2); адаптивный двухосный LOD (OQ-D-11 C) — E-G.

## Последствия

- Позитивные: один рантайм, один формат блупринта, все GM — данные; S6 (регион блупринтом без Go) достижим; бюджет LLM предсказуем (≤ 2 вызова/раунд, ≤ B/час фон); replay единообразен.
- Негативные: `shared/agent` теряет ~40 % кода (StateManager, filter, worker pool, tools); блупринты `configs/gm_*.yaml` и `examples/domain-dark-forest.md` переписываются под контракт §3.
- Что придётся сделать: EPIC-003 (рантайм, 5 блупринтов MVP-1: `global-dark-forest-world`, `domain-dark-forest`, `encounter-wolf`, `player-gm`, `group-narrator`; схемы `tick-global.json`, `tick-region.json`, `narrative.json`); реестр уровней с зарезервированными `monitor`/`object`; `mvctl blueprint validate`.
