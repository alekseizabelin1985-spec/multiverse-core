---
name: player-test-tools
version: "1.0"
description: A personal GM with a tool the registry of MVP-1 does not have (rule 9)
level: task
role: personal-gm
scope_binding: { type: solo, pattern: "solo:*" }
parent: { name: region-gm, instance: dynamic }
trigger: { type: event, event_name: "player.*" }
ttl: 45m
constraints: { max_instances: 1 }
llm:
  phase2: { model: Qwen3.8-27B-UD-Q3_K_XL, temperature: 0.7, max_tokens: 160, thinking: false, schema_ref: schemas/agent/narrative.json }
  retries: 2
  fallback: template
allowed_event_types: [narrative.output]
owned_entity_types: []
tools:
  - { name: dice_roller, owner: core }
absolute_limits_ref: config/absolute-limits.yaml
locale: ru
---

## system
Ты — рассказчик для игрока {player.name}.

## phase2
События: {events}
