package state_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
)

// rigged is the in-process bus with a hook on Publish and a timeline of the
// subscription and of Close, so that a test can fail a publication, panic in
// one or hold one, and see in which order the context let go of the bus.
type rigged struct {
	*membus.Bus

	mu       sync.Mutex
	hook     func(ctx context.Context, ev eventbus.Event) error
	subCtx   context.Context
	subErr   error
	timeline []string
}

func rig(t *testing.T) *rigged { return &rigged{Bus: newBus(t)} }

func (r *rigged) setHook(h func(ctx context.Context, ev eventbus.Event) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hook = h
}

func (r *rigged) Publish(ctx context.Context, ev eventbus.Event) error {
	r.mu.Lock()
	hook := r.hook
	r.mu.Unlock()
	if hook != nil {
		if err := hook(ctx, ev); err != nil {
			return err
		}
	}
	return r.Bus.Publish(ctx, ev)
}

func (r *rigged) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	r.mu.Lock()
	r.subCtx = ctx
	failure := r.subErr
	r.mu.Unlock()
	if failure != nil {
		return failure
	}
	err := r.Bus.Subscribe(ctx, topic, group, h)
	r.note(fmt.Sprintf("subscription returned (context: %v)", ctx.Err()))
	return err
}

func (r *rigged) Close() error {
	r.note("bus closed")
	return r.Bus.Close()
}

func (r *rigged) note(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.timeline = append(r.timeline, s)
}

func (r *rigged) events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.timeline...)
}

func (r *rigged) subscriptionCancelled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.subCtx != nil && r.subCtx.Err() != nil
}

// running starts State on the bus with its own log, and stops it when the test
// ends.
func running(t *testing.T, bus eventbus.Bus, worlds ...string) (*state.Context, *lockedBuffer) {
	t.Helper()
	c, logged, _ := runningOnClock(t, bus, worlds...)
	return c, logged
}

// runningOnClock is running with the manual clock the pauses of State wait on:
// an answer that does not go out is published again only when the test moves
// the clock.
func runningOnClock(t *testing.T, bus eventbus.Bus, worlds ...string) (*state.Context, *lockedBuffer, *clock.Manual) {
	t.Helper()
	sources := testkit.Deterministic(t, "t055")
	eventbus.SetRegistry(contracts.Default())
	logged := &lockedBuffer{}
	c := state.New(state.Config{Worlds: worlds, Timers: sources.Timers,
		Log: slog.New(slog.NewJSONHandler(logged, &slog.HandlerOptions{Level: slog.LevelDebug}))})
	if err := c.Start(context.Background(), runtime.Deps{Bus: bus}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		_ = c.Stop(ctx)
	})
	return c, logged, sources.Clock
}

func publish(t *testing.T, bus eventbus.Bus, events ...eventbus.Event) {
	t.Helper()
	for _, ev := range events {
		if err := bus.Publish(context.Background(), ev); err != nil {
			t.Fatalf("publish %s: %v", ev.Type, err)
		}
	}
}

func inWorld(ev eventbus.Event, worldID string) eventbus.Event {
	ev.World = &eventbus.WorldRef{Entity: eventbus.EntityRef{ID: worldID, Type: "world"}}
	return ev
}

// untilEnd waits until system_events holds at least n records.
func untilEnd(t *testing.T, bus *membus.Bus, n int64) {
	t.Helper()
	waitFor(t, fmt.Sprintf("%d records on %s", n, eventbus.TopicSystemEvents), func() bool {
		end, err := bus.End(context.Background(), eventbus.TopicSystemEvents)
		return err == nil && end >= n
	})
}

// --- the pipeline over the bus ---

// The DoD of T-055: a thousand proposals over the bus move the version by one
// each — no gap, no repeat — and every fact carries the proposal it answers.
func TestAThousandProposalsMoveTheVersionByOneEach(t *testing.T) {
	bus := newBus(t)
	c, _ := running(t, bus, world)
	const n = 1000

	publish(t, bus, create("prop-born", ref("counter", entity.TypeNPC), "", map[string]any{"turns": 0}))
	proposals := make([]eventbus.Event, n)
	for i := range n {
		proposals[i] = update(t, fmt.Sprintf("prop-%04d", i), "author", true,
			set(ref("counter", entity.TypeNPC), nil, op(entity.OpInc, "turns", 1)))
		publish(t, bus, proposals[i])
	}
	untilEnd(t, bus, 2*n+2)

	facts := onTopic(t, bus, state.TypeUpdated)
	if len(facts) != n {
		t.Fatalf("%d facts, want %d", len(facts), n)
	}
	seen := make(map[int]bool, n)
	for i, fact := range facts {
		v, _ := fact.Path().GetInt("version")
		if v != i+2 || seen[v] {
			t.Fatalf("fact %d announces v%d, want v%d: versions must move by exactly one", i, v, i+2)
		}
		seen[v] = true
		if fact.Meta.CausationID != proposals[i].ID || !fact.Timestamp.Equal(proposals[i].Timestamp) {
			t.Fatalf("fact %d is caused by %s at %v, want the proposal %s at %v",
				i, fact.Meta.CausationID, fact.Timestamp, proposals[i].ID, proposals[i].Timestamp)
		}
	}
	if refusals := onTopic(t, bus, state.TypeRejected); len(refusals) != 0 {
		t.Errorf("%d refusals, want none", len(refusals))
	}
	if e, _ := c.Get(world, "counter"); e.Version != n+1 || intAt(e, "turns") != n {
		t.Errorf("counter v%d turns %v, want v%d turns %d", e.Version, e.Attributes["turns"], n+1, n)
	}
}

