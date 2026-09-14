//go:build integration

// The consumer of the gateway against the broker the platform ships with
// (T-316, T-473, ADR-010): the order of a topic, a repeat of the bus, a restart
// of the consumer group, the catch-up to the end of the journal and below the
// start of the log, dead_letters, the outbox while the broker is away or hung,
// and the subscriptions started again after the broker is back. The unit tests
// of the package prove the same rules on membus; here they meet kafka-go and
// Redpanda, whose group offsets, watermarks and reconnections membus does not
// have.
//
// One broker serves every subtest, and they run in order. The group and the
// topics of the consumer are fixed (consumer.Group, consumer.Topics), so a
// subtest does not own the topics: it starts from the end of the journal as it
// finds it, gives its events ids of its own and counts only those. The outages
// run last, because they stop and pause the broker.
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
	"github.com/segmentio/kafka-go/protocol"
	"github.com/testcontainers/testcontainers-go"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
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
	"multiverse-core.io/shared/runtime"
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
	// resubscribeLimit bounds the wait for a consumer to hear every topic
	// again after the broker is back, without a restart. The parts: the join
	// of a group whose members from before the outage are still on the books
	// of the broker (T-316: 29.8 s), the pause before the next subscription
	// (at most consumer.ResubscribeCeiling, 15 s), and an attempt started
	// while the broker was away that kafka-go retries up to three joins 5 s
	// apart before it reports — about 65 s at worst, and the limit keeps a
	// margin over it. The runs of T-473 measured the wait itself (card of
	// T-473, "Замеры").
	resubscribeLimit = 90 * time.Second
	// hangLimit bounds the pause of the broker while the loop of the bot waits
	// for the deliveries of a failed ack to be given again: the estimate of the
	// acceptance of T-316 is 31 s for a broker that refuses, and two flushes of
	// the bot against a broker that does not answer are about 43 s.
	hangLimit = 100 * time.Second
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
	t.Run("ARestartOfTheGroupResumesFromItsCommittedOffset", b.groupRestart)
	t.Run("AnEventThatKeepsFailingIsParkedWithItsReason", b.deadLetter)
	t.Run("CursorsBelowTheStartOfTheLogCatchUpFromTheStart", b.belowTheStartOfTheLog)
	t.Run("OutboxWhileTheBrokerIsAway", b.outage)
	t.Run("AHungBrokerAndTheSubscriptionsAfterIt", b.hungBroker)
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
		Bus: &quietBus{Bus: bus}, Journal: journal, DB: f.db, Model: f.model, Clock: f.clock, Timers: clock.RealTimers{},
		Log: discardLog,
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

// A consumer that stops and starts again resumes from the committed offset of
// its group and takes the effects of what came while it was away, once. The
// consumer started again has a gateway.db of its own, without an effects
// cursor, and a snapshot cursor at the end of the journal: its catch-up reads
// nothing (the catch-up of T-309 would otherwise take the backlog from the
// effects cursor before the group subscribes), so what it takes can come only
// through the subscription of the group, from its committed offset. A consumer
// that reads the journal again from an older offset — a group that lost its
// offsets, an older snapshot — takes no effect twice: the effects cursor of
// gateway.db decides (ADR-027). The last one is stopped right after its Start,
// while its readers join the group, within runtime.StopTimeout (T-473).
func (b *broker) groupRestart(t *testing.T) {
	wall := testkit.Wall()
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
	// The offset of the group is committed after the handler, so the effects
	// cursor of gateway.db can be there first. The second consumer has a
	// gateway.db of its own, where a repeat of event 2 would take its effect
	// again (N-5 of review #1 of T-473).
	within(t, joinLimit, "the group to commit the offset after event 2", func() bool {
		return b.committed(t, eventbus.TopicSystemEvents) == base+2
	})
	t.Logf("T473 measure: a consumer that had joined its group stopped in %s", stop(t, first).Round(time.Millisecond))

	b.publish(t, event(3), event(4))
	g := newFixture(t, membus.Chaos{})
	second := b.dispatcher(t, g, effects)
	began := wall.Now()
	g.start(t, second, b.ends(t))
	within(t, joinLimit, "the second consumer to take what came while it was away", func() bool {
		offset, _ := g.cursor(t, eventbus.TopicSystemEvents)
		return offset == base+3
	})
	t.Logf("T316 measure: the restarted group resumed from its committed offset and took the backlog in %s",
		wall.Now().Sub(began).Round(time.Millisecond))
	if got, want := b.effectsOf(t, g, prefix), []string{event(3).ID, event(4).ID}; !slices.Equal(got, want) {
		t.Errorf("effects of the restarted group %v, want %v: only what came after its committed offset", got, want)
	}
	if _, ok := g.model.Character(player(2)); ok {
		t.Error("the restarted group delivered an event before its committed offset again")
	}
	if _, ok := g.model.Character(player(4)); !ok {
		t.Error("the backlog did not reach the projection")
	}
	stop(t, second)

	g.model = readmodel.New(readmodel.Config{Timers: g.clock.Timers()})
	third := b.dispatcher(t, g, effects)
	g.start(t, third, map[string]int64{eventbus.TopicSystemEvents: base})
	for n := 1; n <= 4; n++ {
		if _, ok := g.model.Character(player(n)); !ok {
			t.Errorf("the catch-up from an older offset did not bring %s into the projection", player(n))
		}
	}
	if got, want := b.effectsOf(t, g, prefix), []string{event(3).ID, event(4).ID}; !slices.Equal(got, want) {
		t.Errorf("effects %v after the catch-up from an older offset, want still %v", got, want)
	}
	if got, want := b.effectsOf(t, f, prefix), []string{event(1).ID, event(2).ID}; !slices.Equal(got, want) {
		t.Errorf("effects of the first consumer %v, want %v", got, want)
	}
	if offset, _ := g.cursor(t, eventbus.TopicSystemEvents); offset != base+3 {
		t.Errorf("effects cursor %d, want %d", offset, base+3)
	}
	// The third consumer is stopped once it has joined: a consumer stopped while
	// its readers join leaves them joining, and the next consumer of the group
	// waits for the rebalance. That stop is measured at the end of the test
	// (stopRightAfterStart).
	sentinel := b.publish(t, named("sentinel-restart-1", created("sentinel-player", 10)))
	within(t, joinLimit, "the third consumer to join its group", func() bool {
		offset, _ := g.cursor(t, eventbus.TopicSystemEvents)
		return offset == sentinel
	})
	stop(t, third)
}

