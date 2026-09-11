# `testkit/swarm` — временные заглушки роя GM

> **Это временная заглушка точки I1-α, а не часть продукта.**
> Срок снятия и задачи, которые её снимают, — в таблице ниже. Пока она жива,
> всё, что она публикует, — настоящие события реестра с `source: testkit/swarm`
> и `meta.agent.blueprint` с префиксом `fake-` (`contracts.md` §0, исключение
> для заглушек).

Пакет содержит две заглушки и контекст процесса, который их поднимает:

| Что | Файл | Чем заменяется |
|---|---|---|
| `FakeNarrator` — `narrative.output generated_by=template` | `narrator.go` | нарратор роя EPIC-003 (C4, T-220) |
| `FakeEncounter` — бой Phase 1 без роя | `fake_encounter.go` | агент встречи EPIC-003 (I1b) |
| `FakeContext` — `runtime.Context` под именем `swarm` | `fake_context.go` | контекст `internal/swarm` |

## Условие снятия

| Что снимается | Когда | Задача |
|---|---|---|
| e2e `solo-30` переключается с заглушки на реальный рой | I1 | **T-242** |
| хук `cmd/multiverse/fake_contexts.go` (`MV_SWARM_FAKE`) удаляется — критерий готовности I1, тег `mvp-1/i1` | I1 | **T-256** |
| сами заглушки остаются в `shared/testkit/swarm` для unit/e2e на `membus` | — | — |

Основание: `contracts.md` §0 и C-05, `epics/EPIC-003-swarm-llm-laws/design.md`
§4.1, ADR-001 доп. п. 8, `tasks.md` T-219/T-220/T-242/T-255/T-256.

## `FakeEncounter` — что делает

Подписчик `player_events` (и `system_events` — чтобы видеть мир глазами State):

- `player.entered_region` → при живом NPC в регионе и отсутствии активной
  встречи: `entity.create.proposed {encounter}` + `encounter.started {…,
  round{60s, 2}}` (числа раунда — из `rules/dark-forest.yaml`);
- `player.attacked` → `Resolve` → `dice.rolled` (до четырёх: `hit`, `damage`,
  `npc_hit`, `npc_damage`) → `combat.decided` ×2 → один атомарный
  `entity.update.proposed` с `expected_version` → при смерти NPC
  `encounter.ended reason=npc_dead` и трофей тому, кто нанёс последний удар;
  при смерти игрока — `encounter.ended reason=players_out`;
- `player.flee_attempted` → бросок бегства; при успехе — позиция «наружу» и
  `encounter.ended players_out`, при провале — свободная атака NPC;
- `player.rested` — **не обрабатывается**: вне встречи предложение публикует
  gateway (C-02 v1.1), во встрече State его отклоняет.

Каждое принятое действие кладёт в пакет и сущность встречи: номер раунда — на
любом ходу, состояние, исход и ссылку на событие закрытия — на последнем.

Отказ `entity.update.rejected` по причине `version_conflict` — гонка, а не
дефект: удар и факт о мире едут по разным топикам, и порядок между ними C-01 не
гарантирует. Заглушка возвращает свой взгляд на мир к тому, что говорят факты,
пересчитывает пакет по актуальному здоровью и предлагает его снова под тем же
`proposal_id` — не более трёх попыток на действие, дальше внятная ошибка в лог.
Остальные четыре причины отказа (`unknown_entity`, `invalid_op`, `dead_entity`,
`duplicate_entity`) повтору не подлежат: это дефекты, и их надо видеть
(решение оркестратора от 2026-09-11).

Чего сознательно не делает: не читает `MV_SWARM_FAKE` (флаг читает хук T-255),
не ходит в LLM, не открывает встречу по тику региона (упрощение I1-α). Полный
список — в комментарии к типу `FakeEncounter`.

## Как включить

В тестах — напрямую:

```go
fixed, _ := tkmech.Load("rules/dark-forest.yaml")
enc, _ := swarm.NewFakeEncounter(swarm.EncounterConfig{
    Bus: bus, WorldID: "dark-forest-world",
    Rules: fixed.Rules(), Mechanics: fixed,
})
_ = enc.Seed(fixtures)   // testdata/fixtures
_ = enc.Start(ctx)
```

В процессе `core` — хуком `cmd/multiverse/fake_contexts.go` (T-255), который
при `MV_SWARM_FAKE=true` регистрирует `swarm.NewFakeContext(...)` под именем
контекста `swarm`. Сам пакет флага не знает и на `cmd/**` не завязан: это
условие ADR-001 доп. п. 8 — иначе хук нельзя собрать в рабочем бинарнике.
Тесты `TestThePackageCanBeCompiledIntoTheProductionBinary` и
`TestTheStubNamesNoEnvironmentVariable` держат это свойство.
