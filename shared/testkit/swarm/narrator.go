// Package swarm is the stand-in for the narrating roles of the agent swarm:
// it turns the events a player action produces into the one text a player ever
// sees, so that the gateway and State can be written, run and reviewed before
// EPIC-003 exists (design.md §5, contracts.md C-05, tasks.md T-018).
//
// It is a stub about how a text is written, never about what a consumer sees.
// Every narrative.output it publishes is a real event of the registry with
// source testkit/swarm — one of the two publishers contracts.md §0 lists for
// the type — carries meta.agent as the swarm policy of C-01 demands, and is
// validated by the bus against the same schema the personal GM of EPIC-003
// will be validated against. generated_by is template, because that is what it
// is: no model is called, and a consumer that treats template output as second
// class would break the very degradation path FR-034 relies on.
//
// What it deliberately does not do, and who does it instead:
//
//   - No model, no filter, no guard, no absence block, no background_refs.
//     Those are the personal GM and the narrator of EPIC-003 C4 (T-220).
//   - No combat. narrative.output kind=turn tells about combat.decided, and in
//     Phase 1 the only publisher of combat.decided is FakeEncounter of
//     EPIC-003 (contracts.md C-05, the decision of the orchestrator in
//     tasks.md T-018). A subscription to a type nothing publishes would be a
//     branch no test could reach, so kind=turn arrives together with the stub
//     that feeds it, in I1-α.
//   - No kind=death and no kind=world_event: the first needs entity.updated
//     status=dead, which only combat produces here, and the second needs the
//     living world of EPIC-003.
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
	"strings"
	"sync"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit/swarm/template"
)

// Source is the envelope source of everything the narrator publishes
// (contracts.md §0 v0.4).
const Source = contracts.SourceTestkitSwarm

// Group is the consumer group the narrator reads under. Each topic gets its
// own group name derived from it, because a consumer group is a cursor over
// one topic (C-01).
const Group = "testkit-narrator"

// The type the narrator publishes and the types it answers.
const (
	TypeNarrativeOutput = "narrative.output"
	TypeEnteredRegion   = "player.entered_region"
	TypeLooked          = "player.looked"
	TypeRoundClosed     = "round.closed"
	TypeCreated         = "entity.created"
	TypeUpdated         = "entity.updated"
)

// The kinds of narrative.output the stub of v0 produces (C-05).
const (
	KindEntry = "entry"
	KindRound = "round"
)

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
// It is the whole table of v0 and the one place that decides what the stub
// tells about.
var narrated = map[string]string{
	TypeEnteredRegion: KindEntry,
	TypeLooked:        KindEntry,
	TypeRoundClosed:   KindRound,
}

// KindFor is the narrative kind the stub answers an event type with, and
// whether it answers it at all.
//
// It is exported so that a test can derive how many narratives a scenario is
// worth from the scenario itself, instead of writing the number down a second
// time (tasks.md T-018).
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

// FakeNarrator writes the text of a turn from a template table.
type FakeNarrator struct {
	bus         eventbus.Bus
	worldID     string
	lawsVersion string
	log         *slog.Logger

	mu    sync.Mutex
	known map[string]character

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
		known: make(map[string]character),
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

// Start opens the three subscriptions the narrator needs: the player actions
// it tells about, the rounds it closes, and the facts of State it learns the
// characters from.
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
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.subErr
}

// Observe folds a fact of State into what the narrator knows about a
// character. It is the handler Start subscribes system_events with, exported
// so that a test can hand the narrator a fact directly.
func (n *FakeNarrator) Observe(_ context.Context, ev eventbus.Event) error {
	if ev.Type != TypeCreated && ev.Type != TypeUpdated {
		return nil
	}
	var p factPayload
	if err := decodePayload(ev.Payload, &p); err != nil {
		n.log.Warn("a fact the narrator cannot read", "event_id", ev.ID, "err", err)
		return nil
	}
	id := p.Entity.Entity.ID
	if id == "" {
		return nil
	}

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
	return nil
}

