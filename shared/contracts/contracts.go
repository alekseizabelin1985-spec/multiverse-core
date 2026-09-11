package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"multiverse-core.io/schemas"
	"multiverse-core.io/shared/eventbus"
)

// schemaBase is the identifier space of the schema files: it matches the $id
// of every file under schemas/events, so that a relative $ref resolves to the
// embedded copy and never reaches the network.
const schemaBase = "https://multiverse-core.io/schemas/events/"

// envelopeFile holds the envelope every event obeys whatever its type is.
const envelopeFile = "_envelope.json"

// Registry resolves a type to its contract and validates events against it.
// It satisfies eventbus.Registry, which is how the bus routes and checks what
// it publishes and reads.
type Registry struct {
	specs    map[string]Spec
	ordered  []Spec
	envelope *jsonschema.Schema
}

var _ eventbus.Registry = (*Registry)(nil)

// New builds a registry from a file system rooted where schemas/events lives
// (schemas.FS in production, a fixture tree in a test). Every schema is
// compiled once, here: validating an event afterwards is a few tens of
// microseconds and needs no I/O.
func New(fsys fs.FS) (*Registry, error) {
	compiler := jsonschema.NewCompiler()
	// Assert formats: a timestamp that is not RFC 3339 is a defect of the
	// publisher, and the schema is the only place that says so.
	compiler.AssertFormat()

	files, err := fs.Glob(fsys, "events/*.json")
	if err != nil {
		return nil, fmt.Errorf("contracts: list schemas: %w", err)
	}
	for _, file := range files {
		doc, err := readSchema(fsys, file)
		if err != nil {
			return nil, err
		}
		if err := compiler.AddResource(schemaBase+path.Base(file), doc); err != nil {
			return nil, fmt.Errorf("contracts: add %s: %w", file, err)
		}
	}

	envelope, err := compiler.Compile(schemaBase + envelopeFile)
	if err != nil {
		return nil, fmt.Errorf("contracts: compile %s: %w", envelopeFile, err)
	}

	reg := &Registry{
		specs:    make(map[string]Spec, len(definitions)),
		ordered:  make([]Spec, 0, len(definitions)),
		envelope: envelope,
	}
	for _, spec := range definitions {
		if _, ok := reg.specs[spec.Type]; ok {
			return nil, fmt.Errorf("contracts: duplicate type %s", spec.Type)
		}
		if !spec.Deprecated {
			name := SchemaFile(spec.Type, spec.SchemaVersion)
			if !slices.Contains(files, "events/"+name) {
				return nil, fmt.Errorf("%w: %s (%s)", ErrNoSchema, spec.Type, name)
			}
			schema, err := compiler.Compile(schemaBase + name)
			if err != nil {
				return nil, fmt.Errorf("contracts: compile %s: %w", name, err)
			}
			spec.Schema = schema
		}
		reg.specs[spec.Type] = spec
		reg.ordered = append(reg.ordered, spec)
	}
	return reg, nil
}

// SchemaFile is the file name of the payload schema of a type, relative to
// schemas/events.
func SchemaFile(typ string, version int) string {
	return fmt.Sprintf("%s.v%d.json", typ, version)
}

// Lookup reports the part of the contract the bus needs. It is the
// eventbus.Registry method; Spec returns the whole entry.
func (r *Registry) Lookup(typ string) (eventbus.TypeSpec, bool) {
	spec, ok := r.specs[typ]
	if !ok {
		return eventbus.TypeSpec{}, false
	}
	return spec.TypeSpec, true
}

// Spec reports the whole contract of a type. The Publishers and Consumers of
// the returned copy are the caller's own: the registry is read from several
// goroutines and must not be rewritable through a slice it handed out.
func (r *Registry) Spec(typ string) (Spec, bool) {
	spec, ok := r.specs[typ]
	if !ok {
		return Spec{}, false
	}
	return cloneSpec(spec), true
}

// All returns a copy of every registered contract in declaration order.
func (r *Registry) All() []Spec {
	all := make([]Spec, len(r.ordered))
	for i, spec := range r.ordered {
		all[i] = cloneSpec(spec)
	}
	return all
}

