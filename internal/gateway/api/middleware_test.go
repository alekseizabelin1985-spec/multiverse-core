package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
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
)

// externalID is shaped like a Telegram user id; it is not a real account.
const externalID = "7391846205"

// syncBuffer is the log of a test server, written by its goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// lines returns the JSON log lines written so far.
func (b *syncBuffer) lines(t *testing.T) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(b.buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type testServer struct {
	url string
	log *syncBuffer
	cfg *api.Config
}

// serve mounts routes with the middleware chain of the gateway on a test
// server. The client lists are those of the manifest defaults.
func serve(t *testing.T, routes func(r *api.Router)) testServer {
	t.Helper()
	logs := &syncBuffer{}
	var n int
	var mu sync.Mutex
	cfg := &api.Config{
		ClientIDs:        []string{"telegram-bot", "ci-harness", "mvctl"},
		ActorKindClients: []string{"ci-harness", "mvctl"},
		RequestIDs: func() string {
			mu.Lock()
			defer mu.Unlock()
			n++
			return "req-" + strconv.Itoa(n)
		},
		Clock: clock.NewManual(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)),
		Log:   slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	r := api.NewRouter()
	routes(r)
	mux := http.NewServeMux()
	r.Mount(mux, api.Chain(cfg)...)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return testServer{url: srv.URL, log: logs, cfg: cfg}
}

func ok() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = api.WriteJSON(w, http.StatusOK, map[string]string{
			"client": api.ClientIDFrom(r.Context()), "actor": api.ActorKindFrom(r.Context()),
			"request": api.RequestIDFrom(r.Context()),
		})
	})
}

type call struct {
	method, path string
	headers      map[string]string
	body         io.Reader
}

type answer struct {
	status int
	header http.Header
	code   string
	body   map[string]any
}

func (s testServer) do(t *testing.T, c call) answer {
	t.Helper()
	req, err := http.NewRequest(c.method, s.url+c.path, c.body)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	// A bounded client: a guard that lets a request through into a handler
	// that waits must fail the test, not hang it.
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Error(err)
		return answer{}
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	a := answer{status: resp.StatusCode, header: resp.Header}
	var body map[string]any
	if json.Unmarshal(data, &body) == nil {
		a.body = body
		if e, ok := body["error"].(map[string]any); ok {
			a.code, _ = e["code"].(string)
		}
	}
	return a
}

func bot(extra ...string) map[string]string {
	h := map[string]string{api.HeaderClientID: "telegram-bot"}
	for i := 0; i+1 < len(extra); i += 2 {
		h[extra[i]] = extra[i+1]
	}
	return h
}

func TestClientAdmission(t *testing.T) {
	s := serve(t, func(r *api.Router) {
		r.Handle("getPlayer", http.MethodGet, "/v1/players/{player_id}", ok())
		r.Handle("pollDeliveries", http.MethodGet, "/v1/clients/{client_id}/deliveries", ok())
	})
	for name, tc := range map[string]struct {
		call   call
		status int
		code   string
		actor  string
	}{
		"no client":             {call{method: "GET", path: "/v1/players/p"}, 403, api.CodeClientUnknown, ""},
		"unlisted client":       {call{"GET", "/v1/players/p", map[string]string{api.HeaderClientID: "stranger"}, nil}, 403, api.CodeClientUnknown, ""},
		"listed client":         {call{"GET", "/v1/players/p", bot(), nil}, 200, "", api.ActorHuman},
		"human declared":        {call{"GET", "/v1/players/p", bot(api.HeaderActorKind, "human"), nil}, 200, "", api.ActorHuman},
		"ci from the bot":       {call{"GET", "/v1/players/p", bot(api.HeaderActorKind, "ci"), nil}, 403, api.CodeActorKindForbidden, ""},
		"sim from the bot":      {call{"GET", "/v1/players/p", bot(api.HeaderActorKind, "sim"), nil}, 403, api.CodeActorKindForbidden, ""},
		"system from a client":  {call{"GET", "/v1/players/p", bot(api.HeaderActorKind, "system"), nil}, 400, api.CodeInvalidRequest, ""},
		"ci from the harness":   {call{"GET", "/v1/players/p", map[string]string{api.HeaderClientID: "ci-harness", api.HeaderActorKind: "ci"}, nil}, 200, "", api.ActorCI},
		"own deliveries":        {call{"GET", "/v1/clients/telegram-bot/deliveries", bot(), nil}, 200, "", api.ActorHuman},
		"deliveries of another": {call{"GET", "/v1/clients/ci-harness/deliveries", bot(), nil}, 403, api.CodeClientMismatch, ""},
	} {
		t.Run(name, func(t *testing.T) {
			a := s.do(t, tc.call)
			if a.status != tc.status || a.code != tc.code {
				t.Fatalf("%s %s = %d %q, want %d %q", tc.call.method, tc.call.path, a.status, a.code, tc.status, tc.code)
			}
			if tc.actor != "" && a.body["actor"] != tc.actor {
				t.Errorf("actor kind seen by the handler = %v, want %s", a.body["actor"], tc.actor)
			}
			if a.header.Get(api.HeaderRequestID) == "" {
				t.Error("no X-Request-Id in the answer")
			}
		})
	}
}

