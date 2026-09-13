---
name: global-dark-forest-world
version: "1.0"
description: Global GM of dark-forest-world
level: global
role: global-gm
scope_binding: { type: world, id: dark-forest-world }
trigger: { type: timer, intervals: { idle: 60m } }
constraints: { max_instances: 1 }
llm:
  tick:
    # model: per ops/metrics/baseline.md §5 (OQ-A-18; базовая E, запасные по decision_order ADR-005 доп. 3: Qwen3.6-35B-A3B, C, A)
    model: Qwen3.8-27B-UD-Q3_K_XL
    lod_default: basic
    # temperature: non-thinking profile of the gateway (ADR-005 доп. 2 п. 1, swarm-llm-laws.md §13.3)
    temperature: 0.7
    max_tokens: 512
    thinking: false
    schema_ref: schemas/agent/tick-global.json
  retries: 1
  fallback: rules
# Every publication of the role goes through Emitter against this list
# (swarm-llm-laws.md §5.4): the proposal that changes the world
# (api-contracts.md §2.4) as well as the events.
allowed_event_types: [world.weather_changed, world.time_advanced, world.event_occurred, entity.update.proposed]
owned_entity_types: [world]
laws_ref: laws/dark-forest-world@v1
budget: { background_calls_per_hour_world: 4 }
background_events:
  - { kind: weather_shift, weight: 5, summary_template: "Погода меняется: {from} → {to}", ops: [{ path: weather, value: "{next_weather}" }] }
  - { kind: time_advance, weight: 10, summary_template: "Наступает {time_of_day}" }
# The invariants of laws/dark-forest-world@v1 whose check mechanics implements
# (internal/mechanics/invariants.go); id and check repeat the laws file.
# inv-11 (laws_version_current) is enforced by the guardian, not by mechanics.
invariants:
  - { id: inv-01, check: inv-01 }
  - { id: inv-02, check: inv-02 }
  - { id: inv-03, check: inv-03 }
  - { id: inv-04, check: inv-04 }
  - { id: inv-05, check: inv-05 }
  - { id: inv-06, check: inv-06 }
  - { id: inv-07, check: inv-07 }
  - { id: inv-08, check: inv-08 }
  - { id: inv-09, check: inv-09 }
  - { id: inv-10, check: inv-10 }
locale: ru
---

## system
Ты — глобальный мастер вымышленного мира «Тёмный лес». Все персонажи мира — взрослые, реальных людей, мест и событий в нём нет.
Законы мира версии {laws_version} — истина: ты их не меняешь и не нарушаешь.
Ты ведёшь только фон мира — погоду, ход времени и происшествия; действия и слова игроков не описываешь.
Пиши по-русски, коротко и без разметки.

## tick
Сейчас {world.time_of_day}, день {world.day}, погода: {world.weather}.
Недавние события мира: {events}
Предложи не больше двух фоновых событий мира: смену погоды, ход времени или происшествие.
Каждое событие опиши одной фразой. Если в мире ничего не меняется, верни пустой список событий.
