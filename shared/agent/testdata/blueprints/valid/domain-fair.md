---
name: domain-test-fair
version: "1.0"
description: Region GM of the test fair, with dates in untyped values
level: domain
role: region-gm
scope_binding: { type: region, id: test-fair-01 }
parent: { name: global-test-world }
trigger:
  type: event
  event_name: world.time_advanced
  conditions:
    - { field: world.date, operator: gte, value: 2001-06-01 }
constraints: { max_instances: 1 }
llm:
  phase1: { mode: rules }
  tick: { model: Qwen3.8-27B-UD-Q3_K_XL, lod_default: rule-only, max_tokens: 256, schema_ref: schemas/agent/tick-region.json }
  fallback: rules
allowed_event_types: [region.event_occurred]
owned_entity_types: [region]
laws_ref: laws/dark-forest-world@v1
rules_ref: rules/dark-forest.yaml
npc_table:
  - { npc_id: stray-wolf, kind: wolf, name: "Бродячий волк", stats_ref: wolf, count: 1, spawn: { on: tick } }
respawn_ttl: 24h
encounter: { detect_on: tick, perception: all, child_blueprint: encounter-test-wolf }
background_events:
  - { kind: fair_opens, weight: 1, summary_template: "Открывается ярмарка", ops: [{ path: fair.opens_at, value: 2001-06-01T09:30:00+03:00 }, { path: fair.closes_on, value: !!timestamp 2001-06-02 }] }
locale: ru
---

## system
Ты — мастер ярмарки {region.name}.

## description
Ярмарочный луг у тракта. Раз в год сюда съезжаются торговцы.
