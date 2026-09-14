// Package snapshot writes the snapshot of the gateway: an object
// snapshots-{world}/gateway/{ts}-{seq}.json with the cursors, the hash of the
// projection, the active sessions and the open rounds, the pointer
// snapshots-{world}/gateway/latest.json after it, and the event
// snapshot.created component=gateway (C-14 v1.1, component gateway-and-bot.md
// §11.2).
//
// The snapshot is not the truth of the gateway: gateway.db is. It is written
// for the audit and for a quick check of the projection against State, so a
// snapshot that cannot be written costs a line of the log and the health of
// the gateway, never an action of a player.
//
// The layout of the pointer and of the object follows the one State writes
// (state-and-mechanics.md §4.4): the pointer is written only after its object,
// size_bytes lives in the pointer only, and snapshot.created is the block of
// the pointer without the fields its closed schema does not know.
package snapshot

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
)

// Component is the value of component of the snapshot of the gateway (C-14).
const Component = "gateway"

// PointerKey is the key of the pointer inside snapshots-{world}.
const PointerKey = Component + "/latest.json"

// TypeCreated is the event announcing a written snapshot.
const TypeCreated = "snapshot.created"

// Keep is how many snapshots stay after a write; the pointer is not one of
// them (C-14 v1.1, ADR-021 p. 3: the rotation lives in code, not in bucket
// versioning).
const Keep = 5

// SchemaVersion is the shape of the pointer and of the object.
const SchemaVersion = 1

// WrittenBy identifies the writer of a pointer.
const WrittenBy = contracts.SourceGateway

// The reasons a snapshot is taken (component §11.2).
const (
	ReasonShutdown     = "shutdown"
	ReasonSessionEnded = "session_ended"
)

var (
	// ErrNoLawsVersion: the projection holds no world entity, so the snapshot
	// has no laws_version, which snapshot.created requires. Nothing is
	// written: a snapshot that cannot be announced is not taken.
	ErrNoLawsVersion = errors.New("snapshot: the world of the projection has no laws_version")
	// ErrNoSnapshot: the gateway has not written a snapshot of this world yet.
	ErrNoSnapshot = errors.New("snapshot: no snapshot of the gateway")
	// ErrCorrupted: a pointer is there, but it or the object it names is not
	// what the gateway writes.
	ErrCorrupted = errors.New("snapshot: the snapshot of the gateway is corrupted")
)

// Publisher publishes snapshot.created; eventbus.Bus is one.
type Publisher interface {
	Publish(ctx context.Context, ev eventbus.Event) error
}

// Session is an active session as the snapshot records it: identifiers and
// times, no names (SEC-01/03).
type Session struct {
	ID           string            `json:"id"`
	WorldID      string            `json:"world_id"`
	Scope        eventbus.ScopeRef `json:"scope"`
	Kind         string            `json:"kind"`
	ActorKind    string            `json:"actor_kind"`
	Participants []string          `json:"participants"`
	StartedAt    time.Time         `json:"started_at"`
	LastActionAt time.Time         `json:"last_action_at"`
	TurnsCount   int               `json:"turns_count"`
}

// Round is an open or closing round of a group (table rounds, ADR-020).
type Round struct {
	ScopeID     string    `json:"scope_id"`
	Seq         int       `json:"seq"`
	EncounterID string    `json:"encounter_id"`
	State       string    `json:"state"`
	DeadlineAt  time.Time `json:"deadline_at"`
}

// Cursors are the two cursors of the gateway per topic, both as the offset of
// the next event not yet taken: the projection in memory and the effects in
// gateway.db (component §11.2).
type Cursors struct {
	Projection map[string]int64 `json:"projection"`
	Effects    map[string]int64 `json:"effects"`
}

// State is what one snapshot records.
type State struct {
	Cursors        Cursors
	ProjectionHash string
	ActiveSessions []Session
	OpenRounds     []Round
	LawsVersion    string
}

