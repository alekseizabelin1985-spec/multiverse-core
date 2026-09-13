package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
)

const testToken = "123456:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789"

// Telegram user ids of the tests: a stranger outside the allow-list and two
// players on it. Each is long enough that a leak into output is no accident.
const (
	stranger = int64(700000000111)
	playerA  = int64(700000000222)
	playerB  = int64(700000000444)
	// deliveryChat is where the gateway sends a delivery.
	deliveryChat = int64(700000000333)
)

var epoch = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

// syncBuffer is a writer the process and the test may use at once.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// botAPI is the Bot API on httptest. getMe answers by getMe; getUpdates takes
// the next answer of polls and, once they ran out, holds the request until the
// client leaves; sendMessage answers by the script of its chat, whose last
// answer repeats, or holds the request when the chat is in hang.
type botAPI struct {
	t     *testing.T
	srv   *httptest.Server
	mu    sync.Mutex
	getMe apiAnswer
	// getMeDelay holds the answer of getMe, so that a component started
	// before it has the time to show.
	getMeDelay time.Duration
	polls      []apiAnswer
	sends      map[string][]apiAnswer
	hang       map[string]bool
	tries      map[string]int
	calls      map[string]int
	// released ends every held request; the cleanup closes it before the
	// server, whose Close waits for them.
	released chan struct{}
}

type apiAnswer struct {
	status int
	body   string
}

func ok(result string) apiAnswer {
	return apiAnswer{http.StatusOK, `{"ok":true,"result":` + result + `}`}
}

var (
	sentOK       = ok(`{"message_id":1,"date":1,"chat":{"id":1,"type":"private"}}`)
	unauthorized = apiAnswer{http.StatusUnauthorized, `{"ok":false,"error_code":401,"description":"Unauthorized"}`}
	conflict     = apiAnswer{http.StatusConflict, `{"ok":false,"error_code":409,"description":"Conflict: terminated by other getUpdates request; make sure that only one bot instance is running"}`}
	serverError  = apiAnswer{http.StatusInternalServerError, `{"ok":false,"error_code":500,"description":"Internal Server Error"}`}
)

func newBotAPI(t *testing.T) *botAPI {
	b := &botAPI{
		t:        t,
		getMe:    ok(`{"id":123456,"is_bot":true,"first_name":"bot"}`),
		sends:    map[string][]apiAnswer{},
		hang:     map[string]bool{},
		tries:    map[string]int{},
		calls:    map[string]int{},
		released: make(chan struct{}),
	}
	b.srv = httptest.NewServer(http.HandlerFunc(b.serve))
	t.Cleanup(func() {
		close(b.released)
		b.srv.Close()
	})
	return b
}

// updatesOf is the body of a getUpdates answer with one text message per
// sender, in a private chat of that sender.
func updatesOf(texts ...msg) apiAnswer {
	var parts []string
	for i, m := range texts {
		id := strconv.FormatInt(m.from, 10)
		parts = append(parts, `{"update_id":`+strconv.Itoa(1000+i)+`,"message":{"message_id":`+strconv.Itoa(i+1)+
			`,"date":1,"chat":{"id":`+id+`,"type":"private"},"from":{"id":`+id+`,"is_bot":false,"first_name":"Игрок"},"text":`+strconv.Quote(m.text)+`}}`)
	}
	return ok("[" + strings.Join(parts, ",") + "]")
}

// msg is one text message of a player for updatesOf.
type msg struct {
	from int64
	text string
}

func (b *botAPI) serve(w http.ResponseWriter, r *http.Request) {
	prefix := "/bot" + testToken + "/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	method := strings.TrimPrefix(r.URL.Path, prefix)
	_ = r.ParseMultipartForm(1 << 20)
	b.mu.Lock()
	b.calls[method]++
	var (
		answer *apiAnswer
		held   bool
		delay  time.Duration
	)
	switch method {
	case "getMe":
		a := b.getMe
		answer = &a
		delay = b.getMeDelay
	case "getUpdates":
		if len(b.polls) > 0 {
			a := b.polls[0]
			b.polls = b.polls[1:]
			answer = &a
		}
	case "sendMessage":
		chat := r.FormValue("chat_id")
		b.tries[chat]++
		if b.hang[chat] {
			held = true
			break
		}
		a := sentOK
		if script := b.sends[chat]; len(script) == 1 {
			a = script[0]
		} else if len(script) > 1 {
			a, b.sends[chat] = script[0], script[1:]
		}
		answer = &a
	default:
		b.t.Errorf("unexpected Bot API method %s", method)
	}
	b.mu.Unlock()
	if answer == nil || held {
		hold(r, b.released)
		return
	}
	time.Sleep(delay)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(answer.status)
	_, _ = io.WriteString(w, answer.body)
}

func (b *botAPI) triesOf(chat int64) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tries[strconv.FormatInt(chat, 10)]
}

func (b *botAPI) callsOf(method string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls[method]
}

// gatewayMock is the gateway on httptest: links/resolve answers «no link»
// unless the external id is told to hang; the long-poll hands out the
// deliveries given to it once and then holds until the client leaves; every
// request is recorded with its arrival.
type gatewayMock struct {
	t          *testing.T
	srv        *httptest.Server
	mu         sync.Mutex
	hangFor    map[string]bool
	deliveries []api.Delivery
	resolved   map[string]chan struct{}
	requests   []string
	released   chan struct{}
}

func newGatewayMock(t *testing.T) *gatewayMock {
	g := &gatewayMock{t: t, hangFor: map[string]bool{}, resolved: map[string]chan struct{}{}, released: make(chan struct{})}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(func() {
		close(g.released)
		g.srv.Close()
	})
	return g
}

