package membus_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
)

// noPause is a backoff of three retries that does not wait: the number of
// attempts is what the tests check, not the wall time.
var noPause = []time.Duration{0, 0, 0}

// platformTopics is the topic list of the platform, the one redpanda-init
// creates. A bus built with it refuses an unknown topic, like the broker.
func platformTopics() []string {
	specs := contracts.Topics()
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	return names
}

// newBus builds the bus every test here uses: the real registry, the real
// topics, no waiting between retries.
func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	testkit.Deterministic(t, t.Name())
	eventbus.SetRegistry(contracts.Default())
	bus, err := membus.New(membus.Config{
		Registry: contracts.Default(),
		Topics:   platformTopics(),
		Backoff:  noPause,
	})
	if err != nil {
		t.Fatalf("new membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func looked(world, name string) eventbus.Event {
	return eventbus.NewRoot("player.looked", contracts.SourceTestkitGateway, world,
		nil, eventbus.ActorCI, map[string]any{
			"entity": map[string]any{
				"entity": map[string]any{"id": "player-1", "type": "player"},
				"name":   name,
			},
		})
}

func TestNewNeedsARegistry(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	if _, err := membus.New(membus.Config{}); !errors.Is(err, eventbus.ErrNoRegistry) {
		t.Errorf("new membus without a registry = %v, want ErrNoRegistry", err)
	}
}

func TestNewFallsBackToThePackageRegistry(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	eventbus.SetRegistry(contracts.Default())
	if _, err := membus.New(membus.Config{}); err != nil {
		t.Errorf("new membus with the package registry = %v, want it accepted", err)
	}
}

// A bus built with a topic list behaves like the broker: nothing creates a
// topic by writing to it, because a topic born that way would carry no
// retention (infrastructure.md §5.1).
func TestUnknownTopicIsRefused(t *testing.T) {
	bus := newBus(t)

	if err := bus.Append("no_such_topic", []byte("{}")); !errors.Is(err, membus.ErrNoTopic) {
		t.Errorf("append to an unknown topic = %v, want ErrNoTopic", err)
	}
	if _, err := bus.End(t.Context(), "no_such_topic"); !errors.Is(err, membus.ErrNoTopic) {
		t.Errorf("End of an unknown topic = %v, want ErrNoTopic", err)
	}
	if err := bus.Subscribe(t.Context(), "no_such_topic", "g", func(context.Context, eventbus.Event) error {
		return nil
	}); !errors.Is(err, membus.ErrNoTopic) {
		t.Errorf("Subscribe to an unknown topic = %v, want ErrNoTopic", err)
	}
	if err := bus.Tail(t.Context(), "no_such_topic", 0, func(context.Context, eventbus.Event) error {
		return nil
	}); !errors.Is(err, membus.ErrNoTopic) {
		t.Errorf("Tail of an unknown topic = %v, want ErrNoTopic", err)
	}
}

// Without a topic list the bus creates topics on first use. That is the shape
// a unit test of another package uses, and it is deliberately more permissive
// than the broker.
func TestWithoutATopicListTopicsAppearOnFirstUse(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	eventbus.SetRegistry(contracts.Default())
	bus, err := membus.New(membus.Config{Registry: contracts.Default()})
	if err != nil {
		t.Fatalf("new membus: %v", err)
	}
	defer func() { _ = bus.Close() }()

	if err := bus.Publish(t.Context(), looked("w", "x")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	end, err := bus.End(t.Context(), eventbus.TopicPlayerEvents)
	if err != nil || end != 1 {
		t.Fatalf("End = (%d, %v), want (1, nil)", end, err)
	}
}

// The at-least-once chaos of C-01: every publish is delivered twice, and a
// consumer that does not deduplicate sees both copies.
func TestChaosDuplicateDeliversTwice(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	eventbus.SetRegistry(contracts.Default())
	bus, err := membus.New(membus.Config{
		Registry: contracts.Default(),
		Topics:   platformTopics(),
		Chaos:    membus.Chaos{Duplicate: true},
	})
	if err != nil {
		t.Fatalf("new membus: %v", err)
	}
	defer func() { _ = bus.Close() }()

	ev := looked("w", "x")
	if err := bus.Publish(t.Context(), ev); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var seen []string
	next, err := bus.ReadRange(t.Context(), eventbus.TopicPlayerEvents, 0, 10, func(_ context.Context, got eventbus.Event) error {
		seen = append(seen, got.ID)
		return nil
	})
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if next != 2 || len(seen) != 2 || seen[0] != ev.ID || seen[1] != ev.ID {
		t.Fatalf("read %v (next %d), want the same id twice: --chaos=duplicate makes delivery visibly at-least-once", seen, next)
	}

	window := testkit.NewDedup(0)
	unique := 0
	for _, id := range seen {
		if !window.Seen(id) {
			unique++
		}
	}
	if unique != 1 {
		t.Fatalf("Dedup let %d copies through, want 1", unique)
	}
}

// A closed bus refuses to publish, and a reader parked on an empty topic is
// released rather than left waiting for an event that will never come.
//
// The reader returns nil, not an error: closing the bus is an orderly stop,
// and that is what the kafka adapter reports — its Close closes the readers,
// FetchMessage then fails with io.ErrClosedPipe, and eventbus.stopped turns it
// into a nil return. The first version of membus returned ErrClosed here and
// left the reader parked forever, which is the divergence this case exists to
// hold shut.
func TestCloseReleasesReaders(t *testing.T) {
	bus := newBus(t)

	done := make(chan error, 1)
	go func() {
		done <- bus.Tail(t.Context(), eventbus.TopicPlayerEvents, 0, func(context.Context, eventbus.Event) error {
			return nil
		})
	}()
	time.Sleep(50 * time.Millisecond)

	if err := bus.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Fatalf("closing twice: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Tail on a closed bus = %v, want nil (an orderly stop)", err)
		}
	case <-testkit.After(5 * time.Second):
		t.Fatal("Close left a reader parked on an empty topic")
	}

	if err := bus.Publish(t.Context(), looked("w", "x")); !errors.Is(err, eventbus.ErrClosed) {
		t.Errorf("publish on a closed bus = %v, want ErrClosed", err)
	}
}

