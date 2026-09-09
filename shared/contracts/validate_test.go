package contracts

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"multiverse-core.io/shared/eventbus"
)

// newEvent builds a well-formed envelope of the given type. The payload is
// taken as JSON so that the fixtures below read like the wire format they
// describe (api-contracts.md §2.3).
func newEvent(t *testing.T, typ, actorKind string, payload map[string]any) eventbus.Event {
	t.Helper()
	scope := &eventbus.ScopeRef{ID: "solo:player-A", Type: "solo"}
	return eventbus.NewRoot(typ, SourceGateway, "dark-forest-world", scope, actorKind, payload)
}

func payloadOf(t *testing.T, raw string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("fixture is not JSON: %v", err)
	}
	return payload
}

// TestEnvelopeRejects covers the mandatory fields of meta (api-contracts.md
// §2.1). Every case is an envelope a publisher could build by hand today, and
// every one of them must be refused before it reaches a consumer.
func TestEnvelopeRejects(t *testing.T) {
	valid := payloadOf(t, `{"entity": {"entity": {"id": "player-A", "type": "player"}}}`)

	cases := map[string]func(ev *eventbus.Event){
		"empty gm_path":         func(ev *eventbus.Event) { ev.Meta.GMPath = "" },
		"unknown gm_path":       func(ev *eventbus.Event) { ev.Meta.GMPath = "manual" },
		"foreign actor_kind":    func(ev *eventbus.Event) { ev.Meta.ActorKind = "operator" },
		"missing correlation":   func(ev *eventbus.Event) { ev.Meta.CorrelationID = "" },
		"schema version zero":   func(ev *eventbus.Event) { ev.Meta.SchemaVersion = 0 },
		"empty locale":          func(ev *eventbus.Event) { ev.Meta.Locale = "" },
		"empty source":          func(ev *eventbus.Event) { ev.Source = "" },
		"agent without a level": func(ev *eventbus.Event) { ev.Meta.Agent = &eventbus.AgentRef{ID: "a", Blueprint: "b"} },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			ev := newEvent(t, "player.looked", eventbus.ActorHuman, valid)
			breakIt(&ev)
			err := Validate(ev)
			if !errors.Is(err, eventbus.ErrInvalidEnvelope) {
				t.Fatalf("error %v, want an invalid envelope", err)
			}
		})
	}
}

func TestEnvelopeAccepts(t *testing.T) {
	ev := newEvent(t, "player.looked", eventbus.ActorHuman,
		payloadOf(t, `{"entity": {"entity": {"id": "player-A", "type": "player"}, "name": "Vasya"}}`))
	if err := Validate(ev); err != nil {
		t.Fatalf("a well-formed event is rejected: %v", err)
	}

	// A consequence published by the swarm: meta.agent is filled and the
	// envelope carries the cause it derives from.
	root := newEvent(t, "player.attacked", eventbus.ActorHuman,
		payloadOf(t, `{"entity": {"entity": {"id": "player-A", "type": "player"}},
			"action": {"type": "attack"},
			"target": {"entity": {"id": "wolf-alpha", "type": "npc"}}}`))
	derived := eventbus.Derive(root, "dice.rolled", SourceSwarm,
		payloadOf(t, `{"roll": {"index": 0, "formula": "1d20", "seed": 4133419312, "result": 17, "natural": 17},
			"purpose": "hit",
			"roller": {"entity": {"id": "player-A", "type": "player"}}}`),
		eventbus.WithAgent(eventbus.AgentRef{ID: "encounter-wolf:solo:player-A", Level: "task", Blueprint: "encounter-wolf"}))
	if err := Validate(derived); err != nil {
		t.Fatalf("a derived swarm event is rejected: %v", err)
	}
	if derived.Meta.CausationID != root.ID {
		t.Errorf("causation_id %q, want the identifier of the cause", derived.Meta.CausationID)
	}
}

