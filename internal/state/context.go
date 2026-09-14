// Package state is the single writer of the worlds (state-and-mechanics.md §4):
// it reads entity.create.proposed and entity.update.proposed off system_events,
// decides them against the world and answers with entity.created,
// entity.updated or entity.update.rejected (C-02).
//
// T-055 is the pipeline: the working set in memory (memstore), the Applier and
// its versions, one worker per world, and the context the process runs. T-056
// adds ownership, the laws of the world, the terminal status and deduplication
// by last_change; T-057 the object store behind the working set (write-through,
// intents) and the snapshots with their pointer (store.go, intent.go,
// snapshot.go); T-059 the recovery of a world before it takes a proposal, the
// state section of /health and the admin route (recovery.go, health.go,
// admin.go).
package state

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
)

// Name is the name of the context in --contexts.
const Name = "state"

// Config builds the context. Every field has a default.
type Config struct {
	// Worlds are the worlds State serves, one worker each; empty reads
	// MV_STATE_WORLDS when the context starts.
	Worlds []string
	// Log receives what State decided; empty takes the logger of Deps.
	Log *slog.Logger
	// DedupCapacity is the size of the windows of event and proposal
	// identifiers; zero selects the default of eventbus.Dedup.
	DedupCapacity int
	// Timers pace the attempts to publish an answer again; nil selects
	// clock.RealTimers. They are not Deps.Timers: those are the domain time of
	// the contexts, NullTimers in replay, and a pause of the transport is not
	// domain time — a NullTimers pause would never end, the way a redelivery of
	// the bus would not (C-01 v1.4 "Таймеры повторной доставки",
	// state-and-mechanics.md §6.2; review #2 of T-055, Mi-5). A test passes
	// manual timers here.
	Timers clock.Timers
	// Invariants are the laws every world of the context is held to (§4.5
	// p. 8), from the rule set of the process (mechanics.Rules.Invariants);
	// nil checks none.
	Invariants []mechanics.Invariant
	// Objects is the object store every world of the context writes through
	// to, snapshots into and is recovered from (§4.3, §4.8, §4.9); nil keeps
	// the worlds in memory only, writes no snapshot and recovers nothing. The
	// buckets of a world are created by mvctl world init (EnsureWorldBuckets),
	// not here.
	Objects objstore.Client
	// SnapshotEvery is the number of applied facts per world between two
	// snapshots; zero reads MV_SNAPSHOT_EVERY_FACTS when the context starts,
	// below zero writes none on the count. It matters only with Objects.
	SnapshotEvery int
	// RulesVersion is the version of the rules of the process, recorded in
	// every pointer (§4.4) and shown by /health.
	RulesVersion string
}

// Context is State as a context of the process (runtime.Context).
//
// State reads system_events with a cursor of its own, not under a consumer
// group (ADR-011 p. 4, state-and-mechanics.md §4.8): Journal.Tail from the end
// of the journal its recovery caught up to. With an object store every Start
// rebuilds the worlds from it (Applier.Recover), so a Start after a Stop in the
// same process is the restart of a process. Without one the worlds live in
// memory and are not rebuilt: the first Start reads the journal from its first
// offset, as a new consumer group would (C-01 v1.2), and a later Start goes on
// from the event after the last one handled, so that a proposal whose handling
// a Stop interrupted is read again.
//
// Its reading is a mediator of delivery (C-01 v1.6): it hands each proposal to
// the worker of its world, waits for the answer and returns the error of the
// worker to the bus. A worker answers with an error only when its world has
// stopped (ErrWorldStopped); an answer that does not go out is published again
// by the worker itself (Applier.Apply). The event goes into the window of the
// reading only after the worker answered without an error; remembered earlier,
// a redelivery would stop here and never reach the worker.
type Context struct {
	cfg   Config
	store *memstore.Store

	mu        sync.Mutex
	log       *slog.Logger
	runLog    *slog.Logger
	appliers  map[string]*Applier
	workers   map[string]*worker
	worlds    []string
	delivered *eventbus.Dedup
	running   bool
	cancel    context.CancelFunc
	subDone   chan struct{}
	subErr    error
	// next is the offset of system_events after the last event the reading
	// handled; the reading of a Start without an object store goes on from it.
	next atomic.Int64
}

// New builds the context. Nothing is read and nothing is published until Start.
func New(cfg Config) *Context { return newOver(cfg, memstore.New()) }

