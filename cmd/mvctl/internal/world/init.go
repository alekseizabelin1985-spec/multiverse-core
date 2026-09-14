package world

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
)

// DefaultFixtures is the fixtures directory of the repository, relative to its
// root, where an operator runs mvctl.
const DefaultFixtures = "testdata/fixtures"

// initTimeout bounds the whole run: the buckets, a bootstrap of the six
// entities of MVP-1 at state.BootstrapTimeout each at worst, and the snapshot
// at state.SnapshotTimeout. A store or a State that does not answer must fail
// the command rather than hang a pipeline.
const initTimeout = 2 * time.Minute

// InitResult is what `mvctl world init` did: the details of its JSON report.
type InitResult struct {
	World string `json:"world"`
	Bus   string `json:"bus"`
	Store string `json:"store"`
	// Core is the address of the core whose State created the world, on the
	// path of --bus kafka.
	Core      string                `json:"core,omitempty"`
	Bootstrap state.BootstrapResult `json:"bootstrap"`
	// Refusal is the refusal of State that ended the bootstrap, with its
	// details (invariant_id, expected_version, actual_version).
	Refusal  *state.Refusal      `json:"refusal,omitempty"`
	Snapshot *state.SnapshotMeta `json:"snapshot,omitempty"`
	// Warnings are what went wrong after latest.json was written: the world is
	// initialized all the same (state-and-mechanics.md §4.10, exit codes).
	Warnings []string `json:"warnings,omitempty"`
}

// initArgs are the flags of `mvctl world init` once parsed.
type initArgs struct {
	world, fixtures, bus, store, rules string
	force, asJSON                      bool
}

const initCommand = "world init"

// runInit implements `mvctl world init` (state-and-mechanics.md §4.10, "Правила
// `world init`"): the buckets of the world, a refusal over a world that already
// has latest.json, the bootstrap from the fixtures and the snapshot State writes
// of it — by a State inside the command over the memory bus, or by the State of
// the running core over Redpanda.
func (c Command) runInit(args []string, stdout, stderr io.Writer) int {
	command := initCommand
	flags := cli.FlagSet("mvctl "+command, stderr)
	worldID := flags.String("world", env.WorldID.String(), "world to create (default "+env.WorldID.Name()+")")
	fixtures := flags.String("fixtures", DefaultFixtures, "directory of the fixture entities")
	bus := flags.String("bus", env.Bus.String(),
		"bus State answers on: memory runs State inside this command, kafka asks the running core "+
			"(brokers "+env.KafkaBrokers.Name()+", core "+env.CoreURL.Name()+") (default "+env.Bus.Name()+")")
	store := flags.String("store", "",
		"object store; with --bus memory only memory (the default), whose objects are gone when the command ends; "+
			"with --bus kafka only minio (the default), the store of the deployment")
	rules := flags.String("rules", env.RulesPath.String(),
		"rule book the in-process State holds the world to; not read with --bus kafka, "+
			"where the running core holds the world to its own (default "+env.RulesPath.Name()+")")
	force := flags.Bool("force", false,
		"create the world again: the objects of State of the world are removed first (entities and state snapshots); "+
			"with --bus memory only")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}
	if code, ok := noArguments(stderr, command, flags.Args()); !ok {
		return code
	}
	if *worldID == "" {
		return usage(stderr, command, "no world: pass --world or set %s", env.WorldID.Name())
	}
	a := initArgs{world: *worldID, fixtures: *fixtures, bus: *bus, store: *store, rules: *rules,
		force: *force, asJSON: *asJSON}
	switch *bus {
	case BusMemory:
		return c.initInMemory(a, stdout, stderr)
	case BusKafka:
		return c.initOverKafka(a, stdout, stderr)
	default:
		return usage(stderr, command, "unknown bus %q, expected %s or %s", *bus, BusMemory, BusKafka)
	}
}

