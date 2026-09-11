# C4 уровень 3 — состояние и механика (EPIC-002)

**Вопрос читателя: кто и как превращает предложение об изменении в факт, и откуда берутся числа боя?**

Дата: 2026-09-11 · system-architect#1
Иллюстрирует: [`../components/state-and-mechanics.md`](../components/state-and-mechanics.md) §2–§7, [`../contracts.md`](../contracts.md) C-02 (предложения и факты), C-03 (Go-API механики), C-14 (снапшоты и восстановление), ADR-003, ADR-011, ADR-012, ADR-013, ADR-024.
Проверено по дереву: `internal/mechanics/*.go` (10 файлов), `shared/entity/*.go`, `rules/dark-forest.yaml`, `shared/testkit/{state,mechanics}/`.

```mermaid
C4Component
    title core — компоненты State и Mechanics; написан только правый блок

    Container_Boundary(state, "internal/state — БУДУЩЕЕ, каталога в дереве нет") {
        Component(worker, "worker", "Go", "последовательная обработка предложений одного мира, почтовый ящик; чтение system_events с курсора")
        Component(prop, "proposal", "Go", "разбор entity.create.proposed и entity.update.proposed в типизированное предложение")
        Component(apply, "apply", "Go", "Validate, применение на копиях, инварианты, запись, публикация факта - в этом порядке")
        Component(own, "ownership", "Go", "level_violation по contracts.OwnershipRules")
        Component(inv, "invariants", "Go", "адаптер: реестр механики над проекцией мира - law_violation")
        Component(world, "world", "Go", "рабочий набор мира в памяти и индексы по типу, трофеям, группам")
        Component(store, "store", "Go", "интерфейс Store и реализация над objstore; объект на сущность")
        Component(intent, "intent", "Go", "намерение атомарного пакета: журнал из одного объекта, докат вперёд")
        Component(snap, "snapshot", "Go", "сборка снапшота, state_hash, объект и указатель latest.json, ротация K=5")
        Component(rec, "recovery", "Go", "протокол старта: указатель, снапшот, догон фактов, сверка, analytics.replay.completed")
        Component(dd, "dedup", "Go", "дедуп proposal_id и event.id")
    }

    Container_Boundary(mech, "internal/mechanics — НАПИСАН") {
        Component(rules, "rules.go", "Go, YAML", "Rules, Load и LoadBytes, компиляция rules/dark-forest.yaml, Stats, Attack, Flee, Rest, Loot, Round, DamageDice, ClampHP, FleeThreshold, Excluded")
        Component(form, "formula.go", "Go", "мини-грамматика вместо движка выражений: DiceExpr и CheckExpr, разбор, вычисление, порог")
        Component(rng, "rng.go", "Go, math/rand/v2 PCG", "Seed = SHA-256 от event_id и индекса броска, NewRNG, Roll, RollCheck - единственная реализация детерминизма")
        Component(res, "resolve.go", "Go", "Resolve - таблица исходов действия. СЕГОДНЯ ВОЗВРАЩАЕТ ErrNotImplemented, задача T-053")
        Component(chg, "changes.go", "Go", "ChangesFor - операции предложения из исхода. СЕГОДНЯ ВОЗВРАЩАЕТ ErrNotImplemented, задача T-053")
        Component(tgt, "target.go", "Go", "NPCTarget: последний ранивший, минимум hp, порядок id")
        Component(act, "actor.go", "Go", "ActorFromEntity: актор из сущности игрока и сущности встречи")
        Component(de, "dice_event.go", "Go", "DiceRolledPayload - единственный конструктор payload dice.rolled")
        Component(invr, "invariants.go", "Go", "реестр inv-01…inv-10: идентификаторы и места проверки. Функции проверки пока nil, задача T-054")
    }

    Component_Ext(ent, "shared/entity", "EPIC-001, написан", "Entity, Ref, Op и ApplyOps, Change, LastChange, StateHash, геттеры атрибутов")
    Component_Ext(ctr, "shared/contracts", "EPIC-001, написан", "реестр типов, OwnershipRules")
    Component_Ext(fs, "testkit/state.FakeState", "написан", "двойник State: применяет предложения в память, публикует факты, при старте публикует analytics.replay.completed mode=recovery")
    Component_Ext(fm, "testkit/mechanics.FixedMechanics", "написан", "двойник механики: табличные исходы по seed, пока Resolve не написан")

    ContainerQueue(bus, "Redpanda", "C-01", "читает system_events; публикует entity.created, entity.updated, entity.update.rejected, snapshot.created, analytics.replay.completed")
    ContainerDb(minio, "MinIO", "objstore", "entities-{world} - объект на сущность; snapshots-{world}/state - снапшоты и latest.json")

    Rel(worker, prop, "разбор входящего предложения")
    Rel(prop, apply, "типизированное предложение")
    Rel(apply, own, "кто это предлагает и что ему позволено")
    Rel(apply, inv, "инварианты на копиях до записи")
    Rel(apply, world, "рабочий набор и версии")
    Rel(apply, store, "запись объекта сущности")
    Rel(apply, intent, "намерение атомарного пакета до первой записи")
    Rel(apply, dd, "повтор proposal_id не применяется дважды")
    Rel(apply, bus, "факт публикуется ПОСЛЕ успешной записи")
    Rel(inv, invr, "реестр инвариантов")
    Rel(own, ctr, "OwnershipRules")
    Rel(world, ent, "Entity и ApplyOps")
    Rel(store, minio, "Put, Get, List")
    Rel(snap, minio, "объект снапшота, затем указатель latest.json")
    Rel(rec, minio, "чтение указателя и снапшота")
    Rel(rec, bus, "догон фактов через Journal.ReadRange до End")
    Rel(res, form, "формулы броска и проверки")
    Rel(res, rng, "детерминированные броски")
    Rel(act, ent, "чтение hp, def, статуса, участия")
    Rel(chg, ent, "операции set, inc, append")
    Rel(fs, bus, "двойник издаёт те же факты")
    Rel(fm, res, "подменяет Resolve на время")
```

