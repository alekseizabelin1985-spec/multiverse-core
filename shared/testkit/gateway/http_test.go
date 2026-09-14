package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/testkit/gateway"
)

// The harness sends through the client of the bot, not a copy of it (NFR-092):
// a harness over another type does not compile.
var _ func(*client.Client, []*entity.Entity) (*gateway.HTTPHarness, error) = gateway.NewHTTPHarness
var _ func() *client.Client = (*gateway.HTTPHarness)(nil).Client

// fakeAPI is the HTTP API of the gateway as far as a unit of the harness needs
// it: every request is recorded, and each operation answers what the test set.
type fakeAPI struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
	// deliveries are handed out one poll at a time; an empty poll waits
	// pollWait before it answers, as the gateway waits wait_ms.
	deliveries [][]api.Delivery
	pollWait   time.Duration
	acked      [][]string
	unknownAck []string
	// action answers POST …/actions.
	actionStatus int
	actionBody   string
	// closeRound answers POST …/rounds/close; nil is the mux of I1, a bare 404.
	closeRound http.HandlerFunc
	// position is where GET /v1/players/{id} places the character.
	position string
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, r)
	f.bodies = append(f.bodies, string(body))
	actionStatus, actionBody, closeRound, position := f.actionStatus, f.actionBody, f.closeRound, f.position
	f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/deliveries"):
		f.poll(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/deliveries/ack"):
		var req api.AckRequest
		_ = json.Unmarshal(body, &req)
		f.mu.Lock()
		f.acked = append(f.acked, req.IDs)
		unknown := f.unknownAck
		f.mu.Unlock()
		_ = api.WriteJSON(w, http.StatusOK, api.AckResponse{Acked: len(req.IDs) - len(unknown), Unknown: append([]string{}, unknown...)})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/actions"):
		w.Header().Set("Content-Type", api.ContentTypeJSON)
		w.WriteHeader(actionStatus)
		_, _ = w.Write([]byte(actionBody))
	case strings.HasSuffix(r.URL.Path, "/rounds/close") && closeRound != nil:
		closeRound(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/links/resolve":
		id := "player-1"
		_ = api.WriteJSON(w, http.StatusOK, api.ResolveResponse{LinkStatus: "consented", PlayerID: &id, CharacterStatus: entity.StatusAlive})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/links/consent":
		_ = api.WriteJSON(w, http.StatusOK, api.ConsentResponse{LinkStatus: "consented"})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/characters":
		_ = api.WriteJSON(w, http.StatusCreated, api.CreateCharacterResponse{PlayerID: "player-1"})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/players/"):
		_ = api.WriteJSON(w, http.StatusOK, api.CharacterState{PlayerID: "player-1", Status: entity.StatusAlive,
			Position: &api.PositionRef{Kind: "outside", ID: position}})
	default:
		http.NotFound(w, r)
	}
}

// set changes what the fake answers while its server runs.
func (f *fakeAPI) set(change func(f *fakeAPI)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f)
}

func (f *fakeAPI) poll(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	var next []api.Delivery
	if len(f.deliveries) > 0 {
		next, f.deliveries = f.deliveries[0], f.deliveries[1:]
	}
	f.mu.Unlock()
	if len(next) == 0 && f.pollWait > 0 {
		<-clock.RealTimers{}.After(f.pollWait).C()
	}
	cursor := strconv.Itoa(len(f.polls()))
	_ = api.WriteJSON(w, http.StatusOK, api.DeliveriesResponse{Deliveries: append([]api.Delivery{}, next...), Cursor: cursor})
}

func (f *fakeAPI) polls() []*http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*http.Request
	for _, r := range f.requests {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/deliveries") {
			out = append(out, r)
		}
	}
	return out
}

func httpFixtures() []*entity.Entity {
	return []*entity.Entity{
		{ID: "w", Type: entity.TypeWorld, WorldID: "w", Name: "Мир"},
		{ID: "player-A", Type: entity.TypePlayer, WorldID: "w", Name: "Вася"},
	}
}

