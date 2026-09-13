package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/replay"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/runtime"
)

// recordedAt is the timestamp of the first event of the recordings written
// here: far from any wall clock, so that a time taken from the machine cannot
// pass for it.
var recordedAt = time.Date(2031, 5, 17, 8, 30, 0, 0, time.UTC)

func replayOptions(recording string) serveOptions {
	opts := memoryOptions()
	opts.mode = runtime.ModeReplay
	opts.recording = recording
	return opts
}

// writeSession writes a recording of two events, the first at recordedAt.
func writeSession(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	w, err := replay.NewWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, at := range []time.Time{recordedAt, recordedAt.Add(time.Minute)} {
		ev := lookedEvent("recorded")
		ev.ID, ev.Meta.CorrelationID, ev.Timestamp = "rec-"+string(rune('1'+i)), "rec-1", at
		if err := w.Append(ev); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func lookedEvent(name string) eventbus.Event {
	return eventbus.NewRoot("player.looked", contracts.SourceTestkitGateway, "dark-forest-world",
		nil, eventbus.ActorCI, map[string]any{
			"entity": map[string]any{
				"entity": map[string]any{"id": "player-A", "type": "player"},
				"name":   name,
			},
		})
}

// timersSpy is openBus that keeps the timers the process handed the bus.
func timersSpy(got *clock.Timers, calls *atomic.Int32) openBusFunc {
	return func(bus string, reg *contracts.Registry, timers clock.Timers, log *slog.Logger) (transport, error) {
		calls.Add(1)
		*got = timers
		return openBus(bus, reg, timers, log)
	}
}

// C-01 v1.4 and the DoD of T-060, both modes side by side: the contexts get
// the time of their mode, the bus gets real timers in either. In replay the
// contexts get an EventClock standing at the first recorded event and
// NullTimers, and the constructors stamp a root event with that clock rather
// than with the machine's — "no time.Now in replay" as a behaviour and not
// only as a lint rule.
func TestTheModeDecidesTheTimeOfTheContextsAndNotOfTheBus(t *testing.T) {
	recording := writeSession(t)
	tests := map[string]struct {
		opts   serveOptions
		inside func(t *testing.T, deps runtime.Deps)
	}{
		"live": {
			opts: memoryOptions(),
			inside: func(t *testing.T, deps runtime.Deps) {
				if _, ok := deps.Clock.(clock.Real); !ok {
					t.Errorf("Deps.Clock = %T, want clock.Real in live mode", deps.Clock)
				}
				if _, ok := deps.Timers.(clock.RealTimers); !ok {
					t.Errorf("Deps.Timers = %T, want clock.RealTimers in live mode", deps.Timers)
				}
			},
		},
		"replay": {
			opts: replayOptions(recording),
			inside: func(t *testing.T, deps runtime.Deps) {
				ec, ok := deps.Clock.(*replay.EventClock)
				if !ok {
					t.Fatalf("Deps.Clock = %T, want *replay.EventClock in replay", deps.Clock)
				}
				if _, ok := deps.Timers.(replay.NullTimers); !ok {
					t.Errorf("Deps.Timers = %T, want replay.NullTimers in replay", deps.Timers)
				}
				if now := ec.Now(); !now.Equal(recordedAt) {
					t.Errorf("the replay clock starts at %v, want the first recorded event %v", now, recordedAt)
				}
				if ts := lookedEvent("built in replay").Timestamp; !ts.Equal(recordedAt) {
					t.Errorf("a root event built in replay is stamped %v, want the event time %v: "+
						"the constructors still read the wall clock", ts, recordedAt)
				}
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			onLoopback(t)
			var busTimers clock.Timers
			var calls atomic.Int32
			started := make(chan runtime.Deps, 1)
			p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}},
				timersSpy(&busTimers, &calls))
			p.opts = tc.opts

			runUntilStarted(t, p, started, func(deps runtime.Deps) {
				if _, ok := busTimers.(clock.RealTimers); !ok {
					t.Errorf("the bus got %T, want clock.RealTimers: its redelivery pauses are not domain time (C-01 v1.4)", busTimers)
				}
				tc.inside(t, deps)
			})
		})
	}
	// The replay clock belongs to the run: once it is over, the constructors
	// are back on the wall clock.
	if ts := lookedEvent("after").Timestamp; ts.Equal(recordedAt) {
		t.Error("the replay clock is still installed in the constructors after the run ended")
	}
}

