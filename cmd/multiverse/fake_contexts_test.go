package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

// --- which swarm the process builds ---

// The two branches of the hook, and the moment the flag is read: every case
// sets the variable after the package was initialised, so a hook that read the
// flag in an init would build the same context in all four.
func TestTheFlagChoosesTheSwarm(t *testing.T) {
	tests := map[string]struct {
		value string
		unset bool
		fake  bool
	}{
		"unset": {unset: true},
		"false": {value: "false"},
		"true":  {value: "true", fake: true},
		"1":     {value: "1", fake: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.unset {
				clearVar(t, env.SwarmFake.Name())
			} else {
				t.Setenv(env.SwarmFake.Name(), tc.value)
			}
			contexts, err := runtime.New([]string{runtime.All})
			if err != nil {
				t.Fatalf("New(all): %v", err)
			}
			built := swarmsOf(contexts)
			if len(built) != 1 {
				t.Fatalf("New(all) built %d contexts named %s, want one", len(built), swarmContext)
			}
			_, isFake := built[0].(flagged)
			_, isStub := built[0].(stub)
			switch {
			case tc.fake && !isFake:
				t.Errorf("%s=%s built %T, want the fake (flagged *swarm.FakeContext)", env.SwarmFake.Name(), tc.value, built[0])
			case !tc.fake && !isStub:
				t.Errorf("%s=%q built %T, want the stub of wave 0", env.SwarmFake.Name(), tc.value, built[0])
			}
		})
	}
}

// The name the hook registers under is the name the fake answers to: a process
// whose /health said "swarm" for a context that called itself something else
// would report on a context --contexts cannot select.
func TestTheFakeAnswersToTheNameItIsRegisteredUnder(t *testing.T) {
	if swarm.ContextName != swarmContext {
		t.Fatalf("swarm.ContextName = %q, the hook registers %q", swarm.ContextName, swarmContext)
	}
	t.Setenv(env.SwarmFake.Name(), "true")
	if got := newSwarm().Name(); got != swarmContext {
		t.Fatalf("the fake calls itself %q, want %q", got, swarmContext)
	}
}

// A flag that is neither true nor false is refused by name, at start, and the
// process does not come up: read as false it would be a healthy process that
// never fights and says nothing about why.
func TestAMalformedFlagRefusesToStart(t *testing.T) {
	onLoopback(t)
	t.Setenv(env.SwarmFake.Name(), "maybe")
	contexts, err := runtime.New([]string{runtime.All})
	if err != nil {
		t.Fatalf("New(all): %v", err)
	}
	if got := swarmsOf(contexts)[0].Health().Status; got != runtime.StatusFail {
		t.Errorf("health of the refused swarm = %q, want %q", got, runtime.StatusFail)
	}
	// A refusal returns at once. The deadline only bounds a process that did
	// come up, so that such a defect is reported by name instead of hanging the
	// package until the timeout of go test.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = newProcess(contexts, openBus).run(ctx, cancel)
	if err == nil {
		t.Fatalf("the process started with %s=maybe", env.SwarmFake.Name())
	}
	for _, want := range []string{env.SwarmFake.Name(), "maybe", swarmContext} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}
	// runtime.StartAll names the context; the refusal must not name it again
	// (review #1 of T-255, N-1).
	if n := strings.Count(err.Error(), "context "+swarmContext); n != 1 {
		t.Errorf("refusal %q names the context %d times, want once", err, n)
	}
}

// A fake that cannot start says which setting asked for it. From a directory
// without rules/ — which is where the process stands inside the image of the
// platform — the refusal begins with the flag and says what the fake looked
// for and where, instead of a bare "open rules/dark-forest.yaml" (review #1 of
// T-255, Mi-1).
func TestAFakeWithoutRulesNamesTheFlag(t *testing.T) {
	onLoopback(t)
	t.Chdir(t.TempDir())
	t.Setenv(env.SwarmFake.Name(), "true")
	contexts, err := runtime.New([]string{runtime.All})
	if err != nil {
		t.Fatalf("New(all): %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = newProcess(contexts, openBus).run(ctx, cancel)
	if err == nil {
		t.Fatal("the fake started without rules/ in the working directory")
	}
	prefix := "start context " + swarmContext + ": " + env.SwarmFake.Name() + "=true: "
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Errorf("refusal %q does not begin with %q", err, prefix)
	}
	for _, want := range []string{"working directory", "rules/dark-forest.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not say %q", err, want)
		}
	}
}

