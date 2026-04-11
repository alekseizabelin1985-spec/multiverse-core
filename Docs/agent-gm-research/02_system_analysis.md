# Агентная архитектура GM: Системный анализ

**Версия:** 1.0  
**Дата:** 2026-01-22  
**Статус:** Утверждено для планирования

---

## 1. Архитектурный паттерн: OpenClaw-style

```
┌─────────────────────────────────────────┐
│           Agent Orchestrator            │
│  ┌─────────┬─────────┬─────────────┐   │
│  │ Router  │ Workers │ Lifecycle   │   │
│  └────┬────┴────┬────┴──────┬──────┘   │
│       │         │           │          │
│  ┌────▼────┐ ┌▼──────┐ ┌────▼─────┐   │
│  │EventBus │ │State  │ │ToolReg   │   │
│  │(Kafka)  │ │Manager│ │+Sandbox  │   │
│  └────┬────┘ └───┬───┘ └────┬─────┘   │
│       │          │          │          │
└───────┼──────────┼──────────┼──────────┘
        │          │          │
        ▼          ▼          ▼
┌────────────┬────────────┬────────────┐
│   Redis    │ PostgreSQL │  LLM Gateway│
│ (Context)  │ (Archive)  │(Routing+Cache)│
└────────────┴────────────┴────────────┘
```

### Ключевые принципы
| Принцип | Реализация | Обоснование |
|---------|------------|-------------|
| **Event-Driven Spawning** | Агенты создаются по событию + blueprint, не заранее | Экономия ресурсов, реакция на реальную активность |
| **Stateless Workers** | Воркеры не хранят состояние, контекст грузится из Redis | Горизонтальное масштабирование, отказоустойчивость |
| **Hierarchical Delegation** | Global → Domain → Task → Object (родитель → ребёнок) | Параллелизм, изоляция сбоев, управляемая сложность |
| **Tool-Centric Execution** | Все действия через `ToolRegistry` с валидацией | Безопасность, аудит, возможность sandboxing |
| **Dynamic LOD** | Уровень детализации меняется по метрикам | Баланс качества/производительности, адаптация к нагрузке |

---

## 2. Модель агента

### 2.1. Блупринт (MD-файл)

```markdown
---
id: domain-dark-forest
type: domain
version: 1
spawn: condition
conditions: [region.player_count >= 1, region.tags contains "dark_forest"]
tools: [spawn_task, query_lore, modify_weather]
lod_range: [1, 3]
ttl: 3600
memory: [vector, recent]
child_ttl: 300
---

# Заголовок (используется как системный промпт для LLM)

## Goal
Цель агента (1-2 предложения).

## Rules
- Список детерминированных правил (для rule-engine, без LLM)
- Формат: условие → действие

## Constraints
- Ограничения на инструменты (частота, параметры)
- Условия использования LLM

## Memory Policy
- Какие слои памяти использовать (Redis/Chroma/Neo4j)
- TTL контекста, стратегия очистки
```

**Преимущества формата:**
- Человеко-читаемый, редактируемый в любом редакторе
- Единый источник: конфиг + промпт + ограничения
- Git-friendly: диффы, code review, ветвление
- Быстрый парсинг: frontmatter (YAML) + H2-секции

### 2.2. Контекст агента (Runtime)

```go
type AgentContext struct {
    // Идентификация
    ID        string    // "dom:dark-forest-01"
    Type      string    // global|domain|task|monitor|object
    ParentID  string    // для иерархии
    Version   int64     // optimistic locking
    
    // Состояние
    State     string    // active|suspended|completed|failed
    LOD       int       // 0-3
    Goal      json.RawMessage
    
    // Ссылки на данные
    RegionID  string
    PlayerID  string    // опционально
    MemoryRef string    // ссылка на вектор/граф
    
    // Жизненный цикл
    CreatedAt time.Time
    TTL       time.Duration
    LastTick  time.Time
}
```

**Хранение:**
- **Hot (Redis)**: `agent:{id}:context` (HSET, TTL 15м), `agent:{id}:lock` (WATCH для версионирования)
- **Cold (PostgreSQL)**: `agent_contexts` (полная история, аудит, восстановление)
- **Sync**: Delta-save при каждом тике, full snapshot при завершении

### 2.3. Уровень детализации (LOD)

| LOD | Описание | Когда используется |
|-----|----------|-------------------|
| **0** | Спящий | Агент создан, но нет активных событий. Только метрики. |
| **1** | Правила | Детерминированная логика (`rule-engine`), кэш. Без LLM. |
| **2** | Гибридный | Простые запросы к LLM (кэшируемые), базовый ReAct. |
| **3** | Полный | Полный ReAct loop, креативные запросы, сложные решения. |

