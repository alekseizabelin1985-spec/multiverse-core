package session_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

const world = "dark-forest-world"

var (
	t0   = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	solo = eventbus.ScopeRef{ID: "player-A", Type: "solo"}
)

func openDB(t *testing.T) *sql.DB {
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
	return db
}

// failing is a bus whose publications fail while fail is set.
type failing struct {
	eventbus.Bus
	mu   sync.Mutex
	fail bool
}

func (f *failing) set(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = fail
}

func (f *failing) Publish(ctx context.Context, ev eventbus.Event) error {
	f.mu.Lock()
	fail := f.fail
	f.mu.Unlock()
	if fail {
		return errors.New("broker down")
	}
	return f.Bus.Publish(ctx, ev)
}

type fixture struct {
	db  *sql.DB
	bus *membus.Bus
	pub *failing
	mgr *session.Manager
}

func newFixture(t *testing.T, replay bool) fixture {
	t.Helper()
	eventbus.SetIDSource(eventbus.SequenceIDs("ev"))
	t.Cleanup(func() { eventbus.SetIDSource(nil) })
	bus, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: []string{eventbus.TopicAnalyticsEvents}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	f := fixture{db: openDB(t), bus: bus, pub: &failing{Bus: bus}}
	cfg := session.Config{DB: f.db, Bus: f.pub}
	if replay {
		cfg.Bus = nil
	}
	if f.mgr, err = session.New(cfg); err != nil {
		t.Fatal(err)
	}
	return f
}

// analytics are the events published on analytics_events, oldest first.
func (f fixture) analytics(t *testing.T) []eventbus.Event {
	t.Helper()
	records, err := f.bus.Records(eventbus.TopicAnalyticsEvents)
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

func types(events []eventbus.Event) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		out = append(out, ev.Type)
	}
	return out
}

func touch(t *testing.T, f fixture, at time.Time, actorKind string) (session.Session, bool) {
	t.Helper()
	s, opened, err := f.mgr.Touch(context.Background(), solo, world, "player-A", actorKind, at)
	if err != nil {
		t.Fatalf("Touch: %v", err)
	}
	return s, opened
}

// The first action of a scope without a session opens one and publishes its
// start: the id of C-10, the actor kind of that action, one participant, and
// nothing a player typed or an external ID (US-038). Later actions of the
// scope stay in it, whatever their actor kind.
func TestTheFirstActionOfAScopeOpensItsSession(t *testing.T) {
	f := newFixture(t, false)
	s, opened := touch(t, f, t0, eventbus.ActorCI)
	if !opened || s.ID != "player-A:"+"1789300800" || s.ActorKind != eventbus.ActorCI || s.Kind != session.KindSolo {
		t.Fatalf("first Touch = %+v opened %v", s, opened)
	}
	again, opened := touch(t, f, t0.Add(time.Minute), eventbus.ActorHuman)
	if opened || again.ID != s.ID || again.ActorKind != eventbus.ActorCI || !again.LastActionAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("second Touch = %+v opened %v, want the same session of ci", again, opened)
	}

	events := f.analytics(t)
	if len(events) != 1 || events[0].Type != session.TypeStarted {
		t.Fatalf("analytics = %v, want one session.started", types(events))
	}
	ev := events[0]
	if err := contracts.Validate(ev); err != nil {
		t.Errorf("session.started is not valid by C-10: %v", err)
	}
	pa := ev.Path()
	if id, _ := pa.GetString("session.id"); id != s.ID {
		t.Errorf("session.id = %q", id)
	}
	if kind, _ := pa.GetString("session.actor_kind"); kind != eventbus.ActorCI || ev.Meta.ActorKind != eventbus.ActorCI {
		t.Errorf("actor kind = %q, meta %q", kind, ev.Meta.ActorKind)
	}
	if n, _ := pa.GetInt("session.players_count"); n != 1 {
		t.Errorf("players_count = %d", n)
	}
	if ev.Scope != nil || pa.Has("scope") || pa.Has("participants[0].name") || ev.World == nil || ev.World.Entity.ID != world {
		t.Errorf("session.started carries a scope or a name, or misses its world: %+v", ev)
	}
	raw, _ := json.Marshal(ev)
	for _, forbidden := range []string{"external", "7391846205", "link_id"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("session.started holds %q: %s", forbidden, raw)
		}
	}
}

