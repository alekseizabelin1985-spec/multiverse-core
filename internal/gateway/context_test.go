package gateway_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/internal/gateway/handlers"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// externalID is shaped like a Telegram user id; it is not a real account.
const externalID = "7391846205"

var t0 = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

func sequence() func() string {
	var mu sync.Mutex
	n := 0
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		n++
		return "seq-" + strconv.Itoa(n)
	}
}

type running struct {
	ctx    *gateway.Context
	clock  *clock.Manual
	client *client.Client
	dir    string
	bus    *membus.Bus
}

// start runs the context the way serve does: Routes on the mux, Start, and
// only then the server.
func start(t *testing.T, mode runtime.Mode) running {
	t.Helper()
	return startIn(t, mode, sqlitedir.Temp(t))
}

// startIn is start on a data directory the test prepared.
func startIn(t *testing.T, mode runtime.Mode, dir string) running {
	t.Helper()
	return startWith(t, mode, dir, nil)
}

// startWith is startIn with the object store the projection is loaded from;
// nil is no store, the process on the memory bus.
func startWith(t *testing.T, mode runtime.Mode, dir string, objects objstore.Client) running {
	t.Helper()
	return startOpts(t, mode, dir, options{objects: objects})
}

// options vary what a context under test is built with.
type options struct {
	// objects is the object store the projection is loaded from; nil builds
	// the client from the variables.
	objects objstore.Client
	// vars are added to the source of the context.
	vars map[string]string
	// bus and journal stand in front of membus when set.
	bus     func(*membus.Bus) eventbus.Bus
	journal func(*membus.Bus) eventbus.Journal
	// budget shortens both budgets of the start when set.
	budget time.Duration
	// timers stands in front of the manual timers of the clock when set.
	timers func(clock.Timers) clock.Timers
	// prepare runs on the data directory before the context is built.
	prepare func(dir string)
	// log receives the lines of the context when set.
	log io.Writer
	// probeInterval shortens the interval of the checks of /health when set.
	probeInterval time.Duration
	// repairBudget and snapshotBudget shorten the budget of a repair of stale
	// entities and of a snapshot write when set.
	repairBudget, snapshotBudget time.Duration
	// beforeStart runs on the bus after the context is built and before it
	// starts: the journal a stopped process finds.
	beforeStart func(bus *membus.Bus)
}

