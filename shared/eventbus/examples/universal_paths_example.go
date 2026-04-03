// Пример использования универсального механизма путей в событиях Multiverse-Core
// Файл: shared/eventbus/examples/universal_paths_example.go

package main

import (
	"encoding/json"
	"fmt"

	"multiverse-core.io/shared/eventbus"
)

func main() {
	fmt.Println("=== Пример: Универсальный механизм путей в событиях ===\n")

	// === 1. Создание события с полной иерархической структурой ===

	// Используем типобезопасный builder
	payload := eventbus.NewEventPayload().
		WithEntity("player-123", "player", "Вася").
		WithTarget("region-456", "region", "Темный лес").
		WithWorld("world-789").
		WithScope("group-abc", "group") // Новый метод!

	// Добавляем кастомные поля через dot-notation
	eventbus.SetNested(payload.GetCustom(), "weather.change.to", "шторм")
	eventbus.SetNested(payload.GetCustom(), "weather.change.in.region.id", "region-456")
	eventbus.SetNested(payload.GetCustom(), "player.stats.hp.current", 85)
	eventbus.SetNested(payload.GetCustom(), "player.stats.hp.max", 100)

	// Создаём событие
	event := eventbus.NewStructuredEvent(
		"player.entered_region",
		"entity-actor",
		"world-789",
		payload,
	)

	// Сериализуем в JSON для наглядности
	jsonBytes, _ := json.MarshalIndent(event.Payload, "", "  ")
	fmt.Println("1. JSON представления события (payload):")
	fmt.Println(string(jsonBytes))
	fmt.Println()

	// === 2. Извлечение данных через готовые функции ===

	// Извлечение сущности (поддержка новой и старой структуры)
	entity := eventbus.ExtractEntityID(event.Payload)
	if entity != nil {
		fmt.Printf("2. ExtractEntityID:\n")
		fmt.Printf("   ID: %s, Type: %s, Name: %s, World: %s\n",
			entity.ID, entity.Type, entity.Name, entity.World)
	}

	// Извлечение скоупа (новый формат: scope: {id, type})
	scope := eventbus.ExtractScope(event.Payload)
	if scope != nil {
		fmt.Printf("3. ExtractScope:\n")
		fmt.Printf("   ID: %s, Type: %s (solo/group/city/region/quest)\n",
			scope.ID, scope.Type)
	}

	// Извлечение world_id
	worldID := eventbus.ExtractWorldID(event.Payload)
	fmt.Printf("4. ExtractWorldID: %s\n\n", worldID)

	// === 3. Универсальный доступ через PathAccessor ===

	fmt.Println("5. PathAccessor — универсальный доступ по dot-путям:")

	// Создаём аксессор (или используем event.Path())
	pa := event.Path()

	// Извлечение строк
	if val, ok := pa.GetString("entity.id"); ok {
		fmt.Printf("   entity.id = %q\n", val)
	}
	if val, ok := pa.GetString("scope.type"); ok {
		fmt.Printf("   scope.type = %q\n", val)
	}
	if val, ok := pa.GetString("world.id"); ok {
		fmt.Printf("   world.id = %q\n", val)
	}

	// Извлечение чисел
	if val, ok := pa.GetInt("player.stats.hp.current"); ok {
		fmt.Printf("   player.stats.hp.current = %d\n", val)
	}
	if val, ok := pa.GetFloat("player.stats.hp.max"); ok {
		fmt.Printf("   player.stats.hp.max = %.1f\n", val)
	}

	// Извлечение булевых значений
	// (в нашем примере нет, но демонстрируем синтаксис)
	// if val, ok := pa.GetBool("entity.active"); ok { ... }

	// Извлечение map
	if metadata, ok := pa.GetMap("entity"); ok {
		fmt.Printf("   entity = map with keys: ")
		for k := range metadata {
			fmt.Printf("%s ", k)
		}
		fmt.Println()
	}

	// Проверка существования без извлечения (быстрая)
	if pa.Has("weather.change.to") {
		fmt.Printf("   weather.change.to exists ✓\n")
	}

	fmt.Println()

	// === 4. Отладка: все доступные пути в данных ===

	fmt.Println("6. GetAllPaths() — все доступные пути для отладки:")
	paths := pa.GetAllPaths()
	for _, path := range paths {
		fmt.Printf("   • %s\n", path)
	}
	fmt.Println()

	// === 5. Обработчик события — пример реального использования ===

	fmt.Println("7. Пример хендлера события:")
	handlePlayerAction(event)
}

// handlePlayerAction демонстрирует типичное использование в реальном сервисе
func handlePlayerAction(event eventbus.Event) {
	// Универсальный доступ через встроенный PathAccessor
	pa := event.Path()

	// Извлекаем ключевые данные по иерархическим путям
	entityID, _ := pa.GetString("entity.id")
	entityType, _ := pa.GetString("entity.type")
	// scopeID, _ := pa.GetString("scope.id")
	scopeType, _ := pa.GetString("scope.type")
	worldID, _ := pa.GetString("world.id")

	// Кастомные поля через dot-notation
	// action, _ := pa.GetString("action")
	targetID, _ := pa.GetString("target.entity.id")

	// Метрики/статы
	hpCurrent, _ := pa.GetInt("player.stats.hp.current")
	hpMax, _ := pa.GetInt("player.stats.hp.max")

	// Логика обработки (пример)
	fmt.Printf("   [Handler] Player %s (%s) in %s scope entered target %s\n",
		entityID, entityType, scopeType, targetID)
	fmt.Printf("   [Handler] World: %s, HP: %d/%d\n", worldID, hpCurrent, hpMax)

	// Проверка специфичных условий
	if scopeType == "group" && hpCurrent < hpMax/2 {
		fmt.Printf("   [Handler] ⚠️  Low HP in group scope — may trigger healing quest\n")
	}
}

// === 8. Backward Compatibility — работа со старыми событиями ===

func demonstrateBackwardCompatibility() {
	fmt.Println("\n8. Backward Compatibility — старые форматы:")

	// Старый плоский формат
	oldPayload := map[string]any{
		"entity_id":   "npc-456",
		"entity_type": "npc",
		"entity_name": "Старейшина",
		"target_id":   "item-789",
		"world_id":    "world-123",
		"scope_id":    "city-abc",
		"scope_type":  "city",
		"action":      "trade",
	}

	// Все функции извлечения работают с ОБЕИМИ структурами!
	entity := eventbus.ExtractEntityID(oldPayload)
	scope := eventbus.ExtractScope(oldPayload)
	worldID := eventbus.ExtractWorldID(oldPayload)

	fmt.Printf("   Old format entity: %s (%s)\n", entity.ID, entity.Type)
	fmt.Printf("   Old format scope: %s (%s)\n", scope.ID, scope.Type)
	fmt.Printf("   Old format world: %s\n", worldID)

	// PathAccessor тоже работает с плоскими ключами
	pa := eventbus.NewPathAccessor(oldPayload)
	if val, ok := pa.GetString("entity_id"); ok {
		fmt.Printf("   PathAccessor can access flat keys: entity_id = %q\n", val)
	}
}
