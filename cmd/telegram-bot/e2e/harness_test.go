//go:build e2e

// Package e2e_test runs the Telegram bot end to end without Telegram (T-315,
// design §7, component §15): updates.Fake plays the messages of the players,
// recording senders take what the bot answers and delivers, and the gateway is
// the real one in-process (gatewaytest.FakeGateway) on the memory bus, with
// the doubles of State and the swarm answering on that bus. Nothing leaves the
// process: no Telegram, no model.
//
// The directory is under cmd/telegram-bot because the packages of the bot are
// internal to it; its own depguard rule, cmd-telegram-bot-e2e, lets these
// tests see FakeGateway and the bus, which the bot itself never imports.
package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/access"
	"multiverse-core.io/cmd/telegram-bot/internal/deliver"
	"multiverse-core.io/cmd/telegram-bot/internal/flow"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/internal/gateway/gatewaytest"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/objstore"
	"multiverse-core.io/shared/testkit"
	tkmech "multiverse-core.io/shared/testkit/mechanics"
	"multiverse-core.io/shared/testkit/state"
	"multiverse-core.io/shared/testkit/swarm"
)

// The accounts of the scenarios. The ids are long enough that a leak into a
// text or a log could not be a coincidence; they are not personal data.
const (
	player      = int64(700000000501)
	otherPlayer = int64(700000000502)
	stranger    = int64(700000000909)
	groupChat   = int64(-1000000000777)
)

// botClientID is the X-Client-Id of the bot at the gateway.
const botClientID = "telegram-bot"

// actionKeySalt is the key of action_key of the bot under test.
var actionKeySalt = []byte("t315-e2e-action-key-salt")

// waitTimeout bounds every wait of a scenario on the wall clock. Nothing
// should reach it: the whole path runs in-process.
const waitTimeout = 10 * time.Second

// secondWorldID is a world of the projection besides the fixture world, so
// that the onboarding asks which world to create the character in.
const (
	secondWorldID   = "misty-hills-world"
	secondWorldName = "Мир Туманных холмов"
)

func fixturesDir() string { return filepath.Join("..", "..", "..", "testdata", "fixtures") }

func rulesPath() string { return filepath.Join("..", "..", "..", "rules", "dark-forest.yaml") }

// worldOptions changes the world of a scenario.
type worldOptions struct {
	// prefix starts the identifiers of the doubles of State and the swarm.
	prefix string
	// vars are variables of the gateway.
	vars map[string]string
	// narrative, when set, replaces the text of every narrative.output the
	// narrator publishes.
	narrative string
}

// world is the platform of a scenario: the gateway with State, the encounter
// and the narrator of Phase 1 behind it on one bus.
type world struct {
	g   *gatewaytest.FakeGateway
	bus *membus.Bus
}

