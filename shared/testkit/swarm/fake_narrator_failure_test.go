package swarm_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit/membus"
	"multiverse-core.io/shared/testkit/swarm"
)

// This file is iteration 2 of T-220, the two Minor findings of review #1.
//
// Mi-1: a narrative that could not be published used to disappear without a
// trace — the event had been remembered as told before the publication, so the
// retry of the bus found it told and answered nil. No narrative, no dead
// letter, no log line, no Err(). The event is now remembered only once its
// narrative is out, and the probe of the reviewer (P1) is what the first three
// tests are.
//
// Mi-2: a flight that failed with nothing striking was told as a flight caught
// by a strike — over a wolf lying dead, "бьёт вдогонку: 0".

// errBrokerDown is what flakyBus answers a refused publication with.
var errBrokerDown = errors.New("the broker is not taking narrative.output")

// flakyBus refuses to publish narrative.output: a number of times, or for
// ever when failures is negative. Everything else goes through, so the
// narrator reads and the bus retries exactly as they would.
type flakyBus struct {
	*membus.Bus
	mu       sync.Mutex
	failures int
	attempts int
}

func (b *flakyBus) Publish(ctx context.Context, ev eventbus.Event) error {
	if ev.Type == swarm.TypeNarrativeOutput {
		b.mu.Lock()
		b.attempts++
		fail := b.failures != 0
		if b.failures > 0 {
			b.failures--
		}
		b.mu.Unlock()
		if fail {
			return errBrokerDown
		}
	}
	return b.Bus.Publish(ctx, ev)
}

func (b *flakyBus) tried() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.attempts
}

func (b *flakyBus) refuse(times int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = times
}

