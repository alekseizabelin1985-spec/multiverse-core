package swarm_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
	tkmech "multiverse-core.io/shared/testkit/mechanics"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

const (
	worldID  = "dark-forest-world"
	regionID = "dark-forest-01"
	playerA  = "player-A"
	nameA    = "Вася"
)

// --- what the stub tells about, and what it does not ---

// TestTheTableIsTheWholeTable states out loud the six occasions C-05 gives the
// narrator («Заглушка»), so that a type quietly added to the table has to be
// added here too — and a type quietly dropped is noticed.
func TestTheTableIsTheWholeTable(t *testing.T) {
	tells := map[string]string{
		swarm.TypeEnteredRegion:    swarm.KindEntry,
		swarm.TypeLooked:           swarm.KindEntry, // C-05 v1.2: a look is not a turn
		swarm.TypeEncounterStarted: swarm.KindWorldEvent,
		swarm.TypeCombatDecided:    swarm.KindTurn,
		swarm.TypeUpdated:          swarm.KindDeath,
		swarm.TypeRoundClosed:      swarm.KindRound,
	}
	// Every type of the registry is asked, rather than a list of types written
	// out here, so that no type can be added to the table without a line in
	// tells. A type outside the registry needs no line: the bus refuses to
	// carry one at all, so an entry for it in the table could never be reached.
	answered := 0
	for _, spec := range contracts.All() {
		kind, ok := swarm.KindFor(spec.Type)
		want, listed := tells[spec.Type]
		if ok != listed || kind != want {
			t.Errorf("the narrator answers %s with %q (answers=%v); "+
				"C-05 says %q (answers=%v)", spec.Type, kind, ok, want, listed)
		}
		if ok {
			answered++
		}
	}
	if answered != len(tells) {
		t.Errorf("%d types of the registry are answered, the test knows %d",
			answered, len(tells))
	}
}

