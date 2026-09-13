package fake_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/llm"
	"multiverse-core.io/internal/llm/providers"
	"multiverse-core.io/internal/llm/providers/fake"
	"multiverse-core.io/shared/clock"
)

func reply(content string) fake.Reply {
	return fake.Reply{Response: llm.Response{Content: content}}
}

func request(phase llm.Phase, user string) llm.Request {
	return llm.Request{
		Phase:         phase,
		Model:         "model-e",
		System:        "system of " + string(phase),
		Messages:      []llm.Message{{Role: llm.RoleUser, Content: user}},
		CorrelationID: "cid-1",
	}
}

func generate(t *testing.T, p *fake.Provider, req llm.Request) llm.Response {
	t.Helper()
	resp, err := p.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate(%s, %q): %v", req.Phase, req.Messages, err)
	}
	return resp
}

// The table is read in order and the first rule that matches both the phase
// and the matcher answers; an empty phase and a nil matcher match everything.
func TestTheFirstMatchingRuleAnswers(t *testing.T) {
	p := fake.New(fake.WithRules(
		fake.Rule{Phase: llm.PhaseNarrative, Match: fake.UserContains("wolf"), Replies: []fake.Reply{reply("narrative wolf")}},
		fake.Rule{Phase: llm.PhaseNarrative, Replies: []fake.Reply{reply("narrative any")}},
		fake.Rule{Phase: llm.PhaseTick, Match: fake.ModelIs("model-e"), Replies: []fake.Reply{reply("tick model-e")}},
		fake.Rule{Replies: []fake.Reply{reply("anything")}},
	))
	cases := []struct {
		req  llm.Request
		want string
	}{
		{request(llm.PhaseNarrative, "attack the wolf"), "narrative wolf"},
		{request(llm.PhaseNarrative, "look around"), "narrative any"},
		{request(llm.PhaseTick, "tick"), "tick model-e"},
		{func() llm.Request { r := request(llm.PhaseTick, "tick"); r.Model = "other"; return r }(), "anything"},
		{request(llm.PhaseOther, "x"), "anything"},
	}
	for _, c := range cases {
		if got := generate(t, p, c.req).Content; got != c.want {
			t.Errorf("Generate(%s, model %s, %q) = %q, want %q", c.req.Phase, c.req.Model, c.req.Messages[0].Content, got, c.want)
		}
	}
}

// A rule added later does not shadow an earlier one, Add included.
func TestAddAppendsAfterTheTable(t *testing.T) {
	p := fake.New(fake.WithRules(fake.Rule{Replies: []fake.Reply{reply("first")}}))
	p.Add(fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{reply("second")}})
	if got := generate(t, p, request(llm.PhaseTick, "")).Content; got != "first" {
		t.Fatalf("content = %q, want the rule of the table, which comes first", got)
	}
	q := fake.New()
	q.Add(fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{reply("added")}})
	if got := generate(t, q, request(llm.PhaseTick, "")).Content; got != "added" {
		t.Fatalf("content = %q, want added", got)
	}
}

// Replies are handed out in order and the last one repeats: "dirty, then
// clean" is a retry.
func TestRepliesAreConsumedInOrderAndTheLastRepeats(t *testing.T) {
	boom := errors.New("boom")
	p := fake.New(fake.WithRules(fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{
		{Err: boom},
		reply(fake.WithPreamble(`{"events":[]}`)),
		reply(`{"events":[]}`),
	}}))
	if _, err := p.Generate(context.Background(), request(llm.PhaseTick, "")); !errors.Is(err, boom) {
		t.Fatalf("attempt 1: err = %v, want boom", err)
	}
	want := []string{fake.WithPreamble(`{"events":[]}`), `{"events":[]}`, `{"events":[]}`, `{"events":[]}`}
	for i, w := range want {
		if got := generate(t, p, request(llm.PhaseTick, "")).Content; got != w {
			t.Fatalf("attempt %d: content = %q, want %q", i+2, got, w)
		}
	}
}

