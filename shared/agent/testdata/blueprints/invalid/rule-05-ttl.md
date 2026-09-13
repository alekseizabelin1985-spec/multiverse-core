---
name: global-test-ttl
version: "1.0"
description: A global GM with a ttl, which never applies to it (rule 5)
level: global
role: global-gm
scope_binding: { type: world, id: test-world }
trigger: { type: timer, intervals: { idle: 60m } }
ttl: 60m
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
  - { kind: time_advance, weight: 10, summary_template: "Наступает {time_of_day}" }
invariants:
  - { id: inv-01, check: dead_does_not_act }
locale: ru
---

## system
Ты — глобальный мастер вымышленного мира. Законы мира: {laws_version}.

## tick
Сейчас {world.time_of_day}.