// initInMemory is `mvctl world init --bus memory`: State inside the command,
// over the memory bus and the memory store.
func (c Command) initInMemory(a initArgs, stdout, stderr io.Writer) int {
	command := initCommand
	storeKind := a.store
	if storeKind == "" {
		storeKind = StoreMemory
	}
	if storeKind != StoreMemory {
		// The cursor of the snapshot is an offset of the journal of membus, which
		// is gone with the command: a core over Redpanda would recover the world
		// from an offset of another journal (§4.8), and next to a running core
		// the State of this command would be a second writer of the world.
		return usage(stderr, command,
			"--bus memory takes --store memory only, not %q: the cursor of its snapshot is an offset of a journal "+
				"that is gone with the command, and next to a running core it would be a second writer of the world; "+
				"a world in the object store of a deployment is created with --bus kafka: nothing was done", storeKind)
	}

	report := cli.NewReport(command)
	book, err := mechanics.Load(a.rules)
	if err != nil {
		report.Add(CheckRules, a.rules, err.Error())
		return report.Write(stdout, stderr, a.asJSON)
	}
	if _, err := state.LoadFixtures(a.world, a.fixtures); err != nil {
		report.Add(CheckFixtures, a.fixtures, err.Error())
		return report.Write(stdout, stderr, a.asJSON)
	}
	client, err := c.OpenStore(storeKind)
	if err != nil {
		report.Add(CheckStore, storeKind, err.Error())
		return report.Write(stdout, stderr, a.asJSON)
	}

	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()
	if err := state.EnsureWorldBuckets(ctx, client, a.world); err != nil {
		report.Add(CheckStore, storeKind, err.Error())
		return report.Write(stdout, stderr, a.asJSON)
	}
	pointer, err := state.NewObjectStore(client).ReadLatest(ctx, a.world)
	switch {
	case err == nil && !a.force:
		_, _ = fmt.Fprintf(stderr,
			"mvctl %s: the world %s is initialized: %s/%s points at snapshot %d (%s); pass --force to initialize it again\n",
			command, a.world, objstore.SnapshotsBucket(a.world), state.PointerKey,
			pointer.Snapshot.Seq, pointer.Snapshot.Reason)
		return cli.ExitUsage
	case errors.Is(err, state.ErrUndecodable) && !a.force:
		// A pointer that does not read is still a pointer: the world may be
		// there, and creating it again is the operator's choice (§4.10).
		report.Addf(CheckStore, storeKind, "%v; the world is not created over it: pass --force to create it again", err)
		return report.Write(stdout, stderr, a.asJSON)
	case errors.Is(err, state.ErrUndecodable):
		// --force removes the pointer anyway, and reads nothing of it.
	case err != nil && !errors.Is(err, state.ErrNoSnapshot):
		report.Add(CheckStore, storeKind, err.Error())
		return report.Write(stdout, stderr, a.asJSON)
	}
	if a.force {
		if err := clearWorld(ctx, client, a.world); err != nil {
			report.Add(CheckStore, storeKind, err.Error())
			return report.Write(stdout, stderr, a.asJSON)
		}
	}

	result := InitResult{World: a.world, Bus: a.bus, Store: storeKind}
	run := initInProcess(ctx, client, a.world, a.fixtures, book)
	result.Bootstrap, result.Snapshot = run.bootstrap, run.snapshot
	for _, warning := range run.warnings {
		result.Warnings = append(result.Warnings, warning.Error())
	}
	report.Details = &result
	if run.err != nil {
		check := CheckBootstrap
		if errors.Is(run.err, errSnapshot) {
			check = CheckSnapshot
		}
		result.Refusal = refusalIn(run.err)
		report.Add(check, a.world, run.err.Error())
		return report.Write(stdout, stderr, a.asJSON)
	}
	return writeInitialized(report, &result, storeLine(storeKind), stdout, stderr, a.asJSON)
}

// writeInitialized reports a world whose latest.json is written: the lines of
// the snapshot, and the warnings of what went wrong after it.
func writeInitialized(report *cli.Report, result *InitResult, where string, stdout, stderr io.Writer, asJSON bool) int {
	snap, worldID := result.Snapshot, result.World
	report.Details = result
	report.Linef("world %s: entities created %d, skipped %d", worldID,
		len(result.Bootstrap.Created), len(result.Bootstrap.Skipped))
	report.Linef("snapshot: seq %d, reason %s, key %s/%s", snap.Seq, snap.Reason,
		objstore.SnapshotsBucket(worldID), snap.Key)
	report.Linef("entities_count: %d", snap.EntitiesCount)
	report.Linef("state_hash: %s", snap.StateHash)
	report.Linef("rules_version: %s", snap.RulesVersion)
	report.Linef("cursor.%s: %d", eventbus.TopicSystemEvents, snap.Cursor[eventbus.TopicSystemEvents])
	report.Line(where)
	report.Summary = "world " + worldID + " initialized in the " + result.Store + " store"
	// A warning does not change the exit code: latest.json is written, and a
	// second init would be refused as initialized (§4.10, exit codes). It goes
	// to stderr, next to the findings, so that it is not lost in the lines.
	if !asJSON {
		for _, warning := range result.Warnings {
			_, _ = fmt.Fprintf(stderr, "mvctl %s: warning: the world is initialized, but %s\n", initCommand, warning)
		}
	}
	return report.Write(stdout, stderr, asJSON)
}

