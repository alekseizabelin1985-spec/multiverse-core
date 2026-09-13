package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
)

// instantTimers fires every pause at once and remembers its length, so a test
// sees the backoff without waiting for it.
type instantTimers struct {
	mu     sync.Mutex
	pauses []time.Duration
}

func (it *instantTimers) After(d time.Duration) clock.Timer {
	it.mu.Lock()
	it.pauses = append(it.pauses, d)
	it.mu.Unlock()
	ch := make(chan time.Time, 1)
	ch <- time.Time{}
	return chanTimer{ch}
}

func (it *instantTimers) Every(time.Duration) clock.Timer { panic("client uses no periodic timer") }

func (it *instantTimers) Pauses() []time.Duration {
	it.mu.Lock()
	defer it.mu.Unlock()
	return slices.Clone(it.pauses)
}

// silentTimers never fire, so a pause lasts until the context ends; armed
// tells the test that the client is inside a pause.
type silentTimers struct{ armed chan struct{} }

func (s silentTimers) After(time.Duration) clock.Timer {
	s.armed <- struct{}{}
	return chanTimer{make(chan time.Time)}
}

func (silentTimers) Every(time.Duration) clock.Timer { panic("client uses no periodic timer") }

type chanTimer struct{ ch chan time.Time }

func (c chanTimer) C() <-chan time.Time { return c.ch }
func (c chanTimer) Stop() bool          { return false }

// recorded is one request as the fake gateway saw it.
type recorded struct {
	Method, Path, Query string
	Header              http.Header
	Body                []byte
}

// fakeGateway answers each request with the next reply; the last reply
// repeats. A reply with status 0 drops the connection without an answer, a
// reply with status holdReply answers nothing until the client goes away.
// hits receives one value per request, after it is recorded.
type fakeGateway struct {
	t       *testing.T
	mu      sync.Mutex
	replies []reply
	seen    []recorded
	srv     *httptest.Server
	hits    chan struct{}
}

const holdReply = -1

type reply struct {
	status int
	body   string
	header map[string]string
}

func newFakeGateway(t *testing.T, replies ...reply) *fakeGateway {
	t.Helper()
	g := &fakeGateway{t: t, replies: replies, hits: make(chan struct{}, 64)}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGateway) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	g.mu.Lock()
	g.seen = append(g.seen, recorded{Method: r.Method, Path: r.URL.EscapedPath(), Query: r.URL.RawQuery, Header: r.Header.Clone(), Body: body})
	rep := g.replies[min(len(g.seen), len(g.replies))-1]
	g.mu.Unlock()
	select {
	case g.hits <- struct{}{}:
	default:
	}
	if rep.status == holdReply {
		<-r.Context().Done()
		return
	}
	if rep.status == 0 {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			g.t.Errorf("hijack: %v", err)
			return
		}
		_ = conn.Close()
		return
	}
	for k, v := range rep.header {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", api.ContentTypeJSON)
	w.WriteHeader(rep.status)
	_, _ = io.WriteString(w, rep.body)
}

func (g *fakeGateway) Seen() []recorded {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.seen)
}

// newClient returns a client of g with instant pauses and no connection reuse,
// so that a dropped connection is one attempt and not a transport-level retry.
func newClient(g *fakeGateway) (*client.Client, *instantTimers) {
	c := client.New(g.srv.URL+"/", "telegram-bot")
	timers := &instantTimers{}
	c.Timers = timers
	c.HTTP = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 5 * time.Second}
	return c, timers
}

func errorBody(code, message string) string {
	data, _ := json.Marshal(api.ErrorResponse{Error: api.ErrorBody{Code: code, Message: message, Details: map[string]any{"group_id": "g-1"}}})
	return string(data)
}

func ptr[T any](v T) *T { return &v }

var attack = api.ActionRequest{ActionKey: "k-1", Type: api.ActionAttack, Target: ptr("wolf-alpha")}

const acceptedBody = `{"correlation_id":"ev-1","turn":{"seq":10,"session_id":"solo:player-A:1","round_seq":3},"status":"accepted","acked_at":"2026-09-13T10:00:00Z"}`

