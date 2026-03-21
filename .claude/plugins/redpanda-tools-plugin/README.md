# Redpanda Tools Plugin

Плагин для автоматизации работы с Redpanda в multiverse-core.

## Skills

### 1. redpanda-topic-check (existing)
Проверка обязательных тем Redpanda.

**Invocation**: `/check-topics`, `/create-topic`

### 2. redpanda-lag-monitor
Мониторинг lag потребителей Redpanda.

**Commands**:
- `/redpanda-lag` - показать lag для всех тем
- `/redpanda-lag-topic TOPIC=<name>` - lag для конкретной темы

### 3. redpanda-replay
Воспроизведение событий из Redpanda для тестирования.

**Commands**:
- `/redpanda-replay TOPIC=<topic> FROM=<timestamp>` - replay событий
- `/redpanda-replay-latest TOPIC=<topic>` - replay последних событий

### 4. redpanda-schema-check
Проверка схемы событий против schema registry.

**Commands**:
- `/redpanda-schema-check TOPIC=<topic>` - проверить схему
- `/redpanda-schema-validate` - валидировать все схемы

## Установка

```bash
# Plugin уже установлен в .claude/plugins/redpanda-tools-plugin/
```

## Конфигурация

### Переменные окружения

| Variable | Default | Description |
|----------|---------|-------------|
| `REDPANDA_BROKERS` | `localhost:9092` | Brokers Redpanda |
| `SCHEMA_REGISTRY` | `localhost:8081` | Schema Registry URL |
| `KAFKA_CONSUMER_GROUP` | `multiverse-consumers` | Consumer group ID |

## Использование

### Проверка тем

```bash
/check-topics
```

Покажет статус всех обязательных тем:
- player_events
- world_events
- game_events
- system_events
- scope_management
- narrative_output

### Проверка lag

```bash
/redpanda-lag
```

Покажет lag для каждого consumer group.

### Replay событий

```bash
/redpanda-replay TOPIC=player_events FROM=2026-03-21T10:00:00Z
```

### Валидация схем

```bash
/redpanda-schema-check TOPIC=entity_events
```

## Примеры

### Проверка перед деплоем

```bash
/check-topics && /redpanda-lag && /redpanda-schema-check
```

### Мониторинг после изменений

```bash
/redpanda-replay-latest TOPIC=world_events
```

## Troubleshooting

### Темы не создались

```bash
/create-topic player_events world_events
```

### Высокий lag

```bash
/redpanda-lag TOPIC=player_events
# Проверьте consumer group и скорость обработки
```

### Schema registry недоступен

```bash
/redpanda-schema-check TOPIC=player_events --skip-registry
```

## Ссылки

- Docker Compose: `docker-compose.yml` (redpanda, redpanda-console)
- Makefile: `make logs-service SERVICE=redpanda`
- Docs: `shared/eventbus/`, `services/*/README.md`
