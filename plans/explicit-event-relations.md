# 🎯 Explicit Event Relations — План реализации

> **Цель:** Перенести ответственность за генерацию связей из `semantic-memory` в `Oracle/GM/WorldGenerator`.  
> **Принцип:** «Кто генерирует событие — тот знает какие связи нужны».  
> **Результат:** Семантический граф с типизированными связями, без эвристик и LLM-угадываний.

---

## 📊 Контекст и проблема

### Текущее состояние
```
Oracle → [Event без связей] → EventBus → semantic-memory → [Эвристика извлечения] → Neo4j
                                                        ↑
                                            extractEntitiesFromPayload()
                                            «Угадай по 15+ ключам в payload»
```

**Проблемы:**
| Проблема | Последствие |
|----------|-------------|
| semantic-memory «угадывает» связи | Теряется семантика (всё → `[:RELATED_TO]`) |
| Хардкод 15+ ключей в `extractEntitiesFromPayload()` | Сложно поддерживать, невозможно тестировать |
| Oracle не передаёт контекст | Дублирование логики, расхождения в графе |
| Нет типизации связей | Невозможно query: «Кто владеет мечом?» |

### Целевое состояние
```
Oracle → [Event + Relations] → EventBus → semantic-memory → [Применение связей] → Neo4j
           ↑                                          ↑
   Знает контекст:                          «Не думаю — исполняю»
   «Игрок p1 нашёл меч sword1»              MERGE (p1)-[:FOUND]->(sword1)
```

---

## 📐 Распределение ответственности

| Компонент | Было | Стало |
|-----------|------|-------|
| **Oracle/GM/WorldGenerator** | Генерирует только payload | Генерирует payload + **явные relations** |
| **EventBus** | Транспортирует событие | Без изменений |
| **semantic-memory** | Извлекает связи эвристикой | Применяет relations из события, создаёт сущности-заглушки |
| **Neo4j** | Хранит `[:RELATED_TO]` | Хранит семантические связи: `[:FOUND]`, `[:LOCATED_IN]`, etc. |

---

## 🏗️ Архитектура новых структур

### 1. Расширение Event struct
```go
// shared/eventbus/types.go
type Event struct {
    EventID   string         `json:"event_id"`
    EventType string         `json:"event_type"`
    Timestamp time.Time      `json:"timestamp"`
    Source    string         `json:"source"`
    WorldID   string         `json:"world_id,omitempty"`
    ScopeID   *string        `json:"scope_id,omitempty"`
    Payload   map[string]any `json:"payload"`
    
    // ✨ NEW: Явные связи для графа (опционально)
    Relations []Relation `json:"relations,omitempty"`
}

type Relation struct {
    From     string         `json:"from"`      // ID сущности-источника
    To       string         `json:"to"`        // ID сущности-цели
    Type     string         `json:"type"`      // Тип связи
    Directed bool           `json:"directed"`  // true = однонаправленная
    Metadata map[string]any `json:"metadata,omitempty"`
}
```

### 2. Константы типов связей
```go
// shared/eventbus/relation_types.go
const (
    // Действия
    RelActedOn     = "ACTED_ON"
    RelFound       = "FOUND"
    RelMovedTo     = "MOVED_TO"
    RelUsedItem    = "USED_ITEM"
    
    // Владение и расположение
    RelPossesses   = "POSSESSES"
    RelLocatedIn   = "LOCATED_IN"
    RelWorldOf     = "WORLD_OF"
    RelContains    = "CONTAINS"
    
    // Социальные
    RelAlliedWith  = "ALLIED_WITH"   // undirected
    RelHostileTo   = "HOSTILE_TO"
    
    // Пространственные
    RelAdjacentTo  = "ADJACENT_TO"   // undirected
)
```

### 3. Формат ID сущностей
Рекомендуемый формат: `{type}:{id}`
```
"player:p123"
"item:sword_456"
"region:forest_1"
"world:w1"
"npc:merchant_5"
"location:tavern_2"
```

---

## 🧭 Этапы реализации

### Этап 1: Структуры данных (1-2 дня)

**Файлы:**
- `shared/eventbus/types.go` — добавить `Relation` и `Relations []Relation`
- `shared/eventbus/relation_types.go` — новый файл с константами
- `shared/eventbus/validator.go` — новый файл с валидацией relations

