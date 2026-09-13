package state_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/testkit"
)

// --- the Store over shared/objstore (§4.3) ---

// An entity goes out whole under {type}/{id}.json and comes back as it was,
// last_change and history included; the listing is in (type, id) order and
// does not mistake an intent for an entity.
func TestAnEntityObjectRoundTrips(t *testing.T) {
	ctx := context.Background()
	objects := newTracedObjects(t, world)
	store := state.NewObjectStore(objects)

	wolf := entity.New(ref("wolf-alpha", entity.TypeNPC), world, "Альфа-волк", map[string]any{"hp": 10, "loot": []any{"pelt"}}, testkit.Epoch)
	wolf.Commit(map[string]any{"hp": 7, "loot": []any{"pelt"}},
		[]entity.Change{{Path: "hp", Old: 10, New: 7, HasOld: true, HasNew: true}},
		entity.LastChange{ProposalID: "prop-1", ProposalEventID: "ev-1", Cause: "combat", AppliedAt: testkit.Epoch, Atomic: true, BatchSize: 1})
	player := entity.New(ref("player-A", entity.TypePlayer), world, "", map[string]any{"hp": 10}, testkit.Epoch)
	for _, e := range []*entity.Entity{wolf, player} {
		if err := store.PutEntity(ctx, world, e); err != nil {
			t.Fatalf("PutEntity %s: %v", e.ID, err)
		}
	}
	if err := store.PutIntent(ctx, world, &state.Intent{ProposalID: "prop-1", World: world}); err != nil {
		t.Fatalf("PutIntent: %v", err)
	}
	if _, err := objects.Stat(ctx, objstore.EntitiesBucket(world), "npc/wolf-alpha.json"); err != nil {
		t.Errorf("the wolf is not under npc/wolf-alpha.json: %v", err)
	}

	got, err := store.GetEntity(ctx, world, entity.TypeNPC, "wolf-alpha")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if entity.StateHash([]*entity.Entity{got}) != entity.StateHash([]*entity.Entity{wolf}) ||
		got.Version != 2 || got.LastChange == nil || got.LastChange.ProposalID != "prop-1" ||
		got.LastChange.FactEventID != "" || len(got.LastChange.Changed) != 1 || len(got.History) != 1 {
		t.Errorf("the wolf came back as %+v, want it as it was written", got)
	}
	listed, err := store.ListEntities(ctx, world)
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if len(listed) != 2 || listed[0].ID != "wolf-alpha" || listed[1].ID != "player-A" {
		t.Errorf("listed %v, want npc wolf-alpha then player player-A and no intent", refsOf(listed))
	}
	if _, err := store.GetEntity(ctx, world, entity.TypeNPC, "nobody"); !errors.Is(err, state.ErrNotFound) {
		t.Errorf("GetEntity of nobody = %v, want ErrNotFound", err)
	}
}

// An intent is kept under _intents/ with its proposal_id escaped — a
// proposal_id is free text with slashes in it — and removing it twice is not
// an error.
func TestAnIntentIsKeptUnderItsProposal(t *testing.T) {
	ctx := context.Background()
	objects := newTracedObjects(t, world)
	store := state.NewObjectStore(objects)
	in := &state.Intent{
		ProposalID: "bootstrap:" + world + ":npc/wolf", ProposalEventID: "ev-1", World: world, Cause: "move",
		Changes: []state.IntentChange{{Ref: ref("g-1", "group"), FromVersion: 7, ToVersion: 8,
			AttributesAfter: map[string]any{"position": "dark-forest-01"}}},
	}
	if err := store.PutIntent(ctx, world, in); err != nil {
		t.Fatalf("PutIntent: %v", err)
	}
	if _, err := objects.Stat(ctx, objstore.EntitiesBucket(world), "_intents/bootstrap:"+world+":npc%2Fwolf.json"); err != nil {
		t.Errorf("the intent is not under its escaped key: %v", err)
	}
	intents, err := store.ListIntents(ctx, world)
	if err != nil || len(intents) != 1 || !reflect.DeepEqual(intents[0], in) {
		t.Fatalf("ListIntents = %+v, %v; want the intent as written", intents, err)
	}
	for range 2 {
		if err := store.DeleteIntent(ctx, world, in.ProposalID); err != nil {
			t.Fatalf("DeleteIntent: %v", err)
		}
	}
	if intents, _ := store.ListIntents(ctx, world); len(intents) != 0 {
		t.Errorf("%d intents after DeleteIntent, want none", len(intents))
	}
}

