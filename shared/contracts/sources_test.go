package contracts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

// TestKnownSourcesHoldsEverySourceConstant is the guard the list itself cannot
// give: knownSources is written by hand next to the constants, and a Source*
// added in EPIC-002 or EPIC-003 without a line here would not break the build
// and would not fail the tests of the package that added it. It would fail
// `mvctl contracts check` in another job, with a message that blames the wrong
// side — "core/x is not an envelope source of the platform", when the source
// is real and the list is short.
//
// The constants are read out of sources.go rather than listed here a second
// time: a table would have to be kept in step by hand too, and would only move
// the omission one file along.
func TestKnownSourcesHoldsEverySourceConstant(t *testing.T) {
	declared := sourceConstants(t)
	if len(declared) == 0 {
		t.Fatal("no Source* constant was found in sources.go")
	}

	known := KnownSources()
	for name, value := range declared {
		if !slices.Contains(known, value) {
			t.Errorf("%s = %q is a source constant that knownSources does not list", name, value)
		}
	}

	values := make([]string, 0, len(declared))
	for _, value := range declared {
		values = append(values, value)
	}
	for _, source := range known {
		if !slices.Contains(values, source) {
			t.Errorf("knownSources lists %q, which no Source* constant declares", source)
		}
	}
	seen := map[string]bool{}
	for _, source := range known {
		if seen[source] {
			t.Errorf("knownSources lists %q twice", source)
		}
		seen[source] = true
	}
}

// TestKnownSourcesIsACopy keeps a caller from editing the platform's own list
// through the slice it was handed.
func TestKnownSourcesIsACopy(t *testing.T) {
	first := KnownSources()
	first[0] = "core/nowhere"
	if slices.Contains(KnownSources(), "core/nowhere") {
		t.Error("KnownSources returns the list itself, not a copy of it")
	}
}

// sourceConstants returns the Source* constants of sources.go by name and
// value.
func sourceConstants(t *testing.T) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "sources.go", nil, 0)
	if err != nil {
		t.Fatalf("parse sources.go: %v", err)
	}

	out := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != len(value.Values) {
				continue
			}
			for i, name := range value.Names {
				if len(name.Name) <= len("Source") || name.Name[:len("Source")] != "Source" {
					continue
				}
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Errorf("%s is not declared as a string literal", name.Name)
					continue
				}
				unquoted, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatalf("%s: %v", name.Name, err)
				}
				out[name.Name] = unquoted
			}
		}
	}
	return out
}