func newWorld(t *testing.T, opts worldOptions) *world {
	t.Helper()
	testkit.Deterministic(t, opts.prefix)
	fixtures, err := state.LoadFixtures(fixturesDir())
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	topics := make([]string, 0, 8)
	for _, spec := range contracts.Topics() {
		topics = append(topics, spec.Name)
	}
	bus, err := membus.New(membus.Config{Registry: contracts.Default(), Topics: topics, Backoff: []time.Duration{0, 0, 0}})
	if err != nil {
		t.Fatalf("membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })

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
	manual := clock.NewManual(gatewaytest.Epoch)
	fake, err := state.New(state.Config{Bus: bus, Store: objstore.NewMemoryWithClock(manual), WorldID: worldID, RulesVersion: "0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.Seed(seed); err != nil {
		t.Fatal(err)
	}
	rules, err := tkmech.Load(rulesPath())
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	enc, err := swarm.NewFakeEncounter(swarm.EncounterConfig{Bus: bus, WorldID: worldID, Rules: rules.Rules(), Mechanics: rules})
	if err != nil {
		t.Fatal(err)
	}
	if err := enc.Seed(seed); err != nil {
		t.Fatal(err)
	}
	var narratorBus eventbus.Bus = bus
	if opts.narrative != "" {
		narratorBus = &rewritingBus{Bus: bus, text: opts.narrative}
	}
	narrator, err := swarm.NewFakeNarrator(narratorBus, worldID)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = fake.Wait()
		_ = enc.Wait()
		_ = narrator.Wait()
	})
	// In a fixed order (N-5 of review #1): the order of the subscriptions is
	// one source of spread fewer between the runs of a scenario.
	for _, double := range []struct {
		name  string
		start func(context.Context) error
	}{{"state", fake.Start}, {"encounter", enc.Start}, {"narrator", narrator.Start}} {
		if err := double.start(ctx); err != nil {
			t.Fatalf("start %s: %v", double.name, err)
		}
	}
	second := &entity.Entity{ID: secondWorldID, Type: entity.TypeWorld, WorldID: secondWorldID, Name: secondWorldName, Attributes: map[string]any{}}
	for _, e := range append(seed, second) {
		if err := bus.Publish(ctx, created(e)); err != nil {
			t.Fatalf("fact of %s: %v", e.ID, err)
		}
	}

	g, err := gatewaytest.Start(gatewaytest.Config{Bus: bus, Vars: opts.vars})
	if err != nil {
		t.Fatalf("gateway: %v", err)
	}
	t.Cleanup(func() {
		if err := g.Close(); err != nil {
			t.Errorf("gateway close: %v", err)
		}
	})
	return &world{g: g, bus: bus}
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

// rewritingBus is the bus as the narrator sees it when a model wrote markup:
// the text of every narrative.output is replaced before it is published.
type rewritingBus struct {
	*membus.Bus
	text string
}

func (b *rewritingBus) Publish(ctx context.Context, ev eventbus.Event) error {
	if ev.Type == "narrative.output" {
		payload := make(map[string]any, len(ev.Payload))
		for k, v := range ev.Payload {
			payload[k] = v
		}
		payload["text"] = b.text
		ev.Payload = payload
	}
	return b.Bus.Publish(ctx, ev)
}

// events returns the events of a topic of the bus of the world.
func (w *world) events(t *testing.T, topic string) []eventbus.Event {
	t.Helper()
	records, err := w.bus.Records(topic)
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

// countOf is the number of events of a type on a topic.
func (w *world) countOf(t *testing.T, topic, typ string) int {
	t.Helper()
	n := 0
	for _, ev := range w.events(t, topic) {
		if ev.Type == typ {
			n++
		}
	}
	return n
}

// line is one message the bot sent, as the transcript keeps it.
type line struct {
	Stream   string
	Chat     int64
	Text     string
	Keyboard string
}

func (l line) String() string {
	return fmt.Sprintf("%s %d %q %s", l.Stream, l.Chat, l.Text, l.Keyboard)
}

// Streams of the transcript: the answers of the update handler (the gate and
// the flow) and the messages of the delivery loop. They are two senders in the
// bot, and the order between them is not the bot's to keep — an answer
// "Принято." and the deliveries of that action race by design — so each is
// kept in its own order.
const (
	streamReply    = "reply"
	streamDelivery = "delivery"
)

// transcript records what the senders of the bot sent.
type transcript struct {
	mu    sync.Mutex
	lines []line
}

func (tr *transcript) all(stream string) []line {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	var out []line
	for _, l := range tr.lines {
		if l.Stream == stream {
			out = append(out, l)
		}
	}
	return out
}

// recordingSender is a Sender of one stream of the transcript.
type recordingSender struct {
	stream string
	tr     *transcript
}

func (s *recordingSender) Send(ctx context.Context, chatID int64, text string, kb *sender.Keyboard) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.tr.mu.Lock()
	s.tr.lines = append(s.tr.lines, line{Stream: s.stream, Chat: chatID, Text: text, Keyboard: keyboard(kb)})
	s.tr.mu.Unlock()
	return nil
}

// keyboard writes a keyboard as one line: rows in brackets, buttons apart by |.
func keyboard(kb *sender.Keyboard) string {
	switch {
	case kb == nil:
		return "-"
	case kb.Remove:
		return "[remove]"
	}
	var b strings.Builder
	for _, row := range kb.Rows {
		b.WriteString("[" + strings.Join(row, "|") + "]")
	}
	return b.String()
}

// call is one call of the flow to the gateway.
type call struct {
	Op        string
	ActionKey string
	Err       error
}

// flowGateway is the client of the flow with every call recorded.
type flowGateway struct {
	c     *client.Client
	mu    sync.Mutex
	calls []call
}

var _ flow.Gateway = (*flowGateway)(nil)

func (f *flowGateway) record(op, key string, err error) {
	f.mu.Lock()
	f.calls = append(f.calls, call{Op: op, ActionKey: key, Err: err})
	f.mu.Unlock()
}

func (f *flowGateway) all() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *flowGateway) Resolve(ctx context.Context, platform, externalID string) (api.ResolveResponse, error) {
	res, err := f.c.Resolve(ctx, platform, externalID)
	f.record("resolve", "", err)
	return res, err
}

func (f *flowGateway) Consent(ctx context.Context, req api.ConsentRequest) (api.ConsentResponse, error) {
	res, err := f.c.Consent(ctx, req)
	f.record("consent", "", err)
	return res, err
}

func (f *flowGateway) Forget(ctx context.Context, platform, externalID string) (client.ForgetResult, error) {
	res, err := f.c.Forget(ctx, platform, externalID)
	f.record("forget", "", err)
	return res, err
}

func (f *flowGateway) Worlds(ctx context.Context) (api.WorldsResponse, error) {
	res, err := f.c.Worlds(ctx)
	f.record("worlds", "", err)
	return res, err
}

func (f *flowGateway) CreateCharacter(ctx context.Context, req api.CreateCharacterRequest) (api.CreateCharacterResponse, int, error) {
	res, status, err := f.c.CreateCharacter(ctx, req)
	f.record("create", req.ActionKey, err)
	return res, status, err
}

func (f *flowGateway) Player(ctx context.Context, playerID string) (api.CharacterState, error) {
	res, err := f.c.Player(ctx, playerID)
	f.record("player", "", err)
	return res, err
}

func (f *flowGateway) Action(ctx context.Context, playerID string, req api.ActionRequest) (client.ActionResult, error) {
	res, err := f.c.Action(ctx, playerID, req)
	f.record("action "+req.Type, req.ActionKey, err)
	return res, err
}

// loopGateway is the client of the delivery loop with the ids it was given and
// the ids the gateway took an ack of.
type loopGateway struct {
	*client.Client
	acks  *client.Client
	mu    sync.Mutex
	given []string
	acked []string
	// lines are the messages the deliveries given should become, in the order
	// the gateway gave them.
	lines []line
}

func (l *loopGateway) Deliveries(ctx context.Context, after string, limit int, wait time.Duration) (api.DeliveriesResponse, error) {
	res, err := l.Client.Deliveries(ctx, after, limit, wait)
	if err == nil {
		l.mu.Lock()
		for _, d := range res.Deliveries {
			l.given = append(l.given, d.ID)
			var chat int64
			if d.Route != nil {
				chat, _ = strconv.ParseInt(d.Route.ExternalID, 10, 64)
			}
			for _, part := range render.Delivery(d) {
				l.lines = append(l.lines, line{Stream: streamDelivery, Chat: chat, Text: part.Text, Keyboard: keyboard(part.Keyboard)})
			}
		}
		l.mu.Unlock()
	}
	return res, err
}

func (l *loopGateway) Ack(ctx context.Context, ids []string) (api.AckResponse, error) {
	res, err := l.acks.Ack(ctx, ids)
	if err == nil {
		l.mu.Lock()
		l.acked = append(l.acked, ids...)
		l.mu.Unlock()
	}
	return res, err
}

// givenLines are the messages of the deliveries given, in the order given.
func (l *loopGateway) givenLines() []line {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.lines)
}

