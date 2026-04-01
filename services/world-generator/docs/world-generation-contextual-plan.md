# План: Контекстная и случайная генерация мира

## Задача для агента

Доработать сервис `world-generator` так, чтобы поддерживались два режима генерации мира:
1. **Contextual** — пользователь задаёт описание/предысторию мира (тему, ключевые элементы, масштаб, ограничения)
2. **Random** — полностью случайная генерация без привязки к какой-либо конкретной тематике

---

## Контекст: текущее состояние

### Файлы, которые нужно изменить

| Файл | Что делать |
|------|-----------|
| `services/world-generator/worldgenerator/generator.go` | Основной рефакторинг: новый парсинг payload, двухэтапная генерация, новые структуры |
| `services/world-generator/worldgenerator/oracle.go` | Новые промпт-билдеры, переход на `CallStructuredJSON` |
| `services/world-generator/worldgenerator/generator_test.go` | Новые тесты для обоих режимов |

### Файлы только для чтения (контекст)

| Файл | Зачем читать |
|------|-------------|
| `shared/oracle/client.go` | Понять API Oracle-клиента: `CallStructuredJSON(ctx, systemPrompt, userPrompt)` — это рекомендуемый метод для JSON-ответов |
| `shared/eventbus/eventbus.go` | Понять структуру `Event` и доступные топики |
| `services/world-generator/cmd/main.go` | Entry point — не трогать |

### Текущий flow

1. Событие `world.generation.requested` приходит в `HandleEvent`
2. Из payload извлекается `seed` (string) и `constraints` (map)
3. Создаётся world entity через `entity.created` event
4. Вызывается `generateEnhancedWorldDetails` — один hardcoded промпт через `client.Call()` (устаревший метод)
5. Промпт всегда генерирует культивационную онтологию с фиксированным набором (3-5 регионов, 2-4 водоёма, 2-4 города)
6. Результат парсится в `WorldGeography`, создаются entity-события для регионов, водоёмов, городов
7. Публикуется `world.generated` событие

---

## Шаг 1: Новые структуры данных

### В `generator.go` — добавить новые типы

```go
// WorldGenerationRequest — структура запроса из payload события world.generation.requested
type WorldGenerationRequest struct {
    Seed        string                 `json:"seed"`                   // обязательное
    Mode        string                 `json:"mode"`                   // "contextual" | "random"; default "random"
    UserContext *UserWorldContext       `json:"user_context,omitempty"` // заполняется только для mode="contextual"
    Constraints map[string]interface{} `json:"constraints,omitempty"`
}

// UserWorldContext — пользовательское описание желаемого мира
type UserWorldContext struct {
    Description  string   `json:"description"`            // свободное описание: "Мир культивации с несколькими континентами"
    Theme        string   `json:"theme,omitempty"`        // "cultivation", "steampunk", "dark_fantasy", "sci-fi", "mythology", etc.
    KeyElements  []string `json:"key_elements,omitempty"` // ["континенты", "секты", "духовная энергия", "древние артефакты"]
    Scale        string   `json:"scale,omitempty"`        // "small" | "medium" | "large"; default "medium"
    Restrictions []string `json:"restrictions,omitempty"` // чего НЕ должно быть: ["нет магии огня", "нет драконов"]
}

// WorldConcept — промежуточный результат первого этапа генерации (концепция мира)
type WorldConcept struct {
    Core         string   `json:"core"`          // ядро мира (2-3 предложения)
    Theme        string   `json:"theme"`         // определённая тема
    Era          string   `json:"era"`           // эпоха / временной период
    UniqueTraits []string `json:"unique_traits"` // 3-5 уникальных черт этого мира
    Scale        string   `json:"scale"`         // итоговый масштаб
}
```

### Модифицировать существующую `WorldOntology` — сделать универсальной

```go
// WorldOntology — система силы/прогрессии мира (не обязательно культивация)
type WorldOntology struct {
    System    string   `json:"system"`    // тип системы: "cultivation", "magic", "technology", "divine", "nature" и т.д.
    Carriers  []string `json:"carriers"`  // носители силы (ци, мана, эфир, нанороботы...)
    Paths     []string `json:"paths"`     // пути развития
    Forbidden []string `json:"forbidden"` // запреты / табу
    Hierarchy []string `json:"hierarchy"` // уровни/ранги прогрессии (опционально)
}
```

---

## Шаг 2: Рефакторинг `HandleEvent` в `generator.go`

Заменить текущую реализацию:

