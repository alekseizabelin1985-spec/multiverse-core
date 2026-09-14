package deliver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/deliver"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
)

// signalTimers are the manual timers of a clock that report every pause the
// loop starts, so that a test moves the clock only once the loop waits.
type signalTimers struct {
	inner *clock.ManualTimers
	made  chan time.Duration
}

func (s signalTimers) After(d time.Duration) clock.Timer {
	t := s.inner.After(d)
	s.made <- d
	return t
}

func (s signalTimers) Every(time.Duration) clock.Timer { panic("no periodic timer expected") }

func runLoop(t *testing.T, l *deliver.Loop) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(done)
	}()
	return func() {
		cancel()
		<-done
	}
}

// Acceptance of T-307, addition 2 of the orchestrator: a gateway that stops
// answers the long-poll with an empty list at once. The loop pauses after such
// an answer as after a failed long-poll — a second, growing — and polls no more
// than once per pause.
func TestAGatewayThatAnswersEmptyAtOnceIsPolledOncePerPause(t *testing.T) {
	ob := newOutbox(nil)
	ob.stopping = true
	timers := signalTimers{inner: ob.clock.Timers(), made: make(chan time.Duration, 16)}
	log, buf := logBuffer()
	l, err := deliver.New(deliver.Options{Gateway: ob, Sender: &sender.Fake{}, Timers: timers, Clock: ob.clock, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	stop := runLoop(t, l)
	defer stop()

	for i, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second} {
		var pause time.Duration
		select {
		case pause = <-timers.made:
		case <-clock.RealTimers{}.After(5 * time.Second).C():
			t.Fatalf("no pause after empty answer %d: the loop polls a stopping gateway without waiting (%d polls)", i+1, len(ob.pollAfters()))
		}
		if pause != want {
			t.Errorf("pause after empty answer %d = %s, want %s", i+1, pause, want)
		}
		if n := len(ob.pollAfters()); n != i+1 {
			t.Fatalf("polls before pause %d = %d, want %d", i+1, n, i+1)
		}
		ob.clock.Advance(pause - time.Millisecond)
		time.Sleep(20 * time.Millisecond)
		if n := len(ob.pollAfters()); n != i+1 {
			t.Fatalf("polled %d times within pause %d, want %d", n, i+1, i+1)
		}
		ob.clock.Advance(time.Millisecond)
	}
	waitUntil(t, func() bool { return len(ob.pollAfters()) == 4 })
	if !strings.Contains(buf.String(), "deliveries answered empty before the wait") {
		t.Errorf("no line of the early empty answer:\n%s", buf.String())
	}
}

// An empty answer that took its wait is the ordinary end of a long-poll: the
// next one follows at once, and so does it after an answer that took
// EarlyEmpty. Only an answer sooner than that is paused after.
func TestAnEmptyAnswerThatTookTheWaitIsNotPaused(t *testing.T) {
	for _, c := range []struct {
		took  time.Duration
		pause bool
	}{
		{deliver.Wait, false},
		{deliver.EarlyEmpty, false},
		{deliver.EarlyEmpty - time.Millisecond, true},
	} {
		ob := newOutbox(nil)
		ob.block = true
		ob.emptyTook = c.took
		ob.pollErr = []error{nil, nil, nil}
		timers := &instantTimers{}
		l, err := deliver.New(deliver.Options{Gateway: ob, Sender: &sender.Fake{}, Timers: timers, Clock: ob.clock})
		if err != nil {
			t.Fatal(err)
		}
		stop := runLoop(t, l)
		waitUntil(t, func() bool { return len(ob.pollAfters()) == 4 })
		stop()
		if paused := len(timers.all()) > 0; paused != c.pause {
			t.Errorf("empty answers of %s: paused %t (%v), want %t", c.took, paused, timers.all(), c.pause)
		}
	}
}

// advancingTimers move a manual clock by every pause and fire it at once: the
// pauses of a client that runs in the goroutine of the loop, told in the time
// of the clock of the test.
type advancingTimers struct{ clock *clock.Manual }

func (a advancingTimers) After(d time.Duration) clock.Timer {
	a.clock.Advance(d)
	ch := make(chan time.Time, 1)
	ch <- a.clock.Now()
	return chanTimer{ch}
}

func (a advancingTimers) Every(time.Duration) clock.Timer { panic("no periodic timer expected") }

func jsonAnswer(status int, v any) *http.Response {
	body, _ := json.Marshal(v)
	h := http.Header{}
	h.Set("Content-Type", api.ContentTypeJSON)
	return &http.Response{StatusCode: status, Header: h, Body: nopCloser{bytes.NewReader(body)}}
}

type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }

