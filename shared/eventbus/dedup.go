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
func (d *Dedup) Seen(id string) bool {
	if id == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if el, ok := d.index[id]; ok {
		d.order.MoveToFront(el)
		return true
	}
	d.index[id] = d.order.PushFront(id)
	d.evict()
	return false
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
		if el, ok := d.index[id]; ok {
			d.order.MoveToFront(el)
			continue
		}
		d.index[id] = d.order.PushFront(id)
		d.evict()
	}
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
