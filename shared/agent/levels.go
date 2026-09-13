package agent

import "slices"

// Level names as written in a blueprint (level:). They are the String of
// AgentLevel.
const (
	LevelNameGlobal  = "global"
	LevelNameDomain  = "domain"
	LevelNameTask    = "task"
	LevelNameMonitor = "monitor"
	LevelNameObject  = "object"
)

// Roles of the swarm (role:), swarm-llm-laws.md §4.1, C-11.
const (
	RoleGlobalGM        = "global-gm"
	RoleRegionGM        = "region-gm"
	RoleCityGM          = "city-gm"
	RolePersonalGM      = "personal-gm"
	RoleEncounter       = "encounter"
	RoleGroupNarrator   = "group-narrator"
	RoleGuardianMonitor = "guardian-monitor"
	RoleEntityActor     = "entity-actor"
)

// Scope types of a scope binding (scope_binding.type).
const (
	ScopeTypeWorld  = "world"
	ScopeTypeRegion = "region"
	ScopeTypeSolo   = "solo"
	ScopeTypeGroup  = "group"
)

// roleSpec is what the registry knows about one role. It holds no ownership
// of entity types: that table has one source, contracts.OwnershipRules, and
// the validator receives its view from the caller (ADR-025, C-02 v1.4).
type roleSpec struct {
	level string
	// scopeTypes are the scope types an instance of the role binds to; nil
	// means the role puts no constraint beyond the known scope types.
	scopeTypes []string
	// events are the event types the role may publish (analysis/api-contracts.md
	// §2.4, "white lists of the levels", after data-model.md §4).
	events []string
}

// roles is the registry of levels and roles of the swarm. The reserved roles
// of monitor and object publish nothing: their spawn is disabled in MVP-1
// (C-13), and an empty white list keeps it that way whatever a blueprint says.
var roles = map[string]roleSpec{
	RoleGlobalGM: {
		level:      LevelNameGlobal,
		scopeTypes: []string{ScopeTypeWorld},
		events:     []string{"entity.update.proposed", "world.event_occurred", "world.time_advanced", "world.weather_changed"},
	},
	RoleRegionGM: {
		level:      LevelNameDomain,
		scopeTypes: []string{ScopeTypeRegion},
		events:     domainEvents,
	},
	// A city is a domain of its own kind (target, not in MVP-1); it publishes
	// what a region publishes.
	RoleCityGM: {
		level:      LevelNameDomain,
		scopeTypes: []string{ScopeTypeRegion},
		events:     domainEvents,
	},
	RolePersonalGM: {
		level:      LevelNameTask,
		scopeTypes: []string{ScopeTypeSolo},
		events:     []string{"narrative.output"},
	},
	RoleGroupNarrator: {
		level:      LevelNameTask,
		scopeTypes: []string{ScopeTypeGroup},
		events:     []string{"narrative.output"},
	},
	// The encounter agent changes entities only through the mechanics
	// (mechanics.ChangesFor), so it proposes updates and creates nothing.
	RoleEncounter: {
		level:      LevelNameTask,
		scopeTypes: []string{ScopeTypeSolo, ScopeTypeGroup},
		events:     []string{"combat.decided", "dice.rolled", "encounter.ended", "entity.update.proposed"},
	},
	RoleGuardianMonitor: {
		level:      LevelNameMonitor,
		scopeTypes: []string{ScopeTypeWorld},
	},
	// An entity actor binds to an entity, which is none of the scope types of
	// MVP-1; the binding is left to C-13.
	RoleEntityActor: {
		level: LevelNameObject,
	},
}

var domainEvents = []string{
	"encounter.started", "entity.create.proposed", "entity.update.proposed",
	"npc.moved", "npc.spawned", "region.event_occurred",
}

var scopeTypes = []string{ScopeTypeGroup, ScopeTypeRegion, ScopeTypeSolo, ScopeTypeWorld}

// LevelNames returns the levels of the swarm, sorted.
func LevelNames() []string {
	return []string{LevelNameDomain, LevelNameGlobal, LevelNameMonitor, LevelNameObject, LevelNameTask}
}

// RoleNames returns the roles of the swarm, sorted.
func RoleNames() []string {
	names := make([]string, 0, len(roles))
	for name := range roles {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// ScopeTypeNames returns the scope types a binding may name, sorted.
func ScopeTypeNames() []string { return slices.Clone(scopeTypes) }

// IsKnownLevel reports whether level is a level of the swarm.
func IsKnownLevel(level string) bool {
	return slices.Contains(LevelNames(), level)
}

// IsReservedLevel reports whether level is kept for later epics (monitor,
// object): a blueprint of it is valid, but no agent of it is spawned in MVP-1.
func IsReservedLevel(level string) bool {
	return level == LevelNameMonitor || level == LevelNameObject
}

// RoleLevel returns the level a role belongs to.
func RoleLevel(role string) (string, bool) {
	spec, ok := roles[role]
	return spec.level, ok
}

// ParseLevel returns the AgentLevel of a level name, LevelUnknown for any
// other string.
func ParseLevel(level string) AgentLevel {
	for _, l := range []AgentLevel{LevelGlobal, LevelDomain, LevelTask, LevelObject, LevelMonitor} {
		if l.String() == level {
			return l
		}
	}
	return LevelUnknown
}

// AllowedEventTypes returns the event types an agent of the role may publish,
// sorted. It is nil when the role publishes nothing (the reserved levels),
// when the role is unknown or when it does not belong to the level: a white
// list is given to a pair that makes sense, never guessed for one that does
// not.
func AllowedEventTypes(level, role string) []string {
	spec, ok := roles[role]
	if !ok || spec.level != level {
		return nil
	}
	events := slices.Clone(spec.events)
	slices.Sort(events)
	return events
}

// roleScopeTypes returns the scope types of a role, nil when the role does
// not constrain them.
func roleScopeTypes(role string) []string {
	return roles[role].scopeTypes
}