// Every other refusal of the fake carries the flag too, keeps the error of the
// fake under it, and does not blame rules/: a bus that is missing or a rule book
// that does not parse is not a directory missing from the image (review #2 of
// T-255, Mi-3). A refusal that went missing altogether would bring the process
// up with a swarm that does not run.
func TestTheOtherRefusalsOfTheFakeNameTheFlag(t *testing.T) {
	clearVar(t, env.BusValidateOnRead.Name())
	broken := filepath.Join(t.TempDir(), "broken.yaml")
	if err := os.WriteFile(broken, []byte("rules: [unclosed\n"), 0o600); err != nil {
		t.Fatalf("write the broken rule book: %v", err)
	}
	bus, err := openBus(busMemory, contracts.Default(), clock.RealTimers{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("openBus(memory): %v", err)
	}
	defer func() { _ = bus.Close() }()

	tests := map[string]struct {
		cfg  swarm.ContextConfig
		deps runtime.Deps
	}{
		"no bus in deps":          {deps: runtime.Deps{}},
		"rules that do not parse": {cfg: swarm.ContextConfig{RulesPath: broken}, deps: runtime.Deps{Bus: bus}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			bare := swarm.NewFakeContext(tc.cfg).Start(t.Context(), tc.deps)
			if bare == nil {
				t.Fatal("the fake itself started: the case tests nothing")
			}
			err := flagged{swarm.NewFakeContext(tc.cfg)}.Start(t.Context(), tc.deps)
			if err == nil {
				t.Fatalf("the refusal %q of the fake was swallowed", bare)
			}
			if prefix := env.SwarmFake.Name() + "=true: "; !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("refusal %q does not begin with %q", err, prefix)
			}
			if strings.Contains(err.Error(), "working directory") {
				t.Errorf("refusal %q blames rules/ in the working directory; the cause is %q", err, bare)
			}
			if inner := errors.Unwrap(err); inner == nil || inner.Error() != bare.Error() {
				t.Errorf("refusal %q does not carry the error of the fake %q", err, bare)
			}
		})
	}
}

// --- /health of the process ---

// The process answers /health for the swarm it runs: the fake says it is the
// stub of I1-α and which task removes it, the stub of wave 0 says nothing.
func TestTheHealthOfTheProcessNamesTheSwarmItRuns(t *testing.T) {
	tests := map[string]struct {
		value string
		stub  any
	}{
		"with the flag":    {value: "true", stub: "testkit/swarm.FakeContext"},
		"without the flag": {},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			clearVar(t, env.BusValidateOnRead.Name())
			t.Chdir(filepath.Join("..", ".."))
			if tc.value == "" {
				clearVar(t, env.SwarmFake.Name())
			} else {
				t.Setenv(env.SwarmFake.Name(), tc.value)
			}
			contexts, err := runtime.New([]string{runtime.All})
			if err != nil {
				t.Fatalf("New(all): %v", err)
			}
			p := startProcess(t, contexts, openBus)

			code, health := p.health(t, func(code int, _ processHealth) bool { return code != 0 })
			if code != http.StatusOK || health.Status != runtime.StatusOK {
				t.Fatalf("/health = %d %q, want 200 ok: %+v", code, health.Status, health.Details.Contexts)
			}
			if n := len(health.Details.Contexts); n != len(platformContexts) {
				t.Errorf("/health names %d contexts, want %d", n, len(platformContexts))
			}
			sw := health.Details.Contexts[swarmContext]
			if sw.Status != runtime.StatusOK {
				t.Errorf("swarm reports %q, want ok: %v", sw.Status, sw.Details)
			}
			if got := sw.Details["stub"]; got != tc.stub {
				t.Errorf("swarm details.stub = %v, want %v", got, tc.stub)
			}
			if tc.stub != nil && sw.Details["removed_by"] != "T-256" {
				t.Errorf("the fake does not say which task removes it: %v", sw.Details)
			}
			if err := p.stop(t); err != nil {
				t.Fatalf("run: %v", err)
			}
		})
	}
}

