// Package swarm is the stand-in for the swarm of EPIC-003 in Phase 1: the
// narrator that writes the one text a player ever sees (FakeNarrator), the
// fight (FakeEncounter) and the context of the process that raises both
// (FakeContext), so that the gateway and State can be written, run and
// reviewed before the swarm exists (design.md §4.1, §5; contracts.md C-05;
// tasks.md T-018, T-219, T-220).
//
// FakeNarrator is a stub about how a text is written, never about what a
// consumer sees. Every narrative.output it publishes is a real event of the
// registry with source testkit/swarm — one of the publishers contracts.md §0
// lists for the type — carries meta.agent as the swarm policy of C-01 demands,
// and is validated by the bus against the same schema the personal GM of
// EPIC-003 will be validated against. generated_by is template and
// fallback_reason is unavailable, because that is what it is: no model is
// called, and a consumer that treats template output as second class would
// break the very degradation path FR-034 relies on.
//
// It answers the six occasions C-05 lists, and nothing else:
//
//	player.entered_region, player.looked     → entry
//	encounter.started                        → world_event
//	the last combat.decided of an exchange   → turn (solo scope)
//	entity.updated of a player, status=dead  → death
//	round.closed                             → round
//
// What it deliberately does not do, and who does it instead:
//
//   - No combat. It reads combat.decided and never publishes it, nor
//     dice.rolled, nor a proposal: the fight of Phase 1 is FakeEncounter
//     (C-05, the decision of the orchestrator in tasks.md T-018), and a
//     narrator that decided a blow would be a second, unreviewed rule book.
//   - No death for a character their player forgot. status=abandoned is
//     terminal like dead, and C-02 v1.2 says it is not told about.
//   - No text of its own for a group decision: a group hears one text per
//     round, written on round.closed (C-05).
//   - No model, no filter, no guard, no absence block, no background_refs.
//     Those are the personal GM and the narrator of EPIC-003 C4.
package swarm

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"

	// The narrator reads the action of a combat.decided by the same constants
	// FakeEncounter writes it with, so that the two halves of the stub cannot
	// drift apart on a spelling. The import is covered by the depguard rule
	// shared-testkit-swarm of .golangci.yml, as the one in fake_encounter.go is.
	mech "multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit/swarm/template"
)

// Source is the envelope source of everything the stubs publish
// (contracts.md §0 v0.4).
const Source = contracts.SourceTestkitSwarm

// Group is the consumer group the narrator reads under. Each topic gets its
// own group name derived from it, because a consumer group is a cursor over
// one topic (C-01).
const Group = "testkit-narrator"

// The type the narrator publishes and the types it answers, next to the ones
// fake_encounter.go names (TypeEncounterStarted, TypeCombatDecided).
const (
	TypeNarrativeOutput = "narrative.output"
	TypeEnteredRegion   = "player.entered_region"
	TypeLooked          = "player.looked"
	TypeRoundClosed     = "round.closed"
	TypeCreated         = "entity.created"
	TypeUpdated         = "entity.updated"
)

// The kinds of narrative.output (C-05, narrative.output.v1.json kind).
const (
	KindEntry      = "entry"
	KindRound      = "round"
	KindTurn       = "turn"
	KindDeath      = "death"
	KindWorldEvent = "world_event"
)

// FallbackReason is why a template wrote the text rather than a model: there
// is no model at all (swarm-llm-laws.md 8.3, unavailable).
const FallbackReason = "unavailable"

// DefaultLawsVersion is the laws version the narrative carries when the caller
// names none. The stub does not read world.laws.changed — that is the living
// world of EPIC-003 — so the value comes from the fixture world, whose
// laws_version is v1 (testdata/fixtures/world.json). Nothing in the code reads
// that file, so the two are held together by a test instead:
// TestTheDefaultLawsIsTheOneTheFixtureWorldLivesUnder.
const DefaultLawsVersion = "v1"

// FilterVersion is what the filter block carries when no filter ran. The
// contract requires the block on every narrative.output and requires the
// version to be a non-empty string, so "нет фильтра" is said with a value
// rather than by leaving the block out (C-05, §2.3.11).
const FilterVersion = "none"

// Agent is the swarm agent every narrative of the stub is attributed to. The
// level is task, the level C-05 fixes for a personal GM or a group narrator;
// the blueprint version is 0.0.0 because there is no blueprint behind it.
var Agent = eventbus.AgentRef{
	ID:               "fake-narrator",
	Level:            "task",
	Blueprint:        "fake-narrator",
	BlueprintVersion: "0.0.0",
}

