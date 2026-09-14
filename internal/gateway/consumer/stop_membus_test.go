package consumer_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"multiverse-core.io/internal/gateway/consumer"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
)

// On membus a cancelled subscription returns at once, so Stop returns as soon
// as the subscriptions did: StopTransportGrace is for a transport that does
// not return — a reader of kafka-go joining its group — and is never waited
// for here (T-480). On the manual clock the grace never passes, so a Stop that
// waited for it would not return at all.
func TestStopOverMembusDoesNotWaitForTheGrace(t *testing.T) {
	r := newResubscribing(t)
	d := r.dispatcher(t, r.bus)
	if err := d.Start(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	r.publish(t, named("live", created("player-A", 10)))
	eventually(t, "the subscription to take an event", func() bool {
		_, ok := r.cursor(t, eventbus.TopicSystemEvents)
		return ok
	})

	stopped := make(chan error, 1)
	go func() { stopped <- d.Stop(context.Background()) }()
	if err := receive(t, stopped, "Stop over membus to return while the clock stands"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// The same on the wall clock, as in a process on --bus=memory or a replay: a
// Stop over membus takes a small part of the grace, not the grace.
func TestStopOverMembusOnTheWallClockTakesLessThanTheGrace(t *testing.T) {
	f := newFixture(t, membus.Chaos{})
	d, err := consumer.New(consumer.Config{
		Bus: f.bus, Journal: f.bus, DB: f.db, Model: f.model, Clock: f.clock, Timers: clock.RealTimers{},
		Log:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Effects: map[string][]consumer.Effect{readmodel.TypeEntityCreated: {}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Start(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	wall := testkit.Wall()
	began := wall.Now()
	if err := d.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if took := wall.Now().Sub(began); took >= consumer.StopTransportGrace/2 {
		t.Errorf("Stop over membus took %s, want well below the grace %s", took, consumer.StopTransportGrace)
	}
}
