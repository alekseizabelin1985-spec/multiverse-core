// Command script-parity is the parity stand of the two implementations of the
// LLM scripts: scripts/llm-server.{sh,ps1} and scripts/llm-bench.{sh,ps1},
// with their modules scripts/lib/llm-endpoint.sh and LlmEndpoint.psm1 (T-405).
//
// Why it exists. The pair is one rule written twice, and the property "both
// halves say and do the same thing on the same input" broke twice inside one
// task (T-404 review #1 found 10 divergences on 24 inputs, opposite exit codes
// among them). Every later task rebuilt a stand of its own in a scratch
// directory and threw it away, so between tasks nothing held the property.
// This program is that stand, kept in the repository and called by
// `make scripts-parity`, `make ci` and the CI job scripts-parity;
// `make parity-mutants` (flag -mutants) checks the stand itself.
//
// What it compares, per scenario, after normalising only what cannot repeat
// (ports, pids, milliseconds, the path of the scratch copy): exit codes, stdout
// and stderr line by line, the argument vector the script hands to llama-server
// (recorded by a double that stands in for it), the requests a double server
// received, and for the bench the CSV, the table, the stream and order of every
// message, and the fields, values and value types of the JSON report. Scenarios
// also carry absolute expectations, so that two implementations that are wrong
// in the same way do not pass. Known defects are marked difference by
// difference (known.go), never a whole scenario or a whole class of checks.
//
// Where it lives, and why testdata/: the go tool skips directories named
// testdata in every ./... pattern, so this program is not part of the module's
// build, vet, lint, vulnerability scan or coverage, and none of the replay
// rules of the platform (shared/clock, shared/env) apply to a tool that has to
// read the wall clock and hand environments to child processes. It is started
// explicitly: go run ./testdata/script-parity. scripts/lib/ is not the place:
// it holds what the production scripts source, and a test double next to the
// one rule of the address would ship with it.
//
// What it needs: Go, bash with curl and python3 (the bash half of the bench
// uses python3 as its JSON tool), and pwsh 7 for the PowerShell half. No
// Docker, no network beyond 127.0.0.1 and ::1, no real llama-server. Without
// pwsh the stand runs the bash half against the absolute expectations and
// compares the message texts of the two files statically; it says so in its
// first lines and marks every comparison it could not make as SKIP.
//
// Exit codes: 0 — every scenario passed, the reports of known defects saw
// exactly their items (KNOWN-FAILING); 1 — a scenario failed, or an item of a
// known defect no longer differs (FIXED-BUT-MARKED: the item must go); 2 — the
// stand itself could not run, or was interrupted.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

func main() {
	// The same binary is the llama-server double: the scripts start it through
	// MV_LLM_BIN, and the role comes with the environment they inherit.
	// It is also nvidia-smi, under that name, first on the PATH of every script.
	// The name is checked first: the scripts hand the role of the llama-server
	// double down to every child, the GPU tool included.
	if strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe") == gpuToolName {
		os.Exit(runGPUTool(os.Args[1:]))
	}
	if os.Getenv(envRole) == roleDouble {
		os.Exit(runDouble(os.Args[1:]))
	}
	os.Exit(run())
}

type options struct {
	repo    string
	source  string
	jobs    int
	filter  *regexp.Regexp
	pwsh    string
	keep    bool
	mutants bool
	verbose bool
}

func run() int {
	var opt options
	var filter string
	flag.StringVar(&opt.repo, "repo", "", "root of the repository (default: the nearest directory with go.mod above the working directory)")
	flag.StringVar(&opt.source, "source", "", "tree to take the scripts from (default: -repo); the mutant mode passes a mutated copy")
	flag.IntVar(&opt.jobs, "jobs", defaultJobs(), "scenarios run in parallel")
	flag.StringVar(&filter, "run", "", "only scenarios whose id or title matches this regular expression")
	flag.StringVar(&opt.pwsh, "pwsh", "auto", "auto: use pwsh when it is on PATH; require: fail without it; off: bash half only")
	flag.BoolVar(&opt.keep, "keep", false, "keep the scratch directory (its path is printed)")
	flag.BoolVar(&opt.mutants, "mutants", false, "run the control mutants instead: every one of them must turn the stand red")
	flag.BoolVar(&opt.verbose, "v", false, "print the normalised output of failing scenarios in full")
	flag.Parse()

	if filter != "" {
		re, err := regexp.Compile(filter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "script-parity: -run: %v\n", err)
			return exitStand
		}
		opt.filter = re
	}
	if opt.repo == "" {
		root, err := findRepo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "script-parity: %v\n", err)
			return exitStand
		}
		opt.repo = root
	}
	abs, err := filepath.Abs(opt.repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "script-parity: %v\n", err)
		return exitStand
	}
	opt.repo = abs
	if opt.source == "" {
		opt.source = opt.repo
	}
	if opt.mutants {
		return runMutants(opt)
	}
	return runStand(opt)
}

const (
	exitPass  = 0
	exitFail  = 1
	exitStand = 2
)

func defaultJobs() int {
	// pwsh costs a second of CPU to start, and eight of them at once on a
	// four-core runner make every probe timeout of the scripts a coin toss.
	n := runtime.NumCPU() / 2
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	return n
}

func findRepo() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above the working directory; pass -repo")
		}
		dir = parent
	}
}
