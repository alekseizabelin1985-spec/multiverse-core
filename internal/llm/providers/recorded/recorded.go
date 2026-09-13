// Package recorded is the LLM provider of replay (C-07, C-15): it answers a
// call with the llm.output record of the same call and never with anything
// else. A call without a record is ErrIncompleteRecord — not a template, not a
// live call — so a replay that has drifted from its recording fails where it
// drifted (ADR-010 p. 2).
//
// Where the records come from is not this package's decision: New takes a
// Source, a function that hands llm.output events over. The package reads no
// file and no journal itself — the format and the reading of a recording live
// outside the providers (T-457) — and internal/replay is not imported
// (depguard internal-llm, internal-replay).
package recorded

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	"multiverse-core.io/internal/llm"
	"multiverse-core.io/internal/llm/providers"
	"multiverse-core.io/shared/eventbus"
)

// TypeLLMOutput is the type of the events the provider answers from.
const TypeLLMOutput = "llm.output"

var (
	// ErrIncompleteRecord is a call the recording holds no llm.output for. It
	// is the one outcome of a miss: in mode=replay the run fails on it, in a
	// recovery the caller answers with a marked template (КД §9.1).
	ErrIncompleteRecord = errors.New("llm/providers/recorded: no record of the call")
	// ErrNoCallKey is a call made without the agent and the attempt in its
	// context (WithCall). It is a defect of the caller, not a gap of the
	// recording, and is not ErrIncompleteRecord.
	ErrNoCallKey = errors.New("llm/providers/recorded: the call carries no record key")
	// ErrMalformedRecord is an llm.output of the source that cannot be keyed or
	// read, or whose fields break the table "field by validation_status" of
	// C-07 v1.3. New refuses the whole source on it: a recording with a broken
	// line would otherwise surface as a miss far from its cause, and a withheld
	// text that is there after all would be handed on as an answer.
	ErrMalformedRecord = errors.New("llm/providers/recorded: malformed llm.output record")
	// ErrRecordedFailure is a call whose record has validation_status=error:
	// the provider failed in the recorded run, and it fails again here.
	ErrRecordedFailure = errors.New("llm/providers/recorded: the recorded call failed")
	// ErrResponseWithheld is a call whose record withholds the text because the
	// filter blocked or failed on it (quarantined, filter_error). The status
	// decides, not the presence of response_raw: there is no text to give back,
	// and the caller reads the outcome from Lookup.
	ErrResponseWithheld = errors.New("llm/providers/recorded: the recorded answer was withheld")
)

// Key is the replay key of an llm.output: (meta.correlation_id, meta.agent.id,
// phase, attempt) (C-07, ADR-010 p. 2).
type Key struct {
	CorrelationID string
	AgentID       string
	Phase         llm.Phase
	Attempt       int
}

func (k Key) String() string {
	return fmt.Sprintf("correlation_id=%q agent=%q phase=%q attempt=%d", k.CorrelationID, k.AgentID, k.Phase, k.Attempt)
}

// Record is what the provider keeps of one llm.output.
type Record struct {
	Key     Key
	EventID string
	Status  llm.ValidationStatus
	// Response is the answer as the recorded provider gave it; Content is
	// response_raw. HasRaw is false when the record withholds the text.
	Response    llm.Response
	HasRaw      bool
	PromptHash  string
	LawsVersion string
	// ErrorCode and ErrorMessage are error{} of a record with status error.
	ErrorCode    string
	ErrorMessage string
}

// FailureError is the error of a call whose record has status error. It
// matches ErrRecordedFailure.
type FailureError struct {
	Key     Key
	Code    string
	Message string
}

func (e *FailureError) Error() string {
	return fmt.Sprintf("llm/providers/recorded: the recorded call failed with %s: %s", e.Code, e.Key)
}

// Is makes the error match ErrRecordedFailure.
func (e *FailureError) Is(target error) bool { return target == ErrRecordedFailure }

type callKey struct{}

type callRef struct {
	agentID string
	attempt int
}

// WithCall attaches the two parts of the key llm.Request does not carry: the
// agent and the attempt. The gateway calls it before every Generate; for any
// other provider the value is inert. The chain and the phase come from the
// request itself.
func WithCall(ctx context.Context, agentID string, attempt int) context.Context {
	return context.WithValue(ctx, callKey{}, callRef{agentID: agentID, attempt: attempt})
}

// KeyOf builds the key of req made under ctx.
func KeyOf(ctx context.Context, req llm.Request) (Key, error) {
	ref, _ := ctx.Value(callKey{}).(callRef)
	k := Key{CorrelationID: req.CorrelationID, AgentID: ref.agentID, Phase: req.Phase, Attempt: ref.attempt}
	switch {
	case k.CorrelationID == "":
		return k, fmt.Errorf("%w: the request has no correlation_id", ErrNoCallKey)
	case k.AgentID == "":
		return k, fmt.Errorf("%w: no agent in the context (recorded.WithCall)", ErrNoCallKey)
	case k.Attempt < 1:
		return k, fmt.Errorf("%w: attempt %d in the context is not positive (recorded.WithCall)", ErrNoCallKey, k.Attempt)
	case !k.Phase.Valid():
		return k, fmt.Errorf("%w: phase %q is not a phase of llm.output", ErrNoCallKey, k.Phase)
	}
	return k, nil
}

