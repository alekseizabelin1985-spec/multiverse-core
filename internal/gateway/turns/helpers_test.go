package turns_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

const (
	world   = "dark-forest-world"
	forest  = "dark-forest-01"
	playerA = "player-A"
)

var t0 = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// hooked is the bus of the tests: before runs ahead of each publication and
// may refuse it, after runs once it went out.
type hooked struct {
	*membus.Bus
	mu     sync.Mutex
	before func(ctx context.Context, ev eventbus.Event) error
	after  func(ctx context.Context, ev eventbus.Event)
}

func (h *hooked) set(before func(context.Context, eventbus.Event) error, after func(context.Context, eventbus.Event)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.before, h.after = before, after
}

func (h *hooked) Publish(ctx context.Context, ev eventbus.Event) error {
	h.mu.Lock()
	before, after := h.before, h.after
	h.mu.Unlock()
	if before != nil {
		if err := before(ctx, ev); err != nil {
			return err
		}
	}
	if err := h.Bus.Publish(ctx, ev); err != nil {
		return err
	}
	if after != nil {
		after(ctx, ev)
	}
	return nil
}

// records are the events of a topic, oldest first.
func (h *hooked) records(t *testing.T, topic string) []eventbus.Event {
	t.Helper()
	raw, err := h.Records(topic)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]eventbus.Event, 0, len(raw))
	for _, body := range raw {
		var ev eventbus.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatal(err)
		}
		out = append(out, ev)
	}
	return out
}

