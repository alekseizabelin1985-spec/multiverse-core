package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// errDBNotWired is returned until a context of this process owns a database.
// The only SQLite databases of the platform (links.db, gateway.db) arrive with
// internal/gateway in EPIC-004 (ADR-019), which also fills these subcommands.
var errDBNotWired = errors.New("no database is compiled into this process yet (EPIC-004, ADR-019)")

// runDB implements "multiverse db backup|check": the operator entry points for
// the embedded databases of the process.
func runDB(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("multiverse db", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() == 0 {
		_, _ = fmt.Fprintln(stderr, "multiverse db: expected subcommand backup or check")
		return 2
	}
	switch fs.Arg(0) {
	case "backup", "check":
		_, _ = fmt.Fprintf(stderr, "multiverse db %s: %v\n", fs.Arg(0), errDBNotWired)
		return 1
	default:
		_, _ = fmt.Fprintf(stderr, "multiverse db: unknown subcommand %q, expected backup or check\n", fs.Arg(0))
		return 2
	}
}
