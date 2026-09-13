package openaicompat_test

// The answers under testdata/ follow the JSON llama-server gives (the README of
// tools/server of llama.cpp, the fields ADR-005 add. 2 names, the double of
// testdata/script-parity): they are written by hand, because this task does
// not talk to a live server (DoD T-208). No test here leaves the process.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"multiverse-core.io/internal/llm"
	"multiverse-core.io/internal/llm/providers"
	openaicompat "multiverse-core.io/internal/llm/providers/openai_compat"
	"multiverse-core.io/shared/env"
)

const model = "Qwen3.8-27B-UD-Q3_K_XL"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return data
}

type route struct {
	code int
	body []byte
}

func okFixture(t *testing.T, name string) route {
	return route{code: http.StatusOK, body: fixture(t, name)}
}

type seen struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// stand is an httptest server that answers each path with its route and keeps
// every request it got.
type stand struct {
	srv    *httptest.Server
	mu     sync.Mutex
	routes map[string]route
	reqs   []seen
}

func newStand(t *testing.T, routes map[string]route) *stand {
	t.Helper()
	s := &stand{routes: routes}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.reqs = append(s.reqs, seen{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone(), Body: body})
		rt, ok := s.routes[r.URL.Path]
		s.mu.Unlock()
		if !ok {
			rt = route{code: http.StatusNotFound, body: []byte(`{"error":{"code":404,"message":"File Not Found","type":"not_found_error"}}`)}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(rt.code)
		_, _ = w.Write(rt.body)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *stand) requests() []seen {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]seen{}, s.reqs...)
}

func (s *stand) provider(t *testing.T, opts ...openaicompat.Option) *openaicompat.Provider {
	t.Helper()
	p, err := openaicompat.New(llm.Config{Provider: llm.ProviderOpenAICompat, URL: s.srv.URL}, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(p.CloseIdleConnections)
	return p
}

func narrativeRequest() llm.Request {
	return llm.Request{
		Phase:  llm.PhaseNarrative,
		Model:  model,
		System: "Ты — рассказчик тёмного леса.",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Игрок бьёт волка."},
		},
		Schema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string","maxLength":210}},"required":["text"]}`),
		Params: llm.Params{Temperature: 0.7, MaxTokens: 160},
	}
}

func bodyOf(t *testing.T, r seen) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("request body is not JSON: %v\n%s", err, r.Body)
	}
	return m
}

func only(t *testing.T, s *stand) seen {
	t.Helper()
	reqs := s.requests()
	if len(reqs) != 1 {
		t.Fatalf("server got %d requests, want 1: %+v", len(reqs), reqs)
	}
	return reqs[0]
}

