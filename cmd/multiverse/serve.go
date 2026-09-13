package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"

	"multiverse-core.io/internal/replay"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
)

type serveOptions struct {
	contexts []string
	mode     runtime.Mode
	bus      string
	// modeFrom and busFrom name what decided the two values — "MV_MODE" or
	// "--mode". They exist so that a refusal points at the thing the operator
	// actually set and so that the start line says which one won: a setting
	// that is applied silently is indistinguishable from one that is ignored
	// silently, and the second is what T-408 was opened for.
	modeFrom string
	busFrom  string
	// recording is the recorded session of replay mode. The process reads it
	// at start: an unreadable one refuses the start, and the clock of the
	// contexts starts at its earliest event (timeOf).
	recording string
	idSource  string
}

// retiredBus is the value MV_BUS used to declare while --bus accepted kafka and
// memory: two dictionaries with nothing in common. It is refused by name rather
// than mapped onto kafka, because a synonym kept alive is the second dictionary
// again — this time with a warning nobody reads attached to it (T-408).
const retiredBus = "redpanda"

// startFunc runs the process once its options are resolved; serve in the
// binary, a stand-in in the tests of dispatch.
type startFunc func(opts serveOptions, stdout, stderr io.Writer) error

func runServe(args []string, stdout, stderr io.Writer, start startFunc) int {
	opts, err := parseServe(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		_, _ = fmt.Fprintln(stderr, "multiverse:", err)
		return 2
	}
	if err := start(*opts, stdout, stderr); err != nil {
		_, _ = fmt.Fprintln(stderr, "multiverse:", err)
		return 1
	}
	return 0
}

// parseServe resolves the options of the serve subcommand.
//
// The mode and the bus have ONE source, the environment manifest, and the flags
// only override it: their default is env.Mode / env.Bus, so an explicitly
// passed --mode or --bus wins for whoever is at a terminal, and everything else
// — compose above all, which passes no such flag — obeys the manifest. Before
// T-408 the two flags carried defaults written here as literals, the manifest
// was read by nobody, and MV_MODE=replay in a .env produced a live process
// without a word said. The accepted values are taken from the manifest too
// (Enum), so the flag cannot grow a dictionary of its own again.
func parseServe(args []string, stderr io.Writer) (*serveOptions, error) {
	fs := flag.NewFlagSet("multiverse", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// The help of the bare binary is the help of serve, so it is the one place
	// an operator who types "multiverse -h" learns that the other subcommands
	// exist. The list is the one dispatch refuses with, not a copy (T-414).
	fs.Usage = func() {
		others := make([]string, 0, len(subcommands))
		for _, name := range subcommands {
			if name != "serve" {
				others = append(others, name)
			}
		}
		_, _ = fmt.Fprintf(fs.Output(), "usage: multiverse [serve] [flags] | %s\n\nflags of serve:\n",
			strings.Join(others, " | "))
		fs.PrintDefaults()
	}
	contexts := fs.String("contexts", "", "comma separated contexts to run, or \"all\" ("+strings.Join(runtime.Names(), ",")+")")
	mode := fs.String("mode", env.Mode.String(),
		joinOr(env.Mode.Enum())+"; overrides "+env.Mode.Name())
	bus := fs.String("bus", env.Bus.String(),
		joinOr(env.Bus.Enum())+" (memory only with --contexts=all); overrides "+env.Bus.Name())
	recording := fs.String("recording", "", "recorded session (JSONL) of replay mode: the clock of the contexts "+
		"starts at its earliest event; its events are not fed to the bus")
	idSource := fs.String("id-source", "uuid", "uuid|sequence (sequence gives deterministic event ids)")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		// Parsing stops at the first word that is not a flag, so a subcommand
		// written after the flags lands here rather than in dispatch.
		for _, name := range subcommands {
			if fs.Arg(0) == name {
				return nil, fmt.Errorf("unexpected argument %q: the subcommand goes first: multiverse %s <flags>",
					name, name)
			}
		}
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	passed := passedFlags(fs)
	opts := &serveOptions{
		mode:      runtime.Mode(*mode),
		bus:       *bus,
		modeFrom:  origin(passed, "mode", env.Mode),
		busFrom:   origin(passed, "bus", env.Bus),
		recording: *recording,
		idSource:  *idSource,
	}
	if *contexts == "" {
		return nil, errors.New("--contexts is required")
	}
	for _, name := range strings.Split(*contexts, ",") {
		if name = strings.TrimSpace(name); name != "" {
			opts.contexts = append(opts.contexts, name)
		}
	}
	if len(opts.contexts) == 0 {
		return nil, errors.New("--contexts is required")
	}
	if !declares(env.Mode, *mode) {
		return nil, fmt.Errorf("%s=%q: expected %s", opts.modeFrom, *mode, joinOr(env.Mode.Enum()))
	}
	// The retired value is named before the enum is consulted, or the operator
	// whose .env says redpanda would be told "expected kafka or memory" and
	// left to guess which of the two his broker is (T-408).
	if opts.bus == retiredBus {
		return nil, fmt.Errorf("%s=%q: %q is the name of the broker product, and the transport is named by "+
			"its protocol — write %q. The compose broker is Redpanda and it speaks the Kafka API; the "+
			"brokers themselves live in %s (T-408)",
			opts.busFrom, retiredBus, retiredBus, "kafka", env.KafkaBrokers.Name())
	}
	if !declares(env.Bus, opts.bus) {
		return nil, fmt.Errorf("%s=%q: expected %s", opts.busFrom, opts.bus, joinOr(env.Bus.Enum()))
	}
	if opts.bus == busMemory && (len(opts.contexts) != 1 || opts.contexts[0] != runtime.All) {
		return nil, fmt.Errorf("%s=%s requires --contexts=all, got %q",
			opts.busFrom, busMemory, strings.Join(opts.contexts, ","))
	}
	if opts.idSource != "uuid" && opts.idSource != "sequence" {
		return nil, fmt.Errorf("--id-source must be uuid or sequence, got %q", *idSource)
	}
	if opts.recording != "" && opts.mode != runtime.ModeReplay {
		return nil, fmt.Errorf("--recording is only used with mode %s, and %s says %q",
			runtime.ModeReplay, opts.modeFrom, *mode)
	}
	return opts, nil
}

// busMemory is the in-process bus; it is a value of the manifest enum, not a
// name of its own.
const busMemory = "memory"

// passedFlags returns the flags the command line actually carried. flag.Visit
// walks exactly those, which is what separates "the developer asked for this"
// from "this is what the manifest says".
func passedFlags(fs *flag.FlagSet) map[string]bool {
	passed := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { passed[f.Name] = true })
	return passed
}

