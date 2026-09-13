package main

// The table of the local address (T-450). The rule "is this address local, the
// cloud, or a configuration error" used to be written three times, three ways —
// the cloud gate of the platform, scripts/lib/llm-endpoint.* and rule 6 of
// compose-lint — and MV_OLLAMA_URL=http://ollama:11434 passed the linter and
// stopped the platform. The rule now has one table of cases,
// testdata/llm/local-endpoints.tsv, and every implementation is held to it:
// the Go test of IsLocalEndpoint reads it (EPIC-003, T-451), compose-lint calls
// the bash function, and this file runs both script halves over every row.
//
//   T01 — llm_endpoint_classify of llm-endpoint.sh answers every row as the
//         table does;
//   T02 — Get-LlmEndpointClass of LlmEndpoint.psm1 answers every row as the
//         table does, and gives the same kind (the reason in one word) and the
//         same error sentence as bash. The sentence is compared because the
//         kind of every refusal of the parser is `unparsed`: two halves that
//         run the same checks in a different order agree on class and kind and
//         tell the operator different things (T-450 review #1 Mi-3).
//
// Both halves are started once each, over all rows at once: a process per row
// would cost pwsh a second a row. The table is read from -repo, never from the
// mutated -source tree: it is the specification, not the code under test.
// Like the message pairs, the check runs whatever -run selects, so a mutant of
// the rule is caught even when no scenario is selected.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const endpointTablePath = "testdata/llm/local-endpoints.tsv"

// endpointTableMinRows is the floor the task set for the table (T-450 p. 1).
const endpointTableMinRows = 40

var endpointAnswers = []string{"local", "cloud", "invalid"}

// endpointStandCases are the cases the format of the table cannot hold — a URL
// with a space or with a character outside printable ASCII — run after the rows,
// by both halves, under the same checks. Go's url.Parse refuses a space in a host
// (T-450 review #1 Mi-2); a character outside printable ASCII is invalid anywhere
// in the value (C-15 v1.5, T-451). A case that can be written in the table
// belongs there, not here.
var endpointStandCases = []endpointCase{
	{url: "http://local host:8888", want: "invalid", reason: "a space in the host, which url.Parse refuses"},
	{url: "http://127.0.0.1 .evil.com:80", want: "invalid", reason: "a space after a loopback address, which url.Parse refuses"},
	{url: "http://\uff4c\uff4f\uff43\uff41\uff4c\uff48\uff4f\uff53\uff54:8080", want: "invalid", reason: "a full-width localhost is outside printable ASCII"},
	{url: "http://127\u30020\u30020\u30021:8080", want: "invalid", reason: "ideographic full stops in a dotted quad are outside printable ASCII"},
	{url: "http://localhost\u3002:8080", want: "invalid", reason: "an ideographic full stop as the root dot is outside printable ASCII"},
	{url: "http://local\u00adhost:8080", want: "invalid", reason: "a soft hyphen, invisible, is outside printable ASCII"},
	{url: "http://l\u0585calhost:8080", want: "invalid", reason: "an Armenian letter that looks like o is outside printable ASCII"},
}

type endpointCase struct {
	line   int
	url    string
	want   string
	reason string
}

