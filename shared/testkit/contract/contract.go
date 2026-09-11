// Package contract holds the behavioural contract of the event bus: one set of
// tests, run against every implementation of C-01.
//
// The point of writing it once is that membus and the kafka adapter cannot
// drift apart unnoticed. Whatever an epic learns about the bus from membus in
// a unit test must hold on the broker, or membus is lying — and the whole
// parallel plan of the waves rests on membus telling the truth (foundation.md
// §9, ADR-010, tasks.md T-014).
//
// The suite is offset relative and never assumes an empty journal: every case
// takes End as its baseline, publishes into a world of its own and ignores
// everything else, so the same file runs against a fresh membus and against a
// broker that already holds the events of the previous cases.
package contract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit"
)

// Retries is the number of retries C-01 gives a failing handler, and therefore
// four calls of the handler in all before the event is parked (ADR-007 p. 6,
// journal decision of 2026-09-09). Every target must be built with a backoff
// of this length.
const Retries = 3

// PublishBudget is the ceiling a single publish must stay under on a live
// broker. NFR-001 gives the whole acknowledgement 300 ms, and publishing is
// only the first hop of it; the number exists because the kafka-go defaults
// (a batch of 100 messages or one second) made every publish of MVP-1 wait for
// the batch timer — review finding M-1 of T-005.
const PublishBudget = 300 * time.Millisecond

// Target is one implementation under test.
type Target struct {
	// Name appears in the test output: membus or redpanda.
	Name string
	Bus  eventbus.Bus
	// Journal is the same object as Bus in both implementations, taken
	// through the other interface.
	Journal eventbus.Journal
	// Append writes a body straight into a topic, bypassing routing and
	// validation. It is how the suite produces the message a publisher never
	// could — an envelope that fails validation, or bytes that are not an
	// event at all.
	Append func(topic string, body []byte) error
	// DeadLetters returns everything parked so far. It cannot go through the
	// journal: a dead letter is not an event, and reading it as one would
	// park it again.
	DeadLetters func(ctx context.Context) ([]eventbus.DeadLetter, error)
	// Duplicate publishes an event that the bus then delivers twice: the
	// --chaos=duplicate mode C-01 gives the stub (ADR-010 p. 4). A broker has
	// no such switch, so the live target repeats the publish, which is what a
	// producer retry after a lost acknowledgement puts on the wire. Either way
	// the consumer sees the one thing that matters — the same event id
	// arriving twice.
	Duplicate func(ctx context.Context, ev eventbus.Event) error
	// Lenient is the same transport read with validation on read turned off,
	// together with the function that releases it. It is the other position of
	// MV_BUS_VALIDATE_ON_READ: without it the suite only ever sees the flag
	// switched on, and a context passing it inverted would look correct here.
	Lenient func() (bus eventbus.Bus, release func(), err error)
	// Close closes the bus under test. The case that uses it runs last and
	// nothing runs after it, so a target that must survive the suite may leave
	// it nil and skip that case.
	Close func() error
	// Live marks a target that talks to a real broker over a socket. The
	// publish latency is only meaningful there — in process it measures a
	// slice append.
	Live bool
}

// Timeout is how long a case waits for an event to come back. Nothing waits
// for it on a happy path — the slowest case of the suite takes under a second
// against a live broker — so its only job is to turn a hang into a failure
// with a message. It is the whole cost of a red run: the budget of the CI is
// ten minutes for every job (ADR-010 p. 5), and a suite that stops on the
// first failure of every case must fit in it several times over. Fifteen
// seconds leaves room for joining a consumer group on a cold broker and for a
// runner several times slower than a workstation.
const Timeout = 15 * time.Second

// runSeq numbers the cases so that each gets a world and a consumer group of
// its own: a broker keeps what the previous case published, and a group that
// shared a name would inherit its cursor.
var runSeq atomic.Int64

// Run executes the whole contract against one target.
func Run(t *testing.T, target Target) {
	t.Helper()
	if target.Bus == nil || target.Journal == nil {
		t.Fatal("contract: the target must provide both a Bus and a Journal")
	}
	if target.Append == nil || target.DeadLetters == nil {
		t.Fatal("contract: the target must provide Append and DeadLetters")
	}
	if target.Duplicate == nil || target.Lenient == nil {
		t.Fatal("contract: the target must provide Duplicate and Lenient")
	}

	cases := []struct {
		name string
		fn   func(*testing.T, Target)
	}{
		{"PublishIsReceivedInOrderWithItsPosition", publishIsReceivedInOrder},
		{"UnknownAndInvalidEventsAreNotPublished", unknownAndInvalidAreNotPublished},
		{"TopicPolicyOfPlayerEvents", topicPolicyOfPlayerEvents},
		{"TopicPolicyOfTheSwarmTopics", topicPolicyOfTheSwarmTopics},
		{"JournalReadsByIncreasingOffset", journalReadsByIncreasingOffset},
		{"JournalEndIsMonotonic", journalEndIsMonotonic},
		{"JournalStopsAtTheEndOfTheJournal", journalStopsAtTheEnd},
		{"JournalTailFollowsUntilTheContextIsDone", journalTailFollows},
		{"AGroupResumesFromItsCursor", groupResumesFromItsCursor},
		{"DedupDropsTheRepeatedDelivery", dedupDropsTheRepeat},
		{"TwoStepDedupRemembersOnlyAfterTheSideEffect", twoStepDedupAfterTheSideEffect},
		{"AnUncommittedEventIsDeliveredAgain", uncommittedIsDeliveredAgain},
		{"CancellingASubscriptionStopsItOnTheBacklog", cancellingStopsOnTheBacklog},
		{"InvalidOnReadGoesToDeadLettersWithoutTheHandler", invalidOnReadGoesToDeadLetters},
		{"WithoutValidationOnReadTheEventReachesTheHandler", withoutValidationOnReadItReachesTheHandler},
		{"UndecodableMessageGoesToDeadLetters", undecodableGoesToDeadLetters},
		{"RetriesThenDeadLetterAndTheStreamMovesOn", retriesThenDeadLetter},
		{"SubscribeReturnsNilOnAnOrderlyStop", subscribeReturnsNilOnStop},
		{"PublishOfOneEventIsNotBatched", publishOfOneEventIsNotBatched},
		// The big body is left until after the other dead-letter cases: every
		// later case that reads dead_letters would carry it over the wire on
		// every poll.
		{"ABigUndecodableBodyIsTruncatedInItsDeadLetter", bigUndecodableBodyIsTruncated},
		// Last: it closes the bus, and nothing works afterwards.
		{"CloseStopsTheSubscriptionsAndRefusesToPublish", closeStopsEverything},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { c.fn(t, target) })
	}
}