// origin names what decided a value: the flag when it was passed, otherwise the
// variable whose value became the flag's default.
func origin(passed map[string]bool, name string, v env.Var) string {
	if passed[name] {
		return "--" + name
	}
	return v.Name()
}

// declares reports whether value is one the manifest allows for v.
func declares(v env.Var, value string) bool {
	for _, allowed := range v.Enum() {
		if allowed == value {
			return true
		}
	}
	return false
}

// joinOr renders a manifest enum the way an error message reads it.
func joinOr(values []string) string {
	switch len(values) {
	case 0:
		return "any value"
	case 1:
		return values[0]
	default:
		return strings.Join(values[:len(values)-1], ", ") + " or " + values[len(values)-1]
	}
}

func serve(opts serveOptions, stdout, stderr io.Writer) error {
	contexts, err := runtime.New(opts.contexts)
	if err != nil {
		return err
	}

	return withSignals(context.Background(), process{
		opts:     opts,
		contexts: contexts,
		openBus:  openBus,
		stdout:   stdout,
		log:      slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}.run)
}

// withSignals runs fn under a context that SIGINT and SIGTERM cancel, and hands
// fn the function that unregisters the two. fn calls it once its wait is over,
// so that a second signal during the shutdown reaches the default handler and
// kills the process instead of being swallowed. Unregistering also cancels
// ctx, which is how a test sees that fn got the real release and not a no-op —
// a signal cannot be sent to a test process on Windows.
func withSignals(parent context.Context, fn func(ctx context.Context, release func()) error) error {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return fn(ctx, stop)
}

// process is one run of serve after the options are resolved and the contexts
// built: what it owns (the bus, the HTTP server) and in which order it lets go
// of them. It is split from serve so that a test can drive a whole lifetime
// with contexts of its own and watch the transport close.
type process struct {
	opts     serveOptions
	contexts []runtime.Context
	openBus  openBusFunc
	stdout   io.Writer
	log      *slog.Logger
}

