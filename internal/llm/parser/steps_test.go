package parser

import (
	"strings"
	"testing"
	"testing/fstest"
)

// Each step alone: what it removes, and what it must leave for the next step.
func TestEachStepRemovesOnlyItsOwnWrapping(t *testing.T) {
	const obj = `{"a":"b"}`
	cases := []struct {
		name  string
		apply func(string) string
		in    string
		want  string
	}{
		{"think: leading block", stripThink, "  <think>x</think>\n" + obj, obj},
		{"think: two leading blocks", stripThink, "<think>x</think><think>y</think> " + obj, obj},
		{"think: unclosed block leaves no answer", stripThink, "<think>x " + obj, ""},
		{"think: second block unclosed leaves no answer", stripThink, "<think>x</think><think>y " + obj, ""},
		{"think: closing tag only", stripThink, "reasoning\n</think>\n" + obj, "\n" + obj},
		{"think: closing tag only, a quote in the reasoning", stripThink, `он пишет "стой </think>` + obj, obj},
		{"think: closing tag only, after a draft", stripThink, `черновик {"a":"c"} </think>` + obj, obj},
		{"think: closing tag in a string of the answer is kept", stripThink, `{"a":"x </think> y"}`, `{"a":"x </think> y"}`},
		{"think: closing tag after an escaped quote is kept", stripThink, `{"a":"x \" </think> y"}`, `{"a":"x \" </think> y"}`},
		{"think: closing tag in a string of an array is kept", stripThink, `Ответ: ["</think>", {"a":1}]`, `Ответ: ["</think>", {"a":1}]`},
		{"think: a block after text is kept", stripThink, "Ответ: <think>x</think>" + obj, "Ответ: <think>x</think>" + obj},
		{"think: none", stripThink, obj, obj},
		{"think: closing tag in a string of the answer after a leading block", stripThink, "<think>x</think>" + `{"a":"y </think> z"}`, `{"a":"y </think> z"}`},
		{"think: closing tag after a bracket in a string is kept", stripThink, `{"a":"[x] </think> y"}`, `{"a":"[x] </think> y"}`},
		{"think: closing tag only, a draft and a quote after it", stripThink, `черновик {"a":"c"} и " </think>` + obj, obj},
		{"think: closing tag only, a draft array and a quote after it", stripThink, `черновик ["c"] и " </think>` + obj, obj},
		{"think: closing tag only, after a draft cut outside a string", stripThink, `черновик {"a": </think>` + obj, obj},
		{"think: closing tag in a string of a value after a draft is kept", stripThink, `черновик {"a":"c"} {"b":"x </think> y"}`, `черновик {"a":"c"} {"b":"x </think> y"}`},
		{"think: unclosed block after prose leaves the prose", stripThink, "Ответ: <think>" + obj + " проверю", "Ответ: "},
		{"think: unclosed block after a byte order mark leaves no answer", stripThink, "\ufeff<think>x " + obj, ""},
		{"think: leading block after a byte order mark", stripThink, "\ufeff<think>x</think>" + obj, obj},
		{"think: unclosed block after the answer", stripThink, obj + " <think>y", obj + " "},
		{"think: unclosed block after a closed one further in", stripThink, "Ответ: <think>x</think> " + obj + " <think>", "Ответ: <think>x</think> " + obj + " "},
		{"think: opening tag in a string of the answer is kept", stripThink, `{"a":"x <think> y"}`, `{"a":"x <think> y"}`},
		{"fence: json", stripFence, "```json\n" + obj + "\n```", obj + "\n"},
		{"fence: JSON upper case", stripFence, "```JSON\n" + obj + "\n```", obj + "\n"},
		{"fence: no info string", stripFence, "text\n```\n" + obj + "\n```\nmore", obj + "\n"},
		{"fence: another language is kept", stripFence, "```yaml\na: b\n```", "```yaml\na: b\n```"},
		{"fence: no line break is kept", stripFence, "```" + obj + "```", "```" + obj + "```"},
		{"fence: unclosed is kept", stripFence, "```json\n" + obj, "```json\n" + obj},
		{"fence: none", stripFence, obj, obj},
		{"object: first of two", balancedObject, `x {"a":1} y {"b":2}`, `{"a":1}`},
		{"object: nested", balancedObject, `x {"a":{"b":{}}} y`, `{"a":{"b":{}}}`},
		{"object: braces in strings", balancedObject, `x {"a":"}{\"}"} y`, `{"a":"}{\"}"}`},
		{"object: escaped backslash before a quote", balancedObject, `x {"a":"\\"} y`, `{"a":"\\"}`},
		{"object: unclosed is kept", balancedObject, `x {"a":{}`, `x {"a":{}`},
		{"object: none", balancedObject, `[1,2]`, `[1,2]`},
		{"object: element of an array is kept", balancedObject, `[{"a":1}, {"b":`, `[{"a":1}, {"b":`},
		{"object: element of an array after prose is kept", balancedObject, `Вот: [{"a":1}] готово`, `Вот: [{"a":1}] готово`},
		{"object: element after another element is kept", balancedObject, `x [1, {"a":1}]`, `x [1, {"a":1}]`},
		{"object: element of a nested array is kept", balancedObject, `x [[], [{"a":1}]]`, `x [[], [{"a":1}]]`},
		{"object: a closed bracket in the preamble", balancedObject, `Ответ [JSON]: {"a":1}`, `{"a":1}`},
		{"object: a stray closing bracket in the preamble", balancedObject, `Пункт 1] [2]: {"a":1}`, `{"a":1}`},
		{"object: a stray closing bracket does not close an array", balancedObject, `Пункт 1] [{"a":1}]`, `Пункт 1] [{"a":1}]`},
		{"object: a bracket in a string element does not close an array", balancedObject, `["]", {"a":1}, {"b":`, `["]", {"a":1}, {"b":`},
		{"object: a bracket in a string element after prose", balancedObject, `x ["a]", {"a":1}]`, `x ["a]", {"a":1}]`},
		{"object: a bracket in a string of a closed array in the preamble", balancedObject, `Примеры: ["[e1", "b2"] Ответ: {"a":1}`, `{"a":1}`},
		{"object: a quote of the prose before an array is not a string", balancedObject, `Ответ "черновой: [{"a":1}]`, `Ответ "черновой: [{"a":1}]`},
		{"commas: before closers", trailingCommas, `{"a":[1,2, ],"b":{"c":1,	}, }`, `{"a":[1,2 ],"b":{"c":1	} }`},
		{"commas: in strings are kept", trailingCommas, `{"a":"x,}","b":"y, ]",}`, `{"a":"x,}","b":"y, ]"}`},
		{"commas: between values are kept", trailingCommas, `[1,2,3]`, `[1,2,3]`},
		{"commas: at the end are kept", trailingCommas, `[1,`, `[1,`},
	}
	for _, c := range cases {
		if got := c.apply(c.in); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// A schema file that is not an object — the boolean schemas true and false of
// 2020-12 — has no $schema to check and compiles.
func TestABooleanSchemaCompiles(t *testing.T) {
	set, err := LoadSchemas(fstest.MapFS{
		"agent/any.json":  {Data: []byte(`true`)},
		"agent/none.json": {Data: []byte(`false`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	anything, _ := set.Get("any")
	if _, _, err := Parse(`{"x":1}`, anything); err != nil {
		t.Errorf("true: %v", err)
	}
	nothing, _ := set.Get("none")
	if _, _, err := Parse(`{"x":1}`, nothing); err == nil || !strings.Contains(err.Error(), "/: ") {
		t.Errorf("false: %v", err)
	}
}

// A path that leaves the value — past a scalar or an index out of range — can
// only come from a validator that disagrees with the decoded answer; nothing
// past that point is printed.
func TestALocationThatLeavesTheValueIsMasked(t *testing.T) {
	names := map[string]struct{}{"a": {}}
	doc := map[string]any{"a": []any{"x"}}
	for path, want := range map[string]string{
		"a/0":   "a/0",
		"a/0/a": "a/0/*",
		"a/5/a": "a/5/*",
		"b/a":   "*/*",
	} {
		if got := location(doc, strings.Split(path, "/"), names); got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
}
