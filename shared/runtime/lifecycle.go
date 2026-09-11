package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

// StopTimeout is the budget for stopping every context of the process.
const StopTimeout = 15 * time.Second

// StartAll mounts the routes of contexts that ask for them and starts the
// contexts in the given order. On the first failure the already started
// contexts are stopped in reverse order before the error is returned.
//
// A panic in Routes or Start is such a failure too. Left to unwind, it would
// run the deferred Close of the bus in the caller while the contexts started
// before it are still up — the order of ADR-023 p. 4 turned upside down on the
// one path nobody tests by hand (review #1 of T-410, N-1; T-415).
func StartAll(ctx context.Context, contexts []Context, deps Deps) error {
	for i, c := range contexts {
		if err := start(ctx, c, deps); err != nil {
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), StopTimeout)
			StopAll(stopCtx, contexts[:i])
			cancel()
			return fmt.Errorf("start context %s: %w", c.Name(), err)
		}
	}
	return nil
}

// start mounts the routes of one context and starts it, turning a panic into
// an error. The stack is logged from inside the deferred function, the only
// place where it still shows the frames that panicked; the error carries the
// value alone, because it ends up in a line of stderr.
func start(ctx context.Context, c Context, deps Deps) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		text := panicText(r)
		err = fmt.Errorf("panic: %s", text)
		if deps.Log != nil {
			deps.Log.Error("context panicked while starting",
				slog.String("context", c.Name()),
				slog.String("panic", text),
				slog.String("stack", string(debug.Stack())))
		}
	}()
	if r, ok := c.(Routes); ok && deps.Mux != nil {
		r.Routes(deps.Mux)
	}
	return c.Start(ctx, deps)
}

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

// StopAll stops the contexts in reverse order and returns the errors met.
func StopAll(ctx context.Context, contexts []Context) []error {
	var errs []error
	for i := len(contexts) - 1; i >= 0; i-- {
		if err := contexts[i].Stop(ctx); err != nil {
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
