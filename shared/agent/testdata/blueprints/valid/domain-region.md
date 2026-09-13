---
name: domain-test-forest
version: "1.1"
description: Region GM of the test forest
level: domain
role: region-gm
scope_binding: { type: region, id: test-forest-01 }
parent: { name: global-test-world }
trigger: { type: timer, intervals: { idle: 30m, active: 60s } }
constraints: { max_instances: 1, priority: 50 }
llm:
  phase1: { mode: rules }
  tick: { model: Qwen3.8-27B-UD-Q3_K_XL, lod_default: basic, temperature: 0.7, max_tokens: 512, thinking: false, schema_ref: schemas/agent/tick-region.json }
  fallback: rules
allowed_event_types: [region.event_occurred, npc.moved, npc.spawned, encounter.started]
owned_entity_types: [region, npc, encounter]
laws_ref: laws/dark-forest-world@v1
rules_ref: rules/dark-forest.yaml
npc_table:
  - { npc_id: wolf-alpha, kind: wolf, name: "Альфа-волк", stats_ref: wolf, count: 1, spawn: { on: tick, chance: "0.25" } }
respawn_ttl: 24h
encounter: { detect_on: tick, perception: all, chance: "0.5", child_blueprint: encounter-test-wolf }
background_events:
  - { kind: howl, weight: 5, summary_template: "На севере слышен вой" }
locale: ru
immediate_broadcast: [region.event_occurred]
---

## description
Старый лес на краю мира. Тропы зарастают быстрее, чем их протаптывают.

## canon
- В лесу живёт стая волков.
- Альфа-волк не уходит от логова
  дальше старой мельницы.

- Ночью туман скрывает тропы.

## system
Ты — мастер региона {region.name}.

```text
## not a section: an example inside a fence
```

## tick
Опиши, что происходит в регионе: {events}
