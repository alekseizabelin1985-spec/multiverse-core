package recorded_test

import (
	"context"
	"errors"
	"testing"

	"multiverse-core.io/internal/llm"
	"multiverse-core.io/internal/llm/providers/fake"
	"multiverse-core.io/internal/llm/providers/recorded"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

// step is one provider call of a scripted session: what the gateway would ask
// for one attempt of one agent.
type step struct {
	agent   string
	phase   llm.Phase
	attempt int
}

// session plays steps against p the way the gateway calls a provider — the
// key parts the request lacks go into the context — and, when recording,
// writes an llm.output per answer before it is used (C-07). The status is not
// judged here: that is the pipeline of T-212/T-213; the record only has to be
// one the schema accepts.
func session(t *testing.T, p llm.Provider, root eventbus.Event, steps []step, record bool) ([]llm.Response, []eventbus.Event) {
	t.Helper()
	var (
		answers []llm.Response
		records []eventbus.Event
	)
	for _, s := range steps {
		ctx := recorded.WithCall(context.Background(), s.agent, s.attempt)
		resp, err := p.Generate(ctx, llm.Request{Phase: s.phase, Model: "qwen3.8-27b", CorrelationID: root.CorrelationID()})
		if err != nil {
			t.Fatalf("%+v: Generate: %v", s, err)
		}
		answers = append(answers, resp)
		if !record {
			continue
		}
		ev := eventbus.Derive(root, recorded.TypeLLMOutput, contracts.SourceLLM, map[string]any{
			"phase":             string(s.phase),
			"attempt":           s.attempt,
			"provider":          resp.Provider,
			"model":             resp.Model,
			"params":            map[string]any{"thinking": false},
			"prompt_hash":       hash("prompt"),
			"response_raw":      resp.Content,
			"response_hash":     hash(resp.Content),
			"response_len":      len(resp.Content),
			"validation_status": "invalid",
			"laws_version":      "v1",
			"latency_ms":        resp.LatencyMs,
			"tokens":            map[string]any{"prompt": resp.Tokens.Prompt, "completion": resp.Tokens.Completion},
			"cost_usd":          0,
		}, eventbus.WithAgent(eventbus.AgentRef{ID: s.agent, Level: "task", Blueprint: "blueprints/x.md"}))
		if err := contracts.Validate(ev); err != nil {
			t.Fatalf("%+v: the record does not pass the schema: %v", s, err)
		}
		records = append(records, ev)
	}
	return answers, records
}

// replay: llm_calls=0 (NFR-014), the provider side. A session recorded on the
// fake is played back through recorded after a trip over the wire: every
// answer equals the recorded one, and a call the recording does not hold is
// ErrIncompleteRecord. That is where the weight of the test lies. The fake's
// counter staying put holds by construction here — recorded has no reference
// to a live provider to fall through to — and is kept as the measure of
// NFR-014 that the gateway test will reuse. Choosing recorded for mode=replay,
// with no fallback to a live provider on a miss, is the gateway's and is
// tested by T-212.
func TestReplayMakesNoLLMCalls(t *testing.T) {
	live := fake.New(fake.WithRules(
		fake.Rule{Phase: llm.PhaseTick, Replies: []fake.Reply{
			{Response: llm.Response{Content: fake.WithThink(`{"events":[]}`), Tokens: llm.Tokens{Prompt: 300, Completion: 40}}},
			{Response: llm.Response{Content: `{"events":[]}`, Tokens: llm.Tokens{Prompt: 310, Completion: 12}}},
		}},
		fake.Rule{Phase: llm.PhaseNarrative, Replies: []fake.Reply{{Response: llm.Response{
			Content: fake.Narrative{Text: "Волк отступает.", Mentions: []string{fake.EntityLabel(1)}}.JSON(),
			Tokens:  llm.Tokens{Prompt: 420, Completion: 60},
		}}}},
	))
	root := cause()
	steps := []step{
		{agentWolf, llm.PhaseTick, 1},
		{agentWolf, llm.PhaseTick, 2},
		{agentGM, llm.PhaseNarrative, 1},
	}
	liveAnswers, records := session(t, live, root, steps, true)
	if live.Calls() != len(steps) {
		t.Fatalf("live run: llm_calls = %d, want %d", live.Calls(), len(steps))
	}
	callsBeforeReplay := live.Calls()

	rec, err := recorded.New(context.Background(), recorded.Slice(overTheWire(t, records...)...))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	replayAnswers, _ := session(t, rec, root, steps, false)
	for i := range steps {
		if replayAnswers[i] != liveAnswers[i] {
			t.Errorf("%+v: replay answered %+v, the run answered %+v", steps[i], replayAnswers[i], liveAnswers[i])
		}
	}
	if got := live.Calls() - callsBeforeReplay; got != 0 {
		t.Fatalf("replay: llm_calls = %d, want 0", got)
	}

	_, err = rec.Generate(recorded.WithCall(context.Background(), agentGM, 2),
		llm.Request{Phase: llm.PhaseNarrative, CorrelationID: root.CorrelationID()})
	if !errors.Is(err, recorded.ErrIncompleteRecord) {
		t.Fatalf("a call the recording does not hold: err = %v, want ErrIncompleteRecord", err)
	}
	if got := live.Calls() - callsBeforeReplay; got != 0 {
		t.Fatalf("replay after a miss: llm_calls = %d, want 0", got)
	}
}