// newOver builds the context over a working set that already holds worlds.
func newOver(cfg Config, store *memstore.Store) *Context {
	return &Context{
		cfg:       cfg,
		store:     store,
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		delivered: eventbus.NewDedup(cfg.DedupCapacity),
	}
}

// Name is the context name of the process.
func (c *Context) Name() string { return Name }

// DependsOn is nothing: State starts first, and the other contexts wait for it
// over the bus (§7.3, C-14).
func (c *Context) DependsOn() []string { return nil }

// Start recovers every world when there is an object store, builds a worker per
// world and opens the reading of system_events. It returns as soon as the
// reading runs; it lives until Stop.
//
// Every world is recovered before the reading begins: the facts of the journal
// are caught up and the intents rolled forward before a proposal is decided
// (§4.8). A store or a journal that cannot be read fails the start; a snapshot
// that does not read whole or a world that does not fit together stops that
// world and fails /health instead.
//
// The lifetime of the reading is the context's own, not the one Start was
// given: Stop cancels it, before the process closes the bus (C-01 v1.7,
// ADR-023 p. 4).
//
// A context stopped once may be started again. It is refused while the run
// before is still ending — a Stop that ran out of its context leaves the
// reading or a worker behind, and a second worker over the same world would be
// a second writer (review #1 of T-055, Mi-2). The process starts each context
// once; the second start is what a test of the shutdown needs.
func (c *Context) Start(ctx context.Context, deps runtime.Deps) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return errors.New("state: context already started")
	}
	if !c.previousRunEnded() {
		return errors.New("state: the run before has not ended yet: its reading or a worker is still busy")
	}
	if deps.Bus == nil {
		return errors.New("state: no bus in deps")
	}
	journal := deps.Journal
	if journal == nil {
		// A test hands one transport as the bus alone; the process fills both
		// with it (C-01 v1.3).
		journal, _ = deps.Bus.(eventbus.Journal)
	}
	if journal == nil {
		return errors.New("state: no journal in deps")
	}
	switch {
	case c.cfg.Log != nil:
		c.log = c.cfg.Log
	case deps.Log != nil:
		c.log = deps.Log
	}
	log := c.log.With("context", Name)
	worlds, err := c.worldsToServe()
	if err != nil {
		return err
	}
	timers := c.cfg.Timers
	if timers == nil {
		timers = clock.RealTimers{}
	}
	var objects Store
	every := 0
	if c.cfg.Objects != nil {
		objects = NewObjectStore(c.cfg.Objects)
		if every, err = c.snapshotEvery(); err != nil {
			return err
		}
	}

	from, store, appliers := c.next.Load(), c.store, c.appliers
	if objects != nil {
		// Each start is a restart: the worlds, their windows and a world
		// stopped before are what the store and the journal say, not what this
		// process remembers.
		store, appliers = memstore.New(), nil
		if from, err = journal.End(ctx, eventbus.TopicSystemEvents); err != nil {
			return fmt.Errorf("state: end of %s: %w", eventbus.TopicSystemEvents, err)
		}
	}
	if appliers == nil {
		appliers = make(map[string]*Applier, len(worlds))
	}
	for _, world := range worlds {
		if _, ok := appliers[world]; ok {
			continue
		}
		applier, err := NewApplier(ApplierConfig{
			WorldID: world, Store: store, Publisher: deps.Bus,
			DedupCapacity: c.cfg.DedupCapacity, Timers: timers, Log: log,
			Invariants: c.cfg.Invariants,
			Objects:    objects, Clock: deps.Clock, SnapshotEvery: every,
			RulesVersion: c.cfg.RulesVersion,
		})
		if err != nil {
			return err
		}
		if objects != nil {
			if _, err := applier.Recover(ctx, journal, from); err != nil {
				return err
			}
		}
		appliers[world] = applier
	}
	workers := make(map[string]*worker, len(worlds))
	for _, world := range worlds {
		w := newWorker(world, appliers[world], log)
		w.start()
		workers[world] = w
	}

	subCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	c.store, c.appliers = store, appliers
	c.workers, c.worlds, c.cancel, c.subDone, c.subErr, c.running = workers, worlds, cancel, done, nil, true
	c.runLog = log
	c.next.Store(from)
	go func() {
		defer close(done)
		err := journal.Tail(subCtx, eventbus.TopicSystemEvents, from, c.dispatch)
		if err != nil {
			log.Error("reading of system_events stopped", "err", err, "handled", false)
		}
		// Only the run that owns done reports its error. Today the check
		// always holds: Start does not begin a new run until done is closed,
		// and done closes after this write (the defer above). It stays so
		// that a Start that one day stops waiting for the run before does not
		// get its /health overwritten by that run (review #2 of T-055, N-4).
		c.mu.Lock()
		if c.subDone == done {
			c.subErr = err
		}
		c.mu.Unlock()
	}()
	log.Info("state running", "worlds", strings.Join(worlds, ","), "from", from)
	return nil
}

