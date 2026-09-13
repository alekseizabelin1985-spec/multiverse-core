package recorded_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"multiverse-core.io/internal/llm"
	"multiverse-core.io/internal/llm/providers"
	"multiverse-core.io/internal/llm/providers/recorded"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

const (
	agentWolf = "encounter-wolf:solo:p1"
	agentGM   = "personal-gm:p1"
)

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// cause is the root of the chain every record of a test derives from.
func cause() eventbus.Event {
	return eventbus.NewRoot("player.attacked", "gateway", "dark-forest-world", nil, eventbus.ActorCI,
		map[string]any{"target": "wolf-alpha"})
}

// record builds an llm.output the way the gateway records one (schemas/events/
// llm.output.v1.json): the payload differs by status as C-07 v1.3 requires.
func record(root eventbus.Event, agent string, phase llm.Phase, attempt int, status llm.ValidationStatus, raw string) eventbus.Event {
	p := map[string]any{
		"phase":             string(phase),
		"attempt":           attempt,
		"provider":          "openai_compat",
		"model":             "qwen3.8-27b",
		"params":            map[string]any{"temperature": 0.7, "max_tokens": 160, "thinking": false},
		"prompt_hash":       hash("prompt of " + agent + string(phase)),
		"response_hash":     hash(raw),
		"response_len":      len(raw),
		"validation_status": string(status),
		"laws_version":      "v1",
		"latency_ms":        1234,
		"tokens":            map[string]any{"prompt": 420, "completion": 60, "cached": 400},
		"cost_usd":          0,
	}
	switch status {
	case llm.ValidationValid, llm.ValidationPartiallyRejected:
		p["response_raw"] = raw
		p["filter"] = map[string]any{"applied": true, "status": "pass", "filter_version": "a-1"}
		p["parse"] = map[string]any{"strategy": "strip_think", "recovered": true, "reasoning_len": 17}
	case llm.ValidationInvalid:
		p["response_raw"] = raw
	case llm.ValidationQuarantined:
		p["filter"] = map[string]any{"applied": true, "status": "block", "filter_version": "a-1"}
	case llm.ValidationFilterError:
		p["filter"] = map[string]any{"applied": true, "status": "error", "filter_version": "a-1"}
	case llm.ValidationError:
		p["response_hash"] = hash("")
		p["response_len"] = 0
		p["error"] = map[string]any{"code": "timeout", "message": "phase timeout 30s"}
	}
	return eventbus.Derive(root, recorded.TypeLLMOutput, contracts.SourceLLM, p,
		eventbus.WithAgent(eventbus.AgentRef{ID: agent, Level: "task", Blueprint: "blueprints/" + agent + ".md"}))
}

func withCall(agent string, attempt int) context.Context {
	return recorded.WithCall(context.Background(), agent, attempt)
}

func req(root eventbus.Event, phase llm.Phase) llm.Request {
	return llm.Request{Phase: phase, Model: "qwen3.8-27b", CorrelationID: root.CorrelationID()}
}