// run starts the contexts, serves until ctx is done or the HTTP server fails,
// and stops everything. release is called once the wait is over, so that a
// second signal during the shutdown reaches the default handler and kills the
// process instead of being swallowed.
//
// The process creates the bus and the journal and is their only closer. The
// order of the shutdown is HTTP server → contexts in reverse start order → bus,
// and the bus is last on every path out through a return, a failed start
// included (ADR-023 p. 4): a context is stopped through its own context by the
// runtime, not by the bus closing under it, and a context that still publishes
// or reads the journal in its Stop — a final snapshot pointer, a cursor — needs
// the bus open to do it.
//
// A panic in a Start is such a path as well: runtime.StartAll turns it into a
// failed start and stops the contexts started before it, so the deferred Close
// still runs last (review #1 of T-410, N-1; T-415). A panic in a Stop is
// recovered the same way by runtime.StopAll: the contexts after it in the
// reverse order still stop before the bus closes (T-430).
func (p process) run(ctx context.Context, release func()) error {
	log := p.log
	opts := p.opts
	deps := runtime.Deps{
		Mode:      opts.mode,
		IDs:       idGenerator(opts.idSource),
		Log:       log,
		Contracts: contracts.Default(),
	}
	times, err := timeOf(opts)
	if err != nil {
		return err
	}
	deps.Clock, deps.Timers = times.clock, times.timers
	// The sources of the event constructors are the objects of Deps: the id
	// generator --id-source chose, the clock of the contexts — the wall clock
	// in live mode, the EventClock in replay — and the registry the bus routes
	// by (C-01 v1.4 "Источники конструкторов", T-055). They are installed in
	// one place, before the bus is opened and before the first context starts,
	// and taken back in one place: deferred before closeBus, so they are taken
	// back after the bus has closed and a dead letter written during the
	// shutdown still gets the id and the time of this run.
	//
	// Taken back means reset to the defaults of eventbus, not to what stood
	// before the run: eventbus has no reader of its sources. A binary runs once
	// and loses nothing; a test that installed its own sources — with
	// testkit.Deterministic — has them replaced by the ones of the process for
	// the length of the run and reset after it. While the sources are process
	// wide, a test of this package that runs process.run must not call
	// t.Parallel: two runs would install their sources over each other.
	installSources(deps)
	defer installSources(runtime.Deps{})

	// The bus gets its own timers, real in every mode (C-01 v1.4 "Таймеры
	// повторной доставки"): the pauses between redeliveries are not domain
	// time, and a NullTimers pause would hang the first failing handler of a
	// replay until the process is cancelled (review #1 of T-410).
	bus, err := p.openBus(opts.bus, deps.Contracts, times.bus, log)
	if err != nil {
		return fmt.Errorf("bus %s (%s): %w", opts.bus, opts.busFrom, err)
	}
	// Deferred first, so it runs last: after the contexts are stopped on the
	// normal path, after StartAll has stopped the started ones on a failed
	// start, and after shutdown on a failed listen.
	defer closeBus(bus, log)
	if times.events != nil {
		bus = replay.WithMiddleware(bus, replay.Middleware(opts.mode, times.events))
	}
	deps.Bus, deps.Journal = bus, bus

	contexts := p.contexts
	srv := runtime.NewHTTP(env.CoreAddr.String(), runtime.Aggregate(contexts))
	deps.Mux = srv.Mux

	if err := runtime.StartAll(ctx, contexts, deps); err != nil {
		return err
	}
	if err := srv.Start(); err != nil {
		shutdown(contexts, log)
		return fmt.Errorf("listen %s: %w", srv.Addr, err)
	}
	names := make([]string, 0, len(contexts))
	for _, c := range contexts {
		names = append(names, c.Name())
	}
	// The source of each choice is part of the start line and of the start
	// record, not a detail: the defect this shape closes was invisible exactly
	// because a process that ignored MV_MODE looked the same as one that obeyed
	// it (T-408).
	attrs := []any{
		slog.String("version", version),
		slog.String("addr", srv.Addr),
		slog.String("mode", string(opts.mode)),
		slog.String("mode_from", opts.modeFrom),
		slog.String("bus", opts.bus),
		slog.String("bus_from", opts.busFrom),
		slog.String("contexts", strings.Join(names, ",")),
	}
	if opts.recording != "" {
		attrs = append(attrs, slog.String("recording", opts.recording), slog.Int("recorded_events", times.recorded))
	}
	log.Info("multiverse started", attrs...)
	if times.events != nil && opts.recording == "" {
		// Replay without a recording is allowed: a run on fakes needs none, and
		// MV_MODE=replay has no variable for one. But its clock starts at year
		// one and root events built before the first read carry that time, so
		// the run says so rather than pass for a replay of a session (T-408).
		log.Warn("replay without a recording",
			slog.Time("clock_start", times.start),
			slog.String("mode_from", opts.modeFrom))
	}
	_, _ = fmt.Fprintf(p.stdout, "multiverse %s listening on %s, contexts: %s, mode: %s (%s), bus: %s (%s)\n",
		version, srv.Addr, strings.Join(names, ","),
		opts.mode, opts.modeFrom, opts.bus, opts.busFrom)

	var serveErr error
	select {
	case <-ctx.Done():
	case err, ok := <-srv.Err():
		if ok {
			serveErr = err
		}
	}
	release()

	stopCtx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	if err := srv.Stop(stopCtx); err != nil {
		log.Error("http shutdown", slog.String("error", err.Error()))
	}
	for _, err := range runtime.StopAll(stopCtx, contexts, log) {
		log.Error("stop", slog.String("error", err.Error()))
	}
	return serveErr
}

