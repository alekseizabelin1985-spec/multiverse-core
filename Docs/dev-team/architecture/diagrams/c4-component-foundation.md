# C4 уровень 3 — фундамент (EPIC-001)

**Вопрос читателя: из чего собран каркас, который используют все остальные блоки, и что из него уже написано?**

Дата: 2026-09-11 · system-architect#1
Иллюстрирует: [`../components/foundation.md`](../components/foundation.md), [`../contracts.md`](../contracts.md) C-01 (конверт, реестр, шина и журнал), ADR-001 (один модуль, контексты — параметр деплоя), ADR-007 (формат события), ADR-021 (объектное хранилище).
Проверено по дереву: все файлы `shared/{eventbus,contracts,jsonpath,entity,objstore,env,logging,clock,runtime,testkit}`, `schemas/embed.go`, `cmd/multiverse/*`, `cmd/mvctl/*`.

Это единственный блок, который на дату ревизии написан целиком: диаграмма ниже — снимок дерева, а не намерения.

```mermaid
C4Component
    title Фундамент — компоненты, все существуют в дереве

    Container_Boundary(cmd, "cmd") {
        Component(mv, "multiverse", "Go, main.go, serve.go, health.go, db.go, contexts.go", "разбор флагов --contexts/--mode/--bus/--recording/--id-source; сборка Deps; старт и остановка контекстов; подкоманды health и db. contexts.go регистрирует семь ПУСТЫХ контекстов")
        Component(mvctl, "mvctl", "Go, cmd/mvctl", "env check, contracts check, topics, storage init, privacy scan; каркас подкоманд без cobra")
    }

    Container_Boundary(shared, "shared") {
        Component(rt, "runtime", "Go", "Context{Name,DependsOn,Start,Stop,Health}, Deps, Mode, Status, Registry с топологическим порядком, HTTP-сервер процесса и Aggregate для /health, интерфейс Routes")
        Component(eb, "eventbus", "Go", "Event и Meta, NewRoot/Derive, Bus и Journal, kafka-адаптер на kafka-go, Dedup LRU, Chain middleware, обёртка DeadLetter с тремя повторами 100/500/2000 мс, восемь констант топиков")
        Component(ct, "contracts", "Go, santhosh-tekuri/jsonschema/v6", "реестр типов: Spec с Topic, SchemaVersion, Owner, Publishers, Consumers, Policy, Reserved, Deprecated; Lookup, Validate, Topics; OwnershipRules - статичная копия таблицы владения; список допустимых значений Source")
        Component(sch, "schemas", "Go + JSON Schema 2020-12", "embed.go отдаёт FS со схемами; schemas/events - 58 файлов, включая _common.json и _envelope.json")
        Component(jp, "jsonpath", "Go", "Accessor над map[string]any: GetString/GetInt/GetBool/GetSlice/GetMap, доступ по индексу, Has, GetAllPaths")
        Component(ent, "entity", "Go", "Entity, Ref, Op и ApplyOps, Change и ChangeSet, LastChange, CanonicalJSON и StateHash, типизированные геттеры атрибутов, константы типов и статусов")
        Component(os, "objstore", "Go, minio-go v7", "Put/Get/Stat/List/Delete/EnsureBucket/Capabilities; таблица правил бакетов в одном месте; реализация в памяти для тестов; versioning и ILM ставит EnsureBucket")
        Component(env, "env", "Go", "манифест переменных MV_* и внешних, Declare/Validate/CheckExample; секреты не выводятся; сверка с .env.example")
        Component(lg, "logging", "Go, slog", "JSON-логгер, обязательные поля, редакция секретов, BusMiddleware, LogDeadLetter")
        Component(clk, "clock", "Go", "Clock и Timers, Real и Manual; time.Now в internal/* запрещён линтером")
        Component(tk, "testkit", "Go", "membus - шина и журнал в памяти с той же семантикой; contract - общий тест обеих реализаций шины; Versions из build/versions.env; containers для testcontainers; двойники контрактов в подпакетах владельцев")
    }

    ContainerQueue(bus, "Redpanda", "kafka-go", "восемь топиков")
    ContainerDb(minio, "MinIO", "S3", "бакеты платформы")
    Component_Ext(mech, "internal/mechanics", "EPIC-002", "единственный написанный контекст; импортирует entity")

    Rel(mv, rt, "Register, New, StartAll, StopAll, NewHTTP")
    Rel(mv, clk, "Real или Manual в зависимости от --mode")
    Rel(mv, env, "чтение адреса процесса и проверка обязательных переменных включённых контекстов")
    Rel(rt, eb, "поля Deps: Bus и Journal")
    Rel(rt, ct, "поле Deps: реестр")
    Rel(eb, ct, "выбор топика по типу и валидация перед публикацией")
    Rel(ct, sch, "загрузка и компиляция схем")
    Rel(eb, jp, "Event.Path для чтения payload")
    Rel(eb, lg, "BusMiddleware и запись в dead_letters")
    Rel(eb, clk, "паузы между повторами")
    Rel(eb, bus, "Publish, Subscribe, ReadRange, Tail, End")
    Rel(os, minio, "S3 API")
    Rel(tk, eb, "membus реализует Bus и Journal")
    Rel(mech, ent, "Actor из сущности, ops предложений")
    Rel(mvctl, ct, "contracts check, topics")
    Rel(mvctl, env, "env check")
    Rel(mvctl, os, "storage init")
```

## Что здесь будущее

Ничего из нарисованного — все компоненты существуют. Будущее в фундаменте — только то, чего на диаграмме нет:

