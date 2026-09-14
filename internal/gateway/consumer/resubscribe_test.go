package consumer_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
)

// A subscription of the dispatcher ends only by Stop: after an end the topic
// is subscribed again with a pause on the timers of the dispatcher, and /health
// sees the topic down until the new subscription is heard (T-473). The tests
// run on the manual clock: a pause passes when the test moves the clock, and
// only after the dispatcher armed it.

// armedTimers are the manual timers of the fixture that report every timer
// armed through After.
type armedTimers struct {
	clock.Timers
	armed chan time.Duration
}

func newArmedTimers(inner clock.Timers) *armedTimers {
	return &armedTimers{Timers: inner, armed: make(chan time.Duration, 256)}
}

func (a *armedTimers) After(d time.Duration) clock.Timer {
	t := a.Timers.After(d)
	a.armed <- d
	return t
}

// await waits until a timer of d is armed; timers of other durations armed
// meanwhile are passed over.
func (a *armedTimers) await(t *testing.T, d time.Duration) {
	t.Helper()
	deadline := testkit.After(15 * time.Second)
	for {
		select {
		case got := <-a.armed:
			if got == d {
				return
			}
		case <-deadline:
			t.Fatalf("no timer of %v was armed", d)
		}
	}
}

// none says no timer of d is armed among those reported so far.
func (a *armedTimers) none(d time.Duration) bool {
	for {
		select {
		case got := <-a.armed:
			if got == d {
				return false
			}
		default:
			return true
		}
	}
}

// scriptedBus plays a script for the subscriptions of one topic, one entry per
// call; a call past the script subscribes to the bus beneath. The other topics
// subscribe to the bus beneath at once.
type scriptedBus struct {
	eventbus.Bus
	topic  string
	script []func(ctx context.Context, h eventbus.Handler, inner func(context.Context, eventbus.Handler) error) error

	mu    sync.Mutex
	calls int
}

func (b *scriptedBus) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	inner := func(ctx context.Context, h eventbus.Handler) error { return b.Bus.Subscribe(ctx, topic, group, h) }
	if topic != b.topic {
		return inner(ctx, h)
	}
	b.mu.Lock()
	n := b.calls
	b.calls++
	b.mu.Unlock()
	if n < len(b.script) {
		return b.script[n](ctx, h, inner)
	}
	return inner(ctx, h)
}

func (b *scriptedBus) subscriptions() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

var errBroker = errors.New("eventbus: read system_events: unexpected EOF")

// failing ends the subscription with err at once.
func failing(err error) func(context.Context, eventbus.Handler, func(context.Context, eventbus.Handler) error) error {
	return func(context.Context, eventbus.Handler, func(context.Context, eventbus.Handler) error) error {
		return err
	}
}

