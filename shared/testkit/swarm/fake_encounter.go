package swarm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	// The double of the encounter agent has to speak the types of C-03, exactly
	// as the double of the mechanics does: Action, Actor, Outcome and Roll are
	// the contract, and a stub that renamed them could not be swapped for the
	// real agent of EPIC-003 C4 without touching everything around it
	// (contracts.md §17, .golangci.yml rule shared-testkit-mechanics). The
	// boundary of ADR-001 p. 3 stays what it was — the exception is one import
	// in one directory, and it is granted by the named depguard rule
	// shared-testkit-swarm of .golangci.yml, which lets this package see
	// internal/mechanics and no other context. Do not add a //nolint here: the
	// rule is the whole permission, and a directive would only hide it.
	mech "multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// EncounterGroup is the consumer group prefix FakeEncounter reads under. Each
// topic gets its own group name derived from it, because a consumer group is a
// cursor over one topic (C-01).
const EncounterGroup = "testkit-encounter"

// The types FakeEncounter answers and publishes, next to the ones the narrator
// already names (TypeEnteredRegion, TypeCreated, TypeUpdated).
const (
	TypeAttacked      = "player.attacked"
	TypeFleeAttempted = "player.flee_attempted"
	TypeRested        = "player.rested"

	TypeEncounterStarted = "encounter.started"
	TypeEncounterEnded   = "encounter.ended"
	TypeCombatDecided    = "combat.decided"
	TypeDiceRolled       = "dice.rolled"

	TypeCreateProposed = "entity.create.proposed"
	TypeUpdateProposed = "entity.update.proposed"
	TypeRejected       = "entity.update.rejected"
)

// EncounterAgent is the swarm agent the fight is attributed to. The blueprint
// carries the fake- prefix contracts.md §0 requires of every stub, over the
// name of the blueprint it stands in for (blueprints/encounter-wolf.md, T-203).
// The identifier is per player and is built by encounterAgent.
//
// The two levels are the two roles the stub plays. Opening an encounter belongs
// to the region GM: §4.6 gives create of an encounter with cause=spawn to the
// domain level, and contracts.md §0 names region-gm as the publisher of
// encounter.started. Everything the fight does afterwards belongs to the
// encounter agent, which is a task.
const (
	EncounterAgentPrefix = "fake-encounter"
	EncounterBlueprint   = "fake-encounter-wolf"
	AgentLevelTask       = "task"
	AgentLevelDomain     = "domain"
)

// Phase1Mode is what combat.decided reports about how the fight was decided:
// by the rules, not by a model. The stub never calls one (design §4.1 p. 3).
const Phase1Mode = "rules"

// The causes of the proposals the stub publishes (C-02, ownership table
// state-and-mechanics.md §4.6).
const (
	CauseSpawn  = "spawn"
	CauseCombat = "combat"
	CauseFlee   = "flee"
	// CauseResolve is the cause of the package that closes a fight whose last
	// NPC fell to somebody else's hand (C-05 v1.4 p. 6). The package resolves
	// the encounter and nothing else: the death of the NPC is already recorded
	// by its own fact, and nobody of this fight struck (decision of the
	// orchestrator on T-419). The ownership row of the encounter at the task
	// level lists it.
	CauseResolve = "resolve"
)

// The reasons of entity.update.rejected (C-02, §4.5). Only one of the five is
// a race: version_conflict says the world moved between the moment the stub
// read it and the moment State was asked to change it, which is what an
// optimistic lock is there to catch. unknown_entity, invalid_op, dead_entity
// and duplicate_entity say the stub asked for something that was never going
// to work, and those are defects to be seen rather than races to be retried.
//
// They are spelled out here rather than imported from testkit/state, exactly
// as the event types above are: the stub speaks the contract, not the
// implementation of the double that happens to answer it today.
const ReasonVersionConflict = "version_conflict"

// maxProposalAttempts is how many times one action offers its package before
// the stub stops. The package is offered once and then re-offered on a version
// conflict, so two retries follow the first attempt.
//
// The bound is what makes the retry a recovery and not a livelock: an agent
// that lost the race twice in a row against a world nobody else is writing is
// not racing any more, and the honest end of that is a loud line in the log
// (see refused) rather than a loop nobody can see.
const maxProposalAttempts = 3

// proposalPrefix is what the stub names its proposals by, so that a refusal on
// the bus can be told from a refusal of somebody else's package.
const proposalPrefix = "prop-"

// The roll indices of one turn. They are fixed rather than counted, so that a
// replay of the same action event rolls the same dice: an index is half of the
// seed (C-03, mechanics.Seed).
//
// The index of the damage roll is spent whether or not the swing landed, which
// is what keeps the answer of the NPC addressed the same way in a round that
// began with a miss.
const (
	rollAttack     = 0 // hit, damage
	rollNPCCounter = 2 // npc_hit, npc_damage
	rollFlee       = 0 // flee
	rollFreeAttack = 1 // npc_hit, npc_damage of the free attack
)

// Mechanics is the one decision FakeEncounter does not make itself: what
// happened when somebody swung.
//
// The interface is declared here, on the side of the consumer, so that both
// halves of C-03 satisfy it — *mechanics.Rules once EPIC-002 implements Resolve
// (T-053) and testkit/mechanics.FixedMechanics until then — and swapping one
// for the other is a change of one field of EncounterConfig (design.md §11,
// contracts.md §17).
type Mechanics interface {
	Resolve(causeEventID string, rollIndexStart int, a mech.Action,
		actors map[string]*mech.Actor) (mech.Outcome, []mech.Roll, error)
}

// EncounterConfig builds a FakeEncounter. Bus, WorldID, Rules and Mechanics are
// mandatory: a fight without a rule book has no numbers, and a fight without a
// decision has no outcome.
type EncounterConfig struct {
	// Bus is where the player actions are read from and everything the fight
	// produces is published to.
	Bus eventbus.Bus
	// WorldID is the world the stub serves. An action of another world is
	// passed over.
	WorldID string
	// Rules is the rule book itself — the numbers, the round, the loot table
	// and rules_version. Nothing here is faked: it is rules/dark-forest.yaml as
	// internal/mechanics loaded it.
	Rules *mech.Rules
	// Mechanics decides one action. FixedMechanics answers from a table until
	// EPIC-002 writes Resolve (T-053).
	Mechanics Mechanics
	// Log receives what the stub decided; it defaults to a logger that
	// discards everything, so that a test says nothing unless it asks to.
	Log *slog.Logger
	// DedupCapacity is how many identifiers are remembered against an
	// at-least-once redelivery — of the actions answered, and separately of the
	// refusals answered; zero selects the default of eventbus.Dedup.
	DedupCapacity int
}

// FakeEncounter is the stand-in for Phase 1 of a fight: it turns the actions of
// a solo player into the encounter, the dice, the decisions and the proposals
// the encounter agent of EPIC-003 will publish (contracts.md C-05, design.md
// §4.1, tasks.md T-219).
//
// It is a temporary stub of I1-α and carries an expiry date: the e2e switches
// to the real swarm in T-242 and the hook that mounts it in core goes away in
// T-256 (ADR-001 addendum p. 8). See README.md of this package.
//
// It is a stub about who decides, never about what a consumer sees. Everything
// it publishes is a real event of the registry with source testkit/swarm — one
// of the publishers contracts.md §0 lists for these types — carries meta.agent
// as the swarm policy demands, and is validated by the bus against the same
// schema the encounter agent will be validated against.
//
// It answers one action with one package, and it holds that promise across a
// lost race: a package refused for a version conflict is worked out again
// against the world the fact of the conflict describes and offered under the
// same identifier, a bounded number of times (refused). That is not a second
// answer to the action — the dice are rolled once and the decision is
// published once — and it is what the encounter agent of EPIC-003 will have to
// do, which is why the stub shows the shape rather than avoiding the problem.
// The retry ends with the first fact that names its proposal, and a retry that
// would change who is left standing is not offered at all (propose).
//
// What it deliberately does not do, and who does it instead:
//
//   - It does not read MV_SWARM_FAKE and knows nothing of cmd/**. The flag is
//     read by the hook of T-255, because at I1-α there is no internal/swarm in
//     integration/mvp-1 for a context to read it in, and because a package that
//     reached for the flag could not be compiled into the production binary at
//     all (ADR-001 addendum p. 8).
//   - It opens an encounter on entry rather than on a tick of the region. The
//     tick, the chance roll and the domain GM that owns them are I1b (design
//     §4.1, risk 12).
//   - It never calls a model: no LLM, no filter, no guardian.
//   - It does not answer player.rested. Outside an encounter the rest is
//     proposed by the gateway (C-02 v1.1), and inside one State refuses it.
//   - It says nothing at all about an action it cannot take: an attack after
//     the fight ended, a swing with nothing left to hit, an entry naming no
//     region, an action of a world it does not serve. The silence is
//     deliberate and stays. The decision of 2026-09-11 calls those four errors
//     of the scenario rather than events of the world, and gives the immediate
//     refusal to the harness of shared/testkit/gateway, which already knows the
//     state of the fight from encounter.started and encounter.ended. Publishing
//     a refusal from here would need a new event type for a stub two epics from
//     removal — which is what T-018 turned down. Do not "fix" this by
//     answering: it would put the harness and the stub in disagreement about
//     whose error a bad scenario is.
type FakeEncounter struct {
	bus     eventbus.Bus
	worldID string
	rules   *mech.Rules
	mech    Mechanics
	log     *slog.Logger

	mu    sync.Mutex
	world map[string]*entity.Entity
	// facts is the same world as the facts of State describe it, without the
	// changes the stub folded in before they were answered for (apply). It is
	// kept because a package State refuses moved nothing at all: the view a
	// retry is computed from is this one, down to the version, and it holds
	// what arrived while the refused package was on its way.
	facts map[string]*entity.Entity
	// byPlayer is the encounter a character is standing in. One player, one
	// encounter: the group fight of C-04 is the coordinator of the gateway and
	// is not part of Phase 1 of the stub.
	byPlayer map[string]*encounter
	acted    *eventbus.Dedup
	// refusals is the same guard as acted, over entity.update.rejected: a
	// refusal delivered twice is one lost race, and answering the copy would
	// spend a second attempt on it — or, once the retry has gone in, offer a
	// package State has already applied (review #2 of T-219, Ma-1). It is a
	// window of its own so that a burst of refusals cannot push the actions out
	// of theirs.
	refusals *eventbus.Dedup
	started  bool

	subscriptions sync.WaitGroup
	subErr        error
}

