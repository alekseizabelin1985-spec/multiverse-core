package contracts

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/shared/eventbus"
)

// blockVExample is one payload of api-contracts.md §2.3 for a type of block
// "в", together with what its envelope must carry: the swarm types name the
// agent behind them, the analytics and the laws do not.
type blockVExample struct {
	source    string
	actorKind string
	agent     bool
	payload   string
}

// blockVExamples holds one valid example per type of block "в" (T-009). The
// completeness of the table is checked by TestPayloadExamples, which counts
// the examples of both blocks against the registry.
var blockVExamples = map[string]blockVExample{
	"combat.decided": {SourceSwarm, eventbus.ActorHuman, true, `{
		"encounter": {"entity": {"id": "enc-7", "type": "encounter"}},
		"round": {"seq": 3},
		"action": "attack",
		"attacker": {"entity": {"id": "player-A", "type": "player"}, "name": "Vasya"},
		"defender": {"entity": {"id": "wolf-alpha", "type": "npc"}, "name": "Alpha wolf"},
		"outcome": {"hit": true, "natural": 15, "damage": 3, "critical": false, "fumble": false,
			"target_dead": false, "success": null},
		"hp": {"defender_before": 7, "defender_after": 4, "defender_max": 10},
		"rolls": [{"event": {"id": "ev-1"}, "index": 0}, {"event": {"id": "ev-2"}, "index": 1}],
		"rules_version": "0.1", "phase1_mode": "rules", "lod": "rule-only",
		"free_attack": false}`},
	"encounter.started": {SourceSwarm, eventbus.ActorSystem, true, `{
		"encounter": {"entity": {"id": "enc-7", "type": "encounter"}},
		"region": {"entity": {"id": "dark-forest-01", "type": "region"}, "name": "Dark forest"},
		"participants": [{"entity": {"id": "player-A", "type": "player"}, "name": "Vasya"}],
		"npcs": [{"entity": {"id": "wolf-alpha", "type": "npc"}, "name": "Alpha wolf"}],
		"round": {"timeout": "60s", "idle_after_missed": 2}}`},
	"encounter.ended": {SourceSwarm, eventbus.ActorSystem, true, `{
		"encounter": {"entity": {"id": "enc-7", "type": "encounter"}},
		"reason": "npc_dead",
		"killer": {"entity": {"id": "player-A", "type": "player"}},
		"rounds": 4}`},
	"world.weather_changed": {SourceSwarm, eventbus.ActorSystem, true, `{
		"weather": {"from": "clear", "to": "overcast"}}`},
	"world.time_advanced": {SourceSwarm, eventbus.ActorSystem, true, `{
		"time_of_day": {"from": "day", "to": "evening"}, "day": 12}`},
	"world.event_occurred": {SourceSwarm, eventbus.ActorSystem, true, `{
		"kind": "storm",
		"summary": "Над лесом собралась гроза.",
		"affects": [{"entity": {"id": "dark-forest-01", "type": "region"}}]}`},
	"region.event_occurred": {SourceSwarm, eventbus.ActorSystem, true, `{
		"kind": "howling",
		"summary": "Волки воют у северной опушки.",
		"affects": [{"entity": {"id": "wolf-alpha", "type": "npc"}}]}`},
	"npc.moved": {SourceSwarm, eventbus.ActorSystem, true, `{
		"entity": {"entity": {"id": "wolf-alpha", "type": "npc"}, "name": "Alpha wolf"},
		"position": {"from": "glade", "to": "north-edge"}}`},
	"npc.spawned": {SourceSwarm, eventbus.ActorSystem, true, `{
		"entity": {"entity": {"id": "wolf-beta", "type": "npc"}, "name": "Beta wolf"},
		"kind": "wolf",
		"position": "north-edge",
		"spawned_by": {
			"agent": {"id": "domain-dark-forest:region:dark-forest-01", "level": "domain",
				"blueprint": "domain-dark-forest"},
			"tick": {"event": {"id": "ev-tick-1", "type": "tick.fired"}}}}`},
	"tick.fired": {SourceSwarm, eventbus.ActorSystem, true, `{
		"agent": {"id": "global-gm:dark-forest-world", "level": "global", "blueprint": "global-gm"},
		"scope": {"id": "world:dark-forest-world", "type": "world"},
		"tick": {"seq": 7, "mode": "background",
			"scheduled_at": "2026-09-09T12:00:00Z", "fired_at": "2026-09-09T12:00:01Z",
			"lod_allowed": "rule-only"},
		"budget": {"window_calls": 3, "cap": 4}}`},
	"tick.aborted": {SourceSwarm, eventbus.ActorSystem, true, `{
		"tick": {"seq": 7}, "reason": "shutdown"}`},
	"agent.spawned": {SourceSwarm, eventbus.ActorSystem, true, `{
		"agent": {"id": "encounter-wolf:solo:player-A", "level": "task", "blueprint": "encounter-wolf",
			"blueprint_version": "1.0"},
		"parent": {"id": "domain-dark-forest:region:dark-forest-01"},
		"scope": {"id": "solo:player-A", "type": "solo"},
		"ttl_expires_at": "2026-09-09T12:45:00Z",
		"lod": "rule-only",
		"content_hash": "9f2c4d"}`},
	"agent.child_resolved": {SourceSwarm, eventbus.ActorSystem, true, `{
		"agent": {"id": "encounter-wolf:solo:player-A", "level": "task", "blueprint": "encounter-wolf"},
		"parent": {"id": "domain-dark-forest:region:dark-forest-01"},
		"reason": "npc_dead"}`},
	"agent.stopped": {SourceSwarm, eventbus.ActorSystem, true, `{
		"agent": {"id": "player-gm:solo:player-A", "level": "task", "blueprint": "player-gm"},
		"reason": "ttl"}`},
	"agent.spawn_rejected": {SourceSwarm, eventbus.ActorSystem, true, `{
		"blueprint": "encounter-wolf",
		"scope": {"id": "solo:player-A", "type": "solo"},
		"reason": "max_instances"}`},
	"agent.blueprint_reloaded": {SourceSwarm, eventbus.ActorCI, true, `{
		"blueprint": "domain-dark-forest",
		"version_from": "1.0", "version_to": "1.1",
		"content_hash": "9f2c4d"}`},
	"llm.output": {SourceLLM, eventbus.ActorHuman, true, `{
		"phase": "narrative", "attempt": 1,
		"provider": "openai_compat", "model": "qwen3-8b",
		"params": {"temperature": 0.7, "max_tokens": 512, "thinking": false, "format": "json_schema"},
		"prompt_hash": "9f2c4d", "response_raw": "{\"text\": \"…\"}", "response_hash": "1a2b3c",
		"response_len": 214,
		"validation_status": "valid",
		"laws_version": "v1",
		"latency_ms": 1840, "tokens": {"prompt": 1200, "completion": 180, "cached": 900},
		"cost_usd": 0,
		"lod": "basic",
		"parse": {"strategy": "strip_fence", "recovered": true}}`},
	"llm.output.rejected": {SourceLLM, eventbus.ActorHuman, true, `{
		"llm_output": {"event": {"id": "ev-llm-1", "type": "llm.output"}},
		"reason": "unknown_entity",
		"element": {"index": 2, "type": "npc.moved"},
		"entity": {"entity": {"id": "wolf-omega", "type": "npc"}},
		"phase": "tick", "attempt": 1}`},
	"content.incident.recorded": {SourceLLM, eventbus.ActorHuman, true, `{
		"incident": {"category": "a", "filter_version": "a-2026-09",
			"llm_output": {"event": {"id": "ev-llm-1", "type": "llm.output"}}}}`},
	"narrative.output": {SourceSwarm, eventbus.ActorHuman, true, `{
		"recipients": [{"entity": {"id": "player-A", "type": "player"}},
			{"entity": {"id": "player-B", "type": "player"}}],
		"text": "Волк отступает, зализывая рану.",
		"generated_by": "llm", "fallback_reason": null,
		"kind": "round",
		"round": {"seq": 3},
		"llm_output": {"event": {"id": "ev-llm-1"}},
		"based_on": [{"event": {"id": "ev-combat-1", "type": "combat.decided"}}],
		"absence": {"since_at": "2026-09-08T21:00:00Z", "background_events_count": 3},
		"background_refs": [{"event": {"id": "ev-weather-1", "type": "world.weather_changed"}}],
		"filter": {"applied": true, "status": "pass", "filter_version": "a-2026-09"},
		"locale": "ru", "laws_version": "v1",
		"narrative_event_id": "ev-narrative-1"}`},
	"config.cloud_enabled": {SourceLLM, eventbus.ActorSystem, false, `{
		"enabled": false, "provider": "openai_compat", "external_players_ack": true}`},
	"world.laws.changed": {SourceMvctl, eventbus.ActorCI, false, `{
		"laws": {"version_from": "v1", "version_to": "v2", "status": "approved"},
		"created_by": "author",
		"diff": {"added": ["law-fire-burns"], "removed": []},
		"effective_from": {"round_boundary": true}}`},
	"world.law_breach.proposed": {SourceLaws, eventbus.ActorSystem, false, `{
		"breach": {"id": "breach-1"},
		"initiator": {"kind": "agent",
			"agent": {"id": "global-gm:dark-forest-world", "level": "global", "blueprint": "global-gm"}},
		"cause_event_ids": ["ev-1", "ev-2"],
		"strain": 0.7,
		"laws_removed": ["law-fire-burns"], "laws_added": ["law-fire-sings"],
		"canon_invalidated": ["fact-9"],
		"narrative_hook": "Огонь запел вместо того, чтобы жечь.",
		"laws_version_base": "v1"}`},
	"world.law_breach.rejected": {SourceLaws, eventbus.ActorSystem, false, `{
		"breach": {"id": "breach-1"}, "reason": "immutable_touched"}`},
	"world.law_breach.applied": {SourceLaws, eventbus.ActorSystem, false, `{
		"breach": {"id": "breach-1"},
		"laws": {"version_from": "v1", "version_to": "v2"},
		"review": {"deadline_at": "2026-09-10T12:00:00Z"}}`},
	"world.law_breach.review_decided": {SourceLaws, eventbus.ActorSystem, false, `{
		"breach": {"id": "breach-1"}, "decision": "approved", "reviewer_kind": "timeout",
		"decided_at": "2026-09-10T12:00:00Z"}`},
	"world.law_breach.rolled_back": {SourceLaws, eventbus.ActorSystem, false, `{
		"breach": {"id": "breach-1"},
		"laws": {"version_from": "v2", "version_to": "v1"},
		"retconned_fact_ids": ["fact-9"]}`},
	"analytics.session.started": {SourceGateway, eventbus.ActorHuman, false, `{
		"world": {"entity": {"id": "dark-forest-world", "type": "world"}},
		"scope": {"id": "solo:player-A", "type": "solo"},
		"session": {"id": "solo:player-A:1789000000", "kind": "solo", "actor_kind": "human",
			"started_at": "2026-09-09T12:00:00Z", "players_count": 1},
		"participants": [{"entity": {"id": "player-A", "type": "player"}}]}`},
	"analytics.session.ended": {SourceGateway, eventbus.ActorHuman, false, `{
		"world": {"entity": {"id": "dark-forest-world", "type": "world"}},
		"scope": {"id": "solo:player-A", "type": "solo"},
		"session": {"id": "solo:player-A:1789000000", "kind": "solo", "actor_kind": "human",
			"started_at": "2026-09-09T12:00:00Z", "ended_at": "2026-09-09T12:40:00Z",
			"end_reason": "forget", "players_count": 1,
			"turns_count": 30, "turns_degraded": 1, "turns_failed": 0},
		"participants": [{"entity": {"id": "player-A", "type": "player"}}]}`},
	"analytics.turn.completed": {SourceGateway, eventbus.ActorHuman, false, `{
		"world": {"entity": {"id": "dark-forest-world", "type": "world"}},
		"scope": {"id": "solo:player-A", "type": "solo"},
		"entity": {"entity": {"id": "player-A", "type": "player"}, "name": "Vasya"},
		"session": {"id": "solo:player-A:1789000000"},
		"turn": {"seq": 12, "action_type": "attack",
			"target": {"entity": {"id": "wolf-alpha", "type": "npc"}},
			"status": "ok", "gm_path": "agent", "phase1_mode": "rules", "lod": "basic"},
		"timings": {"received_at": "2026-09-09T12:00:00Z", "acked_at": "2026-09-09T12:00:00Z",
			"mechanics_at": "2026-09-09T12:00:00Z", "narrative_at": "2026-09-09T12:00:04Z",
			"mechanics_ms": 320, "narrative_ms": 3600, "total_ms": 3920},
		"narrative": {"generated_by": "llm", "agent_level": "task", "agent_blueprint": "player-gm",
			"fallback_reason": null, "filter_applied": true},
		"delivery": {"recipients_count": 1, "delivered_count": 1,
			"result_event_id": "ev-combat-1", "narrative_event_id": "ev-narrative-1"},
		"absence": {"since_at": "2026-09-08T21:00:00Z", "background_events_count": 3,
			"surfaced_event_ids": ["ev-weather-1"]}}`},
	"analytics.consistency.violated": {SourceMvctl, eventbus.ActorCI, false, `{
		"world": {"entity": {"id": "dark-forest-world", "type": "world"}},
		"scope": {"id": "solo:player-A", "type": "solo"},
		"session": {"id": "solo:player-A:1789000000"},
		"violation": {"code": "state_divergence", "layer": "state", "severity": "break",
			"detected_by": "snapshot_diff",
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"expected": 7, "actual": 4}}`},
}