// --- the cases -------------------------------------------------------------

// publishIsReceivedInOrder is the base guarantee of C-01: what is published
// comes back, in the order it was published, and the handler is told where it
// came from.
func publishIsReceivedInOrder(t *testing.T, target Target) {
	r := newRun(t, target)
	base := r.end(eventbus.TopicPlayerEvents)

	sub := r.subscribe(eventbus.TopicPlayerEvents)
	defer sub.stop(t)

	const n = 5
	for i := range n {
		r.publish(r.looked(strconv.Itoa(i)))
	}
	got := sub.wait(t, n)

	for i, ev := range got {
		if want := strconv.Itoa(i); ev.payloadName() != want {
			t.Fatalf("event %d is %q, want %q: a topic keeps the order it was written in", i, ev.payloadName(), want)
		}
		if ev.pos.Topic != eventbus.TopicPlayerEvents {
			t.Fatalf("position of event %d is %+v, want topic %s", i, ev.pos, eventbus.TopicPlayerEvents)
		}
		if i > 0 && ev.pos.Offset <= got[i-1].pos.Offset {
			t.Fatalf("offsets %d and %d are not increasing: %d then %d",
				i-1, i, got[i-1].pos.Offset, ev.pos.Offset)
		}
	}
	if got[0].pos.Offset < base {
		t.Fatalf("first offset %d is below the end of the journal before the run (%d)", got[0].pos.Offset, base)
	}
}

// unknownAndInvalidAreNotPublished: an unknown type or a broken envelope is a
// defect of the publisher, reported to it rather than written to a topic.
func unknownAndInvalidAreNotPublished(t *testing.T, target Target) {
	r := newRun(t, target)
	ctx := t.Context()

	unknown := r.looked("x")
	unknown.Type = "player.teleported.through.walls"
	if err := target.Bus.Publish(ctx, unknown); !errors.Is(err, eventbus.ErrUnknownType) {
		t.Errorf("publish of an unregistered type = %v, want ErrUnknownType", err)
	}

	noSource := r.looked("x")
	noSource.Source = ""
	if err := target.Bus.Publish(ctx, noSource); !errors.Is(err, eventbus.ErrInvalidEnvelope) {
		t.Errorf("publish without a source = %v, want ErrInvalidEnvelope", err)
	}

	badPayload := r.looked("x")
	badPayload.Payload = map[string]any{"entity": "not an object"}
	if err := target.Bus.Publish(ctx, badPayload); err == nil {
		t.Error("a payload that does not match the schema of its type was published")
	}
}

// topicPolicyOfPlayerEvents: player_events carries what a person, a CI harness
// or a simulator did — never what an agent or the system decided (C-01).
func topicPolicyOfPlayerEvents(t *testing.T, target Target) {
	r := newRun(t, target)
	ctx := t.Context()

	system := r.looked("x")
	system.Meta.ActorKind = eventbus.ActorSystem
	if err := target.Bus.Publish(ctx, system); !errors.Is(err, eventbus.ErrPolicyViolation) {
		t.Errorf("publish of actor_kind=system to player_events = %v, want ErrPolicyViolation", err)
	}

	withAgent := r.looked("x")
	withAgent.Meta.Agent = &eventbus.AgentRef{ID: "agent-1", Level: "domain", Blueprint: "dark-forest"}
	if err := target.Bus.Publish(ctx, withAgent); !errors.Is(err, eventbus.ErrPolicyViolation) {
		t.Errorf("publish with meta.agent to player_events = %v, want ErrPolicyViolation", err)
	}
}

// topicPolicyOfTheSwarmTopics: an event of the swarm that does not name the
// agent behind it cannot be replayed, budgeted or audited, so the policy
// rejects it.
func topicPolicyOfTheSwarmTopics(t *testing.T, target Target) {
	r := newRun(t, target)
	ctx := t.Context()

	if err := target.Bus.Publish(ctx, r.tick(false)); !errors.Is(err, eventbus.ErrPolicyViolation) {
		t.Errorf("publish of tick.fired without meta.agent = %v, want ErrPolicyViolation", err)
	}
	if err := target.Bus.Publish(ctx, r.tick(true)); err != nil {
		t.Errorf("publish of tick.fired with meta.agent = %v, want it accepted", err)
	}
}

// journalReadsByIncreasingOffset: ReadRange delivers [from, to) strictly by
// increasing offset and returns the offset to continue from (C-01 guarantee).
func journalReadsByIncreasingOffset(t *testing.T, target Target) {
	r := newRun(t, target)
	ctx := t.Context()
	base := r.end(eventbus.TopicPlayerEvents)

	const n = 4
	for i := range n {
		r.publish(r.looked(strconv.Itoa(i)))
	}

	var offsets []int64
	next, err := target.Journal.ReadRange(ctx, eventbus.TopicPlayerEvents, base, base+n, func(ctx context.Context, ev eventbus.Event) error {
		pos, ok := eventbus.PositionFromContext(ctx)
		if !ok {
			t.Error("the journal called the handler without a position in the context")
		}
		offsets = append(offsets, pos.Offset)
		return nil
	})
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if next != base+n {
		t.Fatalf("ReadRange returned next = %d, want %d (the offset to continue from)", next, base+n)
	}
	if len(offsets) != n {
		t.Fatalf("ReadRange delivered %d events, want %d", len(offsets), n)
	}
	for i, off := range offsets {
		if want := base + int64(i); off != want {
			t.Fatalf("offset %d = %d, want %d: ReadRange is strictly increasing and has no gaps", i, off, want)
		}
	}

	// A second, narrower range starts where it is told to, not where the
	// previous read stopped.
	var second []int64
	if _, err := target.Journal.ReadRange(ctx, eventbus.TopicPlayerEvents, base+1, base+3, func(ctx context.Context, _ eventbus.Event) error {
		pos, _ := eventbus.PositionFromContext(ctx)
		second = append(second, pos.Offset)
		return nil
	}); err != nil {
		t.Fatalf("ReadRange of a sub-range: %v", err)
	}
	if len(second) != 2 || second[0] != base+1 || second[1] != base+2 {
		t.Fatalf("sub-range delivered %v, want [%d %d]", second, base+1, base+2)
	}

	// An empty range delivers nothing and does not move the cursor.
	called := false
	next, err = target.Journal.ReadRange(ctx, eventbus.TopicPlayerEvents, base, base, func(context.Context, eventbus.Event) error {
		called = true
		return nil
	})
	if err != nil || next != base || called {
		t.Fatalf("empty ReadRange = (%d, %v), handler called = %v; want (%d, nil, false)", next, err, called, base)
	}
}