// C-02 v1.6 over the bus: the literal 9007199254740993 reaches State as the
// float64 2^53 and is refused as invalid_op — in the value of an operation and
// in the attributes of a create.
func TestANumberPastTheSafeRangeOverTheBusIsInvalidOp(t *testing.T) {
	bus := newBus(t)
	c, _ := running(t, bus, world)
	publish(t, bus, create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	untilEnd(t, bus, 2)

	literal := func(ev eventbus.Event) []byte {
		raw, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		out := bytes.Replace(raw, []byte(`"PAST"`), []byte("9007199254740993"), 1)
		if bytes.Equal(out, raw) {
			t.Fatal("no placeholder in the event")
		}
		return out
	}
	setOp := update(t, "prop-set-past", "author", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpSet, "seed", "PAST")))
	createPast := create("prop-create-past", ref("player-B", entity.TypePlayer), "", map[string]any{"seed": "PAST"})
	for _, ev := range []eventbus.Event{setOp, createPast} {
		if err := bus.Append(eventbus.TopicSystemEvents, literal(ev)); err != nil {
			t.Fatal(err)
		}
	}
	untilEnd(t, bus, 6)

	refusals := onTopic(t, bus, state.TypeRejected)
	if len(refusals) != 2 {
		t.Fatalf("%d refusals, want two", len(refusals))
	}
	assertRejected(t, refusals[0], "prop-set-past", state.ReasonInvalidOp, "player-A")
	assertRejected(t, refusals[1], "prop-create-past", state.ReasonInvalidOp, "player-B")
	if _, ok := c.Get(world, "player-B"); ok {
		t.Error("player-B was created with a number past the range")
	}
	if e, _ := c.Get(world, "player-A"); e.Version != 1 {
		t.Errorf("player-A at v%d, want v1", e.Version)
	}
}

// --- an answer that does not go out (review #1 of T-055, Ma-1) ---

// The probe of the review, kept as a regression test. The fact of one entity
// of a package does not go out thirty times — more than the bus would ever
// redeliver the proposal — and the world goes on: the worker publishes the
// same event again until it is out, the bus is not handed an error, nothing is
// parked in dead_letters, and the proposal after it is decided against the
// world the facts announced. Whatever happens, no entity has two different
// facts under one version, and every version the world holds was announced.
func TestNoTwoFactsOfOneVersionWhileAPublicationKeepsFailing(t *testing.T) {
	bus := rig(t)
	c, logged, manual := runningOnClock(t, bus, world)
	publish(t, bus.Bus,
		create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}),
		create("prop-w", ref("wolf-alpha", entity.TypeNPC), "", map[string]any{"hp": 10}))
	untilEnd(t, bus.Bus, 4)

	const failures = 30
	var attempts atomic.Int32
	bus.setHook(func(_ context.Context, ev eventbus.Event) error {
		if id, _ := ev.Path().GetString("entity.entity.id"); ev.Type == state.TypeUpdated && id == "wolf-alpha" &&
			attempts.Add(1) <= failures {
			return errors.New("the broker is away")
		}
		return nil
	})
	publish(t, bus.Bus,
		update(t, "prop-round", "author", true,
			set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)),
			set(ref("wolf-alpha", entity.TypeNPC), version(1), op(entity.OpInc, "hp", -3))),
		update(t, "prop-next", "author", true,
			set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))

	waitFor(t, "the world to report the failed attempts", func() bool {
		section := worldHealth(c, world)
		return section["status"] == runtime.StatusDegraded && section["publish_attempts_failed"] == 1
	})
	if got := c.Health().Status; got != runtime.StatusDegraded {
		t.Errorf("health while an answer is published again %q, want degraded", got)
	}
	waitFor(t, "the answer to prop-next", advancing(manual, func() bool {
		return len(answersTo(t, bus.Bus, "prop-next")) > 0
	}))

	if letters, _ := bus.DeadLetters(); len(letters) != 0 {
		t.Fatalf("dead letters %v, want none: the bus was handed the failure", letters)
	}
	if got := attempts.Load(); got != failures+1 {
		t.Errorf("the fact of wolf-alpha went out at attempt %d, want %d", got, failures+1)
	}
	assertOneFactPerVersion(t, bus.Bus)
	assertAnnounced(t, c, bus.Bus, map[string][2]int{"player-A": {3, 7}, "wolf-alpha": {2, 7}})
	if got := c.Health().Status; got != runtime.StatusOK {
		t.Errorf("health once the answer is out %q, want ok", got)
	}
	if warned := recordsOf(t, logged.String(), "an event of the answer did not go out; it is published again as it is"); len(warned) != failures {
		t.Errorf("%d warnings of an attempt, want %d", len(warned), failures)
	}
}

