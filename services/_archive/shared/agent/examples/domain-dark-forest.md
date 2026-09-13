name: domain-dark-forest
version: "1.0"
description: Агенты региона "Тёмный лес" — дикие животные, погодные эффекты, скрытые опасности

trigger:
  type: event
  event_name: player.entered_region
  conditions:
    - field: player_count
      operator: "=="
      value: 1
    - field: time_of_day
      operator: "in"
      value: ["night", "twilight"]

constraints:
  max_instances: 1
  priority: 50
  shared_resources:
    - name: chroma:region:dark-forest
    - name: neo4j:region:dark-forest

ttl: "1h"

llm:
  model: qwen:7b
  temperature: 0.7
  max_tokens: 2048
  fallback: qwen:7b-turbo
  schema:
    type: object
    required:
      - decisions
      - narrative_phase
    properties:
      decisions:
        type: array
        items:
          type: object
          properties:
            type:
              type: string
            target:
              type: string
            payload:
              type: object
      narrative_phase:
        type: string
        enum: ["skip", "generate", "async"]

tools:
  - name: search_entities
    owner: entity-manager
  - name: spawn_npc
    owner: city-governor
  - name: modify_terrain
    owner: world-generator
  - name: trigger_weather
    owner: narrative-orchestrator
  - name: update_memory
    owner: semantic-memory

parent:
  name: global_supervisor
  instance: universe-genesis-oracle

# Prompt template для фазы 1 (механика)
phase1_prompt: |
  Ты — Game Master тёмного леса. Игрок {player_name} вошёл в регион.
  
  Окружение:
  - Время: {time_of_day}
  - Погода: {weather}
  - Сущности рядом: {nearby_entities}
  - История региона: {region_history}
  
  Твоя задача:
  1. Проанализировать текущую ситуацию
  2. Решить: что происходит дальше?
  3. Вернуть JSON с решениями
  
  Верни ТОЛЬКО JSON в формате схемы выше.

# Prompt template для фазы 2 (нарратив)
phase2_prompt: |
  Игрок {player_name} вошёл в тёмный лес. 
  Событие: {event_description}
  Результат фазы 1: {phase1_result}
  
  Опиши атмосферу, звуки, запахи, ощущения.
  Будь креативным, но в рамках лора региона.
