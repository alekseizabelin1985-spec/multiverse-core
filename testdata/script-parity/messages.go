package main

// The static half of the stand: the texts of the messages, read out of the
// source files and compared as templates. With pwsh on PATH it is a second line
// of defence behind the runs; without pwsh — a Linux runner without PowerShell
// — it is the only thing that watches the PowerShell half, which is why a
// message changed in one file of a pair turns it red even when no scenario
// prints that message.
//
// A template is a string literal with every expansion replaced by <V>:
// $VAR, ${VAR}, $(…) in bash; $var, $script:var, ${var}, $(…) in PowerShell;
// %s and %d in printf and in the Python helper of the bench. A literal counts
// as a message when its text starts with "llm: " or "bench: ", or when it is
// handed to a sink that adds the prefix or prints it later (die, fail(),
// Stop-Bench, LLM_EP_ERROR=, .Error =, the reason of the pid file).
//
// A template present in one file of a pair and absent from the other is a
// finding, unless it is listed in messageExceptions with the reason the two
// halves legitimately differ there. An exception that no longer matches
// anything is a finding too: the list may not outlive what it excuses.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type lintResult struct {
	status string
	id     string
	title  string
	notes  []string
}

type messagePair struct {
	id    string
	title string
	sh    []string
	ps    []string
}

var messagePairs = []messagePair{
	{id: "P01", title: "messages: llm-server.sh + lib/llm-endpoint.sh against llm-server.ps1 + lib/LlmEndpoint.psm1",
		sh: []string{"scripts/llm-server.sh", "scripts/lib/llm-endpoint.sh"},
		ps: []string{"scripts/llm-server.ps1", "scripts/lib/LlmEndpoint.psm1"}},
	{id: "P02", title: "messages: llm-bench.sh against llm-bench.ps1",
		sh: []string{"scripts/llm-bench.sh"},
		ps: []string{"scripts/llm-bench.ps1"}},
}

type messageException struct {
	pair     string
	side     impl
	template string
	reason   string
}

// messageExceptions: the places where the two halves say different things on
// purpose. Every entry names why.
var messageExceptions = []messageException{
	// llm-server: the argument parser. PowerShell validates -Action with
	// ValidateSet and prints its own error record; bash has to say it.
	{"P01", implSh, "llm: unknown argument <V>", "PowerShell binds -Action with ValidateSet and reports an unknown value itself"},
	// The closed set of providers: a literal list in bash, $LlmProviders -join
	// ', ' in PowerShell. The rendered sentence is the same (scenario H36).
	{"P01", implSh, "llm: MV_LLM_PROVIDER=<V> is not one of openai_compat, ollama, anthropic, recorded, fake; the set is declared in shared/env and mvctl env check refuses the same value", "bash writes the set out; H36 compares the printed sentence"},
	{"P01", implPS, "llm: MV_LLM_PROVIDER=<V> is not one of <V>; the set is declared in shared/env and mvctl env check refuses the same value", "PowerShell joins $LlmProviders; H36 compares the printed sentence"},

	// llm-bench: tools and arguments that exist in one half only.
	{"P02", implSh, "bench: curl not found in PATH", "the PowerShell half makes its requests with Invoke-WebRequest"},
	{"P02", implSh, "bench: python3 not found in PATH (it is the JSON tool of this script; jq is not used)", "the PowerShell half reads JSON with ConvertFrom-Json"},
	{"P02", implSh, "bench: --repeats must be a whole number, got <V>", "PowerShell binds -Repeats as [int] and rejects the value itself"},
	{"P02", implSh, "bench: --repeats must be at least 1", "the same check, with the argument spelled -Repeats (the PowerShell template below)"},
	{"P02", implPS, "bench: -Repeats must be at least 1", "the same check, with the argument spelled --repeats (the bash template above)"},
	{"P02", implSh, "bench: unknown argument <V> (try --help)", "PowerShell binds the parameters itself"},
	{"P02", implSh, "bench: internal: unknown helper command <V>", "the Python helper exists in the bash half only"},
	{"P02", implSh, "bench: internal: could not describe the <V> file <V>", "the Python helper exists in the bash half only"},
	{"P02", implSh, "bench: internal: the warm-up body could not be built", "the Python helper exists in the bash half only"},
	{"P02", implSh, "bench: internal: the answer parser failed on <V>", "the Python helper exists in the bash half only"},
	{"P02", implSh, "bench: internal: the aggregator failed on <V>/<V>", "the Python helper exists in the bash half only"},
	{"P02", implSh, "bench: configuration <V> could not be read from <V>", "a crash of the Python helper on a matrix that is not JSON; PowerShell stops with the error of ConvertFrom-Json"},
	{"P02", implSh, "bench: <V> line <V> is not JSON: <V>", "the prompt set is parsed by the Python helper; PowerShell stops with the error of ConvertFrom-Json"},
	{"P02", implSh, "bench: <V> line <V> has no <V>", "the Python helper checks the keys of a prompt; PowerShell reads them under Set-StrictMode"},
	{"P02", implSh, "bench: no prompts with phase=<V> in <V>", "bash stops the run on a phase without prompts (fail of the helper); PowerShell skips the phase — the template below"},
	{"P02", implPS, "bench: no prompts with phase=<V> in <V> — skipped", "PowerShell skips a phase without prompts; bash stops the run — the template above"},
	{"P02", implPS, "bench: <V> has no prompts", "PowerShell loads the whole prompt set first; bash finds out per phase"},
}

