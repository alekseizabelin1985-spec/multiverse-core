package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Severity of an Issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Issue is a finding about a blueprint: the file, the field, the reason.
type Issue struct {
	File     string
	Field    string
	Reason   string
	Severity Severity
}

// ParseError is a blueprint that cannot be parsed. Line is the line of the
// file (not of the frontmatter), 0 when the error has no line.
type ParseError struct {
	File   string
	Line   int
	Field  string
	Reason string
}

func (e *ParseError) Error() string {
	var b strings.Builder
	b.WriteString(e.File)
	if e.Line > 0 {
		b.WriteString(":")
		b.WriteString(strconv.Itoa(e.Line))
	}
	if e.Field != "" {
		b.WriteString(": ")
		b.WriteString(e.Field)
	}
	b.WriteString(": ")
	b.WriteString(e.Reason)
	return b.String()
}

// Section names of a .md blueprint, in the order of C-11.
const (
	SectionSystem      = "system"
	SectionPhase1      = "phase1"
	SectionPhase2      = "phase2"
	SectionTick        = "tick"
	SectionCanon       = "canon"
	SectionDescription = "description"
)

var sectionList = SectionSystem + ", " + SectionPhase1 + ", " + SectionPhase2 + ", " +
	SectionTick + ", " + SectionCanon + ", " + SectionDescription

// MaxBlueprintSize is the largest blueprint accepted, in bytes. A blueprint is
// a page of configuration and prompts; the ceiling keeps mvctl blueprint
// validate on an arbitrary directory and the hot reload from reading a stray
// large file whole.
const MaxBlueprintSize = 1 << 20

// ParseFile reads and parses a blueprint file (.md, .yaml or .yml).
func ParseFile(path string) (*AgentBlueprint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read blueprint: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only file: a close error loses nothing
	data, err := io.ReadAll(io.LimitReader(f, MaxBlueprintSize+1))
	if err != nil {
		return nil, fmt.Errorf("read blueprint %s: %w", path, err)
	}
	return ParseBytes(path, data)
}

// ParseBytes parses the content of a blueprint; name selects the format by its
// extension and is reported in errors and in SourceFile.
//
// A .md file is a YAML frontmatter between two lines "---" followed by the
// sections ## system, ## phase1, ## phase2, ## tick, ## canon (a list, one fact
// per "- " item) and ## description. A .yaml/.yml file has the same keys, with
// the prompts under prompts:. Unknown keys and unknown sections are errors.
func ParseBytes(name string, data []byte) (*AgentBlueprint, error) {
	if len(data) > MaxBlueprintSize {
		return nil, &ParseError{File: name, Reason: fmt.Sprintf("the file is larger than %d bytes", MaxBlueprintSize)}
	}
	text := normalizeText(data)
	if !utf8.ValidString(text) {
		return nil, &ParseError{File: name, Line: invalidUTF8Line(text), Reason: "the file is not valid UTF-8"}
	}
	bp := &AgentBlueprint{}
	var err error
	switch ext := strings.ToLower(filepath.Ext(name)); ext {
	case ".md":
		err = parseMarkdown(name, text, bp)
	case ".yaml", ".yml":
		if err = decodeYAML(name, text, 0, bp); err == nil {
			trimPrompts(&bp.Prompts)
		}
	default:
		err = &ParseError{File: name, Reason: fmt.Sprintf("unsupported blueprint extension %q: want .md, .yaml or .yml", ext)}
	}
	if err != nil {
		return nil, err
	}
	issues, err := migrateLegacy(name, bp)
	if err != nil {
		return nil, err
	}
	bp.SourceFile = name
	bp.ParseIssues = issues
	bp.ContentHash = ContentHash(bp)
	return bp, nil
}

// normalizeText drops a UTF-8 byte order mark and CRLF line ends, so that a
// checkout with autocrlf parses to the same blueprint and the same hash.
func normalizeText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func invalidUTF8Line(text string) int {
	for i, line := range strings.Split(text, "\n") {
		if !utf8.ValidString(line) {
			return i + 1
		}
	}
	return 0
}

func parseMarkdown(name, text string, bp *AgentBlueprint) error {
	lines := strings.Split(text, "\n")
	if strings.TrimRight(lines[0], " \t") != "---" {
		return &ParseError{File: name, Line: 1, Reason: "missing YAML frontmatter: a .md blueprint starts with a line ---"}
	}
	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == "---" {
			closing = i
			break
		}
	}
	if closing < 0 {
		return &ParseError{File: name, Line: 1, Reason: "frontmatter is not closed by a line ---"}
	}
	if err := decodeYAML(name, strings.Join(lines[1:closing], "\n"), 1, bp); err != nil {
		return err
	}
	if !promptsEmpty(bp.Prompts) {
		return &ParseError{File: name, Field: "prompts", Reason: "a .md blueprint gives its prompts as sections (## " + SectionSystem + " …), not under the key prompts"}
	}
	return parseSections(name, lines[closing+1:], closing+2, bp)
}

