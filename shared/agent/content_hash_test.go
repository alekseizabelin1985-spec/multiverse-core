package agent_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/agent"
)

func TestContentHashIsStableOverRepeatedParses(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(validDir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		first := parseValid(t, filepath.Base(f)).ContentHash
		if !contentHashForm.MatchString(first) {
			t.Fatalf("%s: ContentHash = %q, want ^sha256:[0-9a-f]{64}$", f, first)
		}
		for i := 0; i < 100; i++ {
			bp := parseValid(t, filepath.Base(f))
			if bp.ContentHash != first {
				t.Fatalf("%s: parse %d gave %s, first gave %s", f, i, bp.ContentHash, first)
			}
			if got := agent.ContentHash(bp); got != first {
				t.Fatalf("%s: ContentHash(bp) = %s, the parser stored %s", f, got, first)
			}
		}
	}
}

func TestContentHashDoesNotDependOnKeyOrderOrFileName(t *testing.T) {
	a := "---\nname: order\nversion: \"1.0\"\nlevel: task\nrole: personal-gm\n" +
		"llm:\n  phase2: { model: m, temperature: 0.7, max_tokens: 160 }\n  fallback: template\n" +
		"npc_table:\n  - { npc_id: w, spawn: { on: tick, chance: \"0.5\" } }\n" +
		"background_events:\n  - { kind: k, ops: [{ path: p, value: { b: 2, a: 1 } }] }\n---\n## system\nS\n## phase2\nP\n"
	b := "---\nbackground_events:\n  - ops:\n      - value: { a: 1, b: 2 }\n        path: p\n    kind: k\n" +
		"npc_table:\n  - spawn: { chance: \"0.5\", on: tick }\n    npc_id: w\n" +
		"llm:\n  fallback: template\n  phase2:\n    max_tokens: 160\n    temperature: 0.7\n    model: m\n" +
		"role: personal-gm\nlevel: task\nversion: \"1.0\"\nname: order\n---\n## phase2\nP\n## system\nS\n"

	first, err := agent.ParseBytes("a.md", []byte(a))
	if err != nil {
		t.Fatal(err)
	}
	second, err := agent.ParseBytes("elsewhere/b.md", []byte(b))
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentHash != second.ContentHash {
		t.Fatalf("reordered keys and sections change the hash: %s vs %s", first.ContentHash, second.ContentHash)
	}
}

// Every byte of the file is replaced in turn. A replacement either breaks the
// blueprint (the parser refuses it) or changes its content, and then the hash
// has to change: no byte of a parsed blueprint is outside the hash.
func TestContentHashChangesWithAnyByteOfTheContent(t *testing.T) {
	for _, file := range []string{"domain-region.md", "personal-gm.yaml", "global-world.md", "domain-fair.md"} {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(validDir, file))
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(strings.ReplaceAll(string(data), "\r\n", "\n"))
			original, err := agent.ParseBytes(file, data)
			if err != nil {
				t.Fatal(err)
			}
			parsed := 0
			// Equal values have equal hashes and different values different
			// ones: each mutant that parses is recorded both ways, by its value
			// (as JSON, which keeps nil apart from [] and follows pointers) and
			// by its hash.
			hashByValue := map[string]string{valueFingerprint(t, original): original.ContentHash}
			valueByHash := map[string]string{original.ContentHash: valueFingerprint(t, original)}
			for i := range data {
				mutated := append([]byte(nil), data...)
				mutated[i] = 'z'
				if data[i] == 'z' {
					mutated[i] = 'y'
				}
				bp, err := agent.ParseBytes(file, mutated)
				if err != nil {
					continue
				}
				parsed++
				value := valueFingerprint(t, bp)
				if h, seen := hashByValue[value]; seen && h != bp.ContentHash {
					t.Fatalf("byte %d: an equal value has another hash (%s vs %s)", i, h, bp.ContentHash)
				}
				if v, seen := valueByHash[bp.ContentHash]; seen && v != value {
					t.Fatalf("byte %d: a different value has the same hash %s", i, bp.ContentHash)
				}
				hashByValue[value], valueByHash[bp.ContentHash] = bp.ContentHash, value
				if bp.ContentHash == original.ContentHash {
					t.Fatalf("byte %d (%q in %q) replaced, the blueprint still parses, the hash did not change",
						i, data[i], around(data, i))
				}
			}
			if parsed < len(data)/4 {
				t.Fatalf("only %d of %d mutations parsed: the test no longer exercises the hash", parsed, len(data))
			}
		})
	}
}

