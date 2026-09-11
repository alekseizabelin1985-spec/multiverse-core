package main

import (
	"context"

	"multiverse-core.io/shared/runtime"
)

// Wave 0 registers empty contexts so that --contexts=all and the compose
// profiles work before internal/* exists. Each owner replaces its stub with a
// real runtime.Register in its own branch (design.md §4.1):
// state, mechanics, replay — EPIC-002; llm, laws — EPIC-003;
// gateway — EPIC-004; memory — EPIC-005.
//
// swarm is the exception until T-256 removes the hook of I1-α. It keeps its
// place in the order below but is not built here: its factory is newSwarm of
// fake_contexts.go, which chooses between the stub of this file and the fake by
// MV_SWARM_FAKE (T-255). It is registered once, from here, and internal/swarm
// (EPIC-003) arrives as the constructor on the last line of newSwarm — not as a
// runtime.Register of its own: a second registration of the name panics.
//
// The order below is the documented start order of the platform
// (foundation.md §2): contexts without dependencies keep it as a tie-break.
var platformContexts = []string{"state", "laws", "mechanics", "llm", swarmContext, "gateway", "memory"}

// swarmContext is the name of the context the hook of fake_contexts.go builds.
const swarmContext = "swarm"

func init() {
	for _, name := range platformContexts {
		runtime.Register(name, factoryOf(name))
	}
}

// factoryOf is the factory a context of the platform is registered with: the
// stub below for every context whose owner has not implemented it, and the
// hook for swarm.
func factoryOf(name string) func() runtime.Context {
	if name == swarmContext {
		return newSwarm
	}
	return newStub(name)
}

func newStub(name string) func() runtime.Context {
	return func() runtime.Context { return stub{name: name} }
}

// stub is a context that exists only to answer /health until its owner
// implements it.
type stub struct{ name string }

func (s stub) Name() string                              { return s.name }
func (s stub) DependsOn() []string                       { return nil }
func (s stub) Start(context.Context, runtime.Deps) error { return nil }
func (s stub) Stop(context.Context) error                { return nil }
func (s stub) Health() runtime.Status                    { return runtime.OK() }
