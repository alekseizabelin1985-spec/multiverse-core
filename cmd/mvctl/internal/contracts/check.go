package contracts

import (
	"slices"
	"sort"
	"strings"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	sharedcontracts "multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

// The five rules of `mvctl contracts check` (design.md §4.1 a-д). The names are
// what a finding is filed under, so they are stable: a CI log that says
// "registry" means the same thing next release.
const (
	CheckSchemas  = "schemas"
	CheckRegistry = "registry"
	CheckSources  = "sources"
	CheckTopics   = "topics"
	CheckPolicies = "policies"
)

// Input is the whole world of the check: the registry entries, the topic map,
// the schema files on disk and the source values a publisher may carry.
//
// The check works on this rather than on contracts.Default() so that a test can
// plant a phantom — a spec without a schema, a schema without a spec, a topic
// nobody publishes to — and watch the command refuse it. A check that can only
// be run against the real registry is a check nobody can prove wrong.
type Input struct {
	Specs  []sharedcontracts.Spec
	Topics []sharedcontracts.TopicSpec
	// SchemaFiles are the file names under schemas/events, "_common.json" and
	// "_envelope.json" included: the check knows to skip them.
	SchemaFiles []string
	// KnownSources are the accepted values of Event.Source.
	KnownSources []string
}

// Check runs every rule over in and returns what it disagreed with. The order
// is stable — rule by rule, subject by subject — so two runs of the contracts
// job produce the same diff.
func Check(in Input) []cli.Finding {
	report := cli.NewReport("check")
	checkRegistry(report, in)
	checkSources(report, in)
	checkTopics(report, in)
	checkPolicies(report, in)
	return report.Findings
}

// schemaOwner is the type a schema file belongs to, and the file name.
type schemaOwner struct {
	typ  string
	file string
}

// checkRegistry is rule (б): every registered type has its schema file and
// every schema file has its type. A schema nobody registered is a type that
// will never be published; a spec without a schema is a payload nobody checks.
//
// The files are held by name rather than by type, because a type may have more
// than one file on disk: the version the registry is at and the one a version
// bump left behind. Keyed by type the map would keep only the last of them and
// the leftover would be invisible to the check for good.
func checkRegistry(report *cli.Report, in Input) {
	files := map[string]schemaOwner{}
	byType := map[string][]string{}
	for _, file := range in.SchemaFiles {
		name := strings.TrimPrefix(file, "events/")
		if strings.HasPrefix(name, "_") {
			// _common.json and _envelope.json belong to no type.
			continue
		}
		typ, ok := typeOfSchemaFile(name)
		if !ok {
			report.Add(CheckSchemas, name, "a schema file is named <type>.v<n>.json")
			continue
		}
		files[name] = schemaOwner{typ: typ, file: name}
		byType[typ] = append(byType[typ], name)
	}

	// The version each type is registered at, so that a file left in files can
	// be told apart afterwards: a stale version of a live type, or a type
	// nobody registered at all.
	registered := map[string]int{}
	for _, spec := range in.Specs {
		switch {
		case spec.Type == "":
			report.Add(CheckRegistry, "(unnamed)", "a registry entry without a type")
			continue
		case spec.Deprecated:
			// A legacy type carries no payload schema by contract
			// (foundation.md §6); only its envelope is checked.
			if names := byType[spec.Type]; len(names) > 0 {
				report.Add(CheckRegistry, spec.Type,
					"marked deprecated but a payload schema file exists for it")
				forget(files, names)
			}
			continue
		}
		registered[spec.Type] = spec.SchemaVersion
		want := sharedcontracts.SchemaFile(spec.Type, spec.SchemaVersion)
		names := byType[spec.Type]
		switch {
		case len(names) == 0:
			report.Addf(CheckRegistry, spec.Type, "registered without the schema file %s", want)
		case !slices.Contains(names, want):
			// The drift finding already names every file of the type; leaving
			// them in the map would report each of them a second time.
			report.Addf(CheckRegistry, spec.Type,
				"registered at schema version %d, but the file on disk is %s",
				spec.SchemaVersion, strings.Join(names, ", "))
			forget(files, names)
		default:
			delete(files, want)
		}
	}

	for _, name := range sortedKeys(files) {
		typ := files[name].typ
		if version, ok := registered[typ]; ok {
			report.Addf(CheckRegistry, name,
				"a schema version of %s nobody registers any more: the registry is at version %d",
				typ, version)
			continue
		}
		report.Addf(CheckRegistry, name, "schema file of a type nobody registered (%s)", typ)
	}
}

// forget drops the named files from the map of what is still unaccounted for.
func forget(files map[string]schemaOwner, names []string) {
	for _, name := range names {
		delete(files, name)
	}
}

// checkSources is rule (в): a type nobody may publish is dead, a type nobody
// reads is noise (contracts.md §16 p. 4, p. 7). It also checks that the sources
// named exist at all — the check `source ∈ Spec.Publishers` that runs in the
// contracts job is worth nothing if Publishers holds a value no publisher can
// ever write.
func checkSources(report *cli.Report, in Input) {
	for _, spec := range in.Specs {
		if spec.Type == "" || spec.Deprecated {
			continue
		}
		if spec.Owner == "" {
			report.Add(CheckSources, spec.Type, "no schema owner")
		}
		// A reserved type is registered before anybody publishes or reads it
		// (contracts.md §16 p. 4). It still names who will, so the lists are
		// checked for typos below either way.
		if !spec.Reserved {
			if len(spec.Publishers) == 0 {
				report.Add(CheckSources, spec.Type, "no publisher: nobody may publish this type")
			}
			if len(spec.Consumers) == 0 {
				report.Add(CheckSources, spec.Type, "no consumer: nobody reads this type")
			}
		}
		reportUnknownSources(report, spec.Type, "publisher", spec.Publishers, in.KnownSources)
		reportUnknownSources(report, spec.Type, "consumer", spec.Consumers, in.KnownSources)
	}
}

func reportUnknownSources(report *cli.Report, typ, role string, sources, known []string) {
	for _, source := range sources {
		if slices.Contains(known, source) {
			continue
		}
		report.Addf(CheckSources, typ, "%s %q is not an envelope source of the platform", role, source)
	}
}

// checkTopics is the topic half of rule (б): a type goes to a topic of the
// map, and a topic of the map carries types. dead_letters is the one topic
// without a type by design — the bus writes the DeadLetter wrapper there
// itself, and the wrapper is not an event with a type of its own.
func checkTopics(report *cli.Report, in Input) {
	known := map[string]sharedcontracts.TopicSpec{}
	for _, topic := range in.Topics {
		if topic.Name == "" {
			report.Add(CheckTopics, "(unnamed)", "a topic of the map without a name")
			continue
		}
		if _, dup := known[topic.Name]; dup {
			report.Add(CheckTopics, topic.Name, "listed twice in the topic map")
		}
		known[topic.Name] = topic
		if topic.RetentionMS <= 0 {
			report.Addf(CheckTopics, topic.Name,
				"retention.ms is %d: redpanda-init would create a topic that keeps nothing",
				topic.RetentionMS)
		}
	}

	counted := map[string]int{}
	for _, spec := range in.Specs {
		if spec.Type == "" {
			continue
		}
		if spec.Topic == "" {
			report.Add(CheckTopics, spec.Type, "no topic")
			continue
		}
		if _, ok := known[spec.Topic]; !ok {
			report.Addf(CheckTopics, spec.Type, "topic %q is not in the topic map", spec.Topic)
			continue
		}
		counted[spec.Topic]++
	}

	for _, name := range sortedTopicNames(in.Topics) {
		if name == eventbus.TopicDeadLetters {
			if counted[name] > 0 {
				report.Add(CheckTopics, name, "a type is routed to the dead letter topic")
			}
			continue
		}
		if counted[name] == 0 {
			report.Add(CheckTopics, name, "no registered type goes to this topic")
		}
	}
}

// topicPolicies is the policy every type of a topic must carry
// (api-contracts.md §2.0, C-01 v1.1, foundation.md §6): player_events carries
// what a person, a harness or a simulator did and never what an agent decided;
// llm_records and narrative_output carry what an agent produced and must name
// it, or the record cannot be replayed, budgeted or audited.
//
// The other four topics are mixed on purpose — the author bumping the laws
// through the CLI and the tick of the swarm share system_events — so they have
// no topic-wide rule to check a type against.
var topicPolicies = map[string]eventbus.Policy{
	eventbus.TopicPlayerEvents:    eventbus.PlayerEventsPolicy(),
	eventbus.TopicLLMRecords:      eventbus.SwarmPolicy(),
	eventbus.TopicNarrativeOutput: eventbus.SwarmPolicy(),
}

// checkPolicies is rule (г): the policy a type carries is the policy its topic
// demands, held against fixtures rather than compared field by field. The
// fixtures are built here instead of read from a file because what is under
// test is the policy, not a payload: an envelope claiming actor_kind=system on
// player_events must be turned away, and one naming no agent on llm_records
// must be too.
func checkPolicies(report *cli.Report, in Input) {
	for _, spec := range in.Specs {
		if spec.Type == "" || spec.Deprecated {
			// A legacy envelope predates meta; its policy is not applied.
			continue
		}
		for _, kind := range spec.Policy.ActorKinds {
			if !slices.Contains(actorKinds, kind) {
				report.Addf(CheckPolicies, spec.Type,
					"policy admits actor_kind %q, which is not one of %s",
					kind, strings.Join(actorKinds, ", "))
			}
		}
		want, ruled := topicPolicies[spec.Topic]
		if !ruled {
			continue
		}
		for _, fixture := range policyFixtures(spec.Type) {
			accepted := spec.Policy.Check(fixture.event) == nil
			shouldAccept := want.Check(fixture.event) == nil
			switch {
			case accepted && !shouldAccept:
				report.Addf(CheckPolicies, spec.Type,
					"policy accepts %s, which %s does not allow", fixture.name, spec.Topic)
			case !accepted && shouldAccept:
				report.Addf(CheckPolicies, spec.Type,
					"policy refuses %s, which %s allows", fixture.name, spec.Topic)
			}
		}
	}
}

// policyFixture is one envelope a policy is held against.
type policyFixture struct {
	name  string
	event eventbus.Event
}

// actorKinds are the four values of meta.actor_kind (api-contracts.md §2.1).
var actorKinds = []string{
	eventbus.ActorHuman, eventbus.ActorCI, eventbus.ActorSim, eventbus.ActorSystem,
}

// policyFixtures builds the eight envelopes of a type: every actor kind, with
// and without an agent. Only the fields a policy reads are filled — the payload
// and its schema are the business of Validate, which the tests of
// shared/contracts run over the examples of api-contracts.md §2.3.
func policyFixtures(typ string) []policyFixture {
	agent := &eventbus.AgentRef{ID: "agent-1", Level: "task", Blueprint: "fixture"}
	fixtures := make([]policyFixture, 0, 2*len(actorKinds))
	for _, kind := range actorKinds {
		fixtures = append(fixtures,
			policyFixture{
				name:  "actor_kind=" + kind + " without meta.agent",
				event: eventbus.Event{Type: typ, Meta: eventbus.Meta{ActorKind: kind}},
			},
			policyFixture{
				name: "actor_kind=" + kind + " with meta.agent",
				event: eventbus.Event{
					Type: typ,
					Meta: eventbus.Meta{ActorKind: kind, Agent: agent},
				},
			})
	}
	return fixtures
}

// typeOfSchemaFile splits "player.looked.v1.json" into its type.
func typeOfSchemaFile(name string) (string, bool) {
	trimmed, ok := strings.CutSuffix(name, ".json")
	if !ok {
		return "", false
	}
	typ, version, ok := cutLast(trimmed, ".v")
	if !ok || typ == "" || version == "" {
		return "", false
	}
	for _, r := range version {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return typ, true
}

// cutLast splits s around the last occurrence of sep.
func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

// sortedKeys returns the file names of the map in a stable order.
func sortedKeys(m map[string]schemaOwner) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedTopicNames(topics []sharedcontracts.TopicSpec) []string {
	names := make([]string, 0, len(topics))
	for _, topic := range topics {
		if topic.Name != "" {
			names = append(names, topic.Name)
		}
	}
	sort.Strings(names)
	return slices.Compact(names)
}
