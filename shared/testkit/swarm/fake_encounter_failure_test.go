package swarm_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit/swarm"
)

// T-415: a publication of FakeEncounter the bus refused used to leave no trace.
// The action is remembered as answered before its package goes out, so the
// retry of the bus found it answered and returned nil — no dead letter, no log
// line, nothing in Err and so nothing on /health. Now it is an error in the
// log and in Err. The retry still answers nil: remembering late, as the
// narrator does, would re-run a half-published turn, and that is for EPIC-003
// to decide, not for a fix of the trace.
//
// A publication cut short by the stop is not a refusal and leaves no trace in
// Err (review #1 of T-415, Mi-1).

var errPublishRefused = errors.New("the broker is not taking this type")

// publishRefusingBus refuses to publish one type while refuse is on; everything
// else goes through to the real bus.
type publishRefusingBus struct {
	*membus.Bus
	mu  sync.Mutex
	typ string
}

func (b *publishRefusingBus) refuse(typ string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.typ = typ
}

func (b *publishRefusingBus) Publish(ctx context.Context, ev eventbus.Event) error {
	b.mu.Lock()
	refused := b.typ != "" && ev.Type == b.typ
	b.mu.Unlock()
	if refused {
		return errPublishRefused
	}
	return b.Bus.Publish(ctx, ev)
}

// refusedStub is the stub over the fixtures publishing through a bus that can
// refuse, with what it says written into said.
func refusedStub(t *testing.T, said *strings.Builder) (*swarm.FakeEncounter, *publishRefusingBus) {
	t.Helper()
	bus := &publishRefusingBus{Bus: newBus(t)}
	fixed := mechanics(t)
	enc, err := swarm.NewFakeEncounter(swarm.EncounterConfig{
		Bus: bus, WorldID: worldID, Rules: fixed.Rules(), Mechanics: fixed,
		Log: slog.New(slog.NewTextHandler(said, nil)),
	})
	if err != nil {
		t.Fatalf("encounter: %v", err)
	}
	world := mustFixtures(t)
	if err := enc.Seed(world); err != nil {
		t.Fatalf("seed: %v", err)
	}
	answeredBy(t, bus.Bus, world)
	return enc, bus
}

// answerCollecting is answer with the facts handed to the stub under ctx and
// the errors the stub answers them with collected instead of failing the test.
func answerCollecting(ctx context.Context, t *testing.T, enc *swarm.FakeEncounter, bus *membus.Bus) []error {
	t.Helper()
	held, _ := rigs.Load(bus)
	r := held.(*rig)
	var errs []error
	for {
		events := eventsOf(t, bus, eventbus.TopicSystemEvents)
		if r.read >= len(events) {
			return errs
		}
		ev := events[r.read]
		r.read++
		switch ev.Source {
		case contracts.SourceTestkitSwarm:
			if err := r.state.Apply(t.Context(), ev); err != nil {
				t.Fatalf("State on %s: %v", ev.Type, err)
			}
		case contracts.SourceTestkitState:
			if err := enc.Observe(ctx, ev); err != nil {
				errs = append(errs, err)
			}
		}
	}
}

// traced says whether the refusal of typ left its trace: an error naming the
// type in the log, and the refusal in Err.
func traced(t *testing.T, enc *swarm.FakeEncounter, said *strings.Builder, typ string) {
	t.Helper()
	if !errors.Is(enc.Err(), errPublishRefused) {
		t.Errorf("Err() = %v, want the refusal of %s", enc.Err(), typ)
	}
	log := said.String()
	if !strings.Contains(log, "level=ERROR") || !strings.Contains(log, "publication refused by the bus") ||
		!strings.Contains(log, "type="+typ) {
		t.Errorf("the refused %s left no error in the log: %q", typ, log)
	}
}

// untraced says that the stop left nothing behind: no failure in Err, no error
// in the log.
func untraced(t *testing.T, enc *swarm.FakeEncounter, said *strings.Builder) {
	t.Helper()
	if err := enc.Err(); err != nil {
		t.Errorf("Err() = %v after a publication cut short by the stop, want nil: a clean stop is not a failure", err)
	}
	if strings.Contains(said.String(), "level=ERROR") {
		t.Errorf("the stop left an error in the log: %q", said.String())
	}
}

