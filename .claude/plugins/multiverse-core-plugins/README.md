# Multiverse Core Plugins

Комплексный плагин для автоматизации multiverse-core - распределённой event-driven системы.

## Навыки (Skills)

### make-command
Выполнение Makefile команд для сборки, запуска и тестирования сервисов.
- `/make-command` - выбрать команду из меню

### docker-compose-control
Управление docker-compose сервисами multiverse-core.
- `/dc-status` - статус всех контейнеров
- `/dc-start`, `/dc-stop` - управление
- `/dc-logs <service>` - логи сервиса

### redpanda-topic-check
Проверка наличия и конфигурации тем Redpanda.
- `/check-topics` - проверка обязательных тем
- `/create-topic <topic>` - создание темы

### service-health-check
Проверка здоровья микросервисов.
- `/health-all` - все сервисы
- `/health <service>` - конкретный сервис

### go-test-automation (NEW)
Автоматизация тестирования Go сервисов.
- `/go-test` - все тесты
- `/go-test-service SERVICE=<name>` - тесты сервиса
- `/go-coverage` - анализ покрытия
- `/go-race-test` - проверка race conditions

### redpanda-schema-validator (NEW)
Валидация схем событий (автоматически при коммитах).

## Subagents

### microservice-reviewer
Проверка согласованности микросервисов:
- Docker Compose конфигурации
- Go модулей и зависимостей
- Event bus topics
- Рекомендуемых библиотек

### go-workspace-manager (NEW)
Управление Go workspace:
- Проверка версий go.mod
- Синхронизация go.work
- Дубликаты зависимостей

### event-schema-validator (NEW)
Валидация схем событий:
- Структура событий
- Совместимость schema
- publish/subscribe consistency

## MCP Servers

### context7
Доступ к документации библиотек:
- Neo4j Go Driver (Cypher queries)
- MinIO Go SDK (object storage)
- Kafka/Redpanda client
- ChromaDB client

### database
Прямой доступ к базам данных:
- Neo4j (граф знаний)
- TimescaleDB (метрики)
- MinIO (объектное хранилище)
- ChromaDB (векторный поиск)

### github (NEW - external)
Интеграция с GitHub:
- PR описания
- Issue tracking
- Workflow status

### qdrant (NEW - external)
Векторная база данных:
- Query collections
- Vector similarity search
- Schema inspection

## Установка

### 1. MCP Серверы

```bash
# Context7 (если еще не установлен)
claude mcp add context7

# Database
claude mcp add database --config .mcp.json

# GitHub (нужен GITHUB_PERSONAL_ACCESS_TOKEN)
docker run -e GITHUB_TOKEN=your_token -i ghcr.io/github/github-mcp-server

# Qdrant
docker run -p 6333:6333 -v qdrant_storage:/qdrant/storage qdrant/qdrant
```

### 2. Настройка переменных окружения

Создайте `.mcp.env`:

```bash
GITHUB_TOKEN=your_github_token
DB_MCP_TOKEN=your_token
CONTEXT7_API_KEY=your_api_key
```

### 3. Подключение баз данных

В `.mcp.json` укажите подключения:

```json
{
  "database": {
    "neo4j": {
      "uri": "neo4j://localhost:7687",
      "username": "neo4j",
      "password": "password"
    },
    "timescaledb": {
      "connectionString": "postgres://metrics:metrics@localhost:5433/metrics"
    },
    "minio": {
      "endpoint": "localhost:9000",
      "accessKey": "minioadmin",
      "secretKey": "minioadmin"
    }
  }
}
```

## Использование

### Проверка сервиса перед коммитом
```
/agent microservice-reviewer
```

### Проверка Go workspace
```
/agent go-workspace-manager
```

### Валидация событий
```
/agent event-schema-validator
```

### Запуск тестов
```
/go-test-service SERVICE=narrative-orchestrator
```

### Проверка Redpanda
```
/check-topics
```

### Мониторинг здоровья
```
/health-all
```

## Ссылки

- [Makefile](../../../../Makefile)
- [docker-compose.yml](../../../../docker-compose.yml)
- [CLAUDE.md](../../../../CLAUDE.md)
- [AUTOMATION-SETUP.md](../../../../AUTOMATION-SETUP.md)