// newBlockVEvent builds the envelope an example of block "в" travels in: the
// source of its publisher and, for a swarm type, the agent its topic policy
// demands.
func newBlockVEvent(t *testing.T, typ string, example blockVExample) eventbus.Event {
	t.Helper()
	scope := &eventbus.ScopeRef{ID: "solo:player-A", Type: "solo"}
	ev := eventbus.NewRoot(typ, example.source, "dark-forest-world", scope,
		example.actorKind, payloadOf(t, example.payload))
	if example.agent {
		ev.Meta.Agent = &eventbus.AgentRef{
			ID:        "player-gm:solo:player-A",
			Level:     "task",
			Blueprint: "player-gm",
		}
	}
	return ev
}

// TestBlockVPayloadExamples runs the examples of api-contracts.md §2.3 for the
// types of block "в" against their schemas, envelope included.
func TestBlockVPayloadExamples(t *testing.T) {
	for typ, example := range blockVExamples {
		t.Run(typ, func(t *testing.T) {
			if err := Validate(newBlockVEvent(t, typ, example)); err != nil {
				t.Fatalf("the example of §2.3 is rejected: %v", err)
			}
		})
	}
}

// TestBlockVPayloadRejects is the other half: a field nobody declared, a
// mandatory one left out and a value outside its enum must not reach a
// consumer. One case per family, chosen where a defect would be expensive.
func TestBlockVPayloadRejects(t *testing.T) {
	cases := map[string]struct {
		typ     string
		payload string
	}{
		// The scheduler computes lod_allowed and the agent does not recompute
		// it (C-06): an event without it would make a replay guess.
		"tick without lod_allowed": {"tick.fired", `{
			"tick": {"seq": 1, "mode": "background",
				"scheduled_at": "2026-09-09T12:00:00Z", "fired_at": "2026-09-09T12:00:01Z"},
			"budget": {"window_calls": 0, "cap": 4}}`},
		"unknown tick mode": {"tick.fired", `{
			"tick": {"seq": 1, "mode": "idle",
				"scheduled_at": "2026-09-09T12:00:00Z", "fired_at": "2026-09-09T12:00:01Z",
				"lod_allowed": "basic"},
			"budget": {"window_calls": 0, "cap": 4}}`},
		"unknown lod": {"tick.fired", `{
			"tick": {"seq": 1, "mode": "active",
				"scheduled_at": "2026-09-09T12:00:00Z", "fired_at": "2026-09-09T12:00:01Z",
				"lod_allowed": "maximum"},
			"budget": {"window_calls": 0, "cap": 4}}`},
		// The status of a record is the outcome of the pipeline; the reasons
		// of a refusal are a different field of a different type (ADR-017).
		"rejected reason used as a status": {"llm.output", `{
			"phase": "narrative", "attempt": 1, "provider": "fake", "model": "m", "params": {},
			"prompt_hash": "h", "response_hash": "h", "response_len": 1,
			"validation_status": "rejected_unknown_entity",
			"laws_version": "v1", "latency_ms": 1,
			"tokens": {"prompt": 1, "completion": 1}, "cost_usd": 0}`},
		"budget_exceeded used as a status": {"llm.output", `{
			"phase": "narrative", "attempt": 1, "provider": "fake", "model": "m", "params": {},
			"prompt_hash": "h", "response_hash": "h", "response_len": 1,
			"validation_status": "budget_exceeded",
			"laws_version": "v1", "latency_ms": 1,
			"tokens": {"prompt": 1, "completion": 1}, "cost_usd": 0}`},
		"record without a prompt hash": {"llm.output", `{
			"phase": "narrative", "attempt": 1, "provider": "fake", "model": "m", "params": {},
			"response_hash": "h", "response_len": 1, "validation_status": "valid",
			"laws_version": "v1", "latency_ms": 1,
			"tokens": {"prompt": 1, "completion": 1}, "cost_usd": 0}`},
		"unknown parse strategy": {"llm.output", `{
			"phase": "narrative", "attempt": 1, "provider": "fake", "model": "m", "params": {},
			"prompt_hash": "h", "response_hash": "h", "response_len": 1, "validation_status": "valid",
			"laws_version": "v1", "latency_ms": 1,
			"tokens": {"prompt": 1, "completion": 1}, "cost_usd": 0,
			"parse": {"strategy": "guess", "recovered": true}}`},
		"unknown rejection reason": {"llm.output.rejected", `{
			"reason": "too_long", "phase": "narrative", "attempt": 1}`},
		"narrative without recipients": {"narrative.output", `{
			"recipients": [], "text": "…", "generated_by": "llm", "kind": "turn",
			"locale": "ru", "laws_version": "v1"}`},
		"narrative generated by nobody": {"narrative.output", `{
			"recipients": [{"entity": {"id": "player-A", "type": "player"}}],
			"text": "…", "generated_by": "none", "kind": "turn",
			"locale": "ru", "laws_version": "v1"}`},
		"misspelt background refs": {"narrative.output", `{
			"recipients": [{"entity": {"id": "player-A", "type": "player"}}],
			"text": "…", "generated_by": "llm", "kind": "turn",
			"locale": "ru", "laws_version": "v1",
			"backgroud_refs": []}`},
		"round timeout is not a duration": {"encounter.started", `{
			"encounter": {"entity": {"id": "enc-7", "type": "encounter"}},
			"region": {"entity": {"id": "dark-forest-01", "type": "region"}},
			"participants": [], "npcs": [],
			"round": {"timeout": "one minute", "idle_after_missed": 2}}`},
		"unknown encounter end reason": {"encounter.ended", `{
			"encounter": {"entity": {"id": "enc-7", "type": "encounter"}},
			"reason": "boredom", "rounds": 1}`},
		"summary longer than 300": {"world.event_occurred", `{
			"kind": "storm", "summary": "` + strings.Repeat("a", 301) + `", "affects": []}`},
		"unknown spawn rejection": {"agent.spawn_rejected", `{
			"blueprint": "encounter-wolf", "reason": "too_many"}`},
		"agent stopped for no reason": {"agent.stopped", `{
			"agent": {"id": "a", "level": "task", "blueprint": "b"}}`},
		"unknown agent level": {"agent.spawned", `{
			"agent": {"id": "a", "level": "regional", "blueprint": "b"}}`},
		"unknown breach rejection": {"world.law_breach.rejected", `{
			"breach": {"id": "breach-1"}, "reason": "did_not_like_it"}`},
		"laws changed by nobody": {"world.laws.changed", `{
			"laws": {"version_from": "v1", "version_to": "v2", "status": "approved"},
			"created_by": "operator",
			"diff": {"added": [], "removed": []},
			"effective_from": {"round_boundary": true}}`},
		"cloud flag without a value": {"config.cloud_enabled", `{
			"provider": "openai_compat"}`},
		"cloud flag carrying a key": {"config.cloud_enabled", `{
			"enabled": true, "provider": "openai_compat", "api_key": "sk-secret"}`},
		"unknown end reason": {"analytics.session.ended", `{
			"session": {"id": "s", "kind": "solo", "actor_kind": "human",
				"started_at": "2026-09-09T12:00:00Z", "ended_at": "2026-09-09T12:40:00Z",
				"end_reason": "bored", "players_count": 1,
				"turns_count": 1, "turns_degraded": 0, "turns_failed": 0},
			"participants": [{"entity": {"id": "player-A", "type": "player"}}]}`},
		"turn without timings": {"analytics.turn.completed", `{
			"entity": {"entity": {"id": "player-A", "type": "player"}},
			"session": {"id": "s"},
			"turn": {"seq": 1, "action_type": "look", "status": "ok", "gm_path": "agent"},
			"narrative": {"generated_by": "none", "filter_applied": false},
			"delivery": {"recipients_count": 1, "delivered_count": 1}}`},
		"unknown violation severity": {"analytics.consistency.violated", `{
			"violation": {"code": "state_divergence", "layer": "state", "severity": "fatal",
				"detected_by": "snapshot_diff"}}`},
		"unknown violation code": {"analytics.consistency.violated", `{
			"violation": {"code": "world_is_odd", "layer": "state", "severity": "warn",
				"detected_by": "human"}}`},
		"violation without detected_by": {"analytics.consistency.violated", `{
			"violation": {"code": "log_gap", "layer": "state", "severity": "warn"}}`},
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			ev := newBlockVEvent(t, bad.typ, blockVExample{
				source:    SourceSwarm,
				actorKind: eventbus.ActorSystem,
				agent:     true,
				payload:   bad.payload,
			})
			if err := Validate(ev); !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("error %v, want an invalid payload", err)
			}
		})
	}
}

