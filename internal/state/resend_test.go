package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
)

// --- a write under Stop and the facts sent after a restart ---

// Stop while an entity is being written (C-01 v1.7, ADR-023 p. 4): the write
// is carried to its end, the fact goes out after it and not before, and Stop
// returns within runtime.StopTimeout. Started again, State recognises the
// proposal when it comes again — the same event and a new one under the same
// proposal_id — and neither writes the entity nor publishes a fact a second
// time.
func TestStopDuringAWriteFinishesTheWriteAndItsFact(t *testing.T) {
	bus := rig(t)
	objects := newTracedObjects(t, world)
	c, _ := runningStored(t, bus, objects, -1)
	publish(t, bus.Bus, createWorld(), create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	untilEnd(t, bus.Bus, 4)

	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	objects.setHook(func(_, key string) error {
		if key == "player/player-A.json" {
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
		t.Fatalf("Stop returned %v while the entity was still being written", err)
	default:
	}
	if facts := onTopic(t, bus.Bus, state.TypeUpdated); len(facts) != 0 {
		t.Fatalf("%d facts out while their entity was still being written", len(facts))
	}
	close(release)
	if err := <-stopped; err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if took := testkit.Wall().Now().Sub(began); took >= runtime.StopTimeout {
		t.Errorf("Stop took %v, the budget is %v", took, runtime.StopTimeout)
	}
	objects.setHook(nil)
	if facts := onTopic(t, bus.Bus, state.TypeUpdated); len(facts) != 1 {
		t.Fatalf("%d facts after Stop, want the one of the proposal in hand", len(facts))
	}
	if object := objects.entityObject(t, world, entity.TypePlayer, "player-A"); object.Version != 2 {
		t.Errorf("the object of player-A at v%d, want v2", object.Version)
	}
	steps := objects.timeline.all()
	if len(steps) < 3 || steps[len(steps)-3] != "put player/player-A.json" || steps[len(steps)-1] != "put state/latest.json" {
		t.Errorf("writes %q, want the entity written before the shutdown snapshot", steps)
	}
	writes := len(objects.timeline.matching("put player/player-A.json"))

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
		return len(answersTo(t, bus.Bus, "prop-after-restart")) == 1
	})
	if facts := answersTo(t, bus.Bus, "prop-slow"); len(facts) != 1 {
		t.Errorf("%d answers to prop-slow, want the one fact from before the restart", len(facts))
	}
	if got := len(objects.timeline.matching("put player/player-A.json")); got != writes+1 {
		t.Errorf("player-A written %d times after the restart, want once, for prop-after-restart", got-writes)
	}
}

// A fact whose entity is written but which never got out — the publication
// failed until Stop — is published after a restart over what the object store
// kept, once, under the id it was first derived with; the version does not
// move and the entity is not written again (C-02 "Гарантии"; DoD T-057 from the
// acceptance of T-056). For an atomic package the facts that did get out come
// out again under their own ids, and no version is announced by two facts.
func TestAFactLostAfterItsWriteIsSentAfterARestart(t *testing.T) {
	cases := map[string]struct {
		sets    func(t *testing.T) eventbus.Event
		failing string
	}{
		"one entity": {
			sets: func(t *testing.T) eventbus.Event {
				return update(t, "prop-lost", "author", true,
					set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)))
			},
			failing: "player-A",
		},
		"an atomic package": {
			sets: func(t *testing.T) eventbus.Event {
				return update(t, "prop-lost", "author", true,
					set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)),
					set(ref("wolf-alpha", entity.TypeNPC), version(1), op(entity.OpInc, "hp", -3)))
			},
			failing: "wolf-alpha",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			bus := rig(t)
			objects := newTracedObjects(t, world)
			c, manual := runningStored(t, bus, objects, -1)
			publish(t, bus.Bus, createWorld(),
				create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}),
				create("prop-w", ref("wolf-alpha", entity.TypeNPC), "", map[string]any{"hp": 10}))
			untilEnd(t, bus.Bus, 6)

			var attempts atomic.Int32
			var tried atomic.Value
			bus.setHook(func(_ context.Context, ev eventbus.Event) error {
				if ev.Type == state.TypeUpdated && subjectOf(ev) == tc.failing {
					attempts.Add(1)
					tried.Store(ev.ID)
					return errors.New("the broker is away")
				}
				return nil
			})
			publish(t, bus.Bus, tc.sets(t))
			waitFor(t, "a second attempt", advancing(manual, func() bool { return attempts.Load() >= 2 }))
			if object := objects.entityObject(t, world, entity.TypePlayer, "player-A"); object.Version != 2 ||
				object.LastChange.ProposalID != "prop-lost" || object.LastChange.FactEventID != "" {
				t.Fatalf("the object of player-A %+v, want v2 under prop-lost without a fact id", object)
			}
			ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
			defer cancel()
			if err := c.Stop(ctx); err != nil {
				t.Fatalf("Stop: %v", err)
			}
			before := answersTo(t, bus.Bus, "prop-lost")
			writes := len(objects.timeline.all())

			bus.setHook(nil)
			restarted := state.NewOver(state.Config{Worlds: []string{world}, Objects: objects, SnapshotEvery: -1},
				loadedFrom(t, objects.Memory, world))
			if err := restarted.Start(context.Background(), runtime.Deps{Bus: bus}); err != nil {
				t.Fatalf("Start of the restarted process: %v", err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
				defer cancel()
				_ = restarted.Stop(ctx)
			})
			waitFor(t, "the lost fact after the restart", func() bool {
				for _, ev := range answersTo(t, bus.Bus, "prop-lost") {
					if subjectOf(ev) == tc.failing {
						return true
					}
				}
				return false
			})

			byEntity := idsByEntity(answersTo(t, bus.Bus, "prop-lost"))
			if ids := byEntity[tc.failing]; len(ids) != 1 || ids[0] != tried.Load() {
				t.Errorf("facts of %s %v, want one under the id it was first tried with, %v", tc.failing, ids, tried.Load())
			}
			for _, ev := range before {
				if again := byEntity[subjectOf(ev)]; len(again) != 2 || again[1] != ev.ID {
					t.Errorf("the fact of %s out before the restart as %s came out again as %v", subjectOf(ev), ev.ID, again)
				}
			}
			if steps := objects.timeline.all()[writes:]; len(steps) != 0 {
				t.Errorf("writes after the restart %q, want none", steps)
			}
			for id := range byEntity {
				e, _ := restarted.Get(world, id)
				if e.Version != 2 || e.LastChange.FactEventID == "" {
					t.Errorf("%s at v%d with fact %q, want v2 and the id of its fact", id, e.Version, e.LastChange.FactEventID)
				}
			}
			assertOneFactPerVersion(t, bus.Bus)
			if letters, _ := bus.DeadLetters(); len(letters) != 0 {
				t.Errorf("dead letters %v, want none", letters)
			}
		})
	}
}

