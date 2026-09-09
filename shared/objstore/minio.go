package objstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
)

// Config is what a client needs to reach the object store.
type Config struct {
	// Endpoint is host:port without a scheme, the way MV_MINIO_ENDPOINT is
	// written.
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	// Timers paces the retries; the wall clock when nil.
	Timers clock.Timers
}

// ConfigFromEnv builds the configuration from the declared manifest. It is the
// only place the store reads the environment.
func ConfigFromEnv() (Config, error) {
	useSSL, err := env.MinIOUseSSL.Bool()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Endpoint:  env.MinIOEndpoint.String(),
		AccessKey: env.MinIOAccessKey.String(),
		SecretKey: env.MinIOSecretKey.String(),
		UseSSL:    useSSL,
	}, nil
}

// retryAttempts is the number of calls of a network operation, the first one
// included: a transient failure of the store is retried, a refusal is not
// (foundation.md §7).
const retryAttempts = 3

// retryBackoff pauses before the second and the third attempt.
var retryBackoff = []time.Duration{100 * time.Millisecond, 500 * time.Millisecond}

// minioClient is the object store over minio-go. It is created by New; a
// context holds it as an objstore.Client and never sees minio-go itself, which
// is what makes ADR-021 p. 4 a change of one package.
type minioClient struct {
	api    *minio.Client
	timers clock.Timers
}

// New returns a client for the MinIO server described by cfg.
func New(cfg Config) (Client, error) {
	switch {
	case cfg.Endpoint == "":
		return nil, fmt.Errorf("objstore: no endpoint (%s)", env.MinIOEndpoint.Name())
	case cfg.AccessKey == "" || cfg.SecretKey == "":
		// The values themselves are never named here: they are secrets.
		return nil, fmt.Errorf("objstore: no credentials (%s, %s)",
			env.MinIOAccessKey.Name(), env.MinIOSecretKey.Name())
	}
	api, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("objstore: connect to %s: %w", cfg.Endpoint, err)
	}
	timers := cfg.Timers
	if timers == nil {
		timers = clock.RealTimers{}
	}
	return &minioClient{api: api, timers: timers}, nil
}

// Capabilities reports what a MinIO server supports: both versioning and
// lifecycle rules.
func (c *minioClient) Capabilities() Capabilities {
	return Capabilities{Versioning: true, Lifecycle: true}
}

// Put writes the whole object in one request.
func (c *minioClient) Put(ctx context.Context, bucket, key string, body []byte, opts PutOptions) (string, error) {
	if err := validKey(bucket, key); err != nil {
		return "", err
	}
	contentType := opts.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	var etag string
	err := c.retry(ctx, func() error {
		info, err := c.api.PutObject(ctx, bucket, key, bytes.NewReader(body), int64(len(body)),
			minio.PutObjectOptions{ContentType: contentType})
		if err != nil {
			return err
		}
		etag = info.ETag
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("objstore: put %s/%s: %w", bucket, key, err)
	}
	return etag, nil
}

// Get reads the whole object.
func (c *minioClient) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	var body []byte
	err := c.retry(ctx, func() error {
		obj, err := c.api.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return err
		}
		defer func() { _ = obj.Close() }()
		// GetObject is lazy: a missing object surfaces here, not above.
		body, err = io.ReadAll(obj)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("objstore: get %s/%s: %w", bucket, key, err)
	}
	return body, nil
}

// Stat returns the metadata of the object.
func (c *minioClient) Stat(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	var info ObjectInfo
	err := c.retry(ctx, func() error {
		stat, err := c.api.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
		if err != nil {
			return err
		}
		info = ObjectInfo{Key: stat.Key, Size: stat.Size, LastModified: stat.LastModified, ETag: stat.ETag}
		return nil
	})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("objstore: stat %s/%s: %w", bucket, key, err)
	}
	return info, nil
}

