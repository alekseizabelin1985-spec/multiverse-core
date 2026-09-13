package state_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
)

// --- the stopped world (C-01 v1.10, КД State §9) ---

// TestAStoppedWorldAnswersAnOrdinaryError: the refusal of a stopped world is a
// state of the receiver, not a defect of the event, so it is never
// eventbus.Permanent — neither for the proposal abandoned on publish_failed nor
// for a later one.
func TestAStoppedWorldAnswersAnOrdinaryError(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	f.journal.fail = func(int, eventbus.Event) error { return errors.New("the broker is away") }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := f.applier.Apply(ctx, update(t, "prop-lost", "author", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	if !errors.Is(err, state.ErrPublishFailed) || errors.Is(err, eventbus.ErrPermanent) {
		t.Fatalf("Apply = %v, want publish_failed and not permanent", err)
	}
	f.journal.fail = nil
	later := f.applier.Apply(context.Background(), update(t, "prop-later", "author", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	if !errors.Is(later, state.ErrWorldStopped) || errors.Is(later, eventbus.ErrPermanent) {
		t.Errorf("Apply of the stopped world = %v, want ErrWorldStopped and not permanent", later)
	}
}

// TestAStopDuringPublishFailedDeliversTheFactsAfterARestart: Stop ends the
// attempts to publish an answer that does not go out; the proposal is left
// uncommitted, not parked. A process restarted over the world the store kept —
// here the working set of the context, until recovery exists (T-059) — is
// handed the proposal again, publishes its facts under the same ids, and
// dead_letters stays empty (decision of the orchestrator on T-056; C-01 v1.10).
func TestAStopDuringPublishFailedDeliversTheFactsAfterARestart(t *testing.T) {
	bus := rig(t)
	c, _, manual := runningOnClock(t, bus, world)
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
	half := answersTo(t, bus.Bus, "prop-round")
	if len(half) != 1 {
		t.Fatalf("%d answers out before the restart, want the fact of player-A alone", len(half))
	}

	bus.setHook(nil)
	restarted := state.NewOver(state.Config{Worlds: []string{world}}, c.Store())
	if err := restarted.Start(context.Background(), runtime.Deps{Bus: bus}); err != nil {
		t.Fatalf("Start of the restarted process: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		_ = restarted.Stop(ctx)
	})
	waitFor(t, "both facts of prop-round after the restart", func() bool {
		return len(idsByEntity(answersTo(t, bus.Bus, "prop-round"))) == 2
	})

	if letters, _ := bus.DeadLetters(); len(letters) != 0 {
		t.Errorf("dead letters %v, want none", letters)
	}
	assertOneFactPerVersion(t, bus.Bus)
	for id, facts := range idsByEntity(answersTo(t, bus.Bus, "prop-round")) {
		if e, _ := restarted.Get(world, id); e.Version != 2 {
			t.Errorf("%s at v%d, want v2", id, e.Version)
		}
		if id == "player-A" && len(facts) == 2 && facts[0] != facts[1] {
			t.Errorf("the fact of player-A came out again under %s, the first was %s", facts[1], facts[0])
		}
	}
	if health := restarted.Health(); health.Status != runtime.StatusOK {
		t.Errorf("health of the restarted process %q %v, want ok", health.Status, health.Details)
	}
}