// TestTheKindsAreTheKindsOfTheSchema ties the kinds the narrator writes to the
// enum of narrative.output.v1.json, both ways: a kind the schema knows and the
// narrator never writes is a delivery kind of EPIC-004 nobody feeds, and a kind
// the narrator writes and the schema does not know is a dead letter.
func TestTheKindsAreTheKindsOfTheSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "events", "narrative.output.v1.json"))
	if err != nil {
		t.Fatalf("read the schema: %v", err)
	}
	var schema struct {
		Properties struct {
			Kind struct {
				Enum []string `json:"enum"`
			} `json:"kind"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode the schema: %v", err)
	}
	written := map[string]struct{}{}
	for _, spec := range contracts.All() {
		if kind, ok := swarm.KindFor(spec.Type); ok {
			written[kind] = struct{}{}
		}
	}
	have := slices.Sorted(func(yield func(string) bool) {
		for kind := range written {
			if !yield(kind) {
				return
			}
		}
	})
	want := slices.Sorted(slices.Values(schema.Properties.Kind.Enum))
	if !slices.Equal(have, want) {
		t.Errorf("the narrator writes the kinds %v, the schema knows %v", have, want)
	}
}

// TestTheDefaultLawsIsTheOneTheFixtureWorldLivesUnder ties the constant to the
// fixture it says it comes from. The stub does not read world.laws.changed, so
// nothing else connects the two: a fixture world that changed its laws would
// otherwise leave every narrative carrying a version nobody lives under
// (review T-018 Nit-2).
func TestTheDefaultLawsIsTheOneTheFixtureWorldLivesUnder(t *testing.T) {
	fixtures, err := state.LoadFixtures(filepath.Join("..", "..", "..", "testdata", "fixtures"))
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	for _, e := range fixtures {
		if e.ID != worldID {
			continue
		}
		laws, ok := e.LawsVersion()
		if !ok {
			t.Fatalf("the fixture world %s names no laws_version", worldID)
		}
		if laws != swarm.DefaultLawsVersion {
			t.Errorf("the fixture world lives under laws %q, the narrator writes under %q",
				laws, swarm.DefaultLawsVersion)
		}
		return
	}
	t.Fatalf("%s is not in the fixtures", worldID)
}

// --- every occasion, on the wire ---

// TestEveryOccasionOfTheContractIsTold drives the narrator through the six
// occasions of C-05 in one run and holds every narrative it writes to what the
// contract promises of all of them: the kind of its occasion, a valid event of
// the registry, narrative_event_id equal to the id of the event, the template
// said out loud, and recipients who are the players of the scope and nobody
// else.
func TestEveryOccasionOfTheContractIsTold(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)

	entry := entered(playerA, nameA, regionID, regName)
	swing := attack(playerA, nameA, wolfID)
	occasions := []struct {
		ev   eventbus.Event
		fact bool
		kind string
	}{
		{ev: entry, kind: swarm.KindEntry},
		{ev: look(playerA, nameA), kind: swarm.KindEntry},
		{ev: encounterStarted(entry, actor(playerA, nameA)), kind: swarm.KindWorldEvent},
		{ev: decided(swing, strike("attack", playerA, wolfID, true, 6, 0)), kind: swarm.KindTurn},
		{ev: playerDied(playerA), fact: true, kind: swarm.KindDeath},
		{ev: roundClosed(2, []string{playerA}, nil, nil), kind: swarm.KindRound},
	}
	for _, o := range occasions {
		if o.fact {
			learnFact(t, n, o.ev)
		} else {
			narrate(t, n, o.ev)
		}
	}

	got := narratives(t, bus)
	if len(got) != len(occasions) {
		t.Fatalf("%d narratives for %d occasions", len(got), len(occasions))
	}
	spec, known := contracts.Lookup(swarm.TypeNarrativeOutput)
	if !known || !slices.Contains(spec.Publishers, swarm.Source) {
		t.Fatalf("%q may not publish %s (known=%v)", swarm.Source, swarm.TypeNarrativeOutput, known)
	}
	for i, ev := range got {
		o := occasions[i]
		pa := ev.Path()
		if kind, _ := pa.GetString("kind"); kind != o.kind {
			t.Errorf("the narrative of %s is kind %q, want %q", o.ev.Type, kind, o.kind)
		}
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("the narrative of %s is not valid: %v", o.ev.Type, err)
		}
		if id, _ := pa.GetString("narrative_event_id"); id != ev.ID {
			t.Errorf("the narrative of %s carries narrative_event_id %q, its id is %q",
				o.ev.Type, id, ev.ID)
		}
		if by, _ := pa.GetString("generated_by"); by != "template" {
			t.Errorf("the narrative of %s was generated_by %q", o.ev.Type, by)
		}
		if why, _ := pa.GetString("fallback_reason"); why != swarm.FallbackReason {
			t.Errorf("the narrative of %s gives fallback_reason %q, want %q",
				o.ev.Type, why, swarm.FallbackReason)
		}
		if ev.Meta.CausationID != o.ev.ID {
			t.Errorf("the narrative of %s is caused by %q, want %q", o.ev.Type, ev.Meta.CausationID, o.ev.ID)
		}
		if on := basedOn(t, ev); len(on) == 0 || on[len(on)-1] != o.ev.ID {
			t.Errorf("the narrative of %s is based on %v", o.ev.Type, on)
		}
		if to := recipientsOf(t, ev); !slices.Equal(to, []string{playerA}) {
			t.Errorf("the narrative of %s is addressed to %v, the scope is %s", o.ev.Type, to, playerA)
		}
		if text, _ := pa.GetString("text"); strings.ContainsAny(text, "<*_{}") {
			t.Errorf("the narrative of %s carries markup or a placeholder: %q", o.ev.Type, text)
		}
	}
}

// --- the entry narrative ---

func TestALookBecomesAnEntryNarrative(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)

	narrate(t, n, look(playerA, nameA))

	ev := onlyNarrative(t, bus)
	pa := ev.Path()
	if kind, _ := pa.GetString("kind"); kind != swarm.KindEntry {
		t.Errorf("kind %q, want %q: C-05 v1.2, a look is not a turn", kind, swarm.KindEntry)
	}
	if name, _ := pa.GetString("recipients[0].name"); name != nameA {
		t.Errorf("the recipient is named %q, want %q", name, nameA)
	}
	if version, _ := pa.GetString("laws_version"); version != swarm.DefaultLawsVersion {
		t.Errorf("laws_version %q, want %q", version, swarm.DefaultLawsVersion)
	}
	if locale, _ := pa.GetString("locale"); locale != eventbus.DefaultLocale {
		t.Errorf("locale %q, want %q", locale, eventbus.DefaultLocale)
	}
}

// TestTheFilterBlockSaysNoFilterRan is the block C-05 requires on every
// narrative. The stub has no filter, and says so with values rather than by
// leaving the block out.
func TestTheFilterBlockSaysNoFilterRan(t *testing.T) {
	n, bus := running(t)
	narrate(t, n, look(playerA, nameA))

	pa := onlyNarrative(t, bus).Path()
	if applied, _ := pa.GetBool("filter.applied"); applied {
		t.Error("filter.applied is true; the stub runs no filter")
	}
	if status, _ := pa.GetString("filter.status"); status != "pass" {
		t.Errorf("filter.status %q, want pass", status)
	}
	if version, _ := pa.GetString("filter.filter_version"); version != swarm.FilterVersion {
		t.Errorf("filter_version %q, want %q", version, swarm.FilterVersion)
	}
}

// TestTheNarrativeNamesItsAgentAndItsCause pins the two envelope promises the
// swarm policy of C-01 and the trace of C-05 make.
func TestTheNarrativeNamesItsAgentAndItsCause(t *testing.T) {
	n, bus := running(t)
	cause := look(playerA, nameA)
	narrate(t, n, cause)

	ev := onlyNarrative(t, bus)
	if ev.Meta.Agent == nil {
		t.Fatal("no meta.agent: narrative.output is a swarm type")
	}
	if *ev.Meta.Agent != swarm.Agent {
		t.Errorf("agent %+v, want %+v", *ev.Meta.Agent, swarm.Agent)
	}
	if ev.Meta.Agent.Level != "task" {
		t.Errorf("agent level %q, want task (C-05)", ev.Meta.Agent.Level)
	}
	if ev.Meta.CorrelationID != cause.Meta.CorrelationID {
		t.Errorf("correlates to %q, the action to %q",
			ev.Meta.CorrelationID, cause.Meta.CorrelationID)
	}
	typ, _ := ev.Path().GetString("based_on[0].event.type")
	if typ != cause.Type {
		t.Errorf("based_on names a %s, want %s", typ, cause.Type)
	}
}

// TestTheTextCarriesWhatStateSaid is why the narrator reads system_events at
// all: a text about a character says how that character is doing, and only
// State knows.
func TestTheTextCarriesWhatStateSaid(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)
	learnFact(t, n, updated(playerA, entity.AttrHP, 4))

	narrate(t, n, look(playerA, nameA))

	text, _ := onlyNarrative(t, bus).Path().GetString("text")
	if !strings.Contains(text, nameA) {
		t.Errorf("the text does not name the character: %q", text)
	}
	if !strings.Contains(text, strconv.Itoa(4)) {
		t.Errorf("the text does not carry the health State published: %q", text)
	}
}

// TestACharacterTheNarratorNeverHeardOfIsStillWrittenAbout is the degradation
// the package doc promises: a missing number is a gap in the text, not a
// missing text. A player who gets nothing at all has no way to play on.
func TestACharacterTheNarratorNeverHeardOfIsStillWrittenAbout(t *testing.T) {
	n, bus := running(t)

	narrate(t, n, look(playerA, nameA))

	text, _ := onlyNarrative(t, bus).Path().GetString("text")
	if !strings.Contains(text, nameA) {
		t.Errorf("the name of the action was not used as the fallback: %q", text)
	}
}

// TestAnEntryNamesThePlaceItHappensIn covers the other substitution an entry
// makes: where the character is.
func TestAnEntryNamesThePlaceItHappensIn(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)

	narrate(t, n, entered(playerA, nameA, regionID, regName))

	text, _ := onlyNarrative(t, bus).Path().GetString("text")
	// Not every template names the place, so the assertion is that the
	// placeholder is gone, not that the name is present.
	if strings.Contains(text, "{") {
		t.Errorf("a placeholder was left unsubstituted: %q", text)
	}
}

// --- the world event ---

// TestAnEncounterBecomesAWorldEventForItsCharacters: the text of an encounter
// goes to the characters it was opened around, and an NPC named among the
// participants is not somebody a text is delivered to.
func TestAnEncounterBecomesAWorldEventForItsCharacters(t *testing.T) {
	n, bus := running(t)
	started := encounterStarted(entered(playerA, nameA, regionID, regName),
		actor(playerA, nameA), fighter(wolfID), actor("player-B", "Лена"))

	narrate(t, n, started)

	ev := onlyNarrative(t, bus)
	if kind, _ := ev.Path().GetString("kind"); kind != swarm.KindWorldEvent {
		t.Errorf("kind %q, want %q", kind, swarm.KindWorldEvent)
	}
	if to := recipientsOf(t, ev); !slices.Equal(to, []string{playerA, "player-B"}) {
		t.Errorf("addressed to %v, want the two characters and not the wolf", to)
	}
	text, _ := ev.Path().GetString("text")
	if !strings.Contains(text, wolfName) {
		t.Errorf("the text does not say what came out of the forest: %q", text)
	}
	if on := basedOn(t, ev); !slices.Equal(on, []string{started.ID}) {
		t.Errorf("based on %v, want the encounter %s", on, started.ID)
	}
}

// TestAnEncounterAroundNoCharacterIsNotTold: recipients has minItems 1, and an
// encounter of NPCs alone has nobody to be told to.
func TestAnEncounterAroundNoCharacterIsNotTold(t *testing.T) {
	n, bus := running(t)

	narrate(t, n, encounterStarted(entered(playerA, nameA, regionID, regName), fighter(wolfID)))

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives about an encounter nobody is in", len(got))
	}
}

// --- the turn ---

// TestATurnIsToldAfterItsLastDecision is the rule of closesExchange, one case
// per way an exchange of C-05 can go: the turn is told once, after the last of
// its decisions and not before, and is based on all of them in order.
func TestATurnIsToldAfterItsLastDecision(t *testing.T) {
	cases := []struct {
		name      string
		decisions []map[string]any
		says      []string
	}{
		{
			name: "a blow that did not kill is answered by the wolf",
			decisions: []map[string]any{
				strike("attack", playerA, wolfID, true, 4, 6),
				strike("npc_attack", wolfID, playerA, true, 2, 8),
			},
			says: []string{"4", "2", "8", wolfName},
		},
		{
			name: "a miss is answered too",
			decisions: []map[string]any{
				strike("attack", playerA, wolfID, false, 0, 10),
				strike("npc_attack", wolfID, playerA, false, 0, 10),
			},
			says: []string{wolfName},
		},
		{
			name:      "a killing blow is the whole turn",
			decisions: []map[string]any{strike("attack", playerA, wolfID, true, 6, 0)},
			says:      []string{wolfName},
		},
		{
			name:      "an escape is the whole turn",
			decisions: []map[string]any{flight(true, false, 1)},
			says:      []string{wolfName},
		},
		{
			name: "a caught flight waits for the strike out of turn",
			decisions: []map[string]any{
				flight(false, true, 1),
				freeAttack(3, 7),
			},
			says: []string{"3", "7", wolfName},
		},
		// The two failed flights nothing struck at are told without a foe:
		// TestAFlightNothingStruckAtClaimsNoBlow holds what they must not say.
		{
			name:      "a failed flight the rules do not punish is the whole turn",
			decisions: []map[string]any{flight(false, false, 1)},
			says:      []string{nameA},
		},
		{
			name:      "a failed flight with nobody left standing is the whole turn",
			decisions: []map[string]any{flight(false, true, 0)},
			says:      []string{nameA},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, bus := running(t)
			knows(t, n, playerA, nameA, 10, 10)
			entry := entered(playerA, nameA, regionID, regName)
			narrate(t, n, encounterStarted(entry, actor(playerA, nameA)))

			cause := attack(playerA, nameA, wolfID)
			ids := make([]string, 0, len(tc.decisions))
			for i, payload := range tc.decisions {
				ev := decided(cause, payload)
				ids = append(ids, ev.ID)
				narrate(t, n, ev)
				told := ofKind(narratives(t, bus), swarm.KindTurn)
				want := 0
				if i == len(tc.decisions)-1 {
					want = 1
				}
				if len(told) != want {
					t.Fatalf("after decision %d of %d: %d turns told, want %d",
						i+1, len(tc.decisions), len(told), want)
				}
			}

			turn := ofKind(narratives(t, bus), swarm.KindTurn)[0]
			if on := basedOn(t, turn); !slices.Equal(on, ids) {
				t.Errorf("the turn is based on %v, want every decision of it %v", on, ids)
			}
			if to := recipientsOf(t, turn); !slices.Equal(to, []string{playerA}) {
				t.Errorf("the turn is addressed to %v, the scope is %s", to, playerA)
			}
			text, _ := turn.Path().GetString("text")
			for _, s := range tc.says {
				if !strings.Contains(text, s) {
					t.Errorf("the turn does not say %q: %q", s, text)
				}
			}
		})
	}
}

// TestATurnClosesOnTheMarkOfItsExchange is C-05 v1.4 p. 7 on the side of the
// narrator: a publisher that says where a decision stands in its exchange is
// taken at its word over the guess from the fields of the decision. Each case
// is one the guess would get wrong, so a narrator that went on guessing would
// tell the turn at the wrong decision.
func TestATurnClosesOnTheMarkOfItsExchange(t *testing.T) {
	cases := []struct {
		name      string
		decisions []map[string]any
	}{
		{
			name: "a blow that did not kill, marked as the whole exchange",
			decisions: []map[string]any{
				marked(strike("attack", playerA, wolfID, true, 4, 6), 0, true),
			},
		},
		{
			name: "a killing blow the publisher says is answered",
			decisions: []map[string]any{
				marked(strike("attack", playerA, wolfID, true, 6, 0), 0, false),
				marked(strike("npc_attack", wolfID, playerA, false, 0, 10), 1, true),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, bus := running(t)
			cause := attack(playerA, nameA, wolfID)
			ids := make([]string, 0, len(tc.decisions))
			for i, payload := range tc.decisions {
				ev := decided(cause, payload)
				ids = append(ids, ev.ID)
				narrate(t, n, ev)
				want := 0
				if i == len(tc.decisions)-1 {
					want = 1
				}
				if told := ofKind(narratives(t, bus), swarm.KindTurn); len(told) != want {
					t.Fatalf("after decision %d of %d: %d turns told, want %d",
						i+1, len(tc.decisions), len(told), want)
				}
			}
			turn := ofKind(narratives(t, bus), swarm.KindTurn)[0]
			if on := basedOn(t, turn); !slices.Equal(on, ids) {
				t.Errorf("the turn is based on %v, want every decision of it %v", on, ids)
			}
		})
	}
}

// marked is a decision that says where it stands in its exchange.
func marked(p map[string]any, index int, last bool) map[string]any {
	p["exchange"] = map[string]any{"index": index, "last": last}
	return p
}

// TestEveryTurnOfTheFightIsToldOnce holds the rule of the last decision to the
// only publisher of combat.decided there is in Phase 1: every action the real
// FakeEncounter answers is told as exactly one turn, based on exactly the
// decisions that answered it. A rule that told after the first of two
// decisions would tell one action twice; a rule that waited for a decision
// that never comes would tell it never.
func TestEveryTurnOfTheFightIsToldOnce(t *testing.T) {
	cases := []struct {
		name    string
		changes []func(*entity.Entity)
		play    func(t *testing.T, enc *swarm.FakeEncounter, bus *membus.Bus)
	}{
		{name: "the wolf falls", play: swingUntilOver},
		{
			name: "the character falls",
			changes: []func(*entity.Entity){attr(playerA, entity.AttrHP, 1),
				attr(wolfID, entity.AttrHP, 40), attr(wolfID, entity.AttrHPMax, 40)},
			play: swingUntilOver,
		},
		{
			name: "a flight is caught, then the character gets away",
			play: func(t *testing.T, enc *swarm.FakeEncounter, _ *membus.Bus) {
				act(t, enc, fleeWhere(t, func(id string) bool {
					return !hits(tkmech.Verdict(id, 0)) && hits(tkmech.Verdict(id, 1))
				}))
				act(t, enc, fleeWhere(t, func(id string) bool { return hits(tkmech.Verdict(id, 0)) }))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc, bus := fightStubWith(t, tc.changes...)
			n, err := swarm.NewFakeNarrator(bus, worldID)
			if err != nil {
				t.Fatalf("narrator: %v", err)
			}
			openFight(t, enc, bus)
			tc.play(t, enc, bus)

			// Everything the fight published, in the order it was published on
			// each topic — which is what the subscriptions would deliver.
			for _, topic := range []string{eventbus.TopicWorldEvents, eventbus.TopicGameEvents} {
				for _, ev := range eventsOf(t, bus, topic) {
					narrate(t, n, ev)
				}
			}

			var actions []string
			byAction := map[string][]string{}
			for _, ev := range ofType(eventsOf(t, bus, eventbus.TopicGameEvents), swarm.TypeCombatDecided) {
				cause := ev.Meta.CausationID
				if _, seen := byAction[cause]; !seen {
					actions = append(actions, cause)
				}
				byAction[cause] = append(byAction[cause], ev.ID)
			}
			if len(actions) == 0 {
				t.Fatal("the fight answered no action: the test proves nothing")
			}
			turns := ofKind(narratives(t, bus), swarm.KindTurn)
			if len(turns) != len(actions) {
				t.Fatalf("%d turns told for %d actions the fight answered", len(turns), len(actions))
			}
			for i, turn := range turns {
				if on := basedOn(t, turn); !slices.Equal(on, byAction[actions[i]]) {
					t.Errorf("turn %d is based on %v, the action was answered by %v",
						i+1, on, byAction[actions[i]])
				}
			}
		})
	}
}

// TestAGroupDecisionIsNotAToldTurn: a group hears one text per round, and it
// is written on round.closed (C-05). A turn per decision on top of it would
// tell the same blow twice, to people who were not all in it.
func TestAGroupDecisionIsNotAToldTurn(t *testing.T) {
	n, bus := running(t)
	cause := actionIn(&eventbus.ScopeRef{ID: "group-1", Type: "group"})

	narrate(t, n, decided(cause, strike("attack", playerA, wolfID, true, 6, 0)))

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives for a decision of a group", len(got))
	}
}

// TestADecisionOfNobodyInItsScopeIsNotTold: the recipient of a solo turn is the
// player the scope is, and a decision that names another character has nobody
// in that scope to be told to.
func TestADecisionOfNobodyInItsScopeIsNotTold(t *testing.T) {
	n, bus := running(t)
	cause := actionIn(&eventbus.ScopeRef{ID: "player-B", Type: "solo"})

	narrate(t, n, decided(cause, strike("attack", playerA, wolfID, true, 6, 0)))

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives to a scope the decision is not about", len(got))
	}
}

// TestATurnNeverClosedIsDroppedUntold: a decision of a new action means the
// turn before it is not coming back. Its decisions are dropped rather than
// folded into the next turn, which would tell one action with the blows of
// another.
func TestATurnNeverClosedIsDroppedUntold(t *testing.T) {
	n, bus := running(t)
	first := attack(playerA, nameA, wolfID)
	narrate(t, n, decided(first, strike("attack", playerA, wolfID, true, 4, 6)))

	second := attack(playerA, nameA, wolfID)
	kill := decided(second, strike("attack", playerA, wolfID, true, 6, 0))
	narrate(t, n, kill)

	turn := onlyNarrative(t, bus)
	if on := basedOn(t, turn); !slices.Equal(on, []string{kill.ID}) {
		t.Errorf("the turn of the second action is based on %v, want only %s", on, kill.ID)
	}
}

// --- the death ---

func TestADeathIsToldToTheCharacterWhoDied(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)
	fact := playerDied(playerA)

	learnFact(t, n, fact)

	ev := onlyNarrative(t, bus)
	if kind, _ := ev.Path().GetString("kind"); kind != swarm.KindDeath {
		t.Errorf("kind %q, want %q", kind, swarm.KindDeath)
	}
	if to := recipientsOf(t, ev); !slices.Equal(to, []string{playerA}) {
		t.Errorf("addressed to %v, want the character who died", to)
	}
	if text, _ := ev.Path().GetString("text"); !strings.Contains(text, nameA) {
		t.Errorf("the text does not name who died: %q", text)
	}
	if on := basedOn(t, ev); !slices.Equal(on, []string{fact.ID}) {
		t.Errorf("based on %v, want the fact %s", on, fact.ID)
	}
}

// TestAForgottenCharacterIsNotToldAsADeath is sweep 3 of C-02 v1.2:
// status=abandoned is terminal like dead and consumers treat it like one for
// scope, targets and rounds — but narrative.output kind=death is not
// generated for it.
func TestAForgottenCharacterIsNotToldAsADeath(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)

	learnFact(t, n, forgotten(playerA))

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives for a character their player forgot; C-02 v1.2 says none", len(got))
	}
}

// TestTheDeathOfAnNPCIsNotACharacterDeath: kind=death is the death of the
// player's own character. A wolf that falls is told as the turn that felled
// it.
func TestTheDeathOfAnNPCIsNotACharacterDeath(t *testing.T) {
	n, bus := running(t)

	learnFact(t, n, died(wolfID))

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives for the death of an NPC", len(got))
	}
}

// TestAFactThatKillsNobodyTellsNothing: a wound is a fact the narrator learns
// from, not an occasion to write.
func TestAFactThatKillsNobodyTellsNothing(t *testing.T) {
	n, bus := running(t)

	learnFact(t, n, updated(playerA, entity.AttrHP, 4))
	learnFact(t, n, created(playerA, nameA, 10, 10))

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives for facts that record no death", len(got))
	}
}

func TestADeathInAnotherWorldIsNotTold(t *testing.T) {
	n, bus := running(t)
	fact := playerDied(playerA)
	fact.World = &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: "some-other-world", Type: "world"}}

	learnFact(t, n, fact)

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives about a death in a world this narrator does not serve", len(got))
	}
}

// --- what the narrator passes over ---

// TestTheNarratorHasNoFightInIt is the negative of the DoD: Phase 1 fights
// through FakeEncounter only (C-05), and a narrator that answered an attack
// with dice or a decision would be a second rule book nobody reviews. It is
// asked twice: through the handler, where nothing at all may come out, and
// through the subscriptions, where the look that follows the attack is the
// point by which everything the attack could have caused has been caused.
func TestTheNarratorHasNoFightInIt(t *testing.T) {
	n, bus := running(t)

	narrate(t, n, attack(playerA, nameA, wolfID))
	narrate(t, n, flee(playerA, nameA))
	if got := allEvents(t, bus); len(got) != 0 {
		t.Fatalf("the narrator answered an attack and a flight with %d events", len(got))
	}

	for _, ev := range []eventbus.Event{attack(playerA, nameA, wolfID), look(playerA, nameA)} {
		if err := bus.Publish(t.Context(), ev); err != nil {
			t.Fatalf("publish %s: %v", ev.Type, err)
		}
	}
	waitFor(t, "the narrative of the look", func() bool { return len(narratives(t, bus)) > 0 })
	for _, typ := range []string{swarm.TypeDiceRolled, swarm.TypeCombatDecided} {
		if got := ofType(eventsOf(t, bus, eventbus.TopicGameEvents), typ); len(got) != 0 {
			t.Errorf("%d %s on the bus: the narrator has no fight in it", len(got), typ)
		}
	}
	published := allEvents(t, bus)
	if len(published) != 1 || published[0].Type != swarm.TypeNarrativeOutput {
		types := make([]string, 0, len(published))
		for _, ev := range published {
			types = append(types, ev.Type)
		}
		t.Errorf("the narrator published %v, want only the narrative of the look", types)
	}
}

func TestAnEventOfAnotherWorldIsPassedOver(t *testing.T) {
	n, bus := running(t)

	action := look(playerA, nameA)
	action.World = &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: "some-other-world", Type: "world"}}
	narrate(t, n, action)

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives about a world this narrator does not serve", len(got))
	}
}

func TestATypeTheStubDoesNotTellAboutIsPassedOver(t *testing.T) {
	n, bus := running(t)

	narrate(t, n, said(playerA, nameA))
	// entity.updated is in the table, but as a fact of State: Narrate leaves it
	// to Observe.
	narrate(t, n, playerDied(playerA))

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives about events Narrate does not tell", len(got))
	}
}

func TestAnActionWithoutAnActorIsPassedOver(t *testing.T) {
	n, bus := running(t)

	action := look(playerA, nameA)
	delete(action.Payload, "entity")
	narrate(t, n, action)

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives about nobody", len(got))
	}
}

// TestAnEventDeliveredTwiceIsToldOnce: delivery is at-least-once (C-01), and
// C-05 promises one narrative per turn. A look and a death delivered twice are
// one look and one death.
func TestAnEventDeliveredTwiceIsToldOnce(t *testing.T) {
	n, bus := running(t)
	action := look(playerA, nameA)
	fact := playerDied(playerA)

	narrate(t, n, action)
	narrate(t, n, action)
	learnFact(t, n, fact)
	learnFact(t, n, fact)

	if got := narratives(t, bus); len(got) != 2 {
		t.Errorf("%d narratives for one look and one death, each delivered twice", len(got))
	}
}

// --- the round narrative ---

func TestARoundClosedBecomesOneTextForEveryoneInIt(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)

	narrate(t, n, roundClosed(3,
		[]string{playerA}, []string{"player-B"}, []string{"player-C", playerA}))

	ev := onlyNarrative(t, bus)
	pa := ev.Path()
	if kind, _ := pa.GetString("kind"); kind != swarm.KindRound {
		t.Errorf("kind %q, want %q", kind, swarm.KindRound)
	}
	if seq, _ := pa.GetInt("round.seq"); seq != 3 {
		t.Errorf("round.seq %d, want 3", seq)
	}
	want := []string{playerA, "player-B", "player-C"}
	if got := recipientsOf(t, ev); !slices.Equal(got, want) {
		t.Errorf("recipients are %v, want %v: acted, then auto-defended, then idle, once each",
			got, want)
	}
	text, _ := pa.GetString("text")
	if !strings.Contains(text, "3") {
		t.Errorf("the text does not name the round: %q", text)
	}
	if !strings.Contains(text, nameA) {
		t.Errorf("the text does not name the character the narrator knows: %q", text)
	}
}

// TestARoundWithNobodyInItIsNotNarrated: recipients has minItems 1, so a text
// for nobody would be a schema violation dressed up as a narrative.
func TestARoundWithNobodyInItIsNotNarrated(t *testing.T) {
	n, bus := running(t)

	narrate(t, n, roundClosed(1, nil, nil, nil))

	if narratives := narratives(t, bus); len(narratives) != 0 {
		t.Errorf("%d narratives for a round nobody was in", len(narratives))
	}
}

// --- the life cycle ---

func TestNewFakeNarratorRefusesWhatItCannotServe(t *testing.T) {
	if _, err := swarm.NewFakeNarrator(nil, worldID); err == nil {
		t.Error("a narrator without a bus was built")
	}
	if _, err := swarm.NewFakeNarrator(newBus(t), ""); err == nil {
		t.Error("a narrator without a world was built")
	}
}

func TestStartTwiceIsRefused(t *testing.T) {
	n, _ := running(t)
	if err := n.Start(t.Context()); err == nil {
		t.Fatal("a second set of subscriptions was opened over the first")
	}
}

func TestACancelledSubscriptionIsNotAFailure(t *testing.T) {
	n, err := swarm.NewFakeNarrator(newBus(t), worldID)
	if err != nil {
		t.Fatalf("narrator: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := n.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	cancel()
	if err := n.Wait(); err != nil {
		t.Errorf("subscriptions their caller cancelled reported %v, want nil (C-01)", err)
	}
	if err := n.Err(); err != nil {
		t.Errorf("Err after a clean stop is %v, want nil", err)
	}
}

// TestErrReportsADeadSubscriptionWithoutWaiting is question 6 of the review of
// T-219: the failure of a subscription has to be readable while the others are
// still running, because that is when /health asks. One of the four
// subscriptions is refused and three go on; Wait would block for as long as
// they run, Err answers now.
func TestErrReportsADeadSubscriptionWithoutWaiting(t *testing.T) {
	bus := refusingBus{Bus: newBus(t), refuse: swarm.Group + "-" + eventbus.TopicWorldEvents}
	n, err := swarm.NewFakeNarrator(bus, worldID)
	if err != nil {
		t.Fatalf("narrator: %v", err)
	}
	if err := n.Err(); err != nil {
		t.Fatalf("a narrator that has not started reports %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = n.Wait() })
	if err := n.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	waitFor(t, "the refused subscription to be reported", func() bool { return n.Err() != nil })
	if err := n.Err(); !errors.Is(err, errRefused) || !strings.Contains(err.Error(), eventbus.TopicWorldEvents) {
		t.Errorf("Err is %v, want the refusal of %s", err, eventbus.TopicWorldEvents)
	}

	// The other three are alive: the narrator still answers a look.
	if err := bus.Publish(t.Context(), look(playerA, nameA)); err != nil {
		t.Fatalf("publish: %v", err)
	}
	waitFor(t, "the narrative of the look", func() bool { return len(narratives(t, bus.Bus)) > 0 })
}

// TestTheNarratorWorksOffTheBusToo runs it the way a scenario does — through
// the subscriptions rather than by calling the handler — so that the topics it
// listens on are covered as well as the decisions it makes.
func TestTheNarratorWorksOffTheBusToo(t *testing.T) {
	_, bus := running(t)

	if err := bus.Publish(t.Context(), created(playerA, nameA, 7, 10)); err != nil {
		t.Fatalf("publish the fact: %v", err)
	}
	if err := bus.Publish(t.Context(), look(playerA, nameA)); err != nil {
		t.Fatalf("publish the action: %v", err)
	}

	var text string
	waitFor(t, "the narrative of the look", func() bool {
		got := narratives(t, bus)
		if len(got) == 0 {
			return false
		}
		text, _ = got[0].Path().GetString("text")
		return true
	})
	// The fact was published before the action, and both subscriptions read
	// their topic from the first offset, so the health in the text is the one
	// State gave. This is what an ordering the narrator does not control looks
	// like when it goes well; it is not a guarantee the stub makes.
	if !strings.Contains(text, "7") && !strings.Contains(text, "—") {
		t.Errorf("the text carries neither the health nor the gap: %q", text)
	}
}

// --- the world of the tests ---

func running(t *testing.T) (*swarm.FakeNarrator, *membus.Bus) {
	t.Helper()
	bus := newBus(t)
	n, err := swarm.NewFakeNarrator(bus, worldID)
	if err != nil {
		t.Fatalf("narrator: %v", err)
	}
	n.WithLaws("").WithLog(nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = n.Wait() })
	if err := n.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	return n, bus
}

func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	testkit.Deterministic(t, "sw")
	return plainBus(t)
}

// plainBus is a bus over the registry without touching the identifier
// sequence, for a test that has chosen its own prefix.
func plainBus(t *testing.T) *membus.Bus {
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

// errRefused is what refusingBus answers a refused subscription with.
var errRefused = errors.New("the broker refused the consumer group")

// refusingBus is a bus that will not serve the consumer groups starting with a
// prefix — a broker that refuses a group is how a subscription dies on its own
// rather than by being cancelled.
type refusingBus struct {
	*membus.Bus
	refuse string
}

func (b refusingBus) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	if strings.HasPrefix(group, b.refuse) {
		return errRefused
	}
	return b.Bus.Subscribe(ctx, topic, group, h)
}

func narrate(t *testing.T, n *swarm.FakeNarrator, ev eventbus.Event) {
	t.Helper()
	if err := n.Narrate(t.Context(), ev); err != nil {
		t.Fatalf("narrate %s: %v", ev.Type, err)
	}
}

func learnFact(t *testing.T, n *swarm.FakeNarrator, ev eventbus.Event) {
	t.Helper()
	if err := n.Observe(t.Context(), ev); err != nil {
		t.Fatalf("observe %s: %v", ev.Type, err)
	}
}

// knows teaches the narrator a character the way State does: with a fact.
func knows(t *testing.T, n *swarm.FakeNarrator, id, name string, hp, hpMax int) {
	t.Helper()
	learnFact(t, n, created(id, name, hp, hpMax))
}

// --- the events of the tests ---

func playerAction(typ string, payload map[string]any) eventbus.Event {
	return eventbus.NewRoot(typ, contracts.SourceTestkitGateway, worldID,
		&eventbus.ScopeRef{ID: playerA, Type: "solo"}, entity.ActorKindCI, payload)
}

// actionIn is an attack of the first character in a scope of the caller's
// choosing.
func actionIn(scope *eventbus.ScopeRef) eventbus.Event {
	return eventbus.NewRoot(swarm.TypeAttacked, contracts.SourceTestkitGateway, worldID,
		scope, entity.ActorKindCI, map[string]any{
			"entity": actor(playerA, nameA),
			"action": map[string]any{"type": "attack"},
			"target": fighter(wolfID),
		})
}

func actor(id, name string) map[string]any {
	return map[string]any{
		"entity": map[string]any{"id": id, "type": entity.TypePlayer},
		"name":   name,
	}
}

// fighter is somebody a decision names: the character of the tests, or the
// wolf.
func fighter(id string) map[string]any {
	if id == playerA {
		return actor(playerA, nameA)
	}
	return map[string]any{"entity": map[string]any{"id": id, "type": entity.TypeNPC}, "name": wolfName}
}

func look(id, name string) eventbus.Event {
	return playerAction(swarm.TypeLooked, map[string]any{
		"entity": actor(id, name),
		"action": map[string]any{"type": "look"},
	})
}

func said(id, name string) eventbus.Event {
	return playerAction("player.said", map[string]any{
		"entity": actor(id, name),
		"text":   "Тише.",
	})
}

func entered(id, name, region, regionName string) eventbus.Event {
	return playerAction(swarm.TypeEnteredRegion, map[string]any{
		"entity": actor(id, name),
		"target": map[string]any{
			"entity": map[string]any{"id": region, "type": entity.TypeRegion},
			"name":   regionName,
		},
		"position": map[string]any{"from": nil, "to": region},
	})
}

// encounterStarted is the encounter the world opens in answer to an entry,
// derived from it as FakeEncounter derives it.
func encounterStarted(cause eventbus.Event, participants ...map[string]any) eventbus.Event {
	return eventbus.Derive(cause, swarm.TypeEncounterStarted, swarm.Source, map[string]any{
		"encounter": map[string]any{"entity": map[string]any{"id": "encounter-1", "type": entity.TypeEncounter}},
		"region": map[string]any{
			"entity": map[string]any{"id": regionID, "type": entity.TypeRegion},
			"name":   regName,
		},
		"participants": participants,
		"npcs":         []map[string]any{fighter(wolfID)},
	}, eventbus.WithAgent(eventbus.AgentRef{ID: "fake-encounter:solo:" + playerA,
		Level: swarm.AgentLevelDomain, Blueprint: swarm.EncounterBlueprint, BlueprintVersion: "0.0.0"}))
}

// decided is a combat.decided answering an action, derived from it the way
// FakeEncounter derives it: the action is its cause, and so is the turn it
// belongs to.
func decided(cause eventbus.Event, payload map[string]any) eventbus.Event {
	return eventbus.Derive(cause, swarm.TypeCombatDecided, swarm.Source, payload,
		eventbus.WithAgent(eventbus.AgentRef{ID: "fake-encounter:solo:" + playerA,
			Level: swarm.AgentLevelTask, Blueprint: swarm.EncounterBlueprint, BlueprintVersion: "0.0.0"}))
}

// decisionBase is what every combat.decided carries whatever the action.
func decisionBase(action, by string) map[string]any {
	return map[string]any{
		"encounter":     map[string]any{"entity": map[string]any{"id": "encounter-1", "type": entity.TypeEncounter}},
		"round":         map[string]any{"seq": 1},
		"action":        action,
		"attacker":      fighter(by),
		"rolls":         []map[string]any{},
		"rules_version": "0.1",
		"phase1_mode":   swarm.Phase1Mode,
	}
}

// strike is a blow of one fighter at another, leaving the one struck at
// hpAfter of ten. The action is written out as the schema spells it rather
// than through a constant of the code, so that a constant renamed on one side
// is caught here.
func strike(action, by, at string, hit bool, damage, hpAfter int) map[string]any {
	p := decisionBase(action, by)
	p["defender"] = fighter(at)
	p["outcome"] = map[string]any{
		"hit": hit, "natural": 12, "damage": damage, "critical": false, "fumble": false,
		"target_dead": hpAfter == 0,
	}
	p["hp"] = map[string]any{"defender_before": hpAfter + damage, "defender_after": hpAfter, "defender_max": 10}
	return p
}

// freeAttack is the strike out of turn that answers a caught flight.
func freeAttack(damage, hpAfter int) map[string]any {
	p := strike("free_attack", wolfID, playerA, true, damage, hpAfter)
	p["free_attack"] = true
	return p
}

// flight is an attempt to get away. freeAttack says whether the rules punish a
// failure with a strike out of turn; living is how many enemies still stand.
func flight(success, punished bool, living int) map[string]any {
	p := decisionBase("flee", playerA)
	p["outcome"] = map[string]any{
		"hit": false, "natural": 7, "success": success, "threshold": 10, "living_enemies": living,
	}
	if punished {
		p["free_attack"] = true
	}
	return p
}

func roundClosed(seq int, acted, defended, idle []string) eventbus.Event {
	list := func(ids []string) []map[string]any {
		out := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			out = append(out, actor(id, ""))
		}
		return out
	}
	// acted[] carries the event that was the action, unlike the two lists of
	// characters who did nothing (§2.3.3).
	action := make([]map[string]any, 0, len(acted))
	for _, id := range acted {
		item := actor(id, "")
		item["event"] = map[string]any{"id": "ev-" + id, "type": "player.attacked"}
		action = append(action, item)
	}
	return eventbus.NewRoot(swarm.TypeRoundClosed, contracts.SourceTestkitGateway, worldID,
		&eventbus.ScopeRef{ID: "group-1", Type: "group"}, entity.ActorKindCI,
		map[string]any{
			"round":         map[string]any{"seq": seq, "close_reason": "all_acted"},
			"acted":         action,
			"auto_defended": list(defended),
			"idle":          list(idle),
			"closed_at":     testkit.Epoch.Format(time.RFC3339),
		})
}

func created(id, name string, hp, hpMax int) eventbus.Event {
	return eventbus.NewRoot(swarm.TypeCreated, contracts.SourceTestkitState, worldID, nil,
		eventbus.ActorSystem, map[string]any{
			"entity":  actor(id, name),
			"version": 1,
			"attributes": map[string]any{
				entity.AttrHP: hp, entity.AttrHPMax: hpMax,
				entity.AttrStatus: entity.StatusAlive,
			},
		})
}

func updated(id, path string, value int) eventbus.Event {
	return fact(id, "combat", map[string]any{"path": path, "old": 10, "new": value})
}

// playerDied is the entity.updated State publishes when a character falls.
func playerDied(id string) eventbus.Event {
	return fact(id, "combat",
		map[string]any{"path": entity.AttrHP, "old": 1, "new": 0},
		map[string]any{"path": entity.AttrStatus, "old": entity.StatusAlive, "new": entity.StatusDead})
}

// forgotten is the entity.updated State publishes when the gateway abandons a
// character in the cascade of /forget (C-02 v1.2, C-04 v1.1).
func forgotten(id string) eventbus.Event {
	return fact(id, "forget",
		map[string]any{"path": entity.AttrStatus, "old": entity.StatusAlive, "new": entity.StatusAbandoned})
}

func fact(id, cause string, changed ...map[string]any) eventbus.Event {
	return eventbus.NewRoot(swarm.TypeUpdated, contracts.SourceTestkitState, worldID, nil,
		eventbus.ActorSystem, map[string]any{
			"entity":      actor(id, ""),
			"version":     2,
			"cause":       cause,
			"proposal_id": "p-" + cause,
			"applied_at":  testkit.Epoch.Format(time.RFC3339),
			"changed":     changed,
		})
}

// --- reading the bus ---

func narratives(t *testing.T, bus *membus.Bus) []eventbus.Event {
	t.Helper()
	return eventsOf(t, bus, eventbus.TopicNarrativeOutput)
}

func onlyNarrative(t *testing.T, bus *membus.Bus) eventbus.Event {
	t.Helper()
	got := narratives(t, bus)
	if len(got) != 1 {
		t.Fatalf("%d narratives, want exactly one", len(got))
	}
	return got[0]
}

func ofKind(evs []eventbus.Event, kind string) []eventbus.Event {
	out := make([]eventbus.Event, 0, len(evs))
	for _, ev := range evs {
		if k, _ := ev.Path().GetString("kind"); k == kind {
			out = append(out, ev)
		}
	}
	return out
}

// recipientsOf is who a narrative is addressed to. Every recipient has to be a
// player: C-05 gives the text to the players of the scope and to nobody else.
func recipientsOf(t *testing.T, ev eventbus.Event) []string {
	t.Helper()
	var p struct {
		Recipients []struct {
			Entity eventbus.EntityRef `json:"entity"`
		} `json:"recipients"`
	}
	decodeInto(t, ev, &p)
	ids := make([]string, 0, len(p.Recipients))
	for _, r := range p.Recipients {
		if r.Entity.Type != entity.TypePlayer {
			t.Errorf("a %s narrative is addressed to %s %s, which is not a player",
				ev.Type, r.Entity.Type, r.Entity.ID)
		}
		ids = append(ids, r.Entity.ID)
	}
	return ids
}

// basedOn is the identifiers of the events a narrative tells about, in order.
func basedOn(t *testing.T, ev eventbus.Event) []string {
	t.Helper()
	var p struct {
		BasedOn []struct {
			Event struct {
				ID string `json:"id"`
			} `json:"event"`
		} `json:"based_on"`
	}
	decodeInto(t, ev, &p)
	ids := make([]string, 0, len(p.BasedOn))
	for _, ref := range p.BasedOn {
		ids = append(ids, ref.Event.ID)
	}
	return ids
}

func decodeInto(t *testing.T, ev eventbus.Event, dst any) {
	t.Helper()
	raw, err := json.Marshal(ev.Payload)
	if err != nil {
		t.Fatalf("marshal %s: %v", ev.Type, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode %s: %v", ev.Type, err)
	}
}

// waitFor polls until the condition holds. The deadline is wall time: the
// narrator serves its subscriptions in goroutines, and a manual clock cannot
// say how much time those goroutines have actually had.
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