// An action after the idle time ends the idle session with end_reason=idle at
// the moment it became idle and opens a new one (BR-17).
func TestAnActionAfterTheIdleTimeStartsANewSession(t *testing.T) {
	f := newFixture(t, false)
	first, _ := touch(t, f, t0, eventbus.ActorHuman)
	last := t0.Add(10 * time.Minute)
	touch(t, f, last, eventbus.ActorHuman)
	if _, opened := touch(t, f, last.Add(session.DefaultIdle-time.Second), eventbus.ActorHuman); opened {
		t.Fatal("a session idle for less than the idle time was replaced")
	}
	last = last.Add(session.DefaultIdle - time.Second)
	// The next action comes well after the session became idle: the end is
	// dated when it became idle, not when the action came.
	second, opened := touch(t, f, last.Add(session.DefaultIdle+5*time.Minute), eventbus.ActorSim)
	if !opened || second.ID == first.ID || second.ActorKind != eventbus.ActorSim {
		t.Fatalf("Touch after the idle time = %+v opened %v", second, opened)
	}

	events := f.analytics(t)
	if got := types(events); strings.Join(got, ",") != "analytics.session.started,analytics.session.ended,analytics.session.started" {
		t.Fatalf("analytics = %v", got)
	}
	ended := events[1].Path()
	reason, _ := ended.GetString("session.end_reason")
	id, _ := ended.GetString("session.id")
	at, _ := ended.GetString("session.ended_at")
	if reason != session.EndIdle || id != first.ID || at != session.Timestamp(last.Add(session.DefaultIdle)) {
		t.Errorf("session.ended = %s %s at %s", id, reason, at)
	}
	if err := contracts.Validate(events[1]); err != nil {
		t.Errorf("session.ended is not valid by C-10: %v", err)
	}
}

// An action exactly MV_GATEWAY_SESSION_IDLE after the last one opens a new
// session, the way the sweeper ends a session at that moment (N-2 of review #1
// of T-306): the end of a session does not depend on whether the sweeper or
// the action comes first.
func TestAnActionAtTheIdleTimeStartsANewSession(t *testing.T) {
	f := newFixture(t, false)
	first, _ := touch(t, f, t0, eventbus.ActorHuman)
	last := t0.Add(session.DefaultIdle - time.Nanosecond)
	if _, opened := touch(t, f, last, eventbus.ActorHuman); opened {
		t.Fatal("an action a nanosecond before the idle time opened a new session")
	}
	second, opened := touch(t, f, last.Add(session.DefaultIdle), eventbus.ActorHuman)
	if !opened || second.ID == first.ID {
		t.Fatalf("Touch at the idle time = %+v opened %v, want a new session", second, opened)
	}
	events := f.analytics(t)
	if got := types(events); strings.Join(got, ",") != "analytics.session.started,analytics.session.ended,analytics.session.started" {
		t.Fatalf("analytics = %v", got)
	}
	if at, _ := events[1].Path().GetString("session.ended_at"); at != session.Timestamp(last.Add(session.DefaultIdle)) {
		t.Errorf("session.ended_at = %s, want %s", at, session.Timestamp(last.Add(session.DefaultIdle)))
	}
}

