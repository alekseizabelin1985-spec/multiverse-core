package replay_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/replay"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/recording"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
)

// seen is what a handler observed during one call: the event as handed to it
// and the time of the replay clock at that moment.
type seen struct {
	ev  eventbus.Event
	now time.Time
}

func TestMiddlewareInLiveModeChangesNothing(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	ec := replay.NewEventClock(t0)
	var got eventbus.Event
	h := replay.Middleware(runtime.ModeLive, ec)(func(_ context.Context, ev eventbus.Event) error {
		got = ev
		return nil
	})
	ev := looked("a")
	ev.Timestamp = t0.Add(time.Hour)
	if err := h(t.Context(), ev); err != nil {
		t.Fatal(err)
	}
	if got.Meta.Replay {
		t.Error("live mode marked the event as replayed")
	}
	if !ec.Now().Equal(t0) {
		t.Errorf("live mode moved the replay clock to %v", ec.Now())
	}
	// Live mode never has a replay clock; building its middleware without one
	// is not an error.
	_ = replay.Middleware(runtime.ModeLive, nil)
}

func TestMiddlewareInReplayNeedsAClock(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Middleware(replay, nil) did not panic: the missing clock would surface on the first delivery")
		}
	}()
	replay.Middleware(runtime.ModeReplay, nil)
}

