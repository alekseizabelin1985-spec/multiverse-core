package swarm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	tkmech "multiverse-core.io/shared/testkit/mechanics"
	"multiverse-core.io/shared/testkit/membus"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

const (
	wolfID   = "wolf-alpha"
	wolfName = "Альфа-волк"
	regName  = "Тёмный лес"
)

// --- opening an encounter ---

// TestEnteringARegionWithAWolfOpensAnEncounter is the first half of the stub:
// a step into the forest is answered with the proposal that creates the
// encounter and with the event that opens it (design §4.1).
func TestEnteringARegionWithAWolfOpensAnEncounter(t *testing.T) {
	enc, bus := fightStub(t)

	act(t, enc, entered(playerA, nameA, regionID, regName))

	create := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed)
	pa := create.Path()
	if kind, _ := pa.GetString("entity.entity.type"); kind != entity.TypeEncounter {
		t.Errorf("the proposal creates a %q, want an %s", kind, entity.TypeEncounter)
	}
	if cause, _ := pa.GetString("cause"); cause != swarm.CauseSpawn {
		t.Errorf("cause %q, want %q", cause, swarm.CauseSpawn)
	}
	if region, _ := pa.GetString("attributes.region_id"); region != regionID {
		t.Errorf("the encounter stands in %q, want %q", region, regionID)
	}
	if open, _ := pa.GetString("attributes.state"); open != entity.EncounterStateActive {
		t.Errorf("state %q, want %q", open, entity.EncounterStateActive)
	}
	if who, _ := pa.GetString("attributes.participants[0].player_id"); who != playerA {
		t.Errorf("participant %q, want %q", who, playerA)
	}
	if npc, _ := pa.GetString("attributes.npcs[0].npc_id"); npc != wolfID {
		t.Errorf("npc %q, want %q", npc, wolfID)
	}

	started := onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterStarted)
	sp := started.Path()
	if id, _ := sp.GetString("encounter.entity.id"); id != encounterOf(t, create) {
		t.Errorf("encounter.started is about %q, the proposal creates %q",
			id, encounterOf(t, create))
	}
	if opened, _ := pa.GetString("attributes.opened_by_event_id"); opened != started.ID {
		t.Errorf("opened_by_event_id %q, want the id of encounter.started %q", opened, started.ID)
	}
	if npc, _ := sp.GetString("npcs[0].name"); npc != wolfName {
		t.Errorf("the wolf is called %q, want %q", npc, wolfName)
	}
	if region, _ := sp.GetString("region.name"); region != regName {
		t.Errorf("the region is called %q, want %q", region, regName)
	}
}

// TestTheRoundOfTheEncounterComesFromTheRulesFile ties the numbers C-05 v1.1
// puts into encounter.started to rules/dark-forest.yaml, where the balance of
// the world lives: a change of the round is a change of the file, and this test
// is what notices it (FR-020).
func TestTheRoundOfTheEncounterComesFromTheRulesFile(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))

	started := onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterStarted)
	round := mechanics(t).Rules().Document().Round
	if timeout, _ := started.Path().GetString("round.timeout"); timeout != round.Timeout {
		t.Errorf("round timeout %q, the rules say %q", timeout, round.Timeout)
	}
	if missed, _ := started.Path().GetInt("round.idle_after_missed"); missed != round.IdleAfterMissed {
		t.Errorf("idle_after_missed %d, the rules say %d", missed, round.IdleAfterMissed)
	}
}

// TestNothingOpensWhereNothingIsAlive: an encounter needs somebody to fight.
func TestNothingOpensWhereNothingIsAlive(t *testing.T) {
	enc, bus := fightStubWith(t, dead(wolfID))

	act(t, enc, entered(playerA, nameA, regionID, regName))

	if got := eventsOf(t, bus, eventbus.TopicWorldEvents); len(got) != 0 {
		t.Errorf("%d events about a fight that has nobody in it", len(got))
	}
	if _, open := enc.ActiveEncounter(playerA); open {
		t.Error("an encounter was opened against a dead wolf")
	}
}

// TestASecondEntryDoesNotOpenASecondEncounter: one character, one fight.
func TestASecondEntryDoesNotOpenASecondEncounter(t *testing.T) {
	enc, bus := fightStub(t)

	act(t, enc, entered(playerA, nameA, regionID, regName))
	act(t, enc, entered(playerA, nameA, regionID, regName))

	if got := ofType(eventsOf(t, bus, eventbus.TopicWorldEvents), swarm.TypeEncounterStarted); len(got) != 1 {
		t.Errorf("%d encounters opened, want exactly one", len(got))
	}
}

// --- one exchange ---

// TestAnExchangeIsFourDiceAndTwoDecisions is the shape design §4.1 promises for
// a turn: the character swings and the wolf answers, each with a roll to hit
// and a roll for damage, and the two decisions travel with one atomic proposal
// carrying the version each entity was expected to be at.
func TestAnExchangeIsFourDiceAndTwoDecisions(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	blow := attackWhere(t, func(id string) bool {
		// A hit that cannot kill (a wolf has 10 hit points and d6 cannot reach
		// them), answered by a hit: the exchange both sides land.
		return tkmech.Verdict(id, 0) == tkmech.VerdictHit &&
			hits(tkmech.Verdict(id, 2))
	})

	act(t, enc, blow)

	dice := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeDiceRolled)
	if len(dice) != 4 {
		t.Fatalf("%d dice.rolled, an exchange both sides land is four", len(dice))
	}
	purposes := []string{"hit", "damage", "npc_hit", "npc_damage"}
	for i, ev := range dice {
		if got, _ := ev.Path().GetInt("roll.index"); got != i {
			t.Errorf("roll %d carries index %d", i, got)
		}
		if got, _ := ev.Path().GetString("purpose"); got != purposes[i] {
			t.Errorf("roll %d is a %q, want %q", i, got, purposes[i])
		}
	}

	decided := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided)
	if len(decided) != 2 {
		t.Fatalf("%d combat.decided, an exchange is two", len(decided))
	}
	if action, _ := decided[0].Path().GetString("action"); action != "attack" {
		t.Errorf("the first decision is a %q, want an attack", action)
	}
	if action, _ := decided[1].Path().GetString("action"); action != "npc_attack" {
		t.Errorf("the second decision is a %q, want an npc_attack", action)
	}
	if version, _ := decided[0].Path().GetString("rules_version"); version != mechanics(t).Rules().Version {
		t.Errorf("rules_version %q, the rules say %q", version, mechanics(t).Rules().Version)
	}
	if mode, _ := decided[0].Path().GetString("phase1_mode"); mode != swarm.Phase1Mode {
		t.Errorf("phase1_mode %q, want %q", mode, swarm.Phase1Mode)
	}
	// The rolls a decision rests on are named by the events that published
	// them, and they were published first (C-03).
	if id, _ := decided[0].Path().GetString("rolls[0].event.id"); id != dice[0].ID {
		t.Errorf("the decision rests on roll %q, the first roll is %q", id, dice[0].ID)
	}

	proposal := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	if atomic, _ := proposal.Path().GetBool("atomic"); !atomic {
		t.Error("the package of one exchange is not atomic")
	}
	if cause, _ := proposal.Path().GetString("cause"); cause != swarm.CauseCombat {
		t.Errorf("cause %q, want %q", cause, swarm.CauseCombat)
	}
	sets := changeSets(t, proposal)
	if len(sets) != 3 {
		t.Fatalf("%d change sets, an exchange both sides land changes two fighters "+
			"and the encounter that recorded the round", len(sets))
	}
	for _, set := range sets {
		if set.ExpectedVersion == nil {
			t.Errorf("%s is changed without expected_version; hp and status require one", set.Ref())
		} else if *set.ExpectedVersion != 1 {
			t.Errorf("%s is expected at version %d, the fixtures are at 1",
				set.Ref(), *set.ExpectedVersion)
		}
	}
}

// TestOneEntityOneChangeSet is C-02 v1.3 seen from the publishing side: State
// refuses a package that names an entity twice, so the operations of one
// fighter are merged before the package leaves.
func TestOneEntityOneChangeSet(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	act(t, enc, attackWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) }))

	for _, proposal := range ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed) {
		seen := make(map[string]bool)
		for _, set := range changeSets(t, proposal) {
			if seen[set.Ref().ID] {
				t.Errorf("%s is named twice in one package", set.Ref())
			}
			seen[set.Ref().ID] = true
		}
	}
}

