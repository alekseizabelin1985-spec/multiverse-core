package world

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/runtime"
)

// The path of --bus kafka (state-and-mechanics.md §4.10, T-475) against the
// State of a core that runs in the test process: over a membus the command
// shares with it in place of Redpanda — the point of injection is
// Command.OpenBus, like OpenStore — over the memory store of the stand in place
// of MinIO, with its admin route and /health on runtime.HTTP at 127.0.0.1:0.

// newSharedBus is the journal a core of the test and the command both use.
func newSharedBus(t *testing.T) *membus.Bus {
	t.Helper()
	reg := contracts.Default()
	topics := make([]string, 0, len(reg.Topics()))
	for _, topic := range reg.Topics() {
		topics = append(topics, topic.Name)
	}
	bus, err := membus.New(membus.Config{Registry: reg, Topics: topics, Timers: clock.RealTimers{},
		Backoff: []time.Duration{0, 0, 0}, Log: discard()})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}

// unclosable is the shared bus as the command gets it: the command closes its
// bus when it ends, and the core of the test still reads this one.
type unclosable struct{ *membus.Bus }

func (unclosable) Close() error { return nil }

// publishRefusing is a bus that refuses to publish the events refuse names.
type publishRefusing struct {
	*membus.Bus
	refuse func(eventbus.Event) bool
}

func (b publishRefusing) Publish(ctx context.Context, ev eventbus.Event) error {
	if b.refuse(ev) {
		return errors.New("the broker refused " + ev.Type)
	}
	return b.Bus.Publish(ctx, ev)
}

// testCore is the State of a running core.
type testCore struct {
	state *state.Context
	// mux holds the admin route of State; url serves it with /health, unless
	// the core was started without an HTTP server.
	mux     *http.ServeMux
	url     string
	stopped bool
	stopFn  func()
}

func (c *testCore) stop() {
	if !c.stopped {
		c.stopped = true
		c.stopFn()
	}
}

// health is /health of the process of the core.
func (c *testCore) health() runtime.Status {
	return runtime.Aggregate([]runtime.Context{c.state})()
}

// cursor is the cursor of system_events of the world in the State of the core.
func (c *testCore) cursor(t *testing.T) int64 {
	t.Helper()
	section, _, _ := worldOfHealth(toJSON(t, c.health()), world)
	cursor, ok := section["cursor"].(float64)
	if !ok {
		t.Fatalf("the health of core has no cursor of the world: %v", section)
	}
	return int64(cursor)
}