// TestQuarantinedRecordKeepsNoText pins the rule the category (a) filter rests
// on (C-07, ADR-016 p. 3): a text that was blocked is not stored, so a record
// with validation_status=quarantined must not carry response_raw. The hash and
// the length stay — they are what an audit needs.
func TestQuarantinedRecordKeepsNoText(t *testing.T) {
	const withoutText = `{
		"phase": "narrative", "attempt": 1, "provider": "openai_compat", "model": "qwen3-8b",
		"params": {"temperature": 0.7},
		"prompt_hash": "9f2c4d", "response_hash": "1a2b3c", "response_len": 214,
		"validation_status": "quarantined",
		"filter": {"applied": true, "status": "block", "filter_version": "a-2026-09"},
		"laws_version": "v1", "latency_ms": 1840,
		"tokens": {"prompt": 1200, "completion": 180}, "cost_usd": 0,
		"reasons": ["filter_blocked"]}`

	example := blockVExample{SourceLLM, eventbus.ActorHuman, true, withoutText}
	if err := Validate(newBlockVEvent(t, "llm.output", example)); err != nil {
		t.Fatalf("a quarantined record without the text is rejected: %v", err)
	}

	example.payload = strings.Replace(withoutText,
		`"validation_status": "quarantined",`,
		`"validation_status": "quarantined", "response_raw": "the blocked text",`, 1)
	if err := Validate(newBlockVEvent(t, "llm.output", example)); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("error %v, want the blocked text refused", err)
	}

	// The same text under a status that keeps it is fine: the rule is about
	// quarantine, not about response_raw as such.
	example.payload = strings.Replace(withoutText,
		`"validation_status": "quarantined",`,
		`"validation_status": "valid", "response_raw": "the text",`, 1)
	if err := Validate(newBlockVEvent(t, "llm.output", example)); err != nil {
		t.Fatalf("a valid record with the text is rejected: %v", err)
	}

	// filter_error means the filter itself failed, so the text was never
	// judged: it is withheld on the same grounds as a blocked one
	// (orchestrator decision on OV-33, review T-009 M-1).
	example.payload = strings.Replace(withoutText,
		`"validation_status": "quarantined",`,
		`"validation_status": "filter_error",`, 1)
	if err := Validate(newBlockVEvent(t, "llm.output", example)); err != nil {
		t.Fatalf("a filter_error record without the text is rejected: %v", err)
	}

	example.payload = strings.Replace(withoutText,
		`"validation_status": "quarantined",`,
		`"validation_status": "filter_error", "response_raw": "the unjudged text",`, 1)
	if err := Validate(newBlockVEvent(t, "llm.output", example)); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("error %v, want the unjudged text refused", err)
	}
}

