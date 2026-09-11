package eventbus

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewRootStartsAChain(t *testing.T) {
	deterministicSources(t)

	ev := NewRoot("player.attacked", "gateway", "dark-forest-world",
		&ScopeRef{ID: "solo:player-A", Type: "solo"}, ActorHuman,
		map[string]any{"target": "wolf"})

	if ev.ID != "ev-1" {
		t.Fatalf("id = %q, want ev-1", ev.ID)
	}
	if ev.Meta.CorrelationID != ev.ID {
		t.Errorf("correlation_id = %q, want the event id %q", ev.Meta.CorrelationID, ev.ID)
	}
	if ev.Meta.CausationID != "" || ev.Meta.CausationType != "" {
		t.Errorf("root event carries a cause: %+v", ev.Meta)
	}
	if !ev.Timestamp.Equal(fixtureTime) {
		t.Errorf("timestamp = %s, want the clock reading %s", ev.Timestamp, fixtureTime)
	}
	if ev.Meta.ActorKind != ActorHuman || ev.Meta.Locale != DefaultLocale {
		t.Errorf("meta = %+v, want actor_kind human and locale ru", ev.Meta)
	}
	if ev.Meta.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1 without a registry", ev.Meta.SchemaVersion)
	}
	if ev.World == nil || ev.World.Entity.ID != "dark-forest-world" || ev.World.Entity.Type != "world" {
		t.Errorf("world = %+v", ev.World)
	}
	if ev.Key() != "dark-forest-world" {
		t.Errorf("key = %q, want the world id", ev.Key())
	}
}

func TestNewRootDefaultsToTheAgentGMPath(t *testing.T) {
	deterministicSources(t)

	// gm_path is mandatory (api-contracts §2.1). Without a default every root
	// event — and, through Derive, every chain — would be rejected by the
	// envelope schema of T-006 and land in dead_letters.
	ev := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	if ev.Meta.GMPath != GMPathAgent {
		t.Errorf("gm_path = %q, want %q", ev.Meta.GMPath, GMPathAgent)
	}
	if err := ev.ValidateEnvelope(); err != nil {
		t.Errorf("a root event does not pass its own envelope check: %v", err)
	}

	legacy := NewRoot("player.attacked", "legacy-orchestrator", "w1", nil, ActorHuman, nil,
		WithGMPath(GMPathLegacy))
	if legacy.Meta.GMPath != GMPathLegacy {
		t.Errorf("gm_path = %q, want the override %q", legacy.Meta.GMPath, GMPathLegacy)
	}
	if got := Derive(legacy, "narrative.output", "legacy-orchestrator", nil).Meta.GMPath; got != GMPathLegacy {
		t.Errorf("derived gm_path = %q, want the inherited %q", got, GMPathLegacy)
	}
}