```go
func (wg *WorldGenerator) HandleEvent(ev eventbus.Event) {
    if ev.EventType != "world.generation.requested" {
        return
    }

    // 1. Парсинг запроса
    request, err := parseGenerationRequest(ev.Payload)
    if err != nil {
        log.Printf("Invalid world generation request: %v", err)
        return
    }

    log.Printf("Starting world generation: seed=%s, mode=%s", request.Seed, request.Mode)

    // 2. Генерация концепции (этап A)
    concept, err := wg.generateWorldConcept(ctx, request)
    if err != nil {
        log.Printf("World concept generation failed: %v", err)
        return
    }

    // 3. Создание world entity (теперь с концепцией)
    worldID := "world-" + uuid.New().String()[:8]
    wg.publishWorldCreated(ctx, worldID, request, concept)

    // 4. Генерация деталей (этап B)
    geography, err := wg.generateWorldDetails(ctx, worldID, concept, request.getScale())
    if err != nil {
        log.Printf("World details generation failed: %v", err)
        return
    }

    // 5. Создание geographic entities
    wg.createGeographicEntities(ctx, worldID, geography)

    // 6. Финальное событие
    wg.publishWorldGenerated(ctx, worldID, request, concept)

    log.Printf("World %s generated successfully (mode=%s, theme=%s)", worldID, request.Mode, concept.Theme)
}
```

### Вспомогательные функции в `generator.go`

```go
// parseGenerationRequest парсит payload события в структурированный запрос
func parseGenerationRequest(payload map[string]interface{}) (*WorldGenerationRequest, error) {
    // Сериализовать payload в JSON, затем десериализовать в WorldGenerationRequest
    // Валидация: seed обязателен
    // Если mode пустой — установить "random"
    // Если mode="contextual" и user_context=nil — вернуть ошибку
    // Если scale пустой — установить "medium"
}

// getScale возвращает масштаб из запроса (с дефолтом "medium")
func (r *WorldGenerationRequest) getScale() string {
    if r.UserContext != nil && r.UserContext.Scale != "" {
        return r.UserContext.Scale
    }
    return "medium"
}

// scaleParams возвращает параметры количества элементов по масштабу
func scaleParams(scale string) (minRegions, maxRegions, minWater, maxWater, minCities, maxCities int) {
    switch scale {
    case "small":
        return 2, 3, 1, 2, 1, 2
    case "large":
        return 5, 8, 4, 7, 4, 8
    default: // "medium"
        return 3, 5, 2, 4, 2, 4
    }
}

// publishWorldCreated публикует entity.created для мира — обновить payload
func (wg *WorldGenerator) publishWorldCreated(ctx context.Context, worldID string, req *WorldGenerationRequest, concept *WorldConcept) {
    // В payload теперь включаем:
    // - seed, plan (0), constraints (как раньше)
    // - mode, theme, core, era, unique_traits (новое)
}

// publishWorldGenerated публикует финальное событие world.generated
func (wg *WorldGenerator) publishWorldGenerated(ctx context.Context, worldID string, req *WorldGenerationRequest, concept *WorldConcept) {
    // В payload включаем:
    // - world_id, seed (как раньше)
    // - mode, theme (новое — для downstream-сервисов)
}
```

---

## Шаг 3: Новые промпт-билдеры в `oracle.go`

### Полностью переработать `oracle.go`

Убрать прямые вызовы `client.Call()`. Использовать `client.CallStructuredJSON()` для гарантированного JSON-ответа.

```go
// generateWorldConcept — ЭТАП A: генерация концепции мира
func (wg *WorldGenerator) generateWorldConcept(ctx context.Context, req *WorldGenerationRequest) (*WorldConcept, error) {
    systemPrompt, userPrompt := buildConceptPrompts(req)

    client := oracle.NewClient()
    var concept WorldConcept
    err := client.CallAndUnmarshal(ctx, func() (string, error) {
        return client.CallStructuredJSON(ctx, systemPrompt, userPrompt)
    }, &concept)
    if err != nil {
        return nil, fmt.Errorf("concept generation failed: %w", err)
    }

    // Если scale не задан в концепции — унаследовать из запроса
    if concept.Scale == "" {
        concept.Scale = req.getScale()
    }

    return &concept, nil
}

// generateWorldDetails — ЭТАП B: детализация географии и онтологии на основе концепции
func (wg *WorldGenerator) generateWorldDetails(ctx context.Context, worldID string, concept *WorldConcept, scale string) (*WorldGeography, error) {
    systemPrompt, userPrompt := buildDetailsPrompts(concept, scale)

    client := oracle.NewClient()
    var geography WorldGeography
    err := client.CallAndUnmarshal(ctx, func() (string, error) {
        return client.CallStructuredJSON(ctx, systemPrompt, userPrompt)
    }, &geography)
    if err != nil {
        return nil, fmt.Errorf("world details generation failed: %w", err)
    }

    return &geography, nil
}
```

### Промпт-билдеры

```go
// buildConceptPrompts формирует system и user промпты для этапа A (концепция)
func buildConceptPrompts(req *WorldGenerationRequest) (systemPrompt, userPrompt string) {
    systemPrompt = `Ты — Демиург, создатель миров. Твоя задача — создать уникальную концепцию мира.
