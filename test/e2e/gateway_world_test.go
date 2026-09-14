//go:build e2e

// The world of the solo scenarios of the gateway (T-313): the real gateway
// behind its HTTP API, State and the Phase 1 stubs of the swarm on one memory
// bus, and the HTTP harness of ci-harness playing a fixture character through
// it — the same client the bot uses (design.md EPIC-004 §3.1 p. 8, §7).
//
// One process, the way design §7 runs it: --bus=memory is the membus every
// context shares, --id-source=sequence is the sequence of the identifiers of
// the events (testkit.Deterministic) and of the gateway (gatewaytest), the
// clock is one clock.Manual for all of them, --contexts=all is the gateway,
// FakeState and the context MV_SWARM_FAKE=true mounts under the name swarm
// (swarm.FakeContext: FakeEncounter over FixedMechanics and FakeNarrator), and
// --chaos=duplicate is membus.Chaos. The scenarios run in live mode: replay
// publishes no analytics (component gateway-and-bot.md §11.2), and a turn of
// solo-30 without analytics.turn.completed is not one the DoD can count. The
// limit of 30 actions a minute of live is met by moving the clock two seconds
// before every action — one token of the limit — instead of raising it.
//
// The world of the projection comes from the journal: the entity.created of
// the world, the region and the wolf are on the bus before the gateway starts,
// and State is seeded with the same entities (acceptance of T-308). No object
// store is needed.
//
// A step of a scenario is one action and everything it causes. The next action
// is sent only once the step has settled — every proposal answered, every
// decided exchange proposed, every opening and closing of an encounter
// announced, every event the narrator tells about told, the consumer of the
// gateway at the end of each of its topics, the turn completed when the step
// waits for it, and a long poll of the harness quiet with the bus unchanged
// around it — because the identifiers of the events come from one sequence and
// the dice of FixedMechanics are addressed by the identifier of the action: an
// action sent while the step before it still publishes would roll other dice.

package e2e_test

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/internal/gateway/gatewaytest"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
	tkgateway "multiverse-core.io/shared/testkit/gateway"
	tkstate "multiverse-core.io/shared/testkit/state"
	tkswarm "multiverse-core.io/shared/testkit/swarm"
)

const (
	// gatewayFixture is the character the scenarios play, gatewayRegion the
	// region it fights in and gatewayWolf the creature of that region.
	gatewayFixture = "player-A"
	gatewayRegion  = "dark-forest-01"
	gatewayWolf    = "wolf-alpha"

	// gatewayPace is what the clock moves before every action: one token of
	// the limit of live, 30 actions a minute (MV_GATEWAY_RATE_ACTIONS_PER_MIN).
	gatewayPace = 2 * time.Second
	// gatewayTimeoutStep moves the clock past the deadline of a turn nobody
	// tells about (MV_GATEWAY_TURN_TIMEOUT) and over a tick of the sweeper,
	// whose period is a minute: any move of more than a minute crosses one.
	gatewayTimeoutStep = turns.DefaultTimeout + time.Second
	// gatewayQuiet is how long a long poll of the harness has to stay empty
	// for a step to count as settled.
	gatewayQuiet = 40 * time.Millisecond
	// gatewaySettleBudget bounds a step in wall time.
	gatewaySettleBudget = 20 * time.Second
	// gatewayAcceptBudget is the answer 202 of an action on membus (NFR-003:
	// the share of the gateway in ack_latency; the absolute measure is the
	// stand of T-392).
	gatewayAcceptBudget = 100 * time.Millisecond
)

// gatewayOptions are what one run of a scenario varies.
type gatewayOptions struct {
	// duplicate appends every record twice (--chaos=duplicate, NFR-013).
	duplicate bool
}

// gatewayWorld is one run: the contexts on one bus, the harness, and what the
// run has seen so far.
type gatewayWorld struct {
	t       *testing.T
	ctx     context.Context
	opts    gatewayOptions
	bus     *membus.Bus
	clock   *clock.Manual
	state   *tkstate.FakeState
	harness *tkgateway.HTTPHarness
	journal *gatewayJournal
	// gatewayDB is a connection of the test to gateway.db of the gateway,
	// read for the cursors of its consumer and nothing else.
	gatewayDB *sql.DB

	player string
	steps  []gatewayStep
	// accepted are the wall times of the answers 202 of the actions sent by
	// act.
	accepted []time.Duration
}

// gatewayStep is one action of a scenario and what it caused.
type gatewayStep struct {
	action      string
	correlation string
	// end is the number of events of the journal when the step settled.
	end        int
	deliveries []api.Delivery
}

