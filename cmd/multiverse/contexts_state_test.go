package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
	tkstate "multiverse-core.io/shared/testkit/state"
)

// The rule book and the fixture world of the tree, resolved when the package is
// initialised: go test starts in the directory of the package, and a test that
// changes its working directory later still finds them.
var (
	ruleBookOfTheTree, _ = filepath.Abs(filepath.Join("..", "..", "rules", "dark-forest.yaml"))
	fixturesOfTheTree, _ = filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures"))
)

// withTheRuleBook points MV_RULES_PATH at the rule book of the tree. A process
// under test runs in the directory of this package, where the default
// rules/dark-forest.yaml is not, and state does not start without its laws
// (T-471).
func withTheRuleBook(t *testing.T) {
	t.Helper()
	t.Setenv(env.RulesPath.Name(), ruleBookOfTheTree)
}

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
				"attributes": map[string]any{
					"hp": 10, "hp_max": 10, "status": "alive", "position": "outside:dark-forest-world",
				},
				"cause": "create",
			})
		fact := answerTo(t, deps, proposal)
		if fact.Type != state.TypeCreated {
			reason, _ := fact.Path().GetString("reason")
			t.Fatalf("answer %s (%s), want %s", fact.Type, reason, state.TypeCreated)
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

// TestTheStateOfTheProcessHoldsTheLaws is the process of --contexts=all
// --bus=memory with the context state as serve builds it: a proposal that breaks
// a law of rules/dark-forest.yaml is refused law_violation with the law named,
// and a legal move of the same character goes through (T-471). State built with
// state.Config{} applies all three refusals below.
func TestTheStateOfTheProcessHoldsTheLaws(t *testing.T) {
	onLoopback(t)
	clearVar(t, env.SwarmFake.Name())
	clearVar(t, env.StateWorlds.Name())
	fixtures, err := tkstate.LoadFixtures(fixturesOfTheTree)
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	world := ""
	for _, e := range fixtures {
		if e.Type == entity.TypeWorld {
			world = e.ID
		}
	}
	contexts, err := runtime.New([]string{runtime.All})
	if err != nil {
		t.Fatalf("New(all): %v", err)
	}
	started := make(chan runtime.Deps, 1)
	probe := &witness{name: "probe", rec: &recorder{}, started: started}

	runUntilStarted(t, newProcess(append(contexts, probe), openBus), started, func(deps runtime.Deps) {
		for _, e := range fixtures {
			created := answerTo(t, deps, eventbus.NewRoot(state.TypeCreateProposed, contracts.SourceMvctl, world, nil,
				eventbus.ActorSystem, map[string]any{
					"proposal_id": "bootstrap:" + world + ":" + e.Type + "/" + e.ID,
					"entity":      map[string]any{"entity": map[string]any{"id": e.ID, "type": e.Type}, "name": e.Name},
					"attributes":  e.Attributes,
					"cause":       "init",
				}))
			if created.Type != state.TypeCreated {
				reason, _ := created.Path().GetString("reason")
				t.Fatalf("the fixture %s was refused %s: the world of the fixtures keeps every law", e.ID, reason)
			}
		}

		gateway := func(id, cause string, op entity.Op) eventbus.Event {
			return changeProposal(t, world, contracts.SourceGateway, nil, id, cause, op)
		}
		task := func(id, cause string, op entity.Op) eventbus.Event {
			return changeProposal(t, world, contracts.SourceSwarm,
				&eventbus.AgentRef{ID: "encounter-wolf:solo:player-A", Level: contracts.ProposerTask, Blueprint: "encounter-wolf"},
				id, cause, op)
		}
		for _, tc := range []struct {
			name     string
			proposal eventbus.Event
			law      string
		}{
			{"the gateway moves the character to a region that does not exist",
				gateway("move-nowhere", "move", entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: "nowhere"}), "inv-10"},
			{"the encounter writes an object at the root of hp",
				task("hp-object", "combat", entity.Op{Op: entity.OpSet, Path: entity.AttrHP, Value: map[string]any{"x": 999}}), "inv-02"},
			{"the encounter writes hp above hp_max",
				task("hp-above", "combat", entity.Op{Op: entity.OpSet, Path: entity.AttrHP, Value: 11}), "inv-02"},
		} {
			answer := answerTo(t, deps, tc.proposal)
			if answer.Type != state.TypeRejected {
				t.Errorf("%s: answered %s, want %s law_violation %s", tc.name, answer.Type, state.TypeRejected, tc.law)
				continue
			}
			reason, _ := answer.Path().GetString("reason")
			law, _ := answer.Path().GetString("details.invariant_id")
			if reason != string(state.ReasonLawViolation) || law != tc.law {
				t.Errorf("%s: refused %s %q, want law_violation %s", tc.name, reason, law, tc.law)
			}
		}

		legal := answerTo(t, deps, gateway("move-into-the-forest", "move",
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: "dark-forest-01"}))
		if legal.Type != state.TypeUpdated {
			reason, _ := legal.Path().GetString("reason")
			t.Errorf("a legal move was answered %s %s, want %s: the laws refuse what they must not", legal.Type, reason, state.TypeUpdated)
		}
	})
}

