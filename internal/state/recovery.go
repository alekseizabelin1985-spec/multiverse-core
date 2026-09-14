package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
)

// TypeReplayCompleted is the event State ends its recovery with (C-14).
const TypeReplayCompleted = "analytics.replay.completed"

// ReplayModeRecovery is the mode of analytics.replay.completed State publishes.
const ReplayModeRecovery = "recovery"

// ErrSnapshotCorrupted is why a world is not served when latest.json names a
// snapshot and neither it nor any of the snapshots kept before it reads whole
// (state-and-mechanics.md §4.8, §9).
var ErrSnapshotCorrupted = errors.New("state: snapshot_corrupted")

// ErrStateDivergence is why a world is not served when what the journal and
// the object store say of it does not fit together: a fact whose version is
// not the next one, two facts under one version, an element past the end of
// its list, an object ahead of its facts by more than one version (§4.8, §9).
var ErrStateDivergence = errors.New("state: state_divergence")

// Recovery is what the recovery of one world found and did (§4.8).
type Recovery struct {
	// SnapshotID and StateHashBefore are those of the snapshot the world was
	// rebuilt from; empty without one.
	SnapshotID      string
	StateHashBefore string
	// StateHashAfter is the hash of the world when it takes proposals.
	StateHashAfter string
	// EventsReplayed is the number of facts of the journal applied over the
	// snapshot; a fact the snapshot already held is not counted.
	EventsReplayed int
	// Accepted is the number of entity objects taken into the world ahead of
	// their facts (a write whose fact was not confirmed), RolledForward the
	// number of entities an intent was rolled forward on.
	Accepted      int
	RolledForward int
	// Cursor is the offset of system_events proposals are read from.
	Cursor int64
	// Uninitialized: neither latest.json nor an entity object (§18).
	Uninitialized bool
	// NoSnapshot: entity objects without latest.json; the world is rebuilt
	// from them.
	NoSnapshot bool
	// LogGap: the journal no longer holds the facts after the cursor of the
	// snapshot; the world is rebuilt from the entity objects.
	LogGap bool
	// FellBack is the key of the snapshot recovery used when the one
	// latest.json names did not read whole.
	FellBack string
	// Failure is what stopped the world: ErrSnapshotCorrupted or
	// ErrStateDivergence; nil for a world that is served.
	Failure error
	// Duration is how long the recovery took on the clock of the Applier.
	Duration time.Duration
}

// Identical reports whether the world is the snapshot it was rebuilt from
// (§4.8, "identical"). Without a snapshot there is nothing to compare, and the
// second result is false.
func (r Recovery) Identical() (identical, known bool) {
	if r.StateHashBefore == "" || r.StateHashAfter == "" {
		return false, false
	}
	return r.StateHashBefore == r.StateHashAfter, true
}

// Recover rebuilds the world from the object store and the journal before the
// world takes a proposal (state-and-mechanics.md §4.8, ADR-011 p. 3):
//
//  1. latest.json, then the snapshot object it names, checked against its
//     state_hash; a snapshot that does not read whole gives way to the one
//     kept before it, up to SnapshotsKept of them;
//  2. the facts of the world on system_events from the cursor of the snapshot
//     to end, applied by the rule of catching up (C-02 v1.6), and the window
//     of proposal identifiers restored from the snapshot and completed by the
//     proposal_id of those facts (C-14 v1.2 (a));
//  3. the entity objects compared with the world: an object one version ahead
//     is a write whose fact was not confirmed, and is taken into the world as
//     it is, without its fact — the fact goes out only when the proposal comes
//     again (§18, resendUnpublished);
//  4. every intent rolled forward — the entities below their to_version
//     written from it — and removed.
//
// end is the end of system_events the process took once before any world
// recovered; proposals are read from it.
//
// Nothing of the world is published: the facts a restart owes are sent by the
// repeat of their proposal, so that a fact goes out once whatever the path
// (§18). What Recover publishes is analytics.replay.completed.
//
// A snapshot that does not read whole and a world that does not fit together
// stop the world (Recovery.Failure, /health fail); the error is for a store or
// a journal that could not be read or written, which fails the start of the
// context rather than serve a world half rebuilt.
func (a *Applier) Recover(ctx context.Context, journal eventbus.Journal, end int64) (Recovery, error) {
	if a.objects == nil {
		return Recovery{}, ErrNoObjectStore
	}
	began := a.clock.Now()
	rec, err := a.recover(ctx, journal, end)
	rec.Duration = max(a.clock.Now().Sub(began), 0)
	if err != nil {
		return rec, fmt.Errorf("state: recovery of %s: %w", a.worldID, err)
	}
	if rec.Failure == nil {
		a.cursor.Store(end)
		rec.Cursor = end
		rec.StateHashAfter = entity.StateHash(a.store.List(a.worldID))
	}
	a.mu.Lock()
	a.recovery = rec
	a.mu.Unlock()
	a.logRecovery(rec)
	if rec.Failure == nil {
		a.announceRecovery(ctx, rec, end)
	}
	return rec, nil
}

