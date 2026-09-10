package state_test

import (
	"context"
	"encoding/json"
	"maps"
	"reflect"
	"sync"
	"testing"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/state"
)

func read(t *testing.T, store *objstore.Memory, key string, dst any) []byte {
	t.Helper()
	body, err := store.Get(context.Background(), objstore.SnapshotsBucket(worldID), key)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		t.Fatalf("decode %s: %v", key, err)
	}
	return body
}

// TestSnapshotWritesTheObjectAndThenThePointer is the format of §4.4 and the
// order C-14 fixes: the object first, the pointer after it, so that a reader
// that finds a pointer finds the object it names.
//
// Every number of the pointer is recomputed here from the world it describes.
// Nothing is compared with a literal, because a literal would only prove that
// the stub still writes what it wrote when the test was written.
func TestSnapshotWritesTheObjectAndThenThePointer(t *testing.T) {
	fake, _, store := world(t)

	pointer, err := fake.Snapshot(context.Background(), state.ReasonBootstrap)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	var onDisk state.Pointer
	read(t, store, state.PointerKey, &onDisk)
	if !reflect.DeepEqual(onDisk, pointer) {
		t.Errorf("the pointer returned and the pointer written differ\n  returned: %+v\n  written:  %+v",
			pointer, onDisk)
	}

	if onDisk.SchemaVersion != 1 || onDisk.Component != state.Component {
		t.Errorf("schema_version %d component %q, want 1 / %s",
			onDisk.SchemaVersion, onDisk.Component, state.Component)
	}
	if want := (entity.Ref{ID: worldID, Type: entity.TypeWorld}); onDisk.World.Entity != want {
		t.Errorf("world %+v, want %+v", onDisk.World.Entity, want)
	}
	if onDisk.Writer != state.Writer {
		t.Errorf("writer %q, want %q", onDisk.Writer, state.Writer)
	}
	if onDisk.WrittenAt.Before(onDisk.Snapshot.TakenAt) {
		t.Errorf("written_at %s is before taken_at %s", onDisk.WrittenAt, onDisk.Snapshot.TakenAt)
	}

	meta := onDisk.Snapshot
	if meta.Seq != 0 {
		t.Errorf("seq %d, want 0 for the first snapshot of a process", meta.Seq)
	}
	if want := "state:" + worldID + ":000000"; meta.ID != want {
		t.Errorf("id %q, want %q", meta.ID, want)
	}
	if want := state.SnapshotKey(meta.TakenAt, meta.Seq); meta.Key != want {
		t.Errorf("key %q, want %q: the key is taken_at and seq, not a name of its own", meta.Key, want)
	}
	if meta.Reason != state.ReasonBootstrap {
		t.Errorf("reason %q, want %q", meta.Reason, state.ReasonBootstrap)
	}
	if want := map[string]int64{"system_events": 0}; !maps.Equal(meta.Cursor, want) {
		t.Errorf("cursor %v, want %v: nothing has been read off the journal", meta.Cursor, want)
	}

	entities := fake.All()
	if meta.EntitiesCount != len(entities) {
		t.Errorf("entities_count %d, the world holds %d", meta.EntitiesCount, len(entities))
	}
	if want := entity.StateHash(entities); meta.StateHash != want {
		t.Errorf("state_hash %q, the world hashes to %q", meta.StateHash, want)
	}
	if meta.LawsVersion == "" {
		t.Error("laws_version is empty: it is on the world entity")
	}
	if meta.RulesVersion != "0.1" {
		t.Errorf("rules_version %q, want what the caller configured", meta.RulesVersion)
	}

	// The object the pointer names, reached the way a consumer reaches it.
	var object state.Object
	raw := read(t, store, meta.Key, &object)
	if meta.SizeBytes == nil {
		t.Fatal("no size_bytes: the pointer is what tells a consumer how big the object is")
	}
	if want := int64(len(raw)); *meta.SizeBytes != want {
		t.Errorf("size_bytes %d, the object is %d bytes", *meta.SizeBytes, want)
	}
	if object.Snapshot.SizeBytes != nil {
		t.Error("the object states its own size: only the pointer can (§4.4, T-016)")
	}

	block := meta
	block.SizeBytes = nil
	if !reflect.DeepEqual(object.Snapshot, block) {
		t.Errorf("the metadata of the object and of the pointer differ\n  object:  %+v\n  pointer: %+v",
			object.Snapshot, block)
	}
	if object.WorldID != worldID {
		t.Errorf("world_id %q, want %q", object.WorldID, worldID)
	}
	if !reflect.DeepEqual(object.Entities, entities) {
		t.Error("the object and the world hold different entities")
	}
	if hash := entity.StateHash(object.Entities); hash != meta.StateHash {
		t.Errorf("the object hashes to %q, the pointer says %q", hash, meta.StateHash)
	}
}

