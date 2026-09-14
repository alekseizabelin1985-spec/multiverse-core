package state_test

import (
	"context"
	"fmt"

	"multiverse-core.io/shared/eventbus"
)

// Tail is the reading State opens on system_events (ADR-011 p. 4, T-059), with
// the same trace as Subscribe: the context it runs under, a failure the test
// wants it to return, and a note when it returns.
func (r *rigged) Tail(ctx context.Context, topic string, from int64, h eventbus.Handler) error {
	r.mu.Lock()
	r.subCtx = ctx
	failure := r.subErr
	r.mu.Unlock()
	if failure != nil {
		return failure
	}
	err := r.Bus.Tail(ctx, topic, from, h)
	r.note(fmt.Sprintf("subscription returned (context: %v)", ctx.Err()))
	return err
}
