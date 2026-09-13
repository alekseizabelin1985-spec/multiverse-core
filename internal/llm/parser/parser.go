// Package parser turns the text of a model answer into the JSON value the
// gateway uses (ADR-016 p. 1): it recovers the JSON a local model wraps in
// reasoning, code fences or prose, validates it with the compiled schema of the
// phase, and checks the language of the texts a player reads (ADR-016 p. 2).
//
// The package decides whether an answer is usable and why not; it records and
// publishes nothing. The gateway turns ErrSchemaInvalid and ErrLanguage into
// validation_status=invalid and llm.output.rejected with the reason
// schema_invalid or language (C-07 v1.2, ADR-017 addendum 1); the dictionary of
// the reasons is internal/llm/guardian, not this package.
//
// Neither an error nor anything else this package returns carries the text of
// the answer: an answer that failed the checks may be exactly the text that
// must not be stored (C-07, FR-050). Errors name a strategy, a location in the
// value, a schema keyword or a count, never a fragment of the answer: a key of
// the location that the schemas do not declare is written as *.
package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Strategy is the step of recovery at which the answer became JSON. The values
// are the enum of llm.output.parse.strategy (C-07 v1.1).
type Strategy string

// The steps in the order Parse applies them (ADR-016 p. 1).
const (
	// StrategyDirect: the answer, trimmed of surrounding white space, is JSON.
	StrategyDirect Strategy = "direct"
	// StrategyStripThink: a leading <think>…</think> block was removed.
	StrategyStripThink Strategy = "strip_think"
	// StrategyStripFence: the body of a ```json code fence was taken.
	StrategyStripFence Strategy = "strip_fence"
	// StrategyBalancedObject: the first { up to its matching } was taken.
	StrategyBalancedObject Strategy = "balanced_object"
	// StrategyTrailingCommas: commas before a closing } or ] were removed.
	StrategyTrailingCommas Strategy = "trailing_commas"
)

// Strategies lists every strategy in the order Parse tries them.
func Strategies() []Strategy {
	return []Strategy{StrategyDirect, StrategyStripThink, StrategyStripFence,
		StrategyBalancedObject, StrategyTrailingCommas}
}

// Recovered reports whether the JSON had to be recovered, the value of
// llm.output.parse.recovered.
func (s Strategy) Recovered() bool { return s != StrategyDirect }

// ErrSchemaInvalid is an answer the gateway cannot use as JSON of the phase:
// no JSON could be recovered from it (an empty or a truncated answer among
// them), or the JSON does not match the schema. It maps to the reason
// schema_invalid.
var ErrSchemaInvalid = errors.New("llm/parser: schema_invalid")

// ErrLabelInText is a narrative whose text carries a call label (ADR-029 p. 4).
// It matches ErrSchemaInvalid as well: the reason is schema_invalid, and only
// the hint of the retry may want to tell the two apart.
var ErrLabelInText = fmt.Errorf("%w: a call label in the text of a narrative", ErrSchemaInvalid)

// ErrNoSchema is a call of Parse without a schema: a defect of the caller, not
// of the answer, so it does not match ErrSchemaInvalid.
var ErrNoSchema = errors.New("llm/parser: no schema to validate the answer with")

// step is one recovery of the chain: it returns its input unchanged when it
// has nothing to remove.
type step struct {
	strategy Strategy
	apply    func(string) string
}

var recovery = []step{
	{StrategyStripThink, stripThink},
	{StrategyStripFence, stripFence},
	{StrategyBalancedObject, balancedObject},
	{StrategyTrailingCommas, trailingCommas},
}

