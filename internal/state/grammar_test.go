package state

import (
	"slices"
	"testing"
)

// canonicalPath over the spellings the reviews and system-architect named
// (review #1 Ma-2, review #2 N-3, review #3 N-4, condition У-1).
func TestTheCanonicalSpellingOfAPath(t *testing.T) {
	for path, want := range map[string]bool{
		"hp": true, "inventory[0]": true, "inventory[10]": true, "inventory[999999999]": true,
		"npcs[0].last_damager": true, "position.region": true, "метка.лис": true, "a[1][2]": true,
		"": false, "status.": false, ".description": false, "mark..fox": false, "a[]": false, "a[x]": false,
		"inventory[01]": false, "inventory[00]": false, "inventory[1000000000]": false,
		"a[99999999999999999999]": false, "inventory.0": false, "inventory.0.kind": false,
		"a.+1": false, "a.-1": false, "[0]": false, "inventory[-1]": false,
	} {
		if got := canonicalPath(path); got != want {
			t.Errorf("canonicalPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// Every scalar data-model.md §3 names is refused a child path, whatever the type
// of the entity it belongs to (condition У-2 of system-architect; review #3,
// Mi-7). scope is not a scalar: it can be the object {id, type}.
func TestEveryScalarOfTheDataModelIsListed(t *testing.T) {
	byType := map[string][]string{
		"every type (§3)":  {"name"},
		"world (§3.1)":     {"laws_version", "weather", "time_of_day", "day", "season", "epoch", "locale", "blueprint_ref"},
		"region (§3.2)":    {"description", "respawn_ttl", "perception_radius", "encounter_chance", "last_background_event_at", "blueprint_ref"},
		"player (§3.3)":    {"hp", "hp_max", "atk", "def", "dmg", "flee", "status", "position", "actor_kind", "group_id", "encounter_id", "last_session_ended_at"},
		"npc (§3.4)":       {"kind", "region_id", "hp", "hp_max", "status", "position", "died_at", "killed_by", "loot_claimed_by"},
		"group (§3.6)":     {"leader_id", "state"},
		"encounter (§3.7)": {"region_id", "state", "resolution", "round_seq", "task_agent_id", "opened_by_event_id", "closed_by_event_id"},
	}
	for typ, names := range byType {
		for _, name := range names {
			if !slices.Contains(scalarAttributes, name) {
				t.Errorf("%s: %s is not listed as a scalar", typ, name)
			}
			if !underScalar(name + ".x") {
				t.Errorf("%s: %s.x is not refused", typ, name)
			}
		}
	}
	if slices.Contains(scalarAttributes, "scope") || underScalar("scope.id") {
		t.Error("scope is listed as a scalar; it can be the object {id, type}")
	}
}
