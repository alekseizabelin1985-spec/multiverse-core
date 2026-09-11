package mechanics

// NPCTarget picks whom an NPC bites: the character who hit it last if that
// character is still a candidate, otherwise the one with the fewest hit points,
// and on a tie the lowest id — the order the rules file writes in
// npc_target.order. Anyone the rules exclude is not a candidate at all: the
// dead, the abandoned, the idle and those out of combat (§5.4, inv-01).
//
// It is not implemented here, for the same reason Resolve is not: the
// foundation delivers the contract and the rules it will read (Rules.TargetRules
// and Rules.Excluded already answer both halves), EPIC-002 writes the choice
// (tasks.md T-053).
//
// The signature of C-03 v1 has no error to return, so the stub answers nil —
// the same thing the implemented function answers when nobody is left to bite
// (UC-008 A2). A caller that must tell the two apart before EPIC-002 lands
// should use testkit/mechanics.FixedMechanics instead of this.
func (r *Rules) NPCTarget(npc *Actor, candidates []*Actor) *Actor {
	return nil
}