// A proposer that publishes the proposal again under the same proposal_id sends
// a new event: another id, another instant, maybe another cause. The fact sent
// after it is still the fact first derived — its id, its applied_at and its
// cause come from the commit record, not from the new event (ADR-027; review #1
// of T-057, Mi-3).
//
// The rest of the envelope is not in the commit record: the correlation of the
// fact is that of the new event. The test pins that as it is; whether the
// record should keep the envelope is a question to T-059.
func TestAFactSentAgainKeepsItsFirstIDUnderANewEvent(t *testing.T) {
	first, objects := newStoredFixture(t, state.ApplierConfig{})
	first.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	lost := update(t, "prop-lost", "combat", true, set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)))
	var tried string
	first.journal.fail = func(_ int, ev eventbus.Event) error {
		tried = ev.ID
		return errors.New("the broker is away")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := first.applier.Apply(cancelled, lost); !errors.Is(err, state.ErrPublishFailed) {
		t.Fatalf("Apply = %v, want publish_failed", err)
	}

	// The repeat is built on the sources of the first run, before the second
	// fixture installs its own: the next id of the sequence and a later instant.
	first.clock.Advance(time.Hour)
	again := update(t, "prop-lost", "loot", true, set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)))
	if again.ID == lost.ID || again.Timestamp.Equal(lost.Timestamp) || again.CorrelationID() == lost.CorrelationID() {
		t.Fatalf("the repeat %s at %s is not a new event after %s at %s", again.ID, again.Timestamp, lost.ID, lost.Timestamp)
	}
	// The intent of another package does not hold up the facts of this one.
	if err := state.NewObjectStore(objects).PutIntent(context.Background(), world,
		&state.Intent{ProposalID: "prop-elsewhere", World: world}); err != nil {
		t.Fatal(err)
	}
	second := newStoredFixtureOver(t, objects, state.ApplierConfig{Store: loadedFrom(t, objects.Memory, world)})
	second.apply(t, again)
	facts := second.journal.ofType(state.TypeUpdated)
	if len(facts) != 1 || facts[0].ID != tried {
		t.Fatalf("entity.updated %d, want one under the id first tried, %s", len(facts), tried)
	}
	fact := facts[0]
	if at, _ := fact.Path().GetString("applied_at"); !sameInstant(at, lost.Timestamp) || !fact.Timestamp.Equal(lost.Timestamp) {
		t.Errorf("applied_at %s and timestamp %s, want the instant of the first proposal %s", at, fact.Timestamp, lost.Timestamp)
	}
	if cause, _ := fact.Path().GetString("cause"); cause != "combat" || fact.Meta.CausationID != lost.ID {
		t.Errorf("cause %q caused by %s, want combat from the commit record and the first proposal %s", cause, fact.Meta.CausationID, lost.ID)
	}
	if fact.CorrelationID() != again.CorrelationID() {
		t.Errorf("correlation %s, want that of the event at hand %s: the commit record keeps no envelope", fact.CorrelationID(), again.CorrelationID())
	}
	if held := second.get(t, "player-A"); held.Version != 2 {
		t.Errorf("player-A at v%d, want v2", held.Version)
	}
}