// Through every way a context reads — Subscribe, ReadRange, Tail — the handler
// of a replay sees the clock at the timestamp of the event it handles, and the
// event marked replayed. The clock does not go back for an older event, and
// what the journal holds is left as it was published.
func TestWithMiddlewareDrivesTheClockOnEveryReadPath(t *testing.T) {
	src := testkit.Deterministic(t, t.Name())
	bus := newBus(t)
	topic := topicOf(t, "player.looked")

	stamps := []time.Duration{time.Minute, 3 * time.Minute, 2 * time.Minute} // the last is older
	for i, d := range stamps {
		src.Clock.Set(t0.Add(d))
		if err := bus.Publish(t.Context(), looked(string(rune('a'+i)))); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	wantNow := []time.Time{t0.Add(time.Minute), t0.Add(3 * time.Minute), t0.Add(3 * time.Minute)}

	check := func(t *testing.T, got []seen) {
		t.Helper()
		if len(got) != len(stamps) {
			t.Fatalf("handled %d events, want %d", len(got), len(stamps))
		}
		for i, s := range got {
			if !s.ev.Meta.Replay {
				t.Errorf("event %d reached the handler without meta.replay", i)
			}
			if !s.now.Equal(wantNow[i]) {
				t.Errorf("event %d (timestamp %v): clock at %v in the handler, want %v", i, s.ev.Timestamp, s.now, wantNow[i])
			}
		}
	}
	record := func(ec *replay.EventClock, got *[]seen, mu *sync.Mutex, done chan struct{}) eventbus.Handler {
		return func(_ context.Context, ev eventbus.Event) error {
			mu.Lock()
			defer mu.Unlock()
			*got = append(*got, seen{ev: ev, now: ec.Now()})
			if len(*got) == len(stamps) && done != nil {
				close(done)
			}
			return nil
		}
	}

	t.Run("ReadRange", func(t *testing.T) {
		ec := replay.NewEventClock(t0)
		tr := replay.WithMiddleware(bus, replay.Middleware(runtime.ModeReplay, ec))
		var got []seen
		var mu sync.Mutex
		if _, err := tr.ReadRange(t.Context(), topic, 0, int64(len(stamps)), record(ec, &got, &mu, nil)); err != nil {
			t.Fatal(err)
		}
		check(t, got)
	})

	for name, read := range map[string]func(ctx context.Context, tr replay.Transport, h eventbus.Handler) error{
		"Subscribe": func(ctx context.Context, tr replay.Transport, h eventbus.Handler) error {
			return tr.Subscribe(ctx, topic, "replay-test", h)
		},
		"Tail": func(ctx context.Context, tr replay.Transport, h eventbus.Handler) error {
			return tr.Tail(ctx, topic, 0, h)
		},
	} {
		t.Run(name, func(t *testing.T) {
			ec := replay.NewEventClock(t0)
			tr := replay.WithMiddleware(bus, replay.Middleware(runtime.ModeReplay, ec))
			var got []seen
			var mu sync.Mutex
			done := make(chan struct{})
			ctx, cancel := context.WithCancel(t.Context())
			stopped := make(chan error, 1)
			go func() { stopped <- read(ctx, tr, record(ec, &got, &mu, done)) }()
			select {
			case <-done:
			case err := <-stopped:
				t.Fatalf("%s returned before the events were read: %v", name, err)
			case <-testkit.After(10 * time.Second):
				t.Fatalf("%s did not deliver %d events within 10s", name, len(stamps))
			}
			cancel()
			if err := <-stopped; err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			mu.Lock()
			defer mu.Unlock()
			check(t, got)
		})
	}

	// The mark is put on the copy handed to the handler, not on the log.
	if _, err := bus.ReadRange(t.Context(), topic, 0, 1, func(_ context.Context, ev eventbus.Event) error {
		if ev.Meta.Replay {
			t.Error("the journal holds the event marked replayed: the middleware changed the log")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// C-01 v1.11 (review of T-458 by system-architect, A5): recording.ReadJournal
// is a read of history, not a delivery. Through the middleware of a replay it
// gets the events as the journal holds them — meta.replay as published, true
// or false — and the clock stays where it was. A ReadRange with a handler of
// the caller over the same transport and the same events is a delivery and
// runs under the clock, as in T-060.
func TestMiddlewareInReplayLeavesAReadOfHistoryAsTheJournalHoldsIt(t *testing.T) {
	src := testkit.Deterministic(t, t.Name())
	bus := newBus(t)
	topic := topicOf(t, "player.looked")

	src.Clock.Set(t0.Add(2 * time.Hour))
	plain := looked("plain")
	src.Clock.Set(t0.Add(3 * time.Hour))
	replayed := looked("replayed")
	replayed.Meta.Replay = true
	for _, ev := range []eventbus.Event{plain, replayed} {
		if err := bus.Publish(t.Context(), ev); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	ec := replay.NewEventClock(t0)
	tr := replay.WithMiddleware(bus, replay.Middleware(runtime.ModeReplay, ec))

	rec, err := recording.ReadJournal(t.Context(), tr, topic, 0)
	if err != nil {
		t.Fatalf("ReadJournal: %v", err)
	}
	if got := ec.Now(); !got.Equal(t0) {
		t.Errorf("after ReadJournal the clock is at %v, want it unmoved at %v", got, t0)
	}
	want := []eventbus.Event{plain, replayed}
	if rec.Len() != len(want) {
		t.Fatalf("Len = %d, want %d", rec.Len(), len(want))
	}
	i := 0
	for ev := range rec.Events() {
		if ev.ID != want[i].ID || ev.Meta.Replay != want[i].Meta.Replay || !ev.Timestamp.Equal(want[i].Timestamp) {
			t.Errorf("event %d came out of ReadJournal as %s replay=%t at %v, want %s replay=%t at %v",
				i, ev.ID, ev.Meta.Replay, ev.Timestamp, want[i].ID, want[i].Meta.Replay, want[i].Timestamp)
		}
		i++
	}

	var delivered []eventbus.Event
	if _, err := tr.ReadRange(t.Context(), topic, 0, 2, func(ctx context.Context, ev eventbus.Event) error {
		if recording.InReadJournal(ctx) {
			t.Error("a ReadRange with a handler of the caller runs under the mark of ReadJournal")
		}
		delivered = append(delivered, ev)
		return nil
	}); err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if got := ec.Now(); !got.Equal(replayed.Timestamp) {
		t.Errorf("after a delivery the clock is at %v, want the latest event %v", got, replayed.Timestamp)
	}
	for _, ev := range delivered {
		if !ev.Meta.Replay {
			t.Errorf("the delivery handed %s on without meta.replay", ev.ID)
		}
	}
}

// The middleware runs inside the delivery of the bus, so a failing handler goes
// through it on every retry and the dead letter is still written — replay
// changes neither the number of attempts nor what becomes of the event (C-01
// v1.4).
func TestWithMiddlewareKeepsTheRetriesOfTheBus(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	bus := newBus(t)
	topic := topicOf(t, "player.looked")
	if err := bus.Publish(t.Context(), looked("a")); err != nil {
		t.Fatal(err)
	}
	ec := replay.NewEventClock(time.Time{})
	tr := replay.WithMiddleware(bus, replay.Middleware(runtime.ModeReplay, ec))

	calls := 0
	if _, err := tr.ReadRange(t.Context(), topic, 0, 1, func(_ context.Context, ev eventbus.Event) error {
		calls++
		if !ev.Meta.Replay {
			t.Errorf("call %d without meta.replay", calls)
		}
		return errors.New("boom")
	}); err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if calls != 4 {
		t.Errorf("handler called %d times, want 4 (one call and three retries)", calls)
	}
	if end, err := bus.End(t.Context(), eventbus.TopicDeadLetters); err != nil || end != 1 {
		t.Errorf("dead_letters end = %d (%v), want 1", end, err)
	}
}

// C-07: what a handler derives from a replayed event is replayed too. The
// middleware marks only the event it hands on; the mark reaches the facts,
// decisions and records built from it through eventbus.Derive, which also
// hands them the event's own timestamp — the time rule of the package
// documentation. Both are checked on the event as it lands in the journal
// (review #1 of T-060, Mi-2).
func TestAnEventDerivedInReplayIsMarkedReplayed(t *testing.T) {
	src := testkit.Deterministic(t, t.Name())
	bus := newBus(t)
	topic := topicOf(t, "player.looked")
	src.Clock.Set(t0)
	if err := bus.Publish(t.Context(), looked("cause")); err != nil {
		t.Fatal(err)
	}
	ec := replay.NewEventClock(t0.Add(time.Hour)) // the clock is ahead of the cause
	tr := replay.WithMiddleware(bus, replay.Middleware(runtime.ModeReplay, ec))

	var derived eventbus.Event
	if _, err := tr.ReadRange(t.Context(), topic, 0, 1, func(ctx context.Context, ev eventbus.Event) error {
		derived = eventbus.Derive(ev, "player.looked", contracts.SourceTestkitGateway, ev.Payload)
		return bus.Publish(ctx, derived)
	}); err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if !derived.Meta.Replay {
		t.Error("an event derived from a replayed one is not marked replayed")
	}

	if _, err := bus.ReadRange(t.Context(), topic, 1, 2, func(_ context.Context, ev eventbus.Event) error {
		if ev.ID != derived.ID {
			t.Errorf("journal holds %s at offset 1, want the derived %s", ev.ID, derived.ID)
		}
		if !ev.Meta.Replay {
			t.Error("the derived event reached the journal without meta.replay")
		}
		if !ev.Timestamp.Equal(t0) {
			t.Errorf("the derived event is stamped %v, want its cause's %v, not the clock's %v", ev.Timestamp, t0, ec.Now())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A nil handler is refused by the transport, as it is without the wrapper:
// wrapping must not turn it into a handler that panics on the first event.
func TestWithMiddlewareLeavesANilHandlerToTheTransport(t *testing.T) {
	testkit.Deterministic(t, t.Name())
	bus := newBus(t)
	tr := replay.WithMiddleware(bus, replay.Middleware(runtime.ModeReplay, replay.NewEventClock(t0)))
	if err := tr.Subscribe(t.Context(), topicOf(t, "player.looked"), "g", nil); err == nil {
		t.Error("Subscribe with a nil handler was accepted")
	}
	if err := tr.Close(); err != nil {
		t.Errorf("Close through the wrapper: %v", err)
	}
}