// narrated maps an event type to the narrative kind the stub answers it with.
// It is the whole table and the one place that decides what the stub tells
// about.
var narrated = map[string]string{
	TypeEnteredRegion:    KindEntry,
	TypeLooked:           KindEntry,
	TypeEncounterStarted: KindWorldEvent,
	TypeCombatDecided:    KindTurn,
	TypeUpdated:          KindDeath,
	TypeRoundClosed:      KindRound,
}

// KindFor is the narrative kind the stub may answer an event type with, and
// whether it answers the type at all.
//
// For four of the six types the type is the whole answer. The other two are
// answered only for what the event says: combat.decided when it is the last
// decision of an exchange (closesExchange), and entity.updated when it records
// the death of a player's character (diedIn). A test that derives how many
// narratives a scenario is worth counts those two from what happened, not from
// the types (tasks.md T-018).
func KindFor(eventType string) (string, bool) {
	kind, ok := narrated[eventType]
	return kind, ok
}

// character is what the narrator knows about someone it writes about. It comes
// from the facts of State, never from the player action: an action says who
// acted, and only State says what shape they are in.
type character struct {
	name  string
	hp    int
	hpMax int
}

// exchange is the turn of one character while it is being decided: the
// decisions that have arrived, and the action they all answer.
type exchange struct {
	cause     string
	decisions []decision
}

type decision struct {
	ev eventbus.Event
	p  decisionPayload
}

// FakeNarrator writes the text of a turn from a template table.
type FakeNarrator struct {
	bus         eventbus.Bus
	worldID     string
	lawsVersion string
	log         *slog.Logger

	// answered is the window of events already answered — the two-step
	// eventbus.Dedup of C-01 v1.4 (ADR-027 p. 3). Delivery is at-least-once
	// (C-01), and a look delivered twice is still one look: one narrative per
	// turn is what C-05 promises the gateway.
	//
	// The narrator asks (Has) before it answers and remembers (Add) only once
	// its answer is out. A publication that fails leaves the event unanswered
	// and returns the error, so the bus retries the handler and, when the
	// failure persists, parks the event in dead_letters — the failure is seen
	// instead of swallowed. Seen would remember first and lose it.
	//
	// The price is a possible duplicate, paid on purpose for a stub: remembering
	// late trades a silent loss for a narrative that may be told twice under
	// two different ids. That happens when Publish wrote the narrative and still
	// answered an error (a lost ack) and the retry publishes it again — the
	// narratives of the stub take their id from the generator, so the gateway
	// cannot fold the two (probe P4 of review #2 of T-220) — and it would happen
	// if one event were handled twice at the same time (P5; see the
	// precondition of Narrate). An id derived from the cause (WithCauseID) is
	// what closes the first case for the narrator of the swarm (T-229).
	//
	// The window is not part of any snapshot: the stub is not a stateful
	// context of C-14 and has none. A narrator that is one carries IDs in its
	// snapshot and Restore on recovery (C-14 v1.2 (a)).
	answered *eventbus.Dedup

	mu    sync.Mutex
	known map[string]character
	// foes is whom each character is fighting, learned from encounter.started.
	// A flight names nobody it flees from — combat.decided of a flee has no
	// defender — and the text of it still should.
	foes map[string]string
	// exchanges are the turns being decided, one per character of a solo
	// scope. A turn is dropped from here the moment it is told, so the map
	// holds at most one entry per character.
	exchanges map[string]*exchange

	subscriptions sync.WaitGroup
	subErr        error
	started       bool
}

// NewFakeNarrator builds the narrator for one world. Nothing is published and
// nothing is read until Start.
func NewFakeNarrator(bus eventbus.Bus, worldID string) (*FakeNarrator, error) {
	if bus == nil {
		return nil, errors.New("testkit/swarm: no bus")
	}
	if worldID == "" {
		return nil, errors.New("testkit/swarm: no world")
	}
	return &FakeNarrator{
		bus:         bus,
		worldID:     worldID,
		lawsVersion: DefaultLawsVersion,
		log: slog.New(slog.NewTextHandler(io.Discard, nil)).
			With("component", Source, "world_id", worldID),
		answered:  eventbus.NewDedup(eventbus.DefaultDedupCapacity),
		known:     make(map[string]character),
		foes:      make(map[string]string),
		exchanges: make(map[string]*exchange),
	}, nil
}

// WithLaws sets the laws version the narratives carry. Like WithLog it
// configures the narrator and is called before Start.
func (n *FakeNarrator) WithLaws(version string) *FakeNarrator {
	if version != "" {
		n.lawsVersion = version
	}
	return n
}

