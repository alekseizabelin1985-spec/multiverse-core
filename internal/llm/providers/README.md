# `internal/llm/providers`

Реестр провайдеров LLM и две тестовые реализации — `fake` и `recorded`.

Контракты: `Docs/dev-team/architecture/contracts.md` **C-15 v1.2** (интерфейс `llm.Provider`), **C-07 v1.3** (записи `llm.output`, ключ replay) · ADR-005 доп. 2 п. 2, ADR-010 п. 2, ADR-029 (ярлыки вызова) · КД `components/swarm-llm-laws.md` §9.1 · design EPIC-003 §3.1 B2, §4.3.

| Пакет | Что это | Задача |
|---|---|---|
| `providers` | `Registry`: имя из `MV_LLM_PROVIDER` → `Factory` | T-206 |
| `providers/fake` | провайдер тестов: таблица правил, счётчик вызовов, задержка, «грязные» ответы | T-207 |
| `providers/recorded` | провайдер replay: ответ — только из записи `llm.output`, промах — `ErrIncompleteRecord` | T-207 (`Writer` добавит T-221) |
| `providers/openai_compat`, `providers/ollama` | живые рантаймы | T-208, T-254 |

В DoD, КД и контрактах типы тестовых провайдеров названы `FakeProvider` и `RecordedProvider`. В коде это `fake.Provider` и `recorded.Provider`.

## Кто может импортировать

Границы держит `depguard` в `.golangci.yml` (ADR-001 доп. 2026-09-13 п. 3, решение 1c). Этот файл их пересказывает, но не меняет.

| Кто | `providers` | `providers/fake` | `providers/recorded` |
|---|---|---|---|
| `internal/llm/**` (шлюз и сами провайдеры) | да | да | да |
| `internal/memory` (production) | да (`providers$`) | нет | нет |
| `internal/memory` (`_test.go`) | да | да (`fake$`) | нет |
| `internal/swarm` (production) | нет | нет | нет |
| `internal/swarm` (`_test.go`) | нет | да (`fake$`) | нет |
| `internal/replay` (EPIC-002) | нет | нет | нет |
| `cmd/*` | без ограничений | без ограничений | без ограничений |
| `shared/*` | нет (`shared` не видит `internal/*`) | нет | нет |

Следствия:
- **Replay.** `internal/replay` провайдеров не видит. В `mode=replay` провайдер на `recorded` подменяет сам контекст `llm` (шаг 0 КД §9.2, T-212). Записи он получает через `recorded.Source` от проводки процесса.
- **Golden (EPIC-005)** пользуется провайдерами из тестов `internal/llm` (T-244).
- **Рой** в production провайдеров не импортирует: он вызывает `llm.Gateway`.

Ни один провайдер не регистрирует себя в `init`. Какие имена есть в процессе, решает проводка: `providers.Register(llm.ProviderFake, fake.Factory(...))`. Импорт пакета в тесте не добавляет имя в реестр процесса. Проводка, которая регистрирует провайдер сама, не упадёт на повторной регистрации из чужого `init`.

## `providers/fake`

Провайдер без сети, окружения и настенных часов. Безопасен для конкурентного использования.

```go
p := fake.New(
    fake.WithModels("qwen3.8-27b"),                 // что вернёт Models(); без опции — модели из ответов таблицы
    fake.WithTimers(manual.Timers()),               // таймеры задержки; по умолчанию clock.RealTimers
    fake.WithOnDelay(func(llm.Request) { armed <- struct{}{} }),
    fake.WithRules(
        fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{
            {Response: llm.Response{Content: fake.WithThink(`{"events":[]}`)}}, // попытка 1 — «грязный» ответ
            {Response: llm.Response{Content: `{"events":[]}`}},                 // попытка 2 и далее
        }},
        fake.Rule{Phase: llm.PhaseNarrative, Match: fake.UserContains("волк"), Replies: []fake.Reply{
            {Response: llm.Response{Content: fake.Narrative{Text: "Волк рычит.", Mentions: []string{fake.EntityLabel(1)}}.JSON()}},
        }},
        fake.Rule{Replies: []fake.Reply{{Err: llm.ErrUnavailable}}}, // всё остальное
    ),
)
```

### Таблица

