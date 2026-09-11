package gateway_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
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
	"multiverse-core.io/shared/testkit/state"
)

const (
	worldID  = "dark-forest-world"
	regionID = "dark-forest-01"
	playerA  = "player-A"
)

func fixturesDir() string { return filepath.Join("..", "..", "..", "testdata", "fixtures") }

// --- what a harness refuses to be built from ---

func TestNewHarnessRefusesAWorldItCannotAct(t *testing.T) {
	fixtures := loadFixtures(t)
	region := pick(t, fixtures, regionID)

	cases := map[string]struct {
		bus      eventbus.Bus
		fixtures []*entity.Entity
	}{
		"no bus":      {nil, fixtures},
		"no world":    {newBus(t), []*entity.Entity{region}},
		"two worlds":  {newBus(t), append(slices.Clone(fixtures), pick(t, fixtures, worldID))},
		"no id":       {newBus(t), []*entity.Entity{{Type: entity.TypeWorld}}},
		"nil fixture": {newBus(t), []*entity.Entity{nil}},
		"no fixtures": {newBus(t), nil},
		"same id twice": {newBus(t), []*entity.Entity{
			pick(t, fixtures, worldID), pick(t, fixtures, regionID), pick(t, fixtures, regionID)}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := gateway.NewHarness(tc.bus, tc.fixtures); err == nil {
				t.Fatal("built a harness that cannot act")
			}
		})
	}
}

func TestNewHarnessTakesTheWorldFromTheFixtures(t *testing.T) {
	h, err := gateway.NewHarness(newBus(t), loadFixtures(t))
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	if h.WorldID() != worldID {
		t.Errorf("the harness acts in %q, the fixtures describe %q", h.WorldID(), worldID)
	}
}

// --- one action at a time ---

// TestCreatePlayerProposesTheFixtureCharacter pins where a character comes
// from: the fixture file, attribute for attribute. A harness that invented its
// own numbers would be testing a world no other test shares (T-016).
func TestCreatePlayerProposesTheFixtureCharacter(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()

	if err := h.CreatePlayer(ctx, playerA); err != nil {
		t.Fatalf("create: %v", err)
	}

	proposals := ofType(read(t, bus, eventbus.TopicSystemEvents), gateway.TypeCreateProposed)
	if len(proposals) != 1 {
		t.Fatalf("%d create proposals, want one", len(proposals))
	}
	ev := proposals[0]
	if ev.Source != gateway.Source {
		t.Errorf("source %q, want %q", ev.Source, gateway.Source)
	}
	if cause, _ := ev.Path().GetString("cause"); cause != gateway.CauseCreate {
		t.Errorf("cause %q, want %q", cause, gateway.CauseCreate)
	}

	fixture := pick(t, loadFixtures(t), playerA)
	created, inWorld := fake.Get(playerA)
	if !inWorld {
		t.Fatal("the character State created is not in the world")
	}
	if created.Name != fixture.Name {
		t.Errorf("the character is called %q, the fixture calls it %q", created.Name, fixture.Name)
	}
	for _, attr := range []string{entity.AttrHP, entity.AttrHPMax, entity.AttrAtk, entity.AttrDef} {
		want, _ := fixture.AttrInt(attr)
		if got, _ := created.AttrInt(attr); got != want {
			t.Errorf("%s is %d, the fixture says %d", attr, got, want)
		}
	}
	if version, known := h.Version(playerA); !known || version != created.Version {
		t.Errorf("the harness thinks %s is at version %d, State says %d",
			playerA, version, created.Version)
	}
}

