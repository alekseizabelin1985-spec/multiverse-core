package state

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/objstore"
)

// ErrNotFound is an entity or a snapshot object the store does not hold.
var ErrNotFound = errors.New("state: not found in the store")

// ErrUndecodable is an object of the store that is there but does not read as
// what its key says it is: a snapshot object cut short or overwritten.
var ErrUndecodable = errors.New("state: undecodable object")

// ErrNoSnapshot is a world without latest.json: no snapshot was ever written
// for it (state-and-mechanics.md §4.8).
var ErrNoSnapshot = errors.New("state: no snapshot")

// Store is where State keeps what it decided, behind the working set in memory
// (state-and-mechanics.md §4.3): one object per entity as the commit record of
// its last change (ADR-011 p. 1), the intent of a package that changes more
// than one entity (ADR-013 p. 4), and the rotated snapshots of the world with
// the pointer at the last one (ADR-011 p. 2, C-14).
//
// A write replaces the whole object. Nothing here depends on bucket
// versioning (ADR-021 p. 3).
type Store interface {
	// ListEntities returns every entity of the world in (type, id) order.
	ListEntities(ctx context.Context, worldID string) ([]*entity.Entity, error)
	// GetEntity returns one entity, or ErrNotFound.
	GetEntity(ctx context.Context, worldID, entityType, id string) (*entity.Entity, error)
	// PutEntity writes the entity as a whole.
	PutEntity(ctx context.Context, worldID string, e *entity.Entity) error

	// PutIntent writes the intent of a package before its entities.
	PutIntent(ctx context.Context, worldID string, in *Intent) error
	// ListIntents returns the intents left behind, in key order.
	ListIntents(ctx context.Context, worldID string) ([]*Intent, error)
	// DeleteIntent removes the intent once every entity of the package is
	// written. Removing an intent that is not there is not an error.
	DeleteIntent(ctx context.Context, worldID, proposalID string) error

	// PutSnapshot writes the snapshot object, then latest.json pointing at it,
	// and returns the pointer as written. A failure after the object leaves the
	// pointer at the snapshot before (§4.4).
	PutSnapshot(ctx context.Context, worldID string, s *Snapshot, writtenAt time.Time, writer string) (*LatestPointer, error)
	// ReadLatest returns the pointer, or ErrNoSnapshot.
	ReadLatest(ctx context.Context, worldID string) (*LatestPointer, error)
	// ReadSnapshot returns the snapshot object under key, or ErrNotFound.
	ReadSnapshot(ctx context.Context, worldID, key string) (*Snapshot, error)
	// ListSnapshots returns the snapshot objects of the world, the highest seq
	// first; latest.json is not one of them.
	ListSnapshots(ctx context.Context, worldID string) ([]SnapshotRef, error)
	// DeleteSnapshot removes one snapshot object.
	DeleteSnapshot(ctx context.Context, worldID, key string) error
}

// Intent is entities-{world}/_intents/{proposal_id}.json: what an atomic
// package that changes more than one entity is about to write, put before the
// first entity and removed after the last (ADR-013 p. 4, §4.7). A restart that
// finds one rolls the package forward (T-059).
//
// AppliedAt is the instant the package was applied at, which the commit record
// of an entity rolled forward keeps, so that its fact sent later carries the
// applied_at of the entities written before the cut (T-059). An intent written
// before the field existed reads as the zero instant, and the fact then takes
// the instant of the proposal at hand.
type Intent struct {
	ProposalID      string         `json:"proposal_id"`
	ProposalEventID string         `json:"proposal_event_id"`
	World           string         `json:"world"`
	Cause           string         `json:"cause"`
	AppliedAt       time.Time      `json:"applied_at,omitzero"`
	Changes         []IntentChange `json:"changes"`
}

// IntentChange is one entity of an intent: the version it leaves, the version
// it reaches and the attributes and changes it reaches it with.
type IntentChange struct {
	Ref             entity.Ref      `json:"ref"`
	FromVersion     int64           `json:"from_version"`
	ToVersion       int64           `json:"to_version"`
	AttributesAfter map[string]any  `json:"attributes_after"`
	Changed         []entity.Change `json:"changed"`
}

// SnapshotRef is one snapshot object of a world as the listing shows it.
type SnapshotRef struct {
	Key string
	Seq int64
}

// intentsPrefix is the folder of the intents inside entities-{world}. The
// leading underscore keeps it apart from every entity type.
const intentsPrefix = "_intents/"

// jsonObject is the metadata every object of State is written with (§4.3).
var jsonObject = objstore.PutOptions{ContentType: "application/json"}

// objectStore is Store over the object store of the platform.
type objectStore struct {
	client objstore.Client
}

