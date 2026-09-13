package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func schemaEnum(t *testing.T, file string, path ...string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "schemas", "events", file))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var node any
	if err := json.Unmarshal(data, &node); err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	for _, key := range path {
		m, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%s: %v is not an object", file, path)
		}
		node = m[key]
	}
	items, ok := node.([]any)
	if !ok {
		t.Fatalf("%s: %v is not an enum", file, path)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.(string))
	}
	slices.Sort(out)
	return out
}

// The Go enums are the dictionaries of llm.output; a value only one side
// knows is a record the schema refuses or a status the code never names.
func TestEnumsMatchTheLLMOutputSchema(t *testing.T) {
	statuses := []string{
		string(ValidationValid), string(ValidationPartiallyRejected), string(ValidationInvalid),
		string(ValidationError), string(ValidationQuarantined), string(ValidationFilterError),
	}
	slices.Sort(statuses)
	if want := schemaEnum(t, "llm.output.v1.json", "properties", "validation_status", "enum"); !slices.Equal(statuses, want) {
		t.Errorf("ValidationStatus values %v, schema %v", statuses, want)
	}

	phases := []string{string(PhaseNarrative), string(PhaseTick), string(PhaseDecision), string(PhaseOther)}
	slices.Sort(phases)
	if want := schemaEnum(t, "llm.output.v1.json", "properties", "phase", "enum"); !slices.Equal(phases, want) {
		t.Errorf("Phase values %v, llm.output schema %v", phases, want)
	}
	if want := schemaEnum(t, "llm.output.rejected.v1.json", "properties", "phase", "enum"); !slices.Equal(phases, want) {
		t.Errorf("Phase values %v, llm.output.rejected schema %v", phases, want)
	}
	for _, p := range phases {
		if !Phase(p).Valid() {
			t.Errorf("Phase(%q).Valid() = false", p)
		}
	}
	if Phase("chat").Valid() || Phase("").Valid() {
		t.Error("Valid accepts a phase outside the schema")
	}
}

func TestStatusString(t *testing.T) {
	for s, want := range map[Status]string{
		StatusOK:                           "ok",
		StatusLoading:                      "loading",
		StatusUnavailable:                  "unavailable",
		Degraded(DegradedModelNotResident): "degraded(model_not_resident)",
	} {
		if got := s.String(); got != want {
			t.Errorf("%#v.String() = %q, want %q", s, got, want)
		}
	}
	if StatusLoading == StatusUnavailable {
		t.Error("loading and unavailable are the same status")
	}
}

// A v1 caller sets Temperature, MaxTokens and Think only. The fields v1.1
// added stay zero and out of the recorded params, so the record of such a call
// is what it was before (C-15 "Гарантии").
func TestZeroValuesOfTheV11FieldsKeepAV1Call(t *testing.T) {
	v1 := Params{Temperature: 0.7, MaxTokens: 160}
	got, err := json.Marshal(v1)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if want := `{"temperature":0.7,"max_tokens":160,"thinking":false}`; string(got) != want {
		t.Errorf("v1 params = %s, want %s", got, want)
	}

	var back Params
	if err := json.Unmarshal([]byte(`{"temperature":0.7,"max_tokens":160}`), &back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if back != v1 {
		t.Errorf("a v1 record reads back as %+v, want %+v", back, v1)
	}

	tokens, err := json.Marshal(Tokens{Prompt: 10, Completion: 5})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if want := `{"prompt":10,"completion":5}`; string(tokens) != want {
		t.Errorf("tokens without cached = %s, want %s", tokens, want)
	}

	var r Response
	if r.ReasoningLen != 0 || r.Tokens.Cached != 0 {
		t.Errorf("zero Response carries %+v", r)
	}
}
