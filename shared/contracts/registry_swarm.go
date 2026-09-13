package contracts

import "multiverse-core.io/shared/eventbus"

// Types of EPIC-003. The owner adds the line of its type here, reviewed by the
// system architect (contracts.md §16 p. 8); the order of the lists is in
// registry.go.
var swarmDefinitions = []Spec{
	// --- block "в": swarm, LLM records, narrative, laws ---
	// The encounter (owner EPIC-003). The type is published by the encounter
	// agent and, until EPIC-003 I1a lands, by FakeEncounter on membus.
	swarmEvent(eventbus.TopicGameEvents, "combat.decided",
		[]string{SourceSwarm, SourceTestkitSwarm},
		[]string{SourceGateway, SourceSwarm, SourceMemory}),
	swarmEvent(eventbus.TopicWorldEvents, "encounter.started",
		[]string{SourceSwarm, SourceTestkitSwarm},
		[]string{SourceGateway, SourceSwarm, SourceMemory}),
	swarmEvent(eventbus.TopicWorldEvents, "encounter.ended",
		[]string{SourceSwarm, SourceTestkitSwarm},
		[]string{SourceGateway, SourceSwarm, SourceMemory}),

	// The living world: the global GM tells what happened to the world, the
	// domain GM what happened in its region. Memory indexes all of it.
	swarmEvent(eventbus.TopicWorldEvents, "world.weather_changed",
		[]string{SourceSwarm}, []string{SourceSwarm, SourceMemory}),
	swarmEvent(eventbus.TopicWorldEvents, "world.time_advanced",
		[]string{SourceSwarm}, []string{SourceSwarm, SourceMemory}),
	swarmEvent(eventbus.TopicWorldEvents, "world.event_occurred",
		[]string{SourceSwarm}, []string{SourceSwarm, SourceMemory}),
	swarmEvent(eventbus.TopicWorldEvents, "region.event_occurred",
		[]string{SourceSwarm}, []string{SourceSwarm, SourceMemory}),
	swarmEvent(eventbus.TopicWorldEvents, "npc.moved",
		[]string{SourceSwarm}, []string{SourceSwarm, SourceMemory}),
	swarmEvent(eventbus.TopicWorldEvents, "npc.spawned",
		[]string{SourceSwarm}, []string{SourceSwarm, SourceMemory}),

	// The scheduler and the life cycle of the swarm. mvctl reads them for
	// session-report and for a replay of what the scheduler decided.
	swarmEvent(eventbus.TopicSystemEvents, "tick.fired",
		[]string{SourceSwarm}, []string{SourceSwarm, SourceMvctl}),
	swarmEvent(eventbus.TopicSystemEvents, "tick.aborted",
		[]string{SourceSwarm}, []string{SourceSwarm, SourceMvctl}),
	swarmEvent(eventbus.TopicSystemEvents, "agent.spawned",
		[]string{SourceSwarm}, []string{SourceMvctl, SourceMemory}),
	swarmEvent(eventbus.TopicSystemEvents, "agent.child_resolved",
		[]string{SourceSwarm}, []string{SourceMvctl, SourceMemory}),
	swarmEvent(eventbus.TopicSystemEvents, "agent.stopped",
		[]string{SourceSwarm}, []string{SourceMvctl, SourceMemory}),
	swarmEvent(eventbus.TopicSystemEvents, "agent.spawn_rejected",
		[]string{SourceSwarm}, []string{SourceMvctl, SourceMemory}),
	swarmEvent(eventbus.TopicSystemEvents, "agent.blueprint_reloaded",
		[]string{SourceSwarm}, []string{SourceMvctl, SourceMemory}),

	// The record of every call of the model (C-07). The gateway of the LLM
	// writes it; RecordingWriter of the testkit writes the fixtures the
	// recorded provider replays. Memory reads the metadata only.
	swarmEvent(eventbus.TopicLLMRecords, "llm.output",
		[]string{SourceLLM, SourceTestkitSwarm},
		[]string{SourceSwarm, SourceMvctl, SourceMemory}),
	swarmEvent(eventbus.TopicLLMRecords, "llm.output.rejected",
		[]string{SourceLLM, SourceTestkitSwarm},
		[]string{SourceSwarm, SourceMvctl, SourceMemory}),
	swarmEvent(eventbus.TopicSystemEvents, "content.incident.recorded",
		[]string{SourceLLM}, []string{SourceMvctl, SourceMemory}),

	// The one text a player ever sees. FakeNarrator publishes it until the
	// personal GM of EPIC-003 does.
	swarmEvent(eventbus.TopicNarrativeOutput, "narrative.output",
		[]string{SourceSwarm, SourceTestkitSwarm},
		[]string{SourceGateway, SourceMemory}),

	// The cloud flag, published on every start of core so that the projection
	// of the gateway is deterministic after a restart (C-06 v1.1). No agent
	// is involved: the context itself publishes it, hence no swarm policy.
	systemEvent("config.cloud_enabled", OwnerSwarm,
		[]string{SourceLLM}, []string{SourceGateway, SourceMvctl}),

	// The laws of the world. The author bumps them from the CLI; the breach
	// mechanics of E-B will do the same from core/laws, which is why the type
	// carries no swarm policy.
	worldEvent("world.laws.changed", OwnerSwarm,
		[]string{SourceMvctl, SourceLaws},
		[]string{SourceSwarm, SourceState, SourceMemory}),
	// The breach family is registered and valid, and nothing publishes it in
	// MVP-1: the exception of contracts.md §16 p. 4 and C-12, marked so that
	// mvctl contracts check can tell a reserved type from a dead one.
	reserved(worldEvent("world.law_breach.proposed", OwnerSwarm,
		[]string{SourceLaws}, []string{SourceLaws})),
	reserved(worldEvent("world.law_breach.rejected", OwnerSwarm,
		[]string{SourceLaws}, []string{SourceLaws})),
	reserved(worldEvent("world.law_breach.applied", OwnerSwarm,
		[]string{SourceLaws}, []string{SourceLaws})),
	reserved(worldEvent("world.law_breach.review_decided", OwnerSwarm,
		[]string{SourceLaws}, []string{SourceLaws})),
	reserved(worldEvent("world.law_breach.rolled_back", OwnerSwarm,
		[]string{SourceLaws}, []string{SourceLaws})),
}
