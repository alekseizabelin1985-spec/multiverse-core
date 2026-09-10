//go:build e2e

// The end-to-end scenario of the v0 stubs: the harness of C-04 acts, the stub
// of State answers, and the stub of the narrator writes the text a player
// would see — all three on the in-process bus, with no HTTP, no broker and no
// model (design.md §10, tasks.md T-018).
//
// It is the readiness check of wave 0 stated as behaviour: "testkit holds
// membus plus a v0 of every stub of C-02…C-05" is a sentence one can only
// believe after the stubs have been run together.
//
// There is no fight in it. The only stub of Phase 1 that resolves a blow is
// testkit/swarm.FakeEncounter of EPIC-003, and the combat scenario — thirty
// narratives, a fact per hit — moves to I1-α with it (contracts.md C-05, the
// decision of the orchestrator in tasks.md T-018).
package e2e_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway"
	"multiverse-core.io/shared/testkit/membus"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

// scenario is the script the whole test is derived from. Nothing below counts
// anything by hand: how many narratives, how many facts and which actions in
// which order all come from the steps of this scenario, so that a scenario
// that grows a step cannot leave a stale number behind.
const scenario = gateway.ScenarioParty

func fixturesDir() string { return filepath.Join("..", "..", "testdata", "fixtures") }

