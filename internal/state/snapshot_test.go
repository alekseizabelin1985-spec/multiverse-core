package state_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
)

// --- snapshots (§4.4, §4.9, C-14) ---

// A snapshot is the world, the window of proposals and the cursor at one
// moment: the object under the key its instant and sequence give, then the
// pointer at it, then snapshot.created — the pointer without the three fields
// the closed schema does not know, and the world in the envelope only.
func TestASnapshotHoldsTheWorldItsWindowAndItsCursor(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.seedWorld(t)
	at := func(offset int64) context.Context {
		return eventbus.ContextWithPosition(context.Background(), eventbus.Position{Topic: eventbus.TopicSystemEvents, Offset: offset})
	}
	for i, id := range []string{"player-A", "wolf-alpha"} {
		typ := entity.TypePlayer
		if id == "wolf-alpha" {
			typ = entity.TypeNPC
		}
		if err := f.applier.Apply(at(int64(40+i)), create("prop-"+id, ref(id, typ), "", map[string]any{"hp": 10})); err != nil {
			t.Fatal(err)
		}
	}
	f.clock.Advance(90 * time.Second)

	pointer, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	s := pointer.Snapshot
	world0 := f.store.List(world)
	if s.Seq != 0 || s.ID != "state:"+world+":000000" || s.Key != state.SnapshotKey(f.clock.Now(), 0) ||
		!s.TakenAt.Equal(f.clock.Now()) || s.Cursor["system_events"] != 42 || s.LawsVersion != "v1" ||
		s.RulesVersion != "0.1" || s.EntitiesCount != 3 || s.Reason != state.SnapshotAdmin ||
		s.StateHash != entity.StateHash(world0) {
		t.Errorf("snapshot block %+v, want seq 0 at the clock, cursor 42, three entities and their hash", s)
	}
	if pointer.Writer != "core/state@test:0" || pointer.World.Entity.ID != world || pointer.Component != "state" {
		t.Errorf("pointer %+v, want the writer, the world and the component", pointer)
	}

	var stored state.LatestPointer
	readJSON(t, objects.Memory, objstore.SnapshotsBucket(world), state.PointerKey, &stored)
	if !reflect.DeepEqual(&stored, pointer) {
		t.Errorf("latest.json %+v, Snapshot returned %+v", stored, *pointer)
	}
	var object state.Snapshot
	raw := readJSON(t, objects.Memory, objstore.SnapshotsBucket(world), s.Key, &object)
	if s.SizeBytes == nil || *s.SizeBytes != int64(len(raw)) {
		t.Errorf("size_bytes %v, the object is %d bytes", s.SizeBytes, len(raw))
	}
	block := s
	block.SizeBytes = nil
	if !reflect.DeepEqual(object.Snapshot, block) || object.WorldID != world || object.SchemaVersion != 1 {
		t.Errorf("object block %+v, want the pointer block without size_bytes", object.Snapshot)
	}
	if got := refsOf(object.Entities); !slices.Equal(got, []string{"npc:wolf-alpha", "player:player-A", "world:" + world}) {
		t.Errorf("entities %v, want the (type, id) order", got)
	}
	if entity.StateHash(object.Entities) != s.StateHash {
		t.Error("the entities of the object do not hash to state_hash")
	}
	if !slices.Equal(object.AppliedProposals, []string{"prop-player-A", "prop-wolf-alpha"}) {
		t.Errorf("applied_proposals %v, want the two proposals oldest first", object.AppliedProposals)
	}

	announced := f.journal.ofType(state.TypeSnapshotCreated)
	if len(announced) != 1 {
		t.Fatalf("%d snapshot.created, want 1", len(announced))
	}
	ev := announced[0]
	if ev.World == nil || ev.World.Entity.ID != world || ev.Source != contracts.SourceState {
		t.Errorf("snapshot.created in world %+v from %s, want %s from core/state", ev.World, ev.Source, world)
	}
	payload, _ := ev.Payload["snapshot"].(map[string]any)
	if keys := slices.Sorted(maps.Keys(payload)); !slices.Equal(keys,
		[]string{"cursor", "id", "key", "laws_version", "seq", "size_bytes", "state_hash", "taken_at"}) {
		t.Errorf("snapshot.created carries %v, want the eight fields of the schema", keys)
	}
	if ev.Payload["component"] != "state" || payload["key"] != s.Key || payload["state_hash"] != s.StateHash {
		t.Errorf("snapshot.created %v, want the key and the hash of the pointer", ev.Payload)
	}
}

