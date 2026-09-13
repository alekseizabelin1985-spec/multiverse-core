// Package fake is the in-memory LLM provider of tests (C-15 "Заглушка"): a
// table of rules (phase, matcher) → replies, a counter of the calls that
// reached it, a reply delay driven by shared/clock and generators of the
// answers a real model gives — a preamble, a <think> block, a code fence, call
// labels of ADR-029.
//
// The package never talks to the network and never reads the environment or
// the wall clock. Outside internal/llm the linter admits it only into the
// tests of swarm and memory, and into cmd/* (ADR-001 addendum 2026-09-13 p. 3,
// boundary 1c; C-15 v1.2).
package fake

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"multiverse-core.io/internal/llm"
	"multiverse-core.io/internal/llm/providers"
	"multiverse-core.io/shared/clock"
)

// ErrNoMatch is a request no rule of the table answers. It is an error and not
// an empty answer: a test that forgot a rule must fail where the request was
// made, not three stages later in the parser.
var ErrNoMatch = errors.New("llm/providers/fake: no rule matches the request")

// DefaultEmbedDim is the length of an embedding when WithEmbedDim is not given.
const DefaultEmbedDim = 8

// Matcher selects the requests a rule answers. It runs under the lock of the
// provider and must not call the provider back.
type Matcher func(llm.Request) bool

// Any matches every request.
func Any() Matcher { return func(llm.Request) bool { return true } }

// ModelIs matches a request for model.
func ModelIs(model string) Matcher {
	return func(r llm.Request) bool { return r.Model == model }
}

// CorrelationIs matches a request of the chain id.
func CorrelationIs(id string) Matcher {
	return func(r llm.Request) bool { return r.CorrelationID == id }
}

// SystemContains matches a request whose system prompt contains s.
func SystemContains(s string) Matcher {
	return func(r llm.Request) bool { return strings.Contains(r.System, s) }
}

// UserContains matches a request whose last user message contains s. The last
// one, because that is where the gateway adds the hint of a retry (КД §9.2):
// a rule for the retry is told apart from a rule for the first attempt by it.
func UserContains(s string) Matcher {
	return func(r llm.Request) bool {
		for i := len(r.Messages) - 1; i >= 0; i-- {
			if r.Messages[i].Role == llm.RoleUser {
				return strings.Contains(r.Messages[i].Content, s)
			}
		}
		return false
	}
}

// All matches a request every matcher matches; All() matches everything.
func All(ms ...Matcher) Matcher {
	return func(r llm.Request) bool {
		for _, m := range ms {
			if !m(r) {
				return false
			}
		}
		return true
	}
}

// Reply is one answer of a rule.
type Reply struct {
	Response llm.Response
	// Err is returned instead of the response when set.
	Err error
	// Delay holds the answer back on the Timers of the provider. A context
	// done while waiting ends the call with the error of the context, the way
	// a real provider gives up a cancelled request (ADR-014 p. 2).
	Delay time.Duration
}

// Rule answers the requests of Phase that Match selects. An empty Phase is
// every phase and a nil Match every request. Replies are handed out in order;
// the last one repeats, so a one-reply rule answers forever and a rule
// "dirty, then clean" models a retry.
//
// A call uses up its reply when the rule is chosen, before the context is
// looked at: a call made with a done context, or cancelled during the Delay,
// consumes the reply it would have got. A yielded call queued again gets the
// next reply, as the second request to a real model gets a second answer.
type Rule struct {
	Phase   llm.Phase
	Match   Matcher
	Replies []Reply
}

// Option configures a Provider.
type Option func(*Provider)

// WithRules adds rules to the table, after those already there.
func WithRules(rules ...Rule) Option {
	return func(p *Provider) {
		for _, r := range rules {
			p.addLocked(r)
		}
	}
}

// WithModels sets what Models returns. Without it Models lists the models
// named by the replies of the table.
func WithModels(models ...string) Option {
	return func(p *Provider) { p.models = append([]string{}, models...) }
}

// WithTimers sets the timers a Delay waits on. The default is
// clock.RealTimers; a test that drives time passes clock.Manual.Timers() and
// learns that the call waits from WithOnDelay.
func WithTimers(t clock.Timers) Option {
	return func(p *Provider) { p.timers = t }
}

// WithOnDelay registers a hook called once the timer of a delayed reply is
// armed, before the call waits on it. A test that advances a manual clock
// waits for the hook first; advancing earlier would find no timer to fire.
func WithOnDelay(hook func(llm.Request)) Option {
	return func(p *Provider) { p.onDelay = hook }
}

// WithHealth sets the initial status; the default is ok.
func WithHealth(s llm.Status) Option {
	return func(p *Provider) { p.health = s }
}

// WithEmbedDim sets the length of an embedding vector.
func WithEmbedDim(n int) Option {
	return func(p *Provider) { p.embedDim = n }
}

// Provider is the fake llm.Provider. It is safe for concurrent use.
type Provider struct {
	mu         sync.Mutex
	rules      []*rule
	models     []string
	timers     clock.Timers
	onDelay    func(llm.Request)
	health     llm.Status
	embedDim   int
	calls      int
	perPhase   map[llm.Phase]int
	requests   []llm.Request
	embedCalls int
}

type rule struct {
	Rule
	next int
}

var _ llm.Provider = (*Provider)(nil)