Отвечай строго в формате JSON без пояснений.

Формат ответа:
{
  "core": "описание ядра мира (2-3 предложения)",
  "theme": "основная тема мира",
  "era": "эпоха или временной период",
  "unique_traits": ["черта 1", "черта 2", "черта 3"],
  "scale": "small|medium|large"
}`

    if req.Mode == "contextual" && req.UserContext != nil {
        // Контекстный режим — включаем описание пользователя
        userPrompt = fmt.Sprintf(`Создай концепцию мира на основе следующего описания.

Семя мира: "%s"
Описание: %s
Тема: %s
Ключевые элементы: %s
Масштаб: %s
Ограничения (чего НЕ должно быть): %s

Концепция должна уважать все указанные элементы и ограничения, но может творчески дополнять и расширять описание.`,
            req.Seed,
            req.UserContext.Description,
            defaultIfEmpty(req.UserContext.Theme, "на усмотрение Демиурга"),
            strings.Join(req.UserContext.KeyElements, ", "),
            defaultIfEmpty(req.UserContext.Scale, "medium"),
            strings.Join(req.UserContext.Restrictions, ", "),
        )
    } else {
        // Случайный режим
        userPrompt = fmt.Sprintf(`Создай полностью оригинальную и неожиданную концепцию мира.

Семя мира: "%s"

Мир может быть любой тематики — фэнтези, научная фантастика, мифология, стимпанк, пост-апокалипсис, культивация, или что-то совершенно необычное. Удиви.
Масштаб: medium.`, req.Seed)
    }

    return systemPrompt, userPrompt
}

// buildDetailsPrompts формирует system и user промпты для этапа B (детализация)
func buildDetailsPrompts(concept *WorldConcept, scale string) (systemPrompt, userPrompt string) {
    minR, maxR, minW, maxW, minC, maxC := scaleParams(scale)

    systemPrompt = fmt.Sprintf(`Ты — Демиург, детализирующий мир.

Концепция мира:
- Ядро: %s
- Тема: %s
- Эпоха: %s
- Уникальные черты: %s

Все детали должны быть внутренне согласованы с этой концепцией. Названия, биомы, города — всё должно отражать тему и эпоху мира.

Отвечай строго в формате JSON без пояснений.`,
        concept.Core,
        concept.Theme,
        concept.Era,
        strings.Join(concept.UniqueTraits, "; "),
    )

    userPrompt = fmt.Sprintf(`Сгенерируй полную детализацию мира.

Требования:
1. Онтология (система силы/прогрессии, соответствующая теме мира):
   - system: тип системы (cultivation, magic, technology, divine, nature и т.д.)
   - carriers: носители силы (2-4 шт.)
   - paths: пути развития (3-5 шт.)
   - forbidden: запреты/табу (2-3 шт.)
   - hierarchy: уровни прогрессии (4-7 шт.)

2. География:
   - %d-%d регионов с уникальными биомами, координатами и размером
   - %d-%d водных объектов (реки, моря, озёра) с координатами и размером
   - %d-%d городов с населением, типом (major/minor) и привязкой к региону

3. Мифология: краткий основополагающий миф мира (3-5 предложений)

Формат JSON:
{
  "core": "string (повторить ядро мира)",
  "ontology": {
    "system": "string",
    "carriers": ["string"],
    "paths": ["string"],
    "forbidden": ["string"],
    "hierarchy": ["string"]
  },
  "geography": {
    "regions": [{"name": "string", "biome": "string", "coordinates": {"x": 0.0, "y": 0.0}, "size": 0.0}],
    "water_bodies": [{"name": "string", "type": "river|sea|lake", "coordinates": {"x": 0.0, "y": 0.0}, "size": 0.0}],
    "cities": [{"name": "string", "population": 0, "type": "major|minor", "location": {"region": "string", "coordinates": {"x": 0.0, "y": 0.0}}}]
  },
  "mythology": "string"
}`, minR, maxR, minW, maxW, minC, maxC)

    return systemPrompt, userPrompt
}

// defaultIfEmpty возвращает значение по умолчанию, если строка пустая
func defaultIfEmpty(s, def string) string {
    if s == "" {
        return def
    }
    return s
}
```

---

## Шаг 4: Обновить существующие функции

### `createGeographicEntities` — без изменений
Функция уже работает корректно, принимает `WorldGeography` и создаёт entity-события.

### `publishGeographyGeneratedEvent` — без изменений
Работает корректно.

### Удалить `generateEnhancedWorldDetails`
Заменена двумя новыми функциями: `generateWorldConcept` + `generateWorldDetails`.

---

## Шаг 5: Тесты в `generator_test.go`

### Обязательные тесты