func TestGenerateWithASchema(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-schema.json")})
	req := narrativeRequest()

	resp, err := s.provider(t).Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	got := only(t, s)
	if got.Method != http.MethodPost || got.Path != "/v1/chat/completions" {
		t.Errorf("request = %s %s, want POST /v1/chat/completions", got.Method, got.Path)
	}
	if ct := got.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	body := bodyOf(t, got)
	if body["model"] != model || body["stream"] != false {
		t.Errorf("model/stream = %v/%v", body["model"], body["stream"])
	}
	rf, ok := body["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_schema" {
		t.Fatalf("response_format = %#v, want type json_schema", body["response_format"])
	}
	js, ok := rf["json_schema"].(map[string]any)
	if !ok || js["name"] != "narrative" {
		t.Fatalf("response_format.json_schema = %#v, want name narrative", rf["json_schema"])
	}
	sent, _ := json.Marshal(js["schema"])
	var wantSchema, gotSchema any
	_ = json.Unmarshal(req.Schema, &wantSchema)
	_ = json.Unmarshal(sent, &gotSchema)
	if !reflect.DeepEqual(gotSchema, wantSchema) {
		t.Errorf("json_schema.schema = %s, want %s", sent, req.Schema)
	}
	if kw, _ := body["chat_template_kwargs"].(map[string]any); kw == nil || kw["enable_thinking"] != false {
		t.Errorf("chat_template_kwargs = %#v, want enable_thinking false", body["chat_template_kwargs"])
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages = %#v, want system and user", body["messages"])
	}
	if m := msgs[0].(map[string]any); m["role"] != "system" || m["content"] != req.System {
		t.Errorf("messages[0] = %#v", m)
	}
	if m := msgs[1].(map[string]any); m["role"] != "user" || m["content"] != req.Messages[0].Content {
		t.Errorf("messages[1] = %#v", m)
	}

	want := llm.Response{
		Content:   `{"kind":"narrative","text":"Волк рычит и отступает к опушке.","mentions":["e1"],"background_refs":[],"tone":"tense"}`,
		Tokens:    llm.Tokens{Prompt: 495, Completion: 76, Cached: 312},
		LatencyMs: 2019, // round(210.5 + 1808.153)
		Provider:  llm.ProviderOpenAICompat,
		Model:     model,
	}
	if resp != want {
		t.Errorf("Response =\n%+v\nwant\n%+v", resp, want)
	}
}

func TestGenerateWithoutASchema(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-text.json")})
	req := narrativeRequest()
	req.Schema = nil
	req.System = ""

	resp, err := s.provider(t).Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body := bodyOf(t, only(t, s))
	if _, ok := body["response_format"]; ok {
		t.Errorf("response_format sent without a schema: %#v", body["response_format"])
	}
	if kw, _ := body["chat_template_kwargs"].(map[string]any); kw == nil || kw["enable_thinking"] != false {
		t.Errorf("chat_template_kwargs = %#v, want enable_thinking false without a schema too", body["chat_template_kwargs"])
	}
	if msgs, _ := body["messages"].([]any); len(msgs) != 1 {
		t.Errorf("messages = %#v, want the user message only when System is empty", body["messages"])
	}
	if resp.Content != "Ночь тиха, тропа пуста." || resp.Tokens != (llm.Tokens{Prompt: 40, Completion: 11}) || resp.LatencyMs != 323 {
		t.Errorf("Response = %+v", resp)
	}
}

func TestThinkIsPassedAsItIs(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-schema.json")})
	req := narrativeRequest()
	req.Params.Think = true
	if _, err := s.provider(t).Generate(context.Background(), req); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body := bodyOf(t, only(t, s))
	if kw, _ := body["chat_template_kwargs"].(map[string]any); kw == nil || kw["enable_thinking"] != true {
		t.Errorf("chat_template_kwargs = %#v, want enable_thinking true", body["chat_template_kwargs"])
	}
	if _, ok := body["response_format"]; !ok {
		t.Error("response_format dropped when thinking is on")
	}
}

func TestSamplingOfTheRequestOverTheDefaultsOfThePhase(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params llm.Params
		want   map[string]any // absent key: must not be sent
	}{
		{
			name:   "blueprint sets temperature and max_tokens only",
			params: llm.Params{Temperature: 0.4, MaxTokens: 160},
			want:   map[string]any{"temperature": 0.4, "top_p": 0.8, "top_k": 20.0, "min_p": 0.0, "presence_penalty": 1.5, "max_tokens": 160.0},
		},
		{
			name:   "nothing set",
			params: llm.Params{},
			want:   map[string]any{"temperature": 0.7, "top_p": 0.8, "top_k": 20.0, "min_p": 0.0, "presence_penalty": 1.5},
		},
		{
			name:   "everything set",
			params: llm.Params{Temperature: 1.1, TopP: 0.9, TopK: 40, MinP: 0.05, PresencePenalty: 0.5, MaxTokens: 700},
			want:   map[string]any{"temperature": 1.1, "top_p": 0.9, "top_k": 40.0, "min_p": 0.05, "presence_penalty": 0.5, "max_tokens": 700.0},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-schema.json")})
			req := narrativeRequest()
			req.Params = tc.params
			if _, err := s.provider(t).Generate(context.Background(), req); err != nil {
				t.Fatalf("Generate: %v", err)
			}
			body := bodyOf(t, only(t, s))
			for _, key := range []string{"temperature", "top_p", "top_k", "min_p", "presence_penalty", "max_tokens"} {
				want, wantSent := tc.want[key]
				got, sent := body[key]
				if sent != wantSent || (sent && got != want) {
					t.Errorf("%s = %v (sent %v), want %v (sent %v)", key, got, sent, want, wantSent)
				}
			}
		})
	}
}

func TestReasoningContentIsNotKept(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-reasoning.json")})
	resp, err := s.provider(t).Generate(context.Background(), narrativeRequest())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	const reasoning = "СКРЫТОЕ-РАССУЖДЕНИЕ: игрок ранен, волк голоден."
	if resp.ReasoningLen != utf8.RuneCountInString(reasoning) {
		t.Errorf("ReasoningLen = %d, want %d characters", resp.ReasoningLen, utf8.RuneCountInString(reasoning))
	}
	if resp.Content != `{"kind":"narrative","text":"Волк кружит рядом.","mentions":[],"background_refs":[],"tone":"tense"}` {
		t.Errorf("Content = %q", resp.Content)
	}
	printed, _ := json.Marshal(resp)
	if strings.Contains(string(printed), "СКРЫТОЕ") || strings.Contains(resp.Content, "СКРЫТОЕ") {
		t.Errorf("the reasoning text reached the response: %s", printed)
	}
	if resp.Tokens != (llm.Tokens{Prompt: 420, Completion: 58}) {
		t.Errorf("Tokens = %+v, want cached 0 without prompt_tokens_details", resp.Tokens)
	}
}