// journalEndIsMonotonic: End is the offset the next message will get, and it
// never goes backwards (C-01 guarantee; it is the "am I caught up" test of
// every consumer that resumes from a snapshot).
func journalEndIsMonotonic(t *testing.T, target Target) {
	r := newRun(t, target)

	previous := r.end(eventbus.TopicPlayerEvents)
	for i := range 3 {
		r.publish(r.looked(strconv.Itoa(i)))
		end := r.end(eventbus.TopicPlayerEvents)
		if end < previous {
			t.Fatalf("End went backwards: %d then %d", previous, end)
		}
		if end != previous+1 {
			t.Fatalf("End after one publish = %d, want %d: one event moves the watermark by one", end, previous+1)
		}
		previous = end
	}
	// Reading does not move it.
	if end := r.end(eventbus.TopicPlayerEvents); end != previous {
		t.Fatalf("End changed without a publish: %d then %d", previous, end)
	}
}

// journalStopsAtTheEnd: a catch-up read must terminate. Asking for more than
// the journal holds delivers what there is and returns the offset to continue
// from, rather than waiting for an event that may never be published.
func journalStopsAtTheEnd(t *testing.T, target Target) {
	r := newRun(t, target)
	ctx, cancel := context.WithTimeout(t.Context(), Timeout)
	defer cancel()

	base := r.end(eventbus.TopicPlayerEvents)
	r.publish(r.looked("only"))

	count := 0
	next, err := target.Journal.ReadRange(ctx, eventbus.TopicPlayerEvents, base, base+1000, func(context.Context, eventbus.Event) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("ReadRange past the end: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("ReadRange past the end waited for the context to expire instead of stopping at the end of the journal")
	}
	if count != 1 || next != base+1 {
		t.Fatalf("ReadRange past the end delivered %d events and returned %d, want 1 and %d", count, next, base+1)
	}

	// Starting beyond the end is not an error either: a consumer whose
	// snapshot is newer than the topic is simply caught up.
	next, err = target.Journal.ReadRange(ctx, eventbus.TopicPlayerEvents, next, next+10, func(context.Context, eventbus.Event) error {
		t.Error("ReadRange delivered an event from beyond the end of the journal")
		return nil
	})
	if err != nil || next != base+1 {
		t.Fatalf("ReadRange from the end = (%d, %v), want (%d, nil)", next, err, base+1)
	}
}

// journalTailFollows: Tail keeps delivering until the caller stops it, and
// stopping that way is not a failure.
func journalTailFollows(t *testing.T, target Target) {
	r := newRun(t, target)
	base := r.end(eventbus.TopicPlayerEvents)

	ctx, cancel := context.WithCancel(t.Context())
	received := make(chan string, 8)
	done := make(chan error, 1)
	var offsets []int64
	go func() {
		done <- target.Journal.Tail(ctx, eventbus.TopicPlayerEvents, base, func(ctx context.Context, ev eventbus.Event) error {
			pos, ok := eventbus.PositionFromContext(ctx)
			if !ok {
				t.Error("Tail called the handler without a position in the context")
			}
			offsets = append(offsets, pos.Offset)
			if ev.Key() == r.world {
				received <- payloadName(ev)
			}
			return nil
		})
	}()

	r.publish(r.looked("first"))
	r.publish(r.looked("second"))
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("Tail delivered %q, want %q", got, want)
			}
		case <-testkit.After(Timeout):
			t.Fatalf("Tail did not deliver %q within %s", want, Timeout)
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Tail stopped by its caller returned %v, want nil", err)
		}
	case <-testkit.After(Timeout):
		t.Fatal("Tail did not return after its context was cancelled")
	}

	for i, off := range offsets {
		if want := base + int64(i); off != want {
			t.Fatalf("Tail reported offset %d for event %d, want %d", off, i, want)
		}
	}
}

// groupResumesFromItsCursor: a consumer group remembers how far it got. It is
// what every context of the platform relies on when it restarts — the cursor
// of a group and the cursor in a snapshot are the two halves of the recovery
// of ADR-011 p. 4 — and it is the one thing an in-process stub is tempted to
// get wrong by starting from the beginning every time.
func groupResumesFromItsCursor(t *testing.T, target Target) {
	r := newRun(t, target)

	first := r.subscribe(eventbus.TopicPlayerEvents)
	r.publish(r.looked("before"))
	first.wait(t, 1)
	first.stop(t)

	r.publish(r.looked("after"))

	second := r.subscribe(eventbus.TopicPlayerEvents)
	defer second.stop(t)
	got := second.wait(t, 1)
	if got[0].payloadName() != "after" {
		t.Fatalf("the resumed subscription received %q, want \"after\": a group starts where it left off, not at the beginning",
			got[0].payloadName())
	}
}

// dedupDropsTheRepeat: delivery is at-least-once, so the same event may arrive
// twice; turning that into at-most-once processing is the duty of the consumer
// and eventbus.Dedup is what it uses (C-01).
func dedupDropsTheRepeat(t *testing.T, target Target) {
	r := newRun(t, target)

	// Both events are built before the subscription so that the handler can
	// recognise the repeat and count every delivery of it, dedup or no dedup.
	// Without that count the case passes on a single delivery exactly as it
	// does on two, so it says nothing about at-least-once: a Duplicate that
	// quietly stopped duplicating — the stub's chaos switch turned into a
	// no-op, the live target publishing once — would leave the gate green
	// (review #2 of T-014, Major-3).
	repeated := r.looked("repeated")
	marker := r.looked("marker")

	window := testkit.NewDedup(0)
	var delivered atomic.Int64
	var mu sync.Mutex
	var handled []string
	sub := r.subscribeWith(eventbus.TopicPlayerEvents, func(_ context.Context, ev eventbus.Event) error {
		if ev.Key() != r.world {
			return nil
		}
		if ev.ID == repeated.ID {
			delivered.Add(1)
		}
		if window.Seen(ev.ID) {
			return nil
		}
		mu.Lock()
		handled = append(handled, ev.ID)
		mu.Unlock()
		return nil
	})
	defer sub.stop(t)

	if err := target.Duplicate(t.Context(), repeated); err != nil {
		t.Fatalf("duplicate publish: %v", err)
	}
	r.publish(marker)

	// The marker is published after the repeat, and a topic keeps its order:
	// once the marker has been handled, both copies of the repeat have been
	// delivered.
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(handled) >= 2
	}, "the marker event to be handled")

	if got := delivered.Load(); got < 2 {
		t.Fatalf("the repeated event %s was delivered %d time(s): the duplicate mode of %s produced no repeat, so nothing here exercises dedup",
			repeated.ID, got, target.Name)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(handled) != 2 || handled[0] != repeated.ID || handled[1] != marker.ID {
		t.Fatalf("handled %v, want the repeated event once (%s) and the marker (%s)", handled, repeated.ID, marker.ID)
	}
}

