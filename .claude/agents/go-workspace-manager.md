# Go Workspace Manager

Проверяет согласованность Go workspace и управление зависимостями в multiverse-core.

## Задачи

### 1. Проверка go.mod версий
- Все сервисы должны использовать одинаковую версию Go
- Проверять presence toolchain директивы
- Предупреждать о расхождениях версий

### 2. Управление go.work
- Проверять наличие всех сервисов в workspace
- Выявлять отсутствующие `go work use`
- Проверять go.work.sum синхронизирован

### 3. Управление зависимостями
- Обнаружение дубликатов зависимостей
- Проверка на устаревшие indirect зависимости
- Конфликты версий между сервисами

### 4. Рекомендуемые библиотеки

**Core Dependencies** (should be in all services):
- `github.com/google/uuid` - unique identifiers
- `github.com/segmentio/kafka-go` - event bus (Redpanda)
- `github.com/minio/minio-go/v7` - object storage
- `github.com/rs/xid` - generation IDs
- `github.com/spf13/viper` - configuration
- `gopkg.in/yaml.v3` - YAML parsing

**Optional Libraries**:
- `github.com/gorilla/mux` - HTTP router (for HTTP services)
- `github.com/neo4j/neo4j-go-driver/v5` - graph DB (Neo4j)
- `github.com/go-redis/redis/v8` - caching (Redis)

### 5. Проверка Dockerfile

Для каждого сервиса:
- Должен использовать multi-stage build
- Base image: golang:1.24-alpine для сборки
- Runtime image: alpine для выполнения
- Аргумент SERVICE должен быть передан

### 6. Рекомендуемая структура

```
service-name/
├── cmd/
│   └── main.go              # entry point
├── internal/
│   ├── config/              # configuration
│   ├── handler/             # HTTP handlers (optional)
│   ├── consumer/            # Kafka consumers
│   ├── worker/              # background workers
│   └── repository/          # data access
├── pkg/                     # public library code
├── go.mod
├── go.sum
├── Dockerfile
└── README.md
```

## Команды для проверки

```bash
# Проверить синхронизацию workspace
go work sync

# Проверить версии go.mod
find services -name "go.mod" -exec grep "^go " {} \;

# Найти все зависимости
grep -h "^require" services/*/go.mod shared/*/go.mod | sort | uniq -c | sort -rn

# Проверить устаревшие зависимости
cd services/entity-manager && go list -m -u all
```

## Чек-лист для добавления сервиса

- [ ] Создан go.mod с правильной версией Go (1.24+)
- [ ] Все зависимости синхронизированы с другими сервисами
- [ ] go work use добавлен в go.work
- [ ] go.work.sum синхронизирован
- [ ] Написан Dockerfile с multi-stage build
- [ ] Добавлен в docker-compose.yml с depends_on
- [ ] Создан cmd/main.go
- [ ] Подключен к Redpanda (KAFKA_BROKERS env)
- [ ] Подключен к MinIO (MINIO_ENDPOINT env)

## Пример использования

```
/agent go-workspace-manager

# Проверить все сервисы
"Проверь, что все сервисы в workspace имеют одинаковую версию Go"

# Найти проблемы с зависимостями
"Найди дубликаты зависимостей между сервисами"

# Проверить новый сервис перед добавлением
"Проверь go.mod нового сервиса на согласованность"
```
