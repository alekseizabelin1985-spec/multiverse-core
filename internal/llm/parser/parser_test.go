package parser_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"multiverse-core.io/internal/llm/parser"
	"multiverse-core.io/internal/llm/providers/fake"
)

// corpusDir holds the dirty answers of a local model (design EPIC-003 §3.1 B4).
// Every file there has a row in corpus below and every row has a file: a case
// added on one side only fails TestTheCorpusAndItsTableHoldTheSameCases.
const corpusDir = "../../../testdata/llm/preamble"

// outcome is what one answer of the corpus must give.
type outcome struct {
	schema   string
	strategy parser.Strategy // "" when no JSON is recovered
	err      error           // nil, parser.ErrSchemaInvalid or parser.ErrLabelInText
	language error           // nil or parser.ErrLanguage, checked on "text" of a parsed answer
	// text, when set, is what "text" of the value must be exactly: the steps
	// never change the content of a string.
	text string
}

var corpus = map[string]outcome{
	"01-direct.txt":                      {schema: "narrative", strategy: parser.StrategyDirect},
	"02-whitespace-around.txt":           {schema: "narrative", strategy: parser.StrategyDirect},
	"03-preamble.txt":                    {schema: "narrative", strategy: parser.StrategyBalancedObject},
	"04-think.txt":                       {schema: "narrative", strategy: parser.StrategyStripThink},
	"05-think-without-opening-tag.txt":   {schema: "narrative", strategy: parser.StrategyStripThink},
	"06-fence-json.txt":                  {schema: "narrative", strategy: parser.StrategyStripFence},
	"07-fence-plain.txt":                 {schema: "narrative", strategy: parser.StrategyStripFence},
	"08-preamble-and-fence.txt":          {schema: "narrative", strategy: parser.StrategyStripFence},
	"09-think-and-fence.txt":             {schema: "narrative", strategy: parser.StrategyStripFence},
	"10-trailing-text.txt":               {schema: "narrative", strategy: parser.StrategyBalancedObject},
	"11-trailing-commas.txt":             {schema: "narrative", strategy: parser.StrategyTrailingCommas, text: "Волк отступает, ] и } остаются в тексте,"},
	"12-think-fence-trailing-commas.txt": {schema: "narrative", strategy: parser.StrategyTrailingCommas},
	"13-nested-object-in-prose.txt":      {schema: "tick-region", strategy: parser.StrategyBalancedObject},
	"14-truncated.txt":                   {schema: "narrative", err: parser.ErrSchemaInvalid},
	"15-truncated-after-preamble.txt":    {schema: "narrative", err: parser.ErrSchemaInvalid},
	"16-empty.txt":                       {schema: "narrative", err: parser.ErrSchemaInvalid},
	"17-whitespace-only.txt":             {schema: "narrative", err: parser.ErrSchemaInvalid},
	"18-array.txt":                       {schema: "narrative", strategy: parser.StrategyDirect, err: parser.ErrSchemaInvalid},
	"19-prose-only.txt":                  {schema: "narrative", err: parser.ErrSchemaInvalid},
	"20-schema-violation.txt":            {schema: "narrative", strategy: parser.StrategyDirect, err: parser.ErrSchemaInvalid},
	"21-label-in-text.txt":               {schema: "narrative", strategy: parser.StrategyDirect, err: parser.ErrLabelInText},
	"22-chinese.txt":                     {schema: "narrative", strategy: parser.StrategyDirect, language: parser.ErrLanguage},
	"23-latin.txt":                       {schema: "narrative", strategy: parser.StrategyDirect, language: parser.ErrLanguage},
	"24-latin-identifiers.txt":           {schema: "narrative", strategy: parser.StrategyDirect},
	"25-array-truncated.txt":             {schema: "narrative", err: parser.ErrSchemaInvalid},
	"26-array-after-preamble.txt":        {schema: "narrative", err: parser.ErrSchemaInvalid},
	"27-think-unclosed-draft.txt":        {schema: "narrative", err: parser.ErrSchemaInvalid},
	// The first { of the answer is in the preamble: ADR-016 p. 1 takes the
	// first {, and the answer is not recovered — a retry, not a guess.
	"28-brace-in-preamble.txt": {schema: "narrative", err: parser.ErrSchemaInvalid},
}