**Расчёт LOD (Router):**
```
lod = base_lod
if player_density > threshold_high: lod = min(lod+1, max)
if queue_depth > threshold_urgent: lod = max(lod-1, min)
if event_priority == "critical": lod = max(lod, 2)
return clamp(lod, blueprint.lod_range)
```

---

## 3. LLM-конвейер: Замена if-else

### 3.1. Двухфазная архитектура

```
[Event: player.attack]
        │
        ▼
┌─────────────────────────────────┐
│ Phase 1: Decision (Механика)    │
│ • Модель: fast/cheap (Qwen-7B)  │
│ • Промт: правила + статы + броски│
│ • Вывод: строго валидный JSON   │
│ • SLA: < 100ms (p95)            │
└──────────────┬──────────────────┘
               │ Emit: combat.decided
               ▼
┌─────────────────────────────────┐
│ Phase 2: Narrative (Описание)   │
│ • Модель: creative (Haiku/72B)  │
│ • Промт: фаза1 + лор + окружение│
│ • Вывод: {text, effects[], mood}│
│ • SLA: async, < 500ms (не блокирует)│
└──────────────┬──────────────────┘
               │ Emit: combat.rendered
               ▼
        [Game Service / EventBus]
```

### 3.2. Шаблон промпта (Phase 1: механика)

```json
{
  "system": "Ты — движок разрешения механик {rule_system}. Возвращай ТОЛЬКО валидный JSON. Никаких пояснений.",
  "context": {
    "attacker": {"str": 16, "prof": 2, "weapon": "longsword"},
    "defender": {"ac": 13, "hp": 11, "type": "beast"},
    "rolls": {"d20": 14, "d8": 5},
    "environment": "dark_forest"
  },
  "rules": [
    "hit = d20 + str_mod + prof >= ac",
    "damage = d8 + str_mod",
    "status = 'killed' if hp - damage <= 0 else 'wounded'"
  ],
  "output_schema": {
    "type": "object",
    "required": ["hit", "damage", "remaining_hp", "status", "next_phase"],
    "properties": {
      "hit": {"type": "boolean"},
      "damage": {"type": "integer"},
      "remaining_hp": {"type": "integer"},
      "status": {"enum": ["wounded", "killed", "missed"]},
      "next_phase": {"enum": ["narrative", "done"]}
    }
  }
}
```

### 3.3. Валидация и fallback

```go
func (g *Gateway) Resolve(ctx context.Context, tmpl *PromptTemplate, vars map[string]any) (*json.RawMessage, error) {
    // 1. Кэш по хэшу(vars)
    if cached, ok := g.cache.Get(ctx, hash(vars)); ok {
        return cached, nil
    }
    
    // 2. Вызов модели
    resp, err := g.client.Generate(ctx, tmpl, vars)
    if err != nil {
        return g.fallback(tmpl, vars) // детерминированный калькулятор
    }
    
    // 3. Валидация по схеме
    if err := g.validator.Validate(resp, tmpl.OutputSchema); err != nil {
        // Retry 1: ужесточение системного промпта
        tmpl.System += " СТРОГО ТОЛЬКО JSON. БЕЗ ТЕКСТА."
        resp, err = g.client.Generate(ctx, tmpl, vars)
        if err != nil || g.validator.Validate(resp, tmpl.OutputSchema) != nil {
            // Fallback: детерминированная логика
            return g.fallback(tmpl, vars)
        }
    }
    
    // 4. Кэширование и возврат
    g.cache.Set(ctx, hash(vars), resp)
    return resp, nil
}
```

**Преимущества:**
- Нет `if-else` в коде игры — вся логика в промптах и схемах
- Смена правил (D&D → GURPS → своя) = замена шаблона промпта, не кода
- Гарантированный формат ответа → безопасная интеграция с геймплейным кодом
- Fallback на детерминистику при сбое LLM → отказоустойчивость

---

## 4. Компоненты и интерфейсы

### 4.1. Agent Orchestrator (`services/agent-orchestrator`)

| Модуль | Ответственность | Зависимости |
|--------|-----------------|-------------|
| `engine.go` | Event loop, dispatch, backpressure | EventBus, Router, Workers |
| `router.go` | Пространственный + типовой роутинг, LOD-фильтр | BlueprintLoader, Metrics |
| `lifecycle.go` | Spawn/suspend/resume/archive агентов | StateManager, EventBus |
| `worker/pool.go` | Пул goroutine, конкурентность по регионам | Worker, ContextManager |
| `metrics.go` | Prometheus: queue_depth, agent_active, tool_calls | prometheus/client_golang |