// ids are the deliveries given and the ids acknowledged, in order.
func (l *loopGateway) ids() (given, acked []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.given), slices.Clone(l.acked)
}

// settled says whether every delivery given to the loop was acknowledged.
func (l *loopGateway) settled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, id := range l.given {
		if !slices.Contains(l.acked, id) {
			return false
		}
	}
	return true
}

// faults answers chosen requests of the flow in place of the gateway, and lets
// every other request through to it: the gateway as the flow sees it when its
// broker refuses a publication. It records the action_key of every action
// request that reached it, answered or not.
type faults struct {
	proxy *httputil.ReverseProxy
	mu    sync.Mutex
	// unavailable is how many of the next action requests answer 503
	// bus_unavailable.
	unavailable int
	keys        []string
}

func newFaults(t *testing.T, target string) (*faults, string) {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	f := &faults{proxy: httputil.NewSingleHostReverseProxy(u)}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv.URL
}

func (f *faults) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/actions") {
		var req api.ActionRequest
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if err == nil {
			_ = json.Unmarshal(body, &req)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		f.mu.Lock()
		f.keys = append(f.keys, req.ActionKey)
		refuse := f.unavailable > 0
		if refuse {
			f.unavailable--
		}
		f.mu.Unlock()
		if refuse {
			_ = api.WriteError(w, api.NewError(api.CodeBusUnavailable, nil))
			return
		}
	}
	f.proxy.ServeHTTP(w, r)
}

