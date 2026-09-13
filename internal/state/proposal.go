package state

import (
	"encoding/json"
	"errors"
	"fmt"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/jsonpath"
)

// The event types State reads and writes (C-02).
const (
	TypeCreateProposed = "entity.create.proposed"
	TypeUpdateProposed = "entity.update.proposed"
	TypeCreated        = "entity.created"
	TypeUpdated        = "entity.updated"
	TypeRejected       = "entity.update.rejected"
)

// IsProposal reports whether State answers events of this type. Everything else
// on system_events — its own facts above all — is passed over, or State would
// answer itself.
func IsProposal(typ string) bool {
	return typ == TypeCreateProposed || typ == TypeUpdateProposed
}

// Kind tells the two proposals apart.
type Kind int

const (
	// KindCreate is entity.create.proposed.
	KindCreate Kind = iota + 1
	// KindUpdate is entity.update.proposed.
	KindUpdate
)

// Proposal is a proposal as State decides it (state-and-mechanics.md §4.1): the
// payload read into the model types, and the event it came in, which every fact
// derives from.
//
// Who proposed it (Proposer of §4.1) is not read here yet: it is what the
// ownership check of T-056 needs, and nothing before it does.
type Proposal struct {
	// ID is proposal_id. It is required in both types (C-02 v1.6) and nothing
	// is put in its place.
	ID    string
	Event eventbus.Event
	Kind  Kind
	// Create is the entity a create proposes; nil for an update.
	Create *CreateSpec
	// Changes are the change sets of an update; nil for a create.
	Changes []entity.ChangeSet
	Atomic  bool
	Cause   string
}

// CreateSpec is what entity.create.proposed asks to create.
type CreateSpec struct {
	Ref        entity.Ref
	Name       string
	Attributes map[string]any
}

// ErrNoProposalID is a proposal without proposal_id. There is nothing to refuse
// it by — entity.update.rejected requires the identifier — so State passes it
// over with a warning (§4.5 p. 1).
var ErrNoProposalID = errors.New("state: proposal without proposal_id")

// ErrMalformed is a proposal that cannot be read into the model. With a
// proposal_id it is refused as invalid_op (§4.5 p. 1).
var ErrMalformed = errors.New("state: malformed proposal")

// wireUpdate and wireCreate are the payloads of C-02 (api-contracts.md §2.3.4).
type wireUpdate struct {
	ProposalID string             `json:"proposal_id"`
	Changes    []entity.ChangeSet `json:"changes"`
	Atomic     bool               `json:"atomic"`
	Cause      string             `json:"cause"`
}

type wireCreate struct {
	ProposalID string          `json:"proposal_id"`
	Entity     eventbus.Entity `json:"entity"`
	Attributes map[string]any  `json:"attributes"`
	Cause      string          `json:"cause"`
}

// ParseProposal reads a proposal out of its event.
//
// The payload goes through JSON rather than through type assertions, because
// that is how it travelled: a subscriber holds a float64 wherever the publisher
// wrote a number. The result shares nothing with the event, so no caller that
// keeps the event can change a value State is deciding about.
//
// A payload that does not read returns ErrMalformed, with the proposal_id in
// the returned proposal when that much could be read; a payload without
// proposal_id returns ErrNoProposalID.
func ParseProposal(ev eventbus.Event) (*Proposal, error) {
	p := &Proposal{Event: ev}
	raw, err := json.Marshal(ev.Payload)
	if err != nil {
		return p, fmt.Errorf("%w: %s: %w", ErrMalformed, ev.Type, err)
	}
	p.ID = proposalIDOf(raw)
	switch ev.Type {
	case TypeCreateProposed:
		var w wireCreate
		if err := json.Unmarshal(raw, &w); err != nil {
			return p, noIDFirst(p, fmt.Errorf("%w: %s: %w", ErrMalformed, ev.Type, err))
		}
		p.Kind, p.Cause = KindCreate, w.Cause
		p.Create = &CreateSpec{Ref: entity.RefFrom(w.Entity.Entity), Name: w.Entity.Name, Attributes: w.Attributes}
	case TypeUpdateProposed:
		var w wireUpdate
		if err := json.Unmarshal(raw, &w); err != nil {
			return p, noIDFirst(p, fmt.Errorf("%w: %s: %w", ErrMalformed, ev.Type, err))
		}
		p.Kind, p.Cause, p.Atomic, p.Changes = KindUpdate, w.Cause, w.Atomic, w.Changes
	default:
		return p, fmt.Errorf("%w: %s is not a proposal", ErrMalformed, ev.Type)
	}
	if p.ID == "" {
		return p, ErrNoProposalID
	}
	return p, nil
}

// proposalIDOf reads proposal_id on its own, so that a payload malformed
// elsewhere can still be refused under its identifier.
func proposalIDOf(raw []byte) string {
	var head struct {
		ProposalID any `json:"proposal_id"`
	}
	if json.Unmarshal(raw, &head) != nil {
		return ""
	}
	id, _ := head.ProposalID.(string)
	return id
}

// noIDFirst lets the missing identifier win over the malformed rest: without
// one there is no refusal to publish either way.
func noIDFirst(p *Proposal, err error) error {
	if p.ID == "" {
		return ErrNoProposalID
	}
	return err
}

// cloneOps deep-copies the values of operations. entity.ApplyOps puts the map or
// the slice it is given into the attributes it returns, so an operation value
// still shared with anyone else would be shared with the copy State decides
// about until Commit (§4.5 p. 8; backlog of T-448 p. 3).
func cloneOps(ops []entity.Op) []entity.Op {
	out := make([]entity.Op, len(ops))
	for i, op := range ops {
		out[i] = op
		if op.Value == nil {
			continue
		}
		if copied, ok := jsonpath.New(op.Value).Clone().GetAny(""); ok {
			out[i].Value = copied
		}
	}
	return out
}
