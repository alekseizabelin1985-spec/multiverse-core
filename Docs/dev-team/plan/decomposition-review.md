# Проверка декомпозиции (предложение `plan/epics.md` v0.1)

Версия 0.1 · 2026-09-09 · tech-lead#1 (TEAM-1, тимлид проекта) · для сводки G2 «Эпики и команды».
Основание: `plan/epics.md`, `plan/ownership.md`, `architecture/contracts.md` (§0, §17), `architecture/components/{foundation,state-and-mechanics,swarm-llm-laws,gateway-and-bot}.md`, `architecture/infrastructure.md` §10–11, `requirements/user-stories.md` v0.3, `requirements/prd.md` §4–§5.13, `journal.md`, `team-process/references/parallel-teams.md`.
Правки в `epics.md`/`ownership.md`/`contracts.md` **не вносились** (их ведёт system-architect на шаге 4); все замечания — в §5 этого файла.

Вердикт: предложение архитектора **принимается с уточнениями**. Границы эпиков совпадают с контекстами `overview.md` §11 и с картой владения; три команды соответствуют решению пользователя. Требуют исправления до старта волны 1: (1) заглушки C-02…C-05 принадлежат эпикам волны 1 и на момент её старта не существуют; (2) `mvctl world init` и `mvctl record` лежат в EPIC-005 I2, хотя нужны для критериев готовности I1; (3) правило «I2 не стартует до S1/S3/S14» простаивает две команды из трёх; (4) список F-задач не учитывает OQ-A-17 (`services/_archive/`), сломанный `git status` и решение по Go 1.26.

---

## 1. Покрытие историй MVP-1 эпиками

Истории MVP-1: US-001…US-019, US-036…US-038 (22). «Осн.» — эпик, отвечающий за приёмку истории; «Вклад» — эпики, без которых критерии не выполняются.

| US | Название (кратко) | Осн. эпик | Вклад | Инкремент | Замечание |
|---|---|---|---|---|---|
| US-001 | Создание персонажа и возврат | EPIC-004 | EPIC-002 (`entity.create.proposed` → факт) | I1 | покрыто |
| US-002 | Соло 30 ходов «Тёмный лес» (S1) | EPIC-003 | EPIC-002, EPIC-004 | I1 | сквозной сценарий — принимается на интеграции I1, не одной командой |
| US-003 | Предсказуемая механика | EPIC-002 | — | I1 | покрыто |
| US-004 | Нарратив асинхронно, деградация | EPIC-003 | EPIC-004 (доставка `template`) | I1 | покрыто |
| US-005 | Мир помнит последствия | EPIC-002 | EPIC-005 (память — Should), EPIC-003 (`journalContext` FR-127) | I1 (Must) / I2 (Should) | покрыто; Must-часть не зависит от памяти |
| US-006 | Групповая сессия из трёх | EPIC-004 | EPIC-002 I2 (atomic группы), EPIC-003 I2 | I2 | покрыто |
| US-007 | Бой по раундам (S2) | EPIC-004 (координатор) | EPIC-003 I2 (раунд в `encounter`, `group-narrator`), EPIC-002 I2 | I2 | покрыто |
| US-008 | Telegram-бот | EPIC-004 | — | I1 | покрыто |
| US-009 | Псевдонимизация, `/forget` | EPIC-004 | — | I1 | покрыто |
| US-010 | `make up`, `/health` | EPIC-001 | EPIC-003 I1 (критерий «глобальный GM и GM региона активны») | 0 → I1 | **частично в EPIC-001**: полный критерий закрывается только после EPIC-003 I1b; в `epics.md` не отражено |
| US-011 | Восстановление снапшот + replay (S3) | EPIC-002 | **EPIC-003** (снапшот роя), **EPIC-004** (снапшот gateway) — C-14 | I1 | **не отражено**: в `epics.md` US-011 только у EPIC-002; в US-списках EPIC-003/004 её нет, хотя `snapshot/writer.go` (gateway) и `swarm/snapshot.go` — их код |
| US-012 | Контроль LLM, деградация, лимиты | EPIC-003 | EPIC-005 (`mvctl llm usage`) | I1 (учёт) / I2 (CLI) | покрыто |
| US-013 | Гигиена секретов | EPIC-001 | — | 0 | покрыто (F-1) |
| US-014 | Переключение провайдера/моделей | EPIC-003 | EPIC-001 (F-8 замер) | I1 | покрыто |
| US-015 | Регион из блупринта без кода (S6) | EPIC-003 | EPIC-005 (`mvctl blueprint validate`, `/v1/trace` Should) | I1 (Must) / I2 (Should) | покрыто |
| US-016 | Точки вставки фильтра, запреты | EPIC-003 (фильтр выхода) | **EPIC-004** (`InputFilter` FR-056 — `actions/inputfilter.go`) | I1 | **не отражено**: FR-056 и US-016 отсутствуют в списке EPIC-004, хотя код — в gateway (`swarm-llm-laws.md` §19) |
| US-017 | Воспроизводимый прогон | EPIC-002 (`replay`) | EPIC-003 (`recorded` провайдер), EPIC-005 (`mvctl record`, записи) | I1 | покрыто, но `mvctl record` нужен уже в I1 (см. §5.1 п. 2) |
| US-018 | LLM-выводы как события | EPIC-003 | — | I1 | покрыто |
| US-019 | Фича-флаг миграции (S5) | EPIC-003 | **EPIC-004** (`GM_PATH` в gateway, `gm.created` при `legacy`), EPIC-001 (профиль `legacy` в compose) | I1 → I2 | **не отражено** у EPIC-004; жизнеспособность профиля `legacy` под вопросом (`infrastructure.md` §11 п. 3) — влияет на критерий S5 |
| US-036 | Мир живёт между сессиями (S14) | EPIC-003 | EPIC-002 (факты фона), EPIC-005 (сводка из памяти — Should) | I1 | покрыто (`journalContext` — штатно) |
| US-037 | Рой GM — иерархия событий | EPIC-003 | — | I1 | покрыто |
| US-038 | Отчёт по сессии | EPIC-004 (события — Must) | EPIC-005 (CLI, CSV — Must/Should) | I1 (события) / I2 (CLI) | покрыто; CSV Must → часть EPIC-005 обязательна (см. §2, EPIC-005) |