func (f *faults) refuse(n int) {
	f.mu.Lock()
	f.unavailable = n
	f.mu.Unlock()
}

func (f *faults) actionKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.keys)
}

// botOptions changes the bot of a scenario.
type botOptions struct {
	// gatewayURL is where the flow sends its requests; empty is the gateway.
	gatewayURL string
}

// bot is the bot of cmd/telegram-bot assembled after the pattern of serve.go
// from the same components, with updates.Fake for Telegram and recording
// senders. The wiring of serve.go itself is checked by main_test.go.
type bot struct {
	w      *world
	source *updates.Fake
	tr     *transcript
	flowGW *flowGateway
	loopGW *loopGateway
	gate   *access.Gate
	flow   *flow.Flow
	stop   func()
	// ends are the numbers of delivery messages at the end of each step.
	ends []int
}

// startBot assembles and runs the bot. Where it differs from build and run of
// serve.go (Mi-1 of review #1; a shared assembly is in the backlog):
//
//   - Telegram is updates.Fake and recording senders, so there is no getMe:
//     the delivery loop starts at once instead of after OnReady;
//   - one recording sender takes the answers of the gate and of the flow, which
//     serve.go gives two Telegram clients of their own;
//   - the limits of the client of the flow are copied from productionLimits,
//     which package main does not export; TestTheProductionLimits keeps them
//     within their bounds;
//   - the commands per minute are the default of the manifest;
//   - the flow and the gate run on the manual clock of the gateway, the loop
//     and every pause on the wall clock;
//   - no /health, no privacy logger, no signal handling.
func startBot(t *testing.T, w *world, opts botOptions) *bot {
	t.Helper()
	tr := &transcript{}
	replies := &recordingSender{stream: streamReply, tr: tr}

	flowURL := opts.gatewayURL
	if flowURL == "" {
		flowURL = w.g.URL
	}
	fc := client.New(flowURL, botClientID)
	// A copy of FlowGatewayTimeout and FlowGatewayBackoff of productionLimits
	// (serve.go), unexported in package main.
	fc.HTTP = &http.Client{Timeout: 6 * time.Second}
	fc.Backoff = client.Backoff{Retries: 1, Initial: 200 * time.Millisecond, Max: 200 * time.Millisecond}
	flowGW := &flowGateway{c: fc}

	lc := client.New(w.g.URL, botClientID)
	loopGW := &loopGateway{Client: lc, acks: deliver.NewAckClient(w.g.URL, botClientID, nil)}

	perMinute, err := env.TelegramCommandsPerMin.IntFrom(env.MapSource(nil))
	if err != nil {
		t.Fatal(err)
	}
	gate, err := access.New(access.Options{AllowedUserIDs: []int64{player, otherPlayer}, CommandsPerMinute: perMinute, Clock: w.g.Clock(), Sender: replies})
	if err != nil {
		t.Fatal(err)
	}
	f, err := flow.New(flow.Options{Gateway: flowGW, Sender: replies, Clock: w.g.Clock(), Timers: clock.RealTimers{}, ActionKeySalt: actionKeySalt})
	if err != nil {
		t.Fatal(err)
	}
	loop, err := deliver.New(deliver.Options{Gateway: loopGW, Sender: &recordingSender{stream: streamDelivery, tr: tr}, Timers: clock.RealTimers{}, Clock: clock.Real{}})
	if err != nil {
		t.Fatal(err)
	}
	source := updates.NewFake()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		loop.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		if err := source.Start(ctx, gate.Wrap(f.Handle)); err != nil {
			t.Errorf("update source: %v", err)
		}
	}()
	b := &bot{w: w, source: source, tr: tr, flowGW: flowGW, loopGW: loopGW, gate: gate, flow: f}
	var once sync.Once
	b.stop = func() {
		once.Do(func() {
			cancel()
			wg.Wait()
		})
	}
	t.Cleanup(b.stop)
	return b
}

