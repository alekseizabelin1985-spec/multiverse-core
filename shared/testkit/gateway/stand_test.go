package gateway_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway"
	tkmech "multiverse-core.io/shared/testkit/mechanics"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

// This file is the one place in the tree where the harness meets the only
// thing that resolves a fight in Phase 1 — testkit/swarm.FakeEncounter of
// EPIC-003 (contracts.md C-05, tasks.md T-219) — with State between them and
// nothing faked in the middle.
//
// It exists because both halves were green apart and broke together: the
// harness was checked against a stand-in encounter of its own that never
// missed and never ended a fight, the encounter was checked by publishing
// player actions straight onto the bus, and the review of 2026-09-11 was the
// first thing in the project to put the two on one bus — 25 runs out of 60
// hung. A set that never runs them together cannot see that class at all.

// standFights is how many different fights the stand runs. Different, because
// the table of FixedMechanics answers by the identifier of the causing event,
// so the prefix of the identifier sequence is what moves the dice: one fight
// proves the path exists, sixteen sample the outcomes it has to survive — a
// miss on both sides, a wolf that falls early, a wolf that outlasts the blows.
const standFights = 16

// standTimeout is short on purpose. Nothing here should ever reach it: what an
// encounter does not answer, it now says it does not answer (ErrFightOver),
// and what it answers, it answers in microseconds on an in-process bus. A
// timeout is therefore a failure and not a slow machine, and it costs seconds
// rather than minutes when the path breaks again.
const standTimeout = 2 * time.Second

func TestTheHarnessAndTheEncounterOfTheSwarmOnOneBus(t *testing.T) {
	for i := range standFights {
		t.Run(fmt.Sprintf("fight-%02d", i), func(t *testing.T) {
			standFight(t, fmt.Sprintf("st%02d", i), false)
		})
	}
}

// TestTheStandWhenTheHarnessHearsOfTheFightLast is the same stand with
// world_events held back from the harness (lateWorld): the order C-01 allows
// and a busy machine produces one run in many, made the order of every run.
//
// It is how the flake of T-433 was reproduced, and it stays because it is the
// only place the two ends of that order are checked on the real encounter —
// that the stand does not ask the harness for a start it has not been handed
// yet, and that a flight into a fight the harness has not heard open, and has
// already heard close, does not wait out the timeout.
func TestTheStandWhenTheHarnessHearsOfTheFightLast(t *testing.T) {
	for i := range standFights {
		t.Run(fmt.Sprintf("fight-%02d", i), func(t *testing.T) {
			standFight(t, fmt.Sprintf("lt%02d", i), true)
		})
	}
}

