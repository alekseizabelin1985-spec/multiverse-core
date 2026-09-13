package contracts

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/shared/eventbus"
)

// The tests of this file pin what C-07 v1.3 moved into the schemas of the LLM
// record (T-445): the form of the hashes, and which fields of llm.output and
// llm.output.rejected a value of validation_status or reason demands or
// forbids. Every branch of an if/then is held from both sides — a document
// the branch lets through and one it has to stop — so that deleting the
// branch from the schema turns a case red.
//
// What depends on the stage the pipeline reached (filter at invalid, reasons[]
// against the related refusals) is not a property of one record and is not
// tested here: it belongs to the property tests of the gateway (T-211, T-213).

// payloadCase is one document of a table and whether the schema must accept it.
type payloadCase struct {
	name    string
	payload map[string]any
	valid   bool
}

// runPayloadCases validates every case as an event of typ. Validate checks the
// envelope and the payload and nothing else, so one envelope serves every
// type of the file; the publisher and the topic policy are tested elsewhere.
func runPayloadCases(t *testing.T, typ string, cases []payloadCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := eventbus.NewRoot(typ, SourceLLM, "dark-forest-world",
				&eventbus.ScopeRef{ID: "solo:player-A", Type: "solo"}, eventbus.ActorHuman, c.payload)
			ev.Meta.Agent = &eventbus.AgentRef{ID: "player-gm:solo:player-A", Level: "task", Blueprint: "player-gm"}
			err := Validate(ev)
			switch {
			case c.valid && err != nil:
				t.Fatalf("rejected, want accepted: %v", err)
			case !c.valid && !errors.Is(err, ErrInvalidPayload):
				t.Fatalf("error %v, want an invalid payload", err)
			}
		})
	}
}

// llmRecord is a complete llm.output that the schema accepts for status: the
// fields every record carries plus exactly the conditional fields the status
// demands. A case starts from it and changes one thing.
func llmRecord(status string) map[string]any {
	record := map[string]any{
		"phase":             "narrative",
		"attempt":           1,
		"provider":          "openai_compat",
		"model":             "qwen3-8b",
		"params":            map[string]any{"temperature": 0.7},
		"prompt_hash":       exampleHashA,
		"response_hash":     exampleHashB,
		"response_len":      214,
		"validation_status": status,
		"laws_version":      "v1",
		"latency_ms":        1840,
		"tokens":            map[string]any{"prompt": 1200, "completion": 180},
		"cost_usd":          0,
	}
	switch status {
	case "valid", "partially_rejected":
		record["response_raw"] = `{"text": "…"}`
		record["filter"] = filterOf("pass")
	case "invalid":
		record["response_raw"] = `{"text": "…"}`
	case "quarantined":
		record["filter"] = filterOf("block")
	case "filter_error":
		record["filter"] = filterOf("error")
	case "error":
		record["error"] = map[string]any{"code": "timeout"}
	}
	return record
}

func filterOf(status string) map[string]any {
	return map[string]any{"applied": true, "status": status, "filter_version": "a-2026-09"}
}

// with returns a copy of payload with the given fields set; a nil value
// removes the field.
func with(payload map[string]any, fields map[string]any) map[string]any {
	changed := maps.Clone(payload)
	for key, value := range fields {
		if value == nil {
			delete(changed, key)
			continue
		}
		changed[key] = value
	}
	return changed
}

