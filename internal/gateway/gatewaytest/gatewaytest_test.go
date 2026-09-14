package gatewaytest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/internal/gateway/gatewaytest"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

const regionID = "dark-forest-01"

func fixturesDir() string { return filepath.Join("..", "..", "..", "testdata", "fixtures") }

// start is a FakeGateway closed with the test.
func start(t *testing.T, cfg gatewaytest.Config) *gatewaytest.FakeGateway {
	t.Helper()
	g, err := gatewaytest.Start(cfg)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		if err := g.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return g
}

// The double comes up and goes down within a second, without anything outside
// the process, and leaves nothing behind: the server answers /health and the
// routes of the gateway on loopback, Close stops both and removes the data
// directory it made, and a second Close returns what the first did.
func TestAFakeGatewayStartsAndStopsWithinASecond(t *testing.T) {
	wall := clock.Real{}
	began := wall.Now()
	g, err := gatewaytest.Start(gatewaytest.Config{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	started := wall.Now()
	if !strings.HasPrefix(g.URL, "http://127.0.0.1:") {
		t.Errorf("URL = %q, want loopback", g.URL)
	}
	if h := g.Health(); h.Status != runtime.StatusOK || h.Details["mode"] != string(runtime.ModeLive) {
		t.Errorf("Health = %+v, want ok in live", h)
	}
	resp, err := http.Get(g.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/health = %d", resp.StatusCode)
	}
	if _, err := g.Client(gateway.ClientID).Worlds(context.Background()); err != nil {
		t.Errorf("Worlds through the routes of the gateway: %v", err)
	}
	if _, err := os.Stat(store.GatewayPath(g.Dir)); err != nil {
		t.Errorf("gateway.db in the data directory: %v", err)
	}
	closing := wall.Now()
	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// The phases are named so that a run over the threshold says where the
	// time went: Start opens and migrates two SQLite files, and on a loaded
	// machine that alone takes most of the second (T-480).
	if took := wall.Now().Sub(began); took > time.Second {
		t.Errorf("start and stop took %s, want at most 1s: Start %s, the requests %s, Close %s",
			took, started.Sub(began), closing.Sub(started), wall.Now().Sub(closing))
	}
	if _, err := os.Stat(g.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("data directory after Close: %v, want removed", err)
	}
	if h := g.Health(); h.Status != runtime.StatusFail {
		t.Errorf("Health after Close = %+v, want fail", h)
	}
	if err := g.Close(); err != nil {
		t.Errorf("second Close = %v", err)
	}
}

// What a test passed stays the test's: its data directory is not removed and
// its bus is not closed; the variables reach the gateway, and the directory of
// the manifest does not (the process environment is never read).
//
// The directory is sqlitedir.Temp and not t.TempDir: the store refuses a data
// directory wider than 0700, and t.TempDir makes its directory 0777 less the
// umask, 0755 on Linux (T-482).
func TestAFakeGatewayLeavesWhatTheTestPassed(t *testing.T) {
	dir := sqlitedir.Temp(t)
	bus := newBus(t)
	g, err := gatewaytest.Start(gatewaytest.Config{Dir: dir, Bus: bus, Mode: runtime.ModeReplay,
		Vars: map[string]string{"MV_GATEWAY_DATA_DIR": filepath.Join(dir, "elsewhere")}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if g.Dir != dir || g.Bus() != bus {
		t.Errorf("Dir %q, bus %p, want %q and %p", g.Dir, g.Bus(), dir, bus)
	}
	if h := g.Health(); h.Details["mode"] != string(runtime.ModeReplay) {
		t.Errorf("Health = %+v, want replay", h)
	}
	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(store.GatewayPath(dir)); err != nil {
		t.Errorf("gateway.db of the directory of the test after Close: %v", err)
	}
	w := &entity.Entity{ID: "w", Type: entity.TypeWorld, WorldID: "w", Name: "Мир", Attributes: map[string]any{}}
	if err := bus.Publish(context.Background(), created(w)); err != nil {
		t.Errorf("the bus of the test is closed by Close: %v", err)
	}
}

// A start that fails releases what it made: a variable the gateway refuses
// fails Start, the temporary directory is gone and the bus the double made for
// itself is closed (review #1 of T-308, N-4).
func TestAFailedStartLeavesNothing(t *testing.T) {
	parent := t.TempDir()
	t.Setenv("TMP", parent)
	t.Setenv("TMPDIR", parent)
	var buses []*membus.Bus
	restore := gatewaytest.WatchBuses(func(b *membus.Bus) { buses = append(buses, b) })
	defer restore()
	_, err := gatewaytest.Start(gatewaytest.Config{Vars: map[string]string{"MV_GATEWAY_INPUT_FILTER": "unknown"}})
	if err == nil {
		t.Fatal("Start with a filter the gateway refuses succeeded")
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("left behind: %v", entries)
	}
	if len(buses) != 1 {
		t.Fatalf("buses made = %d, want 1", len(buses))
	}
	w := &entity.Entity{ID: "w", Type: entity.TypeWorld, WorldID: "w", Name: "Мир", Attributes: map[string]any{}}
	if err := buses[0].Publish(context.Background(), created(w)); !errors.Is(err, eventbus.ErrClosed) {
		t.Errorf("publish on the bus of a failed start = %v, want ErrClosed", err)
	}
}

// Close stops the server before the gateway (review #1 of T-308, Mi-2): a
// long-poll waiting in it answers an empty list at once instead of meeting
// closed databases, and the port no longer answers.
func TestCloseAnswersAWaitingLongPollAndStopsTheServer(t *testing.T) {
	g, err := gatewaytest.Start(gatewaytest.Config{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out := waitingPoll(t, ctx, g)
	wall := clock.Real{}
	began := wall.Now()
	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case a := <-out:
		if a.err != nil || len(a.Deliveries) != 0 {
			t.Errorf("waiting long-poll at Close = %+v %v, want an empty list", a.DeliveriesResponse, a.err)
		}
	case <-ctx.Done():
		t.Fatal("the waiting long-poll did not answer at Close")
	}
	if took := wall.Now().Sub(began); took > 2*time.Second {
		t.Errorf("Close with a waiting long-poll took %s", took)
	}
	probe := &http.Client{Timeout: 2 * time.Second}
	if resp, err := probe.Get(g.URL + "/health"); err == nil {
		_ = resp.Body.Close()
		t.Errorf("/health after Close = %d, want the server gone", resp.StatusCode)
	}
}

// pollAnswer is the answer of a long-poll.
type pollAnswer struct {
	api.DeliveriesResponse
	err error
}

// waitingPoll starts a long-poll of 15 s of ci-harness and returns once it is
// inside the server: a probe of the same client is refused 409
// poll_in_progress. A probe can reach the server first and refuse the poll
// instead; the poll is started again then.
func waitingPoll(t *testing.T, ctx context.Context, g *gatewaytest.FakeGateway) <-chan pollAnswer {
	t.Helper()
	wall := clock.Real{}
	deadline := wall.Now().Add(5 * time.Second)
	for wall.Now().Before(deadline) {
		out := make(chan pollAnswer, 1)
		go func() {
			res, err := g.Client(gateway.ClientID).Deliveries(ctx, "", 10, 15*time.Second)
			out <- pollAnswer{res, err}
		}()
		<-clock.RealTimers{}.After(50 * time.Millisecond).C()
		_, err := g.Client(gateway.ClientID).Deliveries(ctx, "", 10, 0)
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.Code == api.CodePollInProgress {
			return out
		}
		select {
		case a := <-out:
			t.Logf("the poll met the probe (%v); started again", a.err)
		case <-clock.RealTimers{}.After(200 * time.Millisecond).C():
			// The probe went first and the poll came in after it: it waits.
			return out
		}
	}
	t.Fatal("the long-poll did not get into the server")
	return nil
}

// world is a FakeGateway with State and the narrator of Phase 1 on its bus and
// the fixture world in its projection, and the HTTP harness of ci-harness on
// it.
type world struct {
	g       *gatewaytest.FakeGateway
	harness *gateway.HTTPHarness
}

func newWorld(t *testing.T) world {
	t.Helper()
	testkit.Deterministic(t, "e2e")
	fixtures, err := state.LoadFixtures(fixturesDir())
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	bus := newBus(t)
	worldID := ""
	var seed []*entity.Entity
	for _, e := range fixtures {
		if e.Type == entity.TypeWorld {
			worldID = e.ID
		}
		if e.Type != entity.TypePlayer {
			seed = append(seed, e)
		}
	}
	fake, err := state.New(state.Config{Bus: bus, WorldID: worldID})
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.Seed(seed); err != nil {
		t.Fatal(err)
	}
	narrator, err := swarm.NewFakeNarrator(bus, worldID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = fake.Wait()
		_ = narrator.Wait()
	})
	if err := fake.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := narrator.Start(ctx); err != nil {
		t.Fatal(err)
	}
	// The world State already holds, told the way State tells a consumer that
	// starts from an empty journal: a fact per entity. The gateway has no object
	// store here, so its projection is built from the journal alone.
	for _, e := range seed {
		if err := bus.Publish(ctx, created(e)); err != nil {
			t.Fatalf("fact of %s: %v", e.ID, err)
		}
	}
	g := start(t, gatewaytest.Config{Bus: bus})
	harness, err := gateway.NewHTTPHarness(gateway.NewCIClient(g.URL), fixtures)
	if err != nil {
		t.Fatal(err)
	}
	return world{g: g, harness: harness.WithClock(g.Clock())}
}

// created is the entity.created of State for an entity it already holds.
func created(e *entity.Entity) eventbus.Event {
	raw, _ := json.Marshal(map[string]any{
		"entity":      map[string]any{"entity": map[string]any{"id": e.ID, "type": e.Type}, "name": e.Name},
		"version":     1,
		"attributes":  e.Attributes,
		"proposal_id": "seed-" + e.ID,
	})
	var payload map[string]any
	_ = json.Unmarshal(raw, &payload)
	return eventbus.NewRoot(state.TypeCreated, state.Source, e.WorldID, nil, eventbus.ActorSystem, payload)
}

// RegisterAndEnter takes a fixture character from nothing to alive in its
// region over HTTP, and Act answers with the correlation of the player event
// it published (C-08, C-04).
func TestRegisterAndEnterBringsAFixtureCharacterAliveIntoTheRegion(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()

	entered, err := w.harness.RegisterAndEnter(ctx, "player-A", regionID)
	if err != nil {
		t.Fatalf("RegisterAndEnter: %v", err)
	}
	player, err := w.harness.Client().Player(ctx, entered.PlayerID)
	if err != nil || player.Status != entity.StatusAlive || player.Name != "Вася" || player.Position == nil ||
		player.Position.ID != regionID || player.WorldID != w.harness.WorldID() {
		t.Fatalf("Player = %+v %v, want Вася alive in %s", player, err, regionID)
	}
	if id, ok := w.harness.PlayerID("player-A"); !ok || id != entered.PlayerID {
		t.Errorf("PlayerID = %q %v, want %q", id, ok, entered.PlayerID)
	}
	if !strings.HasPrefix(entered.PlayerID, "player-"+gatewaytest.IDPrefix+"-") {
		t.Errorf("player_id %q is not from the sequence of the double", entered.PlayerID)
	}

	correlation, err := w.harness.Act(ctx, entered.PlayerID, api.ActionRequest{Type: api.ActionLook})
	if err != nil || correlation == "" {
		t.Fatalf("Act = %q %v", correlation, err)
	}
	var looked, enteredEvent bool
	for _, ev := range events(t, w, eventbus.TopicPlayerEvents) {
		switch {
		case ev.Type == gateway.TypeLooked && ev.Meta.CorrelationID == correlation:
			looked = true
		case ev.Type == gateway.TypeEnteredRegion && ev.Meta.CorrelationID == entered.CorrelationID:
			enteredEvent = ev.Meta.ActorKind == eventbus.ActorCI
		}
	}
	if !looked || !enteredEvent {
		t.Errorf("player_events: look under %s %v, enter of ci under %s %v", correlation, looked, entered.CorrelationID, enteredEvent)
	}

	if _, err := w.harness.RegisterAndEnter(ctx, "player-Z", regionID); err == nil {
		t.Error("RegisterAndEnter of a character the fixtures do not hold succeeded")
	}
}

// The long-poll of the harness end to end (acceptance of T-307, review #2,
// backlog p. 3): the mechanics and the narrative of a turn come through the
// real GET …/deliveries of ci-harness, the harness acknowledges them with
// POST …/ack, and the turn completes with one analytics.turn.completed.
func TestAwaitDeliveryTakesATurnThroughTheLongPollAndCompletesIt(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	entered, err := w.harness.RegisterAndEnter(ctx, "player-A", regionID)
	if err != nil {
		t.Fatalf("RegisterAndEnter: %v", err)
	}

	mechanics, err := w.harness.AwaitDeliveryOf(ctx, entered.CorrelationID, outbox.KindMechanics, 5*time.Second)
	if err != nil {
		t.Fatalf("mechanics: %v", err)
	}
	narrative, err := w.harness.AwaitDeliveryOf(ctx, entered.CorrelationID, outbox.KindNarrative, 5*time.Second)
	if err != nil {
		t.Fatalf("narrative: %v", err)
	}
	for _, d := range []api.Delivery{mechanics, narrative} {
		// Variant (a) of the platform of ci-harness, as the code of T-307 has
		// it: the harness takes the queue of telegram and sees the route of the
		// link it made (handlers.ClientPlatforms).
		if d.PlayerID != entered.PlayerID || d.Route == nil || d.Route.ExternalPlatform != gateway.Platform ||
			d.Route.ExternalID != "player-A" || d.Text == "" {
			t.Errorf("delivery = %+v", d)
		}
	}

	var completed []eventbus.Event
	waitFor(t, "analytics.turn.completed of the enter", func() bool {
		completed = completed[:0]
		for _, ev := range events(t, w, eventbus.TopicAnalyticsEvents) {
			if ev.Type == turns.TypeCompleted && ev.Meta.CorrelationID == entered.CorrelationID {
				completed = append(completed, ev)
			}
		}
		return len(completed) > 0
	})
	if len(completed) != 1 {
		t.Fatalf("analytics.turn.completed of the enter: %d, want 1", len(completed))
	}
	if status, _ := completed[0].Path().GetString("turn.status"); status != turns.OutcomeDegraded && status != turns.OutcomeOK {
		t.Errorf("turn.completed status = %q, want the narrative delivered", status)
	}

	// Nothing is left to take: both deliveries were acknowledged.
	left, err := w.harness.Client().Deliveries(ctx, "", 10, 0)
	if err != nil || len(left.Deliveries) != 0 {
		t.Errorf("deliveries after the acks = %+v %v, want none", left, err)
	}
}

// AwaitDelivery does not hang when nothing comes: it returns ErrNoDelivery
// soon after its timeout and names what it waited for.
func TestAwaitDeliveryReturnsWhenNothingArrives(t *testing.T) {
	g := start(t, gatewaytest.Config{})
	fixtures, err := state.LoadFixtures(fixturesDir())
	if err != nil {
		t.Fatal(err)
	}
	h, err := gateway.NewHTTPHarness(gateway.NewCIClient(g.URL), fixtures)
	if err != nil {
		t.Fatal(err)
	}
	wall := clock.Real{}
	began := wall.Now()
	_, err = h.AwaitDelivery(context.Background(), outbox.KindNarrative, 300*time.Millisecond)
	took := wall.Now().Sub(began)
	if !errors.Is(err, gateway.ErrNoDelivery) || !strings.Contains(err.Error(), outbox.KindNarrative) {
		t.Fatalf("AwaitDelivery = %v, want ErrNoDelivery naming the kind", err)
	}
	if took < 300*time.Millisecond || took > 3*time.Second {
		t.Errorf("AwaitDelivery returned after %s, want soon after 300ms", took)
	}
}

// CloseRound in I1: the gateway does not mount closeRound, and the harness
// says so as 501 not_implemented instead of a bare 404.
func TestCloseRoundIsNotImplementedInI1(t *testing.T) {
	g := start(t, gatewaytest.Config{})
	h, err := gateway.NewHTTPHarness(gateway.NewCIClient(g.URL), []*entity.Entity{{ID: "w", Type: entity.TypeWorld}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.CloseRound(context.Background(), "group:g-1")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotImplemented || apiErr.Code != api.CodeNotImplemented {
		t.Errorf("CloseRound = %v, want 501 not_implemented", err)
	}
}

// Two doubles driven the same way give the same identifiers: the sequence and
// the manual clock make a run repeatable.
func TestTwoRunsGiveTheSameIdentifiers(t *testing.T) {
	var ids []string
	for range 2 {
		w := newWorld(t)
		entered, err := w.harness.RegisterAndEnter(context.Background(), "player-B", regionID)
		if err != nil {
			t.Fatalf("RegisterAndEnter: %v", err)
		}
		ids = append(ids, entered.PlayerID)
	}
	if ids[0] != ids[1] {
		t.Errorf("player_id of two runs = %v, want one", ids)
	}
}

func events(t *testing.T, w world, topic string) []eventbus.Event {
	t.Helper()
	records, err := w.g.Bus().Records(topic)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]eventbus.Event, 0, len(records))
	for _, r := range records {
		var ev eventbus.Event
		if err := json.Unmarshal(r, &ev); err != nil {
			t.Fatal(err)
		}
		out = append(out, ev)
	}
	return out
}

// waitFor waits for cond for five seconds of wall time (review #1 of T-308,
// N-3: a count of pauses lasts as long as the timers of the system make it).
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	wall := clock.Real{}
	deadline := wall.Now().Add(5 * time.Second)
	for !cond() {
		if !wall.Now().Before(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		<-clock.RealTimers{}.After(10 * time.Millisecond).C()
	}
}

// newBus is a membus over every topic of the registry.
func newBus(t *testing.T) *membus.Bus {
	t.Helper()
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: topics, Backoff: []time.Duration{0, 0, 0}})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })
	return bus
}