- **Наполнение семи контекстов.** `cmd/multiverse/contexts.go` регистрирует `state, laws, mechanics, llm, swarm, gateway, memory` как `stub{}`: `Start` ничего не делает, `Health` всегда `ok`. Владельцы заменяют регистрацию в своих ветках.
- **Хук `MV_SWARM_FAKE`.** `contracts.md` §0 и ADR-001 дополнение п. 8 описывают файл `cmd/multiverse/fake_contexts.go`, монтирующий двойников роя в `core`. Файла в дереве нет; двойники работают только в тестах на `membus`.
- **Переезд `membus` в `shared/eventbus/membus`** (C-01 v1.4, ADR-001 доп. 2026-09-11). До него `cmd/multiverse/bus_memory.go` импортирует `shared/testkit` по узкому временному исключению линтера (один файл, пакет `…/testkit/membus$`). В индексе на 2026-09-11 это единственный такой импорт; хук T-255 (`fake_contexts.go`, в работе) добавит второй, и после переезда он останется единственным. После переезда компонент `testkit` на диаграмме теряет `membus`, а `eventbus` его получает.
- **Подкоманды `multiverse db backup|check`** возвращают «в этот процесс не вкомпилирована ни одна база» до появления `internal/gateway`.

## Расхождения с деревом

**Сверка T-409 (2026-09-11, architect#1).** «Закрыто» — документ приведён к дереву; «код» — прав контракт, названа задача; «открыто» — оставлено намеренно.

| # | Статус | Что сделано |
|---|---|---|
| 1 | закрыто (хвост закрыт T-410; сверка T-416) | C-01 v1.3 и `foundation.md` §3: `Deps{Bus, Journal, Contracts, Clock, Timers, Mode, IDs, Log, Mux}`, без `Store` и `Env` — прав код (решение оркестратора). Комментарий над `Deps` в `shared/runtime/runtime.go` исправлен в T-410 |
| 2 | закрыто (T-410; сверка T-416) | `serve.go` заполняет все поля `Deps`: `Bus` и `Journal` — один транспорт по `MV_BUS`/`--bus`, `Contracts` — `contracts.Default()`, транспорт закрывает процесс после остановки последнего контекста (C-01 v1.3a). Остались два пробела процесса, оба названы в C-01 v1.4: глобальные источники конструкторов не ставятся (T-055) и шина в replay получает ручные таймеры (T-060) |
| 3 | закрыто | `state-and-mechanics.md` §2: раскладка `shared/entity` по дереву — `Ref`/`LastChange` в `entity.go`, `Change`/`ChangeSet` в `ops.go`, `path.go` назван |
| 4 | закрыто | подкоманда отчёта — `mvctl report` во всех документах области T-409 (`overview.md`, `infrastructure.md`, `components/*.md`, C-10, `analysis/*.md`); `c4-container.md` поправлен. Хвост закрыт T-416: `plan/epics.md` и `design.md` эпиков приведены; в `tasks.md` эпиков и в `plan/{roadmap,teams,decomposition-review}.md` старое имя осталось — это область тимлида и PM, список правок — в отчёте T-416 |
| 5 | закрыто | `contracts.md` §0: легаси-типы названы; `foundation.md` §6 описывал их и раньше |

Формулировки, записанные при рисовании (до сверки):

1. **`runtime.Deps` не содержит `Store` и `Env`.** C-01 v1.1 перечисляет `Deps{Bus, Journal, Store, Clock, Timers, Mode, IDs, Log, Env, Contracts}`; в коде поля: `Bus, Journal, Contracts, Clock, Timers, Mode, IDs, Log, Mux`. Комментарий в `runtime.go` обещает `Store` и `Env` «в F-5 (T-007)», но T-007 закрыт, а полей нет: контексты будут читать окружение напрямую через `shared/env`, а объектное хранилище создавать сами. Поле `Mux`, наоборот, есть в коде и отсутствует в контракте. Это несовместимое расхождение Go-контракта: правится либо C-01, либо `Deps` — решение за системным архитектором, пока первый контекст его не потребовал.
2. **`serve.go` не строит ни шину, ни журнал, ни реестр**: `Deps` собирается с `Mode`, `IDs`, `Log`, `Clock`, `Timers`, `Mux`. Флаг `--bus=kafka|memory` разбирается и проверяется, но нигде не используется — пустым контекстам шина не нужна. Первый настоящий контекст обнаружит, что подключение шины к `Deps` ещё никто не написал.
3. **`shared/entity` не имеет файлов `ref.go` и `change.go`**, которые перечисляет `components/state-and-mechanics.md` §2: `Ref` и `LastChange` живут в `entity.go`, `Change` и `ChangeSet` — в `ops.go`, плюс есть незадокументированный `path.go`. Расхождение косметическое, но карта файлов в документе уже не годится как навигация.
4. **Имена будущих команд CLI уже заняты, и одно из них не то.** `cmd/mvctl/main.go` держит места под `world`, `blueprint`, `laws`, `record`, `golden`, `llm`, `memory`, `report`, `trace` — занятое имя дешевле, чем спор двух команд на слиянии. Но контракты и план говорят `mvctl session-report`, а зарезервировано `report`: либо документы правятся до реализации, либо владелец EPIC-005 обнаружит расхождение уже в задаче. *(Запись до сверки; имя заменено: `mvctl session-report` → `mvctl report`, закрыто строкой 4 таблицы выше, см. T-409/T-416.)*
5. **Реестр несёт восемь легаси-типов** (`player.moved`, `gm.*`, `narrative.generate`, `violation.detected`) с пометкой `Deprecated` и источником `legacy`, которых нет ни в `contracts.md` §0, ни среди файлов `schemas/events/`.
