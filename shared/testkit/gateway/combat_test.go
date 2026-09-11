package gateway_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit/gateway"
	"multiverse-core.io/shared/testkit/state"
)

// This file is the half of the harness somebody else answers for (T-400).
// Everything the visit does, the harness both publishes and settles; a blow it
// only publishes, and the change belongs to whoever runs the fight — in Phase 1
// testkit/swarm.FakeEncounter of EPIC-003 (contracts.md C-05, tasks.md T-219).
// The encounter at the bottom of this file is the smallest thing that answers
// like one.

const npcID = "wolf-alpha"

// TestAttackPublishesTheActionAndWaitsForTheChange pins both halves of a blow:
// what goes on the wire, and what has to be true of the world before the
// harness lets the next step of a script take a turn.
//
// The second half is the point of the task. The harness proposes nothing for
// an attack, so the only thing that can make the version it holds equal to the
// version State holds is waiting for the fact.
func TestAttackPublishesTheActionAndWaitsForTheChange(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()
	startEncounter(t, bus, fake, encounterFair)
	create(t, h, playerA)
	if err := h.Enter(ctx, playerA, regionID); err != nil {
		t.Fatalf("enter: %v", err)
	}
	before, _ := fake.Get(playerA)

	if err := h.Attack(ctx, playerA, npcID); err != nil {
		t.Fatalf("attack: %v", err)
	}

	actions := ofType(read(t, bus, eventbus.TopicPlayerEvents), gateway.TypeAttacked)
	if len(actions) != 1 {
		t.Fatalf("%d blows, want one", len(actions))
	}
	action := actions[0]
	if action.Source != gateway.Source {
		t.Errorf("source %q, want %q", action.Source, gateway.Source)
	}
	if action.Meta.ActorKind != entity.ActorKindCI {
		t.Errorf("actor_kind %q, want %q", action.Meta.ActorKind, entity.ActorKindCI)
	}
	if action.Meta.CorrelationID != action.ID {
		t.Errorf("correlation_id %q, the action is %q: a player action is a root (C-04)",
			action.Meta.CorrelationID, action.ID)
	}
	if action.Meta.Agent != nil {
		t.Error("the blow named an agent: player_events forbid one (C-01)")
	}
	pa := action.Path()
	if target, _ := pa.GetString("target.entity.id"); target != npcID {
		t.Errorf("the blow is aimed at %q, want the wolf %q", target, npcID)
	}
	if kind, _ := pa.GetString("action.type"); kind != gateway.ActionAttack {
		t.Errorf("action.type is %q, want %q", kind, gateway.ActionAttack)
	}

	// The harness did not propose the change and must still know it happened.
	wounded, _ := fake.Get(playerA)
	if wounded.Version == before.Version {
		t.Fatal("the attack returned before the fight had changed anything: " +
			"the next step of a script would act on a world that is still moving")
	}
	if version, known := h.Version(playerA); !known || version != wounded.Version {
		t.Errorf("the harness thinks %s is at version %d, State says %d",
			playerA, version, wounded.Version)
	}
	wolf, _ := fake.Get(npcID)
	if version, known := h.Version(npcID); !known || version != wolf.Version {
		t.Errorf("the harness thinks %s is at version %d, State says %d",
			npcID, version, wolf.Version)
	}
	// The change came from the encounter under its own source, not from here.
	for _, ev := range ofType(read(t, bus, eventbus.TopicSystemEvents), gateway.TypeUpdateProposed) {
		if ev.Meta.CorrelationID == action.ID && ev.Source == gateway.Source {
			t.Error("the harness proposed the change of a blow itself")
		}
	}
}

// TestAnAttackLeavesTheHarnessAbleToPropose is the reason Attack waits for the
// fact rather than for the decision of the fight.
//
// The rest that follows the blow pins the version the harness holds. Under any
// shorter wait — none at all, or combat.decided, which is published before the
// change and carries no version — that version is one the world has already
// left, and the rest is refused with version_conflict in some runs and not in
// others. Here it is refused in none.
func TestAnAttackLeavesTheHarnessAbleToPropose(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()
	startEncounter(t, bus, fake, encounterFair)
	create(t, h, playerA)
	if err := h.Enter(ctx, playerA, regionID); err != nil {
		t.Fatalf("enter: %v", err)
	}

	if err := h.Attack(ctx, playerA, npcID); err != nil {
		t.Fatalf("attack: %v", err)
	}
	// No waitFor here on purpose: if the harness needed one, Attack has not
	// done its job.
	if err := h.Rest(ctx, playerA); err != nil {
		t.Fatalf("the rest after a blow was refused: %v", err)
	}

	rested, _ := fake.Get(playerA)
	hp, _ := rested.HP()
	hpMax, _ := rested.HPMax()
	if hp != hpMax {
		t.Errorf("the character rested and is at %d of %d", hp, hpMax)
	}
}

