package contracts

import (
	"slices"

	"multiverse-core.io/shared/eventbus"
)

// definitions is the registry of event types. One entry per type, one pull
// request per addition, reviewed by the system architect (contracts.md C-01).
//
// The table is put together here from the files of the owners of the types
// (contracts.md §16 p. 8, ownership.md v0.6): registry_gateway.go (EPIC-004),
// registry_state.go (EPIC-002), registry_swarm.go (EPIC-003) and
// registry_ops.go (EPIC-005); the legacy types of EPIC-001 stay below. An
// owner adds the line of its type to its own file; this file and the order of
// the lists change only through a task of EPIC-001.
//
// The lists follow the split of foundation.md §6: block "а" (player.*,
// group.*, round.*) and block "b" (entity.*, snapshot.created, dice.rolled,
// analytics.replay.completed) came with F-4b-1; block "в" (combat.decided,
// encounter.*, world.*, region.*, npc.*, tick.*, agent.*, llm.*,
// narrative.output, content.incident.recorded, config.cloud_enabled,
// world.laws.changed, world.law_breach.*, analytics.session.*,
// analytics.turn.completed, analytics.consistency.violated) with F-4b-2. The
// analytics of the gateway is a list of its own so that the analytics types
// read together, which keeps the declaration order of All() what it was before
// the split (T-446).
//
// Owner is the epic that owns the schema and the semantics; Publishers are the
// envelope sources actually allowed to publish, which is a different thing
// (contracts.md §0 v0.4, §16 p. 7).
var definitions = slices.Concat(
	gatewayDefinitions,
	stateDefinitions,
	swarmDefinitions,
	gatewayAnalyticsDefinitions,
	opsDefinitions,
	legacyDefinitions,
)

// legacyDefinitions are the types of the as-is code (owner EPIC-001).
var legacyDefinitions = []Spec{
	// --- legacy types (foundation.md §6) ---
	// No payload schema and no meta: only the envelope is checked. They are
	// publishable in the legacy profile alone and go away with it at S5.
	legacyEvent("player.moved", eventbus.TopicPlayerEvents),
	legacyEvent("player.used_skill", eventbus.TopicPlayerEvents),
	// With MV_GM_PATH=legacy the gateway publishes gm.created beside the
	// services of the profile (contracts.md C-04 v1.4).
	legacyEvent("gm.created", eventbus.TopicSystemEvents, SourceGateway),
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
// profile: no payload schema, no meta, no topic policy. The services of the
// profile publish it; publishers names the new sources that publish it beside
// them.
func legacyEvent(typ, topic string, publishers ...string) Spec {
	return Spec{
		TypeSpec:   eventbus.TypeSpec{Topic: topic, SchemaVersion: 1, Deprecated: true},
		Type:       typ,
		Owner:      OwnerFoundation,
		Publishers: append([]string{SourceLegacy}, publishers...),
		Consumers:  []string{SourceLegacy},
		Since:      "as-is",
	}
}
