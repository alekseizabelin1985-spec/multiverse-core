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
// snapshot.go); recovery, the state section of /health and the admin route are
// T-059.
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

// Group is the consumer group State reads system_events under.
//
// State reads through a subscription until recovery exists: ADR-011 p. 4 gives
// it a cursor of its own — the cursor of its snapshot, caught up by the journal
// — and that arrives with the snapshot and recovery (T-057, T-059). Until then
// the world lives in memory only, and a group that resumes where it stopped is
// the one reading that does not apply a proposal a second time after a restart.
const Group = "core.state"

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
	// to and snapshots into (§4.3, §4.9); nil keeps the worlds in memory only
	// and writes no snapshot. The buckets of a world are created by mvctl
	// world init (EnsureWorldBuckets), not here.
	Objects objstore.Client
	// SnapshotEvery is the number of applied facts per world between two
	// snapshots; zero reads MV_SNAPSHOT_EVERY_FACTS when the context starts,
	// below zero writes none on the count. It matters only with Objects.
	SnapshotEvery int
	// RulesVersion is the version of the rules of the process, recorded in
	// every pointer (§4.4).
	RulesVersion string
}

// Context is State as a context of the process (runtime.Context).
//
// Its subscription is a mediator of delivery (C-01 v1.6): it hands each
// proposal to the worker of its world, waits for the answer and returns the
// error of the worker to the bus. A worker answers with an error only when its
// world has stopped (ErrWorldStopped); an answer that does not go out is
// published again by the worker itself (Applier.Apply). The event goes into
// the window of the subscription only after the worker answered without an
// error; remembered earlier, a redelivery would stop here and never reach the
// worker.
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

// Start builds a worker per world and opens the subscription to system_events.
// It returns as soon as the subscription runs; it lives until Stop.
//
// The lifetime of the subscription is the context's own, not the one Start was
// given: Stop cancels it, before the process closes the bus (C-01 v1.7,
// ADR-023 p. 4).
//
// A context stopped once may be started again: the world, the windows of
// identifiers and a world stopped on a failure stay, so a proposal that comes
// again after the restart is recognised. It is refused while the run before is
// still ending — a Stop that ran out of its context leaves the subscription or a
// worker behind, and a second worker over the same world would be a second
// writer (review #1 of T-055, Mi-2). The process starts each context once; the
// second start is what a test of the shutdown needs.
func (c *Context) Start(ctx context.Context, deps runtime.Deps) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return errors.New("state: context already started")
	}
	if !c.previousRunEnded() {
		return errors.New("state: the run before has not ended yet: its subscription or a worker is still busy")
	}
	if deps.Bus == nil {
		return errors.New("state: no bus in deps")
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

	if c.appliers == nil {
		c.appliers = make(map[string]*Applier, len(worlds))
	}
	workers := make(map[string]*worker, len(worlds))
	for _, world := range worlds {
		applier, ok := c.appliers[world]
		if !ok {
			applier, err = NewApplier(ApplierConfig{
				WorldID: world, Store: c.store, Publisher: deps.Bus,
				DedupCapacity: c.cfg.DedupCapacity, Timers: timers, Log: log,
				Invariants: c.cfg.Invariants,
				Objects:    objects, Clock: deps.Clock, SnapshotEvery: every,
				RulesVersion: c.cfg.RulesVersion,
			})
			if err != nil {
				return err
			}
			c.appliers[world] = applier
		}
		w := newWorker(world, applier, log)
		w.start()
		workers[world] = w
	}

	subCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	c.workers, c.worlds, c.cancel, c.subDone, c.subErr, c.running = workers, worlds, cancel, done, nil, true
	c.runLog = log
	go func() {
		defer close(done)
		err := deps.Bus.Subscribe(subCtx, eventbus.TopicSystemEvents, Group, c.dispatch)
		if err != nil {
			log.Error("subscription to system_events stopped", "err", err, "handled", false)
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
	log.Info("state running", "worlds", strings.Join(worlds, ","), "group", Group)
	return nil
}

// previousRunEnded reports whether the subscription and every worker of the run
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

// dispatch is the handler of the subscription: the mediator between the bus
// and the workers (C-01 v1.6).
func (c *Context) dispatch(ctx context.Context, ev eventbus.Event) error {
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

// Stop cancels the subscription, waits until its handler has returned — the
// proposal a worker holds is finished first — and stops the workers. The wait
// is bounded by ctx, which the process gives runtime.StopTimeout (C-01 v1.7).
//
// The cancellation also ends the attempts of a worker to publish an answer that
// does not go out: its world stops with publish_failed, and the proposal is left
// uncommitted rather than parked in dead_letters (Delivery.Deliver on a
// cancelled context).
//
// A subscription that ended on its own before Stop is what Stop reports.
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
		return fmt.Errorf("state: stop: the subscription to %s did not return: %w", eventbus.TopicSystemEvents, ctx.Err())
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
	// The shutdown snapshot (§4.9): the subscription has returned and the
	// workers have ended, so nothing changes the worlds any more, and the bus
	// is still open for snapshot.created — the process closes it after Stop
	// (ADR-023 p. 4).
	if c.cfg.Objects != nil {
		for _, world := range stopped {
			applier := workers[world].applier
			if applier.Stopped() != nil {
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
		return nil, errors.New("state: snapshot: the context is not running")
	case w == nil:
		return nil, fmt.Errorf("state: snapshot: %s is not a world of this context", worldID)
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

// Health is what /health of the process reports for State: fail when a world
// stopped — after a panic or an answer abandoned before it got out — or the
// subscription ended on its own; degraded before Start, after Stop and while a
// world publishes an answer again; ok otherwise.
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
		section := map[string]any{"entities": c.store.Len(world), "status": runtime.StatusOK}
		applier := workers[world].applier
		last, snapshotErr := applier.SnapshotHealth()
		if last != nil {
			section["snapshot"] = map[string]any{"seq": last.Snapshot.Seq, "taken_at": last.Snapshot.TakenAt}
		}
		if err := applier.Stopped(); err != nil {
			section["status"], section["err"], section["reason"] = runtime.StatusFail, err.Error(), stopReason(err)
			status = runtime.StatusFail
		} else if attempts := applier.Retrying(); attempts > 0 {
			section["status"], section["publish_attempts_failed"] = runtime.StatusDegraded, attempts
			if status == runtime.StatusOK {
				status = runtime.StatusDegraded
			}
		} else if snapshotErr != nil {
			// §9, "Ошибка записи снапшота": the world goes on, the snapshot is
			// stale until the next trigger writes one.
			section["status"], section["snapshot_stale"] = runtime.StatusDegraded, snapshotErr.Error()
			if status == runtime.StatusOK {
				status = runtime.StatusDegraded
			}
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
	return c.store.Get(worldID, id)
}

// stopReason names why a world stopped, for /health.
func stopReason(err error) string {
	switch {
	case errors.Is(err, ErrPublishFailed):
		return "publish_failed"
	case errors.Is(err, ErrPersistFailed):
		return "persist_failed"
	case errors.Is(err, errPanicked):
		return "panic"
	default:
		return "stopped"
	}
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
