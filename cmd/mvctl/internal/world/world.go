// Package world implements `mvctl world init` and `mvctl world status`: a world
// created from the fixtures together with its storage and its first snapshot,
// and the snapshot a world stands on read back (state-and-mechanics.md §4.10,
// EPIC-002 design.md §4.1 I1-8).
//
// A world is created through State and nothing else: the command proposes the
// fixture entities (state.Bootstrap) and asks State for the snapshot, so the
// entities, their commit records and the snapshot are the ones State wrote.
// With --bus memory that State runs inside the command; with --bus kafka it is
// the State of the running core, which the command reaches over Redpanda and
// its admin route.
package world

import (
	"fmt"
	"io"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
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
	// CheckCore names a core that could not be reached or does not serve the
	// world, before anything was proposed.
	CheckCore = "core"
	// CheckBus names a bus that could not be opened.
	CheckBus = "bus"
)

// StoreOpener builds the client named by --store.
type StoreOpener func(kind string) (objstore.Client, error)

// Transport is the bus of --bus kafka: one object that is both the live side
// and the journal, so that the answers are read off the log the proposals go
// into.
type Transport interface {
	eventbus.Bus
	eventbus.Journal
}

// BusOpener builds the transport of --bus kafka over the registry of the
// command.
type BusOpener func(reg *contracts.Registry) (Transport, error)

// Command is `mvctl world`, with what it opens. The registry of mvctl runs Run;
// a test builds its own Command over a store and a bus it keeps, so that a
// second run sees what the first one wrote, and over a core of its own.
type Command struct {
	OpenStore StoreOpener
	// OpenBus builds the bus of --bus kafka. Without it that path opens
	// nothing and reports so: a Command of a test never reaches the brokers of
	// the environment by accident.
	OpenBus BusOpener
	// CoreURL is the address of the HTTP server of core, whose State the path
	// of --bus kafka asks for the snapshot (MV_CORE_URL). Empty is refused for
	// the same reason as a missing OpenBus.
	CoreURL string
}

// Run dispatches `mvctl world <subcommand>` over the stores, the bus and the
// core of a deployment.
func Run(args []string, stdout, stderr io.Writer) int {
	return Command{OpenStore: OpenStore, OpenBus: OpenBus, CoreURL: env.CoreURL.String()}.Run(args, stdout, stderr)
}

// OpenBus builds the bus of a deployment from the manifest: the brokers of
// MV_KAFKA_BROKERS, validation on read as MV_BUS_VALIDATE_ON_READ says (SEC-16).
func OpenBus(reg *contracts.Registry) (Transport, error) {
	validate, err := env.BusValidateOnRead.Bool()
	if err != nil {
		return nil, err
	}
	return eventbus.NewKafka(eventbus.KafkaConfig{
		Brokers:            env.KafkaBrokers.List(),
		Registry:           reg,
		SkipValidateOnRead: !validate,
		Timers:             clock.RealTimers{},
		Log:                discard(),
	})
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
