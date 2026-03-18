# 🧠 SemanticMemory

> **SemanticMemory — это мощная система памяти с двойным индексированием (векторный + графовый) для RAG.**

## 🎯 Назначение

- Хранение и поиск семантических сущностей
- Поддержка RAG для повествования и других AI-сервисов
- Объединение векторного и графового индексирования
- Поиск по контексту сущностей и их связям
- Хранение всех событий системы для контекста и воспроизведения
- Графовые запросы по связям событий и сущностей

## 🔄 Жизненный цикл

Система работает как **stateless сервис**:

1. Получает события через топики Kafka/Redpanda
2. Индексирует сущности в ChromaDB (векторный индекс)
3. Создаёт графовые узлы и связи в Neo4j
4. Сохраняет все события для контекста и воспроизведения
5. Обеспечивает поиск по запросам

## 🧠 Состояние SemanticMemory

- Векторный индекс в ChromaDB
- **Графовая модель в Neo4j с отдельными узлами Event и Entity**
- Индексы Neo4j: `event_id`, `entity_id`, `event_type`, `world_id`, `timestamp`
- События хранятся как узлы `:Event` с свойствами: `id`, `type`, `timestamp`, `source`, `world_id`, `scope_id`, `payload_json`
- Сущности хранятся как узлы `:Entity` с свойствами: `id`, `type`, `world_id`, `payload`
- Связи между событиями и сущностями: `(:Event)-[:RELATED_TO]->(:Entity)`

## 📡 Обработка событий

1. Подписывается на все топики событий:
   - `player_events`
   - `world_events`
   - `game_events`
   - `system_events`
   - `scope_management`
   - `narrative_output`

2. Извлекает данные событий

3. **Графовое сохранение в Neo4j:**
   - Создаёт узел `:Event` с метаданными события
   - Извлекает entity IDs из payload: `entity_id`, `player_id`, `target_id`, `source_id`, `actor_id`
   - Создаёт связи `[:RELATED_TO]` между событиями и сущностями
   - Сохраняет payload как JSON строку для совместимости с Neo4j

4. Сохраняет все события для контекста и воспроизведения

5. Обновляет контекст для других сервисов

## 🌐 Графовая модель Neo4j

### Узлы

```cypher
:Event {
  id: string,              // EventID события
  type: string,            // EventType события
  timestamp: datetime,     // Временная метка
  source: string,          // Источник события
  world_id: string,        // ID мира
  scope_id: string,        // ID scope (опционально)
  payload_json: string     // Весь payload как JSON строка
}

:Entity {
  id: string,              // ID сущности
  type: string,            // Тип сущности
  world_id: string,        // ID мира
  payload: map            // payload как JSON строка
}
```

### Связи

```cypher
(:Event)-[:RELATED_TO]->(:Entity)
```

### Индексы

```cypher
CREATE INDEX event_id FOR (e:Event) ON (e.id);
CREATE INDEX entity_id FOR (e:Entity) ON (e.id);
CREATE INDEX event_type FOR (e:Event) ON (e.type);
CREATE INDEX world_id FOR (e:Event) ON (e.world_id);
CREATE INDEX event_timestamp FOR (e:Event) ON (e.timestamp);
CREATE INDEX entity_type FOR (e:Entity) ON (e.type);
CREATE INDEX entity_world_id FOR (e:Entity) ON (e.world_id);
```

### Примеры запросов

```cypher
// Все события для сущности через граф
MATCH (e:Event)-[:RELATED_TO]->(en:Entity)
WHERE en.id = 'player-123'
RETURN e
ORDER BY e.timestamp DESC
LIMIT 10

// События по миру и типу
MATCH (e:Event)
WHERE e.world_id = 'world-456' AND e.type = 'player.action.attack'
RETURN e.payload_json
ORDER BY e.timestamp DESC

// Все сущности в мире
MATCH (e:Entity)
WHERE e.world_id = 'world-456'
RETURN e.id, e.type, e.payload
```

