package contracts

import "multiverse-core.io/shared/eventbus"

// definitions is the registry of event types. One entry per type, one pull
// request per addition, reviewed by the system architect (contracts.md C-01).
//
// The blocks below follow the split of foundation.md §6: block "а"
// (player.*, group.*, round.*) and block "b" (entity.*, snapshot.created,
// dice.rolled, analytics.replay.completed) are registered here by F-4b-1;
// block "в" (agent.*, tick.*, llm.*, narrative.output, world.*, region.*,
// npc.*, encounter.*, content.incident.recorded, analytics.session.*,
// analytics.turn.completed, analytics.consistency.violated) arrives with
// F-4b-2 (T-009), which appends to this same table.
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
