package replay

import (
	"context"

	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
)

// Middleware returns the handler middleware of a mode. In live mode it changes
// nothing. In replay it moves the clock of the process to the timestamp of the
// event before the handler runs, and marks the event handed on as replayed
// (meta.replay=true, C-07): events the handler derives from it inherit the
// mark through eventbus.Derive.
//
// The clock is one for the whole process and never goes back. When the
// handler runs, it stands at the latest event any reader of the process has
// observed, which is this event's timestamp or later; with more than one
// reader, how much later depends on the scheduler. A handler that needs the
// time of its own event takes ev.Timestamp, or builds with eventbus.Derive,
// which inherits it (see the package documentation).
//
// A nil clock in replay is a defect of the process, not of an event, and
// panics when the middleware is built — at start, not on the first delivery.
func Middleware(mode runtime.Mode, ec *EventClock) eventbus.Middleware {
	if mode != runtime.ModeReplay {
		return func(h eventbus.Handler) eventbus.Handler { return h }
	}
	if ec == nil {
		panic("replay: Middleware in mode replay needs an EventClock")
	}
	return func(h eventbus.Handler) eventbus.Handler {
		return func(ctx context.Context, ev eventbus.Event) error {
			ec.Observe(ev.Timestamp)
			ev.Meta.Replay = true
			return h(ctx, ev)
		}
	}
}

// Transport is the one object a process builds for the bus and the journal
// (cmd/multiverse passes it to the contexts as both Deps.Bus and Deps.Journal).
type Transport interface {
	eventbus.Bus
	eventbus.Journal
}

// WithMiddleware wraps every handler the contexts hand to the transport —
// Subscribe, ReadRange and Tail — in mws, the first listed outermost. The
// middleware runs inside the delivery of the bus, so validation on read, the
// retries and the dead letters see the event as published, and a retried
// handler goes through the middleware again. Publish, End and Close are the
// transport's own.
func WithMiddleware(t Transport, mws ...eventbus.Middleware) Transport {
	return &wrapped{Transport: t, mws: mws}
}

// wrapped is used through a pointer: the process hands one transport to the
// contexts as both Deps.Bus and Deps.Journal, and a caller comparing the two
// must be able to — a struct holding a slice cannot be compared as a value.
type wrapped struct {
	Transport
	mws []eventbus.Middleware
}

func (w *wrapped) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	return w.Transport.Subscribe(ctx, topic, group, w.chain(h))
}

func (w *wrapped) ReadRange(ctx context.Context, topic string, from, to int64, h eventbus.Handler) (int64, error) {
	return w.Transport.ReadRange(ctx, topic, from, to, w.chain(h))
}

func (w *wrapped) Tail(ctx context.Context, topic string, from int64, h eventbus.Handler) error {
	return w.Transport.Tail(ctx, topic, from, w.chain(h))
}

// chain leaves a nil handler nil, so that the transport refuses it as it
// would refuse it unwrapped.
func (w *wrapped) chain(h eventbus.Handler) eventbus.Handler {
	if h == nil {
		return nil
	}
	return eventbus.Chain(h, w.mws...)
}
