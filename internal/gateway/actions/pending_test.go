package actions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/eventbus"
)

// count is how many of events have the type.
func count(events []eventbus.Event, typ string) int {
	n := 0
	for _, ev := range events {
		if ev.Type == typ {
			n++
		}
	}
	return n
}

// idsOf are the ids of the events of the type, in order.
func idsOf(events []eventbus.Event, typ string) []string {
	var out []string
	for _, ev := range events {
		if ev.Type == typ {
			out = append(out, ev.ID)
		}
	}
	return out
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func keyStored(t *testing.T, f fixture, playerID, key string) bool {
	t.Helper()
	_, found, err := f.keys.Lookup(context.Background(), playerID, key, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// P1 of review #1. A publication that fails partway answers 503 and keeps
// nothing under the key; the repeat of the key publishes the rest of the same
// batch — the same ids and bytes, from the event that failed — and no second
// action. Whatever failed may have reached the broker unacknowledged, so the
// event that failed is published again with its id.
func TestTheRepeatOfAHalfPublishedActionFinishesItsBatch(t *testing.T) {
	cases := []struct {
		name, gmPath, failing string
	}{
		{"agent, the action fails", eventbus.GMPathAgent, actions.TypeEnteredRegion},
		{"agent, the proposal fails", eventbus.GMPathAgent, actions.TypeUpdateProposed},
		{"legacy, the action fails after gm.created", eventbus.GMPathLegacy, actions.TypeEnteredRegion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, options{gmPath: tc.gmPath})
			f.bus.setFail(func(ev eventbus.Event) bool { return ev.Type == tc.failing })
			cmd := actions.Command{PlayerID: playerA, ActionKey: "k-enter", Type: api.ActionEnter, Target: forest}
			if a := submit(t, f, cmd); a.Status != http.StatusServiceUnavailable || code(a) != api.CodeBusUnavailable {
				t.Fatalf("answer = %d %s, want 503 bus_unavailable", a.Status, code(a))
			}
			if keyStored(t, f, playerA, "k-enter") || f.svc.Pending() != 1 {
				t.Fatalf("after the failure: key stored %v, pending %d; want no key and one batch", keyStored(t, f, playerA, "k-enter"), f.svc.Pending())
			}
			firstTry := f.bus.attempted()

			f.bus.setFail(nil)
			a := submit(t, f, cmd)
			if a.Status != http.StatusAccepted {
				t.Fatalf("repeat = %d %s", a.Status, code(a))
			}
			published := f.bus.published()
			for _, typ := range []string{actions.TypeGMCreated, actions.TypeEnteredRegion, actions.TypeUpdateProposed} {
				want := 1
				if typ == actions.TypeGMCreated && tc.gmPath == eventbus.GMPathAgent {
					want = 0
				}
				if n := count(published, typ); n != want {
					t.Errorf("%s published %d times, want %d", typ, n, want)
				}
			}
			// The event that failed goes out as it was built the first time.
			failed := firstTry[len(firstTry)-1]
			for _, ev := range published {
				if ev.Type == failed.Type && (ev.ID != failed.ID || !bytes.Equal(mustJSON(t, ev), mustJSON(t, failed))) {
					t.Errorf("the repeat published %s as %s, the first try built %s", ev.Type, mustJSON(t, ev), mustJSON(t, failed))
				}
			}
			if ids := idsOf(published, actions.TypeEnteredRegion); len(ids) != 1 || a.Accepted.CorrelationID != ids[0] {
				t.Errorf("correlation_id %s, player.* ids %v", a.Accepted.CorrelationID, ids)
			}
			if a.Accepted.Turn.Seq != 1 || len(f.turns.accepted) != 1 {
				t.Errorf("turn %+v, accepted turns %v", a.Accepted.Turn, f.turns.accepted)
			}
			if !keyStored(t, f, playerA, "k-enter") || f.svc.Pending() != 0 {
				t.Errorf("after the repeat: key stored %v, pending %d", keyStored(t, f, playerA, "k-enter"), f.svc.Pending())
			}
			before := len(published)
			if again := submit(t, f, cmd); again.Accepted == nil || again.Accepted.CorrelationID != a.Accepted.CorrelationID ||
				len(f.bus.published()) != before {
				t.Errorf("a third request = %+v, published %d more", again, len(f.bus.published())-before)
			}
		})
	}
}

// P2 of review #1, the hang-up. The request of an action ends once player.* is
// out — the client hung up, or the request timed out — and the action is
// finished all the same: the proposal goes, the key is kept, 202.
func TestAHangUpDoesNotCutTheBatch(t *testing.T) {
	f := newFixture(t, options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.bus.setHooks(nil, func(ev eventbus.Event) {
		if ev.Type == actions.TypeEnteredRegion {
			cancel()
		}
	})
	a := f.svc.Submit(ctx, actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionEnter, Target: forest, ActorKind: api.ActorHuman})
	if a.Status != http.StatusAccepted {
		t.Fatalf("answer = %d %s, want 202", a.Status, code(a))
	}
	if got := types(f.bus.published()); len(got) != 2 {
		t.Errorf("published %v, want the action and its proposal", got)
	}
	if !keyStored(t, f, playerA, "k") || f.svc.Pending() != 0 {
		t.Errorf("key stored %v, pending %d", keyStored(t, f, playerA, "k"), f.svc.Pending())
	}
}

// P2 of review #1, the budget. A decision that runs out of its budget after
// player.* — a broker that holds the proposal — answers 503 and holds the
// batch; the repeat publishes the proposal with its id and no second action.
func TestABudgetThatRunsOutHoldsTheBatch(t *testing.T) {
	f := newFixture(t, options{})
	hanging := make(chan struct{})
	f.bus.setHooks(func(ctx context.Context, ev eventbus.Event) error {
		if ev.Type != actions.TypeUpdateProposed {
			return nil
		}
		close(hanging)
		<-ctx.Done()
		return context.Cause(ctx)
	}, nil)
	cmd := actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionEnter, Target: forest, ActorKind: api.ActorHuman}
	answer := make(chan actions.Answer, 1)
	go func() { answer <- f.svc.Submit(context.Background(), cmd) }()
	<-hanging
	f.clock.Advance(api.RequestTimeout)
	if a := <-answer; code(a) != api.CodeBusUnavailable {
		t.Fatalf("answer = %d %s, want 503 bus_unavailable", a.Status, code(a))
	}
	held := f.bus.attempted()[1]

	f.bus.setHooks(nil, nil)
	a := submit(t, f, cmd)
	if a.Status != http.StatusAccepted {
		t.Fatalf("repeat = %d %s", a.Status, code(a))
	}
	published := f.bus.published()
	if got := types(published); len(got) != 2 || count(published, actions.TypeEnteredRegion) != 1 {
		t.Fatalf("published %v, want one action and one proposal", got)
	}
	if published[1].ID != held.ID {
		t.Errorf("the proposal went out as %s, the batch held %s", published[1].ID, held.ID)
	}
}

