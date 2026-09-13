package replay_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/replay"
	"multiverse-core.io/shared/testkit"
)

var t0 = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

func TestEventClockStartsWhereItIsTold(t *testing.T) {
	if got := replay.NewEventClock(t0).Now(); !got.Equal(t0) {
		t.Errorf("Now = %v, want the start %v", got, t0)
	}
	if got := replay.NewEventClock(time.Time{}).Now(); !got.IsZero() {
		t.Errorf("Now of a clock started at the zero time = %v, want the zero time: something read the wall clock", got)
	}
}

// NFR-061: the clock follows the events and nothing else. It moves forward to
// a later event, ignores an earlier one, and between two observations it does
// not move at all however much real time passes.
func TestEventClockIsMonotonicAndMovesOnlyByEvents(t *testing.T) {
	c := replay.NewEventClock(t0)

	steps := []struct {
		observe time.Time
		want    time.Time
	}{
		{t0.Add(time.Minute), t0.Add(time.Minute)},
		{t0.Add(30 * time.Second), t0.Add(time.Minute)}, // a redelivered older event
		{t0.Add(time.Minute), t0.Add(time.Minute)},
		{time.Time{}, t0.Add(time.Minute)},
		{t0.Add(time.Hour), t0.Add(time.Hour)},
	}
	for i, step := range steps {
		c.Observe(step.observe)
		if got := c.Now(); !got.Equal(step.want) {
			t.Fatalf("step %d: Observe(%v) → Now = %v, want %v", i, step.observe, got, step.want)
		}
	}

	<-testkit.After(20 * time.Millisecond)
	if got := c.Now(); !got.Equal(t0.Add(time.Hour)) {
		t.Errorf("Now = %v after real time passed with no event, want %v", got, t0.Add(time.Hour))
	}
}

// The middleware of every subscription observes concurrently; the clock ends
// at the latest event whatever the interleaving.
func TestEventClockKeepsTheLatestUnderConcurrentObservers(t *testing.T) {
	c := replay.NewEventClock(t0)
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 500 {
				c.Observe(t0.Add(time.Duration((i*7+g*13)%1000) * time.Second))
				_ = c.Now()
			}
		}()
	}
	wg.Wait()
	if got, want := c.Now(), t0.Add(999*time.Second); !got.Equal(want) {
		t.Errorf("Now = %v, want the latest observed %v", got, want)
	}
}

// C-01 v1.9, the DoD of T-458 B1: Advance moves the clock forward, leaves it
// on a time equal to its own, and refuses a time before it without moving it.
func TestEventClockAdvance(t *testing.T) {
	c := replay.NewEventClock(t0)

	if err := c.Advance(t0.Add(time.Minute)); err != nil {
		t.Fatalf("Advance(later) = %v, want nil", err)
	}
	if got := c.Now(); !got.Equal(t0.Add(time.Minute)) {
		t.Fatalf("Now after Advance(later) = %v, want %v", got, t0.Add(time.Minute))
	}

	// The same instant in another location: equal, so the clock is untouched,
	// down to the location it reports.
	same := t0.Add(time.Minute).In(time.FixedZone("UTC+3", 3*3600))
	if err := c.Advance(same); err != nil {
		t.Fatalf("Advance(equal) = %v, want nil", err)
	}
	if got := c.Now(); got != t0.Add(time.Minute) {
		t.Errorf("Now after Advance(equal) = %v, want the clock unchanged at %v", got, t0.Add(time.Minute))
	}

	err := c.Advance(t0)
	if !errors.Is(err, replay.ErrClockBehind) {
		t.Fatalf("Advance(earlier) = %v, want ErrClockBehind", err)
	}
	for _, part := range []string{"2026-09-13T10:00:00Z", "2026-09-13T10:01:00Z"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not name %s: it must name both times", err, part)
		}
	}
	if got := c.Now(); !got.Equal(t0.Add(time.Minute)) {
		t.Errorf("Now after a refused Advance = %v, want the clock unchanged at %v", got, t0.Add(time.Minute))
	}
}