func load(t *testing.T, events ...eventbus.Event) *recorded.Provider {
	t.Helper()
	p, err := recorded.New(context.Background(), recorded.Slice(events...))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

// The records of these tests are llm.output of the schema, so what the
// provider reads is what the gateway will write.
func TestTheRecordsOfTheTestsPassTheSchema(t *testing.T) {
	root := cause()
	for _, status := range []llm.ValidationStatus{llm.ValidationValid, llm.ValidationPartiallyRejected,
		llm.ValidationInvalid, llm.ValidationQuarantined, llm.ValidationFilterError, llm.ValidationError} {
		if err := contracts.Validate(record(root, agentGM, llm.PhaseNarrative, 1, status, `{"text":"x"}`)); err != nil {
			t.Errorf("record with status %s: %v", status, err)
		}
	}
}

// The answer of a call is the record of the same key, as recorded.
func TestGenerateReturnsTheRecordOfTheKey(t *testing.T) {
	root := cause()
	raw := "<think>\nwolf\n</think>\n{\"text\":\"Волк рычит.\",\"mentions\":[\"e1\"],\"background_refs\":[]}"
	p := load(t,
		record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationInvalid, "not json"),
		record(root, agentGM, llm.PhaseNarrative, 2, llm.ValidationValid, raw),
		record(root, agentWolf, llm.PhaseTick, 1, llm.ValidationPartiallyRejected, `{"events":[]}`),
	)
	got, err := p.Generate(withCall(agentGM, 2), req(root, llm.PhaseNarrative))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want := llm.Response{Content: raw, ReasoningLen: 17, Tokens: llm.Tokens{Prompt: 420, Completion: 60, Cached: 400},
		LatencyMs: 1234, Provider: "openai_compat", Model: "qwen3.8-27b"}
	if got != want {
		t.Fatalf("response:\n got %+v\nwant %+v", got, want)
	}
	if got, err := p.Generate(withCall(agentGM, 1), req(root, llm.PhaseNarrative)); err != nil || got.Content != "not json" {
		t.Fatalf("attempt 1 = %q, %v; want the invalid answer, which a replay retries like the run did", got.Content, err)
	}
	if got, err := p.Generate(withCall(agentWolf, 1), req(root, llm.PhaseTick)); err != nil || got.Content != `{"events":[]}` {
		t.Fatalf("tick = %q, %v", got.Content, err)
	}
	if p.Len() != 3 {
		t.Fatalf("Len = %d, want 3", p.Len())
	}
}

// Every part of the key counts: a call that differs in one of them has no
// record, and that is ErrIncompleteRecord with an empty response.
func TestAMissOfAnyKeyPartIsAnIncompleteRecord(t *testing.T) {
	root, other := cause(), cause()
	p := load(t, record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"x"}`))
	cases := map[string]struct {
		ctx context.Context
		req llm.Request
	}{
		"correlation": {withCall(agentGM, 1), req(other, llm.PhaseNarrative)},
		"agent":       {withCall(agentWolf, 1), req(root, llm.PhaseNarrative)},
		"phase":       {withCall(agentGM, 1), req(root, llm.PhaseTick)},
		"attempt":     {withCall(agentGM, 2), req(root, llm.PhaseNarrative)},
	}
	for name, c := range cases {
		resp, err := p.Generate(c.ctx, c.req)
		if !errors.Is(err, recorded.ErrIncompleteRecord) {
			t.Errorf("%s: err = %v, want ErrIncompleteRecord", name, err)
		}
		if errors.Is(err, recorded.ErrNoCallKey) {
			t.Errorf("%s: a miss reported as a missing key: %v", name, err)
		}
		if resp != (llm.Response{}) {
			t.Errorf("%s: response of a miss = %+v, want zero: no template", name, resp)
		}
	}
	_, err := p.Generate(withCall(agentWolf, 3), req(root, llm.PhaseTick))
	for _, part := range []string{root.CorrelationID(), agentWolf, "tick", "attempt=3"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not name %q of the key", err, part)
		}
	}
}

// A call that does not carry its key is a defect of the caller, told apart
// from a gap in the recording.
func TestACallWithoutItsKeyIsNotAMiss(t *testing.T) {
	root := cause()
	p := load(t, record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"x"}`))
	noCorrelation := req(root, llm.PhaseNarrative)
	noCorrelation.CorrelationID = ""
	cases := map[string]struct {
		ctx context.Context
		req llm.Request
	}{
		"no WithCall":      {context.Background(), req(root, llm.PhaseNarrative)},
		"empty agent":      {withCall("", 1), req(root, llm.PhaseNarrative)},
		"attempt zero":     {withCall(agentGM, 0), req(root, llm.PhaseNarrative)},
		"no correlation":   {withCall(agentGM, 1), noCorrelation},
		"unknown phase":    {withCall(agentGM, 1), req(root, "dream")},
		"negative attempt": {withCall(agentGM, -1), req(root, llm.PhaseNarrative)},
	}
	for name, c := range cases {
		_, err := p.Generate(c.ctx, c.req)
		if !errors.Is(err, recorded.ErrNoCallKey) || errors.Is(err, recorded.ErrIncompleteRecord) {
			t.Errorf("%s: err = %v, want ErrNoCallKey only", name, err)
		}
	}
	k, err := recorded.KeyOf(withCall(agentGM, 2), req(root, llm.PhaseTick))
	want := recorded.Key{CorrelationID: root.CorrelationID(), AgentID: agentGM, Phase: llm.PhaseTick, Attempt: 2}
	if err != nil || k != want {
		t.Fatalf("KeyOf = %+v, %v; want %+v", k, err, want)
	}
}

