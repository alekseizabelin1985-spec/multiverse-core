// Пример использования универсального пакета jsonpath
// Файл: shared/jsonpath/examples/usage_examples.go

package main

import (
	"encoding/json"
	"fmt"
	"log"

	"multiverse-core.io/shared/jsonpath"
)

func main() {
	fmt.Println("=== Примеры использования пакета jsonpath ===")
	fmt.Println()

	// === Пример 1: Работа с конфигурацией из YAML/JSON ===
	fmt.Println("1. Конфигурация приложения:")

	// Типичный конфиг после yaml.Unmarshal / json.Unmarshal
	configJSON := `{
		"app": {
			"name": "Multiverse-Core",
			"version": "1.0.0",
			"debug": true
		},
		"database": {
			"host": "localhost",
			"port": 5432,
			"pool_size": 20,
			"ssl": false
		},
		"features": {
			"ai_enabled": true,
			"max_players": 1000,
			"regions": ["europe", "asia", "americas"]
		}
	}`

	var config map[string]any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		log.Fatalf("Unmarshal failed: %v", err)
	}

	acc := jsonpath.New(config)

	// Извлечение значений разных типов
	appName, _ := acc.GetString("app.name")
	appVer, _ := acc.GetString("app.version")
	dbPort, _ := acc.GetInt("database.port")
	poolSize, _ := acc.GetInt("database.pool_size")
	aiEnabled, _ := acc.GetBool("features.ai_enabled")
	maxPlayers, _ := acc.GetInt("features.max_players")
	regions, _ := acc.GetSlice("features.regions")

	debug, _ := acc.GetBool("app.debug")
	dbHost, _ := acc.GetString("database.host")
	ssl, _ := acc.GetBool("database.ssl")
	fmt.Printf("   App: %s v%s (debug: %v)\n", appName, appVer, debug)
	fmt.Printf("   DB: %s:%d (pool: %d, ssl: %v)\n", dbHost, dbPort, poolSize, ssl)
	fmt.Printf("   Features: AI=%v, max_players=%d, regions=%v\n",
		aiEnabled, maxPlayers, regions)
	fmt.Println()

	// === Пример 2: API-ответ с вложенной структурой ===
	fmt.Println("2. Обработка API-ответа:")

	apiResponse := map[string]any{
		"meta": map[string]any{
			"success": true,
			"code":    200,
			"message": "OK",
		},
		"data": map[string]any{
			"user": map[string]any{
				"id":       "usr_12345",
				"username": "alex_dev",
				"profile": map[string]any{
					"email":    "alex@example.com",
					"verified": true,
					"stats": map[string]any{
						"level": 42,
						"xp":    12500.5,
						"achievements": []any{
							map[string]any{"id": "ach_1", "name": "First Steps"},
							map[string]any{"id": "ach_2", "name": "Explorer"},
						},
					},
				},
			},
		},
	}

	respAcc := jsonpath.New(apiResponse)

	// Проверка успеха запроса
	if success, _ := respAcc.GetBool("meta.success"); success {
		// userID, _ := respAcc.GetString("data.user.id")
		username, _ := respAcc.GetString("data.user.username")
		email, _ := respAcc.GetString("data.user.profile.email")
		level, _ := respAcc.GetInt("data.user.profile.stats.level")
		xp, _ := respAcc.GetFloat("data.user.profile.stats.xp")

		fmt.Printf("   ✓ User: %s (%s) — Level %d, XP %.1f\n", username, email, level, xp)

		achievements, _ := respAcc.GetSlice("data.user.profile.stats.achievements")
		fmt.Printf("   ✓ Achievements: %d unlocked\n", len(achievements))

		// Доступ к элементу массива по индексу
		achName, _ := respAcc.GetString("data.user.profile.stats.achievements[0].name")
		fmt.Printf("   ✓ First achievement: %s\n", achName)
	}
	fmt.Println()

	// === Пример 3: Валидация входных данных ===
	fmt.Println("3. Валидация входных данных:")

	validateUser := func(data map[string]any) error {
		acc := jsonpath.New(data)

		// Обязательные поля
		if !acc.Has("user.id") {
			return fmt.Errorf("required: user.id")
		}
		if id, _ := acc.GetString("user.id"); id == "" {
			return fmt.Errorf("user.id cannot be empty")
		}

		// Типы и ограничения
		if age, ok := acc.GetInt("user.age"); ok {
			if age < 18 {
				return fmt.Errorf("user must be 18+, got %d", age)
			}
		}

		// Проверка вложенных структур
		if acc.Has("user.profile") {
			if email, ok := acc.GetString("user.profile.email"); ok && email != "" {
				// Простая валидация email
				if len(email) < 5 || !contains(email, "@") {
					return fmt.Errorf("invalid email format: %s", email)
				}
			}
		}

		return nil
	}

	validData := map[string]any{
		"user": map[string]any{
			"id":  "u_123",
			"age": 25,
			"profile": map[string]any{
				"email": "test@example.com",
			},
		},
	}

	if err := validateUser(validData); err != nil {
		fmt.Printf("   ✗ Validation failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Data is valid\n")
	}
	fmt.Println()

	// === Пример 4: Отладка и интроспекция структуры ===
	fmt.Println("4. Интроспекция структуры данных (GetAllPaths):")

	sampleData := map[string]any{
		"entity": map[string]any{
			"id":   "e_1",
			"type": "player",
			"stats": map[string]any{
				"hp": 100,
				"mp": 50,
			},
		},
		"active": true,
	}

	inspectAcc := jsonpath.New(sampleData)
	fmt.Println("   Доступные пути:")

	paths := inspectAcc.GetAllPaths()
	for _, path := range paths {
		// Показываем тип значения для каждого пути
		if val, ok := inspectAcc.GetAny(path); ok {
			fmt.Printf("     • %s → %T\n", path, val)
		}
	}
	fmt.Println()

	// === Пример 5: Безопасная модификация через Clone ===
	fmt.Println("5. Безопасная модификация (Clone):")

	original := map[string]any{
		"session": map[string]any{
			"user_id": "u_123",
			"token":   "secret_token",
		},
	}

	origAcc := jsonpath.New(original)
	clonedAcc := origAcc.Clone()

	// Модифицируем клон — оригинал не меняется
	clonedAcc.Set("session.token", "new_token")
	clonedAcc.Set("session.expires_at", "2024-12-31T23:59:59Z")

	origToken, _ := origAcc.GetString("session.token")
	cloneToken, _ := clonedAcc.GetString("session.token")

	fmt.Printf("   Original token: %s (не изменён)\n", origToken)
	fmt.Printf("   Cloned token:   %s (изменён)\n", cloneToken)
	expiresAt, _ := clonedAcc.GetString("session.expires_at")
	fmt.Printf("   New field in clone: %s\n", expiresAt)
	fmt.Println()

	// === Пример 6: Работа с массивами и индексами ===
	fmt.Println("6. Доступ к элементам массивов по индексу:")

	playersData := map[string]any{
		"party": []any{
			map[string]any{"name": "Warrior", "class": "tank", "hp": 150},
			map[string]any{"name": "Mage", "class": "caster", "hp": 80},
			map[string]any{"name": "Rogue", "class": "dps", "hp": 100},
		},
	}

	partyAcc := jsonpath.New(playersData)

	// Доступ по индексу
	for i := 0; i < 3; i++ {
		name, _ := partyAcc.GetString(fmt.Sprintf("party[%d].name", i))
		class, _ := partyAcc.GetString(fmt.Sprintf("party[%d].class", i))
		hp, _ := partyAcc.GetInt(fmt.Sprintf("party[%d].hp", i))
		fmt.Printf("   [%d] %s (%s) — HP: %d\n", i, name, class, hp)
	}
	fmt.Println()

	fmt.Println("=== Все примеры завершены ===")
}

// contains — простая утилита для примера валидации
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
