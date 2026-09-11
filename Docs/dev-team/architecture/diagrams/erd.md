# Сущности мира и их связи

**Вопрос читателя: какие сущности есть в мире, кто ими владеет и как они связаны?**

Дата: 2026-09-11 · system-architect#1
Иллюстрирует: [`../../analysis/data-model.md`](../../analysis/data-model.md) §2–§7, [`../contracts.md`](../contracts.md) C-02 (владение и версии), C-14 (снапшоты).
Проверено по дереву: `shared/entity/{entity.go,types.go,attrs.go,ops.go}`, `testdata/fixtures/{world,region,npc,players}.json`, `schemas/events/entity.*.json`, `rules/dark-forest.yaml`.

Диаграмма разделена на три класса данных, потому что они живут в разных хранилищах и по разным правилам: **состояние мира** (объект на сущность в MinIO, единственный писатель — State), **приватные и служебные данные шлюза** (два файла SQLite, наружу не выходят) и **журнал** (append-only в Redpanda, не сущности, а записи).

```mermaid
erDiagram
    World ||--o{ Region : "содержит"
    World ||--o{ Character : "населяют"
    World ||--|| LawsVersion : "действующая laws_version"
    Region ||--o{ NPC : "домашний регион"
    Region ||--o{ Encounter : "происходит в"
    Character }o--o| Group : "участник"
    Group |o--o| Character : "лидер, может быть null"
    Encounter }o--o{ Character : "участники"
    Encounter }o--o{ NPC : "противники"
    Character ||--o{ Item : "инвентарь, значение внутри сущности"
    NPC ||--o{ Item : "источник трофея, не более одного"

    Link |o--o| Character : "player_id, псевдоним"
    Link ||--o{ CharacterRequest : "идемпотентность создания"
    Session ||--o{ Turn : "ходы"
    Session ||--o{ Round : "раунды, только область группы"
    Round ||--o{ Turn : "действия раунда"
    Turn |o--o| IdempotencyKey : "action_key"
    Character ||--o{ Delivery : "адресат"

    EventRecord ||--o| LLMOutput : "тип llm.output"
    EventRecord ||--o| DiceRoll : "тип dice.rolled"
    EventRecord ||--o| TickRecord : "тип tick.fired"
    Turn ||--o{ EventRecord : "цепочка по correlation_id"
    Blueprint ||--o{ AgentInstance : "экземпляры по блупринту"
    AgentInstance }o--o| AgentInstance : "родитель"
    AgentInstance ||--o{ TickRecord : "тики"
    AgentInstance ||--o{ LLMOutput : "вызовы модели"
    Blueprint |o--o| RulesDocument : "ссылка на правила"
    Blueprint |o--o| LawsVersion : "ссылка на законы"
    LawsVersion ||--o{ LawBreach : "пробой, эпик E-B"
    Snapshot }o--|| World : "снимок мира"
    ContentIncident }o--|| LLMOutput : "карантин фильтра"

    World {
        string id PK
        string laws_version "меняет только world.laws.changed"
        enum weather "владелец - глобальный GM"
        enum time_of_day
        int day "монотонно, от 1"
        string blueprint_ref
        string locale "ru"
        int version "монотонная, +1 на применённое изменение"
    }
    Region {
        string id PK
        string world_id FK
        text description "авторский слой, из блупринта"
        list npc_ids FK
        duration respawn_ttl "кулдаун возрождения, по умолчанию 24 ч"
        list players_present "проекция позиций, проверяется инвариантом"
        timestamp last_background_event_at
    }
    Character {
        string id PK "player_id, псевдоним; не выводится из внешнего ID"
        string world_id FK
        string name "2-32 символа, вводит игрок"
        int hp "0 <= hp <= hp_max, inv-02"
        int hp_max
        int atk_def_dmg_flee "копия чисел правил на момент создания"
        enum status "alive dead abandoned ascended_final; три последних терминальны"
        string position "outside плюс world_id ИЛИ region_id; ровно одна, inv-10"
        object scope "solo с player_id ИЛИ group с group_id; ровно один, inv-06"
        string group_id FK
        string encounter_id FK
        list inventory "значения Item"
        enum actor_kind "human ci sim; не меняется"
        int version
    }
    NPC {
        string id PK
        string region_id FK
        string kind "wolf"
        int hp "и hp_max, atk, def, dmg"
        enum status "alive dead; dead в alive не переходит, inv-09"
        timestamp died_at "для кулдауна возрождения"
        string killed_by FK "для трофея и памяти"
        list loot "выдаётся не более одного раза, inv-03"
    }
    Item {
        string item_id PK
        string kind "wolf-pelt"
        string name
        object source "NPC-источник и combat.decided последнего удара"
        timestamp acquired_at
    }
    Group {
        string id PK
        string leader_id FK "alive-участник ИЛИ null, если живых нет"
        list members "player_id, joined_at, participation, missed_rounds; 1..6, inv-05"
        string position "= позиции каждого участника, inv-04"
        enum state "forming active disbanded"
    }
    Encounter {
        string id PK
        string region_id FK
        object scope "область игрока или группы"
        list participants "состояние, нанесённый урон, время последнего удара"
        list npcs "npc_id и последний ранивший"
        enum state "active resolved"
        enum resolution "npc_dead players_out abandoned"
        int round_seq
    }
    Link {
        string link_id PK "ULID, суррогат; присваивается при первом resolve"
        string platform "telegram"
        string external_id "ЕДИНСТВЕННОЕ место с внешним идентификатором"
        string player_id FK "может быть пустым до создания персонажа"
        enum status "pending_consent consented"
    }
    Delivery {
        string id PK
        string player_id FK "маршрут спрашивается у links в момент выдачи"
        string kind "narrative mechanics"
        enum state "pending delivered dropped"
    }
    LLMOutput {
        string event_id PK
        string provider_model
        string prompt_hash
        string response_hash
        enum validation_status "valid partially_rejected invalid error quarantined filter_error"
        string laws_version
        string phase
        int attempt
        int tokens
        number cost_usd
    }
    Snapshot {
        string key PK "бакет snapshots мира, префикс компонента, метка времени и порядковый номер"
        enum component "state swarm gateway"
        object cursor "офсет следующего сообщения по каждому читаемому топику"
        string state_hash
        string laws_version
    }
```

