# Команды

Версия 0.1 · 2026-09-09 · tech-lead#1 · к утверждению на G2 вместе с `plan/epics.md` и `plan/ownership.md`.
Лимиты `.dev-team.json` → `parallelism`: maxTeams **3** · maxAgentsPerRole **3** · maxParallelAgents **6**. Политика git: `branches=true`, `commits=ask`. Решение пользователя: три команды (TEAM-1 фундамент + состояние/механика + память; TEAM-2 рой GM/LLM/законы; TEAM-3 gateway/бот). Команды виртуальные: разработчик-человек один, он же ревьюер последней инстанции и оператор стенда. Нумерация `counters.team` = 0 → после утверждения = 3.

## 1. Состав команд

| Команда | Эпики | Ветка | Роли × экземпляры | Владение (кратко; полностью — `plan/ownership.md`) |
|---|---|---|---|---|
| **TEAM-1** «Фундамент, состояние, память» | EPIC-001 (волна 0) → EPIC-002 (волны 1–2) → EPIC-005 (волна 2; 005-ops, затем 005-memory) | `epic/EPIC-001-foundation` → `epic/EPIC-002-state` → `epic/EPIC-005-memory-ops` | tech-lead#1 (он же тимлид проекта), architect#1, developer#1…#3 (волна 0: до 3; волны 1–2: 2), code-reviewer#1, qa-engineer#1, tester#1 | `shared/{eventbus,jsonpath,contracts,objstore,env,logging,runtime,clock,testkit-ядро}`, `cmd/multiverse`, `build/`, compose, CI, `Makefile` (после волны 0 — через tech-lead#1); `shared/entity`, `internal/{state,mechanics,replay}`, `rules/`, `schemas/events/entity.*`, `dice.rolled`, `snapshot.created`; `internal/memory`, `cmd/mvctl` (реестр), `ops/metrics/`, `testdata/analytics/`, golden |
| **TEAM-2** «Рой GM, LLM, страж, законы» | EPIC-003 (I1a → I1b → I2) | `epic/EPIC-003-swarm` | tech-lead#2, architect#2, developer#1…#3, code-reviewer#2, qa-engineer#2, tester#2 | `shared/agent`, `internal/{swarm,llm,laws}`, `blueprints/`, `laws/`, `schemas/agent/`, `schemas/events/<типы EPIC-003>`, `config/absolute-limits.yaml`, `shared/testkit/swarm`, `testdata/{recordings,llm,golden}`, `services/narrative-orchestrator` (только на время профиля `legacy`) |
| **TEAM-3** «Вход игрока: gateway и бот» | EPIC-004 (I1 → I2) | `epic/EPIC-004-gateway` | tech-lead#3, architect#3, developer#1…#2, code-reviewer#3, qa-engineer#3, tester#3 | `internal/gateway/**` (в т. ч. `migrations/*.sql`, `client`), `cmd/telegram-bot/**`, `api/gateway.openapi.yaml`, `schemas/events/{player,group,round,analytics.session,analytics.turn}.*`, `shared/testkit/gateway`, `links.db`/`gateway.db`, `snapshots-{world}/gateway/` |

Правила экземпляров (по `parallel-teams.md`):
- номер экземпляра уникален внутри команды (`TEAM-2/developer#1` и `TEAM-3/developer#1` — разные агенты); в промпт каждому — команда, эпик, ветка, фрагмент карты владения, нужные разделы `contracts.md`, номер экземпляра для подписи `dev-log.md`;
- ревьюер не ревьюит задачу своего же экземпляра разработчика; при одном разработчике в волне ревьюер команды ревьюит его задачу (это другой агент);
- число разработчиков в волне = число независимых задач волны, но ≤ 3 на команду и ≤ 6 суммарно;
- тимлид команды (`tech-lead#N`) нарезает задачи своего эпика волнами (`Docs/dev-team/epics/EPIC-NNN/tasks.md`), назначает `developer#K`, принимает задачи; `tech-lead#1` дополнительно — интеграция, карта владения, конфликты в общем коде (вместе с system-architect);
- `architect#N` — детальный дизайн внутри эпика в рамках `contracts.md`; запросы на изменение контрактов — в `ownership.md` §2, решает system-architect.

## 2. Общие роли проекта

product-owner, business-analyst (правки `api-contracts.md`/FR по запросам §14 дизайнов — на планировании), system-analyst, **system-architect#1** (контракты, ADR, ревью дизайнов, конфликты в `shared/*`), security-engineer (F-1 с developer; security-review стыков gateway ↔ core на интеграции; threat-model), devops-engineer (F-6/F-7; сборка интеграционной ветки; бэкапы томов), project-manager (roadmap, risks, граф зависимостей в `epics.md`), **tech-lead#1** (тимлид проекта: интеграция, `plan/**`), tech-writer (F-9; README/CLAUDE.md/AGENTS.md по факту), release-manager (G4 MVP-1).

## 3. Ветки и интеграция

### 3.1. Интеграционная ветка

