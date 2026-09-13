package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type impl string

const (
	implSh impl = "sh"
	implPS impl = "ps1"
)

// The files a scenario runs against, copied into a fresh tree per scenario and
// per implementation: `up` writes ops/llm-server.pid and the bench writes its
// reports next to the scripts, and neither may touch the repository.
var treeFiles = []string{
	"scripts/llm-server.sh",
	"scripts/llm-server.ps1",
	"scripts/llm-bench.sh",
	"scripts/llm-bench.ps1",
	"scripts/lib/llm-endpoint.sh",
	"scripts/lib/LlmEndpoint.psm1",
	"build/versions.env",
	"ops/metrics/bench-matrix.json",
	"testdata/bench/prompts.jsonl",
}

type stand struct {
	opt         options
	scratch     string
	longScratch string
	gpuDir      string
	msysScratch string
	bash        string
	pwsh        string
	haveCurl    bool
	havePython  bool
	windows     bool
	doubleExe   string
	token       string
	baseEnv     []string

	mu           sync.Mutex
	usedPorts    map[int]bool
	programPorts []int
}

func runStand(opt options) int {
	// Ctrl+C in `make ci` or a cancelled CI run: stop taking scenarios, let the
	// running ones end (their child processes get the context), stop the doubles
	// by the token of this run and remove the scratch directory by its saved path
	// (T-405 review #1, Mi-3). The scripts under test receive the same console
	// signal and end on their own.
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	s, err := newStand(opt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "script-parity: %v\n", err)
		return exitStand
	}
	defer s.close()
	// A panic outside the workers still unwinds through here: the doubles are
	// stopped before the deferred close removes the directory their executable
	// lives in.
	defer func() {
		if r := recover(); r != nil {
			s.releasePorts()
			panic(r)
		}
	}()

	scenarios := allScenarios()
	selected := scenarios[:0:0]
	for _, sc := range scenarios {
		if opt.filter == nil || opt.filter.MatchString(sc.ID) || opt.filter.MatchString(sc.Title) {
			selected = append(selected, sc)
		}
	}

	mode := "full: both implementations are run and compared"
	if s.pwsh == "" {
		mode = "bash half only (no pwsh): absolute expectations of every scenario plus the static comparison of the message texts; parity comparisons are SKIP"
	}
	fmt.Printf("script-parity: source %s\n", opt.source)
	fmt.Printf("script-parity: bash %s; pwsh %s; curl %v; python3 %v\n", s.bash, orNone(s.pwsh), s.haveCurl, s.havePython)
	fmt.Printf("script-parity: mode %s\n", mode)
	fmt.Printf("script-parity: %d scenarios, %d in parallel, scratch %s\n", len(selected), opt.jobs, s.scratch)

	results := make([]result, len(selected))
	ran := make([]bool, len(selected))
	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < opt.jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				func() {
					// A panic in a scenario is a FAIL of that scenario, not the end of
					// the run: the rest still runs and the cleanup still happens.
					defer func() {
						if r := recover(); r != nil {
							results[i] = result{status: statusFail, notes: []string{fmt.Sprintf("the stand panicked: %v", r)}}
						}
					}()
					results[i] = s.runScenario(ctx, selected[i])
				}()
				ran[i] = true
			}
		}()
	}
