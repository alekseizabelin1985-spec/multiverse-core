package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/snapshot"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// hookedStore runs onPut before every write of a snapshot of the gateway.
type hookedStore struct {
	objstore.Client
	onPut func(ctx context.Context, key string) error
}

func (h *hookedStore) Put(ctx context.Context, bucket, key string, body []byte, opts objstore.PutOptions) (string, error) {
	if h.onPut != nil && strings.HasPrefix(key, snapshot.Component+"/") {
		if err := h.onPut(ctx, key); err != nil {
			return "", err
		}
	}
	return h.Client.Put(ctx, bucket, key, body, opts)
}

// countingBus counts the subscriptions that run.
type countingBus struct {
	eventbus.Bus
	active atomic.Int32
}

func (b *countingBus) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	b.active.Add(1)
	defer b.active.Add(-1)
	return b.Bus.Subscribe(ctx, topic, group, h)
}

func gatewayPointer(t *testing.T, store objstore.Client) (snapshot.Pointer, bool) {
	t.Helper()
	body, err := store.Get(context.Background(), objstore.SnapshotsBucket(world), snapshot.PointerKey)
	if errors.Is(err, objstore.ErrNotFound) || errors.Is(err, objstore.ErrNoBucket) {
		return snapshot.Pointer{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	var p snapshot.Pointer
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatal(err)
	}
	return p, true
}

func gatewayObject(t *testing.T, store objstore.Client, key string) snapshot.Object {
	t.Helper()
	body, err := store.Get(context.Background(), objstore.SnapshotsBucket(world), key)
	if err != nil {
		t.Fatal(err)
	}
	var o snapshot.Object
	if err := json.Unmarshal(body, &o); err != nil {
		t.Fatal(err)
	}
	return o
}

func snapshotEvents(t *testing.T, bus *membus.Bus) []eventbus.Event {
	t.Helper()
	records, err := bus.Records(eventbus.TopicSystemEvents)
	if err != nil {
		t.Fatal(err)
	}
	var out []eventbus.Event
	for _, raw := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type == snapshot.TypeCreated {
			out = append(out, ev)
		}
	}
	return out
}

// Stop writes the snapshot of the shutdown after the subscriptions and the
// sweeper stopped, and returns only once the object, the pointer and
// snapshot.created are written; the event is valid by its schema and a second
// Stop writes nothing more (component §11.2, reconciliation of 2026-09-13).
func TestTheShutdownSnapshotIsWrittenAfterEverythingStopped(t *testing.T) {
	testkit.Deterministic(t, "gw")
	timers := &countingTimers{}
	var bus *countingBus
	var mu sync.Mutex
	var seen []string
	objects := &hookedStore{Client: snapshotStore(t, nil)}
	objects.onPut = func(_ context.Context, key string) error {
		_, every, stopped := timers.counts()
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, fmt.Sprintf("%s subscriptions=%d tickers=%d stopped=%d", key, bus.active.Load(), every, stopped))
		return nil
	}
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{
		objects: objects,
		bus:     func(b *membus.Bus) eventbus.Bus { bus = &countingBus{Bus: b}; return bus },
		timers:  func(inner clock.Timers) clock.Timers { timers.Timers = inner; return timers },
	})
	eventually(t, "the four subscriptions to run", func() bool { return bus.active.Load() == 4 })
	eventually(t, "the sweeper to arm its tickers", func() bool { _, every, _ := timers.counts(); return every == 3 })
	hash := gateway.ReadModel(r.ctx).Hash()

	if err := r.ctx.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	mu.Lock()
	writes := append([]string(nil), seen...)
	mu.Unlock()
	if len(writes) != 2 {
		t.Fatalf("writes of the shutdown = %v, want the object and the pointer", writes)
	}
	for _, w := range writes {
		if !strings.HasSuffix(w, "subscriptions=0 tickers=3 stopped=3") {
			t.Errorf("written while the gateway still ran: %s", w)
		}
	}
	p, ok := gatewayPointer(t, objects)
	if !ok || p.Snapshot.Reason != snapshot.ReasonShutdown || p.Snapshot.StateHash != hash {
		t.Fatalf("pointer after Stop = %+v %v, want the shutdown with the hash of the projection", p, ok)
	}
	events := snapshotEvents(t, r.bus)
	if len(events) != 1 {
		t.Fatalf("snapshot.created after Stop: %d, want 1", len(events))
	}
	if err := contracts.Default().Validate(events[0]); err != nil {
		t.Errorf("snapshot.created is not valid: %v", err)
	}
	if key, _ := events[0].Path().GetString("snapshot.key"); key != p.Snapshot.Key {
		t.Errorf("snapshot.created names %s, the pointer %s", key, p.Snapshot.Key)
	}

	if err := r.ctx.Stop(context.Background()); err != nil {
		t.Errorf("a second Stop = %v", err)
	}
	if n := len(snapshotEvents(t, r.bus)); n != 1 {
		t.Errorf("a second Stop announced another snapshot: %d", n)
	}
}

