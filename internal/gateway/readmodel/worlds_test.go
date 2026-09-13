package readmodel_test

import (
	"testing"

	"multiverse-core.io/shared/entity"
)

// World reads one world with its weather, time and day; Regions lists the
// regions of one world by id and leaves those of another world out.
func TestWorldAndItsRegions(t *testing.T) {
	m, _ := newModel(t)
	mustApply(t, m, created(t, world, entity.TypeWorld, "c-w", map[string]any{
		"laws_version": "v2", "locale": "ru", "weather": "rain", "time_of_day": "night", "day": 12,
	}))
	mustApply(t, m, created(t, "region-b", entity.TypeRegion, "c-b", map[string]any{"description": "b"}))
	mustApply(t, m, created(t, "region-a", entity.TypeRegion, "c-a", map[string]any{"description": "a", "npc_ids": []any{"wolf-1"}}))
	other := created(t, "region-far", entity.TypeRegion, "c-far", map[string]any{"description": "far"})
	other.World.Entity.ID = "other-world"
	mustApply(t, m, other)

	w, ok := m.World(world)
	if !ok || w.LawsVersion != "v2" || w.Weather != "rain" || w.TimeOfDay != "night" || w.Day == nil || *w.Day != 12 {
		t.Errorf("World = %+v %v", w, ok)
	}
	if _, ok := m.World("region-a"); ok {
		t.Error("World returned a region")
	}
	regions := m.Regions(world)
	if len(regions) != 2 || regions[0].ID != "region-a" || regions[1].ID != "region-b" ||
		len(regions[0].NPCIDs) != 1 || regions[0].Description != "a" {
		t.Errorf("Regions = %+v", regions)
	}
	if far := m.Regions("other-world"); len(far) != 1 || far[0].ID != "region-far" {
		t.Errorf("Regions of the other world = %+v", far)
	}
	if none := m.Regions("no-world"); len(none) != 0 {
		t.Errorf("Regions of no world = %+v", none)
	}
}
