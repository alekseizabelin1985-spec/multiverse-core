// Package eventbus defines the event envelope of the platform together with
// the bus and journal interfaces every context talks through (contracts.md
// C-01 v1.1, ADR-007).
//
// The cross-cutting fields live in the envelope (Meta), not in the payload: a
// publisher copies them with one call (Derive) instead of six, and the payload
// stays purely domain data described by the JSON schema of its type. Event
// identifiers and timestamps come from package-level sources the process
// installs (SetIDSource, SetClock), so that replaying the same journal
// produces the same bytes (NFR-061).
package eventbus

import (
	"time"

	"multiverse-core.io/shared/jsonpath"
)

// Actor kinds accepted in Meta.ActorKind (ADR-007 p. 1).
const (
	ActorHuman  = "human"
	ActorCI     = "ci"
	ActorSim    = "sim"
	ActorSystem = "system"
)

// GM paths accepted in Meta.GMPath: the agent swarm, or the legacy
// orchestrator that goes away with the legacy profile (S5).
const (
	GMPathAgent  = "agent"
	GMPathLegacy = "legacy"
)

// DefaultLocale is the only locale of MVP-1 (api-contracts §2.1).
const DefaultLocale = "ru"

// GlobalKey is the message key of an event that belongs to no world.
const GlobalKey = "global"

// AgentRef identifies the swarm agent that produced an event. It is required
// on the swarm topics and forbidden on player_events (see Policy).
type AgentRef struct {
	ID               string `json:"id"`
	Level            string `json:"level"`
	Blueprint        string `json:"blueprint"`
	BlueprintVersion string `json:"blueprint_version,omitempty"`
}

// Meta carries the fields that travel with every event regardless of its type:
// the trace of the chain that produced it, who acted, and how the event must
// be interpreted on replay.
type Meta struct {
	// SchemaVersion is the version of the payload schema of the type, from 1.
	SchemaVersion int `json:"schema_version"`
	// CorrelationID is the identifier of the root event of the chain; on a
	// root event it equals Event.ID.
	CorrelationID string `json:"correlation_id"`
	// CausationID and CausationType name the immediate cause; both are absent
	// on a root event.
	CausationID   string `json:"causation_id,omitempty"`
	CausationType string `json:"causation_type,omitempty"`
	// ActorKind is one of human, ci, sim, system.
	ActorKind string `json:"actor_kind"`
	// Agent is set on events published by the swarm runtime.
	Agent *AgentRef `json:"agent,omitempty"`
	// Replay marks an event fed back from a journal in test mode.
	Replay bool   `json:"replay"`
	Locale string `json:"locale"`
	// GMPath is agent or legacy. It is mandatory (api-contracts §2.1): the
	// tag carries no omitempty, so an envelope built outside the constructors
	// fails validation instead of silently reaching the schema of T-006
	// without a required field.
	GMPath string `json:"gm_path"`
}

// Event is the wire envelope. New fields may be added without breaking
// existing consumers; removing or renaming one is a schema version bump
// (ADR-007 p. 3).
type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Source    string         `json:"source"`
	World     *WorldRef      `json:"world,omitempty"`
	Scope     *ScopeRef      `json:"scope,omitempty"`
	Meta      Meta           `json:"meta"`
	Payload   map[string]any `json:"payload"`
	// Relations declares explicit semantic edges for the knowledge graph.
	Relations []Relation `json:"relations,omitempty"`
}

// DeriveOption customises an event built by NewRoot or Derive (C-01).
type DeriveOption func(*Event)

// WithAgent records the swarm agent that produced the event.
func WithAgent(a AgentRef) DeriveOption {
	return func(e *Event) {
		agent := a
		e.Meta.Agent = &agent
	}
}

// WithScope overrides the scope inherited from the cause.
func WithScope(s *ScopeRef) DeriveOption {
	return func(e *Event) {
		if s == nil {
			e.Scope = nil
			return
		}
		scope := *s
		e.Scope = &scope
	}
}

// WithWorld overrides the world inherited from the cause.
func WithWorld(worldID string) DeriveOption {
	return func(e *Event) { e.World = newWorldRef(worldID) }
}

// WithRelations attaches semantic edges for the knowledge graph.
func WithRelations(rels ...Relation) DeriveOption {
	return func(e *Event) { e.Relations = append(e.Relations, rels...) }
}

// WithGMPath marks which game master path produced the event.
func WithGMPath(path string) DeriveOption {
	return func(e *Event) { e.Meta.GMPath = path }
}

// WithReplay marks the event as fed back from a journal.
func WithReplay(replay bool) DeriveOption {
	return func(e *Event) { e.Meta.Replay = replay }
}

// WithTimestamp overrides the timestamp of the envelope.
//
// Deprecated: the timestamp comes from the clock on a root event and from the
// cause on a derived one; overriding it breaks byte-for-byte replay. It exists
// for the legacy profile and for fixtures only.
func WithTimestamp(t time.Time) DeriveOption {
	return func(e *Event) { e.Timestamp = t.UTC() }
}