func TestBodyLimitAndContentType(t *testing.T) {
	var got api.ResolveRequest
	s := serve(t, func(r *api.Router) {
		r.Handle("resolveLink", http.MethodPost, "/v1/links/resolve", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if e := api.DecodeJSON(r, &got); e != nil {
				_ = api.WriteError(w, e)
				return
			}
			_ = api.WriteJSON(w, http.StatusOK, got)
		}))
	})
	json := func(extra ...string) map[string]string {
		return bot(append([]string{"Content-Type", api.ContentTypeJSON}, extra...)...)
	}
	body := func(size int) string {
		const head, tail = `{"external_platform":"telegram","external_id":"`, `"}`
		return head + strings.Repeat("x", size-len(head)-len(tail)) + tail
	}
	big, exact := body(65<<10), body(api.BodyLimit)
	justUnder := body(api.BodyLimit - 1)
	stranger := map[string]string{api.HeaderClientID: "stranger", "Content-Type": api.ContentTypeJSON}

	for name, tc := range map[string]struct {
		call   call
		status int
		code   string
	}{
		"65 KiB":                 {call{"POST", "/v1/links/resolve", json(), strings.NewReader(big)}, 413, api.CodePayloadTooLarge},
		"65 KiB chunked":         {call{"POST", "/v1/links/resolve", json(), io.MultiReader(strings.NewReader(big))}, 413, api.CodePayloadTooLarge},
		"under 64 KiB":           {call{"POST", "/v1/links/resolve", json(), strings.NewReader(justUnder)}, 200, ""},
		"exactly 64 KiB":         {call{"POST", "/v1/links/resolve", json(), strings.NewReader(exact)}, 200, ""},
		"exactly 64 KiB chunked": {call{"POST", "/v1/links/resolve", json(), io.MultiReader(strings.NewReader(exact))}, 200, ""},
		// The client is admitted before the body is measured (component §5.1
		// p. 3 before p. 4): a stranger learns nothing about the limit.
		"65 KiB from a stranger": {call{"POST", "/v1/links/resolve", stranger, strings.NewReader(big)}, 403, api.CodeClientUnknown},
		"text body":              {call{"POST", "/v1/links/resolve", bot("Content-Type", "text/plain"), strings.NewReader(`{}`)}, 400, api.CodeInvalidRequest},
		"not json":               {call{"POST", "/v1/links/resolve", json(), strings.NewReader(`{"external_id":`)}, 400, api.CodeInvalidRequest},
		"two values":             {call{"POST", "/v1/links/resolve", json(), strings.NewReader(`{} {}`)}, 400, api.CodeInvalidRequest},
		"no body":                {call{method: "POST", path: "/v1/links/resolve", headers: json()}, 400, api.CodeInvalidRequest},
	} {
		t.Run(name, func(t *testing.T) {
			a := s.do(t, tc.call)
			if a.status != tc.status || a.code != tc.code {
				t.Fatalf("= %d %q, want %d %q", a.status, a.code, tc.status, tc.code)
			}
			if a.header.Get(api.HeaderRequestID) == "" {
				t.Error("no X-Request-Id in the answer")
			}
		})
	}
}

