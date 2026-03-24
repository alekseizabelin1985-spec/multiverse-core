# 🌌 UniverseGenesisOracle

> **UniverseGenesisOracle — это специализированный AI-сервис для генерации вселенных с философской целостностью.**

## 🎯 Назначение

- Генерация новых вселенных с соблюдением философской целостности
- Создание структуры мира и его основных параметров
- Поддержка семантической глубины в генерации
- Генерация схем и структур для WorldGenerator

## 🔄 Жизненный цикл

Сервис работает как **long-running сервис**:

1. При запуске подключается к Redpanda и начинает слушать топик `system_events`
2. Ожидает события `universe.genesis.request`
3. При получении события генерирует структуру вселенной
4. Публикует событие `universe.genesis.completed` при завершении
5. Продолжает работать и ожидать новых запросов

## 🧠 Состояние UniverseGenesisOracle

- Не хранит состояние между запросами
- Использует Qwen3 для генерации
- Обеспечивает семантическую глубину и философскую целостность
- Работает постоянно, обрабатывая события по требованию

## 📡 Обработка событий

### Входное событие: `universe.genesis.request`

Топик: `system_events`

| Поле | Тип | Описание |
|------|-----|----------|
| `event_type` | string | `"universe.genesis.request"` |
| `world_id` | string | Seed вселенной (если пустой — генерируется случайно) |
| `payload.constraints` | []string | Ограничения генерации (например: `["no_healing", "ascension_through_suffering"]`) |

**Пример события:**
```json
{
  "event_id": "uuid",
  "event_type": "universe.genesis.request",
  "timestamp": "2026-03-24T10:00:00Z",
  "source": "world-generator",
  "world_id": "my-universe-seed",
  "payload": {
    "constraints": ["no_healing", "ascension_through_suffering"]
  }
}
```

### Выходное событие: `universe.genesis.completed`

Топик: `system_events`

| Поле | Тип | Описание |
|------|-----|----------|
| `event_type` | string | `"universe.genesis.completed"` |
| `world_id` | string | Seed сгенерированной вселенной |
| `payload` | object | Содержит `genesis_seed`, `universe_core`, `cosmic_laws` |

**Пример события:**
```json
{
  "event_id": "uuid",
  "event_type": "universe.genesis.completed",
  "timestamp": "2026-03-24T10:05:00Z",
  "source": "universe-genesis-oracle",
  "world_id": "my-universe-seed",
  "payload": {
    "genesis_seed": "my-universe-seed",
    "universe_core": "описание ядра",
    "cosmic_laws": ["закон 1", "закон 2"]
  }
}
```

## 🌐 Интеграция

- **WorldGenerator**: получает сгенерированную структуру через `universe.genesis.completed`
- **OntologicalArchivist**: сохранение сгенерированных схем (universe_core, universe_ontology_profile, entity schemas)
- **AscensionOracle**: философская целостность генерации

## ✅ Преимущества

- Философская целостность генерации
- Семантическая глубина в AI-ответах
- Гибкость в генерации различных типов вселенных
- Интеграция с другими AI-сервисами
- Асинхронная обработка запросов через события

## 🛠️ Техническая реализация

- Сервис реализован в пакете `services/universegenesis`
- Использует `eventbus.EventBus` для подписки на события
- Подписывается на `eventbus.TopicSystemEvents`
- Использует AscensionOracle для генерации
- Сервис продолжает работать после генерации, ожидая новых запросов

## 🔧 Конфигурация

- Переменные окружения:
  - `KAFKA_BROKERS` — адрес Redpanda (по умолчанию: `localhost:9092`)
  - `ARCHIVIST_URL` — адрес OntologicalArchivist (по умолчанию: `http://localhost:8083`)
  - `ORACLE_URL` — адрес AI-модели (по умолчанию: `http://localhost:11434/v1/chat/completions`)

## 📊 Мониторинг

- Количество сгенерированных вселенных
- Время генерации
- Качество сгенерированных структур
- Использование ресурсов AI-модели
- Очередь ожидающих запросов на генерацию
