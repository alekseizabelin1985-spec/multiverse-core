package state_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
	teststate "multiverse-core.io/shared/testkit/state"
)

// --- a process over a store, started again and again ---

// stored is one bus and one object store that a test starts State over as
// many times as it restarts the process. The deterministic sources are taken
// once: a second call would start the sequence of ids again.
type stored struct {
	bus     *rigged
	objects *tracedObjects
	sources testkit.Sources
	journal eventbus.Journal
}

func newStored(t *testing.T) *stored {
	t.Helper()
	sources := testkit.Deterministic(t, "t059")
	eventbus.SetRegistry(contracts.Default())
	return &stored{bus: rig(t), objects: newTracedObjects(t, world), sources: sources}
}

// start is the start of a process: a new State over the bus and the store.
// The manual clock moves on while it starts, so that the pauses between the
// attempts of a recovery against a store that refuses end.
func (s *stored) start(t *testing.T) (*state.Context, *lockedBuffer) {
	t.Helper()
	logged := &lockedBuffer{}
	c := state.New(state.Config{Worlds: []string{world}, Timers: s.sources.Timers,
		Objects: s.objects, SnapshotEvery: -1, RulesVersion: "0.1",
		Log: slog.New(slog.NewJSONHandler(logged, &slog.HandlerOptions{Level: slog.LevelDebug}))})
	started := make(chan error, 1)
	go func() {
		started <- c.Start(context.Background(), runtime.Deps{Bus: s.bus, Journal: s.journal, Clock: s.sources.Clock})
	}()
	var err error
	waitFor(t, "Start to return", advancing(s.sources.Clock, func() bool {
		select {
		case err = <-started:
			return true
		default:
			return false
		}
	}))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		_ = c.Stop(ctx)
	})
	return c, logged
}

// stop is a Stop within the budget of the process; its error is returned, not
// judged.
func stop(c *state.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
	defer cancel()
	return c.Stop(ctx)
}

// answered publishes proposals and waits until each has an answer.
func (s *stored) answered(t *testing.T, proposals ...eventbus.Event) {
	t.Helper()
	publish(t, s.bus.Bus, proposals...)
	for _, p := range proposals {
		id, _ := p.Path().GetString("proposal_id")
		waitFor(t, "the answer to "+id, func() bool { return len(answersTo(t, s.bus.Bus, id)) > 0 })
	}
}

// allOn is every event of a topic as a subscriber reads it.
func allOn(t *testing.T, bus *membus.Bus, topic string) []eventbus.Event {
	t.Helper()
	records, err := bus.Records(topic)
	if err != nil {
		t.Fatalf("records of %s: %v", topic, err)
	}
	out := make([]eventbus.Event, 0, len(records))
	for _, body := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatalf("decode: %v", err)
		}
		out = append(out, ev)
	}
	return out
}

// replaysCompleted is every analytics.replay.completed on the bus.
func replaysCompleted(t *testing.T, bus *membus.Bus) []eventbus.Event {
	t.Helper()
	var out []eventbus.Event
	for _, ev := range allOn(t, bus, eventbus.TopicAnalyticsEvents) {
		if ev.Type == state.TypeReplayCompleted {
			out = append(out, ev)
		}
	}
	return out
}

func recoveryOf(t *testing.T, c *state.Context) state.Recovery {
	t.Helper()
	rec, ok := c.Recovery(world)
	if !ok {
		t.Fatal("no recovery of the world")
	}
	return rec
}

// theWorld is the world as it stands: its hash from /health.
func theWorld(c *state.Context) string {
	hash, _ := worldHealth(c, world)["state_hash"].(string)
	return hash
}

// seededWorld is the world entity and player-A at hp 10, answered.
func (s *stored) seededWorld(t *testing.T) {
	t.Helper()
	s.answered(t, createWorld(), create("prop-a", ref("player-A", entity.TypePlayer), "", map[string]any{"hp": 10}))
}

func hit(t *testing.T, proposalID string, by int) eventbus.Event {
	return update(t, proposalID, "author", true, set(ref("player-A", entity.TypePlayer), nil, op(entity.OpInc, "hp", -by)))
}

