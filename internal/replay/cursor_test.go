package replay_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/replay"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
)

func TestCursorAdvanceNeverGoesBack(t *testing.T) {
	c := replay.Cursor{}
	c.Advance(eventbus.Position{Topic: "system_events", Offset: 4})
	c.Advance(eventbus.Position{Topic: "system_events", Offset: 2}) // redelivered
	c.Advance(eventbus.Position{Topic: "game_events", Offset: 0})
	want := replay.Cursor{"system_events": 5, "game_events": 1}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("cursor = %v, want %v", c, want)
	}
}

func TestCursorMergeAndMin(t *testing.T) {
	a := replay.Cursor{"system_events": 10, "game_events": 3}
	b := replay.Cursor{"system_events": 7, "world_events": 2}

	if got, want := a.Merge(b), (replay.Cursor{"system_events": 10, "game_events": 3, "world_events": 2}); !reflect.DeepEqual(got, want) {
		t.Errorf("Merge = %v, want %v", got, want)
	}
	if got, want := a.Min(b), (replay.Cursor{"system_events": 7, "game_events": 3, "world_events": 2}); !reflect.DeepEqual(got, want) {
		t.Errorf("Min = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(a, replay.Cursor{"system_events": 10, "game_events": 3}) ||
		!reflect.DeepEqual(b, replay.Cursor{"system_events": 7, "world_events": 2}) {
		t.Errorf("Merge or Min changed its operands: %v, %v", a, b)
	}

	clone := a.Clone()
	clone["system_events"] = 0
	if a["system_events"] != 10 {
		t.Error("Clone shares its map with the original")
	}
}

// A cursor goes into a snapshot, and the bytes of a snapshot are compared
// between runs (C-14 v1.2): the same cursor must marshal the same way.
func TestCursorMarshalsDeterministically(t *testing.T) {
	c := replay.Cursor{"world_events": 1, "system_events": 20, "game_events": 3}
	first, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"game_events":3,"system_events":20,"world_events":1}`; string(first) != want {
		t.Errorf("JSON = %s, want %s", first, want)
	}
	var back replay.Cursor
	if err := json.Unmarshal(first, &back); err != nil || !reflect.DeepEqual(back, c) {
		t.Errorf("round trip = %v (%v), want %v", back, err, c)
	}
	if got := c.Topics(); !reflect.DeepEqual(got, []string{"game_events", "system_events", "world_events"}) {
		t.Errorf("Topics = %v, want sorted", got)
	}
}

// --- journal ---

func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	eventbus.SetRegistry(contracts.Default())
	names := make([]string, 0)
	for _, spec := range contracts.Topics() {
		names = append(names, spec.Name)
	}
	bus, err := membus.New(membus.Config{
		Registry: contracts.Default(),
		Topics:   names,
		Backoff:  []time.Duration{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func looked(name string) eventbus.Event {
	return eventbus.NewRoot("player.looked", contracts.SourceTestkitGateway, "dark-forest-world",
		nil, eventbus.ActorCI, map[string]any{
			"entity": map[string]any{
				"entity": map[string]any{"id": "player-A", "type": "player"},
				"name":   name,
			},
		})
}

func topicOf(t *testing.T, typ string) string {
	t.Helper()
	spec, ok := contracts.Default().Lookup(typ)
	if !ok {
		t.Fatalf("%s is not registered", typ)
	}
	return spec.Topic
}

// CatchUp reads each topic of the cursor from where it stands to the end taken
// at the call, and returns the cursor after the read. An event published while
// the catch-up runs is left for the next read: otherwise a busy topic would
// never let the reader leave replay.
func TestCatchUpReadsEachTopicToItsEnd(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	bus := newBus(t)
	topic := topicOf(t, "player.looked")
	for _, name := range []string{"a", "b", "c"} {
		if err := bus.Publish(t.Context(), looked(name)); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	var read []string
	spy := &rangeSpy{Journal: bus}
	start := replay.Cursor{topic: 1, eventbus.TopicSystemEvents: 0}
	got, err := replay.CatchUp(t.Context(), spy, start, func(ctx context.Context, ev eventbus.Event) error {
		name, _ := ev.Path().GetString("entity.name")
		read = append(read, name)
		if name == "b" {
			return bus.Publish(ctx, looked("late"))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("CatchUp: %v", err)
	}
	if !reflect.DeepEqual(read, []string{"b", "c"}) {
		t.Errorf("read %v, want [b c]: from the cursor to the end at the call", read)
	}
	if want := (replay.Cursor{topic: 3, eventbus.TopicSystemEvents: 0}); !reflect.DeepEqual(got, want) {
		t.Errorf("cursor = %v, want %v", got, want)
	}
	if start[topic] != 1 {
		t.Errorf("CatchUp changed the cursor passed in: %v", start)
	}
	// The range asked of the journal is the one fixed at the call. membus
	// clamps a longer range to what it holds when the read starts, so the
	// arguments are checked here and not only what was delivered: a broker
	// reading to a later offset would wait for events that are not there yet.
	if want := []string{topic + " [1, 3)"}; !reflect.DeepEqual(spy.ranges, want) {
		t.Errorf("ranges read = %v, want %v: nothing for an empty topic, and the end taken at the call", spy.ranges, want)
	}

	// The late event is where the returned cursor resumes.
	read = nil
	if _, err := replay.CatchUp(t.Context(), bus, got, func(_ context.Context, ev eventbus.Event) error {
		name, _ := ev.Path().GetString("entity.name")
		read = append(read, name)
		return nil
	}); err != nil || !reflect.DeepEqual(read, []string{"late"}) {
		t.Errorf("second CatchUp read %v (%v), want [late]", read, err)
	}
}

// rangeSpy records the ranges CatchUp asks of the journal.
type rangeSpy struct {
	eventbus.Journal
	ranges []string
}

func (s *rangeSpy) ReadRange(ctx context.Context, topic string, from, to int64, h eventbus.Handler) (int64, error) {
	s.ranges = append(s.ranges, fmt.Sprintf("%s [%d, %d)", topic, from, to))
	return s.Journal.ReadRange(ctx, topic, from, to, h)
}

// A topic the journal does not know stops the catch-up with an error that
// names it, and the cursor keeps what was read before it.
func TestCatchUpKeepsTheProgressOnAnError(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	bus := newBus(t)
	topic := topicOf(t, "player.looked")
	if err := bus.Publish(t.Context(), looked("a")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// "zz_no_such_topic" sorts after the real topic, so the real one is read first.
	got, err := replay.CatchUp(t.Context(), bus, replay.Cursor{topic: 0, "zz_no_such_topic": 0},
		func(context.Context, eventbus.Event) error { return nil })
	if !errors.Is(err, membus.ErrNoTopic) {
		t.Fatalf("CatchUp over an unknown topic = %v, want membus.ErrNoTopic", err)
	}
	if got[topic] != 1 {
		t.Errorf("cursor = %v, want %s at 1: the progress before the failing topic is lost", got, topic)
	}

	if next, err := replay.ReadToEnd(t.Context(), bus, "a_no_such_topic", 5, nil); !errors.Is(err, membus.ErrNoTopic) || next != 5 {
		t.Errorf("ReadToEnd(unknown) = %d, %v; want 5 and membus.ErrNoTopic", next, err)
	}

	// A read that fails half-way through a topic keeps the offset it reached.
	for _, name := range []string{"b", "c"} {
		if err := bus.Publish(t.Context(), looked(name)); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	boom := errors.New("boom")
	got, err = replay.CatchUp(t.Context(), halfway{Journal: bus, err: boom}, replay.Cursor{topic: 0},
		func(context.Context, eventbus.Event) error { return nil })
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), topic) {
		t.Fatalf("CatchUp over a failing read = %v, want boom naming %s", err, topic)
	}
	if got[topic] != 1 {
		t.Errorf("cursor = %v, want %s at 1: the offset reached before the failure is lost", got, topic)
	}
}

// halfway reads one event of the range and then fails, as a broker connection
// lost in the middle of a catch-up does.
type halfway struct {
	eventbus.Journal
	err error
}

func (h halfway) ReadRange(ctx context.Context, topic string, from, _ int64, handler eventbus.Handler) (int64, error) {
	next, err := h.Journal.ReadRange(ctx, topic, from, from+1, handler)
	if err != nil {
		return next, err
	}
	return next, h.err
}
