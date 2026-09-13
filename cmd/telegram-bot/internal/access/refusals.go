package access

import (
	"sync"
	"time"
)

// refusals remembers, per chat, when a series of its refused updates began, so
// that a series is answered and logged once within a Window, and when the bot
// last answered a stranger, so that the answers of the whole bot stay within a
// budget per Window.
//
// The ids are held in memory only and are never written anywhere. A chat is
// let go by the first sweep after its series is a Window old: the gate sweeps
// on every update, so an id outlives its Window only until the next update —
// or until the process stops, when no update comes.
type refusals struct {
	limit   int
	budget  int
	mu      sync.Mutex
	since   map[int64]time.Time
	replies []time.Time
}

func newRefusals(limit, budget int) *refusals {
	return &refusals{limit: limit, budget: budget, since: make(map[int64]time.Time)}
}

// sweep lets go of the chats whose series began a Window ago or earlier.
func (r *refusals) sweep(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked(now)
}

func (r *refusals) sweepLocked(now time.Time) {
	for c, began := range r.since {
		if now.Sub(began) >= Window {
			delete(r.since, c)
		}
	}
}

// opens records a refused update of chat at now and reports whether it opens a
// series: no series of the chat began within the last Window. A chat the full
// table cannot take opens none — silence is the safe side of a flood.
func (r *refusals) opens(chat int64, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if began, ok := r.since[chat]; ok {
		if now.Sub(began) < Window {
			return false
		}
		r.since[chat] = now
		return true
	}
	if len(r.since) >= r.limit {
		r.sweepLocked(now)
		if len(r.since) >= r.limit {
			return false
		}
	}
	r.since[chat] = now
	return true
}

// reply spends one answer of the budget at now and reports whether there was
// one left: fewer than budget answers within the last Window.
func (r *refusals) reply(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := now.Add(-Window)
	kept := r.replies[:0]
	for _, at := range r.replies {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	r.replies = kept
	if len(r.replies) >= r.budget {
		return false
	}
	r.replies = append(r.replies, now)
	return true
}
