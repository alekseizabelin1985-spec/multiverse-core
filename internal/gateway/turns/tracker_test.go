package turns_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
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
				return f.tracker.OnNarrative(ctx, tx, narrative(t, action, tc.generatedBy, playerA, "player-B"), 2)
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
		return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByLLM, playerA, "player-B"), 2)
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

// Mi-6 of T-305 with the tracker of gateway.db, in the order of T-308: the turn
// is recorded before the action goes out, and the budget of the decision runs
// out between the last publication and Turns.Acked. acked_at is recorded all
// the same — on the context of the decision the update would fail and leave the
// moment of the record before the publication (review #2 of T-308, Mi-1) — the
// repeat of the key answers the kept 202, and the action is not published
// twice.
func TestTheAcknowledgementIsRecordedWhenTheBudgetRunsOutAfterTheLastPublication(t *testing.T) {
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
	if row := rowOf(t, f, first.CorrelationID); row.Status != turns.StatusAccepted || !row.AckedAt.Equal(first.AckedAt) ||
		!first.AckedAt.Equal(t0.Add(api.RequestTimeout)) {
		t.Fatalf("turn = %+v, answer acked_at %s; want acked_at of the answer, after the budget", row, first.AckedAt)
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

// only returns the one turn.completed of a run, or fails.
func only(t *testing.T, f *fixture) eventbus.Event {
	t.Helper()
	got := completed(t, f)
	if len(got) != 1 {
		t.Fatalf("turn.completed = %d, want 1", len(got))
	}
	if err := contracts.Validate(got[0]); err != nil {
		t.Errorf("turn.completed is not valid by C-10: %v", err)
	}
	return got[0]
}

// mechanicsFields are the fields of turn.completed a turn has only with its
// mechanics.
var mechanicsFields = []string{"timings.mechanics_at", "timings.mechanics_ms", "delivery.result_event_id", "turn.phase1_mode", "turn.lod"}

// A turn of an action with Phase 1 whose narrative is acknowledged before its
// mechanics comes is not completed by the acknowledgement: the mechanics
// completes it, once, with every field of the mechanics and the narrative_at of
// the acknowledgement (component §7.7; case 1 of architect#3, T-478).
func TestAnAttackAcknowledgedBeforeItsMechanicsCompletesWithIt(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	action := f.begun(t, api.ActionAttack)
	f.clock.Advance(100 * time.Millisecond)
	f.inTx(t, func(tx *sql.Tx) error {
		return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByTemplate, playerA), 1)
	})
	f.clock.Advance(time.Second)
	acked := f.clock.Now()
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, acked) })
	if n := len(completed(t, f)); n != 0 {
		t.Fatalf("turn.completed of an attack without its mechanics: %d", n)
	}
	if row := rowOf(t, f, action.ID); row.Status != turns.StatusNarrated || row.NarrativeAt == nil || !row.NarrativeAt.Equal(acked) {
		t.Fatalf("attack acknowledged without its mechanics = %+v, want narrated with narrative_at %s", row, acked)
	}

	f.clock.Advance(300 * time.Millisecond)
	mech := decided(action, "m")
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, mech, f.clock.Now()) })
	pa := only(t, f).Path()
	status, _ := pa.GetString("turn.status")
	mechanicsAt, _ := pa.GetString("timings.mechanics_at")
	mechanicsMS, _ := pa.GetInt("timings.mechanics_ms")
	narrativeAt, _ := pa.GetString("timings.narrative_at")
	narrativeMS, _ := pa.GetInt("timings.narrative_ms")
	totalMS, _ := pa.GetInt("timings.total_ms")
	result, _ := pa.GetString("delivery.result_event_id")
	mode, _ := pa.GetString("turn.phase1_mode")
	lod, _ := pa.GetString("turn.lod")
	delivered, _ := pa.GetInt("delivery.delivered_count")
	if status != turns.OutcomeDegraded || mechanicsAt != session.Timestamp(t0.Add(1400*time.Millisecond)) || mechanicsMS != 1400 ||
		narrativeAt != session.Timestamp(acked) || narrativeMS != 0 || totalMS != 1100 || result != mech.ID ||
		mode != "rules" || lod != "full" || delivered != 1 {
		t.Errorf("turn.completed = %s mechanics %s (%d ms) narrative %s (%d ms) total %d result %q mode %q lod %q delivered %d",
			status, mechanicsAt, mechanicsMS, narrativeAt, narrativeMS, totalMS, result, mode, lod, delivered)
	}
	if row := rowOf(t, f, action.ID); row.Status != turns.StatusDegraded {
		t.Errorf("row status = %s", row.Status)
	}
	if s, _, _ := f.sessions.Current(ctx, playerA); s.TurnsDegraded != 1 || s.TurnsFailed != 0 {
		t.Errorf("session degraded %d failed %d, want 1 and 0", s.TurnsDegraded, s.TurnsFailed)
	}
}

