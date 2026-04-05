# Multiverse-Core Development Guide

## Project Overview

**Multiverse-Core** — это распределённая система для управления сложными виртуальными мирами и нарративами. Платформа сочетает event-driven архитектуру, векторные базы данных, графовые базы данных и AI-оркестрацию для создания динамических, эволюционирующих виртуальных сред.

**Философия**: Миры не программируются, а рождаются — эволюционируют органично через действия игроков, сохраняя внутреннюю согласованность и нарративную глубину.

### Ключевые принципы

| Принцип | Описание |
|---------|----------|
| **Event-Driven Architecture (EDA)** | Все взаимодействия происходят через события в единой шине (Redpanda) |
| **Слабое связывание** | Сервисы знают только о событиях, а не друг о друге |
| **Stateful Services с восстановлением** | Каждый сервис хранит состояние и восстанавливается через snapshot + replay |
| **Генеративность над скриптами** | Qwen3 создаёт уникальные исходы вместо preset реакций |
| **Онтологическая осознанность** | Знания о мире влияют на логику через онтологические профили |

---

## Technology Stack

| Компонент | Технология | Назначение |
|-----------|------------|------------|
| **Язык** | Go 1.24.0 | Основной язык разработки |
| **Event Streaming** | Redpanda v24.2.5 | Шина событий (Kafka-совместимая) |
| **Object Storage** | MinIO | Хранение снапшотов, онтологий, артефактов |
| **Vector Database** | ChromaDB + Qdrant | Семантическая память, RAG, эмбеддинги |
| **Graph Database** | Neo4j 5.18 + APOC | Граф знаний, связи между сущностями |
| **Time-Series DB** | TimescaleDB (PostgreSQL 16) | Метрики производительности |
| **AI Model** | Qwen3 через Ollama | Генерация нарративов, оракулы |
| **Containerization** | Docker & Docker Compose | Развёртывание сервисов |

### Go Dependencies (ключевые)

```go
github.com/google/uuid v1.6.0              // UUID генерация
github.com/minio/minio-go/v7 v7.0.95       // MinIO клиент
github.com/segmentio/kafka-go v0.4.49      // Kafka/Redpanda клиент
github.com/xeipuuv/gojsonschema v1.2.0     // JSON Schema валидация
github.com/stretchr/testify v1.11.1        // Тестирование
github.com/yalue/onnxruntime_go v1.22.0    // ONNX Runtime (для ML)
gopkg.in/yaml.v3 v3.0.1                    // YAML конфигурация
```

---

## Architecture

### Инфраструктурные компоненты (Docker Compose)

```yaml
# Инфраструктура
redpanda:9092        # Event bus (PLAINTEXT)
redpanda-console:8092 # Web UI для Kafka
minio:9000/9001      # Object storage + Console
chromadb:8000        # Vector DB
neo4j:7474/7687      # Graph DB + Browser
timescaledb:5433     # Metrics (PostgreSQL порт)
qwen3-service:11434  # Ollama с моделью Qwen3
qdrant:6333/6334     # Alternative vector DB
```

### Топики Kafka/Redpanda

| Топик | Назначение | Примеры событий |
|-------|-----------|-----------------|
| `player_events` | События игроков | `player.enter.city`, `player.used_skill` |
| `world_events` | События миров | `world.generated`, `entity.travelled`, `violation.detected` |
| `game_events` | Игровые события | `quest.issued`, `game.state.changed` |
| `system_events` | Системные события | `entity.created`, `world.generated`, `system.startup` |
| `scope_management` | Управление скоупами | `scope.created`, `scope.destroyed` |
| `narrative_output` | Выход нарративов | `narrative.description`, `npc.action` |

### Core Services

