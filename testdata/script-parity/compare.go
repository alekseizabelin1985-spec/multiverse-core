package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	statusPass  = "PASS"
	statusFail  = "FAIL"
	statusKnown = "KNOWN-FAILING"
	statusFixed = "FIXED-BUT-MARKED"
	statusSkip  = "SKIP"
	// statusUncovered: an item of a known defect no scenario produces yet.
	statusUncovered = "UNCOVERED"
)

type result struct {
	status string
	notes  []string
	// seen: the items of known defects this scenario met (defect id -> item),
	// gathered over the whole run by knownSummary.
	seen map[string]map[string]bool
}

var (
	defaultServerChecks = []string{"exit", "stdout", "stderr", "argv", "requests", "models"}
	// json-types and streams run in every bench scenario: the findings that
	// belong to a known defect are set aside item by item (known.go), anything
	// else fails the scenario.
	defaultBenchChecks = []string{"exit", "messages", "streams", "table", "csv", "json-fields", "json-values", "json-types", "requests"}
)

func (s *stand) runScenario(ctx context.Context, sc Scenario) result {
	if s.pwsh == "" && sc.ParityOnly {
		return result{status: statusSkip, notes: []string{"needs pwsh: the scenario is a comparison of the two halves and has nothing to check in one"}}
	}
	if sc.Script == scriptBench && !s.havePython {
		return result{status: statusSkip, notes: []string{"python3 is not on PATH: the bash half of the bench uses it as its JSON tool"}}
	}

	impls := []impl{implSh}
	if s.pwsh != "" {
		impls = append(impls, implPS)
	}
	runs := make([]*implRun, len(impls))
	var wg sync.WaitGroup
	for i, which := range impls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					runs[i] = &implRun{impl: which, err: fmt.Errorf("panic: %v", r)}
				}
			}()
			runs[i] = s.runImpl(ctx, &sc, which)
		}()
	}
	wg.Wait()
	if ctx.Err() != nil {
		return result{status: statusFail, notes: []string{"interrupted"}}
	}

	var notes []string
	views := make([][]stepView, len(runs))
	for i, r := range runs {
		if r.err != nil {
			return result{status: statusFail, notes: []string{fmt.Sprintf("%s: the stand could not run the scenario: %v", r.impl, r.err)}}
		}
		n := s.normalizer(r)
		views[i] = make([]stepView, len(r.steps))
		for j := range r.steps {
			views[i][j] = buildView(&sc, sc.Steps[j], r, &r.steps[j], n)
		}
	}

	seen := map[string]map[string]bool{} // known defect -> items seen
	for j, step := range sc.Steps {
		label := fmt.Sprintf("step %d (%s)", j+1, step.Action)
		if j >= len(runs[0].steps) || (len(runs) == 2 && j >= len(runs[1].steps)) {
			notes = append(notes, label+": not run")
			break
		}
		for i, r := range runs {
			for _, msg := range checkExpect(step.Expect, r, &r.steps[j], views[i][j]) {
				notes = append(notes, fmt.Sprintf("%s %s: %s", r.impl, label, msg))
			}
		}
		if len(runs) == 2 {
			checks := defaultServerChecks
			if sc.Script == scriptBench {
				checks = defaultBenchChecks
			}
			for _, f := range compareViews(checks, views[0][j], views[1][j]) {
				if d, item := matchKnown(f); d != nil {
					if seen[d.id] == nil {
						seen[d.id] = map[string]bool{}
					}
					seen[d.id][item] = true
					continue
				}
				notes = append(notes, fmt.Sprintf("parity %s: %s", label, f.text))
			}
		}
	}

	if len(notes) > 0 {
		return result{status: statusFail, notes: notes, seen: seen}
	}
	if len(sc.Reports) == 0 {
		return result{status: statusPass, seen: seen}
	}
	var missing, present []string
	for _, id := range sortedKeys(sc.Reports) {
		for _, item := range sc.Reports[id] {
			if seen[id][item] {
				present = append(present, id+" "+item)
			} else {
				missing = append(missing, id+" "+item)
			}
		}
	}
	if len(missing) > 0 {
		return result{status: statusFixed, seen: seen, notes: []string{
			"these items of the known defect no longer differ — take them out of knownDefects and Scenario.Reports, and close the backlog line when none is left: " + strings.Join(missing, "; "),
		}}
	}
	return result{status: statusKnown, seen: seen, notes: []string{"every expected item differs, and nothing else does: " + strings.Join(present, "; ")}}
}

