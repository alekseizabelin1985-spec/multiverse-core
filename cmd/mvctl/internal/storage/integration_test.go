//go:build integration

// `mvctl storage init` against a real server. The memory store answers every
// EnsureBucket the same way, so the thing that can only be learnt here is
// whether the bucket really appears on MinIO and whether a second run of the
// command leaves it alone (the criterion of T-010).
//
// The image is the one build/minio.Dockerfile produces, pinned as MINIO_IMAGE
// in build/versions.env — read from the file rather than from the environment,
// which is not readable outside shared/env.
package storage_test

import (
	"context"
	"os"
	"testing"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	storagecmd "multiverse-core.io/cmd/mvctl/internal/storage"
	sharedenv "multiverse-core.io/shared/env"
	"multiverse-core.io/shared/objstore"
)

const (
	testUser     = "multiverse-test"
	testPassword = "multiverse-test-password"
)

func minioImage(t *testing.T) string {
	t.Helper()
	f, err := os.Open("../../../../build/versions.env")
	if err != nil {
		t.Fatalf("read the pins: %v", err)
	}
	defer func() { _ = f.Close() }()
	entries, err := sharedenv.ParseExample(f)
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

// server starts the pinned image and returns the client of the package plus the
// raw SDK client the assertion on the bucket needs.
func server(t *testing.T) (objstore.Client, *miniogo.Client) {
	t.Helper()
	ctx := context.Background()
	image := minioImage(t)

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
		t.Fatalf("minio-go: %v", err)
	}
	return client, api
}

func TestInitOnMinIO(t *testing.T) {
	ctx := context.Background()
	client, api := server(t)

	report := cli.NewReport("storage init")
	storagecmd.Init(ctx, client, report)
	if !report.OK() {
		t.Fatalf("findings: %v", report.Findings)
	}

	exists, err := api.BucketExists(ctx, objstore.OpsArtifacts)
	if err != nil {
		t.Fatalf("BucketExists: %v", err)
	}
	if !exists {
		t.Fatalf("%s was not created on the server", objstore.OpsArtifacts)
	}

	// A real server can version and expire; the command must not report the
	// degradation notes it prints for the memory store.
	if caps := client.Capabilities(); !caps.Versioning || !caps.Lifecycle {
		t.Errorf("capabilities %+v, want both on a MinIO server", caps)
	}
}

// TestInitOnMinIOIsIdempotent runs the command the way `make up` does: twice,
// on a store that already holds an artefact.
func TestInitOnMinIOIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client, _ := server(t)

	storagecmd.Init(ctx, client, cli.NewReport("storage init"))
	if _, err := client.Put(ctx, objstore.OpsArtifacts, "report.csv",
		[]byte("kept"), objstore.PutOptions{}); err != nil {
		t.Fatalf("write into the bucket: %v", err)
	}

	second := cli.NewReport("storage init")
	storagecmd.Init(ctx, client, second)
	if !second.OK() {
		t.Fatalf("the second run found something: %v", second.Findings)
	}

	body, err := client.Get(ctx, objstore.OpsArtifacts, "report.csv")
	if err != nil {
		t.Fatalf("the object did not survive the second run: %v", err)
	}
	if string(body) != "kept" {
		t.Errorf("the object was rewritten: %q", body)
	}
}
