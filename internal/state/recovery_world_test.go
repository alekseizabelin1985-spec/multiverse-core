package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	teststate "multiverse-core.io/shared/testkit/state"
)

// recover is the recovery of the Applier of a fixture over its object store,
// with no journal to catch up from: the entity objects and the intents.
func (f *fixture) recover(t *testing.T) state.Recovery {
	t.Helper()
	rec, err := f.applier.Recover(context.Background(), nil, 0)
	if err != nil || rec.Failure != nil {
		t.Fatalf("Recover = %+v, %v", rec, err)
	}
	return rec
}

// intentsOf is the intents of the world in the object store.
func intentsOf(t *testing.T, objects objstore.Client) []*state.Intent {
	t.Helper()
	intents, err := state.NewObjectStore(objects).ListIntents(context.Background(), world)
	if err != nil {
		t.Fatal(err)
	}
	return intents
}

func round(t *testing.T) eventbus.Event {
	return update(t, "prop-round", "author", true,
		set(ref("player-A", entity.TypePlayer), version(1), op(entity.OpInc, "hp", -2)),
		set(ref("wolf-alpha", entity.TypeNPC), version(1), op(entity.OpInc, "hp", -3)))
}

func (s *stored) seededRound(t *testing.T) {
	t.Helper()
	s.seededWorld(t)
	s.answered(t, create("prop-w", ref("wolf-alpha", entity.TypeNPC), "", map[string]any{"hp": 10}))
}

// repeat appends the same event again, as a redelivery or a proposer that
// sends its proposal again does.
func (s *stored) repeat(t *testing.T, ev eventbus.Event) {
	t.Helper()
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.bus.Append(eventbus.TopicSystemEvents, raw); err != nil {
		t.Fatal(err)
	}
}

// --- (d) the intents ---

// (d) of §11 and the acceptance of T-057: an atomic package cut between the
// writes of its entities is rolled forward by the restart before the world
// takes a proposal — the entity not written is written from the intent, the
// intent is removed, and nothing is published. The repeat of the proposal then
// sends the facts of the whole package, each once, and the world goes on.
func TestAnUnfinishedPackageIsRolledForwardBeforeItsRepeat(t *testing.T) {
	s := newStored(t)
	c, _ := s.start(t)
	s.seededRound(t)
	if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err != nil {
		t.Fatal(err)
	}
	s.objects.setHook(func(_, key string) error {
		if key == "npc/wolf-alpha.json" {
			return errors.New("the process died")
		}
		return nil
	})
	cut := round(t)
	publish(t, s.bus.Bus, cut)
	waitFor(t, "the world to stop between the writes", advancing(s.sources.Clock, func() bool {
		return worldHealth(c, world)["reason"] == "persist_failed"
	}))
	_ = stop(c)
	s.objects.setHook(nil)
	if n := len(intentsOf(t, s.objects.Memory)); n != 1 {
		t.Fatalf("%d intents, want the one of the cut package", n)
	}

	r, _ := s.start(t)
	rec := recoveryOf(t, r)
	if rec.RolledForward != 1 || rec.Accepted != 1 || rec.Failure != nil {
		t.Errorf("recovery %+v, want player-A taken in and wolf-alpha rolled forward", rec)
	}
	if n := len(intentsOf(t, s.objects.Memory)); n != 0 || worldHealth(r, world)["pending_intents"] != int64(0) {
		t.Errorf("%d intents and health %+v after the roll forward, want none", n, worldHealth(r, world))
	}
	wolf := s.objects.entityObject(t, world, entity.TypeNPC, "wolf-alpha")
	if wolf.Version != 2 || intAt(wolf, "hp") != 7 || wolf.LastChange.ProposalID != "prop-round" || !wolf.LastChange.Atomic ||
		wolf.LastChange.BatchSize != 2 || !wolf.LastChange.AppliedAt.Equal(cut.Timestamp) {
		t.Errorf("the object of wolf-alpha %+v, want v2 hp 7 under the commit record of prop-round", wolf)
	}
	if answers := answersTo(t, s.bus.Bus, "prop-round"); len(answers) != 0 {
		t.Fatalf("answers %v before the repeat: the roll forward publishes nothing", types(answers))
	}

	s.repeat(t, cut)
	waitFor(t, "the facts of the package", func() bool { return len(answersTo(t, s.bus.Bus, "prop-round")) == 2 })
	s.answered(t, hit(t, "prop-after", 1))
	byEntity := idsByEntity(answersTo(t, s.bus.Bus, "prop-round"))
	if len(byEntity["player-A"]) != 1 || len(byEntity["wolf-alpha"]) != 1 || r.Health().Status != runtime.StatusOK {
		t.Errorf("facts %v and health %s, want one fact of each entity and a world that goes on", byEntity, r.Health().Status)
	}
	teststate.OneStateOverTheWorld(t, allOn(t, s.bus.Bus, eventbus.TopicSystemEvents))
}

