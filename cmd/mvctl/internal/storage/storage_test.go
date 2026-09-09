package storage_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	storagecmd "multiverse-core.io/cmd/mvctl/internal/storage"
	"multiverse-core.io/shared/objstore"
)

// TestInitCreatesOpsArtifacts is the criterion of the task on the in-memory
// store; the same assertions run against a real MinIO in integration_test.go.
func TestInitCreatesOpsArtifacts(t *testing.T) {
	store := objstore.NewMemory()
	report := cli.NewReport("storage init")

	storagecmd.Init(context.Background(), store, report)

	if !report.OK() {
		t.Fatalf("findings: %v", report.Findings)
	}
	if _, ok := store.BucketOptionsOf(objstore.OpsArtifacts); !ok {
		t.Fatalf("%s was not created", objstore.OpsArtifacts)
	}
}

// TestInitIsIdempotent is what makes the command safe in a start-up script: it
// runs on every `make up`, and a bucket that is already there is not an error.
func TestInitIsIdempotent(t *testing.T) {
	store := objstore.NewMemory()

	first := cli.NewReport("storage init")
	storagecmd.Init(context.Background(), store, first)
	if _, err := store.Put(context.Background(), objstore.OpsArtifacts, "report.csv",
		[]byte("kept"), objstore.PutOptions{}); err != nil {
		t.Fatalf("write into the bucket: %v", err)
	}

	second := cli.NewReport("storage init")
	storagecmd.Init(context.Background(), store, second)
	if !second.OK() {
		t.Fatalf("the second run found something: %v", second.Findings)
	}

	body, err := store.Get(context.Background(), objstore.OpsArtifacts, "report.csv")
	if err != nil {
		t.Fatalf("the object did not survive the second run: %v", err)
	}
	if string(body) != "kept" {
		t.Errorf("the object was rewritten: %q", body)
	}
}

// TestInitAppliesTheBucketTable keeps the command and objstore.BucketOptionsFor
// together: the rules of a bucket are decided in one place, and this is what
// notices when the command starts deciding them itself.
func TestInitAppliesTheBucketTable(t *testing.T) {
	store := objstore.NewMemory()
	report := cli.NewReport("storage init")
	storagecmd.Init(context.Background(), store, report)

	for _, bucket := range storagecmd.PlatformBuckets {
		got, ok := store.BucketOptionsOf(bucket)
		if !ok {
			t.Errorf("%s was not created", bucket)
			continue
		}
		if want := objstore.BucketOptionsFor(bucket); got != want {
			t.Errorf("%s: options %+v, want %+v", bucket, got, want)
		}
	}
}

// refusingStore fails every EnsureBucket, the way a store answers when the
// credentials are wrong or the server is not there.
type refusingStore struct {
	objstore.Client
	err error
}

func (s refusingStore) EnsureBucket(context.Context, string, objstore.BucketOptions) error {
	return s.err
}

func (s refusingStore) Capabilities() objstore.Capabilities {
	return objstore.Capabilities{Versioning: true, Lifecycle: true}
}

func TestInitReportsAStoreThatRefuses(t *testing.T) {
	report := cli.NewReport("storage init")
	storagecmd.Init(context.Background(),
		refusingStore{err: errors.New("access denied")}, report)

	if report.OK() {
		t.Fatal("a store that refused every bucket produced no finding")
	}
	var stdout, stderr bytes.Buffer
	if code := report.Write(&stdout, &stderr, false); code != cli.ExitFindings {
		t.Errorf("exit code %d, want %d", code, cli.ExitFindings)
	}
	if !strings.Contains(stderr.String(), "ops-artifacts: cannot be created: access denied") {
		t.Errorf("the finding does not say what happened: %q", stderr.String())
	}
}

