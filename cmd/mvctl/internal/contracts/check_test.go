package contracts_test

import (
	"bytes"
	"strings"
	"testing"

	"multiverse-core.io/cmd/mvctl/internal/cli"
	mvcontracts "multiverse-core.io/cmd/mvctl/internal/contracts"
	sharedcontracts "multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
)

// synthetic is a registry that agrees with itself: one player type, one swarm
// type, the two topics they go to and the dead letter topic. Every test below
// breaks exactly one thing in it and watches the check catch that one thing.
func synthetic() mvcontracts.Input {
	return mvcontracts.Input{
		Specs: []sharedcontracts.Spec{
			{
				TypeSpec: eventbus.TypeSpec{
					Topic:         eventbus.TopicPlayerEvents,
					SchemaVersion: 1,
					Policy:        eventbus.PlayerEventsPolicy(),
				},
				Type:       "player.attacked",
				Owner:      sharedcontracts.OwnerGateway,
				Publishers: []string{sharedcontracts.SourceGateway},
				Consumers:  []string{sharedcontracts.SourceSwarm},
			},
			{
				TypeSpec: eventbus.TypeSpec{
					Topic:         eventbus.TopicLLMRecords,
					SchemaVersion: 2,
					Policy:        eventbus.SwarmPolicy(),
				},
				Type:       "llm.output",
				Owner:      sharedcontracts.OwnerSwarm,
				Publishers: []string{sharedcontracts.SourceLLM},
				Consumers:  []string{sharedcontracts.SourceMemory},
			},
		},
		Topics: []sharedcontracts.TopicSpec{
			{Name: eventbus.TopicPlayerEvents, RetentionMS: 1, ReplayRead: true},
			{Name: eventbus.TopicLLMRecords, RetentionMS: 2, ReplayRead: true},
			{Name: eventbus.TopicDeadLetters, RetentionMS: 3},
		},
		SchemaFiles: []string{
			"events/_common.json",
			"events/_envelope.json",
			"events/player.attacked.v1.json",
			"events/llm.output.v2.json",
		},
		KnownSources: sharedcontracts.KnownSources(),
	}
}

// findingsOn runs the check and returns the messages, so that a test asserts on
// what an operator would read rather than on a count.
func findingsOn(t *testing.T, in mvcontracts.Input) []string {
	t.Helper()
	var messages []string
	for _, finding := range mvcontracts.Check(in) {
		messages = append(messages, finding.String())
	}
	return messages
}

