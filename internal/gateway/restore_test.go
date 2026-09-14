package gateway_test

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// stamp is the form of the time columns of gateway.db.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000000Z") }

// seedGateway migrates gateway.db of dir and runs the statements in it, the
// rows a stopped process left behind.
func seedGateway(t *testing.T, dir string, statements ...string) {
	t.Helper()
	ctx := context.Background()
	db, err := store.OpenGateway(ctx, store.GatewayPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := store.MigrateGateway(ctx, db); err != nil {
		t.Fatal(err)
	}
	for _, q := range statements {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

func sessionRow(scope string, started, last time.Time) (string, string) {
	id := scope + ":" + strconv.FormatInt(started.Unix(), 10)
	return id, `INSERT INTO sessions (id, world_id, scope_id, scope_type, kind, actor_kind, participants, started_at,
		last_action_at, state) VALUES ('` + id + `', '` + world + `', '` + scope + `', 'solo', 'solo', 'human', '["` + scope + `"]', '` +
		stamp(started) + `', '` + stamp(last) + `', 'active')`
}

func turnRow(correlation, sessionID, player string, received, deadline time.Time) string {
	return `INSERT INTO turns (correlation_id, session_id, seq, world_id, scope_id, scope_type, player_id, player_name,
		action_type, status, received_at, acked_at, deadline_at) VALUES ('` + correlation + `', '` + sessionID + `', 1, '` + world +
		`', '` + player + `', 'solo', '` + player + `', 'Вася', 'look', 'accepted', '` + stamp(received) + `', '` +
		stamp(received) + `', '` + stamp(deadline) + `')`
}

// countingTimers counts the timers the gateway arms.
type countingTimers struct {
	clock.Timers
	mu            sync.Mutex
	after, every  int
	stopped, made []clock.Timer
}

func (c *countingTimers) After(d time.Duration) clock.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.after++
	return c.Timers.After(d)
}

func (c *countingTimers) Every(d time.Duration) clock.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.every++
	tm := &trackedTimer{Timer: c.Timers.Every(d), owner: c}
	c.made = append(c.made, tm)
	return tm
}

func (c *countingTimers) counts() (after, every, stopped int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.after, c.every, len(c.stopped)
}

type trackedTimer struct {
	clock.Timer
	owner *countingTimers
	once  sync.Once
}

func (t *trackedTimer) Stop() bool {
	t.once.Do(func() {
		t.owner.mu.Lock()
		t.owner.stopped = append(t.owner.stopped, t)
		t.owner.mu.Unlock()
	})
	return t.Timer.Stop()
}

func analyticsOf(t *testing.T, r running) []eventbus.Event {
	t.Helper()
	records, err := r.bus.Records(eventbus.TopicAnalyticsEvents)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]eventbus.Event, 0, len(records))
	for _, raw := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		out = append(out, ev)
	}
	return out
}

// A restart finds a session idle for 30 minutes with a turn past its deadline,
// and a session that is not idle. Before the first request and before the
// sweeper ever ticks, the turn is timeout and the idle session ended idle at
// the moment it became idle; the other goes on, and sessions_active counts it
// (component §7.6, acceptance of T-306). In live mode their analytics are
// published; in replay nothing is, and no timer of the gateway is armed
// (component §11.2).
func TestARestartClosesIdleSessionsAndLateTurnsBeforeItServes(t *testing.T) {
	for _, mode := range []runtime.Mode{runtime.ModeLive, runtime.ModeReplay} {
		t.Run(string(mode), func(t *testing.T) {
			idleStart, idleLast := t0.Add(-50*time.Minute), t0.Add(-45*time.Minute)
			idleID, idleSQL := sessionRow("player-A", idleStart, idleLast)
			freshID, freshSQL := sessionRow("player-B", t0.Add(-10*time.Minute), t0.Add(-5*time.Minute))
			timers := &countingTimers{}
			r := startOpts(t, mode, sqlitedir.Temp(t), options{
				prepare: func(dir string) {
					seedGateway(t, dir, idleSQL, freshSQL, turnRow("act-late", idleID, "player-A", idleLast, idleLast.Add(time.Minute)))
				},
				timers: func(inner clock.Timers) clock.Timers { timers.Timers = inner; return timers },
			})
			ctx := context.Background()
			db, err := store.OpenGateway(ctx, store.GatewayPath(r.dir))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()

			var state, reason, endedAt string
			if err := db.QueryRowContext(ctx, `SELECT state, end_reason, ended_at FROM sessions WHERE id = ?`, idleID).
				Scan(&state, &reason, &endedAt); err != nil {
				t.Fatal(err)
			}
			if state != session.StateEnded || reason != session.EndIdle || endedAt != stamp(idleLast.Add(session.DefaultIdle)) {
				t.Errorf("idle session after the start: %s %s %s, want ended idle at %s", state, reason, endedAt, stamp(idleLast.Add(session.DefaultIdle)))
			}
			if err := db.QueryRowContext(ctx, `SELECT state FROM sessions WHERE id = ?`, freshID).Scan(&state); err != nil || state != session.StateActive {
				t.Errorf("the session that is not idle: %s %v, want active", state, err)
			}
			var status string
			if err := db.QueryRowContext(ctx, `SELECT status FROM turns WHERE correlation_id = 'act-late'`).Scan(&status); err != nil ||
				status != turns.StatusTimeout {
				t.Errorf("late turn after the start: %s %v, want timeout", status, err)
			}
			if h := r.ctx.Health(); h.Details["sessions_active"] != 1 {
				t.Errorf("sessions_active = %v, want 1: %+v", h.Details["sessions_active"], h)
			}

			got := map[string]int{}
			for _, ev := range analyticsOf(t, r) {
				got[ev.Type]++
			}
			after, every, _ := timers.counts()
			switch mode {
			case runtime.ModeLive:
				if got[session.TypeEnded] != 1 || got[turns.TypeCompleted] != 1 {
					t.Errorf("live analytics of the restoration = %v, want one session.ended and one turn.completed", got)
				}
			case runtime.ModeReplay:
				if len(got) != 0 {
					t.Errorf("replay published analytics %v", got)
				}
				if after != 0 || every != 0 {
					t.Errorf("replay armed %d timers and %d tickers, want none", after, every)
				}
				if gateway.Limiter(r.ctx) != nil {
					t.Error("replay built the rate limit")
				}
			}
		})
	}
}