func (a *Applier) recover(ctx context.Context, journal eventbus.Journal, end int64) (Recovery, error) {
	var rec Recovery
	pointer, err := a.objects.ReadLatest(ctx, a.worldID)
	switch {
	case errors.Is(err, ErrNoSnapshot), errors.Is(err, objstore.ErrNoBucket):
		return a.recoverWithoutSnapshot(ctx, rec)
	case err != nil:
		return rec, err
	}
	snap, fellBack, err := a.intactSnapshot(ctx, pointer)
	if errors.Is(err, ErrSnapshotCorrupted) {
		rec.Failure = a.stopRecovery(err, "every snapshot kept is corrupted; the world is not served",
			"restore a snapshot object or the entity objects of the world and restart the process")
		return rec, nil
	}
	if err != nil {
		return rec, err
	}
	rec.SnapshotID, rec.StateHashBefore, rec.FellBack = snap.Snapshot.ID, snap.Snapshot.StateHash, fellBack
	if err := a.store.Replace(a.worldID, snap.Entities...); err != nil {
		return rec, err
	}
	a.applied.Restore(snap.AppliedProposals)
	a.setRecoveredSnapshot(pointer, snap, fellBack)

	cursor := snap.Snapshot.Cursor[eventbus.TopicSystemEvents]
	facts, gap, err := a.readFacts(ctx, journal, cursor, end)
	if err != nil {
		return rec, err
	}
	if gap {
		rec.LogGap = true
		if err := a.rebuildFromObjects(ctx, snap, facts); err != nil {
			return rec, err
		}
		return a.rollForward(ctx, rec)
	}
	for _, fact := range facts {
		applied, err := a.catchUp(fact)
		if err != nil {
			rec.Failure = a.stopRecovery(err, "a fact of the journal does not fit the world; the world is not served",
				"compare the journal with the snapshot (mvctl report --audit) before the world is served again")
			return rec, nil
		}
		if applied {
			rec.EventsReplayed++
		}
	}
	accepted, err := a.reconcile(ctx)
	if err != nil {
		if errors.Is(err, ErrStateDivergence) {
			rec.Failure = a.stopRecovery(err, "the entity objects do not fit the world; the world is not served",
				"compare the entity objects with the journal before the world is served again")
			return rec, nil
		}
		return rec, err
	}
	rec.Accepted = accepted
	return a.rollForward(ctx, rec)
}

// recoverWithoutSnapshot is a world without latest.json: rebuilt from its
// entity objects when there are any (reason no_snapshot, /health degraded),
// uninitialized otherwise (§18).
func (a *Applier) recoverWithoutSnapshot(ctx context.Context, rec Recovery) (Recovery, error) {
	objects, err := a.entityObjects(ctx)
	if err != nil {
		return rec, err
	}
	if len(objects) == 0 {
		rec.Uninitialized = true
		a.uninitialized.Store(true)
		return rec, a.store.Replace(a.worldID)
	}
	rec.NoSnapshot = true
	if err := a.store.Replace(a.worldID, objects...); err != nil {
		return rec, err
	}
	return a.rollForward(ctx, rec)
}

// entityObjects lists the entity objects of the world; a world whose bucket
// was never created has none.
func (a *Applier) entityObjects(ctx context.Context) ([]*entity.Entity, error) {
	objects, err := a.objects.ListEntities(ctx, a.worldID)
	if errors.Is(err, objstore.ErrNoBucket) {
		return nil, nil
	}
	return objects, err
}

