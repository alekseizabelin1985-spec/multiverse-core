# jsonpath — Универсальный доступ к вложенным данным в Go

Пакет `jsonpath` предоставляет типобезопасный доступ к вложенным структурам данных через dot-notation пути. Работает с любыми данными в формате `map[string]any` / `[]any` — типичный результат `json.Unmarshal`.

**Не привязан к домену** — можно использовать для событий, конфигураций, API-ответов, любых JSON-подобных структур.

---

## 🚀 Быстрый старт

```go
import "multiverse-core.io/shared/jsonpath"

// Данные (например, из json.Unmarshal)
data := map[string]any{
    "entity": map[string]any{
        "id":   "player-123",
        "type": "player",
        "stats": map[string]any{
            "hp": 100,
            "mp": 50.5,
        },
        "inventory": []any{"sword", "potion"},
    },
    "active": true,
}

// Создаём аксессор
acc := jsonpath.New(data)

// Извлечение по типам
id, _ := acc.GetString("entity.id")              // "player-123"
hp, _ := acc.GetInt("entity.stats.hp")           // 100
mp, _ := acc.GetFloat("entity.stats.mp")         // 50.5
active, _ := acc.GetBool("active")               // true
inv, _ := acc.GetSlice("entity.inventory")       // []any{"sword", "potion"}
meta, _ := acc.GetMap("entity.stats")            // map[string]any{...}

// Проверка существования без извлечения
if acc.Has("entity.metadata") {
    // Обработка метаданных...
}

// Доступ к элементам массива по индексу
firstItem, _ := acc.GetString("entity.inventory[0]")  // "sword"

// Отладка: все доступные пути
for _, path := range acc.GetAllPaths() {
    fmt.Println(path)
    // entity, entity.id, entity.type, entity.stats, entity.stats.hp, ...
}
```

---

## 📦 API Reference

### Создание

```go
// New создаёт аксессор для любых данных
acc := jsonpath.New(data any) *Accessor
```

### Геттеры (типобезопасные)

| Метод | Возвращает | Пример пути |
|-------|-----------|-------------|
| `GetString(path)` | `string, bool` | `"user.email"`, `"config.api_key"` |
| `GetInt(path)` | `int, bool` | `"stats.level"`, `"retry.count"` |
| `GetFloat(path)` | `float64, bool` | `"weather.temp"`, `"metrics.accuracy"` |
| `GetBool(path)` | `bool, bool` | `"enabled"`, `"flags.admin"` |
| `GetMap(path)` | `map[string]any, bool` | `"metadata"`, `"settings"` |
| `GetSlice(path)` | `[]any, bool` | `"items"`, `"tags"` |
| `GetAny(path)` | `any, bool` | Любое значение или подструктура |

Все геттеры возвращают `(value, ok)` — проверяйте `ok` перед использованием!

### Утилиты

```go
// Has — быстрая проверка существования
if acc.Has("user.permissions.admin") {
    // ...
}

// GetAllPaths — все пути для отладки/интроспекции
paths := acc.GetAllPaths()

// Set — установка значения (модифицирует исходные данные!)
acc.Set("user.last_seen", "2024-01-15T10:30:00Z")

// Delete — удаление по пути
acc.Delete("user.temp_token")

// Clone — глубокая копия для безопасной модификации
safeAcc := acc.Clone()
safeAcc.Set("temp.value", 42) // не влияет на оригинал
```

### Поддерживаемые форматы путей

```go
// Простая вложенность
acc.GetString("a.b.c")

// Доступ к массивам по индексу
acc.GetString("items[0].name")
acc.GetInt("users[2].stats.level")

// Комбинированные
acc.GetFloat("data.nested[1].metrics[0].value")

// Ведущие точки игнорируются
acc.GetString(".entity.id")  // эквивалентно "entity.id"
```

---

## 🔧 Преобразование типов

### GetInt
Поддерживает входные типы:
- `int`, `int8-64`, `uint`, `uint8-64` → прямое преобразование
- `float32`, `float64` → округление вниз
- `string` → попытка парсинга (будущее расширение)

### GetFloat
Поддерживает:
- `float32`, `float64` → прямое
- Все целочисленные типы → преобразование к `float64`