func TestActionSendsHeadersAndBodyAndDecodesAccepted(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusAccepted, body: acceptedBody})
	c, timers := newClient(g)
	c.ActorKind = api.ActorCI

	res, err := c.Action(context.Background(), "player-A", attack)
	if err != nil {
		t.Fatalf("Action: %v", err)
	}
	if res.Status != http.StatusAccepted || res.Accepted == nil || res.Pending != nil || res.Group != nil {
		t.Fatalf("result = %+v, want only Accepted", res)
	}
	if res.Accepted.CorrelationID != "ev-1" || res.Accepted.Turn.Seq != 10 || res.Accepted.Turn.RoundSeq == nil || *res.Accepted.Turn.RoundSeq != 3 {
		t.Errorf("accepted = %+v", res.Accepted)
	}

	seen := g.Seen()
	if len(seen) != 1 {
		t.Fatalf("requests = %d, want 1", len(seen))
	}
	req := seen[0]
	if req.Method != http.MethodPost || req.Path != "/v1/players/player-A/actions" {
		t.Errorf("request = %s %s", req.Method, req.Path)
	}
	for name, want := range map[string]string{
		api.HeaderClientID:  "telegram-bot",
		api.HeaderActorKind: api.ActorCI,
		"Content-Type":      api.ContentTypeJSON,
	} {
		if got := req.Header.Get(name); got != want {
			t.Errorf("header %s = %q, want %q", name, got, want)
		}
	}
	var sent api.ActionRequest
	if err := json.Unmarshal(req.Body, &sent); err != nil || sent.ActionKey != "k-1" || sent.Type != api.ActionAttack || *sent.Target != "wolf-alpha" {
		t.Errorf("body %s decoded %+v (%v)", req.Body, sent, err)
	}
	if strings.Contains(string(req.Body), `"text"`) {
		t.Errorf("body %s sends an absent text", req.Body)
	}
	if len(timers.Pauses()) != 0 {
		t.Errorf("pauses = %v, want none", timers.Pauses())
	}
}

func TestActionWithoutActorKindSendsNoHeader(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusAccepted, body: acceptedBody})
	c, _ := newClient(g)
	if _, err := c.Action(context.Background(), "player-A", attack); err != nil {
		t.Fatal(err)
	}
	if _, ok := g.Seen()[0].Header[api.HeaderActorKind]; ok {
		t.Error("X-Actor-Kind sent although the client has none; the gateway default is human")
	}
}

func TestActionDecodesPendingAndGroup(t *testing.T) {
	t.Run("202 pending", func(t *testing.T) {
		g := newFakeGateway(t, reply{status: http.StatusAccepted, body: `{"status":"pending","group_id":"g-1","correlation_id":"ev-2"}`})
		c, _ := newClient(g)
		res, err := c.Action(context.Background(), "player-A", api.ActionRequest{ActionKey: "k-2", Type: api.ActionGroupCreate})
		if err != nil {
			t.Fatal(err)
		}
		if res.Pending == nil || res.Accepted != nil || res.Pending.GroupID != "g-1" || res.Pending.CorrelationID != "ev-2" {
			t.Errorf("result = %+v, pending %+v", res, res.Pending)
		}
	})
	t.Run("200 group without leader", func(t *testing.T) {
		g := newFakeGateway(t, reply{status: http.StatusOK, body: `{"group_id":"g-1","leader_id":null,"members":[{"player_id":"player-A","name":"A","participation":"active"}]}`})
		c, _ := newClient(g)
		res, err := c.Action(context.Background(), "player-A", api.ActionRequest{ActionKey: "k-3", Type: api.ActionGroupJoin, Target: ptr("g-1")})
		if err != nil {
			t.Fatal(err)
		}
		if res.Status != http.StatusOK || res.Group == nil || res.Group.LeaderID != nil || len(res.Group.Members) != 1 {
			t.Errorf("result = %+v, group %+v", res, res.Group)
		}
	})
}