// refuseSnapshots makes every write of a snapshot fail, the way a crash before
// the shutdown snapshot leaves the store.
func (s *stored) refuseSnapshots() {
	s.objects.setHook(func(_, key string) error {
		if strings.HasPrefix(key, state.Component+"/") {
			return errors.New("the process died")
		}
		return nil
	})
}

// --- (a) a shutdown and a restart ---

// (a) of §11: after a shutdown the world comes back as the snapshot of the
// shutdown — identical, nothing replayed — analytics.replay.completed says so
// and is valid against its schema, and the world goes on. A second restart
// without a proposal publishes no fact at all: the guard of one State per
// world holds over every restart.
func TestAShutdownAndARestartGiveTheSameWorld(t *testing.T) {
	s := newStored(t)
	c, _ := s.start(t)
	s.seededWorld(t)
	s.answered(t, hit(t, "prop-hit", 2))
	before := theWorld(c)
	if err := stop(c); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	r, _ := s.start(t)
	rec := recoveryOf(t, r)
	if identical, known := rec.Identical(); !identical || !known || rec.EventsReplayed != 0 || rec.SnapshotID == "" {
		t.Errorf("recovery %+v, want identical from the shutdown snapshot with nothing replayed", rec)
	}
	if rec.StateHashAfter != before || theWorld(r) != before {
		t.Errorf("the world after the restart %s, before it %s", rec.StateHashAfter, before)
	}
	end, _ := s.bus.End(context.Background(), eventbus.TopicSystemEvents)
	if section := worldHealth(r, world); section["status"] != runtime.StatusOK || section["cursor"] != end ||
		section["rules_version"] != "0.1" || section["pending_intents"] != int64(0) {
		t.Errorf("health %+v, want ok at the cursor %d", section, end)
	}
	replays := replaysCompleted(t, s.bus.Bus)
	if len(replays) != 2 {
		t.Fatalf("%d analytics.replay.completed, want one per start", len(replays))
	}
	signal := replays[1]
	if err := contracts.Validate(signal); err != nil {
		t.Errorf("analytics.replay.completed is not valid: %v", err)
	}
	p := signal.Path()
	mode, _ := p.GetString("mode")
	snapshotID, _ := p.GetString("replay.snapshot_id")
	identical, _ := p.GetBool("replay.identical")
	replayed, _ := p.GetInt("replay.events_replayed")
	llm, _ := p.GetInt("replay.llm_calls")
	dice, _ := p.GetInt("replay.dice_rolled_new")
	if mode != "recovery" || snapshotID != rec.SnapshotID || !identical || replayed != 0 || llm != 0 || dice != 0 ||
		signal.Source != contracts.SourceState || eventbus.GetWorldIDFromEvent(signal) != world {
		t.Errorf("analytics.replay.completed %+v, want mode recovery, identical, nothing replayed, from core/state in the world", signal.Payload)
	}

	s.answered(t, hit(t, "prop-after", 1))
	if e, _ := r.Get(world, "player-A"); e.Version != 3 || intAt(e, "hp") != 7 {
		t.Errorf("player-A %+v, want v3 hp 7 after one more hit", e)
	}
	facts := len(onTopic(t, s.bus.Bus, state.TypeUpdated))
	if err := stop(r); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	again, _ := s.start(t)
	if identical, _ := recoveryOf(t, again).Identical(); !identical {
		t.Errorf("the second restart %+v, want identical", recoveryOf(t, again))
	}
	if got := len(onTopic(t, s.bus.Bus, state.TypeUpdated)); got != facts {
		t.Errorf("%d facts after the second restart, want %d: a restart publishes none", got, facts)
	}
	teststate.OneStateOverTheWorld(t, allOn(t, s.bus.Bus, eventbus.TopicSystemEvents))
}

// --- (b) the facts after the snapshot ---