// ---------------------------------------------------------------------------
// normalisation: only what cannot repeat between two runs

type normalizer struct {
	pairs []string // old, new, old, new...
	port  string
	dead  string
}

var (
	reANSI     = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)
	rePID      = regexp.MustCompile(`\bpid \d+`)
	reFirst    = regexp.MustCompile(`first call \d+ ms`)
	reStamp    = regexp.MustCompile(`\b\d{8}-\d{4}\b`)
	reMarkPath = regexp.MustCompile(`(<DOUBLE>|<TREE>|<RUN>|<SCRATCH>)([^\s,;()'"]*)`)
	reDots     = regexp.MustCompile(`^\.+$`)
)

func (s *stand) normalizer(r *implRun) *normalizer {
	n := &normalizer{port: r.vars["PORT"], dead: r.vars["DEAD"]}
	add := func(path, marker string) {
		rel, err := filepath.Rel(s.scratch, path)
		variants := []string{path, filepath.ToSlash(path)}
		if err == nil && !strings.HasPrefix(rel, "..") {
			variants = append(variants, s.msysScratch+"/"+filepath.ToSlash(rel))
			// pwsh expands the 8.3 short names of %TEMP% (C:\Users\ABCD~1) into
			// the long form the file system keeps; the scripts print either.
			if s.longScratch != s.scratch {
				long := filepath.Join(s.longScratch, rel)
				variants = append(variants, long, filepath.ToSlash(long))
			}
		}
		if marker == "<SCRATCH>" {
			variants = append(variants, s.msysScratch)
		}
		for _, v := range variants {
			n.pairs = append(n.pairs, v, marker)
		}
	}
	exe := strings.TrimSuffix(s.doubleExe, ".exe")
	add(s.doubleExe, "<DOUBLE>")
	add(exe, "<DOUBLE>")
	add(r.tree, "<TREE>")
	add(r.run, "<RUN>")
	add(s.scratch, "<SCRATCH>")
	return n
}