// Stop ends the attempts: the world stops with publish_failed, the proposal is
// not parked in dead_letters but left uncommitted, the world keeps nothing of
// it, and started again the context decides nothing more of that world — so a
// restart catches up on the facts already out (§4.8) instead of a later
// proposal announcing another fact under their version.
func TestAStopThatEndsTheAttemptsStopsTheWorld(t *testing.T) {
	bus := rig(t)
	c, logged, manual := runningOnClock(t, bus, world)
	publish(t, bus.Bus,
		create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}),
		create("prop-w", ref("wolf-alpha", entity.TypeNPC), "", map[string]any{"hp": 10}))
	untilEnd(t, bus.Bus, 4)

	var attempts atomic.Int32
	bus.setHook(func(_ context.Context, ev eventbus.Event) error {
		if id, _ := ev.Path().GetString("entity.entity.id"); ev.Type == state.TypeUpdated && id == "wolf-alpha" {
			attempts.Add(1)
			return errors.New("the broker is away")
		}
		return nil
	})
	publish(t, bus.Bus, update(t, "prop-round", "author", true,
		set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)),
		set(ref("wolf-alpha", entity.TypeNPC), version(1), op(entity.OpInc, "hp", -3))))
	waitFor(t, "a second attempt", advancing(manual, func() bool { return attempts.Load() >= 2 }))

	ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	if err := c.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if letters, _ := bus.DeadLetters(); len(letters) != 0 {
		t.Fatalf("dead letters %v, want none: the proposal stays uncommitted", letters)
	}
	for _, id := range []string{"player-A", "wolf-alpha"} {
		if e, _ := c.Get(world, id); e.Version != 1 {
			t.Errorf("%s at v%d, want v1: the world kept an answer that did not get out", id, e.Version)
		}
	}
	abandoned := recordsOf(t, logged.String(), "the answer to a proposal was abandoned before it got out; the world is stopped")
	if len(abandoned) != 1 || abandoned[0]["reason"] != "publish_failed" || abandoned[0]["proposal_id"] != "prop-round" ||
		abandoned[0]["handled"] != false {
		t.Errorf("log %v, want one ERROR with reason publish_failed for prop-round", abandoned)
	}

	bus.setHook(nil)
	if err := c.Start(context.Background(), runtime.Deps{Bus: bus}); err != nil {
		t.Fatalf("Start again: %v", err)
	}
	if section := worldHealth(c, world); c.Health().Status != runtime.StatusFail ||
		section["status"] != runtime.StatusFail || section["reason"] != "publish_failed" {
		t.Errorf("health after the restart %+v, want fail with reason publish_failed", c.Health())
	}
	// The abandoned proposal was left uncommitted: the group of the restarted
	// subscription is handed prop-round again, and the stopped world refuses it
	// into dead_letters. Had it been committed on the Stop, it would never come
	// again, and a restart would not see the proposal half of whose answer is
	// already on the bus (review #2 of T-055, Mi-6).
	waitFor(t, "prop-round delivered again and refused by the stopped world", func() bool {
		return refusedByTheStoppedWorld(bus.Bus, "prop-round")
	})
	publish(t, bus.Bus, update(t, "prop-next", "author", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	waitFor(t, "prop-next refused by the stopped world", func() bool {
		return refusedByTheStoppedWorld(bus.Bus, "prop-next")
	})
	assertOneFactPerVersion(t, bus.Bus)
	if len(answersTo(t, bus.Bus, "prop-next")) != 0 {
		t.Error("the stopped world answered prop-next")
	}
	if e, _ := c.Get(world, "player-A"); e.Version != 1 {
		t.Errorf("player-A at v%d, want v1", e.Version)
	}
}

// refusedByTheStoppedWorld reports whether dead_letters holds a proposal parked
// with ErrWorldStopped.
func refusedByTheStoppedWorld(bus *membus.Bus, proposalID string) bool {
	letters, _ := bus.DeadLetters()
	for _, letter := range letters {
		if id, _ := letter.Original.Path().GetString("proposal_id"); id == proposalID &&
			strings.Contains(letter.Error, state.ErrWorldStopped.Error()) {
			return true
		}
	}
	return false
}

// Review #2 of T-055, Mi-5: the pauses between the attempts to publish an
// answer are not domain time. A replay run hands the contexts timers that never
// fire (replay.NullTimers; neverTimers stands in for them, since depguard keeps
// internal/replay out of internal/state), and one failed publication of the
// answer must still be followed by a second attempt: State paces its attempts
// on Config.Timers, real by default, as the bus paces its redeliveries.
func TestAReplayRunPublishesAnAnswerAgainOnRealTimers(t *testing.T) {
	bus := rig(t)
	testkit.Deterministic(t, "t055")
	eventbus.SetRegistry(contracts.Default())
	c := state.New(state.Config{Worlds: []string{world}})
	if err := c.Start(context.Background(), runtime.Deps{Bus: bus, Timers: neverTimers{}}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		_ = c.Stop(ctx)
	})

	var attempts atomic.Int32
	bus.setHook(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == state.TypeCreated && attempts.Add(1) == 1 {
			return errors.New("the broker blinked")
		}
		return nil
	})
	publish(t, bus.Bus, create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	waitFor(t, "the answer to prop-born after one failed attempt", func() bool {
		return len(answersTo(t, bus.Bus, "prop-born")) == 1
	})

	if got := attempts.Load(); got != 2 {
		t.Errorf("the fact went out at attempt %d, want 2", got)
	}
	if e, ok := c.Get(world, "player-A"); !ok || e.Version != 1 {
		t.Errorf("player-A %+v, want v1 once its fact is out", e)
	}
	if got := c.Health().Status; got != runtime.StatusOK {
		t.Errorf("health once the answer is out %q, want ok", got)
	}
}

