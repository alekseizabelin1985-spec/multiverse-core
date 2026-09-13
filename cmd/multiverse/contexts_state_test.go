package main

import (
	"context"
	"testing"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
)

// The context state of the binary is internal/state (T-055): built by
// runtime.New as serve builds it, run by process.run on the bus of
// --bus=memory, it answers a proposal of its world with a fact of core/state.
func TestTheProcessRunsTheStateOfEPIC002(t *testing.T) {
	onLoopback(t)
	t.Setenv(env.StateWorlds.Name(), "dark-forest-world")
	built, err := runtime.New([]string{stateContext})
	if err != nil {
		t.Fatalf("New(state): %v", err)
	}
	if _, ok := built[0].(*state.Context); !ok {
		t.Fatalf("the context state is %T, want *state.Context of internal/state", built[0])
	}
	started := make(chan runtime.Deps, 1)
	probe := &witness{name: "probe", rec: &recorder{}, started: started}

	runUntilStarted(t, newProcess(append(built, probe), openBus), started, func(deps runtime.Deps) {
		proposal := eventbus.NewRoot(state.TypeCreateProposed, contracts.SourceGateway, "dark-forest-world", nil,
			eventbus.ActorCI, map[string]any{
				"proposal_id": "prop-through-the-process",
				"entity":      map[string]any{"entity": map[string]any{"id": "player-A", "type": entity.TypePlayer}},
				"attributes":  map[string]any{"hp": 10},
				"cause":       "create",
			})
		if err := deps.Bus.Publish(context.Background(), proposal); err != nil {
			t.Fatalf("publish: %v", err)
		}
		var fact *eventbus.Event
		deadline := time.Duration(0)
		for fact == nil {
			end, err := deps.Journal.End(context.Background(), eventbus.TopicSystemEvents)
			if err != nil {
				t.Fatalf("End: %v", err)
			}
			if _, err := deps.Journal.ReadRange(context.Background(), eventbus.TopicSystemEvents, 0, end,
				func(_ context.Context, ev eventbus.Event) error {
					if ev.Type == state.TypeCreated {
						fact = &ev
					}
					return nil
				}); err != nil {
				t.Fatalf("ReadRange: %v", err)
			}
			if fact != nil {
				break
			}
			if deadline += 5 * time.Millisecond; deadline > 30*time.Second {
				t.Fatal("no entity.created within 30s of the proposal")
			}
			<-clock.RealTimers{}.After(5 * time.Millisecond).C()
		}
		if fact.Source != contracts.SourceState || fact.Meta.CausationID != proposal.ID {
			t.Errorf("entity.created from %q caused by %q, want core/state answering %s",
				fact.Source, fact.Meta.CausationID, proposal.ID)
		}
		if id, _ := fact.Path().GetString("proposal_id"); id != "prop-through-the-process" {
			t.Errorf("proposal_id %q, want the proposal's", id)
		}
	})
}