// toJSON is a health as a client of /health reads it.
func toJSON(t *testing.T, health runtime.Status) runtime.Status {
	t.Helper()
	body, err := json.Marshal(health)
	if err != nil {
		t.Fatal(err)
	}
	var decoded runtime.Status
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// coreOptions shape a core of the test.
type coreOptions struct {
	// bus is what State publishes into; nil is the journal itself.
	bus eventbus.Bus
	// noHTTP starts no HTTP server: a double of the test serves core instead.
	noHTTP bool
	// stopMayFail is a core whose Stop fails by design of the test: its bus
	// refuses the snapshot.created of the shutdown snapshot too.
	stopMayFail bool
}

// startCore runs State over the store of the stand and the journal, held to the
// laws of the rule book, with its admin route admitting the client of mvctl
// only.
func (s *stand) startCore(t *testing.T, journal *membus.Bus, opts coreOptions) *testCore {
	t.Helper()
	book, err := mechanics.Load(rulesBook)
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	bus := opts.bus
	if bus == nil {
		bus = journal
	}
	c := state.New(state.Config{Worlds: []string{world}, Log: discard(), Invariants: book.Invariants(),
		Objects: s.objects, SnapshotEvery: -1, RulesVersion: book.Version})
	deps := runtime.Deps{Bus: bus, Journal: journal, Contracts: contracts.Default(), Clock: clock.Real{},
		Timers: clock.RealTimers{}, Mode: runtime.ModeLive, Log: discard()}
	if err := c.Start(context.Background(), deps); err != nil {
		t.Fatalf("Start of core: %v", err)
	}
	t.Setenv(env.CoreAdminClients.Name(), AdminClientID)
	core := &testCore{state: c, mux: http.NewServeMux()}
	c.Routes(core.mux)
	var server *runtime.HTTP
	if !opts.noHTTP {
		server = runtime.NewHTTP("127.0.0.1:0", runtime.Aggregate([]runtime.Context{c}))
		c.Routes(server.Mux)
		if err := server.Start(); err != nil {
			t.Fatalf("HTTP of core: %v", err)
		}
		core.url = "http://" + server.Addr
	}
	core.stopFn = func() {
		ctx, cancel := context.WithTimeout(context.Background(), runtime.StopTimeout)
		defer cancel()
		if server != nil {
			_ = server.Stop(ctx)
		}
		if err := c.Stop(ctx); err != nil && !opts.stopMayFail {
			t.Errorf("Stop of core: %v", err)
		}
	}
	t.Cleanup(core.stop)
	s.core = core
	return core
}

// kafka is the command of the stand over the shared bus and a core at url.
func (s *stand) kafka(bus *membus.Bus, url string) Command {
	cmd := s.command()
	cmd.OpenBus = func(*contracts.Registry) (Transport, error) {
		s.busOpened++
		return unclosable{bus}, nil
	}
	cmd.CoreURL = url
	return cmd
}

func (s *stand) initKafka(cmd Command, extra ...string) run {
	args := append([]string{"init", "--world", world, "--fixtures", fixturesDir, "--bus", BusKafka}, extra...)
	var stdout, stderr strings.Builder
	code := cmd.Run(args, &stdout, &stderr)
	return run{code, stdout.String(), stderr.String()}
}

// proposals is the number of entity.create.proposed on the journal.
func proposals(t *testing.T, bus *membus.Bus) int {
	t.Helper()
	raw, err := bus.Records(eventbus.TopicSystemEvents)
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	n := 0
	for _, body := range raw {
		var ev eventbus.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if ev.Type == state.TypeCreateProposed {
			n++
		}
	}
	return n
}

// forget stops the core and removes the snapshots of State of the world, and
// the entity objects named: what a store holds after a bootstrap cut off before
// its snapshot.
func (s *stand) forget(t *testing.T, core *testCore, entityKeys ...string) {
	t.Helper()
	core.stop()
	ctx := context.Background()
	snapshots := objstore.SnapshotsBucket(world)
	infos, err := s.objects.List(ctx, snapshots, state.Component+"/")
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range infos {
		if err := s.objects.Delete(ctx, snapshots, info.Key); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range entityKeys {
		if err := s.objects.Delete(ctx, objstore.EntitiesBucket(world), key); err != nil {
			t.Fatal(err)
		}
	}
}

// The DoD of T-475: the world is created in the store of the deployment by the
// State of core — six proposals over the bus, the snapshot seq 0 with reason
// bootstrap asked of the admin route, the cursor of system_events of that State
// — and world status over the same store reads the same state_hash.
func TestInitOverKafkaCreatesTheWorldThroughCore(t *testing.T) {
	s := newStand()
	bus := newSharedBus(t)
	core := s.startCore(t, bus, coreOptions{})

	r := s.initKafka(s.kafka(bus, core.url))
	if r.code != cli.ExitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	for _, want := range []string{"entities created 6, skipped 0", "seq 0, reason bootstrap", "entities_count: 6",
		"state_hash: " + fixtureHash(t), "store: minio", core.url} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout does not say %q:\n%s", want, r.stdout)
		}
	}
	if r.stderr != "" {
		t.Errorf("stderr of a clean run: %s", r.stderr)
	}
	if len(s.kinds) != 1 || s.kinds[0] != StoreMinIO || s.busOpened != 1 {
		t.Errorf("stores opened %v, buses %d; want minio once and the bus once", s.kinds, s.busOpened)
	}
	if got := proposals(t, bus); got != 6 {
		t.Errorf("%d proposals, want 6", got)
	}
	meta := s.latest(t).Snapshot
	if meta.Seq != 0 || meta.Reason != state.SnapshotBootstrap || meta.EntitiesCount != 6 || meta.StateHash != fixtureHash(t) {
		t.Errorf("latest.json: seq %d, reason %s, %d entities, %s; want seq 0 bootstrap of the fixture world",
			meta.Seq, meta.Reason, meta.EntitiesCount, meta.StateHash)
	}
	if got, want := meta.Cursor[eventbus.TopicSystemEvents], core.cursor(t); got != want || got != 2*6-1 {
		t.Errorf("cursor.system_events %d, want the cursor of the State of core %d, past the last proposal (11)", got, want)
	}

	status := s.run("status", "--world", world, "--store", StoreMinIO)
	if status.code != cli.ExitOK || !strings.Contains(status.stdout, "state_hash: "+meta.StateHash) {
		t.Errorf("world status: exit %d\nstdout: %s\nstderr: %s", status.code, status.stdout, status.stderr)
	}
}