### 4.2. Shared Libraries (`shared/agent`)

| Модуль | Ответственность | Интерфейсы |
|--------|-----------------|------------|
| `blueprint/` | Парсинг MD, кэш, hot-reload | `Loader.Load(id) (*Blueprint)` |
| `context/` | StateManager (Redis+PG, versioning, TTL) | `Manager.Load/Create/Save/Delete(ctx, id)` |
| `tools/` | Registry инструментов, sandbox, рейт-лимит | `Registry.Get(name).Execute(params, agentCtx)` |
| `llm/` | Gateway: routing, cache, validation, fallback | `Gateway.Resolve(ctx, tmpl, vars) (*json.RawMessage)` |
| `events/` | Типы событий, маршалинг, routing keys | `EventBus.Publish(target, event)` |

### 4.3. Интеграция с существующими сервисами

| Существующий сервис | Изменение | Точка интеграции |
|---------------------|-----------|------------------|
| `shared/eventbus` | Добавить `AgentEvent` wrapper с `routing_key`, `priority` | `eventbus.Publish(target, event)` |
| `shared/redis` | Добавить `AgentStateManager` (TTL, HSET, WATCH) | `redis.NewAgentManager(cfg)` |
| `rule-engine` | Использовать как первый фильтр в `evaluator.Run()` до LLM | `rules.Match(event, context)` |
| `shared/oracle` | Обернуть в `LLMGateway` с batching, cache, fallback | `llm.NewGateway(oracle, cfg)` |
| `semantic-memory` | Использовать для загрузки `context.MemoryRef` (векторный поиск) | `memory.Query(ctx, ref, query)` |
| `configs/gm_*.yaml` | Мигрировать в `configs/agents/*.md` (блупринты) | `blueprint.Loader.LoadAll(dir)` |

---

## 5. Поток данных (Data Flow)

```
1. [Game Service] → EventBus: {type: "player.attack", payload: {...}}
2. [Router] → Match blueprints: [task-wolf-encounter]
3. [Lifecycle] → Load/Create context: Redis HGET/SET, version check
4. [Worker Pool] → Submit(agent_ctx, event) to region-scoped queue
5. [Worker] → 
   a. Load context from Redis
   b. evaluator.Run():
      - rule-engine.Match() → если найдено → пропустить LLM
      - иначе → llm.Gateway.Resolve(Phase1) → валидация → JSON
   c. Если next_phase == "narrative" → llm.Gateway.Resolve(Phase2) async
   d. Save delta to Redis (HSET + INCR version)
   e. Emit actions to EventBus: {type: "combat.decided", ...}
6. [Game Service] ← EventBus: render text/effects to player
7. [Metrics] → Update counters: agent_ticks_total, llm_latency_hist, cache_hits
```

---

## 6. Масштабируемость и отказоустойчивость

### 6.1. Горизонтальное масштабирование
- **Шардирование по региону**: `region_id % shard_count` → конкретный инстанс оркестратора
- **Пул воркеров**: Настройка `max_concurrency_per_region` в конфиге
- **Авто-скейлинг**: Метрики `queue_depth` → HPA в Kubernetes

### 6.2. Отказоустойчивость
| Сценарий | Механизм | Восстановление |
|----------|----------|----------------|
| Падение воркера | Heartbeat + timeout | Переназначение тика другому воркеру |
| Потеря контекста (Redis) | PG backup + versioning | Загрузка последней сохранённой версии |
| Сбой LLM | Retry + fallback на rule-engine | Детерминированный расчёт, логирование инцидента |
| Взрывной спавн | `max_children_per_parent`, `spawn_rate_limit` | Отклонение лишних запросов, алерт |

### 6.3. Мониторинг (Prometheus)
```yaml
# Пример метрик
agent_active_total{type="domain", region="dark-forest-01"} 3
agent_lod_distribution{lod="2"} 12
llm_calls_total{phase="decision", model="qwen-7b"} 15420
llm_latency_seconds{phase="decision", quantile="0.95"} 0.089
cache_hit_ratio{layer="llm_exact"} 0.73
queue_depth{region="dark-forest-01"} 5
tool_calls_total{name="spawn_task", status="success"} 892
```

---

## 7. Безопасность и валидация