// Provider answers calls from a recording. It is immutable after New and safe
// for concurrent use.
type Provider struct {
	records map[Key]Record
	models  []string
}

var _ llm.Provider = (*Provider)(nil)

// New reads every llm.output of src and indexes it by its key. Other event
// types are skipped: a journal of llm_records carries llm.output.rejected too.
// When two records share a key the first one is kept: a record is written
// before it is used (C-07), so a later one is a redelivery of the
// at-least-once bus, the rule internal/replay follows as well.
//
// Every llm.output is checked before its key is looked up, so a malformed
// redelivery of a good record fails the whole source with ErrMalformedRecord
// even though the first copy would be kept. internal/replay indexes such a
// source; the provider does not, because a recording that carries a broken
// copy of a record is not one to trust for the others either.
func New(ctx context.Context, src Source) (*Provider, error) {
	if src == nil {
		return nil, errors.New("llm/providers/recorded: no source of records")
	}
	p := &Provider{records: make(map[Key]Record)}
	err := src(ctx, func(_ context.Context, ev eventbus.Event) error {
		if ev.Type != TypeLLMOutput {
			return nil
		}
		rec, err := parseRecord(ev)
		if err != nil {
			return err
		}
		if _, dup := p.records[rec.Key]; dup {
			return nil
		}
		p.records[rec.Key] = rec
		if m := rec.Response.Model; !slices.Contains(p.models, m) {
			p.models = append(p.models, m)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("llm/providers/recorded: read records: %w", err)
	}
	slices.Sort(p.models)
	return p, nil
}

// Factory returns a factory for providers.Registry that reads src under ctx
// once, at the first call of the factory. Every later call returns the same
// provider, or the same error: the provider is immutable, and a source over a
// one-shot sequence read a second time would give a provider without records,
// every call of which is a miss far from its cause. The configuration of the
// block is not used: a recording names its own providers and models.
func Factory(ctx context.Context, src Source) providers.Factory {
	load := sync.OnceValues(func() (*Provider, error) { return New(ctx, src) })
	return func(llm.Config) (llm.Provider, error) {
		p, err := load()
		if err != nil {
			return nil, err
		}
		return p, nil
	}
}

// Generate returns the recorded answer of the call. A miss is
// ErrIncompleteRecord; a record of a failed call is a *FailureError; a record
// the filter withheld (quarantined, filter_error) is ErrResponseWithheld,
// whatever else the record holds.
func (p *Provider) Generate(ctx context.Context, req llm.Request) (llm.Response, error) {
	if err := ctx.Err(); err != nil {
		return llm.Response{}, err
	}
	key, err := KeyOf(ctx, req)
	if err != nil {
		return llm.Response{}, err
	}
	rec, ok := p.records[key]
	if !ok {
		return llm.Response{}, fmt.Errorf("%w: %s", ErrIncompleteRecord, key)
	}
	// The status and not HasRaw decides: a filter that closed the text in the
	// recorded run closes it here too, even for a record that reached the
	// index past parseRecord.
	switch rec.Status {
	case llm.ValidationError:
		return llm.Response{}, &FailureError{Key: key, Code: rec.ErrorCode, Message: rec.ErrorMessage}
	case llm.ValidationQuarantined, llm.ValidationFilterError:
		return llm.Response{}, fmt.Errorf("%w: validation_status=%s, %s", ErrResponseWithheld, rec.Status, key)
	}
	return rec.Response, nil
}

// Lookup returns the record of key. The gateway reads prompt_hash and the
// status of a withheld answer from it (ADR-029 p. 8).
func (p *Provider) Lookup(key Key) (Record, bool) {
	rec, ok := p.records[key]
	return rec, ok
}

// Len is the number of indexed records.
func (p *Provider) Len() int { return len(p.records) }

// Embed is not recorded: C-07 records generations only. The call fails with
// ErrIncompleteRecord rather than inventing a vector.
func (p *Provider) Embed(context.Context, string, []string) ([][]float32, error) {
	return nil, fmt.Errorf("%w: embeddings are not recorded", ErrIncompleteRecord)
}

// Health is ok: the records are in memory from New on.
func (p *Provider) Health(context.Context) llm.Status { return llm.StatusOK }

// Models lists the distinct models of the records, sorted.
func (p *Provider) Models(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return slices.Clone(p.models), nil
}

// output is the part of the llm.output payload the provider reads
// (schemas/events/llm.output.v1.json). The payload is decoded through JSON
// whatever its origin, so a map built in Go and a map decoded from a file give
// the same record, and a fractional attempt is refused instead of truncated.
type output struct {
	Phase            llm.Phase            `json:"phase"`
	Attempt          *int                 `json:"attempt"`
	Provider         string               `json:"provider"`
	Model            string               `json:"model"`
	PromptHash       string               `json:"prompt_hash"`
	ResponseRaw      *string              `json:"response_raw"`
	ValidationStatus llm.ValidationStatus `json:"validation_status"`
	LawsVersion      string               `json:"laws_version"`
	LatencyMs        int64                `json:"latency_ms"`
	Tokens           llm.Tokens           `json:"tokens"`
	Parse            *struct {
		ReasoningLen int `json:"reasoning_len"`
	} `json:"parse"`
	Filter *struct {
		Status string `json:"status"`
	} `json:"filter"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// statusFields is one row of the table "field by validation_status" of C-07
// v1.3, the allOf of llm.output.v1.json. The journal validates a record on
// delivery; a recording file is not validated on reading, so the provider
// holds the rows itself.
type statusFields struct {
	// withRaw: response_raw is required when true and forbidden when false.
	withRaw bool
	// filter is the filter.status required; "" leaves filter to the stage of
	// the pipeline (invalid), which is not a property of one record.
	filter string
	// noFilter forbids filter: nothing was filtered.
	noFilter bool
	// withError: error{code} is required when true and forbidden when false.
	withError bool
}

var tableOfC07 = map[llm.ValidationStatus]statusFields{
	llm.ValidationValid:             {withRaw: true, filter: "pass"},
	llm.ValidationPartiallyRejected: {withRaw: true, filter: "pass"},
	llm.ValidationInvalid:           {withRaw: true},
	llm.ValidationQuarantined:       {filter: "block"},
	llm.ValidationFilterError:       {filter: "error"},
	llm.ValidationError:             {noFilter: true, withError: true},
}

// check returns what out breaks of the row of its status, or "".
func (row statusFields) check(out output) string {
	s := out.ValidationStatus
	switch {
	case row.withRaw && out.ResponseRaw == nil:
		return fmt.Sprintf("validation_status=%s without response_raw", s)
	case !row.withRaw && out.ResponseRaw != nil:
		return fmt.Sprintf("validation_status=%s with response_raw", s)
	case row.filter != "" && (out.Filter == nil || out.Filter.Status != row.filter):
		return fmt.Sprintf("validation_status=%s without filter.status=%s", s, row.filter)
	case row.noFilter && out.Filter != nil:
		return fmt.Sprintf("validation_status=%s with filter", s)
	case row.withError && (out.Error == nil || out.Error.Code == ""):
		return fmt.Sprintf("validation_status=%s without error.code", s)
	case !row.withError && out.Error != nil:
		return fmt.Sprintf("validation_status=%s with error", s)
	}
	return ""
}

func parseRecord(ev eventbus.Event) (Record, error) {
	bad := func(format string, args ...any) (Record, error) {
		return Record{}, fmt.Errorf("%w: event %q: %s", ErrMalformedRecord, ev.ID, fmt.Sprintf(format, args...))
	}
	raw, err := json.Marshal(ev.Payload)
	if err != nil {
		return bad("payload: %v", err)
	}
	var out output
	if err := json.Unmarshal(raw, &out); err != nil {
		return bad("payload: %v", err)
	}
	if ev.Meta.Agent == nil || ev.Meta.Agent.ID == "" {
		return bad("no meta.agent.id")
	}
	if !out.Phase.Valid() {
		return bad("phase %q", out.Phase)
	}
	if out.Attempt == nil || *out.Attempt < 1 {
		return bad("no positive attempt")
	}
	row, known := tableOfC07[out.ValidationStatus]
	if !known {
		return bad("validation_status %q", out.ValidationStatus)
	}
	if broken := row.check(out); broken != "" {
		return bad("%s", broken)
	}
	if out.Model == "" {
		return bad("no model")
	}
	rec := Record{
		Key: Key{
			CorrelationID: ev.CorrelationID(),
			AgentID:       ev.Meta.Agent.ID,
			Phase:         out.Phase,
			Attempt:       *out.Attempt,
		},
		EventID:     ev.ID,
		Status:      out.ValidationStatus,
		PromptHash:  out.PromptHash,
		LawsVersion: out.LawsVersion,
		HasRaw:      out.ResponseRaw != nil,
		Response: llm.Response{
			Tokens:    out.Tokens,
			LatencyMs: out.LatencyMs,
			Provider:  out.Provider,
			Model:     out.Model,
		},
	}
	if out.ResponseRaw != nil {
		rec.Response.Content = *out.ResponseRaw
	}
	if out.Parse != nil {
		rec.Response.ReasoningLen = out.Parse.ReasoningLen
	}
	if out.Error != nil {
		rec.ErrorCode, rec.ErrorMessage = out.Error.Code, out.Error.Message
	}
	return rec, nil
}