// TestValidationStatusEnum pins the six values of C-07 v1.2 (ADR-017 addendum
// 1) in both directions: all six are accepted, and the statuses that were
// removed from data-model.md §7.2 v0.2 are not.
func TestValidationStatusEnum(t *testing.T) {
	want := []string{"valid", "partially_rejected", "invalid", "error", "quarantined", "filter_error"}
	got := schemaEnum(t, "llm.output.v1.json", "validation_status")
	if !slices.Equal(got, want) {
		t.Errorf("validation_status %v, want the six values of C-07 v1.2 %v", got, want)
	}
}

// TestRejectionReasonEnum pins the reason enum of ADR-017 p. 3, which the
// guardian of EPIC-003 shares with this schema: llm.output.reasons[] is the
// denormalised copy of it and must not drift.
func TestRejectionReasonEnum(t *testing.T) {
	want := []string{"unknown_entity", "player_agency", "level_violation", "schema_invalid",
		"language", "filter_blocked", "filter_error", "budget_exceeded", "law_violation", "other"}

	got := schemaEnum(t, "llm.output.rejected.v1.json", "reason")
	if !slices.Equal(got, want) {
		t.Errorf("llm.output.rejected.reason %v, want %v", got, want)
	}

	doc := readSchemaDoc(t, "llm.output.v1.json")
	properties, _ := doc["properties"].(map[string]any)
	reasons, _ := properties["reasons"].(map[string]any)
	items, _ := reasons["items"].(map[string]any)
	if copied := enumOf(t, items); !slices.Equal(copied, want) {
		t.Errorf("llm.output.reasons[] %v, want the same enum %v", copied, want)
	}
}