func loadSchemas(t *testing.T) *parser.Schemas {
	t.Helper()
	set, err := parser.LoadSchemas(os.DirFS("../../../schemas"))
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func schemaOf(t *testing.T, set *parser.Schemas, name string) *parser.Schema {
	t.Helper()
	s, ok := set.Get(name)
	if !ok {
		t.Fatalf("no schema %q among %v", name, set.Names())
	}
	return s
}

var policy = parser.LanguagePolicy{MaxLatinRatio: 0.10}

// The corpus is the golden set of dirty answers: each gives its strategy, its
// error, and — for an answer that parses — the verdict of the language check
// on its text, the order the gateway runs them in (КД §9.2).
func TestTheCorpusGivesItsStrategyAndItsError(t *testing.T) {
	set := loadSchemas(t)
	for file, want := range corpus {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(corpusDir, file))
			if err != nil {
				t.Fatal(err)
			}
			value, strategy, err := parser.Parse(string(raw), schemaOf(t, set, want.schema))
			if strategy != want.strategy {
				t.Errorf("strategy %q, want %q", strategy, want.strategy)
			}
			if want.err != nil {
				if !errors.Is(err, want.err) {
					t.Fatalf("error %v, want %v", err, want.err)
				}
				if value != nil {
					t.Errorf("value %s next to an error", value)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if !json.Valid(value) || len(bytes.TrimSpace(value)) != len(value) {
				t.Fatalf("value is not trimmed JSON: %q", value)
			}
			if want.schema != parser.NarrativeSchema {
				return
			}
			var answer struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(value, &answer); err != nil {
				t.Fatal(err)
			}
			if want.text != "" && answer.Text != want.text {
				t.Errorf("text %q, want %q unchanged", answer.Text, want.text)
			}
			if err := parser.CheckLanguage([]string{answer.Text}, policy); !errors.Is(err, want.language) {
				t.Errorf("language: %v, want %v", err, want.language)
			}
		})
	}
}

// DoD T-209: the corpus holds at least twelve cases, and each named kind of
// dirty answer is among them.
func TestTheCorpusAndItsTableHoldTheSameCases(t *testing.T) {
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		files = append(files, e.Name())
	}
	var rows []string
	for name := range corpus {
		rows = append(rows, name)
	}
	slices.Sort(rows)
	if !slices.Equal(files, rows) {
		t.Fatalf("files %v, table %v", files, rows)
	}
	if len(files) < 12 {
		t.Errorf("%d cases, the DoD asks for at least 12", len(files))
	}
	for _, kind := range []string{"preamble", "think", "fence", "trailing-commas", "nested-object",
		"truncated", "empty", "array"} {
		if !slices.ContainsFunc(files, func(f string) bool { return strings.Contains(f, kind) }) {
			t.Errorf("no case of %s in the corpus", kind)
		}
	}
}

