---
name: group-test-narrator
version: "1.0"
description: Narrator of a group round
level: task
role: group-narrator
scope_binding: { type: group, pattern: "group:*" }
parent: { name: region-gm, instance: dynamic }
trigger: { type: event, event_name: group.created }
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
Ты описываешь раунд для всей группы; каждый участник назван по имени.

## phase2
Раунд {encounter.round}. События: {events}. Погода: {world.weather}.
