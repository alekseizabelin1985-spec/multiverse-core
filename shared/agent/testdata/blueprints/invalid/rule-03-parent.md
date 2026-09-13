---
name: encounter-test-orphan
version: "1.0"
description: An encounter whose parent is not a blueprint of the project (rule 3)
level: task
role: encounter
scope_binding: { type: "solo|group", pattern: "*" }
parent: { name: domain-test-swamp }
trigger: { type: event, event_name: encounter.started }
ttl: 30m
constraints: { max_instances: 1 }
llm: { phase1: { mode: rules }, fallback: rules }
allowed_event_types: [combat.decided, encounter.ended]
owned_entity_types: [encounter, npc, player]
rules_ref: rules/dark-forest.yaml
absolute_limits_ref: config/absolute-limits.yaml
round: { timeout: 60s, idle_after_missed: 2 }
locale: ru
---