func TestGMPathIsAlwaysOnTheWire(t *testing.T) {
	deterministicSources(t)

	body, err := json.Marshal(NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"gm_path":"agent"`) {
		t.Errorf("gm_path is missing from the wire form: %s", body)
	}
}

func TestNewRootWithoutWorldIsGlobal(t *testing.T) {
	deterministicSources(t)

	ev := NewRoot("tick.fired", "core/swarm", "", nil, ActorSystem, nil)

	if ev.World != nil {
		t.Errorf("world = %+v, want none", ev.World)
	}
	if ev.Key() != GlobalKey {
		t.Errorf("key = %q, want %q", ev.Key(), GlobalKey)
	}
	if ev.Payload == nil {
		t.Error("payload is nil: a handler would have to nil-check every read")
	}
}

func TestNewRootTakesSchemaVersionFromRegistry(t *testing.T) {
	deterministicSources(t)
	SetRegistry(testRegistry())

	if got := NewRoot("entity.updated", "core/state", "w1", nil, ActorSystem, nil).Meta.SchemaVersion; got != 2 {
		t.Errorf("schema_version = %d, want 2 from the registry", got)
	}
	if got := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil).Meta.SchemaVersion; got != 1 {
		t.Errorf("schema_version = %d, want 1", got)
	}
}

func TestDeriveInheritsTheTrace(t *testing.T) {
	manual := deterministicSources(t)

	root := NewRoot("player.attacked", "gateway", "dark-forest-world",
		&ScopeRef{ID: "solo:player-A", Type: "solo"}, ActorHuman, nil,
		WithGMPath(GMPathAgent))

	// Time moves on: the derived event must still carry the timestamp of its
	// cause, otherwise two replays of one recording differ (NFR-061).
	manual.Advance(5 * time.Second)

	child := Derive(root, "combat.decided", "core/swarm", map[string]any{"outcome": "hit"},
		WithAgent(AgentRef{ID: "encounter-wolf:solo:player-A", Level: "task", Blueprint: "encounter-wolf"}))

	if child.ID == root.ID {
		t.Fatal("the derived event reuses the id of its cause")
	}
	if child.Meta.CorrelationID != root.ID {
		t.Errorf("correlation_id = %q, want the root id %q", child.Meta.CorrelationID, root.ID)
	}
	if child.Meta.CausationID != root.ID || child.Meta.CausationType != root.Type {
		t.Errorf("cause = %q/%q, want %q/%q",
			child.Meta.CausationID, child.Meta.CausationType, root.ID, root.Type)
	}
	if !child.Timestamp.Equal(root.Timestamp) {
		t.Errorf("timestamp = %s, want the timestamp of the cause %s", child.Timestamp, root.Timestamp)
	}
	if child.Meta.ActorKind != root.Meta.ActorKind ||
		child.Meta.Locale != root.Meta.Locale ||
		child.Meta.GMPath != root.Meta.GMPath {
		t.Errorf("meta not inherited: %+v", child.Meta)
	}
	if child.Meta.Agent == nil || child.Meta.Agent.Blueprint != "encounter-wolf" {
		t.Errorf("agent = %+v", child.Meta.Agent)
	}
	if child.World == nil || child.World.Entity.ID != "dark-forest-world" {
		t.Errorf("world not inherited: %+v", child.World)
	}
	if child.Scope == nil || child.Scope.ID != "solo:player-A" {
		t.Errorf("scope not inherited: %+v", child.Scope)
	}
}

func TestDeriveGrandchildKeepsOneCorrelation(t *testing.T) {
	deterministicSources(t)

	root := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, nil)
	child := Derive(root, "combat.decided", "core/swarm", nil)
	grandchild := Derive(child, "entity.update.proposed", "core/swarm", nil)

	if grandchild.Meta.CorrelationID != root.ID {
		t.Errorf("correlation_id = %q, want the root id %q", grandchild.Meta.CorrelationID, root.ID)
	}
	if grandchild.Meta.CausationID != child.ID {
		t.Errorf("causation_id = %q, want the direct cause %q", grandchild.Meta.CausationID, child.ID)
	}
}

func TestDeriveDoesNotShareReferencesWithItsCause(t *testing.T) {
	deterministicSources(t)

	root := NewRoot("player.attacked", "gateway", "w1", &ScopeRef{ID: "solo:a", Type: "solo"}, ActorHuman, nil)
	child := Derive(root, "combat.decided", "core/swarm", nil)

	child.Scope.ID = "group:g-1"
	child.World.Entity.ID = "other-world"

	if root.Scope.ID != "solo:a" || root.World.Entity.ID != "w1" {
		t.Errorf("mutating the child changed the cause: scope=%+v world=%+v", root.Scope, root.World)
	}
}

func TestWithScopeOverridesTheInheritedScope(t *testing.T) {
	deterministicSources(t)

	root := NewRoot("player.attacked", "gateway", "w1", &ScopeRef{ID: "solo:a", Type: "solo"}, ActorHuman, nil)

	group := Derive(root, "narrative.output", "core/swarm", nil, WithScope(&ScopeRef{ID: "group:g-1", Type: "group"}))
	if group.Scope == nil || group.Scope.ID != "group:g-1" {
		t.Errorf("scope = %+v, want the override", group.Scope)
	}

	none := Derive(root, "narrative.output", "core/swarm", nil, WithScope(nil))
	if none.Scope != nil {
		t.Errorf("scope = %+v, want none", none.Scope)
	}
}

func TestDeterministicSourcesReproduceTheSameBytes(t *testing.T) {
	encode := func(t *testing.T) []byte {
		t.Helper()
		deterministicSources(t)
		root := NewRoot("player.attacked", "gateway", "w1", nil, ActorHuman, map[string]any{"target": "wolf"})
		child := Derive(root, "combat.decided", "core/swarm", map[string]any{"outcome": "hit"})
		body, err := json.Marshal([]Event{root, child})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return body
	}

	if first, second := string(encode(t)), string(encode(t)); first != second {
		t.Errorf("two runs differ:\n%s\n%s", first, second)
	}
}

func TestCorrelationIDFallsBackToTheEventID(t *testing.T) {
	ev := Event{ID: "ev-9"}
	if got := ev.CorrelationID(); got != "ev-9" {
		t.Errorf("correlation id = %q, want the event id", got)
	}
}

func TestEnvelopeRoundTripsThroughJSON(t *testing.T) {
	deterministicSources(t)

	ev := NewRoot("tick.fired", "core/swarm", "w1", &ScopeRef{ID: "region:r1", Type: "region"},
		ActorSystem, map[string]any{"seq": float64(7)},
		WithAgent(AgentRef{ID: "global-gm", Level: "strategic", Blueprint: "global-gm", BlueprintVersion: "1.0"}),
		WithGMPath(GMPathAgent))

	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Event
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Meta.Agent == nil || *back.Meta.Agent != *ev.Meta.Agent {
		t.Errorf("agent lost in transit: %+v", back.Meta.Agent)
	}
	back.Meta.Agent, ev.Meta.Agent = nil, nil
	if back.Meta != ev.Meta {
		t.Errorf("meta = %+v, want %+v", back.Meta, ev.Meta)
	}
	if !back.Timestamp.Equal(ev.Timestamp) {
		t.Errorf("timestamp = %s, want %s", back.Timestamp, ev.Timestamp)
	}
}

func TestDeprecatedNewEventCarriesNoMeta(t *testing.T) {
	deterministicSources(t)

	ev := NewEvent("player.moved", "legacy-orchestrator", "w1", nil)
	if ev.Meta != (Meta{}) {
		t.Errorf("meta = %+v, want the zero value for a legacy envelope", ev.Meta)
	}
	if ev.ID == "" || ev.Timestamp.IsZero() {
		t.Errorf("legacy envelope is incomplete: %+v", ev)
	}
}

func TestSequenceIDsCounts(t *testing.T) {
	gen := SequenceIDs("ev")
	for i, want := range []string{"ev-1", "ev-2", "ev-3"} {
		if got := gen(); got != want {
			t.Errorf("id %d = %q, want %q", i, got, want)
		}
	}
}
