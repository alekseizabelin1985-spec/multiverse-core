// Package state is the stand-in for internal/state: it applies proposals in
// memory and publishes the facts of C-02 on the bus, so that the swarm and the
// gateway can be written, run and reviewed before State exists (design.md §5,
// tasks.md T-017).
//
// It is a stub about how a decision is reached, never about what a consumer
// sees. Every event it publishes is a real event of the registry, validated by
// the bus against the same schema the real State will be validated against,
// with source testkit/state — one of the publishers contracts.md §0 lists for
// these types. A consumer that stops working when the stub is replaced by
// internal/state is a consumer that read something the contract never
// promised.
//
// The start protocol is part of that promise. FakeState.Start publishes
// analytics.replay.completed in mode recovery once its subscription is live,
// because the consumers of MVP-1 wait for that signal before they build their
// projections (C-14, clarification v0.4; consolidation.md §14.1 TL2-6). A stub
// that stayed silent would force every consumer to grow a branch for the stub,
// which is exactly the coupling the stub exists to avoid.
//
// What it deliberately does not do — ownership (level_violation), invariants
// (law_violation), persistence of individual entities, intents, recovery from
// a journal, and deduplication by last_change — is listed in the "Чего не
// делает" column of design.md §5 and repeated on the methods below.
package state

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
)

// Source is the envelope source of everything the stub publishes. It is a
// value of the registry, not a name invented here: the contracts job checks
// source ∈ Spec.Publishers, and testkit/state is listed for entity.created,
// entity.updated, entity.update.rejected and analytics.replay.completed
// (contracts.md §0 v0.4).
const Source = contracts.SourceTestkitState

// Group is the consumer group the stub reads system_events under.
const Group = "testkit-state"

// Reasons of entity.update.rejected the stub can produce. They are the whole
// matrix of the stub, five of the seven C-02 defines: level_violation needs
// the ownership table and law_violation needs the invariants, and v0 has
// neither (design.md §5).
const (
	ReasonUnknownEntity   = "unknown_entity"
	ReasonVersionConflict = "version_conflict"
	ReasonInvalidOp       = "invalid_op"
	ReasonDuplicateEntity = "duplicate_entity"
	ReasonDeadEntity      = "dead_entity"
)

// CauseForget is the cause of the one transition that ends a living character
// without killing them: alive -> abandoned, proposed by the gateway in the
// cascade of /forget (C-02 v1.2, З-2).
const CauseForget = "forget"

// The event types the stub reads and writes.
const (
	TypeCreateProposed = "entity.create.proposed"
	TypeUpdateProposed = "entity.update.proposed"
	TypeCreated        = "entity.created"
	TypeUpdated        = "entity.updated"
	TypeRejected       = "entity.update.rejected"
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
	// WorldID is the world the stub owns. Everything it publishes carries it.
	WorldID string
	// RulesVersion travels in the snapshot pointer (§4.4). The stub does not
	// load the rules — internal/mechanics is a context library and shared/*
	// does not import one — so the caller that has the rule set passes its
	// version; empty is written as it is.
	RulesVersion string
	// Clock stamps the snapshot and measures the recovery. It defaults to the
	// real clock; a test that compares golden bytes passes clock.Manual.
	Clock clock.Clock
	// Log receives what the stub decided. It defaults to a logger that
	// discards everything, so that a test says nothing unless it asks to.
	Log *slog.Logger
	// DedupCapacity is how many proposal identifiers are remembered; zero
	// selects the default of eventbus.Dedup.
	DedupCapacity int
}

// FakeState is the world in memory plus the subscription that changes it.
type FakeState struct {
	bus     eventbus.Bus
	store   objstore.Client
	worldID string
	rules   string
	clock   clock.Clock
	log     *slog.Logger

	// invariants records that a caller asked for them. The flag changes
	// nothing except the log line: checking them is EPIC-002 work (T-054).
	invariants bool

	mu       sync.Mutex
	entities map[string]*entity.Entity
	applied  *eventbus.Dedup
	// cursor is the offset of the next unread message of system_events, the
	// value a snapshot carries so that a read-model can catch up from it
	// (C-14). It comes from the position of the event being handled.
	cursor int64
	seq    int64

	subscription sync.WaitGroup
	subErr       error
	started      bool
}

