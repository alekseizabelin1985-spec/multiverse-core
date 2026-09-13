package actions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

const (
	world      = "dark-forest-world"
	otherWorld = "other-world"
	forest     = "dark-forest-01"
	farRegion  = "other-region"
)

var t0 = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// The characters of the world of the tests:
//   - player-A alive, outside every region, in no encounter;
//   - player-B alive in the forest, in encounter-1 against wolf-alpha and
//     wolf-dead, whose task agent runs;
//   - player-C alive in the forest, in encounter-2, created at t0 and still
//     without a task agent;
//   - player-D dead;
//   - player-F abandoned, as /forget leaves a character (C-08 v1.2).
const (
	playerA = "player-A"
	playerB = "player-B"
	playerC = "player-C"
	playerD = "player-D"
	playerF = "player-F"
)

// fact builds entity.created as the wire delivers it.
func fact(t *testing.T, worldID, id, typ, name string, attrs map[string]any) eventbus.Event {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"entity":      map[string]any{"entity": map[string]any{"id": id, "type": typ}, "name": name},
		"version":     1,
		"attributes":  attrs,
		"proposal_id": "create-" + id,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return eventbus.Event{
		ID: "created-" + id, Type: readmodel.TypeEntityCreated, Timestamp: t0, Source: contracts.SourceState,
		World: &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: worldID, Type: entity.TypeWorld}},
		Meta: eventbus.Meta{SchemaVersion: 1, CorrelationID: "create-" + id, ActorKind: eventbus.ActorSystem,
			Locale: "ru", GMPath: eventbus.GMPathAgent},
		Payload: payload,
	}
}

func player(id, position string) map[string]any {
	return map[string]any{
		"hp": 7, "hp_max": 10, "atk": 2, "def": 12, "dmg": "d6", "flee": "2",
		"status": "alive", "position": position,
		"scope":      map[string]any{"id": id, "type": "solo"},
		"actor_kind": "human", "inventory": []any{},
	}
}

func npc(status string) map[string]any {
	return map[string]any{"kind": "wolf", "region_id": forest, "position": forest, "status": status, "hp": 10, "hp_max": 10}
}

func encounter(playerID, agent string, npcs ...string) map[string]any {
	list := make([]any, 0, len(npcs))
	for _, id := range npcs {
		list = append(list, map[string]any{"npc_id": id})
	}
	attrs := map[string]any{
		"region_id": forest, "state": "active", "round_seq": 0,
		"scope":        map[string]any{"id": playerID, "type": "solo"},
		"participants": []any{map[string]any{"player_id": playerID, "state": "active", "damage_dealt": 0}},
		"npcs":         list,
	}
	if agent != "" {
		attrs["task_agent_id"] = agent
	}
	return attrs
}

// newWorld is the projection of the world of the tests.
func newWorld(t *testing.T) *readmodel.Model {
	t.Helper()
	manual := clock.NewManual(t0)
	m := readmodel.New(readmodel.Config{Timers: manual.Timers()})
	dead := player(playerD, forest)
	dead["status"] = entity.StatusDead
	abandoned := player(playerF, actions.OutsidePosition(world))
	abandoned["status"] = entity.StatusAbandoned
	for _, ev := range []eventbus.Event{
		fact(t, world, world, entity.TypeWorld, "Тёмный лес", map[string]any{"laws_version": "1", "locale": "ru"}),
		fact(t, world, forest, entity.TypeRegion, "Опушка", map[string]any{"description": "лес"}),
		fact(t, otherWorld, farRegion, entity.TypeRegion, "Далеко", map[string]any{"description": "не здесь"}),
		fact(t, world, "wolf-alpha", entity.TypeNPC, "Альфа-волк", npc(entity.StatusAlive)),
		fact(t, world, "wolf-dead", entity.TypeNPC, "Мёртвый волк", npc(entity.StatusDead)),
		fact(t, world, "wolf-far", entity.TypeNPC, "Далёкий волк", npc(entity.StatusAlive)),
		fact(t, world, playerA, entity.TypePlayer, "Вася", player(playerA, actions.OutsidePosition(world))),
		fact(t, world, playerB, entity.TypePlayer, "Петя", player(playerB, forest)),
		fact(t, world, playerC, entity.TypePlayer, "Коля", player(playerC, forest)),
		fact(t, world, playerD, entity.TypePlayer, "Гена", dead),
		fact(t, world, playerF, entity.TypePlayer, "Фёдор", abandoned),
		fact(t, world, "encounter-1", entity.TypeEncounter, "", encounter(playerB, "encounter-wolf:solo:player-B", "wolf-alpha", "wolf-dead")),
		fact(t, world, "encounter-2", entity.TypeEncounter, "", encounter(playerC, "", "wolf-alpha", "wolf-dead")),
	} {
		if _, err := m.Apply(ev); err != nil {
			t.Fatalf("Apply %s: %v", ev.ID, err)
		}
	}
	return m
}

func text(s string) *string { return &s }

// newBus is membus over the contract registry with every topic.
func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: topics, Backoff: []time.Duration{0}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

