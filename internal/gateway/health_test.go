package gateway_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

func deliveryRow(event, player string, created time.Time) string {
	return `INSERT INTO deliveries (id, world_id, player_id, platform, kind, correlation_id, event_id, generated_by, text,
		state, created_at, expires_at) VALUES ('d-` + event + `', '` + world + `', '` + player + `', 'telegram', 'mechanics', 'act-` +
		event + `', '` + event + `', 'rules', 'x', 'pending', '` + stamp(created) + `', '` + stamp(created.Add(24*time.Hour)) + `')`
}

// /health of the gateway carries every field of component §11.4 the gateway
// has: the two stores, the bus, the projection, the mode, the active sessions,
// the open rounds and the queue of the outbox with the age of its oldest
// delivery.
func TestHealthCarriesTheFieldsOfTheComponent(t *testing.T) {
	r := start(t, runtime.ModeLive)
	h := r.ctx.Health()
	want := map[string]any{
		"links_store": runtime.StatusOK, "gateway_store": runtime.StatusOK, "bus": runtime.StatusOK,
		"projection": "missing", "mode": "live",
		"sessions_active": 0, "rounds_open": 0, "outbox_pending": 0, "outbox_oldest_age_s": int64(0),
	}
	for key, value := range want {
		if got, ok := h.Details[key]; !ok || got != value {
			t.Errorf("%s = %v (%v), want %v", key, got, ok, value)
		}
	}
	if h.Status != runtime.StatusOK {
		t.Errorf("status = %s, want ok", h.Status)
	}
}

// Two pending deliveries, the oldest created 90 s ago, are 2 and 90 (acceptance
// of T-307); the rounds that are open or closing are counted.
func TestHealthCountsTheOutboxAndTheOpenRounds(t *testing.T) {
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{prepare: func(dir string) {
		seedGateway(t, dir,
			deliveryRow("e1", "player-A", t0.Add(-90*time.Second)),
			deliveryRow("e2", "player-B", t0.Add(-20*time.Second)),
			`INSERT INTO rounds (scope_id, seq, world_id, encounter_id, state, timeout_ms, idle_after_missed, expected, acted,
				auto_defended, idle, opened_at, deadline_at, opened_event_id) VALUES ('group-1', 1, '`+world+`', 'enc-1', 'open',
				60000, 2, '[]', '[]', '[]', '[]', '`+stamp(t0)+`', '`+stamp(t0.Add(time.Minute))+`', 'ro-1')`,
		)
	}})
	h := r.ctx.Health()
	if h.Details["outbox_pending"] != 2 || h.Details["outbox_oldest_age_s"] != int64(90) || h.Details["rounds_open"] != 1 {
		t.Errorf("Health = %+v, want outbox 2/90 and one open round", h.Details)
	}
}

// The databases are read at most once per HealthProbeInterval (NFR-016): a
// delivery written meanwhile shows only once the interval has passed.
func TestHealthReadsTheDatabasesAtMostEveryTenSeconds(t *testing.T) {
	r := start(t, runtime.ModeLive)
	if h := r.ctx.Health(); h.Details["outbox_pending"] != 0 {
		t.Fatalf("control: %+v", h.Details)
	}
	seedGateway(t, r.dir, deliveryRow("e1", "player-A", t0))
	r.clock.Advance(gateway.HealthProbeInterval - time.Nanosecond)
	if h := r.ctx.Health(); h.Details["outbox_pending"] != 0 {
		t.Errorf("read again before the interval: %+v", h.Details)
	}
	r.clock.Advance(time.Nanosecond)
	if h := r.ctx.Health(); h.Details["outbox_pending"] != 1 {
		t.Errorf("not read again after the interval: %+v", h.Details)
	}
}

// A database that fails its check fails the gateway (component §11.4), each
// store reported on its own (acceptance of T-303, N-3).
func TestAClosedDatabaseFailsTheGateway(t *testing.T) {
	for name, closeDB := range map[string]func(*gateway.Context) error{
		"gateway_store": gateway.CloseGatewayDB,
		"links_store":   gateway.CloseLinksDB,
	} {
		t.Run(name, func(t *testing.T) {
			r := start(t, runtime.ModeLive)
			if h := r.ctx.Health(); h.Status != runtime.StatusOK {
				t.Fatalf("control: %+v", h)
			}
			if err := closeDB(r.ctx); err != nil {
				t.Fatal(err)
			}
			r.clock.Advance(gateway.HealthProbeInterval)
			h := r.ctx.Health()
			if h.Status != runtime.StatusFail || h.Details[name] != runtime.StatusFail {
				t.Errorf("Health with %s closed = %+v, want fail", name, h)
			}
			other := map[string]string{"gateway_store": "links_store", "links_store": "gateway_store"}[name]
			if h.Details[other] != runtime.StatusOK {
				t.Errorf("%s = %v, want ok: only %s is closed", other, h.Details[other], name)
			}
		})
	}
}