// twoStepDedupAfterTheSideEffect: a consumer whose only side effect is one
// publish asks with Has and remembers with Add after the side effect has
// succeeded (C-01 v1.4, ADR-027 p. 3). The first attempt fails; the bus's own
// retry must then do the work, and the repeat that follows must be dropped.
// With Seen the retry would be dropped too and the work lost without a trace
// (review #1 of T-220, Mi-1) — which is why this runs on the transport, whose
// retry and redelivery the consumer relies on, and not only on the window.
func twoStepDedupAfterTheSideEffect(t *testing.T, target Target) {
	r := newRun(t, target)

	repeated := r.looked("repeated")
	marker := r.looked("marker")

	window := testkit.NewDedup(0)
	var delivered, failed, sideEffects, dropped atomic.Int64
	markerHandled := make(chan struct{}, 1)
	sub := r.subscribeWith(eventbus.TopicPlayerEvents, func(_ context.Context, ev eventbus.Event) error {
		switch ev.ID {
		case marker.ID:
			// Non-blocking: delivery is at-least-once, and a marker arriving
			// twice on a broker must not hang the handler on a full channel.
			select {
			case markerHandled <- struct{}{}:
			default:
			}
			return nil
		case repeated.ID:
		default:
			return nil
		}
		delivered.Add(1)
		if window.Has(ev.ID) {
			dropped.Add(1)
			return nil
		}
		if failed.Load() == 0 {
			failed.Add(1)
			return errors.New("contract: the side effect of the first attempt fails")
		}
		sideEffects.Add(1)
		window.Add(ev.ID)
		return nil
	})
	defer sub.stop(t)

	if err := target.Duplicate(t.Context(), repeated); err != nil {
		t.Fatalf("duplicate publish: %v", err)
	}
	r.publish(marker)

	// The marker follows the repeat in one topic, so once it is handled the
	// failed attempt, its retry and the second copy have all been delivered.
	select {
	case <-markerHandled:
	case <-testkit.After(Timeout):
		t.Fatal("the marker event was never handled")
	}

	if got := delivered.Load(); got < 3 {
		t.Fatalf("the repeated event %s reached the handler %d time(s), want at least 3 (a failed attempt, its retry, the duplicate): nothing here exercises the two steps on %s",
			repeated.ID, got, target.Name)
	}
	if got := sideEffects.Load(); got != 1 {
		t.Errorf("the side effect ran %d time(s), want exactly once: 0 means the failed attempt was remembered, 2 that the duplicate was not dropped", got)
	}
	if dropped.Load() < 1 {
		t.Error("the duplicate after the successful attempt was not dropped")
	}
}

// uncommittedIsDeliveredAgain: delivery is at-least-once because the offset is
// committed only after the event is accounted for. An event whose delivery was
// interrupted — the handler was still working when the subscription was
// cancelled — was not accounted for, so the next subscription of the same
// group gets it again. It is what makes a context that died mid-event recover
// rather than lose it, and the two implementations do it by different means:
// an uncommitted kafka offset against a cursor that did not move.
func uncommittedIsDeliveredAgain(t *testing.T, target Target) {
	r := newRun(t, target)
	ev := r.looked("uncommitted")

	// The slowest case of the suite: it joins the consumer group twice, and on
	// a loaded runner the two waits together took 18.8 s of the 30 s they had
	// between them (review #2 of T-014, Minor-5). One deadline for both is
	// what gives either of them the whole 30 s if it needs it, while a hang
	// still costs the red run the same 2 × Timeout it costs today.
	deadline := testkit.Wall().Now().Add(2 * Timeout)

	reached := make(chan struct{}, 1)
	first := r.subscribeWith(eventbus.TopicPlayerEvents, func(ctx context.Context, got eventbus.Event) error {
		if got.ID != ev.ID {
			return nil
		}
		select {
		case reached <- struct{}{}:
		default:
		}
		// Hold the event until the subscription is cancelled and report the
		// cancellation: an interrupted delivery is not a handled event.
		<-ctx.Done()
		return ctx.Err()
	})
	r.publish(ev)
	select {
	case <-reached:
	case <-testkit.After(deadline.Sub(testkit.Wall().Now())):
		first.stop(t)
		t.Fatalf("the first subscription did not receive the event within %s", 2*Timeout)
	}
	first.stop(t)

	second := r.subscribe(eventbus.TopicPlayerEvents)
	defer second.stop(t)
	got := second.waitUntil(t, deadline, 1)
	if got[0].ev.ID != ev.ID {
		t.Fatalf("the next subscription of the group received %q, want the uncommitted event %q again", got[0].ev.ID, ev.ID)
	}
}

// cancellingStopsOnTheBacklog: a cancelled subscription stops, it does not
// finish the topic first.
//
// Both implementations return nil on a cancellation, which is easy to get
// right and says nothing about when they stop. The stub used to check the
// context only where it was parked at the end of the log, so a cancelled
// subscription with a backlog quietly delivered the whole of it, while the
// broker stops within one event — the fetch or the commit fails. A consumer
// test shaped "publish N, cancel, expect N handled" was then green in the unit
// job and red against Redpanda, which is exactly the lie this suite exists to
// catch (review #1 of T-014, Major-1).
func cancellingStopsOnTheBacklog(t *testing.T, target Target) {
	r := newRun(t, target)

	// A backlog no handler can drain in the time the case gives it: forty
	// events at 25 ms each is a full second of work, and the cancellation
	// comes after the third.
	const backlog = 40
	const perEvent = 25 * time.Millisecond

	var calls atomic.Int64
	ctx, cancel := context.WithCancel(context.WithoutCancel(t.Context()))
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- target.Bus.Subscribe(ctx, eventbus.TopicPlayerEvents, r.group, func(_ context.Context, ev eventbus.Event) error {
			if ev.Key() != r.world {
				return nil
			}
			calls.Add(1)
			time.Sleep(perEvent)
			return nil
		})
	}()

	for range backlog {
		r.publish(r.looked("backlog"))
	}
	waitFor(t, func() bool { return calls.Load() >= 3 }, "the subscription to start on the backlog")

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a cancelled subscription returned %v, want nil", err)
		}
	case <-testkit.After(Timeout):
		t.Fatalf("Subscribe did not return within %s of its context being cancelled", Timeout)
	}
	atReturn := calls.Load()

	// Half the backlog, not all of it: a subscription that stops when it is
	// told stops within a few events (both implementations handle three of the
	// forty), and "39 of 40" is the divergence this case exists for just as
	// much as "40 of 40" is.
	if atReturn >= backlog/2 {
		t.Errorf("the cancelled subscription handled %d of the %d events of the backlog: cancelling a subscription stops it, it does not tell it to hurry",
			atReturn, backlog)
	}
	// And nothing arrives after Subscribe has returned: the loop is gone, not
	// merely detached from its caller.
	time.Sleep(20 * perEvent)
	if after := calls.Load(); after != atReturn {
		t.Errorf("the handler was called %d more times after Subscribe returned", after-atReturn)
	}
	t.Logf("%s: %d of %d events of the backlog were handled before the cancelled subscription returned", target.Name, atReturn, backlog)
}