// trimPrompts gives the prompts of a YAML file the form the sections of a .md
// file get, so that one blueprint written in either format is one content:
// a block scalar (|) keeps a final newline a section does not have.
func trimPrompts(p *Prompts) {
	for _, s := range []*string{&p.System, &p.Phase1, &p.Phase2, &p.Tick, &p.Description} {
		*s = strings.TrimSpace(*s)
	}
	for i := range p.Canon {
		p.Canon[i] = strings.TrimSpace(p.Canon[i])
	}
}

func promptsEmpty(p Prompts) bool {
	return p.System == "" && p.Phase1 == "" && p.Phase2 == "" && p.Tick == "" &&
		len(p.Canon) == 0 && p.Description == ""
}

type section struct {
	name  string
	line  int
	lines []string
}

// parseSections splits the body after the frontmatter into sections. A "## "
// inside a fenced code block is text, not a heading. firstLine is the file
// line of body[0].
func parseSections(name string, body []string, firstLine int, bp *AgentBlueprint) error {
	var sections []*section
	seen := map[string]int{}
	fence, fenceLine := "", 0
	for i, line := range body {
		lineNo := firstLine + i
		if marker := fenceMarker(line); marker != "" {
			switch {
			case fence == "":
				fence, fenceLine = marker, lineNo
			case marker == fence:
				fence = ""
			}
		} else if fence == "" && isSectionHeading(line) {
			sectionName := strings.TrimSpace(strings.TrimPrefix(line, "##"))
			if !knownSection(sectionName) {
				return &ParseError{File: name, Line: lineNo, Field: "## " + sectionName, Reason: "unknown section: want one of " + sectionList}
			}
			if prev, dup := seen[sectionName]; dup {
				return &ParseError{File: name, Line: lineNo, Field: "## " + sectionName, Reason: fmt.Sprintf("section repeated (first at line %d)", prev)}
			}
			seen[sectionName] = lineNo
			sections = append(sections, &section{name: sectionName, line: lineNo})
			continue
		}
		if len(sections) == 0 {
			if strings.TrimSpace(line) != "" {
				return &ParseError{File: name, Line: lineNo, Field: "body", Reason: "text outside sections: after the frontmatter a .md blueprint has only the sections " + sectionList}
			}
			continue
		}
		cur := sections[len(sections)-1]
		cur.lines = append(cur.lines, line)
	}
	if fence != "" {
		// An unclosed fence would swallow every heading after it: the task of
		// ## phase2 would end up inside the system role without an error.
		field := "body"
		if len(sections) > 0 {
			field = "## " + sections[len(sections)-1].name
		}
		return &ParseError{File: name, Line: fenceLine, Field: field, Reason: "code fence " + fence + " is not closed"}
	}
	for _, s := range sections {
		if err := assignSection(name, s, &bp.Prompts); err != nil {
			return err
		}
	}
	return nil
}

// fenceMarker returns the marker of a fence line, or "" for any other line.
// A backtick in the rest of a ``` line makes it inline code, not a fence
// (CommonMark: the info string of a backtick fence has no backticks), so
// "```json``` example" neither opens nor closes a block.
func fenceMarker(line string) string {
	t := strings.TrimLeft(line, " ")
	switch {
	case strings.HasPrefix(t, "```"):
		if strings.Contains(strings.TrimLeft(t, "`"), "`") {
			return ""
		}
		return "```"
	case strings.HasPrefix(t, "~~~"):
		return "~~~"
	}
	return ""
}

func isSectionHeading(line string) bool {
	return line == "##" || strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "##\t")
}

func knownSection(s string) bool {
	switch s {
	case SectionSystem, SectionPhase1, SectionPhase2, SectionTick, SectionCanon, SectionDescription:
		return true
	}
	return false
}

func assignSection(name string, s *section, p *Prompts) error {
	if s.name == SectionCanon {
		canon, err := parseCanon(name, s)
		if err != nil {
			return err
		}
		p.Canon = canon
		return nil
	}
	text := strings.TrimSpace(strings.Join(s.lines, "\n"))
	switch s.name {
	case SectionSystem:
		p.System = text
	case SectionPhase1:
		p.Phase1 = text
	case SectionPhase2:
		p.Phase2 = text
	case SectionTick:
		p.Tick = text
	case SectionDescription:
		p.Description = text
	}
	return nil
}

