package main

// Known defects, item by item.
//
// Review #1 of T-405 (M-1) found that a scenario marked known-failing used to
// swallow its whole class of checks: any difference of JSON types, or of the
// streams of the bench, turned into KNOWN-FAILING, and a new regression of the
// same class passed. So the mark is no longer on a scenario but on the exact
// differences a defect is made of:
//
//   - a check that can carry a known defect ("streams", "json-types") reports
//     every difference as its own finding, with a key;
//   - a finding whose key matches an item of a known defect is set aside; any
//     other finding of the same check fails the scenario as before — in EVERY
//     bench scenario, not only in the one that reports the defect;
//   - the scenario that reports a defect (Scenario.Reports) names the items it
//     must see. Every item seen and nothing else: KNOWN-FAILING. An item gone:
//     FIXED-BUT-MARKED, and the item comes out of this list. A finding outside
//     the items: FAIL.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type knownItem struct {
	name string
	re   *regexp.Regexp
	// uncovered: no scenario of the stand produces this line yet (backlog of
	// T-405). knownSummary reports it as UNCOVERED instead of FIXED-BUT-MARKED,
	// and fails the run if a scenario does produce it — the mark must go then.
	uncovered bool
}

type knownDefect struct {
	id      string
	check   string
	backlog string
	items   []knownItem
}

// finding is one difference between the two halves.
type finding struct {
	check string
	key   string // what an item of a known defect is matched against
	text  string // what is printed
}

func exact(name, key string) knownItem {
	return knownItem{name: name, re: regexp.MustCompile("^" + regexp.QuoteMeta(key) + "$")}
}

func pattern(name, re string) knownItem {
	return knownItem{name: name, re: regexp.MustCompile(re)}
}

// movedToStdout is the shape of every item of K01: bash prints the line to
// stderr, PowerShell prints it to stdout with Write-Host.
func movedToStdout(name, message string) knownItem {
	return pattern(name, "^"+message+" — sh stderr, ps1 stdout$")
}

func withoutScenario(it knownItem) knownItem {
	it.uncovered = true
	return it
}

var knownDefects = []knownDefect{
	{
		id:    "K01",
		check: "streams",
		backlog: "T-434 card, executor backlog p. 2 (backlog of T-405): llm-bench.sh writes its diagnostic lines to stderr, " +
			"llm-bench.ps1 to stdout with Write-Host; the rule of the streams is decided at the acceptance of T-405",
		// Every Write-Host line of llm-bench.ps1 that the bash twin writes to
		// stderr. The warm-up line and the missing module are stderr in both and
		// are NOT here: moving either of them is a new defect.
		items: []knownItem{
			movedToStdout("prompts", `bench: prompts <TREE>/testdata/bench/prompts\.jsonl, matrix .+`),
			movedToStdout("build", `bench: \S+ build \S+`),
			withoutScenario(movedToStdout("loading", `bench: .+/health = 503 \(model loading\), waited \d+s`)),
			movedToStdout("skipped phase", `bench: model .+ is not in .+/v1/models — phase \S+ skipped \(server mode .*\?\)`),
			withoutScenario(movedToStdout("no usable answer", `bench: \S+/\S+ produced no usable answer \(\d+ errors\) — no row written`)),
			withoutScenario(movedToStdout("failed requests", `bench: \S+/\S+ had \d+ failed requests`)),
			movedToStdout("measured nothing", `bench: \S+ measured nothing — no report written`),
			movedToStdout("report", `bench: \S*bench-\S+-<STAMP>\.json`),
			movedToStdout("csv", `bench: \S*bench-<STAMP>\.csv \(\d+ rows\)`),
			movedToStdout("one run", `bench: this is ONE run\. .+`),
			movedToStdout("nothing measured", `bench: nothing was measured — every phase was skipped or failed; see the messages above`),
			movedToStdout("no answer", `bench: no answer from .+ — the LLM process is not running\..+`),
			movedToStdout("unknown configuration", `bench: configuration \S+ is not in .+ \(have: .+\)`),
			withoutScenario(movedToStdout("not an endpoint", `bench: .+ answered \d+ — this is not a llama-server / OpenAI-compatible endpoint .+`)),
			withoutScenario(movedToStdout("still loading", `bench: .+/health is still 503 after 180 s .+`)),
			withoutScenario(movedToStdout("endpoint variable", `bench: .+ \(configuration \S+ expects the endpoint there; .+\)`)),
		},
	},
	{
		id:    "K02",
		check: "json-types",
		backlog: "T-434 backlog in tasks.md: the JSON report of llm-bench.sh writes meta as strings and requests[].http as \"200\", " +
			"llm-bench.ps1 writes numbers — and the other way round for cells[].vram_used_mb; changing it rewrites the format of existing reports",
		// Found by running the reports of both halves side by side (review #1
		// of T-405 counted seven). vram_used_mb is stable only because the stand
		// puts its own nvidia-smi first on PATH: without it the value is empty
		// on a machine without a GPU and the type follows the machine.
		items: []knownItem{
			exact("meta.num_ctx", "meta.num_ctx: sh string, ps1 number"),
			exact("meta.n_per_cell", "meta.n_per_cell: sh string, ps1 number"),
			exact("meta.repeats", "meta.repeats: sh string, ps1 number"),
			exact("meta.runs_required", "meta.runs_required: sh string, ps1 number"),
			exact("meta.first_call_ms", "meta.first_call_ms: sh string, ps1 number"),
			exact("requests[].http", "requests[].http: sh string, ps1 number"),
			exact("cells[].vram_used_mb", "cells[].vram_used_mb: sh number, ps1 string"),
		},
	},
}

