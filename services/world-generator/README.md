# 🌍 WorldGenerator

> **WorldGenerator отвечает за создание и управление игровыми мирами с использованием Ascension Oracle.**

## 🎯 Назначение

WorldGenerator отвечает за генерацию игровых миров и их структуры с использованием Ascension Oracle. Сервис обеспечивает создание схем и сущностей для мира, поддержку различных типов миров и параметров, а также интеграцию с другими сервисами.

### Основные функции:
- Генерация игровых миров и их структуры
- Создание схем и сущностей для мира
- Поддержка различных типов миров и их параметров
- Интеграция с OntologicalArchivist для хранения схем
- Динамическая генерация регионов, ландшафта и воды
- Генерация городов и их характеристик

## 🔄 Жизненный цикл

1. Получает запрос на генерацию мира через `world_events`
2. Запрашивает схему у UniverseGenesisOracle
3. Генерирует полную географическую структуру (регионы, вода, города)
4. Создает сущности мира через EntityManager
5. Публикует готовый мир в `world_events`

## 🧠 Состояние WorldGenerator

- Не хранит состояние мира между генерациями
- Использует схемы для структурирования мира
- Поддерживает различные типы генерации
- Генерирует полную географическую структуру

## 📡 Обработка событий

### Подписка на события:
- `world_events` с типом `world.generate`

### Публикация событий:
- `world.generated` в `world_events`
- `entity.created` для регионов, городов и воды в `world_events` и `system_events`

### Последовательность обработки:
1. Извлекает параметры генерации
2. Запрашивает схему у UniverseGenesisOracle
3. Генерирует географическую структуру (регионы, вода, города)
4. Создает сущности мира через EntityManager
5. Публикует результат в `world_events` и `system_events`

## 📝 Примеры событий для генерации мира

### Входящее событие: World Generation Requested

