package eventbus

import (
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestDedupLetsAnEventThroughOnlyOnce(t *testing.T) {
	d := NewDedup(4)

	if d.Seen("ev-1") {
		t.Error("the first delivery is reported as a duplicate")
	}
	if !d.Seen("ev-1") {
		t.Error("the redelivery of ev-1 is not reported as a duplicate")
	}
	if d.Seen("ev-2") {
		t.Error("a different event is reported as a duplicate")
	}
	if got := d.Len(); got != 2 {
		t.Errorf("len = %d, want 2", got)
	}
}

func TestDedupForgetsTheOldestBeyondItsCapacity(t *testing.T) {
	d := NewDedup(3)

	for _, id := range []string{"ev-1", "ev-2", "ev-3", "ev-4"} {
		d.Seen(id)
	}

	if d.Len() != 3 {
		t.Errorf("len = %d, want the capacity 3", d.Len())
	}
	if d.Seen("ev-1") {
		t.Error("ev-1 survived beyond the window")
	}
	if !d.Seen("ev-4") {
		t.Error("ev-4 was evicted although it is the newest")
	}
}

func TestDedupKeepsWhatIsBeingRedelivered(t *testing.T) {
	d := NewDedup(3)
	for _, id := range []string{"ev-1", "ev-2", "ev-3"} {
		d.Seen(id)
	}

	// Touching ev-1 makes it the most recent, so the next insert evicts ev-2.
	d.Seen("ev-1")
	d.Seen("ev-4")

	if d.Seen("ev-2") {
		t.Error("ev-2 survived although it was the least recently seen")
	}
	if !d.Seen("ev-1") {
		t.Error("ev-1 was evicted although it had just been seen again")
	}
}

func TestDedupIgnoresAnEmptyIdentifier(t *testing.T) {
	d := NewDedup(4)

	for i := range 2 {
		if d.Seen("") {
			t.Errorf("call %d: an empty id is remembered, which would make every such event a duplicate", i)
		}
	}
	if d.Len() != 0 {
		t.Errorf("len = %d, want 0", d.Len())
	}
}

func TestDedupDefaultsToTenThousand(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		if got := NewDedup(capacity).Capacity(); got != DefaultDedupCapacity {
			t.Errorf("NewDedup(%d).Capacity() = %d, want %d", capacity, got, DefaultDedupCapacity)
		}
	}
}

func TestDedupSurvivesASnapshot(t *testing.T) {
	d := NewDedup(3)
	for _, id := range []string{"ev-1", "ev-2", "ev-3"} {
		d.Seen(id)
	}

	ids := d.IDs()
	if strings.Join(ids, ",") != "ev-1,ev-2,ev-3" {
		t.Fatalf("ids = %v, want oldest first", ids)
	}

	restored := NewDedup(3)
	restored.Restore(ids)

	for _, id := range ids {
		if !restored.Seen(id) {
			t.Errorf("%s is not remembered after a restart: the work would be redone", id)
		}
	}
	// The restored window keeps the eviction order of the original.
	restored.Restore(ids)
	restored.Seen("ev-4")
	if restored.Seen("ev-1") {
		t.Error("the restored window evicted the wrong entry")
	}
}

func TestDedupRestoreTrimsToCapacity(t *testing.T) {
	d := NewDedup(2)
	d.Restore([]string{"ev-1", "ev-2", "ev-3", ""})

	if d.Len() != 2 {
		t.Errorf("len = %d, want the capacity 2", d.Len())
	}
	if d.Seen("ev-1") {
		t.Error("the oldest entry of an oversized snapshot was kept")
	}
}

func TestDedupIsSafeForConcurrentConsumers(t *testing.T) {
	d := NewDedup(64)

	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 50 {
				d.Seen("ev-" + strconv.Itoa(worker) + "-" + strconv.Itoa(i))
			}
		}()
	}
	wg.Wait()

	if got := d.Len(); got != 64 {
		t.Errorf("len = %d, want the capacity 64", got)
	}
}
