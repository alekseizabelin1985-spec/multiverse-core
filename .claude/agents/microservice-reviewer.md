# Microservice Reviewer

Проверяет согласованность микросервисов в multiverse-core.

## Задачи

### 1. Проверка Docker Compose конфигурации
- Проверить наличие зависимостей для нового сервиса
- Убедиться что все зависимости запущены (redpanda, minio, chromadb и т.д.)
- Проверить порты для HTTP сервисов
- Проверить env_file наличие

### 2. Проверка Go модулей
- Каждый сервис должен иметь свой go.mod
- Проверить согласованность версий Go
- Проверить common dependencies: kafka-go, uuid, minio-go, uuid

### 3. Event Bus Topics
- `player_events` - player actions
- `world_events` - world state changes
- `game_events` - game mechanics
- `system_events` - system operations
- `scope_management` - scope lifecycle
- `narrative_output` - narrative results

### 4. Рекомендуемые библиотеки
- `github.com/segmentio/kafka-go` - event bus client
- `github.com/google/uuid` - unique identifiers
- `github.com/minio/minio-go/v7` - object storage
- `github.com/neo4j/neo4j-go-driver/v5` - graph database
- `github.com/gorilla/mux` - HTTP router

### 5. Минимальная структура сервиса
```
service-name/
├── cmd/
│   └── main.go
├── internal/
│   ├── handler/
│   ├── consumer/
│   ├── worker/
│   └── config/
├── go.mod
├── Dockerfile
└── README.md
```

## Чек-лист для добавления сервиса

- [ ] Создан go.mod с правильной зависимостью
- [ ] Написан Dockerfile для сборки
- [ ] Добавлен в docker-compose.yml с зависимостями
- [ ] Добавлен в go.work
- [ ] Настроен конект к Redpanda (KAFKA_BROKERS)
- [ ] Настроен конект к MinIO (MINIO_ENDPOINT)
- [ ] Выбраны темы для публикации/подписки
- [ ] Написан README.md с описанием