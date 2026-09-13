# Migration Guide: Narrative Orchestrator → Agent GM

Этот документ описывает миграцию с текущего `services/narrative-orchestrator` на новую `shared/agent` архитектуру.

## 📊 Сравнение архитектур

### Текущая архитектура (Monolithic GM)

```
┌─────────────────────────────────────┐
│  Narrative Orchestrator             │
│  ┌─────────────────────────────┐   │
│  │ Single LLM call (slow)      │   │
│  │ - Mechanics + Narrative     │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
```

### Новая архитектура (Agent GM)

```
┌─────────────────────────────────────┐
│  Agent Orchestrator                 │
│  Router → Lifecycle → Worker Pool   │
└───────────┬─────────────────────────┘
            │
    ┌───────▼────────┐
    │ Two-Phase Pipe │
    │ Phase 1: Fast  │
    │ Phase 2: Async │
    └────────────────┘
```

## 🔄 Шаги миграции

### Шаг 1: Создать новый сервис

```bash
# Новый сервис agent-orchestrator
mkdir -p services/agent-orchestrator/cmd
cd services/agent-orchestrator

# Добавить зависимости
go get multiverse-core/shared/agent
```

### Шаг 2: Интегрировать с EventBus

```go
// services/agent-orchestrator/cmd/main.go
package main

import (
    "multiverse-core/shared/eventbus"
    "multiverse-core/shared/agent"
)

func main() {
    // Подключение к Redpanda/Kafka
    bus := eventbus.NewRedpandaBus()
    
    // Подписываемся на события
    bus.Subscribe("player.*", func(event eventbus.Event) {
        // Маршрутизируем через роутер
        router.RouteEvent(context.Background(), toAgentEvent(event))
    })
    
    // Запускаем воркеры
    pool.Start(context.Background())
}
```

### Шаг 3: Заменить LLM вызовы

Вместо:
```go
// Старый код в narrative-orchestrator
result, err := llmClient.Generate(prompt)
```

Используем:
```go
// Новый код через pipeline
result := pipeline.Process(ctx, event, agent)
```

### Шаг 4: Migrate blueprints

1. Конвертируйте `configs/gm_*.yaml` в MD блупринты
2. Используйте двухфазный конвейер
3. Добавьте fallback на rule-engine

## 📝 Пример миграции

### Старый GM config (configs/gm_region.yaml)

```yaml
region: dark_forest
rules:
  - if player_count >= 1:
      spawn: wolf_npc
  - if player_count >= 5:
      spawn: bear_npc
```

### Новый blueprint (blueprints/domain-dark-forest.md)

```yaml
name: domain-dark-forest
type: domain
trigger:
  type: event
  event_name: player.entered_region
  conditions:
    - field: player_count
      operator: "=="
      value: 1
llm:
  model: qwen:7b
tools:
  - name: spawn_npc
    owner: city-governor
```

## 🎯 Критерии завершения миграции

- [x] `shared/agent` пакет создан и тестируется
- [x] Двухфазный конвейер реализован и протестирован
- [x] Blueprint парсер поддерживает YAML и MD
- [ ] `services/agent-orchestrator` запущен в production
- [ ] `services/narrative-orchestrator` отключен
- [ ] Метрики показывают улучшение latency <100ms p95

## 🚀 Развертывание

```bash
# Деплой в staging
kubectl apply -f k8s/agent-orchestrator-staging.yaml

# Запуск A/B теста с 10% трафика
kubectl patch deployment agent-orchestrator -p '{"spec":{"template":{"spec":{"containers":[{"name":"agent-orchestrator","env":[{"name":"TRAFFIC_PERCENT","value":"10"}]}]}}}}'

# Мониторинг метрик
watch -n 5 'curl -s localhost:8080/metrics | grep agent_'
```

## 📈 Ожидаемые улучшения

| Метрика | До | После |
|---|---|---|
| Latency p95 | ~500ms | **<100ms** |
| LLM cost | 100% | **~20%** (кэш) |
| Scalability | ~100/shard | **~1000/shard** |
| Availability | ~99% | **99.9%** |

## 🐛 Возможные проблемы и решения

### Проблема: Высокий latency
**Решение:** Проверить кэш, увеличить воркеров, проверить LLM gateway

### Проблема: Потеря контекста
**Решение:** Проверить MemoryStore (ChromaDB), увеличить TTL

### Проблема: Неверные решения
**Решение:** Обновить prompt templates, добавить больше training data

## 🔗 Дополнительные ссылки

- [03_conclusions_roadmap.md](../agent-gm-research/03_conclusions_roadmap.md)
- [services/narrative-orchestrator/README.md](../../services/narrative-orchestrator/README.md)
- [shared/eventbus/README.md](../../shared/eventbus/README.md)