| API | Поведение |
|---|---|
| `Rule{Phase, Match, Replies}` | Пустая `Phase` — любая фаза, `nil` `Match` — любой запрос. Ответы выдаются по порядку, последний повторяется. Правило без ответов — паника (ошибка теста). **Отменённый вызов расходует ответ**: и вызов с уже завершённым контекстом, и отменённый во время `Delay`. Повторная постановка уступившего вызова получит следующий ответ правила. |
| `Reply{Response, Err, Delay}` | При `Err` возвращается ошибка. `Delay` ждёт на `Timers` провайдера. Контекст, завершённый во время ожидания, даёт `ctx.Err()` — так проверяется уступка фонового вызова (ADR-014 п. 2). |
| `WithRules(...)`, `(*Provider).Add(Rule)` | Правила добавляются в конец. Отвечает **первое** подходящее правило, поэтому частные правила ставятся раньше общих. |
| Матчеры | `Any()`, `ModelIs(m)`, `CorrelationIs(id)`, `SystemContains(s)`, `UserContains(s)` (только последнее сообщение `user`, куда шлюз кладёт подсказку повтора), `All(...)`. Матчер работает под замком провайдера и не должен вызывать провайдер. |
| Промах | `ErrNoMatch` — ошибка, а не пустой ответ. Вызов засчитывается. |
| Умолчания ответа | `Provider` пустой → `fake`; `Model` пустая → модель запроса; `LatencyMs` 0 → `Delay` в миллисекундах. |

### Наблюдение

| API | Что возвращает |
|---|---|
| `Calls() int` | Число вызовов `Generate`, включая промахи и отменённые (контекст завершён до вызова или во время задержки). Это `llm_calls` NFR-014: в тесте replay должно остаться 0. |
| `CallsFor(phase) int` | То же по фазе. |
| `EmbedCalls() int` | Число вызовов `Embed`. |
| `Requests() []llm.Request` | Копии запросов в порядке поступления. |
| `SetHealth(llm.Status)`, `WithHealth` | Что вернёт `Health`; по умолчанию `ok`. |
| `Embed` | Детерминированный вектор из SHA-256 по `(model, text)`, значения в [-1, 1]. Длина — `WithEmbedDim(n)`, по умолчанию `DefaultEmbedDim` = 8 (для EPIC-005). |
| `Factory(opts...) providers.Factory` | Фабрика для реестра: каждый вызов строит новый `fake` с этими опциями. |

### «Грязные» ответы для тестов парсера (ADR-016 п. 1)

Чистые функции, их можно вкладывать друг в друга. Константы `Preamble`, `Think`, `Trailer` — фиксированные строки: тест называет стратегию, которую ждёт.

| Функция | Результат | Ожидаемая стратегия парсера |
|---|---|---|
| `WithPreamble(body)` | `Preamble + "\n\n" + body` | `balanced_object` |
| `WithTrailingText(body)` | `body + "\n\n" + Trailer` | `balanced_object` |
| `WithThink(body)` | `<think>\n…\n</think>\n\n` + body | `strip_think` |
| `InFence(body)` | тело между строками `` ```json `` и `` ``` `` | `strip_fence` |
| `WithTrailingComma(body)` | запятая перед последней `}` или `]` | `trailing_commas` |
| `JSON(v)` | компактный JSON значения; значение без кодировки — паника | — |

### Ярлыки вызова (ADR-029)

`EntityLabel(k)` → `"e<k>"`, `BackgroundLabel(k)` → `"b<k>"`. `k` не проверяется: ярлык вне `e1…e99` или отсутствующий в таблице вызова нужен тестам стража. `Narrative{Text, Mentions, BackgroundRefs, Tone}.JSON()` — ответ по `schemas/agent/narrative.json`; массивы всегда `[]`, а не `null`.

## `providers/recorded`

Отвечает на вызов записью `llm.output` того же ключа и ничем другим. Ключ — `(meta.correlation_id, meta.agent.id, phase, attempt)` (C-07). Промах — `ErrIncompleteRecord`: не шаблон и не живой вызов (ADR-010 п. 2). После `New` провайдер неизменяем и безопасен для конкурентного использования.

### Источник записей

Провайдер сам не читает ни файлы, ни журнал: формат записи, её чтение и индекс по решению system-architect (T-457, уточняется) переезжают в отдельный пакет `shared/recording`. Записи передаются функцией. `internal/replay` не импортируется.

```go
type Source func(ctx context.Context, h eventbus.Handler) error
```

Контракт `Source` строже, чем у подписки, и `New` на него опирается:
- источник отдаёт **все** события записи или возвращает ошибку. Неполное чтение (не дошли до конца диапазона, остановка) — ошибка, а не успех;
- первая ошибка обработчика возвращается как есть и прекращает чтение: без повторов, backoff и `dead_letters`;
- ошибка самого чтения тоже возвращается.

`eventbus.Journal.ReadRange` совпадает по сигнатуре, но этот контракт не держит. Ошибка его обработчика идёт по пути доставки: повторы, затем `dead_letters`, затем `nil` вызывающему. `membus` при остановке возвращает неполное чтение без ошибки. Если передать `ReadRange` в `New` напрямую, получится неполный индекс без ошибки. Поэтому адаптер над журналом сначала собирает события диапазона, проверяет, что диапазон прочитан целиком, и только потом отдаёт их через `Events` или `Slice`.

