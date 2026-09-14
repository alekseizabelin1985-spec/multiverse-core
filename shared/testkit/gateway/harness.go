// Package gateway is the stand-in for internal/gateway on the publishing
// side: it turns the fixture world into the player actions of C-04 and puts
// them on an in-process bus, so that State and the swarm can be written, run
// and reviewed before the gateway exists (design.md §5, tasks.md T-018).
//
// It is a stub about where an action comes from, never about what an action
// looks like. Everything it publishes is a real event of the registry with
// source testkit/gateway — one of the two publishers contracts.md §0 lists for
// player_events — validated by the bus against the same schema the real
// gateway will be validated against. A consumer that only works when the
// harness drives it is a consumer that read something C-04 never promised.
//
// What Harness deliberately does not do: HTTP. There is no client, no session,
// no action_key and no idempotency window; the harness publishes what a request
// would have produced after the gateway had accepted it. Groups, rounds and
// the analytics of C-10 are not here either. The same fixture characters over
// the HTTP API of the real gateway are HTTPHarness (http.go, T-308), which
// stands next to Harness and changes nothing of it.
//
// Combat is here, and only half of it: Attack and Flee publish the two actions
// of C-04 that open a fight, and somebody else decides what the blow did. The
// only such somebody of Phase 1 is testkit/swarm.FakeEncounter of EPIC-003
// (contracts.md C-05, tasks.md T-219); against a bus where nothing resolves a
// fight, the two methods report that instead of returning a success nobody
// answered for (tasks.md T-400).
package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Source is the envelope source of everything the harness publishes. It is a
// value of the registry, not a name invented here: testkit/gateway is listed
// as a publisher of every player_events type and of both proposals
// (contracts.md §0 v0.4).
const Source = contracts.SourceTestkitGateway

// Group is the consumer group prefix the harness reads under. Each topic gets
// its own name derived from it, because a consumer group is a cursor over one
// topic (C-01): the harness reads system_events to learn what became of the
// changes it proposed, and world_events to learn whether the fight its
// character is in is still on.
const Group = "testkit-gateway"

// group is the consumer group of one topic.
func group(topic string) string { return Group + "-" + topic }

// The event types the harness publishes.
const (
	TypeEnteredRegion  = "player.entered_region"
	TypeLeftRegion     = "player.left_region"
	TypeLooked         = "player.looked"
	TypeSaid           = "player.said"
	TypeRested         = "player.rested"
	TypeAttacked       = "player.attacked"
	TypeFleeAttempted  = "player.flee_attempted"
	TypeCreateProposed = "entity.create.proposed"
	TypeUpdateProposed = "entity.update.proposed"
)

// The types the harness reads to learn the answer to a proposal.
const (
	TypeCreated  = "entity.created"
	TypeUpdated  = "entity.updated"
	TypeRejected = "entity.update.rejected"
)

// The types the harness reads to learn whether a fight is still on. They are
// the lifecycle of C-05, published by whoever runs the encounter, and they are
// all the harness needs to tell "nobody is listening" from "there is nothing
// left to answer this" — no new event type and no change of a contract
// (journal.md 2026-09-11, decision on Cr-2 of T-219).
const (
	TypeEncounterStarted = "encounter.started"
	TypeEncounterEnded   = "encounter.ended"
)

// ReasonVersionConflict is the one refusal of C-02 that says nothing about
// whoever proposed the change. It means only that somebody moved an entity of
// the package between the moment the package was built and the moment State
// was asked to apply it — the case an optimistic lock exists for, and one the
// harness itself causes: it moves the character it plays, while the encounter
// answers a blow from a view it fills over a subscription of its own, and C-01
// orders no two topics against each other.
//
// The answer to it belongs to the publisher, not to the harness: fold in what
// arrived and offer the same package again, a bounded number of times
// (decision of 2026-09-11 on the residual race of T-400; testkit/swarm gives
// an action three attempts). So the harness treats it as a race rather than as
// an answer — see record and awaitReaction. Every other reason is a defect and
// ends the wait on the spot.
const ReasonVersionConflict = "version_conflict"

// ErrFightOver is what Attack and Flee report instead of waiting out the
// timeout when the harness already knows there is no fight left to answer
// them: the encounter ended, the character is a corpse, or the target is.
//
// A repeated step of a script stops on it rather than failing (see Run): a
// fight that ended between two blows is how a fight ends, not a broken script.
var ErrFightOver = errors.New("the fight is over")

// The causes of C-02 a gateway attaches to its proposals: a character is
// created, a character moves, a character rests.
const (
	CauseCreate = "create"
	CauseMove   = "move"
	CauseRest   = "rest"
)

// DefaultTimeout is how long an action waits for State to answer the proposal
// it published. It is wall time on purpose: State applies proposals in a
// goroutine, and a manual clock cannot say how much of it that goroutine has
// actually had.
const DefaultTimeout = 5 * time.Second

// Outside is where a character stands when it is in the world but in no
// region. The fixtures start every player there (testdata/fixtures).
func Outside(worldID string) string { return "outside:" + worldID }