// parseCanon reads "## canon" as a list: an item starts with "- ", the lines
// that follow it without a blank line continue it.
func parseCanon(name string, s *section) ([]string, error) {
	var items []string
	var cur []string
	flush := func() {
		if cur != nil {
			items = append(items, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for i, raw := range s.lines {
		lineNo := s.line + 1 + i
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			flush()
		case line == "-":
			return nil, &ParseError{File: name, Line: lineNo, Field: "## " + SectionCanon, Reason: "empty list item"}
		case strings.HasPrefix(line, "- "):
			flush()
			cur = []string{strings.TrimSpace(line[2:])}
		case cur != nil:
			cur = append(cur, line)
		default:
			return nil, &ParseError{File: name, Line: lineNo, Field: "## " + SectionCanon, Reason: "the section is a list: every fact starts with \"- \""}
		}
	}
	flush()
	return items, nil
}

var (
	yamlLineRe         = regexp.MustCompile(`line (\d+): (.*)`)
	yamlUnknownFieldRe = regexp.MustCompile(`^field (\S+) not found in type`)
	yamlValueRe        = regexp.MustCompile("`([^`]*)`")
)

// decodeYAML decodes text strictly into bp. lineOffset is the number of file
// lines before the first line of text, so that errors name file lines.
func decodeYAML(name, text string, lineOffset int, bp *AgentBlueprint) error {
	if strings.TrimSpace(text) == "" {
		return &ParseError{File: name, Line: lineOffset + 1, Reason: "empty YAML: a blueprint has at least name, version, level and role"}
	}
	dec := yaml.NewDecoder(strings.NewReader(text))
	dec.KnownFields(true)
	if err := dec.Decode(bp); err != nil {
		if errors.Is(err, io.EOF) {
			// Only comments: the decoder finds no document at all.
			return &ParseError{File: name, Line: lineOffset + 1, Reason: "empty YAML: only comments, a blueprint has at least name, version, level and role"}
		}
		return yamlError(name, text, lineOffset, err)
	}
	if reflect.ValueOf(*bp).IsZero() {
		// "~" or "{}": a document that sets no field of a blueprint.
		return &ParseError{File: name, Line: lineOffset + 1, Reason: "empty YAML: the document sets no field of a blueprint"}
	}
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case errors.Is(err, io.EOF):
		return nil
	case err != nil:
		return yamlError(name, text, lineOffset, err)
	default:
		return &ParseError{File: name, Line: extra.Line + lineOffset, Reason: "more than one YAML document"}
	}
}

// yamlError turns a yaml.v3 error into ParseErrors with a file line and, where
// the line allows it, the dotted path of the field.
func yamlError(name, text string, lineOffset int, err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		pe := &ParseError{File: name, Reason: strings.TrimPrefix(err.Error(), "yaml: ")}
		if m := yamlLineRe.FindStringSubmatch(err.Error()); m != nil {
			line, _ := strconv.Atoi(m[1])
			pe.Line = line + lineOffset
			pe.Reason = m[2]
		}
		return pe
	}
	var root yaml.Node
	// The text decoded far enough to report type errors, so it is valid YAML;
	// if it were not, fieldPath would just find nothing.
	_ = yaml.Unmarshal([]byte(text), &root)
	errs := make([]error, 0, len(typeErr.Errors))
	for _, msg := range typeErr.Errors {
		pe := &ParseError{File: name, Reason: msg}
		if m := yamlLineRe.FindStringSubmatch(msg); m != nil {
			line, _ := strconv.Atoi(m[1])
			pe.Line = line + lineOffset
			pe.Reason = m[2]
			key, value := "", ""
			if u := yamlUnknownFieldRe.FindStringSubmatch(m[2]); u != nil {
				key = u[1]
				pe.Reason = "unknown field"
			} else if v := yamlValueRe.FindStringSubmatch(m[2]); v != nil {
				value = v[1]
			}
			pe.Field = fieldPath(&root, line, key, value)
			if pe.Field == "" {
				pe.Field = key
			}
		}
		errs = append(errs, pe)
	}
	return errors.Join(errs...)
}