// P3 of review #1. The request ends after the last publication: the turn and
// the key are recorded all the same, and the repeat answers the kept 202.
func TestAHangUpAfterTheLastPublicationKeepsTheKey(t *testing.T) {
	f := newFixture(t, options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.bus.setHooks(nil, func(ev eventbus.Event) {
		if ev.Type == actions.TypeUpdateProposed {
			cancel()
		}
	})
	cmd := actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionRest, ActorKind: api.ActorHuman}
	first := f.svc.Submit(ctx, cmd)
	if first.Status != http.StatusAccepted {
		t.Fatalf("answer = %d %s", first.Status, code(first))
	}
	if !keyStored(t, f, playerA, "k") || len(f.turns.accepted) != 1 {
		t.Fatalf("after a hang-up: key stored %v, accepted turns %v", keyStored(t, f, playerA, "k"), f.turns.accepted)
	}
	f.bus.setHooks(nil, nil)
	again := submit(t, f, cmd)
	if again.Accepted == nil || again.Accepted.CorrelationID != first.Accepted.CorrelationID ||
		!again.Accepted.AckedAt.Equal(first.Accepted.AckedAt) || len(f.bus.published()) != 2 {
		t.Errorf("repeat = %+v, published %d", again.Accepted, len(f.bus.published()))
	}
}

