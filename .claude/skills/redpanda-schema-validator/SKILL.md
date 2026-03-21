---
name: redpanda-schema-validator
description: Валидация схем событий Redpanda и проверка совместимости между сервисами
user-invocable: false
---

Валидирует схемы событий Redpanda на совместимость и целостность.

## Автоматическая валидация

Запускается автоматически при:
- Изменении событий в коде
- Изменении конфигурации тем
- Добавлении новых сервисов

## Основные проверки

### 1. Проверка существования тем

Проверяет, что все упомянутые темы существуют в:
- `docker-compose.yml` (redpanda-init)
- Фактически запущены в Redpanda

**Темы**:
- `player_events`
- `world_events`
- `game_events`
- `system_events`
- `scope_management`
- `narrative_output`

### 2. Проверка структур событий

**Обязательные поля Event**:
```go
type Event struct {
    ID        uuid.UUID                // required
    Type      string                 // required
    Timestamp time.Time              // required
    Payload   map[string]interface{} // required
    Topic     string                 // optional
}
```

**Проверки**:
- Все события должны иметь UUID
- Тип события не пустой
- Timestamp не zero value
- Payload не nil

### 3. Проверка совместимости schema

**Breaking changes** (запрещены без версионирования):
- Изменение типа поля `string` -> `int`
- Удаление обязательного поля
- Изменение формата UUID

**Non-breaking changes** (допустимы):
- Добавление optional полей
- Добавление новых тем
- Уменьшение обязательности полей

### 4. Проверка publisher/subscriber consistency

**Проверки**:
- Все сервисы-издатели используют существующие темы
- Все сервисы-потребители подписаны на правильные темы
- No typos в названиях тем

## Ошибки и предупреждения

### Critical errors (блокируют коммит):

1. **Missing topic**: Сервис использует несуществующую тему
2. **Invalid payload**: Структура события не валидна
3. **Breaking change**: Изменение, ломающее обратную совместимость

### Warnings (рекомендации):

1. **Deprecated field**: Поле устарело, рекомендуется использовать новое
2. **Missing documentation**: Событие не документировано
3. **Typo in topic**: Возможно, опечатка в названии темы

## Примеры

### Валидация нового события

```go
// BAD - missing required fields
type BadEvent struct {
    Type string  // missing ID, Timestamp
    Data interface{}
}

// GOOD - all required fields
type GoodEvent struct {
    ID        uuid.UUID
    Type      string
    Timestamp time.Time
    Payload   map[string]interface{}
}
```

### Breaking change detection

```go
// V1
type EntityEvent struct {
    EntityID string  // string format
}

// V2 - BREAKING: changed to UUID
type EntityEvent struct {
    EntityID uuid.UUID  // breaking change!
}

// V2 - NON-BREAKING: new optional field
type EntityEvent struct {
    EntityID uuid.UUID
    Metadata map[string]string  // optional
}
```

## Интеграция с hooks

### Pre-commit hook

```json
{
  "PreCommit": {
    "services/*/cmd/*.go": "redpanda-schema-validator --strict"
  }
}
```

### Pre-push hook

```json
{
  "PrePush": {
    "*": "redpanda-schema-validator --check-all"
  }
}
```

## Конфигурация

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDPANDA_SCHEMA_MODE` | `strict` | strict | warn |
| `CHECK_PERSISTENCE` | `true` | Проверка тем в docker-compose |
| `ALLOW_BREAKING_CHANGES` | `false` | Разрешить breaking changes |

## Примеры использования

### Проверить все события

```bash
redpanda-schema-validator --check-all
```

### Проверить только новый сервис

```bash
redpanda-schema-validator --service cultiation-module
```

### В режиме предупреждений

```bash
redpanda-schema-validator --mode warn
```

### Строгая валидация

```bash
redpanda-schema-validator --strict
```

## Checkpoints в workflow

1. **При создании события**:
   - Проверить структуру Event
   - Убедиться, что Type уникален
   - Проверить Topic существует

2. **При изменении события**:
   - Проверить обратную совместимость
   - Обновить документацию
   - Протестировать consumers

3. **Перед коммитом**:
   - `redpanda-schema-validator --check-all`
   - Проверить все publishers
   - Проверить все subscribers

## Ссылки

- Event Bus: `shared/eventbus/eventbus.go`
- Docker Compose: `docker-compose.yml` (redpanda-init)
- Service Docs: `services/*/README.md`
- Agent: `.claude/agents/event-schema-validator.md`
