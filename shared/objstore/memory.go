package objstore

import (
	"context"
	"crypto/md5" //nolint:gosec // the S3 ETag of a single-part object is its md5; it identifies content, it does not protect it
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"multiverse-core.io/shared/clock"
)

// ErrNoBucket is returned for a bucket nobody created. The memory store fails
// the same way the server does, so a unit test written against it does not
// pass while the integration test against MinIO fails on a missing bucket.
var ErrNoBucket = errors.New("objstore: bucket does not exist")

// Memory is the in-process object store: the unit tests of every context run
// against it, and so does a process started with the memory bus. It applies no
// bucket rule — Capabilities reports neither versioning nor lifecycle, and
// EnsureBucket records the options it was given without acting on them.
type Memory struct {
	mu      sync.RWMutex
	clock   clock.Clock
	buckets map[string]*memoryBucket
}

type memoryBucket struct {
	opts    BucketOptions
	objects map[string]memoryObject
}

type memoryObject struct {
	body []byte
	info ObjectInfo
}

// NewMemory returns an empty memory store on the wall clock.
func NewMemory() *Memory { return NewMemoryWithClock(clock.Real{}) }

// NewMemoryWithClock returns an empty memory store stamping LastModified from
// c, so that a replayed run produces the same metadata twice.
func NewMemoryWithClock(c clock.Clock) *Memory {
	return &Memory{clock: c, buckets: make(map[string]*memoryBucket)}
}

// Capabilities reports a store with neither versioning nor lifecycle.
func (m *Memory) Capabilities() Capabilities { return Capabilities{} }

// EnsureBucket creates the bucket if it is missing and remembers opts. The
// options are not applied: nothing expires in memory, and a test that needs to
// see the rules of a bucket reads them with BucketOptionsOf.
func (m *Memory) EnsureBucket(_ context.Context, bucket string, opts BucketOptions) error {
	if err := validBucket(bucket); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.buckets[bucket]; ok {
		b.opts = opts
		return nil
	}
	m.buckets[bucket] = &memoryBucket{opts: opts, objects: make(map[string]memoryObject)}
	return nil
}

// BucketOptionsOf returns the options EnsureBucket recorded for a bucket.
func (m *Memory) BucketOptionsOf(bucket string) (BucketOptions, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.buckets[bucket]
	if !ok {
		return BucketOptions{}, false
	}
	return b.opts, true
}

// Put stores a copy of body, replacing whatever the key held.
func (m *Memory) Put(_ context.Context, bucket, key string, body []byte, _ PutOptions) (string, error) {
	if err := validKey(bucket, key); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.buckets[bucket]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNoBucket, bucket)
	}
	stored := make([]byte, len(body))
	copy(stored, body)
	etag := etagOf(stored)
	b.objects[key] = memoryObject{
		body: stored,
		info: ObjectInfo{Key: key, Size: int64(len(stored)), LastModified: m.clock.Now().UTC(), ETag: etag},
	}
	return etag, nil
}

// Get returns a copy of the object, or ErrNotFound.
func (m *Memory) Get(_ context.Context, bucket, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	obj, err := m.object(bucket, key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(obj.body))
	copy(out, obj.body)
	return out, nil
}

// Stat returns the metadata of the object, or ErrNotFound.
func (m *Memory) Stat(_ context.Context, bucket, key string) (ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	obj, err := m.object(bucket, key)
	if err != nil {
		return ObjectInfo{}, err
	}
	return obj.info, nil
}

// List returns the objects of the prefix, sorted by key.
func (m *Memory) List(_ context.Context, bucket, prefix string) ([]ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.buckets[bucket]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoBucket, bucket)
	}
	out := make([]ObjectInfo, 0, len(b.objects))
	for key, obj := range b.objects {
		if strings.HasPrefix(key, prefix) {
			out = append(out, obj.info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Delete removes the object and reports nil for a key that is not there: the
// server answers 204 either way, and a memory store that returned ErrNotFound
// would make a unit test pass where the same code against MinIO takes the
// other branch. A bucket nobody created is still ErrNoBucket.
func (m *Memory) Delete(_ context.Context, bucket, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.buckets[bucket]
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoBucket, bucket)
	}
	delete(b.objects, key)
	return nil
}

// object resolves a key under the lock held by the caller.
func (m *Memory) object(bucket, key string) (memoryObject, error) {
	b, ok := m.buckets[bucket]
	if !ok {
		return memoryObject{}, fmt.Errorf("%w: %s", ErrNoBucket, bucket)
	}
	obj, ok := b.objects[key]
	if !ok {
		return memoryObject{}, fmt.Errorf("%w: %s/%s", ErrNotFound, bucket, key)
	}
	return obj, nil
}

// etagOf is the S3 ETag of an object written in one part.
func etagOf(body []byte) string {
	sum := md5.Sum(body) //nolint:gosec // content identity, not a security primitive
	return hex.EncodeToString(sum[:])
}

func validBucket(bucket string) error {
	if strings.TrimSpace(bucket) == "" {
		return errors.New("objstore: empty bucket name")
	}
	return nil
}

func validKey(bucket, key string) error {
	if err := validBucket(bucket); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("objstore: empty object key")
	}
	return nil
}

// Memory implements the whole contract; the assignment says so at compile time
// rather than at the first call from a context.
var _ Client = (*Memory)(nil)