// encounter is one fight in progress, as the stub keeps it. It is not the
// encounter entity: the entity lives in State and is what a read-model sees,
// while this is what the stub needs to decide the next action without waiting
// for a fact to come back.
type encounter struct {
	id     string
	region string

	players []*participant
	npcs    []string
	// lastDamager is who hit an NPC last — the first candidate of the real
	// NPCTarget (C-03), carried so that the encounter the stub hands to
	// ActorFromEntity says the same thing the entity would.
	lastDamager map[string]string

	round int
	// scope is the scope the encounter was opened for — the scope of whoever
	// walked in (C-04). An end the encounter agent reaches on somebody else's
	// fact carries it explicitly, because the scope that fact brings along is
	// not the scope of the fight (C-05 v1.4 p. 6).
	scope eventbus.ScopeRef

	// The lifecycle of the fight is announced only after State has accepted
	// what the announcement says (C-05 v1.4 p. 4–5, ADR-026):
	//
	//   opening — the creation is proposed and entity.created has not come
	//             back; encounter.started is built and held in opening.
	//   active  — State created the encounter and encounter.started is out.
	//   closing — the package that resolves the encounter is on its way, and
	//             encounter.ended is built and held in closing. The fight is
	//             still active: State holds it open until the fact.
	//   over    — the fact of the closing package came back, encounter.ended
	//             is out and active is false.
	//
	// A creation State refuses ends the fight before it began: it leaves
	// byPlayer, and nothing about it is ever announced.
	opening *eventbus.Event
	// openedBy is the proposal that creates the encounter entity — what a
	// refusal of the creation names.
	openedBy string
	active   bool
	closing  *closure
	// deferred are the actions of the character that arrived while the
	// creation or the closing package was on its way. They are answered in the
	// order they came once State has said whether it went in: an answer worked
	// out from a view holding a package State has not applied would be silence
	// in a fight that goes on, or a blow in one that is over (C-05 v1.4 p. 5).
	// Only a package in flight holds them, so the list lives as long as one
	// turn of State.
	deferred []eventbus.Event
	// downed is the fact that took an NPC of the fight off its feet — the
	// cause the encounter agent closes the fight on when it was not this fight
	// that did it (C-05 v1.4 p. 6). orphaned is the fact a closure was already
	// offered on, so that a closure State keeps turning down is not offered
	// again and again through the bus.
	downed   *eventbus.Event
	orphaned string
	// pending is the package of the action being answered right now, kept
	// until State says whether it went in: a fact naming its proposal settles
	// it (Observe), and so does the stub giving up on it (refused, propose).
	// One fight answers one action at a time, so one slot is all a fight ever
	// needs and nothing here can grow.
	pending *answer
	// touched says whether the action being answered right now moved
	// participation. A package must not carry a participants[] identical to the
	// one State already holds: State would apply it, find nothing different and
	// publish a fact with an empty changed[] (§4.6).
	touched bool
}

// participant is how one character takes part in the fight, as participants[]
// of the encounter entity carries it (data-model.md §3.7).
//
// The stub keeps the record here rather than reading it back from State,
// because the package of the next action is built before the fact of the
// previous one comes back — the same reason apply folds a proposal into the
// view of the stub.
type participant struct {
	id          string
	state       string
	damageDealt int
	lastHitAt   time.Time
}

func (p *participant) payload() map[string]any {
	out := map[string]any{
		"player_id":    p.id,
		"state":        p.state,
		"damage_dealt": p.damageDealt,
	}
	if !p.lastHitAt.IsZero() {
		out["last_hit_at"] = p.lastHitAt.UTC().Format(time.RFC3339)
	}
	return out
}

// answer is what one action decided, kept until State says whether the package
// it became went in.
//
// It holds decisions rather than operations on purpose. A package refused for
// a version conflict is offered again, and the second offer has to be computed
// against the world as it is by then: the same swing takes the same damage off
// whatever hit points the target turns out to have. A record of the operations
// alone could only be re-sent under a new version, which would write hit
// points the world has already left (ADR-013 p. 1, §4.5).
//
// What is decided once and never recomputed is everything that does not read
// the world: the round the turn consumed, how the fight ended, where a
// character who got away now stands. Those are the decisions of the action
// itself, and a retry that changed them would be answering a different action.
// That is also why a retry that would change who is left standing is not
// offered at all (propose): the end of the fight rests on who fell, and it has
// been decided and published already.
type answer struct {
	// cause is the player action being answered. It names the proposal, and
	// every event of the package is derived from it.
	cause     eventbus.Event
	causeName string
	// blows are the strikes that landed, in the order they landed. A miss
	// leaves nothing here: it wounds nobody and costs only the round.
	blows []blow
	// fleeTo is where a character who got away now stands, and fleer is who
	// they are. Both are empty when nobody fled or the flight failed.
	fleeTo any
	fleer  string
	// encOps is what the turn wrote on the encounter entity.
	encOps []entity.Op
	// attempts is how many times the package has been offered, and undo is how
	// the view of the stub is put back if the last offer was refused.
	attempts int
	undo     []restore
	// before is participation as it stood before the action. A package State
	// never applied gives it back, exactly as it gives back hit points: the
	// damage dealt and the last damager live only in the package and in the
	// encounter entity (C-05 v1.4 p. 1г).
	before participation
	// scope, when set, is the scope the package is published under instead of
	// the scope of its cause.
	scope *eventbus.ScopeRef
}

// options are the options every event of the package is derived with beyond
// the agent.
func (a *answer) options() []eventbus.DeriveOption {
	if a.scope == nil {
		return nil
	}
	return []eventbus.DeriveOption{eventbus.WithScope(a.scope)}
}

// closure is the end of a fight while the package that resolves it is on its
// way: the encounter.ended built for it, which the package names in
// closed_by_event_id, and the proposal whose fact releases it (C-05 v1.4 p. 5).
type closure struct {
	ended      eventbus.Event
	proposalID string
	reason     string
}

// participation is the part of a fight a package carries for its fighters and
// a refused package gives back: each character's record in participants[] and
// the last damager of each NPC.
type participation struct {
	players     []participant
	lastDamager map[string]string
}

// exchangeStep is where a decision stands in the exchange that answers one
// action (combat.decided.exchange, C-05 v1.4 p. 7).
type exchangeStep struct {
	index int
	last  bool
}

// blow is a strike that landed: the dice are rolled, the decision is published
// and what is left is to take the damage off whoever was struck.
type blow struct {
	targetID   string
	attackerID string
	damage     int
	// decisionID is the combat.decided this blow came from — what the trophy
	// of a killing blow names as its source (inv-03).
	decisionID string
	// kills is what the decision said about the target: that it fell. It is
	// published with the decision and the end of the fight follows from it, so
	// a package that says otherwise is a package that contradicts both.
	kills bool
	at    time.Time
}

// restore is what one entity looked like before the stub folded a package into
// its own view, and what it is put back to when State refuses that package.
//
// The attributes are the map itself and not a copy of it: entity.ApplyOps
// computes into a fresh map and never writes into the one it was given, so the
// map the entity carried before is still exactly what it was.
type restore struct {
	ent        *entity.Entity
	attributes map[string]any
	version    int64
}

// proposalID is the identifier the package of this action is offered under. It
// does not change between attempts, because a retry is the same answer to the
// same action offered again: State applies a proposal identifier once (§4.5),
// so a duplicate that crosses a successful attempt on the wire is dropped
// there rather than applied twice.
func (a *answer) proposalID() string { return proposalPrefix + a.cause.ID }

// NewFakeEncounter builds the stub. It does not touch the bus: nothing is
// published and nothing is read until Start.
func NewFakeEncounter(cfg EncounterConfig) (*FakeEncounter, error) {
	switch {
	case cfg.Bus == nil:
		return nil, errors.New("testkit/swarm: no bus")
	case cfg.WorldID == "":
		return nil, errors.New("testkit/swarm: no world")
	case cfg.Rules == nil:
		return nil, errors.New("testkit/swarm: no rules")
	case cfg.Mechanics == nil:
		return nil, errors.New("testkit/swarm: no mechanics")
	}
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &FakeEncounter{
		bus:      cfg.Bus,
		worldID:  cfg.WorldID,
		rules:    cfg.Rules,
		mech:     cfg.Mechanics,
		log:      log.With("component", Source, "world_id", cfg.WorldID),
		world:    make(map[string]*entity.Entity),
		facts:    make(map[string]*entity.Entity),
		byPlayer: make(map[string]*encounter),
		acted:    eventbus.NewDedup(cfg.DedupCapacity),
		refusals: eventbus.NewDedup(cfg.DedupCapacity),
	}, nil
}

// Seed puts entities into the world the stub sees, without a proposal and
// without a fact: the fixtures of testdata/fixtures, or a world assembled by
// hand. It is how the stub knows there is a living wolf in the region before
// anybody has changed anything (design §4.1: "a living NPC of the fixtures").
//
// Everything else it learns from the facts of State (Observe). The entities are
// copied, so a caller that goes on using the fixtures cannot change the world
// behind the back of the stub.
func (e *FakeEncounter) Seed(entities []*entity.Entity) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ent := range entities {
		if ent == nil || ent.ID == "" {
			return errors.New("testkit/swarm: seed: entity without an id")
		}
		e.world[ent.ID] = entity.Clone(ent)
		// A seeded entity is the world before anybody changed it, which is
		// exactly what State was seeded with: the first fact about it.
		e.facts[ent.ID] = entity.Clone(ent)
	}
	return nil
}

// Start opens the two subscriptions the stub needs: the player actions it
// answers, and the facts of State it keeps its view of the world by.
//
// Nothing is announced at start. The stub is not a stateful context of C-14
// whose recovery consumers wait for; what it knows it learns from Seed and from
// the journal.
func (e *FakeEncounter) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return errors.New("testkit/swarm: already started")
	}
	e.started = true
	e.mu.Unlock()

	e.subscribe(ctx, eventbus.TopicPlayerEvents, e.Act)
	e.subscribe(ctx, eventbus.TopicSystemEvents, e.Observe)
	return nil
}

