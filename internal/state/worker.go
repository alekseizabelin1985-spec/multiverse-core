package state

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	"multiverse-core.io/shared/eventbus"
)

// ErrWorldStopped is the answer of a world that decides nothing more: after a
// panic of its Applier, after an answer that did not get out (ErrPublishFailed),
// or once the context has stopped its worker.
var ErrWorldStopped = errors.New("state: world stopped")

// errPanicked marks the failure of a world whose Applier panicked.
var errPanicked = errors.New("panic")

// worker is the one goroutine that changes a world (state-and-mechanics.md
// §4.2): proposals of the world reach its Applier one at a time and in the
// order of system_events, and nothing else writes the world.
//
// It is also the boundary a panic stops at. Delivery recovers a panic of the
// handler it calls (C-01 v1.5), but the Applier runs here, on another
// goroutine, and a world whose Applier panicked halfway through a proposal
// cannot be shown to be intact: the worker stops the world, and everything
// after that is refused with ErrWorldStopped (NFR-012, §9).
type worker struct {
	worldID string
	applier *Applier
	log     *slog.Logger

	jobs chan job
	quit chan struct{}
	done chan struct{}

	stopOnce sync.Once
}

// job is one proposal handed to the worker and the channel its answer comes
// back on.
type job struct {
	ctx   context.Context
	ev    eventbus.Event
	reply chan error
}

func newWorker(worldID string, applier *Applier, log *slog.Logger) *worker {
	return &worker{
		worldID: worldID,
		applier: applier,
		log:     log.With("world_id", worldID),
		jobs:    make(chan job),
		quit:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (w *worker) start() {
	go func() {
		defer close(w.done)
		for {
			select {
			case <-w.quit:
				return
			case j := <-w.jobs:
				j.reply <- w.run(j)
			}
		}
	}()
}

// submit hands one proposal to the worker and waits for its answer. It does not
// give up when ctx is cancelled: a proposal the worker has taken is finished —
// its answer published, or the world stopped — rather than cut off halfway, and
// a worker that never finishes is bounded by the Stop of the context, not here
// (C-01 v1.7, ADR-023 p. 4).
func (w *worker) submit(ctx context.Context, ev eventbus.Event) error {
	if err := w.err(); err != nil {
		return err
	}
	reply := make(chan error, 1)
	select {
	case w.jobs <- job{ctx: ctx, ev: ev, reply: reply}:
	case <-w.done:
		return w.stoppedErr()
	}
	return <-reply
}

// run applies one proposal under the recover of the worker.
//
// The Applier gets the context of the subscription as the lifetime of the
// world: its cancellation by Stop ends the attempts to publish an answer that
// does not go out, and never a publication under way (Applier.publish).
func (w *worker) run(j job) (err error) {
	if failure := w.err(); failure != nil {
		return failure
	}
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		failure := fmt.Errorf("%w: %w while applying %s %s: %v", ErrWorldStopped, errPanicked, j.ev.Type, j.ev.ID, r)
		w.applier.halt(failure)
		w.log.Error("state worker panicked; the world is stopped",
			"event_id", j.ev.ID, "type", j.ev.Type, "panic", fmt.Sprint(r),
			"stack", string(debug.Stack()), "handled", false)
		err = failure
	}()
	return w.applier.Apply(j.ctx, j.ev)
}

// err is the failure that stopped the world, or nil.
func (w *worker) err() error { return w.applier.Stopped() }

func (w *worker) stoppedErr() error {
	if err := w.err(); err != nil {
		return err
	}
	return fmt.Errorf("%w: %s", ErrWorldStopped, w.worldID)
}

// signalStop tells the goroutine to end once it has finished the proposal it
// holds, without waiting for that.
func (w *worker) signalStop() { w.stopOnce.Do(func() { close(w.quit) }) }

// stop ends the goroutine once it has finished the proposal it holds, and
// waits for that within ctx.
func (w *worker) stop(ctx context.Context) error {
	w.signalStop()
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("state: stop the worker of %s: %w", w.worldID, ctx.Err())
	}
}

// finished reports whether the goroutine has ended.
func (w *worker) finished() bool {
	select {
	case <-w.done:
		return true
	default:
		return false
	}
}