func valueFingerprint(t *testing.T, bp *agent.AgentBlueprint) string {
	t.Helper()
	v := *bp
	v.SourceFile, v.ContentHash, v.ParseIssues = "", "", nil
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func around(data []byte, i int) string {
	lo, hi := max(0, i-20), min(len(data), i+20)
	return string(data[lo:hi])
}

func TestContentHashChangesWithEachField(t *testing.T) {
	base := parseValid(t, "domain-region.md")
	mutations := map[string]func(bp *agent.AgentBlueprint){
		"version":             func(bp *agent.AgentBlueprint) { bp.Version = "1.2" },
		"scope type":          func(bp *agent.AgentBlueprint) { bp.ScopeBinding.Type = agent.ScopeTypes{"world"} },
		"tick temperature":    func(bp *agent.AgentBlueprint) { v := 0.8; bp.LLM.Tick.Temperature = &v },
		"tick thinking":       func(bp *agent.AgentBlueprint) { bp.LLM.Tick.Thinking = true },
		"retries set":         func(bp *agent.AgentBlueprint) { v := 2; bp.LLM.Retries = &v },
		"canon item":          func(bp *agent.AgentBlueprint) { bp.Prompts.Canon[0] += "!" },
		"canon order":         func(bp *agent.AgentBlueprint) { c := bp.Prompts.Canon; c[0], c[1] = c[1], c[0] },
		"npc count":           func(bp *agent.AgentBlueprint) { bp.NPCTable[0].Count = 2 },
		"immediate broadcast": func(bp *agent.AgentBlueprint) { bp.ImmediateBroadcast = nil },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			bp := parseValid(t, "domain-region.md")
			mutate(bp)
			if agent.ContentHash(bp) == base.ContentHash {
				t.Fatalf("changing %s does not change the hash", name)
			}
		})
	}
}

func TestContentHashIgnoresParseMetadata(t *testing.T) {
	bp := parseValid(t, "personal-gm.md")
	want := bp.ContentHash
	bp.SourceFile = "other.md"
	bp.ContentHash = "sha256:stale"
	bp.ParseIssues = []agent.Issue{{Field: "x", Reason: "y", Severity: agent.SeverityWarning}}
	if got := agent.ContentHash(bp); got != want {
		t.Fatalf("ContentHash depends on SourceFile/ContentHash/ParseIssues: %s vs %s", got, want)
	}
}