func harnessOver(t *testing.T, f *fakeAPI) *gateway.HTTPHarness {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	h, err := gateway.NewHTTPHarness(gateway.NewCIClient(srv.URL), httpFixtures())
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestNewHTTPHarnessRefusesWhatItCannotPlay(t *testing.T) {
	world := &entity.Entity{ID: "w", Type: entity.TypeWorld}
	player := &entity.Entity{ID: "player-A", Type: entity.TypePlayer}
	c := gateway.NewCIClient("http://127.0.0.1:1")
	cases := map[string]struct {
		c        *client.Client
		fixtures []*entity.Entity
	}{
		"no client":    {nil, []*entity.Entity{world}},
		"no world":     {c, []*entity.Entity{player}},
		"two worlds":   {c, []*entity.Entity{world, world}},
		"nil fixture":  {c, []*entity.Entity{world, nil}},
		"no id":        {c, []*entity.Entity{world, {Type: entity.TypePlayer}}},
		"player twice": {c, []*entity.Entity{world, player, player}},
		"no fixtures":  {c, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := gateway.NewHTTPHarness(tc.c, tc.fixtures); err == nil {
				t.Fatal("built a harness that cannot play")
			}
		})
	}
	h, err := gateway.NewHTTPHarness(c, []*entity.Entity{world, player})
	if err != nil || h.WorldID() != "w" || h.Client() != c {
		t.Errorf("harness = %v %v", h, err)
	}
}

// The client of the harness is ci-harness with X-Actor-Kind ci and no repeats.
func TestTheCIClientIsTheHarness(t *testing.T) {
	c := gateway.NewCIClient("http://127.0.0.1:1/")
	if c.ClientID != "ci-harness" || c.ActorKind != api.ActorCI || c.Backoff != client.NoRetry || c.BaseURL != "http://127.0.0.1:1" {
		t.Errorf("client = %+v", c)
	}
}

// Act fills an empty action_key with the next key of the harness, keeps a key
// the test gave, and answers the correlation of 202 accepted and 202 pending.
func TestActAnswersTheCorrelationOfTheAction(t *testing.T) {
	f := &fakeAPI{actionStatus: http.StatusAccepted,
		actionBody: `{"status":"accepted","correlation_id":"c-1","turn":{"seq":1,"session_id":"s"},"acked_at":"2026-09-14T10:00:00Z"}`}
	h := harnessOver(t, f)
	ctx := context.Background()
	for i, key := range []string{"", "", "mine"} {
		corr, err := h.Act(ctx, "player-1", api.ActionRequest{ActionKey: key, Type: api.ActionLook})
		if err != nil || corr != "c-1" {
			t.Fatalf("Act %d = %q %v", i, corr, err)
		}
	}
	var keys []string
	f.mu.Lock()
	bodies, first := slices.Clone(f.bodies), f.requests[0]
	f.mu.Unlock()
	for _, b := range bodies {
		var req api.ActionRequest
		if json.Unmarshal([]byte(b), &req) == nil && req.Type != "" {
			keys = append(keys, req.ActionKey)
		}
	}
	if !slices.Equal(keys, []string{"ci-1", "ci-2", "mine"}) {
		t.Errorf("action keys = %v", keys)
	}
	if got := first.Header.Get(api.HeaderActorKind); got != api.ActorCI {
		t.Errorf("X-Actor-Kind = %q", got)
	}

	f.set(func(f *fakeAPI) { f.actionBody = `{"status":"pending","group_id":"g-1","correlation_id":"c-2"}` })
	if corr, err := h.Act(ctx, "player-1", api.ActionRequest{Type: api.ActionGroupCreate}); err != nil || corr != "c-2" {
		t.Errorf("pending Act = %q %v", corr, err)
	}
	f.set(func(f *fakeAPI) {
		f.actionStatus, f.actionBody = http.StatusOK, `{"group_id":"g-1","leader_id":null,"members":[]}`
	})
	if corr, err := h.Act(ctx, "player-1", api.ActionRequest{Type: api.ActionGroupJoin}); err == nil {
		t.Errorf("200 group view = %q, want an error without a correlation", corr)
	}
	f.set(func(f *fakeAPI) {
		f.actionStatus, f.actionBody = http.StatusConflict, `{"error":{"code":"not_in_encounter","message":"m"}}`
	})
	_, err := h.Act(ctx, "player-1", api.ActionRequest{Type: api.ActionFlee})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != api.CodeNotInEncounter {
		t.Errorf("refused Act = %v, want the APIError of the gateway", err)
	}
}

