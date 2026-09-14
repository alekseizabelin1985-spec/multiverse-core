package state_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
)

// --- write-through: the object before the fact (C-02 "Гарантии", ADR-011) ---

// One entity, one PUT, and its fact after it; the object carries the commit
// record with an empty fact_event_id, the working set learns the id of the fact
// once it is out. No intent for a single entity (ADR-013 p. 4).
func TestAFactFollowsTheWriteOfItsEntity(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.apply(t, create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	f.apply(t, update(t, "prop-hit", "combat", true, set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -3))))

	want := []string{
		"put player/player-A.json", "publish entity.created player-A",
		"put player/player-A.json", "publish entity.updated player-A",
	}
	if got := objects.timeline.all(); !slices.Equal(got, want) {
		t.Errorf("writes and publications %q, want %q", got, want)
	}
	object := objects.entityObject(t, world, entity.TypePlayer, "player-A")
	if object.Version != 2 || intAt(object, "hp") != 7 || object.LastChange == nil ||
		object.LastChange.ProposalID != "prop-hit" || object.LastChange.FactEventID != "" {
		t.Errorf("object %+v, want v2 hp 7 under prop-hit without a fact id", object)
	}
	fact := f.journal.ofType(state.TypeUpdated)[0]
	if held := f.get(t, "player-A"); held.LastChange.FactEventID != fact.ID || held.LastEventID != fact.ID {
		t.Errorf("the working set holds fact %q, the fact out is %s", held.LastChange.FactEventID, fact.ID)
	}
}

