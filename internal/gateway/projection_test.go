package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

const world = "dark-forest-world"

// snapshotStore holds the snapshot of State of the fixtures, optionally with
// its object replaced.
func snapshotStore(t *testing.T, object []byte) *objstore.Memory {
	t.Helper()
	ctx := context.Background()
	dir := filepath.Join("..", "..", "testdata", "fixtures", "snapshots", "state")
	store := objstore.NewMemory()
	bucket := objstore.SnapshotsBucket(world)
	if err := store.EnsureBucket(ctx, bucket, objstore.BucketOptionsFor(bucket)); err != nil {
		t.Fatal(err)
	}
	const key = "state/20260101T000000Z-000000.json"
	for name, stored := range map[string]string{"latest.json": readmodel.StatePointerKey, "20260101T000000Z-000000.json": key} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if stored == key && object != nil {
			body = object
		}
		if _, err := store.Put(ctx, bucket, stored, body, objstore.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

// The projection the gateway reports in /health: ok when the snapshot of State
// loaded, missing without a snapshot — the start of a new world, which does
// not degrade the gateway — and missing with the reason, degraded, when a
// snapshot is there and does not load (US-011, component §11.4). The reason is
// a code: the text of the error names the bucket and the key of the store.
func TestTheHealthOfTheGatewayReportsItsProjection(t *testing.T) {
	cases := []struct {
		name       string
		objects    objstore.Client
		projection string
		status     string
		reason     string
	}{
		{"snapshot", snapshotStore(t, nil), "ok", runtime.StatusOK, ""},
		{"no store", nil, "missing", runtime.StatusOK, ""},
		{"no snapshot yet", objstore.NewMemory(), "missing", runtime.StatusOK, ""},
		{"broken snapshot", snapshotStore(t, []byte("{")), "missing", runtime.StatusDegraded, readmodel.ReasonSnapshotUnreadable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := startWith(t, runtime.ModeLive, sqlitedir.Temp(t), tc.objects)
			h := r.ctx.Health()
			reason, hasReason := h.Details["projection_error"]
			if h.Status != tc.status || h.Details["projection"] != tc.projection || hasReason != (tc.reason != "") ||
				(hasReason && reason != tc.reason) {
				t.Errorf("Health = %+v, want %s with projection %s (reason %q)", h, tc.status, tc.projection, tc.reason)
			}
		})
	}
}

// An object store configured wrong is not a new world: the gateway does not
// report the snapshot missing in silence but degraded, with a reason that
// names neither the keys nor the endpoint (US-011; review #1, Mi-6).
func TestAStoreConfiguredWrongIsNotANewWorld(t *testing.T) {
	const access, secret = "gateway-access-key-under-test", "gateway-secret-key-under-test"
	cases := map[string]map[string]string{
		"ssl flag is not a boolean": {env.MinIOAccessKey.Name(): access, env.MinIOSecretKey.Name(): secret, env.MinIOUseSSL.Name(): "maybe"},
		"access key alone":          {env.MinIOAccessKey.Name(): access},
		"secret key alone":          {env.MinIOSecretKey.Name(): secret},
		"empty endpoint":            {env.MinIOAccessKey.Name(): access, env.MinIOSecretKey.Name(): secret, env.MinIOEndpoint.Name(): ""},
	}
	for name, vars := range cases {
		t.Run(name, func(t *testing.T) {
			r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{vars: vars})
			h := r.ctx.Health()
			if h.Status != runtime.StatusDegraded || h.Details["projection"] != "missing" ||
				h.Details["projection_error"] != readmodel.ReasonStoreMisconfigured {
				t.Errorf("Health = %+v, want degraded, missing, %s", h, readmodel.ReasonStoreMisconfigured)
			}
			for _, value := range []string{access, secret} {
				for _, v := range h.Details {
					if s, ok := v.(string); ok && strings.Contains(s, value) {
						t.Errorf("Health carries a key: %+v", h.Details)
					}
				}
			}
		})
	}
}

// silentStore accepts a read and never answers, like a store behind a network
// that dropped it.
type silentStore struct{ objstore.Client }