// intactSnapshot is the snapshot latest.json names when it reads whole, or the
// newest of the kept snapshots before it that does, with its key as the second
// result.
func (a *Applier) intactSnapshot(ctx context.Context, pointer *LatestPointer) (*Snapshot, string, error) {
	keys := []string{pointer.Snapshot.Key}
	refs, err := a.objects.ListSnapshots(ctx, a.worldID)
	if err != nil {
		return nil, "", err
	}
	for _, ref := range refs {
		// Only the snapshots before the one latest.json names: an object newer
		// than the pointer was written by a process that died before its
		// pointer, and §4.8 rebuilds from the snapshot before (review #1 of
		// T-059, N-2).
		if ref.Seq < pointer.Snapshot.Seq && len(keys) < SnapshotsKept {
			keys = append(keys, ref.Key)
		}
	}
	var refused []error
	for i, key := range keys {
		snap, err := a.objects.ReadSnapshot(ctx, a.worldID, key)
		if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrUndecodable) {
			return nil, "", err
		}
		if err == nil {
			err = a.checkSnapshot(snap, key)
		}
		if err != nil {
			a.log.Error("a snapshot does not read whole; the one kept before it is tried",
				"key", key, "err", err, "handled", true)
			refused = append(refused, err)
			continue
		}
		if i == 0 {
			return snap, "", nil
		}
		a.log.Warn("the world is rebuilt from a snapshot older than latest.json names",
			"key", key, "pointer_key", pointer.Snapshot.Key, "handled", true)
		return snap, key, nil
	}
	return nil, "", fmt.Errorf("%w: %w", ErrSnapshotCorrupted, errors.Join(refused...))
}

// checkSnapshot holds a snapshot object to what its key and its metadata say:
// the component, the world, the key derived from taken_at and seq (§4.4) and
// the state_hash of its entities.
func (a *Applier) checkSnapshot(snap *Snapshot, key string) error {
	meta := snap.Snapshot
	switch {
	case snap.SchemaVersion != SnapshotSchemaVersion || snap.Component != Component:
		return fmt.Errorf("snapshot %s: schema_version %d of component %q", key, snap.SchemaVersion, snap.Component)
	case snap.WorldID != a.worldID:
		return fmt.Errorf("snapshot %s: of world %q", key, snap.WorldID)
	case meta.Key != key || SnapshotKey(meta.TakenAt, meta.Seq) != key:
		return fmt.Errorf("snapshot %s: names the key %q, taken at %s as seq %d", key, meta.Key, meta.TakenAt, meta.Seq)
	}
	if _, ok := meta.Cursor[eventbus.TopicSystemEvents]; !ok {
		return fmt.Errorf("snapshot %s: no cursor of %s", key, eventbus.TopicSystemEvents)
	}
	for i, e := range snap.Entities {
		if e == nil || e.ID == "" {
			return fmt.Errorf("snapshot %s: entity %d has no identifier", key, i)
		}
	}
	if hash := entity.StateHash(snap.Entities); hash != meta.StateHash {
		return fmt.Errorf("snapshot %s: its entities hash to %s, it says %s", key, hash, meta.StateHash)
	}
	return nil
}