// TestEnterPublishesTheActionThenTheProposal pins the shape of a movement:
// the action first, the change it implies second, and the two tied together by
// the trace fields so that a reader of the journal can see which action moved
// the world (C-04).
func TestEnterPublishesTheActionThenTheProposal(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()
	create(t, h, playerA)

	if err := h.Enter(ctx, playerA, regionID); err != nil {
		t.Fatalf("enter: %v", err)
	}

	actions := ofType(read(t, bus, eventbus.TopicPlayerEvents), gateway.TypeEnteredRegion)
	if len(actions) != 1 {
		t.Fatalf("%d entries, want one", len(actions))
	}
	action := actions[0]
	pa := action.Path()
	if to, _ := pa.GetString("position.to"); to != regionID {
		t.Errorf("position.to is %q, want %q", to, regionID)
	}
	if from, _ := pa.GetString("position.from"); from != gateway.Outside(worldID) {
		t.Errorf("position.from is %q, want %q: the fixtures start outside every region",
			from, gateway.Outside(worldID))
	}
	if target, _ := pa.GetString("target.entity.id"); target != regionID {
		t.Errorf("target is %q, want the region %q", target, regionID)
	}

	proposals := ofType(read(t, bus, eventbus.TopicSystemEvents), gateway.TypeUpdateProposed)
	if len(proposals) != 1 {
		t.Fatalf("%d update proposals, want one", len(proposals))
	}
	proposal := proposals[0]
	if proposal.Meta.CausationID != action.ID {
		t.Errorf("the proposal was caused by %q, the action is %q: the two must be one chain",
			proposal.Meta.CausationID, action.ID)
	}
	if proposal.Meta.CorrelationID != action.Meta.CorrelationID {
		t.Errorf("the proposal correlates to %q, the action to %q",
			proposal.Meta.CorrelationID, action.Meta.CorrelationID)
	}
	if cause, _ := proposal.Path().GetString("cause"); cause != gateway.CauseMove {
		t.Errorf("cause %q, want %q", cause, gateway.CauseMove)
	}

	moved, _ := fake.Get(playerA)
	if position, _ := moved.Position(); position != regionID {
		t.Errorf("the character stands at %q, want %q", position, regionID)
	}
	if where, ok := h.Position(playerA); !ok || where != regionID {
		t.Errorf("the harness thinks the character is at %q", where)
	}
}

// TestLeaveTakesTheCharacterBackOutside is the other half of a visit, and the
// one that shows position.from is the place the previous step left behind.
func TestLeaveTakesTheCharacterBackOutside(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()
	create(t, h, playerA)
	if err := h.Enter(ctx, playerA, regionID); err != nil {
		t.Fatalf("enter: %v", err)
	}

	if err := h.Leave(ctx, playerA); err != nil {
		t.Fatalf("leave: %v", err)
	}

	actions := ofType(read(t, bus, eventbus.TopicPlayerEvents), gateway.TypeLeftRegion)
	if len(actions) != 1 {
		t.Fatalf("%d departures, want one", len(actions))
	}
	pa := actions[0].Path()
	if from, _ := pa.GetString("position.from"); from != regionID {
		t.Errorf("position.from is %q, want the region the character stood in", from)
	}
	if to, _ := pa.GetString("position.to"); to != gateway.Outside(worldID) {
		t.Errorf("position.to is %q, want %q", to, gateway.Outside(worldID))
	}
	left, _ := fake.Get(playerA)
	if position, _ := left.Position(); position != gateway.Outside(worldID) {
		t.Errorf("the character stands at %q after leaving", position)
	}
}

// TestLookAndSayChangeNothing pins the other half of C-04: an action that
// changes nothing State records proposes nothing.
func TestLookAndSayChangeNothing(t *testing.T) {
	h, _, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()
	create(t, h, playerA)
	before := len(ofType(read(t, bus, eventbus.TopicSystemEvents), gateway.TypeUpdateProposed))

	if err := h.Look(ctx, playerA); err != nil {
		t.Fatalf("look: %v", err)
	}
	if err := h.Say(ctx, playerA, "Кто здесь?"); err != nil {
		t.Fatalf("say: %v", err)
	}

	after := len(ofType(read(t, bus, eventbus.TopicSystemEvents), gateway.TypeUpdateProposed))
	if after != before {
		t.Errorf("%d proposals appeared: looking and speaking change nothing", after-before)
	}
	said := ofType(read(t, bus, eventbus.TopicPlayerEvents), gateway.TypeSaid)
	if len(said) != 1 {
		t.Fatalf("%d replies, want one", len(said))
	}
	if text, _ := said[0].Path().GetString("text"); text != "Кто здесь?" {
		t.Errorf("the character said %q", text)
	}
	looked := ofType(read(t, bus, eventbus.TopicPlayerEvents), gateway.TypeLooked)
	if len(looked) != 1 {
		t.Fatalf("%d looks, want one", len(looked))
	}
	// The character has not entered anywhere, so there is nothing to look at
	// and the optional target is left out rather than filled with a guess.
	if _, has := looked[0].Path().GetMap("target"); has {
		t.Error("a look from outside every region named a target")
	}
}

