package consumer_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

const world = "dark-forest-world"

var t0 = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

type fixture struct {
	bus   *membus.Bus
	db    *sql.DB
	model *readmodel.Model
	clock *clock.Manual
}

func newFixture(t *testing.T, chaos membus.Chaos) *fixture {
	t.Helper()
	testkit.Deterministic(t, "ev")
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{
		Registry: contracts.Default(), Topics: topics, Backoff: []time.Duration{0, 0, 0}, Chaos: chaos,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	ctx := context.Background()
	db, err := store.OpenGateway(ctx, store.GatewayPath(sqlitedir.Temp(t)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.MigrateGateway(ctx, db); err != nil {
		t.Fatal(err)
	}
	// The effect of the tests writes here, through the transaction it is given.
	if _, err := db.ExecContext(ctx, `CREATE TABLE effects_log (event_id TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	manual := clock.NewManual(t0)
	return &fixture{bus: bus, db: db, model: readmodel.New(readmodel.Config{Timers: manual.Timers()}), clock: manual}
}

func (f *fixture) dispatcher(t *testing.T, bus eventbus.Bus, effects map[string][]consumer.Effect) *consumer.Dispatcher {
	t.Helper()
	if bus == nil {
		bus = f.bus
	}
	d, err := consumer.New(consumer.Config{
		Bus: bus, Journal: f.bus, DB: f.db, Model: f.model, Clock: f.clock,
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)), Effects: effects,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func (f *fixture) start(t *testing.T, d *consumer.Dispatcher, from map[string]int64) {
	t.Helper()
	if err := d.Start(context.Background(), from); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := d.Stop(ctx); err != nil {
			t.Errorf("Stop: %v", err)
		}
	})
}

func (f *fixture) publish(t *testing.T, events ...eventbus.Event) {
	t.Helper()
	for _, ev := range events {
		if err := f.bus.Publish(context.Background(), ev); err != nil {
			t.Fatalf("publish %s: %v", ev.Type, err)
		}
	}
}

func (f *fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *fixture) cursor(t *testing.T, topic string) (int64, bool) {
	t.Helper()
	cursors, err := consumer.Cursors(context.Background(), f.db)
	if err != nil {
		t.Fatal(err)
	}
	offset, ok := cursors[topic]
	return offset, ok
}

func created(id string, hp int) eventbus.Event {
	return eventbus.NewRoot(readmodel.TypeEntityCreated, contracts.SourceState, world, nil, eventbus.ActorSystem,
		map[string]any{
			"entity":  map[string]any{"entity": map[string]any{"id": id, "type": entity.TypePlayer}, "name": id},
			"version": 1, "proposal_id": "create-" + id,
			"attributes": map[string]any{"hp": hp, "hp_max": 10, "status": "alive"},
		})
}

func updated(id string, version, oldHP, newHP int) eventbus.Event {
	return eventbus.NewRoot(readmodel.TypeEntityUpdated, contracts.SourceState, world, nil, eventbus.ActorSystem,
		map[string]any{
			"entity":  map[string]any{"entity": map[string]any{"id": id, "type": entity.TypePlayer}, "name": id},
			"version": version, "cause": "combat", "proposal_id": fmt.Sprintf("p-%s-%d", id, version),
			"applied_at": t0.Format(time.RFC3339),
			"changed":    []any{map[string]any{"path": "hp", "old": oldHP, "new": newHP}},
		})
}

// logging is an effect that writes the id of its event through the
// transaction of the consumer and counts its calls.
func logging(calls *atomic.Int32, fail func(n int32) error) consumer.Effect {
	return func(ctx context.Context, tx *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
		n := calls.Add(1)
		if _, err := tx.ExecContext(ctx, `INSERT INTO effects_log (event_id) VALUES (?)`, ev.ID); err != nil {
			return err
		}
		if fail != nil {
			return fail(n)
		}
		return nil
	}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.After(15 * time.Second)
	for !cond() {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		case <-testkit.After(2 * time.Millisecond):
		}
	}
}

// The bus delivers at least once; the same event twice takes its effects
// once, and the projection applies it once (NFR-013).
func TestADuplicateDeliveryTakesItsEffectsOnce(t *testing.T) {
	f := newFixture(t, membus.Chaos{Duplicate: true})
	var calls atomic.Int32
	effect := logging(&calls, nil)
	d := f.dispatcher(t, nil, map[string][]consumer.Effect{
		readmodel.TypeEntityCreated: {effect}, readmodel.TypeEntityUpdated: {effect},
	})
	f.start(t, d, nil)
	create, hit := created("player-A", 10), updated("player-A", 2, 10, 7)
	f.publish(t, create, hit)

	eventually(t, "the cursor to pass both copies", func() bool {
		offset, ok := f.cursor(t, eventbus.TopicSystemEvents)
		return ok && offset == 3
	})
	if n := calls.Load(); n != 2 {
		t.Errorf("the effects ran %d times for two events delivered twice each", n)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM effects_log`); n != 2 {
		t.Errorf("%d effect rows, want 2", n)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events`); n != 2 {
		t.Errorf("%d marks processed, want 2", n)
	}
	if c, _ := f.model.Character("player-A"); c.HP != 7 || c.Version != 2 {
		t.Errorf("projection %+v, want hp 7 at version 2", c)
	}
	if next := f.model.Cursor()[eventbus.TopicSystemEvents]; next != 4 {
		t.Errorf("projection cursor %d, want 4", next)
	}
}

var errEffect = errors.New("effect failed")

// The effect, the mark "processed" and the cursor commit together, and only
// after the effect succeeded: a failed attempt leaves none of the three, and
// the retry of the bus takes the effect (ADR-027; a consumer that marked the
// event before its effect would lose it here).
func TestAFailedEffectLeavesTheEventForTheRetry(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	var calls atomic.Int32
	d := f.dispatcher(t, nil, map[string][]consumer.Effect{
		readmodel.TypeEntityCreated: {logging(&calls, func(n int32) error {
			if n <= 2 {
				return errEffect
			}
			return nil
		})},
	})
	f.start(t, d, nil)
	f.publish(t, created("player-A", 10))

	eventually(t, "the cursor to move", func() bool {
		_, ok := f.cursor(t, eventbus.TopicSystemEvents)
		return ok
	})
	if n := calls.Load(); n != 3 {
		t.Errorf("the effect ran %d times, want two failures and one success", n)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM effects_log`); n != 1 {
		t.Errorf("%d effect rows: a failed attempt committed its writes", n)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events`); n != 1 {
		t.Errorf("%d marks processed, want 1", n)
	}
	if letters, _ := f.bus.DeadLetters(); len(letters) != 0 {
		t.Errorf("%d dead letters for an event whose retry succeeded", len(letters))
	}
}

// An effect that keeps failing: the error reaches the bus, which parks the
// event, and the event stays unprocessed — no mark, no cursor, no effect —
// so a later delivery takes it.
func TestAnEffectErrorReachesTheBusAndMarksNothing(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	var calls atomic.Int32
	var broken atomic.Bool
	broken.Store(true)
	d := f.dispatcher(t, nil, map[string][]consumer.Effect{
		readmodel.TypeEntityCreated: {logging(&calls, func(int32) error {
			if broken.Load() {
				return errEffect
			}
			return nil
		})},
	})
	f.start(t, d, nil)
	ev := created("player-A", 10)
	f.publish(t, ev)

	eventually(t, "the event to be parked", func() bool {
		letters, _ := f.bus.DeadLetters()
		return len(letters) == 1
	})
	if letters, _ := f.bus.DeadLetters(); !strings.Contains(letters[0].Error, errEffect.Error()) {
		t.Errorf("dead letter error %q, want the error of the effect", letters[0].Error)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events`) + f.count(t, `SELECT COUNT(*) FROM cursors`) +
		f.count(t, `SELECT COUNT(*) FROM effects_log`); n != 0 {
		t.Errorf("%d rows of mark, cursor or effect after the effect failed", n)
	}

	pos := eventbus.ContextWithPosition(context.Background(), eventbus.Position{Topic: eventbus.TopicSystemEvents, Offset: 0})
	if err := d.Handle(pos, ev); !errors.Is(err, errEffect) {
		t.Errorf("Handle = %v, want the error of the effect returned", err)
	}
	broken.Store(false)
	if err := d.Handle(pos, ev); err != nil {
		t.Fatalf("the repeat after the effect recovered: %v", err)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM effects_log`); n != 1 {
		t.Errorf("%d effect rows after the repeat, want 1", n)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events WHERE event_id = ?`, ev.ID); n != 1 {
		t.Errorf("the repeat did not mark the event processed")
	}
}

// Events at or behind the effects cursor of gateway.db — what the gateway
// already did before it restarted — go to the projection only.
func TestEventsBehindTheEffectsCursorReachTheProjectionOnly(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	events := []eventbus.Event{created("player-A", 10), updated("player-A", 2, 10, 8), updated("player-A", 3, 8, 5)}
	f.publish(t, events...)
	if _, err := f.db.ExecContext(context.Background(),
		`INSERT INTO cursors (topic, "offset", event_id, updated_at) VALUES (?, 1, ?, '2026-09-13T11:00:00.000000000Z')`,
		eventbus.TopicSystemEvents, events[1].ID); err != nil {
		t.Fatal(err)
	}
	var seen []string
	var mu sync.Mutex
	effect := func(_ context.Context, _ *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, ev.ID)
		return nil
	}
	d := f.dispatcher(t, nil, map[string][]consumer.Effect{
		readmodel.TypeEntityCreated: {effect}, readmodel.TypeEntityUpdated: {effect},
	})
	f.start(t, d, nil)

	if c, _ := f.model.Character("player-A"); c.HP != 5 || c.Version != 3 {
		t.Errorf("projection after the catch-up %+v, want every fact applied", c)
	}
	eventually(t, "the subscription to reach the end", func() bool {
		offset, _ := f.cursor(t, eventbus.TopicSystemEvents)
		return offset == 2
	})
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 || seen[0] != events[2].ID {
		t.Errorf("effects ran for %v, want only the event past the cursor", seen)
	}
}

// The two cursors are separate: an event behind the effects cursor takes no
// effect, and it still moves the cursor of the projection, which names what the
// projection holds — the cursor the snapshot of the gateway will record
// (T-309). Closed at acceptance: moving the projection cursor after the check
// of the effects cursor left every other test green.
func TestEventsBehindTheEffectsCursorMoveTheProjectionCursor(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	events := []eventbus.Event{created("player-A", 10), updated("player-A", 2, 10, 8)}
	f.publish(t, events...)
	if _, err := f.db.ExecContext(context.Background(),
		`INSERT INTO cursors (topic, "offset", event_id, updated_at) VALUES (?, 1, ?, '2026-09-13T11:00:00.000000000Z')`,
		eventbus.TopicSystemEvents, events[1].ID); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	d := f.dispatcher(t, &quietBus{Bus: f.bus}, map[string][]consumer.Effect{
		readmodel.TypeEntityCreated: {logging(&calls, nil)}, readmodel.TypeEntityUpdated: {logging(&calls, nil)},
	})
	f.start(t, d, nil)

	if n := calls.Load(); n != 0 {
		t.Errorf("the effects of %d events behind the effects cursor ran", n)
	}
	if c, _ := f.model.Character("player-A"); c.HP != 8 || c.Version != 2 {
		t.Errorf("projection %+v, want both facts applied", c)
	}
	if next := f.model.Cursor()[eventbus.TopicSystemEvents]; next != 2 {
		t.Errorf("projection cursor %d, want 2: both events are in the projection", next)
	}
	if offset, _ := f.cursor(t, eventbus.TopicSystemEvents); offset != 1 {
		t.Errorf("effects cursor %d, want it left at 1", offset)
	}
}

// quietBus subscribes and delivers nothing: what the projection holds when
// Start returns came from the journal.
type quietBus struct {
	eventbus.Bus
	active atomic.Int32
	fail   string
}

func (b *quietBus) Subscribe(ctx context.Context, topic, _ string, _ eventbus.Handler) error {
	if topic == b.fail {
		return errors.New("broker gone")
	}
	b.active.Add(1)
	defer b.active.Add(-1)
	<-ctx.Done()
	return nil
}

func TestStartCatchesUpFromTheCursorOfTheSnapshot(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	f.publish(t, created("player-A", 10), updated("player-A", 2, 10, 8), updated("player-A", 3, 8, 5))
	// The snapshot held player-A at version 1 and was taken at offset 1.
	if _, err := f.model.Apply(created("player-A", 10)); err != nil {
		t.Fatal(err)
	}
	bus := &quietBus{Bus: f.bus}
	var calls atomic.Int32
	d := f.dispatcher(t, bus, map[string][]consumer.Effect{
		readmodel.TypeEntityCreated: {logging(&calls, nil)}, readmodel.TypeEntityUpdated: {logging(&calls, nil)},
	})
	f.start(t, d, map[string]int64{eventbus.TopicSystemEvents: 1})

	if c, _ := f.model.Character("player-A"); c.HP != 5 || c.Version != 3 {
		t.Errorf("when Start returned the projection was %+v, want the facts after the snapshot", c)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("the catch-up took the effects of %d events, want the two after the cursor", n)
	}
	if next := f.model.Cursor()[eventbus.TopicSystemEvents]; next != 3 {
		t.Errorf("projection cursor %d, want 3", next)
	}
	if p, _ := f.model.Status(); p == readmodel.ProjectionStale {
		t.Error("the catch-up left the projection stale")
	}
}

func TestStopEndsTheSubscriptionsAndAFailedOneIsReported(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	bus := &quietBus{Bus: f.bus, fail: eventbus.TopicNarrativeOutput}
	d := f.dispatcher(t, bus, nil)
	if err := d.Start(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "three subscriptions to run", func() bool { return bus.active.Load() == 3 })
	eventually(t, "the failed subscription to be reported", func() bool { return d.Err() != nil })
	if !strings.Contains(d.Err().Error(), eventbus.TopicNarrativeOutput) {
		t.Errorf("Err = %v, want it to name the topic", d.Err())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := d.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if n := bus.active.Load(); n != 0 {
		t.Errorf("%d subscriptions still run after Stop", n)
	}
	if err := d.Stop(ctx); err != nil {
		t.Errorf("a second Stop: %v", err)
	}
}

func TestAnEventWithoutAPositionOrAnIDIsRefused(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	d := f.dispatcher(t, nil, nil)
	ev := created("player-A", 10)
	if err := d.Handle(context.Background(), ev); err == nil {
		t.Error("an event without a position was handled")
	}
	ev.ID = ""
	pos := eventbus.ContextWithPosition(context.Background(), eventbus.Position{Topic: eventbus.TopicSystemEvents})
	if err := d.Handle(pos, ev); err == nil {
		t.Error("an event without an id was handled")
	}
	if _, ok := f.model.Character("player-A"); ok {
		t.Error("a refused event reached the projection")
	}
}

func TestSweepForgetsOldMarksAndKeepsTheWindowBounded(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	d := f.dispatcher(t, nil, nil)
	ctx := context.Background()
	stamp := func(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05.000000000Z") }
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	insert := func(id string, at time.Time) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO processed_events (event_id, topic, processed_at) VALUES (?, ?, ?)`,
			id, eventbus.TopicSystemEvents, stamp(at)); err != nil {
			t.Fatal(err)
		}
	}
	insert("old", t0.Add(-store.ProcessedEventsKeep-time.Second))
	insert("young", t0.Add(-store.ProcessedEventsKeep+time.Second))
	for i := range store.ProcessedEventsMax {
		insert(fmt.Sprintf("fill-%06d", i), t0.Add(-time.Hour).Add(time.Duration(i)*time.Millisecond))
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := d.Sweep(ctx, t0); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events WHERE event_id = 'old'`); n != 0 {
		t.Error("a mark older than the retention survived the sweep")
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events`); n != store.ProcessedEventsMax {
		t.Errorf("%d marks after the sweep, want the bound %d", n, store.ProcessedEventsMax)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events WHERE event_id = 'young'`); n != 0 {
		t.Error("the oldest mark within the retention survived the bound on the count")
	}
}

// Below the bound on the count only the age decides: a mark past the retention
// goes, a mark within it stays (review #1, Mi-5).
func TestSweepForgetsAMarkByItsAgeAlone(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	d := f.dispatcher(t, nil, nil)
	ctx := context.Background()
	stamp := func(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05.000000000Z") }
	for id, at := range map[string]time.Time{
		"old":   t0.Add(-store.ProcessedEventsKeep - time.Second),
		"young": t0.Add(-store.ProcessedEventsKeep + time.Second),
	} {
		if _, err := f.db.ExecContext(ctx, `INSERT INTO processed_events (event_id, topic, processed_at) VALUES (?, ?, ?)`,
			id, eventbus.TopicSystemEvents, stamp(at)); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Sweep(ctx, t0); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events WHERE event_id = 'old'`); n != 0 {
		t.Error("a mark older than the retention survived the sweep")
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events WHERE event_id = 'young'`); n != 1 {
		t.Error("a mark within the retention was swept below the bound on the count")
	}
}

// tailJournal publishes one more event right after the catch-up read the end of
// topic: the tail that arrives between End and the subscriptions.
type tailJournal struct {
	*membus.Bus
	topic string
	tail  eventbus.Event
	once  sync.Once
	t     *testing.T
}

func (j *tailJournal) End(ctx context.Context, topic string) (int64, error) {
	end, err := j.Bus.End(ctx, topic)
	if err == nil && topic == j.topic {
		j.once.Do(func() {
			if err := j.Publish(ctx, j.tail); err != nil {
				j.t.Errorf("publish the tail: %v", err)
			}
		})
	}
	return end, err
}

// The catch-up reads each topic to the End it saw and only then goes live
// (component §11.2, C-01 v1.1). The tail published after End reaches the
// gateway through the subscription, and the subscription repeating the
// catch-up takes no effect twice (component §16 p. 7).
func TestTheCatchUpGoesLiveAtTheEndAndTheTailTakesItsEffectsOnce(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	events := []eventbus.Event{created("player-A", 10), updated("player-A", 2, 10, 8)}
	f.publish(t, events...)
	tail := updated("player-A", 3, 8, 5)
	journal := &tailJournal{Bus: f.bus, topic: eventbus.TopicSystemEvents, tail: tail, t: t}

	var mu sync.Mutex
	seen := map[string]int{}
	effect := func(_ context.Context, _ *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
		mu.Lock()
		defer mu.Unlock()
		seen[ev.ID]++
		return nil
	}
	d, err := consumer.New(consumer.Config{
		Bus: f.bus, Journal: journal, DB: f.db, Model: f.model, Clock: f.clock,
		Log:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Effects: map[string][]consumer.Effect{readmodel.TypeEntityCreated: {effect}, readmodel.TypeEntityUpdated: {effect}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Live() {
		t.Fatal("live before Start")
	}
	f.start(t, d, nil)
	if !d.Live() {
		t.Fatal("not live after the catch-up")
	}
	if end := d.Ends()[eventbus.TopicSystemEvents]; end != 2 {
		t.Errorf("the catch-up read system_events to %d, want the End it saw, 2", end)
	}
	eventually(t, "the tail to arrive through the subscription", func() bool {
		offset, _ := f.cursor(t, eventbus.TopicSystemEvents)
		return offset == 2
	})
	// Let the subscription repeat what it has; nothing more may happen.
	eventually(t, "the projection to hold the tail", func() bool {
		c, _ := f.model.Character("player-A")
		return c.Version == 3
	})
	mu.Lock()
	defer mu.Unlock()
	for _, ev := range append(events, tail) {
		if seen[ev.ID] != 1 {
			t.Errorf("effects of %s ran %d times, want once", ev.ID, seen[ev.ID])
		}
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events`); n != 3 {
		t.Errorf("%d events marked processed, want 3", n)
	}
}

// A topic no snapshot names is caught up from its effects cursor before Start
// returns: the effects the gateway had not taken when it stopped happen before
// its first request, not whenever the subscription gets to them.
func TestATopicWithAnEffectsCursorIsCaughtUpBeforeTheSubscriptions(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	scope := &eventbus.ScopeRef{ID: "player-A", Type: "solo"}
	first, second := combatEvent("c-1", scope), combatEvent("c-2", scope)
	f.publish(t, first, second)
	if _, err := f.db.ExecContext(context.Background(),
		`INSERT INTO cursors (topic, "offset", event_id, updated_at) VALUES (?, 0, ?, '2026-09-13T11:00:00.000000000Z')`,
		eventbus.TopicGameEvents, first.ID); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var got []string
	var mu sync.Mutex
	effect := func(_ context.Context, _ *sql.Tx, ev eventbus.Event, _ readmodel.Result) error {
		calls.Add(1)
		mu.Lock()
		defer mu.Unlock()
		got = append(got, ev.ID)
		return nil
	}
	d := f.dispatcher(t, &quietBus{Bus: f.bus}, map[string][]consumer.Effect{"combat.decided": {effect}})
	f.start(t, d, map[string]int64{eventbus.TopicSystemEvents: 0})
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0] != second.ID {
		t.Errorf("when Start returned the effects ran for %v, want only the event past the cursor", got)
	}
	if end, ok := d.Ends()[eventbus.TopicGameEvents]; !ok || end != 2 {
		t.Errorf("game_events read to %d (%v), want 2", end, ok)
	}
	if _, ok := d.Ends()[eventbus.TopicNarrativeOutput]; ok {
		t.Error("a topic with neither a snapshot cursor nor an effects cursor was read by the catch-up")
	}
}

func combatEvent(id string, scope *eventbus.ScopeRef) eventbus.Event {
	ev := eventbus.NewRoot("combat.decided", contracts.SourceSwarm, world, scope, eventbus.ActorSystem, map[string]any{
		"encounter": map[string]any{"entity": map[string]any{"id": "enc-1", "type": entity.TypeEncounter}},
		"round":     map[string]any{"seq": 1}, "action": "attack",
		"attacker": map[string]any{"entity": map[string]any{"id": "player-A", "type": entity.TypePlayer}},
		"defender": map[string]any{"entity": map[string]any{"id": "wolf-1", "type": entity.TypeNPC}},
		"outcome":  map[string]any{"hit": true, "natural": 15, "damage": 3},
		"hp":       map[string]any{"defender_before": 10, "defender_after": 7, "defender_max": 10},
		"rolls":    []any{}, "rules_version": "1", "phase1_mode": "rules",
	})
	ev.Meta.CorrelationID = "act-" + id
	ev.Meta.Agent = &eventbus.AgentRef{ID: "encounter-wolf:solo:player-A", Level: "task", Blueprint: "encounter-wolf"}
	return ev
}

func stateSnapshotEvent(seq int) eventbus.Event {
	return eventbus.NewRoot(readmodel.TypeSnapshotCreated, contracts.SourceState, world, nil, eventbus.ActorSystem, map[string]any{
		"component": "state",
		"snapshot": map[string]any{"id": fmt.Sprintf("state:%s:%06d", world, seq), "seq": seq, "taken_at": "2026-09-13T12:00:00Z",
			"cursor": map[string]any{"system_events": seq}, "laws_version": "v1",
			"state_hash": "sha256:" + strings.Repeat("0", 64), "size_bytes": 1, "key": fmt.Sprintf("state/20260913T120000Z-%06d.json", seq)},
	})
}

// / The repair of stale entities in the subscription runs with a snapshot.created
// the projection has not seen, before the event moves the cursor of the
// projection or the effects cursor. In the catch-up it runs once, with the last
// announcement Repairs accepts, after the journal was read: the earlier
// announcements and the one Repairs refuses are not read from, and the
// subscription repeating the catch-up repairs nothing again (component §11.2;
// Mi-1 of review #1 of T-309).
func TestTheRepairRunsBeforeTheCursorsMove(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	refused := stateSnapshotEvent(3)
	refused.World = &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: "another-world", Type: entity.TypeWorld}}
	second := stateSnapshotEvent(2)
	f.publish(t, created("player-A", 10), stateSnapshotEvent(1), second, refused)
	type call struct {
		id                          string
		positioned                  bool
		offset, projection, effects int64
		hasEffects                  bool
	}
	var mu sync.Mutex
	var calls []call
	d, err := consumer.New(consumer.Config{
		Bus: f.bus, Journal: f.bus, DB: f.db, Model: f.model, Clock: f.clock,
		Log:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Repairs: func(ev eventbus.Event) bool { return ev.World != nil && ev.World.Entity.ID == world },
		Repair: func(ctx context.Context, ev eventbus.Event) {
			pos, positioned := eventbus.PositionFromContext(ctx)
			cursors, err := consumer.Cursors(ctx, f.db)
			if err != nil {
				t.Errorf("cursors in the repair: %v", err)
			}
			effects, has := cursors[eventbus.TopicSystemEvents]
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, call{ev.ID, positioned, pos.Offset, f.model.Cursor()[eventbus.TopicSystemEvents], effects, has})
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.start(t, d, nil)
	mu.Lock()
	caughtUp := append([]call(nil), calls...)
	mu.Unlock()
	if len(caughtUp) != 1 || caughtUp[0].id != second.ID || caughtUp[0].projection != 4 {
		t.Fatalf("repairs of the catch-up = %+v, want one with the last announcement of the world, after the journal was read", caughtUp)
	}

	late := stateSnapshotEvent(5)
	f.publish(t, late)
	eventually(t, "the repair of the snapshot published after the start", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(calls) >= 2
	})
	eventually(t, "the subscription to reach the end", func() bool {
		offset, _ := f.cursor(t, eventbus.TopicSystemEvents)
		return offset == 4
	})
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("repairs = %+v, want the catch-up and the late announcement only", calls)
	}
	c := calls[1]
	if c.id != late.ID || !c.positioned || c.offset != 4 || c.projection > c.offset || (c.hasEffects && c.effects >= c.offset) {
		t.Errorf("repair in the subscription = %+v, want offset 4 before either cursor moved", c)
	}
}