// stepClock moves by step on every reading.
type stepClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(c.step)
	return c.now
}

func TestTokensAreEstimatedWithAMarkWithoutUsage(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-no-usage.json")})
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	req := narrativeRequest()

	resp, err := s.provider(t, openaicompat.WithLogger(logger),
		openaicompat.WithClock(&stepClock{now: time.Unix(1757716800, 0), step: 1500 * time.Millisecond})).
		Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	promptChars := utf8.RuneCountInString(req.System) + utf8.RuneCountInString(req.Messages[0].Content)
	want := llm.Tokens{
		Prompt:     int(math.Ceil(float64(promptChars) / 2.5)),
		Completion: int(math.Ceil(float64(utf8.RuneCountInString("Тропа пуста.")) / 2.5)),
	}
	if resp.Tokens != want {
		t.Errorf("Tokens = %+v, want the estimate %+v", resp.Tokens, want)
	}
	if resp.LatencyMs != 1500 {
		t.Errorf("LatencyMs = %d, want 1500 from the clock without timings", resp.LatencyMs)
	}
	out := logs.String()
	if !strings.Contains(out, `"level":"WARN"`) || !strings.Contains(out, "tokens estimated") {
		t.Errorf("no warning marks the estimate: %q", out)
	}
	for _, text := range []string{req.System, req.Messages[0].Content, "Тропа пуста."} {
		if strings.Contains(out, text) {
			t.Errorf("the log carries the text %q: %s", text, out)
		}
	}
}

func TestTheEstimateCountsTheReasoningAsCompletion(t *testing.T) {
	body := []byte(`{"choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"двадцать пять знаков тут","content":"ответ"}}]}`)
	s := newStand(t, map[string]route{openaicompat.PathChat: {code: http.StatusOK, body: body}})
	resp, err := s.provider(t, openaicompat.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))).
		Generate(context.Background(), narrativeRequest())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	chars := utf8.RuneCountInString("двадцать пять знаков тут") + utf8.RuneCountInString("ответ")
	if want := int(math.Ceil(float64(chars) / 2.5)); resp.Tokens.Completion != want {
		t.Errorf("Completion = %d, want %d: the reasoning is generated and counted", resp.Tokens.Completion, want)
	}
}

func TestUsageIsNotMarkedWhenReported(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-schema.json")})
	var logs bytes.Buffer
	if _, err := s.provider(t, openaicompat.WithLogger(slog.New(slog.NewJSONHandler(&logs, nil)))).
		Generate(context.Background(), narrativeRequest()); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if logs.Len() != 0 {
		t.Errorf("a reported usage logged: %s", logs.String())
	}
}

