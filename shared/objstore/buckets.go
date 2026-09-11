package objstore

import "strings"

// The bucket layout of the platform (infrastructure.md §5.2, ADR-011).
const (
	entitiesPrefix  = "entities-"
	snapshotsPrefix = "snapshots-"
	promptsPrefix   = "prompts-"

	// OpsArtifacts holds what the operator and CI produce — recordings,
	// reports, exports. It carries no rule: an artifact is deleted when
	// somebody decides to, not on a schedule.
	OpsArtifacts = "ops-artifacts"

	// retentionDays is the single retention of the platform: how long a
	// non-current version of an entity or a snapshot survives, and how long a
	// stored prompt does (SEC-22 lowered the prompts from 90 to 30).
	retentionDays = 30
)

// EntitiesBucket holds one object per entity of a world (ADR-011).
func EntitiesBucket(worldID string) string { return entitiesPrefix + worldID }

// SnapshotsBucket holds the rotated snapshots of a world and its latest.json.
func SnapshotsBucket(worldID string) string { return snapshotsPrefix + worldID }

// PromptsBucket holds the full LLM prompts of a world, written only while
// MV_LLM_STORE_PROMPTS is on. It is not versioned: keeping old versions of
// prompts around would outlive the retention that is the point of the bucket.
func PromptsBucket(worldID string) string { return promptsPrefix + worldID }

// BucketOptionsFor is the one table of bucket rules. EnsureBucket, mvctl world
// init and build/minio-init.sh all read the rules of a bucket from here, so
// the bucket created by the init container and the one created by the platform
// cannot drift apart.
//
// A name outside the layout gets no rules: an unknown bucket is somebody's
// scratch space, not something to start expiring objects in.
func BucketOptionsFor(bucket string) BucketOptions {
	switch {
	case strings.HasPrefix(bucket, entitiesPrefix), strings.HasPrefix(bucket, snapshotsPrefix):
		return BucketOptions{Versioned: true, NoncurrentExpireDays: retentionDays}
	case strings.HasPrefix(bucket, promptsPrefix):
		return BucketOptions{ExpireDays: retentionDays}
	default:
		return BucketOptions{}
	}
}
