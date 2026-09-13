# EPIC-003 «Рой GM, LLM-шлюз, страж, законы» — дизайн эпика

Версия 0.3a · 2026-09-11 · architect#2 (TEAM-2), 0.3a — system-architect#1; v0.2 — правки сведения 2 внесены tech-lead#2; **v0.3 — решения architect#2 по §10 п. 14 и T-427 индекса, раздел §14**; **v0.3a — проверка system-architect#1 (T-431): §14.5, уточнения без переписывания §14.1–§14.4** · статус: к нарезке tech-lead#2 (Flow A, A4 шаг 2). Ветка эпика: `epic/EPIC-003-swarm-llm-laws` от `integration/mvp-1` (после `mvp-1/wave-0`).
Основание: `architecture/components/swarm-llm-laws.md` v0.2 (далее — **КД**, детальная архитектура блока; здесь не дублируется), ADR-002/005 (+ «Дополнение 2»)/008/010 (с дополнениями), ADR-001 доп. п. 7–8, ADR-014…017, `architecture/contracts.md` v0.3 (C-15 v1.1, C-05 заглушки), `architecture/consolidation.md` §4/§9 и **§10–§13 (сведение 2)**, `architecture/overview.md` §18.1, `plan/epics.md` v0.3 (EPIC-003, §2 I1-α, §5), `plan/decomposition-review.md` §2.1/§3.3, `plan/teams.md` §4, `journal.md` (решения G2; «Сведение 2 завершено»), `requirements/user-stories.md` v0.3, `analysis/use-cases.md`.

### Changelog

