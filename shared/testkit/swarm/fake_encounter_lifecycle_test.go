package swarm_test

import (
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	mech "multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	tkmech "multiverse-core.io/shared/testkit/mechanics"
	"multiverse-core.io/shared/testkit/membus"
	"multiverse-core.io/shared/testkit/swarm"
)

// This file holds what C-05 v1.4 and ADR-026 changed in the encounter agent,
// one test per rule, on the double the agent of T-230 takes its shape from:
//
//   - encounter.started goes out after entity.created, and never for a creation
//     State refused (p. 4);
//   - encounter.ended goes out after the fact of the package that closes the
//     fight; an action meanwhile waits for that package, and a package State
//     never applied ends nothing (p. 5);
//   - one NPC is held by one active encounter (p. 6);
//   - a fight whose last NPC fell to somebody else's hand is closed by its
//     agent, with no killer and under the scope of the fight (p. 6);
//   - participation goes back with a package State never applied, the round
//     does not (p. 1г);
//   - every decision says where it stands in its exchange (p. 7).

const (
	playerB = "player-B"
	nameB   = "Лена"
)

// --- p. 4: the start after the fact ---

// TestTheEncounterIsAnnouncedOnlyOnceStateCreatedIt: an event that announces
// an encounter State has not created cannot be taken back, so it is not sent
// before the fact (variant A1 of ADR-026).
func TestTheEncounterIsAnnouncedOnlyOnceStateCreatedIt(t *testing.T) {
	enc, bus := fightStub(t)

	act(t, enc, entered(playerA, nameA, regionID, regName))

	create := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed)
	if got := ofType(eventsOf(t, bus, eventbus.TopicWorldEvents), swarm.TypeEncounterStarted); len(got) != 0 {
		t.Fatalf("%d encounter.started before State created the encounter", len(got))
	}
	if _, open := enc.ActiveEncounter(playerA); open {
		t.Error("the fight is on before State has created its encounter")
	}

	answer(t, enc, bus)

	started := onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterStarted)
	if opened, _ := create.Path().GetString("attributes.opened_by_event_id"); opened != started.ID {
		t.Errorf("the encounter names %q as what opened it, the announcement is %q", opened, started.ID)
	}
	if _, open := enc.ActiveEncounter(playerA); !open {
		t.Error("State created the encounter and the fight is still not on")
	}
}

// TestARefusedCreationAnnouncesNothing: there is no encounter to tell about, so
// nothing is told — not the start, not the swing that waited for it. With one
// NPC to one encounter kept, a refused creation is a defect and is said on the
// level of one.
func TestARefusedCreationAnnouncesNothing(t *testing.T) {
	said := &strings.Builder{}
	enc, bus := fightStubLogging(t, said)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	create := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed)
	// A swing sent before State answered the creation: it waits for it.
	act(t, enc, attack(playerA, nameA, wolfID))

	observe(t, enc, refusal(proposalOf(t, create), "duplicate_entity", encounterOf(t, create)))

	if got := ofType(eventsOf(t, bus, eventbus.TopicWorldEvents), swarm.TypeEncounterStarted); len(got) != 0 {
		t.Errorf("%d encounter.started for an encounter State refused to create", len(got))
	}
	if !saidAt(said.String(), "ERROR", "encounter.started is not published") {
		t.Errorf("a refused creation is a defect and is said out loud; the log says:\n%s", said)
	}
	if _, open := enc.ActiveEncounter(playerA); open {
		t.Error("a fight State refused to create is on")
	}
	if got := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided); len(got) != 0 {
		t.Errorf("%d decisions: the swing held for a creation State refused belongs to no fight", len(got))
	}

	// The wolf is nobody's: the next entry offers the fight again.
	act(t, enc, entered(playerA, nameA, regionID, regName))
	if got := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeCreateProposed); len(got) != 2 {
		t.Errorf("%d creations: after a refused one the wolf is free to be met again", len(got))
	}
}

// --- p. 5: the end after the fact ---