// Meta is the block the pointer and the object share. SizeBytes is set in the
// pointer only: an object cannot state its own length.
type Meta struct {
	ID          string           `json:"id"`
	Seq         int64            `json:"seq"`
	Key         string           `json:"key"`
	TakenAt     time.Time        `json:"taken_at"`
	Cursor      map[string]int64 `json:"cursor"`
	LawsVersion string           `json:"laws_version"`
	StateHash   string           `json:"state_hash"`
	SizeBytes   *int64           `json:"size_bytes,omitempty"`
	Reason      string           `json:"reason"`
}

// Pointer is snapshots-{world}/gateway/latest.json.
type Pointer struct {
	SchemaVersion int    `json:"schema_version"`
	Component     string `json:"component"`
	World         struct {
		Entity entity.Ref `json:"entity"`
	} `json:"world"`
	Snapshot  Meta      `json:"snapshot"`
	WrittenAt time.Time `json:"written_at"`
	Writer    string    `json:"writer"`
}

// Object is the snapshot itself (component §11.2).
type Object struct {
	SchemaVersion  int       `json:"schema_version"`
	Component      string    `json:"component"`
	Snapshot       Meta      `json:"snapshot"`
	WorldID        string    `json:"world_id"`
	Cursors        Cursors   `json:"cursors"`
	ProjectionHash string    `json:"projection_hash"`
	ActiveSessions []Session `json:"active_sessions"`
	OpenRounds     []Round   `json:"open_rounds"`
	LawsVersion    string    `json:"laws_version"`
}

// Key is the key of the object of seq taken at takenAt:
// gateway/{YYYYMMDDTHHMMSSZ}-{seq:06d}.json, the form of State (§4.3).
func Key(takenAt time.Time, seq int64) string {
	return fmt.Sprintf("%s/%s-%06d.json", Component, takenAt.UTC().Format("20060102T150405Z"), seq)
}

var keyPattern = regexp.MustCompile(`^` + Component + `/\d{8}T\d{6}Z-(\d{6,})\.json$`)

// seqOf is the sequence of a key of an object; false for any other key, the
// pointer included.
func seqOf(key string) (int64, bool) {
	m := keyPattern.FindStringSubmatch(key)
	if m == nil {
		return 0, false
	}
	seq, err := strconv.ParseInt(m[1], 10, 64)
	return seq, err == nil
}

// Config builds a writer. Objects, WorldID and Clock are required.
type Config struct {
	Objects objstore.Client
	// Bus receives snapshot.created; nil writes the files and announces
	// nothing.
	Bus     Publisher
	WorldID string
	Clock   clock.Clock
	Log     *slog.Logger
}

// Writer writes the snapshots of one world. Writes are serialized: two
// triggers at once cannot take the same sequence.
type Writer struct {
	cfg Config

	mu   sync.Mutex
	next int64
	// scanned says next was read from the store.
	scanned bool
}

