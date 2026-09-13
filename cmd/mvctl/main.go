// Command mvctl is the operator and CI command line of the platform: it checks
// contracts, prints the target configuration of the cluster, reads the
// environment manifest and prepares the object store (foundation.md §12,
// design.md §4.1).
//
// The registry of subcommands is put together here from one list per owner:
// foundation() below (EPIC-001) and stateCmds, swarmCmds and opsCmds in
// commands_<owner>.go. Five epics fill it, so the names of the commands that
// are not written yet are reserved in the file of their owner: a reserved name
// exits with the usage code and says which epic owns it, which is cheaper than
// discovering at merge time that two teams both called their command "report".
//
// An owner replaces its reserved line with the command in its own file
// (contracts.md §16 p. 8, ownership.md v0.6). This file and the order of the
// lists change only through a task of EPIC-001; the help lists the commands
// alphabetically whatever the order.
package main

import (
	"io"
	"os"
	"slices"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	contractscmd "multiverse-core.io/cmd/mvctl/internal/contracts"
	envcmd "multiverse-core.io/cmd/mvctl/internal/env"
	privacycmd "multiverse-core.io/cmd/mvctl/internal/privacy"
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

// commands is the registry: the lists of the owners, the implemented commands
// of the foundation first.
func commands() *cli.Registry {
	return cli.NewRegistry(slices.Concat(foundation(), stateCmds(), swarmCmds(), opsCmds())...)
}

// foundation lists the commands of EPIC-001.
func foundation() []cli.Command {
	return []cli.Command{
		{
			Name:    "contracts",
			Summary: contractscmd.Summary,
			Run:     contractscmd.Run,
		},
		{
			Name:    "env",
			Summary: envcmd.Summary,
			Run:     envcmd.Run,
		},
		{
			Name:    "storage",
			Summary: storagecmd.Summary,
			Run:     storagecmd.Run,
		},
		// The minimal `privacy scan` belongs to F-7 rather than to EPIC-005:
		// the security job of CI runs it from wave 0 on, and EPIC-005 (T-139)
		// replaces the implementation behind the same name (decision ОВ-47).
		{
			Name:    "privacy",
			Summary: privacycmd.Summary,
			Run:     privacycmd.Run,
		},
		{
			Name:    "version",
			Summary: "print the version this binary was built from",
			Run:     runVersion,
		},
	}
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
