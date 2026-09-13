---
name: player-test-gm
version: "1.0"
description: Personal GM of a player
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
absolute_limits_ref: config/absolute-limits.yaml
locale: ru
---

## system
Ты — рассказчик для игрока {player.name}. Числа механики — истина.

## phase2
Состояние: {state}
События: {events}
Отсутствие: {absence}
Ответь JSON вида {"text": "...", "mentions": [], "background_refs": []}.