func TestClientErrorIsAPIErrorAndIsNotRepeated(t *testing.T) {
	g := newFakeGateway(t, reply{
		status: http.StatusConflict,
		body:   errorBody(api.CodeNoLeader, "У группы нет лидера."),
		header: map[string]string{api.HeaderRequestID: "req-1"},
	})
	c, timers := newClient(g)

	_, err := c.Action(context.Background(), "player-A", api.ActionRequest{ActionKey: "k-4", Type: api.ActionEnter, Target: ptr("dark-forest-01")})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Status != http.StatusConflict || apiErr.Code != api.CodeNoLeader || apiErr.RequestID != "req-1" || apiErr.Details["group_id"] != "g-1" {
		t.Errorf("APIError = %+v", apiErr)
	}
	if n := len(g.Seen()); n != 1 {
		t.Errorf("requests = %d, want 1: a 4xx is never repeated", n)
	}
	if len(timers.Pauses()) != 0 {
		t.Errorf("pauses = %v, want none", timers.Pauses())
	}
}

func TestUnavailableIsRepeatedThreeTimesWithTheSameActionKey(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "Сервис временно недоступен, повторите.")})
	c, timers := newClient(g)

	_, err := c.Action(context.Background(), "player-A", attack)
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != api.CodeBusUnavailable {
		t.Fatalf("err = %v, want APIError bus_unavailable after the last repeat", err)
	}
	seen := g.Seen()
	if len(seen) != 4 {
		t.Fatalf("requests = %d, want 4 (the attempt and 3 repeats)", len(seen))
	}
	for i, req := range seen {
		var sent api.ActionRequest
		if err := json.Unmarshal(req.Body, &sent); err != nil || sent.ActionKey != "k-1" {
			t.Errorf("request %d: action_key %q (%v), want k-1", i, sent.ActionKey, err)
		}
		if string(req.Body) != string(seen[0].Body) {
			t.Errorf("request %d body %s differs from the first %s", i, req.Body, seen[0].Body)
		}
	}
	want := []time.Duration{200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond}
	if got := timers.Pauses(); !slices.Equal(got, want) {
		t.Errorf("pauses = %v, want %v", got, want)
	}
}

func TestUnavailableThenAcceptedSucceeds(t *testing.T) {
	unavailable := reply{status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "")}
	g := newFakeGateway(t, unavailable, unavailable, reply{status: http.StatusAccepted, body: acceptedBody})
	c, timers := newClient(g)

	res, err := c.Action(context.Background(), "player-A", attack)
	if err != nil || res.Accepted == nil {
		t.Fatalf("Action = %+v, %v", res, err)
	}
	if n := len(g.Seen()); n != 3 {
		t.Errorf("requests = %d, want 3", n)
	}
	if got := len(timers.Pauses()); got != 2 {
		t.Errorf("pauses = %d, want 2", got)
	}
}

func TestNetworkErrorIsRepeated(t *testing.T) {
	t.Run("then accepted", func(t *testing.T) {
		g := newFakeGateway(t, reply{}, reply{}, reply{status: http.StatusAccepted, body: acceptedBody})
		c, timers := newClient(g)
		res, err := c.Action(context.Background(), "player-A", attack)
		if err != nil || res.Accepted == nil {
			t.Fatalf("Action = %+v, %v", res, err)
		}
		if n := len(g.Seen()); n != 3 {
			t.Errorf("requests = %d, want 3", n)
		}
		if got := timers.Pauses(); len(got) != 2 {
			t.Errorf("pauses = %v, want 2", got)
		}
	})
	t.Run("exhausted", func(t *testing.T) {
		g := newFakeGateway(t, reply{})
		c, _ := newClient(g)
		_, err := c.Action(context.Background(), "player-A", attack)
		var apiErr *client.APIError
		if err == nil || errors.As(err, &apiErr) {
			t.Fatalf("err = %v, want a transport error", err)
		}
		if !strings.Contains(err.Error(), "attempt 4") {
			t.Errorf("err = %v, want it to say the attempt", err)
		}
		if n := len(g.Seen()); n != 4 {
			t.Errorf("requests = %d, want 4", n)
		}
	})
}