// A recorded failure fails again, with its code; a withheld answer has no text
// to give back, and Lookup tells the gateway what the record says.
func TestRecordsWithoutAnAnswer(t *testing.T) {
	root := cause()
	p := load(t,
		record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationError, ""),
		record(root, agentGM, llm.PhaseNarrative, 2, llm.ValidationQuarantined, "blocked text"),
		record(root, agentGM, llm.PhaseNarrative, 3, llm.ValidationFilterError, "unjudged text"),
	)
	_, err := p.Generate(withCall(agentGM, 1), req(root, llm.PhaseNarrative))
	var failure *recorded.FailureError
	if !errors.Is(err, recorded.ErrRecordedFailure) || !errors.As(err, &failure) || failure.Code != "timeout" || failure.Message != "phase timeout 30s" {
		t.Fatalf("err = %v, want a FailureError with code timeout", err)
	}
	if errors.Is(err, recorded.ErrIncompleteRecord) || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("err = %v: a recorded failure is not a miss and names its code", err)
	}
	for attempt, status := range map[int]llm.ValidationStatus{2: llm.ValidationQuarantined, 3: llm.ValidationFilterError} {
		resp, err := p.Generate(withCall(agentGM, attempt), req(root, llm.PhaseNarrative))
		if !errors.Is(err, recorded.ErrResponseWithheld) || errors.Is(err, recorded.ErrIncompleteRecord) {
			t.Errorf("attempt %d: err = %v, want ErrResponseWithheld", attempt, err)
		}
		if resp != (llm.Response{}) {
			t.Errorf("attempt %d: response = %+v, want zero", attempt, resp)
		}
		rec, ok := p.Lookup(recorded.Key{CorrelationID: root.CorrelationID(), AgentID: agentGM, Phase: llm.PhaseNarrative, Attempt: attempt})
		if !ok || rec.Status != status || rec.HasRaw || rec.Response.Content != "" {
			t.Errorf("attempt %d: Lookup = %+v, %v; want status %s without text", attempt, rec, ok, status)
		}
	}
}

// Lookup exposes what the gateway compares in replay: prompt_hash, the laws
// version and the event the answer came from (ADR-029 p. 8).
func TestLookupReturnsTheRecord(t *testing.T) {
	root := cause()
	ev := record(root, agentWolf, llm.PhaseTick, 1, llm.ValidationValid, `{"events":[]}`)
	p := load(t, ev)
	key := recorded.Key{CorrelationID: root.CorrelationID(), AgentID: agentWolf, Phase: llm.PhaseTick, Attempt: 1}
	rec, ok := p.Lookup(key)
	if !ok {
		t.Fatal("Lookup: no record")
	}
	if rec.Key != key || rec.EventID != ev.ID || rec.Status != llm.ValidationValid || !rec.HasRaw ||
		rec.PromptHash != hash("prompt of "+agentWolf+"tick") || rec.LawsVersion != "v1" || rec.ErrorCode != "" {
		t.Fatalf("record = %+v", rec)
	}
	if _, ok := p.Lookup(recorded.Key{CorrelationID: root.CorrelationID(), AgentID: agentWolf, Phase: llm.PhaseTick, Attempt: 2}); ok {
		t.Fatal("Lookup found a record of another attempt")
	}
}

// The first record of a key is kept: a later one is a redelivery.
func TestTheFirstRecordOfAKeyIsKept(t *testing.T) {
	root := cause()
	p := load(t,
		record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"first"}`),
		record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"second"}`),
	)
	got, err := p.Generate(withCall(agentGM, 1), req(root, llm.PhaseNarrative))
	if err != nil || got.Content != `{"text":"first"}` || p.Len() != 1 {
		t.Fatalf("Generate = %q, %v, Len = %d; want the first record", got.Content, err, p.Len())
	}
}