// Only the actions with Phase 1 — enter, leave, attack, flee, rest — wait for
// their mechanics; look, say and defend complete by the acknowledgement as
// before (api-contracts.md §1.4; case 2 of architect#3, T-478).
func TestOnlyATurnWithPhase1WaitsForItsMechanics(t *testing.T) {
	for _, tc := range []struct {
		action string
		waits  bool
	}{
		{api.ActionEnter, true}, {api.ActionLeave, true}, {api.ActionAttack, true}, {api.ActionFlee, true},
		{api.ActionRest, true}, {api.ActionLook, false}, {api.ActionSay, false}, {api.ActionDefend, false},
	} {
		t.Run(tc.action, func(t *testing.T) {
			f := newFixture(t, options{})
			ctx := context.Background()
			action := f.begun(t, tc.action)
			f.inTx(t, func(tx *sql.Tx) error {
				return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByLLM, playerA), 1)
			})
			f.clock.Advance(time.Second)
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, f.clock.Now()) })
			if got := len(completed(t, f)); (got == 0) != tc.waits {
				t.Fatalf("turn.completed after the acknowledgement without mechanics = %d, waits for the mechanics %v", got, tc.waits)
			}
			if tc.waits {
				return
			}
			pa := only(t, f).Path()
			for _, field := range mechanicsFields {
				if pa.Has(field) {
					t.Errorf("turn.completed of %s without mechanics has %s", tc.action, field)
				}
			}
		})
	}
}

// A rest whose narrative reached the player and whose mechanics never came —
// State refused the proposal — completes at its deadline with the status of
// its narrative, without the fields of the mechanics, logged, and not as a
// failure (component §7.7; case 3 of architect#3, T-478).
func TestADeliveredTurnWithoutItsMechanicsCompletesAtItsDeadline(t *testing.T) {
	var log strings.Builder
	f := newFixture(t, options{log: slog.New(slog.NewJSONHandler(&log, nil))})
	ctx := context.Background()
	action := f.begun(t, api.ActionRest)
	f.inTx(t, func(tx *sql.Tx) error {
		return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByTemplate, playerA), 1)
	})
	f.clock.Advance(2 * time.Second)
	acked := f.clock.Now()
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, acked) })

	if n, err := f.tracker.Sweep(ctx, t0.Add(turns.DefaultTimeout-time.Nanosecond)); err != nil || n != 0 || len(completed(t, f)) != 0 {
		t.Fatalf("Sweep before the deadline = %d %v, published %d", n, err, len(completed(t, f)))
	}
	if n, err := f.tracker.Sweep(ctx, t0.Add(turns.DefaultTimeout)); err != nil || n != 1 {
		t.Fatalf("Sweep at the deadline = %d %v, want 1", n, err)
	}
	pa := only(t, f).Path()
	status, _ := pa.GetString("turn.status")
	narrativeAt, _ := pa.GetString("timings.narrative_at")
	if status != turns.OutcomeDegraded || narrativeAt != session.Timestamp(acked) {
		t.Errorf("turn.completed = %s narrative_at %s, want degraded at %s", status, narrativeAt, session.Timestamp(acked))
	}
	for _, field := range mechanicsFields {
		if pa.Has(field) {
			t.Errorf("turn.completed without mechanics has %s", field)
		}
	}
	if s, _, _ := f.sessions.Current(ctx, playerA); s.TurnsFailed != 0 || s.TurnsDegraded != 1 {
		t.Errorf("session failed %d degraded %d, want 0 and 1", s.TurnsFailed, s.TurnsDegraded)
	}
	if !strings.Contains(log.String(), `"level":"WARN"`) || !strings.Contains(log.String(), action.ID) {
		t.Errorf("no warning names the turn: %s", log.String())
	}
	if n, err := f.tracker.Sweep(ctx, t0.Add(2*turns.DefaultTimeout)); err != nil || n != 0 || len(completed(t, f)) != 1 {
		t.Errorf("second Sweep = %d %v, published %d in all", n, err, len(completed(t, f)))
	}
}

