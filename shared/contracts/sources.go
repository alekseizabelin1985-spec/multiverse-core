package contracts

import (
	"slices"

	"multiverse-core.io/shared/eventbus"
)

// The envelope source values of MVP-1. A publisher writes one of these into
// Event.Source, and the registry lists in Spec.Publishers which of them may
// publish which type (contracts.md §0 v0.4).
//
// The shape is <process>/<context> for the contexts of core, the bare command
// name for a CLI, and testkit/<owner> for a contract fake. Fakes are allowed
// on membus and in core with MV_SWARM_FAKE=true only — the exception for
// stubs of contracts.md §0.
const (
	SourceGateway = "core/gateway"
	SourceState   = "core/state"
	SourceSwarm   = "core/swarm"
	SourceLLM     = "core/llm"
	SourceLaws    = "core/laws"
	SourceMemory  = "core/memory"
	// SourceBus is the library itself: it wraps what could not be handled
	// into dead_letters.
	SourceBus = "core/bus"
	// SourceMvctl is the operator CLI: world init --fixtures, replay, laws.
	SourceMvctl = "mvctl"

	SourceTestkitState   = "testkit/state"
	SourceTestkitSwarm   = "testkit/swarm"
	SourceTestkitGateway = "testkit/gateway"

	// SourceLegacy covers the services of the legacy profile (agent_mode=off).
	// They publish the deprecated types below and go away with the profile at
	// milestone S5.
	SourceLegacy = "legacy"
)

// The owners of schemas and semantics (contracts.md §0). The owner of a type
// maintains its schema file and its Publishers list; other epics revise, they
// do not create.
const (
	OwnerFoundation = "EPIC-001"
	OwnerState      = "EPIC-002"
	OwnerSwarm      = "EPIC-003"
	OwnerGateway    = "EPIC-004"
	OwnerOps        = "EPIC-005"
)

// knownSources is every value Event.Source may carry. Publishers and Consumers
// of the registry are checked against it by mvctl contracts check: a source
// nobody can produce is a typo, and a typo in Publishers turns the static
// check of contracts.md §16 p. 7 into a check of nothing.
//
// The list is written by hand and kept complete by the test of this file: a
// Source* constant missing from it would make contracts check blame a correct
// Publishers entry, in another job and for the wrong reason.
var knownSources = []string{
	SourceGateway, SourceState, SourceSwarm, SourceLLM, SourceLaws, SourceMemory,
	SourceBus, SourceMvctl,
	SourceTestkitState, SourceTestkitSwarm, SourceTestkitGateway,
	SourceLegacy,
}

// KnownSources returns the envelope source values of MVP-1.
func KnownSources() []string { return slices.Clone(knownSources) }

// Retention of the topics in milliseconds (api-contracts.md §2.2, decision U-3).
const (
	retention30d = 30 * 24 * 60 * 60 * 1000
	retention90d = 90 * 24 * 60 * 60 * 1000
	retention180 = 180 * 24 * 60 * 60 * 1000

	// llmRecordsMaxMessageBytes is 4 MiB: llm_records carries whole prompts
	// and completions (infrastructure.md v0.3 §5.1, ОВ-37).
	llmRecordsMaxMessageBytes = 4 * 1024 * 1024
)

// topics is the whole topic map of MVP-1, including the topics whose types
// arrive with the schemas of block "в" (T-009): redpanda-init creates all
// eight from the first run.
var topics = []TopicSpec{
	{Name: eventbus.TopicPlayerEvents, RetentionMS: retention30d, ReplayRead: true},
	{Name: eventbus.TopicGameEvents, RetentionMS: retention30d, ReplayRead: true},
	{Name: eventbus.TopicWorldEvents, RetentionMS: retention30d, ReplayRead: true},
	{Name: eventbus.TopicSystemEvents, RetentionMS: retention30d, ReplayRead: true},
	{Name: eventbus.TopicNarrativeOutput, RetentionMS: retention30d, ReplayRead: true},
	{Name: eventbus.TopicLLMRecords, RetentionMS: retention90d, ReplayRead: true,
		MaxMessageBytes: llmRecordsMaxMessageBytes},
	{Name: eventbus.TopicAnalyticsEvents, RetentionMS: retention180, ReplayRead: false},
	{Name: eventbus.TopicDeadLetters, RetentionMS: retention30d, ReplayRead: false},
}
