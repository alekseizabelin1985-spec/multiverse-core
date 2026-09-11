package swarm_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

// This file puts the narrator where the gateway will meet it: on one bus with
// the harness of the gateway, the double of State and the fight of the swarm,
// with nothing faked in between (the stand of T-219, shared/testkit/gateway,
// plus the one stub it does not raise).
//
// What it says is counted from what the harness did, never from the table of
// the narrator: a turn per blow and per flight the harness took, an entry per
// step into the region and per look, a world event per encounter the world
// opened, a death when State says the character is dead. A narrator that told
// a blow twice, or never, or to somebody else, is caught here even where every
// unit test of it is green — which is the lesson of the stand itself.

// narratedFights is how many different fights are told. Different, because the
// table of FixedMechanics answers by the identifier of the causing event: a
// fight that ends on the first blow, one that lasts, one the character loses
// and one they run from are all among them.
const narratedFights = 16

func TestTheNarratorTellsTheFightsOfTheStand(t *testing.T) {
	for i := range narratedFights {
		t.Run(fmt.Sprintf("fight-%02d", i), func(t *testing.T) {
			narratedFight(t, fmt.Sprintf("nr%02d", i))
		})
	}
}

func narratedFight(t *testing.T, prefix string) {
	t.Helper()
	testkit.Deterministic(t, prefix)
	bus := plainBus(t)
	fixtures := mustFixtures(t)

	fake, err := state.New(state.Config{
		Bus: bus, Store: objstore.NewMemoryWithClock(clock.NewManual(testkit.Epoch)),
		WorldID: worldID, RulesVersion: "0.1",
	})
	if err != nil {
		t.Fatalf("fake state: %v", err)
	}
	// State starts without the characters: creating one is the first step of
	// the script, and a world that already held it would refuse that step.
	world := make([]*entity.Entity, 0, len(fixtures))
	for _, e := range fixtures {
		if e.Type != entity.TypePlayer {
			world = append(world, e)
		}
	}
	if err := fake.Seed(world); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	enc := stubOver(t, bus)
	if err := enc.Seed(fixtures); err != nil {
		t.Fatalf("seed the encounter: %v", err)
	}
	n, err := swarm.NewFakeNarrator(bus, worldID)
	if err != nil {
		t.Fatalf("narrator: %v", err)
	}
	h, err := gateway.NewHarness(bus, fixtures)
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	h.WithTimeout(2 * time.Second).WithLog(nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = fake.Wait(); _ = enc.Wait(); _ = n.Wait(); _ = h.Wait() })
	for name, start := range map[string]func(context.Context) error{
		"state": fake.Start, "encounter": enc.Start, "narrator": n.Start, "harness": h.Start,
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
	if err != nil {
		t.Fatalf("the skirmish: %v", err)
	}

	want := map[string]int{}
	for _, step := range taken {
		switch step.Action {
		case gateway.ActionEnter, gateway.ActionLook:
			want[swarm.KindEntry]++
		case gateway.ActionAttack, gateway.ActionFlee:
			want[swarm.KindTurn]++
		}
	}
	if want[swarm.KindTurn] == 0 {
		t.Fatal("the run struck no blow and fled from nothing: nothing here was a fight")
	}
	want[swarm.KindWorldEvent] = len(ofType(eventsOf(t, bus, eventbus.TopicWorldEvents),
		swarm.TypeEncounterStarted))
	if who, ok := fake.Get(playerA); ok {
		if status, _ := who.Status(); status == entity.StatusDead {
			want[swarm.KindDeath] = 1
		}
	}
	total := 0
	for _, count := range want {
		total += count
	}

	var told []eventbus.Event
	waitFor(t, "the narratives of the fight", func() bool {
		told = narratives(t, bus)
		return len(told) >= total
	})
	have := map[string]int{}
	for _, ev := range told {
		kind, _ := ev.Path().GetString("kind")
		have[kind]++
		if to := recipientsOf(t, ev); len(to) != 1 || to[0] != playerA {
			t.Errorf("a %s narrative is addressed to %v, the scope is %s", kind, to, playerA)
		}
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("a %s narrative is not valid: %v", kind, err)
		}
	}
	for _, kind := range []string{swarm.KindEntry, swarm.KindWorldEvent, swarm.KindTurn,
		swarm.KindDeath, swarm.KindRound} {
		if have[kind] != want[kind] {
			t.Errorf("%d %s narratives, the run is worth %d (steps taken: %d)",
				have[kind], kind, want[kind], len(taken))
		}
	}
}
