package outbox_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/shared/clock"
)

type served struct {
	resp api.DeliveriesResponse
	err  error
	at   time.Time
}

// serve runs Serve in the background and returns where its answer arrives.
func (f *fixture) serve(p outbox.Poll) <-chan served {
	out := make(chan served, 1)
	go func() {
		resp, err := f.svc.Serve(context.Background(), p)
		out <- served{resp: resp, err: err, at: clock.Real{}.Now()}
	}()
	return out
}

// waiting blocks until a long-poll has leased nothing and armed its wake-up
// with the bell of telegram taken: from then on it waits, and a ring reaches it.
func waiting(t *testing.T, f *fixture) {
	t.Helper()
	for range 2000 {
		if f.timers.afters.Load() >= 2 && outbox.HasBell(f.store.Notifier(), telegram) {
			return
		}
		pause := clock.RealTimers{}.After(time.Millisecond)
		<-pause.C()
	}
	t.Fatal("the long-poll never waited")
}

func receive(t *testing.T, ch <-chan served, within time.Duration) served {
	t.Helper()
	limit := clock.RealTimers{}.After(within)
	defer limit.Stop()
	select {
	case s := <-ch:
		return s
	case <-limit.C():
		t.Fatalf("no answer within %s", within)
		return served{}
	}
}

func poll(wait time.Duration) outbox.Poll {
	return outbox.Poll{ClientID: bot, Platform: telegram, Limit: 100, Wait: wait}
}

// The route comes from the link at the moment of the answer; the answer
// carries the delivery and a cursor at its seq.
func TestServeGivesTheDeliveryWithItsRoute(t *testing.T) {
	f := newFixture(t)
	f.enqueue(t, delivery("e1", playerA))
	resp, err := f.svc.Serve(context.Background(), poll(0))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Deliveries) != 1 {
		t.Fatalf("deliveries = %+v", resp)
	}
	d := resp.Deliveries[0]
	if d.Route == nil || d.Route.ExternalPlatform != telegram || d.Route.ExternalID != "100200300" ||
		d.EventID != "e1" || d.Kind != outbox.KindMechanics || d.GeneratedBy != outbox.GeneratedByRules || resp.Cursor != "1" {
		t.Errorf("delivery = %+v, cursor %q", d, resp.Cursor)
	}
}

// A delivery whose player has no link is dropped and not given; the answer is
// empty (component §8.2, §7.5).
func TestServeDropsADeliveryWithoutARoute(t *testing.T) {
	f := newFixture(t)
	f.enqueue(t, delivery("e1", "player-forgotten"))
	resp, err := f.svc.Serve(context.Background(), outbox.Poll{ClientID: bot, Platform: telegram, After: 7, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Deliveries) != 0 || resp.Cursor != "7" {
		t.Fatalf("answer = %+v, want no deliveries and the cursor of the request", resp)
	}
	if got := f.state(t, "e1", "player-forgotten"); got != outbox.StateDropped {
		t.Errorf("state = %s, want dropped", got)
	}
}

// SEC-12: the external ID of a link goes only to a client of its platform. A
// delivery whose link is of another platform is not given to this client and
// its external ID is not in the answer; a client without a platform is given
// nothing.
func TestServeGivesNoRouteOfAnotherPlatform(t *testing.T) {
	f := newFixture(t)
	f.routes.set(playerA, "ci", "fixture-player-A")
	f.enqueue(t, delivery("e1", playerA), delivery("e2", playerB))

	resp, err := f.svc.Serve(context.Background(), outbox.Poll{ClientID: "ci-harness", Limit: 10})
	if err != nil || len(resp.Deliveries) != 0 {
		t.Fatalf("a client without a platform got %+v, %v", resp, err)
	}
	resp, err = f.svc.Serve(context.Background(), poll(0))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Deliveries) != 1 || resp.Deliveries[0].PlayerID != playerB {
		t.Fatalf("deliveries = %+v, want only player-B", resp.Deliveries)
	}
	for _, d := range resp.Deliveries {
		if d.Route != nil && d.Route.ExternalID == "fixture-player-A" {
			t.Error("the external ID of a link of another platform was given")
		}
	}
	if got := f.state(t, "e1", playerA); got != outbox.StateDropped {
		t.Errorf("the delivery routed to another platform is %s, want dropped", got)
	}
}