// Stop keeps within the deadline it is given: a store that does not answer
// the write of the snapshot costs half of what was left of the deadline and a
// warning, not an error of Stop, and the databases are closed. The wipe a
// blocked /forget left is finished before the snapshot, so the hanging store
// does not leave the external ID in links.db (Mi-2 of review #1 of T-309,
// ADR-019 addendum p. 1; acceptance of T-309, component §11.2).
func TestAStoreThatDoesNotAnswerDoesNotHoldTheStop(t *testing.T) {
	const deadline = 2 * time.Second
	var (
		mu      sync.Mutex
		writeBy time.Time
		cut     time.Time
	)
	objects := &hookedStore{Client: snapshotStore(t, nil), onPut: func(ctx context.Context, _ string) error {
		by, _ := ctx.Deadline()
		<-ctx.Done()
		mu.Lock()
		writeBy, cut = by, clock.Real{}.Now()
		mu.Unlock()
		return ctx.Err()
	}}
	logs := &syncLog{}
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{objects: objects, log: logs})
	consent(t, r.client, externalID)
	if err := gateway.SetLinksBusyTimeout(r.ctx, 0); err != nil {
		t.Fatal(err)
	}
	leave := holdReader(t, r.dir)
	if code := forgetRaw(t, r.client.BaseURL, externalID).StatusCode; code != http.StatusServiceUnavailable {
		t.Fatalf("/forget under a reader = %d, want 503 with the compaction pending", code)
	}
	leave()
	if !gateway.CompactionPending(r.ctx) || occurrences(t, r.dir, externalID) == 0 {
		t.Fatal("control: no wipe is pending, Stop has nothing to finish")
	}
	began := clock.Real{}.Now()
	ctx, cancel := context.WithDeadline(context.Background(), began.Add(deadline))
	defer cancel()
	err := r.ctx.Stop(ctx)
	if took := (clock.Real{}).Now().Sub(began); took > deadline {
		t.Errorf("Stop took %v with a deadline of %v", took, deadline)
	}
	if err != nil {
		t.Errorf("Stop = %v, want a snapshot not written to be a warning only", err)
	}
	mu.Lock()
	by, at := writeBy, cut
	mu.Unlock()
	if at.IsZero() {
		t.Fatal("control: the snapshot of the shutdown did not reach the store")
	}
	// Stop takes half of its deadline from its own first line, a moment after
	// began: by is began + deadline/2 plus half of that moment. The margin
	// keeps the check off the resolution of the monotonic clock, which is a
	// tick on Windows and nanoseconds on Linux (Ma-1 of review #1 of T-473);
	// the whole rest of the deadline (began + deadline) is still far outside.
	const margin = 50 * time.Millisecond
	if by.Sub(began) > deadline/2+margin || at.Sub(began) > deadline/2+deadline/4 {
		t.Errorf("the write of the snapshot had until %v and was cut %v after Stop began, want half of the %v of Stop",
			by.Sub(began), at.Sub(began), deadline)
	}
	if !strings.Contains(logs.String(), `"level":"WARN","msg":"snapshot at the shutdown not written"`) {
		t.Errorf("no warning of the snapshot not written in the log:\n%s", logs.String())
	}
	if !gateway.DatabasesClosed(r.ctx) {
		t.Error("Stop left the databases open")
	}
	if gateway.CompactionPending(r.ctx) {
		t.Error("the hanging store left the wipe of /forget unfinished")
	}
	if n := occurrences(t, r.dir, externalID); n != 0 {
		t.Errorf("the forgotten ID occurs %d times after Stop", n)
	}
}

