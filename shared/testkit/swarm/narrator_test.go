package swarm_test

import (
	"context"
	"encoding/json"
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
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/membus"
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

// TestTheTableOfV0IsTheWholeTable states out loud what the narrator of v0
// answers and what it leaves to EPIC-003, so that a type quietly added to the
// table has to be added here too — and a type quietly dropped is noticed.
func TestTheTableOfV0IsTheWholeTable(t *testing.T) {
	tells := map[string]string{
		swarm.TypeEnteredRegion: swarm.KindEntry,
		swarm.TypeLooked:        swarm.KindEntry,
		swarm.TypeRoundClosed:   swarm.KindRound,
	}
	// Every type of the registry is asked, rather than a list of types written
	// out here, so that no type can be added to the table without a line in
	// tells. Among them are the types C-05 gives the narrator that v0 has
	// nothing to feed: each arrives with the stub that publishes it, in I1-α —
	// combat.decided and encounter.* with testkit/swarm.FakeEncounter, the
	// death of a character with the fight that kills it. A type outside the
	// registry needs no line: the bus refuses to carry one at all, so an entry
	// for it in the table could never be reached.
	answered := 0
	for _, spec := range contracts.All() {
		kind, ok := swarm.KindFor(spec.Type)
		want, listed := tells[spec.Type]
		if ok != listed || kind != want {
			t.Errorf("the narrator answers %s with %q (answers=%v); "+
				"the table of v0 is %q (answers=%v)", spec.Type, kind, ok, want, listed)
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

// --- the entry narrative ---

func TestALookBecomesAnEntryNarrative(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)

	narrate(t, n, look(playerA, nameA))

	ev := onlyNarrative(t, bus)
	pa := ev.Path()
	if kind, _ := pa.GetString("kind"); kind != swarm.KindEntry {
		t.Errorf("kind %q, want %q", kind, swarm.KindEntry)
	}
	if by, _ := pa.GetString("generated_by"); by != "template" {
		t.Errorf("generated_by %q, want template", by)
	}
	if to, _ := pa.GetString("recipients[0].entity.id"); to != playerA {
		t.Errorf("addressed to %q, want %q", to, playerA)
	}
	if name, _ := pa.GetString("recipients[0].name"); name != nameA {
		t.Errorf("the recipient is named %q, want %q", name, nameA)
	}
	if list, _ := pa.GetSlice("recipients"); len(list) != 1 {
		t.Errorf("%d recipients, a solo scope has one", len(list))
	}
	if version, _ := pa.GetString("laws_version"); version != swarm.DefaultLawsVersion {
		t.Errorf("laws_version %q, want %q", version, swarm.DefaultLawsVersion)
	}
	if locale, _ := pa.GetString("locale"); locale != eventbus.DefaultLocale {
		t.Errorf("locale %q, want %q", locale, eventbus.DefaultLocale)
	}
	if id, _ := pa.GetString("narrative_event_id"); id != ev.ID {
		t.Errorf("narrative_event_id %q, want the id of the event itself %q", id, ev.ID)
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
	if ev.Meta.CausationID != cause.ID {
		t.Errorf("caused by %q, want the action %q", ev.Meta.CausationID, cause.ID)
	}
	if ev.Meta.CorrelationID != cause.Meta.CorrelationID {
		t.Errorf("correlates to %q, the action to %q",
			ev.Meta.CorrelationID, cause.Meta.CorrelationID)
	}
	id, _ := ev.Path().GetString("based_on[0].event.id")
	typ, _ := ev.Path().GetString("based_on[0].event.type")
	if id != cause.ID || typ != cause.Type {
		t.Errorf("based_on names %s %s, want %s %s", typ, id, cause.Type, cause.ID)
	}
}

// TestTheTextCarriesWhatStateSaid is why the narrator reads system_events at
// all: a text about a character says how that character is doing, and only
// State knows.
func TestTheTextCarriesWhatStateSaid(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)
	if err := n.Observe(t.Context(), updated(playerA, entity.AttrHP, 4)); err != nil {
		t.Fatalf("observe: %v", err)
	}

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
	if text == "" {
		t.Fatal("no text at all for a character the narrator has not heard of")
	}
	if !strings.Contains(text, nameA) {
		t.Errorf("the name of the action was not used as the fallback: %q", text)
	}
}

// TestAnEntryNamesThePlaceItHappensIn covers the other substitution an entry
// makes: where the character is.
func TestAnEntryNamesThePlaceItHappensIn(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)

	narrate(t, n, entered(playerA, nameA, regionID, "Тёмный лес"))

	text, _ := onlyNarrative(t, bus).Path().GetString("text")
	// Not every template names the place, so the assertion is that the
	// placeholder is gone, not that the name is present.
	if strings.Contains(text, "{") {
		t.Errorf("a placeholder was left unsubstituted: %q", text)
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
	list, _ := pa.GetSlice("recipients")
	if len(list) != 3 {
		t.Fatalf("%d recipients, want the three characters of the round without repeats", len(list))
	}
	got := make([]string, 0, len(list))
	for i := range list {
		id, _ := pa.GetString("recipients[" + strconv.Itoa(i) + "].entity.id")
		got = append(got, id)
	}
	want := []string{playerA, "player-B", "player-C"}
	if !slices.Equal(got, want) {
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

// --- what the narrator passes over ---

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

	if got := narratives(t, bus); len(got) != 0 {
		t.Errorf("%d narratives about a type not in the table", len(got))
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

// --- the shape on the wire ---

// TestEveryNarrativeIsValidAgainstTheContract is the promise of the package
// doc: the stub is about how a text is written, not about what a consumer
// sees.
func TestEveryNarrativeIsValidAgainstTheContract(t *testing.T) {
	n, bus := running(t)
	knows(t, n, playerA, nameA, 10, 10)

	narrate(t, n, look(playerA, nameA))
	narrate(t, n, entered(playerA, nameA, regionID, "Тёмный лес"))
	narrate(t, n, roundClosed(2, []string{playerA}, nil, nil))

	got := narratives(t, bus)
	if len(got) != 3 {
		t.Fatalf("%d narratives, want three", len(got))
	}
	spec, known := contracts.Lookup(swarm.TypeNarrativeOutput)
	if !known {
		t.Fatalf("%s is not in the registry", swarm.TypeNarrativeOutput)
	}
	if !slices.Contains(spec.Publishers, swarm.Source) {
		t.Errorf("%q may not publish %s, only %v",
			swarm.Source, swarm.TypeNarrativeOutput, spec.Publishers)
	}
	for i, ev := range got {
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("narrative %d is not valid: %v", i+1, err)
		}
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

func narrate(t *testing.T, n *swarm.FakeNarrator, ev eventbus.Event) {
	t.Helper()
	if err := n.Narrate(t.Context(), ev); err != nil {
		t.Fatalf("narrate %s: %v", ev.Type, err)
	}
}

// knows teaches the narrator a character the way State does: with a fact.
func knows(t *testing.T, n *swarm.FakeNarrator, id, name string, hp, hpMax int) {
	t.Helper()
	if err := n.Observe(t.Context(), created(id, name, hp, hpMax)); err != nil {
		t.Fatalf("observe: %v", err)
	}
}

// --- the events of the tests ---

func playerAction(typ string, payload map[string]any) eventbus.Event {
	return eventbus.NewRoot(typ, contracts.SourceTestkitGateway, worldID,
		&eventbus.ScopeRef{ID: playerA, Type: "solo"}, entity.ActorKindCI, payload)
}

func actor(id, name string) map[string]any {
	return map[string]any{
		"entity": map[string]any{"id": id, "type": entity.TypePlayer},
		"name":   name,
	}
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
	return eventbus.NewRoot(swarm.TypeUpdated, contracts.SourceTestkitState, worldID, nil,
		eventbus.ActorSystem, map[string]any{
			"entity":      actor(id, ""),
			"version":     2,
			"cause":       "combat",
			"proposal_id": "p-1",
			"applied_at":  testkit.Epoch.Format(time.RFC3339),
			"changed": []map[string]any{
				{"path": path, "old": 10, "new": value},
			},
		})
}

// --- reading the bus ---

func narratives(t *testing.T, bus *membus.Bus) []eventbus.Event {
	t.Helper()
	records, err := bus.Records(eventbus.TopicNarrativeOutput)
	if err != nil {
		t.Fatalf("read %s: %v", eventbus.TopicNarrativeOutput, err)
	}
	out := make([]eventbus.Event, 0, len(records))
	for i, body := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatalf("decode %s[%d]: %v", eventbus.TopicNarrativeOutput, i, err)
		}
		out = append(out, ev)
	}
	return out
}

func onlyNarrative(t *testing.T, bus *membus.Bus) eventbus.Event {
	t.Helper()
	got := narratives(t, bus)
	if len(got) != 1 {
		t.Fatalf("%d narratives, want exactly one", len(got))
	}
	return got[0]
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