// TestTheSecondExchangeProposesAgainstTheVersionTheFirstOneLeft is what lets a
// fight go on faster than the facts come back: the stub folds its own proposal
// into its view (C-02 expected_version, ADR-013 p. 1), so the next package
// names the version the previous one moved the entity to rather than the one it
// was seeded at.
func TestTheSecondExchangeProposesAgainstTheVersionTheFirstOneLeft(t *testing.T) {
	// A wolf too big to fall in two blows, so that both exchanges reach a
	// proposal about the same living entity.
	enc, bus := fightStubWith(t, attr(wolfID, entity.AttrHP, 40), attr(wolfID, entity.AttrHPMax, 40))
	act(t, enc, entered(playerA, nameA, regionID, regName))

	lands := func(id string) bool { return hits(tkmech.Verdict(id, 0)) }
	act(t, enc, attackWhere(t, lands))
	act(t, enc, attackWhere(t, lands))

	proposals := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed)
	if len(proposals) != 2 {
		t.Fatalf("%d proposals for two landed blows", len(proposals))
	}
	for i, proposal := range proposals {
		for _, set := range changeSets(t, proposal) {
			if set.Ref().ID != wolfID {
				continue
			}
			if set.ExpectedVersion == nil {
				t.Fatalf("proposal %d changes the wolf without expected_version", i+1)
			}
			if want := int64(i + 1); *set.ExpectedVersion != want {
				t.Errorf("proposal %d expects the wolf at version %d, want %d",
					i+1, *set.ExpectedVersion, want)
			}
		}
	}
}

// TestAWolfKilledElsewhereIsNotAttackedAgain: an encounter is open, and the
// wolf in it dies of something the stub did not decide — another agent, another
// player, a proposal of the world. The next swing finds no target and is
// answered with nothing rather than resolved against a corpse (inv-01).
func TestAWolfKilledElsewhereIsNotAttackedAgain(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	before := len(allEvents(t, bus))

	if err := enc.Observe(t.Context(), died(wolfID)); err != nil {
		t.Fatalf("observe the death: %v", err)
	}
	act(t, enc, attack(playerA, nameA, wolfID))
	// The same swing with no target named, which is answered by picking the
	// first NPC of the encounter still standing — and there is none.
	act(t, enc, attackAnything(playerA, nameA))

	if after := len(allEvents(t, bus)); after != before {
		t.Errorf("%d events were published for a swing at a corpse", after-before)
	}
}

// TestTheFactsOfStateMoveWhatTheStubProposesAgainst: the world of the stub is
// the world of State, and a change the stub did not make is still a change it
// has to propose against — the version and the hit points both.
func TestTheFactsOfStateMoveWhatTheStubProposesAgainst(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))

	if err := enc.Observe(t.Context(), hurt(wolfID, 4, 10, 8)); err != nil {
		t.Fatalf("observe: %v", err)
	}
	act(t, enc, attackWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) }))

	decided := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided)
	if len(decided) == 0 {
		t.Fatal("the blow was not decided")
	}
	if before, _ := decided[0].Path().GetInt("hp.defender_before"); before != 8 {
		t.Errorf("the wolf is struck at %d hit points, the fact left it at 8", before)
	}
	proposal := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	for _, set := range changeSets(t, proposal) {
		if set.Ref().ID != wolfID {
			continue
		}
		if set.ExpectedVersion == nil || *set.ExpectedVersion != 4 {
			t.Errorf("the wolf is proposed at %v, the fact left it at version 4", set.ExpectedVersion)
		}
	}
}

// TestAnEntryWithoutARegionIsPassedOver: Act is exported, and a caller that
// drives it by hand may hand it a payload the schema would have refused. It is
// answered with nothing rather than with a fight in a region called "".
func TestAnEntryWithoutARegionIsPassedOver(t *testing.T) {
	enc, bus := fightStub(t)

	act(t, enc, playerAction(swarm.TypeEnteredRegion, map[string]any{"entity": actor(playerA, nameA)}))

	if got := allEvents(t, bus); len(got) != 0 {
		t.Errorf("%d events for an entry that names no region", len(got))
	}
}

// TestAMissStillCostsTheRound is the decision of 2026-09-11 seen from the one
// exchange that wounds nobody: an encounter answers every action it takes with
// exactly one atomic package, because a miss spends the round even when it
// moves no hit points — and a round nobody wrote down can be closed neither by
// "everybody acted" nor by the threshold of missed turns.
//
// What carries the package is the encounter entity, which §4.6 gives to the
// task level on any path with cause=combat. It replaces the test this file used
// to hold, TestAMissChangesNothing, which asserted the opposite and so kept the
// defect alive.
func TestAMissStillCostsTheRound(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	encounterID := encounterOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed))

	act(t, enc, attackWhere(t, func(id string) bool {
		return !hits(tkmech.Verdict(id, 0)) && !hits(tkmech.Verdict(id, 2))
	}))

	proposal := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	sets := changeSets(t, proposal)
	if len(sets) != 1 {
		t.Fatalf("%d change sets for an exchange nobody landed, want the encounter alone", len(sets))
	}
	if sets[0].Ref().ID != encounterID {
		t.Fatalf("the package is about %s, want the encounter %s", sets[0].Ref(), encounterID)
	}
	if round, ok := numericOp(sets[0].Ops, entity.AttrRoundSeq); !ok || round != 2 {
		t.Errorf("the package moves round_seq to %v (found: %v), want 2: the miss spent round 1",
			round, sets[0].Ops)
	}
	// And nothing else: participation did not move, and a set that changes
	// nothing would make State publish a fact with an empty changed[] (§4.6).
	if len(sets[0].Ops) != 1 {
		t.Errorf("the package of a miss carries %v; the round is all it moved", sets[0].Ops)
	}
	if cause, _ := proposal.Path().GetString("cause"); cause != swarm.CauseCombat {
		t.Errorf("cause %q, want %q", cause, swarm.CauseCombat)
	}
	if got := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided); len(got) != 2 {
		t.Errorf("%d decisions, a missed exchange is still two", len(got))
	}
}

// TestEveryActionTheEncounterTakesIsAnsweredWithOnePackage is the same property
// over a whole fight rather than over one hand-picked exchange: hits, misses
// and the blow that ends it all count one package each. The harness of T-400
// waits for exactly this and gives up on an action that goes unanswered.
func TestEveryActionTheEncounterTakesIsAnsweredWithOnePackage(t *testing.T) {
	// A wolf too big to fall quickly, so that the run reaches exchanges of
	// every kind before somebody goes down.
	enc, bus := fightStubWith(t, attr(wolfID, entity.AttrHP, 40), attr(wolfID, entity.AttrHPMax, 40))
	act(t, enc, entered(playerA, nameA, regionID, regName))

	actions := 0
	for i := 0; i < 20; i++ {
		if _, open := enc.ActiveEncounter(playerA); !open {
			break
		}
		act(t, enc, attack(playerA, nameA, wolfID))
		actions++
		got := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed)
		if len(got) != actions {
			t.Fatalf("%d actions and %d packages: an encounter answers every action "+
				"it takes with exactly one", actions, len(got))
		}
	}
	if actions < 3 {
		t.Fatalf("the fight was over after %d exchanges: the run proves nothing", actions)
	}
}

// TestThePackageKeepsTheOrderTheFightersWereTouchedIn pins the promise of
// changes.sets: the sets are in the order the entities entered the package —
// the wolf that was struck, the character struck back, and the encounter that
// recorded the round. Recorded runs are compared to one another (NFR-061), and
// a package whose sets moved around would read as a different run.
func TestThePackageKeepsTheOrderTheFightersWereTouchedIn(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	encounterID := encounterOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed))

	act(t, enc, attackWhere(t, func(id string) bool {
		return tkmech.Verdict(id, 0) == tkmech.VerdictHit && hits(tkmech.Verdict(id, 2))
	}))

	proposal := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	got := make([]string, 0, 3)
	for _, set := range changeSets(t, proposal) {
		got = append(got, set.Ref().ID)
	}
	want := []string{wolfID, playerA, encounterID}
	if !slices.Equal(got, want) {
		t.Errorf("the package names %v, want %v: the order is the order they were touched", got, want)
	}
}