// (b) of §11: a process that died after its snapshot leaves facts in the
// journal the snapshot does not hold. The restart catches them up — the same
// world as the one that died, not identical to its snapshot — and publishes
// none of them again.
func TestTheFactsAfterTheSnapshotAreCaughtUp(t *testing.T) {
	s := newStored(t)
	c, _ := s.start(t)
	s.seededWorld(t)
	if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	s.answered(t, hit(t, "prop-1", 1), hit(t, "prop-2", 1), hit(t, "prop-3", 1))
	died := theWorld(c)
	s.refuseSnapshots()
	_ = stop(c)
	s.objects.setHook(nil)

	r, _ := s.start(t)
	rec := recoveryOf(t, r)
	identical, known := rec.Identical()
	if rec.EventsReplayed != 3 || identical || !known || rec.StateHashAfter != died {
		t.Errorf("recovery %+v, want three facts caught up to the world that died (%s)", rec, died)
	}
	facts := onTopic(t, s.bus.Bus, state.TypeUpdated)
	e, _ := r.Get(world, "player-A")
	if e.Version != 4 || intAt(e, "hp") != 7 || e.LastEventID != facts[len(facts)-1].ID ||
		e.LastChange == nil || e.LastChange.FactEventID != facts[len(facts)-1].ID || e.LastChange.ProposalID != "prop-3" {
		t.Errorf("player-A %+v, want v4 hp 7 announced by the last fact of prop-3", e)
	}
	if replayed, _ := replaysCompleted(t, s.bus.Bus)[1].Path().GetInt("replay.events_replayed"); replayed != 3 {
		t.Errorf("events_replayed %d, want 3", replayed)
	}
	s.answered(t, hit(t, "prop-4", 1))
	teststate.OneStateOverTheWorld(t, allOn(t, s.bus.Bus, eventbus.TopicSystemEvents))
}

// C-14 v1.2 (a): the window of proposal identifiers is in the snapshot and is
// completed by the facts caught up. A proposal whose entity was changed again
// since is recognised by the window alone — its commit record names the later
// proposal — and a repeat gets no answer: neither for one applied before the
// snapshot, nor for one whose fact is in the journal after the cursor, nor for
// a create whose entity has moved on.
func TestTheWindowOfProposalsSurvivesARestart(t *testing.T) {
	s := newStored(t)
	c, _ := s.start(t)
	s.seededWorld(t)
	// One after the other: the fact of prop-before is before the cursor of the
	// snapshot, so only the window of the snapshot remembers it.
	s.answered(t, hit(t, "prop-before", 1))
	s.answered(t, hit(t, "prop-over-it", 1))
	if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	s.answered(t,
		create("prop-wolf", ref("wolf-alpha", entity.TypeNPC), "", map[string]any{"hp": 5}),
		hit(t, "prop-after", 1), hit(t, "prop-over-after", 1),
		update(t, "prop-wolf-hit", "author", true, set(ref("wolf-alpha", entity.TypeNPC), nil, op(entity.OpInc, "hp", -1))))
	s.refuseSnapshots()
	_ = stop(c)
	s.objects.setHook(nil)

	r, _ := s.start(t)
	for _, id := range []string{"prop-before", "prop-after", "prop-wolf"} {
		before := len(answersTo(t, s.bus.Bus, id))
		repeat := hit(t, id, 1)
		if id == "prop-wolf" {
			repeat = create(id, ref("wolf-alpha", entity.TypeNPC), "", map[string]any{"hp": 5})
		}
		sentinel := hit(t, "sentinel-"+id, 1)
		s.answered(t, repeat, sentinel)
		if got := len(answersTo(t, s.bus.Bus, id)); got != before {
			t.Errorf("%s answered %d times after the restart, want no answer to a repeat", id, got-before)
		}
	}
	if e, _ := r.Get(world, "player-A"); e.Version != 8 {
		t.Errorf("player-A at v%d, want v8: four hits before the restart and three sentinels", e.Version)
	}
	teststate.OneStateOverTheWorld(t, allOn(t, s.bus.Bus, eventbus.TopicSystemEvents))
}

// --- (e) a snapshot that does not read whole ---