// The dirty forms of providers/fake are the ones the gateway tests will feed
// the parser (T-207); each one lands on the strategy named for it there.
func TestTheDirtyAnswersOfTheFakeProviderAreRecovered(t *testing.T) {
	narrative := schemaOf(t, loadSchemas(t), parser.NarrativeSchema)
	body := fake.Narrative{Text: "Волк отступает в чащу.", Mentions: []string{fake.EntityLabel(1)},
		BackgroundRefs: []string{fake.BackgroundLabel(2)}}.JSON()
	cases := []struct {
		name string
		raw  string
		want parser.Strategy
	}{
		{"clean", body, parser.StrategyDirect},
		{"preamble", fake.WithPreamble(body), parser.StrategyBalancedObject},
		{"think", fake.WithThink(body), parser.StrategyStripThink},
		{"fence", fake.InFence(body), parser.StrategyStripFence},
		{"trailing text", fake.WithTrailingText(body), parser.StrategyBalancedObject},
		{"trailing comma", fake.WithTrailingComma(body), parser.StrategyTrailingCommas},
		{"think, preamble, fence", fake.WithThink(fake.WithPreamble(fake.InFence(body))), parser.StrategyStripFence},
		{"preamble, trailing comma", fake.WithPreamble(fake.WithTrailingComma(body)), parser.StrategyTrailingCommas},
	}
	for _, c := range cases {
		value, strategy, err := parser.Parse(c.raw, narrative)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if strategy != c.want {
			t.Errorf("%s: strategy %q, want %q", c.name, strategy, c.want)
		}
		if strategy.Recovered() != (c.want != parser.StrategyDirect) {
			t.Errorf("%s: recovered %v for %q", c.name, strategy.Recovered(), strategy)
		}
		if !jsonEqual(t, value, []byte(body)) {
			t.Errorf("%s: value %s, want %s", c.name, value, body)
		}
	}
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	ax, _ := json.Marshal(x)
	by, _ := json.Marshal(y)
	return string(ax) == string(by)
}

// A step never adds a character, so no step closes a truncated object — a
// truncated answer is schema_invalid wherever it is cut (DoD T-209: not a
// repair by guess). Nor is a whole object taken out of a wrapping that is cut:
// an array that is never closed, or a reasoning block that is never closed —
// there the answer is cut even when the object is not.
func TestATruncatedAnswerIsNeverCompleted(t *testing.T) {
	narrative := schemaOf(t, loadSchemas(t), parser.NarrativeSchema)
	body := `{"text":"Волк отступает, ] и } остаются.","mentions":["e1","e2"],"background_refs":["b1"]}`
	cases := []struct {
		name string
		wrap func(string) string
		// whole: the wrapping is cut too, so the whole object must fail as well.
		whole bool
	}{
		{"bare", func(s string) string { return s }, false},
		{"preamble", fake.WithPreamble, false},
		{"think", fake.WithThink, false},
		{"preamble and unclosed fence", func(s string) string { return fake.WithPreamble("```json\n" + s) }, false},
		{"second element of an array", func(s string) string { return "[" + body + ", " + s }, true},
		{"array after a preamble", func(s string) string { return "Вот: [" + s }, true},
		{"unclosed think", func(s string) string { return "<think>" + s }, true},
		{"unclosed think after prose", func(s string) string { return "Ответ: <think>" + s }, true},
		{"unclosed think after a byte order mark", func(s string) string { return "\ufeff<think>" + s }, true},
		{"a bracket in a string element of an array", func(s string) string { return `["]", ` + s }, true},
		{"a bracket in a string element of an array after a preamble", func(s string) string { return `Вот: ["x]", ` + s }, true},
		{"a draft and a quote in the reasoning before a closing tag", func(s string) string {
			return "Черновик " + body + " и кавычка \" </think>\n" + s
		}, false},
	}
	for _, c := range cases {
		last := len(body) - 1
		if c.whole {
			last = len(body)
		}
		for cut := 1; cut <= last; cut++ {
			raw := c.wrap(body[:cut])
			if _, _, err := parser.Parse(raw, narrative); !errors.Is(err, parser.ErrSchemaInvalid) {
				t.Fatalf("%s, cut at %d of %q: error %v, want schema_invalid", c.name, cut, raw, err)
			}
		}
	}
}

// The reasoning of a model whose template opens <think> ends with the closing
// tag only. A draft in it and a stray quote after the draft do not make the
// tag look like part of a string: the answer after the tag is taken, not the
// draft (a cut answer there is schema_invalid, TestATruncatedAnswerIsNeverCompleted).
func TestTheAnswerAfterTheReasoningIsTakenNotTheDraft(t *testing.T) {
	narrative := schemaOf(t, loadSchemas(t), parser.NarrativeSchema)
	draft := fake.Narrative{Text: "Черновик: волк рычит.", Mentions: []string{}, BackgroundRefs: []string{}}.JSON()
	answer := fake.Narrative{Text: "Волк отступает в чащу.", Mentions: []string{}, BackgroundRefs: []string{}}.JSON()
	for name, raw := range map[string]string{
		"object draft": "Набросаю " + draft + " и проверю кавычку \" в нём.\n</think>\n\n" + answer,
		"array draft":  `Поля ["text"] и " лишняя кавычка` + "</think>" + answer,
	} {
		value, strategy, err := parser.Parse(raw, narrative)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if strategy != parser.StrategyStripThink {
			t.Errorf("%s: strategy %q, want %q", name, strategy, parser.StrategyStripThink)
		}
		if !jsonEqual(t, value, []byte(answer)) {
			t.Errorf("%s: value %s, want the answer %s", name, value, answer)
		}
	}
}