func TestMatchersCombine(t *testing.T) {
	req := request(llm.PhaseNarrative, "hint: answer in JSON")
	req.Messages = append([]llm.Message{{Role: llm.RoleUser, Content: "first turn"}, {Role: llm.RoleAssistant, Content: "wolf"}}, req.Messages...)
	cases := []struct {
		name string
		m    fake.Matcher
		want bool
	}{
		{"any", fake.Any(), true},
		{"model", fake.ModelIs("model-e"), true},
		{"other model", fake.ModelIs("model-q"), false},
		{"correlation", fake.CorrelationIs("cid-1"), true},
		{"other correlation", fake.CorrelationIs("cid-2"), false},
		{"system", fake.SystemContains("of narrative"), true},
		{"system miss", fake.SystemContains("tick"), false},
		{"last user message", fake.UserContains("hint"), true},
		{"an earlier user message", fake.UserContains("first turn"), false},
		{"an assistant message", fake.UserContains("wolf"), false},
		{"all", fake.All(fake.ModelIs("model-e"), fake.UserContains("hint")), true},
		{"all with a miss", fake.All(fake.ModelIs("model-e"), fake.UserContains("nope")), false},
		{"all of none", fake.All(), true},
	}
	for _, c := range cases {
		if got := c.m(req); got != c.want {
			t.Errorf("%s: matcher = %v, want %v", c.name, got, c.want)
		}
	}
	if fake.UserContains("x")(llm.Request{}) {
		t.Error("UserContains matched a request without user messages")
	}
}

// A request no rule answers is ErrNoMatch, and it still counts as a call.
func TestAMissIsAnErrorAndCounts(t *testing.T) {
	p := fake.New(fake.WithRules(fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{reply("tick")}}))
	resp, err := p.Generate(context.Background(), request(llm.PhaseNarrative, ""))
	if !errors.Is(err, fake.ErrNoMatch) {
		t.Fatalf("err = %v, want ErrNoMatch", err)
	}
	if resp != (llm.Response{}) {
		t.Fatalf("response of a miss = %+v, want zero", resp)
	}
	if p.Calls() != 1 || p.CallsFor(llm.PhaseNarrative) != 1 || p.CallsFor(llm.PhaseTick) != 0 {
		t.Fatalf("calls = %d, narrative = %d, tick = %d; want 1, 1, 0", p.Calls(), p.CallsFor(llm.PhaseNarrative), p.CallsFor(llm.PhaseTick))
	}
}

func TestCallsCountEveryGenerateByPhase(t *testing.T) {
	p := fake.New(fake.WithRules(fake.Rule{Replies: []fake.Reply{reply("x")}}))
	if p.Calls() != 0 {
		t.Fatalf("calls of a new provider = %d", p.Calls())
	}
	for range 3 {
		generate(t, p, request(llm.PhaseTick, ""))
	}
	generate(t, p, request(llm.PhaseNarrative, ""))
	if p.Calls() != 4 || p.CallsFor(llm.PhaseTick) != 3 || p.CallsFor(llm.PhaseNarrative) != 1 || p.CallsFor(llm.PhaseDecision) != 0 {
		t.Fatalf("calls = %d, tick = %d, narrative = %d, decision = %d; want 4, 3, 1, 0",
			p.Calls(), p.CallsFor(llm.PhaseTick), p.CallsFor(llm.PhaseNarrative), p.CallsFor(llm.PhaseDecision))
	}
	if _, err := p.Embed(context.Background(), "m", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if p.Calls() != 4 || p.EmbedCalls() != 1 {
		t.Fatalf("after Embed: calls = %d, embed calls = %d; want 4, 1", p.Calls(), p.EmbedCalls())
	}
}

// Requests keeps what the provider saw: a caller changing its slices later
// changes neither the record nor another copy of it.
func TestRequestsAreCopies(t *testing.T) {
	p := fake.New(fake.WithRules(fake.Rule{Replies: []fake.Reply{reply("x")}}))
	req := request(llm.PhaseTick, "original")
	req.Schema = []byte(`{"type":"object"}`)
	generate(t, p, req)
	req.Messages[0].Content = "changed by the caller"
	req.Schema[0] = 'X'

	got := p.Requests()
	if len(got) != 1 || got[0].Messages[0].Content != "original" || string(got[0].Schema) != `{"type":"object"}` {
		t.Fatalf("requests = %+v, want the request as it was sent", got)
	}
	got[0].Messages[0].Content = "changed by the reader"
	if again := p.Requests(); again[0].Messages[0].Content != "original" {
		t.Fatalf("a reader changed the record: %q", again[0].Messages[0].Content)
	}
}

// The provider and the model of a reply default to the fake and to the model
// of the request; set values are kept.
func TestResponseDefaults(t *testing.T) {
	p := fake.New(fake.WithRules(
		fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{{Response: llm.Response{Content: "t"}}}},
		fake.Rule{Phase: llm.PhaseNarrative, Replies: []fake.Reply{{Response: llm.Response{
			Content: "n", Provider: "openai_compat", Model: "recorded-model", LatencyMs: 42,
			ReasoningLen: 7, Tokens: llm.Tokens{Prompt: 10, Completion: 5, Cached: 3},
		}}}},
	))
	tick := generate(t, p, request(llm.PhaseTick, ""))
	if tick.Provider != llm.ProviderFake || tick.Model != "model-e" || tick.LatencyMs != 0 {
		t.Fatalf("tick = %+v, want provider fake, model of the request, latency 0", tick)
	}
	want := llm.Response{Content: "n", Provider: "openai_compat", Model: "recorded-model", LatencyMs: 42,
		ReasoningLen: 7, Tokens: llm.Tokens{Prompt: 10, Completion: 5, Cached: 3}}
	if got := generate(t, p, request(llm.PhaseNarrative, "")); got != want {
		t.Fatalf("narrative = %+v, want %+v", got, want)
	}
}