// TestTheEndIsAnnouncedOnlyAfterTheFactOfItsPackage: the end is built before the
// package, which names it in closed_by_event_id, and sent after State applied
// it.
func TestTheEndIsAnnouncedOnlyAfterTheFactOfItsPackage(t *testing.T) {
	enc, bus := fightStubWith(t, attr(wolfID, entity.AttrHP, 1))
	openFight(t, enc, bus)
	encounterID := encounterOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed))
	act(t, enc, attackWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) }))

	closing := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	if !closes(t, closing) {
		t.Fatalf("the package of the killing blow does not resolve the encounter: %v", changeSets(t, closing))
	}
	if got := ofType(eventsOf(t, bus, eventbus.TopicWorldEvents), swarm.TypeEncounterEnded); len(got) != 0 {
		t.Fatalf("%d encounter.ended before State applied the package that closes the fight", len(got))
	}
	if _, open := enc.ActiveEncounter(playerA); !open {
		t.Error("the fight is over before State said so")
	}

	answer(t, enc, bus)

	ended := onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterEnded)
	assertOp(t, opsFor(t, bus, encounterID), entity.AttrClosedByEventID, ended.ID)
	if _, open := enc.ActiveEncounter(playerA); open {
		t.Error("State applied the package that closes the fight, and the fight is still on")
	}
}

// TestAnActionWhileTheClosingPackageIsOnItsWayWaitsForIt: answering it from a
// view that holds the unapplied package would be silence in a fight that may
// go on. It waits, and once the package is in, the fight is over and the
// action is answered by nobody.
func TestAnActionWhileTheClosingPackageIsOnItsWayWaitsForIt(t *testing.T) {
	enc, bus := fightStubWith(t, attr(wolfID, entity.AttrHP, 1))
	openFight(t, enc, bus)
	act(t, enc, attackWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) }))
	decisions := func() int {
		return len(ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided))
	}
	before := decisions()

	act(t, enc, flee(playerA, nameA))
	act(t, enc, attack(playerA, nameA, wolfID))
	if got := decisions(); got != before {
		t.Fatalf("%d decisions for actions sent while the closing package was on its way; "+
			"they wait for it", got-before)
	}

	answer(t, enc, bus)

	onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterEnded)
	if got := decisions(); got != before {
		t.Errorf("%d decisions after the fight closed: what waited for the package is "+
			"answered by nobody", got-before)
	}
}

// TestADroppedClosingPackageEndsNothing is the other way a closing package can
// go: State never applies it, the end is never announced, and the fight goes
// on — with the character back in it, the round the decision spent kept, and
// the action that waited for the package answered.
func TestADroppedClosingPackageEndsNothing(t *testing.T) {
	said := &strings.Builder{}
	enc, bus, spy := fightStubSpying(t, said, attr(wolfID, entity.AttrHP, 1))
	openFight(t, enc, bus)
	act(t, enc, attackWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) }))
	kill := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	// A flight that fails and whose strike out of turn misses: it cannot end
	// the fight, so what it is answered with is all this test reads.
	act(t, enc, fleeWhere(t, func(id string) bool {
		return !hits(tkmech.Verdict(id, 0)) && !hits(tkmech.Verdict(id, 1))
	}))

	for range 3 {
		observe(t, enc, refusal(proposalOf(t, kill), swarm.ReasonVersionConflict, wolfID))
	}

	if !strings.Contains(said.String(), "gave up") {
		t.Fatalf("the stub did not give up on the package; the log says:\n%s", said)
	}
	if got := ofType(eventsOf(t, bus, eventbus.TopicWorldEvents), swarm.TypeEncounterEnded); len(got) != 0 {
		t.Errorf("%d encounter.ended for a package State never applied", len(got))
	}
	if _, open := enc.ActiveEncounter(playerA); !open {
		t.Error("the fight is over, though the package that would have closed it never went in")
	}
	var flights []eventbus.Event
	for _, ev := range ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided) {
		if action, _ := ev.Path().GetString("action"); action == mech.ActionFlee {
			flights = append(flights, ev)
		}
	}
	if len(flights) != 1 {
		t.Fatalf("%d decisions of the flight that waited for the dropped package, want one", len(flights))
	}
	// The killing blow spent round 1: the decision said so, and it stays said.
	if seq, _ := flights[0].Path().GetInt("round.seq"); seq != 2 {
		t.Errorf("the flight is decided in round %d, want 2: the round of a dropped package is not "+
			"taken back", seq)
	}
	// The package took the character out of the fight; dropped, it gives them back.
	if got := spy.last(t, mech.ActionFlee).actors[playerA].Participation; got != entity.ParticipationActive {
		t.Errorf("the flight was decided for a character whose participation is %q, want %q",
			got, entity.ParticipationActive)
	}
}

// --- p. 1г: participation goes with the package ---