// setRecoveredSnapshot makes the snapshot the world was rebuilt from the last
// snapshot /health tells of.
func (a *Applier) setRecoveredSnapshot(pointer *LatestPointer, snap *Snapshot, fellBack string) {
	recovered := pointer
	if fellBack != "" {
		recovered = &LatestPointer{SchemaVersion: SnapshotSchemaVersion, Component: Component, Snapshot: snap.Snapshot}
		recovered.World.Entity = entity.Ref{ID: a.worldID, Type: entity.TypeWorld}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastSnapshot = recovered
}

// readFacts reads [from, end) of system_events and keeps the facts of the
// world. The events are collected and applied after the read (C-01 v1.9, "Чтение
// журнала"): an error returned from inside the read would go into the retries
// and the dead letters of the delivery.
//
// The second result is a gap: the journal does not start at from — its first
// offsets are gone past the retention, or it is a younger journal than the
// snapshot (a memory bus after a restart) — so the facts after the cursor
// cannot all be read. The facts of the rest of the journal are returned with
// it: they are not applied, they mark the objects they announced
// (rebuildFromObjects).
func (a *Applier) readFacts(ctx context.Context, journal eventbus.Journal, from, end int64) ([]eventbus.Event, bool, error) {
	if from > end {
		return nil, true, nil
	}
	if from == end {
		return nil, false, nil
	}
	if journal == nil {
		return nil, false, errors.New("no journal to catch up from")
	}
	var facts []eventbus.Event
	first := int64(-1)
	next, err := journal.ReadRange(ctx, eventbus.TopicSystemEvents, from, end, func(hctx context.Context, ev eventbus.Event) error {
		if pos, ok := eventbus.PositionFromContext(hctx); ok && first < 0 {
			first = pos.Offset
		}
		if (ev.Type == TypeCreated || ev.Type == TypeUpdated) && worldOf(ev) == a.worldID {
			facts = append(facts, ev)
		}
		return nil
	})
	switch {
	case err != nil:
		return nil, false, fmt.Errorf("read %s [%d, %d): %w", eventbus.TopicSystemEvents, from, end, err)
	case ctx.Err() != nil:
		return nil, false, fmt.Errorf("read %s stopped at %d of %d: %w", eventbus.TopicSystemEvents, next, end, ctx.Err())
	case first != from:
		return facts, true, nil
	case next < end:
		return nil, false, fmt.Errorf("read %s stopped at %d of %d", eventbus.TopicSystemEvents, next, end)
	}
	return facts, false, nil
}

// rebuildFromObjects is the world after a gap of the journal (§4.8, "Разрыв
// журнала"; §19 p. 1): the entity objects, which are never behind the snapshot.
//
//   - An object at the version of the snapshot under the same proposal keeps the
//     commit record of the snapshot, which knows the fact that announced it.
//   - The facts the rest of the journal still holds are not applied — the
//     objects are the truth — but mark the object they announced: an object
//     with the id, the version and the proposal_id of a fact learns the id of
//     that fact, so that a repeat of its proposal publishes nothing.
//   - An object whose fact is not found does not know whether it went out, and
//     a repeat of its proposal publishes it under the id it was first derived
//     with, possibly a second time; consumers drop it by id. The fact could only
//     have gone into the part of the journal that is lost.
func (a *Applier) rebuildFromObjects(ctx context.Context, snap *Snapshot, facts []eventbus.Event) error {
	objects, err := a.entityObjects(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]*entity.Entity, len(snap.Entities))
	for _, e := range snap.Entities {
		known[e.ID] = e
	}
	announced := make(map[announcement]eventbus.Event, len(facts))
	for _, fact := range facts {
		announced[factKey(fact)] = fact
	}
	world := make([]*entity.Entity, 0, len(objects))
	for _, object := range objects {
		if held, ok := known[object.ID]; ok && held.Version == object.Version && sameProposal(held, object) {
			object = entity.Clone(held)
		} else if fact, ok := announced[objectKey(object)]; ok {
			object.SetFactEventID(fact.ID)
		}
		world = append(world, object)
	}
	a.log.Warn("the journal no longer holds the facts after the snapshot; the world is rebuilt from the entity objects",
		"code", "log_gap", "snapshot_id", snap.Snapshot.ID, "entities", len(world), "handled", true)
	return a.store.Replace(a.worldID, world...)
}

// factKey and objectKey tell which object a fact announced: the entity, its
// version and the proposal (§19 p. 1).
type announcement struct {
	entity   string
	version  int64
	proposal string
}

func factKey(ev eventbus.Event) announcement {
	id, _ := ev.Path().GetString("entity.entity.id")
	version, _ := ev.Path().GetInt("version")
	proposal, _ := ev.Path().GetString("proposal_id")
	return announcement{entity: id, version: int64(version), proposal: proposal}
}

func objectKey(e *entity.Entity) announcement {
	return announcement{entity: e.ID, version: e.Version, proposal: proposalOf(e)}
}

// --- the facts of the journal (§4.8, the rule of catching up) ---

