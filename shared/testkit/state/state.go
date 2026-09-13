// Package state is the double of internal/state: State itself, run in memory.
// FakeState is state.Applier over memstore, without the object store, the
// intents and the snapshots of internal/state (state-and-mechanics.md §8, C-02
// "Заглушка"; T-056 replaced the stand-in of T-017 with it). A consumer of the
// double — the swarm, the gateway, their e2e — is answered by the decisions of
// the real pipeline: the versions, atomic packages, the terminal status, the
// deduplication of proposals, the form of changed[], the refusals and, with
// WithInvariants, the laws of the world.
//
// One thing it does not hold its consumers to: the ownership table
// (level_violation for a path that is not the proposer's). The tests of the
// consumers propose the paths of a fight as whoever is at hand, and the double
// stands in for the decisions about the world, not for the rights of the
// proposers; State enforces them. The transition into abandoned is held all the
// same (C-02 v1.2).
//
// Every event it publishes is a real event of the registry, validated by the
// bus against the same schema the real State is validated against, with source
// testkit/state — one of the publishers contracts.md §0 lists for these types.
//
// The start protocol is part of that promise. FakeState.Start publishes
// analytics.replay.completed in mode recovery once it has started its
// subscription, because the consumers of MVP-1 wait for that signal before they
// build their projections (C-14, clarification v0.4; consolidation.md §14.1
// TL2-6).
package state

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	internalstate "multiverse-core.io/internal/state"
	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
)

// Source is the envelope source of everything the double publishes. It is a
// value of the registry, not a name invented here: the contracts job checks
// source ∈ Spec.Publishers, and testkit/state is listed for entity.created,
// entity.updated, entity.update.rejected and analytics.replay.completed
// (contracts.md §0 v0.4).
const Source = contracts.SourceTestkitState

// Group is the consumer group the double reads system_events under.
const Group = "testkit-state"

// Reasons of entity.update.rejected, the seven of C-02 — the values of
// internal/state, spelled as the strings a consumer reads off the bus.
const (
	ReasonUnknownEntity   = string(internalstate.ReasonUnknownEntity)
	ReasonVersionConflict = string(internalstate.ReasonVersionConflict)
	ReasonInvalidOp       = string(internalstate.ReasonInvalidOp)
	ReasonDuplicateEntity = string(internalstate.ReasonDuplicateEntity)
	ReasonDeadEntity      = string(internalstate.ReasonDeadEntity)
	ReasonLevelViolation  = string(internalstate.ReasonLevelViolation)
	ReasonLawViolation    = string(internalstate.ReasonLawViolation)
)

// CauseForget is the cause of the one transition that ends a living character
// without killing them: alive -> abandoned, proposed by the gateway in the
// cascade of /forget (C-02 v1.2, З-2).
const CauseForget = internalstate.CauseForget

// The event types the double reads and writes.
const (
	TypeCreateProposed = internalstate.TypeCreateProposed
	TypeUpdateProposed = internalstate.TypeUpdateProposed
	TypeCreated        = internalstate.TypeCreated
	TypeUpdated        = internalstate.TypeUpdated
	TypeRejected       = internalstate.TypeRejected
	TypeReplayDone     = "analytics.replay.completed"
)

// Config builds a FakeState. Bus and WorldID are mandatory.
type Config struct {
	// Bus is where proposals are read from and facts are published to. The
	// in-process membus is what every test of MVP-1 passes.
	Bus eventbus.Bus
	// Store is where Snapshot writes. objstore.NewMemory() is the stub of the
	// object store; a nil store makes Snapshot an error rather than a silent
	// no-op.
	Store objstore.Client
	// WorldID is the world the double owns. Everything it publishes carries it.
	WorldID string
	// RulesVersion travels in the snapshot pointer (§4.4); empty is written as
	// it is.
	RulesVersion string
	// Clock stamps the snapshot and measures the recovery. It defaults to the
	// real clock; a test that compares golden bytes passes clock.Manual.
	Clock clock.Clock
	// Log receives what the double decided. It defaults to a logger that
	// discards everything, so that a test says nothing unless it asks to.
	Log *slog.Logger
	// DedupCapacity is how many proposal identifiers are remembered; zero
	// selects the default of eventbus.Dedup.
	DedupCapacity int
}