// An atomic package of several entities writes its intent first, the entities
// in ascending id order, removes the intent, and only then publishes; the
// intent names every entity with the version it leaves and reaches. A package
// that is not atomic, and an atomic one of one entity, write no intent.
func TestAnIntentGuardsOnlyAnAtomicPackageOfSeveralEntities(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.seed(t, "wolf-alpha", entity.TypeNPC, 4, map[string]any{"hp": 10})
	f.seed(t, "player-A", entity.TypePlayer, 2, map[string]any{"hp": 10})

	f.apply(t, update(t, "prop-round", "combat", true,
		set(ref("wolf-alpha", entity.TypeNPC), version(4), op(entity.OpInc, "hp", -3)),
		set(ref("player-A", entity.TypePlayer), version(2), op(entity.OpInc, "hp", -2))))

	want := []string{
		"put _intents/prop-round.json", "put player/player-A.json", "put npc/wolf-alpha.json",
		"delete _intents/prop-round.json",
		"publish entity.updated player-A", "publish entity.updated wolf-alpha",
	}
	if got := objects.timeline.all(); !slices.Equal(got, want) {
		t.Fatalf("writes and publications %q, want %q", got, want)
	}
	if intents, _ := state.NewObjectStore(objects).ListIntents(context.Background(), world); len(intents) != 0 {
		t.Errorf("%d intents left after the package, want none", len(intents))
	}

	// The intent as it was written, read while the last entity is written.
	var intent state.Intent
	f2, objects2 := newStoredFixture(t, state.ApplierConfig{})
	f2.seed(t, "wolf-alpha", entity.TypeNPC, 4, map[string]any{"hp": 10})
	f2.seed(t, "player-A", entity.TypePlayer, 2, map[string]any{"hp": 10})
	objects2.setHook(func(_, key string) error {
		if key == "npc/wolf-alpha.json" {
			readJSON(t, objects2.Memory, objstore.EntitiesBucket(world), "_intents/prop-round.json", &intent)
		}
		return nil
	})
	f2.apply(t, update(t, "prop-round", "combat", true,
		set(ref("wolf-alpha", entity.TypeNPC), version(4), op(entity.OpInc, "hp", -3)),
		set(ref("player-A", entity.TypePlayer), version(2), op(entity.OpInc, "hp", -2))))
	if intent.ProposalID != "prop-round" || intent.World != world || intent.Cause != "combat" || len(intent.Changes) != 2 ||
		intent.Changes[0].Ref.ID != "player-A" || intent.Changes[0].FromVersion != 2 || intent.Changes[0].ToVersion != 3 ||
		intent.Changes[1].Ref.ID != "wolf-alpha" || intent.Changes[1].FromVersion != 4 || intent.Changes[1].ToVersion != 5 ||
		intAt(&entity.Entity{Attributes: intent.Changes[1].AttributesAfter}, "hp") != 7 || len(intent.Changes[1].Changed) != 1 {
		t.Errorf("intent %+v, want both entities with from and to versions and what they reach", intent)
	}

	f3, objects3 := newStoredFixture(t, state.ApplierConfig{})
	f3.seed(t, "wolf-alpha", entity.TypeNPC, 1, map[string]any{"hp": 10})
	f3.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	f3.apply(t, update(t, "prop-loose", "combat", false,
		set(ref("wolf-alpha", entity.TypeNPC), nil, op(entity.OpInc, "hp", -1)),
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	f3.apply(t, update(t, "prop-one", "combat", true, set(ref("wolf-alpha", entity.TypeNPC), nil, op(entity.OpInc, "hp", -1))))
	if intents := objects3.timeline.matching("put _intents/"); len(intents) != 0 {
		t.Errorf("intents written %q, want none for a loose package or a package of one", intents)
	}
}

// A write that fails is tried again, three more times at most; one that
// succeeds within them publishes the fact as usual.
func TestAWriteThatFailsIsTriedAgain(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	failures := 0
	objects.setHook(func(_, key string) error {
		if key == "player/player-A.json" && failures < 3 {
			failures++
			return errors.New("the disk is full")
		}
		return nil
	})
	err := f.applyAdvancing(t, context.Background(),
		update(t, "prop-hit", "combat", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	if err != nil {
		t.Fatalf("Apply = %v, want the fact after the fourth attempt", err)
	}
	if refused, facts := objects.timeline.matching("refused "), f.journal.ofType(state.TypeUpdated); len(refused) != 3 || len(facts) != 1 {
		t.Errorf("%d refused writes and %d facts, want 3 and 1", len(refused), len(facts))
	}
}

// A write refused after every attempt stops the world with persist_failed: no
// fact, the working set unchanged, every later proposal refused by the stopped
// world, and /health names the reason.
func TestAWriteRefusedForGoodStopsTheWorld(t *testing.T) {
	f, objects := newStoredFixture(t, state.ApplierConfig{})
	f.seedWorld(t)
	f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	objects.setHook(func(_, key string) error {
		if key == "player/player-A.json" {
			return errors.New("the disk is full")
		}
		return nil
	})
	err := f.applyAdvancing(t, context.Background(),
		update(t, "prop-hit", "combat", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	if !errors.Is(err, state.ErrPersistFailed) || !errors.Is(err, state.ErrWorldStopped) || errors.Is(err, eventbus.ErrPermanent) {
		t.Fatalf("Apply = %v, want persist_failed of a stopped world, not permanent", err)
	}
	if refused := objects.timeline.matching("refused "); len(refused) != 4 {
		t.Errorf("%d attempts, want 4", len(refused))
	}
	if events := f.journal.all(); len(events) != 0 {
		t.Errorf("published %v, want nothing: the entity was not written", types(events))
	}
	if held := f.get(t, "player-A"); held.Version != 1 {
		t.Errorf("player-A at v%d in the working set, want v1", held.Version)
	}
	records := f.logged(t, "the object store refused an answer; the world is stopped")
	if len(records) != 1 || records[0]["level"] != "ERROR" || records[0]["handled"] != false || records[0]["reason"] != "persist_failed" {
		t.Errorf("log %v, want one ERROR with persist_failed and handled=false", records)
	}
	if stopped := f.applier.Stopped(); !errors.Is(stopped, state.ErrPersistFailed) {
		t.Fatalf("the world after persist_failed: %v, want it stopped with persist_failed", stopped)
	}
	writes := len(objects.timeline.all())
	later := f.applier.Apply(context.Background(), update(t, "prop-later", "combat", true,
		set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	if !errors.Is(later, state.ErrWorldStopped) || len(objects.timeline.all()) != writes {
		t.Errorf("Apply after persist_failed = %v with %d more writes, want ErrWorldStopped and none",
			later, len(objects.timeline.all())-writes)
	}
	// Its memory may be behind what its last proposal wrote: no snapshot of it.
	if _, err := f.applier.Snapshot(context.Background(), state.SnapshotAdmin); !errors.Is(err, state.ErrWorldStopped) {
		t.Errorf("Snapshot of the stopped world = %v, want ErrWorldStopped", err)
	}
	if steps := objects.timeline.matching("put state/"); len(steps) != 0 {
		t.Errorf("snapshot writes %q of a stopped world", steps)
	}
}

// /health of a context whose world stopped on a write names persist_failed.
func TestHealthNamesPersistFailed(t *testing.T) {
	bus := rig(t)
	objects := newTracedObjects(t, world)
	c, manual := runningStored(t, bus, objects, -1)
	publish(t, bus.Bus, create("prop-born", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
	untilEnd(t, bus.Bus, 2)
	objects.setHook(func(_, key string) error {
		if key == "player/player-A.json" {
			return errors.New("the disk is full")
		}
		return nil
	})
	publish(t, bus.Bus, update(t, "prop-hit", "author", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -1))))
	waitFor(t, "the world to stop on the write", advancing(manual, func() bool {
		return worldHealth(c, world)["reason"] == "persist_failed"
	}))
	if health := c.Health(); health.Status != runtime.StatusFail {
		t.Errorf("health %q, want fail", health.Status)
	}
}
