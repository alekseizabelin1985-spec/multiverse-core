package state

import (
	"cmp"
	"slices"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/entity"
)

// Step 8 of §4.5: the laws of the world over the world as the proposal would
// leave it.
//
// A law sees the world after the change and the entities the change touches,
// never the world before it (mechanics.StateView, §5.7). What needs the world
// before — a corpse that is changed, a terminal status left — is decided at
// step 5 against the entity as it was (dead_entity), before any law runs.

// overlayView is the working set of a world with the copies of the proposal in
// place of the originals: what the laws are asked about (§4.2). It is built
// once per decision from copies, so a law can read it as often as it likes and
// change nothing.
type overlayView struct {
	worldID string
	byID    map[string]*entity.Entity
}

// Get is one entity of the world as the proposal would leave it.
func (v *overlayView) Get(id string) (*entity.Entity, bool) {
	e, ok := v.byID[id]
	return e, ok
}

// ByType lists the entities of one type in identifier order, so that a law
// that walks them answers the same on every run (ADR-003 p. 5).
func (v *overlayView) ByType(t string) []*entity.Entity {
	var out []*entity.Entity
	for _, e := range v.byID {
		if e.Type == t {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b *entity.Entity) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// WorldID is the world the view is of.
func (v *overlayView) WorldID() string { return v.worldID }

var _ mechanics.StateView = (*overlayView)(nil)

// overlay builds the view of the world with the entities of the proposal put
// over it: the plans of an update as their attributes will be, or the entity a
// create brings.
func (a *Applier) overlay(plans []planned, created *entity.Entity) *overlayView {
	world := a.store.List(a.worldID)
	view := &overlayView{worldID: a.worldID, byID: make(map[string]*entity.Entity, len(world)+1)}
	for _, e := range world {
		view.byID[e.ID] = e
	}
	for _, plan := range plans {
		after := entity.Clone(plan.entity)
		after.Attributes = plan.attrs
		view.byID[after.ID] = after
	}
	if created != nil {
		view.byID[created.ID] = created
	}
	return view
}

// violation is the first law the view breaks around the touched entities, in
// the order of the register (Invariants), and within a law the first answer of
// its Check, which the check has already ordered. A law without a Check is
// decided elsewhere (inv-07, inv-08) or later (inv-04…06, T-066) and is not
// asked.
func (a *Applier) violation(view *overlayView, touched []string) (*Rejection, bool) {
	for _, inv := range a.lawsInForce() {
		if inv.Check == nil {
			continue
		}
		found := inv.Check(view, touched)
		if len(found) == 0 {
			continue
		}
		first := found[0]
		r := &Rejection{Reason: ReasonLawViolation, InvariantID: first.InvariantID}
		if first.InvariantID == "" {
			r.InvariantID = inv.ID
		}
		// The law may answer on an entity the proposal does not name — inv-01
		// on the NPC of an encounter that only the encounter changed — so the
		// type comes from the world, not from changes[] (§4.5 p. 8).
		ref := entity.Ref{ID: first.EntityID}
		if e, ok := view.Get(first.EntityID); ok {
			ref.Type = e.Type
		}
		r.Ref = &ref
		a.log.Info("proposal breaks a law of the world", "invariant_id", r.InvariantID,
			"entity_id", first.EntityID, "message", first.Message)
		return r, true
	}
	return nil, false
}

// touchedBy are the identifiers of every entity of the applied part of a
// package, those whose changed[] is empty included: a set to the value already
// there still asks the laws about its entity (§4.5 p. 8).
func touchedBy(plans []planned) []string {
	out := make([]string, 0, len(plans))
	for _, plan := range plans {
		out = append(out, plan.entity.ID)
	}
	return out
}

// checkLaws runs step 8 over the plans a package kept after steps 3–7.
//
// atomic=true: the first break refuses the whole package. atomic=false: every
// change set gets an answer (C-02: one rejected per entity). The change set of
// the entity the law names is refused and the laws are asked again about the
// rest (§4.5 p. 9). A break on an entity no remaining change set names — an NPC
// of an encounter the package only touches, or an entity whose own change set
// was already refused at steps 3–7 — cannot be pinned on one change set, so each
// remaining change set is refused under its own entity with the law that
// answered (review #1 of T-056, Ma-1; confirmed by system-architect, answer 1
// of the review of T-056, C-02 v1.8 p. 1).
func (a *Applier) checkLaws(p *Proposal, plans []planned) (kept []planned, refusals []Rejection, whole *Rejection) {
	if len(a.lawsInForce()) == 0 {
		return plans, nil, nil
	}
	kept = plans
	for len(kept) > 0 {
		r, broken := a.violation(a.overlay(kept, nil), touchedBy(kept))
		if !broken {
			return kept, refusals, nil
		}
		if p.Atomic {
			return nil, nil, r
		}
		i := slices.IndexFunc(kept, func(plan planned) bool { return plan.entity.ID == r.Ref.ID })
		if i >= 0 {
			refusals = append(refusals, *r)
			kept = slices.Delete(slices.Clone(kept), i, i+1)
			continue
		}
		for _, plan := range kept {
			own := plan.entity.Ref()
			refusals = append(refusals, Rejection{Reason: ReasonLawViolation, Ref: &own, InvariantID: r.InvariantID})
		}
		return nil, refusals, nil
	}
	return kept, refusals, nil
}