// TestPayloadExamples runs the examples of api-contracts.md §2.3 for the
// blocks "а" and "б" against their schemas.
func TestPayloadExamples(t *testing.T) {
	cases := map[string]struct {
		actorKind string
		payload   string
	}{
		"player.entered_region": {eventbus.ActorHuman, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}, "name": "Vasya"},
			"action": {"type": "enter", "key_hash": "9f2c"},
			"target": {"entity": {"id": "dark-forest-01", "type": "region"}, "name": "Dark forest"},
			"position": {"from": "village", "to": "dark-forest-01"},
			"session": {"id": "sess-1"}}`},
		"player.left_region": {eventbus.ActorHuman, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"action": {"type": "leave", "key_hash": "9f2c"},
			"target": {"entity": {"id": "dark-forest-01", "type": "region"}},
			"position": {"from": "dark-forest-01", "to": "village"}}`},
		"player.looked": {eventbus.ActorHuman, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"action": {"type": "look", "key_hash": "9f2c"}}`},
		"player.attacked": {eventbus.ActorHuman, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}, "name": "Vasya"},
			"action": {"type": "attack", "key_hash": "9f2c"},
			"target": {"entity": {"id": "wolf-alpha", "type": "npc"}, "name": "Alpha wolf"},
			"encounter": {"entity": {"id": "enc-7", "type": "encounter"}},
			"round": {"seq": 3},
			"session": {"id": "sess-1"}}`},
		"player.flee_attempted": {eventbus.ActorHuman, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"action": {"type": "flee"},
			"encounter": {"entity": {"id": "enc-7", "type": "encounter"}},
			"round": {"seq": 4}}`},
		"player.rested": {eventbus.ActorHuman, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"action": {"type": "rest"}}`},
		"player.said": {eventbus.ActorHuman, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"action": {"type": "say", "key_hash": "9f2c"},
			"text": "Who goes there?"}`},
		"player.defended": {eventbus.ActorSim, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"cause": "round_timeout",
			"round": {"seq": 3}}`},
		"group.created": {eventbus.ActorHuman, `{
			"group": {"entity": {"id": "g-1", "type": "group"}},
			"leader": {"entity": {"id": "player-A", "type": "player"}, "name": "Vasya"},
			"members": [{"entity": {"id": "player-A", "type": "player"}, "name": "Vasya", "participation": "active"}],
			"cause": "create"}`},
		"group.joined": {eventbus.ActorHuman, `{
			"group": {"entity": {"id": "g-1", "type": "group"}},
			"leader": {"entity": {"id": "player-A", "type": "player"}},
			"members": [{"entity": {"id": "player-A", "type": "player"}},
			            {"entity": {"id": "player-B", "type": "player"}, "participation": "idle"}],
			"cause": "join"}`},
		"group.left": {eventbus.ActorHuman, `{
			"group": {"entity": {"id": "g-1", "type": "group"}},
			"leader": {"entity": {"id": "player-A", "type": "player"}},
			"members": [{"entity": {"id": "player-A", "type": "player"}}],
			"cause": "forget"}`},
		"group.leader_changed": {eventbus.ActorHuman, `{
			"group": {"entity": {"id": "g-1", "type": "group"}},
			"leader": null,
			"cause": "forget"}`},
		"group.disbanded": {eventbus.ActorHuman, `{
			"group": {"entity": {"id": "g-1", "type": "group"}},
			"members": [],
			"cause": "disband"}`},
		"group.entered_region": {eventbus.ActorHuman, `{
			"group": {"entity": {"id": "g-1", "type": "group"}},
			"target": {"entity": {"id": "dark-forest-01", "type": "region"}},
			"position": {"from": null, "to": "dark-forest-01"},
			"by": {"entity": {"id": "player-A", "type": "player"}},
			"members": [{"entity": {"id": "player-A", "type": "player"}, "participation": "active"}]}`},
		"group.left_region": {eventbus.ActorHuman, `{
			"group": {"entity": {"id": "g-1", "type": "group"}},
			"target": {"entity": {"id": "dark-forest-01", "type": "region"}},
			"position": {"from": "dark-forest-01", "to": "village"},
			"by": {"entity": {"id": "player-A", "type": "player"}},
			"members": [{"entity": {"id": "player-A", "type": "player"}}]}`},
		"round.opened": {eventbus.ActorHuman, `{
			"encounter": {"entity": {"id": "enc-7", "type": "encounter"}},
			"round": {"seq": 1},
			"expected": [{"entity": {"id": "player-A", "type": "player"}}],
			"deadline_at": "2026-09-09T12:00:30Z"}`},
		"round.closed": {eventbus.ActorHuman, `{
			"encounter": {"entity": {"id": "enc-7", "type": "encounter"}},
			"round": {"seq": 1, "close_reason": "timeout"},
			"acted": [{"entity": {"id": "player-A", "type": "player"}, "event": {"id": "ev-1", "type": "player.attacked"}}],
			"auto_defended": [{"entity": {"id": "player-B", "type": "player"}}],
			"idle": [],
			"closed_at": "2026-09-09T12:00:30Z"}`},
		"entity.create.proposed": {eventbus.ActorHuman, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}, "name": "Vasya"},
			"attributes": {"hp": 10, "hp_max": 10, "status": "alive"},
			"cause": "create"}`},
		"entity.update.proposed": {eventbus.ActorSystem, `{
			"proposal_id": "p-1",
			"changes": [
				{"entity": {"entity": {"id": "wolf-alpha", "type": "npc"}}, "expected_version": 12,
				 "ops": [{"op": "set", "path": "hp", "value": 7}, {"op": "set", "path": "status", "value": "alive"}]},
				{"entity": {"entity": {"id": "player-A", "type": "player"}}, "expected_version": 23,
				 "ops": [{"op": "append", "path": "inventory",
				          "value": {"item_id": "item-1", "kind": "wolf-pelt"}}]}],
			"atomic": true,
			"cause": "combat"}`},
		"entity.created": {eventbus.ActorSystem, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}, "name": "Vasya"},
			"version": 1,
			"attributes": {"hp": 10}}`},
		"entity.updated": {eventbus.ActorSystem, `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"version": 24,
			"changed": [{"path": "hp", "old": 10, "new": 7}],
			"cause": "combat",
			"proposal_id": "p-1",
			"applied_at": "2026-09-09T12:00:00Z"}`},
		"entity.update.rejected": {eventbus.ActorSystem, `{
			"proposal_id": "p-1",
			"reason": "version_conflict",
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"details": {"expected_version": 23, "actual_version": 24}}`},
		"dice.rolled": {eventbus.ActorSystem, `{
			"roll": {"index": 1, "formula": "1d8+2", "seed": 12345678, "result": 7, "natural": 5},
			"purpose": "damage",
			"roller": {"entity": {"id": "player-A", "type": "player"}}}`},
		"snapshot.created": {eventbus.ActorSystem, `{
			"component": "state",
			"snapshot": {"id": "snap-1", "seq": 3, "taken_at": "2026-09-09T12:00:00Z",
				"cursor": {"system_events": 1024},
				"laws_version": "v1", "state_hash": "abcdef", "size_bytes": 4096,
				"key": "snapshots-dark-forest-world/state/20260909T120000Z-3.json"}}`},
		"analytics.replay.completed": {eventbus.ActorSystem, `{
			"mode": "recovery",
			"replay": {"run_id": "run-1", "snapshot_id": null, "events_replayed": 0, "llm_calls": 0,
				"dice_rolled_new": 0, "duration_ms": 12, "state_hash_after": "abcdef",
				"incomplete_record": false}}`},
	}

	for typ, example := range cases {
		t.Run(typ, func(t *testing.T) {
			ev := newEvent(t, typ, example.actorKind, payloadOf(t, example.payload))
			if err := Validate(ev); err != nil {
				t.Fatalf("the example of §2.3 is rejected: %v", err)
			}
		})
	}

	// Blocks "а" and "б" live here, block "в" in blockv_test.go; together they
	// must cover every type that has a schema.
	if got, want := len(cases)+len(blockVExamples), countTypesWithSchema(t); got != want {
		t.Errorf("%d examples for %d registered types with a schema: every type needs one", got, want)
	}
}