| Сервис | Порт | Назначение | Stateful |
|--------|------|-----------|----------|
| **entity-manager** | - | Управление иерархическими сущностями с историей | ✅ MinIO + replay |
| **narrative-orchestrator** | - | Генерация живых нарративов на основе событий | ✅ Semantic state |
| **world-generator** | - | Генерация новых миров, регионов, онтологий | ❌ Stateless |
| **ban-of-world** | - | Guardian реальности, детекция аномалий | ✅ World health metrics |
| **city-governor** | - | Управление городской жизнью: экономика, NPC, квесты | ✅ Reputation, quests |
| **cultivation-module** | - | Система культивации: навыки, dao, ascension | ✅ Player profiles |
| **reality-monitor** | - | Агрегация метрик, публикация аномалий | ✅ Aggregated metrics |
| **plan-manager** | - | Переходы между планами, fusion zones | ✅ Plane graph (DAG) |
| **semantic-memory** | 8082 | Векторные эмбеддинги + граф знаний для RAG | ✅ ChromaDB + Neo4j |
| **ontological-archivist** | 8083 | Хранение и эволюция онтологических схем | ✅ Versioned schemas |
| **universe-genesis-oracle** | - | Генерация фундаментальной иерархии вселенной | ❌ Stateless/one-time |
| **game-service** | 8088 | HTTP API для взаимодействия игроков | ✅ Cache TTL |
| **entity-actor** | - | Нейронные агенты для автономных сущностей (Living Worlds) | ✅ Neural state |
| **evolution-watcher** | - | Детекция эволюции и аномалий в поведении | ✅ Anomaly models |
| **rule-engine** | - | Применение универсальных правил к типам сущностей | ✅ Rule sets |

**Примечания**:
- `ascension-oracle` отсутствует в main branch — функциональность покрыта через `universe-genesis-oracle` и интеграцию с Qwen3
- **Qdrant** — это инфраструктурный компонент (векторная БД), а не Go-сервис

### Порты сервисов

| Сервис | Порт по умолчанию | Порт в Docker Compose |
|--------|-------------------|------------------------|
| semantic-memory | 8080 | 8082 |
| ontological-archivist | 8081 | 8083 |
| game-service | 8080 | 8088 |
| qdrant (infra) | 6333/6334 | 6333/6334 |

**Важно**: Docker Compose переопределяет порты через `HTTP_ADDR` для избежания конфликтов.

---

## Building and Running

### Prerequisites

- Docker и Docker Compose
- Go 1.24.0
- Git
- (Опционально) MinIO CLI (`mc`) для отладки

### Quick Start

```bash
# 1. Клонировать репозиторий
git clone https://github.com/alekseizabelin1985-spec/multiverse-core.git
cd multiverse-core

# 2. Скопировать окружение
cp .env.example .env
# Отредактировать .env с вашими настройками

# 3. Запустить инфраструктуру
docker-compose up -d redpanda minio chromadb neo4j timescaledb qwen3-service

# 4. Собрать все сервисы
make build

# 5. Запустить конкретный сервис
make run SERVICE=entity-manager

# 6. Посмотреть логи
make logs-service SERVICE=entity-manager
```

### Make Commands

| Команда | Описание |
|---------|----------|
| `make build` | Собрать все сервисы (Docker) |
| `make build-service SERVICE=<name>` | Собрать конкретный сервис (Linux binary) |
| `make build-all` | Собрать все сервисы локально (Linux binaries) |
| `make up` | Запустить все сервисы через Docker Compose |
| `make run SERVICE=<name>` | Запустить конкретный сервис |
| `make down` | Остановить все сервисы |
| `make logs` | Показать логи всех сервисов |
| `make logs-service SERVICE=<name>` | Логи конкретного сервиса |
| `make clean` | Очистить артефакты сборки |
| `make test` | Запустить все тесты |
| `make test-service SERVICE=<name>` | Тесты конкретного сервиса |
| `make test-shared` | Тестировать только shared модули |
| `make sync` | Синхронизировать go workspace |
| `make help` | Показать справу по всем командам |

### Local Development

```bash
# Запустить инфраструктуру
docker-compose up -d redpanda minio chromadb neo4j

# Запустить сервис локально (на примере entity-manager)
cd services/entity-manager/cmd/entity-manager
KAFKA_BROKERS=localhost:9092 \
MINIO_ENDPOINT=localhost:9000 \
MINIO_ACCESS_KEY=minioadmin \
MINIO_SECRET_KEY=minioadmin \
go run main.go
```

### Docker Build (мульти-стадия)