func TestCancelDuringPauseStopsRepeating(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "")})
	c, _ := newClient(g)
	timers := silentTimers{armed: make(chan struct{}, 1)}
	c.Timers = timers
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := c.Action(ctx, "player-A", attack)
		done <- err
	}()
	select {
	case <-timers.armed:
	case err := <-done:
		t.Fatalf("Action returned without pausing: %v", err)
	}
	cancel()
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v, want the outcome of the last attempt", err)
	}
	if n := len(g.Seen()); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestNoRetryRepeatsNothing(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "")})
	c, timers := newClient(g)
	c.Backoff = client.NoRetry
	if _, err := c.Action(context.Background(), "player-A", attack); err == nil {
		t.Fatal("want an error")
	}
	if n := len(g.Seen()); n != 1 || len(timers.Pauses()) != 0 {
		t.Errorf("requests = %d, pauses %v; want 1 and none", n, timers.Pauses())
	}
}

// A client written as a literal, the way component §6 shows it, must repeat
// like one made by New: the zero Backoff is DefaultBackoff, not "no repeats".
func TestLiteralClientRepeatsLikeNew(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "")})
	timers := &instantTimers{}
	c := &client.Client{BaseURL: g.srv.URL + "/", ClientID: "telegram-bot", Timers: timers}

	if _, err := c.Action(context.Background(), "player-A", attack); err == nil {
		t.Fatal("want an error")
	}
	if n := len(g.Seen()); n != 4 {
		t.Errorf("requests = %d, want 4 (DefaultBackoff)", n)
	}
	want := []time.Duration{200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond}
	if got := timers.Pauses(); !slices.Equal(got, want) {
		t.Errorf("pauses = %v, want %v", got, want)
	}
	if got := g.Seen()[0].Path; got != "/v1/players/player-A/actions" {
		t.Errorf("path = %q: a trailing slash of a literal BaseURL must not double the slash", got)
	}
}

// Only a network error and 503 are repeated. 429 in particular is not: a
// repeat would spend the rate limit of the player (SEC-11).
func TestNoOtherStatusIsRepeated(t *testing.T) {
	for _, status := range []int{400, 403, 404, 409, 413, 422, 429, 500, 501, 502, 504} {
		g := newFakeGateway(t, reply{status: status, body: errorBody("some_code", "")})
		c, timers := newClient(g)
		_, err := c.Action(context.Background(), "player-A", attack)
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) || apiErr.Status != status {
			t.Errorf("status %d: err = %v, want APIError with the status", status, err)
		}
		if n := len(g.Seen()); n != 1 || len(timers.Pauses()) != 0 {
			t.Errorf("status %d: requests %d, pauses %v; want 1 and none", status, n, timers.Pauses())
		}
	}
}

// The bodies of links and characters carry the external id; the bot logs the
// errors of these calls, so no error text may carry it (SEC-01/02).
func TestErrorsDoNotCarryTheExternalID(t *testing.T) {
	const externalID = "tg-4242"
	calls := map[string]func(c *client.Client) error{
		"Resolve": func(c *client.Client) error {
			_, err := c.Resolve(context.Background(), "telegram", externalID)
			return err
		},
		"Consent": func(c *client.Client) error {
			_, err := c.Consent(context.Background(), api.ConsentRequest{ExternalPlatform: "telegram", ExternalID: externalID})
			return err
		},
		"Forget": func(c *client.Client) error {
			_, err := c.Forget(context.Background(), "telegram", externalID)
			return err
		},
		"CreateCharacter": func(c *client.Client) error {
			_, _, err := c.CreateCharacter(context.Background(), api.CreateCharacterRequest{ExternalPlatform: "telegram", ExternalID: externalID, WorldID: "w", CharacterName: "Вася", ActionKey: "k"})
			return err
		},
	}
	replies := map[string]reply{
		"network errors, repeats spent": {},
		"error answer":                  {status: http.StatusBadRequest, body: errorBody(api.CodeInvalidRequest, "Некорректный запрос.")},
		"503, repeats spent":            {status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "")},
		"undecodable success":           {status: http.StatusOK, body: "{"},
	}
	for name, call := range calls {
		for kind, rep := range replies {
			g := newFakeGateway(t, rep)
			c, _ := newClient(g)
			err := call(c)
			if err == nil {
				t.Errorf("%s, %s: want an error", name, kind)
				continue
			}
			for _, text := range []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
				if strings.Contains(text, externalID) {
					t.Errorf("%s, %s: error text carries the external id: %s", name, kind, text)
				}
			}
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && strings.Contains(fmt.Sprintf("%+v", *apiErr), externalID) {
				t.Errorf("%s, %s: APIError carries the external id: %+v", name, kind, *apiErr)
			}
			if !strings.Contains(string(g.Seen()[0].Body), externalID) {
				t.Errorf("%s, %s: the request body lost the external id; the test proves nothing", name, kind)
			}
		}
	}
}