**Задачи:**
- [x] 1.1 Добавить `Relation` struct
- [x] 1.2 Добавить `Relations []Relation` в `Event`
- [x] 1.3 Создать `relation_types.go` с константами
- [x] 1.4 Создать `ValidateEventRelations()` для валидации
- [x] 1.5 Обновить конструкторы событий (опциональные relations)
- [x] 1.6 Написать unit-тесты для валидации

**Критерий завершения:** `go test ./shared/eventbus/...` — зелёный

---

### Этап 2: Neo4j методы (2-3 дня)

**Файлы:**
- `services/semantic-memory/semanticmemory/neo4j.go`

**Новые методы:**
- [ ] 2.1 `CreateRelation(fromID, toID, relType string, directed bool, metadata map[string]any) error`
- [ ] 2.2 `EntityExists(entityID string) (bool, error)`
- [ ] 2.3 `EnsureEntity(entityID, entityType, worldID string, payload map[string]any) error`
- [ ] 2.4 Обновить `createIndexes()` — индексы для новых типов связей

**Пример реализации 2.1:**
```go
func (n *Neo4jClient) CreateRelation(fromID, toID, relType string, directed bool, metadata map[string]any) error {
    session := n.driver.NewSession(neo4j.SessionConfig{DatabaseName: "neo4j"})
    defer session.Close()
    
    arrow := "->"
    if !directed {
        arrow = "-"
    }
    
    query := fmt.Sprintf(`
        MATCH (a {id: $from_id})
        MATCH (b {id: $to_id})
        MERGE (a)-[r:%s]%s(b)
        SET r += $metadata
        RETURN r
    `, relType, arrow)
    
    _, err := session.WriteTransaction(func(tx neo4j.Transaction) (any, error) {
        result, runErr := tx.Run(query, map[string]any{
            "from_id":  fromID,
            "to_id":    toID,
            "metadata": metadata,
        })
        if runErr != nil {
            return nil, runErr
        }
        return nil, result.Consume()
    })
    return err
}
```

**Пример реализации 2.3:**
```go
func (n *Neo4jClient) EnsureEntity(entityID, entityType, worldID string, payload map[string]any) error {
    exists, err := n.EntityExists(entityID)
    if err != nil {
        return err
    }
    if exists {
        return nil // уже существует
    }
    
    // Создаём сущность-заглушку
    if payload == nil {
        payload = make(map[string]any)
    }
    payload["stub"] = true
    payload["world_id"] = worldID
    
    return n.UpsertEntity(entityID, entityType, payload)
}
```

**Критерий завершения:** Интеграционные тесты с Neo4j test-container проходят

---

### Этап 3: semantic-memory логика (2-3 дня)

**Файлы:**
- `services/semantic-memory/semanticmemory/indexer.go`
- `services/semantic-memory/semanticmemory/neo4j.go`

**Изменения:**
- [ ] 3.1 Переписать `saveEventToNeo4j()` — приоритет явным relations
- [ ] 3.2 Реализовать `applyExplicitRelations(ev Event) error`
- [ ] 3.3 Реализовать `ensureEntitiesFromRelations(relations []Relation, worldID string)`
- [ ] 3.4 Сохранить fallback на старую логику (обратная совместимость)
- [ ] 3.5 Добавить метрики: `relations_explicit_count`, `relations_fallback_count`
- [ ] 3.6 Написать интеграционные тесты

**Пример реализации 3.1:**
```go
func (i *Indexer) saveEventToNeo4j(ctx context.Context, ev eventbus.Event) error {
    // 1. Сохраняем узел события
    if err := i.neo4j.SaveEventNode(ev); err != nil {
        return fmt.Errorf("save event node: %w", err)
    }
    
    // 2. ✨ Если есть явные связи — применяем их
    if len(ev.Relations) > 0 {
        return i.applyExplicitRelations(ev)
    }
    
    // 3. Fallback: старая логика для обратной совместимости
    return i.neo4j.LinkEventToEntities(ev.EventID, ev.Payload)
}

func (i *Indexer) applyExplicitRelations(ev eventbus.Event) error {
    // Создаём сущности если их нет
    seen := make(map[string]bool)
    for _, rel := range ev.Relations {
        if !seen[rel.From] {
            i.ensureEntityFromRelation(rel.From, ev.WorldID)
            seen[rel.From] = true
        }
        if !seen[rel.To] {
            i.ensureEntityFromRelation(rel.To, ev.WorldID)
            seen[rel.To] = true
        }
        
        // Создаём связь
        if err := i.neo4j.CreateRelation(
            rel.From, rel.To, rel.Type, rel.Directed, rel.Metadata,
        ); err != nil {
            log.Printf("Failed to create relation %s->%s: %v", rel.From, rel.To, err)
        }
    }
    return nil
}

func (i *Indexer) ensureEntityFromRelation(entityID, worldID string) {
    exists, err := i.neo4j.EntityExists(entityID)
    if err != nil || exists {
        return
    }
    
    // Извлекаем тип из ID (format: "type:id")
    parts := strings.SplitN(entityID, ":", 2)
    entityType := "unknown"
    if len(parts) > 1 {
        entityType = parts[0]
    }
    
    i.neo4j.EnsureEntity(entityID, entityType, worldID, map[string]any{"stub": true})
}
```