// TestEndReasonIncludesForget guards the addition of C-10 v1.1: the /forget
// cascade closes an active session, and that close is not an error.
func TestEndReasonIncludesForget(t *testing.T) {
	doc := readSchemaDoc(t, "analytics.session.ended.v1.json")
	properties, _ := doc["properties"].(map[string]any)
	session, _ := properties["session"].(map[string]any)
	fields, _ := session["properties"].(map[string]any)
	endReason, _ := fields["end_reason"].(map[string]any)
	got := enumOf(t, endReason)
	want := []string{"leave", "idle", "death", "error", "forget"}
	if !slices.Equal(got, want) {
		t.Errorf("end_reason %v, want %v", got, want)
	}
}

// TestSwarmTopicsDemandAnAgent covers the topic policies of api-contracts.md
// §2.0: an event of the swarm that does not name the agent behind it cannot be
// replayed, budgeted or audited, so the bus refuses it on publish and on read.
func TestSwarmTopicsDemandAnAgent(t *testing.T) {
	swarm := []string{
		"llm.output", "llm.output.rejected", "narrative.output", "combat.decided",
		"tick.fired", "tick.aborted",
		"agent.spawned", "agent.child_resolved", "agent.stopped",
		"agent.spawn_rejected", "agent.blueprint_reloaded",
		"encounter.started", "encounter.ended",
		"world.weather_changed", "world.time_advanced", "world.event_occurred",
		"region.event_occurred", "npc.moved", "npc.spawned",
		"content.incident.recorded",
	}
	for _, typ := range swarm {
		t.Run(typ, func(t *testing.T) {
			spec, ok := Lookup(typ)
			if !ok {
				t.Fatalf("%s is not registered", typ)
			}
			example, ok := blockVExamples[typ]
			if !ok {
				t.Fatalf("%s has no example", typ)
			}

			with := newBlockVEvent(t, typ, example)
			if err := spec.Policy.Check(with); err != nil {
				t.Errorf("an event naming its agent is refused: %v", err)
			}

			example.agent = false
			without := newBlockVEvent(t, typ, example)
			if err := spec.Policy.Check(without); !errors.Is(err, eventbus.ErrPolicyViolation) {
				t.Errorf("error %v, want meta.agent demanded", err)
			}
		})
	}
}