func (silentStore) Get(ctx context.Context, _, _ string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// silentJournal is a broker that accepts the connection and stays silent: the
// kafka adapter bounds such a read by the deadline of its context alone.
type silentJournal struct{ eventbus.Journal }

func (silentJournal) End(ctx context.Context, _ string) (int64, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}

// startWithin runs Start and fails the test if it does not return within a
// few seconds: without the budgets a silent source holds it forever.
func startWithin(t *testing.T, r running, deps runtime.Deps) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- r.ctx.Start(context.Background(), deps) }()
	select {
	case err := <-done:
		if err == nil {
			t.Cleanup(func() { _ = r.ctx.Stop(context.Background()) })
		}
		return err
	case <-testkit.After(5 * time.Second):
		t.Fatal("Start did not return within its budget")
		return nil
	}
}

// A store that never answers does not hold the start: past the budget of the
// load the projection is missing with a timeout, the gateway degraded, and it
// goes on from the journal (review #1, Mi-7).
func TestASilentStoreDoesNotHoldTheStart(t *testing.T) {
	r, _, deps := build(t, runtime.ModeLive, sqlitedir.Temp(t), options{objects: silentStore{}, budget: 50 * time.Millisecond})
	if err := startWithin(t, r, deps); err != nil {
		t.Fatalf("Start = %v, want a start without the snapshot", err)
	}
	h := r.ctx.Health()
	if h.Status != runtime.StatusDegraded || h.Details["projection"] != "missing" ||
		h.Details["projection_error"] != readmodel.ReasonSnapshotTimeout {
		t.Errorf("Health = %+v, want degraded, missing, %s", h, readmodel.ReasonSnapshotTimeout)
	}
}

// A journal that never answers fails the start within the budget of the
// catch-up, and says so; the databases are closed behind it (review #1, Mi-7).
func TestASilentJournalFailsTheStartWithinItsBudget(t *testing.T) {
	dir := sqlitedir.Temp(t)
	r, _, deps := build(t, runtime.ModeLive, dir, options{
		budget:  50 * time.Millisecond,
		journal: func(bus *membus.Bus) eventbus.Journal { return silentJournal{bus} },
	})
	err := startWithin(t, r, deps)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "catch-up") {
		t.Fatalf("Start = %v, want the catch-up past its budget", err)
	}
	if h := r.ctx.Health(); h.Status != runtime.StatusFail {
		t.Errorf("Health of a context that did not start = %+v", h)
	}
	// A failed start leaves no file open: the directory can be started again.
	again := startIn(t, runtime.ModeLive, dir)
	if h := again.ctx.Health(); h.Status != runtime.StatusOK {
		t.Errorf("Health after a start on the same directory = %+v", h)
	}
}

// deafBus refuses the subscription to one topic and serves the others.
type deafBus struct {
	eventbus.Bus
	topic string
}

func (b deafBus) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	if topic == b.topic {
		return errors.New("broker gone")
	}
	return b.Bus.Subscribe(ctx, topic, group, h)
}