// A session that ends takes a snapshot: the idle session the restoration ends
// at the start is one (component §11.2, §7.6).
func TestASessionThatEndsTakesASnapshot(t *testing.T) {
	testkit.Deterministic(t, "gw")
	idleID, idleSQL := sessionRow("player-A", t0.Add(-50*time.Minute), t0.Add(-45*time.Minute))
	_, freshSQL := sessionRow("player-B", t0.Add(-10*time.Minute), t0.Add(-5*time.Minute))
	objects := snapshotStore(t, nil)
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{
		objects: objects,
		prepare: func(dir string) { seedGateway(t, dir, idleSQL, freshSQL) },
	})
	var p snapshot.Pointer
	eventually(t, "the snapshot after the end of the idle session", func() bool {
		var ok bool
		p, ok = gatewayPointer(t, objects)
		return ok && len(snapshotEvents(t, r.bus)) > 0
	})
	if p.Snapshot.Reason != snapshot.ReasonSessionEnded {
		t.Errorf("reason = %s, want %s", p.Snapshot.Reason, snapshot.ReasonSessionEnded)
	}
	object := gatewayObject(t, objects, p.Snapshot.Key)
	if len(object.ActiveSessions) != 1 || object.ActiveSessions[0].ID == idleID || object.LawsVersion != "v1" {
		t.Errorf("object = %+v, want the session that goes on and not the idle one", object)
	}
	if n := len(snapshotEvents(t, r.bus)); n != 1 {
		t.Errorf("snapshot.created: %d, want 1", n)
	}
}

// The projection_hash of the snapshot of the gateway is the state_hash State
// reaches on the same facts, the removal of a key included — on the facts of
// C-02 v1.6 only (review #1 of T-304, Mi-1: under v1.5 a removal and a set to
// null are one form, and the hashes part).
func TestTheProjectionHashOfTheSnapshotIsTheStateHashOnFactsOfV16(t *testing.T) {
	testkit.Deterministic(t, "gw")
	objects := snapshotStore(t, nil)
	r := startWith(t, runtime.ModeLive, sqlitedir.Temp(t), objects)

	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "snapshots", "state", "20260101T000000Z-000000.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Entities []*entity.Entity `json:"entities"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	var truth *entity.Entity
	for _, e := range fixture.Entities {
		if e.ID == "player-A" {
			truth = e
		}
	}
	if truth == nil {
		t.Fatal("the fixture has no player-A")
	}
	for i, op := range []entity.Op{
		{Op: entity.OpSet, Path: "encounter_id", Value: "enc-1"},
		{Op: entity.OpInc, Path: "hp", Value: -3},
		{Op: entity.OpRemove, Path: "encounter_id"},
	} {
		before := entity.Clone(truth)
		attrs, changed, err := entity.ApplyOps(truth, []entity.Op{op})
		if err != nil {
			t.Fatal(err)
		}
		truth.Commit(attrs, changed, entity.LastChange{ProposalID: fmt.Sprint(i), AppliedAt: t0})
		wire := make([]any, 0, len(changed))
		for _, c := range changed {
			element := map[string]any{"path": c.Path}
			if before.HasAttr(c.Path) {
				element["old"] = c.Old
			}
			if truth.HasAttr(c.Path) {
				element["new"] = c.New
			}
			wire = append(wire, element)
		}
		body, _ := json.Marshal(map[string]any{
			"entity":  map[string]any{"entity": map[string]any{"id": truth.ID, "type": truth.Type}, "name": truth.Name},
			"version": truth.Version, "cause": "combat", "proposal_id": fmt.Sprint("p-", i), "applied_at": t0.Format(time.RFC3339),
			"changed": wire,
		})
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if err := r.bus.Publish(context.Background(), eventbus.NewRoot(readmodel.TypeEntityUpdated, contracts.SourceState, world, nil,
			eventbus.ActorSystem, payload)); err != nil {
			t.Fatal(err)
		}
	}
	model := gateway.ReadModel(r.ctx)
	eventually(t, "the facts to reach the projection", func() bool { v, _ := model.Version("player-A"); return v == truth.Version })
	for i, e := range fixture.Entities {
		if e.ID == truth.ID {
			fixture.Entities[i] = truth
		}
	}
	state := entity.StateHash(fixture.Entities)

	if err := r.ctx.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	p, ok := gatewayPointer(t, objects)
	if !ok {
		t.Fatal("no snapshot after Stop")
	}
	object := gatewayObject(t, objects, p.Snapshot.Key)
	if object.ProjectionHash != state || p.Snapshot.StateHash != state {
		t.Errorf("projection_hash %s (pointer %s), State reaches %s", object.ProjectionHash, p.Snapshot.StateHash, state)
	}
	// The three facts are offsets 0, 1 and 2 of system_events: both cursors
	// name offset 3, the next event neither has taken.
	if object.Cursors.Projection[eventbus.TopicSystemEvents] != 3 || object.Cursors.Effects[eventbus.TopicSystemEvents] != 3 ||
		p.Snapshot.Cursor[eventbus.TopicSystemEvents] != 3 {
		t.Errorf("cursors = %+v, pointer %v; want 3 for system_events", object.Cursors, p.Snapshot.Cursor)
	}
}