// refusalIn is the refusal of State among the causes of err, or nil.
func refusalIn(err error) *state.Refusal {
	var refusal *state.Refusal
	if errors.As(err, &refusal) {
		return refusal
	}
	return nil
}

// storeLine says where the world is, for the reader of the text report.
func storeLine(kind string) string {
	if kind == StoreMemory {
		return "store: memory (its objects are gone when the command ends)"
	}
	return "store: " + kind
}

// clearWorld removes the objects of State of the world before --force creates it
// again (§4.10, "`--force`"): every object of entities-{world}, intents
// included, and every object of snapshots-{world}/state/, the pointer last.
// swarm/ and gateway/ of the snapshots bucket are not State's and stay.
//
// The pointer goes last so that a clean-up cut off halfway leaves it: the next
// init without --force refuses instead of creating a world over what is left.
func clearWorld(ctx context.Context, client objstore.Client, worldID string) error {
	entities := objstore.EntitiesBucket(worldID)
	objects, err := client.List(ctx, entities, "")
	if err != nil {
		return fmt.Errorf("--force: list %s: %w", entities, err)
	}
	for _, info := range objects {
		if err := client.Delete(ctx, entities, info.Key); err != nil {
			return fmt.Errorf("--force: delete %s/%s: %w", entities, info.Key, err)
		}
	}
	snapshots := objstore.SnapshotsBucket(worldID)
	objects, err = client.List(ctx, snapshots, state.Component+"/")
	if err != nil {
		return fmt.Errorf("--force: list %s/%s/: %w", snapshots, state.Component, err)
	}
	for _, info := range objects {
		if info.Key == state.PointerKey {
			continue
		}
		if err := client.Delete(ctx, snapshots, info.Key); err != nil {
			return fmt.Errorf("--force: delete %s/%s, the pointer stays: %w", snapshots, info.Key, err)
		}
	}
	if err := client.Delete(ctx, snapshots, state.PointerKey); err != nil {
		return fmt.Errorf("--force: delete %s/%s: %w", snapshots, state.PointerKey, err)
	}
	return nil
}

// errSnapshot marks the failure of the snapshot after a bootstrap that went
// through.
var errSnapshot = errors.New("snapshot")

// inProcessRun is what the State inside the command did. snapshot is set once
// latest.json is written; err is a failure before that, warnings are the
// failures after it.
type inProcessRun struct {
	bootstrap state.BootstrapResult
	snapshot  *state.SnapshotMeta
	warnings  []error
	err       error
}

// initInProcess runs State inside the command over the memory bus and the
// store, bootstraps the world through it and has it write the snapshot of the
// world with reason bootstrap (§4.10 p. 4, Context.Snapshot of T-057).
//
// The in-process State lives as long as the command and writes one snapshot,
// bootstrap (§4.10, "Снапшот `shutdown`"). Its Stop would write a snapshot of
// its own (§4.9, reason shutdown), moving latest.json off the bootstrap one: the
// store is sealed once the bootstrap snapshot is written — or the run has
// failed, so that latest.json of half a world does not refuse the next init —
// and the refusal of the sealed store is the one error of Stop the command
// does not show.
func initInProcess(ctx context.Context, client objstore.Client, worldID, fixtures string,
	book *mechanics.Rules) (run inProcessRun) {
	reg := contracts.Default()
	specs := reg.Topics()
	topics := make([]string, 0, len(specs))
	for _, t := range specs {
		topics = append(topics, t.Name)
	}
	timers := clock.RealTimers{}
	bus, err := membus.New(membus.Config{Registry: reg, Topics: topics, Timers: timers, Log: discard()})
	if err != nil {
		run.err = err
		return run
	}
	defer func() { _ = bus.Close() }()

	objects := &sealable{Client: client}
	world := state.New(state.Config{
		Worlds:        []string{worldID},
		Log:           discard(),
		Timers:        timers,
		Invariants:    book.Invariants(),
		Objects:       objects,
		SnapshotEvery: -1,
		RulesVersion:  book.Version,
	})
	deps := runtime.Deps{
		Bus: bus, Journal: bus, Contracts: reg,
		Clock: clock.Real{}, Timers: timers, Mode: runtime.ModeLive, Log: discard(),
	}
	if err := world.Start(ctx, deps); err != nil {
		run.err = err
		return run
	}
	// Whatever happens below, the store is sealed before Stop.
	defer func() {
		objects.seal()
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), runtime.StopTimeout)
		defer cancel()
		if err := unexpected(world.Stop(stopCtx)); err != nil {
			stopped := fmt.Errorf("stop the in-process state: %w", err)
			if run.snapshot != nil {
				run.warnings = append(run.warnings, stopped)
			} else {
				// A State that refused the bootstrap and then did not stop says
				// both (review #2 of T-058, N-4); errSnapshot stays in the chain.
				run.err = errors.Join(run.err, stopped)
			}
		}
	}()

	run.bootstrap, err = state.Bootstrap(ctx, deps, worldID, fixtures)
	if err != nil {
		run.err = withHealth(err, world)
		return run
	}
	var warning error
	run.snapshot, warning, run.err = snapshotOutcome(world.Snapshot(ctx, worldID, state.SnapshotBootstrap))
	if warning != nil {
		run.warnings = append(run.warnings, warning)
	}
	return run
}

