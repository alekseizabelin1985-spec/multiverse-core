package objstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/objstore"
)

var epoch = time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

func newMemory(t *testing.T) (*objstore.Memory, *clock.Manual) {
	t.Helper()
	manual := clock.NewManual(epoch)
	return objstore.NewMemoryWithClock(manual), manual
}

func TestMemoryCapabilitiesAreEmpty(t *testing.T) {
	store, _ := newMemory(t)
	if caps := store.Capabilities(); caps.Versioning || caps.Lifecycle {
		t.Fatalf("Capabilities = %+v, want neither versioning nor lifecycle", caps)
	}
}

func TestMemoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, manual := newMemory(t)
	bucket := objstore.EntitiesBucket("dark-forest")

	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	body := []byte(`{"id":"npc-1"}`)
	etag, err := store.Put(ctx, bucket, "npc-1.json", body, objstore.PutOptions{ContentType: "application/json"})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if etag == "" {
		t.Fatal("Put returned an empty ETag")
	}

	got, err := store.Get(ctx, bucket, "npc-1.json")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("Get = %q, want %q", got, body)
	}
	// The store owns its copy: mutating what a caller reads back or wrote must
	// not reach the stored object.
	got[0] = 'X'
	body[1] = 'X'
	again, err := store.Get(ctx, bucket, "npc-1.json")
	if err != nil || string(again) != `{"id":"npc-1"}` {
		t.Fatalf("Get after mutation = %q, %v; want the stored object", again, err)
	}

	info, err := store.Stat(ctx, bucket, "npc-1.json")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	want := objstore.ObjectInfo{Key: "npc-1.json", Size: int64(len(again)), LastModified: epoch, ETag: etag}
	if info != want {
		t.Fatalf("Stat = %+v, want %+v", info, want)
	}

	manual.Advance(time.Minute)
	if _, err := store.Put(ctx, bucket, "npc-1.json", []byte("{}"), objstore.PutOptions{}); err != nil {
		t.Fatalf("Put over an existing key: %v", err)
	}
	info, err = store.Stat(ctx, bucket, "npc-1.json")
	if err != nil {
		t.Fatalf("Stat after overwrite: %v", err)
	}
	if !info.LastModified.Equal(epoch.Add(time.Minute)) || info.Size != 2 {
		t.Fatalf("Stat after overwrite = %+v, want the new size and time", info)
	}
}