func TestContentHashOfEdgeValues(t *testing.T) {
	if got := agent.ContentHash(nil); got != "" {
		t.Errorf("ContentHash(nil) = %q, want empty", got)
	}
	empty := agent.ContentHash(&agent.AgentBlueprint{})
	if !contentHashForm.MatchString(empty) {
		t.Errorf("ContentHash of an empty blueprint = %q", empty)
	}
	withMap := &agent.AgentBlueprint{BackgroundEvents: []agent.BackgroundEvent{{Ops: []agent.OpTemplate{{Path: "p", Value: map[any]any{1: "a"}}}}}}
	for _, key := range []string{"1", "int:1"} {
		withStringKey := &agent.AgentBlueprint{BackgroundEvents: []agent.BackgroundEvent{{Ops: []agent.OpTemplate{{Path: "p", Value: map[string]any{key: "a"}}}}}}
		h1, h2 := agent.ContentHash(withMap), agent.ContentHash(withStringKey)
		if !contentHashForm.MatchString(h1) || h1 == h2 {
			t.Errorf("the key 1 must hash apart from the string key %q: %q vs %q", key, h1, h2)
		}
	}
	withValue := func(v any) *agent.AgentBlueprint {
		return &agent.AgentBlueprint{BackgroundEvents: []agent.BackgroundEvent{{Ops: []agent.OpTemplate{{Path: "p", Value: v}}}}}
	}
	for name, value := range map[string]any{"func": func() {}, "channel": make(chan int), "complex": complex(1, 2)} {
		if got := agent.ContentHash(withValue(value)); got != "" {
			t.Errorf("a %s value has no canonical form, got hash %q, want empty", name, got)
		}
	}
	if agent.ContentHash(withValue(map[any]any{"a": 1})) != agent.ContentHash(withValue(map[string]any{"a": 1})) {
		t.Error("a map[any]any with string keys only must hash as the map[string]any with the same content")
	}
	if !contentHashForm.MatchString(agent.ContentHash(withValue(map[any]any{nil: 1}))) {
		t.Error("a null map key must hash")
	}
	hash := func(v any) string { return agent.ContentHash(withValue(v)) }
	for _, pair := range [][2]any{{1, 1.0}, {1000000, 1000000.0}, {0, math.Copysign(0, -1)}, {uint64(1) << 63, 0x1p63}, {int64(-1) << 63, -0x1p63}} {
		if hash(pair[0]) != hash(pair[1]) {
			t.Errorf("%T %v and %T %v are one number, as in JSON", pair[0], pair[0], pair[1], pair[1])
		}
	}
	for _, pair := range [][2]any{{1, 1.5}, {1, "1"}, {uint64(1) << 63, 0x1p64}, {1e20, "1e+20"}} {
		if hash(pair[0]) == hash(pair[1]) {
			t.Errorf("%T %v and %T %v are different values with one hash", pair[0], pair[0], pair[1], pair[1])
		}
	}
	if hash(math.NaN()) == hash("NaN") || hash(math.Inf(1)) == hash(math.Inf(-1)) || hash([]any(nil)) == hash([]any{}) {
		t.Error("NaN is not the string NaN, +Inf is not -Inf, a nil list is not an empty one")
	}
}

func TestParseRejectsInvalidUTF8(t *testing.T) {
	_, err := agent.ParseBytes("bp.md", []byte("---\nname: x\n---\n## system\n\xff\n"))
	pe := parseError(t, err)
	if pe.Line != 5 || !strings.Contains(pe.Reason, "UTF-8") {
		t.Fatalf("ParseError = %+v", pe)
	}
}