// TestLLMOutputFieldsFollowTheStatus is the table of C-07 v1.3, row by row.
func TestLLMOutputFieldsFollowTheStatus(t *testing.T) {
	cases := []payloadCase{}
	for _, status := range []string{"valid", "partially_rejected", "invalid", "quarantined", "filter_error", "error"} {
		cases = append(cases, payloadCase{status + " with exactly its fields", llmRecord(status), true})
	}

	// Row 1: an answer that was parsed keeps its text, whatever was done
	// with it afterwards.
	for _, status := range []string{"valid", "partially_rejected", "invalid"} {
		cases = append(cases, payloadCase{status + " without response_raw",
			with(llmRecord(status), map[string]any{"response_raw": nil}), false})
	}

	// Row 2: an answer that was used went through the filter and passed.
	for _, status := range []string{"valid", "partially_rejected"} {
		cases = append(cases,
			payloadCase{status + " without filter",
				with(llmRecord(status), map[string]any{"filter": nil}), false},
			payloadCase{status + " with a blocking filter",
				with(llmRecord(status), map[string]any{"filter": filterOf("block")}), false},
			payloadCase{status + " with a failed filter",
				with(llmRecord(status), map[string]any{"filter": filterOf("error")}), false})
	}
	// invalid may come before the filter stage or after it; the schema
	// does not decide which (C-07 v1.3, the property test of the gateway does).
	cases = append(cases,
		payloadCase{"invalid with a filter that passed",
			with(llmRecord("invalid"), map[string]any{"filter": filterOf("pass")}), true})

	// Row 3: a blocked text is not stored (ADR-016 p. 3); the hash and the
	// length stay, they are what an audit needs.
	cases = append(cases,
		payloadCase{"quarantined with the blocked text",
			with(llmRecord("quarantined"), map[string]any{"response_raw": "the blocked text"}), false},
		payloadCase{"quarantined without filter",
			with(llmRecord("quarantined"), map[string]any{"filter": nil}), false},
		payloadCase{"quarantined with a filter that passed",
			with(llmRecord("quarantined"), map[string]any{"filter": filterOf("pass")}), false})

	// Row 4: the filter itself failed, so the text was never judged and is
	// withheld on the same grounds (orchestrator decision on OV-33).
	cases = append(cases,
		payloadCase{"filter_error with the unjudged text",
			with(llmRecord("filter_error"), map[string]any{"response_raw": "the unjudged text"}), false},
		payloadCase{"filter_error without filter",
			with(llmRecord("filter_error"), map[string]any{"filter": nil}), false},
		payloadCase{"filter_error with a blocking filter",
			with(llmRecord("filter_error"), map[string]any{"filter": filterOf("block")}), false})

	// Row 5: there is no answer — no text, nothing filtered, and what went
	// wrong is error{}.
	cases = append(cases,
		payloadCase{"error without error",
			with(llmRecord("error"), map[string]any{"error": nil}), false},
		payloadCase{"error with a text",
			with(llmRecord("error"), map[string]any{"response_raw": `{"text": "…"}`}), false},
		payloadCase{"error with a filter",
			with(llmRecord("error"), map[string]any{"filter": filterOf("pass")}), false})

	// Row 6: error{} belongs to the status error alone.
	for _, status := range []string{"valid", "partially_rejected", "invalid", "quarantined", "filter_error"} {
		cases = append(cases, payloadCase{status + " with an error",
			with(llmRecord(status), map[string]any{"error": map[string]any{"code": "timeout"}}), false})
	}

	runPayloadCases(t, "llm.output", cases)
}

// rejection is a complete llm.output.rejected for reason that the schema
// accepts: a refused answer points at its record, a refused budget names it.
func rejection(reason string) map[string]any {
	payload := map[string]any{"reason": reason, "phase": "tick", "attempt": 1}
	switch reason {
	case "budget_exceeded":
		payload["budget"] = map[string]any{"kind": "background", "limit": 4, "window": "1m"}
	case "unknown_entity":
		payload["llm_output"] = llmOutputRef()
		payload["element"] = map[string]any{"index": 2, "type": "npc.moved"}
		payload["entity"] = map[string]any{"entity": map[string]any{"id": "wolf-omega", "type": "npc"}}
	default:
		payload["llm_output"] = llmOutputRef()
	}
	return payload
}

func llmOutputRef() map[string]any {
	return map[string]any{"event": map[string]any{"id": "ev-llm-1", "type": "llm.output"}}
}

func backgroundRef() map[string]any {
	return map[string]any{"event": map[string]any{"id": "ev-weather-1"}}
}