// Parse recovers the JSON of raw and validates it with schema (ADR-016 p. 1).
//
// The steps run in order and each works on the output of the previous one, so
// an answer with a reasoning block, a preamble and a fence is recovered in one
// call. The first step after which the text is JSON is the strategy; a step
// that changes nothing is not tried. The steps remove what surrounds the JSON
// and commas before a closer — they never add a character, so a truncated
// answer stays truncated and fails, instead of being completed by guess.
//
// Once the text is JSON the chain stops: a JSON value of the wrong shape (an
// array where the schema wants an object) is a schema error, not a reason to
// cut an object out of it. An array that is not JSON — cut, or wrapped in
// prose — is not cut into objects either, and a reasoning block that is never
// closed means there is no answer.
//
// For the narrative schema the text is then checked for a call label (ADR-029
// p. 4); the string is not changed.
//
// The strategy is returned with a schema error too, so that the record of the
// attempt can say how far the recovery got; it is empty when no JSON was found.
func Parse(raw string, schema *Schema) (json.RawMessage, Strategy, error) {
	if schema == nil {
		return nil, "", ErrNoSchema
	}
	text, strategy, ok := recoverJSON(raw)
	if !ok {
		if strings.TrimSpace(raw) == "" {
			return nil, "", fmt.Errorf("%w: the answer is empty", ErrSchemaInvalid)
		}
		return nil, "", fmt.Errorf("%w: no JSON value could be recovered from the answer", ErrSchemaInvalid)
	}
	doc, err := schema.validate(text)
	if err != nil {
		return nil, strategy, fmt.Errorf("%w: strategy %s: %v", ErrSchemaInvalid, strategy, err)
	}
	if schema.Name() == NarrativeSchema && hasLabel(doc) {
		return nil, strategy, ErrLabelInText
	}
	return json.RawMessage(text), strategy, nil
}

// recoverJSON runs the chain and returns the first text that is JSON.
func recoverJSON(raw string) (string, Strategy, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", "", false
	}
	if json.Valid([]byte(text)) {
		return text, StrategyDirect, true
	}
	for _, s := range recovery {
		next := strings.TrimSpace(s.apply(text))
		if next == text {
			continue
		}
		text = next
		if json.Valid([]byte(text)) {
			return text, s.strategy, true
		}
	}
	return "", "", false
}

const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"
)

// stripThink removes the reasoning in front of the answer. Two shapes occur:
// a block that starts the answer, <think>…</think>, and the tail of a block
// whose opening tag was part of the prompt template, so that the answer starts
// with the reasoning and only the closing tag is there. A byte order mark in
// front of the block is white space here.
//
// A <think> that is never closed leaves nothing from it on: the answer was cut
// while the model was still reasoning, and an object in the reasoning is a
// draft, not the answer. That holds wherever the tag stands — at the start of
// the answer, after a byte order mark or after prose — as long as it is not in
// a string of the JSON.
//
// Only reasoning in FRONT of the answer is removed otherwise. A closed block
// further in, or a closing tag inside a string of the JSON, is left: ADR-016
// does not change what strings say.
func stripThink(s string) string {
	out := strings.TrimLeftFunc(s, isLeadingSpace)
	if strings.HasPrefix(out, thinkOpen) {
		for strings.HasPrefix(out, thinkOpen) {
			end := strings.Index(out, thinkClose)
			if end < 0 {
				return ""
			}
			out = strings.TrimLeftFunc(out[end+len(thinkClose):], isLeadingSpace)
		}
		return out
	}
	if open := unclosedThink(s); open >= 0 {
		s = s[:open]
	}
	end := strings.Index(s, thinkClose)
	if end < 0 || strings.Contains(s[:end], thinkOpen) || inJSONString(s, end) {
		return s
	}
	return s[end+len(thinkClose):]
}

