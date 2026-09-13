package main

// The control mutants: a stand that stays green whatever is done to the
// scripts proves nothing, so `-mutants` breaks the scripts on purpose, one
// defect at a time, in a copy of the tree, and requires the stand to go red on
// every one of them. The control goes first (a syntax error: if even that
// stays green, the stand is not running the scripts at all), the identity
// mutant second (no change: it must stay green, or red means something other
// than the defect).
//
// Each mutant replaces one anchor that must occur exactly once — a mutant whose
// anchor has moved away is reported as such, not quietly skipped.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type mutant struct {
	id      string
	title   string
	file    string
	anchor  string
	replace string
	run     string // scenarios that must catch it
	pwshOff bool   // run the bash half only: the static comparison must catch it
	green   bool   // the identity mutant: must stay green
}

var mutants = []mutant{
	{id: "M00", title: "control: a syntax error in llm-server.sh",
		file: "scripts/llm-server.sh", anchor: `case "$action" in`, replace: `case "$action" inn`, run: "^(H01|U01)$"},
	{id: "M01", title: "identity: nothing changed, the stand must stay green", green: true, run: "^(H01|H34|U01|U06|B01)$"},
	{id: "M02", title: "llm-server.sh stops passing --alias (T-437)",
		file: "scripts/llm-server.sh", anchor: `server_args+=(-m "$model_file" --alias "$model_id")`, replace: `server_args+=(-m "$model_file")`, run: "^(U01|U06)$"},
	{id: "M03", title: "llm-bench.ps1 rounds the cache share with [math]::Round again (T-434 N-1)",
		file: "scripts/llm-bench.ps1", anchor: `{ Get-BenchRound ($cachedTok / $promptTok) 2 }`, replace: `{ [math]::Round($cachedTok / $promptTok, 2) }`, run: "^(B01|B02)$"},
	{id: "M04", title: "llm-bench.ps1 rounds tps with [math]::Round again",
		file: "scripts/llm-bench.ps1", anchor: `{ Get-BenchRound ($completionTok / ($predictedMs / 1000.0)) 1 }`, replace: `{ [math]::Round($completionTok / ($predictedMs / 1000.0), 1) }`, run: "^B02$"},
	{id: "M05", title: "llm-server.ps1 hands Start-Process the bare array again (T-437 backlog)",
		file: "scripts/llm-server.ps1", anchor: `ArgumentList           = (ConvertTo-ArgumentLine $serverArgs)`, replace: `ArgumentList           = $serverArgs`, run: "^(U06|U07|U08)$"},
	{id: "M06", title: "a message of llm-server.ps1 changes, the twin does not",
		file: "scripts/llm-server.ps1", anchor: `names no file, so the server would have no model id to report`, replace: `names no file, so the server has no model id to report`, run: "^U05$"},
	{id: "M07", title: "the same message drift, bash half only: the static comparison alone must see it", pwshOff: true,
		file: "scripts/llm-server.ps1", anchor: `names no file, so the server would have no model id to report`, replace: `names no file, so the server has no model id to report`, run: "^U05$"},
	{id: "M08", title: "LlmEndpoint.psm1 trims every Unicode space again",
		file: "scripts/lib/LlmEndpoint.psm1", anchor: `return $Value.Trim($script:LlmAsciiSpace)`, replace: `return $Value.Trim()`, run: "^(H33|H57|H58)$"},
	{id: "M09", title: "llm-endpoint.sh folds a TLS failure into 'nothing answered' (review #2 M-2)",
		file: "scripts/lib/llm-endpoint.sh", anchor: `  0 | 6 | 7 | 28) ;;`, replace: `  *) ;;`, run: "^H34$"},
	{id: "M10", title: "llm-bench.ps1 leaves the header-only CSV behind again",
		file: "scripts/llm-bench.ps1", anchor: `    Remove-Item -LiteralPath $script:csvPath -ErrorAction SilentlyContinue`, replace: `    $null = $script:csvPath`, run: "^(B07|B08)$"},
	{id: "M11", title: "llm-bench.ps1 prints the absolute path of the report again",
		file: "scripts/llm-bench.ps1", anchor: `Write-Host "bench: $(Get-ShownPath $reportPath)"`, replace: `Write-Host "bench: $reportPath"`, run: "^B10$"},
	{id: "M12", title: "llm-bench.sh prints the second, vaguer line for an unknown configuration again",
		file: "scripts/llm-bench.sh", anchor: `    [ "$status" -eq 3 ] && exit 2`, replace: `    :`, run: "^B08$"},
	// Review #1 of T-405, M-1: a known defect must not hide its whole class.
	{id: "M13", title: "a new difference of JSON types (tps of a cell written as a string by llm-bench.ps1) next to the known ones of K02",
		file: "scripts/llm-bench.ps1", anchor: `            tps              = $tps`, replace: `            tps              = [string]$tps`, run: "^(B01|K02)$"},
	{id: "M14", title: "a new difference of streams (the warm-up line of llm-bench.ps1 moved to stdout) next to the known ones of K01",
		file: "scripts/llm-bench.ps1", anchor: `[Console]::Error.WriteLine("bench: warm-up call answered`, replace: `Write-Host ("bench: warm-up call answered`, run: "^B03$"},
	{id: "M15", title: "one item of a known defect goes away (meta.num_ctx written as a string by llm-bench.ps1 too): K02 must say FIXED-BUT-MARKED",
		file: "scripts/llm-bench.ps1", anchor: `        num_ctx        = $NumCtx`, replace: `        num_ctx        = [string]$NumCtx`, run: "^K02$"},
	{id: "M16", title: "llm-bench.sh takes the complaint of a failing nvidia-smi for a VRAM figure again",
		file: "scripts/llm-bench.sh", anchor: `nounits 2>/dev/null) || return 0`, replace: `nounits 2>/dev/null) || :`, run: "^B11$"},
	// X5 of review #2: three items of K01 get fixed at once, and none of them is
	// in the Reports of K01 — only the summary over every bench scenario sees it.
	{id: "M17", title: "X5 of review #2: Stop-Bench of llm-bench.ps1 writes to stderr, three items of K01 no longer differ",
		file: "scripts/llm-bench.ps1", anchor: `  Write-Host "bench: $Message" -ForegroundColor Red`, replace: `  [Console]::Error.WriteLine("bench: $Message")`, run: "^[BK]\\d\\d$"},
}