// standFight runs the whole skirmish once against the real encounter stub and
// checks what only this stand can check: that the world came to rest where the
// harness thinks it did. late holds world_events back from the harness.
func standFight(t *testing.T, prefix string, late bool) {
	t.Helper()
	bus := busWith(t, prefix)
	fixtures := loadFixtures(t)
	fake := stateOver(t, bus)
	seedWithoutPlayers(t, fake, fixtures)
	enc := encounterOver(t, bus, fixtures)

	var seen eventbus.Bus = bus
	release := func() {}
	if late {
		held := &lateWorld{Bus: bus, gate: make(chan struct{})}
		seen, release = held, held.release
	}
	h, err := gateway.NewHarness(seen, fixtures)
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	h.WithTimeout(standTimeout).WithLog(nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = fake.Wait(); _ = enc.Wait(); _ = h.Wait() })
	for name, start := range map[string]func(context.Context) error{
		"state": fake.Start, "encounter": enc.Start, "harness": h.Start,
	} {
		if err := start(ctx); err != nil {
			t.Fatalf("start %s: %v", name, err)
		}
	}

	script, err := gateway.Script(gateway.ScenarioSkirmish)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	taken, err := h.Run(ctx, script)
	release()
	if err != nil {
		t.Fatalf("the skirmish against the encounter of the swarm: %v", err)
	}

	// A fight happened, and the harness knew it was in one. Without this the
	// rest of the run would pass just as well over a bus where the encounter
	// never opened.
	if blows := turns(taken, gateway.ActionAttack); blows == 0 {
		t.Error("the run struck no blow: nothing here was a fight")
	}
	if !heardOfTheFight(h, playerA) {
		t.Errorf("the harness never heard that %s was in an encounter", playerA)
	}
	if started := ofType(read(t, bus, eventbus.TopicWorldEvents),
		swarm.TypeEncounterStarted); len(started) != 1 {
		t.Errorf("%d encounters opened around one character, want one", len(started))
	}

	// What the stand is for: the read-model of the harness holds what State
	// holds, for everything the fight touched. A blow the harness returned from
	// too early leaves exactly this behind — a version the world has left —
	// and the next proposal of a script pinned to it is refused (ADR-013 p. 1).
	for _, id := range []string{playerA, npcID} {
		who, ok := fake.Get(id)
		if !ok {
			t.Fatalf("%s is not in the world State kept", id)
		}
		if version, known := h.Version(id); !known || version != who.Version {
			t.Errorf("the harness thinks %s is at version %d, State says %d",
				id, version, who.Version)
		}
		if status, _ := who.Status(); entity.IsTerminalStatus(status) {
			if held, known := h.Status(id); !known || held != status {
				t.Errorf("State says %s is %q, the harness holds %q", id, status, held)
			}
		}
	}
	facts := read(t, bus, eventbus.TopicSystemEvents)
	standRefusals(t, facts)
	// A run that ended without the character having acted at all would satisfy
	// everything above; this is what says the script went through.
	for _, action := range []string{gateway.ActionCreate, gateway.ActionEnter} {
		if turns(taken, action) != 1 {
			t.Errorf("the run did not %s", action)
		}
	}
	for _, ev := range read(t, bus, eventbus.TopicPlayerEvents) {
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("%s of the harness is not valid: %v", ev.Type, err)
		}
	}
}

// standRefusals is what the stand makes of every proposal State turned down.
//
// One of them is not a defect. A package refused for a version conflict says
// that somebody moved an entity under it, and this stand is where that happens:
// the harness moves the character it plays while the encounter answers a blow
// from a view it fills over a subscription of its own, and C-01 orders no two
// topics against each other. The publisher owes that refusal a retry, and a
// retry that was applied leaves the world exactly where a package that never
// lost the race would have left it — the same identifier, now with a fact
// behind it (§4.5: State applies an identifier once).
//
// So the conflict is forgiven only on the evidence that the package came back:
// a conflict without a fact under its identifier is a publisher that gave up,
// which is the defect the retry exists to prevent, and every other reason is a
// defect of whoever proposed the change.
func standRefusals(t *testing.T, facts []eventbus.Event) {
	t.Helper()
	applied := make(map[string]struct{}, len(facts))
	for _, ev := range facts {
		if ev.Type != state.TypeCreated && ev.Type != state.TypeUpdated {
			continue
		}
		if id, ok := ev.Path().GetString("proposal_id"); ok {
			applied[id] = struct{}{}
		}
	}
	for _, refused := range ofType(facts, state.TypeRejected) {
		reason, _ := refused.Path().GetString("reason")
		proposal, _ := refused.Path().GetString("proposal_id")
		if reason != gateway.ReasonVersionConflict {
			t.Errorf("State refused %s: %s", proposal, reason)
			continue
		}
		if _, ok := applied[proposal]; !ok {
			t.Errorf("%s lost the version race and was never applied: whoever proposed it "+
				"gave up instead of offering it again", proposal)
		}
	}
}