```dockerfile
# Сборка сервиса
docker build --build-arg SERVICE=entity-manager -t multiverse-core:entity-manager .

# Сборка с CGO (для semantic-memory с ONNX)
docker build \
  --build-arg SERVICE=semantic-memory \
  --build-arg GO_BUILD_TAGS=chroma_v2_enabled \
  --build-arg BUILD_CGO=1 \
  -t multiverse-core:semantic-memory .
```

---

## Development Conventions

### Код

- **Язык**: Go 1.24.0
- **CGO**: `CGO_ENABLED=0` для всех сервисов, кроме `semantic-memory`
- **JSON Schema**: Draft 7 для валидации payload сущностей
- **UUID**: Использовать `github.com/google/uuid` для event_id и entity_id
- **Логирование**: Структурированное с контекстом (`event_id`, `world_id`, `entity_id`)
- **Context**: Использовать `context.Context` для отмены и таймаутов
- **Graceful Shutdown**: Обработка `SIGINT`/`SIGTERM` с очисткой ресурсов

### Event-Driven Architecture

```go
// Пример подписки на события
topics := []string{
    eventbus.TopicPlayerEvents,  // player.*
    eventbus.TopicWorldEvents,   // world.*, entity.*
    eventbus.TopicGameEvents,    // quest.*, game.*
    eventbus.TopicSystemEvents,  // system.*, entity.created
}

// Формат события
type Event struct {
    EventID   string                 `json:"event_id"`
    EventType string                 `json:"event_type"`
    WorldID   string                 `json:"world_id"`
    Timestamp time.Time              `json:"timestamp"`
    Payload   map[string]interface{} `json:"payload"`  // Динамический!
}
```

### 🔀 Event Access Patterns (MANDATORY)

```go
// ✅ ALWAYS use for reading event data:
pa := event.Path()  // *jsonpath.Accessor

// Extract with fallback chain — NEW format: entity.entity.id
entityID, _ := pa.GetString("entity.entity.id")
if entityID == "" {
    entityID, _ = pa.GetString("entity.id")  // fallback previous format
}
if entityID == "" {
    entityID, _ = pa.GetString("entity_id")  // fallback legacy
}

// World/Scope: use unified helpers
worldID := eventbus.GetWorldIDFromEvent(event)  // reads event.World.Entity.ID
scope := eventbus.GetScopeFromEvent(event)      // returns *ScopeRef{ID, Type}

// Type-safe getters for any depth:
level, _ := pa.GetInt("entity.stats.level")
active, _ := pa.GetBool("entity.active")
items, _ := pa.GetSlice("entity.inventory")

// Array access by index:
firstItem, _ := pa.GetString("entity.inventory[0].name")

// Quick existence check:
if pa.Has("quest.objectives") { /* ... */ }
```

### 📝 Creating Events (MANDATORY)

```go
// ✅ ALWAYS use builder for new events:
payload := eventbus.NewEventPayload().
    WithEntity(id, entityType, name).
    WithScope(scopeID, scopeType).  // solo/group/city/region/quest
    WithWorld(worldID)

// Add custom fields with dot-notation — use entity/event reference format:
eventbus.SetNested(payload.GetCustom(), "entity.entity.id", entityID)
eventbus.SetNested(payload.GetCustom(), "entity.entity.type", entityType)
eventbus.SetNested(payload.GetCustom(), "trigger.event.id", triggerEventID)
eventbus.SetNested(payload.GetCustom(), "trigger.event.type", "event")

event := eventbus.NewStructuredEvent(type, source, worldID, payload)
bus.Publish(ctx, topic, event)
```

### 🕸️ EntityRef Format (ALL References)

Все ссылки на сущности/события в payload используют единый формат:

```json
{
  "entity":  {"entity": {"id": "player-123", "type": "player"}},
  "target":  {"entity": {"id": "sword-456", "type": "item"}},
  "world":   {"entity": {"id": "world-789", "type": "world"}},
  "trigger": {"event":  {"id": "evt-abc", "type": "event"}}
}
```

Neo4j автоматически создаёт связи из payload ключей:
- `(ev)-[:ENTITY]->(player-123:Entity)`
- `(ev)-[:TARGET]->(sword-456:Entity)`
- `(ev)-[:WORLD]->(world-789:Entity)`
- `(ev)-[:TRIGGER]->(evt-abc:Event)`

