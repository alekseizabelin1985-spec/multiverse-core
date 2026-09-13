package main

import "multiverse-core.io/cmd/mvctl/internal/cli"

// swarmCmds lists the commands of EPIC-003. The owner replaces a reserved line
// with its command here and nothing else (ownership.md v0.6, §3 p. 7).
func swarmCmds() []cli.Command {
	return []cli.Command{
		cli.Reserved("blueprint", "validate and inspect agent blueprints", "EPIC-003"),
		cli.Reserved("laws", "read and bump the laws of a world", "EPIC-003"),
		cli.Reserved("record", "record and replay a session", "EPIC-003"),
	}
}