// TestFleeAsksToBreakOff is the other action of the fight. It names no target:
// a flight is resolved against a threshold, so player.flee_attempted requires
// the character and the action and nothing else.
func TestFleeAsksToBreakOff(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()
	startEncounter(t, bus, fake, encounterFair)
	create(t, h, playerA)
	if err := h.Enter(ctx, playerA, regionID); err != nil {
		t.Fatalf("enter: %v", err)
	}
	before, _ := fake.Get(playerA)

	if err := h.Flee(ctx, playerA); err != nil {
		t.Fatalf("flee: %v", err)
	}

	attempts := ofType(read(t, bus, eventbus.TopicPlayerEvents), gateway.TypeFleeAttempted)
	if len(attempts) != 1 {
		t.Fatalf("%d attempts to break off, want one", len(attempts))
	}
	pa := attempts[0].Path()
	if kind, _ := pa.GetString("action.type"); kind != gateway.ActionFlee {
		t.Errorf("action.type is %q, want %q", kind, gateway.ActionFlee)
	}
	if _, has := pa.GetMap("target"); has {
		t.Error("the flight named a target: it is resolved against a threshold, " +
			"not against somebody")
	}
	after, _ := fake.Get(playerA)
	if after.Version == before.Version {
		t.Fatal("the flight returned before the fight had answered it")
	}
	if version, known := h.Version(playerA); !known || version != after.Version {
		t.Errorf("the harness thinks %s is at version %d, State says %d",
			playerA, version, after.Version)
	}
}

// TestAnActionNobodyResolvesGivesUp is what a bus without an encounter looks
// like: the harness names the action nobody answered instead of reporting a
// blow that landed nowhere as a success.
func TestAnActionNobodyResolvesGivesUp(t *testing.T) {
	h, _, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()
	create(t, h, playerA)

	// A second harness on the same bus, deaf to nothing but impatient: it is
	// handed the one fact the running harness already folded, so it knows the
	// character, and it gives up in a fraction of a second (WithTimeout is a
	// setting, so it is made before Start).
	hasty, err := gateway.NewHarness(bus, loadFixtures(t))
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	hasty.WithTimeout(150 * time.Millisecond).WithLog(nil)
	facts := ofType(read(t, bus, eventbus.TopicSystemEvents), gateway.TypeCreated)
	if err := hasty.Observe(ctx, facts[len(facts)-1]); err != nil {
		t.Fatalf("observe: %v", err)
	}
	if err := hasty.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	h = hasty

	cases := map[string]func() error{
		"attack": func() error { return h.Attack(ctx, playerA, npcID) },
		"flee":   func() error { return h.Flee(ctx, playerA) },
	}
	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			err := act()
			if err == nil {
				t.Fatal("an action nobody resolved was reported as success")
			}
			if !strings.Contains(err.Error(), "nobody resolved") {
				t.Errorf("the error is %q; it should say that nobody answered", err)
			}
		})
	}
}

// TestAChangeOfTheFightThatNeverComesBackIsCaughtByTheDeadline is what a
// version conflict looks like against a real State, and it is the one refusal
// the harness does not report as an answer.
//
// The encounter here is a version behind on purpose, and — unlike
// testkit/swarm — it never offers the package again. That makes it the worse
// half of the decision of 2026-09-11: a package refused for a race the
// publisher owes a retry to, and a publisher that never pays it. The harness
// waits, because a package offered again a millisecond later is a package that
// arrived; and then it stops, because the deadline is still there. The message
// has to say which of the two dead ends this was — a publisher out of retries,
// not a bus nobody answered on — or the next reader looks for a missing
// encounter that was never missing.
//
// The refusal of a change that is a defect rather than a race ends the wait at
// once instead, by all four of its reasons: reaction_test.go.
func TestAChangeOfTheFightThatNeverComesBackIsCaughtByTheDeadline(t *testing.T) {
	h, fake, bus := runningWithin(t, seedWithoutPlayers, 500*time.Millisecond)
	ctx := t.Context()
	startEncounter(t, bus, fake, encounterStale)
	create(t, h, playerA)
	if err := h.Enter(ctx, playerA, regionID); err != nil {
		t.Fatalf("enter: %v", err)
	}

	err := h.Attack(ctx, playerA, npcID)
	if err == nil {
		t.Fatal("a blow whose change State refused and nobody offered again was reported " +
			"as success")
	}
	if !strings.Contains(err.Error(), "ran out of retries and gave up") {
		t.Errorf("the error is %q; it should say that the package lost the version race "+
			"and never came back", err)
	}
	if strings.Contains(err.Error(), "none arrived") {
		t.Errorf("the error is %q; it blames a silent bus for a package that did arrive "+
			"and was refused", err)
	}
	// The refusal is on the bus and State applied nothing: what the harness sat
	// through is a real conflict and not a slow answer.
	refusals := ofType(read(t, bus, eventbus.TopicSystemEvents), state.TypeRejected)
	if len(refusals) == 0 {
		t.Fatal("nothing was refused: this case is not about a version conflict at all")
	}
	for _, refused := range refusals {
		if reason, _ := refused.Path().GetString("reason"); reason != state.ReasonVersionConflict {
			t.Errorf("State refused for %q, and this case is about %q",
				reason, state.ReasonVersionConflict)
		}
	}
}

