// Package worldgenerator implements world generation logic.
package worldgenerator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/oracle"

	"github.com/google/uuid"
)

// WorldGenerationRequest структура запроса из payload события world.generation.requested
type WorldGenerationRequest struct {
	Seed        string                 `json:"seed"`                   // обязательное
	Mode        string                 `json:"mode"`                   // "contextual" | "random"; default "random"
	UserContext *UserWorldContext       `json:"user_context,omitempty"` // заполняется только для mode="contextual"
	Constraints map[string]interface{} `json:"constraints,omitempty"`
}

// UserWorldContext пользовательское описание желаемого мира
type UserWorldContext struct {
	Description  string   `json:"description"`            // свободное описание
	Theme        string   `json:"theme,omitempty"`        // "cultivation", "steampunk", "dark_fantasy", "sci-fi", "mythology", etc.
	KeyElements  []string `json:"key_elements,omitempty"` // ключевые элементы мира
	Scale        string   `json:"scale,omitempty"`        // "small" | "medium" | "large"; default "medium"
	Restrictions []string `json:"restrictions,omitempty"` // чего НЕ должно быть
}

// WorldConcept промежуточный результат первого этапа генерации (концепция мира)
type WorldConcept struct {
	Core         string   `json:"core"`          // ядро мира (2-3 предложения)
	Theme        string   `json:"theme"`         // определённая тема
	Era          string   `json:"era"`           // эпоха / временной период
	UniqueTraits []string `json:"unique_traits"` // 3-5 уникальных черт этого мира
	Scale        string   `json:"scale"`         // итоговый масштаб
}

// WorldGeography представляет полную географическую структуру мира
type WorldGeography struct {
	Core      string        `json:"core"`
	Ontology  WorldOntology `json:"ontology"`
	Geography Geography     `json:"geography"`
	Mythology string        `json:"mythology"`
}

// WorldOntology представляет онтологию (систему силы/прогрессии) мира
type WorldOntology struct {
	System    string   `json:"system"`    // тип системы: "cultivation", "magic", "technology", "divine", "nature" и т.д.
	Carriers  []string `json:"carriers"`  // носители силы (ци, мана, эфир, нанороботы...)
	Paths     []string `json:"paths"`     // пути развития
	Forbidden []string `json:"forbidden"` // запреты / табу
	Hierarchy []string `json:"hierarchy"` // уровни/ранги прогрессии (опционально)
}

// Geography представляет географическую структуру мира
type Geography struct {
	Regions     []Region    `json:"regions"`
	WaterBodies []WaterBody `json:"water_bodies"`
	Cities      []City      `json:"cities"`
}

// Region представляет регион мира
type Region struct {
	Name        string  `json:"name"`
	Biome       string  `json:"biome"`
	Coordinates Point   `json:"coordinates"`
	Size        float64 `json:"size"`
}

// WaterBody представляет водный объект
type WaterBody struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"` // river, sea, lake
	Coordinates Point   `json:"coordinates"`
	Size        float64 `json:"size"`
}

// City представляет город
type City struct {
	Name       string   `json:"name"`
	Population int      `json:"population"`
	Type       string   `json:"type"` // major, minor
	Location   Location `json:"location"`
}

// Location представляет местоположение
type Location struct {
	Region      string `json:"region"`
	Coordinates Point  `json:"coordinates"`
}

// Point представляет точку координат
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// WorldGenerator creates new worlds with unique ontologies.
type WorldGenerator struct {
	bus       *eventbus.EventBus
	archivist ArchivistClient
	oracle    *oracle.Client
}