// The first test of system-architect (§4.10 (b)): the store holds the objects
// of all six entities and the journal none of their facts — nothing is
// proposed, and the snapshot is written through the admin route.
func TestInitOverKafkaProposesNothingTheStoreHolds(t *testing.T) {
	s := newStand()
	first := newSharedBus(t)
	if r := s.initKafka(s.kafka(first, s.startCore(t, first, coreOptions{}).url)); r.code != cli.ExitOK {
		t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
	}
	s.forget(t, s.lastCore(t))
	bus := newSharedBus(t)
	core := s.startCore(t, bus, coreOptions{})

	r := s.initKafka(s.kafka(bus, core.url))
	if r.code != cli.ExitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, "entities created 0, skipped 6") {
		t.Errorf("stdout does not say every entity was skipped:\n%s", r.stdout)
	}
	if got := proposals(t, bus); got != 0 {
		t.Errorf("%d proposals over a store that holds the world", got)
	}
	meta := s.latest(t).Snapshot
	if meta.Seq != 0 || meta.Reason != state.SnapshotBootstrap || meta.EntitiesCount != 6 ||
		meta.Cursor[eventbus.TopicSystemEvents] != core.cursor(t) {
		t.Errorf("latest.json: seq %d, reason %s, %d entities, cursor %d; want seq 0 bootstrap of 6 at the cursor of core %d",
			meta.Seq, meta.Reason, meta.EntitiesCount, meta.Cursor[eventbus.TopicSystemEvents], core.cursor(t))
	}
}

// The second test of system-architect: the store is empty and the journal holds
// the entity.created of the same proposal_id from a world that is gone — all six
// are proposed.
func TestInitOverKafkaIgnoresTheFactsOfTheJournal(t *testing.T) {
	bus := newSharedBus(t)
	gone := newStand()
	old := gone.startCore(t, bus, coreOptions{})
	if r := gone.initKafka(gone.kafka(bus, old.url)); r.code != cli.ExitOK {
		t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
	}
	old.stop()

	s := newStand()
	core := s.startCore(t, bus, coreOptions{})
	before := proposals(t, bus)
	r := s.initKafka(s.kafka(bus, core.url))
	if r.code != cli.ExitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, "entities created 6, skipped 0") {
		t.Errorf("stdout does not say every entity was created:\n%s", r.stdout)
	}
	if got := proposals(t, bus) - before; got != 6 {
		t.Errorf("%d proposals, want all 6 despite the facts of the journal", got)
	}
	if meta := s.latest(t).Snapshot; meta.Seq != 0 || meta.StateHash != fixtureHash(t) {
		t.Errorf("latest.json: seq %d, %s; want seq 0 of the fixture world", meta.Seq, meta.StateHash)
	}
}