// Harness publishes player actions on behalf of the fixture characters and
// keeps track of what State answered.
type Harness struct {
	bus     eventbus.Bus
	worldID string
	log     *slog.Logger
	timeout time.Duration

	// fixtures is the world the harness knows, by identifier: the characters
	// it can create and the regions it can walk them into.
	fixtures map[string]*entity.Entity

	mu sync.Mutex
	// versions is what State last said an entity is at — the optimistic lock
	// the next proposal pins (ADR-013 p. 1).
	versions map[string]int64
	// positions is where the harness last put a character, so that
	// position.from of a movement is what the previous action left behind.
	positions map[string]string
	// settled maps a proposal identifier to the reason it was refused, or to
	// the empty string once it was applied.
	settled map[string]string
	// awaited maps the correlation identifier of an action the harness is
	// waiting on to what somebody else proposed in answer to it. Only Attack
	// and Flee put anything here: every other action of the harness proposes
	// its own change and waits by proposal identifier.
	awaited map[string]*reaction
	// fights is the last encounter the harness saw a character in, by
	// character. It is what tells an action nobody will ever answer from an
	// action whose answer is still on its way.
	fights map[string]*fight
	// endings is the reason every encounter the harness has seen end ended
	// with, by encounter. An end can reach the harness before the start does —
	// the fact of the encounter entity on system_events overtaking
	// encounter.started on world_events — and a start heard after its end must
	// not reopen the fight (C-04 v1.3).
	endings map[string]string
	// statuses is the last status a fact gave an entity. The harness reads it
	// for one question only — is this one past acting — so it holds every
	// status rather than a flag, and answers "unknown" by holding nothing.
	statuses map[string]string
	// changed is closed and replaced whenever settled or awaited grows, so
	// that a waiting action wakes without polling.
	changed chan struct{}
	seq     int

	subscription sync.WaitGroup
	subErr       error
	started      bool
}

// fight is the encounter a character was last seen in, as the lifecycle of
// C-05 describes it from the outside: an identifier, and the reason it closed.
//
// Two things are deliberately absent. The harness does not track the round,
// the participants or the damage — that is the read-model of a gateway, and
// this is a stub of where an action comes from. And it never concludes "there
// is no fight" from silence: encounter.started travels on world_events while
// the answer to a blow travels on system_events, so a harness that has just
// seen its blow land may not have heard the encounter open or close yet.
// Knowing a fight is over is a fact; not knowing is not one.
type fight struct {
	id string
	// ended is the reason of encounter.ended, empty while the fight is on.
	ended string
}

// reaction is the change somebody else proposed in answer to one action of the
// harness, and how much of it State has already answered for.
//
// It is filled in from two events of system_events, in either order: the
// proposal says which entities are going to move (named), the facts say which
// of them have moved (answered). The order does not matter because a bus that
// reordered the two would still leave the same set difference behind.
type reaction struct {
	// player is the character whose action is being answered, so that the end
	// of its fight can stop a wait that has nothing left to wait for.
	player string
	// proposalID is the identifier the proposal named itself by, kept for the
	// error message of a refusal.
	proposalID string
	// named is the entities of the proposal; nil until the proposal is seen.
	named map[string]struct{}
	// answered is the entities State has published a fact for.
	answered map[string]struct{}
	// refusal is the reason State gave for turning the whole proposal down.
	// A version conflict is never one of them: it is a race, and it is counted
	// in conflicts instead.
	refusal string
	// conflicts is how many times a package of this action lost the version
	// race. It is what tells the two ends of a deadline apart — a publisher
	// that ran out of attempts from a bus where nobody ever answered — and it
	// is a count and not a flag because "it kept losing" and "it lost once"
	// are different stories for whoever reads the failure.
	conflicts int
	// over is the reason the encounter closed while the action was still
	// waiting for an answer that is therefore never coming.
	over string
}

// done reports whether the world has come to rest around the action: the
// proposal is refused, or every entity it named has a fact.
func (r *reaction) done() bool {
	if r == nil {
		return false
	}
	if r.refusal != "" || r.over != "" {
		return true
	}
	if r.named == nil {
		return false
	}
	for id := range r.named {
		if _, has := r.answered[id]; !has {
			return false
		}
	}
	return true
}

// progress is how far the answer to the action got, for the message of a wait
// that ran out. Its three ends are three different defects in three different
// places — nobody answered at all, somebody answered for less than they
// proposed to change, and somebody kept losing the version race until they gave
// up — and a message that did not tell them apart would send the next reader to
// the wrong half of the bus.
//
// The third is the one a deadline reports without anything being broken here:
// the harness waits through a version conflict because the publisher is
// expected to offer the package again, and a publisher out of attempts leaves
// exactly this behind — a package that was refused, offered, refused, and is
// never coming back.
func (r *reaction) progress() string {
	if r == nil || (r.named == nil && r.conflicts == 0) {
		return "an encounter answers every action it takes with one proposal of change, " +
			"and none arrived"
	}
	if r.conflicts > 0 {
		return fmt.Sprintf("the change proposed for it (%s) lost the version race and never "+
			"came back as a fact: this is a publisher that ran out of retries and gave up, "+
			"not silence on the bus (version conflicts: %d, entities with a fact: %d of %d)",
			r.proposalID, r.conflicts, len(r.answered), len(r.named))
	}
	return fmt.Sprintf("the change proposed for it (%s) named %d entities and State has "+
		"published a fact for %d of them", r.proposalID, len(r.named), len(r.answered))
}

// NewHarness builds the harness over a bus and the entities of a fixture
// world (state.LoadFixtures). The world is the one the fixture world entity
// names: a harness driving two worlds at once is not a thing C-04 describes.
//
// Nothing is published and nothing is read until Start.
func NewHarness(bus eventbus.Bus, fixtures []*entity.Entity) (*Harness, error) {
	if bus == nil {
		return nil, errors.New("testkit/gateway: no bus")
	}
	byID := make(map[string]*entity.Entity, len(fixtures))
	worldID := ""
	for _, e := range fixtures {
		if e == nil || e.ID == "" {
			return nil, errors.New("testkit/gateway: a fixture without an id")
		}
		if _, seen := byID[e.ID]; seen {
			return nil, fmt.Errorf("testkit/gateway: %s appears twice in the fixtures", e.ID)
		}
		byID[e.ID] = entity.Clone(e)
		if e.Type == entity.TypeWorld {
			if worldID != "" {
				return nil, errors.New("testkit/gateway: the fixtures hold more than one world")
			}
			worldID = e.ID
		}
	}
	if worldID == "" {
		return nil, errors.New("testkit/gateway: the fixtures hold no world")
	}
	return &Harness{
		bus:       bus,
		worldID:   worldID,
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)).With("component", Source),
		timeout:   DefaultTimeout,
		fixtures:  byID,
		versions:  make(map[string]int64),
		positions: make(map[string]string),
		settled:   make(map[string]string),
		awaited:   make(map[string]*reaction),
		fights:    make(map[string]*fight),
		endings:   make(map[string]string),
		statuses:  make(map[string]string),
		changed:   make(chan struct{}),
	}, nil
}