### GetBool
Поддерживает:
- `bool` → прямое
- `string`: `"true"`, `"1"`, `"yes"` → `true`; остальные → `false`
- Числа: `0` → `false`; `!=0` → `true`

---

## 🧪 Примеры использования

### Конфигурация приложения
```go
config := loadConfig() // map[string]any из YAML/JSON
acc := jsonpath.New(config)

dbHost, _ := acc.GetString("database.host")
dbPort, _ := acc.GetInt("database.port")
debug, _ := acc.GetBool("features.debug_mode")

if !acc.Has("cache.enabled") || debug {
    // Отключить кеш в отладке
}
```

### Обработка API-ответа
```go
var response map[string]any
json.Unmarshal(apiResp, &response)
acc := jsonpath.New(response)

userID, _ := acc.GetString("data.user.id")
email, _ := acc.GetString("data.user.profile.email")
roles, _ := acc.GetSlice("data.user.permissions")

if acc.GetBool("meta.success") {
    // Успешный ответ
}
```

### Валидация входных данных
```go
func validateInput(data map[string]any) error {
    acc := jsonpath.New(data)
    
    if !acc.Has("user.id") {
        return errors.New("user.id required")
    }
    if id, _ := acc.GetString("user.id"); id == "" {
        return errors.New("user.id cannot be empty")
    }
    if age, ok := acc.GetInt("user.age"); ok && age < 18 {
        return errors.New("user must be 18+")
    }
    return nil
}
```

### Генерация схем / документация
```go
// Интроспекция структуры данных
func generateSchema(data map[string]any) map[string]string {
    acc := jsonpath.New(data)
    schema := make(map[string]string)
    
    for _, path := range acc.GetAllPaths() {
        if val, ok := acc.GetAny(path); ok {
            schema[path] = fmt.Sprintf("%T", val)
        }
    }
    return schema
}
```

---

## ⚡ Производительность

- **O(d)** для доступа по пути, где `d` — глубина вложенности
- **Has()** быстрее чем геттеры — не создаёт промежуточные значения
- **GetAllPaths()** рекурсивный обход — используйте для отладки, не в hot path
- **Clone()** создаёт глубокую копию — избегайте в циклах

---

## 🔒 Безопасность и ограничения

### Не паникует
Все методы возвращают `(value, bool)` — нет паник при:
- Несуществующих путях
- Неверных типах
- Выходе за границы массива

### Не модифицирует при чтении
Геттеры (`Get*`) только читают данные. Для изменения используйте `Set()`/`Delete()`.

### Thread-safety
- ✅ Чтение (`Get*`, `Has`) безопасно при конкурентном доступе, если данные не модифицируются
- ❌ Запись (`Set`, `Delete`) требует внешней синхронизации
- ✅ `Clone()` создаёт независимую копию для безопасной модификации

---

## 🔄 Интеграция с eventbus

Для удобства в `eventbus` добавлен алиас:

```go
// В eventbus/types.go:
func (e *Event) Path() *jsonpath.Accessor {
    return jsonpath.New(e.Payload)
}

// Использование:
func handler(event eventbus.Event) {
    pa := event.Path()  // тот же jsonpath.Accessor
    entityID, _ := pa.GetString("entity.id")
    // ...
}
```

---

## 📋 Чеклист миграции с плоских ключей

```go
// Было:
entityID := payload["entity_id"].(string)  // паник если нет или неверный тип!

// Стало:
acc := jsonpath.New(payload)
entityID, ok := acc.GetString("entity.id")  // безопасно + иерархия
if !ok {
    // fallback или ошибка
}
```

---

## 🧪 Тестирование

```bash
# Запустить тесты пакета
go test ./shared/jsonpath/... -v

# С покрытием
go test ./shared/jsonpath/... -cover

# Конкретный тест
go test ./shared/jsonpath -run TestAccessor_GetString -v
```

---

## 🤝 Contributing

1. Добавьте тест для новой функциональности
2. Обновите примеры в `examples/`
3. Документируйте публичные методы в godoc-стиле
4. Проверьте обратную совместимость

---

> 💡 **Совет**: Используйте `GetAllPaths()` при отладке новых структур данных — это быстрый способ понять, какие пути доступны для извлечения.