// TestAFightNeedsSomebodyToFight is what the harness refuses to swing at.
func TestAFightNeedsSomebodyToFight(t *testing.T) {
	h, _, _ := running(t, seedWithoutPlayers)
	ctx := t.Context()

	// None of these reaches the wait: whom the blow is aimed at and whether
	// the harness has heard of the character are both known before anything
	// is published, which is why the test needs no timeout of its own.
	//
	// Each case says why it is refused, and not only that it is. An action
	// nobody answers ends in an error too, so a test that asked for nothing
	// more than an error would still be green with every one of these checks
	// taken out — it would just be waiting for the timeout instead.
	cases := map[string]struct {
		act     func() error
		because string
	}{
		"attack with somebody who is not in the fixtures": {
			func() error { return h.Attack(ctx, "player-Z", npcID) }, "not in the fixtures",
		},
		"attack something that is not in the fixtures": {
			func() error { return h.Attack(ctx, playerA, "wolf-omega") }, "not in the fixtures",
		},
		"attack a region": {
			func() error { return h.Attack(ctx, playerA, regionID) }, "not an npc",
		},
		"attack another character": {
			func() error { return h.Attack(ctx, playerA, "player-B") }, "not an npc",
		},
		"attack before the character exists": {
			func() error { return h.Attack(ctx, "player-C", npcID) }, "create it before it fights",
		},
		"flee before the character exists": {
			func() error { return h.Flee(ctx, "player-C") }, "create it before it flees",
		},
		"flee with somebody who is not in the fixtures": {
			func() error { return h.Flee(ctx, "player-Z") }, "not in the fixtures",
		},
		"step nobody wrote": {
			func() error { return h.Step(ctx, gateway.Step{Action: "parry", Player: playerA}) },
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

// TestTheSkirmishIsTheVisitPlusTheFight is the derivation the e2e of T-219
// stands on: what a scenario with combat in it is worth is read off the
// script, not written down beside it.
func TestTheSkirmishIsTheVisitPlusTheFight(t *testing.T) {
	visit, err := gateway.Script(gateway.ScenarioVisit)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	skirmish, err := gateway.Script(gateway.ScenarioSkirmish)
	if err != nil {
		t.Fatalf("script: %v", err)
	}

	fights := 0
	for _, step := range skirmish {
		if step.Resolves() {
			fights++
		}
	}
	if fights == 0 {
		t.Fatal("the skirmish resolves nothing: it is a visit under another name")
	}
	if len(skirmish) != len(visit)+fights {
		t.Errorf("the skirmish is %d steps, the visit is %d and the fight adds %d",
			len(skirmish), len(visit), fights)
	}
	// The steps of the visit survive the fight, in order: combat is added to a
	// scenario, it does not rewrite one. What combat does change about them is
	// the condition they are taken under, so the comparison is made without it
	// and the conditions are checked right below.
	kept := make([]gateway.Step, 0, len(visit))
	for _, step := range skirmish {
		if step.Resolves() {
			continue
		}
		step.When = gateway.Anytime
		kept = append(kept, step)
	}
	if !slices.Equal(kept, visit) {
		t.Errorf("what is left of the skirmish without the fight is %v, the visit is %v",
			kept, visit)
	}
	// Nothing of the script promises how long the fight lasts, and nothing of
	// it walks a character the fight may have killed. Both are conditions on
	// the steps, which is why they are read off the script here instead of
	// being trusted to the run.
	fought := false
	for _, step := range skirmish {
		switch {
		case step.Resolves():
			fought = true
			if step.When != gateway.InFight {
				t.Errorf("%s is taken %q: a combat step outside a fight is answered by nobody",
					step.Action, step.When)
			}
		case fought && step.When != gateway.Alive:
			t.Errorf("%s follows the fight and is taken %q: a rest and a walk out are "+
				"refused over a corpse (dead_entity)", step.Action, step.When)
		}
	}
	if blows := slices.IndexFunc(skirmish, func(s gateway.Step) bool {
		return s.Action == gateway.ActionAttack
	}); blows < 0 || skirmish[blows].Turns < 2 {
		t.Error("the script strikes a fixed number of blows: how long a wolf stands up " +
			"to a character is decided by the mechanics, not by the script")
	}
	// Every combat step is answered by somebody else, so it proposes nothing
	// of its own and leaves no fact the script can count — while the action it
	// publishes stays derivable.
	for _, step := range skirmish {
		if !step.Resolves() {
			continue
		}
		if step.Proposes() != "" || step.Fact() != "" {
			t.Errorf("%s proposes %q and expects %q: the fight proposes, not the harness",
				step.Action, step.Proposes(), step.Fact())
		}
		if step.PlayerEvent() == "" {
			t.Errorf("%s publishes nothing on player_events", step.Action)
		}
	}
	if attacks := slices.ContainsFunc(skirmish, func(s gateway.Step) bool {
		return s.Action == gateway.ActionAttack && s.Target == ""
	}); attacks {
		t.Error("a blow with nobody to aim at was written into the script")
	}
}

// TestTheSkirmishRunsAgainstAnEncounter walks the whole scenario and derives
// what it expects from the run, exactly as the visit derives it from its
// script: the sequence of actions on player_events is the sequence of
// Step.PlayerEvent, and the number of facts the harness itself asked for is
// the number of turns with a Fact.
//
// The turns and not the script, because a script with a fight in it is a plan:
// a blow repeats while the encounter is on and a step is skipped when its
// condition has gone. Run reports what was actually taken, and everything
// below is read off that — there is still not one literal here.
func TestTheSkirmishRunsAgainstAnEncounter(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	startEncounter(t, bus, fake, encounterFair)

	script, err := gateway.Script(gateway.ScenarioSkirmish)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	taken, err := h.Run(t.Context(), script)
	if err != nil {
		t.Fatalf("scenario: %v", err)
	}

	want := make([]string, 0, len(taken))
	ownFacts, fights := 0, 0
	for _, step := range taken {
		if typ := step.PlayerEvent(); typ != "" {
			want = append(want, typ)
		}
		if step.Fact() != "" {
			ownFacts++
		}
		if step.Resolves() {
			fights++
		}
	}
	have := make([]string, 0, len(want))
	for _, ev := range read(t, bus, eventbus.TopicPlayerEvents) {
		have = append(have, ev.Type)
	}
	if !slices.Equal(have, want) {
		t.Errorf("the harness published %v, the script says %v", have, want)
	}

	// The facts of the harness are the ones whose proposal it named; the rest
	// came out of the fight, and how many of those there are depends on who
	// was hit, which is why the script counts the fights and not the facts.
	facts := read(t, bus, eventbus.TopicSystemEvents)
	own, fought := 0, 0
	for _, ev := range facts {
		if ev.Type != gateway.TypeCreated && ev.Type != gateway.TypeUpdated {
			continue
		}
		if proposal, _ := ev.Path().GetString("proposal_id"); strings.HasPrefix(proposal, "gw-") {
			own++
			continue
		}
		fought++
	}
	if own != ownFacts {
		t.Errorf("State answered %d proposals of the harness, the script asks for %d",
			own, ownFacts)
	}
	if fought < fights {
		t.Errorf("%d facts came out of the fight, the run fights %d times", fought, fights)
	}
	if fights == 0 {
		t.Error("the run took no combat turn at all: a skirmish that never fought is a visit")
	}
	if refusals := ofType(facts, state.TypeRejected); len(refusals) != 0 {
		t.Errorf("%d proposals of the scenario were refused", len(refusals))
	}
}

// TestEverythingTheFightMakesTheHarnessPublishIsValid is the promise of the
// package doc held over the two new actions: the harness is a stub about where
// an action comes from, not about what it looks like on the wire.
func TestEverythingTheFightMakesTheHarnessPublishIsValid(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	startEncounter(t, bus, fake, encounterFair)
	if err := h.Scenario(t.Context(), gateway.ScenarioSkirmish); err != nil {
		t.Fatalf("scenario: %v", err)
	}

	fought := 0
	for _, ev := range read(t, bus, eventbus.TopicPlayerEvents) {
		if ev.Type != gateway.TypeAttacked && ev.Type != gateway.TypeFleeAttempted {
			continue
		}
		fought++
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
	if fought == 0 {
		t.Fatal("the skirmish published no combat action at all")
	}
}

// --- a fight that ends under the script ---

// TestAnActionOfAFightThatIsOverIsRefusedAtOnce is the harness side of the
// decision of 2026-09-11 on Cr-2 of T-219: a blow struck after the encounter
// closed is an error of the script, not an event of the world, and the answer
// to it is an immediate error naming the reason — not the timeout, which is
// indistinguishable from "nobody is listening".
//
// Neither half of the check is a stopwatch. ErrFightOver is not a thing the
// deadline can produce — the wait that runs out reports how far the answer
// got — and an action that was never published never reached anybody to be
// waited for. Together they say "at once" without measuring anything.
func TestAnActionOfAFightThatIsOverIsRefusedAtOnce(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	ctx := t.Context()
	startEncounter(t, bus, fake, encounterDeadlyToTheNPC)
	create(t, h, playerA)
	if err := h.Enter(ctx, playerA, regionID); err != nil {
		t.Fatalf("enter: %v", err)
	}
	if err := h.Attack(ctx, playerA, npcID); err != nil {
		t.Fatalf("the blow that ended the fight: %v", err)
	}
	waitFor(t, "the harness to hear that the fight is over", func() bool {
		_, ended, known := h.Fight(playerA)
		return known && ended != ""
	})
	published := len(read(t, bus, eventbus.TopicPlayerEvents))

	cases := map[string]func() error{
		"attack": func() error { return h.Attack(ctx, playerA, npcID) },
		"flee":   func() error { return h.Flee(ctx, playerA) },
	}
	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			err := act()
			if err == nil {
				t.Fatal("an action of a fight that is over was reported as success")
			}
			if !errors.Is(err, gateway.ErrFightOver) {
				t.Errorf("the error is %q; it should be ErrFightOver and not a timeout", err)
			}
			if !strings.Contains(err.Error(), entity.ResolutionNPCDead) {
				t.Errorf("the error is %q; it should name the reason the fight ended", err)
			}
		})
	}
	if now := len(read(t, bus, eventbus.TopicPlayerEvents)); now != published {
		t.Errorf("%d actions went on the wire after the fight was over, want none",
			now-published)
	}
}

// TestTheSkirmishStopsSwingingWhenTheFightEnds is the scenario half of the
// same thing, and the reason the script has no fixed number of blows. The wolf
// falls to the first one here; every blow the script still had, and the flight
// after them, belong to a fight that no longer exists.
func TestTheSkirmishStopsSwingingWhenTheFightEnds(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	startEncounter(t, bus, fake, encounterDeadlyToTheNPC)

	script, err := gateway.Script(gateway.ScenarioSkirmish)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	taken, err := h.Run(t.Context(), script)
	if err != nil {
		t.Fatalf("the skirmish did not survive the death of the wolf: %v", err)
	}

	if blows := turns(taken, gateway.ActionAttack); blows != 1 {
		t.Errorf("%d blows: the wolf fell to the first one and the fight ended with it", blows)
	}
	if flights := turns(taken, gateway.ActionFlee); flights != 0 {
		t.Errorf("%d attempts to flee a fight that was over", flights)
	}
	// The walk goes on: what ended is the fight, not the character.
	for _, action := range []string{gateway.ActionSay, gateway.ActionRest, gateway.ActionLeave} {
		if turns(taken, action) != 1 {
			t.Errorf("the run did not %s after the fight", action)
		}
	}
	if wolf, _ := fake.Get(npcID); !wolf.IsTerminal() {
		t.Error("the wolf outlived the blow that was supposed to end it")
	}
	if refusals := ofType(read(t, bus, eventbus.TopicSystemEvents), state.TypeRejected); len(refusals) != 0 {
		t.Errorf("%d proposals of the scenario were refused", len(refusals))
	}
}

// TestTheSkirmishDoesNotWalkACorpse is the other end of a fight that ends: the
// character falls. A rest and a walk out are refused over a corpse
// (dead_entity, C-02 v1.2), so a script that took them anyway would fail on
// the world instead of on itself — which is exactly what the review found the
// old script doing.
func TestTheSkirmishDoesNotWalkACorpse(t *testing.T) {
	h, fake, bus := running(t, seedWithoutPlayers)
	startEncounter(t, bus, fake, encounterDeadlyToThePlayer)

	script, err := gateway.Script(gateway.ScenarioSkirmish)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	taken, err := h.Run(t.Context(), script)
	if err != nil {
		t.Fatalf("the skirmish did not survive the death of the character: %v", err)
	}

	if blows := turns(taken, gateway.ActionAttack); blows != 1 {
		t.Errorf("%d blows: the character fell to the answer to the first", blows)
	}
	for _, action := range []string{gateway.ActionFlee, gateway.ActionRest, gateway.ActionLeave} {
		if took := turns(taken, action); took != 0 {
			t.Errorf("the run took %d %s turns with a dead character", took, action)
		}
	}
	if who, _ := fake.Get(playerA); !who.IsTerminal() {
		t.Error("the character survived the blow that was supposed to kill it")
	}
	if status, known := h.Status(playerA); !known || status != entity.StatusDead {
		t.Errorf("the harness holds %s at status %q, State says dead", playerA, status)
	}
	if refusals := ofType(read(t, bus, eventbus.TopicSystemEvents), state.TypeRejected); len(refusals) != 0 {
		t.Errorf("%d proposals were refused: the script walked a character it should not have",
			len(refusals))
	}
}

// turns is how many times a run took one action.
func turns(taken []gateway.Step, action string) int {
	n := 0
	for _, step := range taken {
		if step.Action == action {
			n++
		}
	}
	return n
}

// --- what the harness does when the fight ends under an action ---

// TestAnActionTheFightDoesNotOutliveFailsWithTheFight is the race the
// lifecycle costs: an encounter closes on world_events while the action it
// will never answer is already waiting on system_events. Waiting it out is a
// whole timeout spent on an answer that cannot come, so the wait ends with the
// fight — and says which fight and why.
//
// The end is published as a root of its own here, because that is what makes
// the case: an encounter.ended derived from the action itself belongs to that
// action and is checked by the test below.
func TestAnActionTheFightDoesNotOutliveFailsWithTheFight(t *testing.T) {
	bus := newBus(t)
	h := knowing(t, bus, nil)
	opens(t, bus, h, "encounter-1", playerA)
	answerFirstAction(t, bus, func(ctx context.Context, _ eventbus.Event) error {
		return bus.Publish(ctx, ended(t, "encounter-1", entity.ResolutionNPCDead))
	})

	err := h.Attack(t.Context(), playerA, npcID)

	if err == nil {
		t.Fatal("an action of a fight that closed under it was reported as success")
	}
	if !errors.Is(err, gateway.ErrFightOver) {
		t.Errorf("the error is %q; it should be ErrFightOver and not a timeout", err)
	}
	if !strings.Contains(err.Error(), entity.ResolutionNPCDead) {
		t.Errorf("the error is %q; it should name the reason the fight ended", err)
	}
}

// TestTheBlowThatEndedTheFightIsStillAnswered is the other side of that, and
// the reason the end of a fight is not simply the end of every wait: the blow
// that killed the wolf ends the fight AND is answered by a change, and the two
// arrive on different topics. Failing it on the end would report a blow that
// landed as a blow nobody took — in whichever runs world_events happened to be
// the faster of the two.
//
// Here the end is deliberately the faster one, in every run.
func TestTheBlowThatEndedTheFightIsStillAnswered(t *testing.T) {
	bus := newBus(t)
	h := knowing(t, bus, nil)
	opens(t, bus, h, "encounter-1", playerA)
	answerFirstAction(t, bus, func(ctx context.Context, action eventbus.Event) error {
		over := eventbus.Derive(action, gateway.TypeEncounterEnded, contracts.SourceTestkitSwarm,
			map[string]any{
				"encounter": ref("encounter-1", entity.TypeEncounter),
				"reason":    entity.ResolutionNPCDead,
				"rounds":    1,
			}, eventbus.WithAgent(agent()))
		if err := bus.Publish(ctx, over); err != nil {
			return err
		}
		proposal := packageNaming(t, action, playerA)
		if err := bus.Publish(ctx, proposal); err != nil {
			return err
		}
		return bus.Publish(ctx, factFor(t, proposal, playerA))
	})

	if err := h.Attack(t.Context(), playerA, npcID); err != nil {
		t.Fatalf("the blow that ended the fight was not answered: %v", err)
	}
	if version, known := h.Version(playerA); !known || version != answeredVersion {
		t.Errorf("the harness thinks %s is at version %d, the fact said %d",
			playerA, version, answeredVersion)
	}
}

// TestARepeatedStepStopsWhenTheFightDoes is what a script does with that. The
// step is taken while the encounter is on; the turn that finds it closed is
// not a failure of the script but the end of the fight, and the run goes on.
func TestARepeatedStepStopsWhenTheFightDoes(t *testing.T) {
	bus := newBus(t)
	h := knowing(t, bus, nil)
	opens(t, bus, h, "encounter-1", playerA)
	blows := 0
	answerEveryAction(t, bus, func(ctx context.Context, action eventbus.Event) error {
		blows++
		if blows > 1 {
			// The second blow finds a fight that is over — and finds it only
			// after it was struck, which is the whole point of the case.
			return bus.Publish(ctx, ended(t, "encounter-1", entity.ResolutionNPCDead))
		}
		proposal := packageNaming(t, action, playerA)
		if err := bus.Publish(ctx, proposal); err != nil {
			return err
		}
		return bus.Publish(ctx, factFor(t, proposal, playerA))
	})

	taken, err := h.Run(t.Context(), []gateway.Step{
		{Action: gateway.ActionAttack, Player: playerA, Target: npcID,
			When: gateway.InFight, Turns: 4},
		{Action: gateway.ActionLook, Player: playerA},
	})

	if err != nil {
		t.Fatalf("a repeated step that outlived its fight failed the run: %v", err)
	}
	if struck := turns(taken, gateway.ActionAttack); struck != 1 {
		t.Errorf("%d blows were counted as taken, want the one that was answered", struck)
	}
	if turns(taken, gateway.ActionLook) != 1 {
		t.Error("the run stopped at the end of the fight instead of going on")
	}
}

// TestTheHarnessSwingsNeitherWithNorAtACorpse is the rest of the four cases the
// encounter stub answers with silence (Cr-2 of T-219): a character that is past
// acting, and a target that is past being one. Both are known from the facts
// the harness already reads, and both cost a line instead of a timeout.
func TestTheHarnessSwingsNeitherWithNorAtACorpse(t *testing.T) {
	cases := map[string]struct {
		dead    string
		because string
	}{
		"with a dead character": {playerA, playerA + " is dead and takes no turn"},
		"at a dead target":      {npcID, npcID + " is dead and is nobody's target any more"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			bus := newBus(t)
			h := knowing(t, bus, nil)
			falls(t, bus, h, tc.dead)

			err := h.Attack(t.Context(), playerA, npcID)

			if err == nil {
				t.Fatal("the harness swung a fight that has nobody left in it")
			}
			if !errors.Is(err, gateway.ErrFightOver) {
				t.Errorf("the error is %q; it should be ErrFightOver and not a timeout", err)
			}
			if !strings.Contains(err.Error(), tc.because) {
				t.Errorf("the error is %q, want it to say %q", err, tc.because)
			}
		})
	}
}

