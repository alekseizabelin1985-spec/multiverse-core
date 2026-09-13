package eventbus

import (
	"errors"
	"fmt"
	"slices"
)

// Errors reported by routing, validation and the bus implementations.
var (
	// ErrUnknownType is returned for a type nobody registered: publishing it
	// is a defect of the publisher, not a runtime condition (ADR-007 p. 2).
	ErrUnknownType = errors.New("eventbus: unknown event type")
	// ErrInvalidEnvelope marks an envelope missing a mandatory field.
	ErrInvalidEnvelope = errors.New("eventbus: invalid envelope")
	// ErrPolicyViolation marks an event that the policy of its topic rejects.
	ErrPolicyViolation = errors.New("eventbus: topic policy violation")
	// ErrNoRegistry marks a bus built without a contract registry.
	ErrNoRegistry = errors.New("eventbus: no contract registry configured")
	// ErrClosed is returned by a bus that has been closed.
	ErrClosed = errors.New("eventbus: bus is closed")
	// ErrHandlerPanic is the error a dead letter carries when the handler
	// panicked (C-01 v1.5); the panic value follows it in the text.
	ErrHandlerPanic = errors.New("eventbus: handler panic")
)

// TypeSpec is the part of a registry entry the bus itself needs: where the
// type goes, which payload schema version it carries, and what its topic
// demands of the envelope.
type TypeSpec struct {
	Topic         string
	SchemaVersion int
	Policy        Policy
	// Deprecated marks a legacy type that has no payload schema and predates
	// the meta envelope: only the envelope is checked, neither the schema nor
	// the topic policy (foundation.md §6). Legacy types are publishable in the
	// legacy profile only and go away with it.
	Deprecated bool
}

// Registry resolves an event type to its contract.
//
// The interface lives here rather than in shared/contracts because the
// registry validates eventbus.Event and therefore imports this package: the
// bus depends on the narrow interface, the registry on the envelope.
type Registry interface {
	// Lookup reports the contract of a type; false for an unknown type.
	Lookup(eventType string) (TypeSpec, bool)
	// Validate checks the envelope and the payload against the JSON schema of
	// the type.
	Validate(ev Event) error
}

// AgentRule states what the policy of a topic demands of Meta.Agent.
type AgentRule uint8

const (
	// AgentOptional accepts an envelope with or without an agent.
	AgentOptional AgentRule = iota
	// AgentRequired rejects an envelope without an agent: the swarm topics
	// must name the agent that produced the event.
	AgentRequired
	// AgentForbidden rejects an envelope with an agent: player_events carries
	// what a person did, never what an agent decided.
	AgentForbidden
)

// WorldRule states what the policy of a type demands of Event.World (C-01
// v1.10).
type WorldRule uint8

const (
	// WorldOptional accepts an envelope with or without a world. It is the
	// zero value, so every policy written before the rule keeps its behaviour.
	WorldOptional WorldRule = iota
	// WorldRequired rejects an envelope without a world or with an empty
	// World.Entity.ID. The publisher that forgot the world learns it from
	// Publish, in its own process; a check in the consumer would only leave a
	// Warn in the log of somebody else's.
	WorldRequired
)

// Policy is the policy of a type, checked on publish always and on read when
// validation on read is on (ADR-007 addendum p. 4, SEC-16).
type Policy struct {
	// ActorKinds lists the accepted Meta.ActorKind values; empty accepts any
	// of the four.
	ActorKinds []string
	Agent      AgentRule
	World      WorldRule
}

// PlayerEventsPolicy is the policy of player_events: a person, a CI harness or
// a simulator acted, and no agent is involved.
func PlayerEventsPolicy() Policy {
	return Policy{ActorKinds: []string{ActorHuman, ActorCI, ActorSim}, Agent: AgentForbidden}
}

// SwarmPolicy is the policy of the swarm types (llm_records, tick.*, agent.*,
// narrative.output, combat.decided): the agent that produced the event must be
// named.
func SwarmPolicy() Policy {
	return Policy{Agent: AgentRequired}
}

