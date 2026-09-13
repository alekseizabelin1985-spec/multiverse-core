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

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Publisher is the half of the bus the Applier needs.
type Publisher interface {
	Publish(ctx context.Context, ev eventbus.Event) error
}

// ErrPublishFailed is why a world stops when the answer to a proposal could not
// be published before the context of the world ended: some of the answer may be
// on the bus, none of it is in the world, and nothing more of the world is
// decided until a restart catches up on the facts of the journal (§4.8).
var ErrPublishFailed = errors.New("state: publish_failed")

// The pauses between the attempts to publish one event double from the first
// to the cap and stay there.
const (
	publishRetryFirst = 100 * time.Millisecond
	publishRetryMax   = 5 * time.Second
)

// ApplierConfig builds an Applier. Store, Publisher and WorldID are mandatory.
type ApplierConfig struct {
	WorldID   string
	Store     *memstore.Store
	Publisher Publisher
	// Source is the envelope source of what the Applier publishes; empty
	// selects core/state. The replacement of shared/testkit/state (T-056)
	// passes testkit/state, which the registry lists for the same types.
	Source string
	// DedupCapacity is how many proposal identifiers are remembered; zero
	// selects the default of eventbus.Dedup.
	DedupCapacity int
	// Timers pace the attempts to publish an answer again; nil selects the
	// wall clock.
	Timers clock.Timers
	Log    *slog.Logger
	// Invariants are the laws of the world checked at step 8 (§4.5), in the
	// order of the register (mechanics.Rules.Invariants); nil checks none.
	Invariants []mechanics.Invariant
	// WithoutOwnership turns off step 6 — the ownership table and the norms of
	// the gateway over it — for the double of shared/testkit/state, whose
	// consumers propose the paths of a fight as whoever is at hand (C-02
	// "Заглушка", state-and-mechanics.md §8). The zero value enforces it, as
	// State does. The transition into abandoned is held either way (C-02 v1.2).
	WithoutOwnership bool

	// Objects is the object store the answers are written to before their
	// facts go out, and the snapshots with them (§4.3, §4.9); nil keeps the
	// world in memory only, as the double of shared/testkit/state does.
	Objects Store
	// Clock stamps taken_at and written_at of a snapshot, the one wall clock
	// State reads (§6.2); nil selects clock.Real.
	Clock clock.Clock
	// SnapshotEvery is the number of applied facts after which a snapshot is
	// written (MV_SNAPSHOT_EVERY_FACTS, §4.9); zero or less writes none on
	// the count.
	SnapshotEvery int
	// RulesVersion is the version of the rules the process runs, which the
	// pointer records (§4.4).
	RulesVersion string
	// Writer names the writer of the pointer; empty selects Writer().
	Writer string
}

// Applier turns the proposals of one world into facts (state-and-mechanics.md
// §4.5). Apply is not safe for concurrent use: the worker of the world is its
// only caller, and that is what makes State the single writer of the world.
// Stopped and Retrying may be read from anywhere.
type Applier struct {
	worldID string
	store   *memstore.Store
	pub     Publisher
	source  string
	timers  clock.Timers
	log     *slog.Logger
	// applied is the window of proposal identifiers whose facts are out. An
	// identifier goes in only after every event of its answer was published
	// (§4.5 p. 12).
	applied *eventbus.Dedup
	// owners is the ownership table of step 6; checkOwnership says whether it
	// is in force.
	owners         ownership
	checkOwnership bool

	objects      Store
	clock        clock.Clock
	every        int
	rulesVersion string
	writer       string
	// cursor is the offset of system_events past the last proposal the
	// Applier answered, and sinceSnapshot the facts applied since the last
	// snapshot. Only the goroutine that calls Apply touches them, and a
	// snapshot is written by that goroutine or after it has ended.
	cursor        int64
	sinceSnapshot int

	mu sync.Mutex
	// failure is what stopped the world, or nil. It stays for the life of the
	// Applier, over a Stop and a Start of its context.
	failure error
	// retrying is the number of failed attempts to publish the event the
	// Applier is publishing now; zero once it is out.
	retrying int
	// laws are the invariants of step 8; SetInvariants replaces them.
	laws []mechanics.Invariant
	// lastSnapshot is the pointer of the last snapshot written, and
	// snapshotErr the failure of an attempt after it.
	lastSnapshot *LatestPointer
	snapshotErr  error
}

