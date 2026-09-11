// Package entity is the model of an entity of the world: what State stores in
// the object store, what a snapshot carries and what every read-model projects
// (state-and-mechanics.md §3, data-model.md §3).
//
// Three rules shape the package.
//
// It has no clock. Every timestamp is the timestamp of the event that caused
// the change and is passed in by the caller, so a replay of the same journal
// produces byte-identical objects (ADR-003 p. 6).
//
// It never writes behind the caller's back. ApplyOps computes the effect of a
// proposal on a copy of the attributes and hands it back; State decides whether
// to keep it after the ownership and invariant checks, and only Commit moves
// the entity to the next version.
//
// It owns no policy. Which proposer may touch which path, which invariant holds
// and which rejection reason an error becomes are decisions of internal/state
// (state-and-mechanics.md §4.5, §4.6) — this package only says what an
// operation means and when it is malformed.
package entity

import (
	"fmt"
	"time"

	"multiverse-core.io/shared/eventbus"
)

// SchemaVersion is the version of the shape below. It is written into every
// object so that a reader can tell an entity of the platform apart from the
// as-is objects with a payload field, which are not read at all (overview §19).
const SchemaVersion = 1

// HistoryLimit is how many entries History keeps. The full history of an
// entity is the journal; History is the tail that makes an object
// self-explanatory without it (state-and-mechanics.md §3.1).
const HistoryLimit = 50

// Entity is one entity of one world. The shape in memory and the shape on disk
// are the same one: an object under entities-{world}/{type}/{id}.json is this
// struct, and so is an element of the entities array of a snapshot.
type Entity struct {
	SchemaVersion int            `json:"schema_version"`
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	WorldID       string         `json:"world_id"` // equals ID when Type is world
	Name          string         `json:"name"`
	Version       int64          `json:"version"`       // strictly +1 per applied fact
	CreatedAt     time.Time      `json:"created_at"`    // timestamp of entity.create.proposed
	UpdatedAt     time.Time      `json:"updated_at"`    // timestamp of the last *.proposed
	LastEventID   string         `json:"last_event_id"` // id of the last entity.created/updated
	Attributes    map[string]any `json:"attributes"`    // domain attributes, data-model.md §3
	LastChange    *LastChange    `json:"last_change,omitempty"`
	History       []HistoryEntry `json:"history,omitempty"`
}

// HistoryEntry is one applied version of an entity: enough to find the fact in
// the journal, not enough to replace it.
type HistoryEntry struct {
	Version    int64     `json:"version"`
	EventID    string    `json:"event_id"`
	ProposalID string    `json:"proposal_id"`
	At         time.Time `json:"at"`
}

// LastChange is the commit record of the last applied proposal (ADR-011). It is
// what tells a restarted State whether a fact was published for the version it
// finds on disk: FactEventID is empty between the write of the object and the
// publication of the fact (state-and-mechanics.md §4.5 p. 12, §4.8).
type LastChange struct {
	ProposalID      string    `json:"proposal_id"`
	ProposalEventID string    `json:"proposal_event_id"`
	FactEventID     string    `json:"fact_event_id,omitempty"`
	Cause           string    `json:"cause"`
	Changed         []Change  `json:"changed"`
	AppliedAt       time.Time `json:"applied_at"`
	Atomic          bool      `json:"atomic"`
	BatchSize       int       `json:"batch_size"`
}

// Ref is an entity addressed by identity alone — the form every event payload
// carries (_common.json#/$defs/EntityRef).
type Ref struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// String renders a reference as type:id, the form used in log lines and error
// messages. It is not an identifier: id alone is unique within a world.
func (r Ref) String() string { return r.Type + ":" + r.ID }

// EventRef converts a reference into the payload type of the bus.
func (r Ref) EventRef() eventbus.EntityRef {
	return eventbus.EntityRef{ID: r.ID, Type: r.Type}
}

// RefFrom converts a reference read off the bus into the model type.
func RefFrom(ref eventbus.EntityRef) Ref {
	return Ref{ID: ref.ID, Type: ref.Type}
}

// New builds an entity at version 1, the state right after State accepted an
// entity.create.proposed. at is the timestamp of that proposal, not the wall
// clock: created_at and updated_at come from the event (ADR-003 p. 6).
func New(ref Ref, worldID, name string, attrs map[string]any, at time.Time) *Entity {
	if ref.Type == TypeWorld && worldID == "" {
		worldID = ref.ID
	}
	return &Entity{
		SchemaVersion: SchemaVersion,
		ID:            ref.ID,
		Type:          ref.Type,
		WorldID:       worldID,
		Name:          name,
		Version:       1,
		CreatedAt:     at,
		UpdatedAt:     at,
		Attributes:    cloneAttrs(attrs),
	}
}