// /health of the process reports both halves of the fake: a fight that runs
// while the narrator has gone deaf is a player who swings and never reads a
// word, and the probe compose calls has to fail on it — not a log line.
func TestTheHealthOfTheProcessFailsWhenTheNarratorWentDeaf(t *testing.T) {
	clearVar(t, env.BusValidateOnRead.Name())
	t.Chdir(filepath.Join("..", ".."))
	t.Setenv(env.SwarmFake.Name(), "true")
	contexts, err := runtime.New([]string{runtime.All})
	if err != nil {
		t.Fatalf("New(all): %v", err)
	}
	deaf := func(bus string, reg *contracts.Registry, timers clock.Timers, log *slog.Logger) (transport, error) {
		tr, err := openBus(bus, reg, timers, log)
		if err != nil {
			return nil, err
		}
		return refusingTransport{transport: tr, refuse: swarm.Group + "-"}, nil
	}
	p := startProcess(t, contexts, deaf)

	code, health := p.health(t, func(_ int, h processHealth) bool { return h.Status == runtime.StatusFail })
	if code != http.StatusServiceUnavailable {
		t.Errorf("/health answered %d over a deaf narrator, want %d", code, http.StatusServiceUnavailable)
	}
	sw := health.Details.Contexts[swarmContext]
	if sw.Status != runtime.StatusFail {
		t.Fatalf("swarm reports %q over a deaf narrator, want fail", sw.Status)
	}
	if msg, _ := sw.Details["err"].(string); !strings.Contains(msg, errRefusedGroup.Error()) {
		t.Errorf("swarm reports %q, want the refusal of the narrator", msg)
	}
	if err := p.stop(t); err != nil {
		t.Fatalf("run: %v", err)
	}
}

// --- I1-α through the process ---

// processFights is how many fights on the fixture world go through the
// process. Different fights, because FixedMechanics answers by the identifier
// of the causing event. The identifiers are not reproducible here, though: the
// sequence of eventbus is one for the whole process, and the fake, the doubles
// and the harness draw from it on goroutines of their own, so which blow gets
// which slot of the table depends on the scheduler. Each fight is therefore
// checked against what it did, and the one ending the fixtures cannot promise —
// a character that dies — has a world of its own below.
const processFights = 16

// The fixture characters and the wolf the scripts of the harness name.
const (
	standPlayer = "player-A"
	standWolf   = "wolf-alpha"
)

// TestTheProcessRunsTheFightsOfIAlpha is I1-α told through cmd/multiverse and
// not through a test of the package: the contexts are the ones the hook
// registers, built by runtime.New as serve builds them, and they run through
// process.run on the bus --bus=memory opens. On that bus, and on nothing the
// process owns, stand the two doubles of the parts the binary does not have
// yet — FakeState for the context state, which is still a stub, and the
// harness of the gateway.
//
// The world comes over the bus, the way mvctl world init will bring it
// (state-and-mechanics.md §4.10): the hook gives the fake no fixtures, so a
// fight here opens only because the fake learnt the wolf from entity.created,
// and the character walks in only once it has (learning).
func TestTheProcessRunsTheFightsOfIAlpha(t *testing.T) {
	told := make(map[string]int)
	endings := make(map[string]int)
	for i := range processFights {
		t.Run(fmt.Sprintf("fight-%02d", i), func(t *testing.T) {
			kinds, ending := fightThroughTheProcess(t, fmt.Sprintf("hk%02d", i), nil, factsAsTheyCome)
			for kind, n := range kinds {
				told[kind] += n
			}
			endings[ending]++
		})
	}
	if t.Failed() {
		return
	}
	// What the whole set has to have told at least once: the entry, the
	// opening of the fight and its turns — and one fight fought to its end and
	// won. The death has a test of its own: on the fixture world it is luck.
	for _, kind := range []string{swarm.KindEntry, swarm.KindWorldEvent, swarm.KindTurn} {
		if told[kind] == 0 {
			t.Errorf("no fight of the set told a %s narrative (told: %v)", kind, told)
		}
	}
	if endings["npc_dead"] == 0 {
		t.Errorf("no fight of the set ended with the wolf dead (endings: %v)", endings)
	}
	t.Logf("narratives told: %v; endings: %v", told, endings)
}

