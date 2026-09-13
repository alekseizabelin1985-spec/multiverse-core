package main

import (
	"os"
	"path/filepath"
	"time"
)

const (
	scriptServer = "llm-server"
	scriptBench  = "llm-bench"
)

// Scenario is one input, run through both implementations.
type Scenario struct {
	ID     string
	Title  string
	Script string
	// Env is the whole MV_* environment of the run: a key that is absent is
	// unset. {PORT} is the port of the double, {DEAD} a port nobody listens on,
	// {DOUBLE} the executable of the double, {TREE} and {RUN} the directories of
	// this run.
	Env map[string]string
	// Server starts the in-process double on {PORT} for the whole run.
	Server *DoubleConfig
	// Program configures the double the scripts start through MV_LLM_BIN; a
	// non-nil value also makes the stand stop it and check its port afterwards.
	Program *DoubleConfig
	Setup   func(r *implRun) error
	Steps   []Step
	// Reports makes this scenario the visible report of known defects
	// (known.go): defect id -> the items it must see differ. Every item seen and
	// no other difference: KNOWN-FAILING. An item gone: FIXED-BUT-MARKED. Any
	// difference outside the items — and every expectation — fails it like any
	// other scenario.
	Reports map[string][]string
	// ParityOnly: there is nothing to check without the second implementation.
	ParityOnly bool
}

// Step is one invocation of the script.
type Step struct {
	Action     string // up | down | health (llm-server); "bench" for the bench
	Router     bool
	WithUI     bool
	Bench      *BenchArgs
	ReadModels bool // GET /v1/models on {PORT} after the step
	Expect     Expect
}

func (s Step) timeout() time.Duration {
	switch s.Action {
	case "up":
		return 90 * time.Second
	case "bench":
		return 150 * time.Second
	}
	return 60 * time.Second
}

// BenchArgs are the arguments of llm-bench in the shape of both halves.
type BenchArgs struct {
	Configs string
	Limit   int
	Matrix  string
	OutDir  string
}

// Expect holds for EACH implementation, parity or not. It is what keeps two
// halves that are wrong in the same way from passing, and it is all the bash
// half is checked against when pwsh is not there.
type Expect struct {
	Exit     int
	Contains []string // in stdout+stderr, after normalisation
	Stderr   []string // in stderr, after normalisation
	Absent   []string // in the raw output
	// Argv is the exact argument vector of the one start of llama-server in
	// this step, after normalisation; NoLaunch says there must be none.
	Argv     []string
	NoLaunch bool
	Models   []string // what GET /v1/models reports after the step
	PortFree bool     // nothing answers on {PORT} after the step
	NoFile   string   // this file of the tree must not exist after the step
	Requests []string // "METHOD path" in order, consecutive repeats folded
	Auth     string   // every request to the double carried exactly this header
	Bench    *BenchExpect
}

// BenchExpect is what a bench run must have written.
type BenchExpect struct {
	Rows     int               // data rows of the CSV
	Columns  map[string]string // column -> value in every row ("<set>" = not empty)
	Cache    string            // CACHE of every row of the table
	Meta     map[string]string // meta field -> value (type-normalised)
	MetaFile map[string]string // meta sha256 field -> file of the run it describes
	PromptMS string            // "mixed": numbers and nulls among requests[].prompt_ms
	Reports  int               // JSON reports written
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