Итог: незакрытых историй нет; четыре истории (US-010, US-011, US-016, US-019) имеют вклад эпиков, не указанный в `epics.md` — добавить в US/FR-списки соответствующих эпиков, иначе тимлиды команд не нарежут эти задачи.

Целевые истории US-020…US-035 → EPIC-006…013 по таблице §1 `epics.md`: US-020/021 → 007; US-022/023 → 006; US-024/025 → 008; US-026 → 009; US-027…029 → 010; US-030/031/032/035 → 013 (US-032 «свободный текст» — в EPIC-013 явно не назван, добавить); US-033/034 → 012. Покрытие полное.

---

## 2. Размеры эпиков и оценка в волнах

Шкала `epics.md`: S ≤ 5 · M 6–12 · L 13–25 · XL > 25 задач. Задача = один разработчик, одна сессия, без уточнений (≤ M по размеру задачи). Оценка числа задач — по структуре пакетов в `components/*.md` и чек-листу `infrastructure.md` §10; уточняется тимлидами команд на A4.

| Эпик | Задач (оценка) | Размер | Разработчиков в волне | Волн (последовательных групп) | Обоснование |
|---|---|---|---|---|---|
| EPIC-001 Фундамент | ~14 (F-1, F-2, F-3, F-4a, F-4b×2, F-4c, F-5, F-5t, F-6, F-7, F-8, F-9, + F-10 заглушки v0 — см. §3) | **L** (согласен) | 2–3 dev + devops + security + tech-writer | 5 | F-2 — узкое место (один разработчик, последовательно после F-1/F-3); F-4b (схемы) и F-6 идут параллельно F-4a |
| EPIC-002 Состояние и механика | I1 ~12 (`shared/entity` v2; схемы; `mechanics` rules+formula; rng+resolve+target+changes; invariants; `state` memstore+applier+ownership; objStore+intents; snapshot; recovery; `replay`; bootstrap+admin+health; e2e S1/S3) · I2 ~5 (atomic группы; `Participation`; снапшот по `session.ended`; `state_hash` для `--audit`; e2e group-3x30) | **L** (согласен, ~17) | 2 (`mechanics` ∥ `entity`/`state`) | I1: 5–6 · I2: 2–3 | цепочка `entity → mechanics → state → objStore → recovery` (§12 дизайна) почти линейна; параллелизм — `mechanics` отдельно от `state` |
| EPIC-003 Рой, LLM, страж, законы | I1a ~14 (`shared/agent` types/parser/validator/levels; блупринты + `schemas/agent`; схемы событий ×2 задачи; `llm` gateway/types/config; budget/record/usage; providers ollama+fake+recorded; parser+language; filter; guardian+visibility; prompt; `laws`; FakeNarrator+RecordingWriter+`mvctl record`) · I1b ~14 (registry+scope; router; lifecycle; scheduler+budget; emitter+pipeline; roles global+region; role encounter; role personal+template; context worldview/journal/builder; snapshot; admin+health; миграция I1-0/I1-3 + профиль `legacy`; e2e S1/S14/death/flee/injections/degraded/recovery; golden) · I2 ~7 (`group-narrator`; раунд в `encounter`; `MemoryClient`; `blueprint_reloaded`; S2/S6; удаление legacy; лимиты) | **XL** (~35; согласен) | 3 (максимум по `maxAgentsPerRole`) | I1a: 5 · I1b: 5 · I2: 3 | критический путь MVP-1; даже с тремя разработчиками I1 длиннее I1 других команд в 1,5–1,7 раза |
| EPIC-004 Gateway и бот | I1 ~14 (миграции+store; links+pseudonym+forget; api server/middleware/errors/dto; handlers links/characters/players; actions validate/publish/idempotency/ratelimit/inputfilter; readmodel+bootstrap+waiters; consumer ×4 обработчика; outbox+longpoll+render; session+turns; snapshot; openapi+client; FakeGateway+Harness; бот: updates/sender/commands; flow+render; deliver+privacy) · I2 ~6 (groups; rounds coordinator+store; `rounds/close`; групповая доставка; proxy admin; `actor_kind` ci/sim) | **L** (верх, ~20) | 2 (gateway ∥ бот) | I1: 6–7 · I2: 3 | бот тестируется без gateway через `FakeUpdateSource`/`FakeSender` + интерфейс клиента; e2e бота — после `FakeGateway` |
| EPIC-005 Память и операции | ops ~6 (`session-report` `--audit`/`--json`; `incidents.csv`; golden-набор; `contracts check` наполнение; `llm usage`; пороги NFR) · memory ~6 (Qdrant-адаптер; перенос Neo4j за `integration`; `/v1/context/scope`; `/v1/context/absence`; `/v1/trace`; `memory rebuild`) | **M** (согласен, ~12) | 2 (ops ∥ memory) | 3 + 3 | половина Must (US-038 CSV, golden NFR-065), половина Should (память) — разделить (см. §5.1 п. 6) |

