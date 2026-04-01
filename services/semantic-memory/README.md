# 🧠 SemanticMemory

> **SemanticMemory — это мощная система памяти с двойным индексированием (векторный + графовый) для RAG.**

## 🎯 Назначение

- Хранение и поиск семантических сущностей
- Поддержка RAG для повествования и других AI-сервисов
- Объединение векторного и графового индексирования
- Поиск по контексту сущностей и их связям
- Хранение всех событий системы для контекста и воспроизведения
- Графовые запросы по связям событий и сущностей

## 📝 Новая структура событий (Structured IDs)

Система поддерживает как **старый плоский формат**, так и **новый структурированный формат**:

### Старый формат (поддерживается)
```json
{
  "entity_id": "player-123",
  "entity_type": "player",
  "target_id": "region-456",
  "world_id": "world-789"
}
```

### Новый формат (рекомендуемый)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася",
    "world": {
      "id": "world-789"
    }
  },
  "target": {
    "entity": {
      "id": "region-456",
      "type": "region",
      "name": "Темный Лес"
    }
  }
}
```

**Backward Compatibility**: Все функции извлечения ID поддерживают оба формата!

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
   - Извлекает entity IDs из payload (поддержка нового формата `entity.id` и старого `entity_id`):
     - `entity_id` → `entity.id`
     - `player_id` → `entity.id`
     - `target_id` → `target.entity.id`
     - `source_id` → `source.entity.id`
     - `npc_id`, `item_id`, `region_id` → fallback keys
   - Создаёт связи `[:RELATED_TO]` между событиями и сущностями
   - Сохраняет payload как JSON строку для совместимости с Neo4j

4. **Structured Context для AI:**
   - Форматирование событий для LLM: `{entity.id:type:name} {timestamp} {action} {target.entity.id:type:name}`
   - Пример: `{player-123:player:Вася} {14:30} {вошел в} {region-456:region:Темный Лес}`

5. Сохраняет все события для контекста и воспроизведения

6. Обновляет контекст для других сервисов

## 📊 Примеры событий для Semantic Memory

### Player Action (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "target": {
    "entity": {
      "id": "npc-456",
      "type": "npc",
      "name": "Старейшина"
    }
  },
  "action": "talked_to",
  "dialogue": {
    "topic": "ancient_tales",
    "mood": "mysterious"
  }
}
```

### Entity Moved (входящее)
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "from": {
    "x": 45.0,
    "y": 67.0
  },
  "to": {
    "x": 48.5,
    "y": 70.2
  },
  "world": {
    "id": "world-789"
  }
}
```

### Weather Change (входящее с вложенной структурой)
```json
{
  "entity": {
    "id": "world-789",
    "type": "world"
  },
  "weather": {
    "change": {
      "to": "шторм",
      "in": {
        "region": {
          "id": "region-456",
          "type": "region"
        }
      }
    },
    "previous": {
      "condition": "ясно",
      "temperature": {
        "value": 25.5,
        "unit": "celsius"
      }
    }
  }
}
```

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