// WithLog sends what the narrator did to a logger instead of discarding it.
func (n *FakeNarrator) WithLog(log *slog.Logger) *FakeNarrator {
	if log != nil {
		n.log = log.With("component", Source, "world_id", n.worldID)
	}
	return n
}

// Start opens the four subscriptions the narrator needs: the player actions it
// tells about, the rounds and decisions of game_events, the encounters the
// world opens, and the facts of State it learns the characters and their
// deaths from.
//
// Nothing is announced at start. The narrator is not a stateful context of
// C-14 whose recovery consumers wait for; what it knows it learns from the
// journal, and a character it has not heard of yet is written about without a
// number rather than not written about at all.
func (n *FakeNarrator) Start(ctx context.Context) error {
	n.mu.Lock()
	if n.started {
		n.mu.Unlock()
		return errors.New("testkit/swarm: already started")
	}
	n.started = true
	n.mu.Unlock()

	n.subscribe(ctx, eventbus.TopicPlayerEvents, n.Narrate)
	n.subscribe(ctx, eventbus.TopicGameEvents, n.Narrate)
	n.subscribe(ctx, eventbus.TopicWorldEvents, n.Narrate)
	n.subscribe(ctx, eventbus.TopicSystemEvents, n.Observe)
	return nil
}

func (n *FakeNarrator) subscribe(ctx context.Context, topic string, h eventbus.Handler) {
	n.subscriptions.Add(1)
	go func() {
		defer n.subscriptions.Done()
		if err := n.bus.Subscribe(ctx, topic, Group+"-"+topic, h); err != nil {
			n.mu.Lock()
			if n.subErr == nil {
				n.subErr = fmt.Errorf("testkit/swarm: subscribe %s: %w", topic, err)
			}
			n.mu.Unlock()
			n.log.Error("subscription stopped", "topic", topic, "err", err)
		}
	}()
}

// Wait blocks until every subscription started by Start has stopped and
// reports the first failure among them. A subscription cancelled through its
// context is not a failure and reports nil (C-01).
func (n *FakeNarrator) Wait() error {
	n.subscriptions.Wait()
	return n.Err()
}

// Err is the first failure of a subscription, or nil while they are healthy.
// Unlike Wait it does not block, which is what lets the health of the context
// ask about the narrator while the other subscriptions are still running.
func (n *FakeNarrator) Err() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.subErr
}

// Observe folds a fact of State into what the narrator knows about a
// character, and tells the death of a player's character when the fact records
// one. It is the handler Start subscribes system_events with, exported so that
// a test can hand the narrator a fact directly.
//
// It has the precondition of Narrate: it is not called concurrently for one
// event.
func (n *FakeNarrator) Observe(ctx context.Context, ev eventbus.Event) error {
	if ev.Type != TypeCreated && ev.Type != TypeUpdated {
		return nil
	}
	if !n.serves(ev) {
		return nil
	}
	var p factPayload
	if err := decodePayload(ev.Payload, &p); err != nil {
		n.log.Warn("a fact the narrator cannot read", "event_id", ev.ID, "err", err)
		return nil
	}
	if p.Entity.Entity.ID == "" {
		return nil
	}
	n.learn(p)

	if ev.Type != TypeUpdated || p.Entity.Entity.Type != entity.TypePlayer || !diedIn(p.Changed) {
		return nil
	}
	if n.answered.Has(ev.ID) {
		return nil
	}
	if err := n.death(ctx, ev, p.Entity); err != nil {
		return err
	}
	n.answered.Add(ev.ID)
	return nil
}

// learn folds one fact into what the narrator knows about a character.
func (n *FakeNarrator) learn(p factPayload) {
	id := p.Entity.Entity.ID
	n.mu.Lock()
	defer n.mu.Unlock()
	who, seen := n.known[id]
	if !seen {
		who = character{hp: template.Unknown, hpMax: template.Unknown}
	}
	if p.Entity.Name != "" {
		who.name = p.Entity.Name
	}
	// entity.created carries the whole attribute set; entity.updated carries
	// only the paths that ended up different (C-02).
	who.hp = intOf(p.Attributes[entity.AttrHP], who.hp)
	who.hpMax = intOf(p.Attributes[entity.AttrHPMax], who.hpMax)
	for _, change := range p.Changed {
		switch change.Path {
		case entity.AttrHP:
			who.hp = intOf(change.New, who.hp)
		case entity.AttrHPMax:
			who.hpMax = intOf(change.New, who.hpMax)
		}
	}
	n.known[id] = who
}