// startedOver is a narrator reading and publishing through the flaky bus.
func startedOver(t *testing.T, bus *flakyBus) *swarm.FakeNarrator {
	t.Helper()
	n, err := swarm.NewFakeNarrator(bus, worldID)
	if err != nil {
		t.Fatalf("narrator: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = n.Wait() })
	if err := n.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	return n
}

// TestAPublicationThatFailedOnceIsToldAfterTheRetry: one failure of the broker
// costs one retry of the bus, not the narrative.
func TestAPublicationThatFailedOnceIsToldAfterTheRetry(t *testing.T) {
	bus := &flakyBus{Bus: newBus(t), failures: 1}
	startedOver(t, bus)

	if err := bus.Bus.Publish(t.Context(), look(playerA, nameA)); err != nil {
		t.Fatalf("publish the look: %v", err)
	}
	waitFor(t, "the narrative of the look after the retry", func() bool {
		return len(narratives(t, bus.Bus)) > 0
	})

	if got := narratives(t, bus.Bus); len(got) != 1 {
		t.Errorf("%d narratives for one look told after one retry", len(got))
	}
	if tried := bus.tried(); tried != 2 {
		t.Errorf("%d publications tried, want the refused one and its retry", tried)
	}
	if letters, _ := bus.DeadLetters(); len(letters) != 0 {
		t.Errorf("%d dead letters; a retry that succeeded parks nothing", len(letters))
	}
}

// TestAPublicationThatAlwaysFailsEndsInDeadLetters: a failure that does not
// pass — a broker that is gone, a narrative the schema refuses — ends where the
// bus puts everything it could not handle, with the event that caused it.
func TestAPublicationThatAlwaysFailsEndsInDeadLetters(t *testing.T) {
	bus := &flakyBus{Bus: newBus(t), failures: -1}
	startedOver(t, bus)

	action := look(playerA, nameA)
	if err := bus.Bus.Publish(t.Context(), action); err != nil {
		t.Fatalf("publish the look: %v", err)
	}
	var letters []eventbus.DeadLetter
	waitFor(t, "the look to be parked in dead_letters", func() bool {
		letters, _ = bus.DeadLetters()
		return len(letters) > 0
	})

	if len(letters) != 1 || letters[0].Original.ID != action.ID {
		t.Fatalf("dead letters %+v, want the look %s", letters, action.ID)
	}
	if !strings.Contains(letters[0].Error, errBrokerDown.Error()) {
		t.Errorf("the dead letter says %q, want the refusal of the broker", letters[0].Error)
	}
	// Every call of the handler tried to publish: the window never cut a retry
	// short, which is exactly what it used to do.
	if tried := bus.tried(); tried != letters[0].Attempts {
		t.Errorf("%d publications tried over %d attempts of the bus", tried, letters[0].Attempts)
	}
	if got := narratives(t, bus.Bus); len(got) != 0 {
		t.Errorf("%d narratives from a broker that took none", len(got))
	}
}

// TestATurnWhosePublicationFailedIsToldFromAllItsDecisions: the turn is told
// from the whole exchange on the retry — the decisions gathered before the
// failure are not dropped with it — and the failure is written in the log,
// because until the bus parks the event nothing else says it.
func TestATurnWhosePublicationFailedIsToldFromAllItsDecisions(t *testing.T) {
	bus := &flakyBus{Bus: newBus(t)}
	var said strings.Builder
	n, err := swarm.NewFakeNarrator(bus, worldID)
	if err != nil {
		t.Fatalf("narrator: %v", err)
	}
	n.WithLog(slog.New(slog.NewTextHandler(&said, nil)))

	cause := attack(playerA, nameA, wolfID)
	blow := decided(cause, strike("attack", playerA, wolfID, true, 4, 6))
	bite := decided(cause, strike("npc_attack", wolfID, playerA, true, 2, 8))
	narrate(t, n, blow)

	bus.refuse(1)
	if err := n.Narrate(t.Context(), bite); !errors.Is(err, errBrokerDown) {
		t.Fatalf("a turn the broker refused answered %v, want the refusal for the bus to retry", err)
	}
	if !strings.Contains(said.String(), "level=ERROR") || !strings.Contains(said.String(), "narrative not published") {
		t.Errorf("the failed publication left no error in the log: %q", said.String())
	}

	narrate(t, n, bite) // the retry of the bus
	narrate(t, n, bite) // and a redelivery after it went out
	turn := onlyNarrative(t, bus.Bus)
	if on := basedOn(t, turn); !slices.Equal(on, []string{blow.ID, bite.ID}) {
		t.Errorf("the turn told on the retry is based on %v, want the whole exchange %v",
			on, []string{blow.ID, bite.ID})
	}
}

// TestADeathWhosePublicationFailedIsToldOnTheRetry is the same promise for the
// narrative Observe writes.
func TestADeathWhosePublicationFailedIsToldOnTheRetry(t *testing.T) {
	bus := &flakyBus{Bus: newBus(t), failures: 1}
	n, err := swarm.NewFakeNarrator(bus, worldID)
	if err != nil {
		t.Fatalf("narrator: %v", err)
	}
	fact := playerDied(playerA)

	if err := n.Observe(t.Context(), fact); !errors.Is(err, errBrokerDown) {
		t.Fatalf("a death the broker refused answered %v, want the refusal", err)
	}
	learnFact(t, n, fact)
	learnFact(t, n, fact)

	if got := ofKind(narratives(t, bus.Bus), swarm.KindDeath); len(got) != 1 {
		t.Errorf("%d deaths told for one death refused once and delivered three times", len(got))
	}
}

// TestAFlightNothingStruckAtClaimsNoBlow is Mi-2: a flight that failed and was
// answered by nothing — the rules call for no strike, or the wolf lies dead,
// killed by somebody else — is told without a blow. The caught flight, which
// was struck at, still says so.
func TestAFlightNothingStruckAtClaimsNoBlow(t *testing.T) {
	cases := []struct {
		name      string
		decisions []map[string]any
		struck    bool
	}{
		{"nobody left standing", []map[string]any{flight(false, true, 0)}, false},
		{"the rules call for no strike", []map[string]any{flight(false, false, 1)}, false},
		{"caught and struck", []map[string]any{flight(false, true, 1), freeAttack(3, 7)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, bus := running(t)
			knows(t, n, playerA, nameA, 10, 10)
			narrate(t, n, encounterStarted(entered(playerA, nameA, regionID, regName), actor(playerA, nameA)))
			cause := flee(playerA, nameA)
			for _, payload := range tc.decisions {
				narrate(t, n, decided(cause, payload))
			}

			turns := ofKind(narratives(t, bus), swarm.KindTurn)
			if len(turns) != 1 {
				t.Fatalf("%d turns told for one flight", len(turns))
			}
			text, _ := turns[0].Path().GetString("text")
			claims := strings.Contains(text, "бьёт") || strings.Contains(text, "вдогонку")
			if claims != tc.struck {
				t.Errorf("struck=%v, and the text claims a blow: %v — %q", tc.struck, claims, text)
			}
		})
	}
}
