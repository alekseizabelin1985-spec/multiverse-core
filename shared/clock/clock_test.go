package clock_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
)

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestManualNowMovesOnlyOnDemand(t *testing.T) {
	m := clock.NewManual(epoch)
	if got := m.Now(); !got.Equal(epoch) {
		t.Fatalf("Now() = %v, want %v", got, epoch)
	}
	m.Advance(90 * time.Second)
	if got, want := m.Now(), epoch.Add(90*time.Second); !got.Equal(want) {
		t.Fatalf("after Advance Now() = %v, want %v", got, want)
	}
	m.Set(epoch)
	if got := m.Now(); !got.Equal(epoch) {
		t.Fatalf("after Set Now() = %v, want %v", got, epoch)
	}
}

func TestManualTimersAfterFiresOnce(t *testing.T) {
	m := clock.NewManual(epoch)
	timer := m.Timers().After(10 * time.Second)

	m.Advance(9 * time.Second)
	assertNotFired(t, timer, "before the deadline")

	m.Advance(time.Second)
	if got := receive(t, timer); !got.Equal(epoch.Add(10 * time.Second)) {
		t.Fatalf("fired at %v, want %v", got, epoch.Add(10*time.Second))
	}

	m.Advance(time.Hour)
	assertNotFired(t, timer, "after it already fired")
}

func TestManualTimersEveryFiresEachPeriod(t *testing.T) {
	m := clock.NewManual(epoch)
	timer := m.Timers().Every(5 * time.Second)

	for i := 1; i <= 3; i++ {
		m.Advance(5 * time.Second)
		want := epoch.Add(time.Duration(i) * 5 * time.Second)
		if got := receive(t, timer); !got.Equal(want) {
			t.Fatalf("tick %d fired at %v, want %v", i, got, want)
		}
	}
	if !timer.Stop() {
		t.Fatal("Stop() = false on an armed ticker, want true")
	}
	m.Advance(5 * time.Second)
	assertNotFired(t, timer, "after Stop")
}

// A period skipped entirely still yields exactly one tick: the channel has
// room for one value, like time.Ticker.
func TestManualTimersEveryCoalescesSkippedPeriods(t *testing.T) {
	m := clock.NewManual(epoch)
	timer := m.Timers().Every(time.Second)

	m.Advance(10 * time.Second)
	receive(t, timer)
	assertNotFired(t, timer, "the skipped periods must coalesce into one tick")

	m.Advance(time.Second)
	if got := receive(t, timer); !got.Equal(epoch.Add(11 * time.Second)) {
		t.Fatalf("next tick at %v, want %v", got, epoch.Add(11*time.Second))
	}
}

func TestManualSetBackwardsDoesNotFire(t *testing.T) {
	m := clock.NewManual(epoch)
	timer := m.Timers().After(time.Second)

	m.Set(epoch.Add(-time.Hour))
	assertNotFired(t, timer, "after moving the clock backwards")

	m.Set(epoch.Add(time.Second))
	receive(t, timer)
}

func TestManualTimerStopReportsPreviousState(t *testing.T) {
	m := clock.NewManual(epoch)
	timer := m.Timers().After(time.Second)
	if !timer.Stop() {
		t.Fatal("first Stop() = false, want true")
	}
	if timer.Stop() {
		t.Fatal("second Stop() = true, want false")
	}
	m.Advance(time.Hour)
	assertNotFired(t, timer, "after Stop")
}

// The same script must produce the same sequence on every run: two runs of the
// identical program are compared value by value.
func TestManualIsDeterministic(t *testing.T) {
	script := func() []time.Time {
		m := clock.NewManual(epoch)
		timers := m.Timers()
		slow := timers.After(3 * time.Second)
		fast := timers.Every(time.Second)
		var fired []time.Time
		for i := 0; i < 4; i++ {
			m.Advance(time.Second)
			fired = append(fired, drain(fast)...)
			fired = append(fired, drain(slow)...)
		}
		return fired
	}
	first, second := script(), script()
	if len(first) != len(second) {
		t.Fatalf("run lengths differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if !first[i].Equal(second[i]) {
			t.Fatalf("event %d differs: %v vs %v", i, first[i], second[i])
		}
	}
	if len(first) != 5 {
		t.Fatalf("got %d events (%v), want 4 ticks and 1 timeout", len(first), first)
	}
}

func TestRealClockAdvances(t *testing.T) {
	var c clock.Clock = clock.Real{}
	before := c.Now()
	timer := clock.RealTimers{}.After(time.Millisecond)
	<-timer.C()
	if !c.Now().After(before) {
		t.Fatal("Real.Now() did not advance across a timer")
	}
}

func TestRealTickerStopIsIdempotentAndConcurrent(t *testing.T) {
	ticker := clock.RealTimers{}.Every(time.Millisecond)
	if !ticker.Stop() {
		t.Fatal("first Stop() = false, want true for an armed ticker")
	}
	if ticker.Stop() {
		t.Fatal("second Stop() = true, want false")
	}

	// One owner arms the ticker, several goroutines race to stop it: exactly
	// one of them must be told it did the stopping (-race in CI, T-012).
	concurrent := clock.RealTimers{}.Every(time.Millisecond)
	const racers = 8
	var stoppers atomic.Int32
	var wg sync.WaitGroup
	wg.Add(racers)
	for range racers {
		go func() {
			defer wg.Done()
			if concurrent.Stop() {
				stoppers.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := stoppers.Load(); got != 1 {
		t.Fatalf("%d goroutines saw Stop() = true, want exactly 1", got)
	}
}

func receive(t *testing.T, timer clock.Timer) time.Time {
	t.Helper()
	select {
	case at := <-timer.C():
		return at
	default:
		t.Fatal("timer did not fire")
		return time.Time{}
	}
}

func drain(timer clock.Timer) []time.Time {
	var out []time.Time
	for {
		select {
		case at := <-timer.C():
			out = append(out, at)
		default:
			return out
		}
	}
}

func assertNotFired(t *testing.T, timer clock.Timer, when string) {
	t.Helper()
	select {
	case at := <-timer.C():
		t.Fatalf("timer fired at %v %s", at, when)
	default:
	}
}