// WithLog sends what the harness did to a logger instead of discarding it. It
// returns the harness so that it can be chained onto NewHarness.
//
// Like WithTimeout it configures the harness and is therefore called before
// Start: after Start the subscription reads these fields, and a setter that
// took the lock would only make a caller believe reconfiguring mid-run is
// something the harness supports.
func (h *Harness) WithLog(log *slog.Logger) *Harness {
	if log != nil {
		h.log = log.With("component", Source)
	}
	return h
}

// WithTimeout changes how long an action waits for the answer to its
// proposal. A non-positive value restores DefaultTimeout. Call it before
// Start (see WithLog).
//
// The default is meant for a real State and is worth shortening in a set that
// runs a whole scenario: the timeout is per action, so a script pays it once
// per step that nobody answers. What used to cost the most — a blow struck
// after the fight was over — costs nothing now (see Attack), but a stub that
// answers no action at all still costs one timeout per step.
func (h *Harness) WithTimeout(d time.Duration) *Harness {
	if d <= 0 {
		d = DefaultTimeout
	}
	h.timeout = d
	return h
}

// WorldID is the world the harness acts in.
func (h *Harness) WorldID() string { return h.worldID }

// Start subscribes to the two topics the harness reads: system_events, where
// the answers to its proposals arrive, and world_events, where the encounter
// its character fights in opens and closes.
//
// Unlike State the harness announces nothing at start: a gateway is not a
// stateful context of C-14 that consumers wait for, it is the thing that acts.
// Start returns once the subscription goroutine is running; an action that
// needs an answer waits for that answer rather than for the subscription.
func (h *Harness) Start(ctx context.Context) error {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return errors.New("testkit/gateway: already started")
	}
	h.started = true
	h.mu.Unlock()

	for _, topic := range []string{eventbus.TopicSystemEvents, eventbus.TopicWorldEvents} {
		h.subscribe(ctx, topic)
	}
	return nil
}

// subscribe runs one subscription of the harness until its context is done and
// remembers the first failure among them.
func (h *Harness) subscribe(ctx context.Context, topic string) {
	h.subscription.Add(1)
	go func() {
		defer h.subscription.Done()
		if err := h.bus.Subscribe(ctx, topic, group(topic), h.Observe); err != nil {
			h.mu.Lock()
			if h.subErr == nil {
				h.subErr = fmt.Errorf("testkit/gateway: subscribe %s: %w", topic, err)
			}
			h.mu.Unlock()
			h.log.Error("subscription stopped", "topic", topic, "err", err)
		}
	}()
}

// Wait blocks until the subscription started by Start has stopped and reports
// what stopped it. A subscription cancelled through its context is not a
// failure and reports nil (C-01).
func (h *Harness) Wait() error {
	h.subscription.Wait()
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.subErr
}

// Observe folds one event of the topics the harness reads into what it knows.
// It is the handler Start subscribes with, and it is exported so that a test
// can hand the harness a fact directly instead of running a subscription.
//
// It is the read-model of the gateway cut down to what a harness needs: the
// version and the status of every entity, the fate of every proposal of its
// own, how far State has got with a proposal somebody else made in answer to
// an action of the harness, and whether the fight its character is in is still
// on. Nothing else here reads the world.
func (h *Harness) Observe(_ context.Context, ev eventbus.Event) error {
	pa := ev.Path()
	switch ev.Type {
	case TypeCreated, TypeUpdated:
		id, _ := pa.GetString("entity.entity.id")
		version, _ := pa.GetInt("version")
		proposal, _ := pa.GetString("proposal_id")
		// The end of a fight goes in before the fact wakes anybody: the action
		// the package answered returns on this very fact, and the next step of
		// a script should find the fight over. The order narrows the window
		// rather than closing it, and nothing depends on it being closed: a
		// step that slips through is still woken by end with ErrFightOver
		// (N-1 of review #1 of T-419).
		if reason, resolved := resolutionIn(ev); resolved {
			h.end(id, reason, ev.CorrelationID())
		}
		h.record(ev.CorrelationID(), id, int64(version), proposal, "", statusIn(ev))
	case TypeRejected:
		proposal, _ := pa.GetString("proposal_id")
		reason, _ := pa.GetString("reason")
		if reason == "" {
			reason = "unknown"
		}
		h.record(ev.CorrelationID(), "", 0, proposal, reason, "")
	case TypeCreateProposed, TypeUpdateProposed:
		// A proposal of the harness is waited for by its own identifier; only
		// a proposal somebody else made in answer to an action of the harness
		// has to be discovered from the bus.
		if ev.Source != Source {
			h.expect(ev)
		}
	case TypeEncounterStarted:
		h.opened(ev)
	case TypeEncounterEnded:
		h.closed(ev)
	}
	return nil
}

// statusIn is the status a fact gave the entity, or the empty string when the
// fact changed no status. It is read from changed[] because that is where C-02
// puts what an update did — a fact carries the paths that moved, not the whole
// entity.
func statusIn(ev eventbus.Event) string {
	pa := ev.Path()
	changed, _ := pa.GetSlice("changed")
	for i := range changed {
		path, _ := pa.GetString(fmt.Sprintf("changed[%d].path", i))
		if path != entity.AttrStatus {
			continue
		}
		status, _ := pa.GetString(fmt.Sprintf("changed[%d].new", i))
		return status
	}
	return ""
}

