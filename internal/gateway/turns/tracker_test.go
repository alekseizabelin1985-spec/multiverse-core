package turns_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

func completed(t *testing.T, f *fixture) []eventbus.Event {
	t.Helper()
	return f.bus.ofType(t, eventbus.TopicAnalyticsEvents, turns.TypeCompleted)
}

func rowOf(t *testing.T, f *fixture, correlationID string) turns.Row {
	t.Helper()
	row, found, err := turns.Load(context.Background(), f.db, correlationID)
	if err != nil || !found {
		t.Fatalf("turn %s: found %v %v", correlationID, found, err)
	}
	return row
}

// An accepted action opens the session of its scope and becomes a turn of it:
// accepted, counted, with the deadline of the turn and no analytics yet.
func TestAnAcceptedActionIsATurnOfItsSession(t *testing.T) {
	f := newFixture(t, options{})
	a := accepted(t, f.submit(t, "k-1", api.ActionEnter, forest))
	if a.Turn.Seq != 1 || a.Turn.SessionID != session.ID(playerA, t0) {
		t.Fatalf("turn = %+v", a.Turn)
	}
	row := rowOf(t, f, a.CorrelationID)
	if row.Status != turns.StatusAccepted || row.TargetID != forest || row.ActionType != api.ActionEnter ||
		!row.ReceivedAt.Equal(t0) || row.ActorKind != eventbus.ActorCI {
		t.Errorf("row = %+v", row)
	}
	var deadline string
	if err := f.db.QueryRowContext(context.Background(), `SELECT deadline_at FROM turns WHERE correlation_id = ?`, a.CorrelationID).Scan(&deadline); err != nil ||
		!strings.HasPrefix(deadline, t0.Add(turns.DefaultTimeout).Format("2006-01-02T15:04:05")) {
		t.Errorf("deadline_at = %s %v", deadline, err)
	}
	s, _, _ := f.sessions.Current(context.Background(), playerA)
	if s.TurnsCount != 1 || s.ActorKind != eventbus.ActorCI {
		t.Errorf("session = %+v", s)
	}
	if got := completed(t, f); len(got) != 0 {
		t.Errorf("turn.completed before the turn ended: %d", len(got))
	}
	if started := f.bus.ofType(t, eventbus.TopicAnalyticsEvents, session.TypeStarted); len(started) != 1 {
		t.Errorf("session.started = %d", len(started))
	}
}

// A refused action of a scope with a session is published at once as
// status=rejected with received_at and acked_at only (US-038).
func TestARefusedActionIsARejectedTurn(t *testing.T) {
	f := newFixture(t, options{})
	accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
	f.clock.Advance(time.Second)
	refused := f.submit(t, "k-2", api.ActionFlee, "")
	if refused.Err == nil || refused.Err.Code != api.CodeNotInEncounter {
		t.Fatalf("flee = %+v", refused)
	}
	got := completed(t, f)
	if len(got) != 1 {
		t.Fatalf("turn.completed = %d, want the rejected one", len(got))
	}
	ev := got[0]
	if err := contracts.Validate(ev); err != nil {
		t.Errorf("rejected turn is not valid by C-10: %v", err)
	}
	pa := ev.Path()
	status, _ := pa.GetString("turn.status")
	seq, _ := pa.GetInt("turn.seq")
	action, _ := pa.GetString("turn.action_type")
	received, _ := pa.GetString("timings.received_at")
	if status != turns.OutcomeRejected || seq != 2 || action != api.ActionFlee || received != session.Timestamp(t0.Add(time.Second)) {
		t.Errorf("rejected turn = %s seq %d %s received %s", status, seq, action, received)
	}
	if !pa.Has("timings.acked_at") || pa.Has("timings.mechanics_at") || pa.Has("timings.narrative_at") || pa.Has("timings.total_ms") {
		t.Errorf("timings of a rejected turn = %v", ev.Payload["timings"])
	}
	// The repeat of the refused key answers from the key and publishes nothing.
	f.submit(t, "k-2", api.ActionFlee, "")
	if n := len(completed(t, f)); n != 1 {
		t.Errorf("the repeat of a refused key published %d turns in all", n)
	}
	// The next action does not take the number of the rejected turn.
	if a := accepted(t, f.submit(t, "k-3", api.ActionLook, "")); a.Turn.Seq != 3 {
		t.Errorf("seq after a rejected turn = %d, want 3", a.Turn.Seq)
	}
}