// TestTheProcessTellsTheDeathOfACharacter is the fight the character loses,
// through the same path as the set above. The world is the one the test
// initialises, and it is made so that the loss does not depend on the
// scheduler: the wolf has a thousand hit points and outlasts every blow of the
// script, and the character has one and falls to the first bite that lands.
//
// A bite lands with 0.7 (hit or critical of the table of FixedMechanics; d4
// never deals less than 1). The character lives only if all eight bites of the
// script miss and then the flight succeeds (0.7), or fails and the free bite
// misses as well (0.3 · 0.3): 0.3⁸ · (0.7 + 0.09) ≈ 5.2·10⁻⁵, about once in
// 19 000 runs; 0.3⁸ ≈ 6.6·10⁻⁵ is the upper bound. The message says so for
// every outcome that is not a death, because that is the one false red here.
func TestTheProcessTellsTheDeathOfACharacter(t *testing.T) {
	lethal := func(fixtures []*entity.Entity) {
		for _, e := range fixtures {
			switch e.ID {
			case standWolf:
				e.Attributes[entity.AttrHP], e.Attributes[entity.AttrHPMax] = 1000, 1000
			case standPlayer:
				e.Attributes[entity.AttrHP] = 1
			}
		}
	}
	kinds, ending := fightThroughTheProcess(t, "hkdeath", lethal, factsAsTheyCome)
	if kinds[swarm.KindDeath] != 1 || ending != "players_out" {
		t.Errorf("%d death narratives and the encounter ended %q, want the one death of %s and players_out "+
			"(narratives: %v). On this world the character lives only if the wolf misses all eight bites "+
			"and then the flight succeeds or its free bite misses too, about 5.2e-5 of runs; "+
			"any other outcome is a defect", kinds[swarm.KindDeath], ending, standPlayer, kinds)
	}
}

// TestTheStandWaitsUntilTheFakeHasLearntTheWorld is fight-05 of CI run
// 34754402826 made to happen every time (T-454). The facts of the world are
// held back from the fake — a subscription to system_events slower than
// /health — until the stand begins to wait for them. A stand that let the
// character in on /health alone would send the entry into a region where the
// fake knows nobody alive, no encounter would open, and the first blow would
// go unanswered for the whole deadline of the harness, with the message of the
// CI run. A stand that waits lets the facts through and fights as usual. What
// is pinned is that the stand waits before the entry. The held facts only
// stand for a slow runner, so the wait itself lets them through: a stand that
// released them without waiting would pass here too, and is not what this
// test is about.
func TestTheStandWaitsUntilTheFakeHasLearntTheWorld(t *testing.T) {
	fightThroughTheProcess(t, "hklate", nil, factsHeldUntilAwaited)
}

// factsOrder is how the facts of the initialised world reach the fake of the
// hook in fightThroughTheProcess.
type factsOrder int

const (
	// factsAsTheyCome delivers them the moment the bus hands them over.
	factsAsTheyCome factsOrder = iota
	// factsHeldUntilAwaited holds them until the stand begins to wait for them
	// (learning.hold): a stand that lets the character in first has the entry
	// answered by a fake that knows nobody in the region.
	factsHeldUntilAwaited
)