**Критерий завершения:** 
- Старые события (без relations) обрабатываются как раньше
- Новые события (с relations) создают семантические связи
- Unit + интеграционные тесты проходят

---

### Этап 4: Oracle/GM генерация relations (3-5 дней)

**Файлы:**
- `services/narrative-orchestrator/narrativeorchestrator/oracle.go`
- `services/world-generator/worldgenerator/generator.go`
- `services/world-generator/worldgenerator/archivist.go`

**Изменения:**
- [ ] 4.1 Обновить `narrative-orchestrator` для генерации relations при `player.action`
- [ ] 4.2 Обновить `world-generator` для генерации relations при создании мира/регионов
- [ ] 4.3 Добавить `ValidateEventRelations()` перед публикацией
- [ ] 4.4 Написать примеры генерации для 5 типов событий
- [ ] 4.5 Интеграционные тесты: событие → Neo4j → проверка графа

**Пример 4.1:**
```go
func (o *Oracle) PublishPlayerAction(playerID, action, targetID, regionID string) {
    event := eventbus.NewEvent(
        "player.action",
        "narrative-orchestrator",
        o.worldID,
        map[string]any{
            "entity": map[string]any{"id": "player:" + playerID, "type": "player"},
            "action": action,
            "target": map[string]any{"id": "item:" + targetID, "type": "item"},
        },
    )
    
    event.Relations = []eventbus.Relation{
        {
            From:     "player:" + playerID,
            To:       "item:" + targetID,
            Type:     eventbus.RelActedOn,
            Directed: true,
            Metadata: map[string]any{"action": action},
        },
        {
            From:     "player:" + playerID,
            To:       "region:" + regionID,
            Type:     eventbus.RelLocatedIn,
            Directed: true,
        },
    }
    
    // Валидация перед публикацией
    if err := eventbus.ValidateEventRelations(event); err != nil {
        log.Printf("Invalid relations: %v", err)
        return
    }
    
    o.bus.Publish(context.TODO(), eventbus.TopicPlayerEvents, event)
}
```

**Пример 4.2 (world-generator):**
```go
func (g *Generator) PublishWorldCreated(worldID string, worldData map[string]any) {
    event := eventbus.NewEvent(
        "world.created",
        "world-generator",
        worldID,
        worldData,
    )
    
    // Создаём связи между миром и регионами
    var relations []eventbus.Relation
    if regions, ok := worldData["regions"].([]any); ok {
        for _, r := range regions {
            if region, ok := r.(map[string]any); ok {
                regionID, _ := region["id"].(string)
                relations = append(relations, eventbus.Relation{
                    From:     "world:" + worldID,
                    To:       "region:" + regionID,
                    Type:     eventbus.RelContains,
                    Directed: true,
                })
            }
        }
    }
    
    event.Relations = relations
    g.bus.Publish(context.TODO(), eventbus.TopicWorldEvents, event)
}
```

**Критерий завершения:** 
- 5 типов событий генерируют relations
- Граф в Neo4j содержит семантические связи
- Валидация отлавливает некорректные relations

---

### Этап 5: Тестирование и мониторинг (1-2 недели)

**Задачи:**
- [ ] 5.1 Создать staging-окружение с тестовыми данными
- [ ] 5.2 Написать скрипт миграции: сравнение графов до/после
- [ ] 5.3 Добавить Prometheus-метрики:
  - `semantic_memory_relations_explicit_total` (counter)
  - `semantic_memory_relations_fallback_total` (counter)
  - `semantic_memory_entities_auto_created_total` (counter)
  - `semantic_memory_relation_validation_errors_total` (counter)
- [ ] 5.4 Настроить алерты на ошибки валидации
- [ ] 5.5 Нагрузочное тестирование: 1000 events/sec
- [ ] 5.6 Документация: обновить `Docs/services/semantic-memory.md`

