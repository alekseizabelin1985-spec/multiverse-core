package mechanics_test

import (
	"testing"

	mech "multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/entity"
	fixed "multiverse-core.io/shared/testkit/mechanics"
)

// This file is the promise of T-017 that the stub can be swapped for the
// implementation of EPIC-002 without a consumer changing a line, held to a
// compiler instead of to a reviewer.
//
// Mechanics is written the way the consumer of C-03 writes it: on its own
// side, over the set of methods it actually calls (design.md §11, rejected
// alternative "an interface in C-03" — the contract stays a set of functions
// and nobody has to publish an interface for a stub to satisfy). Because the
// stub repeats the signatures of *mechanics.Rules exactly, both types satisfy
// it; if a signature of the stub drifted by one argument or one return value,
// this file would stop compiling and the drift would be found here rather than
// in the epic that trusted it.
type Mechanics interface {
	Resolve(causeEventID string, rollIndexStart int, a mech.Action,
		actors map[string]*mech.Actor) (mech.Outcome, []mech.Roll, error)
	NPCTarget(npc *mech.Actor, candidates []*mech.Actor) (*mech.Actor, error)
	Roll(causeEventID string, rollIndex int, formula, purpose string) (mech.Roll, error)
	Stats(kind string) (mech.Actor, bool)
	Invariants() []mech.Invariant
}

// The assertions themselves. They are what fails at build time.
var (
	_ Mechanics = (*mech.Rules)(nil)
	_ Mechanics = (*fixed.FixedMechanics)(nil)
)

// encounterTurn is a consumer written against the interface and nothing else —
// the shape of what the encounter agent of EPIC-003 will do with C-03. It is
// here so that the substitution is exercised and not merely declared: the same
// function is handed the stub and the real rule set below.
func encounterTurn(m Mechanics, causeEventID string, attacker, defender *mech.Actor) (hp, dice int, err error) {
	actors := map[string]*mech.Actor{attacker.ID: attacker, defender.ID: defender}

	out, rolls, err := m.Resolve(causeEventID, 0,
		mech.Action{Kind: mech.ActionAttack, Actor: attacker.ID, Target: defender.ID}, actors)
	if err != nil {
		return 0, 0, err
	}
	// A consumer publishes one dice.rolled per roll before the decision that
	// rests on them, so how many there were is part of what it reads (C-03).
	dice = len(rolls)
	if out.TargetDead {
		return out.HPAfter, dice, nil
	}

	// Whom the NPC answers is a decision of the rules, not of the caller, and
	// nobody left to bite is an answer rather than an error (C-03 v1.2).
	bitten, err := m.NPCTarget(defender, []*mech.Actor{attacker})
	if err != nil {
		return 0, 0, err
	}
	if bitten == nil {
		return out.HPAfter, dice, nil
	}
	_, answer, err := m.Resolve(causeEventID, dice,
		mech.Action{Kind: mech.ActionNPCAttack, Actor: defender.ID, Target: bitten.ID}, actors)
	if err != nil {
		return 0, 0, err
	}
	return out.HPAfter, dice + len(answer), nil
}

// TestOneConsumerDrivesBothImplementations runs the same consumer against the
// stub and against the loaded rule set — the substitution T-017 promised,
// exercised now that EPIC-002 has filled Resolve and NPCTarget in (T-053): the
// only difference between the two runs is which constructor wired them up.
func TestOneConsumerDrivesBothImplementations(t *testing.T) {
	stub := load(t)
	loaded, err := mech.Load(rulesPath())
	if err != nil {
		t.Fatalf("load the rules: %v", err)
	}

	for name, m := range map[string]Mechanics{"stub": stub, "rules": loaded} {
		t.Run(name, func(t *testing.T) {
			player, wolf := actorsOf(t, stub)
			hp, dice, err := encounterTurn(m, "ev-1", player, wolf)
			if err != nil {
				t.Fatalf("could not drive a turn: %v", err)
			}
			if hp < 0 || hp > wolf.HPMax {
				t.Errorf("hp_after %d outside [0, %d]", hp, wolf.HPMax)
			}
			if dice == 0 {
				t.Error("a turn that rolled nothing: there would be no dice.rolled to publish")
			}
		})
	}
}

// TestBothImplementationsTellNobodyFromADefect is the reason C-03 v1.2 gave
// NPCTarget an error: nobody left to bite and a defect of the caller are
// different answers, and a consumer written against either implementation
// tells them apart the same way — for every defect the real one refuses, so
// that a consumer debugged on the stub meets no new error on the real mechanics.
func TestBothImplementationsTellNobodyFromADefect(t *testing.T) {
	stub := load(t)
	loaded, err := mech.Load(rulesPath())
	if err != nil {
		t.Fatalf("load the rules: %v", err)
	}

	for name, m := range map[string]Mechanics{"stub": stub, "rules": loaded} {
		t.Run(name, func(t *testing.T) {
			player, wolf := actorsOf(t, stub)
			player.Status = entity.StatusAbandoned

			bitten, err := m.NPCTarget(wolf, []*mech.Actor{player})
			if err != nil || bitten != nil {
				t.Errorf("nobody to bite answered (%v, %v), want (nil, nil)", bitten, err)
			}

			alive, _ := actorsOf(t, stub)
			deadWolf := *wolf
			deadWolf.Status = entity.StatusDead
			wolfAsCandidate := *wolf
			for defect, call := range map[string]struct {
				npc        *mech.Actor
				candidates []*mech.Actor
			}{
				"no npc":                {nil, []*mech.Actor{alive}},
				"a character picks":     {alive, []*mech.Actor{alive}},
				"a dead npc picks":      {&deadWolf, []*mech.Actor{alive}},
				"a nil candidate":       {wolf, []*mech.Actor{alive, nil}},
				"an npc as a candidate": {wolf, []*mech.Actor{&wolfAsCandidate}},
				"one character twice":   {wolf, []*mech.Actor{alive, alive}},
			} {
				if got, err := m.NPCTarget(call.npc, call.candidates); err == nil || got != nil {
					t.Errorf("%s answered (%v, %v), want an error", defect, got, err)
				}
			}
		})
	}
}
