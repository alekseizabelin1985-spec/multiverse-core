package updates

import (
	"context"
	"errors"
	"sync"
)

// Fake is a Source driven by a test (FakeUpdateSource of component §15). Push
// queues updates; Start hands them to the handler one at a time, in order,
// and returns when ctx ends, when Close was called and the queue is drained,
// or with the error given to Fail once the queue before it is drained.
type Fake struct {
	mu      sync.Mutex
	queue   []item
	closed  bool
	started bool
	wake    chan struct{}
}

type item struct {
	update Update
	err    error
}

// NewFake returns a source with the given updates already queued.
func NewFake(updates ...Update) *Fake {
	f := &Fake{wake: make(chan struct{}, 1)}
	f.Push(updates...)
	return f
}

// Push queues updates behind those already queued.
func (f *Fake) Push(updates ...Update) {
	f.mu.Lock()
	for _, u := range updates {
		f.queue = append(f.queue, item{update: u})
	}
	f.mu.Unlock()
	f.signal()
}

// Fail makes Start return err once the updates queued before it are handled;
// ErrConflict imitates a second instance of the bot.
func (f *Fake) Fail(err error) {
	f.mu.Lock()
	f.queue = append(f.queue, item{err: err})
	f.mu.Unlock()
	f.signal()
}

// Close makes Start return nil once the queue is drained.
func (f *Fake) Close() {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	f.signal()
}

// Start implements Source. A Fake is started at most once.
func (f *Fake) Start(ctx context.Context, h Handler) error {
	if h == nil {
		return errors.New("updates: Start needs a handler")
	}
	f.mu.Lock()
	if f.started {
		f.mu.Unlock()
		return errors.New("updates: the fake source is already started")
	}
	f.started = true
	f.mu.Unlock()

	for {
		if ctx.Err() != nil {
			return nil
		}
		f.mu.Lock()
		if len(f.queue) > 0 {
			next := f.queue[0]
			f.queue = f.queue[1:]
			f.mu.Unlock()
			if next.err != nil {
				return next.err
			}
			h(ctx, next.update)
			continue
		}
		closed := f.closed
		f.mu.Unlock()
		if closed {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-f.wake:
		}
	}
}

func (f *Fake) signal() {
	select {
	case f.wake <- struct{}{}:
	default:
	}
}
