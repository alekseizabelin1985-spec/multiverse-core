// Command multiverse is the single platform binary. The set of bounded
// contexts it runs is a deployment parameter (--contexts), not a build
// parameter (ADR-001, NFR-075).
//
// After wave 0 this file changes only through tech-lead#1: a context owner
// adds the registration import in one pull request (design.md §4.1).
package main

import (
	"fmt"
	"io"
	"os"
)

// version is stamped by the build: -ldflags "-X main.version=<git sha>".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches the subcommand and returns the process exit code. It takes
// its streams so that tests do not touch the real stdout.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "health":
			return runHealth(args[1:], stdout, stderr)
		case "db":
			return runDB(args[1:], stderr)
		case "version":
			_, _ = fmt.Fprintln(stdout, version)
			return 0
		}
	}
	return runServe(args, stdout, stderr)
}