// Narrate answers one event with a narrative.output, or with nothing when the
// type is not one the stub tells about. It is the handler Start subscribes
// player_events and game_events with.
//
// An event of another world is passed over: a narrator is a role inside one
// world, and a text about a world it does not serve would reach the players of
// that world through the gateway as if it were theirs.
func (n *FakeNarrator) Narrate(ctx context.Context, ev eventbus.Event) error {
	kind, tells := KindFor(ev.Type)
	if !tells {
		return nil
	}
	if world := eventbus.GetWorldIDFromEvent(ev); world != "" && world != n.worldID {
		n.log.Debug("event of another world passed over",
			"event_id", ev.ID, "type", ev.Type, "event_world_id", world)
		return nil
	}
	switch kind {
	case KindEntry:
		return n.entry(ctx, ev)
	case KindRound:
		return n.round(ctx, ev)
	default:
		return nil
	}
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
		n.log.Warn("event names no acting entity", "event_id", ev.ID, "type", ev.Type)
		return nil
	}
	fields := n.fieldsFor(p.Entity.Entity.ID, p.Entity.Name)
	if p.Target != nil {
		fields.Place = cmp.Or(p.Target.Name, p.Target.Entity.ID)
	}
	return n.publish(ctx, ev, KindEntry, []map[string]any{who},
		template.Entry(ev.ID, fields), nil)
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
	recipients := make([]map[string]any, 0, 4)
	names := make([]string, 0, 4)
	seen := make(map[string]struct{})
	for _, list := range [][]namedRef{p.Acted, p.AutoDefended, p.Idle} {
		for _, ref := range list {
			who := recipientOf(&ref)
			if who == nil {
				continue
			}
			if _, dup := seen[ref.Entity.ID]; dup {
				continue
			}
			seen[ref.Entity.ID] = struct{}{}
			recipients = append(recipients, who)
			names = append(names, n.nameOfKnown(ref.Entity.ID, ref.Name))
		}
	}
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
	return n.publish(ctx, ev, KindRound, recipients, template.Round(ev.ID, fields), extra)
}

// publish builds and sends the narrative. narrative_event_id is the identifier
// of this very event, copied into the payload so that the gateway can group
// the deliveries of one round without reading the envelope (C-05 v1.1).
func (n *FakeNarrator) publish(ctx context.Context, cause eventbus.Event, kind string,
	recipients []map[string]any, text string, extra map[string]any) error {
	payload := map[string]any{
		"recipients":   recipients,
		"text":         text,
		"generated_by": "template",
		"kind":         kind,
		"locale":       eventbus.DefaultLocale,
		"laws_version": n.lawsVersion,
		"filter": map[string]any{
			"applied":        false,
			"status":         "pass",
			"filter_version": FilterVersion,
		},
		"based_on": []map[string]any{
			{"event": map[string]any{"id": cause.ID, "type": cause.Type}},
		},
	}
	maps.Copy(payload, extra)
	ev := eventbus.Derive(cause, TypeNarrativeOutput, Source, payload, eventbus.WithAgent(Agent))
	ev.Payload["narrative_event_id"] = ev.ID
	if err := n.bus.Publish(ctx, ev); err != nil {
		return fmt.Errorf("testkit/swarm: publish %s (%s): %w", TypeNarrativeOutput, kind, err)
	}
	n.log.Info("narrative written", "kind", kind, "event_id", ev.ID,
		"caused_by", cause.ID, "recipients", len(recipients))
	return nil
}

// fieldsFor fills in what the narrator knows about a character from the facts
// of State, falling back to the name the action carried.
func (n *FakeNarrator) fieldsFor(id, name string) template.Fields {
	n.mu.Lock()
	who, known := n.known[id]
	n.mu.Unlock()
	fields := template.Fields{Name: name, HP: template.Unknown, HPMax: template.Unknown}
	if known {
		fields.Name = cmp.Or(who.name, name)
		fields.HP, fields.HPMax = who.hp, who.hpMax
	}
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
// narrative carries.
func recipientOf(ref *namedRef) map[string]any {
	if ref == nil || ref.Entity.ID == "" || ref.Entity.Type == "" {
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
