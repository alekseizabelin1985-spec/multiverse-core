---
name: encounter-test-timer
version: "1.0"
description: An encounter on a timer without intervals (rule 4)
level: task
role: encounter
scope_binding: { type: "solo|group", pattern: "*" }
parent: { name: domain-test-forest }
trigger: { type: timer }
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
