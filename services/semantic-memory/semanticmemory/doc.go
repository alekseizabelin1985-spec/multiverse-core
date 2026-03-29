/*
Package semanticmemory реализует семантическую память для игровых миров Multiverse.

# Назначение

Сервис индексирует все события игрового мира (движение, бои, диалоги, квесты и т.д.)
в двух хранилищах и предоставляет REST API для получения контекста, необходимого AI-агентам
(нарративный оркестратор, NPC, и др.).

# Архитектура хранилищ

Сервис использует три хранилища одновременно:

  - ChromaDB — векторная БД для семантического поиска по тексту событий.
    Поддерживает два клиента: HTTP (v1, по умолчанию) и официальный Go-клиент (v2, по флагу сборки).

  - Neo4j — граф-БД для хранения связей Event → Entity.
    Позволяет запрашивать все события, связанные с конкретными сущностями, по типу, миру и времени.

  - MinIO — объектное хранилище для снимков состояния сущностей (опционально).

# Поток событий

 1. Внешний сервис публикует событие в EventBus (Kafka/NATS).
 2. Service подписывается на топики: player, world, game, system, scope, narrative.
 3. Indexer.HandleEvent сохраняет событие:
    a. В ChromaDB — текстовое представление + metadata (event_id, event_type, world_id, source, timestamp).
    b. В Neo4j — узел Event + рёбра RELATED_TO к Entity-узлам из payload.
 4. Данные доступны через REST API.

# Конфигурация (переменные окружения)

	CHROMA_URL          URL ChromaDB            (default: http://chromadb:8000)
	CHROMA_USE_V2       Использовать Go-клиент  (default: false)
	CHROMA_COLLECTION_NAME Имя коллекции        (default: world_memory)
	EMBEDING_URL        URL Ollama для эмбеддингов (default: http://qwen3-service:11434)
	EMBEDING_MODEL      Модель эмбеддингов      (default: nomic-embed-text:latest)
	NEO4J_URI           URI Neo4j               (default: neo4j://neo4j:7687)
	NEO4J_USER          Логин Neo4j             (default: neo4j)
	NEO4J_PASSWORD      Пароль Neo4j            (default: password)
	MINIO_ENDPOINT      Адрес MinIO             (default: minio:9000)
	SEMANTIC_PORT       HTTP-порт сервиса       (default: 8080)

# HTTP API

## Контекст для AI

	POST /v1/context
	  Тело: {"entity_ids": ["id1", "id2"], "depth": 1}
	  Ответ: {"contexts": {"id1": "<текст>", ...}}
	  Возвращает текстовый контекст сущностей из ChromaDB.

	POST /v1/context-with-events
	  Тело: {"entity_ids": ["id1"], "event_types": ["player.move"], "depth": 1}
	  Ответ: {"contexts": {"id1": "<текст>"}}
	  Возвращает контекст сущностей, дополненный событиями заданных типов.

	POST /v1/context/structured
	  Тело: {
	    "entity_ids":  ["id1", "id2"],   // обязательно
	    "world_id":    "world-1",        // опционально
	    "region_id":   "region-x",       // опционально
	    "time_range":  "last_24h",       // опционально (default: last_2h)
	    "max_events":  50,               // опционально (default: 50)
	    "event_types": ["player.move"],  // опционально
	    "include_description": true      // опционально
	  }
	  Ответ: StructuredContext — контекст с встроенными ID для LLM,
	         включает поля context, entities, timeline, metadata.

	  Допустимые значения time_range:
	    last_1h, last_2h, last_6h, last_12h, last_24h / last_1d, last_7d, last_30d

## События

	POST /v1/events
	  Тело: {"event_type": "player.move", "limit": 10}
	  Ответ: {"events": ["<текст события>", ...]}
	  Поиск событий по типу через ChromaDB (полнотекстовый).

	GET /v1/events/{event_id}
	  Ответ: JSON объект eventbus.Event или {"error": "event_not_found"} (404).
	  Получение конкретного события по его ID из Neo4j.

	POST /v1/events/query
	  Тело: {
	    "entity_ids":  ["player-1"],   // опционально — события связанных сущностей
	    "world_id":    "world-1",      // опционально — фильтр по миру
	    "event_types": ["player.move"], // опционально — фильтр по типам
	    "time_range":  "last_24h",     // опционально (default: last_2h)
	    "limit":       20              // опционально (default: 10)
	  }
	  Ответ: {"events": [...]}
	  Гибкий запрос событий. Логика выбора источника:
	    1. entity_ids → Neo4j (граф, с фильтрами world_id и time_range)
	    2. world_id без event_types → Neo4j (GetEventsByWorldID)
	    3. один event_type → ChromaDB (SearchEventsByType)
	    4. несколько event_types + world_id → ChromaDB (QueryByMetadata) + фильтр в памяти

## Сущности (Entity)

	GET /v1/entities/{entity_id}
	  Ответ: JSON объект EntityInfo или {"error": "entity_not_found"} (404).
	  Получение сущности по ID из Neo4j.

	POST /v1/entities/query
	  Тело: {
	    "ids":      ["id1", "id2"],  // опционально — список ID (OR внутри списка)
	    "type":     "npc",           // опционально — точное совпадение типа
	    "world_id": "world-1",       // опционально — фильтр по миру
	    "name":     "Васи",          // опционально — подстрока имени (case-insensitive)
	    "limit":    20               // опционально (default: 20)
	  }
	  Ответ: {"entities": [EntityInfo, ...]}
	  Все поля объединяются через AND. Возвращает пустой массив если ничего не найдено.

	GET /v1/entity-context/{entity_id}?time_range=last_24h
	  Ответ: {"entity_id": "...", "context": {...}, "time_range": "last_24h"}
	  Контекст сущности из MinIO (снимок состояния). Требует настроенного MinIO.

## Служебные

	GET /health
	  Ответ: {"status": "healthy", "time": "<RFC3339>"}

# Ключевые типы

## SemanticStorage (storage.go)

Интерфейс для смены бэкенда векторного хранилища:
  - UpsertDocument    — добавить/обновить документ
  - GetDocuments      — получить документы по списку ID
  - SearchEventsByType — найти события по типу (through /get с where-фильтром)
  - QueryByMetadata   — гибкий запрос по metadata-полям (event_type, world_id, ...)
  - Close             — закрыть соединение

Реализации: ChromaClient (chroma.go), ChromaV2Client (chroma_v2.go, тег сборки chroma_v2_enabled).

## EntityInfo (context_structured.go)

	type EntityInfo struct {
	    ID          string
	    Name        string
	    Type        string
	    WorldID     string
	    Description string
	    Payload     map[string]interface{}
	    Coordinates *Coordinates  // position из payload: {x, y, z}
	}

	type Coordinates struct {
	    X float64
	    Y float64
	    Z float64
	}

## EntityQuery (entity.go)

Фильтр для запроса сущностей. Все поля опциональны, объединяются через AND.

## StructuredContext (context_structured.go)

Ответ /v1/context/structured. Содержит:
  - Context  — строка с встроенными ID в формате {id:имя} для LLM
  - Entities — карта EntityInfo по ID
  - Timeline — хронологический список событий с форматированием
  - Metadata — статистика запроса (время, кол-во сущностей/событий, источники)

# Схема Neo4j

Узлы:
  - (:Entity {id, type, name, world_id, description, ...payload})
  - (:Event  {id, type, timestamp, source, world_id, scope_id, payload, raw_data})

Рёбра:
  - (:Event)-[:RELATED_TO]->(:Entity)  — создаётся при сохранении события
  - (:Entity)-[:CONTAINS]->(:Entity)   — создаётся для инвентаря (entity.created)

# ChromaDB metadata-поля событий

При сохранении каждого события в ChromaDB индексируются следующие поля:
  - event_id, event_type, world_id, source, timestamp, scope_id (если есть)

Используйте QueryByMetadata для фильтрации по любому из этих полей.
*/
package semanticmemory
