package fake_test

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"multiverse-core.io/internal/llm"
	"multiverse-core.io/internal/llm/providers/fake"
)

const body = `{"text":"Волк отступает в чащу.","mentions":["e1"],"background_refs":[]}`

// The dirty forms are fixed strings, so a parser test can name the strategy it
// expects for each of them.
func TestDirtyAnswersWrapTheBody(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"preamble", fake.WithPreamble(body), "Here is the answer in JSON format:\n\n" + body},
		{"think", fake.WithThink(body), "<think>\n" + fake.Think + "\n</think>\n\n" + body},
		{"fence", fake.InFence(body), "```json\n" + body + "\n```"},
		{"trailing text", fake.WithTrailingText(body), body + "\n\nLet me know if you need anything else."},
		{"trailing comma", fake.WithTrailingComma(`{"a":[1,2]}`), `{"a":[1,2],}`},
		{"trailing comma in an array", fake.WithTrailingComma(`[1,2]`), `[1,2,]`},
		{"trailing comma without a closer", fake.WithTrailingComma(`plain`), `plain`},
		{"composed", fake.WithThink(fake.WithPreamble(fake.InFence(body))),
			"<think>\n" + fake.Think + "\n</think>\n\n" + fake.Preamble + "\n\n```json\n" + body + "\n```"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, c.got, c.want)
		}
	}
}

// A dirty answer is not JSON and still carries the body: the parser has to
// recover it, not the fake.
func TestDirtyAnswersAreNotJSONButHoldTheBody(t *testing.T) {
	for name, s := range map[string]string{
		"preamble":       fake.WithPreamble(body),
		"think":          fake.WithThink(body),
		"fence":          fake.InFence(body),
		"trailing text":  fake.WithTrailingText(body),
		"trailing comma": fake.WithTrailingComma(body),
	} {
		if json.Valid([]byte(s)) {
			t.Errorf("%s: %q is valid JSON", name, s)
		}
	}
	if !json.Valid([]byte(body)) {
		t.Fatal("the clean body is not JSON")
	}
}

// A narrative answer references entities and background events by call
// labels (ADR-029): the labels match the patterns of narrative.json, the
// arrays are never null, and an invented label is produced as asked.
func TestNarrativeAnswersCarryCallLabels(t *testing.T) {
	entity := regexp.MustCompile(`^e[1-9][0-9]?$`)
	background := regexp.MustCompile(`^b[1-9][0-9]?$`)
	for k := 1; k <= 99; k++ {
		if !entity.MatchString(fake.EntityLabel(k)) || !background.MatchString(fake.BackgroundLabel(k)) {
			t.Fatalf("labels of %d = %q, %q do not match the schema patterns", k, fake.EntityLabel(k), fake.BackgroundLabel(k))
		}
	}
	if fake.EntityLabel(7) != "e7" || fake.BackgroundLabel(2) != "b2" {
		t.Fatalf("labels = %q, %q; want e7, b2", fake.EntityLabel(7), fake.BackgroundLabel(2))
	}
	if entity.MatchString(fake.EntityLabel(100)) || entity.MatchString(fake.EntityLabel(0)) {
		t.Fatal("a label outside e1…e99 is produced as asked and must not match the pattern")
	}

	n := fake.Narrative{
		Text:           "Альфа-волк рычит.",
		Mentions:       []string{fake.EntityLabel(1), fake.EntityLabel(7)},
		BackgroundRefs: []string{fake.BackgroundLabel(2)},
		Tone:           "tense",
	}
	want := `{"text":"Альфа-волк рычит.","mentions":["e1","e7"],"background_refs":["b2"],"tone":"tense"}`
	if got := n.JSON(); got != want {
		t.Fatalf("narrative:\n got %s\nwant %s", got, want)
	}
	if got := (fake.Narrative{Text: "Тихо."}).JSON(); got != `{"text":"Тихо.","mentions":[],"background_refs":[]}` {
		t.Fatalf("narrative without references = %s", got)
	}

	// The whole path of a guardian test: the fake answers with labels in a
	// dirty envelope and the body comes back unchanged.
	p := fake.New(fake.WithRules(fake.Rule{Phase: llm.PhaseNarrative, Replies: []fake.Reply{
		{Response: llm.Response{Content: fake.WithThink(n.JSON())}},
	}}))
	resp, err := p.Generate(context.Background(), llm.Request{Phase: llm.PhaseNarrative})
	if err != nil || resp.Content != fake.WithThink(want) {
		t.Fatalf("Generate = %q, %v", resp.Content, err)
	}
}

func TestJSONPanicsOnAValueWithoutEncoding(t *testing.T) {
	if got := fake.JSON(map[string]int{"b": 2, "a": 1}); got != `{"a":1,"b":2}` {
		t.Fatalf("JSON = %s", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("no panic for a channel")
		}
	}()
	fake.JSON(make(chan int))
}