func newGatewayWorld(t *testing.T, opts gatewayOptions) *gatewayWorld {
	t.Helper()
	testkit.Deterministic(t, "e2e")
	manual := clock.NewManual(gatewaytest.Epoch)
	// The envelopes are stamped by the clock the gateway reads, so that a
	// timing of a turn and the timestamp of its action are one time.
	eventbus.SetClock(manual)

	fixtures := loadFixtures(t)
	worldID := worldOf(t, fixtures)
	world := without(fixtures, entity.TypePlayer)
	discard := slog.New(slog.NewJSONHandler(io.Discard, nil))

	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: topics,
		Backoff: []time.Duration{0, 0, 0}, Chaos: membus.Chaos{Duplicate: opts.duplicate}})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	fake, err := tkstate.New(tkstate.Config{Bus: bus, WorldID: worldID, RulesVersion: "0.1",
		Store: objstore.NewMemoryWithClock(manual), Clock: manual})
	if err != nil {
		t.Fatalf("fake state: %v", err)
	}
	if err := fake.Seed(world); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	if err := fake.Start(ctx); err != nil {
		t.Fatalf("start state: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = fake.Wait()
	})

	// The stub of the swarm as the hook MV_SWARM_FAKE=true mounts it
	// (cmd/multiverse/fake_contexts.go), with the paths of this package.
	swarm := tkswarm.NewFakeContext(tkswarm.ContextConfig{WorldID: worldID,
		RulesPath: filepath.Join("..", "..", "rules", "dark-forest.yaml"), FixturesDir: fixturesDir(), Log: discard})
	if err := swarm.Start(ctx, runtime.Deps{Bus: bus, Log: discard}); err != nil {
		t.Fatalf("start swarm: %v", err)
	}
	t.Cleanup(func() {
		if err := swarm.Stop(context.Background()); err != nil {
			t.Errorf("stop swarm: %v", err)
		}
	})

	// The world State already holds, told the way State tells a consumer that
	// starts from an empty journal.
	for _, e := range world {
		if err := bus.Publish(ctx, gatewayCreated(e)); err != nil {
			t.Fatalf("fact of %s: %v", e.ID, err)
		}
	}
	gw, err := gatewaytest.Start(gatewaytest.Config{Bus: bus, Clock: manual, Log: discard})
	if err != nil {
		t.Fatalf("gateway: %v", err)
	}
	t.Cleanup(func() {
		if err := gw.Close(); err != nil {
			t.Errorf("close gateway: %v", err)
		}
	})
	// Registered after the gateway, so closed before it.
	gatewayDB, err := store.OpenGateway(ctx, store.GatewayPath(gw.Dir))
	if err != nil {
		t.Fatalf("open gateway.db for its cursors: %v", err)
	}
	t.Cleanup(func() { _ = gatewayDB.Close() })
	harness, err := tkgateway.NewHTTPHarness(tkgateway.NewCIClient(gw.URL), fixtures)
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	return &gatewayWorld{t: t, ctx: ctx, opts: opts, bus: bus, clock: manual, state: fake,
		harness: harness.WithClock(manual), journal: newGatewayJournal(t, bus, topics), gatewayDB: gatewayDB}
}

// gatewayCreated is the entity.created of State for an entity it already
// holds.
func gatewayCreated(e *entity.Entity) eventbus.Event {
	raw, _ := json.Marshal(map[string]any{
		"entity":      map[string]any{"entity": map[string]any{"id": e.ID, "type": e.Type}, "name": e.Name},
		"version":     1,
		"attributes":  e.Attributes,
		"proposal_id": "seed-" + e.ID,
	})
	var payload map[string]any
	_ = json.Unmarshal(raw, &payload)
	return eventbus.NewRoot(tkstate.TypeCreated, tkstate.Source, e.WorldID, nil, eventbus.ActorSystem, payload)
}

// enter registers the fixture character, walks it into the region and
// settles the step. It is the first turn of every scenario.
func (w *gatewayWorld) enter() {
	w.t.Helper()
	w.clock.Advance(gatewayPace)
	entered, err := w.harness.RegisterAndEnter(w.ctx, gatewayFixture, gatewayRegion)
	if err != nil {
		w.t.Fatalf("register and enter %s: %v", gatewayFixture, err)
	}
	w.player = entered.PlayerID
	w.settle(api.ActionEnter, entered.CorrelationID)
}

// act sends one action of the character and settles its step. A turn nobody
// tells about — say, rest, leave — completes by its deadline: the clock is
// moved past it once the step has settled without it.
func (w *gatewayWorld) act(req api.ActionRequest) string {
	w.t.Helper()
	w.clock.Advance(gatewayPace)
	began := testkit.Wall().Now()
	correlation, err := w.harness.Act(w.ctx, w.player, req)
	took := testkit.Wall().Now().Sub(began)
	if err != nil {
		w.t.Fatalf("turn %d, %s: %v", len(w.steps)+1, req.Type, err)
	}
	w.accepted = append(w.accepted, took)
	w.settle(req.Type, correlation)
	return correlation
}

// refused sends an action the gateway must refuse and returns its error.
func (w *gatewayWorld) refused(req api.ActionRequest) error {
	w.t.Helper()
	w.clock.Advance(gatewayPace)
	_, err := w.harness.Act(w.ctx, w.player, req)
	if err == nil {
		w.t.Fatalf("%s of a character that cannot act was accepted", req.Type)
	}
	w.settle(req.Type, "")
	return err
}