#### 1. Контекстуальная генерация мира
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "seed": "cultivation-world-2026",
  "mode": "contextual",
  "user_context": {
    "description": "Мир для культивации с магическими реками и летающими островами",
    "theme": "cultivation",
    "key_elements": [
      "magical_rivers",
      "floating_islands",
      "ancient_temples",
      "spirit_beasts",
      "sect_system"
    ],
    "scale": "large",
    "restrictions": [
      "no_modern_technology",
      "no_guns_or_firearms"
    ]
  }
}
```

#### 2. Случайная генерация мира
```json
{
  "seed": "random-789xyz",
  "mode": "random",
  "constraints": {
    "min_regions": 5,
    "max_regions": 10,
    "biome_distribution": {
      "forest": 0.4,
      "desert": 0.2,
      "mountain": 0.2,
      "plains": 0.2
    }
  }
}
```

#### 3. Генерация с пользовательскими ограничениями
```json
{
  "entity": {
    "id": "player-456",
    "type": "player",
    "name": "Линь"
  },
  "seed": "steampunk-world",
  "mode": "contextual",
  "user_context": {
    "description": "Мир в стиле стимпанк с паровыми машинами и аэростатами",
    "theme": "steampunk",
    "key_elements": [
      "steam_machines",
      "aerostats",
      "clockwork_golems",
      "steam_railways"
    ],
    "scale": "medium"
  },
  "constraints": {
    "theme_restrictions": ["no_magic", "no_mana"],
    "technology_level": "industrial_revolution"
  }
}
```

---

### Исходящие события WorldGenerator

#### World Created (с концепцией мира)
```json
{
  "entity": {
    "id": "world-cult-123abc",
    "type": "world",
    "name": "",
    "world": {
      "id": "world-cult-123abc"
    }
  },
  "payload": {
    "seed": "cultivation-world-2026",
    "mode": "contextual",
    "theme": "cultivation",
    "core": "Мир, где энергия ци течёт через магические реки, а практикующие могут летать на летающих островах через достижение высоких уровней культивации.",
    "era": "древний",
    "unique_traits": [
      "magical_rivers_flow_with_cultivation_energy",
      "floating_islands_accessible_by_flight",
      "ancient_temples_holding_secret_cultivation_methods",
      "spirit_beasts_as_cultivation_partners",
      "multi-sect_system_with_competition"
    ],
    "plan": 0,
    "scale": "large"
  }
}
```

#### Region Created — Леса
```json
{
  "entity": {
    "id": "region-forest-456def",
    "type": "region",
    "name": "Лес Древних Деревьев",
    "world": {
      "id": "world-cult-123abc"
    }
  },
  "payload": {
    "name": "Лес Древних Деревьев",
    "biome": "ancient_forest",
    "coordinates": {
      "x": 45.5,
      "y": 67.2
    },
    "size": 1250.5,
    "difficulty": 3,
    "resources": [
      "spirit_wood",
      "herb_spirit_shroom",
      "rare_plant_moonflower"
    ],
    "spirit_beasts": [
      "spirit_fox_level_5",
      "spirit_tiger_level_7"
    ]
  }
}
```

#### Region Created — Пустошь
```json
{
  "entity": {
    "id": "region-desert-789ghi",
    "type": "region",
    "name": "Песчаная Пустошь",
    "world": {
      "id": "world-cult-123abc"
    }
  },
  "payload": {
    "name": "Песчаная Пустошь",
    "biome": "desert",
    "coordinates": {
      "x": 120.3,
      "y": 30.8
    },
    "size": 3500.0,
    "difficulty": 5,
    "resources": [
      "sand_crystal",
      "cactus_essence",
      "ancient_ruin_artifacts"
    ],
    "spirit_beasts": [
      "sand_worm_level_6",
      "phoenix_feather_level_8"
    ],
    "landmarks": [
      "ancient_sunken_temple",
      "oasis_of_mist"
    ]
  }
}
```

#### Water Body Created — Река
```json
{
  "entity": {
    "id": "water-river-012jkl",
    "type": "water_body",
    "name": "Река Ци",
    "world": {
      "id": "world-cult-123abc"
    }
  },
  "payload": {
    "name": "Река Ци",
    "type": "river",
    "coordinates": {
      "x": 75.0,
      "y": 55.0
    },
    "size": 5000.0,
    "flow_speed": "fast",
    "cultivation_benefit": {
      "qi_regen_multiplier": 1.5,
      "risk_level": 3
    },
    "features": [
      "qi_aura_boost",
      "spirit_fish_sporadic",
      "currents_help_flight_practice"
    ]
  }
}
```

#### Water Body Created — Озеро
```json
{
  "entity": {
    "id": "water-lake-345mno",
    "type": "water_body",
    "name": "Озеро Луны",
    "world": {
      "id": "world-cult-123abc"
    }
  },
  "payload": {
    "name": "Озеро Луны",
    "type": "lake",
    "coordinates": {
      "x": 90.5,
      "y": 40.2
    },
    "size": 800.0,
    "depth": "deep",
    "cultivation_benefit": {
      "meditation_bonus": 2.0,
      "spiritual_clarity": true
    },
    "features": [
      "moon_reflection_amplifies_yin_energy",
      "rare_pearls_drop_sporadically",
      "ghost_visits_during_full_moon"
    ]
  }
}
```

#### City Created — Главный город
```json
{
  "entity": {
    "id": "city-main-678pqr",
    "type": "city",
    "name": "Вершина Небесного Клана",
    "world": {
      "id": "world-cult-123abc"
    }
  },
  "payload": {
    "name": "Вершина Небесного Клана",
    "population": 25000,
    "type": "major",
    "location": {
      "region": "Плато Небес",
      "coordinates": {
        "x": 78.5,
        "y": 52.3
      }
    },
    "factions": [
      "heaven_clan_main",
      "sword_immortal_school",
      "alchemist_guild",
      "merchant_union"
    ],
    "facilities": [
      "sect_headquarters",
      "cultivation_arena",
      "marketplace",
      "training_halls",
      "alchemist_district",
      "blacksmith_district"
    ],
    "difficulty_level": 7,
    "security_level": "high",
    "features": [
      "sky_gate_for_teleportation",
      "annual_cultivation_festival",
      "tournaments_for_ranking"
    ]
  }
}
```

#### City Created — Маленький город
```json
{
  "entity": {
    "id": "city-small-901stu",
    "type": "city",
    "name": "Деревня Лунного Света",
    "world": {
      "id": "world-cult-123abc"
    }
  },
  "payload": {
    "name": "Деревня Лунного Света",
    "population": 3500,
    "type": "minor",
    "location": {
      "region": "Долина Туманов",
      "coordinates": {
        "x": 55.2,
        "y": 65.8
      }
    },
    "specialization": "herbalism",
    "features": [
      "spirit_herb_gardens",
      "small_temple_of_wisdom",
      "beginner_cultivation_circle"
    ],
    "notable_npcs": [
      "elder_green_leaf",
      "herbalist_sister_white"
    ]
  }
}
```

#### Geography Generated — Итоговое событие
```json
{
  "world": {
    "id": "world-cult-123abc"
  },
  "regions": {
    "count": 8,
    "types": {
      "ancient_forest": 3,
      "desert": 2,
      "mountain": 2,
      "plains": 1
    }
  },
  "water_bodies": {
    "count": 5,
    "types": {
      "river": 3,
      "lake": 2
    }
  },
  "cities": {
    "count": 3,
    "types": {
      "major": 1,
      "minor": 2
    }
  },
  "generation_summary": {
    "total_entities_created": 16,
    "generation_time_ms": 2500,
    "theme_applied": "cultivation",
    "scale_applied": "large"
  }
}
```

---

### Полный цикл генерации мира

#### Шаг 1: Запрос на генерацию
```json
{
  "entity": {
    "id": "player-123",
    "type": "player",
    "name": "Вася"
  },
  "seed": "my-new-world",
  "mode": "contextual",
  "user_context": {
    "description": "Мир для культивации",
    "theme": "cultivation",
    "scale": "medium"
  }
}
```

#### Шаг 2-5: Генерация World Created
```json
{
  "entity": {
    "id": "world-my-7f8a9b",
    "type": "world"
  },
  "payload": {
    "seed": "my-new-world",
    "theme": "cultivation",
    "core": "Новый мир для культивации",
    "era": "древний"
  }
}
```

#### Шаг 6: Создание регионов
```json
{
  "entity": {
    "id": "region-a1b2c3",
    "type": "region",
    "name": "Лес Духов"
  },
  "payload": {
    "biome": "spirit_forest",
    "coordinates": {"x": 50, "y": 60}
  }
}
```

#### Шаг 7: Создание городов
```json
{
  "entity": {
    "id": "city-d4e5f6",
    "type": "city",
    "name": "Стартовый Город"
  },
  "payload": {
    "population": 5000,
    "type": "minor"
  }
}
```

#### Шаг 8: Финальное событие
```json
{
  "world": {"id": "world-my-7f8a9b"},
  "regions": 4,
  "water_bodies": 3,
  "cities": 2
}
```

## 🌐 Интеграция

### Входящие интеграции:
- **UniverseGenesisOracle**: получение схемы вселенной
- **OntologicalArchivist**: сохранение сгенерированных схем
- **EntityManager**: создание сущностей мира (регионы, города, воды)
- **SemanticMemory**: семантический контекст для генерации
- **CityGovernor**: получает информацию о городах для управления

### Исходящие интеграции:
- Публикует события для других сервисов:
  - `world.generated` для BanOfWorld и RealityMonitor
  - `entity.created` для EntityManager и CityGovernor

## ✅ Преимущества

- Гибкость в генерации различных миров
- Интеграция с AI-сервисами для качественной генерации
- Структурированная генерация схем
- Поддержка масштабирования
- Динамическая генерация полной географической структуры
- Поддержка интеграции с другими сервисами (городов, управления)

## 🛠️ Техническая реализация

### Архитектура:
- Сервис реализован в пакете `services/worldgenerator`
- Использует `eventbus.EventBus` для подписки на события
- Подписывается на `eventbus.TopicWorldEvents` и `eventbus.TopicSystemEvents`
- Использует AscensionOracle для генерации географической структуры
- Генерирует события для других сервисов

### Генерируемые типы сущностей:
- Регионы (`entity_type: region`)
- Водные объекты (`entity_type: water_body`)
- Города (`entity_type: city`)

## 🔧 Конфигурация

### Переменные окружения:
- `ORACLE_URL` - URL Ascension Oracle сервиса
- `KAFKA_BROKERS` - адреса брокеров Kafka/Redpanda
- `MINIO_ENDPOINT` - адрес MinIO хранилища

### Значения по умолчанию:
- `ORACLE_URL`: `http://localhost:8080`
- `KAFKA_BROKERS`: `localhost:9092`
- `MINIO_ENDPOINT`: `localhost:9000`

