package state

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Component is the component of the snapshots of State (C-14): the folder
// inside snapshots-{world} and the value of snapshot.created.component.
const Component = "state"

// PointerKey is latest.json inside snapshots-{world} (§4.3).
const PointerKey = Component + "/latest.json"

// SnapshotSchemaVersion is the shape of the snapshot object and of its pointer.
const SnapshotSchemaVersion = 1

// TypeSnapshotCreated is the event State announces a snapshot with (C-14).
const TypeSnapshotCreated = "snapshot.created"

const (
	// SnapshotProposals is how many proposal identifiers of the window a
	// snapshot carries, the newest ones (§4.4, ADR-013 p. 5).
	SnapshotProposals = 1000
	// SnapshotsKept is how many snapshot objects the rotation leaves (K=5,
	// §4.9); latest.json is not one of them.
	SnapshotsKept = 5
)

// The reasons a snapshot is taken (§4.4, §4.9).
const (
	SnapshotInterval     = "interval"
	SnapshotSessionEnded = "session_ended"
	SnapshotShutdown     = "shutdown"
	SnapshotAdmin        = "admin"
	SnapshotBootstrap    = "bootstrap"
)

// SnapshotTimeout bounds one snapshot: the object, the pointer, the rotation
// and snapshot.created. It is 10 s of the 15 s runtime.StopTimeout gives the
// whole Stop of the process, so that the shutdown snapshot leaves the rest
// for the subscription, the workers and the contexts after State (§4.9
// "≤ 10 с"; review #1 of T-057, Mi-1).
const SnapshotTimeout = 10 * time.Second

// ErrNoObjectStore is a snapshot asked of a State that keeps nothing but memory.
var ErrNoObjectStore = errors.New("state: no object store")

// errSnapshotTimeout is the cause of a snapshot cut off by SnapshotTimeout.
var errSnapshotTimeout = errors.New("state: snapshot timeout")

// SnapshotMeta is the block the pointer and the object both carry (§4.4, the
// fixture of seq 0). SizeBytes is set in the pointer only: an object cannot
// state its own length without changing it.
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

// LatestPointer is snapshots-{world}/state/latest.json: the metadata of the last
// snapshot and the key of its object (ADR-011 p. 2, C-14).
type LatestPointer struct {
	SchemaVersion int    `json:"schema_version"`
	Component     string `json:"component"`
	World         struct {
		Entity entity.Ref `json:"entity"`
	} `json:"world"`
	Snapshot  SnapshotMeta `json:"snapshot"`
	WrittenAt time.Time    `json:"written_at"`
	Writer    string       `json:"writer"`
}

// Snapshot is the snapshot object: the metadata, every entity of the world in
// (type, id) order and the newest identifiers of the window of applied
// proposals, oldest first (§4.4, C-14 v1.2 (a)).
type Snapshot struct {
	SchemaVersion    int              `json:"schema_version"`
	Component        string           `json:"component"`
	Snapshot         SnapshotMeta     `json:"snapshot"`
	WorldID          string           `json:"world_id"`
	Entities         []*entity.Entity `json:"entities"`
	AppliedProposals []string         `json:"applied_proposals"`
}

// SnapshotKey is state/{YYYYMMDDTHHMMSSZ}-{seq:06d}.json (§4.3): derived from
// the instant and the sequence, so that a key cannot name a moment other than
// the one it was taken at.
func SnapshotKey(takenAt time.Time, seq int64) string {
	return fmt.Sprintf("%s/%s-%06d.json", Component, takenAt.UTC().Format("20060102T150405Z"), seq)
}

// Writer is who writes the pointers of this process: core/state@host:pid (§4.4).
func Writer() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return contracts.SourceState + "@" + host + ":" + strconv.Itoa(os.Getpid())
}

