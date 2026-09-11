package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"
)

func TestNewKafkaNeedsBrokersAndARegistry(t *testing.T) {
	deterministicSources(t)

	if _, err := NewKafka(KafkaConfig{Registry: testRegistry()}); err == nil {
		t.Error("a bus was built without brokers")
	}
	if _, err := NewKafka(KafkaConfig{Brokers: []string{"localhost:9092"}}); !errors.Is(err, ErrNoRegistry) {
		t.Errorf("err = %v, want ErrNoRegistry: without a registry the bus cannot route a type", err)
	}
}

func TestNewKafkaFallsBackToThePackageRegistry(t *testing.T) {
	deterministicSources(t)
	SetRegistry(testRegistry())

	bus, err := NewKafka(KafkaConfig{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("new kafka: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
}

// newTestBus builds a bus that never reaches the network: every test below
// exercises a path that fails before a message is written.
func newTestBus(t *testing.T) *Kafka {
	t.Helper()
	bus, err := NewKafka(KafkaConfig{
		Brokers:  []string{"127.0.0.1:9092"},
		Registry: testRegistry(),
	})
	if err != nil {
		t.Fatalf("new kafka: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func TestWriterSendsEachEventWithoutWaitingForABatch(t *testing.T) {
	deterministicSources(t)
	bus := newTestBus(t)

	// The defaults of kafka-go (100 messages, 1 s) would make every publish
	// of MVP-1 wait for the batch timer, because publication is one event at
	// a time — a second per event, against the 300 ms of NFR-001. Only the
	// configuration is checked here; the latency itself belongs to the
	// contract test of T-014, which has a broker.
	w, err := bus.writer(TopicPlayerEvents)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	if w.BatchSize != 1 {
		t.Errorf("batch size = %d, want 1: a batch of MVP-1 never fills up", w.BatchSize)
	}
	if w.BatchTimeout != kafkaBatchTimeout || w.BatchTimeout > 100*time.Millisecond {
		t.Errorf("batch timeout = %s, want %s", w.BatchTimeout, kafkaBatchTimeout)
	}
	if w.Async {
		t.Error("the writer is asynchronous: Publish would report success before the broker acked")
	}
	if same, err := bus.writer(TopicPlayerEvents); err != nil || same != w {
		t.Errorf("a second writer was built for the same topic (%v)", err)
	}
}

func TestSubscribeReturnsNilWhenTheCallerStopsIt(t *testing.T) {
	deterministicSources(t)
	bus := newTestBus(t)

	// A shutdown is not a failure: the runtime stops a context by cancelling
	// it, and membus reports the same nil in T-014.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := bus.Subscribe(ctx, TopicPlayerEvents, "core.state", func(context.Context, Event) error { return nil })
	if err != nil {
		t.Errorf("err = %v, want nil on a cancelled context", err)
	}
	if err := bus.Tail(ctx, TopicPlayerEvents, 0, func(context.Context, Event) error { return nil }); err != nil {
		t.Errorf("tail err = %v, want nil on a cancelled context", err)
	}
}

func TestStoppedTellsAShutdownFromAFailure(t *testing.T) {
	cases := map[string]struct {
		cancelled bool
		err       error
		want      bool
	}{
		"cancelled context": {cancelled: true, err: errHandler, want: true},
		"closed reader":     {err: io.EOF, want: true},
		"broken pipe":       {err: io.ErrClosedPipe, want: true},
		"real failure":      {err: errHandler, want: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			if got := stopped(ctx, tc.err); got != tc.want {
				t.Errorf("stopped = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPublishRefusesAnUnknownTypeBeforeTheNetwork(t *testing.T) {
	deterministicSources(t)
	bus := newTestBus(t)

	ev := NewRoot("player.teleported", "gateway", "w1", nil, ActorHuman, nil)
	if err := bus.Publish(t.Context(), ev); !errors.Is(err, ErrUnknownType) {
		t.Errorf("err = %v, want ErrUnknownType", err)
	}
}

func TestPublishRefusesAPolicyViolationBeforeTheNetwork(t *testing.T) {
	deterministicSources(t)
	bus := newTestBus(t)

	ev := NewRoot("player.attacked", "gateway", "w1", nil, ActorSystem, nil)
	if err := bus.Publish(t.Context(), ev); !errors.Is(err, ErrPolicyViolation) {
		t.Errorf("err = %v, want ErrPolicyViolation", err)
	}
}

func TestPublishRefusesABrokenEnvelopeBeforeTheNetwork(t *testing.T) {
	deterministicSources(t)
	bus := newTestBus(t)

	ev := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	ev.Source = ""
	if err := bus.Publish(t.Context(), ev); !errors.Is(err, ErrInvalidEnvelope) {
		t.Errorf("err = %v, want ErrInvalidEnvelope", err)
	}
}

func TestSubscribeRejectsAnEmptyGroup(t *testing.T) {
	deterministicSources(t)
	bus := newTestBus(t)

	if err := bus.Subscribe(t.Context(), TopicPlayerEvents, "", func(_ context.Context, _ Event) error { return nil }); err == nil {
		t.Error("a subscription without a consumer group was accepted")
	}
}

func TestReadRangeOfAnEmptyIntervalReadsNothing(t *testing.T) {
	deterministicSources(t)
	bus := newTestBus(t)

	next, err := bus.ReadRange(t.Context(), TopicPlayerEvents, 12, 12, func(_ context.Context, _ Event) error {
		t.Error("the handler was called for an empty interval")
		return nil
	})
	if err != nil {
		t.Fatalf("read range: %v", err)
	}
	if next != 12 {
		t.Errorf("next = %d, want 12", next)
	}
}

func TestJournalRejectsANegativeOffset(t *testing.T) {
	deterministicSources(t)
	bus := newTestBus(t)

	if _, err := bus.ReadRange(t.Context(), TopicPlayerEvents, -1, 10, nil); err == nil {
		t.Error("ReadRange accepted a negative offset")
	}
	if err := bus.Tail(t.Context(), TopicPlayerEvents, -1, nil); err == nil {
		t.Error("Tail accepted a negative offset")
	}
}

func TestCloseIsIdempotentAndRefusesLaterWork(t *testing.T) {
	deterministicSources(t)
	bus, err := NewKafka(KafkaConfig{Brokers: []string{"127.0.0.1:9092"}, Registry: testRegistry()})
	if err != nil {
		t.Fatalf("new kafka: %v", err)
	}

	if err := bus.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Errorf("the second close reported %v, want nil", err)
	}
	ev := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	if err := bus.Publish(t.Context(), ev); !errors.Is(err, ErrClosed) {
		t.Errorf("err = %v, want ErrClosed", err)
	}
}

// The read loops of the adapter — Subscribe, ReadRange and Tail — hand every
// message to deliver and commit or advance only on its nil. This is that step
// with a panicking handler, the part of the kafka path that runs without a
// broker; the loops themselves meet the panic in the contract case on
// Redpanda.
func TestKafkaDeliverParksAHandlerPanic(t *testing.T) {
	deterministicSources(t)
	bus := newTestBus(t)
	sink := &recordingSink{}
	d := bus.delivery("core.state")
	d.DLQ = sink
	d.Backoff = noPause
	ev := validEvent(t)
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	calls := 0
	err = bus.deliver(t.Context(), d, Position{Topic: TopicPlayerEvents, Offset: 4}, body,
		func(context.Context, Event) error {
			calls++
			panic("boom")
		})

	if err != nil {
		t.Fatalf("deliver: %v, want nil so that the offset is committed", err)
	}
	if calls != 1 {
		t.Errorf("handler called %d times, want 1", calls)
	}
	if letters := sink.all(); len(letters) != 1 || letters[0].Original.ID != ev.ID || letters[0].Attempts != 1 {
		t.Errorf("dead letters = %+v, want the event parked after one call", letters)
	}
}

func TestDeadLetterSinkWrapsTheOriginal(t *testing.T) {
	deterministicSources(t)
	ev := NewRoot("player.attacked", "gateway", "dark-forest-world", nil, ActorHuman, nil)

	dl := DeadLetter{Original: ev, Error: "boom", Consumer: "core.state", Attempts: 4}
	body, err := json.Marshal(dl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The wrapper is written raw to dead_letters, not published: its own type
	// is not in the registry and the type of the original routes elsewhere.
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"original", "error", "consumer", "attempts"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("the dead letter has no %q field", key)
		}
	}
	if got := dl.Original.Key(); got != "dark-forest-world" {
		t.Errorf("key = %q, want the world of the original", got)
	}
}