// The rotation keeps the five newest snapshot objects: the sixth removes the
// first, the seventh the second, and latest.json points at the last. The
// numbers keep rising past the rotation — the count of objects stops rising
// at five, the sequence must not (review #1 of T-057, Mi-4).
func TestTheRotationKeepsFiveSnapshots(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.seedWorld(t)
	var keys []string
	ids := map[string]bool{}
	for i := range 8 {
		pointer, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin)
		if err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		if pointer.Snapshot.Seq != int64(i) || ids[pointer.Snapshot.ID] {
			t.Errorf("snapshot %d got seq %d and id %s (seen before: %v), want seq %d and a new id",
				i+1, pointer.Snapshot.Seq, pointer.Snapshot.ID, ids[pointer.Snapshot.ID], i)
		}
		ids[pointer.Snapshot.ID] = true
		keys = append(keys, pointer.Snapshot.Key)
		f.clock.Advance(time.Minute)
	}
	store := state.NewObjectStore(objects)
	refs, err := store.ListSnapshots(context.Background(), world)
	if err != nil {
		t.Fatal(err)
	}
	var seqs []int64
	for _, r := range refs {
		seqs = append(seqs, r.Seq)
	}
	if !slices.Equal(seqs, []int64{7, 6, 5, 4, 3}) {
		t.Errorf("snapshots left %v, want 7 6 5 4 3", seqs)
	}
	for _, gone := range keys[:3] {
		if _, err := objects.Stat(context.Background(), objstore.SnapshotsBucket(world), gone); !errors.Is(err, objstore.ErrNotFound) {
			t.Errorf("the rotated snapshot %s is still there (%v)", gone, err)
		}
	}
	latest, err := store.ReadLatest(context.Background(), world)
	if err != nil || latest.Snapshot.Key != keys[7] || latest.Snapshot.Seq != 7 {
		t.Errorf("latest.json %+v, %v; want the eighth snapshot %s", latest, err, keys[7])
	}
}

// The log of a written snapshot carries its size and the time its writes took
// on the clock of State (§10; the condition of review of ADR-011, addendum
// 2026-09-14, p. 1).
func TestTheLogOfASnapshotCarriesItsSizeAndDuration(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.seedWorld(t)
	objects.setHook(func(_, key string) error {
		if key != state.PointerKey {
			f.clock.Advance(250 * time.Millisecond)
		}
		return nil
	})
	pointer, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin)
	if err != nil {
		t.Fatal(err)
	}
	records := f.logged(t, "snapshot written")
	if len(records) != 1 {
		t.Fatalf("%d records of the snapshot, want 1", len(records))
	}
	if size, _ := records[0]["size_bytes"].(float64); pointer.Snapshot.SizeBytes == nil || int64(size) != *pointer.Snapshot.SizeBytes {
		t.Errorf("size_bytes %v in the log, the pointer says %v", records[0]["size_bytes"], pointer.Snapshot.SizeBytes)
	}
	if records[0]["duration_ms"] != float64(250) || records[0]["reason"] != state.SnapshotAdmin || records[0]["seq"] != float64(0) {
		t.Errorf("log %v, want duration_ms 250, reason admin and seq 0", records[0])
	}
}