// The intent of a package written whole stays when its removal fails (§9: the
// world is not stopped). Here the answer did not get out either. The restart
// removes the intent — or, when the removal fails again, counts it in
// pending_intents — and the repeat of the proposal sends the facts that did
// not go out rather than stopping the world as an unfinished package.
func TestAWrittenPackageWhoseIntentStayedIsSentByItsRepeat(t *testing.T) {
	for name, removalFailsAgain := range map[string]bool{"the intent removed": false, "the intent stays": true} {
		t.Run(name, func(t *testing.T) {
			s := newStored(t)
			c, _ := s.start(t)
			s.seededRound(t)
			if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err != nil {
				t.Fatal(err)
			}
			s.objects.setDeleteHook(func(_, key string) error {
				if strings.HasPrefix(key, "_intents/") {
					return errors.New("the store refuses the delete")
				}
				return nil
			})
			var attempts atomic.Int32
			s.bus.setHook(func(_ context.Context, ev eventbus.Event) error {
				if ev.Type == state.TypeUpdated && subjectOf(ev) == "wolf-alpha" {
					attempts.Add(1)
					return errors.New("the broker is away")
				}
				return nil
			})
			cut := round(t)
			publish(t, s.bus.Bus, cut)
			waitFor(t, "a second attempt", advancing(s.sources.Clock, func() bool { return attempts.Load() >= 2 }))
			if pending := worldHealth(c, world)["pending_intents"]; pending != int64(1) {
				t.Errorf("pending_intents %v while the intent stays, want 1", pending)
			}
			_ = stop(c)
			s.bus.setHook(nil)
			if !removalFailsAgain {
				s.objects.setDeleteHook(nil)
			}

			r, _ := s.start(t)
			wantPending := 0
			if removalFailsAgain {
				wantPending = 1
			}
			if n := len(intentsOf(t, s.objects.Memory)); n != wantPending || worldHealth(r, world)["pending_intents"] != int64(wantPending) {
				t.Errorf("%d intents, health %+v; want %d", n, worldHealth(r, world), wantPending)
			}
			if rec := recoveryOf(t, r); rec.RolledForward != 0 || rec.Accepted != 1 || rec.EventsReplayed != 1 {
				t.Errorf("recovery %+v, want wolf-alpha taken in, player-A caught up and nothing rolled forward", rec)
			}
			s.repeat(t, cut)
			waitFor(t, "the fact of wolf-alpha", func() bool { return len(answersTo(t, s.bus.Bus, "prop-round")) == 2 })
			s.answered(t, hit(t, "prop-after", 1))
			if r.Health().Status == runtime.StatusFail {
				t.Errorf("health %+v, want a world that goes on", r.Health())
			}
			teststate.OneStateOverTheWorld(t, allOn(t, s.bus.Bus, eventbus.TopicSystemEvents))
		})
	}
}

// --- a world before its init (§18) ---

