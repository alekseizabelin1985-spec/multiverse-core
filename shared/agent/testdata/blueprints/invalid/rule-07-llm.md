---
name: player-test-llm
version: "1.0"
description: A personal GM with a broken narrative phase and no fallback (rule 7)
level: task
role: personal-gm
scope_binding: { type: solo, pattern: "solo:*" }
parent: { name: region-gm, instance: dynamic }
trigger: { type: event, event_name: "player.*" }
ttl: 45m
constraints: { max_instances: 1 }
llm:
  phase2: { model: "qwen:7b", temperature: 2.5, max_tokens: 0, thinking: false, schema_ref: schemas/agent/narrative-v9.json }
  retries: 2
allowed_event_types: [narrative.output]
owned_entity_types: []
absolute_limits_ref: config/absolute-limits.yaml
locale: ru
---

## system
Ты — рассказчик для игрока {player.name}.

## phase2
События: {events}
