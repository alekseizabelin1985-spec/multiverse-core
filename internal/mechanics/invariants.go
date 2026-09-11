package mechanics

// The register of the laws of a world in executable form (NFR-020, §5.7).
//
// There is one list, and it is this one. The text of every law lives in
// laws/dark-forest-world.v1.yaml and nowhere else; the logic lives here and
// nowhere else; the identifiers are the same in both, and a contract test
// (mvctl laws check) compares the two sets so that a law cannot be written
// without being enforced or enforced without being written (ADR-012 p. 5).
//
// A rules file switches identifiers on through its invariants list — it does
// not describe them. An invariant is a property of the world, not a number a
// balance patch may change.

// Where an invariant is enforced.
const (
	WhereState     = "state"
	WhereMechanics = "mechanics"
	WhereGateway   = "gateway"
	WhereGuardian  = "guardian"
	WhereAudit     = "audit"
)

// The identifiers of the ten invariants of MVP-1. They are the law ids of
// laws@v1 and the values of the invariants list of a rules file.
const (
	InvDeadDoesNotAct     = "inv-01" // the dead neither act nor are targets
	InvHPInRange          = "inv-02" // 0 <= hp <= hp_max
	InvOneTrophyPerNPC    = "inv-03" // one NPC leaves at most one trophy
	InvGroupPosition      = "inv-04" // a member stands where the group stands
	InvGroupSize          = "inv-05" // a group holds between 1 and 6 members
	InvOneScope           = "inv-06" // a character is in exactly one scope
	InvHPMatchesDecision  = "inv-07" // hp after the fact equals hp_after of combat.decided
	InvNoDecisionsUnasked = "inv-08" // no combat.decided against a character that did not act
	InvRespawnTTL         = "inv-09" // an NPC does not come back before its cooldown; dead never becomes alive
	InvOnePosition        = "inv-10" // one entity, one position
)

// allInvariants is the register in identifier order. Check is nil throughout:
// the foundation delivers the identifiers, the places each one is enforced in
// and the contract test that keeps them aligned with the laws file; EPIC-002
// fills the checks in (tasks.md T-015 "not included", T-054).
//
// Two of them keep a nil Check for good — inv-07 and inv-08 are decided against
// the journal rather than against the world, so no Applier can answer them
// (§5.7).
var allInvariants = []Invariant{
	{ID: InvDeadDoesNotAct, Where: []string{WhereState, WhereMechanics, WhereGateway}},
	{ID: InvHPInRange, Where: []string{WhereState, WhereMechanics}},
	{ID: InvOneTrophyPerNPC, Where: []string{WhereState, WhereAudit}},
	{ID: InvGroupPosition, Where: []string{WhereState}},
	{ID: InvGroupSize, Where: []string{WhereState, WhereGateway}},
	{ID: InvOneScope, Where: []string{WhereState}},
	{ID: InvHPMatchesDecision, Where: []string{WhereAudit}},
	{ID: InvNoDecisionsUnasked, Where: []string{WhereGuardian, WhereAudit}},
	{ID: InvRespawnTTL, Where: []string{WhereState}},
	{ID: InvOnePosition, Where: []string{WhereState}},
}

// InvariantIDs are every identifier the register knows, in order. It is what a
// rules file may switch on and what the laws file is compared against.
func InvariantIDs() []string {
	out := make([]string, 0, len(allInvariants))
	for _, inv := range allInvariants {
		out = append(out, inv.ID)
	}
	return out
}