// TestADroppedAttemptTakesItsParticipationWithIt: the damage dealt and the last
// damager live only in the package and in the encounter entity. Left in the
// view of the agent, the next package would write into State a blow State never
// applied, and the last damager would steer the choice of the NPC's target.
func TestADroppedAttemptTakesItsParticipationWithIt(t *testing.T) {
	enc, bus, spy := fightStubSpying(t, nil,
		attr(wolfID, entity.AttrHP, 40), attr(wolfID, entity.AttrHPMax, 40))
	openFight(t, enc, bus)
	encounterID := encounterOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed))
	lands := func(id string) bool { return tkmech.Verdict(id, 0) == tkmech.VerdictHit }

	act(t, enc, attackWhere(t, lands))
	first := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	for range 3 {
		observe(t, enc, refusal(proposalOf(t, first), swarm.ReasonVersionConflict, wolfID))
	}
	act(t, enc, attackWhere(t, lands))

	offers := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed)
	second := offers[len(offers)-1]
	if proposalOf(t, second) == proposalOf(t, first) {
		t.Fatal("the second swing was not answered with a package of its own")
	}
	dealt := 40 - hpProposedFor(t, second, wolfID)
	var ops []entity.Op
	for _, set := range changeSets(t, second) {
		if set.Ref().ID == encounterID {
			ops = set.Ops
		}
	}
	if people := participantsOf(t, ops); people[0].DamageDealt != dealt {
		t.Errorf("the package records %d damage dealt, the one blow State can apply dealt %d: "+
			"the dropped one went with its package", people[0].DamageDealt, dealt)
	}
	if damager := spy.last(t, mech.ActionAttack).actors[wolfID].LastDamager; damager != "" {
		t.Errorf("the second swing was decided against a wolf last wounded by %q; State holds "+
			"no wound of it", damager)
	}
	// The dropped blow spent round 1 and the second swing spent round 2.
	if got := roundProposedFor(t, second); got != 3 {
		t.Errorf("the package moves round_seq to %d, want 3: the round of a dropped package is "+
			"not taken back", got)
	}
}

// --- p. 6: one NPC, one active encounter; the NPC somebody else killed ---

// TestTwoCharactersDoNotShareOneWolf: the region GM does not open a second
// encounter around an NPC that is in one (data-model.md §3.7). Two fights over
// one wolf would give its hit points two writers.
func TestTwoCharactersDoNotShareOneWolf(t *testing.T) {
	enc, bus := fightStub(t)
	openFight(t, enc, bus)

	act(t, enc, enteredAs(playerB, nameB))
	answer(t, enc, bus)

	if got := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeCreateProposed); len(got) != 1 {
		t.Fatalf("%d encounters proposed around one wolf, want one", len(got))
	}
	if _, open := enc.ActiveEncounter(playerB); open {
		t.Errorf("%s is in a fight with a wolf that is already in %s's", playerB, playerA)
	}

	// Once the first fight is over — here the character gets away — the wolf
	// is free, and the second character meets it.
	act(t, enc, fleeWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) }))
	answer(t, enc, bus)
	act(t, enc, enteredAs(playerB, nameB))
	answer(t, enc, bus)

	started := ofType(eventsOf(t, bus, eventbus.TopicWorldEvents), swarm.TypeEncounterStarted)
	if len(started) != 2 {
		t.Fatalf("%d encounters opened, want the second one once the wolf was free", len(started))
	}
	if who, _ := started[1].Path().GetString("participants[0].entity.id"); who != playerB {
		t.Errorf("the second encounter is %s's, want %s's", who, playerB)
	}
}