// previousRunEnded reports whether the reading and every worker of the run
// before have returned. The caller holds the lock.
func (c *Context) previousRunEnded() bool {
	if c.subDone != nil {
		select {
		case <-c.subDone:
		default:
			return false
		}
	}
	for _, w := range c.workers {
		if !w.finished() {
			return false
		}
	}
	return true
}

// snapshotEvery is the count of facts between two snapshots: the configuration,
// or MV_SNAPSHOT_EVERY_FACTS when it names none.
func (c *Context) snapshotEvery() (int, error) {
	if c.cfg.SnapshotEvery != 0 {
		return c.cfg.SnapshotEvery, nil
	}
	every, err := env.SnapshotEveryFacts.Int()
	if err != nil {
		return 0, fmt.Errorf("state: %w", err)
	}
	return every, nil
}

// worldsToServe is the list of worlds, without blanks and repeats, in the order
// given. The caller holds the lock.
func (c *Context) worldsToServe() ([]string, error) {
	listed := c.cfg.Worlds
	from := "Config.Worlds"
	if len(listed) == 0 {
		listed, from = env.StateWorlds.List(), env.StateWorlds.Name()
	}
	var worlds []string
	for _, world := range listed {
		if world = strings.TrimSpace(world); world != "" && !slices.Contains(worlds, world) {
			worlds = append(worlds, world)
		}
	}
	if len(worlds) == 0 {
		return nil, fmt.Errorf("state: %s names no world", from)
	}
	return worlds, nil
}

// dispatch is the handler of the reading: the mediator between the bus and the
// workers (C-01 v1.6). An event it handled without an error moves the offset a
// later Start without an object store reads from past it.
func (c *Context) dispatch(ctx context.Context, ev eventbus.Event) error {
	if err := c.handle(ctx, ev); err != nil {
		return err
	}
	if pos, ok := eventbus.PositionFromContext(ctx); ok && pos.Topic == eventbus.TopicSystemEvents {
		c.next.Store(pos.Offset + 1)
	}
	return nil
}

func (c *Context) handle(ctx context.Context, ev eventbus.Event) error {
	if !IsProposal(ev.Type) {
		return nil
	}
	if c.delivered.Has(ev.ID) {
		return nil
	}
	world := worldOf(ev)
	c.mu.Lock()
	w, ok := c.workers[world]
	log := c.runLog
	c.mu.Unlock()
	if world == "" {
		warnWorldless(log, ev)
		return nil
	}
	if !ok {
		// Not a world of this State. It may be a world of another process of
		// State; answering for it here would be answering for a world this
		// process does not hold.
		return nil
	}
	if err := w.submit(ctx, ev); err != nil {
		return err
	}
	c.delivered.Add(ev.ID)
	return nil
}

