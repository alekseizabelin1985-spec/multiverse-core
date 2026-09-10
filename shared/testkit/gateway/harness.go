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
// What it deliberately does not do: HTTP. There is no client, no session, no
// action_key and no idempotency window; the harness publishes what a request
// would have produced after the gateway had accepted it. Groups, rounds and
// the analytics of C-10 are not here either, and neither is combat: Attack and
// Flee would need someone to fight back, and the only stub of Phase 1 that
// does is testkit/swarm.FakeEncounter of EPIC-003 (contracts.md C-05, the
// decision of the orchestrator recorded in tasks.md T-018). The combat
// scenario lands with it, in I1-α.
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

// Group is the consumer group the harness reads system_events under, to know
// what became of the changes it proposed.
const Group = "testkit-gateway"

// The event types the harness publishes.
const (
	TypeEnteredRegion  = "player.entered_region"
	TypeLeftRegion     = "player.left_region"
	TypeLooked         = "player.looked"
	TypeSaid           = "player.said"
	TypeRested         = "player.rested"
	TypeCreateProposed = "entity.create.proposed"
	TypeUpdateProposed = "entity.update.proposed"
)

// The types the harness reads to learn the answer to a proposal.
const (
	TypeCreated  = "entity.created"
	TypeUpdated  = "entity.updated"
	TypeRejected = "entity.update.rejected"
)

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
	// changed is closed and replaced whenever settled grows, so that a waiting
	// action wakes without polling.
	changed chan struct{}
	seq     int

	subscription sync.WaitGroup
	subErr       error
	started      bool
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
func (h *Harness) WithTimeout(d time.Duration) *Harness {
	if d <= 0 {
		d = DefaultTimeout
	}
	h.timeout = d
	return h
}

// WorldID is the world the harness acts in.
func (h *Harness) WorldID() string { return h.worldID }

// Start subscribes to system_events, where the answers to the proposals of the
// harness arrive.
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

	h.subscription.Add(1)
	go func() {
		defer h.subscription.Done()
		if err := h.bus.Subscribe(ctx, eventbus.TopicSystemEvents, Group, h.Observe); err != nil {
			h.mu.Lock()
			h.subErr = err
			h.mu.Unlock()
			h.log.Error("subscription stopped", "err", err)
		}
	}()
	return nil
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

// Observe folds one event of system_events into what the harness knows. It is
// the handler Start subscribes with, and it is exported so that a test can
// hand the harness a fact directly instead of running a subscription.
//
// It is the read-model of the gateway cut down to what a harness needs: the
// version of every entity and the fate of every proposal. Nothing else here
// reads the world.
func (h *Harness) Observe(_ context.Context, ev eventbus.Event) error {
	switch ev.Type {
	case TypeCreated, TypeUpdated:
		pa := ev.Path()
		id, _ := pa.GetString("entity.entity.id")
		version, _ := pa.GetInt("version")
		proposal, _ := pa.GetString("proposal_id")
		h.record(id, int64(version), proposal, "")
	case TypeRejected:
		pa := ev.Path()
		proposal, _ := pa.GetString("proposal_id")
		reason, _ := pa.GetString("reason")
		if reason == "" {
			reason = "unknown"
		}
		h.record("", 0, proposal, reason)
	}
	return nil
}

// record folds one answer into the state of the harness and wakes whoever
// waits for it.
func (h *Harness) record(entityID string, version int64, proposalID, refusal string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if entityID != "" {
		h.versions[entityID] = version
	}
	if proposalID == "" {
		return
	}
	if _, known := h.settled[proposalID]; known {
		return
	}
	h.settled[proposalID] = refusal
	close(h.changed)
	h.changed = make(chan struct{})
}

// Version is what State last said the entity is at, and whether the harness
// has heard about it at all.
func (h *Harness) Version(id string) (int64, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.versions[id]
	return v, ok
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