// TestAWolfKilledElsewhereClosesTheFightWithoutAKiller is variant Б1 of
// ADR-026: the lifecycle of the fight belongs to its agent, so the agent closes
// it — resolved as npc_dead, nobody left fighting, no trophy, and after the fact
// an end that names no killer, under the scope of the fight rather than the
// scope of the fact that caused it. A swing at the corpse meanwhile is
// answered by nobody (inv-01).
func TestAWolfKilledElsewhereClosesTheFightWithoutAKiller(t *testing.T) {
	enc, bus := fightStub(t)
	openFight(t, enc, bus)
	encounterID := encounterOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed))
	decisions := func() int {
		return len(ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided))
	}
	fight := eventbus.ScopeRef{ID: playerA, Type: "solo"}

	observe(t, enc, died(wolfID))

	closing := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed)
	sets := changeSets(t, closing)
	if len(sets) != 1 || sets[0].Ref().ID != encounterID {
		t.Fatalf("the closure changes %v, want the encounter alone: no trophy, nobody struck", sets)
	}
	assertOp(t, sets[0].Ops, entity.AttrState, entity.EncounterStateResolved)
	assertOp(t, sets[0].Ops, entity.AttrResolution, entity.ResolutionNPCDead)
	if people := participantsOf(t, sets[0].Ops); people[0].State != entity.ParticipationOutOfCombat {
		t.Errorf("a closed fight leaves %s %q, want %q", playerA, people[0].State,
			entity.ParticipationOutOfCombat)
	}
	// The package resolves the encounter; the death of the wolf is already
	// recorded by its own fact (decision of the orchestrator on T-419). The
	// value is spelled as the contract spells it rather than through the
	// constant of the code, so that a constant changed on one side is caught.
	if cause, _ := closing.Path().GetString("cause"); cause != "resolve" {
		t.Errorf("the closure is proposed with cause %q, want %q", cause, "resolve")
	}
	for _, want := range proposalsOf(t, closing) {
		if !ownershipAllows(want) {
			t.Errorf("no rule of §4.6 allows the closure %s", want)
		}
	}
	if closing.Scope == nil || *closing.Scope != fight {
		t.Errorf("the closure is published under %v, want the scope of the fight %v", closing.Scope, fight)
	}
	if got := ofType(eventsOf(t, bus, eventbus.TopicWorldEvents), swarm.TypeEncounterEnded); len(got) != 0 {
		t.Fatalf("%d encounter.ended before State applied the closure", len(got))
	}

	act(t, enc, attack(playerA, nameA, wolfID))
	act(t, enc, attackAnything(playerA, nameA))
	answer(t, enc, bus)

	ended := onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterEnded)
	if reason, _ := ended.Path().GetString("reason"); reason != entity.ResolutionNPCDead {
		t.Errorf("the fight ended as %q, want %q", reason, entity.ResolutionNPCDead)
	}
	if _, named := ended.Payload["killer"]; named {
		t.Error("the end names a killer; nobody of this fight struck the blow")
	}
	if ended.Scope == nil || *ended.Scope != fight {
		t.Errorf("the end is published under %v, want the scope of the fight %v", ended.Scope, fight)
	}
	assertOp(t, sets[0].Ops, entity.AttrClosedByEventID, ended.ID)
	if got := decisions(); got != 0 {
		t.Errorf("%d decisions for swings at a wolf somebody else killed", got)
	}
	if _, open := enc.ActiveEncounter(playerA); open {
		t.Error("the fight is still on over a dead wolf")
	}
}

// TestAFightWhoseClosureStateRefusedStillAnswersItsActions is probe P1 of
// review #1 of T-419 (Mi-1): the wolf fell to somebody else, the agent offered
// the closure, a flight arrived while it was on its way, and State turned the
// closure down. The fight stays open over the corpse, the closure is not
// offered again on the same fact, and the flight is answered — it does not wait
// for an end that will never come.
func TestAFightWhoseClosureStateRefusedStillAnswersItsActions(t *testing.T) {
	said := &strings.Builder{}
	enc, bus := fightStubLogging(t, said)
	openFight(t, enc, bus)
	encounterID := encounterOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed))
	flights := func() int {
		n := 0
		for _, ev := range ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided) {
			if action, _ := ev.Path().GetString("action"); action == mech.ActionFlee {
				n++
			}
		}
		return n
	}

	observe(t, enc, died(wolfID))
	closure := proposalOf(t, onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeUpdateProposed))
	act(t, enc, flee(playerA, nameA))
	if got := flights(); got != 0 {
		t.Fatalf("%d flights decided while the closure was on its way; they wait for it", got)
	}

	observe(t, enc, refusal(closure, "invalid_op", encounterID))

	if got := flights(); got != 1 {
		t.Errorf("the flight held for the refused closure was decided %d times, want once", got)
	}
	if got := offersOf(t, bus, closure); got != 1 {
		t.Errorf("the refused closure was offered %d times; on the same fact it is offered once", got)
	}
	if !saidAt(said.String(), "ERROR", "was not applied") {
		t.Errorf("a fight left open over a fallen NPC is said out loud; the log says:\n%s", said)
	}
}

