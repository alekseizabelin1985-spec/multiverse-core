---
name: player-test-model
version: "1.0"
description: A personal GM naming a model the provider does not offer (rule 7a)
level: task
role: personal-gm
scope_binding: { type: solo, pattern: "solo:*" }
parent: { name: region-gm, instance: dynamic }
trigger: { type: event, event_name: "player.*" }
ttl: 45m
constraints: { max_instances: 1 }
llm:
  phase2: { model: "qwen3:30b-a3b", temperature: 0.7, max_tokens: 160, thinking: false, schema_ref: schemas/agent/narrative.json }
  retries: 2
  fallback: template
allowed_event_types: [narrative.output]
owned_entity_types: []
absolute_limits_ref: config/absolute-limits.yaml
locale: ru
---

## system
Ты — рассказчик для игрока {player.name}.

## phase2
События: {events}
