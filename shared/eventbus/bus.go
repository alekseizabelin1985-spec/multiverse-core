package eventbus

import "context"

// Handler processes one event. Returning an error asks the bus to retry;
// after the retries are exhausted the event goes to dead_letters and the
// stream moves on, so that one bad event cannot block a topic.
//
// A panic is not caught: by NFR-012 a panic is a defect and must bring the
// process down with a stack trace, not be swallowed by the delivery loop.
type Handler func(ctx context.Context, ev Event) error

// Middleware wraps a handler; logging.BusMiddleware is the one every
// subscription installs (foundation.md §5.3).
type Middleware func(Handler) Handler

// Chain applies middlewares to h so that the first one listed is the
// outermost.
func Chain(h Handler, mws ...Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] != nil {
			h = mws[i](h)
		}
	}
	return h
}

// Bus is the live side of the event stream: publication by type, and
// subscriptions with a consumer group.
//
// Guarantees (C-01): at-least-once delivery; order within a topic; an unknown
// or invalid type is not published but reported as an error; a handler error
// is retried three times and then parked in dead_letters.
type Bus interface {
	// Publish sends the event to the topic its type is registered under.
	Publish(ctx context.Context, ev Event) error
	// Subscribe consumes a topic under a consumer group until ctx is done.
	// Stopping that way is not a failure: a subscription cancelled by its
	// caller returns nil, whichever implementation serves it.
	Subscribe(ctx context.Context, topic, group string, h Handler) error
	Close() error
}

// Journal is the catch-up side: reading a topic by offset without joining a
// consumer group. State keeps its own cursor, Swarm and Gateway resume from
// the cursor recorded in their snapshot (ADR-011 p. 4, C-14).
type Journal interface {
	// ReadRange delivers the events of [from, to) in increasing offset order
	// and returns the offset to continue from.
	ReadRange(ctx context.Context, topic string, from, to int64, h Handler) (next int64, err error)
	// Tail delivers events from the given offset until ctx is done, and
	// returns nil when it stops that way — like Subscribe.
	Tail(ctx context.Context, topic string, from int64, h Handler) error
	// End returns the offset the next message will get (the high watermark).
	// A reader is at the end of the journal when its position is at least
	// End-1 on every topic it reads.
	End(ctx context.Context, topic string) (int64, error)
}

// Position is where in a topic the event being handled came from.
type Position struct {
	Topic  string
	Offset int64
}

type positionKey struct{}

// ContextWithPosition attaches the position of the current event. Both the
// subscription loop and the journal call it before invoking the handler, so a
// consumer can record its cursor in a snapshot.
func ContextWithPosition(ctx context.Context, pos Position) context.Context {
	return context.WithValue(ctx, positionKey{}, pos)
}

// PositionFromContext returns the position of the event being handled.
func PositionFromContext(ctx context.Context) (Position, bool) {
	pos, ok := ctx.Value(positionKey{}).(Position)
	return pos, ok
}