// TestEveryProposalPassesTheOwnershipTable is what State answers with
// level_violation turned into a guard the stub cannot walk past. §4.6 decides
// what a proposer of a level may change; the table is read from
// contracts.OwnershipRules() rather than copied into this test, so a row that
// moves there moves here.
//
// Two runs are needed to see everything the stub proposes: a fight to the death
// (the encounter it creates, the wounds of the wolf, the trophy of the winner,
// the record of the round) and a flight (the position of an escape).
func TestEveryProposalPassesTheOwnershipTable(t *testing.T) {
	fight, fightBus := fightStub(t)
	act(t, fight, entered(playerA, nameA, regionID, regName))
	swingUntilOver(t, fight, fightBus)

	flight, flightBus := fightStub(t)
	act(t, flight, entered(playerA, nameA, regionID, regName))
	act(t, flight, fleeWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) }))

	kinds := make(map[string]bool)
	for _, bus := range []*membus.Bus{fightBus, flightBus} {
		for _, ev := range eventsOf(t, bus, eventbus.TopicSystemEvents) {
			if ev.Source != contracts.SourceTestkitSwarm {
				continue
			}
			for _, want := range proposalsOf(t, ev) {
				kinds[want.kind()] = true
				if ownershipAllows(want) {
					t.Logf("OK   %s", want)
					continue
				}
				t.Errorf("DENY %s: no rule of §4.6 lets a %s agent change a %s "+
					"with cause=%s on %v (create=%v)", want,
					want.level, want.entityType, want.cause, want.paths, want.create)
			}
		}
	}
	// The five kinds the stub publishes. A run that stopped producing one of
	// them would leave this test green over less than it claims to cover.
	for _, want := range []string{
		"create encounter cause=spawn level=domain",
		"update npc cause=combat level=task",
		"update player cause=combat level=task",
		"update encounter cause=combat level=task",
		"update player cause=flee level=task",
	} {
		if !kinds[want] {
			t.Errorf("the runs produced no proposal of the kind %q; the check saw %v", want, kinds)
		}
	}
}

// TestAnAttackOutsideAnEncounterIsPassedOver, like a rest, is not an error: the
// gateway decides what a player may do, the stub decides what a fight is.
func TestAnAttackOutsideAnEncounterIsPassedOver(t *testing.T) {
	enc, bus := fightStub(t)

	act(t, enc, attack(playerA, nameA, wolfID))

	if got := allEvents(t, bus); len(got) != 0 {
		t.Errorf("%d events about an attack outside an encounter", len(got))
	}
}

// TestAnActionAfterTheFightIsOverIsAnsweredWithSilence pins the silence as a
// decision rather than an oversight, so that the next author does not turn it
// into a published refusal.
//
// "One action, exactly one package" is about the actions the encounter takes.
// An attack sent after the wolf fell is not one of them: the decision of
// 2026-09-11 calls it an error of the scenario, and the immediate refusal
// belongs to the harness of shared/testkit/gateway, which knows the fight is
// over from encounter.ended. A refusal published from here would need a new
// event type for a stub two epics from removal — what T-018 turned down.
func TestAnActionAfterTheFightIsOverIsAnsweredWithSilence(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	swingUntilOver(t, enc, bus)
	if _, open := enc.ActiveEncounter(playerA); open {
		t.Fatal("the fight is still on; this test is about what comes after it")
	}
	before := len(allEvents(t, bus))

	act(t, enc, attack(playerA, nameA, wolfID))
	act(t, enc, flee(playerA, nameA))

	if after := len(allEvents(t, bus)); after != before {
		t.Errorf("%d events answered an attack and a flight sent after the fight ended; "+
			"the stub answers neither, and the harness is what refuses them", after-before)
	}
}

// --- the three ends ---

// TestTheWolfDies is the first of the three ends the DoD names: the encounter
// closes with npc_dead and names who landed the blow.
func TestTheWolfDies(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))

	swingUntilOver(t, enc, bus)

	ended := onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterEnded)
	if reason, _ := ended.Path().GetString("reason"); reason != entity.ResolutionNPCDead {
		t.Fatalf("the fight ended as %q, want %q", reason, entity.ResolutionNPCDead)
	}
	if killer, _ := ended.Path().GetString("killer.entity.id"); killer != playerA {
		t.Errorf("killer %q, want %q", killer, playerA)
	}
	if rounds, _ := ended.Path().GetInt("rounds"); rounds < 1 {
		t.Errorf("the fight lasted %d rounds", rounds)
	}
	if _, open := enc.ActiveEncounter(playerA); open {
		t.Error("the encounter is still open after the wolf died")
	}

	// The corpse: the status, who killed it and whose the trophy is — the four
	// paths State keeps accepting after death (§4.5 p. 5).
	ops := opsFor(t, bus, wolfID)
	assertOp(t, ops, entity.AttrStatus, entity.StatusDead)
	assertOp(t, ops, entity.AttrKilledBy, playerA)
	assertOp(t, ops, "loot_claimed_by", playerA)
}

// TestTheEncounterEntityRecordsTheFightItHeld is the other half of "one action,
// one package": the encounter entity is a living record and not a row frozen at
// the moment it was created. The blow that ends the fight closes the entity in
// the same package that carries the wound — a second package would be a second
// answer to one action.
func TestTheEncounterEntityRecordsTheFightItHeld(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	encounterID := encounterOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed))
	swingUntilOver(t, enc, bus)

	ended := onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterEnded)
	proposals := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed)
	if len(proposals) == 0 {
		t.Fatal("the fight proposed nothing at all")
	}
	last := changeSets(t, proposals[len(proposals)-1])

	var closing *entity.ChangeSet
	for i, set := range last {
		if set.Ref().ID == encounterID {
			closing = &last[i]
		}
	}
	if closing == nil {
		t.Fatalf("the package of the killing blow says nothing about the encounter: %v", last)
	}
	assertOp(t, closing.Ops, entity.AttrState, entity.EncounterStateResolved)
	assertOp(t, closing.Ops, entity.AttrResolution, entity.ResolutionNPCDead)
	assertOp(t, closing.Ops, entity.AttrClosedByEventID, ended.ID)

	// Participation: nobody is left fighting, and the character who did the
	// killing is recorded as having dealt damage.
	people := participantsOf(t, closing.Ops)
	if len(people) != 1 || people[0].State != entity.ParticipationOutOfCombat {
		t.Errorf("participants of a fight that is over: %v", people)
	}
	if people[0].DamageDealt <= 0 {
		t.Errorf("the character who killed the wolf is recorded as dealing %d damage",
			people[0].DamageDealt)
	}
	if people[0].LastHitAt == nil {
		t.Error("the record of the participant names no last_hit_at")
	}

	// And the entity moved while the fight was on, not only when it ended.
	rounds := 0
	for _, proposal := range proposals {
		for _, set := range changeSets(t, proposal) {
			if set.Ref().ID != encounterID {
				continue
			}
			if _, ok := numericOp(set.Ops, entity.AttrRoundSeq); ok {
				rounds++
			}
		}
	}
	if rounds == 0 {
		t.Error("no package moved round_seq: the encounter entity is frozen at the round it opened")
	}
}

// TestTheTrophyIsHandedOutOnce is inv-03. The wolf falls once, so the pelt is
// appended once — and a swing at a corpse produces nothing at all.
func TestTheTrophyIsHandedOutOnce(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	swingUntilOver(t, enc, bus)

	// Two more swings after the fight is over, the way an impatient player
	// would send them.
	act(t, enc, attack(playerA, nameA, wolfID))
	act(t, enc, attack(playerA, nameA, wolfID))

	trophies := 0
	for _, op := range opsFor(t, bus, playerA) {
		if op.Op == entity.OpAppend && op.Path == entity.AttrInventory {
			trophies++
			item := itemOf(t, op)
			if want := mechanics(t).Rules().Loot("wolf"); len(want) == 0 || item.Kind != want[0].Kind {
				t.Errorf("the trophy is a %q, the loot table of a wolf says %v", item.Kind, want)
			}
			if item.Source.Entity.ID != wolfID {
				t.Errorf("the trophy comes from %q, want %q", item.Source.Entity.ID, wolfID)
			}
			if item.Source.EventID == "" {
				t.Error("the trophy names no combat.decided as its source; inv-03 rests on it")
			}
		}
	}
	if trophies != 1 {
		t.Errorf("%d trophies for one wolf, inv-03 says exactly one", trophies)
	}
}

