package main

import (
	"log/slog"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/testkit/membus"
)

// newMemoryBus builds the in-process transport of --bus=memory, the one process
// e2e runs without Docker (foundation.md §2, ADR-010).
//
// This file is one of the two places the binary reaches into shared/testkit,
// and it is a file of its own for that reason: .golangci.yml lets exactly this
// file import exactly shared/testkit/membus, so no other double can come in
// through the same door. The other is fake_contexts.go, the hook of I1-α that
// mounts the FAKE swarm context (T-255) and leaves again with T-256. The memory
// bus is the real transport of a mode that stays.
//
// The topics are the real list of the registry, as the contract test of the bus
// passes them: an empty list would make membus create topics on first use,
// which the broker of the platform never does (auto_create_topics_enabled is
// false), and the two modes would then disagree about an unknown topic.
func newMemoryBus(reg *contracts.Registry, skipValidateOnRead bool, timers clock.Timers, log *slog.Logger) (transport, error) {
	specs := reg.Topics()
	topics := make([]string, 0, len(specs))
	for _, t := range specs {
		topics = append(topics, t.Name)
	}
	return membus.New(membus.Config{
		Registry:           reg,
		Topics:             topics,
		SkipValidateOnRead: skipValidateOnRead,
		Timers:             timers,
		Log:                log,
	})
}