// wireFact is entity.created and entity.updated as the journal holds them.
type wireFact struct {
	Entity     eventbus.Entity `json:"entity"`
	Version    int64           `json:"version"`
	Attributes map[string]any  `json:"attributes"`
	Changed    []entity.Change `json:"changed"`
	Cause      string          `json:"cause"`
	ProposalID string          `json:"proposal_id"`
	AppliedAt  time.Time       `json:"applied_at"`
}

// catchUp applies one fact of the journal to the world and reports whether it
// changed anything. A fact the world already holds — a duplicate, or a fact
// the snapshot was taken after — is passed over; a fact that does not fit is
// ErrStateDivergence.
func (a *Applier) catchUp(ev eventbus.Event) (bool, error) {
	var fact wireFact
	raw, err := json.Marshal(ev.Payload)
	if err == nil {
		err = json.Unmarshal(raw, &fact)
	}
	ref := entity.RefFrom(fact.Entity.Entity)
	if err != nil || ref.ID == "" {
		return false, fmt.Errorf("%w: %s %s does not read: %v", ErrStateDivergence, ev.Type, ev.ID, err)
	}
	if fact.ProposalID != "" {
		a.applied.Add(fact.ProposalID)
	}
	current, exists := a.store.Get(a.worldID, ref.ID)
	if ev.Type == TypeCreated {
		return a.catchUpCreated(ev, fact, ref, current, exists)
	}
	if !exists {
		return false, fmt.Errorf("%w: %s %s of %s, which the world does not hold", ErrStateDivergence, ev.Type, ev.ID, ref.ID)
	}
	return a.catchUpUpdated(ev, fact, current)
}

func (a *Applier) catchUpCreated(ev eventbus.Event, fact wireFact, ref entity.Ref, current *entity.Entity, exists bool) (bool, error) {
	if exists {
		if announcer, ok := announcerOf(current, fact.Version); ok && announcer != ev.ID {
			return false, fmt.Errorf("%w: %s v%d is announced by %s and by %s", ErrStateDivergence, ref.ID, fact.Version, announcer, ev.ID)
		}
		return false, nil
	}
	if fact.Version != 1 {
		return false, fmt.Errorf("%w: %s %s creates %s at v%d", ErrStateDivergence, ev.Type, ev.ID, ref.ID, fact.Version)
	}
	created := entity.New(ref, a.worldID, fact.Entity.Name, fact.Attributes, ev.Timestamp)
	created.Commit(created.Attributes, nil, entity.LastChange{
		ProposalID: fact.ProposalID, ProposalEventID: ev.Meta.CausationID, AppliedAt: ev.Timestamp, BatchSize: 1,
	})
	created.SetFactEventID(ev.ID)
	return true, a.store.Put(a.worldID, created)
}

func (a *Applier) catchUpUpdated(ev eventbus.Event, fact wireFact, current *entity.Entity) (bool, error) {
	id := current.ID
	if len(fact.Changed) == 0 {
		// A turn that changed nothing announces no version (C-02): it is held
		// to its id alone.
		switch {
		case fact.Version < current.Version || announced(current, ev.ID):
			return false, nil
		case fact.Version > current.Version:
			return false, fmt.Errorf("%w: %s %s leaves %s at v%d, the world holds v%d", ErrStateDivergence, ev.Type, ev.ID, id, fact.Version, current.Version)
		}
	} else {
		switch {
		case fact.Version <= current.Version:
			if announcer, ok := announcerOf(current, fact.Version); ok && announcer != ev.ID {
				return false, fmt.Errorf("%w: %s v%d is announced by %s and by %s", ErrStateDivergence, id, fact.Version, announcer, ev.ID)
			}
			return false, nil
		case fact.Version != current.Version+1:
			return false, fmt.Errorf("%w: %s %s announces %s v%d over v%d", ErrStateDivergence, ev.Type, ev.ID, id, fact.Version, current.Version)
		}
	}
	attrs, err := CatchUpChanged(current.Attributes, fact.Changed)
	if err != nil {
		return false, fmt.Errorf("%w: %s %s of %s: %w", ErrStateDivergence, ev.Type, ev.ID, id, err)
	}
	appliedAt := fact.AppliedAt
	if appliedAt.IsZero() {
		appliedAt = ev.Timestamp
	}
	current.Commit(attrs, fact.Changed, entity.LastChange{
		ProposalID: fact.ProposalID, ProposalEventID: ev.Meta.CausationID, Cause: fact.Cause, AppliedAt: appliedAt,
	})
	if current.Version != fact.Version {
		return false, fmt.Errorf("%w: %s %s announces %s v%d, catching up gave v%d", ErrStateDivergence, ev.Type, ev.ID, id, fact.Version, current.Version)
	}
	current.SetFactEventID(ev.ID)
	return true, a.store.Put(a.worldID, current)
}

