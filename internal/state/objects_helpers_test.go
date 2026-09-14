package state_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"multiverse-core.io/internal/state"
	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
)

// tracedObjects is the memory object store with a hook in front of every write
// and a timeline of what was written and removed, so that a test can fail a
// write, hold one, and see where the writes stand among the publications.
type tracedObjects struct {
	*objstore.Memory

	mu       sync.Mutex
	hook     func(bucket, key string) error
	hold     func(key string) bool
	held     chan string
	timeline *timeline
}

// holdUntilCancelled makes every write whose key hold accepts hang until its
// context ends, the way a request to a server that does not answer does. The
// key of each held write is sent on the channel returned.
func (o *tracedObjects) holdUntilCancelled(hold func(key string) bool) <-chan string {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.hold, o.held = hold, make(chan string, 16)
	return o.held
}

// timeline is the order of the writes and publications of one test.
type timeline struct {
	mu    sync.Mutex
	steps []string
}

func (tl *timeline) add(step string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.steps = append(tl.steps, step)
}

func (tl *timeline) all() []string {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	return append([]string(nil), tl.steps...)
}

// matching is every step that starts with prefix.
func (tl *timeline) matching(prefix string) []string {
	var out []string
	for _, step := range tl.all() {
		if strings.HasPrefix(step, prefix) {
			out = append(out, step)
		}
	}
	return out
}

func newTracedObjects(t *testing.T, worlds ...string) *tracedObjects {
	t.Helper()
	objects := &tracedObjects{Memory: objstore.NewMemoryWithClock(testkit.Wall()), timeline: &timeline{}}
	for _, w := range worlds {
		if err := state.EnsureWorldBuckets(context.Background(), objects.Memory, w); err != nil {
			t.Fatalf("buckets of %s: %v", w, err)
		}
	}
	return objects
}

func (o *tracedObjects) setHook(h func(bucket, key string) error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.hook = h
}

func (o *tracedObjects) Put(ctx context.Context, bucket, key string, body []byte, opts objstore.PutOptions) (string, error) {
	o.mu.Lock()
	hook, hold, held := o.hook, o.hold, o.held
	o.mu.Unlock()
	if hold != nil && hold(key) {
		o.timeline.add("held " + key)
		held <- key
		<-ctx.Done()
		return "", ctx.Err()
	}
	if hook != nil {
		if err := hook(bucket, key); err != nil {
			o.timeline.add("refused " + key)
			return "", err
		}
	}
	// A client of the server gives up a request whose context is cancelled;
	// the memory store would not notice, and a write handed the context of a
	// stopping subscription would pass here and fail against MinIO.
	if err := ctx.Err(); err != nil {
		o.timeline.add("cancelled " + key)
		return "", err
	}
	etag, err := o.Memory.Put(ctx, bucket, key, body, opts)
	if err == nil {
		o.timeline.add("put " + key)
	}
	return etag, err
}

func (o *tracedObjects) Delete(ctx context.Context, bucket, key string) error {
	err := o.Memory.Delete(ctx, bucket, key)
	if err == nil {
		o.timeline.add("delete " + key)
	}
	return err
}

// entityObject reads the object of one entity as a restarted State would.
func (o *tracedObjects) entityObject(t *testing.T, worldID, typ, id string) *entity.Entity {
	t.Helper()
	e, err := state.NewObjectStore(o.Memory).GetEntity(context.Background(), worldID, typ, id)
	if err != nil {
		t.Fatalf("object of %s: %v", id, err)
	}
	return e
}

// newStoredFixture is the Applier of the world writing through to a memory
// object store, with the deterministic sources of newFixture. Every write and
// every publication goes into one timeline.
func newStoredFixture(t *testing.T, cfg state.ApplierConfig) (*fixture, *tracedObjects) {
	t.Helper()
	objects := newTracedObjects(t, world)
	return newStoredFixtureOver(t, objects, cfg), objects
}

