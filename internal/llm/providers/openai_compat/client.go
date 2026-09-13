// Package openaicompat is the LLM provider of an OpenAI-compatible server, and
// the default provider of the platform: llama-server on the operator's machine,
// or a vendor behind the cloud gate (C-15 v1.1, ADR-005 add. 2 p. 1-4).
//
// It speaks net/http without an SDK: POST /v1/chat/completions with the
// json_schema structured output and the sampling of the request, POST
// /v1/embeddings, GET /v1/models and GET /health. It records nothing and
// publishes nothing: the records, the budget, the parser and the guardian are
// the gateway's (C-15 "Гарантии").
//
// Like every provider it does not register itself; the wiring of a process
// does: providers.Register(llm.ProviderOpenAICompat, openaicompat.Factory()).
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"multiverse-core.io/internal/llm"
	"multiverse-core.io/internal/llm/providers"
	"multiverse-core.io/shared/clock"
)

// The paths under the base address. MV_LLM_URL holds the base address; a /v1
// written at its end is dropped, so the provider always appends the whole path
// (C-15, T-404).
const (
	PathChat       = "/v1/chat/completions"
	PathEmbeddings = "/v1/embeddings"
	PathModels     = "/v1/models"
	PathHealth     = "/health"
)

// The sampling a phase gets for a field its Params leave at zero: the
// non-thinking profile of Qwen3 (ADR-005 add. 2 p. 1). The server defaults of
// llama-server are the thinking profile (--temp 1 --top-p 0.95), so every field
// is sent on every request and the server never decides one. A blueprint sets
// temperature and max_tokens only (T-203); the rest comes from here.
const (
	DefaultTemperature     = 0.7
	DefaultTopP            = 0.8
	DefaultTopK            = 20
	DefaultMinP            = 0.0
	DefaultPresencePenalty = 1.5
)

// charsPerToken turns a length into tokens when the server reports no usage.
// It is the assumption of ADR-005 (clarification of T-435 p. 4) for Cyrillic
// until /tokenize measures it on the stand.
const charsPerToken = 2.5

// maxBody bounds an answer the provider reads into memory. A chat answer of
// MVP-1 is a few kilobytes and a batch of embeddings a few hundred; a server
// that sends more is broken, and the process must not pay for it.
const maxBody = 32 << 20

// ErrMalformedResponse is a 200 answer the provider cannot read: not JSON, no
// choices, a count of embeddings that does not match the texts, a body past
// maxBody.
var ErrMalformedResponse = errors.New("llm/providers/openai_compat: malformed response")

// Option configures a Provider.
type Option func(*Provider)

// WithClock sets the clock the latency of an answer is measured by when the
// server reports no timings. The default is clock.Real.
func WithClock(c clock.Clock) Option {
	return func(p *Provider) { p.clock = c }
}

// WithLogger sets the logger of the provider. The default is slog.Default.
func WithLogger(l *slog.Logger) Option {
	return func(p *Provider) { p.log = l }
}

// WithRequiredModels names the models Health expects the server to serve —
// the models of the blueprints (C-11 rule 7a). A server that does not list one
// of them is degraded(model_not_resident). Names are compared as they are: the
// id of a model is what the server lists, and nothing is derived from the path
// of a file (T-437). Without the option Health does not look at the models.
func WithRequiredModels(models ...string) Option {
	return func(p *Provider) {
		for _, m := range models {
			if m != "" && !slices.Contains(p.required, m) {
				p.required = append(p.required, m)
			}
		}
	}
}

// Provider is the openai_compat llm.Provider. It is safe for concurrent use.
type Provider struct {
	base   string
	apiKey llm.Secret
	// withheld is true for a base address that holds an @: what stands in front
	// of it may be a key where url.Parse sees a host, so no error of the provider
	// names the host or repeats a message of the network stack (aboutTheHost of
	// internal/llm, T-451).
	withheld  bool
	transport *http.Transport
	client    *http.Client
	clock     clock.Clock
	log       *slog.Logger
	required  []string
}

var _ llm.Provider = (*Provider)(nil)