// A refused action of a scope without a session opens none and publishes
// nothing: a session opens with an accepted action (metrics.md §4.2).
func TestARefusedActionWithoutASessionPublishesNothing(t *testing.T) {
	f := newFixture(t, options{})
	if a := f.submit(t, "k-1", api.ActionFlee, ""); a.Err == nil {
		t.Fatalf("flee = %+v", a)
	}
	if got := f.bus.records(t, eventbus.TopicAnalyticsEvents); len(got) != 0 {
		t.Errorf("analytics = %d events", len(got))
	}
	if a := f.svc.Submit(context.Background(), actions.Command{PlayerID: "player-X", ActionKey: "k", Type: api.ActionLook}); a.Err == nil || a.Err.Code != api.CodePlayerNotFound {
		t.Fatalf("unknown player = %+v", a)
	}
	if got := f.bus.records(t, eventbus.TopicAnalyticsEvents); len(got) != 0 {
		t.Errorf("analytics after an unknown player = %d events", len(got))
	}
}

// The turn completes when the last recipient acknowledged its narrative: ok for
// a narrative of the model, degraded for a template, with the timings of the
// stages and the counters of the session (US-038).
func TestATurnCompletesAfterItsLastRecipient(t *testing.T) {
	for _, tc := range []struct {
		generatedBy, want string
		degraded          int
	}{
		{turns.GeneratedByLLM, turns.OutcomeOK, 0},
		{turns.GeneratedByTemplate, turns.OutcomeDegraded, 1},
	} {
		t.Run(tc.generatedBy, func(t *testing.T) {
			f := newFixture(t, options{})
			ctx := context.Background()
			a := accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
			action := f.actionEvent(t, a.CorrelationID)
			f.clock.Advance(200 * time.Millisecond)
			mech := eventbus.Derive(action, "combat.decided", contracts.SourceSwarm, map[string]any{"phase1_mode": "rules"}, eventbus.WithCauseID("m"))
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, mech, f.clock.Now()) })
			f.inTx(t, func(tx *sql.Tx) error {
				return f.tracker.OnNarrative(ctx, tx, narrative(t, action, tc.generatedBy, playerA, "player-B"))
			})
			f.clock.Advance(time.Second)
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, a.CorrelationID, f.clock.Now()) })
			if got := completed(t, f); len(got) != 0 {
				t.Fatalf("turn.completed after the first of two recipients: %d", len(got))
			}
			f.clock.Advance(500 * time.Millisecond)
			last := f.clock.Now()
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, a.CorrelationID, last) })
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, a.CorrelationID, last.Add(time.Second)) })

			got := completed(t, f)
			if len(got) != 1 {
				t.Fatalf("turn.completed = %d, want 1", len(got))
			}
			ev := got[0]
			if err := contracts.Validate(ev); err != nil {
				t.Errorf("turn.completed is not valid by C-10: %v", err)
			}
			if ev.Meta.CausationID != a.CorrelationID || ev.Meta.CorrelationID != a.CorrelationID || ev.Scope != nil {
				t.Errorf("envelope = %+v", ev.Meta)
			}
			pa := ev.Path()
			status, _ := pa.GetString("turn.status")
			narrativeAt, _ := pa.GetString("timings.narrative_at")
			mechanicsMS, _ := pa.GetInt("timings.mechanics_ms")
			totalMS, _ := pa.GetInt("timings.total_ms")
			recipients, _ := pa.GetInt("delivery.recipients_count")
			delivered, _ := pa.GetInt("delivery.delivered_count")
			generated, _ := pa.GetString("narrative.generated_by")
			if status != tc.want || narrativeAt != session.Timestamp(last) || mechanicsMS != 200 || totalMS != 1700 ||
				recipients != 2 || delivered != 2 || generated != tc.generatedBy {
				t.Errorf("turn.completed = status %s narrative_at %s mechanics %d total %d delivery %d/%d by %s",
					status, narrativeAt, mechanicsMS, totalMS, delivered, recipients, generated)
			}
			raw, _ := json.Marshal(ev)
			if strings.Contains(string(raw), "Вася") || strings.Contains(string(raw), "Волк рычит") {
				t.Errorf("turn.completed carries a name or a text: %s", raw)
			}
			s, _, _ := f.sessions.Current(ctx, playerA)
			if s.TurnsDegraded != tc.degraded {
				t.Errorf("turns_degraded = %d, want %d", s.TurnsDegraded, tc.degraded)
			}
		})
	}
}

