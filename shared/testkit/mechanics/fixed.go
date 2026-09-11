// Package mechanics is the stand-in for the fight of MVP-1: everything C-03
// promises, with the one decision EPIC-002 still owes answered from a table
// (design.md §5, tasks.md T-017).
//
// It exists so that the epics downstream of the mechanics can be written, run
// and reviewed before Resolve is implemented. The rules themselves are already
// real — the numbers, the formulas and the dice all come from
// rules/dark-forest.yaml through internal/mechanics — so Roll, Stats and
// Invariants are not faked at all: they are forwarded to the loaded rule set.
// What is faked is which of four things happened when somebody swung
// (FixedMechanics.Resolve) and whom a wolf bit (NPCTarget).
//
// The methods repeat the signatures of *mechanics.Rules rather than hiding
// behind an interface of their own. That is the point of the stub: the
// consumer declares the interface it needs on its own side (design.md §11,
// rejected alternative "an interface in C-03"), both this type and the real
// rule set satisfy it, and swapping one for the other is a change of one
// constructor call — nothing a consumer wrote has to move. The consumer test
// of this package is what holds that promise to a compiler rather than to a
// reviewer.
package mechanics

import (
	"cmp"
	"fmt"
	"slices"

	mech "multiverse-core.io/internal/mechanics"
)

// FixedMechanics is a loaded rule set whose Resolve answers from a table.
//
// It carries no state of its own: two calls with the same cause event and the
// same index answer the same thing, on any machine and in any order, which is
// what lets a recorded run be compared with a fresh one exactly as the real
// mechanics allow (ADR-012 p. 3).
type FixedMechanics struct {
	rules *mech.Rules
}

// New wraps a loaded rule set. The rules are not copied: they are immutable
// after Load and one instance serves every caller.
func New(rules *mech.Rules) *FixedMechanics { return &FixedMechanics{rules: rules} }

// Load reads a rules file and wraps it, for a caller that wants the stub and
// has no rule set in hand (rules/dark-forest.yaml in every test of MVP-1).
func Load(path string) (*FixedMechanics, error) {
	rules, err := mech.Load(path)
	if err != nil {
		return nil, err
	}
	return New(rules), nil
}

// Rules is the rule set behind the stub: the version, the numbers and the
// formulas, for a caller that has to report rules_version or read a table.
func (m *FixedMechanics) Rules() *mech.Rules { return m.rules }

// tableSize is the number of slots the outcome table has. Ten is what makes
// the shares of design.md §5 whole slots: six hits, one critical, one fumble,
// two misses.
const tableSize = 10

// verdict is what the table says happened.
type verdict uint8

const (
	verdictHit verdict = iota
	verdictCritical
	verdictFumble
	verdictMiss
)

// The names Verdict reports a row of the table under.
const (
	VerdictHit      = "hit"
	VerdictCritical = "critical"
	VerdictFumble   = "fumble"
	VerdictMiss     = "miss"
)

// table is the outcome table itself: 60 % a hit, 10 % a critical, 10 % a
// fumble, 20 % a miss (design.md §5, tasks.md T-017).
//
// The slot is the seed of the first roll of the action modulo the size of the
// table, so the answer is addressed by (cause event, roll index) exactly as a
// real roll is: the same swing decides the same way on a replay, and two
// swings of one event — different indices — decide independently.
var table = [tableSize]verdict{
	verdictHit, verdictHit, verdictHit, verdictHit, verdictHit, verdictHit,
	verdictCritical,
	verdictFumble,
	verdictMiss, verdictMiss,
}

var verdictNames = map[verdict]string{
	verdictHit:      VerdictHit,
	verdictCritical: VerdictCritical,
	verdictFumble:   VerdictFumble,
	verdictMiss:     VerdictMiss,
}

// Verdict is the row of the table an action addressed this way falls on. It is
// exported so that a test can state what it expects to happen without
// repeating the arithmetic of the table, and so that a scenario can pick the
// cause event that lands on the outcome it wants to demonstrate.
func Verdict(causeEventID string, rollIndex int) string {
	return verdictNames[slot(causeEventID, rollIndex)]
}

func slot(causeEventID string, rollIndex int) verdict {
	return table[mech.Seed(causeEventID, rollIndex)%tableSize]
}

