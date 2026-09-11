//go:build integration

// The contract test of the object store (ADR-021 p. 4): the same assertions
// the memory implementation satisfies, run against a real server. When the
// server is replaced, this file does not change — only the image does.
//
// It runs the image built by build/minio.Dockerfile, whose tag is MINIO_IMAGE
// in build/versions.env — the one pin of the platform (NFR-071). The name is
// read from that file rather than from the environment: the file is the source
// of truth, and the environment is not readable outside shared/env anyway.
package objstore_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/objstore"
)

const (
	testUser     = "multiverse-test"
	testPassword = "multiverse-test-password"
)

// minioImage returns MINIO_IMAGE from build/versions.env.
func minioImage(t *testing.T) string {
	t.Helper()
	f, err := os.Open("../../build/versions.env")
	if err != nil {
		t.Fatalf("read the pins: %v", err)
	}
	defer func() { _ = f.Close() }()
	entries, err := env.ParseExample(f)
	if err != nil {
		t.Fatalf("parse the pins: %v", err)
	}
	for _, e := range entries {
		if e.Name == "MINIO_IMAGE" && !e.Commented && e.Value != "" {
			return e.Value
		}
	}
	t.Fatal("MINIO_IMAGE is empty in build/versions.env")
	return ""
}

// server starts the pinned MinIO image and returns a client of the package
// plus the raw SDK client the assertions on versioning and ILM need.
func server(t *testing.T) (objstore.Client, *miniogo.Client) {
	t.Helper()
	image := minioImage(t)

	ctx := context.Background()
	container, err := tcminio.Run(ctx, image,
		tcminio.WithUsername(testUser), tcminio.WithPassword(testPassword))
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start %s: %v", image, err)
	}
	endpoint, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	client, err := objstore.New(objstore.Config{
		Endpoint: endpoint, AccessKey: testUser, SecretKey: testPassword,
	})
	if err != nil {
		t.Fatalf("objstore.New: %v", err)
	}
	api, err := miniogo.New(endpoint, &miniogo.Options{
		Creds: credentials.NewStaticV4(testUser, testPassword, ""),
	})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	return client, api
}

