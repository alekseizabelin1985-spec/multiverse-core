// Command mvctl is the operator and CI command line of the platform: it checks
// contracts, prints the target configuration of the cluster, reads the
// environment manifest and prepares the object store (foundation.md §12,
// design.md §4.1).
//
// The table below is the registry of subcommands. Five epics fill it, so the
// names of the ones that are not written yet are reserved here: a reserved name
// exits with the usage code and says which epic owns it, which is cheaper than
// discovering at merge time that two teams both called their command "report".
//
// After wave 0 this file changes only through tech-lead#1: an owner adds the
// line of their command in one pull request (design.md §4.1).
package main

import (
	"io"
	"os"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	contractscmd "multiverse-core.io/cmd/mvctl/internal/contracts"
	envcmd "multiverse-core.io/cmd/mvctl/internal/env"
	storagecmd "multiverse-core.io/cmd/mvctl/internal/storage"
)

// version is stamped by the build: -ldflags "-X main.version=<git sha>".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches the subcommand and returns the process exit code. It takes its
// streams so that tests do not touch the real stdout.
func run(args []string, stdout, stderr io.Writer) int {
	return commands().Dispatch("mvctl", args, stdout, stderr)
}

// commands is the registry: implemented commands first, then the names held for
// the epics that will implement them.
func commands() *cli.Registry {
	return cli.NewRegistry(
		// --- EPIC-001 (foundation) ---
		cli.Command{
			Name:    "contracts",
			Summary: contractscmd.Summary,
			Run:     contractscmd.Run,
		},
		cli.Command{
			Name:    "env",
			Summary: envcmd.Summary,
			Run:     envcmd.Run,
		},
		cli.Command{
			Name:    "storage",
			Summary: storagecmd.Summary,
			Run:     storagecmd.Run,
		},
		cli.Command{
			Name:    "version",
			Summary: "print the version this binary was built from",
			Run:     runVersion,
		},

		// --- reserved: the owner writes the command in its own epic ---
		cli.Reserved("world", "create a world, load fixtures, write a snapshot", "EPIC-002"),
		cli.Reserved("blueprint", "validate and inspect agent blueprints", "EPIC-003"),
		cli.Reserved("laws", "read and bump the laws of a world", "EPIC-003"),
		cli.Reserved("record", "record and replay a session", "EPIC-003"),
		cli.Reserved("golden", "run and update the golden set", "EPIC-005"),
		cli.Reserved("llm", "report LLM usage and cost", "EPIC-005"),
		cli.Reserved("memory", "rebuild, reset and query the memory index", "EPIC-005"),
		cli.Reserved("privacy", "scan artefacts for external identifiers", "EPIC-005"),
		cli.Reserved("report", "session and audit reports as CSV", "EPIC-005"),
		cli.Reserved("trace", "follow one correlation id through the journal", "EPIC-005"),
	)
}

// runVersion prints the build stamp, the way `multiverse version` does.
func runVersion(args []string, stdout, stderr io.Writer) int {
	flags := cli.FlagSet("mvctl version", stderr)
	if code, ok := cli.Parse(flags, args); !ok {
		return code
	}
	_, _ = io.WriteString(stdout, version+"\n")
	return cli.ExitOK
}