func TestLatencyNeedsBothTimings(t *testing.T) {
	body := []byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}],` +
		`"usage":{"prompt_tokens":3,"completion_tokens":1},"timings":{"predicted_ms":900.0}}`)
	s := newStand(t, map[string]route{openaicompat.PathChat: {code: http.StatusOK, body: body}})
	resp, err := s.provider(t, openaicompat.WithClock(&stepClock{now: time.Unix(0, 0), step: 250 * time.Millisecond})).
		Generate(context.Background(), narrativeRequest())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.LatencyMs != 250 {
		t.Errorf("LatencyMs = %d, want 250 from the clock when prompt_ms is absent", resp.LatencyMs)
	}
}

func TestAuthorizationOnlyWithAKey(t *testing.T) {
	const key = "t208-test-key-value"
	for _, tc := range []struct {
		name string
		key  llm.Secret
		want string
	}{
		{name: "no key", key: llm.Secret{}, want: ""},
		{name: "key", key: llm.NewSecret(key), want: "Bearer " + key},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStand(t, map[string]route{
				openaicompat.PathChat:       okFixture(t, "chat-schema.json"),
				openaicompat.PathModels:     okFixture(t, "models.json"),
				openaicompat.PathEmbeddings: okFixture(t, "embeddings.json"),
				openaicompat.PathHealth:     okFixture(t, "health-ok.json"),
			})
			p, err := openaicompat.New(llm.Config{URL: s.srv.URL, APIKey: tc.key})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer p.CloseIdleConnections()
			ctx := context.Background()
			_, _ = p.Generate(ctx, narrativeRequest())
			_, _ = p.Models(ctx)
			_, _ = p.Embed(ctx, "nomic-embed-text-v1.5", []string{"a", "b"})
			_ = p.Health(ctx)
			reqs := s.requests()
			if len(reqs) != 4 {
				t.Fatalf("server got %d requests, want 4", len(reqs))
			}
			for _, r := range reqs {
				_, present := r.Header["Authorization"]
				if got := r.Header.Get("Authorization"); got != tc.want || present != (tc.want != "") {
					t.Errorf("%s %s: Authorization = %q (present %v), want %q", r.Method, r.Path, got, present, tc.want)
				}
			}
		})
	}
}

func TestTheV1TailGivesOneRequestURL(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-schema.json")})
	for _, raw := range []string{s.srv.URL, s.srv.URL + "/", s.srv.URL + "/v1", s.srv.URL + "/v1/"} {
		cfg, err := llm.LoadConfig(env.MapSource(map[string]string{"MV_LLM_URL": raw}))
		if err != nil {
			t.Fatalf("LoadConfig(%q): %v", raw, err)
		}
		fromEnv, err := openaicompat.New(cfg)
		if err != nil {
			t.Fatalf("New(LoadConfig(%q)): %v", raw, err)
		}
		// A Config assembled by hand keeps the tail LoadConfig would have dropped.
		byHand, err := openaicompat.New(llm.Config{URL: raw})
		if err != nil {
			t.Fatalf("New(Config{URL: %q}): %v", raw, err)
		}
		for _, p := range []*openaicompat.Provider{fromEnv, byHand} {
			if _, err := p.Generate(context.Background(), narrativeRequest()); err != nil {
				t.Fatalf("Generate via %q: %v", raw, err)
			}
			p.CloseIdleConnections()
		}
	}
	reqs := s.requests()
	if len(reqs) != 8 {
		t.Fatalf("server got %d requests, want 8", len(reqs))
	}
	for _, r := range reqs {
		if r.Path != "/v1/chat/completions" {
			t.Errorf("request path = %q, want /v1/chat/completions for every spelling", r.Path)
		}
	}
}

func TestAPathPrefixIsKept(t *testing.T) {
	var paths []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		_, _ = w.Write(fixture(t, "models.json"))
	}))
	defer srv.Close()
	p, err := openaicompat.New(llm.Config{URL: srv.URL + "/proxy/llm/v1"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.CloseIdleConnections()
	if _, err := p.Models(context.Background()); err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/proxy/llm/v1/models" {
		t.Errorf("paths = %v, want [/proxy/llm/v1/models]", paths)
	}
}

func TestAnEmptyURLIsAConfigurationError(t *testing.T) {
	_, err := llm.LoadConfig(env.MapSource(map[string]string{"MV_LLM_URL": ""}))
	if !errors.Is(err, llm.ErrConfig) {
		t.Errorf("LoadConfig with an empty MV_LLM_URL: %v, want ErrConfig", err)
	}
	p, err := openaicompat.New(llm.Config{})
	if !errors.Is(err, llm.ErrConfig) || p != nil {
		t.Errorf("New without an address = %v, %v; want nil, ErrConfig", p, err)
	}
	if _, err := openaicompat.New(llm.Config{URL: "http://127.0.0.1"}); !errors.Is(err, llm.ErrConfig) {
		t.Errorf("New with a local address without a port: %v, want ErrConfig", err)
	}
}

func TestNewRunsTheCloudGate(t *testing.T) {
	_, err := openaicompat.New(llm.Config{URL: "https://api.example.com/v1"})
	if !errors.Is(err, llm.ErrCloudDisabled) {
		t.Errorf("cloud address without the flag: %v, want ErrCloudDisabled", err)
	}
	cloud := llm.Config{URL: "https://api.example.com/v1", Cloud: llm.Cloud{Enabled: true}}
	if _, err := openaicompat.New(cloud); err != nil {
		t.Errorf("cloud address with the flag: %v", err)
	}
	if _, err := openaicompat.New(llm.Config{Provider: llm.ProviderOllama, URL: "http://127.0.0.1:8080"}); err == nil {
		t.Error("New accepted the configuration of another provider")
	}
}

func TestTheRegistryBuildsTheProvider(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-schema.json")})
	r := providers.NewRegistry()
	r.Register(llm.ProviderOpenAICompat, openaicompat.Factory())

	p, err := r.New(llm.ProviderOpenAICompat, llm.Config{URL: s.srv.URL})
	if err != nil {
		t.Fatalf("Registry.New: %v", err)
	}
	if _, err := p.Generate(context.Background(), narrativeRequest()); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	p.(*openaicompat.Provider).CloseIdleConnections()

	if p, err := openaicompat.Factory()(llm.Config{URL: "https://api.example.com"}); p != nil || !errors.Is(err, llm.ErrCloudDisabled) {
		t.Errorf("Factory on a cloud address without the flag = %v, %v; want a nil interface and ErrCloudDisabled", p, err)
	}
}

// proxyChild marks the child process TestALocalAddressDoesNotUseTheProxy
// starts. It is a flag of the test binary and not a variable of the
// environment: the environment is read through shared/env only (forbidigo).
var proxyChild = flag.Bool("openaicompat.proxy-child", false,
	"run TestALocalAddressGoesPastTheProxyInAFreshProcess: set by TestALocalAddressDoesNotUseTheProxy for its child process")

// unreachableProxy is the proxy the child process gets: nothing listens on
// port 1, and the dialer of the test refuses it before the network does.
const unreachableProxy = "http://127.0.0.1:1"

// localHosts are local addresses http.ProxyFromEnvironment does not bypass: a
// service of compose, RFC 1918 and the host of Docker Desktop.
var localHosts = []string{"llm-stand", "10.20.30.40", "host.docker.internal"}

// The proxy of the environment is used for the cloud only. Two things are
// checked, because one is not enough: the transport of a local provider has no
// proxy function (here), and a request of it really goes past an unreachable
// proxy to a name only the dialer of the test knows (in a child process).
// Without the second check a provider could keep ProxyFromEnvironment and pass
// on 127.0.0.1, which it bypasses anyway.
//
// The second check runs in a child process because http.ProxyFromEnvironment
// reads the environment once per process: in a run of the whole package an
// earlier test has already sent a request through it, and t.Setenv here would
// change nothing (review #1 of T-208, Mi-1). The child starts with HTTP_PROXY
// and HTTPS_PROXY in its environment, so its first reading sees them.
func TestALocalAddressDoesNotUseTheProxy(t *testing.T) {
	if *proxyChild {
		t.Skip("the child process runs the dialing check only")
	}
	for _, host := range localHosts {
		p, err := openaicompat.New(llm.Config{URL: "http://" + net.JoinHostPort(host, "8080")})
		if err != nil {
			t.Fatalf("New(%s): %v", host, err)
		}
		if openaicompat.TransportOf(p).Proxy != nil {
			t.Errorf("Transport.Proxy of the local address %s is set", host)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	const child = "TestALocalAddressGoesPastTheProxyInAFreshProcess"
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+child+"$", "-test.count=1", "-test.v", "-openaicompat.proxy-child")
	env := make([]string, 0, len(cmd.Environ())+3)
	for _, kv := range cmd.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		// REQUEST_METHOD makes net/http ignore HTTP_PROXY (a CGI process), and a
		// lower-case no_proxy would stand in for the empty NO_PROXY.
		if slices.ContainsFunc([]string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "REQUEST_METHOD"},
			func(s string) bool { return strings.EqualFold(s, name) }) {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(env, "HTTP_PROXY="+unreachableProxy, "HTTPS_PROXY="+unreachableProxy, "NO_PROXY=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the child process: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+child+" ") {
		t.Fatalf("the child process did not pass %s:\n%s", child, out)
	}
}

// TestALocalAddressGoesPastTheProxyInAFreshProcess is the dialing half of
// TestALocalAddressDoesNotUseTheProxy, and runs only in its child process.
func TestALocalAddressGoesPastTheProxyInAFreshProcess(t *testing.T) {
	if !*proxyChild {
		t.Skip("runs in the child process of TestALocalAddressDoesNotUseTheProxy")
	}
	s := newStand(t, map[string]route{openaicompat.PathModels: okFixture(t, "models.json")})
	target := s.srv.Listener.Addr().String()
	_, port, _ := net.SplitHostPort(target)

	for _, host := range localHosts {
		t.Run(host, func(t *testing.T) {
			addr := net.JoinHostPort(host, port)
			// The control: the environment of this process does name the proxy for
			// the address, so a transport that asked it would dial the proxy.
			proxy, err := http.ProxyFromEnvironment(&http.Request{URL: &url.URL{Scheme: "http", Host: addr}})
			if err != nil || proxy == nil || proxy.Host != "127.0.0.1:1" {
				t.Fatalf("ProxyFromEnvironment(%s) = %v, %v; want the unreachable proxy of the parent", addr, proxy, err)
			}
			var dialed []string
			var mu sync.Mutex
			dial := func(ctx context.Context, network, a string) (net.Conn, error) {
				mu.Lock()
				dialed = append(dialed, a)
				mu.Unlock()
				if a != addr {
					// Anything else is the proxy: refused at once, where Windows
					// would retry a refused connection for seconds.
					return nil, errors.New("dial " + a + ": not the stand")
				}
				return (&net.Dialer{}).DialContext(ctx, network, target)
			}
			p, err := openaicompat.New(llm.Config{URL: "http://" + addr}, openaicompat.WithDialContext(dial))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer p.CloseIdleConnections()
			models, err := p.Models(context.Background())
			if err != nil {
				t.Fatalf("Models through %s with HTTP_PROXY unreachable: %v (dialed %v)", host, err, dialed)
			}
			if len(models) != 1 || len(dialed) != 1 || dialed[0] != addr {
				t.Errorf("models %v, dialed %v; want one direct dial of %s", models, dialed, addr)
			}
		})
	}
}

func TestACloudAddressUsesTheProxyOfTheEnvironment(t *testing.T) {
	p, err := openaicompat.New(llm.Config{URL: "https://api.example.com/v1", Cloud: llm.Cloud{Enabled: true}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	proxy := openaicompat.TransportOf(p).Proxy
	if proxy == nil {
		t.Fatal("Transport.Proxy of a cloud address is nil")
	}
	if reflect.ValueOf(proxy).Pointer() != reflect.ValueOf(http.ProxyFromEnvironment).Pointer() {
		t.Error("Transport.Proxy of a cloud address is not http.ProxyFromEnvironment")
	}
}

func TestModels(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathModels: okFixture(t, "models.json")})
	models, err := s.provider(t).Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if !reflect.DeepEqual(models, []string{model}) {
		t.Errorf("Models = %v, want [%s]", models, model)
	}
	if r := only(t, s); r.Method != http.MethodGet || r.Path != "/v1/models" {
		t.Errorf("request = %s %s", r.Method, r.Path)
	}

	for name, body := range map[string]string{
		"no data":  `{"object":"list","models":[]}`,
		"not json": `<html>404</html>`,
	} {
		bad := newStand(t, map[string]route{openaicompat.PathModels: {code: http.StatusOK, body: []byte(body)}})
		if _, err := bad.provider(t).Models(context.Background()); !errors.Is(err, openaicompat.ErrMalformedResponse) {
			t.Errorf("%s: Models error = %v, want ErrMalformedResponse", name, err)
		}
	}
	empty := newStand(t, map[string]route{openaicompat.PathModels: {code: http.StatusOK, body: []byte(`{"object":"list","data":[]}`)}})
	if models, err := empty.provider(t).Models(context.Background()); err != nil || len(models) != 0 {
		t.Errorf("empty data: Models = %v, %v; want an empty list", models, err)
	}
}

func TestHealth(t *testing.T) {
	loadingLlama := fixture(t, "health-loading.json")
	for _, tc := range []struct {
		name     string
		routes   map[string]route
		required []string
		want     llm.Status
		paths    []string
	}{
		{name: "200", routes: map[string]route{openaicompat.PathHealth: okFixture(t, "health-ok.json")},
			want: llm.StatusOK, paths: []string{"/health"}},
		{name: "503 of llama-server is loading", routes: map[string]route{openaicompat.PathHealth: {code: 503, body: loadingLlama}},
			want: llm.StatusLoading, paths: []string{"/health"}},
		{name: "503 status loading", routes: map[string]route{openaicompat.PathHealth: {code: 503, body: []byte(`{"status":"loading"}`)}},
			required: []string{model}, want: llm.StatusLoading, paths: []string{"/health"}},
		{name: "500 is unavailable", routes: map[string]route{openaicompat.PathHealth: {code: 500, body: []byte(`{}`)}},
			want: llm.StatusUnavailable, paths: []string{"/health"}},
		{name: "required model resident", routes: map[string]route{
			openaicompat.PathHealth: okFixture(t, "health-ok.json"), openaicompat.PathModels: okFixture(t, "models.json")},
			required: []string{model}, want: llm.StatusOK, paths: []string{"/health", "/v1/models"}},
		{name: "required model not resident", routes: map[string]route{
			openaicompat.PathHealth: okFixture(t, "health-ok.json"), openaicompat.PathModels: okFixture(t, "models.json")},
			required: []string{model, "Qwen3.6-35B-A3B-UD-Q3_K_XL"}, want: llm.Degraded(llm.DegradedModelNotResident),
			paths: []string{"/health", "/v1/models"}},
		{name: "a model is compared as it is", routes: map[string]route{
			openaicompat.PathHealth: okFixture(t, "health-ok.json"), openaicompat.PathModels: okFixture(t, "models.json")},
			required: []string{model + ".gguf"}, want: llm.Degraded(llm.DegradedModelNotResident),
			paths: []string{"/health", "/v1/models"}},
		{name: "404 falls back to the models", routes: map[string]route{openaicompat.PathModels: okFixture(t, "models.json")},
			want: llm.StatusOK, paths: []string{"/health", "/v1/models"}},
		{name: "404 and models not resident", routes: map[string]route{openaicompat.PathModels: okFixture(t, "models.json")},
			required: []string{"other"}, want: llm.Degraded(llm.DegradedModelNotResident), paths: []string{"/health", "/v1/models"}},
		{name: "401 falls back and models refuse too", routes: map[string]route{
			openaicompat.PathHealth: {code: 401, body: []byte(`{}`)}, openaicompat.PathModels: {code: 401, body: []byte(`{}`)}},
			want: llm.StatusUnavailable, paths: []string{"/health", "/v1/models"}},
		{name: "404 everywhere", routes: map[string]route{},
			want: llm.StatusUnavailable, paths: []string{"/health", "/v1/models"}},
		{name: "models loading after health", routes: map[string]route{
			openaicompat.PathHealth: okFixture(t, "health-ok.json"), openaicompat.PathModels: {code: 503, body: loadingLlama}},
			required: []string{model}, want: llm.StatusLoading, paths: []string{"/health", "/v1/models"}},
		{name: "models broken after health", routes: map[string]route{
			openaicompat.PathHealth: okFixture(t, "health-ok.json"), openaicompat.PathModels: {code: 500, body: []byte(`{}`)}},
			required: []string{model}, want: llm.StatusUnavailable, paths: []string{"/health", "/v1/models"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStand(t, tc.routes)
			got := s.provider(t, openaicompat.WithRequiredModels(tc.required...)).Health(context.Background())
			if got != tc.want {
				t.Errorf("Health = %s, want %s", got, tc.want)
			}
			var paths []string
			for _, r := range s.requests() {
				paths = append(paths, r.Method+" "+r.Path)
			}
			var want []string
			for _, p := range tc.paths {
				want = append(want, http.MethodGet+" "+p)
			}
			if !reflect.DeepEqual(paths, want) {
				t.Errorf("requests = %v, want %v", paths, want)
			}
		})
	}
}

func TestHealthOfAServerThatIsGone(t *testing.T) {
	s := newStand(t, nil)
	p := s.provider(t)
	s.srv.Close()
	if got := p.Health(context.Background()); got != llm.StatusUnavailable {
		t.Errorf("Health = %s, want unavailable", got)
	}
}

func TestEmbed(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathEmbeddings: okFixture(t, "embeddings.json")})
	p := s.provider(t)
	vectors, err := p.Embed(context.Background(), "nomic-embed-text-v1.5", []string{"волк", "тропа"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	want := [][]float32{{-0.5, 0.25, -0.125, 1.0}, {0.5, -0.25, 0.125, 0.0}}
	if !reflect.DeepEqual(vectors, want) {
		t.Errorf("Embed = %v, want %v (in the order of index)", vectors, want)
	}
	r := only(t, s)
	body := bodyOf(t, r)
	if r.Path != "/v1/embeddings" || body["model"] != "nomic-embed-text-v1.5" ||
		!reflect.DeepEqual(body["input"], []any{"волк", "тропа"}) || body["encoding_format"] != "float" {
		t.Errorf("request = %s %s", r.Path, r.Body)
	}

	none, err := p.Embed(context.Background(), "nomic-embed-text-v1.5", nil)
	if err != nil || none == nil || len(none) != 0 || len(s.requests()) != 1 {
		t.Errorf("Embed of no texts = %v, %v after %d requests; want an empty result without a request", none, err, len(s.requests()))
	}
	if _, err := p.Embed(context.Background(), "", []string{"a"}); err == nil {
		t.Error("Embed without a model: no error")
	}
}

func TestEmbedRefusesAnAnswerThatDoesNotFit(t *testing.T) {
	for name, body := range map[string]string{
		"fewer vectors":   `{"data":[{"embedding":[1],"index":0}]}`,
		"index repeated":  `{"data":[{"embedding":[1],"index":0},{"embedding":[2],"index":0}]}`,
		"index too large": `{"data":[{"embedding":[1],"index":0},{"embedding":[2],"index":2}]}`,
		"empty vector":    `{"data":[{"embedding":[],"index":0},{"embedding":[2],"index":1}]}`,
		"nested vectors":  `{"data":[{"embedding":[[1]],"index":0},{"embedding":[[2]],"index":1}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newStand(t, map[string]route{openaicompat.PathEmbeddings: {code: http.StatusOK, body: []byte(body)}})
			if _, err := s.provider(t).Embed(context.Background(), "m", []string{"a", "b"}); !errors.Is(err, openaicompat.ErrMalformedResponse) {
				t.Errorf("Embed error = %v, want ErrMalformedResponse", err)
			}
		})
	}
	s := newStand(t, map[string]route{openaicompat.PathEmbeddings: {code: http.StatusOK,
		body: []byte(`{"data":[{"embedding":[1]},{"embedding":[2]}]}`)}})
	vectors, err := s.provider(t).Embed(context.Background(), "m", []string{"a", "b"})
	if err != nil || !reflect.DeepEqual(vectors, [][]float32{{1}, {2}}) {
		t.Errorf("without index: %v, %v; want the order of the answer", vectors, err)
	}
}

