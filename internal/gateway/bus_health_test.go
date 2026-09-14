package gateway_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// breakingBus ends the first subscription of one topic with an error of the
// transport and reports every subscription of that topic.
type breakingBus struct {
	eventbus.Bus
	topic string
	calls chan int

	mu sync.Mutex
	n  int
}

func (b *breakingBus) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	if topic != b.topic {
		return b.Bus.Subscribe(ctx, topic, group, h)
	}
	b.mu.Lock()
	b.n++
	n := b.n
	b.mu.Unlock()
	b.calls <- n
	if n == 1 {
		return errors.New("eventbus: read narrative_output: failed to dial")
	}
	return b.Bus.Subscribe(ctx, topic, group, h)
}

// timerLog reports the timers armed through After.
type timerLog struct {
	clock.Timers
	armed chan time.Duration
}

func (l *timerLog) After(d time.Duration) clock.Timer {
	t := l.Timers.After(d)
	select {
	case l.armed <- d:
	default:
	}
	return t
}

// lineWatch reports the lines of the log that hold every fragment.
type lineWatch struct {
	fragments [][]byte
	seen      chan struct{}
}

func (w *lineWatch) Write(p []byte) (int, error) {
	for _, f := range w.fragments {
		if !bytes.Contains(p, f) {
			return len(p), nil
		}
	}
	select {
	case w.seen <- struct{}{}:
	default:
	}
	return len(p), nil
}

func awaitValue[T comparable](t *testing.T, ch <-chan T, want T, what string) {
	t.Helper()
	deadline := testkit.After(15 * time.Second)
	for {
		select {
		case got := <-ch:
			if got == want {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// (c) While the subscription of a topic is down /health is degraded with
// bus: fail, also once the topic is subscribed again and nothing proves that
// subscription takes events; once it does — here by running for
// consumer.RestoredAfter — /health is ok again (component §11.4, T-473).
func TestHealthReportsTheBusDownUntilTheSubscriptionIsHeardAgain(t *testing.T) {
	bus := &breakingBus{topic: eventbus.TopicNarrativeOutput, calls: make(chan int, 16)}
	timers := &timerLog{armed: make(chan time.Duration, 256)}
	restored := &lineWatch{seen: make(chan struct{}, 1), fragments: [][]byte{
		[]byte(`"msg":"consumer: subscription restored"`), []byte(`"topic":"narrative_output"`),
	}}
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{
		bus:    func(b *membus.Bus) eventbus.Bus { bus.Bus = b; return bus },
		timers: func(inner clock.Timers) clock.Timers { timers.Timers = inner; return timers },
		log:    restored,
	})
	awaitValue(t, bus.calls, 1, "the first subscription")
	awaitValue(t, timers.armed, consumer.ResubscribeFirst, "the pause before the subscription again")
	if h := r.ctx.Health(); h.Status != runtime.StatusDegraded || h.Details["bus"] != runtime.StatusFail {
		t.Errorf("Health while the subscription is down = %+v, want degraded with bus fail", h)
	}

	r.clock.Advance(consumer.ResubscribeFirst)
	awaitValue(t, bus.calls, 2, "the subscription again")
	awaitValue(t, timers.armed, consumer.RestoredAfter, "the wait for the subscription to count as heard")
	if h := r.ctx.Health(); h.Status != runtime.StatusDegraded || h.Details["bus"] != runtime.StatusFail {
		t.Errorf("Health once subscribed again and not heard yet = %+v, want degraded with bus fail", h)
	}

	r.clock.Advance(consumer.RestoredAfter)
	awaitValue(t, restored.seen, struct{}{}, "the subscription to count as heard")
	if h := r.ctx.Health(); h.Status != runtime.StatusOK || h.Details["bus"] != runtime.StatusOK {
		t.Errorf("Health once the subscription is heard = %+v, want ok", h)
	}
}

// In replay Deps.Timers are the timers of the domain, which the events move
// or which never fire (NullTimers); the pauses of the consumer between its
// subscriptions are waits of the transport and run on the wall clock, so a
// subscription that ended is subscribed again in replay too (deviation 1 of
// T-473, C-01 v1.12; Mi-3 of review #1). The manual timers of the test never
// move here: the second subscription comes after one pause of the wall clock.
func TestReplaySubscribesAgainOnTheWallClock(t *testing.T) {
	bus := &breakingBus{topic: eventbus.TopicNarrativeOutput, calls: make(chan int, 16)}
	startOpts(t, runtime.ModeReplay, sqlitedir.Temp(t), options{
		bus: func(b *membus.Bus) eventbus.Bus { bus.Bus = b; return bus },
	})
	awaitValue(t, bus.calls, 1, "the first subscription")
	awaitValue(t, bus.calls, 2, "the subscription again after a pause of the wall clock")
}
