package state

// Step 2 of §4.5: a proposal already applied is not applied again (C-02
// "Гарантии", NFR-013).
//
// Two memories answer. The window of proposal identifiers (eventbus.Dedup, an
// LRU) remembers the last proposals whose answer is out. The commit record of
// an entity (last_change.proposal_id) remembers the last proposal that changed
// it, however long ago: it is what still recognises a repeat once the window
// has let the identifier go, and what a snapshot and a restart carry (§4.4,
// §4.8).
//
// A refusal enters neither: a proposer that fixes a refused proposal and sends
// it again under the same identifier is answered, not ignored. The same event
// delivered again after a refusal is stopped by the window of event ids of the
// context (Context.dispatch), not here.

// alreadyApplied reports whether the proposal was applied before.
//
// One entity carrying the proposal in its commit record is enough. §4.5 p. 2
// asks for every target entity, and for atomic=true that is the same thing;
// for atomic=false a package applied in part leaves the record only on the
// entities that were applied, and "every" would apply that part a second time
// once the window has forgotten the identifier (dev-log T-056).
func (a *Applier) alreadyApplied(p *Proposal) bool {
	if a.applied.Has(p.ID) {
		return true
	}
	for _, id := range targetsOf(p) {
		if e, ok := a.store.Get(a.worldID, id); ok && e.LastChange != nil && e.LastChange.ProposalID == p.ID {
			return true
		}
	}
	return false
}

// targetsOf are the entities a proposal names: the one a create brings, or the
// entity of every change set of an update.
func targetsOf(p *Proposal) []string {
	if p.Kind == KindCreate {
		if p.Create == nil || p.Create.Ref.ID == "" {
			return nil
		}
		return []string{p.Create.Ref.ID}
	}
	out := make([]string, 0, len(p.Changes))
	for _, set := range p.Changes {
		if id := set.Ref().ID; id != "" {
			out = append(out, id)
		}
	}
	return out
}

// remember records a proposal whose answer is out and kept.
func (a *Applier) remember(proposalID string) { a.applied.Add(proposalID) }

// AppliedProposals are the proposal identifiers in the window, oldest first:
// what a snapshot carries as applied_proposals (§4.4).
func (a *Applier) AppliedProposals() []string { return a.applied.IDs() }