// --- an encounter, small enough to live in a test ---

// The wounds the test encounter deals. They are numbers of this file and of
// nothing else: what a blow really costs is decided by mechanics.Resolve,
// which is not what these tests are about. One point a turn is what keeps the
// fixture wolf (10 hp) standing longer than a script may swing, so that a
// scenario which runs out of blows is a case these tests cover too.
const (
	encounterBite = -1
	encounterBlow = -1
)

// encounterMode is what the test encounter does to a fight beyond answering
// it: how it pins the version of what it changes, and whom it kills.
type encounterMode int

const (
	// encounterFair pins the version State published last and kills nobody:
	// the fight outlives the script and is closed by the flight at its end.
	encounterFair encounterMode = iota
	// encounterStale pins a version the world has already left, so that State
	// refuses the whole package.
	encounterStale
	// encounterDeadlyToTheNPC kills the wolf with the first blow and closes
	// the encounter, which is what the rest of a fight has to survive.
	encounterDeadlyToTheNPC
	// encounterDeadlyToThePlayer kills the character instead, which is what
	// the rest of a script has to survive.
	encounterDeadlyToThePlayer
)

// encounter is the smallest thing that answers a blow: it opens a fight when a
// character walks in, publishes a roll and then the one atomic change the
// fight implies, and closes the fight when there is nothing left to fight —
// the shape T-219 gives FakeEncounter ("dice.rolled ... then
// entity.update.proposed atomic with expected_version") over the lifecycle
// C-05 gives an encounter (encounter.started, encounter.ended).
//
// It lives in this test and not in shared/testkit because a second stub of
// combat beside FakeEncounter is the one thing T-018 refused to build, and
// because what it proves is a property of the harness: the harness works with
// anything that answers an action with one proposal, waits until that proposal
// is a fact, and stops swinging the moment the fight is closed under it.
//
// The roll in the middle is not decoration. It puts two hops of the bus
// between the action and the change, so that a harness returning any earlier
// than the fact is seen doing it.
type encounter struct {
	bus  *membus.Bus
	fake *state.FakeState
	mode encounterMode

	mu    sync.Mutex
	seq   int
	plan  map[string]*swing
	fight map[string]string
}