// TestMissingCapabilitiesAreANoteNotAFailure is ADR-021 p. 2 and D-6: a store
// that cannot version is a degradation the platform runs on, and the command
// says so instead of failing.
//
// The buckets of the platform carry no rules, so the note is proven on buckets
// that ask for them — the buckets of a world, which `mvctl world init` creates
// in EPIC-002 through the same Init.
func TestMissingCapabilitiesAreANoteNotAFailure(t *testing.T) {
	withBuckets(t, []string{"entities-w1", "prompts-w1"})

	var stdout, stderr bytes.Buffer
	code := storagecmd.Run([]string{"init", "--store=memory"}, &stdout, &stderr)

	if code != cli.ExitOK {
		t.Fatalf("exit code %d, want %d (stderr: %s)", code, cli.ExitOK, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "bucket entities-w1 ready") {
		t.Errorf("the bucket was not reported: %q", out)
	}
	if !strings.Contains(out, "does not support versioning") {
		t.Errorf("the skipped versioning rule was not mentioned: %q", out)
	}
	if !strings.Contains(out, "does not support lifecycle") {
		t.Errorf("the skipped expiry was not mentioned: %q", out)
	}
}

// TestNoNoteWhenThereIsNothingToSkip is the other half of the same rule
// (Mi-4 of the review of T-010): ops-artifacts asks for neither versioning nor
// expiry (infrastructure.md §5.2), so an in-memory store takes nothing away
// from it and the command has nothing to warn about. A note here would tell an
// operator, and a CI log, that something broke.
func TestNoNoteWhenThereIsNothingToSkip(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := storagecmd.Run([]string{"init", "--store=memory"}, &stdout, &stderr)

	if code != cli.ExitOK {
		t.Fatalf("exit code %d, want %d (stderr: %s)", code, cli.ExitOK, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "bucket ops-artifacts ready (no rules)") {
		t.Errorf("the bucket was not reported: %q", out)
	}
	if strings.Contains(out, "was skipped") {
		t.Errorf("a rule nobody asked for was reported as skipped: %q", out)
	}
}

// withBuckets runs the test against another set of buckets and puts the
// platform ones back afterwards.
func withBuckets(t *testing.T, buckets []string) {
	t.Helper()
	restore := storagecmd.PlatformBuckets
	storagecmd.PlatformBuckets = buckets
	t.Cleanup(func() { storagecmd.PlatformBuckets = restore })
}

func TestJSONForm(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := storagecmd.Run([]string{"init", "--store=memory", "--json"},
		&stdout, &stderr); code != cli.ExitOK {
		t.Fatalf("exit code %d (stderr: %s)", code, stderr.String())
	}
	var report struct {
		Status  string `json:"status"`
		Details struct {
			Buckets      []storagecmd.BucketResult `json:"buckets"`
			Capabilities objstore.Capabilities     `json:"capabilities"`
		} `json:"details"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if report.Status != cli.StatusOK {
		t.Errorf("status %q", report.Status)
	}
	if len(report.Details.Buckets) != 1 || !report.Details.Buckets[0].Ready {
		t.Errorf("buckets %+v", report.Details.Buckets)
	}
}

// TestUnreachableStoreIsAFindingNotAUsageError keeps the two exit codes apart: a
// mistyped flag is the caller's problem (2), an object store that cannot be
// reached is a report (1).
func TestUnreachableStoreIsAFindingNotAUsageError(t *testing.T) {
	t.Setenv("MV_MINIO_ACCESS_KEY", "")
	t.Setenv("MV_MINIO_SECRET_KEY", "")

	var stdout, stderr bytes.Buffer
	code := storagecmd.Run([]string{"init"}, &stdout, &stderr)
	if code != cli.ExitFindings {
		t.Fatalf("exit code %d, want %d (stderr: %s)", code, cli.ExitFindings, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no credentials") {
		t.Errorf("the finding does not say what is missing: %q", stderr.String())
	}
	// The names of the variables are useful; their values are not printed.
	if !strings.Contains(stderr.String(), "MV_MINIO_ACCESS_KEY") {
		t.Errorf("the finding does not name the variable: %q", stderr.String())
	}
}

func TestUsageErrors(t *testing.T) {
	cases := map[string][]string{
		"no subcommand":      nil,
		"unknown subcommand": {"reset"},
		"unknown store":      {"init", "--store=s3"},
		"unknown flag":       {"init", "--force"},
		"stray argument":     {"init", "ops-artifacts"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := storagecmd.Run(args, &stdout, &stderr); code != cli.ExitUsage {
				t.Errorf("exit code %d, want %d (stderr: %s)", code, cli.ExitUsage, stderr.String())
			}
		})
	}
}
