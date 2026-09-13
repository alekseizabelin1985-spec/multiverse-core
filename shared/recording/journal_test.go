package recording_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/recording"
	"multiverse-core.io/shared/testkit"
)

var t0 = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

var player = map[string]any{"id": "player-A", "type": "player"}

func looked(name string) eventbus.Event {
	return eventbus.NewRoot("player.looked", contracts.SourceTestkitGateway, "dark-forest-world",
		nil, eventbus.ActorCI, map[string]any{"entity": map[string]any{"entity": player, "name": name}})
}

func said(text string) eventbus.Event {
	return eventbus.NewRoot("player.said", contracts.SourceTestkitGateway, "dark-forest-world",
		nil, eventbus.ActorCI, map[string]any{"entity": map[string]any{"entity": player, "name": "A"}, "text": text})
}

// newBus is membus over the topics of the registry, the transport of
// --bus=memory. skipValidate turns validation on read off, as
// MV_BUS_VALIDATE_ON_READ=false does.
func newBus(t *testing.T, skipValidate bool) *membus.Bus {
	t.Helper()
	testkit.Deterministic(t, t.Name())
	eventbus.SetRegistry(contracts.Default())
	names := make([]string, 0)
	for _, spec := range contracts.Topics() {
		names = append(names, spec.Name)
	}
	bus, err := membus.New(membus.Config{
		Registry:           contracts.Default(),
		Topics:             names,
		SkipValidateOnRead: skipValidate,
		Backoff:            []time.Duration{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func publish(t *testing.T, bus eventbus.Bus, events ...eventbus.Event) {
	t.Helper()
	for _, ev := range events {
		if err := bus.Publish(context.Background(), ev); err != nil {
			t.Fatalf("publish %s: %v", ev.Type, err)
		}
	}
}

// journalHook is a journal that lets a test act at the two moments ReadJournal
// cannot be interrupted from outside: right after it took the end, and inside
// the read after an event was delivered.
type journalHook struct {
	eventbus.Journal
	afterEnd       func()
	afterDelivered func(n int)
}

func (j journalHook) End(ctx context.Context, topic string) (int64, error) {
	end, err := j.Journal.End(ctx, topic)
	if j.afterEnd != nil {
		j.afterEnd()
	}
	return end, err
}

func (j journalHook) ReadRange(ctx context.Context, topic string, from, to int64, h eventbus.Handler) (int64, error) {
	delivered := 0
	return j.Journal.ReadRange(ctx, topic, from, to, func(ctx context.Context, ev eventbus.Event) error {
		err := h(ctx, ev)
		delivered++
		if j.afterDelivered != nil {
			j.afterDelivered(delivered)
		}
		return err
	})
}

// C-01 v1.9: ReadJournal reads [from, End) in the order of the offsets, events
// of every type on the topic, and a read from the middle starts there.
func TestReadJournalReadsTheTopicInOffsetOrder(t *testing.T) {
	bus := newBus(t, false)
	events := []eventbus.Event{looked("one"), said("two"), looked("three"), said("four")}
	publish(t, bus, events...)
	topic := eventbus.TopicPlayerEvents

	for _, from := range []int64{0, 2} {
		rec, err := recording.ReadJournal(context.Background(), bus, topic, from)
		if err != nil {
			t.Fatalf("ReadJournal(from %d): %v", from, err)
		}
		if want := int64(len(events)) - from; int64(rec.Len()) != want {
			t.Fatalf("from %d: Len = %d, want End - from = %d", from, rec.Len(), want)
		}
		i := from
		for ev := range rec.Events() {
			if ev.ID != events[i].ID || ev.Type != events[i].Type {
				t.Errorf("from %d: offset %d is %s %s, want %s %s", from, i, ev.Type, ev.ID, events[i].Type, events[i].ID)
			}
			i++
		}
	}
}

func TestReadJournalAtOrPastTheEndIsEmpty(t *testing.T) {
	bus := newBus(t, false)
	publish(t, bus, looked("one"))
	for _, from := range []int64{1, 5} {
		rec, err := recording.ReadJournal(context.Background(), bus, eventbus.TopicPlayerEvents, from)
		if err != nil || rec == nil || rec.Len() != 0 {
			t.Errorf("ReadJournal(from %d) = %v, %v; want an empty recording and no error", from, rec, err)
		}
	}
}

// The end is taken once, before the read: what is published after it belongs
// to the next read, or a busy topic would never let this one finish.
func TestReadJournalLeavesOutWhatIsPublishedAfterTheEnd(t *testing.T) {
	bus := newBus(t, false)
	publish(t, bus, looked("one"), said("two"))
	late := looked("late")
	j := journalHook{Journal: bus, afterEnd: func() { publish(t, bus, late) }}

	rec, err := recording.ReadJournal(context.Background(), j, eventbus.TopicPlayerEvents, 0)
	if err != nil {
		t.Fatalf("ReadJournal: %v", err)
	}
	if rec.Len() != 2 {
		t.Errorf("Len = %d, want the 2 events before the end", rec.Len())
	}
	for ev := range rec.Events() {
		if ev.ID == late.ID {
			t.Error("the event published after the end was taken is in the recording")
		}
	}
	if end, _ := bus.End(context.Background(), eventbus.TopicPlayerEvents); end != 3 {
		t.Fatalf("End = %d after the late publication, want 3: the probe did not publish", end)
	}
}

// membus answers a Close in the middle of ReadRange with the offset it reached
// and no error. A recording cut there would pass for the whole topic, so it is
// an error naming both the offset and the end.
func TestReadJournalRefusesARecordingCutByClose(t *testing.T) {
	bus := newBus(t, false)
	publish(t, bus, looked("one"), said("two"), looked("three"))
	j := journalHook{Journal: bus, afterDelivered: func(n int) {
		if n == 1 {
			_ = bus.Close()
		}
	}}

	rec, err := recording.ReadJournal(context.Background(), j, eventbus.TopicPlayerEvents, 0)
	if err == nil {
		t.Fatalf("ReadJournal after Close = %d events and no error, want an error", rec.Len())
	}
	if want := "recording: journal player_events read stopped at 1 of 3"; err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
	if rec != nil {
		t.Error("a partial recording came back together with the error")
	}
}

// A cancelled ctx is an error even when the read reached the end: the caller is
// stopping, and a recording it asked for under a cancelled context is not one
// it can rely on having been read to the end on every transport.
func TestReadJournalRefusesACancelledContext(t *testing.T) {
	bus := newBus(t, false)
	publish(t, bus, looked("one"), said("two"))

	t.Run("before the read", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rec, err := recording.ReadJournal(ctx, bus, eventbus.TopicPlayerEvents, 0)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadJournal = %v, want context.Canceled in the chain", err)
		}
		if rec != nil {
			t.Error("a recording came back together with the error")
		}
		if !strings.Contains(err.Error(), "read stopped at 0 of 2") {
			t.Errorf("error %q does not name where the read stopped", err)
		}
	})
	t.Run("after the last event", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		j := journalHook{Journal: bus, afterDelivered: func(n int) {
			if n == 2 {
				cancel()
			}
		}}
		rec, err := recording.ReadJournal(ctx, j, eventbus.TopicPlayerEvents, 0)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReadJournal = %v, want context.Canceled in the chain", err)
		}
		if rec != nil {
			t.Error("a recording read to the end came back together with the error of the cancelled context")
		}
		if !strings.Contains(err.Error(), "read stopped at 2 of 2") {
			t.Errorf("error %q does not name where the read stopped", err)
		}
	})
}