// snapshotOutcome decides what a snapshot asked of State means for world init
// (§4.10, exit codes): a pointer is a written latest.json, and the world is
// initialized whatever else went wrong — a snapshot.created that did not go
// out is a warning. Without a pointer the error is the failure of the
// snapshot. Applier.Snapshot returns the pointer together with the error of an
// event that did not go out; the admin route answers 200 in that case and
// /health says snapshot_event_failed.
func snapshotOutcome(pointer *state.LatestPointer, err error) (snapshot *state.SnapshotMeta, warning, failure error) {
	if pointer != nil {
		meta := pointer.Snapshot
		return &meta, err, nil
	}
	if err == nil {
		err = errors.New("the State wrote no snapshot and said nothing")
	}
	return nil, nil, fmt.Errorf("%w: %w", errSnapshot, err)
}

// withHealth adds what State says of itself to a bootstrap that failed: a
// proposal left without an answer is explained by the world State stopped.
func withHealth(err error, world *state.Context) error {
	health := world.Health()
	return fmt.Errorf("%w (state health: %s %v)", err, health.Status, health.Details)
}

// errSealed is a write into the store after the bootstrap snapshot.
var errSealed = errors.New("world init: the store is sealed after the bootstrap snapshot")

// sealable is the object store of the in-process State: once sealed, it takes
// no more writes and removes nothing.
type sealable struct {
	objstore.Client
	sealed atomic.Bool
}

func (s *sealable) seal() { s.sealed.Store(true) }

func (s *sealable) Put(ctx context.Context, bucket, key string, body []byte, opts objstore.PutOptions) (string, error) {
	if s.sealed.Load() {
		return "", fmt.Errorf("%w: put %s/%s", errSealed, bucket, key)
	}
	return s.Client.Put(ctx, bucket, key, body, opts)
}

func (s *sealable) Delete(ctx context.Context, bucket, key string) error {
	if s.sealed.Load() {
		return fmt.Errorf("%w: delete %s/%s", errSealed, bucket, key)
	}
	return s.Client.Delete(ctx, bucket, key)
}

// unexpected is err without the refusals of the sealed store. An error it
// takes nothing out of comes back as it was, with its whole text.
func unexpected(err error) error {
	rest, _ := withoutSeal(err)
	return rest
}

// withoutSeal is err without the refusals of the sealed store, and whether it
// took anything out. Only a join is taken apart: an error that wraps several
// others with one format keeps its text unless the seal is among them.
func withoutSeal(err error) (error, bool) {
	if err == nil {
		return nil, false
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		if errors.Is(err, errSealed) {
			return nil, true
		}
		return err, false
	}
	var rest []error
	dropped := false
	for _, e := range joined.Unwrap() {
		kept, took := withoutSeal(e)
		dropped = dropped || took
		if kept != nil {
			rest = append(rest, kept)
		}
	}
	if !dropped {
		return err, false
	}
	return errors.Join(rest...), true
}

// discard is the logger of the in-process State and bus. The command reports
// what an operator acts on itself — a refusal, a proposal left without an
// answer with the health of State, a snapshot that was not written.
func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }
