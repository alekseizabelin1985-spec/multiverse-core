package world

import (
	"context"
	"errors"
	"fmt"
	"io"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
)

// initOverKafka is `mvctl world init --bus kafka` (state-and-mechanics.md
// §4.10, the path of --bus kafka): the world is created in the object store of
// the deployment by the State of the running core. The command creates the
// buckets, proposes over Redpanda the fixture entities that have no object in
// entities-{world} yet, waits for the answers in system_events and asks the
// admin route of State for the snapshot seq 0 with reason bootstrap.
//
// The rule book is not read: the State of core holds the world to its own.
func (c Command) initOverKafka(a initArgs, stdout, stderr io.Writer) int {
	command := initCommand
	entities, snapshots := objstore.EntitiesBucket(a.world), objstore.SnapshotsBucket(a.world)
	if a.force {
		// The running State holds the world in memory: a clean-up of the store
		// from outside does not reset it, and the world it then snapshots is
		// the old one over a store that no longer has it (§4.10 (c)).
		return usage(stderr, command,
			"--force is not taken with --bus kafka: the running core holds the world in memory, and removing its "+
				"objects from outside does not reset it. To create the world %s again: stop core, remove the buckets "+
				"%s and %s, start core (the world is uninitialized) and run mvctl world init --bus kafka: nothing was done",
			a.world, entities, snapshots)
	}
	storeKind := a.store
	if storeKind == "" {
		storeKind = StoreMinIO
	}
	switch storeKind {
	case StoreMinIO:
	case StoreMemory:
		return usage(stderr, command,
			"--bus kafka takes --store minio only, not memory: the State of core writes the world into the object "+
				"store of the deployment, and a memory store of this command is a store nobody else reads: nothing was done")
	default:
		return usage(stderr, command, "unknown store %q, expected %s or %s", storeKind, StoreMinIO, StoreMemory)
	}
	if c.CoreURL == "" {
		return usage(stderr, command, "--bus kafka needs the address of core: set %s: nothing was done", env.CoreURL.Name())
	}

	server := newCore(c.CoreURL)
	report := cli.NewReport(command)
	result := InitResult{World: a.world, Bus: a.bus, Store: storeKind, Core: server.display}
	report.Details = &result
	fail := func(check, subject string, err error) int {
		report.Add(check, subject, err.Error())
		// What core did not say is still said on a failure: a bootstrap that
		// timed out reads differently next to "no context state in /health".
		if !a.asJSON {
			for _, warning := range result.Warnings {
				_, _ = fmt.Fprintf(stderr, "mvctl %s: warning: %s\n", command, warning)
			}
		}
		return report.Write(stdout, stderr, a.asJSON)
	}
	if _, err := state.LoadFixtures(a.world, a.fixtures); err != nil {
		return fail(CheckFixtures, a.fixtures, err)
	}
	client, err := c.OpenStore(storeKind)
	if err != nil {
		return fail(CheckStore, storeKind, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()
	if err := state.EnsureWorldBuckets(ctx, client, a.world); err != nil {
		return fail(CheckStore, storeKind, err)
	}
	objects := state.NewObjectStore(client)
	pointer, err := objects.ReadLatest(ctx, a.world)
	switch {
	case err == nil:
		_, _ = fmt.Fprintf(stderr,
			"mvctl %s: the world %s is initialized: %s/%s points at snapshot %d (%s); to initialize it again stop core, "+
				"remove the buckets %s and %s, start core and run mvctl world init --bus kafka\n",
			command, a.world, snapshots, state.PointerKey, pointer.Snapshot.Seq, pointer.Snapshot.Reason, entities, snapshots)
		return cli.ExitUsage
	case errors.Is(err, state.ErrUndecodable):
		return fail(CheckStore, storeKind, fmt.Errorf("%w; the world is not created over it: to create it again stop core, "+
			"remove the buckets %s and %s, start core and run mvctl world init --bus kafka", err, entities, snapshots))
	case !errors.Is(err, state.ErrNoSnapshot):
		return fail(CheckStore, storeKind, err)
	}

	stateSeen, err := servesTheWorld(ctx, server, a.world)
	if err != nil {
		return fail(CheckCore, server.display, err)
	}
	if !stateSeen {
		result.Warnings = append(result.Warnings, errNoContextState(server).Error())
	}
	if c.OpenBus == nil {
		return fail(CheckBus, BusKafka, errors.New("no transport to open"))
	}
	reg := contracts.Default()
	bus, err := c.OpenBus(reg)
	if err != nil {
		return fail(CheckBus, BusKafka, err)
	}
	defer func() { _ = bus.Close() }()
	deps := runtime.Deps{
		Bus: bus, Journal: bus, Contracts: reg,
		Clock: clock.Real{}, Timers: clock.RealTimers{}, Mode: runtime.ModeLive, Log: discard(),
	}
	result.Bootstrap, err = state.Bootstrap(ctx, deps, a.world, a.fixtures, state.WithObjects(objects))
	if err != nil {
		result.Refusal = refusalIn(err)
		return fail(CheckBootstrap, a.world, err)
	}
	if stateSeen {
		if err := holdsTheWorld(ctx, server, a.world, result.Bootstrap); err != nil {
			return fail(CheckCore, server.display, err)
		}
	}

	written, err := server.snapshot(ctx, a.world)
	snapshot, warning, failure := snapshotOutcome(written, err)
	if failure != nil {
		return fail(CheckSnapshot, a.world, failure)
	}
	result.Snapshot = snapshot
	if warning != nil {
		result.Warnings = append(result.Warnings, warning.Error())
	}
	for _, w := range afterTheSnapshot(ctx, server, a.world, snapshot) {
		result.Warnings = append(result.Warnings, w.Error())
	}
	return writeInitialized(report, &result, storeLine(storeKind)+", written by the State of core at "+server.display,
		stdout, stderr, a.asJSON)
}

// servesTheWorld asks /health of core whether its State serves the world
// before anything is proposed: without it every proposal would wait
// state.BootstrapTimeout for an answer nobody gives, and stay in the journal.
// A health that says nothing of State is not a refusal — the answers decide —
// but seen is false, and the command says so.
func servesTheWorld(ctx context.Context, server core, worldID string) (seen bool, err error) {
	health, err := server.health(ctx)
	if err != nil {
		return false, err
	}
	section, stateStatus, seen := worldOfHealth(health, worldID)
	switch {
	case !seen:
		return false, nil
	case stateStatus.Details["state"] == "stopped":
		return true, fmt.Errorf("the context state of core at %s is not running", server.display)
	case section == nil && stateStatus.Details["worlds"] != nil:
		return true, fmt.Errorf("the State of core at %s does not serve the world %s (%s)", server.display, worldID, env.StateWorlds.Name())
	case section["status"] == runtime.StatusFail:
		return true, fmt.Errorf("the world %s is stopped in core at %s (%v): State answers none of its proposals",
			worldID, server.display, section["reason"])
	}
	return true, nil
}

// errNoContextState is a health of core without the context state.
func errNoContextState(server core) error {
	return fmt.Errorf("core at %s reported no context state in /health: whether its State serves the world is not known, "+
		"and its world is not checked against the store", server.display)
}

// holdsTheWorld checks, after the bootstrap and before the snapshot, that the
// State of core holds every entity the store has (review #1 of T-475, Mi-1).
// What is proposed is decided by the objects (§4.10 (b)), and the snapshot is
// written from the memory of State: objects put into the store under a running
// State, without a restart, are objects it does not hold, and the snapshot seq
// 0 would be a part of the world. A world still uninitialized is the same case
// with none held. Nothing is written then.
func holdsTheWorld(ctx context.Context, server core, worldID string, bootstrap state.BootstrapResult) error {
	health, err := server.health(ctx)
	if err != nil {
		return fmt.Errorf("the world held by State is not known before the snapshot: %w", err)
	}
	section, _, _ := worldOfHealth(health, worldID)
	want := len(bootstrap.Created) + len(bootstrap.Skipped)
	held, counted := section["entities"].(float64)
	uninitialized := section["world"] == "uninitialized"
	if counted && int(held) >= want && !uninitialized {
		return nil
	}
	holds := fmt.Sprintf("%d of the %d entities of the store", int(held), want)
	switch {
	case !counted:
		holds = fmt.Sprintf("no count of the world %s in /health, the store has %d entities", worldID, want)
	case uninitialized:
		holds += ", the world still uninitialized"
	}
	return fmt.Errorf("the State of core at %s holds %s: restart core (the world is recovered from its objects) "+
		"and run mvctl world init --bus kafka again; no snapshot was asked for", server.display, holds)
}

// afterTheSnapshot is what went wrong once latest.json was written: a
// snapshot.created that did not go out, which /health of core says
// (snapshot_event_failed, §19 p. 4), and a snapshot with another reason than
// bootstrap — a core whose admin route does not read the reason of the request.
func afterTheSnapshot(ctx context.Context, server core, worldID string, snapshot *state.SnapshotMeta) []error {
	var warnings []error
	if snapshot.Reason != state.SnapshotBootstrap {
		warnings = append(warnings, fmt.Errorf("core wrote the snapshot with reason %s, not %s: its admin route does not "+
			"take the reason of the request", snapshot.Reason, state.SnapshotBootstrap))
	}
	health, err := server.health(ctx)
	if err != nil {
		return append(warnings, fmt.Errorf("whether snapshot.created went out is not known: %w", err))
	}
	if section, _, _ := worldOfHealth(health, worldID); section["snapshot_event_failed"] == true {
		warnings = append(warnings, fmt.Errorf("snapshot.created of snapshot %s did not go out "+
			"(/health of core: snapshot_event_failed); readers of the read-model see the snapshot by latest.json", snapshot.ID))
	}
	return warnings
}
