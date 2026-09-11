package main

import (
	"log/slog"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus/membus"
)

// newMemoryBus builds the in-process transport of --bus=memory, the one process
// e2e runs without Docker (foundation.md §2, ADR-010).
//
// membus is the second implementation of C-01, next to the kafka adapter, and
// the real transport of a mode that stays, so it lives in shared/eventbus and
// this file needs no exception from the testkit ban of .golangci.yml (T-418,
// ADR-001 addendum 2026-09-11).
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
