package readmodel_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit"
)

func rejected(t *testing.T, id, correlation, proposal string) eventbus.Event {
	t.Helper()
	return event(t, id, readmodel.TypeUpdateRejected, correlation, map[string]any{
		"proposal_id": proposal, "reason": "version_conflict",
	})
}

func TestAwaitFactReturnsTheFactOnItsWay(t *testing.T) {
	m := readmodel.New(readmodel.Config{Timers: clock.RealTimers{}})
	type answer struct {
		ev  eventbus.Event
		err error
	}
	got := make(chan answer, 1)
	go func() {
		ev, err := m.AwaitFact(context.Background(), "create-A", readmodel.DefaultFactWait)
		got <- answer{ev, err}
	}()
	// The fact is applied once the wait is registered: the goroutine above may
	// not have got that far yet.
	for readmodel.PendingWaiters(m) == 0 {
		time.Sleep(time.Millisecond)
	}
	fact := created(t, "player-A", entity.TypePlayer, "create-A", playerAttrs("player-A"))
	mustApply(t, m, fact)
	a := <-got
	if a.err != nil || a.ev.ID != fact.ID {
		t.Errorf("AwaitFact = %s %v, want the fact", a.ev.ID, a.err)
	}
	if n := readmodel.PendingWaiters(m); n != 0 {
		t.Errorf("%d waits left registered", n)
	}
}

// Expect arms the wait before the proposal goes out: a fact that arrives
// before Wait is called is not lost.
func TestAFactBeforeWaitIsNotLost(t *testing.T) {
	m, _ := newModel(t)
	wait := m.Expect("create-A", time.Second, "create-A")
	fact := created(t, "player-A", entity.TypePlayer, "create-A", playerAttrs("player-A"))
	mustApply(t, m, fact)
	if ev, err := wait.Wait(context.Background()); err != nil || ev.ID != fact.ID {
		t.Errorf("Wait = %s %v, want the fact", ev.ID, err)
	}
}

func TestAWaitEndsAtItsDeadline(t *testing.T) {
	m, manual := newModel(t)
	wait := m.Expect("create-A", readmodel.DefaultFactWait, "create-A")
	manual.Advance(readmodel.DefaultFactWait - time.Millisecond)
	result := waitAsync(wait)
	select {
	case <-result:
		t.Fatal("the wait ended before its deadline")
	case <-testkit.After(20 * time.Millisecond):
	}
	manual.Advance(time.Millisecond)
	if err := (<-result).err; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Wait = %v, want context.DeadlineExceeded", err)
	}
	if n := readmodel.PendingWaiters(m); n != 0 {
		t.Errorf("%d waits left registered after the deadline", n)
	}
	// A fact after the deadline has nobody to wake.
	mustApply(t, m, created(t, "player-A", entity.TypePlayer, "create-A", playerAttrs("player-A")))
}

type waitResult struct {
	ev  eventbus.Event
	err error
}

func waitAsync(w *readmodel.Waiter) <-chan waitResult {
	out := make(chan waitResult, 1)
	go func() {
		ev, err := w.Wait(context.Background())
		out <- waitResult{ev, err}
	}()
	return out
}