func lintMessages(source string) []lintResult {
	var out []lintResult
	for _, pair := range messagePairs {
		res := lintResult{id: pair.id, title: pair.title}
		sh, errSh := collectTemplates(source, pair.sh, scanShell)
		ps, errPs := collectTemplates(source, pair.ps, scanPowerShell)
		if errSh != nil || errPs != nil {
			res.status = statusFail
			res.notes = append(res.notes, fmt.Sprint(errSh, errPs))
			out = append(out, res)
			continue
		}
		excused := map[string]bool{}
		used := map[int]bool{}
		for i, ex := range messageExceptions {
			if ex.pair != pair.id {
				continue
			}
			var own, other map[string][]string
			if ex.side == implSh {
				own, other = sh, ps
			} else {
				own, other = ps, sh
			}
			if _, ok := own[ex.template]; ok {
				if _, ok := other[ex.template]; !ok {
					excused[string(ex.side)+"\x00"+ex.template] = true
					used[i] = true
				}
			}
		}
		for _, missing := range oneSided(sh, ps) {
			if !excused["sh\x00"+missing] {
				res.notes = append(res.notes, fmt.Sprintf("only in bash (%s): %q", strings.Join(sh[missing], ", "), missing))
			}
		}
		for _, missing := range oneSided(ps, sh) {
			if !excused["ps1\x00"+missing] {
				res.notes = append(res.notes, fmt.Sprintf("only in PowerShell (%s): %q", strings.Join(ps[missing], ", "), missing))
			}
		}
		for i, ex := range messageExceptions {
			if ex.pair == pair.id && !used[i] {
				res.notes = append(res.notes, fmt.Sprintf("stale exception (%s side): %q no longer differs — remove it from messageExceptions", ex.side, ex.template))
			}
		}
		common := 0
		for t := range sh {
			if _, ok := ps[t]; ok {
				common++
			}
		}
		res.title = fmt.Sprintf("%s — %d shared templates, %d accepted differences", pair.title, common, len(used))
		res.status = statusPass
		if len(res.notes) > 0 {
			res.status = statusFail
		}
		out = append(out, res)
	}
	return out
}