// resolvedCh closes when links/resolve for the external id arrives.
func (g *gatewayMock) resolvedCh(user int64) chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	id := strconv.FormatInt(user, 10)
	ch, found := g.resolved[id]
	if !found {
		ch = make(chan struct{})
		g.resolved[id] = ch
	}
	return ch
}

func (g *gatewayMock) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	g.mu.Lock()
	g.requests = append(g.requests, r.Method+" "+r.URL.Path)
	g.mu.Unlock()
	w.Header().Set("Content-Type", api.ContentTypeJSON)
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/links/resolve":
		var req api.ResolveRequest
		_ = json.Unmarshal(body, &req)
		ch := g.resolvedCh(mustParse(g.t, req.ExternalID))
		g.mu.Lock()
		select {
		case <-ch:
		default:
			close(ch)
		}
		hang := g.hangFor[req.ExternalID]
		g.mu.Unlock()
		if hang {
			hold(r, g.released)
			return
		}
		_ = json.NewEncoder(w).Encode(api.ResolveResponse{LinkStatus: "none", CharacterStatus: "none"})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/clients/"+ClientID+"/deliveries":
		g.mu.Lock()
		ds := g.deliveries
		g.deliveries = nil
		g.mu.Unlock()
		if len(ds) == 0 {
			hold(r, g.released)
			return
		}
		_ = json.NewEncoder(w).Encode(api.DeliveriesResponse{Deliveries: ds, Cursor: "1"})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/clients/"+ClientID+"/deliveries/ack":
		_ = json.NewEncoder(w).Encode(api.AckResponse{Unknown: []string{}})
	default:
		g.t.Errorf("unexpected gateway request %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected", http.StatusTeapot)
	}
}

func (g *gatewayMock) requestsSeen() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.requests)
}

// hold keeps a request open until its client leaves or the test ends.
func hold(r *http.Request, released <-chan struct{}) {
	select {
	case <-r.Context().Done():
	case <-released:
	}
}

func mustParse(t *testing.T, s string) int64 {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Errorf("external id %q is not a number", s)
	}
	return id
}

// instantTimers fires every pause at once.
type instantTimers struct{}

func (instantTimers) After(time.Duration) clock.Timer {
	ch := make(chan time.Time, 1)
	ch <- time.Time{}
	return chanTimer{ch}
}

func (instantTimers) Every(time.Duration) clock.Timer { panic("no periodic timer expected") }

type chanTimer struct{ ch chan time.Time }

func (c chanTimer) C() <-chan time.Time { return c.ch }
func (c chanTimer) Stop() bool          { return false }

// process is one run of the bot in a goroutine.
type process struct {
	stdout, stderr *syncBuffer
	addr           chan string
	code           chan int
	cancel         context.CancelFunc
}

// testLimits keep the production shape of the clients with timeouts a test
// can wait for; the long ones are an hour, so that a component that took the
// wrong client stalls visibly.
var testLimits = limits{
	TelegramTimeout:       time.Hour,
	FlowTelegramTimeout:   200 * time.Millisecond,
	GateTimeout:           200 * time.Millisecond,
	FlowGatewayTimeout:    200 * time.Millisecond,
	FlowGatewayBackoff:    productionLimits.FlowGatewayBackoff,
	DeliverGatewayTimeout: time.Hour,
}

// vars are the variables of a bot that polls bot and calls gw, admitting
// playerA and playerB.
func vars(bot *botAPI, gw *gatewayMock) map[string]string {
	return map[string]string{
		"MV_TELEGRAM_BOT_TOKEN":        testToken,
		"MV_TELEGRAM_ALLOWED_USER_IDS": strconv.FormatInt(playerA, 10) + "," + strconv.FormatInt(playerB, 10),
		"MV_TELEGRAM_GATEWAY_URL":      gw.srv.URL,
		"MV_TELEGRAM_POLL_TIMEOUT_S":   "1",
		"MV_TELEGRAM_HEALTH_ADDR":      "127.0.0.1:0",
		"MV_LOG_LEVEL":                 "debug",
	}
}

func testEnvironment(bot *botAPI, source map[string]string) (environment, *process) {
	p := &process{stdout: &syncBuffer{}, stderr: &syncBuffer{}, addr: make(chan string, 1), code: make(chan int, 1)}
	e := environment{
		source:      env.MapSource(source),
		stdout:      p.stdout,
		stderr:      p.stderr,
		clock:       clock.NewManual(epoch),
		timers:      instantTimers{},
		limits:      testLimits,
		telegramURL: bot.srv.URL,
		listening:   func(addr string) { p.addr <- addr },
	}
	return e, p
}

// start runs the bot in a goroutine.
func start(t *testing.T, e environment, p *process) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func() { p.code <- run(ctx, nil, e) }()
	t.Cleanup(func() {
		cancel()
		wait, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		select {
		case <-p.code:
		case <-wait.Done():
			t.Error("the bot did not stop within 10 s of the end of the test")
		}
	})
}

// exitCode waits up to 10 s for the process to stop.
func (p *process) exitCode(t *testing.T) int {
	t.Helper()
	wait, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	select {
	case code := <-p.code:
		p.code <- code
		return code
	case <-wait.Done():
		t.Fatalf("the bot did not stop within 10 s\nstdout:\n%s\nstderr:\n%s", p.stdout, p.stderr)
		return -1
	}
}

// waitUntil polls cond for up to 5 s of the wall clock.
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for !cond() {
		if deadline.Err() != nil {
			t.Fatalf("%s: not within 5 s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