// neverTimers is replay.NullTimers for this package: every timer stays armed
// and never fires.
type neverTimers struct{}

func (neverTimers) After(time.Duration) clock.Timer { return neverTimer{} }
func (neverTimers) Every(time.Duration) clock.Timer { return neverTimer{} }

type neverTimer struct{}

func (neverTimer) C() <-chan time.Time { return nil }
func (neverTimer) Stop() bool          { return true }

// Review #2 of T-055, Mi-7: with MV_BUS_VALIDATE_ON_READ=false a change set with
// expected_version -1 reaches State. It is refused as invalid_op, without
// details, so the refusal passes the schema of entity.update.rejected (versions
// from zero up) and goes out at once: a version_conflict naming -1 would be
// refused by the bus on every attempt and hold the world until Stop. The other
// change set of the package is applied.
func TestANegativeExpectedVersionIsRefusedInAFormTheBusPublishes(t *testing.T) {
	bus := newBus(t)
	c, _ := running(t, bus.Lenient(), world)
	publish(t, bus,
		create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}),
		create("prop-w", ref("wolf-alpha", entity.TypeNPC), "", map[string]any{"hp": 10}))
	untilEnd(t, bus, 4)

	proposal := update(t, "prop-below-zero", "author", false,
		set(ref("player-A", entity.TypePlayer), version(7), op(entity.OpInc, "hp", -2)),
		set(ref("wolf-alpha", entity.TypeNPC), nil, op(entity.OpInc, "hp", -3)))
	raw, err := json.Marshal(proposal)
	if err != nil {
		t.Fatal(err)
	}
	below := bytes.Replace(raw, []byte(`"expected_version":7`), []byte(`"expected_version":-1`), 1)
	if bytes.Equal(below, raw) {
		t.Fatal("no expected_version to put below zero")
	}
	if err := bus.Append(eventbus.TopicSystemEvents, below); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the answer to prop-below-zero", func() bool {
		return len(answersTo(t, bus, "prop-below-zero")) == 2
	})

	refusals := onTopic(t, bus, state.TypeRejected)
	if len(refusals) != 1 {
		t.Fatalf("%d refusals, want one", len(refusals))
	}
	assertRejected(t, refusals[0], "prop-below-zero", state.ReasonInvalidOp, "player-A")
	if refusals[0].Path().Has("details") {
		t.Errorf("the refusal carries details %v, want none", refusals[0].Payload["details"])
	}
	if err := contracts.Validate(refusals[0]); err != nil {
		t.Errorf("the refusal does not pass its schema: %v", err)
	}
	if e, _ := c.Get(world, "player-A"); e.Version != 1 {
		t.Errorf("player-A at v%d, want v1", e.Version)
	}
	if e, _ := c.Get(world, "wolf-alpha"); e.Version != 2 {
		t.Errorf("wolf-alpha at v%d, want v2: the valid change set of the package", e.Version)
	}
	if letters, _ := bus.DeadLetters(); len(letters) != 0 {
		t.Errorf("dead letters %v, want none", letters)
	}
}