// A compaction blocked by a reader holds the only connection of links.db for
// busy_timeout. /health does not wait for it: it answers within the deadline
// of its check with the last known value (acceptance of T-303, N-3).
func TestHealthDoesNotWaitForTheOnlyConnection(t *testing.T) {
	r := start(t, runtime.ModeLive)
	consent(t, r.client, externalID)
	const busyMillis = 3000
	if err := gateway.SetLinksBusyTimeout(r.ctx, busyMillis); err != nil {
		t.Fatal(err)
	}
	leave := holdReader(t, r.dir)
	// A second connection opened before the /forget: opening one checks the
	// PRAGMAs of the file, which waits for the compaction.
	links, err := store.OpenLinks(context.Background(), store.LinksPath(r.dir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = links.Close() }()
	forgot := make(chan int, 1)
	go func() { forgot <- forgetRaw(t, r.client.BaseURL, externalID).StatusCode }()
	// The /forget is inside its compaction once its DELETE is committed and it
	// has not answered: nothing else runs between the two.
	eventually(t, "the /forget to reach its compaction", func() bool {
		var n int
		err := links.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM links`).Scan(&n)
		return err == nil && n == 0 && len(forgot) == 0
	})
	r.clock.Advance(gateway.HealthProbeInterval)
	began := clock.Real{}.Now()
	h := r.ctx.Health()
	took := clock.Real{}.Now().Sub(began)
	if took > time.Second {
		t.Errorf("Health took %v while the connection was held, want well below busy_timeout (%d ms)", took, busyMillis)
	}
	if h.Details["links_store"] != runtime.StatusOK {
		t.Errorf("links_store = %v, want the last known ok", h.Details["links_store"])
	}
	if len(forgot) != 0 {
		t.Fatal("control: /forget answered before Health was measured, the connection was not held")
	}
	leave()
	if code := <-forgot; code != http.StatusOK && code != http.StatusServiceUnavailable {
		t.Errorf("/forget = %d", code)
	}
}

// Stop does not hold the lock of the context while it waits for a compaction
// blocked by a reader: /health answers fail within 100 ms meanwhile (R2-N-3 of
// review #2 of T-303).
func TestHealthAnswersFailAtOnceWhileStopWaits(t *testing.T) {
	r := start(t, runtime.ModeReplay)
	consent(t, r.client, externalID)
	if err := gateway.SetLinksBusyTimeout(r.ctx, 0); err != nil {
		t.Fatal(err)
	}
	leave := holdReader(t, r.dir)
	defer leave()
	if code := forgetRaw(t, r.client.BaseURL, externalID).StatusCode; code != http.StatusServiceUnavailable {
		t.Fatalf("/forget under a reader = %d, want 503 with the compaction pending", code)
	}
	if err := gateway.SetLinksBusyTimeout(r.ctx, 2000); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- r.ctx.Stop(context.Background()) }()
	time.Sleep(300 * time.Millisecond)
	select {
	case err := <-stopped:
		t.Fatalf("control: Stop returned before the blocked compaction could hold it: %v", err)
	default:
	}
	began := clock.Real{}.Now()
	h := r.ctx.Health()
	if took := (clock.Real{}).Now().Sub(began); took > 100*time.Millisecond {
		t.Errorf("Health took %v while Stop waited, want at most 100 ms", took)
	}
	if h.Status != runtime.StatusFail {
		t.Errorf("Health while Stop waits = %+v, want fail", h)
	}
	leave()
	<-stopped
	if !gateway.DatabasesClosed(r.ctx) {
		t.Error("Stop left the databases open")
	}
}

// In replay the clock of the events stands between events, and /health still
// sees a database that failed after the last one: the cadence of its checks
// runs on the wall clock (component §11.4, NFR-016). Deps.Clock never moves
// here.
func TestReplayHealthSeesAClosedDatabaseOnTheWallClock(t *testing.T) {
	const interval = 50 * time.Millisecond
	r := startOpts(t, runtime.ModeReplay, sqlitedir.Temp(t), options{probeInterval: interval})
	if h := r.ctx.Health(); h.Status != runtime.StatusOK {
		t.Fatalf("control: %+v", h)
	}
	if err := gateway.CloseGatewayDB(r.ctx); err != nil {
		t.Fatal(err)
	}
	began := clock.Real{}.Now()
	for {
		h := r.ctx.Health()
		if h.Status == runtime.StatusFail && h.Details["gateway_store"] == runtime.StatusFail {
			break
		}
		if (clock.Real{}).Now().Sub(began) > 3*time.Second {
			t.Fatalf("Health in replay did not see the closed gateway.db: %+v", h)
		}
		runtimeYield()
	}
	if took := (clock.Real{}).Now().Sub(began); took > interval+gateway.HealthProbeDeadline+time.Second {
		t.Errorf("the closed database showed after %v, want about the interval %v", took, interval)
	}
}

// In replay the age of the oldest delivery is taken on the clock of the
// process, the one created_at was written by, while the checks run on the wall
// clock (Mi-4 of review #1 of T-309).
func TestReplayTakesTheAgeOfTheOutboxOnTheClockOfTheProcess(t *testing.T) {
	r := startOpts(t, runtime.ModeReplay, sqlitedir.Temp(t), options{prepare: func(dir string) {
		seedGateway(t, dir, deliveryRow("e1", "player-A", t0.Add(-90*time.Second)))
	}})
	if h := r.ctx.Health(); h.Details["outbox_pending"] != 1 || h.Details["outbox_oldest_age_s"] != int64(90) {
		t.Errorf("Health in replay = %+v, want one pending delivery 90 s old", h.Details)
	}
}