Всего MVP-1: ≈ 95–100 задач. Это верхняя граница того, что один человек-ревьюер пропустит за три инкремента по 2–4 недели (см. §4).

### 2.1. EPIC-003 XL: разбиение

Рекомендация: **не выделять второй эпик**, а внутри EPIC-003 зафиксировать два подинкремента I1 с собственной точкой интеграции:

- **I1a «LLM-шлюз, страж, законы, формат блупринта»** — `shared/agent`, `internal/llm/**`, `internal/laws`, `blueprints/*`, `schemas/agent/*`, схемы событий EPIC-003, `testkit/swarm` (FakeNarrator, RecordingWriter), `mvctl record`, `mvctl blueprint validate` (валидатор). Готовность: `mvctl blueprint validate blueprints/` зелёный на пяти блупринтах; `llm.Gateway.Generate` на `fake`/`recorded`/живом Ollama проходит парсер → язык → фильтр (a) → страж; `llm.output` записывается; первая запись `solo-30.jsonl` снята на стенде.
- **I1b «Рой: рантайм, роли, тики, миграция»** — `internal/swarm/**`, admin-порт `core`, e2e S1/S14, профиль `legacy`.

Почему не отдельный эпик: `internal/llm` и `internal/swarm` связаны через `llm.Call/Result` и двухфазный конвейер (§8 дизайна), общий владелец (`architect#2`), одна ветка и один тимлид; отдельный эпик потребовал бы контракт `llm.Gateway` в `contracts.md`, ещё одну ветку и интеграцию, и перенумерацию EPIC-004…013. Почему всё же делить на I1a/I1b: (1) I1a не зависит от EPIC-002 вообще (только C-01 и `FakeProvider`), I1b — зависит от C-02/C-03 (заглушки); (2) I1a даёт TEAM-3 настоящий `FakeNarrator` и записи для e2e бота раньше, чем готов рой; (3) для ревьюера партия I1a (~14 задач) отделена от партии I1b и может быть слита в интеграционную ветку отдельно; (4) три разработчика TEAM-2 в I1a работают по независимым пакетам (`llm/providers+parser`, `llm/filter+guardian+prompt`, `shared/agent+laws+blueprints`) без пересечений в файлах.