// New builds a provider. A rule without replies or a non-positive embedding
// length is a programming error of the test and panics, like
// providers.Register does.
func New(opts ...Option) *Provider {
	p := &Provider{
		timers:   clock.RealTimers{},
		health:   llm.StatusOK,
		embedDim: DefaultEmbedDim,
		perPhase: make(map[llm.Phase]int),
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.embedDim <= 0 {
		panic(fmt.Sprintf("llm/providers/fake: embedding length %d is not positive", p.embedDim))
	}
	return p
}

// Factory returns a factory for providers.Registry that builds a new fake with
// opts each time. Nothing registers itself: the process wiring decides which
// providers exist, as for every other provider.
func Factory(opts ...Option) providers.Factory {
	return func(llm.Config) (llm.Provider, error) { return New(opts...), nil }
}

// Add appends a rule to the table.
func (p *Provider) Add(r Rule) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.addLocked(r)
}

func (p *Provider) addLocked(r Rule) {
	if len(r.Replies) == 0 {
		panic(fmt.Sprintf("llm/providers/fake: rule for phase %q has no replies", r.Phase))
	}
	r.Replies = slices.Clone(r.Replies)
	p.rules = append(p.rules, &rule{Rule: r})
}

// Generate answers from the first rule, in the order of the table, that
// matches the request. Every call counts, the ones no rule answers and the
// cancelled ones — done before the call or during the delay — included: the
// counter says how many times the gateway went to a provider.
func (p *Provider) Generate(ctx context.Context, req llm.Request) (llm.Response, error) {
	reply, err := p.take(req)
	if err != nil {
		return llm.Response{}, err
	}
	if reply.Delay > 0 {
		if err := p.wait(ctx, req, reply.Delay); err != nil {
			return llm.Response{}, err
		}
	} else if err := ctx.Err(); err != nil {
		return llm.Response{}, err
	}
	if reply.Err != nil {
		return llm.Response{}, reply.Err
	}
	resp := reply.Response
	if resp.Provider == "" {
		resp.Provider = llm.ProviderFake
	}
	if resp.Model == "" {
		resp.Model = req.Model
	}
	if resp.LatencyMs == 0 {
		resp.LatencyMs = reply.Delay.Milliseconds()
	}
	return resp, nil
}

// take counts the call and picks its reply under one lock, so that two
// concurrent calls of one rule get two consecutive replies.
func (p *Provider) take(req llm.Request) (Reply, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.perPhase[req.Phase]++
	p.requests = append(p.requests, cloneRequest(req))
	for _, r := range p.rules {
		if r.Phase != "" && r.Phase != req.Phase {
			continue
		}
		if r.Match != nil && !r.Match(req) {
			continue
		}
		reply := r.Replies[r.next]
		if r.next < len(r.Replies)-1 {
			r.next++
		}
		return reply, nil
	}
	return Reply{}, fmt.Errorf("%w: phase %q, model %q", ErrNoMatch, req.Phase, req.Model)
}

func (p *Provider) wait(ctx context.Context, req llm.Request, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	timers, hook := p.timers, p.onDelay
	p.mu.Unlock()
	timer := timers.After(d)
	defer timer.Stop()
	if hook != nil {
		hook(req)
	}
	select {
	case <-timer.C():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Embed returns a vector per text that depends on the model and the text
// only: the first bytes of SHA-256 over them, mapped to [-1, 1]. Equal inputs
// give equal vectors in every run, which is all an index test needs (C-15).
func (p *Provider) Embed(ctx context.Context, model string, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.embedCalls++
	dim := p.embedDim
	p.mu.Unlock()
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = embedding(model, text, dim)
	}
	return out, nil
}

// embedding stretches the digest past 32 bytes by hashing a block counter in
// front of the input; the zero byte keeps "ab"+"c" and "a"+"bc" apart.
func embedding(model, text string, dim int) []float32 {
	const perBlock = sha256.Size / 4
	vec := make([]float32, dim)
	var block []byte
	for i := range vec {
		if i%perBlock == 0 {
			h := sha256.New()
			_ = binary.Write(h, binary.BigEndian, uint32(i/perBlock))
			h.Write([]byte(model))
			h.Write([]byte{0})
			h.Write([]byte(text))
			block = h.Sum(nil)
		}
		u := binary.BigEndian.Uint32(block[(i%perBlock)*4:])
		vec[i] = float32(float64(u)/float64(^uint32(0))*2 - 1)
	}
	return vec
}

// Health returns the status set by WithHealth or SetHealth.
func (p *Provider) Health(context.Context) llm.Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.health
}

// SetHealth changes the status, for tests of degradation and recovery.
func (p *Provider) SetHealth(s llm.Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health = s
}

// Models returns the models of WithModels, or else the distinct non-empty
// models of the replies in the table, sorted.
func (p *Provider) Models(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.models != nil {
		return slices.Clone(p.models), nil
	}
	var models []string
	for _, r := range p.rules {
		for _, reply := range r.Replies {
			if m := reply.Response.Model; m != "" && !slices.Contains(models, m) {
				models = append(models, m)
			}
		}
	}
	slices.Sort(models)
	return models, nil
}

// Calls is the number of Generate calls so far — llm_calls of NFR-014: a
// replay that went to this provider even once is not a replay.
func (p *Provider) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// CallsFor is the number of Generate calls of phase.
func (p *Provider) CallsFor(phase llm.Phase) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.perPhase[phase]
}

// EmbedCalls is the number of Embed calls so far.
func (p *Provider) EmbedCalls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.embedCalls
}

// Requests returns copies of the requests of Generate, in the order they came.
func (p *Provider) Requests() []llm.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]llm.Request, len(p.requests))
	for i, r := range p.requests {
		out[i] = cloneRequest(r)
	}
	return out
}

// cloneRequest copies the slices of a request, so that neither the caller nor
// a reader of Requests can change what the provider saw.
func cloneRequest(r llm.Request) llm.Request {
	r.Messages = slices.Clone(r.Messages)
	r.Schema = slices.Clone(r.Schema)
	return r
}
