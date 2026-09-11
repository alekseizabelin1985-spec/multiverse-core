package eventbus

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestDedupHasDoesNotRemember(t *testing.T) {
	d := NewDedup(4)

	for i := range 2 {
		if d.Has("ev-1") {
			t.Errorf("call %d: Has reports an id nobody added", i)
		}
	}
	if d.Len() != 0 {
		t.Errorf("len = %d, want 0: Has remembered the id", d.Len())
	}
}

func TestDedupAddThenHas(t *testing.T) {
	d := NewDedup(4)

	d.Add("ev-1")
	if !d.Has("ev-1") {
		t.Error("an added id is not in the window")
	}
	if d.Has("ev-2") {
		t.Error("a different id is reported as seen")
	}
	d.Add("ev-1")
	if d.Len() != 1 {
		t.Errorf("len = %d, want 1: adding an id twice took two slots", d.Len())
	}
}

// TestDedupHasDoesNotRefreshRecency: ADR-027 p. 3 says Has leaves the window
// as it is, recency included. A duplicate answered through Has alone is not
// touched; the eviction order changes only through Add or Seen.
func TestDedupHasDoesNotRefreshRecency(t *testing.T) {
	d := NewDedup(3)
	for _, id := range []string{"ev-1", "ev-2", "ev-3"} {
		d.Add(id)
	}

	d.Has("ev-1")
	d.Add("ev-4")

	if d.Has("ev-1") {
		t.Error("ev-1 survived the insert: Has made it more recent")
	}
	if !d.Has("ev-2") {
		t.Error("ev-2 was evicted instead of ev-1")
	}
	if got := strings.Join(d.IDs(), ","); got != "ev-2,ev-3,ev-4" {
		t.Errorf("ids = %s, want ev-2,ev-3,ev-4", got)
	}
}

// TestDedupAddEvictsLikeSeen runs the same history through Add and through
// Seen and wants the same window: the two-step path must not have an eviction
// policy of its own.
func TestDedupAddEvictsLikeSeen(t *testing.T) {
	history := []string{"ev-1", "ev-2", "ev-3", "ev-1", "ev-4", "ev-5"}

	byAdd, bySeen := NewDedup(3), NewDedup(3)
	for _, id := range history {
		byAdd.Add(id)
		bySeen.Seen(id)
	}

	if a, s := strings.Join(byAdd.IDs(), ","), strings.Join(bySeen.IDs(), ","); a != s {
		t.Errorf("Add leaves %s, Seen leaves %s", a, s)
	}
	// ev-1 was touched again before ev-4, so ev-2 and ev-3 are the ones gone.
	if got := strings.Join(byAdd.IDs(), ","); got != "ev-1,ev-4,ev-5" {
		t.Errorf("ids = %s, want ev-1,ev-4,ev-5", got)
	}
}

func TestDedupTwoStepIgnoresAnEmptyIdentifier(t *testing.T) {
	d := NewDedup(4)

	d.Add("")
	if d.Has("") {
		t.Error("an empty id is reported as seen, which would make every such event a duplicate")
	}
	if d.Len() != 0 {
		t.Errorf("len = %d, want 0", d.Len())
	}
}

func TestDedupTwoStepAndSeenShareOneWindow(t *testing.T) {
	d := NewDedup(4)

	d.Add("ev-1")
	if !d.Seen("ev-1") {
		t.Error("Seen does not know an id added through Add")
	}
	d.Seen("ev-2")
	if !d.Has("ev-2") {
		t.Error("Has does not know an id recorded through Seen")
	}
}

// TestDedupTwoStepSurvivesASnapshot: the window of a stateful consumer goes
// into its snapshot (C-14 v1.2 (a)), whichever of the two paths filled it.
func TestDedupTwoStepSurvivesASnapshot(t *testing.T) {
	d := NewDedup(3)
	for _, id := range []string{"ev-1", "ev-2", "ev-3"} {
		d.Add(id)
	}

	restored := NewDedup(3)
	restored.Restore(d.IDs())

	for _, id := range []string{"ev-1", "ev-2", "ev-3"} {
		if !restored.Has(id) {
			t.Errorf("%s is not remembered after a restart: the text would be published twice", id)
		}
	}
	restored.Add("ev-4")
	if restored.Has("ev-1") {
		t.Error("the restored window evicted the wrong entry")
	}
}

// TestDedupRemembersOnlyAfterTheSideEffect is the case the two steps exist for
// (review #1 of T-220, Mi-1): the publish fails, the bus redelivers, and the
// event must still be handled; once handled, a redelivery is dropped.
func TestDedupRemembersOnlyAfterTheSideEffect(t *testing.T) {
	d := NewDedup(4)
	publishFails := true
	published := 0

	handle := func(id string) error {
		if d.Has(id) {
			return nil
		}
		if publishFails {
			return errors.New("publish failed")
		}
		published++
		d.Add(id)
		return nil
	}

	if err := handle("ev-1"); err == nil {
		t.Fatal("the failed publish was not reported")
	}
	publishFails = false
	for delivery := range 3 {
		if err := handle("ev-1"); err != nil {
			t.Fatalf("delivery %d: %v", delivery, err)
		}
	}

	if published != 1 {
		t.Errorf("published %d times, want once: the retry after the failure must publish and the repeats after it must not", published)
	}
}

func TestDedupTwoStepIsSafeForConcurrentConsumers(t *testing.T) {
	d := NewDedup(64)

	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 50 {
				id := "ev-" + strconv.Itoa(worker) + "-" + strconv.Itoa(i)
				if !d.Has(id) {
					d.Add(id)
				}
			}
		}()
	}
	wg.Wait()

	if got := d.Len(); got != 64 {
		t.Errorf("len = %d, want the capacity 64", got)
	}
	if got := len(d.IDs()); got != 64 {
		t.Errorf("the list holds %d ids and the index %d: the two went apart", got, d.Len())
	}
}