// profile is the Telegram profile of the player: its username is a valid
// character name, so the onboarding meets the name that repeats it (US-009).
const (
	profileUsername  = "Vasya"
	profileFirstName = "Василий"
)

// message is an update of a private chat of from.
func message(id, from int64, text string) updates.Update {
	return updates.Update{ID: id, Message: &updates.Message{
		Chat: updates.Chat{ID: from, Type: updates.ChatPrivate},
		From: &updates.User{ID: from, Username: profileUsername, FirstName: profileFirstName},
		Text: text,
	}}
}

// groupMessage is an update of a group chat.
func groupMessage(id, from int64, text string) updates.Update {
	u := message(id, from, text)
	u.Message.Chat = updates.Chat{ID: groupChat, Type: "group"}
	return u
}

// step pushes one update and waits until the bot answered it with replies
// messages, delivered deliveries more, and acknowledged every delivery it was
// given: the next update then meets a quiet platform, which keeps the
// identifiers, and with them the dice, of a run the same as of the last.
func (b *bot) step(t *testing.T, u updates.Update, replies, deliveries int) {
	t.Helper()
	wantReplies := len(b.tr.all(streamReply)) + replies
	wantDeliveries := len(b.tr.all(streamDelivery)) + deliveries
	b.source.Push(u)
	waitFor(t, fmt.Sprintf("%d answers and %d deliveries of update %d", replies, deliveries, u.ID), func() bool {
		return len(b.tr.all(streamReply)) >= wantReplies && len(b.tr.all(streamDelivery)) >= wantDeliveries && b.loopGW.settled()
	})
	b.ends = append(b.ends, len(b.tr.all(streamDelivery)))
}

// deliveriesByStep are the delivery messages of each step, each step sorted.
// The gateway enqueues the deliveries of one action from four topics as their
// events arrive, and C-01 orders no two topics: the order within a step is the
// gateway's and may differ between runs, while the bot keeps whatever order it
// was given (checked against loopGateway.givenLines).
func (b *bot) deliveriesByStep() [][]string {
	all := b.tr.all(streamDelivery)
	out := make([][]string, 0, len(b.ends))
	from := 0
	for _, end := range b.ends {
		var step []string
		for _, l := range all[from:end] {
			step = append(step, l.String())
		}
		slices.Sort(step)
		out = append(out, step)
		from = end
	}
	return out
}

// waitFor waits for cond on the wall clock.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	wall := clock.Real{}
	deadline := wall.Now().Add(waitTimeout)
	for !cond() {
		if !wall.Now().Before(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		<-clock.RealTimers{}.After(5 * time.Millisecond).C()
	}
}