// TestTopicsWithoutAnAgentRule names the types that share a swarm topic and
// carry no agent: the author bumping the laws from the CLI, the breach engine
// of E-B and the context that publishes the cloud flag at start. Demanding an
// agent there would block a legitimate publisher.
func TestTopicsWithoutAnAgentRule(t *testing.T) {
	for _, typ := range []string{
		"world.laws.changed", "config.cloud_enabled",
		"world.law_breach.proposed", "world.law_breach.rejected", "world.law_breach.applied",
		"world.law_breach.review_decided", "world.law_breach.rolled_back",
		"analytics.session.started", "analytics.session.ended",
		"analytics.turn.completed", "analytics.consistency.violated",
	} {
		spec, ok := Lookup(typ)
		if !ok {
			t.Errorf("%s is not registered", typ)
			continue
		}
		if spec.Policy.Agent != eventbus.AgentOptional {
			t.Errorf("%s: agent rule %v, want none", typ, spec.Policy.Agent)
		}
	}
}

// TestPlayerEventsRejectSystem is the other side of the same rule: the topic
// of player actions carries what a person, a harness or a simulator did. A
// tick is not a player, and neither is an agent.
func TestPlayerEventsRejectSystem(t *testing.T) {
	payload := payloadOf(t, `{"entity": {"entity": {"id": "player-A", "type": "player"}}}`)
	for _, spec := range All() {
		if spec.Topic != eventbus.TopicPlayerEvents || spec.Deprecated {
			continue
		}
		ev := newEvent(t, spec.Type, eventbus.ActorSystem, payload)
		if err := spec.Policy.Check(ev); !errors.Is(err, eventbus.ErrPolicyViolation) {
			t.Errorf("%s: actor_kind=system accepted on player_events: %v", spec.Type, err)
		}
		ev.Meta.ActorKind = eventbus.ActorSim
		if err := spec.Policy.Check(ev); err != nil {
			t.Errorf("%s: actor_kind=sim refused on player_events: %v", spec.Type, err)
		}
	}
}