// In replay a context reads through the bus of the process — by the journal
// and by a subscription, the way most contexts will read — and its handler sees
// the clock moved to the event it handles, with the event marked replayed. Deps
// carries one wrapped transport in both fields: a journal wrapped while the bus
// stays raw would pass the first half and leave every subscriber on the clock
// of the last journal read (review #1 of T-060, Mi-1).
func TestReplayMovesTheClockOfTheContextsByTheEventsTheyRead(t *testing.T) {
	onLoopback(t)
	started := make(chan runtime.Deps, 1)
	p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}}, openBus)
	p.opts = replayOptions(writeSession(t))

	runUntilStarted(t, p, started, func(deps runtime.Deps) {
		later := lookedEvent("later")
		later.Timestamp = recordedAt.Add(time.Hour)
		if err := deps.Bus.Publish(context.Background(), later); err != nil {
			t.Fatalf("publish: %v", err)
		}
		spec, _ := deps.Contracts.Lookup("player.looked")
		var inHandler time.Time
		var replayed bool
		if _, err := deps.Journal.ReadRange(context.Background(), spec.Topic, 0, 1, func(_ context.Context, ev eventbus.Event) error {
			inHandler, replayed = deps.Clock.Now(), ev.Meta.Replay
			return nil
		}); err != nil {
			t.Fatalf("read: %v", err)
		}
		if !inHandler.Equal(later.Timestamp) {
			t.Errorf("the clock in the handler is %v, want the event time %v", inHandler, later.Timestamp)
		}
		if !replayed {
			t.Error("the event reached the handler without meta.replay")
		}

		if any(deps.Bus) != any(deps.Journal) {
			t.Errorf("Deps.Bus (%T) and Deps.Journal (%T) are two transports in replay", deps.Bus, deps.Journal)
		}

		// One reader only, so the clock in the handler is exactly the time of
		// the event read; with several readers it may be later (package doc).
		viaBus := lookedEvent("via bus")
		viaBus.Timestamp = recordedAt.Add(2 * time.Hour)
		type seenBySubscriber struct {
			now      time.Time
			replayed bool
		}
		got := make(chan seenBySubscriber, 1)
		ctx, cancel := context.WithCancel(context.Background())
		stopped := make(chan error, 1)
		go func() {
			stopped <- deps.Bus.Subscribe(ctx, spec.Topic, "replay-probe", func(_ context.Context, ev eventbus.Event) error {
				if name, _ := ev.Path().GetString("entity.name"); name == "via bus" {
					got <- seenBySubscriber{now: deps.Clock.Now(), replayed: ev.Meta.Replay}
				}
				return nil
			})
		}()
		if err := deps.Bus.Publish(context.Background(), viaBus); err != nil {
			cancel()
			t.Fatalf("publish: %v", err)
		}
		select {
		case s := <-got:
			if !s.now.Equal(viaBus.Timestamp) {
				t.Errorf("the clock in the subscriber is %v, want the event time %v: Deps.Bus is not wrapped", s.now, viaBus.Timestamp)
			}
			if !s.replayed {
				t.Error("the event reached the subscriber without meta.replay: Deps.Bus is not wrapped")
			}
		case err := <-stopped:
			t.Fatalf("Subscribe returned before the event arrived: %v", err)
		case <-clock.RealTimers{}.After(10 * time.Second).C():
			t.Fatal("the subscriber got nothing within 10s")
		}
		cancel()
		if err := <-stopped; err != nil {
			t.Errorf("Subscribe: %v", err)
		}
	})
}

// failing is a context that subscribes in Start with a handler that always
// fails, the case review #1 of T-410 found: in replay the first redelivery
// waited on a timer nothing moved.
type failing struct {
	calls   atomic.Int32
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started chan runtime.Deps
}

func (f *failing) Name() string           { return "failing" }
func (f *failing) DependsOn() []string    { return nil }
func (f *failing) Health() runtime.Status { return runtime.OK() }

func (f *failing) Start(_ context.Context, deps runtime.Deps) error {
	spec, _ := deps.Contracts.Lookup("player.looked")
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		_ = deps.Bus.Subscribe(ctx, spec.Topic, "failing", func(context.Context, eventbus.Event) error {
			f.calls.Add(1)
			return errors.New("handler failed")
		})
	}()
	f.started <- deps
	return nil
}

func (f *failing) Stop(context.Context) error {
	f.cancel()
	f.wg.Wait()
	return nil
}

// The DoD of T-060 (C-01 v1.4): in replay a failing handler is redelivered three
// times without hanging and the event is parked in dead_letters. The pauses are
// shortened to a millisecond, but they are still measured by the timers the
// process handed the bus — NullTimers there would hang this test.
func TestReplayRedeliversAFailingHandlerWithoutHanging(t *testing.T) {
	onLoopback(t)
	f := &failing{started: make(chan runtime.Deps, 1)}
	open := func(_ string, reg *contracts.Registry, timers clock.Timers, log *slog.Logger) (transport, error) {
		names := make([]string, 0)
		for _, topic := range reg.Topics() {
			names = append(names, topic.Name)
		}
		return membus.New(membus.Config{
			Registry: reg,
			Topics:   names,
			Backoff:  []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
			Timers:   timers,
			Log:      log,
		})
	}
	p := newProcess([]runtime.Context{f}, open)
	p.opts = replayOptions(writeSession(t))

	runUntilStarted(t, p, f.started, func(deps runtime.Deps) {
		if err := deps.Bus.Publish(context.Background(), lookedEvent("fails")); err != nil {
			t.Fatalf("publish: %v", err)
		}
		deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for {
			end, err := deps.Journal.End(context.Background(), eventbus.TopicDeadLetters)
			if err != nil {
				t.Fatalf("End(dead_letters): %v", err)
			}
			if end == 1 {
				break
			}
			select {
			case <-deadline.Done():
				t.Fatalf("no dead letter within 10s after %d calls: the redelivery hangs in replay", f.calls.Load())
			case <-clock.RealTimers{}.After(5 * time.Millisecond).C():
			}
		}
		if got := f.calls.Load(); got != 4 {
			t.Errorf("handler called %d times, want 4 — the call and three retries, as in live mode", got)
		}
	})
}