// A State over a store that holds nothing of its world — not even the buckets
// — is uninitialized: /health degraded {world: uninitialized}. A proposal of
// the gateway before mvctl world init is refused unknown_entity under the world
// entity before anything is written, so the world is not stopped with
// persist_failed; world init then passes without a restart, and the world is
// served.
func TestAWorldBeforeItsInitTakesOnlyItsInit(t *testing.T) {
	s := newStored(t)
	s.objects = newTracedObjects(t)
	c, _ := s.start(t)
	if rec := recoveryOf(t, c); !rec.Uninitialized {
		t.Errorf("recovery %+v, want uninitialized", rec)
	}
	if section := worldHealth(c, world); c.Health().Status != runtime.StatusDegraded || section["world"] != "uninitialized" {
		t.Errorf("health %+v, want degraded with world: uninitialized", section)
	}

	s.answered(t,
		proposed(t, gateway, "prop-move", "move", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpSet, "position", "dark-forest-01"))),
		createdBy(gateway, "prop-character", "create", ref("player-B", entity.TypePlayer), "", map[string]any{"hp": 10}))
	for _, id := range []string{"prop-move", "prop-character"} {
		answers := answersTo(t, s.bus.Bus, id)
		if len(answers) != 1 {
			t.Fatalf("answers to %s %v, want one refusal", id, types(answers))
		}
		assertRejected(t, answers[0], id, state.ReasonUnknownEntity, world)
	}
	if steps := s.objects.timeline.all(); len(steps) != 0 {
		t.Errorf("writes %q before the init, want none", steps)
	}
	if c.Health().Status == runtime.StatusFail {
		t.Fatalf("health %+v, want a world still waiting for its init", c.Health())
	}

	if err := state.EnsureWorldBuckets(context.Background(), s.objects.Memory, world); err != nil {
		t.Fatal(err)
	}
	s.seededWorld(t)
	if section := worldHealth(c, world); c.Health().Status != runtime.StatusOK || section["world"] != nil {
		t.Errorf("health %+v after the init, want ok", section)
	}
	s.answered(t, hit(t, "prop-after-init", 1))
	if answers := answersTo(t, s.bus.Bus, "prop-after-init"); len(answers) != 1 || answers[0].Type != state.TypeUpdated {
		t.Errorf("answers %v after the init, want entity.updated", types(answers))
	}
}

// --- /health of a world (§9, §10, §18 p. 2) ---

// Every field of the section of a world, on the manual clock: the snapshot with
// its age, snapshot_stale as the age of the last successful snapshot in whole
// seconds — not the text of its failure, which only the log gets —,
// snapshot_event_failed until a snapshot.created goes out, pending_intents and
// publish_attempts_failed.
func TestTheHealthOfAWorld(t *testing.T) {
	s := newStored(t)
	c, logged := s.start(t)
	s.seededWorld(t)
	pointer, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin)
	if err != nil {
		t.Fatal(err)
	}
	s.sources.Clock.Advance(90 * time.Second)
	section := worldHealth(c, world)
	snapshot, _ := section["snapshot"].(map[string]any)
	if section["status"] != runtime.StatusOK || section["seq"] != pointer.Snapshot.Seq || snapshot["age_s"] != int64(90) ||
		snapshot["seq"] != pointer.Snapshot.Seq || section["state_hash"] != pointer.Snapshot.StateHash || section["entities"] != 2 {
		t.Errorf("health %+v, want ok with the snapshot 90 s old", section)
	}

	s.refuseSnapshots()
	if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err == nil {
		t.Fatal("a snapshot the store refused was written")
	}
	s.sources.Clock.Advance(30 * time.Second)
	section = worldHealth(c, world)
	if c.Health().Status != runtime.StatusDegraded || section["snapshot_stale"] != int64(120) {
		t.Errorf("health %+v, want degraded with snapshot_stale 120", section)
	}
	if records := recordsOf(t, logged.String(), "snapshot not written; the world goes on"); len(records) != 1 ||
		!strings.Contains(records[0]["err"].(string), "the process died") {
		t.Errorf("log %v, want the failure of the snapshot in the log", records)
	}
	s.objects.setHook(nil)

	s.bus.setHook(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == state.TypeSnapshotCreated {
			return errors.New("the broker is away")
		}
		return nil
	})
	if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err == nil {
		t.Fatal("Snapshot hid that snapshot.created did not go out")
	}
	if section := worldHealth(c, world); section["snapshot_event_failed"] != true || section["snapshot_stale"] != nil ||
		c.Health().Status != runtime.StatusDegraded {
		t.Errorf("health %+v, want degraded with snapshot_event_failed", section)
	}
	s.bus.setHook(nil)
	if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err != nil {
		t.Fatal(err)
	}
	if section := worldHealth(c, world); section["snapshot_event_failed"] != nil || c.Health().Status != runtime.StatusOK {
		t.Errorf("health %+v once a snapshot.created went out, want ok", section)
	}

	var attempts atomic.Int32
	s.bus.setHook(func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == state.TypeUpdated {
			attempts.Add(1)
			return errors.New("the broker is away")
		}
		return nil
	})
	publish(t, s.bus.Bus, hit(t, "prop-stuck", 1))
	waitFor(t, "two failed attempts", advancing(s.sources.Clock, func() bool { return attempts.Load() >= 2 }))
	waitFor(t, "publish_attempts_failed in health", func() bool {
		n, _ := worldHealth(c, world)["publish_attempts_failed"].(int)
		return n >= 2
	})
	s.bus.setHook(nil)
	s.sources.Clock.Advance(time.Minute)
	waitFor(t, "the fact out", func() bool { return len(answersTo(t, s.bus.Bus, "prop-stuck")) == 1 })
	if section := worldHealth(c, world); section["publish_attempts_failed"] != nil || section["cursor"] == int64(0) {
		t.Errorf("health %+v once the fact is out, want no attempts and a cursor past the proposal", section)
	}
}