```go
// 1. Парсинг запроса — оба режима
func TestParseGenerationRequest_Random(t *testing.T) {
    // payload с seed и без mode → mode="random", user_context=nil
}

func TestParseGenerationRequest_Contextual(t *testing.T) {
    // payload с seed, mode="contextual", user_context заполнен
}

func TestParseGenerationRequest_MissingSeeed(t *testing.T) {
    // payload без seed → ошибка
}

func TestParseGenerationRequest_ContextualWithoutContext(t *testing.T) {
    // mode="contextual" но user_context=nil → ошибка
}

func TestParseGenerationRequest_DefaultScale(t *testing.T) {
    // scale пустой → "medium"
}

// 2. Промпт-билдеры
func TestBuildConceptPrompts_Random(t *testing.T) {
    // Проверить что userPrompt НЕ содержит description/key_elements
    // Проверить что userPrompt содержит seed
    // Проверить что systemPrompt содержит JSON-формат
}

func TestBuildConceptPrompts_Contextual(t *testing.T) {
    // Проверить что userPrompt содержит description, theme, key_elements, restrictions
    // Проверить что seed присутствует
}

func TestBuildDetailsPrompts_Scale(t *testing.T) {
    // Проверить что для "small" промпт содержит "2-3 регионов"
    // Проверить что для "large" промпт содержит "5-8 регионов"
}

// 3. Scale params
func TestScaleParams(t *testing.T) {
    // Проверить значения для small, medium, large
}

// 4. Структуры данных
func TestWorldGenerationRequestDefaults(t *testing.T) {
    // getScale() с nil UserContext → "medium"
    // getScale() с заполненным UserContext.Scale → заданное значение
}
```

---

## Шаг 6: Примеры событий (для валидации)

### Пример 1: Случайная генерация

```json
{
  "event_type": "world.generation.requested",
  "payload": {
    "seed": "Eternal Void"
  }
}
```

Ожидание: `mode` автоматически `"random"`, генерируется произвольная концепция.

### Пример 2: Контекстная генерация

```json
{
  "event_type": "world.generation.requested",
  "payload": {
    "seed": "Jade Heavens",
    "mode": "contextual",
    "user_context": {
      "description": "Мир культивации с несколькими континентами, где секты борются за ресурсы духовной энергии",
      "theme": "cultivation",
      "key_elements": ["континенты", "секты", "духовная энергия", "древние руины"],
      "scale": "large",
      "restrictions": ["нет технологий", "нет огнестрельного оружия"]
    }
  }
}
```

Ожидание: концепция строится вокруг культивации, `large` масштаб (5-8 регионов), онтология system="cultivation".

### Пример 3: Контекстная генерация с минимальным контекстом

```json
{
  "event_type": "world.generation.requested",
  "payload": {
    "seed": "Clockwork Dawn",
    "mode": "contextual",
    "user_context": {
      "description": "Стимпанк мир с летающими городами"
    }
  }
}
```

Ожидание: тема определяется как "steampunk", масштаб `"medium"` по умолчанию, онтология system="technology" или подобное.

---

## Требования к реализации

1. **Обратная совместимость**: старый формат payload (только `seed` + `constraints`) должен работать как `mode="random"`
2. **Не трогать**: `cmd/main.go`, `service.go`, `archivist.go`, `schema.go`, shared-пакеты
3. **Oracle client**: использовать `CallStructuredJSON` вместо `Call` для обоих этапов генерации. НЕ создавать новый Oracle-клиент на каждый вызов — создать один в `NewWorldGenerator` и хранить в структуре (или передавать)
4. **Промпты на русском**: все промпты для Oracle писать на русском (как в текущей реализации)
5. **Логирование**: добавить логи для каждого этапа с указанием mode, seed, theme
6. **Тесты**: все новые публичные функции должны быть покрыты unit-тестами. Тесты не должны требовать реального Oracle — тестируем парсинг, промпт-билдеры, scale params, валидацию
7. **Go workspace**: после изменений выполнить `go work sync` и убедиться что `go build ./...` и `go test ./...` проходят в директории `services/world-generator/`

---

## Порядок выполнения

1. Добавить новые структуры (`WorldGenerationRequest`, `UserWorldContext`, `WorldConcept`, обновить `WorldOntology`)
2. Реализовать `parseGenerationRequest` с валидацией
3. Реализовать промпт-билдеры (`buildConceptPrompts`, `buildDetailsPrompts`, `scaleParams`, `defaultIfEmpty`)
4. Реализовать `generateWorldConcept` и `generateWorldDetails`
5. Рефакторить `HandleEvent` — новый flow с двумя этапами
6. Обновить `publishWorldCreated` и `publishWorldGenerated` — расширенный payload
7. Удалить `generateEnhancedWorldDetails` (заменена)
8. Написать тесты
9. Проверить компиляцию и тесты: `cd services/world-generator && go test ./...`