// diedIn says whether a fact records a death: the status moved to dead.
//
// A character their player forgot moves to abandoned, which is as terminal as
// dead and is deliberately not a death here: C-02 v1.2 says that consumers
// treat it like dead for scope, targets and rounds, but that narrative.output
// kind=death is not generated for it.
func diedIn(changed []entity.Change) bool {
	for _, change := range changed {
		if change.Path == entity.AttrStatus && change.New == entity.StatusDead {
			return true
		}
	}
	return false
}

// Narrate answers one event with a narrative.output, or with nothing when the
// type is not one the stub tells about. It is the handler Start subscribes
// player_events, game_events and world_events with.
//
// An event of another world is passed over: a narrator is a role inside one
// world, and a text about a world it does not serve would reach the players of
// that world through the gateway as if it were theirs.
//
// The event is remembered as told only once it has been answered. An error is
// a publication that did not go out, and the bus retries the handler on it
// (see answered).
//
// Precondition: Narrate is not called concurrently for one event. The window
// asks (Has) and remembers (Add) in two steps, and two deliveries of one event
// racing between them would both publish. membus, kafka and eventbus.Delivery hold it
// today — a subscription hands its topic over one event at a time and retries
// in sequence — and a caller driving the handler by hand has to hold it too.
func (n *FakeNarrator) Narrate(ctx context.Context, ev eventbus.Event) error {
	kind, tells := KindFor(ev.Type)
	if !tells || kind == KindDeath {
		// A death is a fact of State and arrives through Observe.
		return nil
	}
	if !n.serves(ev) {
		return nil
	}
	if n.answered.Has(ev.ID) {
		n.log.Debug("event already told about", "event_id", ev.ID, "type", ev.Type)
		return nil
	}
	var err error
	switch kind {
	case KindEntry:
		err = n.entry(ctx, ev)
	case KindRound:
		err = n.round(ctx, ev)
	case KindWorldEvent:
		err = n.worldEvent(ctx, ev)
	default:
		err = n.turn(ctx, ev)
	}
	if err != nil {
		return err
	}
	n.answered.Add(ev.ID)
	return nil
}

// serves says whether an event belongs to the world this narrator serves. An
// event that names no world is taken as this one: a legacy producer may carry
// none, and the bus has already checked everything the envelope must carry.
func (n *FakeNarrator) serves(ev eventbus.Event) bool {
	world := eventbus.GetWorldIDFromEvent(ev)
	if world == "" || world == n.worldID {
		return true
	}
	n.log.Debug("event of another world passed over",
		"event_id", ev.ID, "type", ev.Type, "event_world_id", world)
	return false
}

// entry writes the text of one character arriving somewhere or looking around.
// The recipient is that character: in a solo scope the scope is the player
// (C-04, scope solo:{player_id}).
func (n *FakeNarrator) entry(ctx context.Context, ev eventbus.Event) error {
	var p actionPayload
	if err := decodePayload(ev.Payload, &p); err != nil {
		n.log.Warn("an action the narrator cannot read", "event_id", ev.ID, "err", err)
		return nil
	}
	who := recipientOf(p.Entity)
	if who == nil {
		n.log.Warn("event names no acting character", "event_id", ev.ID, "type", ev.Type)
		return nil
	}
	fields := n.fieldsFor(p.Entity.Entity.ID, p.Entity.Name)
	if p.Target != nil {
		fields.Place = cmp.Or(p.Target.Name, p.Target.Entity.ID)
	}
	return n.publish(ctx, ev, KindEntry, []map[string]any{who},
		template.Entry(ev.ID, fields), []eventbus.Event{ev}, nil)
}

// round writes the text of a closed round. Everyone the round names is a
// recipient - those who acted, those who defended by default and those who sat
// it out: C-05 says recipients are every player of the scope, idle ones
// included.
func (n *FakeNarrator) round(ctx context.Context, ev eventbus.Event) error {
	var p roundPayload
	if err := decodePayload(ev.Payload, &p); err != nil {
		n.log.Warn("a round the narrator cannot read", "event_id", ev.ID, "err", err)
		return nil
	}
	recipients, names, _ := n.playersOf(p.Acted, p.AutoDefended, p.Idle)
	if len(recipients) == 0 {
		// recipients has minItems 1: a round nobody was in has no text, and
		// publishing an empty one would be a schema violation dressed up as a
		// narrative.
		n.log.Warn("round closed with nobody in it", "event_id", ev.ID)
		return nil
	}
	fields := template.Fields{
		Names: strings.Join(names, ", "),
		Round: p.Round.Seq,
		HP:    template.Unknown,
		HPMax: template.Unknown,
	}
	if p.Encounter != nil {
		fields.Place = cmp.Or(p.Encounter.Name, p.Encounter.Entity.ID)
	}
	extra := map[string]any{"round": map[string]any{"seq": p.Round.Seq}}
	return n.publish(ctx, ev, KindRound, recipients, template.Round(ev.ID, fields),
		[]eventbus.Event{ev}, extra)
}