func (e *FakeEncounter) subscribe(ctx context.Context, topic string, h eventbus.Handler) {
	e.subscriptions.Add(1)
	go func() {
		defer e.subscriptions.Done()
		if err := e.bus.Subscribe(ctx, topic, EncounterGroup+"-"+topic, h); err != nil {
			e.mu.Lock()
			if e.subErr == nil {
				e.subErr = fmt.Errorf("testkit/swarm: subscribe %s: %w", topic, err)
			}
			e.mu.Unlock()
			e.log.Error("subscription stopped", "topic", topic, "err", err)
		}
	}()
}

// Wait blocks until every subscription started by Start has stopped and reports
// the first failure among them. A subscription cancelled through its context is
// not a failure and reports nil (C-01).
func (e *FakeEncounter) Wait() error {
	e.subscriptions.Wait()
	return e.Err()
}

// Err is the first failure of a subscription, or nil while they are healthy.
func (e *FakeEncounter) Err() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.subErr
}

// ActiveEncounter is the identifier of the fight a character is standing in.
func (e *FakeEncounter) ActiveEncounter(playerID string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	enc, ok := e.byPlayer[playerID]
	if !ok || !enc.active {
		return "", false
	}
	return enc.id, true
}

// ActiveCount is how many fights are in progress — what the health of the
// context reports.
func (e *FakeEncounter) ActiveCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, enc := range e.byPlayer {
		if enc.active {
			n++
		}
	}
	return n
}

