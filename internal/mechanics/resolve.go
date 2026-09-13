package mechanics

import (
	"fmt"

	"multiverse-core.io/shared/entity"
)

// Resolve decides one action: it rolls what the rules say to roll and reports
// what happened, without touching the actors it was given (C-03, §5.4).
//
// The rolls are addressed from rollIndexStart inside causeEventID, one index
// per roll the action may make — including the damage roll of a strike that
// missed, whose index is spent all the same, so that the next action of the
// same cause starts where it would have started on a hit (AttackDoc.Rolls).
// The caller publishes one dice.rolled per returned Roll before the decision
// they rest on.
//
// What each kind does (§5.4):
//
//   - attack is a character striking an NPC: hit, then damage on a hit.
//   - npc_attack and free_attack are an NPC biting a character, decided the
//     same way with the purposes npc_hit and npc_damage. A free attack is the
//     answer to a failed flight and says so in Outcome.FreeAttack.
//   - flee is one roll against a threshold that grows with the enemies still
//     standing. It decides only the flight: when it fails and the rules call
//     for a free attack, Outcome.FreeAttack says so and the caller resolves
//     the free attack itself, from the next index, against the target
//     NPCTarget picks — Resolve is not told who the enemy is.
//   - rest restores hit points and rolls nothing under the rules of v0.1. It
//     is a decision of the mechanics but not of a fight: combat.decided has no
//     rest in its action enum, the gateway proposes it with cause=rest
//     (C-02 v1.1, C-03 "Отдых"), and whether it is allowed inside an encounter
//     is checked by the gateway and State, not here.
//
// An error is a defect of the caller or of its inputs, never an outcome: an
// action by the dead, a strike at a target the rules exclude (ErrInvalidTarget),
// a character biting a character. A miss is an outcome.
func (r *Rules) Resolve(causeEventID string, rollIndexStart int, a Action, actors map[string]*Actor) (Outcome, []Roll, error) {
	if causeEventID == "" {
		return Outcome{}, nil, fmt.Errorf("mechanics: resolve without a cause event")
	}
	if rollIndexStart < 0 {
		return Outcome{}, nil, fmt.Errorf("mechanics: roll index %d is negative", rollIndexStart)
	}
	actor, err := actorOf(actors, a.Actor, "actor")
	if err != nil {
		return Outcome{}, nil, err
	}
	// inv-01: the dead do not act. Idle and out of combat do not stop a
	// character from acting — acting is how an idle one comes back.
	if !actor.Alive() {
		return Outcome{}, nil, fmt.Errorf("mechanics: %s %s is %s and does not act", actor.Type, actor.ID, actor.Status)
	}

	switch a.Kind {
	case ActionAttack:
		return r.strike(causeEventID, rollIndexStart, a, actor, actors,
			entity.TypePlayer, PurposeHit, PurposeDamage)
	case ActionNPCAttack, ActionFreeAttack:
		return r.strike(causeEventID, rollIndexStart, a, actor, actors,
			entity.TypeNPC, PurposeNPCHit, PurposeNPCDamage)
	case ActionFlee:
		return r.flee(causeEventID, rollIndexStart, a, actor)
	case ActionRest:
		return r.rest(actor)
	default:
		return Outcome{}, nil, fmt.Errorf("mechanics: unknown action kind %q: want one of %v", a.Kind, ActionKinds)
	}
}