// resolutionIn reads the end of a fight off a fact: the encounter entity moved
// to state=resolved, and the reason is the resolution the same fact wrote.
//
// For a consumer of state this fact is the end of the encounter, and
// encounter.ended is the lifecycle event that follows it (C-05 v1.4 p. 5,
// C-04 v1.3): the end is published only after the fact, so a harness that
// waited for the event alone would strike its next blow into a fight State
// has already closed.
func resolutionIn(ev eventbus.Event) (string, bool) {
	pa := ev.Path()
	if kind, _ := pa.GetString("entity.entity.type"); kind != entity.TypeEncounter {
		return "", false
	}
	changed, _ := pa.GetSlice("changed")
	resolved, reason := false, ""
	for i := range changed {
		path, _ := pa.GetString(fmt.Sprintf("changed[%d].path", i))
		value, _ := pa.GetString(fmt.Sprintf("changed[%d].new", i))
		switch path {
		case entity.AttrState:
			resolved = value == entity.EncounterStateResolved
		case entity.AttrResolution:
			reason = value
		}
	}
	if reason == "" {
		reason = entity.EncounterStateResolved
	}
	return reason, resolved
}

// opened folds encounter.started: from here on the harness knows the
// characters it names are in a fight, and which one — unless the fight has
// already been heard to end.
//
// A start heard after its end is also the first moment the harness learns
// whose fight that end closed: the fact of the encounter entity names the
// encounter, not its characters. An action a character took in between went
// out into a fight the harness did not know about, and nobody is left to answer
// it, so its wait ends here the way end ends it (T-433). The action whose
// package closed the fight is never among them: an end heard before its start
// was heard from that fact, and the proposal behind the fact came before it on
// the same topic, so that action already knows what it waits for.
func (h *Harness) opened(ev eventbus.Event) {
	pa := ev.Path()
	id, _ := pa.GetString("encounter.entity.id")
	if id == "" {
		return
	}
	participants, _ := pa.GetSlice("participants")
	h.mu.Lock()
	defer h.mu.Unlock()
	ended := h.endings[id]
	late := make(map[string]struct{}, len(participants))
	for i := range participants {
		who, ok := pa.GetString(fmt.Sprintf("participants[%d].entity.id", i))
		if !ok || who == "" {
			continue
		}
		h.fights[who] = &fight{id: id, ended: ended}
		if ended != "" {
			late[who] = struct{}{}
		}
		h.log.Info("a character of the harness is in a fight",
			"encounter_id", id, "player_id", who)
	}
	if len(late) > 0 {
		h.stopWaits(late, ended, "")
	}
}

// closed folds encounter.ended: the fight is over, and so is every wait that
// was still expecting the encounter to answer an action.
//
// The action that closed the fight is the one exception, and it is told apart
// by its correlation identifier: encounter.ended is derived from the action
// that caused it, and that action was answered by a proposal of its own —
// which the harness may not have folded yet, because the answer travels on
// system_events and the end of the fight on world_events. Failing it here
// would report a blow that landed as a blow nobody took.
func (h *Harness) closed(ev eventbus.Event) {
	pa := ev.Path()
	id, _ := pa.GetString("encounter.entity.id")
	reason, _ := pa.GetString("reason")
	if reason == "" {
		reason = "unknown"
	}
	h.end(id, reason, ev.CorrelationID())
}

// end is the fight over, from whichever of its two ends arrived first: the fact
// of the encounter entity or encounter.ended (C-04 v1.3). correlation is the
// action whose package closed the fight, which is still answered.
func (h *Harness) end(id, reason, correlation string) {
	if id == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, heard := h.endings[id]; !heard {
		h.endings[id] = reason
	}
	closed := make(map[string]struct{}, len(h.fights))
	for who, f := range h.fights {
		if f.id != id || f.ended != "" {
			continue
		}
		f.ended = reason
		closed[who] = struct{}{}
	}
	if len(closed) == 0 {
		return
	}
	h.log.Info("the fight of the harness is over", "encounter_id", id, "reason", reason)
	h.stopWaits(closed, reason, correlation)
}

// stopWaits ends every wait of these characters that nothing was proposed for:
// their fight is over, and no answer is coming. correlation is the action whose
// package closed the fight, which is still answered. The caller holds the lock.
func (h *Harness) stopWaits(players map[string]struct{}, reason, correlation string) {
	woke := false
	for waiting, r := range h.awaited {
		if _, fighting := players[r.player]; !fighting {
			continue
		}
		if waiting == correlation || r.named != nil {
			continue
		}
		r.over = reason
		woke = true
	}
	if woke {
		close(h.changed)
		h.changed = make(chan struct{})
	}
}

// record folds one answer into the state of the harness and wakes whoever
// waits for it.
func (h *Harness) record(correlation, entityID string, version int64,
	proposalID, refusal, status string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if entityID != "" {
		h.versions[entityID] = version
		if status != "" {
			h.statuses[entityID] = status
		}
	}
	woke := false
	if proposalID != "" {
		if _, known := h.settled[proposalID]; !known {
			h.settled[proposalID] = refusal
			woke = true
		}
	}
	if r := h.awaited[correlation]; r != nil {
		switch {
		case refusal == ReasonVersionConflict:
			// Not an answer: the package was refused because the world moved
			// under it, and the publisher is expected to fold in what moved and
			// offer the same package again (see ReasonVersionConflict). The
			// wait goes on — through the retry to its facts, or, if the
			// publisher runs out of attempts and says so, to the deadline.
			// Nothing a waiter reads has changed, so nobody is woken.
			r.conflicts++
			h.log.Info("a change proposed for an action of the harness lost the version race",
				"proposal_id", proposalID, "conflicts", r.conflicts,
				"correlation_id", correlation)
		case refusal != "" && r.refusal == "":
			r.refusal = refusal
			woke = true
		case refusal == "" && entityID != "":
			if _, seen := r.answered[entityID]; !seen {
				if r.answered == nil {
					r.answered = make(map[string]struct{}, 2)
				}
				r.answered[entityID] = struct{}{}
				woke = true
			}
		}
	}
	if woke {
		close(h.changed)
		h.changed = make(chan struct{})
	}
}

