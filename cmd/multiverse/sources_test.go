package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"multiverse-core.io/internal/replay"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
)

// C-01 v1.4 "Источники конструкторов" and the DoD of T-055: the process
// installs the sources of the event constructors from the objects of Deps —
// the id generator, the clock of the contexts, the registry — in both modes,
// over whatever a test installed before, and takes all three back after the
// run (N-4 of review #1 of T-060 and the conditions of tech-lead#1: one block,
// a test in replay after testkit.Deterministic).
func TestTheProcessInstallsTheSourcesOfTheConstructorsFromDeps(t *testing.T) {
	recording := writeSession(t)
	for name, opts := range map[string]serveOptions{
		"live":   memoryOptions(),
		"replay": replayOptions(recording),
	} {
		t.Run(name, func(t *testing.T) {
			onLoopback(t)
			testkit.Deterministic(t, "installed-by-the-test")
			opts.idSource = "sequence"
			started := make(chan runtime.Deps, 1)
			p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}}, openBus)
			p.opts = opts

			runUntilStarted(t, p, started, func(deps runtime.Deps) {
				ev := lookedEvent("inside the run")
				if !strings.HasPrefix(ev.ID, "seq-") {
					t.Errorf("a root event built inside the run has the id %q, want one of the generator of "+
						"--id-source=sequence (seq-N), not of the test", ev.ID)
				}
				if want := deps.Clock.Now(); !ev.Timestamp.Equal(want) && name == "replay" {
					t.Errorf("a root event built in replay is stamped %v, want the EventClock of Deps at %v", ev.Timestamp, want)
				}
				if name == "live" {
					if ev.Timestamp.Equal(testkit.Epoch) {
						t.Error("a root event built in live mode is stamped with the manual clock of the test, " +
							"want the wall clock of Deps")
					}
					if gap := testkit.Wall().Now().Sub(ev.Timestamp); gap < 0 || gap > time.Hour {
						t.Errorf("a root event built in live mode is stamped %v, want the wall clock", ev.Timestamp)
					}
				}
				if reg := eventbus.PackageRegistry(); reg == nil || reg != eventbus.Registry(deps.Contracts) {
					t.Errorf("the registry of the constructors is %v, want Deps.Contracts", reg)
				}
			})

			after := lookedEvent("after the run")
			if _, err := uuid.Parse(after.ID); err != nil {
				t.Errorf("after the run a root event has the id %q, want a UUID: the generator of the run is still installed", after.ID)
			}
			if after.Timestamp.Equal(testkit.Epoch) || after.Timestamp.Equal(recordedAt) {
				t.Errorf("after the run a root event is stamped %v, want the wall clock", after.Timestamp)
			}
			if reg := eventbus.PackageRegistry(); reg != nil {
				t.Errorf("after the run the registry of the constructors is %v, want none", reg)
			}
		})
	}
}

// closingBus is the transport of the process that builds a root event while it
// closes, the way a dead letter written during the shutdown is built.
type closingBus struct {
	transport
	mu     sync.Mutex
	closed eventbus.Event
}

func (b *closingBus) Close() error {
	b.mu.Lock()
	b.closed = lookedEvent("while the bus closes")
	b.mu.Unlock()
	return b.transport.Close()
}

// N-1 of review #1 of T-055: the sources of the run are taken back after the
// bus has closed, not before — an event built while the bus closes still gets
// an id of the generator of the run.
func TestTheSourcesOutliveTheBusOfTheRun(t *testing.T) {
	onLoopback(t)
	testkit.Deterministic(t, "installed-by-the-test")
	started := make(chan runtime.Deps, 1)
	bus := &closingBus{}
	p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}},
		func(kind string, reg *contracts.Registry, timers clock.Timers, log *slog.Logger) (transport, error) {
			tr, err := openBus(kind, reg, timers, log)
			bus.transport = tr
			return bus, err
		})
	p.opts.idSource = "sequence"

	runUntilStarted(t, p, started, nil)

	bus.mu.Lock()
	defer bus.mu.Unlock()
	if !strings.HasPrefix(bus.closed.ID, "seq-") {
		t.Errorf("an event built while the bus closes has the id %q, want seq-N of the run: "+
			"the sources were taken back before the bus closed", bus.closed.ID)
	}
}

// moving is a context that reads in its Start and so moves the replay clock
// before the start record is written — what State will do when it catches up on
// the journal (C-14).
type moving struct {
	witness
	to time.Time
}

func (m *moving) Start(ctx context.Context, deps runtime.Deps) error {
	if ec, ok := deps.Clock.(*replay.EventClock); ok {
		ec.Observe(m.to)
	}
	return m.witness.Start(ctx, deps)
}

// Н-1 of tech-lead#1 on T-060: the warning of a replay without a recording
// names where the clock started, which is the zero time, even when a context
// has moved the clock in its Start.
func TestTheWarningOfAReplayWithoutARecordingNamesTheStartOfTheClock(t *testing.T) {
	onLoopback(t)
	started := make(chan runtime.Deps, 1)
	ctx := &moving{witness: witness{name: "probe", rec: &recorder{}, started: started}, to: recordedAt}
	p := newProcess([]runtime.Context{ctx}, openBus)
	p.opts = replayOptions("")
	var logged strings.Builder
	var mu sync.Mutex
	p.log = slog.New(slog.NewJSONHandler(&lockedWriter{w: &logged, mu: &mu}, nil))

	runUntilStarted(t, p, started, nil)

	mu.Lock()
	defer mu.Unlock()
	for line := range strings.Lines(logged.String()) {
		var rec struct {
			Msg        string    `json:"msg"`
			ClockStart time.Time `json:"clock_start"`
		}
		if json.Unmarshal([]byte(line), &rec) != nil || rec.Msg != "replay without a recording" {
			continue
		}
		if !rec.ClockStart.IsZero() {
			t.Errorf("clock_start = %v, want the zero time the clock started at, not where a context moved it", rec.ClockStart)
		}
		return
	}
	t.Fatalf("no warning in the log: %q", logged.String())
}

// Н-2 of tech-lead#1 on T-060: the help of --recording says what the recording
// does in this build — it sets the start of the clock — and does not promise a
// journal that is read into the bus.
func TestTheHelpOfRecordingSaysWhatItDoes(t *testing.T) {
	var help bytes.Buffer
	if _, err := parseServe([]string{"-h"}, &help); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseServe(-h) = %v, want flag.ErrHelp", err)
	}
	text := help.String()
	i := strings.Index(text, "-recording")
	if i < 0 {
		t.Fatalf("the help does not name --recording: %q", text)
	}
	line := text[i:]
	if j := strings.Index(line, "\n  -"); j > 0 {
		line = line[:j]
	}
	if !strings.Contains(line, "clock") || strings.Contains(line, "journal read") {
		t.Errorf("the help of --recording reads %q: it must say the recording sets the clock and not "+
			"promise a journal read in replay", line)
	}
	// C-01 v1.9: the recording also reaches the recorded provider through
	// Deps.Recording (T-458, Н-2 of the acceptance of T-060 by tech-lead#1).
	if !strings.Contains(line, "providers/recorded") {
		t.Errorf("the help of --recording reads %q: it must say the recording feeds providers/recorded", line)
	}
}
