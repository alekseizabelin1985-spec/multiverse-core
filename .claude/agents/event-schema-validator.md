# Event Schema Validator

Проверяет согласованность событий (events) в event-driven архитектуре multiverse-core.

## Event Topics

**Основные темы Redpanda**:
- `player_events` - действия игроков
- `world_events` - изменения состояния мира
- `game_events` - игровые механики
- `system_events` - системные операции
- `scope_management` - жизненный цикл scope
- `narrative_output` - результаты нарратива

## Структура события

```go
type Event struct {
    ID        uuid.UUID                 // уникальная ID события
    Type      string                   // тип события (e.g., "entity.create")
    Timestamp time.Time                // время создания
    Payload   map[string]interface{}   // данные события
    Topic     string                   // тема Redpanda
}
```

## Основные паттерны событий

### 1. Entity Events (entity-manager)
```json
{
  "type": "entity.created",
  "payload": {
    "id": "uuid",
    "name": "string",
    "type": "character|item|location",
    "attributes": {},
    "parent_id": "optional"
  }
}
```

### 2. Narrative Events (narrative-orchestrator)
```json
{
  "type": "narrative.scoped",
  "payload": {
    "scope_id": "uuid",
    "world_id": "uuid",
    "entities": ["uuids"],
    "context": "string"
  }
}
```

### 3. Cultivation Events (cultivation-module)
```json
{
  "type": "cultivation.progressed",
  "payload": {
    "player_id": "uuid",
    "realm": "string",
    "experience": number
  }
}
```

## Задачи валидации

### 1. Проверка совместимости схем

**Противоречия**:
- Изменение типа поля с `string` на `number`
- Удаление обязательного поля payload
- Изменение формата UUID

**Рекомендации**:
- Использовать forward/backward compatibility
- Добавлять новые поля как `optional`
- ДеPRECATED вместо удаления полей

### 2. Проверка publish/subscribe

- Проверить, что topic существует в Redpanda
- Выявить несоответствия в именах тем (типографы)
- Проверить, что потребитель подписан на тему издателя

### 3. Проверка зависимостей между сервисами

**Publishers**:
- `entity-manager` -> `entity.created`, `entity.updated`, `entity.history.appended`
- `narrative-orchestrator` -> `narrative.scoped`, `narrative.completed`
- `cultivation-module` -> `cultivation.progressed`, `cultivation.ascended`
- `city-governor` -> `quest.started`, `quest.completed`, `economy.transacted`

**Consumers**:
- `semantic-memory` -> all event topics
- `ban-of-world` -> `entity.updated`
- `rule-engine` -> various event types

### 4. Валидация payload

**Обязательные поля для всех событий**:
- `id` - uuid.UUID
- `type` - string
- `timestamp` - time.Time
- `payload` - map[string]interface{}

**Паттерны для payload**:
- Использовать `snake_case` для ключей
- Избегать вложенных объектов > 3 уровня
- Хранить UUID как строки в формате RFC 4122

## Рекомендуемые практики

### 1. Версионирование событий

```go
// v1
type EventV1 struct {
    Type   string `json:"type"`
    Player string `json:"player_id"`
}

// v2 (breaking change avoided)
type EventV2 struct {
    Type   string `json:"type"`
    Player PlayerInfo `json:"player"`  // structured instead of string
}
```

### 2. Schema Registry

Использовать MinIO для хранения схем:
```
s3://schemas/{topic}/schemas.json
s3://schemas/{topic}/v1.json
s3://schemas/{topic}/v2.json
```

### 3. Проверка до изменений

```bash
# Проверить, что все сервисы компилируются
make build-all

# Проверить тесты
make test

# Проверить зависимости
go work sync
```

## Чек-лист перед коммитом

- [ ] Новые события имеют уникальный type
- [ ] Payload валидирован против существующих паттернов
- [ ] Topic существует в Redpanda (или добавлен в redpanda-init)
- [ ] Обратная совместимость сохранена
- [ ] Документирован в README сервиса
- [ ] Есть пример в тесте

## Пример использования

```
/agent event-schema-validator

# Проверить новый event type
"Проверь событие 'cultivation.ascended' на соответствие паттернам"

# Проверить совместимость
"Проверь, что изменение payload entity.updated обратно совместимо"

# Найти несоответствия
"Найди сервисы, которые публикуют на непроведенные темы"
```

## Ссылки

- Event Bus: `shared/eventbus/eventbus.go`
- Redpanda topics: docker-compose.yml (redpanda-init)
- MinIO schemas: `ontological-archivist` service