// newStoredFixtureOver is newStoredFixture over an object store that already
// holds the world, and — with cfg.Store — over the working set a restart would
// have found in it.
func newStoredFixtureOver(t *testing.T, objects *tracedObjects, cfg state.ApplierConfig) *fixture {
	t.Helper()
	cfg.Objects = state.NewObjectStore(objects)
	cfg.WithoutOwnership = true
	cfg.Writer = "core/state@test:0"
	if cfg.RulesVersion == "" {
		cfg.RulesVersion = "0.1"
	}
	f := fixtureWith(t, cfg)
	f.journal.fail = func(_ int, ev eventbus.Event) error {
		objects.timeline.add("publish " + ev.Type + " " + subjectOf(ev))
		return nil
	}
	return f
}

// runningStored starts State over the bus writing through to objects, on the
// manual clock of the deterministic sources: its pauses and its snapshots read
// that clock. every is Config.SnapshotEvery (below zero: no snapshot on the
// count).
func runningStored(t *testing.T, bus eventbus.Bus, objects objstore.Client, every int) (*state.Context, *clock.Manual) {
	t.Helper()
	c, manual, _ := runningStoredLogged(t, bus, objects, every)
	return c, manual
}

// runningStoredLogged is runningStored with the log of the context.
func runningStoredLogged(t *testing.T, bus eventbus.Bus, objects objstore.Client, every int) (*state.Context, *clock.Manual, *lockedBuffer) {
	t.Helper()
	sources := testkit.Deterministic(t, "t057")
	eventbus.SetRegistry(contracts.Default())
	logged := &lockedBuffer{}
	c := state.New(state.Config{Worlds: []string{world}, Timers: sources.Timers,
		Objects: objects, SnapshotEvery: every, RulesVersion: "0.1",
		Log: slog.New(slog.NewJSONHandler(logged, &slog.HandlerOptions{Level: slog.LevelDebug}))})
	if err := c.Start(context.Background(), runtime.Deps{Bus: bus, Clock: sources.Clock}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		_ = c.Stop(ctx)
	})
	return c, sources.Clock, logged
}

// createWorld is the proposal that brings the world entity with its laws,
// which every snapshot reads.
func createWorld() eventbus.Event {
	return create("prop-world", ref(world, entity.TypeWorld), "", map[string]any{"laws_version": "v1"})
}

// subjectOf is the entity an event is about, when it names one.
func subjectOf(ev eventbus.Event) string {
	id, _ := ev.Path().GetString("entity.entity.id")
	return id
}

// seedWorld puts the world entity with its laws in, which every snapshot reads.
func (f *fixture) seedWorld(t *testing.T) {
	t.Helper()
	f.seed(t, world, entity.TypeWorld, 1, map[string]any{"laws_version": "v1"})
}

// loadedFrom is the working set a restarted process would find in the object
// store: every entity object of the world. It stands for the object branch of
// recovery (§4.8) until T-059 writes recovery itself.
func loadedFrom(t *testing.T, objects objstore.Client, worldIDs ...string) *memstore.Store {
	t.Helper()
	store := memstore.New()
	for _, w := range worldIDs {
		entities, err := state.NewObjectStore(objects).ListEntities(context.Background(), w)
		if err != nil {
			t.Fatalf("entities of %s: %v", w, err)
		}
		if err := store.Put(w, entities...); err != nil {
			t.Fatalf("working set of %s: %v", w, err)
		}
	}
	return store
}

// readJSON decodes one object of the memory store.
func readJSON(t *testing.T, objects objstore.Client, bucket, key string, into any) []byte {
	t.Helper()
	body, err := objects.Get(context.Background(), bucket, key)
	if err != nil {
		t.Fatalf("get %s/%s: %v", bucket, key, err)
	}
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("decode %s/%s: %v", bucket, key, err)
	}
	return body
}