// Ref is the identity of the entity.
func (e *Entity) Ref() Ref { return Ref{ID: e.ID, Type: e.Type} }

// EventEntity is the entity as an event payload carries it
// (_common.json#/$defs/EntityWithName).
func (e *Entity) EventEntity() eventbus.Entity {
	return eventbus.Entity{Entity: e.Ref().EventRef(), Name: e.Name}
}

// Clone returns a deep copy: the attributes, the history and the commit record
// of the copy share nothing with the original. State applies a proposal to a
// clone and swaps it in only once every check has passed
// (state-and-mechanics.md §4.5 p. 7).
func Clone(e *Entity) *Entity {
	if e == nil {
		return nil
	}
	out := *e
	out.Attributes = cloneAttrs(e.Attributes)
	if e.History != nil {
		out.History = make([]HistoryEntry, len(e.History))
		copy(out.History, e.History)
	}
	if e.LastChange != nil {
		lc := *e.LastChange
		lc.Changed = cloneChanges(e.LastChange.Changed)
		out.LastChange = &lc
	}
	return &out
}

// ErrVersionConflict reports that a proposal pinned a version the entity no
// longer has. State turns it into entity.update.rejected reason=version_conflict
// with both numbers in details (C-02).
type ErrVersionConflict struct {
	Ref      Ref
	Expected int64
	Actual   int64
}

func (e ErrVersionConflict) Error() string {
	return fmt.Sprintf("entity %s: expected version %d, have %d", e.Ref, e.Expected, e.Actual)
}

// CheckVersion compares the optimistic lock of a proposal with the entity.
// A nil expected version is no lock at all and always passes: the contract
// makes expected_version mandatory only for hp, status, inventory and the
// position of a fighter (ADR-013 p. 1), and enforcing that is State's job.
func (e *Entity) CheckVersion(expected *int64) error {
	if expected == nil || *expected == e.Version {
		return nil
	}
	return ErrVersionConflict{Ref: e.Ref(), Expected: *expected, Actual: e.Version}
}

// Commit moves the entity to the state that ApplyOps computed
// (state-and-mechanics.md §4.5 p. 10): the new attributes, the commit record,
// one history entry, and the version — which grows only when something actually
// changed. An empty changed list still counts as a turn and still publishes a
// fact, at the same version (C-02).
//
// change carries the identity of the proposal; its Changed and AppliedAt are
// filled in from the arguments, and FactEventID stays empty until the fact is
// published (SetFactEventID).
//
// The attributes are copied rather than adopted: the caller keeps whatever it
// was holding, and a map it goes on using — the overlay State builds between
// the check and the write (§4.5 p. 8) — cannot change the entity behind its
// back afterwards.
func (e *Entity) Commit(attrs map[string]any, changed []Change, change LastChange) {
	e.Attributes = cloneAttrs(attrs)
	if len(changed) > 0 {
		e.Version++
	}
	e.UpdatedAt = change.AppliedAt
	change.Changed = changed
	change.FactEventID = ""
	e.LastChange = &change
	e.appendHistory(HistoryEntry{
		Version:    e.Version,
		ProposalID: change.ProposalID,
		At:         change.AppliedAt,
	})
}

// SetFactEventID records the fact that announced the current version
// (state-and-mechanics.md §4.5 p. 12). It is called after the publication, on
// the entity already in the working set: the object on disk learns the id with
// the next change or in the next snapshot.
func (e *Entity) SetFactEventID(eventID string) {
	e.LastEventID = eventID
	if e.LastChange != nil {
		e.LastChange.FactEventID = eventID
	}
	if n := len(e.History); n > 0 {
		e.History[n-1].EventID = eventID
	}
}

// appendHistory adds an entry and keeps the tail at HistoryLimit.
func (e *Entity) appendHistory(entry HistoryEntry) {
	e.History = append(e.History, entry)
	if len(e.History) > HistoryLimit {
		e.History = e.History[len(e.History)-HistoryLimit:]
	}
}

func cloneChanges(in []Change) []Change {
	if in == nil {
		return nil
	}
	out := make([]Change, len(in))
	for i, c := range in {
		out[i] = Change{Path: c.Path, Old: snapshot(c.Old), New: snapshot(c.New)}
	}
	return out
}