// settle waits until the step of an action has settled and records it.
func (w *gatewayWorld) settle(action, correlation string) {
	w.t.Helper()
	step := gatewayStep{action: action, correlation: correlation}
	told := correlation != "" && gatewayTold(action)
	w.wait(&step, told)
	if correlation != "" && !told {
		w.clock.Advance(gatewayTimeoutStep)
		w.wait(&step, true)
	}
	step.end = w.journal.len()
	w.steps = append(w.steps, step)
}

// gatewayTold reports whether the narrator answers an action, and with it
// whether its turn completes by the acknowledgement of a narrative: the entry
// and the look by their type, an attack and a flight by the last decision of
// their exchange (C-05 "Заглушка").
func gatewayTold(action string) bool {
	switch action {
	case api.ActionAttack, api.ActionFlee:
		return true
	case api.ActionEnter:
		_, told := tkswarm.KindFor(tkgateway.TypeEnteredRegion)
		return told
	case api.ActionLook:
		_, told := tkswarm.KindFor(tkgateway.TypeLooked)
		return told
	}
	return false
}

// wait drains the deliveries of the harness until nothing is pending on the
// bus, the consumer of the gateway has committed every record of its topics,
// the turn of the step is completed when turn says so, and a quiet poll saw
// the journal unchanged. The polls run under the deadline of the step, so a
// gateway that never stops giving deliveries fails the step instead of the
// run.
func (w *gatewayWorld) wait(step *gatewayStep, turn bool) {
	w.t.Helper()
	deadline := testkit.Wall().Now().Add(gatewaySettleBudget)
	ctx, cancel := context.WithDeadline(w.ctx, deadline)
	defer cancel()
	pending := ""
	for {
		before := w.journal.refresh()
		got, err := w.harness.Drain(ctx, gatewayQuiet)
		if err != nil {
			w.t.Fatalf("turn %d (%s) did not settle within %s (%s): %v", len(w.steps)+1, step.action,
				gatewaySettleBudget, cmp.Or(pending, "nothing pending"), err)
		}
		step.deliveries = append(step.deliveries, got...)
		after := w.journal.refresh()
		pending = w.journal.pending()
		if pending == "" && step.action == api.ActionEnter && step.correlation != "" && w.journal.alive(gatewayWolf) &&
			len(w.journal.correlated(tkswarm.TypeEncounterStarted, step.correlation)) == 0 {
			// The one thing an entry causes that the journal cannot tell is
			// still to come: FakeEncounter opens a fight with a living creature
			// that is not fighting anyone, which is every entry of a solo
			// scenario the gateway accepts while the wolf lives.
			pending = "encounter.started of the entry " + step.correlation
		}
		if pending == "" {
			pending = w.consumerBehind()
		}
		if pending == "" && turn && len(w.journal.completed(step.correlation)) == 0 {
			pending = "analytics.turn.completed of " + step.correlation
		}
		if pending == "" && len(got) == 0 && after == before {
			return
		}
		if !testkit.Wall().Now().Before(deadline) {
			w.t.Fatalf("turn %d (%s) did not settle within %s: %s", len(w.steps)+1, step.action,
				gatewaySettleBudget, cmp.Or(pending, "the bus kept changing"))
		}
	}
}

// consumerBehind names the first topic of the consumer of the gateway whose
// committed cursor is short of the last record of the topic, or is empty. The
// cursor is written in the transaction of the effects of the record
// (consumer.Dispatcher.commit), so a cursor at the end means every delivery
// and every step of a turn the topic causes is in gateway.db.
func (w *gatewayWorld) consumerBehind() string {
	w.t.Helper()
	cursors, err := consumer.Cursors(w.ctx, w.gatewayDB)
	if err != nil {
		w.t.Fatalf("read the cursors of the gateway: %v", err)
	}
	for _, topic := range consumer.Topics {
		records, err := w.bus.Records(topic)
		if err != nil {
			w.t.Fatalf("read %s: %v", topic, err)
		}
		last := int64(len(records)) - 1
		if at, ok := cursors[topic]; last >= 0 && (!ok || at < last) {
			if !ok {
				at = -1
			}
			return fmt.Sprintf("the consumer of the gateway on %s at offset %d of %d", topic, at, last)
		}
	}
	return ""
}

// character is the character of the scenario as the gateway reports it.
func (w *gatewayWorld) character() api.CharacterState {
	w.t.Helper()
	c, err := w.harness.Client().Player(w.ctx, w.player)
	if err != nil {
		w.t.Fatalf("GET /v1/players/%s: %v", w.player, err)
	}
	return c
}

// deliveries are the deliveries of every step, in the order they came.
func (w *gatewayWorld) deliveries() []api.Delivery {
	var out []api.Delivery
	for _, s := range w.steps {
		out = append(out, s.deliveries...)
	}
	return out
}