// The recovery works on the structure around the JSON only: a <think> block, a
// fence and a comma that sit inside a string of the answer stay there.
func TestTheStepsDoNotChangeTheStrings(t *testing.T) {
	narrative := schemaOf(t, loadSchemas(t), parser.NarrativeSchema)
	cases := []struct {
		name, text string
		raw        func(body string) string
		want       parser.Strategy
	}{
		{"think in a string", "Он шепчет <think>тише</think>, и стая замирает.",
			fake.WithPreamble, parser.StrategyBalancedObject},
		{"fence in a string", "Руна ```json``` светится на камне.",
			fake.WithPreamble, parser.StrategyBalancedObject},
		{"comma before a closer in a string", "Следы: {левый, } и [правый, ]",
			func(b string) string { return fake.WithPreamble(fake.WithTrailingComma(b)) }, parser.StrategyTrailingCommas},
		{"escaped quote and backslash", `Он сказал: "стой\" — и }`,
			fake.WithTrailingText, parser.StrategyBalancedObject},
		{"closing think tag in a string", "Он шепчет </think> тише, и стая замирает.",
			fake.WithTrailingComma, parser.StrategyTrailingCommas},
		{"closing think tag in a string after a quote", `Он сказал: "стой" </think> тише`,
			func(b string) string { return fake.InFence(fake.WithTrailingComma(b)) }, parser.StrategyTrailingCommas},
	}
	for _, c := range cases {
		body := fake.Narrative{Text: c.text, Mentions: []string{}, BackgroundRefs: []string{}}.JSON()
		value, strategy, err := parser.Parse(c.raw(body), narrative)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if strategy != c.want {
			t.Errorf("%s: strategy %q, want %q", c.name, strategy, c.want)
		}
		var got fake.Narrative
		if err := json.Unmarshal(value, &got); err != nil {
			t.Fatal(err)
		}
		if got.Text != c.text {
			t.Errorf("%s: text %q, want %q", c.name, got.Text, c.text)
		}
	}
}

