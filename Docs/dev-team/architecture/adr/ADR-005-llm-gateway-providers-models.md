# ADR-005: LLM-шлюз — абстракция провайдера, Ollama native API со structured output, размещение моделей на RTX 4090, облако за флагом

Статус: предложено (к утверждению на G2) · Дата: 2026-09-09 · Автор: system-architect#1
Связи: OQ-A-05 (часть LLM/эмбеддинги), OQ-D-13, OQ-P-07, OQ-R-10, OQ-A-18 (новый: базовая конфигурация моделей); FR-015, FR-032, FR-034, FR-050, FR-051, FR-070…FR-072; BR-14, BR-15; NFR-002, NFR-004, NFR-006, NFR-007, NFR-022, NFR-046, NFR-050…NFR-053, NFR-076, NFR-090; `integrations.md` §2; `domain-review.md` DR-22.

## Контекст

As-is: четыре LLM-клиента (`shared/oracle`, `shared/intent`, `orchestrator/oracle.go`, `oracle_llm_adapter.go`), все через OpenAI-совместимый `/v1/chat/completions`, без учёта токенов, бюджета, записи ответов; модели в блупринтах `qwen:7b/72b` (нет в линейке Qwen3). Решения: локальный Ollama по умолчанию, облако через абстракцию; две модели Qwen3 одновременно (8b + 30b-a3b/32b) на RTX 4090 24 ГБ; structured output и `thinking off` для Phase 1 (DR-22); пороги — после замера.

Проверка размеров (Q4_K_M, публичные данные Ollama/HF, сентябрь 2026): `qwen3:30b-a3b` ≈ 18,6 ГБ веса + KV-кэш ≈ 2–4 ГБ при 8–16k контекста; `qwen3:8b` ≈ 5,2 ГБ; `qwen3:14b` ≈ 9,0 ГБ; `qwen3:32b` ≈ 20 ГБ. Ollama грузит модель в VRAM целиком, если помещается, иначе молча выгружает часть слоёв в RAM (`num_gpu` управляет вручную); `OLLAMA_MAX_LOADED_MODELS` (по умолчанию 3) и `keep_alive` держат модели резидентными. Structured output (`format` = JSON-схема) поддерживается с Ollama 0.5; для Qwen3 есть открытые issue о «преамбуле» перед JSON на части версий — нужен парсер с восстановлением.

## Рассмотренные варианты

| Вопрос | Вариант | Плюсы | Минусы |
|---|---|---|---|
| API Ollama | OpenAI-совместимый `/v1/chat/completions` | один клиент на локаль и облако | нет `think`, `keep_alive`, `format`-схемы (частично), `num_gpu` |
| | **native `/api/chat` для Ollama + отдельные адаптеры для облака** | полный контроль (`think:false`, `keep_alive`, `format`, `options`) | два формата запросов (скрыто за интерфейсом) |
| Размещение моделей | 8b + 30b-a3b полностью в VRAM | как решено р5 | ≈ 24–28 ГБ с KV-кэшем — не помещается |
| | 8b + 30b-a3b, MoE частично в RAM | «две модели» формально | скорость Phase 2 зависит от выгрузки; замер |
| | **одна 30b-a3b на нарратив и тики** | помещается; 3B активных — тики дёшевы; одна модель в прогреве | нет «малой модели» для Phase 1 с LLM (в MVP-1 Phase 1 без LLM — не нужна) |
| | 8b + 14b | помещается с запасом | качество русского у 14b — замер |
| Фильтр (a) | вне шлюза (в Gateway при доставке) | ближе к клиенту | `response_raw` успел бы записаться (NFR-048) |
| | **в шлюзе до записи `llm.output`** | fail-closed до сохранения | шлюз знает о контенте (приемлемо: это его middleware) |

## Решение

1. **Интерфейс** `internal/llm`:
   ```go
   type Provider interface {
       Generate(ctx, Request) (Response, error) // Request{Phase, Model, System, Messages, Schema json.RawMessage, Params{Temperature, MaxTokens, Think bool}, Timeout, CorrelationID}
       Embed(ctx, model string, texts []string) ([][]float32, error)
       Health(ctx) Status                       // модели загружены/резидентны
   }
   ```
   Реализации: `providers/ollama` (native `/api/chat`, `/api/embed`, `format`=схема, `think`, `keep_alive=-1`, `options.num_ctx`), `providers/openai_compat` (OpenAI, DeepSeek; `response_format: json_schema`), `providers/anthropic` (целевое), `providers/recorded` (replay), `providers/fake` (тесты). Выбор — конфигурацией `LLM_PROVIDER=ollama|openai|deepseek|anthropic`, модель — из блупринта на фазу/уровень.
