---
name: player-test-greedy
version: "1.0"
description: A personal GM publishing beyond its role and owning an entity type while it proposes no entity change (rule 8)
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
allowed_event_types: [narrative.output, world.weather_changed, player.teleported]
owned_entity_types: [world]
absolute_limits_ref: config/absolute-limits.yaml
locale: ru
---

## system
Ты — рассказчик для игрока {player.name}.

## phase2
События: {events}
