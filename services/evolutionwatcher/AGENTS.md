# AGENTS.md for Evolution Watcher

> **Evolution Watcher** — событийно-управляемый сервис, отслеживающий аномалии в поведении сущностей и обеспечивающий целостность игровых миров через нейронные модели анализа.

---
## 📋 Service Overview

Evolution Watcher реализует систему наблюдения за поведением сущностей в реальном времени. Сервис использует нейронные модели для обнаружения неожиданного поведения, которое может указывать на аномалии в игровом мире.

### 🔑 Key Principles

| Принцип | Описание |
|---------|----------|
| **Наблюдение** | Постоянное отслеживание изменений поведения сущностей |
| **Аномалии** | Обнаружение неожиданного поведения через нейронные модели |
| **Интеграция** | Интеграция с Entity-Actor сервисом для получения данных о сущностях |
| **Событийность** | Работа через Kafka события для отслеживания изменений |

### 🔍 Event Processing Flow

```
Событие из Kafka
       │
       ▼
┌─────────────────┐
│ HandleEvent()     │
└────────┬────────┘
         │
    ┌────┴────┬─────────────────┐
    ▼         ▼                 ▼
entity_  state_        entity.created
snapshots changes     (новая сущность)
    │         │                 │
    ▼         ▼                 ▼
Обнаружить  Загрузить →   Создать →
аномалии    сущность →   применить →
            применить    сохранить
            изменения
```

---
## 📡 Event Integration

### Подписанные топики Kafka

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
| *(любое другое)* | `any` | Для других сервисов (игнорируется Evolution Watcher) |

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
make build-service SERVICE=evolution-watcher

# Запуск в Docker Compose
docker-compose up evolution-watcher

# Просмотр логов
docker-compose logs -f evolution-watcher

# Локальная сборка (Linux)
CGO_ENABLED=0 GOOS=linux go build -o bin/evolution-watcher ./cmd/evolution-watcher
```

---
## 📁 Directory Structure

```
multiverse-core/
├── cmd/evolution-watcher/
│   └── main.go              # Точка входа: инициализация + graceful shutdown
├── services/evolutionwatcher/
│   ├── AGENTS.md            # Эта документация
│   ├── watcher.go           # Anomaly detection
│   ├── anomaly.go           # Neural anomaly model
│   └── service.go             # Service orchestration
├── internal/entity/
│   └── entity.go            # Универсальная структура Entity
├── internal/eventbus/
│   ├── types.go             # Event struct, NewEvent()
│   ├── eventbus.go          # EventBus: Publish(), Subscribe()
│   └── topics.go          # Топики Kafka
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
Все логи Evolution Watcher включают `event_id` для трассировки:
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
| Пустой `payload` | Evolution Watcher игнорирует события без `entity_snapshots`/`state_changes` |
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
> **Evolution Watcher — это глаз и сердце наблюдения за эволюцией игровых миров.**