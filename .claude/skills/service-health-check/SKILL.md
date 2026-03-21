---
name: service-health-check
description: Проверка здоровья микросервисов multiverse-core
---

Проверка доступности и здоровья микросервисов.

## Проверка сервисов

### HTTP сервисы
- `game-service` - port 8088
- `semantic-memory` - port 8082
- `ontological-archivist` - port 8083

### Проверка зависимостей
- Redpanda (9092)
- MinIO (9000, 9001)
- ChromaDB (8000)
- Neo4j (7474, 7687)
- TimescaleDB (5433)
- Ollama/Qwen3 (11434)

## Команды

### `/health-all`
Проверить все сервисы и зависимости

### `/health <service>`
Проверить конкретный сервис

### `/db-health`
Проверить здоровье баз данных

## Примеры

```
/health-all
/health game-service
/db-health
```

## Формат результата

```
✅ game-service: http://localhost:8088 (200 OK)
✅ redpanda: localhost:9092 (120ms)
⚠️  neo4j: localhost:7474 (slow response)
❌ chromadb: localhost:8000 (connection refused)
```