// putRecorder is a store that remembers the order in which it was written to.
// The order is the only thing about C-14 no assertion over the files
// themselves can see: both files are there whichever way round they were
// written, and a stub that wrote the pointer first would hand a reader a
// reference to an object that does not exist yet.
type putRecorder struct {
	objstore.Client
	mu   sync.Mutex
	keys []string
}

func (r *putRecorder) Put(ctx context.Context, bucket, key string, body []byte,
	opts objstore.PutOptions) (string, error) {
	r.mu.Lock()
	r.keys = append(r.keys, key)
	r.mu.Unlock()
	return r.Client.Put(ctx, bucket, key, body, opts)
}

func (r *putRecorder) written() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.keys...)
}

// TestTheObjectIsWrittenBeforeThePointer is the order C-14 fixes, watched from
// inside the store.
func TestTheObjectIsWrittenBeforeThePointer(t *testing.T) {
	store := &putRecorder{Client: objstore.NewMemoryWithClock(clock.NewManual(testkit.Epoch))}
	fake := seeded(t, newBus(t), store)

	pointer, err := fake.Snapshot(context.Background(), state.ReasonBootstrap)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	want := []string{pointer.Snapshot.Key, state.PointerKey}
	have := store.written()
	if len(have) != len(want) || have[0] != want[0] || have[1] != want[1] {
		t.Fatalf("the store was written %v, want %v: the pointer must follow the object it names",
			have, want)
	}
}

// TestSnapshotFollowsTheWorld covers what a second snapshot has to show: a new
// sequence number, a new key, the cursor the subscription has reached, the
// proposals applied since, and a hash that moved with the world.
func TestSnapshotFollowsTheWorld(t *testing.T) {
	fake, bus, store := world(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); _ = fake.Wait() }()
	if err := fake.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	first, err := fake.Snapshot(ctx, state.ReasonBootstrap)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if err := bus.Publish(ctx, proposal("prop-snap", "combat", true,
		changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -4}))); err != nil {
		t.Fatalf("publish: %v", err)
	}
	waitFor(t, "the proposal to be applied", func() bool {
		hp, _ := mustGet(t, fake, playerA).HP()
		return hp == 6
	})

	second, err := fake.Snapshot(ctx, state.ReasonInterval)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if second.Snapshot.Seq != first.Snapshot.Seq+1 {
		t.Errorf("seq %d after %d", second.Snapshot.Seq, first.Snapshot.Seq)
	}
	if second.Snapshot.Key == first.Snapshot.Key {
		t.Error("the second snapshot overwrote the first")
	}
	if second.Snapshot.StateHash == first.Snapshot.StateHash {
		t.Error("the hash did not move although the world did")
	}
	if second.Snapshot.Cursor["system_events"] <= 0 {
		t.Errorf("cursor %v: the subscription has read at least the proposal",
			second.Snapshot.Cursor)
	}

	var object state.Object
	read(t, store, second.Snapshot.Key, &object)
	if len(object.AppliedProposals) == 0 || object.AppliedProposals[0] != "prop-snap" {
		t.Errorf("applied_proposals %v, want the proposal that was applied", object.AppliedProposals)
	}

	// The pointer now names the newer object, and the older one is still
	// there: a consumer that was reading it does not lose it (§4.4).
	var latest state.Pointer
	read(t, store, state.PointerKey, &latest)
	if latest.Snapshot.Key != second.Snapshot.Key {
		t.Errorf("the pointer names %q, the newest object is %q",
			latest.Snapshot.Key, second.Snapshot.Key)
	}
	if _, err := store.Stat(ctx, objstore.SnapshotsBucket(worldID), first.Snapshot.Key); err != nil {
		t.Errorf("the first object is gone: %v", err)
	}
}