// TestRestPinsTheVersionItSawLast is the guarantee behind every proposal that
// touches hp: the optimistic lock is mandatory there (ADR-013 p. 1), and the
// version it carries is the one State last published, not one the harness
// remembered from a fixture.
func TestRestPinsTheVersionItSawLast(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()
	create(t, h, playerA)
	if err := h.Enter(ctx, playerA, regionID); err != nil {
		t.Fatalf("enter: %v", err)
	}
	wounded := wound(t, fake, playerA, -4)
	// The harness pins the version it has heard of, so it has to have heard of
	// the wound first. That is not a weakness of the stub but the position of
	// any gateway: a change it has not seen yet is a version_conflict — the
	// refusal TestAChangeTheHarnessNeverHeardOfIsRefused pins — and the wait
	// is what a real client does by reading its own read-model.
	waitFor(t, "the harness to hear about the wound", func() bool {
		version, known := h.Version(playerA)
		return known && version == wounded
	})

	if err := h.Rest(ctx, playerA); err != nil {
		t.Fatalf("rest: %v", err)
	}

	rested, _ := fake.Get(playerA)
	hp, _ := rested.HP()
	hpMax, _ := rested.HPMax()
	if hp != hpMax {
		t.Errorf("the character rested and is at %d of %d", hp, hpMax)
	}

	proposals := ofType(read(t, bus, eventbus.TopicSystemEvents), gateway.TypeUpdateProposed)
	last := proposals[len(proposals)-1]
	if cause, _ := last.Path().GetString("cause"); cause != gateway.CauseRest {
		t.Errorf("cause %q, want %q", cause, gateway.CauseRest)
	}
	pinned, ok := last.Path().GetInt("changes[0].expected_version")
	if !ok {
		t.Fatal("the proposal carries no expected_version: hp is one of the four paths " +
			"the contract makes the lock mandatory for")
	}
	// The wound moved the version behind the back of the harness; a harness
	// that pinned anything but what it heard last would have been refused.
	if int64(pinned) != rested.Version-1 {
		t.Errorf("expected_version %d, the character was at %d when it rested",
			pinned, rested.Version-1)
	}
}

// TestAChangeTheHarnessNeverHeardOfIsRefused pins what the wait in
// TestRestPinsTheVersionItSawLast stands in for. A gateway knows the world
// only through the facts it has read, so a proposal pinned to a version the
// world has already left is refused, and refused for that reason and no other
// (C-02 v1.1, ADR-013 p. 1).
//
// The harness here is deaf on purpose: it is never started, is handed the one
// fact a started harness would have folded, and hears nothing after. That is
// the window the wait of the other test closes, held open.
func TestAChangeTheHarnessNeverHeardOfIsRefused(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	create(t, h, playerA)
	facts := ofType(read(t, bus, eventbus.TopicSystemEvents), gateway.TypeCreated)

	deaf, err := gateway.NewHarness(bus, loadFixtures(t))
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	deaf.WithTimeout(300 * time.Millisecond).WithLog(nil)
	if err := deaf.Observe(t.Context(), facts[len(facts)-1]); err != nil {
		t.Fatalf("observe: %v", err)
	}
	wound(t, fake, playerA, -4)

	// The deaf harness never hears the answer either, so the rest ends in the
	// timeout of awaitProposal; what State said is on the bus.
	if err := deaf.Rest(t.Context(), playerA); err == nil {
		t.Fatal("a rest pinned to a version the world had left was reported as success")
	}
	proposals := ofType(read(t, bus, eventbus.TopicSystemEvents), gateway.TypeUpdateProposed)
	stale, _ := proposals[len(proposals)-1].Path().GetString("proposal_id")
	var answer eventbus.Event
	waitFor(t, "the answer of State to "+stale, func() bool {
		for _, ev := range read(t, bus, eventbus.TopicSystemEvents) {
			if ev.Type != gateway.TypeRejected && ev.Type != gateway.TypeUpdated {
				continue
			}
			if id, _ := ev.Path().GetString("proposal_id"); id == stale {
				answer = ev
				return true
			}
		}
		return false
	})

	if answer.Type != gateway.TypeRejected {
		t.Fatalf("State answered the stale rest with %s: a proposal pinned to a version "+
			"the harness never heard of has to be refused", answer.Type)
	}
	if reason, _ := answer.Path().GetString("reason"); reason != state.ReasonVersionConflict {
		t.Errorf("State refused the stale rest with %q, want %q",
			reason, state.ReasonVersionConflict)
	}
}