// The third test of system-architect: a bootstrap cut off without a pointer
// left the world and the region — only the entities without an object are
// proposed, and the snapshot holds all six.
func TestInitOverKafkaProposesWhatIsMissing(t *testing.T) {
	s := newStand()
	first := newSharedBus(t)
	if r := s.initKafka(s.kafka(first, s.startCore(t, first, coreOptions{}).url)); r.code != cli.ExitOK {
		t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
	}
	s.forget(t, s.lastCore(t), "npc/wolf-alpha.json", "player/player-A.json", "player/player-B.json", "player/player-C.json")
	bus := newSharedBus(t)
	core := s.startCore(t, bus, coreOptions{})

	r := s.initKafka(s.kafka(bus, core.url), "--json")
	report := decodeInit(t, r.stdout)
	if r.code != cli.ExitOK || report.Details.Snapshot == nil {
		t.Fatalf("exit %d, report %+v\nstderr: %s", r.code, report, r.stderr)
	}
	created, skipped := report.Details.Bootstrap.Created, report.Details.Bootstrap.Skipped
	if len(skipped) != 2 || skipped[0].Type != entity.TypeWorld || skipped[1].Type != entity.TypeRegion || len(created) != 4 {
		t.Errorf("created %v, skipped %v; want the world and the region skipped, the other four created", created, skipped)
	}
	if got := proposals(t, bus); got != 4 {
		t.Errorf("%d proposals, want the 4 entities without an object", got)
	}
	if meta := report.Details.Snapshot; meta.Seq != 0 || meta.EntitiesCount != 6 || meta.StateHash != fixtureHash(t) {
		t.Errorf("snapshot seq %d of %d entities, %s; want seq 0 of the fixture world", meta.Seq, meta.EntitiesCount, meta.StateHash)
	}
}

// lastCore is the core the last startCore of the test built; the tests that
// forget a world stop it first.
func (s *stand) lastCore(t *testing.T) *testCore {
	t.Helper()
	if s.core == nil {
		t.Fatal("no core started")
	}
	return s.core
}

// What the path does not do is refused with exit 2 before anything is opened:
// --force (§4.10 (c)), the memory store, a core without an address.
func TestInitOverKafkaRefusesWhatItDoesNotDo(t *testing.T) {
	cases := map[string]struct {
		extra []string
		url   string
		want  []string
	}{
		"--force": {[]string{"--force"}, "http://127.0.0.1:1", []string{"--force is not taken with --bus kafka", "stop core",
			objstore.EntitiesBucket(world), objstore.SnapshotsBucket(world), "start core", "mvctl world init --bus kafka",
			"nothing was done"}},
		"--store memory": {[]string{"--store", StoreMemory}, "http://127.0.0.1:1", []string{"--store minio only", "nothing was done"}},
		"--store s3":     {[]string{"--store", "s3"}, "http://127.0.0.1:1", []string{"unknown store"}},
		"no core":        {nil, "", []string{env.CoreURL.Name(), "nothing was done"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStand()
			r := s.initKafka(s.kafka(nil, tc.url), tc.extra...)
			if r.code != cli.ExitUsage {
				t.Fatalf("exit %d, want %d\nstderr: %s", r.code, cli.ExitUsage, r.stderr)
			}
			for _, want := range tc.want {
				if !strings.Contains(r.stderr, want) {
					t.Errorf("stderr does not say %q: %s", want, r.stderr)
				}
			}
			if s.opened != 0 || s.busOpened != 0 {
				t.Errorf("opened %d stores and %d buses on a refused call", s.opened, s.busOpened)
			}
		})
	}
}

// A world with latest.json is refused with exit 2, as on the path of memory, and
// the text names the way to create it again under the stack.
func TestInitOverKafkaRefusesAWorldAlreadyInitialized(t *testing.T) {
	s := newStand()
	bus := newSharedBus(t)
	core := s.startCore(t, bus, coreOptions{})
	if r := s.initKafka(s.kafka(bus, core.url)); r.code != cli.ExitOK {
		t.Fatalf("first run: exit %d: %s", r.code, r.stderr)
	}
	before := proposals(t, bus)

	r := s.initKafka(s.kafka(bus, core.url))
	if r.code != cli.ExitUsage || !strings.Contains(r.stderr, "is initialized") || !strings.Contains(r.stderr, "stop core") {
		t.Fatalf("exit %d, stderr %q; want %d refused as initialized", r.code, r.stderr, cli.ExitUsage)
	}
	if s.busOpened != 1 || proposals(t, bus) != before {
		t.Errorf("the refused run opened the bus (%d) or proposed (%d → %d)", s.busOpened, before, proposals(t, bus))
	}
}