// swing is what the encounter decided to do about one action, kept between the
// roll and the change the roll is supposed to have caused.
type swing struct {
	purpose string
	actor   string
	wounded []string
	// killed is whom the change is going to leave a corpse, or empty.
	killed string
}

func startEncounter(t *testing.T, bus *membus.Bus, fake *state.FakeState, mode encounterMode) {
	t.Helper()
	e := &encounter{
		bus: bus, fake: fake, mode: mode,
		plan: map[string]*swing{}, fight: map[string]string{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	var alive sync.WaitGroup
	for _, topic := range []string{eventbus.TopicPlayerEvents, eventbus.TopicGameEvents} {
		alive.Add(1)
		go func() {
			defer alive.Done()
			_ = bus.Subscribe(ctx, topic, "test-encounter-"+topic, e.observe)
		}()
	}
	t.Cleanup(func() { cancel(); alive.Wait() })
}

func (e *encounter) observe(ctx context.Context, ev eventbus.Event) error {
	actor, _ := ev.Path().GetString("entity.entity.id")
	switch ev.Type {
	case gateway.TypeEnteredRegion:
		return e.open(ctx, ev, actor)
	case gateway.TypeAttacked:
		target, _ := ev.Path().GetString("target.entity.id")
		return e.roll(ctx, ev, &swing{
			purpose: "hit", actor: actor, wounded: []string{actor, target},
			killed: e.doomed(actor, target),
		})
	case gateway.TypeFleeAttempted:
		return e.roll(ctx, ev, &swing{purpose: "flee", actor: actor, wounded: []string{actor}})
	case typeDiceRolled:
		return e.propose(ctx, ev)
	default:
		return nil
	}
}

// doomed is who the next blow of this encounter kills, if the mode says one
// dies at all.
func (e *encounter) doomed(actor, target string) string {
	switch e.mode {
	case encounterDeadlyToTheNPC:
		return target
	case encounterDeadlyToThePlayer:
		return actor
	default:
		return ""
	}
}

const typeDiceRolled = "dice.rolled"

// open starts a fight around whoever walked into the region and says so on
// world_events, which is where the harness learns there is one.
func (e *encounter) open(ctx context.Context, ev eventbus.Event, playerID string) error {
	region, _ := ev.Path().GetString("target.entity.id")
	if playerID == "" || region == "" {
		return nil
	}
	e.mu.Lock()
	e.seq++
	id := "encounter-" + strconv.Itoa(e.seq)
	e.fight[playerID] = id
	e.mu.Unlock()
	return e.publish(ctx, ev, gateway.TypeEncounterStarted, playerID, map[string]any{
		"encounter":    ref(id, entity.TypeEncounter),
		"region":       ref(region, entity.TypeRegion),
		"participants": []map[string]any{ref(playerID, entity.TypePlayer)},
		"npcs":         []map[string]any{ref(npcID, entity.TypeNPC)},
	})
}

// roll publishes the die the decision will rest on and remembers what the
// decision is going to do.
func (e *encounter) roll(ctx context.Context, cause eventbus.Event, plan *swing) error {
	e.mu.Lock()
	e.plan[cause.CorrelationID()] = plan
	e.mu.Unlock()
	die := eventbus.Derive(cause, typeDiceRolled, contracts.SourceTestkitSwarm, map[string]any{
		"roll": map[string]any{
			"index": 0, "formula": "d20", "seed": "1", "result": 15, "natural": 15,
		},
		"purpose": plan.purpose,
		"roller": map[string]any{
			"entity": map[string]any{"id": plan.actor, "type": entity.TypePlayer},
		},
	}, eventbus.WithAgent(eventbus.AgentRef{
		ID: "test-encounter", Level: "task", Blueprint: "encounter-wolf",
	}))
	return e.bus.Publish(ctx, die)
}

// propose publishes the one atomic change the fight decided on, pinning the
// version of every entity it touches (C-02, ADR-013 p. 1), and then closes the
// fight if that change ended it — in that order and under one correlation
// identifier, which is the order C-05 gives the two.
func (e *encounter) propose(ctx context.Context, die eventbus.Event) error {
	e.mu.Lock()
	plan, planned := e.plan[die.CorrelationID()]
	if planned {
		delete(e.plan, die.CorrelationID())
		e.seq++
	}
	seq := e.seq
	e.mu.Unlock()
	if !planned {
		return nil
	}

	changes := make([]entity.ChangeSet, 0, len(plan.wounded))
	for _, id := range slices.Sorted(slices.Values(plan.wounded)) {
		current, ok := e.fake.Get(id)
		if !ok {
			return fmt.Errorf("test encounter: %s is not in the world", id)
		}
		version := current.Version
		if e.mode == encounterStale {
			version--
		}
		changes = append(changes, entity.ChangeSet{
			Entity:          eventbus.Entity{Entity: current.Ref().EventRef(), Name: current.Name},
			ExpectedVersion: &version,
			Ops:             e.ops(current, id == plan.killed),
		})
	}
	if err := e.bus.Publish(ctx, eventbus.Derive(die, gateway.TypeUpdateProposed,
		contracts.SourceTestkitSwarm, map[string]any{
			"proposal_id": "enc-" + strconv.Itoa(seq),
			"changes":     changes,
			"atomic":      true,
			"cause":       "combat",
		})); err != nil {
		return err
	}
	return e.close(ctx, die, plan)
}

// ops is what one blow does to one fighter: a scratch, or everything a death
// is made of.
func (e *encounter) ops(who *entity.Entity, fatal bool) []entity.Op {
	if fatal {
		hp, _ := who.HP()
		return []entity.Op{
			{Op: entity.OpInc, Path: entity.AttrHP, Value: -hp},
			{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusDead},
		}
	}
	by := encounterBite
	if who.Type == entity.TypeNPC {
		by = encounterBlow
	}
	return []entity.Op{{Op: entity.OpInc, Path: entity.AttrHP, Value: by}}
}

// close ends the fight when the last action ended it: somebody fell, or the
// character got away.
func (e *encounter) close(ctx context.Context, cause eventbus.Event, plan *swing) error {
	reason := ""
	switch {
	case plan.killed != "" && plan.killed != plan.actor:
		reason = entity.ResolutionNPCDead
	case plan.killed != "" || plan.purpose == "flee":
		reason = entity.ResolutionPlayersOut
	default:
		return nil
	}
	e.mu.Lock()
	id, fighting := e.fight[plan.actor]
	delete(e.fight, plan.actor)
	e.mu.Unlock()
	if !fighting {
		return nil
	}
	return e.publish(ctx, cause, gateway.TypeEncounterEnded, plan.actor, map[string]any{
		"encounter": ref(id, entity.TypeEncounter),
		"reason":    reason,
		"rounds":    1,
	})
}

// publish sends one event of the encounter. meta.agent travels with everything
// on world_events, which is what the policy of the topic requires of a fight
// (C-01) and what contracts.md §0 requires of a stub.
func (e *encounter) publish(ctx context.Context, cause eventbus.Event, typ, playerID string,
	payload map[string]any) error {
	return e.bus.Publish(ctx, eventbus.Derive(cause, typ, contracts.SourceTestkitSwarm, payload,
		eventbus.WithAgent(eventbus.AgentRef{
			ID: "test-encounter:" + playerID, Level: "domain", Blueprint: "encounter-wolf",
		})))
}

// ref is the EntityWithName shape the payloads of C-05 name an entity by.
func ref(id, typ string) map[string]any {
	return map[string]any{"entity": map[string]any{"id": id, "type": typ}}
}