// NewApplier builds the Applier of one world.
func NewApplier(cfg ApplierConfig) (*Applier, error) {
	switch {
	case cfg.WorldID == "":
		return nil, errors.New("state: applier: no world")
	case cfg.Store == nil:
		return nil, errors.New("state: applier: no store")
	case cfg.Publisher == nil:
		return nil, errors.New("state: applier: no publisher")
	}
	source := cfg.Source
	if source == "" {
		source = contracts.SourceState
	}
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	timers := cfg.Timers
	if timers == nil {
		timers = clock.RealTimers{}
	}
	wall := cfg.Clock
	if wall == nil {
		wall = clock.Real{}
	}
	writer := cfg.Writer
	if writer == "" {
		writer = Writer()
	}
	return &Applier{
		objects:      cfg.Objects,
		clock:        wall,
		every:        cfg.SnapshotEvery,
		rulesVersion: cfg.RulesVersion,
		writer:       writer,

		worldID: cfg.WorldID,
		store:   cfg.Store,
		pub:     cfg.Publisher,
		source:  source,
		timers:  timers,
		log:     log.With("world_id", cfg.WorldID),
		applied: eventbus.NewDedup(cfg.DedupCapacity),

		owners:         newOwnership(),
		checkOwnership: !cfg.WithoutOwnership,
		laws:           slices.Clone(cfg.Invariants),
	}, nil
}

// SetInvariants replaces the laws checked at step 8. It is for a caller that
// learns the rules after the Applier was built, as the double of
// shared/testkit/state does in WithInvariants.
func (a *Applier) SetInvariants(laws []mechanics.Invariant) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.laws = slices.Clone(laws)
}

func (a *Applier) lawsInForce() []mechanics.Invariant {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.laws
}

// Apply answers one event of system_events.
//
// A refusal is not an error: it is an entity.update.rejected on the bus, and the
// proposal was handled. The entities of an answer are written to the object
// store first (Config.Objects, C-02 "Гарантии"), its events are published
// next, and the working set keeps the answer last. An event that does not go
// out is published again — the same event, with its id and its bytes — until
// it does. This is the decision on Ma-1 of review #1 of T-055 (dev-log T-055,
// iteration 2), kept for the facts after the write as well: state-and-mechanics.md
// v0.4, §9, "Ошибка `Publish` ответа — факта или отказа, до и после PUT".
// Handing the failure to the bus instead would have it redeliver the proposal
// and, after its last attempt, park it in dead_letters: the events already out
// would stay on the bus, the world would not have moved, and the next proposal
// would announce another fact under the same version.
//
// ctx is the lifetime of the world: it ends those attempts, never a publication
// or a write under way. Once it has ended one, the world is stopped with
// ErrPublishFailed (or ErrPersistFailed), and every later call returns
// ErrWorldStopped without deciding anything.
//
// An answered proposal moves the cursor of the world past its offset in
// system_events, and the facts it applied may complete the count of a snapshot
// (§4.9), which is then written before Apply returns.
func (a *Applier) Apply(ctx context.Context, ev eventbus.Event) error {
	if err := a.answer(ctx, ev); err != nil {
		return err
	}
	if pos, ok := eventbus.PositionFromContext(ctx); ok && pos.Topic == eventbus.TopicSystemEvents {
		a.cursor = pos.Offset + 1
	}
	if a.objects != nil && a.every > 0 && a.sinceSnapshot >= a.every {
		// A snapshot that fails is reported by /health and the log; the
		// proposal was answered all the same.
		_, _ = a.snapshot(ctx, SnapshotInterval, &ev)
	}
	return nil
}