Порядок внутри TEAM-2 при трёх разработчиках: волна 1.1 — `shared/agent` (dev#1) ∥ `llm` core+providers (dev#2) ∥ схемы событий + `laws` (dev#3); 1.2 — блупринты+`schemas/agent` ∥ parser+language+filter ∥ guardian+prompt; 1.3 — record/budget/usage ∥ FakeNarrator+RecordingWriter+`mvctl record` ∥ registry+scope+router (старт I1b); далее I1b тремя потоками: рантайм (router/lifecycle/scheduler/emitter) ∥ роли (global/region, encounter, personal) ∥ context+snapshot+admin; последняя волна I1b — миграция `legacy` и e2e.

---

## 3. Критический путь, зависимости, достаточность заглушек

### 3.1. Критический путь

`F-1 → F-2 → F-4a → F-5t (contract-тест шины) → [волна 1] EPIC-003 I1a → I1b → интеграция I1 (002 → 003 → 004; S1/S3/S14) → EPIC-003 I2 → интеграция I2 (S2/S5/S6) → G4`.

Оценка длительности при полной загрузке слотов: волна 0 — 1,5–2 нед.; волна 1 — 4–5 нед. (ограничена EPIC-003, не 3–4 из `epics.md`); волна 2 — 3–4 нед. Итого MVP-1 ≈ 9–11 нед. Не на критическом пути: F-8 (замер), EPIC-005 memory, EPIC-004 I1 (запас ≈ 1,5 нед.), EPIC-002 I1 (запас ≈ 1 нед.). Запас TEAM-1/TEAM-3 стоит тратить на ранний старт I2 и на EPIC-005 ops (см. §5.1 п. 4).

### 3.2. Волны (уточнённые)

| Волна | Содержание | Условие входа | Условие выхода |
|---|---|---|---|
| 0 | EPIC-001 (TEAM-1): F-1, F-3 → F-2 → F-4a ∥ F-4b ∥ F-5 ∥ F-6 → F-4c ∥ F-5t ∥ **F-10 заглушки v0** ∥ F-7 ∥ F-8 → F-9 | G2 утверждён; подготовительный коммит (см. §5.3 п. 0) | `make ci` зелёный; `/health` пустых контекстов; contract-тест шины; `testkit` содержит membus + v0 заглушек C-02…C-05; `baseline.md` |
| 1 | EPIC-002 I1 ∥ EPIC-003 I1a→I1b ∥ EPIC-004 I1 | выход волны 0; объединённый G3 по планам трёх команд | S1, S3, S8, S9, S10, S14 на интеграционной ветке; промежуточная точка **I1-α** (см. §4) |
| 2 | EPIC-002 I2 → EPIC-005 (ops, затем memory) ∥ EPIC-003 I2 ∥ EPIC-004 I2 | своя I1 принята тимлидом команды (не ждать интеграции всех трёх — см. §5.1 п. 4) | S2, S4, S5, S6; G4 MVP-1 |

### 3.3. Достаточность заглушек для старта TEAM-2/TEAM-3 без EPIC-002

Проверка `contracts.md` §17 против владения `ownership.md`:

| Потребитель | Нужно на старте волны 1 | Заглушка по §17 | Владелец заглушки | Есть на старте волны 1? |
|---|---|---|---|---|
| все | C-01 шина, реестр, схемы `_common` | `testkit/membus`, `shared/contracts` | EPIC-001 | да (F-4a/F-5t) |
| TEAM-2 (I1b), TEAM-3 | C-02 факты State | `testkit.FakeState` | **EPIC-002** (`shared/testkit/state`) | **нет** — EPIC-002 стартует одновременно |
| TEAM-2 (роль `encounter`) | C-03 типы `mechanics.*` (компиляция) + `FixedMechanics` | `testkit.FixedMechanics` + `rules/dark-forest.yaml` | **EPIC-002** | **нет**; без пакета `internal/mechanics` с типами `Actor/Action/Outcome/Roll` роль встречи не компилируется |
| TEAM-2, TEAM-1 | C-04 генератор `player.*` | `testkit.Harness` | **EPIC-004** | **нет** |
| TEAM-3 | C-05 нарратив/механика | `testkit.FakeNarrator` | **EPIC-003** | **нет** (появится в I1a) |
| TEAM-3 | `shared/entity` (типизированные геттеры для read-model) | — (не заглушка, общий код) | **EPIC-002 переписывает** в I1 | as-is `shared/entity` переносится в F-2, но EPIC-002 меняет его структуру в I1 → TEAM-3 либо пишет против as-is и переделывает, либо ждёт |
| TEAM-3 | C-14 `latest.json` (формат указателя) | — | EPIC-002 (§4.4 дизайна) | формат описан; нужна фикстура |
| TEAM-2 | C-09 память | `journalContext` (штатно) | EPIC-003 сам | да |
| telegram-bot | C-08 | `testkit.FakeGateway` | EPIC-004 (та же команда) | внутри команды — норм |

Вывод: семантически заглушки достаточны, но **четыре из них принадлежат эпикам волны 1 и на её старте не существуют**, а `shared/entity` меняется в волне 1 под ногами у TEAM-3. Две меры (рекомендую первую):

1. **F-10 «Заглушки контрактов v0 и модель сущности v2» в волне 0** (размер M, developer + architect#1, который проектировал и EPIC-001, и EPIC-002): `shared/entity` по §3 `state-and-mechanics.md` (структура, ops, hash, attrs — спецификация полная); `internal/mechanics` — только типы C-03 + `Load` + `rules/dark-forest.yaml` (без `Resolve`); `testkit/state.FakeState` v0 (применяет ops в память, публикует факты, без инвариантов); `testkit/mechanics.FixedMechanics`; `testkit/gateway.Harness` v0 (только генератор `player.*` из фикстур в membus, без HTTP); `testkit/swarm.FakeNarrator` (шаблон на `combat.decided`/`round.closed`/`player.looked`); фикстуры `testdata/fixtures/*` и `snapshots/state/latest.json`. После волны 0 владение подпакетами переходит поставщикам по `ownership.md` (v0 → реальная реализация в их ветках). Стоимость: +3–4 дня волны 0; выигрыш: три команды стартуют одновременно без «первой недели на заглушки» и без гонки за `shared/entity`.
2. Альтернатива (если волну 0 не удлинять): правило «заглушка — первая задача поставщика, сливается в `integration/mvp-1` сразу после приёмки тимлидом команды, не дожидаясь интеграции эпика»; TEAM-3 первые 3–5 дней делает бот и миграции SQLite (не зависят от `shared/entity`), TEAM-2 — I1a. Риск: ранние слияния в интеграционную ветку из трёх веток вне порядка 002 → 003 → 004.

Дополнительно: `contracts.OwnershipRules` (уровни агентов для `level_violation`) — «список уровней передаёт EPIC-003» (C-02), а использует `internal/state` (EPIC-002) с первой волны. Для MVP-1 таблица статична (§4.6 `state-and-mechanics.md`) — положить её в `shared/contracts` в F-4b, чтобы обе команды читали одно место.

---

## 4. Риски поставки при одном человеке-ревьюере и реакции

| # | Риск | Реакция |
|---|---|---|
| 1 | **Объём ревью**: ~100 задач за ~10 недель; при 6 параллельных разработчиках человек получает партию из 6 результатов на каждую волну | ревью в два слоя: `code-reviewer#N` — на каждую задачу (обязателен, другой экземпляр, чем разработчик); человек — одну сводку на волну команды (что слито, что изменилось в `shared/*`, что в контрактах, где риски) и выборочно код в `shared/*`, `schemas/`, `internal/gateway/links`, `llm/filter`, `llm/guardian` (безопасность и контракты). Размер задачи — не больше M; задача, чей дифф > ~600 строк без тестов, дробится тимлидом команды |
| 2 | **Стенд один и ручной**: F-8 (замер моделей), S9 (живой Telegram), S1 на живом Ollama, nightly-gpu выполняет только человек | все стендовые проверки — отдельные задачи с явной пометкой «стенд», планируются на конец волны, не блокируют CI-часть; CI работает на записях (`recorded` провайдер) и `membus`; F-8 не блокирует код (модели параметризованы в блупринтах) |
| 3 | **Порядок интеграции 002 → 003 → 004** и конфликты в общем коде | при F-10 заглушки v0 уже в интеграционной ветке — слияния поставщиков заменяют v0 на реализацию в своих подпакетах `testkit/{state,swarm,gateway}`, конфликты локализованы; `shared/{eventbus,contracts,entity}` после волны 0 меняются только через system-architect (пометка `contract-change`); тимлид проекта сливает лично, после каждого слияния — e2e `solo-30` на оставшихся заглушках |
| 4 | **Первая играбельная точка слишком поздно** (после всего I1 = 5–6 нед. от старта волны 1) | ввести промежуточную точку **I1-α «соло на шаблонах»**: EPIC-002 I1 + EPIC-004 I1 + `FakeNarrator` (из F-10/I1a) → бой с волком через Telegram с `generated_by=template`, без роя и LLM. Достижима на ~3-й неделе волны 1; даёт человеку реальную игру и раннюю проверку бота, механики, доставок, `/forget`. Затем I1 (с роем и LLM) — S1/S9 полностью |
| 5 | **Правило «I2 не начинается до S1/S3/S14 в I1»** простаивает TEAM-1/TEAM-3 на 1,5–2 нед., пока TEAM-2 заканчивает I1b | смягчить: старт I2 в команде — по приёмке своей I1 тимлидом команды; условие интеграционного прогона I1 — для *слияния* I2 в `integration/mvp-1`, не для старта. Освободившиеся слоты TEAM-1 идут в EPIC-005 ops (`session-report`, golden — нужны для приёмки I1 и I2) |
| 6 | **Три команды виртуальные, разработчик — один**: дефекты интеграции возвращаются «в команду», но чинит всё тот же человек-владелец решений | дефект интеграции = задача в бэклоге эпика-владельца по карте владения, с явным `T-NNN` и ревью; не «быстрая правка в интеграционной ветке» |
| 7 | **Записи LLM для CI** делаются на стенде человеком; при изменении промптов записи устаревают | `mvctl record` и `RecordingWriter` в I1a (EPIC-003), не в EPIC-005; ключ записи `(correlation_id, agent.id, phase, attempt)` не зависит от текста промпта; обновление записей — осознанная задача с ревью диффа (ADR-010) |
| 8 | **Профиль `legacy` может не работать** без Chroma/as-is semantic-memory (`infrastructure.md` §11 п. 3) → критерий S5 (`legacy_gm_path_share`) не проверяем | решение архитектора до волны 1: либо `legacy` = as-is `narrative-orchestrator` + `semantic-memory` + `chromadb` в одном профиле (Chroma переезжает в `legacy`, не удаляется), либо S5 сводится к «флаг удалён, `gm_path=agent` в 100 % `llm.output`» без сравнения нарративов. Второе дешевле и не блокирует; рекомендую его как запасной вариант |
| 9 | **Замер F-8 меняет конфигурацию моделей** (OQ-A-18: одна `30b-a3b` или `8b + 14b`) после старта TEAM-2 | модель — параметр блупринта; I1a пишется против `fake`/`recorded`; живой прогон — конец I1b. Не блокирует |

---

## 5. Замечания

### 5.1. К `plan/epics.md` (вносит system-architect / оркестратор)

1. **`mvctl world init` → EPIC-002 I1.** Сейчас в EPIC-005 I2, но S1/S9 на стенде и e2e требуют инициализированного мира уже в I1. Логика — `internal/state/bootstrap.go` из `testdata/fixtures` (EPIC-002); NPC из `npc_table` блупринта (`swarm.InitWorld`, `swarm-llm-laws.md` §17) — дополнение EPIC-003 I1b. Два дизайна описывают инициализацию по-разному (фикстуры vs блупринты) — архитектору зафиксировать одно: рекомендую фикстуры как источник в MVP-1, `npc_table` — для респауна.
2. **`mvctl record` + `RecordingWriter` → EPIC-003 I1a.** Критерий готовности EPIC-003 I1 «S1 на записях в CI» невыполним, если запись — EPIC-005 I2. Golden-набор и `session-report` остаются в EPIC-005.
3. **US/FR-списки:** EPIC-004 добавить US-011 (снапшот gateway, C-14), US-016/FR-056 (`InputFilter` noop), US-019/FR-014 (`GM_PATH` в gateway); EPIC-003 добавить US-011 (снапшот роя); EPIC-001 у US-010 пометить «полный критерий — после EPIC-003 I1b». EPIC-013 — добавить US-032.
4. **Правило старта I2** («ни одна задача I2 не начинается до S1/S3/S14») заменить на: старт I2 в команде — по приёмке своей I1; слияние I2 в интеграционную ветку — после интеграционного прогона I1.
5. **EPIC-003:** зафиксировать I1a/I1b (§2.1) и точку интеграции I1-α (§4 п. 4). Волна 1 для TEAM-2 — 4–5 нед., не 3–4.
6. **EPIC-005 разделить на две части внутри TEAM-1** с раздельной приёмкой: **005-ops** (Must: `session-report` `--audit`/`--json`, `incidents.csv`, golden NFR-065, `contracts check` наполнение, `llm usage`, пороги NFR) и **005-memory** (Should: Qdrant, Neo4j, `/v1/context/*`, `/v1/trace`, `memory rebuild`). Тогда при перерасходе волны 2 память отрезается на G3/G4 чисто, без потери Must-историй (US-038 CSV, US-005 Must-часть работает на `journalContext`).
7. **Волна 2 — ветки:** вместо новых `epic/EPIC-00N-group` вести одну ветку на эпик на весь MVP-1 (`epic/EPIC-002-state` и т. д.), после интеграции I1 подтягивать `integration/mvp-1` в ветку эпика. Меньше веток, проще история; инкремент отмечается тегом на интеграционной ветке (`mvp-1/i1`, `mvp-1/i2`).
8. **§4 таблица команд:** слоты волны 1 не 2+2+2, а асимметрично: TEAM-2 — 3, TEAM-1 — 2, TEAM-3 — 1 в первых волнах (бот стартует после `FakeGateway` или на интерфейсе клиента), затем 2/3/1 → 2/2/2 по мере освобождения. Детали — `plan/teams.md` §4.
9. **§1 EPIC-001 «Не входит»:** «удаление ban-of-world/reality-monitor — по OQ-A-17» → решено: перенос в `services/_archive/` (ничего не удаляется). Перенести формулировку в «Входит».
10. **§6 F-4:** зафиксировать разбиение F-4a/b/c из `foundation.md` §12 прямо в таблице; добавить F-10 (§3.3) и F-0 (§5.3).
11. **Готовность EPIC-001:** добавить «`git status` чист и работает; `pre-commit` с `gitleaks` установлен; `services/_archive/` вне `go build ./...`».

### 5.2. К `plan/ownership.md`

1. **`shared/testkit/**`:** уточнить трёхступенчатое владение: ядро (`membus`, `dedup`, `containers.go`, `versions`, contract-тест шины) — EPIC-001 → после волны 0 tech-lead#1; подпакеты `state/`, `mechanics/`, `swarm/`, `gateway/` — v0 создаёт EPIC-001 (F-10), с волны 1 владелец — поставщик контракта (EPIC-002/003/004), который заменяет v0 реализацией. Правило: потребитель заглушки не правит её — запрос владельцу.
2. **`shared/entity/**`:** если принята F-10 — v2 пишется в волне 0 (EPIC-001 + architect#1), с волны 1 владелец EPIC-002; TEAM-3/TEAM-2 — только чтение. Если F-10 не принята — пометить «меняется в EPIC-002 I1, волна 1.1; до слияния другим командам использовать только `Ref`/константы типов».
3. **`shared/runtime`, `shared/clock`** (запрос `state-and-mechanics.md` §14 п. 2) — EPIC-001 → tech-lead#1; добавить строкой.
4. **`cmd/mvctl/**`:** сейчас целиком EPIC-005, но подкоманды приходят из трёх эпиков (`world init` — 002, `blueprint validate`/`laws bump|show`/`record` — 003, `session-report`/`llm usage`/`memory rebuild` — 005, `contracts check`/`env check` — 001). Предложение: `cmd/mvctl/main.go` и реестр подкоманд — tech-lead#1; `cmd/mvctl/internal/<cmd>/` — владелец подкоманды по списку выше.
5. **`services/_archive/**`** (OQ-A-17): новая строка — «архив заменённого кода; вне `go build`/compose/CI; владелец tech-lead#1; изменения запрещены; возврат кода — через эпик-владелец с ADR». Строки про `services/ban-of-world`, `reality-monitor`, `shared/{schema,redis,tinyml,intent,rules,spatial,config,minio,oracle}` → «переносятся в `services/_archive/` в F-1/F-3». Для `services/{game-service,entity-manager,rule-engine,semantic-memory}` («удаляются/переносятся владельцами новых пакетов») — уточнить: после переписывания тоже в `services/_archive/`, не удаление (то же решение пользователя). Замороженные 8 сервисов остаются на месте с `FROZEN.md` (они не заменяются, а ждут своих эпиков) — подтвердить у архитектора.
6. **`snapshots-{world}/gateway/`** → EPIC-004 (запрос `gateway-and-bot.md` §14 п. 5); `analytics.replay.completed` — совладение EPIC-002 (`mode=recovery`) / EPIC-005 (`mode=test`) одним файлом схемы.
7. **`Docs/dev-team/epics/EPIC-NNN/**`** — тимлид команды `tech-lead#N` (`tasks.md`, `dev-log.md`, `review.md`, `test-*.md`), `design.md` — `architect#N`; `integration-report.md` — tech-lead#1.
8. **`testdata/recordings/**`** — сейчас EPIC-005; записи снимает EPIC-003 (`mvctl record`, `RecordingWriter`) → владелец EPIC-003, EPIC-005 добавляет `testdata/analytics/**` и golden.

### 5.3. К списку F-1…F-9 (порядок и недостающее)

Проверено по репозиторию 2026-09-09:

- `git status` **не работает**: `fatal: not a git repository: D:/my project/Go project/multiverse-core/.git/worktrees/laughing-kare` — в индексе 13 записей `.claude/worktrees/*`, каталоги содержат `.git`-указатели на несуществующий путь; в `.git/worktrees/` — устаревшие записи. Это блокер любых git-операций команд (подтверждает `infrastructure.md` §11 п. 11).
- В индексе: `examples.exe`, `fake_deps/*`, `mcp_audit.log`, `mcp_kafka.log` (и по журналу — `.mcp.env`).
- `shared/oracle/README.md` строки 14, 29, 68 — реальный ключ `sk-…` (решение пользователя: считать скомпрометированным, заменить плейсхолдером).
- `.gitleaks.toml`, `.pre-commit-config.yaml` отсутствуют; есть `.gitattributes`, `.gitignore`.
- Версии Go: `go.work` 1.24.11, сервисы 1.24.0, локальный toolchain go1.25.3; ADR-001 — 1.25; `infrastructure.md` §11 п. 1 предлагает 1.26.
- `Docs/dev-team/**` не в индексе (0 файлов отслеживается) — артефакты G1/G2 не зафиксированы.

Предлагаемый порядок и дополнения:

| # | Задача | Изменение относительно `epics.md` §6 | Размер | Исполнитель |
|---|---|---|---|---|
| **F-0** (новая, до волны 0) | Подготовительный коммит (по подтверждению пользователя, `git.commits=ask`): `git worktree prune` + удаление `.claude/worktrees/*` из индекса и `.gitignore` для `.claude/worktrees/`; фиксация `Docs/dev-team/**`; создание `integration/mvp-1` и `epic/EPIC-001-foundation` | отсутствовала; без неё `git status` и ветки не работают | S | tech-lead#1 + пользователь |
| **F-1** | Гигиена: **`gitleaks` и `pre-commit` — первым шагом** (до любых других правок в индексе), затем `git rm --cached` секретов, бинарников, логов; **плейсхолдер вместо ключа в `shared/oracle/README.md`** (`ORACLE_API_KEY=<your-key>`); `.gitignore`, `.gitattributes` (`*.jsonl -diff`), `.env.example`, `.mcp.env.example`; проверка `gitleaks git --redact` = 0 по рабочему дереву | добавлены: порядок (gitleaks первым), README-плейсхолдер | S | developer + security-engineer |
| **F-3** | **`services/_archive/` вместо удаления** (OQ-A-17): `git mv` `services/{ban-of-world,reality-monitor}`, `shared/{schema,redis,config,minio,oracle,rules,intent,tinyml,spatial}`, `fake_deps/`, `test_minio.go` → `services/_archive/<путь>`, каждый со своим `go.mod` (вне сборки), `services/_archive/README.md` с индексом и причиной; `FROZEN.md` в 8 замороженных сервисах (остаются на месте); `Docs/archive/` | «удаление» → «архив»; `fake_deps`/`test_minio.go` — тоже в архив (код), в отличие от бинарников и логов | S→M | developer + tech-writer |
| **F-2** | Единый модуль: **`go 1.26` + `toolchain go1.26.x`** (решение архитектора по `infrastructure.md` §11 п. 1; тимлид — за, стоимость нулевая), `depguard`/`forbidigo`, `cmd/multiverse` каркас, `shared/runtime`, `shared/clock`; `services/_archive/**` и замороженные — вне `./...` | Go 1.26; явно `_archive` вне сборки | M | developer |
| F-4a / F-4b / F-4c | без изменений (`foundation.md` §12); в F-4b — `contracts.OwnershipRules` статичная таблица MVP-1 | разбиение a/b/c в таблицу | L | developer×2 |
| F-5 / F-5t | без изменений; `build/versions.env` — единый источник пинов для compose и testcontainers (`infrastructure.md` §11 п. 13) | + versions.env | M | developer |
| **F-10** (новая) | Заглушки контрактов v0 и `shared/entity` v2 (§3.3 п. 1): `entity` v2, типы C-03 + `rules/dark-forest.yaml`, `FakeState` v0, `FixedMechanics`, `Harness` v0 (генератор), `FakeNarrator`, фикстуры мира и `latest.json` | отсутствовала | M | developer + architect#1 |
| F-6 / F-7 | без изменений; F-6 — решение по профилю `legacy` (§4 п. 8) и профилю `bot` (`infrastructure.md` §11 п. 10) до старта волны 1 | — | M / M | devops-engineer |
| F-8 | без изменений; не блокирует волну 1 | — | M | architect#1 + человек (стенд) |
| F-9 | без изменений; добавить `services/_archive/README.md` и статус «архив» в таблицу сервисов | — | S | tech-writer |

Порядок: F-0 → F-1 ∥ F-3 → F-2 → (F-4a ∥ F-4b ∥ F-5 ∥ F-6) → (F-4c ∥ F-5t ∥ F-10 ∥ F-7 ∥ F-8) → F-9. Слоты по подволнам — `plan/teams.md` §4.

---

## 6. Допущения этой проверки

- Число задач — оценка по структуре пакетов дизайнов; тимлиды команд уточняют на A4 (после G2), отклонение ±20 % не меняет размеров эпиков.
- «Ничего не удалять» (OQ-A-17) относится к коду; бинарники, логи, секреты и указатели worktree удаляются из индекса (не из истории — `git filter-repo` только по явной команде пользователя).
- Лимит `maxAgentsPerRole=3` трактуется как «не более трёх экземпляров роли в одной команде одновременно», `maxParallelAgents=6` — суммарно по всем командам в одном запуске оркестратора (разработчики и ревьюеры — разные запуски).
