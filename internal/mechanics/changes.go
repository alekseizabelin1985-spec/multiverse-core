package mechanics

import (
	"fmt"
	"time"

	"multiverse-core.io/shared/entity"
)

// The causes ChangesFor names its proposals by — the values of cause in
// entity.update.proposed (C-02) that §4.6 gives the task level for a fight, and
// the one the gateway proposes a rest under.
const (
	causeCombat = "combat"
	causeFlee   = "flee"
	causeRest   = "rest"
)

// ChangesFor turns a decision into the operations that make it true: the hit
// points of whoever was struck, the status of whoever fell, the death record of
// a fallen NPC, and the trophy it leaves to the character who landed the last
// hit. The caller wraps them into one entity.update.proposed — one atomic
// package per exchange (C-03, C-05 v1.3 p. 1).
//
// attacker is the actor of the action and target the one it was aimed at; a
// flight and a rest have no target, and target may be nil for them. factEventID
// is the combat.decided the change follows from — the source of a trophy, which
// is how inv-03 proves a trophy was handed out once.
//
// Everything a proposal needs comes from the arguments (C-03 v1.3):
//
//   - expected_version of every change is the Version of the actor it changes.
//     hp, status and inventory are never proposed without one (ADR-013 p. 1),
//     so an actor of version 0 — one not read from an entity — is refused.
//   - died_at of a fallen NPC and acquired_at of its trophy are Action.At, the
//     time of the cause event (ADR-003 p. 6). A package that needs them and has
//     no At is refused rather than sent without them.
//
// The hit points are computed from the target as given, not copied from
// Outcome.HPAfter: the damage of a blow comes off whatever hit points the target
// has when the package is built. On the first attempt the two agree. On a
// package worked out again after a version conflict the target has moved, and
// a blow that would now change who is left standing is refused — the decision
// that says who fell is already published (C-05 v1.4 p. 1в). For the same
// reason, when one package carries several blows on one target, the caller
// passes the target as the previous blow left it — hit points and version —
// for every blow after the first.
//
// Two things of an exchange are not changes of a decision and stay with the
// caller: the position of a character who got away, which the caller writes
// with Rules.FleePosition, and everything of the encounter entity, which the
// package of the encounter agent writes.
//
// An error means the outcome cannot belong to the action, or the arguments
// cannot make a valid proposal: a trophy of a target that did not fall, damage
// without a hit, a flight result on a strike, a blow on somebody already dead,
// an actor without a version, a death without a time.
func ChangesFor(a Action, o Outcome, attacker, target *Actor, factEventID string) ([]ProposedChange, error) {
	if attacker == nil {
		return nil, fmt.Errorf("mechanics: changes for %s without an attacker", a.Kind)
	}
	if a.Actor != attacker.ID {
		return nil, fmt.Errorf("mechanics: changes for %s: the action is of %q, the attacker is %s", a.Kind, a.Actor, attacker.ID)
	}
	if factEventID == "" {
		return nil, fmt.Errorf("mechanics: changes for %s of %s without the fact they follow from", a.Kind, attacker.ID)
	}

	switch a.Kind {
	case ActionAttack, ActionNPCAttack:
		return strikeChanges(a, o, attacker, target, factEventID, causeCombat)
	case ActionFreeAttack:
		// A free attack is the price of a failed flight and is part of its
		// exchange: the package it travels in answers player.flee_attempted.
		return strikeChanges(a, o, attacker, target, factEventID, causeFlee)
	case ActionFlee:
		if o.Success == nil {
			return nil, fmt.Errorf("mechanics: changes for flee of %s: the outcome is not a flight", attacker.ID)
		}
		return nil, nil
	case ActionRest:
		return restChanges(o, attacker)
	default:
		return nil, fmt.Errorf("mechanics: changes for unknown action kind %q", a.Kind)
	}
}

