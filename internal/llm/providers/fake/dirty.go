package fake

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The generators below wrap a clean body in what a local model puts around
// JSON when the grammar is not applied: the tests of the parser (ADR-016 p. 1)
// feed them to it and check the strategy it reports. Each one is a pure
// function of its input and composes with the others:
// WithPreamble(InFence(body)) is a preamble followed by a fenced body.

// Preamble is the sentence WithPreamble puts before the body.
const Preamble = "Here is the answer in JSON format:"

// Think is the reasoning WithThink puts before the body.
const Think = "The player attacks the wolf, so the answer must describe the strike."

// Trailer is the sentence WithTrailingText puts after the body.
const Trailer = "Let me know if you need anything else."

// WithPreamble puts a sentence and a blank line before body — the
// balanced_object strategy of the parser.
func WithPreamble(body string) string { return Preamble + "\n\n" + body }

// WithThink puts a <think> block before body, the way a model with thinking
// switched on answers through a template that leaks the reasoning into the
// content — the strip_think strategy.
func WithThink(body string) string { return "<think>\n" + Think + "\n</think>\n\n" + body }

// InFence wraps body in a ```json code fence — the strip_fence strategy.
func InFence(body string) string { return "```json\n" + body + "\n```" }

// WithTrailingText puts a sentence after body — balanced_object again, from
// the other side.
func WithTrailingText(body string) string { return body + "\n\n" + Trailer }

// WithTrailingComma inserts a comma before the last closing brace or bracket
// of body: {"a":1} becomes {"a":1,} — the trailing_commas strategy. A body
// without a closing brace or bracket is returned unchanged.
func WithTrailingComma(body string) string {
	i := strings.LastIndexAny(body, "}]")
	if i < 0 {
		return body
	}
	return body[:i] + "," + body[i:]
}

// JSON returns v as compact JSON. It is for the bodies of replies: a value
// that cannot be encoded is an error of the test and panics.
func JSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("llm/providers/fake: JSON: %v", err))
	}
	return string(b)
}

// EntityLabel is the call label of the k-th entity of a narrative prompt,
// "e<k>" (ADR-029 p. 2). k is not checked: a label outside e1…e99, or one the
// call table does not hold, is exactly what a test of the guardian needs.
func EntityLabel(k int) string { return "e" + strconv.Itoa(k) }

// BackgroundLabel is the call label of the k-th background event, "b<k>".
func BackgroundLabel(k int) string { return "b" + strconv.Itoa(k) }

// Narrative is an answer of the narrative phase (schemas/agent/narrative.json,
// КД §13.4): the elements of Mentions and BackgroundRefs are call labels, not
// ids (ADR-029 p. 3).
type Narrative struct {
	Text           string   `json:"text"`
	Mentions       []string `json:"mentions"`
	BackgroundRefs []string `json:"background_refs"`
	Tone           string   `json:"tone,omitempty"`
}

// JSON renders the answer. The two arrays are required by the schema, so a
// nil one is written as [] and never as null.
func (n Narrative) JSON() string {
	if n.Mentions == nil {
		n.Mentions = []string{}
	}
	if n.BackgroundRefs == nil {
		n.BackgroundRefs = []string{}
	}
	return JSON(n)
}