// A snapshot of the gateway that does not check is not passed over in
// silence: the gateway starts from gateway.db, its truth, and /health says
// snapshot corrupted, degraded (US-011).
func TestACorruptedSnapshotOfTheGatewayIsReported(t *testing.T) {
	for name, damage := range map[string]func(t *testing.T, s *objstore.Memory){
		"pointer not json": func(t *testing.T, s *objstore.Memory) {
			if _, err := s.Put(context.Background(), objstore.SnapshotsBucket(world), snapshot.PointerKey, []byte("{"), objstore.PutOptions{}); err != nil {
				t.Fatal(err)
			}
		},
		"object gone": func(t *testing.T, s *objstore.Memory) {
			w, err := snapshot.New(snapshot.Config{Objects: s, WorldID: world, Clock: clock.NewManual(t0)})
			if err != nil {
				t.Fatal(err)
			}
			p, err := w.Write(context.Background(), snapshot.State{ProjectionHash: entity.StateHash(nil), LawsVersion: "v1"}, snapshot.ReasonShutdown)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Delete(context.Background(), objstore.SnapshotsBucket(world), p.Snapshot.Key); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			objects := snapshotStore(t, nil)
			damage(t, objects)
			r := startWith(t, runtime.ModeLive, sqlitedir.Temp(t), objects)
			if h := r.ctx.Health(); h.Status != runtime.StatusDegraded || h.Details["snapshot"] != "corrupted" {
				t.Errorf("Health = %+v, want degraded with snapshot corrupted", h)
			}
		})
	}
	t.Run("intact", func(t *testing.T) {
		objects := snapshotStore(t, nil)
		w, err := snapshot.New(snapshot.Config{Objects: objects, WorldID: world, Clock: clock.NewManual(t0)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(context.Background(), snapshot.State{ProjectionHash: entity.StateHash(nil), LawsVersion: "v1"},
			snapshot.ReasonShutdown); err != nil {
			t.Fatal(err)
		}
		r := startWith(t, runtime.ModeLive, sqlitedir.Temp(t), objects)
		if h := r.ctx.Health(); h.Status != runtime.StatusOK || h.Details["snapshot"] != nil {
			t.Errorf("Health with an intact snapshot = %+v, want ok", h)
		}
	})
}

// gatewayKeys lists the keys of the gateway in the bucket of the world.
func gatewayKeys(t *testing.T, objects objstore.Client) []string {
	t.Helper()
	infos, err := objects.List(context.Background(), objstore.SnapshotsBucket(world), snapshot.Component+"/")
	if errors.Is(err, objstore.ErrNoBucket) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, info := range infos {
		keys = append(keys, info.Key)
	}
	return keys
}

// syncLog is a log the goroutines of the context write while a test reads it.
type syncLog struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (l *syncLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *syncLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// A replay takes no snapshot of the gateway and announces none: not after a
// session ended, not at Stop (component §11.2, NFR-061). The check of the last
// snapshot at the start still runs: a corrupted one is reported.
func TestReplayTakesNoSnapshotOfTheGateway(t *testing.T) {
	testkit.Deterministic(t, "gw")
	idleID, idleSQL := sessionRow("player-A", t0.Add(-50*time.Minute), t0.Add(-45*time.Minute))
	_, freshSQL := sessionRow("player-B", t0.Add(-10*time.Minute), t0.Add(-5*time.Minute))
	objects := snapshotStore(t, nil)
	r := startOpts(t, runtime.ModeReplay, sqlitedir.Temp(t), options{
		objects: objects,
		prepare: func(dir string) { seedGateway(t, dir, idleSQL, freshSQL) },
	})
	gatewayDB, err := store.OpenGateway(context.Background(), store.GatewayPath(r.dir))
	if err != nil {
		t.Fatal(err)
	}
	var state string
	err = gatewayDB.QueryRowContext(context.Background(), `SELECT state FROM sessions WHERE id = ?`, idleID).Scan(&state)
	_ = gatewayDB.Close()
	if err != nil || state != "ended" {
		t.Fatalf("control: the idle session is %s %v, want ended at the start", state, err)
	}
	if err := r.ctx.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if keys := gatewayKeys(t, objects); len(keys) != 0 {
		t.Errorf("replay wrote %v", keys)
	}
	if n := len(snapshotEvents(t, r.bus)); n != 0 {
		t.Errorf("replay announced %d snapshots of the gateway", n)
	}

	t.Run("corrupted at the start", func(t *testing.T) {
		damaged := snapshotStore(t, nil)
		if _, err := damaged.Put(context.Background(), objstore.SnapshotsBucket(world), snapshot.PointerKey, []byte("{"), objstore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		r := startWith(t, runtime.ModeReplay, sqlitedir.Temp(t), damaged)
		if h := r.ctx.Health(); h.Status != runtime.StatusDegraded || h.Details["snapshot"] != "corrupted" {
			t.Errorf("Health = %+v, want snapshot corrupted", h)
		}
	})
}

// endLaterSession is a session of scope the sweeper ends a minute after t0.
func endLaterSession(scope string) (string, string) {
	return sessionRow(scope, t0.Add(-40*time.Minute), t0.Add(-29*time.Minute-30*time.Second))
}

// sweepUntil advances the clock of a live context minute by minute until cond
// holds: the sweeper arms its tickers in a goroutine of its own.
func sweepUntil(t *testing.T, r running, what string, cond func() bool) {
	t.Helper()
	for range 200 {
		if cond() {
			return
		}
		r.clock.Advance(time.Minute)
		runtimeYield()
	}
	t.Fatalf("%s: not reached", what)
}

func sessionEnded(t *testing.T, dir, id string) bool {
	t.Helper()
	db, err := store.OpenGateway(context.Background(), store.GatewayPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var state string
	if err := db.QueryRowContext(context.Background(), `SELECT state FROM sessions WHERE id = ?`, id).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state == "ended"
}

// A world the projection knows without laws_version is a defect of the data:
// the snapshot after a session is not taken, and /health says no_laws_version,
// degraded. A world the projection does not know is a new world: the snapshot
// is skipped with a warning and /health stays ok (component §11.2).
func TestASnapshotWithoutLawsVersionIsReportedOnlyForAKnownWorld(t *testing.T) {
	for name, known := range map[string]bool{"known world without laws": true, "unknown world": false} {
		t.Run(name, func(t *testing.T) {
			testkit.Deterministic(t, "gw")
			id, row := endLaterSession("player-A")
			log := &syncLog{}
			objects := objstore.NewMemory()
			r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{
				objects: objects, log: log,
				prepare: func(dir string) { seedGateway(t, dir, row) },
			})
			if known {
				if _, err := gateway.ReadModel(r.ctx).Apply(created(t, world, entity.TypeWorld, "Тёмный лес", map[string]any{"locale": "ru"})); err != nil {
					t.Fatal(err)
				}
			}
			sweepUntil(t, r, "the session to end", func() bool { return sessionEnded(t, r.dir, id) })
			want := "does not know the world yet"
			if known {
				want = "has no laws_version"
			}
			eventually(t, "the snapshot after the session to be skipped", func() bool { return strings.Contains(log.String(), want) })
			h := r.ctx.Health()
			switch {
			case known && (h.Status != runtime.StatusDegraded || h.Details["snapshot"] != "no_laws_version"):
				t.Errorf("Health = %+v, want degraded with snapshot no_laws_version", h)
			case !known && (h.Status != runtime.StatusOK || h.Details["snapshot"] != nil):
				t.Errorf("Health = %+v, want ok without snapshot", h)
			}
			if keys := gatewayKeys(t, objects); len(keys) != 0 {
				t.Errorf("a snapshot was written: %v", keys)
			}
		})
	}
}

// Ends of sessions that come while a snapshot is written make exactly one more
// snapshot, and it records them all: the ended sessions are not among its
// active sessions (component §11.2, question 8 of architect#3).
func TestEndsDuringAWriteMakeOneMoreSnapshot(t *testing.T) {
	testkit.Deterministic(t, "gw")
	idleA, rowA := sessionRow("player-A", t0.Add(-50*time.Minute), t0.Add(-45*time.Minute))
	idB, rowB := endLaterSession("player-B")
	idC, rowC := endLaterSession("player-C")
	idD, rowD := sessionRow("player-D", t0.Add(-5*time.Minute), t0)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	objects := &hookedStore{Client: snapshotStore(t, nil)}
	objects.onPut = func(_ context.Context, key string) error {
		if key == snapshot.PointerKey {
			return nil
		}
		held := false
		once.Do(func() { held = true })
		if held {
			close(entered)
			<-release
		}
		return nil
	}
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{
		objects: objects,
		prepare: func(dir string) { seedGateway(t, dir, rowA, rowB, rowC, rowD) },
	})
	select {
	case <-entered:
	case <-testkit.After(15 * time.Second):
		t.Fatal("the first snapshot did not begin")
	}
	if !sessionEnded(t, r.dir, idleA) {
		t.Fatal("control: the write began before the end of A")
	}
	sweepUntil(t, r, "B and C to end while the first snapshot is written", func() bool {
		return sessionEnded(t, r.dir, idB) && sessionEnded(t, r.dir, idC)
	})
	close(release)
	eventually(t, "the second snapshot", func() bool {
		p, ok := gatewayPointer(t, objects)
		return ok && p.Snapshot.Seq == 1 && len(snapshotEvents(t, r.bus)) == 2
	})
	for range 20 {
		runtimeYield()
	}
	keys := gatewayKeys(t, objects)
	if len(keys) != 3 {
		t.Fatalf("keys = %v, want two objects and the pointer", keys)
	}
	p, _ := gatewayPointer(t, objects)
	second := gatewayObject(t, objects, p.Snapshot.Key)
	if len(second.ActiveSessions) != 1 || second.ActiveSessions[0].ID != idD {
		t.Errorf("active sessions of the second snapshot = %+v, want only %s", second.ActiveSessions, idD)
	}
}

// A snapshot after a session is bounded by its budget: a store that hangs makes
// /health say write_failed instead of holding the goroutine of the snapshots
// until Stop (N-4 of review #1 of T-309).
func TestAHangingStoreFailsTheSnapshotWithinItsBudget(t *testing.T) {
	testkit.Deterministic(t, "gw")
	_, row := sessionRow("player-A", t0.Add(-50*time.Minute), t0.Add(-45*time.Minute))
	objects := &hookedStore{Client: snapshotStore(t, nil), onPut: func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{
		objects: objects, snapshotBudget: 200 * time.Millisecond,
		prepare: func(dir string) { seedGateway(t, dir, row) },
	})
	eventually(t, "the snapshot after the idle session to fail", func() bool {
		return r.ctx.Health().Details["snapshot"] == "write_failed"
	})
	if h := r.ctx.Health(); h.Status != runtime.StatusDegraded {
		t.Errorf("Health = %+v, want degraded", h)
	}
}
