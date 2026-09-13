package outbox_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

const (
	telegram = "telegram"
	bot      = "telegram-bot"
	playerA  = "player-A"
	playerB  = "player-B"
)

var t0 = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// routes is the links.db of the tests: player → platform and external ID.
type routes struct {
	mu   sync.Mutex
	rows map[string][2]string
}

func newRoutes(players ...string) *routes {
	r := &routes{rows: map[string][2]string{}}
	for i, p := range players {
		r.rows[p] = [2]string{telegram, fmt.Sprintf("10020030%d", i)}
	}
	return r
}

func (r *routes) set(player, platform, externalID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[player] = [2]string{platform, externalID}
}

func (r *routes) RouteFor(_ context.Context, playerID string) (string, string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[playerID]
	return row[0], row[1], ok, nil
}

// countingTimers counts the timers a long-poll arms: the wait, then one
// wake-up per round of waiting.
type countingTimers struct {
	clock.Timers
	afters atomic.Int32
}

func (c *countingTimers) After(d time.Duration) clock.Timer {
	c.afters.Add(1)
	return c.Timers.After(d)
}

type fixture struct {
	db     *sql.DB
	store  *outbox.Store
	clock  *clock.Manual
	timers *countingTimers
	routes *routes
	svc    *outbox.Service
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	db, err := store.OpenGateway(ctx, store.GatewayPath(sqlitedir.Temp(t)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.MigrateGateway(ctx, db); err != nil {
		t.Fatal(err)
	}
	s, err := outbox.New(outbox.Config{DB: db})
	if err != nil {
		t.Fatal(err)
	}
	manual := clock.NewManual(t0)
	r := newRoutes(playerA, playerB)
	timers := &countingTimers{Timers: manual.Timers()}
	svc, err := outbox.NewService(outbox.ServiceConfig{Store: s, Routes: r, Clock: manual, Timers: timers})
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{db: db, store: s, clock: manual, timers: timers, routes: r, svc: svc}
}

// delivery is a pending mechanics delivery of event to player on telegram.
func delivery(event, player string) outbox.Delivery {
	return outbox.Delivery{WorldID: "dark-forest-world", PlayerID: player, Platform: telegram, Kind: outbox.KindMechanics,
		CorrelationID: "action-" + event, EventID: event, GeneratedBy: outbox.GeneratedByRules, Text: "Попадание! Урон 3."}
}

// enqueue writes deliveries in a transaction of their own, as the consumer
// does, at the time of the clock.
func (f *fixture) enqueue(t *testing.T, ds ...outbox.Delivery) int {
	t.Helper()
	ctx := context.Background()
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	n, err := f.store.Enqueue(ctx, tx, f.clock.Now(), ds...)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *fixture) lease(t *testing.T, client string, limit int) []string {
	t.Helper()
	ds, err := f.store.Lease(context.Background(), client, telegram, limit, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	return eventsOf(ds)
}

func eventsOf(ds []outbox.Delivery) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.EventID)
	}
	return out
}

func (f *fixture) state(t *testing.T, event, player string) string {
	t.Helper()
	var state string
	id := outbox.EventDeliveryID(event, player, outbox.KindMechanics)
	if err := f.db.QueryRowContext(context.Background(), `SELECT state FROM deliveries WHERE id = ?`, id).Scan(&state); err != nil {
		t.Fatalf("state of %s: %v", id, err)
	}
	return state
}

func (f *fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func id(event, player string) string {
	return outbox.EventDeliveryID(event, player, outbox.KindMechanics)
}