// TestEveryTopicCarriesAType closes the loop of the topic map: a topic nobody
// publishes to is a topic redpanda-init creates for nothing. dead_letters is
// the one exception by design — the bus writes the DeadLetter wrapper there
// itself, and the wrapper is not an event with a type of its own.
func TestEveryTopicCarriesAType(t *testing.T) {
	counted := map[string]int{}
	for _, spec := range All() {
		counted[spec.Topic]++
	}
	for _, topic := range Topics() {
		if topic.Name == eventbus.TopicDeadLetters {
			if counted[topic.Name] != 0 {
				t.Errorf("%s: a type was routed to the dead letter topic", topic.Name)
			}
			continue
		}
		if counted[topic.Name] == 0 {
			t.Errorf("%s: no registered type goes to this topic", topic.Name)
		}
	}
}

// TestReservedTypes names the exception of contracts.md §16 p. 4: the breach
// family is registered, valid and published by nobody until E-B. Anything else
// marked reserved is a defect — the flag is not a way around the rule that a
// type needs a publisher and a reader.
func TestReservedTypes(t *testing.T) {
	var got []string
	for _, spec := range All() {
		if spec.Reserved {
			got = append(got, spec.Type)
		}
	}
	want := []string{
		"world.law_breach.proposed", "world.law_breach.rejected", "world.law_breach.applied",
		"world.law_breach.review_decided", "world.law_breach.rolled_back",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("reserved types %v, want %v", got, want)
	}
}

