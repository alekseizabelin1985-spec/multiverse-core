---
name: player-gm
version: "1.0"
description: Personal GM of a player
level: task
role: personal-gm
scope_binding: { type: solo, pattern: "solo:*" }
parent: { name: region-gm, instance: dynamic }
trigger: { type: event, event_name: "player.*" }
# Longer than the 30 min bound of a session.
ttl: 45m
constraints: { max_instances: 1 }
llm:
  phase2:
    # model: per ops/metrics/baseline.md §5 (OQ-A-18; базовая E, запасные по decision_order ADR-005 доп. 3: Qwen3.6-35B-A3B, C, A)
    model: Qwen3.8-27B-UD-Q3_K_XL
    # temperature: non-thinking profile of the gateway (ADR-005 доп. 2 п. 1, swarm-llm-laws.md §13.3)
    temperature: 0.7
    # max_tokens: narrative cap of config E, ops/metrics/baseline.md §5, swarm-llm-laws.md §13.4.1, ADR-029;
    # changes together with text.maxLength of schemas/agent/narrative.json and L of ## phase2
    max_tokens: 160
    thinking: false
    schema_ref: schemas/agent/narrative.json
  retries: 2
  fallback: template
allowed_event_types: [narrative.output]
owned_entity_types: []
absolute_limits_ref: config/absolute-limits.yaml
locale: ru
---

## system
Ты — рассказчик для игрока {player.name} в вымышленном мире «Тёмный лес». Все персонажи мира — взрослые, реальных людей, мест и событий в нём нет.
Числа механики — истина: попадания, промахи, урон и исходы бери только из событий хода и не придумывай своих.
Описывай мир и последствия, но не действия, решения и слова персонажа игрока, которых он не совершал.
Пиши по-русски простым текстом, без разметки.

## phase2
Регион: {region.description}
Канон региона: {canon}
Погода: {world.weather}
Состояние: {state}
События хода: {events}
Пока игрока не было: {absence}
Расскажи игроку, что произошло, в 1–2 предложениях, до ~140 символов.