// corrupt overwrites a snapshot object with bytes that are not JSON.
func (s *stored) corrupt(t *testing.T, key string) {
	t.Helper()
	if _, err := s.objects.Memory.Put(context.Background(), objstore.SnapshotsBucket(world), key, []byte(`{"schema_version": 1,`),
		objstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
}

// tamper changes an entity of a snapshot object and leaves its state_hash.
func (s *stored) tamper(t *testing.T, key string) {
	t.Helper()
	var snap state.Snapshot
	readJSON(t, s.objects.Memory, objstore.SnapshotsBucket(world), key, &snap)
	snap.Entities[len(snap.Entities)-1].Attributes["hp"] = 999
	body, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.objects.Memory.Put(context.Background(), objstore.SnapshotsBucket(world), key, body, objstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
}

// twoSnapshots is a world with a snapshot, a hit, a second snapshot and a
// second hit, and a process that died without a shutdown snapshot. It returns
// the keys of the snapshots, oldest first, and the world that died.
func (s *stored) twoSnapshots(t *testing.T) ([]string, string) {
	t.Helper()
	c, _ := s.start(t)
	s.seededWorld(t)
	var keys []string
	for i, id := range []string{"prop-1", "prop-2"} {
		pointer, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin)
		if err != nil {
			t.Fatalf("Snapshot %d: %v", i, err)
		}
		keys = append(keys, pointer.Snapshot.Key)
		s.sources.Clock.Advance(time.Second)
		s.answered(t, hit(t, id, 1))
	}
	died := theWorld(c)
	s.refuseSnapshots()
	_ = stop(c)
	s.objects.setHook(nil)
	return keys, died
}

// (e) of §11: the snapshot latest.json names does not read — it gives way to
// the one kept before it, and the facts after that one's cursor are caught up.
func TestACorruptedSnapshotGivesWayToTheOneBefore(t *testing.T) {
	for name, spoil := range map[string]func(s *stored, t *testing.T, key string){
		"not JSON":         (*stored).corrupt,
		"a hash that lies": (*stored).tamper,
	} {
		t.Run(name, func(t *testing.T) {
			s := newStored(t)
			keys, died := s.twoSnapshots(t)
			spoil(s, t, keys[1])

			r, logged := s.start(t)
			rec := recoveryOf(t, r)
			if rec.Failure != nil || rec.FellBack != keys[0] || rec.EventsReplayed != 2 || rec.StateHashAfter != died {
				t.Errorf("recovery %+v, want the world that died rebuilt from %s", rec, keys[0])
			}
			if records := recordsOf(t, logged.String(), "a snapshot does not read whole; the one kept before it is tried"); len(records) != 1 ||
				records[0]["key"] != keys[1] {
				t.Errorf("log %v, want the refusal of %s", records, keys[1])
			}
			if health := r.Health(); health.Status != runtime.StatusOK {
				t.Errorf("health %+v, want ok", health)
			}
		})
	}
}

// (e) of §11: every snapshot kept is corrupted. The world is not served — not
// rebuilt empty, not answered, not snapshotted over — and /health fails with
// snapshot: corrupted.
func TestEverySnapshotCorruptedStopsTheWorld(t *testing.T) {
	s := newStored(t)
	keys, _ := s.twoSnapshots(t)
	s.corrupt(t, keys[1])
	s.tamper(t, keys[0])
	pointer := readJSON(t, s.objects.Memory, objstore.SnapshotsBucket(world), state.PointerKey, &state.LatestPointer{})
	replays := len(replaysCompleted(t, s.bus.Bus))

	r, _ := s.start(t)
	rec := recoveryOf(t, r)
	if !errors.Is(rec.Failure, state.ErrSnapshotCorrupted) {
		t.Fatalf("recovery %+v, want snapshot_corrupted", rec)
	}
	section := worldHealth(r, world)
	if r.Health().Status != runtime.StatusFail || section["reason"] != "snapshot_corrupted" ||
		section["snapshot"] != "corrupted" || section["entities"] != 0 {
		t.Errorf("health %+v, want fail with snapshot: corrupted and no entity", section)
	}
	if len(replaysCompleted(t, s.bus.Bus)) != replays {
		t.Error("analytics.replay.completed of a world that is not served")
	}
	publish(t, s.bus.Bus, hit(t, "prop-refused", 1))
	waitFor(t, "the proposal refused by the stopped world", func() bool {
		return refusedByTheStoppedWorld(s.bus.Bus, "prop-refused")
	})
	if answers := answersTo(t, s.bus.Bus, "prop-refused"); len(answers) != 0 {
		t.Errorf("answers %v of a world that is not served", types(answers))
	}
	writes := len(s.objects.timeline.all())
	if err := stop(r); err != nil {
		t.Errorf("Stop: %v", err)
	}
	if steps := s.objects.timeline.all()[writes:]; len(steps) != 0 {
		t.Errorf("writes %q on the Stop of a world that is not served", steps)
	}
	if after := readJSON(t, s.objects.Memory, objstore.SnapshotsBucket(world), state.PointerKey, &state.LatestPointer{}); string(after) != string(pointer) {
		t.Error("latest.json was written over")
	}
}

// --- (f) a world that does not fit together ---

// fact publishes an entity.updated of player-A as State would, over the
// proposal of a test.
func (s *stored) fact(t *testing.T, proposalID string, v int64, changed ...entity.Change) eventbus.Event {
	t.Helper()
	cause := hit(t, proposalID, 1)
	fact := eventbus.Derive(cause, state.TypeUpdated, contracts.SourceState, map[string]any{
		"entity":      map[string]any{"entity": map[string]any{"id": "player-A", "type": entity.TypePlayer}},
		"version":     v,
		"changed":     changed,
		"cause":       "author",
		"proposal_id": proposalID,
		"applied_at":  cause.Timestamp,
	}, eventbus.WithCauseID("player-A"))
	publish(t, s.bus.Bus, fact)
	return fact
}

func hpTo(old, new float64) entity.Change {
	return entity.Change{Path: "hp", Old: old, HasOld: true, New: new, HasNew: true}
}

// (f) of §11: what the journal and the objects say does not fit — a version
// that skips, two facts under one version, an element past the end of its
// list, an object two versions ahead. The world is not served, /health fails
// with state_divergence and nothing is published for it.
func TestADivergenceStopsTheWorld(t *testing.T) {
	const byTheJournal = "a fact of the journal does not fit the world; the world is not served"
	const byTheObjects = "the entity objects do not fit the world; the world is not served"
	for name, tc := range map[string]struct {
		spoil func(s *stored, t *testing.T)
		msg   string
	}{
		"a version that skips one": {func(s *stored, t *testing.T) { s.fact(t, "prop-x", 3, hpTo(10, 9)) }, byTheJournal},
		"two facts under one version": {func(s *stored, t *testing.T) {
			s.fact(t, "prop-x", 2, hpTo(10, 9))
			s.fact(t, "prop-y", 2, hpTo(10, 8))
		}, byTheJournal},
		"an element past the end of its list": {func(s *stored, t *testing.T) {
			s.fact(t, "prop-x", 2, entity.Change{Path: "inventory[1]", New: map[string]any{"item_id": "i"}, HasNew: true})
		}, byTheJournal},
		"an object two versions ahead": {func(s *stored, t *testing.T) {
			ahead := s.objects.entityObject(t, world, entity.TypePlayer, "player-A")
			ahead.Version = 3
			if err := state.NewObjectStore(s.objects.Memory).PutEntity(context.Background(), world, ahead); err != nil {
				t.Fatal(err)
			}
		}, byTheObjects},
	} {
		t.Run(name, func(t *testing.T) {
			s := newStored(t)
			c, _ := s.start(t)
			s.seededWorld(t)
			if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err != nil {
				t.Fatal(err)
			}
			if err := stop(c); err != nil {
				t.Fatal(err)
			}
			tc.spoil(s, t)
			replays := len(replaysCompleted(t, s.bus.Bus))

			r, logged := s.start(t)
			if rec := recoveryOf(t, r); !errors.Is(rec.Failure, state.ErrStateDivergence) {
				t.Fatalf("recovery %+v, want state_divergence", rec)
			}
			if section := worldHealth(r, world); r.Health().Status != runtime.StatusFail || section["reason"] != "state_divergence" {
				t.Errorf("health %+v, want fail with state_divergence", section)
			}
			if len(replaysCompleted(t, s.bus.Bus)) != replays {
				t.Error("analytics.replay.completed of a world that is not served")
			}
			stopped := recordsOf(t, logged.String(), tc.msg)
			if len(stopped) != 1 || stopped[0]["reason"] != "state_divergence" || stopped[0]["handled"] != false {
				t.Errorf("log %v, want one ERROR %q with reason state_divergence", stopped, tc.msg)
			}
		})
	}
}

// --- (g) a gap in the journal ---

// trimmed is a journal whose offsets below earliest are gone past the
// retention of the broker.
type trimmed struct {
	*membus.Bus
	earliest int64
}

func (j trimmed) ReadRange(ctx context.Context, topic string, from, to int64, h eventbus.Handler) (int64, error) {
	return j.Bus.ReadRange(ctx, topic, max(from, j.earliest), to, h)
}

// (g) of §11 and UC-025 E3: the journal no longer holds the facts after the
// cursor of the snapshot — trimmed by retention, or a younger journal than the
// snapshot, the memory bus of a restarted process. The world is rebuilt from
// the entity objects, which are never behind: the same world as the one that
// died. /health is degraded with log_gap until a snapshot is written. An
// entity at the version of the snapshot keeps the fact that announced it; one
// ahead of it does not know whether its fact went out.
func TestAGapInTheJournalRebuildsTheWorldFromTheObjects(t *testing.T) {
	for name, journal := range map[string]func(s *stored, t *testing.T) eventbus.Journal{
		"offsets past the retention": func(s *stored, t *testing.T) eventbus.Journal {
			end, _ := s.bus.End(context.Background(), eventbus.TopicSystemEvents)
			return trimmed{Bus: s.bus.Bus, earliest: end - 1}
		},
		"a journal younger than the snapshot": func(s *stored, t *testing.T) eventbus.Journal {
			return newBus(t)
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := newStored(t)
			c, _ := s.start(t)
			s.seededWorld(t)
			s.answered(t, createWolf())
			if _, err := c.Snapshot(context.Background(), world, state.SnapshotAdmin); err != nil {
				t.Fatal(err)
			}
			s.answered(t, hit(t, "prop-1", 1), hit(t, "prop-2", 1))
			died := theWorld(c)
			s.refuseSnapshots()
			_ = stop(c)
			s.objects.setHook(nil)

			s.journal = journal(s, t)
			r, _ := s.start(t)
			rec := recoveryOf(t, r)
			if !rec.LogGap || rec.Failure != nil || rec.StateHashAfter != died {
				t.Errorf("recovery %+v, want log_gap and the world that died (%s)", rec, died)
			}
			section := worldHealth(r, world)
			if r.Health().Status != runtime.StatusDegraded || section["log_gap"] != true {
				t.Errorf("health %+v, want degraded with log_gap", section)
			}
			if wolf, _ := r.Get(world, "wolf-alpha"); wolf.LastChange == nil || wolf.LastChange.FactEventID == "" {
				t.Errorf("wolf-alpha %+v, want the fact of its snapshot remembered", wolf)
			}
			if player, _ := r.Get(world, "player-A"); player.Version != 3 || player.LastChange.FactEventID != "" {
				t.Errorf("player-A %+v, want v3 from its object, its fact unknown", player)
			}
			if _, err := r.Snapshot(context.Background(), world, state.SnapshotAdmin); err != nil {
				t.Fatal(err)
			}
			if section := worldHealth(r, world); r.Health().Status != runtime.StatusOK || section["log_gap"] != nil {
				t.Errorf("health %+v after a snapshot, want ok", section)
			}
		})
	}
}

func createWolf() eventbus.Event {
	return create("prop-wolf-born", ref("wolf-alpha", entity.TypeNPC), "", map[string]any{"hp": 5})
}