// announcerOf is the fact that announced version v of an entity, as its history
// remembers it: the first entry at that version.
func announcerOf(e *entity.Entity, v int64) (string, bool) {
	for _, h := range e.History {
		if h.Version == v {
			return h.EventID, h.EventID != ""
		}
	}
	return "", false
}

// announced reports whether a fact is in the history of an entity.
func announced(e *entity.Entity, eventID string) bool {
	if e.LastEventID == eventID {
		return true
	}
	for _, h := range e.History {
		if h.EventID == eventID {
			return true
		}
	}
	return false
}

// --- the entity objects (§4.8, §18) ---

// reconcile compares every entity object with the world caught up from the
// journal and returns how many objects it took in ahead of their facts.
//
//   - one version ahead, or an entity the world does not hold yet at v1: a
//     write whose fact was not confirmed. The object is taken in as it is,
//     with its empty fact_event_id, and its fact is not published here (§18);
//   - the same version under the same proposal: the fact of that version is in
//     the journal, and the world keeps the id of that fact — an empty
//     fact_event_id would have a repeat of the proposal publish it again (§18,
//     "Догон атомарного пакета") — and takes from the object what the fact does
//     not carry: atomic and batch_size;
//   - the same version under another proposal: a turn that changed nothing and
//     whose fact was not confirmed; the object is taken in;
//   - anything else — further ahead, behind, a different state at the same
//     version, an entity of the world without an object — is ErrStateDivergence.
func (a *Applier) reconcile(ctx context.Context) (int, error) {
	objects, err := a.entityObjects(ctx)
	if err != nil {
		return 0, err
	}
	accepted := 0
	seen := make(map[string]bool, len(objects))
	for _, object := range objects {
		seen[object.ID] = true
		held, ok := a.store.Get(a.worldID, object.ID)
		switch {
		case !ok && object.Version == 1, ok && object.Version == held.Version+1:
			if err := a.store.Put(a.worldID, object); err != nil {
				return accepted, err
			}
			accepted++
			a.log.Info("an entity object ahead of its fact is taken into the world; its fact is sent when the proposal comes again",
				"entity_id", object.ID, "version", object.Version, "proposal_id", proposalOf(object), "handled", true)
		case !ok:
			return accepted, fmt.Errorf("%w: the object of %s is at v%d, the world does not hold it", ErrStateDivergence, object.ID, object.Version)
		case object.Version != held.Version:
			return accepted, fmt.Errorf("%w: the object of %s is at v%d, the world at v%d", ErrStateDivergence, object.ID, object.Version, held.Version)
		case entity.StateHash([]*entity.Entity{object}) != entity.StateHash([]*entity.Entity{held}):
			return accepted, fmt.Errorf("%w: the object of %s at v%d differs from the world at the same version", ErrStateDivergence, object.ID, object.Version)
		case object.LastChange == nil:
		case sameProposal(held, object):
			record := *object.LastChange
			record.FactEventID = held.LastChange.FactEventID
			held.LastChange = &record
			if err := a.store.Put(a.worldID, held); err != nil {
				return accepted, err
			}
		default:
			if err := a.store.Put(a.worldID, object); err != nil {
				return accepted, err
			}
			accepted++
		}
	}
	for _, held := range a.store.List(a.worldID) {
		if !seen[held.ID] {
			return accepted, fmt.Errorf("%w: %s v%d of the world has no object", ErrStateDivergence, held.ID, held.Version)
		}
	}
	return accepted, nil
}

// sameProposal reports whether two copies of an entity were last changed by
// the same proposal.
func sameProposal(x, y *entity.Entity) bool {
	return x.LastChange != nil && y.LastChange != nil && x.LastChange.ProposalID == y.LastChange.ProposalID
}

