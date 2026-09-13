package agent_test

import (
	"context"
	"testing"
	"time"

	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/clock"
)

var lodEpoch = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

func newLODManager() (*agent.LODManager, *clock.Manual) {
	m := clock.NewManual(lodEpoch)
	return agent.NewLODManager(agent.DefaultLODConfig(), m, m.Timers()), m
}

// DecayLOD measures the time since the last change on the injected clock: the
// test moves that clock, and nothing depends on how long the test runs.
func TestLODDecayUsesTheInjectedClock(t *testing.T) {
	lm, m := newLODManager()
	lm.SetAgentLOD("a", agent.LODFull)
	lm.SetAgentLOD("a", agent.LODBasic)

	m.Advance(agent.DefaultLODConfig().DecayTime)
	lm.DecayLOD("a")
	if got := lm.GetAgentLOD("a"); got != agent.LODBasic {
		t.Fatalf("at exactly DecayTime LOD = %v, want basic (decay needs more than DecayTime)", got)
	}

	m.Advance(time.Second)
	lm.DecayLOD("a")
	if got := lm.GetAgentLOD("a"); got != agent.LODFull {
		t.Fatalf("after DecayTime LOD = %v, want full", got)
	}
}

func TestLODStatsTakeTheChangeTimeFromTheClock(t *testing.T) {
	lm, m := newLODManager()
	lm.SetAgentLOD("a", agent.LODFull)
	m.Advance(90 * time.Second)
	lm.SetAgentLOD("a", agent.LODRuleOnly)

	stats := lm.GetStats()
	if got, want := stats["last_change"], lodEpoch.Add(90*time.Second).Format(time.RFC3339); got != want {
		t.Fatalf("last_change = %v, want %v", got, want)
	}
	if stats["total_downgrades"] != int64(1) {
		t.Fatalf("total_downgrades = %v, want 1", stats["total_downgrades"])
	}
}

// Start checks the agents on every tick of the injected timers.
func TestLODStartTicksOnTheInjectedTimers(t *testing.T) {
	lm, m := newLODManager()
	lm.SetAgentLOD("a", agent.LODFull)

	calls := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		lm.Start(ctx, func() (int, int, time.Duration) {
			select {
			case calls <- struct{}{}:
			default:
			}
			return 100, 0, 0
		})
	}()

	deadline := clock.RealTimers{}.After(5 * time.Second)
	defer deadline.Stop()
	// Start registers its timer in its own goroutine, so the test cannot know
	// when an Advance will reach it: it keeps advancing until the first tick
	// arrives, yielding in between.
	poll := clock.RealTimers{}.Every(time.Millisecond)
	defer poll.Stop()
	for ticked := false; !ticked; {
		m.Advance(agent.DefaultLODConfig().CheckInterval)
		select {
		case <-calls:
			ticked = true
		case <-deadline.C():
			t.Fatal("Start did not tick on the manual timers")
		case <-poll.C():
		}
	}
	cancel()
	select {
	case <-done:
	case <-deadline.C():
		t.Fatal("Start did not return after the context was cancelled")
	}
	if got := lm.GetAgentLOD("a"); got != agent.LODRuleOnly {
		t.Fatalf("after a tick with high player density LOD = %v, want rule-only", got)
	}
}
