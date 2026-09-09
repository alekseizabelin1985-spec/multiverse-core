// Package contracts is the registry of event types: which topic a type goes
// to, which schema its payload obeys, who may publish it and who reads it
// (contracts.md C-01 v1.1, foundation.md §6, ADR-007).
//
// The registry is a Go table rather than a YAML file on purpose: adding a type
// is one reviewed pull request, the compiler catches the typos, and a test
// checks that every entry has a schema file and every schema file an entry.
package contracts

import (
	"errors"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"multiverse-core.io/shared/eventbus"
)

// ErrInvalidPayload marks a payload that does not match the schema of its
// type. An invalid envelope is reported with eventbus.ErrInvalidEnvelope, so
// that a consumer can tell the two apart without parsing messages.
var ErrInvalidPayload = errors.New("contracts: invalid payload")

// ErrNoSchema marks a registered type whose schema file is missing from
// schemas/events: a defect of the registry, caught when it is built.
var ErrNoSchema = errors.New("contracts: no schema file for type")

// Spec is the contract of one event type.
//
// It embeds eventbus.TypeSpec — the part the bus itself needs (topic, schema
// version, topic policy, deprecation) — instead of repeating those fields, so
// that the registry and the bus cannot drift apart.
type Spec struct {
	eventbus.TypeSpec

	// Type is the event type, e.g. player.attacked.
	Type string
	// Owner is the epic that owns the schema and the semantics of the type
	// (contracts.md §0). The owner is not necessarily the only publisher.
	Owner string
	// Publishers lists the envelope source values allowed to publish the type
	// (contracts.md §0 v0.4, §16 p. 7). The check source ∈ Publishers is
	// static: it runs in the contracts job and in mvctl contracts check, not
	// on the hot path of Publish.
	Publishers []string
	// Consumers lists the components that read the type. A type with no
	// consumer is a defect, except for the reserved families named in
	// contracts.md §16 p. 4.
	Consumers []string
	// Reserved marks a type whose schema is registered before anything
	// publishes or reads it: world.law_breach.* waits for E-B (C-12), and the
	// exception is named in contracts.md §16 p. 4. Publishers and Consumers
	// still name the component that will do it, so that the check of the
	// contracts job has something to compare a source against once it exists.
	// It is this flag that mvctl contracts check reads to skip the "one
	// publisher and at least one consumer" rule.
	Reserved bool
	// Since is the release the type appeared in.
	Since string
	// Schema is the compiled payload schema, nil for a deprecated legacy type
	// that has none.
	Schema *jsonschema.Schema
}

// TopicSpec describes a topic of the platform: the single source for
// redpanda-init and for mvctl contracts topics (foundation.md §6).
type TopicSpec struct {
	Name        string
	RetentionMS int64
	// ReplayRead reports whether the topic is read during a replay. Analytics
	// and dead letters are not: they describe a run, they do not constitute it.
	ReplayRead bool
}
