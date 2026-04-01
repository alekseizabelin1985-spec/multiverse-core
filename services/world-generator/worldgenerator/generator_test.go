// Package worldgenerator implements world generation logic.
package worldgenerator

import (
	"strings"
	"testing"

	"multiverse-core.io/shared/eventbus"

	"github.com/stretchr/testify/assert"
)

// Test parseGenerationRequest — парсинг запроса для случайного режима
func TestParseGenerationRequest_Random(t *testing.T) {
	payload := map[string]interface{}{
		"seed": "Eternal Void",
	}

	request, err := parseGenerationRequest(payload)
	assert.NoError(t, err)
	assert.NotNil(t, request)
	assert.Equal(t, "Eternal Void", request.Seed)
	assert.Equal(t, "random", request.Mode)
	assert.Nil(t, request.UserContext)
}

// Test parseGenerationRequest — парсинг запроса для контекстного режима
func TestParseGenerationRequest_Contextual(t *testing.T) {
	payload := map[string]interface{}{
		"seed": "Jade Heavens",
		"mode": "contextual",
		"user_context": map[string]interface{}{
			"description": "Мир культивации с несколькими континентами",
			"theme":       "cultivation",
			"key_elements": []interface{}{"континенты", "секты", "духовная энергия"},
			"scale":       "large",
		},
	}

	request, err := parseGenerationRequest(payload)
	assert.NoError(t, err)
	assert.NotNil(t, request)
	assert.Equal(t, "Jade Heavens", request.Seed)
	assert.Equal(t, "contextual", request.Mode)
	assert.NotNil(t, request.UserContext)
	assert.Equal(t, "Мир культивации с несколькими континентами", request.UserContext.Description)
	assert.Equal(t, "cultivation", request.UserContext.Theme)
	assert.Equal(t, "large", request.UserContext.Scale)
}

// Test parseGenerationRequest — ошибка: отсутствует seed
func TestParseGenerationRequest_MissingSeed(t *testing.T) {
	payload := map[string]interface{}{
		"mode": "random",
	}

	request, err := parseGenerationRequest(payload)
	assert.Error(t, err)
	assert.Nil(t, request)
	assert.Contains(t, err.Error(), "seed is required")
}

// Test parseGenerationRequest — ошибка: contextual без user_context
func TestParseGenerationRequest_ContextualWithoutContext(t *testing.T) {
	payload := map[string]interface{}{
		"seed": "Test World",
		"mode": "contextual",
	}

	request, err := parseGenerationRequest(payload)
	assert.Error(t, err)
	assert.Nil(t, request)
	assert.Contains(t, err.Error(), "mode=contextual requires user_context")
}

// Test parseGenerationRequest — дефолт scale в UserContext
func TestParseGenerationRequest_DefaultScale(t *testing.T) {
	payload := map[string]interface{}{
		"seed": "Clockwork Dawn",
		"mode": "contextual",
		"user_context": map[string]interface{}{
			"description": "Стимпанк мир",
		},
	}

	request, err := parseGenerationRequest(payload)
	assert.NoError(t, err)
	assert.NotNil(t, request)
	assert.Equal(t, "medium", request.UserContext.Scale)
}

// Test WorldGenerationRequest.getScale() — с nil UserContext
func TestWorldGenerationRequest_GetScale_Random(t *testing.T) {
	req := &WorldGenerationRequest{
		Seed:        "Test",
		Mode:        "random",
		UserContext: nil,
	}

	assert.Equal(t, "medium", req.getScale())
}

// Test WorldGenerationRequest.getScale() — с заполненным UserContext
func TestWorldGenerationRequest_GetScale_Contextual(t *testing.T) {
	req := &WorldGenerationRequest{
		Seed: "Test",
		Mode: "contextual",
		UserContext: &UserWorldContext{
			Description: "Test world",
			Scale:       "large",
		},
	}

	assert.Equal(t, "large", req.getScale())
}

// Test scaleParams — для "small"
func TestScaleParams_Small(t *testing.T) {
	minR, maxR, minW, maxW, minC, maxC := scaleParams("small")
	assert.Equal(t, 2, minR)
	assert.Equal(t, 3, maxR)
	assert.Equal(t, 1, minW)
	assert.Equal(t, 2, maxW)
	assert.Equal(t, 1, minC)
	assert.Equal(t, 2, maxC)
}

// Test scaleParams — для "medium"
func TestScaleParams_Medium(t *testing.T) {
	minR, maxR, minW, maxW, minC, maxC := scaleParams("medium")
	assert.Equal(t, 3, minR)
	assert.Equal(t, 5, maxR)
	assert.Equal(t, 2, minW)
	assert.Equal(t, 4, maxW)
	assert.Equal(t, 2, minC)
	assert.Equal(t, 4, maxC)
}