// A mechanics that comes after its turn is completed — a repeat of the same
// decision, or another fact of the chain — publishes nothing more and changes
// nothing (case 4 of architect#3, T-478).
func TestAMechanicsAfterItsTurnCompletedPublishesNothing(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	action := f.begun(t, api.ActionFlee)
	f.inTx(t, func(tx *sql.Tx) error {
		return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByLLM, playerA), 1)
	})
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, f.clock.Now()) })
	mech := decided(action, "m")
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, mech, f.clock.Now()) })
	before := rowOf(t, f, action.ID)
	f.clock.Advance(time.Second)
	fact := eventbus.Derive(action, "entity.updated", contracts.SourceState, map[string]any{}, eventbus.WithCauseID("fact"))
	for _, ev := range []eventbus.Event{mech, fact} {
		f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, ev, f.clock.Now()) })
	}
	only(t, f)
	if after := rowOf(t, f, action.ID); after.Status != turns.StatusCompleted || after.MechanicsAt == nil ||
		!after.MechanicsAt.Equal(*before.MechanicsAt) || after.ResultEventID != mech.ID {
		t.Errorf("turn after a late mechanics = %+v, want it as it was completed", after)
	}
}

// The mechanics of a turn with Phase 1 is the event of the type of its action,
// whichever topic the consumer reads first: a fight by its decision, not by
// the facts of State the decision causes, and a move by its fact, not by a
// decision in its chain. Another event does not complete the turn and names
// nothing in it.
func TestTheMechanicsOfATurnIsTheEventOfItsAction(t *testing.T) {
	for _, tc := range []struct {
		action, mechanics, other string
	}{
		{api.ActionAttack, "combat.decided", "entity.updated"},
		{api.ActionEnter, "entity.updated", "combat.decided"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			f := newFixture(t, options{})
			ctx := context.Background()
			action := f.begun(t, tc.action)
			f.inTx(t, func(tx *sql.Tx) error {
				return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByLLM, playerA), 1)
			})
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, f.clock.Now()) })
			f.clock.Advance(100 * time.Millisecond)
			other := eventbus.Derive(action, tc.other, contracts.SourceState, map[string]any{"phase1_mode": "llm"}, eventbus.WithCauseID("other"))
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, other, f.clock.Now()) })
			if n := len(completed(t, f)); n != 0 {
				t.Fatalf("turn.completed of %s after %s = %d", tc.action, tc.other, n)
			}
			if row := rowOf(t, f, action.ID); row.MechanicsAt != nil || row.ResultEventID != "" || row.Phase1Mode != "" {
				t.Fatalf("turn of %s after %s = %+v, want no mechanics", tc.action, tc.other, row)
			}
			f.clock.Advance(100 * time.Millisecond)
			mech := eventbus.Derive(action, tc.mechanics, contracts.SourceSwarm, map[string]any{"phase1_mode": "rules"}, eventbus.WithCauseID("m"))
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, mech, f.clock.Now()) })
			pa := only(t, f).Path()
			result, _ := pa.GetString("delivery.result_event_id")
			mode, _ := pa.GetString("turn.phase1_mode")
			mechanicsMS, _ := pa.GetInt("timings.mechanics_ms")
			if result != mech.ID || mode != "rules" || mechanicsMS != 200 {
				t.Errorf("turn.completed of %s: result %q mode %q mechanics %d ms, want %q, rules, 200", tc.action, result, mode, mechanicsMS, mech.ID)
			}
		})
	}
}