// worldEvent writes the text of an encounter the world has just opened. The
// recipients are the characters the encounter was opened around — the players
// of its scope — and never the NPCs it was opened with.
//
// It is told when encounter.started arrives, and that is always an encounter
// State has created: the event is published only after entity.created of the
// encounter, and a creation State refuses announces nothing at all (C-05 v1.4
// p. 4, ADR-026). The fact may still reach a reader of two topics later than
// the event; it is in the journal all the same.
func (n *FakeNarrator) worldEvent(ctx context.Context, ev eventbus.Event) error {
	var p startedPayload
	if err := decodePayload(ev.Payload, &p); err != nil {
		n.log.Warn("an encounter the narrator cannot read", "event_id", ev.ID, "err", err)
		return nil
	}
	recipients, names, ids := n.playersOf(p.Participants)
	if len(recipients) == 0 {
		n.log.Warn("encounter opened around no character", "event_id", ev.ID)
		return nil
	}
	foes := make([]string, 0, len(p.NPCs))
	for _, npc := range p.NPCs {
		foes = append(foes, n.nameOfKnown(npc.Entity.ID, npc.Name))
	}
	foe := strings.Join(foes, ", ")
	n.mu.Lock()
	for _, id := range ids {
		n.foes[id] = foe
	}
	n.mu.Unlock()

	fields := template.Fields{
		Names: strings.Join(names, ", "),
		Foe:   foe,
		Place: cmp.Or(p.Region.Name, p.Region.Entity.ID),
		HP:    template.Unknown,
		HPMax: template.Unknown,
	}
	return n.publish(ctx, ev, KindWorldEvent, recipients, template.WorldEvent(ev.ID, fields),
		[]eventbus.Event{ev}, nil)
}

// turn gathers the decisions of one exchange and tells the turn once the last
// of them has arrived (closesExchange). The text is caused by that last
// decision and based on all of them, in the order they were decided.
//
// Only a solo scope hears a turn: a group hears one text per round, written on
// round.closed, and a decision of a group is part of what that text is about
// (C-05: one narrative.output per solo turn and one per group round).
//
// The turn is dropped from the exchanges only once it has been told. When the
// publication fails the decisions stay gathered, the bus delivers the last one
// again, and the retry tells the same turn from the same decisions.
func (n *FakeNarrator) turn(ctx context.Context, ev eventbus.Event) error {
	var p decisionPayload
	if err := decodePayload(ev.Payload, &p); err != nil {
		n.log.Warn("a decision the narrator cannot read", "event_id", ev.ID, "err", err)
		return nil
	}
	player := soloPlayer(eventbus.GetScopeFromEvent(ev), p)
	if player == nil {
		n.log.Debug("a decision with no character of a solo scope to tell", "event_id", ev.ID)
		return nil
	}
	decisions := n.collect(player.Entity.ID, ev, p)
	if decisions == nil {
		return nil
	}
	basedOn := make([]eventbus.Event, 0, len(decisions))
	for _, d := range decisions {
		basedOn = append(basedOn, d.ev)
	}
	if err := n.publish(ctx, ev, KindTurn, []map[string]any{recipientOf(player)},
		n.turnText(ev.ID, player, decisions), basedOn, nil); err != nil {
		return err
	}
	n.mu.Lock()
	delete(n.exchanges, player.Entity.ID)
	n.mu.Unlock()
	return nil
}

