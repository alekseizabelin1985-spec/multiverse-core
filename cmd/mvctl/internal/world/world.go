// Package world implements `mvctl world init` and `mvctl world status`: a world
// created from the fixtures together with its storage and its first snapshot,
// and the snapshot a world stands on read back (state-and-mechanics.md §4.10,
// EPIC-002 design.md §4.1 I1-8).
//
// A world is created through State and nothing else: the command proposes the
// fixture entities (state.Bootstrap) and asks State for the snapshot, so the
// entities, their commit records and the snapshot are the ones State wrote.
package world

import (
	"fmt"
	"io"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/shared/objstore"
)

// Summary is the line `mvctl help` shows for the command.
const Summary = "create a world from fixtures with its first snapshot, show its snapshot"

// The values of --bus, as the manifest names them (MV_BUS).
const (
	BusKafka  = "kafka"
	BusMemory = "memory"
)

// The values of --store.
const (
	// StoreMinIO is the object store of a deployment.
	StoreMinIO = "minio"
	// StoreMemory is the in-process store: a world that lives as long as the
	// command, for CI and for trying the fixtures out.
	StoreMemory = "memory"
)

// The rules a finding is filed under.
const (
	// CheckStore names a store that could not be reached or prepared.
	CheckStore = "store"
	// CheckRules names a rule book that did not load.
	CheckRules = "rules"
	// CheckFixtures names fixtures a world cannot be created from.
	CheckFixtures = "fixtures"
	// CheckBootstrap names a bootstrap that did not bring the world into being.
	CheckBootstrap = "bootstrap"
	// CheckSnapshot names a snapshot that was not written, or does not match
	// its pointer.
	CheckSnapshot = "snapshot"
	// CheckWorld names a world that is not initialized.
	CheckWorld = "world"
)

// StoreOpener builds the client named by --store.
type StoreOpener func(kind string) (objstore.Client, error)

// Command is `mvctl world`, with the object store it opens. The registry of
// mvctl runs Run; a test builds its own Command over a store it keeps, so that
// a second run sees what the first one wrote.
type Command struct {
	OpenStore StoreOpener
}

// Run dispatches `mvctl world <subcommand>` over the stores of a deployment.
func Run(args []string, stdout, stderr io.Writer) int {
	return Command{OpenStore: OpenStore}.Run(args, stdout, stderr)
}

// Run dispatches `mvctl world <subcommand>`.
func (c Command) Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return cli.UnknownSubcommand(stderr, "mvctl world", "", "init", "status")
	}
	switch args[0] {
	case "init":
		return c.runInit(args[1:], stdout, stderr)
	case "status":
		return c.runStatus(args[1:], stdout, stderr)
	default:
		return cli.UnknownSubcommand(stderr, "mvctl world", args[0], "init", "status")
	}
}

// OpenStore builds the client of a deployment: MinIO from the manifest, or the
// memory store.
func OpenStore(kind string) (objstore.Client, error) {
	switch kind {
	case StoreMemory:
		return objstore.NewMemory(), nil
	case StoreMinIO:
		cfg, err := objstore.ConfigFromEnv()
		if err != nil {
			return nil, err
		}
		return objstore.New(cfg)
	default:
		return nil, fmt.Errorf("unknown store %q, expected %s or %s", kind, StoreMinIO, StoreMemory)
	}
}

// usage prints a wrong call and returns its exit code.
func usage(stderr io.Writer, command, format string, args ...any) int {
	_, _ = fmt.Fprintf(stderr, "mvctl %s: %s\n", command, fmt.Sprintf(format, args...))
	return cli.ExitUsage
}

// noArguments refuses positional arguments after the flags.
func noArguments(stderr io.Writer, command string, rest []string) (int, bool) {
	if len(rest) > 0 {
		return usage(stderr, command, "unexpected argument %q", rest[0]), false
	}
	return cli.ExitOK, true
}