// Snapshot writes the world into the object store, points latest.json at it,
// removes the snapshots past the newest SnapshotsKept and announces it with
// snapshot.created (§4.9). It is for a caller that holds the world still — the
// worker of the world (Context.Snapshot) or a Stop that has ended it.
//
// A stopped world is not written: after a failure its memory may be behind
// the objects its last proposal wrote, and a snapshot would hide that. A
// snapshot with reason bootstrap is written only for a world without
// latest.json (§4.9): over a pointer, readable or not, it is
// ErrWorldInitialized and nothing is written.
func (a *Applier) Snapshot(ctx context.Context, reason string) (*LatestPointer, error) {
	return a.snapshot(ctx, reason, nil)
}

// snapshot is Snapshot with the event that caused it: the proposal whose facts
// reached the interval, which snapshot.created then derives from; nil for a
// snapshot nobody's event asked for.
//
// The whole snapshot is written on the worker, not on a goroutine of its own
// (ADR-011, addendum 2026-09-14, p. 1): the order of snapshot.created among the
// facts stays the order of the proposals, and a replay publishes the same
// events in the same order (NFR-061).
//
// Unlike the write of an answer, a snapshot sees the cancellation of ctx and is
// bounded by SnapshotTimeout besides: a snapshot cut off leaves the pointer at
// the snapshot before, which is what the order of its writes is for (§4.4), and
// a store that hangs must not hold the world, or a Stop, for longer (review #1
// of T-057, Mi-1).
func (a *Applier) snapshot(ctx context.Context, reason string, cause *eventbus.Event) (*LatestPointer, error) {
	if a.objects == nil {
		return nil, ErrNoObjectStore
	}
	if err := a.Stopped(); err != nil {
		return nil, err
	}
	out, release := a.withinSnapshotTimeout(ctx)
	defer release()
	if reason == SnapshotBootstrap {
		// Before the count starts again: a refused bootstrap is not an attempt
		// at a snapshot, and changes neither the count nor /health.
		if err := a.uninitializedPointer(out); err != nil {
			return nil, err
		}
	}
	// The count starts again whether or not the write succeeds: a failed
	// snapshot is tried again at the next trigger, not on every proposal after
	// it (§9, "Ошибка записи снапшота").
	a.sinceSnapshot = 0
	began := a.clock.Now()
	pointer, err := a.writeSnapshot(out, reason)
	if err != nil {
		if errors.Is(context.Cause(out), errSnapshotTimeout) {
			err = fmt.Errorf("state: snapshot of %s did not finish within %s: %w", a.worldID, SnapshotTimeout, err)
		}
		a.setSnapshotFailure(err)
		a.log.Error("snapshot not written; the world goes on", "reason", reason, "err", err, "handled", true)
		return nil, err
	}
	a.setSnapshotWritten(pointer)
	var size int64
	if pointer.Snapshot.SizeBytes != nil {
		size = *pointer.Snapshot.SizeBytes
	}
	a.log.Info("snapshot written", "seq", pointer.Snapshot.Seq, "key", pointer.Snapshot.Key,
		"reason", reason, "entities", pointer.Snapshot.EntitiesCount, "state_hash", pointer.Snapshot.StateHash,
		"size_bytes", size, "duration_ms", a.clock.Now().Sub(began).Milliseconds())
	a.rotate(out)
	if err := a.pub.Publish(out, a.snapshotCreated(pointer, cause)); err != nil {
		// Not published again (§9): the snapshot is written and its readers
		// read the pointer. /health tells of it until a snapshot.created goes
		// out (snapshot_event_failed; N-4 of review #1 of T-057).
		err = fmt.Errorf("state: snapshot %s is written, snapshot.created is not: %w", pointer.Snapshot.ID, err)
		a.setSnapshotEventFailure(err)
		a.log.Error("snapshot.created did not go out; the snapshot is written", "seq", pointer.Snapshot.Seq,
			"err", err, "handled", true)
		return pointer, err
	}
	a.setSnapshotEventFailure(nil)
	return pointer, nil
}