// --- what the harness refuses to do ---

// TestAnActionNeedsACharacterTheFixturesKnow is what the harness refuses
// before it publishes anything.
//
// Each case says why it is refused and not only that it is, for the reason the
// combat half of this set already gives (TestAFightNeedsSomebodyToFight): an
// action nobody answers ends in an error too, so a test asking for nothing
// more than an error would stay green with every one of these checks taken
// out — it would just be waiting for the timeout instead.
func TestAnActionNeedsACharacterTheFixturesKnow(t *testing.T) {
	h, _, _ := running(t, seedWithoutPlayers)
	ctx := t.Context()

	cases := map[string]struct {
		act     func() error
		because string
	}{
		"create someone who is not in the fixtures": {
			func() error { return h.CreatePlayer(ctx, "player-Z") }, "not in the fixtures",
		},
		"create a region as if it were a character": {
			func() error { return h.CreatePlayer(ctx, regionID) }, "is a region, not a player",
		},
		"look with someone who is not a character": {
			func() error { return h.Look(ctx, worldID) }, "is a world, not a player",
		},
		"enter a region that is not one": {
			func() error { return h.Enter(ctx, playerA, "nowhere") },
			"not a region of the fixtures",
		},
		"enter the world itself": {
			func() error { return h.Enter(ctx, playerA, worldID) }, "is a world, not a region",
		},
		"say nothing": {func() error { return h.Say(ctx, playerA, "") }, "empty text"},
		"leave from outside a region": {
			func() error {
				create(t, h, playerA)
				return h.Leave(ctx, playerA)
			},
			"not a region of the fixtures",
		},
		"act before the character exists": {
			func() error { return h.Enter(ctx, "player-B", regionID) },
			"create it before it acts",
		},
		"run a scenario nobody wrote": {
			func() error { return h.Scenario(ctx, "solo-30") }, "no scenario",
		},
		"step nobody wrote": {
			func() error { return h.Step(ctx, gateway.Step{Action: "fight", Player: playerA}) },
			"unknown action",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.act()
			if err == nil {
				t.Fatal("the harness did something it has no way to do")
			}
			if !strings.Contains(err.Error(), tc.because) {
				t.Errorf("the error is %q, want it to say %q", err, tc.because)
			}
		})
	}
}

// TestARefusalStopsTheAction is what makes a scenario trustworthy: a proposal
// State refused leaves the world somewhere the script did not mean it to be,
// so the harness reports it instead of walking on.
func TestARefusalStopsTheAction(t *testing.T) {
	h, _, _ := running(t, seedEverything)
	ctx := t.Context()

	err := h.CreatePlayer(ctx, playerA)
	if err == nil {
		t.Fatal("creating a character the world already holds was reported as success")
	}
	if got := err.Error(); !strings.Contains(got, state.ReasonDuplicateEntity) {
		t.Errorf("the error is %q; it should name the reason State gave", got)
	}
}