// FakeState is the world in memory, the Applier that decides it and the
// subscription that feeds the Applier.
type FakeState struct {
	bus     eventbus.Bus
	store   objstore.Client
	worldID string
	rules   string
	clock   clock.Clock
	log     *slog.Logger

	world   *memstore.Store
	applier *internalstate.Applier
	// deciding makes the Applier the single writer of the world, as the worker
	// of a world is in internal/state: the subscription, Seed and a test that
	// calls Apply directly take turns.
	deciding sync.Mutex

	mu sync.Mutex
	// broken is why the double cannot decide at all — the rules WithInvariants
	// asked for did not load — or nil. Apply returns it.
	broken error
	// cursor is the offset of the next unread message of system_events, the
	// value a snapshot carries so that a read-model can catch up from it
	// (C-14). It comes from the position of the event being handled.
	cursor int64
	seq    int64

	subscription sync.WaitGroup
	subErr       error
	started      bool
}

// New builds the double. It does not touch the bus: nothing is published and
// nothing is read until Start.
func New(cfg Config) (*FakeState, error) {
	if cfg.Bus == nil {
		return nil, errors.New("testkit/state: no bus")
	}
	if cfg.WorldID == "" {
		return nil, errors.New("testkit/state: no world")
	}
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	c := cfg.Clock
	if c == nil {
		c = clock.Real{}
	}
	log = log.With("component", Source)
	world := memstore.New()
	applier, err := internalstate.NewApplier(internalstate.ApplierConfig{
		WorldID:          cfg.WorldID,
		Store:            world,
		Publisher:        cfg.Bus,
		Source:           Source,
		DedupCapacity:    cfg.DedupCapacity,
		Log:              log,
		WithoutOwnership: true,
	})
	if err != nil {
		return nil, fmt.Errorf("testkit/state: %w", err)
	}
	return &FakeState{
		bus:     cfg.Bus,
		store:   cfg.Store,
		worldID: cfg.WorldID,
		rules:   cfg.RulesVersion,
		clock:   c,
		log:     log.With("world_id", cfg.WorldID),
		world:   world,
		applier: applier,
	}, nil
}

// WithInvariants holds every proposal to the laws of the world: the invariants
// rules/dark-forest.yaml switches on, checked by internal/mechanics over the
// world as the proposal would leave it (§4.5 p. 8). A break is refused
// law_violation with the identifier of the law. It returns the double so that
// it can be chained onto New.
//
// The rules are read from the tree the package was built in. When they do not
// load, every Apply returns why rather than deciding without the laws a test
// asked for.
func (s *FakeState) WithInvariants() *FakeState {
	laws, err := invariantsOfTheTree()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.broken = fmt.Errorf("testkit/state: with invariants: %w", err)
		s.log.Error("the laws of the world did not load; the double decides nothing", "err", err)
		return s
	}
	s.applier.SetInvariants(laws)
	s.log.Info("the laws of the world are checked", "invariants", len(laws))
	return s
}

// Seed puts entities into the world without a proposal and without a fact: it
// is the bootstrap of a test, the four fixture files of testdata/fixtures
// (LoadFixtures) or a world assembled by hand.
//
// The entities are copied, so a caller that goes on using the fixtures it
// loaded cannot change the world behind the back of the double. Seeding twice
// over the same identifier is an error rather than a replacement: a test that
// seeds a world it already seeded is a test whose setup is wrong.
func (s *FakeState) Seed(entities []*entity.Entity) error {
	s.deciding.Lock()
	defer s.deciding.Unlock()
	named := make(map[string]bool, len(entities))
	for _, e := range entities {
		switch {
		case e == nil:
			return errors.New("testkit/state: seed: nil entity")
		case e.ID == "":
			return errors.New("testkit/state: seed: entity without an id")
		case named[e.ID]:
			return fmt.Errorf("testkit/state: seed: %s is named twice", e.ID)
		}
		named[e.ID] = true
		if _, ok := s.world.Get(s.worldID, e.ID); ok {
			return fmt.Errorf("testkit/state: seed: %s is already in the world", e.ID)
		}
	}
	return s.world.Put(s.worldID, entities...)
}