// Test scaleParams — для "large"
func TestScaleParams_Large(t *testing.T) {
	minR, maxR, minW, maxW, minC, maxC := scaleParams("large")
	assert.Equal(t, 5, minR)
	assert.Equal(t, 8, maxR)
	assert.Equal(t, 4, minW)
	assert.Equal(t, 7, maxW)
	assert.Equal(t, 4, minC)
	assert.Equal(t, 8, maxC)
}

// Test scaleParams — дефолт для неизвестного масштаба
func TestScaleParams_Default(t *testing.T) {
	minR, maxR, _, _, _, _ := scaleParams("unknown")
	assert.Equal(t, 3, minR)
	assert.Equal(t, 5, maxR) // medium by default
}

// Test buildConceptPrompts — случайный режим
func TestBuildConceptPrompts_Random(t *testing.T) {
	req := &WorldGenerationRequest{
		Seed: "Eternal Void",
		Mode: "random",
	}

	systemPrompt, userPrompt := buildConceptPrompts(req)

	// Проверить что systemPrompt содержит JSON-формат
	assert.Contains(t, systemPrompt, "JSON")
	assert.Contains(t, systemPrompt, "core")
	assert.Contains(t, systemPrompt, "theme")

	// Проверить что userPrompt содержит seed и НЕ содержит description
	assert.Contains(t, userPrompt, "Eternal Void")
	assert.NotContains(t, userPrompt, "Описание")
	assert.NotContains(t, userPrompt, "description")
	assert.Contains(t, userPrompt, "оригинальную")
}

// Test buildConceptPrompts — контекстный режим
func TestBuildConceptPrompts_Contextual(t *testing.T) {
	req := &WorldGenerationRequest{
		Seed: "Jade Heavens",
		Mode: "contextual",
		UserContext: &UserWorldContext{
			Description:  "Мир культивации",
			Theme:        "cultivation",
			KeyElements:  []string{"континенты", "секты"},
			Scale:        "large",
			Restrictions: []string{"нет магии огня"},
		},
	}

	systemPrompt, userPrompt := buildConceptPrompts(req)

	// Проверить что systemPrompt содержит JSON-формат
	assert.Contains(t, systemPrompt, "JSON")

	// Проверить что userPrompt содержит все элементы контекста
	assert.Contains(t, userPrompt, "Jade Heavens")
	assert.Contains(t, userPrompt, "Мир культивации")
	assert.Contains(t, userPrompt, "cultivation")
	assert.Contains(t, userPrompt, "континенты")
	assert.Contains(t, userPrompt, "large")
	assert.Contains(t, userPrompt, "нет магии огня")
}

// Test buildDetailsPrompts — проверка масштаба small
func TestBuildDetailsPrompts_ScaleSmall(t *testing.T) {
	concept := &WorldConcept{
		Core:         "Ядро мира",
		Theme:        "cultivation",
		Era:          "Ancient times",
		UniqueTraits: []string{"трейт1", "трейт2"},
	}

	systemPrompt, userPrompt := buildDetailsPrompts(concept, "small")

	assert.Contains(t, systemPrompt, "cultivation")
	assert.Contains(t, systemPrompt, "Ancient times")
	// Проверить что userPrompt содержит "2-3 регионов"
	assert.Contains(t, userPrompt, "2-3")
}

// Test buildDetailsPrompts — проверка масштаба large
func TestBuildDetailsPrompts_ScaleLarge(t *testing.T) {
	concept := &WorldConcept{
		Core:         "Ядро мира",
		Theme:        "steampunk",
		Era:          "Industrial age",
		UniqueTraits: []string{"трейт1", "трейт2"},
	}

	systemPrompt, userPrompt := buildDetailsPrompts(concept, "large")

	assert.Contains(t, systemPrompt, "steampunk")
	// Проверить что userPrompt содержит "5-8 регионов"
	assert.Contains(t, userPrompt, "5-8")
}

// Test buildDetailsPrompts — проверка структуры JSON
func TestBuildDetailsPrompts_JSONStructure(t *testing.T) {
	concept := &WorldConcept{
		Core:         "Core",
		Theme:        "magic",
		Era:          "Medieval",
		UniqueTraits: []string{"trait1"},
	}

	_, userPrompt := buildDetailsPrompts(concept, "medium")

	// Проверить что промпт содержит требуемую JSON-структуру
	assert.Contains(t, userPrompt, "ontology")
	assert.Contains(t, userPrompt, "system")
	assert.Contains(t, userPrompt, "carriers")
	assert.Contains(t, userPrompt, "paths")
	assert.Contains(t, userPrompt, "forbidden")
	assert.Contains(t, userPrompt, "hierarchy")
	assert.Contains(t, userPrompt, "geography")
	assert.Contains(t, userPrompt, "regions")
	assert.Contains(t, userPrompt, "water_bodies")
	assert.Contains(t, userPrompt, "cities")
	assert.Contains(t, userPrompt, "mythology")
}