// uninitializedPointer is nil when the world has no latest.json, the one state
// a snapshot with reason bootstrap is taken in (§4.9, T-475). A pointer that
// is there — readable or not — is ErrWorldInitialized; any other failure to
// read it is returned as it is. The rule is the pointer and not seq 0: a
// snapshot object left without its pointer (a crash between the two PUT, §4.4)
// would otherwise lock world init out for good.
//
// The check runs on the worker of the world, in the order of its snapshots, so
// that two bootstraps asked at once cannot both be written.
func (a *Applier) uninitializedPointer(ctx context.Context) error {
	_, err := a.objects.ReadLatest(ctx, a.worldID)
	switch {
	case errors.Is(err, ErrNoSnapshot):
		return nil
	case err == nil, errors.Is(err, ErrUndecodable):
		refused := fmt.Errorf("%w: %s has its %s", ErrWorldInitialized, a.worldID, PointerKey)
		if err != nil {
			refused = fmt.Errorf("%w (%w)", refused, err)
		}
		a.log.Info("snapshot with reason bootstrap refused: the world is initialized", "err", refused, "handled", true)
		return refused
	default:
		return fmt.Errorf("state: snapshot of %s with reason bootstrap: read the pointer: %w", a.worldID, err)
	}
}

// ErrWorldInitialized is a snapshot with reason bootstrap asked of a world that
// has latest.json (§4.9): nothing is written.
var ErrWorldInitialized = errors.New("state: the world is initialized")

// withinSnapshotTimeout is ctx cancelled as well once SnapshotTimeout has passed
// on the timers of the Applier — the transport timers, real in every mode, so
// that the bound holds in replay too (C-01 v1.4). release ends the wait.
func (a *Applier) withinSnapshotTimeout(ctx context.Context) (context.Context, func()) {
	out, cancel := context.WithCancelCause(ctx)
	timer := a.timers.After(SnapshotTimeout)
	done := make(chan struct{})
	go func() {
		select {
		case <-timer.C():
			cancel(errSnapshotTimeout)
		case <-done:
		}
	}()
	return out, func() {
		close(done)
		timer.Stop()
		cancel(nil)
	}
}

// writeSnapshot takes the world, the window and the cursor, which the caller
// holds still, and writes the object and the pointer.
func (a *Applier) writeSnapshot(ctx context.Context, reason string) (*LatestPointer, error) {
	seq, err := a.nextSeq(ctx)
	if err != nil {
		return nil, err
	}
	entities := a.store.List(a.worldID)
	laws := lawsVersionOf(entities, a.worldID)
	if laws == "" {
		return nil, fmt.Errorf("state: snapshot of %s: the world entity holds no laws_version", a.worldID)
	}
	takenAt := a.clock.Now().UTC()
	meta := SnapshotMeta{
		ID:            fmt.Sprintf("%s:%s:%06d", Component, a.worldID, seq),
		Seq:           seq,
		Key:           SnapshotKey(takenAt, seq),
		TakenAt:       takenAt,
		Cursor:        map[string]int64{eventbus.TopicSystemEvents: a.cursor.Load()},
		LawsVersion:   laws,
		RulesVersion:  a.rulesVersion,
		StateHash:     entity.StateHash(entities),
		EntitiesCount: len(entities),
		Reason:        reason,
	}
	snap := &Snapshot{
		SchemaVersion:    SnapshotSchemaVersion,
		Component:        Component,
		Snapshot:         meta,
		WorldID:          a.worldID,
		Entities:         entities,
		AppliedProposals: newest(a.applied.IDs(), SnapshotProposals),
	}
	return a.objects.PutSnapshot(ctx, a.worldID, snap, a.clock.Now(), a.writer)
}

// nextSeq is one past the highest snapshot object of the world. It asks the
// store rather than the pointer: an object written by a process that died
// before its pointer still holds its number.
func (a *Applier) nextSeq(ctx context.Context) (int64, error) {
	refs, err := a.objects.ListSnapshots(ctx, a.worldID)
	if err != nil {
		return 0, err
	}
	if len(refs) == 0 {
		return 0, nil
	}
	return refs[0].Seq + 1, nil
}