// --- the phase of creation (Mi-3 of review #1 of T-419) ---

// TestTwoCharactersEnteringBeforeTheFirstFightExistsDoNotShareOneWolf is probe
// P4: the second character walks in while the encounter of the first is still
// being created. The wolf is held from the proposal on, not from the fact.
func TestTwoCharactersEnteringBeforeTheFirstFightExistsDoNotShareOneWolf(t *testing.T) {
	enc, bus := fightStub(t)

	act(t, enc, entered(playerA, nameA, regionID, regName))
	act(t, enc, enteredAs(playerB, nameB))
	answer(t, enc, bus)

	if got := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeCreateProposed); len(got) != 1 {
		t.Fatalf("%d encounters proposed around one wolf, want one", len(got))
	}
	if _, open := enc.ActiveEncounter(playerA); !open {
		t.Errorf("the fight of %s, who came first, is not on", playerA)
	}
	if _, open := enc.ActiveEncounter(playerB); open {
		t.Errorf("%s is in a fight with a wolf that was already being given to %s", playerB, playerA)
	}
}

// TestAnActionDuringTheCreationIsAnsweredOnceTheFightExists is probe P6: a blow
// sent before entity.created is not lost and not answered twice. It waits for
// the fact, is answered once the fight is on, and its redelivery is dropped.
func TestAnActionDuringTheCreationIsAnsweredOnceTheFightExists(t *testing.T) {
	enc, bus := fightStub(t)
	act(t, enc, entered(playerA, nameA, regionID, regName))
	blow := attackWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) })
	swings := func() int {
		n := 0
		for _, ev := range ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided) {
			if action, _ := ev.Path().GetString("action"); action == mech.ActionAttack &&
				ev.Meta.CausationID == blow.ID {
				n++
			}
		}
		return n
	}

	act(t, enc, blow)
	if got := swings(); got != 0 {
		t.Fatalf("the blow was decided %d times before State created the encounter", got)
	}

	answer(t, enc, bus)
	if got := swings(); got != 1 {
		t.Fatalf("the blow held for the creation was decided %d times once the fight was on, want once", got)
	}

	act(t, enc, blow)
	answer(t, enc, bus)
	if got := swings(); got != 1 {
		t.Errorf("a redelivered blow was decided again: %d decisions, want one", got)
	}
}

// --- the identifiers of the lifecycle (C-01 v1.4, ADR-027 p. 2) ---

// TestTheLifecycleOfAFightKeepsItsIdentifiersAcrossARestart: encounter.started
// and encounter.ended take their id from their cause and the encounter, so an
// agent rebuilt after a restart announces the same events under the same ids —
// the ones opened_by_event_id and closed_by_event_id name — and a copy that
// already went out is dropped by id (C-14 v1.2 (b)). The second stub is that
// agent: a fresh FakeEncounter handed the same entry and the same blow.
func TestTheLifecycleOfAFightKeepsItsIdentifiersAcrossARestart(t *testing.T) {
	type run struct {
		started, ended            eventbus.Event
		openedBy, closedBy, encID string
	}
	entry := entered(playerA, nameA, regionID, regName)
	blow := attackWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) })
	fight := func() run {
		enc, bus := fightStubWith(t, attr(wolfID, entity.AttrHP, 1))
		act(t, enc, entry)
		answer(t, enc, bus)
		act(t, enc, blow)
		answer(t, enc, bus)
		create := onlyOne(t, bus, eventbus.TopicSystemEvents, swarm.TypeCreateProposed)
		r := run{
			started: onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterStarted),
			ended:   onlyOne(t, bus, eventbus.TopicWorldEvents, swarm.TypeEncounterEnded),
			encID:   encounterOf(t, create),
		}
		r.openedBy, _ = create.Path().GetString("attributes.opened_by_event_id")
		for _, op := range opsFor(t, bus, r.encID) {
			if op.Path == entity.AttrClosedByEventID {
				r.closedBy, _ = op.Value.(string)
			}
		}
		return r
	}

	first, again := fight(), fight()

	if first.openedBy != first.started.ID || first.closedBy != first.ended.ID {
		t.Fatalf("the encounter names %q and %q, the events are %q and %q",
			first.openedBy, first.closedBy, first.started.ID, first.ended.ID)
	}
	if again.started.ID != first.started.ID {
		t.Errorf("encounter.started is %q after the restart and %q before it", again.started.ID, first.started.ID)
	}
	if again.ended.ID != first.ended.ID {
		t.Errorf("encounter.ended is %q after the restart and %q before it", again.ended.ID, first.ended.ID)
	}
	if again.openedBy != first.openedBy || again.closedBy != first.closedBy {
		t.Errorf("after the restart the encounter names %q and %q, before it %q and %q",
			again.openedBy, again.closedBy, first.openedBy, first.closedBy)
	}
	// Derived from the cause, with the encounter as the part ADR-027 p. 2 names
	// for both events, and not from the sequence of the generator.
	for _, want := range []struct {
		got   eventbus.Event
		cause eventbus.Event
	}{{first.started, entry}, {first.ended, blow}} {
		derived := eventbus.Derive(want.cause, want.got.Type, swarm.Source, nil,
			eventbus.WithCauseID(first.encID))
		if want.got.ID != derived.ID {
			t.Errorf("%s is %q; derived from its cause and the encounter it is %q",
				want.got.Type, want.got.ID, derived.ID)
		}
	}
}