| Версия | Дата | Изменения |
|---|---|---|
| 0.1 | 2026-09-09 | Первая редакция (architect#2). |
| **0.2** | **2026-09-09** | **Сведение 2, внесено tech-lead#2** (architect#2 не запущен; основание — `consolidation.md` §13 строка «architect#2 / tech-lead#2»): §3.1 **B3 → B3a `openai_compat` (llama-server) первым + B3b `ollama` вторым**; §3.1 B1 — поля `Params`/`Response`, `MV_LLM_URL`/`MV_LLM_API_KEY`, гейт облака по host; §3.1 A3, §6, §11 п. 3 — стартовая модель **E `Qwen3.8-27B-UD-Q3_K_XL`, `thinking: false`**, запасные C/A по `baseline.md`; §4.1 п. 1 и §13 п. 1 — хук `MV_SWARM_FAKE` читает **`cmd/multiverse` (`fake_contexts.go`)**, `FakeContext` как `runtime.Context`, удаление хука — критерий I1; §5 схема (B3a/B3b); §8, §9 — llama-server вместо Ollama как основной рантайм стенда; §10 риск 2 — **llama.cpp #20345**; §10 риск 3/9 — переформулированы под E. Содержательных решений архитектора не добавлено — только перенос принятых в сведении 2. |
| **0.3** | **2026-09-11** | **architect#2: новый §14 — два решения уровня реализации.** §14.1 (§10 п. 14 индекса): окно действий агента встречи — `Dedup.Has`/`Add`; id из причины у всех событий ответа; ответ собирается один раз и публикуется по курсору; постоянный отказ шины → `dead_letters`. §14.2 (T-427): агента встречи поднимает решение GM региона открыть встречу, внутри процесса и до публикации предложения (**ADR-028**). §14.3 — влияние на задачи, §14.4 — риски. Контракты не меняются; у system-architect запрошена редакционная правка примера в C-01 «Дедуп» и ADR-027 п. 3. Диаграммы: `seq-swarm-llm-laws-encounter-opening.md`, `seq-swarm-llm-laws-answer-publish-retry.md`. |
| 0.3a | 2026-09-11 | **system-architect#1 (T-431): §14.5 — проверка решений §14 на соответствие контрактам.** Подтверждено с четырьмя уточнениями, текст §14.1–§14.4 не переписан. Запрос §14.1 выполнен: C-01 v1.6, C-05 v1.6, ADR-027 «Уточнение исполнения 2», ADR-028 «Уточнение исполнения». Итерация 2 (ревью #1 T-431): §14.5 п. 1 дополнен, строки DoD внесены в индекс EPIC-003, КД §5.1 исправлена. |

---

## 1. Цель и границы

**Цель.** Игрок получает от роя GM на Agent GM Core: (а) механически честный бой (Phase 1 без LLM), (б) нарратив на русском от персонального GM / нарратора группы через LLM-шлюз с записью каждого вызова, стражем и фильтром (a), (в) живой мир — фоновые тики глобального и регионального GM в бюджете B/час; всё воспроизводимо в replay без LLM и описано данными (блупринты, законы, правила), а не Go-кодом.

**Входит** (по `contracts.md` §0, `ownership.md`): `internal/swarm`, `internal/llm` (+`guardian`, `prompt`, `providers`, `filter`, `parser`), `internal/laws`, `shared/agent` (типы, парсер, валидатор, `levels.go`), `blueprints/*.md` (5), `laws/dark-forest-world.v1.yaml`, `schemas/agent/*.json`, `schemas/events/<типы EPIC-003>.v1.json` (26 типов), `config/absolute-limits.yaml`, `config/llm-prices.yaml`, `shared/testkit/swarm` (`FakeNarrator`, `FakeEncounter`, `RecordingWriter`), `cmd/mvctl/internal/{record,blueprint,laws}`, `testdata/{recordings,llm,golden}`, admin-маршруты `/v1/admin/agents*`, `/v1/admin/llm/usage`; `services/narrative-orchestrator` + `services/semantic-memory` — только as-is в профиле `legacy` до S5.

**Не входит**: State/механика (`internal/state`, `internal/mechanics`, `rules/*.yaml`, `mvctl world init` — EPIC-002); действия игрока, раунды, доставка, бот (EPIC-004); память C-09 и `mvctl report`/golden-набор как CLI (EPIC-005); HTTP-сервер процесса, шина, реестр, `shared/clock`/`runtime`/`objstore` (EPIC-001); фаза пробоя законов (EPIC-007), Entity-Actor/Monitor (EPIC-006), облачные провайдеры как рабочая опция (EPIC-013), адаптивный LOD (EPIC-012). Первичное создание мира из блупринтов (`swarm.InitWorld`) — не реализуется (решение G2).

**Инкременты**: I1a → I1b → I2; точка I1-α («соло на шаблонах через бота») — вклад EPIC-003 в неё минимален и описан в §4.1.

---

## 2. Трассировка требований

| US | FR / BR / NFR | UC | Инкремент | Компоненты (КД) |
|---|---|---|---|---|
| US-002 соло 30 ходов | FR-010…013, FR-120…123, BR-05, BR-10 | UC-004…008, UC-010 | I1b (сквозная приёмка — интеграция I1) | Router/Lifecycle §5–6, roles `encounter`/`personal-gm`/`region-gm` §8, journalContext §11.2 |
| US-004 нарратив асинхронно, деградация | FR-013, FR-015, FR-050…052, BR-14, NFR-001/002 | UC-020, UC-021 | I1a (шлюз, деградация) + I1b (роль, очереди) | Gateway §9, template §8.4, Scheduler §7 |
| US-011 снапшот роя, replay | FR-031…033, FR-087, NFR-010…014 | UC-025, UC-026 | I1b | Snapshot §4.3, `providers/recorded` §9.1, догон §4.3 |
| US-012 контроль LLM, бюджеты | FR-070…072, FR-125, NFR-006/007/050…053 | UC-024 | I1a (учёт, бюджет шлюза) + I1b (`BackgroundBudget`, `lod_allowed`) | budget/usage §9.3, §7.3 |
| US-014 провайдер/модели | FR-070, BR-15, NFR-046, NFR-076 | UC-030 | I1a | providers §9.1, config, `config.cloud_enabled` |
| US-015 регион из блупринта без кода | FR-090…092, FR-120, FR-124, NFR-083 | UC-029 | I1a (формат, валидатор, CLI) + I1b (спавн, S6) + I2 (FR-091 hot reload, Should) | shared/agent §13, Registry §2 |
| US-016 фильтр выхода, абсолютные запреты | FR-050, FR-051, FR-053, BR-08 (`InputFilter` FR-056 — EPIC-004) | UC-023 | I1a | filter §9.2/§13.5, prompt `<absolute_limits>` §11.4 |
| US-017 воспроизводимый прогон | FR-021, FR-032, FR-100, NFR-060/061 | UC-026 | I1a (`recorded`, `RecordingWriter`, `mvctl record`) + I1b (e2e replay) | §9.1, §18 |
| US-018 `llm.output` как события | FR-032, FR-034, FR-045, BR-04, NFR-021/022 | UC-022 | I1a | record §9.2, guardian §10, laws_version §12 |
| US-019 фича-флаг `MV_GM_PATH` | FR-014, BR-12 | UC-034 | I1b (`gm_path` из `meta`, deprecated-фильтр) → I2 (S5, архив) | §17 |
| US-036 мир живёт между сессиями | FR-037, FR-124…128, BR-10, BR-16, NFR-007/053 | UC-018, UC-019, UC-004 (сводка) | I1b | Scheduler §7, roles global/region, journalContext `Absence` |
| US-037 иерархия роя | FR-010, FR-120…123, BR-12, BR-16, NFR-034 | UC-020, UC-022 | I1a (страж `level_violation`) + I1b (Router вверх/вниз, Emitter) | §5.2, §5.4, §10 |
| US-006/US-007 (вклад) группа, раунды | FR-013, FR-025, FR-033, BR-13, NFR-005 | UC-016, UC-020 | I2 | `group-narrator`, `encounter` раунд §8.3 |
| US-005 (вклад, Must-часть) мир помнит | FR-127 | UC-004 A2 | I1b (`journalContext`), I2 (`MemoryClient` за флагом) | §11.2–11.3 |
| US-003 (вклад) `player_agency`/`level_violation` | BR-06, BR-16 | UC-022 | I1a | guardian §10.2 |
| US-010 (вклад) «глобальный GM и GM региона активны» | FR-080/081 | UC-028 | I1b | `Health()`, `agent.spawned` при старте |
| NFR-020/021/026 законы | FR-040, FR-042, FR-045, BR-02 | UC-022 | I1a | laws §12, guardian §10.4 |
| NFR-090/091 русский язык | — | UC-022 п. 2 | I1a | parser.CheckLanguage |
| NFR-041/043/048 ПДн, инъекции, контент (a) | BR-07, BR-08 | UC-023 | I1a (+ e2e I1b) | prompt escape, filter, схема без действий |

Полный список FR/NFR эпика — `epics.md` §2 EPIC-003; ответ «как обеспечивается NFR» — КД §16.

---

## 3. Инкременты: точный состав

Правило: задача ≤ M, один пакет-владелец, тесты уровня unit в той же задаче (ADR-010). Ниже — «единицы работы» (не задачи: нарезку и оценку делает tech-lead#2, §9 подсказывает, что параллельно).

### 3.1. I1a «LLM-шлюз, страж, законы, формат блупринта» (зависит только от EPIC-001: C-01, `membus`, `contracts`, `clock`)

| # | Единица работы | Пакет | Что входит | Критерий готовности единицы |
|---|---|---|---|---|
| A1 | Типы и парсер блупринта v2 | `shared/agent` (`types.go`, `blueprint.go`, `parser.go`, `placeholders.go`) | `AgentBlueprint` v2 (КД §13.1), frontmatter + секции, чистый YAML, `KnownFields(true)`, миграция плоских `llm.model` → `phase2` с предупреждением, `ContentHash`; удаление `md_parser.go`, `blueprint_loader.go`, `helpers.go`, `worker_pool.go`, `state_manager.go`, `filter.go`, `router.go`, `lifecycle.go`, `pipeline.go`, `examples/` (ADR-015 п. 6; старый код не удаляется из истории — только из пакета; `tools/*_tool.go` → `services/_archive/shared/agent/tools` уже в F-3) | `ParseFile` на 5 блупринтах MVP-1 и `testdata/blueprints/valid|invalid`; `ContentHash` стабилен |
| A2 | Реестр уровней и валидатор | `shared/agent` (`levels.go`, `validator.go`) | `AllowedEventTypes(level, role)` по `data-model.md` §4; вид «типы сущностей уровня» валидатор получает от вызывающего через `ValidationEnv`, собранный из `contracts.OwnershipRules` — единственной истины таблицы владения (`shared/contracts/ownership.go`, ADR-025, C-02 v1.4); своей таблицы `levels.go` не держит; 14 правил КД §13.2 + 7а (модели); `ValidationEnv`, `EnvFromProject(root, types, invariants, models)`; glob по сегментам | по одному invalid-файлу на правило; блупринт, чей `owned_entity_types` шире строки уровня в `contracts.OwnershipRules`, отвергается |
| A3 | Пять блупринтов + `schemas/agent` | `blueprints/`, `schemas/agent/` | `global-dark-forest-world.md`, `domain-dark-forest.md`, `encounter-wolf.md`, `player-gm.md`, `group-narrator.md` (КД §13.3; **модели — стартовая конфигурация E: `model: Qwen3.8-27B-UD-Q3_K_XL`, `thinking: false` на все фазы, комментарий-указатель `# per ops/metrics/baseline.md (OQ-A-18)`; запасные — по `decision_order` ADR-005 доп. 3: Qwen3.6-35B-A3B, C (`qwen3:30b-a3b`), A (`8b`+`14b`) (редакционно, T-440), сведение 2, внесено tech-lead#2**); `narrative.json`, `tick-global.json`, `tick-region.json`, `breach.json` (заглушка); генерация `enum` типов тика из `allowed_event_types` | `mvctl blueprint validate blueprints/` зелёный; схемы компилируются `jsonschema/v6` |
| A4 | `mvctl blueprint validate` | `cmd/mvctl/internal/blueprint` | подкоманда в реестре mvctl (каркас — F-4c), `--offline` (пропуск 7а), вывод «файл, поле, причина, severity», код возврата | job `contracts` CI вызывает команду |
| A5 | Схемы событий EPIC-003 (часть 1: рой/тики/законы) | `schemas/events/` + PR в `shared/contracts/registry.go` | `tick.fired`, `tick.aborted`, `agent.spawned/child_resolved/stopped/spawn_rejected/blueprint_reloaded`, `encounter.started/ended`, `combat.decided`, `world.weather_changed/time_advanced/event_occurred`, `region.event_occurred`, `npc.moved/spawned`, `world.laws.changed`, `world.law_breach.*` ×5 (без издателя), `config.cloud_enabled` | тест `contracts`: схемы валидны, тип → топик; фикстуры payload по `api-contracts.md` §2.3 |
| A6 | Схемы событий (часть 2: LLM/нарратив) | `schemas/events/` | `llm.output` (+`parse{}`, `error.code` с `yielded`), `llm.output.rejected` (`reason` enum ADR-017 п. 3), `narrative.output` (+`narrative_event_id`), `content.incident.recorded`; политика `llm_records`: `meta.agent` обязателен | то же; `reason` enum один с `guardian/reasons.go` (тест) |
| A7 | Законы | `internal/laws`, `laws/dark-forest-world.v1.yaml`, `cmd/mvctl/internal/laws` | `Service{Current, Get, Watch, Strain, Bump}`, `FileSource`, `ObjectSource` (интерфейс), `document.go` (enum статусов E-B), подписка `world.laws.changed`, `mvctl laws bump|show`; `MV_LAWS_*` | unit КД §18; `Bump` публикует `world.laws.changed` + `entity.update.proposed(world.laws_version)` в `membus` |
| B1 | Типы шлюза, конфигурация, реестр провайдеров | `internal/llm` (`types.go`, `config.go`, `providers/registry.go`) | `Provider{Generate, Embed, Health, Models}` (C-15 **v1.1**), `Request/Response/Params`, `Gateway`, `Call/Result`, `ValidationStatus`, ошибки; **(сведение 2, внесено tech-lead#2)** `Params{Temperature, TopP, TopK, MinP, PresencePenalty, MaxTokens, Think}` (`Think=false` по умолчанию для всех фаз MVP-1), `Response.Tokens{Prompt, Completion, Cached}` (`Cached` — из `usage.prompt_tokens_details.cached_tokens`, 0 при отсутствии) и `Response.ReasoningLen` (длина отброшенного `reasoning_content`, сам текст не сохраняется); `MV_LLM_*` через `env.Declare`, **добавлены `MV_LLM_URL` (по умолчанию `http://127.0.0.1:1234`) и `MV_LLM_API_KEY`** (`MV_OLLAMA_URL` — только при `MV_LLM_PROVIDER=ollama`); значения `MV_LLM_PROVIDER = openai_compat \| ollama \| anthropic \| recorded \| fake`; **гейт облака по host `MV_LLM_URL`** (не loopback / не `host.docker.internal` / не RFC1918 → отказ старта без `MV_LLM_CLOUD_ENABLED=true`, ADR-005 доп. 2 п. 3); таблица цен `config/llm-prices.yaml` по ключу `(endpoint_host, model)`, локальные адреса = 0 | компилируется с `fake`; `env check` видит переменные; внешний `MV_LLM_URL` без `MV_LLM_CLOUD_ENABLED` → отказ старта |
| B2 | Провайдеры `fake`, `recorded` | `internal/llm/providers/{fake,recorded}` | `FakeProvider` (таблица `(phase, matcher) → Response`, счётчик, генерация «преамбулы» для тестов парсера), `RecordedProvider` (ключ `(correlation_id, agent.id, phase, attempt)`, чтение `*.jsonl` или журнала `llm_records`, `ErrIncompleteRecord`), `Models()` | unit; промах ключа → ошибка, не шаблон |
| **B3a** | **Провайдер `openai_compat` (llama-server) — по умолчанию** *(сведение 2, внесено tech-lead#2: U-8, ADR-005 доп. 2 п. 1–5, C-15 v1.1)* | `internal/llm/providers/openai_compat` | `Generate` → `POST /v1/chat/completions` с `response_format {type: json_schema, json_schema:{name, schema}}`, `chat_template_kwargs {"enable_thinking": Params.Think}` (**`false` для всех фаз MVP-1** — llama.cpp #20345), `stream:false`, семплинг **на запрос** из блупринта фазы (`temperature, top_p, top_k, min_p, presence_penalty, max_tokens`; профиль non-thinking `0.7 / 0.8 / 20 / 0 / 1.5` — значения по умолчанию фазы, серверные `--temp 1 …` переопределяются); `Embed` → `POST /v1/embeddings`; `Models` → `GET /v1/models` (single-model режим `-m` → одно имя, stem файла, напр. `Qwen3.8-27B-UD-Q3_K_XL`); `Health` → `GET /health` (**503 = `loading`, не `unavailable`**) + `Models` (модель блупринта отсутствует → `degraded(model_not_resident)`); токены из `usage{prompt_tokens, completion_tokens, prompt_tokens_details.cached_tokens}`, задержка из `timings` (при отсутствии — по часам шлюза); `message.reasoning_content` **не сохраняется** (только `ReasoningLen` в `llm.output.parse{}`), `strip_think` в парсере остаётся страховкой; облако — та же реализация с другим `MV_LLM_URL`/`MV_LLM_API_KEY` (гейт по host); `net/http`, без SDK | httptest с записанными ответами `/v1/chat/completions`, `/v1/models`, `/health` (200 и 503), `/v1/embeddings`; тест отмены контекста (ADR-014 п. 2); тест «`Think=false` всегда уходит в `chat_template_kwargs`»; тест разбора `usage`/`timings` |
| **B3b** | **Провайдер `ollama` native — второй** *(сведение 2: может уйти за F-8, если конфигурация E проходит замер)* | `internal/llm/providers/ollama` | `/api/chat` (`format`=схема, `think:false`, `keep_alive`, `options{num_ctx,…}`), `/api/embed`, `/api/tags` (`Models`), `/api/ps` (`Health`), токены из `prompt_eval_count`/`eval_count`, отмена контекста закрывает соединение (ADR-014 п. 2); `net/http` без модуля `ollama`; включается `MV_LLM_PROVIDER=ollama` + `MV_OLLAMA_URL` | httptest с записанными ответами; тест отмены; реализуется **только** если F-8 выбирает конфигурацию C или A (иначе задача остаётся отложенной) |
| B4 | Парсер + язык | `internal/llm/parser` | `Parse(raw, schema)` — стратегии `direct → strip_think → strip_fence → balanced_object → trailing_commas` (ADR-016 п. 1), `schema.go` (компиляция 2020-12), `CheckLanguage` (0 CJK, латиница ≤ `MV_LLM_LATIN_MAX_RATIO`, идентификаторы исключены) | корпус `testdata/llm/preamble/*.txt` (≥ 12 кейсов); обрезанный JSON → `schema_invalid` |
| B5 | Бюджет, запись, учёт | `internal/llm/{budget,record,usage}.go` | окна `(world, level, phase, provider)`, восстановление из `llm_records` при догоне (через `Journal` — интерфейс, вызов из swarm), `Recorder` (`llm.output`/`.rejected`/`content.incident.recorded` через `Derive(cause)`, `quarantined` без `response_raw`, `MV_LLM_STORE_PROMPTS` → `prompts-{world}`), `Usage` агрегаты, `GET /v1/admin/llm/usage` (монтируется на `runtime.Mux`) | unit; `record` — свойство «raw отсутствует при quarantined/filter_error» |
| B6 | `Gateway.Generate` — конвейер | `internal/llm/gateway.go` | порядок КД §9.2 (бюджет → провайдер+повторы → парсер → язык → фильтр → страж (чистая) → запись → публикация `rejected`), таймауты фаз, `MV_LLM_TIMEOUT_DEGRADED`, повтор `stale laws`, `Health()` (опрос 10 с), `config.cloud_enabled` при старте | e2e-unit на `fake`: все ветки статусов (`valid`, `partially_rejected`, `invalid`, `quarantined`, `filter_error`, `rejected_language`, `error/unavailable`, `budget_exceeded`); один `llm.output` на попытку |
| C1 | Фильтр (a) | `internal/llm/filter`, `config/absolute-limits.yaml` | `NarrativeFilter`, `CategoryAFilter` (RE2, `context_terms` окно 80), `filter_version`, встроенный словарь по умолчанию, fail-closed; `prompt_notice` | golden 0 FP + позитивные кейсы; `error` → `filter_error` |
| C2 | Страж | `internal/llm/guardian` | `Evaluate(value, Input) Verdict`, правила 3–6 × (`narrative`, `tick`), `visibility.go` (таблица КД §10.3), `reasons.go`, гипотетическое применение `ops` к копии сущности, инварианты по `check`-ключам законов, `inv-11` | таблица 9 правил × 2 вида схем (ADR-017); без шины |
| C3 | Промпт-билдер | `internal/llm/prompt` | секции КД §11.4, плейсхолдеры, `escape.go` (`<`/`>` для `player_text` и `<facts source="memory">`), `events.go` (перенос `clusterEvents/formatEventDescription` из narrative-orchestrator), `prompt_hash`; тесты переписаны из `prompt_builder_test.go` | `prompt_hash` стабилен; system неизменен между попытками |
| C4 | `FakeEncounter` + `FakeNarrator` (реализация, замена v0 из F-10) | `shared/testkit/swarm` | **для I1-α** (§4.1): `FakeEncounter` — на `player.entered_region` публикует `entity.create.proposed encounter` + `encounter.started`; на `player.attacked/flee_attempted/rested` — Phase 1 через `mechanics.Rules` (реальный `Resolve` EPIC-002 или `FixedMechanics`), `dice.rolled`, `combat.decided` ×2, `entity.update.proposed atomic`, `encounter.ended`; `FakeNarrator` — `narrative.output generated_by=template` (`template/ru.go` вынесен в `shared/testkit/swarm/template` и переиспользуется ролями I1b) на `combat.decided`/`encounter.started`/`player.looked`/`round.closed`/`entity.updated status=dead`; без LLM, без Router | e2e `solo-30` на `membus` + `FakeState` + `Harness` v0 проходит без роя: 30 ходов, 0 без ответа |
| C5 | `RecordingWriter` + `mvctl record` | `shared/testkit/swarm/recording_writer.go`, `cmd/mvctl/internal/record` | запись `llm.output` из живого прогона (`Journal.ReadRange` по `llm_records` за окно сценария) в `*.jsonl` по ключу; отказ при `actor_kind≠ci` или игроке вне `player-A/B/C`; `mvctl record diff`; `.gitattributes` уже в F-1 | запись `solo-30.jsonl` снимается на стенде (стендовая задача); unit на фикстуре |

**Готовность I1a** (`epics.md`): `mvctl blueprint validate blueprints/` зелёный на пяти блупринтах; `llm.Gateway.Generate` на `fake`/`recorded`/**живом llama-server** (`openai_compat`; Ollama — только если F-8 выберет C/A) проходит парсер → язык → фильтр (a) → страж; `llm.output` записывается; `FakeEncounter`/`FakeNarrator` реализованы и слиты в `integration/mvp-1` до I1-α; первая запись `solo-30.jsonl` снята на стенде (стенд, не блокирует слияние I1a).

### 3.2. I1b «Рой: рантайм, роли, тики, миграция» (зависит от I1a; от EPIC-002 — через заглушки v0 `FakeState`/`FixedMechanics`/`rules/dark-forest.yaml`, затем реальные)

| # | Единица работы | Пакет | Что входит | Критерий |
|---|---|---|---|---|
| R1 | Реестр блупринтов и индекс scope | `internal/swarm/{registry,scope}.go` | `LoadDir` (валидация, `/health degraded {blueprints}`), `ByTrigger`, `ScopeIndex` (`solo/group → region → world`, `PlayersIn`, `RegionOf`, `GroupOf`) из `entity.*`/`group.*`/`encounter.*` | unit на фикстурах |
| R2 | Router + Emitter + Instance | `internal/swarm/{router,emitter,instance}.go` | `Route` (КД §5.1: dedup `eventbus.Dedup`, legacy-фильтр, проекции, `Match` по glob+scope_binding, `bindScope`, `Recipients`), `Behaviour` интерфейс, таблица видимости §5.2 — как метод ролей, динамический parent §5.3, `Emitter` с белыми списками §5.4 (`ErrLevelViolation`) | таблица видимости — по кейсу на строку; Emitter отклоняет `player.*` |
| R3 | Lifecycle | `internal/swarm/lifecycle.go` | `Ensure/Stop/Touch/ExpireDue`, детерминированный `agent.id`, `agent.*` события с `content_hash`, TTL на `clock.Timers`, `MV_SWARM_MAX_AGENTS`, replay-ветка (`agent.stopped` из журнала) | unit на `clock.Manual` |
| R4 | Scheduler + BackgroundBudget | `internal/swarm/{scheduler,budget}.go` | ADR-014: две очереди, quiet-window, yield (отмена контекста → `yielded`), max_wait → rule-only, `RegisterTimer/SetTickMode/FireNow`, `tick.fired` до выполнения с `lod_allowed` (обязателен), один тик после простоя, `tick.aborted` после догона; окно бюджета из `llm.output actor_kind=system` | unit на `clock.Manual` + `FakeProvider` с задержкой |
| R5 | Pipeline + шаблоны деградации | `internal/swarm/pipeline.go`, `template` (из C4) | `HandleEvent/HandleTick`, постановка LLM-заданий, `fallback_reason`, «рестарт между Phase 1 и 2» (КД §8.4), per-instance FIFO, сериализация по `correlation_id` | unit: все `fallback_reason` |
| R6 | Роли `global-gm`, `region-gm` | `internal/swarm/roles/{global_gm,region_gm,triggers}.go` | tick-фаза (`basic` — LLM по `tick-*.json`; `rule-only` — `background_events` по весам с `Seed(tick.id, 0)`), `world.*`/`region.*`/`npc.*` + `entity.update.proposed cause=tick`; обнаружение встреч (rules, `encounter.detect_on: tick`, `chance` через `Rules.Roll`), `entity.create.proposed encounter`, `encounter.started` с `round{}` из блупринта встречи; респаун по `npc_table` (`respawn_ttl`, `died_at`, новый `entity_id`); тест согласованности `npc_table` ↔ `rules`/фикстуры | unit на `FixedMechanics` + `FakeProvider`; US-036 критерии в `membus` |
| R7 | Роль `encounter` (соло) | `internal/swarm/roles/encounter.go` | Phase 1 синхронно: `Resolve` атаки/бегства/`npc_attack`, `dice.rolled` ×k до `combat.decided`, один `entity.update.proposed atomic` с `expected_version`, трофей один раз, `encounter.ended reason=npc_dead|players_out`, `agent.child_resolved`; `rest` во встрече — не его дело (State отклоняет) | unit: seed-детерминизм, `flee` провал → свободная атака, смерть игрока |
| R8 | Роль `personal-gm` | `internal/swarm/roles/personal_gm.go` | таблица триггеров КД §8.2 (`entry` c `absence`, `turn` по последнему событию цикла, `world_event`, `death`), Phase 2 через `Gateway`, `narrative.output` (`recipients`, `based_on[]`, `background_refs[]` ⊂ контекста, `absence{}`, `filter{}`, `laws_version`, `narrative_event_id`), rule-only в группе | unit: по кейсу на строку таблицы; один вызов на ход |
| R9 | Контекст: WorldView, journalContext, builder, `NopMemory` | `internal/swarm/context/` | проекция сущностей (старт из `state/latest.json`, далее `entity.*` по `version`), `EventWindow`/`BackgroundIndex`/`Presence`/`Absence` (КД §11.2), `builder.go` → `prompt.Sections`, `MemoryClient` интерфейс + `NopMemory` (реализация HTTP — I2) | unit: `Absence` по паузе ≥ 30 мин; страж пропускает только `background_refs` из контекста |
| R10 | Snapshot + догон | `internal/swarm/{snapshot,runtime,swarm}.go` | `SwarmSnapshot` (КД §4.3), триггеры, `K=5`, `latest.json`-указатель, `snapshot.created component=swarm`; `Context{Start, Stop, Health}` для `shared/runtime`; фаза догона через `Journal.ReadRange…End` без эмиссии; `Subscribe` consumer-group `core.swarm`; `MV_BUS_VALIDATE_ON_READ` | integration (testcontainers Redpanda+MinIO): рестарт → `llm_calls=0`; unit round-trip + `state_hash` |
| R11 | Admin-маршруты и `/health` | `internal/swarm/admin.go` | `GET /v1/admin/agents`, `POST /v1/admin/agents/{id}/tick` (→ `FireNow`, `actor_kind=ci`) на `runtime.Mux`; `Health()`: `agents_by_level`, `blueprints`, `laws: unknown_check`, `swarm: region_missing`; раздел `admin` в `api/gateway.openapi.yaml` — согласовать с EPIC-004 (текст маршрутов даёт EPIC-003) | httptest; `/health` процесса содержит `agents_by_level` |
| R12 | Миграция I1-0…I1-3 и профиль `legacy` | `swarm/router.go` (deprecated-фильтр), `emitter.go`/`llm/record.go` (`gm_path` из `meta`), compose (`legacy` — совместно с devops) | `MV_GM_PATH` не читается `core`; `gm_path` в `combat.decided`/`llm.output`; профиль `legacy` работает (или принят запасной критерий S5 — решение F-6) | ручная проверка профиля на стенде |
| R13 | e2e и golden | `test/e2e/`, `internal/llm/golden_test.go`, `testdata/recordings/` | `solo-30` (S1), `background-6h` (S14, admin-тики, ≤ B), `death`, `flee-fail`, `injections-10` (0 `entity.updated` от нарратива, 0 `world.law_breach.proposed`), `degraded` (`fake` → ошибка → 100 % шаблонов), `recovery` (рестарт контекстов в процессе → `llm_calls=0`, `identical=true`); golden 20 ходов + 3 тика (эталоны — вместе с 005-ops) | все в CI ≤ 10 мин без GPU на `--mode=replay --bus=memory` |

**Готовность I1** (`epics.md`): S1 (нарратив на записях в CI; **живой llama-server на стенде**, сведение 2), S14 с ускоренными тиками, **удалён хук `MV_SWARM_FAKE` из `cmd/multiverse`** (критерий тега, ADR-001 доп. п. 8), S6 (второй регион блупринтом + фикстурой), S3 (снапшот роя восстанавливается); слияние в `integration/mvp-1` последним (002 → 004 → 003) — тег `mvp-1/i1`.

### 3.3. I2 «Группа, память, завершение миграции» (старт — по приёмке I1 tech-lead#2; слияние — после интеграционного прогона I1; порядок волны 2: 002 → **003** → 004 → 005)

| # | Единица работы | Пакет | Что входит | Зависит от |
|---|---|---|---|---|
| G1 | Раунд группы в `encounter` | `roles/encounter.go` | `round.closed` → порядок `acted[]`, `NPCTarget` (искл. `idle/out_of_combat/dead`), один `npc_attack` с `causeEventID = round.closed.id`, `idle` после 2 `auto_defended` (`entity.update.proposed participation`), `group.left` во встрече = `flee` (BR-13), `players_out` | EPIC-002 I2 (atomic группы, `Participation`) — до слияния; до того `FakeState` |
| G2 | Роль `group-narrator` | `roles/group_narrator.go`, `blueprints/group-narrator.md` (уже в A3) | Phase 2 `round` по завершению раунда (последнее `combat.decided npc_attack` или `encounter.ended`), `recipients[]` = `active|idle|out_of_combat`, один `narrative_event_id`; `turn`/`world_event` вне встречи; TTL по любому участнику; `personal-gm` в группе — rule-only (`entry` с `absence` — LLM, иначе шаблон) | R8, G1 |
| G3 | `MemoryClient` HTTP (C-09) | `context/memory.go` | `MV_MEMORY_URL`, таймаут 500 мс, деградация в `journalContext`, `<facts source="memory">` с экранированием (SEC-17), `NopMemory` в replay | 005-memory (Should; без него — флаг пуст, поведение I1) |
| G4 | Горячая перезагрузка блупринтов (FR-091, Should) | `registry.go` | fsnotify каталога → новая версия к следующему спавну и timer-агентам на следующем тике; `agent.blueprint_reloaded {content_hash}` | R1 |
| G5 | Лимиты Should | `llm/budget.go` | `MV_LLM_TURN_CALLS_PER_MIN` (интерактив), облачный денежный лимит → переключение/шаблон | B5 |
| G6 | e2e I2 | `test/e2e/` | `group-3x30` (S2, `rounds/close` от харнесса), `injections-10` + ≥ 3 инъекции через память, S6 повтор | G1, G2 |
| G7 | Завершение миграции (S5) — **последняя задача I2, после зелёного S2** | compose, `shared/contracts` (через system-architect), `services/_archive/` | перенос `narrative-orchestrator` + `semantic-memory` в `services/_archive/` с `ARCHIVED.md`, снятие профиля `legacy` и `chromadb`, deprecated-типов, `MV_GM_PATH` (EPIC-004 убирает чтение флага у себя) | все |

**Готовность I2**: S2 (раунды группы, 30 раундов ≥ 60 действий), S5, S6; тег `mvp-1/i2`.

---

## 4. Точка I1-α и заглушки

### 4.1. I1-α «соло на шаблонах через бота» — минимальный вклад EPIC-003

Состав точки (`epics.md` §2): EPIC-002 I1 + EPIC-004 I1 + заглушка нарратива. Без роя: `combat.decided`/`encounter.started`/`dice.rolled`/`entity.update.proposed` в бою **некому публиковать** (типы EPIC-003, а `dice.rolled` — EPIC-002, но издаёт агент встречи). Поэтому заглушка v0 из F-10 (`FakeNarrator`: шаблон на `combat.decided`) для игры недостаточна — нужна заглушка Phase 1.

Минимально необходимое от EPIC-003 (единица C4, первая волна I1a, слить в `integration/mvp-1` сразу после приёмки tech-lead#2, до I1-α):

1. **`testkit/swarm.FakeEncounter`** — подписчик `player_events` в процессе `core`. **Форма включения решена (сведение 2, запрос e, ADR-001 доп. п. 8; внесено tech-lead#2)**: флаг `MV_SWARM_FAKE=true` читает **`cmd/multiverse`**, а не контекст `swarm` (на момент I1-α `internal/swarm` в `integration/mvp-1` ещё нет — контексту негде читать флаг). Файл-хук — **`cmd/multiverse/fake_contexts.go`**: при `MV_SWARM_FAKE=true` под именем контекста `swarm` регистрируется **`shared/testkit/swarm.FakeContext` — тип, реализующий `runtime.Context` (`Start/Stop/Health`)** и поднимающий внутри себя `FakeEncounter` + `FakeNarrator`; при наличии `internal/swarm` регистрируется настоящий контекст вместо фейкового. Это **единственный разрешённый импорт `shared/testkit/*` в production-бинарнике** (исключение `depguard` по файлу); `shared/testkit/swarm` не должен зависеть от `testify` и тестовых пакетов. **Хук удаляется при слиянии EPIC-003 I1 — это критерий готовности I1 (тег `mvp-1/i1`)**; сами фейки остаются в `shared/testkit/swarm` для unit/e2e на `membus`. Файл `cmd/multiverse/fake_contexts.go` — совладение с EPIC-001: правка идёт PR через tech-lead#1.
   - `player.entered_region` → если в регионе есть живой NPC из фикстур и нет активной встречи → `entity.create.proposed {encounter}` → `encounter.started {…, round{60s, 2}}` (немедленно, без тика — упрощение I1-α; в I1b — по тику региона);
   - `player.attacked` → `mechanics.Rules.Resolve` (реальный из EPIC-002 I1; если ещё нет — `FixedMechanics`) → `dice.rolled` ×4 → `combat.decided` ×2 → `entity.update.proposed atomic` с `expected_version` из `FakeState`/State → при смерти NPC `encounter.ended reason=npc_dead`; `player.flee_attempted` → бросок бегства, при провале свободная атака;
   - `gm_path` из `meta.gm_path`, `meta.agent = {id: "fake-encounter:solo:…", level: task, blueprint: encounter-wolf}` (политики `llm_records`/`system_events` требуют `meta.agent` для типов роя — соблюдаем).
2. **`testkit/swarm.FakeNarrator`** (реализация вместо v0) — `narrative.output generated_by=template fallback_reason=unavailable filter{applied:false} laws_version=v1` на `player.entered_region` (`entry`), `player.looked` (`turn`), `encounter.started` (`world_event`), последнее `combat.decided` цикла (`turn`), `entity.updated(player) status=dead` (`death`), `round.closed` (`round`, для EPIC-004 I2). Шаблоны — `shared/testkit/swarm/template/ru.go`, те же используются ролями в I1b (переезд в `internal/swarm/template` — при R5, экспорт через testkit остаётся).
3. **Без LLM** (`MV_LLM_PROVIDER` не читается). Вариант «с `fake`-провайдером» появляется только с R5/R8 (I1b) — тогда I1-α+ можно прогонять на реальном рое с `MV_LLM_PROVIDER=fake`; для самой точки I1-α это не требуется.

Что проверяет человек на I1-α (в части EPIC-003): бой с волком доходит до `combat.decided` и `entity.updated`, нарратив приходит отдельным сообщением с пометкой шаблона, смерть/трофей/бегство работают. Замечания к шаблонам → задачи EPIC-003 I1b.

### 4.2. Заглушки, которые эпик потребляет

| Контракт | Заглушка | Откуда/когда | Как используем | Замена |
|---|---|---|---|---|
| C-01 шина/журнал/реестр | `membus` (`shared/eventbus/membus`, с T-418; +`Journal`, `Dedup`, `--chaos=duplicate`), `shared/contracts` со схемами `_common` | EPIC-001, волна 0 | все unit/e2e; свои схемы добавляем в `schemas/events/` + PR в реестр (A5/A6) | kafka-адаптер в integration |
| C-02 факты State | `testkit/state.FakeState` v0 (ops в память, факты, без инвариантов; `WithInvariants()` — позже) | F-10 (v0) → EPIC-002 I1 | WorldView, `expected_version`, `entity.create.proposed encounter/npc` | реальный State при интеграции I1 (слияние 002 раньше 003) |
| C-03 механика | `internal/mechanics` типы + `Load` + `rules/dark-forest.yaml` (F-10), `testkit/mechanics.FixedMechanics` (табличные исходы по seed) | F-10 → EPIC-002 I1 (`Resolve`) | R7/G1/C4 компилируются против типов C-03 v1.1; `Invariants()` для стража — при отсутствии реализации страж получает пустую карту → `/health degraded {laws: unknown_check}` (ожидаемо до слияния 002) | реальный `Resolve/Invariants` |
| C-04 действия игрока | `testkit/gateway.Harness` v0 (генератор `player.*` из фикстур в `membus`, без HTTP) | F-10 → EPIC-004 | e2e `solo-30`, `death`, `flee-fail`; `round.closed` для G1 генерирует харнесс | HTTP-харнесс EPIC-004 при интеграции |
| C-09 память | `internal/swarm/context/journal.go` — **штатная деградация** (не тест) | сами, R9 | всегда; `MV_MEMORY_URL` пуст | `MemoryClient` HTTP в G3, если 005-memory не отрезана |
| C-14 `state/latest.json` | фикстура `testdata/fixtures/snapshots/state/latest.json` (F-10) | F-10 | старт WorldView в unit/e2e | реальный указатель State |
| часы/таймеры | `clock.Manual`, `ManualTimers` (EPIC-001) | волна 0 | Lifecycle/Scheduler unit | `EventClock`/`NullTimers` в replay (EPIC-002 `internal/replay`, внедряет `cmd/multiverse`) |

Правило: заглушку потребитель не правит — запрос владельцу (через tech-lead#1).

### 4.3. Заглушки, которые эпик поставляет

| Контракт | Заглушка | Кому | Когда |
|---|---|---|---|
| C-05 нарратив/механика | `testkit/swarm.FakeNarrator` (реализация) + **`FakeEncounter`** (§4.1) | EPIC-004 (e2e gateway/бота, I1-α), EPIC-002 (e2e `solo-30` на «FixedNarrator» = `FakeNarrator`) | I1a, первая волна |
| C-06 тики/admin | `FireNow` в `--mode=replay`; до готовности роя харнесс EPIC-005 генерирует `tick.fired` в `membus` сам (C-06) — от нас только схема (A5) | EPIC-005 | A5 (схема) / R4 (`FireNow`) |
| C-07 записи LLM | `testkit/swarm.RecordingWriter`, `providers/fake` как источник записей; `testdata/recordings/*.jsonl` | EPIC-005 (golden, `llm usage`), EPIC-002 (`replay`) | C5 (I1a), записи — стенд |
| C-11 формат блупринта | `agent.Validate` + `mvctl blueprint validate`; `testdata/blueprints/valid|invalid` | авторы, EPIC-005 (`contracts check` вызывает) | A2/A4 |
| C-12 законы | `laws/dark-forest-world.v1.yaml`, схемы `world.law_breach.*` (без издателя) | EPIC-007 | A5/A7 |
| C-15 провайдер | `Provider` + `providers.Registry`; `fake`/`recorded` | EPIC-013 (облако), EPIC-005 (`Embed`) | B1/B2 |

---

## 5. Порядок внутри эпика и внутренние зависимости

```mermaid
flowchart LR
    subgraph I1a
      A1[A1 типы+парсер] --> A2[A2 levels+валидатор] --> A3[A3 блупринты+schemas/agent] --> A4[A4 mvctl blueprint validate]
      A5[A5 схемы событий: рой/тики/законы] --> A7[A7 laws]
      A6[A6 схемы: llm/narrative]
      B1[B1 типы шлюза+config+registry] --> B2[B2 fake/recorded] --> B6[B6 Gateway.Generate]
      B1 --> B3a[B3a openai_compat / llama-server]
      B1 -.после F-8.-> B3b[B3b ollama]
      B1 --> B4[B4 parser+language] --> B6
      B1 --> B5[B5 budget/record/usage] --> B6
      A6 --> B5
      C1[C1 filter] --> B6
      C2[C2 guardian] --> B6
      A7 --> C2
      C3[C3 prompt] --> B6
      A1 --> C3
      C4[C4 FakeEncounter+FakeNarrator] -.I1-α.-> EXT4[EPIC-004 I1]
      B2 --> C5[C5 RecordingWriter+mvctl record]
    end
    subgraph I1b
      A3 --> R1[R1 registry+scope] --> R2[R2 router+emitter] --> R3[R3 lifecycle] --> R4[R4 scheduler+budget] --> R5[R5 pipeline+template]
      B6 --> R5
      R5 --> R6[R6 global/region] & R7[R7 encounter] & R8[R8 personal-gm]
      C3 --> R9[R9 context] --> R8
      R3 --> R10[R10 snapshot+догон] --> R11[R11 admin+health]
      R6 & R7 & R8 & R10 --> R12[R12 миграция/legacy] --> R13[R13 e2e+golden]
    end
    subgraph I2
      R7 --> G1[G1 раунд группы] --> G2[G2 group-narrator] --> G6[G6 e2e S2]
      R9 --> G3[G3 MemoryClient]
      R1 --> G4[G4 hot reload]
      B5 --> G5[G5 лимиты Should]
      G6 --> G7[G7 S5 архив legacy]
    end
```

Порядок, заданный оркестратором (шлюз → законы/страж → блупринты/валидатор → роутер/lifecycle → планировщик/тики → пайплайн → нарратор группы), соблюдается по критическому пути `B1 → B6 → R5 → R8 → G2`; при трёх разработчиках три независимых потока I1a: **A** (`shared/agent` → блупринты → CLI → законы), **B** (шлюз: типы → провайдеры → парсер → бюджет/запись → `Generate`), **C** (схемы событий → фильтр → страж → промпт → `FakeEncounter/FakeNarrator` → `RecordingWriter`). Пересечений по файлам между потоками нет (`A7 laws` зависит от `A5` схем — оба в потоке A/C по договорённости тимлида; `C2 guardian` импортирует типы `laws` — B/C ждут A7 только на этапе компиляции стража, до этого — локальный интерфейс `LawsVersion`).

I1b — три потока: **рантайм** (R1 → R2 → R3 → R4 → R5), **роли** (R7 на `FixedMechanics` можно начинать сразу после R2-интерфейсов `Behaviour/Emitter`; R6, R8), **контекст/снапшот/admin** (R9 → R10 → R11). Последняя волна I1b — R12 + R13 (все три).

Внешние зависимости по времени: EPIC-002 I1 (`Resolve`, State) нужен к R7/R13 — до слияния используем заглушки; EPIC-004 I1 (`Harness` HTTP) — только для интеграции I1; EPIC-005 — ничего не блокирует.

---

## 6. Данные

| Артефакт | Содержание | Владелец / где | Инкремент |
|---|---|---|---|
| `blueprints/{global-dark-forest-world,domain-dark-forest,encounter-wolf,player-gm,group-narrator}.md` | формат v2 (КД §13.1, §13.3); **модели: E стартово (`Qwen3.8-27B-UD-Q3_K_XL`, `thinking: false`), запасные — по `decision_order` ADR-005 доп. 3: Qwen3.6-35B-A3B, C, A** (сведение 2; редакционно, T-440), комментарий-указатель на `baseline.md`; семплинг фазы (`temperature/top_p/top_k/min_p/presence_penalty/max_tokens`, non-thinking-профиль по умолчанию); `round{60s,2}` в `encounter-wolf`; `budget.background_calls_per_hour_world: 4` в global; `respawn_ttl: 24h`, `npc_table` (только респаун), `background_events[]`, `encounter{detect_on: tick, chance}` в domain | EPIC-003, `blueprints/` | A3 |
| `schemas/agent/{narrative,tick-global,tick-region,breach}.json` | схемы structured output (КД §13.4); `enum` тика подставляется из `allowed_event_types` | A3 | I1a |
| `schemas/events/<26 типов>.v1.json` | JSON Schema 2020-12, `$ref` `_common.json`; payload по `api-contracts.md` §2.3.6–2.3.15 + дополнения v1.1 | A5/A6; регистрация — PR в `shared/contracts/registry.go` (ревью system-architect) | I1a |
| `laws/dark-forest-world.v1.yaml` | `inv-1…inv-11` (`check`-ключи ↔ `mechanics.Invariants().ID`), декларативные `law-1…` (уточняет автор) | A7 | I1a |
| `config/absolute-limits.yaml` | `filter_version`, `prompt_notice`, категория (a) `terms`/`context_terms`, реестр (b)–(g) `enabled=false` | C1 | I1a |
| `config/llm-prices.yaml` | цены за 1k токенов по ключу **`(endpoint_host, model)`** (сведение 2, ADR-005 доп. 2 п. 3); локальные адреса (llama-server, Ollama) = 0 | B1 | I1a |
| `testdata/recordings/{solo-30,death,flee-fail,injections-10,background-6h,recovery}.jsonl` (I1), `group-3x30.jsonl` (I2) | строки `llm.output` с ключом; только `actor_kind=ci`; `merge=binary` | C5 + стенд | I1b / I2 |
| `testdata/llm/preamble/*.txt` | корпус «грязных» ответов Qwen3 (преамбула, `<think>`, код-блоки, trailing commas, обрезка) | B4 | I1a |
| `testdata/golden/*.json` | 20 ходов + 3 тика с эталонами (схема, язык, числа, фильтр) | R13 + 005-ops | I1b |
| `shared/agent/testdata/blueprints/{valid,invalid}/*.md` | по файлу на правило валидатора | A2 | I1a |
| `snapshots-{world}/swarm/{ts}-{seq}.json` + `latest.json` (MinIO) | `SwarmSnapshot` (КД §4.3), `K=5` | R10 | I1b |
| `prompts-{world}/{cid}/{agent}/{phase}-{attempt}.txt` (MinIO, по флагу) | полный промпт; ILM 30 дней, versioning off (devops) | B5 | I1a |

Модель данных агента (`AgentInstance`), снапшота и индексов — КД §4.2–4.3, §11.2; изменений относительно `data-model.md` §6–§7 нет.

---

## 7. Изменения API / событий / конфигурации

- **События (совместимые, уже в contracts v0.2)**: `llm.output.parse{}`, `error.code=yielded`, `narrative.output.narrative_event_id`, `tick.fired.tick.lod_allowed` (обязателен), `agent.spawned.content_hash`, `encounter.started.round{}`, `config.cloud_enabled{by, provider, allow_external_players}`. Новых типов вне реестра ADR-007 нет.
- **HTTP**: `GET /v1/admin/agents`, `POST /v1/admin/agents/{id}/tick`, `GET /v1/admin/llm/usage` — маршруты на сервере процесса `core` (`MV_CORE_ADDR`), описание — раздел `admin` `api/gateway.openapi.yaml` (файл EPIC-004; текст маршрутов даёт EPIC-003 задачей R11 через tech-lead#3). Доступ — только `ci`/operator через прокси gateway (ADR-009 п. 9).
- **CLI** (`cmd/mvctl`, реестр — tech-lead#1): `blueprint validate [--offline] <dir>`, `laws bump --world … --from <file>`, `laws show --world …`, `record --scenario <name> --from <journal|jsonl> --out <file>`, `record diff`.
- **Env**: таблица КД §14 (все `MV_*`), объявление через `shared/env.Declare`; **(сведение 2)** добавлены `MV_LLM_URL` (по умолчанию `http://127.0.0.1:1234`; из контейнеров `http://host.docker.internal:1234`), `MV_LLM_API_KEY` (только облако), `MV_SWARM_FAKE` (читает `cmd/multiverse`, не контекст); `MV_LLM_PROVIDER` по умолчанию `openai_compat`; `MV_OLLAMA_URL` — только при `MV_LLM_PROVIDER=ollama`; `.env.example` — правка через tech-lead#1/devops (NFR-074).
- **Зависимости `go.mod`** (пометка для tech-lead#1): `github.com/santhosh-tekuri/jsonschema/v6` (уже в F-4), `gopkg.in/yaml.v3` (уже), `github.com/fsnotify/fsnotify` (только G4, I2 — Should; можно заменить опросом каталога по `clock.Timers` и не добавлять зависимость — рекомендую опрос раз в 5 с).
- **Compose/CI** (через devops): профиль `legacy` (D-3); **(сведение 2)** LLM-рантайм — нативный llama-server **вне compose** (`scripts/llm-server.ps1`, `make llm-up/llm-down/llm-health`, проверка `GET $MV_LLM_URL/health` в `make up`); профиль `gpu` (Ollama) и `OLLAMA_*` — опционально, только для конфигураций C/A; job `contracts` вызывает `mvctl blueprint validate`, job `privacy-scan` для `testdata/`.

---

## 8. Безопасность (по `threat-model.md`, ADR-009)

| Угроза / SEC | Мера в эпике | Где проверяется |
|---|---|---|
| SEC-17 инъекции через `say` и память | `<player_text>`/`<facts source="memory">` — данные с экранированием `<`/`>`, лимит 500; схема Phase 2 без действий; фаза пробоя выключена | `prompt/escape_test.go`; e2e `injections-10` (0 `entity.updated` от нарратива, 0 `law_breach.proposed`) |
| SEC-21 блупринт выбирает облако | поля провайдера нет (`KnownFields`), модель ∈ `Provider.Models()`, облако только `MV_LLM_CLOUD_ENABLED` + `config.cloud_enabled {by, provider, endpoint_host}` без ключей; **(сведение 2)** «облако» определяется **адресом `MV_LLM_URL`**, а не именем провайдера: не-локальный host без `MV_LLM_CLOUD_ENABLED=true` → провайдер не стартует | валидатор 7а; тест гейта по host (loopback / `host.docker.internal` / RFC1918 → старт; внешний → отказ); тест «событие не содержит подстрок ключей» |
| SEC-23 ПДн в записях/golden | `mvctl record` принимает только `actor_kind=ci` и `player-A/B/C`; `privacy-scan` | C5 unit; CI `security` |
| NFR-041 ПДн в промптах/логах | в промпт только `player_id`, имена персонажей, текст `say`; `slog` без payload `say` (только длина/хэш) | тест NFR-041 (EPIC-005) читает `llm_records` и логи |
| NFR-048 контент (a) | фильтр до записи `response_raw`; `quarantined` без raw; fail-closed при ошибке фильтра | C1 unit; `record` свойство |
| SEC-10 разметка в нарративе | шаблоны и схема — plain text; бот без `parse_mode` (EPIC-004) | golden: текст без `<`/`*`-разметки |
| BR-16 уровни через данные | белые списки уровня: типы событий — `levels.go`, типы сущностей — строка уровня в `contracts.OwnershipRules` (ADR-025), обе ⊇ блупринт; `Emitter` и страж; State — последняя линия (`contracts.OwnershipRules`) | A2 (валидатор против `contracts.OwnershipRules`); R2; C2 |
| Отмена фонового вызова (ADR-014) | `providers/openai_compat` (и `providers/ollama`, если реализуется) закрывает соединение по `ctx`; нет утечки горутин | **B3a** тест отмены (B3b — при реализации), `goleak` в unit |
| admin-маршруты | только через прокси gateway (`ci`/operator), `MV_CORE_ADDR` на `127.0.0.1`; в compose порт не публикуется наружу кроме `127.0.0.1:8090` | compose-lint (EPIC-001) |

Секретов в блоке нет; ключи облака — только env `MV_*_API_KEY`, в события/логи не попадают (тест на `config.cloud_enabled` и `llm.output`).

---

## 9. Тестируемость (ADR-010)

| Уровень | Что | Инструменты | Инкремент |
|---|---|---|---|
| unit (`-short`) | всё из КД §18 по пакетам; таблицы: видимость (§5.2), триггеры (§8.2), страж 9×2, валидатор 15 правил, парсер ≥ 12 кейсов, планировщик (quiet/yield/max_wait/бюджет/один тик), Lifecycle TTL, Emitter, шаблоны всех `kind`, `record`-свойства, `prompt_hash` | `membus`, `FakeProvider`, `FakeState`, `FixedMechanics`, `clock.Manual`, `goleak` | I1a/I1b |
| integration (`-tags integration`) | подписка/дедуп/DLQ/`MV_BUS_VALIDATE_ON_READ` через testcontainers Redpanda; снапшот роя + `latest.json` в MinIO (образ из `build/minio.Dockerfile`, `testkit.Versions()`); догон `ReadRange…End`; `providers/openai_compat` (и `ollama`, если реализуется) против httptest — **живой llama-server только стенд (сведение 2)** | testcontainers-go | I1b |
| e2e (`-tags e2e`) | `cmd/multiverse --contexts=all --mode=replay --bus=memory --recording=…`: `solo-30`, `background-6h` (admin-тики, ≤ B), `death`, `flee-fail`, `injections-10`, `degraded`, `recovery`; I2: `group-3x30`; до готовности EPIC-002/004 — на `FakeState`/`Harness` v0, после интеграции — на реальных | `providers/recorded`, `EventClock`, `--id-source=sequence`, `--chaos=duplicate` | I1b / I2 |
| golden (NFR-065) | 20 ходов + 3 тика: схема, язык, числа механики не противоречат `combat.decided`, фильтр `pass`, 0 FP фильтра | `internal/llm/golden_test.go`; эталоны — 005-ops | I1b |
| детерминизм (NFR-061) | два прогона `solo-30` → равные хэши последовательности доменных событий; `prompt_hash` стабилен между прогонами при `NopMemory` | e2e | I1b |
| стенд (вне CI, человек + tester#2/architect#2) | F-8 → модели в блупринтах; запись `solo-30.jsonl`, `background-6h.jsonl`, `death`, `flee-fail`, `injections-10` (`mvctl record`); живой S1/S14 **на llama-server (`openai_compat`, `scripts/llm-server.ps1`, `make llm-up`); Ollama — только если F-8 выберет C/A** (сведение 2); профиль `legacy` (S5); nightly-gpu матрица (`scripts/llm-bench.ps1` → `baseline.md`, `valid_first_try`, `phase2 p95`, язык, `usage`/`timings`) | целевая машина RTX 4090 | конец I1a (первая запись), конец I1b, конец I2 |

Покрытие ≥ 60 % по `internal/{swarm,llm}` — job `unit` (NFR-064). Записи обновляются только осознанной задачей с ревью диффа нарративов (ADR-010 доп. п. 2).

---

## 10. Риски и реакции

| # | Риск | Вероятность / влияние | Реакция |
|---|---|---|---|
| 1 | Объём XL (~35 единиц), критический путь MVP-1 | высокая / высокое | три независимых потока I1a (§5); `FakeEncounter`/`FakeNarrator` первыми — I1-α не ждёт рой; e2e на заглушках до интеграции; задача ≤ M; стендовые задачи вне слотов |
| 2 | Structured output Qwen3: преамбула/`<think>`/обрезка → низкий `valid_first_try`. **(сведение 2, внесено tech-lead#2)** Добавлен риск **llama.cpp #20345** (открыт с 2026-03): при включённом thinking грамматика `json_schema` **не применяется** — llama-server может вернуть markdown-обёртку или чужие поля вместо валидного JSON | средняя / среднее (лишние секунды GPU, деградации) | **`chat_template_kwargs.enable_thinking=false` на каждом запросе со схемой (B3a), `Params.Think=false` — значение по умолчанию всех фаз MVP-1**; стратегия `strip_think` в парсере (B4) остаётся как страховка даже при выключенном thinking; **пин билда llama.cpp** (справочная запись `build/versions.env` у devops) — обновление рантайма может изменить API/грамматику; парсер с восстановлением (ADR-016) + корпус `preamble`; повтор с подсказкой в `user`; метрика `parse.strategy` в `llm.output`; если после замера `valid_first_try < 80 %` — уменьшить `max_tokens`/упростить схему (данные, не код) |
| 3 | VRAM: **(сведение 2)** конфигурация E (`Qwen3.8-27B-UD-Q3_K_XL` 13,1 ГБ + KV 256 КиБ/токен ≈ 2 ГБ @8k / 4 ГБ @16k → 16–19 ГБ) помещается в 24 ГБ с запасом; риск смещается на **скорость** dense-27B (p95 NFR-002) и на запасные по ADR-005 доп. 3 — Qwen3.6-35B-A3B (размещение — `baseline.md` §5), C (`30b-a3b` ≈ 21–23 ГБ, впритык), A | низкая по VRAM / средняя по p95 (NFR-002) | модель и семплинг — параметры блупринта; порядок отката — ADR-005 доп. 3 (E → Qwen3.6-35B-A3B → C → A), итог замера — `baseline.md` §5 *(редакционно, T-440)*; `MV_LLM_NUM_CTX` в env, KV f16/q8_0 — материал замера; переключение на две модели (A/E+) требует router-режима llama-server или второго процесса (devops, `scripts/llm-server.ps1`); I1a/I1b пишутся против `fake`/`recorded` — замер не блокирует |
| 4 | Замер F-8 задерживается → живой S1 в конце I1b без `baseline.md` | средняя / низкое | **стартово E в блупринтах (сведение 2)**; живой прогон допускается с любой конфигурацией порядка ADR-005 доп. 3 (E, Qwen3.6-35B-A3B, C, A; редакционно, T-440); критерий I1 «на записях в CI» не зависит от замера |
| 5 | `Journal.ReadRange`/`End` семантика `membus` ≠ kafka → догон роя ведёт себя иначе на стенде | низкая (contract-тест F-5t) / высокое | R10 integration на testcontainers обязательна до слияния I1; расхождения — запрос к EPIC-001 |
| 6 | `mechanics.Invariants()` EPIC-002 приходит позже стража → `check`-ключи не сопоставлены | высокая в I1a / низкое | страж получает карту инвариантов через `Input`; при пустой карте — правило 6б пропускается с `/health degraded {laws: unknown_check}`; unit-тесты стража на собственных фейковых `Invariant` |
| 7 | Профиль `legacy` нежизнеспособен → S5 не проверяем | средняя / низкое | запасной критерий S5 (КД §17), решение до волны 1 (F-6) |
| 8 | Записи устаревают при изменении промптов/схем | высокая / среднее | ключ записи не зависит от текста промпта; `prompt_hash` расхождение — предупреждение, не ошибка; перезапись — задача с ревью; golden отдельно от записей |
| 9 | Отмена фонового вызова не освобождает GPU (рантайм держит генерацию после закрытия соединения; **проверяется для llama-server, сведение 2**; для Ollama — если F-8 выберет C/A) | низкая–средняя / среднее | замер на стенде в R4; если не освобождает — `MV_SWARM_BACKGROUND_QUIET` увеличить и фон только `rule-only` при активных сессиях (данные/env, не код) |
| 10 | Строки уровней роя в `contracts.OwnershipRules` отстают от блупринтов (таблица одна — `shared/contracts/ownership.go`, ADR-025) | низкая / высокое (State отклоняет предложения роя) | валидатор блупринтов сверяет `owned_entity_types` с таблицей (A2); строку уровня меняет EPIC-003 PR с меткой `contract-change` и ревью system-architect + tech-lead#1, в описании — затронутые блупринты |
| 11 | Раздел `admin` в `api/gateway.openapi.yaml` принадлежит EPIC-004 — гонка правок | низкая / низкое | EPIC-003 передаёт текст маршрутов tech-lead#3 задачей; тест OpenAPI ↔ маршруты у EPIC-004 |
| 12 | `FakeEncounter` (упрощение «встреча по входу без тика») закрепится в ожиданиях бота/State | низкая / низкое | явная пометка `testkit`; e2e `solo-30` на I1 переключается на рой (встреча по тику региона — `background-6h`/S1 с `FireNow`) |

---

## 11. Допущения

1. **Решено сведением 2 (запрос d, C-05 v1.2; допущение снято, внесено tech-lead#2):** F-10 создаёт `FakeNarrator` **v0 без боя** (`WithEncounterStub` **не делается**); **единственная заглушка Phase 1 — `FakeEncounter` EPIC-003 (C4), первая единица I1a**, подпакет `shared/testkit/swarm` вливается в `integration/mvp-1` сразу после приёмки tech-lead#2 (ранний merge, до I1-α), не дожидаясь остального I1a. Запасной вариант при опоздании C4 к 2-й неделе волны 1: временный `encounterStub` **внутри тестов EPIC-002** (не в `shared/testkit`), эскалация через tech-lead#1 к system-architect.
2. `contracts.OwnershipRules` — единственная истина таблицы владения в `shared/contracts/ownership.go` (создана в волне 0 по `data-model.md` §4; ADR-025, подтверждён T-416). `shared/agent/levels.go` своей таблицы не держит, синхронизировать нечего; прежнее допущение «источник с волны 1, синхронизация тестом» отменено.
3. **Стартовая конфигурация моделей — E (сведение 2, U-8, ADR-005 доп. 2 п. 5; внесено tech-lead#2)**: одна `Qwen3.8-27B-UD-Q3_K_XL` (GGUF unsloth) на нативном llama-server, провайдер `openai_compat`, `thinking: false` на все фазы; блупринты MVP-1 задают E до появления `ops/metrics/baseline.md`. Выбор конфигурации — по ADR-005 доп. 3: пороги и длина нарратива, E — базовая на переход, `Qwen3.6-35B-A3B` — целевая, порядок E → Qwen3.6-35B-A3B → C (`qwen3:30b-a3b`) → A (`8b` + `14b`), итог — `baseline.md` §5. Смена конфигурации — правкой пяти YAML-полей блупринтов, без Go *(редакционно, T-440)*. Единственный критерий выбора в задачах — «по `ops/metrics/baseline.md`»; конкретные имена моделей в DoD задач не фиксируются. В CI модели не участвуют (`fake`/`recorded`).
4. Декларативные законы `law-1…` в `laws@v1` — примеры; автор мира (пользователь) уточняет перечень на стенде, не блокирует.
5. `mvctl` — единый бинарник с реестром подкоманд у tech-lead#1; подкоманды EPIC-003 живут в `cmd/mvctl/internal/{record,blueprint,laws}` и регистрируются PR.
6. Профиль `legacy` собирается devops в F-6; EPIC-003 в I1b только подтверждает `gm_path` и deprecated-фильтр; сравнение нарративов legacy/agent — `mvctl report` EPIC-005 (Should; прежнее имя подкоманды `session-report`, T-409).
7. `MemoryClient` HTTP (G3) — за флагом `MV_MEMORY_URL`; при отрезании 005-memory на G3/G4 единица G3 остаётся как интерфейс + `NopMemory` (уже в R9), объём I2 уменьшается.
8. `fsnotify` для FR-091 не добавляем; опрос каталога по `clock.Timers` (5 с) — достаточно (Should).
9. Второй регион для S6 — `blueprints/domain-swamp.md` + записи региона/NPC в `testdata/fixtures` (данные), Go не меняется; тест «второй регион» проверяет `git diff --stat -- '*.go'` = 0.

---

## 12. Отклонённые альтернативы (уровень дизайна эпика)

| Альтернатива | Почему отклонена |
|---|---|
| Выделить `internal/llm` в отдельный эпик | `decomposition-review.md` §2.1: общий владелец, одна ветка, связка `llm.Call/Result` с конвейером; I1a/I1b дают ту же изоляцию без нового контракта в `contracts.md` |
| I1-α на реальном рое с `fake`-провайдером (вместо `FakeEncounter`) | рой готов только к концу I1b (5–6 нед.); I1-α нужна на ~3-й неделе; `FakeEncounter` ≈ 1 единица работы на реальной механике |
| Встреча на I1-α по тику региона | требует Scheduler/`FireNow` — это I1b; для точки достаточно «встреча по входу», бот/State от этого не зависят |
| `swarm.InitWorld` из блупринтов (КД v0.1 §17) | решение G2: один источник инициализации — фикстуры EPIC-002; `npc_table` только респаун |
| Отдельный `core.openapi.yaml` для admin | consolidation D-7: раздел `admin` в `gateway.openapi.yaml`, единый вход харнесса через прокси |
| Валидатор моделей — `error` в рантайме | блупринт без модели у провайдера всё равно полезен (шаблоны, механика); `warning` + `/health degraded` не роняет старт стека без GPU (US-010 критерий 2) |
| Кэш ответов LLM | ADR-016: ломает record-replay; `providers/recorded` — единственный «кэш» |

---

## 13. Решения уровня реализации, принятые в этом дизайне (без отдельного ADR)

1. `FakeEncounter` в `shared/testkit/swarm` — **единственная** заглушка Phase 1 для I1-α (§4.1; C-05 v1.2 — `WithEncounterStub` в F-10 не делается). **Уточнено сведением 2 (запрос e, ADR-001 доп. п. 8; внесено tech-lead#2):** флаг `MV_SWARM_FAKE=true` читает **`cmd/multiverse`** (файл `cmd/multiverse/fake_contexts.go`), а не контекст `swarm`; хук регистрирует `testkit/swarm.FakeContext` (реализация `runtime.Context`) под именем контекста `swarm`; **удаление хука из `cmd/multiverse` — критерий готовности I1**. Файл — совладение с EPIC-001, правка PR через tech-lead#1; исключение `depguard` по файлу оформляет EPIC-001 (`.golangci.yml`).
2. Валидатор правило 7а: `error` в CLI при доступном провайдере, `warning` + `/health degraded {llm: model_missing}` в рантайме, пропуск с `info` при `--offline`/`env.Models == nil`.
3. Запасная конфигурация моделей A = `qwen3:8b` для `tick`, `qwen3:14b` для `phase2`; параметризация — блупринты + `baseline.md`.
4. `region-gm` стартует при отсутствии сущности региона в `WorldView` с `/health degraded {swarm: region_missing}` (не отказ спавна), чтобы порядок старта контекстов не зависел от готовности State.
5. Шаблоны деградации (`template/ru.go`) живут в `shared/testkit/swarm/template` и переиспользуются ролями — один текст шаблонов для I1-α и для деградации роя.
6. Hot reload блупринтов (FR-091) — опрос каталога по `clock.Timers`, без `fsnotify`.
7. Догон роя стартует после `analytics.replay.completed mode=recovery` от State (порядок C-14), но WorldView читает `state/latest.json` раньше — параллельно, чтобы RTO ≤ 2 мин.

Существенные архитектурные решения блока — ADR-014…017 (без изменений после G2); с 2026-09-11 — также **ADR-028** (§14.2).

---

## 14. Решения architect#2 (2026-09-11): окно действий агента встречи и окно до его подъёма

Основание:
- §10 п. 14 и T-427 индекса `tasks.md`;
- контракты: C-01 v1.5, C-05 v1.5, C-14 v1.2;
- ADR-026 и ADR-027 с их «Уточнениями исполнения»;
- T-415: подтверждение tech-lead#2 и ревью #2 (зонд P3);
- код: `shared/eventbus/{dedup,delivery,cause_id}.go` и `shared/testkit/swarm/fake_encounter.go` (образец поведения, не меняется).

Оба решения — уровня реализации блока, контракты не меняются. У system-architect запрошена одна редакционная правка текста C-01 и ADR-027 (§14.1, «Запрос»). T-230 она не блокирует.

**Проверено system-architect#1 (T-431, 2026-09-11): подтверждено с уточнениями — §14.5.**

### 14.1. Окно действий агента встречи: `Has`/`Add`, id из причины, ответ собирается один раз (§10 п. 14)

**Вопрос.** ADR-027 п. 3 оставляет агенту встречи `Seen` до обработки: повтор задвоил бы кости. У двойника T-415 показала цену. Шина отказала в публикации решения или пакета, но id действия уже в окне. Повтор шины отвечает `nil`, письма в `dead_letters` нет, остаётся только след в логе и на `/health`. Нужно решить, как поступает настоящий агент.

**Чем настоящий агент отличается от двойника.** У настоящего агента `Seen` обходится дороже, и причин две.
1. **Окно попадает в снапшот роя** (C-14 v1.2 (а), DoD T-236), а снапшот пишется и при остановке. Остановка отменяет контекст подписок (ADR-023 п. 4), и публикация посреди ответа прерывается. `Deliver` на отменённом контексте событие не паркует и не коммитит (`delivery.go:121-125`), поэтому действие придёт снова после рестарта. Но при `Seen` id действия уже лежит в окне, окно — в снапшоте, и после рестарта действие гасится как отвеченное. На каждом плановом рестарте посреди боя полуопубликованный ответ теряется без следа. Довод tech-lead#2 по T-415 («окно `acted` живёт в памяти, ответ будет дан заново») верен только для двойника. Правило T-223/T-229 «при отмене — `Info`, событие придёт снова» при `Seen` не выполняется.
2. **События ответа ссылаются друг на друга по id.** `combat.decided` ссылается на `dice.rolled`, трофей в пакете — на `combat.decided` (inv-03). Пока id берутся из генератора, повторная сборка ответа даёт другие байты. Повтор тогда задваивает события у потребителей, а не только кости.

**Варианты.**

| Вариант | Отказ шины | Остановка посреди ответа | Потеря и дубли | Replay (NFR-061) | Сложность |
|---|---|---|---|---|---|
| **A. `Seen` до обработки и след** (как двойник; буква ADR-027 п. 3) | повтор отвечает `nil`, ход остаётся полуопубликованным | id действия в окне и в снапшоте → после рестарта действие погашено молча | ход теряется и при отказе, и при остановке; дублей нет | живой прогон опубликовал часть ответа, replay публикует весь → расхождение | минимальная: запоминание и след |
| **Б. `Has`/`Add`, id из причины у всех событий ответа, ответ собирается один раз** | повтор допубликовывает тот же ответ теми же байтами; постоянный отказ → `dead_letters` | id нет в окне → после рестарта ответ допубликовывается теми же id | потери нет; повторная копия несёт тот же id, и её гасит дедупликация потребителя (C-01) | id не зависят от `--id-source` и числа повторов (ADR-027 «Уточнение исполнения» п. 2), живой прогон и replay совпадают побайтно | средняя: слот ответа, курсор публикаций, выбор `parts` |
| **В. Гибрид: `Has`/`Add`, но `Add` и при отказе шины** | как A: отказ не доходит до `dead_letters` | как Б | при отказе ход теряется, как в A | как Б — только для остановки | почти как Б: id из причины и слот в снапшоте нужны всё равно; выигрыш мал |

**Решение — вариант Б.** Правила ниже.

1. **Id всех событий ответа выводятся из причины** (`eventbus.WithCauseID`). Причина — действие игрока; в группе — `round.closed` (T-246).

   | Событие | `parts` | Почему так |
   |---|---|---|
   | `dice.rolled` | индекс броска `roll.index`, сквозной по ответу: удар — 0–1, ответ существа — 2–3, побег — 0 (`diagrams/seq-combat-solo.md`) | ADR-027 п. 2: у `dice.rolled` индекс в `parts` обязателен |
   | `combat.decided` | `exchange.index` | на один обмен приходится два решения (C-05 п. 7), индекс их различает |
   | `entity.update.proposed` (пакет) | номер попытки, 1…3 | повтор при конфликте версий (T-421) — новое событие под тем же `proposal_id`; повтор публикации той же попытки — то же событие |
   | `encounter.ended` | id встречи | решено раньше (ADR-027 «Уточнение исполнения» п. 3, T-422) |

   Части — десятичные строки. Тест: одна причина и одинаковые части дают один id, разные части — разные (ADR-027, «Последствия»). В группе один `round.closed` порождает несколько обменов, поэтому первая часть там — id действующего (T-246).
2. **Ответ собирается один раз.** До первой публикации собираются все события ответа: броски, решения обмена, пакет, а при закрывающем пакете — и событие конца. Собранный ответ лежит в **слоте незаконченного ответа** экземпляра вместе с курсором публикаций. Номер раунда — часть решения (C-05 п. 1г). Пакет применяется к представлению оптимистично, только после его успешной публикации — как у двойника. Повтор публикует события из слота, начиная с курсора, и не решает заново. Факт, пришедший между повторами по другой подписке, решение не меняет. Так под одним id не окажутся разные байты.
3. **Окно действий двухшаговое: `Has`/`Add`.**
   - `Has(id)` — до обработки.
   - `Add(id)` — в одном из трёх случаев: опубликовано последнее событие ответа; действие молча отклонено (C-05 п. 3 — публикаций нет, сорваться нечему); действие принято в очередь отложенных (T-422).
   - Очередь — часть состояния экземпляра и входит в снапшот. Без `Add` при постановке дубль действия встал бы в очередь второй раз, и пакет попал бы в представление дважды.
   - `Seen` в роли `encounter` не используется.
   - Окно отказов (C-05 п. 1б, T-421) устроено так же: `Add` — после публикации повторного предложения или после решения выбросить пакет.
4. **Отказ публикации.**
   - След оставляет `Emitter` по правилу T-223: `Error` с полями `type`, `event_id`, `cause_id`, `err` и признак для `Health()`.
   - Ошибка возвращается той доставке, во время которой сорвалась публикация, а слот остаётся.
   - Любая следующая доставка экземпляру — повтор того же действия, факт или новое действие — сначала допубликовывает слот с курсора. Факты применяются к представлению раньше: сами они ничего не публикуют.
   - Если допубликовать не удалось, доставка отвечает той же ошибкой.
5. **Предел допубликования.** У слота есть счётчик неудачных публикаций. Предел — именованная константа роли, 4: столько раз шина вызывает обработчик (ADR-007 п. 6). На неудаче, которой достигнут предел, ответ сразу выбрасывается так же, как брошенный пакет (T-421, C-05 п. 1г):
   - откатывается всё, что ответ внёс в представление, кроме номера раунда;
   - `Add(id действия)` — частично опубликованный ответ нельзя решать заново под теми же id;
   - в лог идёт `Error` «ответ не допубликован»;
   - решаются отложенные действия.

   Доставка возвращает ошибку и уходит в `dead_letters`. Предел нужен, чтобы дефект сборки события (схема отвергает его на каждой попытке) не отравил бой навсегда.
6. **Остановка.** Публикацию прервала отмена — это проверяется узкой проверкой T-223: `ctx.Err() != nil`, и ошибка — отмена или истёкший срок. Тогда в лог идёт `Info`, следа нет, счётчик не растёт, `Add` не делается. Слот и окно уходят в снапшот остановки (T-236). После рестарта ответ допубликовывается побайтно теми же событиями.
7. **Падение процесса,** когда снапшот старше ответа. Слота нет. Действие не закоммичено и после догона приходит снова, ответ решается заново. По порядку старта C-14 факты всех пакетов, опубликованных до падения, к концу догона уже лежат в `WorldView`. Входы решения совпадают с прежними, броски детерминированы (NFR-060), id выводятся из причины. Ответ совпадает с прежним побайтно, а уже опубликованную часть потребители гасят по id. Что остаётся рискованным — §14.4 п. 1.

**Почему это не изменение контракта.**
- API C-01 не меняется: `Has`/`Add` и `WithCauseID` уже есть (C-01 v1.4).
- Правило C-01 «`Seen` — для тех, кому запоминание до обработки меньшее зло» соблюдено. У агента, чей ответ идемпотентен по построению (п. 1–2), меньшее зло — позднее запоминание.
- Индекс броска в `parts` у `dice.rolled` ADR-027 п. 2 прямо разрешает.
- Потребителям C-05 выполнять ничего нового не нужно. Событие с тем же id и теми же байтами — обычный дубль at-least-once, его гасит дедупликация по id, которую C-01 и так требует от каждого потребителя.
- Фраза C-05 п. 1 «`dice.rolled` и `combat.decided` повторно не публикуются» относится к повтору при конфликте версий. Там она верна и дальше: пакет повторяется под тем же `proposal_id`, решения заново не принимаются.

**Запрос system-architect.** Правка редакционная, T-230 не блокирует.
- В C-01 v1.4 («Дедуп») и в ADR-027 п. 3 устарел пример «(агент встречи: повтор задвоил бы кости)». Предлагаю заменить его в «Уточнении исполнения» ADR-027: «`Seen` — для потребителя, чей побочный эффект нельзя сделать идемпотентным. Агент встречи делает ответ идемпотентным по построению: id из причины у всех событий ответа, ответ собирается один раз (design EPIC-003 §14.1). Поэтому он использует `Has`/`Add`».
- К C-05 п. 1 прошу подтвердить прочтение: «повторно не публикуются» значит «не решаются заново».
- Если system-architect сочтёт замену изменением по существу, действует вариант A, и строки DoD T-230, T-421, T-236 с пометкой «(architect#2, 2026-09-11)» по §14.1 снимаются.

**Цена.**
- Слот ответа, курсор и счётчик — код T-230 и T-421.
- Неверно выбранные `parts` дают тихую ошибку. Страхуют таблица п. 1 и тест на различие частей.
- Потребитель без дедупликации по id после отказа шины увидит дубль решения. Такой потребитель нарушает C-01 уже сегодня: брокер даёт дубли при перебалансировке.

### 14.2. Действие до `encounter.started`: агента встречи поднимает решение открывающего (T-427, ADR-028)

**Вопрос.** Сейчас агента встречи поднимает `encounter.started` (КД §4.1), а по ADR-026 его публикуют только после `entity.created`. Шлюз пропускает `attack` и `flee`, как только сам узнал о встрече — по `encounter.started` или по факту сущности (C-05 п. 3); без встречи он отвечает 409 `not_in_encounter`. Действие и события встречи идут разными топиками — `player_events`, `world_events`, `system_events`, — а порядок между топиками не гарантирован (C-01). Поэтому Router может получить принятое шлюзом действие раньше, чем поднял агента. Получателя у действия нет: GM региона на действия не подписан (КД §5.2), и действие теряется вопреки C-05 v1.5 п. 4.

**Варианты.**

| Вариант | Закрывает окно? | Детерминизм, replay | Сложность и последствия |
|---|---|---|---|
| (а) Агента поднимает предложение создания, пришедшее **из шины** | сужает, но не закрывает: Router читает топик предложений и `player_events` независимо | как сегодня | правка КД §4.1 и §5.1 и триггера блупринта; рою нужна подписка на топик предложений State |
| **(а′) Агента поднимает в процессе решение GM региона открыть встречу, до публикации предложения** | **закрывает.** Цепочка «подъём агента → предложение → факт State → шлюз → удар» начинается с подъёма, поэтому удар застаёт агента при любом порядке чтения топиков | подъём происходит при разборе того же события, что разбирает GM региона (`tick.fired` или вход): в replay путь тот же; у `agent.spawned` id из причины | правка КД §4.1; интерфейс подъёма дочернего агента для роли (T-223, реализация — T-225); триггер блупринта и C-11 не меняются |
| (б) Router держит действие до `agent.spawned` его scope | закрывает, если Router знает, что встреча открывается; иначе нужен срок ожидания | срок ожидания в Router зависит от чередования топиков, которое в replay может быть другим (NFR-061) | вторая очередь, хотя по T-425 п. 5 очередь одна; буфер вне экземпляра приходится отдельно класть в снапшот; новая задача потока рантайма |
| (в) Шлюз пропускает удар только после `encounter.started` | нет: у Router это тоже разные топики | — | меняет C-05 п. 3 v1.4 (шлюзу разрешён факт) и C-04 — контракт другого эпика |

**Решение — (а′)** (ADR-028, диаграмма `architecture/diagrams/seq-swarm-llm-laws-encounter-opening.md`):
1. **Решив открыть встречу, GM региона сначала просит рантайм поднять дочерний агент встречи** — `Spawner.SpawnChild(blueprint, scope, cause)`, реализованный через `Lifecycle.Ensure`. Блупринт ребёнка — `encounter.child_blueprint` региона (поле есть в C-11). Затем GM публикует `entity.create.proposed`. Это вызов рантайма, а не другого агента: правило КД §5.2 «прямых вызовов между агентами нет» соблюдено.
2. **Агент поднимается в фазе «открывается»** (ADR-026 «Уточнение исполнения» п. 4). Он знает id встречи, `proposal_id` создания и NPC. Действия своего scope он откладывает, пока сущности встречи нет в `WorldView` — это уже часть T-422.
   - Пришёл `entity.created` — отложенные действия решаются по порядку.
   - Пришёл `entity.update.rejected` по `proposal_id` создания — встречи нет. Отложенные действия получают молчание (C-05 п. 3–4), агент останавливается с `agent.stopped {reason: error}`, `agent.child_resolved` не публикуется. Отказ создания — дефект (ADR-026 п. 2). `error` уже есть в enum, схема не меняется.
3. **`encounter.started` по-прежнему публикует GM региона** — после `entity.created` (C-05 п. 4, T-232). Подъём агента от этого события больше не зависит.
4. **Триггер блупринта `encounter-wolf` (`event_name: encounter.started`) остаётся страховкой.** `Ensure` идемпотентен: живой агент просто получает событие. Если агента по какой-то причине нет, он поднимется, как раньше.
5. **Подъём идемпотентен.** `agent.id` детерминирован: `{blueprint}:{scope}` (КД §4.1). Повторная обработка открытия после отказа публикации второго экземпляра не создаёт. `agent.spawned` получает id из причины — события, по которому GM открывает встречу, с `parts` = `agent.id`. Повтор публикации даёт тот же id. В фазе догона подъём ничего не публикует (КД §5.1 п. 7, T-224).
6. **Одна встреча за другой в одном scope.** Агент встречи один на scope. Поэтому GM региона не открывает встречу игроку, у которого жив агент встречи в любой фазе, в том числе ждущий остановки. Жив ли агент, GM спрашивает у рантайма через `Spawner.Alive`. Иначе новое открытие досталось бы останавливающемуся агенту. Цена — в худшем случае один лишний активный тик сразу после конца предыдущей встречи. FR-124 не нарушается: он говорит о первой встрече после входа.
7. **Страховочный TTL агента встречи** (30 мин, КД §4.1) останавливает агента, чьё предложение создания так и не дошло до State.

**Новой задачи рантайма не нужно.** Подъём раскладывается по уже нарезанным задачам:
- T-223 — интерфейс;
- T-225 — реализация;
- T-229 — видимость;
- T-232 — вызов подъёма;
- T-422 — фаза «открывается».

### 14.3. Влияние на задачи

Строки внесены в индекс с пометкой «(architect#2, 2026-09-11)».

| Задача | Что меняется |
|---|---|
| T-223 | интерфейс `Spawner` для роли (`SpawnChild`, `Alive`); `Emit` пропускает `DeriveOption` роли (`WithCauseID`, `WithScope`) |
| T-225 | `SpawnChild` идемпотентен; у `agent.spawned` id из причины; в фазе догона подъём не публикуется; `Alive` видит экземпляр в любой фазе |
| T-229 | видимость роли `encounter`: `entity.created` своей встречи и `entity.update.rejected` по `proposal_id` её создания |
| T-230 | окно действий — `Has`/`Add` (§14.1 п. 3); id из причины у `dice.rolled`, `combat.decided` и пакета (п. 1); ответ собирается один раз и публикуется по курсору (п. 2); в тестах агента поднимает тест |
| T-421 | строка «(из T-415)» заменена: постоянный отказ → `dead_letters`, повтор публикует те же события с теми же id; предел допубликования и выброс незаконченного ответа (п. 4–5); окно отказов — `Has`/`Add` |
| T-422 | агент поднимается в фазе «открывается»; отказ создания → молчание отложенным действиям и `agent.stopped {reason: error}`; `Add` при постановке в очередь; открытый вопрос снят |
| T-232 | агент встречи поднимается до публикации предложения; GM не открывает встречу игроку с живым агентом встречи |
| T-236 | в снапшот входят слот незаконченного ответа и очередь отложенных действий агента встречи |
| T-246 | в группе `parts` начинаются с id действующего |
| T-424 | DoD без изменений; уточнён довод «рой поднимает агента по `encounter.started`» |
| T-231 | без изменений |

Размер T-230 остаётся M. Слот ответа — это структура `answer` двойника, к которой добавлен курсор. Выброс незаконченного ответа отдан T-421: механизм выброса уже там.

### 14.4. Риски и допущения
1. **Падение процесса посреди ответа, когда пакет в полёте был выброшен** (§14.1 п. 7). Если State отверг пакет, который был в полёте в момент падения, ответ, решённый заново, может разойтись с уже опубликованной частью под теми же id. Потребители оставят первую копию. Это класс ADR-026 п. 9: «решение описывает исход, которого State не применил». Нужно совпадение трёх редких событий: падения, отказа State и незаконченного ответа. **Условие пересмотра:** первая такая запись на стенде. Тогда догон собирает незаконченные ответы из журнала — события агента с `causation_id` действия, но без пакета — и не решает их заново.
2. **Очередь отложенных действий теряется при падении процесса** (при остановке она сохраняется). Действия в ней уже закоммичены. Окно — один оборот State, ≈ 50 мс. Это свойство принято ещё решением T-425 п. 5, новым оно не является. Ждущий потребитель увидит только истечение срока (C-05 п. 1д).
3. **Допущение:** шина вызывает обработчик последовательно в пределах топика и группы (C-01, предусловие ADR-027 п. 3). На нём стоят п. 4–5 §14.1.
4. **Допущение:** шлюз пропускает `attack` и `flee` только при известной ему встрече (`api-contracts.md` §1.6, 409 `not_in_encounter`; C-05 п. 3). Полнота (а′) держится на нём. Удар без встречи, пришедший в обход шлюза (харнесс, нарушающий C-05 п. 3), получит молчание — так и должно быть.

### 14.5. Проверка system-architect (T-431, 2026-09-11)

**Подтверждено system-architect#1, 2026-09-11, с уточнениями ниже.**
- §14.1 и §14.2 соответствуют C-01 v1.5, C-05 v1.5, C-14 v1.2, ADR-026 и ADR-027 с их «Уточнениями исполнения». ADR-028 принят в том же виде.
- Запрос §14.1 выполнен. Пример у `Seen` заменён (ADR-027, «Уточнение исполнения 2» п. 5; C-01 v1.6). Прочтение C-05 п. 1 «повторно не публикуются» как «не решаются заново» подтверждено (C-05 v1.6).
- Вариант A не восстанавливается. Строки DoD T-230, T-421, T-236 с пометкой «(architect#2, 2026-09-11)» остаются.
- Уточнения обязательны для задач. Текст §14.1–§14.4 выше не переписан; где он расходится с этим разделом, действует раздел.

1. **Router не гасит повтор раньше агента (к §14.1 п. 4–6; C-01 v1.6, «посредник доставки»).**
   - **Проблема.** КД §5.1 шаг 1 запоминает событие в окне Router (`Dedup` по `ev.ID`, окно в снапшоте роя) ещё до доставки. Тогда повтор шины после ошибки агента и повторная доставка после рестарта гаснут у Router и до агента не доходят. §14.1 п. 4–6 при этом не работают: слот не допубликовывается, счётчик предела не растёт, в `dead_letters` ничего нет. Это та же потеря без следа, от которой уходит §14.1.
   - **Правило.** Router спрашивает окно до маршрутизации (`Has`). Запоминает (`Add`) — после того, как все получатели ответили без ошибки. Ошибку получателя он возвращает шине; если ошиблись несколько, возвращается ошибка первого по порядку `Recipients(ev)`. Порядок детерминирован, поэтому текст в `dead_letters` и в логе воспроизводим. Ошибка одного получателя не отменяет доставку остальным. Иначе после повторов в `dead_letters` уйдёт событие, которого не получили и невиновные получатели, — тот же класс, что п. 3 снимает для действий.
   - **Синхронность.** Доставка Phase 1 синхронна: обработчик шины возвращается после ответа получателей. КД §5.1 шаг 5 уже доставляет последовательно на агента.
   - **Идемпотентность повтора — требование.** Повтор выполнит второй раз шаги 3–7 КД §5.1, и каждый из них обязан быть идемпотентным:
     - шаг 3: `WorldView.Apply` — по версии сущности; `ScopeIndex.Apply` — как множество (членство, а не счётчик); `journal.Observe`, `budget.Observe` и `presence.Observe` — по id события. Для `budget.Observe` это новое требование: без него повтор `llm.output` дважды учтётся в `BackgroundBudget` и сдвинет `lod_allowed` в живом прогоне;
     - шаг 4: `Ensure` — по `agent.id`;
     - шаг 6: повтор `tick.fired` агент-адресат гасит по `tick_seq` — тик с номером не больше последнего закрытого — выполненного или прерванного (`tick.aborted`, КД §7.2) — не выполняется (номер монотонен на агента, T-227). Ошибка тика его закрывает: экземпляр отмечает `tick_seq` прерванным и публикует `tick.aborted` с id из причины (`WithCauseID`, `parts` — `tick_seq`). Шине ошибку тика `HandleTick` возвращает, только если `tick.aborted` не опубликован. Тогда повтор допубликовывает `tick.aborted` тем же id, а сам тик не выполняет. Иначе в журнале за `tick.aborted` шли бы результаты того же тика, и догон прочёл бы его иначе, чем живой прогон;
     - шаг 7: `stopSilently` и `Ensure` без публикации — по `agent.id`.

     Получатель, уже ответивший на первую доставку, гасит повтор своим окном (C-01).
   - **Счётчик ошибок роли (T-228; приёмка T-431, TL-1).** Повторы теперь доходят до агента. Поэтому правило «3 ошибки → `agent.stopped reason=error`» считает события, а не доставки. Иначе агент встречи остановился бы на третьей доставке одного действия — раньше предела 4 из §14.1 п. 5.
     - Счётчик растёт на ошибке или перехваченной панике роли при первой доставке события. Повтор того же `ev.ID` его не увеличивает.
     - В счётчик не входят отказ шины при публикации (след `Emitter` T-223; слот и предел §14.1 п. 4–5) и отмена при остановке.
     - Останавливают агента три разных события с ошибкой подряд. Успешная доставка другого события сбрасывает счётчик.
     - Детерминированная паника на одном событии: четыре доставки → событие в `dead_letters` (C-01, после повторов шины), агент жив.
     - Роль остановлена счётчиком во время доставки события. Тогда Router помнит в памяти, до `Add` или рестарта: «у `ev.ID` получатель остановлен с ошибкой». На повторе он возвращает ту же ошибку и не принимает событие за «получателей нет». Иначе повтор не нашёл бы получателя, Router сделал бы `Add`, и событие ушло бы мимо `dead_letters`.
     - Цена: при падении процесса между повторами память Router теряется, и незакоммиченное событие после рестарта коммитится без получателя. След остаётся — `agent.stopped reason=error` и `Error` роли. Класс редкий (три разных ошибки подряд плюс падение между повторами), принято.
   - **Где записано.** По решению оркестратора (ревью #1 T-431, Mi-10) вносит system-architect, отдельной задачи нет. КД §5.1 исправлена: шаги 1, 3, 5 и новый шаг 8. Строки DoD с пометкой «(T-431, 2026-09-11)»: T-224 — окно Router и идемпотентность проекций; T-227 — `budget.Observe` по id события; T-228 — ошибка роли возвращается шине, а не только пишется в лог. Тест T-224: ошибка агента → повтор шины доходит до агента; ошибка на остановке → окно Router не содержит id.
   - **Срок** — до T-230 и T-421. Без этого их тесты допубликования проходят только на агенте, которого поднял тест в обход Router.
2. **Граница ответа — пакет (к §14.1 п. 2–5).**
   - «Последнее событие ответа» — это пакет: каждое принятое действие оставляет пакет (C-05 п. 2).
   - `encounter.ended` собирается вместе с ответом, потому что его id называет `closed_by_event_id`. Но в курсор оно не входит: его публикует состояние «пакет в полёте» после факта (C-05 п. 5).
   - Предел и выброс с откатом (§14.1 п. 5) действуют, только пока пакет не опубликован. Опубликованный пакет решает State, и его судьба определяется C-05 п. 1–1е и 5.
   - Сбой публикации `encounter.ended` после факта ничего не откатывает: встреча уже закрыта в State, и потребитель состояния знает о конце по факту (C-05 п. 5). Событие повторяется со следующей доставкой экземпляру. После предела — `Error`, дальше повторов нет. После рестарта событие с тем же id дописывает сверка C-14 v1.2 (ADR-026 п. 10).
   - `Add(id действия)` делается после публикации пакета, а не после `encounter.ended`.
3. **Невиновное действие не паркуется (к §14.1 п. 4–5; C-05 v1.5 п. 4: «действие не теряется»).**
   - Пока у экземпляра есть незаконченный слот, другое действие не решается. Оно встаёт в очередь отложенных (с `Add` при постановке) — так же, как действие при пакете в полёте (C-05 п. 5), — и его доставка возвращает `nil`.
   - Иначе на пределе в `dead_letters` уйдёт действие, которое никто не решал, а виновное останется без записи.
   - Ошибку незаконченного слота возвращает доставка его причины. Её же возвращает доставка факта или отложенного ответа, если во время неё слот допубликовывается: факт к этому моменту уже применён, и повтор его доставки безопасен.
4. **Открытие встречи тоже собирается один раз (к §14.2 п. 5; ADR-028, «Уточнение исполнения» п. 1).**
   - Id встречи и `proposal_id` создания выводятся из события открытия **и scope игрока или группы** (приёмка T-431, TL-2).
     - Id события предложения создания — `WithCauseID(<тип scope>, <id scope>)` от события открытия. Id встречи — `encounter-<id этого события>`, `proposal_id` — из него же. Нового Go-API не нужно.
     - Формула двойника `encounter-<id события>` верна только для входа, где на причину приходится один игрок.
     - Один `tick.fired` GM региона (`detect_on: tick`, `perception: all`) может открыть встречи двум игрокам. Без scope оба предложения получили бы один id, второе State отверг бы (`duplicate_entity`), и FR-124 для второго игрока был бы нарушен.
   - Выбор NPC и тело предложения записываются в запись «открытие в полёте» до `SpawnChild`. Повтор публикует эту запись, а не решает заново.
   - **Где записано.** Строки DoD T-232 и T-236 (запись «открытие в полёте» входит в снапшот) с пометкой «(T-431, 2026-09-11)». Тест T-232: между отказом публикации предложения и повтором приходит факт о NPC → повтор предлагает ту же встречу с тем же NPC, и агент встречи один.

Риски: цена TTL агента в фазе «открывается» — ADR-028, «Уточнение исполнения» п. 3.