// Resolve decides one action from the table (C-03).
//
// The shape of the answer is the real one: the rolls it returns are what the
// caller publishes as dice.rolled before combat.decided, the outcome is what
// combat.decided carries, and the hit points are already clamped into
// [0, hp_max] the way inv-02 demands. The actors are not touched.
//
// The table decides the verdict alone. Everything the verdict implies is then
// computed from the rules: the damage is a real roll of the damage dice of the
// attacker, doubled on a critical by the multiplier of the rules, and the
// natural die reported with the hit is chosen so that it agrees with the
// verdict — a critical shows the critical natural, a fumble the fumble
// natural, a hit a number that clears the defence of the target and a miss one
// that does not. A consumer that renders "выпало 12, попадание" therefore
// renders something a player can believe.
//
// Two things the real Resolve will do are deliberately absent, because the
// contract gives the stub nothing to do them with: Outcome.Loot is always
// empty (an Actor carries no kind to look the loot table up by — see the
// dev-log of T-017), and a rest rolls nothing (rest.restore of v0.1 is hp_max).
func (m *FixedMechanics) Resolve(causeEventID string, rollIndexStart int, a mech.Action,
	actors map[string]*mech.Actor) (mech.Outcome, []mech.Roll, error) {
	if causeEventID == "" {
		return mech.Outcome{}, nil, fmt.Errorf("testkit/mechanics: resolve without a cause event")
	}
	if rollIndexStart < 0 {
		return mech.Outcome{}, nil, fmt.Errorf("testkit/mechanics: roll index %d is negative", rollIndexStart)
	}
	actor, err := lookup(actors, a.Actor, "actor")
	if err != nil {
		return mech.Outcome{}, nil, err
	}

	switch a.Kind {
	case mech.ActionAttack:
		return m.attack(causeEventID, rollIndexStart, a, actor, actors,
			mech.PurposeHit, mech.PurposeDamage)
	case mech.ActionNPCAttack, mech.ActionFreeAttack:
		return m.attack(causeEventID, rollIndexStart, a, actor, actors,
			mech.PurposeNPCHit, mech.PurposeNPCDamage)
	case mech.ActionFlee:
		return m.flee(causeEventID, rollIndexStart, a, actor)
	case mech.ActionRest:
		return m.rest(causeEventID, rollIndexStart, actor)
	default:
		return mech.Outcome{}, nil, fmt.Errorf("testkit/mechanics: unknown action kind %q", a.Kind)
	}
}

// attack answers a strike, whoever swings. free_attack differs from the answer
// of an NPC in when it happens and not in how it is decided (§5.4), so both
// come through here and differ only in the purposes their rolls carry.
func (m *FixedMechanics) attack(causeEventID string, idx int, a mech.Action,
	attacker *mech.Actor, actors map[string]*mech.Actor,
	hitPurpose, damagePurpose string) (mech.Outcome, []mech.Roll, error) {
	target, err := lookup(actors, a.Target, "target")
	if err != nil {
		return mech.Outcome{}, nil, err
	}
	if m.rules.Excluded(*target) {
		return mech.Outcome{}, nil, fmt.Errorf("testkit/mechanics: %s: %w", target.ID, mech.ErrInvalidTarget)
	}
	check, ok := m.rules.Check(mech.CheckAttackHit)
	if !ok {
		return mech.Outcome{}, nil, fmt.Errorf("testkit/mechanics: the rules have no %s", mech.CheckAttackHit)
	}

	v := slot(causeEventID, idx)
	natural := m.naturalFor(v, *attacker, *target)
	rolls := []mech.Roll{{
		Index:   idx,
		Formula: check.Dice.String(),
		Seed:    mech.Seed(causeEventID, idx),
		Result:  natural + check.Dice.Modifier,
		Natural: natural,
		Purpose: hitPurpose,
	}}

	out := mech.Outcome{
		Hit:        v == verdictHit || v == verdictCritical,
		Critical:   v == verdictCritical,
		Fumble:     v == verdictFumble,
		Natural:    natural,
		HPBefore:   target.HP,
		HPAfter:    target.HP,
		FreeAttack: a.Kind == mech.ActionFreeAttack,
	}
	if !out.Hit {
		// The index of the damage roll is spent whether or not it happens, so
		// that the second swing of one cause event starts where it would have
		// started on a hit (AttackDoc.Rolls).
		return out, rolls, nil
	}

	dice, err := m.rules.DamageDice(*attacker)
	if err != nil {
		return mech.Outcome{}, nil, fmt.Errorf("testkit/mechanics: %w", err)
	}
	damage, err := m.rules.Roll(causeEventID, idx+1, dice.String(), damagePurpose)
	if err != nil {
		return mech.Outcome{}, nil, fmt.Errorf("testkit/mechanics: %w", err)
	}
	rolls = append(rolls, damage)

	out.Damage = m.rules.Damage(damage.Result, out.Critical)
	out.HPAfter = m.rules.ClampHP(target.HP-out.Damage, target.HPMax)
	out.TargetDead = out.HPAfter == 0
	return out, rolls, nil
}

// naturalFor is the die the verdict has to have shown. A critical and a fumble
// are the naturals the rules name; a hit is the lowest die that still clears
// the defence of the target and a miss the highest that does not, both held
// away from the two naturals that decide on their own.
//
// When the arithmetic cannot produce such a die — an attacker who cannot miss
// this target without fumbling, or cannot hit it without a critical — the
// verdict of the table still stands and the natural is the nearest legal one.
// The stub is a table, not a simulation, and a consumer reads the verdict from
// Outcome rather than recomputing it from the die.
func (m *FixedMechanics) naturalFor(v verdict, attacker, target mech.Actor) int {
	attack := m.rules.Attack()
	low, high := attack.FumbleNatural+1, attack.CritNatural-1
	switch v {
	case verdictCritical:
		return attack.CritNatural
	case verdictFumble:
		return attack.FumbleNatural
	case verdictHit:
		return clamp(target.Def-attacker.Atk, low, high)
	default:
		return clamp(target.Def-attacker.Atk-1, low, high)
	}
}