// testWait bounds every wait of a test on a goroutine of the provider: a
// regression that leaves the call hanging fails the test here instead of
// running into the timeout of go test.
const testWait = 10 * time.Second

func receive[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	timer := clock.RealTimers{}.After(testWait)
	defer timer.Stop()
	var v T
	select {
	case v = <-ch:
	case <-timer.C():
		t.Fatalf("%s: nothing within %s", what, testWait)
	}
	return v
}

// spyTimers records the duration of every timer armed through it.
type spyTimers struct {
	clock.Timers
	mu    sync.Mutex
	armed []time.Duration
}

func (s *spyTimers) After(d time.Duration) clock.Timer {
	timer := s.Timers.After(d)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = append(s.armed, d)
	return timer
}

func (s *spyTimers) durations() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.armed)
}

// A delayed reply waits on the timers of the provider: the timer is armed for
// the delay before the hook runs, nothing comes back until the manual clock
// passes the delay, and the latency is the delay.
func TestADelayWaitsOnTheManualClock(t *testing.T) {
	manual := clock.NewManual(time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC))
	spy := &spyTimers{Timers: manual.Timers()}
	type hooked struct {
		req   llm.Request
		armed []time.Duration
	}
	hook := make(chan hooked, 1)
	p := fake.New(
		fake.WithTimers(spy),
		// What the spy has seen when the hook runs is the order itself: a hook
		// called before the timer is armed sees no timer.
		fake.WithOnDelay(func(r llm.Request) { hook <- hooked{r, spy.durations()} }),
		fake.WithRules(fake.Rule{Replies: []fake.Reply{{Response: llm.Response{Content: "late"}, Delay: 3 * time.Second}}}),
	)
	type result struct {
		resp llm.Response
		err  error
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan result, 1)
	go func() {
		resp, err := p.Generate(ctx, request(llm.PhaseTick, "wait"))
		done <- result{resp, err}
	}()
	h := receive(t, hook, "the hook of the delay")
	if h.req.Messages[0].Content != "wait" {
		t.Fatalf("hook got %+v", h.req)
	}
	if !slices.Equal(h.armed, []time.Duration{3 * time.Second}) {
		t.Fatalf("timers armed when the hook ran = %v, want [3s]: the timer is armed for the delay, then the hook runs", h.armed)
	}
	manual.Advance(2 * time.Second)
	// A short wall-clock look: an answer before the delay passed shows up in
	// it, a correct provider is never failed by it.
	early := clock.RealTimers{}.After(50 * time.Millisecond)
	select {
	case r := <-done:
		t.Fatalf("answered before the delay passed: %+v", r)
	case <-early.C():
	}
	manual.Advance(time.Second)
	r := receive(t, done, "the answer after the delay")
	if r.err != nil || r.resp.Content != "late" || r.resp.LatencyMs != 3000 {
		t.Fatalf("result = %+v, %v; want late, latency 3000", r.resp, r.err)
	}
	if got := spy.durations(); !slices.Equal(got, []time.Duration{3 * time.Second}) {
		t.Fatalf("timers armed = %v, want one of 3s", got)
	}
}