// expect folds a proposal of somebody else into the reaction the harness is
// waiting for: which entities are about to move, and under which identifier
// State will answer for them.
//
// A proposal under an identifier the action has already seen is the same
// attempt offered again, not a second package: a publisher that lost the
// version race keeps the identifier precisely because it is still answering
// the one action once (testkit/swarm, and §4.5 — State applies an identifier
// once, so a retry that crossed a success on the wire is dropped there). What
// it names may differ from what the first offer named — the package is
// recomputed against the world it will now be applied to, and a target that
// fell in the meantime is not struck again — so the entities the wait expects
// are taken from the latest offer and not from the first.
func (h *Harness) expect(ev eventbus.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.awaited[ev.CorrelationID()]
	if r == nil {
		return
	}
	proposalID, named := proposedChange(ev)
	again := r.named != nil && proposalID != "" && proposalID == r.proposalID
	if r.named != nil && !again {
		// The action has already been answered once, and this is a different
		// package: the first proposal is the one it caused (C-02 — one atomic
		// package per action), and a second is about something the harness
		// never asked for. Returning by the first is what the decision of
		// 2026-09-11 allows until C-04/C-05 say two packages are legal; being
		// quiet about it is not, because the action reports the world at rest
		// while half of it is still moving.
		h.log.Warn("a second proposal for one action was ignored",
			"type", ev.Type, "source", ev.Source, "proposal_id", proposalID,
			"answered_by", r.proposalID, "correlation_id", ev.CorrelationID())
		return
	}
	r.proposalID, r.named = proposalID, named
	if len(r.named) == 0 {
		// A package the harness cannot read an entity out of is a package it
		// can never see the facts of, and an empty set of names would make
		// done() true on the spot: every entity it named has a fact, because
		// it named none. That is the one way this wait can report the world at
		// rest without having seen a single fact, so it is a refusal instead —
		// the defect is in whoever published it (C-02 requires
		// changes[].entity.entity.id), and a silent success would hide it.
		r.refusal = "it named no entity the harness can read " +
			"(C-02: changes[].entity.entity.id, entity.entity.id of a create)"
	}
	what := "somebody answered an action of the harness"
	if again {
		what = "the change proposed for an action of the harness was offered again"
	}
	h.log.Info(what, "type", ev.Type, "source", ev.Source, "proposal_id", r.proposalID,
		"names", len(r.named))
	close(h.changed)
	h.changed = make(chan struct{})
}

// proposedChange reads the identifier of a proposal and the entities it names.
// A create names one entity, an update one per change set (C-02).
func proposedChange(ev eventbus.Event) (string, map[string]struct{}) {
	pa := ev.Path()
	proposalID, _ := pa.GetString("proposal_id")
	named := make(map[string]struct{}, 2)
	if ev.Type == TypeCreateProposed {
		if id, ok := pa.GetString("entity.entity.id"); ok {
			named[id] = struct{}{}
		}
		return proposalID, named
	}
	changes, _ := pa.GetSlice("changes")
	for i := range changes {
		if id, ok := pa.GetString(fmt.Sprintf("changes[%d].entity.entity.id", i)); ok {
			named[id] = struct{}{}
		}
	}
	return proposalID, named
}

// Version is what State last said the entity is at, and whether the harness
// has heard about it at all.
func (h *Harness) Version(id string) (int64, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.versions[id]
	return v, ok
}

// Status is the last status a fact gave the entity, and whether the harness has
// seen one at all. An entity nobody has changed the status of is unknown here
// and alive in the world: the fixtures start every character alive, and the
// harness folds a status only when a fact says it moved.
func (h *Harness) Status(id string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.statuses[id]
	return s, ok
}

// Fight is what the harness knows about the encounter of a character: its
// identifier, the reason it ended — empty while it is on — and whether the
// harness has heard of it at all.
//
// A false here means "not heard of", never "there is none": the fight opens on
// world_events while the harness is busy waiting for a fact on system_events.
func (h *Harness) Fight(playerID string) (id, ended string, known bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	f, ok := h.fights[playerID]
	if !ok {
		return "", "", false
	}
	return f.id, f.ended, true
}

// Position is where the harness last put a character.
func (h *Harness) Position(id string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.positions[id]
	return p, ok
}

// CreatePlayer proposes the fixture character with this identifier, exactly as
// the fixture file describes it, and waits until State has created it.
//
// The character comes from the fixtures rather than from arguments on purpose:
// one world means one answer to "which player has 10 hp", and a harness that
// invented its own characters would be testing numbers no other test shares
// (state.LoadFixtures, T-016).
func (h *Harness) CreatePlayer(ctx context.Context, playerID string) error {
	fixture, err := h.character(playerID)
	if err != nil {
		return err
	}
	proposalID := h.nextProposalID("create", playerID)
	ev := h.root(TypeCreateProposed, fixture, map[string]any{
		"proposal_id": proposalID,
		"entity":      refPayload(fixture),
		"attributes":  fixture.Attributes,
		"cause":       CauseCreate,
	})
	if err := h.publish(ctx, ev); err != nil {
		return err
	}
	if err := h.awaitProposal(ctx, proposalID); err != nil {
		return err
	}
	position, _ := fixture.Position()
	h.mu.Lock()
	h.positions[playerID] = position
	h.mu.Unlock()
	return nil
}

// Enter walks a character into a region: player.entered_region, then the
// proposal that moves the character there (C-04, "сопутствующие предложения").
//
// The proposal is derived from the action rather than published as a root of
// its own, so that the fact of State and the action of the player share one
// correlation identifier and a reader of the journal can see which action
// moved the world.
func (h *Harness) Enter(ctx context.Context, playerID, regionID string) error {
	player, err := h.character(playerID)
	if err != nil {
		return err
	}
	region, err := h.region(regionID)
	if err != nil {
		return err
	}
	from := h.positionOf(playerID)
	action := h.root(TypeEnteredRegion, player, map[string]any{
		"entity":   refPayload(player),
		"action":   map[string]any{"type": "enter"},
		"target":   refPayload(region),
		"position": map[string]any{"from": nullable(from), "to": regionID},
	})
	if err := h.publish(ctx, action); err != nil {
		return err
	}
	return h.move(ctx, action, playerID, regionID)
}