// fightThroughTheProcess runs one skirmish through the process and returns
// the narratives it told by kind and the reason the encounter ended ("" when
// the character walked away from a fight still on). tweak, when given, changes
// the fixture world before it is initialised and before the harness reads it.
func fightThroughTheProcess(t *testing.T, prefix string, tweak func([]*entity.Entity),
	order factsOrder) (map[string]int, string) {
	testkit.Deterministic(t, prefix)
	clearVar(t, env.BusValidateOnRead.Name())
	// The rule book of the fake is rules/dark-forest.yaml under the working
	// directory, as for `go run ./cmd/multiverse` from the root of the tree.
	t.Chdir(filepath.Join("..", ".."))
	t.Setenv(env.SwarmFake.Name(), "true")

	fixtures, err := state.LoadFixtures(filepath.Join("testdata", "fixtures"))
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	if tweak != nil {
		tweak(fixtures)
	}
	contexts, err := runtime.New([]string{runtime.All})
	if err != nil {
		t.Fatalf("New(all): %v", err)
	}
	fake, ok := swarmsOf(contexts)[0].(flagged)
	if !ok {
		t.Fatalf("the hook built %T under %s=true", swarmsOf(contexts)[0], env.SwarmFake.Name())
	}

	doubles, cancelDoubles := context.WithCancel(context.Background())
	stand := &stand{}
	t.Cleanup(func() { cancelDoubles(); stand.wait() })
	// open runs on the goroutine of process.run; the channel hands the watched
	// transport over to the test explicitly instead of through a variable both
	// goroutines touch.
	opened := make(chan *learning, 1)
	open := func(bus string, reg *contracts.Registry, timers clock.Timers, log *slog.Logger) (transport, error) {
		tr, err := openBus(bus, reg, timers, log)
		if err != nil {
			return nil, err
		}
		// The world is initialised before the contexts start, as it is on a
		// platform where mvctl world init ran before the process came up.
		if err := stand.bootstrap(doubles, tr, fixtures); err != nil {
			_ = tr.Close()
			return nil, err
		}
		watched := newLearning(tr, stand.created, order == factsHeldUntilAwaited)
		opened <- watched
		return watched, nil
	}
	p := startProcess(t, contexts, open)
	if code, health := p.health(t, func(code int, _ processHealth) bool { return code != 0 }); code != http.StatusOK {
		t.Fatalf("/health = %d %q before the fight: %+v", code, health.Status, health.Details.Contexts)
	}
	var watched *learning
	select {
	case watched = <-opened:
	default:
		t.Fatal("/health answered, but the process never opened its bus through the stand")
	}
	// /health is ok as soon as the subscriptions of the fake run, not once they
	// have read the world: the character walks in only after that.
	watched.ready(t, p)
	bus, ok := stand.bus.(*membus.Bus)
	if !ok {
		t.Fatalf("the process runs %T, want the in-process bus of --bus=memory", stand.bus)
	}

	h, err := gateway.NewHarness(bus, fixtures)
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	h.WithTimeout(2 * time.Second).WithLog(nil)
	stand.harness = h
	if err := h.Start(doubles); err != nil {
		t.Fatalf("start the harness: %v", err)
	}
	script, err := gateway.Script(gateway.ScenarioSkirmish)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	taken, err := h.Run(doubles, script)
	if err != nil {
		if len(eventsOfType(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterStarted)) == 0 {
			// Nothing opened the fight. The blow the harness reports is not the
			// cause, and neither is the world reaching the fake after the
			// character (review #1 of T-255, N-3): ready ruled that out before
			// the entry (T-454). What is left is the entry itself or what the
			// fake made of it.
			t.Fatalf("the skirmish through the process: %v\nno encounter was opened, although the fake had "+
				"learnt all of %v from entity.created before %s entered: the cause is the entry or what the "+
				"fake made of it", err, stand.created, standPlayer)
		}
		t.Fatalf("the skirmish through the process: %v", err)
	}

	// What the run is worth, counted from what the harness did and from what
	// the world says, never from the table of the narrator.
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
	started := eventsOfType(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterStarted)
	if len(started) != 1 {
		t.Fatalf("%d encounters opened around one character, want one", len(started))
	}
	want[swarm.KindWorldEvent] = len(started)
	const player = standPlayer
	if who, ok := stand.world.Get(player); ok {
		if status, _ := who.Status(); status == entity.StatusDead {
			want[swarm.KindDeath] = 1
		}
	}
	total := 0
	for _, n := range want {
		total += n
	}

	var told []eventbus.Event
	waitUntil(t, "the narratives of the fight", func() bool {
		told = eventsOfType(t, bus, eventbus.TopicNarrativeOutput, swarm.TypeNarrativeOutput)
		return len(told) >= total
	})
	have := map[string]int{}
	for _, ev := range told {
		kind, _ := ev.Path().GetString("kind")
		have[kind]++
		if ev.Source != swarm.Source {
			t.Errorf("a %s narrative came from %q, want the fake of the hook (%s)", kind, ev.Source, swarm.Source)
		}
		if to, _ := ev.Path().GetString("recipients[0].entity.id"); to != player {
			t.Errorf("a %s narrative is addressed to %q, the scope is %s", kind, to, player)
		}
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("a %s narrative is not valid: %v", kind, err)
		}
	}
	for _, kind := range []string{swarm.KindEntry, swarm.KindWorldEvent, swarm.KindTurn, swarm.KindDeath, swarm.KindRound} {
		if have[kind] != want[kind] {
			t.Errorf("%d %s narratives, the run is worth %d (steps taken: %d)", have[kind], kind, want[kind], len(taken))
		}
	}
	onlyRetriedRefusals(t, eventsOn(t, bus, eventbus.TopicSystemEvents))
	if letters, err := bus.DeadLetters(); err != nil || len(letters) != 0 {
		t.Errorf("dead letters: %v (err %v), want none", letters, err)
	}

	// The encounter announces the end only after the fact of the package that
	// closes the fight, and it hears that fact on a subscription of its own:
	// the harness and the narrator hear it on theirs, so Run can return and the
	// narratives can all be out while encounter.ended is still on its way. Read
	// at once, the end of a finished fight came back "" in 5 to 7 of 200 runs
	// of the death under -cpu 1 (T-454). The package names the end in
	// closed_by_event_id, and State applied it before the harness heard of it,
	// so that id is what is awaited.
	ending := ""
	encounterID, _ := started[0].Path().GetString("encounter.entity.id")
	if closed, ok := stand.world.Get(encounterID); ok {
		if endedBy, _ := closed.Attributes[entity.AttrClosedByEventID].(string); endedBy != "" {
			waitUntil(t, "encounter.ended "+endedBy+" announced after its fact", func() bool {
				for _, ev := range eventsOfType(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterEnded) {
					if ev.ID == endedBy {
						ending, _ = ev.Path().GetString("reason")
						return true
					}
				}
				return false
			})
		}
	}
	if code, health := p.health(t, func(code int, _ processHealth) bool { return code != 0 }); code != http.StatusOK {
		t.Errorf("/health = %d %q after the fight: %+v", code, health.Status, health.Details.Contexts)
	}
	if err := p.stop(t); err != nil {
		t.Fatalf("run: %v", err)
	}
	// The process stopped the fake through StopAll, like any other context.
	if got := fake.Health(); got.Status != runtime.StatusDegraded || got.Details["state"] != "stopped" {
		t.Errorf("the fake after the process stopped reports %q %v, want degraded/stopped", got.Status, got.Details)
	}
	return have, ending
}