// New builds the stub. It does not touch the bus: nothing is published and
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
	return &FakeState{
		bus:      cfg.Bus,
		store:    cfg.Store,
		worldID:  cfg.WorldID,
		rules:    cfg.RulesVersion,
		clock:    c,
		log:      log.With("component", Source, "world_id", cfg.WorldID),
		entities: make(map[string]*entity.Entity),
		applied:  eventbus.NewDedup(cfg.DedupCapacity),
	}, nil
}

// WithInvariants records that the caller wants the laws of the world checked.
// In v0 it is a no-op with a log line: the identifiers exist and every Check
// is still nil (design.md §5, mechanics.Invariants). It returns the stub so
// that it can be chained onto New.
//
// It is not silently ignored, because a test that asks for invariants and gets
// none has to be able to see why it passed.
func (s *FakeState) WithInvariants() *FakeState {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invariants = true
	s.log.Warn("invariants are not checked by the stub: every Check is nil until EPIC-002 T-054")
	return s
}

// Seed puts entities into the world without a proposal and without a fact: it
// is the bootstrap of a test, the four fixture files of testdata/fixtures
// (LoadFixtures) or a world assembled by hand.
//
// The entities are copied, so a caller that goes on using the fixtures it
// loaded cannot change the world behind the back of the stub. Seeding twice
// over the same identifier is an error rather than a replacement: a test that
// seeds a world it already seeded is a test whose setup is wrong.
func (s *FakeState) Seed(entities []*entity.Entity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entities {
		if e == nil {
			return errors.New("testkit/state: seed: nil entity")
		}
		if e.ID == "" {
			return errors.New("testkit/state: seed: entity without an id")
		}
		if _, ok := s.entities[e.ID]; ok {
			return fmt.Errorf("testkit/state: seed: %s is already in the world", e.ID)
		}
		s.entities[e.ID] = entity.Clone(e)
	}
	return nil
}

// Get is one entity of the world, copied. A consumer of the stub reads the
// world off the bus like it will read it off the real State; this is for the
// test that has to assert what the stub decided.
func (s *FakeState) Get(id string) (*entity.Entity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entities[id]
	if !ok {
		return nil, false
	}
	return entity.Clone(e), true
}

// All is the whole world, copied, in the (type, id) order a snapshot uses.
func (s *FakeState) All() []*entity.Entity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotOrder()
}

// StateHash is the hash of the world as it stands (§3.3).
func (s *FakeState) StateHash() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return entity.StateHash(s.snapshotOrder())
}

// Start subscribes to system_events and announces the end of recovery, in
// that order.
//
// The order is what a consumer leans on: it starts proposing the moment it
// sees the signal, and on a broker where a new consumer group starts at the
// end of the journal a proposal published before the group is registered is
// never delivered. So the signal goes out only once the subscription has
// reported itself live.
//
// No broker of this platform is that broker, and the comment used to imply
// one: both implementations of C-01 start a new group at the first offset —
// the kafka adapter with StartOffset: kafka.FirstOffset (kafka.go), membus by
// keeping a cursor that begins at zero — so nothing published around Start is
// lost on either of them today (review #2 of T-017, Nit-5). The order is kept
// for the broker that does not have that property, not against the two that
// do.
//
// Who reports the moment is the bus. eventbus.Bus cannot: Subscribe blocks
// until the subscription ends, so the moment its group is registered is inside
// a call that has not returned (C-01). A bus that can name it implements
// readySubscriber and the stub waits for it; neither membus nor the kafka
// adapter does, and there the fallback runs on the first-offset guarantee
// above. Closing the residual gap needs a change to C-01 itself, which is an
// open question of T-017.
//
// Start returns as soon as the signal is published; the subscription lives
// until ctx is done, and Wait joins it. On a bus that reports readiness a
// subscription failing before it is live fails Start instead of being
// announced over; on a bus that does not, the fallback reports readiness
// before it calls Subscribe, so such a failure reaches the caller through Wait
// and the log, not through Start (review #2 of T-017, Minor-7).
func (s *FakeState) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("testkit/state: already started")
	}
	s.started = true
	s.mu.Unlock()

	began := s.clock.Now()
	ready := make(chan struct{})
	stopped := make(chan struct{})
	s.subscription.Add(1)
	go func() {
		defer s.subscription.Done()
		defer close(stopped)
		if err := s.subscribe(ctx, ready); err != nil {
			s.mu.Lock()
			s.subErr = err
			s.mu.Unlock()
			s.log.Error("subscription stopped", "err", err)
		}
	}()

	select {
	case <-ready:
	case <-stopped:
		s.mu.Lock()
		err := s.subErr
		s.mu.Unlock()
		if err != nil {
			return fmt.Errorf("testkit/state: subscribe %s: %w", eventbus.TopicSystemEvents, err)
		}
	}
	return s.announceRecovery(ctx, began)
}