// List walks the prefix and returns the objects sorted by key. The listing is
// not retried as a whole: it is a stream, and a failure halfway through would
// otherwise replay the part already delivered.
func (c *minioClient) List(ctx context.Context, bucket, prefix string) ([]ObjectInfo, error) {
	var out []ObjectInfo
	for obj := range c.api.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("objstore: list %s/%s: %w", bucket, prefix, translate(obj.Err))
		}
		out = append(out, ObjectInfo{
			Key: obj.Key, Size: obj.Size, LastModified: obj.LastModified, ETag: obj.ETag,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Delete removes the object. S3 answers 204 for a key that is not there, so
// this is idempotent and the memory store matches it (Client.Delete).
func (c *minioClient) Delete(ctx context.Context, bucket, key string) error {
	err := c.retry(ctx, func() error {
		return c.api.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	})
	if err != nil {
		return fmt.Errorf("objstore: delete %s/%s: %w", bucket, key, err)
	}
	return nil
}

// EnsureBucket creates the bucket when it is missing and applies opts to it:
// versioning first, because a lifecycle rule on non-current versions is
// meaningless without it, then the lifecycle configuration. Both are
// idempotent, so a second call on an existing bucket only re-applies the rules
// — which is what makes `mvctl world init` safe to run twice.
func (c *minioClient) EnsureBucket(ctx context.Context, bucket string, opts BucketOptions) error {
	if err := validBucket(bucket); err != nil {
		return err
	}
	err := c.retry(ctx, func() error {
		exists, err := c.api.BucketExists(ctx, bucket)
		if err != nil {
			return err
		}
		if !exists {
			if err := c.api.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				// Another process may have created it in between; that is the
				// outcome we wanted, not a failure.
				if minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("objstore: ensure bucket %s: %w", bucket, err)
	}

	if opts.Versioned {
		if err := c.retry(ctx, func() error { return c.api.EnableVersioning(ctx, bucket) }); err != nil {
			return fmt.Errorf("objstore: enable versioning on %s: %w", bucket, err)
		}
	}
	if cfg := lifecycleOf(bucket, opts); cfg != nil {
		if err := c.retry(ctx, func() error { return c.api.SetBucketLifecycle(ctx, bucket, cfg) }); err != nil {
			return fmt.Errorf("objstore: set lifecycle on %s: %w", bucket, err)
		}
	}
	return nil
}

// lifecycleOf renders the retention of a bucket as an ILM configuration, or
// nil when the bucket has no rule to apply.
func lifecycleOf(bucket string, opts BucketOptions) *lifecycle.Configuration {
	if opts.ExpireDays <= 0 && opts.NoncurrentExpireDays <= 0 {
		return nil
	}
	rule := lifecycle.Rule{
		ID:         "multiverse-retention-" + bucket,
		Status:     "Enabled",
		RuleFilter: lifecycle.Filter{Prefix: ""},
	}
	if opts.ExpireDays > 0 {
		rule.Expiration = lifecycle.Expiration{Days: lifecycle.ExpirationDays(opts.ExpireDays)}
	}
	if opts.NoncurrentExpireDays > 0 {
		rule.NoncurrentVersionExpiration = lifecycle.NoncurrentVersionExpiration{
			NoncurrentDays: lifecycle.ExpirationDays(opts.NoncurrentExpireDays),
		}
	}
	cfg := lifecycle.NewConfiguration()
	cfg.Rules = []lifecycle.Rule{rule}
	return cfg
}

// retry runs op up to retryAttempts times, pausing between attempts. Only a
// transient failure is retried: a missing object or a refused request is the
// answer, and repeating it three times only delays it.
func (c *minioClient) retry(ctx context.Context, op func() error) error {
	var err error
	for attempt := 0; attempt < retryAttempts; attempt++ {
		if attempt > 0 {
			if waitErr := c.wait(ctx, retryBackoff[attempt-1]); waitErr != nil {
				return errors.Join(err, waitErr)
			}
		}
		if err = translate(op()); err == nil {
			return nil
		}
		if !retryable(err) {
			return err
		}
	}
	return err
}

func (c *minioClient) wait(ctx context.Context, pause time.Duration) error {
	// Checked before the timer is armed: a cancelled context ends the retries
	// even when the pause has already elapsed, which a select over two ready
	// channels would decide at random.
	if err := ctx.Err(); err != nil {
		return err
	}
	t := c.timers.After(pause)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C():
		return nil
	}
}

// translate maps the errors of minio-go onto the errors of the package, so
// that a caller matches ErrNotFound instead of reading response codes.
func translate(err error) error {
	if err == nil {
		return nil
	}
	resp := minio.ToErrorResponse(err)
	switch resp.Code {
	case "NoSuchKey", "NoSuchVersion":
		return fmt.Errorf("%w: %s", ErrNotFound, err)
	case "NoSuchBucket":
		return fmt.Errorf("%w: %s", ErrNoBucket, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ErrNotFound, err)
	}
	return err
}

// retryable reports whether the failure may go away on its own: a network
// error carries no response code, and the store answers 429 or 5xx while it is
// busy or restarting.
func retryable(err error) bool {
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNoBucket) || errors.Is(err, context.Canceled) {
		return false
	}
	code := minio.ToErrorResponse(err).StatusCode
	switch {
	case code == 0:
		return true
	case code == http.StatusTooManyRequests:
		return true
	case code >= http.StatusInternalServerError:
		return true
	default:
		return false
	}
}

var _ Client = (*minioClient)(nil)