// Check reports whether the envelope satisfies the policy.
func (p Policy) Check(ev Event) error {
	if len(p.ActorKinds) > 0 && !slices.Contains(p.ActorKinds, ev.Meta.ActorKind) {
		return fmt.Errorf("%w: actor_kind %q not in %v", ErrPolicyViolation, ev.Meta.ActorKind, p.ActorKinds)
	}
	switch p.Agent {
	case AgentRequired:
		if ev.Meta.Agent == nil {
			return fmt.Errorf("%w: meta.agent is required for %s", ErrPolicyViolation, ev.Type)
		}
	case AgentForbidden:
		if ev.Meta.Agent != nil {
			return fmt.Errorf("%w: meta.agent is not allowed for %s", ErrPolicyViolation, ev.Type)
		}
	case AgentOptional:
	}
	switch p.World {
	case WorldRequired:
		if ev.World == nil {
			return fmt.Errorf("%w: world is required for %s", ErrPolicyViolation, ev.Type)
		}
		if ev.World.Entity.ID == "" {
			return fmt.Errorf("%w: world.entity.id is empty for %s", ErrPolicyViolation, ev.Type)
		}
	case WorldOptional:
	}
	return nil
}

// ValidateEnvelope checks the fields the envelope must carry whatever the
// type is. The payload is the business of the JSON schema in the registry.
func (e Event) ValidateEnvelope() error {
	switch {
	case e.ID == "":
		return fmt.Errorf("%w: id is empty", ErrInvalidEnvelope)
	case e.Type == "":
		return fmt.Errorf("%w: type is empty", ErrInvalidEnvelope)
	case e.Source == "":
		return fmt.Errorf("%w: source is empty", ErrInvalidEnvelope)
	case e.Timestamp.IsZero():
		return fmt.Errorf("%w: timestamp is zero", ErrInvalidEnvelope)
	}
	// A legacy envelope carries no meta at all; the registry decides whether
	// the type is allowed to be one, this check only rejects a half-filled
	// meta, which is always a defect.
	if e.Meta == (Meta{}) {
		return nil
	}
	if e.Meta.CorrelationID == "" {
		return fmt.Errorf("%w: meta.correlation_id is empty", ErrInvalidEnvelope)
	}
	switch e.Meta.ActorKind {
	case ActorHuman, ActorCI, ActorSim, ActorSystem:
	default:
		return fmt.Errorf("%w: meta.actor_kind %q", ErrInvalidEnvelope, e.Meta.ActorKind)
	}
	// gm_path is mandatory like the fields above (api-contracts §2.1): the
	// schema of T-006 lists it in required, so an envelope without it would
	// be parked in dead_letters on read anyway — better to refuse it at the
	// publisher, where the defect is.
	switch e.Meta.GMPath {
	case GMPathAgent, GMPathLegacy:
	default:
		return fmt.Errorf("%w: meta.gm_path %q", ErrInvalidEnvelope, e.Meta.GMPath)
	}
	return nil
}

// Route resolves the topic of an event and runs every check that must pass
// before it is sent: the envelope, the topic policy, the payload schema. Both
// bus implementations call it, so that publishing behaves identically on
// kafka and on membus.
func Route(reg Registry, ev Event) (string, error) {
	if err := ev.ValidateEnvelope(); err != nil {
		return "", err
	}
	if reg == nil {
		return "", ErrNoRegistry
	}
	spec, ok := reg.Lookup(ev.Type)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnknownType, ev.Type)
	}
	if spec.Deprecated {
		// A legacy producer fills neither meta nor a payload schema; the
		// envelope is all there is to check until the legacy profile is gone.
		return spec.Topic, nil
	}
	if err := spec.Policy.Check(ev); err != nil {
		return "", err
	}
	if err := reg.Validate(ev); err != nil {
		return "", fmt.Errorf("eventbus: validate %s: %w", ev.Type, err)
	}
	return spec.Topic, nil
}