func TestCloseRoundIsNotRepeated(t *testing.T) {
	for _, rep := range []reply{{}, {status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "")}} {
		g := newFakeGateway(t, rep)
		c, timers := newClient(g)
		c.ActorKind = api.ActorCI
		if _, err := c.CloseRound(context.Background(), "group:g-1"); err == nil {
			t.Errorf("status %d: want an error", rep.status)
		}
		if n := len(g.Seen()); n != 1 || len(timers.Pauses()) != 0 {
			t.Errorf("status %d: requests %d, pauses %v; want 1 and none: a repeat turns a closed round into 409 no_open_round",
				rep.status, n, timers.Pauses())
		}
	}
}

func TestForgetReportsARepeat(t *testing.T) {
	t.Run("first attempt answered", func(t *testing.T) {
		g := newFakeGateway(t, reply{status: http.StatusOK, body: `{"deleted":true,"player_id_detached":"player-A"}`})
		c, _ := newClient(g)
		res, err := c.Forget(context.Background(), "telegram", "42")
		if err != nil || !res.Deleted || res.PlayerIDDetached == nil || *res.PlayerIDDetached != "player-A" || res.Repeated {
			t.Errorf("Forget = %+v, %v; want deleted player-A without a repeat", res, err)
		}
	})
	t.Run("answer of a repeat", func(t *testing.T) {
		g := newFakeGateway(t, reply{}, reply{status: http.StatusOK, body: `{"deleted":false,"player_id_detached":null}`})
		c, _ := newClient(g)
		res, err := c.Forget(context.Background(), "telegram", "42")
		if err != nil || res.Deleted || !res.Repeated {
			t.Errorf("Forget = %+v, %v; want deleted=false marked as the answer of a repeat", res, err)
		}
		if n := len(g.Seen()); n != 2 {
			t.Errorf("requests = %d, want 2", n)
		}
	})
}

// 503 forget_incomplete is returned at once with its Retry-After: a repeat
// under the reader that blocks the wipe would hold the only connection of
// links.db for another 5 s (C-08 v1.5). The other 503 of /forget is still
// repeated.
func TestForgetIncompleteIsNotRepeatedAndCarriesRetryAfter(t *testing.T) {
	g := newFakeGateway(t, reply{
		status: http.StatusServiceUnavailable,
		body:   errorBody(api.CodeForgetIncomplete, "Удаление не завершено, повторите /forget."),
		header: map[string]string{api.HeaderRetryAfter: "5"},
	})
	c, timers := newClient(g)
	_, err := c.Forget(context.Background(), "telegram", "42")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != api.CodeForgetIncomplete || apiErr.RetryAfter != 5*time.Second {
		t.Fatalf("Forget = %v (%+v), want forget_incomplete with RetryAfter 5s", err, apiErr)
	}
	if n := len(g.Seen()); n != 1 || len(timers.Pauses()) != 0 {
		t.Errorf("requests %d, pauses %v; want 1 and none", n, timers.Pauses())
	}

	busy := newFakeGateway(t, reply{status: http.StatusServiceUnavailable, body: errorBody(api.CodeBusUnavailable, "")})
	c, _ = newClient(busy)
	if _, err := c.Forget(context.Background(), "telegram", "42"); err == nil {
		t.Fatal("Forget under bus_unavailable: want an error")
	}
	if n := len(busy.Seen()); n != 4 {
		t.Errorf("bus_unavailable of /forget: requests %d, want 4", n)
	}
}