// The errors of End and ReadRange stay in the chain.
func TestReadJournalWrapsTheErrorsOfTheJournal(t *testing.T) {
	bus := newBus(t, false)
	if rec, err := recording.ReadJournal(context.Background(), bus, "no_such_topic", 0); !errors.Is(err, membus.ErrNoTopic) || rec != nil {
		t.Errorf("ReadJournal(unknown topic) = %v, %v; want no recording and membus.ErrNoTopic in the chain", rec, err)
	}

	publish(t, bus, looked("one"), said("two"))
	boom := errors.New("broker gone")
	// The journal delivers the first event and fails on the second: the part
	// already read must not come back with the error.
	failing := failingRange{Journal: bus, err: boom}
	rec, err := recording.ReadJournal(context.Background(), failing, eventbus.TopicPlayerEvents, 0)
	if !errors.Is(err, boom) {
		t.Fatalf("ReadJournal = %v, want the error of ReadRange in the chain", err)
	}
	if rec != nil {
		t.Errorf("a partial recording of %d events came back together with the error", rec.Len())
	}
	if !strings.HasPrefix(err.Error(), "recording: journal player_events read stopped at 1 of 2: ") {
		t.Errorf("error %q, want the prefix of the package, the offset and the end", err)
	}
}

type failingRange struct {
	eventbus.Journal
	err error
}

// ReadRange delivers the first event of the range and fails before the next.
func (f failingRange) ReadRange(ctx context.Context, topic string, from, _ int64, h eventbus.Handler) (int64, error) {
	next, err := f.Journal.ReadRange(ctx, topic, from, from+1, h)
	if err != nil {
		return next, err
	}
	return next, f.err
}

// markSpy is a journal that notes whether the context of each call and of each
// handler it delivers to carries the mark of ReadJournal.
type markSpy struct {
	eventbus.Journal
	call, handler []bool
}

func (m *markSpy) ReadRange(ctx context.Context, topic string, from, to int64, h eventbus.Handler) (int64, error) {
	m.call = append(m.call, recording.InReadJournal(ctx))
	return m.Journal.ReadRange(ctx, topic, from, to, func(ctx context.Context, ev eventbus.Event) error {
		m.handler = append(m.handler, recording.InReadJournal(ctx))
		return h(ctx, ev)
	})
}