// --- envelopes without an id (Mi-A of review #2 of T-419) ---

// TestAnActionWithoutAnIDIsRefusedOutLoud is probe RF: with validation on read
// off, an action without an id reaches the stub. The lifecycle derives its ids
// from the action, and deriving from nothing panics and takes the process down.
// The action is refused instead — not answered, and not in silence.
func TestAnActionWithoutAnIDIsRefusedOutLoud(t *testing.T) {
	said := &strings.Builder{}
	enc, bus := fightStubLogging(t, said)

	entry := entered(playerA, nameA, regionID, regName)
	entry.ID = ""
	act(t, enc, entry)

	if got := allEvents(t, bus); len(got) != 0 {
		t.Errorf("%d events for an entry without an id", len(got))
	}
	if !saidAt(said.String(), "ERROR", "an action without an id") {
		t.Errorf("an action without an id is a defect and is said out loud; the log says:\n%s", said)
	}

	// The same inside a fight that is on: a swing whose end would be derived
	// from it.
	openFight(t, enc, bus)
	before := len(allEvents(t, bus))
	blow := attack(playerA, nameA, wolfID)
	blow.ID = ""
	act(t, enc, blow)
	if after := len(allEvents(t, bus)); after != before {
		t.Errorf("%d events for a swing without an id", after-before)
	}
}

// TestAFactWithoutAnIDDoesNotCloseTheFight is probe RG: the death of the wolf
// arrives in a fact without an id. The closure of the fight would derive its
// end from that fact, so it is not offered — and the log says why the fight
// stays open over a dead wolf.
func TestAFactWithoutAnIDDoesNotCloseTheFight(t *testing.T) {
	said := &strings.Builder{}
	enc, bus := fightStubLogging(t, said)
	openFight(t, enc, bus)

	fact := died(wolfID)
	fact.ID = ""
	observe(t, enc, fact)

	if got := ofType(eventsOf(t, bus, eventbus.TopicSystemEvents), swarm.TypeUpdateProposed); len(got) != 0 {
		t.Errorf("%d packages closed a fight on a fact without an id", len(got))
	}
	if !saidAt(said.String(), "ERROR", "a fact without an id") {
		t.Errorf("a fight left open on a fact without an id is said out loud; the log says:\n%s", said)
	}
	if _, open := enc.ActiveEncounter(playerA); !open {
		t.Error("the fight closed on a fact it could not derive its end from")
	}
}

// --- p. 7: the mark of the exchange ---