## 📊 Мониторинг

### Метрики:
- Количество сгенерированных миров
- Время генерации
- Количество сгенерированных сущностей
- Качество сгенерированных схем
- Количество сгенерированных регионов
- Количество сгенерированных городов

### Методы мониторинга:
- Логирование событий
- Сбор метрик через Prometheus
- Отслеживание производительности генерации

---

## 🎬 Полный пример генерации мира

Вот пример полного цикла генерации мира с помощью WorldGenerator:

1. **Отправьте запрос** (в `world_events`):
```json
{
  "entity": {"id": "player-123", "type": "player", "name": "Вася"},
  "seed": "cultivation-world",
  "mode": "contextual",
  "user_context": {
    "description": "Мир для культивации",
    "theme": "cultivation",
    "scale": "medium"
  }
}
```

2. **WorldGenerator создаёт мир** (публикует `entity.created`):
```json
{
  "entity": {"id": "world-abc123", "type": "world"},
  "payload": {
    "seed": "cultivation-world",
    "theme": "cultivation",
    "core": "Мир с магическими реками и летающими островами"
  }
}
```

3. **Создаются регионы** (публикуются `entity.created`):
```json
{
  "entity": {"id": "region-forest", "type": "region", "name": "Лес Духов"},
  "payload": {"biome": "spirit_forest", "coordinates": {"x": 50, "y": 60}}
}
```

4. **Создаются города** (публикуются `entity.created`):
```json
{
  "entity": {"id": "city-starter", "type": "city", "name": "Стартовый Город"},
  "payload": {"population": 5000, "type": "minor"}
}
```

5. **Финальное событие** (`world.geography.generated`):
```json
{
  "world": {"id": "world-abc123"},
  "regions": 4,
  "water_bodies": 3,
  "cities": 2
}
```