// slowBroker is a gateway whose broker takes 5 s to refuse: every ack answers
// 503 bus_unavailable after 5 s of the clock. The first long-poll leases one
// delivery; the moments of the long-polls are recorded.
type slowBroker struct {
	clock *clock.Manual
	mu    sync.Mutex
	polls []time.Time
	acks  int
	// acksBefore is the number of ack attempts each long-poll found made.
	acksBefore []int
}

func (b *slowBroker) RoundTrip(r *http.Request) (*http.Response, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch r.URL.Path {
	case "/v1/clients/telegram-bot/deliveries":
		b.polls = append(b.polls, b.clock.Now())
		b.acksBefore = append(b.acksBefore, b.acks)
		res := api.DeliveriesResponse{Deliveries: []api.Delivery{}, Cursor: "1"}
		if len(b.polls) == 1 {
			res.Deliveries = []api.Delivery{delivery("a1", "player-A", chatA, render.KindMechanics, "удар")}
		}
		return jsonAnswer(http.StatusOK, res), nil
	case "/v1/clients/telegram-bot/deliveries/ack":
		b.acks++
		b.clock.Advance(5 * time.Second)
		return jsonAnswer(http.StatusServiceUnavailable, api.NewError(api.CodeBusUnavailable, nil).Body()), nil
	}
	return jsonAnswer(http.StatusTeapot, nil), nil
}

// Acceptance of T-307, review #1: an ack the broker refuses in 5 s does not
// hold the loop past the lease. The delivery was leased by the first
// long-poll; its ack fails after the answer and once more before the next
// long-poll, and that long-poll starts before the lease of 30 s is over. The
// control runs the same with the default client for the ack — four attempts —
// and shows the next long-poll would start after the lease, when the gateway
// hands the delivery out again.
func TestAnAckTheBrokerRefusesDoesNotHoldTheNextPollPastTheLease(t *testing.T) {
	gap := func(acks func(b *slowBroker) *client.Client) (time.Duration, int) {
		b := &slowBroker{clock: clock.NewManual(epoch)}
		gw := client.New("http://gateway.test", "telegram-bot")
		gw.HTTP = &http.Client{Transport: b}
		l, err := deliver.New(deliver.Options{Gateway: gw, Acks: acks(b), Sender: &sender.Fake{}, Timers: &instantTimers{}, Clock: b.clock})
		if err != nil {
			t.Fatal(err)
		}
		mustOnce(t, l)
		mustOnce(t, l)
		b.mu.Lock()
		defer b.mu.Unlock()
		if len(b.polls) != 2 {
			t.Fatalf("long-polls %d, want 2", len(b.polls))
		}
		return b.polls[1].Sub(b.polls[0]), b.acksBefore[1]
	}

	took, acks := gap(func(b *slowBroker) *client.Client {
		c := deliver.NewAckClient("http://gateway.test", "telegram-bot", advancingTimers{b.clock})
		c.HTTP.Transport = b
		return c
	})
	if took >= lease {
		t.Errorf("the next long-poll started %s after the lease was given, want under the lease of %s", took, lease)
	}
	if want := 2 * (deliver.AckBackoff.Retries + 1); acks != want {
		t.Errorf("ack attempts before the next long-poll %d, want %d: two flushes of %d attempts each", acks, want, deliver.AckBackoff.Retries+1)
	}

	control, _ := gap(func(b *slowBroker) *client.Client {
		c := client.New("http://gateway.test", "telegram-bot")
		c.HTTP = &http.Client{Transport: b}
		c.Timers = advancingTimers{b.clock}
		return c
	})
	if control < lease {
		t.Fatalf("control: with the default client the next long-poll started %s after the lease was given; the test proves nothing", control)
	}
}

// The limits of the client of the ack: one flush ends within MaxFlush, and the
// two flushes between long-polls stay under the lease.
func TestTheAckClientEndsWithinTheLease(t *testing.T) {
	c := deliver.NewAckClient("http://gateway.test", "telegram-bot", nil)
	if c.HTTP.Timeout != deliver.AckHTTPTimeout || c.Backoff != deliver.AckBackoff || c.ClientID != "telegram-bot" {
		t.Errorf("ack client: timeout %s, backoff %+v, client id %q", c.HTTP.Timeout, c.Backoff, c.ClientID)
	}
	if c.HTTP.Timeout <= api.RequestTimeout {
		t.Errorf("AckHTTPTimeout %s cuts a request the gateway may take %s to answer", c.HTTP.Timeout, api.RequestTimeout)
	}
	flush := time.Duration(deliver.AckBackoff.Retries+1) * deliver.AckHTTPTimeout
	for n := range deliver.AckBackoff.Retries {
		flush += deliver.AckBackoff.Pause(n)
	}
	if flush > deliver.MaxFlush || 2*deliver.MaxFlush >= lease {
		t.Errorf("a flush lasts up to %s (MaxFlush %s); two of them must stay under the lease of %s", flush, deliver.MaxFlush, lease)
	}
}