func TestParallelWaitsOfDifferentCorrelationsDoNotMeet(t *testing.T) {
	m, manual := newModel(t)
	const n = 40
	waits := make([]*readmodel.Waiter, n)
	for i := range waits {
		cid := fmt.Sprintf("create-%d", i)
		waits[i] = m.Expect(cid, time.Second, cid)
	}
	var wg sync.WaitGroup
	results := make([]waitResult, n)
	for i, w := range waits {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ev, err := w.Wait(context.Background())
			results[i] = waitResult{ev, err}
		}()
	}
	// Every correlation but the last gets its own fact, in reverse order and
	// from several goroutines at once.
	var appliers sync.WaitGroup
	for i := n - 2; i >= 0; i-- {
		appliers.Add(1)
		go func() {
			defer appliers.Done()
			id := fmt.Sprintf("player-%d", i)
			if _, err := m.Apply(created(t, id, entity.TypePlayer, fmt.Sprintf("create-%d", i), playerAttrs(id))); err != nil {
				t.Error(err)
			}
		}()
	}
	appliers.Wait()
	for readmodel.PendingWaiters(m) > 1 {
		time.Sleep(time.Millisecond)
	}
	manual.Advance(time.Second)
	wg.Wait()
	for i, r := range results[:n-1] {
		if r.err != nil || r.ev.ID != fmt.Sprintf("created-player-%d", i) {
			t.Errorf("wait %d got %s %v", i, r.ev.ID, r.err)
		}
	}
	if last := results[n-1]; !errors.Is(last.err, context.DeadlineExceeded) {
		t.Errorf("the wait without a fact got %s %v, want its deadline", last.ev.ID, last.err)
	}
	if left := readmodel.PendingWaiters(m); left != 0 {
		t.Errorf("%d waits left registered", left)
	}
}

// A rejection ends the wait of its own proposal only: another package refused
// under the same correlation says nothing about this one (C-05 v1.4 p. 1а).
func TestARejectionEndsTheWaitOfItsProposalOnly(t *testing.T) {
	m, manual := newModel(t)
	wait := m.Expect("create-A", time.Second, "create-A")
	mustApply(t, m, rejected(t, "r-other", "create-A", "another-package"))
	manual.Advance(time.Second / 2)
	mustApply(t, m, rejected(t, "r-own", "create-A", "create-A"))
	ev, err := wait.Wait(context.Background())
	if !errors.Is(err, readmodel.ErrRejected) || ev.ID != "r-own" {
		t.Errorf("Wait = %s %v, want the rejection of its own proposal", ev.ID, err)
	}
}

func TestACancelledContextEndsTheWait(t *testing.T) {
	m, _ := newModel(t)
	ctx, cancel := context.WithCancel(context.Background())
	wait := m.Expect("create-A", time.Minute, "create-A")
	cancel()
	if _, err := wait.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Wait = %v, want context.Canceled", err)
	}
	abandoned := m.Expect("create-B", time.Minute)
	abandoned.Cancel()
	abandoned.Cancel()
	if n := readmodel.PendingWaiters(m); n != 0 {
		t.Errorf("%d waits left registered", n)
	}
	if _, err := m.AwaitFact(context.Background(), "create-C", 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("AwaitFact without a timeout = %v, want the deadline at once", err)
	}
}

// A waiter is used once. A second Wait, or a Wait after Cancel, returns at
// once instead of blocking on a timer that was stopped; a waiter whose
// publication failed is cancelled and leaves nothing registered (review #1,
// N-2).
func TestAWaiterIsUsedOnce(t *testing.T) {
	m, _ := newModel(t)
	wait := m.Expect("create-A", time.Minute, "create-A")
	mustApply(t, m, created(t, "player-A", entity.TypePlayer, "create-A", playerAttrs("player-A")))
	if _, err := wait.Wait(context.Background()); err != nil {
		t.Fatalf("the first Wait = %v", err)
	}
	cancelled := m.Expect("create-B", time.Minute, "create-B")
	if n := readmodel.PendingWaiters(m); n != 1 {
		t.Fatalf("%d waits registered, want the one before its publication", n)
	}
	// The publication failed.
	cancelled.Cancel()
	if n := readmodel.PendingWaiters(m); n != 0 {
		t.Errorf("%d waits left registered after Cancel", n)
	}
	for name, w := range map[string]*readmodel.Waiter{"waited": wait, "cancelled": cancelled} {
		result := waitAsync(w)
		select {
		case r := <-result:
			if !errors.Is(r.err, readmodel.ErrWaiterUsed) {
				t.Errorf("%s: Wait again = %v, want ErrWaiterUsed", name, r.err)
			}
		case <-testkit.After(5 * time.Second):
			t.Fatalf("%s: Wait again blocks", name)
		}
	}
}