// The sweeper ends an idle session without waiting for the next action, and
// leaves a session that is not idle yet (NFR-036).
func TestTheSweeperEndsIdleSessionsWithoutAnAction(t *testing.T) {
	f := newFixture(t, false)
	touch(t, f, t0, eventbus.ActorHuman)
	other := eventbus.ScopeRef{ID: "player-B", Type: "solo"}
	if _, _, err := f.mgr.Touch(context.Background(), other, world, "player-B", eventbus.ActorHuman, t0.Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if n, err := f.mgr.Sweep(ctx, t0.Add(session.DefaultIdle-time.Nanosecond)); err != nil || n != 0 {
		t.Fatalf("Sweep before the idle time = %d %v", n, err)
	}
	if n, err := f.mgr.Sweep(ctx, t0.Add(session.DefaultIdle)); err != nil || n != 1 {
		t.Fatalf("Sweep at the idle time = %d %v, want 1", n, err)
	}
	if _, found, _ := f.mgr.Current(ctx, solo.ID); found {
		t.Error("the idle session is still active")
	}
	if _, found, _ := f.mgr.Current(ctx, other.ID); !found {
		t.Error("the session that is not idle was ended")
	}
	active, err := f.mgr.Active(ctx)
	if err != nil || len(active) != 1 || active[0].Scope.ID != other.ID {
		t.Errorf("Active = %+v %v", active, err)
	}
}

// Every session.started has its session.ended, however the sessions end: by
// a later action, by the sweeper, by /forget (NFR-036).
func TestStartsAndEndsArePaired(t *testing.T) {
	f := newFixture(t, false)
	ctx := context.Background()
	at := t0
	for range 3 {
		touch(t, f, at, eventbus.ActorHuman)
		at = at.Add(session.DefaultIdle + time.Minute)
	}
	if _, err := f.mgr.Sweep(ctx, at); err != nil {
		t.Fatal(err)
	}
	touch(t, f, at, eventbus.ActorHuman)
	if err := f.mgr.End(ctx, solo, session.EndForget, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := f.mgr.End(ctx, solo, session.EndForget, at.Add(2*time.Minute)); err != nil {
		t.Fatalf("End of a scope without a session: %v", err)
	}
	started, ended := map[string]int{}, map[string]int{}
	for _, ev := range f.analytics(t) {
		id, _ := ev.Path().GetString("session.id")
		switch ev.Type {
		case session.TypeStarted:
			started[id]++
		case session.TypeEnded:
			ended[id]++
		}
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("%s: %v", ev.Type, err)
		}
	}
	if len(started) != 4 || len(ended) != 4 {
		t.Fatalf("started %v, ended %v, want four pairs", started, ended)
	}
	for id, n := range started {
		if n != 1 || ended[id] != 1 {
			t.Errorf("session %s: started %d, ended %d", id, n, ended[id])
		}
	}
}

// A start the bus refuses takes its row back, and an end it refuses leaves the
// session active: the next action or sweep repeats it, and the pairs hold.
func TestAnAnalyticsEventThatIsNotPublishedIsTakenBack(t *testing.T) {
	f := newFixture(t, false)
	ctx := context.Background()
	f.pub.set(true)
	if _, _, err := f.mgr.Touch(ctx, solo, world, "player-A", eventbus.ActorHuman, t0); !errors.Is(err, session.ErrPublish) {
		t.Fatalf("Touch with the bus down = %v, want ErrPublish", err)
	}
	if _, found, _ := f.mgr.Current(ctx, solo.ID); found {
		t.Fatal("a session whose start was not published is active")
	}
	f.pub.set(false)
	touch(t, f, t0, eventbus.ActorHuman)
	f.pub.set(true)
	if n, err := f.mgr.Sweep(ctx, t0.Add(session.DefaultIdle)); !errors.Is(err, session.ErrPublish) || n != 0 {
		t.Fatalf("Sweep with the bus down = %d %v", n, err)
	}
	if _, found, _ := f.mgr.Current(ctx, solo.ID); !found {
		t.Fatal("a session whose end was not published is not active")
	}
	f.pub.set(false)
	if n, err := f.mgr.Sweep(ctx, t0.Add(session.DefaultIdle)); err != nil || n != 1 {
		t.Fatalf("Sweep after the bus came back = %d %v", n, err)
	}
	if got := types(f.analytics(t)); len(got) != 2 {
		t.Errorf("analytics = %v, want one started and one ended", got)
	}
}

// In replay the sessions are kept and nothing is published (C-10).
func TestReplayPublishesNoAnalytics(t *testing.T) {
	f := newFixture(t, true)
	ctx := context.Background()
	touch(t, f, t0, eventbus.ActorCI)
	if err := f.mgr.End(ctx, solo, session.EndLeave, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := f.analytics(t); len(got) != 0 {
		t.Errorf("replay published %v", types(got))
	}
	var state, reason string
	if err := f.db.QueryRowContext(ctx, `SELECT state, end_reason FROM sessions`).Scan(&state, &reason); err != nil ||
		state != session.StateEnded || reason != session.EndLeave {
		t.Errorf("session row = %s %s %v", state, reason, err)
	}
}

// The counters of a session are those session.ended reports; participants
// can be replaced while it is active.
func TestCountersAndParticipantsReachTheEnd(t *testing.T) {
	f := newFixture(t, false)
	ctx := context.Background()
	s, _ := touch(t, f, t0, eventbus.ActorHuman)
	for _, c := range []session.Counts{{Turns: 1}, {Turns: 1, Degraded: 1}, {Failed: 1}} {
		if err := session.CountTurn(ctx, f.db, s.ID, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.mgr.UpdateParticipants(ctx, solo, []string{"player-A", "player-B"}); err != nil {
		t.Fatal(err)
	}
	if cur, _, _ := f.mgr.Current(ctx, solo.ID); cur.TurnsCount != 2 || len(cur.Participants) != 2 {
		t.Fatalf("Current = %+v", cur)
	}
	if err := f.mgr.End(ctx, solo, session.EndDeath, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	events := f.analytics(t)
	pa := events[len(events)-1].Path()
	turns, _ := pa.GetInt("session.turns_count")
	degraded, _ := pa.GetInt("session.turns_degraded")
	failed, _ := pa.GetInt("session.turns_failed")
	players, _ := pa.GetInt("session.players_count")
	if turns != 2 || degraded != 1 || failed != 1 || players != 2 {
		t.Errorf("session.ended counters = %d %d %d players %d", turns, degraded, failed, players)
	}
}

func TestNewChecksItsConfig(t *testing.T) {
	if _, err := session.New(session.Config{}); err == nil {
		t.Error("New without a database succeeded")
	}
	if _, err := session.New(session.Config{DB: openDB(t), Idle: -time.Second}); err == nil {
		t.Error("New with a negative idle time succeeded")
	}
	m, err := session.New(session.Config{DB: openDB(t)})
	if err != nil || m.Idle() != session.DefaultIdle {
		t.Errorf("New with the defaults = %v, idle %v", err, m.Idle())
	}
}

// OnEnded hears every end that was recorded, in live and in replay, and no end
// the bus refused (the trigger of the snapshot, component §11.2).
func TestOnEndedHearsEveryRecordedEnd(t *testing.T) {
	for _, replay := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "replay"}[replay], func(t *testing.T) {
			f := newFixture(t, replay)
			var ended []string
			cfg := session.Config{DB: f.db, Bus: f.pub, OnEnded: func(s session.Session) {
				ended = append(ended, s.ID+" "+s.EndReason)
			}}
			if replay {
				cfg.Bus = nil
			}
			mgr, err := session.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			s, _, err := mgr.Touch(ctx, solo, world, "player-A", eventbus.ActorHuman, t0)
			if err != nil {
				t.Fatal(err)
			}
			if !replay {
				f.pub.set(true)
				if _, err := mgr.Sweep(ctx, t0.Add(session.DefaultIdle)); !errors.Is(err, session.ErrPublish) {
					t.Fatalf("Sweep with the bus down = %v", err)
				}
				if len(ended) != 0 {
					t.Fatalf("an end the bus refused was heard: %v", ended)
				}
				f.pub.set(false)
			}
			if n, err := mgr.Sweep(ctx, t0.Add(session.DefaultIdle)); err != nil || n != 1 {
				t.Fatalf("Sweep = %d %v", n, err)
			}
			if want := []string{s.ID + " " + session.EndIdle}; !slices.Equal(ended, want) {
				t.Errorf("OnEnded heard %v, want %v", ended, want)
			}
		})
	}
}