// finish checks what every scenario promises about the run as a whole: nothing
// went to dead letters, every event is valid by the registry, every event was
// carried once (twice when the bus carries every record twice), every fact a
// player hears about reached the player exactly once, and the mechanics a turn
// recorded is the mechanics of that turn.
func (w *gatewayWorld) finish() {
	w.t.Helper()
	assertNothingWasDeadLettered(w.t, w.bus)
	assertEverythingIsValid(w.t, w.bus)
	w.journal.refresh()
	copies := 1
	if w.opts.duplicate {
		copies = 2
	}
	w.journal.assertCopies(copies)
	gatewayAssertDeliveries(w.t, w.journal.all(), w.deliveries(), w.player)
	gatewayAssertMechanics(w.t, w.journal)
}

// --- the journal ---

// gatewayRecord is an event of the journal with its topic and its body.
type gatewayRecord struct {
	topic string
	body  string
	ev    eventbus.Event
}

// gatewayJournal reads the bus incrementally. The records of one event — one
// topic and one id — are one event of the journal. Their number and their
// bodies are kept: the bus carries a record twice under --chaos=duplicate, and
// a context that applies something twice publishes a second record under the
// same id wherever the id is derived from the cause (eventbus.WithCauseID) —
// the turn of the gateway, the facts of State, the end of an encounter.
type gatewayJournal struct {
	t      *testing.T
	bus    *membus.Bus
	topics []string
	read   map[string]int
	// copies and bodies are the number of records and the first body of each
	// topic/id.
	copies map[string]int
	bodies map[string]string
	events []gatewayRecord
}

func newGatewayJournal(t *testing.T, bus *membus.Bus, topics []string) *gatewayJournal {
	return &gatewayJournal{t: t, bus: bus, topics: topics, read: make(map[string]int),
		copies: make(map[string]int), bodies: make(map[string]string)}
}

// refresh reads what was published since the last call and returns the number
// of events of the journal. A record with the id of an earlier one and another
// body is an error at once: it is a second, different event under one id.
func (j *gatewayJournal) refresh() int {
	j.t.Helper()
	for _, topic := range j.topics {
		if topic == eventbus.TopicDeadLetters {
			continue
		}
		records, err := j.bus.Records(topic)
		if err != nil {
			j.t.Fatalf("read %s: %v", topic, err)
		}
		for _, body := range records[j.read[topic]:] {
			var ev eventbus.Event
			if err := json.Unmarshal(body, &ev); err != nil {
				j.t.Fatalf("decode %s: %v", topic, err)
			}
			key := topic + "/" + ev.ID
			j.copies[key]++
			if first, seen := j.bodies[key]; seen {
				if first != string(body) {
					j.t.Errorf("%s %s: a copy with another body\n first: %s\n  copy: %s", key, ev.Type, first, body)
				}
				continue
			}
			j.bodies[key] = string(body)
			j.events = append(j.events, gatewayRecord{topic: topic, body: string(body), ev: ev})
		}
		j.read[topic] = len(records)
	}
	return len(j.events)
}

// assertCopies checks that every event of the journal was carried exactly
// want times.
func (j *gatewayJournal) assertCopies(want int) {
	j.t.Helper()
	for _, r := range j.events {
		if n := j.copies[r.topic+"/"+r.ev.ID]; n != want {
			j.t.Errorf("%s/%s %s: %d records, want %d", r.topic, r.ev.ID, r.ev.Type, n, want)
		}
	}
}

func (j *gatewayJournal) len() int { return len(j.events) }

func (j *gatewayJournal) all() []eventbus.Event {
	out := make([]eventbus.Event, 0, len(j.events))
	for _, r := range j.events {
		out = append(out, r.ev)
	}
	return out
}

// ofType are the events of one type, in the order they were read.
func (j *gatewayJournal) ofType(typ string) []eventbus.Event {
	var out []eventbus.Event
	for _, r := range j.events {
		if r.ev.Type == typ {
			out = append(out, r.ev)
		}
	}
	return out
}

// correlated are the events of one type in the chain of one action.
func (j *gatewayJournal) correlated(typ, correlation string) []eventbus.Event {
	var out []eventbus.Event
	for _, ev := range j.ofType(typ) {
		if correlation != "" && ev.Meta.CorrelationID == correlation {
			out = append(out, ev)
		}
	}
	return out
}

// completed are the analytics.turn.completed of one action.
func (j *gatewayJournal) completed(correlation string) []eventbus.Event {
	return j.correlated(turns.TypeCompleted, correlation)
}

// alive reports whether no fact of State has killed an entity.
func (j *gatewayJournal) alive(id string) bool {
	for _, ev := range j.ofType(tkstate.TypeUpdated) {
		if who, _ := ev.Path().GetString("entity.entity.id"); who == id &&
			gatewayChangedTo(ev, entity.AttrStatus, entity.StatusDead) {
			return false
		}
	}
	return true
}