dispatch:
	for i := range selected {
		select {
		case work <- i:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(work)
	wg.Wait()

	if ctx.Err() != nil {
		released, total := s.releasePorts()
		done := 0
		for _, r := range ran {
			if r {
				done++
			}
		}
		fmt.Fprintf(os.Stderr, "script-parity: interrupted after %d of %d scenarios; ports of the llama-server double released %d/%d\n", done, len(selected), released, total)
		return exitStand
	}

	counts := map[string]int{}
	for i, r := range results {
		counts[r.status]++
		fmt.Printf("%-14s %s  %s\n", r.status, selected[i].ID, selected[i].Title)
		for _, id := range sortedKeys(selected[i].Reports) {
			if d := defectByID(id); d != nil && (r.status == statusKnown || r.status == statusFixed) {
				fmt.Printf("               known defect %s: %s\n", id, d.backlog)
			}
		}
		if r.status == statusSkip {
			fmt.Printf("               %s\n", strings.Join(r.notes, "; "))
			continue
		}
		if r.status == statusFail || r.status == statusFixed || (r.status == statusKnown && opt.verbose) {
			for _, n := range r.notes {
				for _, line := range strings.Split(n, "\n") {
					fmt.Printf("               %s\n", line)
				}
			}
		}
	}

	// Items of known defects across the whole run (review #2 of T-405, Mi-1).
	summary, fixedItems, failedItems := knownSummary(selected, results, s.pwsh != "")
	for _, line := range summary {
		fmt.Println(line)
	}
	counts[statusFixed] += fixedItems
	counts[statusFail] += failedItems

	// The message texts, compared statically. With pwsh this is a second line
	// of defence; without it, the only comparison of the PowerShell half.
	lint := lintMessages(opt.source)
	// The table of the local address, run through both halves (T-450).
	lint = append(lint, s.endpointTable(ctx)...)
	for _, l := range lint {
		counts[l.status]++
		fmt.Printf("%-14s %s  %s\n", l.status, l.id, l.title)
		for _, n := range l.notes {
			fmt.Printf("               %s\n", n)
		}
	}

	released, total := s.releasePorts()
	fmt.Printf("script-parity: scenarios and message pairs: %d passed, %d failed, %d known-failing, %d fixed-but-marked, %d skipped; ports of the llama-server double released %d/%d\n",
		counts[statusPass], counts[statusFail], counts[statusKnown], counts[statusFixed], counts[statusSkip], released, total)

	if released != total {
		fmt.Fprintln(os.Stderr, "script-parity: a double is still listening after its run — see the ports above")
		return exitFail
	}
	if counts[statusFail] > 0 || counts[statusFixed] > 0 {
		return exitFail
	}
	return exitPass
}

func orNone(s string) string {
	if s == "" {
		return "<not found>"
	}
	return s
}

func newStand(opt options) (*stand, error) {
	s := &stand{opt: opt, windows: runtime.GOOS == "windows"}
	for _, f := range treeFiles {
		if _, err := os.Stat(filepath.Join(opt.source, filepath.FromSlash(f))); err != nil {
			return nil, fmt.Errorf("the source tree has no %s: %v", f, err)
		}
	}

	bash, err := exec.LookPath("bash")
	if err != nil {
		return nil, errors.New("bash is not on PATH")
	}
	s.bash = bash
	if s.windows {
		// C:\Windows\System32\bash.exe is WSL: a Linux shell that cannot see the
		// Windows paths this stand hands it. Git Bash (MSYS) is the one make uses.
		out, err := exec.Command(bash, "-c", "uname -s").Output()
		kernel := strings.TrimSpace(string(out))
		if err != nil || !(strings.HasPrefix(kernel, "MINGW") || strings.HasPrefix(kernel, "MSYS") || strings.HasPrefix(kernel, "CYGWIN")) {
			return nil, fmt.Errorf("%s is not Git Bash (uname -s says %q); put Git's usr/bin first on PATH, as make does", bash, kernel)
		}
	}
	_, errCurl := exec.LookPath("curl")
	s.haveCurl = errCurl == nil
	_, errPy := exec.LookPath("python3")
	s.havePython = errPy == nil
	if !s.haveCurl {
		return nil, errors.New("curl is not on PATH; the bash half probes with it")
	}

	switch opt.pwsh {
	case "off":
	case "auto", "require":
		if p, err := exec.LookPath("pwsh"); err == nil {
			s.pwsh = p
		}
		// In CI the job exists to run the PowerShell half on Linux: a runner
		// image without pwsh must fail the job, not turn it into the bash half
		// quietly (T-405 review #1, Mi-2).
		if opt.pwsh == "require" && s.pwsh == "" {
			return nil, errors.New("-pwsh=require and pwsh is not on PATH")
		}
	default:
		return nil, fmt.Errorf("-pwsh must be auto, require or off, not %q", opt.pwsh)
	}

	s.scratch, err = os.MkdirTemp("", "script-parity-")
	if err != nil {
		return nil, err
	}
	s.longScratch = s.scratch
	if long, err := filepath.EvalSymlinks(s.scratch); err == nil {
		s.longScratch = long
	}
	// The form bash gives the same directory: Git Bash maps %TEMP% to /tmp, and
	// the scripts print their own root the way bash sees it.
	cmd := exec.Command(bash, "-c", "pwd")
	cmd.Dir = s.scratch
	out, err := cmd.Output()
	if err != nil {
		s.close()
		return nil, fmt.Errorf("bash cannot enter the scratch directory: %v", err)
	}
	s.msysScratch = strings.TrimSpace(string(out))

	exe, err := os.Executable()
	if err != nil {
		s.close()
		return nil, err
	}
	name := "parity-double"
	if s.windows {
		name += ".exe"
	}
	s.doubleExe = filepath.Join(s.scratch, "bin", name)
	if err := copyFile(exe, s.doubleExe, 0o755); err != nil {
		s.close()
		return nil, fmt.Errorf("copy the double: %v", err)
	}

	// nvidia-smi of the stand, first on PATH for every script: the VRAM line of
	// `health` and the vram_used_mb of the bench then read the same thing on
	// every machine, and the type of cells[].vram_used_mb (item of K02) no longer
	// depends on whether the machine has a GPU (T-405 review #1, M-1). The real
	// tool is not called at all.
	s.gpuDir = filepath.Join(s.scratch, "gpu")
	if err := copyFile(exe, filepath.Join(s.gpuDir, gpuToolName+filepath.Ext(name)), 0o755); err != nil {
		s.close()
		return nil, fmt.Errorf("copy the nvidia-smi double: %v", err)
	}

	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	s.token = hex.EncodeToString(buf)
	s.baseEnv = prependPath(cleanEnv(os.Environ(), s.windows), s.gpuDir, s.windows)
	return s, nil
}

// prependPath puts dir in front of PATH (Path on Windows, whatever its case).
func prependPath(env []string, dir string, windows bool) []string {
	for i, kv := range env {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		key := kv[:eq]
		if key == "PATH" || (windows && strings.EqualFold(key, "PATH")) {
			env[i] = key + "=" + dir + string(os.PathListSeparator) + kv[eq+1:]
			return env
		}
	}
	return append(env, "PATH="+dir)
}

func (s *stand) close() {
	if s.scratch == "" {
		return
	}
	dir := s.scratch
	s.scratch = ""
	if s.opt.keep {
		fmt.Printf("script-parity: scratch kept at %s\n", dir)
		return
	}
	// The one directory this run created, by the path it saved — nothing else.
	// On Windows a directory holding a running executable cannot be removed, and
	// a double stopped a moment ago may still be exiting: a few attempts, then
	// the exact path for the operator, never a pattern (T-405 review #1, Mi-3).
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		if err = os.RemoveAll(dir); err == nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "script-parity: could not remove the scratch directory %s: %v — remove this exact directory by hand\n", dir, err)
}