func proposalOf(e *entity.Entity) string {
	if e.LastChange == nil {
		return ""
	}
	return e.LastChange.ProposalID
}

// --- the intents (ADR-013 p. 4, §4.8) ---

// rollForward writes every intent of the world through: an entity still at the
// from_version of its change is written at its to_version from the intent, an
// entity already there is left, and the intent is removed. The facts of the
// package are not published: the package was cut before its answer, and the
// repeat of its proposal sends them (§18). An intent that could not be removed
// stays counted in /health (pending_intents) and does not stop the world: a
// repeat of its proposal sees its package finished.
//
// It runs before the world takes a proposal: a repeat that came first would
// find the package unfinished and stop the world (refuseUnfinishedPackage).
func (a *Applier) rollForward(ctx context.Context, rec Recovery) (Recovery, error) {
	intents, err := a.objects.ListIntents(ctx, a.worldID)
	if errors.Is(err, objstore.ErrNoBucket) {
		intents, err = nil, nil
	}
	if err != nil {
		return rec, err
	}
	pending := int64(0)
	for _, in := range intents {
		rolled, err := a.rollForwardIntent(ctx, in)
		rec.RolledForward += rolled
		if errors.Is(err, ErrStateDivergence) {
			rec.Failure = a.stopRecovery(err, "an intent does not fit the world; the world is not served",
				"compare the intent with the entity objects before the world is served again")
			return rec, nil
		}
		if err != nil {
			return rec, err
		}
		if err := a.withAttempts(ctx, in.ProposalID, "delete the intent", func(out context.Context) error {
			return a.objects.DeleteIntent(out, a.worldID, in.ProposalID)
		}); err != nil {
			pending++
			a.log.Error("the intent of a package rolled forward was not removed; the world goes on",
				"proposal_id", in.ProposalID, "err", err, "handled", true)
		}
	}
	a.pendingIntents.Store(pending)
	return rec, nil
}

// rollForwardIntent writes the entities of one intent still behind it.
// Every change of the intent is checked, and committed on a copy, before
// anything is written (review #1 of T-059, Ma-1; review #2, N-5):
//
//   - an entity the package is written on (writtenBy) is left;
//   - an entity at from_version is committed from the intent on a copy, and
//     the commit has to reach to_version;
//   - anything else — below from_version, not in the world, or changes that
//     do not reach to_version — is a divergence, and no entity of the intent is
//     written.
func (a *Applier) rollForwardIntent(ctx context.Context, in *Intent) (int, error) {
	var behind []*entity.Entity
	for _, change := range in.Changes {
		held, ok := a.store.Get(a.worldID, change.Ref.ID)
		switch {
		case !ok:
			return 0, fmt.Errorf("%w: the intent of %s names %s, which the world does not hold", ErrStateDivergence, in.ProposalID, change.Ref.ID)
		case writtenBy(held, in, change):
			continue
		case held.Version != change.FromVersion:
			return 0, fmt.Errorf("%w: the intent of %s takes %s from v%d to v%d, the world holds v%d",
				ErrStateDivergence, in.ProposalID, change.Ref.ID, change.FromVersion, change.ToVersion, held.Version)
		}
		held.Commit(change.AttributesAfter, change.Changed, entity.LastChange{
			ProposalID: in.ProposalID, ProposalEventID: in.ProposalEventID, Cause: in.Cause,
			AppliedAt: in.AppliedAt, Atomic: true, BatchSize: len(in.Changes),
		})
		if held.Version != change.ToVersion {
			return 0, fmt.Errorf("%w: the intent of %s takes %s to v%d, its changes give v%d",
				ErrStateDivergence, in.ProposalID, change.Ref.ID, change.ToVersion, held.Version)
		}
		behind = append(behind, held)
	}
	rolled := 0
	for _, held := range behind {
		if err := a.withAttempts(ctx, in.ProposalID, "roll forward "+held.ID, func(out context.Context) error {
			return a.objects.PutEntity(out, a.worldID, held)
		}); err != nil {
			return rolled, err
		}
		if err := a.store.Put(a.worldID, held); err != nil {
			return rolled, err
		}
		rolled++
		a.log.Info("an entity of a cut package is rolled forward from its intent", "proposal_id", in.ProposalID,
			"entity_id", held.ID, "version", held.Version, "handled", true)
	}
	return rolled, nil
}

