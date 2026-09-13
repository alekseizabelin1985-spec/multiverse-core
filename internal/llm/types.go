// Package llm is the LLM gateway of the platform: the provider interface of
// C-15, the types a caller hands to the gateway and gets back, and the
// configuration of the block.
//
// The package is a leaf library of swarm and memory (ADR-001 p. 3): it imports
// no other context, and the linter rule internal-llm keeps it that way.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"multiverse-core.io/shared/eventbus"
)

// Provider is one LLM runtime behind the gateway (C-15 v1.1). Generate has no
// side effect on the bus: recording llm.output is the gateway's job, which is
// what lets every provider share the budget, the parser, the filter and the
// guardian.
type Provider interface {
	Generate(ctx context.Context, req Request) (Response, error)
	Embed(ctx context.Context, model string, texts []string) ([][]float32, error)
	Health(ctx context.Context) Status
	// Models names the models the provider serves. The blueprint validator
	// (C-11 rule 7a) and /health.llm compare the model of a blueprint with it.
	Models(ctx context.Context) ([]string, error)
}

// Phase is the kind of work a call does. The values are the enum of
// llm.output.phase and llm.output.rejected.phase.
type Phase string

const (
	PhaseNarrative Phase = "narrative"
	PhaseTick      Phase = "tick"
	// PhaseDecision is reserved: MVP-1 makes no decision calls.
	PhaseDecision Phase = "decision"
	PhaseOther    Phase = "other"
)

// Valid reports whether p is one of the phases the records accept.
func (p Phase) Valid() bool {
	switch p {
	case PhaseNarrative, PhaseTick, PhaseDecision, PhaseOther:
		return true
	}
	return false
}

// Role is the author of a chat message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn of the conversation sent to a provider.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Request is what the gateway sends to a provider.
type Request struct {
	Phase    Phase
	Model    string
	System   string
	Messages []Message
	// Schema is the JSON Schema 2020-12 of the answer (structured output); nil
	// asks for free text.
	Schema        json.RawMessage
	Params        Params
	Timeout       time.Duration
	CorrelationID string
}