// cleanEnv drops everything a scenario sets on its own: no MV_* of the caller's
// .env, no proxy that would answer for an address where nothing listens (T-404
// review #1, M-2), no role of a double.
func cleanEnv(env []string, windows bool) []string {
	var out []string
	for _, kv := range env {
		key := kv
		if i := strings.IndexByte(kv, '='); i > 0 {
			key = kv[:i]
		}
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "MV_") || strings.HasPrefix(upper, "MVPARITY_") {
			if windows || strings.HasPrefix(key, "MV_") || strings.HasPrefix(key, "MVPARITY_") {
				continue
			}
		}
		switch upper {
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "FTP_PROXY":
			continue
		}
		out = append(out, kv)
	}
	return out
}

func copyFile(from, to string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// freePort hands out a port nobody listens on and that no other scenario of
// this run has been given. The OS gives a closed port back to the next
// listen on :0, and scenarios run in parallel: a "dead" port of one scenario
// became the port of another scenario's double, and B07 found a server where
// it expected none (seen in iteration 3 of T-405, the risk review #1 named).
func (s *stand) freePort() (int, error) {
	for attempt := 0; attempt < 100; attempt++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		s.mu.Lock()
		taken := s.usedPorts[port]
		if !taken {
			if s.usedPorts == nil {
				s.usedPorts = map[int]bool{}
			}
			s.usedPorts[port] = true
		}
		s.mu.Unlock()
		if !taken {
			return port, nil
		}
	}
	return 0, errors.New("no port left that this run has not handed out")
}

// ---------------------------------------------------------------------------
// one implementation of one scenario

type stepRun struct {
	action   string
	exit     int
	stdout   string
	stderr   string
	timedOut bool
	models   []string
	answers  bool // something answered on {PORT} after the step
	launches [][]string
	requests []requestRecord
}

type implRun struct {
	impl     impl
	run      string
	tree     string
	vars     map[string]string
	steps    []stepRun
	launches [][]string
	requests []requestRecord
	err      error
}