func TestRetryAfterIsReadInWholeSeconds(t *testing.T) {
	for value, want := range map[string]time.Duration{
		"30": 30 * time.Second, "": 0, "0": 0, "-3": 0, "soon": 0, "Wed, 21 Oct 2026 07:28:00 GMT": 0,
		// N-3 of review #1 of T-311: a huge value does not overflow the duration.
		"86400": client.MaxRetryAfter, "86401": client.MaxRetryAfter, "99999999999999999": client.MaxRetryAfter,
	} {
		g := newFakeGateway(t, reply{status: http.StatusTooManyRequests, body: errorBody(api.CodeRateLimited, ""),
			header: map[string]string{api.HeaderRetryAfter: value}})
		c, _ := newClient(g)
		_, err := c.Action(context.Background(), "player-A", attack)
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) || apiErr.RetryAfter != want {
			t.Errorf("Retry-After %q: err %v, RetryAfter %v; want %v", value, err, apiErr, want)
		}
	}
}

func TestAcceptedWithUnknownStatusIsAnError(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusAccepted, body: `{"status":"queued","correlation_id":"ev-1"}`})
	c, _ := newClient(g)
	res, err := c.Action(context.Background(), "player-A", attack)
	if !errors.Is(err, client.ErrUnexpectedStatus) || res.Accepted != nil || res.Pending != nil {
		t.Errorf("Action = %+v, %v; want ErrUnexpectedStatus and no result", res, err)
	}
}

func TestMalformedBaseURLIsNotRepeated(t *testing.T) {
	timers := &instantTimers{}
	c := &client.Client{BaseURL: "http://[::1", ClientID: "telegram-bot", Timers: timers}
	if _, err := c.Worlds(context.Background()); err == nil || !strings.Contains(err.Error(), "build request") {
		t.Errorf("err = %v, want a build error", err)
	}
	if got := timers.Pauses(); len(got) != 0 {
		t.Errorf("pauses = %v, want none: a malformed URL fails the same way every time", got)
	}
}

func TestBackoffPause(t *testing.T) {
	ms := time.Millisecond
	for _, tc := range []struct {
		name string
		b    client.Backoff
		want []time.Duration
	}{
		{"default", client.DefaultBackoff, []time.Duration{200 * ms, 400 * ms, 800 * ms, 1600 * ms, 1600 * ms}},
		{"no ceiling", client.Backoff{Initial: 100 * ms}, []time.Duration{100 * ms, 200 * ms, 400 * ms, 800 * ms, 1600 * ms}},
		{"initial above the ceiling", client.Backoff{Initial: 5 * time.Second, Max: time.Second}, []time.Duration{time.Second, time.Second, time.Second, time.Second, time.Second}},
	} {
		var got []time.Duration
		for n := range 5 {
			got = append(got, tc.b.Pause(n))
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: pauses %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestErrorWithoutContractBodyKeepsTheStatus(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusBadGateway, body: "<html>bad gateway</html>"})
	c, _ := newClient(g)
	_, err := c.Player(context.Background(), "player-A")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadGateway || apiErr.Code != "" || apiErr.Message != "Bad Gateway" {
		t.Fatalf("err = %v (%+v), want APIError 502 without code", err, apiErr)
	}
	if n := len(g.Seen()); n != 1 {
		t.Errorf("requests = %d, want 1: only network errors and 503 are repeated", n)
	}
}

func TestUnexpectedSuccessStatusIsAnError(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusNoContent})
	c, _ := newClient(g)
	if _, err := c.Resolve(context.Background(), "telegram", "42"); !errors.Is(err, client.ErrUnexpectedStatus) {
		t.Errorf("err = %v, want ErrUnexpectedStatus", err)
	}
}