2. **Конвейер шлюза (middleware, порядок фиксирован)**: бюджет (окно по `(world, level, phase)`; фон — `B`/час/мир; облако — денежный лимит) → провайдер с таймаутом фазы (ориентиры: phase1 5 с, phase2 20 с, tick 30 с; уточняются замером) и повторами `attempt ≤ retries` → парсер JSON с восстановлением (обрезка преамбулы/код-блоков, повтор при `schema_invalid`) → проверка языка (0 CJK, латиница ≤ порога) → **фильтр категории (a)** (словарь/регулярные выражения из `config/absolute-limits.yaml`, fail-closed) → запись `llm.output` (при `quarantined`/`filter_error` — без `response_raw`, плюс `content.incident.recorded`) → страж (`guardian`: сущности существуют и видимы, нет `player.*`, типы ∈ `allowed_event_types`, сущности ∈ `owned_entity_types`, инварианты законов, `laws_version` актуальна) → результат конвейеру агента. `llm.output.rejected` публикуется на каждый отклонённый элемент/вызов с `reason` из перечисления FR-034.
3. **Учёт**: `tokens{prompt, completion}` из ответа провайдера, `cost_usd` по таблице цен в конфигурации (0 для Ollama), `latency_ms`, `provider`, `model`, `attempt`; агрегаты доступны `mvctl llm usage` и `/v1/admin/llm/usage` (Should).
4. **Промпт**: секции `<laws laws_version>`, `<canon>`, `<state>`, `<events>`, `<absence>`, `<player_text>` (реплики и имена — как данные, экранированы), `<absolute_limits>` (короткая позитивная формулировка BR-08 из конфигурации), `locale`; `prompt_hash` = SHA-256 полного промпта; полный текст — в `prompts-{world}` по флагу. Основа — `prompt_builder.go` narrative-orchestrator (структурированные секции, тесты) → `internal/llm/prompt`.
5. **Размещение моделей**: базовая конфигурация выбирается **после матрицы замера** (`overview.md` §18.1): A `8b+14b`, B `8b+30b-a3b (частично в RAM)`, C `30b-a3b одна`, D `32b`. Рекомендация архитектора — C при прохождении NFR-002/NFR-090, иначе A; решение фиксируется в `ops/metrics/baseline.md` и блупринтах (OQ-A-18). Compose: `OLLAMA_KEEP_ALIVE=-1`, `OLLAMA_MAX_LOADED_MODELS=2`, `OLLAMA_NUM_PARALLEL=1`, прогрев моделей в `make up` (`/api/generate` пустой промпт) — NFR-004.
6. **Облако** (BR-15, NFR-046): включается `LLM_CLOUD_ENABLED=true` + ключ в env; при числе связок с `alive`-персонажем > 1 требуется `LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS=true` — факт включения пишется в `system_events` (`config.cloud_enabled {by: operator}`); в промпт уходят только `player_id` и имена персонажей; лимит `LLM_CLOUD_BUDGET_USD_PER_DAY` → `budget_exceeded` → локальный провайдер/шаблон.
7. **Деградация** (BR-14, FR-015): любой `error`/таймаут после повторов → `ErrUnavailable` → конвейер агента публикует `narrative.output generated_by=template fallback_reason=…`; тики — LOD 1; `/health.llm=unavailable`; восстановление без рестарта.

## Последствия

- Позитивные: один клиент вместо четырёх; учёт и бюджет в одном месте; record-replay и фильтр — гарантированно до использования ответа; смена модели — правка блупринта.
- Негативные: native API Ollama — отдельный адаптер (≈ 300 строк); риск «преамбулы» у Qwen3 — парсер с восстановлением и золотой набор NFR-065.
- Что придётся сделать: EPIC-001 — матрица замера и `baseline.md`; EPIC-003 — шлюз, провайдеры `ollama`/`recorded`/`fake`, страж, фильтр (a), `config/absolute-limits.yaml`; E-H — `openai_compat`/`anthropic` как рабочие опции.

## Дополнение 2026-09-09 (сведение A3 шаг 4)

1. **Имена переменных — с префиксом `MV_`**: `MV_LLM_PROVIDER`, `MV_LLM_CLOUD_ENABLED`, `MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS`, `MV_LLM_CLOUD_BUDGET_USD_PER_DAY`, `MV_LLM_STORE_PROMPTS`, `MV_LLM_KEEP_ALIVE`, `MV_LLM_NUM_CTX`, `MV_LLM_TIMEOUT_*`, `MV_LLM_LATIN_MAX_RATIO`, `MV_LLM_TURN_CALLS_PER_MIN`, `MV_OLLAMA_URL`, ключи `MV_OPENAI_API_KEY`/`MV_DEEPSEEK_API_KEY`/`MV_ANTHROPIC_API_KEY`. Все имена в тексте решения выше читать с префиксом; сторонние `OLLAMA_KEEP_ALIVE`, `OLLAMA_MAX_LOADED_MODELS`, `OLLAMA_NUM_PARALLEL` — без префикса (переменные самого Ollama). Правило платформы — `shared/env.Declare` (`contracts.md` §16 п. 5).
2. **Базовая конфигурация моделей — решение пользователя (OQ-A-18)**: **одна `qwen3:30b-a3b`** на нарратив и тики (вариант C), **если** проходит замер NFR-002 (p95 нарратива) и NFR-090 (русский) по матрице F-8; **иначе — `qwen3:8b` + `qwen3:14b`** (вариант A). Варианты B и D — только как материал замера. Блупринты MVP-1 до `baseline.md` задают C; `OLLAMA_MAX_LOADED_MODELS=2` остаётся (для A).
3. **Валидация модели и запрет неявного облака** (SEC-21, п. 6 уточнение): `Provider.Models(ctx)` возвращает список доступных моделей; валидатор блупринтов (C-11) отклоняет модель, отсутствующую у выбранного провайдера; блупринт **не содержит** поля провайдера; включение облака — только `MV_LLM_CLOUD_ENABLED=true` оператором с записью `config.cloud_enabled {by: operator, provider}` в `system_events` (без ключей и их фрагментов в событии и логах). `config.cloud_enabled` — тип реестра, издатель — контекст `llm` (EPIC-003).
4. **Промпт (п. 4 уточнение, SEC-17)**: факты памяти `source=generated` (C-09) и сводка `<absence>` — данные: секция `<memory>` с тем же экранированием `<`/`>` и лимитом длины, что `<player_text>`; e2e `injections-10` содержит ≥ 3 инъекции второго порядка через память.
