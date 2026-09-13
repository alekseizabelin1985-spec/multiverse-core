# EPIC-003 «Рой GM, LLM-шлюз, страж, законы» — задачи и волны

Версия 0.3.2 · 2026-09-13 · tech-lead#2 (TEAM-2) · статус: в работе (перезапуск трёх команд; подволна A).

**Действующее на 2026-09-13 (ревизия 4, tech-lead#2).** Этот абзац отменяет устаревшие места ниже; они не стёрты, а помечены «(ревизия 4)» там, где меняют смысл.
- **Команда и ветки (gitflow).** Эпик ведёт TEAM-2: tech-lead#2, architect#2, developer#1…#3, code-reviewer (указание пользователя 2026-09-13 «три команды», `parallelism.maxTeams = 3`, `maxAgentsPerRole = 3`). Ветка эпика — `epic/EPIC-003-swarm-llm-laws` от `develop` (`1c2ee7e`), рабочая папка `.worktrees/EPIC-003`. Задача — ветка `task/T-NNN-<slug>` от ветки эпика в своей папке `.worktrees/T-NNN`. После «принято» tech-lead#2 оркестратор сливает ветку задачи в ветку эпика. `integration/mvp-1` не используется.
- **Поставки другим командам** (прежний «ранний merge в `integration/mvp-1`») — **контрольное слияние** ветки эпика в `develop` с сохранением ветки. Разрешение пользователя постоянное (журнал 2026-09-13). Условия: задачи приняты, `make ci` зелёный, `make secrets-scan` чистый. Без тегов, push и `main`, каждое слияние — запись в журнал. Другие эпики забирают поставку из `develop` своим `sync`.
- **Чужие пути** (`cmd/**`, `.golangci.yml`, `shared/{eventbus,contracts,clock,runtime}`, общие документы) — не «PR через tech-lead#1». `shared/{eventbus,contracts,clock,runtime}` и `.golangci.yml` меняет только EPIC-001: задачей с меткой `contract-change` через system-architect#1. Общие документы (`plan/**`, `contracts.md`, `ownership.md`, `components/**`) правятся в одном месте, в `develop`. Файлы регистрации контекстов и подкоманд — файлы владельцев после T-446 (EPIC-001): EPIC-003 правит свой файл в ветке задачи.
- **`tasks.md` эпика правит только tech-lead#2.** Исполнитель ведёт карточку `tasks/T-NNN.md`, `dev-log.md` и `review.md` в ветке задачи.
- **Номера задач** — из общего счётчика `counters.task` в `.dev-team.json` (сейчас 446), их выдаёт оркестратор. Диапазоны команд T-200…T-299 и т. п. больше не действуют.
- **Основание ревизии 4**: `contracts.md` v0.10 (C-01 v1.7), решения system-architect#1 по вопросам сверок и ревью (журнал 2026-09-13; тексты контрактов — T-444, линтер, реестр и схемы — T-445, раскладка регистрации — T-446, все в EPIC-001), сверка плана tech-lead#2 с деревом `1c2ee7e`.
**Правки сведения 3 внесены tech-lead#1 от имени tech-lead#2** (одна команда по решению пользователя; основание — `architecture/consolidation.md` §14, `contracts.md` v0.4, ADR-017 «Дополнение 1»): T-202, T-212, T-215, T-217, T-230, T-237, T-246; ссылки «C-05 v1.2» → «C-05 (contracts v0.3+)»; §9 риск 9 и §10 п. 1/2/3/6/7/8/9 — статусы обновлены. Структура подволн, состав и число задач не менялись.
**Правки ревизии контрактов T-416 внесены tech-lead#1 (2026-09-11; один проход по всем эпикам по решению оркестратора)** — основание `contracts.md` v0.7 (C-01 v1.4, C-05 v1.4, C-14 v1.2, §16 п. 6), ADR-025 (подтверждён), ADR-026, ADR-027: T-202 (таблица владения одна — строка gateway, копия и тест равенства сняты), T-229, T-230, T-232, T-233, T-236, T-237; §9 риск 9, §10 п. 3. Зависимости на новые задачи T-417 (EPIC-001, C-01 v1.4 в коде) и T-419 (двойники по C-05 v1.4). Добавленные пункты помечены «(T-416, 2026-09-11)», отменённые — «заменено (T-416)» и не удалены.
**Нарезка T-230 и сверка встреч при восстановлении — tech-lead#2 (2026-09-11; решение оркестратора после T-416).** T-230 R7 разрезана на четыре задачи ≤ M: T-230 R7a (решение обмена и пакет), **T-421** R7b (судьба пакета), **T-422** R7c (жизненный цикл после факта и откладывание), **T-423** R7d (закрытие по чужому факту). Сверка встреч с State после догона вынесена из T-237 в **T-424** R10d. Подволны 1.10–1.16 пересобраны: прежняя таблица ставила T-236 в одну подволну с его зависимостью T-225, а T-237 — раньше его зависимости T-228; I1b заканчивается в 1.16. Прежняя редакция T-230 сохранена в её разделе, у каждого пункта названа задача, которой он передан; пункт T-237 зачёркнут со ссылкой. **Правки T-425** (contracts v0.8: C-05 v1.5, C-01 v1.5; «Уточнения исполнения» ADR-026 и ADR-027) внесены тем же проходом в T-230, T-421…T-424, T-229, T-232, T-237, T-246, каждая с пометкой «(T-425, 2026-09-11)». Очередь действий на время создания встречи держит агент встречи (T-422). Окно до его подъёма — ~~открытый вопрос architect#2~~ **решено architect#2 (2026-09-11): агента поднимает решение GM региона открыть встречу, внутри процесса, до публикации предложения — design §14.2, ADR-028.**
~~Ветка эпика: `epic/EPIC-003-swarm` от `integration/mvp-1`. Диапазон номеров команды: **T-200…T-299** (TEAM-1 — T-001…T-199, TEAM-3 — T-300…T-399). Использовано: T-201…T-256 (разработка) + T-260…T-266 (стенд). Свободны: T-257…T-259, T-267…T-299. Из общего счётчика проекта (вне диапазона, по решению оркестратора): T-419, **T-421…T-424**.~~ **(ревизия 4)** Ветка — `epic/EPIC-003-swarm-llm-laws` от `develop`. Номера — только из общего счётчика: в эпике T-201…T-256, T-260…T-266 (стенд), T-419, T-421…T-424, T-427, T-439 (передана из EPIC-001), T-452 (из ревью T-053 EPIC-002, §12).
Основание: `epics/EPIC-003-swarm-llm-laws/design.md` **v0.2** (единицы A1–A7 / B1–B3a/B3b–B6 / C1–C5 / R1–R13 / G1–G7, точка I1-α, §13 решения), `architecture/components/swarm-llm-laws.md` v0.2 (**КД**), `architecture/contracts.md` **v0.3** (§0, C-05 (contracts v0.3+) / C-06 / C-07 / C-11 / C-12 / **C-15 v1.1**, §17), `architecture/consolidation.md` §10–§13 (**сведение 2**), `architecture/adr/ADR-005` «Дополнение 2» п. 1–9, ADR-001 доп. п. 7–8, `architecture/overview.md` §18.1, `plan/epics.md` v0.3, `plan/teams.md` §1/§4, `plan/ownership.md` v0.3, `plan/decomposition-review.md` §2.1/§3, `requirements/user-stories.md` v0.4, `requirements/nfr.md` v0.3, ADR-002/005/008/010/014…017, `journal.md` (решения G2; «Сведение 2 завершено», «EPIC-003 · tech-lead#2»).

### Changelog

| Версия | Дата | Изменения |
|---|---|---|
| 0.1 | 2026-09-09 | Первая нарезка: T-201…T-253 + стенд T-260…T-266, 19 подволн. |
| **0.2** | **2026-09-09** | **Сведение 2, внесено tech-lead#2** (`consolidation.md` §13): **T-208 B3 → B3a `openai_compat` (llama-server)** — провайдер по умолчанию, критический путь; **новая T-254 · B3b `ollama` native** — второй, реализуется только если F-8 выберет C/A (плановое место — подволна 2.1); **новая T-255** — хук `cmd/multiverse/fake_contexts.go` (`MV_SWARM_FAKE`, совладение с EPIC-001, PR через tech-lead#1); **новая T-256** — удаление хука (критерий I1); T-206 B1 — `Params{TopP,TopK,MinP,PresencePenalty}`, `Response.Tokens.Cached`, `ReasoningLen`, `MV_LLM_URL`/`MV_LLM_API_KEY`, гейт облака по host; T-203/T-260 — стартовая модель **E `Qwen3.8-27B-UD-Q3_K_XL`, `thinking: false`**, критерий «по `ops/metrics/baseline.md`»; T-215 — `validation_status` по решению **сведения 3**; T-219/T-220 — ссылка на **C-05 (contracts v0.3+)** (`FakeEncounter` — единственная заглушка боя) и пометка «ранний merge `shared/testkit/swarm`»; стенд T-260/T-261/T-263/T-265 — замер и живые прогоны **на llama-server** (`scripts/llm-bench.ps1`, `scripts/llm-server.ps1`), Ollama — только при выборе C/A; §6 волны, §7 чек-листы, §8 сводка и оценка пересчитаны (53 → 56 задач). |
| **0.2.2** | **2026-09-11** | **Нарезка T-230, tech-lead#2**: T-230 R7 → T-230 R7a + **T-421** R7b + **T-422** R7c + **T-423** R7d; сверка встреч при восстановлении — из T-237 в **T-424** R10d (T-237 больше не зависит от T-230); зависимости T-242, T-246 — на все нужные части; §6 подволны 1.10–1.16 пересобраны по зависимостям (T-231/T-232/T-233 → developer#3, T-236 → 1.11, T-237 → 1.14, T-238/T-239/T-242 → 1.15, T-243…T-245/T-256 → 1.16); §7, §8 пересчитаны (56 → 60 задач). |
| **0.2.3** | **2026-09-11** | **Бэклог из T-415 (EPIC-001), tech-lead#2**: строки DoD в T-223 (след отказа публикации в `Emitter`, узкое исключение остановки), T-229 (нарратив на остановке — не `Error`), T-421 (след отказа переживает повтор); §10 п. 14 — вопрос architect#2 об окне действий встречи (`Seen` или `Has`/`Add`); новый §11 — бэклог двойников `shared/testkit/swarm` (2 пункта). Число задач и подволны не менялись. |
| **0.2.4** | **2026-09-11** | **Решения architect#2** (design v0.3 §14, ADR-028). §10 п. 14 решён: окно действий агента встречи — `Has`/`Add`, id из причины у всех событий ответа, ответ собирается один раз, постоянный отказ шины → `dead_letters`. T-427 решена: агента встречи поднимает решение GM региона открыть встречу, внутри процесса и до публикации предложения. DoD изменены у T-223, T-225, T-229, T-230, T-232, T-236, T-246, T-421, T-422, у T-424 уточнён текст; все правки помечены «(architect#2, 2026-09-11)». Число задач и подволны не менялись. |
| **0.2.5** | **2026-09-11** | **Ревизия контрактов 3 (T-431, EPIC-001), внесено system-architect#1 по решению оркестратора; строка журнала — tech-lead#1 при приёмке T-431** (основание — `contracts.md` v0.9: C-01 v1.6 «посредник доставки», C-05 v1.6; design v0.3a §14.5; ADR-027 «Уточнение исполнения 2», ADR-028 «Уточнение исполнения»). Добавлены строки DoD:<br>• T-224 — двухшаговое окно Router, идемпотентность проекций;<br>• T-227 — `budget.Observe` по id события, тик закрывается один раз;<br>• T-228 — ошибка роли возвращается шине; счётчик «3 ошибки» считает события, а не доставки;<br>• T-232 — открытие встречи собирается один раз, id из события открытия и scope;<br>• T-236 — «открытие в полёте», id окна бюджета, последний закрытый `tick_seq`.<br>У T-421 уточнён тест 2, у T-224 и T-228 — «Что». Все правки помечены «(T-431, 2026-09-11)». Число задач и подволны не менялись. |
| **0.3** | **2026-09-13** | **Перезапуск трёх команд, ревизия 4 — tech-lead#2.** Основание: сверка плана с деревом `1c2ee7e` и `contracts.md` v0.10, решения system-architect#1 (1a–1f, 2, 3, 4, 7), gitflow. Шапка «Действующее на 2026-09-13»; §1 общий DoD (`forbidigo` — четыре семейства, `membus` — реализация C-01, статус `done`); `integration/mvp-1`, «ранний merge» и «PR через tech-lead#1» заменены контрольным слиянием и правилами владения трёх веток. Исправлены DoD: T-201, T-202, T-203, T-204, T-205, T-206, T-207, T-208, T-209, T-210, T-211, T-212, T-213, T-216, T-217, T-218, T-221, T-226, T-228, T-233, T-235, T-237, T-238, T-254, T-256. Статусы: T-214 и T-215 — `review`, T-206 — `in-progress`, T-219, T-220, T-255, T-419 — `done`, T-427 — `review`. T-439 заведена разделом (передана из EPIC-001). §6 подволны пересобраны: A…N вместо 1.1–1.16. Все правки помечены «(ревизия 4)». Объём задач не менялся: новая задача подключения контекстов предложена оркестратору, номера у неё нет. |
| **0.3.1** | **2026-09-13** | **T-452, tech-lead#2.** Новый §12 «Задачи из ревью других эпиков»: в него вынесена T-452, которая стояла внутри §11 «Бэклог двойников». T-452 добавлена в перечень номеров эпика. Прежнее «Волна 1» заменено на «вне подволн §6.1, до T-062 EPIC-002». Статус — `done`. Подволны §6.1 не менялись. |
| **0.3.2** | **2026-09-13** | **Приёмка T-214 и T-215, tech-lead#2** (по дереву `fd52e31`, после слияния T-445 и T-444 в эпик). Обе задачи — `done`. Замечания ревью #1, не закрытые T-444/T-445, разнесены так: Mi-4 — строка DoD T-230, N-1 — строка DoD T-217, п. 5 бэклога ревью — строка DoD T-211, остальное — новый §13 «Бэклог схем событий». Подволны §6.1 и объём задач не менялись. |

**Правила нарезки.** Задача ≤ M, один пакет-владелец, выполнима одним разработчиком за одну сессию без уточнений. Unit-тесты — внутри задачи (ADR-010). Integration / e2e / golden / документация — отдельные задачи. Крупные единицы дизайна (A1 частично, B6, C4, R2, R4, R10, R13) разбиты. Задачи-поставки другим командам идут первыми. ~~и сливаются в `integration/mvp-1` до готовности эпика (пометка **ранний merge**)~~ **(ревизия 4)** Поставка уходит контрольным слиянием эпика в `develop` после приёмки (пометка **поставка**; прежняя пометка «ранний merge» читается так же).

---

## 1. Общий DoD (входит в каждую задачу, в карточках — только специфика)

- [ ] Unit-тесты написаны и зелёные: `go test -short ./<пакет>/...`; в пакетах с горутинами — `goleak`; покрытие `internal/{swarm,llm}` не снижается, цель ≥ 60 % (NFR-064).
- [ ] Линтер и сборка чисты: `go build ./... && go vet ./... && golangci-lint run`. Границы `depguard` не нарушены. `forbidigo` запрещает четыре семейства: `os.Getenv|LookupEnv|Environ|ExpandEnv` (→ `shared/env`), `time.Now` (→ `shared/clock.Clock`), `time.After|Tick|NewTimer|NewTicker|Since|Until` (→ `shared/clock.Timers`), `log.Print*|Fatal*|Panic*` (→ `log/slog`). *(ревизия 4; было: «`forbidigo` — без `fmt.Print*`» — такого правила в `.golangci.yml` нет)*
- [ ] Новые/изменённые схемы событий лежат в `schemas/events/<type>.v1.json` (JSON Schema 2020-12, `$ref` на `_common.json`) и **зарегистрированы** в `shared/contracts/registry.go`; `mvctl contracts check` зелёный. **(ревизия 4)** `registry.go` общий: до T-446 (EPIC-001) правка реестра — задачей EPIC-001 через system-architect#1, после T-446 — в файле владельца EPIC-003.
- [ ] Переменная окружения объявлена через `shared/env.Declare` с префиксом `MV_` (КД §14) **в `shared/env/vars.go` тем же изменением, которое вводит её чтение, и не раньше** (C-14 уточнение v0.4, T-409). Уже объявленную переменную не объявлять заново. `mvctl env check` зелёный. Строка в `.env.example` — в той же ветке задачи (NFR-074). *(ревизия 4; было: «правка `.env.example` запрошена у tech-lead#1»)*
- [ ] Изменены только пути владения TEAM-2 (`plan/ownership.md`); правка чужого пути или контракта — не сделана, а описана в отчёте как запрос.
- [ ] Заглушки-потребляемые (`FakeState`, `FixedMechanics`, `Harness`) **не правились**: при нехватке функциональности — запрос владельцу через tech-lead#2. `membus` — не заглушка, а вторая реализация C-01 (`shared/eventbus/membus`, владелец EPIC-001), её правка — только через system-architect#1. *(ревизия 4)*
- [ ] Карточка `tasks/T-NNN.md` («Выполнение») и `dev-log.md` эпика заполнены в ветке задачи: что сделано, решения по ходу, отклонения от дизайна и почему, какие тесты; подпись `developer#K`.
- [ ] Поведение соответствует критериям приёмки указанных US и разделам КД, на которые ссылается задача.
- [ ] Секретов и ПДн в коде/фикстурах нет (`make secrets-scan`, `privacy-scan` по `testdata/` зелёные).

Статусы: `todo | in-progress | review | done | blocked` *(ревизия 4; было `accepted` — в `state.js` и карточках статус принятой задачи `done`)*. Задача стартует в `todo`. `done` ставится после приёмки, когда в карточке заполнены «Выполнение», «Ревью», «Приёмка» и «Учёт времени».

---

## 2. Инкремент I1a «LLM-шлюз, страж, законы, формат блупринта» (23 задачи, из них T-254 — условная)

Зависимости вовне: только EPIC-001 (C-01 `membus`/`Journal`/`Dedup`, `shared/contracts`, `clock`, `runtime`, `env`, `objstore`) и заглушки F-10.

### Поток A — `shared/agent`, блупринты, CLI, законы (developer#1)

#### T-201 · A1 · `shared/agent` v2: типы блупринта, парсер, плейсхолдеры, чистка пакета
Инкремент I1a · подволна **A** *(ревизия 4; было 1.1)* · developer#1 · Размер M · Статус **done** *(принята tech-lead#2 2026-09-13, итераций ревью 3; подробности — `tasks/T-201.md`)* · ветка `task/T-201-agent-blueprint-v2`
Что сделать: `types.go` (перенос `AgentLevel`/`LODLevel`/`AgentLifecycleState` из `agent_types.go`), `blueprint.go` — структура `AgentBlueprint` v2 по КД §13.1 со всеми вложенными типами; `parser.go` — `ParseFile/ParseBytes`: YAML-frontmatter + секции `## system|phase1|phase2|tick|canon|description`, чистый YAML для `.yaml/.yml`, `yaml.v3` с `KnownFields(true)`, миграция плоских `llm.model/llm.temperature` → `llm.phase2` с `Issue{Severity: warning}`, `ContentHash` (SHA-256 нормализованного содержимого, независим от порядка ключей) **в виде `sha256:<64 hex>`** *(ревизия 4, решение 3 system-architect#1; шаблон в схемах — T-445)*; `placeholders.go` — фиксированный словарь КД §11.4. ~~Удалить **из пакета** (не из истории): `md_parser.go`, `blueprint_loader.go`, `helpers.go`, `worker_pool.go`, `state_manager.go`, `filter.go`, `router.go`, `lifecycle.go`, `pipeline.go`, устаревшие части `interfaces.go`, `examples/`, `agent_test.go`, `e2e_dark_forest_test.go`~~ **(ревизия 4, список по дереву `1c2ee7e`)** Удалить **из пакета** (не из истории): `agent_types.go` (после переноса типов), `md_parser.go`, `blueprint_loader.go`, `blueprint_validator.go` и `blueprint_validator_test.go` (валидатор пишется заново в T-202), `helpers.go`, `worker_pool.go`, `state_manager.go`, `router.go`, `lifecycle.go`, `pipeline.go`, устаревшие части `interfaces.go`, `examples/`, `agent_test.go`. Файлов `filter.go` и `e2e_dark_forest_test.go` в дереве нет. `README.md`/`MIGRATION.md` → `Docs/archive/` (уведомить tech-writer). Сохранить `lod.go`, `tools/registry.go`.
**(ревизия 4) `time.Now` в сохраняемых файлах.** `lod.go:115` и `tools/registry.go:134, 165, 180` зовут `time.Now`/`time.Since`. Сейчас их прикрывает исключение `.golangci.yml` для пути `shared/agent/` (`forbidigo`, `errcheck`, `staticcheck`, `unused`), а оно обещано снять вместе с переписыванием пакета. Файл `.golangci.yml` — EPIC-001, поэтому **до старта задачи нужно решение tech-lead#1 об исключении**. Варианты: (а) T-201 переводит оба файла на `shared/clock` и исключение снимается задачей EPIC-001; (б) исключение сужается до двух файлов с комментарием «до `internal/swarm` (T-225)». Без решения исполнитель исключение не трогает и пишет расхождение в отчёт.
Файлы: `shared/agent/{types,blueprint,parser,placeholders}.go`, `shared/agent/testdata/blueprints/valid/*.md`.
Зависит от: F-2, F-4b (внешние, волна 0); решение tech-lead#1 об исключении `shared/agent/` в `.golangci.yml` *(ревизия 4)*.
Ссылки: US-015, FR-090, FR-124, C-11, ADR-015 п. 6, design §3.1 A1, КД §13.1, §2, §17 I1-2.
DoD (специфика):
- [x] `ParseFile` разбирает все файлы `testdata/blueprints/valid/` и файл в формате «чистый YAML»; секции доступны как `Prompts.{System,Phase1,Phase2,Tick,Canon,Description}`.
- [x] Неизвестный ключ frontmatter → ошибка с именем поля и файлом; плоский `llm.model` → предупреждение о миграции, разбор не падает.
- [x] `ContentHash` стабилен на 100 повторных разборах и меняется при изменении любого байта содержимого. **(ревизия 4)** Значение соответствует `^sha256:[0-9a-f]{64}$`; тест на форму.
- [x] `go build ./...` зелёный после удаления файлов; ни один пакет вне `services/_archive/**` не ссылается на удалённые символы.
- [x] **(ревизия 4)** `golangci-lint run ./shared/agent/...` зелёный с исключением `.golangci.yml` в той форме, которую выбрал tech-lead#1. В отчёте названо, что осталось под исключением и почему.

**Приёмка (tech-lead#2, 2026-09-13): принято.** Прогоны зелёные: `go build/vet`, `go test -short`, `golangci-lint run ./...` (0 issues), `mvctl contracts check`, `make test`. Исключение `shared/agent/` в `.golangci.yml` снято целиком по варианту (а) и решению ОВ-7. **Нужна отметка tech-lead#1** — файл EPIC-001, что проверить — в карточке. Nit ревью #3 N-3/N-4 закрыты тестами при приёмке, мутанты M16/M9/M10 убиты. Слияние с кончиком эпика без конфликтов кода; `dev-log.md`/`review.md` — через драйвер `appendtail`.

**Бэклог T-201 (приёмка T-201, 2026-09-13; без номера — номер выдаёт оркестратор при взятии в работу):**
1. **`shared/agent/tools/registry.go` и `shared/agent/lod.go`** — до первого импорта `LODManager`/`ToolRegistry` из `internal/swarm`. Размер S.
   - Гонка данных: `checkRateLimit` пишет под `RLock`, `updateStats`/`GetStats` работают без блокировки.
   - Ключи `lodDistribution` не совпадают с `LODLevel.String()`; `lastHighLOD` пишется при любой смене LOD.
   - DoD: тест на совпадение ключей; `go test -race ./shared/agent/...` в CI (Linux, cgo) зелёный на параллельном тесте `Execute`/`GetStats`.
2. **Документы об as-is `shared/agent`** (architect#2 — архитектура, tech-writer — остальное; `swarm-llm-laws.md` правит только его владелец):
   - `analysis/api-contracts.md` §3.1: `llm.tick` без `temperature`/`thinking`;
   - КД `components/swarm-llm-laws.md`: §2 («`lod.go`/`tools/registry.go` без изменений», шапка «ни одного целевого файла»);
   - `diagrams/c4-component-swarm-llm-laws.md:45`;
   - ADR-015 п. 6;
   - `PULL_REQUEST.md:21-29` (ссылки на перенесённые `md_parser.go`, `examples/`, `README.md`, `MIGRATION.md`).
3. **Реестр блупринтов (T-222, горячая перезагрузка T-249):**
   - решение о перевалидации — по `content_hash`, с тестом «файл без ключа → `[]` вызывает перевалидацию»;
   - лимит `MaxBlueprintSize` — тот же, что у `ParseFile`.

#### T-202 · A2 · Реестр уровней `levels.go` и валидатор блупринтов
Инкремент I1a · подволна **B** *(ревизия 4; было 1.2)* · developer#1 · Размер M · Статус todo · ветка `task/T-202-levels-validator`
**(ревизия 4)** В перечне полей `ValidationEnv` ниже не хватало `OwnedEntityTypes`, хотя DoD на него опирается. Полный состав: `ValidationEnv{EventTypes, OwnedEntityTypes, Blueprints, FileExists, Tools, Schemas, Invariants, Models}`. `OwnedEntityTypes` — функция или карта `(level, role) → []entity_type`, её собирает вызывающий из `contracts.OwnershipRules`. `EnvFromProject` принимает её параметром: `shared/agent` не импортирует `shared/contracts`.
Что сделать: `levels.go` — реестр уровней роя: `AllowedEventTypes(level, role)` по `data-model.md` §4. **Таблицы владения `levels.go` не держит (T-416, 2026-09-11; ADR-025, C-02 v1.4).** Вид «какие типы сущностей у уровня» (`OwnedEntityTypes`) валидатор получает из окружения `ValidationEnv`. Окружение собирает вызывающий (`mvctl blueprint validate`, T-204; рантайм роя) из `contracts.OwnershipRules`, а `shared/agent` не импортирует `shared/contracts` (ADR-015 п. 3). *(Заменено (T-416): «`OwnedEntityTypes(level, role)` … (источник для `contracts.OwnershipRules`)».)* `validator.go` — `Validate(bp, env) []Issue{File, Field, Reason, Severity}`, правила 1–14 + 7а КД §13.2, glob по сегментам (`*` не пересекает точку); `ValidationEnv{EventTypes, Blueprints, FileExists, Tools, Schemas, Invariants, Models}` и `EnvFromProject(root, types, invariants, models)`; корпус `testdata/blueprints/invalid/` — по файлу на правило.
**Заменено (T-416, 2026-09-11; ADR-025 подтверждён, C-02 v1.4, `contracts.md` §16 п. 6)** — процедура сведения 3 ниже отменена. Единственная истина таблицы владения — `shared/contracts/ownership.go`. Строка gateway там уже есть (T-006), в `levels.go` она не вносится: строкам без агента не место в реестре уровней роя. Копии и теста равенства нет. Если блупринтам EPIC-003 нужна правка строки уровня, её делает ~~отдельный PR в `shared/contracts`~~ **задача EPIC-001** *(ревизия 4: `shared/contracts` меняет только EPIC-001)* с меткой `contract-change` и ревью system-architect и tech-lead#1; tech-lead#2 в запросе перечисляет затронутые блупринты («Подтверждение» ADR-025), тест таблицы покрывает строки всех трёх групп. Прежний текст — для истории:
~~**Дополнение (сведение 3, `contracts.md` v0.4 §16 п. 6, C-02 v1.2; TL2-3, З-2; внесено tech-lead#1 от имени tech-lead#2)**: в `levels.go` вносится **строка gateway** — `Character.status: alive → abandoned` (только этот переход) и `Group.leader_id` (включая значение `null`); процедура закреплена: `shared/agent/levels.go` — **истина**, `shared/contracts.OwnershipRules` — статичная копия, и она правится **тем же PR** (метка `contract-change`, ревью tech-lead#1 + system-architect, приоритет блокера). Обратный порядок (правка копии без истины) запрещён; State (EPIC-002) — только потребитель. Замечание §10 п. 3 закрыто.~~
Файлы: `shared/agent/{levels,validator}.go`, `shared/agent/testdata/blueprints/{valid,invalid}/*.md`. *(Заменено (T-416): «`shared/contracts/ownership.go` (копия — тем же PR, …)» — T-202 файл таблицы владения не правит.)*
Зависит от: T-201, T-006 (EPIC-001 F-4b-1 — `contracts.OwnershipRules`, единственная истина по ADR-025).
Ссылки: US-015, US-037, FR-090…092, FR-061, BR-16, **`contracts.md` v0.7 C-02 v1.4, §16 п. 6, ADR-025 (с «Подтверждением»)**, ADR-015 п. 3, C-11 v1.1, SEC-21, `consolidation.md` §14.1 (TL2-3, З-2 — история), design §3.1 A2, КД §13.2.
DoD (специфика):
- [ ] По одному невалидному файлу на каждое правило 1–14; тест таблицей «файл → ожидаемые `Field`/`Reason`/`Severity`».
- [ ] ~~Тест равенства `levels.AllowedEventTypes/OwnedEntityTypes` ↔ `contracts.OwnershipRules` (расхождение = падение теста, а не тихая правка чужой таблицы); тест запускается в job `contracts` и **блокирует merge**.~~ **Заменено (T-416): таблица одна (ADR-025), сравнивать не с чем.**
- [ ] ~~**(сведение 3)** строка gateway присутствует в обеих таблицах: `Character.status → abandoned` и `Group.leader_id` (в т. ч. `null`); PR помечен `contract-change` и содержит обе правки.~~ **Заменено (T-416): строка gateway живёт только в `ownership.go` (T-006), в `levels.go` её нет.**
- [ ] **(T-416, 2026-09-11; ADR-025, C-02 v1.4)** Валидатор проверяет `owned_entity_types` блупринта по `ValidationEnv.OwnedEntityTypes`, который собирает вызывающий из `contracts.OwnershipRules`. Тест: тип вне строки своего уровня → `error` с `Field`/`Reason`. `shared/agent` не импортирует `shared/contracts`; `go list -deps ./shared/agent/...` — в тесте или в правиле depguard.
- [ ] Правило 7а: `env.Models == nil` → `info: models not checked`; модель вне списка → `error`; поле `provider` в блупринте → `error` (`KnownFields`).
- [ ] `monitor`/`object` — `info: reserved level, spawn disabled`, не `error`.
- [ ] **(приёмка T-201, 2026-09-13; ревью #1 T-201 Mi-3, ревью #3 T-201 бэклог п. 2)** Правило 13 использует и `UnknownPlaceholders`, и `SuspiciousPlaceholders`.
  - Неизвестный плейсхолдер точной формы — `error`.
  - Подозрительная запись (`{Player.name}`, `{ x }`, `{игрок}`) — `warning`. Ложные срабатывания известны: `\frac{A}{B}`, `\text{Привет}`, `{日本}`.
  - Тест таблицей на обе серьёзности.
- [ ] **(приёмка T-201, 2026-09-13; ревью #3 T-201 бэклог п. 2; C-02 v1.6, T-448)** `warning` на значения в `ops[].value` и `conditions[].value`, которые State отвергнет только при применении:
  - число за |x| ≤ 2^53−1;
  - `time.Time`: незакавыченная дата YAML, в JSON уйдёт строкой.

  Тест на оба случая и на граничное значение 2^53−1 без предупреждения.

#### T-203 · A3 · Пять блупринтов MVP-1 и схемы `schemas/agent/`
Инкремент I1a · подволна **C** *(ревизия 4; было 1.3)* · developer#1 · Размер M · Статус todo · ветка `task/T-203-mvp-blueprints` · **стартует после T-439**
**(ревизия 4) Семплинг в блупринте.** Формат фазы блупринта — `{model, temperature, max_tokens, thinking, schema_ref}` (`api-contracts.md` §3, КД §13.1). Разбор идёт с `KnownFields(true)`, поэтому `top_p`, `top_k`, `min_p`, `presence_penalty` в YAML блупринта — ошибка разбора, а не настройка. В блупринте задаются `temperature: 0.7` (non-thinking-профиль, ADR-005 доп. 2 п. 1) и `max_tokens` из T-439. Остальные значения профиля — `0.8 / 20 / 0 / 1.5` — умолчания фазы в шлюзе (T-206 `Params`, T-208). Пример КД §13.3 и `api-contracts.md` §3 с `temperature: 0.8` — старый; правка передана владельцам (отчёт tech-lead#2). `max_tokens` фаз и `maxLength`/`maxItems` в `schemas/agent/narrative.json` — по T-439: до T-438 значения базовой E (160), с комментарием-указателем на `baseline.md` §5.
Что сделать: `blueprints/{global-dark-forest-world,domain-dark-forest,encounter-wolf,player-gm,group-narrator}.md` по КД §13.3 (**модели — стартовая конфигурация E (сведение 2, U-8, ADR-005 доп. 2 п. 5): `model: Qwen3.8-27B-UD-Q3_K_XL`, `thinking: false` на все фазы**, семплинг фазы — non-thinking-профиль `temperature 0.7 / top_p 0.8 / top_k 20 / min_p 0 / presence_penalty 1.5`, с комментарием-указателем `# model: per ops/metrics/baseline.md (OQ-A-18; базовая E, запасные по decision_order ADR-005 доп. 3: Qwen3.6-35B-A3B, C, A)` *(T-440)*; `round{60s, 2}` в `encounter-wolf`; `budget.background_calls_per_hour_world: 4` в global; `respawn_ttl: 24h`, `npc_table`, `background_events[]`, `encounter{detect_on: tick, chance}` в domain); `schemas/agent/{narrative,tick-global,tick-region}.json` по КД §13.4 + `breach.json` (заглушка E-B).
Файлы: `blueprints/*.md`, `schemas/agent/*.json`.
Зависит от: T-201, T-202, T-214 (реестр типов для правила 4/8; схемы уже в дереве, задача в `review`), **T-439** (потолок `max_tokens` и `maxLength` нарратива; architect#2) *(ревизия 4)*.
Ссылки: US-015, US-002, FR-090, FR-120, ADR-015, **ADR-005 доп. 2 п. 5, доп. 3**, `overview.md` §18.1 (матрица E/E+/C/A), design §3.1 A3 / §11 п. 3, КД §13.3–13.4.
DoD (специфика):
- [ ] `agent.Validate` на всех пяти файлах — 0 `error`; `mvctl blueprint validate blueprints/` включается в CI задачей T-204.
- [ ] Модель во всех пяти блупринтах — **стартовая E** (`Qwen3.8-27B-UD-Q3_K_XL`), `thinking: false`; поля провайдера в блупринте нет (SEC-21). Смена модели — **только правкой YAML по `ops/metrics/baseline.md`** (T-260), Go не меняется; критерий «модели соответствуют `baseline.md`» проверяется на стенде, не в CI (в CI — `fake`/`recorded`).
- [ ] Схемы компилируются `santhosh-tekuri/jsonschema/v6` (draft 2020-12); `enum` типов в `tick-*.json` — полный набор MVP-1, сужение по `allowed_event_types` блупринта делается в рантайме (решение тимлида, см. §10 п. 7).
- [ ] `blueprints/domain-dark-forest.md` заменяет `shared/agent/examples/domain-dark-forest.md` (старый файл перенесён в `services/_archive/shared/agent/examples/` в T-201 — *приёмка T-201, было «уже удалён»*).
- [ ] Плейсхолдеры секций — только из словаря `placeholders.go`.
- [ ] **(ревизия 4)** В фазах блупринтов только поля формата `{model, temperature, max_tokens, thinking, schema_ref}`; `temperature: 0.7`; `max_tokens` фазы нарратива и `text.maxLength`/`maxItems` в `schemas/agent/narrative.json` равны значениям T-439 для текущей конфигурации. Тест: разбор каждого блупринта с `KnownFields(true)` без ошибок; `maxLength` схемы совпадает с числом, записанным в T-439.

#### T-204 · A4 · `mvctl blueprint validate` и документация формата блупринта
Инкремент I1a · подволна **G** *(ревизия 4; было 1.4)* · developer#3 *(ревизия 4; было developer#1 — поток A занят рантаймом)* · Размер S · Статус todo · ветка `task/T-204-mvctl-blueprint-validate`
Что сделать: подкоманда `cmd/mvctl/internal/blueprint` (~~регистрация в реестре `mvctl` — PR к каркасу F-4c, ревью tech-lead#1~~ **(ревизия 4)** регистрация в реестре `mvctl` — правкой в ветке задачи. Имя `blueprint` уже зарезервировано в `cmd/mvctl/main.go`. После T-446 регистрация живёт в файле владельца EPIC-003, до T-446 — одна строка в `commands()` с ревью tech-lead#2. Каркас `cmd/mvctl/internal/cli` не меняется): `blueprint validate [--offline] <dir>`, вывод «файл, поле, причина, severity», код возврата ≠ 0 при `error`; **документация**: `blueprints/README.md` (формат v2, секции, словарь плейсхолдеров, как добавить регион — вход для S6) и `shared/agent/README.md` (API пакета, правила валидатора).
Файлы: `cmd/mvctl/internal/blueprint/*.go`, `blueprints/README.md`, `shared/agent/README.md`.
Зависит от: T-202, T-203.
Ссылки: US-015, FR-090, NFR-083, C-11, design §3.1 A4, КД §13.2.
DoD (специфика):
- [ ] `mvctl blueprint validate blueprints/` = 0; на `shared/agent/testdata/blueprints/invalid/` = 1 с перечнем проблем.
- [ ] `--offline` пропускает правило 7а с `info`.
- [ ] **(ревизия 4)** Вызывающий собирает `ValidationEnv.OwnedEntityTypes` из `contracts.OwnershipRules` (T-202); тест подкоманды на блупринте с чужим типом сущности → код 1.
- [ ] Запрос devops на job `contracts` («вызывать `mvctl blueprint validate blueprints/`») оформлен в отчёте.
- [ ] `blueprints/README.md` содержит пошаговую инструкцию «второй регион без Go» (проверяется задачей T-241).

#### T-205 · A7 · `internal/laws`, `laws/dark-forest-world.v1.yaml`, `mvctl laws bump|show`
Инкремент I1a · подволна **B** *(ревизия 4; было 1.5)* · developer#3 *(ревизия 4; было developer#1)* · Размер M · Статус todo · ветка `task/T-205-laws`
Что сделать: `document.go` (`LawsVersion`, `Law{ID, Kind, Text, Check, Source}`, enum статусов `approved|pending_review|rolled_back|superseded` — только типы, машина состояний в EPIC-007); `laws.go` — `Service{Current, Get, Watch, Strain, Bump}`, подписка `world.laws.changed`; `source.go` — `FileSource` (реализация) + `ObjectSource` (интерфейс); `strain.go` — накопительный счётчик; файл `laws/dark-forest-world.v1.yaml` (~~`inv-1…inv-11`~~ **`inv-01…inv-10`** — те же id, что `internal/mechanics/invariants.go`, плюс `inv-11` `laws_version_current`; `law-1/law-2`); подкоманды `cmd/mvctl/internal/laws` (`bump --world … --from <file>`, `show --world …`); env `MV_LAWS_DIR` (объявляется этой задачей в `shared/env/vars.go`: сейчас её в манифесте нет), ~~`MV_LAWS_BREACH_PHASE_ENABLED`~~ **`MV_LAWS_BREACH_PHASE`** (уже объявлена в `shared/env/vars.go` как `env.LawsBreachPhase`; заново не объявлять).
**(ревизия 4) Уточнения по дереву и решениям system-architect#1.**
- **Граница `depguard`.** Правило `internal-laws` разрешает `internal/laws` видеть только себя, а `WorldView` живёт в `internal/swarm`. Поэтому `Current(world)` читает версию мира через **интерфейс потребителя `laws.WorldVersions`** (решение 1f), объявленный в `internal/laws`; имена методов — по тексту T-444, до него — «версия законов мира и признак, что сущность мира есть». Реализует его рой поверх `WorldView` (T-234). В тестах законов — фейк.
- **Публикации `Bump`.** `world.laws.changed` по схеме в дереве: `laws{version_from, version_to, status}`, `created_by: author`, `diff{added, removed}`, **`effective_from{round_boundary: true}`** — все четыре поля обязательны. `entity.update.proposed{world.laws_version}` — с **`cause: author`** и `actor_kind=system`. Значения `cause=laws` в enum схемы нет, а строка `system` в `contracts.OwnershipRules` разрешает причины `init` и `author`.
- **Регистрация.** Подкоманды `laws` — в реестре `mvctl` по правилу T-204 (имя уже зарезервировано). Контекст `laws` в `cmd/multiverse` подключает отдельная задача (предложена оркестратору, см. §6).
Файлы: `internal/laws/*.go`, `laws/dark-forest-world.v1.yaml`, `cmd/mvctl/internal/laws/*.go`, строка `MV_LAWS_DIR` в `shared/env/vars.go` и `.env.example`.
Зависит от: T-214 (схема `world.laws.changed`; схема уже в дереве, задача в `review` — старту не мешает).
Ссылки: US-018, NFR-020/021/026, FR-040/042/045, BR-02, C-12, ADR-008, design §3.1 A7, КД §12.
DoD (специфика):
- [ ] ~~`Current(world)` берёт версию из `WorldView.World.laws_version`~~ **(ревизия 4)** `Current(world)` берёт версию через `laws.WorldVersions` (интерфейс потребителя в `internal/laws`, решение 1f), при отсутствии сущности мира — старшая `approved` из файлов. Тест на фейке `WorldVersions`; `go list -deps ./internal/laws/...` не содержит `internal/swarm` (правило `internal-laws`).
- [ ] `Bump` валидирует `vN+1` (`based_on`, `status`, `created_by`, известные `check`-ключи) и публикует в `membus` `world.laws.changed` ~~(+`diff{added,removed}`)~~ **(ревизия 4)** со всеми обязательными полями схемы — `laws{version_from, version_to, status}`, `created_by: author`, `diff{added, removed}`, `effective_from{round_boundary: true}` — и `entity.update.proposed{world.laws_version}` с `actor_kind=system`, ~~`cause=laws`~~ **`cause=author`**. Оба события проходят валидацию схем на `membus`.
- [ ] **(ревизия 4)** Id инвариантов в `laws/dark-forest-world.v1.yaml` — `inv-01…inv-11`; тест в `internal/laws` на этот список. Сверку с `internal/mechanics.Invariants()` пакет законов сделать не может: `internal-laws` не видит `mechanics`. Её делает интеграционный тест роя T-238, которому видны оба пакета.
- [ ] Неизвестный `check` при загрузке → ошибка загрузки версии и признак `unknown_check` для `/health` (потребляется T-239).
- [ ] `Strain().Inc(lawID)`/`Snapshot()` покрыты тестом; `mvctl laws show` печатает `Current` + strain.

### Поток B — LLM-шлюз (developer#2, критический путь)

#### T-206 · B1 · Типы шлюза, конфигурация, реестр провайдеров, таблица цен
Инкремент I1a · подволна **A** *(ревизия 4; было 1.1)* · TEAM-2/developer#2 · Размер M · Статус **in-progress** · ветка `task/T-206-llm-gateway-types`, папка `.worktrees/T-206`. Карточку `tasks/T-206.md` ведёт исполнитель в ветке задачи.
**(ревизия 4) Поправки к тексту ниже — по дереву и `contracts.md` v0.10.**
- **`MV_LLM_URL` обязательна и без значения по умолчанию.** Решение T-404, ревью #2 M-4; C-15 в v0.10: адрес — только `MV_LLM_URL`, хвост `/v1` отбрасывается. Прежнее «по умолчанию `http://127.0.0.1:1234`» ниже отменено.
- **Манифест.** Уже объявлены в `shared/env/vars.go`: `MV_LLM_PROVIDER`, `MV_LLM_URL`, `MV_LLM_API_KEY`, `MV_LLM_NUM_CTX`, `MV_LLM_STORE_PROMPTS`, `MV_LLM_CLOUD_ENABLED`, `MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS`, `MV_LLM_CLOUD_BUDGET_USD_PER_DAY`, `MV_ANTHROPIC_API_KEY`, `MV_OLLAMA_URL`. `config.go` читает их через эти объявления (`env.LLMURL` и т. д.) и заново их не объявляет. В `vars.go` задача добавляет **только те переменные, которые впервые начинает читать сама** (таймауты фаз и т. п.), с типом (`IsInt`, `IsBool`, длительность). Переменную, которую прочтёт более поздняя задача (`MV_LLM_LATIN_MAX_RATIO` — T-209, `MV_LLM_TURN_CALLS_PER_MIN` — T-250), объявляет она (C-14 уточнение v0.4). Строки `.env.example` — в той же ветке.
- **Имя провайдера в событиях.** `config.cloud_enabled.provider` и `llm.output.provider` в схемах ограничены шаблоном `^[a-z][a-z0-9_]{0,31}$` (решение 2). Имена реестра этому шаблону соответствуют.
Что сделать: `types.go` — `Provider{Generate, Embed, Health, Models}` (**C-15 v1.1**), `Request/Response/Params/Phase`, `Call/Result/Rejection`, `ValidationStatus`, ошибки `ErrUnavailable/ErrBudget/ErrQuarantined/ErrInvalidAfterRetries/ErrFilter`. **Поля по сведению 2 (C-15 v1.1, ADR-005 доп. 2):** `Params{Temperature, TopP, TopK, MinP, PresencePenalty, MaxTokens, Think}` (`Think=false` — значение по умолчанию всех фаз MVP-1), `Response.Tokens{Prompt, Completion, Cached}` (`Cached` — `usage.prompt_tokens_details.cached_tokens`, 0 при отсутствии), `Response.ReasoningLen` (длина отброшенного `reasoning_content`; сам текст не хранится), `Status ∈ ok|loading|degraded(model_not_resident)|unavailable`. `config.go` — все `MV_LLM_*` через `env.Declare` (КД §14), добавлены `MV_LLM_URL` (~~по умолчанию `http://127.0.0.1:1234`~~ обязательна, без умолчания) и `MV_LLM_API_KEY` — *ревизия 4: обе уже объявлены в `vars.go`*; `MV_LLM_PROVIDER` по умолчанию **`openai_compat`**, допустимые значения `openai_compat|ollama|anthropic|recorded|fake`; `MV_OLLAMA_URL` объявляется, но читается только при `MV_LLM_PROVIDER=ollama`; таймауты фаз, `MV_LLM_CLOUD_*`, `MV_LLM_STORE_PROMPTS`; **гейт облака по host `MV_LLM_URL`** — общая функция `IsLocalEndpoint(url)` (loopback, `host.docker.internal`, RFC1918) для всех провайдеров; загрузка `config/llm-prices.yaml` по ключу `(endpoint_host, model)`; `providers/registry.go` — `Register(name, factory)`, `New(name, cfg)`.
Файлы: `internal/llm/{types,config}.go`, `internal/llm/providers/registry.go`, `config/llm-prices.yaml`.
Зависит от: —
Ссылки: US-014, US-012, FR-070, NFR-046/076, **C-15 v1.1**, ADR-005 (+ **доп. 2 п. 1–3**), design §3.1 B1, КД §9.1, §14.
DoD (специфика):
- [ ] Пакет компилируется без единой реализации провайдера; `Registry.New("unknown")` → внятная ошибка; имена `openai`/`deepseek` как значения `MV_LLM_PROVIDER` **не поддерживаются** (это только URL/ключ) — тест на понятную ошибку.
- [ ] `mvctl env check` видит все переменные блока `internal/llm` с дефолтами из КД §14, включая `MV_LLM_URL`/`MV_LLM_API_KEY`; ~~правка `.env.example` запрошена у tech-lead#1/devops~~ **(ревизия 4)** строки `.env.example` для впервые читаемых переменных — в ветке задачи. `MV_LLM_URL` без умолчания: пустое значение → ошибка конфигурации с именем переменной (тест). В `vars.go` нет объявления без читателя.
- [ ] `llm-prices.yaml`: цены за 1k токенов по `(endpoint_host, model)`, локальные адреса = 0; тест загрузки и подсчёта `cost_usd`.
- [ ] Тест гейта облака: не-локальный host `MV_LLM_URL` без `MV_LLM_CLOUD_ENABLED=true` → провайдер не стартует (ошибка конфигурации); локальные адреса стартуют без флага. Ключ и его фрагменты не логируются.
- [ ] Нулевые значения новых полей `Params`/`Response` не ломают вызывающий код (совместимость v1 → v1.1).

#### T-207 · B2 · Провайдеры `fake` и `recorded` — **поставка: контрольное слияние эпика в `develop`** *(ревизия 4; было «ранний merge в `integration/mvp-1`»)*
Инкремент I1a · подволна **B** *(ревизия 4; было 1.2)* · developer#2 · Размер M · Статус todo · ветка `task/T-207-fake-recorded-providers`
**(ревизия 4) Кто может импортировать пакеты.** `depguard` пускает `internal/llm/providers/*` только в `internal/llm` и `internal/memory`. Для `internal/memory` T-445 сужает доступ до `providers$` (реестр, ADR-001 доп. п. 7), `fake$` — только в тестах. `internal/replay` (EPIC-002) их не видит: правило `internal-replay` разрешает только себя, replay управляет процессом через `runtime.Deps`. Поэтому «доступен EPIC-002 `replay`» читается так: в `mode=replay` контекст `llm` сам подменяет провайдер на `recorded` (T-212). EPIC-005 golden пользуется ими тестами из `internal/llm` (T-244). Рою `providers/fake` разрешён **только в тестах** (решение 1c, правило — T-445). В тот же пакет `recorded` T-221 добавит `Writer` (решение 1b).
Что сделать: `providers/fake` — таблица `(phase, matcher) → Response`, счётчик вызовов, задержка ответа (для тестов уступки), генерация «грязных» ответов с преамбулой/`<think>` для тестов парсера, `Models()` из таблицы; `providers/recorded` — ключ `(meta.correlation_id, meta.agent.id, phase, attempt)`, чтение `*.jsonl` и журнала `llm_records`, `ErrIncompleteRecord` при промахе.
Файлы: `internal/llm/providers/{fake,recorded}/*.go`.
Зависит от: T-206.
Ссылки: US-017, US-012, NFR-014/061, C-07, C-15, ADR-010 п. 2, design §3.1 B2 / §4.3, КД §9.1.
DoD (специфика):
- [ ] Промах ключа у `recorded` → `ErrIncompleteRecord` (не тихий шаблон, не живой вызов).
- [ ] Счётчик `FakeProvider.Calls()` используется в тесте «replay: `llm_calls=0`».
- [ ] ~~Пакет доступен потребителям вне EPIC-003 (EPIC-002 `replay`, EPIC-005 golden) — экспортируемое API описано в `internal/llm/providers/README.md`.~~ **(ревизия 4)** Экспортируемое API описано в `internal/llm/providers/README.md`, там же — кто может импортировать пакеты по `depguard` (абзац выше). Тест `mode=replay` → `recorded` — в T-212.
- [ ] ~~Ветка задачи слита в `integration/mvp-1` сразу после приёмки (ранняя поставка C-07/C-15).~~ **(ревизия 4)** После приёмки и слияния в эпик tech-lead#2 просит у оркестратора контрольное слияние эпика в `develop` (поставка C-07/C-15). Условия — шапка индекса.

#### T-208 · B3a · Провайдер `openai_compat` (llama-server) — **провайдер по умолчанию**
Инкремент I1a · подволна **C** *(ревизия 4; было 1.3)* · developer#2 · Размер M · Статус todo · ветка `task/T-208-openai-compat-provider`
*(переномерован сведением 2: бывший B3 `ollama` → B3a `openai_compat`; U-8, ADR-005 доп. 2 п. 1–4, C-15 v1.1)*
**(ревизия 4) По дереву и `contracts.md` v0.10.**
- **Адрес.** Базовый адрес берётся только из `MV_LLM_URL` (обязательна, без умолчания). Хвост `/v1` в значении допустим и отбрасывается, путь `/v1/...` дописывает клиент. Правило то же, что у `scripts/lib/llm-endpoint.sh` и `LlmEndpoint.psm1` (T-404): тест на оба написания даёт один и тот же URL запроса. Порт 1234 в фикстурах и текстах не закладывается: у стенда владельца другой порт.
- **Id модели.** `/v1/models` отдаёт id модели — имя файла без `.gguf`, при условии что сервер поднят с `--alias` (T-437, ADR-005 доп. 3 п. 6). В router-режиме (`--models-dir`) модели называет сам сервер. Сверка «модель блупринта ∈ `Models()`» сравнивает строки как есть и ничего не выводит из пути к файлу. Фикстура `/v1/models` снимается с сервера, поднятого через `make llm-up`.
- **Умолчания семплинга фазы** — `temperature 0.7 / top_p 0.8 / top_k 20 / min_p 0 / presence_penalty 1.5`. Из блупринта приходят только `temperature` и `max_tokens` (T-203), остальное подставляет провайдер, если `Params` пусты.
Что сделать: `providers/openai_compat/client.go` на `net/http` (без SDK): `Generate` → `POST /v1/chat/completions` с `response_format {type: "json_schema", json_schema: {name, schema}}` (при `Request.Schema != nil`), **`chat_template_kwargs: {"enable_thinking": Params.Think}` — в MVP-1 всегда `false`** (llama.cpp #20345: при thinking грамматика не применяется), семплинг **на запрос** из `Params` (`temperature, top_p, top_k, min_p, presence_penalty, max_tokens`; значения по умолчанию фазы — non-thinking-профиль `0.7 / 0.8 / 20 / 0 / 1.5`, серверные `--temp 1 …` переопределяются), `stream:false`, заголовок `Authorization: Bearer $MV_LLM_API_KEY` только при непустом ключе; `Embed` → `POST /v1/embeddings`; `Models` → `GET /v1/models`; `Health` → `GET /health` (**200 → `ok`; 503 → `loading`, не `unavailable`**) + сверка модели блупринта со списком `Models()` (нет → `degraded(model_not_resident)`); токены из `usage{prompt_tokens, completion_tokens, prompt_tokens_details.cached_tokens}` → `Response.Tokens{Prompt, Completion, Cached}`; задержка из `timings{prompt_ms, predicted_ms}` (при отсутствии — по часам шлюза); `message.reasoning_content` **не сохраняется**, в `Response.ReasoningLen` попадает только длина; таймаут фазы через контекст; отмена контекста закрывает соединение (ADR-014 п. 2); гейт облака по host (`IsLocalEndpoint` из T-206).
Файлы: `internal/llm/providers/openai_compat/*.go`.
Зависит от: T-206.
Ссылки: US-014, US-012, NFR-004/052, **C-15 v1.1**, **ADR-005 доп. 2 п. 1–4**, ADR-014 п. 2, design §3.1 **B3a**, КД §9.1, §9.5.
DoD (специфика):
- [ ] Unit на `httptest` с записанными ответами **llama-server**: `/v1/chat/completions` (со схемой и без), `/v1/models`, `/health` (**200 и 503**), `/v1/embeddings`.
- [ ] Тест: при `Request.Schema != nil` в теле присутствуют `response_format.json_schema` **и** `chat_template_kwargs.enable_thinking=false`; при `Params.Think=true` — `true` (значение прокидывается, но в блупринтах MVP-1 не используется).
- [ ] `Health`: 503 `{"status":"loading"}` → `loading` (шлюз не переводит мир в `unavailable` и не даёт `fallback_reason=unavailable` немедленно); отсутствие модели блупринта в `/v1/models` → `degraded(model_not_resident)`.
- [ ] Токены и `cached_tokens` разбираются из `usage`; при отсутствии `usage` — оценка по длине с пометкой (NFR-052 ≤ 5 %).
- [ ] `reasoning_content` не попадает ни в `Response.Content`, ни в события; в `llm.output.parse{}` уходит только длина.
- [ ] Тест отмены: `ctx.Cancel` во время ответа → соединение закрыто, горутин не осталось (`goleak`), возвращается ошибка отмены.
- [ ] Живой llama-server в unit-тестах не используется (только стенд, T-260/T-263).
- [ ] **(ревизия 4)** `MV_LLM_URL` с `/v1` и без него даёт один URL запроса; пустой `MV_LLM_URL` → ошибка конфигурации, а не адрес по умолчанию. Тест на обе ветки.
- [ ] **(ревизия 4)** Запрос, в котором заданы только `temperature` и `max_tokens` (как из блупринта), даёт в теле эти значения и умолчания фазы `0.8 / 20 / 0 / 1.5` для `top_p/top_k/min_p/presence_penalty`. Как отличить «не задано» от нуля, решает `Params` из T-206. Тест по телу на `httptest`.

#### T-254 · B3b · Провайдер `ollama` (native API) — **второй, условный (после F-8)**
Инкремент I1a (реализация отложена; плановое место — подволна 2.1) · developer#3 · Размер M · Статус todo
**(ревизия 4) Условие старта по решению T-435 (ADR-005 доп. 3).** Замер F-8 закончен: E — базовая, Qwen3.6-35B-A3B — целевая, порядок `E → Qwen3.6-35B-A3B → C → A`. E и Qwen3.6 идут по пути платформы — `llama-server`, `openai_compat`. Задача нужна, только если выбор дойдёт до C или A (Ollama), либо если `llama-server` не обслужит эмбеддинги для EPIC-005. Смотреть исход T-438 и `baseline.md` §5. `MV_OLLAMA_URL` уже объявлена в `shared/env/vars.go`, заново не объявлять.
*(новая задача сведения 2: `ollama` перестал быть провайдером по умолчанию; выполняется, **только если F-8 выбирает конфигурацию C или A**, либо если llama-server не обслуживает эмбеддинги для EPIC-005 — ADR-005 доп. 2 п. 2, п. 8)*
Что сделать: `providers/ollama/client.go` на `net/http` (без модуля `ollama`): `POST /api/chat` (`format`=JSON-схема, `think:false`, `keep_alive`, `options{temperature, top_p, top_k, num_predict, num_ctx, seed?}`, `stream:false`), `POST /api/embed`, `GET /api/tags` → `Models()`, `GET /api/ps` → `Health()`; токены из `prompt_eval_count`/`eval_count`; таймаут фазы через контекст; отмена контекста закрывает соединение (ADR-014 п. 2); регистрация в `providers.Registry` под именем `ollama`, включение — `MV_LLM_PROVIDER=ollama` + `MV_OLLAMA_URL`.
Файлы: `internal/llm/providers/ollama/*.go`.
Зависит от: T-206; **решение F-8** (`ops/metrics/baseline.md`) — до него задача не стартует.
Ссылки: US-014, US-012, NFR-004/052, C-15 v1.1, **ADR-005 доп. 2 п. 2**, ADR-014 п. 2, design §3.1 **B3b**, КД §9.1.
DoD (специфика):
- [ ] Unit на `httptest` с записанными ответами `/api/chat`, `/api/tags`, `/api/ps`, `/api/embed`.
- [ ] Тест отмены (`goleak`), как в T-208.
- [ ] Переключение `MV_LLM_PROVIDER=openai_compat ↔ ollama` не требует изменений в шлюзе, ролях и блупринтах (тест на реестре провайдеров) — критерий US-014.
- [ ] Если F-8 подтвердил конфигурацию E — задача закрывается как **не требуется** с отметкой в `dev-log.md` и в `state.js` (осознанное отрезание, не «забыли»). **(ревизия 4)** То же — если по итогам T-438 выбрана E или Qwen3.6-35B-A3B (обе на `llama-server`).

#### T-209 · B4 · Парсер ответа, компиляция схем, проверка языка
Инкремент I1a · подволна **D** *(ревизия 4; было 1.4)* · developer#2 · Размер M · Статус todo · ветка `task/T-209-llm-parser-language`
**(ревизия 4)** Статуса `rejected_language` больше нет (C-07 v1.2, ADR-017 доп. 1). Нарушение языка — это `llm.output validation_status=invalid` и `llm.output.rejected reason=language`. Парсер возвращает причину `language`, статус и событие строит шлюз (T-212/T-213). `MV_LLM_LATIN_MAX_RATIO` объявляет эта задача, если читает её первой.
Что сделать: `parser/parser.go` — `Parse(raw, schema)` со стратегиями `direct → strip_think → strip_fence → balanced_object → trailing_commas` (ADR-016 п. 1) и возвратом `Strategy`/`recovered`; `parser/schema.go` — компиляция `schemas/agent/*.json` (2020-12); `parser/language.go` — `CheckLanguage`: 0 CJK, доля латиницы ≤ `MV_LLM_LATIN_MAX_RATIO`, идентификаторы (`[a-z0-9-]+:[a-z0-9-]+`, ID сущностей) исключены; корпус `testdata/llm/preamble/*.txt`.
Файлы: `internal/llm/parser/*.go`, `testdata/llm/preamble/*.txt`.
Зависит от: T-206, T-203 (схемы `schemas/agent`).
Ссылки: US-004, US-018, NFR-022/090/091, ADR-016, design §3.1 B4, КД §9.2.
DoD (специфика):
- [ ] Корпус ≥ 12 кейсов: преамбула, `<think>…</think>`, ```json-блоки, trailing commas, вложенный объект, обрезанный JSON, пустой ответ, ответ-массив.
- [ ] Обрезанный JSON → ошибка `schema_invalid` (не «починка наугад»); стратегия и `recovered` попадают в результат.
- [ ] `CheckLanguage`: китайские иероглифы → ~~`rejected_language`~~ **причина `language`** *(ревизия 4)*; латиница в `player-A`/`wolf-alpha` не считается нарушением.

#### T-210 · B5a · Бюджет вызовов LLM
Инкремент I1a · подволна **C** *(ревизия 4; было 1.5)* · developer#3 *(ревизия 4; было developer#2 — поток B занят T-208)* · Размер S · Статус todo · ветка `task/T-210-llm-budget`
**(ревизия 4)** Восстановление окна идёт по событиям `llm.output`, которые может доставить повтор шины или догон (C-01 v1.6 «посредник доставки»). Поэтому учёт идемпотентен по id события, как `budget.Observe` роя в T-227. `MV_LLM_TURN_CALLS_PER_MIN` здесь не читается (по умолчанию выключен, T-250) и не объявляется.
Что сделать: `internal/llm/budget.go` — скользящие окна по ключу `(world, level, phase, provider)`; фон `(world, {global,domain}, tick)` cap `B`; интерактив `(world, task, narrative)` cap `MV_LLM_TURN_CALLS_PER_MIN` (по умолчанию выключен); восстановление окон из `llm_records` при догоне через интерфейс `Journal` (вызов из swarm, задача T-237); `Allow()` c причиной отказа.
Файлы: `internal/llm/budget.go`.
Зависит от: T-206.
Ссылки: US-012, FR-071/072/125, NFR-006/007/050…053, design §3.1 B5, КД §9.3, §7.3.
DoD (специфика):
- [ ] Тест окна на `clock.Manual`: B вызовов проходят, (B+1)-й — отказ; через час окно освобождается.
- [ ] Восстановление окна из последовательности `llm.output` (`actor_kind=system`, `replay=false`) даёт то же состояние, что живой набор вызовов.
- [ ] Отказ бюджета не выполняет вызов провайдера (счётчик `FakeProvider.Calls()` не растёт).
- [ ] **(ревизия 4)** Одно и то же `llm.output`, поданное в восстановление дважды, учитывается один раз. Тест по id события.
- [ ] **(ревизия 4)** Тест окна — на `clock.NewManual` из `shared/clock`; `time.Now`/`time.Since` в пакете нет (`forbidigo`).

#### T-211 · B5b · Запись вызовов (`Recorder`) и учёт (`Usage`) + `GET /v1/admin/llm/usage`
Инкремент I1a · подволна **E** *(ревизия 4; было 1.6)* · developer#2 · Размер M · Статус todo · ветка `task/T-211-llm-recorder-usage`
**(ревизия 4) По дереву.**
- **Причина в конверте** — `meta.causation_id`: так её ставит `eventbus.Derive`. Поля `meta.caused_by` в конверте нет.
- **Объектное хранилище.** В `runtime.Deps` клиента объектного хранилища нет. Промпты при `MV_LLM_STORE_PROMPTS=true` пишутся клиентом `objstore.New(objstore.Config{…})` из переменных `MV_MINIO_*` (уже объявлены) в бакет `objstore.PromptsBucket(world)`. Клиент создаёт контекст `llm` при старте. В тестах — `objstore.NewMemory()`.
- **Условия схемы `llm.output`** (решение 4) выражает сама схема (T-445, EPIC-001). До слияния T-445 в схеме есть только правило «без `response_raw` при `quarantined|filter_error`». После T-445:
  - `response_raw` обязателен при `valid|partially_rejected|invalid`;
  - `filter{status=pass}` — при `valid|partially_rejected`, `filter{status=block}` — при `quarantined`, `filter{status=error}` — при `filter_error`;
  - при `error` обязателен `error{}`, а `response_raw` и `filter` запрещены; при любом другом статусе `error{}` запрещён.

  Условия по стадии конвейера в схему не входят: это свойство-тесты ниже и в T-213.
- **Хеши** `prompt_hash` и `response_hash` — вида `sha256:<64 hex>` (решение 3; шаблон схемы — T-445).
Что сделать: `record.go` — `Recorder` публикует `llm.output` (все поля `data-model.md` §7.2, `parse{}`, `filter{}`, `laws_version`, `lod`, `gm_path` из `meta` события-причины), `llm.output.rejected`, `content.incident.recorded`; события — через `eventbus.Derive(cause)`; при `quarantined`/`filter_error` **нет** `response_raw`; при `MV_LLM_STORE_PROMPTS=true` полный промпт в `prompts-{world}/{cid}/{agent}/{phase}-{attempt}.txt`; `usage.go` — агрегаты `calls/tokens/cost_usd/latency p50,p95` по `(phase, level, provider, model)` и маршрут `GET /v1/admin/llm/usage`, монтируемый на `runtime.Mux`.
Файлы: `internal/llm/{record,usage}.go`.
Зависит от: T-206, T-215 (схемы `llm.*`, `content.incident.recorded`), T-210.
Ссылки: US-018, US-012, FR-032/034/070…072, NFR-041/048/052, C-06, C-07, SEC-23, design §3.1 B5, КД §9.2–9.3.
DoD (специфика):
- [ ] Свойство-тест: при `validation_status ∈ {quarantined, filter_error}` поле `response_raw` отсутствует в 100 % случаев (генератор случайных ответов).
- [ ] Один `llm.output` на попытку (`attempt=1..n`), `meta.agent` обязателен, ~~`meta.caused_by`~~ **`meta.causation_id`** *(ревизия 4)* = id события-причины.
- [ ] `GET /v1/admin/llm/usage` отвечает на `httptest` и не публикуется наружу (монтаж на `runtime.Mux`, `MV_CORE_ADDR`).
- [ ] Ключи облака и их фрагменты не попадают в события и логи (негативный тест по подстроке).
- [ ] **(приёмка T-215, бэклог ревью #1 п. 5; журнал 2026-09-11)** `llm.output.error.message` строится через редактор секретов `shared/logging`, а не из `err.Error()` провайдера как есть. Тест: ошибка провайдера с URL и ключом в тексте даёт `error.message` без них.
- [ ] **(ревизия 4, решение 4)** Свойство-тесты на генераторе исходов конвейера:
  - `filter{}` в `llm.output` при `validation_status=invalid` есть или отсутствует по стадии, на которой ответ отброшен: до фильтра (`schema_invalid`, `language`) — нет, после фильтра (страж, устаревшие законы) — есть;
  - `reasons[]` равно множеству `reason` связанных `llm.output.rejected`;
  - число `llm.output.rejected` с `element` равно числу отброшенных элементов.

  Каждое `llm.output` проходит валидацию по схеме из дерева (после слияния T-445 — с условиями).
- [ ] **(ревизия 4)** `response_hash` пишется как `sha256:` + 64 строчные hex-цифры; тест на форму. На каждый статус конвейера — пример `llm.output`, проходящий схему с условиями T-445, и контрпример, который она отвергает.
- [ ] **(ревизия 4)** Промпт при `MV_LLM_STORE_PROMPTS=true` пишется в `objstore.PromptsBucket(world)` по ключу `{cid}/{agent}/{phase}-{attempt}.txt`; тест на `objstore.NewMemory()`. При `false` обращений к хранилищу нет.

#### T-212 · B6a · `Gateway.Generate`: ядро конвейера, повторы, таймауты, здоровье
Инкремент I1a · подволна **F** *(ревизия 4; было 1.7)* · developer#2 · Размер M · Статус todo · ветка `task/T-212-gateway-core`
**(ревизия 4) По решениям system-architect#1 и дереву.**
- **`config.cloud_enabled`** (решение 2, текст C-06 — T-444). Payload по схеме в дереве: `enabled` обязателен; `provider` — имя из `MV_LLM_PROVIDER`, схема ограничивает его шаблоном `^[a-z][a-z0-9_]{0,31}$`; `external_players_ack` — булево подтверждение оператора. Источник значения назовёт C-06 в T-444; кандидат — `MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS`, до T-444 не закладывать. Прежнее «только булев флаг, без имени провайдера» заменено на «**без URL, ключей и их фрагментов**».
- **Статуса `rejected_language` нет:** язык даёт `invalid` и `llm.output.rejected reason=language`.
- **Имя схемы ответа в `Call` — строка** (решение 1c): рой не импортирует `internal/llm/parser`, а шлюз по имени находит скомпилированную схему (T-209).
- **Контекст `llm` в `cmd/multiverse`** подключает отдельная задача (§6, номер у оркестратора). Шлюз в процессе — один: и контекст `llm`, и рой берут его из одного ленивого стека (`sync.Once`).
Что сделать: `gateway.go` — шаги 0–3 КД §9.2 в порядке «replay → бюджет → `prompt.Build` → цикл попыток (провайдер → парсер → язык)», таймауты фаз и `MV_LLM_TIMEOUT_DEGRADED`, подсказка в `user` при повторе (system неизменен), `Health()` с опросом провайдера раз в 10 с (`ok|unavailable|degraded(model_not_resident)`), публикация `config.cloud_enabled` при старте контекста `llm`.
**Изменение (сведение 3, C-06 v1.1, З-3; внесено tech-lead#1 от имени tech-lead#2)**: `config.cloud_enabled {enabled}` публикуется **при каждом старте `core`** — и при `MV_LLM_CLOUD_ENABLED=true`, и при `false` — а также при изменении значения. Прежнее «только при `true`» отменено: проекция потребителя (gateway, `GET /v1/worlds → worlds[].llm.cloud_enabled`, EPIC-004 T-320) после рестарта иначе недетерминирована. ~~Payload — только булев флаг: без имени провайдера, URL и ключей (SEC-21).~~ **(ревизия 4, решение 2)** Payload — `enabled`, `provider` (имя по шаблону схемы), `external_players_ack`; без URL, ключей и их фрагментов (SEC-21). Шлюз также заполняет опциональное `llm.output.reasons[]` (C-07 v1.2, схема — T-215).
Файлы: `internal/llm/gateway.go`.
Зависит от: T-207, T-209, T-210, T-218 (интерфейс `prompt.Sections`), T-215 (схема `config.cloud_enabled`).
Ссылки: US-004, US-014, US-012, US-008, FR-013/015/070, NFR-002/015/016, SEC-21, ADR-005 п. 2, **`contracts.md` v0.4 C-06 v1.1, C-07 v1.2**, `consolidation.md` §14.1 (З-3), design §3.1 B6, КД §9.2, §9.5, §14 п. 10.
DoD (специфика):
- [ ] На `fake`: ветки `valid`, `error/unavailable`, `budget_exceeded`, ~~`rejected_language`~~ **`invalid` с `reason=language`** *(ревизия 4)*, `schema_invalid → retry → ErrInvalidAfterRetries` покрыты тестами.
- [ ] `prompt_hash` и `system` идентичны между попытками одного вызова.
- [ ] **(сведение 3)** `config.cloud_enabled` публикуется при каждом старте контекста `llm` — тесты на обе ветки (`enabled=true` и `enabled=false`) и на повторную публикацию при изменении значения; ~~событие не содержит ключей, их фрагментов, URL и имени провайдера (негативный тест по подстроке)~~ **(ревизия 4, решение 2)** событие проходит схему (`enabled`, `provider`, `external_players_ack`) и не содержит URL, ключа и их фрагментов. Негативный тест по подстроке: `MV_LLM_URL` с внешним host и `MV_LLM_API_KEY` заданы, в сериализованном событии нет ни host, ни ключа, ни 8-символьного фрагмента ключа.
- [ ] **(ревизия 4)** Имя схемы ответа приходит в `Call` строкой; неизвестное имя → ошибка конфигурации без вызова провайдера (тест).
- [ ] **(ревизия 4, условия `llm.output.rejected` — T-445)** Ветка `budget_exceeded`: `llm.output.rejected` несёт `budget{}` и не ссылается на `llm_output` (вызова не было). Тест проходит схему.
- [ ] В `mode=replay` провайдер подменяется на `recorded`, новых `llm.output` не издаётся.

#### T-213 · B6b · `Gateway.Generate`: фильтр, страж, запись, публикация `rejected`
Инкремент I1a · подволна **G** *(ревизия 4; было 1.8)* · developer#2 · Размер M · Статус todo · ветка `task/T-213-gateway-filter-guardian`
**(ревизия 4, решение 1a)** Страж получает законы и механику не типами `internal/laws` и `internal/mechanics`, а через `guardian.Input` с интерфейсами `View`, `InvariantFunc`, `LawsSource` (T-217). Поэтому шлюз от T-205 не зависит: в unit-тестах — фейки этих интерфейсов. Стык «настоящие законы → страж» проверяет интеграционный тест роя (T-238).
Что сделать: достроить конвейер шагами «фильтр → страж (чистая функция) → запись `llm.output` **до использования** → публикация `llm.output.rejected` по каждому отброшенному элементу → возврат `Result`»; статусы `valid|partially_rejected|invalid|quarantined|filter_error`; повтор при `law_violation (stale laws)` с обновлением `laws_version` в промпте; итоговое отображение ошибок в `Err*`.
Файлы: `internal/llm/gateway.go`.
Зависит от: T-212, T-216, T-217, T-211. ~~T-205~~ *(ревизия 4, решение 1a: зависимость ушла в интеграционный тест роя T-238)*.
Ссылки: US-016, US-018, US-037, FR-050/051/053, NFR-021/022/026/048, ADR-005 п. 2, ADR-017, design §3.1 B6, КД §9.2, §10.1.
DoD (специфика):
- [ ] e2e-unit на `fake`: все восемь ветвей статусов дают ожидаемые события и ошибки; порядок эффектов — запись раньше `rejected` и раньше возврата значения.
- [ ] `quarantined` → `content.incident.recorded` + отсутствие `response_raw`; `filter_error` → fail-closed (значение не возвращается).
- [ ] Stale laws: один повтор с новой версией, при повторном расхождении — `invalid`. **(ревизия 4)** Версия приходит через фейк `guardian.LawsSource`, `internal/laws` в тесте не участвует.
- [ ] Ровно один `llm.output` на попытку при любой ветке.
- [ ] **(ревизия 4, решение 4)** Свойство-тесты сквозного конвейера на `fake`: при `invalid` наличие `filter{}` соответствует стадии отказа; `reasons[]` = множество `reason` опубликованных `rejected`; число `rejected` с `element` = числу элементов, отброшенных стражем; при `partially_rejected` применённый остаток не содержит отброшенных элементов.
- [ ] **(ревизия 4, условия `llm.output.rejected` — T-445)** Каждое `llm.output.rejected`, кроме `budget_exceeded`, ссылается на своё `llm_output` и не несёт `budget`. `unknown_entity` несёт `element` и либо `entity`, либо `background_ref{event{id}}` (`$defs.EventRef`). Все публикации конвейера проходят схемы после слияния T-445.

### Поток C — схемы событий, фильтр, страж, промпт, testkit (developer#3)

#### T-214 · A5 · Схемы событий EPIC-003, часть 1 (рой, тики, мир/регион/NPC, законы) — **ранний merge**
Инкремент I1a · подволна 1.1 (выполнена до перезапуска) · developer#3 · Размер M · Статус **done** *(принята tech-lead#2 2026-09-13 по дереву `fd52e31`, итераций ревью 1; подробности — `tasks/T-214.md`)*
**(ревизия 4) Статус.** Работа в дереве `develop` с 2026-09-11. Ревью #1 (TEAM-2/code-reviewer#1, 2026-09-13, по дереву `acd653c`): **принять** — Critical 0, Major 0, Minor 3, Nit 3. Реестр 27 типов верен, невалидные фикстуры падают ровно на описанной причине. Осталось: приёмка tech-lead#2 и раздел ревью в `review.md` эпика — его вносит ревьюер в ветке эпика. Пункт DoD «слито в `integration/mvp-1`» читается как «в `develop`»: схемы там с контрольного слияния EPIC-001 (`1c2ee7e`).
Что сделать: `schemas/events/*.v1.json` для `tick.fired` (`tick.lod_allowed` обязателен), `tick.aborted`, `agent.spawned` (+`content_hash`), `agent.child_resolved`, `agent.stopped`, `agent.spawn_rejected`, `agent.blueprint_reloaded`, `encounter.started` (+`round{timeout, idle_after_missed}`), `encounter.ended`, `combat.decided`, `world.weather_changed`, `world.time_advanced`, `world.event_occurred`, `region.event_occurred`, `npc.moved`, `npc.spawned`, `world.laws.changed`, `world.law_breach.{proposed,rejected,applied,review_decided,rolled_back}` (без издателя); регистрация типов и топиков — PR в `shared/contracts/registry.go` (ревью system-architect); фикстуры payload по `api-contracts.md` §2.3.6–2.3.9, 2.3.13.
Файлы: `schemas/events/*.v1.json`, PR в `shared/contracts/registry.go`, `testdata/fixtures/events/*.json`.
Зависит от: F-4a/F-4b (внешние).
Ссылки: US-036, US-037, US-011, C-05 v1.1, C-06, C-12, ADR-007, ADR-008, design §3.1 A5, КД §14.
DoD (специфика):
- [x] `mvctl contracts check` зелёный: схемы валидны, тип → топик по §0, `world.law_breach.*` в списке исключений «без издателя».
- [x] Для каждого типа — фикстура валидного и невалидного payload; `MV_BUS_VALIDATE_ON_READ` отклоняет невалидный.
- [x] Слито в `integration/mvp-1` до задач C4 (T-219/T-220) и до I1-α. *(ревизия 4: в `develop` с `1c2ee7e`)*

**Приёмка (tech-lead#2, 2026-09-13): принята.** `mvctl contracts check` (65 типов, 8 топиков, 58 схем), `go test -short -count=1 ./shared/contracts/... ./test/fixtures/...`, `golangci-lint run ./shared/contracts/... ./test/...` — зелёные в дереве эпика `fd52e31`. Minor ревью #1: Mi-5 закрыт T-444 (`scope` встречи) и T-445 (`$defs.EventRef`), кроме словаря `tick.aborted.reason`; Mi-3, Mi-4, N-2…N-4 не блокируют и разнесены: Mi-4 — DoD T-230, остальное — §13.

#### T-215 · A6 · Схемы событий, часть 2 (LLM и нарратив) — **ранний merge**
Инкремент I1a · подволна 1.2 (выполнена до перезапуска) · developer#3 · Размер S · Статус **done** *(принята tech-lead#2 2026-09-13 по дереву `fd52e31`: Ma-1 и Mi-1 закрыты T-445; итераций ревью 1; подробности — `tasks/T-215.md`)*
**(ревизия 4) Статус.** Ревью #1 (TEAM-2/code-reviewer#1, 2026-09-13): **вернуть** — Critical 0, Major 1, Minor 3, Nit 2.
- **Ma-1.** Условная обязательность в `llm.output.v1.json` не выражена: `response_raw` при `valid|partially_rejected|invalid`, `filter` и `error{}` при `error`. Закрывается задачей **T-445** (EPIC-001, решение 4 system-architect#1: условия `llm.output` и `llm.output.rejected` — в схеме). Итерация T-215 в ветке EPIC-003 не нужна.
- **После слияния T-445** в `develop` и `sync` эпика — приёмка tech-lead#2 по дереву. Minor и Nit ревью #1 разбираются там же: закрыты T-445 или уходят строками бэклога.
Что сделать: `llm.output` (+`parse{strategy, recovered}`, `error.code` с `yielded`), `llm.output.rejected` (`reason` enum ADR-017 п. 3), `narrative.output` (+`narrative_event_id`), `content.incident.recorded`, `config.cloud_enabled`; политика топика `llm_records`: `meta.agent` обязателен, `actor_kind` ∈ `human|ci|sim|system`.
Файлы: `schemas/events/{llm.output,llm.output.rejected,narrative.output,content.incident.recorded,config.cloud_enabled}.v1.json`, PR в `registry.go`.
Зависит от: T-214.
Ссылки: US-004, US-018, US-016, FR-032/034/045, C-05 (contracts v0.3+), **C-07 v1.2**, ADR-017 **+ «Дополнение 1»**, `consolidation.md` §14.1 (TL2-1, З-3), design §3.1 A6, КД §14.
**(сведение 3, внесено tech-lead#1 от имени tech-lead#2)** Решение принято, замечание §10 п. 1 закрыто: `validation_status` — **единый enum из 6 значений** `valid | partially_rejected | invalid | error | quarantined | filter_error`. Причины — **только** в `llm.output.rejected.reason` (`unknown_entity | player_agency | level_violation | schema_invalid | language | filter_blocked | filter_error | budget_exceeded | law_violation | other`), одно событие на отброшенный элемент (`element{index,type}`), без `element` — на весь ответ; `budget_exceeded` — **не статус**, а `llm.output.rejected` без `llm.output`. Значения `rejected_*` из `data-model.md` §7.2 v0.2 удалены (никогда не издавались). Добавляется **опциональное** поле `llm.output.reasons[]` (сводка `reason` связанных `rejected`; заполняет шлюз — денормализация для `mvctl llm-usage` без join).
DoD (специфика):
- [x] `reason` enum в схеме совпадает с `guardian/reasons.go` (тест равенства; при отсутствии пакета — TODO-тест, включается в T-217).
- [x] **`enum validation_status` — ровно 6 значений** `valid|partially_rejected|invalid|error|quarantined|filter_error` (C-07 v1.2, ADR-017 доп. 1); негативный тест: документ с `rejected_unknown_entity`/`budget_exceeded` в `validation_status` не валиден. `reasons[]` — опциональный массив значений enum `reason`.
- [x] `config.cloud_enabled` — схема без изменений; **семантика (C-06 v1.1)**: событие публикуется при **каждом** старте `core` (и `true`, и `false`) и при изменении — иначе проекция `worlds[].llm.cloud_enabled` у gateway (EPIC-004 T-320) после рестарта недетерминирована. Реализация публикации — T-212/T-213 (поток B, шлюз `llm`), см. пометку там.
- [x] Слито в `integration/mvp-1` (нужны EPIC-004 для доставки и EPIC-005 для отчётов). *(ревизия 4: в `develop` с `1c2ee7e`)*

**Приёмка (tech-lead#2, 2026-09-13): принята.** Ma-1 закрыт T-445: шесть `if/then` по статусу (`llm.output.v1.json:110–176`), таблица C-07 v1.3 построчно (`llmrecord_test.go:106–174`). Mi-1 закрыт T-445: `budget_exceeded`/`unknown_entity` (`llm.output.rejected.v1.json:59–89`, `llmrecord_test.go:201–240`). Шаблон `provider` закреплён в C-06 v1.2 (`contracts.md:518`) и работает (проба приёмки: пять имён приняты, адрес, адрес с ключом и `OpenAI` отвергнуты), но теста на него в дереве по-прежнему нет — Mi-2 открыт, §13 п. 1. Прогоны — как у T-214. «Схема без изменений» у `config.cloud_enabled` — нарушение снято текстом C-06 v1.2 (`contracts.md:526`).

#### T-216 · C1 · Фильтр категории (a) и `config/absolute-limits.yaml`
Инкремент I1a · подволна **A** *(ревизия 4; было 1.5)* · developer#3 · Размер M · Статус todo · ветка `task/T-216-category-a-filter`
**(ревизия 4)** Зависимость от T-206 снята: фильтр не использует типов шлюза, связь была номинальной. `filter_version` в `llm.output.filter{}` проставляет шлюз (T-213). **Перед стартом** нужен словарь категории (a): готовит security-engineer, утверждает пользователь, золотой набор — маркерные пары. Вопрос пользователю задаёт оркестратор.
Что сделать: `filter/filter.go` — интерфейс `NarrativeFilter` и `CategoryAFilter` (RE2 без lookahead, `context_terms` в окне 80 символов, `filter_version`), fail-closed при ошибке реализации; `filter/config.go` — загрузка `config/absolute-limits.yaml`, встроенный словарь по умолчанию при пустом `terms` + предупреждение; `prompt_notice` для секции `<absolute_limits>`; реестр категорий (b)–(g) с `enabled: false`.
Файлы: `internal/llm/filter/*.go`, `config/absolute-limits.yaml`, `internal/llm/filter/testdata/golden/*.txt`.
Зависит от: ~~T-206~~ — *(ревизия 4: снято)*; словарь категории (a), утверждённый пользователем.
Ссылки: US-016, FR-050/051/053, BR-08, NFR-048/065, SEC-24, design §3.1 C1, КД §13.5.
DoD (специфика):
- [ ] Золотой набор: 0 ложных срабатываний на жанровом насилии и нейтральных текстах; позитивные кейсы категории (a) блокируются.
- [ ] Ошибка реализации фильтра → `filter_error` и отказ доставки (fail-closed), а не «пропустить как есть».
- [ ] Смена словаря в YAML применяется без пересборки (тест перезагрузки конфигурации).
- [ ] ~~`filter_version` попадает в `narrative.output.filter{}` и `llm.output.filter{}`.~~ **(ревизия 4)** Фильтр отдаёт `filter_version` в результате проверки (тест). Попадание в `llm.output.filter{}` проверяет T-213, в `narrative.output.filter{}` — T-233.

#### T-217 · C2 · Страж (`guardian`): правила 3–6, видимость, реестр причин
Инкремент I1a · подволна **F** *(ревизия 4; было 1.7)* · developer#3 · Размер M · Статус todo · ветка `task/T-217-guardian`
**(ревизия 4, решение 1a)** Правило `internal-llm` запрещает `internal/llm/**` видеть `internal/laws` и `internal/mechanics`. Поэтому `guardian.Input` **не содержит их типов**: представление мира — интерфейс `View`, проверка инварианта — `InvariantFunc` (по `check`-ключу), текущая версия законов — `LawsSource`. Реализации подаёт вызывающий: рой (T-228/T-233) поверх `internal/laws` и `internal/mechanics`, в тестах — фейки. Точные сигнатуры — по тексту T-444, до него — «минимум, нужный правилам 3–6». Рою пакет `guardian` разрешён отдельной строкой `depguard` (`internal/llm/guardian$`, решение 1c, T-445).
Что сделать: `guardian.go` — `Evaluate(value, Input) Verdict` (чистая функция, без шины): правило 3 `unknown_entity`, 4 `player_agency`, 5 `level_violation`, 6 `law_violation` для двух видов схем (`narrative`, `tick`), гипотетическое применение `ops` к копии сущности, проверка инвариантов по `check`-ключам из `Input.Invariants`, `inv-11` (`laws_version_current`); `visibility.go` — таблица КД §10.3; `reasons.go` — enum причин (единый со схемой `llm.output.rejected`).
Файлы: `internal/llm/guardian/*.go`.
**Дополнение (сведение 3, ADR-017 доп. 1 п. 5, C-02 v1.2; З-2; внесено tech-lead#1 от имени tech-lead#2)**: `status = abandoned` для правил стража **эквивалентен `dead`** — сущность остаётся **видимой** (нужна для фраз «его больше нет»), таблица видимости КД §10.3 не меняется, но любые `ops` над ней → `law_violation` c `inv-01 dead_does_not_act` (инвариант расширен на `status ∈ dead | abandoned | ascended_final`).
Зависит от: ~~T-205 (типы `laws`)~~ *(ревизия 4, решение 1a: удалено)*, T-215 (enum причин и 6 значений `validation_status`; схемы в дереве, задача в `review`), T-202 (`levels`).
Ссылки: US-003, US-018, US-037, NFR-020/021/026, BR-06/BR-16, **ADR-017 + «Дополнение 1»**, **`contracts.md` v0.4 C-02 v1.2, C-07 v1.2**, design §3.1 C2, КД §10.
DoD (специфика):
- [ ] Таблица тестов «9 правил × 2 вида схем» (ADR-017); итоговые статусы `valid|partially_rejected|invalid` покрыты (`Verdict.Status` — по-прежнему 3 значения; сведение статусов конвейера в `validation_status` из 6 значений — шлюз, T-213).
- [ ] **(сведение 3)** тест: `ops` над сущностью со `status=abandoned` → `law_violation`/`dead_does_not_act` наравне с `dead`; сущность при этом видима (в `mentions` не удаляется).
- [ ] Пустая карта инвариантов → правило 6б пропускается и возвращает признак `unknown_check` (для `/health degraded`), тест на это есть.
- [ ] `background_refs` пропускаются только если присутствовали в контексте вызова; `mentions` вне видимости удаляются, текст остаётся.
- [ ] Страж не публикует событий и не обращается к шине (проверяется отсутствием зависимости пакета от `eventbus`).
- [ ] **(ревизия 4, решение 1a)** `go list -deps ./internal/llm/guardian/...` не содержит `internal/laws`, `internal/mechanics` и `internal/swarm`. Правила 6а/6б проверены на фейках `View`, `InvariantFunc`, `LawsSource`.
- [ ] **(ревизия 4)** Тест равенства enum причин `guardian/reasons.go` со схемой `llm.output.rejected` (прежний TODO-тест T-215) включён и зелёный.
- [ ] **(приёмка T-215, ревью #1 N-1)** `TestRejectionReasonsMatchTheGuardianPackage` (`test/fixtures/events_test.go:405`) считает причиной **любой** строковый литерал `reasons.go`. Поэтому `reasons.go` содержит только словарь причин (сообщения и `String()` — в других файлах), либо тест переводится на константы именованного типа причины. Выбранный вариант записан в dev-log.

#### T-218 · C3 · Промпт-билдер: секции, экранирование, рендер событий, `prompt_hash`
Инкремент I1a · подволна **D** *(ревизия 4; было 1.6)* · developer#3 · Размер M · Статус todo · ветка `task/T-218-prompt-builder`
**(ревизия 4)** `prompt_hash` — в виде **`sha256:<64 hex>`** (решение 3, шаблон схемы — T-445). Рою пакет `prompt` разрешён отдельной строкой `depguard` (`internal/llm/prompt$`, решение 1c): `prompt.Sections` собирает T-235.
Что сделать: `prompt/builder.go` — `Build(Sections) (system, user, promptHash)` по КД §11.4 (порядок секций фиксирован); `placeholders.go` — подстановка словаря блупринта; `escape.go` — `<`/`>` → сущности для `<player_text>` и `<facts source="memory">`, лимит 500 символов; `events.go` — перенос `clusterEvents`/`formatEventDescription` из `services/narrative-orchestrator/narrativeorchestrator/prompt_builder.go`; тесты переписаны из `prompt_builder_test.go`.
Файлы: `internal/llm/prompt/*.go`.
Зависит от: T-201 (плейсхолдеры), T-216 (`prompt_notice`).
Ссылки: US-004, US-016, US-018, FR-051, NFR-041/043, SEC-17, ADR-005 п. 4, design §3.1 C3, КД §11.4, §17 I1-1.
DoD (специфика):
- [ ] `prompt_hash = SHA-256(system + "\n \n" + user)` стабилен между прогонами при одинаковых входах (тест повторяемости). **(ревизия 4)** Запись — `"sha256:" + hex` (64 строчные hex-цифры); тест на `^sha256:[0-9a-f]{64}$`.
- [ ] Инъекция `</player_text><task>…` в тексте игрока экранируется; тест SEC-17 на 10 типовых инъекциях (структурная часть; поведенческая — e2e T-243).
- [ ] `system` не содержит данных игрока (кэшируемая часть) — проверяется тестом.
- [ ] Исходный `prompt_builder.go` в `services/narrative-orchestrator` **не изменяется** (профиль `legacy` живёт до S5).

#### T-219 · C4a · `testkit/swarm.FakeEncounter` — Phase 1 без роя — **ранний merge `shared/testkit/swarm`, вход в I1-α**
Инкремент I1a · подволна 1.3 (выполнена до перезапуска) · developer#3 · Размер M · Статус **done** *(ревизия 4; принята tech-lead#1 после ревью #3, 2026-09-11; затем T-415 и T-419 — там же)*. Пункты «ранний merge в `integration/mvp-1`» ниже исполнены иначе: код в `develop` с контрольного слияния EPIC-001 (`1c2ee7e`).
*(сведение 2, запрос d: **C-05 (contracts v0.3+)** — `FakeEncounter` является **единственной** заглушкой боя Phase 1; `FakeNarrator.WithEncounterStub` в F-10 **не делается**. Подпакет `shared/testkit/swarm` вливается в `integration/mvp-1` сразу после приёмки — **ранний merge**, не дожидаясь остального I1a.)*
Что сделать: `shared/testkit/swarm/fake_encounter.go` — подписчик `player_events` в процессе `core`, включается флагом `MV_SWARM_FAKE=true`, который **читает `cmd/multiverse`** (хук `fake_contexts.go`, задача T-255; ADR-001 доп. п. 8) — сама заглушка флаг не читает и на `cmd/**` не завязана: `player.entered_region` → при живом NPC в регионе и отсутствии активной встречи `entity.create.proposed{encounter}` + `encounter.started{…, round{60s,2}}`; `player.attacked` → `mechanics.Rules.Resolve` (реальный из EPIC-002 или `FixedMechanics`) → `dice.rolled` ×4 → `combat.decided` ×2 → `entity.update.proposed{atomic, expected_version}` → при смерти NPC `encounter.ended reason=npc_dead`; `player.flee_attempted` → бросок бегства, при провале — свободная атака NPC; `player.rested` вне встречи не обрабатывается; `meta.agent = {id: "fake-encounter:solo:<player>", level: task, blueprint: encounter-wolf}`, `gm_path` из `meta.gm_path` причины.
Файлы: `shared/testkit/swarm/fake_encounter.go`, `shared/testkit/swarm/fake_context.go` (`FakeContext` — реализация `runtime.Context{Start, Stop, Health}`, поднимает `FakeEncounter` и `FakeNarrator`; регистрируется хуком T-255 под именем контекста `swarm`).
Зависит от: T-214 (схемы), F-10 (`FixedMechanics`, `FakeState`, фикстуры), F-2/`shared/runtime` (интерфейс `Context`).
Ссылки: US-002, US-003, C-03, **C-05 (contracts v0.3+)**, ADR-001 доп. п. 8, design §4.1, §11 п. 1, КД §2, §8.1, §20 п. 11.
DoD (специфика):
- [ ] e2e `solo-30` на `membus` + `FakeState` + `Harness` v0 проходит **без роя**: 30 ходов, 0 действий без ответа, 0 паник.
- [ ] Смерть NPC, смерть игрока, провал `flee` покрыты тестами; трофей выдаётся один раз.
- [ ] Все публикации проходят валидацию схем (`MV_BUS_VALIDATE_ON_READ=true`).
- [ ] `FakeContext` реализует `runtime.Context` и стартует/останавливается без обращения к `internal/**`; пакет `shared/testkit/swarm` **не зависит от `testify` и тестовых пакетов** (условие ADR-001 доп. п. 8 — иначе хук нельзя собрать в production-бинарнике).
- [ ] Помечено в README `shared/testkit/swarm` как временная заглушка I1-α с условием снятия (переключение e2e на рой — T-242; удаление хука — T-256).
- [ ] **Ранний merge:** ветка задачи слита в `integration/mvp-1` сразу после приёмки — сливаются только пути владения TEAM-2 (`shared/testkit/swarm`, `schemas/events/<типы EPIC-003>`), без `internal/**` (нужно EPIC-004 для I1-α и EPIC-002 для e2e).

#### T-220 · C4b · `testkit/swarm.FakeNarrator` и шаблоны `template/ru.go` — **ранний merge `shared/testkit/swarm`, вход в I1-α**
Инкремент I1a · подволна 1.4 (выполнена до перезапуска) · developer#3 · Размер M · Статус **done** *(ревизия 4; принята tech-lead#2, итерация 3 сверена по диффу, 2026-09-11)*. Код в `develop` с `1c2ee7e`, вместо «ранний merge в `integration/mvp-1`».
*(сведение 2, запрос d: **C-05 (contracts v0.3+)** — F-10 даёт `FakeNarrator` **v0 без боя**, `WithEncounterStub` не делается; бой поставляет только `FakeEncounter` T-219.)*
Что сделать: реализация вместо v0 из F-10: `narrative.output {generated_by: template, fallback_reason: unavailable, filter{applied:false}, laws_version: v1}` на `player.entered_region` (`entry`), `player.looked` (**`entry`**, не `turn` — C-05 v1.2, сведение волны 0), `encounter.started` (`world_event`), последнее `combat.decided` цикла (`turn`), `entity.updated(player) status=dead` (`death`), `round.closed` (`round`, для EPIC-004 I2); шаблоны — `shared/testkit/swarm/template/ru.go` (те же тексты переиспользуют роли в I1b, задача T-229).
Файлы: `shared/testkit/swarm/{fake_narrator.go,template/ru.go}`.
Зависит от: T-215 (схема `narrative.output`), T-219.
Ссылки: US-004, US-002, **C-05 (contracts v0.3+)**, BR-14, FR-015, design §4.1 п. 2, §11 п. 1, КД §8.4.
DoD (специфика):
- [ ] Все шесть `kind` покрыты тестами; `recipients[]` содержит только игроков scope.
- [ ] Текст без разметки, требующей `parse_mode` (SEC-10): тест на отсутствие `<`, `*`, `_` в шаблонах.
- [ ] `narrative_event_id` = `id` события.
- [ ] `FakeNarrator` **не содержит логики боя** (Phase 1 — только `FakeEncounter`, C-05 (contracts v0.3+)): негативный тест «на `player.attacked` нарратор не публикует `dice.rolled`/`combat.decided`».
- [ ] **Ранний merge:** слито в `integration/mvp-1` вместе с T-219, только пути `shared/testkit/swarm` (совместная поставка C-05 для EPIC-004 и EPIC-002).

#### T-255 · Хук `cmd/multiverse/fake_contexts.go` (`MV_SWARM_FAKE`) — **совладение с EPIC-001, вход в I1-α**
Инкремент I1a · подволна 1.4 (выполнена до перезапуска) · developer#1 · Размер S · Статус **done** *(ревизия 4; принята tech-lead#1, итерация 3 сверена по диффу, 2026-09-11)*. Файл, правило `depguard` `cmd-multiverse-fake-contexts` и исключение в `no-testkit-in-production` в дереве есть; «PR через tech-lead#1» исполнен в одной ветке, пока команда была одна.
*(новая задача сведения 2, запрос e / ADR-001 доп. п. 8: флаг читает **`cmd/multiverse`**, а не контекст `swarm` — на момент I1-α `internal/swarm` в `integration/mvp-1` отсутствует.)*
Что сделать: файл `cmd/multiverse/fake_contexts.go` — при `MV_SWARM_FAKE=true` регистрирует `shared/testkit/swarm.FakeContext` (реализация `runtime.Context`, T-219) под именем контекста **`swarm`**; при наличии `internal/swarm` (после слияния I1) регистрируется настоящий контекст, фейковый — только при явном флаге; объявление `MV_SWARM_FAKE` через `shared/env.Declare` (флаг принадлежит процессу, не контексту); короткий комментарий в файле: «временный хук I1-α, удаляется задачей T-256 (критерий тега `mvp-1/i1`)».
Файлы: `cmd/multiverse/fake_contexts.go` (**чужой путь владения — EPIC-001/tech-lead#1**).
**Порядок работы (обязательно):** файл не правится напрямую в ветке эпика — изменение оформляется **PR через tech-lead#1** (совладение, `plan/ownership.md` v0.3); исключение `depguard` по этому файлу (единственный разрешённый импорт `shared/testkit/*` в production-бинарнике) и правка `.golangci.yml` — **на стороне EPIC-001**, TEAM-2 оформляет запрос в отчёте.
Зависит от: T-219 (`FakeContext`), F-2 (`shared/runtime`, регистрация контекстов), F-10.
Ссылки: **ADR-001 доп. п. 8**, `contracts.md` v0.3 §0 (исключение «публикует только владелец типа» для фейков), C-05 (contracts v0.3+), design §4.1 п. 1, §13 п. 1.
DoD (специфика):
- [ ] ~~`MV_SWARM_FAKE=true` → процесс `core` поднимается с фейковым контекстом `swarm`; `MV_SWARM_FAKE` не задан → контекст `swarm` отсутствует (или настоящий, если `internal/swarm` есть) — тест на обе ветки.~~ **Заменено (оркестратор, 2026-09-11):**
- [ ] **(оркестратор, 2026-09-11)** `MV_SWARM_FAKE=true` → процесс `core` поднимается с фейковым контекстом `swarm`. Флаг не задан или `false` → прежняя заглушка `swarm` с `/health`, порядок старта контекстов не меняется. Нечитаемое значение → процесс не стартует, в отказе названо имя переменной. Тест на каждую из трёх ветвей.
- [ ] Бинарник без флага **не тянет** `shared/testkit` в рантайм-путь исполнения (импорт есть только в этом файле); `golangci-lint run` зелёный с исключением по файлу.
- [ ] PR отправлен tech-lead#1 и принят; запрос на исключение `depguard` и на строку в `.env.example` оформлен в отчёте.
- [ ] ~~I1-α: бой с волком и шаблонный нарратив работают в собранном `cmd/multiverse` (не только в тестах).~~ **Заменено (оркестратор, 2026-09-11):**
- [ ] **(оркестратор, 2026-09-11)** Бой с волком и шаблонный нарратив доказываются тестом через путь `process.run` с `FakeState` и харнессом на той же шине. Живой запуск собранного бинарника доказывает только `/health` фейкового `swarm`: `state` в бинарнике пока заглушка. Прогон боя в собранном бинарнике — после EPIC-002, при интеграции I1-α.

#### T-221 · C5 · `RecordingWriter` и `mvctl record`
Инкремент I1a · подволна **L** *(ревизия 4; было 1.6)* · developer#3 *(ревизия 4; было developer#1)* · Размер M · Статус todo · ветка `task/T-221-recorded-writer-mvctl-record`
**(ревизия 4, решение 1b)** Правило `no-testkit-in-production` запрещает `cmd/**` импортировать `shared/testkit`. Поэтому писатель переезжает из `shared/testkit/swarm/recording_writer.go` в **`internal/llm/providers/recorded`** под именем **`Writer`** — рядом с читателем записей T-207, с общим ключом и форматом `*.jsonl`. **Фильтр SEC-23** — отказ при `meta.actor_kind ≠ ci` или игроке вне `player-A/B/C` — живёт **в писателе**, а не в CLI: любой, кто пишет запись, проходит одну проверку. `cmd/mvctl/internal/record` только разбирает флаги и зовёт `recorded.Writer`. Заглушка C-07 «`testkit/swarm.RecordingWriter`» в `contracts.md` устарела — правка передана (отчёт tech-lead#2).
~~Что сделать: `shared/testkit/swarm/recording_writer.go` — запись~~ Что сделать *(ревизия 4: путь — `internal/llm/providers/recorded/writer.go`)*: запись `llm.output` живого прогона (через `Journal.ReadRange` по `llm_records` за окно сценария) в `*.jsonl` по ключу `(correlation_id, agent.id, phase, attempt)`; `cmd/mvctl/internal/record` — `record --scenario <name> --from <journal|jsonl> --out <file>` и `record diff`; отказ при `meta.actor_kind ≠ ci` или игроке вне `player-A/B/C` (SEC-23).
Файлы: ~~`shared/testkit/swarm/recording_writer.go`~~ `internal/llm/providers/recorded/writer.go` *(ревизия 4)*, `cmd/mvctl/internal/record/*.go` (регистрация подкоманды — по правилу T-204).
Зависит от: T-207, T-215 (схема `llm.output` — писатель читает события из журнала по схеме, реализация `Recorder` T-211 не требуется).
Ссылки: US-017, US-011, C-07 v1.1, SEC-23, ADR-010 доп. п. 1–2, design §3.1 C5, КД §18.
DoD (специфика):
- [ ] Unit на фикстуре журнала: запись/чтение по ключу, `record diff` показывает построчную разницу нарративов.
- [ ] Событие с `actor_kind=human` в сценарии → отказ с понятным сообщением; тест есть. **(ревизия 4)** Тест — на `recorded.Writer` напрямую, без CLI (фильтр в писателе); второй тест — CLI передаёт отказ писателя кодом возврата ≠ 0.
- [ ] **(ревизия 4)** `go list -deps ./cmd/mvctl/...` без тестов не содержит `shared/testkit`; `golangci-lint run` зелёный без новых исключений.
- [ ] `.gitattributes` уже содержит `testdata/recordings/*.jsonl merge=binary` (F-1) — проверено, при отсутствии оформлен запрос tech-lead#1.
- [ ] Снятие реальных записей — стендовые задачи T-261/T-262 (не блокируют приёмку).

#### T-439 · Потолок длины нарратива под целевую конфигурацию — **передана из EPIC-001 (ревизия 4)**
Инкремент I1a · подволна **A** · **architect#2** (первая задача architect#2 в ветке эпика), подтверждает tech-lead#2 · Размер S · Статус todo · ветка `task/T-439-narrative-length-cap`
Причина и полный текст — раздел T-439 индекса EPIC-001 (`epics/EPIC-001-foundation/tasks.md`), он источник истины; здесь только то, что нужно эпику. КД §13.3 (`player-gm llm.phase2 max_tokens: 700`, `group-narrator` — «как у `player-gm`») и §13.4 (`narrative.json text.maxLength: 1500`) не проходят NFR-002 ни на E (≈ 163 токена в 5 с), ни на Qwen3.6-35B-A3B (≈ 220). Кроме того, в КД `:735` и `:977` осталась стартовая модель C.
Что сделать: `max_tokens` фазы нарратива и `maxLength`/`maxItems` схемы — по конфигурации: E — 160; Qwen3.6 — предварительно 220, окончательно после T-438. Правило расчёта — формулой из EPIC-001 T-439 п. 2. Модель в КД `:735`, `:977` — по `baseline.md`, запасные — по `decision_order` (E → Qwen3.6 → C → A). Расхождение `temperature: 0.8` в тех же строках КД с non-thinking-профилем 0.7 объём задачи не расширяет: оно передано владельцам общих документов (отчёт tech-lead#2, ревизия 4).
Файлы: `architecture/components/swarm-llm-laws.md` §13.3–13.4 и строки `:735`, `:977`; строка в DoD T-203 этого индекса (вносит tech-lead#2).
**Порядок правки КД:** документ сейчас правит system-architect#1 в T-444. Правка architect#2 идёт после слияния T-444 в `develop` и `sync` эпика, иначе конфликт в одном файле. Если T-444 задерживается, architect#2 передаёт текст правки system-architect#1 для внесения в T-444.
Зависит от: T-435 (done, EPIC-001); окончательные числа — T-438 (стенд); T-444 — только очерёдность правки файла.
Ссылки: ADR-005 доп. 3, `ops/metrics/baseline.md` §5, NFR-002, КД §13.3–13.4, EPIC-001 T-439.
DoD:
- [ ] КД §13.3–13.4 и строка `:977` не противоречат `baseline.md` §5.
- [ ] В T-203 записаны значения `max_tokens` и `maxLength`/`maxItems` для текущей конфигурации (tech-lead#2 подтверждает правкой индекса).
- [ ] Формула расчёта и «символов на токен» (допущение 2,5 до калибровки T-438 шаг 5) записаны рядом с числами.
- [ ] Стендовая проверка «ответ предельной длины не обрезается» передана в T-260 (не блокирует приёмку).

---

## 3. Инкремент I1b «Рой: рантайм, роли, тики, миграция» (25 задач)

Внешние зависимости: I1a полностью; C-02/C-03 — сначала заглушки `FakeState`/`FixedMechanics` (F-10), затем реальные при интеграции (слияние 002 раньше 003); C-04 — `Harness` v0.

### Поток «рантайм» (developer#1)

#### T-222 · R1 · Реестр блупринтов и индекс scope
I1b · **D** · developer#1 · M · todo *(ревизия 4: подволна D, было 1.7)*
Что: `registry.go` — `LoadDir` с валидацией (невалидный файл не активируется, остальные грузятся, признак для `/health degraded {blueprints:[file]}`), `Get`, `ByTrigger`, `ContentHash`; `scope.go` — `ScopeIndex` (`solo/group → region → world`, `PlayersIn`, `RegionOf`, `GroupOf`), обновление из `entity.*`/`group.*`/`encounter.*`.
Зависит: T-202, T-203. Ссылки: US-015, US-037, FR-090/120, КД §2, design §3.2 R1.
DoD: [ ] фикстуры каталога с одним невалидным блупринтом → активны остальные, список в `Health()`; [ ] `ScopeIndex` покрыт таблицей переходов (вход в регион, группа, встреча, выход); [ ] `MV_SWARM_BLUEPRINTS_DIR` объявлен.

#### T-223 · R2a · `AgentInstance`, интерфейс `Behaviour`, `Emitter` с белыми списками
I1b · **E** · developer#1 · M · todo *(ревизия 4: подволна E, было 1.8)*
Что: `instance.go` — данные `AgentInstance` (КД §4.2) и интерфейс `Behaviour{OnEvent, OnTick, Subscribes}`; `emitter.go` — `Emit` через `eventbus.Derive(cause)` + `meta.agent`, проверка `allowed_event_types`/`owned_entity_types` и scope для `task`, `ErrLevelViolation`, лог `handled=false`.
Дополнительно (решение тимлида, разводит потоки «рантайм» и «роли»): здесь же фиксируются интерфейсы `Phase2Runner`/`TickRunner`, через которые роль запрашивает LLM-задание; их реализует Pipeline (T-228), а роли тестируются на фейковом раннере — поэтому роли не ждут T-228.
Зависит: T-222, T-202. Ссылки: US-037, BR-16, NFR-031/034, КД §4.2, §5.4, design §3.2 R2.
DoD: [ ] `Emitter` отклоняет `player.*` и чужие типы сущностей — тест на каждую роль MVP-1; [ ] каждое событие содержит `meta.agent{id, level, blueprint}` и `caused_by`; [ ] интерфейсы `Behaviour`, `Phase2Runner`, `TickRunner` зафиксированы и задокументированы — далее меняются только задачей тимлида (потребители — поток ролей); [ ] фейковый раннер лежит в `internal/swarm/internal/swarmtest` и используется тестами ролей.
- [ ] **(из T-415, 2026-09-11; ревью #2 T-415 п. 1)** Отказ шины при публикации через `Emitter` оставляет след: `Error` в лог (поля `type`, `event_id`, `cause_id`, `err`) и признак для `Health()` роя (выводит T-239). Публикация, прерванная остановкой, отказом не считается (`Info`, без следа) только при двух условиях сразу: `ctx.Err() != nil` **и** `errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)`. Настоящий отказ шины в момент отмены остаётся отказом. Образец — `FakeEncounter.refusedPublication` (T-415); у двойника проверка шире (только `ctx.Err()`), для двойника это допустимо. Тест на три ветки: отказ, отмена, отказ под отменённым контекстом. Правило общее для всех ролей; T-229 и T-421 на него ссылаются.
- [ ] **(architect#2, 2026-09-11; design §14.2, ADR-028)** Интерфейс роли `Spawner`:
  - `SpawnChild(blueprint, scope, cause) (agentID, error)` — поднять дочерний агент;
  - `Alive(agentID) bool` — жив ли агент, в любой фазе.

  Реализует `Lifecycle` (T-225), фейк лежит в `internal/swarm/internal/swarmtest`. Это вызов рантайма, а не другого агента. Интерфейс фиксируется вместе с `Behaviour`.
- [ ] **(architect#2, 2026-09-11; design §14.1)** `Emit` пропускает `DeriveOption` вызывающей роли (`WithCauseID`, `WithScope`). Тест: id события, выведенный из причины, доходит до шины без изменений.

#### T-224 · R2b · `Router.Route`: дедуп, legacy-фильтр, проекции, спавн
I1b · **F** · developer#1 · M · todo *(ревизия 4: подволна F, было 1.9)*
Что: `router.go` шаги 1–4 и 6–8 КД §5.1 (шаги 1 и 8 — по C-01 v1.6, design §14.5 п. 1; изм. T-431): `eventbus.Dedup` по `ev.ID` (LRU 10k, сериализуется в снапшот), игнор deprecated-типов, `WorldView.Apply`/`ScopeIndex.Apply`/`journal.Observe`/`budget.Observe`/`presence.Observe`, `Match` по glob + `scope_binding`, `bindScope` (в т. ч. `personal-gm` = `solo:{entity.entity.id}`), ветки `tick.fired` и `agent.*` в replay.
Зависит: T-223, T-234 (проекции контекста). Ссылки: US-037, NFR-013/014, FR-120…123, КД §5.1, design §3.2 R2.
DoD: [ ] дубль события не меняет состояние (NFR-013); [ ] спавн по glob-триггеру и по `scope_binding.pattern` покрыт тестами; [ ] в replay `agent.stopped`/`agent.spawned` из журнала не публикуются повторно.
- [ ] **(T-431, 2026-09-11; C-01 v1.6 «посредник доставки», design §14.5 п. 1, КД §5.1 шаги 1, 3, 5, 8)** Окно Router двухшаговое.
  - `Has(ev.ID)` — до шагов 2–7, `Add` — только когда все получатели ответили без ошибки.
  - Ошибка получателя возвращается шине (если ошиблись несколько — ошибка первого по порядку `Recipients(ev)`). Ошибка одного получателя не отменяет доставку остальным.
  - Проекции шага 3 идемпотентны при повторе: `WorldView.Apply` — по версии, `ScopeIndex.Apply` — как множество, `journal.Observe` и `presence.Observe` — по `ev.ID` (`budget.Observe` — T-227).

  Тесты:
  - ошибка агента → повтор шины доходит до агента, окно Router id не содержит;
  - ошибка одного из двух получателей → второй событие получил, а на повторе гасит его своим окном;
  - отмена при остановке → окно Router id не содержит, после восстановления событие доходит до агента;
  - дубль после успешной доставки гасится у Router (NFR-013).

#### T-225 · R3 · Lifecycle агентов
I1b · **G** · developer#1 · M · todo *(ревизия 4: подволна G, было 1.10)*
Что: `lifecycle.go` — `Ensure/Stop/Touch/ExpireDue`, детерминированный `agent.id` по `(level, role, scope)`, события `agent.spawned{content_hash}`/`child_resolved`/`stopped`/`spawn_rejected`, TTL на `clock.Timers`, `MV_SWARM_MAX_AGENTS=64` (мягкий лимит, `max_instances` для `global/domain` всегда), replay-ветка (стоп читается из журнала, `ExpireDue` не вызывается).
Зависит: T-223. Ссылки: US-037, US-002 (TTL), NFR-083, FR-016, КД §6, design §3.2 R3.
DoD: [ ] TTL на `clock.Manual`: 45 мин без действий → `agent.stopped reason=ttl`; [ ] повторный `Ensure` при `max_instances=1` не создаёт второй экземпляр (`agent.spawn_rejected reason=max_instances` только для `task`); [ ] `agents_by_level` доступен для `Health()`.
- [ ] **(architect#2, 2026-09-11; design §14.2 п. 5–6, ADR-028)** Требования к `Spawner` (интерфейс — T-223):
  - `SpawnChild` идемпотентен: второй вызов для того же scope возвращает живой экземпляр, второго `agent.spawned` нет;
  - `agent.spawned` дочернего агента получает id из причины (`WithCauseID`, `parts` — `agent.id`), поэтому повтор после отказа публикации даёт тот же id;
  - в фазе догона подъём не публикуется;
  - `Alive` видит экземпляр в любой фазе, включая `finishing`.

  Тест на каждое требование.
- [ ] **(приёмка T-201, 2026-09-13; ревью #3 T-201 бэклог п. 1)** Пустой `ContentHash` блупринта — ошибка подъёма, а не пропуск поля.
  - `agent.spawned.content_hash` не обязателен в схеме, поэтому при `omitempty` пустое значение молча пропало бы из события.
  - `Ensure`/`SpawnChild` с блупринтом без хэша возвращают ошибку, `agent.spawned` не публикуется.
  - Тест на блупринт, собранный в Go без `ParseFile`.

#### T-226 · R4a · Scheduler: две очереди, воркеры, уступка, FIFO на агента
I1b · **H** · developer#1 · M · todo *(ревизия 4: подволна H, было 1.11)*
Что: `scheduler.go` — очереди `interactive`/`background`, `MV_SWARM_LLM_WORKERS=1`, quiet-window `MV_SWARM_BACKGROUND_QUIET`, `max_wait` → `rule-only`, уступка (отмена контекста фонового вызова → `llm.output validation_status=error error.code=yielded`), per-instance FIFO и сериализация по `correlation_id`.
Зависит: T-225, T-213. Ссылки: US-004 (BR-10), US-012, NFR-001/002/005, ADR-014, КД §7.1, design §3.2 R4.
**(ревизия 4, решение 1c)** `internal/swarm` видит из `internal/llm` корень (`llm$`), `prompt$` и `guardian$`, а `providers/fake$` — **только в `_test.go`** (строки `depguard` — T-445). Шедулер держит `llm.Gateway` (корневой пакет), провайдер в production-коде роя не упоминается.
DoD: [ ] тест на `clock.Manual` + `FakeProvider` с задержкой: интерактивное задание вытесняет фоновое, фоновое переходит в `rule-only`; [ ] `max_wait` истёк → задание выполняется без LLM; [ ] нет утечки горутин (`goleak`). [ ] **(ревизия 4)** `go list -deps` production-кода `internal/swarm` (без тестов) не содержит `internal/llm/providers/...`; `golangci-lint run` зелёный без исключений по файлам.

#### T-227 · R4b · Тики, `FireNow`, `BackgroundBudget`, `tick.aborted`
I1b · **I** · developer#1 · M · todo *(ревизия 4: подволна I, было 1.12)*
Что: `RegisterTimer/SetTickMode/FireNow`, интервалы `idle/active` из блупринта, публикация `tick.fired` **до** выполнения с обязательным `lod_allowed` и `budget{window_calls, cap}`, один тик после простоя (`next_tick_at = now + interval`), `tick.aborted reason=restart` после догона (идемпотентно по `tick_seq`), `budget.go` — `BackgroundBudget` на мир (окно 1 ч из `llm.output actor_kind=system, replay=false`, cap `B` из блупринта global), в replay `lod_allowed` читается из события.
Зависит: T-226, T-214. Ссылки: US-036, US-012, FR-124/125, NFR-007/053, C-06, ADR-014, КД §7.2–7.3, design §3.2 R4.
DoD: [ ] `tick_seq` монотонен на агента; в `mode=replay` планировщик тиков не издаёт, `FireNow` работает; [ ] при `window_calls ≥ cap` → `lod_allowed=rule-only`, LLM не вызывается; [ ] «регион переходит в `active` при первом игроке и обратно в `idle`» — тест по `ScopeIndex.PlayersIn`; [ ] число `tick.fired` региона за час без игроков ≤ 2.
- [ ] **(T-431, 2026-09-11; C-01 v1.6, design §14.5 п. 1)** `budget.Observe` идемпотентен по id события. Повтор `llm.output` — шиной после ошибки другого получателя или при догоне — второй раз в `BackgroundBudget` не учитывается и `lod_allowed` не сдвигает. Учтённые id окна бюджета входят в снапшот вместе с окном (T-236). Тест: одно `llm.output` доставлено дважды → `window_calls` вырос на 1.
- [ ] **(T-431, 2026-09-11; design §14.5 п. 1, шаг 6; КД §5.1 шаг 6, §7.2)** Тик закрывается один раз.
  - Ошибка выполнения тика его закрывает: экземпляр отмечает `tick_seq` прерванным и публикует `tick.aborted` с id из причины (`WithCauseID`, `parts` — `tick_seq`).
  - Шине ошибка возвращается, только если `tick.aborted` не опубликован.
  - Повтор `tick.fired` с `tick_seq` не больше последнего закрытого (выполненного или прерванного) тик не выполняет. Если `tick.aborted` не был опубликован, повтор допубликовывает его тем же id.

  Тесты:
  - ошибка тика → `tick.aborted` → повтор `tick.fired` с тем же `tick_seq` тик не выполняет, второго `tick.aborted` нет;
  - публикация `tick.aborted` сорвалась → повтор публикует его тем же id, тик не выполняется.

#### T-228 · R5 · Pipeline и шаблоны деградации в рое
I1b · **J** · developer#1 · M · todo *(ревизия 4: подволна J, было 1.13)*
Что: `pipeline.go` — `HandleEvent`/`HandleTick`, постановка LLM-заданий в очереди, обработка `Err*` шлюза → `fallback_reason ∈ {unavailable,timeout,invalid_after_retries,budget,filter_blocked,filter_error,language,restart}`, перенос шаблонов из `shared/testkit/swarm/template` в `internal/swarm/template` (экспорт для testkit сохраняется), «рестарт между Phase 1 и Phase 2» (КД §8.4), перехват паники роли (`handled=false`, 3 ошибки → `agent.stopped reason=error`; что считает счётчик — строка T-431 ниже).
Зависит: T-226, T-213, T-220, **T-205** *(ревизия 4, решение 1a: адаптер законов для стража)*. Ссылки: US-004, US-019, FR-013/015, BR-14, КД §8.4, design §3.2 R5.
**(ревизия 4, решение 1d)** Переезд шаблонов: `internal/swarm/template` — **листовой пакет**, импортирует только стандартную библиотеку. `shared/testkit/swarm` получает к нему доступ отдельной строкой `depguard` (`internal/swarm/template$`, T-445): двойник `FakeNarrator` переходит на тот же текст шаблонов, а `shared/testkit/swarm/template` удаляется. Правка двойника — в этой задаче: поставщик двойника — EPIC-003. Тесты EPIC-002/004 на двойнике остаются зелёными.
**(ревизия 4, решение 1a)** Pipeline собирает для шлюза `guardian.Input` и реализует `View`, `InvariantFunc`, `LawsSource` поверх `WorldView` (T-234), `internal/mechanics` и `internal/laws` (T-205). Это единственное место, где рой переводит типы законов и механики в интерфейсы стража: роли зовут LLM через `Phase2Runner`/`TickRunner` (T-223), а реализует их Pipeline. Добавлены зависимость от T-205 и строка DoD ниже.
DoD: [ ] все `fallback_reason` покрыты тестами; [ ] Phase 1 не проходит через очереди (тест: `OnEvent` завершается синхронно); [ ] паника роли не роняет процесс и не теряет событие. [ ] **(ревизия 4, решение 1d)** `go list -deps ./internal/swarm/template` — только стандартная библиотека (тест или правило `depguard`); `shared/testkit/swarm` импортирует `internal/swarm/template` и больше ничего из `internal/swarm`; `shared/testkit/swarm/template` удалён, тесты `shared/testkit/swarm`, `cmd/multiverse` и e2e зелёные. [ ] **(ревизия 4, решение 1a)** Адаптеры `View`/`InvariantFunc`/`LawsSource` покрыты тестом на фейковой проекции и настоящих `internal/laws` (`FileSource` на фикстуре) и `internal/mechanics.Invariants()`: устаревшая версия законов доходит до стража, инвариант `inv-01` срабатывает на `dead` и `abandoned`.
- [ ] **(T-431, 2026-09-11; C-01 v1.6, design §14.5 п. 1)** Ошибка роли в Phase 1 возвращается из `HandleEvent` в Router, а оттуда шине, а не только пишется в лог. `HandleEvent` для Phase 1 синхронен. Отмена при остановке — тоже ошибка для Router, окно не пополняется; лог `Info` по правилу T-223. Тест: роль вернула ошибку → `Route` вернул её же.
- [ ] **(T-431, 2026-09-11; приёмка T-431 TL-1, design §14.5 п. 1)** Счётчик «3 ошибки → `agent.stopped reason=error`» считает события, а не доставки.
  - Растёт на ошибке или перехваченной панике роли при первой доставке события. Повтор того же `ev.ID` его не увеличивает.
  - Отказ шины при публикации (след `Emitter` T-223, слот T-421) и отмена в счётчик не входят.
  - Останавливают агента три разных события подряд. Успешная доставка другого события сбрасывает счётчик.
  - Если роль остановлена во время доставки события, Router до `Add` или рестарта помнит «у `ev.ID` получатель остановлен с ошибкой» и на повторе возвращает ту же ошибку.

  Тесты:
  - паника роли на каждой доставке одного события → событие в `dead_letters`, агент жив;
  - три разных события с ошибкой подряд → `agent.stopped reason=error`, а третье событие на повторе получает ту же ошибку и уходит в `dead_letters`, а не коммитится;
  - четыре отказа шины при публикации одного ответа → счётчик 0, агент жив (T-421, тест 2).

### Поток «роли» (developer#2)

#### T-229 · R2c · Видимость: `Behaviour.Subscribes` и `Recipients`
I1b · **H** · developer#2 · M · todo *(ревизия 4: подволна H, было 1.9)*
Что: реализация таблицы КД §5.2 как методов ролей + `Router.Recipients(ev)`; динамический `ParentID` (КД §5.3) при `entity.updated(position)`; игнор собственных событий по `meta.agent.id` (кроме `tick.fired`).
Зависит: T-223, **T-417** (EPIC-001: `Dedup.Has`/`Add`, `WithCauseID` — C-01 v1.4; T-416). Ссылки: US-037, FR-121…123, КД §5.2–5.3, design §3.2 R2, **C-01 v1.4, C-14 v1.2, ADR-027**, **C-01 v1.5** (T-425, 2026-09-11). По C-01 v1.5 паника обработчика перехватывается в `eventbus.Delivery`, но это последняя линия защиты: проверка `ev.ID != ""` в DoD ниже остаётся обязательной (ADR-027 «Уточнение исполнения» п. 4).
DoD: [ ] по одному тест-кейсу на каждую строку таблицы видимости (5 ролей); [ ] «вверх» и «вниз» проверены: `region.event_occurred` доходит до `global-gm` и не доходит до игроков другого региона (US-037); [ ] смена родителя не публикует событий. [ ] **(ревью T-220, 2026-09-11)** событие запоминается в окне дедупликации только после успешной публикации нарратива; при неудаче — `log.Error` и повтор шиной, при постоянном отказе — мёртвая очередь (тест). **Уточнено (T-416, 2026-09-11; C-01 v1.4, ADR-027 п. 3):** окно — `eventbus.Dedup`, своего окна нет. `Has` — до обработки, окно не меняет; `Add` — только после успешной `Publish`; `Seen` здесь не используется. [ ] ~~**(ревью #2 T-220)** `id` нарратива выводится из причины, чтобы дубль при потерянном подтверждении публикации гасился шлюзом по `id` (нужна опция `Derive` — через системного архитектора); окно «уже ответил» входит в снапшот C-14.~~ **Заменено (T-416):** [ ] **(T-416, 2026-09-11; C-01 v1.4, ADR-027 п. 1–2)** `narrative.output` строится с `eventbus.WithCauseID(...)` (T-417). `parts` — вид нарратива, если на одну причину возможны разные виды. `narrative_event_id` = `id`. Тест: повтор публикации той же причины (потерянное подтверждение, рестарт) даёт тот же `id`; разные виды на одну причину — разные `id`. Окно «уже ответил» сериализуется в снапшот роя (C-14 v1.2 (а); запись и восстановление — T-236). [ ] **(ревью #2 T-419, 2026-09-11)** до `Derive(…, WithCauseID(…))` проверяется `ev.ID != ""`: причина без id — след в логе и отказ от ответа, а не паника процесса (правило `shared/eventbus/README.md`); тест.
- [ ] **(из T-415, 2026-09-11; ревью #1 T-415 п. 3)** Публикация нарратива, прерванная остановкой, не пишет `Error` «narrative not published», как сегодня делает двойник (`fake_narrator.go:815-817`). Действует правило `Emitter` T-223: при отмене — `Info`, событие в окно `Dedup` не попадает (`Add` только после успешной `Publish`) и придёт снова. Тест: публикация под отменённым контекстом → в логе нет `Error`, окно событие не запомнило.
- [ ] **(architect#2, 2026-09-11; design §14.2 п. 2, ADR-028)** В строку видимости роли `encounter` добавлено: пока встреча открывается, экземпляр видит `entity.created` своей встречи (по id встречи) и `entity.update.rejected` по `proposal_id` её создания. Тест на каждую из двух строк.

#### T-230 · R7a · Роль `encounter` (соло): решение обмена и пакет
I1b · **I** · developer#2 · M · todo *(ревизия 4: подволна I, было 1.10)*
**Нарезка (tech-lead#2, 2026-09-11; решение оркестратора после T-416).** После T-416 прежняя R7 получила пять новых поведений и три уточнения из ревью T-419 — это больше M. Задача разрезана по швам двойника T-419 на четыре задачи одной роли. Их файлы — `internal/swarm/roles/encounter*.go`, исполнитель один, задачи идут подряд:
- **T-230 R7a** — решение обмена и пакет;
- **T-421 R7b** — судьба пакета: повтор, откат участия, несколько пакетов в полёте;
- **T-422 R7c** — жизненный цикл после факта и откладывание действий;
- **T-423 R7d** — закрытие по чужому факту (`abandoned`, Б1).

Роль `encounter` готова, когда приняты все четыре. Прежняя редакция R7 — ниже, под заголовком «Прежняя редакция R7»: флажки там сняты, у каждого пункта названа задача, которая его теперь держит. Сверка встреч при восстановлении — **T-424** (вынесена из T-237).
Что: `roles/encounter.go` — Phase 1 синхронно по действию игрока (`player.attacked`, `player.flee_attempted`):
- `Resolve` атаки, бегства и ответа NPC (`npc_attack`) с `causeEventID` = id действия;
- `dice.rolled` ×k;
- `combat.decided` ×2 с `exchange{index, last}` (C-05 п. 7);
- один `entity.update.proposed{atomic, expected_version}` на действие.

Пакет несёт след в сущности встречи (C-05 п. 2): номер раунда — через `set`, участников — если участие изменилось. Трофей выдаётся один раз.

Правила роли:
- **Молчание.** На действие, которое встреча не принимает, агент молчит (C-05 п. 3): атака без живых целей, действие вне региона встречи, чужой мир. Случай «после конца встречи» — T-422.
- **`rest`.** Во встрече не обрабатывается — его отклоняет State.
- **`abandoned`.** Учитывается как `dead` в выборе целей (`NPCTarget`), в `expected[]`/`acted[]` и в `participation=active` (сведение 3).
- ~~**Окно действий** — `eventbus.Dedup.Seen`. Здесь запоминание до обработки — меньшее зло: повтор задвоил бы кости (ADR-027 п. 3). Окно — часть состояния экземпляра, которое сериализует снапшот T-236.~~ **Заменено (architect#2, 2026-09-11; design §14.1, §10 п. 14 решён).** Окно действий:
  - **Окно двухшаговое** — `eventbus.Dedup` в два шага:
    - `Has` — до обработки;
    - `Add` — после публикации последнего события ответа, при молчании (C-05 п. 3) или при постановке действия в очередь отложенных (T-422);
    - `Seen` в роли не используется.
  - **Ответ собирается один раз.** Броски, решения и пакет собираются до первой публикации и лежат в слоте экземпляра вместе с курсором публикаций. Повтор доставки допубликовывает слот, а не решает действие заново.
  - **Id всех событий ответа — из причины** (`WithCauseID`):
    - у `dice.rolled` — индекс броска;
    - у `combat.decided` — `exchange.index`;
    - у пакета — номер попытки.
  - **Окно и слот** — часть состояния экземпляра, их сериализует снапшот T-236.
  - **Отказ шины** (след, предел допубликования, выброс незаконченного ответа) — T-421.
- **Действие без `id`** не решается: `Error` в лог, без паники.
- **Применение пакета.** Пакет применён, когда пришёл `entity.updated` под его `proposal_id`; представление агента пополняется фактами.

**Не входит:**
- отказ State по пакету и всё, что из него следует, — T-421; здесь поведение на отказе тестами не закрепляется;
- поля закрытия встречи в пакете, `encounter.ended`, `agent.child_resolved`, откладывание — T-422;
- закрытие без действия игрока — T-423;
- копирование `gm_path` — T-240.

Зависит:
- T-223 — `Behaviour`/`Emitter`, фейковый раннер;
- T-234 — `WorldView`, источник `expected_version`;
- F-10/EPIC-002 — `FixedMechanics` → `Resolve`;
- **T-419** — двойник по C-05 v1.4, образец формы; принят до старта;
- T-417 — `Dedup`, C-01 v1.4.

От T-232 не зависит: в тестах агента поднимает сам тест. ~~агента поднимает `encounter.started` (КД §4.1), в тестах это событие подаёт тест, как у двойника~~ *(architect#2, 2026-09-11: в рое агента встречи поднимает решение GM региона открыть встречу, до публикации предложения — design §14.2, ADR-028; триггер `encounter.started` остаётся страховкой.)*

Ссылки: US-002, US-003, FR-020…024, FR-061, NFR-060, C-03, C-02 v1.2, C-04 v1.1, C-05 v1.4 п. 2, 3, 7, ADR-027 п. 3, КД §8.3, design §3.2 R7.
DoD: [ ] **(приёмка T-419, 2026-09-11)** тест по образцу зонда RF требует для действия без id: ноль публикаций, Error в логе, без паники — и для обычного удара, и для закрывающего, с настоящим Resolve (у двойника вторая половина RF держалась на ошибке FixedMechanics, а не на проверке).
- [ ] seed-детерминизм: два прогона с одним `causeEventID` дают одинаковые броски (NFR-060). *(прежний флажок 1)*
- [ ] провал `flee` → свободная атака NPC. *(прежний флажок 2, первая половина)*
- [ ] `expected_version` пакета берётся из `WorldView`. Тест: пакет несёт версию сущности из проекции на момент решения. *(прежний флажок 4, первая половина)*
- [ ] `exchange.last`: каждое `combat.decided` несёт `exchange{index, last}`, `index` — с 0 (решение по действию игрока — 0, ответ существа — 1), `last=true` ровно на одном решении обмена. Тест на три формы: «удар + ответ NPC», смертельный удар без ответа, провал `flee` со свободной атакой. *(прежний флажок 13)*
- [ ] **(приёмка T-214, ревью #1 Mi-4)** В валидную фикстуру `testdata/fixtures/events/combat.decided.v1.valid.json` добавлено `"exchange": {"index": 0, "last": false}` — образцовая форма издателя по C-05 v1.4 п. 7; тесты `./test/fixtures/...` зелёные.
- [ ] действие без `id` не роняет агента паникой и не решается: `Error` в лог, публикаций 0. Тест — по образцу `TestAnActionWithoutAnIDIsRefusedOutLoud` двойника T-419. *(прежний флажок 8, первая половина)*
- [ ] один пакет на действие, `atomic`; номер раунда в сущности встречи — через `set`; трофей выдаётся один раз; повторная доставка действия не даёт второго решения и второго пакета (окно `Dedup`). Тесты на каждое. *(«Что» прежней R7)*
- [ ] покинутый (`abandoned`) игрок — не цель NPC и не входит в `expected[]`, `acted[]`, `participation=active`. Тест. *(дополнение сведения 3, первая часть)*
- [ ] молчание на непринятое действие: атака без живых целей, действие вне региона встречи, действие в чужом мире — публикаций 0; `rest` во встрече не обрабатывается. Тест на каждый случай. *(«Что» прежней R7, C-05 п. 3)*
- [ ] **(T-425, 2026-09-11; C-05 v1.5, `api-contracts.md` §2.3.6 — исправлено)** `causation_id` пакета обмена — действие игрока, а не `combat.decided`. Тест: у `entity.update.proposed` обмена `causation_id` равен id действия. Для группы причина — `round.closed`, это T-246. На этом правиле стоит сверка T-424: по пакету она находит причину `encounter.ended`.
- [ ] **(architect#2, 2026-09-11; design §14.1 п. 1–3)** Id из причины и ответ, собранный один раз. Тесты:
  1. Два построения ответа на одно действие дают одинаковые id и одинаковые байты у всех `dice.rolled`, всех `combat.decided` и пакета. Разные индексы дают разные id.
  2. Шина отказывает на k-й публикации ответа, затем принимает. Повтор доставки публикует события начиная с k-го под теми же id. Пакет один, `Add` делается после последней публикации.
  3. Факт, пришедший между отказом и повтором, решение не меняет: повтор публикует слот.
  4. Повтор действия, на которое ответ уже есть (`Has` = true), ничего не публикует.

  `Seen` в роли не используется: проверка чтением кода на ревью или тестом по `go/ast`.

**Прежняя редакция R7 (до нарезки 2026-09-11).** Сохранена для истории. Флажки сняты; «→ T-NNN» — задача, которая держит пункт теперь.
Что (прежнее): `roles/encounter.go` — Phase 1 синхронно: `Resolve` атаки/бегства/`npc_attack`, `dice.rolled` ×k, `combat.decided` ×2, один `entity.update.proposed{atomic, expected_version}`, трофей один раз, `encounter.ended reason=npc_dead|players_out`, `agent.child_resolved`; `rest` во встрече не обрабатывается (отклоняет State). **→ T-230** (решение, броски, пакет, трофей, `rest`), **→ T-422** (`encounter.ended`, `agent.child_resolved`).
**→ T-230** (исключение покинутого из целей, `expected[]`/`acted[]`, `participation`) **/ → T-423** (закрытие встречи по `abandoned`) — **Дополнение (сведение 3, C-02 v1.2, C-04 v1.1; З-2; внесено tech-lead#1 от имени tech-lead#2)**: роль трактует `status = abandoned` **как `dead`** — покинутый игрок исключается из целей (`NPCTarget`), из `expected[]`/`acted[]` и из `participation=active`; gateway при `/forget` во встрече ничего дополнительно не публикует, роль сама видит `entity.updated status=abandoned` и завершает встречу `encounter.ended reason=players_out`, если игроков со `status=alive` не осталось. `narrative.output kind=death` по `abandoned` **не** издаётся.
**→ T-230** (решения обмена до пакета), **→ T-421** (выброс пакета), **→ T-422** (конец после факта, id из причины, откладывание), **→ T-423** (закрытие по `abandoned`) — **Дополнение (T-416, 2026-09-11; C-05 v1.4 п. 1в–1д, 5–7; ADR-026 п. 1, 3, 5, 7, 8; ADR-027).** Порядок публикаций меняется. Решения обмена (`dice.rolled` ×k, `combat.decided` ×2) идут до пакета, как прежде (C-03). **`encounter.ended` — только после факта** `entity.updated` сущности встречи со `state=resolved` под `proposal_id` закрывающего пакета. Событие конца строится до пакета: его id называет `closed_by_event_id`, id выводится из причины — `eventbus.WithCauseID`, `parts` — id встречи. Выброшенный закрывающий пакет (повторы исчерпаны или «мир ушёл дальше решения») — `encounter.ended` не публикуется, встреча продолжается. Действия по встрече, чей закрывающий пакет в полёте, откладываются до его решения. Закрытие по `abandoned` последнего игрока (дополнение сведения 3 выше) идёт тем же путём: пакет закрытия → факт → `encounter.ended reason=players_out`. Действующий образец формы — двойник после T-419.
Зависимости (прежние; теперь у каждой части свои): T-223, F-10/EPIC-002 (`FixedMechanics` → `Resolve`), **T-419** (двойники по C-05 v1.4 — образец формы, T-416), **T-417** (`WithCauseID`, C-01 v1.4). Ссылки: US-002, US-003, FR-020…024, FR-061, NFR-060, C-03, **`contracts.md` v0.4 C-02 v1.2, C-04 v1.1**, **C-05 v1.4, C-01 v1.4, ADR-026, ADR-027**, КД §8.3, design §3.2 R7.
DoD (прежний): **→ T-230** seed-детерминизм: два прогона с одним `causeEventID` дают одинаковые броски (NFR-060); **→ T-230 / T-422** провал `flee` → свободная атака NPC (T-230); смерть игрока → `players_out` (T-422); **→ T-423** **(сведение 3)** тест: `entity.updated status=abandoned` последнего игрока → `encounter.ended reason=players_out`, без `narrative.output kind=death`; **→ T-230 / T-421** `expected_version` берётся из `WorldView` (T-230), конфликт версии обрабатывается повтором (T-421). ~~**(приёмка T-219, 2026-09-11)** выброшенная или сданная попытка повтора либо откатывает участие (урон, последний ударивший) вместе с пакетом, либо C-05 явно относит участие к решению — вариант выбирает архитектор в ревизии C-05 п. 1в; тест на выбранный вариант.~~ **Заменено (T-416): архитектор выбрал откат — пункт «участие» ниже (→ T-421).** **→ T-422** **(T-419, 2026-09-11)** очередь отложенных действий, пока закрывающий пакет или создание встречи в полёте, ограничена — переполнение даёт громкий отказ, а не молчание; тест. **→ T-421** **(ревью T-419, 2026-09-11)** слот ожидаемого пакета у встречи не один: второе действие до факта первого пакета не перезаписывает ожидание первого, и откат п. 1г применяется к отказу именно того пакета, чей он (харнесс так не ходит, шлюз EPIC-004 может) — тест. **→ T-230 (действие) / T-423 (чужой факт)** **(ревью #2 T-419, 2026-09-11)** действие или чужой факт без `id` не роняет агента паникой `WithCauseID` и не закрывает бой молча: пропуск с записью в лог — по образцу исправления в двойнике T-419; тест.
- **→ T-422** **(T-416, 2026-09-11; C-05 v1.4 п. 5, ADR-026 п. 1, 3; ADR-027) Конец встречи после факта, id из причины.** `encounter.ended` публикуется только после `entity.updated` сущности встречи со `state=resolved` под `proposal_id` закрывающего пакета. Его `id` равен `closed_by_event_id` пакета и выведен через `WithCauseID` (`parts` — id встречи). Тесты: факт пришёл → событие с этим id; два построения из одной причины → один id; закрывающий пакет выброшен (исчерпаны повторы, «мир ушёл дальше») → события нет, встреча открыта и у агента, и в State, следующее действие решается от фактов.
- **→ T-422** **(T-416, 2026-09-11; C-05 v1.4 п. 5, ADR-026 п. 5) Откладывание.** Действие по встрече, чей закрывающий пакет в полёте, ждёт решения пакета. Пакет применён → молчание по п. 3; пакет выброшен → действие решается обычным путём. Ответа по представлению с неприменённым пакетом нет. Тест на обе ветки.
- **→ T-421** **(T-416, 2026-09-11; C-05 v1.4 п. 1в–1г, ADR-026 п. 3, 8) Участие откатывается вместе с пакетом, номер раунда — нет.** Выброшенная попытка (агент сдался или «мир ушёл дальше решения») откатывает из представления агента `damage_dealt`, `last_damager` и состояние участия вместе со здоровьем и статусом. Номер раунда соло уже опубликован в `combat.decided.round.seq` и не откатывается: следующий принятый пакет пишет его через `set`, не `inc`. Повтор, меняющий, кто жив, не предлагается — `Error` «мир ушёл дальше решения» и откат. Тест: после выброшенной попытки следующий пакет несёт `damage_dealt` и `last_damager` без неприменённого удара, выбор цели NPC (`NPCTarget`) от него не зависит, `round.seq` не повторяется.
- **→ T-423** **(T-416, 2026-09-11; C-05 v1.4 п. 6, ADR-026 п. 7 — вариант Б1) Закрытие по чужому факту.** NPC встречи стал терминальным по чужому факту, живых противников нет → агент предлагает пакет закрытия без трофея: `state=resolved`, `resolution=npc_dead`, участники выходят из боя. После факта — `encounter.ended {reason: npc_dead}` **без `killer`**, scope конверта — scope встречи (`eventbus.WithScope`), а не унаследованный из чужого факта. Нарратив на такой конец не обязателен. Тест с подложенным чужим `entity.updated status=dead` NPC.
- **→ T-230** **(T-416, 2026-09-11; C-05 v1.4 п. 7) `exchange.last`.** Каждое `combat.decided` несёт `exchange{index, last}`: `index` с 0 (решение по действию игрока — 0, ответ существа — 1), `last=true` ровно на одном решении обмена. Схему поле получает в T-419. Тест: обмен «удар + ответ NPC», смертельный удар без ответа, провал `flee` со свободной атакой.

#### T-421 · R7b · Роль `encounter`: судьба пакета — повтор, откат участия, несколько пакетов в полёте
I1b · **J** · developer#2 · M · todo *(ревизия 4: подволна J, было 1.11)*
*(нарезка T-230, tech-lead#2, 2026-09-11; номер из общего счётчика проекта)*
Что: `roles/encounter_package.go` — учёт пакетов агента в полёте по `proposal_id`. Пакетов может быть несколько сразу: второе действие приходит до факта первого пакета. Правила по каждому пакету:
- **Факт.** Пришёл `entity.updated` под его `proposal_id` → пакет применён. Повтор гаснет с первым фактом (C-05 п. 1б).
- **Конфликт версии.** `entity.update.rejected {reason: version_conflict}` → агент обновляет представление по фактам и предлагает пакет заново под **тем же** `proposal_id`. Кости не перебрасываются, `dice.rolled`/`combat.decided` повторно не публикуются, попыток всего три, как у двойника (C-05 п. 1, 1а). Отказы проходят своё окно `eventbus.Dedup` по `event.id`, поэтому дубль отказа второй повтор не запускает (п. 1б). Окно — часть состояния экземпляра, его сериализует снапшот T-236 (C-14 v1.2 (а)).
- **«Мир ушёл дальше».** Пересчитанный пакет меняет, кто жив, → агент его не предлагает: `Error` «мир ушёл дальше решения» и выброс (п. 1в).
- **Отказ по другой причине** (`unknown_entity`, `invalid_op`, `dead_entity`, `duplicate_entity`, в настоящем State ещё `level_violation`) → выброс без повтора и `Error`, отложенные действия решаются. **(T-425, 2026-09-11)** Это теперь текст контракта — C-05 v1.5 п. 1е, ADR-026 «Уточнение исполнения» п. 2. Прежде это было отступлением двойника T-419, которое принял оркестратор.
- **Выброс** — это откат попытки из представления агента: здоровье, статус, `damage_dealt`, `last_damager`, состояние участия (п. 1г). Номер раунда не откатывается: следующий принятый пакет пишет его через `set`.

Итог по пакету — «применён» или «выброшен» — роль отдаёт внутренним вызовом, на нём строится T-422.
Зависит: T-230, T-419 (образец), T-417. Ссылки: C-05 v1.3–v1.4 п. 1, 1а–1г, ADR-026 п. 3, 8, ADR-013, C-14 v1.2 (а), КД §8.3.
DoD:
- [ ] Конфликт версии обрабатывается повтором: тот же `proposal_id`, новых `dice.rolled`/`combat.decided` нет, попыток не больше трёх. Тесты: повтор после чужого факта проходит; дубль отказа не запускает второй повтор (окно `Dedup`); отказ после факта своего пакета повтора не даёт. *(прежний флажок 4, вторая половина; C-05 п. 1а–1б)*
- [ ] Участие откатывается вместе с пакетом, номер раунда — нет. Повтор, меняющий, кто жив, не предлагается: `Error` «мир ушёл дальше решения» и откат. Тест: после выброшенной попытки следующий пакет несёт `damage_dealt` и `last_damager` без неприменённого удара, выбор цели NPC (`NPCTarget`) от него не зависит, `round.seq` не повторяется. *(прежний флажок 11)*
- [ ] Слот ожидаемого пакета не один: второе действие до факта первого пакета не перезаписывает ожидание первого. Откат п. 1г применяется к отказу именно того пакета, чей он. Тест: два пакета в полёте, отказ по каждому по очереди. *(прежний флажок 7)*
- [ ] **(T-425, 2026-09-11; C-05 v1.5 п. 1е, ADR-026 «Уточнение исполнения» п. 2)** Любая причина отказа не по гонке — `unknown_entity`, `invalid_op`, `dead_entity`, `duplicate_entity`, в настоящем State ещё `level_violation` — даёт один и тот же исход: повтора нет, пакет выброшен с откатом попытки вместе с участием (п. 1г), `Error` в лог. Табличный тест на каждую из пяти причин. Для закрывающего пакета «`encounter.ended` нет, отложенные действия решены» проверяет T-422. *(Уточнение нарезки по образцу T-419; T-425 сделал его текстом контракта.)*
- [ ] ~~**(из T-415, 2026-09-11)** Отказ шины при публикации решения обмена или пакета оставляет след по правилу `Emitter` T-223, и повтор этот след не стирает. Пока действует ADR-027 п. 3 (`Seen` до обработки), повтор шины отвечает `nil`, и письма в `dead_letters` нет. Тест закрепляет «след остаётся после повтора» по образцу `TestARefusedDecisionLeavesATrace` двойника. Если architect#2 по §10 п. 14 выберет `Has`/`Add`, пункт заменяется на «постоянный отказ → `dead_letters`, повтор публикует те же события с теми же id».~~ **Заменено (architect#2, 2026-09-11; design §14.1 п. 3–5; §10 п. 14 решён: `Has`/`Add`).** Отказ шины при публикации решения обмена или пакета:
  - **След** — по правилу `Emitter` T-223. Ошибка возвращается доставке, слот ответа остаётся.
  - **Любая следующая доставка экземпляру** сначала допубликовывает слот с курсора, теми же событиями с теми же id. Факты применяются к представлению раньше.
  - **Предел допубликования.** У слота есть счётчик неудачных публикаций; предел — именованная константа роли, 4. На неудаче, которой он достигнут, ответ выбрасывается как пакет по п. 1г: откат всего, кроме номера раунда; `Add(id действия)`; `Error` «ответ не допубликован»; решение отложенных действий. Доставка уходит в `dead_letters`.
  - **Окно отказов (п. 1б) тоже двухшаговое:** `Add` — после публикации повторного предложения или после решения выбросить пакет.

  Тесты:
  1. Разовый отказ: повтор публикует те же id, `dead_letters` пуст, след в логе остаётся.
  2. Постоянный отказ: действие в `dead_letters`, ответ выброшен с откатом, следующее действие решается от фактов. Агент жив: отказы шины счётчик ошибок роли T-228 не считает *(T-431, 2026-09-11; приёмка T-431 TL-1)*.
  3. Отказ публикации повторного предложения: повтор доставки отказа публикует его под тем же id.
  4. Отмена контекста посреди публикации: `Info`, счётчик не растёт, `Add` нет.

#### T-422 · R7c · Роль `encounter`: жизненный цикл после факта и откладывание действий
I1b · **K** · developer#2 · M · todo *(ревизия 4: подволна K, было 1.12)*
*(нарезка T-230, tech-lead#2, 2026-09-11; номер из общего счётчика проекта)*
Что: `roles/encounter_lifecycle.go` — у агента три фазы встречи: «идёт», «закрывается», «окончена». Начало встречи агент не объявляет: `encounter.started` публикует GM региона после `entity.created` (T-232). ~~а агент поднимается по этому событию~~ **(architect#2, 2026-09-11; design §14.2, ADR-028)** Агент поднимается раньше — решением GM региона открыть встречу, до публикации предложения создания. Поэтому у агента есть ещё одна, первая, фаза — «открывается»: встреча создаётся, NPC занят, действия откладываются.
- **Закрывающий пакет.** Встречу закрывает смертельный удар по последнему NPC (`npc_dead`, `killer`) или смерть либо бегство последнего живого игрока (`players_out`). Пакет обмена тогда дополнительно несёт для сущности встречи `state=resolved`, `resolution` и `closed_by_event_id`. Это id события `encounter.ended`, построенного заранее: `Derive` от причины с `eventbus.WithCauseID(<id встречи>)` (ADR-027 п. 2). **(T-425, 2026-09-11; ADR-027 «Уточнение исполнения» п. 3)** Причина — действие игрока, на которое отвечает обмен. Не `combat.decided`: решений на обмен два, а обмен один. Для группы причина — `round.closed`, это T-246.
- **Публикация конца.** `encounter.ended` уходит только после `entity.updated` сущности встречи со `state=resolved` под `proposal_id` закрывающего пакета. Следом — `agent.child_resolved` и остановка агента.
- **Выброс.** Закрывающий пакет выброшен (итог T-421: сдался, мир ушёл дальше или отказ не по гонке по п. 1е) → конца нет, встреча продолжается и у агента, и в State, отложенные действия решаются.
- **Откладывание.** Действия по встрече ждут в ограниченной очереди, пока закрывающий пакет в полёте или пока сущности встречи ещё нет в `WorldView` агента. Второе нужно потому, что `encounter.started` может прийти раньше `entity.created`: порядок между топиками не гарантирован (C-05 п. 4). Как только пакет решён или сущность появилась, действия решаются в порядке прихода. Пакет применён — молчание по п. 3. Пакет выброшен — обычный путь.
- **Лимит очереди** — именованная константа роли, стартовое значение 16, без env. При переполнении действие не решается, а в лог идёт `Error` с id действия и размером очереди.

**(T-425, 2026-09-11; C-05 v1.5 п. 4, ADR-026 «Уточнение исполнения» п. 3.) Кто держит очередь на время создания встречи.** Держит агент встречи, а не GM региона. Причины: очередь, её лимит и громкий отказ уже здесь; отложенное действие решает тот же агент, который решает действия; GM региона `player.attacked` не видит (КД §5.2). Поэтому в T-232 очереди нет.
**Решено architect#2 (2026-09-11) — design §14.2, ADR-028, T-427.** Выбран первый вариант, но подъём идёт внутри процесса и до публикации предложения, а не по предложению, пришедшему из шины: через шину окно только сужается. Второй вариант отклонён — это вторая очередь в Router, срок ожидания и расхождение в replay. Исходный вопрос: ~~**Открытый вопрос architect#2 — ответ нужен до старта этой задачи.**~~ Агента поднимает `encounter.started` (КД §4.1). Действие, которое пришло в Router раньше `encounter.started`, не получает никто: агента ещё нет, а GM региона на действия не подписан. Варианты:
- поднимать агента по предложению создания встречи от GM региона — правка КД §4.1/§5.1 и триггера блупринта `encounter-wolf`;
- держать такое действие в Router до `agent.spawned` своего scope — правка `router.go`, отдельная задача потока рантайма.

Номер под второй вариант не взят: лимит нарезки — четыре номера.

Зависит: T-421 (итог пакета), T-230, T-417 (`WithCauseID`), T-419 (образец); ~~до старта — ответ architect#2 на открытый вопрос выше~~ ответ дан (architect#2, 2026-09-11; design §14.2). В тестах агента в фазе «открывается» поднимает тест; `Spawner` (T-223/T-225) для этой задачи не нужен. Ссылки: C-05 v1.4–v1.5 п. 1е, 3–5, ADR-026 п. 1, 3, 4, 5 и «Уточнение исполнения» п. 2–5, ADR-027 п. 1–2 и «Уточнение исполнения» п. 3, US-002, US-003, КД §6, §8.3.
DoD:
- [ ] **(T-425, 2026-09-11; C-05 v1.5 п. 1е, 5; ADR-027 «Уточнение исполнения» п. 3)** Причина `encounter.ended` по обмену — действие игрока, а не `combat.decided`. Тест: id события равен `closed_by_event_id` пакета и одинаков при двух построениях. Закрывающий пакет, выброшенный отказом не по гонке (п. 1е), `encounter.ended` не даёт, а отложенные действия решены. Тест.
- [ ] Конец встречи — после факта, id — из причины. `encounter.ended` публикуется только после `entity.updated` сущности встречи со `state=resolved` под `proposal_id` закрывающего пакета. Его `id` равен `closed_by_event_id` пакета и выведен через `WithCauseID` (`parts` — id встречи). Тесты: факт пришёл → событие с этим id; два построения из одной причины → один id; закрывающий пакет выброшен (исчерпаны повторы, «мир ушёл дальше») → события нет, встреча открыта и у агента, и в State, следующее действие решается от фактов. *(прежний флажок 9)*
- [ ] Откладывание: действие по встрече, чей закрывающий пакет в полёте, ждёт решения пакета. Пакет применён → молчание по п. 3. Пакет выброшен → действие решается обычным путём. Ответа по представлению с неприменённым пакетом нет. Тест на обе ветки. *(прежний флажок 10)*
- [ ] Очередь отложенных действий ограничена — и пока закрывающий пакет в полёте, и пока сущности встречи нет в `WorldView`. Переполнение даёт громкий отказ (`Error`), а не молчание. Тесты: переполнение; удар сразу за `encounter.started`, до `entity.created`, не теряется и решается, когда пришёл факт. *(прежний флажок 6. «Создание встречи в полёте» у настоящего агента читается как «сущности встречи ещё нет в `WorldView`». T-425 п. 5: очередь держит агент встречи; окно до подъёма агента — ~~открытый вопрос выше~~ решено, строка ниже.)*
- [ ] **(architect#2, 2026-09-11; design §14.2 п. 2, ADR-028)** Фаза «открывается». Агент, поднятый до `entity.created`, откладывает действия своего scope.
  - Пришёл `entity.created` — отложенные действия решаются по порядку.
  - Пришёл `entity.update.rejected` по `proposal_id` создания — отложенные действия получают молчание (C-05 п. 3–4). Агент останавливается с `agent.stopped {reason: error}`, без `agent.child_resolved`.

  Тесты: удар, пришедший до `encounter.started` и до `entity.created`, не теряется; при отказе создания по отложенным действиям публикаций 0 и есть `agent.stopped reason=error`.
- [ ] **(architect#2, 2026-09-11; design §14.1 п. 3)** Действие, поставленное в очередь, сразу `Add`-ится в окно: очередь — состояние экземпляра в снапшоте (T-236). Тест: дубль действия, пока оно в очереди, второй раз в очередь не встаёт. Ответ на отложенное действие собирается и публикуется по тем же правилам слота, что в T-230/T-421.
- [ ] Смерть последнего живого игрока → `encounter.ended reason=players_out` тем же путём: закрывающий пакет → факт → событие. Тест. *(прежний флажок 2, вторая половина)*
- [ ] После `encounter.ended` — `agent.child_resolved`, агент остановлен. Действие после конца встречи получает молчание (C-05 п. 3). Тест. *(«Что» прежней R7)*

#### T-423 · R7d · Роль `encounter`: закрытие по чужому факту (`abandoned`, Б1)
I1b · **L** · developer#2 · M · todo *(ревизия 4: подволна L, было 1.13)*
*(нарезка T-230, tech-lead#2, 2026-09-11; номер из общего счётчика проекта. Размер при нарезке — S, поднят до M после правок T-425: два пути закрытия с проверкой id и scope на каждом.)*
Что: `roles/encounter_foreign.go` — конец встречи без действия игрока; причина конца — чужой факт `entity.updated`. Агент предлагает закрытие, когда у встречи нет своего пакета в полёте. Дальше путь тот же, что в T-422: пакет закрытия, факт, `encounter.ended`; отложенные действия ждут решения. **(T-425, 2026-09-11; ADR-027 «Уточнение исполнения» п. 3, C-05 v1.5 п. 5)** На обоих путях ниже причина `WithCauseID` у `encounter.ended` и основа `proposal_id` — сам чужой факт. Scope конверта у пакета и у события — scope встречи (`eventbus.WithScope`), а не унаследованный из факта.
- **Покинутый игрок.** `status=abandoned` последнего живого игрока (`/forget`, C-04 v1.1) → пакет закрытия, участники выходят из боя → факт → `encounter.ended reason=players_out`. Gateway ничего дополнительно не публикует (сведение 3).
- **Б1** (C-05 п. 6, ADR-026 п. 7). NPC встречи стал терминальным по чужому факту, и живых противников нет. Агент предлагает пакет закрытия без трофея, с `cause=resolve`. **(T-425; C-05 v1.5 п. 6, ADR-026 «Уточнение исполнения» п. 1)** Пакет трогает **только** сущность встречи: `state=resolved`, `resolution=npc_dead`, `closed_by_event_id`, а участники выходят из боя через её `participants[]`. Изменений игрока и NPC в пакете нет. После факта — `encounter.ended {reason: npc_dead}` **без `killer`**. Закрытие по одному и тому же факту предлагается один раз. Если State его отверг — `Error` один раз, отложенные действия решаются, бой не висит (образец T-419, Mi-1 ревью #1; T-425 подтвердил, C-05 v1.5 п. 1е).
- **Факт без `id`.** Чужой факт без `id` не становится причиной закрытия: `Error` в лог, без паники `WithCauseID`.

Зависит: T-422, T-417, T-419 (образец). Ссылки: C-04 v1.1, C-02 v1.2, C-05 v1.4–v1.5 п. 1е, 5–6, ADR-026 п. 7 и «Уточнение исполнения» п. 1, ADR-027 и «Уточнение исполнения» п. 3, `data-model.md` §3.7.
DoD:
- [ ] Тест: `entity.updated status=abandoned` последнего игрока → `encounter.ended reason=players_out`, без `narrative.output kind=death`. *(прежний флажок 3)*
- [ ] Закрытие по чужому факту (Б1). Тест с подложенным чужим `entity.updated status=dead` NPC: пакет без трофея с `cause=resolve`, после факта — `encounter.ended {reason: npc_dead}` без `killer`, scope конверта — scope встречи; нарратив на такой конец не обязателен. Отдельный тест: State отверг закрытие → `Error` один раз, отложенное действие решено ровно раз, закрытие не предлагается снова. *(прежний флажок 12; второй тест — образец T-419, Mi-1 ревью #1, подтверждён T-425)*
- [ ] **(T-425, 2026-09-11; C-05 v1.5 п. 5–6; ADR-026 и ADR-027 «Уточнение исполнения»)** В пакете Б1 изменения только у сущности встречи (`state`, `resolution`, `closed_by_event_id`, `participants[]`), изменений игрока и NPC нет. На обоих путях (Б1 и `abandoned`) причина `encounter.ended` — сам чужой факт, scope конверта — scope встречи. Тесты: состав `changes` пакета Б1; на каждом пути id события равен `closed_by_event_id` и одинаков при двух построениях; scope события — scope встречи, а не scope факта.
- [ ] Чужой факт без `id` не роняет агента паникой `WithCauseID` и не закрывает бой молча: пропуск с `Error`. Тест — по образцу `TestAFactWithoutAnIDDoesNotCloseTheFight` двойника. *(прежний флажок 8, вторая половина)*

#### T-231 · R6a · Роль `global-gm` (тик мира)
I1b · **J** · developer#3 · M · todo *(ревизия 4: подволна J, было 1.12)* · *(developer#2 → developer#3 при нарезке T-230: поток ролей developer#2 занят частями T-230)*
Что: `roles/global_gm.go` + `roles/triggers.go` (часть tick): tick-фаза `basic` (LLM по `tick-global.json`) и `rule-only` (выбор из `background_events[]` по весам через `mechanics.NewRNG(Seed(tick.id, 0))`); публикация `world.weather_changed|time_advanced|event_occurred` + `entity.update.proposed{cause: tick}` c `actor_kind=system`, `correlation_id = tick.fired.id`.
Зависит: T-223 (интерфейсы `Behaviour`/`TickRunner`), T-214 (схема `tick.fired`), T-213; интеграция со Scheduler (T-227) проверяется e2e T-243. Ссылки: US-036, US-037, FR-124…128, NFR-007, КД §8.1 п. 3, design §3.2 R6.
DoD: [ ] `rule-only` детерминирован по `tick.id` (два прогона — одинаковый выбор); [ ] при `invalid` от стража тик переходит в `rule-only`; [ ] в тике не меняются HP/позиция/инвентарь игроков (FR-128) — негативный тест.

#### T-232 · R6b · Роль `region-gm` (тик региона, встречи, респаун)
I1b · **K** · developer#3 · M · todo *(ревизия 4: подволна K, было 1.13)* · *(developer#2 → developer#3 при нарезке T-230: поток ролей developer#2 занят частями T-230)*
Что: `roles/region_gm.go` — tick `idle/active`; обнаружение встреч по правилам (`encounter.detect_on: tick`, `chance` через `Rules.Roll`) → `entity.create.proposed{encounter}` + `encounter.started` с `round{}` из блупринта встречи; респаун по `npc_table` (`respawn_ttl`, `died_at`, новый `entity_id`, `npc.spawned` + `entity.create.proposed`); события `region.event_occurred`/`npc.moved`/`npc.spawned`; старт при отсутствии сущности региона в `WorldView` с признаком `region_missing`.
**Дополнение (T-416, 2026-09-11; C-05 v1.4 п. 4, 6; ADR-026 п. 1–2, 6).** `encounter.started` публикуется **только после** факта `entity.created` встречи, а не сразу за предложением. Правило «один NPC — одна активная встреча» (`data-model.md` §3.7) — требование к GM региона как единственному открывающему в регионе.
Зависит: T-231, **T-417** (`WithCauseID`, C-01 v1.4). Ссылки: US-036, US-002, US-015, FR-124/128, C-05 v1.1, **C-05 v1.4 п. 4, 6, ADR-026, ADR-027**, `data-model.md` §3.7, КД §4.1, §8.3, §17, design §3.2 R6, решение §13 п. 4.
DoD: [ ] тест согласованности: каждая запись `npc_table` имеет `stats_ref` в `rules/dark-forest.yaml` и `kind` из фикстур; регион `scope_binding.id` есть в `testdata/fixtures/region.json`; [ ] убитый NPC не возрождается в течение `respawn_ttl`, после — `npc.spawned` с новым `entity_id` (US-036); [ ] встреча порождается не позднее одного активного интервала после входа игрока (FR-124). [ ] **(T-416, 2026-09-11; C-05 v1.4 п. 4, ADR-026 п. 1–2 — вариант А1)** `encounter.started` — только после `entity.created` встречи. Id события — из причины (`eventbus.WithCauseID`, `parts` — id встречи). Создание отвергнуто (`entity.update.rejected` по `proposal_id`) → `encounter.started` не публикуется, отказ пишется в лог `Error`. Тест на оба исхода. [ ] **(T-416, 2026-09-11; C-05 v1.4 п. 6, ADR-026 п. 6)** «Один NPC — одна активная встреча»: GM региона не открывает встречу с NPC, уже участвующим в активной встрече. Тест: два игрока входят в регион с одним волком → одна встреча, второе создание не предлагается. [ ] **(T-425, 2026-09-11; C-05 v1.5 п. 4, ADR-027 «Уточнение исполнения» п. 3)** Причина `encounter.started` для `WithCauseID` — событие, по которому GM открывает встречу: вход игрока или `tick.fired`. Не `entity.created`: id нужен до факта, его пишет `opened_by_event_id` в самом предложении. Тест: id события равен `opened_by_event_id` в предложении создания — и для входа, и для `tick.fired`. [ ] **(T-425; C-05 v1.5 п. 6, ADR-026 «Уточнение исполнения» п. 4)** NPC занят с предложения создания до факта закрытия или отказа в создании. Тест: два входа до `entity.created` → одно предложение создания; после отказа в создании NPC снова свободен. *(Очередь действий на время создания держит агент встречи — T-422, нарезка T-230. В этой задаче её нет, тест «удар сразу за входом» — там же. Окно до подъёма агента — ~~открытый вопрос architect#2 в T-422~~ решено: design §14.2, ADR-028; строка DoD ниже.)* [ ] **(architect#2, 2026-09-11; design §14.2 п. 1, 5–6, ADR-028)** Подъём агента встречи. Решив открыть встречу, GM региона сначала поднимает агента встречи (`Spawner.SpawnChild`, блупринт — `encounter.child_blueprint`) и только потом публикует `entity.create.proposed`. Тесты: в журнале `agent.spawned` агента встречи стоит раньше предложения создания; повтор обработки открытия после отказа публикации предложения второго агента не поднимает. Кроме того, GM региона не открывает встречу игроку, у которого жив агент встречи в любой фазе (`Spawner.Alive`). Тест: встреча кончилась, сразу следующий тик — второе открытие не предлагается, пока прежний агент не остановлен.
- [ ] **(T-431, 2026-09-11; ADR-028 «Уточнение исполнения» п. 1, design §14.5 п. 4)** Открытие встречи собирается один раз.
  - Id встречи и `proposal_id` создания выводятся из события открытия **и scope игрока или группы**. Id события предложения создания — `WithCauseID(<тип scope>, <id scope>)` от события открытия, id встречи — `encounter-<id этого события>`, `proposal_id` — из него же (приёмка T-431, TL-2: формула `encounter-<id события>` даёт один id двум игрокам одного тика).
  - Выбор NPC и тело предложения записываются в запись «открытие в полёте» до `SpawnChild`.
  - Повтор (отказ шины, повторная доставка, рестарт) публикует эту запись и заново не выбирает.

  Тесты:
  - между отказом публикации предложения и повтором приходит факт о NPC → повтор предлагает ту же встречу с тем же NPC, агент встречи один;
  - один `tick.fired` открывает встречи двум игрокам с разными NPC → разные id встречи и `proposal_id`, оба создания приняты State, два `encounter.started` с разными id.

#### T-233 · R8 · Роль `personal-gm` и таблица триггеров нарратива
I1b · **I** · developer#3 · M · todo *(ревизия 4: подволна I, было 1.10)* · *(1.11 developer#2 → 1.10 developer#3 при нарезке T-230: поток ролей developer#2 занят частями T-230, а у developer#3 подволна 1.10 свободна — T-236 зависит от T-225 из 1.10)*
Что: `roles/personal_gm.go` + завершение `roles/triggers.go` по таблице КД §8.2 (`entry` c `absence`, `turn` по последнему событию цикла с тем же `correlation_id`, `world_event`, `death`); Phase 2 через `Gateway`; `narrative.output` со всеми полями C-05 (`recipients`, `based_on[]`, `background_refs[]` ⊂ контекста, `absence{}`, `filter{}`, `laws_version`, `narrative_event_id`); в группе — `rule-only` (LLM только `entry` при `absence`).
**(ревизия 4, решение 1c)** Роль зовёт Phase 2 через `Phase2Runner` (T-223): `llm.Call` из корневого пакета, имя схемы ответа — **строкой** (`narrative`). `internal/llm/parser` и `providers/*` роль не импортирует. Проверка `background_refs ⊂ контекста` делается через `guardian$`, это разрешённая связь. `filter_version` в `narrative.output.filter{}` берётся из результата шлюза.
**Дополнение (T-416, 2026-09-11; C-05 v1.4 п. 7–8).** `turn` закрывается по `combat.decided.exchange.last`. Прежнее «по последнему событию цикла с тем же `correlation_id`» заменено: вывод по полям решения остаётся только для издателя без поля `exchange`. Смерть персонажа из пакета боя публикуется **после** текста хода.
Зависит: T-223 (интерфейсы `Behaviour`/`Phase2Runner`), T-235, T-213, **T-419** (поле `combat.decided.exchange` в схеме, T-416); интеграция с Pipeline (T-228) проверяется e2e T-242. Ссылки: US-002, US-004, US-005, US-036, FR-013/015/127, NFR-002/006, C-05, **C-05 v1.4 п. 7–8**, КД §8.2, design §3.2 R8.
DoD: [ ] по кейсу на каждую строку таблицы триггеров; [ ] ровно один вызов LLM на ход соло (счётчик `FakeProvider`); [ ] `world.*` вне хода не порождает вызова, а попадает в контекст следующего нарратива; [ ] смерть игрока → `kind: death`, дальше агент живёт до TTL. [ ] **(T-416, 2026-09-11; C-05 v1.4 п. 8)** Смерть из пакета боя (в `entity.updated.cause` — причина боя или бегства) публикуется после `narrative.output kind=turn` с той же `correlation_id`. Ожидание ограничено таймаутом фазы нарратива, по истечении — смерть без хода с записью `Warn`. Смерть не из пакета боя публикуется сразу. Тест с задержкой чтения `game_events` 20 мс: в `narrative_output` `death` идёт после `turn` в каждом бою прогона; ветка таймаута — отдельный тест. [ ] **(T-416, 2026-09-11; C-05 v1.4 п. 7)** Ход закрывается по `exchange.last=true`. Тест: обмен из двух решений — один `turn` после второго; пропущенный `index` виден (лог), а не «незакрытый ход». [ ] **(нарезка T-230, 2026-09-11; C-02 v1.2, сведение 3)** `narrative.output kind=death` по `entity.updated status=abandoned` не публикуется — это не смерть; тест (агент встречи проверяет свою сторону в T-423). [ ] **(ревизия 4)** `narrative.output.filter{filter_version}` равен версии из результата шлюза (тест на `fake`-раннере); production-код роли не импортирует `internal/llm/parser` и `internal/llm/providers/...`.

### Поток «контекст, снапшот, admin» (developer#3)

#### T-234 · R9a · `WorldView` и `journalContext`
I1b · **E** · developer#3 · M · todo *(ревизия 4: подволна E, было 1.8)*
Что: `context/worldview.go` — проекция сущностей (старт из `snapshots-{world}/state/latest.json`, далее `entity.created/updated` строго по `version`); `context/journal.go` — `EventWindow` (кольцо N=50 на scope), `BackgroundIndex` (world/region, TTL 30 дн.), `Presence`, `Absence(player, region, now)` (пауза ≥ 30 мин, лимит 20 событий + счётчик).
Зависит: T-222. Ссылки: US-005, US-036, FR-127, C-02, C-09 (штатная деградация), C-14, КД §11.1–11.2, design §3.2 R9.
DoD: [ ] `Absence` строится по паузе ≥ 30 мин и по первому входу после `left_region`; [ ] отставание версии сущности не ломает проекцию (лог, без публикации `analytics.consistency.violated`); [ ] структуры сериализуемы для снапшота (готово к T-236).

#### T-235 · R9b · Сборка контекста в секции промпта, `MemoryClient` + `NopMemory`
I1b · **G** · developer#3 · S · todo *(ревизия 4: подволна G, было 1.9)*
Что: `context/builder.go` — `AgentContext → prompt.Sections` (state, events, absence, canon, laws, player_text); `context/memory.go` — интерфейс `MemoryClient{ScopeContext, AbsenceSummary, Trace}` и `NopMemory` (HTTP-реализация — I2, T-248).
Зависит: T-234, T-218. Ссылки: US-005, US-018, FR-127, C-09, КД §11.3–11.4, design §3.2 R9.
**(ревизия 4, решение 1c)** Сборщик возвращает `prompt.Sections` из `internal/llm/prompt` — рою разрешена строка `depguard` `internal/llm/prompt$` (T-445). До слияния T-445 в эпик линтер на этом импорте красный, поэтому задача стартует после `sync` эпика с `develop`, где T-445 уже есть.
DoD: [ ] `background_refs`, попадающие в промпт, доступны стражу как `AbsenceEventIDs` (стыковка с T-217); [ ] `NopMemory` используется в `mode=replay` (детерминизм `prompt_hash`); [ ] секции соответствуют порядку КД §11.4.

#### T-236 · R10a · Снапшот роя
I1b · **H** · developer#3 · M · todo *(ревизия 4: подволна H, было 1.11)* · *(1.10 → 1.11 при нарезке T-230: зависит от T-225, а тот стоит в 1.10)*
Что: `snapshot.go` — `SwarmSnapshot` (КД §4.3: экземпляры, курсоры, окна журнала, `BackgroundIndex`, LRU дедупа, окно бюджета, `laws_version`), запись в `snapshots-{world}/swarm/{ts}-{seq}.json` + `latest.json`-указатель, ротация `K=5`, публикация `snapshot.created component=swarm`, триггеры (каждые `MV_SWARM_SNAPSHOT_EVERY_EVENTS=200`, при остановке).
Зависит: T-225, T-234, **T-417** (`Dedup.Has`/`Add` и сериализация окон, C-01 v1.4; T-416). Ссылки: US-011, FR-031…033/087, NFR-010…012, C-14 v1.1, **C-14 v1.2, ADR-027 п. 4**, ADR-011, КД §4.3, design §3.2 R10.
DoD: [ ] round-trip: снапшот → восстановление → `state_hash` совпадает; сортировка map при сериализации (NFR-061); [ ] `latest.json` пишется после успешного PUT объекта; [ ] повреждённый снапшот → отказ старта с сообщением в `/health`, не «тихий пустой рой». [ ] **(T-416, 2026-09-11; C-14 v1.2 (а), ADR-027 п. 4)** Окна дедупликации входят в `SwarmSnapshot` (`eventbus.Dedup`: `IDs` → снапшот, `Restore` ← снапшот): окно «уже ответил» нарратора (T-229), окно агента встречи (T-230) и окно отказов агента встречи (C-05 п. 1б). Окно без сериализации у контекста роя — дефект. Тест: после восстановления повтор уже отвеченного события не даёт ни второго текста, ни второго пакета. [ ] **(architect#2, 2026-09-11; design §14.1 п. 2, 6)** В снапшот агента встречи входят слот незаконченного ответа (собранные события, курсор, счётчик) и очередь отложенных действий. Тест: остановка посреди публикации ответа → снапшот → восстановление → повтор действия допубликовывает ответ теми же байтами; уже опубликованная часть не повторяется под новыми id.
- [ ] **(T-431, 2026-09-11; ADR-028 «Уточнение исполнения» п. 1, design §14.5 п. 4)** В `SwarmSnapshot` входит запись GM региона «открытие в полёте»: id встречи, `proposal_id`, NPC, тело предложения. Тест: остановка между `SpawnChild` и `entity.created` → после восстановления повтор публикует то же предложение.
- [ ] **(T-431, 2026-09-11; design §14.5 п. 1, строки T-227)** В `SwarmSnapshot` вместе с окном бюджета входят id `llm.output`, уже учтённые в окне, и последний закрытый (выполненный или прерванный) `tick_seq` каждого timer-агента. Тест: снапшот → восстановление → повтор уже учтённого `llm.output` не меняет `window_calls`, а повтор закрытого `tick.fired` тик не выполняет.

#### T-237 · R10b · `swarm.Context`: подписки, фаза догона, режимы live/replay
I1b · **K** · developer#1 · M · todo *(ревизия 4: подволна K, было 1.14, developer#3)* · *(подволна 1.11 → 1.14 при нарезке T-230, 2026-09-11: T-237 зависит от T-228, а тот стоит в 1.13)*
Что: `swarm.go`/`runtime.go` — `Context{Start, Stop, Health}` для `shared/runtime`; подписки consumer-group `core.swarm` с `MV_BUS_VALIDATE_ON_READ`; фаза догона через `Journal.ReadRange…End` без эмиссии (проекции и индексы восстанавливаются, события не публикуются), восстановление окна бюджета и `strain` из `llm.output`/`llm.output.rejected`; старт догона после `analytics.replay.completed mode=recovery` от State (WorldView читает `state/latest.json` параллельно — решение design §13 п. 7).
**Изменение (сведение 3, C-14 уточнение v0.4, TL2-6; внесено tech-lead#1 от имени tech-lead#2)**: вариант «не блокировать старт» **заменён** на «ждать с таймаутом». Контекст ждёт `analytics.replay.completed {mode: recovery}` не дольше `MV_SWARM_REPLAY_WAIT` (по умолчанию `120s`, = RTO); по истечении — стартует и отмечает `/health degraded {state_replay: missing}` (эта же ветка нужна в production при падении `state`, а не только с заглушкой). Заглушка `testkit/state.FakeState` v0 **публикует** сигнал при старте (EPIC-001 T-017), поэтому в норме таймаут не срабатывает. Переменная объявляется через `shared/env` и попадает в `.env.example` (запрос devops). Замечание §10 п. 6 закрыто.
**(ревизия 4; C-01 v1.7, ADR-023 «Уточнение исполнения» п. 1)** `Close` шины прерывает только выборку и коммит транспорта, но не контекст обработчика. Остановку обработчиков роя ограничивает сам контекст: `Stop` отменяет контекст подписок **до** того, как процесс закроет шину, и укладывается в общий срок `runtime.StopTimeout` (15 с). Ссылку «C-01 v1.1» ниже читать как C-01 v1.7. Регистрацию контекста `swarm` в `cmd/multiverse` делает отдельная задача подключения контекстов (§6), а не T-237.
Зависит: T-236, T-228, **T-017 (EPIC-001 — `FakeState` v0 публикует `replay.completed`)**, ~~**T-230** (агент встречи — его встречи сверяются при восстановлении; T-416), **T-417** (`WithCauseID`)~~ — обе зависимости сняты вместе со сверкой, она теперь в **T-424** (нарезка T-230, tech-lead#2, 2026-09-11). Ссылки: US-011, NFR-010…014, C-01 v1.1, **C-14 (уточнение v0.4)**, **C-14 v1.2 (б), C-05 v1.4 п. 5, ADR-026 п. 10, ADR-027**, ADR-003, `consolidation.md` §14.1 (TL2-6), КД §4.3, §5.1, design §3.2 R10, §13 п. 7, **C-01 v1.5 (T-425)**.
DoD: ~~**(T-416, 2026-09-11; C-14 v1.2 (б), C-05 v1.4 п. 5, ADR-026 п. 10)** Восстановившись, агент встречи сверяет свои встречи с State **после** фазы догона: догон по-прежнему ничего не публикует. Сущность встречи есть, а `encounter.started` с id из причины в журнале нет → публикуется `encounter.started`. Встреча `resolved` с `closed_by_event_id`, а события с этим id нет → публикуется `encounter.ended` с тем же id. Тесты: оба случая; событие успело уйти до падения → второго не публикуется (id в журнале есть), а если ушло дважды, потребители гасят его по id.~~ **Перенесено в T-424 (нарезка T-230, tech-lead#2, 2026-09-11)** — сверка идёт отдельной задачей сразу после фазы догона. От T-237 требуется одно: догон по-прежнему ничего не публикует (пункт ниже). [ ] **(T-425, 2026-09-11; C-01 v1.5 «Паника обработчика»)** На границе обработчиков роя — свой `recover`: паника в обработчике агента останавливает контекст роя с `/health fail`, как у State («Паника в worker'е»). Альтернатива — записать в `dev-log.md` обоснование, что состояние роя переживает панику, и держать это обоснование тестом. Пример такого обоснования: перехват паники роли в Pipeline (T-228) изолирует экземпляр, а общие проекции (`WorldView`, `ScopeIndex`, окна `Dedup`) обработчик роли не меняет. Перехват паники в `eventbus.Delivery` эту границу не заменяет: шина не знает, чьё состояние испорчено. Тест на выбранный вариант. [ ] догон не публикует событий (счётчик публикаций = 0) и восстанавливает `AgentInstance` по `agent.spawned/stopped`; [ ] **(сведение 3)** при получении `analytics.replay.completed mode=recovery` догон стартует сразу; при отсутствии сигнала — старт по истечении `MV_SWARM_REPLAY_WAIT` (тест с укороченным значением), `Health()` = `degraded {state_replay: missing}`, после запоздавшего сигнала статус возвращается в `ok`; [ ] `Health()` возвращает `agents_by_level`, `blueprints`, `laws`, `swarm`. [ ] **(ревизия 4; C-01 v1.7)** `Stop` отменяет контекст подписок до `Close` шины и возвращается в пределах `runtime.StopTimeout`. Тесты: обработчик, слушающий контекст, завершён после `Stop`, горутин не осталось (`goleak`); обработчик, зависший мимо контекста, не держит `Stop` дольше дедлайна его `ctx` (`StopAll` даёт `runtime.StopTimeout`): `Stop` возвращается с ошибкой, а не висит. `MV_SWARM_REPLAY_WAIT` объявляется этой задачей в `shared/env/vars.go` (C-14 уточнение v0.4).

#### T-424 · R10d · Сверка встреч с State после догона
I1b · **M** · developer#2 · M · todo *(ревизия 4: подволна M, было 1.15)*
*(вынесено из T-237 при нарезке T-230, tech-lead#2, 2026-09-11; номер из общего счётчика проекта)*
Что: `internal/swarm/reconcile.go` — шаг сразу после фазы догона T-237: догон закончился, публикаций в live ещё не было. В `swarm.go` добавляется одна строка вызова. `Health()` не трогается — его в той же подволне правит T-239. Шаг находит встречи, у которых факт State есть, а события жизненного цикла нет, и публикует это событие с тем id, который назван в сущности (C-14 v1.2 (б), C-05 v1.4 п. 5, ADR-026 п. 10).
- **Кандидаты.** Полного чтения журнала с начала нет. Кандидаты — сущности встреч, чей факт создания или закрытия лежит в прочитанном при догоне диапазоне журнала, и экземпляры, восстановленные из снапшота с событием, которое построено, но ещё не опубликовано.
- **Проверка «событие есть».** Id события — в сущности встречи: `opened_by_event_id` или `closed_by_event_id`. Событие есть, если этот id встречался в диапазоне догона или известен из снапшота.
- **Сборка события с тем же id.** **(T-425, 2026-09-11; ADR-027 «Уточнение исполнения» п. 3)** Событие перестраивается от той же причины, что в живом режиме:
  - `encounter.started` — событие, по которому GM открыл встречу: вход игрока или `tick.fired`;
  - `encounter.ended` по обмену — действие игрока (соло); для группы — `round.closed` (I2, T-246);
  - `encounter.ended` по чужому факту — сам этот факт (Б1 или `abandoned` последнего игрока).

  Причину находят в журнале по цепочке: факт → предложение по `proposal_id` → его `causation_id`. Цепочка годится, потому что `causation_id` пакета — действие или `round.closed`, а не `combat.decided` (`api-contracts.md` §2.3.6, T-425). Строит событие тот же построитель, что в живом режиме: для `encounter.started` — GM региона (T-232), для `encounter.ended` — агент встречи (T-422, T-423). Опция — `WithCauseID(<id встречи>)`. Событие не публикуется, а в лог идёт `Error`, если причины нет в журнале или полученный id не равен id в сущности: другой id хуже пропуска.
- **Интерфейс.** Роли подключаются к шагу через необязательный интерфейс `Reconciler` в `internal/swarm`. Он проверяется утверждением типа, поэтому `Behaviour` (T-223) не меняется. Реализации — в новых файлах `roles/region_gm_reconcile.go` и `roles/encounter_reconcile.go`.

**Почему отдельная задача.** `encounter.started` некому дописать внутри агента встречи: рой поднимает агента по этому самому событию (КД §4.1), а живой издатель — GM региона. *(architect#2, 2026-09-11: агента теперь поднимает решение GM региона открыть встречу — design §14.2, ADR-028. На довод это не влияет: живой издатель `encounter.started` по-прежнему GM региона. DoD без изменений.)* Сверке нужны построители двух ролей и фаза догона одновременно. Внутри T-237 она лежала бы на критическом пути (T-228 → T-237 → T-242).
Зависит: T-237 (фаза догона), T-236 (восстановление экземпляров), T-232 (построитель `encounter.started`), T-422, T-423 (построители `encounter.ended`), T-417. Ссылки: US-011, NFR-013/014, NFR-061, C-01 v1.4–v1.5, C-05 v1.4–v1.5 п. 4–5, C-14 v1.2 (б), ADR-026 п. 10, ADR-027 и «Уточнение исполнения» п. 3, КД §4.3, design §3.2 R10.
DoD:
- [ ] *(перенесено из T-237)* Восстановившись, рой сверяет встречи с State **после** фазы догона, и догон по-прежнему ничего не публикует. Сущность встречи есть, а `encounter.started` с id из причины в журнале нет → публикуется `encounter.started`. Встреча `resolved` с `closed_by_event_id`, а события с этим id нет → публикуется `encounter.ended` с тем же id. Тесты: оба случая. Если событие успело уйти до падения, второго не публикуется: id в журнале есть. Если оно ушло дважды, потребители гасят его по id.
- [ ] Id события, опубликованного при сверке, равен id, названному в сущности встречи. Причина не найдена в журнале или id не совпал → публикаций 0, `Error`. Тест на обе ветки. *(уточнение нарезки: как выполнить «с тем же id», не заводя второго источника причины)*
- [ ] **(T-425, 2026-09-11; ADR-027 «Уточнение исполнения» п. 3)** Перестроение — от той же причины, что в живом режиме. Тесты, по одному на строку списка: `encounter.started`, открытая по входу; открытая по `tick.fired`; `encounter.ended` по обмену; `encounter.ended` по чужому факту. Строка `round.closed` проверяется в I2 (T-246).
- [ ] Тест T-237 «догон не публикует событий (счётчик публикаций = 0)» остаётся зелёным; публикации сверки идут после конца догона.

#### T-238 · R10c · Integration-тест: снапшот, догон, рестарт (testcontainers)
I1b · **N** · developer#3 · M · todo *(ревизия 4: подволна N, было 1.15)* · *(1.12 → 1.15 при нарезке T-230: зависит от T-237, а тот теперь в 1.14)*
Что: `internal/swarm/integration_test.go` (`//go:build integration`) на testcontainers Redpanda + MinIO (образы из `build/versions.env` через `testkit.Versions()`): подписка/дедуп/DLQ, снапшот роя и `latest.json` в MinIO, догон `ReadRange…End` с курсора, рестарт контекста → `llm_calls=0`.
Зависит: T-237, **T-205** *(ревизия 4, решение 1a: сюда ушла зависимость T-213 от законов)*. Ссылки: US-011, NFR-010…014, ADR-010 п. 1, КД §18, design §9.
**(ревизия 4)** Интеграционные прогоны задачам эпика разрешены только так: `go test -tags integration ./internal/swarm/...` на одноразовых контейнерах testcontainers, не больше одного прогона одновременно, после прогона — проверка, что контейнеров не осталось (решение пользователя 2026-09-13). `make test-integration` целиком и `make up/down` — нет.
DoD: [ ] **(ревизия 4, решение 1a)** стык «рой → шлюз → страж» на настоящих законах: `internal/laws` (`FileSource` на `laws/dark-forest-world.v1.yaml`) подаёт стражу `LawsSource` и `InvariantFunc` через адаптеры T-228; после `mvctl laws bump`-эквивалента (`Bump` на `membus`) ответ со старой `laws_version` даёт один повтор, затем `invalid`; id инвариантов закона совпадают с `internal/mechanics.Invariants()` (`inv-01…inv-10`, плюс `inv-11`). [ ] тест проходит в job `integration` и падает при подмене семантики `Journal.End`; [ ] расхождение семантики `membus` ↔ kafka зафиксировано запросом к EPIC-001 (если найдено); [ ] время выполнения ≤ 3 мин.

#### T-239 · R11 · Admin-маршруты, `/health` роя, текст спецификации для gateway
I1b · **M** · developer#1 · M · todo *(ревизия 4: подволна M, было 1.15)* · *(1.13 developer#3 → 1.15 developer#1 при нарезке T-230: зависит от T-237, а тот теперь в 1.14)*
Что: `admin.go` — `GET /v1/admin/agents`, `POST /v1/admin/agents/{id}/tick` (→ `FireNow`, `meta.actor_kind=ci`) на `runtime.Mux`; `Health()` — `agents_by_level`, `blueprints`, `laws: unknown_check`, `llm: model_missing`, `swarm: region_missing`; **передача текста раздела `admin`** (маршруты, схемы ответов, коды) tech-lead#3 для `api/gateway.openapi.yaml` — файл EPIC-004 не правится.
Зависит: T-227, T-237. Ссылки: US-010, US-012, US-036, C-06, ADR-009 п. 9, КД §14, design §3.2 R11.
DoD: [ ] `httptest` на оба маршрута; admin-тик публикует `tick.fired mode=background, actor_kind=ci` и не сдвигает `next_tick_at` в replay; [ ] `/health` процесса содержит `agents_by_level` (US-010 критерий 2 и US-037 критерий 1); [ ] текст спецификации отправлен tech-lead#3 (отметка в отчёте), задача EPIC-004 на вставку заведена.

### Хвост I1b

#### T-240 · R12 · Миграция I1-0…I1-3, `gm_path`, профиль `legacy`
I1b · **M** · developer#3 · M · todo *(ревизия 4: подволна M, было 1.14, developer#1)*
Что: deprecated-фильтр в `router.go` (типы `gm.*`, `narrative.generate`, `time.syncTime`, `player.moved`, `player.used_skill` игнорируются), копирование `meta.gm_path` события-причины в `combat.decided.gm_path` и `llm.output.gm_path`, `core` не читает `MV_GM_PATH`; проверка профиля `legacy` в compose совместно с devops (запрос, файл compose — EPIC-001).
Зависит: T-228, T-231, T-232, T-233, T-230. Ссылки: US-019, FR-014, BR-12, КД §17 I1-0…I1-3, design §3.2 R12.
DoD: [ ] `gm_path` присутствует в 100 % `combat.decided`/`llm.output` (тест на фикстурах обоих значений); [ ] `grep MV_GM_PATH internal/` = 0 совпадений в `core`; [ ] deprecated-типы не доходят до ролей; [ ] результат проверки профиля `legacy` или применение запасного критерия S5 зафиксированы в `dev-log.md` (стенд — T-264).

#### T-241 · S6 · Второй регион блупринтом и фикстурой (без Go)
I1b · **L** · 2-й слот (tester#2; без него — developer#2 после T-424) · S · todo *(ревизия 4: подволна L, было 1.14, developer#2)*
Что: `blueprints/domain-swamp.md` + записи региона и NPC в `testdata/fixtures/{region,npc}.json` + при необходимости строки в `rules/dark-forest.yaml` (запрос EPIC-002) и `laws/`; тест «второй регион»: цикл входа/встречи проходит для нового региона, `git diff --stat -- '*.go'` = 0 относительно базы задачи.
Зависит: T-232, T-204. Ссылки: US-015, NFR-083, C-11, design §11 п. 9, КД §17.
DoD: [ ] оба `region-gm` работают независимо (два экземпляра `domain`); [ ] тест проверяет отсутствие изменений `*.go`; [ ] инструкция `blueprints/README.md` фактически воспроизводима (по ней и сделан регион).

#### T-242 · R13a · e2e `solo-30`, `death`, `flee-fail`
I1b · **M** · 2-й слот (tester#2 по `plan/backlog.md` M5; без него I1b длиннее — §6) · M · todo *(ревизия 4: подволна M, было 1.15)* · *(1.14 → 1.15 при нарезке T-230: зависит от T-237, а тот теперь в 1.14)*
Что: `test/e2e/` (`//go:build e2e`) на `cmd/multiverse --contexts=all --mode=replay --bus=memory --recording=testdata/recordings/<s>.jsonl --id-source=sequence`; переключение сценариев с `FakeEncounter`/`FakeNarrator` на реальный рой; проверки US-002 (30 ходов, 0 без ответа, 0 расхождений, 0 паник), смерть игрока, провал бегства.
Зависит: T-233, T-232, T-230, **T-421, T-422** (нарезка T-230: смерть и провал бегства идут через закрытие после факта и повтор пакета), T-237, T-221. T-423 и T-424 для этих сценариев не нужны. **(ревизия 4)** Ещё: T-229 (видимость — без неё рой не доставит событие роли) и **задача подключения контекстов `llm`, `laws`, `swarm` в `cmd/multiverse`** (предложена оркестратору, номер не выдан): e2e идёт через `cmd/multiverse --contexts=all`. Ссылки: US-002, US-004, US-017, NFR-061, ADR-010, КД §18, design §3.2 R13.
DoD: [ ] сценарии зелёные без GPU, суммарно ≤ 5 мин; [ ] два прогона `solo-30` дают равные хэши последовательности доменных событий (NFR-061); [ ] при отсутствии записи — понятная ошибка «запись неполная», а не живой вызов.

#### T-243 · R13b · e2e `background-6h`, `degraded`, `recovery`, `injections-10`
I1b · **N** · developer#2 · M · todo *(ревизия 4: подволна N, было 1.16)* · *(1.15 → 1.16 при нарезке T-230: зависит от T-242, а тот теперь в 1.15)*
Что: `background-6h` (S14: admin-тики, число фоновых `llm.output` ≤ B, HP/позиция игроков не меняются), `degraded` (`fake` возвращает ошибку → 100 % шаблонов, ошибок нет), `recovery` (рестарт контекстов в процессе → `llm_calls=0`, `identical=true`), `injections-10` (0 `entity.updated` от нарратива, 0 `world.law_breach.proposed`).
Зависит: T-242, T-227, T-237. Ссылки: US-036, US-004, US-011, US-012, NFR-007/014/043/053, SEC-17, ADR-010, design §3.2 R13.
DoD: [ ] все четыре сценария зелёные в CI без GPU; [ ] `background-6h` подтверждает ≤ B вызовов в час и 0 вызовов от `task`-агентов; [ ] `injections-10` включает инъекции в `say` и в имена сущностей.

#### T-244 · R13c · Golden-набор нарративов (20 ходов + 3 тика)
I1b · **N** · 2-й слот (tester#2 или qa-engineer#2, §8 мера 1; без него — developer#3 после T-238) · M · todo *(ревизия 4: подволна N, было 1.16, developer#3)* · *(1.15 → 1.16 при нарезке T-230: зависит от T-242, а тот теперь в 1.15)*
Что: `internal/llm/golden_test.go` — сравнение с эталонами `testdata/golden/*.json`: валидность по схеме, язык, числа механики не противоречат `combat.decided`, `filter.status=pass`, 0 ложных срабатываний фильтра; сбор эталонов — совместно с EPIC-005 (005-ops), из записей `actor_kind=ci`.
Зависит: T-242, T-216, T-221. Ссылки: US-016, US-018, NFR-022/048/065, ADR-010 п. 1, КД §18.
DoD: [ ] тест падает при подмене числа урона в нарративе и при появлении CJK; [ ] эталоны получены только из записей `actor_kind=ci` (проверка в тесте); [ ] обновление эталонов возможно только осознанной задачей (описано в `testdata/golden/README.md`).

#### T-256 · Удаление хука `MV_SWARM_FAKE` из `cmd/multiverse` — **критерий готовности I1**
I1b · **N** · developer#1 · S · todo *(ревизия 4: подволна N, было 1.16)* · *(1.15 → 1.16 при нарезке T-230: зависит от T-242, а тот теперь в 1.15)*
*(новая задача сведения 2: ADR-001 доп. п. 8 — «хук удаляется при слиянии EPIC-003 I1 (критерий тега `mvp-1/i1`)»; `qa-engineer` включает это в `testing/strategy.md` §5.4.)*
Что: удалить `cmd/multiverse/fake_contexts.go` и объявление `MV_SWARM_FAKE` у процесса; снять исключение `depguard` по файлу в `.golangci.yml`; убедиться, что e2e и I1-α-сценарии переведены на настоящий контекст `swarm` (T-242). **Сами фейки остаются** в `shared/testkit/swarm` (unit/e2e на `membus`, EPIC-002/004). ~~Файлы `cmd/multiverse/*` и `.golangci.yml` — **владение EPIC-001: PR через tech-lead#1**, задача TEAM-2 сводится к подготовке PR и проверке.~~
**(ревизия 4) Что снимается — по дереву `1c2ee7e`, и кто это делает.**
- **В ветке задачи** (файлы владельца EPIC-003 после T-446): `cmd/multiverse/fake_contexts.go` и `fake_contexts_test.go`; ветка `MV_SWARM_FAKE` в `cmd/multiverse/main_test.go`; фабрика `newSwarm` и константа `swarmContext` в регистрации контекстов. Контекст `swarm` к этому моменту уже зарегистрирован настоящим — задачей подключения контекстов.
- **Задачей EPIC-001** (`contract-change` через system-architect#1; запрос оформляет tech-lead#2 при старте T-256): в `.golangci.yml` — правило `cmd-multiverse-fake-contexts` и строка `!**/cmd/multiverse/fake_contexts.go` в `no-testkit-in-production`; в `shared/env/vars.go` — `SwarmFake`; в `shared/contracts/sources.go` — исключение для фейков под `MV_SWARM_FAKE=true`. Строка в `.env.example` уходит тем же запросом.
- **Проверить, не правя:** комментарий в `test/e2e/empty_world_test.go:124` и `cmd/multiverse/bus_memory.go:16`. Текст о `testkit` в `bus_memory.go` — пояснение, а не импорт.
Зависит: T-242 (e2e переключены на рой), T-237, T-255, **задача подключения контекстов** (номер у оркестратора) *(ревизия 4)*. Ссылки: **ADR-001 доп. п. 8**, `contracts.md` v0.3 §0, C-05 (contracts v0.3+), design §4.1 п. 1, §13 п. 1.
DoD: [ ] ~~`grep -r "MV_SWARM_FAKE\|testkit" cmd/multiverse/` = 0 совпадений~~ **(ревизия 4)** `go list -deps ./cmd/multiverse` без тестов не содержит `shared/testkit/...`; `grep -rn "MV_SWARM_FAKE" --include=*.go cmd/ internal/ shared/env/ shared/contracts/` = 0 совпадений. Упоминания в `shared/testkit/swarm` (двойники остаются) и в комментарии `test/e2e` проверены и поправлены по смыслу. Прежний грэп по `testkit` ловил пояснение в `bus_memory.go` и тестовые импорты, поэтому заменён; [ ] исключение `depguard` по файлу снято, `golangci-lint run` зелёный; [ ] e2e `solo-30`/`death`/`flee-fail` зелёные **без** флага (на реальном рое); [ ] ~~PR принят tech-lead#1~~ **(ревизия 4)** задача EPIC-001 на `.golangci.yml`, `vars.go`, `sources.go` слита в `develop`, эпик синхронизирован **до** проставления тега `mvp-1/i1`; [ ] `shared/testkit/swarm` не удалён и остаётся рабочим (тесты EPIC-002/004 зелёные).

#### T-245 · Документация I1b: README пакетов и фрагменты runbook
I1b · **N** · developer#1 · S · todo *(ревизия 4: подволна N, было 1.16)* · *(1.15 → 1.16 при нарезке T-230: зависит от T-239, а тот теперь в 1.15)*
Что: `internal/swarm/README.md` (карта пакета, потоки события, роли, TTL, тики), `internal/llm/README.md` (конвейер, статусы, провайдеры, env), `internal/laws/README.md`; фрагменты runbook для devops/оператора: «как выполнить admin-тик», «как читать `/health` роя», «что значит `degraded {laws|llm|swarm|blueprints}`», «как включить `MV_LLM_STORE_PROMPTS` и где лежат промпты» — передаются tech-writer/devops (файлы `Docs/` вне владения TEAM-2 не правятся).
Зависит: T-239, T-240. Ссылки: NFR-095, US-010, design §7.
DoD: [ ] README трёх пакетов есть и соответствуют коду (ссылки на файлы существуют); [ ] фрагменты runbook переданы (отметка в отчёте); [ ] таблица env блока сверена с `mvctl env check`.

---

## 4. Инкремент I2 «Группа, память, завершение миграции» (8 задач)

Старт — по приёмке I1 тимлидом команды; слияние — после интеграционного прогона I1 (порядок волны 2: 002 → **003** → 004 → 005).

#### T-246 · G1 · Раунд группы в роли `encounter`
I2 · 2.1 · developer#1 · M · todo
Что: `round.closed{acted[], auto_defended[], idle[]}` → восстановление порядка действий, ответ NPC один раз за раунд (`mechanics.NPCTarget` без `idle|out_of_combat|dead|abandoned`, `Resolve(causeEventID = round.closed.id)`), `combat.decided{action: npc_attack, round{seq}}`, `idle` после 2 подряд `auto_defended` (`entity.update.proposed participation`), `group.left` во встрече = `flee` (BR-13), `encounter.ended reason=players_out`.
**Дополнение (сведение 3, C-04 v1.1; З-2/З-4)**: `dead`/`abandoned` участники остаются в `members[]` (история), но исключаются из `expected[]`/`acted[]`, целей NPC и `participation=active`; группа может остаться **без лидера** (`Group.leader_id = null`) — роль это допускает и на `leader_id=null` не падает.
Зависит: T-230, **T-421, T-422, T-423** (нарезка T-230, 2026-09-11: раунд группы строится на всей соло-роли); EPIC-002 I2 (atomic группы, `Participation`) — до слияния используем `FakeState`. **(T-425, 2026-09-11; ADR-027 «Уточнение исполнения» п. 3, `api-contracts.md` §2.3.6)** В группе причина `encounter.ended` для `WithCauseID` и `causation_id` пакета раунда — `round.closed`, а не `combat.decided`. Тест: id события равен `closed_by_event_id` и одинаков при двух построениях. Ссылки: US-006, US-007, FR-025, FR-061, BR-13 v0.4, NFR-005, **`contracts.md` v0.4 C-02 v1.2, C-04 v1.1**, КД §8.3.
DoD: [ ] порядок действий раунда детерминирован; [ ] ровно один `npc_attack` на раунд; [ ] позиция каждого `alive` участника = позиции группы (инв. 4); [ ] **(сведение 3)** участник со `status=abandoned` не попадает в `expected[]`/цели NPC; при `leader_id=null` роль продолжает раунд без ошибок. [ ] **(architect#2, 2026-09-11; design §14.1 п. 1)** Один `round.closed` порождает несколько обменов, поэтому `parts` у `dice.rolled`, `combat.decided` и пакета начинаются с id действующего (для ответа NPC — id NPC). Тест: у разных участников раунда при одинаковых индексах id разные.

#### T-247 · G2 · Роль `group-narrator` и персональные GM в группе
I2 · 2.2 · developer#2 · M · todo
Что: `roles/group_narrator.go` — Phase 2 `round` по завершению раунда (последнее `combat.decided npc_attack` или `encounter.ended` после `round.closed seq=N`), `recipients[] = active|idle|out_of_combat`, один `narrative_event_id`; `turn`/`world_event` вне встречи; TTL 45 мин от действия любого участника; персональные GM участников — `rule-only` (LLM только `entry` при `absence`).
Зависит: T-246, T-233. Ссылки: US-006, US-007, US-004, NFR-005/006, C-05, КД §8.1 п. 4, §8.2.
DoD: [ ] ≤ 2 вызова LLM на раунд группы (счётчик); [ ] один `narrative.output` на раунд, дублей от персональных GM нет; [ ] мёртвый участник получает только `kind: death` от своего персонального GM.

#### T-248 · G3 · `MemoryClient` HTTP (C-09, Should)
I2 · 2.2 · developer#3 · M · todo
Что: `context/memory.go` — HTTP-клиент к 005-memory (`MV_MEMORY_URL`, таймаут 500 мс, деградация в `journalContext` без ошибки игроку, лог `memory_degraded=true`), секция `<facts source="memory">` с тем же экранированием и лимитом, что `<player_text>` (SEC-17/T-9); `NopMemory` в replay.
Зависит: T-235; 005-memory (при отрезании — задача сводится к проверке `NopMemory`, см. §9 риск 4). Ссылки: US-005, US-015, FR-127, C-09, ADR-005 доп. п. 4, КД §11.3.
DoD: [ ] недоступная память не ломает ход (тест на таймаут); [ ] авторский факт приоритетнее `generated` (тест с подложенным противоречием); [ ] инъекция через факт памяти экранируется (входит в `injections-10`, T-251).

#### T-249 · G4 · Горячая перезагрузка блупринтов (FR-091, Should)
I2 · 2.2 · developer#1 · S · todo
Что: `registry.go` — опрос каталога по `clock.Timers` раз в 5 с (без `fsnotify`, решение design §13 п. 6), новая версия применяется к следующему спавну и timer-агентам на следующем тике, событие `agent.blueprint_reloaded{content_hash}`.
Зависит: T-222. Ссылки: US-015, FR-091, C-11, design §13 п. 6.
DoD: [ ] изменение файла на диске → `agent.blueprint_reloaded` в пределах 5 с (тест на `clock.Manual`); [ ] невалидная новая версия не заменяет рабочую и попадает в `/health degraded {blueprints}`; [ ] зависимость `fsnotify` в `go.mod` не добавлена.

#### T-250 · G5 · Лимиты Should: интерактивный и облачный
I2 · 2.1 · developer#2 · S · todo
Что: `MV_LLM_TURN_CALLS_PER_MIN` (интерактив: превышение → шаблон + предупреждение, механика не ждёт), облачный денежный лимит `MV_LLM_CLOUD_BUDGET_USD_PER_DAY` → переключение на локальный провайдер или шаблон.
Зависит: T-210. Ссылки: US-012, FR-071/072, NFR-051, КД §9.3.
DoD: [ ] при превышении лимита `llm.output.rejected reason=budget_exceeded` с `budget{kind, limit, window}`; [ ] 0 вызовов в облако сверх лимита (тест с низким лимитом); [ ] лимиты задаются конфигурацией, не кодом.

#### T-251 · G6 · e2e I2: `group-3x30`, инъекции через память, повтор S6
I2 · 2.3 · developer#2 · M · todo
Что: сценарий `group-3x30` (S2: 30 раундов, ≥ 60 действий, `rounds/close` от харнесса, молчащий игрок, индивидуальное бегство, выход участника), `injections-10` + ≥ 3 инъекции через факты памяти, повтор проверки второго региона (S6) на I2.
Зависит: T-246, T-247, T-248. Ссылки: US-006, US-007, US-015, NFR-005/043, ADR-010, КД §18.
DoD: [ ] S2 зелёный: 0 расхождений, 0 без ответа, позиция `alive` участников = позиции группы; [ ] инъекции через память не меняют состояние и не порождают `world.law_breach.proposed`; [ ] прогон ≤ 4 мин без GPU.

#### T-252 · G7 · Завершение миграции (S5) — последняя задача I2
I2 · 2.4 · developer#1 · M · todo
Что: перенос `services/narrative-orchestrator` и `services/semantic-memory` в `services/_archive/` с `ARCHIVED.md` (ничего не удалять — U-1/OQ-A-17); снятие профиля `legacy` и `chromadb` из compose (запрос devops/EPIC-001), удаление deprecated-типов из реестра (~~PR через system-architect~~ *ревизия 4: задача EPIC-001 `contract-change` через system-architect, либо файл владельца EPIC-003 после T-446*) и флага `MV_GM_PATH` (чтение у gateway снимает EPIC-004).
Зависит: T-251 (зелёный S2), T-240. Ссылки: US-019, BR-12, D-3, U-1, КД §17 I2-2, design §3.3 G7.
DoD: [ ] `gm_path=agent` в 100 % `llm.output` (или запасной критерий S5, F-6); [ ] `services/_archive/**` вне `go build ./...`, `ARCHIVED.md` заполнены (причина, коммит, эпик возврата); [ ] запросы к EPIC-001 (compose) и system-architect (реестр типов) оформлены и приняты.

#### T-253 · Документация I2 и заметка о миграции
I2 · 2.3 · developer#3 · S · todo
Что: обновление `internal/swarm/README.md` (группа, раунд, память), `blueprints/README.md` (group-narrator, hot reload), заметка «переход с legacy завершён» для `Docs/archive/` и вход для tech-writer в CLAUDE.md/AGENTS.md (файлы tech-writer не правим).
Зависит: T-247, T-249. Ссылки: NFR-095, design §7.
DoD: [ ] README отражают фактическое поведение I2; [ ] заметка передана tech-writer (отметка в отчёте); [ ] ссылки на удалённые/архивированные пути отсутствуют.

---

## 5. Стендовые задачи (**stand**, вне слотов разработчиков)

Выполняет человек (оператор целевой машины RTX 4090) вместе с `tester#2` или `architect#2`. Слот разработчика не занимают (`teams.md` §4), приёмку инкремента не блокируют, кроме отмеченного.

| ID | Задача | Кто | Когда | Критерий |
|---|---|---|---|---|
| **T-260** stand | **Замер на llama-server и подтверждение моделей в пяти блупринтах по `ops/metrics/baseline.md` (сведение 2)**: прогон `scripts/llm-bench.ps1` против `/v1/chat/completions` (`response_format json_schema`, `chat_template_kwargs.enable_thinking=false`, `usage`/`timings` в CSV, VRAM по `nvidia-smi`) для конфигурации **E** (`Qwen3.8-27B-UD-Q3_K_XL`, `num_ctx` 8192/16384, KV f16/q8_0); **Ollama привлекается только если F-8 выбирает C или A** (тогда же стартует T-254 B3b) | architect#2 + человек | после F-8, до конца I1a | правка пяти YAML-полей блупринтов **по `baseline.md`**; NFR-002 (p95) / NFR-090 (русский) / NFR-022 (`valid_first_try` с грамматикой llama.cpp) подтверждены для E **или** зафиксирован переход на следующую конфигурацию по `decision_order` ADR-005 доп. 3 (Qwen3.6-35B-A3B, C, A) с записью в `baseline.md` и `dev-log.md` *(T-440)* |
| **T-261** stand | Снять первую запись `testdata/recordings/solo-30.jsonl` на **живом llama-server** (`make llm-up`, провайдер `openai_compat`) через `mvctl record` | tester#2 + человек | конец I1a | запись проходит `providers/recorded` в e2e; только `actor_kind=ci` |
| **T-262** stand | Снять записи `death`, `flee-fail`, `injections-10`, `background-6h`, `recovery` | tester#2 + человек | конец I1b (до T-243) | все сценарии e2e зелёные на записях; `privacy-scan` чистый |
| **T-263** stand | Живой прогон S1 и S14 **на llama-server** (нарратив, тики, бюджет B); Ollama — только при конфигурации C/A | tester#2 + человек | интеграция I1 | S1/S14 зелёные; `llm usage` показывает ≤ B фоновых вызовов в час; `/health` шлюза корректно отражает `loading` при старте llama-server (503) |
| **T-264** stand | Проверка профиля `legacy` (S5) и решение по запасному критерию | tester#2 + devops + человек | I1b → I2 | профиль поднимается и сравнение нарративов возможно **или** зафиксирован запасной критерий |
| **T-265** stand | nightly-gpu матрица **на llama-server** (`scripts/llm-bench.ps1`, `bench-matrix.json` с вариантами E/E+ и, при необходимости, C/A): `valid_first_try`, `phase2 p95`, доля латиницы/CJK, `usage`/`timings`, VRAM → `ops/metrics/baseline.md` | architect#2 + человек | конец I1b | пороги NFR-002/022/090 закреплены (совместно с BA и EPIC-005); зафиксирован пин билда llama.cpp (риск #20345, `build/versions.env` — devops) |
| **T-266** stand | Запись `group-3x30.jsonl` | tester#2 + человек | I2, до T-251 | e2e `group-3x30` зелёный на записи |

**Внесено tech-lead#1 при приёмке T-435 (решение оркестратора о дублировании):**

- **T-439 (из T-435, 2026-09-12)** · потолок длины нарратива под целевую конфигурацию (`max_tokens` фазы и `text.maxLength`/`maxItems` схемы нарратива, КД §13.3–13.4, стартовая модель в `:735` и `:977`) · исполнитель **architect#2**, подтверждает **tech-lead#2** · ветка `task/T-439-narrative-length-cap` · числа окончательны после T-438 · полный текст — раздел T-439 индекса EPIC-001. **(ревизия 4)** Задача передана в EPIC-003 (`state.js`) и заведена разделом в §2 этого индекса — «T-439 · потолок длины нарратива».
- **Бэклог стенда, без номера (из T-435, 2026-09-12; предложение ревью #2, рядом с T-260/T-261)** · проверить на стенде, что `llama-server` прекращает генерацию при закрытии соединения клиентом: при `MV_LLM_SLOTS=1` на этом держится NFR-002 — брошенный запрос иначе занимает единственный слот и задерживает следующий нарратив (ADR-014). Номер выдаёт оркестратор при взятии в работу.

---

## 6. Волны (подволны эпика)

Оркестратор запускает подволну одним сообщением: до 3 разработчиков TEAM-2 (лимит `maxAgentsPerRole = 3`), каждый в своей папке `.worktrees/T-NNN` на ветке `task/T-NNN-<slug>` от ветки эпика. Ревьюеры — отдельным запуском после. Задачи одной подволны независимы и ведутся параллельно. Свободный слот TEAM-2 отдаётся другой команде (`teams.md` §4).

### 6.1 Подволны A…N (ревизия 4, 2026-09-13)

Пересобраны по сверке tech-lead#2. Выполненные до перезапуска T-214, T-215 (обе в `review`), T-219, T-220, T-255, T-419 из таблицы убраны. T-205, T-210, T-216 подняты в начало: у них нет зависимостей внутри эпика. A, B, C заданы оркестратором, дальше — по зависимостям. «Готова к старту» — все зависимости `done` (или схема в дереве для T-214/T-215 в `review`) и ветка эпика синхронизирована с `develop`.

| Подволна | developer#1 | developer#2 | developer#3 | 2-й слот (tester#2) / architect#2 | Выход и условия старта |
|---|---|---|---|---|---|
| **A** | T-201 A1 `shared/agent` v2 | **T-206 B1** (in-progress) | T-216 C1 фильтр (a) | architect#2: **T-439** потолок нарратива (первая задача) | T-201 — после решения tech-lead#1 об исключении `shared/agent/`; T-216 — после утверждения словаря (a) пользователем |
| **B** | T-202 A2 уровни + валидатор | T-207 B2 `fake`/`recorded` | T-205 A7 законы | — | после T-207 — контрольное слияние эпика в `develop` (поставка C-07/C-15) |
| **C** | T-203 A3 блупринты — **после T-439** | T-208 B3a `openai_compat` | T-210 B5a бюджет (S) | — | T-203 ждёт T-439; стык B↔C: `fake` для тестов бюджета |
| **D** | T-222 R1 реестр + scope (старт I1b) | T-209 B4 парсер + язык | T-218 C3 промпт | — | `prompt_hash` вида `sha256:`; схемы `schemas/agent` компилируются |
| **E** | T-223 R2a `Behaviour`/`Emitter` | T-211 B5b `Recorder`/`Usage` | T-234 R9a `WorldView` | — | интерфейсы T-223 принимает tech-lead#2 отдельно (§9 риск 3) |
| **F** | T-224 R2b `Router.Route` | T-212 B6a ядро шлюза | T-217 C2 страж | — | `Generate` на `fake` проходит парсер и язык; страж покрыт таблицей 9×2. T-445 старт не блокирует: схема `config.cloud_enabled` с `external_players_ack` и шаблоном `provider` уже в дереве |
| **G** | T-225 R3 Lifecycle | T-213 B6b фильтр/страж/запись | T-235 R9b сборщик (S) **и** T-204 A4 CLI (S) | — | **готовность шлюза I1a**: всё I1a, кроме T-221 (подволна L) и стенда T-261; T-235 — после T-445 (`prompt$`) |
| **H** | T-226 R4a Scheduler | T-229 R2c видимость | T-236 R10a снапшот | — | T-226 — после T-445 (`providers/fake$` в тестах) |
| **I** | T-227 R4b тики/бюджет | T-230 R7a встреча: решение и пакет | T-233 R8 personal-gm | — | T-233 — после T-445 (`guardian$`) |
| **J** | T-228 R5 Pipeline + шаблоны | T-421 R7b судьба пакета | T-231 R6a global-gm | — | T-228 — после T-445 (`template$`) |
| **K** | T-237 R10b `swarm.Context`, догон | T-422 R7c конец после факта | T-232 R6b region-gm | — | догон без эмиссии; `Stop` по C-01 v1.7 |
| **L** | **задача подключения `llm`, `laws`, `swarm` в `cmd/multiverse`** (номер у оркестратора; после T-446) | T-423 R7d закрытие по чужому факту | T-221 C5 `recorded.Writer` + `mvctl record` | T-241 S6 второй регион (S) | роль `encounter` целиком; один `llm.Gateway` на процесс |
| **M** | T-239 R11 admin/`/health` | T-424 R10d сверка встреч | T-240 R12 миграция, `gm_path` | T-242 R13a e2e solo/death/flee | S1 на записях; текст admin → tech-lead#3 |
| **N** | T-245 доки I1b (S) **и** T-256 удаление хука (S) | T-243 R13b e2e фон/деградация/recovery | T-238 R10c integration + стык законов | T-244 R13c golden | **готовность I1** → контрольное слияние эпика в `develop`; хук снят **до** тега `mvp-1/i1` |
| **2.1–2.4** | как в §6.2 (T-246, T-249, T-252) | как в §6.2 (T-250, T-247, T-251) | как в §6.2 (T-254 условно, T-248, T-253) | — | без изменений, кроме условия T-254 (раздел задачи) |

**Проверка порядка (ревизия 4).** Ни одна задача не стоит в одной подволне со своей зависимостью или раньше неё. Проверены все пары, включая новые зависимости ревизии: T-203 → T-439, T-228 → T-205, T-238 → T-205, T-242 → T-229 и задача подключения, T-256 → задача подключения. Пары S в одной ячейке (G у developer#3, N у developer#1) между собой не зависят и по файлам не пересекаются.

**Критический путь (ревизия 4):** `T-439 → T-203 → T-209` (A, C, D) и `T-206 → T-207`/`T-210` (A…C), затем `T-209 → T-212 → T-213` (D, F, G). T-439 теперь на пути: схемы `schemas/agent` для парсера делает T-203, а он ждёт потолок нарратива. Дальше → `T-226 → T-227 → T-228 → T-237` (H…K) → подключение контекстов (L) → `T-242` (M) → `T-243`/`T-256` (N). Параллельно: `T-222 → T-223 → T-225` (D, E, G) — вход в T-226.

**Без второго слота** T-241, T-242 и T-244 уходят к разработчикам, и I1b заканчивается в подволне O или P, а не в N. **Задержка T-445** (EPIC-001) сдвигает T-235, T-226, T-233 и T-228 — им нужны новые строки `depguard` — и всё, что за ними; приёмку T-215 — тоже. **Задержка T-446** сдвигает подволну L. Эти три условия tech-lead#2 проверяет перед запуском каждой подволны.

**Правило файлов (ревизия 4).** `roles/triggers.go` — только developer#3 (T-233, T-231). `roles/encounter*.go` — только developer#2 (T-230 → T-421 → T-422 → T-423 подряд; T-424 добавляет `roles/encounter_reconcile.go`). `router.go` — только developer#1 (T-224). `shared/env/vars.go` и `.env.example` правят многие задачи, у каждой — свои строки: конфликт слияния разрешает исполнитель позже слитой ветки. `internal/llm/providers/recorded` — developer#2 (T-207), затем developer#3 (T-221, подволна L, после слияния T-207).

**Ветки задач (ревизия 4)** — `task/T-NNN-<slug>` от `epic/EPIC-003-swarm-llm-laws`. Слаги I1a названы в заголовках задач §2, слаги I1b и I2 — в карточках `tasks/T-NNN.md`: T-222 `blueprint-registry-scope`, T-223 `agent-instance-emitter`, T-224 `router-route`, T-225 `agent-lifecycle`, T-226 `scheduler`, T-227 `ticks-background-budget`, T-228 `pipeline-templates`, T-229 `visibility-recipients`, T-230 `encounter-exchange`, T-421 `encounter-package-fate`, T-422 `encounter-lifecycle`, T-423 `encounter-foreign-close`, T-231 `global-gm`, T-232 `region-gm`, T-233 `personal-gm`, T-234 `worldview-journal`, T-235 `context-builder`, T-236 `swarm-snapshot`, T-237 `swarm-context-catchup`, T-424 `encounter-reconcile`, T-238 `swarm-integration`, T-239 `swarm-admin-health`, T-240 `migration-gm-path`, T-241 `second-region`, T-242 `e2e-solo-death-flee`, T-243 `e2e-background-recovery`, T-244 `golden-narratives`, T-245 `docs-i1b`, T-256 `remove-swarm-fake-hook`, T-246 `group-round`, T-247 `group-narrator`, T-248 `memory-client`, T-249 `blueprint-hot-reload`, T-250 `llm-limits-should`, T-251 `e2e-group`, T-252 `legacy-archive`, T-253 `docs-i2`, T-254 `ollama-provider`.

### 6.2 Прежняя таблица 1.1–2.4 (до ревизии 4; для истории, не для запуска)

| Подволна | developer#1 | developer#2 | developer#3 | Параллельность | Выход (что сливается / что проверяется) |
|---|---|---|---|---|---|
| **1.1** | T-201 A1 | T-206 B1 | T-214 A5 схемы ч.1 | 3 | схемы ч.1 → **ранний merge** в `integration/mvp-1`; `mvctl contracts check` зелёный |
| **1.2** | T-202 A2 | T-207 B2 fake/recorded | T-215 A6 схемы ч.2 | 3 | схемы ч.2 + `providers/fake` → **ранний merge** (C-07/C-15 для EPIC-002/005) |
| **1.3** | T-203 A3 блупринты (модель E) | **T-208 B3a `openai_compat`** | T-219 C4a FakeEncounter + `FakeContext` | 3 | `FakeEncounter` → **ранний merge `shared/testkit/swarm`**; e2e `solo-30` без роя зелёный; провайдер по умолчанию отвечает на httptest-фикстурах llama-server |
| **1.4** | T-204 A4 CLI + доки **и T-255 хук `fake_contexts.go`** (обе S) | T-209 B4 parser | T-220 C4b FakeNarrator | 3 | **вклад EPIC-003 в I1-α закрыт** (включая хук `MV_SWARM_FAKE` в `cmd/multiverse`, PR через tech-lead#1); `mvctl blueprint validate blueprints/` зелёный в CI |
| **1.5** | T-205 A7 laws | T-210 B5a budget | T-216 C1 filter | 3 | `laws@v1` загружается, `mvctl laws show` работает; golden фильтра 0 FP |
| **1.6** | T-221 C5 record | T-211 B5b record/usage | T-218 C3 prompt | 3 | `llm.output` пишется; `prompt_hash` стабилен |
| **1.7** | T-222 R1 (старт I1b) | T-212 B6a | T-217 C2 guardian | 3 | `Generate` на `fake` проходит парсер и язык; страж покрыт таблицей 9×2 |
| **1.8** | T-223 R2a | T-213 B6b | T-234 R9a context | 3 | **готовность I1a** (кроме стендовой записи T-261) |
| **1.9** | T-224 R2b Route | T-229 R2c видимость | T-235 R9b builder | 3 | маршрутизация и видимость покрыты таблицами |
| **1.10** | T-225 R3 Lifecycle | T-230 R7a encounter: решение и пакет | T-233 R8 personal-gm | 3 | решение обмена соло на `FixedMechanics` детерминировано, `exchange` на каждом решении; один вызов LLM на ход |
| **1.11** | T-226 R4a Scheduler | T-421 R7b судьба пакета | T-236 R10a snapshot | 3 | повтор под тем же `proposal_id`, откат участия, отказ не по гонке; снапшот round-trip |
| **1.12** | T-227 R4b тики/бюджет | T-422 R7c конец после факта, откладывание | T-231 R6a global-gm | 3 | `encounter.ended` только после факта, id из причины; `tick.fired` с `lod_allowed` |
| **1.13** | T-228 R5 Pipeline | T-423 R7d закрытие по чужому факту | T-232 R6b region-gm | 3 | роль `encounter` целиком (T-230, T-421…T-423); встреча по тику региона, `encounter.started` после факта |
| **1.14** | T-240 R12 миграция | T-241 S6 второй регион (S) | T-237 R10b runtime/догон | 3 | `gm_path` в 100 % событий; S6 без Go-диффа; догон без эмиссии |
| **1.15** | T-239 R11 admin/health | T-424 R10d сверка встреч | T-238 R10c integration | 3 + 2-й слот: T-242 R13a e2e solo/death/flee | `/health` с `agents_by_level`, текст admin → tech-lead#3; встречи сверены после догона; integration на testcontainers зелёный; S1 на записях |
| **1.16** | T-245 доки I1b **и T-256 удаление хука** (обе S) | T-243 R13b e2e фон/деградация/recovery | T-244 R13c golden | 3 | **готовность I1** → передача tech-lead#1 на интеграцию (порядок 002 → 004 → 003); хук `MV_SWARM_FAKE` удалён из `cmd/multiverse` **до** тега `mvp-1/i1` |
| **2.1** | T-246 G1 раунд | T-250 G5 лимиты | **T-254 B3b `ollama` — только если F-8 выбрал C/A** (иначе слот другой команде) | 2–3 | раунд группы на `FakeState`; лимиты Should; при конфигурации E слот освобождается |
| **2.2** | T-249 G4 hot reload | T-247 G2 group-narrator | T-248 G3 MemoryClient | 3 | ≤ 2 вызова на раунд; `blueprint_reloaded` |
| **2.3** | — | T-251 G6 e2e group-3x30 | T-253 доки I2 | 2 | **S2 зелёный** → разрешение на T-252 |
| **2.4** | T-252 G7 архив legacy | — | — | 1 | **готовность I2**, тег `mvp-1/i2` |

Правило внутри подволны: задачи не пересекаются по файлам (потоки A/B/C и рантайм/роли/контекст разведены по пакетам). Пересечения-исключения: `roles/triggers.go` — только developer#3 (T-233/T-231; до нарезки T-230 — developer#2), `roles/encounter*.go` — только developer#2 (T-230 → T-421 → T-422 → T-423, строго подряд; T-424 добавляет новый файл `roles/encounter_reconcile.go`), `router.go` — только developer#1 (T-224), интерфейсы `Behaviour`/`Phase2Runner`/`TickRunner` фиксируются в T-223 и далее меняются только задачей тимлида.

Проверка порядка: ни одна задача не стоит в одной подволне со своей зависимостью. Поток ролей (T-229…T-233) намеренно отвязан от Pipeline (T-228) через интерфейсы T-223 — иначе роли ждали бы подволны 1.13 и I1b удлинился бы на 3 подволны.

**Пересборка 1.10–1.16 (нарезка T-230, tech-lead#2, 2026-09-11).** Прежняя таблица нарушала правило выше в двух местах: T-236 стоял в 1.10 вместе со своей зависимостью T-225, а T-237 — в 1.11, раньше своей зависимости T-228 (1.13). С ним уезжали T-238, T-239, T-242 и хвост. По зависимостям I1b заканчивается в **1.16**, а не в 1.15, — и так было бы без нарезки. Нарезка T-230 путь не удлиняет. Цепочка ролей T-230 → T-421 → T-422 → T-423 занимает 1.10–1.13, а T-233, T-231, T-232 взял developer#3: его подволны 1.10, 1.12 и 1.13 в прежней таблице были заняты задачами, которые по зависимостям там стоять не могли. Сверка T-424 стоит вне критического пути, в 1.15. **Без второго слота** под T-242 (tester#2 по `plan/backlog.md` M5) хвост 1.15–1.16 не вмещает T-238 или T-424, и I1b заканчивается в 1.17.

---

## 7. Чек-листы готовности инкрементов

### I1-α «соло на шаблонах через бота» — вклад EPIC-003 (подволна 1.4)
**(ревизия 4)** Всё ниже уже в `develop` с контрольного слияния EPIC-001 (`1c2ee7e`); «слиты в `integration/mvp-1`» читать как «в `develop`». Отмечено по состоянию на 2026-09-13.
- [x] T-214 A5 — схемы `combat.decided`, `encounter.*`, `agent.*`, `tick.*` зарегистрированы и в `develop` (done, принята tech-lead#2 2026-09-13).
- [x] T-215 A6 — схема `narrative.output` в `develop` (done, принята tech-lead#2 2026-09-13; условия `llm.output` — T-445, в эпике).
- [x] T-219 `FakeEncounter` + `FakeContext` в `develop` (done); бой с волком доходит до `combat.decided` и `entity.update.proposed`. `FakeEncounter` — **единственная** заглушка боя (C-05 (contracts v0.3+); `WithEncounterStub` в F-10 не делается).
- [x] T-220 `FakeNarrator` + `template/ru.go` в `develop` (done); нарратив приходит отдельным сообщением с пометкой шаблона.
- [x] Смерть, трофей, бегство работают в e2e `solo-30` без роя (T-219, T-419).
- [x] T-255: хук `cmd/multiverse/fake_contexts.go` (`MV_SWARM_FAKE=true` читает **`cmd/multiverse`**, ADR-001 доп. п. 8) в `develop` (done); правило `depguard` по файлу в `.golangci.yml` есть.

### I1a (подволна ~~1.8~~ G, ревизия 4; T-221 — L)
- [ ] `mvctl blueprint validate blueprints/` зелёный на пяти блупринтах (T-203, T-204).
- [ ] `llm.Gateway.Generate` на `fake` и `recorded` проходит парсер → язык → фильтр (a) → страж, все ветки статусов (T-212, T-213).
- [ ] `llm.output` записывается до использования ответа; `quarantined` без `response_raw` (T-211).
- [ ] `laws@v1` загружается, `laws_version` попадает в промпт и запись (T-205, T-218).
- [x] `FakeEncounter`/`FakeNarrator` в `develop` до I1-α (T-219, T-220); хук T-255 принят. *(ревизия 4)*
- [ ] **(ревизия 4)** T-439 закрыта: потолок нарратива записан в КД и T-203; T-215 принята после слияния T-445.
- [ ] Провайдер по умолчанию `openai_compat` реализован и покрыт httptest-фикстурами llama-server (`json_schema`, `enable_thinking=false`, `/health` 200/503, `/v1/models`) — T-208; `ollama` (T-254) **не требуется**, если F-8 подтвердил конфигурацию E.
- [ ] Блупринты содержат стартовую модель E; модели соответствуют `ops/metrics/baseline.md` **или** зафиксирован переход на следующую конфигурацию по `decision_order` ADR-005 доп. 3 — Qwen3.6-35B-A3B, C, A (T-203, T-260) *(T-440)* — стендовая часть **не блокирует** слияние I1a.
- [ ] Первая запись `solo-30.jsonl` снята на стенде (T-261) — **не блокирует** слияние I1a.
- [ ] Схемы EPIC-003 зарегистрированы, `mvctl contracts check` зелёный (T-214, T-215).

### I1 (I1a + I1b, подволна ~~1.16~~ N, ревизия 4)
- [ ] **(ревизия 4)** Контексты `llm`, `laws`, `swarm` зарегистрированы в `cmd/multiverse` настоящими (задача подключения контекстов, номер у оркестратора); в процессе один `llm.Gateway`.
- [ ] S1 `solo-30` на записях в CI зелёный; живой прогон на стенде (T-242, T-263).
- [ ] S14 `background-6h` с ускоренными тиками, фоновых вызовов ≤ B (T-243).
- [ ] S3 снапшот роя восстанавливается, рестарт → `llm_calls=0`, `identical=true` (T-236, T-237, T-238, T-243); встречи сверены с State после догона — `encounter.started/ended` дописаны с тем же id (T-424).
- [ ] Роль `encounter` целиком: T-230, T-421, T-422, T-423 приняты (нарезка T-230).
- [ ] S6 второй регион блупринтом + фикстурой, `git diff *.go` = 0 (T-241).
- [ ] `degraded`, `injections-10`, `death`, `flee-fail` зелёные (T-242, T-243).
- [ ] Golden-набор 20 ходов + 3 тика проходит (T-244).
- [ ] `/health` содержит `agents_by_level`; admin-маршруты работают (T-239).
- [ ] `gm_path` в 100 % `combat.decided`/`llm.output`; `core` не читает `MV_GM_PATH` (T-240).
- [ ] **Хук `MV_SWARM_FAKE` удалён из `cmd/multiverse`** (`fake_contexts.go` снят, исключение `depguard` снято), e2e зелёные на настоящем рое — T-256; **обязательное условие тега `mvp-1/i1`** (ADR-001 доп. п. 8, `testing/strategy.md` §5.4).
- [ ] Покрытие `internal/{swarm,llm}` ≥ 60 % (job `unit`).
- [ ] README пакетов и фрагменты runbook переданы (T-245).

### I2 (подволна 2.4)
- [ ] S2 `group-3x30`: 30 раундов, ≥ 60 действий, 0 расхождений (T-251).
- [ ] ≤ 2 вызова LLM на раунд группы (T-247).
- [ ] S6 повторно зелёный на I2 (T-251).
- [ ] S5: профиль `legacy` снят, сервисы в `services/_archive/`, флаг удалён (T-252) — **после** зелёного S2.
- [ ] Should-часть: `MemoryClient` (T-248), hot reload (T-249), лимиты (T-250) — сделаны или осознанно отрезаны на G3/G4 с отметкой в `state.js`.

---

## 8. Сводка и оценка

**(ревизия 4, 2026-09-13) Состояние.** Объём не менялся: 60 задач разработки по таблице ниже плюс T-419, T-427, T-439, которые таблица не считала. Сделано или в ревью: T-214, T-215 (`review`), T-219, T-220, T-255, T-419 (`done`), T-427 (`review`, решение архитектора). Осталось в I1: 46 задач разработки (I1a — 17, I1b — 29) и T-439. Кандидат на новую задачу — подключение контекстов — в счёт не входит до решения оркестратора. Подволн до готовности I1 — 14 (A…N) при втором слоте; без него — 15–16. Две меры дают разные сроки от старта подволны A. По подволнам (≈ 3 в неделю) — ≈ 4,5–5 недель. По объёму (40 M + 6 S ≈ 43 M-экв при 6–7,5 M-экв в неделю) — 5,7–7 недель. Разница — второй слот и пары S. Планировать по верхней мере.

Пересчитано после сведения 2 (v0.2): +3 задачи — T-254 (B3b `ollama`, условная), T-255 (хук), T-256 (удаление хука); T-208 переномерован (B3 → B3a), новых задач не порождает.

| Инкремент | Задач | Из них S / M | M-эквивалент | Подволн |
|---|---|---|---|---|
| I1a | 23 (из них 1 условная — T-254) | 3 / 20 | ≈ 21,5 (без T-254 — **20,5**) | 1.1–1.8 (8) |
| I1b | 29 (+4 — нарезка T-230) | 3 / 26 | ≈ 27,5 | 1.7–1.16 (10, с нахлёстом на I1a) |
| I2 | 8 | 3 / 5 | ≈ 6,5 | 2.1–2.4 (4) |
| **Всего разработка** | **60** (59 при конфигурации E) | 9 / 51 | ≈ 55,5 (**54,5** при конфигурации E) | 18 |
| Стенд (вне слотов) | 7 | — | — | по инкрементам |

**Оценка в неделях при трёх разработчиках.** Пропускная способность TEAM-2 — 3 разработчика × 2–2,5 M-эквивалента в неделю = 6–7,5 M-экв/нед (то же, что «≈ 3 подволны в неделю»). I1a ≈ 20,5 M-экв → 2,7–3,4 нед; I1b ≈ 23,5 M-экв → 3,1–3,9 нед; с нахлёстом подволн 1.7–1.8 **I1 ≈ 5–6 недель — оценка сведением 2 не изменилась**: три добавленные задачи — две S (T-255/T-256, ≈ 1 M-экв суммарно) и одна условная M (T-254), которая при подтверждении конфигурации E не выполняется, а при выборе C/A уходит в волну 2 (вне критического пути I1). Число подволн (17) не изменилось. I2 ≈ 6,5 M-экв → 1–1,5 нед (плюс интеграция); при выборе C/A — 7,5 M-экв.

Сверка с `decomposition-review.md` §2/§3.1 (волна 1 = 4–5 недель, ограничена EPIC-003): нарезка даёт **+0,5…+1 неделю**. Причина — дробление единиц B6/C4/R2/R4/R10/R13 до размера ≤ M и вынесение integration/e2e/golden/документации в отдельные задачи (28 «единиц работы» дизайна → 47 задач I1 с учётом сведения 2). Способы вернуться в 4–5 недель, в порядке предпочтения:
1. отдать e2e-задачи T-242/T-243 и golden T-244 `tester#2`/`qa-engineer#2` вне слотов разработчиков (−3 задачи из потока, ≈ −0,5 нед; требует, чтобы тестировщик писал Go-тесты);
2. слить T-224+T-229 и T-226+T-227 обратно в единицы R2/R4 (−2 задачи, но два L-риска против правила «≤ M»);
3. отрезать Should-часть I2 (T-248 G3, T-249 G4, T-250 G5) — не влияет на I1, освобождает волну 2;
4. отложить T-241 (S6) в I2 — критерий готовности I1 в `epics.md` придётся ослабить (нужно решение tech-lead#1).
Рекомендация тимлида: пункт 1 плюс готовность к пункту 3; пункты 2 и 4 — только по решению G3.

**Пересчёт нарезки T-230 (v0.2.2, 2026-09-11).** Добавлены четыре задачи M (T-421…T-424), +4 M-экв. Новой работы нет — она стала видна: пять поведений T-416, три уточнения ревью T-419 и правки T-425 жили внутри одной M, а сверка при восстановлении — внутри T-237. I1b ≈ 27,5 M-экв, это 3,7–4,6 недели при 6–7,5 M-экв в неделю. Внутри I1b критический путь такой: T-225 → T-226 → T-227 → T-228 → T-237 → T-242 → T-243/T-244/T-256, конец в 1.16. Сдвиг с 1.15 на 1.16 даёт зависимость T-237 от T-228, которую прежняя таблица §6 нарушала, а не нарезка. Цепочка ролей T-230…T-423 кончается в 1.13 и на путь не выходит.

**Критический путь эпика** (17 подволн по прежнему счёту; после пересборки §6 — 18):
`T-206 B1 → T-207 B2 → T-209 B4 → T-212 B6a → T-213 B6b` (поток B, 8 задач подряд у developer#2) → `T-222 R1 → T-223 R2a → T-224 R2b → T-225 R3 → T-226 R4a → T-227 R4b → T-228 R5` (рантайм, 7 задач подряд у developer#1) → `T-233 R8` → `T-242/T-243 e2e` → интеграция I1 → `T-246 G1 → T-247 G2 → T-251 G6 → T-252 G7`.
Совпадает с `design.md` §5 (`B1 → B6 → R5 → R8 → G2`) с уточнением: внутри I1b фактическое ограничение — **линейная цепочка рантайма из 7 задач**, а не роли.

**(ревизия 4)** Критический путь после пересборки — §6.1. Ранние поставки уходят контрольным слиянием эпика в `develop` (шапка индекса). T-214, T-215, T-219, T-220, T-255 уже в `develop`. Впереди — T-207 (C-07/C-15) и `recorded.Writer` T-221 (теперь в `internal/llm/providers/recorded`). Ниже — прежний текст.

**Ранние поставки другим командам** (~~сливаются в `integration/mvp-1`~~ *ревизия 4: контрольным слиянием в `develop`* по мере приёмки, не дожидаясь готовности эпика): T-214, T-215 (схемы событий — EPIC-004/005/002), T-207 (`providers/fake`, `recorded` — EPIC-002 replay, EPIC-005 golden), **T-219, T-220 (`FakeEncounter`/`FakeContext`, `FakeNarrator` — ранний merge подпакета `shared/testkit/swarm`; EPIC-004 I1-α, EPIC-002 e2e)**, T-221 (`RecordingWriter` — EPIC-005), **T-255 (хук `cmd/multiverse/fake_contexts.go` — PR в путь EPIC-001 через tech-lead#1)**. Ранний merge `shared/testkit/swarm` **закреплён сведением 2** (C-05 (contracts v0.3+): «подпакет вливается в `integration/mvp-1` сразу после приёмки tech-lead#2, не дожидаясь остального I1a»); на G3 остаётся подтвердить лишь порядок веток (см. §9 риск 8).

**Состав MVP-1 от EPIC-003**: Must — все задачи I1a и I1b (включая T-255, T-256) + T-246, T-247, T-251, T-252 из I2. Отрезаемо на G3/G4 без потери Must-историй: T-248 (G3, зависит от 005-memory), T-249 (G4, FR-091 Should), T-250 (G5, лимиты Should). **Условная**: T-254 (B3b `ollama`) — выполняется только при выборе конфигурации C/A на F-8; при E закрывается как «не требуется» (US-014 «переключение провайдера без перезапуска ядра» держится на реестре провайдеров T-206 + `openai_compat`/`fake`/`recorded`).

---

## 9. Риски нарезки

| # | Риск | Реакция |
|---|---|---|
| 1 | Линейная цепочка рантайма I1b (7 задач у одного разработчика) — узкое место, два других потока могут простаивать в подволнах 1.12–1.13 | роли и контекст спланированы так, чтобы закрывать все три слота до 1.13; при опережении свободный слот TEAM-2 отдаётся TEAM-1/TEAM-3 (`teams.md` §4); при отставании — перенос T-241/T-244 на developer#1 |
| 2 | Число задач (56 после сведения 2, из них 1 условная) выше оценки декомпозиции (~35) → срок I1 5–6 нед. вместо 4–5 | §8, меры 1–4; контроль по подволнам: 3 подволны в неделю; отставание на 2 подволны → эскалация tech-lead#1 на G3 |
| 3 | `Behaviour`/`Emitter` (T-223) фиксируют интерфейс, от которого зависят четыре задачи ролей | интерфейс принимается тимлидом отдельно от остальной задачи; изменение после 1.9 — только задачей тимлида с уведомлением потока ролей |
| 4 | 005-memory отрезается на G3 → T-248 теряет смысл | задача сводится к проверке `NopMemory` + сохранению интерфейса (объём I2 уменьшается на M); Must-часть US-005/US-036 держится на `journalContext` (T-234) |
| 5 | EPIC-002 (`Resolve`, реальный State) приходит позже T-230/T-242 | до слияния — `FixedMechanics`/`FakeState`; после интеграции e2e перезапускаются на реальных (дефекты — задачей в эпик-владелец, `epics.md` §5 п. 6) |
| 6 | Записи LLM (T-261/T-262) не сняты вовремя → e2e I1b нечем кормить | e2e пишутся против `FakeProvider` и включают `recorded` вторым режимом; стендовая задача не блокирует приёмку задач, блокирует только критерий S1 «живой» |
| 7 | Раздел `admin` в `api/gateway.openapi.yaml` принадлежит EPIC-004 — гонка правок | T-239 отдаёт текст, файл не правит; задача на вставку — у tech-lead#3; проверка «OpenAPI ↔ маршруты» остаётся у EPIC-004 |
| 8 | ~~Ранний merge `shared/testkit/swarm` вне порядка 002 → 004 → 003~~ **(ревизия 4)** Контрольное слияние эпика в `develop` приносит другим командам незаконченный `internal/**` EPIC-003 | ~~сведение 2 закрепило ранний merge подпакета…~~ **(ревизия 4)** `internal/**` в процессе не участвует, пока контекст не подключён (задача подключения, подволна L): до неё `cmd/multiverse` регистрирует заглушки. Контрольное слияние — только при зелёном `make ci`. T-219, T-220, T-255 уже в `develop` |
| 8а | **(ревизия 4)** Правки `.golangci.yml` (T-445), раскладка регистрации (T-446) и тексты контрактов (T-444) в EPIC-001 опаздывают к подволнам F–L | условия старта в §6.1; tech-lead#2 сверяет их перед каждой подволной; при задержке T-445 — сдвиг T-226/T-228/T-233/T-235, при задержке T-446 — подволны L; эскалация — оркестратору |
| 9 | Строки уровней роя в `contracts.OwnershipRules` отстают от блупринтов: блупринт объявляет `owned_entity_types`, которых нет в строке его уровня | валидатор блупринтов (T-202) сверяет `owned_entity_types` с видом, собранным вызывающим из `shared/contracts/ownership.go`, и падает на первой несостыковке. Строку правит отдельный PR в `shared/contracts` с меткой `contract-change`, ревью system-architect + tech-lead#1, автор перечисляет затронутые блупринты (ADR-025 «Подтверждение», §16 п. 6). *(Переписано T-416, 2026-09-11: прежняя редакция — «тест равенства `levels.go` ↔ копия, копию правит TEAM-2 тем же PR» — отменена ADR-025.)* |
| 10 | Задачи-документация (T-204 частично, T-245, T-253) сдвигаются как «неважные» | они входят в чек-листы готовности инкрементов; инкремент не принимается без них |
| 11 | **(сведение 2)** F-8 задерживается → неизвестно, нужна ли T-254 (B3b `ollama`) | T-254 вне критического пути и вне I1: до решения F-8 не стартует; при E закрывается как «не требуется»; US-014 обеспечивается реестром провайдеров (T-206) и `openai_compat` — переключение проверяется парой `openai_compat`/`fake` |
| 12 | **(сведение 2)** Обновление билда llama.cpp меняет API/поведение грамматики (#20345 и смежные) → падают httptest-фикстуры или живой прогон | фикстуры T-208 снимаются с зафиксированного билда; пин билда — справочная запись `build/versions.env` (devops, запрос в отчёте); при расхождении — задача на обновление фикстур, а не «подправить парсер»; `strip_think` в B4 остаётся страховкой |
| 13 | **ЗАКРЫТ (ревизия 4: T-255 принята, хук в `develop`).** **(сведение 2)** `cmd/multiverse/fake_contexts.go` — чужой путь владения; PR может не успеть к I1-α | задача T-255 (S) ставится в подволну 1.4 и согласуется с tech-lead#1 заранее (запрос отправляется вместе с приёмкой T-219); запасной вариант на время ожидания — включение `FakeContext` в e2e напрямую (без бинарника), I1-α через бота при этом не проверяется — эскалация tech-lead#1 |

---

## 10. Замечания к дизайну и контрактам (файлы архитекторов не правятся; передано в отчёте)

1. **РЕШЕНО (сведение 3, TL2-1; внесено tech-lead#1).** `validation_status` — единый enum из **6 значений** `valid|partially_rejected|invalid|error|quarantined|filter_error`; причины — только в `llm.output.rejected.reason`; `budget_exceeded` — не статус; опц. `llm.output.reasons[]` (C-07 v1.2, ADR-017 доп. 1). Внесено в T-215; US-018 правит BA. Ниже — исходная формулировка замечания. ~~**`validation_status`: два разных перечисления.**~~ `data-model.md` §7.2 задаёт 11 значений (включая `rejected_unknown_entity`, `rejected_player_agency`, `rejected_level_violation`, `budget_exceeded`), а КД §9.2/§10.2 и ADR-017 используют 7 (`valid|partially_rejected|invalid|quarantined|filter_error|rejected_language|error`), перенося `unknown_entity`/`player_agency`/`level_violation` в `llm.output.rejected.reason`, а `budget_exceeded` — в событие `rejected` **без** `llm.output`. US-018 повторяет вариант `data-model`. Требуется одно решение до T-215 (system-analyst + system-architect). **Статус после сведения 2: передано в сведение 3** — `system-architect` работает над ним параллельно (журнал, «A4 волна 3a»); DoD T-215 ссылается на решение сведения 3 без собственного содержания. Ответ нужен к подволне 1.2.
2. **РЕШЕНО (сведение 3, TL2-2).** `contracts.md` v0.4 §0 и §16 п. 7: «тип — владелец схемы; издатели — список `Spec.Publishers` реестра»; job `contracts` проверяет `source ∈ Spec.Publishers`. Реестр с `Publishers` — EPIC-001 T-006. Исходно: **Фактический издатель `dice.rolled`.** `contracts.md` §0 закрепляет тип за EPIC-002, публикует его агент встречи EPIC-003 и `FakeEncounter`. Нужна явная оговорка в C-03/§0, иначе проверка политик топиков в job `contracts` может счесть публикацию роем нарушением.
3. **ОТМЕНЕНО ADR-025 (подтверждён system-architect#1 в T-416, 2026-09-11; `contracts.md` §16 п. 6 v0.6–v0.7).** Единственная истина таблицы владения — `shared/contracts/ownership.go`; копии, теста равенства и строки gateway в `levels.go` нет; T-202 переформулирована (T-416). Ниже — решение сведения 3 для истории. ~~**РЕШЕНО (сведение 3, TL2-3).**~~ `contracts.md` v0.4 §16 п. 6: истина — `levels.go` (TEAM-2), копия `shared/contracts/ownership.go` правится **тем же PR** владельцем истины (метка `contract-change`, ревью tech-lead#1 + system-architect, блокер); тест равенства блокирует merge; State — потребитель. Внесено в T-202. Исходно: **Процедура при расхождении `levels.go` ↔ `contracts.OwnershipRules`.** Тест равенства (T-202) падает, но `shared/contracts` принадлежит EPIC-001/system-architect. Нужен пункт процедуры: кто и в какой волне правит копию (предложение — задача EPIC-001 по запросу TEAM-2, приоритет блокера).
4. **US-014 и NFR-076 против решения по моделям — решено сведением 2 (запрос h, статус BA).** Критерий «обе модели загружены одновременно, проверка `ollama ps`» заменяется на «**модели по `ops/metrics/baseline.md` (конфигурация E/C/A), переключение провайдера `openai_compat`/`ollama` без перезапуска ядра**»; формулировку в `user-stories.md`/`nfr.md` вносит business-analyst (`consolidation.md` §13). DoD T-203/T-260 уже сформулированы «по `baseline.md`»; **если BA не внесёт правку до приёмки T-203, задача принимается по формулировке сведения 2**, расхождение — в отчёт tech-lead#1.
5. **`encounter.started` в заглушке — закрыто сведением 2.** `contracts.md` v0.3 §0 добавил исключение из правила «публикует только владелец типа»: фейки `shared/testkit/*` в режиме e2e/I1-α (`MV_SWARM_FAKE=true`) публикуют типы подменяемого контракта; хук в `cmd/multiverse` удаляется в I1 (T-256). Действий не требуется; T-214 проверяет, что contract-тест учитывает исключение.
6. **РЕШЕНО (сведение 3, TL2-6).** `FakeState` v0 **публикует** `analytics.replay.completed {mode: recovery}` при старте (EPIC-001 T-017); рой **ждёт с таймаутом** `MV_SWARM_REPLAY_WAIT=120s` → `/health degraded {state_replay: missing}`. Вариант «не блокировать старт» отменён; внесено в T-237. Исходно: **Старт догона роя без `analytics.replay.completed`.** design §13 п. 7 требует ждать событие от State, но `FakeState` v0 (F-10) его не публикует — до слияния EPIC-002 рой не стартует. Нужно уточнение: таймаут ожидания или публикация события заглушкой (запрос к architect#1/EPIC-002). В T-237 заложено «не блокировать старт» как временное решение.
7. **ПОДТВЕРЖДЕНО (сведение 3, TL2-7).** Решение тимлида принято: файл схемы — полный набор типов MVP-1 для уровня, рантайм при компиляции подставляет пересечение с `allowed_event_types` блупринта, валидатор проверяет `allowed ⊆ enum файла`. T-203 без изменений. Исходно: **`enum` в `schemas/agent/tick-*.json`.** КД §13.4 говорит «`enum` = пересечение `allowed_event_types` блупринта», но файл схемы один на роль. Принято решение тимлида (зафиксировано в T-203): файл содержит полный набор MVP-1, сужение по блупринту делает рантайм при компиляции схемы. Просьба подтвердить architect#2.
8. **ЗАКРЫТО (сведение 3, TL2-8).** `infrastructure.md` §1 п. 8 уже предусматривает CLI на хосте: `mvctl laws bump | world init | record` → `MV_KAFKA_BROKERS=127.0.0.1:19092` (external listener); раздел runbook «CLI на хосте» — за devops. В задачах TEAM-2 использовать имя **`MV_KAFKA_BROKERS`**. Исходно: **`mvctl laws bump` публикует в шину.** Для рабочего стенда CLI нужен доступ к Redpanda (адрес/учётные данные) — вопрос к devops для runbook; в дизайне не описано.
9. **ЗАКРЫТО (сведение 3).** Правильная ссылка — **«C-05 (contracts v0.3+)»**: контракт остаётся v1.1, изменение касается блока «Заглушка» (`FakeEncounter` — единственная заглушка боя, `WithEncounterStub` в F-10 не делается). Все ссылки в этом файле заменены. Исходно: **Обозначение версии C-05.** В поручении и в этом файле заглушки C-05 обозначены как **v1.2**, но `contracts.md` v0.3 маркирует контракт как «Версия 1 (v1.1 — опциональные поля)», а блок «Заглушка» помечен «v0.3 контрактов»; истории изменений C-05 записи «v1.2» нет. Просьба к system-architect в сведении 3 — либо проставить C-05 **v1.2** (изменение заглушек: `FakeEncounter` — единственная заглушка боя, `WithEncounterStub` снят), либо подтвердить, что правильная ссылка — «C-05 v1.1, заглушки `contracts.md` v0.3». Задачи T-219/T-220/T-255/T-256 ссылку поправят одной правкой, содержание не меняется.
10. **(ревизия 4) Закрыто для T-255, переформулировано для T-256.** Хук принят и в `develop`. Правка `.golangci.yml`, `shared/env/vars.go` и `shared/contracts/sources.go` при удалении хука — задача EPIC-001 по запросу tech-lead#2 (раздел T-256). Файлы `cmd/multiverse` после T-446 — файлы владельцев. Прежний текст: **`MV_SWARM_FAKE` и `.golangci.yml` — на стороне EPIC-001.** Хук (T-255) и его удаление (T-256) правят `cmd/multiverse/**` и `.golangci.yml` (владение EPIC-001). Нужен встречный пункт в `tasks.md` EPIC-001 (F-2/F-10 и `.golangci.yml`-исключение по файлу) — запрос tech-lead#1; без него задачи TEAM-2 не смогут быть приняты (PR некуда влить).
11. **(ревизия 4) Решено T-435, ADR-005 доп. 3:** E и Qwen3.6-35B-A3B — на `llama-server`, Ollama — только C/A; условие T-254 — в её разделе. **Замер F-8 определяет судьбу T-254.** Просьба к tech-lead#1: в F-8 зафиксировать в `ops/metrics/baseline.md` явную строку «провайдер: `openai_compat` | `ollama`» — она является входом для решения «делать ли B3b» и для T-203/T-260.
12. **(ревизия 4) Устарело.** Номера берутся только из общего счётчика `counters.task` (на 2026-09-13 — 446), их выдаёт оркестратор; диапазоны команд не действуют (шапка индекса). Прежний текст: **Счётчик `counters.task` в `.dev-team.json` не изменён** намеренно: три тимлида нарезают задачи параллельно в непересекающихся диапазонах; сведение счётчика (`task = max` по трём эпикам) — за tech-lead#1 после G3, чтобы не получить гонку записи. На 2026-09-09 в файле `counters.task = 148`; EPIC-003 занял T-201…T-256 и T-260…T-266 (максимум **266**).
13. **(ревизия 4) Отменено указанием пользователя 2026-09-13** «три команды одновременно»: `parallelism.maxTeams = 3`, `maxAgentsPerRole = 3`, TEAM-2 ведёт EPIC-003 параллельно с TEAM-1 и TEAM-3. Волны §6.1 рассчитаны на трёх разработчиков. Прежний текст: **Указание пользователя от 2026-09-09 «одна команда одновременно»** (журнал, `parallelism.maxTeams = 1`) в этой версии **не разворачивалось**: файл описывает нарезку на 3 разработчиков TEAM-2. При переходе к последовательной работе одной командой волны §6 читаются как **порядок** задач (17 подволн → ~50 M-экв одним-двумя разработчиками), а ранние merge и часть заглушек (кроме нужных точке I1-α) становятся необязательными. Пересборка волн под `maxTeams=1` — отдельная правка после решения PM/G3, чтобы не смешивать её со сведением 2. Замечание: `.dev-team.json` на момент правки всё ещё содержит `maxTeams: 3` — расхождение с журналом, вопрос оркестратору.
14. **РЕШЕНО (architect#2, 2026-09-11; design §14.1): `Has`/`Add`.**
    - Id всех событий ответа выводятся из причины: у `dice.rolled` — индекс броска, у `combat.decided` — `exchange.index`, у пакета — номер попытки.
    - Ответ собирается один раз и допубликовывается по курсору.
    - Постоянный отказ шины → `dead_letters` после предела допубликования (4).
    - Остановка посреди ответа не теряет действие через снапшот. С `Seen` окно попало бы в снапшот остановки, и действие было бы погашено после рестарта.
    - Внесено в T-230, T-421, T-422, T-236, T-246.
    - Контракты не меняются. У system-architect запрошена редакционная правка примера «(агент встречи: повтор задвоил бы кости)» в C-01 «Дедуп» и ADR-027 п. 3; T-230 она не блокирует.

    Ниже — исходная формулировка. **(из T-415, 2026-09-11) Вопрос architect#2: окно действий агента встречи — `Seen` или `Has`/`Add`.** ADR-027 п. 3 оставляет встрече `Seen` до обработки, потому что повтор задвоил бы кости. Цену показал двойник (T-415). Шина отказала в публикации решения или пакета, а действие уже в окне. Повтор шины отвечает `nil`, письма в `dead_letters` нет, остаётся только след в логе и на `/health`. У настоящего агента кости детерминированы по `causeEventID` (NFR-060, DoD T-230). Если и решения обмена (`dice.rolled`, `combat.decided`) будут получать id из причины (`WithCauseID`, `parts` — номер решения), повтор полуопубликованного хода опубликует те же события с теми же id. Потребители погасят дубли, и `Has`/`Add` станет безопасным: постоянный отказ закончится в `dead_letters`. Нужно решение: оставить `Seen` со следом отказа или перейти на `Has`/`Add` с id решений из причины. При переходе — правка ADR-027 п. 3 и C-05, влияние на T-230 и T-421. Срок — до старта T-230 (подволна 1.10). Двойнику `FakeEncounter` это не нужно: он снимается T-256 (решение tech-lead#2 по T-415).

### T-419: Двойники по C-05 v1.4: встреча после факта, признак конца обмена, окно нарратора на Dedup · Размер: M · Статус: **done** *(ревизия 4; было todo — принята tech-lead#2, итерация 3 сверена по диффу, мутанты убиты; код в `develop` с `1c2ee7e`; подробности — `tasks/T-419.md`)* · Волна 1
- **Причина (T-416, ADR-026, ADR-027)**: `FakeEncounter` публикует `encounter.started`/`encounter.ended` до факта State и берёт NPC второй активной встречи — это нарушает C-05 v1.4 п. 4–6 и `data-model.md` §3.7; нарратор держит своё окно вместо `Dedup.Has`/`Add`.
- **Что сделать**: начало встречи — после `entity.created`, конец — после факта закрывающего пакета; «один NPC — одна активная встреча»; закрытие по чужому факту без `killer`; откат участия при выброшенной попытке (п. 1г); поле `combat.decided.exchange{index,last}` (схема — владелец EPIC-003) и закрытие хода нарратором по `exchange.last`; окно нарратора на `Dedup` (после T-417). Харнесс в `shared/testkit/gateway` — совместно с его владельцем.
- **DoD**: пять расхождений двойника с v1.4 из блока «Заглушка» закрыты тестами; стенды пары и нарратора зелёные. Срок — до T-230: иначе исполнитель T-230 возьмёт неверный образец.

#### T-427 · Вопрос архитектору: Действие до encounter.started: окно до подъёма агента встречи
Инкремент I1b · до подволны 1.12 · architect · Размер S · Статус review *(ревизия 4: так и в карточке; в `state.js` стоит `todo` — расхождение передано оркестратору. Решение внесено в design §14.2, ADR-028 и DoD T-223, T-225, T-229, T-232, T-422. Не хватает только записи «Приёмка (оркестратор)» в карточке. После неё — `done`)* — **решено architect#2 (2026-09-11): design §14.2, ADR-028.** Агента встречи поднимает решение GM региона открыть встречу — внутри процесса, до публикации предложения. Новая задача рантайма не нужна: работа легла в T-223, T-225, T-229, T-232, T-422. Карточка — `tasks/T-427.md`.
- **Причина (нарезка T-230, tech-lead#2, 2026-09-11)**: действие игрока, пришедшее раньше `encounter.started`, не получает никто — агента встречи ещё нет, а GM региона на действия не подписан (КД §5.2). Решение T-425 п. 5 (очередь держит агент встречи) закрывает только окно после подъёма агента.
- **Варианты**: (а) поднимать агента встречи по предложению создания встречи, а не по `encounter.started` — правка КД §4.1/§5.1 и триггера блупринта; (б) удерживать действие в Router до `agent.spawned` — отдельная задача потока рантайма; (в) иное с обоснованием.
- **DoD**: решение с доводами, формулировка в C-05 или дизайне EPIC-003, влияние на T-230…T-232, T-422, T-424; если нужна новая задача рантайма — назвать, номер выдаст оркестратор.
- **Срок**: до старта T-422 (подволна 1.12).
- **Исполнитель**: architect (EPIC-003) или system-architect, если решение меняет контракт.

## 11. Бэклог двойников `shared/testkit/swarm` (без номера; номер выдаёт оркестратор при взятии в работу)

1. **(из T-415, 2026-09-11; ревью #1 T-415 п. 3) `FakeNarrator`: `Error` на чистой остановке.** Публикация нарратива, прерванная остановкой (`ctx.Err() != nil`), пишет `Error` «narrative not published» (`fake_narrator.go:815-817`). Это ложный след, как Mi-1 ревью #1 T-415 у встречи; `Err()` нарратора он не трогает. Сделать по образцу `FakeEncounter.refusedPublication`: при отмене — `Info`, обработчику возвращается прежняя ошибка. DoD: тест «публикация под отменённым контекстом → в логе нет `ERROR`, есть строка про остановку»; мутант без ветки отмены краснеет. Размер S, приоритет низкий: двойник снимается T-256.
2. **(из T-415, 2026-09-11; подтверждение tech-lead#2, мутанты MH и MS2) `FakeContext`: `/health` на отказ встречи не закреплён тестом.** Мутант «`Health` не читает `encounter.Err()`» выживает в `shared/testkit/swarm` и `cmd/multiverse`. Дыра тянется с T-219: отказ подписки встречи на `/health` тоже не проверен, единственный тест идёт через нарратора. Кроме того, doc `Err()` обещает первую ошибку, а мутант «последняя ошибка» выживает. DoD: тест `FakeContext` — отказ публикации встречи и отдельно отказ подписки встречи → `Health().Status == fail`, в `Details["err"]` текст отказа; тест на две ошибки подряд → `Err()` возвращает первую. Оба мутанта краснеют. Размер S.

## 12. Задачи из ревью других эпиков

Номерные задачи EPIC-003, которые выросли из ревью и приёмки задач других эпиков. В подволны §6.1 они не входят. Срок у каждой — от задачи другого эпика, которую она разблокирует.

### T-452: `FakeEncounter` — павшему персонажу только `hp` и `status` · Размер: XS · Статус: **done** *(принята tech-lead#2 2026-09-13, итераций ревью 1; подробности — `tasks/T-452.md`)* · Вне подволн §6.1, до T-062 EPIC-002 (из ревью T-053)
- **Причина (T-053, расхождение Р3; решение system-architect#1 от 2026-09-13 — «Р3 — дефект двойника»)**: `wound` в `shared/testkit/swarm/fake_encounter.go` писал павшему **персонажу** `died_at` и `killed_by`.
  - У персонажа таких атрибутов нет (`data-model.md` §3.3; оба — атрибуты NPC, §3.4). Строка владения `task`/`player` (`shared/contracts/ownership.go`) таких путей не даёт.
  - Двойник State таблицу владения не проверяет, поэтому дефект молчал. Настоящий State (T-056) отклонит весь пакет с `level_violation`, и e2e T-062 на `FakeEncounter` покраснеет.
  - Существующий `TestEveryProposalPassesTheOwnershipTable` смерть персонажа не прогонял, а `TestTheCharacterDies` закреплял ошибку (`killed_by` у персонажа).
- **Что сделать** (`shared/testkit/swarm`):
  - павшему персонажу — только `hp` и `status`; у павшего NPC — как было: `hp`, `status`, `died_at`, `killed_by`, `loot_claimed_by`, трофей победителю;
  - тест: пакеты прогона со смертью персонажа (удар в обмене — `cause=combat`, свободная атака после неудачного побега — `cause=flee`) целиком проходят `contracts.OwnershipRules()`, набор изменений павшего персонажа — ровно `[hp status]`;
  - устаревшие комментарии «once EPIC-002 implements Resolve (T-053)» (`fake_encounter.go:132-133`, `:156`) — нейтрально: механика EPIC-002.
- **Не делать**: переход на `ChangesFor` и форму `changed[]` для `remove` — после T-053/T-448. `ownership.go` и схемы не менять.
- **DoD**:
  - `TestAFallenCharacterIsProposedOnlyWhatTheOwnershipTableAllows` (2 подтеста) и обновлённый `TestTheCharacterDies` зелёные; `TestTheWolfDies` дополнительно закрепляет `died_at` у NPC;
  - мутанты в копии дерева в scratch, без `-overlay`, контрольный первым: возврат дефекта, `killed_by` или `died_at` у персонажа, лишний разрешённый путь, нет `status`, нет `died_at`/`killed_by` у NPC — красные;
  - `go build ./... && go vet ./...`, `go test -short -count=1 ./...`, `go test -tags e2e ./test/e2e/...`, `golangci-lint run ./...`, `make test` зелёные; dev-log заполнен.
- **Метка**: `contract-change` не нужна — контракт, схемы и таблица владения не меняются; двойник приводится к ним.
- **Исполнитель**: TEAM-2/developer#2. Ветка `task/T-452-fake-encounter-fallen-character` (от эпика, `b18ecd6`). Подробности — `tasks/T-452.md`.

## 13. Бэклог схем событий (из приёмки T-214 и T-215; без номера — номер выдаёт оркестратор при взятии в работу)

Замечания ревью #1 T-214/T-215 (`review.md`, раздел «T-214 + T-215 · ревью #1»), которые не закрыли T-444 и T-445 и не ушли строкой в DoD другой задачи. Ни одно не блокирует задачи эпика.

1. **(Mi-2, N-3) `shared/contracts`: тест шаблона `provider` и `SourceLLM` в потребителях записей.** Путь EPIC-001 (`shared/contracts` меняет только EPIC-001). Размер XS.
   - В `TestBlockVPayloadRejects` (`blockv_test.go:253`) — два отрицательных случая: адрес в `llm.output.provider` и в `config.cloud_enabled.provider` (`http://127.0.0.1:8888`, `openai_compat?key=…`, `OpenAI`); положительный цикл по пяти именам `openai_compat|ollama|anthropic|recorded|fake`. Сейчас удаление шаблона оставит все тесты зелёными.
   - `registry.go:131–136`: добавить `SourceLLM` в `Consumers` `llm.output` и `llm.output.rejected` — записи читает провайдер `recorded` в контексте `llm` (§2.2). Изменение реестра — метка `contract-change`, через system-architect#1.
2. **(Mi-3а, N-2) `test/fixtures/events_test.go`: дубли словарей и уровень агента конверта.** Размер XS.
   - Удалить `validationStatuses`, `TestValidationStatusIsTheSixValuesOfTheDecision`, `TestRejectionReasonsAreOneDictionary` (`:347–391`) — их держат `TestValidationStatusEnum`/`TestRejectionReasonEnum` в `shared/contracts/blockv_test.go`. В тексте пропуска `TestRejectionReasonsMatchTheGuardianPackage` печатать `schemaEnum(...)` вместо `rejectionReasons`.
   - Конверт фикстур (`:274–281`): уровень и блупринт агента брать из `payload.agent`, если есть, иначе по таблице «топик/префикс → уровень» (`tick.*`, `world.*` — `global|domain`, §2.3.8). Сегодня политика уровень не читает, ложного прохода нет.
3. **(Mi-5) Словари `tick.aborted.reason` и `narrative.output.fallback_reason` — сужение до enum** в задачах, которые закрепляют их в коде: T-227 (`tick.aborted`), T-229 (нарратив ролей). Совместимое изменение по §16 п. 1, уведомить system-architect#1.
4. **(N-4, бэклог ревью п. 2) Правило «пара фикстур на каждый путь»** — отдельной задачей после фикстур EPIC-002 и EPIC-004; тогда же валидная фикстура `encounter.ended` без `killer` (C-05 v1.4 п. 6).
5. **(бэклог ревью п. 4) `agent.id` в фикстурах — детерминированная форма** после R3 (T-225).

Не переносятся: Mi-7 T-009 — общий `$defs.EventRef` добавлен T-445, встроенные копии формы в прежних схемах оставлены сознательно (`schemas/events/_common.json:57`). N-5 — расхождение «девять/восемь» только в тексте dev-log T-215 §2; записи dev-log — история исполнителя, не правятся.