func TestARefusedDecisionLeavesATrace(t *testing.T) {
	var said strings.Builder
	enc, bus := refusedStub(t, &said)
	openFight(t, enc, bus.Bus)
	if err := enc.Err(); err != nil {
		t.Fatalf("Err() = %v before anything was refused", err)
	}

	bus.refuse(swarm.TypeCombatDecided)
	swing := attack(playerA, nameA, wolfID)
	if err := enc.Act(t.Context(), swing); !errors.Is(err, errPublishRefused) {
		t.Fatalf("an attack whose decision was refused answered %v, want the refusal", err)
	}
	traced(t, enc, &said, swarm.TypeCombatDecided)

	// The retry of the bus: this is the silence the trace is for.
	bus.refuse("")
	if err := enc.Act(t.Context(), swing); err != nil {
		t.Fatalf("the retry answered %v", err)
	}
	if !errors.Is(enc.Err(), errPublishRefused) {
		t.Errorf("the trace went away with the retry: Err() = %v", enc.Err())
	}
}

// The announcement of a fight goes out on the fact of State (announce), which
// is the other way a publication leaves the stub.
func TestARefusedAnnouncementLeavesATrace(t *testing.T) {
	var said strings.Builder
	enc, bus := refusedStub(t, &said)
	bus.refuse(swarm.TypeEncounterStarted)

	act(t, enc, entered(playerA, nameA, regionID, regName))
	errs := answerCollecting(t.Context(), t, enc, bus.Bus)
	if len(errs) != 1 || !errors.Is(errs[0], errPublishRefused) {
		t.Fatalf("the facts were answered with %v, want the one refusal of the creation of the encounter", errs)
	}
	traced(t, enc, &said, swarm.TypeEncounterStarted)
}

// The end of a fight is announced on the fact of the package that closes it —
// the third way out of the stub (review #1 of T-415, N-1).
func TestARefusedEndLeavesATrace(t *testing.T) {
	var said strings.Builder
	enc, bus := refusedStub(t, &said)
	openFight(t, enc, bus.Bus)
	bus.refuse(swarm.TypeEncounterEnded)

	var refusal error
	for i := 0; i < 20 && refusal == nil; i++ {
		act(t, enc, attack(playerA, nameA, wolfID))
		for _, err := range answerCollecting(t.Context(), t, enc, bus.Bus) {
			if !errors.Is(err, errPublishRefused) {
				t.Fatalf("a fact answered %v", err)
			}
			refusal = err
		}
	}
	if refusal == nil {
		t.Fatal("twenty exchanges and the end of the fight was never announced")
	}
	traced(t, enc, &said, swarm.TypeEncounterEnded)
}

// Mi-1 of review #1 of T-415: the stop cancels the context a handler
// publishes under, and the bus answers the cancellation. That is a clean stop,
// not a refusal — on the action and on the fact alike.
func TestAPublicationCutShortByTheStopLeavesNoTrace(t *testing.T) {
	stopped, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("an action", func(t *testing.T) {
		var said strings.Builder
		enc, bus := refusedStub(t, &said)
		openFight(t, enc, bus.Bus)

		if err := enc.Act(stopped, attack(playerA, nameA, wolfID)); !errors.Is(err, context.Canceled) {
			t.Fatalf("an attack under a cancelled context answered %v, want the cancellation", err)
		}
		untraced(t, enc, &said)
		if !strings.Contains(said.String(), "publication interrupted by the stop") {
			t.Errorf("the interrupted publication is not in the log at all: %q", said.String())
		}
	})
	t.Run("a fact", func(t *testing.T) {
		var said strings.Builder
		enc, bus := refusedStub(t, &said)
		act(t, enc, entered(playerA, nameA, regionID, regName))

		errs := answerCollecting(stopped, t, enc, bus.Bus)
		if len(errs) != 1 || !errors.Is(errs[0], context.Canceled) {
			t.Fatalf("the creation handed over under a cancelled context answered %v, want the cancellation", errs)
		}
		untraced(t, enc, &said)
	})
}