func strikeChanges(a Action, o Outcome, attacker, target *Actor, factEventID, cause string) ([]ProposedChange, error) {
	if target == nil {
		return nil, fmt.Errorf("mechanics: changes for %s of %s without a target", a.Kind, attacker.ID)
	}
	if a.Target != target.ID {
		return nil, fmt.Errorf("mechanics: changes for %s: the action is aimed at %q, the target is %s", a.Kind, a.Target, target.ID)
	}
	switch {
	case o.Success != nil:
		return nil, fmt.Errorf("mechanics: changes for %s of %s: the outcome is a flight", a.Kind, attacker.ID)
	case o.Damage < 0:
		return nil, fmt.Errorf("mechanics: changes for %s of %s: damage %d", a.Kind, attacker.ID, o.Damage)
	case !o.Hit && (o.Damage != 0 || o.TargetDead):
		return nil, fmt.Errorf("mechanics: changes for %s of %s: damage %d, target_dead %v without a hit",
			a.Kind, attacker.ID, o.Damage, o.TargetDead)
	case len(o.Loot) > 0 && (!o.TargetDead || target.Type != entity.TypeNPC || attacker.Type != entity.TypePlayer):
		return nil, fmt.Errorf("mechanics: changes for %s of %s: a trophy of %s %s, which did not fall to a character",
			a.Kind, attacker.ID, target.Type, target.ID)
	}
	if !o.Hit {
		// A miss wounds nobody. What it costs — the round — is written on the
		// encounter by the caller.
		return nil, nil
	}
	if !target.Alive() {
		return nil, fmt.Errorf("mechanics: changes for %s of %s: %s %s is already %s",
			a.Kind, attacker.ID, target.Type, target.ID, target.Status)
	}

	hpAfter := clampHP(target.HP-o.Damage, target.HPMax)
	if fell := hpAfter == 0; fell != o.TargetDead {
		return nil, fmt.Errorf("mechanics: changes for %s of %s: %d damage leaves %s at %d hp, and the decision says target_dead %v",
			a.Kind, attacker.ID, o.Damage, target.ID, hpAfter, o.TargetDead)
	}
	if hpAfter == target.HP && !o.TargetDead {
		// A blow that cost nothing — a damage formula that can roll below one —
		// proposes nothing: a set to the same value would make State publish a
		// fact with an empty changed[].
		return nil, nil
	}

	struck, err := changeOf(target, cause, entity.Op{Op: entity.OpSet, Path: entity.AttrHP, Value: hpAfter})
	if err != nil {
		return nil, err
	}
	if !o.TargetDead {
		return []ProposedChange{struck}, nil
	}

	struck.Ops = append(struck.Ops, entity.Op{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusDead})
	if target.Type != entity.TypeNPC {
		// died_at and killed_by are the death record of an NPC — the respawn
		// cooldown and the trophy (data-model.md §3.4). A character has
		// neither, and §4.6 gives the task level no such path on a player.
		return []ProposedChange{struck}, nil
	}
	at, err := timeOf(a, "died_at of "+target.ID)
	if err != nil {
		return nil, err
	}
	struck.Ops = append(struck.Ops,
		entity.Op{Op: entity.OpSet, Path: entity.AttrDiedAt, Value: at.Format(time.RFC3339)},
		entity.Op{Op: entity.OpSet, Path: entity.AttrKilledBy, Value: attacker.ID},
	)
	if len(o.Loot) == 0 {
		return []ProposedChange{struck}, nil
	}

	struck.Ops = append(struck.Ops, entity.Op{Op: entity.OpSet, Path: entity.AttrLootClaimedBy, Value: attacker.ID})
	trophies := make([]entity.Op, 0, len(o.Loot))
	for _, item := range o.Loot {
		trophies = append(trophies, entity.Op{
			Op:   entity.OpAppend,
			Path: entity.AttrInventory,
			Value: entity.Item{
				// Derived from the decision, so that a redelivered proposal
				// appends nothing twice (entity.ApplyOps deduplicates by it).
				ItemID: factEventID + ":" + item.Kind,
				Kind:   item.Kind,
				Name:   item.Name,
				Source: entity.ItemSource{
					Entity:  entity.Ref{ID: target.ID, Type: target.Type}.EventRef(),
					EventID: factEventID,
				},
				AcquiredAt: at,
			},
		})
	}
	winner, err := changeOf(attacker, cause, trophies...)
	if err != nil {
		return nil, err
	}
	return []ProposedChange{struck, winner}, nil
}

// restChanges writes the hit points a rest restored. A rest that restored
// nothing proposes nothing: a set that changes nothing would make State publish
// a fact with an empty changed[].
func restChanges(o Outcome, actor *Actor) ([]ProposedChange, error) {
	if o.Success != nil {
		return nil, fmt.Errorf("mechanics: changes for rest of %s: the outcome is a flight", actor.ID)
	}
	if !actor.Alive() {
		return nil, fmt.Errorf("mechanics: changes for rest of %s: it is %s", actor.ID, actor.Status)
	}
	hpAfter := clampHP(o.HPAfter, actor.HPMax)
	if hpAfter == actor.HP {
		return nil, nil
	}
	change, err := changeOf(actor, causeRest, entity.Op{Op: entity.OpSet, Path: entity.AttrHP, Value: hpAfter})
	if err != nil {
		return nil, err
	}
	return []ProposedChange{change}, nil
}

// changeOf is one change of one actor, pinned at the version the actor was read
// at. Every path ChangesFor writes — hp, status, inventory and the death record
// that travels with status — is one ADR-013 p. 1 changes only against a known
// version, so an actor without one cannot be changed at all.
func changeOf(who *Actor, cause string, ops ...entity.Op) (ProposedChange, error) {
	if who.Version <= 0 {
		return ProposedChange{}, fmt.Errorf("mechanics: %s %s has version %d: hp, status and inventory are proposed against the version of the entity it was read from",
			who.Type, who.ID, who.Version)
	}
	version := who.Version
	return ProposedChange{
		Entity:          entity.Ref{ID: who.ID, Type: who.Type},
		ExpectedVersion: &version,
		Ops:             ops,
		Cause:           cause,
	}, nil
}

// timeOf is the moment of the action in UTC, for the attributes that record
// when something happened.
func timeOf(a Action, what string) (time.Time, error) {
	if a.At.IsZero() {
		return time.Time{}, fmt.Errorf("mechanics: changes for %s: %s needs Action.At, the time of the cause event", a.Kind, what)
	}
	return a.At.UTC(), nil
}