// A snapshot over a store that does not answer is cut off after SnapshotTimeout
// on the timers of State: Stop comes back in time with an error that says so,
// latest.json is not moved, and the log names the bound (review #1 of T-057,
// Mi-1).
func TestAShutdownSnapshotOverAHangingStoreIsBounded(t *testing.T) {
	bus := rig(t)
	objects := newTracedObjects(t, world)
	c, manual, logged := runningStoredLogged(t, bus, objects, -1)
	publish(t, bus.Bus, createWorld())
	untilEnd(t, bus.Bus, 2)
	waitFor(t, "the world entity", func() bool { return worldHealth(c, world)["entities"] == 1 })
	held := objects.holdUntilCancelled(func(key string) bool { return strings.HasPrefix(key, "state/") })

	stopped := make(chan error, 1)
	began := testkit.Wall().Now()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		stopped <- c.Stop(ctx)
	}()
	<-held
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned %v before the bound of the snapshot", err)
	default:
	}
	manual.Advance(state.SnapshotTimeout)
	err := <-stopped
	if err == nil || !strings.Contains(err.Error(), "did not finish within 10s") {
		t.Errorf("Stop = %v, want the shutdown snapshot cut off by its bound", err)
	}
	if took := testkit.Wall().Now().Sub(began); took >= runtime.StopTimeout {
		t.Errorf("Stop took %v, the budget is %v", took, runtime.StopTimeout)
	}
	if _, err := state.NewObjectStore(objects).ReadLatest(context.Background(), world); !errors.Is(err, state.ErrNoSnapshot) {
		t.Errorf("latest.json after the cut-off snapshot: %v, want none", err)
	}
	records := recordsOf(t, logged.String(), "snapshot not written; the world goes on")
	if len(records) != 1 || records[0]["reason"] != state.SnapshotShutdown ||
		!strings.Contains(fmt.Sprint(records[0]["err"]), "did not finish within 10s") {
		t.Errorf("log %v, want one record of the shutdown snapshot naming its bound", records)
	}
}

// A process that dies between the object and the pointer leaves latest.json at
// the snapshot before, whole: its object is there and hashes to its state_hash.
// The object without a pointer keeps its number, and the next snapshot takes
// the one after it.
func TestACrashBetweenTheObjectAndThePointerLeavesTheSnapshotBefore(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.seedWorld(t)
	if _, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin); err != nil {
		t.Fatal(err)
	}
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	f.clock.Advance(time.Minute)
	objects.setHook(func(_, key string) error {
		if key == state.PointerKey {
			return errors.New("the process died")
		}
		return nil
	})
	if _, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin); err == nil {
		t.Fatal("Snapshot succeeded without its pointer")
	}
	if _, snapshotErr := f.applier.SnapshotHealth(); snapshotErr == nil {
		t.Error("the failed snapshot is not reported")
	}

	store := state.NewObjectStore(objects)
	latest, err := store.ReadLatest(context.Background(), world)
	if err != nil || latest.Snapshot.Seq != 0 {
		t.Fatalf("latest.json %+v, %v; want seq 0", latest, err)
	}
	object, err := store.ReadSnapshot(context.Background(), world, latest.Snapshot.Key)
	if err != nil || entity.StateHash(object.Entities) != latest.Snapshot.StateHash || len(object.Entities) != 1 {
		t.Errorf("the snapshot latest.json points at does not hold its world: %+v, %v", object, err)
	}
	if announced := f.journal.ofType(state.TypeSnapshotCreated); len(announced) != 1 {
		t.Errorf("%d snapshot.created, want only the one of seq 0", len(announced))
	}

	objects.setHook(nil)
	f.clock.Advance(time.Minute)
	pointer, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin)
	if err != nil || pointer.Snapshot.Seq != 2 {
		t.Fatalf("the snapshot after the crash %+v, %v; want seq 2", pointer, err)
	}
	if _, snapshotErr := f.applier.SnapshotHealth(); snapshotErr != nil {
		t.Errorf("a written snapshot still reports %v", snapshotErr)
	}
}