// --- the mediator of delivery (C-01 v1.6) ---

// Mi-4 of review #1: the window of event ids of the mediator. An event refused
// once and delivered again — the same bytes — is not answered again: a refusal
// does not enter the window of proposal_id, so only the window of events stops
// the second entity.update.rejected.
func TestAnEventDeliveredTwiceIsAnsweredOnce(t *testing.T) {
	bus := newBus(t)
	running(t, bus, world)
	publish(t, bus, create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	untilEnd(t, bus, 2)

	stale := update(t, "prop-stale", "author", true,
		set(ref("player-A", entity.TypePlayer), version(7), op(entity.OpInc, "hp", -1)))
	publish(t, bus, stale)
	untilEnd(t, bus, 4)
	raw, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := bus.Append(eventbus.TopicSystemEvents, raw); err != nil {
		t.Fatal(err)
	}
	publish(t, bus, update(t, "prop-sentinel", "author", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	waitFor(t, "the answer to the sentinel", func() bool { return len(answersTo(t, bus, "prop-sentinel")) == 1 })

	if refusals := answersTo(t, bus, "prop-stale"); len(refusals) != 1 {
		t.Errorf("%d answers to the event delivered twice, want one", len(refusals))
	}
}

// Mi-1 of review #1: a proposal without a world in its envelope reaches no
// worker, and the log says so at Warn with the event, its type and its
// proposal_id. Since C-02 v1.7 the registry refuses such a proposal on Publish
// and, with MV_BUS_VALIDATE_ON_READ, on read; it reaches State only past a bus
// that does not validate on read, which is how it comes here.
func TestAProposalWithoutAWorldIsReported(t *testing.T) {
	bus := newBus(t)
	_, logged := running(t, bus.Lenient(), world)
	publish(t, bus, create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	worldless := update(t, "prop-worldless", "author", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
	worldless.World = nil
	if err := bus.Publish(context.Background(), worldless); !errors.Is(err, eventbus.ErrPolicyViolation) {
		t.Fatalf("Publish of a proposal without a world = %v, want the policy of the registry (WorldRequired)", err)
	}
	raw, err := json.Marshal(worldless)
	if err != nil {
		t.Fatal(err)
	}
	if err := bus.Append(eventbus.TopicSystemEvents, raw); err != nil {
		t.Fatal(err)
	}
	publish(t, bus, update(t, "prop-sentinel", "author", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	waitFor(t, "the answer to the sentinel", func() bool { return len(answersTo(t, bus, "prop-sentinel")) == 1 })

	if answers := answersTo(t, bus, "prop-worldless"); len(answers) != 0 {
		t.Errorf("the worldless proposal was answered: %v", answers)
	}
	warned := recordsOf(t, logged.String(), "proposal without world in the envelope: no worker is addressed")
	if len(warned) != 1 || warned[0]["level"] != "WARN" || warned[0]["event_id"] != worldless.ID ||
		warned[0]["type"] != state.TypeUpdateProposed || warned[0]["proposal_id"] != "prop-worldless" {
		t.Errorf("log %v, want one WARN with event_id, type and proposal_id", warned)
	}
}

// --- a panic stops the world (C-01 v1.5, NFR-012) ---

// A panic while a proposal is applied is caught at the boundary of the worker:
// Error with the stack, the world stopped, /health fail, no fact. What State
// gives back to the bus for that proposal and for every later one of the world
// is an error, so the bus retries and parks it in dead_letters with the reason;
// the other world goes on (dev-log T-055, "Паника").
func TestAPanicStopsItsWorldAndOnlyItsWorld(t *testing.T) {
	const second = "second-world"
	bus := rig(t)
	c, logged := running(t, bus, world, second)
	publish(t, bus.Bus,
		create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}),
		inWorld(create("prop-b", ref("player-B", entity.TypePlayer), "", map[string]any{"hp": 10}), second))
	untilEnd(t, bus.Bus, 4)

	bus.setHook(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == state.TypeUpdated && eventbus.GetWorldIDFromEvent(ev) == world {
			panic("the applier broke")
		}
		return nil
	})
	hitA := func(id string) eventbus.Event {
		return update(t, id, "author", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
	}
	publish(t, bus.Bus, hitA("prop-panics"), hitA("prop-after"),
		inWorld(update(t, "prop-b-hit", "author", true,
			set(ref("player-B", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))), second))
	waitFor(t, "two dead letters and the fact of the second world", func() bool {
		letters, _ := bus.DeadLetters()
		return len(letters) == 2 && len(onTopic(t, bus.Bus, state.TypeUpdated)) == 1
	})

	facts := onTopic(t, bus.Bus, state.TypeUpdated)
	if id, _ := facts[0].Path().GetString("entity.entity.id"); id != "player-B" {
		t.Errorf("the one fact is about %s, want player-B of the world that did not panic", id)
	}
	letters, _ := bus.DeadLetters()
	for i, want := range []string{"prop-panics", "prop-after"} {
		if id, _ := letters[i].Original.Path().GetString("proposal_id"); id != want ||
			!strings.Contains(letters[i].Error, state.ErrWorldStopped.Error()) || letters[i].Attempts != 4 {
			t.Errorf("dead letter %d: %s, %q after %d attempts; want %s refused by the stopped world after 4",
				i, id, letters[i].Error, letters[i].Attempts, want)
		}
	}
	if e, _ := c.Get(world, "player-A"); e.Version != 1 {
		t.Errorf("player-A at v%d, want v1: the stopped world was changed", e.Version)
	}

	records := recordsOf(t, logged.String(), "state worker panicked; the world is stopped")
	if len(records) != 1 || records[0]["level"] != "ERROR" || records[0]["handled"] != false ||
		!strings.Contains(fmt.Sprint(records[0]["stack"]), "goroutine") || records[0]["panic"] != "the applier broke" {
		t.Errorf("log of the panic %v, want one ERROR with the panic, its stack and handled=false", records)
	}
	health := c.Health()
	worlds, _ := health.Details["worlds"].(map[string]any)
	stopped, _ := worlds[world].(map[string]any)
	alive, _ := worlds[second].(map[string]any)
	if health.Status != runtime.StatusFail || stopped["status"] != runtime.StatusFail || stopped["reason"] != "panic" ||
		alive["status"] != runtime.StatusOK {
		t.Errorf("health %+v, want fail with %s failed and %s ok", health, world, second)
	}
}

// --- stopping (C-01 v1.7, ADR-023 p. 4) ---

// Stop during the publication of a fact: the subscription is cancelled first,
// the proposal in hand is carried through to its fact, Stop returns within
// runtime.StopTimeout, and the subscription has returned before the bus closes.
// Started again over the same world, State recognises the proposal when it
// comes again — the same event and a new one under the same proposal_id — and
// publishes no second fact.
func TestStopFinishesTheProposalInHand(t *testing.T) {
	bus := rig(t)
	c, _ := running(t, bus, world)
	publish(t, bus.Bus, create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	untilEnd(t, bus.Bus, 2)

	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	bus.setHook(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == state.TypeUpdated {
			once.Do(func() { close(entered) })
			<-release
		}
		return nil
	})
	slow := update(t, "prop-slow", "author", true, set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -1)))
	publish(t, bus.Bus, slow)
	<-entered

	stopped := make(chan error, 1)
	began := testkit.Wall().Now()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		stopped <- c.Stop(ctx)
	}()
	waitFor(t, "the subscription cancelled by Stop", bus.subscriptionCancelled)
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned %v while the proposal was still being published", err)
	default:
	}
	close(release)
	if err := <-stopped; err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if took := testkit.Wall().Now().Sub(began); took >= runtime.StopTimeout {
		t.Errorf("Stop took %v, the budget is %v", took, runtime.StopTimeout)
	}
	bus.setHook(nil)
	if facts := onTopic(t, bus.Bus, state.TypeUpdated); len(facts) != 1 {
		t.Fatalf("%d facts after Stop, want the one of the proposal in hand", len(facts))
	}

	if err := c.Start(context.Background(), runtime.Deps{Bus: bus}); err != nil {
		t.Fatalf("Start again: %v", err)
	}
	raw, err := json.Marshal(slow)
	if err != nil {
		t.Fatal(err)
	}
	if err := bus.Append(eventbus.TopicSystemEvents, raw); err != nil {
		t.Fatal(err)
	}
	sentinel := update(t, "prop-after-restart", "author", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1)))
	publish(t, bus.Bus, update(t, "prop-slow", "author", true,
		set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -1))), sentinel)
	waitFor(t, "the fact of the proposal after the restart", func() bool {
		return len(onTopic(t, bus.Bus, state.TypeUpdated)) == 2
	})
	facts := onTopic(t, bus.Bus, state.TypeUpdated)
	if id, _ := facts[1].Path().GetString("proposal_id"); id != "prop-after-restart" {
		t.Errorf("the second fact answers %s, want prop-after-restart: prop-slow was applied twice", id)
	}
	if refusals := onTopic(t, bus.Bus, state.TypeRejected); len(refusals) != 0 {
		t.Errorf("%d refusals: a repeat was answered instead of recognised", len(refusals))
	}

	ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	if err := c.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	_ = bus.Close()
	timeline := bus.events()
	if len(timeline) != 3 || !strings.HasPrefix(timeline[1], "subscription returned (context: context canceled)") ||
		timeline[2] != "bus closed" {
		t.Errorf("timeline %q, want each subscription returned on its cancelled context before the bus closed", timeline)
	}
}

