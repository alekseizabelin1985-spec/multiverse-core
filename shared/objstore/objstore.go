// Package objstore is the object store of the platform behind the smallest
// interface that covers what the platform actually does with S3: write a whole
// object, read it back, look at it, list a prefix, delete it, and make sure a
// bucket exists with the rules it needs (ADR-021 p. 2).
//
// What is deliberately absent: multipart uploads, presigned URLs, object lock
// and notifications. Nothing in the platform uses them, and every method the
// interface does not carry is one fewer thing to reimplement when the server
// is replaced (ADR-021 p. 4).
//
// No path depends on bucket versioning either (ADR-021 p. 3): a snapshot is
// rolled back from the rotation of snapshots-{world} and the history of an
// entity lives inside the entity object. Versioning is a safety net that
// EnsureBucket switches on when the server supports it, and its absence is a
// warning in /health.store rather than an error.
package objstore

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by Get, Stat and Delete for an object that is not
// there. It is not a network error: the retries of the minio client do not
// apply to it.
var ErrNotFound = errors.New("objstore: object not found")

// ObjectInfo is what the store knows about an object without reading it.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
}

// PutOptions carries the metadata written with the object.
type PutOptions struct {
	// ContentType defaults to application/octet-stream when empty.
	ContentType string
}

// BucketOptions is the rule set of a bucket, applied by EnsureBucket when it
// creates one (infrastructure.md §5.2). The values of every bucket of the
// platform are in BucketOptionsFor; nothing else decides them.
type BucketOptions struct {
	// Versioned keeps non-current versions of an object.
	Versioned bool
	// NoncurrentExpireDays expires non-current versions after N days. It is
	// meaningful only on a versioned bucket.
	NoncurrentExpireDays int
	// ExpireDays expires the current version of an object after N days. It is
	// how prompts-{world} keeps nothing older than a month (SEC-22, T-13).
	ExpireDays int
}

// Capabilities says which rules of BucketOptions the server behind the client
// can actually apply. A client that cannot version reports it instead of
// failing: the platform runs on the store either way.
type Capabilities struct {
	Versioning bool
	Lifecycle  bool
}

// Client is the whole object store contract of the platform.
type Client interface {
	// Put writes body as the whole object, replacing whatever was there.
	Put(ctx context.Context, bucket, key string, body []byte, opts PutOptions) (etag string, err error)
	// Get returns the object, or ErrNotFound.
	Get(ctx context.Context, bucket, key string) ([]byte, error)
	// Stat returns the metadata of the object, or ErrNotFound.
	Stat(ctx context.Context, bucket, key string) (ObjectInfo, error)
	// List returns the objects whose key starts with prefix, sorted by key.
	List(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error)
	// Delete removes the object. It is idempotent: removing a key that is
	// not there is not an error, because S3 DeleteObject answers 204 either
	// way and no implementation can tell the difference without a second
	// round-trip. A caller that has to know whether the object existed calls
	// Stat first and accepts the race.
	Delete(ctx context.Context, bucket, key string) error
	// EnsureBucket creates the bucket if it is missing and applies opts to it.
	// It is idempotent, and a capability the server lacks is skipped rather
	// than reported as a failure (ADR-021 p. 2).
	EnsureBucket(ctx context.Context, bucket string, opts BucketOptions) error
	// Capabilities reports what the server behind this client can do.
	Capabilities() Capabilities
}
