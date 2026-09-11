package env

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ExamplePath is the file the manifest is compared against.
const ExamplePath = ".env.example"

// excludedSection holds the variables of the frozen services of the legacy
// compose profile. They keep their as-is names and are outside the manifest by
// decision (infrastructure.md §4.1 p. 2), so the comparison skips the section.
const excludedSection = "legacy"

// Entry is one assignment read from a dotenv file. A commented entry is a
// variable shown in the example but not set by default: an optional block such
// as OLLAMA_* or an alternative value for a host run.
type Entry struct {
	Name  string
	Value string
	// Commented is true for a line of the form "#NAME=value".
	Commented bool
	// Section is the last "# ===== name =====" header above the entry.
	Section string
	Line    int
}

// ParseExample reads the assignments of a dotenv file. It is deliberately
// narrow: .env.example is written by hand and read by compose, so the forms it
// has to understand are NAME=value, a trailing "# comment" and a whole line
// commented out. Everything else is skipped rather than guessed at.
func ParseExample(r io.Reader) ([]Entry, error) {
	var (
		entries []Entry
		section string
	)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for line := 1; sc.Scan(); line++ {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		if name, ok := sectionHeader(raw); ok {
			section = name
			continue
		}
		commented := false
		if strings.HasPrefix(raw, "#") {
			commented = true
			raw = strings.TrimSpace(strings.TrimLeft(raw, "#"))
		}
		name, value, ok := assignment(raw)
		if !ok {
			continue
		}
		entries = append(entries, Entry{
			Name:      name,
			Value:     value,
			Commented: commented,
			Section:   section,
			Line:      line,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read dotenv: %w", err)
	}
	return entries, nil
}

// sectionHeader recognises "# ===== name =====", the way .env.example groups
// variables by the process that reads them.
func sectionHeader(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	body := strings.TrimSpace(strings.TrimLeft(line, "#"))
	if !strings.HasPrefix(body, "=====") {
		return "", false
	}
	return strings.TrimSpace(strings.Trim(body, "= ")), true
}

func assignment(line string) (name, value string, ok bool) {
	eq := strings.Index(line, "=")
	if eq <= 0 {
		return "", "", false
	}
	name = strings.TrimSpace(line[:eq])
	if !validName(name) {
		return "", "", false
	}
	return name, trimInlineComment(line[eq+1:]), true
}

// trimInlineComment drops the trailing comment of a value. A "#" only starts a
// comment after whitespace, so a value may contain one; a quoted value ends at
// its closing quote.
func trimInlineComment(value string) string {
	value = strings.TrimLeft(value, " \t")
	if len(value) > 0 && (value[0] == '"' || value[0] == '\'') {
		if end := strings.IndexByte(value[1:], value[0]); end >= 0 {
			return value[1 : end+1]
		}
	}
	for i := 0; i < len(value); i++ {
		if value[i] != '#' {
			continue
		}
		if i == 0 || value[i-1] == ' ' || value[i-1] == '\t' {
			return strings.TrimSpace(value[:i])
		}
	}
	return strings.TrimSpace(value)
}

// Problem is one disagreement between the manifest and .env.example. Line is 0
// when the problem is a variable missing from the file altogether.
type Problem struct {
	Name string
	Line int
	Text string
}

// String renders the problem the way mvctl env check prints it.
func (p Problem) String() string {
	if p.Line == 0 {
		return fmt.Sprintf("%s: %s", p.Name, p.Text)
	}
	return fmt.Sprintf("%s:%d: %s", p.Name, p.Line, p.Text)
}

// CheckExampleFile compares the manifest with the dotenv example at path.
func CheckExampleFile(path string) ([]Problem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	entries, err := ParseExample(f)
	if err != nil {
		return nil, err
	}
	return CheckExample(entries), nil
}

// CheckExample compares the manifest with the entries of a dotenv example in
// both directions (infrastructure.md v0.3 §4.2):
//
//	a) every declared variable appears in the example, commented or not;
//	b) every variable of the example is declared;
//	c) a variable declared secret is empty in the example;
//	d) a required variable is empty in the example — the file is committed, so
//	   it documents the variable instead of carrying a value;
//	e) a retired variable is absent;
//	f) a value the example does set is inside the enum of its declaration;
//	g) a value the example does set parses as the declared kind.
//
// The legacy section is skipped: those services keep their as-is names and are
// outside the manifest.
//
// Problems are returned sorted by name, so a run of mvctl env check reads the
// same way twice.
func CheckExample(entries []Entry) []Problem {
	var problems []Problem
	seen := make(map[string]Entry, len(entries))
	deprecated := DeprecatedNames()

	for _, e := range entries {
		if isExcluded(e.Section) {
			continue
		}
		if reason, gone := deprecated[e.Name]; gone {
			problems = append(problems, Problem{Name: e.Name, Line: e.Line,
				Text: "declared deprecated and must be removed from " + ExamplePath + " — " + reason})
			continue
		}
		v, declared := Lookup(e.Name)
		if !declared {
			problems = append(problems, Problem{Name: e.Name, Line: e.Line,
				Text: "present in " + ExamplePath + " but not declared through env.Declare"})
			continue
		}
		if prev, dup := seen[e.Name]; dup && !prev.Commented && !e.Commented {
			problems = append(problems, Problem{Name: e.Name, Line: e.Line,
				Text: fmt.Sprintf("assigned twice, the first time on line %d", prev.Line)})
		}
		if _, dup := seen[e.Name]; !dup || !e.Commented {
			seen[e.Name] = e
		}
		if e.Commented {
			// A commented entry documents a variable without setting it; the
			// value rules below only apply to what the file actually assigns.
			continue
		}
		problems = append(problems, valueProblems(v, e)...)
	}

	for _, v := range Manifest() {
		if _, ok := seen[v.name]; !ok {
			problems = append(problems, Problem{Name: v.name,
				Text: "declared through env.Declare but missing from " + ExamplePath})
		}
	}

	sort.SliceStable(problems, func(i, j int) bool { return problems[i].Name < problems[j].Name })
	return problems
}

// isExcluded matches the section by its first word: a header carries an
// explanation after the name ("legacy (profile legacy; as-is names)"), and the
// name is what identifies the section.
func isExcluded(section string) bool {
	name, _, _ := strings.Cut(section, " ")
	return strings.EqualFold(name, excludedSection)
}

func valueProblems(v Var, e Entry) []Problem {
	var problems []Problem
	switch {
	case e.Value == "":
	case v.secret:
		problems = append(problems, Problem{Name: e.Name, Line: e.Line,
			Text: "declared secret: the value in " + ExamplePath + " must be empty"})
	case v.required:
		problems = append(problems, Problem{Name: e.Name, Line: e.Line,
			Text: "required and without a default: the value in " + ExamplePath + " must be empty"})
	case len(v.enum) > 0 && !contains(v.enum, e.Value):
		problems = append(problems, Problem{Name: e.Name, Line: e.Line,
			Text: fmt.Sprintf("value %q is outside the declared set %s", e.Value, strings.Join(v.enum, ", "))})
	default:
		if err := v.checkKind(e.Value); err != nil {
			problems = append(problems, Problem{Name: e.Name, Line: e.Line,
				Text: fmt.Sprintf("value %q is not a valid %s", e.Value, v.Kind())})
		}
	}
	return problems
}
