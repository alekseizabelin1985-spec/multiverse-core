package agent

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentBlueprint is the blueprint format v2 (C-11, ADR-015,
// swarm-llm-laws.md §13.1). The type was extended in place: the fields of the
// as-is format are still accepted by the parser, which moves them to their v2
// place and reports a warning (see the Legacy* fields).
//
// The parser fills the struct; checking it against the project (levels, event
// types, files, models) is the job of the validator, so nothing here is
// required at the type level.
type AgentBlueprint struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`

	Level        string       `yaml:"level"`
	Role         string       `yaml:"role"`
	ScopeBinding ScopeBinding `yaml:"scope_binding"`

	Parent      *ParentReference     `yaml:"parent"`
	Trigger     BlueprintTrigger     `yaml:"trigger"`
	TTL         string               `yaml:"ttl"`
	Constraints BlueprintConstraints `yaml:"constraints"`
	LLM         LLMSettings          `yaml:"llm"`

	AllowedEventTypes []string        `yaml:"allowed_event_types"`
	OwnedEntityTypes  []string        `yaml:"owned_entity_types"`
	Tools             []ToolReference `yaml:"tools"`

	LawsRef           string `yaml:"laws_ref"`
	RulesRef          string `yaml:"rules_ref"`
	AbsoluteLimitsRef string `yaml:"absolute_limits_ref"`

	NPCTable         []NPCRow          `yaml:"npc_table"`
	RespawnTTL       string            `yaml:"respawn_ttl"`
	Encounter        *EncounterCfg     `yaml:"encounter"`
	BackgroundEvents []BackgroundEvent `yaml:"background_events"`
	Round            *RoundCfg         `yaml:"round"`
	Budget           *BudgetCfg        `yaml:"budget"`
	Invariants       []InvariantRef    `yaml:"invariants"`

	Locale             string   `yaml:"locale"`
	ImmediateBroadcast []string `yaml:"immediate_broadcast"`

	// Prompts come from the sections of a .md file, or from the key prompts of
	// a pure YAML file.
	Prompts Prompts `yaml:"prompts"`

	// LegacyType is the as-is key type, an alias of role. The parser moves it
	// to Role and leaves it empty.
	LegacyType string `yaml:"type"`
	// LegacyPhase1Prompt and LegacyPhase2Prompt are the as-is keys
	// phase1_prompt/phase2_prompt. The parser moves them to Prompts with a
	// warning and leaves them nil; a pointer, so that a key present with an
	// empty value is still reported.
	LegacyPhase1Prompt *string `yaml:"phase1_prompt"`
	LegacyPhase2Prompt *string `yaml:"phase2_prompt"`

	// SourceFile is the name the blueprint was parsed from.
	SourceFile string `yaml:"-"`
	// ContentHash is ContentHash(bp) at parse time: "sha256:<64 hex>".
	ContentHash string `yaml:"-"`
	// ParseIssues are the non-fatal findings of the parser (migrations of the
	// as-is format). The validator reports them together with its own.
	ParseIssues []Issue `yaml:"-"`
}

// ScopeBinding binds an agent to the scope it serves.
type ScopeBinding struct {
	// Type is one or more of world|region|solo|group. A blueprint writes it
	// either as a list or as one string separated by "|" ("solo|group").
	Type    ScopeTypes `yaml:"type"`
	ID      string     `yaml:"id"`
	Pattern string     `yaml:"pattern"`
}

// ScopeTypes is the list of scope types of a binding.
type ScopeTypes []string

// UnmarshalYAML accepts "solo|group" as well as [solo, group]. Only scalar and
// sequence nodes reach node.Decode here, so the strict mode of the outer
// decoder (unknown keys) is not bypassed: neither form has keys.
func (s *ScopeTypes) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var joined string
		if err := node.Decode(&joined); err != nil {
			return err
		}
		*s = splitScopeTypes(joined)
		return nil
	}
	var list []string
	if err := node.Decode(&list); err != nil {
		return err
	}
	*s = list
	return nil
}

func splitScopeTypes(joined string) ScopeTypes {
	if strings.TrimSpace(joined) == "" {
		return nil
	}
	parts := strings.Split(joined, "|")
	out := make(ScopeTypes, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// ParentReference names the parent blueprint. Instance "dynamic" means the
// parent is resolved at spawn time (the region GM of the player's position).
type ParentReference struct {
	Name     string `yaml:"name"`
	Instance string `yaml:"instance"`
}

// BlueprintTrigger says when an agent is spawned or woken up.
type BlueprintTrigger struct {
	Type       string      `yaml:"type"`       // timer|event
	EventName  string      `yaml:"event_name"` // a type of the registry; glob by segments is allowed
	Conditions []Condition `yaml:"conditions"`
	Intervals  *Intervals  `yaml:"intervals"`
}

// Intervals of a timer trigger, as durations ("30m", "60s").
type Intervals struct {
	Idle   string `yaml:"idle"`
	Active string `yaml:"active"`
}

// Condition is a spawn condition of the as-is format. Value is untyped because
// the as-is files compare with numbers and with lists ("in").
type Condition struct {
	Field    string `yaml:"field"`
	Operator string `yaml:"operator"`
	Value    any    `yaml:"value"`
}

// BlueprintConstraints limits the instances of a blueprint.
type BlueprintConstraints struct {
	MaxInstances int `yaml:"max_instances"`
	Priority     int `yaml:"priority"`
}

// LLMSettings configures the phases of an agent.
type LLMSettings struct {
	Phase1 *PhaseLLM `yaml:"phase1"`
	Phase2 *PhaseLLM `yaml:"phase2"`
	Tick   *TickLLM  `yaml:"tick"`
	// Retries is nil when the blueprint leaves the default of the gateway.
	Retries  *int   `yaml:"retries"`
	Fallback string `yaml:"fallback"` // template|rules

	// LegacyModel, LegacyTemperature and LegacyMaxTokens are the flat as-is
	// keys llm.model/temperature/max_tokens. The parser moves them to Phase2
	// with a warning and leaves them nil. They are pointers because the
	// warning is about the presence of the key, not about its value.
	LegacyModel       *string  `yaml:"model"`
	LegacyTemperature *float64 `yaml:"temperature"`
	LegacyMaxTokens   *int     `yaml:"max_tokens"`
	// LegacySchema is the flat as-is inline schema. v2 has no place for an
	// inline schema (a phase names a file by schema_ref), so the parser refuses
	// it instead of dropping it.
	LegacySchema map[string]any `yaml:"schema"`
}

// PhaseLLM configures phase 1 or phase 2.
type PhaseLLM struct {
	Mode  string `yaml:"mode"` // rules|llm; rules needs no model
	Model string `yaml:"model"`
	// Temperature is nil when the blueprint leaves the default of the phase
	// profile of the gateway; 0 is a valid temperature.
	Temperature *float64 `yaml:"temperature"`
	MaxTokens   int      `yaml:"max_tokens"`
	Thinking    bool     `yaml:"thinking"`
	SchemaRef   string   `yaml:"schema_ref"`
}

// TickLLM configures the tick phase of timer agents. Temperature and Thinking
// are accepted as in the other phases: the format of a phase is the same for
// all of them (api-contracts.md §3, T-203).
type TickLLM struct {
	Model       string   `yaml:"model"`
	LODDefault  string   `yaml:"lod_default"` // rule-only|basic
	Temperature *float64 `yaml:"temperature"`
	MaxTokens   int      `yaml:"max_tokens"`
	Thinking    bool     `yaml:"thinking"`
	SchemaRef   string   `yaml:"schema_ref"`
}

// ToolReference names a tool of the tool registry.
type ToolReference struct {
	Name  string `yaml:"name"`
	Owner string `yaml:"owner"`
}

// NPCRow is a row of the NPC table of a region; used only for respawn.
type NPCRow struct {
	NPCID    string   `yaml:"npc_id"`
	Kind     string   `yaml:"kind"`
	Name     string   `yaml:"name"`
	StatsRef string   `yaml:"stats_ref"`
	Count    int      `yaml:"count"`
	Spawn    NPCSpawn `yaml:"spawn"`
}

// NPCSpawn says when an NPC of the table appears.
type NPCSpawn struct {
	On     string `yaml:"on"` // region_init|tick
	Chance string `yaml:"chance"`
}

// EncounterCfg says how a region detects an encounter and which blueprint runs it.
type EncounterCfg struct {
	DetectOn       string `yaml:"detect_on"`
	Perception     string `yaml:"perception"` // all|radius
	Chance         string `yaml:"chance"`
	ChildBlueprint string `yaml:"child_blueprint"`
}

// BackgroundEvent is a row of the LOD 1 table of background events (no LLM).
type BackgroundEvent struct {
	Kind            string       `yaml:"kind"`
	Weight          int          `yaml:"weight"`
	SummaryTemplate string       `yaml:"summary_template"`
	Ops             []OpTemplate `yaml:"ops"`
}

// OpTemplate is an entity operation proposed by a background event.
type OpTemplate struct {
	Path  string `yaml:"path"`
	Value any    `yaml:"value"`
}

// RoundCfg is the round of a group encounter; the source of encounter.started.round.
type RoundCfg struct {
	Timeout         string `yaml:"timeout"`
	IdleAfterMissed int    `yaml:"idle_after_missed"`
}

// BudgetCfg is the background budget of a world.
type BudgetCfg struct {
	BackgroundCallsPerHourWorld int `yaml:"background_calls_per_hour_world"`
}

// InvariantRef names an invariant of the world laws.
type InvariantRef struct {
	ID    string `yaml:"id"`
	Check string `yaml:"check"`
}

// Prompts are the text sections of a blueprint.
type Prompts struct {
	System      string   `yaml:"system"`
	Phase1      string   `yaml:"phase1"`
	Phase2      string   `yaml:"phase2"`
	Tick        string   `yaml:"tick"`
	Canon       []string `yaml:"canon"`
	Description string   `yaml:"description"`
}