// NewWorldGenerator creates a new WorldGenerator.
func NewWorldGenerator(bus *eventbus.EventBus) *WorldGenerator {
	return &WorldGenerator{
		bus:       bus,
		archivist: *NewArchivistClient(),
		oracle:    oracle.NewClient(),
	}
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

// defaultIfEmpty возвращает значение по умолчанию, если строка пустая
func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// parseGenerationRequest парсит payload события в структурированный запрос
func parseGenerationRequest(payload map[string]interface{}) (*WorldGenerationRequest, error) {
	// Сериализовать payload в JSON
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Десериализовать в WorldGenerationRequest
	var request WorldGenerationRequest
	err = json.Unmarshal(data, &request)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Валидация: seed обязателен
	if request.Seed == "" {
		return nil, fmt.Errorf("seed is required")
	}

	// Если mode пустой — установить "random"
	if request.Mode == "" {
		request.Mode = "random"
	}

	// Если mode="contextual" и user_context=nil — вернуть ошибку
	if request.Mode == "contextual" && request.UserContext == nil {
		return nil, fmt.Errorf("mode=contextual requires user_context")
	}

	// Если scale пустой — установить "medium" (будет установлено позже через getScale())
	if request.Mode == "contextual" && request.UserContext != nil && request.UserContext.Scale == "" {
		request.UserContext.Scale = "medium"
	}

	return &request, nil
}

// HandleEvent processes world generation requests.
func (wg *WorldGenerator) HandleEvent(ev eventbus.Event) {
	if ev.EventType != "world.generation.requested" {
		return
	}

	ctx := context.Background()

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
	wg.createGeographicEntities(ctx, worldID, *geography)

	// 6. Финальное событие
	wg.publishWorldGenerated(ctx, worldID, request, concept)

	log.Printf("World %s generated successfully (mode=%s, theme=%s)", worldID, request.Mode, concept.Theme)
}

// publishWorldCreated публикует entity.created для мира с концепцией
func (wg *WorldGenerator) publishWorldCreated(ctx context.Context, worldID string, req *WorldGenerationRequest, concept *WorldConcept) {
	worldEvent := eventbus.Event{
		EventID:   "world-gen-" + uuid.New().String()[:8],
		EventType: "entity.created",
		Source:    "world-generator",
		WorldID:   worldID,
		Payload: map[string]interface{}{
			"entity_id":   worldID,
			"entity_type": "world",
			"payload": map[string]interface{}{
				"seed":           req.Seed,
				"mode":           req.Mode,
				"theme":          concept.Theme,
				"core":           concept.Core,
				"era":            concept.Era,
				"unique_traits":  concept.UniqueTraits,
				"plan":           0,
				"constraints":    req.Constraints,
			},
		},
		Timestamp: time.Now(),
	}
	wg.bus.Publish(ctx, eventbus.TopicSystemEvents, worldEvent)
	log.Printf("Published world.created event: %s (theme=%s)", worldID, concept.Theme)
}

// publishWorldGenerated публикует финальное событие world.generated
func (wg *WorldGenerator) publishWorldGenerated(ctx context.Context, worldID string, req *WorldGenerationRequest, concept *WorldConcept) {
	finalEvent := eventbus.Event{
		EventID:   "world-gen-final-" + uuid.New().String()[:8],
		EventType: "world.generated",
		Source:    "world-generator",
		WorldID:   worldID,
		Payload: map[string]interface{}{
			"world_id": worldID,
			"seed":     req.Seed,
			"mode":     req.Mode,
			"theme":    concept.Theme,
		},
		Timestamp: time.Now(),
	}
	wg.bus.Publish(ctx, eventbus.TopicSystemEvents, finalEvent)
	log.Printf("Published world.generated event: %s", worldID)
}

// createGeographicEntities creates entities for geographic objects
func (wg *WorldGenerator) createGeographicEntities(ctx context.Context, worldID string, geography WorldGeography) {
	// Create regions
	for _, region := range geography.Geography.Regions {
		wg.createRegionEntity(ctx, worldID, region)
	}

	// Create water bodies
	for _, water := range geography.Geography.WaterBodies {
		wg.createWaterEntity(ctx, worldID, water)
	}

	// Create cities
	for _, city := range geography.Geography.Cities {
		wg.createCityEntity(ctx, worldID, city)
	}

	// Publish geography generated event
	wg.publishGeographyGeneratedEvent(ctx, worldID, geography)
}

// createRegionEntity creates a region entity
func (wg *WorldGenerator) createRegionEntity(ctx context.Context, worldID string, region Region) {
	regionID := "region-" + uuid.New().String()[:8]

	regionEvent := eventbus.Event{
		EventID:   "region-create-" + uuid.New().String()[:8],
		EventType: "entity.created",
		Source:    "world-generator",
		WorldID:   worldID,
		Payload: map[string]interface{}{
			"entity_id":   regionID,
			"entity_type": "region",
			"payload": map[string]interface{}{
				"name":        region.Name,
				"biome":       region.Biome,
				"coordinates": region.Coordinates,
				"size":        region.Size,
			},
		},
		Timestamp: time.Now(),
	}

	wg.bus.Publish(ctx, eventbus.TopicSystemEvents, regionEvent)
	log.Printf("Created region entity: %s (%s)", region.Name, region.Biome)
}

// createWaterEntity creates a water body entity
func (wg *WorldGenerator) createWaterEntity(ctx context.Context, worldID string, water WaterBody) {
	waterID := "water-" + uuid.New().String()[:8]

	waterEvent := eventbus.Event{
		EventID:   "water-create-" + uuid.New().String()[:8],
		EventType: "entity.created",
		Source:    "world-generator",
		WorldID:   worldID,
		Payload: map[string]interface{}{
			"entity_id":   waterID,
			"entity_type": "water_body",
			"payload": map[string]interface{}{
				"name":        water.Name,
				"type":        water.Type,
				"coordinates": water.Coordinates,
				"size":        water.Size,
			},
		},
		Timestamp: time.Now(),
	}

	wg.bus.Publish(ctx, eventbus.TopicSystemEvents, waterEvent)
	log.Printf("Created water entity: %s (%s)", water.Name, water.Type)
}

// createCityEntity creates a city entity
func (wg *WorldGenerator) createCityEntity(ctx context.Context, worldID string, city City) {
	cityID := "city-" + uuid.New().String()[:8]

	cityEvent := eventbus.Event{
		EventID:   "city-create-" + uuid.New().String()[:8],
		EventType: "entity.created",
		Source:    "world-generator",
		WorldID:   worldID,
		Payload: map[string]interface{}{
			"entity_id":   cityID,
			"entity_type": "city",
			"payload": map[string]interface{}{
				"name":       city.Name,
				"population": city.Population,
				"type":       city.Type,
				"location":   city.Location,
			},
		},
		Timestamp: time.Now(),
	}

	wg.bus.Publish(ctx, eventbus.TopicSystemEvents, cityEvent)
	log.Printf("Created city entity: %s (population: %d)", city.Name, city.Population)
}

// publishGeographyGeneratedEvent publishes an event when geography is generated
func (wg *WorldGenerator) publishGeographyGeneratedEvent(ctx context.Context, worldID string, geography WorldGeography) {
	geographyEvent := eventbus.Event{
		EventID:   "geography-generated-" + uuid.New().String()[:8],
		EventType: "world.geography.generated",
		Source:    "world-generator",
		WorldID:   worldID,
		Payload: map[string]interface{}{
			"world_id":     worldID,
			"regions":      len(geography.Geography.Regions),
			"water_bodies": len(geography.Geography.WaterBodies),
			"cities":       len(geography.Geography.Cities),
		},
		Timestamp: time.Now(),
	}

	wg.bus.Publish(ctx, eventbus.TopicSystemEvents, geographyEvent)
	log.Printf("Published geography generated event for world: %s", worldID)
}
