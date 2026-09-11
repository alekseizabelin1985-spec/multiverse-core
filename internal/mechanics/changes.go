package mechanics

// ChangesFor turns a decision into the operations that make it true: the hit
// points of the target, the status and the death record of whoever fell, the
// trophy in the inventory of whoever landed the last hit, the last damager of
// the NPC. The caller wraps them into one entity.update.proposed — one atomic
// package per round is the recommendation, one per combat.decided is allowed
// (C-03, §7.1).
//
// It is not implemented here (tasks.md T-015 "not included", T-053). The type
// it produces is: ProposedChange is what an EPIC-003 caller compiles against
// today and what EPIC-002 fills in.
//
// factEventID is the combat.decided the change follows from — it goes into the
// source of a trophy, which is how inv-03 proves a trophy was handed out once.
func ChangesFor(a Action, o Outcome, attacker, target *Actor, factEventID string) ([]ProposedChange, error) {
	return nil, ErrNotImplemented
}