// An atomic package cut off between the writes of its entities leaves its
// intent behind with some entities written. A repeat of the proposal before the
// intent is rolled forward publishes nothing — half of an atomic package is not
// announced — writes nothing, and stops the world with persist_failed (review
// #1 of T-057, question 2; decision of the orchestrator).
func TestARepeatOfAnUnfinishedPackageSendsNoFact(t *testing.T) {
	first, objects := newStoredFixture(t, state.ApplierConfig{})
	first.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	first.seed(t, "wolf-alpha", entity.TypeNPC, 1, map[string]any{"hp": 10})
	round := update(t, "prop-round", "combat", true,
		set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)),
		set(ref("wolf-alpha", entity.TypeNPC), version(1), op(entity.OpInc, "hp", -3)))
	objects.setHook(func(_, key string) error {
		if key == "npc/wolf-alpha.json" {
			return errors.New("the process died")
		}
		return nil
	})
	if err := first.applyAdvancing(t, context.Background(), round); !errors.Is(err, state.ErrPersistFailed) {
		t.Fatalf("Apply = %v, want persist_failed between the writes", err)
	}
	objects.setHook(nil)
	store := state.NewObjectStore(objects)
	if intents, _ := store.ListIntents(context.Background(), world); len(intents) != 1 {
		t.Fatalf("%d intents, want the one of the cut-off package", len(intents))
	}
	if object := objects.entityObject(t, world, entity.TypePlayer, "player-A"); object.Version != 2 ||
		object.LastChange.ProposalID != "prop-round" {
		t.Fatalf("player-A %+v, want it written under prop-round", object)
	}
	writes := len(objects.timeline.all())

	second := newStoredFixtureOver(t, objects, state.ApplierConfig{Store: loadedFrom(t, objects.Memory, world)})
	err := second.applier.Apply(context.Background(), round)
	if !errors.Is(err, state.ErrPersistFailed) || !errors.Is(err, state.ErrWorldStopped) {
		t.Errorf("Apply of the repeat = %v, want persist_failed of a stopped world", err)
	}
	if events := second.journal.all(); len(events) != 0 {
		t.Errorf("published %v, want nothing of an unfinished package", types(events))
	}
	if steps := objects.timeline.all()[writes:]; len(steps) != 0 {
		t.Errorf("after the repeat %q, want no write", steps)
	}
	records := second.logged(t, "a repeated proposal has an unfinished package; no fact of it is sent and the world is stopped")
	if len(records) != 1 || records[0]["proposal_id"] != "prop-round" || records[0]["handled"] != false {
		t.Errorf("log %v, want one record of prop-round with handled=false", records)
	}
}

// The same holds for a create: the entity written, its entity.created never
// out, the create delivered again to a State over the object store — the fact
// is published under its first id and nothing is written.
func TestACreatedFactLostAfterItsWriteIsSent(t *testing.T) {
	first, objects := newStoredFixture(t, state.ApplierConfig{})
	born := create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10})
	var tried string
	first.journal.fail = func(_ int, ev eventbus.Event) error {
		tried = ev.ID
		return errors.New("the broker is away")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := first.applier.Apply(cancelled, born); !errors.Is(err, state.ErrPublishFailed) {
		t.Fatalf("Apply = %v, want publish_failed", err)
	}
	writes := len(objects.timeline.all())

	second := newStoredFixtureOver(t, objects, state.ApplierConfig{Store: loadedFrom(t, objects.Memory, world)})
	second.apply(t, born)
	facts := second.journal.ofType(state.TypeCreated)
	if len(facts) != 1 || facts[0].ID != tried {
		t.Fatalf("entity.created %v, want one under the id first tried, %s", types(facts), tried)
	}
	if steps := objects.timeline.all()[writes:]; len(steps) != 1 || steps[0] != "publish entity.created player-A" {
		t.Errorf("after the restart %q, want the publication alone", steps)
	}
	if held := second.get(t, "player-A"); held.Version != 1 || held.LastChange.FactEventID != tried {
		t.Errorf("player-A v%d with fact %q, want v1 and %s", held.Version, held.LastChange.FactEventID, tried)
	}
	second.apply(t, born)
	if n := len(second.journal.ofType(state.TypeCreated)); n != 1 {
		t.Errorf("%d entity.created after a second repeat, want still 1", n)
	}
}
