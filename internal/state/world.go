package state

import (
	"context"

	"multiverse-core.io/shared/entity"
)

// A world recovery finds neither latest.json nor an entity object of is
// uninitialized (state-and-mechanics.md §18): mvctl world init has not run
// yet. Until the world entity is in the working set, State answers only the
// proposals that initialise the world — entity.create.proposed with
// cause=init — and refuses every other one with unknown_entity under the world
// entity, before anything is written.
//
// Before the write, because the buckets of a world do not exist before its
// init: a proposal of the gateway that reached the write would stop the world
// with persist_failed, and the world init that follows would not pass without
// a restart of the process.

// initializes reports whether a proposal may be decided over a world that is
// not initialised.
func initializes(p *Proposal) bool {
	return p.Kind == KindCreate && p.Cause == CauseInit
}

// refuseUninitialized refuses a proposal that came before the world was
// initialised.
func (a *Applier) refuseUninitialized(ctx context.Context, p *Proposal) error {
	a.log.Info("proposal to a world that is not initialised refused", "proposal_id", p.ID,
		"event_id", p.Event.ID, "remedy", "run mvctl world init for the world first")
	return a.refuse(ctx, p, Rejection{Reason: ReasonUnknownEntity, Ref: &entity.Ref{ID: a.worldID, Type: entity.TypeWorld}})
}

// Uninitialized reports whether the world waits for its init.
func (a *Applier) Uninitialized() bool { return a.uninitialized.Load() }