// stand is what the test puts on the bus of the process: the double of State
// with the world bootstrapped into it, and the harness of the gateway. The
// process owns and closes the bus; the stand only stops its own subscriptions.
type stand struct {
	bus     eventbus.Bus
	world   *state.FakeState
	harness *gateway.Harness
	// created is every entity bootstrap had State create: what the fake has to
	// have learnt before a character enters.
	created []string
}

// bootstrap starts FakeState on the bus and creates the world, the region and
// the wolf of the fixtures through entity.create.proposed, as mvctl world init
// does (source mvctl, actor system, cause init; state-and-mechanics.md §4.10).
// It runs inside openBus, off the goroutine of the test, so it reports instead
// of failing the test.
func (s *stand) bootstrap(ctx context.Context, bus eventbus.Bus, fixtures []*entity.Entity) error {
	s.bus = bus
	world := ""
	for _, e := range fixtures {
		if e.Type == entity.TypeWorld {
			world = e.ID
		}
	}
	st, err := state.New(state.Config{
		Bus: bus, Store: objstore.NewMemoryWithClock(clock.NewManual(testkit.Epoch)),
		WorldID: world, RulesVersion: "0.1",
	})
	if err != nil {
		return err
	}
	s.world = st
	if err := st.Start(ctx); err != nil {
		return err
	}
	var created []string
	for _, e := range fixtures {
		if e.Type == entity.TypePlayer {
			continue
		}
		ev := eventbus.NewRoot(state.TypeCreateProposed, contracts.SourceMvctl, world, nil, eventbus.ActorSystem,
			map[string]any{
				"proposal_id": "bootstrap-" + e.ID,
				"entity":      map[string]any{"entity": map[string]any{"id": e.ID, "type": e.Type}, "name": e.Name},
				"attributes":  e.Attributes,
				"cause":       "init",
			})
		if err := bus.Publish(ctx, ev); err != nil {
			return fmt.Errorf("bootstrap %s: %w", e.ID, err)
		}
		created = append(created, e.ID)
	}
	deadline := testkit.After(10 * time.Second)
	for _, id := range created {
		for {
			if _, ok := st.Get(id); ok {
				break
			}
			select {
			case <-deadline:
				return fmt.Errorf("bootstrap: State never created %s", id)
			case <-clock.RealTimers{}.After(time.Millisecond).C():
			}
		}
	}
	s.created = created
	return nil
}

func (s *stand) wait() {
	if s.world != nil {
		_ = s.world.Wait()
	}
	if s.harness != nil {
		_ = s.harness.Wait()
	}
}

// onlyRetriedRefusals forgives a refusal only when it is the version race and
// the package came back applied under the same identifier; any other refusal
// is a defect of whoever proposed the change (the rule of the stand of T-219).
func onlyRetriedRefusals(t *testing.T, facts []eventbus.Event) {
	t.Helper()
	applied := make(map[string]bool)
	for _, ev := range facts {
		if ev.Type == state.TypeCreated || ev.Type == state.TypeUpdated {
			id, _ := ev.Path().GetString("proposal_id")
			applied[id] = true
		}
	}
	for _, ev := range facts {
		if ev.Type != state.TypeRejected {
			continue
		}
		reason, _ := ev.Path().GetString("reason")
		proposal, _ := ev.Path().GetString("proposal_id")
		if reason != gateway.ReasonVersionConflict || !applied[proposal] {
			t.Errorf("State refused %s: %s", proposal, reason)
		}
	}
}