// A malformed redelivery of a good record fails the source all the same: the
// record is checked before its key is looked up (internal/replay would keep
// the first copy; the provider does not trust such a recording).
func TestAMalformedDuplicateFailsTheSource(t *testing.T) {
	root := cause()
	good := record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationQuarantined, "")
	broken := record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationQuarantined, "")
	broken.Payload["response_raw"] = "blocked text"
	_, err := recorded.New(context.Background(), recorded.Slice(good, broken))
	if !errors.Is(err, recorded.ErrMalformedRecord) || !strings.Contains(err.Error(), broken.ID) {
		t.Fatalf("err = %v, want ErrMalformedRecord naming the duplicate %s", err, broken.ID)
	}
}

// invalid leaves filter to the stage of the pipeline (C-07 v1.3): a record
// with it and one without are both records.
func TestAnInvalidRecordMayCarryAFilter(t *testing.T) {
	root := cause()
	filtered := record(root, agentGM, llm.PhaseNarrative, 2, llm.ValidationInvalid, "judged text")
	filtered.Payload["filter"] = map[string]any{"applied": true, "status": "block", "filter_version": "a-1"}
	if err := contracts.Validate(filtered); err != nil {
		t.Fatalf("the schema refuses an invalid record with filter: %v", err)
	}
	p := load(t, record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationInvalid, "not json"), filtered)
	if got, err := p.Generate(withCall(agentGM, 2), req(root, llm.PhaseNarrative)); err != nil || got.Content != "judged text" {
		t.Fatalf("Generate = %q, %v; want the text of the invalid record", got.Content, err)
	}
}

