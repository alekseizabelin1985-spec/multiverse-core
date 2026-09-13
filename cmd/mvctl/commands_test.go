package main

import (
	"testing"

	"multiverse-core.io/cmd/mvctl/internal/cli"
)

// A name reserved in the file of one owner and marked for another would send
// the operator to the wrong epic, and the owner who finally writes the command
// would look for its line in the wrong file (T-446).
func TestReservedNamesSitInTheFileOfTheirOwner(t *testing.T) {
	lists := map[string][]cli.Command{
		"EPIC-002": stateCmds(),
		"EPIC-003": swarmCmds(),
		"EPIC-005": opsCmds(),
	}
	for owner, list := range lists {
		for _, cmd := range list {
			if cmd.Owner != "" && cmd.Owner != owner {
				t.Errorf("%s is reserved for %s in the list of %s", cmd.Name, cmd.Owner, owner)
			}
		}
	}
	for _, cmd := range foundation() {
		if cmd.Owner != "" {
			t.Errorf("%s in the list of EPIC-001 is reserved for %s", cmd.Name, cmd.Owner)
		}
	}
}