// collect adds a decision to the turn of a character and hands the whole turn
// back once its last decision has arrived, or nil while it is still being
// decided. The turn stays where it is until turn has told it.
//
// A turn is recognised by the action every decision of it answers — the cause
// of the decision (C-01: an event derived from the action carries its id as
// causation). A decision of another action means the turn before it is not
// coming back: a solo character acts one action at a time, and the decisions
// of one action travel on one topic in order (C-01). When the turn left
// behind was never closed, that is a defect of the publisher, not a pause, and
// it is said out loud — on the level FakeEncounter says its own defects on —
// rather than papered over with a text about half a turn. When it was closed,
// its text is what failed: the bus gave up on the publication and parked the
// last decision in dead_letters, and the log says that instead.
//
// The last decision delivered again after a publication that failed is the
// same decision, and it is not gathered a second time.
func (n *FakeNarrator) collect(playerID string, ev eventbus.Event, p decisionPayload) []decision {
	cause := cmp.Or(ev.Meta.CausationID, ev.ID)
	n.mu.Lock()
	defer n.mu.Unlock()
	ex := n.exchanges[playerID]
	if ex != nil && ex.cause != cause {
		if closesExchange(ex.decisions[len(ex.decisions)-1].p) {
			n.log.Error("the text of a turn was never published, its last decision is in dead_letters; the turn is dropped",
				"player_id", playerID, "caused_by", ex.cause, "decisions", len(ex.decisions))
		} else {
			n.log.Error("a turn was never closed and is dropped untold",
				"player_id", playerID, "caused_by", ex.cause, "decisions", len(ex.decisions))
		}
		ex = nil
	}
	if ex == nil {
		ex = &exchange{cause: cause}
		n.exchanges[playerID] = ex
	}
	if last := len(ex.decisions); last == 0 || ex.decisions[last-1].ev.ID != ev.ID {
		ex.decisions = append(ex.decisions, decision{ev: ev, p: p})
	}
	if !closesExchange(p) {
		return nil
	}
	return slices.Clone(ex.decisions)
}

// closesExchange says whether a decision is the last one of its exchange —
// whether the turn it belongs to can be told now.
//
// The publisher says so in exchange.last (C-05 v1.4 p. 7), and when it does,
// that is the whole answer: the publisher knows the shape of the exchange,
// while a reader can only guess it, and the guess below is right for exactly
// one shape — an action answered by one NPC — and would go quietly wrong with
// several NPCs or in a group.
//
// The guess is kept for a publisher that does not fill the field, and only for
// it. It reads the decision by the rule every exchange of C-05 follows today
// (combat.decided ×2, design §4.1): a blow that did not kill is answered by the
// NPC it struck; a flight that failed is answered by a strike out of turn when
// the rules call for one (free_attack) and somebody is still standing to deal
// it (living_enemies); an answer is never answered. Nothing here looks at the
// clock or waits for a quiet moment: the decision itself says whether another
// one follows.
func closesExchange(p decisionPayload) bool {
	if p.Exchange != nil {
		return p.Exchange.Last
	}
	switch p.Action {
	case mech.ActionAttack:
		return p.Outcome.TargetDead
	case mech.ActionFlee:
		escaped := p.Outcome.Success != nil && *p.Outcome.Success
		return escaped || !p.FreeAttack || p.Outcome.LivingEnemies == 0
	default:
		// npc_attack and free_attack: the answer of an exchange.
		return true
	}
}

// soloPlayer is the character a turn is told to: the one of attacker and
// defender that is a player and, when the envelope names a scope, the player
// the scope is (C-04, scope solo:{player_id}).
//
// That one rule is also what keeps a group out of it. The scope of a group is
// the group, and no fighter of a decision is a group, so a decision of a group
// has nobody to be told to here: a group hears one text per round, written on
// round.closed (C-05: one narrative.output per solo turn and one per group
// round). A decision whose scope names another character is told to nobody
// for the same reason.
func soloPlayer(scope *eventbus.ScopeRef, p decisionPayload) *namedRef {
	for _, ref := range []*namedRef{&p.Attacker, p.Defender} {
		if ref != nil && ref.Entity.Type == entity.TypePlayer &&
			(scope == nil || scope.ID == ref.Entity.ID) {
			return ref
		}
	}
	return nil
}

