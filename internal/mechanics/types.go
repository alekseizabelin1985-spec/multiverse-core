// Package mechanics is the rule book of a world and the only source of chance
// in the platform (C-03, ADR-012).
//
// Three properties make it what it is.
//
// The rules are data. Every number a fight rests on — how hard a wolf hits,
// what a character has to beat, how long a round waits — lives in
// rules/dark-forest.yaml and not in a switch statement. Changing the balance of
// the world is a change to a file under review, not a release of Go code
// (FR-020).
//
// The functions are pure. Nothing here reads a clock, an environment variable
// or a file after Load; nothing here writes to the bus or to the object store.
// Given the same rules, the same cause event and the same actors, every
// function in this package answers the same thing on every machine and on every
// replay (ADR-003 p. 5).
//
// Chance is addressed, not drawn. A roll is identified by the event that caused
// it and by its index inside that cause, and its seed is a hash of the two
// (Seed). There is no shared generator for anyone to run ahead of, so an agent
// that re-runs a turn during a replay rolls the same dice it rolled the first
// time.
//
// The package owns no state and enforces no policy: which proposer may touch
// which path and what a rejection is called are decisions of internal/state
// (state-and-mechanics.md §4.5, §4.6).
package mechanics

import (
	"errors"
	"time"

	"multiverse-core.io/shared/entity"
)

// ErrInvalidTarget reports an action aimed at something that cannot be hit: a
// dead or abandoned actor, or one the rules exclude from the round. Picking a
// target is the caller's job (NPCTarget), and this is the answer for skipping
// it (state-and-mechanics.md §5.4, inv-01).
var ErrInvalidTarget = errors.New("mechanics: target cannot be attacked")

// Actor is a fighter as the rules see it: the numbers of a character or an NPC
// and nothing else. It is built from an entity (ActorFromEntity) or from the
// rules themselves (Rules.Stats), and no function of this package mutates one —
// what changes is reported in an Outcome.
type Actor struct {
	ID   string
	Type string // player | npc

	// Kind is the sort of NPC ("wolf") — the key of its loot table, which is
	// how Resolve knows what a fallen NPC leaves behind. Every NPC has one and
	// a character has none (data-model.md §3.4, C-03 v1.3).
	Kind string

	// Version is the version of the entity the actor was read from. It is the
	// expected_version ChangesFor pins on every change of this actor: hp,
	// status and inventory are changed only against a known version
	// (ADR-013 p. 1, C-03 v1.3). Zero means the actor was not read from an
	// entity (Rules.Stats), and ChangesFor refuses to change such an actor.
	Version int64

	HP    int
	HPMax int
	Atk   int
	Def   int

	Dmg  string // dice expression of the damage it deals ("d6")
	Flee string // bonus to a flight attempt, as a plain number; "" — it does not run

	Status string // alive | dead | abandoned | ascended_final

	// Participation is how a character takes part in a round —
	// active | idle | out_of_combat, from encounter.participants[].state.
	// Empty reads as active. An NPC has none.
	Participation string

	// LastDamager is the character who hit this NPC last, from
	// encounter.npcs[].last_damager. It is the first candidate the next attack
	// of the NPC considers (NPCTarget). A character has none.
	LastDamager string
}

// Alive says whether the actor still acts and can still be a target. The three
// terminal statuses answer the same way here: an abandoned character is out of
// the fight exactly as a dead one is, and only the narrative tells them apart
// (C-02 v1.2, state-and-mechanics.md addendum).
func (a Actor) Alive() bool { return a.Status == entity.StatusAlive }

// Attr resolves an identifier of a formula against the actor
// (state-and-mechanics.md §5.3). Flee is a term rather than a dice expression,
// so it reads as a number, and an actor that does not run has no flee at all.
func (a Actor) Attr(name string) (int, bool) {
	switch name {
	case IdentAtk:
		return a.Atk, true
	case IdentDef:
		return a.Def, true
	case IdentHP:
		return a.HP, true
	case IdentHPMax:
		return a.HPMax, true
	case IdentFlee:
		if a.Flee == "" {
			return 0, false
		}
		return parseInt(a.Flee)
	default:
		return 0, false
	}
}

// The action kinds of MVP-1 (C-03, state-and-mechanics.md §5.4).
const (
	ActionAttack     = "attack"
	ActionNPCAttack  = "npc_attack"
	ActionFreeAttack = "free_attack"
	ActionFlee       = "flee"
	ActionRest       = "rest"
)

// ActionKinds is the whole vocabulary; anything else is a defect of the caller.
var ActionKinds = []string{ActionAttack, ActionNPCAttack, ActionFreeAttack, ActionFlee, ActionRest}