// The strategies are the enum of llm.output.parse.strategy, in the order of
// ADR-016 p. 1: the record of an attempt must be able to carry every value the
// parser returns.
func TestTheStrategiesAreTheEnumOfTheRecord(t *testing.T) {
	data, err := os.ReadFile("../../../schemas/events/llm.output.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Properties struct {
			Parse struct {
				Properties struct {
					Strategy struct {
						Enum []parser.Strategy `json:"enum"`
					} `json:"strategy"`
				} `json:"properties"`
			} `json:"parse"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if got, want := parser.Strategies(), doc.Properties.Parse.Properties.Strategy.Enum; !slices.Equal(got, want) {
		t.Errorf("parser strategies %v, llm.output.parse.strategy %v", got, want)
	}
}

// ADR-029 p. 4: a narrative with a call label in its text is schema_invalid;
// the check sees the label only when it stands on its own.
func TestALabelInTheTextOfANarrativeIsSchemaInvalid(t *testing.T) {
	set := loadSchemas(t)
	narrative := schemaOf(t, set, parser.NarrativeSchema)
	labelled := []string{
		"Альфа-волк [e3] рычит.",
		"e12 отступает.",
		"ветер стих, b2",
		"e1 в начале строки",
		"в конце строки b99",
		"Волк (e4) и_тень",
		"первая строка\ne5 на второй",
	}
	plain := []string{
		"Обычный русский текст без ярлыков.",
		"Слово re3 не ярлык.",
		"e0 не ярлык.",
		"e100 не ярлык.",
		"е3 с кириллической е.",
		"Кольцо b1x и x_e2 и e2_ — не ярлыки.",
	}
	for _, text := range labelled {
		raw := fake.Narrative{Text: text, Mentions: []string{}, BackgroundRefs: []string{}}.JSON()
		value, strategy, err := parser.Parse(raw, narrative)
		if !errors.Is(err, parser.ErrLabelInText) || !errors.Is(err, parser.ErrSchemaInvalid) {
			t.Errorf("%q: error %v, want a label in the text (schema_invalid)", text, err)
		}
		if value != nil || strategy != parser.StrategyDirect {
			t.Errorf("%q: value %s, strategy %q", text, value, strategy)
		}
		if strings.Contains(err.Error(), text) {
			t.Errorf("%q: the error quotes the text: %v", text, err)
		}
	}
	for _, text := range plain {
		raw := fake.Narrative{Text: text, Mentions: []string{}, BackgroundRefs: []string{}}.JSON()
		if _, _, err := parser.Parse(raw, narrative); err != nil {
			t.Errorf("%q: %v", text, err)
		}
	}

	// A tick answer carries ids, not labels, and its summaries are not checked.
	tick := schemaOf(t, set, "tick-region")
	raw := `{"events":[{"type":"npc.moved","summary":"e3 уходит к броду","ops":[]}]}`
	if _, _, err := parser.Parse(raw, tick); err != nil {
		t.Errorf("tick with e3 in a summary: %v", err)
	}
	// Nor is a "text" of any schema but the narrative one.
	other, err := parser.LoadSchemas(fstest.MapFS{
		"agent/other.json": {Data: []byte(`{"type":"object","properties":{"text":{"type":"string"}}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := parser.Parse(`{"text":"Альфа-волк [e3] рычит."}`, schemaOf(t, other, "other")); err != nil {
		t.Errorf("text with a label under another schema: %v", err)
	}
}

// An error of Parse never carries the answer: not the text of a string, not a
// key the model invented, not a value the schema rejected.
func TestAnErrorDoesNotQuoteTheAnswer(t *testing.T) {
	narrative := schemaOf(t, loadSchemas(t), parser.NarrativeSchema)
	secret := "ТАЙНАЯ-ФРАЗА-МОДЕЛИ"
	cases := map[string]string{
		"pattern":               `{"text":"x","mentions":["` + secret + `"],"background_refs":[]}`,
		"extra key":             `{"text":"x","mentions":[],"background_refs":[],"` + secret + `":1}`,
		"enum":                  `{"text":"x","mentions":[],"background_refs":[],"tone":"` + secret + `"}`,
		"type":                  `{"text":["` + secret + `"],"mentions":[],"background_refs":[]}`,
		"max length":            `{"text":"` + strings.Repeat(secret, 20) + `","mentions":[],"background_refs":[]}`,
		"prose":                 secret + " и ничего больше",
		"truncated":             `{"text":"` + secret,
		"array of the answer":   `["` + secret + `"]`,
		"label in a whole text": `{"text":"` + secret + ` e1","mentions":[],"background_refs":[]}`,
	}
	for name, raw := range cases {
		_, _, err := parser.Parse(raw, narrative)
		if !errors.Is(err, parser.ErrSchemaInvalid) {
			t.Errorf("%s: error %v, want schema_invalid", name, err)
			continue
		}
		if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "ТАЙН") {
			t.Errorf("%s: the error quotes the answer: %v", name, err)
		}
	}
}

// A key the model invented reaches the location of an error only as *: under
// additionalProperties or patternProperties a key is any text of the answer.
// The keys the schemas declare — in any file of the set, under allOf or anyOf
// too — and the indexes of arrays are printed.
func TestAKeyOfTheAnswerIsMaskedInTheLocation(t *testing.T) {
	set, err := parser.LoadSchemas(fstest.MapFS{
		"agent/common.json": {Data: []byte(`{"$defs":{"pair":{"type":"object",` +
			`"properties":{"left":{"type":"string"}},"additionalProperties":{"type":"integer"}}}}`)},
		"agent/open.json": {Data: []byte(`{"type":"object",` +
			`"properties":{"nested":{"type":"object","properties":{"known":{"type":"string"}},` +
			`"patternProperties":{"^x":{"type":"string"}},"additionalProperties":{"type":"string"}},` +
			`"list":{"type":"array","items":{"$ref":"common.json#/$defs/pair"}}},` +
			`"allOf":[{"properties":{"inall":{"type":"string"}}}],` +
			`"additionalProperties":{"type":"string"}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	open := schemaOf(t, set, "open")
	secret := "ТАЙНЫЙ-КЛЮЧ"
	raw := `{"` + secret + `":1,"nested":{"known":1,"xТАЙНА":1,"` + secret + `2":1},` +
		`"list":[{"left":1,"` + secret + `3":"x"}]}`
	_, _, err = parser.Parse(raw, open)
	if !errors.Is(err, parser.ErrSchemaInvalid) {
		t.Fatalf("error %v, want schema_invalid", err)
	}
	if strings.Contains(err.Error(), "ТАЙН") {
		t.Errorf("the error quotes a key of the answer: %v", err)
	}
	for _, part := range []string{"/*: type", "/nested/known: type", "/nested/*: type", "/list/0/left: type", "/list/0/*: type"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %v does not name %q", err, part)
		}
	}
	// A name declared under allOf only: the collection walks arrays of subschemas.
	_, _, err = parser.Parse(`{"inall":1}`, open)
	if err == nil || !strings.Contains(err.Error(), "/inall: type") {
		t.Errorf("error %v does not name %q", err, "/inall: type")
	}
}

// AssertFormat is on: a format of the schema is a check, not an annotation.
func TestAFormatOfTheSchemaIsAsserted(t *testing.T) {
	set, err := parser.LoadSchemas(fstest.MapFS{
		"agent/when.json": {Data: []byte(`{"type":"string","format":"date-time"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	when := schemaOf(t, set, "when")
	if _, _, err := parser.Parse(`"2026-09-13T10:00:00Z"`, when); err != nil {
		t.Errorf("a date-time: %v", err)
	}
	if _, _, err := parser.Parse(`"вчера"`, when); !errors.Is(err, parser.ErrSchemaInvalid) || !strings.Contains(err.Error(), "/: format") {
		t.Errorf("not a date-time: %v, want schema_invalid at /: format", err)
	}
}

// A schema error names the locations and keywords that failed, a bounded
// number of them.
func TestASchemaErrorNamesTheKeywordsThatFailed(t *testing.T) {
	narrative := schemaOf(t, loadSchemas(t), parser.NarrativeSchema)
	_, strategy, err := parser.Parse(fake.WithPreamble(`{"text":"","mentions":["x"]}`), narrative)
	if strategy != parser.StrategyBalancedObject {
		t.Errorf("strategy %q", strategy)
	}
	for _, part := range []string{"strategy balanced_object", "/text: minLength", "/mentions/0: pattern", "/: required"} {
		if err == nil || !strings.Contains(err.Error(), part) {
			t.Errorf("error %v does not name %q", err, part)
		}
	}

	many := make([]string, 30)
	for i := range many {
		many[i] = "x"
	}
	raw, _ := json.Marshal(map[string]any{"text": "ok", "mentions": many, "background_refs": many})
	_, _, err = parser.Parse(string(raw), narrative)
	if err == nil || !strings.Contains(err.Error(), "more") || strings.Count(err.Error(), ": pattern") > 5 {
		t.Errorf("error of a value broken in many places: %v", err)
	}
}

func TestParseWithoutASchemaIsADefectOfTheCaller(t *testing.T) {
	_, _, err := parser.Parse(`{}`, nil)
	if !errors.Is(err, parser.ErrNoSchema) || errors.Is(err, parser.ErrSchemaInvalid) {
		t.Errorf("error %v, want ErrNoSchema only", err)
	}
}
