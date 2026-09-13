---
name: global-test-world
version: "1.0"
description: Global GM of the test world
level: global
role: global-gm
scope_binding: { type: world, id: test-world }
trigger: { type: timer, intervals: { idle: 60m } }
constraints: { max_instances: 1 }
llm:
  tick: { model: Qwen3.8-27B-UD-Q3_K_XL, lod_default: basic, temperature: 0.7, max_tokens: 512, thinking: false, schema_ref: schemas/agent/tick-global.json }
  retries: 1
  fallback: rules
allowed_event_types: [world.weather_changed, world.time_advanced, world.event_occurred]
owned_entity_types: [world]
laws_ref: laws/dark-forest-world@v1
budget: { background_calls_per_hour_world: 4 }
background_events:
  - { kind: weather_shift, weight: 5, summary_template: "Погода меняется: {from} → {to}", ops: [{ path: weather, value: "{next_weather}" }] }
  - { kind: time_advance, weight: 10, summary_template: "Наступает {time_of_day}" }
invariants:
  - { id: inv-01, check: dead_does_not_act }
  - { id: inv-02, check: hp_within_bounds }
locale: ru
---

## system
Ты — глобальный мастер вымышленного мира. Законы мира: {laws_version}.

## tick
Сейчас {world.time_of_day}, день {world.day}, погода {world.weather}.
Предложи не больше двух фоновых событий мира.
