package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/objstore"
)

// Component is the value of the component field of a snapshot: the double writes
// the snapshot of State and nothing else (C-14 v1.1).
const Component = "state"

// Writer identifies who wrote a pointer. The real State writes
// core/state@host:pid; the double says plainly that it is the double.
const Writer = Source + "@fake:0"

// The reasons a snapshot is taken (§4.9). The double does not take one on a
// timer or on a shutdown — a test asks for one — so bootstrap and admin are
// what it normally carries.
const (
	ReasonBootstrap    = "bootstrap"
	ReasonInterval     = "interval"
	ReasonSessionEnded = "session_ended"
	ReasonShutdown     = "shutdown"
	ReasonAdmin        = "admin"
)

// SnapshotMeta is the block §4.4 puts both into the pointer and into the
// object it points at.
//
// SizeBytes is a pointer because only the pointer can carry it: an object
// cannot state its own length without changing it (the same decision the
// fixtures of T-016 record).
type SnapshotMeta struct {
	ID            string           `json:"id"`
	Seq           int64            `json:"seq"`
	Key           string           `json:"key"`
	TakenAt       time.Time        `json:"taken_at"`
	Cursor        map[string]int64 `json:"cursor"`
	LawsVersion   string           `json:"laws_version"`
	RulesVersion  string           `json:"rules_version"`
	StateHash     string           `json:"state_hash"`
	SizeBytes     *int64           `json:"size_bytes,omitempty"`
	EntitiesCount int              `json:"entities_count"`
	Reason        string           `json:"reason"`
}

// Pointer is snapshots-{world}/state/latest.json (§4.4): the metadata plus the
// key of the object, written after the object itself.
type Pointer struct {
	SchemaVersion int    `json:"schema_version"`
	Component     string `json:"component"`
	World         struct {
		Entity entity.Ref `json:"entity"`
	} `json:"world"`
	Snapshot  SnapshotMeta `json:"snapshot"`
	WrittenAt time.Time    `json:"written_at"`
	Writer    string       `json:"writer"`
}

// Object is the snapshot itself: the same metadata block, the entities in
// (type, id) order and the proposals already applied (§4.4).
type Object struct {
	SchemaVersion    int              `json:"schema_version"`
	Component        string           `json:"component"`
	Snapshot         SnapshotMeta     `json:"snapshot"`
	WorldID          string           `json:"world_id"`
	Entities         []*entity.Entity `json:"entities"`
	AppliedProposals []string         `json:"applied_proposals"`
}

// SnapshotSchemaVersion is the shape of both files.
const SnapshotSchemaVersion = 1

// PointerKey is the key of the pointer inside snapshots-{world}.
const PointerKey = Component + "/latest.json"

// SnapshotKey is the key of the object: state/{YYYYMMDDTHHMMSSZ}-{seq:06d}.json
// (§4.3). It is derived from the instant and the sequence rather than named, so
// that a key cannot describe a moment other than the one it was taken at.
func SnapshotKey(takenAt time.Time, seq int64) string {
	return fmt.Sprintf("%s/%s-%06d.json", Component, takenAt.UTC().Format("20060102T150405Z"), seq)
}

// Snapshot writes the world into the object store and then the pointer at it,
// and returns the pointer as it was written (§4.4, C-14).
//
// The order is the guarantee: the pointer is written only after the object is
// there, so a reader that finds a pointer finds the object it names. The
// counters are recomputed here and never remembered — state_hash from the
// entities, entities_count from their number, size_bytes from the bytes of the
// object — which is what makes the result verifiable by a consumer that
// recomputes them itself.
//
// What the double does not do: it does not publish snapshot.created. The
// registry lists state, swarm and gateway as the publishers of that type and
// not testkit/state (contracts.md §0), and a stub that published a type it is
// not a publisher of would fail the contracts job instead of standing in for
// one.
func (s *FakeState) Snapshot(ctx context.Context, reason string) (Pointer, error) {
	if s.store == nil {
		return Pointer{}, errors.New("testkit/state: snapshot: no object store")
	}
	if reason == "" {
		reason = ReasonAdmin
	}

	// The world, the window and the cursor are read between two decisions, so
	// that they describe one moment of the world.
	s.deciding.Lock()
	entities := s.All()
	proposals := s.AppliedProposals()
	s.mu.Lock()
	seq := s.seq
	s.seq++
	cursor := s.cursor
	s.mu.Unlock()
	s.deciding.Unlock()
	meta := SnapshotMeta{
		Seq:           seq,
		Cursor:        map[string]int64{"system_events": cursor},
		LawsVersion:   lawsVersion(entities, s.worldID),
		RulesVersion:  s.rules,
		StateHash:     entity.StateHash(entities),
		EntitiesCount: len(entities),
		Reason:        reason,
	}

	meta.TakenAt = s.clock.Now().UTC()
	meta.ID = fmt.Sprintf("%s:%s:%06d", Component, s.worldID, seq)
	meta.Key = SnapshotKey(meta.TakenAt, seq)

	object := Object{
		SchemaVersion:    SnapshotSchemaVersion,
		Component:        Component,
		Snapshot:         meta,
		WorldID:          s.worldID,
		Entities:         entities,
		AppliedProposals: proposals,
	}
	body, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return Pointer{}, fmt.Errorf("testkit/state: snapshot: encode object: %w", err)
	}

	bucket := objstore.SnapshotsBucket(s.worldID)
	if err := s.store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		return Pointer{}, fmt.Errorf("testkit/state: snapshot: %w", err)
	}
	if _, err := s.store.Put(ctx, bucket, meta.Key, body,
		objstore.PutOptions{ContentType: "application/json"}); err != nil {
		return Pointer{}, fmt.Errorf("testkit/state: snapshot: put %s: %w", meta.Key, err)
	}

	size := int64(len(body))
	pointerMeta := meta
	pointerMeta.SizeBytes = &size
	pointer := Pointer{
		SchemaVersion: SnapshotSchemaVersion,
		Component:     Component,
		Snapshot:      pointerMeta,
		WrittenAt:     s.clock.Now().UTC(),
		Writer:        Writer,
	}
	pointer.World.Entity = entity.Ref{ID: s.worldID, Type: entity.TypeWorld}

	pointerBody, err := json.MarshalIndent(pointer, "", "  ")
	if err != nil {
		return Pointer{}, fmt.Errorf("testkit/state: snapshot: encode pointer: %w", err)
	}
	if _, err := s.store.Put(ctx, bucket, PointerKey, pointerBody,
		objstore.PutOptions{ContentType: "application/json"}); err != nil {
		return Pointer{}, fmt.Errorf("testkit/state: snapshot: put %s: %w", PointerKey, err)
	}
	s.log.Info("snapshot written", "seq", seq, "key", meta.Key,
		"entities", meta.EntitiesCount, "state_hash", meta.StateHash)
	return pointer, nil
}

// lawsVersion reads the laws the world is running under off the world entity,
// which is where they live (data-model.md §3.1).
func lawsVersion(entities []*entity.Entity, worldID string) string {
	for _, e := range entities {
		if e.ID == worldID {
			version, _ := e.LawsVersion()
			return version
		}
	}
	return ""
}
