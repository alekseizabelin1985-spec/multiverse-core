package actions

import (
	"sync"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/eventbus"
)

// DefaultPendingLimit bounds the batches held in memory when Config does not.
// A batch is at most three events, and an event in memory (a payload of
// map[string]any, the text of say up to 500 characters) takes about 1 to 3
// kilobytes, so the bound is up to a few tens of megabytes; it is reached only
// by a long outage of the bus with clients that never repeat.
const DefaultPendingLimit = 4096

// batch is the events of one decided action, built once: gm.created with
// MV_GM_PATH=legacy, the player.* event and the proposal that goes with it. A
// batch whose publication failed partway is held under its key, so that a
// repeat of the key publishes the same events — the same ids, timestamps and
// bytes — from the one that failed, and consumers drop what they already have
// by its id (C-01, ADR-027). A repeat never builds a second action.
type batch struct {
	events []eventbus.Event
	// action is the index of the player.* event in events.
	action int
	// sent is how many events, from the first, the bus has acknowledged.
	sent int
	turn Turn
	ref  api.TurnRef
	// expires is when the key the batch waits under would have expired had
	// its answer been kept: a repeat after it is a new action.
	expires time.Time
}

type batchKey struct{ playerID, actionKey string }

// pendingBatches holds the batches left half published. It is shared by the
// requests, which take and hold batches under the lock of their player, and
// by the sweeper.
type pendingBatches struct {
	mu      sync.Mutex
	limit   int
	batches map[batchKey]*batch
}

func newPendingBatches(limit int) *pendingBatches {
	return &pendingBatches{limit: limit, batches: make(map[batchKey]*batch)}
}

// take removes and returns the batch held under k; nil when there is none or
// it expired.
func (p *pendingBatches) take(k batchKey, now time.Time) *batch {
	p.mu.Lock()
	defer p.mu.Unlock()
	b := p.batches[k]
	if b == nil {
		return nil
	}
	delete(p.batches, k)
	if !b.expires.After(now) {
		return nil
	}
	return b
}

// hold keeps b under k. At the limit the batch closest to its expiry goes, and
// hold reports its key: a repeat of that key builds its action again.
func (p *pendingBatches) hold(k batchKey, b *batch) (evicted batchKey, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, held := p.batches[k]; !held && len(p.batches) >= p.limit {
		first := true
		for key, other := range p.batches {
			if first || other.expires.Before(p.batches[evicted].expires) {
				evicted, first = key, false
			}
		}
		delete(p.batches, evicted)
		ok = true
	}
	p.batches[k] = b
	return evicted, ok
}

// sweep drops the batches that expired by now and returns how many.
func (p *pendingBatches) sweep(now time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for k, b := range p.batches {
		if !b.expires.After(now) {
			delete(p.batches, k)
			n++
		}
	}
	return n
}

func (p *pendingBatches) len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.batches)
}