// A latest.json that does not decode is a finding of the store: the path has no
// --force, and the text says how to create the world again.
func TestInitOverKafkaOverAPointerThatDoesNotDecode(t *testing.T) {
	s := newStand()
	if err := state.EnsureWorldBuckets(context.Background(), s.objects, world); err != nil {
		t.Fatal(err)
	}
	putPointer(t, s, `{"snapshot": [`)
	r := s.initKafka(s.kafka(nil, "http://127.0.0.1:1"))
	if r.code != cli.ExitFindings {
		t.Fatalf("exit %d, want %d\nstderr: %s", r.code, cli.ExitFindings, r.stderr)
	}
	for _, want := range []string{"[store]", state.PointerKey, "stop core"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not say %q: %s", want, r.stderr)
		}
	}
	if s.busOpened != 0 {
		t.Errorf("the bus was opened over a pointer that does not decode")
	}
}

// adminDouble is a core whose /health and snapshot route the test writes. It
// records the requests of the snapshot route.
type adminDouble struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
	server   *httptest.Server
}

func newAdminDouble(t *testing.T, health string, snapshot http.HandlerFunc) *adminDouble {
	t.Helper()
	d := &adminDouble{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, health)
	})
	mux.HandleFunc("POST /v1/admin/state/{world}/snapshot", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		d.mu.Lock()
		d.requests, d.bodies = append(d.requests, r), append(d.bodies, string(body))
		d.mu.Unlock()
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		snapshot(w, r)
	})
	d.server = httptest.NewServer(mux)
	t.Cleanup(d.server.Close)
	return d
}

// The answers of the admin route other than 200 are a finding of the snapshot
// with the code and the message of the answer, exit 1 and no latest.json; the
// body of an error is read tolerantly. The request is the one C-01 names: the
// client of mvctl, the actor ci, the reason bootstrap.
func TestInitOverKafkaReportsTheAnswersOfTheAdminRoute(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   []string
	}{
		"unknown world":          {404, `{"error":{"code":"unknown_world","message":"state: not a world of this context: dark-forest-world"}}`, []string{"404 unknown_world", "not a world of this context"}},
		"no object store":        {409, `{"error":{"code":"no_object_store","message":"state: no object store"}}`, []string{"409 no_object_store"}},
		"not running":            {503, `{"error":{"code":"not_running","message":"state: the context is not running"}}`, []string{"503 not_running"}},
		"world stopped":          {503, `{"error":{"code":"world_stopped","message":"state: the world is stopped"}}`, []string{"503 world_stopped", "the world is stopped"}},
		"snapshot failed":        {500, `{"error":{"code":"snapshot_failed","message":"state: put state/latest.json: refused"}}`, []string{"500 snapshot_failed", "refused"}},
		"forbidden":              {403, `{"error":"admin routes require header X-Client-Id of a client listed in MV_CORE_ADMIN_CLIENTS"}`, []string{"403", "admin routes require", "X-Client-Id mvctl", env.CoreAdminClients.Name()}},
		"an error of the future": {400, `{"error":{"code":"invalid_reason","message":"state: reason \"bootstrap\", expected admin"}}`, []string{"400 invalid_reason"}},
		"a proxy page":           {502, `<html>bad gateway</html>`, []string{"502", "<html>bad gateway</html>"}},
		"200 not latest.json":    {200, `{"ok":true}`, []string{"200", "not latest.json"}},
		// Another init wrote the pointer between the check of this one and its
		// snapshot (§4.9): nothing written here, and the next init gets exit 2.
		"world initialized": {409, `{"error":{"code":"world_initialized","message":"state: the world is initialized: dark-forest-world has its state/latest.json"}}`, []string{"409 world_initialized", "the world is initialized"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStand()
			bus := newSharedBus(t)
			s.startCore(t, bus, coreOptions{noHTTP: true})
			double := newAdminDouble(t, `{"status":"ok"}`, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})

			r := s.initKafka(s.kafka(bus, double.server.URL))
			if r.code != cli.ExitFindings {
				t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, cli.ExitFindings, r.stdout, r.stderr)
			}
			// The health of the double has no context state (N-2 of review #1):
			// the failure says so next to the finding.
			for _, want := range append([]string{"[snapshot]", double.server.URL, "reported no context state"}, tc.want...) {
				if !strings.Contains(r.stderr, want) {
					t.Errorf("stderr does not say %q: %s", want, r.stderr)
				}
			}
			if _, err := state.NewObjectStore(s.objects).ReadLatest(context.Background(), world); !errors.Is(err, state.ErrNoSnapshot) {
				t.Errorf("latest.json after a snapshot that failed: %v", err)
			}
			if len(double.requests) != 1 {
				t.Fatalf("%d requests of the snapshot route, want 1", len(double.requests))
			}
			req := double.requests[0]
			if req.Header.Get(runtime.ClientIDHeader) != AdminClientID || req.Header.Get(runtime.ActorKindHeader) != "ci" ||
				req.PathValue("world") != world || !strings.Contains(double.bodies[0], `"reason":"bootstrap"`) {
				t.Errorf("request %s %v with body %q; want the client mvctl, the actor ci and the reason bootstrap",
					req.URL.Path, req.Header, double.bodies[0])
			}
		})
	}
}