func TestNegativeOffsetsAreRejected(t *testing.T) {
	bus := newBus(t)
	h := func(context.Context, eventbus.Event) error { return nil }

	if _, err := bus.ReadRange(t.Context(), eventbus.TopicPlayerEvents, -1, 10, h); err == nil {
		t.Error("ReadRange accepted a negative from")
	}
	if err := bus.Tail(t.Context(), eventbus.TopicPlayerEvents, -1, h); err == nil {
		t.Error("Tail accepted a negative from")
	}
}

func TestSubscribeRejectsAnEmptyGroupAndANilHandler(t *testing.T) {
	bus := newBus(t)

	if err := bus.Subscribe(t.Context(), eventbus.TopicPlayerEvents, "", func(context.Context, eventbus.Event) error {
		return nil
	}); err == nil {
		t.Error("Subscribe accepted an empty consumer group")
	}
	if err := bus.Subscribe(t.Context(), eventbus.TopicPlayerEvents, "g", nil); err == nil {
		t.Error("Subscribe accepted a nil handler")
	}
}

// A cancelled publish never reaches a topic: the caller asked for the process
// to stop, and an event written after that would be replayed by a consumer
// that has no record of it.
func TestPublishRefusesACancelledContext(t *testing.T) {
	bus := newBus(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := bus.Publish(ctx, looked("w", "x")); !errors.Is(err, context.Canceled) {
		t.Errorf("publish with a cancelled context = %v, want context.Canceled", err)
	}
	end, err := bus.End(t.Context(), eventbus.TopicPlayerEvents)
	if err != nil || end != 0 {
		t.Fatalf("End = (%d, %v), want (0, nil): nothing was written", end, err)
	}
}

// panicOn builds a handler that panics on one event and reports every other
// event of the world to handled.
func panicOn(poison eventbus.Event, calls *atomic.Int64, handled chan<- string) eventbus.Handler {
	return func(_ context.Context, ev eventbus.Event) error {
		if ev.ID == poison.ID {
			calls.Add(1)
			panic("membus: the handler panics on this event")
		}
		handled <- ev.ID
		return nil
	}
}

// panicLetter returns the one dead letter parked for poison.
func panicLetter(t *testing.T, bus *membus.Bus, poison eventbus.Event) eventbus.DeadLetter {
	t.Helper()
	letters, err := bus.DeadLetters()
	if err != nil {
		t.Fatalf("dead letters: %v", err)
	}
	if len(letters) != 1 || letters[0].Original.ID != poison.ID {
		t.Fatalf("dead letters = %+v, want the one event the handler panicked on", letters)
	}
	dl := letters[0]
	if dl.Attempts != 1 {
		t.Errorf("attempts = %d, want 1: a panic is not retried", dl.Attempts)
	}
	if !strings.HasPrefix(dl.Error, eventbus.ErrHandlerPanic.Error()) {
		t.Errorf("error = %q, want ErrHandlerPanic", dl.Error)
	}
	return dl
}

// The journal reads through the same Delivery as a subscription, so a panic in
// a catch-up read is parked and the read goes on rather than taking the
// process down (C-01 v1.5).
func TestReadRangeParksAHandlerPanicAndReadsOn(t *testing.T) {
	bus := newBus(t)
	poison, good := looked("w", "poison"), looked("w", "good")
	for _, ev := range []eventbus.Event{poison, good} {
		if err := bus.Publish(t.Context(), ev); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	var calls atomic.Int64
	handled := make(chan string, 2)
	next, err := bus.ReadRange(t.Context(), eventbus.TopicPlayerEvents, 0, 2, panicOn(poison, &calls, handled))

	if err != nil || next != 2 {
		t.Fatalf("ReadRange = (%d, %v), want (2, nil): the parked event counts as read", next, err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("handler called %d times on the event that panicked, want 1", got)
	}
	if len(handled) != 1 || <-handled != good.ID {
		t.Error("the event after the panic was not handled")
	}
	if dl := panicLetter(t, bus, poison); dl.Consumer != "journal."+eventbus.TopicPlayerEvents {
		t.Errorf("consumer = %q, want the journal of the topic", dl.Consumer)
	}
}

func TestTailParksAHandlerPanicAndFollowsOn(t *testing.T) {
	bus := newBus(t)
	poison, good := looked("w", "poison"), looked("w", "good")

	var calls atomic.Int64
	handled := make(chan string, 2)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- bus.Tail(ctx, eventbus.TopicPlayerEvents, 0, panicOn(poison, &calls, handled)) }()

	for _, ev := range []eventbus.Event{poison, good} {
		if err := bus.Publish(t.Context(), ev); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	select {
	case id := <-handled:
		if id != good.ID {
			t.Fatalf("Tail handled %s, want the event after the panic", id)
		}
	case <-testkit.After(5 * time.Second):
		t.Fatal("Tail did not go on past the event the handler panicked on")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Tail returned %v, want nil", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("handler called %d times on the event that panicked, want 1", got)
	}
	panicLetter(t, bus, poison)
}

// The cursor of a group moves past the event its handler panicked on once the
// dead letter is written: the next subscription of the group does not get it
// again, which is the restart loop C-01 v1.5 exists to break.
func TestSubscribeCommitsTheEventItsHandlerPanickedOn(t *testing.T) {
	bus := newBus(t)
	poison, good := looked("w", "poison"), looked("w", "good")
	for _, ev := range []eventbus.Event{poison, good} {
		if err := bus.Publish(t.Context(), ev); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	var calls atomic.Int64
	handled := make(chan string, 4)
	subscribe := func() (context.CancelFunc, <-chan error) {
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- bus.Subscribe(ctx, eventbus.TopicPlayerEvents, "g", panicOn(poison, &calls, handled)) }()
		return cancel, done
	}
	receive := func(want string) {
		t.Helper()
		select {
		case id := <-handled:
			if id != want {
				t.Fatalf("handled %s, want %s", id, want)
			}
		case <-testkit.After(5 * time.Second):
			t.Fatalf("event %s was never handled", want)
		}
	}

	cancel, done := subscribe()
	receive(good.ID)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("the subscription returned %v after a panic, want nil", err)
	}

	marker := looked("w", "marker")
	if err := bus.Publish(t.Context(), marker); err != nil {
		t.Fatalf("publish: %v", err)
	}
	cancel, done = subscribe()
	defer func() { cancel(); <-done }()
	receive(marker.ID)
	if got := calls.Load(); got != 1 {
		t.Errorf("the handler met the poison %d times, want 1: the group got it again after the panic", got)
	}
	if dl := panicLetter(t, bus, poison); dl.Consumer != "g" {
		t.Errorf("consumer = %q, want the group", dl.Consumer)
	}
}

// Records is the raw view a test needs of dead_letters, which cannot be read
// through the journal.
func TestRecordsAndDeadLettersReadTheRawTopic(t *testing.T) {
	bus := newBus(t)

	ev := looked("w", "x")
	if err := bus.Publish(t.Context(), ev); err != nil {
		t.Fatalf("publish: %v", err)
	}
	records, err := bus.Records(eventbus.TopicPlayerEvents)
	if err != nil || len(records) != 1 {
		t.Fatalf("Records = (%d records, %v), want 1", len(records), err)
	}
	var decoded eventbus.Event
	if err := json.Unmarshal(records[0], &decoded); err != nil || decoded.ID != ev.ID {
		t.Fatalf("the raw record does not decode into the published event: %v", err)
	}

	letters, err := bus.DeadLetters()
	if err != nil || len(letters) != 0 {
		t.Fatalf("DeadLetters = (%d, %v), want none yet", len(letters), err)
	}
	if err := bus.Append(eventbus.TopicDeadLetters, []byte("not a dead letter")); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := bus.DeadLetters(); err == nil {
		t.Error("DeadLetters accepted a body that is not a dead letter")
	}
}

// Two subscriptions of the same group share one cursor, as they would on a
// single-partition topic: the events are split between them, never doubled.
func TestOneGroupHandlesEachEventOnce(t *testing.T) {
	bus := newBus(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	seen := make(chan string, 16)
	for range 2 {
		go func() {
			_ = bus.Subscribe(ctx, eventbus.TopicPlayerEvents, "shared", func(_ context.Context, ev eventbus.Event) error {
				seen <- ev.ID
				return nil
			})
		}()
	}

	ids := make(map[string]bool)
	for i := range 4 {
		ev := looked("w", string(rune('a'+i)))
		ids[ev.ID] = false
		if err := bus.Publish(ctx, ev); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	for range 4 {
		select {
		case id := <-seen:
			if ids[id] {
				t.Fatalf("event %s was handled twice by one consumer group", id)
			}
			ids[id] = true
		case <-testkit.After(5 * time.Second):
			t.Fatal("the group did not handle every event")
		}
	}
	select {
	case id := <-seen:
		t.Fatalf("event %s was delivered a second time to the same group", id)
	case <-testkit.After(100 * time.Millisecond):
	}
}

// The view Lenient returns is the same transport read with the validation flag
// in its other position: what one publishes the other reads, and closing one
// closes both. The contract suite uses it to check the half of
// MV_BUS_VALIDATE_ON_READ that the safe default hides.
func TestLenientSharesTheLogAndSkipsValidationOnRead(t *testing.T) {
	bus := newBus(t)
	lenient := bus.Lenient()

	// An envelope no publisher could produce: player.looked.v1 requires
	// entity. The strict bus parks it, the lenient view hands it over.
	invalid := looked("w", "x")
	invalid.Payload = map[string]any{}
	body, err := json.Marshal(invalid)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := bus.Append(eventbus.TopicAnalyticsEvents, body); err != nil {
		t.Fatalf("append: %v", err)
	}

	var seen []string
	if _, err := lenient.ReadRange(t.Context(), eventbus.TopicAnalyticsEvents, 0, 10, func(_ context.Context, ev eventbus.Event) error {
		seen = append(seen, ev.ID)
		return nil
	}); err != nil {
		t.Fatalf("ReadRange on the lenient view: %v", err)
	}
	if len(seen) != 1 || seen[0] != invalid.ID {
		t.Fatalf("the lenient view delivered %v, want the invalid event %q: with validation on read off the handler sees it", seen, invalid.ID)
	}
	letters, err := bus.DeadLetters()
	if err != nil {
		t.Fatalf("dead letters: %v", err)
	}
	if len(letters) != 0 {
		t.Fatalf("the lenient view parked %d events, want none", len(letters))
	}

	if err := bus.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := lenient.Publish(t.Context(), looked("w", "y")); !errors.Is(err, eventbus.ErrClosed) {
		t.Errorf("publish through the view of a closed bus = %v, want ErrClosed: a view is not a second bus", err)
	}
}

// SetChaos turns a failure mode on while the bus runs, which is how a test
// duplicates one publish instead of building a second bus over an empty log.
func TestSetChaosDuplicatesOnePublish(t *testing.T) {
	bus := newBus(t)

	quiet := looked("w", "quiet")
	if err := bus.Publish(t.Context(), quiet); err != nil {
		t.Fatalf("publish: %v", err)
	}
	bus.SetChaos(membus.Chaos{Duplicate: true})
	noisy := looked("w", "noisy")
	if err := bus.Publish(t.Context(), noisy); err != nil {
		t.Fatalf("publish with chaos: %v", err)
	}
	bus.SetChaos(membus.Chaos{})
	after := looked("w", "after")
	if err := bus.Publish(t.Context(), after); err != nil {
		t.Fatalf("publish after chaos: %v", err)
	}

	var seen []string
	if _, err := bus.ReadRange(t.Context(), eventbus.TopicPlayerEvents, 0, 10, func(_ context.Context, ev eventbus.Event) error {
		seen = append(seen, ev.ID)
		return nil
	}); err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	want := []string{quiet.ID, noisy.ID, noisy.ID, after.ID}
	if len(seen) != len(want) {
		t.Fatalf("read %v, want %v", seen, want)
	}
	for i, id := range want {
		if seen[i] != id {
			t.Fatalf("read %v, want %v: the switch duplicates the publishes it is on for, and only those", seen, want)
		}
	}
}

// A cancelled subscription stops at the next record instead of finishing the
// backlog, which is what the broker does — its fetch or its commit fails
// within one event. The contract suite holds the same rule against both
// implementations; this test holds it here, where the divergence was.
func TestCancelStopsTheSubscriptionOnABacklog(t *testing.T) {
	bus := newBus(t)

	const backlog = 40
	for range backlog {
		if err := bus.Publish(t.Context(), looked("w", "backlog")); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var calls atomic.Int64
	done := make(chan error, 1)
	go func() {
		done <- bus.Subscribe(ctx, eventbus.TopicPlayerEvents, "slow", func(context.Context, eventbus.Event) error {
			calls.Add(1)
			time.Sleep(25 * time.Millisecond)
			return nil
		})
	}()
	for calls.Load() < 3 {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a cancelled subscription returned %v, want nil", err)
		}
	case <-testkit.After(5 * time.Second):
		t.Fatal("the cancelled subscription did not return")
	}
	if got := calls.Load(); got >= backlog {
		t.Fatalf("the cancelled subscription handled %d of %d events: it must stop, not drain the topic", got, backlog)
	}
}