// Serve holds no connection of gateway.db while it waits: the consumer
// enqueues through its own transaction meanwhile, and the ring after the
// enqueue wakes the long-poll within 50 ms (component §8.3).
func TestServeWaitsWithoutTheConnectionAndWakesOnTheBell(t *testing.T) {
	f := newFixture(t)
	answer := f.serve(poll(outbox.MaxWait))
	waiting(t, f)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("the waiting long-poll holds the connection: %v", err)
	}
	if _, err := f.store.Enqueue(ctx, tx, f.clock.Now(), delivery("e1", playerA)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	committed := clock.Real{}.Now()
	got := receive(t, answer, 2*time.Second)
	if got.err != nil || len(got.resp.Deliveries) != 1 {
		t.Fatalf("answer = %+v, %v", got.resp, got.err)
	}
	if woke := got.at.Sub(committed); woke > 50*time.Millisecond {
		t.Errorf("the long-poll woke %s after the enqueue, want ≤ 50 ms", woke)
	}
}

// A row that arrived without a ring — a lost notification — is found by the
// periodic wake-up of WakeEvery.
func TestServeWakesEverySecondWithoutARing(t *testing.T) {
	f := newFixture(t)
	answer := f.serve(poll(outbox.MaxWait))
	waiting(t, f)
	if _, err := f.db.ExecContext(context.Background(), `INSERT INTO deliveries (id, world_id, player_id, platform, kind,
		correlation_id, event_id, generated_by, text, state, created_at, expires_at)
		VALUES ('d-silent', 'w', ?, ?, 'mechanics', 'c', 'e-silent', 'rules', 'Промах.', 'pending', ?, ?)`,
		playerA, telegram, "2026-09-13T12:00:00.000000000Z", "2026-09-14T12:00:00.000000000Z"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-answer:
		t.Fatalf("the long-poll answered before its wake-up: %+v", got)
	default:
	}
	for range 3 {
		f.clock.Advance(outbox.WakeEvery)
		select {
		case got := <-answer:
			if got.err != nil || len(got.resp.Deliveries) != 1 || got.resp.Deliveries[0].ID != "d-silent" {
				t.Fatalf("answer = %+v, %v", got.resp, got.err)
			}
			return
		case <-clock.RealTimers{}.After(100 * time.Millisecond).C():
		}
	}
	t.Fatal("the periodic wake-up did not find the row")
}

// The wait ends with an empty answer when wait_ms is over.
func TestServeAnswersEmptyWhenTheWaitIsOver(t *testing.T) {
	f := newFixture(t)
	answer := f.serve(poll(3 * time.Second))
	waiting(t, f)
	f.clock.Advance(3 * time.Second)
	got := receive(t, answer, 2*time.Second)
	if got.err != nil || len(got.resp.Deliveries) != 0 || got.resp.Deliveries == nil {
		t.Fatalf("answer = %+v, %v; want an empty list", got.resp, got.err)
	}
}

// The long-poll does not hold the stop of the process: when the process server
// begins to stop, it answers an empty list at once, on a clock that never moves
// (component §5.1 p. 8, C-01 v1.8).
func TestServeAnswersAtOnceWhenTheProcessStops(t *testing.T) {
	f := newFixture(t)
	stop := make(chan struct{})
	p := poll(outbox.MaxWait)
	p.Stop = stop
	answer := f.serve(p)
	waiting(t, f)
	close(stop)
	got := receive(t, answer, time.Second)
	if got.err != nil || len(got.resp.Deliveries) != 0 {
		t.Fatalf("answer = %+v, %v", got.resp, got.err)
	}
}