// answer decides one event of system_events and publishes the answer.
func (a *Applier) answer(ctx context.Context, ev eventbus.Event) error {
	if !IsProposal(ev.Type) {
		return nil
	}
	if world := worldOf(ev); world != a.worldID {
		if world == "" {
			warnWorldless(a.log, ev)
			return nil
		}
		// A proposal of another world is not addressed to this worker: a
		// refusal would answer for a world it does not own.
		a.log.Debug("proposal of another world passed over",
			"event_id", ev.ID, "type", ev.Type, "event_world_id", world)
		return nil
	}
	if err := a.Stopped(); err != nil {
		return err
	}
	if ev.ID == "" {
		// Every answer derives its id from the proposal event, and without an
		// id there is nothing to derive from (C-01 v1.5, "Причина без id"). An
		// envelope without an id reaches a handler only past a bus that does
		// not validate on read.
		a.log.Error("proposal without an event id: not answered", "type", ev.Type, "handled", false)
		return nil
	}
	p, err := ParseProposal(ev)
	switch {
	case errors.Is(err, ErrNoProposalID):
		a.log.Warn("proposal without proposal_id passed over: nothing to refuse it by",
			"event_id", ev.ID, "type", ev.Type, "handled", true)
		return nil
	case err != nil:
		a.log.Warn("malformed proposal refused", "event_id", ev.ID, "proposal_id", p.ID, "err", err)
		return a.publish(ctx, p.ID, rejectedFact(ev, a.source, p.ID, Rejection{Reason: ReasonInvalidOp}))
	}
	if p.Kind == KindCreate {
		return a.applyCreate(ctx, p)
	}
	return a.applyUpdate(ctx, p)
}

// applyCreate answers entity.create.proposed (§4.5, the paragraph on create).
func (a *Applier) applyCreate(ctx context.Context, p *Proposal) error {
	spec := p.Create
	if spec.Ref.ID == "" || spec.Ref.Type == "" {
		return a.refuse(ctx, p, Rejection{Reason: ReasonInvalidOp})
	}
	ref := spec.Ref
	// The form of the attributes is checked before the world is asked
	// anything: a number past ±(2^53-1) would be stored as one number and read
	// back from the fact as another (C-02 v1.6), whether or not the entity is
	// already there (acceptance of T-448).
	if !entity.JSONCompatible(spec.Attributes) {
		return a.refuse(ctx, p, Rejection{Reason: ReasonInvalidOp, Ref: &ref})
	}
	if a.alreadyApplied(p) {
		return a.resendUnpublished(ctx, p)
	}
	if _, exists := a.store.Get(a.worldID, ref.ID); exists {
		return a.refuse(ctx, p, Rejection{Reason: ReasonDuplicateEntity, Ref: &ref})
	}
	if a.checkOwnership && !a.owners.mayCreate(p.Proposer, ref.Type, p.Cause) {
		a.logOwnership(p, ref)
		return a.refuse(ctx, p, Rejection{Reason: ReasonLevelViolation, Ref: &ref})
	}

	created := entity.New(ref, a.worldID, spec.Name, spec.Attributes, p.Event.Timestamp)
	created.Commit(created.Attributes, nil, a.lastChange(p, 1))
	if len(a.lawsInForce()) > 0 {
		if r, broken := a.violation(a.overlay(nil, created), []string{ref.ID}); broken {
			return a.refuse(ctx, p, *r)
		}
	}
	fact := createdFact(p.Event, a.source, created, p.ID)
	if err := a.persist(ctx, p, []written{{entity: created}}); err != nil {
		return err
	}
	if err := a.publish(ctx, p.ID, fact); err != nil {
		return err
	}
	created.SetFactEventID(fact.ID)
	if err := a.store.Put(a.worldID, created); err != nil {
		return a.keepFailed(p, err)
	}
	a.remember(p.ID)
	a.sinceSnapshot++
	a.log.Info("entity created", "entity_id", ref.ID, "entity_type", ref.Type,
		"proposal_id", p.ID, "event_id", fact.ID)
	return nil
}

// planned is one change set decided on a copy: the entity as it will be, the
// attributes ApplyOps computed and the changes the fact will carry.
type planned struct {
	entity  *entity.Entity
	attrs   map[string]any
	changed []entity.Change
}