// TestAnActionGivesUpWhenNobodyAnswers pins the other end of the same wait:
// without State on the bus there is no fact to wait for, and the harness says
// so instead of hanging.
func TestAnActionGivesUpWhenNobodyAnswers(t *testing.T) {
	bus := newBus(t)
	h, err := gateway.NewHarness(bus, loadFixtures(t))
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	h.WithTimeout(50 * time.Millisecond)
	if err := h.Start(t.Context()); err != nil {
		t.Fatalf("start: %v", err)
	}

	err = h.CreatePlayer(t.Context(), playerA)
	if err == nil {
		t.Fatal("the harness reported a creation nobody answered as success")
	}
	// The proposal is named, and so is the wait that ran out: without both, a
	// character refused by the fixtures would read the same as a State that
	// never came up, and the two are looked for in different places.
	if !strings.Contains(err.Error(), "no answer to proposal gw-create-"+playerA) {
		t.Errorf("the error is %q; it should name the proposal nobody answered", err)
	}
}

func TestStartTwiceIsRefused(t *testing.T) {
	h, _, _ := running(t, seedWithoutPlayers)
	if err := h.Start(t.Context()); err == nil {
		t.Fatal("a second subscription was opened over the first")
	}
}

func TestACancelledSubscriptionIsNotAFailure(t *testing.T) {
	bus := newBus(t)
	h, err := gateway.NewHarness(bus, loadFixtures(t))
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := h.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	cancel()
	if err := h.Wait(); err != nil {
		t.Errorf("a subscription its caller cancelled reported %v, want nil (C-01)", err)
	}
}

// --- the scripts ---

// TestTheScriptsAreDataATestCanRead is the point of exporting Step: a test
// derives what it expects from the script instead of writing the number down a
// second time, and the visit of the party is the visit of one character three
// times over rather than a second script that can drift.
func TestTheScriptsAreDataATestCanRead(t *testing.T) {
	visit, err := gateway.Script(gateway.ScenarioVisit)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	party, err := gateway.Script(gateway.ScenarioParty)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	if len(party) != 3*len(visit) {
		t.Errorf("the party visit is %d steps, three visits are %d", len(party), 3*len(visit))
	}
	actionsOf := func(steps []gateway.Step) []string {
		out := make([]string, 0, len(steps))
		for _, s := range steps {
			out = append(out, s.Action)
		}
		return out
	}
	if !slices.Equal(actionsOf(party[:len(visit)]), actionsOf(visit)) {
		t.Error("the party visit does not begin with the visit of one character")
	}
	players := map[string]struct{}{}
	for _, step := range party {
		players[step.Player] = struct{}{}
	}
	if len(players) != 3 {
		t.Errorf("the party visit walks %d characters, want the three of the fixtures",
			len(players))
	}

	// A returned script is a copy: a test that edits what it read must not
	// edit the scenario for the next test.
	visit[0].Player = "player-Z"
	again, _ := gateway.Script(gateway.ScenarioVisit)
	if again[0].Player == "player-Z" {
		t.Error("Script hands out the table itself")
	}

	// Every scenario the harness offers is runnable: a name in the table with
	// no script behind it would be found here rather than in the epic that
	// tried to run it.
	names := gateway.Scenarios()
	if !slices.IsSorted(names) {
		t.Errorf("Scenarios() is %v, want them in order", names)
	}
	for _, name := range names {
		steps, err := gateway.Script(name)
		if err != nil || len(steps) == 0 {
			t.Errorf("scenario %q: %d steps, %v", name, len(steps), err)
		}
	}
	if _, err := gateway.Script("solo-30"); err == nil {
		t.Error("the harness claims a scenario nobody wrote")
	}
}