`integration/mvp-1`, создаётся **от `feature/agent-gm-core`** (текущая рабочая ветка, 6 коммитов впереди `main`).

Обоснование: `feature/agent-gm-core` содержит `shared/agent` (типы, парсер, валидатор, блупринт-пример), который `foundation.md` §11 переносит в единый модуль как есть, и артефакты `Docs/dev-team/**` создавались на ней; `main` (`f7a6bde`) этого кода не содержит. Ветвление от `main` потребовало бы сначала слить туда непроверенную фичу-ветку (отдельное решение пользователя), ветвление от `feature/agent-gm-core` — нет. `main` остаётся стабильной; по G4 MVP-1 `integration/mvp-1` сливается в `main` одним PR (история feature-ветки войдёт в него). Если пользователь предпочтёт сначала влить `feature/agent-gm-core` в `main` — схема не меняется, `integration/mvp-1` создаётся от обновлённого `main`.

Предусловие (F-0, `decomposition-review.md` §5.3): `git status` сейчас падает из-за указателей worktree в индексе и устаревших `.git/worktrees/*`; `Docs/dev-team/**` не зафиксированы. Подготовительный коммит — по подтверждению пользователя (`commits=ask`).

### 3.2. Ветки эпиков

- Одна ветка на эпик на весь MVP-1: `epic/EPIC-001-foundation`, `epic/EPIC-002-state`, `epic/EPIC-003-swarm`, `epic/EPIC-004-gateway`, `epic/EPIC-005-memory-ops`; все — от `integration/mvp-1`. Инкременты I1/I2 не заводят новых веток (отклонение от `epics.md` §5 п. 3): после интеграции I1 ветка эпика подтягивает `integration/mvp-1` (merge, не rebase — история общая с человеком) и продолжает I2. Инкремент отмечается тегом на интеграционной ветке: `mvp-1/wave-0`, `mvp-1/i1-alpha`, `mvp-1/i1`, `mvp-1/i2`.
- Задачи — в ветке эпика напрямую (не по ветке на задачу): агенты одной волны работают в одном рабочем дереве, изоляция — карта владения. Если оркестратору доступны worktree — по одному на команду (`.claude/worktrees/` в `.gitignore` после F-0), слияние в ветку эпика после приёмки задачи.
- Коммиты: только по подтверждению пользователя (`commits=ask`). Практика: тимлид команды после приёмки волны формирует один запрос на коммит с перечнем задач волны (`T-NNN: …`) — меньше запросов, чем на каждую задачу. Агенты не коммитят.

### 3.3. Порядок интеграции (tech-lead#1)

1. Волна 0: `epic/EPIC-001-foundation` → `integration/mvp-1` (тег `mvp-1/wave-0`). Критерий: `make ci` зелёный, `/health` пустых контекстов, contract-тест шины, заглушки v0 (F-10), `baseline.md`. С этого момента `shared/{eventbus,contracts,entity}`, `schemas/events/_common.json` — только через system-architect.
2. Волна 1, точка **I1-α** «соло на шаблонах»: `epic/EPIC-002-state` (I1) → `epic/EPIC-004-gateway` (I1) в `integration/mvp-1`; e2e `solo-30` на `FakeNarrator`; живой прогон через Telegram на стенде (человек). Тег `mvp-1/i1-alpha`.
3. Волна 1, точка **I1**: `epic/EPIC-003-swarm` (I1a+I1b) → `integration/mvp-1` (порядок сохранён: 002 → 004 уже влиты, 003 последним заменяет `FakeNarrator`/`FixedMechanics` реализацией). Прогон S1, S3, S8, S9, S10, S14 (ускоренные тики); qa-engineer#1 — `test-plan-integration.md`, tester — выполнение, security-engineer — стыки gateway ↔ core, devops — сборка. Тег `mvp-1/i1`. Объединённый G3 по планам I2 — до или параллельно (планы I2 команды пишут, пока идёт интеграция I1).
4. Волна 2: слияние **002 → 003 → 004 → 005** (поставщики раньше потребителей: atomic-группы State нужны раунду `encounter`, раунд нужен координатору gateway, память — последней). Прогон S2, S4 (пороги закреплены по замеру), S5, S6; удаление профиля `legacy`/`GM_PATH` — последняя задача EPIC-003 I2 после зелёного S2. Тег `mvp-1/i2` → G4.
5. Дефекты интеграции — задачей в эпик-владелец по карте владения; конфликт в `shared/*` или контракте — system-architect (запрос в `ownership.md` §2), обе команды получают задачи.

## 4. Лимит 6 параллельных агентов: распределение по волнам

Оркестратор запускает до 6 агентов одним сообщением. Разработчики и ревьюеры — разные запуски (сначала волна разработчиков, затем волна ревьюеров ≤ 3, затем приёмка тимлидами), поэтому лимит ниже — для разработчиков; ревьюеры/тестировщики добавляются отдельным запуском в пределах тех же 6.

