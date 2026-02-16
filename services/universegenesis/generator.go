// services/universegenesis/generator.go
package universegenesis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"multiverse-core/internal/eventbus"
	"multiverse-core/internal/oracle"
)

type Generator struct {
	bus       *eventbus.EventBus
	archivist *ArchivistClient // Теперь используется для сохранения universeBanProfile
	oracle    *oracle.Client   // <-- Изменён тип
}

func NewGenerator(bus *eventbus.EventBus, archivist *ArchivistClient, oracle *oracle.Client) *Generator {
	return &Generator{
		bus:       bus,
		archivist: archivist,
		oracle:    oracle,
	}
}

func (g *Generator) StartGenesis(ctx context.Context, seed string, constraints []string) error {
	log.Printf("Generating universe core laws and fundamental principles for seed: %s", seed)

	// 1. Генерация изначальных законов Вселенной и Ядра Вселенной через ИИ
	coreLaws, universeCore, err := g.generateUniverseCore(ctx, seed, constraints)
	if err != nil {
		return fmt.Errorf("failed to generate universe core: %w", err)
	}

	// 2. Генерация онтологического профиля для Запрета Вселенной через ИИ
	// Используем coreLaws и universeCore как контекст
	universeBanProfile, err := g.generateUniverseBanProfile(ctx, universeCore, coreLaws)
	if err != nil {
		return fmt.Errorf("failed to generate universe ban profile: %w", err)
	}

	// 3. Сохранение профиля онтологии для Запрета Вселенной
	profileJSON, err := json.Marshal(universeBanProfile)
	if err != nil {
		return fmt.Errorf("failed to marshal universe ban profile: %w", err)
	}

	// Сохраняем профиль с типом "universe_ontology_profile" и именем "cosmic_law"
	// Это даст путь в OntologicalArchivist: schemas/universe_ontology_profile/cosmic_law/1.0
	if err := g.archivist.SaveSchema(ctx, "universe_ontology_profile", "cosmic_law", "1.0", profileJSON); err != nil {
		log.Printf("Warning: Failed to save universe ban profile: %v", err)
		// Опять же, не критично для завершения генезиса, но желательно сохранить
	}
	log.Printf("Universe ban profile saved")

	core, err := json.Marshal(map[string]interface{}{
		"genesis_seed":  seed,
		"universe_core": universeCore,
		"cosmic_laws":   coreLaws,
		// Убираем archetypal_templates
	})

	// Это даст путь в OntologicalArchivist: schemas/universe_ontology_profile/cosmic_law/1.0
	if err := g.archivist.SaveSchema(ctx, "universe_core", "universe_core", "1.0", core); err != nil {
		log.Printf("Warning: Failed to save universe core: %v", err)
		// Опять же, не критично для завершения генезиса, но желательно сохранить
	}

	// 4. Генерация базовых схем для сущностей вселенной
	entityTypes := []string{"player", "npc", "house", "animal", "artifact"}
	for _, entityType := range entityTypes {
		// Используем функцию из worldgenerator для генерации схемы
		if err := GenerateEntitySchemaWithArchivist(g.archivist, ctx, entityType, seed); err != nil {
			log.Printf("Schema generation warning for %s: %v", entityType, err)
			// Continue with other types
		}
	}

	// Удаляем ненужную функцию

	// 5. Публикация финального события о завершении генезиса

	// Удаляем ненужную функцию

	// 5. Публикация финального события о завершении генезиса
	// Содержит только Ядро и Законы Вселенной
	finalEvent := eventbus.NewEvent(
		"universe.genesis.completed",
		"universe-genesis-oracle",
		seed,
		map[string]interface{}{
			"genesis_seed":  seed,
			"universe_core": universeCore,
			"cosmic_laws":   coreLaws,
			// Убираем archetypal_templates
		})
	/* 	finalEvent := eventbus.Event{
		EventID:   "universe-genesis-" + uuid.New().String()[:8],
		EventType: "universe.genesis.completed",
		Source:    "universe-genesis-oracle",
		WorldID:   "universe", // Условный ID для вселенной
		Payload: map[string]interface{}{
			"genesis_seed":  seed,
			"universe_core": universeCore,
			"cosmic_laws":   coreLaws,
			// Убираем archetypal_templates
		},
		Timestamp: time.Now(),
	} */
	g.bus.Publish(ctx, eventbus.TopicSystemEvents, finalEvent)

	log.Printf("Universe Genesis for seed '%s' completed successfully", seed)
	return nil
}