// isLeadingSpace is the white space stripThink skips in front of a block:
// Unicode white space and the byte order mark, which unicode.IsSpace is not.
func isLeadingSpace(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' }

// unclosedThink returns the offset of the first <think> outside a string of
// the JSON that no later </think> closes, or -1. The text is scanned once: a
// closed block is skipped up to its closing tag.
func unclosedThink(s string) int {
	var v valueScanner
	closed := 0
	for i := 0; i < len(s); i++ {
		if i >= closed && !v.sc.in && strings.HasPrefix(s[i:], thinkOpen) {
			end := strings.Index(s[i:], thinkClose)
			if end < 0 {
				return i
			}
			closed = i + end + len(thinkClose)
		}
		v.inString(s[i])
	}
	return -1
}

// inJSONString reports whether the byte at pos falls inside a string of a JSON
// value in front of it. Reasoning around the values is prose, and its quotes
// are not strings: a value that closed before pos — a draft in the reasoning —
// does not carry its strings on into the prose after it.
func inJSONString(s string, pos int) bool {
	var v valueScanner
	for i := 0; i < pos; i++ {
		v.inString(s[i])
	}
	return v.sc.in
}

// valueScanner follows the JSON values that stand in prose. A { or [ opens a
// value; inside it strings are followed and brackets counted, and when the
// value closes the scan is back in prose, where a quote is only a character.
type valueScanner struct {
	depth int
	sc    stringScanner
}

// inString consumes c and reports whether it belongs to a string of a value.
func (v *valueScanner) inString(c byte) bool {
	if v.depth == 0 {
		if c == '{' || c == '[' {
			v.depth = 1
		}
		return false
	}
	if v.sc.inString(c) {
		return true
	}
	switch c {
	case '{', '[':
		v.depth++
	case '}', ']':
		v.depth--
	}
	return false
}

const fence = "```"

// stripFence takes the body of the first code fence whose info string is empty
// or json. A fence of another language, a fence without a line break after
// its opening and an unclosed fence are left to the next steps.
func stripFence(s string) string {
	start := strings.Index(s, fence)
	if start < 0 {
		return s
	}
	rest := s[start+len(fence):]
	eol := strings.IndexByte(rest, '\n')
	if eol < 0 {
		return s
	}
	if info := strings.TrimSpace(rest[:eol]); info != "" && !strings.EqualFold(info, "json") {
		return s
	}
	body := rest[eol+1:]
	end := strings.Index(body, fence)
	if end < 0 {
		return s
	}
	return body[:end]
}

// balancedObject takes the first { up to its matching }, skipping braces that
// sit inside strings. An object that is never closed is left as it is: that is
// the truncated answer, and it must fail.
//
// An object after a [ that nothing closes is an element of an array, and the
// array is the answer — whole, cut or followed by prose. Taking its first
// element would accept a truncated array, so the text is left as it is too.
func balancedObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 || opensArray(s[:start]) {
		return s
	}
	depth := 0
	var sc stringScanner
	for i := start; i < len(s); i++ {
		c := s[i]
		if sc.inString(c) {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s
}

// opensArray reports whether prefix holds a [ that no later ] closes. The
// prefix is the text in front of the first {. From the first [ on the strings
// of the array are followed, so a bracket in a string element — ["]", … — does
// not close it; quotes of the prose outside an array are not strings.
func opensArray(prefix string) bool {
	depth := 0
	var sc stringScanner
	for i := 0; i < len(prefix); i++ {
		c := prefix[i]
		if depth > 0 && sc.inString(c) {
			continue
		}
		switch c {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth > 0
}

// trailingCommas removes every comma outside a string that is followed, after
// white space only, by } or ]. The strings themselves are copied as they are.
func trailingCommas(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var sc stringScanner
	for i := 0; i < len(s); i++ {
		c := s[i]
		if sc.inString(c) || c != ',' || !closesNext(s[i+1:]) {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// closesNext reports whether s starts, after JSON white space, with } or ].
func closesNext(s string) bool {
	t := strings.TrimLeft(s, " \t\r\n")
	return t != "" && (t[0] == '}' || t[0] == ']')
}

// stringScanner follows JSON strings byte by byte. The bytes of a multi-byte
// UTF-8 character are never " or \, so a byte scan is exact.
type stringScanner struct {
	in, escaped bool
}

// inString consumes c and reports whether it belongs to a string, the quotes
// that open and close it included.
func (sc *stringScanner) inString(c byte) bool {
	switch {
	case sc.escaped:
		sc.escaped = false
		return true
	case sc.in && c == '\\':
		sc.escaped = true
		return true
	case c == '"':
		sc.in = !sc.in
		return true
	default:
		return sc.in
	}
}

// labelRe is the call label of ADR-029 p. 4 standing on its own: e or b, then
// 1…99, with no letter, digit or underscore of ASCII on either side. е of
// Cyrillic is not e, and e0 or e100 is not a label.
var labelRe = regexp.MustCompile(`(^|[^0-9A-Za-z_])[eb][1-9][0-9]?([^0-9A-Za-z_]|$)`)

// hasLabel reports whether the text of a validated narrative carries a label.
func hasLabel(doc any) bool {
	obj, _ := doc.(map[string]any)
	text, _ := obj["text"].(string)
	return labelRe.MatchString(text)
}