func countTypesWithSchema(t *testing.T) int {
	t.Helper()
	n := 0
	for _, spec := range All() {
		if !spec.Deprecated {
			n++
		}
	}
	return n
}

// TestPayloadRejects covers what the schemas are there for: a typo in a field
// name, a value outside an enum, a missing mandatory field and a timestamp
// that is not a timestamp.
func TestPayloadRejects(t *testing.T) {
	cases := map[string]struct {
		typ     string
		payload string
	}{
		"unknown field": {"player.looked", `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"entyty": {"entity": {"id": "player-A", "type": "player"}}}`},
		"missing target": {"player.attacked", `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"action": {"type": "attack"}}`},
		"foreign defend cause": {"player.defended", `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"cause": "boredom"}`},
		"text too long": {"player.said", `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"text": "` + strings.Repeat("a", 501) + `"}`},
		"unknown op": {"entity.update.proposed", `{
			"proposal_id": "p-1",
			"changes": [{"entity": {"entity": {"id": "player-A", "type": "player"}},
			             "ops": [{"op": "replace", "path": "hp", "value": 1}]}],
			"atomic": false, "cause": "combat"}`},
		"no changes": {"entity.update.proposed", `{
			"proposal_id": "p-1", "changes": [], "atomic": false, "cause": "combat"}`},
		"unknown reject reason": {"entity.update.rejected", `{
			"proposal_id": "p-1", "reason": "because"}`},
		"applied_at is not a timestamp": {"entity.updated", `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"version": 2, "changed": [], "cause": "combat", "proposal_id": "p-1",
			"applied_at": "yesterday"}`},
		"unknown snapshot component": {"snapshot.created", `{
			"component": "core",
			"snapshot": {"id": "s", "seq": 0, "taken_at": "2026-09-09T12:00:00Z", "cursor": {},
				"laws_version": "v1", "state_hash": "h", "size_bytes": 1, "key": "k"}}`},
		"unknown dice purpose": {"dice.rolled", `{
			"roll": {"index": 0, "formula": "1d20", "seed": 1, "result": 3, "natural": 3},
			"purpose": "luck",
			"roller": {"entity": {"id": "player-A", "type": "player"}}}`},
		"replay without incomplete_record": {"analytics.replay.completed", `{
			"mode": "recovery",
			"replay": {"run_id": "r", "events_replayed": 0, "llm_calls": 0, "dice_rolled_new": 0,
				"duration_ms": 1, "state_hash_after": "h"}}`},
		"created with version two": {"entity.created", `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"version": 2, "attributes": {}}`},
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			ev := newEvent(t, bad.typ, eventbus.ActorSystem, payloadOf(t, bad.payload))
			err := Validate(ev)
			if !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("error %v, want an invalid payload", err)
			}
		})
	}
}