// fieldPath finds the field a yaml.v3 message on the given line is about. With
// a key ("field X not found") only a mapping key of that name counts; with a
// value ("cannot unmarshal !!str `x`") only a scalar with that value, which
// tells apart the keys of a flow mapping written on one line; without either,
// any key on the line. The deepest match wins.
func fieldPath(root *yaml.Node, line int, key, value string) string {
	best := ""
	consider := func(p string) {
		if len(p) > len(best) {
			best = p
		}
	}
	scalarMatches := func(n *yaml.Node) bool {
		if n.Line != line || n.Kind != yaml.ScalarNode {
			return false
		}
		if prefix, truncated := strings.CutSuffix(value, "..."); truncated {
			return strings.HasPrefix(n.Value, prefix)
		}
		return n.Value == value
	}
	var walk func(n *yaml.Node, path string)
	walk = func(n *yaml.Node, path string) {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c, path)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				p := joinPath(path, k.Value)
				switch {
				case key != "":
					if k.Line == line && k.Value == key {
						consider(p)
					}
				case value != "":
					if scalarMatches(v) {
						consider(p)
					}
				case k.Line == line:
					consider(p)
				}
				walk(v, p)
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				p := fmt.Sprintf("%s[%d]", path, i)
				if key == "" && value != "" && scalarMatches(c) {
					consider(p)
				}
				walk(c, p)
			}
		}
	}
	walk(root, "")
	return best
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// migrateLegacy moves the keys of the as-is format to their v2 place. A key
// given both ways is an error: which of the two the author meant is not
// something the parser can decide.
func migrateLegacy(name string, bp *AgentBlueprint) ([]Issue, error) {
	var issues []Issue
	warn := func(field, reason string) {
		issues = append(issues, Issue{File: name, Field: field, Reason: reason, Severity: SeverityWarning})
	}
	twice := func(field, other string) error {
		return &ParseError{File: name, Field: field, Reason: "given twice: as " + field + " and as " + other}
	}

	if bp.LegacyType != "" {
		if bp.Role != "" && bp.Role != bp.LegacyType {
			return nil, &ParseError{File: name, Field: "type", Reason: fmt.Sprintf("type is an alias of role and differs from it (role %q, type %q)", bp.Role, bp.LegacyType)}
		}
		bp.Role = bp.LegacyType
		bp.LegacyType = ""
	}

	for _, lp := range []struct {
		field, section string
		legacy         **string
		target         *string
	}{
		{"phase1_prompt", SectionPhase1, &bp.LegacyPhase1Prompt, &bp.Prompts.Phase1},
		{"phase2_prompt", SectionPhase2, &bp.LegacyPhase2Prompt, &bp.Prompts.Phase2},
	} {
		legacy := *lp.legacy
		if legacy == nil {
			continue
		}
		*lp.legacy = nil
		text := strings.TrimSpace(*legacy)
		if text == "" {
			warn(lp.field, "the as-is key "+lp.field+" is obsolete; its empty value is dropped, use the section ## "+lp.section)
			continue
		}
		if *lp.target != "" {
			return nil, twice(lp.field, "## "+lp.section)
		}
		*lp.target = text
		warn(lp.field, "the as-is key "+lp.field+" is moved to the section ## "+lp.section)
	}

	llm := &bp.LLM
	if llm.LegacySchema != nil {
		return nil, &ParseError{File: name, Field: "llm.schema", Reason: "an inline schema is not supported in v2: name the schema file in llm.phase2.schema_ref"}
	}
	// A flat key is reported when it is present, whatever its value: an author
	// of an as-is file with "max_tokens: 0" has to learn that the key moved just
	// as one with "max_tokens: 700". Only a value that says something is moved.
	phase2 := func() *PhaseLLM {
		if llm.Phase2 == nil {
			llm.Phase2 = &PhaseLLM{}
		}
		return llm.Phase2
	}
	if flat := llm.LegacyModel; flat != nil {
		llm.LegacyModel = nil
		if *flat == "" {
			warn("llm.model", "the flat as-is key llm.model is obsolete; its empty value is dropped, use llm.phase2.model")
		} else {
			if llm.Phase2 != nil && llm.Phase2.Model != "" {
				return nil, twice("llm.model", "llm.phase2.model")
			}
			phase2().Model = *flat
			warn("llm.model", "the flat as-is key llm.model is moved to llm.phase2.model")
		}
	}
	if flat := llm.LegacyTemperature; flat != nil {
		llm.LegacyTemperature = nil
		if llm.Phase2 != nil && llm.Phase2.Temperature != nil {
			return nil, twice("llm.temperature", "llm.phase2.temperature")
		}
		phase2().Temperature = flat
		warn("llm.temperature", "the flat as-is key llm.temperature is moved to llm.phase2.temperature")
	}
	if flat := llm.LegacyMaxTokens; flat != nil {
		llm.LegacyMaxTokens = nil
		if *flat == 0 {
			warn("llm.max_tokens", "the flat as-is key llm.max_tokens is obsolete; its zero value is dropped, use llm.phase2.max_tokens")
		} else {
			if llm.Phase2 != nil && llm.Phase2.MaxTokens != 0 {
				return nil, twice("llm.max_tokens", "llm.phase2.max_tokens")
			}
			phase2().MaxTokens = *flat
			warn("llm.max_tokens", "the flat as-is key llm.max_tokens is moved to llm.phase2.max_tokens")
		}
	}
	return issues, nil
}
