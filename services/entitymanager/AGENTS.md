# AGENTS.md for EntityManager

> **EntityManager** — ленивый, распределённый, событийно-управляемый сервис, обеспечивающий актуальность, историчность и восстанавливаемость всех сущностей в "Живом Мультиверсуме" через атомарные операции чтения/записи в MinIO.

---

## 📋 Service Overview

EntityManager управляет сущностями через прямые операции с MinIO, **без кэша в памяти**. Каждая операция — это загрузка сущности, применение изменений и сохранение обратно.

### 🔑 Key Principles

| Принцип | Описание |
|---------|----------|
| **Ленивость** | Данные загружаются из MinIO только при необходимости обработки события |
| **Изоляция по миру** | Бакеты `entities-{world_id}` обеспечивают изоляцию данных между мирами |
| **Полнота при путешествиях** | `entity_snapshots` передают полный граф сущностей при переходе между мирами |
| **Атомарность** | Каждая сущность обновляется независимо |
| **Восстанавливаемость** | MinIO — единый источник истины; состояние восстанавливается через replay событий |

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
Сохранить  Загрузить →   Создать →
полный    сущность →   применить →
слепок    применить    сохранить
в бакет   изменения
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
| *(любое другое)* | `any` | Для других сервисов (игнорируется EntityManager) |

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

## 💾 Хранение в MinIO

### Структура бакетов

```
entities-{world_id}/     ← сущности конкретного мира
├── player-kain-777.json
├── npc-elder-123.json
└── artifact-sword-456.json

entities-global/         ← глобальные сущности (до первого входа в мир)
├── player-kain-777.json
└── ...
```

### Ключ объекта
```
{entity_id}.json
```

### Определение бакета
```go
func (m *Manager) getBucketForEvent(worldID string) string {
    if worldID == "" {
        return "entities-global"
    }
    return "entities-" + worldID  // ← ВСЕГДА из события!
}
```

> 🔑 **Важно**: Бакет определяется **по `world_id` из события**, а не из `payload` сущности. Это критично для путешествий: событие приходит в **целевой мир**, и сущности должны сохраниться именно там.

---

## 🌍 Обработка путешествий

### Поток данных при `entity.travelled`

1. **PlanManager** публикует событие с `world_id = "новый-мир"`
2. **EntityManager в новом мире** получает событие
3. **`entity_snapshots`** сохраняются в бакет `entities-новый-мир`
4. Сущности в `payload` могут ещё иметь `current_world_id = "старый-мир"` — это нормально
5. Последующие события могут обновить `current_world_id` через `state_changes`

### Почему так?

- Событие — **факт о мире**, а не о сущности
- Сущность "осознаёт" переход позже, через повествование или обновление
- Это позволяет гибко управлять состоянием после путешествия

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
make build-service SERVICE=entity-manager

# Запуск в Docker Compose
docker-compose up entity-manager

# Просмотр логов
docker-compose logs -f entity-manager

# Локальная сборка (Linux)
CGO_ENABLED=0 GOOS=linux go build -o bin/entity-manager ./cmd/entity-manager
```

---

## 🧪 Testing

### Пример теста: сохранение snapshot
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
    // Verify MinIO PutObject was called with correct params
}
```

### Интеграционный тест через Kafka
```bash
# Отправить тестовое событие
echo '{"event_type":"entity.created","world_id":"test-world","payload":{"entity_id":"test-1","entity_type":"item","payload":{"name":"Test"}}}' | \
  kafkacat -P -b localhost:9092 -t system_events

# Проверить в MinIO
mc ls myminio/entities-test-world/
```

---

## 📁 Directory Structure

```
multiverse-core/
├── cmd/entity-manager/
│   └── main.go              # Точка входа: инициализация + graceful shutdown
├── services/entitymanager/
│   ├── service.go           # Service: Start(), Stop(), Config
│   ├── manager.go           # Manager: HandleEvent(), MinIO operations
│   ├── operations.go        # OperationType constants
│   └── AGENTS.md            # Эта документация
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
Все логи EntityManager включают `event_id` для трассировки:
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
| Пустой `payload` | EntityManager игнорирует события без `entity_snapshots`/`state_changes` |
| Конфликт типов в `payload` | Использовать type assertions с проверкой `ok` |
| Путешествия: сущность в "не том" бакете | Бакет определяется по `ev.WorldID`, а не по `payload.current_world_id` |

---

## 🔄 Backward Compatibility

- Legacy события с `entity.created` обрабатываются
- `state_changes` и `entity_snapshots` могут присутствовать одновременно — обрабатываются оба
- Неизвестные поля в `payload` игнорируются (graceful degradation)

---

## 📈 Performance Targets

| Метрика | Target |
|---------|--------|
| Время обработки события | < 50ms (1 чтение + 1 запись в MinIO) |
| Потребление памяти | Константное (не растёт с числом сущностей) |
| Масштабируемость | Тысячи миров через шардинг по `world_id` |

---

> **EntityManager — это память мультиверсума.**  
> Без него — только хаос и забвение.