// The residual risk the card of T-305 records: a batch is held in memory, so a
// process started again between a failed publication and its repeat builds
// the action again — a second player.* with a new id.
func TestARestartBetweenPublicationsBuildsTheActionAgain(t *testing.T) {
	f := newFixture(t, options{})
	f.bus.setFail(func(ev eventbus.Event) bool { return ev.Type == actions.TypeUpdateProposed })
	cmd := actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionEnter, Target: forest, ActorKind: api.ActorHuman}
	if a := submit(t, f, cmd); code(a) != api.CodeBusUnavailable {
		t.Fatalf("answer = %d %s", a.Status, code(a))
	}
	f.bus.setFail(nil)
	restarted := f.newService(t, options{})
	if a := restarted.Submit(context.Background(), cmd); a.Status != http.StatusAccepted {
		t.Fatalf("repeat on the restarted service = %d %s", a.Status, code(a))
	}
	ids := idsOf(f.bus.published(), actions.TypeEnteredRegion)
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Errorf("player.* ids %v: the restart is expected to publish the action again with a new id", ids)
	}
}

// The batches in memory are bounded: at the limit the one closest to its
// expiry goes, with a warning that names the player and not the key, and its
// repeat builds the action again. A batch past the expiry of its key is swept,
// and a repeat after it is a new action.
func TestHalfPublishedActionsAreBoundedAndExpire(t *testing.T) {
	f := newFixture(t, options{pendingLimit: 2})
	f.bus.setFail(func(ev eventbus.Event) bool { return ev.Type == actions.TypeUpdateProposed })
	for _, key := range []string{"k-1", "k-2", "k-3"} {
		if a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: key, Type: api.ActionRest}); code(a) != api.CodeBusUnavailable {
			t.Fatalf("%s = %d %s", key, a.Status, code(a))
		}
		f.clock.Advance(time.Second)
	}
	if n := f.svc.Pending(); n != 2 {
		t.Fatalf("pending = %d, want the limit 2", n)
	}
	warnings := f.log.lines(t, "half published action dropped from memory at the limit")
	if len(warnings) != 1 || warnings[0]["player_id"] != playerA || strings.Contains(f.log.String(), "k-1") {
		t.Errorf("warnings = %v", warnings)
	}

	f.bus.setFail(nil)
	for _, key := range []string{"k-3", "k-1"} {
		if a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: key, Type: api.ActionRest}); a.Status != http.StatusAccepted {
			t.Fatalf("repeat %s = %d %s", key, a.Status, code(a))
		}
	}
	// k-3 was held and publishes its proposal only; k-1 was dropped and
	// publishes its action again.
	if got := types(f.bus.published()); strings.Join(got, ",") != strings.Join([]string{
		actions.TypeRested, actions.TypeRested, actions.TypeRested, actions.TypeUpdateProposed,
		actions.TypeRested, actions.TypeUpdateProposed,
	}, ",") {
		t.Errorf("published %v", got)
	}

	f.clock.Advance(store.KeyTTL - 3*time.Second)
	if n := f.svc.SweepPending(f.clock.Now()); n != 0 || f.svc.Pending() != 1 {
		t.Fatalf("sweep before the expiry dropped %d, pending %d", n, f.svc.Pending())
	}
	f.clock.Advance(time.Second)
	if n := f.svc.SweepPending(f.clock.Now()); n != 1 || f.svc.Pending() != 0 {
		t.Errorf("sweep at the expiry dropped %d, pending %d", n, f.svc.Pending())
	}
}

// A repeat that comes after the expiry of its key, before the sweeper, is a
// new action too.
func TestABatchPastItsExpiryIsNotPublished(t *testing.T) {
	f := newFixture(t, options{})
	f.bus.setFail(func(ev eventbus.Event) bool { return ev.Type == actions.TypeUpdateProposed })
	cmd := actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionRest}
	submit(t, f, cmd)
	f.bus.setFail(nil)
	f.clock.Advance(store.KeyTTL)
	if a := submit(t, f, cmd); a.Status != http.StatusAccepted {
		t.Fatalf("repeat = %d %s", a.Status, code(a))
	}
	if ids := idsOf(f.bus.published(), actions.TypeRested); len(ids) != 2 || f.svc.Pending() != 0 {
		t.Errorf("player.rested ids %v, pending %d: want the action built again", ids, f.svc.Pending())
	}
}