// Events of other types in the source are not records: a journal of
// llm_records carries llm.output.rejected as well.
func TestOtherTypesAreSkipped(t *testing.T) {
	root := cause()
	rejected := eventbus.Derive(root, "llm.output.rejected", contracts.SourceLLM, map[string]any{"reason": "language"})
	p := load(t, root, rejected, record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"x"}`))
	if p.Len() != 1 {
		t.Fatalf("Len = %d, want 1", p.Len())
	}
}

// A record that cannot be keyed or read fails the whole source, with the id
// of the event: a broken line must not turn into a miss later. So does a
// record that breaks the table "field by validation_status" of C-07 v1.3 —
// above all a withheld text that is there after all, which would otherwise be
// handed on as an answer. Every row of the table is checked against the
// schema too: every record of the table the provider refuses, the schema
// refuses as well. The converse does not hold — the provider reads a JSON null
// in a field of the table as an absent field, while the schema refuses null by
// type; no text leaks, since the status alone withholds the answer.
func TestAMalformedRecordFailsTheSource(t *testing.T) {
	root := cause()
	const text = `{"text":"x"}`
	type malformed struct {
		status llm.ValidationStatus
		mutate func(*eventbus.Event)
		// ofTheTable marks a row of C-07 the schema refuses as well.
		ofTheTable bool
	}
	cases := map[string]malformed{
		"no agent":                 {llm.ValidationValid, func(ev *eventbus.Event) { ev.Meta.Agent = nil }, false},
		"empty agent id":           {llm.ValidationValid, func(ev *eventbus.Event) { ev.Meta.Agent.ID = "" }, false},
		"no attempt":               {llm.ValidationValid, func(ev *eventbus.Event) { delete(ev.Payload, "attempt") }, false},
		"attempt zero":             {llm.ValidationValid, func(ev *eventbus.Event) { ev.Payload["attempt"] = 0 }, false},
		"fractional attempt":       {llm.ValidationValid, func(ev *eventbus.Event) { ev.Payload["attempt"] = 1.5 }, false},
		"unknown phase":            {llm.ValidationValid, func(ev *eventbus.Event) { ev.Payload["phase"] = "dream" }, false},
		"phase of another type":    {llm.ValidationValid, func(ev *eventbus.Event) { ev.Payload["phase"] = 7 }, false},
		"unknown status":           {llm.ValidationValid, func(ev *eventbus.Event) { ev.Payload["validation_status"] = "rejected_language" }, false},
		"no model":                 {llm.ValidationValid, func(ev *eventbus.Event) { delete(ev.Payload, "model") }, false},
		"payload that is not JSON": {llm.ValidationValid, func(ev *eventbus.Event) { ev.Payload["x"] = make(chan int) }, false},

		"valid without response_raw":              {llm.ValidationValid, dropField("response_raw"), true},
		"partially_rejected without response_raw": {llm.ValidationPartiallyRejected, dropField("response_raw"), true},
		"invalid without response_raw":            {llm.ValidationInvalid, dropField("response_raw"), true},
		"quarantined with response_raw":           {llm.ValidationQuarantined, setField("response_raw", "blocked text"), true},
		"filter_error with response_raw":          {llm.ValidationFilterError, setField("response_raw", "unjudged text"), true},
		"error with response_raw":                 {llm.ValidationError, setField("response_raw", ""), true},
		"valid without filter":                    {llm.ValidationValid, dropField("filter"), true},
		"valid with a blocking filter":            {llm.ValidationValid, setFilter("block"), true},
		"partially_rejected with a failed filter": {llm.ValidationPartiallyRejected, setFilter("error"), true},
		"quarantined without filter":              {llm.ValidationQuarantined, dropField("filter"), true},
		"quarantined with a passing filter":       {llm.ValidationQuarantined, setFilter("pass"), true},
		"filter_error with a blocking filter":     {llm.ValidationFilterError, setFilter("block"), true},
		"error with filter":                       {llm.ValidationError, setFilter("pass"), true},
		"error without error{}":                   {llm.ValidationError, dropField("error"), true},
		"error without error.code":                {llm.ValidationError, setField("error", map[string]any{"message": "no code"}), true},
		"valid with error{}":                      {llm.ValidationValid, setField("error", map[string]any{"code": "timeout"}), true},
		"quarantined with error{}":                {llm.ValidationQuarantined, setField("error", map[string]any{"code": "timeout"}), true},
	}
	for name, c := range cases {
		ev := record(root, agentGM, llm.PhaseNarrative, 1, c.status, text)
		ev.Payload = maps.Clone(ev.Payload)
		agent := *ev.Meta.Agent
		ev.Meta.Agent = &agent
		c.mutate(&ev)
		if c.ofTheTable {
			if err := contracts.Validate(ev); err == nil {
				t.Errorf("%s: the schema accepts the record, so it is not a row of the table", name)
			}
		}
		p, err := recorded.New(context.Background(), recorded.Slice(ev))
		if !errors.Is(err, recorded.ErrMalformedRecord) || p != nil {
			t.Errorf("%s: New = %v, %v; want ErrMalformedRecord and no provider", name, p, err)
			continue
		}
		if !strings.Contains(err.Error(), ev.ID) {
			t.Errorf("%s: error %q does not name event %s", name, err, ev.ID)
		}
	}
}

func dropField(name string) func(*eventbus.Event) {
	return func(ev *eventbus.Event) { delete(ev.Payload, name) }
}

func setField(name string, v any) func(*eventbus.Event) {
	return func(ev *eventbus.Event) { ev.Payload[name] = v }
}

func setFilter(status string) func(*eventbus.Event) {
	return setField("filter", map[string]any{"applied": true, "status": status, "filter_version": "a-1"})
}

func TestNewRefusesNoSourceAndPassesItsError(t *testing.T) {
	if _, err := recorded.New(context.Background(), nil); err == nil {
		t.Fatal("New(nil) = nil error")
	}
	broken := errors.New("disk gone")
	_, err := recorded.New(context.Background(), func(context.Context, eventbus.Handler) error { return broken })
	if !errors.Is(err, broken) {
		t.Fatalf("err = %v, want the error of the source", err)
	}
}

func TestModelsHealthAndEmbed(t *testing.T) {
	root := cause()
	b := record(root, agentWolf, llm.PhaseTick, 1, llm.ValidationValid, `{"events":[]}`)
	b.Payload["model"] = "gemma-4"
	p := load(t,
		record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"x"}`),
		b,
		record(root, agentGM, llm.PhaseNarrative, 2, llm.ValidationError, ""),
	)
	ctx := context.Background()
	models, err := p.Models(ctx)
	if err != nil || !slices.Equal(models, []string{"gemma-4", "qwen3.8-27b"}) {
		t.Fatalf("Models = %v, %v; want [gemma-4 qwen3.8-27b]", models, err)
	}
	models[0] = "changed"
	if again, _ := p.Models(ctx); again[0] != "gemma-4" {
		t.Fatal("a caller changed the models of the provider")
	}
	if got := p.Health(ctx); got != llm.StatusOK {
		t.Fatalf("Health = %v, want ok", got)
	}
	vectors, err := p.Embed(ctx, "embed", []string{"wolf"})
	if !errors.Is(err, recorded.ErrIncompleteRecord) || vectors != nil {
		t.Fatalf("Embed = %v, %v; want ErrIncompleteRecord and no vector", vectors, err)
	}
	empty := load(t)
	if got, err := empty.Models(ctx); err != nil || len(got) != 0 {
		t.Fatalf("Models of an empty recording = %v, %v", got, err)
	}
}