// runTime is the time of one run: what the contexts get, what the bus gets, and
// the replay clock the bus middleware moves (nil in live mode).
type runTime struct {
	clock  clock.Clock
	timers clock.Timers
	bus    clock.Timers
	events *replay.EventClock
	// start is where the replay clock stood when the run was built. The start
	// record reports it rather than the clock itself: by then the contexts have
	// started, and a context that catches up on the journal in its Start has
	// already moved the clock (review of T-060 by tech-lead#1, Н-1).
	start    time.Time
	recorded int
}

// timeOf builds the time of a run. Live mode is the wall clock throughout.
// Replay gives the contexts an EventClock and NullTimers (state-and-mechanics.md
// §6.2), and the bus the same real timers as in live mode (C-01 v1.4). The
// replay clock starts at the earliest event of the recording, so that a context
// asking the time before it has read anything gets a time of the session
// rather than year one; without a recording it starts at the zero time.
func timeOf(opts serveOptions) (runTime, error) {
	if opts.mode != runtime.ModeReplay {
		return runTime{clock: clock.Real{}, timers: clock.RealTimers{}, bus: clock.RealTimers{}}, nil
	}
	var start time.Time
	var recorded int
	if opts.recording != "" {
		rec, err := replay.OpenRecording(opts.recording)
		if err != nil {
			return runTime{}, fmt.Errorf("--recording: %w", err)
		}
		start, _ = rec.Start()
		recorded = rec.Len()
	}
	events := replay.NewEventClock(start)
	return runTime{
		clock:    events,
		timers:   replay.NullTimers{},
		bus:      clock.RealTimers{},
		events:   events,
		start:    start,
		recorded: recorded,
	}, nil
}

// installSources installs the sources of the event constructors from deps. The
// zero Deps resets all three to the defaults of eventbus: UUIDs, the wall clock
// and no registry.
func installSources(deps runtime.Deps) {
	eventbus.SetIDSource(deps.IDs)
	eventbus.SetClock(deps.Clock)
	if deps.Contracts == nil {
		// A nil *contracts.Registry in the interface would not read as "no
		// registry" to eventbus.
		eventbus.SetRegistry(nil)
		return
	}
	eventbus.SetRegistry(deps.Contracts)
}

func shutdown(contexts []runtime.Context, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	for _, err := range runtime.StopAll(ctx, contexts, log) {
		log.Error("stop", slog.String("error", err.Error()))
	}
}

// closeBus closes the transport of the process. Its error is logged rather
// than returned: it comes after every context has already stopped, there is
// nothing left to undo, and it must not replace the error the run is ending
// with.
func closeBus(bus transport, log *slog.Logger) {
	if err := bus.Close(); err != nil {
		log.Error("bus close", slog.String("error", err.Error()))
	}
}

// idGenerator returns the event id source: real uuids in production, a
// reproducible sequence in tests and replay.
func idGenerator(source string) func() string {
	if source == "sequence" {
		var n atomic.Uint64
		return func() string { return "seq-" + strconv.FormatUint(n.Add(1), 10) }
	}
	return uuid.NewString
}