// Leave walks a character out of the region it stands in, back to the world
// outside every region.
func (h *Harness) Leave(ctx context.Context, playerID string) error {
	player, err := h.character(playerID)
	if err != nil {
		return err
	}
	from := h.positionOf(playerID)
	region, err := h.region(from)
	if err != nil {
		return fmt.Errorf("testkit/gateway: leave %s: %w", playerID, err)
	}
	to := Outside(h.worldID)
	action := h.root(TypeLeftRegion, player, map[string]any{
		"entity":   refPayload(player),
		"action":   map[string]any{"type": "leave"},
		"target":   refPayload(region),
		"position": map[string]any{"from": nullable(from), "to": to},
	})
	if err := h.publish(ctx, action); err != nil {
		return err
	}
	return h.move(ctx, action, playerID, to)
}

// Look publishes player.looked. It proposes nothing: looking around changes
// the world in no way State would have to record.
func (h *Harness) Look(ctx context.Context, playerID string) error {
	player, err := h.character(playerID)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"entity": refPayload(player),
		"action": map[string]any{"type": "look"},
	}
	if region, err := h.region(h.positionOf(playerID)); err == nil {
		payload["target"] = refPayload(region)
	}
	return h.publish(ctx, h.root(TypeLooked, player, payload))
}

// Say publishes player.said. The text is content, so the schema bounds it
// (500 characters, no external identifiers) and the harness only refuses what
// it cannot publish at all.
func (h *Harness) Say(ctx context.Context, playerID, text string) error {
	player, err := h.character(playerID)
	if err != nil {
		return err
	}
	if text == "" {
		return errors.New("testkit/gateway: say: empty text")
	}
	return h.publish(ctx, h.root(TypeSaid, player, map[string]any{
		"entity": refPayload(player),
		"action": map[string]any{"type": "say"},
		"text":   text,
	}))
}

// Rest publishes player.rested and the proposal that restores the character to
// its full health (C-02 v1.1, cause=rest).
//
// The proposal pins the version, because hp is one of the four paths ADR-013
// makes the optimistic lock mandatory for. The version comes from the last
// fact the harness saw, which is why every action that proposes waits for its
// own answer before the next one is allowed to start.
func (h *Harness) Rest(ctx context.Context, playerID string) error {
	player, err := h.character(playerID)
	if err != nil {
		return err
	}
	hpMax, ok := player.HPMax()
	if !ok {
		return fmt.Errorf("testkit/gateway: rest: %s has no hp_max", playerID)
	}
	action := h.root(TypeRested, player, map[string]any{
		"entity": refPayload(player),
		"action": map[string]any{"type": "rest"},
	})
	if err := h.publish(ctx, action); err != nil {
		return err
	}
	return h.propose(ctx, action, playerID, CauseRest,
		entity.Op{Op: entity.OpSet, Path: entity.AttrHP, Value: hpMax})
}

// Attack publishes player.attacked: the character swings at an NPC of the
// fixture world, and whoever runs the fight decides what the blow did.
//
// Unlike every other action that moves the world, the harness proposes
// nothing here. The change belongs to the encounter (C-05, T-219), which is
// why Attack waits for that change instead of for one of its own — see
// awaitReaction.
func (h *Harness) Attack(ctx context.Context, playerID, targetID string) error {
	player, err := h.character(playerID)
	if err != nil {
		return err
	}
	target, err := h.opponent(targetID)
	if err != nil {
		return err
	}
	if _, known := h.Version(playerID); !known {
		return fmt.Errorf("testkit/gateway: %s is not in the world of this harness: "+
			"create it before it fights", playerID)
	}
	what := "the attack of " + playerID + " on " + targetID
	if err := h.stillFighting(playerID, what); err != nil {
		return err
	}
	if status, known := h.Status(targetID); known && entity.IsTerminalStatus(status) {
		return fmt.Errorf("testkit/gateway: %s: %s is %s and is nobody's target any more: %w",
			what, targetID, status, ErrFightOver)
	}
	action := h.root(TypeAttacked, player, map[string]any{
		"entity": refPayload(player),
		"action": map[string]any{"type": "attack"},
		"target": refPayload(target),
	})
	return h.resolved(ctx, action, playerID, what)
}

// stillFighting refuses an action of a fight the harness knows is over, so
// that a script which swings after the wolf fell is told what it did instead
// of waiting out the timeout of an action nobody is left to answer.
//
// This is the harness side of the decision of 2026-09-11 on Cr-2 of T-219: an
// action outside an encounter is an error of the script, not an event of the
// world, and the answer to it is an immediate error naming the reason. It says
// nothing when the harness has heard of no fight at all — that is the state
// every action starts in, and refusing there would refuse the first blow of
// every scenario.
func (h *Harness) stillFighting(playerID, what string) error {
	if status, known := h.Status(playerID); known && entity.IsTerminalStatus(status) {
		return fmt.Errorf("testkit/gateway: %s: %s is %s and takes no turn: %w",
			what, playerID, status, ErrFightOver)
	}
	id, ended, known := h.Fight(playerID)
	if known && ended != "" {
		return fmt.Errorf("testkit/gateway: %s: the encounter %s ended (%s), and an action "+
			"outside an encounter is answered by nobody (C-05): %w", what, id, ended, ErrFightOver)
	}
	return nil
}