// A request that ends ends the long-poll with its error.
func TestServeEndsWithTheRequest(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan error, 1)
	go func() {
		_, err := f.svc.Serve(ctx, poll(outbox.MaxWait))
		out <- err
	}()
	waiting(t, f)
	cancel()
	select {
	case err := <-out:
		if err == nil {
			t.Fatal("Serve answered a request that ended")
		}
	case <-clock.RealTimers{}.After(time.Second).C():
		t.Fatal("Serve did not end with its request")
	}
}

// Deliveries of narrative.output join the queue of the player in the order they
// were enqueued: a death that arrived before the text of its turn is given
// first, without a delay or a reordering (C-05 v1.4 p. 8).
func TestServeDoesNotReorderTheNarrative(t *testing.T) {
	f := newFixture(t)
	death := delivery("n-death", playerA)
	death.Kind, death.GeneratedBy, death.CorrelationID = outbox.KindNarrative, outbox.GeneratedByTemplate, "action-1"
	death.Data = map[string]any{outbox.DataKind: "death", outbox.DataNarrativeEventID: "n-death"}
	turn := death
	turn.EventID, turn.Data = "n-turn", map[string]any{outbox.DataKind: "turn", outbox.DataNarrativeEventID: "n-turn"}
	f.enqueue(t, death)
	f.enqueue(t, turn)

	var order []string
	for range 2 {
		resp, err := f.svc.Serve(context.Background(), poll(0))
		if err != nil || len(resp.Deliveries) != 1 {
			t.Fatalf("answer = %+v, %v", resp, err)
		}
		d := resp.Deliveries[0]
		if d.NarrativeEventID == nil || *d.NarrativeEventID != d.EventID {
			t.Errorf("narrative_event_id = %v, want %s", d.NarrativeEventID, d.EventID)
		}
		order = append(order, d.EventID)
		if _, err := f.svc.Ack(context.Background(), bot, []string{d.ID}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if order[0] != "n-death" || order[1] != "n-turn" {
		t.Errorf("order = %v, want the order of arrival [n-death n-turn]", order)
	}
}

// firedTimer is a timer that has already fired.
type firedTimer struct{ ch chan time.Time }

func (f firedTimer) C() <-chan time.Time { return f.ch }
func (f firedTimer) Stop() bool          { return false }

// firedTimers hands out timers that have already fired; the second one — the
// wake-up of the first round of waiting — runs onWake first.
type firedTimers struct {
	calls  int
	onWake func()
}

func (f *firedTimers) After(time.Duration) clock.Timer {
	f.calls++
	if f.calls == 2 && f.onWake != nil {
		f.onWake()
	}
	ch := make(chan time.Time, 1)
	ch <- t0
	return firedTimer{ch}
}

func (f *firedTimers) Every(d time.Duration) clock.Timer { return f.After(d) }

// A wake-up that comes together with the end of the wait answers empty and
// leases nothing more: the row that arrived meanwhile stays for the next poll.
// select picks among ready cases at random, so the round repeats (review #1 of
// T-307, N-3).
func TestServeLeasesNothingOnceTheWaitIsOver(t *testing.T) {
	f := newFixture(t)
	for i := range 20 {
		event := fmt.Sprintf("e-%d", i)
		timers := &firedTimers{onWake: func() {
			f.enqueue(t, delivery(event, playerA))
		}}
		svc, err := outbox.NewService(outbox.ServiceConfig{Store: f.store, Routes: f.routes, Clock: f.clock, Timers: timers})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := svc.Serve(context.Background(), poll(time.Second))
		if err != nil || len(resp.Deliveries) != 0 {
			t.Fatalf("round %d: answer = %+v, %v; want an empty list", i, resp, err)
		}
		if n := f.count(t, `SELECT attempts FROM deliveries WHERE event_id = ?`, event); n != 0 {
			t.Fatalf("round %d: the row was leased at the end of the wait", i)
		}
		if _, err := f.db.ExecContext(context.Background(), `DELETE FROM deliveries WHERE event_id = ?`, event); err != nil {
			t.Fatal(err)
		}
	}
}