// A context cancelled while the reply waits ends the call with the error of
// the context — the yield of a background call (ADR-014 p. 2).
func TestACancelledContextEndsTheDelay(t *testing.T) {
	manual := clock.NewManual(time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC))
	armed := make(chan struct{}, 1)
	p := fake.New(
		fake.WithTimers(manual.Timers()),
		fake.WithOnDelay(func(llm.Request) { armed <- struct{}{} }),
		fake.WithRules(fake.Rule{Replies: []fake.Reply{{Response: llm.Response{Content: "never"}, Delay: time.Hour}}}),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := p.Generate(ctx, request(llm.PhaseTick, ""))
		done <- err
	}()
	receive(t, armed, "the hook of the delay")
	cancel()
	if err := receive(t, done, "the end of the cancelled call"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if p.Calls() != 1 {
		t.Fatalf("calls = %d, want 1: a yielded call reached the provider", p.Calls())
	}
}

// A done context is honoured without a delay too, and before the timer is
// armed with one; the call still counts.
func TestADoneContextFailsTheCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	hooked := false
	p := fake.New(
		fake.WithOnDelay(func(llm.Request) { hooked = true }),
		fake.WithRules(
			fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{reply("x")}},
			fake.Rule{Phase: llm.PhaseNarrative, Replies: []fake.Reply{{Response: llm.Response{Content: "y"}, Delay: time.Hour}}},
		),
	)
	for _, phase := range []llm.Phase{llm.PhaseTick, llm.PhaseNarrative} {
		if _, err := p.Generate(ctx, request(phase, "")); !errors.Is(err, context.Canceled) {
			t.Fatalf("%s: err = %v, want context.Canceled", phase, err)
		}
	}
	if hooked {
		t.Fatal("the timer of a delay was armed for a context that was already done")
	}
	if p.Calls() != 2 || p.CallsFor(llm.PhaseTick) != 1 || p.CallsFor(llm.PhaseNarrative) != 1 {
		t.Fatalf("calls = %d, tick = %d, narrative = %d; want 2, 1, 1: a call with a done context reached the provider",
			p.Calls(), p.CallsFor(llm.PhaseTick), p.CallsFor(llm.PhaseNarrative))
	}
	if _, err := p.Embed(ctx, "m", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Embed: err = %v, want context.Canceled", err)
	}
	if _, err := p.Models(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Models: err = %v, want context.Canceled", err)
	}
}

// A cancelled call uses up the reply it would have got, whether its context
// was done before the call or ended during the delay: the call queued again
// gets the next reply.
func TestACancelledCallUsesUpItsReply(t *testing.T) {
	manual := clock.NewManual(time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC))
	armed := make(chan struct{}, 1)
	p := fake.New(
		fake.WithTimers(manual.Timers()),
		fake.WithOnDelay(func(llm.Request) { armed <- struct{}{} }),
		fake.WithRules(
			fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{reply("tick 1"), reply("tick 2")}},
			fake.Rule{Phase: llm.PhaseNarrative, Replies: []fake.Reply{
				{Response: llm.Response{Content: "narrative 1"}, Delay: time.Hour},
				reply("narrative 2"),
			}},
		),
	)
	done, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Generate(done, request(llm.PhaseTick, "")); !errors.Is(err, context.Canceled) {
		t.Fatalf("tick with a done context: err = %v, want context.Canceled", err)
	}
	if got := generate(t, p, request(llm.PhaseTick, "")).Content; got != "tick 2" {
		t.Fatalf("tick after a call with a done context = %q, want tick 2", got)
	}

	ctx, cancelDelay := context.WithCancel(context.Background())
	defer cancelDelay()
	ended := make(chan error, 1)
	go func() {
		_, err := p.Generate(ctx, request(llm.PhaseNarrative, ""))
		ended <- err
	}()
	receive(t, armed, "the hook of the delay")
	cancelDelay()
	if err := receive(t, ended, "the end of the cancelled call"); !errors.Is(err, context.Canceled) {
		t.Fatalf("narrative cancelled during the delay: err = %v, want context.Canceled", err)
	}
	if got := generate(t, p, request(llm.PhaseNarrative, "")).Content; got != "narrative 2" {
		t.Fatalf("narrative after a call cancelled during the delay = %q, want narrative 2", got)
	}
}

