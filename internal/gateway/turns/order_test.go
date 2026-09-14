package turns_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

// The mechanics and the narrative of an action can reach the consumer while
// the service is still publishing its batch: the bus delivers asynchronously.
// Here both are handled the moment player.entered_region is out, before the
// proposal of the move is published, and the turn still completes with the
// acknowledgement of its narrative (component §5.5; review #1 of T-308, Mi-1).
func TestTheMechanicsAndTheNarrativeRightAfterThePublicationCompleteTheTurn(t *testing.T) {
	f := newFixture(t, options{})
	f.bus.set(nil, func(_ context.Context, ev eventbus.Event) {
		if ev.Type != actions.TypeEnteredRegion {
			return
		}
		fact := eventbus.Derive(ev, "entity.updated", contracts.SourceState, map[string]any{"cause": "move"})
		told := narrative(t, ev, turns.GeneratedByLLM, playerA)
		f.inTx(t, func(tx *sql.Tx) error {
			if err := f.tracker.OnMechanics(context.Background(), tx, fact, f.clock.Now()); err != nil {
				return err
			}
			return f.tracker.OnNarrative(context.Background(), tx, told, 1)
		})
	})
	a := accepted(t, f.submit(t, "k-enter", api.ActionEnter, forest))
	f.bus.set(nil, nil)

	if row := rowOf(t, f, a.CorrelationID); row.Status != turns.StatusNarrated || row.MechanicsAt == nil || row.RecipientsCount != 1 {
		t.Fatalf("turn after the publication = %+v, want narrated with its mechanics", row)
	}
	f.inTx(t, func(tx *sql.Tx) error {
		return f.tracker.OnDelivered(context.Background(), tx, a.CorrelationID, f.clock.Now())
	})
	got := completed(t, f)
	if len(got) != 1 || got[0].CorrelationID() != a.CorrelationID {
		t.Fatalf("turn.completed = %d, want 1 of %s", len(got), a.CorrelationID)
	}
	if status, _ := got[0].Path().GetString("turn.status"); status != turns.OutcomeOK {
		t.Errorf("turn.status = %s", status)
	}
}

// An action event the bus did not acknowledge takes its turn back: the client
// is answered 503 bus_unavailable as before, no turn waits for a deadline, and
// the repeat of the key records the turn under the same number.
func TestAnActionTheBusDidNotAcknowledgeTakesItsTurnBack(t *testing.T) {
	f := newFixture(t, options{})
	f.bus.set(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == actions.TypeLooked {
			return errors.New("broker down")
		}
		return nil
	}, nil)
	a := f.submit(t, "k-look", api.ActionLook, "")
	if a.Err == nil || a.Err.Code != api.CodeBusUnavailable {
		t.Fatalf("answer = %+v, want 503 bus_unavailable", a)
	}
	var n int
	if err := f.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM turns`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("turns after the failure = %d %v, want none", n, err)
	}
	if s, _, _ := f.sessions.Current(context.Background(), playerA); s.TurnsCount != 0 {
		t.Errorf("turns_count after the failure = %d", s.TurnsCount)
	}
	f.clock.Advance(turns.DefaultTimeout)
	if closed, err := f.tracker.Sweep(context.Background(), f.clock.Now()); err != nil || closed != 0 || len(completed(t, f)) != 0 {
		t.Errorf("sweep after the deadline closed %d %v, published %d", closed, err, len(completed(t, f)))
	}

	f.bus.set(nil, nil)
	again := accepted(t, f.submit(t, "k-look", api.ActionLook, ""))
	if row := rowOf(t, f, again.CorrelationID); row.Status != turns.StatusAccepted || row.Seq != 1 {
		t.Errorf("turn of the repeat = %+v", row)
	}
	if s, _, _ := f.sessions.Current(context.Background(), playerA); s.TurnsCount != 1 {
		t.Errorf("turns_count after the repeat = %d", s.TurnsCount)
	}
}

// The budget of the decision runs out while player.* is being published: 503
// bus_unavailable, and the turn is taken back all the same — on the context of
// the decision the delete would fail in gateway.db and the turn would time out
// for an action nobody accepted (review #2 of T-308, Mi-1).
func TestABudgetThatRunsOutOnTheActionTakesItsTurnBack(t *testing.T) {
	f := newFixture(t, options{})
	hanging := make(chan struct{})
	f.bus.set(func(ctx context.Context, ev eventbus.Event) error {
		if ev.Type != actions.TypeLooked {
			return nil
		}
		close(hanging)
		<-ctx.Done()
		return context.Cause(ctx)
	}, nil)
	answer := make(chan actions.Answer, 1)
	go func() { answer <- f.submit(t, "k-look", api.ActionLook, "") }()
	<-hanging
	f.clock.Advance(api.RequestTimeout)
	if a := <-answer; a.Err == nil || a.Err.Code != api.CodeBusUnavailable {
		t.Fatalf("answer = %+v, want 503 bus_unavailable", a)
	}
	var n int
	if err := f.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM turns`).Scan(&n); err != nil || n != 0 {
		t.Errorf("turns after the budget ran out = %d %v, want none", n, err)
	}
	if s, _, _ := f.sessions.Current(context.Background(), playerA); s.TurnsCount != 0 {
		t.Errorf("turns_count = %d, want 0", s.TurnsCount)
	}
}