// Mi-1 of review #1 (probe P1 of the reviewer): objects put into the store under
// a running core, without a restart, are objects its State does not hold. After
// the bootstrap the command compares the entities State holds with the store
// and asks for no snapshot when State holds fewer — or the world is still
// uninitialized.
func TestInitOverKafkaOverObjectsTheStateOfCoreDoesNotHold(t *testing.T) {
	cases := map[string]struct {
		copied []string
		want   string
	}{
		"all six objects":          {nil, "holds 0 of the 6 entities of the store, the world still uninitialized"},
		"the wolf and the players": {[]string{"npc/", "player/"}, "holds 2 of the 6 entities of the store"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			source := newStand()
			if r := source.init(); r.code != cli.ExitOK {
				t.Fatalf("the world the objects come from: exit %d: %s", r.code, r.stderr)
			}
			s := newStand()
			bus := newSharedBus(t)
			core := s.startCore(t, bus, coreOptions{})
			ctx := context.Background()
			entities := objstore.EntitiesBucket(world)
			if err := state.EnsureWorldBuckets(ctx, s.objects, world); err != nil {
				t.Fatal(err)
			}
			infos, err := source.objects.List(ctx, entities, "")
			if err != nil {
				t.Fatal(err)
			}
			for _, info := range infos {
				if tc.copied != nil && !strings.HasPrefix(info.Key, tc.copied[0]) && !strings.HasPrefix(info.Key, tc.copied[1]) {
					continue
				}
				body, err := source.objects.Get(ctx, entities, info.Key)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.objects.Put(ctx, entities, info.Key, body, objstore.PutOptions{}); err != nil {
					t.Fatal(err)
				}
			}

			r := s.initKafka(s.kafka(bus, core.url))
			if r.code != cli.ExitFindings {
				t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, cli.ExitFindings, r.stdout, r.stderr)
			}
			for _, want := range []string{"[core]", tc.want, "restart core", "no snapshot was asked for"} {
				if !strings.Contains(r.stderr, want) {
					t.Errorf("stderr does not say %q: %s", want, r.stderr)
				}
			}
			if _, err := state.NewObjectStore(s.objects).ReadLatest(ctx, world); !errors.Is(err, state.ErrNoSnapshot) {
				t.Errorf("latest.json over a world State does not hold: %v", err)
			}
			if refs := s.snapshots(t); len(refs) != 0 {
				t.Errorf("snapshot objects %v over a world State does not hold", refs)
			}
		})
	}
}

// N-1 of review #1: the address of core is printed without the password its
// userinfo may carry — in the findings, in the lines and in --json.
func TestInitOverKafkaPrintsTheAddressOfCoreWithoutItsPassword(t *testing.T) {
	gone := httptest.NewServer(http.NotFoundHandler())
	address := strings.Replace(gone.URL, "http://", "http://operator:s3cr3t-word@", 1)
	gone.Close()
	for _, extra := range [][]string{nil, {"--json"}} {
		s := newStand()
		r := s.initKafka(s.kafka(nil, address), extra...)
		if r.code != cli.ExitFindings || !strings.Contains(r.stdout+r.stderr, "operator:xxxxx@") {
			t.Errorf("%v: exit %d, output %q; want %d naming the redacted address", extra, r.code, r.stdout+r.stderr, cli.ExitFindings)
		}
		if strings.Contains(r.stdout+r.stderr, "s3cr3t-word") {
			t.Errorf("%v: the password of MV_CORE_URL is printed: %s%s", extra, r.stdout, r.stderr)
		}
	}
	if got := redacted("http://a b"); strings.Contains(got, "a b") {
		t.Errorf("an address that does not parse is printed: %q", got)
	}
}