// readEndpointTable parses the table and holds it to its own format: a table
// that silently lost its rows, or a class, would let every implementation pass.
func readEndpointTable(repo string) ([]endpointCase, error) {
	raw, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(endpointTablePath)))
	if err != nil {
		return nil, err
	}
	var (
		cases    []endpointCase
		problems []string
		header   bool
		seen     = map[string]int{}
		count    = map[string]int{}
	)
	for i, line := range strings.Split(string(raw), "\n") {
		n := i + 1
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if !header {
			header = true
			if strings.Join(fields, "\t") != "url\tanswer\treason" {
				problems = append(problems, fmt.Sprintf("line %d: the header must be url<TAB>answer<TAB>reason, got %q", n, line))
			}
			continue
		}
		if len(fields) != 3 {
			problems = append(problems, fmt.Sprintf("line %d: %d fields, want 3 separated by one TAB: %q", n, len(fields), line))
			continue
		}
		c := endpointCase{line: n, url: fields[0], want: fields[1], reason: fields[2]}
		switch {
		case c.url == "":
			problems = append(problems, fmt.Sprintf("line %d: the URL is empty", n))
			continue
		case !printableNoSpace(c.url):
			problems = append(problems, fmt.Sprintf("line %d: the URL %q holds a space or a character outside printable ASCII; the two halves read such a line differently", n, c.url))
			continue
		case !contains(endpointAnswers, c.want):
			problems = append(problems, fmt.Sprintf("line %d: answer %q is not one of %s", n, c.want, strings.Join(endpointAnswers, ", ")))
			continue
		case strings.TrimSpace(c.reason) == "":
			problems = append(problems, fmt.Sprintf("line %d: %s has no reason", n, c.url))
			continue
		}
		if prev, dup := seen[c.url]; dup {
			problems = append(problems, fmt.Sprintf("line %d: %s repeats line %d", n, c.url, prev))
			continue
		}
		seen[c.url] = n
		count[c.want]++
		cases = append(cases, c)
	}
	if !header {
		problems = append(problems, "no header and no cases")
	}
	if len(cases) < endpointTableMinRows {
		problems = append(problems, fmt.Sprintf("%d cases, the table must hold at least %d", len(cases), endpointTableMinRows))
	}
	for _, a := range endpointAnswers {
		if count[a] == 0 {
			problems = append(problems, fmt.Sprintf("no case answers %s", a))
		}
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "\n"))
	}
	return cases, nil
}

// where names the origin of a case: a line of the table, or the stand.
func (c endpointCase) where() string {
	if c.line == 0 {
		return "endpoints.go"
	}
	return fmt.Sprintf("line %d", c.line)
}

func printableNoSpace(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] <= 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// The drivers. Each reads one URL per line and prints
// "<class>\t<kind>\t<error>" per line; the error is empty unless the class is
// invalid, and no sentence of the rule holds a TAB or a line break. bash runs under `set -euo pipefail`, as compose-lint does: a helper that
// trips `set -u` or `set -e` there fails the table, not the linter later.
const bashEndpointDriver = `set -euo pipefail
. scripts/lib/llm-endpoint.sh
while IFS= read -r url || [ -n "$url" ]; do
  llm_endpoint_classify "$url"
  printf '%s\t%s\t%s\n' "$LLM_CLASS" "$LLM_CLASS_KIND" "$LLM_CLASS_ERROR"
done`

const pwshEndpointDriver = `$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Import-Module (Join-Path (Get-Location) 'scripts/lib/LlmEndpoint.psm1') -Force
$out = [System.Text.StringBuilder]::new()
foreach ($url in [System.IO.File]::ReadAllLines($env:MVPARITY_ENDPOINTS)) {
  $c = Get-LlmEndpointClass -Url $url
  [void]$out.Append($c.Class).Append("` + "`t" + `").Append($c.Kind).Append("` + "`t" + `").Append($c.Error).Append("` + "`n" + `")
}
[Console]::Out.Write($out.ToString())`

type endpointAnswer struct{ class, kind, err string }