// TestPointerYieldsAValidSnapshotCreated checks that what the stub writes is
// what the type of C-14 carries, even though the stub does not publish it: the
// registry lists state, swarm and gateway as the publishers of snapshot.created
// and not testkit/state, so a stub that published one would fail the contracts
// job instead of standing in for a publisher.
//
// The payload is a strict subset of the pointer — the schema knows neither
// rules_version, entities_count nor reason, which §4.4 keeps in latest.json —
// so the three are removed rather than smuggled in (the same decision T-016
// recorded).
func TestPointerYieldsAValidSnapshotCreated(t *testing.T) {
	fake, bus, store := world(t)
	if _, err := fake.Snapshot(context.Background(), state.ReasonBootstrap); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	for _, ev := range events(t, bus, eventbus.TopicSystemEvents) {
		if ev.Type == "snapshot.created" {
			t.Fatal("the stub published snapshot.created: testkit/state is not among its publishers")
		}
	}

	var pointer map[string]any
	read(t, store, state.PointerKey, &pointer)
	block, ok := pointer["snapshot"].(map[string]any)
	if !ok {
		t.Fatal("the pointer has no snapshot block")
	}
	payload := maps.Clone(block)
	for _, only := range []string{"rules_version", "entities_count", "reason"} {
		if _, present := payload[only]; !present {
			t.Errorf("the pointer lost %s: §4.4 keeps it", only)
		}
		delete(payload, only)
	}

	ev := eventbus.NewRoot("snapshot.created", contracts.SourceState, worldID, nil,
		entity.ActorKindCI, map[string]any{
			"component": pointer["component"],
			"snapshot":  payload,
		})
	if err := contracts.Validate(ev); err != nil {
		t.Fatalf("the pointer does not yield a valid snapshot.created: %v", err)
	}
}

func TestSnapshotWithoutAStoreIsAnError(t *testing.T) {
	testkit.Deterministic(t, "ev")
	fake, err := state.New(state.Config{Bus: nopBus{}, WorldID: worldID})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := fake.Snapshot(context.Background(), state.ReasonAdmin); err == nil {
		t.Fatal("wrote a snapshot with nowhere to write it")
	}
}

// --- the world the stub starts from ---

func TestLoadFixturesReadsTheDarkForestInBootstrapOrder(t *testing.T) {
	entities, err := state.LoadFixtures(fixturesDir())
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	want := []entity.Ref{
		{ID: worldID, Type: entity.TypeWorld},
		{ID: regionID, Type: entity.TypeRegion},
		{ID: npcID, Type: entity.TypeNPC},
		{ID: playerA, Type: entity.TypePlayer},
		{ID: "player-B", Type: entity.TypePlayer},
		{ID: "player-C", Type: entity.TypePlayer},
	}
	have := make([]entity.Ref, 0, len(entities))
	for _, e := range entities {
		have = append(have, e.Ref())
	}
	if !reflect.DeepEqual(have, want) {
		t.Errorf("fixtures %v, want %v", have, want)
	}

	if _, err := state.LoadFixtures(t.TempDir()); err == nil {
		t.Error("loaded a world out of an empty directory")
	}
}

// TestSeedIsACopyAndRefusesADuplicate covers the two things a caller can get
// wrong: editing the fixtures it seeded and seeding the same world twice.
func TestSeedIsACopyAndRefusesADuplicate(t *testing.T) {
	fake, _, _ := world(t)

	entities, err := state.LoadFixtures(fixturesDir())
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	if err := fake.Seed(entities); err == nil {
		t.Error("seeded a world that was already seeded")
	}

	for _, e := range entities {
		if e.ID == playerA {
			e.Attributes[entity.AttrHP] = 1
		}
	}
	if hp, _ := mustGet(t, fake, playerA).HP(); hp != 10 {
		t.Errorf("hp %d: the stub shares its entities with whoever seeded them", hp)
	}
	if err := fake.Seed([]*entity.Entity{nil}); err == nil {
		t.Error("seeded nothing at all")
	}
}

// nopBus is a bus that accepts everything and delivers nothing, for the tests
// that are about the stub rather than about the wiring.
type nopBus struct{}

func (nopBus) Publish(context.Context, eventbus.Event) error { return nil }
func (nopBus) Subscribe(ctx context.Context, _, _ string, _ eventbus.Handler) error {
	<-ctx.Done()
	return nil
}
func (nopBus) Close() error { return nil }