func TestOneLongPollPerClient(t *testing.T) {
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	s := serve(t, func(r *api.Router) {
		r.Handle("pollDeliveries", http.MethodGet, "/v1/clients/{client_id}/deliveries", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, has := r.Context().Deadline(); has {
				t.Error("the long-poll got the request timeout")
			}
			entered <- struct{}{}
			<-release
			_ = api.WriteJSON(w, http.StatusOK, api.DeliveriesResponse{})
		}))
	})
	path := "/v1/clients/telegram-bot/deliveries"
	first := make(chan answer, 1)
	go func() { first <- s.do(t, call{"GET", path, bot(), nil}) }()
	<-entered

	if a := s.do(t, call{"GET", path, bot(), nil}); a.status != 409 || a.code != api.CodePollInProgress {
		t.Errorf("second parallel long-poll = %d %q, want 409 poll_in_progress", a.status, a.code)
	}
	// Another client polls at the same time: the guard is per client.
	harness := make(chan answer, 1)
	go func() {
		harness <- s.do(t, call{"GET", "/v1/clients/ci-harness/deliveries", map[string]string{api.HeaderClientID: "ci-harness"}, nil})
	}()
	<-entered
	close(release)
	if a := <-first; a.status != 200 {
		t.Errorf("first long-poll = %d, want 200", a.status)
	}
	if a := <-harness; a.status != 200 {
		t.Errorf("parallel long-poll of another client = %d %q, want 200", a.status, a.code)
	}
	if a := s.do(t, call{"GET", path, bot(), nil}); a.status != 200 {
		t.Errorf("a long-poll after the first ended = %d %q, want 200", a.status, a.code)
	}
}

func TestRequestTimeoutAndRateLimitHook(t *testing.T) {
	var deadline time.Time
	s := serve(t, func(r *api.Router) {
		r.Handle("getPlayer", http.MethodGet, "/v1/players/{player_id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deadline, _ = r.Context().Deadline()
			_ = api.WriteJSON(w, http.StatusOK, struct{}{})
		}))
		r.Handle("postAction", http.MethodPost, "/v1/players/{player_id}/actions", ok())
	})
	if a := s.do(t, call{"GET", "/v1/players/p", bot(), nil}); a.status != 200 || deadline.IsZero() {
		t.Fatalf("= %d, deadline %v; want 200 under a deadline", a.status, deadline)
	}

	s.cfg.Limiter = limiter(func(r *http.Request, rt api.Route) (bool, int) { return rt.OperationID != "postAction", 7 })
	a := s.do(t, call{"POST", "/v1/players/p/actions", bot("Content-Type", api.ContentTypeJSON), strings.NewReader(`{}`)})
	if a.status != 429 || a.code != api.CodeRateLimited || a.header.Get(api.HeaderRetryAfter) != "7" {
		t.Errorf("limited action = %d %q Retry-After %q, want 429 rate_limited 7", a.status, a.code, a.header.Get(api.HeaderRetryAfter))
	}
	if a := s.do(t, call{"GET", "/v1/players/p", bot(), nil}); a.status != 200 {
		t.Errorf("an operation the limiter allows = %d", a.status)
	}
}

type limiter func(r *http.Request, rt api.Route) (bool, int)

func (l limiter) Allow(r *http.Request, rt api.Route) (bool, int) { return l(r, rt) }