// In replay the gateway arms no timer and runs no sweeper, and the harness still
// publishes its actions (component §11.2): the counted timers stay at zero
// through an action and an hour of the clock, the action reaches the bus, and
// neither analytics nor a snapshot of the gateway are published, Stop included
// (answer 6 of architect#3), although a store and a world are there.
func TestReplayArmsNoTimersAndStillTakesActions(t *testing.T) {
	timers := &countingTimers{}
	r := startOpts(t, runtime.ModeReplay, sqlitedir.Temp(t), options{
		objects: objstore.NewMemory(),
		timers:  func(inner clock.Timers) clock.Timers { timers.Timers = inner; return timers },
	})
	if !gateway.Live(r.ctx) {
		t.Error("the consumer of a replay did not reach the end of the journal")
	}
	withCharacter(t, r)
	ctx := context.Background()
	if a, err := r.client.Action(ctx, "player-A", api.ActionRequest{ActionKey: "look-replay", Type: api.ActionLook}); err != nil || a.Accepted == nil {
		t.Fatalf("Action in replay = %+v %v", a, err)
	}
	r.clock.Advance(time.Hour)
	if after, every, _ := timers.counts(); every != 0 {
		t.Errorf("replay armed %d tickers (and %d timers)", every, after)
	}
	records, err := r.bus.Records(eventbus.TopicPlayerEvents)
	if err != nil || len(records) == 0 {
		t.Errorf("the action of the harness is not on the bus: %d %v", len(records), err)
	}
	if got := analyticsOf(t, r); len(got) != 0 {
		t.Errorf("replay published %d analytics", len(got))
	}
	if err := r.ctx.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if n := len(snapshotEvents(t, r.bus)); n != 0 {
		t.Errorf("replay published %d snapshot.created of the gateway", n)
	}
}

// narrativeOf is narrative.output of the turn correlation for player.
func narrativeOf(correlation, player string) eventbus.Event {
	ev := eventbus.NewRoot("narrative.output", contracts.SourceSwarm, world, nil, eventbus.ActorSystem, map[string]any{
		"recipients": []any{map[string]any{"entity": map[string]any{"id": player, "type": entity.TypePlayer}}},
		"text":       "Волк скалится.", "generated_by": "template", "fallback_reason": "timeout", "kind": "turn",
		"locale": "ru", "laws_version": "v1", "narrative_event_id": "n-" + correlation,
		"filter": map[string]any{"applied": true, "status": "pass", "filter_version": "1"},
	})
	ev.Meta.CorrelationID = correlation
	ev.Meta.Agent = &eventbus.AgentRef{ID: "narrator:solo:" + player, Level: "task", Blueprint: "narrator"}
	return ev
}

// The restoration runs before the catch-up of the journal (component §7.6): a
// turn past its deadline whose narrative lies in the journal after the stop
// is timeout, not completed by that narrative (Mi-3 of review #1 of T-309).
func TestTheRestorationComesBeforeTheCatchUp(t *testing.T) {
	testkit.Deterministic(t, "gw")
	last := t0.Add(-10 * time.Minute)
	sessionID, row := sessionRow("player-A", t0.Add(-15*time.Minute), last)
	var earlier eventbus.Event
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{
		bus: func(b *membus.Bus) eventbus.Bus { return silentBus{Bus: b} },
		beforeStart: func(bus *membus.Bus) {
			earlier = narrativeOf("act-before", "player-A")
			for _, ev := range []eventbus.Event{earlier, narrativeOf("act-late", "player-A")} {
				if err := bus.Publish(context.Background(), ev); err != nil {
					t.Fatalf("publish %s: %v", ev.Type, err)
				}
			}
		},
		prepare: func(dir string) {
			seedGateway(t, dir, row, turnRow("act-late", sessionID, "player-A", last, last.Add(time.Minute)),
				`INSERT INTO cursors (topic, "offset", event_id, updated_at) VALUES ('narrative_output', 0, 'n-0', '`+stamp(last)+`')`)
		},
	})
	db, err := store.OpenGateway(context.Background(), store.GatewayPath(r.dir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var status string
	var offset int64
	if err := db.QueryRowContext(context.Background(), `SELECT status FROM turns WHERE correlation_id = 'act-late'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(context.Background(), `SELECT "offset" FROM cursors WHERE topic = 'narrative_output'`).Scan(&offset); err != nil || offset != 1 {
		t.Fatalf("control: the narrative of the late turn was not caught up: offset %d %v", offset, err)
	}
	if status != turns.StatusTimeout {
		t.Errorf("late turn = %s, want timeout: the restoration ran after the catch-up", status)
	}
}