// --- the process under test ---

// running is one process.run in the background, listening on an address the
// test knows, so that /health is asked over HTTP as compose asks it.
type running struct {
	addr    string
	cancel  context.CancelFunc
	done    chan error
	stopped bool
	err     error
}

func startProcess(t *testing.T, contexts []runtime.Context, open openBusFunc) *running {
	t.Helper()
	addr := loopbackAddr(t)
	t.Setenv(env.CoreAddr.Name(), addr)
	t.Setenv(env.GatewayDataDir.Name(), sqlitedir.Temp(t)) // the gateway of --contexts=all is real since T-303
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{addr: addr, cancel: cancel, done: make(chan error, 1)}
	go func() { r.done <- newProcess(contexts, open).run(ctx, cancel) }()
	t.Cleanup(func() { _ = r.stop(t) })
	return r
}

// stop ends the run the way a signal does and returns what run returned.
func (r *running) stop(t *testing.T) error {
	t.Helper()
	if r.stopped {
		return r.err
	}
	r.cancel()
	select {
	case r.err = <-r.done:
	case <-testkit.After(30 * time.Second):
		t.Fatal("the process did not stop within 30s")
	}
	r.stopped = true
	return r.err
}

// processHealth is the answer of GET /health (runtime.Aggregate).
type processHealth struct {
	Status  string `json:"status"`
	Details struct {
		Contexts map[string]runtime.Status `json:"contexts"`
	} `json:"details"`
}

// health polls /health until the answer satisfies until, and fails the test if
// the process ends first or nothing satisfying arrives within the budget.
func (r *running) health(t *testing.T, until func(code int, h processHealth) bool) (int, processHealth) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := testkit.After(30 * time.Second)
	var last error
	for {
		code, h, err := getHealth(client, "http://"+r.addr+"/health")
		if err == nil && until(code, h) {
			return code, h
		}
		if err != nil {
			last = err
		}
		select {
		case err := <-r.done:
			r.stopped, r.err = true, err
			t.Fatalf("the process ended while /health was awaited: %v", err)
		case <-deadline:
			t.Fatalf("/health never answered as expected within 30s (last: %d %+v, err %v)", code, h, last)
		case <-clock.RealTimers{}.After(10 * time.Millisecond).C():
		}
	}
}

func getHealth(client *http.Client, url string) (int, processHealth, error) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, processHealth{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var h processHealth
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return 0, processHealth{}, err
	}
	return resp.StatusCode, h, nil
}

// loopbackAddr reserves a port on the loopback interface and gives it back, so
// that the test can dial the process it starts.
func loopbackAddr(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	addr := lis.Addr().String()
	if err := lis.Close(); err != nil {
		t.Fatalf("release the port: %v", err)
	}
	return addr
}

// errRefusedGroup is what refusingTransport answers a refused subscription with.
var errRefusedGroup = errors.New("the broker refused the consumer group")

// refusingTransport is the transport of the process refusing the consumer
// groups that start with a prefix — how a subscription dies on its own rather
// than by being cancelled.
type refusingTransport struct {
	transport
	refuse string
}

func (r refusingTransport) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	if strings.HasPrefix(group, r.refuse) {
		return errRefusedGroup
	}
	return r.transport.Subscribe(ctx, topic, group, h)
}

// learning is the transport of the process watched from the side of the fake
// of the hook: it knows when the encounter has folded the entity.created of
// every entity the stand initialised, which is when a character may walk in.
//
// /health cannot tell. The fake reports ok as soon as its subscriptions run,
// and C-01 orders no two topics: the entry on player_events can reach the
// encounter before the wolf on system_events does, and an entry into a region
// where the encounter knows nobody alive opens nothing. That is fight-05 of CI
// run 34754402826 under -race (T-454). So the stand waits for the handler of
// the fake to return on those facts — the moment the encounter holds the
// entity under its lock — rather than for a length of time.
type learning struct {
	transport

	mu      sync.Mutex
	pending map[string]bool
	learnt  chan struct{}
	// refusal is the last error the handler of the fake returned on an
	// entity.created: it tells a fact the fake refused from one that never
	// reached it.
	refusal error

	// hold keeps entity.created from the encounter until gate opens, which is
	// when the stand begins to wait (factsHeldUntilAwaited). A held fact whose
	// subscription is cancelled is let go unhandled: the process is stopping.
	hold bool
	gate chan struct{}
}

