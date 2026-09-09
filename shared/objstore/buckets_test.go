package objstore_test

import (
	"testing"

	"multiverse-core.io/shared/objstore"
)

func TestBucketNames(t *testing.T) {
	const world = "dark-forest"
	cases := []struct {
		got  string
		want string
	}{
		{objstore.EntitiesBucket(world), "entities-dark-forest"},
		{objstore.SnapshotsBucket(world), "snapshots-dark-forest"},
		{objstore.PromptsBucket(world), "prompts-dark-forest"},
		{objstore.OpsArtifacts, "ops-artifacts"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("bucket = %q, want %q", c.got, c.want)
		}
	}
}

// The rules of the layout (infrastructure.md §5.2): entities and snapshots are
// versioned and expire non-current versions after 30 days; prompts are not
// versioned and expire outright after 30 (SEC-22); anything else is left alone.
func TestBucketOptionsFor(t *testing.T) {
	cases := []struct {
		bucket string
		want   objstore.BucketOptions
	}{
		{objstore.EntitiesBucket("w"), objstore.BucketOptions{Versioned: true, NoncurrentExpireDays: 30}},
		{objstore.SnapshotsBucket("w"), objstore.BucketOptions{Versioned: true, NoncurrentExpireDays: 30}},
		{objstore.PromptsBucket("w"), objstore.BucketOptions{ExpireDays: 30}},
		{objstore.OpsArtifacts, objstore.BucketOptions{}},
		{"someones-scratch", objstore.BucketOptions{}},
	}
	for _, c := range cases {
		if got := objstore.BucketOptionsFor(c.bucket); got != c.want {
			t.Errorf("BucketOptionsFor(%q) = %+v, want %+v", c.bucket, got, c.want)
		}
	}
}
