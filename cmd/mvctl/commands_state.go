package main

import "multiverse-core.io/cmd/mvctl/internal/cli"

// stateCmds lists the commands of EPIC-002. The owner replaces a reserved line
// with its command here and nothing else (ownership.md v0.6, §3 p. 7).
func stateCmds() []cli.Command {
	return []cli.Command{
		cli.Reserved("world", "create a world, load fixtures, write a snapshot", "EPIC-002"),
	}
}