## 🌐 Интеграция

- **EntityManager**: получение данных сущностей
- **NarrativeOrchestrator**: контекст для повествования
- **AscensionOracle**: семантический контекст для AI
- **WorldGenerator**: контекст для генерации мира
- **BanOfWorld**: контекст для мониторинга целостности мира
- **CityGovernor**: контекст для управления городами

## 🚀 API Endpoints

### POST /v1/context/structured
Получить структурированный контекст для нескольких сущностей с событиями.

**Request:**
```json
{
  "entity_ids": ["player-1", "enemy-2"],
  "event_types": ["player.action.attack", "player.action.move"],
  "world_id": "world-123",
  "time_range": "last_24h",
  "max_events": 50
}
```

### POST /v1/events/query
Гибкий поиск событий по фильтрам.

**Request:**
```json
{
  "entity_ids": ["player-1"],
  "world_id": "world-123",
  "event_types": ["player.action.attack"],
  "time_range": "last_1h",
  "limit": 20
}
```

### GET /v1/events/{event_id}
Получить событие по ID из Neo4j.

### GET /v1/entities/{entity_id}
Получить сущность по ID из Neo4j.

### POST /v1/entities/query
Поиск сущностей по фильтрам.

**Request:**
```json
{
  "ids": ["player-1", "player-2"],
  "type": "player",
  "world_id": "world-123",
  "name": "hero",
  "limit": 20
}
```

## ✅ Преимущества

- Двойное индексирование для точного поиска
- **Графовые запросы по связям событий и сущностей**
- **Динамическое извлечение entity IDs из payload**
- **Индексы для эффективных поисковых запросов**
- Поддержка сложных связей между сущностями
- Гибкость в поиске по семантике
- Масштабируемость через распределённую архитектуру
- Полная история событий для контекста и воспроизведения
- Возможность извлечения событий по типу для анализа

## 🛠️ Техническая реализация

- Сервис реализован в пакете `services/semanticmemory`
- Использует `eventbus.EventBus` для подписки на события
- Подписывается на все топики событий
- Использует ChromaDB для векторного поиска
- Использует **Neo4j v5 driver для графовой модели**
- Предоставляет HTTP API для получения контекста и событий

### Ключевые функции

| Функция | Описание |
|---------|----------|
| `SaveEventAsGraph()` | Сохраняет событие как узел Event в Neo4j |
| `LinkEventToEntities()` | Создаёт связи RELATED_TO между событием и сущностями |
| `GetEventsByEntity()` | Получает события для сущности через графовые связи |
| `GetEventsByWorldAndType()` | Получает события по миру и типу |
| `GetEntitiesByType()` | Получает сущности по типу и миру |
| `ExtractNestedEntityIDs()` | Извлекает entity IDs из вложенных структур payload |

## 🔧 Конфигурация

Переменные окружения:
- `CHROMA_URL` — адрес ChromaDB (по умолчанию: `http://chromadb:8000`)
- `CHROMA_USE_V2` — использовать ChromaDB v2 (по умолчанию: `false`)
- `NEO4J_URI` — адрес Neo4j (по умолчанию: `neo4j://neo4j:7687`)
- `NEO4J_USER` — пользователь Neo4j (по умолчанию: `neo4j`)
- `NEO4J_PASSWORD` — пароль Neo4j (по умолчанию: `password`)
- `MINIO_ENDPOINT` — адрес MinIO (по умолчанию: `minio:9000`)
- `EMBEDDING_URL` — адрес Ollama с эмбеддингами
- `EMBEDDING_MODEL` — модель для эмбеддингов
- `SEMANTIC_PORT` — порт HTTP сервера (по умолчанию: `8080`)

## 📊 Мониторинг

- Количество проиндексированных сущностей
- Количество сохранённых событий в Neo4j
- Время поиска
- Качество векторных представлений
- Размеры баз данных
- Количество графовых связей
- Латентность графовых запросов