// The default timers are real: a short delay passes without a manual clock.
func TestTheDefaultTimersAreReal(t *testing.T) {
	p := fake.New(fake.WithRules(fake.Rule{Replies: []fake.Reply{{Response: llm.Response{Content: "x"}, Delay: time.Millisecond}}}))
	if got := generate(t, p, request(llm.PhaseTick, "")); got.Content != "x" || got.LatencyMs != 1 {
		t.Fatalf("response = %+v, want x with latency 1 ms", got)
	}
}

// Two concurrent calls of one rule get two consecutive replies, and every call
// is counted once.
func TestConcurrentCallsAreCountedOnce(t *testing.T) {
	const n = 50
	replies := make([]fake.Reply, n)
	for i := range replies {
		replies[i] = reply(string(rune('A' + i%26)))
	}
	p := fake.New(fake.WithRules(fake.Rule{Replies: replies}))
	var wg sync.WaitGroup
	got := make([]string, n)
	for i := range n {
		wg.Go(func() {
			resp, err := p.Generate(context.Background(), request(llm.PhaseTick, ""))
			if err != nil {
				t.Error(err)
			}
			got[i] = resp.Content
		})
	}
	wg.Wait()
	if p.Calls() != n {
		t.Fatalf("calls = %d, want %d", p.Calls(), n)
	}
	want := make([]string, n)
	for i := range want {
		want[i] = replies[i].Response.Content
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("replies handed out = %v, want each reply once: %v", got, want)
	}
}

func TestModelsComeFromTheConfigurationOrTheTable(t *testing.T) {
	ctx := context.Background()
	table := fake.WithRules(
		fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{
			{Response: llm.Response{Model: "qwen-e"}}, {Response: llm.Response{Model: "gemma"}},
		}},
		fake.Rule{Phase: llm.PhaseNarrative, Replies: []fake.Reply{{Response: llm.Response{Model: "qwen-e"}}, reply("no model")}},
	)
	got, err := fake.New(table).Models(ctx)
	if err != nil || !slices.Equal(got, []string{"gemma", "qwen-e"}) {
		t.Fatalf("models of the table = %v, %v; want [gemma qwen-e]", got, err)
	}
	p := fake.New(table, fake.WithModels("model-e", "model-q"))
	got, err = p.Models(ctx)
	if err != nil || !slices.Equal(got, []string{"model-e", "model-q"}) {
		t.Fatalf("configured models = %v, %v; want [model-e model-q]", got, err)
	}
	got[0] = "changed"
	if again, _ := p.Models(ctx); again[0] != "model-e" {
		t.Fatalf("a caller changed the models of the provider: %v", again)
	}
	if got, _ := fake.New(fake.WithModels()).Models(ctx); got == nil || len(got) != 0 {
		t.Fatalf("an explicitly empty list = %#v, want an empty non-nil list", got)
	}
	if got, _ := fake.New().Models(ctx); len(got) != 0 {
		t.Fatalf("models of an empty table = %v, want none", got)
	}
}

func TestHealthIsSetByTheTest(t *testing.T) {
	ctx := context.Background()
	p := fake.New()
	if got := p.Health(ctx); got != llm.StatusOK {
		t.Fatalf("default health = %v, want ok", got)
	}
	p.SetHealth(llm.Degraded(llm.DegradedModelNotResident))
	if got := p.Health(ctx).String(); got != "degraded(model_not_resident)" {
		t.Fatalf("health = %s", got)
	}
	if got := fake.New(fake.WithHealth(llm.StatusLoading)).Health(ctx); got != llm.StatusLoading {
		t.Fatalf("health = %v, want loading", got)
	}
}