// New checks cfg.
func New(cfg Config) (*Writer, error) {
	if cfg.Objects == nil || cfg.WorldID == "" || cfg.Clock == nil {
		return nil, errors.New("snapshot: Objects, WorldID and Clock are required")
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Writer{cfg: cfg}, nil
}

// Latest reads the pointer and the object it names and checks that they are a
// snapshot of the gateway of this world: ErrNoSnapshot when there is no
// pointer, ErrCorrupted when either does not check, another error when the
// store could not be read.
func (w *Writer) Latest(ctx context.Context) (Pointer, error) {
	bucket := objstore.SnapshotsBucket(w.cfg.WorldID)
	body, err := w.cfg.Objects.Get(ctx, bucket, PointerKey)
	switch {
	case errors.Is(err, objstore.ErrNotFound), errors.Is(err, objstore.ErrNoBucket):
		return Pointer{}, ErrNoSnapshot
	case err != nil:
		return Pointer{}, fmt.Errorf("snapshot: read %s/%s: %w", bucket, PointerKey, err)
	}
	var pointer Pointer
	if err := json.Unmarshal(body, &pointer); err != nil {
		return Pointer{}, fmt.Errorf("%w: pointer %s/%s: %w", ErrCorrupted, bucket, PointerKey, err)
	}
	seq, ok := seqOf(pointer.Snapshot.Key)
	if pointer.Component != Component || !ok || seq != pointer.Snapshot.Seq || pointer.Snapshot.Key != Key(pointer.Snapshot.TakenAt, seq) {
		return Pointer{}, fmt.Errorf("%w: pointer %s/%s names no snapshot of the gateway", ErrCorrupted, bucket, PointerKey)
	}
	body, err = w.cfg.Objects.Get(ctx, bucket, pointer.Snapshot.Key)
	switch {
	case errors.Is(err, objstore.ErrNotFound):
		// The pointer is written after its object: a pointer without one is
		// damage, not a new world.
		return Pointer{}, fmt.Errorf("%w: %s/%s is missing", ErrCorrupted, bucket, pointer.Snapshot.Key)
	case err != nil:
		return Pointer{}, fmt.Errorf("snapshot: read %s/%s: %w", bucket, pointer.Snapshot.Key, err)
	}
	var object Object
	if err := json.Unmarshal(body, &object); err != nil {
		return Pointer{}, fmt.Errorf("%w: %s/%s: %w", ErrCorrupted, bucket, pointer.Snapshot.Key, err)
	}
	if object.Component != Component || object.WorldID != w.cfg.WorldID || object.Snapshot.ID != pointer.Snapshot.ID ||
		object.ProjectionHash != pointer.Snapshot.StateHash ||
		(pointer.Snapshot.SizeBytes != nil && *pointer.Snapshot.SizeBytes != int64(len(body))) {
		return Pointer{}, fmt.Errorf("%w: %s/%s does not match its pointer", ErrCorrupted, bucket, pointer.Snapshot.Key)
	}
	return pointer, nil
}

// Write takes a snapshot of st: the object, then the pointer, then the
// rotation down to Keep, then snapshot.created. It returns the pointer as
// written. A failed rotation is logged and does not fail the write; a failed
// announcement does, after the files are in place.
func (w *Writer) Write(ctx context.Context, st State, reason string) (Pointer, error) {
	if st.LawsVersion == "" {
		return Pointer{}, ErrNoLawsVersion
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	bucket := objstore.SnapshotsBucket(w.cfg.WorldID)
	if err := w.cfg.Objects.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		return Pointer{}, fmt.Errorf("snapshot: %w", err)
	}
	if !w.scanned {
		keys, err := w.keys(ctx, bucket)
		if err != nil {
			return Pointer{}, err
		}
		if len(keys) > 0 {
			w.next = keys[len(keys)-1].seq + 1
		}
		w.scanned = true
	}
	seq := w.next
	// The sequence is taken whatever becomes of this write: a retry after a
	// failed pointer must not reuse the key of an object already written.
	w.next++

	takenAt := w.cfg.Clock.Now().UTC()
	meta := Meta{
		ID:          fmt.Sprintf("%s:%s:%06d", Component, w.cfg.WorldID, seq),
		Seq:         seq,
		Key:         Key(takenAt, seq),
		TakenAt:     takenAt,
		Cursor:      clone(st.Cursors.Projection),
		LawsVersion: st.LawsVersion,
		StateHash:   st.ProjectionHash,
		Reason:      reason,
	}
	object := Object{
		SchemaVersion: SchemaVersion, Component: Component, Snapshot: meta, WorldID: w.cfg.WorldID,
		Cursors:        Cursors{Projection: clone(st.Cursors.Projection), Effects: clone(st.Cursors.Effects)},
		ProjectionHash: st.ProjectionHash,
		ActiveSessions: nonNil(st.ActiveSessions),
		OpenRounds:     nonNil(st.OpenRounds),
		LawsVersion:    st.LawsVersion,
	}
	body, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return Pointer{}, fmt.Errorf("snapshot: encode %s: %w", meta.Key, err)
	}
	if _, err := w.cfg.Objects.Put(ctx, bucket, meta.Key, body, objstore.PutOptions{ContentType: "application/json"}); err != nil {
		return Pointer{}, fmt.Errorf("snapshot: put %s: %w", meta.Key, err)
	}

	size := int64(len(body))
	pointer := Pointer{SchemaVersion: SchemaVersion, Component: Component, Snapshot: meta, WrittenAt: w.cfg.Clock.Now().UTC(), Writer: WrittenBy}
	pointer.Snapshot.SizeBytes = &size
	pointer.World.Entity = entity.Ref{ID: w.cfg.WorldID, Type: entity.TypeWorld}
	pointerBody, err := json.MarshalIndent(pointer, "", "  ")
	if err != nil {
		return Pointer{}, fmt.Errorf("snapshot: encode %s: %w", PointerKey, err)
	}
	if _, err := w.cfg.Objects.Put(ctx, bucket, PointerKey, pointerBody, objstore.PutOptions{ContentType: "application/json"}); err != nil {
		return Pointer{}, fmt.Errorf("snapshot: put %s: %w", PointerKey, err)
	}
	if err := w.rotate(ctx, bucket); err != nil {
		w.cfg.Log.Error("snapshot rotation", slog.String("error", err.Error()))
	}
	w.cfg.Log.Info("snapshot written", slog.Int64("seq", seq), slog.String("key", meta.Key),
		slog.String("reason", reason), slog.Int64("size_bytes", size))
	if w.cfg.Bus != nil {
		if err := w.cfg.Bus.Publish(ctx, Created(w.cfg.WorldID, pointer)); err != nil {
			return pointer, fmt.Errorf("snapshot: publish %s of %s: %w", TypeCreated, meta.ID, err)
		}
	}
	return pointer, nil
}

