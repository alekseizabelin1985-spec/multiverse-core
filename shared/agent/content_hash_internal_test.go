package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"reflect"
	"testing"
	"time"
)

// Mi-2: the canonical form of a blueprint does not depend on the set of fields
// of the type. A version of a struct with more fields, all left at their zero
// value, has the same canonical form as the version without them; a new field
// counts only once a blueprint sets it.
func TestCanonicalFormIgnoresFieldsLeftAtZero(t *testing.T) {
	type before struct {
		Name  string   `yaml:"name"`
		Types []string `yaml:"allowed_event_types"`
	}
	type nested struct {
		Timeout string `yaml:"timeout"`
	}
	type after struct {
		Retries *int              `yaml:"retries"`
		Name    string            `yaml:"name"`
		Round   nested            `yaml:"round"`
		Ops     []string          `yaml:"ops"`
		Extra   map[string]string `yaml:"extra"`
		Flag    bool              `yaml:"flag"`
		Types   []string          `yaml:"allowed_event_types"`
		Hidden  string            `yaml:"-"`
		note    string
	}
	canonical := func(v any) string {
		var buf bytes.Buffer
		if err := writeCanonical(&buf, reflect.ValueOf(v), false); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	old := canonical(before{Name: "x", Types: []string{}})
	grown := canonical(after{Name: "x", Types: []string{}, Hidden: "h", note: "n"})
	if old != grown {
		t.Fatalf("fields left at zero change the canonical form:\n%s\n%s", old, grown)
	}
	if want := `{"allowed_event_types":[],"name":"x"}`; old != want {
		t.Fatalf("canonical form = %s, want %s", old, want)
	}
	zero := 0
	if set := canonical(after{Name: "x", Types: []string{}, Retries: &zero}); set == old {
		t.Fatal("a new field set to a pointer to zero must change the canonical form")
	}
}

// Untyped values: a string key is written as the string, any other key unquoted
// with its type, so a mapping read by YAML with mixed keys stays readable and
// exact.
func TestCanonicalFormOfUntypedValues(t *testing.T) {
	op := OpTemplate{Path: "p", Value: map[any]any{"a": 1, 1: []any{"x", nil, 1.5, true}}}
	var buf bytes.Buffer
	if err := writeCanonical(&buf, reflect.ValueOf(op), false); err != nil {
		t.Fatal(err)
	}
	const want = `{"path":"p","value":{"a":1,int:1:["x",null,1.5,true]}}`
	if got := buf.String(); got != want {
		t.Fatalf("canonical form = %s, want %s", got, want)
	}
}

// The canonical text of the blueprint pinned by TestContentHashEncodingIsPinned:
// what the hash is taken over, readable, so that a change of the encoding shows
// in review as a change of text rather than of an opaque hash.
func TestCanonicalFormOfThePinnedBlueprint(t *testing.T) {
	const content = "---\nname: pinned\nversion: \"1.0\"\nlevel: task\nrole: personal-gm\n" +
		"llm:\n  phase2: { model: m, temperature: 0.7, max_tokens: 160 }\n---\n## system\nS {events}\n## canon\n- факт\n"
	bp, err := ParseBytes("pinned.md", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeCanonical(&buf, reflect.ValueOf(*bp), false); err != nil {
		t.Fatal(err)
	}
	const want = `{"level":"task","llm":{"phase2":{"max_tokens":160,"model":"m","temperature":0.7}},` +
		`"name":"pinned","prompts":{"canon":["факт"],"system":"S {events}"},"role":"personal-gm","version":"1.0"}`
	if got := buf.String(); got != want {
		t.Fatalf("canonical form =\n%s\nwant\n%s", got, want)
	}
	sum := sha256.Sum256([]byte(contentHashDomain + want))
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != bp.ContentHash {
		t.Fatalf("the hash is not SHA-256 of the domain and the canonical form: %s vs %s", got, bp.ContentHash)
	}
}

func canonicalText(t *testing.T, v any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := writeCanonical(&buf, reflect.ValueOf(v), false); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// N-1 of review #2: a number is written by its value. An integral float within
// int64 or uint64 is that integer; any other float keeps a "." or an exponent.
func TestCanonicalFormOfNumbers(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{1, "1"},
		{1.0, "1"},
		{1000000.0, "1000000"},
		{math.Copysign(0, -1), "0"},
		{-0x1p63, "-9223372036854775808"},
		{int64(math.MinInt64), "-9223372036854775808"},
		{0x1p63, "9223372036854775808"},
		{uint64(1) << 63, "9223372036854775808"},
		{0x1p64, "1.8446744073709552e+19"},
		{-0x1p64, "-1.8446744073709552e+19"},
		{1e20, "1e+20"},
		{1.5, "1.5"},
		{-2.5e-7, "-2.5e-07"},
		{float32(0.5), "0.5"},
		{math.NaN(), "NaN"},
		{math.Inf(-1), "-Inf"},
	}
	for _, tc := range cases {
		if got := canonicalText(t, tc.value); got != tc.want {
			t.Errorf("canonical form of %T %v = %s, want %s", tc.value, tc.value, got, tc.want)
		}
	}
}

// Ma-2 of review #2: a time.Time has only unexported fields; it is written as
// an unquoted, labelled RFC 3339 instant in its own offset.
func TestCanonicalFormOfTimes(t *testing.T) {
	cases := []struct {
		value time.Time
		want  string
	}{
		{time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC), "time:2001-01-01T00:00:00Z"},
		{time.Date(2001, 6, 1, 9, 30, 0, 500_000_000, time.FixedZone("", 3*3600)), "time:2001-06-01T09:30:00.5+03:00"},
		{time.Date(2001, 6, 1, 9, 30, 0, 1, time.FixedZone("MSK", 3*3600)), "time:2001-06-01T09:30:00.000000001+03:00"},
		{time.Time{}, "time:0001-01-01T00:00:00Z"},
	}
	for _, tc := range cases {
		if got := canonicalText(t, tc.value); got != tc.want {
			t.Errorf("canonical form of %v = %s, want %s", tc.value, got, tc.want)
		}
	}
	if got := canonicalText(t, OpTemplate{Path: "p", Value: map[any]any{time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC): "a"}}); got != `{"path":"p","value":{time.Time:time:2001-01-01T00:00:00Z:"a"}}` {
		t.Errorf("a time key = %s", got)
	}
}