// readySubscriber is what a bus implements when it can say at which moment its
// subscription is live: it closes ready once the consumer group is registered
// and the topic is being read, and goes on serving the subscription like
// Subscribe does.
type readySubscriber interface {
	SubscribeReady(ctx context.Context, topic, group string, h eventbus.Handler,
		ready chan<- struct{}) error
}

// subscribe runs the subscription and reports through ready when it is live —
// on the word of the bus if the bus gives one, otherwise at the last moment
// before the call that blocks.
//
// The branch is chosen by a type assertion, which no compiler checks: the
// assumption that no bus of this tree reports readiness is stated as a test
// instead (TestTheBusOfTheTestsDoesNotReportReadiness, Nit-4).
func (s *FakeState) subscribe(ctx context.Context, ready chan struct{}) error {
	if bus, ok := s.bus.(readySubscriber); ok {
		return bus.SubscribeReady(ctx, eventbus.TopicSystemEvents, Group, s.Apply, ready)
	}
	close(ready)
	return s.bus.Subscribe(ctx, eventbus.TopicSystemEvents, Group, s.Apply)
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
// Everything it counts is zero, and honestly so: the stub has replayed no
// journal, called no model and rolled no dice. snapshot_id is null for the
// same reason — the catch-up started from nothing. state_hash_after is the
// world as Seed left it, which is what a consumer compares its own projection
// against.
//
// The event is a root: no proposal caused it, and the run it identifies is the
// run of the process. It is published like any other event, so the schema of
// T-006 is what decides whether the stub got the payload right.
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

// ReplayModeRecovery is the mode the stub announces: it stands in for State,
// not for the harness of EPIC-005, which publishes mode=test into the same
// schema (C-14).
const ReplayModeRecovery = "recovery"

// seen reports whether a proposal has already been applied, without recording
// it. The window is the LRU of C-01, and remember is what puts an identifier
// into it — only once the proposal actually changed the world, the way §4.5
// p. 12 orders it, so that a proposer may resend a refused proposal under the
// same identifier and get the same refusal instead of silence.
//
// The lookup is a scan of the window rather than a hash probe: eventbus.Dedup
// answers "seen" only by recording, and a stub that reads a few thousand
// identifiers per test does not need the index. The caller holds the lock.
func (s *FakeState) seen(proposalID string) bool {
	if proposalID == "" {
		return false
	}
	return slices.Contains(s.applied.IDs(), proposalID)
}

// remember records an applied proposal. The window is also what a snapshot
// carries as applied_proposals (§4.4). The caller holds the lock.
func (s *FakeState) remember(proposalID string) {
	if proposalID != "" {
		s.applied.Seen(proposalID)
	}
}

// AppliedProposals are the proposal identifiers the stub has applied, most
// recent first — what a snapshot carries and what a test asserts a
// deduplicated proposal against.
func (s *FakeState) AppliedProposals() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applied.IDs()
}

// snapshotOrder returns copies of the entities in the (type, id) order §4.4
// fixes for a snapshot and §3.3 for the hash. The caller holds the lock.
func (s *FakeState) snapshotOrder() []*entity.Entity {
	out := make([]*entity.Entity, 0, len(s.entities))
	for _, e := range s.entities {
		out = append(out, entity.Clone(e))
	}
	slices.SortFunc(out, func(a, b *entity.Entity) int {
		if c := cmp.Compare(a.Type, b.Type); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out
}