// pending names the first thing the journal still waits for, or is empty:
//
//   - every proposal has an answer of State after its last publication — a
//     fact, or a refusal the proposer may answer with a retry;
//   - every exchange FakeEncounter decided has its package proposed: the
//     package leaves after the last decision, which the narrator may tell and
//     the harness acknowledge first;
//   - every encounter State created was announced (encounter.started), and
//     every one it resolved was closed (encounter.ended): FakeEncounter
//     publishes both only after the fact;
//   - every event the narrator tells about has its narrative.output, named in
//     based_on: an entry, a look, an opening of an encounter, the last
//     decision of an exchange and the death of a character.
func (j *gatewayJournal) pending() string {
	answered := make(map[string]int)
	proposed := make(map[string]int)
	packaged := make(map[string]bool)
	started := make(map[string]bool)
	ended := make(map[string]bool)
	told := make(map[string]bool)
	for i, r := range j.events {
		pa := r.ev.Path()
		switch r.ev.Type {
		case tkstate.TypeCreateProposed, tkstate.TypeUpdateProposed:
			id, _ := pa.GetString("proposal_id")
			proposed[id] = i
			if r.ev.Source == tkswarm.Source && r.ev.Type == tkstate.TypeUpdateProposed {
				packaged[r.ev.Meta.CorrelationID] = true
			}
		case tkstate.TypeCreated, tkstate.TypeUpdated, tkstate.TypeRejected:
			id, _ := pa.GetString("proposal_id")
			answered[id] = i
		case tkswarm.TypeEncounterStarted:
			id, _ := pa.GetString("encounter.entity.id")
			started[id] = true
		case tkswarm.TypeEncounterEnded:
			id, _ := pa.GetString("encounter.entity.id")
			ended[id] = true
		case tkswarm.TypeNarrativeOutput:
			refs, _ := pa.GetSlice("based_on")
			for k := range refs {
				if id, ok := pa.GetString("based_on[" + strconv.Itoa(k) + "].event.id"); ok {
					told[id] = true
				}
			}
		}
	}
	for id, at := range proposed {
		if last, ok := answered[id]; !ok || last < at {
			return "an answer of State to proposal " + id
		}
	}
	for _, r := range j.events {
		ev := r.ev
		pa := ev.Path()
		switch {
		case ev.Type == tkstate.TypeCreated && gatewayEntityType(pa) == entity.TypeEncounter:
			if id, _ := pa.GetString("entity.entity.id"); !started[id] {
				return "encounter.started of " + id
			}
		case ev.Type == tkswarm.TypeCombatDecided && gatewayLast(ev) && !packaged[ev.Meta.CorrelationID]:
			return "the package of the exchange of " + ev.Meta.CorrelationID
		case ev.Type == tkstate.TypeUpdated && gatewayEntityType(pa) == entity.TypeEncounter &&
			gatewayChangedTo(ev, entity.AttrState, entity.EncounterStateResolved):
			if id, _ := pa.GetString("entity.entity.id"); !ended[id] {
				return "encounter.ended of " + id
			}
		case gatewayNarrated(ev) && !told[ev.ID]:
			return "the narrative of " + ev.Type + " " + ev.ID
		}
	}
	return ""
}

// gatewayNarrated reports whether FakeNarrator answers ev with a text of its
// own (README of testkit/swarm, "FakeNarrator — что делает").
func gatewayNarrated(ev eventbus.Event) bool {
	pa := ev.Path()
	switch ev.Type {
	case tkswarm.TypeCombatDecided:
		return gatewayLast(ev)
	case tkstate.TypeUpdated:
		return gatewayEntityType(pa) == entity.TypePlayer && gatewayChangedTo(ev, entity.AttrStatus, entity.StatusDead)
	}
	_, told := tkswarm.KindFor(ev.Type)
	return told
}

// gatewayLast reports whether a combat.decided is the last decision of its
// exchange.
func gatewayLast(ev eventbus.Event) bool {
	last, _ := ev.Path().GetBool("exchange.last")
	return last
}

func gatewayEntityType(pa interface {
	GetString(string) (string, bool)
}) string {
	typ, _ := pa.GetString("entity.entity.type")
	return typ
}

// gatewayChangedTo reports whether a fact of State changed path to value.
func gatewayChangedTo(ev eventbus.Event, path, value string) bool {
	pa := ev.Path()
	changed, _ := pa.GetSlice("changed")
	for k := range changed {
		prefix := "changed[" + strconv.Itoa(k) + "]."
		if p, _ := pa.GetString(prefix + "path"); p != path {
			continue
		}
		if v, _ := pa.GetString(prefix + "new"); v == value {
			return true
		}
	}
	return false
}

// --- what a player hears ---