func (h *hooked) ofType(t *testing.T, topic, typ string) []eventbus.Event {
	t.Helper()
	var out []eventbus.Event
	for _, ev := range h.records(t, topic) {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

type fixture struct {
	db       *sql.DB
	bus      *hooked
	clock    *clock.Manual
	model    *readmodel.Model
	sessions *session.Manager
	tracker  *turns.Tracker
	svc      *actions.Service
	keys     *actions.Keys
}

type options struct {
	replay bool
	db     *sql.DB
	// publishTimeout bounds a publication of the tracker; zero is the default.
	publishTimeout time.Duration
	// log receives the log of the tracker; nil discards it.
	log *slog.Logger
}

func newFixture(t *testing.T, o options) *fixture {
	t.Helper()
	eventbus.SetIDSource(eventbus.SequenceIDs("ev"))
	t.Cleanup(func() { eventbus.SetIDSource(nil) })
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	raw, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: topics, Backoff: []time.Duration{0}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	f := &fixture{db: o.db, bus: &hooked{Bus: raw}, clock: clock.NewManual(t0)}
	if f.db == nil {
		f.db = openDB(t)
	}
	f.model = readmodel.New(readmodel.Config{Timers: f.clock.Timers()})
	for _, ev := range []eventbus.Event{
		fact(t, world, entity.TypeWorld, "Тёмный мир", map[string]any{"laws_version": "1", "locale": "ru"}),
		fact(t, forest, entity.TypeRegion, "Опушка", map[string]any{"description": "лес"}),
		fact(t, playerA, entity.TypePlayer, "Вася", map[string]any{
			"hp": 7, "hp_max": 10, "status": "alive", "position": actions.OutsidePosition(world),
			"scope": map[string]any{"id": playerA, "type": "solo"}, "actor_kind": "ci", "inventory": []any{},
		}),
	} {
		if _, err := f.model.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	f.build(t, o)
	return f
}

// build makes the sessions, the tracker and the service over the database of
// f; a second build stands for the process after a restart.
func (f *fixture) build(t *testing.T, o options) {
	t.Helper()
	var pub session.Publisher = f.bus
	if o.replay {
		pub = nil
	}
	var err error
	if f.sessions, err = session.New(session.Config{DB: f.db, Bus: pub}); err != nil {
		t.Fatal(err)
	}
	if f.tracker, err = turns.New(turns.Config{DB: f.db, Sessions: f.sessions, Bus: pub, Clock: f.clock,
		PublishTimeout: o.publishTimeout, Log: o.log}); err != nil {
		t.Fatal(err)
	}
	f.keys = actions.NewKeys(f.db, store.KeyTTL)
	if f.svc, err = actions.New(actions.Config{Bus: f.bus, Model: f.model, Keys: f.keys, Filter: actions.NoopFilter{},
		FilterName: "noop", Turns: f.tracker, Clock: f.clock, Timers: f.clock.Timers(),
		GMPath: eventbus.GMPathAgent, KeyTTL: store.KeyTTL}); err != nil {
		t.Fatal(err)
	}
}

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

func fact(t *testing.T, id, typ, name string, attrs map[string]any) eventbus.Event {
	t.Helper()
	payload := roundTrip(t, map[string]any{
		"entity":      map[string]any{"entity": map[string]any{"id": id, "type": typ}, "name": name},
		"version":     1,
		"attributes":  attrs,
		"proposal_id": "create-" + id,
	})
	return eventbus.Event{
		ID: "created-" + id, Type: readmodel.TypeEntityCreated, Timestamp: t0, Source: contracts.SourceState,
		World: &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: world, Type: entity.TypeWorld}},
		Meta: eventbus.Meta{SchemaVersion: 1, CorrelationID: "create-" + id, ActorKind: eventbus.ActorSystem,
			Locale: "ru", GMPath: eventbus.GMPathAgent},
		Payload: payload,
	}
}

func roundTrip(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// submit sends an action of player-A with the actor kind ci.
func (f *fixture) submit(t *testing.T, key, typ string, target string) actions.Answer {
	t.Helper()
	return f.svc.Submit(context.Background(), actions.Command{PlayerID: playerA, ActionKey: key, Type: typ, Target: target,
		ActorKind: eventbus.ActorCI})
}

func accepted(t *testing.T, a actions.Answer) api.ActionAccepted {
	t.Helper()
	if a.Accepted == nil {
		code := ""
		if a.Err != nil {
			code = a.Err.Code
		}
		t.Fatalf("answer = %d %s, want 202", a.Status, code)
	}
	return *a.Accepted
}

// narrative is narrative.output of a turn for the given recipients, as the
// swarm publishes it (C-05).
func narrative(t *testing.T, action eventbus.Event, generatedBy string, recipients ...string) eventbus.Event {
	t.Helper()
	list := make([]any, 0, len(recipients))
	for _, id := range recipients {
		list = append(list, map[string]any{"entity": map[string]any{"id": id, "type": entity.TypePlayer}})
	}
	payload := map[string]any{
		"recipients": list, "text": "Волк рычит.", "generated_by": generatedBy, "kind": "turn",
		"locale": "ru", "laws_version": "1",
		"filter": map[string]any{"applied": true, "status": "pass", "filter_version": "1"},
	}
	if generatedBy == turns.GeneratedByTemplate {
		payload["fallback_reason"] = "timeout"
	}
	return eventbus.Derive(action, "narrative.output", contracts.SourceSwarm, roundTrip(t, payload),
		eventbus.WithAgent(eventbus.AgentRef{ID: "player-gm:" + playerA, Level: "task", Blueprint: "player-gm"}),
		eventbus.WithCauseID("narrative"))
}

// actionEvent is the published player.* event of a correlation id.
func (f *fixture) actionEvent(t *testing.T, correlationID string) eventbus.Event {
	t.Helper()
	for _, ev := range f.bus.records(t, eventbus.TopicPlayerEvents) {
		if ev.ID == correlationID {
			return ev
		}
	}
	t.Fatalf("no player event %s", correlationID)
	return eventbus.Event{}
}

// inTx runs fn in a transaction of the database of f, as the consumer does.
func (f *fixture) inTx(t *testing.T, fn func(tx *sql.Tx) error) {
	t.Helper()
	tx, err := f.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// begun records a turn of player-A for an action of type typ the way the
// service of actions records an accepted one, without its preconditions — an
// attack needs no encounter here — and returns the event of the action.
func (f *fixture) begun(t *testing.T, typ string) eventbus.Event {
	t.Helper()
	ctx := context.Background()
	turn := actions.Turn{Scope: eventbus.ScopeRef{ID: playerA, Type: "solo"}, WorldID: world, PlayerID: playerA,
		Name: "Вася", Type: typ, ActorKind: eventbus.ActorCI, At: f.clock.Now()}
	ref, err := f.tracker.Begin(ctx, turn)
	if err != nil {
		t.Fatal(err)
	}
	action := eventbus.NewRoot(actions.Rules[typ].Event, contracts.SourceGateway, world, nil, eventbus.ActorCI, map[string]any{})
	if err := f.tracker.Accepted(ctx, turn, ref, action.ID); err != nil {
		t.Fatal(err)
	}
	return action
}

// decided is a combat.decided of the chain of action, with the mode and the
// level of detail of Phase 1.
func decided(action eventbus.Event, cause string) eventbus.Event {
	return eventbus.Derive(action, "combat.decided", contracts.SourceSwarm,
		map[string]any{"phase1_mode": "rules", "lod": "full"}, eventbus.WithCauseID(cause))
}
