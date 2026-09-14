package main

import (
	"context"
	"log/slog"
	"testing"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	tkstate "multiverse-core.io/shared/testkit/state"
)

// unclosed is the transport of a process that outlives it: two runs of the
// process in one test read and write one journal, as two starts of core read
// one Redpanda.
type unclosed struct{ transport }

func (unclosed) Close() error { return nil }

// The object store and the recovery of State come into the process together
// (T-059): a process over a store initialises its world, answers a proposal and
// stops; the next process over the same store and journal rebuilds the world
// from the shutdown snapshot — identical, the same state_hash — and serves it.
func TestTheProcessRecoversItsWorldFromTheStore(t *testing.T) {
	onLoopback(t)
	clearVar(t, env.SwarmFake.Name())
	clearVar(t, env.MinIOAccessKey.Name())
	clearVar(t, env.MinIOSecretKey.Name())
	const world = "dark-forest-world"
	t.Setenv(env.StateWorlds.Name(), world)
	objects := objstore.NewMemory()
	if err := state.EnsureWorldBuckets(context.Background(), objects, world); err != nil {
		t.Fatal(err)
	}
	fromEnv := stateObjects
	stateObjects = func() (objstore.Client, error) { return objects, nil }
	t.Cleanup(func() { stateObjects = fromEnv })
	bus, err := newMemoryBus(contracts.Default(), false, clock.RealTimers{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	open := func(string, *contracts.Registry, clock.Timers, *slog.Logger) (transport, error) {
		return unclosed{bus}, nil
	}
	fixtures, err := tkstate.LoadFixtures(fixturesOfTheTree)
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}

	run := func(inspect func(c *state.Context, deps runtime.Deps)) {
		t.Helper()
		built, err := runtime.New([]string{stateContext})
		if err != nil {
			t.Fatal(err)
		}
		c, ok := built[0].(*state.Context)
		if !ok {
			t.Fatalf("the context state is %T", built[0])
		}
		started := make(chan runtime.Deps, 1)
		probe := &witness{name: "probe", rec: &recorder{}, started: started}
		runUntilStarted(t, newProcess(append(built, probe), open), started, func(deps runtime.Deps) { inspect(c, deps) })
	}
	section := func(c *state.Context) map[string]any {
		worlds, _ := c.Health().Details["worlds"].(map[string]any)
		s, _ := worlds[world].(map[string]any)
		return s
	}

	var before string
	run(func(c *state.Context, deps runtime.Deps) {
		if rec, _ := c.Recovery(world); !rec.Uninitialized {
			t.Errorf("the first start %+v, want a world waiting for its init", rec)
		}
		for _, e := range fixtures {
			created := answerTo(t, deps, eventbus.NewRoot(state.TypeCreateProposed, contracts.SourceMvctl, world, nil,
				eventbus.ActorSystem, map[string]any{
					"proposal_id": "bootstrap:" + world + ":" + e.Type + "/" + e.ID,
					"entity":      map[string]any{"entity": map[string]any{"id": e.ID, "type": e.Type}, "name": e.Name},
					"attributes":  e.Attributes,
					"cause":       state.CauseInit,
				}))
			if created.Type != state.TypeCreated {
				t.Fatalf("the fixture %s answered %s", e.ID, created.Type)
			}
		}
		moved := answerTo(t, deps, changeProposal(t, world, contracts.SourceGateway, nil, "move-into-the-forest", "move",
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: "dark-forest-01"}))
		if moved.Type != state.TypeUpdated {
			t.Fatalf("the move answered %s", moved.Type)
		}
		before, _ = section(c)["state_hash"].(string)
		if c.Health().Status != runtime.StatusOK || section(c)["rules_version"] == "" {
			t.Errorf("health %+v, want ok with the rules version of the process", section(c))
		}
	})
	if _, err := state.NewObjectStore(objects).ReadLatest(context.Background(), world); err != nil {
		t.Fatalf("latest.json after the stop: %v", err)
	}

	run(func(c *state.Context, deps runtime.Deps) {
		rec, _ := c.Recovery(world)
		if identical, known := rec.Identical(); !identical || !known || rec.StateHashAfter != before {
			t.Errorf("recovery %+v, want the world of the first process (%s) identical", rec, before)
		}
		if c.Health().Status != runtime.StatusOK || section(c)["state_hash"] != before {
			t.Errorf("health %+v, want ok with the state_hash %s", section(c), before)
		}
		again := answerTo(t, deps, changeProposal(t, world, contracts.SourceGateway, nil, "move-again", "move",
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: "outside:" + world}))
		if reason, _ := again.Path().GetString("reason"); again.Type != state.TypeRejected || reason != string(state.ReasonVersionConflict) {
			t.Errorf("a move at version 1 answered %s %s, want version_conflict: the restarted world is at v2", again.Type, reason)
		}
	})
	tkstate.OneStateOverTheWorld(t, eventsOn(t, bus.(*membus.Bus), eventbus.TopicSystemEvents))
}