func newLearning(tr transport, created []string, hold bool) *learning {
	l := &learning{
		transport: tr,
		pending:   make(map[string]bool, len(created)),
		learnt:    make(chan struct{}),
		hold:      hold,
		gate:      make(chan struct{}),
	}
	for _, id := range created {
		l.pending[id] = true
	}
	if len(l.pending) == 0 {
		close(l.learnt)
	}
	return l
}

// fold marks one entity as learnt by the encounter. Learnt means only that the
// handler of the fake returned nil on its entity.created: Observe returns nil
// on a fact it cannot read or of another world too, and such a fact still
// counts. The stand creates neither, and a fake that dropped the wolf that way
// would show as an encounter that never opened, not as a wait that never ends.
func (l *learning) fold(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.pending[id] {
		return
	}
	delete(l.pending, id)
	if len(l.pending) == 0 {
		close(l.learnt)
	}
}

// refuse remembers an error the handler of the fake returned on entity.created.
func (l *learning) refuse(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refusal = err
}

func (l *learning) missing() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, 0, len(l.pending))
	for id := range l.pending {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// unlearnt says what is missing and why, for the message of ready: a refusal
// of the fake when there was one, and otherwise that the facts never reached
// its handler.
func (l *learning) unlearnt() string {
	missing := l.missing()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.refusal != nil {
		return fmt.Sprintf("%v unlearnt; the fake refused entity.created, last with: %v", missing, l.refusal)
	}
	return fmt.Sprintf("%v unlearnt; the fake refused none of them, so they never reached its handler", missing)
}

// ready lets held facts through, blocks until the encounter has learnt the
// whole initialised world, and fails the test when the process ends first or
// the budget runs out. The stand calls it once, before any character enters.
func (l *learning) ready(t *testing.T, r *running) {
	t.Helper()
	close(l.gate)
	select {
	case <-l.learnt:
	case err := <-r.done:
		r.stopped, r.err = true, err
		t.Fatalf("the process ended before the fake learnt the world: %v", err)
	case <-testkit.After(10 * time.Second):
		t.Fatalf("the fake of the hook had not learnt the world from entity.created within 10s: %s", l.unlearnt())
	}
}

// Subscribe watches the subscription of the encounter to system_events and
// passes every other one through untouched.
func (l *learning) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	if group != swarm.EncounterGroup+"-"+eventbus.TopicSystemEvents {
		return l.transport.Subscribe(ctx, topic, group, h)
	}
	return l.transport.Subscribe(ctx, topic, group, func(ctx context.Context, ev eventbus.Event) error {
		if l.hold && ev.Type == state.TypeCreated {
			select {
			case <-l.gate:
			case <-ctx.Done():
				return nil
			}
		}
		if err := h(ctx, ev); err != nil {
			if ev.Type == state.TypeCreated {
				l.refuse(err)
			}
			return err
		}
		// After the handler and not before: until Observe has returned, the
		// entity is not in the view the entry is answered from.
		if ev.Type == state.TypeCreated {
			id, _ := ev.Path().GetString("entity.entity.id")
			l.fold(id)
		}
		return nil
	})
}

// --- reading the bus ---

func swarmsOf(contexts []runtime.Context) []runtime.Context {
	var out []runtime.Context
	for _, c := range contexts {
		if c.Name() == swarmContext {
			out = append(out, c)
		}
	}
	return out
}

func eventsOn(t *testing.T, bus *membus.Bus, topic string) []eventbus.Event {
	t.Helper()
	records, err := bus.Records(topic)
	if err != nil {
		t.Fatalf("read %s: %v", topic, err)
	}
	out := make([]eventbus.Event, 0, len(records))
	for i, body := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatalf("decode %s[%d]: %v", topic, i, err)
		}
		out = append(out, ev)
	}
	return out
}

func eventsOfType(t *testing.T, bus *membus.Bus, topic, typ string) []eventbus.Event {
	t.Helper()
	var out []eventbus.Event
	for _, ev := range eventsOn(t, bus, topic) {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

// waitUntil polls until the condition holds. The deadline is wall time: the
// doubles and the contexts run their subscriptions in goroutines.
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.After(10 * time.Second)
	for !cond() {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		case <-clock.RealTimers{}.After(time.Millisecond).C():
		}
	}
}