func oneSided(a, b map[string][]string) []string {
	var out []string
	for t := range a {
		if _, ok := b[t]; !ok {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// collectTemplates maps a template to the places (file:line) it appears at.
func collectTemplates(root string, files []string, scan func(string) []literal) (map[string][]string, error) {
	out := map[string][]string{}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			return nil, err
		}
		src := strings.ReplaceAll(string(raw), "\r\n", "\n")
		for _, lit := range scan(src) {
			t, ok := messageTemplate(lit)
			if !ok {
				continue
			}
			out[t] = append(out[t], fmt.Sprintf("%s:%d", filepath.Base(f), lit.line))
		}
	}
	return out, nil
}

type literal struct {
	text     string // with <V> for every expansion
	line     int
	lineText string
}

var (
	rePrintf     = regexp.MustCompile(`%[-0-9.]*[sdrf]`)
	reManyV      = regexp.MustCompile(`(<V>)+`)
	reShellSinks = regexp.MustCompile(`(^|[\s;|&(])(die|fail\(|LLM_EP_ERROR=|pid_owner_reason=)`)
	rePSSinks    = regexp.MustCompile(`(Stop-Bench\s|\.Error\s*=|PidOwnerReason\s*=)`)
	reBenchSinks = regexp.MustCompile(`(^|[\s;|&(])(die|fail\()|Stop-Bench\s`)
)

func messageTemplate(lit literal) (string, bool) {
	t := strings.TrimSuffix(lit.text, `\n`)
	t = rePrintf.ReplaceAllString(t, "<V>")
	t = reManyV.ReplaceAllString(t, "<V>")
	switch {
	case strings.HasPrefix(t, "llm: ") || strings.HasPrefix(t, "bench: "):
	case reBenchSinks.MatchString(lit.lineText):
		t = "bench: " + t
	case reShellSinks.MatchString(lit.lineText) || rePSSinks.MatchString(lit.lineText):
		// LLM_EP_ERROR / .Error / the pid reason: printed with "llm: " put in
		// front by the caller when it is missing.
		if !strings.HasPrefix(t, "llm: ") {
			t = "llm: " + t
		}
	default:
		return "", false
	}
	if len(t) < 16 || !strings.Contains(strings.TrimPrefix(strings.TrimPrefix(t, "llm: "), "bench: "), " ") {
		return "", false
	}
	return t, true
}

// ---------------------------------------------------------------------------
// bash

func scanShell(src string) []literal {
	var out []literal
	lines := strings.Split(src, "\n")
	heredocEnd := ""
	reHeredoc := regexp.MustCompile(`<<-?\s*'?"?([A-Za-z_]+)'?"?`)
	reFail := regexp.MustCompile(`fail\(\s*"((?:[^"\\]|\\.)*)"`)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if heredocEnd != "" {
			if strings.TrimSpace(line) == heredocEnd {
				heredocEnd = ""
				continue
			}
			// The Python helper of the bench: its fail("…") messages only.
			for _, m := range reFail.FindAllStringSubmatch(line, -1) {
				out = append(out, literal{text: m[1], line: i + 1, lineText: line})
			}
			continue
		}
		code := line
		for j := 0; j < len(code); j++ {
			c := code[j]
			switch {
			case c == '#' && (j == 0 || strings.ContainsRune(" \t;|&(", rune(code[j-1]))):
				j = len(code)
			case c == '\\':
				j++
			case c == '\'':
				end := strings.IndexByte(code[j+1:], '\'')
				if end < 0 {
					j = len(code)
					break
				}
				out = append(out, literal{text: code[j+1 : j+1+end], line: i + 1, lineText: line})
				j += end + 1
			case c == '"':
				text, next := shellDouble(code, j+1)
				out = append(out, literal{text: text, line: i + 1, lineText: line})
				j = next
			}
		}
		if m := reHeredoc.FindStringSubmatch(stripShellStrings(line)); m != nil {
			heredocEnd = m[1]
		}
	}
	return out
}

