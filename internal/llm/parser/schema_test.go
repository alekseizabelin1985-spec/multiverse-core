package parser_test

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"multiverse-core.io/internal/llm/parser"
)

const narrativeDoc = `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["text"],` +
	`"properties":{"text":{"type":"string"}}}`

// Every schema of schemas/agent compiles (T-203), under the name the call of
// the gateway carries.
func TestTheSchemasOfTheTreeCompile(t *testing.T) {
	set := loadSchemas(t)
	want := []string{"breach", "narrative", "tick-global", "tick-region"}
	if got := set.Names(); !slices.Equal(got, want) {
		t.Fatalf("names %v, want %v", got, want)
	}
	for _, name := range want {
		s := schemaOf(t, set, name)
		if s.Name() != name {
			t.Errorf("schema %q calls itself %q", name, s.Name())
		}
		file, err := os.ReadFile("../../../schemas/agent/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(s.Document(), file) {
			t.Errorf("%s: the document for the provider is not the file", name)
		}
	}
	if _, ok := set.Get("tick"); ok {
		t.Error("an unknown name is found")
	}
}

// The document handed out is a copy: a provider that edits its request cannot
// change the schema of the next call.
func TestTheDocumentOfASchemaIsACopy(t *testing.T) {
	s := schemaOf(t, loadSchemas(t), parser.NarrativeSchema)
	doc := s.Document()
	doc[0] = 'X'
	if s.Document()[0] == 'X' {
		t.Error("the document shares its bytes")
	}
}

// The narrative schema is static (ADR-029 p. 3): the compiled schema holds the
// limits of its file, with no narrowing per call.
func TestTheNarrativeSchemaHoldsTheLimitsOfItsFile(t *testing.T) {
	narrative := schemaOf(t, loadSchemas(t), parser.NarrativeSchema)
	cases := []struct {
		raw string
		ok  bool
	}{
		{`{"text":"Волк.","mentions":["e1","e2","e3","e4"],"background_refs":["b1","b2"]}`, true},
		{`{"text":"Волк.","mentions":["e1","e2","e3","e4","e5"],"background_refs":[]}`, false},
		{`{"text":"Волк.","mentions":[],"background_refs":["b1","b2","b3"]}`, false},
		{`{"text":"Волк.","mentions":["b1"],"background_refs":[]}`, false},
		{`{"text":"Волк.","mentions":["e0"],"background_refs":[]}`, false},
		{`{"text":"` + strings.Repeat("я", 185) + `","mentions":[],"background_refs":[]}`, true},
		{`{"text":"` + strings.Repeat("я", 186) + `","mentions":[],"background_refs":[]}`, false},
		{`{"text":"Волк.","mentions":[],"background_refs":[],"tone":"grim"}`, true},
		{`{"text":"Волк.","mentions":[],"background_refs":[],"tone":"sad"}`, false},
	}
	for _, c := range cases {
		_, _, err := parser.Parse(c.raw, narrative)
		if (err == nil) != c.ok {
			t.Errorf("%.60s…: error %v, want ok=%v", c.raw, err, c.ok)
		}
	}
}

// T-203: an id of a tick longer than 128 characters is refused by the parser,
// before the guardian (C-07 v1.5).
func TestAnIdOfATickLongerThan128IsSchemaInvalid(t *testing.T) {
	tick := schemaOf(t, loadSchemas(t), "tick-region")
	event := func(id string) string {
		return `{"events":[{"type":"npc.moved","summary":"Волк уходит.","entity":{"id":"` + id + `"},"ops":[]}]}`
	}
	if _, _, err := parser.Parse(event(strings.Repeat("a", 128)), tick); err != nil {
		t.Errorf("id of 128: %v", err)
	}
	if _, _, err := parser.Parse(event(strings.Repeat("a", 129)), tick); !errors.Is(err, parser.ErrSchemaInvalid) {
		t.Errorf("id of 129: %v, want schema_invalid", err)
	}
}

func TestABrokenSchemaFailsTheLoadWithItsFile(t *testing.T) {
	cases := map[string]struct {
		files fstest.MapFS
		want  string
	}{
		"not JSON": {fstest.MapFS{
			"agent/narrative.json": {Data: []byte(narrativeDoc)},
			"agent/tick.json":      {Data: []byte(`{"type":`)},
		}, "agent/tick.json"},
		"another draft": {fstest.MapFS{
			"agent/tick.json": {Data: []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object"}`)},
		}, "agent/tick.json"},
		"does not compile": {fstest.MapFS{
			"agent/tick.json": {Data: []byte(`{"type":"object","properties":{"a":{"type":"strng"}}}`)},
		}, "agent/tick.json"},
		"unresolved ref": {fstest.MapFS{
			"agent/tick.json": {Data: []byte(`{"$ref":"https://example.org/remote.json"}`)},
		}, "agent/tick.json"},
		"no schemas": {fstest.MapFS{
			"events/x.json": {Data: []byte(`{}`)},
		}, "no schemas"},
	}
	for name, c := range cases {
		_, err := parser.LoadSchemas(c.files)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want one naming %q", name, err, c.want)
		}
	}
}

// A schema without $schema is compiled as 2020-12, and a relative $ref between
// two answer schemas resolves among the loaded files.
func TestASchemaWithoutDraftAndWithARelativeRefCompiles(t *testing.T) {
	set, err := parser.LoadSchemas(fstest.MapFS{
		"agent/common.json": {Data: []byte(`{"$defs":{"id":{"type":"string","maxLength":3}}}`)},
		"agent/tick.json": {Data: []byte(`{"type":"object","required":["id"],` +
			`"properties":{"id":{"$ref":"common.json#/$defs/id"}}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	tick := schemaOf(t, set, "tick")
	if _, _, err := parser.Parse(`{"id":"abc"}`, tick); err != nil {
		t.Errorf("valid: %v", err)
	}
	if _, _, err := parser.Parse(`{"id":"abcd"}`, tick); !errors.Is(err, parser.ErrSchemaInvalid) {
		t.Errorf("too long through the ref: %v", err)
	}
}

// A file that cannot be read fails the load too.
func TestAnUnreadableSchemaFailsTheLoad(t *testing.T) {
	_, err := parser.LoadSchemas(unreadable{fstest.MapFS{"agent/tick.json": {Data: []byte(`{}`)}}})
	if err == nil || !strings.Contains(err.Error(), "agent/tick.json") {
		t.Errorf("error %v", err)
	}
}

// unreadable lists its files and refuses to read them.
type unreadable struct{ fstest.MapFS }

func (u unreadable) ReadFile(string) ([]byte, error) { return nil, errors.New("denied") }