// NewObjectStore is the Store of State over shared/objstore (MinIO in the
// platform, objstore.Memory in the tests). The buckets are not created here:
// mvctl world init creates them (§4.3, EnsureWorldBuckets).
func NewObjectStore(client objstore.Client) Store {
	return &objectStore{client: client}
}

// EnsureWorldBuckets creates entities-{world} and snapshots-{world} with the
// rules of their layout — versioning and the expiry of non-current versions
// where the server has them (ADR-021 p. 2–3, §4.3).
func EnsureWorldBuckets(ctx context.Context, client objstore.Client, worldID string) error {
	for _, bucket := range []string{objstore.EntitiesBucket(worldID), objstore.SnapshotsBucket(worldID)} {
		if err := client.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
			return fmt.Errorf("state: ensure bucket %s: %w", bucket, err)
		}
	}
	return nil
}

// entityKey is {type}/{id}.json inside entities-{world}.
func entityKey(entityType, id string) string { return entityType + "/" + id + ".json" }

// intentKey is _intents/{proposal_id}.json. The identifier is escaped: a
// proposal_id is free text (bootstrap:{world}:{type}/{id}, §4.10), and a slash
// in it would put the intent into a folder of its own.
func intentKey(proposalID string) string {
	return intentsPrefix + url.PathEscape(proposalID) + ".json"
}

func (s *objectStore) ListEntities(ctx context.Context, worldID string) ([]*entity.Entity, error) {
	bucket := objstore.EntitiesBucket(worldID)
	objects, err := s.client.List(ctx, bucket, "")
	if err != nil {
		return nil, fmt.Errorf("state: list %s: %w", bucket, err)
	}
	out := make([]*entity.Entity, 0, len(objects))
	for _, info := range objects {
		if strings.HasPrefix(info.Key, intentsPrefix) {
			continue
		}
		e, err := s.readEntity(ctx, bucket, info.Key)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b *entity.Entity) int {
		if c := cmp.Compare(a.Type, b.Type); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out, nil
}

func (s *objectStore) GetEntity(ctx context.Context, worldID, entityType, id string) (*entity.Entity, error) {
	return s.readEntity(ctx, objstore.EntitiesBucket(worldID), entityKey(entityType, id))
}

func (s *objectStore) readEntity(ctx context.Context, bucket, key string) (*entity.Entity, error) {
	body, err := s.client.Get(ctx, bucket, key)
	if errors.Is(err, objstore.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s/%s", ErrNotFound, bucket, key)
	}
	if err != nil {
		return nil, fmt.Errorf("state: get %s/%s: %w", bucket, key, err)
	}
	var e entity.Entity
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, fmt.Errorf("state: decode %s/%s: %w", bucket, key, err)
	}
	return &e, nil
}

func (s *objectStore) PutEntity(ctx context.Context, worldID string, e *entity.Entity) error {
	if e == nil || e.ID == "" || e.Type == "" {
		return errors.New("state: put entity: no identifier or no type")
	}
	return s.putJSON(ctx, objstore.EntitiesBucket(worldID), entityKey(e.Type, e.ID), e)
}

func (s *objectStore) PutIntent(ctx context.Context, worldID string, in *Intent) error {
	if in == nil || in.ProposalID == "" {
		return errors.New("state: put intent: no proposal_id")
	}
	return s.putJSON(ctx, objstore.EntitiesBucket(worldID), intentKey(in.ProposalID), in)
}

func (s *objectStore) ListIntents(ctx context.Context, worldID string) ([]*Intent, error) {
	bucket := objstore.EntitiesBucket(worldID)
	objects, err := s.client.List(ctx, bucket, intentsPrefix)
	if err != nil {
		return nil, fmt.Errorf("state: list %s/%s: %w", bucket, intentsPrefix, err)
	}
	out := make([]*Intent, 0, len(objects))
	for _, info := range objects {
		body, err := s.client.Get(ctx, bucket, info.Key)
		if err != nil {
			return nil, fmt.Errorf("state: get %s/%s: %w", bucket, info.Key, err)
		}
		var in Intent
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, fmt.Errorf("state: decode %s/%s: %w", bucket, info.Key, err)
		}
		out = append(out, &in)
	}
	return out, nil
}

func (s *objectStore) DeleteIntent(ctx context.Context, worldID, proposalID string) error {
	bucket := objstore.EntitiesBucket(worldID)
	if err := s.client.Delete(ctx, bucket, intentKey(proposalID)); err != nil {
		return fmt.Errorf("state: delete intent %s from %s: %w", proposalID, bucket, err)
	}
	return nil
}