// TestWhatAStepIsWorth is the table the e2e derives its expectations from.
func TestWhatAStepIsWorth(t *testing.T) {
	cases := []struct {
		action, playerEvent, proposes, fact string
	}{
		{gateway.ActionCreate, "", gateway.TypeCreateProposed, gateway.TypeCreated},
		{gateway.ActionEnter, gateway.TypeEnteredRegion, gateway.TypeUpdateProposed, gateway.TypeUpdated},
		{gateway.ActionLeave, gateway.TypeLeftRegion, gateway.TypeUpdateProposed, gateway.TypeUpdated},
		{gateway.ActionRest, gateway.TypeRested, gateway.TypeUpdateProposed, gateway.TypeUpdated},
		{gateway.ActionLook, gateway.TypeLooked, "", ""},
		{gateway.ActionSay, gateway.TypeSaid, "", ""},
		// The two combat steps: the harness publishes the action and somebody
		// else proposes what it changed, so Proposes and Fact stay empty and
		// Resolves is what says the step is answered at all (T-400).
		{gateway.ActionAttack, gateway.TypeAttacked, "", ""},
		{gateway.ActionFlee, gateway.TypeFleeAttempted, "", ""},
		{"parry", "", "", ""},
	}
	resolved := map[string]bool{gateway.ActionAttack: true, gateway.ActionFlee: true}
	for _, tc := range cases {
		step := gateway.Step{Action: tc.action}
		if got := step.PlayerEvent(); got != tc.playerEvent {
			t.Errorf("%s publishes %q, want %q", tc.action, got, tc.playerEvent)
		}
		if got := step.Proposes(); got != tc.proposes {
			t.Errorf("%s proposes %q, want %q", tc.action, got, tc.proposes)
		}
		if got := step.Fact(); got != tc.fact {
			t.Errorf("%s is answered with %q, want %q", tc.action, got, tc.fact)
		}
		if got := step.Resolves(); got != resolved[tc.action] {
			t.Errorf("%s is resolved by the fight: %t, want %t", tc.action, got, resolved[tc.action])
		}
	}
}

// TestScenarioRunsEveryStepInOrder walks the whole solo visit against State
// and checks the world it leaves behind.
func TestScenarioRunsEveryStepInOrder(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()

	if err := h.Scenario(ctx, gateway.ScenarioVisit); err != nil {
		t.Fatalf("scenario: %v", err)
	}

	script, _ := gateway.Script(gateway.ScenarioVisit)
	want := make([]string, 0, len(script))
	for _, step := range script {
		if typ := step.PlayerEvent(); typ != "" {
			want = append(want, typ)
		}
	}
	have := make([]string, 0, len(want))
	for _, ev := range read(t, bus, eventbus.TopicPlayerEvents) {
		have = append(have, ev.Type)
	}
	if !slices.Equal(have, want) {
		t.Errorf("the harness published %v, the script says %v", have, want)
	}

	visited, inWorld := fake.Get(playerA)
	if !inWorld {
		t.Fatal("the character is not in the world after its visit")
	}
	if position, _ := visited.Position(); position != gateway.Outside(worldID) {
		t.Errorf("the character ended at %q", position)
	}
	if refusals := ofType(read(t, bus, eventbus.TopicSystemEvents), state.TypeRejected); len(refusals) != 0 {
		t.Errorf("%d proposals of the scenario were refused", len(refusals))
	}
}

// TestEverythingTheHarnessPublishesIsValid is the promise of the package doc:
// the harness is a stub about where an action comes from, not about what it
// looks like on the wire.
func TestEverythingTheHarnessPublishesIsValid(t *testing.T) {
	h, _, bus := running(t, seedWithoutPlayers)
	if err := h.Scenario(t.Context(), gateway.ScenarioVisit); err != nil {
		t.Fatalf("scenario: %v", err)
	}
	published := 0
	for _, topic := range []string{eventbus.TopicPlayerEvents, eventbus.TopicSystemEvents} {
		for _, ev := range read(t, bus, topic) {
			if ev.Source != gateway.Source {
				continue
			}
			published++
			if err := contracts.Validate(ev); err != nil {
				t.Errorf("%s is not valid: %v", ev.Type, err)
			}
			spec, known := contracts.Lookup(ev.Type)
			if !known {
				t.Errorf("%s is not in the registry", ev.Type)
				continue
			}
			if !slices.Contains(spec.Publishers, gateway.Source) {
				t.Errorf("%s may not be published by %q, only by %v",
					ev.Type, gateway.Source, spec.Publishers)
			}
		}
	}
	if published == 0 {
		t.Fatal("the harness published nothing")
	}
}

// --- the world of the tests ---

type seeding func(*testing.T, *state.FakeState, []*entity.Entity)