// AwaitDelivery acknowledges everything a poll gives, returns the first of the
// kind, keeps the others for a later call, and polls after the last cursor.
func TestAwaitDeliveryAcknowledgesAndKeepsTheRest(t *testing.T) {
	f := &fakeAPI{deliveries: [][]api.Delivery{
		nil,
		{{ID: "d-1", Kind: "mechanics", CorrelationID: "c-1"}, {ID: "d-2", Kind: "system", CorrelationID: "c-0"}},
		{{ID: "d-3", Kind: "narrative", CorrelationID: "c-1"}},
	}}
	h := harnessOver(t, f)
	ctx := context.Background()

	d, err := h.AwaitDelivery(ctx, "mechanics", 5*time.Second)
	if err != nil || d.ID != "d-1" {
		t.Fatalf("mechanics = %+v %v", d, err)
	}
	d, err = h.AwaitDeliveryOf(ctx, "c-1", "narrative", 5*time.Second)
	if err != nil || d.ID != "d-3" {
		t.Fatalf("narrative = %+v %v", d, err)
	}
	pollsBefore := len(f.polls())
	d, err = h.AwaitDelivery(ctx, "system", 5*time.Second)
	if err != nil || d.ID != "d-2" || len(f.polls()) != pollsBefore {
		t.Errorf("system from the inbox = %+v %v, polls %d → %d", d, err, pollsBefore, len(f.polls()))
	}
	f.mu.Lock()
	acked := slices.Clone(f.acked)
	f.mu.Unlock()
	if !slices.EqualFunc(acked, [][]string{{"d-1", "d-2"}, {"d-3"}}, slices.Equal) {
		t.Errorf("acked = %v", acked)
	}
	polls := f.polls()
	if polls[0].URL.Query().Get("after") != "" || polls[1].URL.Query().Get("after") != "1" || polls[2].URL.Query().Get("after") != "2" {
		t.Errorf("after of the polls = %q %q %q", polls[0].URL.Query().Get("after"), polls[1].URL.Query().Get("after"), polls[2].URL.Query().Get("after"))
	}
	if q := polls[0].URL.Query(); q.Get("limit") != "100" || q.Get("wait_ms") == "" || polls[0].URL.Path != "/v1/clients/ci-harness/deliveries" {
		t.Errorf("poll = %s", polls[0].URL)
	}
}

// AwaitDelivery does not hang: without the kind it returns ErrNoDelivery after
// its timeout, naming the kind and what did arrive, and never asks the gateway
// to wait past the deadline.
func TestAwaitDeliveryTimesOutWithAClearError(t *testing.T) {
	f := &fakeAPI{pollWait: 20 * time.Millisecond, deliveries: [][]api.Delivery{{{ID: "d-1", Kind: "system", CorrelationID: "c-9"}}}}
	h := harnessOver(t, f)
	wall := clock.Real{}
	began := wall.Now()
	// The context bounds the test, not the harness: a harness that ignored its
	// timeout fails here in seconds instead of hanging until go test gives up.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := h.AwaitDelivery(ctx, "narrative", 200*time.Millisecond)
	took := wall.Now().Sub(began)
	if !errors.Is(err, gateway.ErrNoDelivery) || !strings.Contains(err.Error(), "narrative") || !strings.Contains(err.Error(), "system/c-9") {
		t.Fatalf("AwaitDelivery = %v", err)
	}
	if took < 200*time.Millisecond || took > 2*time.Second {
		t.Errorf("returned after %s", took)
	}
	for _, p := range f.polls() {
		if ms, _ := strconv.Atoi(p.URL.Query().Get("wait_ms")); ms > 200 {
			t.Errorf("wait_ms %d past the timeout", ms)
		}
	}
}