// A turn whose narrative did not reach its recipients by the deadline times
// out once, with status=timeout, and counts as failed.
func TestATurnPastItsDeadlineTimesOut(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	a := accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
	if n, err := f.tracker.Sweep(ctx, t0.Add(turns.DefaultTimeout-time.Nanosecond)); err != nil || n != 0 {
		t.Fatalf("Sweep before the deadline = %d %v", n, err)
	}
	if n, err := f.tracker.Sweep(ctx, t0.Add(turns.DefaultTimeout)); err != nil || n != 1 {
		t.Fatalf("Sweep at the deadline = %d %v, want 1", n, err)
	}
	if n, err := f.tracker.Sweep(ctx, t0.Add(2*turns.DefaultTimeout)); err != nil || n != 0 {
		t.Fatalf("second Sweep = %d %v", n, err)
	}
	got := completed(t, f)
	if len(got) != 1 {
		t.Fatalf("turn.completed = %d", len(got))
	}
	if status, _ := got[0].Path().GetString("turn.status"); status != turns.OutcomeTimeout {
		t.Errorf("status = %s", status)
	}
	if err := contracts.Validate(got[0]); err != nil {
		t.Errorf("timed out turn is not valid by C-10: %v", err)
	}
	if row := rowOf(t, f, a.CorrelationID); row.Status != turns.StatusTimeout {
		t.Errorf("row status = %s", row.Status)
	}
	// A late acknowledgement changes nothing.
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, a.CorrelationID, f.clock.Now()) })
	if n := len(completed(t, f)); n != 1 {
		t.Errorf("a late acknowledgement published again: %d", n)
	}
	s, _, _ := f.sessions.Current(ctx, playerA)
	if s.TurnsFailed != 1 {
		t.Errorf("turns_failed = %d", s.TurnsFailed)
	}
}

// A turn whose narrative reached some of its recipients by the deadline, not
// all, times out too (N-1 of review #1 of T-306): the status narrated is open.
func TestANarratedTurnPastItsDeadlineTimesOut(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	a := accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
	action := f.actionEvent(t, a.CorrelationID)
	f.inTx(t, func(tx *sql.Tx) error {
		return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByLLM, playerA, "player-B"))
	})
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, a.CorrelationID, f.clock.Now()) })
	if row := rowOf(t, f, a.CorrelationID); row.Status != turns.StatusNarrated || row.DeliveredCount != 1 || row.RecipientsCount != 2 {
		t.Fatalf("turn before the deadline = %+v", row)
	}
	if n, err := f.tracker.Sweep(ctx, t0.Add(turns.DefaultTimeout)); err != nil || n != 1 {
		t.Fatalf("Sweep at the deadline = %d %v, want the narrated turn", n, err)
	}
	got := completed(t, f)
	if len(got) != 1 {
		t.Fatalf("turn.completed = %d", len(got))
	}
	pa := got[0].Path()
	status, _ := pa.GetString("turn.status")
	delivered, _ := pa.GetInt("delivery.delivered_count")
	recipients, _ := pa.GetInt("delivery.recipients_count")
	if status != turns.OutcomeTimeout || delivered != 1 || recipients != 2 {
		t.Errorf("turn.completed = %s, delivered %d of %d", status, delivered, recipients)
	}
	if row := rowOf(t, f, a.CorrelationID); row.Status != turns.StatusTimeout {
		t.Errorf("row status = %s", row.Status)
	}
}