func containsAll(got []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, g := range got {
			if strings.Contains(g, w) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestConsistentRegistryHasNoFindings(t *testing.T) {
	if got := findingsOn(t, synthetic()); len(got) != 0 {
		t.Errorf("a consistent registry produced findings: %v", got)
	}
}

// TestPhantomTypeAndPhantomSchema is the rule the task is named after: a
// registered type without a schema file, and a schema file nobody registered,
// are both defects and both have to be reported, in both directions.
func TestPhantomTypeAndPhantomSchema(t *testing.T) {
	t.Run("spec without a schema file", func(t *testing.T) {
		in := synthetic()
		in.SchemaFiles = []string{"events/llm.output.v2.json"}
		got := findingsOn(t, in)
		if !containsAll(got, "player.attacked: registered without the schema file player.attacked.v1.json") {
			t.Errorf("the phantom type was not reported: %v", got)
		}
	})

	t.Run("schema file without a spec", func(t *testing.T) {
		in := synthetic()
		in.SchemaFiles = append(in.SchemaFiles, "events/world.vanished.v1.json")
		got := findingsOn(t, in)
		if !containsAll(got, "world.vanished.v1.json: schema file of a type nobody registered") {
			t.Errorf("the phantom schema was not reported: %v", got)
		}
	})

	t.Run("schema version drifted", func(t *testing.T) {
		in := synthetic()
		in.Specs[1].SchemaVersion = 3
		got := findingsOn(t, in)
		if !containsAll(got, "llm.output: registered at schema version 3") {
			t.Errorf("the version drift was not reported: %v", got)
		}
	})

	// The half of the rule that only opens on the first version bump: the file
	// of the previous version stays in schemas/events forever, and the registry
	// alone cannot see it — contracts.New only asks whether the file it needs is
	// there, never whether anything else is.
	t.Run("schema version left behind by a bump", func(t *testing.T) {
		in := synthetic()
		// In the order fs.Glob returns them — v1 before v2 — which is the
		// order that used to leave the registered version in the map and hide
		// the leftover behind it.
		in.SchemaFiles = []string{
			"events/_common.json",
			"events/_envelope.json",
			"events/llm.output.v1.json",
			"events/llm.output.v2.json",
			"events/player.attacked.v1.json",
		}
		got := findingsOn(t, in)
		if !containsAll(got,
			"llm.output.v1.json: a schema version of llm.output nobody registers any more") {
			t.Errorf("the leftover schema file was not reported: %v", got)
		}
		if len(got) != 1 {
			t.Errorf("the version in use was reported as well: %v", got)
		}
	})

	t.Run("schema file misnamed", func(t *testing.T) {
		in := synthetic()
		in.SchemaFiles = append(in.SchemaFiles, "events/broken.json")
		got := findingsOn(t, in)
		if !containsAll(got, "broken.json: a schema file is named <type>.v<n>.json") {
			t.Errorf("the misnamed file was not reported: %v", got)
		}
	})
}

func TestTypeWithoutPublisherOrConsumer(t *testing.T) {
	in := synthetic()
	in.Specs[0].Publishers = nil
	in.Specs[1].Consumers = nil
	got := findingsOn(t, in)
	if !containsAll(got,
		"player.attacked: no publisher",
		"llm.output: no consumer") {
		t.Errorf("an orphan type was accepted: %v", got)
	}
}

// TestReservedTypeSkipsThePublisherRule is the machine form of the exception of
// contracts.md §16 p. 4: world.law_breach.* is registered before E-B and must
// not be reported as a phantom for having no publisher yet.
func TestReservedTypeSkipsThePublisherRule(t *testing.T) {
	in := synthetic()
	in.Specs[0].Reserved = true
	in.Specs[0].Publishers = nil
	in.Specs[0].Consumers = nil

	if got := findingsOn(t, in); len(got) != 0 {
		t.Errorf("a reserved type was reported: %v", got)
	}

	// The exemption is narrow: it does not extend to a source that does not
	// exist, because that is a typo either way.
	in.Specs[0].Publishers = []string{"core/nowhere"}
	got := findingsOn(t, in)
	if !containsAll(got, `publisher "core/nowhere" is not an envelope source`) {
		t.Errorf("a typo in a reserved type was accepted: %v", got)
	}
}

// TestUnknownSourceIsReported is what makes `source ∈ Spec.Publishers` worth
// running: a Publishers entry no publisher can ever write turns the check into
// a check of nothing (review of T-009).
func TestUnknownSourceIsReported(t *testing.T) {
	in := synthetic()
	in.Specs[0].Publishers = []string{"core/getaway"}
	in.Specs[1].Consumers = []string{sharedcontracts.SourceMemory, "testkit/nobody"}

	got := findingsOn(t, in)
	if !containsAll(got,
		`player.attacked: publisher "core/getaway" is not an envelope source`,
		`llm.output: consumer "testkit/nobody" is not an envelope source`) {
		t.Errorf("an invented source was accepted: %v", got)
	}
}

func TestTypeWithoutOwner(t *testing.T) {
	in := synthetic()
	in.Specs[0].Owner = ""
	if got := findingsOn(t, in); !containsAll(got, "player.attacked: no schema owner") {
		t.Errorf("a type without an owner was accepted: %v", got)
	}
}

func TestTopicChecks(t *testing.T) {
	t.Run("type routed to a topic that is not in the map", func(t *testing.T) {
		in := synthetic()
		in.Specs[0].Topic = "scope_management"
		got := findingsOn(t, in)
		if !containsAll(got, `player.attacked: topic "scope_management" is not in the topic map`) {
			t.Errorf("a phantom topic was accepted: %v", got)
		}
	})

	t.Run("topic nobody publishes to", func(t *testing.T) {
		in := synthetic()
		in.Topics = append(in.Topics,
			sharedcontracts.TopicSpec{Name: "scope_management", RetentionMS: 1})
		got := findingsOn(t, in)
		if !containsAll(got, "scope_management: no registered type goes to this topic") {
			t.Errorf("a topic redpanda-init would create for nothing was accepted: %v", got)
		}
	})

	t.Run("dead letters carry no type", func(t *testing.T) {
		in := synthetic()
		if got := findingsOn(t, in); len(got) != 0 {
			t.Fatalf("dead_letters was reported as unused: %v", got)
		}
		in.Specs[0].Topic = eventbus.TopicDeadLetters
		got := findingsOn(t, in)
		if !containsAll(got, "dead_letters: a type is routed to the dead letter topic") {
			t.Errorf("a type on dead_letters was accepted: %v", got)
		}
	})

	t.Run("topic that keeps nothing", func(t *testing.T) {
		in := synthetic()
		in.Topics[0].RetentionMS = 0
		got := findingsOn(t, in)
		if !containsAll(got, "player_events: retention.ms is 0") {
			t.Errorf("a topic without retention was accepted: %v", got)
		}
	})
}

// TestPolicyFixtures is rule (г): the policy a type carries is the policy its
// topic demands, and a type that quietly drops it is caught on synthetic
// envelopes, without a broker and without a payload.
func TestPolicyFixtures(t *testing.T) {
	t.Run("the shipped policies agree with their topics", func(t *testing.T) {
		if got := findingsOn(t, synthetic()); len(got) != 0 {
			t.Fatalf("the shipped policies were reported: %v", got)
		}
	})

	t.Run("a player type open to system events", func(t *testing.T) {
		in := synthetic()
		in.Specs[0].Policy = eventbus.Policy{}
		got := findingsOn(t, in)
		if !containsAll(got,
			"player.attacked: policy accepts actor_kind=system without meta.agent",
			"player.attacked: policy accepts actor_kind=human with meta.agent") {
			t.Errorf("a player type open to system events and to agents was accepted: %v", got)
		}
	})

	t.Run("an llm record that names no agent", func(t *testing.T) {
		in := synthetic()
		in.Specs[1].Policy = eventbus.Policy{}
		got := findingsOn(t, in)
		if !containsAll(got, "llm.output: policy accepts actor_kind=human without meta.agent") {
			t.Errorf("an anonymous llm record was accepted: %v", got)
		}
	})

	t.Run("a policy that refuses the actor kinds of its topic", func(t *testing.T) {
		in := synthetic()
		in.Specs[0].Policy = eventbus.Policy{
			ActorKinds: []string{eventbus.ActorHuman},
			Agent:      eventbus.AgentForbidden,
		}
		got := findingsOn(t, in)
		if !containsAll(got, "player.attacked: policy refuses actor_kind=ci without meta.agent") {
			t.Errorf("a policy narrower than its topic was accepted: %v", got)
		}
	})

	t.Run("a policy naming an actor kind that does not exist", func(t *testing.T) {
		in := synthetic()
		in.Specs[0].Policy = eventbus.Policy{ActorKinds: []string{"operator"}}
		got := findingsOn(t, in)
		if !containsAll(got, `player.attacked: policy admits actor_kind "operator"`) {
			t.Errorf("an invented actor kind was accepted: %v", got)
		}
	})
}

// TestDeprecatedTypes documents the deal with the legacy profile: an as-is type
// carries no payload schema and no policy, and neither absence is a finding —
// but a schema file for one is, because nothing would ever validate against it.
func TestDeprecatedTypes(t *testing.T) {
	in := synthetic()
	in.Specs = append(in.Specs, sharedcontracts.Spec{
		TypeSpec: eventbus.TypeSpec{Topic: eventbus.TopicPlayerEvents, Deprecated: true},
		Type:     "player.moved",
	})
	if got := findingsOn(t, in); len(got) != 0 {
		t.Errorf("a legacy type was reported: %v", got)
	}

	in.SchemaFiles = append(in.SchemaFiles, "events/player.moved.v1.json")
	got := findingsOn(t, in)
	if !containsAll(got, "player.moved: marked deprecated but a payload schema file exists") {
		t.Errorf("a schema for a legacy type was accepted: %v", got)
	}
}

func TestUnnamedEntries(t *testing.T) {
	in := synthetic()
	in.Specs = append(in.Specs, sharedcontracts.Spec{})
	in.Topics = append(in.Topics, sharedcontracts.TopicSpec{})
	got := findingsOn(t, in)
	if !containsAll(got, "a registry entry without a type", "a topic of the map without a name") {
		t.Errorf("an empty entry was accepted: %v", got)
	}
}

func TestDuplicateTopic(t *testing.T) {
	in := synthetic()
	in.Topics = append(in.Topics,
		sharedcontracts.TopicSpec{Name: eventbus.TopicPlayerEvents, RetentionMS: 1})
	got := findingsOn(t, in)
	if !containsAll(got, "player_events: listed twice in the topic map") {
		t.Errorf("a duplicated topic was accepted: %v", got)
	}
}

// TestRunCheckOnTheRealRegistry is the end the DoD names: the command run
// against what the binary actually carries reports no phantom.
func TestRunCheckOnTheRealRegistry(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := mvcontracts.Run([]string{"check"}, &stdout, &stderr)
	if code != cli.ExitOK {
		t.Fatalf("contracts check exits %d\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "types") {
		t.Errorf("the summary does not say what was checked: %q", stdout.String())
	}
}

func TestRunRejectsWrongUsage(t *testing.T) {
	cases := map[string][]string{
		"no subcommand":           nil,
		"unknown subcommand":      {"validate"},
		"unknown flag":            {"check", "--strict"},
		"stray argument":          {"check", "schemas/"},
		"stray argument (topics)": {"topics", "all"},
		"unknown format":          {"topics", "--format=yaml"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := mvcontracts.Run(args, &stdout, &stderr); code != cli.ExitUsage {
				t.Errorf("exit code %d, want %d (stderr: %s)", code, cli.ExitUsage, stderr.String())
			}
		})
	}
}