// The parser tells a missing key from an empty one (a nil slice from [], a nil
// pointer from a pointer to zero), and so does the hash: a blueprint missing a
// required list and one with the list empty must not share a content_hash.
func TestContentHashTellsAMissingKeyFromAnEmptyValue(t *testing.T) {
	const head = "---\nname: pair\nversion: \"1.0\"\nlevel: task\nrole: personal-gm\n"
	cases := []struct {
		name, without, with string
	}{
		{"allowed_event_types", head + "---\n", head + "allowed_event_types: []\n---\n"},
		{"owned_entity_types", head + "---\n", head + "owned_entity_types: []\n---\n"},
		{"immediate_broadcast", head + "---\n", head + "immediate_broadcast: []\n---\n"},
		{"tools", head + "---\n", head + "tools: []\n---\n"},
		{"encounter", head + "---\n", head + "encounter: {}\n---\n"},
		{"parent", head + "---\n", head + "parent: {}\n---\n"},
		{"retries", head + "---\n", head + "llm:\n  retries: 0\n---\n"},
		{"phase2", head + "---\n", head + "llm:\n  phase2: {}\n---\n"},
		{"phase2 temperature", head + "llm:\n  phase2: { model: m }\n---\n", head + "llm:\n  phase2: { model: m, temperature: 0 }\n---\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			without, err := agent.ParseBytes("without.md", []byte(tc.without))
			if err != nil {
				t.Fatal(err)
			}
			with, err := agent.ParseBytes("with.md", []byte(tc.with))
			if err != nil {
				t.Fatal(err)
			}
			if valueFingerprint(t, without) == valueFingerprint(t, with) {
				t.Fatal("the parser does not tell the two apart; the case tests nothing")
			}
			if without.ContentHash == with.ContentHash {
				t.Fatalf("missing and empty %s share the hash %s", tc.name, with.ContentHash)
			}
		})
	}

	// Values that are equal stay equal: an explicit zero scalar is the zero the
	// parser gives for a missing key.
	zero, err := agent.ParseBytes("zero.md", []byte(head+"constraints: { max_instances: 0 }\nttl: \"\"\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	missing, err := agent.ParseBytes("missing.md", []byte(head+"---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if valueFingerprint(t, zero) != valueFingerprint(t, missing) || zero.ContentHash != missing.ContentHash {
		t.Fatal("an explicit zero scalar and a missing one are one value and must have one hash")
	}
}

// The hash is published in agent.spawned and agent.blueprint_reloaded and kept
// in the swarm snapshot, so its encoding is part of the contract: a change of
// the canonical form (a library upgrade, a renamed key) changes every stored
// hash and has to be a deliberate change of this value.
func TestContentHashEncodingIsPinned(t *testing.T) {
	const content = "---\nname: pinned\nversion: \"1.0\"\nlevel: task\nrole: personal-gm\n" +
		"llm:\n  phase2: { model: m, temperature: 0.7, max_tokens: 160 }\n---\n## system\nS {events}\n## canon\n- факт\n"
	bp, err := agent.ParseBytes("pinned.md", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	// Iteration 2 of T-201 (review #1, Ma-1) replaced the yaml.Marshal-based
	// form by the reflect-based one: zero fields are left out, [] is kept apart
	// from a missing list. Before: sha256:fa8d91b1c463402aadd1a71be86052bc243cce0c95c4c6d6e299b482628d9457.
	// No hash had been stored anywhere yet. The canonical text behind this value
	// is pinned in TestCanonicalFormOfThePinnedBlueprint.
	const want = "sha256:811cfa8e825941d3d41b1c0cabdaba59b1549e7bc448af0cfdb4047ca1e81371"
	if bp.ContentHash != want {
		t.Fatalf("ContentHash = %s, pinned %s", bp.ContentHash, want)
	}
}

// hashWithValue parses a blueprint whose trigger condition and op carry value,
// written as YAML, and returns the parsed value with the hash.
func hashWithValue(t *testing.T, value string) (any, string) {
	t.Helper()
	content := "---\nname: values\nversion: \"1.0\"\nlevel: task\nrole: personal-gm\n" +
		"trigger: { type: event, event_name: e, conditions: [{ field: f, operator: gte, value: " + value + " }] }\n" +
		"background_events:\n  - { kind: k, ops: [{ path: p, value: " + value + " }] }\n---\n"
	bp, err := agent.ParseBytes("values.md", []byte(content))
	if err != nil {
		t.Fatalf("value %s: %v", value, err)
	}
	return bp.Trigger.Conditions[0].Value, bp.ContentHash
}

// Ma-2 of review #2: YAML reads an unquoted date into an untyped value as a
// time.Time. Before the fix every date had one hash, so a changed date in a
// blueprint went unseen by the registry and the substitution control (T-26).
func TestContentHashTellsDatesApart(t *testing.T) {
	dates := []string{"2001-01-01", "2020-06-01", "2001-01-02", "2001-01-01T00:00:01Z", "2001-01-01T00:00:00.000000001Z",
		"!!timestamp 2001-01-03", "2001-01-01T00:00:00+03:00", `"2001-01-01"`, `"2001-01-01T00:00:00Z"`, `"time:2001-01-01T00:00:00Z"`}
	seen := map[string]string{}
	for _, date := range dates {
		value, hash := hashWithValue(t, date)
		if _, isTime := value.(time.Time); isTime == strings.HasPrefix(date, `"`) {
			t.Fatalf("%s decoded to %T: the case does not test what it says", date, value)
		}
		if other, dup := seen[hash]; dup {
			t.Errorf("%s and %s share the hash %s", date, other, hash)
		}
		seen[hash] = date
	}
}

// One instant with one offset is one value however YAML writes it: the hash
// follows the value, not the notation. The same instant with another offset is
// another value (it reaches the JSON of an event with that offset), so it
// hashes apart; see writeTime.
func TestContentHashOfOneDateInOtherNotations(t *testing.T) {
	_, want := hashWithValue(t, "2001-01-01")
	for _, date := range []string{"2001-1-1", "2001-01-01T00:00:00Z", "2001-01-01 00:00:00", "2001-01-01t00:00:00.000Z",
		"2001-01-01T00:00:00+00:00", "!!timestamp 2001-01-01", "2001-01-01T00:00:00.000000000Z"} {
		if _, got := hashWithValue(t, date); got != want {
			t.Errorf("%s is 2001-01-01 written otherwise, but hashes apart", date)
		}
	}
	utc, inUTC := hashWithValue(t, "2001-01-01T00:00:00Z")
	moscow, inMoscow := hashWithValue(t, "2001-01-01T03:00:00+03:00")
	if !utc.(time.Time).Equal(moscow.(time.Time)) {
		t.Fatal("the pair is not one instant; the case tests nothing")
	}
	if inUTC == inMoscow {
		t.Error("one instant with another offset must hash apart")
	}
}

// Mi-5 of review #2: a key that is not a string cannot take the form of a string
// key, and a mapping with both hashes the same way on every parse.
func TestContentHashOfMixedMapKeys(t *testing.T) {
	_, number := hashWithValue(t, "{1: a}")
	_, text := hashWithValue(t, `{"int:1": a}`)
	if number == text {
		t.Fatal(`{1: a} and {"int:1": a} share a hash`)
	}
	// As values 1 and 1.0 are one number; as keys they are two, since one
	// mapping can hold both, so a key keeps its type.
	if _, float := hashWithValue(t, "{1.0: a}"); float == number {
		t.Fatal("{1: a} and {1.0: a} share a hash, but {1: a, 1.0: b} holds both keys")
	}
	// N-3 of review #3 (acceptance of T-201): the null key is written unquoted,
	// so it cannot take the form of the string key "null".
	null, nullHash := hashWithValue(t, "{~: a}")
	if m, ok := null.(map[any]any); !ok || m[nil] != "a" {
		t.Fatalf("{~: a} decoded to %#v: the case does not test a null key", null)
	}
	if _, textHash := hashWithValue(t, `{"null": a}`); textHash == nullHash {
		t.Fatal(`{~: a} and {"null": a} share a hash`)
	}
	for _, mapping := range []string{`{1: a, "int:1": b}`, `{1: a, 1.0: b, true: c, "bool:true": d, ~: e, "null": f, "float64:1": g}`, `{.nan: a, .NaN: b}`} {
		_, first := hashWithValue(t, mapping)
		for i := 0; i < 100; i++ {
			if _, got := hashWithValue(t, mapping); got != first {
				t.Fatalf("%s: parse %d gave %s, the first %s", mapping, i, got, first)
			}
		}
	}
	_, ab := hashWithValue(t, `{.nan: a, .NaN: b}`)
	_, aa := hashWithValue(t, `{.nan: a, .NaN: a}`)
	if ab == aa {
		t.Error("two NaN keys with different values must hash apart from two with one value")
	}
}