// strike decides a blow of either side. The two sides differ in who may swing
// and in the purposes their rolls carry, which is what dice.rolled reports; the
// arithmetic is the one of attack, which npc_attack inherits (rules
// npc_attack.inherit).
func (r *Rules) strike(cause string, idx int, a Action, attacker *Actor, actors map[string]*Actor,
	attackerType, hitPurpose, damagePurpose string) (Outcome, []Roll, error) {
	if attacker.Type != attackerType {
		return Outcome{}, nil, fmt.Errorf("mechanics: %s is dealt by a %s, and %s is a %s",
			a.Kind, attackerType, attacker.ID, attacker.Type)
	}
	target, err := actorOf(actors, a.Target, "target")
	if err != nil {
		return Outcome{}, nil, err
	}
	if target.ID == attacker.ID {
		return Outcome{}, nil, fmt.Errorf("mechanics: %s %s strikes itself", attacker.Type, attacker.ID)
	}
	if wantTarget := opponentOf(attackerType); target.Type != wantTarget {
		return Outcome{}, nil, fmt.Errorf("mechanics: %s of %s is aimed at a %s, and %s is a %s",
			a.Kind, attacker.ID, wantTarget, target.ID, target.Type)
	}
	if r.Excluded(*target) {
		return Outcome{}, nil, fmt.Errorf("mechanics: %s %s (status %s, participation %q): %w",
			target.Type, target.ID, target.Status, target.Participation, ErrInvalidTarget)
	}

	hit, check, err := r.RollCheck(cause, idx, r.checks[CheckAttackHit], hitPurpose, *attacker, *target, nil)
	if err != nil {
		return Outcome{}, nil, err
	}
	rolls := []Roll{hit}

	out := Outcome{
		Natural:    hit.Natural,
		Fumble:     r.Fumble(hit.Natural),
		HPBefore:   target.HP,
		HPAfter:    r.ClampHP(target.HP, target.HPMax),
		FreeAttack: a.Kind == ActionFreeAttack,
	}
	// A natural fumble misses however high the sum; a natural crit hits however
	// low. Load makes sure the two faces differ, so the order only reads.
	out.Critical = !out.Fumble && r.Critical(hit.Natural)
	out.Hit = !out.Fumble && (out.Critical || check.OK)
	if !out.Hit {
		return out, rolls, nil
	}

	dice, err := r.DamageDice(*attacker)
	if err != nil {
		return Outcome{}, nil, err
	}
	damage, err := r.Roll(cause, idx+1, dice.String(), damagePurpose)
	if err != nil {
		return Outcome{}, nil, err
	}
	rolls = append(rolls, damage)

	out.Damage = r.Damage(damage.Result, out.Critical)
	out.HPAfter = r.ClampHP(target.HP-out.Damage, target.HPMax)
	out.TargetDead = out.HPAfter == 0
	if out.TargetDead && target.Type == entity.TypeNPC {
		out.Loot = r.Loot(target.Kind)
	}
	return out, rolls, nil
}

// flee decides a flight attempt against 10 + the enemies still standing, as
// flee.check writes it. There is no target: the right side of the check reads
// the fleeing character itself and the context of the turn, as FleeThreshold
// does, so the threshold reported and the threshold rolled against are one
// number.
func (r *Rules) flee(cause string, idx int, a Action, actor *Actor) (Outcome, []Roll, error) {
	if a.LivingEnemies < 0 {
		return Outcome{}, nil, fmt.Errorf("mechanics: flee of %s against %d living enemies", actor.ID, a.LivingEnemies)
	}
	roll, check, err := r.RollCheck(cause, idx, r.checks[CheckFlee], PurposeFlee, *actor, *actor,
		map[string]int{IdentLivingEnemies: a.LivingEnemies})
	if err != nil {
		return Outcome{}, nil, err
	}
	success := check.OK
	return Outcome{
		Success:    &success,
		Natural:    roll.Natural,
		Threshold:  check.Threshold,
		HPBefore:   actor.HP,
		HPAfter:    actor.HP,
		FreeAttack: !success && r.doc.Flee.OnFail == OnFailFreeAttack,
	}, []Roll{roll}, nil
}

// rest decides catching a breath. The only rest Load admits restores to the
// maximum (rest.restore = hp_max), so it cannot fail and rolls nothing, and Hit
// carries "it worked" as it does for every action but a flight
// (Outcome.Success).
func (r *Rules) rest(actor *Actor) (Outcome, []Roll, error) {
	return Outcome{
		Hit:      true,
		HPBefore: actor.HP,
		HPAfter:  r.Restore(*actor),
	}, nil, nil
}

func actorOf(actors map[string]*Actor, id, role string) (*Actor, error) {
	if id == "" {
		return nil, fmt.Errorf("mechanics: action without %s", role)
	}
	a, ok := actors[id]
	if !ok || a == nil {
		return nil, fmt.Errorf("mechanics: %s %s is not among the actors", role, id)
	}
	if a.ID != id {
		return nil, fmt.Errorf("mechanics: actors[%s] is %s", id, a.ID)
	}
	return a, nil
}

// opponentOf is whom a side of MVP-1 fights: characters strike NPCs and NPCs
// bite characters. There is no fight among characters or among NPCs in the
// rules of v0.1.
func opponentOf(side string) string {
	if side == entity.TypePlayer {
		return entity.TypeNPC
	}
	return entity.TypePlayer
}
