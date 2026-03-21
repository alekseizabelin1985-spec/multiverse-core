# Автоматизация Claude Code для Multiverse-Core

Этот файл описывает установленные компоненты автоматизации.

## 📦 Установленные компоненты

### Subagents

#### microservice-reviewer
**Путь**: `.claude/agents/microservice-reviewer.md`

Проверяет:
- Docker Compose конфигурации
- Go модули и зависимости
- Event Bus темы (Redpanda)
- Архитектурные паттерны сервисов

### Skills

1. **make-command** - выполнение команд Makefile
   - `make build`, `make test`, `make logs`, `make run`
   - `/make-command`

2. **docker-compose-control** - управление Docker сервисами
   - Статус, запуск/остановка, логи
   - `/dc-status`, `/dc-start`, `/dc-logs`

3. **redpanda-topic-check** - управление темами Redpanda
   - Проверка обязательных тем
   - `/check-topics`, `/create-topic`

4. **service-health-check** - мониторинг сервисов
   - Проверка HTTP endpoints
   - `/health-all`, `/health <service>`

### MCP Серверы

В файле `.mcp.json` настроены:
- **memory-new** - память для кросс-сессий
- **context7** - документация библиотек
- **database** - прямая работа с БД

### Plugin

**Путь**: `.claude/plugins/multiverse-core-plugins/`

Содержит:
- `package.json` - манифест плагина
- `README.md` - документация

## 🚀 Установка MCP серверов

### 1. Context7
```bash
# Установить context7 MCP сервер
claude mcp add context7
```

### 2. Database MCP
```bash
# Установить database MCP сервер
claude mcp add database

# Настроить переменные окружения
echo "DB_MCP_TOKEN=your_token" >> .mcp.env
```

### 3. Настройка подключений

Создайте файл `.mcp.env` с переменными:
```bash
DB_MCP_TOKEN=your_token
CONTEXT7_API_KEY=your_api_key
```

## 🔧 Использование

### Проверка сервиса перед коммитом
```bash
# Subagent проверит согласованность
/agent microservice-reviewer
```

### Выполнение Makefile команд
```bash
# Запуск конкретного сервиса
/make-command
# Выберите "Run service", введите "SERVICE=narrative-orchestrator"
```

### Проверка Redpanda тем
```bash
/ check-topics
```

### Мониторинг здоровья
```bash
/health-all
```

## 📝 Рекомендуемый workflow

1. **Перед коммитом**:
   ```
   /check-service SERVICE=<your-service>
   ```

2. **Во время разработки**:
   ```
   /health <service>
   /dc-logs <service>
   ```

3. **При добавлении нового сервиса**:
   ```
   /agent microservice-reviewer
   ```

## 🐛 Troubleshooting

### MCP серверы не подключаются
```bash
# Проверить статус
claude mcp list

# Перезапустить
claude mcp remove context7
claude mcp add context7
```

### Database MCP недоступен
```bash
# Проверить переменные окружения
cat .mcp.env

# Проверить подключение к базам
/dc-status
```

## 📚 Документация

- [CLAUDE.md](CLAUDE.md) - общая документация проекта
- [docker-compose.yml](docker-compose.yml) - конфигурация сервисов
- [Makefile](Makefile) - доступные команды