package objstore

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"

	"multiverse-core.io/shared/clock"
)

// instantTimers fires every timer at once and records the pauses that were
// asked for, so a retry test asserts on the backoff without waiting for it.
type instantTimers struct{ pauses []time.Duration }

func (t *instantTimers) After(d time.Duration) clock.Timer {
	t.pauses = append(t.pauses, d)
	return firedTimer()
}

func (t *instantTimers) Every(d time.Duration) clock.Timer { return t.After(d) }

func firedTimer() clock.Timer {
	ch := make(chan time.Time, 1)
	ch <- time.Time{}
	return fired(ch)
}

type fired chan time.Time

func (f fired) C() <-chan time.Time { return f }
func (f fired) Stop() bool          { return true }

func TestNewRejectsAnIncompleteConfig(t *testing.T) {
	if _, err := New(Config{AccessKey: "a", SecretKey: "b"}); err == nil {
		t.Fatal("New accepts a configuration without an endpoint")
	}
	_, err := New(Config{Endpoint: "127.0.0.1:9000", AccessKey: "a"})
	if err == nil {
		t.Fatal("New accepts a configuration without credentials")
	}
	if strings.Contains(err.Error(), "\"a\"") {
		t.Fatalf("the error carries a credential: %q", err)
	}
	if !strings.Contains(err.Error(), "MV_MINIO_SECRET_KEY") {
		t.Fatalf("error = %q, want the name of the variable to set", err)
	}
}

func TestNewReportsMinIOCapabilities(t *testing.T) {
	client, err := New(Config{Endpoint: "127.0.0.1:9000", AccessKey: "a", SecretKey: "b"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if caps := client.Capabilities(); !caps.Versioning || !caps.Lifecycle {
		t.Fatalf("Capabilities = %+v, want versioning and lifecycle", caps)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("MV_MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MV_MINIO_ACCESS_KEY", "access")
	t.Setenv("MV_MINIO_SECRET_KEY", "secret")
	t.Setenv("MV_MINIO_USE_SSL", "true")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	want := Config{Endpoint: "minio:9000", AccessKey: "access", SecretKey: "secret", UseSSL: true}
	if cfg != want {
		t.Fatalf("ConfigFromEnv = %+v, want %+v", cfg, want)
	}

	t.Setenv("MV_MINIO_USE_SSL", "maybe")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("ConfigFromEnv accepts a value that is not a boolean")
	} else if strings.Contains(err.Error(), "secret") {
		t.Fatalf("the error carries a credential: %q", err)
	}
}

func TestLifecycleOf(t *testing.T) {
	if cfg := lifecycleOf("ops-artifacts", BucketOptions{}); cfg != nil {
		t.Fatalf("lifecycleOf without rules = %+v, want nil", cfg)
	}

	prompts := lifecycleOf("prompts-w", BucketOptionsFor("prompts-w"))
	if prompts == nil || len(prompts.Rules) != 1 {
		t.Fatalf("lifecycleOf(prompts) = %+v, want one rule", prompts)
	}
	rule := prompts.Rules[0]
	if rule.Status != "Enabled" || int(rule.Expiration.Days) != 30 {
		t.Fatalf("prompts rule = %+v, want an enabled 30 day expiration", rule)
	}
	if int(rule.NoncurrentVersionExpiration.NoncurrentDays) != 0 {
		t.Fatalf("prompts rule = %+v, want no non-current rule on an unversioned bucket", rule)
	}

	entities := lifecycleOf("entities-w", BucketOptionsFor("entities-w"))
	if entities == nil || len(entities.Rules) != 1 {
		t.Fatalf("lifecycleOf(entities) = %+v, want one rule", entities)
	}
	rule = entities.Rules[0]
	if int(rule.NoncurrentVersionExpiration.NoncurrentDays) != 30 || int(rule.Expiration.Days) != 0 {
		t.Fatalf("entities rule = %+v, want only a 30 day non-current expiration", rule)
	}
}

func TestTranslateMapsTheErrorsOfTheServer(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"nil", nil, nil},
		{"missing key", minio.ErrorResponse{Code: "NoSuchKey", StatusCode: http.StatusNotFound}, ErrNotFound},
		{"missing bucket", minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: http.StatusNotFound}, ErrNoBucket},
		{"other 404", minio.ErrorResponse{StatusCode: http.StatusNotFound}, ErrNotFound},
		{"network", errors.New("dial tcp: connection refused"), nil},
	}
	for _, c := range cases {
		got := translate(c.err)
		switch {
		case c.want != nil && !errors.Is(got, c.want):
			t.Errorf("%s: translate = %v, want %v", c.name, got, c.want)
		case c.want == nil && c.err == nil && got != nil:
			t.Errorf("%s: translate = %v, want nil", c.name, got)
		case c.want == nil && c.err != nil && !errors.Is(got, c.err):
			t.Errorf("%s: translate = %v, want the error unchanged", c.name, got)
		}
	}
}

func TestRetryableSeparatesTransientFromFinal(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"network", errors.New("connection refused"), true},
		{"throttled", minio.ErrorResponse{StatusCode: http.StatusTooManyRequests}, true},
		{"server", minio.ErrorResponse{StatusCode: http.StatusInternalServerError}, true},
		{"missing object", ErrNotFound, false},
		{"missing bucket", ErrNoBucket, false},
		{"forbidden", minio.ErrorResponse{StatusCode: http.StatusForbidden}, false},
		{"cancelled", context.Canceled, false},
	}
	for _, c := range cases {
		if got := retryable(c.err); got != c.want {
			t.Errorf("%s: retryable = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRetryRepeatsATransientFailureThreeTimes(t *testing.T) {
	timers := &instantTimers{}
	client := &minioClient{timers: timers}

	calls := 0
	err := client.retry(context.Background(), func() error {
		calls++
		return errors.New("connection refused")
	})
	if err == nil {
		t.Fatal("retry hides a failure that never went away")
	}
	if calls != retryAttempts {
		t.Fatalf("the handler was called %d times, want %d", calls, retryAttempts)
	}
	if len(timers.pauses) != retryAttempts-1 || timers.pauses[0] != retryBackoff[0] {
		t.Fatalf("pauses = %v, want %v", timers.pauses, retryBackoff)
	}
}

func TestRetryStopsOnAFinalAnswer(t *testing.T) {
	client := &minioClient{timers: &instantTimers{}}
	calls := 0
	err := client.retry(context.Background(), func() error {
		calls++
		return minio.ErrorResponse{Code: "NoSuchKey", StatusCode: http.StatusNotFound}
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("retry = %v, want ErrNotFound", err)
	}
	if calls != 1 {
		t.Fatalf("a missing object was asked for %d times, want once", calls)
	}
}

func TestRetrySucceedsAfterATransientFailure(t *testing.T) {
	client := &minioClient{timers: &instantTimers{}}
	calls := 0
	err := client.retry(context.Background(), func() error {
		calls++
		if calls == 1 {
			return errors.New("connection reset")
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("retry = %v after %d calls, want nil after 2", err, calls)
	}
}

func TestRetryStopsWhenTheContextIsDone(t *testing.T) {
	client := &minioClient{timers: &instantTimers{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	err := client.retry(ctx, func() error {
		calls++
		return errors.New("connection refused")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("retry = %v, want the context error", err)
	}
	if calls != 1 {
		t.Fatalf("the handler was called %d times after the context was cancelled, want once", calls)
	}
}