func TestMinIORoundTrip(t *testing.T) {
	ctx := context.Background()
	client, _ := server(t)
	bucket := objstore.EntitiesBucket("integration")

	if err := client.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	body := []byte(`{"id":"npc-1"}`)
	etag, err := client.Put(ctx, bucket, "npc-1.json", body, objstore.PutOptions{ContentType: "application/json"})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if etag == "" {
		t.Fatal("Put returned an empty ETag")
	}

	got, err := client.Get(ctx, bucket, "npc-1.json")
	if err != nil || string(got) != string(body) {
		t.Fatalf("Get = %q, %v; want %q", got, err, body)
	}

	info, err := client.Stat(ctx, bucket, "npc-1.json")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Key != "npc-1.json" || info.Size != int64(len(body)) || info.LastModified.IsZero() {
		t.Fatalf("Stat = %+v, want the key, the size and a timestamp", info)
	}

	for _, key := range []string{"other/1.json", "npc-2.json"} {
		if _, err := client.Put(ctx, bucket, key, []byte("{}"), objstore.PutOptions{}); err != nil {
			t.Fatalf("Put %s: %v", key, err)
		}
	}
	listed, err := client.List(ctx, bucket, "npc-")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 || listed[0].Key != "npc-1.json" || listed[1].Key != "npc-2.json" {
		t.Fatalf("List = %+v, want the two npc keys in order", listed)
	}

	if err := client.Delete(ctx, bucket, "npc-1.json"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := client.Get(ctx, bucket, "npc-1.json"); !errors.Is(err, objstore.ErrNotFound) {
		t.Fatalf("Get of a deleted object = %v, want ErrNotFound", err)
	}
	if _, err := client.Stat(ctx, bucket, "never-written"); !errors.Is(err, objstore.ErrNotFound) {
		t.Fatalf("Stat of a missing object = %v, want ErrNotFound", err)
	}
}

// Delete is idempotent on the server: S3 DeleteObject answers 204 for a key
// that was never written. Memory does the same
// (TestMemoryDeleteOfAMissingObjectIsNotAnError); this half is what keeps the
// two from forking again — the divergence found in review T-007 (M-1) was
// invisible precisely because no test asserted the missing-key case on either
// side.
func TestMinIODeleteIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client, _ := server(t)
	bucket := objstore.EntitiesBucket("integration")

	if err := client.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if err := client.Delete(ctx, bucket, "never-written.json"); err != nil {
		t.Fatalf("Delete of a missing object = %v, want nil", err)
	}
	if _, err := client.Put(ctx, bucket, "npc-9.json", []byte("{}"), objstore.PutOptions{}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := client.Delete(ctx, bucket, "npc-9.json"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := client.Delete(ctx, bucket, "npc-9.json"); err != nil {
		t.Fatalf("Delete of a deleted object = %v, want nil", err)
	}
	if _, err := client.Stat(ctx, bucket, "npc-9.json"); !errors.Is(err, objstore.ErrNotFound) {
		t.Fatalf("Stat after Delete = %v, want ErrNotFound", err)
	}
}

// EnsureBucket switches versioning and ILM on by itself (ADR-021 p. 2,
// infrastructure.md §5.2): entities and snapshots are versioned with a 30 day
// non-current expiration, prompts are not versioned and expire after 30 days
// (SEC-22).
func TestMinIOEnsureBucketAppliesTheLayoutRules(t *testing.T) {
	ctx := context.Background()
	client, api := server(t)

	if caps := client.Capabilities(); !caps.Versioning || !caps.Lifecycle {
		t.Fatalf("Capabilities = %+v, want versioning and lifecycle", caps)
	}

	entities := objstore.EntitiesBucket("integration")
	if err := client.EnsureBucket(ctx, entities, objstore.BucketOptionsFor(entities)); err != nil {
		t.Fatalf("EnsureBucket(%s): %v", entities, err)
	}
	versioning, err := api.GetBucketVersioning(ctx, entities)
	if err != nil {
		t.Fatalf("GetBucketVersioning(%s): %v", entities, err)
	}
	if versioning.Status != "Enabled" {
		t.Fatalf("versioning of %s = %q, want Enabled", entities, versioning.Status)
	}
	ilm, err := api.GetBucketLifecycle(ctx, entities)
	if err != nil {
		t.Fatalf("GetBucketLifecycle(%s): %v", entities, err)
	}
	if len(ilm.Rules) != 1 || int(ilm.Rules[0].NoncurrentVersionExpiration.NoncurrentDays) != 30 {
		t.Fatalf("lifecycle of %s = %+v, want a 30 day non-current expiration", entities, ilm.Rules)
	}

	prompts := objstore.PromptsBucket("integration")
	if err := client.EnsureBucket(ctx, prompts, objstore.BucketOptionsFor(prompts)); err != nil {
		t.Fatalf("EnsureBucket(%s): %v", prompts, err)
	}
	versioning, err = api.GetBucketVersioning(ctx, prompts)
	if err != nil {
		t.Fatalf("GetBucketVersioning(%s): %v", prompts, err)
	}
	if versioning.Status == "Enabled" {
		t.Fatalf("versioning of %s is enabled; SEC-22 wants it off", prompts)
	}
	ilm, err = api.GetBucketLifecycle(ctx, prompts)
	if err != nil {
		t.Fatalf("GetBucketLifecycle(%s): %v", prompts, err)
	}
	if len(ilm.Rules) != 1 || int(ilm.Rules[0].Expiration.Days) != 30 {
		t.Fatalf("lifecycle of %s = %+v, want a 30 day expiration", prompts, ilm.Rules)
	}

	// The bucket without rules gets none: an operator artifact is deleted when
	// somebody decides to.
	if err := client.EnsureBucket(ctx, objstore.OpsArtifacts, objstore.BucketOptionsFor(objstore.OpsArtifacts)); err != nil {
		t.Fatalf("EnsureBucket(%s): %v", objstore.OpsArtifacts, err)
	}
	if _, err := api.GetBucketLifecycle(ctx, objstore.OpsArtifacts); err == nil {
		t.Fatalf("%s carries a lifecycle configuration, want none", objstore.OpsArtifacts)
	}

	// Running it again is what `mvctl world init` does on every start.
	if err := client.EnsureBucket(ctx, entities, objstore.BucketOptionsFor(entities)); err != nil {
		t.Fatalf("EnsureBucket twice: %v", err)
	}
}

func TestMinIOReportsAMissingBucket(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, _ := server(t)

	_, err := client.Get(ctx, "never-created-bucket", "k")
	if !errors.Is(err, objstore.ErrNoBucket) && !strings.Contains(err.Error(), "NoSuchBucket") {
		t.Fatalf("Get on a missing bucket = %v, want ErrNoBucket", err)
	}
}