// --- the attempts of the object store (N-3 of review #1 of T-057) ---

// A missing bucket or object is an answer of the store and is not tried again:
// the world stops on the first refusal, without a pause.
func TestAMissingBucketIsNotTriedAgain(t *testing.T) {
	for name, refusal := range map[string]error{"no bucket": objstore.ErrNoBucket, "not found": objstore.ErrNotFound} {
		t.Run(name, func(t *testing.T) {
			f, objects := newStoredFixture(t, state.ApplierConfig{})
			f.seedWorld(t)
			f.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
			objects.setHook(func(_, key string) error {
				if key == "player/player-A.json" {
					return refusal
				}
				return nil
			})
			err := f.applier.Apply(context.Background(), hit(t, "prop-hit", 1))
			if !errors.Is(err, state.ErrPersistFailed) || !errors.Is(err, refusal) {
				t.Errorf("Apply = %v, want persist_failed at once", err)
			}
			if refused := objects.timeline.matching("refused player/"); len(refused) != 1 {
				t.Errorf("the write was tried %d times, want once", len(refused))
			}
		})
	}
}

// The list of intents a repeat asks for is asked with the attempts of a write:
// one refusal of the store does not stop the world (acceptance of T-057).
func TestTheListOfIntentsIsTriedAgain(t *testing.T) {
	first, objects := newStoredFixture(t, state.ApplierConfig{})
	first.seed(t, "player-A", entity.TypePlayer, 1, map[string]any{"hp": 10})
	lost := hit(t, "prop-lost", 2)
	first.journal.fail = func(int, eventbus.Event) error { return errors.New("the broker is away") }
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := first.applier.Apply(cancelled, lost); !errors.Is(err, state.ErrPublishFailed) {
		t.Fatalf("Apply = %v, want publish_failed", err)
	}

	second := newStoredFixtureOver(t, objects, state.ApplierConfig{})
	second.recover(t)
	var lists atomic.Int32
	objects.setListHook(func(_, prefix string) error {
		if prefix == "_intents/" && lists.Add(1) == 1 {
			return errors.New("the store blinked")
		}
		return nil
	})
	if err := second.applyAdvancing(t, context.Background(), lost); err != nil {
		t.Fatalf("Apply of the repeat = %v, want the fact sent", err)
	}
	if facts := second.journal.ofType(state.TypeUpdated); len(facts) != 1 || lists.Load() != 2 {
		t.Errorf("%d facts after %d lists, want the fact after a second list", len(facts), lists.Load())
	}
}

