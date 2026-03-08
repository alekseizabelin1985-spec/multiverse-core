# AGENTS.md for Entity Actor

> **Entity Actor** — событийно-управляемый сервис, обеспечивающий поведенческую модель сущностей через TinyML модели и правила. Сервис управляет жизненным циклом акторов сущностей, обрабатывает их поведение и сохраняет историю состояний.

---

## 📋 Service Overview

Entity Actor реализует систему акторов сущностей, каждая из которых имеет собственное поведение, основанное на TinyML моделях и правилах. Сервис обеспечивает:

### 🔑 Key Principles

| Принцип | Описание |
|---------|----------|
| **Акторная архитектура** | Каждая сущность имеет собственный актор с уникальным ID |
| **TinyML модели** | Используются упрощенные ML модели для поведенческой генерации |
| **Правила** | Поведение регулируется через движок правил с применением к состоянию актора |
| **Состояние и история** | Акторы сохраняют историю своих состояний для анализа |

### 🔄 Event Processing Flow

```
Событие из Kafka
       │
       ▼
┌─────────────────┐
│ HandleEvent()   │
└────────┬────────┘
       │
   ┌────┴────┬─────────────────┐
   ▼         ▼                 ▼
entity_  state_        entity.created
snapshots changes     (новая сущность)
   │         │                 │
   ▼         ▼                 ▼
Создать   Загрузить →   Создать →
актор     сущность →   применить →
           применить    сохранить
           изменения
```

---

## 📡 Event Integration

### Подписанные топики Kafka/Redpanda

```go
topics := []string{
    eventbus.TopicPlayerEvents,    // player.*
    eventbus.TopicWorldEvents,     // world.*, violation.*, entity.*
    eventbus.TopicGameEvents,      // quest.*, game.*
    eventbus.TopicSystemEvents,    // system.*, entity.created, world.generated
}
```

### Формат события (payload — динамический!)

> ⚠️ **`payload` может содержать любые поля в любых комбинациях, или быть пустым.**

| Поле | Тип | Когда используется |
|------|-----|-----------------|
| `entity_snapshots` | `[]Entity` | Путешествия между мирами, полная синхронизация |
| `state_changes` | `[]StateChange` | Частичные обновления сущностей |
| `entity_id` + `entity_type` + `payload` | `string` + `string` + `map` | Событие `entity.created` |
| *(любое другое)* | `any` | Для других сервисов (игнорируется Entity Actor) |

#### Пример: Путешествие (`entity_snapshots`)
```json
{
  "event_type": "entity.travelled",
  "world_id": "memory-realm",
  "payload": {
    "entity_snapshots": [
      {
        "entity_id": "player-kain-777",
        "entity_type": "player",
        "payload": { "name": "Кайн", "current_world_id": "pain-realm" },
        "history": [ ... ]
      }
    ]
  }
}
```

#### Пример: Обновление (`state_changes`)
```json
{
  "event_type": "player.used_skill",
  "world_id": "pain-realm",
  "payload": {
    "state_changes": [
      {
        "entity_id": "player-kain-777",
        "operations": [
          { "op": "set", "path": "stats.mp", "value": 85 }
        ]
      }
    ]
  }
}
```

#### Пример: Пустой payload (тик мира)
```json
{
  "event_type": "world.tick",
  "world_id": "pain-realm",
  "payload": {}
}
```

### Поддерживаемые операции (state_changes)

| Операция | Описание | Пример |
|----------|----------|--------|
| `set` | Установить значение по пути | `{"op":"set","path":"stats.hp","value":100}` |
| `add_to_slice` | Добавить строку в срез | `{"op":"add_to_slice","path":"inventory","value":"sword-123"}` |
| `remove_from_slice` | Удалить строку из среза | `{"op":"remove_from_slice","path":"inventory","value":"potion-1"}` |
| `remove` | Удалить поле по пути | `{"op":"remove","path":"temporary_effect"}` |

