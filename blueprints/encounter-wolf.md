---
name: encounter-wolf
version: "1.0"
description: Encounter with the wolf of dark-forest-01
level: task
role: encounter
scope_binding: { type: "solo|group", pattern: "*" }
parent: { name: domain-dark-forest }
trigger: { type: event, event_name: encounter.started }
ttl: 30m
constraints: { max_instances: 1 }
llm: { phase1: { mode: rules }, fallback: rules }
# Every publication of the role goes through Emitter against this list
# (swarm-llm-laws.md §5.4): the rolls and the exchange package are its own
# (§15.1, C-05 p. 1–2), not only the decisions.
allowed_event_types: [dice.rolled, combat.decided, encounter.ended, entity.update.proposed]
# Only the participants of the encounter, and only through the mechanics.
owned_entity_types: [encounter, npc, player]
rules_ref: rules/dark-forest.yaml
absolute_limits_ref: config/absolute-limits.yaml
# The source of encounter.started.round (C-05 v1.1); the numbers repeat the
# round of rules/dark-forest.yaml.
round: { timeout: 60s, idle_after_missed: 2 }
locale: ru
---