// TestEveryDecisionSaysWhereItStandsInItsExchange: the agent knows the shape of
// its exchange and says it, so that the narrator does not have to guess it.
// Exactly one decision of an exchange is the last.
func TestEveryDecisionSaysWhereItStandsInItsExchange(t *testing.T) {
	type mark struct {
		index int
		last  bool
	}
	cases := []struct {
		name  string
		world []func(*entity.Entity)
		play  func(t *testing.T) eventbus.Event
		want  []mark
	}{
		{
			name: "a blow answered by the wolf",
			play: func(t *testing.T) eventbus.Event {
				return attackWhere(t, func(id string) bool { return tkmech.Verdict(id, 0) == tkmech.VerdictHit })
			},
			want: []mark{{0, false}, {1, true}},
		},
		{
			name:  "a killing blow",
			world: []func(*entity.Entity){attr(wolfID, entity.AttrHP, 1)},
			play: func(t *testing.T) eventbus.Event {
				return attackWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) })
			},
			want: []mark{{0, true}},
		},
		{
			name: "an escape",
			play: func(t *testing.T) eventbus.Event {
				return fleeWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) })
			},
			want: []mark{{0, true}},
		},
		{
			name: "a flight caught by a strike out of turn",
			play: func(t *testing.T) eventbus.Event {
				return fleeWhere(t, func(id string) bool {
					return !hits(tkmech.Verdict(id, 0)) && hits(tkmech.Verdict(id, 1))
				})
			},
			want: []mark{{0, false}, {1, true}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc, bus := fightStubWith(t, tc.world...)
			openFight(t, enc, bus)

			act(t, enc, tc.play(t))

			decided := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided)
			if len(decided) != len(tc.want) {
				t.Fatalf("%d decisions, want %d", len(decided), len(tc.want))
			}
			for i, ev := range decided {
				if _, has := ev.Payload["exchange"]; !has {
					t.Fatalf("decision %d says nothing of its exchange", i)
				}
				index, _ := ev.Path().GetInt("exchange.index")
				last, _ := ev.Path().GetBool("exchange.last")
				if got := (mark{index, last}); got != tc.want[i] {
					t.Errorf("decision %d is marked %+v, want %+v", i, got, tc.want[i])
				}
			}
		})
	}
}

// --- helpers of this file ---

// enteredAs is an entry into the forest by a character of the caller's choice,
// in that character's own solo scope.
func enteredAs(id, name string) eventbus.Event {
	return eventbus.NewRoot(swarm.TypeEnteredRegion, "testkit/gateway", worldID,
		&eventbus.ScopeRef{ID: id, Type: "solo"}, entity.ActorKindCI, map[string]any{
			"entity": actor(id, name),
			"target": map[string]any{
				"entity": map[string]any{"id": regionID, "type": entity.TypeRegion},
				"name":   regName,
			},
			"position": map[string]any{"from": nil, "to": regionID},
		})
}

// saidAt says whether one line of a text log carries both a level and a
// message.
func saidAt(log, level, message string) bool {
	for line := range strings.Lines(log) {
		if strings.Contains(line, "level="+level) && strings.Contains(line, message) {
			return true
		}
	}
	return false
}

// spyMechanics decides as the mechanics it wraps and remembers the fighters it
// was handed: what the stub believed about participation when it decided.
type spyMechanics struct {
	inner swarm.Mechanics
	mu    sync.Mutex
	calls []resolution
}

type resolution struct {
	action mech.Action
	actors map[string]mech.Actor
}

func (s *spyMechanics) Resolve(cause string, start int, a mech.Action,
	actors map[string]*mech.Actor) (mech.Outcome, []mech.Roll, error) {
	seen := make(map[string]mech.Actor, len(actors))
	for id, who := range actors {
		seen[id] = *who
	}
	s.mu.Lock()
	s.calls = append(s.calls, resolution{action: a, actors: seen})
	s.mu.Unlock()
	return s.inner.Resolve(cause, start, a, actors)
}

// last is the latest decision of one kind the stub asked for.
func (s *spyMechanics) last(t *testing.T, kind string) resolution {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.calls) - 1; i >= 0; i-- {
		if s.calls[i].action.Kind == kind {
			return s.calls[i]
		}
	}
	t.Fatalf("the stub never asked the mechanics to decide a %s", kind)
	return resolution{}
}

// fightStubSpying is the stub over the fixtures, deciding through a spy and
// logging into said when it is given.
func fightStubSpying(t *testing.T, said *strings.Builder,
	changes ...func(*entity.Entity)) (*swarm.FakeEncounter, *membus.Bus, *spyMechanics) {
	t.Helper()
	bus := newBus(t)
	fixed := mechanics(t)
	spy := &spyMechanics{inner: fixed}
	out := io.Discard
	if said != nil {
		out = said
	}
	enc, err := swarm.NewFakeEncounter(swarm.EncounterConfig{
		Bus: bus, WorldID: worldID, Rules: fixed.Rules(), Mechanics: spy,
		Log: slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		t.Fatalf("encounter: %v", err)
	}
	world := mustFixtures(t)
	for _, change := range changes {
		for _, e := range world {
			change(e)
		}
	}
	if err := enc.Seed(world); err != nil {
		t.Fatalf("seed: %v", err)
	}
	answeredBy(t, bus, world)
	return enc, bus, spy
}