// Act answers one player action. It is the handler Start subscribes
// player_events with, and it is exported so that a test can drive the stub
// without a goroutine.
//
// An action of another world is passed over: an encounter agent serves one
// world, and a fight in a world it does not serve would reach the players of
// that world as if it were theirs.
//
// The error it returns is a failure of the stub — a rule that cannot be read, a
// publication the schema refuses — and makes the bus retry and then park the
// action in dead_letters. An action there is nothing to do about (a rest, a
// look, an attack outside an encounter) is not a failure and answers nil.
func (e *FakeEncounter) Act(ctx context.Context, ev eventbus.Event) error {
	switch ev.Type {
	case TypeEnteredRegion, TypeAttacked, TypeFleeAttempted:
	default:
		// player.rested among them: outside an encounter the rest belongs to
		// the gateway (C-02 v1.1), inside one State refuses it.
		return nil
	}
	// An envelope without an id has no cause to derive the fight from: the
	// lifecycle takes its ids from the action (WithCauseID), which panics on
	// it, and a panic in a handler takes down the process with every context
	// in it (shared/eventbus/README.md, "Id из причины"). Validation on read
	// keeps such an envelope from getting here; with it off, the action is a
	// defect of whoever published it. It is logged as an error rather than a
	// warning because nothing about it is an ordinary turn of the world, and
	// whoever waits on the action will see nothing but its deadline.
	if ev.ID == "" {
		e.log.Error("an action without an id is not answered: it has no cause to derive the fight from",
			"type", ev.Type)
		return nil
	}
	if world := eventbus.GetWorldIDFromEvent(ev); world != "" && world != e.worldID {
		e.log.Debug("action of another world passed over",
			"event_id", ev.ID, "type", ev.Type, "event_world_id", world)
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	// Delivery is at-least-once (C-01): a redelivered attack must not roll a
	// second set of dice and must not hand out a second trophy.
	if e.acted.Seen(ev.ID) {
		e.log.Debug("action already answered", "event_id", ev.ID, "type", ev.Type)
		return nil
	}

	return e.route(ctx, ev)
}

// route answers an action, or holds it back while the creation or the closing
// package of the character's fight is on its way (encounter.deferred). It is
// also how a held action is answered once State has spoken. The caller holds
// the lock.
func (e *FakeEncounter) route(ctx context.Context, ev eventbus.Event) error {
	var p actionPayloadOf
	if err := decodePayload(ev.Payload, &p); err != nil {
		e.log.Warn("an action the stub cannot read", "event_id", ev.ID, "err", err)
		return nil
	}
	if p.Entity != nil {
		if enc := e.byPlayer[p.Entity.Entity.ID]; enc != nil && enc.inFlight() {
			enc.deferred = append(enc.deferred, ev)
			e.log.Debug("action deferred until State answers the package of the fight",
				"event_id", ev.ID, "type", ev.Type, "encounter_id", enc.id)
			return nil
		}
	}
	switch ev.Type {
	case TypeEnteredRegion:
		return e.entered(ctx, ev, p)
	case TypeAttacked:
		return e.attacked(ctx, ev, p)
	default:
		return e.fled(ctx, ev, p)
	}
}

// actionPayloadOf is the part of a player action the stub reads. The lists of
// §2.3.1 carry more than this, and an unknown field is ignored rather than
// refused: a payload richer than what a consumer reads is what a versioned
// schema is for.
type actionPayloadOf struct {
	Entity   *namedRef `json:"entity"`
	Target   *namedRef `json:"target"`
	Position *struct {
		To string `json:"to"`
	} `json:"position"`
}

// entered opens an encounter when a character walks into a region where
// something is alive and no fight of theirs is in progress (design §4.1).
func (e *FakeEncounter) entered(ctx context.Context, ev eventbus.Event, p actionPayloadOf) error {
	if p.Entity == nil || p.Entity.Entity.ID == "" {
		e.log.Warn("event names no acting entity", "event_id", ev.ID, "type", ev.Type)
		return nil
	}
	playerID := p.Entity.Entity.ID
	if enc, busy := e.byPlayer[playerID]; busy && enc.open() {
		e.log.Debug("already in an encounter", "player_id", playerID, "encounter_id", enc.id)
		return nil
	}

	region := ""
	if p.Target != nil && p.Target.Entity.Type == entity.TypeRegion {
		region = p.Target.Entity.ID
	}
	if region == "" && p.Position != nil {
		region = p.Position.To
	}
	if region == "" {
		e.log.Warn("entry names no region", "event_id", ev.ID, "player_id", playerID)
		return nil
	}
	npcID, found := e.freeNPCOf(region, playerID)
	if !found {
		return nil
	}

	// The scope of the encounter is the scope of whoever walked in; a solo
	// player is their own scope (C-04). The fallback is for a caller driving
	// Act by hand with an envelope that carries none.
	scope := eventbus.ScopeRef{ID: playerID, Type: "solo"}
	if s := eventbus.GetScopeFromEvent(ev); s != nil && s.ID != "" {
		scope = *s
	}
	enc := &encounter{
		id:          "encounter-" + ev.ID,
		region:      region,
		players:     []*participant{{id: playerID, state: entity.ParticipationActive}},
		npcs:        []string{npcID},
		lastDamager: make(map[string]string),
		round:       1,
		scope:       scope,
		openedBy:    proposalPrefix + ev.ID,
	}

	// Opening the encounter is the one act of the stub that belongs to the
	// region GM rather than to the encounter agent: §4.6 gives create of an
	// encounter with cause=spawn to the domain level, and contracts.md §0 names
	// region-gm as the publisher of encounter.started.
	opener := encounterAgent(playerID, AgentLevelDomain)

	// The encounter.started is built before the proposal is published so that
	// the entity can name the event that opened it, and its identifier is
	// derived from its cause — the entry — and the encounter (C-01 v1.4,
	// ADR-027 p. 2). The same entry gives the same id on every build: after a
	// restart the event can be published again under the id opened_by_event_id
	// names, and a copy that already went out is dropped by id (C-14 v1.2 (b)).
	// It could not be derived from entity.created: the entity names it before
	// that fact exists.
	round := e.rules.Document().Round
	started := eventbus.Derive(ev, TypeEncounterStarted, Source, map[string]any{
		"encounter":    refPayload(enc.id, entity.TypeEncounter, ""),
		"region":       e.refOf(region, entity.TypeRegion),
		"participants": []map[string]any{e.refOf(playerID, entity.TypePlayer)},
		"npcs":         []map[string]any{e.refOf(npcID, entity.TypeNPC)},
		"round": map[string]any{
			"timeout":           round.Timeout,
			"idle_after_missed": round.IdleAfterMissed,
		},
	}, eventbus.WithAgent(opener), eventbus.WithCauseID(enc.id))

	attributes := map[string]any{
		entity.AttrRegionID:        region,
		entity.AttrScope:           map[string]any{"id": scope.ID, "type": scope.Type},
		entity.AttrState:           entity.EncounterStateActive,
		entity.AttrRoundSeq:        enc.round,
		entity.AttrTaskAgentID:     encounterAgent(playerID, AgentLevelTask).ID,
		entity.AttrOpenedByEventID: started.ID,
		entity.AttrParticipants:    enc.participantsPayload(),
		entity.AttrNPCs:            []map[string]any{{"npc_id": npcID}},
	}
	create := map[string]any{
		"proposal_id": enc.openedBy,
		"entity":      refPayload(enc.id, entity.TypeEncounter, ""),
		"attributes":  attributes,
		"cause":       CauseSpawn,
	}
	if _, err := e.publish(ctx, ev, TypeCreateProposed, create, opener); err != nil {
		return err
	}
	// encounter.started waits for entity.created (announce): the encounter is
	// announced once it exists, and a creation State refuses announces nothing
	// at all (C-05 v1.4 p. 4, ADR-026).
	enc.opening = &started

	e.byPlayer[playerID] = enc
	// The stub folds its own create into its view the way apply folds an
	// update, so that the first package of the fight has the version of the
	// encounter entity to pin. No action of the fight is answered before
	// entity.created comes back (route), but the view is ready when it does.
	e.world[enc.id] = entity.New(entity.Ref{ID: enc.id, Type: entity.TypeEncounter},
		e.worldID, "", attributes, ev.Timestamp)
	e.log.Info("encounter proposed", "encounter_id", enc.id, "player_id", playerID,
		"npc_id", npcID, "region_id", region, "event_id", started.ID)
	return nil
}

// attacked answers a swing: the rolls, the two decisions of the exchange, one
// atomic proposal for everything they changed, and the end of the fight when
// somebody fell (design §4.1).
func (e *FakeEncounter) attacked(ctx context.Context, ev eventbus.Event, p actionPayloadOf) error {
	enc, playerID, ok := e.fightOf(p.Entity, ev)
	if !ok {
		return nil
	}
	npcID := e.targetOf(enc, p.Target)
	if npcID == "" {
		e.log.Debug("nothing left to attack", "encounter_id", enc.id, "player_id", playerID)
		return nil
	}

	actors, err := e.actorsOf(enc, playerID, npcID)
	if err != nil {
		return err
	}
	ans := &answer{cause: ev, causeName: CauseCombat, before: enc.participation()}
	enc.touched = false

	out, rolls, err := e.mech.Resolve(ev.ID, rollAttack,
		mech.Action{Kind: mech.ActionAttack, Actor: playerID, Target: npcID}, actors)
	if err != nil {
		return fmt.Errorf("testkit/swarm: resolve attack of %s: %w", playerID, err)
	}
	// A killing blow is the whole exchange; any other is answered by the NPC.
	decision, err := e.report(ctx, ev, enc, mech.ActionAttack, playerID, npcID, out, rolls, 0,
		exchangeStep{index: 0, last: out.TargetDead})
	if err != nil {
		return err
	}
	ans.strike(npcID, playerID, out, decision, ev.Timestamp)
	if out.Hit {
		enc.lastDamager[npcID] = playerID
		enc.dealt(playerID, out.Damage, ev.Timestamp)
		actors[npcID].HP = out.HPAfter
	}

	// The answer of the NPC. A wolf that has just fallen does not bite back,
	// which is why an exchange is two decisions on a hit that did not kill and
	// one on a hit that did.
	playerDead := false
	if !out.TargetDead {
		bite, biteRolls, err := e.mech.Resolve(ev.ID, rollNPCCounter,
			mech.Action{Kind: mech.ActionNPCAttack, Actor: npcID, Target: playerID}, actors)
		if err != nil {
			return fmt.Errorf("testkit/swarm: resolve answer of %s: %w", npcID, err)
		}
		biteDecision, err := e.report(ctx, ev, enc, mech.ActionNPCAttack, npcID, playerID,
			bite, biteRolls, 0, exchangeStep{index: 1, last: true})
		if err != nil {
			return err
		}
		ans.strike(playerID, npcID, bite, biteDecision, ev.Timestamp)
		playerDead = bite.TargetDead
	}

	resolution, killer := "", ""
	switch {
	case out.TargetDead:
		resolution, killer = entity.ResolutionNPCDead, playerID
	case playerDead:
		resolution = entity.ResolutionPlayersOut
	}
	return e.finish(ctx, enc, ans, resolution, killer)
}

// fled answers an attempt to walk away: one roll against a threshold that grows
// with the enemies still standing, and — when it fails and the rules say so —
// a strike out of turn (rules/dark-forest.yaml flee.on_fail).
func (e *FakeEncounter) fled(ctx context.Context, ev eventbus.Event, p actionPayloadOf) error {
	enc, playerID, ok := e.fightOf(p.Entity, ev)
	if !ok {
		return nil
	}
	npcID := e.targetOf(enc, nil)
	actors, err := e.actorsOf(enc, playerID, npcID)
	if err != nil {
		return err
	}
	ans := &answer{cause: ev, causeName: CauseFlee, before: enc.participation()}
	enc.touched = false

	living := e.livingCount(enc)
	out, rolls, err := e.mech.Resolve(ev.ID, rollFlee, mech.Action{
		Kind: mech.ActionFlee, Actor: playerID, LivingEnemies: living,
	}, actors)
	if err != nil {
		return fmt.Errorf("testkit/swarm: resolve flight of %s: %w", playerID, err)
	}
	escaped := out.Success != nil && *out.Success
	// A flight that failed is answered by a strike out of turn when the rules
	// call for one and somebody is left standing to deal it; any other flight
	// is the whole exchange.
	caught := !escaped && out.FreeAttack && npcID != ""
	if _, err := e.report(ctx, ev, enc, mech.ActionFlee, playerID, "", out, rolls, living,
		exchangeStep{index: 0, last: !caught}); err != nil {
		return err
	}

	if escaped {
		// A character who got away stands outside the world, where the rules
		// put them (flee.success_position, DR-19).
		ans.fleer = playerID
		ans.fleeTo = e.rules.FleePosition(e.worldID, enc.region)
	}

	playerDead := false
	if caught {
		bite, biteRolls, err := e.mech.Resolve(ev.ID, rollFreeAttack,
			mech.Action{Kind: mech.ActionFreeAttack, Actor: npcID, Target: playerID}, actors)
		if err != nil {
			return fmt.Errorf("testkit/swarm: resolve free attack of %s: %w", npcID, err)
		}
		decision, err := e.report(ctx, ev, enc, mech.ActionFreeAttack, npcID, playerID,
			bite, biteRolls, 0, exchangeStep{index: 1, last: true})
		if err != nil {
			return err
		}
		ans.strike(playerID, npcID, bite, decision, ev.Timestamp)
		playerDead = bite.TargetDead
	}

	resolution := ""
	if escaped || playerDead {
		// Solo: with the one character gone or down, there is nobody left the
		// fight could go on with (C-04, encounter.ended reason players_out).
		resolution = entity.ResolutionPlayersOut
	}
	return e.finish(ctx, enc, ans, resolution, "")
}

// finish ends every action the encounter accepts, and it is the whole of the
// decision of 2026-09-11: one action, exactly one atomic proposal.
//
// The turn is recorded on the encounter entity before the package leaves, so
// there is always something to propose — a miss wounds nobody, but it spends
// the round, and a round nobody wrote down can be closed neither by "everybody
// acted" nor by the threshold of missed turns. When the exchange also ended the
// fight, the entity is closed in the same package rather than in a second one.
//
// encounter.ended is built before the package, so that closed_by_event_id can
// name it — an identifier is assigned when an event is constructed, not when it
// is sent (C-01) — and it is published only once the fact of the package comes
// back (announce). A package State never applies ends nothing: the fight goes
// on (C-05 v1.4 p. 5, ADR-026).
//
// The round is the decision's and is never taken back: combat.decided has
// published it, and the encounter entity catches up with the next package that
// goes in, because round_seq is set rather than incremented (C-05 v1.4 p. 1г).
func (e *FakeEncounter) finish(ctx context.Context, enc *encounter, ans *answer,
	resolution, killer string) error {
	endedBy := ""
	if resolution != "" {
		enc.leaveAll()
		ended := e.endEvent(ans.cause, enc, resolution, killer)
		enc.closing = &closure{ended: ended, proposalID: ans.proposalID(), reason: resolution}
		endedBy = ended.ID
	}
	ans.encOps = e.turnOps(enc, resolution, endedBy)
	enc.pending = ans
	if err := e.propose(ctx, enc, ans); err != nil {
		return err
	}
	enc.round++
	return nil
}

// strike records a blow that landed. A miss is not recorded at all: it wounds
// nobody, and the round it cost is written by turnOps.
func (a *answer) strike(targetID, attackerID string, out mech.Outcome,
	decisionID string, at time.Time) {
	if !out.Hit {
		return
	}
	a.blows = append(a.blows, blow{
		targetID: targetID, attackerID: attackerID, damage: out.Damage,
		decisionID: decisionID, kills: out.TargetDead, at: at,
	})
}

// turnOps is what the action wrote on the encounter entity — the entity §4.6
// gives to the task level on any path with cause combat and flee, and the
// reason a package is never empty. The round it consumed is written even by a
// miss: a round nobody wrote down can be closed neither by "everybody acted"
// nor by the threshold of missed turns.
//
// It is decided once and kept: a retry offers the same turn again, and a
// second reading of the round counter — which the accepted attempt would have
// already moved — would write a round the fight never fought.
//
// participants[] travels only when participation actually moved: a set that
// changes nothing would make State publish a fact with an empty changed[].
func (e *FakeEncounter) turnOps(enc *encounter, resolution, endedByEventID string) []entity.Op {
	ops := make([]entity.Op, 0, 4)
	if resolution == "" {
		ops = append(ops, entity.Op{
			Op: entity.OpSet, Path: entity.AttrRoundSeq, Value: enc.round + 1,
		})
	} else {
		// A fight that is over opens no next round; what it records instead is
		// how it ended and the event that says so (data-model.md §3.7).
		ops = append(ops,
			entity.Op{Op: entity.OpSet, Path: entity.AttrState, Value: entity.EncounterStateResolved},
			entity.Op{Op: entity.OpSet, Path: entity.AttrResolution, Value: resolution},
			entity.Op{Op: entity.OpSet, Path: entity.AttrClosedByEventID, Value: endedByEventID},
		)
	}
	if enc.touched {
		ops = append(ops, entity.Op{
			Op: entity.OpSet, Path: entity.AttrParticipants, Value: enc.participantsPayload(),
		})
	}
	return ops
}

// pack turns what the action decided into the package that carries it, against
// the world as the stub sees it now, and says whether that world still agrees
// with the decision about who is left standing.
//
// It is called once per attempt, which is what makes a retry a fresh answer
// rather than the refused one with a new version on it: the damage of a blow
// comes off the hit points the target has at this moment. Everything the fight
// decided without reading the world — the round, the end of the fight, where a
// character who got away stands — is carried over unchanged from the first
// attempt.
//
// drift is empty when every blow still does to its target what the decision
// said it did, and otherwise names the fighter the world now disagrees about:
// one who has fallen since, one the same damage would now kill, one it would
// no longer kill. There is no package then. On the first attempt drift is
// always empty, because the decision was made from this very view a moment ago.
func (e *FakeEncounter) pack(enc *encounter, ans *answer) (pending *changes, drift string) {
	pending = newChanges()
	if ans.fleeTo != nil {
		if who := e.entityOf(ans.fleer); who != nil {
			pending.add(who, entity.Op{
				Op: entity.OpSet, Path: entity.AttrPosition, Value: ans.fleeTo,
			})
		}
	}

	// Two blows of one turn never fall on the same fighter today — a character
	// strikes the wolf and the wolf bites back — but the hit points are
	// threaded through anyway, so that the second of two blows on one target
	// would take its damage off what the first one left rather than off what
	// the world still holds.
	hp := make(map[string]int, 2)
	for _, b := range ans.blows {
		target := e.entityOf(b.targetID)
		if target == nil {
			e.log.Warn("a fighter the stub does not know", "entity_id", b.targetID)
			continue
		}
		if target.IsTerminal() {
			// Nobody strikes a corpse, and a blow that can no longer be dealt
			// is a decision the world has moved past: the decision either left
			// this fighter standing or claimed the kill for somebody else.
			return nil, b.targetID
		}
		before, threaded := hp[b.targetID]
		if !threaded {
			var known bool
			if before, known = target.HP(); !known {
				e.log.Warn("a fighter without hit points", "entity_id", b.targetID)
				continue
			}
		}
		hpMax, _ := target.HPMax()
		after := e.rules.ClampHP(before-b.damage, hpMax)
		if fell := after == 0; fell != b.kills {
			return nil, b.targetID
		}
		hp[b.targetID] = after
		e.wound(pending, target, b, after)
	}

	if ent := e.entityOf(enc.id); ent != nil {
		pending.add(ent, ans.encOps...)
	} else {
		e.log.Warn("the encounter entity is not in the world of the stub", "encounter_id", enc.id)
	}
	return pending, ""
}

// report publishes what a decision rests on and then the decision: one
// dice.rolled per roll before combat.decided, which is the order that makes a
// fight auditable (C-03). It answers the identifier of the combat.decided,
// which a trophy names as its source (inv-03).
//
// step is where the decision stands in its exchange. The stub fills it on every
// decision (C-05 v1.4 p. 7): it knows the shape of the exchange, which the
// narrator would otherwise have to guess.
func (e *FakeEncounter) report(ctx context.Context, cause eventbus.Event, enc *encounter,
	action, attacker, defender string, out mech.Outcome, rolls []mech.Roll,
	livingEnemies int, step exchangeStep) (string, error) {
	roller := entity.Ref{ID: attacker, Type: e.typeOf(attacker)}
	refs := make([]map[string]any, 0, len(rolls))
	for _, roll := range rolls {
		ev, err := e.publish(ctx, cause, TypeDiceRolled,
			mech.DiceRolledPayload(roll, roller), enc.agent())
		if err != nil {
			return "", err
		}
		refs = append(refs, map[string]any{
			"event": map[string]any{"id": ev.ID, "type": TypeDiceRolled},
			"index": roll.Index,
		})
	}

	outcome := map[string]any{
		"hit":      out.Hit,
		"natural":  out.Natural,
		"damage":   out.Damage,
		"critical": out.Critical,
		"fumble":   out.Fumble,
	}
	payload := map[string]any{
		"encounter":     refPayload(enc.id, entity.TypeEncounter, ""),
		"round":         map[string]any{"seq": enc.round},
		"action":        action,
		"attacker":      e.refOf(attacker, e.typeOf(attacker)),
		"outcome":       outcome,
		"rolls":         refs,
		"rules_version": e.rules.Version,
		"phase1_mode":   Phase1Mode,
		"exchange":      map[string]any{"index": step.index, "last": step.last},
	}
	if action == mech.ActionFlee {
		// A flight resolves against a threshold rather than against a target,
		// so combat.decided carries neither a defender nor hit points
		// (combat.decided.v1.json).
		outcome["success"] = out.Success
		outcome["threshold"] = out.Threshold
		outcome["living_enemies"] = livingEnemies
	} else {
		outcome["target_dead"] = out.TargetDead
		payload["defender"] = e.refOf(defender, e.typeOf(defender))
		payload["hp"] = map[string]any{
			"defender_before": out.HPBefore,
			"defender_after":  out.HPAfter,
			"defender_max":    e.hpMaxOf(defender),
		}
	}
	if out.FreeAttack {
		payload["free_attack"] = true
	}

	ev, err := e.publish(ctx, cause, TypeCombatDecided, payload, enc.agent())
	if err != nil {
		return "", err
	}
	e.log.Info("combat decided", "action", action, "attacker", attacker,
		"defender", defender, "hit", out.Hit, "event_id", ev.ID)
	return ev.ID, nil
}

// wound records what a landed strike changes: the hit points of whoever was
// struck, the death record of whoever fell, and the trophy of whoever landed
// the last hit.
//
// hpAfter is what pack computed from the hit points the target has now, and
// death follows from it rather than from what the decision said: inv-02 keeps
// hit points in [0, hp_max], so a fighter at zero is a fighter who fell.
//
// It writes into the change set rather than publishing, because everything one
// exchange changed travels as a single atomic proposal (C-03: one package per
// round; C-02 v1.3: one entity, one change set).
func (e *FakeEncounter) wound(pending *changes, target *entity.Entity, b blow, hpAfter int) {
	dead := hpAfter == 0
	ops := []entity.Op{{Op: entity.OpSet, Path: entity.AttrHP, Value: hpAfter}}
	if dead {
		ops = append(ops,
			entity.Op{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusDead},
			entity.Op{Op: entity.OpSet, Path: entity.AttrDiedAt, Value: b.at.UTC().Format(time.RFC3339)},
			entity.Op{Op: entity.OpSet, Path: entity.AttrKilledBy, Value: b.attackerID},
		)
	}
	pending.add(target, ops...)

	if !dead || target.Type != entity.TypeNPC {
		return
	}
	// The trophy: to the character who landed the last hit, and exactly once
	// (inv-03).
	//
	// Nothing counts the handouts, because two things already make a second one
	// impossible and a counter would only hide a break in either. The NPC turns
	// terminal in the view of the stub the moment the killing blow is proposed
	// (apply), so targetOf never offers it as a target again — a swing at a
	// corpse produces no decision at all. And should a proposal be redelivered,
	// the identifier of the item is derived from the decision that produced it,
	// which is what append deduplicates by (entity.ApplyOps, dedupeKeys).
	winner := e.entityOf(b.attackerID)
	if winner == nil {
		return
	}
	kind, _ := target.Kind()
	items := e.rules.Loot(kind)
	if len(items) == 0 {
		return
	}
	pending.add(target, entity.Op{
		Op: entity.OpSet, Path: entity.AttrLootClaimedBy, Value: b.attackerID,
	})
	for _, item := range items {
		pending.add(winner, entity.Op{
			Op:   entity.OpAppend,
			Path: entity.AttrInventory,
			Value: entity.Item{
				ItemID: b.decisionID + ":" + item.Kind,
				Kind:   item.Kind,
				Name:   item.Name,
				Source: entity.ItemSource{
					Entity:  target.Ref().EventRef(),
					EventID: b.decisionID,
				},
				AcquiredAt: b.at.UTC(),
			},
		})
	}
}

// propose publishes everything one action changed as a single atomic package,
// with the version each entity was expected to be at (C-02, ADR-013 p. 1), and
// applies it to the view of the stub.
//
// The empty package is a defect and not a turn: turnOps puts the round on the
// encounter entity for every action the fight accepts, so the only way here
// with nothing to say is a stub that lost the entity it created. changes has
// minItems 1, so publishing it would be a schema violation dressed up as a
// turn; it is logged loudly instead.
//
// A retry the world has moved past is not offered (pack). Working the end of
// the fight out again instead is not open to the stub: the decisions of the
// action are published, and they are what the end rests on — a killing blow is
// announced in combat.decided.outcome.target_dead, and a blow that did not kill
// was answered by a bite of the fighter it did not kill. A package that changed
// who fell would be answering a different action. So the stub gives up on it
// out loud, exactly as on a race it keeps losing, with the view where the facts
// put it (refused has rolled the refused attempt back). The end of the fight
// was never announced — it waits for the fact of this package — so the fight
// simply goes on (C-05 v1.4 p. 1в).
func (e *FakeEncounter) propose(ctx context.Context, enc *encounter, ans *answer) error {
	pending, drift := e.pack(enc, ans)
	if drift != "" {
		e.log.Error("the world moved past the decision, the package is not offered again",
			"proposal_id", ans.proposalID(), "attempts", ans.attempts,
			"entity_id", drift, "encounter_id", enc.id, "event_id", ans.cause.ID)
		return e.drop(ctx, enc, ans)
	}
	sets := pending.sets()
	if len(sets) == 0 {
		e.log.Error("an action of the fight changed nothing at all",
			"event_id", ans.cause.ID, "type", ans.cause.Type)
		return e.drop(ctx, enc, ans)
	}
	ans.attempts++
	if _, err := e.publish(ctx, ans.cause, TypeUpdateProposed, map[string]any{
		"proposal_id": ans.proposalID(),
		"changes":     sets,
		"atomic":      true,
		"cause":       ans.causeName,
	}, enc.agent(), ans.options()...); err != nil {
		return err
	}
	ans.undo = e.apply(pending)
	return nil
}

// endEvent builds the end of the fight (C-05, encounter.ended) without sending
// it, so that the package which closes the encounter entity can name it.
// killer is named only when a character landed the blow, which is what the
// schema allows it for.
func (e *FakeEncounter) endEvent(cause eventbus.Event, enc *encounter,
	reason, killer string, opts ...eventbus.DeriveOption) eventbus.Event {
	payload := map[string]any{
		"encounter": refPayload(enc.id, entity.TypeEncounter, ""),
		"reason":    reason,
		// round is the number of the round being fought, and the one that ends
		// the fight is the last one there was.
		"rounds": enc.round,
	}
	if killer != "" {
		payload["killer"] = e.refOf(killer, entity.TypePlayer)
	}
	// The identifier is derived from the cause of the package that closes the
	// fight and from the encounter (C-01 v1.4, ADR-027 p. 2): the package names
	// it in closed_by_event_id before its fact exists, and a retry of the same
	// package, or the same end rebuilt after a restart, gets the same id.
	return eventbus.Derive(cause, TypeEncounterEnded, Source, payload,
		append([]eventbus.DeriveOption{eventbus.WithAgent(enc.agent()), eventbus.WithCauseID(enc.id)},
			opts...)...)
}

// --- the lifecycle of a fight after the fact (C-05 v1.4 p. 4–6, ADR-026) ---

// announce publishes the lifecycle event a fact has just made true:
// encounter.started once State created the encounter, encounter.ended once
// State applied the package that closes it. An event that would announce what
// State has not accepted cannot be taken back, so it is not sent before the
// fact that makes it true.
//
// A publication that fails leaves the event held and returns the error: the
// bus delivers the fact again, and the retry announces the same event under the
// same identifier. The caller holds the lock.
func (e *FakeEncounter) announce(ctx context.Context, factType, entityID, proposalID string) error {
	enc := e.encounterByID(entityID)
	if enc == nil {
		return nil
	}
	switch {
	case factType == TypeCreated && enc.opening != nil:
		started := *enc.opening
		if err := e.bus.Publish(ctx, started); err != nil {
			return fmt.Errorf("testkit/swarm: publish %s: %w", TypeEncounterStarted, err)
		}
		enc.opening, enc.active = nil, true
		e.log.Info("encounter opened", "encounter_id", enc.id, "event_id", started.ID)
	case factType == TypeUpdated && enc.closing != nil && proposalID == enc.closing.proposalID:
		ended := enc.closing.ended
		if err := e.bus.Publish(ctx, ended); err != nil {
			return fmt.Errorf("testkit/swarm: publish %s: %w", TypeEncounterEnded, err)
		}
		e.log.Info("encounter ended", "encounter_id", enc.id, "reason", enc.closing.reason,
			"rounds", enc.round, "event_id", ended.ID)
		enc.closing, enc.active = nil, false
	}
	return nil
}

// drop lets go of a package State will never apply — the stub gave up on it,
// the world moved past its decision, or State refused it for a reason that is
// not a race. Participation goes back with it, as the hit points already have
// (rollback), and a closing package takes its end along: the fight goes on.
// The round stays where the decision put it (finish).
//
// What the fight held back for the package is answered now, and a fight whose
// last NPC has meanwhile fallen to somebody else is closed (advanceOne). The
// caller holds the lock.
func (e *FakeEncounter) drop(ctx context.Context, enc *encounter, ans *answer) error {
	enc.restore(ans.before)
	enc.pending = nil
	if enc.closing != nil && enc.closing.proposalID == ans.proposalID() {
		enc.closing = nil
	}
	if ans.causeName == CauseResolve {
		// Said once, here, and not on every fact that follows: the fight stays
		// open over a fallen NPC, and the closure is not offered again on the
		// same fact (advanceOne), so the actions of the fight go on being
		// answered rather than waiting for an end that will not come.
		e.log.Error("the closure of a fight whose NPC fell elsewhere was not applied; "+
			"the fight stays open and is not closed again on the same fact",
			"encounter_id", enc.id, "event_id", ans.cause.ID)
	}
	return e.advanceOne(ctx, enc)
}

// advance moves every fight on after a fact: the ones nothing is in flight for
// any more answer what they held back, and the ones left with nobody to fight
// are closed. The fights are taken in the order of their characters, so that
// one scenario publishes the same events in the same order on every run. The
// caller holds the lock.
func (e *FakeEncounter) advance(ctx context.Context) error {
	players := make([]string, 0, len(e.byPlayer))
	for id := range e.byPlayer {
		players = append(players, id)
	}
	slices.Sort(players)
	fights := make([]*encounter, 0, len(players))
	for _, id := range players {
		fights = append(fights, e.byPlayer[id])
	}
	var errs []error
	for _, enc := range fights {
		errs = append(errs, e.advanceOne(ctx, enc))
	}
	return errors.Join(errs...)
}

// advanceOne moves one fight on: nothing happens while its creation or its
// closing package is in flight; an active fight with no package of its own on
// the way and no NPC left standing is closed by its agent (closeOrphaned) —
// once per fact that downed the NPC; and otherwise the actions it held back
// are answered. A closure State turned down is not offered again on the same
// fact, and the fight then answers its actions as any open fight does: were
// it to keep choosing the closure, the actions held for it would wait for an
// end that never comes (Mi-1 of review #1 of T-419, C-05 v1.4 p. 5). The
// caller holds the lock.
func (e *FakeEncounter) advanceOne(ctx context.Context, enc *encounter) error {
	if enc.inFlight() {
		return nil
	}
	if enc.active && enc.pending == nil && enc.downed != nil && e.livingCount(enc) == 0 &&
		enc.orphaned != enc.downed.ID {
		return e.closeOrphaned(ctx, enc)
	}
	held := enc.deferred
	enc.deferred = nil
	var errs []error
	for _, ev := range held {
		errs = append(errs, e.route(ctx, ev))
	}
	return errors.Join(errs...)
}

// closeOrphaned closes a fight whose last NPC fell to a hand that is not this
// fight's (C-05 v1.4 p. 6, variant Б1 of ADR-026). The lifecycle belongs to the
// encounter agent, so the agent closes it: a package that resolves the
// encounter entity as npc_dead and takes every character out of the fight, with
// no trophy, and then — after its fact, like every end — encounter.ended with
// no killer, because nobody of this fight struck the blow.
//
// The cause is somebody else's fact, and the scope it carries is not the scope
// of the fight, so the package and the end are published under the scope of
// the encounter: that is how a narrator and a gateway find the players of the
// fight without participants[] in the event.
//
// Today nothing but a defect reaches here: the region GM gives one NPC to one
// active encounter (p. 6), so the only writer of the health of a wolf is the
// agent of its fight. When Entity-Actor and the breach of laws arrive
// (EPIC-006/007), this is the path they take. The caller holds the lock.
func (e *FakeEncounter) closeOrphaned(ctx context.Context, enc *encounter) error {
	cause := *enc.downed
	enc.orphaned = cause.ID
	scope := enc.scope
	ans := &answer{cause: cause, causeName: CauseResolve, before: enc.participation(), scope: &scope}
	enc.touched = false
	enc.leaveAll()
	ended := e.endEvent(cause, enc, entity.ResolutionNPCDead, "", eventbus.WithScope(&scope))
	enc.closing = &closure{ended: ended, proposalID: ans.proposalID(), reason: entity.ResolutionNPCDead}
	ans.encOps = e.turnOps(enc, entity.ResolutionNPCDead, ended.ID)
	enc.pending = ans
	e.log.Warn("the last NPC of the fight fell to somebody else's hand; the encounter agent closes it",
		"encounter_id", enc.id, "event_id", cause.ID)
	return e.propose(ctx, enc, ans)
}

// abandon ends a fight State refused to create. Nothing was announced about it
// and nothing will be: there is no encounter to tell about (C-05 v1.4 p. 4,
// variant A1 of ADR-026). The NPC is free again, and the actions held back for
// the creation are answered as what they now are — actions outside any fight.
//
// It is logged as an error: the region GM gives one NPC to one active encounter
// (p. 6), and with that rule kept a creation State turns down is a defect. The
// caller holds the lock.
func (e *FakeEncounter) abandon(ctx context.Context, enc *encounter, reason string) error {
	e.log.Error("State refused to create the encounter; encounter.started is not published",
		"encounter_id", enc.id, "proposal_id", enc.openedBy, "reason", reason)
	enc.opening = nil
	for id, fight := range e.byPlayer {
		if fight == enc {
			delete(e.byPlayer, id)
		}
	}
	delete(e.world, enc.id)
	return e.advanceOne(ctx, enc)
}

// encounterByID is the fight whose encounter entity this is. The caller holds
// the lock.
func (e *FakeEncounter) encounterByID(id string) *encounter {
	for _, enc := range e.byPlayer {
		if enc.id == id {
			return enc
		}
	}
	return nil
}

// openingBy is the fight still waiting for the creation this proposal asked
// for. The caller holds the lock.
func (e *FakeEncounter) openingBy(proposalID string) *encounter {
	for _, enc := range e.byPlayer {
		if enc.opening != nil && enc.openedBy == proposalID {
			return enc
		}
	}
	return nil
}

// downedBy remembers the fact that took an NPC of an active fight off its feet,
// whoever dealt the blow: when it was not this fight, it is the cause the fight
// is closed on (closeOrphaned). The caller holds the lock.
//
// A fact without an id cannot be that cause: the end of the fight takes its id
// from it (WithCauseID), which panics on it. Such a fact is a defect of the
// producer that only reaches here with validation on read off; the fight is
// then not closed on it, and the log says so instead of the fight staying
// open in silence.
func (e *FakeEncounter) downedBy(ent *entity.Entity, fact eventbus.Event) {
	if ent == nil || ent.Type != entity.TypeNPC || !ent.IsTerminal() {
		return
	}
	for _, enc := range e.byPlayer {
		if !enc.active || !slices.Contains(enc.npcs, ent.ID) {
			continue
		}
		if fact.ID == "" {
			e.log.Error("a fact without an id downed the NPC of a fight; the fight is not closed on it",
				"encounter_id", enc.id, "npc_id", ent.ID)
			continue
		}
		cause := fact
		enc.downed = &cause
	}
}

// publish derives an event from the action that caused it and sends it. The
// game master path, the correlation and the actor kind travel with the cause
// (C-01); meta.agent is the swarm agent the act belongs to, which the policies
// of the swarm topics require and contracts.md §0 requires of a stub.
func (e *FakeEncounter) publish(ctx context.Context, cause eventbus.Event, typ string,
	payload map[string]any, agent eventbus.AgentRef, opts ...eventbus.DeriveOption) (eventbus.Event, error) {
	ev := eventbus.Derive(cause, typ, Source, payload,
		append([]eventbus.DeriveOption{eventbus.WithAgent(agent)}, opts...)...)
	if err := e.bus.Publish(ctx, ev); err != nil {
		return ev, fmt.Errorf("testkit/swarm: publish %s: %w", typ, err)
	}
	return ev, nil
}

// encounterAgent is the agent one act of the fight is attributed to. The
// identifier names the scope and the character it serves (design §4.1) and does
// not depend on the level, because §4.6 reads meta.agent.level and never the
// name: one fight is served by one agent playing two roles.
func encounterAgent(playerID, level string) eventbus.AgentRef {
	id := EncounterAgentPrefix
	if playerID != "" {
		id += ":solo:" + playerID
	}
	return eventbus.AgentRef{
		ID:               id,
		Level:            level,
		Blueprint:        EncounterBlueprint,
		BlueprintVersion: "0.0.0",
	}
}

// --- the world as the stub sees it ---

// Observe folds a fact of State into the view of the stub. It is the handler
// Start subscribes system_events with, exported so that a test can hand the
// stub a fact directly.
//
// A refusal is answered by refused, which retries exactly one of the five
// reasons and treats the rest as defects to be seen.
func (e *FakeEncounter) Observe(ctx context.Context, ev eventbus.Event) error {
	switch ev.Type {
	case TypeCreated, TypeUpdated, TypeRejected:
	default:
		return nil
	}
	if world := eventbus.GetWorldIDFromEvent(ev); world != "" && world != e.worldID {
		return nil
	}
	if ev.Type == TypeRejected {
		return e.refused(ctx, ev)
	}

	var p factOf
	if err := decodePayload(ev.Payload, &p); err != nil {
		e.log.Warn("a fact the stub cannot read", "event_id", ev.ID, "err", err)
		return nil
	}
	id := p.Entity.Entity.ID
	if id == "" {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.settle(p.ProposalID)
	current, known := e.world[id]
	if !known {
		if ev.Type != TypeCreated {
			// An update about an entity the stub has never seen: it belongs to
			// a part of the world the stub was not seeded with.
			return nil
		}
		born := entity.New(entity.RefFrom(p.Entity.Entity), e.worldID,
			p.Entity.Name, p.Attributes, ev.Timestamp)
		born.Version = p.Version
		e.world[id] = born
		e.facts[id] = entity.Clone(born)
		return nil
	}
	if _, told := e.facts[id]; !told && ev.Type == TypeCreated {
		// The stub has been carrying this entity since it proposed it into
		// existence (entered). The fact is where its life in State begins, and
		// so where a refused package puts it back to.
		born := entity.New(entity.RefFrom(p.Entity.Entity), e.worldID,
			p.Entity.Name, p.Attributes, ev.Timestamp)
		born.Version = p.Version
		e.facts[id] = born
	}

	// entity.updated carries only the paths that ended up different (C-02). The
	// paths of a list element (inventory[0]) are skipped: the stub reads hit
	// points, status and position, and none of them is a list.
	ops := make([]entity.Op, 0, len(p.Changed))
	for _, change := range p.Changed {
		if strings.ContainsRune(change.Path, '[') {
			continue
		}
		ops = append(ops, entity.Op{Op: entity.OpSet, Path: change.Path, Value: change.New})
	}
	e.fold(current, ops, p.Version)
	e.fold(e.facts[id], ops, p.Version)
	e.downedBy(current, ev)
	if err := e.announce(ctx, ev.Type, id, p.ProposalID); err != nil {
		return err
	}
	return e.advance(ctx)
}

// fold writes one fact into one view of an entity. The version only ever goes
// up: the working view may already stand past the fact, because the stub folds
// what it proposed before the fact of it comes back (apply).
func (e *FakeEncounter) fold(ent *entity.Entity, ops []entity.Op, version int64) {
	if ent == nil {
		return
	}
	if len(ops) > 0 {
		attrs, _, err := entity.ApplyOps(ent, ops)
		if err != nil {
			e.log.Warn("a fact the stub cannot apply", "entity_id", ent.ID, "err", err)
		} else {
			ent.Attributes = attrs
		}
	}
	if version > ent.Version {
		ent.Version = version
	}
}

// refused answers a package State turned down.
//
// One of the five reasons is a race and the other four are defects. A version
// conflict says only that somebody moved an entity of the package between the
// moment the stub read it and the moment State was asked to change it — the
// stub reads the world through facts of its own subscription, and C-01 does
// not order two topics against each other, so a blow can overtake the fact
// that moved the character who struck it. That is the case an optimistic lock
// exists for, and the answer to it is the one every agent of the swarm will
// have to give: fold in what has arrived and offer the package again, a
// bounded number of times (decision of 2026-09-11, C-05).
//
// unknown_entity, invalid_op, dead_entity and duplicate_entity are not races.
// Retrying them would turn a defect that shows up once into a defect that
// shows up three times and then disappears, so they are logged and the
// package is dropped as it is when the stub gives up (drop): State applied
// nothing of it, and a closing package that hung on for ever would hold every
// later action of the fight back with it.
//
// A refusal of the creation of an encounter is the end of that fight before it
// began (abandon): encounter.started was never sent and never will be.
//
// The offer that follows is the same answer to the same action: the same
// proposal identifier, one package, recomputed against the world it will now
// be applied to (pack). It is not a second answer, and nothing here answers an
// action twice.
//
// Every way out of here that is not an offer leaves the view where the facts
// put it. The attempt State refused moved nothing, and a view that went on
// holding it would pin the next action to a version State never reached —
// losing that race too, and the one after it.
func (e *FakeEncounter) refused(ctx context.Context, ev eventbus.Event) error {
	pa := ev.Path()
	proposal, _ := pa.GetString("proposal_id")
	if !strings.HasPrefix(proposal, proposalPrefix) {
		// Somebody else's package: the gateway proposes its own changes and
		// answers for them itself (C-02 v1.1).
		return nil
	}
	// Delivery is at-least-once (C-01), for a refusal as for an action: a copy
	// of one lost race must not spend a second attempt on it.
	if e.refusals.Seen(ev.ID) {
		e.log.Debug("refusal already answered", "event_id", ev.ID, "proposal_id", proposal)
		return nil
	}
	reason, _ := pa.GetString("reason")
	entityID, _ := pa.GetString("entity.entity.id")
	e.log.Error("a proposal of the stub was refused", "proposal_id", proposal,
		"reason", reason, "entity_id", entityID, "event_id", ev.ID)

	e.mu.Lock()
	defer e.mu.Unlock()
	if enc := e.openingBy(proposal); enc != nil {
		return e.abandon(ctx, enc, reason)
	}
	enc, ans := e.awaiting(proposal)
	if ans == nil {
		if reason == ReasonVersionConflict {
			e.log.Error("a version conflict on a package the stub is no longer holding",
				"proposal_id", proposal, "entity_id", entityID)
		}
		return nil
	}
	e.rollback(ans)
	if reason != ReasonVersionConflict {
		e.log.Error("the package is dropped: a refusal that is not a race is not offered again",
			"proposal_id", proposal, "reason", reason, "encounter_id", enc.id, "event_id", ans.cause.ID)
		return e.drop(ctx, enc, ans)
	}
	if ans.attempts >= maxProposalAttempts {
		// The end of the road, and it is said out loud: the action has been
		// answered by nothing the world kept, and what the fight decided on it
		// — the wound it dealt, the end it reached — is not in State. The
		// round stays spent: it has been published with the decision.
		e.log.Error("the stub gave up on a package after losing the version race",
			"proposal_id", proposal, "attempts", ans.attempts,
			"entity_id", entityID, "encounter_id", enc.id, "event_id", ans.cause.ID)
		return e.drop(ctx, enc, ans)
	}
	e.log.Info("offering the package again after a version conflict",
		"proposal_id", proposal, "attempt", ans.attempts+1,
		"entity_id", entityID, "encounter_id", enc.id)
	return e.propose(ctx, enc, ans)
}

// settle drops the package a fact has just answered for. A fact that names the
// proposal says State applied it — the first attempt or a retry, it makes no
// difference — and from then on there is nothing left to retry. A refusal of
// that proposal turning up later answers an attempt that is over, and offering
// the package again would fold into the view of the stub a change State drops
// as already applied (§4.5), setting the two apart for good. The caller holds
// the lock.
func (e *FakeEncounter) settle(proposalID string) {
	if proposalID == "" {
		return
	}
	if enc, ans := e.awaiting(proposalID); ans != nil {
		enc.pending = nil
	}
}

// awaiting is the fight whose package is waiting for this answer. A fight
// answers one action at a time, so the search is over the fights and not over
// a register of packages that would have to be pruned. The caller holds the
// lock.
func (e *FakeEncounter) awaiting(proposalID string) (*encounter, *answer) {
	for _, enc := range e.byPlayer {
		if enc.pending != nil && enc.pending.proposalID() == proposalID {
			return enc, enc.pending
		}
	}
	return nil, nil
}

// factOf is the part of entity.created and entity.updated the stub reads.
type factOf struct {
	Entity     namedRef        `json:"entity"`
	ProposalID string          `json:"proposal_id"`
	Version    int64           `json:"version"`
	Attributes map[string]any  `json:"attributes"`
	Changed    []entity.Change `json:"changed"`
}

// apply folds the proposal the stub has just published into its own view,
// without waiting for the fact to come back.
//
// It is what lets two actions follow one another inside one round: expected_version
// is read from this view, and a stub that waited for entity.updated would send
// the second proposal under the version the first one has already moved. State
// applies exactly the operations it was given, so the two views agree — and
// when they do not, the package comes back refused and what it says here is
// taken back again (refused).
//
// It answers how to take it back: the attributes each entity carried before,
// and the version it was at. entity.ApplyOps computes into a fresh map and
// never writes into the one it was given, so the map that was there is still
// intact and nothing has to be copied.
func (e *FakeEncounter) apply(pending *changes) []restore {
	undo := make([]restore, 0, len(pending.order))
	for _, ent := range pending.order {
		ops := pending.ops[ent.ID]
		attrs, changed, err := entity.ApplyOps(ent, ops)
		if err != nil {
			e.log.Warn("the stub cannot apply its own proposal", "entity_id", ent.ID, "err", err)
			continue
		}
		undo = append(undo, restore{ent: ent, attributes: ent.Attributes, version: ent.Version})
		ent.Attributes = attrs
		if len(changed) > 0 {
			ent.Version++
		}
	}
	return undo
}

// rollback takes the last attempt of a package out of the view of the stub,
// because a package State refused changed nothing at all.
//
// What each entity goes back to is what the facts say about it — the version
// included. Putting back what the stub had read before it proposed would lose
// the race again on every attempt: the conflict says a fact has moved the
// entity, and that fact is already folded in, because entity.updated and the
// refusal of the package that lost to it travel on one topic in that order
// (C-01). A fact that arrived in the same window is kept for the same reason,
// which is what lets the next attempt take its damage off the hit points the
// world really has.
//
// An entity the facts say nothing about yet — an encounter the stub proposed
// into existence whose entity.created has not come back — falls back to what
// it was before the package: it is the only view of it there is.
func (e *FakeEncounter) rollback(ans *answer) {
	for _, r := range ans.undo {
		if kept, told := e.facts[r.ent.ID]; told {
			r.ent.Attributes = entity.Clone(kept).Attributes
			r.ent.Version = kept.Version
			continue
		}
		r.ent.Attributes = r.attributes
		r.ent.Version = r.version
	}
	ans.undo = nil
}

// freeNPCOf is the first NPC of a region that is still standing and is not
// already held by a fight, by identifier so that two runs of one scenario meet
// the same wolf.
//
// One NPC, one active encounter (data-model.md §3.7, C-05 v1.4 p. 6): the
// region GM, whose role the stub plays in opening a fight, does not open a
// second encounter around an NPC that is in one. Two fights over one wolf
// would give its hit points two writers, which is exactly what made the
// probes S3 and S4 of review #3 of T-219 reachable. The GM is the only opener
// in the region, so there is no race to guard against here.
func (e *FakeEncounter) freeNPCOf(region, playerID string) (string, bool) {
	living := e.livingNPCsOf(region)
	if len(living) == 0 {
		e.log.Debug("nothing alive in the region", "region_id", region, "player_id", playerID)
		return "", false
	}
	for _, id := range living {
		if !e.engaged(id) {
			return id, true
		}
	}
	e.log.Info("every NPC of the region is already in an encounter",
		"region_id", region, "player_id", playerID)
	return "", false
}

// engaged says whether an NPC is held by a fight that is being opened or is on.
// The caller holds the lock.
func (e *FakeEncounter) engaged(npcID string) bool {
	for _, enc := range e.byPlayer {
		if enc.open() && slices.Contains(enc.npcs, npcID) {
			return true
		}
	}
	return false
}

// livingNPCsOf is every NPC still standing in a region, sorted by identifier.
func (e *FakeEncounter) livingNPCsOf(region string) []string {
	ids := make([]string, 0, 4)
	for id, ent := range e.world {
		if ent.Type != entity.TypeNPC || ent.IsTerminal() {
			continue
		}
		home, _ := ent.RegionID()
		position, _ := ent.Position()
		if home != region && position != region {
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// fightOf is the encounter an action belongs to, and the character who acted.
// An action outside a fight is not an error: a look, an attack after the wolf
// fell, a flight from a fight that has already ended.
func (e *FakeEncounter) fightOf(who *namedRef, ev eventbus.Event) (*encounter, string, bool) {
	if who == nil || who.Entity.ID == "" {
		e.log.Warn("event names no acting entity", "event_id", ev.ID, "type", ev.Type)
		return nil, "", false
	}
	enc, ok := e.byPlayer[who.Entity.ID]
	if !ok || !enc.active {
		e.log.Debug("action outside an encounter",
			"event_id", ev.ID, "type", ev.Type, "player_id", who.Entity.ID)
		return nil, "", false
	}
	return enc, who.Entity.ID, true
}

// targetOf is whom the action names, or the first NPC of the encounter still
// standing. A target the encounter does not hold is refused rather than
// attacked: the stub fights the wolf it opened the encounter with.
func (e *FakeEncounter) targetOf(enc *encounter, target *namedRef) string {
	if target != nil && target.Entity.ID != "" && slices.Contains(enc.npcs, target.Entity.ID) {
		if ent := e.entityOf(target.Entity.ID); ent != nil && !ent.IsTerminal() {
			return target.Entity.ID
		}
		return ""
	}
	for _, id := range enc.npcs {
		if ent := e.entityOf(id); ent != nil && !ent.IsTerminal() {
			return id
		}
	}
	return ""
}

// livingCount is how many enemies are still standing — the number that raises
// the bar of a flight attempt (C-03 Action.LivingEnemies).
func (e *FakeEncounter) livingCount(enc *encounter) int {
	n := 0
	for _, id := range enc.npcs {
		if ent := e.entityOf(id); ent != nil && !ent.IsTerminal() {
			n++
		}
	}
	return n
}

// actorsOf builds the fighters of one exchange out of the world and out of the
// encounter they are standing in (C-03 ActorFromEntity). The encounter is
// passed as an entity because that is where participation and the last damager
// live: two fights would answer differently about the same wolf.
func (e *FakeEncounter) actorsOf(enc *encounter, ids ...string) (map[string]*mech.Actor, error) {
	view := enc.asEntity(e.worldID)
	actors := make(map[string]*mech.Actor, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		ent := e.entityOf(id)
		if ent == nil {
			return nil, fmt.Errorf("testkit/swarm: %s is not in the world of the stub", id)
		}
		actor, err := mech.ActorFromEntity(ent, view)
		if err != nil {
			return nil, fmt.Errorf("testkit/swarm: %w", err)
		}
		actors[id] = actor
	}
	return actors, nil
}

// asEntity renders the fight as the encounter entity C-03 expects, built from
// what the stub knows rather than from what State has published: the answer
// must not depend on whether a fact has come back yet.
func (enc *encounter) asEntity(worldID string) *entity.Entity {
	npcs := make([]any, 0, len(enc.npcs))
	for _, id := range enc.npcs {
		npc := map[string]any{"npc_id": id}
		if damager := enc.lastDamager[id]; damager != "" {
			npc["last_damager"] = damager
		}
		npcs = append(npcs, npc)
	}
	return entity.New(entity.Ref{ID: enc.id, Type: entity.TypeEncounter}, worldID, "",
		map[string]any{
			entity.AttrRegionID:     enc.region,
			entity.AttrState:        entity.EncounterStateActive,
			entity.AttrRoundSeq:     enc.round,
			entity.AttrParticipants: enc.participantsPayload(),
			entity.AttrNPCs:         npcs,
		}, time.Time{})
}

// agent is the encounter agent of this fight — the task level, which is every
// act of the fight after it was opened (§4.6).
func (enc *encounter) agent() eventbus.AgentRef {
	return encounterAgent(enc.players[0].id, AgentLevelTask)
}

// participantsPayload renders participants[] the way the encounter entity
// carries it, in the order the characters joined the fight.
func (enc *encounter) participantsPayload() []map[string]any {
	out := make([]map[string]any, 0, len(enc.players))
	for _, p := range enc.players {
		out = append(out, p.payload())
	}
	return out
}

// dealt records the damage a character has done in this fight. It is the half
// of participation a miss does not move; the round turnOps writes is the half
// it does.
func (enc *encounter) dealt(playerID string, damage int, at time.Time) {
	for _, p := range enc.players {
		if p.id != playerID {
			continue
		}
		p.damageDealt += damage
		p.lastHitAt = at
		enc.touched = true
		return
	}
}

// leaveAll takes every character out of the fight, which is what the end of an
// encounter does to participation whichever way it ended (data-model.md §3.7).
func (enc *encounter) leaveAll() {
	for _, p := range enc.players {
		if p.state == entity.ParticipationOutOfCombat {
			continue
		}
		p.state = entity.ParticipationOutOfCombat
		enc.touched = true
	}
}

// open says whether the fight holds its NPC: it is being created or it is on.
func (enc *encounter) open() bool { return enc.opening != nil || enc.active }

// inFlight says whether an action of the fight has to wait: its creation or
// its closing package has not been answered by State yet.
func (enc *encounter) inFlight() bool { return enc.opening != nil || enc.closing != nil }

// participation is a copy of what the package of the next action may change
// about the fighters — what a package State never applies gives back.
func (enc *encounter) participation() participation {
	players := make([]participant, len(enc.players))
	for i, p := range enc.players {
		players[i] = *p
	}
	return participation{players: players, lastDamager: maps.Clone(enc.lastDamager)}
}

// restore puts participation back to what it was before a package State never
// applied (C-05 v1.4 p. 1г). The characters of a fight do not change while it
// is on, so the copy lines up with them one for one.
func (enc *encounter) restore(was participation) {
	for i, p := range enc.players {
		if i < len(was.players) {
			*p = was.players[i]
		}
	}
	enc.lastDamager = maps.Clone(was.lastDamager)
	if enc.lastDamager == nil {
		enc.lastDamager = make(map[string]string)
	}
}

func (e *FakeEncounter) entityOf(id string) *entity.Entity { return e.world[id] }

func (e *FakeEncounter) typeOf(id string) string {
	if ent := e.entityOf(id); ent != nil {
		return ent.Type
	}
	return entity.TypePlayer
}

func (e *FakeEncounter) hpMaxOf(id string) int {
	if ent := e.entityOf(id); ent != nil {
		if hpMax, ok := ent.HPMax(); ok {
			return hpMax
		}
	}
	return 1 // hp.defender_max has minimum 1: a fighter without one is a defect
}

// refOf is an entity as _common.json#/$defs/EntityWithName carries it, with the
// name the stub knows it under.
func (e *FakeEncounter) refOf(id, fallbackType string) map[string]any {
	name := ""
	kind := fallbackType
	if ent := e.entityOf(id); ent != nil {
		name = ent.Name
		kind = ent.Type
	}
	return refPayload(id, kind, name)
}

func refPayload(id, kind, name string) map[string]any {
	out := map[string]any{"entity": map[string]any{"id": id, "type": kind}}
	if name != "" {
		out["name"] = name
	}
	return out
}

// --- the change set of one action ---

// changes accumulates the operations of one action, one set per entity.
//
// A package that names an entity twice has no outcome C-02 allows, and State
// refuses the whole of it (C-02 v1.3): the operations of an entity are merged
// here instead. The order is the order the entities were first touched, so that
// the same scenario produces the same package on every run.
type changes struct {
	order []*entity.Entity
	ops   map[string][]entity.Op
}

func newChanges() *changes {
	return &changes{ops: make(map[string][]entity.Op, 2)}
}

func (c *changes) add(ent *entity.Entity, ops ...entity.Op) {
	if ent == nil || len(ops) == 0 {
		return
	}
	if _, seen := c.ops[ent.ID]; !seen {
		c.order = append(c.order, ent)
	}
	c.ops[ent.ID] = append(c.ops[ent.ID], ops...)
}

// sets renders the change sets of entity.update.proposed. Every one of them
// pins the version: the paths this stub writes — hit points, status, inventory
// — are exactly the paths ADR-013 p. 1 makes expected_version mandatory for.
func (c *changes) sets() []entity.ChangeSet {
	out := make([]entity.ChangeSet, 0, len(c.order))
	for _, ent := range c.order {
		out = append(out, ent.Propose(c.ops[ent.ID], true))
	}
	return out
}
