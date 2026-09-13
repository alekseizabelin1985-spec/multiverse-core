package agent_test

import (
	"testing"

	"multiverse-core.io/shared/agent"
)

func TestAgentLevelString(t *testing.T) {
	cases := map[agent.AgentLevel]string{
		agent.LevelUnknown:   "unknown",
		agent.LevelGlobal:    "global",
		agent.LevelDomain:    "domain",
		agent.LevelTask:      "task",
		agent.LevelObject:    "object",
		agent.LevelMonitor:   "monitor",
		agent.AgentLevel(99): "unknown",
	}
	for level, want := range cases {
		if got := level.String(); got != want {
			t.Errorf("AgentLevel(%d).String() = %q, want %q", int(level), got, want)
		}
	}
}

func TestLODLevelString(t *testing.T) {
	cases := map[agent.LODLevel]string{
		agent.LODDisabled:  "disabled",
		agent.LODRuleOnly:  "rule-only",
		agent.LODBasic:     "basic",
		agent.LODFull:      "full",
		agent.LODLevel(-1): "unknown",
	}
	for lod, want := range cases {
		if got := lod.String(); got != want {
			t.Errorf("LODLevel(%d).String() = %q, want %q", int(lod), got, want)
		}
	}
}

func TestAgentLifecycleStateString(t *testing.T) {
	cases := map[agent.AgentLifecycleState]string{
		agent.LifecycleInitializing:   "initializing",
		agent.LifecycleRunning:        "running",
		agent.LifecyclePausing:        "pausing",
		agent.LifecyclePaused:         "paused",
		agent.LifecycleResuming:       "resuming",
		agent.LifecycleFinishing:      "finishing",
		agent.LifecycleFinished:       "finished",
		agent.AgentLifecycleState(42): "unknown",
	}
	for state, want := range cases {
		if got := state.String(); got != want {
			t.Errorf("AgentLifecycleState(%d).String() = %q, want %q", int(state), got, want)
		}
	}
}