// writtenBy reports whether the package of an intent is written on an entity.
//
//   - A change that moves the version: the entity is at to_version or past it.
//     Past it when the intent outlived a package written whole — its removal
//     failed, which does not stop the world (§9) — and the world went on. It
//     cannot have gone on while the package was unwritten: the world stopped
//     with persist_failed then, and the roll forward comes before any proposal.
//   - A change without a change (from_version = to_version) does not tell by
//     the version (review #2 of T-059, Mi-4): the entity is written when it has
//     moved past, or when its commit record or its history names the proposal.
//     The history keeps 50 entries, and 50 later turns on the entity can only
//     come after the package was written whole.
func writtenBy(e *entity.Entity, in *Intent, change IntentChange) bool {
	if change.FromVersion != change.ToVersion {
		return e.Version >= change.ToVersion
	}
	if e.Version > change.ToVersion || proposalOf(e) == in.ProposalID {
		return e.Version >= change.ToVersion
	}
	for _, h := range e.History {
		if h.ProposalID == in.ProposalID {
			return e.Version >= change.ToVersion
		}
	}
	return false
}

// --- the end of a recovery ---

// stopRecovery stops the world recovery could not rebuild.
func (a *Applier) stopRecovery(cause error, msg, remedy string) error {
	failure := fmt.Errorf("%w: %w", ErrWorldStopped, cause)
	a.halt(failure)
	reason := "state_divergence"
	if errors.Is(cause, ErrSnapshotCorrupted) {
		reason = "snapshot_corrupted"
	}
	a.log.Error(msg, "reason", reason, "err", cause, "handled", false, "remedy", remedy)
	return failure
}

func (a *Applier) logRecovery(rec Recovery) {
	identical, known := rec.Identical()
	a.log.Info("recovery", "snapshot_id", rec.SnapshotID, "events_replayed", rec.EventsReplayed,
		"accepted", rec.Accepted, "rolled_forward", rec.RolledForward, "duration_ms", rec.Duration.Milliseconds(),
		"identical", identical && known, "uninitialized", rec.Uninitialized, "no_snapshot", rec.NoSnapshot,
		"log_gap", rec.LogGap, "failed", rec.Failure != nil)
}

// announceRecovery publishes analytics.replay.completed in mode recovery, the
// signal the swarm waits for (C-14 v1.4). Nothing an LLM or the dice did is
// replayed by State: llm_calls and dice_rolled_new are zero.
//
// It is published once, without the attempts of an answer: the start of the
// process does not wait on the bus, and a swarm that misses the signal starts
// after its own wait with /health degraded (C-14 v0.4 (б)).
func (a *Applier) announceRecovery(ctx context.Context, rec Recovery, end int64) {
	replay := map[string]any{
		"run_id":            fmt.Sprintf("%s:%s:%s", Component, a.worldID, strconv.FormatInt(end, 10)),
		"snapshot_id":       nil,
		"events_replayed":   rec.EventsReplayed,
		"llm_calls":         0,
		"dice_rolled_new":   0,
		"duration_ms":       rec.Duration.Milliseconds(),
		"state_hash_after":  rec.StateHashAfter,
		"incomplete_record": false,
	}
	if rec.SnapshotID != "" {
		replay["snapshot_id"] = rec.SnapshotID
		replay["state_hash_before"] = rec.StateHashBefore
	}
	if identical, known := rec.Identical(); known {
		replay["identical"] = identical
	}
	ev := eventbus.NewRoot(TypeReplayCompleted, a.source, a.worldID, nil, eventbus.ActorSystem,
		map[string]any{"mode": ReplayModeRecovery, "replay": replay})
	if err := a.pub.Publish(context.WithoutCancel(ctx), ev); err != nil {
		a.log.Error("analytics.replay.completed did not go out; the world is served all the same",
			"err", err, "handled", true)
	}
}

// LastRecovery is what the last recovery of the world found.
func (a *Applier) LastRecovery() Recovery {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.recovery
}