// turnText writes the text of a turn out of its decisions: how the character's
// action ended, what it dealt, what the answer took, and the health it left.
//
// A flight that failed is told by what followed it. A strike out of turn makes
// it a flight that was caught; nothing following — the rules call for no
// strike, or nobody is left standing to deal one — makes it a flight that
// simply did not work, and its text claims no blow. A text that told of a
// strike nobody dealt would be telling the player something that did not
// happen (Mi-2 of the review of T-220).
//
// The health comes from the answer when there was one — hp.defender_after of
// the decision that struck the character is the freshest number there is, a
// fact of State that has not come back yet would be older — and otherwise from
// what State last said.
func (n *FakeNarrator) turnText(seed string, player *namedRef, decisions []decision) string {
	id := player.Entity.ID
	fields := n.fieldsFor(id, player.Name)
	outcome := template.Miss
	for _, d := range decisions {
		p := d.p
		if p.Attacker.Entity.ID == id {
			switch {
			case p.Action == mech.ActionFlee && p.Outcome.Success != nil && *p.Outcome.Success:
				outcome = template.Fled
			case p.Action == mech.ActionFlee:
				outcome = template.Stuck
			case p.Outcome.TargetDead:
				fields.Dealt += p.Outcome.Damage
				outcome = template.Kill
			case p.Outcome.Hit:
				fields.Dealt += p.Outcome.Damage
				outcome = template.Hit
			}
			if p.Defender != nil {
				fields.Foe = n.nameOfKnown(p.Defender.Entity.ID, p.Defender.Name)
			}
			continue
		}
		if outcome == template.Stuck {
			outcome = template.Caught
		}
		if p.Outcome.Hit {
			fields.Taken += p.Outcome.Damage
		}
		fields.Foe = n.nameOfKnown(p.Attacker.Entity.ID, p.Attacker.Name)
		if p.HP != nil {
			fields.HP, fields.HPMax = p.HP.DefenderAfter, p.HP.DefenderMax
		}
	}
	if fields.Foe == "" {
		n.mu.Lock()
		fields.Foe = n.foes[id]
		n.mu.Unlock()
	}
	return template.Turn(seed, outcome, fields)
}

// death writes the text of a player's character who has died. The recipient
// is that character; Observe has already established that it is a player.
func (n *FakeNarrator) death(ctx context.Context, ev eventbus.Event, who namedRef) error {
	fields := n.fieldsFor(who.Entity.ID, who.Name)
	return n.publish(ctx, ev, KindDeath, []map[string]any{recipientOf(&who)},
		template.Death(ev.ID, fields), []eventbus.Event{ev}, nil)
}