func seedWithoutPlayers(t *testing.T, fake *state.FakeState, fixtures []*entity.Entity) {
	t.Helper()
	keep := make([]*entity.Entity, 0, len(fixtures))
	for _, e := range fixtures {
		if e.Type != entity.TypePlayer {
			keep = append(keep, e)
		}
	}
	if err := fake.Seed(keep); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func seedEverything(t *testing.T, fake *state.FakeState, fixtures []*entity.Entity) {
	t.Helper()
	if err := fake.Seed(fixtures); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// running is a started harness with a started State behind it on one bus. Its
// timeout is the one no test here should ever reach.
func running(t *testing.T, seed seeding) (*gateway.Harness, *state.FakeState, *membus.Bus) {
	t.Helper()
	return runningWithin(t, seed, 3*time.Second)
}

// runningWithin is the same for a case that means to run the timeout out: what
// costs three seconds per action is worth shortening when waiting to the end is
// the point of the case, and it is a setting, so it is made before Start.
func runningWithin(t *testing.T, seed seeding,
	timeout time.Duration) (*gateway.Harness, *state.FakeState, *membus.Bus) {
	t.Helper()
	bus := newBus(t)
	fixtures := loadFixtures(t)

	fake, err := state.New(state.Config{
		Bus: bus, Store: objstore.NewMemoryWithClock(clock.NewManual(testkit.Epoch)),
		WorldID: worldID, RulesVersion: "0.1",
	})
	if err != nil {
		t.Fatalf("fake state: %v", err)
	}
	seed(t, fake, fixtures)

	h, err := gateway.NewHarness(bus, fixtures)
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	h.WithTimeout(timeout).WithLog(nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = fake.Wait(); _ = h.Wait() })
	if err := fake.Start(ctx); err != nil {
		t.Fatalf("start state: %v", err)
	}
	if err := h.Start(ctx); err != nil {
		t.Fatalf("start harness: %v", err)
	}
	return h, fake, bus
}

func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	testkit.Deterministic(t, "gw")
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

func loadFixtures(t *testing.T) []*entity.Entity {
	t.Helper()
	fixtures, err := state.LoadFixtures(fixturesDir())
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	return fixtures
}

func pick(t *testing.T, fixtures []*entity.Entity, id string) *entity.Entity {
	t.Helper()
	for _, e := range fixtures {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("%s is not in the fixtures", id)
	return nil
}

func create(t *testing.T, h *gateway.Harness, playerID string) {
	t.Helper()
	if err := h.CreatePlayer(t.Context(), playerID); err != nil {
		t.Fatalf("create %s: %v", playerID, err)
	}
}

// wound moves the character behind the back of the harness, so that the next
// proposal of the harness has to have heard about it to pin the right version.
func wound(t *testing.T, fake *state.FakeState, playerID string, by int) int64 {
	t.Helper()
	current, ok := fake.Get(playerID)
	if !ok {
		t.Fatalf("%s is not in the world", playerID)
	}
	version := current.Version
	ev := eventbus.NewRoot(state.TypeUpdateProposed, contracts.SourceGateway, worldID, nil,
		entity.ActorKindCI, map[string]any{
			"proposal_id": "wound-" + playerID,
			"atomic":      true,
			"cause":       "combat",
			"changes": []entity.ChangeSet{{
				Entity:          eventbus.Entity{Entity: current.Ref().EventRef(), Name: current.Name},
				ExpectedVersion: &version,
				Ops:             []entity.Op{{Op: entity.OpInc, Path: entity.AttrHP, Value: by}},
			}},
		})
	if err := fake.Apply(t.Context(), ev); err != nil {
		t.Fatalf("wound %s: %v", playerID, err)
	}
	after, _ := fake.Get(playerID)
	if after.Version == version {
		t.Fatalf("the wound did not move %s", playerID)
	}
	return after.Version
}

// waitFor polls until the condition holds. The deadline is wall time: the
// harness folds the facts of State in a goroutine, and a manual clock cannot
// say how much time that goroutine has actually had.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.After(3 * time.Second)
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