// awaitGroupState asks the broker for the state of the consumer group of the
// gateway every 100 ms until it is want or limit passed, and says whether it
// was seen and which states were.
func (b *broker) awaitGroupState(t *testing.T, want string, limit time.Duration) (bool, []string) {
	t.Helper()
	cl := &kafka.Client{Addr: kafka.TCP(b.rp.Broker)}
	var seen []string
	deadline := testkit.After(limit)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res, err := cl.DescribeGroups(ctx, &kafka.DescribeGroupsRequest{GroupIDs: []string{consumer.Group}})
		cancel()
		state := "error"
		if err == nil && len(res.Groups) == 1 && res.Groups[0].Error == nil {
			state = res.Groups[0].GroupState
		}
		if len(seen) == 0 || seen[len(seen)-1] != state {
			seen = append(seen, state)
		}
		if state == want {
			return true, seen
		}
		select {
		case <-deadline:
			return false, seen
		case <-testkit.After(100 * time.Millisecond):
		}
	}
}

// committed is the offset the consumer group of the gateway committed for the
// only partition of topic; -1 when it has none.
func (b *broker) committed(t *testing.T, topic string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := (&kafka.Client{Addr: kafka.TCP(b.rp.Broker)}).OffsetFetch(ctx, &kafka.OffsetFetchRequest{
		GroupID: consumer.Group, Topics: map[string][]int{topic: {0}},
	})
	if err != nil {
		t.Fatalf("offset of %s in %s: %v", topic, consumer.Group, err)
	}
	for _, p := range res.Topics[topic] {
		if p.Partition == 0 {
			return p.CommittedOffset
		}
	}
	return -1
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

// The effects cursor of gateway.db and the cursor of the snapshot of State
// both below the start of the log, as the retention of the topic leaves them
// after a long stop (acceptance of T-309; backlog p. 3 of review #1 of T-309).
// The records before the start are deleted with DeleteRecords, which moves the
// start of the log the way deleted segments do: waiting for the retention of
// Redpanda to delete a segment within a run is not in the hands of a test —
// only closed segments go, on the cadence of the housekeeping of the broker,
// and closing them sooner takes settings of the cluster testkit does not give.
//
// Expected from kafka-go v0.4.51 and checked here: a read from an offset below
// the start of the log begins at the start without an error — the reader
// moves its offset to the first one (reader.go, initialize) — so ReadRange
// delivers what is left and Start catches up from the start of the log: the
// events deleted take no effect and never reach the projection, and nothing
// reports the gap.
func (b *broker) belowTheStartOfTheLog(t *testing.T) {
	ctx := context.Background()
	wall := testkit.Wall()
	f := newFixture(t, membus.Chaos{})
	var calls atomic.Int32
	effects := map[string][]consumer.Effect{readmodel.TypeEntityCreated: {logging(&calls, nil)}}
	const prefix = "trimmed"
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
	b.publish(t, event(3), event(4), event(5))
	end := b.end(t, eventbus.TopicSystemEvents)

	start := b.deleteRecords(t, eventbus.TopicSystemEvents, base+4)
	t.Logf("T473 measure: records of %s before %d deleted, the log starts at %d, End %d; the effects cursor is %d, the snapshot cursor %d",
		eventbus.TopicSystemEvents, base+4, start, b.end(t, eventbus.TopicSystemEvents), base+1, base)
	if start <= base+2 || start >= end {
		t.Fatalf("the log starts at %d, want above the effects cursor %d and below End %d", start, base+1, end)
	}

	var delivered []int64
	next, err := b.bus.ReadRange(ctx, eventbus.TopicSystemEvents, base, end, func(ctx context.Context, _ eventbus.Event) error {
		pos, _ := eventbus.PositionFromContext(ctx)
		delivered = append(delivered, pos.Offset)
		return nil
	})
	t.Logf("T473 measure: ReadRange [%d, %d) of a log that starts at %d delivered %v, returned %d, error %v",
		base, end, start, delivered, next, err)
	var left []int64
	for offset := start; offset < end; offset++ {
		left = append(left, offset)
	}
	if err != nil || next != end || !slices.Equal(delivered, left) {
		t.Errorf("ReadRange below the start of the log = %v up to %d (%v), want what is left %v up to End %d",
			delivered, next, err, left, end)
	}

	f.model = readmodel.New(readmodel.Config{Timers: f.clock.Timers()})
	second := b.dispatcher(t, f, effects)
	began := wall.Now()
	err = second.Start(ctx, map[string]int64{eventbus.TopicSystemEvents: base})
	t.Logf("T473 measure: Start with both cursors below the start of the log returned %v in %s",
		err, wall.Now().Sub(began).Round(time.Millisecond))
	if err != nil {
		t.Fatalf("Start below the start of the log: %v", err)
	}
	t.Cleanup(func() { stop(t, second) })

	want := []string{event(1).ID, event(2).ID}
	for n := 3; n <= 5; n++ {
		if base+int64(n)-1 >= start {
			want = append(want, event(n).ID)
		}
	}
	if got := b.effectsOf(t, f, prefix); !slices.Equal(got, want) {
		t.Errorf("effects %v, want %v: the catch-up from the start of the log", got, want)
	}
	if offset, _ := f.cursor(t, eventbus.TopicSystemEvents); offset != end-1 {
		t.Errorf("effects cursor %d, want %d", offset, end-1)
	}
	for n := 1; n <= 5; n++ {
		_, ok := f.model.Character(player(n))
		if kept := base+int64(n)-1 >= start; ok != kept {
			t.Errorf("%s in the projection: %v, want %v (offset %d, the log starts at %d)", player(n), ok, kept, base+int64(n)-1, start)
		}
	}
}

// deleteRecords moves the start of the log of the only partition of topic to
// before, as DeleteRecords (Kafka API key 21) does, and returns the start the
// broker reports. kafka-go has no client call for it; the request and the
// response are its protocol messages of versions 0 and 1.
func (b *broker) deleteRecords(t *testing.T, topic string, before int64) int64 {
	t.Helper()
	registerDeleteRecords.Do(func() { protocol.Register(&deleteRecordsRequest{}, &deleteRecordsResponse{}) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	transport := &kafka.Transport{}
	defer transport.CloseIdleConnections()
	res, err := transport.RoundTrip(ctx, kafka.TCP(b.rp.Broker), &deleteRecordsRequest{
		Topics:    []deleteRecordsTopic{{Name: topic, Partitions: []deleteRecordsPartition{{Offset: before}}}},
		TimeoutMs: 10_000,
	})
	if err != nil {
		t.Fatalf("delete the records of %s before %d: %v", topic, before, err)
	}
	answer, ok := res.(*deleteRecordsResponse)
	if !ok || len(answer.Topics) != 1 || len(answer.Topics[0].Partitions) != 1 {
		t.Fatalf("answer to the deletion of the records of %s: %#v", topic, res)
	}
	p := answer.Topics[0].Partitions[0]
	if p.ErrorCode != 0 {
		t.Fatalf("delete the records of %s before %d: %v", topic, before, kafka.Error(p.ErrorCode))
	}
	return p.LowWatermark
}

var registerDeleteRecords sync.Once

type deleteRecordsRequest struct {
	Topics    []deleteRecordsTopic `kafka:"min=v0,max=v1"`
	TimeoutMs int32                `kafka:"min=v0,max=v1"`
}

// ApiKey is the key of the message in the protocol of kafka-go.
func (r *deleteRecordsRequest) ApiKey() protocol.ApiKey { return protocol.DeleteRecords }

type deleteRecordsTopic struct {
	Name       string                   `kafka:"min=v0,max=v1"`
	Partitions []deleteRecordsPartition `kafka:"min=v0,max=v1"`
}

type deleteRecordsPartition struct {
	PartitionIndex int32 `kafka:"min=v0,max=v1"`
	Offset         int64 `kafka:"min=v0,max=v1"`
}

type deleteRecordsResponse struct {
	ThrottleTimeMs int32                        `kafka:"min=v0,max=v1"`
	Topics         []deleteRecordsResponseTopic `kafka:"min=v0,max=v1"`
}

// ApiKey is the key of the message in the protocol of kafka-go.
func (r *deleteRecordsResponse) ApiKey() protocol.ApiKey { return protocol.DeleteRecords }

type deleteRecordsResponseTopic struct {
	Name       string                           `kafka:"min=v0,max=v1"`
	Partitions []deleteRecordsResponsePartition `kafka:"min=v0,max=v1"`
}

type deleteRecordsResponsePartition struct {
	PartitionIndex int32 `kafka:"min=v0,max=v1"`
	LowWatermark   int64 `kafka:"min=v0,max=v1"`
	ErrorCode      int16 `kafka:"min=v0,max=v1"`
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

// scene is a consumer on Redpanda with the effects of the deliveries and a
// turns.Tracker that publishes into the same bus, and an ack that goes through
// the middleware of the gateway as telegram-bot. Its turn has a narrative for
// player-A and player-B and a decision for player-A: the narrative of player-A
// is acknowledged with the broker up, and batch — the narrative of player-B,
// the last recipient, and the decision — is leased and not acknowledged yet.
type scene struct {
	f       *fixture
	fd      *feed
	d       *consumer.Dispatcher
	tracked *attempts
	agent   *eventbus.AgentRef
	ack     func(ids ...string) (int, string, time.Duration)
	mux     *http.ServeMux
	act     string
	// batch was leased at leasedAt on the wall clock.
	batch         []string
	lastNarrative string
	leasedAt      time.Time
}

// newScene builds the scene of act over f. The outbox leases and acknowledges
// on now: the manual clock of f, or a clock that follows the wall.
func (b *broker) newScene(t *testing.T, f *fixture, act string, now clock.Clock) *scene {
	t.Helper()
	ctx := context.Background()
	m := seeded(t, f.model)
	s := &scene{f: f, act: act, tracked: &attempts{Bus: b.bus},
		agent: &eventbus.AgentRef{ID: "narrator:solo:player-A", Level: "task", Blueprint: "narrator"}}
	s.fd = &feed{fixture: f, links: linkRows{"player-A": links.PlatformTelegram, "player-B": links.PlatformTelegram}}
	var err error
	if s.fd.queue, err = outbox.New(outbox.Config{DB: f.db}); err != nil {
		t.Fatal(err)
	}
	sessions, err := session.New(session.Config{DB: f.db})
	if err != nil {
		t.Fatal(err)
	}
	if s.fd.tracker, err = turns.New(turns.Config{DB: f.db, Sessions: sessions, Bus: s.tracked, Clock: f.clock}); err != nil {
		t.Fatal(err)
	}
	effects, err := (&consumer.Deliveries{Model: m, Outbox: s.fd.queue, Links: s.fd.links, Turns: s.fd.tracker, Clock: f.clock}).Effects()
	if err != nil {
		t.Fatal(err)
	}
	s.d = b.dispatcher(t, f, effects)
	svc, err := outbox.NewService(outbox.ServiceConfig{Store: s.fd.queue, Routes: s.fd.links, Clock: now, Timers: clock.RealTimers{}})
	if err != nil {
		t.Fatal(err)
	}
	s.ack, s.mux = acker(t, &handlers.Deliveries{Service: svc, Turns: s.fd.tracker, Platforms: handlers.ClientPlatforms, Log: discardLog})

	s.fd.turnOf(t, act, "player-A", "look")
	f.start(t, s.d, b.ends(t))
	narrative := told(act+"-n-1", act, "entry", outbox.GeneratedByLLM, "player-A", "player-B")
	narrative.Meta.Agent = s.agent
	decision := combat(act+"-c-1", solo("player-A"), "player-A", "wolf-alpha")
	decision.Meta.Agent, decision.Source = s.agent, contracts.SourceSwarm
	// Two topics are two subscriptions, and the gateway keeps the order within a
	// topic only (C-05 p. 8): the decision is published once the narrative is
	// in the queue, so that it stands behind it.
	b.publish(t, narrative)
	within(t, joinLimit, "the narrative of both recipients", func() bool { return len(s.fd.rows(t)) == 2 })
	b.publish(t, decision)
	within(t, joinLimit, "the decision of player-A", func() bool { return len(s.fd.rows(t)) == 3 })

	leased, err := s.fd.queue.Lease(ctx, "telegram-bot", links.PlatformTelegram, 10, now.Now())
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
	if code, body, _ := s.ack(idOf(leased, "player-A")); code != http.StatusOK {
		t.Fatalf("ack of the first recipient with the broker up = %d %s", code, body)
	}
	s.lastNarrative = idOf(leased, "player-B")
	s.leasedAt = testkit.Wall().Now()
	next, err := s.fd.queue.Lease(ctx, "telegram-bot", links.PlatformTelegram, 10, now.Now())
	if err != nil || len(next) != 1 || next[0].Kind != outbox.KindMechanics {
		t.Fatalf("second lease = %+v, %v; want the decision of player-A behind its narrative", next, err)
	}
	s.batch = []string{s.lastNarrative, next[0].ID}
	return s
}

// ackUntilTaken repeats the ack of the batch every 250 ms until the gateway
// takes it, and logs how long that took after began.
func (s *scene) ackUntilTaken(t *testing.T, began time.Time, what string) {
	t.Helper()
	wall := testkit.Wall()
	refused := 0
	deadline := testkit.After(reconnectLimit)
	for {
		code, body, took := s.ack(s.batch...)
		if code == http.StatusOK {
			t.Logf("T316 measure: the repeat of the ack was taken %s after %s, after %d refusals; the last ack took %s",
				wall.Now().Sub(began).Round(time.Millisecond), what, refused, took.Round(time.Millisecond))
			if !strings.Contains(body, `"acked":2`) {
				t.Logf("T473 measure: the repeat of the ack answered %s", body)
			}
			return
		}
		if code != http.StatusServiceUnavailable {
			t.Fatalf("repeat of the ack = %d %s", code, body)
		}
		refused++
		select {
		case <-deadline:
			t.Fatalf("the ack was refused %d times over %s after %s", refused, reconnectLimit, what)
		case <-testkit.After(250 * time.Millisecond):
		}
	}
}

// While the broker is away, the ack of the last recipient of a narrative
// answers 503 bus_unavailable within the deadline of the publication, rolls
// the whole ack back and leaves gateway.db free; the deliveries are given
// again once their lease ran out. Once the broker is back, the repeat of the
// ack publishes analytics.turn.completed under the id every attempt had — on
// the broker possibly more than once under that id (oneTurnCompleted) — and no
// delivery is lost (acceptance of T-307; review #1 of T-307, decision
// 2). The same consumer then hears every one of its topics again without a
// restart (T-473). The durations are logged with the prefixes "T316 measure"
// and "T473 measure".
func (b *broker) outage(t *testing.T) {
	ctx := context.Background()
	wall := testkit.Wall()
	f := newFixture(t, membus.Chaos{})
	analytics, ok := contracts.Default().Lookup(turns.TypeCompleted)
	if !ok {
		t.Fatalf("%s is not registered", turns.TypeCompleted)
	}
	analyticsBase := b.end(t, analytics.Topic)
	s := b.newScene(t, f, "act-look", f.clock)

	grace := 10 * time.Second
	began := wall.Now()
	if err := b.rp.Container().Stop(ctx, &grace); err != nil {
		t.Fatalf("stop the broker: %v", err)
	}
	t.Logf("T316 measure: the broker stopped in %s", wall.Now().Sub(began).Round(time.Millisecond))

	const tries = 3
	for i := range tries {
		code, body, took := s.ack(s.batch...)
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
	row, _, err := turns.Load(ctx, f.db, s.act)
	if err != nil || row.Status != turns.StatusNarrated || row.DeliveredCount != 1 {
		t.Errorf("turn after the failed acks %+v, %v; want narrated with one delivery", row, err)
	}

	f.clock.Advance(outbox.DefaultLease + time.Second)
	again, err := s.fd.queue.Lease(ctx, "telegram-bot", links.PlatformTelegram, 10, f.clock.Now())
	if err != nil || len(again) != 2 {
		t.Fatalf("lease after the lease ran out = %+v, %v; want both deliveries of the failed ack again", again, err)
	}
	for _, d := range again {
		if !slices.Contains(s.batch, d.ID) || d.Attempts != 2 {
			t.Errorf("given again %s after %d attempts, want a delivery of the batch on its second attempt", d.ID, d.Attempts)
		}
	}

	began = wall.Now()
	if err := b.rp.Container().Start(ctx); err != nil {
		t.Fatalf("start the broker again: %v", err)
	}
	back := wall.Now()
	t.Logf("T316 measure: the broker started again in %s", back.Sub(began).Round(time.Millisecond))
	s.ackUntilTaken(t, back, "the start of the broker")
	if code, body, _ := s.ack(s.batch...); code != http.StatusOK || !strings.Contains(body, `"acked":0`) {
		t.Errorf("a second repeat of the ack = %d %s, want nothing acknowledged again", code, body)
	}

	var published []eventbus.Event
	if _, err := b.bus.ReadRange(ctx, analytics.Topic, analyticsBase, analyticsBase+1000, func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == turns.TypeCompleted && ev.CorrelationID() == s.act {
			published = append(published, ev)
		}
		return nil
	}); err != nil {
		t.Fatalf("read %s: %v", analytics.Topic, err)
	}
	asked, ids := s.tracked.made()
	t.Logf("T316 measure: %d publications of turn.completed attempted under %d id(s); %d on the broker", asked, len(ids), len(published))
	if msg := oneTurnCompleted(published, ids); msg != "" {
		t.Error(msg)
	}
	for _, r := range s.fd.rows(t) {
		if r.State != outbox.StateDelivered {
			t.Errorf("delivery of %s %s left %s", r.Player, r.Kind, r.State)
		}
	}
	if row, _, err := turns.Load(ctx, f.db, s.act); err != nil || row.Status != turns.StatusCompleted || row.DeliveredCount != 2 {
		t.Errorf("turn after the broker came back %+v, %v; want completed with two deliveries", row, err)
	}

	// Step 8: the same consumer, not started again, hears every one of its
	// topics after the outage (T-473).
	b.hearsEveryTopicAgain(t, f, s.d, back, "outage-after", s.agent)
	t.Logf("T473 measure: the consumer that heard every topic again after the outage stopped in %s",
		stop(t, s.d).Round(time.Millisecond))
}

// A broker that does not answer — its container paused — instead of one that
// refuses (T-473, decision on Mi-2 of the acceptance of T-316). Measured: how
// long the ack of the last recipient takes, whether gateway.db is free while
// it runs, and how often the deliveries of the failed ack are given again to a
// loop that does what the loop of telegram-bot does, on the wall clock. After
// the broker is unpaused the same consumer hears every topic again. Then the
// broker is restarted once more: a consumer stopped while it subscribes again
// stops within runtime.StopTimeout, and a consumer started after that outage
// is timed to its first event of every topic.
func (b *broker) hungBroker(t *testing.T) {
	ctx := context.Background()
	wall := testkit.Wall()
	f := newFixture(t, membus.Chaos{})
	now := &wallClock{from: f.clock.Now(), began: wall.Now(), wall: wall}
	s := b.newScene(t, f, "act-hang", now)

	docker, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}
	t.Cleanup(func() { _ = docker.Close() })
	id := b.rp.Container().GetContainerID()
	paused := false
	t.Cleanup(func() {
		if paused {
			_ = dockerCall(context.Background(), docker.ContainerUnpause, id)
		}
	})
	began := wall.Now()
	if err := dockerCall(ctx, docker.ContainerPause, id); err != nil {
		t.Fatalf("pause the broker: %v", err)
	}
	paused = true
	t.Logf("T473 measure: the broker paused in %s", wall.Now().Sub(began).Round(time.Millisecond))

	type answer struct {
		code int
		body string
		took time.Duration
	}
	acked := make(chan answer, 1)
	go func() {
		code, body, took := s.ack(s.batch...)
		acked <- answer{code, body, took}
	}()
	<-testkit.After(time.Second)
	probe, cancel := context.WithTimeout(ctx, time.Second)
	var pending int
	probeBegan := wall.Now()
	probeErr := f.db.QueryRowContext(probe, `SELECT COUNT(*) FROM deliveries WHERE state = 'pending'`).Scan(&pending)
	probeTook := wall.Now().Sub(probeBegan)
	cancel()
	first := <-acked
	when := "after the ack had answered"
	if first.took > time.Second {
		when = "while the ack still ran"
	}
	t.Logf("T473 measure: ack of the last recipient with the broker hung: %d in %s (%s)", first.code, first.took.Round(time.Millisecond), first.body)
	t.Logf("T473 measure: gateway.db read 1 s into that ack, %s: the read took %s, error %v, pending %d",
		when, probeTook.Round(time.Millisecond), probeErr, pending)
	if first.code != http.StatusServiceUnavailable {
		t.Errorf("ack with the broker hung = %d %s, want 503", first.code, first.body)
	}
	if first.took > turns.DefaultPublishTimeout+time.Second {
		t.Errorf("ack with the broker hung took %s, want no longer than the deadline of the publication %s", first.took, turns.DefaultPublishTimeout)
	}

	s.botLoopUntilGivenAgain(t)

	began = wall.Now()
	if err := dockerCall(ctx, docker.ContainerUnpause, id); err != nil {
		t.Fatalf("unpause the broker: %v", err)
	}
	paused = false
	back := wall.Now()
	t.Logf("T473 measure: the broker unpaused in %s", back.Sub(began).Round(time.Millisecond))
	s.ackUntilTaken(t, back, "the broker was unpaused")
	b.hearsEveryTopicAgain(t, f, s.d, back, "hang-after", s.agent)

	grace := 10 * time.Second
	if err := b.rp.Container().Stop(ctx, &grace); err != nil {
		t.Fatalf("stop the broker: %v", err)
	}
	if err := b.rp.Container().Start(ctx); err != nil {
		t.Fatalf("start the broker again: %v", err)
	}
	restarted := wall.Now()
	ended := testkit.After(reconnectLimit)
	for waiting := true; waiting && s.d.Err() == nil; {
		select {
		case <-ended:
			t.Logf("T473 measure: no subscription ended within %s of the restart of the broker", reconnectLimit)
			waiting = false
		case <-testkit.After(20 * time.Millisecond):
		}
	}
	// A subscription started again after the broker came back joins a group
	// whose members from before are still on the books, and the broker holds
	// the join up to their rebalance timeout. The time since the restart does
	// not say where the readers are — a reader may sit in the JoinGroupBackoff
	// of kafka-go or in the pause of the dispatcher (Mi-5 of review #2 of
	// T-473) — so Stop waits for the proof of a held join: the group in
	// PreparingRebalance, which only a JoinGroup of a reader of this consumer,
	// the only live member, puts it in. A reader inside a held JoinGroup does
	// not return, so Stop takes the grace. Without that state seen, the stop
	// is only measured.
	held, states := b.awaitGroupState(t, "PreparingRebalance", resubscribeLimit)
	heldAfter := wall.Now().Sub(restarted)
	down := s.d.Err()
	took := stop(t, s.d)
	t.Logf("T473 measure: %s after the restart of the broker the group was seen %v (held join: %v, a subscription down: %v); the consumer, stopped then, stopped in %s",
		heldAfter.Round(time.Millisecond), states, held, down, took.Round(time.Millisecond))
	if held && took < consumer.StopTransportGrace {
		t.Errorf("the consumer stopped in %s during a held join of its group, less than the grace %s", took, consumer.StopTransportGrace)
	}

	g := newFixture(t, membus.Chaos{})
	fresh := b.newConsumerOfDeliveries(t, g)
	began = wall.Now()
	g.start(t, fresh, b.ends(t))
	offsets := b.publishOnEveryTopic(t, "fresh-after", s.agent)
	within(t, resubscribeLimit, "the consumer started after the outage to take an event of every topic", func() bool {
		return taken(t, g, offsets)
	})
	t.Logf("T473 measure: a consumer started after the outage took an event of every topic through its subscriptions %s after its Start (%s after the restart of the broker)",
		wall.Now().Sub(began).Round(time.Millisecond), wall.Now().Sub(restarted).Round(time.Millisecond))
	if err := fresh.Err(); err != nil {
		t.Errorf("a subscription of the consumer started after the outage is down: %v", err)
	}
	t.Logf("T473 measure: the consumer started after the outage stopped in %s", stop(t, fresh).Round(time.Millisecond))

	b.stopRightAfterStart(t)
}

// stopRightAfterStart stops a consumer right after its Start, while its
// readers join the group, within runtime.StopTimeout (T-316, run 1: more than
// 10 s; finding 2: 29.7 s), and then closes the bus it ran on. It comes last:
// the readers it leaves joining hold the next consumer of the group for a
// rebalance. The consumer runs on a bus of its own, so that the close of the
// bus the process does after the stop of its contexts is measured with them
// (Mi-1 of review #1 of T-473).
func (b *broker) stopRightAfterStart(t *testing.T) {
	wall := testkit.Wall()
	own, err := eventbus.NewKafka(eventbus.KafkaConfig{
		Brokers: b.rp.Brokers(), Registry: contracts.Default(), Backoff: retries, Log: discardLog,
	})
	if err != nil {
		t.Fatalf("new kafka bus: %v", err)
	}
	f := newFixture(t, membus.Chaos{})
	d, err := consumer.New(consumer.Config{
		Bus: own, Journal: own, DB: f.db, Model: f.model, Clock: f.clock, Timers: clock.RealTimers{}, Log: discardLog,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Start(context.Background(), b.ends(t)); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Logf("T473 measure: a consumer stopped right after its Start, while its readers join the group, stopped in %s",
		stop(t, d).Round(time.Millisecond))
	began := wall.Now()
	err = own.Close()
	closed := wall.Now().Sub(began)
	t.Logf("T473 measure: the bus of that consumer closed after its Stop in %s (error %v)", closed.Round(time.Millisecond), err)
	// A Stop with a live deadline leaves the subscriptions time to take their
	// readers off the books of the bus, and Close does not wait for a join
	// (N-6 of review #2 of T-473; a Stop at an expired deadline may not).
	if closed > consumer.StopTransportGrace {
		t.Errorf("the bus closed in %s after a Stop with a live deadline, want no longer than %s: Close waited for a reader joining its group", closed, consumer.StopTransportGrace)
	}
}

// botLoopUntilGivenAgain runs what deliver.Loop.Once of telegram-bot does —
// acknowledge what is kept, long-poll, keep what came, acknowledge — with the
// batch already kept, as the bot keeps a batch it sent, until the last
// narrative of the batch is given again, and logs the period on the wall
// clock. The client repeats a 503 with client.DefaultBackoff, like the bot.
func (s *scene) botLoopUntilGivenAgain(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), hangLimit)
	defer cancel()
	wall := testkit.Wall()
	srv := httptest.NewServer(s.mux)
	defer srv.Close()
	gw := client.New(srv.URL, "telegram-bot")

	pending := slices.Clone(s.batch)
	var flushes []time.Duration
	flush := func() {
		if len(pending) == 0 || ctx.Err() != nil {
			return
		}
		began := wall.Now()
		_, err := gw.Ack(ctx, pending)
		flushes = append(flushes, wall.Now().Sub(began).Round(time.Millisecond))
		if err == nil {
			pending = nil
		}
	}
	cursor := ""
	for ctx.Err() == nil {
		flush()
		res, err := gw.Deliveries(ctx, cursor, 10, 25*time.Second)
		if err != nil {
			select {
			case <-ctx.Done():
			case <-testkit.After(time.Second):
			}
			continue
		}
		if res.Cursor != "" {
			cursor = res.Cursor
		}
		for _, d := range res.Deliveries {
			if d.ID == s.lastNarrative {
				t.Logf("T473 measure: with the broker hung the deliveries of the failed ack were given again %s after their lease, on the wall clock; the acks of the bot took %v",
					wall.Now().Sub(s.leasedAt).Round(time.Millisecond), flushes)
				return
			}
			if !slices.Contains(pending, d.ID) {
				pending = append(pending, d.ID)
			}
		}
		flush()
	}
	t.Errorf("the deliveries of the failed ack were not given again within %s of their lease; the acks of the bot took %v", hangLimit, flushes)
}

// newConsumerOfDeliveries is a consumer over g with the effects of the
// deliveries in the world of seeded, which takes the events of
// publishOnEveryTopic without an error.
func (b *broker) newConsumerOfDeliveries(t *testing.T, g *fixture) *consumer.Dispatcher {
	t.Helper()
	m := seeded(t, g.model)
	queue, err := outbox.New(outbox.Config{DB: g.db})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := session.New(session.Config{DB: g.db})
	if err != nil {
		t.Fatal(err)
	}
	tracker, err := turns.New(turns.Config{DB: g.db, Sessions: sessions, Bus: b.bus, Clock: g.clock})
	if err != nil {
		t.Fatal(err)
	}
	routes := linkRows{"player-A": links.PlatformTelegram, "player-B": links.PlatformTelegram}
	effects, err := (&consumer.Deliveries{Model: m, Outbox: queue, Links: routes, Turns: tracker, Clock: g.clock}).Effects()
	if err != nil {
		t.Fatal(err)
	}
	return b.dispatcher(t, g, effects)
}

// hearsEveryTopicAgain publishes an event into every topic of the consumer
// after the broker came back at back, and waits until the same consumer took
// each through its subscriptions and reports no topic down (d.Err() == nil).
// Each of the events reaches gateway.db once.
func (b *broker) hearsEveryTopicAgain(t *testing.T, f *fixture, d *consumer.Dispatcher, back time.Time, prefix string, agent *eventbus.AgentRef) {
	t.Helper()
	wall := testkit.Wall()
	if err := d.Err(); err != nil {
		t.Logf("T473 measure: %s after the broker was back a subscription was down: %v", wall.Now().Sub(back).Round(time.Millisecond), err)
	}
	offsets := b.publishOnEveryTopic(t, prefix, agent)
	published := wall.Now()
	heard := make(map[string]time.Duration, len(offsets))
	var seen error
	deadline := testkit.After(resubscribeLimit)
	for {
		for topic, offset := range offsets {
			if _, ok := heard[topic]; !ok {
				if cursor, ok := f.cursor(t, topic); ok && cursor >= offset {
					heard[topic] = wall.Now().Sub(back).Round(time.Millisecond)
				}
			}
		}
		err := d.Err()
		if err != nil {
			seen = err
		}
		if err == nil && len(heard) == len(offsets) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("after %s the consumer did not hear every topic again: taken after the broker was back %v of %v, Err %v, effects cursors %v",
				resubscribeLimit, heard, offsets, err, b.cursors(t, f))
		case <-testkit.After(20 * time.Millisecond):
		}
	}
	t.Logf("T473 measure: the same consumer heard every topic again %s after the broker was back (%s after the events were published); each topic after the broker was back %v; a subscription seen down meanwhile: %v",
		wall.Now().Sub(back).Round(time.Millisecond), wall.Now().Sub(published).Round(time.Millisecond), heard, seen)
	for _, id := range eventIDsOnEveryTopic(prefix) {
		if n := f.count(t, `SELECT COUNT(*) FROM processed_events WHERE event_id = ?`, id); n != 1 {
			t.Errorf("%d marks of %s, want 1", n, id)
		}
	}
}

// taken says whether the effects cursor of every topic reached its offset.
func taken(t *testing.T, f *fixture, offsets map[string]int64) bool {
	t.Helper()
	for topic, offset := range offsets {
		if cursor, ok := f.cursor(t, topic); !ok || cursor < offset {
			return false
		}
	}
	return true
}

// wallClock is the clock of the outbox that follows the wall clock from the
// moment of the manual clock of the fixture: the leases of a loop that runs on
// the wall clock run out on it, and the rows the fixture wrote keep their
// times.
type wallClock struct {
	from, began time.Time
	wall        clock.Clock
}

func (c *wallClock) Now() time.Time { return c.from.Add(c.wall.Now().Sub(c.began)) }

// dockerCall calls pause or unpause of the Docker client on the container of
// this test with the zero options of the call: the options type lives in the
// client package of moby, which the test does not import (go.mod lists it as
// indirect).
func dockerCall[O, R any](ctx context.Context, call func(context.Context, string, O) (R, error), id string) error {
	var options O
	_, err := call(ctx, id, options)
	return err
}

// eventIDsOnEveryTopic names the events publishOnEveryTopic publishes under
// prefix.
func eventIDsOnEveryTopic(prefix string) []string {
	return []string{prefix + "-system", prefix + "-game", prefix + "-world", prefix + "-narrative"}
}

// publishOnEveryTopic publishes one event into each topic of the consumer and
// returns the offset of each; the events are ones the effects of the
// deliveries take without an error in the world of seeded. A broker that has
// just come back may refuse a publication or the read of End: each is
// repeated every 250 ms within reconnectLimit.
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
		deadline := testkit.After(reconnectLimit)
		for {
			end, err := b.bus.End(context.Background(), topic)
			if err == nil {
				if err = b.bus.Publish(context.Background(), ev); err == nil {
					offsets[topic] = end
					break
				}
			}
			select {
			case <-deadline:
				t.Fatalf("publish %s %s within %s: %v", ev.Type, ev.ID, reconnectLimit, err)
			case <-testkit.After(250 * time.Millisecond):
			}
		}
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
// and returns the status, the body and how long the request took, and the mux
// the routes of the deliveries are mounted on.
func acker(t *testing.T, h *handlers.Deliveries) (func(ids ...string) (int, string, time.Duration), *http.ServeMux) {
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
	}, mux
}

// dispatcher is a consumer of the tests on the broker. Its timers are real:
// the pauses between its subscriptions and the grace of its Stop run on the
// wall clock, as in the process.
func (b *broker) dispatcher(t *testing.T, f *fixture, effects map[string][]consumer.Effect) *consumer.Dispatcher {
	t.Helper()
	d, err := consumer.New(consumer.Config{
		Bus: b.bus, Journal: b.bus, DB: f.db, Model: f.model, Clock: f.clock, Timers: clock.RealTimers{}, Log: discardLog,
		Effects: effects,
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

// stop stops a consumer within runtime.StopTimeout, the budget of the stop of
// the whole process, and returns how long it took (T-473: a consumer whose
// readers of kafka-go are joining their group stops within it too).
func stop(t *testing.T, d *consumer.Dispatcher) time.Duration {
	t.Helper()
	wall := testkit.Wall()
	began := wall.Now()
	ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	if err := d.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	return wall.Now().Sub(began)
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
