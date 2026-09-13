package mechanics

import (
	"cmp"
	"fmt"
	"slices"

	"multiverse-core.io/shared/entity"
)

// NPCTarget picks whom an NPC bites, in the order the rules file writes in
// npc_target.order: the character who hit it last if that character is still a
// candidate, otherwise the one with the fewest hit points, and on a tie the
// lowest id. Anyone the rules exclude is not a candidate at all: the dead, the
// abandoned, those who ascended, the idle and those out of combat (§5.4, inv-01,
// C-02 v1.2).
//
// Nobody left to bite is an answer, not an error: (nil, nil) is UC-008 A2, and
// it is why C-03 v1.2 gave the function a second result. An error is a defect
// of the caller — no NPC, a biter that is not an NPC or is not alive, a
// candidate that is not a character, one character listed twice.
//
// The answer does not depend on the order of candidates: whatever the order of
// the rules leaves tied is broken by id, so a replay that assembles the same
// candidates differently still bites the same character.
func (r *Rules) NPCTarget(npc *Actor, candidates []*Actor) (*Actor, error) {
	if npc == nil {
		return nil, fmt.Errorf("mechanics: npc target without an npc")
	}
	if npc.Type != entity.TypeNPC {
		return nil, fmt.Errorf("mechanics: %s picks a target, and it is a %s, not an npc", npc.ID, npc.Type)
	}
	if !npc.Alive() {
		return nil, fmt.Errorf("mechanics: npc %s is %s and bites nobody", npc.ID, npc.Status)
	}

	eligible := make([]*Actor, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for i, c := range candidates {
		if c == nil {
			return nil, fmt.Errorf("mechanics: candidate %d of %s is nil", i, npc.ID)
		}
		if c.Type != entity.TypePlayer {
			return nil, fmt.Errorf("mechanics: candidate %s of %s is a %s: an npc bites characters", c.ID, npc.ID, c.Type)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("mechanics: candidate %s of %s is listed twice", c.ID, npc.ID)
		}
		seen[c.ID] = true
		if !r.Excluded(*c) {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return nil, nil
	}

	slices.SortFunc(eligible, func(x, y *Actor) int {
		for _, key := range r.doc.NPCTarget.Order {
			if c := compareBy(key, npc, x, y); c != 0 {
				return c
			}
		}
		return cmp.Compare(x.ID, y.ID)
	})
	return eligible[0], nil
}

// compareBy orders two candidates by one key of npc_target.order; the smaller
// one is bitten first. Load admits only the three keys below.
func compareBy(key string, npc, x, y *Actor) int {
	switch key {
	case OrderLastDamager:
		// The last damager comes first; everyone else is equal on this key.
		return cmp.Compare(boolRank(x.ID != npc.LastDamager), boolRank(y.ID != npc.LastDamager))
	case OrderMinHP:
		return cmp.Compare(x.HP, y.HP)
	case OrderPlayerIDAsc:
		return cmp.Compare(x.ID, y.ID)
	default:
		return 0
	}
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}
