---
name: domain-dark-forest
version: "1.1"
description: Region GM of dark-forest-01
level: domain
role: region-gm
scope_binding: { type: region, id: dark-forest-01 }
parent: { name: global-dark-forest-world }
trigger: { type: timer, intervals: { idle: 30m, active: 60s } }
constraints: { max_instances: 1 }
llm:
  phase1: { mode: rules }
  tick:
    # model: per ops/metrics/baseline.md §5 (OQ-A-18; базовая E, запасные по decision_order ADR-005 доп. 3: Qwen3.6-35B-A3B, C, A)
    model: Qwen3.8-27B-UD-Q3_K_XL
    lod_default: basic
    # temperature: non-thinking profile of the gateway (ADR-005 доп. 2 п. 1, swarm-llm-laws.md §13.3)
    temperature: 0.7
    max_tokens: 512
    thinking: false
    schema_ref: schemas/agent/tick-region.json
  fallback: rules
# Every publication of the role goes through Emitter against this list
# (swarm-llm-laws.md §5.4): the proposal that creates an encounter (ADR-028 p. 1,
# C-05 p. 4) and the proposals of a tick (§15.2) as well as the events.
allowed_event_types: [region.event_occurred, npc.moved, npc.spawned, encounter.started, entity.create.proposed, entity.update.proposed]
owned_entity_types: [region, npc, encounter]
laws_ref: laws/dark-forest-world@v1
rules_ref: rules/dark-forest.yaml
# Respawn only, after respawn_ttl: wolf-alpha itself is created from the
# fixtures (mvctl world init --fixtures, G2).
npc_table:
  - { npc_id: wolf-alpha, kind: wolf, name: "Альфа-волк", stats_ref: wolf, count: 1, spawn: { on: tick } }
respawn_ttl: 24h
# chance is the encounter_chance of the region fixture (testdata/fixtures/region.json).
encounter: { detect_on: tick, perception: all, chance: "0.25", child_blueprint: encounter-wolf }
background_events:
  - { kind: howl, weight: 5, summary_template: "На севере слышен вой" }
  - { kind: npc_wander, weight: 3, summary_template: "{npc.name} переходит в {to}" }
locale: ru
---

## description
Старый ельник на северной окраине мира: тропы теряются в подлеске, между стволами стоит туман, и по ночам за деревьями слышен вой.

## canon
- В Тёмном лесу живёт волчья стая; её ведёт Альфа-волк.
- В лесу нет магии, лечащей раны: силы возвращает только отдых.
- Волки не разговаривают и не торгуют.

## system
Ты — мастер региона {region.name} в вымышленном мире «Тёмный лес». Все персонажи мира — взрослые.
Описание региона: {region.description}
Канон региона — истина: {canon}
Законы мира версии {laws_version} ты не меняешь и не нарушаешь.
Ты ведёшь только фон региона — происшествия и передвижения зверей; действия и слова игроков не описываешь.
Пиши по-русски, коротко и без разметки.

## tick
Сейчас {world.time_of_day}, погода: {world.weather}.
Состояние региона: {state}
Недавние события региона: {events}
Предложи не больше трёх фоновых событий региона: происшествие, передвижение или появление зверя.
Ссылайся только на сущности из состояния региона. Каждое событие опиши одной фразой. Если в регионе ничего не меняется, верни пустой список событий.