// heardOfTheFight waits, no longer than standTimeout, until the harness has
// heard that the character was in an encounter.
//
// The wait is the contract and not patience for a slow machine. Run returns on
// the answers of system_events, encounter.started travels on world_events, and
// C-01 orders no two topics against each other: the start can still be on its
// way to the harness when the last step of the script is answered — false from
// Fight is "not heard of", never "there is none". The encounter has published
// it by then (the journal is read for it right below); only the reading is
// late. A start that never comes still fails, at the same deadline as a step.
func heardOfTheFight(h *gateway.Harness, playerID string) bool {
	deadline := testkit.After(standTimeout)
	for {
		if _, _, known := h.Fight(playerID); known {
			return true
		}
		select {
		case <-deadline:
			_, _, known := h.Fight(playerID)
			return known
		case <-clock.RealTimers{}.After(time.Millisecond).C():
		}
	}
}

// lateWorld is the bus as a harness sees it when world_events reaches it last:
// nothing of that topic is handed to the harness until the first flight it
// publishes or the end of the run, whichever comes first.
//
// The flight is where the gate opens because it is the latest the start can
// arrive with the harness still in a wait. A fight that ended with the wolf is
// heard to end from the fact on system_events, but the fact names no character,
// so a harness that has not heard the start lets the flight go out into a fight
// that is over and waits for an answer nobody gives. The start arriving during
// that wait is what has to end it. Held until the end of the run, it would end
// nothing: the run would already have failed at the deadline.
//
// Only the harness is handed this view; State and the encounter read the bus
// itself.
type lateWorld struct {
	*membus.Bus
	gate chan struct{}
	once sync.Once
}

func (b *lateWorld) release() { b.once.Do(func() { close(b.gate) }) }

func (b *lateWorld) Publish(ctx context.Context, ev eventbus.Event) error {
	err := b.Bus.Publish(ctx, ev)
	if ev.Type == gateway.TypeFleeAttempted {
		b.release()
	}
	return err
}

func (b *lateWorld) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	if topic != eventbus.TopicWorldEvents {
		return b.Bus.Subscribe(ctx, topic, group, h)
	}
	return b.Bus.Subscribe(ctx, topic, group, func(ctx context.Context, ev eventbus.Event) error {
		select {
		case <-b.gate:
		case <-ctx.Done():
			return nil
		}
		return h(ctx, ev)
	})
}

// busWith is a bus whose identifiers start from a prefix of their own, which
// is what makes one fight of the stand differ from another.
func busWith(t *testing.T, prefix string) *membus.Bus {
	t.Helper()
	testkit.Deterministic(t, prefix)
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{
		Registry: contracts.Default(), Topics: topics, Backoff: []time.Duration{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func stateOver(t *testing.T, bus *membus.Bus) *state.FakeState {
	t.Helper()
	fake, err := state.New(state.Config{
		Bus: bus, Store: objstore.NewMemoryWithClock(clock.NewManual(testkit.Epoch)),
		WorldID: worldID, RulesVersion: "0.1",
	})
	if err != nil {
		t.Fatalf("fake state: %v", err)
	}
	return fake
}

// encounterOver is the encounter stub of EPIC-003 over the rules of the
// fixture world. The rules are the real ones — rules/dark-forest.yaml through
// internal/mechanics — and only the one decision EPIC-002 still owes comes
// from the table of FixedMechanics (T-017, T-053).
func encounterOver(t *testing.T, bus *membus.Bus, fixtures []*entity.Entity) *swarm.FakeEncounter {
	t.Helper()
	fixed, err := tkmech.Load(filepath.Join("..", "..", "..", "rules", "dark-forest.yaml"))
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	enc, err := swarm.NewFakeEncounter(swarm.EncounterConfig{
		Bus: bus, WorldID: worldID, Rules: fixed.Rules(), Mechanics: fixed,
	})
	if err != nil {
		t.Fatalf("encounter: %v", err)
	}
	if err := enc.Seed(fixtures); err != nil {
		t.Fatalf("seed the encounter: %v", err)
	}
	return enc
}
