---
name: encounter-test-none
version: "1.0"
description: An encounter that allows no instance at all (rule 6, api-contracts.md §3.1)
level: task
role: encounter
scope_binding: { type: "solo|group", pattern: "*" }
parent: { name: domain-test-forest }
trigger: { type: event, event_name: encounter.started }
ttl: 30m
constraints: { max_instances: 0 }
llm: { phase1: { mode: rules }, fallback: rules }
allowed_event_types: [combat.decided, encounter.ended]
owned_entity_types: [encounter, npc, player]
rules_ref: rules/dark-forest.yaml
absolute_limits_ref: config/absolute-limits.yaml
round: { timeout: 60s, idle_after_missed: 2 }
locale: ru
---
