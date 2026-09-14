package readmodel

import (
	"context"
	"fmt"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
)

// TypeSnapshotCreated announces a written snapshot (C-14); the one of State
// repairs the stale entities of the projection.
const TypeSnapshotCreated = "snapshot.created"

// RepairError is a repair that could not read or check the snapshot it was
// told about. Reason is one of the Reason codes; the error behind it names the
// bucket and the key and belongs to the log only.
type RepairError struct {
	Reason string
	Err    error
}

func (e *RepairError) Error() string { return e.Reason + ": " + e.Err.Error() }

func (e *RepairError) Unwrap() error { return e.Err }

// HasStale says whether the projection holds an entity known to be behind State.
func (m *Model) HasStale() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.stale) > 0
}

// snapshotAnnouncement is the part of snapshot.created a repair reads.
type snapshotAnnouncement struct {
	Component string `json:"component"`
	Snapshot  struct {
		Key       string `json:"key"`
		StateHash string `json:"state_hash"`
	} `json:"snapshot"`
}

// RepairFromStateSnapshot takes the stale entities of the projection from the
// snapshot of State that ev announces (component §11.2, "projection=stale и его
// снятие"). It returns how many stale entities it replaced and how many are
// left stale.
//
// The store is read only for snapshot.created of component state and of
// worldID, while the projection has stale entities and a store is there; any
// other event is left alone. The object is the one of snapshot.key, not
// latest.json, and it is taken only when its entities hash to the state_hash
// of the event. Each stale entity whose version in the snapshot is not below
// the one of the projection is replaced by a copy from the snapshot and is no
// longer stale; one the snapshot holds at a lower version, or not at all,
// stays stale. Nothing else changes: the other entities, the cursor and the
// mark of the load.
//
// An object that cannot be read or does not check leaves every stale entity
// as it was, and the error is a *RepairError.
func (m *Model) RepairFromStateSnapshot(ctx context.Context, store objstore.Client, worldID string, ev eventbus.Event) (repaired, left int, err error) {
	if ev.Type != TypeSnapshotCreated || store == nil || ev.World == nil || ev.World.Entity.ID != worldID || !m.HasStale() {
		return 0, m.staleCount(), nil
	}
	var announced snapshotAnnouncement
	if err := decode(ev, &announced); err != nil {
		return 0, m.staleCount(), &RepairError{Reason: ReasonSnapshotUnreadable, Err: err}
	}
	if announced.Component != stateComponent {
		return 0, m.staleCount(), nil
	}
	if announced.Snapshot.Key == "" {
		return 0, m.staleCount(), &RepairError{Reason: ReasonSnapshotUnreadable,
			Err: fmt.Errorf("readmodel: %s %s names no object", ev.Type, ev.ID)}
	}
	entities, err := readStateObject(ctx, store, worldID, announced.Snapshot.Key, announced.Snapshot.StateHash)
	if err != nil {
		return 0, m.staleCount(), &RepairError{Reason: reasonOf(ctx, err), Err: err}
	}
	byID := make(map[string]*entity.Entity, len(entities))
	for _, e := range entities {
		byID[e.ID] = e
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.stale {
		snap, ok := byID[id]
		cur, known := m.entities[id]
		if !ok || (known && snap.Version < cur.Version) {
			continue
		}
		m.entities[id] = entity.Clone(snap)
		delete(m.stale, id)
		if snap.Type == entity.TypeEncounter {
			m.reindex(id)
		}
		repaired++
	}
	return repaired, len(m.stale), nil
}

func (m *Model) staleCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.stale)
}

// IsStateSnapshotOf says whether ev is snapshot.created of State for worldID:
// the announcement a repair reads from.
func IsStateSnapshotOf(ev eventbus.Event, worldID string) bool {
	if ev.Type != TypeSnapshotCreated || ev.World == nil || ev.World.Entity.ID != worldID {
		return false
	}
	component, _ := ev.Payload["component"].(string)
	return component == stateComponent
}
