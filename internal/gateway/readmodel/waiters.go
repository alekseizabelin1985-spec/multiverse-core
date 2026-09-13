package readmodel

import (
	"context"
	"errors"
	"slices"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
)

// DefaultFactWait is how long a request waits for the fact of its proposal
// before it answers without it (MV_GATEWAY_CHARACTER_WAIT, component §7.1).
const DefaultFactWait = 2 * time.Second

// ErrRejected is returned with the entity.update.rejected that ended a wait:
// State refused the proposal, and no fact will come.
var ErrRejected = errors.New("readmodel: proposal rejected")

// ErrWaiterUsed is returned by a Wait on a waiter that already ended: waited
// once, or cancelled.
var ErrWaiterUsed = errors.New("readmodel: waiter already used")

// Waiter is one wait for the fact of a proposal. It is armed by Expect before
// the proposal is published, so that a fact arriving between the publication
// and the call to Wait is not missed, and it is used once.
//
// A waiter owns no goroutine: Wait blocks the caller, and every way out of it
// — the fact, the rejection, the deadline, the context — unregisters the
// waiter and stops its timer. Nothing unregisters a waiter nobody waits on, so
// every Expect ends in Wait or in Cancel.
type Waiter struct {
	m        *Model
	keys     []string
	timer    clock.Timer
	done     chan outcome
	resolved bool // guarded by m.mu
	waited   bool // guarded by m.mu
}

type outcome struct {
	ev  eventbus.Event
	err error
}

// Expect arms a wait for the first entity.created or entity.updated with the
// given correlation id, or for an entity.update.rejected of one of the given
// proposals. A rejection is matched by its proposal and never by its
// correlation: another package of the same action may be refused without
// saying anything about this one (C-05 v1.4 p. 1а). The deadline starts now.
//
// The caller ends the waiter with Wait or with Cancel, the publication failing
// included: a waiter left alone stays registered, with its timer armed, until
// a fact of the same correlation arrives, which may be never.
//
//	w := m.Expect(id, timeout, id)
//	if err := publish(); err != nil {
//		w.Cancel()
//		return err
//	}
//	fact, err := w.Wait(ctx)
func (m *Model) Expect(correlationID string, timeout time.Duration, proposalIDs ...string) *Waiter {
	w := &Waiter{m: m, done: make(chan outcome, 1)}
	if timeout > 0 {
		w.timer = m.timers.After(timeout)
	}
	w.keys = append(w.keys, factKey(correlationID))
	for _, id := range proposalIDs {
		if id != "" {
			w.keys = append(w.keys, proposalKey(id))
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range w.keys {
		m.waiters[key] = append(m.waiters[key], w)
	}
	return w
}

// Wait blocks until the wait ends. It returns the fact with a nil error, the
// rejection with ErrRejected, context.DeadlineExceeded when the timeout of
// Expect passed first, and the error of ctx when ctx ended first. A waiter is
// used once: Wait after Wait or after Cancel returns ErrWaiterUsed at once.
//
// The deadline is only the end of the wait. It says nothing about the
// encounter or the action behind it: an agent that gave up publishes no
// refusal, and the waiting consumer sees only its deadline (C-05 v1.4 p. 1д).
func (w *Waiter) Wait(ctx context.Context) (eventbus.Event, error) {
	w.m.mu.Lock()
	// A resolved waiter with nothing in done was cancelled: notify always
	// leaves its outcome there under the same lock.
	used := w.waited || (w.resolved && len(w.done) == 0)
	w.waited = true
	w.m.mu.Unlock()
	if used {
		return eventbus.Event{}, ErrWaiterUsed
	}
	defer w.Cancel()
	select {
	case o := <-w.done:
		return o.ev, o.err
	default:
	}
	if w.timer == nil {
		return eventbus.Event{}, context.DeadlineExceeded
	}
	select {
	case o := <-w.done:
		return o.ev, o.err
	case <-w.timer.C():
		select {
		case o := <-w.done:
			return o.ev, o.err
		default:
			return eventbus.Event{}, context.DeadlineExceeded
		}
	case <-ctx.Done():
		return eventbus.Event{}, ctx.Err()
	}
}

// Cancel abandons the wait. It is idempotent, and Wait calls it on its way out.
func (w *Waiter) Cancel() {
	w.m.mu.Lock()
	w.m.unregister(w)
	w.resolved = true
	w.m.mu.Unlock()
	if w.timer != nil {
		w.timer.Stop()
	}
}

// AwaitFact waits for the fact of a proposal published under correlationID,
// whose proposal id is the correlation id itself — the form of a root
// proposal such as the creation of a character (component §7.1). A caller that
// publishes after calling it may miss a fast fact; one that cannot afford that
// arms Expect first.
func (m *Model) AwaitFact(ctx context.Context, correlationID string, timeout time.Duration) (eventbus.Event, error) {
	return m.Expect(correlationID, timeout, correlationID).Wait(ctx)
}

// notify ends the waits registered under key. It must be called with m.mu
// held.
func (m *Model) notify(key string, ev eventbus.Event, err error) {
	for _, w := range slices.Clone(m.waiters[key]) {
		if w.resolved {
			continue
		}
		w.resolved = true
		w.done <- outcome{ev: ev, err: err}
		m.unregister(w)
	}
}

// unregister must be called with m.mu held.
func (m *Model) unregister(w *Waiter) {
	for _, key := range w.keys {
		list := slices.DeleteFunc(m.waiters[key], func(other *Waiter) bool { return other == w })
		if len(list) == 0 {
			delete(m.waiters, key)
			continue
		}
		m.waiters[key] = list
	}
}

func factKey(correlationID string) string { return "fact:" + correlationID }

func proposalKey(proposalID string) string { return "proposal:" + proposalID }
