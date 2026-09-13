package tools_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"multiverse-core.io/shared/agent/tools"
	"multiverse-core.io/shared/clock"
)

var epoch = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// slowTool takes its time on the manual clock.
type slowTool struct {
	clock *clock.Manual
	took  time.Duration
	err   error
}

func (s slowTool) Name() string                   { return "slow" }
func (s slowTool) Description() string            { return "advances the manual clock" }
func (s slowTool) Schema() map[string]interface{} { return nil }
func (s slowTool) Execute(context.Context, map[string]interface{}) (interface{}, error) {
	s.clock.Advance(s.took)
	return "ok", s.err
}

// The latency of a call is measured on the injected clock.
func TestExecuteMeasuresLatencyOnTheInjectedClock(t *testing.T) {
	m := clock.NewManual(epoch)
	r := tools.NewToolRegistry(m)
	if err := r.Register(slowTool{clock: m, took: 250 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := r.Execute(context.Background(), "slow", nil); err != nil {
			t.Fatal(err)
		}
	}
	stats := r.GetStats()
	if stats["avg_latency_ms"] != int64(250) || stats["total_calls"] != int64(2) {
		t.Fatalf("stats = %v, want 2 calls of 250 ms", stats)
	}
}

func TestExecuteCountsErrors(t *testing.T) {
	m := clock.NewManual(epoch)
	r := tools.NewToolRegistry(m)
	boom := errors.New("boom")
	if err := r.Register(slowTool{clock: m, err: boom}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Execute(context.Background(), "slow", nil); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if _, err := r.Execute(context.Background(), "missing", nil); err == nil {
		t.Fatal("an unknown tool must fail")
	}
	if got := r.GetStats()["total_errors"]; got != int64(1) {
		t.Fatalf("total_errors = %v, want 1", got)
	}
}

// The rate-limit window runs on the injected clock: calls beyond the limit fail
// until the manual clock leaves the window.
func TestRateLimitWindowRunsOnTheInjectedClock(t *testing.T) {
	m := clock.NewManual(epoch)
	r := tools.NewToolRegistry(m)
	if err := r.RegisterWithRateLimit(slowTool{clock: m}, 2, time.Minute); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := r.Execute(ctx, "slow", nil); err != nil {
			t.Fatalf("call %d within the limit: %v", i, err)
		}
	}
	if _, err := r.Execute(ctx, "slow", nil); err == nil {
		t.Fatal("the third call within a minute must be refused")
	}
	m.Advance(time.Minute + time.Second)
	if _, err := r.Execute(ctx, "slow", nil); err != nil {
		t.Fatalf("after the window the call must pass: %v", err)
	}
}
