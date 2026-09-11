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
	"strings"
)

// version is stamped by the build: -ldflags "-X main.version=<git sha>".
var version = "dev"

// subcommands are the words the first argument may be. The refusal of any
// other word lists them, so this slice and the switch in dispatch have to
// agree; TestEveryListedSubcommandIsDispatched holds them together.
var subcommands = []string{"serve", "health", "db", "version"}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches the subcommand and returns the process exit code. It takes
// its streams so that tests do not touch the real stdout.
func run(args []string, stdout, stderr io.Writer) int {
	return dispatch(args, stdout, stderr, serve)
}

// dispatch is run with the start of the process passed in, so that a test can
// resolve both spellings of serve through the real dispatch and the real flag
// parsing without a process that only a signal would stop.
//
// "serve" and no subcommand at all are one command (T-414). Compose, the
// ENTRYPOINT of the image, make replay and the e2e test start the platform
// with the flags alone; the documents and the operators say "multiverse serve",
// next to "multiverse health" and "multiverse db". Refusing either spelling
// breaks one of the two, so both reach the same runServe with the same flags.
func dispatch(args []string, stdout, stderr io.Writer, start startFunc) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runServe(args, stdout, stderr, start)
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:], stdout, stderr, start)
	case "health":
		return runHealth(args[1:], stdout, stderr)
	case "db":
		return runDB(args[1:], stderr)
	case "version":
		_, _ = fmt.Fprintln(stdout, version)
		return 0
	}
	// Named rather than handed to the flags of serve: there it would come out
	// as "unexpected argument", which tells a mistyped "helth" nothing about
	// what the binary does know.
	_, _ = fmt.Fprintf(stderr, "multiverse: unknown subcommand %q: expected %s, or the flags of serve without a subcommand\n",
		args[0], joinOr(subcommands))
	return 2
}