// A struct inside an untyped value hides what its unexported fields hold, so it
// has no canonical form; the same struct as a field of a blueprint type is
// written by its exported fields (Mi-2).
func TestCanonicalFormRefusesHiddenFieldsInUntypedValues(t *testing.T) {
	type hidden struct {
		Name string `yaml:"name"`
		note string
	}
	type holder struct {
		Typed hidden `yaml:"typed"`
	}
	if got := canonicalText(t, holder{Typed: hidden{Name: "x", note: "n"}}); got != `{"typed":{"name":"x"}}` {
		t.Fatalf("a typed struct field = %s", got)
	}
	for name, value := range map[string]any{
		"in an op value":       OpTemplate{Value: hidden{Name: "x"}},
		"under a pointer":      OpTemplate{Value: &hidden{Name: "x"}},
		"in a list":            OpTemplate{Value: []hidden{{Name: "x"}}},
		"in a map value":       OpTemplate{Value: map[string]any{"k": hidden{Name: "x"}}},
		"as a map key":         OpTemplate{Value: map[any]any{hidden{Name: "x"}: 1}},
		"as a typed map key":   OpTemplate{Value: map[hidden]int{{Name: "x"}: 1}},
		"in a condition value": Condition{Value: hidden{}},
		// N-4 of review #3 (acceptance of T-201): below an interface the flag
		// reaches a struct field and a value of a typed map too, not only what
		// passes through another interface. The field is non-zero, since a
		// zero field is not written at all.
		"in a field of a struct in an op value": OpTemplate{Value: struct {
			Inner hidden `yaml:"inner"`
		}{Inner: hidden{Name: "x"}}},
		"as a value of a typed map": OpTemplate{Value: map[string]hidden{"k": {Name: "x"}}},
		"as a key of a typed map[any]": struct {
			M map[any]int `yaml:"m"`
		}{M: map[any]int{hidden{Name: "x"}: 1}},
	} {
		var buf bytes.Buffer
		if err := writeCanonical(&buf, reflect.ValueOf(value), false); err == nil {
			t.Errorf("%s: a struct with unexported fields got the form %s, want an error", name, buf.String())
		}
	}
	type exported struct {
		Name string `yaml:"name"`
	}
	if got := canonicalText(t, OpTemplate{Value: exported{Name: "x"}}); got != `{"value":{"name":"x"}}` {
		t.Fatalf("a struct with exported fields only = %s", got)
	}
}
