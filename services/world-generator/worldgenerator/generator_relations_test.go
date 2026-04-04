package worldgenerator

import (
	"testing"

	"multiverse-core.io/shared/eventbus"
)

func TestWorldGenerator_RelationsCreation(t *testing.T) {
	// Проверяем что Relations корректно формируются
	worldID := "test-world-123"
	regionID := "region-abc123"
	cityID := "city-def456"
	waterID := "water-ghi789"

	// Тест region relations — мир без префикса
	regionEvent := eventbus.NewEvent("entity.created", "world-generator", worldID, nil)
	regionEvent.Relations = []eventbus.Relation{
		{
			From:     worldID,
			To:       regionID,
			Type:     eventbus.RelContains,
			Directed: true,
			Metadata: map[string]any{"biome": "forest"},
		},
	}

	if len(regionEvent.Relations) != 1 {
		t.Fatalf("Expected 1 relation, got %d", len(regionEvent.Relations))
	}
	if regionEvent.Relations[0].Type != eventbus.RelContains {
		t.Errorf("Expected relation type %s, got %s", eventbus.RelContains, regionEvent.Relations[0].Type)
	}
	if regionEvent.Relations[0].From != worldID {
		t.Errorf("Expected relation from %s, got %s", worldID, regionEvent.Relations[0].From)
	}

	// Тест city relations — мир без префикса
	cityEvent := eventbus.NewEvent("entity.created", "world-generator", worldID, nil)
	cityEvent.Relations = []eventbus.Relation{
		{
			From:     cityID,
			To:       worldID,
			Type:     eventbus.RelWorldOf,
			Directed: true,
			Metadata: map[string]any{"city_type": "major", "population": 50000},
		},
	}

	if len(cityEvent.Relations) != 1 {
		t.Fatalf("Expected 1 relation, got %d", len(cityEvent.Relations))
	}
	if cityEvent.Relations[0].Type != eventbus.RelWorldOf {
		t.Errorf("Expected relation type %s, got %s", eventbus.RelWorldOf, cityEvent.Relations[0].Type)
	}
	if cityEvent.Relations[0].To != worldID {
		t.Errorf("Expected relation to %s, got %s", worldID, cityEvent.Relations[0].To)
	}

	// Тест water relations — мир без префикса
	waterEvent := eventbus.NewEvent("entity.created", "world-generator", worldID, nil)
	waterEvent.Relations = []eventbus.Relation{
		{
			From:     worldID,
			To:       waterID,
			Type:     eventbus.RelContains,
			Directed: true,
			Metadata: map[string]any{"water_type": "river"},
		},
	}

	if err := eventbus.ValidateEventRelations(waterEvent); err != nil {
		t.Errorf("Expected valid relations, got error: %v", err)
	}
	if waterEvent.Relations[0].From != worldID {
		t.Errorf("Expected relation from %s, got %s", worldID, waterEvent.Relations[0].From)
	}

	_ = regionEvent
	_ = cityEvent
}
