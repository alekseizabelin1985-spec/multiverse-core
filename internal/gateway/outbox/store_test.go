package outbox_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/outbox"
)

// Lease gives the earliest pending delivery of every player, in the order of
// seq, and nothing more of a player while one of its deliveries is in a lease
// that runs; the next one comes after the ack (component §8.2, C-08).
func TestLeaseGivesTheHeadOfEachPlayerInTheOrderOfSeq(t *testing.T) {
	f := newFixture(t)
	f.enqueue(t, delivery("e1", playerA), delivery("e2", playerB), delivery("e3", playerA), delivery("e4", playerA))

	if got := f.lease(t, bot, 10); !slices.Equal(got, []string{"e1", "e2"}) {
		t.Fatalf("first lease = %v, want the head of each player [e1 e2]", got)
	}
	if got := f.lease(t, bot, 10); len(got) != 0 {
		t.Fatalf("lease while both heads are leased = %v, want nothing", got)
	}
	if _, _, err := f.store.Ack(context.Background(), bot, []string{id("e1", playerA)}, f.clock.Now(), nil); err != nil {
		t.Fatal(err)
	}
	if got := f.lease(t, bot, 10); !slices.Equal(got, []string{"e3"}) {
		t.Fatalf("lease after the ack of e1 = %v, want [e3], the next of player-A in order", got)
	}
}

func TestLeaseRespectsItsLimit(t *testing.T) {
	f := newFixture(t)
	f.enqueue(t, delivery("e1", playerA), delivery("e2", playerB))
	if got := f.lease(t, bot, 1); !slices.Equal(got, []string{"e1"}) {
		t.Fatalf("lease of 1 = %v, want [e1]", got)
	}
	if got := f.lease(t, bot, 0); len(got) != 0 {
		t.Fatalf("lease of 0 = %v", got)
	}
}