// TestLLMOutputRejectedFieldsFollowTheReason covers the two conditions of
// llm.output.rejected (C-07 v1.3).
func TestLLMOutputRejectedFieldsFollowTheReason(t *testing.T) {
	budget := map[string]any{"kind": "turn", "limit": 1, "window": "1m"}
	cases := []payloadCase{
		// budget_exceeded is refused before any call: a budget to name,
		// no record to point at.
		{"budget_exceeded with its budget", rejection("budget_exceeded"), true},
		{"budget_exceeded without budget",
			with(rejection("budget_exceeded"), map[string]any{"budget": nil}), false},
		{"budget_exceeded pointing at a record",
			with(rejection("budget_exceeded"), map[string]any{"llm_output": llmOutputRef()}), false},

		// Every other reason refuses an answer that exists.
		{"player_agency pointing at its record", rejection("player_agency"), true},
		{"player_agency without a record",
			with(rejection("player_agency"), map[string]any{"llm_output": nil}), false},
		{"player_agency naming a budget",
			with(rejection("player_agency"), map[string]any{"budget": budget}), false},

		// unknown_entity is a dropped element that is either an entity or a
		// background event, exactly one of the two.
		{"unknown_entity with an entity", rejection("unknown_entity"), true},
		{"unknown_entity with a background event",
			with(rejection("unknown_entity"), map[string]any{"entity": nil, "background_ref": backgroundRef()}), true},
		{"unknown_entity without element",
			with(rejection("unknown_entity"), map[string]any{"element": nil}), false},
		{"unknown_entity naming neither",
			with(rejection("unknown_entity"), map[string]any{"entity": nil}), false},
		{"unknown_entity naming both",
			with(rejection("unknown_entity"), map[string]any{"background_ref": backgroundRef()}), false},
		{"unknown_entity without a record",
			with(rejection("unknown_entity"), map[string]any{"llm_output": nil}), false},
		{"background_ref without an event id",
			with(rejection("unknown_entity"), map[string]any{"entity": nil,
				"background_ref": map[string]any{"event": map[string]any{"type": "world.weather_changed"}}}), false},
		// background_ref is optional beside the other reasons too; the
		// condition is about what unknown_entity demands, not a ban elsewhere.
		{"other with a background event",
			with(rejection("other"), map[string]any{"background_ref": backgroundRef()}), true},
	}
	runPayloadCases(t, "llm.output.rejected", cases)
}

// TestHashesAreSHA256 holds the three hash fields of C-07 v1.3 to the form
// sha256:<64 lowercase hex>, the form shared/entity.StateHash already writes.
func TestHashesAreSHA256(t *testing.T) {
	digits := strings.TrimPrefix(exampleHashA, "sha256:")
	bad := map[string]string{
		"without the prefix":  digits,
		"in upper case":       "sha256:" + strings.ToUpper(digits),
		"prefix in uppercase": "SHA256:" + digits,
		"63 digits":           "sha256:" + digits[:63],
		"65 digits":           "sha256:" + digits + "0",
		"not hex":             "sha256:" + strings.Repeat("g", 64),
	}

	fields := []struct {
		typ, field string
		payload    func() map[string]any
	}{
		{"llm.output", "prompt_hash", func() map[string]any { return llmRecord("valid") }},
		{"llm.output", "response_hash", func() map[string]any { return llmRecord("valid") }},
		{"agent.spawned", "content_hash", func() map[string]any {
			return payloadOf(t, blockVExamples["agent.spawned"].payload)
		}},
		{"agent.blueprint_reloaded", "content_hash", func() map[string]any {
			return payloadOf(t, blockVExamples["agent.blueprint_reloaded"].payload)
		}},
	}
	for _, f := range fields {
		cases := []payloadCase{{f.field + " in the form of C-07", with(f.payload(), map[string]any{f.field: exampleHashB}), true}}
		for _, name := range slices.Sorted(maps.Keys(bad)) {
			cases = append(cases, payloadCase{f.field + " " + name, with(f.payload(), map[string]any{f.field: bad[name]}), false})
		}
		t.Run(f.typ+" "+f.field, func(t *testing.T) { runPayloadCases(t, f.typ, cases) })
	}
}