// TestBlockVOwners pins who answers for each family of block "в"
// (contracts.md §0): the swarm owns the world it tells about, the gateway what
// it measures of a turn, and the operator CLI what its audit found.
func TestBlockVOwners(t *testing.T) {
	cases := map[string]string{
		"combat.decided":                 OwnerSwarm,
		"encounter.started":              OwnerSwarm,
		"tick.fired":                     OwnerSwarm,
		"agent.spawned":                  OwnerSwarm,
		"llm.output":                     OwnerSwarm,
		"narrative.output":               OwnerSwarm,
		"config.cloud_enabled":           OwnerSwarm,
		"world.laws.changed":             OwnerSwarm,
		"world.law_breach.applied":       OwnerSwarm,
		"analytics.session.started":      OwnerGateway,
		"analytics.session.ended":        OwnerGateway,
		"analytics.turn.completed":       OwnerGateway,
		"analytics.consistency.violated": OwnerOps,
	}
	for typ, want := range cases {
		spec, ok := Lookup(typ)
		if !ok {
			t.Errorf("%s is not registered", typ)
			continue
		}
		if spec.Owner != want {
			t.Errorf("%s: owner %s, want %s", typ, spec.Owner, want)
		}
	}
}

// schemaEnum reads the enum of a top-level payload field of a schema file.
func schemaEnum(t *testing.T, file, field string) []string {
	t.Helper()
	doc := readSchemaDoc(t, file)
	properties, _ := doc["properties"].(map[string]any)
	property, ok := properties[field].(map[string]any)
	if !ok {
		t.Fatalf("%s: no field %s", file, field)
	}
	return enumOf(t, property)
}

func enumOf(t *testing.T, property map[string]any) []string {
	t.Helper()
	raw, ok := property["enum"].([]any)
	if !ok {
		t.Fatal("the field carries no enum")
	}
	values := make([]string, 0, len(raw))
	for _, value := range raw {
		text, _ := value.(string)
		values = append(values, text)
	}
	return values
}
