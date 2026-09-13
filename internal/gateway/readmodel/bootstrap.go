package readmodel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/objstore"
)

// StatePointerKey is the pointer of the latest snapshot of State inside
// snapshots-{world} (C-14 v1.1).
const StatePointerKey = "state/latest.json"

// ErrNoSnapshot is a world whose State has not written a snapshot yet. It is
// the ordinary start of a new world, not a failure: the projection is built
// from the journal alone and reports itself missing.
var ErrNoSnapshot = errors.New("readmodel: no snapshot of state")

// The reasons a snapshot of State did not load, as Status and /health report
// them. They are short codes on purpose: the error behind one names the
// bucket, the key and the address of the store, and goes to the log only.
const (
	// ReasonStoreMisconfigured: the variables of the object store are there
	// but do not make a client — one key without the other, an empty
	// endpoint, a flag that is not a boolean.
	ReasonStoreMisconfigured = "store_misconfigured"
	// ReasonSnapshotUnreadable: the store could not be read, the pointer or
	// the object is not what State writes, or the pointer names no object.
	ReasonSnapshotUnreadable = "snapshot_unreadable"
	// ReasonSnapshotTimeout: the load did not finish within its budget.
	ReasonSnapshotTimeout = "snapshot_timeout"
	// ReasonHashMismatch: the entities do not hash to the state_hash of the
	// pointer.
	ReasonHashMismatch = "hash_mismatch"
	// ReasonWorldMismatch: the object is the snapshot of another world.
	ReasonWorldMismatch = "world_mismatch"
)

var (
	errHashMismatch  = errors.New("hash mismatch")
	errWorldMismatch = errors.New("world mismatch")
)

// statePointer is the part of snapshots-{world}/state/latest.json the gateway
// reads (state-and-mechanics.md §4.4).
type statePointer struct {
	Component string       `json:"component"`
	Snapshot  snapshotMeta `json:"snapshot"`
}

type snapshotMeta struct {
	Key       string           `json:"key"`
	Cursor    map[string]int64 `json:"cursor"`
	StateHash string           `json:"state_hash"`
}

type stateObject struct {
	Component string           `json:"component"`
	WorldID   string           `json:"world_id"`
	Snapshot  snapshotMeta     `json:"snapshot"`
	Entities  []*entity.Entity `json:"entities"`
}

// LoadFromStateSnapshot replaces the entities of the projection with the latest
// snapshot of State and returns the cursor the snapshot was taken at: the
// offsets from which the journal has to be read to catch up (C-14).
//
// The pointer is read first and the object it names second, and the object is
// taken only when its entities hash to the state_hash of the pointer: a
// projection loaded from a torn or foreign object would validate actions
// against a world nobody has. Without a pointer the error is ErrNoSnapshot.
// Any other failure leaves the projection empty and missing, with its reason
// in Status, so that it does not come up empty in silence (US-011).
func (m *Model) LoadFromStateSnapshot(ctx context.Context, store objstore.Client, worldID string) (map[string]int64, error) {
	cursor, entities, err := readStateSnapshot(ctx, store, worldID)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.loaded, m.loadErr = false, ""
		if !errors.Is(err, ErrNoSnapshot) {
			m.loadErr = reasonOf(ctx, err)
		}
		return nil, err
	}
	m.entities = make(map[string]*entity.Entity, len(entities))
	for _, e := range entities {
		m.entities[e.ID] = e
	}
	m.cursor = make(map[string]int64, len(cursor))
	for topic, offset := range cursor {
		m.cursor[topic] = offset
	}
	m.stale = make(map[string]struct{})
	m.open, m.listed = make(map[string]map[string]struct{}), make(map[string][]string)
	for id := range m.encounters {
		m.reindex(id)
	}
	for _, e := range entities {
		if e.Type == entity.TypeEncounter {
			m.reindex(e.ID)
		}
	}
	m.loaded, m.loadErr = true, ""
	return cursor, nil
}

// MarkLoadFailed records a load that could not even be tried, such as an
// object store whose configuration does not make a client: the projection
// stays missing and Status reports reason.
func (m *Model) MarkLoadFailed(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loaded, m.loadErr = false, reason
}

func reasonOf(ctx context.Context, err error) string {
	switch {
	case errors.Is(err, errHashMismatch):
		return ReasonHashMismatch
	case errors.Is(err, errWorldMismatch):
		return ReasonWorldMismatch
	case errors.Is(err, context.DeadlineExceeded), ctx.Err() != nil:
		return ReasonSnapshotTimeout
	default:
		return ReasonSnapshotUnreadable
	}
}

func readStateSnapshot(ctx context.Context, store objstore.Client, worldID string) (map[string]int64, []*entity.Entity, error) {
	if store == nil {
		return nil, nil, fmt.Errorf("%w: no object store", ErrNoSnapshot)
	}
	bucket := objstore.SnapshotsBucket(worldID)
	body, err := store.Get(ctx, bucket, StatePointerKey)
	switch {
	case errors.Is(err, objstore.ErrNotFound), errors.Is(err, objstore.ErrNoBucket):
		return nil, nil, fmt.Errorf("%w: %s/%s", ErrNoSnapshot, bucket, StatePointerKey)
	case err != nil:
		return nil, nil, fmt.Errorf("readmodel: read %s/%s: %w", bucket, StatePointerKey, err)
	}
	var pointer statePointer
	if err := json.Unmarshal(body, &pointer); err != nil {
		return nil, nil, fmt.Errorf("readmodel: pointer %s/%s: %w", bucket, StatePointerKey, err)
	}
	if pointer.Component != "state" || pointer.Snapshot.Key == "" {
		return nil, nil, fmt.Errorf("readmodel: pointer %s/%s names no snapshot of state", bucket, StatePointerKey)
	}
	body, err = store.Get(ctx, bucket, pointer.Snapshot.Key)
	if err != nil {
		// The pointer is written after the object (C-14): a pointer without
		// its object is damage, not a new world.
		return nil, nil, fmt.Errorf("readmodel: snapshot %s/%s: %w", bucket, pointer.Snapshot.Key, err)
	}
	var object stateObject
	if err := json.Unmarshal(body, &object); err != nil {
		return nil, nil, fmt.Errorf("readmodel: snapshot %s/%s: %w", bucket, pointer.Snapshot.Key, err)
	}
	if object.WorldID != worldID {
		return nil, nil, fmt.Errorf("readmodel: snapshot %s/%s is of world %q: %w", bucket, pointer.Snapshot.Key, object.WorldID, errWorldMismatch)
	}
	if got := entity.StateHash(object.Entities); got != pointer.Snapshot.StateHash {
		return nil, nil, fmt.Errorf("readmodel: snapshot %s/%s hashes to %s, the pointer says %s: %w",
			bucket, pointer.Snapshot.Key, got, pointer.Snapshot.StateHash, errHashMismatch)
	}
	for _, e := range object.Entities {
		if e == nil || e.ID == "" {
			return nil, nil, fmt.Errorf("readmodel: snapshot %s/%s holds an entity without an id", bucket, pointer.Snapshot.Key)
		}
	}
	return pointer.Snapshot.Cursor, object.Entities, nil
}