// A recording that cannot be read refuses the start, before a transport is
// opened: a replay that went on without it would run on the zero time and look
// like one that read it.
func TestReplayRefusesAnUnreadableRecording(t *testing.T) {
	onLoopback(t)
	var busTimers clock.Timers
	var calls atomic.Int32
	p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}}}, timersSpy(&busTimers, &calls))
	missing := filepath.Join(t.TempDir(), "absent.jsonl")
	p.opts = replayOptions(missing)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := p.run(ctx, cancel)
	if err == nil || !strings.Contains(err.Error(), "--recording") || !strings.Contains(err.Error(), "absent.jsonl") {
		t.Fatalf("run = %v, want a refusal naming --recording and the file", err)
	}
	if calls.Load() != 0 {
		t.Error("the bus was opened for a replay whose recording could not be read")
	}
}

// The recording a replay runs on is part of its start record, like the mode and
// the bus (T-408): a recording applied silently is indistinguishable from one
// ignored silently.
func TestReplayLogsTheRecordingItRunsOn(t *testing.T) {
	onLoopback(t)
	started := make(chan runtime.Deps, 1)
	p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}}, openBus)
	path := writeSession(t)
	p.opts = replayOptions(path)
	var logged strings.Builder
	var mu sync.Mutex
	p.log = slog.New(slog.NewJSONHandler(&lockedWriter{w: &logged, mu: &mu}, nil))

	runUntilStarted(t, p, started, nil)

	mu.Lock()
	defer mu.Unlock()
	for line := range strings.Lines(logged.String()) {
		var rec struct {
			Msg       string `json:"msg"`
			Recording string `json:"recording"`
			Recorded  *int   `json:"recorded_events"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil || rec.Msg != "multiverse started" {
			continue
		}
		if rec.Recording != path {
			t.Errorf("the start record names the recording %q, want %q", rec.Recording, path)
		}
		if rec.Recorded == nil || *rec.Recorded != 2 {
			t.Errorf("the start record says %v recorded events, want 2", rec.Recorded)
		}
		return
	}
	t.Fatalf("no start record in the log: %q", logged.String())
}

// Replay without --recording starts (a run on fakes needs no recording, and
// MV_MODE=replay has no variable for one), but it warns that the clock starts
// at the zero time; a replay on a recording does not warn (acceptance of T-060).
func TestReplayWithoutARecordingStartsAndWarns(t *testing.T) {
	for name, tc := range map[string]struct {
		recording bool
		warns     bool
	}{
		"without a recording": {recording: false, warns: true},
		"on a recording":      {recording: true, warns: false},
	} {
		t.Run(name, func(t *testing.T) {
			onLoopback(t)
			started := make(chan runtime.Deps, 1)
			p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}}, openBus)
			path := ""
			if tc.recording {
				path = writeSession(t)
			}
			p.opts = replayOptions(path)
			var logged strings.Builder
			var mu sync.Mutex
			p.log = slog.New(slog.NewJSONHandler(&lockedWriter{w: &logged, mu: &mu}, nil))

			runUntilStarted(t, p, started, nil)

			mu.Lock()
			defer mu.Unlock()
			warned := false
			for line := range strings.Lines(logged.String()) {
				var rec struct {
					Level      string    `json:"level"`
					Msg        string    `json:"msg"`
					ClockStart time.Time `json:"clock_start"`
				}
				if err := json.Unmarshal([]byte(line), &rec); err != nil || rec.Msg != "replay without a recording" {
					continue
				}
				warned = true
				if rec.Level != slog.LevelWarn.String() {
					t.Errorf("level = %s, want %s", rec.Level, slog.LevelWarn)
				}
				if !rec.ClockStart.IsZero() {
					t.Errorf("clock_start = %v, want the zero time", rec.ClockStart)
				}
			}
			if warned != tc.warns {
				t.Errorf("warned = %v, want %v; log: %q", warned, tc.warns, logged.String())
			}
		})
	}
}

type lockedWriter struct {
	w  *strings.Builder
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
