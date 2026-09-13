package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"
)

// StopTimeout is the budget for stopping every context of the process.
const StopTimeout = 15 * time.Second

// StartAll mounts the routes of contexts that ask for them and starts the
// contexts in the given order. On the first failure the already started
// contexts are stopped in reverse order, through StopAll with deps.Log, before
// the error is returned.
//
// A panic in Routes or Start is such a failure too. Left to unwind, it would
// run the deferred Close of the bus in the caller while the contexts started
// before it are still up — the order of ADR-023 p. 4 turned upside down on the
// one path nobody tests by hand (review #1 of T-410, N-1; T-415). The error
// then reads "start context <name>: panic: <value>" and the stack goes to
// deps.Log.
//
// The process prints the error of a failed start as one line of stderr. A
// panic can never break that line (below); an error a context returns itself
// goes in as it is. The errors of the rollback are appended to it as text,
// separated by "; " —
//
//	start context <name>: <cause>; stop context <name>: <cause>
//
// — rather than joined with errors.Join, whose parts are separated by a line
// break. The chain (errors.Is, errors.As) is the one of the start error; the
// rollback errors are not in it. Line breaks in the value of a panic, in Start
// and in Stop alike, are escaped as \n and \r, so no panic value can break the
// line (C-01 v1.6, T-430).
func StartAll(ctx context.Context, contexts []Context, deps Deps) error {
	for i, c := range contexts {
		if err := start(ctx, c, deps); err != nil {
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), StopTimeout)
			stopErrs := StopAll(stopCtx, contexts[:i], deps.Log)
			cancel()
			var rollback strings.Builder
			for _, stopErr := range stopErrs {
				rollback.WriteString("; ")
				rollback.WriteString(stopErr.Error())
			}
			return fmt.Errorf("start context %s: %w%s", c.Name(), err, rollback.String())
		}
	}
	return nil
}

// start mounts the routes of one context and starts it, turning a panic into
// an error.
func start(ctx context.Context, c Context, deps Deps) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = recovered(c, r, deps.Log, "context panicked while starting")
		}
	}()
	if r, ok := c.(Routes); ok && deps.Mux != nil {
		r.Routes(deps.Mux)
	}
	return c.Start(ctx, deps)
}

// stop stops one context, turning a panic into an error.
func stop(ctx context.Context, c Context, log *slog.Logger) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = recovered(c, r, log, "context panicked while stopping")
		}
	}()
	return c.Stop(ctx)
}

// recovered turns the panic value r of context c into an error and logs it.
// It must be called from the deferred function itself: the stack is taken
// there, the only place where it still shows the frames that panicked. The
// error carries the value alone, escaped to one line, because it ends up in a
// line of stderr; the log gets the value as it is, with the stack.
func recovered(c Context, r any, log *slog.Logger, msg string) error {
	text := panicText(r)
	if log != nil {
		log.Error(msg,
			slog.String("context", c.Name()),
			slog.String("panic", text),
			slog.String("stack", string(debug.Stack())))
	}
	return fmt.Errorf("panic: %s", oneLine.Replace(text))
}

// oneLine escapes the line breaks of a panic value, so that the error that
// carries it stays one line of stderr whatever the value is.
var oneLine = strings.NewReplacer("\n", `\n`, "\r", `\r`)

// panicText prints a panic value without letting a second panic out. The value
// is whatever the context panicked with, so its String or Error method may
// panic in turn, and fmt re-panics when that panic cannot be printed either;
// the type name is what is left to say then (the same guard as
// eventbus.panicText, review #1 of T-426, Mi-1).
func panicText(value any) (text string) {
	defer func() {
		if recover() != nil {
			text = fmt.Sprintf("%T (printing the value panicked)", value)
		}
	}()
	return fmt.Sprint(value)
}

// StopAll stops the contexts in reverse order and returns the errors met, one
// per context that failed, each reading "stop context <name>: <cause>".
//
// A panic in Stop is such an error too: "stop context <name>: panic: <value>",
// with the line breaks of the value escaped as \n and \r, and the stack logged
// at Error with the fields context, panic (the value as it is) and stack. A nil
// log logs nothing. The contexts after it in the reverse order still stop. Left
// to unwind, the panic would cut their Stop short and run the deferred Close of
// the bus in the caller under them — the order of ADR-023 p. 4 that the process
// keeps by closing the bus only after StopAll returns (C-01 v1.6, T-430).
func StopAll(ctx context.Context, contexts []Context, log *slog.Logger) []error {
	var errs []error
	for i := len(contexts) - 1; i >= 0; i-- {
		if err := stop(ctx, contexts[i], log); err != nil {
			errs = append(errs, fmt.Errorf("stop context %s: %w", contexts[i].Name(), err))
		}
	}
	return errs
}

// Aggregate reports the health of the process: ok when every context is ok,
// fail when any context fails, degraded otherwise. Per-context statuses are
// reported under details.contexts.
func Aggregate(contexts []Context) func() Status {
	return func() Status {
		byName := make(map[string]Status, len(contexts))
		overall := StatusOK
		for _, c := range contexts {
			s := c.Health()
			byName[c.Name()] = s
			switch s.Status {
			case StatusFail:
				overall = StatusFail
			case StatusDegraded:
				if overall != StatusFail {
					overall = StatusDegraded
				}
			}
		}
		return Status{Status: overall, Details: map[string]any{"contexts": byName}}
	}
}
