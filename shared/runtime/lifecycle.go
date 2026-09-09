package runtime

import (
	"context"
	"fmt"
	"time"
)

// StopTimeout is the budget for stopping every context of the process.
const StopTimeout = 15 * time.Second

// StartAll mounts the routes of contexts that ask for them and starts the
// contexts in the given order. On the first failure the already started
// contexts are stopped in reverse order before the error is returned.
func StartAll(ctx context.Context, contexts []Context, deps Deps) error {
	for i, c := range contexts {
		if r, ok := c.(Routes); ok && deps.Mux != nil {
			r.Routes(deps.Mux)
		}
		if err := c.Start(ctx, deps); err != nil {
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), StopTimeout)
			StopAll(stopCtx, contexts[:i])
			cancel()
			return fmt.Errorf("start context %s: %w", c.Name(), err)
		}
	}
	return nil
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