// stripShellStrings hides quoted text, so that "<<" inside a message is not
// taken for a here-document.
func stripShellStrings(line string) string {
	var b strings.Builder
	quote := byte(0)
	for j := 0; j < len(line); j++ {
		c := line[j]
		switch {
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
			b.WriteByte(c)
		case quote != 0 && c == quote:
			quote = 0
			b.WriteByte(c)
		case quote == 0:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// shellDouble reads a double-quoted bash string starting after its quote and
// returns its template and the index of the closing quote.
func shellDouble(s string, i int) (string, int) {
	var b strings.Builder
	for i < len(s) {
		c := s[i]
		switch {
		case c == '"':
			return b.String(), i
		case c == '\\' && i+1 < len(s):
			n := s[i+1]
			if strings.IndexByte("\"\\$`", n) >= 0 {
				b.WriteByte(n)
			} else {
				b.WriteByte(c)
				b.WriteByte(n)
			}
			i += 2
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			i = skipBalanced(s, i+1, '(', ')')
			b.WriteString("<V>")
		case c == '$' && i+1 < len(s) && s[i+1] == '{':
			i = skipBalanced(s, i+1, '{', '}')
			b.WriteString("<V>")
		case c == '$' && i+1 < len(s) && (isNameByte(s[i+1]) || strings.IndexByte("*@#?!0123456789", s[i+1]) >= 0):
			i++
			if isNameByte(s[i]) {
				for i < len(s) && (isNameByte(s[i]) || (s[i] >= '0' && s[i] <= '9')) {
					i++
				}
			} else {
				i++
			}
			b.WriteString("<V>")
		case c == '`':
			end := strings.IndexByte(s[i+1:], '`')
			if end < 0 {
				return b.String(), len(s)
			}
			i += end + 2
			b.WriteString("<V>")
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), len(s)
}

// skipBalanced returns the index after the bracket that closes the one at i,
// stepping over quoted text inside.
func skipBalanced(s string, i int, open, close byte) int {
	depth := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\':
			i += 2
			continue
		case c == '\'' && open == '(':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return len(s)
			}
			i += end + 2
			continue
		case c == '"':
			_, next := shellDouble(s, i+1)
			i = next + 1
			continue
		case c == open:
			depth++
		case c == close:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return len(s)
}

func isNameByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// ---------------------------------------------------------------------------
// PowerShell

func scanPowerShell(src string) []literal {
	var out []literal
	lineOf := func(i int) (int, string) {
		start := strings.LastIndexByte(src[:i], '\n') + 1
		end := strings.IndexByte(src[i:], '\n')
		if end < 0 {
			end = len(src) - i
		}
		return strings.Count(src[:i], "\n") + 1, src[start : i+end]
	}
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '<' && i+1 < len(src) && src[i+1] == '#':
			end := strings.Index(src[i+2:], "#>")
			if end < 0 {
				return out
			}
			i += end + 3
		case c == '#':
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				return out
			}
			i += end
		case c == '\'':
			start := i
			var b strings.Builder
			i++
			for i < len(src) {
				if src[i] == '\'' {
					if i+1 < len(src) && src[i+1] == '\'' {
						b.WriteByte('\'')
						i += 2
						continue
					}
					break
				}
				b.WriteByte(src[i])
				i++
			}
			n, text := lineOf(start)
			out = append(out, literal{text: b.String(), line: n, lineText: text})
		case c == '"':
			start := i
			text, next := psDouble(src, i+1)
			n, lt := lineOf(start)
			out = append(out, literal{text: text, line: n, lineText: lt})
			i = next
		}
	}
	return out
}

func psDouble(s string, i int) (string, int) {
	var b strings.Builder
	for i < len(s) {
		c := s[i]
		switch {
		case c == '"':
			if i+1 < len(s) && s[i+1] == '"' {
				b.WriteByte('"')
				i += 2
				continue
			}
			return b.String(), i
		case c == '`' && i+1 < len(s):
			b.WriteByte(s[i+1])
			i += 2
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			i = psSkipSubexpression(s, i+1)
			b.WriteString("<V>")
		case c == '$' && i+1 < len(s) && s[i+1] == '{':
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return b.String(), len(s)
			}
			i += end + 1
			b.WriteString("<V>")
		case c == '$' && i+1 < len(s) && (isNameByte(s[i+1])):
			i++
			for i < len(s) && (isNameByte(s[i]) || (s[i] >= '0' && s[i] <= '9') || s[i] == ':') {
				i++
			}
			b.WriteString("<V>")
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), len(s)
}

func psSkipSubexpression(s string, i int) int {
	depth := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return len(s)
			}
			i += end + 2
			continue
		case c == '"':
			_, next := psDouble(s, i+1)
			i = next + 1
			continue
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return len(s)
}