func defectByID(id string) *knownDefect {
	for i := range knownDefects {
		if knownDefects[i].id == id {
			return &knownDefects[i]
		}
	}
	return nil
}

// matchKnown returns the defect and the item a finding belongs to.
func matchKnown(f finding) (*knownDefect, string) {
	for i := range knownDefects {
		d := &knownDefects[i]
		if d.check != f.check {
			continue
		}
		for _, it := range d.items {
			if it.re.MatchString(f.key) {
				return d, it.name
			}
		}
	}
	return nil, ""
}

// streamFindings: for every message line, which stream it went to in each
// half, and — among the lines that went to the same stream in both — whether
// they came in the same order.
func streamFindings(a, b *benchView) []finding {
	count := func(v *benchView) map[string][2]int {
		m := map[string][2]int{}
		for _, l := range v.outLines {
			c := m[l]
			c[0]++
			m[l] = c
		}
		for _, l := range v.errLines {
			c := m[l]
			c[1]++
			m[l] = c
		}
		return m
	}
	describe := func(c [2]int) string {
		var parts []string
		for i, name := range []string{"stdout", "stderr"} {
			switch {
			case c[i] == 1:
				parts = append(parts, name)
			case c[i] > 1:
				parts = append(parts, fmt.Sprintf("%s×%d", name, c[i]))
			}
		}
		if len(parts) == 0 {
			return "none"
		}
		return strings.Join(parts, "+")
	}
	ca, cb := count(a), count(b)
	lines := map[string]bool{}
	for l := range ca {
		lines[l] = true
	}
	for l := range cb {
		lines[l] = true
	}
	sorted := make([]string, 0, len(lines))
	for l := range lines {
		sorted = append(sorted, l)
	}
	sort.Strings(sorted)

	var out []finding
	same := map[string]bool{}
	for _, l := range sorted {
		if ca[l] == cb[l] {
			same[l] = true
			continue
		}
		key := fmt.Sprintf("%s — sh %s, ps1 %s", l, describe(ca[l]), describe(cb[l]))
		out = append(out, finding{check: "streams", key: key, text: "stream of a message: " + key})
	}
	keep := func(ls []string) []string {
		var r []string
		for _, l := range ls {
			if same[l] {
				r = append(r, l)
			}
		}
		return r
	}
	for _, s := range []struct {
		name string
		x, y []string
	}{{"stdout", keep(a.outLines), keep(b.outLines)}, {"stderr", keep(a.errLines), keep(b.errLines)}} {
		for _, t := range diffLines("order of the messages in "+s.name, s.x, s.y) {
			out = append(out, finding{check: "streams", key: "order " + s.name, text: t})
		}
	}
	return out
}