func (n *normalizer) text(s string) string {
	s = reANSI.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for i := 0; i+1 < len(n.pairs); i += 2 {
		s = strings.ReplaceAll(s, n.pairs[i], n.pairs[i+1])
	}
	s = reMarkPath.ReplaceAllStringFunc(s, func(m string) string { return strings.ReplaceAll(m, `\`, "/") })
	// Each half names its own module when the module is missing (H54, B09).
	s = strings.ReplaceAll(s, "<TREE>/scripts/lib/llm-endpoint.sh", "<TREE>/scripts/lib/<MODULE>")
	s = strings.ReplaceAll(s, "<TREE>/scripts/lib/LlmEndpoint.psm1", "<TREE>/scripts/lib/<MODULE>")
	s = replaceNumber(s, n.port, "<PORT>")
	s = replaceNumber(s, n.dead, "<DEAD>")
	s = rePID.ReplaceAllString(s, "pid <PID>")
	s = reFirst.ReplaceAllString(s, "first call <MS> ms")
	s = reStamp.ReplaceAllString(s, "<STAMP>")
	return s
}

// replaceNumber replaces a port only where it stands as a whole number.
func replaceNumber(s, number, marker string) string {
	if number == "" {
		return s
	}
	var b strings.Builder
	for {
		i := strings.Index(s, number)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(number)
		before := i > 0 && s[i-1] >= '0' && s[i-1] <= '9'
		after := end < len(s) && s[end] >= '0' && s[end] <= '9'
		b.WriteString(s[:i])
		if before || after {
			b.WriteString(number)
		} else {
			b.WriteString(marker)
		}
		s = s[end:]
	}
}

func lines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// ---------------------------------------------------------------------------
// what one implementation did in one step, normalised

type stepView struct {
	exit     int
	timedOut bool
	stdout   []string
	stderr   []string
	raw      string
	argv     [][]string
	requests []string
	models   []string
	bench    *benchView
}

type benchView struct {
	err       string
	csvName   string
	csvLines  []string   // normalised, volatile columns masked
	csvRows   [][]string // as written, for the expectations
	csvHeader []string
	table     []string
	cache     []string
	messages  []string // both streams, sorted
	outLines  []string // stdout without the table, in order
	errLines  []string // stderr, in order
	reports   map[string]map[string]any
}

func buildView(sc *Scenario, step Step, r *implRun, sr *stepRun, n *normalizer) stepView {
	v := stepView{
		exit:     sr.exit,
		timedOut: sr.timedOut,
		stdout:   lines(n.text(sr.stdout)),
		stderr:   lines(n.text(sr.stderr)),
		raw:      sr.stdout + sr.stderr,
		models:   sr.models,
	}
	for _, argv := range sr.launches {
		norm := make([]string, len(argv))
		for i, a := range argv {
			norm[i] = n.text(a)
		}
		v.argv = append(v.argv, norm)
	}
	for _, rec := range sr.requests {
		line := fmt.Sprintf("%s %s", rec.Method, n.text(rec.Path))
		if rec.Auth != "" {
			line += " auth=" + strconv.Quote(n.text(rec.Auth))
		}
		if rec.Path == "/v1/chat/completions" {
			line += fmt.Sprintf(" model=%q max_tokens=%d bad_json=%v", rec.Model, rec.MaxTokens, rec.BadJSON)
		}
		if len(v.requests) == 0 || v.requests[len(v.requests)-1] != line {
			v.requests = append(v.requests, line)
		}
	}
	if sc.Script == scriptBench {
		v.bench = readBench(step, r, sr, n)
	}
	return v
}

var phases = map[string]bool{"PHASE": true, "tick": true, "phase2": true, "phase2-group3": true}

func isTableLine(line string) bool {
	f := strings.Fields(line)
	return len(f) >= 10 && phases[f[1]]
}

func readBench(step Step, r *implRun, sr *stepRun, n *normalizer) *benchView {
	b := &benchView{reports: map[string]map[string]any{}}
	for _, line := range lines(n.text(sr.stdout)) {
		switch {
		case isTableLine(line):
			f := strings.Fields(line)
			if f[0] != "CFG" {
				f[3], f[4] = "<MS>", "<MS>"
				b.cache = append(b.cache, f[8])
			}
			b.table = append(b.table, strings.Join(f, " "))
		case strings.TrimSpace(line) == "" || reDots.MatchString(strings.TrimSpace(line)):
		default:
			b.outLines = append(b.outLines, line)
		}
	}
	for _, line := range lines(n.text(sr.stderr)) {
		if strings.TrimSpace(line) == "" || reDots.MatchString(strings.TrimSpace(line)) {
			continue
		}
		b.errLines = append(b.errLines, line)
	}
	b.messages = append(append([]string(nil), b.outLines...), b.errLines...)
	sort.Strings(b.messages)

	outDir := filepath.Join(r.tree, "ops", "metrics")
	if step.Bench != nil && step.Bench.OutDir != "" {
		outDir = expand(step.Bench.OutDir, r.vars)
	}
	entries, _ := os.ReadDir(outDir)
	reReport := regexp.MustCompile(`^bench-.+-\d{8}-\d{4}\.json$`)
	reCSV := regexp.MustCompile(`^bench-\d{8}-\d{4}\.csv$`)
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(outDir, name)
		switch {
		case reCSV.MatchString(name):
			raw, err := os.ReadFile(path)
			if err != nil {
				b.err = err.Error()
				continue
			}
			b.csvName = n.text(name)
			b.readCSV(string(raw))
		case reReport.MatchString(name):
			raw, err := os.ReadFile(path)
			if err != nil {
				b.err = err.Error()
				continue
			}
			var doc map[string]any
			text := n.text(strings.TrimPrefix(string(raw), string(rune(0xFEFF))))
			if err := json.Unmarshal([]byte(text), &doc); err != nil {
				b.err = fmt.Sprintf("%s is not JSON: %v", name, err)
				continue
			}
			b.reports[n.text(name)] = doc
		}
	}
	return b
}

var volatileColumns = map[string]bool{"p50_ms": true, "p95_ms": true, "started_at": true}

func (b *benchView) readCSV(raw string) {
	// Line ends are kept: CRLF against LF is a divergence of the file (T-404).
	all := strings.Split(strings.TrimSuffix(raw, "\n"), "\n")
	if len(all) == 0 {
		return
	}
	b.csvHeader = strings.Split(all[0], ",")
	b.csvLines = append(b.csvLines, all[0])
	for _, line := range all[1:] {
		cells := strings.Split(line, ",")
		b.csvRows = append(b.csvRows, cells)
		masked := append([]string(nil), cells...)
		for i, name := range b.csvHeader {
			if i >= len(masked) {
				break
			}
			switch {
			case volatileColumns[name]:
				masked[i] = "<x>"
			case name == "first_call_ms" && masked[i] != "":
				masked[i] = "<set>"
			}
		}
		b.csvLines = append(b.csvLines, strings.Join(masked, ","))
	}
}

// ---------------------------------------------------------------------------
// expectations

func checkExpect(e Expect, r *implRun, sr *stepRun, v stepView) []string {
	var out []string
	if v.timedOut {
		out = append(out, "timed out")
	}
	if v.exit != e.Exit {
		out = append(out, fmt.Sprintf("exit %d, want %d%s", v.exit, e.Exit, tail(v)))
	}
	joined := strings.Join(append(append([]string(nil), v.stdout...), v.stderr...), "\n")
	for _, want := range e.Contains {
		if !strings.Contains(joined, want) {
			out = append(out, fmt.Sprintf("output lacks %q%s", want, tail(v)))
		}
	}
	errJoined := strings.Join(v.stderr, "\n")
	for _, want := range e.Stderr {
		if !strings.Contains(errJoined, want) {
			out = append(out, fmt.Sprintf("stderr lacks %q", want))
		}
	}
	for _, bad := range e.Absent {
		if strings.Contains(v.raw, bad) {
			out = append(out, fmt.Sprintf("output contains %q, which must never be printed", bad))
		}
	}
	starts := nonVersion(v.argv)
	if e.NoLaunch && len(starts) > 0 {
		out = append(out, fmt.Sprintf("llama-server was started: %q", starts[0]))
	}
	if e.Argv != nil {
		switch {
		case len(starts) != 1:
			out = append(out, fmt.Sprintf("%d starts of llama-server, want 1", len(starts)))
		case !slices.Equal(starts[0], e.Argv):
			got, want := argvDiff(starts[0], e.Argv)
			out = append(out, fmt.Sprintf("argv %s\n  want %s", got, want))
		}
	}
	if e.Models != nil && !slices.Equal(v.models, e.Models) {
		out = append(out, fmt.Sprintf("/v1/models reports %q, want %q", v.models, e.Models))
	}
	if e.PortFree && sr.answers {
		out = append(out, "something still answers on the port of the server")
	}
	if e.NoFile != "" {
		if _, err := os.Stat(filepath.Join(r.tree, filepath.FromSlash(e.NoFile))); err == nil {
			out = append(out, e.NoFile+" still exists")
		}
	}
	if e.Requests != nil {
		var got []string
		for _, rq := range v.requests {
			f := strings.Fields(rq)
			line := f[0] + " " + f[1]
			if len(got) == 0 || got[len(got)-1] != line {
				got = append(got, line)
			}
		}
		if !slices.Equal(got, e.Requests) {
			out = append(out, fmt.Sprintf("requests %q, want %q", got, e.Requests))
		}
	}
	if e.Auth != "" {
		want := expand(e.Auth, r.vars)
		for _, rec := range sr.requests {
			if rec.Auth != want {
				out = append(out, fmt.Sprintf("%s %s carried Authorization %q, want %q", rec.Method, rec.Path, rec.Auth, want))
				break
			}
		}
	}
	if e.Bench != nil {
		out = append(out, checkBench(*e.Bench, r, v.bench)...)
	}
	return out
}

func tail(v stepView) string {
	all := append(append([]string(nil), v.stdout...), v.stderr...)
	if len(all) > 6 {
		all = all[len(all)-6:]
	}
	if len(all) == 0 {
		return " (no output)"
	}
	return "\n  | " + strings.Join(all, "\n  | ")
}

func nonVersion(argv [][]string) [][]string {
	var out [][]string
	for _, a := range argv {
		if len(a) == 1 && a[0] == "--version" {
			continue
		}
		out = append(out, a)
	}
	return out
}

func checkBench(e BenchExpect, r *implRun, b *benchView) []string {
	var out []string
	if b == nil {
		return []string{"no bench output was read"}
	}
	if b.err != "" {
		out = append(out, b.err)
	}
	if len(b.csvRows) != e.Rows {
		out = append(out, fmt.Sprintf("%d CSV rows, want %d", len(b.csvRows), e.Rows))
	}
	for _, col := range sortedKeys(e.Columns) {
		want := e.Columns[col]
		idx := slices.Index(b.csvHeader, col)
		if idx < 0 {
			out = append(out, "the CSV has no column "+col)
			continue
		}
		for i, row := range b.csvRows {
			got := ""
			if idx < len(row) {
				got = row[idx]
			}
			if (want == "<set>" && got == "") || (want != "<set>" && got != want) {
				out = append(out, fmt.Sprintf("CSV row %d %s = %q, want %q (row has %d columns)", i+1, col, got, want, len(row)))
			}
		}
	}
	if e.Cache != "" {
		if len(b.cache) == 0 {
			out = append(out, "the table has no rows")
		}
		for i, c := range b.cache {
			if c != e.Cache {
				out = append(out, fmt.Sprintf("table row %d CACHE = %s, want %s", i+1, c, e.Cache))
			}
		}
	}
	if e.Reports != len(b.reports) {
		out = append(out, fmt.Sprintf("%d JSON reports, want %d", len(b.reports), e.Reports))
	}
	for _, name := range sortedKeys(b.reports) {
		meta, _ := b.reports[name]["meta"].(map[string]any)
		for _, key := range sortedKeys(e.Meta) {
			if got := normValue(meta[key]); got != e.Meta[key] {
				out = append(out, fmt.Sprintf("%s meta.%s = %s, want %s", name, key, got, e.Meta[key]))
			}
		}
		for _, key := range sortedKeys(e.MetaFile) {
			sum, err := fileSHA256(expand(e.MetaFile[key], r.vars))
			if err != nil {
				out = append(out, err.Error())
				continue
			}
			if got := normValue(meta[key]); got != sum {
				out = append(out, fmt.Sprintf("%s meta.%s = %s, want the sha256 of the file, %s", name, key, got, sum))
			}
		}
		if e.PromptMS != "" {
			numbers, nulls := 0, 0
			reqs, _ := b.reports[name]["requests"].([]any)
			for _, rq := range reqs {
				m, _ := rq.(map[string]any)
				if _, ok := m["prompt_ms"]; !ok {
					continue
				}
				if m["prompt_ms"] == nil {
					nulls++
				} else {
					numbers++
				}
			}
			switch e.PromptMS {
			case "mixed":
				if numbers == 0 || nulls == 0 {
					out = append(out, fmt.Sprintf("%s requests[].prompt_ms: %d numbers and %d nulls, want both", name, numbers, nulls))
				}
			case "null":
				if numbers != 0 || nulls == 0 {
					out = append(out, fmt.Sprintf("%s requests[].prompt_ms: %d numbers and %d nulls, want only nulls", name, numbers, nulls))
				}
			}
		}
	}
	return out
}

func fileSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ---------------------------------------------------------------------------
// parity

func compareViews(checks []string, a, b stepView) []finding {
	var out []finding
	add := func(check string, texts ...string) {
		for _, t := range texts {
			out = append(out, finding{check: check, key: t, text: t})
		}
	}
	for _, check := range checks {
		switch check {
		case "exit":
			if a.exit != b.exit {
				add(check, fmt.Sprintf("exit sh %d, ps1 %d", a.exit, b.exit))
			}
		case "stdout":
			add(check, diffLines("stdout", a.stdout, b.stdout)...)
		case "stderr":
			add(check, diffLines("stderr", a.stderr, b.stderr)...)
		case "argv":
			if !slices.EqualFunc(a.argv, b.argv, slices.Equal) {
				if len(a.argv) != len(b.argv) {
					add(check, fmt.Sprintf("llama-server started %d times by sh, %d times by ps1", len(a.argv), len(b.argv)))
					break
				}
				for k := range a.argv {
					if !slices.Equal(a.argv[k], b.argv[k]) {
						x, y := argvDiff(a.argv[k], b.argv[k])
						add(check, fmt.Sprintf("argv of start %d of llama-server\n  sh  %s\n  ps1 %s", k+1, x, y))
					}
				}
			}
		case "requests":
			add(check, diffLines("requests to the double", a.requests, b.requests)...)
		case "models":
			if !slices.Equal(a.models, b.models) {
				add(check, fmt.Sprintf("/v1/models after the step: sh %q, ps1 %q", a.models, b.models))
			}
		case "messages":
			add(check, diffLines("messages (both streams, sorted)", a.bench.messages, b.bench.messages)...)
		case "streams":
			out = append(out, streamFindings(a.bench, b.bench)...)
		case "table":
			add(check, diffLines("table", a.bench.table, b.bench.table)...)
		case "csv":
			if a.bench.csvName != b.bench.csvName {
				add(check, fmt.Sprintf("CSV file sh %q, ps1 %q", a.bench.csvName, b.bench.csvName))
			}
			add(check, diffLines("CSV", a.bench.csvLines, b.bench.csvLines)...)
		case "json-fields":
			add(check, diffLines("JSON fields", jsonShape(a.bench), jsonShape(b.bench))...)
		case "json-values":
			add(check, diffLines("JSON values", jsonValues(a.bench), jsonValues(b.bench))...)
		case "json-types":
			out = append(out, typeFindings(a.bench, b.bench)...)
		default:
			add(check, "unknown check "+check)
		}
	}
	return out
}

func diffLines(what string, a, b []string) []string {
	if slices.Equal(a, b) {
		return nil
	}
	n := max(len(a), len(b))
	for i := 0; i < n; i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		} else {
			x = "<none>"
		}
		if i < len(b) {
			y = b[i]
		} else {
			y = "<none>"
		}
		if x != y {
			return []string{fmt.Sprintf("%s differ at line %d of %d/%d\n  sh  %s\n  ps1 %s", what, i+1, len(a), len(b), x, y)}
		}
	}
	return nil
}

// jsonShape lists every field path of the reports. The kinds of their values
// are compared by typeFindings (known.go), path by path.
func jsonShape(b *benchView) []string {
	var out []string
	for _, name := range sortedKeys(b.reports) {
		walkJSON(name, b.reports[name], func(path string, _ any) {
			out = append(out, path)
		})
	}
	sort.Strings(out)
	return slices.Compact(out)
}

func walkJSON(prefix string, v any, visit func(string, any)) {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range sortedKeys(x) {
			walkJSON(prefix+"."+k, x[k], visit)
		}
	case []any:
		for _, item := range x {
			walkJSON(prefix+"[]", item, visit)
		}
	default:
		visit(prefix, v)
	}
}

func kind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	}
	return fmt.Sprintf("%T", v)
}

// Fields of the report that differ between two runs by nature, or name the
// implementation that wrote them.
var volatileJSON = map[string]bool{
	"generated_by": true, "meta.script": true, "meta.started_at": true,
	"cells[].p50_ms": true, "cells[].p95_ms": true, "cells[].started_at": true,
	"requests[].latency_ms": true,
}

func jsonValues(b *benchView) []string {
	var out []string
	for _, name := range sortedKeys(b.reports) {
		doc := b.reports[name]
		var walk func(path, show string, v any)
		walk = func(path, show string, v any) {
			switch x := v.(type) {
			case map[string]any:
				for _, k := range sortedKeys(x) {
					walk(path+"."+k, show+"."+k, x[k])
				}
			case []any:
				for i, item := range x {
					walk(path+"[]", fmt.Sprintf("%s[%d]", show, i), item)
				}
			default:
				key := strings.TrimPrefix(path, name+".")
				switch {
				case volatileJSON[key]:
				case strings.HasSuffix(key, "first_call_ms") && v != nil:
					out = append(out, show+" = <set>")
				default:
					out = append(out, show+" = "+normValue(v))
				}
			}
		}
		walk(name, name, doc)
	}
	return out
}

// normValue compares the content of a value, not its JSON type: "8192" and
// 8192 are the same number here, and json-types is the check that tells them
// apart.
func normValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil && strings.TrimSpace(x) == x && x != "" {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
		return x
	case bool:
		return strconv.FormatBool(x)
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

// argvDiff shows two argument vectors from a little before the first place
// they differ: the fixed head of the command line is the same in every
// scenario and says nothing.
func argvDiff(a, b []string) (string, string) {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	from := max(0, i-2)
	show := func(v []string) string {
		if from >= len(v) {
			return fmt.Sprintf("(%d arguments) … <end>", len(v))
		}
		return fmt.Sprintf("(%d arguments) …%q", len(v), v[from:])
	}
	return show(a), show(b)
}