## Порядок, который эта диаграмма закрепляет

`Validate → применение на копиях → инварианты → запись → публикация`. Факт публикуется **после** успешной записи в объектное хранилище, а не до: истина — снапшот, журнал лишь способ его догнать. Из этого следует и запрет на отмену контекста обработчика при `Close` (ADR-023): обработчик посреди записи обязан её закончить.

## Что здесь будущее

Весь левый блок. Каталога `internal/state` в дереве нет; нет и `internal/replay` (`Cursor`, `EventClock`, `NullTimers`, чтение и запись JSONL-записей), поэтому он на диаграмме не показан вовсе — рисовать нечего, а его роль в старте описана в [`seq-recovery.md`](seq-recovery.md).

Роль State сегодня играет `shared/testkit/state.FakeState`: применяет операции в память, публикует `entity.created`/`entity.updated`, отвергает пакет, дважды называющий одну сущность, и при старте объявляет `analytics.replay.completed {mode: recovery}`. Чего у двойника нет: инвариантов по умолчанию (включаются `WithInvariants()`), объектного хранилища, намерений атомарного пакета, снапшотов и настоящего восстановления.

## Расхождения с деревом

**Сверка T-409 (2026-09-11, architect#1).** «Закрыто» — документ приведён к дереву; «код» — прав контракт, названа задача; «открыто» — оставлено намеренно.

| # | Статус | Что сделано |
|---|---|---|
| 1 | код (T-053) | C-03 «Состояние реализации» и шапка `state-and-mechanics.md` больше не выдают `Resolve` за готовую функцию; реализует EPIC-002 T-053 |
| 2 | код (T-053) | там же; при появлении `ChangesFor` сверить форму предложения с двойником встречи — записано в C-03 |
| 3 | код (T-053) | уже был в C-03 v1.2 и ADR-024 как единственная правка кода; не менялось |
| 4 | закрыто, хвост в коде (T-054) | `state-and-mechanics.md` §5.7: пометка «`Check == nil` у всех десяти, таблица — целевая»; проверки пишет T-054 |
| 5 | открыто | файл законов — EPIC-003 T-205; пометка в §5.7 и в шапке `swarm-llm-laws.md` |
| 6 | закрыто | `state-and-mechanics.md` §2: `internal/mechanics/types.go` внесён в раскладку |

Формулировки, записанные при рисовании (до сверки):

1. **`Rules.Resolve` не реализован** — возвращает `ErrNotImplemented`. То есть в дереве нет ни одной строчки, решающей исход удара: бой в тестах решает `testkit/mechanics.FixedMechanics` по таблице. `contracts.md` C-03 описывает `Resolve` как готовую функцию с гарантией детерминизма; гарантия сегодня держится на `Seed`/`NewRNG`, которые написаны и покрыты тестом, но сам исход — нет.
2. **`ChangesFor` не реализован** — возвращает `ErrNotImplemented`. Двойник встречи собирает `ops` предложения руками; значит, форма предложения сегодня закреплена не механикой, а заглушкой, и при появлении настоящей функции их придётся сверять.
3. **`NPCTarget` имеет сигнатуру без канала ошибки** — `func (r *Rules) NPCTarget(npc *Actor, candidates []*Actor) *Actor`. `contracts.md` C-03 v1.2 и ADR-024 требуют `(*Actor, error)` и прямо называют это единственным местом правки кода (задача T-053). Расхождение известно и не закрыто; до правки вызывающий не отличает «некого кусать» от «механика ещё не написана».
4. **Реестр инвариантов существует, проверки — нет**: у всех десяти записей `Check == nil` (задача T-054), причём у `inv-07` и `inv-08` он останется `nil` навсегда — они проверяются по журналу, а не по миру. Документ `components/state-and-mechanics.md` §5.7 описывает реестр так, будто проверки в нём есть.
5. **Файла `laws/dark-forest-world.v1.yaml` нет**, хотя комментарий в `invariants.go` называет его единственным местом, где живёт текст закона, и обещает контрактный тест `mvctl laws check`, сравнивающий два набора идентификаторов. Сравнивать пока не с чем.
6. **`internal/mechanics/types.go` не упомянут** в раскладке `components/state-and-mechanics.md` §2, хотя именно там живут `Actor`, `Action`, `Outcome`, `Roll`, `ProposedChange` и `ErrNotImplemented` — то есть весь публичный словарь C-03.