// Calls of AwaitDelivery on one harness wait for each other, and each counts
// its timeout from its own call (review #1 of T-308, Mi-4): a call that queued
// behind a long one returns at its own deadline, not a timeout after it got
// its turn to poll.
func TestAwaitDeliveryCountsItsTimeoutFromTheCall(t *testing.T) {
	f := &fakeAPI{pollWait: 100 * time.Millisecond}
	h := harnessOver(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wall := clock.Real{}
	type returned struct {
		at  time.Time
		err error
	}
	first := make(chan returned, 1)
	go func() {
		_, err := h.AwaitDelivery(ctx, "narrative", 700*time.Millisecond)
		first <- returned{wall.Now(), err}
	}()
	for len(f.polls()) == 0 {
		<-clock.RealTimers{}.After(5 * time.Millisecond).C()
	}
	_, err := h.AwaitDelivery(ctx, "mechanics", 300*time.Millisecond)
	second := wall.Now()
	if !errors.Is(err, gateway.ErrNoDelivery) {
		t.Fatalf("second call = %v", err)
	}
	r := <-first
	if !errors.Is(r.err, gateway.ErrNoDelivery) {
		t.Error("first call did not time out")
	}
	// The moments are compared with each other, not with the start of the
	// goroutines (review #2 of T-308, N-1): past its deadline when it gets the
	// poll, the second call polls no more and returns right after the first;
	// counted from the lock, its 300 ms would take at least three polls of
	// 100 ms more.
	if gap := second.Sub(r.at); gap > 200*time.Millisecond {
		t.Errorf("second call returned %s after the first, want at most 200 ms", gap)
	}
}

// A poll that fails, an ack that fails and an ack that does not know what the
// poll just gave end the wait with the reason.
func TestAwaitDeliveryReportsTheGateway(t *testing.T) {
	f := &fakeAPI{deliveries: [][]api.Delivery{{{ID: "d-1", Kind: "mechanics"}, {ID: "d-2", Kind: "narrative"}}}, unknownAck: []string{"d-1"}}
	h := harnessOver(t, f)
	if _, err := h.AwaitDelivery(context.Background(), "mechanics", time.Second); err == nil || !strings.Contains(err.Error(), "d-1") {
		t.Errorf("unknown ack = %v", err)
	}
	// N-2 of review #1: what the gateway did acknowledge is not lost with the
	// error.
	if d, err := h.AwaitDelivery(context.Background(), "narrative", time.Second); err != nil || d.ID != "d-2" {
		t.Errorf("the known delivery of the same poll = %+v %v", d, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = api.WriteError(w, api.NewError(api.CodePollInProgress, nil))
	}))
	t.Cleanup(srv.Close)
	h, err := gateway.NewHTTPHarness(gateway.NewCIClient(srv.URL), httpFixtures())
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.AwaitDelivery(context.Background(), "mechanics", time.Second)
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != api.CodePollInProgress {
		t.Errorf("failed poll = %v", err)
	}
}