// The sweeper of turns keeps the reserved number of an active session (Mi-1 of
// review #1 of T-306): a batch that fails, a sweep, another action of the
// scope, then the repeat of the first key — two turns, two numbers, both
// recorded. The reserve of a session that ended is dropped.
func TestTheSweeperKeepsTheReservedNumbersOfActiveSessions(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	f.bus.set(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == actions.TypeLooked {
			return errors.New("broker down")
		}
		return nil
	}, nil)
	if a := f.submit(t, "k-1", api.ActionLook, ""); a.Status != http.StatusServiceUnavailable {
		t.Fatalf("look with the broker down = %d", a.Status)
	}
	f.bus.set(nil, nil)
	if _, err := f.tracker.Sweep(ctx, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if n := turns.Reserves(f.tracker); n != 1 {
		t.Fatalf("reserves after a sweep = %d, want the one of the active session", n)
	}
	other := accepted(t, f.submit(t, "k-2", api.ActionEnter, forest))
	repeat := accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
	if repeat.Turn.Seq != 1 || other.Turn.Seq != 2 {
		t.Fatalf("seq after a sweep: the other action %d, the repeat %d, want 2 and 1", other.Turn.Seq, repeat.Turn.Seq)
	}
	rowOf(t, f, other.CorrelationID)
	rowOf(t, f, repeat.CorrelationID)

	f.clock.Advance(session.DefaultIdle)
	if n, err := f.sessions.Sweep(ctx, f.clock.Now()); err != nil || n != 1 {
		t.Fatalf("sessions.Sweep = %d %v", n, err)
	}
	if _, err := f.tracker.Sweep(ctx, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if n := turns.Reserves(f.tracker); n != 0 {
		t.Errorf("reserves after the session ended = %d, want 0", n)
	}
}

// The sweeper reads the active sessions under the lock of the reserves (Mi-1 of
// review #1 of T-306): a session that opens while the sweeper lists the active
// sessions reserves its first number after the cleanup and keeps it.
func TestASessionOpenedWhileTheSweeperListsKeepsItsNumber(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	other := actions.Turn{Scope: eventbus.ScopeRef{ID: "player-B", Type: "solo"}, WorldID: world, PlayerID: "player-B",
		Type: api.ActionLook, ActorKind: eventbus.ActorCI, At: t0}
	begun := make(chan api.TurnRef, 1)
	turns.SetListed(f.tracker, func() {
		go func() {
			ref, err := f.tracker.Begin(ctx, other)
			if err != nil {
				t.Error(err)
			}
			begun <- ref
		}()
		// Begin opens its session at once; its reserve waits for the lock the
		// sweeper holds. Without that lock it reserves within this wait, and
		// the cleanup below drops the reserve.
		for i := 0; i < 30 && len(begun) == 0; i++ {
			time.Sleep(10 * time.Millisecond)
		}
	})
	if _, err := f.tracker.Sweep(ctx, t0); err != nil {
		t.Fatal(err)
	}
	turns.SetListed(f.tracker, nil)
	first := <-begun
	second, err := f.tracker.Begin(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	if first.Seq != 1 || second.Seq != 2 || first.SessionID != second.SessionID {
		t.Errorf("turns of a session opened during a sweep = %+v and %+v, want seq 1 and 2 of one session", first, second)
	}
}

// In replay the turns are kept and nothing is published (C-10).
func TestReplayPublishesNoTurns(t *testing.T) {
	f := newFixture(t, options{replay: true})
	accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
	f.submit(t, "k-2", api.ActionFlee, "")
	if _, err := f.tracker.Sweep(context.Background(), t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := f.bus.records(t, eventbus.TopicAnalyticsEvents); len(got) != 0 {
		t.Errorf("replay published %d analytics", len(got))
	}
}

// Mi-6 of T-305 with the tracker of gateway.db: the budget of the decision runs
// out between the last publication and Turns.Accepted. The turn is recorded all
// the same, the repeat of the key answers the kept 202, and the action is not
// published twice.
func TestTheTurnIsRecordedWhenTheBudgetRunsOutAfterTheLastPublication(t *testing.T) {
	f := newFixture(t, options{})
	f.bus.set(nil, func(ctx context.Context, ev eventbus.Event) {
		if ev.Type != actions.TypeUpdateProposed {
			return
		}
		f.clock.Advance(api.RequestTimeout)
		<-ctx.Done()
	})
	first := accepted(t, f.submit(t, "k-rest", api.ActionRest, ""))
	f.bus.set(nil, nil)
	if row := rowOf(t, f, first.CorrelationID); row.Status != turns.StatusAccepted {
		t.Fatalf("turn = %+v", row)
	}
	again := accepted(t, f.submit(t, "k-rest", api.ActionRest, ""))
	if again.CorrelationID != first.CorrelationID || !again.AckedAt.Equal(first.AckedAt) || again.Turn != first.Turn {
		t.Errorf("repeat = %+v, want the kept %+v", again, first)
	}
	if n := len(f.bus.ofType(t, eventbus.TopicPlayerEvents, actions.TypeRested)); n != 1 {
		t.Errorf("player.rested published %d times", n)
	}
}

// The number of a turn is reserved when the action begins: a batch that fails,
// another action of the scope, then the repeat of the first key — two turns,
// two numbers.
func TestTheNumberOfATurnIsReservedAtItsBeginning(t *testing.T) {
	f := newFixture(t, options{})
	f.bus.set(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == actions.TypeLooked {
			return errors.New("broker down")
		}
		return nil
	}, nil)
	if a := f.submit(t, "k-1", api.ActionLook, ""); a.Status != http.StatusServiceUnavailable {
		t.Fatalf("look with the broker down = %d", a.Status)
	}
	f.bus.set(nil, nil)
	other := accepted(t, f.submit(t, "k-2", api.ActionEnter, forest))
	repeat := accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
	if other.Turn.Seq == repeat.Turn.Seq || repeat.Turn.Seq != 1 || other.Turn.Seq != 2 {
		t.Fatalf("seq: the other action %d, the repeat %d", other.Turn.Seq, repeat.Turn.Seq)
	}
	rowOf(t, f, other.CorrelationID)
	rowOf(t, f, repeat.CorrelationID)
}

// A restarted process continues the session it finds and numbers its turns
// after the highest number gateway.db holds.
func TestARestartContinuesTheNumbersOfTheSession(t *testing.T) {
	f := newFixture(t, options{})
	for _, key := range []string{"k-1", "k-2"} {
		accepted(t, f.submit(t, key, api.ActionLook, ""))
	}
	f.build(t, options{})
	a := accepted(t, f.submit(t, "k-3", api.ActionLook, ""))
	if a.Turn.Seq != 3 || a.Turn.SessionID != session.ID(playerA, t0) {
		t.Errorf("turn after a restart = %+v", a.Turn)
	}
}

// A session whose start cannot be published answers 503 bus_unavailable, like
// an action whose own event does not go out, and nothing is kept.
func TestABusThatRefusesTheStartOfASessionRefusesTheAction(t *testing.T) {
	f := newFixture(t, options{})
	f.bus.set(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == session.TypeStarted {
			return errors.New("broker down")
		}
		return nil
	}, nil)
	a := f.submit(t, "k-1", api.ActionLook, "")
	if a.Err == nil || a.Err.Code != api.CodeBusUnavailable {
		t.Fatalf("answer = %+v", a)
	}
	if n := len(f.bus.records(t, eventbus.TopicPlayerEvents)); n != 0 {
		t.Errorf("player events = %d", n)
	}
	if _, found, _ := f.keys.Lookup(context.Background(), playerA, "k-1", f.clock.Now()); found {
		t.Error("the key is kept")
	}
}

func TestNewChecksItsConfig(t *testing.T) {
	if _, err := turns.New(turns.Config{}); err == nil {
		t.Error("New without its dependencies succeeded")
	}
	f := newFixture(t, options{})
	if _, err := turns.New(turns.Config{DB: f.db, Sessions: f.sessions, Clock: f.clock, Timeout: -time.Second}); err == nil {
		t.Error("New with a negative timeout succeeded")
	}
	if _, err := f.tracker.Begin(context.Background(), actions.Turn{}); err == nil {
		t.Error("Begin without a scope succeeded")
	}
}
