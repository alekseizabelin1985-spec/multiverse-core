package contracts

import "multiverse-core.io/shared/eventbus"

// definitions is the registry of event types. One entry per type, one pull
// request per addition, reviewed by the system architect (contracts.md C-01).
//
// The blocks below follow the split of foundation.md §6: block "а"
// (player.*, group.*, round.*) and block "b" (entity.*, snapshot.created,
// dice.rolled, analytics.replay.completed) came with F-4b-1; block "в"
// (combat.decided, encounter.*, world.*, region.*, npc.*, tick.*, agent.*,
// llm.*, narrative.output, content.incident.recorded, config.cloud_enabled,
// world.laws.changed, world.law_breach.*, analytics.session.*,
// analytics.turn.completed, analytics.consistency.violated) with F-4b-2.
//
// Owner is the epic that owns the schema and the semantics; Publishers are the
// envelope sources actually allowed to publish, which is a different thing
// (contracts.md §0 v0.4, §16 p. 7).
var definitions = []Spec{
	// --- block "а": player actions, groups, rounds (owner EPIC-004) ---
	playerEvent("player.entered_region", []string{SourceSwarm, SourceMemory, SourceLLM}),
	playerEvent("player.left_region", []string{SourceSwarm, SourceMemory}),
	playerEvent("player.looked", []string{SourceSwarm, SourceMemory}),
	playerEvent("player.attacked", []string{SourceSwarm, SourceMemory}),
	playerEvent("player.flee_attempted", []string{SourceSwarm}),
	playerEvent("player.rested", []string{SourceSwarm}),
	playerEvent("player.said", []string{SourceSwarm, SourceMemory}),
	playerEvent("player.defended", []string{SourceSwarm, SourceMemory}),

	gameEvent("group.created", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	gameEvent("group.joined", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	gameEvent("group.left", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	gameEvent("group.leader_changed", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	gameEvent("group.disbanded", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	// A group moves on player_events: it is the only wire format of a group
	// movement, so it travels with the movements of solo players (C-04).
	playerEvent("group.entered_region", []string{SourceSwarm, SourceMemory}),
	playerEvent("group.left_region", []string{SourceSwarm, SourceMemory}),

	gameEvent("round.opened", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm}),
	gameEvent("round.closed", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm}),

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

	// --- block "в": swarm, LLM records, narrative, laws, analytics ---
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

	// Analytics (C-10). The gateway measures the session and the turn; the
	// operator CLI of EPIC-005 reports on them and publishes what its audit
	// found. Nothing here is read during a replay.
	analyticsEvent("analytics.session.started", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceMvctl}),
	analyticsEvent("analytics.session.ended", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceMvctl}),
	analyticsEvent("analytics.turn.completed", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceMvctl}),
	analyticsEvent("analytics.consistency.violated", OwnerOps,
		[]string{SourceMvctl}, []string{SourceMvctl}),

	// --- legacy types (foundation.md §6) ---
	// No payload schema and no meta: only the envelope is checked. They are
	// publishable in the legacy profile alone and go away with it at S5.
	legacyEvent("player.moved", eventbus.TopicPlayerEvents),
	legacyEvent("player.used_skill", eventbus.TopicPlayerEvents),
	legacyEvent("gm.created", eventbus.TopicSystemEvents),
	legacyEvent("gm.deleted", eventbus.TopicSystemEvents),
	legacyEvent("gm.merged", eventbus.TopicSystemEvents),
	legacyEvent("gm.split", eventbus.TopicSystemEvents),
	legacyEvent("narrative.generate", eventbus.TopicNarrativeOutput),
	legacyEvent("violation.detected", eventbus.TopicSystemEvents),
	legacyEvent("time.syncTime", eventbus.TopicSystemEvents),
}

// since names the release a type appeared in. Every type of MVP-1 shares it.
const since = "mvp-1"

// playerEvent describes a type of player_events: the gateway publishes what a
// person, a CI harness or a simulator did, never an agent (policy of the
// topic, C-01).
func playerEvent(typ string, consumers []string) Spec {
	return Spec{
		TypeSpec: eventbus.TypeSpec{
			Topic:         eventbus.TopicPlayerEvents,
			SchemaVersion: 1,
			Policy:        eventbus.PlayerEventsPolicy(),
		},
		Type:       typ,
		Owner:      OwnerGateway,
		Publishers: []string{SourceGateway, SourceTestkitGateway},
		Consumers:  consumers,
		Since:      since,
	}
}

func gameEvent(typ, owner string, publishers, consumers []string) Spec {
	return topicEvent(eventbus.TopicGameEvents, typ, owner, publishers, consumers)
}

func systemEvent(typ, owner string, publishers, consumers []string) Spec {
	return topicEvent(eventbus.TopicSystemEvents, typ, owner, publishers, consumers)
}

func worldEvent(typ, owner string, publishers, consumers []string) Spec {
	return topicEvent(eventbus.TopicWorldEvents, typ, owner, publishers, consumers)
}

// swarmEvent describes a type the swarm runtime publishes, on whichever topic:
// the policy demands meta.agent, because an event of the swarm that does not
// name the agent behind it cannot be replayed, budgeted or audited
// (api-contracts.md §0 and §2.0, C-01 v1.1).
//
// It is not applied to every type of world_events: world.laws.changed comes
// from the author CLI and world.law_breach.* from E-B, neither of which is an
// agent.
func swarmEvent(topic, typ string, publishers, consumers []string) Spec {
	spec := topicEvent(topic, typ, OwnerSwarm, publishers, consumers)
	spec.Policy = eventbus.SwarmPolicy()
	return spec
}

// reserved marks a registered type nothing publishes yet (contracts.md §16
// p. 4). Written as a wrapper rather than a field of the constructors so that
// the table reads as "this one is reserved" at the call site.
func reserved(spec Spec) Spec {
	spec.Reserved = true
	return spec
}

func analyticsEvent(typ, owner string, publishers, consumers []string) Spec {
	return topicEvent(eventbus.TopicAnalyticsEvents, typ, owner, publishers, consumers)
}

func topicEvent(topic, typ, owner string, publishers, consumers []string) Spec {
	return Spec{
		TypeSpec:   eventbus.TypeSpec{Topic: topic, SchemaVersion: 1},
		Type:       typ,
		Owner:      owner,
		Publishers: publishers,
		Consumers:  consumers,
		Since:      since,
	}
}

// legacyEvent describes a type of the as-is code kept alive by the legacy
// profile: no payload schema, no meta, no topic policy.
func legacyEvent(typ, topic string) Spec {
	return Spec{
		TypeSpec:   eventbus.TypeSpec{Topic: topic, SchemaVersion: 1, Deprecated: true},
		Type:       typ,
		Owner:      OwnerFoundation,
		Publishers: []string{SourceLegacy},
		Consumers:  []string{SourceLegacy},
		Since:      "as-is",
	}
}