## Правила, без которых схема читается неверно

1. **Версия — часть контракта, а не служебное поле.** `version` растёт строго на единицу на сущность при непустом наборе изменений; при пустом наборе факт публикуется, а версия не меняется — ход засчитан, движения не было.
2. **Один набор изменений на сущность в одном предложении.** Пакет, называющий сущность дважды, отвергается целиком (`invalid_op`, C-02 v1.3). Схемой это невыразимо — правило обязана соблюдать каждая реализация State.
3. **`Item` — значение внутри `Character.inventory`, а не сущность.** Отдельной сущностью он станет в целевом состоянии; связь «NPC — источник трофея» существует только через `Item.source` и именно ею доказывается единственность трофея.
4. **`Link` ↔ `Character` — единственный мост между внешним миром и псевдонимом**, и он односторонний: у персонажа нет поля со ссылкой на связку.

## Что здесь будущее

- **`LawBreach`** — контракт зарезервирован, издателя нет, фаза пробоя выключена флагом; механика — эпик E-B.
- **`Blueprint`, `AgentInstance`, `RulesDocument` в части блупринтов** — каталогов `blueprints/` и `laws/` в дереве нет; существует только `rules/dark-forest.yaml`.
- **Весь средний блок (`Link`, `CharacterRequest`, `Session`, `Turn`, `Round`, `IdempotencyKey`, `Delivery`)** — ни файлов SQLite, ни миграций.
- **`Snapshot`** — формат описан контрактом и реализован в двойнике (`testkit/state/snapshot.go`), настоящего писателя нет.

## Расхождения с деревом

**Сверка T-409 (2026-09-11, architect#1).** «Закрыто» — документ приведён к дереву; «снято» — расхождения нет; «открыто» — оставлено намеренно.

| # | Статус | Что сделано |
|---|---|---|
| 1 | открыто | не расхождение документа: `data-model.md` описывает модель, фикстуры — стартовое состояние стенда. Нужны ли фикстуры группы и встречи, решает EPIC-002 при `mvctl world init` (T-058) |
| 2 | закрыто | `data-model.md` §2, абзац «Как читать схему»: атрибуты — содержимое карты `Attributes` с геттерами `shared/entity/attrs.go`; опечатка в пути — `invalid_op` в рантайме |
| 3 | закрыто | `data-model.md`: в тексте только целевые имена State и gateway; строка соответствия as-is → целевое оставлена в шапке |
| 4 | снято | `data-model.md` §3.3 и §9.1 называют `ascended_final` резервом E-C; §3.3 теперь указывает функции `shared/entity` |
| 5 | закрыто | `data-model.md` §2: `Scope` → `ScopeRef`, прямо сказано «значение, не сущность» |

Формулировки, записанные при рисовании (до сверки):

1. **Фикстуры описывают меньше, чем модель.** `testdata/fixtures/` содержит `world.json`, `region.json`, `npc.json`, `players.json` и `snapshots/` — сущностей `group` и `encounter` в фикстурах нет; встречу создаёт на лету двойник встречи. Стартовое состояние мира, из которого поднимается стенд, беднее ER-диаграммы.
2. **Атрибуты живут не в полях структуры, а в карте `Attributes`.** `shared/entity.Entity` хранит доменные атрибуты как `map[string]any` с типизированными геттерами (`HP()`, `Status()`, `Position()`, `Scope()`, `Members()`); ER-таблицы `data-model.md` §3 читаются как описание полей структуры Go, а это описание содержимого карты. Практическое следствие: опечатка в пути операции — не ошибка компиляции, а `invalid_op` в рантайме.
3. **`data-model.md` §2 всё ещё называет писателем состояния `entity-manager`, а шлюзом — `game-service`** (as-is имена сервисов). Оба вынесены: `entity-manager` переписывается в `internal/state`, `game-service` — в `internal/gateway`, и ни того, ни другого в дереве целевого кода нет.
4. **`Character.status = ascended_final` объявлен, но недостижим.** `shared/entity/types.go` знает его и включает в `TerminalStatuses`, `CanTransition` его допускает, а перехода в него в MVP-1 нет ни у кого: это резерв под эпик E-C. На диаграмме оставлен, потому что от него зависит формулировка «три терминальных статуса отвечают одинаково» в C-02 и C-03 — и именно она уже реализована в `Actor.Alive()`.
5. **Область (`scope`) на диаграмме — атрибут персонажа, а не сущность**, хотя `data-model.md` §2 рисует `Scope ||--o{ Session`. Отдельной сущности `scope` в реестре типов нет: в конверте события это `ScopeRef` рядом с `world`, в состоянии — атрибут. Диаграмма следует коду.
