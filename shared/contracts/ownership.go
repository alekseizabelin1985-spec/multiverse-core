package contracts

import "slices"

// Proposers of entity changes: the three agent levels of the swarm, the two
// reserved levels of C-13, and the three non-agent proposers (the gateway on
// behalf of a person, the author through mvctl, the bootstrap of a world).
const (
	ProposerGateway = "gateway"
	ProposerAuthor  = "author"
	ProposerSystem  = "system"
	ProposerGlobal  = "global"
	ProposerDomain  = "domain"
	ProposerTask    = "task"
	ProposerObject  = "object"
	ProposerMonitor = "monitor"
)

// AnyPath and AnyCause are the wildcards of the table below.
const (
	AnyPath  = "*"
	AnyCause = "*"
	AnyType  = "*"
)

// OwnershipRule says what one proposer may change (state-and-mechanics.md
// §4.6, data-model.md §4). State refuses anything the table does not allow
// with entity.update.rejected reason=level_violation.
type OwnershipRule struct {
	// Proposer is an agent level (meta.agent.level) or one of gateway, author,
	// system for a proposal without an agent.
	Proposer string
	// EntityTypes lists the entity types the rule covers; AnyType is any.
	EntityTypes []string
	// Paths lists the allowed path prefixes; AnyPath is any. A path matching
	// none of them is refused.
	Paths []string
	// Causes lists the allowed values of payload.cause; AnyCause is any.
	Causes []string
	// Create reports whether the proposer may create entities of these types
	// with entity.create.proposed.
	Create bool
}

// ownershipRules is the ownership table itself — its only source, not a copy
// of one kept elsewhere (ADR-025, C-02 v1.4, contracts.md §16 p. 6). A row is
// changed by the owner of its meaning (the swarm levels by EPIC-003, gateway
// by EPIC-004, author and system by EPIC-002) in a pull request marked
// contract-change. The blueprint validator of shared/agent gets its view of
// the table from its caller, built from OwnershipRules; there is nothing to
// compare this table with.
//
// Two conditions of §4.6 do not fit the shape of the rule and stay with State,
// which checks them on top of the table:
//   - gateway may write hp of a player only with cause=rest, only outside an
//     encounter and only up to hp_max;
//   - gateway may write status of a player only as the transition
//     alive → abandoned with cause=forget (C-02 v1.2), and abandoned is
//     terminal like dead.
//
// The exclusions of the "*" rows are the same kind of condition: domain may
// change everything of a region except description, and everything of an NPC
// except lowering hp while a player is alive in the encounter; status
// dead → alive is refused always (inv-09).
var ownershipRules = []OwnershipRule{
	{
		Proposer:    ProposerGateway,
		EntityTypes: []string{"player"},
		Paths:       []string{"position", "scope", "group_id", "encounter_id", "hp", "status"},
		Causes:      []string{"move", "group", "rest", "create", "leave", "forget"},
		Create:      true,
	},
	{
		// Everything of a group except encounter_id, which only the encounter
		// agent writes.
		Proposer:    ProposerGateway,
		EntityTypes: []string{"group"},
		Paths:       []string{AnyPath},
		Causes:      []string{"group", "move", "forget"},
		Create:      true,
	},
	{
		Proposer:    ProposerTask,
		EntityTypes: []string{"player"},
		Paths:       []string{"hp", "status", "position", "inventory", "encounter_id"},
		Causes:      []string{"combat", "loot", "flee", "death"},
	},
	{
		Proposer:    ProposerTask,
		EntityTypes: []string{"npc"},
		Paths:       []string{"hp", "status", "died_at", "killed_by", "loot_claimed_by"},
		Causes:      []string{"combat", "death"},
	},
	{
		Proposer:    ProposerTask,
		EntityTypes: []string{"encounter"},
		Paths:       []string{AnyPath},
		Causes:      []string{"combat", "flee", "group", "death", "resolve"},
	},
	{
		// Everything of a region except description. The "create npc,
		// encounter" of the same row of §4.6 lives in the two rows below,
		// whose entity types it names.
		Proposer:    ProposerDomain,
		EntityTypes: []string{"region"},
		Paths:       []string{AnyPath},
		Causes:      []string{"tick", "spawn"},
	},
	{
		Proposer:    ProposerDomain,
		EntityTypes: []string{"npc"},
		Paths:       []string{AnyPath},
		Causes:      []string{"tick", "spawn"},
		Create:      true,
	},
	{
		Proposer:    ProposerDomain,
		EntityTypes: []string{"encounter"},
		Paths:       []string{"state", "participants", "npcs"},
		Causes:      []string{"spawn"},
		Create:      true,
	},
	{
		Proposer:    ProposerGlobal,
		EntityTypes: []string{"world"},
		Paths:       []string{"weather", "time_of_day", "day", "season", "epoch"},
		Causes:      []string{"tick"},
	},
	{
		Proposer:    ProposerAuthor,
		EntityTypes: []string{AnyType},
		Paths:       []string{AnyPath},
		Causes:      []string{"init", "author"},
		Create:      true,
	},
	{
		Proposer:    ProposerSystem,
		EntityTypes: []string{AnyType},
		Paths:       []string{AnyPath},
		Causes:      []string{"init", "author"},
		Create:      true,
	},
	// Reserved by C-13 and empty on purpose: the object agents of Living
	// Worlds and the monitor agent propose nothing until their spawn flag is
	// turned on, and an empty rule refuses everything.
	{Proposer: ProposerObject},
	{Proposer: ProposerMonitor},
}

// OwnershipRules returns a copy of the ownership table, so that no caller can
// rewrite it through the slice. State is the only consumer that refuses by it
// (contracts.md C-02, ADR-025).
func OwnershipRules() []OwnershipRule {
	rules := make([]OwnershipRule, len(ownershipRules))
	for i, rule := range ownershipRules {
		rules[i] = OwnershipRule{
			Proposer:    rule.Proposer,
			EntityTypes: slices.Clone(rule.EntityTypes),
			Paths:       slices.Clone(rule.Paths),
			Causes:      slices.Clone(rule.Causes),
			Create:      rule.Create,
		}
	}
	return rules
}