| Волна / подволна | TEAM-1 | TEAM-2 | TEAM-3 | Прочие в том же запуске | Всего |
|---|---|---|---|---|---|
| 0.1 | developer#1 (F-1), developer#2 (F-3) | — | — | security-engineer (пара к F-1) | 3 |
| 0.2 | developer#1 (F-2) | — | — | devops-engineer (F-6, не зависит от модуля) | 2 |
| 0.3 | developer#1 (F-4a), developer#2 (F-4b), developer#3 (F-5) | — | — | devops-engineer (F-6 → F-7) | 4 |
| 0.4 | developer#1 (F-4c), developer#2 (F-5t), developer#3 (F-10) | — | — | devops-engineer (F-7), architect#1 (F-8 — стенд, с человеком) | 5 |
| 0.5 | — | — | — | tech-writer (F-9), code-reviewer#1 ×2–3 (долги ревью) | ≤ 4 |
| 1.1 – 1.3 | developer#1, #2 (`mechanics` ∥ `entity`/`state`) | developer#1, #2, #3 (I1a: `shared/agent` ∥ `llm` core ∥ схемы+`laws`) | developer#1 (gateway: store, links, api, actions) | — | 6 |
| 1.4 – 1.6 | developer#1, #2 (`state` objStore/snapshot/recovery ∥ `replay`/bootstrap) | developer#1, #2, #3 (I1a хвост → I1b: рантайм ∥ роли ∥ context) | developer#1 (readmodel/consumer/outbox) | — | 6 |
| 1.7 – 1.8 (после I1-α TEAM-1/TEAM-3) | developer#1 (e2e S1/S3, приёмка) → освобождается | developer#1, #2, #3 (I1b) | developer#1 (gateway хвост), developer#2 (бот на `FakeGateway`) | — | 6 |
| 1.9 – 1.10 | developer#1 (EPIC-002 I2 старт: atomic группы), developer#2 (EPIC-005 ops: `session-report`) | developer#1, #2, #3 (I1b: миграция `legacy`, e2e, golden) | developer#2 (бот: flow/deliver/privacy), developer#1 (EPIC-004 I2 старт: groups) | — | 6 |
| интеграция I1 | tech-lead#1 (слияния), qa-engineer#1, tester#1 | tester#2 (S14) | tester#3 (S9 стенд — с человеком) | devops-engineer, security-engineer | ≤ 6 |
| 2.1 – 2.3 | developer#1 (EPIC-002 I2), developer#2 (EPIC-005 ops) | developer#1, #2 (`group-narrator`, раунд в `encounter`), developer#3 (`MemoryClient`, `blueprint_reloaded`) | developer#1 (rounds coordinator), developer#2 (групповая доставка, admin proxy) | — | 7 → **6**: TEAM-2 developer#3 стартует со сдвигом на одну подволну |
| 2.4 – 2.6 | developer#1, #2 (EPIC-005 memory: Qdrant ∥ Neo4j + HTTP) | developer#1, #2 (S2/S6, удаление legacy) | developer#1 (I2 хвост, `actor_kind` ci/sim) | qa-engineer по командам — отдельным запуском | 5–6 |
| интеграция I2 | tech-lead#1, qa-engineer#1, tester#1 | tester#2 | tester#3 | devops, security, release-manager | ≤ 6 |

Правила заполнения слотов:
- приоритет слотов — команде на критическом пути (TEAM-2); если у TEAM-1/TEAM-3 в волне меньше независимых задач, чем разработчиков, слот отдаётся TEAM-2 в пределах ≤ 3 на команду;
- стендовые задачи (F-8, S9, S1 на живом Ollama, nightly) не занимают слот разработчика — их выполняет человек с одним агентом-помощником (architect#1 или tester#N);
- ревью: `code-reviewer#N` на каждую принятую от разработчика задачу, запуском ≤ 3 одновременно после волны разработчиков; человек — сводка на волну команды и выборочно `shared/*`, `schemas/`, `links`, `filter`, `guardian`;
- блокировка одной команды не останавливает остальные: блокер фиксируется в `state.js` → `blockers` с командой; её слоты временно уходят другим.

## 5. Точки принятия решений

| Точка | Кто | Что решается |
|---|---|---|
| G2 (сейчас) | пользователь | утверждение эпиков, границ, контрактов, состава команд, F-0 (подготовительный коммит и базовая ветка) |
| Конец волны 0 | tech-lead#1 + system-architect | старт волны 1: заглушки v0 в `integration/mvp-1`, конверт `meta` заморожен, решение по профилю `legacy` |
| I1-α | человек (играет) | бот, механика, доставки, `/forget` — на шаблонах; замечания → задачи EPIC-002/004 |
| G3 (объединённый) | пользователь | планы I2 трёх команд; возможное сокращение: 005-memory (Should) |
| I1 | tech-lead#1, qa, security | S1/S3/S9/S14 на интеграционной ветке; готовность к волне 2 |
| G4 | пользователь | MVP-1: S2/S4/S5/S6; слияние `integration/mvp-1` → `main` |