// generateUniverseCore вызывает Oracle для генерации изначальных законов и Ядра Вселенной.
func (g *Generator) generateUniverseCore(ctx context.Context, seed string, constraints []string) ([]string, string, error) {
	constraintsStr := ""
	if len(constraints) > 0 {
		constraintsStr = fmt.Sprintf("### ОГРАНИЧЕНИЯ: %s\n", constraints)
	}
	systemPrompt := `Ты — Архитектор Вселенной. 
Твоя задача — генерировать фундаментальные космологические структуры: изначальные законы бытия и метафизическую сущность Ядра. 
Избегай антропоцентризма, религиозных терминов и готовых мифологий. 
Создавай оригинальные, самосогласованные принципы, где противоположности  например (хаос/порядок, ничто/бытие, потенциал/актуальность) существуют как единство, но не обязательно.

Все ответы строго в валидном JSON без комментариев, markdown, ёлочек «», многоточий ... или посторонних символов.
Структура ответа:

	{
		"cosmic_laws": ["Закон"],
		"universe_core": "описание ядра"
}
`

	userPrompt := fmt.Sprintf(`Создай космогоническую основу вселенной из семени: "%s"
%s
Требуется:
1. 3-7 изначальных законов — фундаментальных принципов, определяющих природу реальности не физические законы, а онтологические основания например: ( "Вечное становление вместо бытия", "Ничто как активный потенциал", "Симметрия разрушения и созидания").
2. Ядро Вселенной — 1-10 предложения о метафизической сущности, из которой эмерджентно возникает всё многообразие.

ВАЖНО: Ответ строго в валидном JSON. Никаких префиксов вроде "json", комментариев //, ёлочек «», многоточий ... или дополнительных полей.
Пример:
	{
		"cosmic_laws": ["Хаос","Порядок"],
		"universe_core": "описание ядра одной строкой"
	}
Твой ответ будет передан НАПРЯМУЮ в JSON-парсер. Любая синтаксическая ошибка (комментарии //, многоточия ..., кавычки-ёлочки «») сломает систему.

`, seed, constraintsStr)

	// Создаём переменную нужного типа
	var result struct {
		CosmicLaws   []string `json:"cosmic_laws"`
		UniverseCore string   `json:"universe_core"`
	}

	// Вызываем метод, передавая указатель на переменную
	err := g.oracle.CallAndUnmarshal(ctx, func() (string, error) {
		return g.oracle.CallStructured(ctx, systemPrompt, userPrompt)
	}, &result)
	if err != nil {
		return nil, "", fmt.Errorf("oracle call for universe core failed: %w", err)
	}

	return result.CosmicLaws, result.UniverseCore, nil
}

// generateUniverseBanProfile вызывает Oracle для генерации онтологического профиля Запрета Вселенной.
func (g *Generator) generateUniverseBanProfile(ctx context.Context, universeCore string, cosmicLaws []string) (*OntologyProfile, error) {
	// Преобразуем []string в JSON строку для подстановки в промпт
	cosmicLawsJSON, err := json.Marshal(cosmicLaws)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cosmic laws for prompt: %w", err)
	}

	systemPrompt := fmt.Sprintf(`Ты — Архитектор Онтологии. 
Генерируй профиль Запрета Вселенной — фундаментального ограничительного принципа, охраняющего целостность изначальных законов и Ядра.

КОНТЕКСТ:
Ядро: %s
Законы: %s

ИНСТРУКЦИИ:
1. Запрет — не антропоморфный "страж", а онтологический инвариант: самовоспроизводящееся ограничение, встроенное в ткань реальности.
2. Избегай спиритуальных терминов ("дух", "божество"), антропоморфизма и готовых мифологий. Используй нейтральную онтологическую лексику.
3. Архетипы должны быть абстрактными принципами, а не персонажами (не "Страж Хаоса", а "Принцип Диссипации").
4. Все ответы — строго валидный JSON без префиксов (json), комментариев //, ёлочек «», многоточий ..., экранирования Unicode.

	СТРУКТУРА ОТВЕТА:
	{
		"archetypal_carriers": ["принцип/инвариант, не сущность"],
"archetypal_forces": ["фундаментальные противоположности из законов"],
"archetypal_connections": ["механизмы сохранения целостности"],
"archetypal_forbiddances": ["нарушения, которые Запрет предотвращает"],
"general_principles": ["правила работы Запрета как системы"]
}`, universeCore, string(cosmicLawsJSON))

	userPrompt := `Сгенерируй онтологический профиль Запрета Вселенной.

ТРЕБОВАНИЯ К ПОЛЯМ:
- archetypal_carriers: 3-5 инвариантов (например: "Космическая Инвариантность", "Принцип Неустранимости Потенциала")
- archetypal_forces: 3-6 сил из законов (например: "Диссипативный Хаос", "Конденсированный Потенциал")
- archetypal_connections: 2-4 механизма (например: "Резонанс Нарушения", "Обратная Связь Целостности")
- archetypal_forbiddances: 3-5 фундаментальных нарушений (например: "Фиксация Абсолютного Порядка", "Аннигиляция Потенциала")
- general_principles: 2-4 правила (например: "Запрет проявляется только при угрозе целостности Ядра")

ВАЖНО: Ответ должен быть ЧИСТЫМ JSON без каких-либо дополнительных символов до или после структуры. Не добавляй пояснений.
Твой ответ будет передан НАПРЯМУЮ в JSON-парсер. Любая синтаксическая ошибка (комментарии //, многоточия ..., кавычки-ёлочки «») сломает систему.

ПРИМЕР ВАЛИДНОГО ФОРМАТА:
{"archetypal_carriers":["Космическая Инвариантность"],"archetypal_forces":["Диссипативный Хаос"],"archetypal_connections":["Резонанс Нарушения"],"archetypal_forbiddances":["Фиксация Абсолютного Порядка"],"general_principles":["Запрет проявляется только при угрозе целостности Ядра"]}`

	// Создаём переменную нужного типа
	var profile OntologyProfile

	// Вызываем метод, передавая указатель на переменную
	err = g.oracle.CallAndUnmarshal(ctx, func() (string, error) {
		return g.oracle.CallStructured(ctx, systemPrompt, userPrompt)
	}, &profile)
	if err != nil {
		return nil, fmt.Errorf("oracle call for universe ban profile failed: %w", err)
	}

	return &profile, nil
}