// State does not start without its laws: the context refuses at Start, /health
// fails, and the refusal names the variable, the path, where a relative path
// was looked for and why the book did not load (T-471).
func TestTheStateOfTheProcessDoesNotStartWithoutItsLaws(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "broken.yaml")
	if err := os.WriteFile(broken, []byte("rules_version: [unclosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "dark-forest.yaml")
	for name, tc := range map[string]struct {
		path    string
		chdir   bool
		mention []string
	}{
		"a file that is not there":   {path: missing, mention: []string{missing}},
		"a file that does not parse": {path: broken, mention: []string{broken, "not a rules document"}},
		"the default, from a directory without rules/": {chdir: true,
			mention: []string{"rules/dark-forest.yaml", "working directory"}},
	} {
		t.Run(name, func(t *testing.T) {
			onLoopback(t)
			clearVar(t, env.SwarmFake.Name())
			if tc.chdir {
				dir := t.TempDir()
				t.Chdir(dir)
				clearVar(t, env.RulesPath.Name())
				wd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				tc.mention = append(tc.mention, wd)
			} else {
				t.Setenv(env.RulesPath.Name(), tc.path)
			}
			contexts, err := runtime.New([]string{runtime.All})
			if err != nil {
				t.Fatalf("New(all): %v", err)
			}
			for _, c := range contexts {
				if c.Name() != stateContext {
					continue
				}
				if _, ok := c.(*state.Context); ok {
					t.Fatal("state was built without its laws")
				}
				if got := c.Health().Status; got != runtime.StatusFail {
					t.Errorf("health of state without its laws = %q, want %q", got, runtime.StatusFail)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err = newProcess(contexts, openBus).run(ctx, cancel)
			if err == nil {
				t.Fatal("the process started state without its laws")
			}
			prefix := "start context " + stateContext + ": "
			if !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("refusal %q does not begin with %q", err, prefix)
			}
			for _, want := range append(tc.mention, env.RulesPath.Name()) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not say %q", err, want)
				}
			}
		})
	}
}

// From the root of the tree the default of MV_RULES_PATH is the rule book of
// the tree: `go run ./cmd/multiverse` there builds state with its laws.
func TestTheDefaultRuleBookIsTheOneOfTheTree(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	clearVar(t, env.RulesPath.Name())
	built, err := runtime.New([]string{stateContext})
	if err != nil {
		t.Fatalf("New(state): %v", err)
	}
	if _, ok := built[0].(*state.Context); !ok {
		t.Fatalf("the context state is %T from the root of the tree, want *state.Context: %v",
			built[0], built[0].Health().Details)
	}
}

// changeProposal is entity.update.proposed of one operation on one entity of
// the fixtures, pinned at version 1.
func changeProposal(t *testing.T, world, source string, agent *eventbus.AgentRef, id, cause string,
	op entity.Op) eventbus.Event {
	t.Helper()
	version := int64(1)
	sets := []entity.ChangeSet{{
		Entity:          eventbus.Entity{Entity: eventbus.EntityRef{ID: "player-A", Type: entity.TypePlayer}},
		ExpectedVersion: &version,
		Ops:             []entity.Op{op},
	}}
	raw, err := json.Marshal(sets)
	if err != nil {
		t.Fatal(err)
	}
	var changes any
	if err := json.Unmarshal(raw, &changes); err != nil {
		t.Fatal(err)
	}
	var opts []eventbus.DeriveOption
	if agent != nil {
		opts = append(opts, eventbus.WithAgent(*agent))
	}
	return eventbus.NewRoot(state.TypeUpdateProposed, source, world, nil, eventbus.ActorCI, map[string]any{
		"proposal_id": id,
		"changes":     changes,
		"atomic":      true,
		"cause":       cause,
	}, opts...)
}

// answerTo publishes a proposal and returns the first answer of State to it:
// entity.created, entity.updated or entity.update.rejected caused by it.
func answerTo(t *testing.T, deps runtime.Deps, proposal eventbus.Event) eventbus.Event {
	t.Helper()
	if err := deps.Bus.Publish(context.Background(), proposal); err != nil {
		t.Fatalf("publish %s: %v", proposal.Type, err)
	}
	var waited time.Duration
	for {
		end, err := deps.Journal.End(context.Background(), eventbus.TopicSystemEvents)
		if err != nil {
			t.Fatalf("End: %v", err)
		}
		var answer *eventbus.Event
		if _, err := deps.Journal.ReadRange(context.Background(), eventbus.TopicSystemEvents, 0, end,
			func(_ context.Context, ev eventbus.Event) error {
				if answer == nil && ev.Meta.CausationID == proposal.ID && !state.IsProposal(ev.Type) {
					answer = &ev
				}
				return nil
			}); err != nil {
			t.Fatalf("ReadRange: %v", err)
		}
		if answer != nil {
			return *answer
		}
		if waited += 5 * time.Millisecond; waited > 30*time.Second {
			t.Fatalf("no answer to %s within 30s", proposal.ID)
		}
		<-clock.RealTimers{}.After(5 * time.Millisecond).C()
	}
}