// Every SnapshotEvery applied facts a snapshot with reason interval is written
// before Apply returns, after the facts of the proposal that completed the
// count; its snapshot.created derives from that proposal.
func TestTheCountOfFactsWritesASnapshot(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{SnapshotEvery: 4})
	f.seedWorld(t)
	f.apply(t, create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	f.apply(t, create("prop-w", ref("wolf-alpha", entity.TypeNPC), "", map[string]any{"hp": 10}))
	if got := objects.timeline.matching("put state/"); len(got) != 0 {
		t.Fatalf("a snapshot after two facts: %q", got)
	}
	round := update(t, "prop-round", "combat", true,
		set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -1)),
		set(ref("wolf-alpha", entity.TypeNPC), version(1), op(entity.OpInc, "hp", -1)))
	round.Scope = &eventbus.ScopeRef{ID: "solo:player-A", Type: "solo"}
	round.Meta.ActorKind = eventbus.ActorHuman
	f.apply(t, round)
	f.apply(t, update(t, "prop-after", "combat", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))

	announced := f.journal.ofType(state.TypeSnapshotCreated)
	if len(announced) != 1 {
		t.Fatalf("%d snapshot.created, want 1 after five facts with a count of four", len(announced))
	}
	ev := announced[0]
	if ev.Meta.CausationID != round.ID {
		t.Errorf("snapshot.created caused by %s, want the proposal that completed the count %s", ev.Meta.CausationID, round.ID)
	}
	// The world is the envelope of the snapshot, not the scope of the player
	// whose proposal completed the count; the id stays derived from the cause.
	derived := eventbus.Derive(round, state.TypeSnapshotCreated, contracts.SourceState, nil, eventbus.WithCauseID("state", "0"))
	if ev.Scope != nil || ev.World == nil || ev.World.Entity.ID != world || ev.ID != derived.ID ||
		ev.Meta.ActorKind != eventbus.ActorSystem || !ev.Timestamp.Equal(round.Timestamp) {
		t.Errorf("snapshot.created scope %+v, world %+v, id %s, actor %s; want no scope, %s, %s and system",
			ev.Scope, ev.World, ev.ID, ev.Meta.ActorKind, world, derived.ID)
	}
	all := f.journal.all()
	if last := all[len(all)-2]; last.Type != state.TypeSnapshotCreated {
		t.Errorf("the event before the last fact is %s, want snapshot.created right after the facts of prop-round", last.Type)
	}
	latest, err := state.NewObjectStore(objects).ReadLatest(context.Background(), world)
	if err != nil || latest.Snapshot.Reason != state.SnapshotInterval {
		t.Errorf("latest.json %+v, %v; want reason interval", latest, err)
	}
}

// The window of proposals in a snapshot is bounded by 1000, the newest ones in
// the order they were applied, and two runs of the same proposals write the
// same bytes (C-14 v1.2 (a), §4.4).
func TestTheWindowOfASnapshotIsBoundedAndInOrder(t *testing.T) {
	run := func() ([]byte, state.Snapshot) {
		f, objects := newStoredFixture(t, state.ApplierConfig{})
		f.seedWorld(t)
		f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
		for i := range 1005 {
			f.apply(t, update(t, fmt.Sprintf("prop-%04d", i), "combat", true,
				set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", 1))))
		}
		pointer, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin)
		if err != nil {
			t.Fatal(err)
		}
		var object state.Snapshot
		raw := readJSON(t, objects.Memory, objstore.SnapshotsBucket(world), pointer.Snapshot.Key, &object)
		return raw, object
	}
	first, object := run()
	second, _ := run()
	if n := len(object.AppliedProposals); n != state.SnapshotProposals {
		t.Fatalf("%d proposals in the snapshot, want %d", n, state.SnapshotProposals)
	}
	if object.AppliedProposals[0] != "prop-0005" || object.AppliedProposals[999] != "prop-1004" {
		t.Errorf("window from %s to %s, want prop-0005 to prop-1004, oldest first",
			object.AppliedProposals[0], object.AppliedProposals[999])
	}
	if !bytes.Equal(first, second) {
		t.Error("two runs of the same proposals wrote different snapshots")
	}
}

// snapshot.created without a world is refused by Publish (C-14 v1.3); with the
// world in the envelope it goes out.
func TestSnapshotCreatedRequiresAWorld(t *testing.T) {
	bus := newBus(t)
	testkit.Deterministic(t, "t057")
	eventbus.SetRegistry(contracts.Default())
	payload := map[string]any{"component": "state", "snapshot": map[string]any{
		"id": "state:" + world + ":000000", "seq": 0, "taken_at": "2026-09-09T12:00:00Z",
		"cursor": map[string]any{"system_events": 0}, "laws_version": "v1", "size_bytes": 10,
		"key":        "state/20260909T120000Z-000000.json",
		"state_hash": "sha256:0046b8a738ddec9996ad73e31df7e73534b4bad9cf75e501dd4985a0fbe7c29e",
	}}
	without := eventbus.NewRoot(state.TypeSnapshotCreated, contracts.SourceState, "", nil, eventbus.ActorSystem, payload)
	if err := bus.Publish(context.Background(), without); !errors.Is(err, eventbus.ErrPolicyViolation) {
		t.Errorf("Publish of snapshot.created without a world = %v, want ErrPolicyViolation", err)
	}
	with := eventbus.NewRoot(state.TypeSnapshotCreated, contracts.SourceState, world, nil, eventbus.ActorSystem, payload)
	if err := bus.Publish(context.Background(), with); err != nil {
		t.Errorf("Publish of snapshot.created with its world = %v", err)
	}
}

// Stop writes the shutdown snapshot once the subscription has returned and the
// workers have ended, and publishes snapshot.created before the process closes
// the bus; the cursor is past the last proposal read.
func TestStopWritesTheShutdownSnapshotBeforeTheBusCloses(t *testing.T) {
	bus := rig(t)
	objects := newTracedObjects(t, world)
	c, _ := runningStored(t, bus, objects, -1)
	last := create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10})
	publish(t, bus.Bus, createWorld(), last)
	untilEnd(t, bus.Bus, 4)
	waitFor(t, "both entities in the world", func() bool { return worldHealth(c, world)["entities"] == 2 })
	want := offsetOf(t, bus.Bus, last.ID) + 1
	bus.setHook(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == state.TypeSnapshotCreated {
			bus.note("snapshot.created published")
		}
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	if err := c.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	_ = bus.Close()
	timeline := bus.events()
	if len(timeline) != 3 || timeline[1] != "snapshot.created published" || timeline[2] != "bus closed" {
		t.Errorf("timeline %q, want the subscription returned, snapshot.created, then the bus closed", timeline)
	}
	latest, err := state.NewObjectStore(objects).ReadLatest(context.Background(), world)
	if err != nil || latest.Snapshot.Reason != state.SnapshotShutdown || latest.Snapshot.EntitiesCount != 2 ||
		latest.Snapshot.Cursor["system_events"] != want {
		t.Errorf("latest.json %+v, %v; want a shutdown snapshot of two entities with cursor %d", latest, err, want)
	}
}