// applyUpdate answers entity.update.proposed (§4.5 p. 1–12).
//
// Every change set is decided on a copy. The copies are written to the object
// store, the events of the answer are published, and only then the copies
// replace the originals and the proposal is remembered (§4.5 p. 10–12). atomic=true refuses the package on its first problem;
// atomic=false refuses the change sets that failed and applies the rest
// (§4.5 p. 9), and last_change.batch_size is the number applied.
func (a *Applier) applyUpdate(ctx context.Context, p *Proposal) error {
	if len(p.Changes) == 0 {
		return a.refuse(ctx, p, Rejection{Reason: ReasonInvalidOp})
	}
	// One entity, one change set (C-02 v1.3, §4.5 p. 1a). Two sets over one
	// entity would be decided against the same version and published as two
	// facts under one version — the "strictly +1" of this pipeline broken by
	// the form of the package, whatever atomic says.
	if ref, twice := repeatedEntity(p.Changes); twice {
		a.log.Info("proposal names one entity twice", "proposal_id", p.ID, "entity_id", ref.ID,
			"remedy", "merge the operations of one entity into a single change set")
		return a.refuse(ctx, p, Rejection{Reason: ReasonInvalidOp, Ref: &ref})
	}
	if ref, bad := malformedOp(p.Changes); bad {
		return a.refuse(ctx, p, Rejection{Reason: ReasonInvalidOp, Ref: &ref})
	}
	if a.alreadyApplied(p) {
		return a.resendUnpublished(ctx, p)
	}

	var plans []planned
	var refusals []Rejection
	for _, set := range p.Changes {
		plan, refusal := a.plan(p, set)
		if refusal != nil {
			refusals = append(refusals, *refusal)
			if p.Atomic {
				return a.refuse(ctx, p, *refusal)
			}
			continue
		}
		plans = append(plans, plan)
	}
	plans, broken, whole := a.checkLaws(p, plans)
	if whole != nil {
		return a.refuse(ctx, p, *whole)
	}
	// Every refusal of step 8 names the entity of its own change set, and a
	// package names each entity once (C-02 v1.3), so no two refusals share an
	// entity and none is dropped.
	refusals = append(refusals, broken...)

	// The facts come out in ascending identifier order, the order of the writes
	// they follow (§4.5 p. 11–12).
	slices.SortFunc(plans, func(x, y planned) int { return cmp.Compare(x.entity.ID, y.entity.ID) })
	events := make([]eventbus.Event, 0, len(plans)+len(refusals))
	committed := make([]*entity.Entity, 0, len(plans))
	writes := make([]written, 0, len(plans))
	for _, plan := range plans {
		from := plan.entity.Version
		e := commit(plan, a.lastChange(p, len(plans)))
		events = append(events, updatedFact(p.Event, a.source, e, plan.changed, p))
		committed = append(committed, e)
		writes = append(writes, written{entity: e, fromVersion: from})
	}
	for _, r := range refusals {
		events = append(events, rejectedFact(p.Event, a.source, p.ID, r))
	}
	if err := a.persist(ctx, p, writes); err != nil {
		return err
	}
	if err := a.publish(ctx, p.ID, events...); err != nil {
		return err
	}
	if len(committed) == 0 {
		return nil
	}
	for i, e := range committed {
		e.SetFactEventID(events[i].ID)
	}
	if err := a.store.Put(a.worldID, committed...); err != nil {
		return a.keepFailed(p, err)
	}
	a.remember(p.ID)
	a.sinceSnapshot += len(committed)
	for _, e := range committed {
		a.log.Info("entity updated", "entity_id", e.ID, "version", e.Version,
			"cause", p.Cause, "proposal_id", p.ID, "event_id", e.LastEventID)
	}
	return nil
}