// invalidOnReadGoesToDeadLetters: validation on read is the defence in depth of
// SEC-16. An envelope that a publisher could not have produced — because
// Publish would have refused it — is parked without ever reaching the handler.
func invalidOnReadGoesToDeadLetters(t *testing.T, target Target) {
	r := newRun(t, target)

	// A well-formed envelope of a registered type whose payload the schema
	// rejects: entity is required by player.looked.v1.
	invalid := r.looked("x")
	invalid.Payload = map[string]any{}
	body, err := json.Marshal(invalid)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := target.Append(eventbus.TopicAnalyticsEvents, body); err != nil {
		t.Fatalf("append the invalid event: %v", err)
	}

	var calls atomic.Int64
	sub := r.subscribeWith(eventbus.TopicAnalyticsEvents, func(context.Context, eventbus.Event) error {
		calls.Add(1)
		return nil
	})
	defer sub.stop(t)

	dl := r.waitForDeadLetter(t, func(dl eventbus.DeadLetter) bool {
		return dl.Original.ID == invalid.ID
	})
	if dl.Attempts != 0 {
		t.Errorf("dead letter of an invalid event has Attempts = %d, want 0: the handler is never called", dl.Attempts)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("the handler was called %d times for an event rejected on read, want 0", got)
	}
}

// withoutValidationOnReadItReachesTheHandler is the other position of the
// switch. MV_BUS_VALIDATE_ON_READ defaults to on and every target of this
// suite is built with it on, so the suite would pass just as happily if a
// context passed the flag inverted — the check would simply always be on.
// Here the same event that the previous case parked reaches the handler, and
// nothing is parked.
func withoutValidationOnReadItReachesTheHandler(t *testing.T, target Target) {
	r := newRun(t, target)

	bus, release, err := target.Lenient()
	if err != nil {
		t.Fatalf("build the transport without validation on read: %v", err)
	}
	defer release()

	invalid := r.looked("x")
	invalid.Payload = map[string]any{} // entity is required by player.looked.v1
	body, err := json.Marshal(invalid)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := target.Append(eventbus.TopicAnalyticsEvents, body); err != nil {
		t.Fatalf("append the invalid event: %v", err)
	}

	reached := make(chan struct{}, 1)
	sub := r.subscribeOn(bus, eventbus.TopicAnalyticsEvents, func(_ context.Context, ev eventbus.Event) error {
		if ev.ID != invalid.ID {
			return nil
		}
		select {
		case reached <- struct{}{}:
		default:
		}
		return nil
	})
	defer sub.stop(t)

	select {
	case <-reached:
	case <-testkit.After(Timeout):
		t.Fatalf("an event the schema rejects did not reach the handler within %s with validation on read turned off", Timeout)
	}

	letters, err := target.DeadLetters(t.Context())
	if err != nil {
		t.Fatalf("read dead letters: %v", err)
	}
	for _, dl := range letters {
		if dl.Consumer == r.group && dl.Original.ID == invalid.ID {
			t.Errorf("the event reached the handler and was parked as well: %s", dl.Error)
		}
	}
}

// undecodableGoesToDeadLetters: a body that is not JSON at all cannot be handed
// to a handler, so it goes straight to dead_letters and the topic moves on.
func undecodableGoesToDeadLetters(t *testing.T, target Target) {
	r := newRun(t, target)

	if err := target.Append(eventbus.TopicAnalyticsEvents, []byte("}{ this is not an event")); err != nil {
		t.Fatalf("append the undecodable message: %v", err)
	}

	var calls atomic.Int64
	sub := r.subscribeWith(eventbus.TopicAnalyticsEvents, func(context.Context, eventbus.Event) error {
		calls.Add(1)
		return nil
	})
	defer sub.stop(t)

	dl := r.waitForDeadLetter(t, func(dl eventbus.DeadLetter) bool {
		return len(dl.Raw) > 0
	})
	if dl.Attempts != 0 {
		t.Errorf("dead letter of an undecodable message has Attempts = %d, want 0", dl.Attempts)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("the handler was called %d times for an undecodable message, want 0", got)
	}
}

// retriesThenDeadLetter: a failing handler is retried three times — four calls
// in all — and the event is then parked so that one bad event cannot block a
// topic (ADR-007 p. 6, decision of 2026-09-09).
func retriesThenDeadLetter(t *testing.T, target Target) {
	r := newRun(t, target)

	poison := r.looked("poison")
	good := r.looked("good")

	var calls atomic.Int64
	handled := make(chan string, 4)
	sub := r.subscribeWith(eventbus.TopicPlayerEvents, func(_ context.Context, ev eventbus.Event) error {
		switch ev.ID {
		case poison.ID:
			calls.Add(1)
			return errors.New("contract: the handler refuses this event")
		case good.ID:
			handled <- ev.ID
		}
		return nil
	})
	defer sub.stop(t)

	r.publish(poison)
	r.publish(good)

	select {
	case <-handled:
	case <-testkit.After(Timeout):
		t.Fatal("the event after the parked one was never handled: a dead letter must not block the topic")
	}

	if got := calls.Load(); got != int64(Retries+1) {
		t.Errorf("the handler was called %d times, want %d (three retries after the first call)", got, Retries+1)
	}
	dl := r.waitForDeadLetter(t, func(dl eventbus.DeadLetter) bool {
		return dl.Original.ID == poison.ID
	})
	if dl.Original.ID != poison.ID {
		t.Errorf("dead letter carries %q, want the failing event %q", dl.Original.ID, poison.ID)
	}
	if dl.Attempts != Retries+1 {
		t.Errorf("dead letter has Attempts = %d, want %d", dl.Attempts, Retries+1)
	}
	if dl.Error == "" {
		t.Error("the dead letter does not say why the handler failed")
	}
	if dl.FailedAt.IsZero() {
		t.Error("the dead letter does not say when it failed")
	}
}

