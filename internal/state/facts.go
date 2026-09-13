package state

import (
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Reason is why a proposal was refused: the enum of
// entity.update.rejected.reason (C-02 v1.4).
type Reason string

// The seven reasons of C-02, every one of which State answers with.
const (
	ReasonVersionConflict Reason = "version_conflict"
	ReasonUnknownEntity   Reason = "unknown_entity"
	ReasonLevelViolation  Reason = "level_violation"
	ReasonLawViolation    Reason = "law_violation"
	ReasonInvalidOp       Reason = "invalid_op"
	ReasonDeadEntity      Reason = "dead_entity"
	ReasonDuplicateEntity Reason = "duplicate_entity"
)

// Rejection is one refusal of a proposal (§4.1): the reason, the entity it is
// about when there is one, the numbers that explain a version conflict and the
// law that answered a law_violation.
type Rejection struct {
	Reason                         Reason
	Ref                            *entity.Ref
	ExpectedVersion, ActualVersion *int64
	InvariantID                    string
}

// The id of every event State answers a proposal with is derived from the
// proposal event, the type and the entity (eventbus.WithCauseID, C-01 v1.4,
// ADR-027). A proposal the bus delivers again — after a restart that caught it
// uncommitted — is answered with the same ids, so a consumer that drops repeats
// by id drops the ones published before (dev-log T-055, "WithCauseID").
//
// The entity is the part that tells the answers of one proposal apart: a
// package names each entity once (C-02 v1.3), and a proposal refused as a whole
// has one refusal, under the entity it names or under none.

// createdFact is entity.created for an entity State has just created
// (C-02 v1.6): the attributes as they were stored and the proposal_id of the
// create, which the schema requires.
func createdFact(cause eventbus.Event, source string, e *entity.Entity, proposalID string) eventbus.Event {
	return eventbus.Derive(cause, TypeCreated, source, map[string]any{
		"entity":      entityPayload(e.Ref(), e.Name),
		"version":     e.Version,
		"attributes":  e.Attributes,
		"proposal_id": proposalID,
	}, eventbus.WithCauseID(e.ID))
}

// updatedFact is entity.updated for one entity of an applied package.
//
// changed is the list ApplyOps returned, as it is: entity.Change.MarshalJSON is
// the one encoder of its form (C-02 v1.6), and State has no second one. It is
// never nil — an empty list means the turn counted and the version did not
// move, and the schema requires the field either way.
func updatedFact(cause eventbus.Event, source string, e *entity.Entity, changed []entity.Change,
	proposal *Proposal) eventbus.Event {
	if changed == nil {
		changed = []entity.Change{}
	}
	return eventbus.Derive(cause, TypeUpdated, source, map[string]any{
		"entity":      entityPayload(e.Ref(), e.Name),
		"version":     e.Version,
		"changed":     changed,
		"cause":       proposal.Cause,
		"proposal_id": proposal.ID,
		"applied_at":  cause.Timestamp,
	}, eventbus.WithCauseID(e.ID))
}

// rejectedFact is entity.update.rejected. It answers both proposal types:
// duplicate_entity is the refusal of a create.
//
// The entity is named only when the proposal gave both its id and its type: the
// schema requires both, and a refusal the bus cannot publish would hold up the
// world for good (review #1 of T-055, Ma-1, the deterministic path). The id
// still tells the refusals of one package apart.
func rejectedFact(cause eventbus.Event, source, proposalID string, r Rejection) eventbus.Event {
	payload := map[string]any{"proposal_id": proposalID, "reason": string(r.Reason)}
	part := ""
	if r.Ref != nil && r.Ref.ID != "" {
		part = r.Ref.ID
		if r.Ref.Type != "" {
			payload["entity"] = entityPayload(*r.Ref, "")
		}
	}
	details := map[string]any{}
	if r.ExpectedVersion != nil {
		details["expected_version"] = *r.ExpectedVersion
	}
	if r.ActualVersion != nil {
		details["actual_version"] = *r.ActualVersion
	}
	if r.InvariantID != "" {
		details["invariant_id"] = r.InvariantID
	}
	if len(details) > 0 {
		payload["details"] = details
	}
	return eventbus.Derive(cause, TypeRejected, source, payload, eventbus.WithCauseID(part))
}

// entityPayload is an entity as _common.json#/$defs/EntityWithName carries it.
func entityPayload(ref entity.Ref, name string) map[string]any {
	out := map[string]any{"entity": map[string]any{"id": ref.ID, "type": ref.Type}}
	if name != "" {
		out["name"] = name
	}
	return out
}