// SEC-01, SEC-02: an error on an operation that carries external IDs leaves
// request_id and code in the log, and nothing of the body — whatever failed,
// a handler, a decoder or a panic.
func TestNoLogOperationsLogOnlyTheRequestIDAndTheCode(t *testing.T) {
	body := `{"external_platform":"telegram","external_id":"` + externalID + `","character_name":"Вася"}`
	s := serve(t, func(r *api.Router) {
		r.Handle("resolveLink", http.MethodPost, "/v1/links/resolve", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data, _ := io.ReadAll(r.Body)
			panic("cannot handle " + string(data))
		}))
		r.Handle("createCharacter", http.MethodPost, "/v1/characters", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req api.CreateCharacterRequest
			if e := api.DecodeJSON(r, &req); e != nil {
				_ = api.WriteError(w, e)
				return
			}
			_ = api.WriteError(w, api.NewError(api.CodeNameInvalid, nil))
		}))
		r.Handle("getPlayer", http.MethodGet, "/v1/players/{player_id}", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom in getPlayer")
		}))
	})
	jsonHeaders := bot("Content-Type", api.ContentTypeJSON)

	if a := s.do(t, call{"POST", "/v1/links/resolve", jsonHeaders, strings.NewReader(body)}); a.status != 500 || a.code != api.CodeInternal {
		t.Fatalf("panicking resolve = %d %q, want 500 internal", a.status, a.code)
	}
	if a := s.do(t, call{"POST", "/v1/characters", jsonHeaders, strings.NewReader(body)}); a.status != 400 || a.code != api.CodeNameInvalid {
		t.Fatalf("rejected character = %d %q, want 400 name_invalid", a.status, a.code)
	}
	if a := s.do(t, call{"POST", "/v1/characters", jsonHeaders, strings.NewReader(body[:40])}); a.status != 400 || a.code != api.CodeInvalidRequest {
		t.Fatalf("broken character body = %d %q, want 400 invalid_request", a.status, a.code)
	}
	if a := s.do(t, call{"GET", "/v1/players/p", bot(), nil}); a.status != 500 {
		t.Fatalf("panicking getPlayer = %d, want 500", a.status)
	}

	for _, secret := range []string{externalID, "Вася", "character_name", "external_id", "cannot handle"} {
		if strings.Contains(s.log.String(), secret) {
			t.Errorf("the log contains %q:\n%s", secret, s.log.String())
		}
	}
	allowed := []string{"time", "level", "msg", "request_id", "code", "handled"}
	var nolog, other int
	for _, line := range s.log.lines(t) {
		if line["operation"] == "getPlayer" || strings.Contains(asString(line["panic"]), "getPlayer") {
			other++
			continue
		}
		nolog++
		for key := range line {
			if !slices.Contains(allowed, key) {
				t.Errorf("a nolog line has the field %q: %v", key, line)
			}
		}
		if line["request_id"] == nil {
			t.Errorf("a nolog line has no request_id: %v", line)
		}
		if line["msg"] == "http request" && line["code"] == nil {
			t.Errorf("the access line of an error has no code: %v", line)
		}
	}
	// resolve: panic + access; characters: two access lines; getPlayer: panic + access.
	if nolog != 4 || other != 2 {
		t.Errorf("log lines: %d nolog, %d other; want 4 and 2:\n%s", nolog, other, s.log.String())
	}
	for _, line := range s.log.lines(t) {
		if line["msg"] == "http handler panicked" && line["handled"] != false {
			t.Errorf("a panic is logged without handled=false: %v", line)
		}
	}
	if !strings.Contains(s.log.String(), "boom in getPlayer") || !strings.Contains(s.log.String(), `"route":"/v1/players/{player_id}"`) {
		t.Errorf("an operation outside nolog lost its panic or its route in the log:\n%s", s.log.String())
	}
}

func asString(v any) string { s, _ := v.(string); return s }

// A panic after the handler wrote its header cannot become a 500; the answer
// keeps its status and the panic is still logged.
func TestAPanicAfterTheHeaderKeepsTheStatus(t *testing.T) {
	s := serve(t, func(r *api.Router) {
		r.Handle("getPlayer", http.MethodGet, "/v1/players/{player_id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			panic("late")
		}))
	})
	if a := s.do(t, call{"GET", "/v1/players/p", bot(), nil}); a.status != http.StatusAccepted {
		t.Errorf("= %d, want the status already written", a.status)
	}
	if !strings.Contains(s.log.String(), "late") {
		t.Errorf("the late panic is not logged:\n%s", s.log.String())
	}
}

func TestInfoWithoutTheMiddlewareIsEmpty(t *testing.T) {
	ctx := context.Background()
	if api.RequestIDFrom(ctx) != "" || api.ClientIDFrom(ctx) != "" || api.ActorKindFrom(ctx) != "" {
		t.Error("a context without the middleware carries request info")
	}
}