func TestMemoryListIsSortedAndFilteredByPrefix(t *testing.T) {
	ctx := context.Background()
	store, _ := newMemory(t)
	bucket := objstore.SnapshotsBucket("dark-forest")
	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	for _, key := range []string{"state/3.json", "state/1.json", "state/2.json", "mechanics/1.json"} {
		if _, err := store.Put(ctx, bucket, key, []byte("{}"), objstore.PutOptions{}); err != nil {
			t.Fatalf("Put %s: %v", key, err)
		}
	}

	listed, err := store.List(ctx, bucket, "state/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var keys []string
	for _, info := range listed {
		keys = append(keys, info.Key)
	}
	if len(keys) != 3 || keys[0] != "state/1.json" || keys[2] != "state/3.json" {
		t.Fatalf("List = %v, want the three state keys in order", keys)
	}

	all, err := store.List(ctx, bucket, "")
	if err != nil || len(all) != 4 {
		t.Fatalf("List of the whole bucket = %d objects, %v; want 4, nil", len(all), err)
	}
	empty, err := store.List(ctx, bucket, "nothing/")
	if err != nil || len(empty) != 0 {
		t.Fatalf("List of an empty prefix = %v, %v; want no objects and no error", empty, err)
	}
}

func TestMemoryDelete(t *testing.T) {
	ctx := context.Background()
	store, _ := newMemory(t)
	bucket := objstore.OpsArtifacts
	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if _, err := store.Put(ctx, bucket, "report.json", []byte("{}"), objstore.PutOptions{}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Delete(ctx, bucket, "report.json"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, bucket, "report.json"); !errors.Is(err, objstore.ErrNotFound) {
		t.Fatalf("Get of a deleted object = %v, want ErrNotFound", err)
	}
}

// Delete is idempotent, the same way S3 is: this assertion is the memory half
// of TestMinIODeleteIsIdempotent, and the two must be changed together or the
// contract has silently forked again (review T-007 M-1).
func TestMemoryDeleteOfAMissingObjectIsNotAnError(t *testing.T) {
	ctx := context.Background()
	store, _ := newMemory(t)
	bucket := objstore.OpsArtifacts
	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if err := store.Delete(ctx, bucket, "never-written.json"); err != nil {
		t.Fatalf("Delete of a missing object = %v, want nil", err)
	}
	if _, err := store.Put(ctx, bucket, "report.json", []byte("{}"), objstore.PutOptions{}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Delete(ctx, bucket, "report.json"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := store.Delete(ctx, bucket, "report.json"); err != nil {
		t.Fatalf("Delete of a deleted object = %v, want nil", err)
	}
}

func TestMemoryReportsMissingObjectAndBucket(t *testing.T) {
	ctx := context.Background()
	store, _ := newMemory(t)
	bucket := objstore.PromptsBucket("dark-forest")

	if _, err := store.Get(ctx, bucket, "k"); !errors.Is(err, objstore.ErrNoBucket) {
		t.Fatalf("Get on a bucket nobody created = %v, want ErrNoBucket", err)
	}
	if _, err := store.Put(ctx, bucket, "k", nil, objstore.PutOptions{}); !errors.Is(err, objstore.ErrNoBucket) {
		t.Fatalf("Put on a bucket nobody created = %v, want ErrNoBucket", err)
	}
	if _, err := store.List(ctx, bucket, ""); !errors.Is(err, objstore.ErrNoBucket) {
		t.Fatalf("List on a bucket nobody created = %v, want ErrNoBucket", err)
	}
	if err := store.Delete(ctx, bucket, "k"); !errors.Is(err, objstore.ErrNoBucket) {
		t.Fatalf("Delete on a bucket nobody created = %v, want ErrNoBucket", err)
	}

	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if _, err := store.Get(ctx, bucket, "missing"); !errors.Is(err, objstore.ErrNotFound) {
		t.Fatalf("Get of a missing object = %v, want ErrNotFound", err)
	}
	if _, err := store.Stat(ctx, bucket, "missing"); !errors.Is(err, objstore.ErrNotFound) {
		t.Fatalf("Stat of a missing object = %v, want ErrNotFound", err)
	}
}

func TestMemoryRejectsEmptyNames(t *testing.T) {
	ctx := context.Background()
	store, _ := newMemory(t)
	if err := store.EnsureBucket(ctx, "  ", objstore.BucketOptions{}); err == nil {
		t.Fatal("EnsureBucket accepts an empty bucket name")
	}
	if err := store.EnsureBucket(ctx, "b", objstore.BucketOptions{}); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if _, err := store.Put(ctx, "b", " ", nil, objstore.PutOptions{}); err == nil {
		t.Fatal("Put accepts an empty key")
	}
}

// EnsureBucket is idempotent and records the rules it was given, so a caller
// can assert on the layout without a server (mvctl storage init, T-010).
func TestMemoryEnsureBucketIsIdempotentAndRecordsOptions(t *testing.T) {
	ctx := context.Background()
	store, _ := newMemory(t)
	bucket := objstore.PromptsBucket("dark-forest")

	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if _, err := store.Put(ctx, bucket, "p1.json", []byte("{}"), objstore.PutOptions{}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatalf("EnsureBucket twice: %v", err)
	}
	if _, err := store.Get(ctx, bucket, "p1.json"); err != nil {
		t.Fatalf("the second EnsureBucket dropped the objects: %v", err)
	}

	opts, ok := store.BucketOptionsOf(bucket)
	if !ok {
		t.Fatal("BucketOptionsOf does not know a bucket it created")
	}
	if opts.Versioned || opts.ExpireDays != 30 {
		t.Fatalf("BucketOptionsOf = %+v, want the prompts rules", opts)
	}
	if _, ok := store.BucketOptionsOf("never-created"); ok {
		t.Fatal("BucketOptionsOf knows a bucket nobody created")
	}
}