func (s *stand) endpointTable(ctx context.Context) []lintResult {
	t01 := lintResult{id: "T01", title: "local-endpoints.tsv: llm_endpoint_classify of llm-endpoint.sh (T-450)"}
	t02 := lintResult{id: "T02", title: "local-endpoints.tsv: Get-LlmEndpointClass of LlmEndpoint.psm1, and the kind and the error bash gives (T-450)"}

	cases, err := readEndpointTable(s.opt.repo)
	if err != nil {
		note := fmt.Sprintf("%s: %v", endpointTablePath, err)
		t01.status, t01.notes = statusFail, strings.Split(note, "\n")
		t02.status, t02.notes = statusFail, t01.notes
		return []lintResult{t01, t02}
	}
	cases = append(cases, endpointStandCases...)
	input := make([]string, len(cases))
	for i, c := range cases {
		input[i] = c.url
	}
	inputText := strings.Join(input, "\n") + "\n"

	sh, errSh := s.runEndpointDriver(ctx, implSh, inputText, len(cases))
	judge(&t01, cases, sh, errSh, "bash")

	if s.pwsh == "" {
		t02.status = statusSkip
		t02.notes = []string{"pwsh is not on PATH: the PowerShell half of the table is not run"}
		return []lintResult{t01, t02}
	}
	ps, errPs := s.runEndpointDriver(ctx, implPS, inputText, len(cases))
	judge(&t02, cases, ps, errPs, "PowerShell")
	if errSh == nil && errPs == nil {
		for i, c := range cases {
			if sh[i].kind != ps[i].kind {
				t02.notes = append(t02.notes, fmt.Sprintf("%s: %s — kind %q in bash, %q in PowerShell", c.where(), c.url, sh[i].kind, ps[i].kind))
				t02.status = statusFail
			}
			if sh[i].err != ps[i].err {
				t02.notes = append(t02.notes, fmt.Sprintf("%s: %s — the error differs", c.where(), c.url),
					"  bash:       "+sh[i].err, "  PowerShell: "+ps[i].err)
				t02.status = statusFail
			}
		}
	}
	return []lintResult{t01, t02}
}

func judge(res *lintResult, cases []endpointCase, got []endpointAnswer, err error, who string) {
	if err != nil {
		res.status = statusFail
		res.notes = strings.Split(err.Error(), "\n")
		return
	}
	for i, c := range cases {
		if got[i].class != c.want {
			res.notes = append(res.notes, fmt.Sprintf("%s: %s — the table says %s (%s), %s says %s (kind %s)", c.where(), c.url, c.want, c.reason, who, got[i].class, got[i].kind))
		}
	}
	res.title = fmt.Sprintf("%s — %d cases (%d of the table, %d of the stand)", res.title, len(cases), len(cases)-len(endpointStandCases), len(endpointStandCases))
	res.status = statusPass
	if len(res.notes) > 0 {
		res.status = statusFail
	}
}

// endpointDriverBudget bounds one driver over the whole table. bash forks a few
// times per row in Git Bash (trim, lower case), and under the load of
// parity-mutants next to other work one pass has taken 40 s: 90 s turned a slow
// machine into a FAIL of T01 that read as a killed mutant (T-450 iteration 2).
const endpointDriverBudget = 5 * time.Minute

func (s *stand) runEndpointDriver(ctx context.Context, which impl, input string, rows int) ([]endpointAnswer, error) {
	runCtx, cancel := context.WithTimeout(ctx, endpointDriverBudget)
	defer cancel()
	var cmd *exec.Cmd
	env := append([]string(nil), s.baseEnv...)
	if which == implSh {
		cmd = exec.CommandContext(runCtx, s.bash, "-c", bashEndpointDriver)
		cmd.Stdin = strings.NewReader(input)
	} else {
		path := filepath.Join(s.scratch, "endpoints.txt")
		if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
			return nil, err
		}
		env = append(env, "MVPARITY_ENDPOINTS="+path)
		cmd = exec.CommandContext(runCtx, s.pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", pwshEndpointDriver)
	}
	cmd.Dir = s.opt.source
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			// On Windows a process killed at the deadline exits with status 1 and
			// says nothing, which read as a failure of the rule.
			return nil, fmt.Errorf("the %s driver did not finish within %s — the machine is slow, this is not a verdict of the rule; rerun the stand alone\n%s", which, endpointDriverBudget, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("the %s driver failed: %v\n%s", which, err, strings.TrimSpace(stderr.String()))
	}
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(stdout.String(), "\r\n", "\n"), "\n"), "\n")
	if len(lines) != rows {
		return nil, fmt.Errorf("the %s driver answered %d lines for %d cases\n%s", which, len(lines), rows, strings.TrimSpace(stderr.String()))
	}
	out := make([]endpointAnswer, rows)
	for i, line := range lines {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("the %s driver answered %q for case %d, not <class><TAB><kind><TAB><error>", which, line, i+1)
		}
		out[i] = endpointAnswer{class: fields[0], kind: fields[1], err: fields[2]}
	}
	return out, nil
}
