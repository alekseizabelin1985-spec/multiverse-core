package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/shared/clock"
)

// The cadence and the budget of the checks of the databases in /health.
const (
	// HealthProbeInterval: the databases are read at most this often
	// (NFR-016: a failure is visible within 30 s; component §12).
	HealthProbeInterval = 10 * time.Second
	// HealthProbeDeadline bounds the wait of one check for the only
	// connection of its database (SetMaxOpenConns(1)). A connection held
	// longer — a compaction waiting out busy_timeout under a reader — leaves
	// the last known values in the answer.
	HealthProbeDeadline = 100 * time.Millisecond
)

// probeValues are what /health reads from the databases.
type probeValues struct {
	linksOK, gatewayOK bool
	sessionsActive     int
	roundsOpen         int
	outboxPending      int
	outboxOldestAge    time.Duration
}

// probe reads the databases for /health without holding the lock of the
// context and without taking the only connection of either database for
// longer than HealthProbeDeadline. The result is kept for HealthProbeInterval;
// a check that ran into the deadline keeps the values it could not read, and
// only a check that failed otherwise — the database closed, the file gone —
// reports the database failed.
type probe struct {
	// clock is Deps.Clock in live mode and the wall clock in replay: it paces
	// the checks. ageClock is Deps.Clock in both: the age of the oldest
	// delivery is taken on the clock its created_at was written by. interval
	// is HealthProbeInterval unless a test shortened it.
	clock    clock.Clock
	ageClock clock.Clock
	interval time.Duration
	linksDB  *sql.DB
	gateway  *sql.DB
	outbox   *outbox.Store
	sessions *session.Manager

	mu      sync.Mutex
	last    probeValues
	at      time.Time
	checked bool
	running bool
}

// read returns the values of the databases, reading them again when the last
// read is HealthProbeInterval old. Only one read runs at a time; a caller that
// meets a read in progress gets the last values at once.
func (p *probe) read(ctx context.Context) probeValues {
	now := p.clock.Now()
	p.mu.Lock()
	if p.running || (p.checked && now.Sub(p.at) < p.interval) {
		defer p.mu.Unlock()
		return p.last
	}
	p.running = true
	last := p.last
	p.mu.Unlock()

	next := p.check(ctx, p.ageClock.Now(), last)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.last, p.at, p.checked, p.running = next, now, true, false
	return next
}

// check reads both databases once, starting from the last values.
func (p *probe) check(ctx context.Context, now time.Time, v probeValues) probeValues {
	if ok, err := p.checkLinks(ctx); !busy(err) {
		v.linksOK = ok
	}
	sessions, rounds, pending, oldest, err := p.checkGateway(ctx, now)
	switch {
	case err == nil:
		v.gatewayOK, v.sessionsActive, v.roundsOpen, v.outboxPending, v.outboxOldestAge = true, sessions, rounds, pending, oldest
	case !busy(err):
		v.gatewayOK = false
	}
	return v
}

func (p *probe) checkLinks(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, HealthProbeDeadline)
	defer cancel()
	var n int
	err := p.linksDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master`).Scan(&n)
	return err == nil, deadlineOf(ctx, err)
}

// checkGateway reads the counters of /health from gateway.db: a read of the
// file is the check of the database.
func (p *probe) checkGateway(ctx context.Context, now time.Time) (sessions, rounds, pending int, oldest time.Duration, err error) {
	ctx, cancel := context.WithTimeout(ctx, HealthProbeDeadline)
	defer cancel()
	active, err := p.sessions.Active(ctx)
	if err != nil {
		return 0, 0, 0, 0, deadlineOf(ctx, err)
	}
	if err := p.gateway.QueryRowContext(ctx, `SELECT COUNT(*) FROM rounds WHERE state IN ('open', 'closing')`).Scan(&rounds); err != nil {
		return 0, 0, 0, 0, deadlineOf(ctx, fmt.Errorf("gateway: open rounds: %w", err))
	}
	pending, oldest, err = p.outbox.Pending(ctx, now)
	if err != nil {
		return 0, 0, 0, 0, deadlineOf(ctx, err)
	}
	return len(active), rounds, pending, oldest, nil
}

// errProbeDeadline marks a check that ran out of its deadline rather than one
// the database refused.
var errProbeDeadline = errors.New("gateway: health check ran out of its deadline")

// deadlineOf tells a check cut by its own deadline from a failure: the driver
// does not always wrap the error of the context it was cancelled by.
func deadlineOf(ctx context.Context, err error) error {
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.Join(errProbeDeadline, err)
	}
	return err
}

func busy(err error) bool { return errors.Is(err, errProbeDeadline) }