// Flee publishes player.flee_attempted: the character tries to break off, and
// whoever runs the fight decides whether it got away.
//
// No target travels with it. player.flee_attempted requires the character and
// the action and nothing else, because a flight is resolved against a
// threshold rather than against somebody (schema of the type, C-03 §5.4);
// which encounter is being left is known to the encounter, not to a stub that
// never saw it start.
func (h *Harness) Flee(ctx context.Context, playerID string) error {
	player, err := h.character(playerID)
	if err != nil {
		return err
	}
	if _, known := h.Version(playerID); !known {
		return fmt.Errorf("testkit/gateway: %s is not in the world of this harness: "+
			"create it before it flees", playerID)
	}
	what := "the flight of " + playerID
	if err := h.stillFighting(playerID, what); err != nil {
		return err
	}
	action := h.root(TypeFleeAttempted, player, map[string]any{
		"entity": refPayload(player),
		"action": map[string]any{"type": "flee"},
	})
	return h.resolved(ctx, action, playerID, what)
}

// resolved publishes an action somebody else answers and waits for the answer.
//
// The waiting slot is opened before the action is published: an encounter that
// answered faster than this goroutine got back to the lock would otherwise
// have nobody to answer to.
//
// The fight is checked again under the lock the slot is opened under, although
// stillFighting has just checked it: the lock was let go in between, and a
// start or an end heard in that moment ends the waits it finds — which does not
// yet include this one. Under one lock the two are ordered: either the end came
// first and is seen here, or the slot came first and the end finds it (T-433).
// stillFighting stays for what it says before anything is built.
func (h *Harness) resolved(ctx context.Context, action eventbus.Event,
	playerID, what string) error {
	correlation := action.CorrelationID()
	h.mu.Lock()
	if f := h.fights[playerID]; f != nil && f.ended != "" {
		h.mu.Unlock()
		return fmt.Errorf("testkit/gateway: %s: the encounter %s ended (%s), and an action "+
			"outside an encounter is answered by nobody (C-05): %w", what, f.id, f.ended, ErrFightOver)
	}
	h.awaited[correlation] = &reaction{player: playerID}
	h.mu.Unlock()
	defer h.forget(correlation)

	if err := h.publish(ctx, action); err != nil {
		return err
	}
	return h.awaitReaction(ctx, correlation, what)
}

// awaitReaction blocks until the change the action caused is a fact.
//
// Why the fact and not the decision of the fight. combat.decided is published
// before the change and carries no version: an action that returned there
// would hand the next step of a script a version the world has already left,
// and the refusal that follows would appear in one run out of many. The fact
// is the only point at which the read-model of the harness holds what State
// holds — which is the invariant every other action of the harness keeps, and
// the one the next proposal of a script needs (ADR-013 p. 1).
//
// It rests on one property of whoever runs the fight, and on no other: an
// action that reaches an encounter is answered by exactly one proposal of
// change. That is what T-219 specifies for FakeEncounter, down to the words
// "one entity.update.proposed atomic with expected_version", and what its own
// definition of done demands ("0 actions without an answer"). An encounter
// that breaks it makes every run of the scenario fail here, with the message
// below — never one run in ten.
//
// "Reaches an encounter" is the whole of the promise. An action the encounter
// never takes — struck after the fight closed — is not waited for at all: the
// harness knows the fight is over from world_events and says so at once
// (stillFighting), and an encounter that closes while an action is still in
// flight ends that wait with ErrFightOver rather than with the deadline.
//
// What ends this wait, then, is: a fact for every entity the latest offer of
// the package named; a refusal that is a defect of whoever proposed it —
// unknown_entity, invalid_op, dead_entity, duplicate_entity — reported at once,
// because a defect is worth seeing the moment it happens; the end of the fight;
// and the deadline. A refusal by version conflict is none of these: it is the
// race an optimistic lock exists for, the publisher owes it a bounded number of
// retries, and a wait that ended there would report a package that arrives a
// millisecond later as a package that never arrived. A publisher that runs out
// of retries is caught by the deadline instead, and progress says so in as many
// words — which is why the deadline is left exactly where it was and not
// extended by a conflict: it is the only thing standing between a stuck
// publisher and a wait with no end.
func (h *Harness) awaitReaction(ctx context.Context, correlation, what string) error {
	timeout := h.timeout
	deadline := clock.RealTimers{}.After(timeout)
	defer deadline.Stop()
	for {
		h.mu.Lock()
		r := h.awaited[correlation]
		done, refusal, over, proposalID := r.done(), r.refusal, r.over, r.proposalID
		wake := h.changed
		h.mu.Unlock()
		if done {
			switch {
			case over != "":
				return fmt.Errorf("testkit/gateway: %s: the encounter ended (%s) while it was "+
					"waiting, and nothing was proposed for it: %w", what, over, ErrFightOver)
			case refusal != "":
				return fmt.Errorf("testkit/gateway: %s: the change proposed for it (%s) "+
					"was refused: %s", what, proposalID, refusal)
			}
			return nil
		}
		select {
		case <-wake:
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C():
			h.mu.Lock()
			progress := h.awaited[correlation].progress()
			h.mu.Unlock()
			return fmt.Errorf("testkit/gateway: nobody resolved %s within %s: %s "+
				"(contracts.md C-05, testkit/swarm.FakeEncounter)", what, timeout, progress)
		}
	}
}

// forget drops the waiting slot of an action that has been answered, so that a
// scenario of thirty turns does not carry thirty answers around.
func (h *Harness) forget(correlation string) {
	h.mu.Lock()
	delete(h.awaited, correlation)
	h.mu.Unlock()
}

// move is the proposal behind Enter and Leave: the position of the character,
// in one atomic package.
//
// The scope is not in that package, although design.md §5 names it beside the
// position: the scope of a solo character is {id: player-X, type: solo} and
// entering a region does not change it, so proposing it would be a change set
// that changes nothing — an entity.updated with an empty changed and the same
// version. Whether a group scope has to travel with the position is open
// question 2 of T-018 and a line for C-04, not for a stub to draw.
func (h *Harness) move(ctx context.Context, cause eventbus.Event, playerID, to string) error {
	if err := h.propose(ctx, cause, playerID, CauseMove,
		entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: to}); err != nil {
		return err
	}
	h.mu.Lock()
	h.positions[playerID] = to
	h.mu.Unlock()
	return nil
}