// Stop does not outlive its context: a worker that never finishes its
// publication leaves Stop with the error of the deadline rather than a process
// that never exits. The context is not started again while that worker still
// holds its proposal — a second worker over the same world would be a second
// writer — and is once it has let go (review #1 of T-055, Mi-2).
func TestStopIsBoundedByItsContext(t *testing.T) {
	bus := rig(t)
	c, _ := running(t, bus, world)
	publish(t, bus.Bus, create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	untilEnd(t, bus.Bus, 2)

	entered, release := make(chan struct{}), make(chan struct{})
	var once, released sync.Once
	letGo := func() { released.Do(func() { close(release) }) }
	bus.setHook(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == state.TypeUpdated {
			once.Do(func() { close(entered) })
			<-release
		}
		return nil
	})
	t.Cleanup(letGo)
	publish(t, bus.Bus, update(t, "prop-stuck", "author", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- c.Stop(ctx) }()
	select {
	case err := <-stopped:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Stop = %v, want the deadline of its context", err)
		}
	case <-testkit.After(30 * time.Second):
		t.Fatal("Stop did not return within 30s of a 50ms budget")
	}

	deps := runtime.Deps{Bus: bus}
	if err := c.Start(context.Background(), deps); err == nil {
		t.Fatal("Start succeeded while the worker of the run before still held its proposal: two writers of one world")
	}
	letGo()
	waitFor(t, "Start once the run before has ended", func() bool {
		return c.Start(context.Background(), deps) == nil
	})
	publish(t, bus.Bus, update(t, "prop-after", "author", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	waitFor(t, "the answer to prop-after", func() bool { return len(answersTo(t, bus.Bus, "prop-after")) == 1 })
	assertOneFactPerVersion(t, bus.Bus)
	if e, _ := c.Get(world, "player-A"); e.Version != 3 {
		t.Errorf("player-A at v%d, want v3: one fact each for prop-stuck and prop-after", e.Version)
	}
}

// --- starting and health ---

func TestStartRefusesWhatItCannotRunOn(t *testing.T) {
	bus := newBus(t)
	if err := state.New(state.Config{Worlds: []string{world}}).Start(context.Background(), runtime.Deps{}); err == nil {
		t.Error("Start without a bus succeeded")
	}
	t.Setenv(env.StateWorlds.Name(), " , ")
	err := state.New(state.Config{}).Start(context.Background(), runtime.Deps{Bus: bus})
	if err == nil || !strings.Contains(err.Error(), env.StateWorlds.Name()) {
		t.Errorf("Start with %s blank = %v, want a refusal naming the variable", env.StateWorlds.Name(), err)
	}

	c, _ := running(t, bus, world)
	if err := c.Start(context.Background(), runtime.Deps{Bus: bus}); err == nil {
		t.Error("a second Start of a running context succeeded")
	}
}

// The worlds come from MV_STATE_WORLDS when the configuration names none:
// blanks and repeats dropped, one worker per world.
func TestTheWorldsComeFromTheManifest(t *testing.T) {
	t.Setenv(env.StateWorlds.Name(), "world-a, world-b ,world-a,")
	c := state.New(state.Config{})
	if c.Name() != "state" || c.DependsOn() != nil {
		t.Errorf("name %q depends on %v, want state and nothing", c.Name(), c.DependsOn())
	}
	if err := c.Start(context.Background(), runtime.Deps{Bus: newBus(t)}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = c.Stop(context.Background()) }()
	worlds, _ := c.Health().Details["worlds"].(map[string]any)
	if len(worlds) != 2 || worlds["world-a"] == nil || worlds["world-b"] == nil {
		t.Errorf("health names the worlds %v, want world-a and world-b", worlds)
	}
}

// /health: degraded before Start and after Stop, ok while running with the
// entities of each world, fail once the subscription ends on its own — and Stop
// reports what ended it.
func TestHealthFollowsTheLifeOfTheContext(t *testing.T) {
	c := state.New(state.Config{Worlds: []string{world}})
	if got := c.Health().Status; got != runtime.StatusDegraded {
		t.Errorf("health before Start %q, want degraded", got)
	}
	bus := newBus(t)
	testkit.Deterministic(t, "t055")
	if err := c.Start(context.Background(), runtime.Deps{Bus: bus}); err != nil {
		t.Fatal(err)
	}
	publish(t, bus, create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	waitFor(t, "the world to hold the entity", func() bool {
		worlds, _ := c.Health().Details["worlds"].(map[string]any)
		section, _ := worlds[world].(map[string]any)
		return section["entities"] == 1
	})
	if got := c.Health().Status; got != runtime.StatusOK {
		t.Errorf("health while running %q, want ok", got)
	}
	if err := c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := c.Health().Status; got != runtime.StatusDegraded {
		t.Errorf("health after Stop %q, want degraded", got)
	}
	if err := c.Stop(context.Background()); err != nil {
		t.Errorf("a second Stop = %v, want nil", err)
	}

	refused := rig(t)
	refused.subErr = errors.New("the broker refused the consumer group")
	dead := state.New(state.Config{Worlds: []string{world}})
	if err := dead.Start(context.Background(), runtime.Deps{Bus: refused}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "health to fail on the lost subscription", func() bool {
		return dead.Health().Status == runtime.StatusFail
	})
	if msg, _ := dead.Health().Details["err"].(string); !strings.Contains(msg, "refused the consumer group") {
		t.Errorf("health details %v, want the error of the subscription", dead.Health().Details)
	}
	if err := dead.Stop(context.Background()); err == nil || !strings.Contains(err.Error(), "refused the consumer group") {
		t.Errorf("Stop = %v, want the error that ended the subscription", err)
	}
}

// --- what is on the bus ---

// worldHealth is the section of one world in /health.
func worldHealth(c *state.Context, worldID string) map[string]any {
	worlds, _ := c.Health().Details["worlds"].(map[string]any)
	section, _ := worlds[worldID].(map[string]any)
	return section
}

// answersTo is every fact and refusal on system_events that answers a proposal.
func answersTo(t *testing.T, bus *membus.Bus, proposalID string) []eventbus.Event {
	t.Helper()
	var out []eventbus.Event
	for _, typ := range []string{state.TypeCreated, state.TypeUpdated, state.TypeRejected} {
		for _, ev := range onTopic(t, bus, typ) {
			if id, _ := ev.Path().GetString("proposal_id"); id == proposalID {
				out = append(out, ev)
			}
		}
	}
	return out
}

// assertOneFactPerVersion is the invariant Ma-1 broke: however often a fact is
// on the bus, one version of one entity is announced by one event.
func assertOneFactPerVersion(t *testing.T, bus *membus.Bus) {
	t.Helper()
	announced := map[string]string{}
	for _, typ := range []string{state.TypeCreated, state.TypeUpdated} {
		for _, fact := range onTopic(t, bus, typ) {
			id, _ := fact.Path().GetString("entity.entity.id")
			v, _ := fact.Path().GetInt("version")
			key := fmt.Sprintf("%s v%d", id, v)
			if first, seen := announced[key]; seen && first != fact.ID {
				t.Errorf("%s is announced by two facts, %s and %s", key, first, fact.ID)
			}
			announced[key] = fact.ID
		}
	}
}

// assertAnnounced checks that each entity of the world is at the version and hp
// wanted, and that a fact on the bus announces that version: the world is never
// ahead of its facts.
func assertAnnounced(t *testing.T, c *state.Context, bus *membus.Bus, want map[string][2]int) {
	t.Helper()
	for id, vh := range want {
		e, ok := c.Get(world, id)
		if !ok || e.Version != int64(vh[0]) || intAt(e, "hp") != vh[1] {
			t.Errorf("%s in the world %+v, want v%d hp %d", id, e, vh[0], vh[1])
			continue
		}
		found := false
		for _, typ := range []string{state.TypeCreated, state.TypeUpdated} {
			for _, fact := range onTopic(t, bus, typ) {
				factID, _ := fact.Path().GetString("entity.entity.id")
				v, _ := fact.Path().GetInt("version")
				found = found || (factID == id && int64(v) == e.Version)
			}
		}
		if !found {
			t.Errorf("the world holds %s v%d, and no fact on the bus announces it", id, e.Version)
		}
	}
}

// lockedBuffer is a log that the goroutines of the context write and the test
// reads.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// The laws of Config.Invariants reach the Applier of every world the context
// serves: a create that stands nowhere is refused law_violation inv-10.
func TestTheContextHoldsItsWorldsToTheLaws(t *testing.T) {
	bus := newBus(t)
	testkit.Deterministic(t, "t056")
	eventbus.SetRegistry(contracts.Default())
	c := state.New(state.Config{Worlds: []string{world}, Invariants: laws(t)})
	if err := c.Start(context.Background(), runtime.Deps{Bus: bus}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = c.Stop(context.Background()) })

	publish(t, bus, create("prop-nowhere", ref("wolf-beta", entity.TypeNPC), "", map[string]any{
		"hp": 5, "hp_max": 5, "status": "alive", "position": "nowhere",
	}))
	waitFor(t, "the answer to prop-nowhere", func() bool { return len(answersTo(t, bus, "prop-nowhere")) == 1 })

	answer := answersTo(t, bus, "prop-nowhere")[0]
	assertRejected(t, answer, "prop-nowhere", state.ReasonLawViolation, "wolf-beta")
	if law, _ := answer.Path().GetString("details.invariant_id"); law != "inv-10" {
		t.Errorf("details.invariant_id %q, want inv-10", law)
	}
}