Entity↔Entity связи создаются через `relations[]` массив.

### ⚠️ Deprecated Patterns (AVOID in new code)

```go
// ❌ DON'T use direct map access (panics on missing/wrong type):
entityID := event.Payload["entity_id"].(string)
worldID := event.WorldID

// ❌ DON'T create events with manual maps:
event := eventbus.Event{Payload: map[string]interface{}{...}}

// ❌ DON'T use flat reference fields:
payload["entity_id"] = id
payload["world_id"] = worldID

// ✅ USE the patterns above instead
```

### Entity Paths (dot notation)

```go
// Доступ к вложенным полям через путь
"path": "payload.health.current"  // → entity.Payload.Health.Current
"path": "stats.mp"                 // → entity.Payload.Stats.MP
"path": "inventory"                // → entity.Payload.Inventory (slice)
```

### MinIO Bucket Naming

```go
// Паттерны имён бакетов
`entities-{world_id}`      // Сущности конкретного мира
`entities-global`          // Глобальные сущности (до входа в мир)
`snapshots/em-{world_id}-v{N}.json`  // Снапшоты EntityManager
`schemas/{type}/{name}/v{version}.json`  // Онтологические схемы
`ontologies/{world_id}/v{N}.json`  // Онтологии миров
```

### Docker Compose Conventions

```yaml
# Шаблон сервиса
service-name:
  build:
    context: .
    dockerfile: ./Dockerfile
    args:
      - SERVICE=service-name
      - BUILD_CGO=0  # =1 только для semantic-memory
  command: ./service-name
  depends_on:
    - redpanda
    - minio
  env_file:
    - .env
  # ports: только для HTTP сервисов
  #   - "808X:808X"
```

---

## Testing

### Запуск тестов

```bash
# Все тесты
go test ./...

# С покрытием
go test -cover ./...

# Конкретный пакет
go test ./services/entitymanager

# Интеграционные тесты
go test -tags integration ./...
```

### Пример теста (EntityManager)

```go
func TestManager_SaveSnapshot(t *testing.T) {
    // Arrange
    m := &Manager{minio: mockMinioClient}
    ent := entity.NewEntity("test-123", "artifact", map[string]interface{}{
        "name": "Осколок",
        "current_world_id": "test-world",
    })

    // Act
    err := m.saveEntityToBucket(context.Background(), ent, "entities-test-world")

    // Assert
    assert.NoError(t, err)
    // Проверка вызова MinIO PutObject
}
```

### Интеграционное тестирование через Kafka

```bash
# Отправить тестовое событие
echo '{"event_type":"entity.created","world_id":"test-world","payload":{"entity_id":"test-1","entity_type":"item","payload":{"name":"Test"}}}' | \
  kafkacat -P -b localhost:9092 -t system_events

# Проверить в MinIO
mc alias set myminio http://minio:9000 minioadmin minioadmin
mc ls myminio/entities-test-world/

# Прочитать события
kafkacat -C -b localhost:9092 -t world_events -o beginning -c 10
```

---

## Configuration

### Environment Variables (.env)

```env
# Redpanda/Kafka
KAFKA_BROKERS=redpanda:9092
KAFKA_POLL_FREQUENCY_MS=100

# MinIO
MINIO_ENDPOINT=minio:9000        # БЕЗ http:// префикса!
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_USE_SSL=false

# ChromaDB
CHROMA_URL=http://chromadb:8000
CHROMA_API_KEY=  # Опционально

# Neo4j
NEO4J_URI=neo4j://neo4j:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=password

# TimescaleDB
TIMESCALE_HOST=timescaledb:5432
TIMESCALE_USER=metrics
TIMESCALE_PASSWORD=metrics
TIMESCALE_DB=metrics

# Qwen3 (Ollama)
ORACLE_URL=http://qwen3-service:11434/v1/chat/completions
QWEN_MODEL=qwen3

# Semantic Memory
SEMANTIC_MEMORY_URL=http://semantic-memory:8082

# Service-specific
PORT=8080
HTTP_ADDR=:8088
CACHE_TTL=5m
```

---

## Directory Structure