---

## ⚙️ Configuration

### Переменные окружения

```env
# MinIO
MINIO_ENDPOINT=minio:9000        # ⚠️ БЕЗ http:// префикса!
MINIO_ACCESS_KEY=multiverse
MINIO_SECRET_KEY=securepassword123

# Kafka/Redpanda
KAFKA_BROKERS=redpanda:9092
```

### Config struct
```go
type Config struct {
    MinioEndpoint  string
    MinioAccessKey string
    MinioSecretKey string
    KafkaBrokers   []string
}
```

---

## 🛠️ Build/Run Commands

```bash
# Сборка сервиса
make build-service SERVICE=entity-actor

# Запуск в Docker Compose
docker-compose up entity-actor

# Просмотр логов
docker-compose logs -f entity-actor

# Локальная сборка (Linux)
CGO_ENABLED=0 GOOS=linux go build -o bin/entity-actor ./cmd/entity-actor
```

---

## 📁 Directory Structure

```
multiverse-core/
├── cmd/entity-actor/
│   └── main.go              # Точка входа: инициализация + graceful shutdown
├── services/entityactor/
│   ├── AGENTS.md            # Эта документация
│   ├── service.go           # Service: Start(), Stop(), Config
│   ├── manager.go           # Manager: HandleEvent(), Actor lifecycle management
│   ├── actor.go           # Actor: Behavior processing, State management
│   ├── model.go             # TinyML models, RuleEngine, Result types
│   └── api/                 # API handlers and types
│       ├── handlers.go      # HTTP handlers for actor operations
│       └── types.go           # API request/response types
├── internal/entity/
│   └── entity.go            # Универсальная структура Entity
├── internal/eventbus/
│   ├── types.go             # Event struct, NewEvent()
│   ├── eventbus.go          # EventBus: Publish(), Subscribe()
│   └── topics.go            # Топики Kafka
└── internal/storage/minio/  # (удалён — используем прямой клиент)
```

---

## 🔍 Debugging Tips

### Включить трассировку MinIO
```go
// Временно добавить в NewManager():
minioClient.TraceOn(os.Stdout)
```

### Проверить подключение к MinIO
```bash
mc alias set myminio http://minio:9000 multiverse securepassword123
mc ls myminio/entities-pain-realm/
```

### Проверить события в Kafka
```bash
kafkacat -C -b localhost:9092 -t world_events -o beginning -c 10
```

### Логирование с контекстом
Все логи Entity Actor включают `event_id` для трассировки:
```
[event=evt-123] Processing event of type player.used_skill in world pain-realm
[event=evt-123] Updated entity player-kain-777
```

---

## ⚠️ Common Pitfalls

| Проблема | Решение |
|----------|---------|
| `SignatureDoesNotMatch` | Убедиться, что `MINIO_ENDPOINT` без `http://` |
| `NoSuchKey` при загрузке | Это нормально — сущность может не существовать; обработать как `nil` |
| Пустой `payload` | Entity Actor игнорирует события без `entity_snapshots`/`state_changes` |
| Конфликт типов в `payload` | Использовать type assertions с проверкой `ok` |
| Неправильное определение бакета | Бакет определяется по `world_id`, а не по `payload.current_world_id` |

---

## 🔄 Backward Compatibility

- Legacy события с `entity.created` обрабатываются
- `state_changes` и `entity_snapshots` могут присутствовать одновременно — обрабатываются оба
- Неизвестные поля в `payload` игнорируются (graceful degradation)

---

## 📈 Performance Targets

| Метрика | Target |
|---------|--------|
| Время обработки события | < 50ms |
| Потребление памяти | Константное (не растёт с числом сущностей) |
| Масштабируемость | Тысячи миров через шардинг по `world_id` |

---

> **Entity Actor — это душа игровых сущностей.**  
> Без него — только хаос и забвение.