// subscribeReturnsNilOnStop: a subscription cancelled by its caller is a
// shutdown, not a failure — decision Mi-1 on the review of T-005, and the one
// semantic difference that would otherwise make every context log an error on
// every clean stop.
func subscribeReturnsNilOnStop(t *testing.T, target Target) {
	r := newRun(t, target)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- target.Bus.Subscribe(ctx, eventbus.TopicPlayerEvents, r.group, func(context.Context, eventbus.Event) error {
			return nil
		})
	}()

	// Cancel while the subscription is parked on an empty tail, which is
	// where a context is cancelled in production.
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Subscribe stopped by its caller returned %v, want nil", err)
		}
	case <-testkit.After(Timeout):
		t.Fatal("Subscribe did not return after its context was cancelled")
	}
}

// publishOfOneEventIsNotBatched measures what a single publish actually costs.
// MVP-1 publishes one event at a time into single-partition topics, so a
// client that waits for a batch to fill pays its whole batch timeout on every
// event: with the kafka-go defaults that is one second against the 300 ms of
// NFR-001. Only a live broker can tell whether the writer settings still hold.
func publishOfOneEventIsNotBatched(t *testing.T, target Target) {
	r := newRun(t, target)

	// The first publish of a topic pays for the connection and the metadata
	// round trip; what is measured is the steady state.
	r.publish(r.looked("warm-up"))

	const samples = 5
	wall := testkit.Wall()
	var worst time.Duration
	for i := range samples {
		ev := r.looked("sample-" + strconv.Itoa(i))
		start := wall.Now()
		r.publish(ev)
		if elapsed := wall.Now().Sub(start); elapsed > worst {
			worst = elapsed
		}
	}
	t.Logf("%s: slowest of %d single publishes = %s", target.Name, samples, worst)
	if target.Live && worst > PublishBudget {
		t.Errorf("the slowest single publish took %s, over the %s budget: the writer is waiting for a batch it will never fill (NFR-001)",
			worst, PublishBudget)
	}
}

// bigUndecodableBodyIsTruncated: the body carried into a dead letter is cut to
// MaxDeadLetterRaw, and the cut exists for the transport. A reader accepts a
// record far larger than a writer may produce, so a dead letter that quoted a
// big message whole would be refused by the broker (max.message.bytes, 1 MiB
// by default), its offset would never be committed, and the topic would stall
// on the very message dead_letters exists to get it past. In memory the cut is
// only arithmetic — which is why the unit test of it proves nothing about the
// case it was written for.
func bigUndecodableBodyIsTruncated(t *testing.T, target Target) {
	r := newRun(t, target)

	// The size is chosen against the 1 MiB default of max.message.bytes: the
	// message itself fits under it, but the dead letter quoting it whole would
	// not, because Raw is carried as base64 and grows by a third. Cut to
	// MaxDeadLetterRaw the wrapper is about 680 KiB and goes through.
	big := make([]byte, 900<<10)
	for i := range big {
		big[i] = 'x'
	}
	copy(big, []byte("}{ this is not an event, and there is far too much of it "))
	if err := target.Append(eventbus.TopicAnalyticsEvents, big); err != nil {
		t.Fatalf("append the big message: %v", err)
	}

	sub := r.subscribe(eventbus.TopicAnalyticsEvents)
	defer sub.stop(t)

	dl := r.waitForDeadLetter(t, func(dl eventbus.DeadLetter) bool { return dl.RawTruncated })
	if len(dl.Raw) != eventbus.MaxDeadLetterRaw {
		t.Errorf("the dead letter carries %d bytes of the body, want the first %d", len(dl.Raw), eventbus.MaxDeadLetterRaw)
	}

	// And the topic moved on: what follows the big message is handled.
	marker := r.looked("after the big one")
	body, err := json.Marshal(marker)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := target.Append(eventbus.TopicAnalyticsEvents, body); err != nil {
		t.Fatalf("append the marker: %v", err)
	}
	got := sub.wait(t, 1)
	if got[0].ev.ID != marker.ID {
		t.Fatalf("the subscription received %q after the big message, want the marker %q", got[0].ev.ID, marker.ID)
	}
}

