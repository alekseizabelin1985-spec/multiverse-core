package main

import (
	"bytes"
	"strings"
	"testing"

	"multiverse-core.io/cmd/mvctl/internal/cli"
)

// TestRunDispatches walks the registry from the outside, the way CI and an
// operator do: the exit code is the contract, not the text.
func TestRunDispatches(t *testing.T) {
	cases := map[string]struct {
		args []string
		want int
	}{
		"contracts check":  {[]string{"contracts", "check"}, cli.ExitOK},
		"contracts topics": {[]string{"contracts", "topics", "--format=rpk"}, cli.ExitOK},
		"storage init":     {[]string{"storage", "init", "--store=memory"}, cli.ExitOK},
		// The path is relative to the package directory the test binary runs
		// in; the job of CI runs the same command against testdata/ from the
		// repository root.
		"privacy scan":     {[]string{"privacy", "scan", "../../testdata"}, cli.ExitOK},
		"version":          {[]string{"version"}, cli.ExitOK},
		"help":             {[]string{"help"}, cli.ExitOK},
		"no arguments":     {nil, cli.ExitUsage},
		"unknown command":  {[]string{"nope"}, cli.ExitUsage},
		"reserved command": {[]string{"world", "init"}, cli.ExitUsage},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != tc.want {
				t.Errorf("exit code %d, want %d\nstdout: %s\nstderr: %s",
					code, tc.want, stdout.String(), stderr.String())
			}
		})
	}
}

// TestEnvCheckRunsFromTheRepositoryRoot is the DoD line `make contracts` runs.
// The command reads .env.example relative to the working directory, and the
// test binary runs in the package directory, so the path is given explicitly.
func TestEnvCheckOnTheShippedExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"env", "check", "--file=../../.env.example"}, &stdout, &stderr)
	if code != cli.ExitOK {
		t.Fatalf("exit code %d\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
}

// TestReservedNamesAreHeld is why the table lists commands nobody implemented:
// two epics must not discover at merge time that they both called their command
// "report" (design.md §4.1, decomposition-review.md §5.2 p. 4).
func TestReservedNamesAreHeld(t *testing.T) {
	registry := commands()
	for _, name := range []string{
		"world", "blueprint", "laws", "record",
		"golden", "llm", "memory", "report", "trace",
	} {
		cmd, ok := registry.Lookup(name)
		if !ok {
			t.Errorf("the name %q is not reserved", name)
			continue
		}
		if cmd.Owner == "" {
			t.Errorf("%s: the reserved name does not say which epic owns it", name)
		}
	}
}

// TestImplementedCommandsHaveNoOwnerMark keeps the help readable: a command that
// works must not be listed as reserved.
func TestImplementedCommandsHaveNoOwnerMark(t *testing.T) {
	registry := commands()
	for _, name := range []string{"contracts", "env", "privacy", "storage", "version"} {
		cmd, ok := registry.Lookup(name)
		if !ok {
			t.Fatalf("%s is not registered", name)
		}
		if cmd.Owner != "" {
			t.Errorf("%s is implemented but marked reserved for %s", name, cmd.Owner)
		}
		if cmd.Summary == "" {
			t.Errorf("%s has no summary in the command list", name)
		}
	}
}

func TestVersionIsPrinted(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"version"}, &stdout, &stderr); code != cli.ExitOK {
		t.Fatalf("exit code %d", code)
	}
	if strings.TrimSpace(stdout.String()) != version {
		t.Errorf("printed %q, want %q", stdout.String(), version)
	}
}