// Created is snapshot.created of a written pointer: the block of the pointer
// without reason, which the closed schema does not know. The world is in the
// envelope only (C-14 v1.3).
func Created(worldID string, p Pointer) eventbus.Event {
	var size int64
	if p.Snapshot.SizeBytes != nil {
		size = *p.Snapshot.SizeBytes
	}
	cursor := make(map[string]any, len(p.Snapshot.Cursor))
	for topic, offset := range p.Snapshot.Cursor {
		cursor[topic] = offset
	}
	return eventbus.NewRoot(TypeCreated, contracts.SourceGateway, worldID, nil, eventbus.ActorSystem, map[string]any{
		"component": Component,
		"snapshot": map[string]any{
			"id":           p.Snapshot.ID,
			"seq":          p.Snapshot.Seq,
			"taken_at":     p.Snapshot.TakenAt.UTC().Format(time.RFC3339Nano),
			"cursor":       cursor,
			"laws_version": p.Snapshot.LawsVersion,
			"state_hash":   p.Snapshot.StateHash,
			"size_bytes":   size,
			"key":          p.Snapshot.Key,
		},
	})
}

type keyed struct {
	key string
	seq int64
}

// keys lists the objects of the gateway in the bucket by sequence.
func (w *Writer) keys(ctx context.Context, bucket string) ([]keyed, error) {
	infos, err := w.cfg.Objects.List(ctx, bucket, Component+"/")
	if err != nil {
		return nil, fmt.Errorf("snapshot: list %s/%s/: %w", bucket, Component, err)
	}
	out := make([]keyed, 0, len(infos))
	for _, info := range infos {
		if seq, ok := seqOf(info.Key); ok {
			out = append(out, keyed{key: info.Key, seq: seq})
		}
	}
	slices.SortFunc(out, func(a, b keyed) int { return cmp.Compare(a.seq, b.seq) })
	return out, nil
}

// rotate deletes every object but the last Keep.
func (w *Writer) rotate(ctx context.Context, bucket string) error {
	keys, err := w.keys(ctx, bucket)
	if err != nil {
		return err
	}
	var errs []error
	for i := 0; i < len(keys)-Keep; i++ {
		if err := w.cfg.Objects.Delete(ctx, bucket, keys[i].key); err != nil {
			errs = append(errs, fmt.Errorf("snapshot: delete %s: %w", keys[i].key, err))
		}
	}
	return errors.Join(errs...)
}

func clone(m map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(m))
	maps.Copy(out, m)
	return out
}

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
