---
name: encounter-test-refs
version: "1.0"
description: An encounter with missing rules and without absolute limits (rule 10)
level: task
role: encounter
scope_binding: { type: "solo|group", pattern: "*" }
parent: { name: domain-test-forest }
trigger: { type: event, event_name: encounter.started }
ttl: 30m
constraints: { max_instances: 1 }
llm: { phase1: { mode: rules }, fallback: rules }
allowed_event_types: [combat.decided, encounter.ended]
owned_entity_types: [encounter, npc, player]
rules_ref: rules/swamp.yaml
round: { timeout: 60s, idle_after_missed: 2 }
locale: ru
---
