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
	// recording is the journal read in replay mode; internal/replay (EPIC-002)
	// consumes it once it exists.
	recording string
	idSource  string
}

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

func parseServe(args []string, stderr io.Writer) (*serveOptions, error) {
	fs := flag.NewFlagSet("multiverse", flag.ContinueOnError)
	fs.SetOutput(stderr)
	contexts := fs.String("contexts", "", "comma separated contexts to run, or \"all\" ("+strings.Join(runtime.Names(), ",")+")")
	mode := fs.String("mode", string(runtime.ModeLive), "live|replay")
	bus := fs.String("bus", "kafka", "kafka|memory (memory only with --contexts=all)")
	recording := fs.String("recording", "", "path to the recorded journal read in replay mode")
	idSource := fs.String("id-source", "uuid", "uuid|sequence (sequence gives deterministic event ids)")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	opts := &serveOptions{
		mode:      runtime.Mode(*mode),
		bus:       *bus,
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
	if opts.mode != runtime.ModeLive && opts.mode != runtime.ModeReplay {
		return nil, fmt.Errorf("--mode must be live or replay, got %q", *mode)
	}
	switch opts.bus {
	case "kafka":
	case "memory":
		if len(opts.contexts) != 1 || opts.contexts[0] != runtime.All {
			return nil, errors.New("--bus=memory requires --contexts=all")
		}
	default:
		return nil, fmt.Errorf("--bus must be kafka or memory, got %q", *bus)
	}
	if opts.idSource != "uuid" && opts.idSource != "sequence" {
		return nil, fmt.Errorf("--id-source must be uuid or sequence, got %q", *idSource)
	}
	if opts.recording != "" && opts.mode != runtime.ModeReplay {
		return nil, errors.New("--recording is only used with --mode=replay")
	}
	return opts, nil
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
	log.Info("multiverse started",
		slog.String("version", version),
		slog.String("addr", srv.Addr),
		slog.String("mode", string(opts.mode)),
		slog.String("bus", opts.bus),
		slog.String("contexts", strings.Join(names, ",")))
	_, _ = fmt.Fprintf(stdout, "multiverse %s listening on %s, contexts: %s\n", version, srv.Addr, strings.Join(names, ","))

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
