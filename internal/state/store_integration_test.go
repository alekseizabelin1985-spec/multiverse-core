//go:build integration

// The Store of State against the object store server of the platform: the
// image built by build/minio.Dockerfile, MINIO_IMAGE in build/versions.env
// (ADR-021 p. 1). The unit tests pin the same behaviour on objstore.Memory;
// this file is what keeps the two from forking.
package state_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/testkit"
)

const (
	minioUser     = "multiverse-test"
	minioPassword = "multiverse-test-password"
)

// minioServer starts the pinned image and returns the client of the platform
// and the raw SDK client the assertions on the bucket rules need.
func minioServer(t *testing.T) (objstore.Client, *miniogo.Client) {
	t.Helper()
	image, err := testkit.Version("MINIO_IMAGE")
	if err != nil {
		t.Fatalf("MINIO_IMAGE: %v", err)
	}
	ctx := context.Background()
	container, err := tcminio.Run(ctx, image, tcminio.WithUsername(minioUser), tcminio.WithPassword(minioPassword))
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start %s: %v", image, err)
	}
	endpoint, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	client, err := objstore.New(objstore.Config{Endpoint: endpoint, AccessKey: minioUser, SecretKey: minioPassword})
	if err != nil {
		t.Fatalf("objstore.New: %v", err)
	}
	api, err := miniogo.New(endpoint, &miniogo.Options{Creds: credentials.NewStaticV4(minioUser, minioPassword, "")})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	return client, api
}

// pointerRefused is the server with the write of latest.json failing: a
// process that died between the object of a snapshot and its pointer.
type pointerRefused struct {
	objstore.Client
}

func (p pointerRefused) Put(ctx context.Context, bucket, key string, body []byte, opts objstore.PutOptions) (string, error) {
	if key == state.PointerKey {
		return "", errors.New("the process died")
	}
	return p.Client.Put(ctx, bucket, key, body, opts)
}