// Drain gives what waits in the inbox first, then everything the long-poll
// gives until a poll of the quiet wait comes back empty; everything is
// acknowledged, the inbox is left empty, and the quiet wait is what the gateway
// is asked to wait.
func TestDrainTakesTheInboxAndEverythingUntilAQuietPoll(t *testing.T) {
	f := &fakeAPI{pollWait: 10 * time.Millisecond, deliveries: [][]api.Delivery{
		{{ID: "d-1", Kind: "mechanics", CorrelationID: "c-1"}, {ID: "d-2", Kind: "system", CorrelationID: "c-1"}},
		{{ID: "d-3", Kind: "narrative", CorrelationID: "c-1"}},
		{{ID: "d-4", Kind: "narrative", CorrelationID: "c-2"}},
	}}
	h := harnessOver(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if d, err := h.AwaitDelivery(ctx, "mechanics", time.Second); err != nil || d.ID != "d-1" {
		t.Fatalf("mechanics = %+v %v", d, err)
	}

	got, err := h.Drain(ctx, 30*time.Millisecond)
	ids := make([]string, 0, len(got))
	for _, d := range got {
		ids = append(ids, d.ID)
	}
	if err != nil || !slices.Equal(ids, []string{"d-2", "d-3", "d-4"}) {
		t.Fatalf("Drain = %v %v, want d-2 d-3 d-4", ids, err)
	}
	f.mu.Lock()
	acked := slices.Clone(f.acked)
	f.mu.Unlock()
	if !slices.EqualFunc(acked, [][]string{{"d-1", "d-2"}, {"d-3"}, {"d-4"}}, slices.Equal) {
		t.Errorf("acked = %v", acked)
	}
	polls := f.polls()
	if len(polls) != 4 {
		t.Fatalf("polls = %d, want 4: the await, two that gave, one quiet", len(polls))
	}
	if ms, _ := strconv.Atoi(polls[3].URL.Query().Get("wait_ms")); ms != 30 {
		t.Errorf("wait_ms of the quiet poll = %d, want 30", ms)
	}
	if again, err := h.Drain(ctx, -time.Second); err != nil || len(again) != 0 {
		t.Errorf("second Drain = %v %v, want nothing", again, err)
	}
	if polls := f.polls(); polls[len(polls)-1].URL.Query().Get("wait_ms") != "0" {
		t.Errorf("a quiet below zero polled with wait_ms %q, want 0", polls[len(polls)-1].URL.Query().Get("wait_ms"))
	}
	if _, err := h.AwaitDelivery(ctx, "system", 20*time.Millisecond); !errors.Is(err, gateway.ErrNoDelivery) {
		t.Errorf("the inbox after Drain answered %v, want nothing", err)
	}

	f.set(func(f *fakeAPI) {
		f.deliveries = [][]api.Delivery{{{ID: "d-5", Kind: "mechanics"}, {ID: "d-6", Kind: "narrative"}}}
		f.unknownAck = []string{"d-5"}
	})
	if got, err := h.Drain(ctx, 10*time.Millisecond); err == nil || got != nil || !strings.Contains(err.Error(), "d-5") {
		t.Errorf("Drain with an unknown ack = %v %v, want the error and nothing", got, err)
	}
	// What the gateway did acknowledge is not lost with the error.
	f.set(func(f *fakeAPI) { f.unknownAck = nil })
	if got, err := h.Drain(ctx, 10*time.Millisecond); err != nil || len(got) != 1 || got[0].ID != "d-6" {
		t.Errorf("Drain after the error = %v %v, want d-6 from the inbox", got, err)
	}
}

// CloseRound: the bare 404 of a mux without the route is 501 not_implemented;
// an answer of the route itself — 409 no_open_round, 200 — passes as it came.
func TestCloseRoundIsAStubUntilTheRouteIsMounted(t *testing.T) {
	f := &fakeAPI{}
	h := harnessOver(t, f)
	ctx := context.Background()
	_, err := h.CloseRound(ctx, "group:g-1")
	if !errors.Is(err, gateway.ErrRoundsNotMounted) || !strings.Contains(err.Error(), "404") {
		t.Errorf("unmounted = %v, want ErrRoundsNotMounted with the answer of the server", err)
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotImplemented || apiErr.Code != api.CodeNotImplemented {
		t.Fatalf("unmounted = %v, want 501 not_implemented", err)
	}
	// N-1 of review #1: a caller that changes what it read changes nobody else.
	apiErr.Status, apiErr.Code = http.StatusTeapot, "changed"
	var again *client.APIError
	if _, err := h.CloseRound(ctx, "group:g-1"); !errors.As(err, &again) || again.Status != http.StatusNotImplemented || again.Code != api.CodeNotImplemented {
		t.Errorf("after a caller changed its copy = %v", err)
	}

	f.set(func(f *fakeAPI) {
		f.closeRound = func(w http.ResponseWriter, r *http.Request) {
			_ = api.WriteError(w, api.NewError(api.CodeNoOpenRound, nil))
		}
	})
	_, err = h.CloseRound(ctx, "group:g-1")
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Code != api.CodeNoOpenRound {
		t.Errorf("no open round = %v, want 409 as the gateway gave it", err)
	}
	f.set(func(f *fakeAPI) {
		f.closeRound = func(w http.ResponseWriter, r *http.Request) {
			_ = api.WriteJSON(w, http.StatusOK, api.RoundCloseResponse{Round: api.ClosedRound{Seq: 3, CloseReason: "explicit"}})
		}
	})
	if res, err := h.CloseRound(ctx, "group:g-1"); err != nil || res.Round.Seq != 3 {
		t.Errorf("closed = %+v %v", res, err)
	}
	f.set(func(f *fakeAPI) {
		f.closeRound = func(w http.ResponseWriter, r *http.Request) {
			_ = api.WriteError(w, api.NewError(api.CodePlayerNotFound, nil))
		}
	})
	if _, err := h.CloseRound(ctx, "group:g-1"); errors.Is(err, gateway.ErrRoundsNotMounted) || !errors.As(err, &apiErr) || apiErr.Code != api.CodePlayerNotFound {
		t.Errorf("a 404 with a code = %v, want it as the gateway gave it", err)
	}
}

// RegisterAndEnter plays only the players of the fixtures, links them as
// external accounts of the fixture id, and does not wait past its timeout for a
// character that never arrives.
func TestRegisterAndEnterStopsAtItsTimeout(t *testing.T) {
	f := &fakeAPI{actionStatus: http.StatusAccepted, position: "outside:w",
		actionBody: `{"status":"accepted","correlation_id":"c-1","turn":{"seq":1,"session_id":"s"},"acked_at":"2026-09-14T10:00:00Z"}`}
	h := harnessOver(t, f)
	manual := clock.NewManual(time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC))
	h.WithTimeout(300 * time.Millisecond).WithClock(manual)
	ctx := context.Background()
	if _, err := h.RegisterAndEnter(ctx, "player-B", "r-1"); err == nil {
		t.Error("RegisterAndEnter of a character outside the fixtures succeeded")
	}
	wall := clock.Real{}
	began := wall.Now()
	_, err := h.RegisterAndEnter(ctx, "player-A", "r-1")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "did not arrive in r-1") {
		t.Errorf("RegisterAndEnter = %v, want the deadline", err)
	}
	if took := wall.Now().Sub(began); took > 2*time.Second {
		t.Errorf("returned after %s", took)
	}
	if id, ok := h.PlayerID("player-A"); !ok || id != "player-1" {
		t.Errorf("PlayerID = %q %v", id, ok)
	}
	var consent api.ConsentRequest
	var create api.CreateCharacterRequest
	f.mu.Lock()
	requests, bodies := slices.Clone(f.requests), slices.Clone(f.bodies)
	f.mu.Unlock()
	for i, r := range requests {
		switch r.URL.Path {
		case "/v1/links/consent":
			_ = json.Unmarshal([]byte(bodies[i]), &consent)
		case "/v1/characters":
			_ = json.Unmarshal([]byte(bodies[i]), &create)
		}
	}
	if consent.ExternalPlatform != gateway.Platform || consent.ExternalID != "player-A" || !consent.ShownAt.Equal(manual.Now()) ||
		!consent.Consent || !consent.AgeConfirmed || !consent.NoticeShown {
		t.Errorf("consent = %+v", consent)
	}
	if create.ExternalID != "player-A" || create.CharacterName != "Вася" || create.WorldID != "w" || create.ActionKey == "" {
		t.Errorf("create = %+v", create)
	}
}
