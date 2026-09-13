package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// NarrativeSchema is the name of the schema of a narrative answer
// (schemas/agent/narrative.json). Parse checks the text of this schema for a
// call label: only a narrative carries labels (ADR-029 p. 1, 4).
const NarrativeSchema = "narrative"

// schemaDir is where the answer schemas live in the file system LoadSchemas
// reads, the tree rooted at schemas/ — as shared/contracts reads events/.
const schemaDir = "agent"

// schemaBase is the identifier space of the answer schemas. It is the prefix of
// the $id of every file, so a relative $ref resolves among the loaded files and
// never reaches the network: the compiler has no URL loader.
const schemaBase = "https://multiverse-core.io/schemas/agent/"

// draft2020 is the only $schema an answer schema may declare (ADR-016 p. 1).
const draft2020 = "https://json-schema.org/draft/2020-12/schema"

// maxSchemaErrors bounds how many failed keywords an error names: one answer
// can break a schema in hundreds of places, and the log line is for a person.
const maxSchemaErrors = 5

// Schema is one compiled answer schema. It is safe for concurrent use.
type Schema struct {
	name     string
	document []byte
	compiled *jsonschema.Schema
	// names are the property names every schema of the set declares: the only
	// keys of the answer an error may print.
	names map[string]struct{}
}

// Name is the file name of the schema without .json: "narrative",
// "tick-region".
func (s *Schema) Name() string { return s.name }

// Document returns the schema as it is in its file, for Request.Schema: the
// provider gets the same schema the answer is validated with (ADR-016 p. 1).
// The slice is a copy.
func (s *Schema) Document() []byte { return bytes.Clone(s.document) }

// validate decodes text and validates it; it returns the decoded value for the
// checks that follow the schema.
func (s *Schema) validate(text string) (any, error) {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(text))
	if err != nil {
		return nil, errors.New("the value is not JSON")
	}
	if err := s.compiled.Validate(doc); err != nil {
		return nil, errors.New(describe(err, doc, s.names))
	}
	return doc, nil
}

// Schemas is the set of compiled answer schemas, looked up by name. The
// gateway gets the name of the schema from the call as a string (ADR-001
// addendum 2026-09-13 p. 3) and finds the schema here.
type Schemas struct {
	byName map[string]*Schema
}

// LoadSchemas compiles every agent/*.json of fsys, a tree rooted where
// schemas/agent lives (os.DirFS("schemas") on the host). Every schema is
// compiled once, here, at start: validating an answer afterwards needs no I/O.
//
// A file that is not JSON, declares a draft other than 2020-12 or does not
// compile fails the whole load with the name of the file: a process whose
// schema of a phase is broken must not start and answer from templates.
func LoadSchemas(fsys fs.FS) (*Schemas, error) {
	files, err := fs.Glob(fsys, schemaDir+"/*.json")
	if err != nil {
		return nil, fmt.Errorf("llm/parser: list schemas: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("llm/parser: no schemas in %s/", schemaDir)
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	documents := make(map[string][]byte, len(files))
	names := make(map[string]struct{})
	for _, file := range files {
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("llm/parser: read %s: %w", file, err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("llm/parser: parse %s: %w", file, err)
		}
		if err := checkDraft(doc); err != nil {
			return nil, fmt.Errorf("llm/parser: %s: %w", file, err)
		}
		propertyNames(doc, names)
		base := path.Base(file)
		if err := compiler.AddResource(schemaBase+base, doc); err != nil {
			return nil, fmt.Errorf("llm/parser: add %s: %w", file, err)
		}
		documents[base] = data
	}

	set := &Schemas{byName: make(map[string]*Schema, len(files))}
	for base, data := range documents {
		compiled, err := compiler.Compile(schemaBase + base)
		if err != nil {
			return nil, fmt.Errorf("llm/parser: compile %s/%s: %w", schemaDir, base, err)
		}
		name := strings.TrimSuffix(base, ".json")
		set.byName[name] = &Schema{name: name, document: data, compiled: compiled, names: names}
	}
	return set, nil
}

// checkDraft refuses a schema that declares a draft other than 2020-12. A file
// without $schema is compiled as 2020-12.
func checkDraft(doc any) error {
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	declared, ok := obj["$schema"]
	if !ok {
		return nil
	}
	if declared != draft2020 {
		return fmt.Errorf("$schema must be %s, got %v", draft2020, declared)
	}
	return nil
}

// propertyNames adds to names the keys of every properties object in doc, at
// any depth. The names of all the files go into one set: a $ref brings the
// properties of another file into the value.
func propertyNames(doc any, names map[string]struct{}) {
	switch v := doc.(type) {
	case map[string]any:
		if props, ok := v["properties"].(map[string]any); ok {
			for name := range props {
				names[name] = struct{}{}
			}
		}
		for _, child := range v {
			propertyNames(child, names)
		}
	case []any:
		for _, child := range v {
			propertyNames(child, names)
		}
	}
}

// Get returns the schema called name, e.g. "narrative".
func (s *Schemas) Get(name string) (*Schema, bool) {
	schema, ok := s.byName[name]
	return schema, ok
}

// Names lists the loaded schemas in lexical order.
func (s *Schemas) Names() []string {
	names := make([]string, 0, len(s.byName))
	for name := range s.byName {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// describe renders a validation error as the locations and the keywords that
// failed, "/text: maxLength". The messages of the validator are not used: they
// quote the value they reject, and the value is the answer of the model. The
// location is masked for the same reason (see location).
func describe(err error, doc any, names map[string]struct{}) string {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return "the value does not match the schema"
	}
	var failed []string
	var walk func(*jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, cause := range e.Causes {
				walk(cause)
			}
			return
		}
		keyword := "schema"
		if kp := e.ErrorKind.KeywordPath(); len(kp) > 0 {
			keyword = kp[len(kp)-1]
		}
		entry := "/" + location(doc, e.InstanceLocation, names) + ": " + keyword
		if !slices.Contains(failed, entry) {
			failed = append(failed, entry)
		}
	}
	walk(ve)
	more := ""
	if len(failed) > maxSchemaErrors {
		more = fmt.Sprintf(" and %d more", len(failed)-maxSchemaErrors)
		failed = failed[:maxSchemaErrors]
	}
	return "does not match the schema at " + strings.Join(failed, ", ") + more
}

// location renders the path of a failed value in doc. An index of an array and
// a key the schemas declare are printed; any other key is the model's own —
// under additionalProperties or patternProperties it can be any text — and is
// printed as *.
func location(doc any, path []string, names map[string]struct{}) string {
	segments := make([]string, len(path))
	cur := doc
	for i, seg := range path {
		switch v := cur.(type) {
		case []any:
			segments[i] = seg
			cur = nil
			if idx, err := strconv.Atoi(seg); err == nil && idx >= 0 && idx < len(v) {
				cur = v[idx]
			}
		case map[string]any:
			segments[i] = "*"
			if _, ok := names[seg]; ok {
				segments[i] = seg
			}
			cur = v[seg]
		default:
			segments[i] = "*"
			cur = nil
		}
	}
	return strings.Join(segments, "/")
}
