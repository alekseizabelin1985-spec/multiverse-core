// Package privacy implements `mvctl privacy scan`: the reading of committed
// artefacts for identifiers that belong to a person rather than to a world
// (SEC-01, SEC-23, NFR-041; ADR-010 add. 1).
//
// The security job of CI runs `mvctl privacy scan testdata/`. Everything the
// platform keeps in Git — recordings, fixtures, the golden set, the metric CSVs
// — is written by a machine from CI sessions, so a Telegram identifier, a
// handle or an e-mail address in that tree means a human session leaked into
// the repository, and the only cheap moment to notice is before the merge.
//
// This is the minimal implementation the wave needs (decision ОВ-47): it greps
// text files with the rules of the NFR-041 test. EPIC-005 (T-139) replaces it
// with the full scan — the bus, the object store, the memory index, gateway.db
// and the logs of every process — behind the same command name and the same
// exit codes, so that neither CI nor the runbook changes when it lands.
//
// No matched value is ever printed whole: a scanner that echoes what it found
// turns a private repository leak into a public CI-log leak.
package privacy

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"multiverse-core.io/cmd/mvctl/internal/cli"
)

// Summary is the line `mvctl help` shows for the command.
const Summary = "scan artefacts for external identifiers"

// DefaultRoot is what the command scans when it is called without a path. It
// is the directory the CI job names, so `mvctl privacy scan` and the job do
// the same thing.
const DefaultRoot = "testdata"

// Run dispatches `mvctl privacy <subcommand>`.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return cli.UnknownSubcommand(stderr, "mvctl privacy", "", "scan")
	}
	switch args[0] {
	case "scan":
		return runScan(args[1:], stdout, stderr)
	default:
		return cli.UnknownSubcommand(stderr, "mvctl privacy", args[0], "scan")
	}
}

// runScan implements `mvctl privacy scan [path...]`.
func runScan(args []string, stdout, stderr io.Writer) int {
	flags := cli.FlagSet("mvctl privacy scan", stderr)
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}

	roots := flags.Args()
	if len(roots) == 0 {
		roots = []string{DefaultRoot}
	}

	report := cli.NewReport("privacy scan")
	result, err := Scan(roots)
	if err != nil {
		// Both branches exit with the error code, and neither with the
		// findings code: the walk stops at the first unreadable path, so
		// what came back is a partial scan. Reporting it as "findings"
		// would let CI read an unreadable file as a leak, and — worse —
		// let a clean partial scan pass as a whole one. A path the operator
		// named and that does not exist is the same class of mistake: a
		// pipeline that renamed its fixture directory must fail loudly
		// instead of reporting a clean scan.
		if errors.Is(err, fs.ErrNotExist) {
			_, _ = fmt.Fprintf(stderr, "mvctl privacy scan: %v\n", err)
			return cli.ExitUsage
		}
		_, _ = fmt.Fprintf(stderr,
			"mvctl privacy scan: %v (the scan is incomplete: it stopped at this path)\n", err)
		return cli.ExitUsage
	}

	for _, skipped := range result.Skipped {
		report.Linef("skipped %s", skipped)
	}
	for _, f := range result.Findings {
		report.Addf(f.Rule, fmt.Sprintf("%s:%d", f.Path, f.Line),
			"%s: %s", f.What, Redact(f.Value))
	}
	report.Details = result
	if report.OK() {
		report.Summary = fmt.Sprintf("no external identifiers in %s (%s read)",
			strings.Join(roots, ", "), cli.Plural(result.Files, "file"))
	}
	return report.Write(stdout, stderr, *asJSON)
}