// --- Вспомогательные структуры ---

// OntologyProfile определяет структуру онтологического профиля
type OntologyProfile struct {
	ArchetypalCarriers     []string `json:"archetypal_carriers"`
	ArchetypalForces       []string `json:"archetypal_forces"`
	ArchetypalConnections  []string `json:"archetypal_connections"`
	ArchetypalForbiddances []string `json:"archetypal_forbiddances"`
	GeneralPrinciples      []string `json:"general_principles"`
}

// BaseEntitySchema is the base schema for all entities.
const BaseEntitySchema = `{
	"type": "object",
	"properties": {
		"entity_id": {"type": "string", "format": "entity_id"},
		"entity_type": {"type": "string"},
		"created_at": {"type": "string", "format": "date-time"},
		"updated_at": {"type": "string", "format": "date-time"},
		"payload": {"type": "object"},
		"history": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"event_id": {"type": "string", "format": "event_id"},
					"timestamp": {"type": "string", "format": "date-time"}
				},
				"required": ["event_id", "timestamp"],
				"additionalProperties": false
			}
		}
	},
	"required": ["entity_id", "entity_type", "created_at", "updated_at", "payload", "history"]
}`

// GenerateEntitySchemaWithArchivist generates and saves a schema for an entity type using provided archivist client
func GenerateEntitySchemaWithArchivist(archivist *ArchivistClient, ctx context.Context, entityType, worldSeed string) error {
	log.Printf("Generating schema for entity type: %s", entityType)

	// Generate payload schema via Oracle
	payloadSchemaStr, err := generatePayloadSchema(ctx, entityType, worldSeed)
	if err != nil {
		return fmt.Errorf("payload schema generation failed: %w", err)
	}

	// Parse base schema
	var baseSchema map[string]interface{}
	if err := json.Unmarshal([]byte(BaseEntitySchema), &baseSchema); err != nil {
		return fmt.Errorf("base schema parse failed: %w", err)
	}

	// Parse payload schema
	var payloadSchema map[string]interface{}
	if err := json.Unmarshal([]byte(payloadSchemaStr), &payloadSchema); err != nil {
		return fmt.Errorf("payload schema parse failed: %w", err)
	}

	// Merge schemas
	properties := baseSchema["properties"].(map[string]interface{})
	properties["payload"] = payloadSchema

	// Convert to bytes
	fullSchemaBytes, err := json.Marshal(baseSchema)
	if err != nil {
		return fmt.Errorf("schema marshal failed: %w", err)
	}

	// Save to OntologicalArchivist
	if err := archivist.SaveSchema(ctx, "entity", entityType, "1.0", fullSchemaBytes); err != nil {
		return fmt.Errorf("failed to save schema to archivist: %w", err)
	}

	log.Printf("Schema for %s saved to Archivist", entityType)
	return nil
}

// generatePayloadSchema asks Ascension Oracle to generate a payload schema.
func generatePayloadSchema(ctx context.Context, entityType, worldSeed string) (string, error) {
	prompt := fmt.Sprintf(`
 Сгенерируй ТОЛЬКО JSON Schema Draft 7 для поля "payload" сущности типа "%s" в мире с семенем "%s".

 Требования:
 1. Строго в формате JSON Schema.
 2. Используй "format": "entity_id" для ссылок на другие сущности.
 3. Укажи "required" поля.
 4. Пример для игрока:
 {
   "type": "object",
   "properties": {
     "name": {"type": "string"},
     "hp": {"type": "integer", "minimum": 0},
     "inventory": {
       "type": "array",
       "items": {"type": "string", "format": "entity_id"}
     }
   },
   "required": ["name", "hp"]
 }

 Верни ТОЛЬКО JSON без пояснений.
Твой ответ будет передан НАПРЯМУЮ в JSON-парсер. Любая синтаксическая ошибка (комментарии //, многоточия ..., кавычки-ёлочки «») сломает систему.

 `, entityType, worldSeed)

	client := oracle.NewClient()

	content, err := client.CallAndLog(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to connect to oracle: %w", err)
	}

	if content == "" {
		return "", fmt.Errorf("oracle returned empty content")
	}

	return content, nil
}