// gatewayAssertDeliveries checks the deliveries of a run against its journal:
// no delivery came twice, and every fact a player hears about came exactly
// once — a narrative.output as its narrative, a combat.decided as its
// mechanics, an opened encounter as its world_event and a death as the system
// text that offers /start (component gateway-and-bot.md §8.1).
func gatewayAssertDeliveries(t *testing.T, journal []eventbus.Event, deliveries []api.Delivery, player string) {
	t.Helper()
	byID := make(map[string]int)
	byEvent := make(map[string]int)
	kinds := make(map[string]int)
	for _, d := range deliveries {
		byID[d.ID]++
		byEvent[d.Kind+"/"+d.EventID]++
		kinds[d.Kind]++
		if d.PlayerID != player || d.Route == nil || d.Route.ExternalID != gatewayFixture || d.Text == "" {
			t.Errorf("delivery %s %s of %s: player %q, route %+v, text %q", d.Kind, d.ID, d.EventID, d.PlayerID, d.Route, d.Text)
		}
	}
	for id, n := range byID {
		if n != 1 {
			t.Errorf("delivery %s came %d times", id, n)
		}
	}
	opened := 0
	for _, ev := range journal {
		pa := ev.Path()
		switch {
		case ev.Type == tkswarm.TypeNarrativeOutput:
			if n := byEvent[outbox.KindNarrative+"/"+ev.ID]; n != 1 {
				kind, _ := pa.GetString("kind")
				t.Errorf("narrative %s (%s) delivered %d times, want once", ev.ID, kind, n)
			}
		case ev.Type == tkswarm.TypeCombatDecided:
			if n := byEvent[outbox.KindMechanics+"/"+ev.ID]; n != 1 {
				t.Errorf("combat.decided %s delivered %d times as mechanics, want once", ev.ID, n)
			}
		case ev.Type == tkswarm.TypeEncounterStarted:
			opened++
		case ev.Type == tkstate.TypeUpdated && gatewayEntityType(pa) == entity.TypePlayer &&
			gatewayChangedTo(ev, entity.AttrStatus, entity.StatusDead):
			system := 0
			for _, d := range deliveries {
				if d.Kind == outbox.KindSystem && d.EventID == ev.ID {
					system++
					if !strings.Contains(d.Text, "/start") {
						t.Errorf("the death of %s is told %q, without the offer of /start", player, d.Text)
					}
				}
			}
			if system != 1 {
				t.Errorf("the death %s delivered %d times as system, want once", ev.ID, system)
			}
		}
	}
	if kinds[outbox.KindWorldEvent] != opened {
		t.Errorf("%d world_event deliveries for %d opened encounters", kinds[outbox.KindWorldEvent], opened)
	}
}

// --- the journal of a run, for comparing runs ---