// A delivery whose lease ran out is given again, with the attempt counted, and
// not before (component §8.4: 30 s).
func TestAnExpiredLeaseIsGivenAgain(t *testing.T) {
	f := newFixture(t)
	f.enqueue(t, delivery("e1", playerA), delivery("e2", playerA))
	f.lease(t, bot, 10)

	f.clock.Advance(outbox.DefaultLease)
	if got := f.lease(t, bot, 10); len(got) != 0 {
		t.Fatalf("lease at the end of the lease = %v, want nothing yet", got)
	}
	f.clock.Advance(time.Millisecond)
	ds, err := f.store.Lease(context.Background(), bot, telegram, 10, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := eventsOf(ds); !slices.Equal(got, []string{"e1"}) || ds[0].Attempts != 2 {
		t.Fatalf("lease after the lease = %v (attempts %v), want e1 again, attempt 2", got, ds)
	}
}

// ReleaseExpiredLeases ends only the leases that ran out and rings the bell of
// the waiting long-polls.
func TestReleaseExpiredLeasesRingsTheBell(t *testing.T) {
	f := newFixture(t)
	f.enqueue(t, delivery("e1", playerA))
	f.lease(t, bot, 10)
	bell := f.store.Notifier().Bell(telegram)

	if n, err := f.store.ReleaseExpiredLeases(context.Background(), f.clock.Now()); err != nil || n != 0 {
		t.Fatalf("release of a running lease = %d, %v", n, err)
	}
	f.clock.Advance(outbox.DefaultLease + time.Second)
	if n, err := f.store.ReleaseExpiredLeases(context.Background(), f.clock.Now()); err != nil || n != 1 {
		t.Fatalf("release of an expired lease = %d, %v", n, err)
	}
	select {
	case <-bell:
	default:
		t.Error("the bell did not ring after a release")
	}
}

// Enqueue is idempotent by (event_id, player_id): the same event delivered
// again by the bus writes no second row (NFR-013).
func TestEnqueueTheSameEventTwiceWritesOneRow(t *testing.T) {
	f := newFixture(t)
	if n := f.enqueue(t, delivery("e1", playerA), delivery("e1", playerB)); n != 2 {
		t.Fatalf("first enqueue = %d, want 2", n)
	}
	if n := f.enqueue(t, delivery("e1", playerA)); n != 0 {
		t.Fatalf("repeat = %d, want 0", n)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM deliveries WHERE event_id = 'e1'`); n != 2 {
		t.Errorf("rows of e1 = %d, want 2", n)
	}
}

// A delivery to a player without a link is written dropped and never leased.
func TestADeliveryWithoutAPlatformIsWrittenDropped(t *testing.T) {
	f := newFixture(t)
	d := delivery("e1", playerA)
	d.Platform = ""
	if n := f.enqueue(t, d); n != 0 {
		t.Fatalf("enqueue = %d, want 0 pending", n)
	}
	if got := f.state(t, "e1", playerA); got != outbox.StateDropped {
		t.Errorf("state = %s, want dropped", got)
	}
}

func TestEnqueueRefusesAnIncompleteDelivery(t *testing.T) {
	f := newFixture(t)
	for name, mutate := range map[string]func(*outbox.Delivery){
		"no player": func(d *outbox.Delivery) { d.PlayerID = "" },
		"no event":  func(d *outbox.Delivery) { d.EventID = "" },
		"no text":   func(d *outbox.Delivery) { d.Text = "" },
		"no kind":   func(d *outbox.Delivery) { d.Kind = "" },
	} {
		d := delivery("e1", playerA)
		mutate(&d)
		tx, err := f.db.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.Enqueue(context.Background(), tx, f.clock.Now(), d); err == nil {
			t.Errorf("%s: enqueue accepted", name)
		}
		_ = tx.Rollback()
	}
}

// Ack takes the deliveries of the client that leased them; an unknown id, a
// delivery of another client and a repeat come back unknown without an error,
// and onDelivered runs once per delivery (component §8.4).
func TestAckTakesOnlyWhatTheClientLeased(t *testing.T) {
	f := newFixture(t)
	f.enqueue(t, delivery("e1", playerA), delivery("e2", playerB))
	if _, err := f.store.Lease(context.Background(), bot, telegram, 1, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Lease(context.Background(), "other-bot", telegram, 1, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	var calls []string
	onDelivered := func(_ context.Context, _ outbox.DB, d outbox.Delivery, _ time.Time) error {
		calls = append(calls, d.EventID)
		return nil
	}
	ids := []string{id("e1", playerA), id("e2", playerB), "d-unknown", id("e1", playerA)}
	acked, unknown, err := f.store.Ack(context.Background(), bot, ids, f.clock.Now(), onDelivered)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(acked, []string{id("e1", playerA)}) || !slices.Equal(unknown, []string{id("e2", playerB), "d-unknown"}) {
		t.Fatalf("acked %v unknown %v", acked, unknown)
	}
	if f.state(t, "e2", playerB) != outbox.StatePending {
		t.Error("the delivery of another client was acknowledged")
	}
	acked, unknown, err = f.store.Ack(context.Background(), bot, []string{id("e1", playerA)}, f.clock.Now(), onDelivered)
	if err != nil || len(acked) != 0 || !slices.Equal(unknown, []string{id("e1", playerA)}) {
		t.Fatalf("repeat = %v %v %v, want unknown", acked, unknown, err)
	}
	if !slices.Equal(calls, []string{"e1"}) {
		t.Errorf("onDelivered calls = %v, want one for e1", calls)
	}
}

// A late ack — after the lease ran out and the sweeper released it — is taken
// while nobody leased the delivery again, so that a batch that took longer
// than the lease does not come back as duplicates; once another client leased
// it, the late ack is unknown (addition of the orchestrator from review #1 of
// T-312).
func TestALateAckIsTakenUntilTheDeliveryIsLeasedAgain(t *testing.T) {
	ctx := context.Background()
	t.Run("nobody leased it again", func(t *testing.T) {
		f := newFixture(t)
		f.enqueue(t, delivery("e1", playerA))
		f.lease(t, bot, 10)
		f.clock.Advance(outbox.DefaultLease + time.Minute)
		if err := f.store.Sweep(ctx, f.clock.Now()); err != nil {
			t.Fatal(err)
		}
		acked, _, err := f.store.Ack(ctx, bot, []string{id("e1", playerA)}, f.clock.Now(), nil)
		if err != nil || len(acked) != 1 {
			t.Fatalf("late ack = %v, %v; want it taken", acked, err)
		}
		if got := f.lease(t, bot, 10); len(got) != 0 {
			t.Errorf("the acknowledged delivery is given again: %v", got)
		}
	})
	t.Run("another client leased it", func(t *testing.T) {
		f := newFixture(t)
		f.enqueue(t, delivery("e1", playerA))
		f.lease(t, bot, 10)
		f.clock.Advance(outbox.DefaultLease + time.Second)
		if err := f.store.Sweep(ctx, f.clock.Now()); err != nil {
			t.Fatal(err)
		}
		f.lease(t, "other-bot", 10)
		acked, unknown, err := f.store.Ack(ctx, bot, []string{id("e1", playerA)}, f.clock.Now(), nil)
		if err != nil || len(acked) != 0 || len(unknown) != 1 {
			t.Fatalf("late ack after a new lease = %v %v %v, want unknown", acked, unknown, err)
		}
		if acked, _, err := f.store.Ack(ctx, "other-bot", []string{id("e1", playerA)}, f.clock.Now(), nil); err != nil || len(acked) != 1 {
			t.Fatalf("ack of the new lease = %v %v", acked, err)
		}
	})
}

// An error of onDelivered rolls the whole acknowledgement back: the client
// repeats it.
func TestAFailedStepOfTheTurnRollsTheAckBack(t *testing.T) {
	f := newFixture(t)
	f.enqueue(t, delivery("e1", playerA), delivery("e2", playerB))
	f.lease(t, bot, 10)
	boom := func(_ context.Context, _ outbox.DB, d outbox.Delivery, _ time.Time) error {
		if d.EventID == "e2" {
			return context.DeadlineExceeded
		}
		return nil
	}
	if _, _, err := f.store.Ack(context.Background(), bot, []string{id("e1", playerA), id("e2", playerB)}, f.clock.Now(), boom); err == nil {
		t.Fatal("ack succeeded")
	}
	if f.state(t, "e1", playerA) != outbox.StatePending || f.state(t, "e2", playerB) != outbox.StatePending {
		t.Error("an acknowledgement survived the failure")
	}
}

// The sweeper drops what is pending past its TTL and deletes finished rows
// older than seven days; DropForPlayer drops the queue of a player (/forget).
func TestExpireDropForPlayerAndPurge(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.enqueue(t, delivery("old", playerA))
	f.clock.Advance(outbox.DefaultTTL)
	f.enqueue(t, delivery("new", playerA), delivery("b", playerB))
	// The new rows are created before the moment of Expire and are far from
	// their expires_at: a drop by created_at would take them too (review #1 of
	// T-307, N-2).
	f.clock.Advance(2 * time.Second)

	if n, err := f.store.Expire(ctx, f.clock.Now()); err != nil || n != 1 {
		t.Fatalf("expire = %d, %v", n, err)
	}
	if f.state(t, "old", playerA) != outbox.StateDropped || f.state(t, "new", playerA) != outbox.StatePending {
		t.Error("expire dropped the wrong row")
	}
	if n, err := f.store.DropForPlayer(ctx, playerA); err != nil || n != 1 {
		t.Fatalf("drop for player = %d, %v", n, err)
	}
	if n, err := f.store.DropForPlayer(ctx, playerA); err != nil || n != 0 {
		t.Fatalf("repeat of drop for player = %d, %v", n, err)
	}
	if f.state(t, "b", playerB) != outbox.StatePending {
		t.Error("the queue of another player was dropped")
	}
	f.clock.Advance(7*24*time.Hour - outbox.DefaultTTL)
	if n, err := f.store.Purge(ctx, f.clock.Now()); err != nil || n != 1 {
		t.Fatalf("purge = %d, %v; want the old dropped row only", n, err)
	}
	if n := f.count(t, `SELECT COUNT(*) FROM deliveries`); n != 2 {
		t.Errorf("rows after purge = %d, want 2", n)
	}
}

func TestDeliveryIDsNameWhatTheyDeliver(t *testing.T) {
	first, again := outbox.EventDeliveryID("e1", "p", "k"), outbox.EventDeliveryID("e"+"1", "p", "k")
	if first != again || !strings.HasPrefix(first, "d-") {
		t.Errorf("the ids of the same delivery are %s and %s", first, again)
	}
	if outbox.EventDeliveryID("e1:p", "", "k") == outbox.EventDeliveryID("e1", ":p", "k") {
		t.Error("two different deliveries share an id")
	}
	if outbox.TransitionDeliveryID("enc", "opened", "p") == outbox.EventDeliveryID("enc", "opened", "p") {
		t.Error("a transition and an event share an id")
	}
}