func TestStubsV0RunTheScenarioTogether(t *testing.T) {
	testkit.Deterministic(t, "e2e")
	bus := newBus(t)
	fixtures := loadFixtures(t)
	worldID := worldOf(t, fixtures)

	// State starts with the world, the region and the wolf in it, and without
	// the characters: creating them is the first step of the scenario, and a
	// world that already held them would answer that step with
	// duplicate_entity.
	fake := newState(t, bus, worldID)
	if err := fake.Seed(without(fixtures, entity.TypePlayer)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	harness, err := gateway.NewHarness(bus, fixtures)
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	narrator, err := swarm.NewFakeNarrator(bus, worldID)
	if err != nil {
		t.Fatalf("narrator: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		_ = fake.Wait()
		_ = harness.Wait()
		_ = narrator.Wait()
	}()

	if err := fake.Start(ctx); err != nil {
		t.Fatalf("start state: %v", err)
	}
	if err := harness.Start(ctx); err != nil {
		t.Fatalf("start harness: %v", err)
	}
	if err := narrator.Start(ctx); err != nil {
		t.Fatalf("start narrator: %v", err)
	}

	script, err := gateway.Script(scenario)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	if err := harness.Scenario(ctx, scenario); err != nil {
		t.Fatalf("scenario %s: %v", scenario, err)
	}

	assertActionsInOrder(t, bus, script)
	assertFactPerAction(t, bus, fake, script, worldID)
	// What has already gone wrong is asked about before what is still awaited.
	// An event the bus refused is a dead letter that names the schema, the
	// field and the event that caused it; the wait for a narrative that will
	// therefore never arrive can only report a timeout ten seconds later.
	assertNothingWasDeadLettered(t, bus)
	assertNarratives(t, bus, script, worldID)
	assertEverythingIsValid(t, bus)
}

// --- what the scenario says must have happened ---

// assertActionsInOrder checks that every step that publishes a player action
// published exactly that action, for that character, in the order of the
// script. Order matters: a look before the character has entered the region
// tells a different story than the script does.
func assertActionsInOrder(t *testing.T, bus *membus.Bus, script []gateway.Step) {
	t.Helper()
	type action struct{ typ, player string }
	want := make([]action, 0, len(script))
	for _, step := range script {
		if typ := step.PlayerEvent(); typ != "" {
			want = append(want, action{typ, step.Player})
		}
	}

	evs := read(t, bus, eventbus.TopicPlayerEvents)
	have := make([]action, 0, len(evs))
	for _, ev := range evs {
		id, _ := ev.Path().GetString("entity.entity.id")
		have = append(have, action{ev.Type, id})
	}
	if len(have) != len(want) {
		t.Fatalf("%d player actions on the bus, the script has %d\n  have: %v\n  want: %v",
			len(have), len(want), have, want)
	}
	for i := range want {
		if have[i] != want[i] {
			t.Errorf("action %d is %v, the script says %v", i+1, have[i], want[i])
		}
	}

	// The envelope of every action is what C-04 promises about it: the harness
	// is a CI client, the chain starts here, and no agent is behind a player.
	for _, ev := range evs {
		if ev.Source != gateway.Source {
			t.Errorf("%s came from %q, want %q", ev.Type, ev.Source, gateway.Source)
		}
		if ev.Meta.ActorKind != entity.ActorKindCI {
			t.Errorf("%s has actor_kind %q, want %q", ev.Type, ev.Meta.ActorKind, entity.ActorKindCI)
		}
		if ev.Meta.CorrelationID != ev.ID {
			t.Errorf("%s has correlation_id %q, want its own id %q: a player action is a root",
				ev.Type, ev.Meta.CorrelationID, ev.ID)
		}
		if ev.Meta.Agent != nil {
			t.Errorf("%s names agent %q: player_events allow none", ev.Type, ev.Meta.Agent.ID)
		}
	}
}

// assertFactPerAction checks that every step which proposed a change got the
// fact C-02 answers it with, under the identifier of that proposal, and that
// nothing was refused.
func assertFactPerAction(t *testing.T, bus *membus.Bus, fake *state.FakeState,
	script []gateway.Step, worldID string) {
	t.Helper()
	system := read(t, bus, eventbus.TopicSystemEvents)

	if refusals := ofType(system, state.TypeRejected); len(refusals) != 0 {
		for _, ev := range refusals {
			reason, _ := ev.Path().GetString("reason")
			proposal, _ := ev.Path().GetString("proposal_id")
			t.Errorf("proposal %s refused: %s", proposal, reason)
		}
		t.Fatal("the scenario was refused; the assertions below would describe another run")
	}

	proposalsOf := func(typ string) map[string]string {
		out := make(map[string]string)
		for _, ev := range ofType(system, typ) {
			id, _ := ev.Path().GetString("proposal_id")
			who, _ := ev.Path().GetString("entity.entity.id")
			out[id] = who
		}
		return out
	}
	facts := map[string]map[string]string{
		state.TypeCreated: proposalsOf(state.TypeCreated),
		state.TypeUpdated: proposalsOf(state.TypeUpdated),
	}
	proposed := map[string]int{}
	for _, step := range script {
		if fact := step.Fact(); fact != "" {
			proposed[fact]++
		}
	}
	for typ, want := range proposed {
		if got := len(facts[typ]); got != want {
			t.Errorf("%d %s on the bus, the script asks for %d", got, typ, want)
		}
	}

	// Every character the script walked through the region must be in the
	// world afterwards, back outside it, alive and rested. That is the whole
	// visit read off the world instead of off the journal.
	for _, step := range script {
		if step.Action != gateway.ActionLeave {
			continue
		}
		who, inWorld := fake.Get(step.Player)
		if !inWorld {
			t.Errorf("%s is not in the world after its visit", step.Player)
			continue
		}
		if position, _ := who.Position(); position != gateway.Outside(worldID) {
			t.Errorf("%s ended at %q, want %q after leaving",
				step.Player, position, gateway.Outside(worldID))
		}
		if status, _ := who.Status(); status != entity.StatusAlive {
			t.Errorf("%s is %q after a visit with no fight", step.Player, status)
		}
		hp, _ := who.HP()
		hpMax, _ := who.HPMax()
		if hp != hpMax {
			t.Errorf("%s rested and is at %d of %d", step.Player, hp, hpMax)
		}
	}
}

// assertNarratives checks the texts the players would have received: one per
// action the narrator tells about, addressed to the character that acted, from
// a template, past no filter, under the laws of the world.
func assertNarratives(t *testing.T, bus *membus.Bus, script []gateway.Step, worldID string) {
	t.Helper()
	type expected struct{ kind, player string }
	want := make([]expected, 0, len(script))
	for _, step := range script {
		kind, tells := swarm.KindFor(step.PlayerEvent())
		if !tells {
			continue
		}
		want = append(want, expected{kind, step.Player})
	}

	var narratives []eventbus.Event
	waitFor(t, "the narratives of the scenario", func() bool {
		narratives = ofType(read(t, bus, eventbus.TopicNarrativeOutput), swarm.TypeNarrativeOutput)
		return len(narratives) >= len(want)
	})
	if len(narratives) != len(want) {
		t.Fatalf("%d narrative.output on the bus, the script is worth %d",
			len(narratives), len(want))
	}

	for i, ev := range narratives {
		pa := ev.Path()
		if ev.Source != swarm.Source {
			t.Errorf("narrative %d came from %q, want %q", i+1, ev.Source, swarm.Source)
		}
		if ev.Meta.Agent == nil || *ev.Meta.Agent != swarm.Agent {
			t.Errorf("narrative %d names agent %v, want %v", i+1, ev.Meta.Agent, swarm.Agent)
		}
		if kind, _ := pa.GetString("kind"); kind != want[i].kind {
			t.Errorf("narrative %d is kind %q, want %q", i+1, kind, want[i].kind)
		}
		if by, _ := pa.GetString("generated_by"); by != "template" {
			t.Errorf("narrative %d was generated_by %q, want template: no model runs here", i+1, by)
		}
		if text, _ := pa.GetString("text"); text == "" {
			t.Errorf("narrative %d carries no text", i+1)
		}
		if locale, _ := pa.GetString("locale"); locale != eventbus.DefaultLocale {
			t.Errorf("narrative %d is in %q, want %q", i+1, locale, eventbus.DefaultLocale)
		}
		if laws, _ := pa.GetString("laws_version"); laws != swarm.DefaultLawsVersion {
			t.Errorf("narrative %d carries laws_version %q, want %q",
				i+1, laws, swarm.DefaultLawsVersion)
		}
		if applied, _ := pa.GetBool("filter.applied"); applied {
			t.Errorf("narrative %d claims a filter ran; the stub has none", i+1)
		}
		if status, _ := pa.GetString("filter.status"); status != "pass" {
			t.Errorf("narrative %d has filter.status %q, want pass", i+1, status)
		}
		if version, _ := pa.GetString("filter.filter_version"); version != swarm.FilterVersion {
			t.Errorf("narrative %d has filter_version %q, want %q",
				i+1, version, swarm.FilterVersion)
		}
		if id, _ := pa.GetString("narrative_event_id"); id != ev.ID {
			t.Errorf("narrative %d carries narrative_event_id %q, want its own id %q",
				i+1, id, ev.ID)
		}
		if got := eventbus.GetWorldIDFromEvent(ev); got != worldID {
			t.Errorf("narrative %d belongs to world %q, want %q", i+1, got, worldID)
		}
		if to, _ := pa.GetString("recipients[0].entity.id"); to != want[i].player {
			t.Errorf("narrative %d is addressed to %q, want %q", i+1, to, want[i].player)
		}
		if list, _ := pa.GetSlice("recipients"); len(list) != 1 {
			t.Errorf("narrative %d has %d recipients, a solo scope has one", i+1, len(list))
		}
	}
}

// assertEverythingIsValid re-checks every event of every topic against the
// registry. The bus already validates on publish and on read, so this is the
// belt to that pair of braces: it is the criterion of the task written out, and
// it also covers the topics nobody in this test subscribes to.
func assertEverythingIsValid(t *testing.T, bus *membus.Bus) {
	t.Helper()
	total := 0
	for _, spec := range contracts.Topics() {
		if spec.Name == eventbus.TopicDeadLetters {
			// A dead letter is a wrapper around an event, not an event of the
			// registry; that there are none is asserted above.
			continue
		}
		for i, ev := range read(t, bus, spec.Name) {
			total++
			if err := contracts.Validate(ev); err != nil {
				t.Errorf("%s[%d] (%s) is not valid: %v", spec.Name, i, ev.Type, err)
			}
		}
	}
	if total == 0 {
		t.Fatal("no events at all: the run did not happen")
	}
	t.Logf("%d events published, all valid against the registry", total)
}

// assertNothingWasDeadLettered reports the events the bus refused to carry.
// It runs before the assertions that wait, because it is the one that can say
// what went wrong: a dead letter carries the validation error and the event
// behind it, while a wait can only say that something never came.
func assertNothingWasDeadLettered(t *testing.T, bus *membus.Bus) {
	t.Helper()
	letters, err := bus.DeadLetters()
	if err != nil {
		t.Fatalf("read dead letters: %v", err)
	}
	for _, dl := range letters {
		t.Errorf("dead letter: %+v", dl)
	}
	if len(letters) != 0 {
		t.Fatalf("%d dead letters, want none", len(letters))
	}
}

// --- the world of the test ---

func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{
		Registry: contracts.Default(),
		Topics:   topics,
		Backoff:  []time.Duration{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

func newState(t *testing.T, bus eventbus.Bus, worldID string) *state.FakeState {
	t.Helper()
	fake, err := state.New(state.Config{
		Bus:          bus,
		Store:        objstore.NewMemoryWithClock(clock.NewManual(testkit.Epoch)),
		WorldID:      worldID,
		RulesVersion: "0.1",
	})
	if err != nil {
		t.Fatalf("fake state: %v", err)
	}
	return fake
}

func loadFixtures(t *testing.T) []*entity.Entity {
	t.Helper()
	entities, err := state.LoadFixtures(fixturesDir())
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	return entities
}

func worldOf(t *testing.T, fixtures []*entity.Entity) string {
	t.Helper()
	for _, e := range fixtures {
		if e.Type == entity.TypeWorld {
			return e.ID
		}
	}
	t.Fatal("the fixtures hold no world")
	return ""
}

// without returns the fixtures minus the entities of one type.
func without(fixtures []*entity.Entity, typ string) []*entity.Entity {
	out := make([]*entity.Entity, 0, len(fixtures))
	for _, e := range fixtures {
		if e.Type != typ {
			out = append(out, e)
		}
	}
	return out
}

// --- reading the bus ---

func read(t *testing.T, bus *membus.Bus, topic string) []eventbus.Event {
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

func ofType(evs []eventbus.Event, typ string) []eventbus.Event {
	out := make([]eventbus.Event, 0, len(evs))
	for _, ev := range evs {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

// waitFor polls until the condition holds. The deadline is wall time: the
// stubs run their subscriptions in goroutines, and a manual clock cannot say
// how much time those goroutines have actually had.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.After(10 * time.Second)
	for {
		if cond() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		case <-clock.RealTimers{}.After(time.Millisecond).C():
		}
	}
}
