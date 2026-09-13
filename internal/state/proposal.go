package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

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
// payload read into the model types, who proposed it, and the event it came in,
// which every fact derives from.
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
	// Proposer is read off the envelope for the ownership check (§4.6).
	Proposer Proposer
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
	p := &Proposal{Event: ev, Proposer: proposerOf(ev)}
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

// malformedOp reports the first change set holding an operation that no entity
// could take, whatever the world holds: a verb outside the four of C-02, a path
// not written in its canonical form, a path under a field of the entity rather
// than an attribute, or a value no reader of the bus could hold (§4.5 p. 1,
// C-02 v1.6). Such a
// proposal is refused whole, before the window of applied proposals and before
// anybody's rights are asked: it is a defect of its form, not of its proposer.
//
// What depends on the entity — an inc on a text, an index past the end — is
// left to ApplyOps at step 7 and refuses its own change set.
func malformedOp(sets []entity.ChangeSet) (entity.Ref, bool) {
	for _, set := range sets {
		for _, op := range set.Ops {
			if !knownOp(op.Op) || !canonicalPath(op.Path) || reservedRoot(op.Path) || underScalar(op.Path) ||
				!entity.JSONCompatible(op.Value) {
				return set.Ref(), true
			}
		}
	}
	return entity.Ref{}, false
}

func knownOp(kind entity.OpKind) bool {
	switch kind {
	case entity.OpSet, entity.OpInc, entity.OpAppend, entity.OpRemove:
		return true
	default:
		return false
	}
}

// canonicalPathSegment is one key of a path with its indices: a name without
// dots and brackets, then any number of [n] with n written without leading
// zeros and in at most nine digits. inventory[01] would address inventory[1]
// under a second spelling (review #2 of T-056, N-3), and an index past nine
// digits no longer reads as an index at all: on an object it becomes a key
// made of digits (condition У-1 of system-architect).
var canonicalPathSegment = regexp.MustCompile(`^([^.\[\]]+)(\[(0|[1-9][0-9]{0,8})\])*$`)

// canonicalPath reports whether a path is written the one way State compares
// paths by: keys between single dots, indices in brackets after a key, no dot at
// either end, no empty key, no empty or non-numeric index, and no key that
// reads as an integer — inventory.0 is inventory[0] on a list, and a remove
// written that way would announce the path of the element instead of the path
// of the list that C-02 v1.6 gives it (condition У-1 of system-architect).
//
// entity.ApplyOps reads status., .status and a..b as the attribute status, while
// the checks of steps 5 and 6 and the matrix of the status compare the string
// of the path: a path in another form would write status past all of them, and
// entity.updated would carry that form to every reader (review #1 of T-056,
// Ma-2). system-architect placed the canonical grammar for every reader in
// shared/entity and the catch-up rule (answer 2 of the review of T-056, C-02
// v1.8 p. 2); until that task (T-472) moves it, State refuses the other forms
// itself.
func canonicalPath(path string) bool {
	if path == "" {
		return false
	}
	for _, segment := range strings.Split(path, ".") {
		match := canonicalPathSegment.FindStringSubmatch(segment)
		if match == nil {
			return false
		}
		if _, err := strconv.Atoi(match[1]); err == nil {
			return false
		}
	}
	return true
}

// scalarAttributes are the attributes data-model.md §3 types as one value — a
// number, a text, a time, a duration or a reference — whatever the type of the
// entity: no name of §3 is a scalar of one type and a container of another.
// scope is not one of them: it can be the object {id, type}. A path below a
// scalar (status.x, hp.x, state.x) is a path no entity could hold:
// entity.ApplyOps would replace the value with an object, and every reader of
// the attribute would stop reading it (review #2 of T-056, Ma-3; review #3,
// Mi-7; condition У-2 of system-architect).
//
// system-architect placed the rule in shared/entity, next to entity.Attr*, for
// every reader (answer 2 of the review of T-056); until that task (T-472) moves it,
// State refuses such a path at step 1.
var scalarAttributes = []string{
	// Every type (§3): name has no entity.Attr* constant (review #1 of T-470).
	"name",
	// Character and NPC (§3.3, §3.4); last_session_ended_at has no constant
	// either, see the note in shared/entity/attrs.go.
	"last_session_ended_at",
	entity.AttrHP, entity.AttrHPMax, entity.AttrAtk, entity.AttrDef, entity.AttrDmg, entity.AttrFlee,
	entity.AttrStatus, entity.AttrPosition, entity.AttrActorKind, entity.AttrGroupID, entity.AttrEncounterID,
	entity.AttrKind, entity.AttrRegionID, entity.AttrDiedAt, entity.AttrKilledBy, entity.AttrLootClaimedBy,
	// World (§3.1).
	entity.AttrLawsVersion, entity.AttrWeather, entity.AttrTimeOfDay, entity.AttrDay, entity.AttrSeason,
	entity.AttrEpoch, entity.AttrLocale, entity.AttrBlueprintRef,
	// Region (§3.2).
	entity.AttrDescription, entity.AttrRespawnTTL, entity.AttrPerceptionRadius, entity.AttrEncounterChance,
	entity.AttrLastBackgroundEventAt,
	// Group and encounter (§3.6, §3.7).
	entity.AttrLeaderID, entity.AttrState, entity.AttrResolution, entity.AttrRoundSeq, entity.AttrTaskAgentID,
	entity.AttrOpenedByEventID, entity.AttrClosedByEventID,
}

// underScalar reports whether a path goes below a scalar attribute.
func underScalar(path string) bool {
	root := rootOf(path)
	return root != path && slices.Contains(scalarAttributes, root)
}

// reservedRoot mirrors the rule of entity.ApplyOps: the fields of the entity
// and the roots starting with an underscore are not attributes.
func reservedRoot(path string) bool {
	root := rootOf(path)
	return slices.Contains(entity.ReservedPaths, root) || strings.HasPrefix(root, "_")
}
