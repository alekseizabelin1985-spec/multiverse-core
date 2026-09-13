package actions

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/clock"
)

// RateWindow is the window the per-minute limit counts in.
const RateWindow = time.Minute

// LimiterSweepInterval is how often the buckets of players who stopped acting
// are dropped (component §5.1 p. 6).
const LimiterSweepInterval = 10 * time.Minute

// OperationPostAction is the only operation the limiter applies to.
const OperationPostAction = "postAction"

// Limiter is the action rate limit of SEC-11 per player_id: at most PerMinute
// actions in any minute, and at most Burst of them in a row (component §5.1
// p. 6). It is the RateLimiter of the middleware and runs in live mode only.
//
// It is two limits, not one token bucket. A bucket of PerMinute per minute with
// a capacity of Burst lets PerMinute+Burst actions through in the first minute
// of a full bucket, and the contract promises PerMinute (api-contracts.md §1.1:
// the 31st action in a minute is refused). The bucket keeps the burst, and a
// log of the last PerMinute acceptances keeps the minute.
//
// The bucket is kept in its GCRA form — the time the next action is due at —
// so that it is plain arithmetic on durations, without the rounding of
// fractional tokens that would refuse an action due to the nanosecond.
type Limiter struct {
	clock     clock.Clock
	perMinute int
	// interval is the time one token takes to come back, tolerance how far
	// ahead of its due time an action may come: the burst.
	interval, tolerance time.Duration

	mu      sync.Mutex
	players map[string]*allowance
}

// allowance is the state of one player: when the next action is due by the
// bucket, and the times of the acceptances of the last minute, oldest first.
type allowance struct {
	due      time.Time
	accepted []time.Time
}

// NewLimiter returns a limiter of perMinute actions per minute and burst in a
// row; both must be positive.
func NewLimiter(clk clock.Clock, perMinute, burst int) (*Limiter, error) {
	if perMinute < 1 || burst < 1 {
		return nil, fmt.Errorf("actions: rate limit %d per minute with a burst of %d: both must be at least 1", perMinute, burst)
	}
	interval := RateWindow / time.Duration(perMinute)
	return &Limiter{clock: clk, perMinute: perMinute, interval: interval,
		tolerance: interval * time.Duration(burst-1), players: make(map[string]*allowance)}, nil
}

var _ api.RateLimiter = (*Limiter)(nil)

// Allow implements api.RateLimiter for POST /v1/players/{player_id}/actions.
func (l *Limiter) Allow(r *http.Request, rt api.Route) (bool, int) {
	if rt.OperationID != OperationPostAction {
		return true, 0
	}
	return l.Take(r.PathValue("player_id"))
}

// Take counts one action of the player when both limits allow it; otherwise
// it counts nothing and returns the whole seconds until they will.
func (l *Limiter) Take(playerID string) (bool, int) {
	now := l.clock.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.players[playerID]
	if a == nil {
		a = &allowance{due: now}
		l.players[playerID] = a
	}
	a.forget(now)

	wait := a.due.Add(-l.tolerance).Sub(now)
	if len(a.accepted) >= l.perMinute {
		wait = max(wait, a.accepted[0].Add(RateWindow).Sub(now))
	}
	if wait > 0 {
		return false, int((wait + time.Second - 1) / time.Second)
	}
	a.due = later(a.due, now).Add(l.interval)
	a.accepted = append(a.accepted, now)
	return true, 0
}

// Sweep drops the players whose bucket is full and whose minute is empty: they
// are indistinguishable from a player who never acted.
func (l *Limiter) Sweep(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for id, a := range l.players {
		a.forget(now)
		if len(a.accepted) == 0 && !a.due.After(now) {
			delete(l.players, id)
		}
	}
}

// Players is the number of players the limiter holds state for.
func (l *Limiter) Players() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.players)
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// forget drops the acceptances that left the minute before now.
func (a *allowance) forget(now time.Time) {
	i := 0
	for i < len(a.accepted) && !a.accepted[i].Add(RateWindow).After(now) {
		i++
	}
	a.accepted = a.accepted[i:]
}