func TestADoneContextFailsTheCall(t *testing.T) {
	root := cause()
	p := load(t, record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"x"}`))
	ctx, cancel := context.WithCancel(withCall(agentGM, 1))
	cancel()
	if _, err := p.Generate(ctx, req(root, llm.PhaseNarrative)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate: err = %v, want context.Canceled", err)
	}
	if _, err := p.Models(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Models: err = %v, want context.Canceled", err)
	}
}

func TestConcurrentCallsReadTheSameRecords(t *testing.T) {
	root := cause()
	p := load(t, record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"x"}`))
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if got, err := p.Generate(withCall(agentGM, 1), req(root, llm.PhaseNarrative)); err != nil || got.Content != `{"text":"x"}` {
				t.Errorf("Generate = %q, %v", got.Content, err)
			}
		})
	}
	wg.Wait()
}

// The factory builds the provider for a registry from a source; the provider
// needs no address, so the cloud gate lets it through.
func TestFactoryReadsTheSourceThroughTheRegistry(t *testing.T) {
	root := cause()
	r := providers.NewRegistry()
	r.Register(llm.ProviderRecorded, recorded.Factory(context.Background(),
		recorded.Slice(record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"x"}`))))
	p, err := r.New(llm.ProviderRecorded, llm.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, err := p.Generate(withCall(agentGM, 1), req(root, llm.PhaseNarrative)); err != nil || got.Content != `{"text":"x"}` {
		t.Fatalf("Generate = %q, %v", got.Content, err)
	}
	broken := providers.NewRegistry()
	broken.Register(llm.ProviderRecorded, recorded.Factory(context.Background(),
		func(context.Context, eventbus.Handler) error { return errors.New("no recording") }))
	if _, err := broken.New(llm.ProviderRecorded, llm.Config{}); err == nil || !strings.Contains(err.Error(), "no recording") {
		t.Fatalf("New with a broken source: err = %v", err)
	}
}

// The factory reads its source once: a source over a one-shot sequence gives
// the same provider on every call, and a failed reading the same error.
func TestFactoryReadsTheSourceOnce(t *testing.T) {
	root := cause()
	ev := record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"x"}`)
	pulls := 0
	oneShot := func(yield func(eventbus.Event) bool) {
		pulls++
		if pulls == 1 {
			yield(ev)
		}
	}
	factory := recorded.Factory(context.Background(), recorded.Events(oneShot))
	first, err := factory(llm.Config{})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := factory(llm.Config{})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second || pulls != 1 {
		t.Fatalf("providers equal = %v, pulls = %d; want one provider read once", first == second, pulls)
	}
	if got, err := second.Generate(withCall(agentGM, 1), req(root, llm.PhaseNarrative)); err != nil || got.Content != `{"text":"x"}` {
		t.Fatalf("Generate on the second call of the factory = %q, %v", got.Content, err)
	}

	reads := 0
	failing := recorded.Factory(context.Background(), func(context.Context, eventbus.Handler) error {
		reads++
		return errors.New("no recording")
	})
	for i := range 2 {
		if p, err := failing(llm.Config{}); err == nil || p != nil {
			t.Fatalf("call %d of a failing factory = %v, %v; want an error and no provider", i+1, p, err)
		}
	}
	if reads != 1 {
		t.Fatalf("a failing source was read %d times, want 1", reads)
	}
}
