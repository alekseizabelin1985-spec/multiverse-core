package turns_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/session"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
)

// A narrative without recipients has nobody to acknowledge it: its turn is
// completed when the narrative is applied — ok, or degraded for a template —
// with narrative_at at that moment, and it does not time out afterwards
// (acceptance of T-306).
func TestANarrativeWithoutRecipientsCompletesItsTurn(t *testing.T) {
	for _, tc := range []struct {
		generatedBy, outcome string
		degraded             int
	}{
		{turns.GeneratedByLLM, turns.OutcomeOK, 0},
		{turns.GeneratedByTemplate, turns.OutcomeDegraded, 1},
	} {
		t.Run(tc.generatedBy, func(t *testing.T) {
			f := newFixture(t, options{})
			a := accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
			f.clock.Advance(700 * time.Millisecond)
			nobody := narrative(t, f.actionEvent(t, a.CorrelationID), tc.generatedBy)
			f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnNarrative(context.Background(), tx, nobody, 0) })

			got := completed(t, f)
			if len(got) != 1 {
				t.Fatalf("turn.completed = %d, want 1", len(got))
			}
			pa := got[0].Path()
			status, _ := pa.GetString("turn.status")
			narrated, _ := pa.GetString("timings.narrative_at")
			recipients, _ := pa.GetInt("delivery.recipients_count")
			if status != tc.outcome || narrated != session.Timestamp(t0.Add(700*time.Millisecond)) || recipients != 0 {
				t.Errorf("turn = %s narrative_at %s recipients %d", status, narrated, recipients)
			}
			f.clock.Advance(turns.DefaultTimeout)
			if _, err := f.tracker.Sweep(context.Background(), f.clock.Now()); err != nil {
				t.Fatal(err)
			}
			if n := len(completed(t, f)); n != 1 {
				t.Errorf("turn.completed after the deadline = %d, want still 1", n)
			}
			s, _, _ := f.sessions.Current(context.Background(), playerA)
			if s.TurnsFailed != 0 || s.TurnsDegraded != tc.degraded {
				t.Errorf("session failed %d degraded %d", s.TurnsFailed, s.TurnsDegraded)
			}
		})
	}
}

// A narrative with recipients leaves its turn narrated until the last of them
// acknowledges it.
func TestANarrativeWithRecipientsWaitsForItsAcks(t *testing.T) {
	f := newFixture(t, options{})
	a := accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
	told := narrative(t, f.actionEvent(t, a.CorrelationID), turns.GeneratedByLLM, playerA)
	f.inTx(t, func(tx *sql.Tx) error { return f.tracker.OnNarrative(context.Background(), tx, told, 1) })
	if n := len(completed(t, f)); n != 0 {
		t.Fatalf("turn.completed before the ack = %d", n)
	}
	if row := rowOf(t, f, a.CorrelationID); row.Status != turns.StatusNarrated || row.RecipientsCount != 1 {
		t.Errorf("row = %+v", row)
	}
}

// blocking is a broker that does not answer: a publication waits for the end
// of its context.
type blocking struct {
	mu    sync.Mutex
	on    bool
	tried []string
	calls int
}

func (b *blocking) hook(ctx context.Context, ev eventbus.Event) error {
	b.mu.Lock()
	on := b.on
	if on && ev.Type == turns.TypeCompleted {
		b.tried = append(b.tried, ev.ID)
		b.calls++
	}
	b.mu.Unlock()
	if !on || ev.Type != turns.TypeCompleted {
		return nil
	}
	<-ctx.Done()
	return ctx.Err()
}

// The publication of analytics.turn.completed has a deadline: a broker that
// does not answer releases the connection of gateway.db within it, the step is
// rolled back, and the next tick publishes the same turn.completed with the
// same id (acceptance of T-306, Mi-2; the variant "with a deadline").
func TestAPublicationThatHangsReleasesTheConnectionAndIsRepeated(t *testing.T) {
	const deadline = 100 * time.Millisecond
	f := newFixture(t, options{publishTimeout: deadline})
	a := accepted(t, f.submit(t, "k-1", api.ActionLook, ""))
	broker := &blocking{on: true}
	f.bus.set(broker.hook, nil)
	f.clock.Advance(turns.DefaultTimeout)

	began := clock.Real{}.Now()
	_, err := f.tracker.Sweep(context.Background(), f.clock.Now())
	if !errors.Is(err, turns.ErrPublish) {
		t.Fatalf("Sweep = %v, want ErrPublish", err)
	}
	if took := (clock.Real{}).Now().Sub(began); took > deadline+time.Second {
		t.Errorf("Sweep held on for %s, the deadline is %s", took, deadline)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := f.db.QueryRowContext(ctx, `SELECT 1`).Scan(new(int)); err != nil {
		t.Fatalf("gateway.db is still held: %v", err)
	}
	if row := rowOf(t, f, a.CorrelationID); row.Status != turns.StatusAccepted {
		t.Errorf("status after the failed publication = %s, want the step rolled back", row.Status)
	}

	broker.mu.Lock()
	broker.on = false
	broker.mu.Unlock()
	if n, err := f.tracker.Sweep(context.Background(), f.clock.Now()); err != nil || n != 1 {
		t.Fatalf("next tick = %d, %v", n, err)
	}
	got := completed(t, f)
	if len(got) != 1 || broker.calls != 1 || got[0].ID != broker.tried[0] {
		t.Fatalf("published %d (tried %v), want the same id published once", len(got), broker.tried)
	}
	if status, _ := got[0].Path().GetString("turn.status"); status != turns.OutcomeTimeout {
		t.Errorf("status = %s", status)
	}
}