// gatewayIDToken is what an identifier of a run looks like: an event id of the
// sequence (e2e-12), an id of the gateway (gw-3, player-gw-1) or an id derived
// from a cause (a UUID, d-<UUID>), wherever it stands in a string.
var gatewayIDToken = regexp.MustCompile(`(?:e2e|gw)-\d+|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// canonical is the journal and the deliveries of a run, step by step, in a
// form two runs of the same scenario must agree on byte for byte.
//
// What a run of the scenario does not decide is taken out, and nothing else:
//
//   - the order of the events of one step: the contexts publish from
//     goroutines of their own, so the lines of a step are sorted;
//   - the identifiers two contexts draw from the one sequence at the same
//     moment: an identifier is replaced by the number of its first appearance
//     in the sorted journal, so that every reference to it — causation,
//     proposal, based_on, a delivery — still has to point at the same event;
//   - which narrative a turn with several of them names: FakeNarrator tells
//     the entry and the opening of a fight, or the last blow and the death,
//     from two subscriptions of its own, the two land in narrative_output in
//     either order, and the gateway names the first it reads
//     (turns.Tracker.OnNarrative). The narratives of such a turn are compared
//     as a set, like every event of the step, and delivery.narrative_event_id
//     is replaced by gatewayNarrativeOfTurn; the narrative every turn names is
//     checked to be one of its own, whatever their number (gatewayNormalized);
//   - which of the two events of the opening of an encounter a world_event
//     delivery names: the gateway opens the encounter by encounter.started or
//     by the entity.created of the encounter, whichever its consumer reads
//     first (consumer.Deliveries.OnEncounterOpened), and the two come on
//     different topics. The event is checked to be one of the two and
//     replaced by gatewayOpeningOf;
//   - created_at of a delivery, the moment the consumer of the gateway wrote
//     it: a measure of the consumer, not of the scenario.
//
// Everything else — types, payloads, timestamps of the events, texts, the
// order of the steps — is compared as it was published. That includes the
// fields of the mechanics of a turn: a turn with Phase 1 completes only once
// its mechanics is recorded (component gateway-and-bot.md §7.7), so they no
// longer depend on which topic the gateway read first.
func (w *gatewayWorld) canonical() string {
	w.t.Helper()
	w.journal.refresh()
	aliases := make(map[string]string)
	alias := func(s string) string {
		return gatewayIDToken.ReplaceAllStringFunc(s, func(id string) string {
			if a, ok := aliases[id]; ok {
				return a
			}
			a := "#" + strconv.Itoa(len(aliases)+1)
			aliases[id] = a
			return a
		})
	}
	mask := func(s string) string { return gatewayIDToken.ReplaceAllString(s, "#") }

	narratives := map[string]map[string]bool{}
	for _, ev := range w.journal.ofType(tkswarm.TypeNarrativeOutput) {
		if narratives[ev.Meta.CorrelationID] == nil {
			narratives[ev.Meta.CorrelationID] = map[string]bool{}
		}
		// A turn names the narrative_event_id its narrative carries, and the id
		// of the narrative only when it carries none (turns.Tracker.OnNarrative).
		named := ev.ID
		if id, ok := ev.Path().GetString("narrative_event_id"); ok && id != "" {
			named = id
		}
		narratives[ev.Meta.CorrelationID][named] = true
	}

	openings := map[string]map[string]bool{}
	for _, r := range w.journal.events {
		pa := r.ev.Path()
		id := ""
		switch {
		case r.ev.Type == tkswarm.TypeEncounterStarted:
			id, _ = pa.GetString("encounter.entity.id")
		case r.ev.Type == tkstate.TypeCreated && gatewayEntityType(pa) == entity.TypeEncounter:
			id, _ = pa.GetString("entity.entity.id")
		default:
			continue
		}
		if openings[id] == nil {
			openings[id] = map[string]bool{}
		}
		openings[id][r.ev.ID] = true
	}

	var b strings.Builder
	from := 0
	for i, step := range w.steps {
		lines := make([]string, 0, step.end-from)
		for _, r := range w.journal.events[from:step.end] {
			lines = append(lines, r.topic+" "+gatewayNormalized(w.t, r, narratives[r.ev.Meta.CorrelationID]))
		}
		from = step.end
		for _, d := range step.deliveries {
			if d.Kind == outbox.KindWorldEvent {
				encounter, _ := d.Data["encounter_id"].(string)
				if !openings[encounter][d.EventID] {
					w.t.Errorf("world_event %s names %s, not an event of the opening of %q", d.ID, d.EventID, encounter)
				}
				d.EventID = gatewayOpeningOf
			}
			raw, err := json.Marshal(struct {
				api.Delivery
				CreatedAt string `json:"created_at"`
			}{Delivery: d})
			if err != nil {
				w.t.Fatal(err)
			}
			lines = append(lines, "delivery "+string(raw))
		}
		slices.SortStableFunc(lines, func(a, b string) int { return strings.Compare(mask(a), mask(b)) })
		fmt.Fprintf(&b, "step %d %s\n", i+1, step.action)
		for _, line := range lines {
			b.WriteString(alias(line))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// gatewayOpeningOf is what the event of a world_event delivery becomes in the
// canonical journal.
const gatewayOpeningOf = "an event of the opening of the encounter"

// gatewayNarrativeOfTurn is what delivery.narrative_event_id of a turn with
// several narratives becomes in the canonical journal.
const gatewayNarrativeOfTurn = "one of the narratives of the turn"

// gatewayNormalized is the body of a record for the canonical journal. The
// narrative an analytics.turn.completed names is checked to be one of the
// narratives of its correlation, and a turn without any names none (Mi-3 of
// review #2 of T-313); when there are several, the one it names is replaced by
// gatewayNarrativeOfTurn.
func gatewayNormalized(t *testing.T, r gatewayRecord, narratives map[string]bool) string {
	t.Helper()
	if r.ev.Type != turns.TypeCompleted {
		return r.body
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(r.body), &doc); err != nil {
		t.Fatalf("decode %s: %v", r.ev.ID, err)
	}
	payload, _ := doc["payload"].(map[string]any)
	delivery, _ := payload["delivery"].(map[string]any)
	named, has := delivery["narrative_event_id"].(string)
	switch {
	case len(narratives) == 0 && has:
		t.Errorf("turn.completed %s names the narrative %q, and its turn has none", r.ev.ID, named)
	case len(narratives) > 0 && !narratives[named]:
		t.Errorf("turn.completed %s names the narrative %q, not one of the narratives of its turn %v",
			r.ev.ID, named, slices.Sorted(maps.Keys(narratives)))
	}
	if len(narratives) > 1 {
		delivery["narrative_event_id"] = gatewayNarrativeOfTurn
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// gatewayMechanicsOf are the actions whose turn has a Phase 1 — the column
// "Phase 1" of api-contracts.md §1.4 — with the type of the event that is
// their mechanics (component gateway-and-bot.md §7.7): the decision of a fight
// or a flight, the fact of State of a move or a rest.
var gatewayMechanicsOf = map[string]string{
	api.ActionEnter:  tkstate.TypeUpdated,
	api.ActionLeave:  tkstate.TypeUpdated,
	api.ActionAttack: tkswarm.TypeCombatDecided,
	api.ActionFlee:   tkswarm.TypeCombatDecided,
	api.ActionRest:   tkstate.TypeUpdated,
}

// gatewayAssertMechanics holds the mechanics a turn recorded to the journal.
// A turn with Phase 1 whose narrative was delivered (timings.narrative_at) has
// its mechanics (component gateway-and-bot.md §7.7). A turn that names its
// mechanics names an event of its own chain that is a mechanics — for an
// action with Phase 1 the event of the type of its action (gatewayMechanicsOf),
// otherwise a decision or a fact of State — with the mode and the level of
// detail of a decision of that chain, and its mechanics_at is not before received_at and not after
// the deadline of the turn, received_at + turns.DefaultTimeout. The
// acknowledgement of the narrative is no upper bound — the mechanics of a turn
// may come after it — and neither is the timestamp of turn.completed, which is
// the moment of the action (turns.Completed).
func gatewayAssertMechanics(t *testing.T, j *gatewayJournal) {
	t.Helper()
	byID := map[string]eventbus.Event{}
	for _, r := range j.events {
		byID[r.ev.ID] = r.ev
	}
	for _, ev := range j.ofType(turns.TypeCompleted) {
		pa := ev.Path()
		result, hasResult := pa.GetString("delivery.result_event_id")
		at, hasAt := pa.GetString("timings.mechanics_at")
		if hasResult != hasAt {
			t.Errorf("turn.completed %s: result_event_id %q and mechanics_at %q, want both or neither", ev.ID, result, at)
			continue
		}
		action, _ := pa.GetString("turn.action_type")
		want, withPhase1 := gatewayMechanicsOf[action]
		if withPhase1 && pa.Has("timings.narrative_at") && !hasResult {
			t.Errorf("turn.completed %s: a turn of %s with its narrative delivered and without its mechanics", ev.ID, action)
		}
		if !hasResult {
			continue
		}
		mech, ok := byID[result]
		if !ok || mech.Meta.CorrelationID != ev.Meta.CorrelationID ||
			(mech.Type != tkswarm.TypeCombatDecided && mech.Type != tkstate.TypeUpdated) {
			t.Errorf("turn.completed %s names the mechanics %q (%s of %q), want a decision or a fact of %s",
				ev.ID, result, mech.Type, mech.Meta.CorrelationID, ev.Meta.CorrelationID)
		}
		if withPhase1 && mech.Type != want {
			t.Errorf("turn.completed %s of %s names the mechanics %q of type %s, want %s", ev.ID, action, result, mech.Type, want)
		}
		for _, field := range []string{"phase1_mode", "lod"} {
			value, has := pa.GetString("turn." + field)
			if !has {
				continue
			}
			found := false
			for _, decision := range j.correlated(tkswarm.TypeCombatDecided, ev.Meta.CorrelationID) {
				if v, _ := decision.Path().GetString(field); v == value {
					found = true
				}
			}
			if !found {
				t.Errorf("turn.completed %s has %s %q that no decision of its turn carries", ev.ID, field, value)
			}
		}
		receivedAt, _ := pa.GetString("timings.received_at")
		received, errReceived := time.Parse(time.RFC3339Nano, receivedAt)
		mechanics, errMechanics := time.Parse(time.RFC3339Nano, at)
		if err := errors.Join(errReceived, errMechanics); err != nil {
			t.Errorf("turn.completed %s: timings: %v", ev.ID, err)
			continue
		}
		if deadline := received.Add(turns.DefaultTimeout); mechanics.Before(received) || mechanics.After(deadline) {
			t.Errorf("turn.completed %s: mechanics_at %s outside [received_at %s, deadline %s]", ev.ID, at, receivedAt,
				session.Timestamp(deadline))
		}
	}
}

// gatewayAssertSameRun compares two canonical runs and prints the first step
// they differ in, whole, from both: a single line after the renaming of the
// identifiers may be only a consequence of the difference.
func gatewayAssertSameRun(t *testing.T, what string, want, got string) {
	t.Helper()
	if want == got {
		return
	}
	ws, gs := gatewaySteps(want), gatewaySteps(got)
	for i := range min(len(ws), len(gs)) {
		if ws[i] != gs[i] {
			t.Fatalf("%s: the journals differ in step %d\n--- first run ---\n%s\n--- second run ---\n%s", what, i+1, ws[i], gs[i])
		}
	}
	t.Fatalf("%s: the journals differ in length: %d and %d steps", what, len(ws), len(gs))
}

// gatewaySteps splits a canonical journal into its steps.
func gatewaySteps(canonical string) []string {
	blocks := strings.Split(canonical, "\nstep ")
	for i := 1; i < len(blocks); i++ {
		blocks[i] = "step " + blocks[i]
	}
	return blocks
}

// gatewayAssertAccepted checks that the answers 202 of a run came within
// gatewayAcceptBudget at the 95th percentile, the statistic of NFR-003.
func gatewayAssertAccepted(t *testing.T, accepted []time.Duration) {
	t.Helper()
	if len(accepted) == 0 {
		t.Fatal("no action was timed")
	}
	sorted := slices.Clone(accepted)
	slices.Sort(sorted)
	p95 := sorted[(len(sorted)*95+99)/100-1]
	t.Logf("202 of %d actions on membus: p50 %s, p95 %s, max %s", len(sorted), sorted[len(sorted)/2], p95, sorted[len(sorted)-1])
	if p95 > gatewayAcceptBudget {
		t.Errorf("p95 of 202 is %s, want at most %s on membus", p95, gatewayAcceptBudget)
	}
}

// gatewaySessions are the session.started and session.ended of a run.
func gatewaySessions(j *gatewayJournal) (started, ended []eventbus.Event) {
	return j.ofType(session.TypeStarted), j.ofType(session.TypeEnded)
}