// closeStopsEverything: Close is the third method of the Bus interface, and
// every context of the platform calls it on shutdown (runtime.Deps). A running
// subscription must end the way a cancelled one does — with nil, because
// closing the bus is an orderly stop — and a publish afterwards must be
// refused rather than silently dropped.
//
// The two implementations arrive there differently: the kafka adapter closes
// its readers, the fetch fails with io.ErrClosedPipe and eventbus.stopped
// turns that into nil, while the stub closes a channel its loops select on.
// Neither path was covered before (review #1 of T-014, Major-2): a stub
// returning ErrClosed instead of nil left the whole suite green.
//
// One Close has to end subscriptions in both of the shapes a subscription can
// be in when a context stops, because they are different code on both sides.
// One is parked at the end of its topic with nothing to read, which is what
// almost every subscription of the platform is doing when runtime.Deps.Stop
// runs — and it was the shape no case covered: a stub returning ErrClosed from
// its parked select instead of nil left the whole suite green (review #2 of
// T-014, Major-2). The other is behind a backlog, for the same reason the
// cancellation case has one: closing a bus stops its readers, it does not ask
// them to finish the topic first.
//
// It runs last. The target is unusable afterwards.
func closeStopsEverything(t *testing.T, target Target) {
	if target.Close == nil {
		t.Skip("the target cannot be closed")
	}
	r := newRun(t, target)

	// The parked subscription. It reads a topic of its own so that the backlog
	// below cannot wake it, and the marker event is how the case knows it has
	// caught up: once its own event has been handled there is nothing left in
	// the topic, so the loop is where a shutdown finds it. Its consumer group
	// is its own as well: on a broker two subscriptions sharing a group are
	// two members to rebalance, and the case has no use for the coupling
	// (review #3 of T-014, Nit-2).
	parked := r.subscribeGroup(eventbus.TopicSystemEvents, r.group+"-parked")
	r.publish(r.tick(true))
	parked.wait(t, 1)

	const backlog = 300
	// Short enough that a loop nobody stopped gets through many events while
	// Close is closing readers, long enough that the events are still
	// countable one by one.
	const perEvent = 3 * time.Millisecond
	// The event whose handler hands the case the bus: far enough in to prove
	// the subscription is running, near enough to leave the backlog undrained.
	const signalOn = 20
	// What a loop that was told to stop may still deliver between the moment
	// the case lets the handlers go and the moment Close reaches the loops.
	// Nothing is in flight at that moment — every handler is parked inside
	// the signal event — so the healthy count is exactly the baseline, and it
	// was, in all 64 measurements of iteration 4, 28 of them under
	// GOMAXPROCS=1. The one event of slack is for a runner that descheduled
	// the case between the release and the cancellation; a mutant that only
	// manages one event on one of the subscriptions is caught by the others,
	// which is why the slack costs nothing here.
	const slack = 1

	// Three subscriptions behind the backlog, not one, and that is the point
	// of the case rather than a detail of it.
	//
	// What separates a bus that stops its loops from a bus that only closes
	// its transport is what the loops do while Close runs: the first stops
	// them at once, the second lets them drain the backlog for as long as
	// Close takes — and closing a kafka reader takes tens of milliseconds,
	// because it means leaving a consumer group over the network. But a loop
	// can only show that while its own reader is still in that group: Close
	// walks the readers one after another, and the loop whose reader goes
	// first is stopped by its dead reader rather than by the cancellation the
	// case is about, so its own count says nothing. Which one goes first is
	// the iteration order of a map.
	//
	// One subscription therefore made the case a coin toss: it went green
	// whenever its reader happened to be closed first (review #3 of T-014,
	// Major-1 — six green runs in thirty-five under mutation). With several,
	// only one of them can be first: the rest still have whole reader closes
	// in front of them, the last of them two, and the assertion no longer
	// rides on the order of a map. Three is where the shortest reader close
	// measured on this machine, about nine milliseconds, still buys the last
	// subscription several events of backlog over the slack.
	//
	// Every handler parks itself on the signal event and stays there until the
	// case lets it go, immediately before Close. That is what makes the
	// baseline exact: nothing can be delivered between the count the case
	// takes as its baseline and the count it compares against it.
	release := make(chan struct{})
	drains := []*backlogReader{
		{name: "a", signal: make(chan struct{}, 1)},
		{name: "b", signal: make(chan struct{}, 1)},
		{name: "c", signal: make(chan struct{}, 1)},
	}
	for _, d := range drains {
		d.sub = r.subscribeWithGroup(eventbus.TopicPlayerEvents, r.group+"-"+d.name,
			d.handler(r.world, signalOn, perEvent, release))
	}
	defer func() {
		parked.cancel()
		for _, d := range drains {
			d.sub.cancel()
		}
	}()

	for range backlog {
		r.publish(r.looked("backlog"))
	}
	// One budget for both waits rather than one each: the two subscriptions
	// reach the signal event within milliseconds of one another, and a case
	// that waits twice must not cost the red run twice (review #2 of T-014,
	// Minor-5).
	deadline := testkit.Wall().Now().Add(Timeout)
	for _, d := range drains {
		select {
		case <-d.signal:
		case <-testkit.After(deadline.Sub(testkit.Wall().Now())):
			t.Fatalf("subscription %s did not reach event %d of the backlog within %s", d.name, signalOn, Timeout)
		}
		d.before = d.calls.Load()
	}

	// The handlers are released and the bus is closed with nothing in between:
	// the loops are then closed over between a handler and the commit of its
	// offset, which is where the kafka adapter used to hang, because a commit
	// handed to a reader whose goroutines Close has just stopped waits for a
	// reply nobody will send.
	close(release)
	if err := target.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	ended := []struct {
		what string
		sub  *subscription
	}{{"the subscription parked at the end of its topic", parked}}
	for _, d := range drains {
		ended = append(ended, struct {
			what string
			sub  *subscription
		}{"the subscription " + d.name + " on the backlog", d.sub})
	}
	for _, e := range ended {
		select {
		case err := <-e.sub.done:
			if err != nil {
				t.Fatalf("Subscribe returned %v for %s, want nil: closing the bus is an orderly stop", err, e.what)
			}
		case <-testkit.After(Timeout):
			t.Fatalf("Close left %s running after %s", e.what, Timeout)
		}
	}
	// Nothing beyond the event already in flight, on either of them.
	for _, d := range drains {
		if got := d.calls.Load(); got > d.before+slack {
			t.Errorf("subscription %s handled %d more events while the bus was closing (%d of the %d events of the backlog in all): Close stops a reader, it does not ask it to finish",
				d.name, got-d.before, got, backlog)
		}
	}

	if err := target.Bus.Publish(t.Context(), r.looked("after the close")); !errors.Is(err, eventbus.ErrClosed) {
		t.Fatalf("Publish on a closed bus = %v, want ErrClosed", err)
	}
}

// backlogReader is one of the subscriptions the Close case leaves behind a
// backlog of events.
type backlogReader struct {
	name   string
	sub    *subscription
	signal chan struct{}
	calls  atomic.Int64
	before int64
}

// handler counts what it is given and spends a little time on every event, so
// that a loop still running while the bus closes is visible in the count. On
// the event named by signalOn it hands the case the bus and parks itself there
// until the case lets it go.
func (d *backlogReader) handler(world string, signalOn int64, perEvent time.Duration, release <-chan struct{}) eventbus.Handler {
	return func(_ context.Context, ev eventbus.Event) error {
		if ev.Key() != world {
			return nil
		}
		if d.calls.Add(1) != signalOn {
			time.Sleep(perEvent)
			return nil
		}
		d.signal <- struct{}{}
		// The fuse. A case that fails before it lets the handlers go would
		// otherwise hold its subscription — and with it the -timeout of the
		// whole run — instead of failing inside its own budget.
		select {
		case <-release:
		case <-testkit.After(Timeout):
		}
		return nil
	}
}

// --- the scaffolding of one case -------------------------------------------

// run is one case: its own world, its own consumer group, its own identifier
// prefix. Nothing it publishes can be confused with what another case left in
// the topics of a broker that outlives them all.
type run struct {
	t      *testing.T
	target Target
	world  string
	group  string
}

func newRun(t *testing.T, target Target) *run {
	t.Helper()
	n := runSeq.Add(1)
	suffix := target.Name + "-" + strconv.FormatInt(n, 10)
	testkit.Deterministic(t, "ev-"+suffix)
	eventbus.SetRegistry(contracts.Default())
	return &run{
		t:      t,
		target: target,
		world:  "world-" + suffix,
		group:  "group-" + suffix,
	}
}

func (r *run) publish(ev eventbus.Event) {
	r.t.Helper()
	if err := r.target.Bus.Publish(r.t.Context(), ev); err != nil {
		r.t.Fatalf("publish %s: %v", ev.Type, err)
	}
}

func (r *run) end(topic string) int64 {
	r.t.Helper()
	end, err := r.target.Journal.End(r.t.Context(), topic)
	if err != nil {
		r.t.Fatalf("End(%s): %v", topic, err)
	}
	return end
}

// looked builds a valid player.looked, the simplest event of player_events.
func (r *run) looked(name string) eventbus.Event {
	return eventbus.NewRoot("player.looked", contracts.SourceTestkitGateway, r.world,
		&eventbus.ScopeRef{ID: "solo:" + r.world, Type: "solo"}, eventbus.ActorCI,
		map[string]any{
			"entity": map[string]any{
				"entity": map[string]any{"id": "player-" + r.world, "type": "player"},
				"name":   name,
			},
		})
}