// New builds the provider of cfg. It runs the cloud gate itself, so a provider
// built without the registry cannot skip it either: an empty or invalid
// MV_LLM_URL is a configuration error, and a non-local one needs
// MV_LLM_CLOUD_ENABLED=true (ADR-005 add. 2 p. 3).
func New(cfg llm.Config, opts ...Option) (*Provider, error) {
	switch cfg.Provider {
	case "", llm.ProviderOpenAICompat:
		cfg.Provider = llm.ProviderOpenAICompat
	default:
		return nil, fmt.Errorf("llm/providers/openai_compat: a configuration of provider %q", cfg.Provider)
	}
	if err := cfg.CheckCloudGate(); err != nil {
		return nil, err
	}
	local, err := llm.IsLocalEndpoint(cfg.URL)
	if err != nil {
		return nil, err
	}
	base, err := baseAddress(cfg.URL)
	if err != nil {
		return nil, err
	}

	transport := newTransport()
	// http.ProxyFromEnvironment bypasses a proxy for loopback and localhost only.
	// A local address of the LAN or a service of compose would go through
	// HTTPS_PROXY, out of the machine, with the prompt in the body — past the
	// gate that exists to keep it in (приёмка T-451).
	if local {
		transport.Proxy = nil
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}

	p := &Provider{
		base:      base,
		apiKey:    cfg.APIKey,
		withheld:  strings.Contains(cfg.URL, "@"),
		transport: transport,
		clock:     clock.Real{},
		log:       slog.Default(),
	}
	p.client = &http.Client{
		Transport: transport,
		// A redirect is refused, not followed: a local server that answered 307
		// with a vendor's address would send the prompt past the cloud gate.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

// Factory returns the factory of providers.Registry: every call builds a new
// provider of the configuration with opts.
func Factory(opts ...Option) providers.Factory {
	return func(cfg llm.Config) (llm.Provider, error) {
		p, err := New(cfg, opts...)
		if err != nil {
			return nil, err
		}
		return p, nil
	}
}

func newTransport() *http.Transport {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		return t.Clone()
	}
	return &http.Transport{}
}

// baseAddress drops a trailing slash and a trailing /v1, the rule LoadConfig
// applies to MV_LLM_URL. It is applied again here for a Config assembled by
// hand: both spellings must give one request URL.
func baseAddress(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("llm/providers/openai_compat: the base address is not a URL (the value is not printed: it may hold a key)")
	}
	u.Path = strings.TrimRight(strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/v1"), "/")
	u.RawPath = ""
	return u.String(), nil
}

// CloseIdleConnections closes the connections kept alive for the next request.
func (p *Provider) CloseIdleConnections() { p.transport.CloseIdleConnections() }

// Generate sends one chat completion. Request.Timeout, when set, bounds the
// call through the context; a context done before the answer closes the
// connection and returns an error that matches the error of the context
// (ADR-014 p. 2).
//
// reasoning_content of the answer is not kept: its length in characters goes
// to Response.ReasoningLen and the text is dropped here (ADR-005 add. 2 p. 1).
func (p *Provider) Generate(ctx context.Context, req llm.Request) (llm.Response, error) {
	if req.Model == "" {
		return llm.Response{}, errors.New("llm/providers/openai_compat: the request names no model")
	}
	body, err := chatBody(req)
	if err != nil {
		return llm.Response{}, err
	}
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	start := p.clock.Now()
	var out chatResponse
	if err := p.call(ctx, http.MethodPost, PathChat, body, &out); err != nil {
		return llm.Response{}, err
	}
	elapsed := p.clock.Now().Sub(start)
	if len(out.Choices) == 0 {
		return llm.Response{}, malformed(http.MethodPost, PathChat, "the answer has no choices")
	}

	msg := out.Choices[0].Message
	content, reasoning := deref(msg.Content), deref(msg.ReasoningContent)
	tokens, estimated := tokensOf(out.Usage, req, content, reasoning)
	if estimated {
		p.log.WarnContext(ctx, "llm tokens estimated from length: the server reported no usage",
			slog.String("provider", llm.ProviderOpenAICompat),
			slog.String("model", req.Model),
			slog.String("phase", string(req.Phase)))
	}
	return llm.Response{
		Content:      content,
		ReasoningLen: utf8.RuneCountInString(reasoning),
		Tokens:       tokens,
		LatencyMs:    latencyOf(out.Timings, elapsed),
		Provider:     llm.ProviderOpenAICompat,
		// The model of the request, not the one the server echoes: a vendor
		// answers with a dated snapshot name the price table has no key for, and
		// the records name the model the blueprint asked for.
		Model: req.Model,
	}, nil
}

// Embed returns one vector per text, in the order of the texts.
func (p *Provider) Embed(ctx context.Context, model string, texts []string) ([][]float32, error) {
	if model == "" {
		return nil, errors.New("llm/providers/openai_compat: the embedding request names no model")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	body, err := json.Marshal(embeddingRequest{Model: model, Input: texts, EncodingFormat: "float"})
	if err != nil {
		return nil, fmt.Errorf("llm/providers/openai_compat: encode the embedding request: %w", err)
	}
	var out embeddingResponse
	if err := p.call(ctx, http.MethodPost, PathEmbeddings, body, &out); err != nil {
		return nil, err
	}
	if len(out.Data) != len(texts) {
		return nil, malformed(http.MethodPost, PathEmbeddings,
			fmt.Sprintf("%d vectors for %d texts", len(out.Data), len(texts)))
	}
	vectors := make([][]float32, len(texts))
	for i, d := range out.Data {
		at := i
		if d.Index != nil {
			at = *d.Index
		}
		if at < 0 || at >= len(texts) || vectors[at] != nil {
			return nil, malformed(http.MethodPost, PathEmbeddings, fmt.Sprintf("vector %d has index %d", i, at))
		}
		if len(d.Embedding) == 0 {
			return nil, malformed(http.MethodPost, PathEmbeddings, fmt.Sprintf("vector %d is empty", at))
		}
		vectors[at] = d.Embedding
	}
	return vectors, nil
}

// Models lists the ids of GET /v1/models in the order the server gives them.
func (p *Provider) Models(ctx context.Context) ([]string, error) {
	var out modelList
	if err := p.call(ctx, http.MethodGet, PathModels, nil, &out); err != nil {
		return nil, err
	}
	if out.Data == nil {
		return nil, malformed(http.MethodGet, PathModels, "the answer has no data")
	}
	ids := make([]string, 0, len(*out.Data))
	for _, m := range *out.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// Health asks GET /health: 200 is up, 503 is a model still loading — loading,
// not unavailable, so the world does not fall back to templates for the minute
// a restart takes.
//
// /health belongs to llama.cpp and not to the contract: a server may serve the
// OpenAI surface without it (the owner's stand answers 404, a vendor 401, 404 or
// 405). Those codes fall back to GET /v1/models, the rule of the scripts
// (llm_health_probe, T-403). Every other answer, and no answer, is unavailable.
//
// A server that is up is then asked for its models when WithRequiredModels named
// any: a model it does not list gives degraded(model_not_resident).
func (p *Provider) Health(ctx context.Context) llm.Status {
	code, err := p.probe(ctx, PathHealth)
	if err != nil {
		return llm.StatusUnavailable
	}
	switch code {
	case http.StatusOK:
		if len(p.required) == 0 {
			return llm.StatusOK
		}
	case http.StatusServiceUnavailable:
		return llm.StatusLoading
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
		http.StatusMethodNotAllowed, http.StatusNotImplemented:
	default:
		return llm.StatusUnavailable
	}

	models, err := p.Models(ctx)
	if err != nil {
		var re *RequestError
		if errors.As(err, &re) && re.StatusCode == http.StatusServiceUnavailable {
			return llm.StatusLoading
		}
		return llm.StatusUnavailable
	}
	for _, m := range p.required {
		if !slices.Contains(models, m) {
			return llm.Degraded(llm.DegradedModelNotResident)
		}
	}
	return llm.StatusOK
}

// probe returns the status code of GET path, the body read and dropped.
func (p *Provider) probe(ctx context.Context, path string) (int, error) {
	resp, err := p.send(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBody))
	return resp.StatusCode, nil
}

// call sends a request and decodes a 200 answer into out.
func (p *Provider) call(ctx context.Context, method, path string, body []byte, out any) error {
	resp, err := p.send(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// The body is not read into the error: a vendor's refusal of a key quotes
		// the head and the tail of that key, and a refusal of a request may quote
		// the prompt (SEC-21).
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBody))
		return statusError(method, path, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return p.transportError(ctx, method, path, err)
	}
	if len(data) > maxBody {
		return malformed(method, path, fmt.Sprintf("the answer is longer than %d bytes", maxBody))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return malformed(method, path, err.Error())
	}
	return nil
}

func (p *Provider) send(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.base+path, reader)
	if err != nil {
		return nil, &RequestError{Method: method, Path: path, Detail: "the request cannot be built"}
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.apiKey.IsSet() {
		req.Header.Set("Authorization", "Bearer "+p.apiKey.Reveal())
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, p.transportError(ctx, method, path, err)
	}
	return resp, nil
}

// transportError is a request that got no whole answer. A done context is
// reported as itself; anything else is the endpoint being unavailable. The
// *url.Error of the client is taken apart, because it prints the whole URL.
func (p *Provider) transportError(ctx context.Context, method, path string, err error) error {
	if cerr := ctx.Err(); cerr != nil {
		return &RequestError{Method: method, Path: path, Detail: "the call was given up", cause: cerr}
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	detail := "the endpoint did not answer"
	if p.withheld {
		detail += " (the reason is not printed: the address holds an @, and it may name a key)"
	} else {
		detail += ": " + err.Error()
	}
	return &RequestError{Method: method, Path: path, Detail: detail, cause: llm.ErrUnavailable}
}

// RequestError is a call that failed: no answer, an answer other than 200, or
// an answer that cannot be read. It names the method and the path, never the
// URL, the key or the body of an answer; the host and the port of a network
// error are named unless the address holds an @ (T-451).
//
// errors.Is(err, llm.ErrUnavailable) holds for no answer and for the codes a
// later attempt may not get — 408, 429 and 5xx; a done context matches its own
// error; an unreadable answer matches ErrMalformedResponse. Other codes (400,
// 401, 404, …) match none of them: another attempt sends the same request.
type RequestError struct {
	Method string
	Path   string
	// StatusCode is the code of the answer, 0 when there was none.
	StatusCode int
	Detail     string
	cause      error
}

func (e *RequestError) Error() string {
	msg := "llm/providers/openai_compat: " + e.Method + " " + e.Path
	if e.StatusCode != 0 {
		msg += fmt.Sprintf(": HTTP %d", e.StatusCode)
	}
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	return msg
}

func (e *RequestError) Unwrap() error { return e.cause }

func statusError(method, path string, code int) error {
	e := &RequestError{Method: method, Path: path, StatusCode: code}
	if code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code >= 500 {
		e.cause = llm.ErrUnavailable
	}
	if code == http.StatusServiceUnavailable {
		e.Detail = "the model is loading or the server is overloaded"
	}
	if code >= 300 && code < 400 {
		e.Detail = "a redirect is not followed"
	}
	return e
}

func malformed(method, path, detail string) error {
	return &RequestError{Method: method, Path: path, Detail: detail, cause: ErrMalformedResponse}
}

// chatBody is the body of POST /v1/chat/completions for req.
func chatBody(req llm.Request) ([]byte, error) {
	messages := make([]wireMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		messages = append(messages, wireMessage{Role: string(llm.RoleSystem), Content: req.System})
	}
	for _, m := range req.Messages {
		messages = append(messages, wireMessage{Role: string(m.Role), Content: m.Content})
	}
	params := req.Params
	body := chatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   false,
		// Sent with and without a schema: with thinking on, llama-server does not
		// apply the json_schema grammar (llama.cpp #20345), and MVP-1 turns it off
		// for every phase.
		ChatTemplateKwargs: chatTemplateKwargs{EnableThinking: params.Think},
		Temperature:        orDefault(params.Temperature, DefaultTemperature),
		TopP:               orDefault(params.TopP, DefaultTopP),
		TopK:               params.TopK,
		MinP:               orDefault(params.MinP, DefaultMinP),
		PresencePenalty:    orDefault(params.PresencePenalty, DefaultPresencePenalty),
		MaxTokens:          params.MaxTokens,
	}
	if body.TopK == 0 {
		body.TopK = DefaultTopK
	}
	if len(req.Schema) > 0 {
		if !json.Valid(req.Schema) {
			return nil, errors.New("llm/providers/openai_compat: the schema of the request is not valid JSON")
		}
		body.ResponseFormat = &responseFormat{
			Type:       "json_schema",
			JSONSchema: jsonSchema{Name: schemaName(req.Phase), Schema: req.Schema},
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("llm/providers/openai_compat: encode the chat request: %w", err)
	}
	return data, nil
}

// schemaName is json_schema.name: the phase, which is [a-z]+ and so a name
// every OpenAI-compatible server accepts.
func schemaName(phase llm.Phase) string {
	if phase == "" {
		return "response"
	}
	return string(phase)
}

// orDefault reads a zero field of Params as "not set" (llm.Params).
func orDefault(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

// tokensOf takes the tokens from usage. A count the server did not report is
// estimated from the length of the text, and the second result says so.
func tokensOf(u *usage, req llm.Request, content, reasoning string) (llm.Tokens, bool) {
	var t llm.Tokens
	estimated := false
	if u != nil && u.PromptTokens != nil {
		t.Prompt = *u.PromptTokens
	} else {
		n := utf8.RuneCountInString(req.System)
		for _, m := range req.Messages {
			n += utf8.RuneCountInString(m.Content)
		}
		t.Prompt, estimated = estimate(n), true
	}
	if u != nil && u.CompletionTokens != nil {
		t.Completion = *u.CompletionTokens
	} else {
		// The reasoning is generated too, and a server counts it as completion.
		t.Completion, estimated = estimate(utf8.RuneCountInString(content)+utf8.RuneCountInString(reasoning)), true
	}
	if u != nil && u.PromptTokensDetails != nil {
		t.Cached = u.PromptTokensDetails.CachedTokens
	}
	return t, estimated
}

func estimate(chars int) int {
	return int(math.Ceil(float64(chars) / charsPerToken))
}

// latencyOf is the time the server spent on the prompt and on the answer when
// it reports both, and the time of the whole call on the clock otherwise.
func latencyOf(t *timings, elapsed time.Duration) int64 {
	if t != nil && t.PromptMS != nil && t.PredictedMS != nil && *t.PromptMS >= 0 && *t.PredictedMS >= 0 {
		return int64(math.Round(*t.PromptMS + *t.PredictedMS))
	}
	return elapsed.Milliseconds()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