**Метрики успеха:**
| Метрика | Цель | Как измерить |
|---------|------|--------------|
| Точность связей | >99% совпадение с ожидаемыми | Сравнение с golden-dataset |
| Latency P95 | ≤ текущей (≤50ms на событие) | Prometheus histogram |
| Fallback rate | <5% после полной миграции | `relations_fallback_total / total_events` |
| Auto-created entities | <10% от всех сущностей | `entities_auto_created_total` |
| Error rate | <0.1% | Алерты + логи |

---

### Этап 6: Полная миграция и удаление fallback (2-4 недели после деплоя)

**Задачи:**
- [ ] 6.1 Мигрировать все сервисы на генерацию relations
- [ ] 6.2 Убедиться что fallback rate <1%
- [ ] 6.3 Удалить `extractEntitiesFromPayload()` и `LinkEventToEntities()`
- [ ] 6.4 Обновить документацию
- [ ] 6.5 Ретроспектива: lessons learned

---

## ⚠️ Риски и митигация

| Риск | Вероятность | Влияние | Митигация |
|------|-------------|---------|-----------|
| Обратная совместимость | Высокая | Среднее | Fallback на старую логику, постепенная миграция |
| Дублирование связей | Средняя | Низкое | Проверка `EntityExists` перед созданием, `MERGE` в Cypher |
| Размер события увеличивается | Низкая | Низкое | Relations опциональны, сжатие Kafka |
| Oracle генерирует некорректные relations | Средняя | Высокое | `ValidateEventRelations()` перед публикацией |
| Производительность Neo4j | Низкая | Высокое | Индексы на `id`, batch-операции при необходимости |

---

## 🧪 Тестовый план

### Unit-тесты
| Тест | Что проверяет |
|------|---------------|
| `TestValidateEventRelations_Valid` | Валидные relations проходят |
| `TestValidateEventRelations_Invalid` | Пустые from/to/type отклоняются |
| `TestApplyExplicitRelations` | Relations применяются к Neo4j |
| `TestEnsureEntityFromRelation` | Сущности-заглушки создаются |
| `TestFallbackToOldLogic` | События без relations обрабатываются как раньше |

### Интеграционные тесты
| Тест | Что проверяет |
|------|---------------|
| `TestPlayerActionCreatesGraph` | `player.action` → `[:ACTED_ON]` + `[:LOCATED_IN]` |
| `TestWorldCreatedCreatesRegions` | `world.created` → `[:CONTAINS]` регионы |
| `TestExplicitRelationsOverrideFallback` | Relations имеют приоритет над эвристикой |
| `TestBackwardCompatibility` | Старые события без relations работают |

### Нагрузочные тесты
| Тест | Цель |
|------|------|
| 1000 events/sec | Latency P95 ≤ 50ms |
| 100 relations/event | Neo4j выдерживает нагрузку |
| 1M событий | Память не утекает, индексы работают |

---

## 📚 Ссылки и ресурсы

- [Текущий neo4j.go](../services/semantic-memory/semanticmemory/neo4j.go)
- [Текущий indexer.go](../services/semantic-memory/semanticmemory/indexer.go)
- [Текущий types.go](../shared/eventbus/types.go)
- [KnowledgeBase Schema](../services/semantic-memory/docs/knowledge-base-schema.md)

---

## 📅 Timeline

| Этап | Длительность | Зависимости |
|------|--------------|-------------|
| Этап 1: Структуры | 1-2 дня | — |
| Этап 2: Neo4j методы | 2-3 дня | Этап 1 |
| Этап 3: semantic-memory | 2-3 дня | Этап 1, 2 |
| Этап 4: Oracle генерация | 3-5 дней | Этап 1, 2, 3 |
| Этап 5: Тестирование | 1-2 недели | Этап 1-4 |
| Этап 6: Миграция | 2-4 недели | Этап 5 |

**Итого:** ~6-10 недель до полной миграции

---

## ✅ Чеклист готовности к деплою

- [ ] Все unit-тесты зелёные
- [ ] Интеграционные тесты с Neo4j проходят
- [ ] Нагрузочные тесты в пределах нормы
- [ ] Метрики настроены и отображаются в Grafana
- [ ] Алерты настроены
- [ ] Документация обновлена
- [ ] Staging протестирован
- [ ] Rollback-план готов
- [ ] Команда знает о changes

---

*Создано: 2025-03-20*  
*Статус: 🟡 Планирование*  
*Ответственный: Алексей*