// plan decides one change set against the world: existence (§4.5 p. 3), the
// type it claims, the version (p. 4), the terminal status (p. 5), ownership
// (p. 6), the operations on a copy (p. 7) and the norms that read their result.
// The laws of step 8 are asked about the whole package afterwards (checkLaws).
func (a *Applier) plan(p *Proposal, set entity.ChangeSet) (planned, *Rejection) {
	ref := set.Ref()
	if ref.ID == "" || len(set.Ops) == 0 {
		return planned{}, &Rejection{Reason: ReasonInvalidOp, Ref: &ref}
	}
	// A version below zero is a malformed change set, not a conflict: no entity
	// is ever at it, and entity.update.rejected carries versions from zero up,
	// so a version_conflict naming it could never be published and would hold
	// the world in its attempts for good. The schema of the proposal refuses it
	// on read; this is the path past a bus that does not validate on read
	// (review #2 of T-055, Mi-7).
	if set.ExpectedVersion != nil && *set.ExpectedVersion < 0 {
		return planned{}, &Rejection{Reason: ReasonInvalidOp, Ref: &ref}
	}
	current, ok := a.store.Get(a.worldID, ref.ID)
	if !ok {
		return planned{}, &Rejection{Reason: ReasonUnknownEntity, Ref: &ref}
	}
	// The type a change set claims is checked against the world, and ownership
	// reads the type of the world: a character proposed as an NPC would
	// otherwise be changed under the rights of whoever may change an NPC
	// (backlog of review #1 of T-055, p. 2).
	if ref.Type != current.Type {
		return planned{}, &Rejection{Reason: ReasonInvalidOp, Ref: &ref}
	}
	if err := current.CheckVersion(set.ExpectedVersion); err != nil {
		var conflict entity.ErrVersionConflict
		if !errors.As(err, &conflict) {
			return planned{}, &Rejection{Reason: ReasonInvalidOp, Ref: &ref}
		}
		expected, actual := conflict.Expected, conflict.Actual
		return planned{}, &Rejection{Reason: ReasonVersionConflict, Ref: &ref,
			ExpectedVersion: &expected, ActualVersion: &actual}
	}
	// Step 5, against the entity as it was: a terminal entity takes nothing but
	// the four paths of a corpse. That covers leaving the terminal status, even
	// in a package that also erases died_at and killed_by — no law after the
	// change could tell, the record of the death would be gone (C-02 v1.5b) —
	// and a second abandoned over an abandoned character (C-02 v1.4).
	if current.IsTerminal() && !onlyCorpsePaths(set.Ops) {
		return planned{}, &Rejection{Reason: ReasonDeadEntity, Ref: &ref}
	}
	if a.checkOwnership && !a.owners.mayChange(p.Proposer, current.Type, p.Cause, set.Ops) {
		a.logOwnership(p, ref)
		return planned{}, &Rejection{Reason: ReasonLevelViolation, Ref: &ref}
	}
	attrs, changed, err := entity.ApplyOps(current, cloneOps(set.Ops))
	if err != nil {
		return planned{}, &Rejection{Reason: ReasonInvalidOp, Ref: &ref}
	}
	after := entity.Clone(current)
	after.Attributes = attrs
	if reason := statusRefusal(p.Proposer, p.Cause, set, current, after, a.checkOwnership); reason != "" {
		return planned{}, &Rejection{Reason: reason, Ref: &ref}
	}
	if a.checkOwnership {
		if reason, law := restRefusal(p.Proposer, set, current, after); reason != "" {
			return planned{}, &Rejection{Reason: reason, Ref: &ref, InvariantID: law}
		}
	}
	return planned{entity: current, attrs: attrs, changed: changed}, nil
}

// logOwnership says who was refused what, for the reader of level_violation.
func (a *Applier) logOwnership(p *Proposal, ref entity.Ref) {
	a.log.Info("proposal is not its proposer's to make", "proposal_id", p.ID, "entity_id", ref.ID,
		"proposer", p.Proposer.Kind, "level", p.Proposer.Level, "agent_id", p.Proposer.AgentID,
		"source", p.Proposer.Source, "cause", p.Cause)
}

// commit moves the copy of a plan to its next version (§4.5 p. 10). The copy
// came out of the store, so the working set does not see it until Put.
func commit(plan planned, change entity.LastChange) *entity.Entity {
	plan.entity.Commit(plan.attrs, plan.changed, change)
	return plan.entity
}

// lastChange is the commit record of a proposal (ADR-011). AppliedAt is the
// timestamp of the proposal in both modes, never the wall clock (C-02 v1.1).
func (a *Applier) lastChange(p *Proposal, batch int) entity.LastChange {
	return entity.LastChange{
		ProposalID:      p.ID,
		ProposalEventID: p.Event.ID,
		Cause:           p.Cause,
		AppliedAt:       p.Event.Timestamp,
		Atomic:          p.Atomic,
		BatchSize:       batch,
	}
}

// refuse publishes one refusal of the whole proposal.
func (a *Applier) refuse(ctx context.Context, p *Proposal, r Rejection) error {
	entityID := ""
	if r.Ref != nil {
		entityID = r.Ref.ID
	}
	a.log.Info("proposal rejected", "proposal_id", p.ID, "reason", r.Reason, "entity_id", entityID)
	return a.publish(ctx, p.ID, rejectedFact(p.Event, a.source, p.ID, r))
}