```
multiverse-core/
├── services/                     # Все сервисы (16 штук)
│   ├── entity-manager/
│   │   ├── cmd/
│   │   │   └── main.go          # Точка входа
│   │   ├── entitymanager/       # Бизнес-логика
│   │   │   ├── service.go
│   │   │   ├── manager.go
│   │   │   └── operations.go
│   │   ├── go.mod
│   │   └── AGENTS.md            # Service-specific guide
│   ├── narrative-orchestrator/
│   ├── semantic-memory/
│   ├── entity-actor/            # Living Worlds
│   ├── evolution-watcher/       # Living Worlds
│   ├── rule-engine/             # Living Worlds
│   └── ... (16 сервисов всего)
│
├── shared/                       # Общие пакеты (12 пакетов)
│   ├── config/                  # Конфигурация
│   ├── entity/                  # Entity структура
│   ├── eventbus/                # EventBus, Event types, Topics
│   ├── intent/                  # Oracle intent recognition
│   ├── jsonpath/                # Universal dot-path access
│   ├── minio/                   # MinIO клиенты
│   ├── oracle/                  # Oracle HTTP клиент
│   ├── redis/                   # Redis клиенты
│   ├── rules/                   # Rule engine core
│   ├── schema/                  # JSON Schema валидация
│   ├── spatial/                 # Пространственные утилиты
│   └── tinyml/                  # TinyML модели
│
├── Docs/                         # Документация
│   ├── architecture.md          # Общая архитектура
│   ├── EVENTS-MIGRATION.md      # Миграция на hierarchical events
│   ├── LIVING_WORLDS_*.md       # Living Worlds документация (8 файлов)
│   └── ... (15+ документов)
│
├── configs/                      # Конфигурационные YAML
│   ├── gm_defaults.yaml
│   ├── gm_group.yaml
│   ├── gm_location.yaml
│   ├── gm_player.yaml
│   ├── gm_region.yaml
│   └── gm_world.yaml
│
├── events/                       # Примеры событий (12 JSON)
├── memory/                       # AI agent memory files
│   ├── project_state.md
│   ├── architecture_decisions.md
│   └── ...
│
├── build/                        # Docker конфигурация
│   └── Dockerfile               # Мульти-стадия сборка
│
├── plans/                        # Планы развития
├── reports/                      # Отчёты и аналитика
├── fake_deps/                    # Fake зависимости для CGO
│
├── docker-compose.yml            # Оркестрация сервисов
├── Dockerfile                    # Шаблон сборки
├── Makefile                      # Build команды
├── go.mod / go.sum               # Go модули (Go 1.24.0)
├── go.work                       # Go workspace (16 сервисов + shared)
├── AGENTS.md                     # General agent guide
└── QWEN.md                       # Этот файл
```

---

## Key Design Patterns

### 1. Event Handler Pattern

```go
type Handler interface {
    HandleEvent(ctx context.Context, event Event) error
}

func (s *Service) processEvents(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-s.reader.Messages():
            event := parseEvent(msg)
            if err := s.handler.HandleEvent(ctx, event); err != nil {
                log.Error("Failed to handle event", err, "event_id", event.EventID)
            }
            s.reader.CommitMessages(ctx, msg)
        }
    }
}
```

### 2. Snapshot + Replay Recovery

```go
func (s *Service) Recover(ctx context.Context) error {
    // 1. Загрузить последний снапшот из MinIO
    snapshot, err := s.loadLatestSnapshot(ctx)
    if err != nil {
        return fmt.Errorf("failed to load snapshot: %w", err)
    }
    s.state = snapshot.State

    // 2. Replay событий с момента снапшота
    events, err := s.getEventsSince(ctx, snapshot.Timestamp)
    for _, event := range events {
        s.applyEvent(event)
    }

    return nil
}
```

### 3. Oracle Client with Retry

```go
type OracleClient struct {
    httpClient *http.Client
    baseURL    string
    maxRetries int
}

func (c *OracleClient) Call(ctx context.Context, prompt string) (*Response, error) {
    for i := 0; i < c.maxRetries; i++ {
        resp, err := c.doRequest(ctx, prompt)
        if err == nil {
            return resp, nil
        }
        if i == c.maxRetries-1 {
            return nil, err
        }
        time.Sleep(time.Duration(i+1) * time.Second)
    }
    return nil, fmt.Errorf("max retries exceeded")
}
```

