// Package agent holds what the swarm of GM agents shares with its authors and
// tools: the level and LOD types, the blueprint format v2 (C-11, ADR-015), its
// parser and the dictionary of prompt placeholders.
//
// The package is a library layer: it imports no context (internal/*) and no
// contract registry. Whatever a check needs from the project — event types,
// ownership rows, files — is handed in by the caller (ADR-015 p. 3, ADR-025).
package agent

// AgentLevel is the hierarchical level of an agent in the swarm.
type AgentLevel int

const (
	LevelUnknown AgentLevel = iota
	LevelGlobal             // supervises the whole world
	LevelDomain             // a region, a city or a zone
	LevelTask               // a quest, an encounter, a player's narrator
	LevelObject             // an entity actor; reserved, spawn disabled in MVP-1
	LevelMonitor            // long-running anomaly monitor; reserved in MVP-1
)

// String returns the name of the level as written in a blueprint.
func (l AgentLevel) String() string {
	switch l {
	case LevelGlobal:
		return "global"
	case LevelDomain:
		return "domain"
	case LevelTask:
		return "task"
	case LevelObject:
		return "object"
	case LevelMonitor:
		return "monitor"
	default:
		return "unknown"
	}
}

// LODLevel is the level of detail an agent runs at.
type LODLevel int

const (
	LODDisabled LODLevel = iota // the agent sleeps
	LODRuleOnly                 // rules only, no LLM
	LODBasic                    // a plain LLM call
	LODFull                     // the full loop with tools
)

// String returns the name of the LOD as written in a blueprint.
func (l LODLevel) String() string {
	switch l {
	case LODDisabled:
		return "disabled"
	case LODRuleOnly:
		return "rule-only"
	case LODBasic:
		return "basic"
	case LODFull:
		return "full"
	default:
		return "unknown"
	}
}

// AgentLifecycleState is the state of an agent instance.
type AgentLifecycleState int

const (
	LifecycleInitializing AgentLifecycleState = iota
	LifecycleRunning
	LifecyclePausing
	LifecyclePaused
	LifecycleResuming
	LifecycleFinishing
	LifecycleFinished
)

// String returns the name of the state.
func (s AgentLifecycleState) String() string {
	switch s {
	case LifecycleInitializing:
		return "initializing"
	case LifecycleRunning:
		return "running"
	case LifecyclePausing:
		return "pausing"
	case LifecyclePaused:
		return "paused"
	case LifecycleResuming:
		return "resuming"
	case LifecycleFinishing:
		return "finishing"
	case LifecycleFinished:
		return "finished"
	default:
		return "unknown"
	}
}
