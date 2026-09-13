# Сценарий роя: ярлыки вызова в нарративе — выдача, проверка, разрешение в id, replay

**Вопрос читателя: где рождаются ярлыки `eK`/`bK`, кто превращает их обратно в id и что происходит с ярлыком, которого не было в промпте?**

Дата: 2026-09-13 · architect#2 (EPIC-003, T-439; приведено к C-07 v1.5 — T-459)
Иллюстрирует: ADR-029 п. 2–8; КД `components/swarm-llm-laws.md` §9.2, §10.2, §11.4, §13.4.2.
Целевой сценарий: пакетов `internal/swarm` и `internal/llm/{prompt,parser,guardian}` в дереве ещё нет (T-209, T-217, T-218, T-233, T-235).

```mermaid
sequenceDiagram
    autonumber
    participant R as роль нарратива (player-gm / group-narrator)
    participant B as сборщик контекста
    participant G as шлюз LLM
    participant P as провайдер (live или recorded)
    participant PA as парсер
    participant GU as страж
    participant BUS as шина

    R->>B: контекст хода
    B->>B: state — сущности по id → e1…eN, absence — события по at и id → b1…bM
    B-->>R: prompt.Sections (ярлыки только в user) + таблица ярлык → id
    R->>G: Call{narrative, Guard{Mentions, AbsenceEventIDs}}
    G->>G: prompt.Build → system (без ярлыков), user, prompt_hash
    alt mode=replay или нарратив из записи
        G->>P: recorded: запись по ключу
        P-->>G: llm.output записи
        G->>G: prompt_hash ≠ записи → warn llm_replay_prompt_drift, labels_hash таблицы ≠ записи → warn llm_replay_labels_drift, массивы пусты, text остаётся
    else live
        G->>P: response_format = статическая narrative.json
        P-->>G: {text, mentions:["e1","e7"], background_refs:["b2"]}
    end
    G->>PA: Parse (схема с pattern) и проверка ярлыка в text
    alt ярлык в text
        PA-->>G: schema_invalid → invalid, повтор с подсказкой, затем шаблон
    else text чистый
        PA-->>G: value
        G->>GU: Evaluate(value, Input)
        GU->>GU: e1 есть в таблице и видим — остаётся, e7 нет в таблице — элемент отброшен, текст остаётся
        GU-->>G: partially_rejected, Rejected[mentions[1], ref e7]
        G->>BUS: llm.output (response_raw с ярлыками, labels_hash)
        G->>BUS: llm.output.rejected: unknown_entity + element + ref (C-07 v1.5)
        G-->>R: Result{text, mentions:[e1], background_refs:[b2]}
        R->>R: b2 → id события по таблице, повторы схлопнуты
        R->>BUS: narrative.output (background_refs — id событий, ярлыков нет)
    end
```

Что читать на диаграмме:
- Ярлыки не выходят за пределы вызова: в `system`, во внешних событиях и в `narrative.output` их нет. В `llm.output.response_raw` они остаются как ответ модели (ADR-029 п. 7).
- Ярлык, которого не было в промпте, стоит отброса одного элемента, а не повтора всего ответа (п. 5). Событие отброса — `unknown_entity` с `ref` (C-07 v1.5, п. 6).
- В replay расхождение `prompt_hash` не останавливает прогон и само по себе ссылок не гасит: ярлыки сверяются по `labels_hash` записи. Разошлась таблица — массивы пусты, чтобы ссылка не ушла другому id (п. 8, C-07 v1.5).
