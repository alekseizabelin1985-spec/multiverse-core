//go:build integration

// The consumer of the gateway against the broker the platform ships with
// (T-316, ADR-010): the order of a topic, a repeat of the bus, a restart of the
// consumer group, the catch-up to the end of the journal, dead_letters, and the
// outbox while the broker is away. The unit tests of the package prove the same
// rules on membus; here they meet kafka-go and Redpanda, whose group offsets,
// watermarks and reconnections membus does not have.
//
// One broker serves every subtest, and they run in order. The group and the
// topics of the consumer are fixed (consumer.Group, consumer.Topics), so a
// subtest does not own the topics: it starts from the end of the journal as it
// finds it, gives its events ids of its own and counts only those. The outage
// runs last, because it stops the broker.
package consumer_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/testcontainers/testcontainers-go"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/internal/gateway/handlers"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
)

// retries is the backoff of the bus in these tests: three retries, as in
// production, without the pauses, which would only make the run slower.
var retries = []time.Duration{0, 0, 0}

// A broker answers later than membus: a consumer group joins, a writer
// reconnects after the broker is back.
const (
	joinLimit      = 90 * time.Second
	reconnectLimit = 90 * time.Second
)

var discardLog = slog.New(slog.NewJSONHandler(io.Discard, nil))

type broker struct {
	rp  *testkit.Redpanda
	bus *eventbus.Kafka
}