// Get is one entity of the world, copied. A consumer of the double reads the
// world off the bus like it reads it off the real State; this is for the test
// that has to assert what the double decided.
func (s *FakeState) Get(id string) (*entity.Entity, bool) {
	return s.world.Get(s.worldID, id)
}

// All is the whole world, copied, in the (type, id) order a snapshot uses.
func (s *FakeState) All() []*entity.Entity { return s.world.List(s.worldID) }

// StateHash is the hash of the world as it stands (§3.3).
func (s *FakeState) StateHash() string { return entity.StateHash(s.All()) }

// Start launches the subscription to system_events in a goroutine and
// publishes the end of recovery without waiting for that subscription to be
// live: eventbus.Bus has no way to say when it is, since Subscribe blocks until
// the subscription ends. The signal may therefore reach the bus before the
// group of the double has joined, and a consumer that proposes on seeing it may
// publish before the double reads anything.
//
// Nothing published before or around Start is lost for that. A new consumer
// group reads its topic from the first offset, which C-01 guarantees on every
// implementation of the bus and the contract test of the bus pins on both of
// them (C-01 v1.2, ADR-022).
//
// Start returns as soon as the signal is published; the subscription lives
// until ctx is done, and Wait joins it. A subscription that fails reaches the
// caller through Wait and the log, not through Start.
func (s *FakeState) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("testkit/state: already started")
	}
	s.started = true
	s.mu.Unlock()

	began := s.clock.Now()
	s.subscription.Add(1)
	go func() {
		defer s.subscription.Done()
		if err := s.bus.Subscribe(ctx, eventbus.TopicSystemEvents, Group, s.Apply); err != nil {
			s.mu.Lock()
			s.subErr = err
			s.mu.Unlock()
			s.log.Error("subscription stopped", "err", err)
		}
	}()
	return s.announceRecovery(ctx, began)
}

// Wait blocks until the subscription started by Start has stopped and reports
// what stopped it. A subscription cancelled through its context is not a
// failure and reports nil, like every Bus implementation (C-01).
func (s *FakeState) Wait() error {
	s.subscription.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subErr
}

// announceRecovery publishes analytics.replay.completed in mode recovery: the
// signal the swarm and the gateway wait for before they build a projection
// (C-14 v0.4, TL2-6).
//
// Everything it counts is zero, and honestly so: the double has replayed no
// journal, called no model and rolled no dice. snapshot_id is null for the
// same reason — the catch-up started from nothing. state_hash_after is the
// world as Seed left it, which is what a consumer compares its own projection
// against.
func (s *FakeState) announceRecovery(ctx context.Context, began time.Time) error {
	ev := eventbus.NewRoot(TypeReplayDone, Source, s.worldID, nil, eventbus.ActorSystem,
		map[string]any{
			"mode": ReplayModeRecovery,
			"replay": map[string]any{
				"run_id":            s.worldID + ":" + Source,
				"snapshot_id":       nil,
				"events_replayed":   0,
				"llm_calls":         0,
				"dice_rolled_new":   0,
				"duration_ms":       max(s.clock.Now().Sub(began).Milliseconds(), 0),
				"state_hash_after":  s.StateHash(),
				"incomplete_record": false,
			},
		})
	if err := s.bus.Publish(ctx, ev); err != nil {
		return fmt.Errorf("testkit/state: announce recovery: %w", err)
	}
	s.log.Info("recovery announced", "event_id", ev.ID, "mode", ReplayModeRecovery)
	return nil
}

// ReplayModeRecovery is the mode the double announces: it stands in for State,
// not for the harness of EPIC-005, which publishes mode=test into the same
// schema (C-14).
const ReplayModeRecovery = "recovery"

// AppliedProposals are the proposal identifiers the double has applied, in the
// window of deduplication, oldest first — what a snapshot carries and what a
// test asserts a deduplicated proposal against.
func (s *FakeState) AppliedProposals() []string { return s.applier.AppliedProposals() }
