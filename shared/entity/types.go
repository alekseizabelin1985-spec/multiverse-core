package entity

import "slices"

// The entity types of MVP-1 (data-model.md §3, state-and-mechanics.md §3.1).
// city, item and actor are named in the target model and are deliberately not
// here: a constant that nothing writes is a promise the platform has not made.
const (
	TypeWorld     = "world"
	TypeRegion    = "region"
	TypePlayer    = "player"
	TypeNPC       = "npc"
	TypeGroup     = "group"
	TypeEncounter = "encounter"
)

// Types is the whole set, in the order a bootstrap creates them: a world before
// its regions, a region before what stands in it (state-and-mechanics.md §4.10).
var Types = []string{TypeWorld, TypeRegion, TypeNPC, TypePlayer, TypeGroup, TypeEncounter}

// ValidType says whether a type is one the platform knows.
func ValidType(t string) bool { return slices.Contains(Types, t) }

// The status of a character or an NPC (data-model.md §3.3, §3.4).
//
// dead, abandoned and ascended_final are terminal: nothing moves out of them.
// abandoned is what /forget leaves behind (FR-061) and reads as dead everywhere
// that matters — the dead_entity rule, inv-01, the target list of an NPC, the
// membership of a scope — with one exception: no narrative.output kind=death is
// written for it (C-02 v1.2, consolidation.md §14 З-2).
const (
	StatusAlive         = "alive"
	StatusDead          = "dead"
	StatusAbandoned     = "abandoned"
	StatusAscendedFinal = "ascended_final"
)

// TerminalStatuses are the statuses an entity never leaves.
var TerminalStatuses = []string{StatusDead, StatusAbandoned, StatusAscendedFinal}

// IsTerminalStatus says whether a status is one of the three that end a life.
func IsTerminalStatus(status string) bool { return slices.Contains(TerminalStatuses, status) }

// IsTerminal says whether the entity is past every change but the four paths
// State still allows on a corpse (state-and-mechanics.md §4.5 p. 5).
func (e *Entity) IsTerminal() bool {
	status, _ := e.Status()
	return IsTerminalStatus(status)
}

// StatusTransitionAllowed says whether a status may move from one value to
// another (data-model.md §3.3, §9.1; C-02 v1.2).
//
// Out of a terminal status: never. dead -> alive is the one the invariant
// inv-09 names explicitly, and the same answer covers abandoned and
// ascended_final. Into abandoned: only from alive, and only the gateway may
// propose it (cause=forget) — which of the two rules is broken decides whether
// State answers dead_entity or level_violation, and that decision is State's.
func StatusTransitionAllowed(from, to string) bool {
	if from == to {
		return false
	}
	if IsTerminalStatus(from) {
		return false
	}
	return slices.Contains(TerminalStatuses, to) || to == StatusAlive
}

// How a session behind an entity is driven (data-model.md §3.3). It is fixed at
// creation and never changes: a CI run must stay distinguishable from a person
// in every analytics record it produces.
const (
	ActorKindHuman = "human"
	ActorKindCI    = "ci"
	ActorKindSim   = "sim"
)

// The state of a group (data-model.md §3.6).
const (
	GroupStateForming   = "forming"
	GroupStateActive    = "active"
	GroupStateDisbanded = "disbanded"
)

// How a member takes part in a round (C-03 Actor.Participation, data-model.md §3.6).
const (
	ParticipationActive      = "active"
	ParticipationIdle        = "idle"
	ParticipationOutOfCombat = "out_of_combat"
)

// The state of an encounter and how it ended (data-model.md §3.7).
const (
	EncounterStateActive   = "active"
	EncounterStateResolved = "resolved"

	ResolutionNPCDead    = "npc_dead"
	ResolutionPlayersOut = "players_out"
	ResolutionAbandoned  = "abandoned"
)

// Attribute names. They are here so that an op, a fixture and a getter cannot
// disagree about a spelling; the meaning of each one is in data-model.md §3.
const (
	// Common to a character and an NPC.
	AttrHP          = "hp"
	AttrHPMax       = "hp_max"
	AttrAtk         = "atk"
	AttrDef         = "def"
	AttrDmg         = "dmg"
	AttrFlee        = "flee"
	AttrStatus      = "status"
	AttrPosition    = "position"
	AttrScope       = "scope"
	AttrInventory   = "inventory"
	AttrActorKind   = "actor_kind"
	AttrGroupID     = "group_id"
	AttrEncounterID = "encounter_id"

	// NPC.
	AttrKind     = "kind"
	AttrRegionID = "region_id"
	AttrDiedAt   = "died_at"
	AttrKilledBy = "killed_by"
	// AttrLootClaimedBy is the fourth path of a corpse, and it was the only one
	// written as a literal — in two doubles independently, which is how a typo
	// becomes a silent divergence between them (T-017, T-219).
	AttrLootClaimedBy = "loot_claimed_by"
	AttrLoot          = "loot"
	AttrSpawnedBy     = "spawned_by"

	// World.
	AttrLawsVersion = "laws_version"
	AttrWeather     = "weather"
	AttrTimeOfDay   = "time_of_day"
	AttrDay         = "day"
	AttrSeason      = "season"
	AttrEpoch       = "epoch"
	AttrLocale      = "locale"

	// Region.
	AttrDescription           = "description"
	AttrCanon                 = "canon"
	AttrNPCIDs                = "npc_ids"
	AttrRespawnTTL            = "respawn_ttl"
	AttrPerceptionRadius      = "perception_radius"
	AttrPlayersPresent        = "players_present"
	AttrEncounterChance       = "encounter_chance"
	AttrLastBackgroundEventAt = "last_background_event_at"

	// Group and encounter.
	AttrLeaderID     = "leader_id"
	AttrMembers      = "members"
	AttrState        = "state"
	AttrParticipants = "participants"
	AttrNPCs         = "npcs"
	AttrResolution   = "resolution"
	AttrRoundSeq     = "round_seq"
	AttrTaskAgentID  = "task_agent_id"

	AttrOpenedByEventID = "opened_by_event_id"
	AttrClosedByEventID = "closed_by_event_id"

	// Written by a blueprint, read by the swarm.
	AttrBlueprintRef = "blueprint_ref"
)