// cloneSpec detaches the slices of a contract from the package table, the way
// OwnershipRules does. Schema is shared on purpose: a compiled schema is
// immutable and compiling it again per call would cost more than validating.
func cloneSpec(spec Spec) Spec {
	spec.Publishers = slices.Clone(spec.Publishers)
	spec.Consumers = slices.Clone(spec.Consumers)
	// Policy travels inside the embedded TypeSpec and carries a slice of its
	// own; leaving it shared would keep the same hole open one field deeper.
	spec.Policy.ActorKinds = slices.Clone(spec.Policy.ActorKinds)
	return spec
}

// Topics returns the topic map of the platform.
func (r *Registry) Topics() []TopicSpec {
	return slices.Clone(topics)
}

// Validate checks the envelope of an event and then its payload against the
// schema of its type. A deprecated type carries neither meta nor a payload
// schema, so only the envelope rules of eventbus apply to it (foundation.md
// §6) — and they apply here too: Validate is exported and is called directly
// by mvctl contracts check and by fixtures, not only through the bus.
func (r *Registry) Validate(ev eventbus.Event) error {
	spec, ok := r.specs[ev.Type]
	if !ok {
		return fmt.Errorf("%w: %s", eventbus.ErrUnknownType, ev.Type)
	}
	if spec.Deprecated {
		return ev.ValidateEnvelope()
	}
	doc, err := jsonValue(ev)
	if err != nil {
		return err
	}
	if err := r.envelope.Validate(doc); err != nil {
		return fmt.Errorf("%w: %w", eventbus.ErrInvalidEnvelope, err)
	}
	object, _ := doc.(map[string]any)
	if err := spec.Schema.Validate(object["payload"]); err != nil {
		return fmt.Errorf("%w %s: %w", ErrInvalidPayload, ev.Type, err)
	}
	return nil
}

// ValidateEnvelope checks the envelope alone, without the payload schema. The
// bus uses it for what it reads out of dead_letters and for fixtures.
func (r *Registry) ValidateEnvelope(ev eventbus.Event) error {
	doc, err := jsonValue(ev)
	if err != nil {
		return err
	}
	if err := r.envelope.Validate(doc); err != nil {
		return fmt.Errorf("%w: %w", eventbus.ErrInvalidEnvelope, err)
	}
	return nil
}

// jsonValue renders an event the way it travels on the wire and decodes it
// back into the generic form the validator works on. Going through JSON is
// the point: what is validated is what consumers receive, not the Go struct.
func jsonValue(ev eventbus.Event) (any, error) {
	raw, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("contracts: marshal event %s: %w", ev.Type, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("contracts: decode event %s: %w", ev.Type, err)
	}
	return doc, nil
}

func readSchema(fsys fs.FS, name string) (any, error) {
	file, err := fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("contracts: open %s: %w", name, err)
	}
	// The file is read only; there is nothing a close error could mean here.
	defer func() { _ = file.Close() }()
	doc, err := jsonschema.UnmarshalJSON(file)
	if err != nil {
		return nil, fmt.Errorf("contracts: parse %s: %w", name, err)
	}
	return doc, nil
}

// defaultRegistry compiles the embedded schemas once, on first use.
var defaultRegistry = sync.OnceValues(func() (*Registry, error) { return New(schemas.FS) })

// Default returns the process registry built from the embedded schemas.
//
// It panics when the embedded schemas do not compile: that is a defect of the
// build, not a runtime condition, and TestSchemasValid catches it long before
// a binary is shipped.
func Default() *Registry {
	reg, err := defaultRegistry()
	if err != nil {
		panic("contracts: embedded schemas do not compile: " + err.Error())
	}
	return reg
}

// Lookup reports the whole contract of a type from the default registry
// (contracts.md C-01).
func Lookup(typ string) (Spec, bool) { return Default().Spec(typ) }

// Validate checks an event against the default registry.
func Validate(ev eventbus.Event) error { return Default().Validate(ev) }

// All returns every registered contract.
func All() []Spec { return Default().All() }

// Topics returns the topic map: the single source for redpanda-init and for
// mvctl contracts topics.
func Topics() []TopicSpec { return Default().Topics() }