// --- the admin route ---

// POST /v1/admin/state/{world}/snapshot writes a snapshot with reason admin
// behind runtime.AdminOnly: a request without a client of the allow-list, or
// with an actor kind outside the envelope, is refused 403 and writes nothing.
func TestTheAdminRouteWritesASnapshot(t *testing.T) {
	s := newStored(t)
	c, _ := s.start(t)
	s.seededWorld(t)
	mux := http.NewServeMux()
	c.Routes(mux)
	post := func(path string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	path := "/v1/admin/state/" + world + "/snapshot"
	for name, headers := range map[string]map[string]string{
		"no client":             {runtime.ActorKindHeader: "ci"},
		"a client not listed":   {runtime.ClientIDHeader: "stranger", runtime.ActorKindHeader: "ci"},
		"an actor kind unknown": {runtime.ClientIDHeader: "ci-harness", runtime.ActorKindHeader: "robot"},
	} {
		if got := post(path, headers); got.Code != http.StatusForbidden {
			t.Errorf("%s: %d, want 403", name, got.Code)
		}
	}
	if steps := s.objects.timeline.matching("put state/"); len(steps) != 0 {
		t.Fatalf("snapshot writes %q of refused requests", steps)
	}

	admitted := map[string]string{runtime.ClientIDHeader: "ci-harness", runtime.ActorKindHeader: "ci"}
	got := post(path, admitted)
	var pointer state.LatestPointer
	if err := json.Unmarshal(got.Body.Bytes(), &pointer); got.Code != http.StatusOK || err != nil ||
		pointer.Snapshot.Reason != state.SnapshotAdmin || pointer.Snapshot.EntitiesCount != 2 {
		t.Fatalf("%d %s, want 200 with the pointer of a snapshot with reason admin", got.Code, got.Body)
	}
	if got := post(path, map[string]string{runtime.ClientIDHeader: "ci-harness"}); got.Code != http.StatusOK {
		t.Errorf("without X-Actor-Kind: %d, want 200: the kind is optional (runtime.AdminOnly)", got.Code)
	}
	if got := post("/v1/admin/state/another-world/snapshot", admitted); got.Code != http.StatusNotFound ||
		!strings.Contains(got.Body.String(), "unknown_world") {
		t.Errorf("another world: %d %s, want 404 unknown_world", got.Code, got.Body)
	}
	if got := httptest.NewRecorder(); true {
		mux.ServeHTTP(got, httptest.NewRequest(http.MethodGet, path, nil))
		if got.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET: %d, want 405", got.Code)
		}
	}

	memory := state.New(state.Config{Worlds: []string{world}})
	if err := memory.Start(context.Background(), runtime.Deps{Bus: newBus(t)}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop(memory) })
	memoryMux := http.NewServeMux()
	memory.Routes(memoryMux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set(runtime.ClientIDHeader, "ci-harness")
	memoryMux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "no_object_store") {
		t.Errorf("a State without a store: %d %s, want 409 no_object_store", rec.Code, rec.Body)
	}
}

// --- the rule of catching up (C-02 v1.6, §4.8) ---

