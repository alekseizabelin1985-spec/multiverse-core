# Сценарий роя: открытие встречи и удар до `encounter.started`

**Вопрос читателя: кто и когда поднимает агента встречи и почему удар, принятый шлюзом, не теряется, даже если придёт раньше `encounter.started`?**

Дата: 2026-09-11 · architect#2 (EPIC-003)
Иллюстрирует: ADR-028, design EPIC-003 §14.2, C-05 v1.5 п. 3–4, ADR-026 и его «Уточнение исполнения» п. 3–4.
Целевой сценарий: `internal/swarm` в дереве нет. У двойника `shared/testkit/swarm/fake_encounter.go` открывающий и агент встречи — один объект, поэтому окна там нет по построению.

```mermaid
sequenceDiagram
    autonumber
    participant IN as Router роя (tick.fired или вход игрока)
    participant RG as GM региона
    participant LC as Lifecycle роя
    participant EN as агент встречи
    participant BUS as шина
    participant ST as State
    participant G as gateway

    IN->>RG: tick.fired или player.entered_region
    RG->>RG: NPC свободен, у игрока нет живого агента встречи
    RG->>LC: Spawner.SpawnChild(encounter-wolf, solo:player-A, причина)
    LC->>EN: Ensure - фаза «открывается»: id встречи, proposal_id создания
    LC->>BUS: agent.spawned, id из причины
    RG->>BUS: entity.create.proposed encounter, opened_by_event_id
    BUS->>ST: предложение

    alt State создал встречу
        ST->>BUS: entity.created encounter
        BUS->>G: entity.created - шлюз знает встречу
        G->>BUS: player.attacked
        BUS->>EN: player.attacked - получатель уже есть
        alt сущности встречи ещё нет в WorldView агента
            EN->>EN: в очередь отложенных, Add(id действия)
            BUS->>EN: entity.created
            EN->>EN: фаза «идёт», отложенные решаются по порядку
        else сущность уже есть
            EN->>EN: решить сразу
        end
        BUS->>RG: entity.created
        RG->>BUS: encounter.started, id из причины, после факта
        Note over EN: для живого агента encounter.started - обычное событие,<br/>триггер блупринта - страховка, Ensure идемпотентен
    else State отверг создание
        ST->>BUS: entity.update.rejected по proposal_id
        BUS->>RG: отказ - encounter.started нет, Error, NPC свободен
        BUS->>EN: отказ - отложенные получают молчание
        EN->>BUS: agent.stopped reason error
    end
```

## Почему так

1. **Агент поднимается до публикации предложения.** Удар проходит цепочку «предложение → факт State → шлюз → удар», и в её начале агент уже есть. Поэтому удар застаёт его, как бы Router ни читал разные топики (C-01: порядок только внутри топика).
2. **Очередь одна — у агента** (T-425 п. 5). Router ничего не держит и сроков не ждёт.
3. **`encounter.started` публикует GM региона после факта** (C-05 п. 4). Подъём агента от этого события больше не зависит.
4. **Отказ создания — дефект** (ADR-026 п. 2). Агент останавливается с `reason: error`, этот enum в схеме уже есть.