// Action is what an actor is trying to do this turn. Actor and Target are
// entity ids into the actors map Resolve is given.
type Action struct {
	Kind   string
	Actor  string
	Target string

	// LivingEnemies raises the bar of a flight attempt: the more of them are
	// still standing, the harder it is to walk away (domain-review §3.3).
	LivingEnemies int

	// At is when the action happened — the timestamp of the event that caused
	// it, set by the caller (ADR-003 p. 6, C-03 v1.3). It is not the wall
	// clock: a replay of the same cause writes the same moment. ChangesFor
	// takes died_at of a fallen NPC and acquired_at of its trophy from it;
	// Resolve does not read it.
	At time.Time
}

// Outcome is what the rules decided. It is the whole answer: the caller writes
// combat.decided from it and asks ChangesFor for the operations that make it
// true in the world — the event decides, it does not change anything.
type Outcome struct {
	Hit        bool
	Critical   bool
	Fumble     bool
	TargetDead bool

	Natural   int // the first die of the check, before any bonus
	Damage    int
	Threshold int // flee only: the number the roll had to beat

	// Success is the result of a flight attempt and is nil for everything
	// else, where Hit carries the answer instead (combat.decided.v1.json).
	Success *bool

	// HPBefore and HPAfter are of the target for an attack and of the actor
	// itself for a rest. HPAfter is already clamped into [0, hp_max] (inv-02).
	HPBefore int
	HPAfter  int

	// Loot is what a dead NPC leaves behind, for the character who landed the
	// last hit (inv-03).
	Loot []Item

	// FreeAttack marks the answer to a failed flight: the NPC strikes out of
	// turn (state-and-mechanics.md §5.4).
	FreeAttack bool
}

// Item is one entry of a loot table: what kind of thing it is and what it is
// called in the language of the world.
type Item struct {
	Kind string `yaml:"kind" json:"kind"`
	Name string `yaml:"name" json:"name"`
}

// Roll is one throw of the dice and everything needed to prove it was not
// invented: the index inside its cause event, the formula, the seed derived
// from the two, and the numbers that came up. The caller publishes one
// dice.rolled per Roll before the decision that rests on them (C-03).
type Roll struct {
	Index   int
	Formula string
	Seed    uint64
	Result  int // the sum of the dice plus the modifier of the formula
	Natural int // the first die alone — what a critical and a fumble are read from
	Purpose string
}

// The purposes a roll can have. They are the enum of dice.rolled.v1.json and
// the values a rules file may name in attack.rolls and flee.rolls.
const (
	PurposeHit             = "hit"
	PurposeDamage          = "damage"
	PurposeFlee            = "flee"
	PurposeNPCHit          = "npc_hit"
	PurposeNPCDamage       = "npc_damage"
	PurposeEncounterChance = "encounter_chance"
	PurposeBackground      = "background"
)

// Purposes is the whole set, in the order of the schema.
var Purposes = []string{
	PurposeHit, PurposeDamage, PurposeFlee,
	PurposeNPCHit, PurposeNPCDamage,
	PurposeEncounterChance, PurposeBackground,
}

// ProposedChange is one entity of an entity.update.proposed as the mechanics
// build it: what to change, at which version, and why. It is not the fact of a
// change — State decides whether the proposal survives ownership and the
// invariants (C-02).
type ProposedChange struct {
	Entity          entity.Ref
	ExpectedVersion *int64
	Ops             []entity.Op
	Cause           string
}

// StateView is the world as an invariant sees it: enough to look around, not
// enough to change anything. State implements it over the copies it is about to
// commit, so a check runs against the world as it would be (§5.7).
type StateView interface {
	Get(id string) (*entity.Entity, bool)
	ByType(t string) []*entity.Entity
	WorldID() string
}

// Violation is one broken invariant on one entity. The message explains the
// break to a developer; the identifier is what State puts into
// entity.update.rejected reason=law_violation.
type Violation struct {
	InvariantID string
	EntityID    string
	Message     string
}

// Invariant is one law of the world in executable form. The identifier is the
// identifier of the law in laws/dark-forest-world.v1.yaml: the text lives
// there, the logic lives here, and a contract test compares the two sets so
// that they cannot drift apart (§5.7, ADR-012 p. 5).
type Invariant struct {
	ID string

	// Where the invariant is enforced: state | mechanics | gateway | guardian
	// | audit. An invariant is often checked in more than one place, and the
	// list is what tells a reader which of them is the one that matters.
	Where []string

	// Check is nil for an invariant no Applier can decide — one that needs the
	// journal rather than the world (inv-07, inv-08) — and for every invariant
	// until EPIC-002 implements it (T-054).
	Check func(v StateView, touched []string) []Violation
}