// build makes the context and its dependencies without starting it.
func build(t *testing.T, mode runtime.Mode, dir string, o options) (running, *http.ServeMux, runtime.Deps) {
	t.Helper()
	if o.prepare != nil {
		o.prepare(dir)
	}
	vars := map[string]string{env.GatewayDataDir.Name(): dir}
	for name, value := range o.vars {
		vars[name] = value
	}
	c := gateway.New(env.MapSource(vars))
	if o.objects != nil {
		gateway.SetObjectStore(c, o.objects)
	}
	if o.probeInterval > 0 {
		gateway.SetHealthProbeInterval(c, o.probeInterval)
	}
	if o.repairBudget > 0 {
		gateway.SetStaleRepairBudget(c, o.repairBudget)
	}
	if o.snapshotBudget > 0 {
		gateway.SetSnapshotWriteBudget(c, o.snapshotBudget)
	}
	if o.budget > 0 {
		gateway.SetStartBudgets(c, o.budget, o.budget)
	}
	manual := clock.NewManual(t0)
	mux := http.NewServeMux()
	c.Routes(mux)
	bus := newBus(t)
	deps := runtime.Deps{Clock: manual, Timers: manual.Timers(), IDs: sequence(), Mode: mode,
		Bus: bus, Journal: bus, Log: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	if o.log != nil {
		deps.Log = slog.New(slog.NewJSONHandler(o.log, nil))
	}
	if o.beforeStart != nil {
		o.beforeStart(bus)
	}
	if o.bus != nil {
		deps.Bus = o.bus(bus)
	}
	if o.journal != nil {
		deps.Journal = o.journal(bus)
	}
	if o.timers != nil {
		deps.Timers = o.timers(deps.Timers)
	}
	return running{ctx: c, clock: manual, dir: dir, bus: bus}, mux, deps
}

// startOpts starts the context build made, the way serve does: Routes on the
// mux, Start, and only then the server.
func startOpts(t *testing.T, mode runtime.Mode, dir string, o options) running {
	t.Helper()
	r, mux, deps := build(t, mode, dir, o)
	c := r.ctx
	if err := c.Start(context.Background(), deps); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Stop is idempotent: a test that stopped the context itself loses nothing.
	t.Cleanup(func() { _ = c.Stop(context.Background()) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	r.client = client.New(srv.URL, "telegram-bot")
	r.client.Backoff = client.NoRetry
	return r
}

// The links routes answer through the real middleware, store and files.
func TestTheContextServesTheLinksRoutes(t *testing.T) {
	r := start(t, runtime.ModeLive)
	ctx := context.Background()

	res, err := r.client.Resolve(ctx, links.PlatformTelegram, externalID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.LinkStatus != links.StatusPendingConsent || res.CharacterStatus != "none" || !res.NoticeDue || res.PlayerID != nil {
		t.Errorf("Resolve = %+v, want pending_consent, none, notice due, no player", res)
	}

	_, err = r.client.Consent(ctx, api.ConsentRequest{ExternalPlatform: links.PlatformTelegram, ExternalID: externalID,
		NoticeShown: true, Consent: true, ShownAt: t0})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 400 || apiErr.Code != api.CodeConsentIncomplete || apiErr.RequestID == "" {
		t.Fatalf("incomplete Consent = %v, want 400 consent_incomplete with a request id", err)
	}
	consent, err := r.client.Consent(ctx, api.ConsentRequest{ExternalPlatform: links.PlatformTelegram, ExternalID: externalID,
		NoticeShown: true, Consent: true, AgeConfirmed: true, ShownAt: t0.Add(-time.Minute)})
	if err != nil || consent.LinkStatus != links.StatusConsented || !consent.ConsentAt.Equal(t0) || !consent.NoticeShownAt.Equal(t0.Add(-time.Minute)) {
		t.Fatalf("Consent = %+v %v", consent, err)
	}
	if res, _ := r.client.Resolve(ctx, links.PlatformTelegram, externalID); res.NoticeDue || res.LinkStatus != links.StatusConsented {
		t.Errorf("Resolve after consent = %+v, want consented without the notice", res)
	}
	r.clock.Advance(handlers.NoticeRepeatAfter)
	if res, _ := r.client.Resolve(ctx, links.PlatformTelegram, externalID); !res.NoticeDue {
		t.Errorf("Resolve 30 days after the last visit = %+v, want the notice due", res)
	}

	if _, err := r.client.Resolve(ctx, "discord", externalID); !errors.As(err, &apiErr) || apiErr.Code != api.CodeInvalidRequest {
		t.Errorf("Resolve on another platform = %v, want 400 invalid_request", err)
	}

	forgot, err := r.client.Forget(ctx, links.PlatformTelegram, externalID)
	if err != nil || !forgot.Deleted || forgot.PlayerIDDetached != nil {
		t.Fatalf("Forget = %+v %v, want the link deleted without a character", forgot, err)
	}
	again, err := r.client.Forget(ctx, links.PlatformTelegram, externalID)
	if err != nil || again.Deleted {
		t.Errorf("repeated Forget = %+v %v, want {deleted:false}", again, err)
	}
	for _, name := range []string{store.LinksPath(r.dir), store.LinksPath(r.dir) + "-wal"} {
		data, err := os.ReadFile(name)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if strings.Contains(string(data), externalID) {
			t.Errorf("%s holds the forgotten ID after /forget", name)
		}
	}

	stranger := client.New(strings.TrimSuffix(r.client.BaseURL, "/"), "stranger")
	stranger.Backoff = client.NoRetry
	if _, err := stranger.Resolve(ctx, links.PlatformTelegram, externalID); !errors.As(err, &apiErr) || apiErr.Code != api.CodeClientUnknown {
		t.Errorf("Resolve from an unlisted client = %v, want 403 client_unknown", err)
	}

	if h := r.ctx.Health(); h.Status != runtime.StatusOK || h.Details["links_store"] != runtime.StatusOK {
		t.Errorf("Health = %+v, want ok", h)
	}
	if err := r.ctx.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if h := r.ctx.Health(); h.Status != runtime.StatusFail {
		t.Errorf("Health after Stop = %+v, want fail", h)
	}
	if err := r.ctx.Stop(context.Background()); err != nil {
		t.Errorf("a second Stop = %v", err)
	}
}

// In live mode the sweeper removes expired character requests and answers to
// actions on its tick; in replay mode no timer of the gateway runs (component
// §11.2).
func TestTheSweeperRunsInLiveModeOnly(t *testing.T) {
	for _, mode := range []runtime.Mode{runtime.ModeLive, runtime.ModeReplay} {
		t.Run(string(mode), func(t *testing.T) {
			r := start(t, mode)
			ctx := context.Background()
			if _, err := r.client.Resolve(ctx, links.PlatformTelegram, externalID); err != nil {
				t.Fatal(err)
			}
			// A second connection of the test writes the request the characters
			// route of T-306 will write.
			db, err := store.OpenLinks(ctx, store.LinksPath(r.dir))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()
			if _, err := db.ExecContext(ctx, `INSERT INTO character_requests
				(link_id, action_key, player_id, status_code, response_json, expires_at)
				SELECT link_id, 'k', 'player-1', 201, '{}', '2026-09-13T11:00:00.000000000Z' FROM links`); err != nil {
				t.Fatal(err)
			}
			// And the answer to an action T-305 keeps, expired as well.
			gdb, err := store.OpenGateway(ctx, store.GatewayPath(r.dir))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = gdb.Close() }()
			if _, err := gdb.ExecContext(ctx, `INSERT INTO idempotency_keys
				(player_id, action_key, correlation_id, status_code, response_json, created_at, expires_at)
				VALUES ('player-1', 'k', 'ev-1', 202, '{}', '2026-09-12T11:00:00.000000000Z', '2026-09-13T11:00:00.000000000Z')`); err != nil {
				t.Fatal(err)
			}
			count := func() int {
				var n, keys int
				if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM character_requests").Scan(&n); err != nil {
					t.Fatal(err)
				}
				if err := gdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM idempotency_keys").Scan(&keys); err != nil {
					t.Fatal(err)
				}
				return n + keys
			}
			// The sweeper registers its tickers in its own goroutine: advance
			// until the tick lands, within a bound.
			deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			for count() != 0 && deadline.Err() == nil {
				r.clock.Advance(store.SweepInterval)
				time.Sleep(20 * time.Millisecond)
				if mode == runtime.ModeReplay && r.clock.Now().After(t0.Add(90*time.Minute)) {
					break
				}
			}
			switch n := count(); {
			case mode == runtime.ModeLive && n != 0:
				t.Errorf("live: %d expired requests left after the sweeps", n)
			case mode == runtime.ModeReplay && n != 2:
				t.Errorf("replay: %d requests and keys left, want the two untouched", n)
			}
		})
	}
}

// A Stop whose deadline has passed before the sweeper stopped still closes the
// databases: the context is not started again, and on Windows an open file
// stays locked (review #1, N-4).
func TestStopAtAnExpiredDeadlineClosesTheDatabases(t *testing.T) {
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	// The sweeper is told to stop and has not run yet when Stop selects, so the
	// expired deadline is taken almost every time; a few contexts make sure.
	for i := range 5 {
		r := start(t, runtime.ModeLive)
		err := r.ctx.Stop(expired)
		if err != nil && !strings.Contains(err.Error(), "sweeper did not stop") {
			t.Fatalf("Stop %d = %v", i, err)
		}
		if !gateway.DatabasesClosed(r.ctx) {
			t.Fatalf("Stop %d at an expired deadline (%v) left the databases open", i, err)
		}
	}
}

func TestStartRefusesWhatItCannotRunWith(t *testing.T) {
	manual := clock.NewManual(t0)
	bus := newBus(t)
	deps := runtime.Deps{Clock: manual, Timers: manual.Timers(), IDs: sequence(), Log: slog.New(slog.DiscardHandler),
		Bus: bus, Journal: bus}

	empty := gateway.New(env.MapSource(map[string]string{env.GatewayDataDir.Name(): ""}))
	// An empty value falls back to the default of the manifest, so the refusal
	// is tested on a data directory that is a file.
	file := sqlitedir.Temp(t) + string(os.PathSeparator) + "file"
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	notADir := gateway.New(env.MapSource(map[string]string{env.GatewayDataDir.Name(): file}))
	if err := notADir.Start(context.Background(), deps); err == nil {
		_ = notADir.Stop(context.Background())
		t.Error("Start succeeded on a data directory that is a file")
	}
	noIDs := deps
	noIDs.IDs = nil
	if err := empty.Start(context.Background(), noIDs); err == nil {
		t.Error("Start succeeded without Deps.IDs")
	}
	noBus := deps
	noBus.Bus, noBus.Journal = nil, nil
	if err := empty.Start(context.Background(), noBus); err == nil {
		t.Error("Start succeeded without Deps.Bus and Deps.Journal")
	}
	if h := empty.Health(); h.Status != runtime.StatusFail {
		t.Errorf("Health of a context that did not start = %+v, want fail", h)
	}
	if empty.Name() != "gateway" || empty.DependsOn() != nil {
		t.Errorf("Name %q DependsOn %v", empty.Name(), empty.DependsOn())
	}
}

// With --id-source=sequence the link_id of an account is the same in every run
// of the same calls: an HTTP request does not draw an id of its own from
// Deps.IDs, so the failed and repeated requests before it shift nothing.
func TestRequestsDoNotShiftTheSequenceOfLinkIDs(t *testing.T) {
	r := start(t, runtime.ModeLive)
	ctx := context.Background()
	for range 3 {
		if _, err := r.client.Resolve(ctx, "discord", externalID); err == nil {
			t.Fatal("a refused request succeeded")
		}
	}
	for _, id := range []string{externalID, "5550001111", externalID} {
		if _, err := r.client.Resolve(ctx, links.PlatformTelegram, id); err != nil {
			t.Fatal(err)
		}
	}
	db, err := store.OpenLinks(ctx, store.LinksPath(r.dir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var first, second string
	if err := db.QueryRowContext(ctx, "SELECT link_id FROM links WHERE external_id = ?", externalID).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT link_id FROM links WHERE external_id = '5550001111'").Scan(&second); err != nil {
		t.Fatal(err)
	}
	if first != "seq-1" || second != "seq-2" {
		t.Errorf("link_id = %q and %q, want seq-1 and seq-2", first, second)
	}
}