func TestGenerateRefusesABadRequestBeforeSendingIt(t *testing.T) {
	s := newStand(t, map[string]route{openaicompat.PathChat: okFixture(t, "chat-schema.json")})
	p := s.provider(t)
	noModel := narrativeRequest()
	noModel.Model = ""
	if _, err := p.Generate(context.Background(), noModel); err == nil {
		t.Error("Generate without a model: no error")
	}
	badSchema := narrativeRequest()
	badSchema.Schema = json.RawMessage(`{"type":`)
	if _, err := p.Generate(context.Background(), badSchema); err == nil {
		t.Error("Generate with a schema that is not JSON: no error")
	}
	if n := len(s.requests()); n != 0 {
		t.Errorf("server got %d requests, want 0", n)
	}
}

func TestErrorsOfTheServer(t *testing.T) {
	const keyEcho = "Incorrect API key provided: sk-t208****tail"
	for _, tc := range []struct {
		name        string
		route       route
		unavailable bool
		malformed   bool
		code        int
	}{
		{name: "503", route: route{code: 503, body: fixture(t, "health-loading.json")}, unavailable: true, code: 503},
		{name: "500", route: route{code: 500, body: []byte(`{"error":{"message":"` + keyEcho + `"}}`)}, unavailable: true, code: 500},
		{name: "429", route: route{code: 429, body: []byte(`{}`)}, unavailable: true, code: 429},
		{name: "408", route: route{code: 408, body: []byte(`{}`)}, unavailable: true, code: 408},
		{name: "400", route: route{code: 400, body: []byte(`{"error":{"message":"` + keyEcho + `"}}`)}, code: 400},
		{name: "401", route: route{code: 401, body: []byte(`{"error":{"message":"` + keyEcho + `"}}`)}, code: 401},
		{name: "not json", route: route{code: 200, body: []byte(`<html>` + keyEcho + `</html>`)}, malformed: true},
		{name: "no choices", route: route{code: 200, body: []byte(`{"choices":[]}`)}, malformed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStand(t, map[string]route{openaicompat.PathChat: tc.route})
			_, err := s.provider(t).Generate(context.Background(), narrativeRequest())
			if err == nil {
				t.Fatal("Generate: no error")
			}
			if errors.Is(err, llm.ErrUnavailable) != tc.unavailable {
				t.Errorf("errors.Is(%v, ErrUnavailable) = %v, want %v", err, !tc.unavailable, tc.unavailable)
			}
			if errors.Is(err, openaicompat.ErrMalformedResponse) != tc.malformed {
				t.Errorf("errors.Is(%v, ErrMalformedResponse) = %v, want %v", err, !tc.malformed, tc.malformed)
			}
			var re *openaicompat.RequestError
			if !errors.As(err, &re) || re.StatusCode != tc.code || re.Path != openaicompat.PathChat {
				t.Errorf("error = %#v, want a RequestError of %s with code %d", err, openaicompat.PathChat, tc.code)
			}
			for _, leak := range []string{"sk-t208", "Incorrect API key", s.srv.URL, "127.0.0.1"} {
				if strings.Contains(err.Error(), leak) {
					t.Errorf("error %q carries %q", err, leak)
				}
			}
		})
	}
}

