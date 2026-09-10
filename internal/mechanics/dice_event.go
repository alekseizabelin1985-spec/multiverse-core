package mechanics

import (
	"strconv"

	"multiverse-core.io/shared/entity"
)

// DiceRolledPayload builds the payload of dice.rolled — the only place it is
// built (§5.6, C-03 v1.1).
//
// The mechanics do not publish it. A library with no bus and no clock cannot
// promise that the roll reaches the journal before the decision that rests on
// it, and that order is what makes a fight auditable: the caller — the
// encounter agent, the region GM — derives one dice.rolled per Roll from the
// event that caused it and publishes them all before combat.decided (C-03).
//
// The shape is dice.rolled.v1.json. The seed travels as a decimal string
// (state-and-mechanics.md 5.6): the bus decodes a payload into map[string]any,
// where a JSON number becomes a float64, so a uint64 above 2^53 comes back a
// different number. A string is exact for every seed Seed can produce.
func DiceRolledPayload(roll Roll, roller entity.Ref) map[string]any {
	return map[string]any{
		"roll": map[string]any{
			"index":   roll.Index,
			"formula": roll.Formula,
			"seed":    strconv.FormatUint(roll.Seed, 10),
			"result":  roll.Result,
			"natural": roll.Natural,
		},
		"purpose": roll.Purpose,
		"roller": map[string]any{
			"entity": map[string]any{
				"id":   roller.ID,
				"type": roller.Type,
			},
		},
	}
}