// Params is the sampling of one request, sent per request so that the server
// defaults (the thinking profile of llama-server) never decide it.
//
// A zero field means "not set": a provider sends its default of the phase
// (openai_compat) or leaves the field out. That is what keeps a v1 caller, which knows
// Temperature, MaxTokens and Think only, working unchanged against v1.1
// (C-15 "Гарантии"). The JSON names are those of llm.output.params, which
// records the sampling as sent.
type Params struct {
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"top_p,omitempty"`
	TopK            int     `json:"top_k,omitempty"`
	MinP            float64 `json:"min_p,omitempty"`
	PresencePenalty float64 `json:"presence_penalty,omitempty"`
	MaxTokens       int     `json:"max_tokens,omitempty"`
	// Think is false for every phase of MVP-1: with thinking on, llama-server
	// does not apply the json_schema grammar (llama.cpp #20345, ADR-005 add. 2).
	Think bool `json:"thinking"`
}

// Tokens is the usage of one call as the provider reported it (NFR-052).
type Tokens struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	// Cached is usage.prompt_tokens_details.cached_tokens, 0 when the provider
	// does not report it. It is part of Prompt, not added to it.
	Cached int `json:"cached,omitempty"`
}

// Response is the answer of a provider (C-15 v1.1).
type Response struct {
	// Content is the text of the answer, JSON when the request carried a schema.
	Content string
	// ReasoningLen is the length of the reasoning_content the provider threw
	// away. The length is recorded in llm.output.parse; the text never is.
	ReasoningLen int
	Tokens       Tokens
	LatencyMs    int64
	Provider     string
	Model        string
}

// State is the health of a provider without its reason.
type State string

const (
	StateOK State = "ok"
	// StateLoading is a server still loading its model (llama-server answers
	// 503). It is not unavailable: the world must not fall back to templates
	// for the minute a restart takes.
	StateLoading     State = "loading"
	StateDegraded    State = "degraded"
	StateUnavailable State = "unavailable"
)

// DegradedModelNotResident is the degradation of a provider that is up but does
// not serve the model a blueprint asks for.
const DegradedModelNotResident = "model_not_resident"

// Status is the health of a provider: ok | loading |
// degraded(model_not_resident) | unavailable. Reason is set for degraded only.
type Status struct {
	State  State
	Reason string
}

// Health values without a reason.
var (
	StatusOK          = Status{State: StateOK}
	StatusLoading     = Status{State: StateLoading}
	StatusUnavailable = Status{State: StateUnavailable}
)

// Degraded returns the degraded status with its reason.
func Degraded(reason string) Status { return Status{State: StateDegraded, Reason: reason} }

// String renders the status the way /health shows it: "degraded(model_not_resident)".
func (s Status) String() string {
	if s.Reason == "" {
		return string(s.State)
	}
	return string(s.State) + "(" + s.Reason + ")"
}

// ValidationStatus is the outcome of the pipeline for one attempt, the single
// enum of llm.output.validation_status (C-07 v1.2). The reasons of a refusal
// are not statuses: they travel in llm.output.rejected.
type ValidationStatus string

const (
	// ValidationValid: the answer is used as a whole.
	ValidationValid ValidationStatus = "valid"
	// ValidationPartiallyRejected: at least one element was dropped by the
	// guardian and the rest is used.
	ValidationPartiallyRejected ValidationStatus = "partially_rejected"
	// ValidationInvalid: the answer is not used (schema, language, stale laws or
	// every element dropped); the gateway retries or a template answers.
	ValidationInvalid ValidationStatus = "invalid"
	// ValidationError: there is no answer (provider error, timeout, yielded).
	ValidationError ValidationStatus = "error"
	// ValidationQuarantined: the category (a) filter blocked the answer; it is
	// recorded without response_raw.
	ValidationQuarantined ValidationStatus = "quarantined"
	// ValidationFilterError: the filter failed and closed; recorded without
	// response_raw.
	ValidationFilterError ValidationStatus = "filter_error"
)

// Rejection is one refusal the gateway publishes as llm.output.rejected. The
// fields follow the payload; the link to llm.output, the phase and the attempt
// are added by the recorder, which knows them.
type Rejection struct {
	// Reason is a value of llm.output.rejected.reason. The dictionary of the
	// reasons is internal/llm/guardian/reasons.go (T-217), which the test
	// TestRejectionReasonsMatchTheGuardianPackage already holds equal to the
	// schema; a second copy of the constants here would be a dictionary that
	// test does not see.
	Reason string
	// Element is the dropped element; nil means the whole answer.
	Element *Element
	// Entity is the entity the refusal is about, when there is one.
	Entity *eventbus.EntityRef
	// EntityName is the display name of Entity known to the gateway.
	EntityName string
	// Category is the filter category of a filter_blocked refusal ("a").
	Category string
	// Budget is set for budget_exceeded only.
	Budget *BudgetLimit
}

// Element points at one element of an answer: events[Index] of a tick,
// mentions[Index] of a narrative.
type Element struct {
	Index int
	Type  string
}

// BudgetLimit is the window a budget_exceeded refusal hit.
type BudgetLimit struct {
	Kind   string // background | turn | cloud
	Limit  int
	Window string
}

// Call is what a caller of the gateway asks for: one generation with its
// context, retries and the checks the answer must pass.
//
// Two fields of the component design are not here yet, on purpose. Prompt
// (prompt.Sections) comes with T-212, which depends on T-218 for the sections,
// and Guard (guardian.Input) comes with T-213, which depends on T-217. A field
// of a package that does not exist yet would make this task write the core
// types of two other tasks. Schema is the name of the schema of the phase, a
// string, so the caller does not need the parser package (ADR-001 addendum
// 2026-09-13 p. 3).
type Call struct {
	// Agent is meta.agent of the records.
	Agent eventbus.AgentRef
	Phase Phase
	// Cause is the event llm.output is derived from.
	Cause       eventbus.Event
	ActorKind   string
	LawsVersion string
	// Schema names the schema of the answer, e.g. "narrative" or
	// "tick-region" (schemas/agent/<name>.json).
	Schema  string
	Model   string
	Params  Params
	Retries int
	// Timeout of one attempt; zero means the timeout of the phase from Config.
	Timeout time.Duration
	// TextPaths are the JSON paths of the text fields the language check and
	// the filter read: ["text"], ["events[*].summary"].
	TextPaths []string
	LOD       string
	GMPath    string
}

// Result is what the gateway returns for a call whose answer was used.
type Result struct {
	// Value is what is left of the answer after the guardian.
	Value json.RawMessage
	// OutputEventID is the id of the llm.output of the last attempt.
	OutputEventID string
	Status        ValidationStatus
	Rejected      []Rejection
	Attempts      int
	Tokens        Tokens
	LatencyMs     int64
}

// Gateway is the entry point of the swarm into the LLM. Usage() of the
// component design joins with T-211, which defines the Usage type.
type Gateway interface {
	Generate(ctx context.Context, call Call) (Result, error)
	Health(ctx context.Context) Status
}

// The errors a caller of the gateway tells apart. Each one maps to a fallback
// of the swarm (template, rule-only), so they are sentinels, matched with
// errors.Is.
var (
	ErrUnavailable         = errors.New("llm: provider unavailable")
	ErrBudget              = errors.New("llm: budget exceeded")
	ErrQuarantined         = errors.New("llm: answer quarantined by the filter")
	ErrInvalidAfterRetries = errors.New("llm: answer invalid after retries")
	ErrFilter              = errors.New("llm: filter failed")
)