// An embedding depends on the model and the text only: equal in every run,
// different for another text or model, of the configured length, in [-1, 1].
func TestEmbedIsDeterministic(t *testing.T) {
	ctx := context.Background()
	p := fake.New(fake.WithEmbedDim(40))
	a, err := p.Embed(ctx, "embed-model", []string{"wolf", "forest", "wolf"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := fake.New(fake.WithEmbedDim(40)).Embed(ctx, "embed-model", []string{"wolf"})
	other, _ := p.Embed(ctx, "other-model", []string{"wolf"})
	split, _ := p.Embed(ctx, "embed-mode", []string{"lwolf"})
	if len(a) != 3 || len(a[0]) != 40 {
		t.Fatalf("shape = %d×%d, want 3×40", len(a), len(a[0]))
	}
	if !slices.Equal(a[0], a[2]) || !slices.Equal(a[0], b[0]) {
		t.Fatal("equal inputs gave different vectors")
	}
	if slices.Equal(a[0], a[1]) || slices.Equal(a[0], other[0]) || slices.Equal(a[0], split[0]) {
		t.Fatal("different inputs gave equal vectors")
	}
	// Each value reads its own bytes of the digest, and past the first block
	// the values do not repeat it.
	if slices.Equal(a[0][:8], a[0][8:16]) {
		t.Fatal("the second block repeats the first")
	}
	if slices.Equal(a[0][:4], []float32{a[0][0], a[0][0], a[0][0], a[0][0]}) {
		t.Fatalf("the values of one block are equal: %v", a[0][:4])
	}
	for _, v := range a[0] {
		if v < -1 || v > 1 {
			t.Fatalf("value %v outside [-1, 1]", v)
		}
	}
	if got, _ := fake.New().Embed(ctx, "m", []string{"x"}); len(got[0]) != fake.DefaultEmbedDim {
		t.Fatalf("default length = %d, want %d", len(got[0]), fake.DefaultEmbedDim)
	}
}

func TestProgrammingErrorsPanic(t *testing.T) {
	cases := map[string]func(){
		"rule without replies in New": func() { fake.New(fake.WithRules(fake.Rule{Phase: llm.PhaseTick})) },
		"rule without replies in Add": func() { fake.New().Add(fake.Rule{}) },
		"zero embedding length":       func() { fake.New(fake.WithEmbedDim(0)) },
	}
	for name, f := range cases {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			f()
		}()
	}
}

// The replies of a rule are the provider's own: the test changing its slice
// after New changes nothing.
func TestTheTableIsCopied(t *testing.T) {
	replies := []fake.Reply{reply("kept")}
	p := fake.New(fake.WithRules(fake.Rule{Replies: replies}))
	replies[0] = reply("changed")
	if got := generate(t, p, request(llm.PhaseTick, "")).Content; got != "kept" {
		t.Fatalf("content = %q, want kept", got)
	}
}

// The factory plugs the fake into a registry under its name; the fake needs no
// address, so the cloud gate lets it through without one.
func TestFactoryBuildsAFakeThroughTheRegistry(t *testing.T) {
	r := providers.NewRegistry()
	r.Register(llm.ProviderFake, fake.Factory(
		fake.WithModels("model-e"),
		fake.WithRules(fake.Rule{Replies: []fake.Reply{reply("from the registry")}}),
	))
	a, err := r.New(llm.ProviderFake, llm.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b, _ := r.New(llm.ProviderFake, llm.Config{})
	resp, err := a.Generate(context.Background(), request(llm.PhaseTick, ""))
	if err != nil || resp.Content != "from the registry" {
		t.Fatalf("Generate = %+v, %v", resp, err)
	}
	if a.(*fake.Provider).Calls() != 1 || b.(*fake.Provider).Calls() != 0 {
		t.Fatal("two providers of one factory share their counter")
	}
}
