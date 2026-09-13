---
name: global-test-poor
version: "1.0"
description: A world without a background budget or background events and with an unknown invariant (rule 12)
level: global
role: global-gm
scope_binding: { type: world, id: test-world }
trigger: { type: timer, intervals: { idle: 60m } }
constraints: { max_instances: 1 }
llm:
  tick: { model: Qwen3.8-27B-UD-Q3_K_XL, lod_default: basic, temperature: 0.7, max_tokens: 512, thinking: false, schema_ref: schemas/agent/tick-global.json }
  fallback: rules
allowed_event_types: [world.weather_changed]
owned_entity_types: [world]
laws_ref: laws/dark-forest-world@v1
budget: { background_calls_per_hour_world: 0 }
invariants:
  - { id: inv-01, check: dead_does_not_act }
  - { id: inv-99, check: hp_never_negative }
locale: ru
---

## system
Ты — глобальный мастер вымышленного мира.

## tick
Сейчас {world.time_of_day}.