// offsetOf is the offset of an event on system_events.
func offsetOf(t *testing.T, bus interface {
	Records(topic string) ([][]byte, error)
}, eventID string) int64 {
	t.Helper()
	records, err := bus.Records(eventbus.TopicSystemEvents)
	if err != nil {
		t.Fatal(err)
	}
	for i, body := range records {
		if bytes.Contains(body, []byte(`"id":"`+eventID+`"`)) {
			return int64(i)
		}
	}
	t.Fatalf("%s is not on %s", eventID, eventbus.TopicSystemEvents)
	return -1
}

// An admin snapshot is written on the worker between two proposals; it needs a
// running context, a world of the context and an object store.
func TestASnapshotAskedOfTheContext(t *testing.T) {
	bus := rig(t)
	objects := newTracedObjects(t, world)
	c, _ := runningStored(t, bus, objects, -1)
	publish(t, bus.Bus, createWorld())
	untilEnd(t, bus.Bus, 2)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		publish(t, bus.Bus, create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	}()
	pointer, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin)
	wg.Wait()
	if err != nil || pointer.Snapshot.Reason != state.SnapshotAdmin {
		t.Fatalf("Snapshot = %+v, %v; want an admin snapshot", pointer, err)
	}
	if n := pointer.Snapshot.EntitiesCount; n != 1 && n != 2 {
		t.Errorf("entities_count %d, want the world before or after prop-a, never halfway", n)
	}
	section := worldHealth(c, world)
	if snap, _ := section["snapshot"].(map[string]any); snap["seq"] != int64(0) {
		t.Errorf("/health snapshot %v, want seq 0", section["snapshot"])
	}

	if _, err := c.Snapshot(context.Background(), "another-world", state.SnapshotAdmin); err == nil ||
		!strings.Contains(err.Error(), "another-world") {
		t.Errorf("Snapshot of a world the context does not serve = %v, want a refusal naming it", err)
	}
	memoryOnly, _ := running(t, newBus(t), world)
	if _, err := memoryOnly.Snapshot(context.Background(), world, state.SnapshotAdmin); !errors.Is(err, state.ErrNoObjectStore) {
		t.Errorf("Snapshot without an object store = %v, want ErrNoObjectStore", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	_ = c.Stop(ctx)
	if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err == nil {
		t.Error("Snapshot of a stopped context succeeded")
	}
}