// typeFindings: for every field path present in both reports, the set of JSON
// kinds its values take in each half.
func typeFindings(a, b *benchView) []finding {
	kinds := func(v *benchView) map[string]string {
		sets := map[string]map[string]bool{}
		for _, name := range sortedKeys(v.reports) {
			walkJSON("", v.reports[name], func(path string, val any) {
				path = strings.TrimPrefix(path, ".")
				if sets[path] == nil {
					sets[path] = map[string]bool{}
				}
				sets[path][kind(val)] = true
			})
		}
		out := map[string]string{}
		for p, set := range sets {
			var ks []string
			for k := range set {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			out[p] = strings.Join(ks, "|")
		}
		return out
	}
	ka, kb := kinds(a), kinds(b)
	var out []finding
	for _, p := range sortedKeys(ka) {
		kbp, ok := kb[p]
		if !ok || ka[p] == kbp {
			continue
		}
		key := fmt.Sprintf("%s: sh %s, ps1 %s", p, ka[p], kbp)
		out = append(out, finding{check: "json-types", key: key, text: "JSON type: " + key})
	}
	return out
}

// knownSummary looks at the whole run (review #2 of T-405, Mi-1). An item of a
// known defect is set aside wherever it shows up, and only the report scenarios
// K01/K02 used to notice when one of THEIR items stopped differing: an item seen
// only in, say, B07 could be fixed and keep masking its line forever. Here every
// item is checked against the union of all scenarios:
//
//   - not seen anywhere, and a scenario exists for it: FIXED-BUT-MARKED, and the
//     run exits 1 — take the item out of knownDefects (and out of Reports);
//   - not seen, and marked withoutScenario: UNCOVERED, printed and not failing —
//     the backlog owes it a scenario;
//   - marked withoutScenario but seen: FAIL — the mark is stale.
//
// It only speaks for a run that could have seen every item: both halves, and
// every bench scenario selected. A partial run says that it cannot judge.
func knownSummary(selected []Scenario, results []result, havePwsh bool) (lines []string, fixed, failed int) {
	all := true
	for _, sc := range allScenarios() {
		if sc.Script != scriptBench {
			continue
		}
		found := false
		for _, s := range selected {
			if s.ID == sc.ID {
				found = true
				break
			}
		}
		if !found {
			all = false
			break
		}
	}
	if !havePwsh || !all {
		return []string{"known defects: not judged across the run — it needs both halves and every bench scenario (B*, K*)"}, 0, 0
	}
	union := map[string]map[string]bool{}
	for _, r := range results {
		if r.status == statusSkip {
			return []string{"known defects: not judged across the run — a bench scenario was skipped"}, 0, 0
		}
		for id, items := range r.seen {
			if union[id] == nil {
				union[id] = map[string]bool{}
			}
			for it := range items {
				union[id][it] = true
			}
		}
	}
	for _, d := range knownDefects {
		seen := 0
		for _, it := range d.items {
			switch {
			case union[d.id][it.name] && it.uncovered:
				failed++
				lines = append(lines, fmt.Sprintf("%-14s %s  %s: seen in this run although marked withoutScenario — remove the mark", statusFail, d.id, it.name))
			case union[d.id][it.name]:
				seen++
			case it.uncovered:
				lines = append(lines, fmt.Sprintf("%-14s %s  %s: no scenario produces this line yet (backlog of T-405)", statusUncovered, d.id, it.name))
			default:
				fixed++
				lines = append(lines, fmt.Sprintf("%-14s %s  %s: no scenario of this run shows it any more — take it out of knownDefects and Reports", statusFixed, d.id, it.name))
			}
		}
		lines = append(lines, fmt.Sprintf("known defect %s: %d of %d items seen in this run", d.id, seen, len(d.items)))
	}
	return lines, fixed, failed
}