// publish sends the events of one answer in order. An event that does not go
// out is published again after a pause, as it is, until it does; when ctx ends
// first, the world stops with ErrPublishFailed.
//
// The publications themselves do not see ctx: a bus refuses a cancelled
// context, and a Stop that came while the answer was being published would
// otherwise cut the answer in half (C-01 v1.7).
func (a *Applier) publish(ctx context.Context, proposalID string, events ...eventbus.Event) error {
	out := context.WithoutCancel(ctx)
	for i, ev := range events {
		for attempt := 1; ; attempt++ {
			err := a.pub.Publish(out, ev)
			if err == nil {
				break
			}
			pause := retryPause(attempt)
			a.setRetrying(attempt)
			a.log.Warn("an event of the answer did not go out; it is published again as it is",
				"proposal_id", proposalID, "event_id", ev.ID, "type", ev.Type,
				"attempt", attempt, "pause", pause, "err", err, "handled", true)
			if a.wait(ctx, pause) != nil {
				return a.abandon(proposalID, events[:i], ev, attempt, err)
			}
		}
		a.setRetrying(0)
	}
	return nil
}

// wait is one pause between two attempts; it returns early with the error of
// ctx.
func (a *Applier) wait(ctx context.Context, pause time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t := a.timers.After(pause)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C():
		return nil
	}
}

// retryPause is the pause after the n-th failed attempt.
func retryPause(attempt int) time.Duration {
	pause := publishRetryFirst
	for i := 1; i < attempt && pause < publishRetryMax; i++ {
		pause *= 2
	}
	return min(pause, publishRetryMax)
}

// abandon stops the world on an answer that did not get out whole.
func (a *Applier) abandon(proposalID string, out []eventbus.Event, pending eventbus.Event, attempts int, cause error) error {
	a.setRetrying(0)
	published := make([]string, 0, len(out))
	for _, ev := range out {
		published = append(published, ev.ID)
	}
	failure := fmt.Errorf("%w: %w: %s %s of proposal %s did not go out after %d attempts: %w",
		ErrWorldStopped, ErrPublishFailed, pending.Type, pending.ID, proposalID, attempts, cause)
	a.halt(failure)
	a.log.Error("the answer to a proposal was abandoned before it got out; the world is stopped",
		"reason", "publish_failed", "proposal_id", proposalID, "published", published,
		"pending_event_id", pending.ID, "type", pending.Type, "attempts", attempts,
		"err", cause, "handled", false,
		"remedy", "restart the process: the world catches up on the facts of the journal")
	return failure
}

// keepFailed stops the world whose answer is out but whose store refused the
// entities it announced: the facts on the bus are ahead of the world, and
// deciding the next proposal would announce another fact of the same version.
func (a *Applier) keepFailed(p *Proposal, cause error) error {
	failure := fmt.Errorf("%w: the answer to proposal %s is out but the world did not keep it: %w",
		ErrWorldStopped, p.ID, cause)
	a.halt(failure)
	a.log.Error("the world did not keep an answer already published; the world is stopped",
		"proposal_id", p.ID, "event_id", p.Event.ID, "err", cause, "handled", false)
	return failure
}

// halt stops the world; the first failure stays.
func (a *Applier) halt(failure error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failure == nil {
		a.failure = failure
	}
}

// Stopped is the failure that stopped the world, or nil.
func (a *Applier) Stopped() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.failure
}

// Retrying is the number of failed attempts to publish the event under way, or
// zero.
func (a *Applier) Retrying() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.retrying
}

func (a *Applier) setRetrying(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.retrying = n
}

// worldOf is the world of an event as its envelope names it. The legacy
// fallback to the payload of eventbus.GetWorldIDFromEvent is not for new
// events (CLAUDE.md, "Конверт события").
func worldOf(ev eventbus.Event) string {
	if ev.World == nil {
		return ""
	}
	return ev.World.Entity.ID
}

// warnWorldless reports a proposal nobody answers: without a world in the
// envelope there is no worker to hand it to. C-02 v1.7 settles it: the publisher
// hears so from Publish (WorldRequired, C-01 v1.10), and State passes such a
// proposal over, loudly, without a fact or a refusal.
func warnWorldless(log *slog.Logger, ev eventbus.Event) {
	proposalID, _ := ev.Path().GetString("proposal_id")
	log.Warn("proposal without world in the envelope: no worker is addressed",
		"event_id", ev.ID, "type", ev.Type, "proposal_id", proposalID, "handled", true)
}

// repeatedEntity reports the first entity a package names more than once. The
// identifier alone decides.
func repeatedEntity(sets []entity.ChangeSet) (entity.Ref, bool) {
	seen := make(map[string]struct{}, len(sets))
	for _, set := range sets {
		ref := set.Ref()
		if _, twice := seen[ref.ID]; twice {
			return ref, true
		}
		seen[ref.ID] = struct{}{}
	}
	return entity.Ref{}, false
}