func TestConsumerOnRedpanda(t *testing.T) {
	ctx := context.Background()
	topics := make([]string, 0, 16)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	wall := testkit.Wall()
	began := wall.Now()
	rp, err := testkit.StartRedpanda(ctx, topics...)
	if rp != nil {
		testcontainers.CleanupContainer(t, rp.Container())
	}
	if err != nil {
		t.Fatalf("start redpanda: %v", err)
	}
	t.Logf("redpanda at %s, started in %s", rp.Broker, wall.Now().Sub(began).Round(time.Millisecond))

	bus, err := eventbus.NewKafka(eventbus.KafkaConfig{
		Brokers: rp.Brokers(), Registry: contracts.Default(), Backoff: retries, Log: discardLog,
	})
	if err != nil {
		t.Fatalf("new kafka bus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	b := &broker{rp: rp, bus: bus}

	t.Run("CatchUpStopsAtTheEndOfTheJournalAsOnMembus", b.catchUpAsOnMembus)
	t.Run("OrderAndARepeatOfTheBus", b.orderAndRepeat)
	t.Run("ARestartOfTheGroupResumesFromTheCursors", b.groupRestart)
	t.Run("AnEventThatKeepsFailingIsParkedWithItsReason", b.deadLetter)
	t.Run("OutboxWhileTheBrokerIsAway", b.outage)
}

// catchUp is what Start leaves behind after catching the projection up from
// a snapshot cursor, relative to the end of the journal before the test.
type catchUp struct {
	UnwrittenEnd    int64 // End of a topic nobody writes
	Published       int64 // End after three publishes
	EndAfterReading int64 // End once the catch-up read the journal
	Projection      int64 // cursor of the projection when Start returned
	Effects         int32 // effects taken by the catch-up
	HP, Version     int   // the character when Start returned
	PastTheEnd      int   // events a read far past the end delivered
	Next            int64 // the offset that read returned
}

// Journal.End on the kafka adapter means what it means on membus: the offset
// the next message gets, moved by one per publish and not by a read, 0 for a
// topic nobody wrote; and a catch-up bounded by it terminates (C-01, the
// contract test of shared/testkit/contract). The same catch-up runs on both and
// gives the same result.
func (b *broker) catchUpAsOnMembus(t *testing.T) {
	onMembus := newFixture(t, membus.Chaos{})
	want := runCatchUp(t, onMembus, onMembus.bus, onMembus.bus, "catchup-membus")

	onKafka := newFixture(t, membus.Chaos{})
	got := runCatchUp(t, onKafka, b.bus, b.bus, "catchup-kafka")

	if got != want {
		t.Errorf("catch-up on redpanda %+v, on membus %+v", got, want)
	}
	if want.Published != 3 || want.Projection != 3 || want.Effects != 2 || want.HP != 5 || want.Version != 3 ||
		want.PastTheEnd != 3 || want.Next != 3 || want.UnwrittenEnd != 0 {
		t.Errorf("catch-up on membus %+v: the reference itself is wrong", want)
	}
}

func runCatchUp(t *testing.T, f *fixture, bus eventbus.Bus, journal eventbus.Journal, prefix string) catchUp {
	t.Helper()
	ctx := context.Background()
	end := func(topic string) int64 {
		t.Helper()
		n, err := journal.End(ctx, topic)
		if err != nil {
			t.Fatalf("End(%s): %v", topic, err)
		}
		return n
	}
	var obs catchUp
	obs.UnwrittenEnd = end(eventbus.TopicWorldEvents)
	base := end(eventbus.TopicSystemEvents)
	player := prefix + "-player"
	events := []eventbus.Event{
		named(prefix+"-1", created(player, 10)),
		named(prefix+"-2", updated(player, 2, 10, 8)),
		named(prefix+"-3", updated(player, 3, 8, 5)),
	}
	for _, ev := range events {
		if err := bus.Publish(ctx, ev); err != nil {
			t.Fatalf("publish %s: %v", ev.ID, err)
		}
	}
	obs.Published = end(eventbus.TopicSystemEvents) - base

	// The snapshot held the character at version 1 and was taken after it.
	if _, err := f.model.Apply(events[0]); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	d, err := consumer.New(consumer.Config{
		Bus: &quietBus{Bus: bus}, Journal: journal, DB: f.db, Model: f.model, Clock: f.clock, Log: discardLog,
		Effects: map[string][]consumer.Effect{
			readmodel.TypeEntityCreated: {logging(&calls, nil)}, readmodel.TypeEntityUpdated: {logging(&calls, nil)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.start(t, d, map[string]int64{eventbus.TopicSystemEvents: base + 1})

	obs.Projection = f.model.Cursor()[eventbus.TopicSystemEvents] - base
	obs.Effects = calls.Load()
	if c, ok := f.model.Character(player); ok {
		obs.HP, obs.Version = c.HP, int(c.Version)
	}
	obs.EndAfterReading = end(eventbus.TopicSystemEvents) - base
	next, err := journal.ReadRange(ctx, eventbus.TopicSystemEvents, base, base+1000, func(context.Context, eventbus.Event) error {
		obs.PastTheEnd++
		return nil
	})
	if err != nil {
		t.Fatalf("ReadRange past the end: %v", err)
	}
	obs.Next = next - base
	return obs
}

// The subscription of the consumer group takes the events of a topic in the
// order of their offsets, and the same event published twice — what a
// producer retry after a lost acknowledgement puts on the wire — takes its
// effects once (NFR-013).
func (b *broker) orderAndRepeat(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	var calls atomic.Int32
	effect := logging(&calls, nil)
	d := b.dispatcher(t, f, map[string][]consumer.Effect{
		readmodel.TypeEntityCreated: {effect}, readmodel.TypeEntityUpdated: {effect},
	})
	f.start(t, d, b.ends(t))

	const prefix = "order"
	player := prefix + "-player"
	hit := named(prefix+"-2", updated(player, 2, 10, 7))
	base := b.publish(t, named(prefix+"-1", created(player, 10)), hit, hit, named(prefix+"-3", updated(player, 3, 7, 4)))

	wall := testkit.Wall()
	began := wall.Now()
	within(t, joinLimit, "the cursor to pass both copies", func() bool {
		offset, ok := f.cursor(t, eventbus.TopicSystemEvents)
		return ok && offset == base+3
	})
	t.Logf("T316 measure: the group joined and took four events in %s", wall.Now().Sub(began).Round(time.Millisecond))

	if got := b.effectsOf(t, f, prefix); !slices.Equal(got, []string{prefix + "-1", prefix + "-2", prefix + "-3"}) {
		t.Errorf("effects in the order %v, want each event once in the order of the topic", got)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events WHERE event_id LIKE ?`, prefix+"-%"); n != 3 {
		t.Errorf("%d marks processed, want 3", n)
	}
	if c, _ := f.model.Character(player); c.HP != 4 || c.Version != 3 {
		t.Errorf("projection %+v, want hp 4 at version 3", c)
	}
	if next := f.model.Cursor()[eventbus.TopicSystemEvents]; next != base+4 {
		t.Errorf("projection cursor %d, want %d", next, base+4)
	}
	if err := d.Err(); err != nil {
		t.Errorf("a subscription ended: %v", err)
	}
}

// A consumer that stops and starts again over the same gateway.db resumes
// from the committed offset of its group and takes the effects of what came
// while it was away, once. A consumer that reads the journal again from an
// older offset — a group that lost its offsets, an older snapshot — takes no
// effect twice: the effects cursor of gateway.db decides (ADR-027).
func (b *broker) groupRestart(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	var calls atomic.Int32
	effects := map[string][]consumer.Effect{readmodel.TypeEntityCreated: {logging(&calls, nil)}}
	const prefix = "restart"
	player := func(n int) string { return fmt.Sprintf("%s-player-%d", prefix, n) }
	event := func(n int) eventbus.Event { return named(fmt.Sprintf("%s-%d", prefix, n), created(player(n), 10)) }

	first := b.dispatcher(t, f, effects)
	f.start(t, first, b.ends(t))
	base := b.publish(t, event(1), event(2))
	within(t, joinLimit, "the first consumer to take two events", func() bool {
		offset, ok := f.cursor(t, eventbus.TopicSystemEvents)
		return ok && offset == base+1
	})
	stop(t, first)

	b.publish(t, event(3), event(4))
	f.model = readmodel.New(readmodel.Config{Timers: f.clock.Timers()})
	second := b.dispatcher(t, f, effects)
	wall := testkit.Wall()
	began := wall.Now()
	f.start(t, second, b.ends(t))
	within(t, joinLimit, "the second consumer to take what came while it was away", func() bool {
		offset, _ := f.cursor(t, eventbus.TopicSystemEvents)
		return offset == base+3
	})
	t.Logf("T316 measure: the restarted group took the backlog in %s", wall.Now().Sub(began).Round(time.Millisecond))
	if _, ok := f.model.Character(player(1)); ok {
		t.Error("the restarted group delivered the event before its committed offset again")
	}
	if _, ok := f.model.Character(player(4)); !ok {
		t.Error("the backlog did not reach the projection")
	}
	stop(t, second)

	f.model = readmodel.New(readmodel.Config{Timers: f.clock.Timers()})
	third := b.dispatcher(t, f, effects)
	f.start(t, third, map[string]int64{eventbus.TopicSystemEvents: base})
	for n := 1; n <= 4; n++ {
		if _, ok := f.model.Character(player(n)); !ok {
			t.Errorf("the catch-up from an older offset did not bring %s into the projection", player(n))
		}
	}

	want := []string{event(1).ID, event(2).ID, event(3).ID, event(4).ID}
	if got := b.effectsOf(t, f, prefix); !slices.Equal(got, want) {
		t.Errorf("effects %v over three consumers, want %v once each", got, want)
	}
	if offset, _ := f.cursor(t, eventbus.TopicSystemEvents); offset != base+3 {
		t.Errorf("effects cursor %d, want %d", offset, base+3)
	}

	// A consumer stopped while its readers are still joining the group does
	// not stop within the budget of a stop (first run of T-316: more than
	// 10 s): the reader of kafka-go waits for its join to end. The third
	// consumer is stopped once it took an event, so that it has joined.
	sentinel := b.publish(t, named("sentinel-restart-1", created("sentinel-player", 10)))
	within(t, joinLimit, "the third consumer to join its group", func() bool {
		offset, _ := f.cursor(t, eventbus.TopicSystemEvents)
		return offset == sentinel
	})
	stop(t, third)
}

// An event whose effect keeps failing is parked in dead_letters of the broker
// with the error of the effect, after the retries of the bus; nothing of it
// is committed in gateway.db, and the subscription goes on to the next event.
func (b *broker) deadLetter(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	var calls atomic.Int32
	const doomed = "dead-1"
	effect := logging(&calls, nil)
	failing := func(ctx context.Context, tx *sql.Tx, ev eventbus.Event, res readmodel.Result) error {
		if err := effect(ctx, tx, ev, res); err != nil {
			return err
		}
		if ev.ID == doomed {
			return errEffect
		}
		return nil
	}
	d := b.dispatcher(t, f, map[string][]consumer.Effect{readmodel.TypeEntityCreated: {failing}})
	f.start(t, d, b.ends(t))

	base := b.publish(t, named(doomed, created("dead-player", 10)), named("live-1", created("live-player", 10)))
	within(t, joinLimit, "the event after the parked one to be taken", func() bool {
		offset, ok := f.cursor(t, eventbus.TopicSystemEvents)
		return ok && offset == base+1
	})

	letter, ok := findDeadLetter(t, b.rp.Broker, doomed)
	if !ok {
		t.Fatalf("no dead letter of %s on the broker", doomed)
	}
	if !strings.Contains(letter.Error, errEffect.Error()) || !strings.Contains(letter.Error, doomed) {
		t.Errorf("dead letter error %q, want the error of the effect of %s", letter.Error, doomed)
	}
	if letter.Consumer != consumer.Group || letter.Attempts != len(retries)+1 {
		t.Errorf("dead letter of %s after %d attempts, want %s after %d", letter.Consumer, letter.Attempts, consumer.Group, len(retries)+1)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM processed_events WHERE event_id = ?`, doomed) +
		f.count(t, `SELECT COUNT(*) FROM effects_log WHERE event_id = ?`, doomed); n != 0 {
		t.Errorf("%d rows of mark or effect of the parked event", n)
	}
	if got := b.effectsOf(t, f, "live"); !slices.Equal(got, []string{"live-1"}) {
		t.Errorf("effects of the next event %v, want one", got)
	}
}

// attempts is the bus of the tracker: it remembers the id of every
// publication of analytics.turn.completed it is asked for, taken or not.
type attempts struct {
	eventbus.Bus
	mu  sync.Mutex
	ids []string
}

func (a *attempts) Publish(ctx context.Context, ev eventbus.Event) error {
	if ev.Type == turns.TypeCompleted {
		a.mu.Lock()
		a.ids = append(a.ids, ev.ID)
		a.mu.Unlock()
	}
	return a.Bus.Publish(ctx, ev)
}

// made returns how many publications were asked for and their distinct ids.
func (a *attempts) made() (int, []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := slices.Clone(a.ids)
	slices.Sort(out)
	return len(a.ids), slices.Compact(out)
}

// While the broker is away, the ack of the last recipient of a narrative
// answers 503 bus_unavailable within the deadline of the publication, rolls
// the whole ack back and leaves gateway.db free; the deliveries are given
// again once their lease ran out. Once the broker is back, the repeat of the
// ack publishes one analytics.turn.completed under the id every attempt had,
// and no delivery is lost (acceptance of T-307; review #1 of T-307, decision
// 2). The durations are logged with the prefix "T316 measure".
func (b *broker) outage(t *testing.T) {
	ctx := context.Background()
	wall := testkit.Wall()
	f := newFixture(t, membus.Chaos{})
	m := seeded(t, f.model)
	tracked := &attempts{Bus: b.bus}
	fd := &feed{fixture: f, links: linkRows{"player-A": links.PlatformTelegram, "player-B": links.PlatformTelegram}}
	var err error
	if fd.queue, err = outbox.New(outbox.Config{DB: f.db}); err != nil {
		t.Fatal(err)
	}
	sessions, err := session.New(session.Config{DB: f.db})
	if err != nil {
		t.Fatal(err)
	}
	if fd.tracker, err = turns.New(turns.Config{DB: f.db, Sessions: sessions, Bus: tracked, Clock: f.clock}); err != nil {
		t.Fatal(err)
	}
	effects, err := (&consumer.Deliveries{Model: m, Outbox: fd.queue, Links: fd.links, Turns: fd.tracker, Clock: f.clock}).Effects()
	if err != nil {
		t.Fatal(err)
	}
	d := b.dispatcher(t, f, effects)
	svc, err := outbox.NewService(outbox.ServiceConfig{Store: fd.queue, Routes: fd.links, Clock: f.clock, Timers: clock.RealTimers{}})
	if err != nil {
		t.Fatal(err)
	}
	ack := acker(t, &handlers.Deliveries{Service: svc, Turns: fd.tracker, Platforms: handlers.ClientPlatforms, Log: discardLog})

	analytics, ok := contracts.Default().Lookup(turns.TypeCompleted)
	if !ok {
		t.Fatalf("%s is not registered", turns.TypeCompleted)
	}
	analyticsBase := b.end(t, analytics.Topic)

	fd.turnOf(t, "act-look", "player-A", "look")
	f.start(t, d, b.ends(t))
	agent := &eventbus.AgentRef{ID: "narrator:solo:player-A", Level: "task", Blueprint: "narrator"}
	narrative := told("outage-n-1", "act-look", "entry", outbox.GeneratedByLLM, "player-A", "player-B")
	narrative.Meta.Agent = agent
	decision := combat("outage-c-1", solo("player-A"), "player-A", "wolf-alpha")
	decision.Meta.Agent, decision.Source = agent, contracts.SourceSwarm
	// Two topics are two subscriptions, and the gateway keeps the order within a
	// topic only (C-05 p. 8): the decision is published once the narrative is
	// in the queue, so that it stands behind it.
	b.publish(t, narrative)
	within(t, joinLimit, "the narrative of both recipients", func() bool { return len(fd.rows(t)) == 2 })
	b.publish(t, decision)
	within(t, joinLimit, "the decision of player-A", func() bool { return len(fd.rows(t)) == 3 })

	leased, err := fd.queue.Lease(ctx, "telegram-bot", links.PlatformTelegram, 10, f.clock.Now())
	if err != nil || len(leased) != 2 {
		t.Fatalf("first lease = %d, %v; want the narrative of each recipient", len(leased), err)
	}
	idOf := func(ds []outbox.Delivery, player string) string {
		for _, d := range ds {
			if d.PlayerID == player {
				return d.ID
			}
		}
		t.Fatalf("no delivery of %s in %+v", player, ds)
		return ""
	}
	if code, body, _ := ack(idOf(leased, "player-A")); code != http.StatusOK {
		t.Fatalf("ack of the first recipient with the broker up = %d %s", code, body)
	}
	lastNarrative := idOf(leased, "player-B")
	next, err := fd.queue.Lease(ctx, "telegram-bot", links.PlatformTelegram, 10, f.clock.Now())
	if err != nil || len(next) != 1 || next[0].Kind != outbox.KindMechanics {
		t.Fatalf("second lease = %+v, %v; want the decision of player-A behind its narrative", next, err)
	}
	batch := []string{lastNarrative, next[0].ID}

	grace := 10 * time.Second
	began := wall.Now()
	if err := b.rp.Container().Stop(ctx, &grace); err != nil {
		t.Fatalf("stop the broker: %v", err)
	}
	t.Logf("T316 measure: the broker stopped in %s", wall.Now().Sub(began).Round(time.Millisecond))

	const tries = 3
	for i := range tries {
		code, body, took := ack(batch...)
		t.Logf("T316 measure: ack %d with the broker away: %d in %s", i+1, code, took.Round(time.Millisecond))
		if code != http.StatusServiceUnavailable || !strings.Contains(body, api.CodeBusUnavailable) {
			t.Fatalf("ack with the broker away = %d %s, want 503 %s", code, body, api.CodeBusUnavailable)
		}
		if took > turns.DefaultPublishTimeout+time.Second {
			t.Errorf("ack with the broker away took %s, want no longer than the deadline of the publication %s", took, turns.DefaultPublishTimeout)
		}
		probe, cancel := context.WithTimeout(ctx, time.Second)
		var pending int
		err := f.db.QueryRowContext(probe, `SELECT COUNT(*) FROM deliveries WHERE state = 'pending'`).Scan(&pending)
		cancel()
		if err != nil {
			t.Fatalf("gateway.db is held after the ack failed: %v", err)
		}
		if pending != 2 {
			t.Errorf("%d pending deliveries after the failed ack, want the whole batch rolled back", pending)
		}
	}
	row, _, err := turns.Load(ctx, f.db, "act-look")
	if err != nil || row.Status != turns.StatusNarrated || row.DeliveredCount != 1 {
		t.Errorf("turn after the failed acks %+v, %v; want narrated with one delivery", row, err)
	}

	f.clock.Advance(outbox.DefaultLease + time.Second)
	again, err := fd.queue.Lease(ctx, "telegram-bot", links.PlatformTelegram, 10, f.clock.Now())
	if err != nil || len(again) != 2 {
		t.Fatalf("lease after the lease ran out = %+v, %v; want both deliveries of the failed ack again", again, err)
	}
	for _, d := range again {
		if !slices.Contains(batch, d.ID) || d.Attempts != 2 {
			t.Errorf("given again %s after %d attempts, want a delivery of the batch on its second attempt", d.ID, d.Attempts)
		}
	}

	began = wall.Now()
	if err := b.rp.Container().Start(ctx); err != nil {
		t.Fatalf("start the broker again: %v", err)
	}
	t.Logf("T316 measure: the broker started again in %s", wall.Now().Sub(began).Round(time.Millisecond))
	began = wall.Now()
	refused := 0
	deadline := testkit.After(reconnectLimit)
	for {
		code, body, took := ack(batch...)
		if code == http.StatusOK {
			t.Logf("T316 measure: the repeat of the ack was taken %s after the start, after %d refusals; the last ack took %s",
				wall.Now().Sub(began).Round(time.Millisecond), refused, took.Round(time.Millisecond))
			if !strings.Contains(body, `"acked":2`) {
				t.Errorf("repeat of the ack = %s, want both deliveries acknowledged", body)
			}
			break
		}
		if code != http.StatusServiceUnavailable {
			t.Fatalf("repeat of the ack = %d %s", code, body)
		}
		refused++
		select {
		case <-deadline:
			t.Fatalf("the ack was refused %d times over %s after the broker came back", refused, reconnectLimit)
		case <-testkit.After(250 * time.Millisecond):
		}
	}
	if code, body, _ := ack(batch...); code != http.StatusOK || !strings.Contains(body, `"acked":0`) {
		t.Errorf("a second repeat of the ack = %d %s, want nothing acknowledged again", code, body)
	}

	var published []eventbus.Event
	if _, err := b.bus.ReadRange(ctx, analytics.Topic, analyticsBase, analyticsBase+1000, func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == turns.TypeCompleted && ev.CorrelationID() == "act-look" {
			published = append(published, ev)
		}
		return nil
	}); err != nil {
		t.Fatalf("read %s: %v", analytics.Topic, err)
	}
	asked, ids := tracked.made()
	t.Logf("T316 measure: %d publications of turn.completed attempted under %d id(s)", asked, len(ids))
	if len(published) != 1 || len(ids) != 1 || published[0].ID != ids[0] {
		t.Errorf("turn.completed on the broker %d (ids of the attempts %v), want one under the id of every attempt", len(published), ids)
	}
	for _, r := range fd.rows(t) {
		if r.State != outbox.StateDelivered {
			t.Errorf("delivery of %s %s left %s", r.Player, r.Kind, r.State)
		}
	}
	if row, _, err := turns.Load(ctx, f.db, "act-look"); err != nil || row.Status != turns.StatusCompleted || row.DeliveredCount != 2 {
		t.Errorf("turn after the broker came back %+v, %v; want completed with two deliveries", row, err)
	}

	// After the outage the consumer hears every one of its topics again, and an
	// event of each reaches gateway.db once. Two outcomes are accepted until the
	// consumer subscribes again by itself (decision 1 of the orchestrator on
	// T-316): (a) no subscription ended, and the subscriptions take an event of
	// every topic; (b) a subscription ended with an error of reading from the
	// broker (Err, /health bus=fail), and the consumer the process starts again
	// takes the events from the effects cursor of gateway.db and then hears
	// every topic through its own subscriptions. Which one happened is logged.
	if err := d.Err(); err != nil {
		t.Logf("T316 measure: before the events past the outage were published a subscription had already ended: %v", err)
	}
	began = wall.Now()
	offsets := b.publishOnEveryTopic(t, "outage-after", agent)
	taken := func(offsets map[string]int64) bool {
		for topic, offset := range offsets {
			if cursor, _ := f.cursor(t, topic); cursor != offset {
				return false
			}
		}
		return true
	}
	settle := testkit.After(reconnectLimit)
	for d.Err() == nil && !taken(offsets) {
		select {
		case <-settle:
			t.Fatalf("after %s the events past the outage %v are not all taken and no subscription ended; effects cursors %v",
				reconnectLimit, offsets, b.cursors(t, f))
		case <-testkit.After(20 * time.Millisecond):
		}
	}
	ended := d.Err()
	if ended == nil {
		t.Logf("T316 measure: the subscriptions outlived the broker and took an event of every topic %s after they were published",
			wall.Now().Sub(began).Round(time.Millisecond))
		began = wall.Now()
		stop(t, d)
		t.Logf("T316 measure: the consumer whose subscriptions outlived the broker stopped in %s", wall.Now().Sub(began).Round(time.Millisecond))
	} else {
		t.Logf("T316 measure: a subscription was found ended %s after the events past the outage were published: %v",
			wall.Now().Sub(began).Round(time.Millisecond), ended)
		if !strings.Contains(ended.Error(), readError) {
			t.Fatalf("the subscription ended with %v, want an error of reading from the broker (%q)", ended, readError)
		}
		began = wall.Now()
		stop(t, d)
		t.Logf("T316 measure: the consumer whose subscription ended stopped in %s", wall.Now().Sub(began).Round(time.Millisecond))

		again := b.dispatcher(t, f, effects)
		began = wall.Now()
		f.start(t, again, offsets)
		if !taken(offsets) {
			t.Errorf("the consumer started again did not take the events past the outage from the journal: offsets %v, effects cursors %v",
				offsets, b.cursors(t, f))
		}
		// Its readers join a group whose members of before the outage are still
		// on the books of the broker, so what they take comes later.
		sentinels := b.publishOnEveryTopic(t, "outage-sentinel", agent)
		joined := testkit.After(joinLimit)
		for again.Err() == nil && !taken(sentinels) {
			select {
			case <-joined:
				t.Fatalf("after %s the consumer started again took no event of every topic through its subscriptions: offsets %v, effects cursors %v",
					joinLimit, sentinels, b.cursors(t, f))
			case <-testkit.After(20 * time.Millisecond):
			}
		}
		if err := again.Err(); err != nil {
			t.Fatalf("a subscription of the consumer started again ended: %v", err)
		}
		t.Logf("T316 measure: the consumer started again took an event of every topic through its subscriptions %s after it started",
			wall.Now().Sub(began).Round(time.Millisecond))
		began = wall.Now()
		stop(t, again)
		t.Logf("T316 measure: the consumer started again stopped in %s", wall.Now().Sub(began).Round(time.Millisecond))
	}
	for _, prefix := range []string{"outage-after", "outage-sentinel"} {
		if ended == nil && prefix == "outage-sentinel" {
			continue
		}
		for _, id := range eventIDsOnEveryTopic(prefix) {
			if n := f.count(t, `SELECT COUNT(*) FROM processed_events WHERE event_id = ?`, id); n != 1 {
				t.Errorf("%d marks of %s, want 1", n, id)
			}
		}
	}
}

// readError is how an error of reading from the broker reaches Err: the
// subscription of the dispatcher wraps the error of Kafka.Subscribe, whose
// readerError begins with it.
const readError = "eventbus: read "

// eventIDsOnEveryTopic names the events publishOnEveryTopic publishes under
// prefix.
func eventIDsOnEveryTopic(prefix string) []string {
	return []string{prefix + "-system", prefix + "-game", prefix + "-world", prefix + "-narrative"}
}

// publishOnEveryTopic publishes one event into each topic of the consumer and
// returns the offset of each; the events are ones the effects of the
// deliveries take without an error in the world of seeded.
func (b *broker) publishOnEveryTopic(t *testing.T, prefix string, agent *eventbus.AgentRef) map[string]int64 {
	t.Helper()
	ids := eventIDsOnEveryTopic(prefix)
	fact := named(ids[0], created(prefix+"-player", 10))
	decision := combat(ids[1], solo("player-A"), "player-A", "wolf-alpha")
	opened := mk(ids[2], readmodel.TypeEncounterStarted, "act-"+ids[2], solo("player-A"), map[string]any{
		"encounter": entityRef("enc-"+ids[2], entity.TypeEncounter), "region": entityRef("dark-forest-01", entity.TypeRegion),
		"participants": []any{entityRef("player-A", entity.TypePlayer)}, "npcs": []any{entityRef("wolf-alpha", entity.TypeNPC)},
	})
	narrative := told(ids[3], "act-"+ids[3], "entry", outbox.GeneratedByLLM, "player-A")
	for _, ev := range []*eventbus.Event{&decision, &opened, &narrative} {
		ev.Meta.Agent, ev.Source = agent, contracts.SourceSwarm
	}
	offsets := make(map[string]int64, len(consumer.Topics))
	for _, ev := range []eventbus.Event{fact, decision, opened, narrative} {
		topic, err := eventbus.Route(contracts.Default(), ev)
		if err != nil {
			t.Fatalf("route %s: %v", ev.Type, err)
		}
		offsets[topic] = b.publish(t, ev)
	}
	if len(offsets) != len(consumer.Topics) {
		t.Fatalf("the events past the outage reach %d topics, want the %d of the consumer", len(offsets), len(consumer.Topics))
	}
	return offsets
}

// cursors is the effects cursor of every topic in gateway.db, for a message.
func (b *broker) cursors(t *testing.T, f *fixture) map[string]int64 {
	t.Helper()
	cursors, err := consumer.Cursors(context.Background(), f.db)
	if err != nil {
		t.Fatal(err)
	}
	return cursors
}

// acker serves an ack through the middleware of the gateway as telegram-bot,
// and returns the status, the body and how long the request took.
func acker(t *testing.T, h *handlers.Deliveries) func(ids ...string) (int, string, time.Duration) {
	t.Helper()
	cfg := &api.Config{ClientIDs: []string{"telegram-bot"}, RequestIDs: func() string { return "r-t316" },
		Clock: clock.Real{}, Log: discardLog}
	router := api.GatewayRouter(api.Handlers{
		ResolveLink: http.NotFoundHandler(), ConsentLink: http.NotFoundHandler(), ForgetLink: http.NotFoundHandler(),
		ListWorlds: http.NotFoundHandler(), CreateCharacter: http.NotFoundHandler(), GetPlayer: http.NotFoundHandler(),
		PostAction:     http.NotFoundHandler(),
		PollDeliveries: http.HandlerFunc(h.Poll), AckDeliveries: http.HandlerFunc(h.Ack), StreamDeliveries: http.HandlerFunc(h.Stream),
	})
	mux := http.NewServeMux()
	router.Mount(mux, api.Chain(cfg)...)
	wall := testkit.Wall()
	return func(ids ...string) (int, string, time.Duration) {
		body, err := json.Marshal(api.AckRequest{IDs: ids})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/clients/telegram-bot/deliveries/ack", strings.NewReader(string(body)))
		req.Header.Set(api.HeaderClientID, "telegram-bot")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		began := wall.Now()
		mux.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String(), wall.Now().Sub(began)
	}
}

func (b *broker) dispatcher(t *testing.T, f *fixture, effects map[string][]consumer.Effect) *consumer.Dispatcher {
	t.Helper()
	d, err := consumer.New(consumer.Config{
		Bus: b.bus, Journal: b.bus, DB: f.db, Model: f.model, Clock: f.clock, Log: discardLog, Effects: effects,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// ends is a snapshot cursor at the end of every topic of the consumer: the
// catch-up reads nothing that earlier subtests left in the topics.
func (b *broker) ends(t *testing.T) map[string]int64 {
	t.Helper()
	out := make(map[string]int64, len(consumer.Topics))
	for _, topic := range consumer.Topics {
		out[topic] = b.end(t, topic)
	}
	return out
}

func (b *broker) end(t *testing.T, topic string) int64 {
	t.Helper()
	end, err := b.bus.End(context.Background(), topic)
	if err != nil {
		t.Fatalf("End(%s): %v", topic, err)
	}
	return end
}

// publish publishes events of one topic in order and returns the offset of
// the first one. Nothing else writes the topics of the consumer meanwhile.
func (b *broker) publish(t *testing.T, events ...eventbus.Event) int64 {
	t.Helper()
	topic, err := eventbus.Route(contracts.Default(), events[0])
	if err != nil {
		t.Fatalf("route %s: %v", events[0].Type, err)
	}
	first := b.end(t, topic)
	for _, ev := range events {
		if err := b.bus.Publish(context.Background(), ev); err != nil {
			t.Fatalf("publish %s %s: %v", ev.Type, ev.ID, err)
		}
	}
	return first
}

// effectsOf lists the effects written for the events of prefix, in the order
// they were written.
func (b *broker) effectsOf(t *testing.T, f *fixture, prefix string) []string {
	t.Helper()
	rows, err := f.db.QueryContext(context.Background(),
		`SELECT event_id FROM effects_log WHERE event_id LIKE ? ORDER BY rowid`, prefix+"-%")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// named gives an event an id of its subtest: the broker outlives a subtest,
// and the sequence of testkit.Deterministic starts again in each one.
func named(id string, ev eventbus.Event) eventbus.Event {
	ev.ID = id
	return ev
}

// stop stops a consumer within a minute: a reader of kafka-go that is joining
// its group ends only when the join does.
func stop(t *testing.T, d *consumer.Dispatcher) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := d.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func within(t *testing.T, limit time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.After(limit)
	for !cond() {
		select {
		case <-deadline:
			t.Fatalf("timed out after %s waiting for %s", limit, what)
		case <-testkit.After(20 * time.Millisecond):
		}
	}
}

// findDeadLetter reads dead_letters of the broker with a plain reader — a dead
// letter is not an event, and the journal would park it again — and returns
// the one of the event id.
func findDeadLetter(t *testing.T, broker, eventID string) (eventbus.DeadLetter, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := kafka.DialLeader(ctx, "tcp", broker, eventbus.TopicDeadLetters, 0)
	if err != nil {
		t.Fatalf("dial dead_letters: %v", err)
	}
	end, err := conn.ReadLastOffset()
	_ = conn.Close()
	if err != nil {
		t.Fatalf("end of dead_letters: %v", err)
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker}, Topic: eventbus.TopicDeadLetters, Partition: 0,
		MinBytes: 1, MaxBytes: 10 << 20, MaxWait: 100 * time.Millisecond,
	})
	defer func() { _ = reader.Close() }()
	if err := reader.SetOffset(0); err != nil {
		t.Fatalf("seek dead_letters: %v", err)
	}
	for range end {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("read dead_letters: %v", err)
		}
		var dl eventbus.DeadLetter
		if err := json.Unmarshal(msg.Value, &dl); err != nil {
			t.Fatalf("decode dead letter at %d: %v", msg.Offset, err)
		}
		if dl.Original.ID == eventID {
			return dl, true
		}
	}
	return eventbus.DeadLetter{}, false
}
