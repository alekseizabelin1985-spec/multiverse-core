package main

import "multiverse-core.io/cmd/mvctl/internal/cli"

// opsCmds lists the commands of EPIC-005. The owner replaces a reserved line
// with its command here and nothing else (ownership.md v0.6, §3 p. 7).
func opsCmds() []cli.Command {
	return []cli.Command{
		cli.Reserved("golden", "run and update the golden set", "EPIC-005"),
		cli.Reserved("llm", "report LLM usage and cost", "EPIC-005"),
		cli.Reserved("memory", "rebuild, reset and query the memory index", "EPIC-005"),
		cli.Reserved("report", "session and audit reports as CSV", "EPIC-005"),
		cli.Reserved("trace", "follow one correlation id through the journal", "EPIC-005"),
	}
}
