package flow_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/flow"
	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/logging"
)

const (
	playerUser = int64(424242001)
	username   = "VasyaTG"
	firstName  = "Василий"
)

var (
	epoch = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	salt  = []byte("test-salt-of-action-keys-0123456789")
)

// call is one request as the gateway mock saw it.
type call struct {
	Method, Path string
	Body         []byte
}

func (c call) key() string { return c.Method + " " + c.Path }

// rawBody is a body written as it is, not as JSON: a page of a proxy.
type rawBody string

// answer is a scripted response: a status, a body and headers. A nil body
// with status 0 drops the connection, a network error.
type answer struct {
	status int
	body   any
	header map[string]string
}

// gatewayMock is the gateway over httptest: routes answer by script, every
// request is recorded, and the flow reaches it through the real client.
type gatewayMock struct {
	t      *testing.T
	srv    *httptest.Server
	mu     sync.Mutex
	calls  []call
	routes map[string][]answer
}

func newGatewayMock(t *testing.T) *gatewayMock {
	t.Helper()
	g := &gatewayMock{t: t, routes: make(map[string][]answer)}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.srv.Close)
	return g
}

// on scripts the answers of a route; the last one repeats.
func (g *gatewayMock) on(route string, answers ...answer) {
	g.mu.Lock()
	g.routes[route] = answers
	g.mu.Unlock()
}

func (g *gatewayMock) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	c := call{Method: r.Method, Path: r.URL.Path, Body: body}
	g.mu.Lock()
	g.calls = append(g.calls, c)
	script := g.routes[c.key()]
	var a answer
	switch {
	case len(script) == 0:
		g.mu.Unlock()
		g.t.Errorf("unexpected request %s", c.key())
		http.Error(w, "unexpected", http.StatusTeapot)
		return
	case len(script) == 1:
		a = script[0]
	default:
		a, g.routes[c.key()] = script[0], script[1:]
	}
	g.mu.Unlock()
	if a.status == 0 {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
		return
	}
	for k, v := range a.header {
		w.Header().Set(k, v)
	}
	if raw, ok := a.body.(rawBody); ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(a.status)
		_, _ = io.WriteString(w, string(raw))
		return
	}
	w.Header().Set("Content-Type", api.ContentTypeJSON)
	w.WriteHeader(a.status)
	_ = json.NewEncoder(w).Encode(a.body)
}

func (g *gatewayMock) count(route string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, c := range g.calls {
		if c.key() == route {
			n++
		}
	}
	return n
}

func (g *gatewayMock) total() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.calls)
}

func (g *gatewayMock) last(route string) call {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := len(g.calls) - 1; i >= 0; i-- {
		if g.calls[i].key() == route {
			return g.calls[i]
		}
	}
	g.t.Fatalf("no request %s", route)
	return call{}
}

// Routes of the gateway.
const (
	routeResolve = "POST /v1/links/resolve"
	routeConsent = "POST /v1/links/consent"
	routeForget  = "DELETE /v1/links"
	routeWorlds  = "GET /v1/worlds"
	routeCreate  = "POST /v1/characters"
	routePlayer  = "GET /v1/players/player-A"
	routeActions = "POST /v1/players/player-A/actions"
)

func ptr[T any](v T) *T { return &v }

func resolved(link, character string, playerID *string) answer {
	return answer{status: http.StatusOK, body: api.ResolveResponse{LinkStatus: link, CharacterStatus: character, PlayerID: playerID,
		WorldID: ptr("dark-forest-world"), NoticeDue: link != "consented"}}
}

var (
	resolvedNone      = resolved("pending_consent", "none", nil)
	resolvedConsented = resolved("consented", "none", nil)
	resolvedAlive     = resolved("consented", "alive", ptr("player-A"))
)

func apiError(status int, code string, header map[string]string) answer {
	spec, _ := api.LookupError(code)
	return answer{status: status, body: api.ErrorResponse{Error: api.ErrorBody{Code: code, Message: spec.Message}}, header: header}
}

var oneWorld = answer{status: http.StatusOK, body: api.WorldsResponse{Worlds: []api.WorldSummary{{
	WorldID: "dark-forest-world", Name: "Тёмный лес", LawsVersion: "v1",
	Regions: []api.RegionSummary{{RegionID: "dark-forest-01", Name: "Тёмный лес"}},
}}}}