func runMutants(opt options) int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "script-parity: %v\n", err)
		return exitStand
	}
	root, err := os.MkdirTemp("", "script-parity-mutants-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "script-parity: %v\n", err)
		return exitStand
	}
	defer func() {
		if opt.keep {
			fmt.Printf("script-parity: mutant trees kept at %s\n", root)
			return
		}
		// By the saved path only; a child stand may still be leaving.
		var err error
		for attempt := 0; attempt < 20; attempt++ {
			if err = os.RemoveAll(root); err == nil {
				return
			}
			time.Sleep(250 * time.Millisecond)
		}
		fmt.Fprintf(os.Stderr, "script-parity: could not remove %s: %v — remove this exact directory by hand\n", root, err)
	}()
	// Ctrl+C reaches the child stand through the console and it cleans up after
	// itself; this loop only stops starting new mutants.
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	fmt.Printf("script-parity: %d mutants against %s, control first\n", len(mutants), opt.source)
	bad := 0
	for i, m := range mutants {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "script-parity: interrupted")
			return exitStand
		}
		if opt.filter != nil && !opt.filter.MatchString(m.id) && !opt.filter.MatchString(m.title) {
			continue
		}
		tree := filepath.Join(root, m.id)
		for _, f := range treeFiles {
			if err := copyFile(filepath.Join(opt.source, filepath.FromSlash(f)), filepath.Join(tree, filepath.FromSlash(f)), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "script-parity: %v\n", err)
				return exitStand
			}
		}
		if m.file != "" {
			path := filepath.Join(tree, filepath.FromSlash(m.file))
			raw, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "script-parity: %v\n", err)
				return exitStand
			}
			if n := bytes.Count(raw, []byte(m.anchor)); n != 1 {
				fmt.Printf("%-9s %s  %s\n          the anchor %q occurs %d times in %s — update the mutant\n", "BROKEN", m.id, m.title, m.anchor, n, m.file)
				bad++
				continue
			}
			raw = bytes.Replace(raw, []byte(m.anchor), []byte(m.replace), 1)
			if err := os.WriteFile(path, raw, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "script-parity: %v\n", err)
				return exitStand
			}
		}

		pwsh := opt.pwsh
		if m.pwshOff {
			pwsh = "off"
		}
		cmd := exec.Command(exe, "-repo", opt.repo, "-source", tree, "-run", m.run, "-pwsh", pwsh, "-jobs", fmt.Sprint(opt.jobs))
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		runErr := cmd.Run()
		code := 0
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else if runErr != nil {
			fmt.Fprintf(os.Stderr, "script-parity: %v\n", runErr)
			return exitStand
		}

		verdict := ""
		switch {
		case code == exitStand:
			verdict = "BROKEN"
		case m.green && code == exitPass:
			verdict = "GREEN"
		case m.green:
			verdict = "RED"
		case code == exitFail:
			verdict = "KILLED"
		default:
			verdict = "SURVIVED"
		}
		fmt.Printf("%-9s %s  %s\n", verdict, m.id, m.title)
		for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
			if strings.HasPrefix(line, "FAIL") || strings.HasPrefix(line, statusFixed) || strings.HasPrefix(line, "known defect ") || strings.HasPrefix(line, "script-parity: scenarios and message pairs:") {
				fmt.Printf("          %s\n", line)
			}
		}
		if verdict == "BROKEN" || verdict == "SURVIVED" || verdict == "RED" {
			bad++
			fmt.Println(indent(out.String(), "          | "))
			if i == 0 {
				fmt.Println("script-parity: the control mutant was not killed — the stand does not run the scripts; stopping")
				return exitFail
			}
		}
	}
	if bad > 0 {
		return exitFail
	}
	return exitPass
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return prefix + strings.Join(lines, "\n"+prefix)
}