// TestTheCharacterDies is the second end: with nobody left standing the fight
// closes as players_out (C-04, encounter.ended).
func TestTheCharacterDies(t *testing.T) {
	// A character on their last hit point against a wolf too big to fall in
	// one exchange: whatever the table rolls, the fight can only end one way.
	enc, bus := fightStubWith(t, attr(playerA, entity.AttrHP, 1),
		attr(wolfID, entity.AttrHP, 40), attr(wolfID, entity.AttrHPMax, 40))
	act(t, enc, entered(playerA, nameA, regionID, regName))

	swingUntilOver(t, enc, bus)

	ended := onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterEnded)
	if reason, _ := ended.Path().GetString("reason"); reason != entity.ResolutionPlayersOut {
		t.Fatalf("the fight ended as %q, want %q", reason, entity.ResolutionPlayersOut)
	}
	if _, named := ended.Payload["killer"]; named {
		// killer is for a character who landed the blow; a wolf is not one.
		t.Error("encounter.ended names a killer for a character who fell")
	}
	ops := opsFor(t, bus, playerA)
	assertOp(t, ops, entity.AttrStatus, entity.StatusDead)
	assertOp(t, ops, entity.AttrKilledBy, wolfID)
}

// TestAFailedFlightIsAnsweredWithAFreeAttack is the third end of the DoD, and
// the rule flee.on_fail of rules/dark-forest.yaml: walking away and failing
// costs a strike out of turn.
func TestAFailedFlightIsAnsweredWithAFreeAttack(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))

	act(t, enc, fleeWhere(t, func(id string) bool {
		return !hits(tkmech.Verdict(id, 0)) && hits(tkmech.Verdict(id, 1))
	}))

	decided := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided)
	if len(decided) != 2 {
		t.Fatalf("%d decisions, a failed flight answered by a strike is two", len(decided))
	}
	flight := decided[0].Path()
	if action, _ := flight.GetString("action"); action != "flee" {
		t.Errorf("the first decision is a %q, want a flee", action)
	}
	if success, _ := flight.GetBool("outcome.success"); success {
		t.Error("the flight succeeded; this test is about the one that does not")
	}
	if threshold, _ := flight.GetInt("outcome.threshold"); threshold == 0 {
		t.Error("the flight reports no threshold; a player cannot see what the escape was worth")
	}
	if enemies, _ := flight.GetInt("outcome.living_enemies"); enemies != 1 {
		t.Errorf("living_enemies %d, one wolf is standing", enemies)
	}
	if _, named := decided[0].Payload["defender"]; named {
		t.Error("a flight resolves against a threshold and has no defender")
	}
	if _, named := decided[0].Payload["hp"]; named {
		t.Error("a flight changes nobody's hit points and carries no hp block")
	}

	free := decided[1].Path()
	if action, _ := free.GetString("action"); action != "free_attack" {
		t.Errorf("the second decision is a %q, want a free_attack", action)
	}
	if marked, _ := free.GetBool("free_attack"); !marked {
		t.Error("the strike out of turn is not marked as one")
	}
	if attacker, _ := free.GetString("attacker.entity.id"); attacker != wolfID {
		t.Errorf("the free attack comes from %q, want the wolf", attacker)
	}
	if _, open := enc.ActiveEncounter(playerA); !open {
		t.Error("a failed flight ended the encounter; it costs a strike, not the fight")
	}
}

// TestASuccessfulFlightEndsTheEncounter: the character walks out of the world
// to where the rules put them (flee.success_position, DR-19).
func TestASuccessfulFlightEndsTheEncounter(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))

	act(t, enc, fleeWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) }))

	if got := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided); len(got) != 1 {
		t.Errorf("%d decisions, an escape is one", len(got))
	}
	proposal := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	if cause, _ := proposal.Path().GetString("cause"); cause != swarm.CauseFlee {
		t.Errorf("cause %q, want %q", cause, swarm.CauseFlee)
	}
	want := mechanics(t).Rules().FleePosition(worldID, regionID)
	assertOp(t, opsFor(t, bus, playerA), entity.AttrPosition, want)

	ended := onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterEnded)
	if reason, _ := ended.Path().GetString("reason"); reason != entity.ResolutionPlayersOut {
		t.Errorf("the fight ended as %q, want %q", reason, entity.ResolutionPlayersOut)
	}
}

// --- what the stub does not answer ---

// TestARestIsNotTheBusinessOfTheEncounter: outside a fight the rest is proposed
// by the gateway (C-02 v1.1), and the stub has nothing to add to it.
func TestARestIsNotTheBusinessOfTheEncounter(t *testing.T) {
	enc, bus := fightStub(t)

	act(t, enc, rest(playerA, nameA))
	act(t, enc, entered(playerA, nameA, regionID, regName))
	before := len(allEvents(t, bus))
	act(t, enc, rest(playerA, nameA))

	if after := len(allEvents(t, bus)); after != before {
		t.Errorf("%d events were published for a rest", after-before)
	}
}

// TestAnActionOfAnotherWorldIsPassedOver: an encounter agent serves one world.
func TestAnActionOfAnotherWorldIsPassedOver(t *testing.T) {
	enc, bus := fightStub(t)

	ev := entered(playerA, nameA, regionID, regName)
	ev.World = &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: "another-world", Type: "world"}}
	act(t, enc, ev)

	if got := allEvents(t, bus); len(got) != 0 {
		t.Errorf("%d events about a world the stub does not serve", len(got))
	}
}

// TestARedeliveredActionIsAnsweredOnce: delivery is at-least-once (C-01), and a
// second copy of one swing must not roll a second set of dice.
func TestARedeliveredActionIsAnsweredOnce(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	blow := attackWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) })

	act(t, enc, blow)
	before := len(allEvents(t, bus))
	act(t, enc, blow)

	if after := len(allEvents(t, bus)); after != before {
		t.Errorf("a redelivered action produced %d more events", after-before)
	}
}

// --- through the bus ---

// TestTheStubAnswersThroughItsSubscriptions is the wiring: everything above
// drives the handler directly, and this one proves the handler is the one the
// subscription calls — and that everything it publishes survives the validation
// the bus does on read (MV_BUS_VALIDATE_ON_READ, SEC-16).
func TestTheStubAnswersThroughItsSubscriptions(t *testing.T) {
	enc, bus := fightStub(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := enc.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := bus.Publish(ctx, entered(playerA, nameA, regionID, regName)); err != nil {
		t.Fatalf("publish the entry: %v", err)
	}
	waitFor(t, "the encounter to open", func() bool {
		_, open := enc.ActiveEncounter(playerA)
		return open
	})

	dead, err := bus.DeadLetters()
	if err != nil {
		t.Fatalf("read dead letters: %v", err)
	}
	if len(dead) != 0 {
		t.Fatalf("%d events could not be read back: %s", len(dead), dead[0].Error)
	}
}

// TestTheFactsOfStateMoveTheVersionTheStubProposesAgainst closes the loop the
// fight depends on: FakeState applies what the stub proposed and publishes the
// facts, and the next proposal is made against the version those facts left the
// world at. Without it the second swing of a fight would be a version_conflict.
func TestTheFactsOfStateMoveTheVersionTheStubProposesAgainst(t *testing.T) {
	enc, bus := fightStub(t)
	world, err := state.New(state.Config{Bus: bus, WorldID: worldID})
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	fixtures, err := state.LoadFixtures(fixturesDir(t))
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	if err := world.Seed(fixtures); err != nil {
		t.Fatalf("seed: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := world.Start(ctx); err != nil {
		t.Fatalf("start state: %v", err)
	}
	if err := enc.Start(ctx); err != nil {
		t.Fatalf("start the stub: %v", err)
	}

	if err := bus.Publish(ctx, entered(playerA, nameA, regionID, regName)); err != nil {
		t.Fatalf("publish the entry: %v", err)
	}
	waitFor(t, "the encounter to open", func() bool {
		_, open := enc.ActiveEncounter(playerA)
		return open
	})
	for i := 0; i < 12; i++ {
		if _, open := enc.ActiveEncounter(playerA); !open {
			break
		}
		before := len(ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided))
		if err := bus.Publish(ctx, attack(playerA, nameA, wolfID)); err != nil {
			t.Fatalf("publish an attack: %v", err)
		}
		// The exchange is over when it has been decided and when State has
		// answered every proposal made so far: that is the moment the version
		// the next swing proposes against is settled.
		waitFor(t, "the exchange to be decided and applied", func() bool {
			decided := len(ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided))
			proposed := len(ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed))
			return decided > before && len(world.AppliedProposals()) >= proposed
		})
	}

	for _, ev := range eventsOf(t, bus, eventbus.TopicSystemEvents) {
		if ev.Type == swarm.TypeRejected {
			reason, _ := ev.Path().GetString("reason")
			t.Fatalf("State refused a proposal of the stub: %s", reason)
		}
	}
	if _, open := enc.ActiveEncounter(playerA); open {
		t.Error("twelve exchanges and the fight is still going")
	}
	npc, ok := world.Get(wolfID)
	if !ok {
		t.Fatal("the wolf left the world of State")
	}
	if npc.Version < 2 {
		t.Errorf("the wolf of State is still at version %d: no proposal of the stub reached it",
			npc.Version)
	}
	hp, _ := npc.HP()
	hpMax, _ := npc.HPMax()
	if hp == hpMax {
		t.Error("the wolf came out of a whole fight untouched")
	}
	if dead, err := bus.DeadLetters(); err != nil || len(dead) != 0 {
		t.Fatalf("%d dead letters (err %v)", len(dead), err)
	}
}