// flee answers a flight attempt. The table decides it like a strike — a hit or
// a critical is an escape, a fumble or a miss is not — and what a failure
// costs comes from the rules (flee.on_fail).
func (m *FixedMechanics) flee(causeEventID string, idx int, a mech.Action,
	actor *mech.Actor) (mech.Outcome, []mech.Roll, error) {
	check, ok := m.rules.Check(mech.CheckFlee)
	if !ok {
		return mech.Outcome{}, nil, fmt.Errorf("testkit/mechanics: the rules have no %s", mech.CheckFlee)
	}
	threshold, err := m.rules.FleeThreshold(*actor, a.LivingEnemies)
	if err != nil {
		return mech.Outcome{}, nil, fmt.Errorf("testkit/mechanics: %w", err)
	}

	v := slot(causeEventID, idx)
	success := v == verdictHit || v == verdictCritical
	bonus, _ := actor.Attr(mech.IdentFlee)
	natural := clamp(threshold-bonus, 1, check.Dice.Sides)
	if !success {
		natural = clamp(threshold-bonus-1, 1, check.Dice.Sides)
	}

	rolls := []mech.Roll{{
		Index:   idx,
		Formula: check.Dice.String(),
		Seed:    mech.Seed(causeEventID, idx),
		Result:  natural + check.Dice.Modifier,
		Natural: natural,
		Purpose: mech.PurposeFlee,
	}}
	return mech.Outcome{
		Success:    &success,
		Natural:    natural,
		Threshold:  threshold,
		HPBefore:   actor.HP,
		HPAfter:    actor.HP,
		FreeAttack: !success && m.rules.Flee().OnFail == mech.OnFailFreeAttack,
	}, rolls, nil
}

// rest answers catching a breath. There is nothing for the table to decide: a
// rest of the rules of v0.1 restores hit points to the maximum and cannot
// fail, and whether it was allowed at all is a decision of the gateway and of
// State, not of the mechanics (rest.allowed_in_encounter).
func (m *FixedMechanics) rest(causeEventID string, idx int, actor *mech.Actor) (mech.Outcome, []mech.Roll, error) {
	return mech.Outcome{
		Hit:      true,
		HPBefore: actor.HP,
		HPAfter:  m.rules.Restore(*actor, mech.NewRNG(mech.Seed(causeEventID, idx))),
	}, nil, nil
}

// NPCTarget picks the first living candidate by identifier.
//
// The real order of C-03 is last_damager, then the fewest hit points, then the
// identifier; the stub keeps only the last of the three, because a table-driven
// fight has no history to prefer a last damager by and picking the weakest
// would make a scripted scenario depend on the damage the table happened to
// roll. Anyone the rules exclude is not a candidate at all: the dead, the
// abandoned, the idle and those out of combat (inv-01, C-02 v1.2).
func (m *FixedMechanics) NPCTarget(npc *mech.Actor, candidates []*mech.Actor) *mech.Actor {
	living := make([]*mech.Actor, 0, len(candidates))
	for _, c := range candidates {
		if c == nil || m.rules.Excluded(*c) {
			continue
		}
		living = append(living, c)
	}
	if len(living) == 0 {
		return nil
	}
	slices.SortFunc(living, func(x, y *mech.Actor) int { return cmp.Compare(x.ID, y.ID) })
	return living[0]
}

// Roll throws one formula for one purpose. It is not faked: the dice of the
// rules are already deterministic and addressed by their cause (C-03 v1.1).
func (m *FixedMechanics) Roll(causeEventID string, rollIndex int, formula, purpose string) (mech.Roll, error) {
	return m.rules.Roll(causeEventID, rollIndex, formula, purpose)
}

// Stats are the numbers a kind of actor starts with, straight out of
// rules/dark-forest.yaml.
func (m *FixedMechanics) Stats(kind string) (mech.Actor, bool) { return m.rules.Stats(kind) }

// Invariants are the laws in force, straight out of the rule set — every Check
// still nil until EPIC-002 writes them (T-054).
func (m *FixedMechanics) Invariants() []mech.Invariant { return m.rules.Invariants() }

func lookup(actors map[string]*mech.Actor, id, role string) (*mech.Actor, error) {
	if id == "" {
		return nil, fmt.Errorf("testkit/mechanics: action without %s", role)
	}
	a, ok := actors[id]
	if !ok || a == nil {
		return nil, fmt.Errorf("testkit/mechanics: %s %s is not among the actors", role, id)
	}
	return a, nil
}

func clamp(v, low, high int) int { return min(max(v, low), high) }