// N-2 of review #1: a health of core without the context state is not a
// refusal, but the command says it could not tell — here on a world it created.
func TestInitOverKafkaWarnsOfAHealthWithoutState(t *testing.T) {
	s := newStand()
	bus := newSharedBus(t)
	core := s.startCore(t, bus, coreOptions{noHTTP: true})
	double := newAdminDouble(t, `{"status":"ok"}`, core.mux.ServeHTTP)
	r := s.initKafka(s.kafka(bus, double.server.URL))
	if r.code != cli.ExitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stderr, "warning: the world is initialized, but core at "+double.server.URL+" reported no context state in /health") {
		t.Errorf("no warning of the health without state: %s", r.stderr)
	}
}

// A core that cannot be reached is a finding of the core that names its address
// and where it comes from, before anything is opened or proposed.
func TestInitOverKafkaWithoutACoreToReach(t *testing.T) {
	gone := httptest.NewServer(http.NotFoundHandler())
	url := gone.URL
	gone.Close()
	s := newStand()
	r := s.initKafka(s.kafka(nil, url))
	if r.code != cli.ExitFindings {
		t.Fatalf("exit %d, want %d\nstderr: %s", r.code, cli.ExitFindings, r.stderr)
	}
	for _, want := range []string{"[core]", url, env.CoreURL.Name(), "not reachable"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not say %q: %s", want, r.stderr)
		}
	}
	if s.busOpened != 0 {
		t.Errorf("the bus was opened without a core")
	}
}

// A core that drops the request of the snapshot after the bootstrap is a
// finding of the snapshot with its address.
func TestInitOverKafkaWhenCoreDropsTheSnapshot(t *testing.T) {
	s := newStand()
	bus := newSharedBus(t)
	s.startCore(t, bus, coreOptions{noHTTP: true})
	double := newAdminDouble(t, `{"status":"ok"}`, func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})
	r := s.initKafka(s.kafka(bus, double.server.URL))
	if r.code != cli.ExitFindings || !strings.Contains(r.stderr, "[snapshot]") || !strings.Contains(r.stderr, double.server.URL) ||
		!strings.Contains(r.stderr, "not reachable") {
		t.Errorf("exit %d, stderr %q; want %d with a snapshot finding naming core", r.code, r.stderr, cli.ExitFindings)
	}
}

// /health of core that says its State will not answer is a finding of the core
// before anything is proposed: a world it does not serve, a context that is not
// running, a world that is stopped.
func TestInitOverKafkaWhenCoreDoesNotServeTheWorld(t *testing.T) {
	withState := func(details string) string {
		return `{"status":"degraded","details":{"contexts":{"state":{"status":"degraded","details":` + details + `}}}}`
	}
	cases := map[string]struct{ health, want string }{
		"another world": {withState(`{"worlds":{"another-world":{"status":"ok"}}}`), "does not serve the world " + world},
		"not running":   {withState(`{"state":"stopped"}`), "is not running"},
		"stopped world": {withState(`{"worlds":{"` + world + `":{"status":"fail","reason":"persist_failed"}}}`), "persist_failed"},
		"not a health":  {`<html></html>`, "not a health"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStand()
			double := newAdminDouble(t, tc.health, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			})
			r := s.initKafka(s.kafka(nil, double.server.URL))
			if r.code != cli.ExitFindings || !strings.Contains(r.stderr, "[core]") || !strings.Contains(r.stderr, tc.want) {
				t.Errorf("exit %d, stderr %q; want %d with a core finding saying %q", r.code, r.stderr, cli.ExitFindings, tc.want)
			}
			if s.busOpened != 0 || len(double.requests) != 0 {
				t.Errorf("opened %d buses and asked %d snapshots of a core that does not serve the world",
					s.busOpened, len(double.requests))
			}
		})
	}
}

