package contracts

// Types of EPIC-002. The owner adds the line of its type here, reviewed by the
// system architect (contracts.md §16 p. 8); the order of the lists is in
// registry.go.
var stateDefinitions = []Spec{
	// --- block "b": state, snapshots, dice, replay (owner EPIC-002) ---
	// The proposals have four publishers: the gateway, the swarm agents, the
	// CLI loading fixtures and the contract fakes (contracts.md §0 v0.4).
	systemEvent("entity.create.proposed", OwnerState,
		[]string{SourceGateway, SourceSwarm, SourceMvctl, SourceTestkitSwarm, SourceTestkitGateway},
		[]string{SourceState}),
	systemEvent("entity.update.proposed", OwnerState,
		[]string{SourceGateway, SourceSwarm, SourceMvctl, SourceTestkitSwarm, SourceTestkitGateway},
		[]string{SourceState}),
	systemEvent("entity.created", OwnerState,
		[]string{SourceState, SourceTestkitState},
		[]string{SourceGateway, SourceSwarm, SourceMemory, SourceLLM}),
	systemEvent("entity.updated", OwnerState,
		[]string{SourceState, SourceTestkitState},
		[]string{SourceGateway, SourceSwarm, SourceMemory, SourceLLM}),
	systemEvent("entity.update.rejected", OwnerState,
		[]string{SourceState, SourceTestkitState},
		[]string{SourceGateway, SourceSwarm, SourceMemory}),
	// Every stateful context publishes its own snapshot; the consumers read
	// the latest.json pointer at start (C-14 v1.1).
	systemEvent("snapshot.created", OwnerState,
		[]string{SourceState, SourceSwarm, SourceGateway},
		[]string{SourceState, SourceSwarm, SourceGateway, SourceMvctl}),
	// The type belongs to EPIC-002 (mechanics), the encounter agent publishes
	// it (C-03), and FakeEncounter does the same on membus.
	gameEvent("dice.rolled", OwnerState,
		[]string{SourceSwarm, SourceTestkitSwarm}, []string{SourceMvctl}),
	// One schema for both modes: State signals the end of its recovery, the
	// EPIC-005 harness the end of a test replay (C-14 v0.4).
	analyticsEvent("analytics.replay.completed", OwnerState,
		[]string{SourceState, SourceMvctl, SourceTestkitState},
		[]string{SourceSwarm, SourceGateway, SourceMvctl}),
}