// logBuffer is the log of a dispatcher a test reads.
type logBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuffer) count(fragment string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Count(l.buf.String(), fragment)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

type resubscribing struct {
	*fixture
	timers *armedTimers
	log    *logBuffer
	calls  *atomic.Int32
}

func newResubscribing(t *testing.T) *resubscribing {
	t.Helper()
	f := newFixture(t, membus.Chaos{})
	return &resubscribing{fixture: f, timers: newArmedTimers(f.clock.Timers()), log: &logBuffer{}, calls: &atomic.Int32{}}
}

func (r *resubscribing) dispatcher(t *testing.T, bus eventbus.Bus) *consumer.Dispatcher {
	t.Helper()
	d, err := consumer.New(consumer.Config{
		Bus: bus, Journal: r.bus, DB: r.db, Model: r.model, Clock: r.clock, Timers: r.timers,
		Log:     slog.New(slog.NewJSONHandler(r.log, nil)),
		Effects: map[string][]consumer.Effect{readmodel.TypeEntityCreated: {logging(r.calls, nil)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// pass moves the clock past a pause the dispatcher armed.
func (r *resubscribing) pass(t *testing.T, pause time.Duration) {
	t.Helper()
	r.timers.await(t, pause)
	r.clock.Advance(pause)
}

func stopWithin(t *testing.T, d *consumer.Dispatcher) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.Stop(ctx)
}

// (a) A subscription that takes an event and then ends with an error of the
// transport — before the bus committed the event — is subscribed again after
// the first pause. The event comes again and takes no effect twice, the next
// one takes its effect, and the topic is heard again. The other topics are not
// subscribed again.
func TestASubscriptionThatEndsIsSubscribedAgainAndTakesEachEffectOnce(t *testing.T) {
	r := newResubscribing(t)
	bus := &scriptedBus{Bus: r.bus, topic: eventbus.TopicSystemEvents}
	var others atomic.Int32
	counting := &countingSubscriptions{Bus: bus, others: &others, topic: eventbus.TopicSystemEvents}
	bus.script = append(bus.script, func(ctx context.Context, h eventbus.Handler, inner func(context.Context, eventbus.Handler) error) error {
		innerCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		// The handler of the dispatcher takes the first event; the transport
		// then goes away before it commits the offset.
		_ = inner(innerCtx, func(ctx context.Context, ev eventbus.Event) error {
			if err := h(ctx, ev); err != nil {
				return err
			}
			cancel()
			return ctx.Err()
		})
		return errBroker
	})
	d := r.dispatcher(t, counting)
	r.start(t, d, nil)

	r.publish(t, named("first", created("player-A", 10)))
	eventually(t, "the end of the subscription to be reported", func() bool { return d.Err() != nil })
	if err := d.Err(); !errors.Is(err, errBroker) || !strings.Contains(err.Error(), eventbus.TopicSystemEvents) {
		t.Errorf("Err = %v, want the end of the subscription of %s", err, eventbus.TopicSystemEvents)
	}
	if bus.subscriptions() != 1 {
		t.Fatalf("%d subscriptions of %s before the pause passed, want 1", bus.subscriptions(), eventbus.TopicSystemEvents)
	}

	r.pass(t, consumer.ResubscribeFirst)
	eventually(t, "the event that came again to be heard", func() bool { return d.Err() == nil })
	r.publish(t, named("second", created("player-B", 10)))
	eventually(t, "the event after the new subscription to be taken", func() bool {
		offset, _ := r.cursor(t, eventbus.TopicSystemEvents)
		return offset == 1
	})

	if got := bus.subscriptions(); got != 2 {
		t.Errorf("%d subscriptions of %s, want the one of Start and one again", got, eventbus.TopicSystemEvents)
	}
	if n := others.Load(); n != int32(len(consumer.Topics)-1) {
		t.Errorf("%d subscriptions of the other topics, want each once", n)
	}
	if n := r.calls.Load(); n != 2 {
		t.Errorf("the effects ran %d times for two events, one of them delivered twice", n)
	}
	if n := r.count(t, `SELECT COUNT(*) FROM effects_log`); n != 2 {
		t.Errorf("%d effect rows, want 2", n)
	}
	if n := r.count(t, `SELECT COUNT(*) FROM processed_events`); n != 2 {
		t.Errorf("%d marks processed, want 2", n)
	}
	line := `"msg":"consumer: subscription ended, subscribing again after a pause","topic":"system_events","error":"` + errBroker.Error()
	if n := r.log.count(line); n != 1 {
		t.Errorf("%d lines of the subscription started again with its topic and error, want 1:\n%s", n, r.log)
	}
}

// countingSubscriptions counts the subscriptions of every topic but one.
type countingSubscriptions struct {
	eventbus.Bus
	topic  string
	others *atomic.Int32
}

func (c *countingSubscriptions) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	if topic != c.topic {
		c.others.Add(1)
	}
	return c.Bus.Subscribe(ctx, topic, group, h)
}

// (b) While the subscription keeps ending, the pause doubles up to the
// ceiling; every subscription again writes its line. A subscription that was
// heard before it ended starts the pauses from the first again.
func TestThePauseGrowsToTheCeilingAndStartsAgainAfterASubscriptionWasHeard(t *testing.T) {
	r := newResubscribing(t)
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 15 * time.Second, 15 * time.Second}
	if consumer.ResubscribeFirst != want[0] || consumer.ResubscribeCeiling != want[len(want)-1] {
		t.Fatalf("pauses %v to %v, the test expects %v to %v", consumer.ResubscribeFirst, consumer.ResubscribeCeiling, want[0], want[len(want)-1])
	}
	bus := &scriptedBus{Bus: r.bus, topic: eventbus.TopicSystemEvents}
	for range want {
		bus.script = append(bus.script, failing(errBroker))
	}
	heard := make(chan struct{})
	// Heard, then ended: the next pause is the first again.
	bus.script = append(bus.script, func(ctx context.Context, h eventbus.Handler, inner func(context.Context, eventbus.Handler) error) error {
		innerCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		_ = inner(innerCtx, func(ctx context.Context, ev eventbus.Event) error {
			err := h(ctx, ev)
			close(heard)
			cancel()
			return errors.Join(err, ctx.Err())
		})
		return errBroker
	})
	d := r.dispatcher(t, bus)
	r.start(t, d, nil)

	for i, pause := range want {
		r.pass(t, pause)
		eventually(t, "the next subscription", func() bool { return bus.subscriptions() == i+2 })
	}
	r.publish(t, named("heard", created("player-A", 10)))
	receive(t, heard, "the event of the subscription that was heard")
	r.pass(t, consumer.ResubscribeFirst)
	eventually(t, "the subscription after the one heard", func() bool { return bus.subscriptions() == len(want)+2 })

	if n := r.log.count(`"msg":"consumer: subscription ended, subscribing again after a pause","topic":"system_events"`); n != len(want)+1 {
		t.Errorf("%d lines of subscriptions started again, want %d", n, len(want)+1)
	}
	for _, pause := range []string{`"pause":1000000000`, `"pause":15000000000`} {
		if !strings.Contains(r.log.String(), pause) {
			t.Errorf("no line with %s in the log:\n%s", pause, r.log)
		}
	}
}

// (d) A subscription that returns nil while the dispatcher was not stopped has
// ended as well, and the topic is subscribed again. A bus that refuses with
// ErrClosed is not subscribed again: it will not take a subscription.
func TestANilEndIsSubscribedAgainAndAClosedBusIsNot(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		r := newResubscribing(t)
		bus := &scriptedBus{Bus: r.bus, topic: eventbus.TopicSystemEvents}
		bus.script = append(bus.script, failing(nil))
		d := r.dispatcher(t, bus)
		r.start(t, d, nil)
		eventually(t, "the silent end to be reported", func() bool { return d.Err() != nil })
		if !strings.Contains(d.Err().Error(), "without an error") {
			t.Errorf("Err = %v, want the silent end", d.Err())
		}
		r.pass(t, consumer.ResubscribeFirst)
		eventually(t, "the subscription again", func() bool { return bus.subscriptions() == 2 })
	})
	t.Run("closed", func(t *testing.T) {
		r := newResubscribing(t)
		bus := &scriptedBus{Bus: r.bus, topic: eventbus.TopicSystemEvents}
		// The kafka adapter joins the refusal with the close of its reader.
		bus.script = append(bus.script, failing(errors.Join(eventbus.ErrClosed, nil)))
		d := r.dispatcher(t, bus)
		r.start(t, d, nil)
		eventually(t, "the refusal to be logged", func() bool {
			return r.log.count(`"msg":"consumer: subscription refused by a closed bus, not subscribing again"`) == 1
		})
		r.clock.Advance(consumer.ResubscribeCeiling)
		if n := bus.subscriptions(); n != 1 {
			t.Errorf("%d subscriptions of a closed bus, want no second", n)
		}
		if !r.timers.none(consumer.ResubscribeFirst) {
			t.Error("a pause was armed after the refusal of a closed bus")
		}
		if err := d.Err(); !errors.Is(err, eventbus.ErrClosed) {
			t.Errorf("Err = %v, want the topic reported down by the closed bus", err)
		}
	})
}

// (e) Stop ends a pause at once, and no subscription starts after it.
func TestStopEndsAPauseAndSubscribesNoMore(t *testing.T) {
	r := newResubscribing(t)
	bus := &scriptedBus{Bus: r.bus, topic: eventbus.TopicSystemEvents}
	bus.script = append(bus.script, failing(errBroker))
	d := r.dispatcher(t, bus)
	if err := d.Start(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	r.timers.await(t, consumer.ResubscribeFirst)
	if err := stopWithin(t, d); err != nil {
		t.Fatalf("Stop during the pause: %v", err)
	}
	r.clock.Advance(consumer.ResubscribeCeiling)
	if n := bus.subscriptions(); n != 1 {
		t.Errorf("%d subscriptions after Stop, want none after the first", n)
	}
}

// (e) Stop during a subscription whose transport does not return — a reader
// of kafka-go joining its group — waits for no more than the grace once no
// handler runs, and an event that transport delivers afterwards reaches no
// handler of the dispatcher: nothing is written after Stop.
func TestStopDoesNotWaitForATransportThatDoesNotReturn(t *testing.T) {
	r := newResubscribing(t)
	release := make(chan struct{})
	refused := make(chan error, 1)
	late := named("late", created("player-A", 10))
	bus := &scriptedBus{Bus: r.bus, topic: eventbus.TopicSystemEvents}
	bus.script = append(bus.script, func(ctx context.Context, h eventbus.Handler, _ func(context.Context, eventbus.Handler) error) error {
		<-release
		refused <- h(eventbus.ContextWithPosition(ctx, eventbus.Position{Topic: eventbus.TopicSystemEvents, Offset: 0}), late)
		return nil
	})
	d := r.dispatcher(t, bus)
	if err := d.Start(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	// No deadline: only the grace ends the wait for the transport.
	stopped := make(chan error, 1)
	go func() { stopped <- d.Stop(context.Background()) }()
	r.pass(t, consumer.StopTransportGrace)
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("Stop with a transport that does not return: %v", err)
		}
	case <-testkit.After(10 * time.Second):
		t.Fatal("Stop did not return once the grace passed")
	}

	close(release)
	if err := receive(t, refused, "the answer of the handler after Stop"); err == nil {
		t.Error("the handler took an event after Stop")
	}
	r.clock.Advance(consumer.ResubscribeCeiling)
	if n := r.calls.Load(); n != 0 {
		t.Errorf("the effects of an event delivered after Stop ran %d times", n)
	}
	if n := r.count(t, `SELECT COUNT(*) FROM processed_events`); n != 0 {
		t.Errorf("%d marks processed after Stop", n)
	}
	if _, ok := r.model.Character("player-A"); ok {
		t.Error("an event delivered after Stop reached the projection")
	}
	if n := bus.subscriptions(); n != 1 {
		t.Errorf("%d subscriptions after the transport returned, want none again after Stop", n)
	}
}

// (e) Stop waits for a handler that runs, and the grace for the transport
// does not begin before the handler returned: the databases the handler
// writes are closed by the caller after Stop.
func TestStopWaitsForARunningHandler(t *testing.T) {
	r := newResubscribing(t)
	inHandler, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var finished atomic.Bool
	d, err := consumer.New(consumer.Config{
		Bus: r.bus, Journal: r.bus, DB: r.db, Model: r.model, Clock: r.clock, Timers: r.timers,
		Log: slog.New(slog.NewJSONHandler(r.log, nil)),
		Effects: map[string][]consumer.Effect{readmodel.TypeEntityCreated: {func(ctx context.Context, _ *sql.Tx, _ eventbus.Event, _ readmodel.Result) error {
			close(inHandler)
			<-ctx.Done()
			close(cancelled)
			<-release
			finished.Store(true)
			return ctx.Err()
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Start(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	r.publish(t, named("slow", created("player-A", 10)))
	receive(t, inHandler, "the handler to run")
	stopped := make(chan error, 1)
	go func() {
		err := stopWithin(t, d)
		if !finished.Load() {
			err = errors.Join(err, errors.New("Stop returned while a handler ran"))
		}
		stopped <- err
	}()
	receive(t, cancelled, "the cancellation of Stop to reach the handler")
	// Stop has begun; while the handler runs it arms no grace. The window is
	// a bound on how long to look, not a pause the result depends on.
	window := testkit.After(200 * time.Millisecond)
	for looking := true; looking; {
		select {
		case d := <-r.timers.armed:
			if d == consumer.StopTransportGrace {
				t.Error("Stop armed the grace for the transport while a handler ran")
				r.clock.Advance(d)
			}
		case <-window:
			looking = false
		}
	}
	close(release)
	if err := receive(t, stopped, "Stop to return"); err != nil {
		t.Error(err)
	}
}

// named gives an event an id of its own: an id from testkit.Deterministic
// starts again in every test, and on the broker in every subtest.
func named(id string, ev eventbus.Event) eventbus.Event {
	ev.ID = id
	return ev
}

// receive takes the next value of ch, or fails the test after 15 s: a defect
// fails with what was awaited instead of hanging the package until the
// timeout of go test (N-2 of review #1 of T-473).
func receive[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-testkit.After(15 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		var zero T
		return zero
	}
}

// stoppingTimers stand in front of the timers of a test and, when the pause
// before a subscription again is armed, stop the dispatcher and hand out a
// timer that has already fired: both cases of the wait are ready at once.
type stoppingTimers struct {
	clock.Timers
	d         *consumer.Dispatcher
	cancelled <-chan struct{}
	stopped   chan error
}

func (s *stoppingTimers) After(d time.Duration) clock.Timer {
	if d != consumer.ResubscribeFirst {
		return s.Timers.After(d)
	}
	go func() { s.stopped <- s.d.Stop(context.Background()) }()
	<-s.cancelled
	ch := make(chan time.Time, 1)
	ch <- time.Time{}
	return firedTimer{ch: ch}
}

type firedTimer struct{ ch chan time.Time }

func (f firedTimer) C() <-chan time.Time { return f.ch }
func (f firedTimer) Stop() bool          { return false }

// cancelWatch closes cancelled once the context of the subscription of topic
// is done: the moment Stop has cancelled the subscriptions.
type cancelWatch struct {
	eventbus.Bus
	topic     string
	cancelled chan struct{}
}

func (c *cancelWatch) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	if topic == c.topic {
		go func() {
			<-ctx.Done()
			close(c.cancelled)
		}()
	}
	return c.Bus.Subscribe(ctx, topic, group, h)
}

// (e) A pause that runs out in the moment Stop cancels the subscriptions does
// not lead to a Subscribe after Stop: the wait sees both of its cases ready,
// and select picks either (Mi-2 of review #1 of T-473). The race is played
// twenty times, so that a wait that trusted its timer loses it at least once
// but with a chance of one in a million.
func TestAPauseThatEndsWithStopSubscribesNoMore(t *testing.T) {
	for i := range 20 {
		r := newResubscribing(t)
		cancelled := make(chan struct{})
		bus := &scriptedBus{Bus: &cancelWatch{Bus: r.bus, topic: eventbus.TopicWorldEvents, cancelled: cancelled},
			topic: eventbus.TopicSystemEvents}
		bus.script = append(bus.script, failing(errBroker))
		timers := &stoppingTimers{Timers: r.clock.Timers(), cancelled: cancelled, stopped: make(chan error, 1)}
		d, err := consumer.New(consumer.Config{
			Bus: bus, Journal: r.bus, DB: r.db, Model: r.model, Clock: r.clock, Timers: timers,
			Log: slog.New(slog.NewJSONHandler(r.log, nil)),
		})
		if err != nil {
			t.Fatal(err)
		}
		timers.d = d
		if err := d.Start(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		if err := receive(t, timers.stopped, "Stop to return"); err != nil {
			t.Fatalf("round %d: Stop: %v", i, err)
		}
		if n := bus.subscriptions(); n != 1 {
			t.Fatalf("round %d: %d subscriptions of %s, want none after Stop", i, n, eventbus.TopicSystemEvents)
		}
	}
}

// (e) Stop called when its deadline has already passed — the contexts
// stopped before the gateway spent the budget of the process — reports no
// handler late when none runs (N-4 of review #1 of T-473). A transport that
// does not return keeps the subscriptions from ending first; twenty rounds
// make a select that weighs the deadline against idle pick the deadline.
func TestStopAtAnExpiredDeadlineWithNoHandlerIsNoError(t *testing.T) {
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	for i := range 20 {
		r := newResubscribing(t)
		release := make(chan struct{})
		bus := &scriptedBus{Bus: r.bus, topic: eventbus.TopicSystemEvents}
		bus.script = append(bus.script, func(context.Context, eventbus.Handler, func(context.Context, eventbus.Handler) error) error {
			<-release
			return nil
		})
		d := r.dispatcher(t, bus)
		if err := d.Start(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		// A guard: a Stop that waited for the transport without its deadline
		// fails here instead of hanging the package (N-7 of review #2).
		stopped := make(chan error, 1)
		go func() { stopped <- d.Stop(expired) }()
		err := receive(t, stopped, fmt.Sprintf("round %d: Stop at an expired deadline to return", i))
		close(release)
		if err != nil {
			t.Fatalf("round %d: Stop at an expired deadline without a handler = %v, want nil", i, err)
		}
	}
}