// --- the world of the stub ---

// TestTheDefaultWorldIsTheOneTheFixturesDescribe ties the constant of
// FakeContext to the fixture tree it says it comes from. Nothing in the code
// reads the file, so a fixture world renamed elsewhere would otherwise leave
// the stub serving a world nobody lives in.
func TestTheDefaultWorldIsTheOneTheFixturesDescribe(t *testing.T) {
	loaded, err := state.LoadFixtures(fixturesDir(t))
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	for _, e := range loaded {
		if e.Type == entity.TypeWorld {
			if e.ID != swarm.DefaultWorldID {
				t.Errorf("the fixtures describe %q, the stub serves %q", e.ID, swarm.DefaultWorldID)
			}
			return
		}
	}
	t.Fatal("the fixtures describe no world")
}

// TestAFactTeachesTheStubAboutAWorldItWasNotSeededWith: in core the world may
// come from State rather than from a file (C-14), and the stub has to be able
// to open a fight against a wolf it first heard of on the bus.
func TestAFactTeachesTheStubAboutAWorldItWasNotSeededWith(t *testing.T) {
	bus := newBus(t)
	enc := stubOver(t, bus)

	for _, e := range mustFixtures(t) {
		if err := enc.Observe(t.Context(), createdFact(e)); err != nil {
			t.Fatalf("observe %s: %v", e.ID, err)
		}
	}
	act(t, enc, entered(playerA, nameA, regionID, regName))

	if _, open := enc.ActiveEncounter(playerA); !open {
		t.Error("no encounter was opened against a wolf learned from entity.created")
	}
}

// TestEverythingTheStubPublishesIsAValidEvent is the belt to the braces of the
// bus: Publish refuses an invalid event, and this asks the registry the same
// question about every event of a whole fight, so that a schema loosened
// elsewhere cannot pass unnoticed.
func TestEverythingTheStubPublishesIsAValidEvent(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	swingUntilOver(t, enc, bus)

	got := allEvents(t, bus)
	if len(got) < 6 {
		t.Fatalf("a whole fight is more than %d events", len(got))
	}
	for _, ev := range got {
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("%s (%s): %v", ev.Type, ev.ID, err)
		}
		if ev.Source != contracts.SourceTestkitSwarm {
			t.Errorf("%s carries source %q, want %q", ev.Type, ev.Source, contracts.SourceTestkitSwarm)
		}
		if ev.Meta.Agent == nil {
			t.Fatalf("%s carries no meta.agent", ev.Type)
		}
		// Opening the fight is the region GM and everything after it is the
		// encounter agent: §4.6 gives create of an encounter with cause=spawn
		// to the domain level, and contracts.md §0 names region-gm as the
		// publisher of encounter.started.
		wantLevel := swarm.AgentLevelTask
		if ev.Type == swarm.TypeCreateProposed || ev.Type == swarm.TypeEncounterStarted {
			wantLevel = swarm.AgentLevelDomain
		}
		if ev.Meta.Agent.Level != wantLevel {
			t.Errorf("%s is attributed to a %q agent, want %q",
				ev.Type, ev.Meta.Agent.Level, wantLevel)
		}
		if want := swarm.EncounterAgentPrefix + ":solo:" + playerA; ev.Meta.Agent.ID != want {
			t.Errorf("%s is attributed to %q, want %q", ev.Type, ev.Meta.Agent.ID, want)
		}
		if !strings.HasPrefix(ev.Meta.Agent.Blueprint, "fake-") {
			t.Errorf("%s names blueprint %q; a stub writes the fake- prefix (contracts.md §0)",
				ev.Type, ev.Meta.Agent.Blueprint)
		}
		if ev.Meta.GMPath != eventbus.GMPathAgent {
			t.Errorf("%s carries gm_path %q, want the path of its cause", ev.Type, ev.Meta.GMPath)
		}
	}
}

// --- losing the version race (the decision of 2026-09-11) ---

// TestAPackageRefusedForAVersionConflictIsOfferedAgain is the whole of what the
// retry is for. A blow can overtake the fact that moved somebody it touches —
// the actions and the facts travel on two topics and C-01 orders neither
// against the other — and the package is then pinned to a version the world has
// left. State refuses it, and the answer is the one an optimistic lock exists
// for: fold in what has arrived, work the package out again against it, offer
// it under the same identifier.
func TestAPackageRefusedForAVersionConflictIsOfferedAgain(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	// A blow that lands, answered by a bite that misses: the package is about
	// the wolf and the round it spent, which is what makes the arithmetic of
	// the retry a thing this test can read.
	act(t, enc, attackWhere(t, func(id string) bool {
		return tkmech.Verdict(id, 0) == tkmech.VerdictHit && !hits(tkmech.Verdict(id, 2))
	}))

	first := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	full := fixtureHP(t, wolfID)
	damage := full - hpProposedFor(t, first, wolfID)
	if damage <= 0 {
		t.Fatalf("the blow of the first package took %d hit points off the wolf", damage)
	}
	decided := len(ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided))
	rolled := len(ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeDiceRolled))

	// Somebody else wounded the wolf while the package was on its way, and
	// State refused the package because of exactly that.
	wounded := full - 2
	observe(t, enc, hurt(wolfID, 2, full, wounded))
	observe(t, enc, refusal(proposalOf(t, first), swarm.ReasonVersionConflict, wolfID))

	offers := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed)
	if len(offers) != 2 {
		t.Fatalf("%d packages: a package refused for a version conflict is offered again", len(offers))
	}
	second := offers[1]
	if got, want := proposalOf(t, second), proposalOf(t, first); got != want {
		t.Errorf("the second offer names itself %q and the first %q: a retry is the same "+
			"answer to the same action and not a second one", got, want)
	}

	// The heart of it: the wound is worked out again, against the hit points
	// the world has now. A package re-sent under a new version would write
	// hit points that were true before somebody else struck.
	if hp, want := hpProposedFor(t, second, wolfID), wounded-damage; hp != want {
		t.Errorf("the second offer leaves the wolf at %d hit points, want %d: the same "+
			"damage taken off the %d it has now, not off the %d it had", hp, want, wounded, full)
	}
	if v := expectedVersionOf(t, second, wolfID); v != 2 {
		t.Errorf("the second offer pins the wolf at version %d, the fact left it at 2", v)
	}

	// The round the action spent is written once and is the same in both
	// offers: a retry answers the action it was, not the next one.
	if got, want := roundProposedFor(t, second), roundProposedFor(t, first); got != want {
		t.Errorf("the second offer records round %d and the first %d", got, want)
	}
	// And nothing of the fight was decided a second time: the dice were rolled
	// once and the decision published once.
	if got := len(ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided)); got != decided {
		t.Errorf("%d combat.decided after the retry, %d before it", got, decided)
	}
	if got := len(ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeDiceRolled)); got != rolled {
		t.Errorf("%d dice.rolled after the retry, %d before it", got, rolled)
	}
}