// N-5 of review #1. A request that ends while an earlier request of its player
// holds the lock decides nothing and answers the repeatable 503, not 500; the
// repeat gets the answer the earlier request kept.
func TestARequestThatGivesUpWaitingForItsPlayerDecidesNothing(t *testing.T) {
	f := newFixture(t, options{})
	holding, release := make(chan struct{}), make(chan struct{})
	f.bus.setHooks(func(context.Context, eventbus.Event) error {
		close(holding)
		<-release
		return nil
	}, nil)
	cmd := actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionLook, ActorKind: api.ActorHuman}
	first := make(chan actions.Answer, 1)
	go func() { first <- f.svc.Submit(context.Background(), cmd) }()
	<-holding

	gone, cancel := context.WithCancel(context.Background())
	cancel()
	if a := f.svc.Submit(gone, cmd); a.Status != http.StatusServiceUnavailable || code(a) != api.CodeBusUnavailable {
		t.Errorf("a request that gave up = %d %s, want 503 bus_unavailable", a.Status, code(a))
	}
	f.bus.setHooks(nil, nil)
	close(release)
	accepted := <-first
	if accepted.Status != http.StatusAccepted {
		t.Fatalf("the first request = %d %s", accepted.Status, code(accepted))
	}
	// N-6 of review #2: the request that gave up left the table of locks; had
	// it not, the entry of the player would outlive every request.
	if n := actions.PlayerLocks(f.svc); n != 0 {
		t.Errorf("after both requests: %d entries of player locks, want 0", n)
	}
	if again := submit(t, f, cmd); again.Accepted == nil || again.Accepted.CorrelationID != accepted.Accepted.CorrelationID {
		t.Errorf("repeat = %+v", again)
	}
	if n := len(f.bus.published()); n != 1 {
		t.Errorf("published %d, want 1", n)
	}
	if n := actions.PlayerLocks(f.svc); n != 0 {
		t.Errorf("after the repeat: %d entries of player locks, want 0", n)
	}
}

// Mi-6 of review #2, probe P4. The budget of the decision runs out between the
// last publication and the key: the turn and the key are recorded all the
// same, and the repeat answers the kept 202 without a second player.*.
func TestABudgetThatRunsOutAfterTheLastPublicationKeepsTheKey(t *testing.T) {
	f := newFixture(t, options{})
	var decision context.Context
	f.bus.setHooks(func(ctx context.Context, ev eventbus.Event) error {
		if ev.Type == actions.TypeUpdateProposed {
			decision = ctx
		}
		return nil
	}, func(ev eventbus.Event) {
		if ev.Type != actions.TypeUpdateProposed {
			return
		}
		f.clock.Advance(api.RequestTimeout)
		<-decision.Done()
	})
	cmd := actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionRest, ActorKind: api.ActorHuman}
	first := submit(t, f, cmd)
	if first.Status != http.StatusAccepted {
		t.Fatalf("answer = %d %s, want 202", first.Status, code(first))
	}
	if cause := context.Cause(decision); cause == nil {
		t.Fatal("the budget of the decision did not run out after the last publication")
	}
	if !keyStored(t, f, playerA, "k") || len(f.turns.accepted) != 1 {
		t.Fatalf("after the budget ran out: key stored %v, accepted turns %v; want the key and one turn",
			keyStored(t, f, playerA, "k"), f.turns.accepted)
	}
	if err := f.turns.acceptedOn[0]; err != nil {
		t.Errorf("the turn was recorded on a context that is done (%v); a turn in gateway.db would be lost", err)
	}
	f.bus.setHooks(nil, nil)
	again := submit(t, f, cmd)
	if again.Accepted == nil || again.Accepted.CorrelationID != first.Accepted.CorrelationID ||
		!again.Accepted.AckedAt.Equal(first.Accepted.AckedAt) {
		t.Errorf("repeat = %+v, want the kept 202 %+v", again.Accepted, first.Accepted)
	}
	if n := count(f.bus.published(), actions.TypeRested); n != 1 {
		t.Errorf("player.rested published %d times, want 1", n)
	}
}