func TestARedirectIsNotFollowed(t *testing.T) {
	var elsewhere int
	var mu sync.Mutex
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		elsewhere++
		mu.Unlock()
		_, _ = w.Write(fixture(t, "chat-schema.json"))
	}))
	defer vendor.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, vendor.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer local.Close()

	p, err := openaicompat.New(llm.Config{URL: local.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.CloseIdleConnections()
	_, err = p.Generate(context.Background(), narrativeRequest())
	var re *openaicompat.RequestError
	if !errors.As(err, &re) || re.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("Generate error = %v, want a RequestError with 307", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if elsewhere != 0 {
		t.Errorf("the redirect target got %d requests", elsewhere)
	}
}

func TestNoAnswerIsUnavailableAndNamesNoAddressWithAnAt(t *testing.T) {
	refuse := func(_ context.Context, _, addr string) (net.Conn, error) {
		return nil, errors.New("dial " + addr + ": refused by the test")
	}
	plain, err := openaicompat.New(llm.Config{URL: "http://10.0.0.5:8080"}, openaicompat.WithDialContext(refuse))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = plain.Generate(context.Background(), narrativeRequest())
	if !errors.Is(err, llm.ErrUnavailable) || !strings.Contains(err.Error(), "refused by the test") {
		t.Errorf("no answer: %v, want ErrUnavailable with the reason", err)
	}
	if strings.Contains(err.Error(), "http://") {
		t.Errorf("the error prints the URL: %v", err)
	}

	// url.Parse reads the host pw and the port 2024 here, where the head of a
	// password may stand (T-451).
	at, err := openaicompat.New(llm.Config{URL: "http://pw:2024/x@10.0.0.5:8080"}, openaicompat.WithDialContext(refuse))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = at.Generate(context.Background(), narrativeRequest())
	if !errors.Is(err, llm.ErrUnavailable) {
		t.Errorf("no answer: %v, want ErrUnavailable", err)
	}
	for _, leak := range []string{"pw", "2024", "10.0.0.5", "refused by the test"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error %q names %q of an address with an @", err, leak)
		}
	}
}
