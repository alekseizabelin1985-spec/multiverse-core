package main

import (
	"context"
	"fmt"

	"multiverse-core.io/shared/runtime"
)

// Registration of the contexts of the platform (C-01, ADR-001 addendum
// 2026-09-13 p. 7). The start order and the registration live here, in the one
// file of EPIC-001; the factory of each context is a package variable in the
// file of its owner:
//
//	contexts_state.go   — state, mechanics (EPIC-002)
//	contexts_swarm.go   — llm, laws, swarm (EPIC-003)
//	contexts_gateway.go — gateway (EPIC-004)
//	contexts_memory.go  — memory (EPIC-005)
//
// An owner replaces the stub in its own file by changing the value of its
// variable, and never calls runtime.Register itself: a second registration of
// a name panics, and a registration from an init of its own would put the
// context where the name of its file sorts rather than where the platform
// starts it. Package variables are initialised before any init function runs,
// so the loop below sees every factory whatever the files are called.
//
// swarm is still the exception until T-256 removes the hook of I1-α: its
// variable holds newSwarm of fake_contexts.go, which chooses between the stub
// and the fake by MV_SWARM_FAKE (T-255).
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

// factoryOf is the one place a name of the start order meets the variable its
// owner declares. A name without a variable, or a variable left nil, is a
// defect of the build and stops the binary at start.
func factoryOf(name string) func() runtime.Context {
	var factory func() runtime.Context
	switch name {
	case "state":
		factory = newStateContext
	case "mechanics":
		factory = newMechanicsContext
	case "llm":
		factory = newLLMContext
	case "laws":
		factory = newLawsContext
	case swarmContext:
		factory = newSwarmContext
	case "gateway":
		factory = newGatewayContext
	case "memory":
		factory = newMemoryContext
	}
	if factory == nil {
		panic(fmt.Sprintf("cmd/multiverse: context %q has no factory", name))
	}
	return factory
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