// TestARetryDoesNotStrikeAFighterWhoHasFallen is the other half of working the
// package out again: the world that moved may have moved past the blow
// altogether. Nobody strikes a corpse, and the round the action spent travels
// all the same — a round nobody wrote down can be closed by nothing.
func TestARetryDoesNotStrikeAFighterWhoHasFallen(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	act(t, enc, attackWhere(t, func(id string) bool {
		return tkmech.Verdict(id, 0) == tkmech.VerdictHit && !hits(tkmech.Verdict(id, 2))
	}))
	first := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)

	observe(t, enc, died(wolfID))
	observe(t, enc, refusal(proposalOf(t, first), swarm.ReasonVersionConflict, wolfID))

	offers := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed)
	if len(offers) != 2 {
		t.Fatalf("%d packages, want the refused one offered again", len(offers))
	}
	for _, set := range changeSets(t, offers[1]) {
		if set.Ref().ID == wolfID {
			t.Errorf("the second offer strikes %s, which fell before it was made", wolfID)
		}
	}
	if roundProposedFor(t, offers[1]) != roundProposedFor(t, first) {
		t.Error("the round the action spent did not survive the retry")
	}
}

// TestARefusedPackageMovedNothingAtAll is the property the retry rests on. A
// package refused for a version conflict is refused whole (atomic), so nothing
// it named moved — and the offer that follows has to say the same thing again
// about everyone the conflict was not about. A stub that kept its own package
// in its view would take the damage off twice and pin a version State never
// reached.
func TestARefusedPackageMovedNothingAtAll(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	// An exchange both sides land: the package holds the wolf, the character
	// and the encounter, and only one of the three is in the conflict.
	act(t, enc, attackWhere(t, func(id string) bool {
		return tkmech.Verdict(id, 0) == tkmech.VerdictHit && hits(tkmech.Verdict(id, 2))
	}))
	first := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)

	// The character was walking while the fight was answering, and the fact of
	// it reached the stub after the package had left.
	observe(t, enc, moved(playerA, 2, regionID))
	observe(t, enc, refusal(proposalOf(t, first), swarm.ReasonVersionConflict, playerA))

	offers := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed)
	if len(offers) != 2 {
		t.Fatalf("%d packages, want the refused one offered again", len(offers))
	}
	second := offers[1]
	for _, who := range []string{wolfID, playerA} {
		if hp, want := hpProposedFor(t, second, who), hpProposedFor(t, first, who); hp != want {
			t.Errorf("the second offer leaves %s at %d hit points and the first at %d: "+
				"the refused package took nothing off anybody", who, hp, want)
		}
	}
	if v := expectedVersionOf(t, second, wolfID); v != expectedVersionOf(t, first, wolfID) {
		t.Errorf("the second offer pins the wolf at version %d: nothing moved it", v)
	}
	if v := expectedVersionOf(t, second, playerA); v != 2 {
		t.Errorf("the second offer pins %s at version %d, the fact left it at 2", playerA, v)
	}
}

// TestTheStubGivesUpOnAPackageItKeepsLosingWith holds the bound. An agent that
// lost the race twice in a row against a world nobody else is writing is not
// racing any more, and a loop nobody can see is worse than a defect somebody
// can read about.
func TestTheStubGivesUpOnAPackageItKeepsLosingWith(t *testing.T) {
	said := &strings.Builder{}
	enc, bus := fightStubLogging(t, said)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	act(t, enc, attack(playerA, nameA, wolfID))
	proposal := proposalOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed))

	// One refusal more than the stub is allowed to answer.
	for range 4 {
		observe(t, enc, refusal(proposal, swarm.ReasonVersionConflict, wolfID))
	}

	offers := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed)
	if len(offers) != 3 {
		t.Errorf("%d packages for one action: the stub offers one and retries it twice", len(offers))
	}
	if !strings.Contains(said.String(), "gave up") {
		t.Errorf("the stub gave up in silence; the log says:\n%s", said)
	}
}

// TestARefusalThatIsNotARaceIsNotRetried is the other side of the decision.
// Four of the five reasons say the stub asked for something that was never
// going to work — and a defect retried three times is a defect that disappears
// on the third try instead of being seen.
func TestARefusalThatIsNotARaceIsNotRetried(t *testing.T) {
	for _, reason := range []string{
		"unknown_entity", "invalid_op", "dead_entity", "duplicate_entity",
	} {
		t.Run(reason, func(t *testing.T) {
			said := &strings.Builder{}
			enc, bus := fightStubLogging(t, said)
			act(t, enc, entered(playerA, nameA, regionID, regName))
			act(t, enc, attack(playerA, nameA, wolfID))
			proposal := proposalOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed))

			observe(t, enc, refusal(proposal, reason, wolfID))

			if offers := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents),
				swarm.TypeUpdateProposed); len(offers) != 1 {
				t.Errorf("%d packages: %s is a defect and not a race, and is not offered again",
					len(offers), reason)
			}
			if !strings.Contains(said.String(), reason) {
				t.Errorf("the refusal is not in the log; it says:\n%s", said)
			}
		})
	}
}

// TestARefusalOfSomebodyElsesPackageIsNotTheBusinessOfTheStub keeps the stub
// out of a conversation it is not in: the gateway proposes its own changes and
// answers for them itself (C-02 v1.1).
func TestARefusalOfSomebodyElsesPackageIsNotTheBusinessOfTheStub(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	act(t, enc, attack(playerA, nameA, wolfID))

	observe(t, enc, refusal("gw-7", swarm.ReasonVersionConflict, playerA))

	if offers := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents),
		swarm.TypeUpdateProposed); len(offers) != 1 {
		t.Errorf("%d packages: the stub answered a refusal of somebody else's", len(offers))
	}
}

// --- helpers ---

func fightStub(t *testing.T) (*swarm.FakeEncounter, *membus.Bus) {
	t.Helper()
	return fightStubWith(t)
}

// fightStubWith builds the stub over the fixtures, with the changes a scenario
// needs made to them first.
func fightStubWith(t *testing.T, changes ...func(*entity.Entity)) (*swarm.FakeEncounter, *membus.Bus) {
	t.Helper()
	bus := newBus(t)
	enc := stubOver(t, bus)
	world := mustFixtures(t)
	for _, change := range changes {
		for _, e := range world {
			change(e)
		}
	}
	if err := enc.Seed(world); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return enc, bus
}

