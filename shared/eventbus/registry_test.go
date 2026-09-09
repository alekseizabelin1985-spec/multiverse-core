package eventbus

import (
	"errors"
	"testing"
	"time"
)

func TestPlayerEventsPolicyRejectsTheSwarm(t *testing.T) {
	deterministicSources(t)
	policy := PlayerEventsPolicy()

	human := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	if err := policy.Check(human); err != nil {
		t.Errorf("a human action is rejected by player_events: %v", err)
	}

	system := NewRoot("player.attacked", "gateway", "w1", nil, ActorSystem, nil)
	if err := policy.Check(system); !errors.Is(err, ErrPolicyViolation) {
		t.Errorf("actor_kind=system accepted on player_events: %v", err)
	}

	agent := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil,
		WithAgent(AgentRef{ID: "personal-gm:player-A", Level: "task", Blueprint: "personal-gm"}))
	if err := policy.Check(agent); !errors.Is(err, ErrPolicyViolation) {
		t.Errorf("meta.agent accepted on player_events: %v", err)
	}
}

func TestSwarmPolicyDemandsAnAgent(t *testing.T) {
	deterministicSources(t)
	policy := SwarmPolicy()

	bare := NewRoot("tick.fired", "core/swarm", "w1", nil, ActorSystem, nil)
	if err := policy.Check(bare); !errors.Is(err, ErrPolicyViolation) {
		t.Errorf("tick.fired without meta.agent accepted: %v", err)
	}

	withAgent := NewRoot("tick.fired", "core/swarm", "w1", nil, ActorSystem, nil,
		WithAgent(AgentRef{ID: "global-gm", Level: "strategic", Blueprint: "global-gm"}))
	if err := policy.Check(withAgent); err != nil {
		t.Errorf("tick.fired with meta.agent rejected: %v", err)
	}

	// Any actor kind is fine on the swarm topics; only the agent is demanded.
	if err := (Policy{}).Check(bare); err != nil {
		t.Errorf("the empty policy rejects something: %v", err)
	}
}

func TestValidateEnvelopeRejectsHalfFilledMeta(t *testing.T) {
	deterministicSources(t)
	valid := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)

	cases := map[string]func(*Event){
		"no id":            func(e *Event) { e.ID = "" },
		"no type":          func(e *Event) { e.Type = "" },
		"no source":        func(e *Event) { e.Source = "" },
		"zero timestamp":   func(e *Event) { e.Timestamp = time.Time{} },
		"no correlation":   func(e *Event) { e.Meta.CorrelationID = "" },
		"bad actor kind":   func(e *Event) { e.Meta.ActorKind = "operator" },
		"empty actor kind": func(e *Event) { e.Meta.ActorKind = "" },
		"bad gm path":      func(e *Event) { e.Meta.GMPath = "oracle" },
		"no gm path":       func(e *Event) { e.Meta.GMPath = "" },
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			ev := valid
			corrupt(&ev)
			if err := ev.ValidateEnvelope(); !errors.Is(err, ErrInvalidEnvelope) {
				t.Errorf("accepted a broken envelope (%s): %v", name, err)
			}
		})
	}

	if err := valid.ValidateEnvelope(); err != nil {
		t.Errorf("a well-formed envelope is rejected: %v", err)
	}
}

func TestValidateEnvelopeAcceptsALegacyEnvelope(t *testing.T) {
	deterministicSources(t)
	// A legacy producer sends no meta at all: only the registry decides
	// whether such a type is still allowed.
	if err := NewEvent("player.moved", "legacy-orchestrator", "w1", nil).ValidateEnvelope(); err != nil {
		t.Errorf("a legacy envelope is rejected: %v", err)
	}
}

func TestRouteResolvesTheTopicFromTheRegistry(t *testing.T) {
	deterministicSources(t)
	reg := testRegistry()

	ev := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	topic, err := Route(reg, ev)
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if topic != TopicPlayerEvents {
		t.Errorf("topic = %q, want %q", topic, TopicPlayerEvents)
	}
}

func TestRouteRejectsAnUnknownType(t *testing.T) {
	deterministicSources(t)

	ev := NewRoot("player.teleported", "gateway", "w1", nil, ActorHuman, nil)
	if _, err := Route(testRegistry(), ev); !errors.Is(err, ErrUnknownType) {
		t.Errorf("an unregistered type was routed: %v", err)
	}
}

func TestRouteAppliesTheTopicPolicy(t *testing.T) {
	deterministicSources(t)

	ev := NewRoot("player.attacked", "gateway", "w1", nil, ActorSystem, nil)
	if _, err := Route(testRegistry(), ev); !errors.Is(err, ErrPolicyViolation) {
		t.Errorf("the policy of player_events was not applied: %v", err)
	}
}

func TestRouteValidatesThePayloadExceptForDeprecatedTypes(t *testing.T) {
	deterministicSources(t)
	errSchema := errors.New("payload does not match the schema")
	reg := testRegistry()
	reg.validate = func(Event) error { return errSchema }

	live := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	if _, err := Route(reg, live); !errors.Is(err, errSchema) {
		t.Errorf("the payload schema was not applied: %v", err)
	}

	// A deprecated type has no payload schema: the envelope is all there is.
	legacy := NewEvent("player.moved", "legacy-orchestrator", "w1", nil)
	if _, err := Route(reg, legacy); err != nil {
		t.Errorf("a deprecated type was validated against a schema: %v", err)
	}
}

func TestRouteWithoutARegistryFails(t *testing.T) {
	deterministicSources(t)

	ev := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	if _, err := Route(nil, ev); !errors.Is(err, ErrNoRegistry) {
		t.Errorf("routing without a registry succeeded: %v", err)
	}
}
