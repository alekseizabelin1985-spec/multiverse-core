package mechanics

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
)

// This file is the whole of chance in the platform. Everything that rolls
// anything — a fight, the chance of an encounter, a background table of the
// region GM — comes through it, because determinism that lives in more than one
// place is determinism that holds in one of them (ADR-012 p. 3).

// Seed is the address of a roll: the event that caused it and its index inside
// that cause, hashed into a number (ADR-003 p. 5, NFR-060).
//
// Nothing here reads a clock, and there is no generator carried between rolls.
// A roll is therefore reproducible from the journal alone: an agent re-running
// a turn in replay mode asks for the same (event id, index) and gets the same
// dice, which is what lets a recorded run be compared with a fresh one
// (dice_rolled_new = 0).
//
// The first eight bytes of SHA-256, big endian. SHA-256 rather than a cheap
// hash because neighbouring indices of one event must not produce neighbouring
// seeds: PCG seeded with 1 and PCG seeded with 2 are not visibly related, but
// nothing in the contract says a caller may not use a hash where they would be.
func Seed(eventID string, rollIndex int) uint64 {
	sum := sha256.Sum256([]byte(eventID + ":" + strconv.Itoa(rollIndex)))
	return binary.BigEndian.Uint64(sum[:8])
}

// NewRNG opens the generator of one roll. PCG from math/rand/v2: no global
// state to be raced over, and a stream that is defined by the algorithm rather
// than by the version of Go that happens to be building — a run recorded today
// replays on the next toolchain.
func NewRNG(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 0)) }

// Roll throws one formula for one purpose, addressed by its cause and index.
// It is the entry point for every roll outside a fight — the chance of an
// encounter from a tick, a background table of a region — and the same one
// Resolve will use inside it (C-03 v1.1).
//
// One roll, one generator: the index is what separates two rolls of the same
// event, and reusing an index inside one cause means rolling the same dice
// twice (§5.5).
func (r *Rules) Roll(causeEventID string, rollIndex int, formula, purpose string) (Roll, error) {
	if causeEventID == "" {
		return Roll{}, fmt.Errorf("mechanics: roll without a cause event")
	}
	if rollIndex < 0 {
		return Roll{}, fmt.Errorf("mechanics: roll index %d is negative", rollIndex)
	}
	if !slices.Contains(Purposes, purpose) {
		return Roll{}, fmt.Errorf("mechanics: unknown roll purpose %q", purpose)
	}
	dice, err := ParseDice(formula)
	if err != nil {
		return Roll{}, fmt.Errorf("mechanics: roll: %w", err)
	}

	seed := Seed(causeEventID, rollIndex)
	result, natural := dice.Roll(NewRNG(seed))
	return Roll{
		Index:   rollIndex,
		Formula: dice.String(),
		Seed:    seed,
		Result:  result,
		Natural: natural,
		Purpose: purpose,
	}, nil
}

// RollCheck throws a check — dice plus the bonuses of the actor against the
// threshold of the target — and reports both the roll as dice.rolled carries it
// and the arithmetic the decision rests on. It is what a hit and a flight
// attempt are made of; how the two turn into an Outcome is Resolve (T-053).
func (r *Rules) RollCheck(causeEventID string, rollIndex int, c CheckExpr, purpose string, actor, target Actor, ctx map[string]int) (Roll, CheckResult, error) {
	if causeEventID == "" {
		return Roll{}, CheckResult{}, fmt.Errorf("mechanics: roll without a cause event")
	}
	if rollIndex < 0 {
		return Roll{}, CheckResult{}, fmt.Errorf("mechanics: roll index %d is negative", rollIndex)
	}
	if !slices.Contains(Purposes, purpose) {
		return Roll{}, CheckResult{}, fmt.Errorf("mechanics: unknown roll purpose %q", purpose)
	}

	seed := Seed(causeEventID, rollIndex)
	res, err := c.Eval(NewRNG(seed), actor, target, ctx)
	if err != nil {
		return Roll{}, CheckResult{}, fmt.Errorf("mechanics: %w", err)
	}
	return Roll{
		Index:   rollIndex,
		Formula: c.Dice.String(),
		Seed:    seed,
		Result:  res.Roll,
		Natural: res.Natural,
		Purpose: purpose,
	}, res, nil
}