// fightStubLogging is the stub over the fixtures with everything it says
// written into a buffer, for the tests that are about what it says.
func fightStubLogging(t *testing.T, said *strings.Builder) (*swarm.FakeEncounter, *membus.Bus) {
	t.Helper()
	bus := newBus(t)
	fixed := mechanics(t)
	enc, err := swarm.NewFakeEncounter(swarm.EncounterConfig{
		Bus: bus, WorldID: worldID, Rules: fixed.Rules(), Mechanics: fixed,
		Log: slog.New(slog.NewTextHandler(said, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatalf("encounter: %v", err)
	}
	if err := enc.Seed(mustFixtures(t)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return enc, bus
}

func stubOver(t *testing.T, bus *membus.Bus) *swarm.FakeEncounter {
	t.Helper()
	fixed := mechanics(t)
	enc, err := swarm.NewFakeEncounter(swarm.EncounterConfig{
		Bus:       bus,
		WorldID:   worldID,
		Rules:     fixed.Rules(),
		Mechanics: fixed,
	})
	if err != nil {
		t.Fatalf("encounter: %v", err)
	}
	return enc
}

func observe(t *testing.T, enc *swarm.FakeEncounter, ev eventbus.Event) {
	t.Helper()
	if err := enc.Observe(t.Context(), ev); err != nil {
		t.Fatalf("observe %s: %v", ev.Type, err)
	}
}

// fixtureHP is what a fixture entity starts the world with.
func fixtureHP(t *testing.T, id string) int {
	t.Helper()
	for _, e := range mustFixtures(t) {
		if e.ID != id {
			continue
		}
		hp, ok := e.HP()
		if !ok {
			t.Fatalf("the fixture %s has no hit points", id)
		}
		return hp
	}
	t.Fatalf("%s is not among the fixtures", id)
	return 0
}

func proposalOf(t *testing.T, proposal eventbus.Event) string {
	t.Helper()
	id, _ := proposal.Path().GetString("proposal_id")
	if id == "" {
		t.Fatalf("%s names no proposal", proposal.Type)
	}
	return id
}

// hpProposedFor is the hit points a package leaves an entity at.
func hpProposedFor(t *testing.T, proposal eventbus.Event, entityID string) int {
	t.Helper()
	return opValue(t, proposal, entityID, entity.AttrHP)
}

// roundProposedFor is the round a package writes on the encounter entity, or
// zero when the package closes the fight instead of opening a round.
func roundProposedFor(t *testing.T, proposal eventbus.Event) int {
	t.Helper()
	for _, set := range changeSets(t, proposal) {
		if set.Ref().Type != entity.TypeEncounter {
			continue
		}
		for _, op := range set.Ops {
			if op.Path == entity.AttrRoundSeq {
				return asInt(t, op.Value)
			}
		}
		return 0
	}
	t.Fatalf("the package changes no encounter")
	return 0
}

func expectedVersionOf(t *testing.T, proposal eventbus.Event, entityID string) int64 {
	t.Helper()
	for _, set := range changeSets(t, proposal) {
		if set.Ref().ID != entityID {
			continue
		}
		if set.ExpectedVersion == nil {
			t.Fatalf("the change set of %s pins no version", entityID)
		}
		return *set.ExpectedVersion
	}
	t.Fatalf("the package does not name %s", entityID)
	return 0
}

func opValue(t *testing.T, proposal eventbus.Event, entityID, path string) int {
	t.Helper()
	for _, set := range changeSets(t, proposal) {
		if set.Ref().ID != entityID {
			continue
		}
		for _, op := range set.Ops {
			if op.Path == path {
				return asInt(t, op.Value)
			}
		}
	}
	t.Fatalf("the package writes no %s of %s", path, entityID)
	return 0
}

// asInt reads a number that has been through JSON, where every one of them is
// a float64 whichever way the publisher meant it.
func asInt(t *testing.T, value any) int {
	t.Helper()
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		t.Fatalf("%v is not a number", value)
		return 0
	}
}

// refusal is the entity.update.rejected State publishes when it turns a package
// down. details are what a version conflict carries; the stub reads the reason
// and the entity, and this is the shape C-02 gives both.
func refusal(proposalID, reason, entityID string) eventbus.Event {
	payload := map[string]any{"proposal_id": proposalID, "reason": reason}
	if entityID != "" {
		payload["entity"] = map[string]any{
			"entity": map[string]any{"id": entityID, "type": entity.TypeNPC},
		}
	}
	if reason == swarm.ReasonVersionConflict {
		payload["details"] = map[string]any{"expected_version": 1, "actual_version": 2}
	}
	return eventbus.NewRoot(swarm.TypeRejected, contracts.SourceTestkitState, worldID, nil,
		eventbus.ActorSystem, payload)
}

func mechanics(t *testing.T) *tkmech.FixedMechanics {
	t.Helper()
	fixed, err := tkmech.Load(filepath.Join("..", "..", "..", "rules", "dark-forest.yaml"))
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	return fixed
}

func fixturesDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "fixtures")
}

func mustFixtures(t *testing.T) []*entity.Entity {
	t.Helper()
	loaded, err := state.LoadFixtures(fixturesDir(t))
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	return loaded
}

// attr changes one attribute of one fixture entity before the world is seeded.
func attr(id, path string, value any) func(*entity.Entity) {
	return func(e *entity.Entity) {
		if e.ID == id {
			e.Attributes[path] = value
		}
	}
}

func dead(id string) func(*entity.Entity) {
	return attr(id, entity.AttrStatus, entity.StatusDead)
}

func act(t *testing.T, enc *swarm.FakeEncounter, ev eventbus.Event) {
	t.Helper()
	if err := enc.Act(t.Context(), ev); err != nil {
		t.Fatalf("act on %s: %v", ev.Type, err)
	}
}

// hits says whether a row of the table of FixedMechanics is a landed blow — or,
// for a flight, a successful escape.
func hits(verdict string) bool {
	return verdict == tkmech.VerdictHit || verdict == tkmech.VerdictCritical
}

// attackWhere builds the attack whose cause event lands the scenario on the row
// of the table it wants to show. The identifiers are a deterministic sequence
// (testkit.Deterministic), so the search is over the same events on every run
// and costs nothing.
func attackWhere(t *testing.T, want func(causeEventID string) bool) eventbus.Event {
	t.Helper()
	return eventWhere(t, want, func() eventbus.Event { return attack(playerA, nameA, wolfID) })
}

func fleeWhere(t *testing.T, want func(causeEventID string) bool) eventbus.Event {
	t.Helper()
	return eventWhere(t, want, func() eventbus.Event { return flee(playerA, nameA) })
}

func eventWhere(t *testing.T, want func(string) bool, build func() eventbus.Event) eventbus.Event {
	t.Helper()
	for i := 0; i < 200; i++ {
		ev := build()
		if want(ev.ID) {
			return ev
		}
	}
	t.Fatal("no event of the sequence lands on the outcome this test is about")
	return eventbus.Event{}
}

// swingUntilOver attacks until the fight ends, the way a player would. Twenty
// exchanges are far more than a wolf survives.
func swingUntilOver(t *testing.T, enc *swarm.FakeEncounter, bus *membus.Bus) {
	t.Helper()
	for i := 0; i < 20; i++ {
		if _, open := enc.ActiveEncounter(playerA); !open {
			return
		}
		act(t, enc, attack(playerA, nameA, wolfID))
	}
	t.Fatalf("the fight is still going after twenty exchanges (%d events)", len(allEvents(t, bus)))
}

// --- the ownership table (§4.6) ---

// proposal is one change a proposal asks for, reduced to what §4.6 decides on:
// who proposes at what level, over which entity type, on which paths, for which
// cause, and whether the entity is being created.
type proposal struct {
	op         string // create | update
	entityType string
	cause      string
	level      string
	paths      []string
	create     bool
}

// kind is the family of the proposal, without the paths: what the coverage
// check counts, so that a run which stopped producing a whole family of
// proposals is noticed.
func (p proposal) kind() string {
	return fmt.Sprintf("%s %s cause=%s level=%s", p.op, p.entityType, p.cause, p.level)
}

func (p proposal) String() string {
	if p.op == "create" {
		return p.kind()
	}
	return fmt.Sprintf("%s paths=%v", p.kind(), p.paths)
}

// proposalsOf reduces one entity.create.proposed or entity.update.proposed to
// the proposals §4.6 is asked about — one per change set.
func proposalsOf(t *testing.T, ev eventbus.Event) []proposal {
	t.Helper()
	level := ""
	if ev.Meta.Agent != nil {
		level = ev.Meta.Agent.Level
	}
	cause, _ := ev.Path().GetString("cause")
	switch ev.Type {
	case swarm.TypeCreateProposed:
		kind, _ := ev.Path().GetString("entity.entity.type")
		return []proposal{{op: "create", entityType: kind, cause: cause, level: level, create: true}}
	case swarm.TypeUpdateProposed:
		var out []proposal
		for _, set := range changeSets(t, ev) {
			paths := make([]string, 0, len(set.Ops))
			for _, op := range set.Ops {
				if !slices.Contains(paths, op.Path) {
					paths = append(paths, op.Path)
				}
			}
			slices.Sort(paths)
			out = append(out, proposal{op: "update", entityType: set.Ref().Type,
				cause: cause, level: level, paths: paths})
		}
		return out
	default:
		return nil
	}
}

// ownershipAllows answers the question State answers on step 6 of §4.5: is
// there a row of contracts.OwnershipRules() that covers this proposal whole.
// Paths are prefixes, and every path of the change set has to be covered by the
// same row.
func ownershipAllows(p proposal) bool {
	for _, rule := range contracts.OwnershipRules() {
		if rule.Proposer != p.level {
			continue
		}
		if !covers(rule.EntityTypes, p.entityType, contracts.AnyType) {
			continue
		}
		if !covers(rule.Causes, p.cause, contracts.AnyCause) {
			continue
		}
		if p.create && !rule.Create {
			continue
		}
		allowed := true
		for _, path := range p.paths {
			if !coversPath(rule.Paths, path) {
				allowed = false
				break
			}
		}
		if allowed {
			return true
		}
	}
	return false
}

func covers(list []string, want, wildcard string) bool {
	return slices.Contains(list, wildcard) || slices.Contains(list, want)
}

func coversPath(prefixes []string, path string) bool {
	for _, prefix := range prefixes {
		if prefix == contracts.AnyPath || path == prefix || strings.HasPrefix(path, prefix+".") ||
			strings.HasPrefix(path, prefix+"[") {
			return true
		}
	}
	return false
}

// --- the events of the tests ---

func attack(id, name, target string) eventbus.Event {
	return playerAction(swarm.TypeAttacked, map[string]any{
		"entity": actor(id, name),
		"action": map[string]any{"type": "attack"},
		"target": map[string]any{
			"entity": map[string]any{"id": target, "type": entity.TypeNPC},
		},
	})
}

func flee(id, name string) eventbus.Event {
	return playerAction(swarm.TypeFleeAttempted, map[string]any{
		"entity": actor(id, name),
		"action": map[string]any{"type": "flee"},
	})
}

func rest(id, name string) eventbus.Event {
	return playerAction(swarm.TypeRested, map[string]any{
		"entity": actor(id, name),
		"action": map[string]any{"type": "rest"},
	})
}

// attackAnything is a swing that names no target: whoever of the encounter is
// still standing (§2.3.1 makes target mandatory, and Act is driven by hand
// here).
func attackAnything(id, name string) eventbus.Event {
	return playerAction(swarm.TypeAttacked, map[string]any{
		"entity": actor(id, name),
		"action": map[string]any{"type": "attack"},
	})
}

// hurt is the entity.updated State publishes when somebody else wounded the
// wolf.
func hurt(id string, version, from, to int) eventbus.Event {
	return eventbus.NewRoot(swarm.TypeUpdated, contracts.SourceTestkitState, worldID, nil,
		eventbus.ActorSystem, map[string]any{
			"entity":      map[string]any{"entity": map[string]any{"id": id, "type": entity.TypeNPC}},
			"version":     version,
			"cause":       "combat",
			"proposal_id": "p-elsewhere",
			"changed": []map[string]any{
				{"path": entity.AttrHP, "old": from, "new": to},
			},
		})
}

// died is the entity.updated State publishes when somebody else killed the
// wolf.
func died(id string) eventbus.Event {
	return eventbus.NewRoot(swarm.TypeUpdated, contracts.SourceTestkitState, worldID, nil,
		eventbus.ActorSystem, map[string]any{
			"entity":      map[string]any{"entity": map[string]any{"id": id, "type": entity.TypeNPC}},
			"version":     2,
			"cause":       "combat",
			"proposal_id": "p-elsewhere",
			"changed": []map[string]any{
				{"path": entity.AttrStatus, "old": entity.StatusAlive, "new": entity.StatusDead},
				{"path": entity.AttrHP, "old": 10, "new": 0},
			},
		})
}

// moved is the entity.updated State publishes when the gateway walked a
// character somewhere: a path of the character the fight never writes, which
// is what makes it the fact a blow can overtake.
func moved(id string, version int, to string) eventbus.Event {
	return eventbus.NewRoot(swarm.TypeUpdated, contracts.SourceTestkitState, worldID, nil,
		eventbus.ActorHuman, map[string]any{
			"entity":      map[string]any{"entity": map[string]any{"id": id, "type": entity.TypePlayer}},
			"version":     version,
			"cause":       "move",
			"proposal_id": "gw-move",
			"changed": []map[string]any{
				{"path": entity.AttrPosition, "old": "outside:" + worldID, "new": to},
			},
		})
}

// createdFact is the entity.created State publishes for a fixture entity.
func createdFact(e *entity.Entity) eventbus.Event {
	return eventbus.NewRoot(swarm.TypeCreated, contracts.SourceTestkitState, worldID, nil,
		eventbus.ActorSystem, map[string]any{
			"entity":     map[string]any{"entity": map[string]any{"id": e.ID, "type": e.Type}, "name": e.Name},
			"version":    1,
			"attributes": e.Attributes,
		})
}

// --- reading the bus ---

func eventsOf(t *testing.T, bus *membus.Bus, topic string) []eventbus.Event {
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

// allEvents is everything the stub published, in the order of the topics the
// registry knows. dead_letters is not among them: it is the bus, not a
// publisher.
func allEvents(t *testing.T, bus *membus.Bus) []eventbus.Event {
	t.Helper()
	var out []eventbus.Event
	for _, spec := range contracts.Topics() {
		if spec.Name == eventbus.TopicDeadLetters {
			continue
		}
		for _, ev := range eventsOf(t, bus, spec.Name) {
			if ev.Source == contracts.SourceTestkitSwarm {
				out = append(out, ev)
			}
		}
	}
	return out
}

func ofType(events []eventbus.Event, typ string) []eventbus.Event {
	out := make([]eventbus.Event, 0, len(events))
	for _, ev := range events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func onlyOne(t *testing.T, bus *membus.Bus, topic, typ string) eventbus.Event {
	t.Helper()
	got := ofType(eventsOf(t, bus, topic), typ)
	if len(got) != 1 {
		t.Fatalf("%d events of type %s, want exactly one", len(got), typ)
	}
	return got[0]
}

func encounterOf(t *testing.T, create eventbus.Event) string {
	t.Helper()
	id, _ := create.Path().GetString("entity.entity.id")
	return id
}

func changeSets(t *testing.T, proposal eventbus.Event) []entity.ChangeSet {
	t.Helper()
	var payload struct {
		Changes []entity.ChangeSet `json:"changes"`
	}
	raw, err := json.Marshal(proposal.Payload)
	if err != nil {
		t.Fatalf("marshal the proposal: %v", err)
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode the proposal: %v", err)
	}
	return payload.Changes
}

// opsFor is every operation every proposal of the run asked for on one entity.
func opsFor(t *testing.T, bus *membus.Bus, id string) []entity.Op {
	t.Helper()
	var out []entity.Op
	for _, proposal := range ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed) {
		for _, set := range changeSets(t, proposal) {
			if set.Ref().ID == id {
				out = append(out, set.Ops...)
			}
		}
	}
	return out
}

// participantsOf is the participants[] a set operation writes, decoded the way
// the encounter agent of C4 and mechanics.ActorFromEntity read it.
func participantsOf(t *testing.T, ops []entity.Op) []entity.Participant {
	t.Helper()
	for _, op := range ops {
		if op.Op != entity.OpSet || op.Path != entity.AttrParticipants {
			continue
		}
		raw, err := json.Marshal(op.Value)
		if err != nil {
			t.Fatalf("marshal participants: %v", err)
		}
		var people []entity.Participant
		if err := json.Unmarshal(raw, &people); err != nil {
			t.Fatalf("decode participants: %v", err)
		}
		return people
	}
	t.Fatalf("no operation writes %s; the ops are %v", entity.AttrParticipants, ops)
	return nil
}

// numericOp is the value a set operation writes at one path, as a number: a
// value read back off the bus is a JSON number whatever it was in Go.
func numericOp(ops []entity.Op, path string) (float64, bool) {
	for _, op := range ops {
		if op.Op != entity.OpSet || op.Path != path {
			continue
		}
		n, ok := op.Value.(float64)
		return n, ok
	}
	return 0, false
}

func assertOp(t *testing.T, ops []entity.Op, path string, want any) {
	t.Helper()
	for _, op := range ops {
		if op.Path == path && op.Value == want {
			return
		}
	}
	t.Errorf("no operation sets %s to %v; the proposals hold %v", path, want, ops)
}

func itemOf(t *testing.T, op entity.Op) entity.Item {
	t.Helper()
	raw, err := json.Marshal(op.Value)
	if err != nil {
		t.Fatalf("marshal the trophy: %v", err)
	}
	var item entity.Item
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("decode the trophy: %v", err)
	}
	return item
}
