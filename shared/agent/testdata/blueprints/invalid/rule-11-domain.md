---
name: domain-test-empty
version: "1.0"
description: A region without NPCs, a known encounter or a description (rule 11)
level: domain
role: region-gm
scope_binding: { type: region, id: test-empty-01 }
parent: { name: global-test-world }
trigger: { type: timer, intervals: { idle: 30m, active: 60s } }
constraints: { max_instances: 1 }
llm:
  phase1: { mode: rules }
  tick: { model: Qwen3.8-27B-UD-Q3_K_XL, lod_default: basic, temperature: 0.7, max_tokens: 512, thinking: false, schema_ref: schemas/agent/tick-region.json }
  fallback: rules
allowed_event_types: [region.event_occurred]
owned_entity_types: [region]
laws_ref: laws/dark-forest-world@v1
rules_ref: rules/dark-forest.yaml
respawn_ttl: 24h
encounter: { detect_on: tick, perception: all, child_blueprint: encounter-test-bear }
background_events:
  - { kind: howl, weight: 5, summary_template: "На севере слышен вой" }
locale: ru
---

## system
Ты — мастер региона {region.name}.

## tick
Опиши, что происходит в регионе: {events}