// propose publishes one entity.update.proposed caused by an action and waits
// for the answer of State.
func (h *Harness) propose(ctx context.Context, cause eventbus.Event, playerID, why string,
	ops ...entity.Op) error {
	player, err := h.character(playerID)
	if err != nil {
		return err
	}
	version, known := h.Version(playerID)
	if !known {
		return fmt.Errorf("testkit/gateway: %s is not in the world of this harness: "+
			"create it before it acts", playerID)
	}
	proposalID := h.nextProposalID(why, playerID)
	set := entity.ChangeSet{
		Entity:          eventbus.Entity{Entity: player.Ref().EventRef(), Name: player.Name},
		ExpectedVersion: &version,
		Ops:             ops,
	}
	ev := eventbus.Derive(cause, TypeUpdateProposed, Source, map[string]any{
		"proposal_id": proposalID,
		"changes":     []entity.ChangeSet{set},
		"atomic":      true,
		"cause":       why,
	})
	if err := h.publish(ctx, ev); err != nil {
		return err
	}
	return h.awaitProposal(ctx, proposalID)
}

// awaitProposal blocks until State has answered, and reports a refusal as an
// error: a harness whose proposal was refused has left the world somewhere its
// script did not mean it to be, and going on would test a scenario nobody
// wrote.
func (h *Harness) awaitProposal(ctx context.Context, proposalID string) error {
	timeout := h.timeout
	deadline := clock.RealTimers{}.After(timeout)
	defer deadline.Stop()
	for {
		h.mu.Lock()
		refusal, answered := h.settled[proposalID]
		wake := h.changed
		h.mu.Unlock()
		if answered {
			if refusal != "" {
				return fmt.Errorf("testkit/gateway: proposal %s refused: %s", proposalID, refusal)
			}
			return nil
		}
		select {
		case <-wake:
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C():
			return fmt.Errorf("testkit/gateway: no answer to proposal %s within %s",
				proposalID, timeout)
		}
	}
}

// publish sends the event and says what it was in the log.
func (h *Harness) publish(ctx context.Context, ev eventbus.Event) error {
	if err := h.bus.Publish(ctx, ev); err != nil {
		return fmt.Errorf("testkit/gateway: publish %s: %w", ev.Type, err)
	}
	h.log.Info("published", "type", ev.Type, "event_id", ev.ID)
	return nil
}

// root builds a root event of the harness: correlation_id is its own
// identifier (C-04), the actor is CI, the world is the fixture world and the
// scope is the one the character carries.
//
// meta.agent is never set: player_events forbid it, because no agent is behind
// a player action (policy of the topic, C-01).
func (h *Harness) root(typ string, actor *entity.Entity, payload map[string]any) eventbus.Event {
	var scope *eventbus.ScopeRef
	if s, ok := actor.Scope(); ok {
		scope = &s
	}
	return eventbus.NewRoot(typ, Source, h.worldID, scope, entity.ActorKindCI, payload)
}

// nextProposalID names a proposal after what it is for, so that a failing test
// reads as "the rest of player-A was refused" rather than as a bare number.
func (h *Harness) nextProposalID(why, playerID string) string {
	h.mu.Lock()
	h.seq++
	n := h.seq
	h.mu.Unlock()
	return "gw-" + why + "-" + playerID + "-" + strconv.Itoa(n)
}

// character is the fixture player with this identifier.
func (h *Harness) character(id string) (*entity.Entity, error) {
	e, ok := h.fixtures[id]
	if !ok {
		return nil, fmt.Errorf("testkit/gateway: %q is not in the fixtures", id)
	}
	if e.Type != entity.TypePlayer {
		return nil, fmt.Errorf("testkit/gateway: %q is a %s, not a player", id, e.Type)
	}
	return e, nil
}

// region is the fixture region with this identifier.
func (h *Harness) region(id string) (*entity.Entity, error) {
	e, ok := h.fixtures[id]
	if !ok {
		return nil, fmt.Errorf("testkit/gateway: %q is not a region of the fixtures", id)
	}
	if e.Type != entity.TypeRegion {
		return nil, fmt.Errorf("testkit/gateway: %q is a %s, not a region", id, e.Type)
	}
	return e, nil
}

// opponent is the fixture NPC with this identifier.
//
// Only an NPC: MVP-1 has no player against player (C-04 lists no such action,
// and C-03 excludes a player from the targets of a strike), so a script that
// swings at another character is a script that will never be answered, and
// saying so here costs a line instead of a timeout.
//
// There is deliberately no check that the harness has heard of the target, the
// way Attack requires it of the character: the harness keeps versions for what
// it proposes changes to, and it proposes nothing about an NPC — the fixtures
// are all it knows about the wolf, and all it needs to name it in an action.
// What it does check is what the facts told it: a target it has seen die is
// refused by Attack itself.
func (h *Harness) opponent(id string) (*entity.Entity, error) {
	e, ok := h.fixtures[id]
	if !ok {
		return nil, fmt.Errorf("testkit/gateway: %q is not in the fixtures", id)
	}
	if e.Type != entity.TypeNPC {
		return nil, fmt.Errorf("testkit/gateway: %q is a %s, not an npc: "+
			"MVP-1 knows no fight between characters", id, e.Type)
	}
	return e, nil
}

func (h *Harness) positionOf(playerID string) string {
	if p, ok := h.Position(playerID); ok {
		return p
	}
	if fixture, ok := h.fixtures[playerID]; ok {
		if p, ok := fixture.Position(); ok {
			return p
		}
	}
	return Outside(h.worldID)
}

// refPayload is the EntityWithName shape every payload of C-04 refers to
// another entity by.
func refPayload(e *entity.Entity) map[string]any {
	out := map[string]any{"entity": map[string]any{"id": e.ID, "type": e.Type}}
	if e.Name != "" {
		out["name"] = e.Name
	}
	return out
}

// nullable turns an unknown origin into the JSON null the position block
// allows, instead of into an empty string the schema would refuse.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
