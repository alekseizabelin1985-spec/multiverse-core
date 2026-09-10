package mechanics

// Resolve decides one action: it rolls what the rules say to roll and reports
// what happened, without touching the actors it was given (C-03, §5.4).
//
// It is not implemented here. The foundation delivers the shape of the contract
// — the types, the rules file, the formulas and the dice every outcome is built
// from — so that EPIC-003 can compile against the real signature while the
// table of outcomes itself is written by EPIC-002 (tasks.md T-015 "not
// included", T-053). Until then a caller that needs a fight uses
// testkit/mechanics.FixedMechanics, which answers from a table.
//
// The parts of the decision that are rules rather than logic already work and
// are what the implementation will be assembled from: Rules.Check for the hit
// and the flight check, Rules.RollCheck for a roll addressed by its cause,
// Rules.Critical and Rules.Fumble for the natural die, Rules.DamageDice and
// Rules.Damage for the wound, Rules.ClampHP for inv-02.
func (r *Rules) Resolve(causeEventID string, rollIndexStart int, a Action, actors map[string]*Actor) (Outcome, []Roll, error) {
	return Outcome{}, nil, ErrNotImplemented
}
