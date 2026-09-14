package state_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/objstore"
)

// --- a snapshot with reason bootstrap only for a world without latest.json (§4.9, T-475) ---

// pointerBytes is latest.json of the world as the store holds it, or nil.
func pointerBytes(t *testing.T, objects objstore.Client) []byte {
	t.Helper()
	body, err := objects.Get(context.Background(), objstore.SnapshotsBucket(world), state.PointerKey)
	if errors.Is(err, objstore.ErrNotFound) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func snapshotObjects(t *testing.T, objects objstore.Client) int {
	t.Helper()
	refs, err := state.NewObjectStore(objects).ListSnapshots(context.Background(), world)
	if err != nil {
		t.Fatal(err)
	}
	return len(refs)
}

// (a) Over a world with a pointer a bootstrap is refused: latest.json is the
// same bytes, no snapshot object and no snapshot.created is added, /health has
// no failed snapshot, and the count of facts goes on as if nothing was asked —
// the fact that completes it writes the interval snapshot, and an admin
// snapshot after the refusal is written.
func TestABootstrapSnapshotOfAnInitializedWorldIsRefused(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{SnapshotEvery: 3})
	f.seedWorld(t)
	if _, err := f.applier.Snapshot(context.Background(), state.SnapshotBootstrap); err != nil {
		t.Fatalf("the bootstrap snapshot of a world without a pointer: %v", err)
	}
	f.apply(t, create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	f.apply(t, create("prop-b", ref("player-B", entity.TypePlayer), "", map[string]any{"hp": 10}))
	before, objectsBefore := pointerBytes(t, objects), snapshotObjects(t, objects)
	announcedBefore := len(f.journal.ofType(state.TypeSnapshotCreated))

	pointer, err := f.applier.Snapshot(context.Background(), state.SnapshotBootstrap)
	if !errors.Is(err, state.ErrWorldInitialized) || pointer != nil {
		t.Fatalf("Snapshot(bootstrap) over a pointer = %+v, %v; want ErrWorldInitialized", pointer, err)
	}
	if after := pointerBytes(t, objects); !bytes.Equal(after, before) || snapshotObjects(t, objects) != objectsBefore {
		t.Errorf("the refusal wrote: latest.json changed %v, %d snapshot objects, had %d",
			!bytes.Equal(after, before), snapshotObjects(t, objects), objectsBefore)
	}
	if got := len(f.journal.ofType(state.TypeSnapshotCreated)); got != announcedBefore {
		t.Errorf("%d snapshot.created after the refusal, had %d", got, announcedBefore)
	}
	if _, snapshotErr := f.applier.SnapshotHealth(); snapshotErr != nil {
		t.Errorf("the refusal is reported as a failed snapshot: %v", snapshotErr)
	}
	if _, section := f.applier.WorldHealth(); section["snapshot_stale"] != nil {
		t.Errorf("/health of the world after the refusal: %v", section)
	}

	f.apply(t, create("prop-c", ref("player-C", entity.TypePlayer), "", map[string]any{"hp": 10}))
	latest, err := state.NewObjectStore(objects).ReadLatest(context.Background(), world)
	if err != nil || latest.Snapshot.Reason != state.SnapshotInterval || latest.Snapshot.Seq != 1 {
		t.Fatalf("latest.json %+v, %v; want the interval snapshot seq 1 of the third fact: the refusal kept the count", latest, err)
	}
	admin, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin)
	if err != nil || admin.Snapshot.Reason != state.SnapshotAdmin || admin.Snapshot.Seq != 2 {
		t.Errorf("Snapshot(admin) after the refusal = %+v, %v; want seq 2 admin", admin, err)
	}
}

// (b) A latest.json that does not decode is a pointer all the same.
func TestABootstrapSnapshotOverAPointerThatDoesNotDecodeIsRefused(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.seedWorld(t)
	if _, err := objects.Put(context.Background(), objstore.SnapshotsBucket(world), state.PointerKey, []byte(`{"snapshot": [`),
		objstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	_, err := f.applier.Snapshot(context.Background(), state.SnapshotBootstrap)
	if !errors.Is(err, state.ErrWorldInitialized) || !errors.Is(err, state.ErrUndecodable) {
		t.Fatalf("Snapshot(bootstrap) over a pointer cut short = %v, want ErrWorldInitialized with ErrUndecodable", err)
	}
	if snapshotObjects(t, objects) != 0 || string(pointerBytes(t, objects)) != `{"snapshot": [` {
		t.Errorf("the refusal wrote over a pointer cut short")
	}
}

// (c) A snapshot object left without its pointer — a crash between the two PUT
// — does not lock the bootstrap out: the condition is the pointer, not seq 0,
// and the bootstrap snapshot takes the next seq.
func TestABootstrapSnapshotOverAnObjectWithoutItsPointer(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.seedWorld(t)
	if _, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin); err != nil {
		t.Fatal(err)
	}
	if err := objects.Delete(context.Background(), objstore.SnapshotsBucket(world), state.PointerKey); err != nil {
		t.Fatal(err)
	}
	pointer, err := f.applier.Snapshot(context.Background(), state.SnapshotBootstrap)
	if err != nil || pointer.Snapshot.Seq != 1 || pointer.Snapshot.Reason != state.SnapshotBootstrap {
		t.Fatalf("Snapshot(bootstrap) over an object without a pointer = %+v, %v; want seq 1 bootstrap", pointer, err)
	}
}

// (d) Two bootstraps asked of the context at once over a world without a
// pointer: the worker takes them one after the other, so exactly one is
// written — seq 0, bootstrap — and the other is ErrWorldInitialized.
func TestTwoBootstrapSnapshotsAtOnceWriteOne(t *testing.T) {
	bus := rig(t)
	objects := newTracedObjects(t, world)
	c, _ := runningStored(t, bus, objects, -1)
	publish(t, bus.Bus, createWorld())
	untilEnd(t, bus.Bus, 2)

	var wg sync.WaitGroup
	results := make([]error, 2)
	pointers := make([]*state.LatestPointer, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pointers[i], results[i] = c.Snapshot(context.Background(), world, state.SnapshotBootstrap)
		}()
	}
	wg.Wait()

	written, refused := 0, 0
	for i, err := range results {
		switch {
		case err == nil && pointers[i].Snapshot.Seq == 0 && pointers[i].Snapshot.Reason == state.SnapshotBootstrap:
			written++
		case errors.Is(err, state.ErrWorldInitialized) && pointers[i] == nil:
			refused++
		default:
			t.Errorf("call %d: %+v, %v", i, pointers[i], err)
		}
	}
	if written != 1 || refused != 1 {
		t.Fatalf("%d written and %d refused, want one of each", written, refused)
	}
	latest, err := state.NewObjectStore(objects).ReadLatest(context.Background(), world)
	if err != nil || latest.Snapshot.Seq != 0 || latest.Snapshot.Reason != state.SnapshotBootstrap || snapshotObjects(t, objects) != 1 {
		t.Errorf("latest.json %+v, %v, %d snapshot objects; want the one seq 0 bootstrap", latest, err, snapshotObjects(t, objects))
	}
}