### 4. MinIO Bucket Isolation

```go
func (m *Manager) getBucketForEvent(worldID string) string {
    if worldID == "" {
        return "entities-global"
    }
    return "entities-" + worldID  // Изоляция по миру
}

func (m *Manager) saveEntity(ctx context.Context, ent *Entity, worldID string) error {
    bucket := m.getBucketForEvent(worldID)
    key := ent.ID + ".json"
    return m.minio.PutObject(ctx, bucket, key, ent.Marshal(), nil)
}
```

---

## Living Worlds Architecture

**Статус**: ✅ **Implementation Complete** (merged to `main`)

### Компоненты

| Сервис | Назначение | Статус |
|--------|-----------|--------|
| **entity-actor** | Нейронные агенты (TinyML) для автономных сущностей | ✅ Реализован + Dockerfile |
| **evolution-watcher** | Детекция аномалий в поведении через нейросети | ✅ Реализован + Dockerfile |
| **rule-engine** | Универсальные правила для типов сущностей | ✅ Реализован + Dockerfile |

### Ключевые инновации

- ✅ **No Hardcoded Logic**: Поведение emerges из нейронных весов
- ✅ **Self-Evolution**: Сущности учатся через gameplay опыт
- ✅ **Oracle-First Intent Recognition**: NLU без training
- ✅ **Mechanics/Narrative Separation**: Чистое разделение правил и сторителлинга

### Performance Targets

| Метрика | Target |
|---------|--------|
| Inference Latency | <50ms |
| State Recovery | <200ms |
| Events/Second | 18 TPS/actor |
| Scaling | 10,000+ entities |
| Oracle Cost | $0.0018/1000 actions |

---

## Troubleshooting

### Частые проблемы

| Проблема | Решение |
|----------|---------|
| `SignatureDoesNotMatch` (MinIO) | Убедиться, что `MINIO_ENDPOINT` без `http://` префикса |
| Сервис не подключается к Kafka | Проверить `KAFKA_BROKERS=redpanda:9092` (не localhost!) |
| CGO ошибки при сборке | Использовать `BUILD_CGO=0` (кроме semantic-memory) |
| `NoSuchKey` при загрузке сущности | Нормально — сущность может не существовать; обработать как `nil` |
| Пустой payload в событии | EntityManager игнорирует события без `entity_snapshots`/`state_changes` |

### Debug Commands

```bash
# Проверить статус контейнеров
docker-compose ps

# Логи конкретного сервиса
docker-compose logs -f entity-manager

# Проверить MinIO бакеты
mc alias set myminio http://minio:9000 minioadmin minioadmin
mc ls myminio/

# Проверить топики Kafka
docker-compose exec redpanda rpk topic list

# Прочитать события из топика
docker-compose exec redpanda rpk topic consume world_events -n 10

# Проверить здоровье сервисов
curl http://localhost:8082/health  # Semantic Memory
curl http://localhost:8083/health  # Ontological Archivist
curl http://localhost:8088/health  # Game Service
```

---

## MCP Server Integration

Для работы с проектом через MCP (Model Context Protocol):

### Доступные инструменты (github-official)

- Управление issue и PR
- Поиск кода на GitHub
- Создание/обновление файлов в репозитории
- Code review через Copilot
- Управление branch и tags

### Настройка MCP сервера для проекта

Локальный MCP сервер может предоставлять инструменты для:
- Управления сервисами (start/stop/status)
- Публикации событий в Kafka
- Запросов к MinIO/ChromaDB/Neo4j
- Вызова Oracle (Qwen3)

---

## Resources

- **GitHub**: https://github.com/alekseizabelin1985-spec/multiverse-core
- **Документация**: `/Docs` директория
- **Living Worlds**: `Docs/LIVING_WORLDS_*.md`
- **Контакты**: alekseizabelin1985@gmail.com

---

> **"Мы не строим миры. Мы создаём условия для того, чтобы миры строили себя сами."**
> *— Философия Living Worlds, 2026*