// NewRoot builds the first event of a chain: its correlation identifier is its
// own identifier and it has no cause. The schema version comes from the
// contract registry installed with SetRegistry, and defaults to 1.
//
// The game master path defaults to GMPathAgent, the only path a new publisher
// has; the legacy orchestrator overrides it with WithGMPath(GMPathLegacy)
// until the legacy profile goes away (S5).
func NewRoot(typ, source, worldID string, scope *ScopeRef, actorKind string, payload map[string]any, opts ...DeriveOption) Event {
	id := nextID()
	ev := Event{
		ID:        id,
		Type:      typ,
		Timestamp: nowUTC(),
		Source:    source,
		World:     newWorldRef(worldID),
		Payload:   ensurePayload(payload),
		Meta: Meta{
			SchemaVersion: schemaVersionOf(typ),
			CorrelationID: id,
			ActorKind:     actorKind,
			Locale:        DefaultLocale,
			GMPath:        GMPathAgent,
		},
	}
	if scope != nil {
		s := *scope
		ev.Scope = &s
	}
	return applyOptions(ev, opts)
}

// Derive builds a consequence of parent. It inherits the world, the scope and
// the trace fields of the cause, and records the cause in Meta.
//
// The timestamp is inherited always, not only during replay: two runs of the
// same recording must produce the same bytes (NFR-061).
func Derive(parent Event, typ, source string, payload map[string]any, opts ...DeriveOption) Event {
	ev := Event{
		ID:        nextID(),
		Type:      typ,
		Timestamp: parent.Timestamp,
		Source:    source,
		Payload:   ensurePayload(payload),
		Meta: Meta{
			SchemaVersion: schemaVersionOf(typ),
			CorrelationID: parent.CorrelationID(),
			CausationID:   parent.ID,
			CausationType: parent.Type,
			ActorKind:     parent.Meta.ActorKind,
			Replay:        parent.Meta.Replay,
			Locale:        parent.Meta.Locale,
			GMPath:        parent.Meta.GMPath,
		},
	}
	if ev.Meta.Locale == "" {
		ev.Meta.Locale = DefaultLocale
	}
	// Copy the references: the child must not share mutable state with the
	// cause, which a caller may still be holding.
	if parent.World != nil {
		world := *parent.World
		ev.World = &world
	}
	if parent.Scope != nil {
		scope := *parent.Scope
		ev.Scope = &scope
	}
	return applyOptions(ev, opts)
}

// CorrelationID returns the identifier of the root of the chain, falling back
// to the event identifier for an envelope built outside the constructors.
func (e Event) CorrelationID() string {
	if e.Meta.CorrelationID != "" {
		return e.Meta.CorrelationID
	}
	return e.ID
}

// Key returns the partition key of the event: the world it belongs to, so that
// everything about one world keeps its order.
func (e Event) Key() string {
	if e.World != nil && e.World.Entity.ID != "" {
		return e.World.Entity.ID
	}
	return GlobalKey
}

// Path returns the accessor for dot-path reads of the payload.
// Example: event.Path().GetString("entity.id").
func (e Event) Path() *jsonpath.Accessor {
	return jsonpath.New(e.Payload)
}

// GetEntityIDWithFallback extracts the main entity of the event, trying
// entity.entity.id, then entity.id, then the flat legacy keys.
func (e Event) GetEntityIDWithFallback() (*EntityInfo, bool) {
	info := ExtractEntityID(e.Payload)
	return info, info != nil
}

// GetTargetEntityID extracts the target entity of the event, trying
// target.entity.id, then target.id, then the flat legacy keys.
func (e Event) GetTargetEntityID() (*EntityInfo, bool) {
	info := ExtractTargetEntityID(e.Payload)
	return info, info != nil
}

// GetWorldIDFromEvent reads the world identifier from the envelope, falling
// back to payload.world.entity.id for legacy producers.
func GetWorldIDFromEvent(event Event) string {
	if event.World != nil {
		return event.World.Entity.ID
	}
	if worldID := ExtractWorldID(event.Payload); worldID != "" {
		return worldID
	}
	return ""
}

// GetScopeFromEvent reads the scope from the envelope, falling back to
// payload.scope for legacy producers.
func GetScopeFromEvent(event Event) *ScopeRef {
	if event.Scope != nil {
		return event.Scope
	}
	if scope := ExtractScope(event.Payload); scope != nil && (scope.ID != "" || scope.Type != "") {
		return scope
	}
	return nil
}

// NewEvent builds an envelope without Meta.
//
// Deprecated: use NewRoot or Derive. Events without Meta are accepted only for
// types marked deprecated in the registry (gm_path=legacy) and go away with
// the legacy profile.
func NewEvent(eventType, source, worldID string, payload map[string]any) Event {
	return Event{
		ID:        nextID(),
		Type:      eventType,
		Timestamp: nowUTC(),
		Source:    source,
		World:     newWorldRef(worldID),
		Payload:   ensurePayload(payload),
	}
}

// NewEventWithDescription builds a legacy envelope with a description payload.
//
// Deprecated: use NewRoot with the payload of the registered type.
func NewEventWithDescription(eventType, source, worldID, description string) Event {
	return NewEvent(eventType, source, worldID, map[string]any{"description": description})
}

// NewStructuredEvent builds a legacy envelope from the payload builder.
//
// Deprecated: use NewRoot with the payload of the registered type.
func NewStructuredEvent(eventType, source, worldID string, payload *EventPayload) Event {
	return NewEvent(eventType, source, worldID, payload.ToMap())
}

func applyOptions(ev Event, opts []DeriveOption) Event {
	for _, opt := range opts {
		if opt != nil {
			opt(&ev)
		}
	}
	return ev
}

func ensurePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return make(map[string]any)
	}
	return payload
}

func newWorldRef(worldID string) *WorldRef {
	if worldID == "" {
		return nil
	}
	return &WorldRef{Entity: EntityRef{ID: worldID, Type: "world"}}
}