func aliveCharacter(name string, npcs ...api.NPCView) api.CharacterState {
	hp, hpMax := 10, 10
	st := api.CharacterState{PlayerID: "player-A", Name: name, WorldID: "dark-forest-world", Status: "alive", HP: &hp, HPMax: &hpMax,
		Position: &api.PositionRef{Kind: "outside", ID: "outside:dark-forest-world"}}
	if len(npcs) > 0 {
		st.Encounter = &api.EncounterView{EncounterID: "enc-1", NPCs: npcs}
	}
	return st
}

var accepted = answer{status: http.StatusAccepted, body: api.ActionAccepted{CorrelationID: "ev-1", Status: api.ActionStatusAccepted, Turn: api.TurnRef{Seq: 1, SessionID: "s-1"}}}

// armedTimers wraps the timers of a manual clock and reports every timer it
// arms, so that a test can move the clock once the flow waits.
type armedTimers struct {
	inner clock.Timers
	armed chan time.Duration
}

func (a armedTimers) After(d time.Duration) clock.Timer {
	t := a.inner.After(d)
	a.armed <- d
	return t
}

func (a armedTimers) Every(d time.Duration) clock.Timer { return a.inner.Every(d) }

type fixture struct {
	t      *testing.T
	gw     *gatewayMock
	client *client.Client
	sent   *sender.Fake
	clock  *clock.Manual
	armed  chan time.Duration
	logs   *bytes.Buffer
	flow   *flow.Flow
	nextID int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, gw: newGatewayMock(t), sent: &sender.Fake{}, clock: clock.NewManual(epoch), logs: &bytes.Buffer{},
		armed: make(chan time.Duration, 16), nextID: 1000}
	f.client = client.New(f.gw.srv.URL, "telegram-bot")
	f.client.Backoff = client.NoRetry
	f.client.HTTP = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 5 * time.Second}
	inner := logging.New(logging.Options{Service: "telegram-bot", Level: slog.LevelDebug, Output: f.logs}).Handler()
	fl, err := flow.New(flow.Options{
		Gateway:       f.client,
		Sender:        f.sent,
		Clock:         f.clock,
		Timers:        armedTimers{inner: f.clock.Timers(), armed: f.armed},
		ActionKeySalt: salt,
		Log:           slog.New(privacy.NewHandler(inner, privacy.NewRedactor())),
	})
	if err != nil {
		t.Fatalf("flow.New: %v", err)
	}
	f.flow = fl
	return f
}

func (f *fixture) update(text string) updates.Update {
	f.nextID++
	return updates.Update{ID: f.nextID, Message: &updates.Message{
		Chat: updates.Chat{ID: playerUser, Type: updates.ChatPrivate},
		From: &updates.User{ID: playerUser, Username: username, FirstName: firstName},
		Text: text,
	}}
}

// send handles one message and returns the messages the bot answered with.
func (f *fixture) send(text string) []sender.Message {
	f.t.Helper()
	before := len(f.sent.Sent())
	f.flow.Handle(context.Background(), f.update(text))
	return f.sent.Sent()[before:]
}

func (f *fixture) texts(msgs []sender.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Text
	}
	return out
}

// onboard brings the chat to a character: /start, the consent, the name.
func (f *fixture) onboard() {
	f.t.Helper()
	f.gw.on(routeResolve, resolvedNone)
	f.send("/start")
	f.gw.on(routeConsent, answer{status: http.StatusOK, body: api.ConsentResponse{LinkStatus: "consented"}})
	f.gw.on(routeResolve, resolvedConsented)
	f.send(render.ConsentButton)
	f.gw.on(routeWorlds, oneWorld)
	st := aliveCharacter("Вася")
	f.gw.on(routeCreate, answer{status: http.StatusCreated, body: api.CreateCharacterResponse{PlayerID: "player-A", Character: &st, Created: ptr(true)}})
	f.send("Вася")
	if step := f.flow.StepOf(playerUser); step != flow.Ready {
		f.t.Fatalf("after the onboarding the step is %s, want ready", step)
	}
}

func externalID() string { return strconv.FormatInt(playerUser, 10) }