// A turn that already heard from its action — the event reached the broker
// without its acknowledgement — is not taken back; nor is one with its
// mechanics; a turn that is not there is no error.
func TestATurnThatHeardFromItsActionIsNotTakenBack(t *testing.T) {
	f := newFixture(t, options{})
	a := accepted(t, f.submit(t, "k-look", api.ActionLook, ""))
	ctx := context.Background()
	f.inTx(t, func(tx *sql.Tx) error {
		return f.tracker.OnNarrative(ctx, tx, narrative(t, f.actionEvent(t, a.CorrelationID), turns.GeneratedByLLM, playerA), 1)
	})
	if err := f.tracker.Withdrawn(ctx, a.Turn, a.CorrelationID); err != nil {
		t.Fatal(err)
	}
	if row := rowOf(t, f, a.CorrelationID); row.Status != turns.StatusNarrated {
		t.Errorf("narrated turn after Withdrawn = %+v", row)
	}

	b := accepted(t, f.submit(t, "k-enter", api.ActionEnter, forest))
	f.inTx(t, func(tx *sql.Tx) error {
		fact := eventbus.Derive(f.actionEvent(t, b.CorrelationID), "entity.updated", contracts.SourceState, map[string]any{})
		return f.tracker.OnMechanics(ctx, tx, fact, f.clock.Now())
	})
	if err := f.tracker.Withdrawn(ctx, b.Turn, b.CorrelationID); err != nil {
		t.Fatal(err)
	}
	if row := rowOf(t, f, b.CorrelationID); row.Status != turns.StatusMechanicsApplied {
		t.Errorf("turn with its mechanics after Withdrawn = %+v", row)
	}
	if err := f.tracker.Withdrawn(ctx, b.Turn, "no-such-turn"); err != nil {
		t.Errorf("Withdrawn of a turn that is not there = %v", err)
	}
	if s, _, _ := f.sessions.Current(ctx, playerA); s.TurnsCount != 2 {
		t.Errorf("turns_count = %d, want both turns counted", s.TurnsCount)
	}
}

// acked_at is the moment the batch was acknowledged and 202 answered, not the
// moment the turn was written ahead of the publication: the latency of the
// broker stays in the timings of the turn (NFR-003).
func TestTheAcknowledgementOfATurnIsTheAnswerOfItsAction(t *testing.T) {
	f := newFixture(t, options{})
	f.bus.set(nil, func(_ context.Context, ev eventbus.Event) {
		if ev.Type == actions.TypeUpdateProposed {
			f.clock.Advance(15 * time.Millisecond)
		}
	})
	a := accepted(t, f.submit(t, "k-enter", api.ActionEnter, forest))
	row := rowOf(t, f, a.CorrelationID)
	if !a.AckedAt.Equal(t0.Add(15*time.Millisecond)) || !row.AckedAt.Equal(a.AckedAt) || !row.ReceivedAt.Equal(t0) {
		t.Errorf("answer acked_at %s, turn received_at %s acked_at %s", a.AckedAt, row.ReceivedAt, row.AckedAt)
	}
}