// playersOf turns lists of entity references into the recipients of a
// narrative: players only, each once, in the order the lists name them. It
// answers the recipients, their names as the narrator knows them, and their
// identifiers.
func (n *FakeNarrator) playersOf(lists ...[]namedRef) ([]map[string]any, []string, []string) {
	recipients := make([]map[string]any, 0, 4)
	names := make([]string, 0, 4)
	ids := make([]string, 0, 4)
	for _, list := range lists {
		for _, ref := range list {
			who := recipientOf(&ref)
			if who == nil || contains(ids, ref.Entity.ID) {
				continue
			}
			recipients = append(recipients, who)
			names = append(names, n.nameOfKnown(ref.Entity.ID, ref.Name))
			ids = append(ids, ref.Entity.ID)
		}
	}
	return recipients, names, ids
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// publish builds and sends the narrative. narrative_event_id is the identifier
// of this very event, copied into the payload so that the gateway can group
// the deliveries of one round without reading the envelope (C-05 v1.1).
//
// A publication that fails is said out loud here, because nothing else says
// it: the bus logs an event only when it parks it, and until then the retry of
// the handler is the only trace.
func (n *FakeNarrator) publish(ctx context.Context, cause eventbus.Event, kind string,
	recipients []map[string]any, text string, basedOn []eventbus.Event,
	extra map[string]any) error {
	refs := make([]map[string]any, 0, len(basedOn))
	for _, ev := range basedOn {
		refs = append(refs, map[string]any{"event": map[string]any{"id": ev.ID, "type": ev.Type}})
	}
	payload := map[string]any{
		"recipients":      recipients,
		"text":            text,
		"generated_by":    "template",
		"fallback_reason": FallbackReason,
		"kind":            kind,
		"locale":          eventbus.DefaultLocale,
		"laws_version":    n.lawsVersion,
		"filter": map[string]any{
			"applied":        false,
			"status":         "pass",
			"filter_version": FilterVersion,
		},
		"based_on": refs,
	}
	maps.Copy(payload, extra)
	ev := eventbus.Derive(cause, TypeNarrativeOutput, Source, payload, eventbus.WithAgent(Agent))
	ev.Payload["narrative_event_id"] = ev.ID
	if err := n.bus.Publish(ctx, ev); err != nil {
		n.log.Error("narrative not published; its cause stays untold for the bus to deliver again",
			"kind", kind, "caused_by", cause.ID, "err", err)
		return fmt.Errorf("testkit/swarm: publish %s (%s): %w", TypeNarrativeOutput, kind, err)
	}
	n.log.Info("narrative written", "kind", kind, "event_id", ev.ID,
		"caused_by", cause.ID, "recipients", len(recipients))
	return nil
}

// fieldsFor fills in what the narrator knows about a character from the facts
// of State, falling back to the name the event carried.
func (n *FakeNarrator) fieldsFor(id, name string) template.Fields {
	n.mu.Lock()
	who, known := n.known[id]
	n.mu.Unlock()
	fields := template.Fields{Name: name, HP: template.Unknown, HPMax: template.Unknown}
	if known {
		fields.Name = cmp.Or(who.name, name)
		fields.HP, fields.HPMax = who.hp, who.hpMax
	}
	fields.Name = cmp.Or(fields.Name, id)
	return fields
}

func (n *FakeNarrator) nameOfKnown(id, fallback string) string {
	n.mu.Lock()
	who, known := n.known[id]
	n.mu.Unlock()
	if known {
		return cmp.Or(who.name, fallback, id)
	}
	return cmp.Or(fallback, id)
}

// namedRef is the EntityWithName of a payload. The items of acted[] carry an
// event next to it (2.3.3); the narrator does not read that, and an unknown
// field is ignored rather than refused, because a payload richer than what a
// consumer reads is exactly what a versioned schema is for.
type namedRef struct {
	Entity eventbus.EntityRef `json:"entity"`
	Name   string             `json:"name"`
}

// actionPayload is the part of a player action the narrator reads.
type actionPayload struct {
	Entity *namedRef `json:"entity"`
	Target *namedRef `json:"target"`
}

// roundPayload is the part of round.closed the narrator reads.
type roundPayload struct {
	Encounter *namedRef `json:"encounter"`
	Round     struct {
		Seq int `json:"seq"`
	} `json:"round"`
	Acted        []namedRef `json:"acted"`
	AutoDefended []namedRef `json:"auto_defended"`
	Idle         []namedRef `json:"idle"`
}

// startedPayload is the part of encounter.started the narrator reads.
type startedPayload struct {
	Region       namedRef   `json:"region"`
	Participants []namedRef `json:"participants"`
	NPCs         []namedRef `json:"npcs"`
}

// decisionPayload is the part of combat.decided the narrator reads — what
// happened, and nothing of how it was rolled.
type decisionPayload struct {
	Action   string    `json:"action"`
	Attacker namedRef  `json:"attacker"`
	Defender *namedRef `json:"defender"`
	Outcome  struct {
		Hit           bool  `json:"hit"`
		Damage        int   `json:"damage"`
		TargetDead    bool  `json:"target_dead"`
		Success       *bool `json:"success"`
		LivingEnemies int   `json:"living_enemies"`
	} `json:"outcome"`
	HP *struct {
		DefenderAfter int `json:"defender_after"`
		DefenderMax   int `json:"defender_max"`
	} `json:"hp"`
	FreeAttack bool `json:"free_attack"`
	// Exchange is where the publisher says this decision stands in its exchange
	// (C-05 v1.4 p. 7). It is nil for a publisher that does not fill it.
	Exchange *struct {
		Index int  `json:"index"`
		Last  bool `json:"last"`
	} `json:"exchange"`
}

// factPayload is the part of entity.created and entity.updated the narrator
// reads: who it is about, and what shape they are in.
type factPayload struct {
	Entity     namedRef        `json:"entity"`
	Attributes map[string]any  `json:"attributes"`
	Changed    []entity.Change `json:"changed"`
}

// decodePayload reads a payload into a typed shape. It goes through JSON
// rather than through type assertions because that is how the payload
// travelled: whoever published it wrote a map, and a number in it is a float64
// whichever way the publisher meant it. It is also what makes the exported
// handlers usable with a payload built in Go, where the same field is a slice
// of maps rather than a slice of any.
func decodePayload(payload map[string]any, dst any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

// recipientOf turns an entity reference of a payload into the EntityWithName a
// narrative carries — for a player, and for nobody else: C-05 says recipients
// are the players of the scope, and an NPC or an encounter named next to them
// in a payload is not somebody a text is delivered to.
func recipientOf(ref *namedRef) map[string]any {
	if ref == nil || ref.Entity.ID == "" || ref.Entity.Type != entity.TypePlayer {
		return nil
	}
	out := map[string]any{
		"entity": map[string]any{"id": ref.Entity.ID, "type": ref.Entity.Type},
	}
	if ref.Name != "" {
		out["name"] = ref.Name
	}
	return out
}

// intOf reads a number that travelled through JSON, where every number is a
// float64 whichever way the publisher meant it.
func intOf(value any, fallback int) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return fallback
	}
}