// TestCauseForget pins the addition of C-02 v1.2: the /forget cascade of the
// gateway proposes the transition alive -> abandoned with cause=forget, and
// State reports it back with the same cause.
func TestCauseForget(t *testing.T) {
	proposal := newEvent(t, "entity.update.proposed", eventbus.ActorHuman, payloadOf(t, `{
		"proposal_id": "p-forget",
		"changes": [{"entity": {"entity": {"id": "player-A", "type": "player"}}, "expected_version": 23,
		             "ops": [{"op": "set", "path": "status", "value": "abandoned"}]}],
		"atomic": true,
		"cause": "forget"}`))
	if err := Validate(proposal); err != nil {
		t.Errorf("entity.update.proposed cause=forget is rejected: %v", err)
	}

	fact := newEvent(t, "entity.updated", eventbus.ActorHuman, payloadOf(t, `{
		"entity": {"entity": {"id": "player-A", "type": "player"}},
		"version": 24,
		"changed": [{"path": "status", "old": "alive", "new": "abandoned"}],
		"cause": "forget",
		"proposal_id": "p-forget",
		"applied_at": "2026-09-09T12:00:00Z"}`))
	if err := Validate(fact); err != nil {
		t.Errorf("entity.updated cause=forget is rejected: %v", err)
	}
}

// TestRouteUsesTheRegistry closes the loop with the bus: the same registry
// resolves the topic and refuses what the topic policy forbids.
func TestRouteUsesTheRegistry(t *testing.T) {
	reg := Default()
	payload := payloadOf(t, `{"entity": {"entity": {"id": "player-A", "type": "player"}}}`)

	topic, err := eventbus.Route(reg, newEvent(t, "player.looked", eventbus.ActorHuman, payload))
	if err != nil {
		t.Fatalf("routing a player action failed: %v", err)
	}
	if topic != eventbus.TopicPlayerEvents {
		t.Errorf("topic %q, want %q", topic, eventbus.TopicPlayerEvents)
	}

	agentEvent := newEvent(t, "player.looked", eventbus.ActorHuman, payload)
	agentEvent.Meta.Agent = &eventbus.AgentRef{ID: "personal-gm:solo:player-A", Level: "task", Blueprint: "personal-gm"}
	if _, err := eventbus.Route(reg, agentEvent); !errors.Is(err, eventbus.ErrPolicyViolation) {
		t.Errorf("an agent published a player action: %v", err)
	}
}