// C-01 v1.11: ReadJournal reads under a context marked for InReadJournal, the
// mark the replay middleware lets through unchanged. The mark stays on the
// read: the context of the caller does not carry it after the call, and a
// ReadRange with a handler of the caller does not carry it at all.
func TestReadJournalMarksTheContextOfItsReadOnly(t *testing.T) {
	bus := newBus(t, false)
	publish(t, bus, looked("one"), said("two"))
	spy := &markSpy{Journal: bus}
	ctx := context.Background()

	if _, err := recording.ReadJournal(ctx, spy, eventbus.TopicPlayerEvents, 0); err != nil {
		t.Fatalf("ReadJournal: %v", err)
	}
	if len(spy.call) != 1 || !spy.call[0] {
		t.Errorf("the ReadRange of ReadJournal got marks %v, want one call under the mark", spy.call)
	}
	if len(spy.handler) != 2 || !spy.handler[0] || !spy.handler[1] {
		t.Errorf("the handler of ReadJournal saw marks %v, want the mark on both events", spy.handler)
	}
	if recording.InReadJournal(ctx) {
		t.Error("the context passed to ReadJournal carries the mark after the call")
	}

	spy.call, spy.handler = nil, nil
	if _, err := spy.ReadRange(ctx, eventbus.TopicPlayerEvents, 0, 2, func(context.Context, eventbus.Event) error {
		return nil
	}); err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if len(spy.handler) != 2 || spy.handler[0] || spy.handler[1] || spy.call[0] {
		t.Errorf("a ReadRange with a handler of its own saw marks %v (call %v), want none", spy.handler, spy.call)
	}
	if recording.InReadJournal(context.Background()) {
		t.Error("an empty context carries the mark")
	}
}

// llmRecord is a payload of llm.output valid by its schema.
func llmRecord(phase string, attempt any) map[string]any {
	return map[string]any{
		"phase": phase, "attempt": attempt,
		"provider": "openai_compat", "model": "qwen3-8b",
		"params":        map[string]any{"temperature": 0.7},
		"prompt_hash":   "sha256:" + strings.Repeat("a", 64),
		"response_hash": "sha256:" + strings.Repeat("b", 64),
		"response_raw":  "{}", "response_len": 2,
		"validation_status": "valid",
		"filter":            map[string]any{"applied": true, "status": "pass", "filter_version": "a-2026-09"},
		"laws_version":      "v1", "latency_ms": 10,
		"tokens":   map[string]any{"prompt": 1, "completion": 1},
		"cost_usd": 0,
	}
}

// C-01 v1.9: the handler of ReadJournal never refuses. A record without a key
// is kept and left to the consumer; parsing it in the handler would park it in
// dead_letters and drop it from the recording.
//
// Validation on read is off for this bus: by the schema and the swarm policy an
// llm.output always has an agent, a phase and a whole attempt, so a record
// without a key reaches a reader only past a validation that is switched off
// (MV_BUS_VALIDATE_ON_READ=false) or skipped by the producer. A validating bus
// would park it before the handler of ReadJournal ever ran, and the test
// would prove nothing about that handler.
func TestReadJournalKeepsLLMOutputWithoutAKey(t *testing.T) {
	bus := newBus(t, true)
	agent := eventbus.WithAgent(eventbus.AgentRef{ID: "encounter-wolf:solo:A", Level: "task", Blueprint: "encounter-wolf"})
	whole := eventbus.NewRoot(recording.TypeLLMOutput, contracts.SourceLLM, "dark-forest-world", nil,
		eventbus.ActorSystem, llmRecord("decision", 1), agent)
	publish(t, bus, whole)

	noPhase := eventbus.NewRoot(recording.TypeLLMOutput, contracts.SourceLLM, "dark-forest-world", nil,
		eventbus.ActorSystem, llmRecord("", 1), agent)
	delete(noPhase.Payload, "phase")
	fraction := eventbus.NewRoot(recording.TypeLLMOutput, contracts.SourceLLM, "dark-forest-world", nil,
		eventbus.ActorSystem, llmRecord("decision", 1.5), agent)
	for _, ev := range []eventbus.Event{noPhase, fraction} {
		body, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		if err := bus.Append(eventbus.TopicLLMRecords, body); err != nil {
			t.Fatal(err)
		}
	}

	rec, err := recording.ReadJournal(context.Background(), bus, eventbus.TopicLLMRecords, 0)
	if err != nil {
		t.Fatalf("ReadJournal: %v", err)
	}
	if rec.Len() != 3 {
		t.Errorf("Len = %d, want 3: records without a key are part of the recording", rec.Len())
	}
	if end, _ := bus.End(context.Background(), eventbus.TopicDeadLetters); end != 0 {
		t.Errorf("dead_letters holds %d records: the handler of ReadJournal refused an event", end)
	}
	index := rec.Index(recording.TypeLLMOutput, recording.LLMOutputKeyOf)
	if len(index) != 1 {
		t.Errorf("index has %d records, want only the one with a whole key", len(index))
	}
	if _, ok := index[recording.LLMOutputKey(whole.CorrelationID(), "encounter-wolf:solo:A", "decision", 1)]; !ok {
		t.Error("the record with a whole key is not in the index")
	}
}
