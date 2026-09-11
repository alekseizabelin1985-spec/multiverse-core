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

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
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
	// recording is the journal read in replay mode; internal/replay (EPIC-002)
	// consumes it once it exists.
	recording string
	idSource  string
}

// retiredBus is the value MV_BUS used to declare while --bus accepted kafka and
// memory: two dictionaries with nothing in common. It is refused by name rather
// than mapped onto kafka, because a synonym kept alive is the second dictionary
// again — this time with a warning nobody reads attached to it (T-408).
const retiredBus = "redpanda"

func runServe(args []string, stdout, stderr io.Writer) int {
	opts, err := parseServe(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		_, _ = fmt.Fprintln(stderr, "multiverse:", err)
		return 2
	}
	if err := serve(*opts, stdout, stderr); err != nil {
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
	contexts := fs.String("contexts", "", "comma separated contexts to run, or \"all\" ("+strings.Join(runtime.Names(), ",")+")")
	mode := fs.String("mode", env.Mode.String(),
		joinOr(env.Mode.Enum())+"; overrides "+env.Mode.Name())
	bus := fs.String("bus", env.Bus.String(),
		joinOr(env.Bus.Enum())+" (memory only with --contexts=all); overrides "+env.Bus.Name())
	recording := fs.String("recording", "", "path to the recorded journal read in replay mode")
	idSource := fs.String("id-source", "uuid", "uuid|sequence (sequence gives deterministic event ids)")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
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

	log := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	deps := runtime.Deps{
		Mode: opts.mode,
		IDs:  idGenerator(opts.idSource),
		Log:  log,
	}
	// In replay mode time comes from the journal; internal/replay (EPIC-002)
	// injects EventClock and NullTimers. Until then replay uses a manual clock
	// that nothing advances, which is enough for the empty contexts.
	if opts.mode == runtime.ModeReplay {
		manual := clock.NewManual(time.Time{})
		deps.Clock, deps.Timers = manual, manual.Timers()
	} else {
		deps.Clock, deps.Timers = clock.Real{}, clock.RealTimers{}
	}

	srv := runtime.NewHTTP(env.CoreAddr.String(), runtime.Aggregate(contexts))
	deps.Mux = srv.Mux

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
	log.Info("multiverse started",
		slog.String("version", version),
		slog.String("addr", srv.Addr),
		slog.String("mode", string(opts.mode)),
		slog.String("mode_from", opts.modeFrom),
		slog.String("bus", opts.bus),
		slog.String("bus_from", opts.busFrom),
		slog.String("contexts", strings.Join(names, ",")))
	_, _ = fmt.Fprintf(stdout, "multiverse %s listening on %s, contexts: %s, mode: %s (%s), bus: %s (%s)\n",
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
	stop()

	stopCtx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	if err := srv.Stop(stopCtx); err != nil {
		log.Error("http shutdown", slog.String("error", err.Error()))
	}
	for _, err := range runtime.StopAll(stopCtx, contexts) {
		log.Error("stop", slog.String("error", err.Error()))
	}
	return serveErr
}

func shutdown(contexts []runtime.Context, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	for _, err := range runtime.StopAll(ctx, contexts) {
		log.Error("stop", slog.String("error", err.Error()))
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