func (s *stand) runImpl(ctx context.Context, sc *Scenario, which impl) *implRun {
	r := &implRun{impl: which, vars: map[string]string{}}
	r.run = filepath.Join(s.scratch, "runs", sc.ID, string(which))
	r.tree = filepath.Join(r.run, "tree")
	for _, f := range treeFiles {
		if err := copyFile(filepath.Join(s.opt.source, filepath.FromSlash(f)), filepath.Join(r.tree, filepath.FromSlash(f)), 0o644); err != nil {
			r.err = err
			return r
		}
	}
	doubleDir := filepath.Join(r.run, "double")
	if err := os.MkdirAll(doubleDir, 0o755); err != nil {
		r.err = err
		return r
	}

	port, err := s.freePort()
	if err != nil {
		r.err = err
		return r
	}
	dead, err := s.freePort()
	if err != nil {
		r.err = err
		return r
	}
	r.vars["PORT"] = strconv.Itoa(port)
	r.vars["DEAD"] = strconv.Itoa(dead)
	r.vars["DOUBLE"] = s.doubleExe
	r.vars["TREE"] = r.tree
	r.vars["RUN"] = r.run

	if sc.Setup != nil {
		if err := sc.Setup(r); err != nil {
			r.err = fmt.Errorf("setup: %v", err)
			return r
		}
	}

	if sc.Server != nil {
		ln, err := net.Listen("tcp", "127.0.0.1:"+r.vars["PORT"])
		if err != nil {
			r.err = fmt.Errorf("the double cannot listen: %v", err)
			return r
		}
		stop := serveInProcess(ln, *sc.Server, filepath.Join(doubleDir, "requests.jsonl"))
		defer stop()
	}
	if sc.Program != nil {
		s.mu.Lock()
		s.programPorts = append(s.programPorts, port)
		s.mu.Unlock()
	}

	env := append([]string(nil), s.baseEnv...)
	for _, key := range sortedKeys(sc.Env) {
		env = append(env, key+"="+expand(sc.Env[key], r.vars))
	}
	programCfg := DoubleConfig{}
	if sc.Program != nil {
		programCfg = *sc.Program
	}
	cfgJSON, _ := json.Marshal(programCfg)
	env = append(env,
		envRole+"="+roleDouble,
		envDir+"="+doubleDir,
		envConfig+"="+string(cfgJSON),
		envToken+"="+s.token,
		envLifetime+"=150",
	)

	for i, step := range sc.Steps {
		sr := stepRun{action: step.Action}
		args := s.command(sc, step, which, r.vars)
		stepCtx, cancel := context.WithTimeout(ctx, step.timeout())
		cmd := exec.CommandContext(stepCtx, args[0], args[1:]...)
		cmd.Dir = r.tree
		cmd.Env = env
		// Files and not buffers: `up` leaves a server behind that may inherit the
		// handles, and a pipe would hold Wait until that server exits.
		outPath := filepath.Join(r.run, fmt.Sprintf("step%d.out", i+1))
		errPath := filepath.Join(r.run, fmt.Sprintf("step%d.err", i+1))
		outFile, err1 := os.Create(outPath)
		errFile, err2 := os.Create(errPath)
		if err1 != nil || err2 != nil {
			r.err = errors.Join(err1, err2)
			cancel()
			return r
		}
		cmd.Stdout, cmd.Stderr = outFile, errFile
		cmd.WaitDelay = 5 * time.Second
		runErr := cmd.Run()
		cancel()
		outFile.Close()
		errFile.Close()
		sr.timedOut = errors.Is(stepCtx.Err(), context.DeadlineExceeded)
		if ctx.Err() != nil {
			r.err = errors.New("interrupted")
			if sc.Program != nil {
				s.stopProgram(port)
			}
			return r
		}
		var exitErr *exec.ExitError
		switch {
		case runErr == nil:
			sr.exit = 0
		case errors.As(runErr, &exitErr):
			sr.exit = exitErr.ExitCode()
		default:
			r.err = fmt.Errorf("step %d (%s): %v", i+1, step.Action, runErr)
			return r
		}
		sr.stdout = readText(outPath)
		sr.stderr = readText(errPath)
		if step.ReadModels {
			sr.models = getModels(r.vars["PORT"])
		}
		sr.answers = answers(r.vars["PORT"])
		// What the double saw during THIS step: both files only ever grow.
		launches := readLaunches(filepath.Join(doubleDir, "launches.jsonl"))
		requests := readRequests(filepath.Join(doubleDir, "requests.jsonl"))
		sr.launches = launches[min(len(r.launches), len(launches)):]
		sr.requests = requests[min(len(r.requests), len(requests)):]
		r.launches, r.requests = launches, requests
		r.steps = append(r.steps, sr)
	}

	if sc.Program != nil {
		s.stopProgram(port)
	}
	r.launches = readLaunches(filepath.Join(doubleDir, "launches.jsonl"))
	r.requests = readRequests(filepath.Join(doubleDir, "requests.jsonl"))
	return r
}