// A subscription that ended with an error is the only sign that the gateway
// no longer hears a topic: /health says bus fail and degraded (review #1, Mi-3).
func TestAFailedSubscriptionDegradesTheGateway(t *testing.T) {
	r := startOpts(t, runtime.ModeLive, sqlitedir.Temp(t), options{
		bus: func(bus *membus.Bus) eventbus.Bus { return deafBus{Bus: bus, topic: eventbus.TopicWorldEvents} },
	})
	deadline := testkit.After(15 * time.Second)
	for {
		h := r.ctx.Health()
		if h.Details["bus"] == runtime.StatusFail {
			if h.Status != runtime.StatusDegraded {
				t.Errorf("Health with a failed subscription = %+v, want degraded", h)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("the failed subscription is not in /health: %+v", h)
		case <-testkit.After(2 * time.Millisecond):
		}
	}
}

// In live mode the sweeper forgets the marks of processed events past their
// retention; in replay mode no timer of the gateway runs (review #1, Mi-5).
func TestTheSweeperForgetsOldMarksOfProcessedEvents(t *testing.T) {
	for _, mode := range []runtime.Mode{runtime.ModeLive, runtime.ModeReplay} {
		t.Run(string(mode), func(t *testing.T) {
			r := startIn(t, mode, sqlitedir.Temp(t))
			ctx := context.Background()
			db, err := store.OpenGateway(ctx, store.GatewayPath(r.dir))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()
			old := t0.Add(-store.ProcessedEventsKeep - time.Hour).UTC().Format("2006-01-02T15:04:05.000000000Z")
			if _, err := db.ExecContext(ctx, `INSERT INTO processed_events (event_id, topic, processed_at) VALUES ('old', ?, ?)`,
				eventbus.TopicSystemEvents, old); err != nil {
				t.Fatal(err)
			}
			count := func() int {
				var n int
				if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM processed_events`).Scan(&n); err != nil {
					t.Fatal(err)
				}
				return n
			}
			// The sweeper registers its tickers in its own goroutine: advance
			// until the tick lands, within a bound.
			deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			for count() != 0 && deadline.Err() == nil {
				r.clock.Advance(store.SweepInterval)
				time.Sleep(20 * time.Millisecond)
				if mode == runtime.ModeReplay && r.clock.Now().After(t0.Add(90*time.Minute)) {
					break
				}
			}
			switch n := count(); {
			case mode == runtime.ModeLive && n != 0:
				t.Errorf("live: %d old marks left after the sweeps", n)
			case mode == runtime.ModeReplay && n != 1:
				t.Errorf("replay: %d marks left, want the one untouched", n)
			}
		})
	}
}

// The started gateway is subscribed: a fact published on the bus reaches its
// projection, and after Stop nothing does.
func TestTheGatewayHearsTheFactsOfTheBus(t *testing.T) {
	testkit.Deterministic(t, "gw")
	r := startWith(t, runtime.ModeLive, sqlitedir.Temp(t), snapshotStore(t, nil))
	model := gateway.ReadModel(r.ctx)
	if c, ok := model.Character("player-A"); !ok || c.HP != 10 {
		t.Fatalf("player-A from the snapshot = %+v %v", c, ok)
	}
	hit := func(version, from, to int) eventbus.Event {
		return eventbus.NewRoot(readmodel.TypeEntityUpdated, contracts.SourceState, world, nil, eventbus.ActorSystem,
			map[string]any{
				"entity":  map[string]any{"entity": map[string]any{"id": "player-A", "type": "player"}, "name": "Вася"},
				"version": version, "cause": "combat", "proposal_id": "p", "applied_at": "2026-09-13T10:00:00Z",
				"changed": []any{map[string]any{"path": "hp", "old": from, "new": to}},
			})
	}
	if err := r.bus.Publish(context.Background(), hit(2, 10, 6)); err != nil {
		t.Fatal(err)
	}
	deadline := testkit.After(15 * time.Second)
	for {
		if c, _ := model.Character("player-A"); c.HP == 6 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("the fact did not reach the projection")
		case <-testkit.After(2 * time.Millisecond):
		}
	}
	if h := r.ctx.Health(); h.Status != runtime.StatusOK || h.Details["projection"] != "ok" {
		t.Errorf("Health after the fact = %+v", h)
	}

	// Version 3 never comes: the fact of version 4 leaves the projection
	// behind State, and the gateway says so.
	if err := r.bus.Publish(context.Background(), hit(4, 6, 5)); err != nil {
		t.Fatal(err)
	}
	for {
		if h := r.ctx.Health(); h.Details["projection"] == "stale" {
			if h.Status != runtime.StatusDegraded {
				t.Errorf("Health with a stale projection = %+v, want degraded", h)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("the gap of versions did not make the projection stale")
		case <-testkit.After(2 * time.Millisecond):
		}
	}

	if err := r.ctx.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.bus.Publish(context.Background(), hit(5, 5, 2)); err != nil {
		t.Fatal(err)
	}
	<-testkit.After(50 * time.Millisecond)
	if c, _ := model.Character("player-A"); c.HP != 5 {
		t.Errorf("a fact published after Stop reached the projection: hp %d", c.HP)
	}
}

// hpFact is entity.updated of the hp of player-A at version.
func hpFact(version, from, to int) eventbus.Event {
	return eventbus.NewRoot(readmodel.TypeEntityUpdated, contracts.SourceState, world, nil, eventbus.ActorSystem,
		map[string]any{
			"entity":  map[string]any{"entity": map[string]any{"id": "player-A", "type": "player"}, "name": "Вася"},
			"version": version, "cause": "combat", "proposal_id": fmt.Sprintf("p-hp-%d", version), "applied_at": "2026-09-13T10:00:00Z",
			"changed": []any{map[string]any{"path": "hp", "old": from, "new": to}},
		})
}

// laterStateSnapshot writes into objects a later snapshot object of State in
// which player-A is at version with hp, and returns snapshot.created of it with
// hash as its state_hash; an empty hash is the hash of the object.
func laterStateSnapshot(t *testing.T, objects objstore.Client, version int64, hp int, hash string) eventbus.Event {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "snapshots", "state", "20260101T000000Z-000000.json"))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	var entities []*entity.Entity
	body, _ := json.Marshal(object["entities"])
	if err := json.Unmarshal(body, &entities); err != nil {
		t.Fatal(err)
	}
	for _, e := range entities {
		if e.ID == "player-A" {
			e.Version, e.Attributes["hp"] = version, float64(hp)
		}
	}
	object["entities"] = entities
	const key = "state/20260913T100000Z-000009.json"
	body, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := objects.Put(context.Background(), objstore.SnapshotsBucket(world), key, body, objstore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		hash = entity.StateHash(entities)
	}
	return eventbus.NewRoot(readmodel.TypeSnapshotCreated, contracts.SourceState, world, nil, eventbus.ActorSystem, map[string]any{
		"component": "state",
		"snapshot": map[string]any{"id": "state:" + world + ":000009", "seq": 9, "taken_at": "2026-09-13T10:00:00Z",
			"cursor": map[string]any{eventbus.TopicSystemEvents: 1}, "laws_version": "v1", "state_hash": hash,
			"size_bytes": len(body), "key": key},
	})
}

// staleGateway is a started gateway whose player-A is stale: a fact of version
// 3 came after the version 1 of the snapshot of State.
func staleGateway(t *testing.T, mode runtime.Mode) (running, objstore.Client) {
	t.Helper()
	objects := snapshotStore(t, nil)
	r := startWith(t, mode, sqlitedir.Temp(t), objects)
	if err := r.bus.Publish(context.Background(), hpFact(3, 10, 7)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the gap of versions to make the projection stale", func() bool {
		h := r.ctx.Health()
		return h.Details["projection"] == "stale" && h.Status == runtime.StatusDegraded
	})
	return r, objects
}

// (а) The projection made stale by a gap of versions is repaired, in live and
// in replay alike, by snapshot.created of State whose object holds the entity
// at a version not below: /health is ok again and the entity is the copy of
// the snapshot (component §11.2, review #1 of T-304, N-1).
func TestASnapshotOfStateRepairsAStaleProjection(t *testing.T) {
	for _, mode := range []runtime.Mode{runtime.ModeLive, runtime.ModeReplay} {
		t.Run(string(mode), func(t *testing.T) {
			testkit.Deterministic(t, "gw")
			r, objects := staleGateway(t, mode)
			if err := r.bus.Publish(context.Background(), laterStateSnapshot(t, objects, 3, 4, "")); err != nil {
				t.Fatal(err)
			}
			eventually(t, "the snapshot of State to repair the projection", func() bool {
				h := r.ctx.Health()
				return h.Details["projection"] == "ok" && h.Status == runtime.StatusOK
			})
			if c, _ := gateway.ReadModel(r.ctx).Character("player-A"); c.HP != 4 || c.Version != 3 {
				t.Errorf("player-A = hp %d v%d, want the copy of the snapshot", c.HP, c.Version)
			}
		})
	}
}

// (е) A snapshot of State that does not check — its hash is not the hash of the
// object — repairs nothing: the projection stays stale, the event moves the
// effects cursor and nothing goes to dead_letters.
func TestASnapshotOfStateThatDoesNotCheckLeavesTheProjectionStale(t *testing.T) {
	testkit.Deterministic(t, "gw")
	r, objects := staleGateway(t, runtime.ModeLive)
	if err := r.bus.Publish(context.Background(), laterStateSnapshot(t, objects, 3, 4, entity.StateHash(nil))); err != nil {
		t.Fatal(err)
	}
	gatewayDB, err := store.OpenGateway(context.Background(), store.GatewayPath(r.dir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gatewayDB.Close() }()
	eventually(t, "the event of the snapshot to move the effects cursor", func() bool {
		var offset int64
		err := gatewayDB.QueryRowContext(context.Background(), `SELECT "offset" FROM cursors WHERE topic = ?`, eventbus.TopicSystemEvents).Scan(&offset)
		return err == nil && offset == 1
	})
	if h := r.ctx.Health(); h.Details["projection"] != "stale" || h.Status != runtime.StatusDegraded {
		t.Errorf("Health after a snapshot that does not check = %+v, want stale", h)
	}
	if letters, err := r.bus.DeadLetters(); err != nil || len(letters) != 0 {
		t.Errorf("dead letters = %d %v, want none", len(letters), err)
	}
}

// silentBus subscribes and delivers nothing: what the gateway holds after its
// start came from the catch-up alone.
type silentBus struct{ eventbus.Bus }

func (silentBus) Subscribe(ctx context.Context, _, _ string, _ eventbus.Handler) error {
	<-ctx.Done()
	return nil
}

// hangingStore hangs every read of a later snapshot of State until its context
// ends, and counts those reads.
type hangingStore struct {
	objstore.Client
	hung atomic.Int32
}

func (h *hangingStore) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	if strings.HasPrefix(key, "state/2026091") {
		h.hung.Add(1)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return h.Client.Get(ctx, bucket, key)
}

// stateAnnouncement is snapshot.created of State of seq whose object nobody
// reads without hanging.
func stateAnnouncement(seq int) eventbus.Event {
	return eventbus.NewRoot(readmodel.TypeSnapshotCreated, contracts.SourceState, world, nil, eventbus.ActorSystem, map[string]any{
		"component": "state",
		"snapshot": map[string]any{"id": fmt.Sprintf("state:%s:%06d", world, seq), "seq": seq, "taken_at": "2026-09-13T10:00:00Z",
			"cursor": map[string]any{eventbus.TopicSystemEvents: 1}, "laws_version": "v1", "state_hash": entity.StateHash(nil),
			"size_bytes": 1, "key": fmt.Sprintf("state/20260913T100000Z-%06d.json", seq)},
	})
}

// A catch-up that meets several snapshots of State while the projection is
// stale reads the store once, for the last of them, after the journal: a store
// that hangs costs one repair budget and leaves the projection stale, and the
// start does not fail (Mi-1 of review #1 of T-309).
func TestAHangingStoreCostsTheCatchUpOneRepair(t *testing.T) {
	testkit.Deterministic(t, "gw")
	objects := &hangingStore{Client: snapshotStore(t, nil)}
	const repair, catchUp = 300 * time.Millisecond, 2 * time.Second
	r, mux, deps := build(t, runtime.ModeLive, sqlitedir.Temp(t), options{
		objects: objects, repairBudget: repair, budget: catchUp,
		bus: func(b *membus.Bus) eventbus.Bus { return silentBus{Bus: b} },
		beforeStart: func(bus *membus.Bus) {
			for _, ev := range []eventbus.Event{hpFact(3, 10, 7), stateAnnouncement(1), stateAnnouncement(2), stateAnnouncement(3)} {
				if err := bus.Publish(context.Background(), ev); err != nil {
					t.Fatal(err)
				}
			}
		},
	})
	_ = mux
	began := clock.Real{}.Now()
	if err := r.ctx.Start(context.Background(), deps); err != nil {
		t.Fatalf("Start with a hanging store: %v", err)
	}
	took := (clock.Real{}).Now().Sub(began)
	t.Cleanup(func() { _ = r.ctx.Stop(context.Background()) })
	if n := objects.hung.Load(); n != 1 {
		t.Errorf("the catch-up read %d snapshots of State, want only the last", n)
	}
	if took > catchUp {
		t.Errorf("Start took %v, the budget of the catch-up is %v", took, catchUp)
	}
	if h := r.ctx.Health(); h.Details["projection"] != "stale" || h.Status != runtime.StatusDegraded {
		t.Errorf("Health = %+v, want stale and degraded", h)
	}
}