// rotate removes every snapshot object past the newest SnapshotsKept. A failure
// leaves an extra object behind and nothing else; the next rotation takes it.
func (a *Applier) rotate(ctx context.Context) {
	refs, err := a.objects.ListSnapshots(ctx, a.worldID)
	if err != nil {
		a.log.Error("rotation of the snapshots skipped", "err", err, "handled", true)
		return
	}
	for _, ref := range refs[min(len(refs), SnapshotsKept):] {
		if err := a.objects.DeleteSnapshot(ctx, a.worldID, ref.Key); err != nil {
			a.log.Error("an old snapshot was not removed", "key", ref.Key, "err", err, "handled", true)
		}
	}
}

// snapshotCreated is the event of a written snapshot: the pointer without
// rules_version, entities_count and reason, which the closed schema does not
// know (§4.4). The world is in the envelope only (C-14 v1.3).
func (a *Applier) snapshotCreated(pointer *LatestPointer, cause *eventbus.Event) eventbus.Event {
	s := pointer.Snapshot
	block := map[string]any{
		"id":           s.ID,
		"seq":          s.Seq,
		"taken_at":     s.TakenAt.Format(time.RFC3339Nano),
		"cursor":       s.Cursor,
		"laws_version": s.LawsVersion,
		"state_hash":   s.StateHash,
		"key":          s.Key,
	}
	if s.SizeBytes != nil {
		block["size_bytes"] = *s.SizeBytes
	}
	payload := map[string]any{"component": Component, "snapshot": block}
	if cause != nil && cause.ID != "" {
		// One snapshot per proposal at most, so the proposal and the sequence
		// tell it apart, and a replay derives the same id (ADR-027).
		//
		// The snapshot is an event of the world, whoever's proposal completed
		// the count: it carries no scope — inherited, it would land in the
		// window of that player's scope at the router of the swarm (system
		// architect on T-057, p. 1) — and the actor, locale and path of a root
		// event, so that snapshot.created has one envelope whatever its
		// trigger (review #1 of T-057, N-1). What stays derived is what replay
		// needs: the id, the timestamp and the trace of the cause.
		ev := eventbus.Derive(*cause, TypeSnapshotCreated, contracts.SourceState, payload,
			eventbus.WithCauseID(Component, strconv.FormatInt(s.Seq, 10)), eventbus.WithScope(nil))
		ev.Meta.ActorKind = eventbus.ActorSystem
		ev.Meta.Locale = eventbus.DefaultLocale
		ev.Meta.GMPath = eventbus.GMPathAgent
		ev.Meta.Agent = nil
		return ev
	}
	return eventbus.NewRoot(TypeSnapshotCreated, contracts.SourceState, a.worldID, nil, eventbus.ActorSystem, payload)
}

// lawsVersionOf reads the laws the world runs under off its world entity
// (data-model.md §3.1).
func lawsVersionOf(entities []*entity.Entity, worldID string) string {
	for _, e := range entities {
		if e.ID == worldID {
			version, _ := e.LawsVersion()
			return version
		}
	}
	return ""
}

// newest is the last n identifiers of a window listed oldest first.
func newest(ids []string, n int) []string {
	if len(ids) > n {
		ids = ids[len(ids)-n:]
	}
	return append([]string{}, ids...)
}

// SnapshotHealth is what /health tells of the snapshots of a world: the last
// one written — or the one recovery started from — and the failure of the last
// attempt since, if any.
func (a *Applier) SnapshotHealth() (*LatestPointer, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastSnapshot, a.snapshotErr
}

// SnapshotEventFailure is the failure of snapshot.created of the last snapshot
// written, or nil once one went out.
func (a *Applier) SnapshotEventFailure() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.snapshotEventErr
}

func (a *Applier) setSnapshotEventFailure(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.snapshotEventErr = err
}

func (a *Applier) setSnapshotWritten(pointer *LatestPointer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastSnapshot, a.snapshotErr, a.snapshotSinceRecovery = pointer, nil, true
}

func (a *Applier) setSnapshotFailure(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.snapshotErr = err
}