// Stop cancels the reading, waits until its handler has returned — the proposal
// a worker holds is finished first — and stops the workers. The wait is bounded
// by ctx, which the process gives runtime.StopTimeout (C-01 v1.7).
//
// The cancellation also ends the attempts of a worker to publish an answer that
// does not go out: its world stops with publish_failed, and the proposal is not
// parked in dead_letters (Delivery.Deliver on a cancelled context).
//
// A reading that ended on its own before Stop is what Stop reports.
func (c *Context) Stop(ctx context.Context) error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	cancel, done, workers := c.cancel, c.subDone, c.workers
	c.running = false
	c.mu.Unlock()

	cancel()
	select {
	case <-done:
	case <-ctx.Done():
		// The workers are told to end all the same, so that they do once the
		// proposal they hold is finished; Start refuses until they have.
		for _, w := range workers {
			w.signalStop()
		}
		return fmt.Errorf("state: stop: the reading of %s did not return: %w", eventbus.TopicSystemEvents, ctx.Err())
	}
	var errs []error
	stopped := make([]string, 0, len(workers))
	for _, world := range sortedKeys(workers) {
		if err := workers[world].stop(ctx); err != nil {
			errs = append(errs, err)
			continue
		}
		stopped = append(stopped, world)
	}
	// The shutdown snapshot (§4.9): the reading has returned and the workers
	// have ended, so nothing changes the worlds any more, and the bus is still
	// open for snapshot.created — the process closes it after Stop (ADR-023
	// p. 4). A world stopped on a failure, or not yet initialised, writes none.
	if c.cfg.Objects != nil {
		for _, world := range stopped {
			applier := workers[world].applier
			if applier.Stopped() != nil || applier.Uninitialized() {
				continue
			}
			if _, err := applier.Snapshot(ctx, SnapshotShutdown); err != nil {
				errs = append(errs, fmt.Errorf("state: shutdown snapshot of %s: %w", world, err))
			}
		}
	}
	c.mu.Lock()
	subErr := c.subErr
	c.mu.Unlock()
	return errors.Join(append([]error{subErr}, errs...)...)
}

// Snapshot writes a snapshot of one world now, on the worker of that world
// between two proposals (§4.9): the admin route and the bootstrap of a world
// (T-058, T-059) ask for one with their reason.
func (c *Context) Snapshot(ctx context.Context, worldID, reason string) (*LatestPointer, error) {
	c.mu.Lock()
	running, w := c.running, c.workers[worldID]
	c.mu.Unlock()
	switch {
	case !running:
		return nil, ErrNotRunning
	case w == nil:
		return nil, fmt.Errorf("%w: %s", ErrUnknownWorld, worldID)
	case c.cfg.Objects == nil:
		return nil, ErrNoObjectStore
	}
	var pointer *LatestPointer
	err := w.do(ctx, func() error {
		var err error
		pointer, err = w.applier.Snapshot(ctx, reason)
		return err
	})
	return pointer, err
}

// ErrNotRunning is a snapshot asked of a context that is not running.
var ErrNotRunning = errors.New("state: the context is not running")

// ErrUnknownWorld is a world the context does not serve.
var ErrUnknownWorld = errors.New("state: not a world of this context")

// Health is what /health of the process reports for State: fail when a world
// stopped — after a panic, an answer abandoned before it got out, a snapshot
// that does not read whole or a world that does not fit together — or the
// reading ended on its own; degraded before Start, after Stop and while a world
// is degraded (Applier.WorldHealth); ok otherwise.
func (c *Context) Health() runtime.Status {
	c.mu.Lock()
	running, workers, worlds := c.running, c.workers, c.worlds
	subDone, subErr := c.subDone, c.subErr
	c.mu.Unlock()

	if !running {
		return runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{"state": "stopped"}}
	}
	status := runtime.StatusOK
	details := map[string]any{}
	perWorld := make(map[string]any, len(worlds))
	for _, world := range worlds {
		worldStatus, section := workers[world].applier.WorldHealth()
		switch {
		case worldStatus == runtime.StatusFail:
			status = runtime.StatusFail
		case worldStatus == runtime.StatusDegraded && status == runtime.StatusOK:
			status = runtime.StatusDegraded
		}
		perWorld[world] = section
	}
	details["worlds"] = perWorld
	select {
	case <-subDone:
		status = runtime.StatusFail
		details["subscription"] = "stopped"
		if subErr != nil {
			details["err"] = subErr.Error()
		}
	default:
	}
	return runtime.Status{Status: status, Details: details}
}

// Get returns a copy of one entity of a world, for a test or a caller in the
// same process that has to see what State decided. Consumers read the world
// off the bus (C-02).
func (c *Context) Get(worldID, id string) (*entity.Entity, bool) {
	c.mu.Lock()
	store := c.store
	c.mu.Unlock()
	return store.Get(worldID, id)
}

// Recovery is what the last recovery of a world found; false for a world the
// context has not built.
func (c *Context) Recovery(worldID string) (Recovery, bool) {
	c.mu.Lock()
	applier := c.appliers[worldID]
	c.mu.Unlock()
	if applier == nil {
		return Recovery{}, false
	}
	return applier.LastRecovery(), true
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

var _ runtime.Context = (*Context)(nil)