// The object is written before the pointer, the pointer carries size_bytes of
// the object and the object does not; the listing knows snapshot objects only,
// highest seq first; a world without latest.json is ErrNoSnapshot.
func TestASnapshotObjectAndItsPointer(t *testing.T) {
	ctx := context.Background()
	objects := newTracedObjects(t, world)
	store := state.NewObjectStore(objects)
	if _, err := store.ReadLatest(ctx, world); !errors.Is(err, state.ErrNoSnapshot) {
		t.Fatalf("ReadLatest of a world without snapshots = %v, want ErrNoSnapshot", err)
	}

	for seq := range int64(3) {
		taken := testkit.Epoch.Add(time.Duration(seq) * time.Minute)
		snap := &state.Snapshot{SchemaVersion: 1, Component: state.Component, WorldID: world,
			Snapshot: state.SnapshotMeta{ID: "s", Seq: seq, Key: state.SnapshotKey(taken, seq), TakenAt: taken,
				Cursor: map[string]int64{"system_events": seq}, LawsVersion: "v1", Reason: state.SnapshotAdmin},
			Entities: []*entity.Entity{}, AppliedProposals: []string{}}
		pointer, err := store.PutSnapshot(ctx, world, snap, taken, "core/state@test:0")
		if err != nil {
			t.Fatalf("PutSnapshot %d: %v", seq, err)
		}
		raw, _ := objects.Get(ctx, objstore.SnapshotsBucket(world), snap.Snapshot.Key)
		if pointer.Snapshot.SizeBytes == nil || *pointer.Snapshot.SizeBytes != int64(len(raw)) {
			t.Errorf("size_bytes %v, the object is %d bytes", pointer.Snapshot.SizeBytes, len(raw))
		}
	}
	if steps := objects.timeline.all(); len(steps) != 6 || steps[4] != "put state/20260909T120200Z-000002.json" ||
		steps[5] != "put state/latest.json" {
		t.Errorf("writes %q, want each object before its pointer", steps)
	}
	latest, err := store.ReadLatest(ctx, world)
	if err != nil || latest.Snapshot.Seq != 2 || latest.World.Entity.ID != world || latest.Component != "state" {
		t.Fatalf("ReadLatest = %+v, %v; want seq 2 of %s", latest, err, world)
	}
	object, err := store.ReadSnapshot(ctx, world, latest.Snapshot.Key)
	if err != nil || object.Snapshot.SizeBytes != nil || object.Snapshot.Seq != 2 {
		t.Errorf("ReadSnapshot = %+v, %v; want seq 2 without size_bytes", object, err)
	}
	refs, err := store.ListSnapshots(ctx, world)
	if err != nil || len(refs) != 3 || refs[0].Seq != 2 || refs[2].Seq != 0 {
		t.Errorf("ListSnapshots = %+v, %v; want seq 2, 1, 0 and not latest.json", refs, err)
	}
	if _, err := store.ReadSnapshot(ctx, world, "state/nothing.json"); !errors.Is(err, state.ErrNotFound) {
		t.Errorf("ReadSnapshot of a missing key = %v, want ErrNotFound", err)
	}
}

// EnsureWorldBuckets asks for the rules of the layout: both buckets versioned
// with the expiry of non-current versions (ADR-021 p. 3). The memory store
// records them; the server applies them (store_integration_test.go).
func TestTheBucketsOfAWorldGetTheRulesOfTheLayout(t *testing.T) {
	objects := objstore.NewMemory()
	if err := state.EnsureWorldBuckets(context.Background(), objects, world); err != nil {
		t.Fatal(err)
	}
	for _, bucket := range []string{objstore.EntitiesBucket(world), objstore.SnapshotsBucket(world)} {
		if opts, ok := objects.BucketOptionsOf(bucket); !ok || !opts.Versioned || opts.NoncurrentExpireDays != 30 {
			t.Errorf("%s: %+v (exists %v), want versioned with a 30 day expiry", bucket, opts, ok)
		}
	}
}

func refsOf(entities []*entity.Entity) []string {
	out := make([]string, 0, len(entities))
	for _, e := range entities {
		out = append(out, e.Ref().String())
	}
	return out
}
