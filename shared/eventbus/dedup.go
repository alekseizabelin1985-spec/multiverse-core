package eventbus

import (
	"container/list"
	"sync"
)

// DefaultDedupCapacity is the number of identifiers a consumer remembers by
// default (C-01).
const DefaultDedupCapacity = 10_000

// Dedup is the LRU of event identifiers a consumer keeps to turn the
// at-least-once delivery of the bus into at-most-once processing. Deduplication
// is the duty of the consumer, not of the bus: only the consumer knows whether
// handling an event twice is harmless.
//
// The window is serialised into the consumer snapshot (C-14) with IDs and
// restored with Restore, so that a restart does not replay work that was
// already done.
type Dedup struct {
	mu       sync.Mutex
	capacity int
	order    *list.List               // front = most recently seen
	index    map[string]*list.Element // id -> element in order
}

// NewDedup returns a window of the given capacity; zero or less selects
// DefaultDedupCapacity.
func NewDedup(capacity int) *Dedup {
	if capacity <= 0 {
		capacity = DefaultDedupCapacity
	}
	return &Dedup{
		capacity: capacity,
		order:    list.New(),
		index:    make(map[string]*list.Element, capacity),
	}
}

// Seen records the identifier and reports whether it was already in the
// window. An empty identifier is never remembered and always reports false:
// an envelope without an id is rejected by validation, and remembering it
// would make every such envelope a duplicate of the previous one.
//
// Seen remembers before the event is handled. That is the right order for a
// consumer whose repeat would do harm (the encounter agent would roll the dice
// twice); a consumer whose only side effect is one publish wants Has and Add
// instead, or a failed publish is lost without a trace (C-01 v1.4, ADR-027).
func (d *Dedup) Seen(id string) bool {
	if id == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.remember(id)
}

// Has reports whether the identifier is in the window and leaves the window
// untouched: it neither remembers the identifier nor makes it more recent.
// Together with Add it is the two-step form of Seen, for a consumer that may
// remember an event only once its side effect has succeeded (C-01 v1.4,
// ADR-027 p. 3).
//
// The two steps are not atomic. They rely on the handler not being called
// concurrently for the same event, which both implementations of the bus
// guarantee by delivering a topic one event at a time.
func (d *Dedup) Has(id string) bool {
	if id == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.index[id]
	return ok
}

// Add remembers the identifier, exactly as Seen does: an identifier already in
// the window becomes the most recent, and the oldest one is evicted beyond the
// capacity. An empty identifier is ignored for the reason given on Seen.
func (d *Dedup) Add(id string) {
	if id == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.remember(id)
}

// Len returns the number of identifiers currently remembered.
func (d *Dedup) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.order.Len()
}

// Capacity returns the size of the window.
func (d *Dedup) Capacity() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.capacity
}

// IDs returns the window from the oldest identifier to the newest, the order
// Restore expects.
func (d *Dedup) IDs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	ids := make([]string, 0, d.order.Len())
	for el := d.order.Back(); el != nil; el = el.Prev() {
		ids = append(ids, el.Value.(string))
	}
	return ids
}

// Restore replaces the window with the identifiers of a snapshot, oldest
// first. Identifiers beyond the capacity are dropped from the front.
func (d *Dedup) Restore(ids []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.order.Init()
	d.index = make(map[string]*list.Element, d.capacity)
	for _, id := range ids {
		if id == "" {
			continue
		}
		d.remember(id)
	}
}

// remember makes the identifier the most recent entry of the window and
// reports whether it was there already. Seen, Add and Restore all go through
// it, so the three cannot disagree on eviction. It must be called with d.mu
// held and a non-empty id.
func (d *Dedup) remember(id string) bool {
	if el, ok := d.index[id]; ok {
		d.order.MoveToFront(el)
		return true
	}
	d.index[id] = d.order.PushFront(id)
	d.evict()
	return false
}

// evict must be called with d.mu held.
func (d *Dedup) evict() {
	for d.order.Len() > d.capacity {
		oldest := d.order.Back()
		if oldest == nil {
			return
		}
		d.order.Remove(oldest)
		delete(d.index, oldest.Value.(string))
	}
}