// Test defaultIfEmpty
func TestDefaultIfEmpty(t *testing.T) {
	assert.Equal(t, "default", defaultIfEmpty("", "default"))
	assert.Equal(t, "value", defaultIfEmpty("value", "default"))
	assert.Equal(t, "default", defaultIfEmpty("", "default"))
}

// Test WorldGeographyStructures
func TestWorldGeographyStructures(t *testing.T) {
	// Test that all structures can be created
	var geography WorldGeography
	assert.NotNil(t, geography)

	var ontology WorldOntology
	assert.NotNil(t, ontology)

	var geographyData Geography
	assert.NotNil(t, geographyData)

	var region Region
	assert.NotNil(t, region)

	var waterBody WaterBody
	assert.NotNil(t, waterBody)

	var city City
	assert.NotNil(t, city)

	var location Location
	assert.NotNil(t, location)

	var point Point
	assert.NotNil(t, point)
}

// Test WorldGeneratorCreation
func TestWorldGeneratorCreation(t *testing.T) {
	// Create a mock event bus
	mockBus := &eventbus.EventBus{}

	// Create world generator
	gen := NewWorldGenerator(mockBus)
	assert.NotNil(t, gen)

	// Test that it has the right fields
	assert.Equal(t, mockBus, gen.bus)
	assert.NotNil(t, gen.archivist)
	assert.NotNil(t, gen.oracle)
}

// Test buildConceptPrompts with minimal context
func TestBuildConceptPrompts_ContextualMinimal(t *testing.T) {
	req := &WorldGenerationRequest{
		Seed: "Clockwork Dawn",
		Mode: "contextual",
		UserContext: &UserWorldContext{
			Description: "Стимпанк мир с летающими городами",
		},
	}

	_, userPrompt := buildConceptPrompts(req)

	assert.Contains(t, userPrompt, "Clockwork Dawn")
	assert.Contains(t, userPrompt, "Стимпанк мир с летающими городами")
	assert.Contains(t, userPrompt, "Демиурга") // defaultIfEmpty for empty theme
}

// Test buildDetailsPrompts — проверка схемы ontology
func TestBuildDetailsPrompts_OntologyInPrompt(t *testing.T) {
	concept := &WorldConcept{
		Core:         "Core",
		Theme:        "nature",
		Era:          "Primordial",
		UniqueTraits: []string{"natural", "wild"},
	}

	_, userPrompt := buildDetailsPrompts(concept, "medium")

	// Проверить требования к онтологии
	assert.Contains(t, userPrompt, "cultivation, magic, technology, divine, nature")
	assert.Contains(t, userPrompt, "2-4")  // carriers
	assert.Contains(t, userPrompt, "3-5")  // paths
	assert.Contains(t, userPrompt, "4-7")  // hierarchy
}

// Test backward compatibility — parseGenerationRequest с пустым mode
func TestBackwardCompatibility_DefaultRandomMode(t *testing.T) {
	payload := map[string]interface{}{
		"seed": "Old Format World",
	}

	request, err := parseGenerationRequest(payload)
	assert.NoError(t, err)
	assert.NotNil(t, request)
	assert.Equal(t, "random", request.Mode)
}

// Test parseGenerationRequest с constraints (обратная совместимость)
func TestParseGenerationRequest_WithConstraints(t *testing.T) {
	payload := map[string]interface{}{
		"seed": "World with constraints",
		"constraints": map[string]interface{}{
			"max_regions": 5,
		},
	}

	request, err := parseGenerationRequest(payload)
	assert.NoError(t, err)
	assert.NotNil(t, request)
	assert.NotNil(t, request.Constraints)
	assert.Equal(t, 5.0, request.Constraints["max_regions"])
}

// Test buildConceptPrompts — проверка русского языка в системном промпте
func TestBuildConceptPrompts_RussianPrompt(t *testing.T) {
	req := &WorldGenerationRequest{
		Seed: "Test",
		Mode: "random",
	}

	systemPrompt, userPrompt := buildConceptPrompts(req)

	// Проверить что промпты на русском
	assert.Contains(t, systemPrompt, "Демиург")
	assert.Contains(t, systemPrompt, "JSON")
	assert.Contains(t, userPrompt, "оригинальную")
	assert.Contains(t, strings.ToLower(userPrompt), "тематики")
}

// Test buildDetailsPrompts — проверка русского языка
func TestBuildDetailsPrompts_RussianPrompt(t *testing.T) {
	concept := &WorldConcept{
		Core:         "Core",
		Theme:        "magic",
		Era:          "Medieval",
		UniqueTraits: []string{"trait"},
	}

	systemPrompt, userPrompt := buildDetailsPrompts(concept, "medium")

	assert.Contains(t, systemPrompt, "Демиург")
	assert.Contains(t, userPrompt, "Сгенерируй")
	assert.Contains(t, userPrompt, "Требования")
	assert.Contains(t, userPrompt, "География")
	assert.Contains(t, userPrompt, "Мифология")
}