// Advance and the middleware move one clock from different goroutines. The
// clock ends at the latest time anybody gave it; a refused Advance changes
// nothing. The test does not prove the atomicity of compare-and-move: a
// Now-then-Observe Advance has a logical race with the middleware, not a data
// race, so -race does not see it, and since both calls only raise the clock
// its interleavings end linearizable — nothing observable tells it apart. The
// atomicity is held by the one lock in Advance.
func TestEventClockAdvanceAndObserveConcurrently(t *testing.T) {
	c := replay.NewEventClock(t0)
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 1000 {
				at := t0.Add(time.Duration((i*7+g*13)%2000) * time.Second)
				if g%2 == 0 {
					if err := c.Advance(at); err != nil && !errors.Is(err, replay.ErrClockBehind) {
						t.Errorf("Advance: %v", err)
					}
				} else {
					c.Observe(at)
				}
			}
		}()
	}
	wg.Wait()
	if got, want := c.Now(), t0.Add(1999*time.Second); !got.Equal(want) {
		t.Errorf("Now = %v, want the latest time given %v", got, want)
	}
}

// NullTimers never fires, whatever the duration — a zero or negative one
// included, which a real timer fires at once.
func TestNullTimersNeverFire(t *testing.T) {
	var timers replay.NullTimers
	cases := map[string]interface {
		C() <-chan time.Time
		Stop() bool
	}{
		"After(0)":        timers.After(0),
		"After(-1s)":      timers.After(-time.Second),
		"After(1ns)":      timers.After(time.Nanosecond),
		"Every(1ns)":      timers.Every(time.Nanosecond),
		"Every(0)":        timers.Every(0),
		"Every(negative)": timers.Every(-time.Hour),
	}
	deadline := testkit.After(50 * time.Millisecond)
	for name, timer := range cases {
		if timer.C() == nil {
			t.Errorf("%s: C() is nil; a timer of NullTimers has a channel that never sends", name)
		}
		select {
		case at := <-timer.C():
			t.Errorf("%s fired at %v", name, at)
		case <-deadline:
			deadline = closedChan()
		}
	}
}

func TestNullTimerStopReportsWhetherItWasArmed(t *testing.T) {
	var timers replay.NullTimers
	for name, timer := range map[string]interface{ Stop() bool }{
		"After": timers.After(time.Second),
		"Every": timers.Every(time.Second),
	} {
		if !timer.Stop() {
			t.Errorf("%s: first Stop = false, want true: the timer was armed", name)
		}
		if timer.Stop() {
			t.Errorf("%s: second Stop = true, want false: the timer was already stopped", name)
		}
	}
	if a, b := timers.After(time.Second).C(), timers.After(time.Second).C(); a == b {
		t.Error("two timers share one channel")
	}
}

func closedChan() <-chan time.Time {
	ch := make(chan time.Time)
	close(ch)
	return ch
}

// The DoD of T-060: replay reads no wall clock. forbidigo already refuses
// time.Now and the timers of package time here; this test also refuses the
// way round it — the wall-clock implementations of shared/clock — so that an
// EventClock or NullTimers that quietly delegates to them fails in go test,
// not only in the linter.
func TestThePackageReadsNoWallClock(t *testing.T) {
	forbidden := map[string]bool{
		"time.Now": true, "time.After": true, "time.Tick": true, "time.NewTimer": true,
		"time.NewTicker": true, "time.Since": true, "time.Until": true, "time.AfterFunc": true,
		"time.Sleep": true, "clock.Real": true, "clock.RealTimers": true,
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && forbidden[pkg.Name+"."+sel.Sel.Name] {
				t.Errorf("%s: %s.%s reads the wall clock", fset.Position(sel.Pos()), pkg.Name, sel.Sel.Name)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no source file of the package was checked")
	}
}