// newKeys is the store of action keys over a migrated gateway.db.
func newKeys(t *testing.T) *actions.Keys {
	t.Helper()
	ctx := context.Background()
	dir := sqlitedir.Temp(t)
	db, err := store.OpenGateway(ctx, store.GatewayPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.MigrateGateway(ctx, db); err != nil {
		t.Fatal(err)
	}
	return actions.NewKeys(db, store.KeyTTL)
}

// recorder is the bus a service under test publishes to: it records every
// attempt and what was published, in order, fails a publication while fail
// is set, and runs before and after around each publication when they are set.
type recorder struct {
	eventbus.Bus
	mu       sync.Mutex
	events   []eventbus.Event
	attempts []eventbus.Event
	fail     func(ev eventbus.Event) bool
	// before runs ahead of the publication; its error fails it.
	before func(ctx context.Context, ev eventbus.Event) error
	// after runs once the publication succeeded.
	after func(ev eventbus.Event)
}

var errBusDown = errors.New("broker down")

func (r *recorder) Publish(ctx context.Context, ev eventbus.Event) error {
	r.mu.Lock()
	r.attempts = append(r.attempts, ev)
	fail := r.fail != nil && r.fail(ev)
	before, after := r.before, r.after
	r.mu.Unlock()
	if fail {
		return errBusDown
	}
	if before != nil {
		if err := before(ctx, ev); err != nil {
			return err
		}
	}
	if err := r.Bus.Publish(ctx, ev); err != nil {
		return err
	}
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
	if after != nil {
		after(ev)
	}
	return nil
}

func (r *recorder) published() []eventbus.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]eventbus.Event(nil), r.events...)
}

func (r *recorder) attempted() []eventbus.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]eventbus.Event(nil), r.attempts...)
}

func (r *recorder) setFail(f func(eventbus.Event) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fail = f
}

func (r *recorder) setHooks(before func(context.Context, eventbus.Event) error, after func(eventbus.Event)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.before, r.after = before, after
}

// turns records the calls of the service.
type turns struct {
	inner    *actions.MemoryTurns
	mu       sync.Mutex
	accepted []string
	rejected []string
	// acceptedOn is the error of the context of each call of Accepted: T-306
	// records the turn in gateway.db, where a cancelled context fails the write.
	acceptedOn []error
}

func (f *turns) Begin(ctx context.Context, t actions.Turn) (api.TurnRef, error) {
	return f.inner.Begin(ctx, t)
}

func (f *turns) Accepted(ctx context.Context, t actions.Turn, ref api.TurnRef, eventID string) error {
	f.mu.Lock()
	f.accepted = append(f.accepted, eventID)
	f.acceptedOn = append(f.acceptedOn, ctx.Err())
	f.mu.Unlock()
	return f.inner.Accepted(ctx, t, ref, eventID)
}

func (f *turns) Rejected(_ context.Context, _ actions.Turn, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejected = append(f.rejected, code)
	return nil
}

// fixture is a service of actions over the world of the tests.
type fixture struct {
	svc   *actions.Service
	bus   *recorder
	raw   *membus.Bus
	model *readmodel.Model
	clock *clock.Manual
	keys  *actions.Keys
	turns *turns
	log   *syncBuffer
}

type options struct {
	gmPath       string
	filter       actions.InputFilter
	keys         actions.KeyStore
	pendingLimit int
}

func newFixture(t *testing.T, o options) fixture {
	t.Helper()
	eventbus.SetIDSource(eventbus.SequenceIDs("ev"))
	eventbus.SetClock(nil)
	t.Cleanup(func() { eventbus.SetIDSource(nil) })
	raw := newBus(t)
	f := fixture{bus: &recorder{Bus: raw}, raw: raw, model: newWorld(t), clock: clock.NewManual(t0),
		keys: newKeys(t), turns: &turns{inner: actions.NewMemoryTurns()}, log: &syncBuffer{}}
	f.svc = f.newService(t, o)
	return f
}

// newService is a service over the bus, projection, keys, turns and clock of
// the fixture: the service of newFixture, or a second one standing for the
// process after a restart.
func (f fixture) newService(t *testing.T, o options) *actions.Service {
	t.Helper()
	if o.gmPath == "" {
		o.gmPath = eventbus.GMPathAgent
	}
	if o.filter == nil {
		o.filter = actions.NoopFilter{}
	}
	var keys actions.KeyStore = f.keys
	if o.keys != nil {
		keys = o.keys
	}
	svc, err := actions.New(actions.Config{
		Bus: f.bus, Model: f.model, Keys: keys, Filter: o.filter, FilterName: "test",
		Turns: f.turns, Clock: f.clock, Timers: f.clock.Timers(), GMPath: o.gmPath, Grace: 10 * time.Second,
		KeyTTL: store.KeyTTL, PendingLimit: o.pendingLimit,
		Log: slog.New(slog.NewJSONHandler(f.log, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// lines are the records of the log with the given message.
func (b *syncBuffer) lines(t *testing.T, msg string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(b.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		if m["msg"] == msg {
			out = append(out, m)
		}
	}
	return out
}