// N-3 of review #2 of T-058 on the path where it is real: the State of core
// writes the snapshot and its snapshot.created does not go out. The world is
// initialized — exit 0 — and the command warns, as /health of core says
// snapshot_event_failed.
func TestInitOverKafkaWarnsOfASnapshotEventThatDidNotGoOut(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		t.Run("json="+strconv.FormatBool(asJSON), func(t *testing.T) {
			s := newStand()
			bus := newSharedBus(t)
			core := s.startCore(t, bus, coreOptions{stopMayFail: true, bus: publishRefusing{Bus: bus, refuse: func(ev eventbus.Event) bool {
				return ev.Type == state.TypeSnapshotCreated
			}}})
			var extra []string
			if asJSON {
				extra = []string{"--json"}
			}
			r := s.initKafka(s.kafka(bus, core.url), extra...)
			if r.code != cli.ExitOK {
				t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", r.code, cli.ExitOK, r.stdout, r.stderr)
			}
			if meta := s.latest(t).Snapshot; meta.Seq != 0 || meta.Reason != state.SnapshotBootstrap {
				t.Errorf("latest.json: seq %d %s, want seq 0 bootstrap", meta.Seq, meta.Reason)
			}
			warnings := r.stderr
			if asJSON {
				report := decodeInit(t, r.stdout)
				warnings = strings.Join(report.Details.Warnings, "\n")
				if report.Status != cli.StatusOK {
					t.Errorf("--json: status %s, want ok", report.Status)
				}
			}
			if !strings.Contains(warnings, "snapshot.created") || !strings.Contains(warnings, "snapshot_event_failed") {
				t.Errorf("no warning of snapshot.created: %q", warnings)
			}
			if strings.Contains(r.stderr, "[snapshot]") {
				t.Errorf("the warning is reported as a finding: %s", r.stderr)
			}
		})
	}
}

// A core whose admin route does not read the reason writes the snapshot with
// reason admin: the world is initialized, and the command says the reason is
// not bootstrap.
func TestInitOverKafkaWarnsOfASnapshotWithAnotherReason(t *testing.T) {
	s := newStand()
	bus := newSharedBus(t)
	core := s.startCore(t, bus, coreOptions{noHTTP: true})
	double := newAdminDouble(t, `{"status":"ok"}`, func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.NoBody
		core.mux.ServeHTTP(w, r)
	})
	r := s.initKafka(s.kafka(bus, double.server.URL))
	if r.code != cli.ExitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stderr, "warning: the world is initialized, but core wrote the snapshot with reason admin, not bootstrap") {
		t.Errorf("no warning of the reason: %s", r.stderr)
	}
}

// The refusal of State on the path of kafka names the entity, the reason and
// the details, in the text and in --json.
func TestInitOverKafkaNamesTheDetailsOfARefusal(t *testing.T) {
	s := newStand()
	bus := newSharedBus(t)
	core := s.startCore(t, bus, coreOptions{})
	cmd := s.kafka(bus, core.url)
	var stdout, stderr strings.Builder
	code := cmd.Run([]string{"init", "--world", world, "--fixtures", wolfNowhere(t), "--bus", BusKafka, "--json"}, &stdout, &stderr)
	if code != cli.ExitFindings {
		t.Fatalf("exit %d, want %d\nstdout: %s\nstderr: %s", code, cli.ExitFindings, stdout.String(), stderr.String())
	}
	report := decodeInit(t, stdout.String())
	if len(report.Findings) != 1 || report.Findings[0].Check != CheckBootstrap ||
		!strings.Contains(report.Findings[0].Message, "npc/wolf-alpha refused law_violation (invariant_id inv-") {
		t.Errorf("findings %+v, want the refusal of the wolf with its invariant", report.Findings)
	}
	if refusal := report.Details.Refusal; refusal == nil || refusal.Entity.ID != "wolf-alpha" || refusal.Details["invariant_id"] == nil {
		t.Errorf("refusal %+v, want wolf-alpha with its invariant_id", refusal)
	}
	if _, err := state.NewObjectStore(s.objects).ReadLatest(context.Background(), world); !errors.Is(err, state.ErrNoSnapshot) {
		t.Errorf("latest.json after a refused bootstrap: %v", err)
	}
}