func TestCreateCharacterReportsTheStatus(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{
		{http.StatusCreated, `{"player_id":"player-A","character":{"player_id":"player-A","name":"Вася","world_id":"w","status":"alive"},"created":true}`},
		{http.StatusOK, `{"player_id":"player-A","character":{"player_id":"player-A","name":"Вася","world_id":"w","status":"alive"},"created":false}`},
		{http.StatusAccepted, `{"player_id":"player-A","status":"creating"}`},
	} {
		g := newFakeGateway(t, reply{status: tc.status, body: tc.body})
		c, _ := newClient(g)
		out, status, err := c.CreateCharacter(context.Background(), api.CreateCharacterRequest{ExternalPlatform: "telegram", ExternalID: "42", WorldID: "w", CharacterName: "Вася", ActionKey: "k"})
		if err != nil || status != tc.status || out.PlayerID != "player-A" {
			t.Errorf("status %d: got %+v, %d, %v", tc.status, out, status, err)
		}
		if tc.status == http.StatusAccepted && (out.Status != "creating" || out.Character != nil) {
			t.Errorf("202: %+v, want status creating without character", out)
		}
		if tc.status == http.StatusOK && (out.Created == nil || *out.Created) {
			t.Errorf("200: created = %v, want false", out.Created)
		}
	}
}

func TestOperationsUseTheirRoutes(t *testing.T) {
	g := newFakeGateway(t, reply{status: http.StatusOK, body: `{}`})
	c, _ := newClient(g)
	ctx := context.Background()
	calls := []struct {
		name         string
		call         func() error
		method, path string
		body         bool
	}{
		{"Resolve", func() error { _, err := c.Resolve(ctx, "telegram", "42"); return err }, http.MethodPost, "/v1/links/resolve", true},
		{"Consent", func() error {
			_, err := c.Consent(ctx, api.ConsentRequest{ExternalPlatform: "telegram", ExternalID: "42"})
			return err
		}, http.MethodPost, "/v1/links/consent", true},
		{"Forget", func() error { _, err := c.Forget(ctx, "telegram", "42"); return err }, http.MethodDelete, "/v1/links", true},
		{"Worlds", func() error { _, err := c.Worlds(ctx); return err }, http.MethodGet, "/v1/worlds", false},
		{"Player", func() error { _, err := c.Player(ctx, "player A/1"); return err }, http.MethodGet, "/v1/players/player%20A%2F1", false},
		{"Group", func() error { _, err := c.Group(ctx, "g-1"); return err }, http.MethodGet, "/v1/groups/g-1", false},
		{"CloseRound", func() error { _, err := c.CloseRound(ctx, "group:g-1"); return err }, http.MethodPost, "/v1/scopes/group:g-1/rounds/close", false},
	}
	for i, tc := range calls {
		if err := tc.call(); err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		req := g.Seen()[i]
		if req.Method != tc.method || req.Path != tc.path {
			t.Errorf("%s: %s %s, want %s %s", tc.name, req.Method, req.Path, tc.method, tc.path)
		}
		if (len(req.Body) > 0) != tc.body {
			t.Errorf("%s: body %q, want body %v", tc.name, req.Body, tc.body)
		}
		if req.Header.Get(api.HeaderClientID) != "telegram-bot" {
			t.Errorf("%s: X-Client-Id = %q", tc.name, req.Header.Get(api.HeaderClientID))
		}
	}
}

func TestNewTrimsTheBaseURLAndSetsDefaults(t *testing.T) {
	c := client.New("http://127.0.0.1:8088/", "ci-harness")
	if c.BaseURL != "http://127.0.0.1:8088" || c.ClientID != "ci-harness" || c.Backoff != client.DefaultBackoff {
		t.Errorf("New = %+v", c)
	}
	if c.HTTP == nil || c.HTTP.Timeout != client.DefaultHTTPTimeout || c.Timers == nil {
		t.Errorf("New: HTTP %v, Timers %v", c.HTTP, c.Timers)
	}
}