// One server for the whole scenario: the buckets of a world with their rules,
// entities and intents written and read back, snapshots with their rotation,
// and latest.json after a crash between the two writes of a snapshot.
func TestTheStoreOfStateOnMinIO(t *testing.T) {
	ctx := context.Background()
	client, api := minioServer(t)

	if err := state.EnsureWorldBuckets(ctx, client, world); err != nil {
		t.Fatalf("EnsureWorldBuckets: %v", err)
	}
	for _, bucket := range []string{objstore.EntitiesBucket(world), objstore.SnapshotsBucket(world)} {
		versioning, err := api.GetBucketVersioning(ctx, bucket)
		if err != nil || versioning.Status != "Enabled" {
			t.Errorf("versioning of %s = %+v, %v; want Enabled", bucket, versioning, err)
		}
		ilm, err := api.GetBucketLifecycle(ctx, bucket)
		if err != nil || len(ilm.Rules) != 1 || int(ilm.Rules[0].NoncurrentVersionExpiration.NoncurrentDays) != 30 {
			t.Errorf("lifecycle of %s = %+v, %v; want a 30 day non-current expiration", bucket, ilm, err)
		}
	}
	if err := state.EnsureWorldBuckets(ctx, client, world); err != nil {
		t.Errorf("EnsureWorldBuckets a second time: %v", err)
	}

	store := state.NewObjectStore(client)
	wolf := entity.New(ref("wolf-alpha", entity.TypeNPC), world, "Альфа-волк", map[string]any{"hp": 10}, testkit.Epoch)
	player := entity.New(ref("player-A", entity.TypePlayer), world, "", map[string]any{"hp": 10}, testkit.Epoch)
	for _, e := range []*entity.Entity{player, wolf} {
		if err := store.PutEntity(ctx, world, e); err != nil {
			t.Fatalf("PutEntity %s: %v", e.ID, err)
		}
	}
	intent := &state.Intent{ProposalID: "prop/round", ProposalEventID: "ev-1", World: world, Cause: "combat",
		Changes: []state.IntentChange{{Ref: wolf.Ref(), FromVersion: 1, ToVersion: 2, AttributesAfter: map[string]any{"hp": 7.0}}}}
	if err := store.PutIntent(ctx, world, intent); err != nil {
		t.Fatalf("PutIntent: %v", err)
	}
	got, err := store.GetEntity(ctx, world, entity.TypeNPC, "wolf-alpha")
	if err != nil || entity.StateHash([]*entity.Entity{got}) != entity.StateHash([]*entity.Entity{wolf}) {
		t.Errorf("GetEntity = %+v, %v; want the wolf as written", got, err)
	}
	listed, err := store.ListEntities(ctx, world)
	if err != nil || !slices.Equal(refsOf(listed), []string{"npc:wolf-alpha", "player:player-A"}) {
		t.Errorf("ListEntities = %v, %v; want the two entities and no intent", refsOf(listed), err)
	}
	intents, err := store.ListIntents(ctx, world)
	if err != nil || len(intents) != 1 || intents[0].ProposalID != "prop/round" {
		t.Errorf("ListIntents = %+v, %v; want the intent of prop/round", intents, err)
	}
	if err := store.DeleteIntent(ctx, world, "prop/round"); err != nil {
		t.Errorf("DeleteIntent: %v", err)
	}
	if intents, _ := store.ListIntents(ctx, world); len(intents) != 0 {
		t.Errorf("%d intents after DeleteIntent", len(intents))
	}

	// Snapshots through the Applier, the way State writes them.
	working := memstore.New()
	seedAll(t, working, entity.New(ref(world, entity.TypeWorld), world, "", map[string]any{"laws_version": "v1"}, testkit.Epoch), player, wolf)
	f := newAppliedOver(t, working, store)
	for range 6 {
		if _, err := f.applier.Snapshot(ctx, state.SnapshotAdmin); err != nil {
			t.Fatalf("Snapshot: %v", err)
		}
		f.clock.Advance(time.Minute)
	}
	refs, err := store.ListSnapshots(ctx, world)
	if err != nil || len(refs) != state.SnapshotsKept || refs[0].Seq != 5 || refs[4].Seq != 1 {
		t.Errorf("ListSnapshots = %+v, %v; want seq 5 down to 1", refs, err)
	}

	crashed := newAppliedOver(t, working, state.NewObjectStore(pointerRefused{client}))
	if _, err := crashed.applier.Snapshot(ctx, state.SnapshotAdmin); err == nil {
		t.Fatal("a snapshot without its pointer succeeded")
	}
	latest, err := store.ReadLatest(ctx, world)
	if err != nil || latest.Snapshot.Seq != 5 {
		t.Fatalf("latest.json after the crash = %+v, %v; want seq 5", latest, err)
	}
	object, err := store.ReadSnapshot(ctx, world, latest.Snapshot.Key)
	if err != nil || entity.StateHash(object.Entities) != latest.Snapshot.StateHash {
		t.Errorf("the snapshot latest.json points at = %+v, %v; want its entities to hash to state_hash", object, err)
	}
	raw, err := client.Get(ctx, objstore.SnapshotsBucket(world), latest.Snapshot.Key)
	if err != nil || latest.Snapshot.SizeBytes == nil || *latest.Snapshot.SizeBytes != int64(len(raw)) {
		t.Errorf("size_bytes %v, the object is %d bytes (%v)", latest.Snapshot.SizeBytes, len(raw), err)
	}
}

// newAppliedOver is an Applier of the world over a working set and a Store, on
// the deterministic sources.
func newAppliedOver(t *testing.T, working *memstore.Store, store state.Store) *fixture {
	t.Helper()
	return fixtureWith(t, state.ApplierConfig{Store: working, Objects: store, WithoutOwnership: true,
		RulesVersion: "0.1", Writer: "core/state@integration:0"})
}

func seedAll(t *testing.T, working *memstore.Store, entities ...*entity.Entity) {
	t.Helper()
	if err := working.Put(world, entities...); err != nil {
		t.Fatal(err)
	}
}