### 7.1. Sandbox для инструментов
```go
type ToolSandbox struct {
    limits map[string]RateLimit  // calls_per_minute
    perms  map[string][]string   // agent_type → allowed tools
}

func (s *ToolSandbox) CanExecute(agentType, toolName string) bool {
    allowed, ok := s.perms[agentType]
    return ok && slices.Contains(allowed, toolName)
}

func (s *ToolSandbox) CheckRateLimit(toolName string) error {
    // Redis-based sliding window
}
```

### 7.2. Валидация входных/выходных данных
- **Вход**: Все события валидируются по JSON Schema перед обработкой
- **Выход**: Ответы LLM валидируются по `output_schema` из блупринта
- **Аудит**: Все вызовы инструментов и LLM логируются (INFO/DEBUG) с корреляционным ID

### 7.3. Защита от инъекций в промпты
- Контекст экранируется перед инжекцией в промпт
- Системный промпт отделён от пользовательского контекста
- Ограничение длины контекста (токены) в конфиге

---

## 8. Конфигурация и деплой

### 8.1. Структура конфигов
```
configs/
├── agents/                    # Блупринты агентов (/*.md)
│   ├── global_supervisor.md
│   ├── domain_dark_forest.md
│   └── task_wolf_encounter.md
├── llm_router.yaml           # Маршрутизация моделей, кэш, retry
├── lod_thresholds.yaml       # Плотность игроков → LOD
├── tools.yaml                # Registry: лимиты, права, схемы
└── orchestrator.yaml         # Пул воркеров, таймауты, шардирование
```

### 8.2. Пример `llm_router.yaml`
```yaml
models:
  fast:
    endpoint: "http://ollama:11434/api/generate"
    model: "qwen:7b"
    timeout_ms: 2000
    max_tokens: 256
  creative:
    endpoint: "http://ollama:11434/api/generate"
    model: "qwen:72b"
    timeout_ms: 10000
    max_tokens: 512

cache:
  exact_ttl: 3600          # точные совпадения
  semantic_enabled: true   # семантический кэш (опционально)
  redis_addr: "redis:6379"

retry:
  max_attempts: 2
  backoff_ms: [100, 500]
  fallback_enabled: true   # детерминированный fallback

routing:
  decision_phase: "fast"
  narrative_phase: "creative"
  fallback_on_error: true
```

### 8.3. Деплой (Kubernetes)
```yaml
# services/agent-orchestrator/deployment.yaml (фрагмент)
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    spec:
      containers:
      - name: orchestrator
        image: multiverse/agent-orchestrator:v1.0
        env:
        - name: REDIS_ADDR
          valueFrom: {configMapKeyRef: {name: app-config, key: redis_addr}}
        - name: LLM_ROUTER_CONFIG
          value: /configs/llm_router.yaml
        resources:
          requests: {cpu: "500m", memory: "512Mi"}
          limits: {cpu: "2000m", memory: "2Gi"}
        livenessProbe:
          httpGet: {path: /health, port: 8080}
          initialDelaySeconds: 30
```

---

## 9. Нефункциональные требования (NFR)

| Требование | Значение | Метрика |
|------------|----------|---------|
| **Latency (Phase 1)** | < 100ms (p95) | `llm_latency_seconds{phase="decision", quantile="0.95"}` |
| **Latency (Phase 2)** | < 500ms (async) | `llm_latency_seconds{phase="narrative"}` |
| **Throughput** | 1000 events/sec/shard | `eventbus_events_processed_total` |
| **Availability** | 99.9% | Uptime monitoring |
| **Scalability** | Linear to 10 shards | Load testing results |
| **Cost** | < $0.01 per player-hour | `llm_tokens_total * cost_per_token` |
| **Recovery Time** | < 30 sec after failure | Mean Time To Recovery (MTTR) |

---

## 10. Ограничения и допущения

1. **LLM-зависимость**: Система предполагает доступ к локальной/облачной LLM. При полной недоступности — деградация до `rule-engine` (LOD 1).
2. **Сетевая задержка**: Все вызовы LLM должны быть внутри одного дата-центра (или использовать edge-кэширование).
3. **Детерминизм**: Фаза 1 (механика) должна быть воспроизводимой при одинаковых входах (фиксированный seed для бросков).
4. **Консистентность**: Контекст агента — eventual consistency между Redis и PG. Критические изменения пишутся в PG синхронно.
5. **Безопасность**: Инструменты с побочными эффектами (`modify_world`, `spawn_npc`) требуют явного разрешения в блупринте и аудита.
