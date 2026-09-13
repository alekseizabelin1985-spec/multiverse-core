package agent_test

import (
	"slices"
	"testing"

	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/contracts"
)

// The white lists of analysis/api-contracts.md §2.4.
func TestAllowedEventTypes(t *testing.T) {
	domain := []string{"encounter.started", "entity.create.proposed", "entity.update.proposed", "npc.moved", "npc.spawned", "region.event_occurred"}
	cases := []struct {
		level, role string
		want        []string
	}{
		{"global", "global-gm", []string{"entity.update.proposed", "world.event_occurred", "world.time_advanced", "world.weather_changed"}},
		{"domain", "region-gm", domain},
		{"domain", "city-gm", domain},
		{"task", "encounter", []string{"combat.decided", "dice.rolled", "encounter.ended", "entity.update.proposed"}},
		{"task", "personal-gm", []string{"narrative.output"}},
		{"task", "group-narrator", []string{"narrative.output"}},
		{"monitor", "guardian-monitor", nil},
		{"object", "entity-actor", nil},
		// A pair that does not make sense gets no list.
		{"task", "region-gm", nil},
		{"global", "personal-gm", nil},
		{"domain", "bard", nil},
		{"", "encounter", nil},
	}
	for _, tc := range cases {
		if got := agent.AllowedEventTypes(tc.level, tc.role); !slices.Equal(got, tc.want) {
			t.Errorf("AllowedEventTypes(%q, %q) = %q, want %q", tc.level, tc.role, got, tc.want)
		}
	}
}

func TestAllowedEventTypesIsACopy(t *testing.T) {
	first := agent.AllowedEventTypes("task", "personal-gm")
	first[0] = "player.attacked"
	if got := agent.AllowedEventTypes("task", "personal-gm"); !slices.Equal(got, []string{"narrative.output"}) {
		t.Fatalf("the white list was changed through a returned slice: %q", got)
	}
	types := agent.ScopeTypeNames()
	types[0] = "planet"
	if got := agent.ScopeTypeNames(); got[0] != "group" {
		t.Fatalf("the scope types were changed through a returned slice: %q", got)
	}
}

// Every type a role may publish is a type of the registry: a white list that
// names a type nobody registered would make every blueprint of the role
// invalid.
func TestAllowedEventTypesAreRegistered(t *testing.T) {
	registered := agent.NewSet(registryTypes()...)
	for _, role := range agent.RoleNames() {
		level, _ := agent.RoleLevel(role)
		for _, typ := range agent.AllowedEventTypes(level, role) {
			if !registered.Has(typ) {
				t.Errorf("role %s may publish %s, which is not in the registry", role, typ)
			}
		}
	}
}

// levels.go holds no ownership table, but every level it knows has its row in
// contracts.OwnershipRules, so the view a caller builds covers every level.
func TestEveryLevelHasAnOwnershipRow(t *testing.T) {
	proposers := map[string]bool{}
	for _, rule := range contracts.OwnershipRules() {
		proposers[rule.Proposer] = true
	}
	for _, level := range agent.LevelNames() {
		if !proposers[level] {
			t.Errorf("level %s has no row in contracts.OwnershipRules", level)
		}
	}
}

func TestLevelsAndRoles(t *testing.T) {
	levels := agent.LevelNames()
	if !slices.IsSorted(levels) || !slices.Equal(levels, []string{"domain", "global", "monitor", "object", "task"}) {
		t.Errorf("LevelNames() = %q", levels)
	}
	roles := agent.RoleNames()
	if !slices.IsSorted(roles) || len(roles) != 8 {
		t.Errorf("RoleNames() = %q", roles)
	}
	for _, role := range roles {
		level, ok := agent.RoleLevel(role)
		if !ok || !agent.IsKnownLevel(level) {
			t.Errorf("RoleLevel(%q) = %q, %v", role, level, ok)
		}
	}
	if _, ok := agent.RoleLevel("bard"); ok {
		t.Error("RoleLevel(bard): an unknown role has no level")
	}
	if agent.IsKnownLevel("quest") || agent.IsKnownLevel("") {
		t.Error("IsKnownLevel accepts a level that is not one")
	}
	for level, reserved := range map[string]bool{"global": false, "domain": false, "task": false, "monitor": true, "object": true, "": false} {
		if got := agent.IsReservedLevel(level); got != reserved {
			t.Errorf("IsReservedLevel(%q) = %v, want %v", level, got, reserved)
		}
	}
}

// A level name and its AgentLevel are one thing written two ways.
func TestParseLevel(t *testing.T) {
	for _, level := range agent.LevelNames() {
		if got := agent.ParseLevel(level); got == agent.LevelUnknown || got.String() != level {
			t.Errorf("ParseLevel(%q) = %v", level, got)
		}
	}
	for _, name := range []string{"", "unknown", "Global", "quest"} {
		if got := agent.ParseLevel(name); got != agent.LevelUnknown {
			t.Errorf("ParseLevel(%q) = %v, want LevelUnknown", name, got)
		}
	}
}
