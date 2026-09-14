package snapshot_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/snapshot"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/testkit"
)

const world = "dark-forest-world"

var t0 = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

// recorder keeps what the writer publishes; fail refuses it.
type recorder struct {
	mu     sync.Mutex
	events []eventbus.Event
	fail   bool
}

func (r *recorder) Publish(_ context.Context, ev eventbus.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("broker down")
	}
	r.events = append(r.events, ev)
	return nil
}

func (r *recorder) all() []eventbus.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

type fixture struct {
	store *objstore.Memory
	bus   *recorder
	clock *clock.Manual
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	testkit.Deterministic(t, "snap")
	return &fixture{store: objstore.NewMemory(), bus: &recorder{}, clock: clock.NewManual(t0)}
}

func (f *fixture) writer(t *testing.T) *snapshot.Writer {
	t.Helper()
	w, err := snapshot.New(snapshot.Config{Objects: f.store, Bus: f.bus, WorldID: world, Clock: f.clock})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func (f *fixture) get(t *testing.T, key string, dst any) []byte {
	t.Helper()
	body, err := f.store.Get(context.Background(), objstore.SnapshotsBucket(world), key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	if dst != nil {
		if err := json.Unmarshal(body, dst); err != nil {
			t.Fatalf("decode %s: %v", key, err)
		}
	}
	return body
}

func (f *fixture) objectKeys(t *testing.T) []string {
	t.Helper()
	infos, err := f.store.List(context.Background(), objstore.SnapshotsBucket(world), snapshot.Component+"/")
	if errors.Is(err, objstore.ErrNoBucket) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, info := range infos {
		if info.Key != snapshot.PointerKey {
			keys = append(keys, info.Key)
		}
	}
	return keys
}

func state() snapshot.State {
	return snapshot.State{
		Cursors: snapshot.Cursors{
			Projection: map[string]int64{eventbus.TopicSystemEvents: 12, eventbus.TopicWorldEvents: 3},
			Effects:    map[string]int64{eventbus.TopicSystemEvents: 12, eventbus.TopicNarrativeOutput: 7},
		},
		ProjectionHash: entity.StateHash(nil),
		ActiveSessions: []snapshot.Session{{ID: "player-A:1789380000", WorldID: world,
			Scope: eventbus.ScopeRef{ID: "player-A", Type: "solo"}, Kind: "solo", ActorKind: eventbus.ActorHuman,
			Participants: []string{"player-A"}, StartedAt: t0.Add(-time.Minute), LastActionAt: t0, TurnsCount: 2}},
		OpenRounds:  []snapshot.Round{{ScopeID: "group-1", Seq: 2, EncounterID: "enc-1", State: "open", DeadlineAt: t0.Add(time.Minute)}},
		LawsVersion: "v1",
	}
}

// A snapshot is the object, then the pointer at it, then snapshot.created:
// the object holds the cursors, the hash, the sessions and the rounds; the
// pointer names the object and its size; the event is the block of the
// pointer, valid by its schema, with the world in the envelope (C-14 v1.1,
// v1.3).
func TestAWriteLeavesTheObjectThePointerAndAValidEvent(t *testing.T) {
	f := newFixture(t)
	st := state()
	p, err := f.writer(t).Write(context.Background(), st, snapshot.ReasonShutdown)
	if err != nil {
		t.Fatal(err)
	}
	wantKey := "gateway/20260914T100000Z-000000.json"
	if p.Snapshot.Key != wantKey || p.Snapshot.Seq != 0 || p.Snapshot.ID != "gateway:"+world+":000000" ||
		p.Component != snapshot.Component || p.World.Entity.ID != world || p.Snapshot.Reason != snapshot.ReasonShutdown {
		t.Errorf("pointer = %+v", p)
	}
	var object snapshot.Object
	body := f.get(t, wantKey, &object)
	if p.Snapshot.SizeBytes == nil || *p.Snapshot.SizeBytes != int64(len(body)) {
		t.Errorf("size_bytes = %v, want %d", p.Snapshot.SizeBytes, len(body))
	}
	if object.ProjectionHash != st.ProjectionHash || object.LawsVersion != "v1" || object.WorldID != world ||
		object.Snapshot.SizeBytes != nil ||
		object.Cursors.Effects[eventbus.TopicNarrativeOutput] != 7 || object.Cursors.Projection[eventbus.TopicWorldEvents] != 3 ||
		len(object.ActiveSessions) != 1 || object.ActiveSessions[0].TurnsCount != 2 ||
		len(object.OpenRounds) != 1 || object.OpenRounds[0].EncounterID != "enc-1" {
		t.Errorf("object = %+v", object)
	}
	var pointer snapshot.Pointer
	f.get(t, snapshot.PointerKey, &pointer)
	if pointer.Snapshot.Key != wantKey {
		t.Errorf("latest.json names %s, want %s", pointer.Snapshot.Key, wantKey)
	}

	events := f.bus.all()
	if len(events) != 1 {
		t.Fatalf("published %d events, want one snapshot.created", len(events))
	}
	ev := events[0]
	if err := contracts.Default().Validate(ev); err != nil {
		t.Fatalf("snapshot.created is not valid by its schema: %v", err)
	}
	pa := ev.Path()
	component, _ := pa.GetString("component")
	key, _ := pa.GetString("snapshot.key")
	hash, _ := pa.GetString("snapshot.state_hash")
	size, _ := pa.GetInt("snapshot.size_bytes")
	cursor, _ := pa.GetInt("snapshot.cursor." + eventbus.TopicSystemEvents)
	if ev.Type != snapshot.TypeCreated || ev.Source != contracts.SourceGateway || ev.World == nil || ev.World.Entity.ID != world ||
		component != snapshot.Component || key != wantKey || hash != st.ProjectionHash || size != len(body) || cursor != 12 ||
		pa.Has("snapshot.reason") {
		t.Errorf("snapshot.created = %+v", ev)
	}
}

// The writer keeps the last five snapshots and latest.json names the last;
// a writer of a restarted process continues the sequence it finds (C-14 v1.1).
func TestTheLastFiveStayAndTheSequenceContinuesAfterARestart(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w := f.writer(t)
	for range 4 {
		if _, err := w.Write(ctx, state(), snapshot.ReasonSessionEnded); err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(time.Second)
	}
	w = f.writer(t)
	var last snapshot.Pointer
	for range 3 {
		p, err := w.Write(ctx, state(), snapshot.ReasonSessionEnded)
		if err != nil {
			t.Fatal(err)
		}
		last = p
		f.clock.Advance(time.Second)
	}
	keys := f.objectKeys(t)
	want := []string{
		"gateway/20260914T100002Z-000002.json", "gateway/20260914T100003Z-000003.json",
		"gateway/20260914T100004Z-000004.json", "gateway/20260914T100005Z-000005.json",
		"gateway/20260914T100006Z-000006.json",
	}
	if !slices.Equal(keys, want) {
		t.Errorf("objects left = %v, want %v", keys, want)
	}
	var pointer snapshot.Pointer
	f.get(t, snapshot.PointerKey, &pointer)
	if pointer.Snapshot.Seq != 6 || pointer.Snapshot.Key != want[4] || last.Snapshot.Key != want[4] {
		t.Errorf("latest.json = %+v, want seq 6 at %s", pointer.Snapshot, want[4])
	}
	if got, err := w.Latest(ctx); err != nil || got.Snapshot.ID != pointer.Snapshot.ID {
		t.Errorf("Latest = %+v %v", got.Snapshot, err)
	}
}

// Without the laws of the world nothing is written: snapshot.created requires
// laws_version, and a snapshot that cannot be announced is not taken.
func TestASnapshotWithoutLawsIsNotTaken(t *testing.T) {
	f := newFixture(t)
	st := state()
	st.LawsVersion = ""
	if _, err := f.writer(t).Write(context.Background(), st, snapshot.ReasonShutdown); !errors.Is(err, snapshot.ErrNoLawsVersion) {
		t.Fatalf("Write = %v, want ErrNoLawsVersion", err)
	}
	if keys := f.objectKeys(t); len(keys) != 0 {
		t.Errorf("objects written: %v", keys)
	}
	if len(f.bus.all()) != 0 {
		t.Error("an event was published")
	}
}

// A refused announcement fails the write after the files are in place, and the
// next write takes the next sequence rather than the key already written.
func TestARefusedAnnouncementFailsTheWriteAndKeepsTheFiles(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	w := f.writer(t)
	f.bus.fail = true
	p, err := w.Write(ctx, state(), snapshot.ReasonShutdown)
	if err == nil || !strings.Contains(err.Error(), "broker down") {
		t.Fatalf("Write = %v, want the refusal of the bus", err)
	}
	if _, err := w.Latest(ctx); err != nil {
		t.Errorf("the files of the refused announcement do not check: %v", err)
	}
	f.bus.fail = false
	next, err := w.Write(ctx, state(), snapshot.ReasonShutdown)
	if err != nil || next.Snapshot.Seq != p.Snapshot.Seq+1 {
		t.Errorf("next write = %+v %v, want seq %d", next.Snapshot, err, p.Snapshot.Seq+1)
	}
}

// Latest tells a new world from damage: no pointer is ErrNoSnapshot; a pointer
// that does not parse, that names no object, whose object is gone, is of
// another component or does not match it is ErrCorrupted (US-011).
func TestLatestTellsANewWorldFromDamage(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if _, err := f.writer(t).Latest(ctx); !errors.Is(err, snapshot.ErrNoSnapshot) {
		t.Fatalf("Latest of a new world = %v, want ErrNoSnapshot", err)
	}
	bucket := objstore.SnapshotsBucket(world)
	put := func(t *testing.T, s *objstore.Memory, key string, body []byte) {
		t.Helper()
		if _, err := s.Put(ctx, bucket, key, body, objstore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	damages := map[string]func(t *testing.T, f *fixture, p snapshot.Pointer){
		"pointer not json": func(t *testing.T, f *fixture, _ snapshot.Pointer) { put(t, f.store, snapshot.PointerKey, []byte("{")) },
		"object gone": func(t *testing.T, f *fixture, p snapshot.Pointer) {
			if err := f.store.Delete(ctx, bucket, p.Snapshot.Key); err != nil {
				t.Fatal(err)
			}
		},
		"object not json": func(t *testing.T, f *fixture, p snapshot.Pointer) { put(t, f.store, p.Snapshot.Key, []byte("[")) },
		"another component": func(t *testing.T, f *fixture, p snapshot.Pointer) {
			p.Component = "state"
			body, _ := json.Marshal(p)
			put(t, f.store, snapshot.PointerKey, body)
		},
		"key not of its time": func(t *testing.T, f *fixture, p snapshot.Pointer) {
			p.Snapshot.TakenAt = p.Snapshot.TakenAt.Add(time.Hour)
			body, _ := json.Marshal(p)
			put(t, f.store, snapshot.PointerKey, body)
		},
		"hash of another object": func(t *testing.T, f *fixture, p snapshot.Pointer) {
			var object snapshot.Object
			f.get(t, p.Snapshot.Key, &object)
			object.ProjectionHash = "sha256:" + strings.Repeat("0", 64)
			body, _ := json.MarshalIndent(object, "", "  ")
			put(t, f.store, p.Snapshot.Key, body)
		},
	}
	for name, damage := range damages {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			w := f.writer(t)
			p, err := w.Write(ctx, state(), snapshot.ReasonShutdown)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Latest(ctx); err != nil {
				t.Fatalf("control: the intact snapshot = %v", err)
			}
			damage(t, f, p)
			if _, err := w.Latest(ctx); !errors.Is(err, snapshot.ErrCorrupted) {
				t.Errorf("Latest = %v, want ErrCorrupted", err)
			}
		})
	}
}

func TestNewChecksItsConfig(t *testing.T) {
	for i, cfg := range []snapshot.Config{
		{WorldID: world, Clock: clock.NewManual(t0)},
		{Objects: objstore.NewMemory(), Clock: clock.NewManual(t0)},
		{Objects: objstore.NewMemory(), WorldID: world},
	} {
		if _, err := snapshot.New(cfg); err == nil {
			t.Errorf("config %d accepted", i)
		}
	}
	if got := snapshot.Key(t0, 42); got != fmt.Sprintf("gateway/20260914T100000Z-%06d.json", 42) {
		t.Errorf("Key = %s", got)
	}
}