func (s *stand) command(sc *Scenario, step Step, which impl, vars map[string]string) []string {
	var args []string
	switch sc.Script {
	case scriptServer:
		if which == implSh {
			args = []string{s.bash, "scripts/llm-server.sh", step.Action}
			if step.Router {
				args = append(args, "--router")
			}
			if step.WithUI {
				args = append(args, "--with-ui")
			}
		} else {
			args = []string{s.pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-File", "scripts/llm-server.ps1", "-Action", step.Action}
			if step.Router {
				args = append(args, "-Router")
			}
			if step.WithUI {
				args = append(args, "-WithUi")
			}
		}
	case scriptBench:
		b := step.Bench
		if which == implSh {
			args = []string{s.bash, "scripts/llm-bench.sh", "--configs", b.Configs, "--limit", strconv.Itoa(b.Limit)}
			if b.Matrix != "" {
				args = append(args, "--matrix", expand(b.Matrix, vars))
			}
			if b.OutDir != "" {
				args = append(args, "--out-dir", expand(b.OutDir, vars))
			}
		} else {
			args = []string{s.pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-File", "scripts/llm-bench.ps1", "-Configs", b.Configs, "-Limit", strconv.Itoa(b.Limit)}
			if b.Matrix != "" {
				args = append(args, "-Matrix", expand(b.Matrix, vars))
			}
			if b.OutDir != "" {
				args = append(args, "-OutDir", expand(b.OutDir, vars))
			}
		}
	}
	return args
}

func expand(value string, vars map[string]string) string {
	for k, v := range vars {
		value = strings.ReplaceAll(value, "{"+k+"}", v)
	}
	return value
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func readText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func readLaunches(path string) [][]string {
	var out [][]string
	for _, line := range strings.Split(readText(path), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec struct {
			Argv []string `json:"argv"`
		}
		if json.Unmarshal([]byte(line), &rec) == nil {
			out = append(out, rec.Argv)
		}
	}
	return out
}

func readRequests(path string) []requestRecord {
	var out []requestRecord
	for _, line := range strings.Split(readText(path), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec requestRecord
		if json.Unmarshal([]byte(line), &rec) == nil {
			out = append(out, rec)
		}
	}
	return out
}

// directClient never goes through a proxy: the question is what listens on
// 127.0.0.1, not what a proxy says about it.
var directClient = &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}

func getModels(port string) []string {
	resp, err := directClient.Get("http://127.0.0.1:" + port + "/v1/models")
	if err != nil {
		return []string{"<no answer>"}
	}
	defer resp.Body.Close()
	var doc struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return []string{"<not JSON>"}
	}
	ids := []string{}
	for _, d := range doc.Data {
		ids = append(ids, d.ID)
	}
	return ids
}

func answers(port string) bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// stopProgram asks the double on port to leave, if something still listens
// there — and only if that something says it is our double: the executable it
// reports must be the file this run copied, and the token must be this run's.
func (s *stand) stopProgram(port int) {
	p := strconv.Itoa(port)
	if !answers(p) {
		return
	}
	resp, err := directClient.Get("http://127.0.0.1:" + p + pathWhoami)
	if err != nil {
		return
	}
	var who struct {
		Exe string `json:"exe"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&who)
	resp.Body.Close()
	a, errA := os.Stat(who.Exe)
	b, errB := os.Stat(s.doubleExe)
	if errA != nil || errB != nil || !os.SameFile(a, b) {
		return
	}
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+p+pathShutdown, bytes.NewReader(nil))
	req.Header.Set(headerToken, s.token)
	if resp, err := directClient.Do(req); err == nil {
		resp.Body.Close()
	}
	for i := 0; i < 50 && answers(p); i++ {
		time.Sleep(100 * time.Millisecond)
	}
}

// releasePorts is the last word of a run: nothing of ours listens any more.
func (s *stand) releasePorts() (released, total int) {
	s.mu.Lock()
	ports := append([]int(nil), s.programPorts...)
	s.mu.Unlock()
	for _, port := range ports {
		s.stopProgram(port)
		if answers(strconv.Itoa(port)) {
			fmt.Printf("script-parity: port %d still answers after the run\n", port)
			continue
		}
		released++
	}
	return released, len(ports)
}