func (s *objectStore) PutSnapshot(ctx context.Context, worldID string, snap *Snapshot, writtenAt time.Time, writer string) (*LatestPointer, error) {
	if snap == nil {
		return nil, errors.New("state: put snapshot: no snapshot")
	}
	bucket := objstore.SnapshotsBucket(worldID)
	body, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("state: encode snapshot %s: %w", snap.Snapshot.Key, err)
	}
	if _, err := s.client.Put(ctx, bucket, snap.Snapshot.Key, body, jsonObject); err != nil {
		return nil, fmt.Errorf("state: put %s/%s: %w", bucket, snap.Snapshot.Key, err)
	}
	// The length is known only to whoever wrote the object: the object cannot
	// state its own without changing it (§4.4).
	size := int64(len(body))
	meta := snap.Snapshot
	meta.SizeBytes = &size
	pointer := &LatestPointer{
		SchemaVersion: SnapshotSchemaVersion,
		Component:     Component,
		Snapshot:      meta,
		WrittenAt:     writtenAt.UTC(),
		Writer:        writer,
	}
	pointer.World.Entity = entity.Ref{ID: worldID, Type: entity.TypeWorld}
	pointerBody, err := json.MarshalIndent(pointer, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("state: encode %s/%s: %w", bucket, PointerKey, err)
	}
	if _, err := s.client.Put(ctx, bucket, PointerKey, pointerBody, jsonObject); err != nil {
		return nil, fmt.Errorf("state: put %s/%s: %w", bucket, PointerKey, err)
	}
	return pointer, nil
}

func (s *objectStore) ReadLatest(ctx context.Context, worldID string) (*LatestPointer, error) {
	bucket := objstore.SnapshotsBucket(worldID)
	body, err := s.client.Get(ctx, bucket, PointerKey)
	if errors.Is(err, objstore.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s/%s", ErrNoSnapshot, bucket, PointerKey)
	}
	if err != nil {
		return nil, fmt.Errorf("state: get %s/%s: %w", bucket, PointerKey, err)
	}
	var pointer LatestPointer
	if err := json.Unmarshal(body, &pointer); err != nil {
		return nil, fmt.Errorf("state: decode %s/%s: %w", bucket, PointerKey, err)
	}
	return &pointer, nil
}

func (s *objectStore) ReadSnapshot(ctx context.Context, worldID, key string) (*Snapshot, error) {
	bucket := objstore.SnapshotsBucket(worldID)
	body, err := s.client.Get(ctx, bucket, key)
	if errors.Is(err, objstore.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s/%s", ErrNotFound, bucket, key)
	}
	if err != nil {
		return nil, fmt.Errorf("state: get %s/%s: %w", bucket, key, err)
	}
	var snap Snapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		return nil, fmt.Errorf("%w: decode %s/%s: %w", ErrUndecodable, bucket, key, err)
	}
	return &snap, nil
}

func (s *objectStore) ListSnapshots(ctx context.Context, worldID string) ([]SnapshotRef, error) {
	bucket := objstore.SnapshotsBucket(worldID)
	objects, err := s.client.List(ctx, bucket, Component+"/")
	if err != nil {
		return nil, fmt.Errorf("state: list %s/%s/: %w", bucket, Component, err)
	}
	out := make([]SnapshotRef, 0, len(objects))
	for _, info := range objects {
		if seq, ok := seqOfKey(info.Key); ok {
			out = append(out, SnapshotRef{Key: info.Key, Seq: seq})
		}
	}
	slices.SortFunc(out, func(a, b SnapshotRef) int {
		if c := cmp.Compare(b.Seq, a.Seq); c != 0 {
			return c
		}
		return cmp.Compare(b.Key, a.Key)
	})
	return out, nil
}

func (s *objectStore) DeleteSnapshot(ctx context.Context, worldID, key string) error {
	bucket := objstore.SnapshotsBucket(worldID)
	if err := s.client.Delete(ctx, bucket, key); err != nil {
		return fmt.Errorf("state: delete %s/%s: %w", bucket, key, err)
	}
	return nil
}

func (s *objectStore) putJSON(ctx context.Context, bucket, key string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("state: encode %s/%s: %w", bucket, key, err)
	}
	if _, err := s.client.Put(ctx, bucket, key, body, jsonObject); err != nil {
		return fmt.Errorf("state: put %s/%s: %w", bucket, key, err)
	}
	return nil
}

// seqOfKey reads the sequence out of state/{YYYYMMDDTHHMMSSZ}-{seq:06d}.json.
// A key of another shape — latest.json above all — is not a snapshot object.
func seqOfKey(key string) (int64, bool) {
	name, ok := strings.CutPrefix(key, Component+"/")
	if !ok {
		return 0, false
	}
	name, ok = strings.CutSuffix(name, ".json")
	if !ok {
		return 0, false
	}
	stamp, digits, ok := strings.Cut(name, "-")
	if !ok || len(stamp) != len("20060102T150405Z") || len(digits) < 6 {
		return 0, false
	}
	if _, err := time.Parse("20060102T150405Z", stamp); err != nil {
		return 0, false
	}
	seq, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || seq < 0 {
		return 0, false
	}
	return seq, true
}
