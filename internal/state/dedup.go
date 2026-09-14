package state

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

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

// resendUnpublished answers a proposal already applied (§4.5 p. 2). An entity
// that carries its commit record without a fact — written to the object store,
// its fact never confirmed out (fact_event_id empty, ADR-011 p. 1) — has its
// fact published again, under the id it was first derived with, and learns
// the id in memory. Nothing is applied or written a second time (C-02
// "Гарантии": "факты досылаются, если не были опубликованы"; C-02 v1.8 p. 5).
//
// A refusal of the same answer is not published again: the entity does not
// remember it, and C-02 v1.8 p. 5 gives a repeat of a package applied in part
// no answer.
func (a *Applier) resendUnpublished(ctx context.Context, p *Proposal) error {
	var pending []*entity.Entity
	for _, id := range targetsOf(p) {
		e, ok := a.store.Get(a.worldID, id)
		if !ok || e.LastChange == nil || e.LastChange.ProposalID != p.ID || e.LastChange.FactEventID != "" {
			continue
		}
		pending = append(pending, e)
	}
	if len(pending) == 0 {
		a.log.Debug("proposal already applied", "proposal_id", p.ID, "event_id", p.Event.ID)
		return nil
	}
	if err := a.refuseUnfinishedPackage(ctx, p); err != nil {
		return err
	}
	slices.SortFunc(pending, func(x, y *entity.Entity) int { return cmp.Compare(x.ID, y.ID) })
	facts := make([]eventbus.Event, 0, len(pending))
	for _, e := range pending {
		cause := originalCause(p.Event, e.LastChange)
		if p.Kind == KindCreate {
			facts = append(facts, createdFact(cause, a.source, e, p.ID))
			continue
		}
		// The cause of the fact is the cause the commit record kept, not the
		// one a new event under the same proposal_id may carry (review #1 of
		// T-057, Mi-3). The rest of the envelope — correlation, actor, locale —
		// is not kept by the record and comes from the event at hand.
		recorded := *p
		if e.LastChange.Cause != "" {
			recorded.Cause = e.LastChange.Cause
		}
		facts = append(facts, updatedFact(cause, a.source, e, e.LastChange.Changed, &recorded))
	}
	if err := a.publish(ctx, p.ID, facts...); err != nil {
		return err
	}
	for i, e := range pending {
		e.SetFactEventID(facts[i].ID)
	}
	if err := a.store.Put(a.worldID, pending...); err != nil {
		return a.keepFailed(p, err)
	}
	a.remember(p.ID)
	for _, fact := range facts {
		a.log.Info("the fact of an applied proposal was published again", "proposal_id", p.ID,
			"event_id", fact.ID, "type", fact.Type)
	}
	return nil
}

// refuseUnfinishedPackage stops the world instead of sending the facts of a
// package whose intent is still in the object store. An intent outlives its
// package when the writes of an atomic package were cut off between two
// entities: some of them are written and carry the commit record, the others
// are not. Sending the facts of the written ones would announce half of an
// atomic package (§4.7), and the intent is rolled forward by recovery, not
// here (§4.8, T-059). Nothing of the answer is published; the proposal stays
// uncommitted and is decided again once the intent is gone (review #1 of
// T-057, question 2; decision of the orchestrator).
//
// An intent whose every entity is at its to_version under this proposal is
// not unfinished: the package was written whole — before the cut, or by the
// roll forward of recovery — and only the removal of its intent failed, which
// does not stop the world (§9). Its facts are sent.
//
// The list of intents is asked with the attempts of a write: a single refusal
// of the store would otherwise stop the world (acceptance of T-057).
func (a *Applier) refuseUnfinishedPackage(ctx context.Context, p *Proposal) error {
	if a.objects == nil {
		return nil
	}
	var intents []*Intent
	if err := a.withAttempts(ctx, p.ID, "list the intents", func(out context.Context) error {
		var err error
		intents, err = a.objects.ListIntents(out, a.worldID)
		return err
	}); err != nil {
		return a.persistFailed(p, "the list of intents", err)
	}
	for _, in := range intents {
		if in.ProposalID != p.ID || a.finishedIn(in) {
			continue
		}
		failure := fmt.Errorf("%w: %w: %w: the intent of proposal %s is still in the store, the package is not rolled forward",
			ErrWorldStopped, ErrPersistFailed, errUnfinishedPackage, p.ID)
		a.halt(failure)
		a.log.Error("a repeated proposal has an unfinished package; no fact of it is sent and the world is stopped",
			"reason", "persist_failed", "proposal_id", p.ID, "event_id", p.Event.ID, "handled", false,
			"remedy", "restart the process: recovery rolls the intent forward")
		return failure
	}
	return nil
}

// errUnfinishedPackage is why the facts of a package with an intent left behind
// are not sent.
var errUnfinishedPackage = errors.New("state: unfinished package")

// originalCause is the proposal as the fact was first derived from: the event
// the commit record names and the instant it was applied at. The same event
// delivered again is that event; a new event under the same proposal_id is
// not, and deriving from it would give the fact another id and another
// applied_at than the one that may already be out (ADR-027).
func originalCause(ev eventbus.Event, change *entity.LastChange) eventbus.Event {
	if change.ProposalEventID != "" {
		ev.ID = change.ProposalEventID
	}
	if !change.AppliedAt.IsZero() {
		ev.Timestamp = change.AppliedAt
	}
	return ev
}

// remember records a proposal whose answer is out and kept.
func (a *Applier) remember(proposalID string) { a.applied.Add(proposalID) }

// AppliedProposals are the proposal identifiers in the window, oldest first:
// what a snapshot carries as applied_proposals (§4.4).
func (a *Applier) AppliedProposals() []string { return a.applied.IDs() }