// fromTheJournal is a fact as recovery reads it: published by State, encoded
// and decoded.
func fromTheJournal(t *testing.T, cause eventbus.Event, id string, v int64, changed []entity.Change) eventbus.Event {
	t.Helper()
	fact := eventbus.Derive(cause, state.TypeUpdated, contracts.SourceState, map[string]any{
		"entity":      map[string]any{"entity": map[string]any{"id": id, "type": entity.TypePlayer}},
		"version":     v,
		"changed":     changed,
		"cause":       "author",
		"proposal_id": "prop-vector",
		"applied_at":  cause.Timestamp,
	}, eventbus.WithCauseID(id))
	var decoded eventbus.Event
	if err := json.Unmarshal(wireBytes(t, fact), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func wireBytes(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// The named vectors of shared/entity go through the catching up of recovery:
// the world it leaves hashes as the world live application left
// (TestChangedReportsTheAncestorAProposalCreated,
// TestCatchingUpAppendsWithoutDeduplication,
// TestCatchingUpSkipsAPathAnEarlierEntryAlreadyRemoved), and an element past
// the end of its list is state_divergence
// (TestCatchingUpRefusesAnElementPastTheEndOfItsList). The attributes after are
// those ApplyOps gave for the operations of each vector.
func TestTheVectorsOfCatchingUp(t *testing.T) {
	pelt := func() map[string]any { return map[string]any{"item_id": "item-1"} }
	for name, tc := range map[string]struct {
		before, after map[string]any
		changed       []entity.Change
	}{
		"the ancestor a proposal created, not there": {
			before:  map[string]any{"hp": 10.0},
			changed: []entity.Change{{Path: "fresh", New: map[string]any{}, HasNew: true}},
			after:   map[string]any{"hp": 10.0, "fresh": map[string]any{}},
		},
		"the ancestor a proposal created over a scalar": {
			before:  map[string]any{"hp": 10.0, "fresh": "s"},
			changed: []entity.Change{{Path: "fresh", Old: "s", HasOld: true, New: map[string]any{}, HasNew: true}},
			after:   map[string]any{"hp": 10.0, "fresh": map[string]any{}},
		},
		"the ancestor after its paths": {
			before: map[string]any{"hp": 10.0},
			changed: []entity.Change{
				{Path: "fresh.mark", New: "fox", HasNew: true},
				{Path: "fresh", New: map[string]any{"mark": "fox"}, HasNew: true},
			},
			after: map[string]any{"hp": 10.0, "fresh": map[string]any{"mark": "fox"}},
		},
		"an append without deduplication": {
			before:  map[string]any{"inventory": []any{pelt()}},
			changed: []entity.Change{{Path: "inventory[1]", New: pelt(), HasNew: true}},
			after:   map[string]any{"inventory": []any{pelt(), pelt()}},
		},
		"a path an earlier entry already removed": {
			before: map[string]any{"inventory": []any{map[string]any{"item_id": "item-1", "kind": "wolf-pelt"}}},
			changed: []entity.Change{
				{Path: "inventory[0]", Old: map[string]any{"item_id": "item-1", "kind": "wolf-pelt"}, HasOld: true, New: map[string]any{}, HasNew: true},
				{Path: "inventory[0].kind", Old: "wolf-pelt", HasOld: true},
			},
			after: map[string]any{"inventory": []any{map[string]any{}}},
		},
		"the first element of a list that is not there": {
			before:  map[string]any{},
			changed: []entity.Change{{Path: "tags[0]", New: "hunted", HasNew: true}},
			after:   map[string]any{"tags": []any{"hunted"}},
		},
		"the first element over a null": {
			before:  map[string]any{"tags": nil},
			changed: []entity.Change{{Path: "tags[0]", New: "hunted", HasNew: true}},
			after:   map[string]any{"tags": []any{"hunted"}},
		},
		"a removed key and a removed element": {
			before: map[string]any{"hp": 10.0, "mark": "fox", "tags": []any{"a", "b"}},
			changed: []entity.Change{
				{Path: "mark", Old: "fox", HasOld: true},
				{Path: "tags", Old: []any{"a", "b"}, HasOld: true, New: []any{"b"}, HasNew: true},
			},
			after: map[string]any{"hp": 10.0, "tags": []any{"b"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.seed(t, "player-A", entity.TypePlayer, 1, wire(t, tc.before).(map[string]any))
			cause := hit(t, "prop-vector", 1)
			fact := fromTheJournal(t, cause, "player-A", 2, tc.changed)
			applied, err := f.applier.CatchUpFact(fact)
			if err != nil || !applied {
				t.Fatalf("CatchUpFact = %v, %v", applied, err)
			}
			want := entity.New(ref("player-A", entity.TypePlayer), world, "player-A", wire(t, tc.after).(map[string]any), time.Time{})
			want.Version = 2
			got := f.get(t, "player-A")
			if entity.StateHash([]*entity.Entity{got}) != entity.StateHash([]*entity.Entity{want}) {
				t.Errorf("caught up to %v, want %v", got.Attributes, want.Attributes)
			}
			if got.LastEventID != fact.ID || got.LastChange.FactEventID != fact.ID || got.LastChange.ProposalID != "prop-vector" {
				t.Errorf("player-A %+v, want the fact and its proposal remembered", got)
			}
		})
	}

	for name, tc := range map[string]struct {
		before  map[string]any
		path    string
		corrupt bool
	}{
		"one past the end":                      {map[string]any{"tags": []any{"wounded"}}, "tags[1]", false},
		"inside the list":                       {map[string]any{"tags": []any{"wounded"}}, "tags[0]", false},
		"no list, the first element":            {map[string]any{}, "tags[0]", false},
		"null in place of the list, the first":  {map[string]any{"tags": nil}, "tags[0]", false},
		"two past the end":                      {map[string]any{"tags": []any{"wounded"}}, "tags[2]", true},
		"no list, the second element":           {map[string]any{}, "tags[1]", true},
		"null in place of the list, the second": {map[string]any{"tags": nil}, "tags[1]", true},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.seed(t, "player-A", entity.TypePlayer, 1, wire(t, tc.before).(map[string]any))
			fact := fromTheJournal(t, hit(t, "prop-vector", 1), "player-A", 2,
				[]entity.Change{{Path: tc.path, New: "hunted", HasNew: true}})
			_, err := f.applier.CatchUpFact(fact)
			if errors.Is(err, state.ErrStateDivergence) != tc.corrupt || (!tc.corrupt && err != nil) {
				t.Errorf("CatchUpFact = %v, want state_divergence %v", err, tc.corrupt)
			}
		})
	}
}

// What the live pipeline decided and what recovery catches up from its facts
// are one world: every fact of a run of proposals — an increment, an append, a
// set of a nested path, a remove, a turn that changed nothing — applied over the
// world before them hashes as the world after them.
func TestCatchingUpOnTheFactsOfTheLivePipelineGivesTheSameWorld(t *testing.T) {
	live := newFixture(t)
	start := map[string]any{"hp": 10, "position": "outside", "inventory": []any{}}
	live.seed(t, "player-A", entity.TypePlayer, 1, start)
	for i, ops := range [][]entity.Op{
		{op(entity.OpInc, "hp", -3)},
		{op(entity.OpAppend, "inventory", map[string]any{"item_id": "pelt-1", "kind": "wolf-pelt"})},
		{op(entity.OpSet, "position", "dark-forest-01"), op(entity.OpAppend, "inventory", map[string]any{"item_id": "fang-1"})},
		{op(entity.OpRemove, "inventory", map[string]any{"item_id": "pelt-1"})},
		{op(entity.OpSet, "hp", 7)},
	} {
		live.apply(t, update(t, "prop-live-"+itoa(i), "author", true, set(ref("player-A", entity.TypePlayer), nil, ops...)))
	}

	caught := newFixture(t)
	caught.seed(t, "player-A", entity.TypePlayer, 1, wire(t, start).(map[string]any))
	replayed := 0
	for _, fact := range live.journal.ofType(state.TypeUpdated) {
		applied, err := caught.applier.CatchUpFact(fact)
		if err != nil {
			t.Fatalf("CatchUpFact %s: %v", fact.ID, err)
		}
		if applied {
			replayed++
		}
	}
	got, want := caught.get(t, "player-A"), live.get(t, "player-A")
	if entity.StateHash([]*entity.Entity{got}) != entity.StateHash([]*entity.Entity{want}) || got.Version != want.Version {
		t.Errorf("caught up to v%d %v, the live world is v%d %v", got.Version, got.Attributes, want.Version, want.Attributes)
	}
	if replayed != 5 {
		t.Errorf("%d facts applied, want all five", replayed)
	}
	for _, fact := range live.journal.ofType(state.TypeUpdated) {
		if applied, err := caught.applier.CatchUpFact(fact); applied || err != nil {
			t.Errorf("the fact %s a second time = %v, %v; want it passed over", fact.ID, applied, err)
		}
	}
}