| Адаптер | Откуда |
|---|---|
| `Events(iter.Seq[eventbus.Event])` | Любая последовательность событий, уже прочитанных другим пакетом. Последовательность перечитывается при каждом чтении источника; одноразовую можно прочитать один раз — так и делают `New` и `Factory`. |
| `Slice(events...)` | События в памяти, для тестов. |

Какой источник подключает процесс в `mode=replay` и при восстановлении, решает system-architect (T-457). После появления `shared/recording` — адаптер `Source` над ним (T-212 или задача EPIC-001).

### API

| API | Поведение |
|---|---|
| `New(ctx, src) (*Provider, error)` | Читает весь источник. События других типов пропускаются: в `llm_records` есть и `llm.output.rejected`. Из записей с одним ключом остаётся первая: поздняя — повторная доставка. `ErrMalformedRecord` на весь источник, с id события, дают: запись без ключа, неизвестный `validation_status` и нарушение таблицы C-07 v1.3 «поле ↔ статус» (ниже). Запись проверяется до поиска ключа, поэтому битый дубль хорошей записи тоже валит источник. `internal/replay` в этом случае оставил бы первую копию. |
| `WithCall(ctx, agentID, attempt)` | Кладёт в контекст части ключа, которых нет в `llm.Request`. `CorrelationID` и `Phase` берутся из запроса. Шлюз вызывает `WithCall` перед каждым `Generate`, другие провайдеры это значение не читают. |
| `KeyOf(ctx, req) (Key, error)` | Ключ вызова. Пустые агент, `correlation_id`, попытка < 1 или неизвестная фаза — `ErrNoCallKey` (ошибка вызывающего, не `ErrIncompleteRecord`). |
| `Generate` | Статусы `valid`, `partially_rejected`, `invalid` → `Response` записи: `Content` = `response_raw`, `Tokens`, `LatencyMs`, `Provider` и `Model` записи, `ReasoningLen` = `parse.reasoning_len`. `error` → `*FailureError{Key, Code, Message}` (`errors.Is(err, ErrRecordedFailure)`). `quarantined` и `filter_error` → `ErrResponseWithheld`. Решает статус, а не наличие текста: закрытый фильтром текст не отдаётся, даже если запись попала в индекс в обход разбора. Промах → `ErrIncompleteRecord`. |
| `Lookup(Key) (Record, bool)` | Запись целиком: `Status`, `PromptHash` (сверка ADR-029 п. 8), `LawsVersion`, `EventID`, `HasRaw`, `ErrorCode`. |
| `Len()` | Число записей в индексе. |
| `Models` | Модели записей без повторов, по алфавиту. |
| `Health` | `ok`. |
| `Embed` | `ErrIncompleteRecord`: эмбеддинги не записываются (C-07). |
| `Factory(ctx, src) providers.Factory` | Фабрика для реестра. Источник читается **один раз**, при первом вызове фабрики. Следующие вызовы возвращают тот же провайдер или ту же ошибку. Конфигурация блока не используется. |

### Таблица «поле ↔ статус» при чтении

Журнал проверяет схему `llm.output` при доставке, а файл записи при чтении не проверяется. Поэтому `New` сам держит строки C-07 v1.3 (`allOf` схемы `schemas/events/llm.output.v1.json`):

| `validation_status` | обязательно | запрещено |
|---|---|---|
| `valid`, `partially_rejected` | `response_raw`, `filter.status=pass` | `error` |
| `invalid` | `response_raw` | `error` (`filter` — по стадии конвейера, не проверяется) |
| `quarantined` | `filter.status=block` | `response_raw`, `error` |
| `filter_error` | `filter.status=error` | `response_raw`, `error` |
| `error` | `error.code` | `response_raw`, `filter` |

Остальную схему (формы хешей, `params`, `tokens`) провайдер не проверяет: для ключа и ответа она не нужна.

### Пример

```go
// src — Source над пакетом записей (shared/recording после T-457); в тестах — recorded.Slice(events...)
p, err := recorded.New(ctx, src)

ctx = recorded.WithCall(ctx, call.Agent.ID, attempt)
resp, err := p.Generate(ctx, req)
switch {
case errors.Is(err, recorded.ErrIncompleteRecord): // replay: тест падает; восстановление: шаблон с пометкой
case errors.Is(err, recorded.ErrRecordedFailure):  // попытка упала и в записанном прогоне
case errors.Is(err, recorded.ErrResponseWithheld): // статус — из p.Lookup(key)
}
```