// A completion by the mechanics that the bus does not take fails neither the
// effect of the mechanics nor its record (Mi-1 of review #1 of T-478; decision
// of the orchestrator): the transaction of the consumer commits what it wrote
// and the mechanics of the turn, the turn stays narrated, and the next step —
// the sweeper, or a repeat of the mechanics — publishes turn.completed once,
// with the id of the attempt that failed and the fields of the mechanics.
func TestACompletionTheBusRefusesLeavesTheEffectOfTheMechanics(t *testing.T) {
	for _, next := range []string{"the sweeper", "a repeat of the mechanics"} {
		t.Run(next, func(t *testing.T) {
			var log strings.Builder
			f := newFixture(t, options{log: slog.New(slog.NewJSONHandler(&log, nil))})
			ctx := context.Background()
			if _, err := f.db.ExecContext(ctx, `CREATE TABLE effects (event_id TEXT)`); err != nil {
				t.Fatal(err)
			}
			action := f.begun(t, api.ActionAttack)
			f.inTx(t, func(tx *sql.Tx) error {
				return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByTemplate, playerA), 1)
			})
			f.clock.Advance(time.Second)
			acked := f.clock.Now()
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, acked) })

			var tried []string
			f.bus.set(func(_ context.Context, ev eventbus.Event) error {
				if ev.Type == turns.TypeCompleted {
					tried = append(tried, ev.ID)
					return errors.New("analytics_events refuses")
				}
				return nil
			}, nil)
			f.clock.Advance(300 * time.Millisecond)
			mech := decided(action, "m")
			f.inTx(t, func(tx *sql.Tx) error {
				if _, err := tx.ExecContext(ctx, `INSERT INTO effects (event_id) VALUES (?)`, mech.ID); err != nil {
					return err
				}
				return f.tracker.OnMechanics(ctx, tx, mech, f.clock.Now())
			})
			f.bus.set(nil, nil)

			var effects int
			if err := f.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM effects`).Scan(&effects); err != nil || effects != 1 {
				t.Fatalf("effects of the mechanics after the refusal = %d %v, want 1", effects, err)
			}
			row := rowOf(t, f, action.ID)
			if len(tried) != 1 || row.Status != turns.StatusNarrated || row.MechanicsAt == nil || row.ResultEventID != mech.ID ||
				row.NarrativeAt == nil || !row.NarrativeAt.Equal(acked) {
				t.Fatalf("after the refusal: tried %v, turn %+v; want one attempt and a narrated turn with its mechanics", tried, row)
			}
			if n := len(completed(t, f)); n != 0 {
				t.Fatalf("turn.completed after the refusal = %d", n)
			}
			if s, _, _ := f.sessions.Current(ctx, playerA); s.TurnsDegraded != 0 {
				t.Fatalf("turns_degraded after the refusal = %d, want the completion rolled back", s.TurnsDegraded)
			}
			if !strings.Contains(log.String(), `"level":"WARN"`) || !strings.Contains(log.String(), action.ID) {
				t.Errorf("no warning names the turn: %s", log.String())
			}

			switch next {
			case "the sweeper":
				if n, err := f.tracker.Sweep(ctx, f.clock.Now()); err != nil || n != 1 {
					t.Fatalf("Sweep before the deadline = %d %v, want the turn left open", n, err)
				}
			default:
				f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, mech, f.clock.Now()) })
			}
			ev := only(t, f)
			pa := ev.Path()
			status, _ := pa.GetString("turn.status")
			result, _ := pa.GetString("delivery.result_event_id")
			narrativeAt, _ := pa.GetString("timings.narrative_at")
			if ev.ID != tried[0] || status != turns.OutcomeDegraded || result != mech.ID || narrativeAt != session.Timestamp(acked) {
				t.Errorf("turn.completed %s = %s result %q narrative_at %s; want id %s, degraded, %s, %s",
					ev.ID, status, result, narrativeAt, tried[0], mech.ID, session.Timestamp(acked))
			}
			if n, err := f.tracker.Sweep(ctx, t0.Add(2*turns.DefaultTimeout)); err != nil || n != 0 || len(completed(t, f)) != 1 {
				t.Errorf("Sweep after the completion = %d %v, published %d in all", n, err, len(completed(t, f)))
			}
			if s, _, _ := f.sessions.Current(ctx, playerA); s.TurnsDegraded != 1 || s.TurnsFailed != 0 {
				t.Errorf("session degraded %d failed %d, want 1 and 0", s.TurnsDegraded, s.TurnsFailed)
			}
		})
	}
}

// A mechanics that comes before the acknowledgement leaves the completion to
// the acknowledgement, as before (case 5 of architect#3, T-478).
func TestAnAttackWithItsMechanicsCompletesByTheAcknowledgement(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	action := f.begun(t, api.ActionAttack)
	f.clock.Advance(200 * time.Millisecond)
	mech := decided(action, "m")
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, mech, f.clock.Now()) })
	f.inTx(t, func(tx *sql.Tx) error {
		return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByLLM, playerA), 1)
	})
	if n := len(completed(t, f)); n != 0 {
		t.Fatalf("turn.completed before the acknowledgement = %d", n)
	}
	f.clock.Advance(time.Second)
	acked := f.clock.Now()
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, acked) })
	pa := only(t, f).Path()
	status, _ := pa.GetString("turn.status")
	narrativeAt, _ := pa.GetString("timings.narrative_at")
	mechanicsMS, _ := pa.GetInt("timings.mechanics_ms")
	narrativeMS, _ := pa.GetInt("timings.narrative_ms")
	result, _ := pa.GetString("delivery.result_event_id")
	if status != turns.OutcomeOK || narrativeAt != session.Timestamp(acked) || mechanicsMS != 200 || narrativeMS != 1000 || result != mech.ID {
		t.Errorf("turn.completed = %s narrative_at %s mechanics %d ms narrative %d ms result %q", status, narrativeAt, mechanicsMS, narrativeMS, result)
	}
}

// A turn with Phase 1 that waits for its mechanics counts its deliveries up to
// its recipients: the acknowledgement of a second narrative of its correlation
// — the death after the last blow — changes neither delivered_count nor
// narrative_at, as it would not have after a completion by the acknowledgement.
func TestATurnWaitingForItsMechanicsCountsItsDeliveriesUpToItsRecipients(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	action := f.begun(t, api.ActionAttack)
	told := narrative(t, action, turns.GeneratedByTemplate, playerA)
	death := eventbus.Derive(action, "narrative.output", contracts.SourceSwarm, told.Payload, eventbus.WithCauseID("death"))
	for _, ev := range []eventbus.Event{told, death} {
		f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnNarrative(ctx, tx, ev, 1) })
	}
	acked := f.clock.Now()
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, acked) })
	f.clock.Advance(time.Second)
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, f.clock.Now()) })
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, decided(action, "m"), f.clock.Now()) })

	pa := only(t, f).Path()
	delivered, _ := pa.GetInt("delivery.delivered_count")
	recipients, _ := pa.GetInt("delivery.recipients_count")
	narrativeAt, _ := pa.GetString("timings.narrative_at")
	named, _ := pa.GetString("delivery.narrative_event_id")
	if delivered != 1 || recipients != 1 || narrativeAt != session.Timestamp(acked) || named != told.ID {
		t.Errorf("turn.completed delivered %d of %d, narrative_at %s, narrative %q; want 1 of 1 at %s, %q",
			delivered, recipients, narrativeAt, named, session.Timestamp(acked), told.ID)
	}
}

// A narrative without recipients of a turn with Phase 1 records narrative_at
// when it is applied and waits for the mechanics like an acknowledged one.
func TestANarrativeWithoutRecipientsOfAnEntryWaitsForItsMechanics(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	action := f.begun(t, api.ActionEnter)
	f.clock.Advance(700 * time.Millisecond)
	told := f.clock.Now()
	f.inTx(t, func(tx *sql.Tx) error {
		return f.tracker.OnNarrative(ctx, tx, narrative(t, action, turns.GeneratedByLLM), 0)
	})
	if n := len(completed(t, f)); n != 0 {
		t.Fatalf("turn.completed of an entry without its mechanics = %d", n)
	}
	f.clock.Advance(time.Second)
	fact := eventbus.Derive(action, "entity.updated", contracts.SourceState, map[string]any{}, eventbus.WithCauseID("fact"))
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnMechanics(ctx, tx, fact, f.clock.Now()) })
	pa := only(t, f).Path()
	narrativeAt, _ := pa.GetString("timings.narrative_at")
	result, _ := pa.GetString("delivery.result_event_id")
	if narrativeAt != session.Timestamp(told) || result != fact.ID {
		t.Errorf("turn.completed narrative_at %s result %q, want %s and %s", narrativeAt, result, session.Timestamp(told), fact.ID)
	}
}

// delivery.narrative_event_id of a turn is the id of its narrative, or the
// narrative_event_id the narrative carries when it names one (proposal 2 of
// review #2 of T-313).
func TestATurnNamesItsNarrative(t *testing.T) {
	for _, tc := range []struct {
		name, carried string
	}{
		{"the id of the narrative", ""},
		{"the narrative_event_id it carries", "narrative-of-the-swarm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, options{})
			ctx := context.Background()
			action := f.begun(t, api.ActionLook)
			told := narrative(t, action, turns.GeneratedByLLM, playerA)
			want := told.ID
			if tc.carried != "" {
				told.Payload["narrative_event_id"] = tc.carried
				want = tc.carried
			}
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnNarrative(ctx, tx, told, 1) })
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnDelivered(ctx, tx, action.ID, f.clock.Now()) })
			if named, _ := only(t, f).Path().GetString("delivery.narrative_event_id"); named != want || named == action.ID {
				t.Errorf("delivery.narrative_event_id = %q, want %q", named, want)
			}
		})
	}
}