// tick builds a tick.fired, an event of a swarm topic, with or without the
// agent its policy demands.
func (r *run) tick(withAgent bool) eventbus.Event {
	stamp := testkit.Epoch.Format(time.RFC3339)
	payload := map[string]any{
		"tick": map[string]any{
			"seq": 1, "mode": "background",
			"scheduled_at": stamp, "fired_at": stamp,
			"lod_allowed": "basic",
		},
		"budget": map[string]any{"window_calls": 0, "cap": 10},
	}
	var opts []eventbus.DeriveOption
	if withAgent {
		opts = append(opts, eventbus.WithAgent(eventbus.AgentRef{
			ID: "agent-" + r.world, Level: "domain", Blueprint: "dark-forest",
		}))
	}
	return eventbus.NewRoot("tick.fired", contracts.SourceTestkitSwarm, r.world, nil,
		eventbus.ActorSystem, payload, opts...)
}

// received is one delivery observed by a subscription.
type received struct {
	ev  eventbus.Event
	pos eventbus.Position
}

func (r received) payloadName() string { return payloadName(r.ev) }

func payloadName(ev eventbus.Event) string {
	name, _ := ev.Path().GetString("entity.name")
	return name
}

// subscription is a Subscribe running in the background.
type subscription struct {
	cancel context.CancelFunc
	done   chan error
	mu     sync.Mutex
	seen   []received
}

// subscribe starts a subscription that collects the events of this run and
// ignores everything else in the topic.
func (r *run) subscribe(topic string) *subscription {
	r.t.Helper()
	return r.subscribeGroup(topic, r.group)
}

// subscribeGroup is subscribe in a consumer group of the case's choosing. A
// case that runs two subscriptions at once wants them independent: on a broker
// two members of one group rebalance each other, on the stub they would share
// a cursor, and neither is what such a case is about.
func (r *run) subscribeGroup(topic, group string) *subscription {
	r.t.Helper()
	sub := &subscription{done: make(chan error, 1)}
	handler := func(ctx context.Context, ev eventbus.Event) error {
		if ev.Key() != r.world {
			return nil
		}
		pos, ok := eventbus.PositionFromContext(ctx)
		if !ok {
			r.t.Error("the subscription called the handler without a position in the context")
		}
		sub.mu.Lock()
		sub.seen = append(sub.seen, received{ev: ev, pos: pos})
		sub.mu.Unlock()
		return nil
	}
	r.start(sub, topic, group, handler)
	return sub
}

// subscribeWith starts a subscription with a handler of the case's own. The
// handler sees every event of the topic, this run's and everybody else's.
func (r *run) subscribeWith(topic string, h eventbus.Handler) *subscription {
	r.t.Helper()
	return r.subscribeWithGroup(topic, r.group, h)
}

// subscribeWithGroup is subscribeWith in a consumer group of the case's
// choosing.
func (r *run) subscribeWithGroup(topic, group string, h eventbus.Handler) *subscription {
	r.t.Helper()
	sub := &subscription{done: make(chan error, 1)}
	r.start(sub, topic, group, h)
	return sub
}

// subscribeOn starts a subscription of this run's group on a bus of the
// case's choosing — the lenient view of the transport, for the one case that
// needs the validation flag in its other position.
func (r *run) subscribeOn(bus eventbus.Bus, topic string, h eventbus.Handler) *subscription {
	r.t.Helper()
	sub := &subscription{done: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.WithoutCancel(r.t.Context()))
	sub.cancel = cancel
	go func() { sub.done <- bus.Subscribe(ctx, topic, r.group, h) }()
	return sub
}

func (r *run) start(sub *subscription, topic, group string, h eventbus.Handler) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(r.t.Context()))
	sub.cancel = cancel
	go func() { sub.done <- r.target.Bus.Subscribe(ctx, topic, group, h) }()
}

// wait blocks until the subscription has collected n events of its run.
func (s *subscription) wait(t *testing.T, n int) []received {
	t.Helper()
	return s.waitUntil(t, testkit.Wall().Now().Add(Timeout), n)
}

// waitUntil is wait against a deadline the case owns.
func (s *subscription) waitUntil(t *testing.T, deadline time.Time, n int) []received {
	t.Helper()
	waitUntil(t, deadline, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.seen) >= n
	}, fmt.Sprintf("%d events", n))
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]received(nil), s.seen...)
}

// stop cancels the subscription and reports what Subscribe returned. The
// check is the point of it: a subscription that refused to start — an unknown
// topic, a closed bus, a group name the broker rejects — is otherwise seen
// only as a timeout in whatever the case waits for next, which says nothing
// about the cause. It also spreads the rule of C-01 that an orderly stop
// returns nil over every case of the suite instead of the one case named
// after it.
func (s *subscription) stop(t *testing.T) {
	t.Helper()
	s.cancel()
	// Against a deadline, because a subscription that does not return at all
	// is the class of defect the suite exists for: without one it costs the
	// -timeout of the whole run rather than the budget of its own case
	// (review #3 of T-014, Nit-1).
	select {
	case err := <-s.done:
		if err != nil {
			t.Errorf("the subscription returned %v, want nil: a subscription stopped by its caller is a shutdown, not a failure", err)
		}
	case <-testkit.After(Timeout):
		t.Errorf("the subscription did not return within %s of its context being cancelled", Timeout)
	}
}

// waitForDeadLetter waits for the one dead letter this run's consumer parked.
// The consumer name is what identifies it: an undecodable message carries no
// envelope to match on.
func (r *run) waitForDeadLetter(t *testing.T, match func(eventbus.DeadLetter) bool) eventbus.DeadLetter {
	t.Helper()
	var found eventbus.DeadLetter
	waitFor(t, func() bool {
		letters, err := r.target.DeadLetters(t.Context())
		if err != nil {
			t.Fatalf("read dead letters: %v", err)
		}
		for _, dl := range letters {
			if dl.Consumer == r.group && match(dl) {
				found = dl
				return true
			}
		}
		return false
	}, "a dead letter from "+r.group)
	return found
}

// waitFor polls cond until it holds or Timeout expires. Polling is what the
// bus itself offers: neither implementation has a "the handler has caught up"
// signal, and inventing one for the test would test the signal.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	waitUntil(t, testkit.Wall().Now().Add(Timeout), cond, what)
}

// waitUntil is waitFor against a deadline the case owns. A case that waits
// twice can then give the two waits one budget between them instead of
// Timeout each: a single wait gets more patience without a hang costing the
// red run any more than it does today (review #2 of T-014, Minor-5).
func waitUntil(t *testing.T, deadline time.Time, cond func() bool, what string) {
	t.Helper()
	for {
		if cond() {
			return
		}
		if testkit.Wall().Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
